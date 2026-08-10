package gitutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnapplyGitPreviewApplyAndRestore(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("feature\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "hello.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, dir, "add", "hello.txt", "new.txt")
	runGitTest(t, dir, "commit", "-q", "-m", "feature")

	patch := runGitTest(t, dir, "show", "--format=", "--binary", "HEAD")
	paths := FilesInPatch(patch)
	snapshot, err := SnapshotWorktreePaths(dir, paths)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidatePatchReverse(dir, patch); err != nil {
		t.Fatal(err)
	}
	if err := PreviewReverseApply(dir, patch); err != nil {
		t.Fatal(err)
	}
	if err := ReverseApply(dir, patch); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello\n" {
		t.Fatalf("hello.txt after reverse apply = %q, want hello", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("new.txt must be absent after reverse apply, err=%v", err)
	}

	if err := snapshot.Restore(dir); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(filepath.Join(dir, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "feature\n" {
		t.Fatalf("hello.txt after restore = %q, want feature", got)
	}
	info, err := os.Stat(filepath.Join(dir, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("hello.txt mode after restore = %o, want 755", info.Mode().Perm())
	}
	if got, err := os.ReadFile(filepath.Join(dir, "new.txt")); err != nil || string(got) != "new\n" {
		t.Fatalf("new.txt after restore = %q, %v", got, err)
	}
}

func TestSnapshotWorktreePathsRejectsEscape(t *testing.T) {
	dir := t.TempDir()
	if _, err := SnapshotWorktreePaths(dir, []string{"../outside"}); err == nil {
		t.Fatal("expected path traversal refusal")
	}
}

func TestGitOperationInProgress(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	if operation, err := GitOperationInProgress(dir); err != nil || operation != "" {
		t.Fatalf("idle operation = %q, %v", operation, err)
	}
	gitDir := strings.TrimSpace(runGitTest(t, dir, "rev-parse", "--git-dir"))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(dir, gitDir)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "CHERRY_PICK_HEAD"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if operation, err := GitOperationInProgress(dir); err != nil || operation != "cherry-pick" {
		t.Fatalf("operation = %q, %v, want cherry-pick", operation, err)
	}
}
