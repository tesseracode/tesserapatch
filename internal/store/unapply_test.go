package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUnappliedIsValidFeatureState(t *testing.T) {
	if !ValidFeatureState(StateUnapplied) {
		t.Fatal("StateUnapplied must be accepted by ValidFeatureState")
	}
	if StateUnapplied != "unapplied" {
		t.Fatalf("wire value = %q, want unapplied", StateUnapplied)
	}
}

func TestWriteFileAtomicRenameFailurePreservesTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "status.json")
	before := []byte("{\"state\":\"applied\"}\n")
	if err := os.WriteFile(target, before, 0o644); err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("rename failed")
	err := writeFileAtomicWithRename(target, []byte("{\"state\":\"unapplied\"}\n"), 0o644, func(_, _ string) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}

	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("target changed after failed rename:\n got %q\nwant %q", after, before)
	}

	leftovers, err := filepath.Glob(filepath.Join(dir, ".status.json.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("temporary files remain after failed atomic write: %v", leftovers)
	}
}
