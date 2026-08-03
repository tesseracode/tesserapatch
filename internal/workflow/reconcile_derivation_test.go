package workflow

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// TestLatestTouchedPaths_FallsBackToPostApplyPatch is the PRD-#3 N2
// regression: when a feature has a canonical `post-apply.patch` but
// NO `patch-generations.json` at all (the pre-ADR-024 shape — the
// manifest was only introduced at ADR-024, and refresh.go's fail-soft
// append-failure path can also leave a feature in this state on a
// post-ADR-024 repo), latestTouchedPaths must recover the touched
// paths by parsing the diff --git header lines instead of silently
// returning an empty set.
func TestLatestTouchedPaths_FallsBackToPostApplyPatch(t *testing.T) {
	tmp := t.TempDir()
	setupGitRepo(t, tmp)
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatal(err)
	}
	// installIndependentFeature writes post-apply.patch but never
	// calls seedPatchGenerations, so no patch-generations.json exists
	// on disk for "aaa" — exactly the pre-ADR-024 shape.
	installIndependentFeature(t, s, "aaa")

	if _, err := store.LoadPatchGenerations(s, "aaa"); err != nil {
		t.Fatalf("sanity: LoadPatchGenerations should fail-open to an empty manifest, got err: %v", err)
	}
	if _, statErr := s.ReadFeatureFile("aaa", filepath.Join("artifacts", "patch-generations.json")); statErr == nil {
		t.Fatal("sanity: patch-generations.json should NOT exist for this fixture")
	}

	got, err := latestTouchedPaths(s, "aaa")
	if err != nil {
		t.Fatalf("latestTouchedPaths fallback: unexpected error: %v", err)
	}
	want := []string{"aaa.txt"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("latestTouchedPaths fallback: got %v, want %v", got, want)
	}
}

// TestLatestTouchedPaths_ManifestWinsOverPostApplyPatch confirms the
// N2 fallback never overrides an existing patch-generations.json:
// when the manifest is present with a recorded generation, its
// touched_paths is authoritative even if it disagrees with what a
// naive post-apply.patch parse would produce.
func TestLatestTouchedPaths_ManifestWinsOverPostApplyPatch(t *testing.T) {
	tmp := t.TempDir()
	setupGitRepo(t, tmp)
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatal(err)
	}
	installIndependentFeature(t, s, "bbb")
	// Seed a manifest whose touched_paths deliberately differs from
	// what the post-apply.patch diff --git header would produce
	// (bbb.txt), to prove the manifest — not the fallback — wins.
	seedPatchGenerations(t, s, "bbb", canonicalIndependentPatch("bbb.txt", "bbb"), []string{"manifest-recorded-path.txt"})

	got, err := latestTouchedPaths(s, "bbb")
	if err != nil {
		t.Fatalf("latestTouchedPaths: unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "manifest-recorded-path.txt" {
		t.Errorf("latestTouchedPaths: manifest should win, got %v", got)
	}
}

// TestLatestTouchedPaths_FallbackRecoversD10Hint is the end-to-end
// PRD-#3 N2 scenario: two features where the earlier one has NO
// patch-generations.json (pre-ADR-024) but its post-apply.patch
// touches a subset of the later feature's touched_paths. Prior to
// the N2 fix, the missing manifest made latestTouchedPaths return an
// empty set for the earlier slug and the D10 hint never fired. After
// the fix, the fallback recovers the overlap and the hint fires.
func TestLatestTouchedPaths_FallbackRecoversD10Hint(t *testing.T) {
	tmp := t.TempDir()
	setupGitRepo(t, tmp)
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "A", Request: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkFeatureState("a", store.StateApplied, "apply", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "B", Request: "B"}); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkFeatureState("b", store.StateApplied, "apply", ""); err != nil {
		t.Fatal(err)
	}

	patchA := canonicalIndependentPatch("foo.txt", "hello-a")
	patchB := "diff --git a/foo.txt b/foo.txt\n" +
		"new file mode 100644\n" +
		"index 0000000..0000000\n" +
		"--- /dev/null\n" +
		"+++ b/foo.txt\n" +
		"@@ -0,0 +1 @@\n" +
		"+hello-b\n" +
		"diff --git a/bar.txt b/bar.txt\n" +
		"new file mode 100644\n" +
		"index 0000000..0000000\n" +
		"--- /dev/null\n" +
		"+++ b/bar.txt\n" +
		"@@ -0,0 +1 @@\n" +
		"+bar-only\n"
	if err := s.WriteArtifact("a", "post-apply.patch", patchA); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact("b", "post-apply.patch", patchB); err != nil {
		t.Fatal(err)
	}
	// Deliberately do NOT seed patch-generations.json for "a" — the
	// pre-ADR-024 fixture shape. "b" also has no manifest; the D10
	// check must recover touched_paths for BOTH sides via the N2
	// fallback for the hint to fire.

	orig := migrationDiagHintWriter
	buf := &bytes.Buffer{}
	migrationDiagHintWriter = buf
	t.Cleanup(func() { migrationDiagHintWriter = orig })

	if _, err := RunReconcile(context.Background(), s, []string{"a", "b"}, "HEAD", nil, provider.Config{}, ReconcileOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "hint: prior features may have been recorded cumulatively; retry with --cumulative-legacy (see ADR-030)") {
		t.Errorf("PRD-#3 N2: expected D10 hint recovered via post-apply.patch fallback (no patch-generations.json present), got:\n%s", buf.String())
	}
}
