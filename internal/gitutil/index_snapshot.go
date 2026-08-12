// Isolated index transactions for `tpatch land` (GH #7 rev-6).
//
// Earlier revisions staged into the operator's LIVE index and rolled it
// back on failure. That is unsafe in two ways a reviewer correctly
// caught: a blind restore discards any `git add` the operator ran
// concurrently, and a "successful" land can silently include content
// the operator staged mid-flight.
//
// So land no longer mutates the live index while it works. It seeds a
// private temporary index byte-identically from the live one, performs
// every `git add`, every staged-path audit and the commit itself with
// `GIT_INDEX_FILE` pointing at that temp file, and only publishes the
// result while holding Git's own `<index>.lock`. Before publishing it
// re-compares the live index against the snapshot taken at the start;
// any divergence means a concurrent Git operation and land refuses
// without overwriting anything.
//
// Honest scope of the guarantee:
//
//   - This serializes INDEX writes against other Git processes, because
//     `<index>.lock` is the same lock `git add` and `git commit` take.
//   - It does NOT protect against arbitrary concurrent mutation of refs
//     or of the working tree. A concurrent `git checkout`, `git reset
//     --hard` or a direct ref update is outside what an index lock can
//     express, and land does not claim otherwise.

package gitutil

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IndexFileState is a byte-for-byte capture of an index file, including
// the "there is no index" case.
type IndexFileState struct {
	Path    string
	Existed bool
	Data    []byte
	Mode    fs.FileMode
}

// Matches reports whether two captures describe the same content and
// mode, or agree that the file is absent.
func (s *IndexFileState) Matches(other *IndexFileState) bool {
	if s == nil || other == nil {
		return s == other
	}
	if s.Existed != other.Existed {
		return false
	}
	if !s.Existed {
		return true
	}
	return s.Mode == other.Mode && bytes.Equal(s.Data, other.Data)
}

// EffectiveIndexPath resolves the index file Git will actually use for
// repoRoot, as an absolute path, WITHOUT altering any byte of the name.
//
// Precedence mirrors Git's own:
//
//  1. `GIT_INDEX_FILE`, taken verbatim — leading and trailing spaces
//     and tabs are legitimate path bytes and are preserved. A relative
//     value is resolved against repoRoot, the working directory every
//     git subprocess tpatch spawns runs in.
//  2. Otherwise `git rev-parse --git-path index`. Only the single
//     protocol line terminator is stripped; any further newline means
//     the answer is ambiguous (a path containing a newline is
//     indistinguishable from two answers) and is refused.
func EffectiveIndexPath(repoRoot string) (string, error) {
	if env, ok := os.LookupEnv("GIT_INDEX_FILE"); ok && env != "" {
		if !filepath.IsAbs(env) {
			return filepath.Join(repoRoot, env), nil
		}
		return env, nil
	}
	out, err := runGit(repoRoot, "rev-parse", "--git-path", "index")
	if err != nil {
		return "", fmt.Errorf("resolve effective git index: %w", err)
	}
	p := strings.TrimSuffix(out, "\n")
	p = strings.TrimSuffix(p, "\r")
	if p == "" {
		return "", fmt.Errorf("resolve effective git index: git returned an empty path")
	}
	if strings.Contains(p, "\n") {
		return "", fmt.Errorf("resolve effective git index: ambiguous multi-line output %q; refusing to guess which line names the index", truncateForError(out))
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(repoRoot, p)
	}
	return p, nil
}

// requireRegularIndexFile refuses a symlinked or otherwise non-regular
// index before anything is staged.
//
// Lstat, never Stat: following a symlink and then publishing over it
// would silently replace the link with a regular file and write Git
// state wherever the link happened to point. An absent index is fine —
// that is simply the never-staged state.
func requireRegularIndexFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect effective git index %q: %w", path, err)
	}
	mode := info.Mode()
	if mode&fs.ModeSymlink != 0 {
		return fmt.Errorf(
			"effective git index %q is a symlink.\n"+
				"tpatch refuses to stage through it: publishing an index would replace the link with a regular file and write Git state to wherever it points.\n"+
				"Replace the symlink with a real index (or point GIT_INDEX_FILE at the real path) and retry",
			path)
	}
	if !mode.IsRegular() {
		return fmt.Errorf(
			"effective git index %q is not a regular file (mode %s).\n"+
				"tpatch refuses to stage through it. Point GIT_INDEX_FILE at a regular file and retry",
			path, mode)
	}
	return nil
}

// CaptureIndexFileState reads an index file's existence, exact bytes
// and mode. The file is only read, never touched.
func CaptureIndexFileState(path string) (*IndexFileState, error) {
	st := &IndexFileState{Path: path}
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return nil, fmt.Errorf("stat index %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("index %q is not a regular file (mode %s)", path, info.Mode())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read index %q: %w", path, err)
	}
	st.Existed = true
	st.Data = data
	st.Mode = info.Mode().Perm()
	return st, nil
}

// Failure seams. Production leaves every hook nil; tests set them to
// inject a failure at one exact step of the durable publish and then
// assert that the live index is either the complete old file or the
// complete new one, never a truncation, and that no lock this
// transaction owns is left behind.
//
// Each hook receives the TARGET path, so a test can fail only the live
// index publication and leave unrelated durable writes (such as seeding
// the alternate index) alone.
var (
	publishHookWrite   func(path string) error
	publishHookSync    func(path string) error
	publishHookClose   func(path string) error
	publishHookChmod   func(path string) error
	publishHookRename  func(path string) error
	publishHookDirSync func(path string) error
)

// SetPublishRenameHookForTest injects a failure at the rename step of
// the durable publish. Tests in other packages use it to exercise the
// "publication failed" contracts; production always leaves it nil.
func SetPublishRenameHookForTest(f func(path string) error) { publishHookRename = f }

// IndexTransaction isolates a sequence of index mutations in a private
// temporary index and publishes the result durably, under Git's own
// index lock.
//
// Lock ownership is tracked SEPARATELY from the open descriptor
// (GH #7 rev-7 F1). Closing the descriptor is not the same as giving up
// responsibility for the file: a later chmod/rename failure must still
// remove the `<index>.lock` this transaction created, or the repository
// is left wedged behind a stale lock nobody will clean up. A lock this
// transaction did NOT create is never touched.
type IndexTransaction struct {
	RepoRoot string
	// LivePath is the operator's effective index.
	LivePath string
	// TempPath is the private index every staged operation writes to.
	TempPath string
	// LockNonce identifies the lock this transaction created, so crash
	// recovery can distinguish our stale lock from a foreign one.
	LockNonce string

	live *IndexFileState
	// liveDir is the symlink-resolved directory holding the live index.
	// Durable publication creates its temp there so the final rename is
	// same-directory and same-filesystem, and therefore atomic, even
	// when a parent component is a symlink.
	liveDir  string
	tempDir  string
	lockPath string
	lockFile *os.File
	// lockOwned is the authority on cleanup responsibility. It stays
	// true after lockFile is closed and is only cleared once the lock
	// file has actually been removed.
	lockOwned bool
	closed    bool
}

// BeginIndexTransaction validates the effective index, snapshots it and
// seeds a private temporary index with identical bytes in a throwaway
// directory this transaction owns.
func BeginIndexTransaction(repoRoot string) (*IndexTransaction, error) {
	tempDir, err := os.MkdirTemp("", "tpatch-land-index-*")
	if err != nil {
		return nil, fmt.Errorf("create temp index directory: %w", err)
	}
	tx, err := BeginIndexTransactionAt(repoRoot, filepath.Join(tempDir, "index"))
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}
	tx.tempDir = tempDir
	return tx, nil
}

// BeginIndexTransactionAt is BeginIndexTransaction with the alternate
// index at a caller-chosen path, which the caller owns.
//
// GH #7 rev-8: `land` points this at the DURABLE retained index inside
// its journal directory, so the very file that staging, audits, hooks
// and `git commit` mutate is the file crash recovery later reads. A
// separate ephemeral copy could not capture a hook's edits, which is
// precisely the evidence recovery needs.
//
// When the repository has never staged anything the live index is
// absent; the alternate index is left absent too, so the first
// `git add` creates it exactly as Git would.
func BeginIndexTransactionAt(repoRoot, altIndexPath string) (*IndexTransaction, error) {
	livePath, err := EffectiveIndexPath(repoRoot)
	if err != nil {
		return nil, err
	}
	if err := requireRegularIndexFile(livePath); err != nil {
		return nil, err
	}
	live, err := CaptureIndexFileState(livePath)
	if err != nil {
		return nil, err
	}
	liveDir := filepath.Dir(livePath)
	if resolved, rerr := filepath.EvalSymlinks(liveDir); rerr == nil {
		liveDir = resolved
	}
	nonce, err := randomNonce()
	if err != nil {
		return nil, err
	}
	tx := &IndexTransaction{
		RepoRoot:  repoRoot,
		LivePath:  livePath,
		TempPath:  altIndexPath,
		LockNonce: nonce,
		live:      live,
		liveDir:   liveDir,
		lockPath:  livePath + ".lock",
	}
	if err := os.MkdirAll(filepath.Dir(altIndexPath), 0o700); err != nil {
		return nil, fmt.Errorf("create alternate index directory: %w", err)
	}
	if live.Existed {
		mode := live.Mode
		if mode == 0 {
			mode = 0o644
		}
		if err := DurableWriteFile(filepath.Dir(altIndexPath), altIndexPath, live.Data, mode); err != nil {
			return nil, fmt.Errorf("seed alternate index: %w", err)
		}
	} else if rmErr := os.Remove(altIndexPath); rmErr != nil && !os.IsNotExist(rmErr) {
		return nil, fmt.Errorf("clear a stale alternate index: %w", rmErr)
	}
	return tx, nil
}

// SyncAlternateIndex fsyncs the alternate index so its current bytes —
// including anything a hook wrote — survive a crash.
func (tx *IndexTransaction) SyncAlternateIndex() error {
	f, err := os.Open(tx.TempPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open alternate index for fsync: %w", err)
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return fmt.Errorf("fsync alternate index: %w", err)
	}
	return syncDir(filepath.Dir(tx.TempPath))
}

// AdoptLock takes ownership of an already-created lock file, used by
// crash recovery after it has validated that the stale lock is ours.
func (tx *IndexTransaction) AdoptLock() { tx.lockOwned = true }

// LiveState exposes the start-of-transaction snapshot for journalling.
func (tx *IndexTransaction) LiveState() *IndexFileState { return tx.live }

// LockPath is the `<index>.lock` path this transaction contends for.
func (tx *IndexTransaction) LockPath() string { return tx.lockPath }

// Env returns the environment every git subprocess in this transaction
// must run with, so staging, audits, the commit and its hooks all see
// the private index instead of the operator's.
func (tx *IndexTransaction) Env() []string {
	return append(os.Environ(), "GIT_INDEX_FILE="+tx.TempPath)
}

// LockLive takes Git's own `<index>.lock` with O_EXCL — the same lock
// `git add` and `git commit` contend for — and records this
// transaction's nonce in it so a crash leaves identifiable evidence.
//
// The lock is a MUTEX SENTINEL only. It is never renamed onto the index
// as the data file; publication uses a separate temp (see
// PublishLocked), so a publish failure can never destroy the live index
// by consuming the lock.
func (tx *IndexTransaction) LockLive() error {
	if tx.lockOwned {
		return fmt.Errorf("index lock already held")
	}
	if err := os.MkdirAll(filepath.Dir(tx.lockPath), 0o755); err != nil {
		return fmt.Errorf("prepare index lock directory: %w", err)
	}
	f, err := os.OpenFile(tx.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf(
				"cannot acquire the Git index lock %q: another Git process is writing the index.\n"+
					"Wait for it to finish (or remove the stale lock yourself if you are certain none is running) and retry",
				tx.lockPath)
		}
		return fmt.Errorf("acquire index lock %q: %w", tx.lockPath, err)
	}
	tx.lockFile = f
	tx.lockOwned = true
	if _, werr := f.WriteString(lockSentinelPrefix + tx.LockNonce + "\n"); werr != nil {
		return fmt.Errorf("write index lock sentinel: %w", werr)
	}
	if serr := f.Sync(); serr != nil {
		return fmt.Errorf("sync index lock sentinel: %w", serr)
	}
	return nil
}

// lockSentinelPrefix marks a lock file this tool created.
const lockSentinelPrefix = "tpatch-land-lock "

// FileIno returns a file's inode number when the platform exposes one.
// Used to strengthen stale-lock identification beyond the nonce alone.
// Platforms without inodes report ok=false and the check is skipped.
func FileIno(path string) (uint64, bool) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, false
	}
	return fileInoFromInfo(info)
}

// LockNonceAt reads the nonce out of a lock file, reporting whether the
// lock was created by tpatch at all. A lock with any other content is
// foreign and must never be removed.
func LockNonceAt(lockPath string) (nonce string, ours bool, err error) {
	body, err := os.ReadFile(lockPath)
	if err != nil {
		return "", false, err
	}
	line := strings.TrimSuffix(string(body), "\n")
	if !strings.HasPrefix(line, lockSentinelPrefix) {
		return "", false, nil
	}
	return strings.TrimPrefix(line, lockSentinelPrefix), true, nil
}

// VerifyLiveUnchanged compares the live index against the Begin
// snapshot. Call it while the lock is held.
//
// A divergence means the operator ran a concurrent Git operation while
// land was staging. Publishing over it would discard their work, so the
// caller must refuse.
func (tx *IndexTransaction) VerifyLiveUnchanged() error {
	current, err := CaptureIndexFileState(tx.LivePath)
	if err != nil {
		return err
	}
	if tx.live.Matches(current) {
		return nil
	}
	return fmt.Errorf(
		"the Git index changed while `tpatch land` was staging (a concurrent `git add`, `git reset` or similar).\n" +
			"Nothing was committed and your index was left exactly as you last set it.\n" +
			"Re-run `tpatch land` once the other Git operation has finished")
}

// PublishLocked durably writes the temporary index over the live one
// while the owned lock is held.
//
// The lock is NOT the data file (GH #7 rev-7 F2). Bytes go to a
// separate O_EXCL temp in the live index's own (symlink-resolved)
// directory, are fsynced, chmod'd to the intended mode, renamed onto
// the live index, and the parent directory is fsynced. A reader
// therefore always sees the complete old index or the complete new
// one — never a truncation — and a crash at any step leaves a
// recoverable state.
//
// The owned lock is released only after the publish completes (or its
// cleanup runs), so no stale lock survives a failure.
func (tx *IndexTransaction) PublishLocked() error {
	if !tx.lockOwned {
		return fmt.Errorf("publish index: lock not held")
	}
	temp, err := CaptureIndexFileState(tx.TempPath)
	if err != nil {
		return tx.joinRelease(err)
	}
	if !temp.Existed {
		// Nothing was ever staged and the live index was absent:
		// publishing means leaving that state in place.
		return tx.releaseOwnedLock()
	}
	mode := tx.live.Mode
	if mode == 0 {
		mode = temp.Mode
	}
	if mode == 0 {
		mode = 0o644
	}
	if err := DurableWriteFile(tx.liveDir, tx.LivePath, temp.Data, mode); err != nil {
		return tx.joinRelease(err)
	}
	return tx.releaseOwnedLock()
}

// DurableWriteFile publishes `data` at `target` via an O_EXCL temp in
// `dir`, with file and directory fsyncs, so the result survives a crash
// and is never observed partially written. Exported so the land journal
// layer writes its evidence with identical durability.
func DurableWriteFile(dir, target string, data []byte, mode fs.FileMode) error {
	tmp, err := os.CreateTemp(dir, ".tpatch-publish-*")
	if err != nil {
		return fmt.Errorf("publish index: create temp in %q: %w", dir, err)
	}
	tmpPath := tmp.Name()
	cleanup := func(primary error) error {
		_ = tmp.Close()
		if rerr := os.Remove(tmpPath); rerr != nil && !os.IsNotExist(rerr) {
			return fmt.Errorf("%w; additionally removing the publish temp %q failed: %v", primary, tmpPath, rerr)
		}
		return primary
	}

	if publishHookWrite != nil {
		if herr := publishHookWrite(target); herr != nil {
			return cleanup(fmt.Errorf("publish index: write: %w", herr))
		}
	}
	if _, err := tmp.Write(data); err != nil {
		return cleanup(fmt.Errorf("publish index: write: %w", err))
	}
	if publishHookSync != nil {
		if herr := publishHookSync(target); herr != nil {
			return cleanup(fmt.Errorf("publish index: fsync: %w", herr))
		}
	}
	if err := tmp.Sync(); err != nil {
		return cleanup(fmt.Errorf("publish index: fsync: %w", err))
	}
	if publishHookClose != nil {
		if herr := publishHookClose(target); herr != nil {
			return cleanup(fmt.Errorf("publish index: close: %w", herr))
		}
	}
	if err := tmp.Close(); err != nil {
		if rerr := os.Remove(tmpPath); rerr != nil && !os.IsNotExist(rerr) {
			return fmt.Errorf("publish index: close: %v; additionally removing %q failed: %v", err, tmpPath, rerr)
		}
		return fmt.Errorf("publish index: close: %w", err)
	}
	removeTmp := func(primary error) error {
		if rerr := os.Remove(tmpPath); rerr != nil && !os.IsNotExist(rerr) {
			return fmt.Errorf("%w; additionally removing the publish temp %q failed: %v", primary, tmpPath, rerr)
		}
		return primary
	}
	if publishHookChmod != nil {
		if herr := publishHookChmod(target); herr != nil {
			return removeTmp(fmt.Errorf("publish index: chmod: %w", herr))
		}
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return removeTmp(fmt.Errorf("publish index: chmod: %w", err))
	}
	if publishHookRename != nil {
		if herr := publishHookRename(target); herr != nil {
			return removeTmp(fmt.Errorf("publish index: rename onto %q: %w", target, herr))
		}
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return removeTmp(fmt.Errorf("publish index: rename onto %q: %w", target, err))
	}
	if publishHookDirSync != nil {
		if herr := publishHookDirSync(target); herr != nil {
			return fmt.Errorf("publish index: fsync directory %q: %w", dir, herr)
		}
	}
	return syncDir(dir)
}

// syncDir fsyncs a directory so a rename inside it is durable.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open directory %q for fsync: %w", dir, err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("fsync directory %q: %w", dir, err)
	}
	return nil
}

// joinRelease releases the owned lock and folds any release failure
// into the primary error, so cleanup problems are never silent.
func (tx *IndexTransaction) joinRelease(primary error) error {
	if rerr := tx.releaseOwnedLock(); rerr != nil {
		return fmt.Errorf("%w; additionally: %v", primary, rerr)
	}
	return primary
}

// releaseOwnedLock closes the descriptor if it is still open and
// removes the lock file — but only when this transaction created it.
// Ownership survives a closed descriptor, which is the whole point.
func (tx *IndexTransaction) releaseOwnedLock() error {
	if tx.lockFile != nil {
		_ = tx.lockFile.Close()
		tx.lockFile = nil
	}
	if !tx.lockOwned {
		return nil
	}
	if err := os.Remove(tx.lockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("release index lock %q: %w", tx.lockPath, err)
	}
	tx.lockOwned = false
	if err := syncDir(tx.liveDir); err != nil {
		return err
	}
	return nil
}

// Close releases any lock this transaction still owns and removes the
// private temp index. Safe to call more than once, on every exit path.
func (tx *IndexTransaction) Close() error {
	if tx == nil || tx.closed {
		return nil
	}
	tx.closed = true
	lockErr := tx.releaseOwnedLock()
	var tempErr error
	// Only a directory this transaction created is removed. When the
	// caller supplied the alternate index path (land's durable retained
	// index) its lifetime belongs to the caller's journal.
	if tx.tempDir != "" {
		if err := os.RemoveAll(tx.tempDir); err != nil {
			tempErr = fmt.Errorf("remove temp index %q: %w", tx.tempDir, err)
		}
	}
	switch {
	case lockErr != nil && tempErr != nil:
		return fmt.Errorf("%v; %v", lockErr, tempErr)
	case lockErr != nil:
		return lockErr
	default:
		return tempErr
	}
}

// randomNonce returns a hex nonce used to mark an owned lock.
func randomNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate lock nonce: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// StagedPathsEnv returns every path present in the index reachable
// through `env` as a change against HEAD, byte-exactly.
//
// `git diff --cached --name-only -z` is NUL-delimited and never quotes,
// so paths containing spaces, tabs or newlines survive intact — exactly
// the class of worktree name the audit has to catch. A repository with
// no HEAD yet falls back to `git ls-files --cached -z`, where every
// indexed path is by definition new.
func StagedPathsEnv(repoRoot string, env []string) ([]string, error) {
	out, err := runGitWithEnv(repoRoot, env, "-c", "core.quotePath=false",
		"diff", "--cached", "--name-only", "-z")
	if err != nil {
		fallback, ferr := runGitWithEnv(repoRoot, env, "-c", "core.quotePath=false",
			"ls-files", "--cached", "-z")
		if ferr != nil {
			return nil, fmt.Errorf("list staged paths: %v (ls-files fallback: %v)", err, ferr)
		}
		out = fallback
	}
	var paths []string
	for _, p := range strings.Split(out, "\x00") {
		if p == "" {
			continue
		}
		paths = append(paths, filepath.ToSlash(p))
	}
	return paths, nil
}

// StagedPaths is StagedPathsEnv against the ambient environment.
func StagedPaths(repoRoot string) ([]string, error) {
	return StagedPathsEnv(repoRoot, nil)
}

// AuditStagedPathsForNestedWorktreesEnv rediscovers the registered
// linked worktrees nested under repoRoot and returns every path staged
// in the index reachable through `env` that falls inside one.
//
// This is the post-stage half of land's transaction: it inspects what
// was actually staged rather than what land intended to stage, so it
// catches a worktree registered during the staging step itself.
// Discovery failure is returned as the fail-closed class.
func AuditStagedPathsForNestedWorktreesEnv(repoRoot string, env []string) (contaminated []string, err error) {
	nested, err := NestedWorktreePrefixes(repoRoot)
	if err != nil {
		return nil, err
	}
	staged, err := StagedPathsEnv(repoRoot, env)
	if err != nil {
		return nil, err
	}
	if len(nested) == 0 {
		return nil, nil
	}
	for _, p := range staged {
		if PathUnderNestedWorktree(p, nested) {
			contaminated = append(contaminated, p)
		}
	}
	return contaminated, nil
}

// AuditStagedPathsForNestedWorktrees audits the ambient index.
func AuditStagedPathsForNestedWorktrees(repoRoot string) ([]string, error) {
	return AuditStagedPathsForNestedWorktreesEnv(repoRoot, nil)
}

// runGitWithEnv runs git in repoRoot with an explicit environment.
// A nil env inherits the process environment unchanged.
func runGitWithEnv(repoRoot string, env []string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	if env != nil {
		cmd.Env = env
	}
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(out), fmt.Errorf("git %s: %s", strings.Join(args, " "), string(exitErr.Stderr))
		}
		return string(out), err
	}
	return string(out), nil
}
