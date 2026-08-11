package gitutil

import (
	"os"
	"os/exec"
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

func TestPathsAffectedByPatchIncludesRenameAndCopySides(t *testing.T) {
	patch := `diff --git a/my file.txt b/my file.txt
--- a/my file.txt
+++ b/my file.txt
@@ -1 +1 @@
--- not a header
+++ replacement
diff --git a/b/x.txt b/b/x.txt
old mode 100644
new mode 100755
diff --git a/old.txt b/new.txt
similarity index 100%
rename from old.txt
rename to new.txt
diff --git "a/source name.txt" "b/copied name.txt"
similarity index 100%
copy from source name.txt
copy to copied name.txt
`
	got := PathsAffectedByPatch(patch)
	want := map[string]bool{
		"my file.txt":     true,
		"b/x.txt":         true,
		"old.txt":         true,
		"new.txt":         true,
		"source name.txt": true,
		"copied name.txt": true,
	}
	if len(got) != len(want) {
		t.Fatalf("paths = %v", got)
	}
	for _, path := range got {
		if !want[path] {
			t.Fatalf("unexpected path %q in %v", path, got)
		}
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

func TestWorktreeSnapshotRestoresFileDirectoryTransitions(t *testing.T) {
	t.Run("directory-back-over-file", func(t *testing.T) {
		dir := t.TempDir()
		config := filepath.Join(dir, "config")
		if err := os.Mkdir(config, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(config, "default.yaml"), []byte("feature\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		snapshot, err := SnapshotWorktreePaths(dir, []string{"config", "config/default.yaml"})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(config); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(config, []byte("base\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := snapshot.Restore(dir); err != nil {
			t.Fatal(err)
		}
		if info, err := os.Stat(config); err != nil || !info.IsDir() {
			t.Fatalf("config = %v, %v", info, err)
		}
		if got, err := os.ReadFile(filepath.Join(config, "default.yaml")); err != nil || string(got) != "feature\n" {
			t.Fatalf("default.yaml = %q, %v", got, err)
		}
	})

	t.Run("file-back-over-directory", func(t *testing.T) {
		dir := t.TempDir()
		config := filepath.Join(dir, "config")
		if err := os.WriteFile(config, []byte("base\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		snapshot, err := SnapshotWorktreePaths(dir, []string{"config", "config/default.yaml"})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(config); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(config, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(config, "default.yaml"), []byte("feature\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := snapshot.Restore(dir); err != nil {
			t.Fatal(err)
		}
		if info, err := os.Stat(config); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("config = %v, %v", info, err)
		}
		if got, err := os.ReadFile(config); err != nil || string(got) != "base\n" {
			t.Fatalf("config = %q, %v", got, err)
		}
	})
}

func TestDiffFromCommitForPathsUsesLinkedWorktreeIndex(t *testing.T) {
	mainDir := t.TempDir()
	gitInit(t, mainDir)
	linkedParent := t.TempDir()
	linkedDir := filepath.Join(linkedParent, "linked")
	runGitTest(t, mainDir, "worktree", "add", "-q", "-b", "linked-test", linkedDir)
	t.Cleanup(func() {
		cmd := exec.Command("git", "worktree", "remove", "--force", linkedDir)
		cmd.Dir = mainDir
		_ = cmd.Run()
	})

	if err := os.WriteFile(filepath.Join(linkedDir, "hello.txt"), []byte("linked change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, linkedDir, "add", "hello.txt")
	statusBefore := runGitTest(t, linkedDir, "status", "--porcelain=v1")
	indexBefore := runGitTest(t, linkedDir, "write-tree")

	diff, err := DiffFromCommitForPaths(linkedDir, "HEAD", []string{"hello.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "linked change") {
		t.Fatalf("linked-worktree diff missing staged change:\n%s", diff)
	}
	if statusAfter := runGitTest(t, linkedDir, "status", "--porcelain=v1"); statusAfter != statusBefore {
		t.Fatalf("status changed:\nbefore=%q\nafter=%q", statusBefore, statusAfter)
	}
	if indexAfter := runGitTest(t, linkedDir, "write-tree"); indexAfter != indexBefore {
		t.Fatalf("index changed:\nbefore=%s\nafter=%s", indexBefore, indexAfter)
	}
}
