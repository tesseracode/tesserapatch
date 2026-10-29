package gitutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func treeHashGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func treeHashRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	treeHashGit(t, root, "init", "-q", "-b", "main", ".")
	treeHashGit(t, root, "config", "user.email", "treehash@example.com")
	treeHashGit(t, root, "config", "user.name", "Tree Hash")
	treeHashGit(t, root, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	treeHashGit(t, root, "add", "alpha.txt")
	treeHashGit(t, root, "commit", "-qm", "seed")
	return root
}

func TestTreeHashMatchesGitRevParse(t *testing.T) {
	root := treeHashRepo(t)
	want := treeHashGit(t, root, "rev-parse", "--verify", "HEAD^{tree}")
	got, err := TreeHash(root, "HEAD")
	if err != nil {
		t.Fatalf("TreeHash: %v", err)
	}
	if got != want {
		t.Fatalf("TreeHash = %q, want %q", got, want)
	}
}

func TestTreeHashUsesNoLazyFetchEnvelope(t *testing.T) {
	root := treeHashRepo(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shimDir := t.TempDir()
	logPath := filepath.Join(shimDir, "env.log")
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\037%%s\037%%s\n' "${GIT_NO_LAZY_FETCH-}" "${LC_ALL-}" "$*" >> %q
exec %q "$@"
`, logPath, realGit)
	if err := os.WriteFile(filepath.Join(shimDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GIT_NO_LAZY_FETCH", "0")
	t.Setenv("LC_ALL", "fr_FR.UTF-8")

	if _, err := TreeHash(root, "HEAD"); err != nil {
		t.Fatalf("TreeHash: %v", err)
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := strings.TrimSpace(string(logBytes))
	fields := strings.Split(log, "\x1f")
	if len(fields) != 3 {
		t.Fatalf("unexpected env log %q", log)
	}
	if fields[0] != "1" || fields[1] != "C" {
		t.Fatalf("TreeHash lost mandatory env envelope: %q", log)
	}
	if fields[2] != "rev-parse --verify HEAD^{tree}" {
		t.Fatalf("unexpected command: %q", fields[2])
	}
}

func TestTreeHashDoesNotMutateWorkingState(t *testing.T) {
	root := treeHashRepo(t)
	if err := os.WriteFile(filepath.Join(root, "scratch.txt"), []byte("scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := treeHashGit(t, root, "status", "--porcelain")
	if _, err := TreeHash(root, "HEAD"); err != nil {
		t.Fatalf("TreeHash: %v", err)
	}
	after := treeHashGit(t, root, "status", "--porcelain")
	if after != before {
		t.Fatalf("TreeHash mutated working state:\nbefore=%q\nafter=%q", before, after)
	}
}
