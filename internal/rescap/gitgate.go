// Git ignore/tracked gate semantics for
// PRD-feature-resource-claims-and-capture-adapters §10.
//
// Two Git questions are asked, with two deliberately different
// invocation shapes:
//
//   - `git check-ignore -q --no-index -- <pathname>` — no
//     --literal-pathspecs, because check-ignore has no such option and
//     fails fatally (exit 128) if one is passed, regardless of whether
//     the argument itself looks like pathspec magic. Because its plain
//     pathname argument still parses a leading `:` for pathspec magic,
//     any selector whose first byte is `:` is passed as `./<selector>`
//     instead. This is the one documented exception to the project's
//     general "always pass literal pathspecs" discipline.
//   - `git --literal-pathspecs ls-files ...` — literal pathspecs are
//     supported here and are always used.
//
// Both gates fail closed: any exit/output shape that is not one of the
// two well-known ones is a fatal Git error, never silently read as
// either answer.

package rescap

import (
	"bytes"
	"os/exec"
	"strings"
)

// ignoreCheckArgument applies §10.1's `./`-prefix rule: any selector
// whose first byte is `:` is passed as `./<selector>`, which disarms
// colon-magic parsing while resolving to the identical path. The rule
// is keyed on "any leading colon", not "any unsupported magic keyword",
// so it does not depend on which magic keywords a given Git version
// happens to support.
func ignoreCheckArgument(selector string) string {
	if strings.HasPrefix(selector, ":") {
		return "./" + selector
	}
	return selector
}

// IsIgnored runs the check-ignore gate. Exit 0 means ignored, exit 1
// means not ignored, and any other exit code is a fatal Git error.
func IsIgnored(repoRoot, selector string) (bool, error) {
	cmd := exec.Command("git", "check-ignore", "-q", "--no-index", "--", ignoreCheckArgument(selector))
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return false, Refuse(ReasonGitIgnoreCheckError,
			"git check-ignore %s could not run: %v", selector, err)
	}
	switch exitErr.ExitCode() {
	case 1:
		return false, nil
	default:
		return false, Refuse(ReasonGitIgnoreCheckError,
			"git check-ignore %s exited %d: %s", selector, exitErr.ExitCode(), strings.TrimSpace(stderr.String()))
	}
}

// lsFilesUnmatchedMessage is the standard stderr shape `ls-files
// --error-unmatch` emits for an untracked path. Any other exit-1
// stderr is treated as a fatal Git error.
const lsFilesUnmatchedMessage = "did not match any file(s) known to git"

// IsTracked runs the `ls-files --error-unmatch` gate. Exit 0 means the
// path is tracked; exit 1 with the standard message means untracked;
// anything else is a fatal Git error.
func IsTracked(repoRoot, selector string) (bool, error) {
	cmd := exec.Command("git", "--literal-pathspecs", "ls-files", "--error-unmatch", "--", selector)
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return false, Refuse(ReasonGitLsFilesError,
			"git ls-files --error-unmatch %s could not run: %v", selector, err)
	}
	text := strings.TrimSpace(stderr.String())
	if exitErr.ExitCode() == 1 && strings.Contains(text, lsFilesUnmatchedMessage) {
		return false, nil
	}
	return false, Refuse(ReasonGitLsFilesError,
		"git ls-files --error-unmatch %s exited %d: %s", selector, exitErr.ExitCode(), text)
}

// AnythingTrackedUnder answers the broader "is anything anywhere under
// this prefix tracked" question with the plain, no-flag `ls-files`
// form and an empty-stdout convention (§10.3 step 2). It deliberately
// does not reuse the --error-unmatch gate, whose tracked/untracked
// distinction is inferred from an exit-code/stderr shape designed for
// a single pathname argument.
func AnythingTrackedUnder(repoRoot, prefix string) (bool, error) {
	cmd := exec.Command("git", "--literal-pathspecs", "ls-files", "--", prefix)
	cmd.Dir = repoRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return false, Refuse(ReasonGitLsFilesError,
			"git ls-files -- %s failed: %v: %s", prefix, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Len() > 0, nil
}

// RunGit runs a git subcommand in repoRoot and returns trimmed stdout.
// Callers that need exit-code-specific behaviour use the dedicated
// gates above instead.
func RunGit(repoRoot string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(stdout.String()), &gitCommandError{
			Args:   args,
			Stderr: strings.TrimSpace(stderr.String()),
			Err:    err,
		}
	}
	return stdout.String(), nil
}

type gitCommandError struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *gitCommandError) Error() string {
	return "git " + strings.Join(e.Args, " ") + ": " + e.Stderr
}

func (e *gitCommandError) Unwrap() error { return e.Err }
