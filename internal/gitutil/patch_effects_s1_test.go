package gitutil

// GH #15 / ADR-036 slice S1 — the strict normalized effect grammar.
//
// Rows here cover PRD-recipe-generation-authority §6.1 and the RGA-063,
// RGA-076..RGA-080, RGA-092..RGA-097 matrix entries:
//
//   - the normalized effect model and its three orthogonal kind axes;
//   - `patch_fragment_sha256` over the effect's exact raw byte range,
//     including the embedded-`diff --git` case the grammar closes
//     structurally;
//   - `PathsAffectedByPatchStrict`'s both-side union, including rename and
//     copy SOURCES, and its refusal parity with `FilesInPatchStrict`;
//   - the contradiction and path-safety refusals.
//
// Fixtures are synthetic-but-Git-faithful so every row runs identically on
// every OS with no `git` subprocess. Real-Git quoting is separately pinned
// by patch_paths_strict_test.go.

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"testing"
)

func s1Effects(t *testing.T, patch string) []PatchEffect {
	t.Helper()
	effects, err := NormalizePatchEffects(patch)
	if err != nil {
		t.Fatalf("NormalizePatchEffects: %v\npatch:\n%s", err, patch)
	}
	return effects
}

// TestS1NormalizedEffectAxes covers RGA-092/RGA-093 and the orthogonality
// rule: change, content and object kinds are decided independently, and a
// bare parse (no observation) is truthful about what it did not look at.
func TestS1NormalizedEffectAxes(t *testing.T) {
	cases := []struct {
		name        string
		patch       string
		wantChange  ChangeKind
		wantContent ContentKind
		wantPath    string
		wantOldPath string
		wantHdrOld  string
		wantHdrNew  string
	}{
		{
			name: "ordinary-100644-add",
			patch: "diff --git a/added.txt b/added.txt\nnew file mode 100644\nindex 0000000..3333333\n" +
				"--- /dev/null\n+++ b/added.txt\n@@ -0,0 +1 @@\n+hello\n",
			wantChange:  ChangeKindAdd,
			wantContent: ContentKindUnknown,
			wantPath:    "added.txt",
			wantHdrOld:  "",
			wantHdrNew:  "100644",
		},
		{
			name: "ordinary-100644-modify",
			patch: "diff --git a/command.go b/command.go\nindex 1111111..2222222 100644\n" +
				"--- a/command.go\n+++ b/command.go\n@@ -1 +1 @@\n-old\n+new\n",
			wantChange:  ChangeKindModify,
			wantContent: ContentKindUnknown,
			wantPath:    "command.go",
			wantHdrOld:  "100644",
			wantHdrNew:  "100644",
		},
		{
			name: "delete",
			patch: "diff --git a/gone.txt b/gone.txt\ndeleted file mode 100644\nindex 4444444..0000000\n" +
				"--- a/gone.txt\n+++ /dev/null\n@@ -1 +0,0 @@\n-bye\n",
			wantChange:  ChangeKindDelete,
			wantContent: ContentKindUnknown,
			wantPath:    "gone.txt",
			wantHdrOld:  "100644",
			wantHdrNew:  "",
		},
		{
			name: "binary-rename-is-rename-plus-binary",
			patch: "diff --git a/old.bin b/new.bin\nsimilarity index 60%\nrename from old.bin\nrename to new.bin\n" +
				"index 1111111..2222222 100644\nGIT binary patch\nliteral 4\nzcmZQ\n\n",
			wantChange:  ChangeKindRename,
			wantContent: ContentKindBinary,
			wantPath:    "new.bin",
			wantOldPath: "old.bin",
			wantHdrOld:  "100644",
			wantHdrNew:  "100644",
		},
		{
			name: "executable-mode-transition",
			patch: "diff --git a/run.sh b/run.sh\nold mode 100644\nnew mode 100755\n" +
				"index 1111111..2222222\n",
			wantChange:  ChangeKindModify,
			wantContent: ContentKindUnknown,
			wantPath:    "run.sh",
			wantHdrOld:  "100644",
			wantHdrNew:  "100755",
		},
		{
			name: "symlink-add",
			patch: "diff --git a/link b/link\nnew file mode 120000\nindex 0000000..1111111\n" +
				"--- /dev/null\n+++ b/link\n@@ -0,0 +1 @@\n+target.txt\n\\ No newline at end of file\n",
			wantChange:  ChangeKindAdd,
			wantContent: ContentKindUnknown,
			wantPath:    "link",
			wantHdrNew:  "120000",
		},
		{
			name: "gitlink-add",
			patch: "diff --git a/vendor/sub b/vendor/sub\nnew file mode 160000\nindex 0000000..1111111\n" +
				"--- /dev/null\n+++ b/vendor/sub\n@@ -0,0 +1 @@\n+Subproject commit 1111111\n",
			wantChange:  ChangeKindAdd,
			wantContent: ContentKindUnknown,
			wantPath:    "vendor/sub",
			wantHdrNew:  "160000",
		},
		{
			name: "binary-files-differ-marker",
			patch: "diff --git a/assets/logo.png b/assets/logo.png\nindex 5555555..6666666 100644\n" +
				"Binary files a/assets/logo.png and b/assets/logo.png differ\n",
			wantChange:  ChangeKindModify,
			wantContent: ContentKindBinary,
			wantPath:    "assets/logo.png",
			wantHdrOld:  "100644",
			wantHdrNew:  "100644",
		},
		{
			name:        "copy-keeps-its-source",
			patch:       "diff --git a/src.txt b/dst.txt\nsimilarity index 100%\ncopy from src.txt\ncopy to dst.txt\n",
			wantChange:  ChangeKindCopy,
			wantContent: ContentKindUnknown,
			wantPath:    "dst.txt",
			wantOldPath: "src.txt",
		},
		{
			name: "c-quoted-path-with-a-space",
			patch: "diff --git \"a/sp ace\\ttab.txt\" \"b/sp ace\\ttab.txt\"\nindex 1..2 100644\n" +
				"@@ -1 +1 @@\n-a\n+b\n",
			wantChange:  ChangeKindModify,
			wantContent: ContentKindUnknown,
			wantPath:    "sp ace\ttab.txt",
			wantHdrOld:  "100644",
			wantHdrNew:  "100644",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			effects := s1Effects(t, tc.patch)
			if len(effects) != 1 {
				t.Fatalf("want exactly one effect, got %d: %+v", len(effects), effects)
			}
			e := effects[0]
			if e.Ordinal != 1 {
				t.Errorf("ordinal = %d, want 1", e.Ordinal)
			}
			if e.ChangeKind != tc.wantChange {
				t.Errorf("change_kind = %q, want %q", e.ChangeKind, tc.wantChange)
			}
			if e.ContentKind != tc.wantContent {
				t.Errorf("content_kind = %q, want %q", e.ContentKind, tc.wantContent)
			}
			// A bare parse observed no tree, so object_kind is unknown and
			// both modes are "" — the header is corroboration, never a
			// substitute for having looked.
			if e.ObjectKind != ObjectKindUnknown {
				t.Errorf("object_kind = %q, want unknown for an unobserved parse", e.ObjectKind)
			}
			if e.OldMode != "" || e.NewMode != "" {
				t.Errorf("observed modes = (%q,%q), want both empty for an unobserved parse", e.OldMode, e.NewMode)
			}
			if e.PreimageObserved || e.PostimageObserved {
				t.Errorf("a bare parse must not claim to have observed either side: %+v", e)
			}
			if e.Path != tc.wantPath {
				t.Errorf("path = %q, want %q", e.Path, tc.wantPath)
			}
			if e.OldPath != tc.wantOldPath {
				t.Errorf("old_path = %q, want %q", e.OldPath, tc.wantOldPath)
			}
			if e.HeaderOldMode != tc.wantHdrOld || e.HeaderNewMode != tc.wantHdrNew {
				t.Errorf("header modes = (%q,%q), want (%q,%q)", e.HeaderOldMode, e.HeaderNewMode, tc.wantHdrOld, tc.wantHdrNew)
			}
			// `old_path` is non-empty exactly for rename and copy.
			wantOldPathPresent := tc.wantChange == ChangeKindRename || tc.wantChange == ChangeKindCopy
			if (e.OldPath != "") != wantOldPathPresent {
				t.Errorf("old_path presence = %v, want %v for change_kind %q", e.OldPath != "", wantOldPathPresent, e.ChangeKind)
			}
		})
	}
}

// TestS1EffectOrdinalsAreGapless covers ADR-036 D3: ordinals are the
// one-based positions of the grammar's own recognized record starts,
// counted from the first byte, with no gaps and no sort of the producer's
// choosing.
func TestS1EffectOrdinalsAreGapless(t *testing.T) {
	patch := "diff --git a/z.txt b/z.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n" +
		"diff --git a/a.txt b/a.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n" +
		"diff --git a/m.txt b/m.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n"
	effects := s1Effects(t, patch)
	wantPaths := []string{"z.txt", "a.txt", "m.txt"}
	if len(effects) != len(wantPaths) {
		t.Fatalf("got %d effects, want %d", len(effects), len(wantPaths))
	}
	for i, e := range effects {
		if e.Ordinal != i+1 {
			t.Errorf("effect %d: ordinal = %d, want %d", i, e.Ordinal, i+1)
		}
		if e.Path != wantPaths[i] {
			t.Errorf("effect %d: path = %q, want %q (ordinals follow the patch, not a sort)", i, e.Path, wantPaths[i])
		}
	}
}

// TestS1FragmentDigestsCoverExactRawBytes covers ADR-036 D3's
// `patch_fragment_sha256` definition: the digest is over the exact raw
// bytes from the record's first byte to the byte before the next record's
// first byte, verbatim — CRLF retained, no-newline markers retained.
func TestS1FragmentDigestsCoverExactRawBytes(t *testing.T) {
	first := "diff --git a/one.txt b/one.txt\r\nindex 1..2 100644\r\n@@ -1 +1 @@\r\n-a\r\n+b\r\n"
	second := "diff --git a/two.txt b/two.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n\\ No newline at end of file\n"
	patch := first + second

	effects := s1Effects(t, patch)
	if len(effects) != 2 {
		t.Fatalf("got %d effects, want 2: %+v", len(effects), effects)
	}
	if effects[0].FragmentStart != 0 || effects[0].FragmentEnd != len(first) {
		t.Fatalf("fragment 1 range = [%d,%d), want [0,%d)", effects[0].FragmentStart, effects[0].FragmentEnd, len(first))
	}
	if effects[1].FragmentStart != len(first) || effects[1].FragmentEnd != len(patch) {
		t.Fatalf("fragment 2 range = [%d,%d), want [%d,%d)", effects[1].FragmentStart, effects[1].FragmentEnd, len(first), len(patch))
	}
	for i, want := range []string{first, second} {
		sum := sha256.Sum256([]byte(want))
		if got := effects[i].FragmentSHA256; got != hex.EncodeToString(sum[:]) {
			t.Errorf("fragment %d digest = %s, want the digest of its exact raw bytes", i+1, got)
		}
	}

	// The fragments partition the patch exactly: concatenating them
	// reproduces the input byte-for-byte.
	var rebuilt strings.Builder
	for _, e := range effects {
		rebuilt.WriteString(patch[e.FragmentStart:e.FragmentEnd])
	}
	if rebuilt.String() != patch {
		t.Fatalf("fragments do not partition the patch:\n got %q\nwant %q", rebuilt.String(), patch)
	}

	// A CRLF body hashes differently from an LF body.
	lf := strings.ReplaceAll(first, "\r\n", "\n")
	lfEffects := s1Effects(t, lf)
	if lfEffects[0].FragmentSHA256 == effects[0].FragmentSHA256 {
		t.Fatal("CRLF and LF bodies produced the same fragment digest; the fragment is being normalized")
	}
}

// TestS1EmbeddedDiffGitInHunkBodyDoesNotOpenARecord covers the
// embedded-token case ADR-036 D3 closes structurally: inside a valid hunk
// every body line carries a `+`, `-` or space prefix, so an added line
// whose content is a `diff --git` header is not at line start and cannot
// be a record boundary.
func TestS1EmbeddedDiffGitInHunkBodyDoesNotOpenARecord(t *testing.T) {
	patch := "diff --git a/doc.md b/doc.md\nindex 1..2 100644\n--- a/doc.md\n+++ b/doc.md\n" +
		"@@ -1,2 +1,4 @@\n intro\n+diff --git a/fake.txt b/fake.txt\n+new file mode 100644\n outro\n" +
		"diff --git a/real.txt b/real.txt\nnew file mode 100644\n--- /dev/null\n+++ b/real.txt\n@@ -0,0 +1 @@\n+x\n"

	effects := s1Effects(t, patch)
	if len(effects) != 2 {
		t.Fatalf("got %d effects, want exactly 2 (doc.md and real.txt): %+v", len(effects), effects)
	}
	if effects[0].Path != "doc.md" || effects[1].Path != "real.txt" {
		t.Fatalf("paths = %q/%q, want doc.md/real.txt", effects[0].Path, effects[1].Path)
	}
	if effects[1].ChangeKind != ChangeKindAdd {
		t.Fatalf("real.txt change_kind = %q, want add", effects[1].ChangeKind)
	}
	// The embedded header lives INSIDE fragment 1, which is exactly what
	// a substring scan would have got wrong.
	fragment := patch[effects[0].FragmentStart:effects[0].FragmentEnd]
	if !strings.Contains(fragment, "+diff --git a/fake.txt b/fake.txt") {
		t.Fatalf("the embedded header is not inside fragment 1:\n%s", fragment)
	}
	if strings.Contains(fragment, "\ndiff --git a/real.txt") {
		t.Fatal("fragment 1 swallowed the next real record")
	}
	// And it never became a path.
	for _, e := range effects {
		if e.Path == "fake.txt" {
			t.Fatal("an added hunk line opened a record; the grammar is scanning substrings")
		}
	}
}

// TestS1PathsAffectedByPatchStrictUnion covers RGA-076..RGA-079: the
// sorted unique union of canonical path plus old_path for rename and
// copy, with quoted paths decoded byte-correctly on both sides.
func TestS1PathsAffectedByPatchStrictUnion(t *testing.T) {
	cases := []struct {
		name  string
		patch string
		want  []string
	}{
		{
			name:  "rename-retains-its-source",
			patch: "diff --git a/old.txt b/new.txt\nsimilarity index 100%\nrename from old.txt\nrename to new.txt\n",
			want:  []string{"new.txt", "old.txt"},
		},
		{
			name:  "copy-retains-its-source",
			patch: "diff --git a/src.txt b/dst.txt\nsimilarity index 100%\ncopy from src.txt\ncopy to dst.txt\n",
			want:  []string{"dst.txt", "src.txt"},
		},
		{
			name: "quoted-rename-with-spaces-on-both-sides",
			patch: "diff --git \"a/old name.txt\" \"b/new name.txt\"\nsimilarity index 100%\n" +
				"rename from old name.txt\nrename to new name.txt\n",
			want: []string{"new name.txt", "old name.txt"},
		},
		{
			name: "mixed-quoted-rename",
			patch: "diff --git a/readme.md \"b/l\\303\\251eme.md\"\nsimilarity index 100%\n" +
				"rename from readme.md\nrename to \"l\\303\\251eme.md\"\n",
			want: []string{"léeme.md", "readme.md"},
		},
		{
			name: "unquoted-space-to-quoted-rename",
			patch: "diff --git a/my file.txt \"b/caf\\303\\251.txt\"\nsimilarity index 100%\n" +
				"rename from my file.txt\nrename to \"caf\\303\\251.txt\"\n",
			want: []string{"café.txt", "my file.txt"},
		},
		{
			name: "quoted-to-unquoted-space-rename",
			patch: "diff --git \"a/caf\\303\\251.txt\" b/my file.txt\nsimilarity index 100%\n" +
				"rename from \"caf\\303\\251.txt\"\nrename to my file.txt\n",
			want: []string{"café.txt", "my file.txt"},
		},
		{
			name: "modify-has-one-path",
			patch: "diff --git a/src/app.txt b/src/app.txt\nindex 1..2 100644\n" +
				"--- a/src/app.txt\n+++ b/src/app.txt\n@@ -1 +1 @@\n-a\n+b\n",
			want: []string{"src/app.txt"},
		},
		{
			// Uniqueness is across the two SIDES of the effect set: the
			// rename source `a.txt` and the new file at the same path
			// collapse to one entry. Two records naming the same
			// DESTINATION are a different shape entirely and are refused
			// (see TestS1DuplicateDestinationsAreRefused).
			name: "multiple-records-are-sorted-and-unique",
			patch: "diff --git a/z.txt b/z.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n" +
				"diff --git a/a.txt b/b.txt\nsimilarity index 100%\nrename from a.txt\nrename to b.txt\n" +
				"diff --git a/a.txt b/a.txt\nnew file mode 100644\n--- /dev/null\n+++ b/a.txt\n@@ -0,0 +1 @@\n+new\n",
			want: []string{"a.txt", "b.txt", "z.txt"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PathsAffectedByPatchStrict(tc.patch)
			if err != nil {
				t.Fatalf("PathsAffectedByPatchStrict: %v", err)
			}
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("union = %q, want %q (sorted, unique)", got, tc.want)
			}
			if !sort.StringsAreSorted(got) {
				t.Fatalf("union is not sorted: %q", got)
			}
		})
	}
}

// TestS1StrictProjectionsAgreeOnRefusals covers RGA-080: the all-paths
// projection refuses the inputs the b-side projection refuses, returning
// an error and a nil slice rather than a partial scope.
//
// There is exactly ONE deliberate divergence between the two, and it is
// asserted separately in TestS1DuplicateDestinationsAreRefused: a
// repeated destination path is refused by the authority and accepted —
// de-duplicated in first-seen order — by the frozen PI-12 projection. The
// corpus below therefore carries no duplicate-destination input; adding
// one here would assert parity the contract does not have.
func TestS1StrictProjectionsAgreeOnRefusals(t *testing.T) {
	corpus := []string{
		// refusals
		"this is not a patch at all\n",
		"@@ -1 +1 @@\n-a\n+b\n",
		"diff --git a/foo.txt\nindex 1..2 100644\n",
		"diff --git \"a/foo.txt b/foo.txt\nindex 1..2 100644\n",
		"diff --git \"a/x\\x41.txt\" \"b/x\\x41.txt\"\nindex 1..2 100644\n",
		"diff --git a/old name b/new name\nindex 1..2 100644\n",
		"diff --git a/a.txt b/a.txt\nnew file mode 100644\ndeleted file mode 100644\n",
		"diff --git a/../escape.txt b/../escape.txt\nindex 1..2 100644\n",
		// accepted
		"",
		"   \n\t\n",
		"diff --git a/x.txt b/x.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n",
		"diff --git a/old.txt b/new.txt\nsimilarity index 100%\nrename from old.txt\nrename to new.txt\n",
	}
	for _, patch := range corpus {
		bSide, bErr := FilesInPatchStrict(patch)
		union, uErr := PathsAffectedByPatchStrict(patch)
		if (bErr == nil) != (uErr == nil) {
			t.Fatalf("refusal parity broken for %q: b-side err=%v union err=%v", patch, bErr, uErr)
		}
		if uErr != nil {
			if union != nil {
				t.Fatalf("a refusal must return a nil scope, got %q for %q", union, patch)
			}
			if bSide != nil {
				t.Fatalf("a refusal must return a nil scope, got %q for %q", bSide, patch)
			}
		}
	}
}

// TestS1GrammarRefusesContradictionsAndUnsafePaths covers RGA-097 and the
// path-safety rule: an interpretation that cannot be completed refuses
// generation authority rather than falling back to a partial scan.
func TestS1GrammarRefusesContradictionsAndUnsafePaths(t *testing.T) {
	cases := []struct {
		name  string
		patch string
	}{
		{
			name:  "new-and-deleted-together",
			patch: "diff --git a/x.txt b/x.txt\nnew file mode 100644\ndeleted file mode 100644\n",
		},
		{
			name: "rename-and-copy-together",
			patch: "diff --git a/x.txt b/y.txt\nrename from x.txt\nrename to y.txt\n" +
				"copy from x.txt\ncopy to y.txt\n",
		},
		{
			name:  "rename-missing-its-destination",
			patch: "diff --git a/x.txt b/y.txt\nsimilarity index 100%\nrename from x.txt\n",
		},
		{
			name: "add-declaring-a-preimage-side",
			patch: "diff --git a/x.txt b/x.txt\nnew file mode 100644\n--- a/x.txt\n+++ b/x.txt\n" +
				"@@ -0,0 +1 @@\n+x\n",
		},
		{
			name: "delete-declaring-a-postimage-side",
			patch: "diff --git a/x.txt b/x.txt\ndeleted file mode 100644\n--- a/x.txt\n+++ b/x.txt\n" +
				"@@ -1 +0,0 @@\n-x\n",
		},
		{
			name: "modify-declaring-a-dev-null-side",
			patch: "diff --git a/x.txt b/x.txt\nindex 1..2 100644\n--- /dev/null\n+++ b/x.txt\n" +
				"@@ -0,0 +1 @@\n+x\n",
		},
		{
			name: "header-operands-contradict-the-rename-operands",
			patch: "diff --git \"a/one.txt\" \"b/two.txt\"\nsimilarity index 100%\n" +
				"rename from other.txt\nrename to two.txt\n",
		},
		{
			name:  "sides-differ-with-no-rename-or-copy-corroboration",
			patch: "diff --git \"a/one.txt\" \"b/two.txt\"\nindex 1..2 100644\n",
		},
		{
			name:  "parent-escaping-path",
			patch: "diff --git a/../escape.txt b/../escape.txt\nindex 1..2 100644\n",
		},
		{
			name:  "dot-segment-path",
			patch: "diff --git a/./x.txt b/./x.txt\nindex 1..2 100644\n",
		},
		{
			name:  "rename-source-escapes-the-repository",
			patch: "diff --git a/x.txt b/y.txt\nsimilarity index 100%\nrename from ../x.txt\nrename to y.txt\n",
		},
		{
			name:  "file-mode-is-not-six-octal-digits",
			patch: "diff --git a/x.txt b/x.txt\nnew file mode 10064z\n--- /dev/null\n+++ b/x.txt\n@@ -0,0 +1 @@\n+x\n",
		},
		{
			name:  "drive-rooted-path",
			patch: "diff --git a/C:/x.txt b/C:/x.txt\nindex 1..2 100644\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			effects, err := NormalizePatchEffects(tc.patch)
			if err == nil {
				t.Fatalf("expected a refusal, got %+v", effects)
			}
			if effects != nil {
				t.Fatalf("a refusal must not return partial effects: %+v", effects)
			}
		})
	}

	// The path guard must not over-refuse: a colon is a legal byte in a
	// POSIX filename, and only the `C:/` drive shape is rooted.
	t.Run("colon-in-a-filename-is-accepted", func(t *testing.T) {
		effects := s1Effects(t, "diff --git a/a:b.txt b/a:b.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n")
		if len(effects) != 1 || effects[0].Path != "a:b.txt" {
			t.Fatalf("effects = %+v, want one entry for a:b.txt", effects)
		}
	})
}

// TestS1DuplicateDestinationsAreRefused covers the duplicate-destination
// rule and the ONE compatibility exception to it.
//
// An effect set is a per-destination statement: the recipe derivation,
// the novelty classifier and the immutable observation all key on the
// destination path. Two records naming the same destination therefore
// have no single answer for "what happened to this path", and collapsing
// them to the first-seen record silently discards a real effect — which
// is what the pre-S1 derivation did. The authority refuses instead.
//
// The exception is PI-12's frozen b-side projection: its five shipped
// consumers received a de-duplicated first-seen list before this grammar
// existed, and refusing there would change a frozen result (PRD §6.1.2).
// That divergence is asserted here so it can only ever be deliberate.
func TestS1DuplicateDestinationsAreRefused(t *testing.T) {
	duplicate := "diff --git a/z.txt b/z.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n" +
		"diff --git a/a.txt b/a.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n" +
		"diff --git a/z.txt b/z.txt\nindex 2..3 100644\n@@ -1 +1 @@\n-b\n+c\n"

	t.Run("the-authority-refuses", func(t *testing.T) {
		effects, err := NormalizePatchEffects(duplicate)
		if err == nil {
			t.Fatalf("a repeated destination must be refused, got %d effect(s)", len(effects))
		}
		if effects != nil {
			t.Fatalf("a refusal must return a nil effect set, got %+v", effects)
		}
		// The message names the offending path and both records, so an
		// operator can find them without re-deriving the parse.
		for _, want := range []string{`records 1 and 3`, `"z.txt"`} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("refusal %q does not name %q", err, want)
			}
		}
	})

	t.Run("the-rollback-scope-refuses", func(t *testing.T) {
		union, err := PathsAffectedByPatchStrict(duplicate)
		if err == nil {
			t.Fatalf("the both-side union must refuse a repeated destination, got %q", union)
		}
		if union != nil {
			t.Fatalf("a refusal must return a nil scope, got %q", union)
		}
	})

	t.Run("PI-12-keeps-its-frozen-first-seen-list", func(t *testing.T) {
		bSide, err := FilesInPatchStrict(duplicate)
		if err != nil {
			t.Fatalf("the frozen b-side projection must still accept this input: %v", err)
		}
		want := []string{"z.txt", "a.txt"}
		if strings.Join(bSide, "|") != strings.Join(want, "|") {
			t.Fatalf("b-side = %q, want the frozen first-seen list %q", bSide, want)
		}
	})

	// Wrong-input sensitivity: the refusal must be about DUPLICATE
	// DESTINATIONS, not about multi-record patches in general, and not
	// about a path appearing on both sides of two different records.
	t.Run("wrong-input-sensitivity", func(t *testing.T) {
		accepted := map[string]string{
			"two-distinct-destinations": "diff --git a/z.txt b/z.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n" +
				"diff --git a/a.txt b/a.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n",
			// `a.txt` is a rename SOURCE in one record and a destination
			// in another. That is a real Git shape (move the file, then
			// create a new one at the old path) and it is not a
			// duplicate destination.
			"source-repeats-another-destination": "diff --git a/a.txt b/b.txt\nsimilarity index 100%\nrename from a.txt\nrename to b.txt\n" +
				"diff --git a/a.txt b/a.txt\nnew file mode 100644\n--- /dev/null\n+++ b/a.txt\n@@ -0,0 +1 @@\n+new\n",
			// Two records whose destinations differ only in case are two
			// paths on a case-sensitive filesystem, which is what Git
			// records.
			"case-differing-destinations": "diff --git a/Z.txt b/Z.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n" +
				"diff --git a/z.txt b/z.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n",
		}
		for name, patch := range accepted {
			t.Run(name, func(t *testing.T) {
				effects, err := NormalizePatchEffects(patch)
				if err != nil {
					t.Fatalf("this shape is NOT a duplicate destination and must be accepted: %v", err)
				}
				if len(effects) != 2 {
					t.Fatalf("got %d effect(s), want 2: %+v", len(effects), effects)
				}
			})
		}

	})
}

// TestS1TypeChangePairCoalesces freezes Git's legitimate two-record
// representation of a file type transition. It is one semantic modify effect,
// not the arbitrary duplicate-destination shape refused above.
func TestS1TypeChangePairCoalesces(t *testing.T) {
	deleted := "diff --git a/x.txt b/x.txt\ndeleted file mode 100644\n" +
		"index 1111111..0000000\n--- a/x.txt\n+++ /dev/null\n@@ -1 +0,0 @@\n-old\n"
	added := "diff --git a/x.txt b/x.txt\nnew file mode 120000\n" +
		"index 0000000..2222222\n--- /dev/null\n+++ b/x.txt\n@@ -0,0 +1 @@\n+target\n\\ No newline at end of file\n"
	patch := deleted + added

	effects, err := NormalizePatchEffects(patch)
	if err != nil {
		t.Fatalf("typechange must be accepted: %v", err)
	}
	if len(effects) != 1 {
		t.Fatalf("effects = %+v, want one coalesced typechange", effects)
	}
	effect := effects[0]
	if effect.ChangeKind != ChangeKindModify || effect.Path != "x.txt" {
		t.Fatalf("effect = %+v, want modify x.txt", effect)
	}
	if effect.HeaderOldMode != ModeRegular || effect.HeaderNewMode != ModeSymlink {
		t.Fatalf("header modes = %q -> %q, want 100644 -> 120000", effect.HeaderOldMode, effect.HeaderNewMode)
	}
	if effect.FragmentStart != 0 || effect.FragmentEnd != len(patch) {
		t.Fatalf("fragment = [%d,%d), want [0,%d)", effect.FragmentStart, effect.FragmentEnd, len(patch))
	}
	sum := sha256.Sum256([]byte(patch))
	if effect.FragmentSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal("coalesced fragment digest does not bind both Git records")
	}

	bSide, err := FilesInPatchStrict(patch)
	if err != nil || len(bSide) != 1 || bSide[0] != "x.txt" {
		t.Fatalf("PI-12 b-side = %q (err=%v), want [x.txt]", bSide, err)
	}
}

func TestS1DeleteAddModeChangeIsNotATypeChange(t *testing.T) {
	cases := map[string]string{
		"regular-mode-change": "diff --git a/x.sh b/x.sh\ndeleted file mode 100644\n--- a/x.sh\n+++ /dev/null\n@@ -1 +0,0 @@\n-x\n" +
			"diff --git a/x.sh b/x.sh\nnew file mode 100755\n--- /dev/null\n+++ b/x.sh\n@@ -0,0 +1 @@\n+x\n",
		"same-kind-symlink": "diff --git a/x b/x\ndeleted file mode 120000\n--- a/x\n+++ /dev/null\n@@ -1 +0,0 @@\n-old\n" +
			"diff --git a/x b/x\nnew file mode 120000\n--- /dev/null\n+++ b/x\n@@ -0,0 +1 @@\n+new\n",
		"non-adjacent-typechange": "diff --git a/x b/x\ndeleted file mode 100644\n--- a/x\n+++ /dev/null\n@@ -1 +0,0 @@\n-x\n" +
			"diff --git a/other b/other\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n" +
			"diff --git a/x b/x\nnew file mode 120000\n--- /dev/null\n+++ b/x\n@@ -0,0 +1 @@\n+target\n",
	}
	for name, patch := range cases {
		t.Run(name, func(t *testing.T) {
			effects, err := NormalizePatchEffects(patch)
			if err == nil {
				t.Fatalf("hand-authored delete+add shape must be refused, got %+v", effects)
			}
			if effects != nil {
				t.Fatalf("refusal returned partial effects: %+v", effects)
			}
		})
	}
}

func TestS1MixedQuotedHeaderRejectsExtraOperand(t *testing.T) {
	patch := "diff --git \"a/x\" \"b/x\" \"c/x\"\nsimilarity index 100%\nrename from x\nrename to y\n"
	if effects, err := NormalizePatchEffects(patch); err == nil {
		t.Fatalf("extra quoted operand must be refused, got %+v", effects)
	}
}

// TestS1BSideProjectionIsNotWidened is the PI-12 bite-proof (RGA-091):
// the b-side projection may never grow rename or copy sources, even
// though the shared grammar now knows about them.
func TestS1BSideProjectionIsNotWidened(t *testing.T) {
	for _, patch := range []string{
		"diff --git a/old.txt b/new.txt\nsimilarity index 100%\nrename from old.txt\nrename to new.txt\n",
		"diff --git a/src.txt b/dst.txt\nsimilarity index 100%\ncopy from src.txt\ncopy to dst.txt\n",
	} {
		bSide, err := FilesInPatchStrict(patch)
		if err != nil {
			t.Fatalf("FilesInPatchStrict: %v", err)
		}
		if len(bSide) != 1 {
			t.Fatalf("b-side = %q, want exactly one path; the PI-12 contract has been widened", bSide)
		}
		union, err := PathsAffectedByPatchStrict(patch)
		if err != nil {
			t.Fatalf("PathsAffectedByPatchStrict: %v", err)
		}
		if len(union) != 2 {
			t.Fatalf("union = %q, want both sides; the rollback scope has been narrowed", union)
		}
	}
}

// TestS1ObjectKindForMode pins the mode → object-kind classification. An
// unrecognized or empty mode is `unknown`, never a guess.
func TestS1ObjectKindForMode(t *testing.T) {
	cases := map[string]ObjectKind{
		ModeRegular:    ObjectKindRegular,
		ModeExecutable: ObjectKindExecutable,
		ModeSymlink:    ObjectKindSymlink,
		ModeGitlink:    ObjectKindGitlink,
		"":             ObjectKindUnknown,
		"040000":       ObjectKindUnknown,
	}
	for mode, want := range cases {
		if got := ObjectKindForMode(mode); got != want {
			t.Errorf("ObjectKindForMode(%q) = %q, want %q", mode, got, want)
		}
	}
}

// TestS1ExtantSidesMatchTheChangeAxis pins the extant-side rule the
// classification and completeness rules both read.
func TestS1ExtantSidesMatchTheChangeAxis(t *testing.T) {
	cases := []struct {
		kind     ChangeKind
		wantPre  bool
		wantPost bool
	}{
		{ChangeKindAdd, false, true},
		{ChangeKindDelete, true, false},
		{ChangeKindModify, true, true},
		{ChangeKindRename, true, true},
		{ChangeKindCopy, true, true},
	}
	for _, tc := range cases {
		pre, post := PatchEffect{ChangeKind: tc.kind}.ExtantSides()
		if pre != tc.wantPre || post != tc.wantPost {
			t.Errorf("%s extant sides = (%v,%v), want (%v,%v)", tc.kind, pre, post, tc.wantPre, tc.wantPost)
		}
	}
}
