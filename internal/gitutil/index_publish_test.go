// Durable-publish and owned-lock-lifetime tests (GH #7 rev-7 F1/F2).

package gitutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resetPublishHooks clears every failure seam after a test.
func resetPublishHooks(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		publishHookWrite = nil
		publishHookSync = nil
		publishHookClose = nil
		publishHookChmod = nil
		publishHookRename = nil
		publishHookDirSync = nil
	})
}

// stagedTransaction returns a repo with a committed baseline, a
// transaction whose temp index differs from the live one, and the live
// index bytes captured before publication.
func stagedTransaction(t *testing.T) (root string, tx *IndexTransaction, liveBefore []byte) {
	t.Helper()
	root = nestedWTRepo(t)
	if err := os.WriteFile(filepath.Join(root, "operator.txt"), []byte("operator\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxGit(t, root, "add", "operator.txt")

	var err error
	tx, err = BeginIndexTransaction(root)
	if err != nil {
		t.Fatalf("BeginIndexTransaction: %v", err)
	}
	t.Cleanup(func() { _ = tx.Close() })

	if err := os.WriteFile(filepath.Join(root, "landed.txt"), []byte("landed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxGitEnv(t, root, tx.Env(), "add", "landed.txt")

	liveBefore, err = os.ReadFile(tx.LivePath)
	if err != nil {
		t.Fatal(err)
	}
	return root, tx, liveBefore
}

// F1: a failure at ANY step after the lock is created must still remove
// the lock this transaction owns, and must never truncate the live
// index — the reader sees the complete old file or the complete new one.
func TestPublishFailureSeamsCleanOwnedLockAndNeverTruncate(t *testing.T) {
	seams := []struct {
		name string
		set  func(fail func(string) error)
	}{
		{"write", func(f func(string) error) { publishHookWrite = f }},
		{"fsync", func(f func(string) error) { publishHookSync = f }},
		{"close", func(f func(string) error) { publishHookClose = f }},
		{"chmod", func(f func(string) error) { publishHookChmod = f }},
		{"rename", func(f func(string) error) { publishHookRename = f }},
		{"dir fsync", func(f func(string) error) { publishHookDirSync = f }},
	}
	for _, seam := range seams {
		t.Run(seam.name, func(t *testing.T) {
			resetPublishHooks(t)
			root, tx, liveBefore := stagedTransaction(t)
			tempBytes, err := os.ReadFile(tx.TempPath)
			if err != nil {
				t.Fatal(err)
			}

			if err := tx.LockLive(); err != nil {
				t.Fatalf("LockLive: %v", err)
			}
			seam.set(func(string) error { return fmt.Errorf("injected %s failure", seam.name) })

			perr := tx.PublishLocked()
			if seam.name == "dir fsync" {
				// The rename already happened, so publication is
				// materially complete; the error is still surfaced.
				if perr == nil {
					t.Fatal("a directory-fsync failure must be reported")
				}
			} else if perr == nil {
				t.Fatalf("the %s seam must produce an error", seam.name)
			}

			// No owned lock may survive, whatever failed.
			if _, err := os.Stat(tx.LockPath()); !os.IsNotExist(err) {
				t.Errorf("owned lock survived a %s failure: %v", seam.name, err)
			}
			// The live index is complete: either the old bytes or the
			// new ones, never a partial write.
			nowLive, err := os.ReadFile(tx.LivePath)
			if err != nil {
				t.Fatalf("the live index is unreadable after a %s failure: %v", seam.name, err)
			}
			if string(nowLive) != string(liveBefore) && string(nowLive) != string(tempBytes) {
				t.Errorf("live index is neither the complete old nor the complete new file after a %s failure", seam.name)
			}
			// Git can still read it.
			idxGit(t, root, "status", "--porcelain")
			// No publish temp residue in the index directory.
			entries, err := os.ReadDir(filepath.Dir(tx.LivePath))
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), ".tpatch-publish-") {
					t.Errorf("publish temp residue after a %s failure: %s", seam.name, e.Name())
				}
			}
			if err := tx.Close(); err != nil {
				t.Errorf("Close after a %s failure: %v", seam.name, err)
			}
		})
	}
}

// Ownership must survive the descriptor being closed: the lock is a
// mutex sentinel, never the data file.
func TestPublishDoesNotConsumeTheLockAsData(t *testing.T) {
	resetPublishHooks(t)
	_, tx, _ := stagedTransaction(t)
	if err := tx.LockLive(); err != nil {
		t.Fatalf("LockLive: %v", err)
	}
	// The lock carries our sentinel, not index bytes.
	nonce, ours, err := LockNonceAt(tx.LockPath())
	if err != nil {
		t.Fatal(err)
	}
	if !ours || nonce != tx.LockNonce {
		t.Fatalf("lock sentinel = (%q, %v), want our nonce %q", nonce, ours, tx.LockNonce)
	}
	tempBytes, err := os.ReadFile(tx.TempPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.PublishLocked(); err != nil {
		t.Fatalf("PublishLocked: %v", err)
	}
	live, err := os.ReadFile(tx.LivePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(live) != string(tempBytes) {
		t.Error("published index does not match the temp index")
	}
	if _, err := os.Stat(tx.LockPath()); !os.IsNotExist(err) {
		t.Errorf("lock survived a successful publish: %v", err)
	}
}

// A foreign lock is never removed, even by Close.
func TestOwnedLockCleanupNeverTouchesAForeignLock(t *testing.T) {
	resetPublishHooks(t)
	_, tx, _ := stagedTransaction(t)
	lockPath := tx.LockPath()
	if err := os.WriteFile(lockPath, []byte("foreign\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := tx.LockLive(); err == nil {
		t.Fatal("acquiring an existing lock must fail")
	}
	if err := tx.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	body, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("the foreign lock was removed: %v", err)
	}
	if string(body) != "foreign\n" {
		t.Error("the foreign lock was modified")
	}
	nonce, ours, err := LockNonceAt(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if ours || nonce != "" {
		t.Errorf("a foreign lock must not be reported as ours: (%q, %v)", nonce, ours)
	}
	_ = os.Remove(lockPath)
}

// Publishing an absent temp index over an absent live index leaves the
// absent state and still releases the lock.
func TestPublishAbsentIndexReleasesLock(t *testing.T) {
	resetPublishHooks(t)
	dir := t.TempDir()
	idxGit(t, dir, "init", "-q", "-b", "main", ".")
	tx, err := BeginIndexTransaction(dir)
	if err != nil {
		t.Fatalf("BeginIndexTransaction: %v", err)
	}
	defer tx.Close()
	if err := tx.LockLive(); err != nil {
		t.Fatal(err)
	}
	if err := tx.PublishLocked(); err != nil {
		t.Fatalf("PublishLocked: %v", err)
	}
	if _, err := os.Stat(tx.LivePath); !os.IsNotExist(err) {
		t.Errorf("the live index should still be absent: %v", err)
	}
	if _, err := os.Stat(tx.LockPath()); !os.IsNotExist(err) {
		t.Errorf("lock residue: %v", err)
	}
}
