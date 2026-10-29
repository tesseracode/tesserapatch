package candidatelock

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Handle is one held advisory lock.
type Handle struct {
	file *os.File
	path string
}

// Path returns the canonical lock path.
func (h *Handle) Path() string {
	if h == nil {
		return ""
	}
	return h.path
}

// Release closes the underlying descriptor/handle.
func (h *Handle) Release() error {
	if h == nil || h.file == nil {
		return nil
	}
	file := h.file
	h.file = nil
	return file.Close()
}

func canonicalLockPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("lock path is required")
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve lock path: %w", err)
	}
	abs = filepath.Clean(abs)
	if !filepath.IsAbs(abs) {
		return "", fmt.Errorf("lock path must be absolute: %s", abs)
	}
	return abs, nil
}

func nearestExistingAncestor(path string) (string, error) {
	current := filepath.Clean(path)
	for {
		info, err := os.Lstat(current)
		if err == nil {
			_ = info
			return current, nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect lock path ancestor %s: %w", current, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no existing lock path ancestor for %s", path)
		}
		current = parent
	}
}
