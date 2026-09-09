package store

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRGAS4AtomicArtifactRenameFailureAndCleanup(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "recipe-coverage.json")
	before := []byte("{\"previous\":true}\n")
	if err := os.WriteFile(target, before, 0o640); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("injected rename failure")
	err := writeFileAtomicWithRename(target, []byte("{\"replacement\":true}\n"), 0o640,
		func(source, destination string) error {
			if destination != target || filepath.Dir(source) != root {
				t.Fatal("atomic replacement must stage in the destination directory")
			}
			return failure
		})
	if !errors.Is(err, failure) {
		t.Fatalf("rename cause lost: %v", err)
	}
	after, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("prior artifact damaged: %q %v", after, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("staged artifact leaked: %v %v", entries, err)
	}
}

func TestRGAS4AtomicArtifactAdapterSafetyAndBytes(t *testing.T) {
	s, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(AddFeatureInput{Slug: "atomic", Title: "Atomic", Request: "atomic"}); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"", "{}\n", "\x00exact\r\nbytes"} {
		if err := s.WriteArtifactAtomic("atomic", "arbitrary.bin", body); err != nil {
			t.Fatal(err)
		}
		got, err := s.ReadFeatureFile("atomic", "artifacts/arbitrary.bin")
		if err != nil || got != body {
			t.Fatalf("atomic adapter normalized bytes: %q %v", got, err)
		}
	}
	if err := s.WriteArtifactAtomic("atomic", "../../../../../../outside", "unsafe"); err == nil {
		t.Fatal("atomic adapter accepted path escape")
	}
	if err := s.WriteArtifactAtomic("atomic", "post-apply.patch", "diff --git a/.git/config b/.git/config\n"); err == nil {
		t.Fatal("atomic adapter dropped canonical patch Git-internal refusal")
	}
}
