package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestS1ExtractUpstreamContextUsesStrictAffectedPaths(t *testing.T) {
	root := t.TempDir()
	name := "sp ace\ttab.txt"
	body := "provider context\n"
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := "diff --git \"a/sp ace\\ttab.txt\" \"b/sp ace\\ttab.txt\"\n" +
		"index 1111111..2222222 100644\n" +
		"--- \"a/sp ace\\ttab.txt\"\n+++ \"b/sp ace\\ttab.txt\"\n" +
		"@@ -1 +1 @@\n-old\n+new\n"

	got, err := extractUpstreamContext(root, patch)
	if err != nil {
		t.Fatalf("extractUpstreamContext: %v", err)
	}
	if !strings.Contains(got, "## "+name+"\n") || !strings.Contains(got, body) {
		t.Fatalf("provider context silently dropped the quoted path:\n%s", got)
	}
}

func TestS1ExtractUpstreamContextRetainsRenameSourceAndDestination(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "old name.txt"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "new name.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := "diff --git \"a/old name.txt\" \"b/new name.txt\"\n" +
		"similarity index 100%\nrename from old name.txt\nrename to new name.txt\n"

	got, err := extractUpstreamContext(root, patch)
	if err != nil {
		t.Fatalf("extractUpstreamContext: %v", err)
	}
	for _, name := range []string{"new name.txt", "old name.txt"} {
		if !strings.Contains(got, "## "+name+"\n") {
			t.Fatalf("provider context omitted %q:\n%s", name, got)
		}
	}
}

func TestS1ExtractUpstreamContextFailsClosed(t *testing.T) {
	if got, err := extractUpstreamContext(t.TempDir(), "not a patch\n"); err == nil || got != "" {
		t.Fatalf("malformed patch returned context %q, err=%v", got, err)
	}
}
