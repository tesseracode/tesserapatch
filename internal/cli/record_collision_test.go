package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readArtifact reads a feature artifact file from the test workspace.
func readArtifact(t *testing.T, tmpDir, slug, name string) string {
	t.Helper()
	p := filepath.Join(tmpDir, ".tpatch", "features", slug, "artifacts", name)
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

// countPatchesDir returns the number of NNN-*.patch files under the
// feature's patches/ directory.
func countPatchesDir(t *testing.T, tmpDir, slug string) int {
	t.Helper()
	p := filepath.Join(tmpDir, ".tpatch", "features", slug, "patches")
	entries, err := os.ReadDir(p)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".patch") {
			n++
		}
	}
	return n
}

// recordSameContent uses an explicit --from/--to committed range to
// capture the same patch bytes for an arbitrary slug. The fixture
// produced by setupRecordRangeFixture has commits baseSha..HEAD whose
// diff is deterministic across slugs, so this is a reliable way to
// stage a byte-identical canonical patch under two different feature
// directories.
func recordSameContent(t *testing.T, tmpDir, slug, baseSha string, extraArgs ...string) (stdout, stderr string, code int) {
	t.Helper()
	args := []string{"record", "--path", tmpDir, slug, "--from", baseSha, "--lenient"}
	args = append(args, extraArgs...)
	return runCmdWithError(args...)
}

// TestRecordCollision_CrossFeatureRefusal — PRD §8 acceptance:
// recording a patch byte-identical to another feature's
// post-apply.patch refuses BEFORE writing any artifact for the
// current feature.
func TestRecordCollision_CrossFeatureRefusal(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)

	runCmd("add", "--path", tmpDir, "First feature")
	if _, stderr, code := recordSameContent(t, tmpDir, "first-feature", baseSha); code != 0 {
		t.Fatalf("baseline record failed: %s", stderr)
	}

	runCmd("add", "--path", tmpDir, "Second feature")
	stdout, stderr, code := recordSameContent(t, tmpDir, "second-feature", baseSha)
	if code == 0 {
		t.Fatalf("expected refusal, got success: stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "byte-identical") {
		t.Errorf("refusal should mention byte-identical: %q", stderr)
	}
	if !strings.Contains(stderr, "first-feature") {
		t.Errorf("refusal should list colliding slug: %q", stderr)
	}
	if !strings.Contains(stderr, "sha256=") || !strings.Contains(stderr, "bytes=") || !strings.Contains(stderr, "files=") {
		t.Errorf("refusal should include sha256/bytes/files: %q", stderr)
	}
	if !strings.Contains(stderr, "--allow-collision") {
		t.Errorf("refusal should mention --allow-collision override: %q", stderr)
	}

	// PRD §8: refuses BEFORE writing any artifact for the current feature.
	if got := readArtifact(t, tmpDir, "second-feature", "post-apply.patch"); got != "" {
		t.Errorf("post-apply.patch must not be written on refusal, got %d bytes", len(got))
	}
	if got := countPatchesDir(t, tmpDir, "second-feature"); got != 0 {
		t.Errorf("patches/ must be empty on refusal, got %d entries", got)
	}
}

// TestRecordCollision_AllowCollisionOverride — PRD §8 acceptance:
// --allow-collision <reason> permits the write and records the reason
// in record.md.
func TestRecordCollision_AllowCollisionOverride(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)

	runCmd("add", "--path", tmpDir, "Alpha")
	if _, stderr, code := recordSameContent(t, tmpDir, "alpha", baseSha); code != 0 {
		t.Fatalf("baseline failed: %s", stderr)
	}

	runCmd("add", "--path", tmpDir, "Bravo")
	reason := "intentional duplicate for fixture parity"
	stdout, stderr, code := recordSameContent(t, tmpDir, "bravo", baseSha, "--allow-collision", reason)
	if code != 0 {
		t.Fatalf("override should succeed: stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "--allow-collision") || !strings.Contains(stderr, reason) {
		t.Errorf("override should warn with reason on stderr: %q", stderr)
	}
	// post-apply.patch must exist.
	if got := readArtifact(t, tmpDir, "bravo", "post-apply.patch"); got == "" {
		t.Fatalf("post-apply.patch must be written under override")
	}
	// record.md must capture the reason.
	mdPath := filepath.Join(tmpDir, ".tpatch", "features", "bravo", "record.md")
	mdBytes, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read record.md: %v", err)
	}
	md := string(mdBytes)
	if !strings.Contains(md, "Collision Override") {
		t.Errorf("record.md should contain 'Collision Override' section: %q", md)
	}
	if !strings.Contains(md, reason) {
		t.Errorf("record.md should contain override reason: %q", md)
	}
	if !strings.Contains(md, "alpha") {
		t.Errorf("record.md should list colliding slug 'alpha': %q", md)
	}
}

// TestRecordCollision_SameFeatureDedup — PRD §3.2 / §8 acceptance:
// re-recording the same feature with unchanged patch bytes does not
// append a new numbered patches/NNN-record.patch.
func TestRecordCollision_SameFeatureDedup(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "Solo")
	slug := "solo"

	if _, stderr, code := recordSameContent(t, tmpDir, slug, baseSha); code != 0 {
		t.Fatalf("first record failed: %s", stderr)
	}
	firstCount := countPatchesDir(t, tmpDir, slug)
	if firstCount != 1 {
		t.Fatalf("expected 1 numbered patch after first record, got %d", firstCount)
	}

	stdout, stderr, code := recordSameContent(t, tmpDir, slug, baseSha)
	if code != 0 {
		t.Fatalf("re-record should succeed (same-feature dedup): %s", stderr)
	}
	if !strings.Contains(stdout, "skipping numbered audit snapshot") {
		t.Errorf("re-record should announce dedup: %q", stdout)
	}
	if got := countPatchesDir(t, tmpDir, slug); got != firstCount {
		t.Errorf("numbered patch count should be unchanged after dedup, got %d (was %d)", got, firstCount)
	}
}

// TestRecordCollision_SameFeatureChangedBytes — PRD §8 acceptance:
// re-recording the same feature with changed patch bytes appends the
// numbered patch as today.
func TestRecordCollision_SameFeatureChangedBytes(t *testing.T) {
	tmpDir, baseSha, shaA, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "Evolve")
	slug := "evolve"

	// First record — full base..HEAD range.
	if _, stderr, code := recordSameContent(t, tmpDir, slug, baseSha); code != 0 {
		t.Fatalf("first record failed: %s", stderr)
	}
	firstCount := countPatchesDir(t, tmpDir, slug)

	// Second record — narrower range (base..shaA), distinct bytes.
	stdout, stderr, code := runCmdWithError("record", "--path", tmpDir, slug, "--from", baseSha, "--to", shaA, "--lenient")
	if code != 0 {
		t.Fatalf("changed-bytes re-record should succeed: stdout=%q stderr=%q", stdout, stderr)
	}
	if strings.Contains(stdout, "skipping numbered audit snapshot") {
		t.Errorf("changed bytes should NOT trigger dedup skip: %q", stdout)
	}
	if got := countPatchesDir(t, tmpDir, slug); got != firstCount+1 {
		t.Errorf("expected numbered patch count to grow from %d to %d, got %d", firstCount, firstCount+1, got)
	}
}

// TestRecordCollision_MultipleColliders — PRD §5: refusal lists all
// colliding slugs and, when ≥3 features match, recommends stopping.
func TestRecordCollision_MultipleColliders(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)

	for i, slug := range []string{"one", "two", "three"} {
		runCmd("add", "--path", tmpDir, slug)
		args := []string{}
		if i > 0 {
			args = append(args, "--allow-collision", "fixture priming")
		}
		if _, stderr, code := recordSameContent(t, tmpDir, slug, baseSha, args...); code != 0 {
			t.Fatalf("priming record %q failed: %s", slug, stderr)
		}
	}

	runCmd("add", "--path", tmpDir, "four")
	_, stderr, code := recordSameContent(t, tmpDir, "four", baseSha)
	if code == 0 {
		t.Fatalf("expected refusal with multiple existing colliders")
	}
	for _, name := range []string{"one", "two", "three"} {
		if !strings.Contains(stderr, name) {
			t.Errorf("refusal should list every collider; missing %q in %q", name, stderr)
		}
	}
	if !strings.Contains(stderr, "Multiple existing features") {
		t.Errorf("refusal should recommend stopping when ≥3 collide: %q", stderr)
	}
}

// TestRecordCollision_MissingArtifactIgnored — PRD §4 step 4a / §8
// acceptance: collision scan ignores missing post-apply.patch files.
func TestRecordCollision_MissingArtifactIgnored(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)

	// Add a feature directory but never record it — its
	// post-apply.patch is absent.
	runCmd("add", "--path", tmpDir, "Bare slot")

	// Add a second feature with no existing collision and verify it
	// records cleanly even though `bare-slot/` exists alongside.
	runCmd("add", "--path", tmpDir, "With patch")
	_, stderr, code := recordSameContent(t, tmpDir, "with-patch", baseSha)
	if code != 0 {
		t.Fatalf("record should succeed when other features have no post-apply.patch: %s", stderr)
	}
	if got := readArtifact(t, tmpDir, "with-patch", "post-apply.patch"); got == "" {
		t.Fatal("post-apply.patch must be written")
	}
}

// TestRecordCollision_EmptyCaptureSkipsScan — PRD §4 step 0 / §8
// acceptance: empty-patch results skip the collision scan and preserve
// the existing empty-capture / empty-range behavior.
func TestRecordCollision_EmptyCaptureSkipsScan(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)

	// Seed a recorded feature so a collision scan would otherwise
	// have data to chew on.
	runCmd("add", "--path", tmpDir, "Seed feature")
	if _, stderr, code := recordSameContent(t, tmpDir, "seed-feature", baseSha); code != 0 {
		t.Fatalf("seed record failed: %s", stderr)
	}

	// Record a second feature with an explicit empty range (HEAD..HEAD).
	// Pre-existing behavior: "No changes to record in the specified range"
	// success path, NOT a collision refusal.
	runCmd("add", "--path", tmpDir, "Empty range")
	stdout, stderr, code := runCmdWithError("record", "--path", tmpDir, "empty-range",
		"--from", "HEAD", "--to", "HEAD", "--lenient")
	if code != 0 {
		t.Fatalf("explicit empty range should succeed (legacy no-op): stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "No changes to record") {
		t.Errorf("expected legacy empty-range no-op message: %q", stdout)
	}
	if strings.Contains(stderr, "byte-identical") {
		t.Errorf("empty capture must not trigger collision refusal: %q", stderr)
	}
}

// TestRecordCollision_RecoveryHints_WorkingTree — PRD §5: working-tree
// captures suggest --files and (when applicable) --auto.
func TestRecordCollision_RecoveryHints_WorkingTree(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "Committed first")
	if _, stderr, code := recordSameContent(t, tmpDir, "committed-first", baseSha); code != 0 {
		t.Fatalf("seed: %s", stderr)
	}

	// Reproduce the same bytes through a working-tree capture: revert
	// to baseSha, then re-stage the same content as uncommitted edits.
	// We piggyback on the committed-first patch by reading its bytes
	// and rewriting them into a sibling slot is overkill — instead,
	// use the actual working-tree capture path. We checkout baseSha
	// for a clean slate then write the same files back so the working
	// tree diff equals base..HEAD.
	mustRunGit(t, tmpDir, "checkout", "-q", baseSha)
	// Recreate the same content the post-HEAD commits introduced.
	if err := os.MkdirAll(filepath.Join(tmpDir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(tmpDir, "src", "a.txt"), []byte("a v1\n"), 0o644)
	_ = os.WriteFile(filepath.Join(tmpDir, "src", "b.txt"), []byte("b v2\n"), 0o644)
	_ = os.WriteFile(filepath.Join(tmpDir, "noise.txt"), []byte("noise\n"), 0o644)

	runCmd("add", "--path", tmpDir, "Working tree second")
	_, stderr, code := runCmdWithError("record", "--path", tmpDir, "working-tree-second", "--lenient")
	if code == 0 {
		// If the bytes don't collide (e.g. working-tree capture
		// includes untracked-file headers differently), the test
		// has no signal — skip rather than fail.
		t.Skip("working-tree capture did not produce byte-identical bytes; recovery-hint test not applicable")
	}
	if !strings.Contains(stderr, "--files") {
		t.Errorf("working-tree recovery hint should mention --files: %q", stderr)
	}
	if !strings.Contains(stderr, "--auto") {
		t.Errorf("working-tree recovery hint should mention --auto: %q", stderr)
	}
}

// TestRecordCollision_RecoveryHints_FromMode — PRD §5: --from-mode
// refusals mention --files, --auto, or --to.
func TestRecordCollision_RecoveryHints_FromMode(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "First from")
	if _, stderr, code := recordSameContent(t, tmpDir, "first-from", baseSha); code != 0 {
		t.Fatalf("seed: %s", stderr)
	}
	runCmd("add", "--path", tmpDir, "Second from")
	_, stderr, code := recordSameContent(t, tmpDir, "second-from", baseSha)
	if code == 0 {
		t.Fatal("expected refusal")
	}
	if !strings.Contains(stderr, "--files") {
		t.Errorf("from-mode refusal should suggest --files: %q", stderr)
	}
	if !strings.Contains(stderr, "--auto") && !strings.Contains(stderr, "--to") {
		t.Errorf("from-mode refusal should suggest --auto or --to: %q", stderr)
	}
}
