// Tests for the isolated index transaction (GH #7 rev-6).

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

func idxGitEnv(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = env
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %s: %v", args, dir, out, err)
	}
	return string(out)
}

// ─── effective index resolution ─────────────────────────────────────

func TestEffectiveIndexPathMainWorktree(t *testing.T) {
	root := nestedWTRepo(t)
	got, err := EffectiveIndexPath(root)
	if err != nil {
		t.Fatalf("EffectiveIndexPath: %v", err)
	}
	if want := filepath.Join(root, ".git", "index"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A linked worktree keeps its index under the common repository's
// worktrees/ directory.
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

	// A transaction against a linked worktree must seed from, and
	// publish back to, that worktree's own index.
	if err := os.WriteFile(filepath.Join(linked, "hello.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxGit(t, linked, "add", "hello.txt")
	treeBefore := idxGit(t, linked, "write-tree")

	tx, err := BeginIndexTransaction(linked)
	if err != nil {
		t.Fatalf("BeginIndexTransaction: %v", err)
	}
	defer tx.Close()
	if tx.LivePath != got {
		t.Errorf("transaction live path %q != %q", tx.LivePath, got)
	}
	if treeTemp := strings.TrimSpace(idxGitEnv(t, linked, tx.Env(), "write-tree")); treeTemp != strings.TrimSpace(treeBefore) {
		t.Errorf("temp index not seeded from the linked worktree index: %q vs %q", treeTemp, treeBefore)
	}
	if err := os.WriteFile(filepath.Join(linked, "extra.txt"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxGitEnv(t, linked, tx.Env(), "add", "extra.txt")
	// The live index must be untouched while the temp index moved on.
	if got := idxGit(t, linked, "write-tree"); got != treeBefore {
		t.Errorf("live index mutated during temp staging:\nbefore=%s\nafter=%s", treeBefore, got)
	}
	if err := tx.LockLive(); err != nil {
		t.Fatal(err)
	}
	if err := tx.VerifyLiveUnchanged(); err != nil {
		t.Fatal(err)
	}
	if err := tx.PublishLocked(); err != nil {
		t.Fatalf("PublishLocked: %v", err)
	}
	if got := idxGit(t, linked, "write-tree"); got == treeBefore {
		t.Error("publish did not move the live index")
	}
}

// GIT_INDEX_FILE is honoured byte-for-byte, including whitespace, for
// both absolute and relative values.
func TestEffectiveIndexPathRespectsWhitespaceBearingGitIndexFile(t *testing.T) {
	t.Run("absolute with spaces and a tab", func(t *testing.T) {
		root := nestedWTRepo(t)
		dir := t.TempDir()
		redirect := filepath.Join(dir, " odd\tindex name ")
		t.Setenv("GIT_INDEX_FILE", redirect)
		got, err := EffectiveIndexPath(root)
		if err != nil {
			t.Fatalf("EffectiveIndexPath: %v", err)
		}
		if got != redirect {
			t.Fatalf("got %q, want the redirected path %q (byte-for-byte)", got, redirect)
		}

		// A full transaction round-trip through that path.
		tx, err := BeginIndexTransaction(root)
		if err != nil {
			t.Fatalf("BeginIndexTransaction: %v", err)
		}
		defer tx.Close()
		if err := os.WriteFile(filepath.Join(root, "redirected.txt"), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		idxGitEnv(t, root, tx.Env(), "add", "redirected.txt")
		if _, err := os.Stat(redirect); !os.IsNotExist(err) {
			t.Errorf("staging must not have touched the redirected live index yet: %v", err)
		}
		if err := tx.LockLive(); err != nil {
			t.Fatal(err)
		}
		if err := tx.VerifyLiveUnchanged(); err != nil {
			t.Fatal(err)
		}
		if err := tx.PublishLocked(); err != nil {
			t.Fatalf("PublishLocked: %v", err)
		}
		if _, err := os.Stat(redirect); err != nil {
			t.Fatalf("publish did not create the redirected index: %v", err)
		}
		staged, err := StagedPathsEnv(root, nil)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, p := range staged {
			if p == "redirected.txt" {
				found = true
			}
		}
		if !found {
			t.Errorf("published index does not contain the staged path: %q", staged)
		}
		if err := tx.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		if _, err := os.Stat(redirect + ".lock"); !os.IsNotExist(err) {
			t.Errorf("lock residue left behind: %v", err)
		}
	})

	t.Run("relative with a trailing space", func(t *testing.T) {
		root := nestedWTRepo(t)
		rel := "sub dir/idx "
		if err := os.MkdirAll(filepath.Join(root, "sub dir"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GIT_INDEX_FILE", rel)
		got, err := EffectiveIndexPath(root)
		if err != nil {
			t.Fatalf("EffectiveIndexPath: %v", err)
		}
		want := filepath.Join(root, rel)
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
		if !strings.HasSuffix(got, "idx ") {
			t.Errorf("trailing space was trimmed from %q", got)
		}
	})
}

// ─── non-regular index refusal ──────────────────────────────────────

func TestBeginIndexTransactionRefusesSymlinkedIndex(t *testing.T) {
	root := nestedWTRepo(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxGit(t, root, "add", "a.txt")

	realIndex := filepath.Join(root, ".git", "index")
	moved := filepath.Join(t.TempDir(), "real-index")
	if err := os.Rename(realIndex, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(moved, realIndex); err != nil {
		t.Skipf("platform refuses symlinks: %v", err)
	}
	before, err := os.ReadFile(moved)
	if err != nil {
		t.Fatal(err)
	}

	tx, err := BeginIndexTransaction(root)
	if err == nil {
		tx.Close()
		t.Fatal("a symlinked index must be refused")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("refusal should name the symlink: %v", err)
	}
	// The symlink topology must survive untouched.
	info, lerr := os.Lstat(realIndex)
	if lerr != nil {
		t.Fatal(lerr)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("the symlink was replaced despite the refusal")
	}
	after, err := os.ReadFile(moved)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Error("the symlink target was modified despite the refusal")
	}
}

func TestBeginIndexTransactionRefusesNonRegularIndex(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		root := nestedWTRepo(t)
		indexPath := filepath.Join(root, ".git", "index")
		_ = os.Remove(indexPath)
		if err := os.Mkdir(indexPath, 0o755); err != nil {
			t.Fatal(err)
		}
		tx, err := BeginIndexTransaction(root)
		if err == nil {
			tx.Close()
			t.Fatal("a directory index must be refused")
		}
		if !strings.Contains(err.Error(), "not a regular file") {
			t.Errorf("refusal should name the file kind: %v", err)
		}
	})

	t.Run("fifo", func(t *testing.T) {
		root := nestedWTRepo(t)
		indexPath := filepath.Join(root, ".git", "index")
		_ = os.Remove(indexPath)
		if !makeFifoForTest(t, indexPath) {
			t.Skip("platform has no FIFOs")
		}
		tx, err := BeginIndexTransaction(root)
		if err == nil {
			tx.Close()
			t.Fatal("a FIFO index must be refused")
		}
		if !strings.Contains(err.Error(), "not a regular file") {
			t.Errorf("refusal should name the file kind: %v", err)
		}
	})
}

// ─── isolation, divergence and publication ──────────────────────────

// The operator's live index is untouched while land stages, and a
// concurrent `git add` is detected instead of being overwritten.
func TestIndexTransactionDetectsConcurrentLiveMutation(t *testing.T) {
	root := nestedWTRepo(t)
	if err := os.WriteFile(filepath.Join(root, "operator.txt"), []byte("operator\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxGit(t, root, "add", "operator.txt")

	tx, err := BeginIndexTransaction(root)
	if err != nil {
		t.Fatalf("BeginIndexTransaction: %v", err)
	}
	defer tx.Close()

	// Land stages into the temp index.
	if err := os.WriteFile(filepath.Join(root, "landed.txt"), []byte("landed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxGitEnv(t, root, tx.Env(), "add", "landed.txt")

	// Meanwhile the operator stages something of their own.
	if err := os.WriteFile(filepath.Join(root, "concurrent.txt"), []byte("concurrent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxGit(t, root, "add", "concurrent.txt")
	liveAfterOperator, err := os.ReadFile(tx.LivePath)
	if err != nil {
		t.Fatal(err)
	}

	if err := tx.LockLive(); err != nil {
		t.Fatalf("LockLive: %v", err)
	}
	verr := tx.VerifyLiveUnchanged()
	if verr == nil {
		t.Fatal("a concurrent live-index mutation must be detected")
	}
	if !strings.Contains(verr.Error(), "changed while") {
		t.Errorf("divergence message should explain the cause: %v", verr)
	}
	if err := tx.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// The operator's index survives byte-for-byte, and land's staging
	// never reached it.
	nowLive, err := os.ReadFile(tx.LivePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(nowLive) != string(liveAfterOperator) {
		t.Error("the operator's index was modified by the aborted transaction")
	}
	staged, err := StagedPaths(root)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(staged, ",")
	if !strings.Contains(joined, "concurrent.txt") || !strings.Contains(joined, "operator.txt") {
		t.Errorf("operator's staged content lost: %q", staged)
	}
	if strings.Contains(joined, "landed.txt") {
		t.Errorf("land's staging leaked into the live index: %q", staged)
	}
	if _, err := os.Stat(tx.LivePath + ".lock"); !os.IsNotExist(err) {
		t.Errorf("lock residue left behind: %v", err)
	}
	if _, err := os.Stat(tx.TempPath); !os.IsNotExist(err) {
		t.Errorf("temp index residue left behind: %v", err)
	}
}

// Operator-staged content, including intent-to-add entries, seeds the
// temp index so land's view starts from the operator's real state.
func TestIndexTransactionSeedsFromOperatorState(t *testing.T) {
	root := nestedWTRepo(t)
	if err := os.WriteFile(filepath.Join(root, "operator.txt"), []byte("operator\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "intent.txt"), []byte("intent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxGit(t, root, "add", "operator.txt")
	idxGit(t, root, "add", "--intent-to-add", "intent.txt")

	tx, err := BeginIndexTransaction(root)
	if err != nil {
		t.Fatalf("BeginIndexTransaction: %v", err)
	}
	defer tx.Close()

	staged, err := StagedPathsEnv(root, tx.Env())
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(staged, ",")
	if !strings.Contains(joined, "operator.txt") {
		t.Errorf("temp index missing seeded entry %q: %q", "operator.txt", staged)
	}
	// `git diff --cached` deliberately ignores intent-to-add entries,
	// so the seeded intent-to-add is verified through the index listing
	// instead — it must be present in the temp index.
	cached := idxGitEnv(t, root, tx.Env(), "ls-files", "--cached")
	for _, want := range []string{"operator.txt", "intent.txt"} {
		if !strings.Contains(cached, want) {
			t.Errorf("temp index missing seeded entry %q:\n%s", want, cached)
		}
	}
	liveBytes, err := os.ReadFile(tx.LivePath)
	if err != nil {
		t.Fatal(err)
	}
	tempBytes, err := os.ReadFile(tx.TempPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(liveBytes) != string(tempBytes) {
		t.Error("temp index is not a byte-identical seed of the live index")
	}
}

// An absent live index is a valid start state: the temp index is
// absent too, and publishing creates the live index.
func TestIndexTransactionAbsentIndexLifecycle(t *testing.T) {
	dir := t.TempDir()
	idxGit(t, dir, "init", "-q", "-b", "main", ".")
	idxGit(t, dir, "config", "user.email", "t@example.com")
	idxGit(t, dir, "config", "user.name", "T")

	tx, err := BeginIndexTransaction(dir)
	if err != nil {
		t.Fatalf("BeginIndexTransaction: %v", err)
	}
	defer tx.Close()
	if _, err := os.Stat(tx.TempPath); !os.IsNotExist(err) {
		t.Errorf("temp index should be absent when the live index is: %v", err)
	}
	if _, err := os.Stat(tx.LivePath); !os.IsNotExist(err) {
		t.Fatalf("fixture assumption broken: live index exists")
	}

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxGitEnv(t, dir, tx.Env(), "add", "a.txt")
	if _, err := os.Stat(tx.LivePath); !os.IsNotExist(err) {
		t.Error("staging created a live index; it must stay isolated")
	}
	if err := tx.LockLive(); err != nil {
		t.Fatal(err)
	}
	if err := tx.VerifyLiveUnchanged(); err != nil {
		t.Fatal(err)
	}
	if err := tx.PublishLocked(); err != nil {
		t.Fatalf("PublishLocked: %v", err)
	}
	if _, err := os.Stat(tx.LivePath); err != nil {
		t.Fatalf("publish did not create the live index: %v", err)
	}
	if err := tx.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// A pre-existing lock is somebody else's; it must not be taken or
// removed.
func TestIndexTransactionRefusesAndPreservesForeignLock(t *testing.T) {
	root := nestedWTRepo(t)
	tx, err := BeginIndexTransaction(root)
	if err != nil {
		t.Fatalf("BeginIndexTransaction: %v", err)
	}
	lockPath := tx.LivePath + ".lock"
	if err := os.WriteFile(lockPath, []byte("someone else\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = tx.LockLive()
	if err == nil {
		t.Fatal("an existing lock must not be acquired")
	}
	if !strings.Contains(err.Error(), "another Git process") {
		t.Errorf("message should explain the contention: %v", err)
	}
	if cerr := tx.Close(); cerr != nil {
		t.Errorf("Close: %v", cerr)
	}
	body, rerr := os.ReadFile(lockPath)
	if rerr != nil {
		t.Fatalf("the foreign lock was removed: %v", rerr)
	}
	if string(body) != "someone else\n" {
		t.Error("the foreign lock was overwritten")
	}
	_ = os.Remove(lockPath)
}

// Close is idempotent and reports cleanup problems rather than
// swallowing them.
func TestIndexTransactionCloseIsIdempotent(t *testing.T) {
	root := nestedWTRepo(t)
	tx, err := BeginIndexTransaction(root)
	if err != nil {
		t.Fatalf("BeginIndexTransaction: %v", err)
	}
	if err := tx.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := tx.Close(); err != nil {
		t.Fatalf("second Close should be a no-op: %v", err)
	}
	if _, err := os.Stat(tx.TempPath); !os.IsNotExist(err) {
		t.Errorf("temp index residue: %v", err)
	}
}

// PublishLocked without the lock is a programming error, not a silent
// no-op.
func TestPublishLockedRequiresTheLock(t *testing.T) {
	root := nestedWTRepo(t)
	tx, err := BeginIndexTransaction(root)
	if err != nil {
		t.Fatalf("BeginIndexTransaction: %v", err)
	}
	defer tx.Close()
	if err := tx.PublishLocked(); err == nil {
		t.Fatal("publishing without the lock must fail")
	}
}

// ─── staged-path inspection and audit ───────────────────────────────

func TestStagedPathsEnvByteExact(t *testing.T) {
	root := nestedWTRepo(t)
	tx, err := BeginIndexTransaction(root)
	if err != nil {
		t.Fatalf("BeginIndexTransaction: %v", err)
	}
	defer tx.Close()

	names := []string{"sp ace.txt", "tab\tin.txt", "new\nline.txt"}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(root, n), []byte("x\n"), 0o644); err != nil {
			t.Fatalf("write %q: %v", n, err)
		}
		idxGitEnv(t, root, tx.Env(), "--literal-pathspecs", "add", "--", n)
	}
	got, err := StagedPathsEnv(root, tx.Env())
	if err != nil {
		t.Fatalf("StagedPathsEnv: %v", err)
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
	// The live index saw none of it.
	live, err := StagedPaths(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 0 {
		t.Errorf("live index was mutated: %q", live)
	}
}

func TestAuditStagedPathsForNestedWorktreesEnv(t *testing.T) {
	root := nestedWTRepo(t)
	tx, err := BeginIndexTransaction(root)
	if err != nil {
		t.Fatalf("BeginIndexTransaction: %v", err)
	}
	defer tx.Close()

	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxGitEnv(t, root, tx.Env(), "add", "ok.txt")
	clean, err := AuditStagedPathsForNestedWorktreesEnv(root, tx.Env())
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(clean) != 0 {
		t.Errorf("expected no findings, got %q", clean)
	}

	rel := ".claude/worktrees/agent review"
	addWorktree(t, root, filepath.Join(root, filepath.FromSlash(rel)), "agent-review")
	idxGitEnv(t, root, tx.Env(), "-c", "advice.addEmbeddedRepo=false",
		"--literal-pathspecs", "add", "--", rel)

	contaminated, err := AuditStagedPathsForNestedWorktreesEnv(root, tx.Env())
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(contaminated) != 1 || contaminated[0] != rel {
		t.Fatalf("audit = %q, want [%q]", contaminated, rel)
	}
}

func TestAuditStagedPathsFailsClosedOnDiscoveryFailure(t *testing.T) {
	notARepo := t.TempDir()
	_, err := AuditStagedPathsForNestedWorktreesEnv(notARepo, nil)
	if err == nil {
		t.Fatal("expected a discovery failure outside a Git repository")
	}
	if !strings.Contains(err.Error(), "Refusing to capture") {
		t.Errorf("failure should carry the fail-closed guidance: %v", err)
	}
}
