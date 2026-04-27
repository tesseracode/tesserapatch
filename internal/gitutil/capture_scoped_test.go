package gitutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCapturePatchScoped_DefaultMatchesCapturePatch asserts that the
// new pathspec-aware variant, when called with empty pathspecs,
// reproduces the historical CapturePatch output byte-for-byte. This is
// the "no surprises for the default user base" guarantee.
func TestCapturePatchScoped_DefaultMatchesCapturePatch(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)

	// Modify a tracked file + add an untracked one.
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "extra.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	full, err := CapturePatch(dir)
	if err != nil {
		t.Fatalf("CapturePatch: %v", err)
	}
	scoped, err := CapturePatchScoped(dir, nil)
	if err != nil {
		t.Fatalf("CapturePatchScoped(nil): %v", err)
	}
	if full != scoped {
		t.Errorf("default scoped output diverges from CapturePatch.\n--- CapturePatch ---\n%s\n--- CapturePatchScoped(nil) ---\n%s", full, scoped)
	}
}

// TestCapturePatchScoped_NarrowsToPathspec asserts that providing a
// pathspec drops files outside the scope from the captured diff.
func TestCapturePatchScoped_NarrowsToPathspec(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)

	// Two changes in flight: one inside src/, one at repo root.
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "auth.go"), []byte("package src\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	full, err := CapturePatch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(full, "hello.txt") || !strings.Contains(full, "src/auth.go") {
		t.Fatalf("full capture should include both files; got:\n%s", full)
	}

	scoped, err := CapturePatchScoped(dir, []string{"src/"})
	if err != nil {
		t.Fatalf("CapturePatchScoped: %v", err)
	}
	if !strings.Contains(scoped, "src/auth.go") {
		t.Errorf("scoped capture missing in-scope file:\n%s", scoped)
	}
	if strings.Contains(scoped, "hello.txt") {
		t.Errorf("scoped capture leaked out-of-scope file:\n%s", scoped)
	}
}

// TestCapturePatchScoped_MultiplePathspecs confirms pathspecs OR-merge
// (any matching path is included).
func TestCapturePatchScoped_MultiplePathspecs(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)

	if err := os.MkdirAll(filepath.Join(dir, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "c"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"a/x.txt", "b/y.txt", "c/z.txt"} {
		if err := os.WriteFile(filepath.Join(dir, p), []byte("data\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	scoped, err := CapturePatchScoped(dir, []string{"a/", "b/"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(scoped, "a/x.txt") || !strings.Contains(scoped, "b/y.txt") {
		t.Errorf("multi-pathspec missing expected files:\n%s", scoped)
	}
	if strings.Contains(scoped, "c/z.txt") {
		t.Errorf("multi-pathspec leaked file outside scope:\n%s", scoped)
	}
}
