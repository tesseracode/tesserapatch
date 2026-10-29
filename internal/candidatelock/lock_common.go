package candidatelock

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// Handle is one held advisory lock.
type Handle struct {
	mu       sync.Mutex
	file     *os.File
	path     string
	released bool
	borrowed int
}

// Path returns the canonical lock path.
func (h *Handle) Path() string {
	if h == nil {
		return ""
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.path
}

// Release closes the underlying descriptor/handle.
func (h *Handle) Release() error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.file == nil || h.released {
		return nil
	}
	if h.borrowed > 0 {
		return fmt.Errorf("lock handle is borrowed")
	}
	file := h.file
	h.file = nil
	h.released = true
	h.path = ""
	return file.Close()
}

func canonicalLockPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("lock path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve lock path: %w", err)
	}
	abs = filepath.Clean(abs)
	if !filepath.IsAbs(abs) {
		return "", fmt.Errorf("lock path must be absolute: %s", abs)
	}
	return abs, nil
}

// Borrow validates that h is a live capability for expectedPath and pins the
// descriptor until the returned release function is called.
func (h *Handle) Borrow(expectedPath string) (func(), error) {
	if h == nil {
		return nil, fmt.Errorf("lock handle is nil")
	}
	expectedCanonical, err := canonicalLockPath(expectedPath)
	if err != nil {
		return nil, err
	}

	h.mu.Lock()
	if h.file == nil || h.released {
		h.mu.Unlock()
		return nil, fmt.Errorf("lock handle is released")
	}
	if h.path != expectedCanonical {
		h.mu.Unlock()
		return nil, fmt.Errorf("lock handle path mismatch: have %s want %s", h.path, expectedCanonical)
	}
	file := h.file
	lockPath := h.path
	h.borrowed++
	h.mu.Unlock()

	if err := validateBorrowedLockDescriptor(file, lockPath); err != nil {
		h.endBorrow()
		return nil, err
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			h.endBorrow()
		})
	}, nil
}

func (h *Handle) endBorrow() {
	if h == nil {
		return
	}
	h.mu.Lock()
	if h.borrowed > 0 {
		h.borrowed--
	}
	h.mu.Unlock()
}

func nearestExistingAncestor(path string) (string, error) {
	current := filepath.Clean(path)
	for {
		if _, err := os.Lstat(current); err == nil {
			return current, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect lock path ancestor %s: %w", current, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no existing lock path ancestor for %s", path)
		}
		current = parent
	}
}

func openClassifierFile(path string) (*os.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open lock filesystem probe %s: %w", path, err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat lock filesystem probe %s: %w", path, err)
	}
	if !info.IsDir() {
		_ = file.Close()
		return nil, fmt.Errorf("lock filesystem probe is not a directory: %s", path)
	}
	return file, nil
}

func openLockParent(path string) (*os.Root, *os.File, error) {
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open lock parent root: %w", err)
	}
	dir, err := root.OpenFile(".", os.O_RDONLY, 0)
	if err != nil {
		_ = root.Close()
		return nil, nil, fmt.Errorf("open lock parent directory: %w", err)
	}
	pathInfo, err := os.Stat(path)
	if err != nil {
		_ = dir.Close()
		_ = root.Close()
		return nil, nil, fmt.Errorf("stat lock parent path: %w", err)
	}
	dirInfo, err := dir.Stat()
	if err != nil {
		_ = dir.Close()
		_ = root.Close()
		return nil, nil, fmt.Errorf("stat lock parent handle: %w", err)
	}
	if !os.SameFile(pathInfo, dirInfo) {
		_ = dir.Close()
		_ = root.Close()
		return nil, nil, fmt.Errorf("lock parent path identity changed while opening %s", path)
	}
	return root, dir, nil
}

func openLockLeaf(root *os.Root, leaf string) (*os.File, error) {
	pre, preErr := root.Lstat(leaf)
	if preErr == nil {
		if err := validateLockLeafInfo(pre); err != nil {
			return nil, err
		}
	} else if !errors.Is(preErr, fs.ErrNotExist) {
		return nil, fmt.Errorf("inspect lock leaf: %w", preErr)
	}

	file, err := root.OpenFile(leaf, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	post, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat lock file: %w", err)
	}
	if !post.Mode().IsRegular() {
		_ = file.Close()
		return nil, fmt.Errorf("lock leaf is not a regular file: %s", leaf)
	}
	leafInfo, err := root.Lstat(leaf)
	if err != nil {
		_ = file.Close()
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("lock leaf changed while opening: %s", leaf)
		}
		return nil, fmt.Errorf("lstat lock leaf: %w", err)
	}
	if err := validateLockLeafInfo(leafInfo); err != nil {
		_ = file.Close()
		return nil, err
	}
	if !os.SameFile(post, leafInfo) {
		_ = file.Close()
		return nil, fmt.Errorf("lock leaf identity changed while opening: %s", leaf)
	}
	if preErr == nil && !os.SameFile(pre, post) {
		_ = file.Close()
		return nil, fmt.Errorf("lock leaf identity changed while opening: %s", leaf)
	}
	return file, nil
}

func validateLockLeafInfo(info os.FileInfo) error {
	mode := info.Mode()
	if mode&os.ModeSymlink != 0 || mode&os.ModeIrregular != 0 {
		return fmt.Errorf("lock path must not be a symlink or irregular node")
	}
	if !mode.IsRegular() {
		return fmt.Errorf("lock path must be a regular file")
	}
	return nil
}

func validateBorrowedLockDescriptor(file *os.File, lockPath string) error {
	if file == nil {
		return fmt.Errorf("lock handle is missing a file descriptor")
	}
	descriptorInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat lock descriptor: %w", err)
	}
	if err := validateLockLeafInfo(descriptorInfo); err != nil {
		return err
	}

	parentPath := filepath.Dir(lockPath)
	leaf := filepath.Base(lockPath)
	parentRoot, parentDir, err := openLockParent(parentPath)
	if err != nil {
		return err
	}
	defer parentDir.Close()
	defer parentRoot.Close()

	pathInfo, err := parentRoot.Lstat(leaf)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("lock path changed while borrowing: %s", lockPath)
		}
		return fmt.Errorf("inspect lock path while borrowing: %w", err)
	}
	if err := validateLockLeafInfo(pathInfo); err != nil {
		return err
	}
	if !os.SameFile(descriptorInfo, pathInfo) {
		return fmt.Errorf("lock path identity changed while borrowing: %s", lockPath)
	}
	return nil
}
