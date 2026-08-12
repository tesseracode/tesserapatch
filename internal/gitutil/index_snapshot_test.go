// Tests for the effective-index snapshot/restore primitives and the
// staged-path audit (GH #7 rev-5).

package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func idxGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %s: %v", args, dir, out, err)
	}
	return string(out)
}

func TestEffectiveIndexPathMainWorktree(t *testing.T) {
	root := nestedWTRepo(t)
	got, err := EffectiveIndexPath(root)
	if err != nil {
		t.Fatalf("EffectiveIndexPath: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("path must be absolute, got %q", got)
	}
	want := filepath.Join(root, ".git", "index")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A linked worktree keeps its index under the common repository's
// worktrees/ directory; the snapshot must follow it there.
func TestEffectiveIndexPathLinkedWorktree(t *testing.T) {
	root := nestedWTRepo(t)
	linked := filepath.Join(t.TempDir(), "linked")
	addWorktree(t, root, linked, "linked-index")

	got, err := EffectiveIndexPath(linked)
	if err != nil {
		t.Fatalf("EffectiveIndexPath: %v", err)
	}
	if !strings.Contains(filepath.ToSlash(got), "/worktrees/") {
		t.Errorf("linked worktree index should live under worktrees/, got %q", got)
	}
	if strings.HasSuffix(filepath.ToSlash(got), "/.git/index") {
		t.Errorf("linked worktree resolved to the main index: %q", got)
	}

	// Snapshot/restore must round-trip the linked worktree's own index.
	if err := os.WriteFile(filepath.Join(linked, "hello.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxGit(t, linked, "add", "hello.txt")
	snap, err := SnapshotIndex(linked)
	if err != nil {
		t.Fatalf("SnapshotIndex: %v", err)
	}
	if !snap.Existed {
		t.Fatal("linked worktree index should exist after add")
	}
	treeBefore := idxGit(t, linked, "write-tree")

	if err := os.WriteFile(filepath.Join(linked, "extra.txt"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxGit(t, linked, "add", "extra.txt")
	if err := snap.Restore(); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got := idxGit(t, linked, "write-tree"); got != treeBefore {
		t.Errorf("linked worktree index not restored:\nbefore=%s\nafter=%s", treeBefore, got)
	}
}

// GIT_INDEX_FILE redirection must be honoured.
func TestEffectiveIndexPathRespectsGitIndexFile(t *testing.T) {
	root := nestedWTRepo(t)
	redirect := filepath.Join(t.TempDir(), "redirected-index")
	t.Setenv("GIT_INDEX_FILE", redirect)
	got, err := EffectiveIndexPath(root)
	if err != nil {
		t.Fatalf("EffectiveIndexPath: %v", err)
	}
	if got != redirect {
		t.Errorf("got %q, want the redirected path %q", got, redirect)
	}
	// The redirected index does not exist yet: that is a valid snapshot.
	snap, err := SnapshotIndex(root)
	if err != nil {
		t.Fatalf("SnapshotIndex: %v", err)
	}
	if snap.Existed {
		t.Error("redirected index should not exist yet")
	}
}

// A repository that has never staged anything has no index file.
// Restoring that state means removing whatever index was created.
func TestSnapshotIndexAbsentIndexRoundTrip(t *testing.T) {
	dir := t.TempDir()
	idxGit(t, dir, "init", "-q", "-b", "main", ".")
	idxGit(t, dir, "config", "user.email", "t@example.com")
	idxGit(t, dir, "config", "user.name", "T")

	snap, err := SnapshotIndex(dir)
	if err != nil {
		t.Fatalf("SnapshotIndex: %v", err)
	}
	if snap.Existed {
		t.Fatal("a fresh repo should have no index file")
	}
	if _, err := os.Stat(snap.Path); !os.IsNotExist(err) {
		t.Fatalf("fixture assumption broken: index exists at %q", snap.Path)
	}

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxGit(t, dir, "add", "a.txt")
	if _, err := os.Stat(snap.Path); err != nil {
		t.Fatalf("index should exist after add: %v", err)
	}
	if err := snap.Restore(); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if _, err := os.Stat(snap.Path); !os.IsNotExist(err) {
		t.Errorf("restore should have removed the index, got %v", err)
	}
	// Restoring twice must be a no-op, not an error.
	if err := snap.Restore(); err != nil {
		t.Errorf("second Restore should be a no-op: %v", err)
	}
}

// The operator's own staged state — including intent-to-add entries —
// must survive a rollback byte-for-byte.
func TestSnapshotIndexPreservesOperatorStagedState(t *testing.T) {
	root := nestedWTRepo(t)
	if err := os.WriteFile(filepath.Join(root, "operator.txt"), []byte("operator staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "intent.txt"), []byte("intent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxGit(t, root, "add", "operator.txt")
	idxGit(t, root, "add", "--intent-to-add", "intent.txt")

	snap, err := SnapshotIndex(root)
	if err != nil {
		t.Fatalf("SnapshotIndex: %v", err)
	}
	rawBefore, err := os.ReadFile(snap.Path)
	if err != nil {
		t.Fatal(err)
	}
	statusBefore := idxGit(t, root, "status", "--porcelain", "-z", "--untracked-files=all")
	treeBefore := idxGit(t, root, "write-tree")

	// Simulate land staging something else on top.
	if err := os.WriteFile(filepath.Join(root, "landed.txt"), []byte("landed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxGit(t, root, "add", "landed.txt")
	if got := idxGit(t, root, "write-tree"); got == treeBefore {
		t.Fatal("fixture assumption broken: staging did not change the index")
	}

	if err := snap.Restore(); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	rawAfter, err := os.ReadFile(snap.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(rawAfter) != string(rawBefore) {
		t.Error("index bytes are not identical after restore")
	}
	if got := idxGit(t, root, "write-tree"); got != treeBefore {
		t.Errorf("write-tree differs after restore:\nbefore=%s\nafter=%s", treeBefore, got)
	}
	// The snapshot restores the INDEX, not the working tree, so the
	// file land created is still on disk (now untracked). Remove it
	// before comparing full status.
	if err := os.Remove(filepath.Join(root, "landed.txt")); err != nil {
		t.Fatal(err)
	}
	if got := idxGit(t, root, "status", "--porcelain", "-z", "--untracked-files=all"); got != statusBefore {
		t.Errorf("status differs after restore:\nbefore=%q\nafter=%q", statusBefore, got)
	}
	if fi, err := os.Stat(snap.Path); err != nil {
		t.Fatal(err)
	} else if fi.Mode().Perm() != snap.Mode {
		t.Errorf("mode %v != %v", fi.Mode().Perm(), snap.Mode)
	}
}

// StagedPaths must be byte-exact for names Git would otherwise quote.
func TestStagedPathsByteExact(t *testing.T) {
	root := nestedWTRepo(t)
	names := []string{"sp ace.txt", "tab\tin.txt", "new\nline.txt"}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(root, n), []byte("x\n"), 0o644); err != nil {
			t.Fatalf("write %q: %v", n, err)
		}
		idxGit(t, root, "--literal-pathspecs", "add", "--", n)
	}
	got, err := StagedPaths(root)
	if err != nil {
		t.Fatalf("StagedPaths: %v", err)
	}
	set := map[string]bool{}
	for _, p := range got {
		set[p] = true
	}
	for _, n := range names {
		if !set[n] {
			t.Errorf("staged path %q missing from %q", n, got)
		}
	}
}

// The audit must flag a staged gitlink for a registered nested
// worktree, and must flag nothing when no worktree is nested.
func TestAuditStagedPathsForNestedWorktrees(t *testing.T) {
	root := nestedWTRepo(t)
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxGit(t, root, "add", "ok.txt")

	clean, err := AuditStagedPathsForNestedWorktrees(root)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(clean) != 0 {
		t.Errorf("no nested worktree registered, expected no findings, got %q", clean)
	}

	rel := ".claude/worktrees/agent review"
	addWorktree(t, root, filepath.Join(root, filepath.FromSlash(rel)), "agent-review")
	idxGit(t, root, "-c", "advice.addEmbeddedRepo=false",
		"--literal-pathspecs", "add", "--", rel)

	contaminated, err := AuditStagedPathsForNestedWorktrees(root)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(contaminated) != 1 || contaminated[0] != rel {
		t.Fatalf("audit = %q, want [%q]", contaminated, rel)
	}
}

// Discovery failure inside the audit is the fail-closed class so the
// caller rolls back rather than committing an unaudited index.
func TestAuditStagedPathsFailsClosedOnDiscoveryFailure(t *testing.T) {
	notARepo := t.TempDir()
	_, err := AuditStagedPathsForNestedWorktrees(notARepo)
	if err == nil {
		t.Fatal("expected a discovery failure outside a Git repository")
	}
	if !strings.Contains(err.Error(), "Refusing to capture") {
		t.Errorf("failure should carry the fail-closed guidance: %v", err)
	}
}
