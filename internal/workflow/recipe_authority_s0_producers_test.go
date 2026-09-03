package workflow

// GH #15 / ADR-036 slice S0 — frozen evidence for the seven governed
// producers (PRD-recipe-generation-authority §6.15, §8).
//
// S0 requires: "Freeze the current `feature patch`, `reconcile --accept`,
// `cycle`, `apply --mode done`, `implement` and `tpatch edit` artifact-write
// goldens, since S4 adds a publication step to each."
//
// The registry is P1 `record` (with `land` as orchestration only), P2
// `feature patch refresh|fixup`, P3 `RefreshAfterAccept`, P4 `cycle`, P5
// `apply --mode done`, P6 `implement`, P7 `tpatch edit`.
//
// Runtime cost is deliberately asymmetric: P3 and P6 own real behavioural
// rows because their orderings are cheap to exercise, while P1/P2/P4/P5/P7
// are frozen with AST guards over their shipped write orderings plus the
// bound-write inventory in recipe_authority_s0_source_guards_test.go.
// Re-running their full end-to-end fixtures would duplicate existing
// suites for no additional evidence.

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// rgaS0CoverageArtifactName is the artifact GH #15 will introduce. S0
// asserts every producer currently leaves it absent.
var rgaS0CoverageArtifactName = "recipe-coverage.json"

// rgaS0AssertNoCoverageArtifact fails when a producer left a coverage
// record on disk. S0's whole premise is that none exists yet.
func rgaS0AssertNoCoverageArtifact(t *testing.T, root, slug string) {
	t.Helper()
	path := filepath.Join(root, ".tpatch", "features", slug, "artifacts", rgaS0CoverageArtifactName)
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("a coverage artifact already exists at %s; the S0 baseline is stale", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

// ── the registry itself ──────────────────────────────────────────────────

// TestRGAS0GovernedProducerRegistry freezes which producer owns which
// bound write, derived from source rather than declared twice. `land` is
// asserted to own NONE: PRD §2.7 records it as P1 orchestration, and a
// registry entry for it would double-count `record`'s own write.
func TestRGAS0GovernedProducerRegistry(t *testing.T) {
	type producer struct {
		id    string
		sites []string
	}
	registry := []producer{
		{id: "P1 record", sites: []string{"internal/cli/cobra.go|recordCmd|post-apply.patch"}},
		{id: "P2 feature-patch-amend", sites: []string{"internal/cli/feature_patch.go|runFeaturePatchAmend|post-apply.patch"}},
		{id: "P3 reconcile-accept", sites: []string{"internal/workflow/refresh.go|RefreshAfterAccept|post-apply.patch"}},
		{id: "P4 cycle", sites: []string{"internal/cli/phase2.go|cycleCmd|post-apply.patch"}},
		{id: "P5 apply-done", sites: []string{"internal/cli/cobra.go|runApplyDone|post-apply.patch"}},
		{id: "P6 implement", sites: []string{"internal/workflow/implement.go|RunImplement|apply-recipe.json"}},
		// P1/P2 both reach the shared recipe writer; it maps to a
		// producer by caller (PRD §6.15 rev-5), not by its own identity.
		{id: "P1/P2 shared recipe writer", sites: []string{"internal/workflow/recipe_autogen.go|writeRecipe|apply-recipe.json"}},
	}

	inventory := map[string]int{}
	for _, rel := range rgaS0ProductionGoFiles(t) {
		bound, _, err := rgaS0ScanWriteArtifact(rel, rgaS0ReadRepoFile(t, rel))
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		for _, site := range bound {
			inventory[site.key()]++
		}
	}

	claimed := map[string]bool{}
	for _, p := range registry {
		for _, site := range p.sites {
			if inventory[site] == 0 {
				t.Errorf("%s: registered bound write site %q is gone from production", p.id, site)
			}
			claimed[site] = true
		}
	}
	for site := range inventory {
		if !claimed[site] {
			t.Errorf("bound write site %q belongs to no registered producer", site)
		}
	}

	// P7 `tpatch edit` writes through the operator's editor, so it owns
	// no direct `WriteArtifact` site. Its trigger is pinned by the CLI
	// suite (`resolveArtifactPath` precedence + `openInEditor`).
	// `tpatch land` owns none either.
	for _, rel := range []string{"internal/cli/land.go", "internal/cli/c1.go"} {
		bound, _, err := rgaS0ScanWriteArtifact(rel, rgaS0ReadRepoFile(t, rel))
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		if len(bound) != 0 {
			t.Errorf("%s must own no direct bound write (PRD §2.7: land is P1 orchestration, edit goes through $EDITOR); got %+v", rel, bound)
		}
	}
}

// ── P2: the two zero-write skip branches ─────────────────────────────────

// rgaS0CheckFeaturePatchSkipBranches asserts that both "skipped" branches
// of `runFeaturePatchAmend` print their message and return `nil` WITHOUT
// writing anything. §6.15 turns the second of them into a category-(c)
// checkpoint that publishes coverage; today it publishes nothing.
func rgaS0CheckFeaturePatchSkipBranches(src string) error {
	file, err := rgaS0Parse("feature_patch.go", src)
	if err != nil {
		return err
	}
	fn := rgaS0FuncBody(file, "runFeaturePatchAmend")
	if fn == nil {
		return fmt.Errorf("runFeaturePatchAmend not found")
	}

	var messages []string
	writesInSkipBranch := 0
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		if rgaS0CallName(call) != "fmt.Fprintln" {
			return true
		}
		value, isLit := rgaS0StringLit(call.Args[1])
		if !isLit || !strings.Contains(value, "no patch byte change") {
			return true
		}
		messages = append(messages, value)
		return true
	})
	sort.Strings(messages)
	want := []string{"no patch byte change; fixup skipped", "no patch byte change; refresh skipped"}
	if len(messages) != 3 {
		return fmt.Errorf("want 3 skip messages (2 refresh + 1 fixup), got %d: %v", len(messages), messages)
	}
	if messages[0] != want[0] || messages[1] != want[1] || messages[2] != want[1] {
		return fmt.Errorf("skip message set changed: %v", messages)
	}

	// The `!classification.Append` branch must not write.
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		unary, ok := ifStmt.Cond.(*ast.UnaryExpr)
		if !ok {
			return true
		}
		sel, ok := unary.X.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Append" {
			return true
		}
		ast.Inspect(ifStmt.Body, func(inner ast.Node) bool {
			call, ok := inner.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch rgaS0CallName(call) {
			case "s.WriteArtifact", "s.WritePatch", "s.SaveFeatureStatus", "s.MarkFeatureState":
				writesInSkipBranch++
			}
			return true
		})
		return true
	})
	if writesInSkipBranch != 0 {
		return fmt.Errorf("the same-patch skip branch now performs %d write(s); today it is the zero-side-effect path", writesInSkipBranch)
	}
	return nil
}

// rgaS0CheckFeaturePatchWriteOrder asserts P2's shipped write ordering:
// canonical patch → numbered patch → status → state → recipe autogen →
// generation append. S4 inserts a publication step; this pins the "before".
func rgaS0CheckFeaturePatchWriteOrder(src string) error {
	file, err := rgaS0Parse("feature_patch.go", src)
	if err != nil {
		return err
	}
	fn := rgaS0FuncBody(file, "runFeaturePatchAmend")
	if fn == nil {
		return fmt.Errorf("runFeaturePatchAmend not found")
	}
	watch := map[string]bool{
		"s.WriteArtifact":                 true,
		"s.WritePatch":                    true,
		"s.MarkFeatureState":              true,
		"AutogenRecipeForRecord":          true,
		"AppendPatchGenerationForFeature": true,
	}
	var order []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := rgaS0CallName(call)
		if name == "workflow.AutogenRecipeForRecord" {
			name = "AutogenRecipeForRecord"
		}
		if name == "workflow.AppendPatchGenerationForFeature" {
			name = "AppendPatchGenerationForFeature"
		}
		if watch[name] {
			order = append(order, name)
		}
		return true
	})
	want := []string{
		"s.WriteArtifact",
		"s.WritePatch",
		"s.MarkFeatureState",
		"AutogenRecipeForRecord",
		"AppendPatchGenerationForFeature",
	}
	if strings.Join(order, ">") != strings.Join(want, ">") {
		return fmt.Errorf("P2 write order changed:\n got %v\nwant %v", order, want)
	}
	return nil
}

// TestRGAS0FeaturePatchProducerSourceContract freezes P2.
func TestRGAS0FeaturePatchProducerSourceContract(t *testing.T) {
	src := rgaS0ReadRepoFile(t, "internal/cli/feature_patch.go")
	if err := rgaS0CheckFeaturePatchSkipBranches(src); err != nil {
		t.Fatalf("P2 skip branches changed: %v", err)
	}
	if err := rgaS0CheckFeaturePatchWriteOrder(src); err != nil {
		t.Fatalf("P2 write ordering changed: %v", err)
	}

	t.Run("sensitivity", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			old   string
			new   string
			check func(string) error
		}{
			{
				name:  "skip-branch-gains-a-write",
				old:   "\t\t\tfmt.Fprintln(cmd.ErrOrStderr(), \"no patch byte change; fixup skipped\")",
				new:   "\t\t\tfmt.Fprintln(cmd.ErrOrStderr(), \"no patch byte change; fixup skipped\")\n\t\t\t_ = s.WriteArtifact(slug, \"recipe-coverage.json\", \"{}\")",
				check: rgaS0CheckFeaturePatchSkipBranches,
			},
			{
				name:  "skip-message-text-changes",
				old:   "no patch byte change; refresh skipped",
				new:   "no patch bytes changed; refresh skipped",
				check: rgaS0CheckFeaturePatchSkipBranches,
			},
			{
				name:  "autogen-moves-before-the-state-mark",
				old:   "\tif err := s.MarkFeatureState(slug, store.StateApplied,",
				new:   "\tif _, _, _, agErr := workflow.AutogenRecipeForRecord(s, slug, patch, true, false); agErr != nil {\n\t\t_ = agErr\n\t}\n\tif err := s.MarkFeatureState(slug, store.StateApplied,",
				check: rgaS0CheckFeaturePatchWriteOrder,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if !strings.Contains(src, tc.old) {
					t.Fatalf("mutation anchor no longer present:\n%q", tc.old)
				}
				if err := tc.check(strings.Replace(src, tc.old, tc.new, 1)); err == nil {
					t.Fatalf("guard did not catch mutation %q", tc.name)
				}
			})
		}
	})
}

// ── P4 / P5: the conditional patch writes ────────────────────────────────

// rgaS0CheckConditionalPatchWrite asserts that the named function writes
// the canonical patch only inside a branch conditioned on a non-empty
// captured patch — the "no patch event" row of §6.15's event table.
func rgaS0CheckConditionalPatchWrite(name, relPath, src, fnName string) error {
	file, err := rgaS0Parse(relPath, src)
	if err != nil {
		return err
	}
	fn := rgaS0FuncBody(file, fnName)
	if fn == nil {
		return fmt.Errorf("%s: %s not found", name, fnName)
	}

	guarded := 0
	total := 0
	var walk func(node ast.Node, insideGuard bool)
	walk = func(node ast.Node, insideGuard bool) {
		if node == nil {
			return
		}
		ast.Inspect(node, func(n ast.Node) bool {
			if n == nil {
				return false
			}
			if ifStmt, ok := n.(*ast.IfStmt); ok {
				// The init statement and the condition run in the
				// ENCLOSING context, so they keep `insideGuard`. Only
				// the body inherits this `if`'s own guard status. The
				// init matters: `if err := s.WriteArtifact(...)` puts
				// the bound write inside an `if` header.
				if ifStmt.Init != nil {
					walk(ifStmt.Init, insideGuard)
				}
				walk(ifStmt.Cond, insideGuard)
				walk(ifStmt.Body, insideGuard || rgaS0IsNonEmptyPatchCond(ifStmt.Cond))
				if ifStmt.Else != nil {
					walk(ifStmt.Else, insideGuard)
				}
				return false
			}
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "WriteArtifact" {
				return true
			}
			artifact, isLit := rgaS0StringLit(call.Args[1])
			if !isLit || artifact != "post-apply.patch" {
				return true
			}
			total++
			if insideGuard {
				guarded++
			}
			return true
		})
	}
	walk(fn.Body, false)

	if total != 1 {
		return fmt.Errorf("%s: want exactly one canonical patch write, got %d", name, total)
	}
	if guarded != 1 {
		return fmt.Errorf("%s: the canonical patch write is no longer gated on a non-empty captured patch", name)
	}
	return nil
}

// rgaS0IsNonEmptyPatchCond recognises `patch != ""` in either operand
// order, which is how both P4 and P5 spell the capture gate.
func rgaS0IsNonEmptyPatchCond(cond ast.Expr) bool {
	bin, ok := cond.(*ast.BinaryExpr)
	if !ok || bin.Op.String() != "!=" {
		return false
	}
	hasPatchIdent := false
	hasEmptyString := false
	for _, side := range []ast.Expr{bin.X, bin.Y} {
		if ident, ok := side.(*ast.Ident); ok && strings.Contains(strings.ToLower(ident.Name), "patch") {
			hasPatchIdent = true
		}
		if value, ok := rgaS0StringLit(side); ok && value == "" {
			hasEmptyString = true
		}
	}
	return hasPatchIdent && hasEmptyString
}

// TestRGAS0CycleAndApplyDonePatchGates freezes P4 and P5.
func TestRGAS0CycleAndApplyDonePatchGates(t *testing.T) {
	cycleSrc := rgaS0ReadRepoFile(t, "internal/cli/phase2.go")
	applySrc := rgaS0ReadRepoFile(t, "internal/cli/cobra.go")

	if err := rgaS0CheckConditionalPatchWrite("P4 cycle", "phase2.go", cycleSrc, "cycleCmd"); err != nil {
		t.Fatalf("%v", err)
	}
	if err := rgaS0CheckConditionalPatchWrite("P5 apply --mode done", "cobra.go", applySrc, "runApplyDone"); err != nil {
		t.Fatalf("%v", err)
	}

	t.Run("cycle-discards-its-write-error-today", func(t *testing.T) {
		// PRD §2.7: `cycle` currently drops the WriteArtifact error, so
		// it cannot tell a successful bound write from a failed one.
		// S1/S4 propagate it; S0 records that it does not yet.
		file, err := rgaS0Parse("phase2.go", cycleSrc)
		if err != nil {
			t.Fatal(err)
		}
		fn := rgaS0FuncBody(file, "cycleCmd")
		if fn == nil {
			t.Fatal("cycleCmd not found")
		}
		discarded := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			stmt, ok := n.(*ast.ExprStmt)
			if !ok {
				return true
			}
			call, ok := stmt.X.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "WriteArtifact" {
				return true
			}
			if artifact, isLit := rgaS0StringLit(call.Args[1]); isLit && artifact == "post-apply.patch" {
				discarded = true
			}
			return true
		})
		if !discarded {
			t.Fatal("cycle's canonical patch write no longer appears as a bare expression statement; the S0 record of the discarded error is stale")
		}
	})

	t.Run("sensitivity", func(t *testing.T) {
		cycleAnchor := "\t\t\t\ts.WriteArtifact(slug, \"post-apply.patch\", patch)"
		if !strings.Contains(cycleSrc, cycleAnchor) {
			t.Fatalf("mutation anchor no longer present:\n%q", cycleAnchor)
		}
		// Hoisting the write out of the `patch != ""` gate must be caught.
		mutatedCycle := strings.Replace(cycleSrc,
			"\t\t\tif patch != \"\" {\n"+cycleAnchor,
			cycleAnchor+"\n\t\t\tif patch != \"\" {",
			1)
		if mutatedCycle == cycleSrc {
			t.Fatal("cycle capture-gate mutation anchor no longer present")
		}
		if err := rgaS0CheckConditionalPatchWrite("P4 cycle", "phase2.go", mutatedCycle, "cycleCmd"); err == nil {
			t.Fatal("guard did not catch the ungated cycle patch write")
		}

		applyOld := "\t\tif patch != \"\" {\n" +
			"\t\t\tif err := s.WriteArtifact(slug, \"post-apply.patch\", patch); err != nil {\n" +
			"\t\t\t\treturn \"\", 0, err\n" +
			"\t\t\t}\n"
		if !strings.Contains(applySrc, applyOld) {
			t.Fatalf("apply --mode done mutation anchor no longer present:\n%q", applyOld)
		}
		applyNew := "\t\tif err := s.WriteArtifact(slug, \"post-apply.patch\", patch); err != nil {\n" +
			"\t\t\treturn \"\", 0, err\n" +
			"\t\t}\n" +
			"\t\tif patch != \"\" {\n"
		if err := rgaS0CheckConditionalPatchWrite("P5 apply --mode done", "cobra.go",
			strings.Replace(applySrc, applyOld, applyNew, 1), "runApplyDone"); err == nil {
			t.Fatal("guard did not catch the ungated apply --mode done patch write")
		}
	})
}

// ── P3: RefreshAfterAccept, measured ─────────────────────────────────────

// TestRGAS0RefreshAfterAcceptBaseline freezes P3's shipped shape: the
// canonical patch write is UNCONDITIONAL, the generation append is
// conditional on changed bytes, the recipe is deliberately left alone,
// and nothing publishes coverage.
func TestRGAS0RefreshAfterAcceptBaseline(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "Demo", Slug: "demo", Request: "demo"}); err != nil {
		t.Fatal(err)
	}
	slug := "demo"
	upstream, err := gitutil.HeadCommit(tmpDir)
	if err != nil {
		t.Fatalf("HeadCommit: %v", err)
	}

	// A recipe already exists. P3 must not touch it (refresh.go:20-24).
	staleRecipe := `{"feature":"demo","operations":[{"type":"ensure-directory","path":"src/"}]}` + "\n"
	if err := s.WriteArtifact(slug, "apply-recipe.json", staleRecipe); err != nil {
		t.Fatal(err)
	}
	recipeBefore, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "apply-recipe.json"))
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test\nupdated line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	originalPatch := "diff --git a/README.md b/README.md\n" +
		"--- a/README.md\n+++ b/README.md\n@@ -1 +1,2 @@\n # Test\n+different line\n"
	if err := s.WriteArtifact(slug, "post-apply.patch", originalPatch); err != nil {
		t.Fatal(err)
	}

	if err := RefreshAfterAccept(s, slug, upstream, originalPatch); err != nil {
		t.Fatalf("RefreshAfterAccept: %v", err)
	}

	refreshed, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
	if err != nil {
		t.Fatalf("read refreshed patch: %v", err)
	}
	if !strings.Contains(refreshed, "README.md") || refreshed == originalPatch {
		t.Fatalf("expected a regenerated canonical patch, got:\n%s", refreshed)
	}
	recipeAfter, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "apply-recipe.json"))
	if err != nil {
		t.Fatal(err)
	}
	if recipeAfter != recipeBefore {
		t.Fatalf("P3 must leave the recipe untouched (refresh.go:20-24):\n got %q\nwant %q", recipeAfter, recipeBefore)
	}
	rgaS0AssertNoCoverageArtifact(t, tmpDir, slug)

	manifest, err := store.LoadPatchGenerations(s, slug)
	if err != nil {
		t.Fatalf("LoadPatchGenerations: %v", err)
	}
	if len(manifest.Generations) != 1 {
		t.Fatalf("changed bytes must append exactly one generation, got %+v", manifest.Generations)
	}
	patchesBefore := rgaS0CountPatchSnapshots(t, tmpDir, slug)

	// Second accept over the SAME bytes: the write still happens
	// (unconditional, refresh.go:82) and a numbered snapshot is still
	// taken, but the generation append does not run (refresh.go:93).
	if err := RefreshAfterAccept(s, slug, upstream, refreshed); err != nil {
		t.Fatalf("second RefreshAfterAccept: %v", err)
	}
	again, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
	if err != nil {
		t.Fatal(err)
	}
	if again != refreshed {
		t.Fatalf("a no-byte-change accept must still rewrite identical bytes:\n got %q\nwant %q", again, refreshed)
	}
	manifest2, err := store.LoadPatchGenerations(s, slug)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest2.Generations) != 1 {
		t.Fatalf("unchanged bytes must NOT append a generation, got %+v", manifest2.Generations)
	}
	if got := rgaS0CountPatchSnapshots(t, tmpDir, slug); got != patchesBefore+1 {
		t.Fatalf("numbered snapshot count = %d, want %d (the snapshot write is unconditional)", got, patchesBefore+1)
	}
	rgaS0AssertNoCoverageArtifact(t, tmpDir, slug)
}

func rgaS0CountPatchSnapshots(t *testing.T, root, slug string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, ".tpatch", "features", slug, "patches"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("readdir patches: %v", err)
	}
	count := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".patch") {
			count++
		}
	}
	return count
}

// ── P6: implement, measured ──────────────────────────────────────────────

// rgaS0ScriptedProvider returns a fixed response for every call.
type rgaS0ScriptedProvider struct {
	response string
	calls    int
}

func (p *rgaS0ScriptedProvider) Check(ctx context.Context, cfg provider.Config) (*provider.Health, error) {
	return &provider.Health{}, nil
}

func (p *rgaS0ScriptedProvider) Generate(ctx context.Context, cfg provider.Config, req provider.GenerateRequest) (string, error) {
	p.calls++
	return p.response, nil
}

// TestRGAS0ImplementProducerBaseline freezes P6's two REACHABLE arms:
// a validated provider recipe and the heuristic fallback. Both take the
// valid-JSON arm, write the reserialized recipe, write provenance, advance
// state — and publish no coverage.
func TestRGAS0ImplementProducerBaseline(t *testing.T) {
	cases := []struct {
		name       string
		provider   provider.Provider
		cfg        provider.Config
		wantOpPath string
	}{
		{
			name:       "provider-supplied-recipe",
			provider:   &rgaS0ScriptedProvider{response: `{"feature":"demo","operations":[{"type":"ensure-directory","path":"src/"}]}`},
			cfg:        provider.Config{Type: "openai-compatible", BaseURL: "http://x", Model: "m", AuthEnv: "TPATCH_TEST_KEY"},
			wantOpPath: "src/",
		},
		{
			name:     "heuristic-fallback",
			provider: nil,
			cfg:      provider.Config{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TPATCH_TEST_KEY", "stub")
			tmpDir := t.TempDir()
			setupGitRepo(t, tmpDir)
			s, err := store.Init(tmpDir)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.AddFeature(store.AddFeatureInput{Title: "Demo", Slug: "demo", Request: "demo request"}); err != nil {
				t.Fatal(err)
			}
			slug := "demo"

			var warn strings.Builder
			prev := WarnWriter
			WarnWriter = &warn
			defer func() { WarnWriter = prev }()

			if err := RunImplement(context.Background(), s, slug, tc.provider, tc.cfg); err != nil {
				t.Fatalf("RunImplement: %v (warnings=%q)", err, warn.String())
			}

			raw, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "apply-recipe.json"))
			if err != nil {
				t.Fatalf("recipe missing: %v", err)
			}
			var recipe ApplyRecipe
			if uerr := json.Unmarshal([]byte(raw), &recipe); uerr != nil {
				t.Fatalf("the valid-JSON arm must write decodable bytes; got %q (%v)", raw, uerr)
			}
			if recipe.Feature != slug {
				t.Errorf("recipe.feature = %q, want %q", recipe.Feature, slug)
			}
			if tc.wantOpPath != "" {
				if len(recipe.Operations) != 1 || recipe.Operations[0].Path != tc.wantOpPath {
					t.Errorf("operations = %+v, want a single op on %q", recipe.Operations, tc.wantOpPath)
				}
			}
			// Reserialized, not raw: the valid arm indents and appends
			// a trailing newline.
			if !strings.HasSuffix(raw, "\n") || !strings.Contains(raw, "\n  \"operations\"") {
				t.Errorf("valid arm must write reserialized (indented) bytes, got %q", raw)
			}

			if _, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "recipe-provenance.json")); err != nil {
				t.Errorf("P6 writes recipe-provenance.json today: %v", err)
			}
			st, err := s.LoadFeatureStatus(slug)
			if err != nil {
				t.Fatal(err)
			}
			if st.State != store.StateImplementing {
				t.Errorf("state = %q, want %q", st.State, store.StateImplementing)
			}
			// The ordinary P6 run has no canonical patch yet — the exact
			// situation §6.15 says must become explicit incomplete
			// coverage. Today it is silent absence.
			if _, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch")); err == nil {
				t.Error("fixture precondition: implement must not have produced a canonical patch")
			}
			rgaS0AssertNoCoverageArtifact(t, tmpDir, slug)
		})
	}
}

// TestRGAS0ImplementManualCheckpointBaseline freezes P6's category-(b)
// event: `implement --manual` does NOT author the recipe. It validates
// bytes an agent or human already wrote and advances state — a checkpoint,
// not a bound write. §6.15 makes it publish coverage in S4; today it
// publishes nothing and leaves the artifact byte-identical.
func TestRGAS0ImplementManualCheckpointBaseline(t *testing.T) {
	newFixture := func(t *testing.T, recipeBody string) (*store.Store, string) {
		t.Helper()
		tmpDir := t.TempDir()
		setupGitRepo(t, tmpDir)
		s, err := store.Init(tmpDir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddFeature(store.AddFeatureInput{Title: "Demo", Slug: "demo", Request: "demo"}); err != nil {
			t.Fatal(err)
		}
		if recipeBody != "" {
			if err := s.WriteArtifact("demo", "apply-recipe.json", recipeBody); err != nil {
				t.Fatal(err)
			}
		}
		return s, tmpDir
	}

	t.Run("valid-checkpoint-advances-state-without-writing", func(t *testing.T) {
		body := `{"feature":"demo","operations":[{"type":"ensure-directory","path":"src/"}]}` + "\n"
		s, root := newFixture(t, body)

		if err := s.AdvanceStateManually("demo", "implement"); err != nil {
			t.Fatalf("AdvanceStateManually: %v", err)
		}
		after, err := s.ReadFeatureFile("demo", filepath.Join("artifacts", "apply-recipe.json"))
		if err != nil {
			t.Fatal(err)
		}
		if after != body {
			t.Fatalf("a checkpoint must not rewrite the artifact:\n got %q\nwant %q", after, body)
		}
		st, err := s.LoadFeatureStatus("demo")
		if err != nil {
			t.Fatal(err)
		}
		if st.State != store.StateImplementing {
			t.Errorf("state = %q, want %q", st.State, store.StateImplementing)
		}
		// The manual path publishes no provenance — the exact producer
		// gap the downstream V10 case study §4 names.
		if _, err := s.ReadFeatureFile("demo", filepath.Join("artifacts", "recipe-provenance.json")); err == nil {
			t.Error("implement --manual must not publish provenance today (case study §4 Path B)")
		}
		rgaS0AssertNoCoverageArtifact(t, root, "demo")
	})

	t.Run("refusals-do-not-advance-state", func(t *testing.T) {
		for _, tc := range []struct{ name, body string }{
			{"missing-artifact", ""},
			{"empty-artifact", "   \n"},
			{"invalid-json", "{not json"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				s, root := newFixture(t, tc.body)
				if err := s.AdvanceStateManually("demo", "implement"); err == nil {
					t.Fatal("expected a refusal")
				}
				st, err := s.LoadFeatureStatus("demo")
				if err != nil {
					t.Fatal(err)
				}
				if st.State == store.StateImplementing {
					t.Fatalf("a refused checkpoint must not advance state; got %q", st.State)
				}
				rgaS0AssertNoCoverageArtifact(t, root, "demo")
			})
		}
	})
}

// TestRGAS0ImplementRawArmIsCurrentlyUnreachable records an S0 finding the
// PRD's P6 analysis does not state: the unmarshal-failure arm
// (`internal/workflow/implement.go:192-195`) cannot be reached through
// `RunImplement` today, because the retry validator decodes into the SAME
// `ApplyRecipe` target through the SAME extractor. A response that would
// fail the arm's parse fails validation first and is replaced by the
// heuristic recipe.
//
// The arm's write behaviour is therefore frozen structurally
// (TestRGAS0ImplementParseArmsSourceContract) and its reachability is
// frozen here as an equivalence, so S1/S4 cannot quietly make the arm
// reachable — or quietly delete it — without a visible test change.
func TestRGAS0ImplementRawArmIsCurrentlyUnreachable(t *testing.T) {
	responses := []string{
		`{"feature":"demo","operations":[{"type":"ensure-directory","path":"src/"}]}`,
		`{"feature":"demo","operations":"not-an-array"}`,
		`{"feature":"demo"}`,
		"prose with no JSON at all",
		"```json\n{\"feature\":\"demo\",\"operations\":[]}\n```",
		`{"feature":"demo","operations":[{"type":42}]}`,
	}
	for _, resp := range responses {
		var validatorTarget ApplyRecipe
		validatorErr := JSONObjectValidator(&validatorTarget)(resp)

		var armTarget ApplyRecipe
		armErr := json.Unmarshal([]byte(mustExtractJSON(resp)), &armTarget)

		if (validatorErr == nil) != (armErr == nil) {
			t.Fatalf("validator and arm disagree for %q: validator=%v arm=%v\n"+
				"the raw-invalid arm just became reachable (or unreachable) — update the S0 record deliberately",
				resp, validatorErr, armErr)
		}
	}

	// Sensitivity: the equivalence is a real measurement, so at least one
	// corpus entry must be rejected by both and one accepted by both.
	acceptedAll := true
	rejectedAll := true
	for _, resp := range responses {
		var target ApplyRecipe
		if JSONObjectValidator(&target)(resp) == nil {
			rejectedAll = false
		} else {
			acceptedAll = false
		}
	}
	if acceptedAll || rejectedAll {
		t.Fatal("the reachability corpus must contain both accepted and rejected responses")
	}

	// And the heuristic fallback — the value that actually reaches the
	// parse when validation fails — always takes the VALID arm.
	var heuristicTarget ApplyRecipe
	if err := json.Unmarshal([]byte(mustExtractJSON(heuristicRecipe("demo"))), &heuristicTarget); err != nil {
		t.Fatalf("the heuristic fallback must decode, else the raw arm would be reachable: %v", err)
	}
}
