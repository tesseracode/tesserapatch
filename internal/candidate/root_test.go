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

func acquireCandidateLocks(t *testing.T, root, slug string) (*candidatelock.Handle, *candidatelock.Handle) {
	t.Helper()
	featurePath := filepath.Join(root, ".tpatch", "locks", "reconcile", slug+".lock")
	feature, err := candidatelock.TryAcquire(featurePath)
	if err != nil {
		t.Fatalf("acquire feature lock: %v", err)
	}
	globalPath := filepath.Join(root, ".tpatch", "locks", "worktree-mutation.lock")
	global, err := candidatelock.TryAcquire(globalPath)
	if err != nil {
		_ = feature.Release()
		t.Fatalf("acquire global lock: %v", err)
	}
	return feature, global
}

func releaseCandidateLocks(t *testing.T, feature, global *candidatelock.Handle) {
	t.Helper()
	if global != nil {
		if err := global.Release(); err != nil {
			t.Fatalf("release global lock: %v", err)
		}
	}
	if feature != nil {
		if err := feature.Release(); err != nil {
			t.Fatalf("release feature lock: %v", err)
		}
	}
}

func assertLockHeld(t *testing.T, path string) {
	t.Helper()
	h, err := candidatelock.TryAcquire(path)
	if err == nil {
		_ = h.Release()
		t.Fatalf("expected lock to remain held for %s", path)
	}
	if !candidatelock.IsCode(err, candidatelock.CodeCandidateLockHeld) {
		t.Fatalf("lock error=%v, want %s", err, candidatelock.CodeCandidateLockHeld)
	}
}

func assertLockReacquireAfterRelease(t *testing.T, path string) {
	t.Helper()
	h, err := candidatelock.TryAcquire(path)
	if err != nil {
		t.Fatalf("reacquire lock %s: %v", path, err)
	}
	if err := h.Release(); err != nil {
		t.Fatalf("release reacquired lock %s: %v", path, err)
	}
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
	featureLock, globalLock := acquireCandidateLocks(t, root, "demo-feature")
	defer releaseCandidateLocks(t, featureLock, globalLock)

	candidateRoot, err := CreateCandidateRoot(root, "demo-feature", candidateID, head, featureLock, globalLock)
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
	if err := candidateRoot.Prune(featureLock, globalLock); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if _, err := os.Stat(candidateRoot.WorktreePath()); !os.IsNotExist(err) {
		t.Fatalf("candidate worktree residue remains: %v", err)
	}
	after := candidateGit(t, root, "rev-parse", "HEAD")
	if before != after {
		t.Fatalf("prune changed branch history: before=%s after=%s", before, after)
	}

	assertLockHeld(t, filepath.Join(root, ".tpatch", "locks", "reconcile", "demo-feature.lock"))
	assertLockHeld(t, filepath.Join(root, ".tpatch", "locks", "worktree-mutation.lock"))
}

func TestOpenExistingCandidateRootReusesTypedReference(t *testing.T) {
	root, head := initCandidateRepo(t)
	featureLock, globalLock := acquireCandidateLocks(t, root, "demo-feature")
	defer releaseCandidateLocks(t, featureLock, globalLock)

	created, err := CreateCandidateRoot(root, "demo-feature", "rc_0123456789ab", head, featureLock, globalLock)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}
	if err := created.Close(); err != nil {
		t.Fatalf("close created root: %v", err)
	}

	reference, err := OpenReferenceRoot(root, head)
	if err != nil {
		t.Fatalf("OpenReferenceRoot: %v", err)
	}
	defer func() { _ = reference.Close() }()

	reopened, err := OpenExistingCandidateRoot(reference, "demo-feature", "rc_0123456789ab")
	if err != nil {
		t.Fatalf("OpenExistingCandidateRoot: %v", err)
	}
	body, err := reopened.ReadFile("tracked.txt")
	if err != nil || string(body) != "v1\n" {
		t.Fatalf("reopened candidate did not expose immutable worktree bytes: %q err=%v", body, err)
	}
	if err := reopened.Prune(featureLock, globalLock); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".tpatch", "reconcile-candidates", "demo-feature", "rc_0123456789ab", "worktree")); !os.IsNotExist(err) {
		t.Fatalf("worktree residue remains after reopen+prune: %v", err)
	}
}

func TestCandidateMutationAPIsRequireFeatureLockCapability(t *testing.T) {
	root, head := initCandidateRepo(t)
	featureLock, globalLock := acquireCandidateLocks(t, root, "demo-feature")
	featurePath := filepath.Join(root, ".tpatch", "locks", "reconcile", "demo-feature.lock")

	candidateRoot, err := CreateCandidateRoot(root, "demo-feature", "rc_111111111111", head, featureLock, globalLock)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}
	if err := featureLock.Release(); err != nil {
		t.Fatalf("release seed feature lock: %v", err)
	}
	featureLock = nil
	holder, err := candidatelock.TryAcquire(featurePath)
	if err != nil {
		t.Fatalf("reacquire feature lock: %v", err)
	}

	state := CandidateState{
		SchemaVersion:    CandidateSchemaVersion,
		CandidateID:      "rc_111111111111",
		Status:           CandidateStatusPending,
		AcceptValidation: CandidateAcceptValidationNone,
	}
	if err := candidateRoot.WriteState(state, nil); err == nil {
		t.Fatal("WriteState accepted nil feature lock")
	}
	zero := &candidatelock.Handle{}
	if err := candidateRoot.WritePatch([]byte("patch"), zero); err == nil {
		t.Fatal("WritePatch accepted uninitialized feature lock")
	}
	releasedHandle, err := candidatelock.TryAcquire(filepath.Join(root, ".tpatch", "locks", "reconcile", "released.lock"))
	if err != nil {
		t.Fatalf("acquire released lock fixture: %v", err)
	}
	if err := releasedHandle.Release(); err != nil {
		t.Fatalf("release released lock fixture: %v", err)
	}
	if err := candidateRoot.WriteArtifact(testCandidateArtifact(t, "applied"), releasedHandle); err == nil {
		t.Fatal("WriteArtifact accepted released feature lock")
	}
	if _, err := os.Stat(filepath.Join(root, ".tpatch", "reconcile-candidates", "demo-feature", "rc_111111111111", CandidateMetadataName)); !os.IsNotExist(err) {
		t.Fatalf("WriteArtifact mutated candidate metadata without capability: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".tpatch", "reconcile-candidates", "demo-feature", "rc_111111111111", CandidateStateName)); !os.IsNotExist(err) {
		t.Fatalf("WriteState mutated candidate state without capability: %v", err)
	}
	if err := candidateRoot.WriteState(state, holder); err != nil {
		t.Fatalf("WriteState with valid feature lock: %v", err)
	}
	if err := candidateRoot.WritePatch([]byte("diff --git a/a b/a\n"), holder); err != nil {
		t.Fatalf("WritePatch with valid feature lock: %v", err)
	}

	reference, err := OpenReferenceRoot(root, head)
	if err != nil {
		t.Fatalf("OpenReferenceRoot: %v", err)
	}
	defer func() { _ = reference.Close() }()
	record := RejectedRecordFromArtifact(testCandidateArtifact(t, "applied"))
	if err := reference.AppendRejectedRecord("demo-feature", record, nil); err == nil {
		t.Fatal("AppendRejectedRecord accepted nil feature lock")
	}
	wrongFeatureLock, err := candidatelock.TryAcquire(filepath.Join(root, ".tpatch", "locks", "reconcile", "other-feature.lock"))
	if err != nil {
		t.Fatalf("acquire wrong feature lock: %v", err)
	}
	defer func() { _ = wrongFeatureLock.Release() }()
	if err := reference.AppendRejectedRecord("demo-feature", record, wrongFeatureLock); err == nil {
		t.Fatal("AppendRejectedRecord accepted wrong feature lock")
	}
	if _, err := os.Stat(filepath.Join(root, ".tpatch", "reconcile-candidates", "demo-feature", CandidateRejectedName)); !os.IsNotExist(err) {
		t.Fatalf("AppendRejectedRecord mutated rejection log without capability: %v", err)
	}
	if err := reference.AppendRejectedRecord("demo-feature", record, holder); err != nil {
		t.Fatalf("AppendRejectedRecord with valid feature lock: %v", err)
	}
	if err := reference.ClearRejectedRecords("demo-feature", holder); err != nil {
		t.Fatalf("ClearRejectedRecords with valid feature lock: %v", err)
	}

	if err := candidateRoot.Prune(holder, globalLock); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if err := holder.Release(); err != nil {
		t.Fatalf("release holder feature lock: %v", err)
	}
	if err := globalLock.Release(); err != nil {
		t.Fatalf("release global lock: %v", err)
	}
}

func TestOpenExistingCandidateRootRejectsSlugSymlinkAlias(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture is not portable on windows")
	}
	root, head := initCandidateRepo(t)
	featureLock, globalLock := acquireCandidateLocks(t, root, "real-feature")
	defer releaseCandidateLocks(t, featureLock, globalLock)

	candidateID := "rc_222222222222"
	created, err := CreateCandidateRoot(root, "real-feature", candidateID, head, featureLock, globalLock)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}
	sentinel := filepath.Join(created.WorktreePath(), "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("keep\n"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	slugRoot := filepath.Join(root, ".tpatch", "reconcile-candidates")
	if err := os.Symlink(
		filepath.Join(slugRoot, "real-feature"),
		filepath.Join(slugRoot, "alias-feature"),
	); err != nil {
		t.Fatalf("create slug alias symlink: %v", err)
	}

	reference, err := OpenReferenceRoot(root, head)
	if err != nil {
		t.Fatalf("OpenReferenceRoot: %v", err)
	}
	defer func() { _ = reference.Close() }()
	if _, err := OpenExistingCandidateRoot(reference, "alias-feature", candidateID); err == nil {
		t.Fatal("OpenExistingCandidateRoot accepted slug symlink alias")
	}
	body, err := os.ReadFile(sentinel)
	if err != nil || string(body) != "keep\n" {
		t.Fatalf("alias reopen refusal mutated real candidate worktree: %q err=%v", body, err)
	}
	if err := created.Prune(featureLock, globalLock); err != nil {
		t.Fatalf("Prune: %v", err)
	}
}

func TestOpenExistingCandidateRootRejectsRegisteredSuffixImpostor(t *testing.T) {
	root, head := initCandidateRepo(t)
	const slug, id = "suffix-feature", "rc_333333333333"
	relative := filepath.Join(".tpatch", "reconcile-candidates", slug, id, "worktree")
	registered := filepath.Join(t.TempDir(), relative)
	if err := os.MkdirAll(filepath.Dir(registered), 0o755); err != nil {
		t.Fatal(err)
	}
	candidateGit(t, root, "worktree", "add", "--detach", "-q", registered, head)
	gitfile, err := os.ReadFile(filepath.Join(registered, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	impostor := filepath.Join(root, relative)
	if err := os.MkdirAll(impostor, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(impostor, ".git"), gitfile, 0o644); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(impostor, "not-owned.txt")
	if err := os.WriteFile(sentinel, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reference, err := OpenReferenceRoot(root, head)
	if err != nil {
		t.Fatal(err)
	}
	defer reference.Close()
	reopened, err := OpenExistingCandidateRoot(reference, slug, id)
	if err == nil {
		_ = reopened.Close()
		t.Error("matching suffix granted ownership of an unregistered directory")
	}
	if body, err := os.ReadFile(sentinel); err != nil || string(body) != "keep\n" {
		t.Fatalf("unowned directory changed: %q %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(registered, ".git")); err != nil {
		t.Fatalf("real registration changed: %v", err)
	}
}

func TestOpenExistingCandidateRootRejectsOriginalGitDirectoryAlias(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires Unix")
	}
	root, head := initCandidateRepo(t)
	feature, global := acquireCandidateLocks(t, root, "git-alias")
	defer releaseCandidateLocks(t, feature, global)
	created, err := CreateCandidateRoot(root, "git-alias", "rc_444444444444", head, feature, global)
	if err != nil {
		t.Fatal(err)
	}
	defer created.Close()
	gitfilePath := filepath.Join(created.WorktreePath(), ".git")
	original, err := os.ReadFile(gitfilePath)
	if err != nil {
		t.Fatal(err)
	}
	gitdir, err := parseWorktreeGitfile(created.WorktreePath(), string(original))
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(filepath.Dir(gitdir), "aliased-registry-entry")
	if err := os.Symlink(gitdir, alias); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gitfilePath, []byte("gitdir: "+alias+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reference, err := OpenReferenceRoot(root, head)
	if err != nil {
		t.Fatal(err)
	}
	defer reference.Close()
	reopened, err := OpenExistingCandidateRoot(reference, "git-alias", "rc_444444444444")
	if err == nil {
		_ = reopened.Close()
		t.Error("Git directory alias granted ownership")
	}
}

func TestCreateCandidateRootRejectsInvalidLockCapabilities(t *testing.T) {
	root, head := initCandidateRepo(t)
	repo2, _ := initCandidateRepo(t)
	featureLock, globalLock := acquireCandidateLocks(t, root, "demo-feature")
	defer releaseCandidateLocks(t, featureLock, globalLock)
	wrongRepoFeature, wrongRepoGlobal := acquireCandidateLocks(t, repo2, "demo-feature")
	defer releaseCandidateLocks(t, wrongRepoFeature, wrongRepoGlobal)

	featurePath := filepath.Join(root, ".tpatch", "locks", "reconcile", "demo-feature.lock")
	globalPath := filepath.Join(root, ".tpatch", "locks", "worktree-mutation.lock")

	for _, tc := range []struct {
		name      string
		feature   *candidatelock.Handle
		global    *candidatelock.Handle
		prepare   func(t *testing.T)
		wantError string
	}{
		{
			name:      "nil-feature",
			feature:   nil,
			global:    globalLock,
			wantError: "feature lock handle is required",
		},
		{
			name:      "nil-global",
			feature:   featureLock,
			global:    nil,
			wantError: "worktree mutation lock handle is required",
		},
		{
			name: "zero-feature",
			feature: func() *candidatelock.Handle {
				var zero candidatelock.Handle
				return &zero
			}(),
			global:    globalLock,
			wantError: "not initialized",
		},
		{
			name: "closed-feature",
			feature: func() *candidatelock.Handle {
				h, err := candidatelock.TryAcquire(filepath.Join(root, ".tpatch", "locks", "reconcile", "closed.lock"))
				if err != nil {
					t.Fatalf("acquire closed feature lock: %v", err)
				}
				if err := h.Release(); err != nil {
					t.Fatalf("release closed feature lock: %v", err)
				}
				return h
			}(),
			global:    globalLock,
			wantError: "released",
		},
		{
			name:      "duplicate-handle",
			feature:   featureLock,
			global:    featureLock,
			wantError: "must be distinct",
		},
		{
			name:      "swapped-role",
			feature:   globalLock,
			global:    featureLock,
			wantError: "path mismatch",
		},
		{
			name: "wrong-feature",
			feature: func() *candidatelock.Handle {
				h, err := candidatelock.TryAcquire(filepath.Join(root, ".tpatch", "locks", "reconcile", "other-feature.lock"))
				if err != nil {
					t.Fatalf("acquire wrong-feature lock: %v", err)
				}
				t.Cleanup(func() { _ = h.Release() })
				return h
			}(),
			global:    globalLock,
			wantError: "path mismatch",
		},
		{
			name:      "wrong-repo",
			feature:   wrongRepoFeature,
			global:    wrongRepoGlobal,
			wantError: "path mismatch",
		},
		{
			name:    "replaced-feature-path",
			feature: featureLock,
			global:  globalLock,
			prepare: func(t *testing.T) {
				if runtime.GOOS == "windows" {
					t.Skip("lock path replacement is not portable on windows")
				}
				moved := featurePath + ".moved"
				if err := os.Rename(featurePath, moved); err != nil {
					t.Fatalf("rename feature lock path: %v", err)
				}
				t.Cleanup(func() {
					_ = os.Remove(featurePath)
					_ = os.Rename(moved, featurePath)
				})
				if err := os.WriteFile(featurePath, []byte("replacement"), 0o600); err != nil {
					t.Fatalf("create replacement lock file: %v", err)
				}
			},
			wantError: "identity changed",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.prepare != nil {
				tc.prepare(t)
			}
			_, err := CreateCandidateRoot(root, "demo-feature", "rc_abcdef012345", head, tc.feature, tc.global)
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("CreateCandidateRoot err=%v, want substring %q", err, tc.wantError)
			}
			worktreePath := filepath.Join(root, ".tpatch", "reconcile-candidates", "demo-feature", "rc_abcdef012345", "worktree")
			if _, statErr := os.Stat(worktreePath); !os.IsNotExist(statErr) {
				t.Fatalf("invalid capability mutated candidate namespace: %v", statErr)
			}
			if tc.name != "replaced-feature-path" {
				assertLockHeld(t, featurePath)
			}
			assertLockHeld(t, globalPath)
		})
	}

	assertLockHeld(t, featurePath)
	assertLockHeld(t, globalPath)
}

func TestCandidateReadFileRefusesTraversalAndSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink setup on windows requires privileges in CI")
	}
	root, head := initCandidateRepo(t)
	featureLock, globalLock := acquireCandidateLocks(t, root, "demo-feature")
	defer releaseCandidateLocks(t, featureLock, globalLock)

	candidateRoot, err := CreateCandidateRoot(root, "demo-feature", "rc_abcdef012345", head, featureLock, globalLock)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}
	t.Cleanup(func() {
		if !candidateRoot.closed {
			_ = candidateRoot.Prune(featureLock, globalLock)
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
	featureLock, globalLock := acquireCandidateLocks(t, root, "demo-feature")
	defer releaseCandidateLocks(t, featureLock, globalLock)

	candidateRoot, err := CreateCandidateRoot(root, "demo-feature", "rc_abcdef012346", head, featureLock, globalLock)
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
	if err := candidateRoot.Prune(featureLock, globalLock); err != nil {
		t.Fatalf("Prune after restore: %v", err)
	}
}

func TestPruneRefusesReplacementPathAndPreservesReplacementData(t *testing.T) {
	root, head := initCandidateRepo(t)
	featureLock, globalLock := acquireCandidateLocks(t, root, "demo-feature")
	defer releaseCandidateLocks(t, featureLock, globalLock)

	candidateRoot, err := CreateCandidateRoot(root, "demo-feature", "rc_abcdef012347", head, featureLock, globalLock)
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

	err = candidateRoot.Prune(featureLock, globalLock)
	if err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("Prune = %v, want identity-changed refusal", err)
	}
	if _, statErr := os.Stat(sentinel); statErr != nil {
		t.Fatalf("replacement sentinel missing after prune refusal: %v", statErr)
	}
	if _, statErr := os.Stat(moved); statErr != nil {
		t.Fatalf("held worktree unexpectedly removed: %v", statErr)
	}
	if candidateRoot.closed {
		t.Fatal("candidate root closed on pre-mutation identity refusal")
	}

	if err := os.RemoveAll(candidateRoot.WorktreePath()); err != nil {
		t.Fatalf("remove replacement path: %v", err)
	}
	if err := os.Rename(moved, candidateRoot.WorktreePath()); err != nil {
		t.Fatalf("restore held worktree path: %v", err)
	}
	if err := candidateRoot.Prune(featureLock, globalLock); err != nil {
		t.Fatalf("Prune after identity restore: %v", err)
	}
}

func TestPruneGitRemoveFailureClosesRootAndKeepsCallerLocks(t *testing.T) {
	root, head := initCandidateRepo(t)
	featureLock, globalLock := acquireCandidateLocks(t, root, "demo-feature")
	featurePath := filepath.Join(root, ".tpatch", "locks", "reconcile", "demo-feature.lock")
	globalPath := filepath.Join(root, ".tpatch", "locks", "worktree-mutation.lock")
	defer releaseCandidateLocks(t, featureLock, globalLock)

	candidateRoot, err := CreateCandidateRoot(root, "demo-feature", "rc_abcdef012351", head, featureLock, globalLock)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shimDir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "worktree" ] && [ "$2" = "remove" ]; then
  echo "forced remove failure" >&2
  exit 1
fi
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(shimDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err = candidateRoot.Prune(featureLock, globalLock)
	if err == nil || !strings.Contains(err.Error(), "git worktree remove failed") {
		t.Fatalf("Prune err=%v, want git worktree remove failure", err)
	}
	if !candidateRoot.closed {
		t.Fatal("candidate root should be closed after git remove failure")
	}
	if _, readErr := candidateRoot.ReadFile("tracked.txt"); readErr == nil {
		t.Fatal("closed candidate root unexpectedly remained readable")
	}
	assertLockHeld(t, featurePath)
	assertLockHeld(t, globalPath)

	releaseCandidateLocks(t, featureLock, globalLock)
	featureLock, globalLock = nil, nil
	assertLockReacquireAfterRelease(t, featurePath)
	assertLockReacquireAfterRelease(t, globalPath)
}

func TestPruneResidueCleanupFailureClosesRootAndKeepsCallerLocks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink residue fixture is POSIX-specific in CI")
	}
	root, head := initCandidateRepo(t)
	featureLock, globalLock := acquireCandidateLocks(t, root, "demo-feature")
	featurePath := filepath.Join(root, ".tpatch", "locks", "reconcile", "demo-feature.lock")
	globalPath := filepath.Join(root, ".tpatch", "locks", "worktree-mutation.lock")
	defer releaseCandidateLocks(t, featureLock, globalLock)

	candidateRoot, err := CreateCandidateRoot(root, "demo-feature", "rc_abcdef012352", head, featureLock, globalLock)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shimDir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "worktree" ] && [ "$2" = "remove" ]; then
  target="$4"
  mv "$target" "${target}.original"
  mkdir -p "${target}.replacement"
  printf retained > "${target}.replacement/sentinel"
  ln -s "${target}.replacement" "$target"
  exit 0
fi
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(shimDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err = candidateRoot.Prune(featureLock, globalLock)
	if err == nil || !strings.Contains(err.Error(), "candidate worktree entry changed unexpectedly before cleanup") {
		t.Fatalf("Prune err=%v, want refusal to remove replaced residue", err)
	}
	if !candidateRoot.closed {
		t.Fatal("candidate root should be closed after residue cleanup failure")
	}
	for _, root := range []*heldRoot{candidateRoot.repo, candidateRoot.parent, candidateRoot.worktree} {
		if !root.closed || root.root != nil {
			t.Fatal("prune failure left a rooted handle open")
		}
	}
	if body, err := os.ReadFile(filepath.Join(candidateRoot.worktreePath+".replacement", "sentinel")); err != nil || string(body) != "retained" {
		t.Fatalf("prune removed foreign replacement data: %q %v", body, err)
	}
	assertLockHeld(t, featurePath)
	assertLockHeld(t, globalPath)

	releaseCandidateLocks(t, featureLock, globalLock)
	featureLock, globalLock = nil, nil
	assertLockReacquireAfterRelease(t, featurePath)
	assertLockReacquireAfterRelease(t, globalPath)
}

func TestCandidateRootZeroAndClosedState(t *testing.T) {
	var zero CandidateRoot
	if _, err := zero.ReadFile("tracked.txt"); err == nil {
		t.Fatal("zero CandidateRoot read unexpectedly succeeded")
	}
	if err := zero.Prune(nil, nil); err == nil {
		t.Fatal("zero CandidateRoot prune unexpectedly succeeded")
	}
	if err := zero.Close(); err == nil {
		t.Fatal("zero CandidateRoot close unexpectedly succeeded")
	}

	root, head := initCandidateRepo(t)
	featureLock, globalLock := acquireCandidateLocks(t, root, "demo-feature")
	defer releaseCandidateLocks(t, featureLock, globalLock)
	candidateRoot, err := CreateCandidateRoot(root, "demo-feature", "rc_abcdef012348", head, featureLock, globalLock)
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
printf '%%s\037%%s\037%%s\037%%s\037%%s\037%%s\037%%s\037%%s\037%%s\037%%s\037%%s\037%%s\037%%s\n' \
  "${GIT_NO_LAZY_FETCH-}" "${LC_ALL-}" "${GIT_NO_REPLACE_OBJECTS-}" \
  "${GIT_DIR-}" "${GIT_WORK_TREE-}" "${GIT_COMMON_DIR-}" "${GIT_INDEX_FILE-}" \
  "${GIT_CONFIG_GLOBAL-}" "${GIT_CONFIG_SYSTEM-}" "${GIT_CONFIG_NOSYSTEM-}" "${GIT_REPLACE_REF_BASE-}" "${GIT_CONFIG_PARAMETERS-}" "$*" >> %q
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
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(foreignRoot, ".gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(foreignRoot, "system.gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_PARAMETERS", "'core.autocrlf'='false'")
	t.Setenv("GIT_REPLACE_REF_BASE", "refs/replace-foreign/")
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

	featureLock, globalLock := acquireCandidateLocks(t, targetRoot, "demo-feature")
	candidateRoot, err := CreateCandidateRoot(targetRoot, "demo-feature", "rc_fedcba987654", targetHead, featureLock, globalLock)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}
	if err := candidateRoot.Prune(featureLock, globalLock); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	releaseCandidateLocks(t, featureLock, globalLock)

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
		if len(fields) != 13 {
			t.Fatalf("unexpected env log row %q", line)
		}
		if fields[0] != "1" || fields[1] != "C" || fields[2] != "1" {
			t.Fatalf("candidate git call lost mandatory env envelope: %q", line)
		}
		for index, value := range []struct{ name, want string }{
			{"GIT_DIR", ""}, {"GIT_WORK_TREE", ""}, {"GIT_COMMON_DIR", ""}, {"GIT_INDEX_FILE", ""},
			{"GIT_CONFIG_GLOBAL", os.DevNull}, {"GIT_CONFIG_SYSTEM", os.DevNull},
			{"GIT_CONFIG_NOSYSTEM", "1"}, {"GIT_REPLACE_REF_BASE", ""},
			{"GIT_CONFIG_PARAMETERS", ""},
		} {
			if fields[3+index] != value.want {
				t.Fatalf("candidate git call has unsafe %s: %q", value.name, line)
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
	featureLock, globalLock := acquireCandidateLocks(t, root, "demo-feature")
	defer releaseCandidateLocks(t, featureLock, globalLock)

	candidateRoot, err := CreateCandidateRoot(root, "demo-feature", "rc_abcdef012349", head, featureLock, globalLock)
	if err != nil {
		t.Fatalf("CreateCandidateRoot: %v", err)
	}
	defer func() {
		if !candidateRoot.closed {
			_ = candidateRoot.Prune(featureLock, globalLock)
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
