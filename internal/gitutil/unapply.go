package gitutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/safety"
)

// PathsAffectedByPatch returns the union of both diff-header sides plus
// rename/copy source and destination paths. Unapply needs the full set: a
// reverse rename recreates the a-side path and removes the b-side path.
func PathsAffectedByPatch(patch string) []string {
	seen := map[string]struct{}{}
	var paths []string
	add := func(raw string, stripDiffPrefix bool) {
		raw = strings.TrimSpace(raw)
		if raw == "" || raw == "/dev/null" {
			return
		}
		if tab := strings.IndexByte(raw, '\t'); tab >= 0 {
			raw = raw[:tab]
		}
		if unquoted, err := strconv.Unquote(raw); err == nil {
			raw = unquoted
		}
		if stripDiffPrefix {
			switch {
			case strings.HasPrefix(raw, "a/"):
				raw = strings.TrimPrefix(raw, "a/")
			case strings.HasPrefix(raw, "b/"):
				raw = strings.TrimPrefix(raw, "b/")
			}
		}
		if raw == "" {
			return
		}
		raw = filepath.ToSlash(raw)
		if _, ok := seen[raw]; ok {
			return
		}
		seen[raw] = struct{}{}
		paths = append(paths, raw)
	}

	inHeader := false
	for _, line := range strings.Split(patch, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			inHeader = true
			fields := pathsFromDiffGitHeader(strings.TrimPrefix(line, "diff --git "))
			for _, field := range fields {
				add(field, true)
			}
		case strings.HasPrefix(line, "@@"):
			inHeader = false
		case inHeader && strings.HasPrefix(line, "--- "):
			add(strings.TrimPrefix(line, "--- "), true)
		case inHeader && strings.HasPrefix(line, "+++ "):
			add(strings.TrimPrefix(line, "+++ "), true)
		case inHeader && strings.HasPrefix(line, "rename from "):
			add(strings.TrimPrefix(line, "rename from "), false)
		case inHeader && strings.HasPrefix(line, "rename to "):
			add(strings.TrimPrefix(line, "rename to "), false)
		case inHeader && strings.HasPrefix(line, "copy from "):
			add(strings.TrimPrefix(line, "copy from "), false)
		case inHeader && strings.HasPrefix(line, "copy to "):
			add(strings.TrimPrefix(line, "copy to "), false)
		}
	}
	return paths
}

func pathsFromDiffGitHeader(input string) []string {
	if strings.HasPrefix(input, `"`) {
		fields := splitGitDiffPaths(input)
		if len(fields) == 2 {
			return fields
		}
		return nil
	}

	// Unquoted paths may contain spaces. For the fallback cases that lack
	// ---/+++ headers (binary or mode-only changes), select the delimiter
	// whose a/ and b/ payloads are byte-identical. Renames and copies are
	// covered unambiguously by their dedicated from/to headers.
	for offset := 0; offset < len(input); {
		rel := strings.Index(input[offset:], " b/")
		if rel < 0 {
			break
		}
		at := offset + rel
		left, right := input[:at], input[at+1:]
		if strings.HasPrefix(left, "a/") && strings.HasPrefix(right, "b/") &&
			strings.TrimPrefix(left, "a/") == strings.TrimPrefix(right, "b/") {
			return []string{left, right}
		}
		offset = at + len(" b/")
	}
	return nil
}

func splitGitDiffPaths(input string) []string {
	var fields []string
	for i := 0; i < len(input); {
		for i < len(input) && input[i] == ' ' {
			i++
		}
		if i >= len(input) {
			break
		}
		start := i
		if input[i] == '"' {
			i++
			escaped := false
			for i < len(input) {
				switch {
				case escaped:
					escaped = false
				case input[i] == '\\':
					escaped = true
				case input[i] == '"':
					i++
					fields = append(fields, input[start:i])
					goto next
				}
				i++
			}
			fields = append(fields, input[start:])
			break
		}
		for i < len(input) && input[i] != ' ' {
			i++
		}
		fields = append(fields, input[start:i])
	next:
	}
	return fields
}

// ReverseApply applies patch strictly in reverse. It never falls back to a
// three-way merge because unapply must refuse rather than merge unrelated
// working-tree edits.
func ReverseApply(repoRoot, patch string) error {
	if patch == "" {
		return fmt.Errorf("empty patch")
	}
	cmd := exec.Command("git", "apply", "--reverse", "-")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(patch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git apply --reverse failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// PreviewReverseApply proves that a strict reverse apply succeeds in a
// temporary detached worktree at the current HEAD without mutating repoRoot.
func PreviewReverseApply(repoRoot, patch string) error {
	if patch == "" {
		return fmt.Errorf("empty patch")
	}
	wt, cleanup, err := mkPreviewWorktree(repoRoot)
	if err != nil {
		return fmt.Errorf("reverse-apply preview: %w", err)
	}
	defer cleanup()

	if err := ValidatePatchReverse(wt, patch); err != nil {
		return fmt.Errorf("reverse-apply preview check: %w", err)
	}
	if err := ReverseApply(wt, patch); err != nil {
		return fmt.Errorf("reverse-apply preview: %w", err)
	}
	return nil
}

// ReverseApplyCheckAtHEAD reports whether patch is present in the committed
// HEAD tree, independent of working-tree changes.
func ReverseApplyCheckAtHEAD(repoRoot, patch string) (bool, error) {
	if patch == "" {
		return false, fmt.Errorf("empty patch")
	}
	wt, cleanup, err := mkPreviewWorktree(repoRoot)
	if err != nil {
		return false, fmt.Errorf("HEAD reverse-apply check: %w", err)
	}
	defer cleanup()
	cmd := exec.Command("git", "apply", "--reverse", "--check", "-")
	cmd.Dir = wt
	cmd.Stdin = strings.NewReader(patch)
	if err := cmd.Run(); err == nil {
		return true, nil
	} else if _, ok := err.(*exec.ExitError); ok {
		return false, nil
	} else {
		return false, fmt.Errorf("HEAD reverse-apply check: %w", err)
	}
}

// GitOperationInProgress reports an in-flight merge, rebase, or cherry-pick.
// The returned operation is empty when the repository is idle.
func GitOperationInProgress(repoRoot string) (string, error) {
	gitDir, err := runGit(repoRoot, "rev-parse", "--git-dir")
	if err != nil {
		return "", fmt.Errorf("resolve git directory: %w", err)
	}
	gitDir = strings.TrimSpace(gitDir)
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repoRoot, gitDir)
	}

	checks := []struct {
		operation string
		path      string
	}{
		{operation: "merge", path: "MERGE_HEAD"},
		{operation: "rebase", path: "REBASE_HEAD"},
		{operation: "rebase", path: "rebase-merge"},
		{operation: "rebase", path: "rebase-apply"},
		{operation: "cherry-pick", path: "CHERRY_PICK_HEAD"},
	}
	for _, check := range checks {
		if _, err := os.Stat(filepath.Join(gitDir, check.path)); err == nil {
			return check.operation, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect git %s state: %w", check.operation, err)
		}
	}
	return "", nil
}

// WorktreeFileSnapshot records one touched path before unapply mutation.
type WorktreeFileSnapshot struct {
	Path       string
	Exists     bool
	Mode       os.FileMode
	Data       []byte
	LinkTarget string
}

// WorktreeSnapshot records every path touched by the canonical patch so a
// failed unapply can restore regular files, symlinks, modes, and absence.
type WorktreeSnapshot struct {
	Files []WorktreeFileSnapshot
}

// ValidateWorktreePaths checks that every path is safe to inspect or mutate
// without reading file contents.
func ValidateWorktreePaths(repoRoot string, paths []string) error {
	for _, rel := range paths {
		if _, _, err := safeWorktreePath(repoRoot, rel); err != nil {
			return err
		}
	}
	return nil
}

// SnapshotWorktreePaths snapshots the requested repo-relative paths in sorted
// order. Paths outside repoRoot and paths traversing a symlinked parent outside
// repoRoot are rejected before any data is read.
func SnapshotWorktreePaths(repoRoot string, paths []string) (WorktreeSnapshot, error) {
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	snapshot := WorktreeSnapshot{Files: make([]WorktreeFileSnapshot, 0, len(sorted))}
	for _, rel := range sorted {
		abs, clean, err := safeWorktreePath(repoRoot, rel)
		if err != nil {
			return WorktreeSnapshot{}, err
		}
		entry := WorktreeFileSnapshot{Path: clean}
		info, err := os.Lstat(abs)
		if errors.Is(err, os.ErrNotExist) {
			snapshot.Files = append(snapshot.Files, entry)
			continue
		}
		if err != nil {
			return WorktreeSnapshot{}, fmt.Errorf("snapshot %s: %w", clean, err)
		}
		entry.Exists = true
		entry.Mode = info.Mode()
		switch {
		case info.Mode().IsRegular():
			entry.Data, err = os.ReadFile(abs)
		case info.Mode()&os.ModeSymlink != 0:
			entry.LinkTarget, err = os.Readlink(abs)
		default:
			err = fmt.Errorf("unsupported file mode %s", info.Mode())
		}
		if err != nil {
			return WorktreeSnapshot{}, fmt.Errorf("snapshot %s: %w", clean, err)
		}
		snapshot.Files = append(snapshot.Files, entry)
	}
	return snapshot, nil
}

// Restore restores every snapshot entry. It attempts all paths and returns the
// joined errors so one failed path does not prevent restoration of its peers.
func (s WorktreeSnapshot) Restore(repoRoot string) error {
	var errs []error
	for _, entry := range s.Files {
		if err := restoreWorktreeFile(repoRoot, entry); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func restoreWorktreeFile(repoRoot string, entry WorktreeFileSnapshot) error {
	abs, clean, err := safeWorktreePath(repoRoot, entry.Path)
	if err != nil {
		return err
	}
	if !entry.Exists {
		info, err := os.Lstat(abs)
		switch {
		case errors.Is(err, os.ErrNotExist):
			return nil
		case err != nil:
			return fmt.Errorf("restore absence for %s: %w", clean, err)
		case info.IsDir():
			return fmt.Errorf("restore absence for %s: refusing to remove directory", clean)
		default:
			if err := os.Remove(abs); err != nil {
				return fmt.Errorf("restore absence for %s: %w", clean, err)
			}
			return nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("restore %s parent: %w", clean, err)
	}
	if current, err := os.Lstat(abs); err == nil {
		if current.IsDir() {
			return fmt.Errorf("restore %s: refusing to replace directory", clean)
		}
		if err := os.Remove(abs); err != nil {
			return fmt.Errorf("restore %s: %w", clean, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("restore %s: %w", clean, err)
	}

	switch {
	case entry.Mode.IsRegular():
		if err := writeSnapshotFile(abs, entry.Data, entry.Mode.Perm()); err != nil {
			return fmt.Errorf("restore %s: %w", clean, err)
		}
	case entry.Mode&os.ModeSymlink != 0:
		if err := os.Symlink(entry.LinkTarget, abs); err != nil {
			return fmt.Errorf("restore symlink %s: %w", clean, err)
		}
	default:
		return fmt.Errorf("restore %s: unsupported file mode %s", clean, entry.Mode)
	}
	return nil
}

func writeSnapshotFile(path string, data []byte, mode os.FileMode) (retErr error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".restore-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		if retErr != nil {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func safeWorktreePath(repoRoot, rel string) (abs, clean string, err error) {
	if filepath.IsAbs(rel) {
		return "", "", fmt.Errorf("path %q is absolute", rel)
	}
	clean = filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("path %q escapes the repository root", rel)
	}
	abs = filepath.Join(repoRoot, clean)
	if err := safety.EnsureSafeRepoPath(repoRoot, abs); err != nil {
		return "", "", err
	}

	resolvedRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolve repository root: %w", err)
	}
	probe := filepath.Dir(abs)
	for {
		resolved, resolveErr := filepath.EvalSymlinks(probe)
		if resolveErr == nil {
			if err := safety.EnsureSafeRepoPath(resolvedRoot, resolved); err != nil {
				return "", "", fmt.Errorf("path %q traverses outside the repository root: %w", rel, err)
			}
			break
		}
		if !errors.Is(resolveErr, os.ErrNotExist) {
			return "", "", fmt.Errorf("resolve parent for %q: %w", rel, resolveErr)
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return "", "", fmt.Errorf("resolve parent for %q: no existing ancestor", rel)
		}
		probe = parent
	}
	return abs, filepath.ToSlash(clean), nil
}
