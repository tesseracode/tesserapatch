// Effective-index snapshot/restore and staged-path inspection
// (GH #7 rev-5).
//
// `tpatch land` stages a computed path set and then commits. Between
// the moment land decides WHAT to stage and the moment `git add` runs,
// an agent harness can register a linked worktree — and `git add` on a
// directory that has just become another checkout's working directory
// stages it as a `mode 160000` gitlink. No amount of pre-stage planning
// can close that window on its own, because the window is *inside* the
// staging step.
//
// The closure is a transaction: snapshot the effective index, stage,
// audit what actually landed in the index against a freshly discovered
// worktree set, and restore the exact pre-land index bytes if anything
// is wrong. The operator's own staged state is preserved byte-for-byte
// across a rollback, because the snapshot is of the whole index file.
//
// Scope of the guarantee, stated precisely: a worktree registered
// *after* the audit cannot enter the index by registration alone —
// registration does not stage anything, and land performs no further
// broad staging after the audit. A concurrent third-party `git add`
// racing land's commit remains outside supported semantics.

package gitutil

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// IndexSnapshot is a byte-for-byte capture of a repository's effective
// index file, including the "there was no index" case.
type IndexSnapshot struct {
	// Path is the absolute effective index path this snapshot was
	// taken from.
	Path string
	// Existed reports whether the index file was present.
	Existed bool
	// Data is the exact index bytes when Existed is true.
	Data []byte
	// Mode is the original file mode when Existed is true.
	Mode fs.FileMode
}

// EffectiveIndexPath resolves the index file Git will actually use for
// repoRoot, as an absolute path.
//
// `git rev-parse --git-path index` is authoritative for all three
// shapes tpatch cares about: the main worktree (`.git/index`), a linked
// worktree (`.git/worktrees/<name>/index` under the common directory),
// and a redirected `GIT_INDEX_FILE`. The returned value may be
// relative, in which case it is resolved against repoRoot exactly as
// Git would.
func EffectiveIndexPath(repoRoot string) (string, error) {
	out, err := runGit(repoRoot, "rev-parse", "--git-path", "index")
	if err != nil {
		return "", fmt.Errorf("resolve effective git index: %w", err)
	}
	p := strings.TrimSpace(out)
	if p == "" {
		return "", fmt.Errorf("resolve effective git index: git returned an empty path")
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(repoRoot, p)
	}
	return p, nil
}

// SnapshotIndex captures the effective index for repoRoot.
//
// Reading the file does not disturb it, so the operator's staged state
// is untouched by the snapshot itself. A missing index is a valid
// snapshot (`Existed == false`) — a repository that has never staged
// anything has no index file, and restoring that state means removing
// whatever index land created.
func SnapshotIndex(repoRoot string) (*IndexSnapshot, error) {
	path, err := EffectiveIndexPath(repoRoot)
	if err != nil {
		return nil, err
	}
	snap := &IndexSnapshot{Path: path}
	info, statErr := os.Stat(path)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return snap, nil
		}
		return nil, fmt.Errorf("stat effective git index %q: %w", path, statErr)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return nil, fmt.Errorf("read effective git index %q: %w", path, readErr)
	}
	snap.Existed = true
	snap.Data = data
	snap.Mode = info.Mode().Perm()
	return snap, nil
}

// Restore puts the effective index back exactly as SnapshotIndex found
// it: identical bytes and mode, or absent if it was absent.
//
// The write is atomic (temp file in the same directory, then rename)
// so a crash mid-restore cannot leave a truncated index behind. Any
// stale `index.lock` land itself could have left is not touched —
// `git add` removes its own lock, and removing someone else's lock
// would be unsafe.
func (s *IndexSnapshot) Restore() error {
	if s == nil {
		return fmt.Errorf("restore index: nil snapshot")
	}
	if !s.Existed {
		if err := os.Remove(s.Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("restore index: remove %q: %w", s.Path, err)
		}
		return nil
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("restore index: create %q: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".tpatch-index-restore-*")
	if err != nil {
		return fmt.Errorf("restore index: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(s.Data); err != nil {
		tmp.Close()
		return fmt.Errorf("restore index: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("restore index: close temp: %w", err)
	}
	mode := s.Mode
	if mode == 0 {
		mode = 0o644
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return fmt.Errorf("restore index: chmod temp: %w", err)
	}
	if err := os.Rename(tmpPath, s.Path); err != nil {
		return fmt.Errorf("restore index: rename onto %q: %w", s.Path, err)
	}
	return nil
}

// StagedPaths returns every path currently present in the index as a
// change against HEAD, byte-exactly.
//
// `git diff --cached --name-only -z` is NUL-delimited and never quotes,
// so paths containing spaces, tabs or newlines survive intact — which
// is exactly the class of worktree name this audit has to catch. A
// repository with no HEAD yet falls back to `git ls-files --cached -z`,
// where every indexed path is by definition new.
func StagedPaths(repoRoot string) ([]string, error) {
	out, err := runGit(repoRoot, "-c", "core.quotePath=false",
		"diff", "--cached", "--name-only", "-z")
	if err != nil {
		fallback, ferr := runGit(repoRoot, "-c", "core.quotePath=false",
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

// AuditStagedPathsForNestedWorktrees rediscovers the registered linked
// worktrees nested under repoRoot and returns every currently staged
// path that falls inside one.
//
// This is the post-stage half of land's staging transaction: it catches
// a worktree that was registered after planning, whose directory `git
// add` therefore turned into a gitlink. Discovery failure is returned
// as the fail-closed class so the caller rolls back rather than
// committing an unaudited index.
func AuditStagedPathsForNestedWorktrees(repoRoot string) (contaminated []string, err error) {
	nested, err := NestedWorktreePrefixes(repoRoot)
	if err != nil {
		return nil, err
	}
	staged, err := StagedPaths(repoRoot)
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
