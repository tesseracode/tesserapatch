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
	"go/token"
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
		// The mutations below are anchored on the GATE and on the WRITE
		// independently, never on the two being adjacent. S1 inserted
		// each producer's immutable observation between them (ADR-036
		// D2), and a mutation fixture that assumed adjacency would break
		// on that insertion while proving nothing about the gate.
		cycleGate := "\t\t\tif patch != \"\" {"
		cycleAnchor := "\t\t\t\ts.WriteArtifact(slug, \"post-apply.patch\", patch)"
		if !strings.Contains(cycleSrc, cycleGate) || !strings.Contains(cycleSrc, cycleAnchor) {
			t.Fatalf("cycle mutation anchors no longer present:\n%q\n%q", cycleGate, cycleAnchor)
		}
		// Hoisting the write out of the `patch != ""` gate must be caught.
		hoisted := strings.Replace(cycleSrc, "\n"+cycleAnchor, "", 1)
		hoisted = strings.Replace(hoisted, cycleGate, cycleAnchor+"\n"+cycleGate, 1)
		if hoisted == cycleSrc {
			t.Fatal("cycle capture-gate mutation produced no change")
		}
		if err := rgaS0CheckConditionalPatchWrite("P4 cycle", "phase2.go", hoisted, "cycleCmd"); err == nil {
			t.Fatal("guard did not catch the ungated cycle patch write")
		}
		// So must widening the gate to a condition that is not "the
		// capture produced bytes".
		widened := strings.Replace(cycleSrc, cycleGate, "\t\t\tif interactive {", 1)
		if err := rgaS0CheckConditionalPatchWrite("P4 cycle", "phase2.go", widened, "cycleCmd"); err == nil {
			t.Fatal("guard did not catch the widened cycle capture gate")
		}

		applyGate := "\t\tif patch != \"\" {"
		applyAnchor := "\t\t\tif err := s.WriteArtifact(slug, \"post-apply.patch\", patch); err != nil {\n" +
			"\t\t\t\treturn \"\", 0, err\n" +
			"\t\t\t}"
		if !strings.Contains(applySrc, applyGate) || !strings.Contains(applySrc, applyAnchor) {
			t.Fatalf("apply --mode done mutation anchors no longer present:\n%q\n%q", applyGate, applyAnchor)
		}
		applyHoisted := strings.Replace(applySrc, "\n"+applyAnchor, "", 1)
		applyHoisted = strings.Replace(applyHoisted, applyGate,
			"\t\tif err := s.WriteArtifact(slug, \"post-apply.patch\", patch); err != nil {\n"+
				"\t\t\treturn \"\", 0, err\n"+
				"\t\t}\n"+applyGate, 1)
		if applyHoisted == applySrc {
			t.Fatal("apply --mode done capture-gate mutation produced no change")
		}
		if err := rgaS0CheckConditionalPatchWrite("P5 apply --mode done", "cobra.go",
			applyHoisted, "runApplyDone"); err == nil {
			t.Fatal("guard did not catch the ungated apply --mode done patch write")
		}
		applyWidened := strings.Replace(applySrc, applyGate, "\t\tif checkpoint {", 1)
		if err := rgaS0CheckConditionalPatchWrite("P5 apply --mode done", "cobra.go",
			applyWidened, "runApplyDone"); err == nil {
			t.Fatal("guard did not catch the widened apply --mode done capture gate")
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

// rgaS0ImplementValidatorExpr reports the expression RunImplement
// actually passes as `RetryOptions.Validate`, plus the declared type of
// the variable that expression takes the address of.
//
// The S0 revision of the reachability row below re-implemented the
// validator inline (`JSONObjectValidator(&validatorTarget)` with a
// locally declared `ApplyRecipe`). A review carry-forward flagged that as
// an assumption rather than a measurement: if RunImplement ever passed a
// DIFFERENT validator, or decoded into a different target type, the
// equivalence would keep passing while describing a validator nobody
// runs. This binds the row to the shipped call.
func rgaS0ImplementValidatorExpr(src string) (callee, targetVar, targetType string, err error) {
	file, perr := rgaS0Parse("implement.go", src)
	if perr != nil {
		return "", "", "", perr
	}
	fn := rgaS0FuncBody(file, "RunImplement")
	if fn == nil {
		return "", "", "", fmt.Errorf("RunImplement not found")
	}

	var validate ast.Expr
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		ident, ok := lit.Type.(*ast.Ident)
		if !ok || ident.Name != "RetryOptions" {
			return true
		}
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Validate" {
				validate = kv.Value
			}
		}
		return true
	})
	if validate == nil {
		return "", "", "", fmt.Errorf("RunImplement no longer passes RetryOptions.Validate")
	}
	call, ok := validate.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return "", "", "", fmt.Errorf("RetryOptions.Validate is no longer a one-argument validator call")
	}
	calleeIdent, ok := call.Fun.(*ast.Ident)
	if !ok {
		return "", "", "", fmt.Errorf("RetryOptions.Validate's callee is not a package-level function")
	}
	unary, ok := call.Args[0].(*ast.UnaryExpr)
	if !ok || unary.Op != token.AND {
		return "", "", "", fmt.Errorf("the validator no longer decodes into an addressable target")
	}
	targetIdent, ok := unary.X.(*ast.Ident)
	if !ok {
		return "", "", "", fmt.Errorf("the validator target is not a simple variable")
	}

	declared := ""
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		decl, ok := n.(*ast.DeclStmt)
		if !ok {
			return true
		}
		gen, ok := decl.Decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			return true
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range vs.Names {
				if name.Name != targetIdent.Name {
					continue
				}
				if t, ok := vs.Type.(*ast.Ident); ok {
					declared = t.Name
				}
			}
		}
		return true
	})
	if declared == "" {
		return "", "", "", fmt.Errorf("the validator target %q has no local `var x T` declaration to read a type from", targetIdent.Name)
	}
	return calleeIdent.Name, targetIdent.Name, declared, nil
}

// rgaS0ImplementArmDecodeExpr reports how the RAW-INVALID ARM itself
// decodes: which extractor feeds `json.Unmarshal`, and the declared type
// of the value it decodes into.
//
// The validator half above measures what production validates. This half
// measures what the arm parses. The reachability equivalence is only a
// real measurement when BOTH sides are read out of the shipped source: a
// validator that decodes an `ApplyRecipe` through `mustExtractJSON` and
// an arm that decoded something else through a different extractor could
// disagree at runtime while the equivalence kept passing.
func rgaS0ImplementArmDecodeExpr(src string) (extractor, targetType string, err error) {
	file, perr := rgaS0Parse("implement.go", src)
	if perr != nil {
		return "", "", perr
	}
	fn := rgaS0FuncBody(file, "RunImplement")
	if fn == nil {
		return "", "", fmt.Errorf("RunImplement not found")
	}

	var unmarshal *ast.CallExpr
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if unmarshal != nil {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok || rgaS0CallName(call) != "json.Unmarshal" || len(call.Args) != 2 {
			return true
		}
		unmarshal = call
		return false
	})
	if unmarshal == nil {
		return "", "", fmt.Errorf("RunImplement no longer decodes the recipe with json.Unmarshal")
	}

	conversion, ok := unmarshal.Args[0].(*ast.CallExpr)
	if !ok || len(conversion.Args) != 1 {
		return "", "", fmt.Errorf("the arm no longer decodes a converted extractor result")
	}
	inner, ok := conversion.Args[0].(*ast.CallExpr)
	if !ok {
		return "", "", fmt.Errorf("the arm no longer feeds json.Unmarshal from an extractor call")
	}
	extractor = rgaS0CallName(inner)

	unary, ok := unmarshal.Args[1].(*ast.UnaryExpr)
	if !ok || unary.Op != token.AND {
		return "", "", fmt.Errorf("the arm no longer decodes into an addressable target")
	}
	targetIdent, ok := unary.X.(*ast.Ident)
	if !ok {
		return "", "", fmt.Errorf("the arm's decode target is not a simple variable")
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		decl, ok := n.(*ast.DeclStmt)
		if !ok {
			return true
		}
		gen, ok := decl.Decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			return true
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range vs.Names {
				if name.Name != targetIdent.Name {
					continue
				}
				if t, ok := vs.Type.(*ast.Ident); ok {
					targetType = t.Name
				}
			}
		}
		return true
	})
	if targetType == "" {
		return "", "", fmt.Errorf("the arm's decode target %q has no local `var x T` declaration to read a type from", targetIdent.Name)
	}
	return extractor, targetType, nil
}

// TestRGAS0ImplementRawArmIsCurrentlyUnreachable records an S0 finding the
// PRD's P6 analysis does not state: the unmarshal-failure arm
// (`internal/workflow/implement.go`) cannot be reached through
// `RunImplement` today, because the retry validator decodes into the SAME
// `ApplyRecipe` target through the SAME extractor. A response that would
// fail the arm's parse fails validation first and is replaced by the
// heuristic recipe.
//
// The arm's write behaviour is frozen structurally
// (TestRGAS0ImplementParseArmsSourceContract) and its reachability is
// frozen here as an equivalence, so a later slice cannot quietly make the
// arm reachable — or quietly delete it — without a visible test change.
//
// The equivalence is BOUND to the shipped call: the validator expression
// is read out of RunImplement's own `RetryOptions` literal, so this row
// measures the validator production actually runs.
func TestRGAS0ImplementRawArmIsCurrentlyUnreachable(t *testing.T) {
	callee, targetVar, targetType, err := rgaS0ImplementValidatorExpr(
		rgaS0ReadRepoFile(t, "internal/workflow/implement.go"))
	if err != nil {
		t.Fatalf("RunImplement's validator expression changed: %v", err)
	}
	if callee != "JSONObjectValidator" {
		t.Fatalf("RunImplement now validates with %s(...), not JSONObjectValidator; the equivalence below no longer describes production", callee)
	}
	if targetType != "ApplyRecipe" {
		t.Fatalf("RunImplement's validator target %s is declared %s, not ApplyRecipe; the raw arm and the validator no longer share a decode target", targetVar, targetType)
	}

	// The other half of "the same decode path": the arm's own extractor
	// and decode target, read from the shipped source rather than assumed
	// to match the corpus below.
	armExtractor, armTarget, err := rgaS0ImplementArmDecodeExpr(
		rgaS0ReadRepoFile(t, "internal/workflow/implement.go"))
	if err != nil {
		t.Fatalf("the raw-invalid arm's decode expression changed: %v", err)
	}
	if armExtractor != "mustExtractJSON" {
		t.Fatalf("the arm now extracts with %s(...), not mustExtractJSON; the equivalence below no longer describes production", armExtractor)
	}
	if armTarget != "ApplyRecipe" {
		t.Fatalf("the arm decodes into %s, not ApplyRecipe; it no longer shares the validator's decode target", armTarget)
	}

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

	t.Run("sensitivity", func(t *testing.T) {
		src := rgaS0ReadRepoFile(t, "internal/workflow/implement.go")
		for _, tc := range []struct{ name, old, new string }{
			{
				name: "validator-stops-being-JSONObjectValidator",
				old:  "Validate:   JSONObjectValidator(&tmp),",
				new:  "Validate:   AlwaysValid(&tmp),",
			},
			{
				name: "validator-target-changes-type",
				old:  "\t\tvar tmp ApplyRecipe",
				new:  "\t\tvar tmp RecipeProvenance",
			},
			{
				name: "validator-field-disappears",
				old:  "Validate:   JSONObjectValidator(&tmp),",
				new:  "LogPrefix:  \"implement-2\",",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if !strings.Contains(src, tc.old) {
					t.Fatalf("mutation anchor no longer present:\n%q", tc.old)
				}
				mutated := strings.Replace(src, tc.old, tc.new, 1)
				callee, _, targetType, err := rgaS0ImplementValidatorExpr(mutated)
				if err == nil && callee == "JSONObjectValidator" && targetType == "ApplyRecipe" {
					t.Fatalf("guard did not catch mutation %q", tc.name)
				}
			})
		}

		// The arm half of the same equivalence: swapping the extractor or
		// the decode target must be caught too.
		for _, tc := range []struct{ name, old, new string }{
			{
				name: "arm-stops-using-the-shared-extractor",
				old:  "json.Unmarshal([]byte(mustExtractJSON(recipeContent)), &recipe)",
				new:  "json.Unmarshal([]byte(extractJSON(recipeContent)), &recipe)",
			},
			{
				name: "arm-decodes-into-another-type",
				old:  "\tvar recipe ApplyRecipe\n\tif err := json.Unmarshal([]byte(mustExtractJSON(recipeContent)), &recipe);",
				new:  "\tvar recipe RecipeProvenance\n\tif err := json.Unmarshal([]byte(mustExtractJSON(recipeContent)), &recipe);",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if !strings.Contains(src, tc.old) {
					t.Fatalf("mutation anchor no longer present:\n%q", tc.old)
				}
				extractor, target, err := rgaS0ImplementArmDecodeExpr(strings.Replace(src, tc.old, tc.new, 1))
				if err == nil && extractor == "mustExtractJSON" && target == "ApplyRecipe" {
					t.Fatalf("guard did not catch mutation %q", tc.name)
				}
			})
		}
	})
}
