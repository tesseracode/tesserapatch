package candidate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	root, head := initCandidateRepo(t)
	candidateRoot, err := CreateCandidateRoot(root, "demo-feature", "rc_abcdef012345", head)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}
	defer candidateRoot.Prune()

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

func TestCandidateGitSubprocessesUseNoLazyFetch(t *testing.T) {
	root, head := initCandidateRepo(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shimDir := t.TempDir()
	logPath := filepath.Join(shimDir, "env.log")
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\037%%s\n' "${GIT_NO_LAZY_FETCH-}" "$*" >> %q
exec %q "$@"
`, logPath, realGit)
	if err := os.WriteFile(filepath.Join(shimDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GIT_NO_LAZY_FETCH", "0")

	ref, err := OpenReferenceRoot(root, head)
	if err != nil {
		t.Fatalf("OpenReferenceRoot: %v", err)
	}
	if _, err := ref.ReadFile("tracked.txt"); err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	candidateRoot, err := CreateCandidateRoot(root, "demo-feature", "rc_fedcba987654", head)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}
	if err := candidateRoot.Prune(); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(logBytes)), "\n") {
		fields := strings.Split(line, "\x1f")
		if len(fields) != 2 {
			t.Fatalf("unexpected env log row %q", line)
		}
		if fields[0] != "1" {
			t.Fatalf("candidate git call lost GIT_NO_LAZY_FETCH=1: %q", line)
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
