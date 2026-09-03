package workflow

// GH #15 / ADR-036 slice S0 — frozen evidence, source-derived guards.
//
// PRD-recipe-generation-authority §8 "S0 - Frozen evidence" requires the
// pre-change producer/parser surface to be pinned so the S1/S4 refactors
// are deliberate rather than incidental. Every guard here is expressed as
// a pure `func(src string) error` (or a pure inventory function) so the
// SAME matcher can be run over deliberately mutated source in a
// sensitivity subtest: a guard that cannot fail proves nothing.
//
// Conventions:
//   - never assert absolute line numbers; identity is (file, enclosing
//     function, literal);
//   - parse Go with go/ast rather than counting tokens, so a comment or a
//     renamed local cannot satisfy — or break — a semantic claim;
//   - production scope excludes `_test.go` and every `testdata/` tree.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// ── shared scaffolding ───────────────────────────────────────────────────

// rgaS0RepoRoot walks up from the package directory to the module root.
func rgaS0RepoRoot(t *testing.T) string {
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

// rgaS0ProductionGoFiles lists every non-test Go file under the module's
// own source trees, slash-separated and relative to the module root.
func rgaS0ProductionGoFiles(t *testing.T) []string {
	t.Helper()
	root := rgaS0RepoRoot(t)
	var out []string
	for _, tree := range []string{"internal", "cmd", "assets"} {
		base := filepath.Join(root, tree)
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
				return nil
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			out = append(out, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", tree, err)
		}
	}
	if len(out) == 0 {
		t.Fatal("production Go inventory is empty; the walker is broken")
	}
	sort.Strings(out)
	return out
}

func rgaS0ReadRepoFile(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(rgaS0RepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(body)
}

// rgaS0Parse parses Go source text. Guards take text (not a path) so a
// mutation subtest can feed a modified copy through the same matcher.
func rgaS0Parse(name, src string) (*ast.File, error) {
	return parser.ParseFile(token.NewFileSet(), name, src, parser.ParseComments)
}

// rgaS0EnclosingName renders a declaration's identity, including the
// receiver type for methods.
func rgaS0EnclosingName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	var buf strings.Builder
	buf.WriteString("(")
	rgaS0WriteTypeName(&buf, fn.Recv.List[0].Type)
	buf.WriteString(").")
	buf.WriteString(fn.Name.Name)
	return buf.String()
}

func rgaS0WriteTypeName(buf *strings.Builder, expr ast.Expr) {
	switch t := expr.(type) {
	case *ast.StarExpr:
		buf.WriteString("*")
		rgaS0WriteTypeName(buf, t.X)
	case *ast.Ident:
		buf.WriteString(t.Name)
	case *ast.SelectorExpr:
		rgaS0WriteTypeName(buf, t.X)
		buf.WriteString(".")
		buf.WriteString(t.Sel.Name)
	default:
		buf.WriteString("?")
	}
}

// rgaS0StringLit returns the decoded value of a string-literal expression.
func rgaS0StringLit(expr ast.Expr) (string, bool) {
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

// rgaS0CallName renders a call's callee as `recv.Method`, `pkg.Func` or
// `Func`, which is enough identity for these guards.
func rgaS0CallName(call *ast.CallExpr) string {
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

// rgaS0FuncBody returns the declaration of the named top-level function.
func rgaS0FuncBody(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && rgaS0EnclosingName(fn) == name {
			return fn
		}
	}
	return nil
}

// ── the eight direct bound `WriteArtifact` sites (PRD §6.15) ─────────────

// rgaS0BoundArtifacts is the closed set of artifacts coverage binds.
var rgaS0BoundArtifacts = map[string]bool{
	"post-apply.patch":  true,
	"apply-recipe.json": true,
}

// rgaS0WriteSite identifies one `WriteArtifact` call without using a line
// number: the file, the enclosing declaration, and the artifact literal.
type rgaS0WriteSite struct {
	File     string
	Func     string
	Artifact string
}

func (s rgaS0WriteSite) key() string {
	return s.File + "|" + s.Func + "|" + s.Artifact
}

// rgaS0ScanWriteArtifact returns every `*.WriteArtifact(slug, <literal>,
// ...)` call in one file whose artifact literal is a bound artifact, plus
// the count of calls whose artifact argument is NOT a literal at all.
// A dynamic second argument is how a future bound write could hide from a
// literal-only inventory, so it is counted rather than ignored.
func rgaS0ScanWriteArtifact(relPath, src string) (bound []rgaS0WriteSite, dynamic []string, err error) {
	file, perr := rgaS0Parse(relPath, src)
	if perr != nil {
		return nil, nil, perr
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		owner := rgaS0EnclosingName(fn)
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "WriteArtifact" || len(call.Args) < 2 {
				return true
			}
			name, isLit := rgaS0StringLit(call.Args[1])
			if !isLit {
				dynamic = append(dynamic, relPath+"|"+owner)
				return true
			}
			if rgaS0BoundArtifacts[name] {
				bound = append(bound, rgaS0WriteSite{File: relPath, Func: owner, Artifact: name})
			}
			return true
		})
	}
	return bound, dynamic, nil
}

// TestRGAS0BoundArtifactWriteSiteInventory freezes the pre-S4 producer
// write surface. PRD §6.15 says S4 adds a publication step to each of the
// seven governed producers; this inventory makes an added, moved or
// removed bound write a deliberate, visible edit.
func TestRGAS0BoundArtifactWriteSiteInventory(t *testing.T) {
	// (file, enclosing function, artifact) → number of direct calls.
	want := map[string]int{
		"internal/cli/cobra.go|recordCmd|post-apply.patch":                    1,
		"internal/cli/cobra.go|runApplyDone|post-apply.patch":                 1,
		"internal/cli/feature_patch.go|runFeaturePatchAmend|post-apply.patch": 1,
		"internal/cli/phase2.go|cycleCmd|post-apply.patch":                    1,
		"internal/workflow/implement.go|RunImplement|apply-recipe.json":       2,
		"internal/workflow/recipe_autogen.go|writeRecipe|apply-recipe.json":   1,
		"internal/workflow/refresh.go|RefreshAfterAccept|post-apply.patch":    1,
	}
	wantDynamic := map[string]bool{
		"internal/cli/feature_unapply.go|defaultUnapplyRuntime":    true,
		"internal/workflow/retry.go|GenerateWithRetry":             true,
		"internal/workflow/workflow.go|replayLegacyPhaseResponses": true,
	}

	got := map[string]int{}
	gotDynamic := map[string]bool{}
	total := 0
	for _, rel := range rgaS0ProductionGoFiles(t) {
		bound, dynamic, err := rgaS0ScanWriteArtifact(rel, rgaS0ReadRepoFile(t, rel))
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		for _, site := range bound {
			got[site.key()]++
			total++
		}
		for _, d := range dynamic {
			gotDynamic[d] = true
		}
	}

	if total != 8 {
		t.Errorf("direct bound WriteArtifact call count = %d, want 8 (PRD §6.15 rev-4)", total)
	}
	for key, count := range want {
		if got[key] != count {
			t.Errorf("bound write site %q: got %d call(s), want %d", key, got[key], count)
		}
	}
	for key, count := range got {
		if _, registered := want[key]; !registered {
			t.Errorf("UNREGISTERED bound write site %q (%d call(s)); register it before changing the producer surface", key, count)
		}
	}
	for key := range gotDynamic {
		if !wantDynamic[key] {
			t.Errorf("UNREGISTERED dynamic-artifact WriteArtifact site %q; a variable artifact name can hide a bound write from this inventory", key)
		}
	}
	for key := range wantDynamic {
		if !gotDynamic[key] {
			t.Errorf("registered dynamic-artifact WriteArtifact site %q disappeared", key)
		}
	}
}

// TestRGAS0BoundArtifactWriteScannerIsSensitive proves the scanner above
// would actually catch the regressions it claims to catch.
func TestRGAS0BoundArtifactWriteScannerIsSensitive(t *testing.T) {
	cases := []struct {
		name        string
		src         string
		wantBound   int
		wantDynamic int
	}{
		{
			name: "detects-a-new-bound-write",
			src: "package p\nfunc sneak(s *S, slug string) {\n" +
				"\ts.WriteArtifact(slug, \"apply-recipe.json\", \"x\")\n}\n",
			wantBound: 1,
		},
		{
			name: "detects-a-bound-write-nested-in-a-closure",
			src: "package p\nfunc outer(s *S, slug string) func() {\n" +
				"\treturn func() { s.WriteArtifact(slug, \"post-apply.patch\", \"x\") }\n}\n",
			wantBound: 1,
		},
		{
			name:        "detects-a-dynamic-artifact-name",
			src:         "package p\nfunc dyn(s *S, slug, name string) {\n\ts.WriteArtifact(slug, name, \"x\")\n}\n",
			wantDynamic: 1,
		},
		{
			name: "ignores-an-unbound-artifact",
			src:  "package p\nfunc other(s *S, slug string) {\n\ts.WriteArtifact(slug, \"reconcile.md\", \"x\")\n}\n",
		},
		{
			name: "ignores-a-comment-mentioning-the-artifact",
			src:  "package p\n\n// writes apply-recipe.json eventually\nfunc doc() {}\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bound, dynamic, err := rgaS0ScanWriteArtifact("synthetic.go", tc.src)
			if err != nil {
				t.Fatalf("parse synthetic source: %v", err)
			}
			if len(bound) != tc.wantBound {
				t.Errorf("bound sites = %d, want %d (%+v)", len(bound), tc.wantBound, bound)
			}
			if len(dynamic) != tc.wantDynamic {
				t.Errorf("dynamic sites = %d, want %d (%v)", len(dynamic), tc.wantDynamic, dynamic)
			}
		})
	}
}

// ── no coverage artifact exists yet (PRD §8 S0 pre-change contract) ──────

// rgaS0CoverageTokens are the symbols and filenames GH #15 will introduce.
// S0 asserts NONE of them exists in production today, so S1-S5 cannot
// claim to be "preserving" a surface that was never shipped.
var rgaS0CoverageTokens = []string{
	"recipe-coverage.json",
	"RecipeCoverage",
	"recipe_coverage",
	"CoverageProducer",
	"PublishCoverage",
	"coverage_status",
}

// rgaS0ScanForbiddenTokens reports every forbidden token present in src.
func rgaS0ScanForbiddenTokens(src string, tokens []string) []string {
	var hits []string
	for _, tok := range tokens {
		if strings.Contains(src, tok) {
			hits = append(hits, tok)
		}
	}
	sort.Strings(hits)
	return hits
}

// TestRGAS0NoProductionCoverageSurfaceYet freezes the pre-change artifact
// contract: there is no production `recipe-coverage.json` type, writer or
// reader, and no coverage-producer symbol. Test fixtures may mention the
// name freely — the scan is deliberately scoped to non-test production Go.
func TestRGAS0NoProductionCoverageSurfaceYet(t *testing.T) {
	for _, rel := range rgaS0ProductionGoFiles(t) {
		if hits := rgaS0ScanForbiddenTokens(rgaS0ReadRepoFile(t, rel), rgaS0CoverageTokens); len(hits) > 0 {
			t.Errorf("%s already references the unshipped coverage surface %v; S0 must record its absence", rel, hits)
		}
	}
}

// TestRGAS0CoverageTokenScannerIsSensitive proves the absence assertion
// above is a real measurement and not a vacuous scan.
func TestRGAS0CoverageTokenScannerIsSensitive(t *testing.T) {
	planted := "package store\n\ntype RecipeCoverage struct{}\n\nconst coveragePath = \"recipe-coverage.json\"\n"
	hits := rgaS0ScanForbiddenTokens(planted, rgaS0CoverageTokens)
	if len(hits) != 2 || hits[0] != "RecipeCoverage" || hits[1] != "recipe-coverage.json" {
		t.Fatalf("scanner missed a planted coverage surface; hits = %v", hits)
	}
	if hits := rgaS0ScanForbiddenTokens("package store\n\ntype Recipe struct{}\n", rgaS0CoverageTokens); len(hits) != 0 {
		t.Fatalf("scanner false-positives on unrelated source; hits = %v", hits)
	}
	// The scan must be scoped away from tests: this very file names the
	// tokens, and it must not be part of the production inventory.
	for _, rel := range rgaS0ProductionGoFiles(t) {
		if strings.HasSuffix(rel, "_test.go") {
			t.Fatalf("production inventory leaked a test file: %s", rel)
		}
	}
}

// ── autogeneration derivation contract (PRD §2.8, §6.3) ──────────────────

// rgaS0CheckAutogenDerivation asserts the two load-bearing facts S2
// changes: drift is decided by FILE-SET comparison, and the postimage is
// read from the LIVE worktree under the repo root.
func rgaS0CheckAutogenDerivation(src string) error {
	file, err := rgaS0Parse("recipe_autogen.go", src)
	if err != nil {
		return err
	}

	autogen := rgaS0FuncBody(file, "AutogenRecipeForRecord")
	if autogen == nil {
		return fmt.Errorf("AutogenRecipeForRecord not found")
	}
	callsFileSetCompare := false
	ast.Inspect(autogen.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && rgaS0CallName(call) == "compareRecipeFileSets" {
			callsFileSetCompare = true
		}
		return true
	})
	if !callsFileSetCompare {
		return fmt.Errorf("AutogenRecipeForRecord no longer decides drift with compareRecipeFileSets")
	}

	compare := rgaS0FuncBody(file, "compareRecipeFileSets")
	if compare == nil {
		return fmt.Errorf("compareRecipeFileSets not found")
	}
	pathSetCalls := 0
	ast.Inspect(compare.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && rgaS0CallName(call) == "recipePathSet" {
			pathSetCalls++
		}
		return true
	})
	if pathSetCalls != 2 {
		return fmt.Errorf("compareRecipeFileSets calls recipePathSet %d time(s), want 2 (existing vs derived path sets)", pathSetCalls)
	}

	pathSet := rgaS0FuncBody(file, "recipePathSet")
	if pathSet == nil {
		return fmt.Errorf("recipePathSet not found")
	}
	keysOnPathOnly := false
	ast.Inspect(pathSet.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 {
			return true
		}
		index, ok := assign.Lhs[0].(*ast.IndexExpr)
		if !ok {
			return true
		}
		sel, ok := index.Index.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == "Path" {
			keysOnPathOnly = true
		}
		return true
	})
	if !keysOnPathOnly {
		return fmt.Errorf("recipePathSet no longer keys purely on op.Path; drift is no longer file-set based")
	}

	derive := rgaS0FuncBody(file, "RecipeFromPatch")
	if derive == nil {
		return fmt.Errorf("RecipeFromPatch not found")
	}
	readsWorktree := false
	joinsRepoRoot := false
	ast.Inspect(derive.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch rgaS0CallName(call) {
		case "os.ReadFile":
			readsWorktree = true
		case "filepath.Join":
			if len(call.Args) > 0 {
				if ident, ok := call.Args[0].(*ast.Ident); ok && ident.Name == "repoRoot" {
					joinsRepoRoot = true
				}
			}
		}
		return true
	})
	if !readsWorktree || !joinsRepoRoot {
		return fmt.Errorf("RecipeFromPatch no longer reads the live worktree via os.ReadFile under repoRoot (readFile=%v joinRepoRoot=%v)", readsWorktree, joinsRepoRoot)
	}
	return nil
}

// TestRGAS0AutogenDerivationSourceContract pins the S2 targets: pure
// derivation from a captured observation, and retirement of
// `compareRecipeFileSets` from origin/completeness decisions.
func TestRGAS0AutogenDerivationSourceContract(t *testing.T) {
	if err := rgaS0CheckAutogenDerivation(rgaS0ReadRepoFile(t, "internal/workflow/recipe_autogen.go")); err != nil {
		t.Fatalf("current autogeneration contract changed: %v", err)
	}

	t.Run("sensitivity", func(t *testing.T) {
		src := rgaS0ReadRepoFile(t, "internal/workflow/recipe_autogen.go")
		for _, tc := range []struct{ name, old, new string }{
			{
				name: "drift-stops-being-file-set-based",
				old:  "drift, reason := compareRecipeFileSets(existingRecipe, derived)",
				new:  "drift, reason := compareRecipeBytes(existingRecipe, derived)",
			},
			{
				name: "postimage-stops-coming-from-the-worktree",
				old:  "data, err := os.ReadFile(target)",
				new:  "data, err := postimageFromObservation(fc.Path)",
			},
			{
				name: "path-set-stops-keying-on-path",
				old:  "m[op.Path] = true",
				new:  "m[op.Path+op.Content] = true",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if !strings.Contains(src, tc.old) {
					t.Fatalf("mutation anchor %q no longer present; refresh the sensitivity fixture", tc.old)
				}
				mutated := strings.Replace(src, tc.old, tc.new, 1)
				if err := rgaS0CheckAutogenDerivation(mutated); err == nil {
					t.Fatalf("guard did not catch mutation %q → %q", tc.old, tc.new)
				}
			})
		}
	})
}

// ── implement's two recipe-parse arms (PRD §2.7, §6.15 P6) ───────────────

// rgaS0CheckImplementParseArms asserts the shipped control flow: the
// unmarshal failure arm writes the RAW response bytes, the success arm
// writes reserialized bytes, and neither arm returns the parse error.
func rgaS0CheckImplementParseArms(src string) error {
	file, err := rgaS0Parse("implement.go", src)
	if err != nil {
		return err
	}
	fn := rgaS0FuncBody(file, "RunImplement")
	if fn == nil {
		return fmt.Errorf("RunImplement not found")
	}

	var parseStmt *ast.IfStmt
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if parseStmt != nil {
			return false
		}
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok || ifStmt.Init == nil || ifStmt.Else == nil {
			return true
		}
		assign, ok := ifStmt.Init.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, rhs := range assign.Rhs {
			if call, ok := rhs.(*ast.CallExpr); ok && rgaS0CallName(call) == "json.Unmarshal" {
				parseStmt = ifStmt
				return false
			}
		}
		return true
	})
	if parseStmt == nil {
		return fmt.Errorf("the json.Unmarshal recipe-parse if/else no longer exists in RunImplement")
	}

	rawArg, err := rgaS0ImplementWriteArg(parseStmt.Body)
	if err != nil {
		return fmt.Errorf("failure arm: %w", err)
	}
	if rawArg != "recipeContent" {
		return fmt.Errorf("failure arm writes %q, want the raw response identifier recipeContent", rawArg)
	}
	elseBlock, ok := parseStmt.Else.(*ast.BlockStmt)
	if !ok {
		return fmt.Errorf("the recipe-parse else branch is no longer a block")
	}
	okArg, err := rgaS0ImplementWriteArg(elseBlock)
	if err != nil {
		return fmt.Errorf("success arm: %w", err)
	}
	if okArg == "recipeContent" {
		return fmt.Errorf("success arm must write reserialized bytes, not the raw response")
	}

	// Neither arm may return the parse error: the error identifier bound
	// by the `if` init is consumed by the condition only.
	if !rgaS0BlockReturnsOnlyWriteError(parseStmt.Body) {
		return fmt.Errorf("failure arm now returns the parse error rather than only its WriteArtifact error")
	}
	return nil
}

// rgaS0ImplementWriteArg returns the rendered third argument of the
// block's single bound `WriteArtifact` call.
func rgaS0ImplementWriteArg(block *ast.BlockStmt) (string, error) {
	var found []string
	ast.Inspect(block, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 3 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "WriteArtifact" {
			return true
		}
		name, isLit := rgaS0StringLit(call.Args[1])
		if !isLit || name != "apply-recipe.json" {
			return true
		}
		switch arg := call.Args[2].(type) {
		case *ast.Ident:
			found = append(found, arg.Name)
		default:
			found = append(found, "<expression>")
		}
		return true
	})
	if len(found) != 1 {
		return "", fmt.Errorf("expected exactly one bound apply-recipe.json write, found %d", len(found))
	}
	return found[0], nil
}

// rgaS0BlockReturnsOnlyWriteError reports whether the block contains exactly
// one direct `return err`, as the sole statement in the body of an
// `if err := ...WriteArtifact(...)` guard.
func rgaS0BlockReturnsOnlyWriteError(block *ast.BlockStmt) bool {
	returns := 0
	writeReturns := 0
	ast.Inspect(block, func(n ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok {
			if len(ret.Results) == 1 {
				if ident, ok := ret.Results[0].(*ast.Ident); ok && ident.Name == "err" {
					returns++
				}
			}
			return true
		}
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok || ifStmt.Init == nil || len(ifStmt.Body.List) != 1 {
			return true
		}
		assign, ok := ifStmt.Init.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, rhs := range assign.Rhs {
			call, ok := rhs.(*ast.CallExpr)
			if !ok {
				continue
			}
			sel, isSel := call.Fun.(*ast.SelectorExpr)
			ret, isReturn := ifStmt.Body.List[0].(*ast.ReturnStmt)
			if !isSel || sel.Sel.Name != "WriteArtifact" || !isReturn || len(ret.Results) != 1 {
				continue
			}
			if ident, ok := ret.Results[0].(*ast.Ident); ok && ident.Name == "err" {
				writeReturns++
			}
		}
		return true
	})
	return returns == 1 && writeReturns == 1
}

// TestRGAS0ImplementParseArmsSourceContract freezes P6's shipped shape so
// S4's finalizer is added deliberately.
func TestRGAS0ImplementParseArmsSourceContract(t *testing.T) {
	if err := rgaS0CheckImplementParseArms(rgaS0ReadRepoFile(t, "internal/workflow/implement.go")); err != nil {
		t.Fatalf("RunImplement's recipe-parse arms changed: %v", err)
	}

	t.Run("sensitivity", func(t *testing.T) {
		src := rgaS0ReadRepoFile(t, "internal/workflow/implement.go")
		for _, tc := range []struct{ name, old, new string }{
			{
				name: "failure-arm-stops-writing-raw-bytes",
				old:  `s.WriteArtifact(slug, "apply-recipe.json", recipeContent)`,
				new:  `s.WriteArtifact(slug, "apply-recipe.json", "{}")`,
			},
			{
				name: "success-arm-starts-writing-raw-bytes",
				old:  `s.WriteArtifact(slug, "apply-recipe.json", string(data)+"\n")`,
				new:  `s.WriteArtifact(slug, "apply-recipe.json", recipeContent)`,
			},
			{
				name: "failure-arm-starts-returning-parse-error",
				old:  "if err := s.WriteArtifact(slug, \"apply-recipe.json\", recipeContent); err != nil {\n\t\t\treturn err\n\t\t}",
				new:  "if err := s.WriteArtifact(slug, \"apply-recipe.json\", recipeContent); err != nil {\n\t\t\treturn err\n\t\t}\n\t\treturn err",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if !strings.Contains(src, tc.old) {
					t.Fatalf("mutation anchor %q no longer present; refresh the sensitivity fixture", tc.old)
				}
				mutated := strings.Replace(src, tc.old, tc.new, 1)
				if err := rgaS0CheckImplementParseArms(mutated); err == nil {
					t.Fatalf("guard did not catch mutation %q → %q", tc.old, tc.new)
				}
			})
		}
	})
}

// ── reconcile phase 2's applicable-only gap (case study §3.2) ────────────

// rgaS0CheckPhase2Branches asserts the shipped phase-2 shape: an early
// terminal return ONLY under `allPresent`, a note ONLY under
// `hasConflicts`, and no branch at all for the applicable-only case.
func rgaS0CheckPhase2Branches(src string) error {
	file, err := rgaS0Parse("reconcile.go", src)
	if err != nil {
		return err
	}
	fn := rgaS0FuncBody(file, "reconcileFeature")
	if fn == nil {
		return fmt.Errorf("reconcileFeature not found")
	}

	sawEval := false
	allPresentReturns := 0
	conflictNotes := 0
	applicableBranches := 0
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && rgaS0CallName(call) == "evaluateRecipeOperations" {
			sawEval = true
		}
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		fields := rgaS0OpResultFields(ifStmt.Cond)
		if len(fields) == 0 {
			return true
		}
		hasReturn := rgaS0BlockHasReturn(ifStmt.Body)
		switch {
		case fields["applicableCount"] || fields["presentCount"] || fields["conflictCount"]:
			applicableBranches++
		case fields["allPresent"] && hasReturn:
			allPresentReturns++
		case fields["hasConflicts"] && !hasReturn:
			conflictNotes++
		}
		return true
	})

	if !sawEval {
		return fmt.Errorf("phase 2 no longer calls evaluateRecipeOperations")
	}
	if allPresentReturns != 1 {
		return fmt.Errorf("want exactly one terminal `opResult.allPresent` return, got %d", allPresentReturns)
	}
	if conflictNotes != 1 {
		return fmt.Errorf("want exactly one non-terminal `opResult.hasConflicts` note branch, got %d", conflictNotes)
	}
	if applicableBranches != 0 {
		return fmt.Errorf("phase 2 gained an applicable-only branch (%d); that is GH #13 work, not S0 baseline", applicableBranches)
	}
	return nil
}

func rgaS0BlockHasReturn(block *ast.BlockStmt) bool {
	hit := false
	ast.Inspect(block, func(n ast.Node) bool {
		if _, ok := n.(*ast.ReturnStmt); ok {
			hit = true
		}
		return true
	})
	return hit
}

// rgaS0OpResultFields collects every `opResult.<field>` selector reached
// from an expression, so a condition written as a comparison or a
// conjunction is recognised exactly like a bare selector.
func rgaS0OpResultFields(expr ast.Expr) map[string]bool {
	fields := map[string]bool{}
	ast.Inspect(expr, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if x, ok := sel.X.(*ast.Ident); ok && x.Name == "opResult" {
			fields[sel.Sel.Name] = true
		}
		return true
	})
	return fields
}

// TestRGAS0ReconcilePhase2SourceContract pins the branch structure the
// adjacent case study §3.2 measured: an applicable-only operation is
// neither replayed nor annotated, so reconcile falls through to phase 4.
func TestRGAS0ReconcilePhase2SourceContract(t *testing.T) {
	if err := rgaS0CheckPhase2Branches(rgaS0ReadRepoFile(t, "internal/workflow/reconcile.go")); err != nil {
		t.Fatalf("reconcile phase 2 branch structure changed: %v", err)
	}

	t.Run("sensitivity", func(t *testing.T) {
		src := rgaS0ReadRepoFile(t, "internal/workflow/reconcile.go")
		for _, tc := range []struct{ name, old, new string }{
			{
				name: "phase-2-gains-an-applicable-only-branch",
				old:  "\t\t\tif opResult.hasConflicts {",
				new:  "\t\t\tif opResult.applicableCount > 0 {\n\t\t\t\t_ = opResult\n\t\t\t}\n\t\t\tif opResult.hasConflicts {",
			},
			{
				name: "conflict-branch-becomes-terminal",
				old:  "\t\t\tif opResult.hasConflicts {\n\t\t\t\tresult.Notes = append(result.Notes,",
				new:  "\t\t\tif opResult.hasConflicts {\n\t\t\t\treturn result, nil\n\t\t\t\tresult.Notes = append(result.Notes,",
			},
			{
				name: "evaluator-is-dropped",
				old:  "opResult := evaluateRecipeOperations(s.Root, recipe.Operations)",
				new:  "opResult := operationEvalResult{}",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if !strings.Contains(src, tc.old) {
					t.Fatalf("mutation anchor no longer present; refresh the sensitivity fixture:\n%q", tc.old)
				}
				mutated := strings.Replace(src, tc.old, tc.new, 1)
				if err := rgaS0CheckPhase2Branches(mutated); err == nil {
					t.Fatalf("guard did not catch mutation %q", tc.name)
				}
			})
		}
	})
}

// ── ADR-029 D7 superseded-downgrade note text (PRD §8 S0) ────────────────

// TestRGAS0ADR029D7DowngradeNoteFrozen freezes the exact apply-time
// downgrade note and the routing behaviour behind it: when the feature is
// superseded, preimage-mismatch drift moves from Errors to Warnings with
// a provenance suffix, and later-touch drift receives the same suffix.
// S5's coverage row must not inherit this downgrade (PRD §9.9 RGA-337).
func TestRGAS0ADR029D7DowngradeNoteFrozen(t *testing.T) {
	const superseder = "newer-feature"
	const base = "recipe drift: [old-feature] op 0 src/a.txt: preimage mismatch"
	wantSuffixed := base + ` (downgraded: feature is superseded by "newer-feature" per Wave α; historical drift is warning-class per PRD-feature-supersession §4.5 / ADR-029 D7)`

	t.Run("preimage-drift-not-superseded-stays-an-error", func(t *testing.T) {
		var r PreimagePrecheckResult
		r.appendDrift(base, false, "")
		if len(r.Errors) != 1 || r.Errors[0] != base {
			t.Fatalf("Errors = %v, want the unsuffixed message", r.Errors)
		}
		if len(r.Warnings) != 0 {
			t.Fatalf("Warnings = %v, want none", r.Warnings)
		}
		if len(r.WrappedErrors) != 1 {
			t.Fatalf("WrappedErrors = %v, want one sentinel-wrapped error", r.WrappedErrors)
		}
	})

	t.Run("preimage-drift-superseded-downgrades-with-the-exact-note", func(t *testing.T) {
		var r PreimagePrecheckResult
		r.appendDrift(base, true, superseder)
		if len(r.Errors) != 0 {
			t.Fatalf("Errors = %v, want none after the D7 downgrade", r.Errors)
		}
		if len(r.Warnings) != 1 || r.Warnings[0] != wantSuffixed {
			t.Fatalf("Warnings[0] =\n %q\nwant\n %q", strings.Join(r.Warnings, "|"), wantSuffixed)
		}
		if len(r.WrappedWarnings) != 1 {
			t.Fatalf("WrappedWarnings = %v, want the drift class preserved", r.WrappedWarnings)
		}
	})

	t.Run("later-touch-carries-the-same-note", func(t *testing.T) {
		const lt = "later-touch: src/a.txt also touched by newer-feature"
		var r PreimagePrecheckResult
		r.appendLaterTouchWarn(lt, true, superseder)
		want := lt + ` (downgraded: feature is superseded by "newer-feature" per Wave α; historical drift is warning-class per PRD-feature-supersession §4.5 / ADR-029 D7)`
		if len(r.Warnings) != 1 || r.Warnings[0] != want {
			t.Fatalf("Warnings[0] =\n %q\nwant\n %q", strings.Join(r.Warnings, "|"), want)
		}
		if len(r.Errors) != 0 {
			t.Fatalf("later-touch must never be error-class (ADR-029 D6); got %v", r.Errors)
		}
	})

	t.Run("sensitivity-note-text-is-compared-verbatim", func(t *testing.T) {
		var r PreimagePrecheckResult
		r.appendDrift(base, true, "another-feature")
		if len(r.Warnings) != 1 {
			t.Fatalf("Warnings = %v", r.Warnings)
		}
		if r.Warnings[0] == wantSuffixed {
			t.Fatal("the frozen note must embed the superseder slug; a different superseder produced identical text")
		}
		if !strings.Contains(r.Warnings[0], `superseded by "another-feature"`) {
			t.Fatalf("superseder slug is not quoted into the note: %q", r.Warnings[0])
		}
	})
}

// rgaS0V10DowngradeFormat extracts the decoded supersession-downgrade
// format string from `evaluateV10`. Decoding through go/ast rather than
// matching raw file bytes keeps the assertion about the *string the code
// produces*, not about how the literal happens to be escaped.
func rgaS0V10DowngradeFormat(src string) (string, error) {
	file, err := rgaS0Parse("verify_anchored.go", src)
	if err != nil {
		return "", err
	}
	fn := rgaS0FuncBody(file, "evaluateV10")
	if fn == nil {
		return "", fmt.Errorf("evaluateV10 not found")
	}
	var found []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		expr, ok := n.(ast.Expr)
		if !ok {
			return true
		}
		value, ok := rgaS0StringLit(expr)
		if !ok {
			return true
		}
		if strings.Contains(value, "downgraded to warn") {
			found = append(found, value)
		}
		return true
	})
	if len(found) != 1 {
		return "", fmt.Errorf("expected exactly one downgrade format literal in evaluateV10, found %d", len(found))
	}
	return found[0], nil
}

// TestRGAS0VerifyV10DowngradeNoteFrozen freezes the verify-side twin of
// the ADR-029 D7 note. PRD §9.9 RGA-337 requires the new coverage check
// to receive NO such downgrade, so the exact text and its single home
// must be pinned before S5 adds a neighbouring row.
func TestRGAS0VerifyV10DowngradeNoteFrozen(t *testing.T) {
	const wantFormat = `%s (downgraded to warn: superseded by %q per ADR-029 D7 + PRD-feature-supersession §4.5 "Reconcile interaction with write-file safety")`

	got, err := rgaS0V10DowngradeFormat(rgaS0ReadRepoFile(t, "internal/workflow/verify_anchored.go"))
	if err != nil {
		t.Fatalf("locate the V10 downgrade note: %v", err)
	}
	if got != wantFormat {
		t.Fatalf("V10 downgrade format changed:\n got %q\nwant %q", got, wantFormat)
	}

	// The rendered note is what an operator reads; pin it too.
	rendered := fmt.Sprintf(got, "preimage mismatch at op #1", "newer-feature")
	wantRendered := `preimage mismatch at op #1 (downgraded to warn: superseded by "newer-feature" per ADR-029 D7 + PRD-feature-supersession §4.5 "Reconcile interaction with write-file safety")`
	if rendered != wantRendered {
		t.Fatalf("rendered V10 downgrade note changed:\n got %q\nwant %q", rendered, wantRendered)
	}

	t.Run("sensitivity", func(t *testing.T) {
		src := rgaS0ReadRepoFile(t, "internal/workflow/verify_anchored.go")
		const anchor = `downgraded to warn: superseded by %q per ADR-029 D7`
		if !strings.Contains(src, anchor) {
			t.Fatalf("mutation anchor %q no longer present", anchor)
		}
		mutated := strings.Replace(src, anchor, `downgraded to warn: superseded by %q per ADR-029 D8`, 1)
		got, err := rgaS0V10DowngradeFormat(mutated)
		if err != nil {
			t.Fatalf("mutated source should still parse: %v", err)
		}
		if got == wantFormat {
			t.Fatal("extractor is insensitive to the note text")
		}
	})

	// It must remain the ONLY verify-side downgrade note, so S5's
	// coverage row cannot silently reuse it.
	for _, rel := range rgaS0ProductionGoFiles(t) {
		body := rgaS0ReadRepoFile(t, rel)
		if n := strings.Count(body, "downgraded to warn: superseded by"); n > 0 && rel != "internal/workflow/verify_anchored.go" {
			t.Errorf("%s gained a second verify-side D7 downgrade note (%d occurrence(s))", rel, n)
		}
	}
}
