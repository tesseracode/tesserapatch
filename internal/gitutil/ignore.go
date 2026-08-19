package gitutil

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// ErrGitUnavailable is returned by session-critical compatibility helpers
// when git cannot be located or the selected directory is not a worktree.
var ErrGitUnavailable = errors.New("git unavailable or not a working tree")

// GitState is the single G1 result threaded through prepare's remaining Git
// checks. Helpers that consume it never rediscover the repository.
type GitState string

const (
	GitWorktree     GitState = "worktree"
	GitNonWorktree  GitState = "non-worktree"
	GitUnverifiable GitState = "unverifiable"
)

type gitProcessRequest struct {
	repoRoot      string
	args          []string
	env           []string
	captureStdout bool
	captureStderr bool
}

type gitProcessResult struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

var runGitProcess = defaultRunGitProcess

func defaultRunGitProcess(request gitProcessRequest) gitProcessResult {
	cmd := exec.Command("git", request.args...)
	cmd.Dir = request.repoRoot
	if request.env != nil {
		cmd.Env = request.env
	}
	var stdout, stderr bytes.Buffer
	if request.captureStdout {
		cmd.Stdout = &stdout
	}
	if request.captureStderr {
		cmd.Stderr = &stderr
	}
	err := cmd.Run()
	result := gitProcessResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
		err:    err,
	}
	if err == nil {
		return result
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.exitCode = exitErr.ExitCode()
	} else {
		result.exitCode = -1
	}
	return result
}

// DiscoverGitState performs prepare's G1 exactly once.
func DiscoverGitState(repoRoot string) (GitState, error) {
	result := runGitProcess(gitProcessRequest{
		repoRoot:      repoRoot,
		args:          []string{"rev-parse", "--is-inside-work-tree"},
		env:           prepareGitEnvironment(os.Environ()),
		captureStdout: true,
		captureStderr: true,
	})
	output := strings.TrimSpace(result.stdout)
	if result.err == nil {
		switch output {
		case "true":
			return GitWorktree, nil
		case "false":
			return GitNonWorktree, nil
		default:
			return GitUnverifiable, fmt.Errorf("git rev-parse returned unexpected output")
		}
	}
	if knownNonWorktree(result.exitCode, result.stderr) {
		return GitNonWorktree, nil
	}
	return GitUnverifiable, fmt.Errorf("git rev-parse could not classify the workspace")
}

// IsIgnoredWithState performs prepare's G2 against a repo-relative path.
func IsIgnoredWithState(repoRoot string, state GitState, repoRelative string) (bool, error) {
	if state != GitWorktree {
		return false, fmt.Errorf("git check-ignore requires a worktree state")
	}
	if !validRepoRelative(repoRelative) {
		return false, fmt.Errorf("git check-ignore requires a repo-relative path")
	}
	result := runGitProcess(gitProcessRequest{
		repoRoot:      repoRoot,
		args:          []string{"check-ignore", "-q", "--no-index", "--", disarmLeadingColon(repoRelative)},
		env:           prepareGitEnvironment(os.Environ()),
		captureStderr: true,
	})
	switch {
	case result.err == nil:
		return true, nil
	case result.exitCode == 1:
		return false, nil
	default:
		return false, fmt.Errorf("git check-ignore could not verify the local lane")
	}
}

// AnythingTrackedUnderWithState performs prepare's G3.
func AnythingTrackedUnderWithState(repoRoot string, state GitState) (bool, error) {
	if state != GitWorktree {
		return false, fmt.Errorf("git ls-files requires a worktree state")
	}
	result := runGitProcess(gitProcessRequest{
		repoRoot:      repoRoot,
		args:          []string{"--literal-pathspecs", "ls-files", "--", ".tpatch/local/"},
		env:           prepareGitEnvironment(os.Environ()),
		captureStdout: true,
		captureStderr: true,
	})
	if result.err != nil {
		return false, fmt.Errorf("git ls-files could not verify the local lane")
	}
	return strings.TrimSpace(result.stdout) != "", nil
}

// IsTpatchTrackedWithState performs prepare's G4.
func IsTpatchTrackedWithState(repoRoot string, state GitState) (bool, error) {
	if state != GitWorktree {
		return false, fmt.Errorf("git ls-files requires a worktree state")
	}
	result := runGitProcess(gitProcessRequest{
		repoRoot:      repoRoot,
		args:          []string{"ls-files", "--", ".tpatch"},
		env:           prepareGitEnvironment(os.Environ()),
		captureStdout: true,
		captureStderr: true,
	})
	if result.err != nil {
		return false, fmt.Errorf("git ls-files could not inspect .tpatch")
	}
	return strings.TrimSpace(result.stdout) != "", nil
}

func knownNonWorktree(exitCode int, stderr string) bool {
	if exitCode != 128 {
		return false
	}
	text := strings.TrimSpace(stderr)
	return strings.HasPrefix(text, "fatal: not a git repository") ||
		strings.HasPrefix(text, "fatal: not a git work tree")
}

func validRepoRelative(value string) bool {
	return value != "" && value != "." && fs.ValidPath(value)
}

func disarmLeadingColon(value string) string {
	if strings.HasPrefix(value, ":") {
		return "./" + value
	}
	return value
}

func prepareGitEnvironment(source []string) []string {
	env := make([]string, 0, len(source)+2)
	for _, entry := range source {
		name, _, ok := strings.Cut(entry, "=")
		if !ok || scrubPrepareGitVariable(name) || name == "LC_ALL" || name == "LANG" {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, "LC_ALL=C", "LANG=C")
	return env
}

func scrubPrepareGitVariable(name string) bool {
	switch name {
	case "GIT_DIR",
		"GIT_WORK_TREE",
		"GIT_INDEX_FILE",
		"GIT_COMMON_DIR",
		"GIT_CEILING_DIRECTORIES",
		"GIT_OBJECT_DIRECTORY",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES",
		"GIT_DISCOVERY_ACROSS_FILESYSTEM",
		"GIT_PREFIX",
		"GIT_IMPLICIT_WORK_TREE",
		"GIT_SUPER_PREFIX",
		"GIT_CONFIG_COUNT":
		return true
	}
	for _, prefix := range []string{"GIT_CONFIG_KEY_", "GIT_CONFIG_VALUE_"} {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		_, err := strconv.ParseUint(strings.TrimPrefix(name, prefix), 10, 64)
		return err == nil
	}
	return false
}

// IsGitAvailable preserves the pre-S4 session helper behavior.
func IsGitAvailable(repoRoot string) bool {
	return CompatibilityIsGitAvailable(repoRoot)
}

// CompatibilityIsGitAvailable is the explicit legacy-policy wrapper.
func CompatibilityIsGitAvailable(repoRoot string) bool {
	if _, err := exec.LookPath("git"); err != nil {
		return false
	}
	result := runGitProcess(gitProcessRequest{
		repoRoot:      repoRoot,
		args:          []string{"rev-parse", "--is-inside-work-tree"},
		captureStdout: true,
	})
	return result.err == nil && strings.TrimSpace(result.stdout) == "true"
}

// IsPathIgnored preserves the pre-S4 compatibility contract, including its
// inherited environment and absolute-path acceptance.
func IsPathIgnored(repoRoot, path string) (bool, error) {
	return CompatibilityIsPathIgnored(repoRoot, path)
}

// CompatibilityIsPathIgnored is the explicit legacy-policy wrapper.
func CompatibilityIsPathIgnored(repoRoot, path string) (bool, error) {
	if !CompatibilityIsGitAvailable(repoRoot) {
		return false, ErrGitUnavailable
	}
	result := runGitProcess(gitProcessRequest{
		repoRoot: repoRoot,
		args:     []string{"check-ignore", "-q", "--no-index", "--", path},
	})
	if result.err == nil {
		return true, nil
	}
	if result.exitCode == 1 {
		return false, nil
	}
	if result.exitCode < 0 {
		return false, result.err
	}
	return false, fmt.Errorf("git check-ignore %s: %s", path, strings.TrimSpace(result.stderr))
}

// RunGitCompatibility is the explicit compatibility wrapper used by rescap.
// It preserves the caller's inherited environment and captures both streams.
func RunGitCompatibility(repoRoot string, args ...string) (stdout, stderr string, exitCode int, err error) {
	result := runGitProcess(gitProcessRequest{
		repoRoot:      repoRoot,
		args:          append([]string(nil), args...),
		captureStdout: true,
		captureStderr: true,
	})
	return result.stdout, result.stderr, result.exitCode, result.err
}
