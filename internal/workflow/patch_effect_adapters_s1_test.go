package workflow

// GH #15 / ADR-036 slice S1 — the adapters over the strict effect
// grammar (PRD-recipe-generation-authority §6.1, PI-3..PI-6).
//
// An adapter is acceptable only if it derives its output from the strict
// normalized effect set and propagates the strict error. It may not
// re-implement header splitting, dequoting or `a/`/`b/` stripping, and it
// may not swallow the error to preserve today's short list. Every row
// here measures one of those three obligations.
//
// RGA-071..RGA-075 and RGA-094/RGA-096 live here.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// s1QuotedPathPatch is the shape every fail-soft reader used to drop on
// the floor: a Git C-quoted path containing a space and a tab.
const s1QuotedPathPatch = "diff --git \"a/sp ace\\ttab.txt\" \"b/sp ace\\ttab.txt\"\n" +
	"index 1111111..2222222 100644\n" +
	"--- \"a/sp ace\\ttab.txt\"\n+++ \"b/sp ace\\ttab.txt\"\n" +
	"@@ -1 +1 @@\n-a\n+b\n"

const s1QuotedPath = "sp ace\ttab.txt"

// s1UnreadablePatch is a present, non-empty patch the strict grammar
// refuses. Every adapter must surface the error rather than a short list.
const s1UnreadablePatch = "diff --git a/truncated.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n"

// TestS1AdaptersDecodeQuotedPaths covers RGA-094: the strict decoded
// repo-relative path appears in every projection the migrated readers
// feed — touched paths, novelty and the recipe derivation.
func TestS1AdaptersDecodeQuotedPaths(t *testing.T) {
	views, err := patchEffectViews(s1QuotedPathPatch)
	if err != nil {
		t.Fatalf("patchEffectViews: %v", err)
	}
	if len(views) != 1 || views[0].Path != s1QuotedPath {
		t.Fatalf("effect views = %+v, want one entry for %q", views, s1QuotedPath)
	}
	if views[0].ChangeKind != gitutil.ChangeKindModify {
		t.Fatalf("change_kind = %q, want modify", views[0].ChangeKind)
	}

	touched, err := strictTouchedPaths(s1QuotedPathPatch)
	if err != nil {
		t.Fatalf("strictTouchedPaths: %v", err)
	}
	if len(touched) != 1 || touched[0] != s1QuotedPath {
		t.Fatalf("touched paths = %q, want [%q]", touched, s1QuotedPath)
	}

	novelty, err := parsePatchNoveltyPaths(s1QuotedPathPatch)
	if err != nil {
		t.Fatalf("parsePatchNoveltyPaths: %v", err)
	}
	if len(novelty) != 1 || novelty[0].Path != s1QuotedPath {
		t.Fatalf("novelty paths = %+v, want one entry for %q", novelty, s1QuotedPath)
	}

	hunks, err := parsePatchHunks(s1QuotedPathPatch)
	if err != nil {
		t.Fatalf("parsePatchHunks: %v", err)
	}
	if len(hunks) != 1 || hunks[0].Path != s1QuotedPath {
		t.Fatalf("hunk attribution = %+v, want one hunk on %q", hunks, s1QuotedPath)
	}
}

// TestS1NoveltyChangeKindsComeFromTheStrictAxis covers PI-5's second
// authority claim: the novelty ACTION is projected from the record
// header's change axis, not inferred from a line scan.
func TestS1NoveltyChangeKindsComeFromTheStrictAxis(t *testing.T) {
	cases := []struct {
		name  string
		patch string
		want  PathNovelty
	}{
		{
			name: "create",
			patch: "diff --git a/new.txt b/new.txt\nnew file mode 100644\n" +
				"--- /dev/null\n+++ b/new.txt\n@@ -0,0 +1 @@\n+x\n",
			want: PathNovelty{Path: "new.txt", FeatureAction: FileNoveltyActionCreate},
		},
		{
			name: "modify",
			patch: "diff --git a/mod.txt b/mod.txt\nindex 1..2 100644\n" +
				"--- a/mod.txt\n+++ b/mod.txt\n@@ -1 +1 @@\n-a\n+b\n",
			want: PathNovelty{Path: "mod.txt", FeatureAction: FileNoveltyActionModify},
		},
		{
			name: "delete",
			patch: "diff --git a/gone.txt b/gone.txt\ndeleted file mode 100644\n" +
				"--- a/gone.txt\n+++ /dev/null\n@@ -1 +0,0 @@\n-x\n",
			want: PathNovelty{Path: "gone.txt", FeatureAction: FileNoveltyActionDelete},
		},
		{
			name:  "rename",
			patch: "diff --git a/old.txt b/new.txt\nsimilarity index 100%\nrename from old.txt\nrename to new.txt\n",
			want:  PathNovelty{Path: "new.txt", FeatureAction: FileNoveltyActionRename},
		},
		{
			// A copy creates its destination and leaves its source alone,
			// so the novelty view classifies it as a creation.
			name:  "copy",
			patch: "diff --git a/src.txt b/dst.txt\nsimilarity index 100%\ncopy from src.txt\ncopy to dst.txt\n",
			want:  PathNovelty{Path: "dst.txt", FeatureAction: FileNoveltyActionCreate},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePatchNoveltyPaths(tc.patch)
			if err != nil {
				t.Fatalf("parsePatchNoveltyPaths: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("got %d entries, want 1: %+v", len(got), got)
			}
			if got[0].Path != tc.want.Path || got[0].FeatureAction != tc.want.FeatureAction {
				t.Fatalf("novelty = %+v, want %+v", got[0], tc.want)
			}
		})
	}
}

// TestS1AdaptersPropagateTheStrictError covers RGA-071..RGA-074 and
// RGA-096: no adapter may absorb the strict refusal into a short list.
func TestS1AdaptersPropagateTheStrictError(t *testing.T) {
	if views, err := patchEffectViews(s1UnreadablePatch); err == nil {
		t.Fatalf("patchEffectViews accepted an unreadable patch: %+v", views)
	} else if views != nil {
		t.Fatalf("a refusal must not return a partial view: %+v", views)
	}
	if paths, err := strictTouchedPaths(s1UnreadablePatch); err == nil {
		t.Fatalf("strictTouchedPaths accepted an unreadable patch: %q", paths)
	} else if paths != nil {
		t.Fatalf("a refusal must not return a partial path list: %q", paths)
	}
	if novelty, err := parsePatchNoveltyPaths(s1UnreadablePatch); err == nil {
		t.Fatalf("parsePatchNoveltyPaths accepted an unreadable patch: %+v", novelty)
	} else if novelty != nil {
		t.Fatalf("a refusal must not return a partial novelty list: %+v", novelty)
	}
	if hunks, err := parsePatchHunks(s1UnreadablePatch); err == nil {
		t.Fatalf("parsePatchHunks accepted an unreadable patch: %+v", hunks)
	} else if hunks != nil {
		t.Fatalf("a refusal must not return partial hunk attribution: %+v", hunks)
	}
	if recipe, skipped, err := RecipeFromPatch(t.TempDir(), "demo", s1UnreadablePatch); err == nil {
		t.Fatalf("RecipeFromPatch accepted an unreadable patch: %+v / %v", recipe.Operations, skipped)
	}
	if paths, err := featurePathsFromPatch(s1UnreadablePatch); err == nil {
		t.Fatalf("featurePathsFromPatch accepted an unreadable patch: %q", paths)
	}
}

// TestS1HunkRangeProjectionIsUnchanged covers PI-6's scope limit: only the
// PATH attribution moved to the adapter. The hunk-range projection reads
// the same numbers it always did, and attributes every hunk of a record to
// that record's canonical path.
func TestS1HunkRangeProjectionIsUnchanged(t *testing.T) {
	patch := "diff --git a/one.txt b/one.txt\nindex 1..2 100644\n--- a/one.txt\n+++ b/one.txt\n" +
		"@@ -1,3 +1,4 @@\n a\n-b\n+B\n+extra\n c\n" +
		"@@ -20,2 +21,2 @@\n x\n-y\n+Y\n" +
		"diff --git a/two.txt b/two.txt\nindex 1..2 100644\n--- a/two.txt\n+++ b/two.txt\n" +
		"@@ -5 +5 @@\n-p\n+P\n"

	hunks, err := parsePatchHunks(patch)
	if err != nil {
		t.Fatalf("parsePatchHunks: %v", err)
	}
	want := []patchHunkRange{
		{Path: "one.txt", OldStart: 1, OldLen: 3, NewStart: 1, NewLen: 4},
		{Path: "one.txt", OldStart: 20, OldLen: 2, NewStart: 21, NewLen: 2},
		{Path: "two.txt", OldStart: 5, OldLen: 1, NewStart: 5, NewLen: 1},
	}
	if len(hunks) != len(want) {
		t.Fatalf("got %d hunks, want %d: %+v", len(hunks), len(want), hunks)
	}
	for i, w := range want {
		got := hunks[i]
		if got.Path != w.Path || got.OldStart != w.OldStart || got.OldLen != w.OldLen ||
			got.NewStart != w.NewStart || got.NewLen != w.NewLen {
			t.Errorf("hunk %d = %+v, want %+v", i, got, w)
		}
		if !strings.HasPrefix(got.Header, "@@ ") {
			t.Errorf("hunk %d lost its header: %q", i, got.Header)
		}
	}
}

// TestS1HunkAttributionIgnoresEmbeddedHeaders proves the attribution is
// derived from the grammar's record boundaries: a hunk body line that
// looks like a `diff --git` header cannot re-attribute the hunks that
// follow it.
func TestS1HunkAttributionIgnoresEmbeddedHeaders(t *testing.T) {
	patch := "diff --git a/doc.md b/doc.md\nindex 1..2 100644\n--- a/doc.md\n+++ b/doc.md\n" +
		"@@ -1,2 +1,3 @@\n intro\n+diff --git a/fake.txt b/fake.txt\n outro\n" +
		"@@ -10 +11 @@\n-x\n+y\n"

	hunks, err := parsePatchHunks(patch)
	if err != nil {
		t.Fatalf("parsePatchHunks: %v", err)
	}
	if len(hunks) != 2 {
		t.Fatalf("got %d hunks, want 2: %+v", len(hunks), hunks)
	}
	for _, h := range hunks {
		if h.Path != "doc.md" {
			t.Fatalf("hunk attributed to %q; an embedded header re-attributed it", h.Path)
		}
	}
}

// TestS1TouchedPathsErrorReachesTheExistingHandler covers RGA-075 for
// PI-3: the strict error surfaces through AppendPatchGenerationForFeature's
// EXISTING error return, and no manifest row is written with a silently
// short `touched_paths`.
func TestS1TouchedPathsErrorReachesTheExistingHandler(t *testing.T) {
	tmp := t.TempDir()
	setupGitRepo(t, tmp)
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "demo", Request: "demo"}); err != nil {
		t.Fatal(err)
	}

	changed, err := AppendPatchGenerationForFeature(s, "demo", PatchGenerationInput{
		Kind:       store.PatchGenerationKindRecord,
		Patch:      s1UnreadablePatch,
		BaseCommit: strings.Repeat("0", 40),
		Upper:      store.GenerationUpper{Kind: "working-tree", Ref: "working-tree"},
		Capture:    store.GenerationCapture{Mode: "working-tree-all", Pathspecs: []string{}, ClaimIDs: []string{}},
	})
	if err == nil {
		t.Fatal("an unreadable patch must surface the strict error to the caller's existing handler")
	}
	if changed {
		t.Fatal("a refused patch must not report an appended generation")
	}
	if !strings.Contains(err.Error(), "touched_paths") {
		t.Fatalf("the error should name the field it refused to populate: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(tmp, ".tpatch", "features", "demo", "patch-generations.json")); statErr == nil {
		t.Fatal("no manifest row may be written when touched_paths could not be derived")
	}
}

// TestS1TouchedPathsRetainsQuotedPaths is the PI-3 path-totality
// counterpart: the quoted path the fail-soft splitter dropped now reaches
// the manifest's `touched_paths`.
func TestS1TouchedPathsRetainsQuotedPaths(t *testing.T) {
	got, err := strictTouchedPaths(s1QuotedPathPatch)
	if err != nil {
		t.Fatalf("strictTouchedPaths: %v", err)
	}
	if len(got) != 1 || got[0] != s1QuotedPath {
		t.Fatalf("touched_paths = %q, want [%q]", got, s1QuotedPath)
	}

	// The b-side contract is shared with PI-12, so a rename contributes
	// its destination only — never both sides.
	renamed, err := strictTouchedPaths(
		"diff --git a/old.txt b/new.txt\nsimilarity index 100%\nrename from old.txt\nrename to new.txt\n")
	if err != nil {
		t.Fatalf("strictTouchedPaths: %v", err)
	}
	if len(renamed) != 1 || renamed[0] != "new.txt" {
		t.Fatalf("rename touched_paths = %q, want [new.txt] only", renamed)
	}
}

// TestS1NoveltyActionMappingIsTotal proves the change-axis mapping cannot
// silently fall through to "modify" when the grammar grows an arm.
func TestS1NoveltyActionMappingIsTotal(t *testing.T) {
	for _, kind := range []gitutil.ChangeKind{
		gitutil.ChangeKindAdd,
		gitutil.ChangeKindModify,
		gitutil.ChangeKindDelete,
		gitutil.ChangeKindRename,
		gitutil.ChangeKindCopy,
	} {
		if _, ok := noveltyActionForChangeKind(kind); !ok {
			t.Errorf("change kind %q has no novelty action", kind)
		}
	}
	if action, ok := noveltyActionForChangeKind(gitutil.ChangeKind("invented")); ok {
		t.Errorf("an unmapped change kind resolved to %q; the mapping must refuse instead", action)
	}
}
