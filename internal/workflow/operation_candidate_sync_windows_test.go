//go:build windows

package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestROCS8WindowsDirectoryFlushRejectsReplacedPath(t *testing.T) {
	const sharingViolation syscall.Errno = 32
	base := t.TempDir()
	directory, original := filepath.Join(base, "held"), filepath.Join(base, "original")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	identity, err := root.Stat(".")
	if err != nil {
		t.Fatal(err)
	}
	current, err := os.Stat(directory)
	if err != nil || !os.SameFile(identity, current) {
		t.Fatalf("held directory does not match its original pathname: %v", err)
	}
	if err := operationCandidateSyncDirectory(root); err != nil {
		t.Fatalf("original native directory cannot be flushed: %v", err)
	}
	blocked := false
	if err := os.Rename(directory, original); err != nil {
		if !errors.Is(err, sharingViolation) {
			t.Fatalf("replacement setup failed for a reason other than the Windows sharing barrier: %v", err)
		}
		blocked = true
		current, err := os.Stat(directory)
		if err != nil || !os.SameFile(identity, current) {
			t.Fatalf("denied rename changed the held directory: %v", err)
		}
		if _, err := os.Lstat(original); !os.IsNotExist(err) {
			t.Fatalf("denied rename created its destination: %v", err)
		}
		if err := operationCandidateSyncDirectory(root); err != nil {
			t.Fatalf("sharing barrier was mistaken for a directory-flush failure: %v", err)
		}
		if err := root.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(directory, original); err != nil {
			t.Fatalf("rename still failed after releasing the native barrier: %v", err)
		}
		t.Log("Windows prevented replacement while os.Root was held; testing the actual flush identity check against the retained original identity after release")
	}
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(directory, "foreign.txt")
	if err := os.WriteFile(foreign, []byte("foreign\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := rocS8ReadImage(t, foreign)
	if !blocked {
		if err := operationCandidateSyncDirectory(root); err == nil || !strings.Contains(err.Error(), "acceptance directory changed before flush") {
			t.Fatalf("held-root flush did not reject its replaced path by identity: %v", err)
		}
	}
	if err := operationCandidateFlushDirectory(directory, identity); err == nil || !strings.Contains(err.Error(), "acceptance directory changed before flush") {
		t.Fatalf("native path reopening accepted the original identity for a foreign directory: %v", err)
	}
	replacement, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.Close()
	if err := operationCandidateSyncDirectory(replacement); err != nil {
		t.Fatalf("replacement directory cannot be flushed under its own identity: %v", err)
	}
	if rocS8ReadImage(t, foreign) != before {
		t.Fatal("directory flush changed foreign contents")
	}
}
