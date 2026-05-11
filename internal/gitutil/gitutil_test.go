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

// TestValidatePatchReverse_MarkdownBlockquoteRoundtrip is the regression
// test for bug-record-roundtrip-false-positive-markdown.
//
// Shape: HEAD has a small markdown file. Working tree adds a multi-paragraph
// blockquote (`> [!CAUTION]` style) whose final added line ends in a trailing
// space — a perfectly common pattern in markdown (deliberate soft-break, or
// just an empty blockquote continuation `> `). The captured patch must
// round-trip via `git apply --reverse --check` against that same working
// tree.
//
// Pre-fix: CapturePatchScoped called strings.TrimSpace on the whole patch,
// which ate the trailing space on the final `+> ` line, producing a patch
// whose last hunk line was `+>`. ValidatePatchReverse then rejected it
// (correctly — the patch no longer described the on-disk state) and tpatch
// record emitted a misleading round-trip warning. The corrupted patch was
// also persisted to disk under patches/NNN-record.patch.
//
// Post-fix: capture preserves trailing whitespace on content lines and only
// normalizes the count of trailing newlines.
func TestValidatePatchReverse_MarkdownBlockquoteRoundtrip(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)

	// Commit a baseline README.
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Project\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "README.md"},
		{"commit", "-q", "-m", "add readme"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	// Insert a multi-paragraph blockquote whose last body line is `> `
	// (trailing space). Mimics the t3code v0.4.3 readme-copilot-notice
	// smoke-test failure shape.
	newContent := "# Project\n" +
		"> [!CAUTION]\n" +
		"> This project uses smart quotes — “like these”.\n" +
		">\n" +
		"> Multi-paragraph caution body that exercises both non-ASCII\n" +
		"> codepoints and a final blockquote-continuation line whose\n" +
		"> trailing space is semantically meaningful.\n" +
		"> \n"
	if err := os.WriteFile(readme, []byte(newContent), 0o644); err != nil {
		t.Fatal(err)
	}

	patch, err := CapturePatchScoped(dir, nil)
	if err != nil {
		t.Fatalf("CapturePatchScoped: %v", err)
	}
	if patch == "" {
		t.Fatal("expected non-empty captured patch")
	}
	// Guard rail: the captured patch must still contain the trailing
	// space on the final `+> ` line. Pre-fix this assertion failed.
	if !strings.Contains(patch, "+> \n") {
		t.Fatalf("captured patch lost trailing whitespace on blockquote line.\nPatch:\n%s", patch)
	}
	// Guard rail: a single trailing newline (no extras, no missing).
	if !strings.HasSuffix(patch, "\n") || strings.HasSuffix(patch, "\n\n") {
		t.Fatalf("captured patch must end with exactly one trailing newline; got %q at tail", patch[max(0, len(patch)-8):])
	}

	if err := ValidatePatchReverse(dir, patch); err != nil {
		t.Fatalf("reverse-check should succeed for markdown blockquote round-trip, got: %v\nPatch:\n%s", err, patch)
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
