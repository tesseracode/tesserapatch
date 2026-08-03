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

// TestMigrationHint_FiresOnlyOnceAcrossMultiSlugRun is the PRD-#3 N3
// regression: a three-slug run where BOTH slug B (overlaps A) and
// slug C (overlaps A and/or B) would independently trip the D10
// overlap condition must still print the hint exactly once for the
// whole invocation, not once per triggering slug.
func TestMigrationHint_FiresOnlyOnceAcrossMultiSlugRun(t *testing.T) {
	tmp := t.TempDir()
	setupGitRepo(t, tmp)
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"A", "B", "C"} {
		if _, err := s.AddFeature(store.AddFeatureInput{Title: name, Request: name}); err != nil {
			t.Fatal(err)
		}
		if err := s.MarkFeatureState(strings.ToLower(name), store.StateApplied, "apply", ""); err != nil {
			t.Fatal(err)
		}
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
	// C is cumulative over B: touches foo.txt, bar.txt AND baz.txt, so
	// C also overlaps both A's and B's touched_paths independently.
	patchC := "diff --git a/foo.txt b/foo.txt\n" +
		"new file mode 100644\n" +
		"index 0000000..0000000\n" +
		"--- /dev/null\n" +
		"+++ b/foo.txt\n" +
		"@@ -0,0 +1 @@\n" +
		"+hello-c\n" +
		"diff --git a/bar.txt b/bar.txt\n" +
		"new file mode 100644\n" +
		"index 0000000..0000000\n" +
		"--- /dev/null\n" +
		"+++ b/bar.txt\n" +
		"@@ -0,0 +1 @@\n" +
		"+bar-c\n" +
		"diff --git a/baz.txt b/baz.txt\n" +
		"new file mode 100644\n" +
		"index 0000000..0000000\n" +
		"--- /dev/null\n" +
		"+++ b/baz.txt\n" +
		"@@ -0,0 +1 @@\n" +
		"+baz-only\n"

	if err := s.WriteArtifact("a", "post-apply.patch", patchA); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact("b", "post-apply.patch", patchB); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact("c", "post-apply.patch", patchC); err != nil {
		t.Fatal(err)
	}
	seedPatchGenerations(t, s, "a", patchA, []string{"foo.txt"})
	seedPatchGenerations(t, s, "b", patchB, []string{"bar.txt", "foo.txt"})
	seedPatchGenerations(t, s, "c", patchC, []string{"bar.txt", "baz.txt", "foo.txt"})

	orig := migrationDiagHintWriter
	buf := &bytes.Buffer{}
	migrationDiagHintWriter = buf
	t.Cleanup(func() { migrationDiagHintWriter = orig })

	if _, err := RunReconcile(context.Background(), s, []string{"a", "b", "c"}, "HEAD", nil, provider.Config{}, ReconcileOptions{}); err != nil {
		t.Fatal(err)
	}

	const hint = "hint: prior features may have been recorded cumulatively; retry with --cumulative-legacy (see ADR-030)"
	count := strings.Count(buf.String(), hint)
	if count != 1 {
		t.Errorf("PRD-#3 N3: expected D10 hint exactly once across the run, got %d occurrences:\n%s", count, buf.String())
	}
}

// TestLegacyDetectorSilencedNote_FiresWhenDetectorEnabled is the
// PRD-#3 S1 positive case: --cumulative-legacy silences phase 1.5,
// so when the detector is actually configured on, one informational
// stderr line must be emitted for the whole run.
func TestLegacyDetectorSilencedNote_FiresWhenDetectorEnabled(t *testing.T) {
	tmp := t.TempDir()
	setupGitRepo(t, tmp)
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := s.LoadConfig()
	cfg.PatchIDDetectorEnabled = true
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	installIndependentFeature(t, s, "aaa")
	installIndependentFeature(t, s, "bbb")

	orig := legacyDetectorSilencedNoteWriter
	buf := &bytes.Buffer{}
	legacyDetectorSilencedNoteWriter = buf
	t.Cleanup(func() { legacyDetectorSilencedNoteWriter = orig })

	if _, err := RunReconcile(context.Background(), s, []string{"aaa", "bbb"}, "HEAD", nil, provider.Config{}, ReconcileOptions{CumulativeLegacy: true}); err != nil {
		t.Fatal(err)
	}

	if strings.Count(buf.String(), legacyDetectorSilencedNote) != 1 {
		t.Errorf("PRD-#3 S1: expected the legacy-detector-silenced note exactly once, got:\n%s", buf.String())
	}
}

// TestLegacyDetectorSilencedNote_QuietWhenDetectorDisabled is the S1
// negative case: when the patch-id detector was never configured on,
// --cumulative-legacy silencing it is not news, so the note must NOT
// fire (it would be misleading noise).
func TestLegacyDetectorSilencedNote_QuietWhenDetectorDisabled(t *testing.T) {
	tmp := t.TempDir()
	setupGitRepo(t, tmp)
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatal(err)
	}
	installIndependentFeature(t, s, "aaa")

	orig := legacyDetectorSilencedNoteWriter
	buf := &bytes.Buffer{}
	legacyDetectorSilencedNoteWriter = buf
	t.Cleanup(func() { legacyDetectorSilencedNoteWriter = orig })

	if _, err := RunReconcile(context.Background(), s, []string{"aaa"}, "HEAD", nil, provider.Config{}, ReconcileOptions{CumulativeLegacy: true}); err != nil {
		t.Fatal(err)
	}

	if buf.Len() != 0 {
		t.Errorf("PRD-#3 S1 negative: expected no legacy-detector-silenced note when detector disabled, got:\n%s", buf.String())
	}
}

// TestLegacyDetectorSilencedNote_QuietWhenNotLegacy is a sanity check
// that the S1 note never fires on the default (non-legacy) path even
// if the detector is enabled — the note is specifically about the
// --cumulative-legacy suppression, not a general detector-status log.
func TestLegacyDetectorSilencedNote_QuietWhenNotLegacy(t *testing.T) {
	tmp := t.TempDir()
	setupGitRepo(t, tmp)
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := s.LoadConfig()
	cfg.PatchIDDetectorEnabled = true
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	installIndependentFeature(t, s, "aaa")

	orig := legacyDetectorSilencedNoteWriter
	buf := &bytes.Buffer{}
	legacyDetectorSilencedNoteWriter = buf
	t.Cleanup(func() { legacyDetectorSilencedNoteWriter = orig })

	if _, err := RunReconcile(context.Background(), s, []string{"aaa"}, "HEAD", nil, provider.Config{}, ReconcileOptions{}); err != nil {
		t.Fatal(err)
	}

	if buf.Len() != 0 {
		t.Errorf("PRD-#3 S1 sanity: default path must never emit the legacy-only note, got:\n%s", buf.String())
	}
}
