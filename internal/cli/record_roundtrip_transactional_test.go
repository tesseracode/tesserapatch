package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// snapshotFeatureDir walks .tpatch/features/<slug>/ and returns a
// path→sha256 map. Missing directory returns an empty map so tests can
// compare "before-command" (feature dir may not exist) with "after-
// command" (feature dir untouched by refused command) cleanly.
func snapshotFeatureDir(t *testing.T, tmpDir, slug string) map[string]string {
	t.Helper()
	root := filepath.Join(tmpDir, ".tpatch", "features", slug)
	out := make(map[string]string)
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return out
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		sum := sha256.Sum256(b)
		rel, _ := filepath.Rel(root, path)
		out[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("snapshotFeatureDir: %v", err)
	}
	return out
}

func diffSnapshots(before, after map[string]string) []string {
	var diffs []string
	seen := map[string]bool{}
	for k, v := range before {
		seen[k] = true
		if av, ok := after[k]; !ok {
			diffs = append(diffs, "deleted: "+k)
		} else if av != v {
			diffs = append(diffs, "modified: "+k)
		}
	}
	for k := range after {
		if !seen[k] {
			diffs = append(diffs, "added: "+k)
		}
	}
	sort.Strings(diffs)
	return diffs
}

func gitCmdT(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}

// TestRecordRoundTripFailure_Transactional exercises GH #5: a round-
// trip failure (working-tree edit that the committed --from/--to range
// cannot capture) must be transactional against the feature directory.
// Reproduces the reporter's scenario faithfully:
//
//   - Commit an initial Dockerfile
//   - Record & commit the first feature edit against Dockerfile
//   - Add an additional uncommitted Dockerfile edit in the working
//     tree
//   - `tpatch record --from HEAD^ --to HEAD --files Dockerfile` — the
//     committed diff cannot describe the working-tree edit, so the
//     reverse-apply check will fail.
//
// Asserts (per GH #5 "expected" section):
//  1. exit code non-zero
//  2. feature directory byte-identical to pre-command state
//  3. no "Recorded patch for" on stdout
//  4. guidance about --from/--to committed history appears on stderr
func TestRecordRoundTripFailure_Transactional(t *testing.T) {
	tmpDir := t.TempDir()
	gitInitTestRepo(t, tmpDir)
	runCmd("init", "--path", tmpDir)
	slug := "docker-builder-ignore-scripts"
	runCmd("add", "--path", tmpDir, slug)

	dockerfile := filepath.Join(tmpDir, "Dockerfile")
	os.WriteFile(dockerfile, []byte("FROM node:20\nRUN echo hi\n"), 0o644)
	gitCmdT(t, tmpDir, "add", "Dockerfile")
	gitCmdT(t, tmpDir, "commit", "-m", "add Dockerfile")

	// First feature edit — committed.
	os.WriteFile(dockerfile, []byte("FROM node:20\nRUN npm install --ignore-scripts\n"), 0o644)
	gitCmdT(t, tmpDir, "add", "Dockerfile")
	gitCmdT(t, tmpDir, "commit", "-m", "first source change")

	// Record the feature normally so a .tpatch/features/<slug>/ directory
	// exists in a valid state we can snapshot for equality after the
	// refused re-record.
	_, stderr0, code0 := runCmd("record", "--path", tmpDir, slug,
		"--from", "HEAD^", "--to", "HEAD", "--files", "Dockerfile")
	if code0 != 0 {
		t.Fatalf("initial record failed: %s", stderr0)
	}

	// Second (uncommitted) working-tree edit — the piece that the
	// committed --from/--to range CANNOT capture. This is the reporter's
	// exact drift shape.
	os.WriteFile(dockerfile, []byte("FROM node:20\nRUN npm install --ignore-scripts --production\n"), 0o644)

	before := snapshotFeatureDir(t, tmpDir, slug)
	if len(before) == 0 {
		t.Fatalf("expected feature dir to exist after initial record")
	}

	stdout, stderr, code := runCmd("record", "--path", tmpDir, slug,
		"--from", "HEAD^", "--to", "HEAD", "--files", "Dockerfile", "--regenerate-recipe")

	if code == 0 {
		t.Fatalf("expected non-zero exit on round-trip failure; got 0\n--- stdout ---\n%s\n--- stderr ---\n%s",
			stdout, stderr)
	}
	if strings.Contains(stdout, "Recorded patch for") {
		t.Errorf("expected NO 'Recorded patch for' on stdout after refusal; got stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "does not round-trip against working tree") {
		t.Errorf("expected round-trip diagnostic in stderr; got stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "--from/--to captures committed history only") {
		t.Errorf("expected --from/--to guidance hint in stderr; got stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "--lenient") {
		t.Errorf("expected --lenient bypass hint in stderr; got stderr=%q", stderr)
	}

	after := snapshotFeatureDir(t, tmpDir, slug)
	if diffs := diffSnapshots(before, after); len(diffs) > 0 {
		t.Errorf("feature directory mutated after refused record (GH #5):\n%s", strings.Join(diffs, "\n"))
	}
}

// TestRecordRoundTripFailure_LenientPreserved asserts that --lenient
// retains its historical permissive behavior: warning printed, exit 0,
// artifacts written. This guards the fix for GH #5 from over-tightening
// the escape hatch operators rely on for whitespace-sensitive files.
func TestRecordRoundTripFailure_LenientPreserved(t *testing.T) {
	tmpDir := t.TempDir()
	gitInitTestRepo(t, tmpDir)
	runCmd("init", "--path", tmpDir)
	slug := "lenient-preserved"
	runCmd("add", "--path", tmpDir, slug)

	dockerfile := filepath.Join(tmpDir, "Dockerfile")
	os.WriteFile(dockerfile, []byte("FROM node:20\nRUN echo hi\n"), 0o644)
	gitCmdT(t, tmpDir, "add", "Dockerfile")
	gitCmdT(t, tmpDir, "commit", "-m", "add Dockerfile")

	os.WriteFile(dockerfile, []byte("FROM node:20\nRUN npm install --ignore-scripts\n"), 0o644)
	gitCmdT(t, tmpDir, "add", "Dockerfile")
	gitCmdT(t, tmpDir, "commit", "-m", "first source change")

	// Uncommitted follow-up (the round-trip breaker).
	os.WriteFile(dockerfile, []byte("FROM node:20\nRUN npm install --ignore-scripts --production\n"), 0o644)

	stdout, stderr, code := runCmd("record", "--path", tmpDir, slug,
		"--from", "HEAD^", "--to", "HEAD", "--files", "Dockerfile", "--lenient")

	if code != 0 {
		t.Fatalf("record --lenient must exit 0 on round-trip failure; got code=%d\n--- stdout ---\n%s\n--- stderr ---\n%s",
			code, stdout, stderr)
	}
	if !strings.Contains(stderr, "--lenient: skipping patch round-trip validation") {
		t.Errorf("expected --lenient warning in stderr; got stderr=%q", stderr)
	}
	if !strings.Contains(stdout, "Recorded patch for "+slug) {
		t.Errorf("expected success line on stdout with --lenient; got stdout=%q", stdout)
	}

	// Canonical artifacts should exist.
	post := filepath.Join(tmpDir, ".tpatch", "features", slug, "artifacts", "post-apply.patch")
	if _, err := os.Stat(post); err != nil {
		t.Errorf("--lenient path must still write artifacts/post-apply.patch; got %v", err)
	}
}

// TestRecordRoundTripFailure_NoLenientHintOutsideRangeMode was removed
// after review: its intent (guarding against removal of the range-mode
// hint branch) is fully covered by
// TestRecordRoundTripFailure_Transactional, which asserts the hint IS
// present on the range-mode path. A separate placeholder test added
// noise without behavior.
