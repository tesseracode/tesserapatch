package candidate

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/safety"
)

const (
	CandidateMetadataName = "candidate.json"
	CandidateStateName    = "state.json"
	CandidatePatchName    = "candidate.patch"
	CandidateWorktreeName = "worktree"
	CandidateJournalName  = "accept-journal.json"
	CandidateRejectedName = "rejected.jsonl"
)

func CandidateSlugRelative(slug string) (string, error) {
	if !slugPattern.MatchString(slug) {
		return "", fmt.Errorf("candidate slug must be kebab-case: %q", slug)
	}
	return path.Join(".tpatch", "reconcile-candidates", slug), nil
}

func CandidateDirectoryRelative(slug, candidateID string) (string, error) {
	root, err := CandidateSlugRelative(slug)
	if err != nil {
		return "", err
	}
	if !candidateIDPattern.MatchString(candidateID) {
		return "", fmt.Errorf("candidate id must match rc_<12 lowercase hex>: %q", candidateID)
	}
	return path.Join(root, candidateID), nil
}

func CandidatePointerPath(slug, candidateID string) (string, error) {
	return CandidateDirectoryRelative(slug, candidateID)
}

func CandidateWorktreeRelative(slug, candidateID string) (string, error) {
	root, err := CandidateDirectoryRelative(slug, candidateID)
	if err != nil {
		return "", err
	}
	return path.Join(root, CandidateWorktreeName), nil
}

func CandidateRejectedLogRelative(slug string) (string, error) {
	root, err := CandidateSlugRelative(slug)
	if err != nil {
		return "", err
	}
	return path.Join(root, CandidateRejectedName), nil
}

func CandidateRecoveryJournalRelative(slug, candidateID string) (string, error) {
	root, err := CandidateDirectoryRelative(slug, candidateID)
	if err != nil {
		return "", err
	}
	return path.Join(root, CandidateJournalName), nil
}

func (r *ReferenceRoot) FindCandidateRecoveryJournal(slug string) (string, error) {
	if r == nil {
		return "", fmt.Errorf("reference root is nil")
	}
	if err := r.ensureAvailable(); err != nil {
		return "", err
	}
	slugRoot, err := r.openCandidateSlugRoot(slug, false)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	defer func() { _ = slugRoot.close() }()
	var found string
	walkErr := filepath.WalkDir(slugRoot.path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		mode := entry.Type()
		if mode&os.ModeSymlink != 0 || mode&os.ModeIrregular != 0 {
			return fmt.Errorf("candidate slug path changed unexpectedly")
		}
		if entry.IsDir() || entry.Name() != CandidateJournalName {
			return nil
		}
		relative, err := filepath.Rel(r.repo.path, current)
		if err != nil {
			return err
		}
		found = filepath.ToSlash(relative)
		return fs.SkipAll
	})
	if walkErr != nil && !errors.Is(walkErr, fs.SkipAll) {
		return "", walkErr
	}
	if found != "" {
		return found, nil
	}
	return "", nil
}

func (r *ReferenceRoot) ReadRejectedRecords(slug string) ([]CandidateRejectedRecord, error) {
	if r == nil {
		return nil, fmt.Errorf("reference root is nil")
	}
	if err := r.ensureAvailable(); err != nil {
		return nil, err
	}
	slugRoot, err := r.openCandidateSlugRoot(slug, false)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = slugRoot.close() }()
	body, err := readRootedRegularFile(slugRoot.root, CandidateRejectedName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	rows := []CandidateRejectedRecord{}
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		row, err := DecodeRejectedRecord([]byte(line))
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ReferenceRoot) AppendRejectedRecord(slug string, record CandidateRejectedRecord) error {
	if r == nil {
		return fmt.Errorf("reference root is nil")
	}
	if err := r.ensureAvailable(); err != nil {
		return err
	}
	body, err := EncodeRejectedRecord(record)
	if err != nil {
		return err
	}
	slugRoot, err := r.openCandidateSlugRoot(slug, true)
	if err != nil {
		return err
	}
	defer func() { _ = slugRoot.close() }()
	return appendRootedRegularFile(slugRoot.root, CandidateRejectedName, body)
}

func (r *ReferenceRoot) ClearRejectedRecords(slug string) error {
	if r == nil {
		return fmt.Errorf("reference root is nil")
	}
	if err := r.ensureAvailable(); err != nil {
		return err
	}
	slugRoot, err := r.openCandidateSlugRoot(slug, true)
	if err != nil {
		return err
	}
	defer func() { _ = slugRoot.close() }()
	return clearRootedRegularFile(slugRoot.root, CandidateRejectedName)
}

func (r *CandidateRoot) ReadArtifact() (CandidateArtifact, error) {
	if err := r.ensureAvailable(); err != nil {
		return CandidateArtifact{}, err
	}
	body, err := readRootedRegularFile(r.parent.root, CandidateMetadataName)
	if err != nil {
		return CandidateArtifact{}, err
	}
	return DecodeCandidateArtifact(body)
}

func (r *CandidateRoot) ReadState() (CandidateState, error) {
	if err := r.ensureAvailable(); err != nil {
		return CandidateState{}, err
	}
	body, err := readRootedRegularFile(r.parent.root, CandidateStateName)
	if err != nil {
		return CandidateState{}, err
	}
	return DecodeCandidateState(body)
}

func (r *CandidateRoot) ReadPatch() ([]byte, error) {
	if err := r.ensureAvailable(); err != nil {
		return nil, err
	}
	return readRootedRegularFile(r.parent.root, CandidatePatchName)
}

func (r *CandidateRoot) WriteArtifact(artifact CandidateArtifact) error {
	if err := r.ensureAvailable(); err != nil {
		return err
	}
	body, err := EncodeCandidateArtifact(artifact)
	if err != nil {
		return err
	}
	return rewriteRootedRegularFile(r.parent.root, CandidateMetadataName, body)
}

func (r *CandidateRoot) WriteState(state CandidateState) error {
	if err := r.ensureAvailable(); err != nil {
		return err
	}
	body, err := EncodeCandidateState(state)
	if err != nil {
		return err
	}
	return rewriteRootedRegularFile(r.parent.root, CandidateStateName, body)
}

func (r *CandidateRoot) WritePatch(body []byte) error {
	if err := r.ensureAvailable(); err != nil {
		return err
	}
	return rewriteRootedRegularFile(r.parent.root, CandidatePatchName, body)
}

// OpenExistingCandidateRoot reopens an already-created candidate namespace
// through a typed reference capability; no bare repo root forwarding is used.
func OpenExistingCandidateRoot(reference *ReferenceRoot, slug, candidateID string) (_ *CandidateRoot, returnedErr error) {
	if reference == nil {
		return nil, fmt.Errorf("reference root is nil")
	}
	if err := reference.ensureAvailable(); err != nil {
		return nil, err
	}
	if !slugPattern.MatchString(slug) {
		return nil, fmt.Errorf("candidate slug must be kebab-case: %q", slug)
	}
	if !candidateIDPattern.MatchString(candidateID) {
		return nil, fmt.Errorf("candidate id must match rc_<12 lowercase hex>: %q", candidateID)
	}
	parentRel, err := CandidateDirectoryRelative(slug, candidateID)
	if err != nil {
		return nil, err
	}
	worktreeRel := path.Join(parentRel, CandidateWorktreeName)
	worktreePath := filepath.Join(reference.repo.path, filepath.FromSlash(worktreeRel))
	if err := safety.EnsureSafeRepoPath(reference.repo.path, worktreePath); err != nil {
		return nil, fmt.Errorf("candidate worktree path escaped repository root: %w", err)
	}
	parent, err := reference.repo.openNestedRoot(parentRel)
	if err != nil {
		return nil, err
	}
	defer func() {
		if returnedErr != nil {
			_ = parent.close()
		}
	}()
	worktree, err := parent.openNestedRoot(CandidateWorktreeName)
	if err != nil {
		return nil, err
	}
	candidateRoot := &CandidateRoot{
		repo:         reference.repo,
		ownsRepo:     false,
		parent:       parent,
		worktree:     worktree,
		repoRoot:     reference.repo.path,
		slug:         slug,
		candidateID:  candidateID,
		commit:       reference.commit,
		parentRel:    parentRel,
		worktreeRel:  worktreeRel,
		worktreePath: worktreePath,
	}
	if err := candidateRoot.ensureLockPathSafety(); err != nil {
		_ = candidateRoot.Close()
		return nil, err
	}
	if err := candidateRoot.validateReopenedCandidateAuthority(reference.commit); err != nil {
		_ = candidateRoot.Close()
		return nil, err
	}
	return candidateRoot, nil
}

func (r *ReferenceRoot) openCandidateSlugRoot(slug string, create bool) (*heldRoot, error) {
	if err := r.ensureAvailable(); err != nil {
		return nil, err
	}
	slugRel, err := CandidateSlugRelative(slug)
	if err != nil {
		return nil, err
	}
	if create {
		if err := ensureRootedDirectoryPath(r.repo.root, slugRel); err != nil {
			return nil, err
		}
	}
	root, err := r.repo.openNestedRoot(slugRel)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fs.ErrNotExist
		}
		return nil, err
	}
	return root, nil
}

func ensureRootedDirectoryPath(root *os.Root, relative string) error {
	normalized, err := normalizeRelativePath(relative)
	if err != nil {
		return err
	}
	segments := strings.Split(normalized, "/")
	current := ""
	for _, segment := range segments {
		if current == "" {
			current = segment
		} else {
			current = path.Join(current, segment)
		}
		info, err := root.Lstat(current)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			if mkErr := root.Mkdir(current, 0o755); mkErr != nil && !errors.Is(mkErr, fs.ErrExist) {
				return mkErr
			}
			info, err = root.Lstat(current)
			if err != nil {
				return err
			}
		}
		mode := info.Mode()
		if mode&os.ModeSymlink != 0 || mode&os.ModeIrregular != 0 || !info.IsDir() {
			return fmt.Errorf("path is not a safe directory: %s", current)
		}
	}
	return nil
}

func rewriteRootedRegularFile(root *os.Root, name string, body []byte) error {
	pre, err := root.Lstat(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			file, createErr := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
			if createErr != nil {
				if errors.Is(createErr, fs.ErrExist) {
					return rewriteRootedRegularFile(root, name, body)
				}
				return createErr
			}
			defer file.Close()
			post, statErr := file.Stat()
			if statErr != nil {
				return statErr
			}
			if err := validateRegularFileMode(post.Mode(), name); err != nil {
				return err
			}
			if len(body) == 0 {
				return nil
			}
			_, writeErr := file.Write(body)
			return writeErr
		}
		return err
	}
	if err := validateRegularFileMode(pre.Mode(), name); err != nil {
		return err
	}
	file, err := root.OpenFile(name, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	post, err := file.Stat()
	if err != nil {
		return err
	}
	if err := validateRegularFileMode(post.Mode(), name); err != nil {
		return err
	}
	if !os.SameFile(pre, post) {
		return fmt.Errorf("file identity changed while opening %s", name)
	}
	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}
	if len(body) == 0 {
		return nil
	}
	_, err = file.Write(body)
	return err
}

func appendRootedRegularFile(root *os.Root, name string, body []byte) error {
	pre, err := root.Lstat(name)
	switch {
	case err == nil:
		if err := validateRegularFileMode(pre.Mode(), name); err != nil {
			return err
		}
	case errors.Is(err, fs.ErrNotExist):
		file, createErr := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if createErr != nil {
			if !errors.Is(createErr, fs.ErrExist) {
				return createErr
			}
			return appendRootedRegularFile(root, name, body)
		}
		defer file.Close()
		post, statErr := file.Stat()
		if statErr != nil {
			return statErr
		}
		if err := validateRegularFileMode(post.Mode(), name); err != nil {
			return err
		}
		if len(body) == 0 {
			return nil
		}
		_, writeErr := file.Write(body)
		return writeErr
	default:
		return err
	}
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	post, err := file.Stat()
	if err != nil {
		return err
	}
	if err := validateRegularFileMode(post.Mode(), name); err != nil {
		return err
	}
	if !os.SameFile(pre, post) {
		return fmt.Errorf("file identity changed while opening %s", name)
	}
	if len(body) == 0 {
		return nil
	}
	_, err = file.Write(body)
	return err
}

func clearRootedRegularFile(root *os.Root, name string) error {
	pre, err := root.Lstat(name)
	switch {
	case err == nil:
		if err := validateRegularFileMode(pre.Mode(), name); err != nil {
			return err
		}
		return rewriteRootedRegularFile(root, name, nil)
	case errors.Is(err, fs.ErrNotExist):
		file, createErr := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if createErr != nil {
			if !errors.Is(createErr, fs.ErrExist) {
				return createErr
			}
			return clearRootedRegularFile(root, name)
		}
		return file.Close()
	default:
		return err
	}
}

type worktreeListEntry struct {
	path     string
	head     string
	detached bool
}

func (r *CandidateRoot) validateReopenedCandidateAuthority(expectedCommit string) error {
	if err := r.ensureAvailable(); err != nil {
		return err
	}
	if expectedCommit != r.commit {
		return fmt.Errorf("reference commit drifted while reopening candidate root")
	}
	gitfile, err := readRootedRegularFile(r.repo.root, path.Join(r.worktreeRel, ".git"))
	if err != nil {
		return err
	}
	gitdirPath, err := parseWorktreeGitfile(r.worktree.path, string(gitfile))
	if err != nil {
		return err
	}
	gitdirAbs, err := filepath.Abs(gitdirPath)
	if err != nil {
		return err
	}
	gitdirResolved, err := filepath.EvalSymlinks(gitdirAbs)
	if err != nil {
		return err
	}
	worktreesResolved, err := filepath.EvalSymlinks(filepath.Join(r.repoRoot, ".git", "worktrees"))
	if err != nil {
		return err
	}
	if !strings.HasPrefix(filepath.Clean(gitdirResolved)+string(os.PathSeparator), filepath.Clean(worktreesResolved)+string(os.PathSeparator)) {
		return fmt.Errorf("candidate worktree gitdir is not in the repository worktree registry")
	}
	commonRaw, stderr, err := runGitWithIdentity(r.worktree, nil, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return fmt.Errorf("resolve candidate worktree common gitdir: %v: %s", err, strings.TrimSpace(stderr))
	}
	commonAbs := strings.TrimSpace(string(commonRaw))
	wantCommon := filepath.Join(r.repoRoot, ".git")
	commonInfo, err := os.Stat(commonAbs)
	if err != nil {
		return err
	}
	wantCommonInfo, err := os.Stat(wantCommon)
	if err != nil {
		return err
	}
	if !os.SameFile(commonInfo, wantCommonInfo) {
		return fmt.Errorf("candidate worktree common gitdir does not match repository")
	}
	gitDirRaw, stderr, err := runGitWithIdentity(r.worktree, nil, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		return fmt.Errorf("resolve candidate worktree gitdir: %v: %s", err, strings.TrimSpace(stderr))
	}
	if filepath.Clean(strings.TrimSpace(string(gitDirRaw))) != filepath.Clean(gitdirAbs) {
		return fmt.Errorf("candidate worktree gitdir does not match .git indirection")
	}
	if _, stderr, err := runGitWithIdentity(r.worktree, nil, "symbolic-ref", "-q", "HEAD"); err == nil {
		return fmt.Errorf("candidate worktree is not detached")
	} else if strings.TrimSpace(stderr) != "" {
		return fmt.Errorf("inspect candidate detached HEAD: %v: %s", err, strings.TrimSpace(stderr))
	}
	headRaw, stderr, err := runGitWithIdentity(r.worktree, nil, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("resolve candidate worktree HEAD: %v: %s", err, strings.TrimSpace(stderr))
	}
	if strings.TrimSpace(string(headRaw)) != expectedCommit {
		return fmt.Errorf("candidate worktree HEAD does not match expected commit")
	}
	entries, err := r.loadRegisteredWorktrees()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		entryInfo, err := os.Stat(entry.path)
		if err != nil {
			continue
		}
		if os.SameFile(entryInfo, r.worktree.identity) {
			if entry.head != expectedCommit {
				return fmt.Errorf("registered candidate worktree commit drifted")
			}
			if !entry.detached {
				return fmt.Errorf("registered candidate worktree is not detached")
			}
			return nil
		}
	}
	return fmt.Errorf("candidate worktree is not registered with git")
}

func (r *CandidateRoot) loadRegisteredWorktrees() ([]worktreeListEntry, error) {
	raw, stderr, err := runGitWithIdentity(r.repo, nil, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("read worktree registry: %v: %s", err, strings.TrimSpace(stderr))
	}
	blocks := strings.Split(strings.TrimSpace(string(raw)), "\n\n")
	entries := make([]worktreeListEntry, 0, len(blocks))
	for _, block := range blocks {
		if strings.TrimSpace(block) == "" {
			continue
		}
		entry := worktreeListEntry{}
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "worktree "):
				entry.path = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			case strings.HasPrefix(line, "HEAD "):
				entry.head = strings.TrimSpace(strings.TrimPrefix(line, "HEAD "))
			case strings.TrimSpace(line) == "detached":
				entry.detached = true
			}
		}
		if entry.path != "" {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func parseWorktreeGitfile(worktreePath, body string) (string, error) {
	trimmed := strings.TrimSpace(body)
	if !strings.HasPrefix(trimmed, "gitdir: ") {
		return "", fmt.Errorf("worktree .git file is malformed")
	}
	gitdir := strings.TrimSpace(strings.TrimPrefix(trimmed, "gitdir: "))
	if gitdir == "" {
		return "", fmt.Errorf("worktree .git file is malformed")
	}
	if filepath.IsAbs(gitdir) {
		return gitdir, nil
	}
	return filepath.Join(worktreePath, gitdir), nil
}
