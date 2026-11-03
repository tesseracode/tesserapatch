package candidate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestROCS8DirectoryIdentitySurvivesRestartAndPartialDeletion(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("candidate ownership requires a supported native identity backend")
	}
	base := t.TempDir()
	directory := filepath.Join(base, "owned")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(directory, "child")
	if err := os.WriteFile(child, []byte("owned"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := readAcceptanceDirectoryIdentity(root)
	if err != nil {
		_ = root.Close()
		t.Fatal(err)
	}
	body, err := json.Marshal(identity)
	if err != nil {
		_ = root.Close()
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	var persisted AcceptanceDirectoryIdentity
	if err := json.Unmarshal(body, &persisted); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(child); err != nil {
		t.Fatal(err)
	}
	saved := filepath.Join(base, "renamed-original")
	if err := os.Rename(directory, saved); err != nil {
		t.Fatal(err)
	}
	reopened, err := os.OpenRoot(saved)
	if err != nil {
		t.Fatal(err)
	}
	if err := persisted.CheckRoot(reopened); err != nil {
		_ = reopened.Close()
		t.Fatalf("journaled native identity did not survive reopen/rename/partial deletion: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	replacement, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.Close()
	if err := persisted.CheckRoot(replacement); err == nil {
		t.Fatal("same native identity validator accepted an ordinary replacement directory")
	}
}
