package gitutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var treeHashPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type rootedIdentity struct {
	path string
	info os.FileInfo
}

// TreeHash resolves the tree object ID for ref without touching the index or
// working tree, and with lazy network fetches disabled.
func TreeHash(repoRoot, ref string) (string, error) {
	if strings.TrimSpace(ref) == "" {
		return "", fmt.Errorf("tree hash requires a non-empty ref")
	}
	identity, err := captureRootIdentity(repoRoot)
	if err != nil {
		return "", err
	}
	out, stderr, err := runTreeGit(identity, nil, "rev-parse", "--verify", ref+"^{tree}")
	if err != nil {
		return "", fmt.Errorf("git rev-parse --verify %s^{tree}: %v: %s", ref, err, strings.TrimSpace(stderr))
	}
	hash := strings.TrimSpace(out)
	if !treeHashPattern.MatchString(hash) {
		return "", fmt.Errorf("git returned a non-canonical tree hash %q for %s", hash, ref)
	}
	return hash, nil
}

// WorktreeTreeHash builds the current worktree tree hash (including
// uncommitted edits) through an isolated temporary index.
func WorktreeTreeHash(repoRoot string) (string, error) {
	identity, err := captureRootIdentity(repoRoot)
	if err != nil {
		return "", err
	}
	indexPath, stderr, err := runTreeGit(identity, nil, "rev-parse", "--git-path", "index")
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-path index: %v: %s", err, strings.TrimSpace(stderr))
	}
	indexPath = strings.TrimSpace(indexPath)
	if indexPath == "" {
		return "", fmt.Errorf("git rev-parse --git-path index returned an empty path")
	}
	if !filepath.IsAbs(indexPath) {
		indexPath = filepath.Join(identity.path, filepath.FromSlash(indexPath))
	}
	temp, err := os.CreateTemp(filepath.Dir(indexPath), ".tpatch-tree-index-*")
	if err != nil {
		return "", fmt.Errorf("create temp index file: %w", err)
	}
	tempPath := temp.Name()
	if closeErr := temp.Close(); closeErr != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("close temp index file: %w", closeErr)
	}
	if err := os.Remove(tempPath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("prepare temp index path: %w", err)
	}
	defer func() {
		_ = os.Remove(tempPath)
		_ = os.Remove(tempPath + ".lock")
	}()

	env := []string{"GIT_INDEX_FILE=" + tempPath}
	if _, stderr, err := runTreeGit(identity, env, "read-tree", "HEAD"); err != nil {
		return "", fmt.Errorf("git read-tree HEAD: %v: %s", err, strings.TrimSpace(stderr))
	}
	if _, stderr, err := runTreeGit(identity, env, "add", "-A", "--", "."); err != nil {
		return "", fmt.Errorf("git add -A -- .: %v: %s", err, strings.TrimSpace(stderr))
	}
	out, stderr, err := runTreeGit(identity, env, "write-tree")
	if err != nil {
		return "", fmt.Errorf("git write-tree: %v: %s", err, strings.TrimSpace(stderr))
	}
	hash := strings.TrimSpace(out)
	if !treeHashPattern.MatchString(hash) {
		return "", fmt.Errorf("git write-tree returned a non-canonical tree hash %q", hash)
	}
	return hash, nil
}

func captureRootIdentity(repoRoot string) (rootedIdentity, error) {
	if strings.TrimSpace(repoRoot) == "" {
		return rootedIdentity{}, fmt.Errorf("tree hash requires a non-empty repo root")
	}
	abs, err := filepath.Abs(repoRoot)
	if err != nil {
		return rootedIdentity{}, fmt.Errorf("resolve repo root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return rootedIdentity{}, fmt.Errorf("stat repo root: %w", err)
	}
	if !info.IsDir() {
		return rootedIdentity{}, fmt.Errorf("repo root is not a directory: %s", abs)
	}
	return rootedIdentity{path: abs, info: info}, nil
}

func (identity rootedIdentity) validate() error {
	current, err := os.Stat(identity.path)
	if err != nil {
		return fmt.Errorf("stat repo root identity path: %w", err)
	}
	if !os.SameFile(identity.info, current) {
		return fmt.Errorf("repo root identity changed while hashing tree")
	}
	return nil
}

func runTreeGit(identity rootedIdentity, extraEnv []string, args ...string) (string, string, error) {
	if err := identity.validate(); err != nil {
		return "", "", err
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = identity.path
	cmd.Env = treeHashEnv(extraEnv)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if checkErr := identity.validate(); checkErr != nil {
		if err != nil {
			return stdout.String(), stderr.String(), fmt.Errorf("repo root identity changed while running git %s: %w", strings.Join(args, " "), checkErr)
		}
		return stdout.String(), stderr.String(), checkErr
	}
	return stdout.String(), stderr.String(), err
}

func treeHashEnv(extra []string) []string {
	env := prepareGitEnvironment(os.Environ())
	env = append(env, extra...)
	env = append(env, NoLazyFetchEnv, CLocaleEnv)
	return env
}
