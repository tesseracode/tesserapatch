package gitutil

// GH #15 / ADR-036 slice S1 — the parser-authority guards.
//
// The S0 inventory answers "which production functions read a
// `diff --git` header". These guards answer the three questions that
// actually make the ownership rule enforceable (PRD §6.1, ADR-036 D1):
//
//   - RGA-065: is every registered ADAPTER exact? An adapter must derive
//     from the strict normalized effect set and must be able to return
//     the strict error. One that re-implements header splitting, or that
//     drops the error result, fails.
//   - RGA-066/067: does a registered NON-AUTHORITATIVE scanner stay
//     non-authoritative? `countPatchFiles` is a human file counter with
//     exactly three legitimate consumers; feeding it into an operation,
//     effect or eligibility count fails.
//   - RGA-085/086: does each PI-7 call site FAIL CLOSED? None of the
//     three had a pre-existing fail-soft handler, so each must assign the
//     strict error and return on it. Discarding it into `_`, or
//     continuing past it, fails.
//
// Every guard is a pure `func(src string) error` so the SAME matcher runs
// over deliberately mutated source in a sensitivity subtest: a guard that
// cannot fail proves nothing.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"
)

// ── RGA-065: adapter exactness ───────────────────────────────────────────

// rgaS1GrammarMarkers are the record-header tokens only the authority may
// read. An adapter carrying one of them as a literal is parsing, not
// projecting.
var rgaS1GrammarMarkers = []string{
	"diff --git",
	"+++ ",
	"--- ",
	"new file mode ",
	"deleted file mode ",
	"rename from ",
	"rename to ",
	"copy from ",
	"copy to ",
}

// rgaS1AdapterSpec is one registered adapter and the strict authority it
// must project.
type rgaS1AdapterSpec struct {
	File      string
	Func      string
	Authority string
}

var rgaS1Adapters = []rgaS1AdapterSpec{
	{File: "internal/workflow/patch_effect_adapters.go", Func: "patchEffectViews", Authority: "NormalizePatchEffects"},
	{File: "internal/workflow/patch_effect_adapters.go", Func: "strictTouchedPaths", Authority: "FilesInPatchStrict"},
	{File: "internal/workflow/hunk_overlap.go", Func: "parsePatchHunks", Authority: "NormalizePatchEffects"},
	{File: "internal/workflow/file_novelty.go", Func: "parsePatchNoveltyPaths", Authority: "patchEffectViews"},
}

// rgaS1CheckAdapter asserts that the named function projects the strict
// authority and can propagate its error.
func rgaS1CheckAdapter(spec rgaS1AdapterSpec, src string) error {
	file, err := parser.ParseFile(token.NewFileSet(), spec.File, src, parser.ParseComments)
	if err != nil {
		return err
	}
	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		d, ok := decl.(*ast.FuncDecl)
		if ok && d.Recv == nil && d.Name.Name == spec.Func && d.Body != nil {
			fn = d
		}
	}
	if fn == nil {
		return fmt.Errorf("registered adapter %s is gone from %s", spec.Func, spec.File)
	}

	// It must be able to say "I could not read the patch".
	if fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		return fmt.Errorf("%s returns no result; an adapter that cannot propagate the strict error is not an adapter", spec.Func)
	}
	last := fn.Type.Results.List[len(fn.Type.Results.List)-1]
	ident, ok := last.Type.(*ast.Ident)
	if !ok || ident.Name != "error" {
		return fmt.Errorf("%s's last result is not `error`; the strict refusal has nowhere to go", spec.Func)
	}

	// It must derive from the authority, and it must not re-implement the
	// header grammar with its own literal.
	callsAuthority := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if rgaS1CallName(call) == spec.Authority {
			callsAuthority = true
		}
		return true
	})
	if !callsAuthority {
		return fmt.Errorf("%s no longer derives from %s", spec.Func, spec.Authority)
	}

	var ownGrammar error
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value := strings.Trim(lit.Value, "`\"")
		for _, marker := range rgaS1GrammarMarkers {
			if strings.HasPrefix(value, marker) {
				ownGrammar = fmt.Errorf("%s re-implements the header grammar (literal %q); an adapter projects, it does not parse", spec.Func, value)
			}
		}
		return true
	})
	return ownGrammar
}

// rgaS1CallName renders a call's callee, reusing the identity rule the
// S0 inventory already uses.
func rgaS1CallName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}

// TestRGAS1AdaptersProjectTheStrictAuthority is RGA-065.
func TestRGAS1AdaptersProjectTheStrictAuthority(t *testing.T) {
	for _, spec := range rgaS1Adapters {
		t.Run(spec.Func, func(t *testing.T) {
			if err := rgaS1CheckAdapter(spec, rgaS0PIRead(t, spec.File)); err != nil {
				t.Fatalf("adapter contract broken: %v", err)
			}
		})
	}

	t.Run("sensitivity", func(t *testing.T) {
		spec := rgaS1Adapters[0]
		src := rgaS0PIRead(t, spec.File)
		for _, tc := range []struct{ name, old, new string }{
			{
				// The `adapter-drops-quoted-path` wrong-input fixture:
				// header splitting re-implemented instead of projected.
				name: "adapter-reimplements-header-splitting",
				old:  "\teffects, err := gitutil.NormalizePatchEffects(patch)",
				new:  "\tfor _, line := range strings.Split(patch, \"\\n\") {\n\t\t_ = strings.HasPrefix(line, \"diff --git \")\n\t}\n\teffects, err := gitutil.NormalizePatchEffects(patch)",
			},
			{
				name: "adapter-stops-deriving-from-the-authority",
				old:  "gitutil.NormalizePatchEffects(patch)",
				new:  "legacyScan(patch)",
			},
			{
				name: "adapter-drops-its-error-result",
				old:  "func patchEffectViews(patch string) ([]patchEffectView, error) {",
				new:  "func patchEffectViews(patch string) []patchEffectView {",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if !strings.Contains(src, tc.old) {
					t.Fatalf("mutation anchor no longer present:\n%q", tc.old)
				}
				if err := rgaS1CheckAdapter(spec, strings.Replace(src, tc.old, tc.new, 1)); err == nil {
					t.Fatalf("guard did not catch mutation %q", tc.name)
				}
			})
		}
	})
}

// ── RGA-066/067: the non-authoritative scanner boundary ──────────────────

// rgaS1CountPatchFilesConsumers returns `file|func` for every production
// call of `countPatchFiles` in src.
func rgaS1CountPatchFilesConsumers(relPath, src string) ([]string, error) {
	return rgaS0PICallers(relPath, src, "countPatchFiles")
}

// TestRGAS1CountPatchFilesStaysAHumanFileCount is RGA-066/RGA-068: PI-10
// keeps exactly its three legitimate FILE-count consumers, and the
// operation-count consumer stays migrated.
func TestRGAS1CountPatchFilesStaysAHumanFileCount(t *testing.T) {
	want := map[string]string{
		"internal/cli/cobra.go|recordCmd":                               "filesChanged for record.md",
		"internal/cli/feature_patch.go|runFeaturePatchAmend":            "the `%d files` field",
		"internal/cli/record_collision.go|scanCanonicalPatchCollisions": "the `Files:` field",
	}

	got := map[string]int{}
	for _, rel := range rgaS0PIProductionFiles(t) {
		callers, err := rgaS1CountPatchFilesConsumers(rel, rgaS0PIRead(t, rel))
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		for _, c := range callers {
			got[c]++
		}
	}
	for key, role := range want {
		if got[key] != 1 {
			t.Errorf("registered PI-10 consumer %q (%s) has %d call(s), want 1", key, role, got[key])
		}
	}
	for key, count := range got {
		if _, registered := want[key]; !registered {
			t.Errorf("UNREGISTERED countPatchFiles consumer %q (%d call(s)); PI-10 is a human FILE count only", key, count)
		}
	}
	total := 0
	for _, c := range got {
		total += c
	}
	if total != 3 {
		t.Errorf("countPatchFiles production call count = %d, want 3 (the operation-count consumer is migrated)", total)
	}
}

// TestRGAS1CountPatchFilesConsumerScannerIsSensitive proves the boundary
// guard would catch the `countpatchfiles-feeds-effect-count` and
// `recipe-op-count-from-file-counter` wrong-input fixtures.
func TestRGAS1CountPatchFilesConsumerScannerIsSensitive(t *testing.T) {
	src := "package cli\n" +
		"func legit(patch string) { _ = countPatchFiles(patch) }\n" +
		"func sneak(patch string, skipped []string) int { return countPatchFiles(patch) - len(skipped) }\n"
	got, err := rgaS1CountPatchFilesConsumers("synthetic.go", src)
	if err != nil {
		t.Fatalf("parse synthetic source: %v", err)
	}
	sort.Strings(got)
	want := []string{"synthetic.go|legit", "synthetic.go|sneak"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("consumers = %v, want %v", got, want)
	}
	none, err := rgaS1CountPatchFilesConsumers("synthetic.go", "package cli\n\n// countPatchFiles one day\nfunc doc() {}\n")
	if err != nil {
		t.Fatalf("parse synthetic source: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("a prose mention was counted as a consumer: %v", none)
	}
}

// ── RGA-085/086: PI-7 call sites fail closed ─────────────────────────────

// rgaS1PI7Sites is the complete production inventory of
// `PathsAffectedByPatchStrict` call sites. None of the three had a
// pre-existing fail-soft handler, so each gains a NEW refusal.
var rgaS1PI7Sites = []struct{ File, Func string }{
	{File: "internal/cli/cobra.go", Func: "runApplyExecuteChecked"},
	{File: "internal/cli/cobra.go", Func: "validateReapplyMaterialization"},
	{File: "internal/cli/feature_unapply.go", Func: "runFeatureUnapplyWithRuntime"},
}

// rgaS1CheckPI7FailsClosed asserts that the named function assigns the
// strict error to a named variable and returns on it immediately.
func rgaS1CheckPI7FailsClosed(relPath, fnName, src string) error {
	file, err := parser.ParseFile(token.NewFileSet(), relPath, src, parser.ParseComments)
	if err != nil {
		return err
	}
	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		d, ok := decl.(*ast.FuncDecl)
		if ok && d.Name.Name == fnName && d.Body != nil {
			fn = d
		}
	}
	if fn == nil {
		return fmt.Errorf("%s is gone from %s", fnName, relPath)
	}

	found := false
	var problem error
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		block, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}
		for i, stmt := range block.List {
			assign, ok := stmt.(*ast.AssignStmt)
			if !ok || len(assign.Rhs) != 1 {
				continue
			}
			call, ok := assign.Rhs[0].(*ast.CallExpr)
			if !ok || rgaS1CallName(call) != "PathsAffectedByPatchStrict" {
				continue
			}
			found = true
			if len(assign.Lhs) != 2 {
				problem = fmt.Errorf("%s: the strict call's error is not captured", fnName)
				return false
			}
			errIdent, ok := assign.Lhs[1].(*ast.Ident)
			if !ok || errIdent.Name == "_" {
				problem = fmt.Errorf("%s: the strict error is discarded into %v", fnName, assign.Lhs[1])
				return false
			}
			if i+1 >= len(block.List) {
				problem = fmt.Errorf("%s: nothing follows the strict call to refuse on", fnName)
				return false
			}
			guard, ok := block.List[i+1].(*ast.IfStmt)
			if !ok {
				problem = fmt.Errorf("%s: the strict call is not immediately followed by an error guard", fnName)
				return false
			}
			cond, ok := guard.Cond.(*ast.BinaryExpr)
			if !ok || cond.Op != token.NEQ {
				problem = fmt.Errorf("%s: the guard does not test the strict error", fnName)
				return false
			}
			lhs, ok := cond.X.(*ast.Ident)
			if !ok || lhs.Name != errIdent.Name {
				problem = fmt.Errorf("%s: the guard tests %v, not the strict error %q", fnName, cond.X, errIdent.Name)
				return false
			}
			returns := false
			ast.Inspect(guard.Body, func(inner ast.Node) bool {
				if _, ok := inner.(*ast.ReturnStmt); ok {
					returns = true
				}
				return true
			})
			if !returns {
				problem = fmt.Errorf("%s: the strict error guard does not return; the call site does not fail closed", fnName)
				return false
			}
		}
		return true
	})
	if problem != nil {
		return problem
	}
	if !found {
		return fmt.Errorf("%s no longer calls PathsAffectedByPatchStrict", fnName)
	}
	return nil
}

// TestRGAS1PI7CallSitesFailClosed is RGA-085/RGA-086.
func TestRGAS1PI7CallSitesFailClosed(t *testing.T) {
	for _, site := range rgaS1PI7Sites {
		t.Run(site.Func, func(t *testing.T) {
			if err := rgaS1CheckPI7FailsClosed(site.File, site.Func, rgaS0PIRead(t, site.File)); err != nil {
				t.Fatalf("PI-7 fail-closed contract broken: %v", err)
			}
		})
	}

	t.Run("sensitivity", func(t *testing.T) {
		site := rgaS1PI7Sites[2]
		src := rgaS0PIRead(t, site.File)
		for _, tc := range []struct{ name, old, new string }{
			{
				// The `unapply-strict-error-swallowed` wrong-input
				// fixture: the error discarded, the scope kept.
				name: "strict-error-discarded",
				old:  "\ttouched, err := gitutil.PathsAffectedByPatchStrict(patch)",
				new:  "\ttouched, _ := gitutil.PathsAffectedByPatchStrict(patch)",
			},
			{
				name: "guard-no-longer-returns",
				old:  "\t\treturn validationError(\"feature %q canonical patch is unreadable, refusing to unapply a partial path set: %v\", slug, err)",
				new:  "\t\t_ = err",
			},
			{
				// The `unapply-scope-from-b-side-projection` wrong-input
				// fixture: PI-7 migrated onto the b-side list.
				name: "scope-migrated-to-the-b-side-projection",
				old:  "gitutil.PathsAffectedByPatchStrict(patch)",
				new:  "gitutil.FilesInPatchStrict(patch)",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if !strings.Contains(src, tc.old) {
					t.Fatalf("mutation anchor no longer present:\n%q", tc.old)
				}
				if err := rgaS1CheckPI7FailsClosed(site.File, site.Func, strings.Replace(src, tc.old, tc.new, 1)); err == nil {
					t.Fatalf("guard did not catch mutation %q", tc.name)
				}
			})
		}
	})
}
