// Package safety provides path validation and input sanitization.
package safety

import (
	"fmt"
	"path/filepath"
	"strings"
)

// EnsureSafeRepoPath validates that resolvedPath is inside repoRoot.
// Prevents path traversal attacks (e.g., "../../../etc/passwd").
func EnsureSafeRepoPath(repoRoot, resolvedPath string) error {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return fmt.Errorf("cannot resolve repo root: %w", err)
	}
	absPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		return fmt.Errorf("cannot resolve path: %w", err)
	}

	// Ensure the path starts with the repo root
	rootWithSep := absRoot + string(filepath.Separator)
	if absPath != absRoot && !strings.HasPrefix(absPath, rootWithSep) {
		return fmt.Errorf("path %q is outside the repository root %q", resolvedPath, repoRoot)
	}
	return nil
}
