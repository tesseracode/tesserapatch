package candidate

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/candidatelock"
	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/safety"
)

var (
	commitHexPattern   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	candidateIDPattern = regexp.MustCompile(`^rc_[0-9a-f]{12}$`)
	slugPattern        = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

// ReferenceRoot is a read-only root reconstructed from one immutable commit.
type ReferenceRoot struct {
	repoRoot string
	commit   string
}

// CandidateRoot is a detached worktree rooted at one upstream commit.
type CandidateRoot struct {
	repoRoot     string
	slug         string
	candidateID  string
	commit       string
	worktreePath string
}

// OpenReferenceRoot validates and opens a typed read-only reference root.
func OpenReferenceRoot(repoRoot string, commit string) (*ReferenceRoot, error) {
	rootAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repo root: %w", err)
	}
	if err := safety.EnsureSafeRepoPath(rootAbs, rootAbs); err != nil {
		return nil, fmt.Errorf("unsafe repo root: %w", err)
	}
	if err := validateCommit(commit); err != nil {
		return nil, err
	}
	ref := &ReferenceRoot{
		repoRoot: rootAbs,
		commit:   commit,
	}
	if err := ref.ensureLocalCommit(); err != nil {
		return nil, err
	}
	return ref, nil
}

// CreateCandidateRoot validates inputs, creates one detached candidate worktree,
// and returns its typed root.
func CreateCandidateRoot(repoRoot, slug, candidateID, commit string) (*CandidateRoot, error) {
	rootAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repo root: %w", err)
	}
	if err := safety.EnsureSafeRepoPath(rootAbs, rootAbs); err != nil {
		return nil, fmt.Errorf("unsafe repo root: %w", err)
	}
	if err := validateCommit(commit); err != nil {
		return nil, err
	}
	if !slugPattern.MatchString(slug) {
		return nil, fmt.Errorf("candidate slug must be kebab-case: %q", slug)
	}
	if !candidateIDPattern.MatchString(candidateID) {
		return nil, fmt.Errorf("candidate id must match rc_<12 lowercase hex>: %q", candidateID)
	}
	root := &CandidateRoot{
		repoRoot:    rootAbs,
		slug:        slug,
		candidateID: candidateID,
		commit:      commit,
	}
	if err := root.ensureLocalCommit(); err != nil {
		return nil, err
	}
	if err := root.ensureCandidatePathSafety(); err != nil {
		return nil, err
	}
	if err := root.ensureLockPathSafety(); err != nil {
		return nil, err
	}
	featureLock, err := candidatelock.TryAcquire(root.featureLockPath())
	if err != nil {
		return nil, err
	}
	defer featureLock.Release()

	if err := os.MkdirAll(filepath.Dir(root.worktreePath), 0o755); err != nil {
		return nil, fmt.Errorf("create candidate worktree parent: %w", err)
	}
	if err := assertNoSymlinkComponents(root.repoRoot, root.worktreePath); err != nil {
		return nil, err
	}
	if _, statErr := os.Stat(root.worktreePath); statErr == nil {
		return nil, fmt.Errorf("candidate worktree already exists: %s", root.worktreePath)
	} else if !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("inspect candidate worktree: %w", statErr)
	}

	globalLock, err := candidatelock.TryAcquire(root.globalWorktreeMutationLockPath())
	if err != nil {
		return nil, err
	}
	lockReleased := false
	defer func() {
		if !lockReleased {
			_ = globalLock.Release()
		}
	}()

	stderr, err := root.runGit("worktree", "add", "--detach", "-q", root.worktreePath, root.commit)
	lockReleased = true
	_ = globalLock.Release()
	if err != nil {
		_ = os.RemoveAll(root.worktreePath)
		return nil, fmt.Errorf("git worktree add failed: %v: %s", err, strings.TrimSpace(stderr))
	}
	return root, nil
}

// Commit returns the immutable commit this reference root is bound to.
func (r *ReferenceRoot) Commit() string { return r.commit }

// Commit returns the upstream commit this candidate root was created from.
func (r *CandidateRoot) Commit() string { return r.commit }

// WorktreePath returns the detached worktree path.
func (r *CandidateRoot) WorktreePath() string { return r.worktreePath }

// CandidateID returns the candidate namespace identifier.
func (r *CandidateRoot) CandidateID() string { return r.candidateID }

// ReadFile reads one repo-relative file from the immutable reference commit.
func (r *ReferenceRoot) ReadFile(relPath string) ([]byte, error) {
	normalized, err := normalizeRelativePath(relPath)
	if err != nil {
		return nil, err
	}
	stdout, stderr, err := r.runGitWithOutput("show", r.commit+":"+normalized)
	if err != nil {
		if isGitMissingPath(stderr) {
			return nil, fs.ErrNotExist
		}
		return nil, fmt.Errorf("git show %s:%s failed: %v: %s", r.commit, normalized, err, strings.TrimSpace(stderr))
	}
	return stdout, nil
}

// ReadFile reads one file from the detached candidate worktree.
func (r *CandidateRoot) ReadFile(relPath string) ([]byte, error) {
	normalized, err := normalizeRelativePath(relPath)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(r.worktreePath, filepath.FromSlash(normalized))
	if err := assertNoSymlinkComponents(r.worktreePath, path); err != nil {
		return nil, err
	}
	if err := safety.EnsureSafeRepoPath(r.worktreePath, path); err != nil {
		return nil, fmt.Errorf("unsafe candidate path: %w", err)
	}
	return os.ReadFile(path)
}

// Prune removes the detached candidate worktree and its residue.
func (r *CandidateRoot) Prune() error {
	if err := r.ensureLockPathSafety(); err != nil {
		return err
	}
	featureLock, err := candidatelock.TryAcquire(r.featureLockPath())
	if err != nil {
		return err
	}
	defer featureLock.Release()

	globalLock, err := candidatelock.TryAcquire(r.globalWorktreeMutationLockPath())
	if err != nil {
		return err
	}
	stderr, gitErr := r.runGit("worktree", "remove", "--force", r.worktreePath)
	if releaseErr := globalLock.Release(); releaseErr != nil && gitErr == nil {
		return releaseErr
	}
	if gitErr != nil && !isGitMissingWorktree(stderr) {
		return fmt.Errorf("git worktree remove failed: %v: %s", gitErr, strings.TrimSpace(stderr))
	}
	if err := os.RemoveAll(r.worktreePath); err != nil {
		return fmt.Errorf("remove worktree residue: %w", err)
	}
	_ = os.Remove(filepath.Dir(r.worktreePath))
	_ = os.Remove(filepath.Dir(filepath.Dir(r.worktreePath)))
	return nil
}

func (r *ReferenceRoot) ensureLocalCommit() error {
	stderr, err := r.runGit("cat-file", "-e", r.commit+"^{commit}")
	if err != nil {
		return fmt.Errorf("reference commit %s is not available locally: %v: %s", r.commit, err, strings.TrimSpace(stderr))
	}
	return nil
}

func (r *CandidateRoot) ensureLocalCommit() error {
	stderr, err := r.runGit("cat-file", "-e", r.commit+"^{commit}")
	if err != nil {
		return fmt.Errorf("candidate base commit %s is not available locally: %v: %s", r.commit, err, strings.TrimSpace(stderr))
	}
	return nil
}

func (r *ReferenceRoot) runGit(args ...string) (string, error) {
	_, stderr, err := r.runGitWithOutput(args...)
	return stderr, err
}

func (r *ReferenceRoot) runGitWithOutput(args ...string) ([]byte, string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.repoRoot
	cmd.Env = append(os.Environ(), gitutil.NoLazyFetchEnv)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	return stdout, stderr.String(), err
}

func (r *CandidateRoot) runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.repoRoot
	cmd.Env = append(os.Environ(), gitutil.NoLazyFetchEnv)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stderr.String(), err
}

func (r *CandidateRoot) ensureCandidatePathSafety() error {
	root := filepath.Join(
		r.repoRoot, ".tpatch", "reconcile-candidates", r.slug, r.candidateID, "worktree",
	)
	if err := safety.EnsureSafeRepoPath(r.repoRoot, root); err != nil {
		return fmt.Errorf("candidate worktree path escaped repository root: %w", err)
	}
	if err := assertNoSymlinkComponents(r.repoRoot, root); err != nil {
		return err
	}
	r.worktreePath = root
	return nil
}

func (r *CandidateRoot) featureLockPath() string {
	return filepath.Join(r.repoRoot, ".tpatch", "locks", "reconcile", r.slug+".lock")
}

func (r *CandidateRoot) globalWorktreeMutationLockPath() string {
	return filepath.Join(r.repoRoot, ".tpatch", "locks", "worktree-mutation.lock")
}

func (r *CandidateRoot) ensureLockPathSafety() error {
	for _, path := range []string{r.featureLockPath(), r.globalWorktreeMutationLockPath()} {
		if err := safety.EnsureSafeRepoPath(r.repoRoot, path); err != nil {
			return fmt.Errorf("lock path escaped repository root: %w", err)
		}
		if err := assertNoSymlinkComponents(r.repoRoot, path); err != nil {
			return err
		}
	}
	return nil
}

func validateCommit(commit string) error {
	if !commitHexPattern.MatchString(commit) {
		return fmt.Errorf("commit must be a 40-character lowercase hex string: %q", commit)
	}
	return nil
}

func normalizeRelativePath(path string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(path))
	if strings.Contains(clean, ":") {
		return "", fmt.Errorf("path contains an unsupported colon separator: %q", path)
	}
	if clean == "." || clean == "" || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", fmt.Errorf("path must remain inside the root: %q", path)
	}
	if !fs.ValidPath(clean) {
		return "", fmt.Errorf("path is not a valid rooted relative path: %q", path)
	}
	return clean, nil
}

func assertNoSymlinkComponents(root, path string) error {
	if err := safety.EnsureSafeRepoPath(root, path); err != nil {
		return fmt.Errorf("path escaped root: %w", err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return fmt.Errorf("resolve rooted relative path: %w", err)
	}
	rel = filepath.Clean(rel)
	if rel == "." {
		return nil
	}
	cursor := root
	for _, segment := range strings.Split(filepath.ToSlash(rel), "/") {
		if segment == "." || segment == "" {
			continue
		}
		cursor = filepath.Join(cursor, segment)
		info, statErr := os.Lstat(cursor)
		if os.IsNotExist(statErr) {
			return nil
		}
		if statErr != nil {
			return fmt.Errorf("inspect rooted path component %s: %w", cursor, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path component is a symlink: %s", cursor)
		}
	}
	return nil
}

func isGitMissingPath(stderr string) bool {
	trimmed := strings.TrimSpace(stderr)
	return strings.Contains(trimmed, "exists on disk, but not in") ||
		strings.Contains(trimmed, "does not exist in") ||
		strings.Contains(trimmed, "Path '") && strings.Contains(trimmed, "does not exist")
}

func isGitMissingWorktree(stderr string) bool {
	trimmed := strings.TrimSpace(stderr)
	return strings.Contains(trimmed, "is not a working tree") ||
		strings.Contains(trimmed, "No such file or directory")
}
