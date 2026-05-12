package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupReconcileLockRepo creates a git repo with two commits on a
// `refs/remotes/<remote>/<branch>` ref so classifyUpstreamLock can
// resolve it. Returns the repo root and (head, prev) commit shas.
func setupReconcileLockRepo(t *testing.T, remote, branch string) (root, head, prev string) {
	t.Helper()
	root = t.TempDir()
	runs := [][]string{
		{"git", "init", "-q"},
		{"git", "config", "user.email", "t@t.t"},
		{"git", "config", "user.name", "t"},
		{"git", "config", "commit.gpgsign", "false"},
	}
	for _, args := range runs {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = root
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s: %v", args, out, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("1"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	runGitTest(t, root, "add", ".")
	runGitTest(t, root, "commit", "-q", "-m", "a")
	prev = strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD"))

	if err := os.WriteFile(filepath.Join(root, "b"), []byte("2"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}
	runGitTest(t, root, "add", ".")
	runGitTest(t, root, "commit", "-q", "-m", "b")
	head = strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD"))

	runGitTest(t, root, "update-ref", "refs/remotes/"+remote+"/"+branch, head)
	if err := os.MkdirAll(filepath.Join(root, ".tpatch"), 0o755); err != nil {
		t.Fatalf("mkdir .tpatch: %v", err)
	}
	return
}

func runGitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
	return string(out)
}

func writeLock(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".tpatch", "upstream.lock"), []byte(body), 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}
}

func TestClassifyUpstreamLock_Valid(t *testing.T) {
	root, head, _ := setupReconcileLockRepo(t, "upstream", "main")
	writeLock(t, root, "remote: upstream\nbranch: main\ncommit: "+head+"\nurl: \"\"\n")
	state, _ := classifyUpstreamLock(root, "")
	if state != LockStateValid {
		t.Fatalf("want Valid, got %v", state)
	}
}

func TestClassifyUpstreamLock_ValidAncestor(t *testing.T) {
	// Lock commit is an ancestor of HEAD (not equal).
	root, _, prev := setupReconcileLockRepo(t, "upstream", "main")
	writeLock(t, root, "remote: upstream\nbranch: main\ncommit: "+prev+"\nurl: \"\"\n")
	state, _ := classifyUpstreamLock(root, "")
	if state != LockStateValid {
		t.Fatalf("want Valid (ancestor), got %v", state)
	}
}

func TestClassifyUpstreamLock_Missing(t *testing.T) {
	root, _, _ := setupReconcileLockRepo(t, "upstream", "main")
	// no lock written
	state, _ := classifyUpstreamLock(root, "")
	if state != LockStateMissing {
		t.Fatalf("want Missing, got %v", state)
	}
}

func TestClassifyUpstreamLock_Empty(t *testing.T) {
	root, _, _ := setupReconcileLockRepo(t, "upstream", "main")
	writeLock(t, root, "# scaffold\nremote: \"\"\nbranch: \"\"\ncommit: \"\"\nurl: \"\"\n")
	state, _ := classifyUpstreamLock(root, "")
	if state != LockStateEmpty {
		t.Fatalf("want Empty, got %v", state)
	}
}

func TestClassifyUpstreamLock_StaleCommit(t *testing.T) {
	root, _, _ := setupReconcileLockRepo(t, "upstream", "main")
	// A commit that is NOT an ancestor — make a sibling branch.
	sib := strings.TrimSpace(runGitTest(t, root, "commit-tree", "-m", "sib",
		strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD^{tree}"))))
	writeLock(t, root, "remote: upstream\nbranch: main\ncommit: "+sib+"\nurl: \"\"\n")
	state, d := classifyUpstreamLock(root, "")
	if state != LockStateStale || d.SubCause != StaleSubCauseCommit {
		t.Fatalf("want Stale/STALE-COMMIT, got %v/%v", state, d.SubCause)
	}
}

func TestClassifyUpstreamLock_StaleResolve(t *testing.T) {
	root, head, _ := setupReconcileLockRepo(t, "upstream", "main")
	// Recorded ref does not exist locally.
	writeLock(t, root, "remote: upstream\nbranch: nonexistent\ncommit: "+head+"\nurl: \"\"\n")
	state, d := classifyUpstreamLock(root, "")
	if state != LockStateStale || d.SubCause != StaleSubCauseResolve {
		t.Fatalf("want Stale/STALE-RESOLVE, got %v/%v", state, d.SubCause)
	}
}

func TestClassifyUpstreamLock_StaleRef(t *testing.T) {
	root, _, _ := setupReconcileLockRepo(t, "upstream", "main")
	writeLock(t, root, "remote: upstream\nbranch: main\ncommit: 0000000000000000000000000000000000000000\nurl: \"\"\n")
	state, d := classifyUpstreamLock(root, "")
	if state != LockStateStale || d.SubCause != StaleSubCauseRef {
		t.Fatalf("want Stale/STALE-REF, got %v/%v", state, d.SubCause)
	}
}

func TestClassifyUpstreamLock_SkippedOnOverride(t *testing.T) {
	root, head, _ := setupReconcileLockRepo(t, "upstream", "main")
	// Set up a second remote-tracking ref the override can point at.
	runGitTest(t, root, "update-ref", "refs/remotes/origin/develop", head)
	writeLock(t, root, "remote: upstream\nbranch: main\ncommit: "+head+"\nurl: \"\"\n")
	state, d := classifyUpstreamLock(root, "origin/develop")
	if state != LockStateSkipped {
		t.Fatalf("want Skipped, got %v", state)
	}
	if d.OverrideRef != "origin/develop" || d.LockRefName != "upstream/main" {
		t.Fatalf("unexpected diag: %+v", d)
	}
}

// Acceptance #18: when --upstream-ref happens to be a different
// symbolic name for the SAME underlying ref, do NOT skip — fall
// through to validation. (We can't easily simulate two names for the
// same ref in this minimal repo, so we verify the converse: when the
// override symbolic-full-name equals the lock's, it must NOT be
// Skipped.)
func TestClassifyUpstreamLock_OverrideEqualsLock_NotSkipped(t *testing.T) {
	root, head, _ := setupReconcileLockRepo(t, "upstream", "main")
	writeLock(t, root, "remote: upstream\nbranch: main\ncommit: "+head+"\nurl: \"\"\n")
	state, _ := classifyUpstreamLock(root, "upstream/main")
	if state != LockStateValid {
		t.Fatalf("override matching lock should validate, got %v", state)
	}
}

func TestClassifyUpstreamLock_LegacyBranchNormalization(t *testing.T) {
	// Pre-v0.8 writer left full ref inside branch:. Lock-guard must
	// strip the redundant `<remote>/` prefix when reassembling.
	root, head, _ := setupReconcileLockRepo(t, "upstream", "main")
	writeLock(t, root, "remote: upstream\nbranch: upstream/main\ncommit: "+head+"\nurl: \"\"\n")
	state, _ := classifyUpstreamLock(root, "")
	if state != LockStateValid {
		t.Fatalf("legacy lock should normalize and validate, got %v", state)
	}
}

// Acceptance #19: the legacy single-arg PreflightReconcile must
// continue to work and must NOT classify the lock.
func TestPreflightReconcile_DoesNotClassifyLock(t *testing.T) {
	root, head, _ := setupReconcileLockRepo(t, "upstream", "main")
	writeLock(t, root, "remote: upstream\nbranch: main\ncommit: "+head+"\nurl: \"\"\n")
	p, err := PreflightReconcile(root)
	if err != nil {
		t.Fatalf("PreflightReconcile: %v", err)
	}
	if p.LockState != LockStateUnknown {
		t.Fatalf("single-arg form must leave LockState Unknown, got %v", p.LockState)
	}
}

func TestPreflightReconcileWithOverride_ClassifiesLock(t *testing.T) {
	root, head, _ := setupReconcileLockRepo(t, "upstream", "main")
	writeLock(t, root, "remote: upstream\nbranch: main\ncommit: "+head+"\nurl: \"\"\n")
	p, err := PreflightReconcileWithOverride(root, "upstream/main")
	if err != nil {
		t.Fatalf("PreflightReconcileWithOverride: %v", err)
	}
	if p.LockState != LockStateValid {
		t.Fatalf("want Valid, got %v", p.LockState)
	}
}

func TestSplitUpstreamRef(t *testing.T) {
	cases := []struct {
		in          string
		remote, brn string
		ok          bool
	}{
		{"upstream/main", "upstream", "main", true},
		{"origin/feature-branch", "origin", "feature-branch", true},
		{"", "", "", false},
		{"noslash", "", "", false},
		{"/leading", "", "", false},
		{"trailing/", "", "", false},
		{"too/many/slashes", "", "", false},
	}
	for _, c := range cases {
		r, b, ok := SplitUpstreamRef(c.in)
		if r != c.remote || b != c.brn || ok != c.ok {
			t.Errorf("SplitUpstreamRef(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.in, r, b, ok, c.remote, c.brn, c.ok)
		}
	}
}
