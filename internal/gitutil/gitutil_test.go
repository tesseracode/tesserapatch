package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitInit sets up a minimal git repo with one committed file. Mirrors
// the helper in internal/workflow/reconcile_test.go but kept package-
// local to avoid an import cycle.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "tpatch-test@example.com"},
		{"config", "user.name", "tpatch test"},
		{"config", "commit.gpgsign", "false"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "hello.txt"},
		{"commit", "-q", "-m", "init"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

// TestValidatePatchReverse_RoundtripsAgainstWorkingTree exercises the
// fix for bug-record-validation-false-positive. At record-time, the
// working tree already contains the patch's edits — the old forward
// `git apply --check` would always fail in that scenario. The
// reverse-check must succeed.
func TestValidatePatchReverse_RoundtripsAgainstWorkingTree(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)

	// Make an edit in the working tree (uncommitted).
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Capture the diff as a patch — same shape `tpatch record` saves.
	cmd := exec.Command("git", "diff", "--no-color", "hello.txt")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git diff: %v", err)
	}
	patch := string(out)
	if patch == "" {
		t.Fatal("expected non-empty diff")
	}

	if err := ValidatePatchReverse(dir, patch); err != nil {
		t.Fatalf("reverse-check should succeed at record-time, got: %v", err)
	}

	// And the OLD forward validator must still fail in this scenario,
	// proving the bug it was tripping on (false positive at record-time).
	if err := ValidatePatch(dir, patch, "3way"); err == nil {
		t.Fatal("forward ValidatePatch unexpectedly succeeded against a tree that already contains the patch")
	}
}

// TestValidatePatchReverse_FailsWhenPatchDoesNotMatch covers the inverse:
// if the working tree does NOT contain the patch, the reverse-check
// must fail — preserving warning-as-signal at record-time.
func TestValidatePatchReverse_FailsWhenPatchDoesNotMatch(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)

	// A handcrafted patch that targets a line NOT in hello.txt.
	patch := `diff --git a/hello.txt b/hello.txt
index 0000001..0000002 100644
--- a/hello.txt
+++ b/hello.txt
@@ -1 +1 @@
-something else
+totally different
`
	err := ValidatePatchReverse(dir, patch)
	if err == nil {
		t.Fatal("reverse-check should fail when working tree does not contain the patch")
	}
	if !strings.Contains(err.Error(), "round-trip") {
		t.Fatalf("expected round-trip error message, got: %v", err)
	}
}

func TestValidatePatchReverse_EmptyPatch(t *testing.T) {
	if err := ValidatePatchReverse(t.TempDir(), ""); err == nil {
		t.Fatal("expected error for empty patch")
	}
}

// TestPreflightReconcile covers the four preflight conditions from A10
// doc-reconcile-workflow. Clean tree → Clean() true. Modified tracked
// file → UnstagedFiles populated. Untracked new file → UntrackedFiles.
// Conflict marker → MergeMarkerFiles. *.orig leftover → LeftoverFiles.
// Each case is additive so Clean() transitions to false exactly when
// expected.
func TestPreflightReconcile(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)

	// 1. Clean tree.
	p, err := PreflightReconcile(dir)
	if err != nil {
		t.Fatalf("preflight on clean tree: %v", err)
	}
	if !p.Clean() {
		t.Fatalf("expected clean, got %+v", p)
	}

	// 2. Modify the tracked file.
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ = PreflightReconcile(dir)
	if p.Clean() || len(p.UnstagedFiles) == 0 {
		t.Fatalf("expected unstaged files, got %+v", p)
	}

	// 3. Untracked file.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ = PreflightReconcile(dir)
	found := false
	for _, f := range p.UntrackedFiles {
		if f == "new.txt" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected new.txt untracked, got %+v", p)
	}

	// 4. Conflict marker in a tracked file — need to commit first so
	// `git grep` scans it. Reset the prior dirty edit so modified and
	// marker conditions are clearly separable.
	for _, args := range [][]string{{"checkout", "--", "hello.txt"}} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	markerContent := "line1\n<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> branch\nline2\n"
	if err := os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte(markerContent), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "conflict.txt"},
		{"commit", "-q", "-m", "add conflict file"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	p, _ = PreflightReconcile(dir)
	if len(p.MergeMarkerFiles) == 0 {
		t.Fatalf("expected merge marker detection, got %+v", p)
	}

	// 5. *.orig leftover.
	if err := os.WriteFile(filepath.Join(dir, "x.txt.orig"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ = PreflightReconcile(dir)
	foundLeftover := false
	for _, f := range p.LeftoverFiles {
		if strings.HasSuffix(f, "x.txt.orig") {
			foundLeftover = true
		}
	}
	if !foundLeftover {
		t.Fatalf("expected *.orig detection, got %+v", p)
	}
}

func TestIsPathTracked(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	if !IsPathTracked(dir, "hello.txt") {
		t.Fatal("hello.txt should be tracked")
	}
	if IsPathTracked(dir, "missing.txt") {
		t.Fatal("missing.txt should not be tracked")
	}
}

// TestIsAncestor exercises the three documented outcomes of
// `git merge-base --is-ancestor`:
//   - ancestor reachable -> (true, nil)
//   - ancestor unreachable but repo healthy -> (false, nil)
//   - bogus ref / real git failure -> (false, err)
func TestIsAncestor(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)

	first, err := HeadCommit(dir)
	if err != nil {
		t.Fatalf("HeadCommit: %v", err)
	}

	// Add a second commit so we have a non-trivial chain.
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "hello.txt"},
		{"commit", "-q", "-m", "second"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	second, err := HeadCommit(dir)
	if err != nil {
		t.Fatalf("HeadCommit: %v", err)
	}

	// first is an ancestor of HEAD.
	if ok, err := IsAncestor(dir, first, "HEAD"); err != nil || !ok {
		t.Fatalf("IsAncestor(first, HEAD) = (%v, %v), want (true, nil)", ok, err)
	}
	// HEAD (=second) is not an ancestor of first.
	if ok, err := IsAncestor(dir, second, first); err != nil || ok {
		t.Fatalf("IsAncestor(second, first) = (%v, %v), want (false, nil)", ok, err)
	}
	// Bogus SHA -> real failure (non-zero, non-1 exit).
	if _, err := IsAncestor(dir, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "HEAD"); err == nil {
		t.Fatal("IsAncestor with bogus ancestor sha should error, got nil")
	}
}
