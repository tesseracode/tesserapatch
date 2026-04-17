package safety

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureSafeRepoPath(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		root    string
		path    string
		wantErr bool
	}{
		{"same as root", tmpDir, tmpDir, false},
		{"child of root", tmpDir, filepath.Join(tmpDir, "src", "main.go"), false},
		{"parent traversal", tmpDir, filepath.Join(tmpDir, "..", "etc", "passwd"), true},
		{"absolute escape", tmpDir, "/etc/passwd", true},
		{"dot-dot in middle", tmpDir, filepath.Join(tmpDir, "a", "..", "..", "escape"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnsureSafeRepoPath(tt.root, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnsureSafeRepoPath(%q, %q) error = %v, wantErr %v", tt.root, tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestEnsureSafeRepoPathSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	outsideDir := t.TempDir()

	// Create a symlink inside tmpDir pointing outside
	linkPath := filepath.Join(tmpDir, "escape-link")
	if err := os.Symlink(outsideDir, linkPath); err != nil {
		t.Skip("cannot create symlink:", err)
	}

	// The resolved path of the symlink target is outside root
	resolved, _ := filepath.EvalSymlinks(linkPath)
	err := EnsureSafeRepoPath(tmpDir, resolved)
	if err == nil {
		t.Error("expected error for symlink pointing outside repo root")
	}
}
