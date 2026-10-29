package candidate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/candidatelock"
	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

func candidateGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func initCandidateRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	candidateGit(t, root, "init", "-q", "-b", "main", ".")
	candidateGit(t, root, "config", "user.email", "candidate@example.com")
	candidateGit(t, root, "config", "user.name", "Candidate")
	candidateGit(t, root, "config", "commit.gpgsign", "false")
	if err := os.MkdirAll(filepath.Join(root, ".tpatch"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	candidateGit(t, root, "add", "tracked.txt")
	candidateGit(t, root, "commit", "-qm", "seed")
	return root, candidateGit(t, root, "rev-parse", "HEAD")
}

func TestOpenReferenceRootUsesCommitSnapshot(t *testing.T) {
	root, head := initCandidateRepo(t)
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("live-worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ref, err := OpenReferenceRoot(root, head)
	if err != nil {
		t.Fatalf("OpenReferenceRoot: %v", err)
	}
	t.Cleanup(func() { _ = ref.Close() })

	body, err := ref.ReadFile("tracked.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(body) != "v1\n" {
		t.Fatalf("reference read drifted to live worktree bytes: %q", string(body))
	}
}

func TestCreateCandidateRootNamespaceAndPrune(t *testing.T) {
	root, head := initCandidateRepo(t)
	candidateID := "rc_0123456789ab"
	candidateRoot, err := CreateCandidateRoot(root, "demo-feature", candidateID, head)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}
	want := filepath.Join(root, ".tpatch", "reconcile-candidates", "demo-feature", candidateID, "worktree")
	if candidateRoot.WorktreePath() != want {
		t.Fatalf("worktree path = %q, want %q", candidateRoot.WorktreePath(), want)
	}
	got, err := candidateRoot.ReadFile("tracked.txt")
	if err != nil {
		t.Fatalf("candidate ReadFile: %v", err)
	}
	if string(got) != "v1\n" {
		t.Fatalf("candidate content = %q, want v1\\n", string(got))
	}
	before := candidateGit(t, root, "rev-parse", "HEAD")
	if err := candidateRoot.Prune(); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if _, err := os.Stat(candidateRoot.WorktreePath()); !os.IsNotExist(err) {
		t.Fatalf("candidate worktree residue remains: %v", err)
	}
	after := candidateGit(t, root, "rev-parse", "HEAD")
	if before != after {
		t.Fatalf("prune changed branch history: before=%s after=%s", before, after)
	}
}

func TestCreateCandidateRootRequiresFeatureLock(t *testing.T) {
	root, head := initCandidateRepo(t)
	held, err := candidatelock.TryAcquire(filepath.Join(root, ".tpatch", "locks", "reconcile", "demo.lock"))
	if err != nil {
		t.Fatalf("hold feature lock: %v", err)
	}
	defer held.Release()

	_, err = CreateCandidateRoot(root, "demo", "rc_111111111111", head)
	if !candidatelock.IsCode(err, candidatelock.CodeCandidateLockHeld) {
		t.Fatalf("CreateCandidateRoot err=%v, want %s", err, candidatelock.CodeCandidateLockHeld)
	}
}

func TestCreateCandidateRootGlobalLockFailureReleasesFeatureLock(t *testing.T) {
	root, head := initCandidateRepo(t)
	globalPath := filepath.Join(root, ".tpatch", "locks", "worktree-mutation.lock")
	global, err := candidatelock.TryAcquire(globalPath)
	if err != nil {
		t.Fatalf("hold global lock: %v", err)
	}
	defer global.Release()

	_, err = CreateCandidateRoot(root, "demo", "rc_222222222222", head)
	if !candidatelock.IsCode(err, candidatelock.CodeCandidateLockHeld) {
		t.Fatalf("CreateCandidateRoot err=%v, want %s", err, candidatelock.CodeCandidateLockHeld)
	}

	featurePath := filepath.Join(root, ".tpatch", "locks", "reconcile", "demo.lock")
	feature, err := candidatelock.TryAcquire(featurePath)
	if err != nil {
		t.Fatalf("feature lock was not released after global failure: %v", err)
	}
	_ = feature.Release()
}

func TestCandidateReadFileRefusesTraversalAndSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink setup on windows requires privileges in CI")
	}
	root, head := initCandidateRepo(t)
	candidateRoot, err := CreateCandidateRoot(root, "demo-feature", "rc_abcdef012345", head)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}
	t.Cleanup(func() {
		if !candidateRoot.closed {
			_ = candidateRoot.Prune()
		}
	})

	if _, err := candidateRoot.ReadFile("../tracked.txt"); err == nil {
		t.Fatal("path traversal unexpectedly succeeded")
	}
	if err := os.WriteFile(filepath.Join(root, "outside.txt"), []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "outside.txt"), filepath.Join(candidateRoot.WorktreePath(), "escaped.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := candidateRoot.ReadFile("escaped.txt"); err == nil {
		t.Fatal("symlink leaf unexpectedly read")
	}
}

func TestCandidateReadFileUsesHeldRootAfterPathReplacement(t *testing.T) {
	root, head := initCandidateRepo(t)
	candidateRoot, err := CreateCandidateRoot(root, "demo-feature", "rc_abcdef012346", head)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}
	defer candidateRoot.Close()

	moved := candidateRoot.WorktreePath() + "-moved"
	if err := os.Rename(candidateRoot.WorktreePath(), moved); err != nil {
		t.Fatalf("rename held worktree: %v", err)
	}
	if err := os.MkdirAll(candidateRoot.WorktreePath(), 0o755); err != nil {
		t.Fatalf("create replacement worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(candidateRoot.WorktreePath(), "tracked.txt"), []byte("replacement\n"), 0o644); err != nil {
		t.Fatalf("write replacement tracked file: %v", err)
	}

	body, err := candidateRoot.ReadFile("tracked.txt")
	if err != nil {
		t.Fatalf("ReadFile from held root: %v", err)
	}
	if string(body) != "v1\n" {
		t.Fatalf("held root read redirected to replacement path: %q", string(body))
	}

	_ = os.RemoveAll(candidateRoot.WorktreePath())
	if err := os.Rename(moved, candidateRoot.WorktreePath()); err != nil {
		t.Fatalf("restore moved worktree: %v", err)
	}
	if err := candidateRoot.Prune(); err != nil {
		t.Fatalf("Prune after restore: %v", err)
	}
}

func TestPruneRefusesReplacementPathAndPreservesReplacementData(t *testing.T) {
	root, head := initCandidateRepo(t)
	candidateRoot, err := CreateCandidateRoot(root, "demo-feature", "rc_abcdef012347", head)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}

	moved := candidateRoot.WorktreePath() + "-moved"
	if err := os.Rename(candidateRoot.WorktreePath(), moved); err != nil {
		t.Fatalf("rename held worktree: %v", err)
	}
	if err := os.MkdirAll(candidateRoot.WorktreePath(), 0o755); err != nil {
		t.Fatalf("create replacement worktree: %v", err)
	}
	sentinel := filepath.Join(candidateRoot.WorktreePath(), "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("replacement data\n"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	err = candidateRoot.Prune()
	if err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("Prune = %v, want identity-changed refusal", err)
	}
	if _, statErr := os.Stat(sentinel); statErr != nil {
		t.Fatalf("replacement sentinel missing after prune refusal: %v", statErr)
	}
	if _, statErr := os.Stat(moved); statErr != nil {
		t.Fatalf("held worktree unexpectedly removed: %v", statErr)
	}

	if err := os.RemoveAll(candidateRoot.WorktreePath()); err != nil {
		t.Fatalf("remove replacement path: %v", err)
	}
	if err := os.Rename(moved, candidateRoot.WorktreePath()); err != nil {
		t.Fatalf("restore held worktree path: %v", err)
	}
	if err := candidateRoot.Prune(); err != nil {
		t.Fatalf("Prune after identity restore: %v", err)
	}
}

func TestCandidateRootZeroAndClosedState(t *testing.T) {
	var zero CandidateRoot
	if _, err := zero.ReadFile("tracked.txt"); err == nil {
		t.Fatal("zero CandidateRoot read unexpectedly succeeded")
	}
	if err := zero.Prune(); err == nil {
		t.Fatal("zero CandidateRoot prune unexpectedly succeeded")
	}
	if err := zero.Close(); err == nil {
		t.Fatal("zero CandidateRoot close unexpectedly succeeded")
	}

	root, head := initCandidateRepo(t)
	candidateRoot, err := CreateCandidateRoot(root, "demo-feature", "rc_abcdef012348", head)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}
	if err := candidateRoot.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := candidateRoot.ReadFile("tracked.txt"); err == nil {
		t.Fatal("closed CandidateRoot read unexpectedly succeeded")
	}
}

func TestReferenceRootZeroAndClosedState(t *testing.T) {
	var zero ReferenceRoot
	if _, err := zero.ReadFile("tracked.txt"); err == nil {
		t.Fatal("zero ReferenceRoot read unexpectedly succeeded")
	}
	if err := zero.Close(); err == nil {
		t.Fatal("zero ReferenceRoot close unexpectedly succeeded")
	}

	root, head := initCandidateRepo(t)
	ref, err := OpenReferenceRoot(root, head)
	if err != nil {
		t.Fatalf("OpenReferenceRoot: %v", err)
	}
	if err := ref.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := ref.ReadFile("tracked.txt"); err == nil {
		t.Fatal("closed ReferenceRoot read unexpectedly succeeded")
	}
}

func TestCandidateGitSubprocessesUseNoLazyFetchAndScrubHostileEnv(t *testing.T) {
	targetRoot, targetHead := initCandidateRepo(t)
	foreignRoot, _ := initCandidateRepo(t)
	candidateGit(t, foreignRoot, "checkout", "-q", "-b", "foreign")
	if err := os.WriteFile(filepath.Join(foreignRoot, "tracked.txt"), []byte("foreign\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	candidateGit(t, foreignRoot, "add", "tracked.txt")
	candidateGit(t, foreignRoot, "commit", "-qm", "foreign")

	foreignWorktreesBefore := candidateGit(t, foreignRoot, "worktree", "list", "--porcelain")
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shimDir := t.TempDir()
	logPath := filepath.Join(shimDir, "env.log")
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\037%%s\037%%s\037%%s\037%%s\037%%s\037%%s\037%%s\n' \
  "${GIT_NO_LAZY_FETCH-}" "${LC_ALL-}" "${GIT_NO_REPLACE_OBJECTS-}" \
  "${GIT_DIR-}" "${GIT_WORK_TREE-}" "${GIT_COMMON_DIR-}" "${GIT_INDEX_FILE-}" "$*" >> %q
exec %q "$@"
`, logPath, realGit)
	if err := os.WriteFile(filepath.Join(shimDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GIT_NO_LAZY_FETCH", "0")
	t.Setenv("LC_ALL", "fr_FR.UTF-8")
	t.Setenv("GIT_NO_REPLACE_OBJECTS", "0")
	t.Setenv("GIT_DIR", filepath.Join(foreignRoot, ".git"))
	t.Setenv("GIT_WORK_TREE", foreignRoot)
	t.Setenv("GIT_COMMON_DIR", filepath.Join(foreignRoot, ".git"))
	t.Setenv("GIT_INDEX_FILE", filepath.Join(foreignRoot, ".git", "index"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "alias.cat-file")
	t.Setenv("GIT_CONFIG_VALUE_0", "!false")

	ref, err := OpenReferenceRoot(targetRoot, targetHead)
	if err != nil {
		t.Fatalf("OpenReferenceRoot: %v", err)
	}
	defer ref.Close()
	body, err := ref.ReadFile("tracked.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(body) != "v1\n" {
		t.Fatalf("reference read escaped target repo: %q", string(body))
	}

	candidateRoot, err := CreateCandidateRoot(targetRoot, "demo-feature", "rc_fedcba987654", targetHead)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}
	if err := candidateRoot.Prune(); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	foreignWorktreesAfter := gitWithBinaryAndEnv(t, realGit, foreignRoot, []string{"LC_ALL=C", "LANG=C"}, "worktree", "list", "--porcelain")
	if foreignWorktreesAfter != foreignWorktreesBefore {
		t.Fatalf("foreign repo worktrees mutated by hostile env\nbefore:\n%s\nafter:\n%s", foreignWorktreesBefore, foreignWorktreesAfter)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(logBytes)), "\n") {
		fields := strings.Split(line, "\x1f")
		if len(fields) != 8 {
			t.Fatalf("unexpected env log row %q", line)
		}
		if fields[0] != "1" || fields[1] != "C" || fields[2] != "1" {
			t.Fatalf("candidate git call lost mandatory env envelope: %q", line)
		}
		for index, label := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE"} {
			if fields[3+index] != "" {
				t.Fatalf("candidate git call leaked %s: %q", label, line)
			}
		}
	}
}

func TestOpenReferenceRootMissingPromisorObjectDoesNotFetch(t *testing.T) {
	origin, _ := initCandidateRepo(t)
	clone := filepath.Join(t.TempDir(), "clone")
	candidateGit(t, origin, "-c", "protocol.file.allow=always", "clone", "-q", "--no-checkout", "--filter=blob:none", "--no-local", origin, clone)
	if err := os.MkdirAll(filepath.Join(clone, ".tpatch"), 0o755); err != nil {
		t.Fatal(err)
	}
	candidateGit(t, clone, "remote", "set-url", "origin", "git://127.0.0.1:1/dead-remote")
	head := candidateGit(t, clone, "rev-parse", "HEAD")
	blob := candidateGit(t, clone, "rev-parse", "HEAD:tracked.txt")
	deleteLooseOrPackedObject(t, clone, blob)

	ref, err := OpenReferenceRoot(clone, head)
	if err != nil {
		t.Fatalf("OpenReferenceRoot: %v", err)
	}
	t.Cleanup(func() { _ = ref.Close() })

	_, err = ref.ReadFile("tracked.txt")
	if err == nil {
		t.Fatal("missing promisor object unexpectedly succeeded")
	}
	if gitutil.IsNetworkFetchError(err.Error()) {
		t.Fatalf("missing object reached network despite no-lazy fetch: %v", err)
	}
	if !gitutil.IsMissingObjectError(err.Error()) {
		t.Fatalf("unexpected missing-object error form: %v", err)
	}
}

func TestCandidateWorktreeTreeHashReflectsUncommittedChanges(t *testing.T) {
	root, head := initCandidateRepo(t)
	candidateRoot, err := CreateCandidateRoot(root, "demo-feature", "rc_abcdef012349", head)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}
	defer func() {
		if !candidateRoot.closed {
			_ = candidateRoot.Prune()
		}
	}()

	before, err := candidateRoot.WorktreeTreeHash()
	if err != nil {
		t.Fatalf("WorktreeTreeHash: %v", err)
	}
	if err := os.WriteFile(filepath.Join(candidateRoot.WorktreePath(), "tracked.txt"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := candidateRoot.WorktreeTreeHash()
	if err != nil {
		t.Fatalf("WorktreeTreeHash after edit: %v", err)
	}
	if before == after {
		t.Fatal("WorktreeTreeHash did not change after uncommitted edit")
	}
}

func deleteLooseOrPackedObject(t *testing.T, root, oid string) {
	t.Helper()
	gitDir := candidateGit(t, root, "rev-parse", "--path-format=absolute", "--git-dir")
	loose := filepath.Join(gitDir, "objects", oid[:2], oid[2:])
	if _, err := os.Stat(loose); err == nil {
		if err := os.Remove(loose); err != nil {
			t.Fatalf("remove loose object: %v", err)
		}
		return
	}
	packDir := filepath.Join(gitDir, "objects", "pack")
	entries, err := os.ReadDir(packDir)
	if err != nil {
		t.Fatalf("read pack dir: %v", err)
	}
	stash := t.TempDir()
	var moved []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".pack") && !strings.HasSuffix(name, ".idx") &&
			!strings.HasSuffix(name, ".rev") && !strings.HasSuffix(name, ".promisor") &&
			!strings.HasSuffix(name, ".keep") && !strings.HasSuffix(name, ".mtimes") {
			continue
		}
		dst := filepath.Join(stash, name)
		if err := os.Rename(filepath.Join(packDir, name), dst); err != nil {
			t.Fatalf("move %s: %v", name, err)
		}
		if strings.HasSuffix(name, ".pack") {
			moved = append(moved, dst)
		}
	}
	for _, packPath := range moved {
		data, err := os.Open(packPath)
		if err != nil {
			t.Fatalf("open pack %s: %v", packPath, err)
		}
		cmd := exec.Command("git", "unpack-objects", "-q")
		cmd.Dir = root
		cmd.Stdin = data
		if out, unpackErr := cmd.CombinedOutput(); unpackErr != nil {
			_ = data.Close()
			t.Fatalf("unpack-objects: %v: %s", unpackErr, string(out))
		}
		_ = data.Close()
	}
	if _, err := os.Stat(loose); err != nil {
		t.Fatalf("object %s did not become loose: %v", oid, err)
	}
	if err := os.Remove(loose); err != nil {
		t.Fatalf("remove unpacked object: %v", err)
	}
}

func gitWithBinaryAndEnv(t *testing.T, gitBin, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command(gitBin, args...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git(%s) %v failed: %v\n%s", gitBin, args, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func TestReferenceReadFileMissingPath(t *testing.T) {
	root, head := initCandidateRepo(t)
	ref, err := OpenReferenceRoot(root, head)
	if err != nil {
		t.Fatalf("OpenReferenceRoot: %v", err)
	}
	defer ref.Close()
	_, err = ref.ReadFile("missing.txt")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ReadFile missing err=%v, want fs.ErrNotExist", err)
	}
}
