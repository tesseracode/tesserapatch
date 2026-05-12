package gitutil

import (
	"fmt"
	"os/exec"
	"strings"
)

// PatchID computes the `git patch-id --stable` hash for a unified-diff
// patch string. Returns the 40-character hex hash on success; an empty
// string + non-nil error on any tooling or parsing failure. Callers MUST
// treat a failure as a "no signal" verdict, never as a no-match (see
// PRD-patch-already-upstream-detector §5.1).
//
// The `--stable` flag yields a hash that is stable across cherry-picks,
// minor whitespace drift, and trailing-newline normalisation — the
// property gbp-pq has relied on for ~18 years.
func PatchID(repoRoot, patch string) (string, error) {
	if strings.TrimSpace(patch) == "" {
		return "", fmt.Errorf("empty patch")
	}
	cmd := exec.Command("git", "patch-id", "--stable")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(patch)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git patch-id --stable: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	// `git patch-id` prints "<patch-id> <commit-id-or-zero>". For
	// stdin input the commit-id slot is the all-zero placeholder
	// (or absent). Take the first whitespace-separated token.
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) == 0 {
		return "", fmt.Errorf("git patch-id --stable: no output (patch may not contain any hunks)")
	}
	return fields[0], nil
}

// CommitPatchID computes the `git patch-id --stable` hash for the diff
// introduced by a single commit. Implemented as `git show --format=
// --no-color <sha> | git patch-id --stable`. Mirrors how `git rebase`
// detects already-applied commits.
func CommitPatchID(repoRoot, sha string) (string, error) {
	if strings.TrimSpace(sha) == "" {
		return "", fmt.Errorf("empty sha")
	}
	showCmd := exec.Command("git", "show", "--format=", "--no-color", sha)
	showCmd.Dir = repoRoot
	showOut, err := showCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git show %s: %s", sha, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	if strings.TrimSpace(string(showOut)) == "" {
		// Empty diff (e.g. a tree-only or empty commit) — no patch-id.
		return "", nil
	}
	pidCmd := exec.Command("git", "patch-id", "--stable")
	pidCmd.Dir = repoRoot
	pidCmd.Stdin = strings.NewReader(string(showOut))
	pidOut, err := pidCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git patch-id --stable for %s: %s", sha, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	fields := strings.Fields(strings.TrimSpace(string(pidOut)))
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], nil
}

// RevListInRange returns the list of commit SHAs in (from, to], excluding
// merge commits — mirroring the PRD §5.2 sweep. Commits are returned in
// the order `git rev-list` emits (newest first). The caller may apply
// a cap before iterating.
//
// `from` may be empty: in that case the caller is responsible for
// short-circuiting (an unbounded walk is out of scope per PRD).
func RevListInRange(repoRoot, from, to string) ([]string, error) {
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
		return nil, fmt.Errorf("rev-list range requires both endpoints (got from=%q to=%q)", from, to)
	}
	out, err := runGit(repoRoot, "rev-list", "--no-merges", from+".."+to)
	if err != nil {
		return nil, err
	}
	var shas []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			shas = append(shas, line)
		}
	}
	return shas, nil
}
