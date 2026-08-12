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

// IndexTransaction isolates a sequence of index mutations in a private
// temporary index and publishes the result atomically under Git's own
// index lock.
type IndexTransaction struct {
	RepoRoot string
	// LivePath is the operator's effective index.
	LivePath string
	// TempPath is the private index every staged operation writes to.
	TempPath string

	live     *IndexFileState
	tempDir  string
	lockPath string
	lockFile *os.File
	closed   bool
}

// BeginIndexTransaction validates the effective index, snapshots it and
// seeds a private temporary index with identical bytes.
//
// When the repository has never staged anything the live index is
// absent; the temp index is left absent too, so the first `git add`
// creates it exactly as Git would.
func BeginIndexTransaction(repoRoot string) (*IndexTransaction, error) {
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
	tempDir, err := os.MkdirTemp("", "tpatch-land-index-*")
	if err != nil {
		return nil, fmt.Errorf("create temp index directory: %w", err)
	}
	tx := &IndexTransaction{
		RepoRoot: repoRoot,
		LivePath: livePath,
		TempPath: filepath.Join(tempDir, "index"),
		live:     live,
		tempDir:  tempDir,
		lockPath: livePath + ".lock",
	}
	if live.Existed {
		mode := live.Mode
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(tx.TempPath, live.Data, mode); err != nil {
			_ = os.RemoveAll(tempDir)
			return nil, fmt.Errorf("seed temp index: %w", err)
		}
	}
	return tx, nil
}

// Env returns the environment every git subprocess in this transaction
// must run with, so staging, audits, the commit and its hooks all see
// the private index instead of the operator's.
func (tx *IndexTransaction) Env() []string {
	return append(os.Environ(), "GIT_INDEX_FILE="+tx.TempPath)
}

// LockLive takes Git's own `<index>.lock` with O_EXCL — the same lock
// `git add` and `git commit` contend for.
func (tx *IndexTransaction) LockLive() error {
	if tx.lockFile != nil {
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
	return nil
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

// PublishLocked writes the temporary index over the live one through
// the held lock, using Git's own publish shape: bytes go into
// `<index>.lock`, then the lock is renamed onto the index. The rename
// consumes the lock, so the transaction no longer holds it afterwards.
func (tx *IndexTransaction) PublishLocked() error {
	if tx.lockFile == nil {
		return fmt.Errorf("publish index: lock not held")
	}
	temp, err := CaptureIndexFileState(tx.TempPath)
	if err != nil {
		return err
	}
	if !temp.Existed {
		// Nothing was ever staged and the live index was absent:
		// publishing means leaving that state in place.
		return tx.releaseLock()
	}
	if _, err := tx.lockFile.Write(temp.Data); err != nil {
		return fmt.Errorf("publish index: write lock file: %w", err)
	}
	if err := tx.lockFile.Close(); err != nil {
		tx.lockFile = nil
		return fmt.Errorf("publish index: close lock file: %w", err)
	}
	tx.lockFile = nil
	mode := tx.live.Mode
	if mode == 0 {
		mode = temp.Mode
	}
	if mode == 0 {
		mode = 0o644
	}
	if err := os.Chmod(tx.lockPath, mode); err != nil {
		return fmt.Errorf("publish index: chmod lock file: %w", err)
	}
	if err := os.Rename(tx.lockPath, tx.LivePath); err != nil {
		return fmt.Errorf("publish index: rename onto %q: %w", tx.LivePath, err)
	}
	return nil
}

// releaseLock closes and removes the lock this transaction created. A
// lock tpatch did not create is never removed.
func (tx *IndexTransaction) releaseLock() error {
	if tx.lockFile == nil {
		return nil
	}
	closeErr := tx.lockFile.Close()
	tx.lockFile = nil
	if err := os.Remove(tx.lockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("release index lock %q: %w", tx.lockPath, err)
	}
	if closeErr != nil {
		return fmt.Errorf("release index lock %q: %w", tx.lockPath, closeErr)
	}
	return nil
}

// Close releases any lock this transaction still holds and removes the
// private temp index. Safe to call more than once, on every exit path.
func (tx *IndexTransaction) Close() error {
	if tx == nil || tx.closed {
		return nil
	}
	tx.closed = true
	lockErr := tx.releaseLock()
	var tempErr error
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
