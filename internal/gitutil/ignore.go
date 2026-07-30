package gitutil

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrGitUnavailable is returned by session-critical helpers when the
// git executable cannot be located OR the given directory is not a
// working tree. Callers (PRD-active-feature-session §4 D6 mandate 4)
// treat this as refusal-class.
var ErrGitUnavailable = errors.New("git unavailable or not a working tree")

// IsGitAvailable reports whether the given directory is inside a git
// working tree AND git is executable on PATH. Used by the D6 mandate
// 4 refusal contract (`writers must refuse when Git is unavailable`).
//
// The check is TWO parts:
//  1. `git --version` succeeds (git installed).
//  2. `git rev-parse --is-inside-work-tree` prints "true" (dir is a
//     working tree). This catches the "user unpacked a tarball" and
//     ".tpatch/ exists outside git" cases.
func IsGitAvailable(repoRoot string) bool {
	if _, err := exec.LookPath("git"); err != nil {
		return false
	}
	out, err := runGit(repoRoot, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "true"
}

// IsPathIgnored reports whether `path` (absolute or repo-relative) is
// effectively ignored by git. Uses `git check-ignore -q` semantics per
// PRD-active-feature-session §4 D6 mandate 5 ("Verification must check
// effective Git ignore behavior, not just a textual line").
//
// Exit codes from `git check-ignore -q <path>`:
//   - 0: path is ignored.
//   - 1: path is NOT ignored (no matching rule).
//   - 128: fatal error (not a git repo, invalid path, …). Surface as
//     err != nil so callers do not silently treat "git broke" as
//     "path not ignored".
//
// `--no-index` is passed to check-ignore so the answer does not depend
// on whether the path has been added to the index. `check-ignore` on
// paths BELOW `.tpatch/local/` correctly reports ignored when the
// enclosing `.tpatch/local/` line matches, so callers can pass either
// the concrete session dir or a parent.
//
// Errors:
//   - Returns (false, ErrGitUnavailable) if git is missing or not a work tree.
//   - Returns (false, nil) when git reports "not ignored" (exit 1).
//   - Returns (false, err) with the git stderr wrapped for any other
//     exit class (128, 129, …).
func IsPathIgnored(repoRoot, path string) (bool, error) {
	if !IsGitAvailable(repoRoot) {
		return false, ErrGitUnavailable
	}
	cmd := exec.Command("git", "check-ignore", "-q", "--no-index", "--", path)
	cmd.Dir = repoRoot
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		switch exitErr.ExitCode() {
		case 1:
			return false, nil
		default:
			return false, fmt.Errorf("git check-ignore %s: %s", path, strings.TrimSpace(string(exitErr.Stderr)))
		}
	}
	return false, err
}
