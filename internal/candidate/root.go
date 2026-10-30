package candidate

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
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

const candidateWorktreeLeaf = "worktree"

// ReferenceRoot is a read-only root reconstructed from one immutable commit.
type ReferenceRoot struct {
	repo   *heldRoot
	commit string
	closed bool
}

// CandidateRoot is a detached worktree rooted at one upstream commit.
type CandidateRoot struct {
	repo         *heldRoot
	ownsRepo     bool
	parent       *heldRoot
	worktree     *heldRoot
	repoRoot     string
	slug         string
	candidateID  string
	commit       string
	parentRel    string
	worktreeRel  string
	worktreePath string
	closed       bool
}

type heldRoot struct {
	path     string
	root     *os.Root
	identity os.FileInfo
	closed   bool
}

type absoluteRootPath string

// OpenReferenceRoot validates and opens a typed read-only reference root.
func OpenReferenceRoot(repoRoot string, commit string) (*ReferenceRoot, error) {
	repoRootAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repo root: %w", err)
	}
	if err := safety.EnsureSafeRepoPath(repoRootAbs, repoRootAbs); err != nil {
		return nil, fmt.Errorf("unsafe repo root: %w", err)
	}
	repo, err := openHeldRoot(absoluteRootPath(repoRootAbs))
	if err != nil {
		return nil, err
	}
	if err := validateCommit(commit); err != nil {
		_ = repo.close()
		return nil, err
	}
	ref := &ReferenceRoot{repo: repo, commit: commit}
	if err := ref.ensureLocalCommit(); err != nil {
		_ = ref.Close()
		return nil, err
	}
	return ref, nil
}

// CreateCandidateRoot validates inputs, creates one detached candidate worktree,
// and returns its typed root.
func CreateCandidateRoot(repoRoot, slug, candidateID, commit string, featureLock, worktreeMutationLock *candidatelock.Handle) (_ *CandidateRoot, returnedErr error) {
	repoRootAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repo root: %w", err)
	}
	if err := safety.EnsureSafeRepoPath(repoRootAbs, repoRootAbs); err != nil {
		return nil, fmt.Errorf("unsafe repo root: %w", err)
	}
	repo, err := openHeldRoot(absoluteRootPath(repoRootAbs))
	if err != nil {
		return nil, err
	}
	defer func() {
		if returnedErr != nil {
			_ = repo.close()
		}
	}()

	if err := validateCommit(commit); err != nil {
		return nil, err
	}
	if !slugPattern.MatchString(slug) {
		return nil, fmt.Errorf("candidate slug must be kebab-case: %q", slug)
	}
	if !candidateIDPattern.MatchString(candidateID) {
		return nil, fmt.Errorf("candidate id must match rc_<12 lowercase hex>: %q", candidateID)
	}

	parentRel := path.Join(".tpatch", "reconcile-candidates", slug, candidateID)
	worktreeRel := path.Join(parentRel, candidateWorktreeLeaf)
	worktreePath := filepath.Join(repo.path, filepath.FromSlash(worktreeRel))
	if err := safety.EnsureSafeRepoPath(repo.path, worktreePath); err != nil {
		return nil, fmt.Errorf("candidate worktree path escaped repository root: %w", err)
	}

	candidateRoot := &CandidateRoot{
		repo:         repo,
		ownsRepo:     true,
		repoRoot:     repo.path,
		slug:         slug,
		candidateID:  candidateID,
		commit:       commit,
		parentRel:    parentRel,
		worktreeRel:  worktreeRel,
		worktreePath: worktreePath,
	}
	if err := candidateRoot.ensureLocalCommit(); err != nil {
		return nil, err
	}
	if err := candidateRoot.ensureLockPathSafety(); err != nil {
		return nil, err
	}
	borrowed, err := candidateRoot.borrowMutationLocks(featureLock, worktreeMutationLock)
	if err != nil {
		return nil, err
	}
	defer borrowed.release()

	if err := repo.root.MkdirAll(parentRel, 0o755); err != nil {
		return nil, fmt.Errorf("create candidate namespace: %w", err)
	}
	parent, err := repo.openNestedRoot(parentRel)
	if err != nil {
		return nil, err
	}
	defer func() {
		if returnedErr != nil {
			_ = parent.close()
		}
	}()

	if _, err := parent.root.Lstat(candidateWorktreeLeaf); err == nil {
		return nil, fmt.Errorf("candidate worktree already exists: %s", worktreePath)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("inspect candidate worktree: %w", err)
	}

	stderr, err := candidateRoot.runGit("worktree", "add", "--detach", "-q", worktreePath, commit)
	if err != nil {
		cleanupErr := candidateRoot.cleanupFailedCreation(parent)
		if cleanupErr != nil {
			return nil, fmt.Errorf("git worktree add failed: %v: %s (cleanup: %v)", err, strings.TrimSpace(stderr), cleanupErr)
		}
		return nil, fmt.Errorf("git worktree add failed: %v: %s", err, strings.TrimSpace(stderr))
	}

	worktree, err := parent.openNestedRoot(candidateWorktreeLeaf)
	if err != nil {
		cleanupErr := candidateRoot.cleanupFailedCreation(parent)
		if cleanupErr != nil {
			return nil, fmt.Errorf("open candidate worktree root: %w (cleanup: %v)", err, cleanupErr)
		}
		return nil, fmt.Errorf("open candidate worktree root: %w", err)
	}

	candidateRoot.parent = parent
	candidateRoot.worktree = worktree
	return candidateRoot, nil
}

// Commit returns the immutable commit this reference root is bound to.
func (r *ReferenceRoot) Commit() string { return r.commit }

// Commit returns the upstream commit this candidate root was created from.
func (r *CandidateRoot) Commit() string { return r.commit }

// WorktreePath returns the detached worktree path.
func (r *CandidateRoot) WorktreePath() string { return r.worktreePath }

// CandidateID returns the candidate namespace identifier.
func (r *CandidateRoot) CandidateID() string { return r.candidateID }

// Close releases retained rooted handles.
func (r *ReferenceRoot) Close() error {
	if r == nil {
		return fmt.Errorf("reference root is nil")
	}
	if r.closed {
		return fmt.Errorf("reference root is already closed")
	}
	r.closed = true
	if r.repo == nil {
		return fmt.Errorf("reference root is not initialized")
	}
	return r.repo.close()
}

// Close releases retained rooted handles.
func (r *CandidateRoot) Close() error {
	if r == nil {
		return fmt.Errorf("candidate root is nil")
	}
	if r.closed {
		return fmt.Errorf("candidate root is already closed")
	}
	r.closed = true
	var first error
	if r.worktree != nil {
		if err := r.worktree.close(); err != nil && first == nil {
			first = err
		}
	}
	if r.parent != nil {
		if err := r.parent.close(); err != nil && first == nil {
			first = err
		}
	}
	if r.repo != nil && r.ownsRepo {
		if err := r.repo.close(); err != nil && first == nil {
			first = err
		}
	}
	if r.worktree == nil && r.parent == nil && r.repo == nil && first == nil {
		first = fmt.Errorf("candidate root is not initialized")
	}
	return first
}

// ReadFile reads one repo-relative file from the immutable reference commit.
func (r *ReferenceRoot) ReadFile(relPath string) ([]byte, error) {
	if err := r.ensureAvailable(); err != nil {
		return nil, err
	}
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
	if err := r.ensureAvailable(); err != nil {
		return nil, err
	}
	normalized, err := normalizeRelativePath(relPath)
	if err != nil {
		return nil, err
	}
	return readRootedRegularFile(r.worktree.root, normalized)
}

// WorktreeTreeHash returns the candidate worktree tree object ID including
// uncommitted edits, isolated from the live index.
func (r *CandidateRoot) WorktreeTreeHash() (string, error) {
	if err := r.ensureAvailable(); err != nil {
		return "", err
	}
	missing, err := r.worktree.validateCurrentPath(true)
	if err != nil {
		return "", err
	}
	if missing {
		return "", fmt.Errorf("candidate worktree path no longer exists: %s", r.worktree.path)
	}
	hash, err := gitutil.WorktreeTreeHash(r.worktree.path)
	if err != nil {
		return "", err
	}
	if _, err := r.worktree.validateCurrentPath(false); err != nil {
		return "", err
	}
	return hash, nil
}

// Prune removes the detached candidate worktree and any owned residue.
func (r *CandidateRoot) Prune(featureLock, worktreeMutationLock *candidatelock.Handle) error {
	if err := r.ensureAvailable(); err != nil {
		return err
	}
	if err := r.ensureLockPathSafety(); err != nil {
		return err
	}
	borrowed, err := r.borrowMutationLocks(featureLock, worktreeMutationLock)
	if err != nil {
		return err
	}
	defer borrowed.release()

	missing, err := r.worktree.validateCurrentPath(true)
	if err != nil {
		return err
	}

	stderr, gitErr := r.runGit("worktree", "remove", "--force", r.worktreePath)
	if gitErr != nil && !isGitMissingWorktree(stderr) {
		return r.closeWithContext(fmt.Errorf("git worktree remove failed: %v: %s", gitErr, strings.TrimSpace(stderr)))
	}

	if !missing {
		if err := r.removeOwnedWorktreeResidue(); err != nil {
			return r.closeWithContext(err)
		}
	}

	r.removeEmptyCandidateAncestors()
	if err := r.Close(); err != nil {
		return err
	}
	return nil
}

func (r *ReferenceRoot) ensureAvailable() error {
	if r == nil {
		return fmt.Errorf("reference root is nil")
	}
	if r.closed {
		return fmt.Errorf("reference root is closed")
	}
	if r.repo == nil || r.repo.closed {
		return fmt.Errorf("reference root is not initialized")
	}
	if _, err := r.repo.validateCurrentPath(false); err != nil {
		return err
	}
	return nil
}

func (r *CandidateRoot) ensureAvailable() error {
	if r == nil {
		return fmt.Errorf("candidate root is nil")
	}
	if r.closed {
		return fmt.Errorf("candidate root is closed")
	}
	if r.repo == nil || r.parent == nil || r.worktree == nil {
		return fmt.Errorf("candidate root is not initialized")
	}
	if r.repo.closed || r.parent.closed || r.worktree.closed {
		return fmt.Errorf("candidate root is not initialized")
	}
	if _, err := r.repo.validateCurrentPath(false); err != nil {
		return err
	}
	if _, err := r.parent.validateCurrentPath(false); err != nil {
		return err
	}
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
	if err := r.ensureAvailable(); err != nil {
		return nil, "", err
	}
	return runGitWithIdentity(r.repo, nil, args...)
}

func (r *CandidateRoot) runGit(args ...string) (string, error) {
	_, stderr, err := runGitWithIdentity(r.repo, nil, args...)
	return stderr, err
}

func runGitWithIdentity(repo *heldRoot, extraEnv []string, args ...string) ([]byte, string, error) {
	if repo == nil {
		return nil, "", fmt.Errorf("repository root is not initialized")
	}
	if _, err := repo.validateCurrentPath(false); err != nil {
		return nil, "", err
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = repo.path
	cmd.Env = gitutil.HardenedNoLazyGitEnv(extraEnv...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if _, pathErr := repo.validateCurrentPath(false); pathErr != nil {
		if err != nil {
			return nil, stderr.String(), fmt.Errorf("repository root identity changed while running git %s: %w", strings.Join(args, " "), pathErr)
		}
		return nil, stderr.String(), pathErr
	}
	return stdout, stderr.String(), err
}

func (r *CandidateRoot) cleanupFailedCreation(parent *heldRoot) error {
	if parent == nil {
		return fmt.Errorf("candidate namespace root is unavailable")
	}
	var issues []string
	stderr, err := r.runGit("worktree", "remove", "--force", r.worktreePath)
	if err != nil && !isGitMissingWorktree(stderr) {
		issues = append(issues, fmt.Sprintf("git worktree remove: %v: %s", err, strings.TrimSpace(stderr)))
	}
	if removeErr := parent.root.RemoveAll(candidateWorktreeLeaf); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
		issues = append(issues, fmt.Sprintf("remove owned residue: %v", removeErr))
	}
	if len(issues) == 0 {
		return nil
	}
	return errors.New(strings.Join(issues, "; "))
}

func (r *CandidateRoot) removeOwnedWorktreeResidue() error {
	entry, err := r.parent.root.Lstat(candidateWorktreeLeaf)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect candidate worktree residue: %w", err)
	}
	mode := entry.Mode()
	if mode&os.ModeSymlink != 0 || mode&os.ModeIrregular != 0 || !entry.IsDir() {
		return fmt.Errorf("candidate worktree entry changed unexpectedly before cleanup")
	}
	if !os.SameFile(entry, r.worktree.identity) {
		return fmt.Errorf("candidate worktree identity changed before cleanup")
	}
	if err := r.parent.root.RemoveAll(candidateWorktreeLeaf); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove owned candidate worktree residue: %w", err)
	}
	return nil
}

func (r *CandidateRoot) closeWithContext(primary error) error {
	if primary == nil {
		return nil
	}
	closeErr := r.Close()
	if closeErr == nil {
		return primary
	}
	return fmt.Errorf("%w (close: %v)", primary, closeErr)
}

func (r *CandidateRoot) removeEmptyCandidateAncestors() {
	_ = r.repo.root.Remove(r.parentRel)
	_ = r.repo.root.Remove(path.Dir(r.parentRel))
	_ = r.repo.root.Remove(path.Dir(path.Dir(r.parentRel)))
}

func openHeldRoot(path absoluteRootPath) (*heldRoot, error) {
	if err := ensureRootPathOpenable(string(path)); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(string(path))
	if err != nil {
		return nil, fmt.Errorf("open root %s: %w", string(path), err)
	}
	hold, err := holdRoot(string(path), root)
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	return hold, nil
}

func holdRoot(path string, root *os.Root) (*heldRoot, error) {
	if root == nil {
		return nil, fmt.Errorf("open root %s: root handle is nil", path)
	}
	identity, err := root.Stat(".")
	if err != nil {
		return nil, fmt.Errorf("capture root identity %s: %w", path, err)
	}
	current, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat root path %s: %w", path, err)
	}
	if !current.IsDir() {
		return nil, fmt.Errorf("root path is not a directory: %s", path)
	}
	if !os.SameFile(identity, current) {
		return nil, fmt.Errorf("root path identity changed while opening %s", path)
	}
	return &heldRoot{path: path, root: root, identity: identity}, nil
}

func ensureRootPathOpenable(path string) error {
	leaf, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect root path %s: %w", path, err)
	}
	if leaf.Mode()&os.ModeSymlink != 0 || leaf.Mode()&os.ModeIrregular != 0 {
		return fmt.Errorf("root path must not be a symlink or irregular node: %s", path)
	}
	if !leaf.IsDir() {
		return fmt.Errorf("root path is not a directory: %s", path)
	}
	return nil
}

func (h *heldRoot) openNestedRoot(relative string) (*heldRoot, error) {
	if h == nil || h.root == nil || h.closed {
		return nil, fmt.Errorf("root handle is unavailable")
	}
	normalized, err := normalizeRelativePath(relative)
	if err != nil {
		return nil, err
	}
	childPath := filepath.Join(h.path, filepath.FromSlash(normalized))
	if err := ensureRootPathOpenable(childPath); err != nil {
		return nil, err
	}
	child, err := h.root.OpenRoot(normalized)
	if err != nil {
		return nil, fmt.Errorf("open nested root %s: %w", normalized, err)
	}
	hold, err := holdRoot(childPath, child)
	if err != nil {
		_ = child.Close()
		return nil, err
	}
	return hold, nil
}

func (h *heldRoot) validateCurrentPath(allowMissing bool) (bool, error) {
	if h == nil || h.root == nil || h.closed {
		return false, fmt.Errorf("root handle is unavailable")
	}
	current, err := os.Stat(h.path)
	if err != nil {
		if allowMissing && errors.Is(err, fs.ErrNotExist) {
			return true, nil
		}
		return false, fmt.Errorf("stat rooted path %s: %w", h.path, err)
	}
	if !os.SameFile(h.identity, current) {
		return false, fmt.Errorf("root path identity changed: %s", h.path)
	}
	return false, nil
}

func (h *heldRoot) close() error {
	if h == nil {
		return nil
	}
	if h.closed {
		return nil
	}
	h.closed = true
	if h.root == nil {
		return nil
	}
	err := h.root.Close()
	h.root = nil
	return err
}

func readRootedRegularFile(root *os.Root, name string) ([]byte, error) {
	pre, err := root.Lstat(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fs.ErrNotExist
		}
		return nil, fmt.Errorf("lstat %s: %w", name, err)
	}
	if err := validateRegularFileMode(pre.Mode(), name); err != nil {
		return nil, err
	}
	file, err := root.OpenFile(name, os.O_RDONLY|candidateReadOpenFlags(), 0)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fs.ErrNotExist
		}
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	defer file.Close()
	post, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", name, err)
	}
	if err := validateRegularFileMode(post.Mode(), name); err != nil {
		return nil, err
	}
	if !os.SameFile(pre, post) {
		return nil, fmt.Errorf("file identity changed while opening %s", name)
	}
	body, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return body, nil
}

func validateRegularFileMode(mode fs.FileMode, name string) error {
	if mode&os.ModeSymlink != 0 || mode&os.ModeIrregular != 0 ||
		mode&os.ModeNamedPipe != 0 || mode&os.ModeDevice != 0 ||
		mode&os.ModeCharDevice != 0 || mode&os.ModeSocket != 0 ||
		!mode.IsRegular() {
		return fmt.Errorf("path is not a regular file: %s", name)
	}
	return nil
}

func (r *CandidateRoot) featureLockPath() string {
	return filepath.Join(r.repoRoot, ".tpatch", "locks", "reconcile", r.slug+".lock")
}

func (r *CandidateRoot) globalWorktreeMutationLockPath() string {
	return filepath.Join(r.repoRoot, ".tpatch", "locks", "worktree-mutation.lock")
}

func (r *CandidateRoot) ensureLockPathSafety() error {
	for _, lockPath := range []string{r.featureLockPath(), r.globalWorktreeMutationLockPath()} {
		if err := safety.EnsureSafeRepoPath(r.repoRoot, lockPath); err != nil {
			return fmt.Errorf("lock path escaped repository root: %w", err)
		}
	}
	return nil
}

type borrowedMutationLocks struct {
	releaseGlobal  func()
	releaseFeature func()
}

func (b *borrowedMutationLocks) release() {
	if b == nil {
		return
	}
	if b.releaseGlobal != nil {
		b.releaseGlobal()
	}
	if b.releaseFeature != nil {
		b.releaseFeature()
	}
}

func (r *CandidateRoot) borrowMutationLocks(featureLock, worktreeMutationLock *candidatelock.Handle) (*borrowedMutationLocks, error) {
	if featureLock == nil {
		return nil, fmt.Errorf("feature lock handle is required")
	}
	if worktreeMutationLock == nil {
		return nil, fmt.Errorf("worktree mutation lock handle is required")
	}
	if featureLock == worktreeMutationLock {
		return nil, fmt.Errorf("feature and worktree mutation lock handles must be distinct")
	}
	featurePath := r.featureLockPath()
	globalPath := r.globalWorktreeMutationLockPath()
	if err := r.ensureLockPathBoundToRepo(featurePath); err != nil {
		return nil, err
	}
	if err := r.ensureLockPathBoundToRepo(globalPath); err != nil {
		return nil, err
	}
	releaseFeature, err := featureLock.Borrow(featurePath)
	if err != nil {
		return nil, err
	}
	releaseGlobal, err := worktreeMutationLock.Borrow(globalPath)
	if err != nil {
		releaseFeature()
		return nil, err
	}
	return &borrowedMutationLocks{releaseGlobal: releaseGlobal, releaseFeature: releaseFeature}, nil
}

func (r *CandidateRoot) borrowFeatureLock(featureLock *candidatelock.Handle) (func(), error) {
	if featureLock == nil {
		return nil, fmt.Errorf("feature lock handle is required")
	}
	featurePath := r.featureLockPath()
	if err := r.ensureLockPathBoundToRepo(featurePath); err != nil {
		return nil, err
	}
	return featureLock.Borrow(featurePath)
}

func (r *CandidateRoot) ensureLockPathBoundToRepo(lockPath string) error {
	return ensureLockPathBoundToRepo(r.repo, r.repoRoot, lockPath)
}

func ensureLockPathBoundToRepo(repo *heldRoot, repoRoot, lockPath string) error {
	if repo == nil || repo.root == nil {
		return fmt.Errorf("repository root is not initialized")
	}
	if err := safety.EnsureSafeRepoPath(repoRoot, lockPath); err != nil {
		return fmt.Errorf("lock path escaped repository root: %w", err)
	}
	rel, err := filepath.Rel(repoRoot, lockPath)
	if err != nil {
		return fmt.Errorf("resolve lock path relative to repo root: %w", err)
	}
	normalized, err := normalizeRelativePath(rel)
	if err != nil {
		return fmt.Errorf("normalize lock path relative to repo root: %w", err)
	}
	info, err := repo.root.Lstat(normalized)
	if err != nil {
		return fmt.Errorf("stat lock path from held repo root: %w", err)
	}
	mode := info.Mode()
	if mode&os.ModeSymlink != 0 || mode&os.ModeIrregular != 0 || !mode.IsRegular() {
		return fmt.Errorf("lock path must be a regular file inside the repository root")
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
