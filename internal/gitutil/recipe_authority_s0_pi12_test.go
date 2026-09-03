package gitutil

// GH #15 / ADR-036 slice S0 — frozen evidence for the patch-header
// parser inventory (PRD-recipe-generation-authority §6.1, §6.1.2, §8).
//
// S0 obligations covered here:
//
//   - "Freeze the current `FilesInPatchStrict` b-side results for its five
//     shipped callers (PI-12), so the S1 grammar extension cannot change
//     them."
//   - PI-7: prove `PathsAffectedByPatch` currently retains BOTH diff sides,
//     so S1's `PathsAffectedByPatchStrict` is written against a measured
//     baseline rather than an assumption (PRD §2.5 corrects two earlier
//     revisions that got this backwards).
//   - "Add the source-inventory guard that derives the registry from
//     production sources and fails on an unregistered `diff --git`
//     reader" — frozen at its CURRENT membership so an S1 addition or
//     removal is deliberate.
//
// The behavioural rows use synthetic-but-Git-faithful patch text so they
// run identically on every OS with no `git` subprocess. Real-Git quoting
// is separately pinned by patch_paths_strict_test.go; this file freezes
// the projection contract those bytes feed.

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

// ── PI-12: the frozen b-side projection ──────────────────────────────────

// rgaS0PI12Cases are the input shapes the five shipped `FilesInPatchStrict`
// callers actually feed it: canonical `post-apply.patch` bytes produced by
// `git diff`, covering modification, creation, deletion, rename, copy,
// mode-only, binary and C-quoted paths.
func rgaS0PI12Cases() []struct {
	Name  string
	Patch string
	Want  []string
} {
	return []struct {
		Name  string
		Patch string
		Want  []string
	}{
		{
			Name: "modify",
			Patch: "diff --git a/src/app.txt b/src/app.txt\n" +
				"index 1111111..2222222 100644\n" +
				"--- a/src/app.txt\n+++ b/src/app.txt\n" +
				"@@ -1 +1 @@\n-old\n+new\n",
			Want: []string{"src/app.txt"},
		},
		{
			Name: "create",
			Patch: "diff --git a/added.txt b/added.txt\n" +
				"new file mode 100644\nindex 0000000..3333333\n" +
				"--- /dev/null\n+++ b/added.txt\n" +
				"@@ -0,0 +1 @@\n+hello\n",
			Want: []string{"added.txt"},
		},
		{
			Name: "delete",
			Patch: "diff --git a/gone.txt b/gone.txt\n" +
				"deleted file mode 100644\nindex 4444444..0000000\n" +
				"--- a/gone.txt\n+++ /dev/null\n" +
				"@@ -1 +0,0 @@\n-bye\n",
			Want: []string{"gone.txt"},
		},
		{
			// PI-12's b-side contract: a rename yields ONLY the new
			// path. The old side is dropped — deliberately, and S1 must
			// not change it (PRD §6.1.2).
			Name: "rename-unquoted",
			Patch: "diff --git a/old.txt b/new.txt\n" +
				"similarity index 100%\nrename from old.txt\nrename to new.txt\n",
			Want: []string{"new.txt"},
		},
		{
			// Quoted operands containing spaces, resolved from the
			// two-operand header rather than the corroborating lines.
			Name: "copy-quoted-with-spaces",
			Patch: "diff --git \"a/source name.txt\" \"b/copied name.txt\"\n" +
				"similarity index 100%\ncopy from source name.txt\ncopy to copied name.txt\n",
			Want: []string{"copied name.txt"},
		},
		{
			// An unquoted path containing a literal `b/` segment: the
			// delimiter is disambiguated by payload equality.
			Name: "mode-only-path-containing-b-slash",
			Patch: "diff --git a/b/x.txt b/b/x.txt\n" +
				"old mode 100644\nnew mode 100755\n",
			Want: []string{"b/x.txt"},
		},
		{
			Name: "binary",
			Patch: "diff --git a/assets/logo.png b/assets/logo.png\n" +
				"index 5555555..6666666 100644\n" +
				"Binary files a/assets/logo.png and b/assets/logo.png differ\n",
			Want: []string{"assets/logo.png"},
		},
		{
			// Git C-quotes a path containing a newline. The strict
			// decoder recovers the byte-exact path.
			Name: "c-quoted-newline",
			Patch: "diff --git \"a/wt/new\\nline\" \"b/wt/new\\nline\"\n" +
				"new file mode 100644\n--- /dev/null\n+++ \"b/wt/new\\nline\"\n" +
				"@@ -0,0 +1 @@\n+x\n",
			Want: []string{"wt/new\nline"},
		},
		{
			// Git pads octal escapes to exactly three digits.
			Name: "c-quoted-octal-utf8",
			Patch: "diff --git \"a/caf\\303\\251.txt\" \"b/caf\\303\\251.txt\"\n" +
				"index 7777777..8888888 100644\n" +
				"@@ -1 +1 @@\n-a\n+b\n",
			Want: []string{"caf\u00e9.txt"},
		},
		{
			// Multi-record ordering and de-duplication: first-seen order,
			// each path once.
			Name: "multi-record-order-and-dedup",
			Patch: "diff --git a/z.txt b/z.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n" +
				"diff --git a/a.txt b/a.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n" +
				"diff --git a/z.txt b/z.txt\nindex 2..3 100644\n@@ -1 +1 @@\n-b\n+c\n",
			Want: []string{"z.txt", "a.txt"},
		},
	}
}

// TestRGAS0FilesInPatchStrictBSideFrozen pins the exact projection every
// shipped PI-12 caller receives today.
func TestRGAS0FilesInPatchStrictBSideFrozen(t *testing.T) {
	for _, tc := range rgaS0PI12Cases() {
		t.Run(tc.Name, func(t *testing.T) {
			got, err := FilesInPatchStrict(tc.Patch)
			if err != nil {
				t.Fatalf("FilesInPatchStrict: %v\npatch:\n%s", err, tc.Patch)
			}
			if len(got) != len(tc.Want) {
				t.Fatalf("got %q, want %q", got, tc.Want)
			}
			for i := range tc.Want {
				if got[i] != tc.Want[i] {
					t.Fatalf("index %d: got %q, want %q (full: %q)", i, got[i], tc.Want[i], got)
				}
			}
		})
	}
}

// TestRGAS0FilesInPatchStrictBSideIsSensitive proves the frozen table is a
// real measurement: the very shapes S1 will extend (rename/copy) would
// break it if the projection started returning the a-side too.
func TestRGAS0FilesInPatchStrictBSideIsSensitive(t *testing.T) {
	renamePatch := "diff --git a/old.txt b/new.txt\nsimilarity index 100%\nrename from old.txt\nrename to new.txt\n"
	got, err := FilesInPatchStrict(renamePatch)
	if err != nil {
		t.Fatalf("FilesInPatchStrict: %v", err)
	}
	for _, p := range got {
		if p == "old.txt" {
			t.Fatalf("PI-12's b-side contract already returns the rename SOURCE; the S0 freeze is stale: %q", got)
		}
	}
	// And a shape the strict grammar refuses stays refused, so "no error"
	// is not the default outcome of this helper.
	if _, err := FilesInPatchStrict("this is not a patch at all\n"); err == nil {
		t.Fatal("headerless non-blank input must be refused, else the b-side table proves nothing")
	}
}

// ── PI-7: the both-sides union, measured ─────────────────────────────────

// TestRGAS0PathsAffectedByPatchRetainsOldAndNewSides freezes the fact PRD
// §2.5 had to correct twice: `PathsAffectedByPatch` deliberately returns
// the union of BOTH diff sides plus rename/copy source and destination.
// S1's `PathsAffectedByPatchStrict` must preserve that union; replacing it
// with the b-side projection would silently drop every rename and copy
// SOURCE from unapply's snapshot and rollback scope.
func TestRGAS0PathsAffectedByPatchRetainsOldAndNewSides(t *testing.T) {
	cases := []struct {
		name       string
		patch      string
		wantUnion  []string
		wantBSide  []string
		bSideDrops []string
	}{
		{
			name:       "rename-unquoted",
			patch:      "diff --git a/old.txt b/new.txt\nsimilarity index 100%\nrename from old.txt\nrename to new.txt\n",
			wantUnion:  []string{"new.txt", "old.txt"},
			wantBSide:  []string{"new.txt"},
			bSideDrops: []string{"old.txt"},
		},
		{
			name:       "copy-quoted-with-spaces",
			patch:      "diff --git \"a/source name.txt\" \"b/copied name.txt\"\nsimilarity index 100%\ncopy from source name.txt\ncopy to copied name.txt\n",
			wantUnion:  []string{"copied name.txt", "source name.txt"},
			wantBSide:  []string{"copied name.txt"},
			bSideDrops: []string{"source name.txt"},
		},
		{
			name: "modify-has-one-path-on-both-sides",
			patch: "diff --git a/src/app.txt b/src/app.txt\nindex 1..2 100644\n" +
				"--- a/src/app.txt\n+++ b/src/app.txt\n@@ -1 +1 @@\n-a\n+b\n",
			wantUnion: []string{"src/app.txt"},
			wantBSide: []string{"src/app.txt"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			union := append([]string(nil), PathsAffectedByPatch(tc.patch)...)
			sort.Strings(union)
			want := append([]string(nil), tc.wantUnion...)
			sort.Strings(want)
			if strings.Join(union, "|") != strings.Join(want, "|") {
				t.Fatalf("union = %q, want %q", union, want)
			}

			bSide, err := FilesInPatchStrict(tc.patch)
			if err != nil {
				t.Fatalf("FilesInPatchStrict: %v", err)
			}
			sorted := append([]string(nil), bSide...)
			sort.Strings(sorted)
			wantB := append([]string(nil), tc.wantBSide...)
			sort.Strings(wantB)
			if strings.Join(sorted, "|") != strings.Join(wantB, "|") {
				t.Fatalf("b-side = %q, want %q", sorted, wantB)
			}

			for _, dropped := range tc.bSideDrops {
				for _, p := range bSide {
					if p == dropped {
						t.Fatalf("b-side unexpectedly retains %q; the PI-7 divergence this freezes is gone", dropped)
					}
				}
			}
		})
	}
}

// TestRGAS0PathsAffectedByPatchUsesAGoLiteralDecoder freezes the second
// PI-7 divergence S1 must resolve deliberately: the union decodes operands
// with `strconv.Unquote`, a GO-literal decoder, so it accepts escapes Git
// never emits — exactly the bytes the strict grammar refuses.
func TestRGAS0PathsAffectedByPatchUsesAGoLiteralDecoder(t *testing.T) {
	patch := "diff --git \"a/\\x41.txt\" \"b/\\x41.txt\"\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n"

	if _, err := FilesInPatchStrict(patch); err == nil {
		t.Fatal("the strict grammar must refuse a Go-only \\x escape")
	}
	got := PathsAffectedByPatch(patch)
	if len(got) != 1 || got[0] != "A.txt" {
		t.Fatalf("union currently decodes the Go-only escape to %q, want [A.txt]", got)
	}
}

// ── source-derived reader / caller inventory ─────────────────────────────

func rgaS0PIRepoRoot(t *testing.T) string {
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

// rgaS0PIProductionFiles lists every non-test Go file in the module's own
// source trees, module-relative and slash-separated.
func rgaS0PIProductionFiles(t *testing.T) []string {
	t.Helper()
	root := rgaS0PIRepoRoot(t)
	var out []string
	for _, tree := range []string{"internal", "cmd", "assets"} {
		err := filepath.WalkDir(filepath.Join(root, tree), func(path string, d fs.DirEntry, err error) error {
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

func rgaS0PIRead(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(rgaS0PIRepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(body)
}

func rgaS0PIFuncName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	var buf strings.Builder
	buf.WriteString("(")
	rgaS0PIType(&buf, fn.Recv.List[0].Type)
	buf.WriteString(").")
	buf.WriteString(fn.Name.Name)
	return buf.String()
}

func rgaS0PIType(buf *strings.Builder, expr ast.Expr) {
	switch t := expr.(type) {
	case *ast.StarExpr:
		buf.WriteString("*")
		rgaS0PIType(buf, t.X)
	case *ast.Ident:
		buf.WriteString(t.Name)
	case *ast.SelectorExpr:
		rgaS0PIType(buf, t.X)
		buf.WriteString(".")
		buf.WriteString(t.Sel.Name)
	default:
		buf.WriteString("?")
	}
}

// rgaS0PIReaders returns `file|func` for every function in src whose body
// contains a STRING LITERAL beginning with `diff --git`. Scoping to
// literals (not comments, not identifiers) is what makes this a semantic
// inventory of patch-header readers rather than a prose grep.
func rgaS0PIReaders(relPath, src string) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), relPath, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		owner := relPath + "|" + rgaS0PIFuncName(fn)
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, uerr := strconv.Unquote(lit.Value)
			if uerr != nil {
				return true
			}
			if strings.HasPrefix(value, "diff --git") && !seen[owner] {
				seen[owner] = true
				out = append(out, owner)
			}
			return true
		})
	}
	sort.Strings(out)
	return out, nil
}

// rgaS0PICallers returns `file|func` for every production call of the
// named package-level function.
func rgaS0PICallers(relPath, src, symbol string) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), relPath, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		owner := relPath + "|" + rgaS0PIFuncName(fn)
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := ""
			switch callee := call.Fun.(type) {
			case *ast.Ident:
				name = callee.Name
			case *ast.SelectorExpr:
				name = callee.Sel.Name
			}
			if name == symbol {
				out = append(out, owner)
			}
			return true
		})
	}
	return out, nil
}

// TestRGAS0PatchHeaderReaderInventory freezes the CURRENT registry of
// production `diff --git` readers (PRD §2.5 + §6.1). S1 removes PI-1,
// demotes PI-2 and migrates PI-3..PI-7; every one of those edits must
// move a line here.
func TestRGAS0PatchHeaderReaderInventory(t *testing.T) {
	want := map[string]string{
		// authoritative strict grammar (PI-12)
		"internal/gitutil/patch_paths_strict.go|FilesInPatchStrict": "PI-12 strict authority",
		// fail-soft scanner and its two production consumers (PI-2/3/4)
		"internal/gitutil/gitutil.go|FilesInPatch": "PI-2 fail-soft scanner",
		// both-sides union (PI-7)
		"internal/gitutil/unapply.go|PathsAffectedByPatch": "PI-7 union",
		// naive path/effect readers slated for adapter migration (PI-1/5/6)
		"internal/workflow/recipe_autogen.go|parsePatchTouchedFiles": "PI-1 recipe autogen",
		"internal/workflow/file_novelty.go|parsePatchNoveltyPaths":   "PI-5 novelty classifier",
		"internal/workflow/hunk_overlap.go|parsePatchHunks":          "PI-6 hunk attribution",
		// registered NON-authoritative scanners (PI-8/9/10)
		"internal/store/store.go|headerReferencedGitPath":         "PI-8 .git containment",
		"internal/gitutil/gitutil.go|stripGitInternalFileStanzas": "PI-9 sanitization",
		"internal/gitutil/gitutil.go|headerPathIsGitInternal":     "PI-9 sanitization",
		"internal/cli/cobra.go|countPatchFiles":                   "PI-10 display counter",
	}

	got := map[string]bool{}
	for _, rel := range rgaS0PIProductionFiles(t) {
		readers, err := rgaS0PIReaders(rel, rgaS0PIRead(t, rel))
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		for _, r := range readers {
			got[r] = true
		}
	}

	for key, role := range want {
		if !got[key] {
			t.Errorf("registered %s reader %q disappeared from production", role, key)
		}
	}
	for key := range got {
		if _, registered := want[key]; !registered {
			t.Errorf("UNREGISTERED `diff --git` reader %q; register it in the PRD §6.1 inventory before shipping it", key)
		}
	}
	if len(got) != len(want) {
		t.Errorf("production `diff --git` reader count = %d, want %d", len(got), len(want))
	}
}

// TestRGAS0PatchHeaderReaderScannerIsSensitive proves the inventory would
// catch a newly introduced reader and would not be fooled by prose.
func TestRGAS0PatchHeaderReaderScannerIsSensitive(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{
			name: "detects-a-new-reader",
			src: "package p\nimport \"strings\"\nfunc sneak(patch string) {\n" +
				"\tfor _, l := range strings.Split(patch, \"\\n\") {\n" +
				"\t\tif strings.HasPrefix(l, \"diff --git \") {\n\t\t\t_ = l\n\t\t}\n\t}\n}\n",
			want: 1,
		},
		{
			name: "ignores-a-comment-only-mention",
			src:  "package p\n\n// splits on diff --git headers one day\nfunc doc() {}\n",
			want: 0,
		},
		{
			name: "ignores-an-unrelated-literal",
			src:  "package p\nfunc other() { _ = \"diff -u a b\" }\n",
			want: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rgaS0PIReaders("synthetic.go", tc.src)
			if err != nil {
				t.Fatalf("parse synthetic source: %v", err)
			}
			if len(got) != tc.want {
				t.Fatalf("readers = %v, want %d", got, tc.want)
			}
		})
	}
}

// TestRGAS0PatchParserCallsiteInventory freezes WHO calls each parser.
// PRD §8 S1 names five `FilesInPatchStrict` callers (PI-12), two
// `FilesInPatch` callers (PI-3, PI-4) and three `PathsAffectedByPatch`
// call sites (PI-7). Identity is (file, enclosing function), never a line
// number, so ordinary edits above a call do not churn this guard.
func TestRGAS0PatchParserCallsiteInventory(t *testing.T) {
	want := map[string]map[string]int{
		"FilesInPatchStrict": {
			"internal/cli/land.go|computePathSet":                                  1,
			"internal/cli/land.go|runLandDryRun":                                   1,
			"internal/workflow/refresh.go|RefreshAfterAccept":                      1,
			"internal/workflow/verify_landed.go|(*verifyRunContext).identitiesFor": 1,
			"internal/workflow/verify_landed.go|firstPatchPath":                    1,
		},
		"FilesInPatch": {
			"internal/workflow/patch_generations.go|AppendPatchGenerationForFeature":   1,
			"internal/workflow/reconcile_derivation.go|touchedPathsFromPostApplyPatch": 1,
		},
		"PathsAffectedByPatch": {
			"internal/cli/cobra.go|runApplyExecuteChecked":                 1,
			"internal/cli/cobra.go|validateReapplyMaterialization":         1,
			"internal/cli/feature_unapply.go|runFeatureUnapplyWithRuntime": 1,
		},
	}
	wantTotals := map[string]int{
		"FilesInPatchStrict":   5,
		"FilesInPatch":         2,
		"PathsAffectedByPatch": 3,
	}

	files := rgaS0PIProductionFiles(t)
	for symbol, expected := range want {
		got := map[string]int{}
		total := 0
		for _, rel := range files {
			// The declaration site is not a call site.
			callers, err := rgaS0PICallers(rel, rgaS0PIRead(t, rel), symbol)
			if err != nil {
				t.Fatalf("parse %s: %v", rel, err)
			}
			for _, c := range callers {
				got[c]++
				total++
			}
		}
		if total != wantTotals[symbol] {
			t.Errorf("%s production call count = %d, want %d (sites: %v)", symbol, total, wantTotals[symbol], got)
		}
		for key, count := range expected {
			if got[key] != count {
				t.Errorf("%s call site %q: got %d, want %d", symbol, key, got[key], count)
			}
		}
		for key, count := range got {
			if _, registered := expected[key]; !registered {
				t.Errorf("UNREGISTERED %s call site %q (%d call(s)); PI-12/PI-2/PI-7 migrations must be deliberate", symbol, key, count)
			}
		}
	}
}

// TestRGAS0PatchParserCallsiteScannerIsSensitive proves the caller
// inventory detects an added call and attributes it to the right owner.
func TestRGAS0PatchParserCallsiteScannerIsSensitive(t *testing.T) {
	src := "package p\n" +
		"func owner(patch string) {\n\t_, _ = gitutil.FilesInPatchStrict(patch)\n}\n" +
		"func (r *T) method(patch string) {\n\t_, _ = gitutil.FilesInPatchStrict(patch)\n}\n" +
		"func unrelated(patch string) {\n\t_ = gitutil.FilesInPatch(patch)\n}\n"

	strictCallers, err := rgaS0PICallers("synthetic.go", src, "FilesInPatchStrict")
	if err != nil {
		t.Fatalf("parse synthetic source: %v", err)
	}
	sort.Strings(strictCallers)
	want := []string{"synthetic.go|(*T).method", "synthetic.go|owner"}
	if fmt.Sprint(strictCallers) != fmt.Sprint(want) {
		t.Fatalf("strict callers = %v, want %v", strictCallers, want)
	}

	softCallers, err := rgaS0PICallers("synthetic.go", src, "FilesInPatch")
	if err != nil {
		t.Fatalf("parse synthetic source: %v", err)
	}
	if len(softCallers) != 1 || softCallers[0] != "synthetic.go|unrelated" {
		t.Fatalf("fail-soft callers = %v, want [synthetic.go|unrelated]", softCallers)
	}
	// A prose mention must not be counted.
	none, err := rgaS0PICallers("synthetic.go", "package p\n\n// calls FilesInPatchStrict some day\nfunc doc() {}\n", "FilesInPatchStrict")
	if err != nil {
		t.Fatalf("parse synthetic source: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("comment counted as a call site: %v", none)
	}
}
