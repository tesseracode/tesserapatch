package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeUpstreamLock overwrites .tpatch/upstream.lock with the four
// standard scalar fields. The lock file already exists after
// `tpatch init`, but the scaffolded version has empty values.
func writeUpstreamLock(t *testing.T, dir, remote, branch, commit string) {
	t.Helper()
	path := filepath.Join(dir, ".tpatch", "upstream.lock")
	content := "remote: " + remote + "\nbranch: " + branch + "\ncommit: " + commit + "\nurl: \"\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write upstream.lock: %v", err)
	}
}

// TestRecordAuto_HappyPath_LockAncestor — PRD §6 row 1.
// upstream.lock.commit is an ancestor of HEAD and is chosen directly.
// Ahead count of the linear range should be reported. The recorded
// patch must equal what `--from <base>` would have captured.
func TestRecordAuto_HappyPath_LockAncestor(t *testing.T) {
	tmpDir, baseSha, _, shaB := setupRecordRangeFixture(t)

	runCmd("init", "--path", tmpDir)
	// Pin upstream.lock to the very first commit (ancestor of HEAD,
	// distance = 2 commits ahead).
	writeUpstreamLock(t, tmpDir, "origin", "main", baseSha)

	runCmd("add", "--path", tmpDir, "Auto happy path")
	slug := "auto-happy-path"

	stdout, stderr, code := runCmdWithError("record", "--path", tmpDir, slug, "--auto", "--lenient")
	if code != 0 {
		t.Fatalf("record --auto failed: stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "record --auto selected base "+baseSha[:8]) {
		t.Errorf("decision line missing or wrong base: %q", stdout)
	}
	if !strings.Contains(stdout, "upstream.lock") {
		t.Errorf("decision line missing source label: %q", stdout)
	}
	if !strings.Contains(stdout, "2 commits ahead") {
		t.Errorf("ahead count missing or wrong: %q", stdout)
	}
	// status.apply.base_commit must be the resolved lower bound,
	// not HEAD. We compare to baseSha exactly.
	statusPath := filepath.Join(tmpDir, ".tpatch", "features", slug, "status.json")
	statusBytes, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("read status.json: %v", err)
	}
	if !strings.Contains(string(statusBytes), baseSha) {
		t.Errorf("status.json should record base=%s, got %s", baseSha, statusBytes)
	}
	// The patch should match `--from baseSha`'s output for this fixture.
	patchPath := filepath.Join(tmpDir, ".tpatch", "features", slug, "artifacts", "post-apply.patch")
	got, err := os.ReadFile(patchPath)
	if err != nil {
		t.Fatalf("read post-apply.patch: %v", err)
	}
	patch := string(got)
	if !strings.Contains(patch, "src/a.txt") || !strings.Contains(patch, "noise.txt") {
		t.Errorf("auto-recorded patch should include base..HEAD changes:\n%s", patch)
	}
	_ = shaB
}

// TestRecordAuto_AmbiguousMergeBase_Refuses — PRD §6 row 3.
// When the lock commit is on a divergent branch and merge-base falls
// back to a multi-commit range, --auto must refuse with a diagnostic
// that names the actual base and provides a `git log --oneline` recipe.
func TestRecordAuto_AmbiguousMergeBase_Refuses(t *testing.T) {
	tmpDir := t.TempDir()
	gitInitTestRepo(t, tmpDir)
	baseSha := gitHead(t, tmpDir)

	// Create a divergent "upstream" branch with one extra commit.
	mustRunGit(t, tmpDir, "checkout", "-b", "upstream-main")
	_ = os.WriteFile(filepath.Join(tmpDir, "u.txt"), []byte("upstream\n"), 0o644)
	mustRunGit(t, tmpDir, "add", ".")
	mustRunGit(t, tmpDir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "upstream-side")
	upstreamSha := gitHead(t, tmpDir)

	// Switch back to main and add TWO local commits — this is the
	// multi-commit divergence that should fail the safety gate.
	mustRunGit(t, tmpDir, "checkout", "-q", "main")
	_ = os.WriteFile(filepath.Join(tmpDir, "f1.txt"), []byte("feat1\n"), 0o644)
	mustRunGit(t, tmpDir, "add", ".")
	mustRunGit(t, tmpDir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "f1")
	_ = os.WriteFile(filepath.Join(tmpDir, "f2.txt"), []byte("feat2\n"), 0o644)
	mustRunGit(t, tmpDir, "add", ".")
	mustRunGit(t, tmpDir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "f2")

	runCmd("init", "--path", tmpDir)
	// Point the lock at the divergent upstream tip — not an ancestor
	// of HEAD, so the algorithm falls back to merge-base. The
	// merge-base is `baseSha`, and `baseSha..HEAD` contains 2
	// commits → refuse.
	writeUpstreamLock(t, tmpDir, "upstream", "upstream-main", upstreamSha)
	runCmd("add", "--path", tmpDir, "Auto ambiguous merge-base")
	slug := "auto-ambiguous-merge-base"

	_, stderr, code := runCmdWithError("record", "--path", tmpDir, slug, "--auto", "--lenient")
	if code == 0 {
		t.Fatalf("record --auto should have refused multi-commit merge-base fallback")
	}
	if !strings.Contains(stderr, "merge-base") {
		t.Errorf("refusal should mention merge-base: %q", stderr)
	}
	if !strings.Contains(stderr, "git log --oneline") {
		t.Errorf("refusal should suggest `git log --oneline ...`: %q", stderr)
	}
	if !strings.Contains(stderr, "2 commits") {
		t.Errorf("refusal should name the over-broad count: %q", stderr)
	}
	_ = baseSha
}

// TestRecordAuto_DirtyTree_Refuses — PRD §3.2 dirty-tree refusal.
func TestRecordAuto_DirtyTree_Refuses(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	writeUpstreamLock(t, tmpDir, "origin", "main", baseSha)
	// Dirty the working tree by modifying a tracked file.
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runCmd("add", "--path", tmpDir, "Auto dirty tree")
	_, stderr, code := runCmdWithError("record", "--path", tmpDir, "auto-dirty-tree", "--auto", "--lenient")
	if code == 0 {
		t.Fatalf("record --auto should refuse a dirty tree")
	}
	if !strings.Contains(stderr, "dirty working tree") {
		t.Errorf("refusal should mention dirty tree: %q", stderr)
	}
}

// TestRecordAuto_MutexFlags — PRD §3.1 mutual exclusion.
func TestRecordAuto_MutexFlags(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "Mutex check")
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"auto+from", []string{"record", "--path", tmpDir, "mutex-check", "--auto", "--from", baseSha}, "mutually exclusive with --from"},
		{"auto+range", []string{"record", "--path", tmpDir, "mutex-check", "--auto", "--commit-range", baseSha + "..HEAD"}, "mutually exclusive with --commit-range"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, code := runCmdWithError(tc.args...)
			if code == 0 {
				t.Fatalf("expected refusal")
			}
			if !strings.Contains(stderr, tc.want) {
				t.Errorf("missing mutex message; got %q", stderr)
			}
		})
	}
}

// TestRecordAuto_AutoEqualsFromExplicit — PRD §6 row 6.
// --auto --files <p> must produce the same patch bytes as
// --from <resolved-base> --files <p>.
func TestRecordAuto_AutoEqualsFromExplicit(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	writeUpstreamLock(t, tmpDir, "origin", "main", baseSha)

	// Record once with --auto.
	runCmd("add", "--path", tmpDir, "Auto path")
	if _, stderr, code := runCmdWithError("record", "--path", tmpDir, "auto-path",
		"--auto", "--files", "src/b.txt", "--lenient"); code != 0 {
		t.Fatalf("record --auto failed: %s", stderr)
	}
	autoPatch, err := os.ReadFile(filepath.Join(tmpDir, ".tpatch", "features", "auto-path", "artifacts", "post-apply.patch"))
	if err != nil {
		t.Fatal(err)
	}

	// Record again with explicit --from.
	runCmd("add", "--path", tmpDir, "Explicit path")
	if _, stderr, code := runCmdWithError("record", "--path", tmpDir, "explicit-path",
		"--from", baseSha, "--files", "src/b.txt", "--lenient"); code != 0 {
		t.Fatalf("record --from failed: %s", stderr)
	}
	fromPatch, err := os.ReadFile(filepath.Join(tmpDir, ".tpatch", "features", "explicit-path", "artifacts", "post-apply.patch"))
	if err != nil {
		t.Fatal(err)
	}
	if string(autoPatch) != string(fromPatch) {
		t.Errorf("--auto and --from <base> should produce byte-identical patches\n--auto:\n%s\n--from:\n%s",
			autoPatch, fromPatch)
	}
}

func mustRunGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}
