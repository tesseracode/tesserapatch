package workflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// canonicalIndependentPatch constructs a patch that only creates the
// given file with the given single-line content. The bytes are
// deterministic so tests can compare byte-for-byte.
func canonicalIndependentPatch(path, content string) string {
	return "diff --git a/" + path + " b/" + path + "\n" +
		"new file mode 100644\n" +
		"index 0000000..0000000\n" +
		"--- /dev/null\n" +
		"+++ b/" + path + "\n" +
		"@@ -0,0 +1 @@\n" +
		"+" + content + "\n"
}

// installIndependentFeature seeds an already-applied feature with
// slug=name, canonical post-apply.patch touching exactly one file
// (name + ".txt"), and status = applied. Used by the AC-1/AC-15
// suite to model the reporter's ABC stack.
func installIndependentFeature(t *testing.T, s *store.Store, name string) {
	t.Helper()
	if _, err := s.AddFeature(store.AddFeatureInput{Title: name, Request: name}); err != nil {
		t.Fatalf("AddFeature %s: %v", name, err)
	}
	if err := s.MarkFeatureState(name, store.StateApplied, "apply", ""); err != nil {
		t.Fatalf("MarkFeatureState %s: %v", name, err)
	}
	if err := s.WriteArtifact(name, "post-apply.patch", canonicalIndependentPatch(name+".txt", name)); err != nil {
		t.Fatalf("WriteArtifact %s: %v", name, err)
	}
}

// TestReconcile_DefaultMultiSlug_UsesCanonical_NoIncrementalWritten is
// the AC-1 reporter's ABC regression. Three features A, B, C with
// independent canonical patches must reconcile against their own
// canonical post-apply.patch in a single multi-slug invocation, and
// no incremental.patch must be written for any of them.
//
// PRD-multi-slug-reconcile-canonical-safety §6 AC-1 + INV-1/INV-2.
func TestReconcile_DefaultMultiSlug_UsesCanonical_NoIncrementalWritten(t *testing.T) {
	tmp := t.TempDir()
	setupGitRepo(t, tmp)
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatal(err)
	}
	installIndependentFeature(t, s, "aaa")
	installIndependentFeature(t, s, "bbb")
	installIndependentFeature(t, s, "ccc")

	// Snapshot each canonical patch pre-run so we can compare against
	// the reconcile-side reads.
	canonBefore := map[string]string{}
	for _, slug := range []string{"aaa", "bbb", "ccc"} {
		b, rerr := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
		if rerr != nil {
			t.Fatalf("read canonical %s: %v", slug, rerr)
		}
		canonBefore[slug] = b
	}

	// Default (non-legacy) multi-slug reconcile.
	if _, err := RunReconcile(context.Background(), s, []string{"aaa", "bbb", "ccc"}, "HEAD", nil, provider.Config{}, ReconcileOptions{}); err != nil {
		t.Fatalf("RunReconcile default multi-slug: %v", err)
	}

	// INV-2: no incremental.patch may have been written.
	for _, slug := range []string{"aaa", "bbb", "ccc"} {
		if _, err := os.Stat(filepath.Join(s.Root, ".tpatch", "features", slug, "artifacts", "incremental.patch")); !os.IsNotExist(err) {
			t.Errorf("INV-2 violation: %s/artifacts/incremental.patch exists after default reconcile (err=%v)", slug, err)
		}
	}
	// INV-1: canonical post-apply.patch is byte-identical pre- and
	// post-reconcile (reconcile must not mutate the input artifact).
	for _, slug := range []string{"aaa", "bbb", "ccc"} {
		got, rerr := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
		if rerr != nil {
			t.Fatalf("read canonical %s post-reconcile: %v", slug, rerr)
		}
		if got != canonBefore[slug] {
			t.Errorf("INV-1 violation: %s canonical post-apply.patch changed under default multi-slug reconcile", slug)
		}
	}
}

// TestReconcile_LegacyMultiSlug_WritesIncremental is the AC-4 opt-in
// path: --cumulative-legacy re-enables the pre-v0.12.1 derivation
// branch, so an incremental.patch is written for each slug. The test
// asserts (a) incremental.patch exists for A/B/C, (b) none of them
// carries a `.git/**` header (D4/D5 double-guarded).
func TestReconcile_LegacyMultiSlug_WritesIncremental(t *testing.T) {
	tmp := t.TempDir()
	setupGitRepo(t, tmp)
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatal(err)
	}
	installIndependentFeature(t, s, "aaa")
	installIndependentFeature(t, s, "bbb")
	installIndependentFeature(t, s, "ccc")

	if _, err := RunReconcile(context.Background(), s, []string{"aaa", "bbb", "ccc"}, "HEAD", nil, provider.Config{}, ReconcileOptions{CumulativeLegacy: true}); err != nil {
		t.Fatalf("RunReconcile legacy multi-slug: %v", err)
	}

	for _, slug := range []string{"aaa", "bbb", "ccc"} {
		p, rerr := s.ReadFeatureFile(slug, filepath.Join("artifacts", "incremental.patch"))
		if rerr != nil {
			t.Errorf("AC-4: incremental.patch not written for %s: %v", slug, rerr)
			continue
		}
		if strings.Contains(p, ".git/") || strings.Contains(p, "/.git") {
			t.Errorf("AC-4 / INV-3: legacy incremental.patch for %s contains .git/**:\n%s", slug, p)
		}
	}
}

// TestReconcile_LegacyMode_SkipsPhase15 is AC-12. Under
// --cumulative-legacy, phase 1.5 is skipped for the run and a
// "phase 1.5 skipped: --cumulative-legacy" note is attached to each
// ReconcileResult.
func TestReconcile_LegacyMode_SkipsPhase15(t *testing.T) {
	tmp := t.TempDir()
	setupGitRepo(t, tmp)
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatal(err)
	}
	// Turn phase 1.5 detector ON so we can prove it is skipped.
	cfg, _ := s.LoadConfig()
	cfg.PatchIDDetectorEnabled = true
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	installIndependentFeature(t, s, "aaa")
	installIndependentFeature(t, s, "bbb")

	results, err := RunReconcile(context.Background(), s, []string{"aaa", "bbb"}, "HEAD", nil, provider.Config{}, ReconcileOptions{CumulativeLegacy: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		found := false
		for _, n := range r.Notes {
			if strings.Contains(n, "phase 1.5 skipped: --cumulative-legacy") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("AC-12: expected 'phase 1.5 skipped: --cumulative-legacy' note on %s, notes=%v", r.Slug, r.Notes)
		}
	}
}

// TestMigrationHint_FiresOnOverlap is AC-15 positive: when the default
// (non-legacy) path fails phase 1 on slug N and an earlier slug in
// the run touched a subset of N's touched_paths, the D10 hint is
// emitted to the diagnostic sink.
func TestMigrationHint_FiresOnOverlap(t *testing.T) {
	tmp := t.TempDir()
	setupGitRepo(t, tmp)
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatal(err)
	}
	// Two features. A touches foo.txt only. B touches foo.txt AND
	// bar.txt (cumulative shape: A ⊆ B on touched_paths).
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
	// Seed patch-generations.json for both slugs with the touched_paths
	// the D10 check consults.
	seedPatchGenerations(t, s, "a", patchA, []string{"foo.txt"})
	seedPatchGenerations(t, s, "b", patchB, []string{"bar.txt", "foo.txt"})

	// Redirect the diagnostic sink into a buffer.
	orig := migrationDiagHintWriter
	buf := &bytes.Buffer{}
	migrationDiagHintWriter = buf
	t.Cleanup(func() { migrationDiagHintWriter = orig })

	if _, err := RunReconcile(context.Background(), s, []string{"a", "b"}, "HEAD", nil, provider.Config{}, ReconcileOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "hint: prior features may have been recorded cumulatively; retry with --cumulative-legacy (see ADR-030)") {
		t.Errorf("AC-15 positive: expected D10 migration hint, got:\n%s", buf.String())
	}
}

// TestMigrationHint_QuietOnDisjoint is AC-15 negative: independent
// canonical patches (touched_paths are disjoint) MUST NOT trigger
// the D10 hint. The reporter's ABC repro must be quiet.
func TestMigrationHint_QuietOnDisjoint(t *testing.T) {
	tmp := t.TempDir()
	setupGitRepo(t, tmp)
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatal(err)
	}
	installIndependentFeature(t, s, "aaa")
	installIndependentFeature(t, s, "bbb")
	installIndependentFeature(t, s, "ccc")
	// Seed disjoint touched_paths (independent shape).
	seedPatchGenerations(t, s, "aaa", canonicalIndependentPatch("aaa.txt", "aaa"), []string{"aaa.txt"})
	seedPatchGenerations(t, s, "bbb", canonicalIndependentPatch("bbb.txt", "bbb"), []string{"bbb.txt"})
	seedPatchGenerations(t, s, "ccc", canonicalIndependentPatch("ccc.txt", "ccc"), []string{"ccc.txt"})

	orig := migrationDiagHintWriter
	buf := &bytes.Buffer{}
	migrationDiagHintWriter = buf
	t.Cleanup(func() { migrationDiagHintWriter = orig })

	if _, err := RunReconcile(context.Background(), s, []string{"aaa", "bbb", "ccc"}, "HEAD", nil, provider.Config{}, ReconcileOptions{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "hint: prior features may have been recorded cumulatively") {
		t.Errorf("AC-15 negative: D10 hint fired on disjoint-touched-paths run:\n%s", buf.String())
	}
}

// seedPatchGenerations writes a minimal-but-valid patch-generations.json
// for the given slug with the given `touched_paths`. The manifest is
// what the D10 diagnostic reads to decide whether to emit its hint.
func seedPatchGenerations(t *testing.T, s *store.Store, slug, patch string, touchedPaths []string) {
	t.Helper()
	if _, err := AppendPatchGenerationForFeature(s, slug, PatchGenerationInput{
		Kind:       store.PatchGenerationKindRecord,
		Patch:      patch,
		AuditPatch: "artifacts/post-apply.patch",
		BaseCommit: "0000000000000000000000000000000000000000",
		Upper: store.GenerationUpper{
			Kind:   "working-tree",
			Ref:    "working-tree",
			Commit: "",
		},
		Reason: "test seed",
		Capture: store.GenerationCapture{
			Mode:      "working-tree-all",
			Pathspecs: []string{},
			ClaimIDs:  []string{},
		},
	}); err != nil {
		t.Fatalf("seedPatchGenerations %s: %v", slug, err)
	}
	// Overwrite touched_paths to the deterministic value the test
	// expects — the auto-computed set from FilesInPatch would work
	// too but this makes the test's intent explicit.
	m, err := store.LoadPatchGenerations(s, slug)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Generations) == 0 {
		t.Fatalf("seedPatchGenerations: no generation appended for %s", slug)
	}
	m.Generations[len(m.Generations)-1].TouchedPaths = touchedPaths
	if err := store.SavePatchGenerations(s, m); err != nil {
		t.Fatalf("SavePatchGenerations %s: %v", slug, err)
	}
}

// Ensure io is used even when the file only references it via other
// helpers indirectly (avoids the unused-import lint on some
// go-test-invocation shapes). The bytes buffer above uses io.Writer.
var _ io.Writer = (*bytes.Buffer)(nil)
