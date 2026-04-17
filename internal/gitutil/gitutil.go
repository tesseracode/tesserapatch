// Package gitutil provides git operations: diff, patch capture, reverse-apply, head commit.
package gitutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// HeadCommit returns the current HEAD commit hash.
func HeadCommit(repoRoot string) (string, error) {
	out, err := runGit(repoRoot, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// CaptureDiffStat returns `git diff --stat` output.
func CaptureDiffStat(repoRoot string) (string, error) {
	out, err := runGit(repoRoot, "diff", "--stat")
	if err != nil {
		return "", err
	}
	return out, nil
}

// CapturePatch captures a unified diff including tracked modifications and untracked new files.
// It excludes .tpatch/, .claude/skills/, .github/skills/, .github/prompts/, .cursor/rules/.
func CapturePatch(repoRoot string) (string, error) {
	excludePatterns := []string{
		":(exclude).tpatch",
		":(exclude).claude/skills",
		":(exclude).github/skills",
		":(exclude).github/prompts",
		":(exclude).cursor/rules",
		":(exclude).windsurfrules",
	}

	skipPrefixes := []string{".tpatch/", ".claude/skills/", ".github/skills/", ".github/prompts/", ".cursor/rules/", ".windsurfrules"}

	// Stage untracked files with --intent-to-add so they appear in git diff
	untrackedFiles, _ := runGit(repoRoot, "ls-files", "--others", "--exclude-standard")
	var stagedNewFiles []string
	for _, file := range strings.Split(strings.TrimSpace(untrackedFiles), "\n") {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		skip := false
		for _, prefix := range skipPrefixes {
			if strings.HasPrefix(file, prefix) || file == strings.TrimSuffix(prefix, "/") {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		// Stage as intent-to-add (makes new files visible to git diff)
		if _, err := runGit(repoRoot, "add", "--intent-to-add", file); err == nil {
			stagedNewFiles = append(stagedNewFiles, file)
		}
	}

	// Capture unified diff (now includes tracked changes AND intent-to-add new files)
	args := append([]string{"diff", "--"}, excludePatterns...)
	patch, err := runGit(repoRoot, args...)
	if err != nil {
		patch = ""
	}

	// Unstage the intent-to-add files to leave the working tree clean
	for _, file := range stagedNewFiles {
		runGit(repoRoot, "reset", "--", file)
	}

	result := strings.TrimSpace(patch)
	if result != "" {
		result += "\n" // git patches must end with a newline
	}
	return result, nil
}

// CapturePatchFromCommits captures the diff between two commits, excluding tpatch artifacts.
func CapturePatchFromCommits(repoRoot, fromRef, toRef string) (string, error) {
	excludePatterns := []string{
		":(exclude).tpatch",
		":(exclude).claude/skills",
		":(exclude).github/skills",
		":(exclude).github/prompts",
		":(exclude).cursor/rules",
		":(exclude).windsurfrules",
	}
	args := append([]string{"diff", fromRef, toRef, "--"}, excludePatterns...)
	out, err := runGit(repoRoot, args...)
	if err != nil {
		return "", err
	}
	result := strings.TrimSpace(out)
	if result != "" {
		result += "\n" // git patches must end with a newline
	}
	return result, nil
}

// ValidatePatch runs `git apply --check` to verify a patch is well-formed and can be applied.
// It checks against the given repoRoot (which should be at the target baseline).
// Returns nil if valid. Tries strict first, then 3-way if strategy is "3way".
func ValidatePatch(repoRoot, patch, strategy string) error {
	if patch == "" {
		return fmt.Errorf("empty patch")
	}
	// Strict check first
	cmd := exec.Command("git", "apply", "--check", "-")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(patch)
	if cmd.Run() == nil {
		return nil
	}
	if strategy == "3way" || strategy == "" {
		cmd = exec.Command("git", "apply", "--3way", "--check", "-")
		cmd.Dir = repoRoot
		cmd.Stdin = strings.NewReader(patch)
		if cmd.Run() == nil {
			return nil
		}
	}
	return fmt.Errorf("patch validation failed: patch cannot be applied cleanly")
}

// ReverseApplyCheck tests if a patch can be reverse-applied (already present in the tree).
func ReverseApplyCheck(repoRoot, patch string) (bool, error) {
	cmd := exec.Command("git", "apply", "--reverse", "--check", "-")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(patch)
	err := cmd.Run()
	return err == nil, nil
}

// ForwardApplyCheck tests if a patch can be applied cleanly.
// Tries strict apply first, then falls back to 3-way merge check.
func ForwardApplyCheck(repoRoot, patch string) (bool, error) {
	// Try strict apply first
	cmd := exec.Command("git", "apply", "--check", "-")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(patch)
	if cmd.Run() == nil {
		return true, nil
	}
	// Fall back to 3-way merge check (handles context mismatches)
	cmd = exec.Command("git", "apply", "--3way", "--check", "-")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(patch)
	return cmd.Run() == nil, nil
}

// ForwardApply applies a patch. Uses 3-way merge if strict apply fails.
func ForwardApply(repoRoot, patch string) error {
	cmd := exec.Command("git", "apply", "-")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(patch)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	// Retry with 3-way merge
	cmd = exec.Command("git", "apply", "--3way", "-")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(patch)
	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git apply failed: %s: %w", string(out), err)
	}
	return nil
}

// FetchUpstream fetches from a remote ref.
func FetchUpstream(repoRoot, remote string) error {
	_, err := runGit(repoRoot, "fetch", remote)
	return err
}

// DiffBetween returns the diff between two refs.
func DiffBetween(repoRoot, fromRef, toRef string) (string, error) {
	return runGit(repoRoot, "diff", fromRef, toRef)
}

// ResolveRef resolves a ref to its commit hash.
func ResolveRef(repoRoot, ref string) (string, error) {
	out, err := runGit(repoRoot, "rev-parse", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// DeriveIncrementalPatch computes the diff that only contains one feature's changes,
// given the cumulative patches for the previous features and the current feature.
// prevCumulativePatch = everything up to (but not including) this feature.
// currentCumulativePatch = everything up to and including this feature.
// Returns only this feature's changes (the delta).
func DeriveIncrementalPatch(repoRoot, baseCommit, prevCumulativePatch, currentCumulativePatch string) (string, error) {
	// Create temp dirs
	tmpDir, err := os.MkdirTemp("", "tpatch-incremental-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	prevDir := filepath.Join(tmpDir, "prev")
	currDir := filepath.Join(tmpDir, "curr")

	// Clone base state into both dirs
	for _, dir := range []string{prevDir, currDir} {
		if _, err := runGit(".", "clone", "--no-checkout", repoRoot, dir); err != nil {
			return "", fmt.Errorf("clone failed: %w", err)
		}
		if _, err := runGit(dir, "checkout", baseCommit); err != nil {
			return "", fmt.Errorf("checkout %s failed: %w", baseCommit, err)
		}
	}

	// Apply previous features' cumulative patch to prevDir
	if prevCumulativePatch != "" {
		cmd := exec.Command("git", "apply", "--3way", "-")
		cmd.Dir = prevDir
		cmd.Stdin = strings.NewReader(prevCumulativePatch)
		cmd.Run() // best-effort
	}

	// Apply current features' cumulative patch to currDir
	if currentCumulativePatch != "" {
		cmd := exec.Command("git", "apply", "--3way", "-")
		cmd.Dir = currDir
		cmd.Stdin = strings.NewReader(currentCumulativePatch)
		cmd.Run() // best-effort
	}

	// Diff the two: this gives only the incremental changes for this feature
	cmd := exec.Command("diff", "-ruN", prevDir, currDir)
	out, _ := cmd.Output()
	result := string(out)

	// Fix paths: replace temp dir paths with relative paths
	result = strings.ReplaceAll(result, prevDir+"/", "a/")
	result = strings.ReplaceAll(result, currDir+"/", "b/")

	trimmed := strings.TrimSpace(result)
	if trimmed != "" {
		trimmed += "\n"
	}
	return trimmed, nil
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(out), fmt.Errorf("git %s: %s", strings.Join(args, " "), string(exitErr.Stderr))
		}
		return string(out), err
	}
	return string(out), nil
}
