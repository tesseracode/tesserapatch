package cli

// GH #15 / ADR-036 slice S0 — frozen evidence for the shipped CLI
// contracts (PRD-recipe-generation-authority §8 "S0 - Frozen evidence").
//
// S0 must "preserve current record/apply/verify output goldens, including
// the exact `Recipe executed: %d/%d operations succeeded` line, the
// current `Recipe generated: artifacts/apply-recipe.json (%d ops)` line,
// the legacy `no recipe found — run 'tpatch implement <slug>' first`
// error with its exit `1`".
//
// Everything here drives the real cobra root through the package's
// existing `runCmd` / `runCmdExit` / `runRecord` helpers and the existing
// `modesFixture` / `gitInitTestRepo` fixtures — no competing runner is
// introduced. Static claims are expressed as pure matchers so a mutation
// subtest can prove each one would fail on the regression it guards.

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

// ── shared scaffolding ───────────────────────────────────────────────────

func rgaS0CLIRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the package directory")
		}
		dir = parent
	}
}

func rgaS0CLISource(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(rgaS0CLIRepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(body)
}

func rgaS0CLIFunc(name, src, fnName string) (*ast.FuncDecl, error) {
	file, err := parser.ParseFile(token.NewFileSet(), name, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name == fnName {
			return fn, nil
		}
	}
	return nil, fmt.Errorf("%s not found in %s", fnName, name)
}

func rgaS0CLIStringLit(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}

func rgaS0CLICallName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		if x, ok := fn.X.(*ast.Ident); ok {
			return x.Name + "." + fn.Sel.Name
		}
		return "." + fn.Sel.Name
	}
	return ""
}

// rgaS0AssertNoCLICoverageArtifact fails when a CLI producer left a
// coverage record on disk. Nothing writes one before S3/S4.
func rgaS0AssertNoCLICoverageArtifact(t *testing.T, root, slug string) {
	t.Helper()
	path := filepath.Join(root, ".tpatch", "features", slug, "artifacts", "recipe-coverage.json")
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("a coverage artifact already exists at %s; the S0 baseline is stale", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

// ── `Recipe generated: ... (%d ops)` ─────────────────────────────────────

// TestRGAS0RecordRecipeGeneratedLine freezes the exact `record` line and
// the arithmetic behind its number, including the `- len(skippedPaths)`
// term that a deleted file exercises.
func TestRGAS0RecordRecipeGeneratedLine(t *testing.T) {
	t.Run("two-new-files-print-two-ops", func(t *testing.T) {
		tmp := modesFixture(t, "s0-gen-two")
		rgaS0CommitAll(t, tmp)
		modesWriteFile(t, tmp, "src/one.txt", "one\n")
		modesWriteFile(t, tmp, "src/two.txt", "two\n")

		stdout, stderr, code := runRecord(t, "record", "--path", tmp, "s0-gen-two", "--lenient")
		if code != 0 {
			t.Fatalf("record failed: %s", stderr)
		}
		const want = "  Recipe generated: artifacts/apply-recipe.json (2 ops)\n"
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing the frozen line %q:\nstdout:\n%s\npatch:\n%s",
				want, stdout, readRecordedPatch(t, tmp, "s0-gen-two"))
		}
		if strings.Contains(stderr, "recipe autogen skipped:") {
			t.Fatalf("no path should be skipped for this fixture:\n%s", stderr)
		}
		recipe := rgaS0ReadRecipe(t, tmp, "s0-gen-two")
		if len(recipe.Operations) != 2 {
			t.Fatalf("derived recipe operations = %d, want 2: %+v", len(recipe.Operations), recipe.Operations)
		}
		rgaS0AssertNoCLICoverageArtifact(t, tmp, "s0-gen-two")
	})

	t.Run("a-deleted-file-is-subtracted-and-reported", func(t *testing.T) {
		tmp := modesFixture(t, "s0-gen-del")
		rgaS0CommitAll(t, tmp)
		modesWriteFile(t, tmp, "src/one.txt", "one\n")
		// README.md is committed by gitInitTestRepo; deleting it makes
		// the capture carry a deletion the recipe schema cannot express.
		if err := os.Remove(filepath.Join(tmp, "README.md")); err != nil {
			t.Fatal(err)
		}

		stdout, stderr, code := runRecord(t, "record", "--path", tmp, "s0-gen-del", "--lenient")
		if code != 0 {
			t.Fatalf("record failed: %s", stderr)
		}
		const wantLine = "  Recipe generated: artifacts/apply-recipe.json (1 ops)\n"
		if !strings.Contains(stdout, wantLine) {
			t.Fatalf("stdout missing %q:\nstdout:\n%s\npatch:\n%s",
				wantLine, stdout, readRecordedPatch(t, tmp, "s0-gen-del"))
		}
		const wantSkip = "  recipe autogen skipped: README.md (deleted — recipe schema has no delete-file op)\n"
		if !strings.Contains(stderr, wantSkip) {
			t.Fatalf("stderr missing the frozen skip line %q:\n%s", wantSkip, stderr)
		}
		recipe := rgaS0ReadRecipe(t, tmp, "s0-gen-del")
		if len(recipe.Operations) != 1 || recipe.Operations[0].Path != "src/one.txt" {
			t.Fatalf("derived recipe = %+v, want one op on src/one.txt", recipe.Operations)
		}
		// The captured patch really did carry two file records, so the
		// `- len(skippedPaths)` term is genuinely load-bearing here.
		if got := countPatchFiles(readRecordedPatch(t, tmp, "s0-gen-del")); got != 2 {
			t.Fatalf("captured patch file count = %d, want 2 (one create + one delete)", got)
		}
		rgaS0AssertNoCLICoverageArtifact(t, tmp, "s0-gen-del")
	})
}

func rgaS0ReadRecipe(t *testing.T, root, slug string) workflow.ApplyRecipe {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".tpatch", "features", slug, "artifacts", "apply-recipe.json"))
	if err != nil {
		t.Fatalf("read apply-recipe.json: %v", err)
	}
	var recipe workflow.ApplyRecipe
	if err := json.Unmarshal(raw, &recipe); err != nil {
		t.Fatalf("apply-recipe.json is not decodable: %v\n%s", err, raw)
	}
	return recipe
}

// rgaS0CommitAll commits everything currently in the worktree so a later
// capture contains exactly the changes the test makes. `tpatch init`
// creates or appends a root `.gitignore` (PRD-active-feature-session §4
// D6), which would otherwise land in the captured patch and make the
// frozen operation counts environment-dependent.
func rgaS0CommitAll(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"add", "-A"},
		{"-c", "commit.gpgsign=false", "commit", "-q", "-m", "s0 baseline"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
}

// ── PI-10: the printed number is a FILE count, not an operation count ────

// TestRGAS0RecipeGeneratedCountIsAFileCount freezes §2.5.1: the number in
// the `Recipe generated` line comes from `countPatchFiles`, a `diff --git`
// prefix counter that never reads the derived recipe. The two answers
// already disagree today for shapes the counter mis-reads, which is the
// concrete defect §6.1.3 migrates in S1.
func TestRGAS0RecipeGeneratedCountIsAFileCount(t *testing.T) {
	root := t.TempDir()

	cases := []struct {
		name        string
		patch       string
		wantCounted int
		wantOps     int
		wantSkipped int
	}{
		{
			// A truncated header: `strings.Fields` yields three fields,
			// so the lenient parser assigns no path and emits no
			// operation — but the display counter still counts the line.
			name:        "truncated-header-counted-but-not-derived",
			patch:       "diff --git a/x.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n",
			wantCounted: 1,
			wantOps:     0,
			wantSkipped: 0,
		},
		{
			// The counter matches on `diff --git` with NO trailing
			// space, so a token that is not a header at all is counted.
			name:        "non-header-token-counted",
			patch:       "diff --gitignore-notes\n",
			wantCounted: 1,
			wantOps:     0,
			wantSkipped: 0,
		},
		{
			// Two records for one path: the counter counts records, the
			// derivation de-duplicates paths.
			name: "duplicate-path-counted-twice",
			patch: "diff --git a/dup.txt b/dup.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n" +
				"diff --git a/dup.txt b/dup.txt\nindex 2..3 100644\n@@ -1 +1 @@\n-b\n+c\n",
			wantCounted: 2,
			wantOps:     1,
			wantSkipped: 0,
		},
	}

	if err := os.WriteFile(filepath.Join(root, "dup.txt"), []byte("c\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := countPatchFiles(tc.patch); got != tc.wantCounted {
				t.Fatalf("countPatchFiles = %d, want %d", got, tc.wantCounted)
			}
			recipe, skipped, err := workflow.RecipeFromPatch(root, "s0", tc.patch)
			if err != nil {
				t.Fatalf("RecipeFromPatch: %v", err)
			}
			if len(recipe.Operations) != tc.wantOps {
				t.Fatalf("derived operations = %d, want %d: %+v", len(recipe.Operations), tc.wantOps, recipe.Operations)
			}
			if len(skipped) != tc.wantSkipped {
				t.Fatalf("skipped = %d, want %d: %v", len(skipped), tc.wantSkipped, skipped)
			}
			printed := countPatchFiles(tc.patch) - len(skipped)
			if printed == len(recipe.Operations) {
				t.Fatalf("this row exists because the printed count (%d) currently DIFFERS from the operation count (%d); the PI-10 defect is gone",
					printed, len(recipe.Operations))
			}
		})
	}
}

// rgaS0CheckRecipeGeneratedCountPath asserts that the shipped line is
// printed in the `AutogenGenerated` arm from `countPatchFiles(patch) -
// len(skippedPaths)` — the exact expression §6.1.3 replaces in S1.
func rgaS0CheckRecipeGeneratedCountPath(src string) error {
	fn, err := rgaS0CLIFunc("cobra.go", src, "recordCmd")
	if err != nil {
		return err
	}

	const wantFormat = "  Recipe generated: artifacts/apply-recipe.json (%d ops)\n"
	var clause *ast.CaseClause
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		cc, ok := n.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, expr := range cc.List {
			sel, ok := expr.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "AutogenGenerated" {
				continue
			}
			clause = cc
			return false
		}
		return true
	})
	if clause == nil {
		return fmt.Errorf("the `case workflow.AutogenGenerated` arm is gone from recordCmd")
	}

	var arg ast.Expr
	prints := 0
	for _, stmt := range clause.Body {
		exprStmt, ok := stmt.(*ast.ExprStmt)
		if !ok {
			continue
		}
		call, ok := exprStmt.X.(*ast.CallExpr)
		if !ok || rgaS0CLICallName(call) != "fmt.Fprintf" || len(call.Args) != 3 {
			continue
		}
		format, isLit := rgaS0CLIStringLit(call.Args[1])
		if !isLit {
			continue
		}
		if format != wantFormat {
			return fmt.Errorf("the generated-recipe line changed:\n got %q\nwant %q", format, wantFormat)
		}
		prints++
		arg = call.Args[2]
	}
	if prints != 1 {
		return fmt.Errorf("want exactly one `Recipe generated` print in the AutogenGenerated arm, got %d", prints)
	}

	bin, ok := arg.(*ast.BinaryExpr)
	if !ok || bin.Op != token.SUB {
		return fmt.Errorf("the operation count is no longer `countPatchFiles(patch) - len(skippedPaths)`")
	}
	left, ok := bin.X.(*ast.CallExpr)
	if !ok || rgaS0CLICallName(left) != "countPatchFiles" {
		return fmt.Errorf("the minuend is no longer countPatchFiles(...)")
	}
	right, ok := bin.Y.(*ast.CallExpr)
	if !ok || rgaS0CLICallName(right) != "len" || len(right.Args) != 1 {
		return fmt.Errorf("the subtrahend is no longer len(...)")
	}
	ident, ok := right.Args[0].(*ast.Ident)
	if !ok || ident.Name != "skippedPaths" {
		return fmt.Errorf("the subtrahend no longer counts skippedPaths")
	}
	return nil
}

// TestRGAS0RecipeGeneratedCountPathSource pins the source expression that
// §6.1.3 migrates, so the migration cannot happen by accident.
func TestRGAS0RecipeGeneratedCountPathSource(t *testing.T) {
	src := rgaS0CLISource(t, "internal/cli/cobra.go")
	if err := rgaS0CheckRecipeGeneratedCountPath(src); err != nil {
		t.Fatalf("the Recipe-generated count path changed: %v", err)
	}

	t.Run("sensitivity", func(t *testing.T) {
		for _, tc := range []struct{ name, old, new string }{
			{
				name: "count-migrates-to-the-derived-recipe",
				old:  "countPatchFiles(patch)-len(skippedPaths)",
				new:  "len(derived.Operations)",
			},
			{
				name: "the-line-text-changes",
				old:  "  Recipe generated: artifacts/apply-recipe.json (%d ops)\\n",
				new:  "  Recipe generated: artifacts/apply-recipe.json (%d operations)\\n",
			},
			{
				name: "the-skipped-term-is-dropped",
				old:  "countPatchFiles(patch)-len(skippedPaths)",
				new:  "countPatchFiles(patch)-0",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if !strings.Contains(src, tc.old) {
					t.Fatalf("mutation anchor no longer present:\n%q", tc.old)
				}
				if err := rgaS0CheckRecipeGeneratedCountPath(strings.Replace(src, tc.old, tc.new, 1)); err == nil {
					t.Fatalf("guard did not catch mutation %q", tc.name)
				}
			})
		}
	})
}

// ── `Recipe executed: %d/%d operations succeeded` ────────────────────────

// TestRGAS0ApplyExecuteSuccessLine freezes the exact success line for a
// one-operation and a two-operation recipe.
func TestRGAS0ApplyExecuteSuccessLine(t *testing.T) {
	cases := []struct {
		name string
		ops  string
		want string
	}{
		{
			name: "one-operation",
			ops:  `{"type": "write-file", "path": "hello.txt", "content": "hi\n"}`,
			want: "Recipe executed: 1/1 operations succeeded\n",
		},
		{
			name: "two-operations",
			ops: `{"type": "write-file", "path": "hello.txt", "content": "hi\n"},` +
				`{"type": "ensure-directory", "path": "src/"}`,
			want: "Recipe executed: 2/2 operations succeeded\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			gitInitTestRepo(t, tmp)
			runCmd("init", "--path", tmp)
			slug := "s0-exec-" + tc.name
			runCmd("add", "--path", tmp, "--slug", slug, "S0 execute line")

			artDir := filepath.Join(tmp, ".tpatch", "features", slug, "artifacts")
			if err := os.MkdirAll(artDir, 0o755); err != nil {
				t.Fatal(err)
			}
			recipe := "{\n  \"feature\": \"" + slug + "\",\n  \"operations\": [" + tc.ops + "]\n}\n"
			if err := os.WriteFile(filepath.Join(artDir, "apply-recipe.json"), []byte(recipe), 0o644); err != nil {
				t.Fatal(err)
			}

			stdout, stderr, code := runCmd("apply", "--path", tmp, slug, "--mode", "execute")
			if code != 0 {
				t.Fatalf("apply execute failed: %s", stderr)
			}
			if !strings.Contains(stdout, tc.want) {
				t.Fatalf("stdout missing the frozen line %q:\n%s", tc.want, stdout)
			}
			rgaS0AssertNoCLICoverageArtifact(t, tmp, slug)
		})
	}
}

// rgaS0CheckRecipeExecutedLine pins the source of the executed line.
func rgaS0CheckRecipeExecutedLine(src string) error {
	fn, err := rgaS0CLIFunc("cobra.go", src, "runApplyExecuteChecked")
	if err != nil {
		return err
	}
	const wantFormat = "Recipe executed: %d/%d operations succeeded\n"
	found := 0
	var args []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || rgaS0CLICallName(call) != "fmt.Fprintf" || len(call.Args) != 4 {
			return true
		}
		format, isLit := rgaS0CLIStringLit(call.Args[1])
		if !isLit || !strings.HasPrefix(format, "Recipe executed") {
			return true
		}
		if format != wantFormat {
			args = append(args, format)
			return true
		}
		found++
		for _, a := range call.Args[2:] {
			sel, ok := a.(*ast.SelectorExpr)
			if !ok {
				args = append(args, "<non-selector>")
				continue
			}
			args = append(args, sel.Sel.Name)
		}
		return true
	})
	if found != 1 {
		return fmt.Errorf("want exactly one verbatim `Recipe executed` print, got %d (seen: %v)", found, args)
	}
	if len(args) != 2 || args[0] != "Applied" || args[1] != "Operations" {
		return fmt.Errorf("the executed line no longer reports result.Applied/result.Operations, got %v", args)
	}
	return nil
}

// TestRGAS0RecipeExecutedLineSource pins the shipped format string and its
// two arguments.
func TestRGAS0RecipeExecutedLineSource(t *testing.T) {
	src := rgaS0CLISource(t, "internal/cli/cobra.go")
	if err := rgaS0CheckRecipeExecutedLine(src); err != nil {
		t.Fatalf("the Recipe-executed line changed: %v", err)
	}

	t.Run("sensitivity", func(t *testing.T) {
		for _, tc := range []struct{ name, old, new string }{
			{
				name: "line-text-changes",
				old:  "Recipe executed: %d/%d operations succeeded\\n",
				new:  "Recipe executed: %d of %d operations succeeded\\n",
			},
			{
				name: "arguments-change",
				old:  "result.Applied, result.Operations)",
				new:  "result.Operations, result.Applied)",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if !strings.Contains(src, tc.old) {
					t.Fatalf("mutation anchor no longer present:\n%q", tc.old)
				}
				if err := rgaS0CheckRecipeExecutedLine(strings.Replace(src, tc.old, tc.new, 1)); err == nil {
					t.Fatalf("guard did not catch mutation %q", tc.name)
				}
			})
		}
	})
}

// ── the legacy missing-recipe error and its exit 1 ───────────────────────

// TestRGAS0ApplyExecuteMissingRecipeIsExitOne freezes the legacy refusal
// §6.11 replaces with a named `recipe-generation-incomplete` contract in
// S5. Today it is `LoadRecipe`'s read error, surfaced verbatim, exit 1.
func TestRGAS0ApplyExecuteMissingRecipeIsExitOne(t *testing.T) {
	tmp := t.TempDir()
	gitInitTestRepo(t, tmp)
	runCmd("init", "--path", tmp)
	slug := "s0-no-recipe"
	runCmd("add", "--path", tmp, "--slug", slug, "S0 missing recipe")

	recipePath := filepath.Join(tmp, ".tpatch", "features", slug, "artifacts", "apply-recipe.json")
	if _, err := os.Stat(recipePath); !os.IsNotExist(err) {
		t.Fatalf("fixture precondition: no recipe may exist; stat err = %v", err)
	}

	_, stderr, code := runCmdExit("apply", "--path", tmp, slug, "--mode", "execute")
	if code != 1 {
		t.Fatalf("missing recipe must exit 1 today, got %d (stderr=%q)", code, stderr)
	}
	want := "no recipe found — run 'tpatch implement " + slug + "' first"
	if !strings.Contains(stderr, want) {
		t.Fatalf("stderr missing the frozen legacy error %q:\n%s", want, stderr)
	}
	// The message is produced by LoadRecipe, not by the CLI, and it is
	// the ONLY refusal on this path today: no named contract, no exit 2.
	if strings.Contains(stderr, "recipe-generation-incomplete") {
		t.Error("the §6.11 named refusal already exists; the S0 record is stale")
	}
	rgaS0AssertNoCLICoverageArtifact(t, tmp, slug)
}

// TestRGAS0LoadRecipeErrorTextSource pins the exact error string and its
// single production home.
func TestRGAS0LoadRecipeErrorTextSource(t *testing.T) {
	src := rgaS0CLISource(t, "internal/workflow/recipe.go")
	const wantLiteral = "no recipe found — run 'tpatch implement %s' first"
	count := func(body string) int { return strings.Count(body, wantLiteral) }
	if count(src) != 1 {
		t.Fatalf("the legacy no-recipe error is no longer present exactly once in recipe.go")
	}
	rendered := fmt.Sprintf(wantLiteral, "demo")
	if rendered != "no recipe found — run 'tpatch implement demo' first" {
		t.Fatalf("rendered legacy error changed: %q", rendered)
	}

	t.Run("sensitivity", func(t *testing.T) {
		// Rewording the message must be visible here.
		if got := count(strings.Replace(src, wantLiteral, "no apply recipe found for %s", 1)); got != 0 {
			t.Fatalf("reworded message still matched %d time(s)", got)
		}
		// So must a second copy appearing elsewhere.
		if got := count(src + "\n// " + wantLiteral + "\n"); got != 2 {
			t.Fatalf("a duplicated message was not detected; count = %d", got)
		}
	})
}

// ── P7: `tpatch edit` resolution and the discarded editor error ──────────

// TestRGAS0EditResolvedPathPrecedence freezes P7's trigger: the resolved
// path, never the typed token. A feature-root decoy shadows the canonical
// artifact, and the explicit `artifacts/...` spelling resolves to the
// canonical path on the first probe.
func TestRGAS0EditResolvedPathPrecedence(t *testing.T) {
	tmp := t.TempDir()
	slug := "s0-edit"
	featureDir := filepath.Join(tmp, ".tpatch", "features", slug)
	artifactsDir := filepath.Join(featureDir, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(artifactsDir, "apply-recipe.json")
	if err := os.WriteFile(canonical, []byte("{\"feature\":\""+slug+"\",\"operations\":[]}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, "spec.md"), []byte("spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &store.Store{Root: tmp}

	t.Run("bare-token-resolves-to-the-canonical-artifact", func(t *testing.T) {
		got, exists := resolveArtifactPath(s, slug, "apply-recipe.json")
		if !exists || got != canonical {
			t.Fatalf("resolved %q (exists=%v), want %q", got, exists, canonical)
		}
	})

	t.Run("explicit-artifacts-spelling-resolves-on-the-first-probe", func(t *testing.T) {
		got, exists := resolveArtifactPath(s, slug, filepath.Join("artifacts", "apply-recipe.json"))
		if !exists || got != canonical {
			t.Fatalf("resolved %q (exists=%v), want %q", got, exists, canonical)
		}
	})

	t.Run("feature-root-decoy-shadows-the-canonical-artifact", func(t *testing.T) {
		decoy := filepath.Join(featureDir, "apply-recipe.json")
		if err := os.WriteFile(decoy, []byte("decoy\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(decoy)
		got, exists := resolveArtifactPath(s, slug, "apply-recipe.json")
		if !exists || got != decoy {
			t.Fatalf("resolved %q, want the feature-root decoy %q (PRD §6.15 P7)", got, decoy)
		}
		if got == canonical {
			t.Fatal("the decoy no longer takes precedence; the P7 trigger analysis is stale")
		}
	})

	t.Run("an-unrelated-artifact-is-not-bound", func(t *testing.T) {
		got, exists := resolveArtifactPath(s, slug, "spec.md")
		if !exists {
			t.Fatalf("spec.md should resolve, got exists=false")
		}
		if strings.HasSuffix(filepath.ToSlash(got), "artifacts/post-apply.patch") ||
			strings.HasSuffix(filepath.ToSlash(got), "artifacts/apply-recipe.json") {
			t.Fatalf("spec.md resolved to a bound artifact: %q", got)
		}
	})
}

// TestRGAS0EditWithoutEditorIsNotAnEvent freezes the `$EDITOR` unset row:
// no process starts, no byte changes, nothing is published, exit 0.
func TestRGAS0EditWithoutEditorIsNotAnEvent(t *testing.T) {
	tmp := t.TempDir()
	gitInitTestRepo(t, tmp)
	runCmd("init", "--path", tmp)
	slug := "s0-edit-noeditor"
	runCmd("add", "--path", tmp, "--slug", slug, "S0 edit without editor")

	artifactsDir := filepath.Join(tmp, ".tpatch", "features", slug, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(artifactsDir, "apply-recipe.json")
	body := "{\"feature\":\"" + slug + "\",\"operations\":[]}\n"
	if err := os.WriteFile(canonical, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("EDITOR", "")

	stdout, stderr, code := runCmd("edit", "--path", tmp, slug, "artifacts/apply-recipe.json")
	if code != 0 {
		t.Fatalf("edit failed: %s", stderr)
	}
	const prefix = "  (set $EDITOR to review "
	const suffix = " in your editor)\n"
	start := strings.Index(stdout, prefix)
	if start < 0 {
		t.Fatalf("stdout missing the frozen pointer line prefix %q:\n%s", prefix, stdout)
	}
	rest := stdout[start+len(prefix):]
	end := strings.Index(rest, suffix)
	if end < 0 {
		t.Fatalf("stdout missing the frozen pointer line suffix %q:\n%s", suffix, stdout)
	}
	// The resolved path is compared by suffix so a symlinked temp root
	// (macOS `/var` → `/private/var`) does not make the row flaky.
	wantSuffix := filepath.Join(".tpatch", "features", slug, "artifacts", "apply-recipe.json")
	if got := rest[:end]; !strings.HasSuffix(got, wantSuffix) {
		t.Fatalf("pointer line names %q, want a path ending in %q", got, wantSuffix)
	}
	after, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != body {
		t.Fatalf("no byte may change when $EDITOR is unset:\n got %q\nwant %q", after, body)
	}
	rgaS0AssertNoCLICoverageArtifact(t, tmp, slug)
}

// rgaS0CheckOpenInEditorDiscardsError asserts the shipped helper: no error
// result, and `c.Run()`'s error assigned to the blank identifier. §6.2
// refactors this in S1; S0 records what it is refactoring from.
func rgaS0CheckOpenInEditorDiscardsError(src string) error {
	fn, err := rgaS0CLIFunc("phase2.go", src, "openInEditor")
	if err != nil {
		return err
	}
	if fn.Type.Results != nil && len(fn.Type.Results.List) != 0 {
		return fmt.Errorf("openInEditor now returns %d result(s); the discarded-error baseline is stale", len(fn.Type.Results.List))
	}
	discarded := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		blank, ok := assign.Lhs[0].(*ast.Ident)
		if !ok || blank.Name != "_" {
			return true
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if ok && rgaS0CLICallName(call) == "c.Run" {
			discarded = true
		}
		return true
	})
	if !discarded {
		return fmt.Errorf("openInEditor no longer discards c.Run()'s error")
	}
	return nil
}

// TestRGAS0OpenInEditorSourceContract freezes the two P7 facts §2.7
// records: the helper cannot report an editor failure, and it has exactly
// two call sites, only one of which can resolve to a bound artifact.
func TestRGAS0OpenInEditorSourceContract(t *testing.T) {
	phase2 := rgaS0CLISource(t, "internal/cli/phase2.go")
	if err := rgaS0CheckOpenInEditorDiscardsError(phase2); err != nil {
		t.Fatalf("openInEditor changed: %v", err)
	}

	callSites := 0
	for _, rel := range []string{"internal/cli/phase2.go", "internal/cli/c1.go"} {
		src := rgaS0CLISource(t, rel)
		file, err := parser.ParseFile(token.NewFileSet(), rel, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "openInEditor" {
				callSites++
			}
			return true
		})
	}
	if callSites != 2 {
		t.Errorf("openInEditor call sites = %d, want 2 (internal/cli/c1.go and internal/cli/phase2.go)", callSites)
	}

	t.Run("sensitivity", func(t *testing.T) {
		for _, tc := range []struct{ name, old, new string }{
			{
				name: "error-becomes-propagated",
				old:  "\t_ = c.Run()",
				new:  "\treturn c.Run()",
			},
			{
				name: "signature-gains-an-error-result",
				old:  "func openInEditor(out io.Writer, path string) {",
				new:  "func openInEditor(out io.Writer, path string) error {",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if !strings.Contains(phase2, tc.old) {
					t.Fatalf("mutation anchor no longer present:\n%q", tc.old)
				}
				if err := rgaS0CheckOpenInEditorDiscardsError(strings.Replace(phase2, tc.old, tc.new, 1)); err == nil {
					t.Fatalf("guard did not catch mutation %q", tc.name)
				}
			})
		}
	})
}

// ── P5: `apply --mode done` writes the patch and publishes nothing ───────

// TestRGAS0ApplyDoneProducerBaseline freezes P5 through the auto pipeline
// (the shape `TestApplyAutoMode` already exercises): a non-empty capture
// writes the canonical patch and a numbered snapshot, and no coverage
// record appears beside them.
func TestRGAS0ApplyDoneProducerBaseline(t *testing.T) {
	tmp := t.TempDir()
	gitInitTestRepo(t, tmp)
	runCmd("init", "--path", tmp)
	slug := "s0-apply-done"
	runCmd("add", "--path", tmp, "--slug", slug, "S0 apply done")

	artDir := filepath.Join(tmp, ".tpatch", "features", slug, "artifacts")
	if err := os.MkdirAll(artDir, 0o755); err != nil {
		t.Fatal(err)
	}
	recipe := "{\n  \"feature\": \"" + slug + "\",\n  \"operations\": [\n" +
		"    {\"type\": \"write-file\", \"path\": \"hello.txt\", \"content\": \"hi\\n\"}\n  ]\n}\n"
	if err := os.WriteFile(filepath.Join(artDir, "apply-recipe.json"), []byte(recipe), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runCmd("apply", "--path", tmp, slug)
	if code != 0 {
		t.Fatalf("apply auto failed: %s", stderr)
	}
	if !strings.Contains(stdout, "marked as applied") {
		t.Fatalf("expected the done step to run:\n%s", stdout)
	}
	patch := readRecordedPatch(t, tmp, slug)
	if !strings.Contains(patch, "hello.txt") {
		t.Fatalf("P5 must write the canonical patch it captured:\n%s", patch)
	}
	// P5 writes no recipe: the recipe on disk is the one the fixture
	// seeded, byte-identical.
	after, err := os.ReadFile(filepath.Join(artDir, "apply-recipe.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != recipe {
		t.Fatalf("P5 must not rewrite the recipe:\n got %q\nwant %q", after, recipe)
	}
	rgaS0AssertNoCLICoverageArtifact(t, tmp, slug)
}
