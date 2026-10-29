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
printf '%%s\037%%s\037%%s\037%%s\037%%s\n' "${GIT_NO_LAZY_FETCH-}" "${LC_ALL-}" "${GIT_DIR-}" "${GIT_INDEX_FILE-}" "$*" >> %q
exec %q "$@"
`, logPath, realGit)
	if err := os.WriteFile(filepath.Join(shimDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GIT_NO_LAZY_FETCH", "0")
	t.Setenv("LC_ALL", "fr_FR.UTF-8")
	t.Setenv("GIT_DIR", filepath.Join(root, ".git"))

	if _, err := TreeHash(root, "HEAD"); err != nil {
		t.Fatalf("TreeHash: %v", err)
	}
	if _, err := WorktreeTreeHash(root); err != nil {
		t.Fatalf("WorktreeTreeHash: %v", err)
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(logBytes)), "\n") {
		fields := strings.Split(line, "\x1f")
		if len(fields) != 5 {
			t.Fatalf("unexpected env log %q", line)
		}
		if fields[0] != "1" || fields[1] != "C" {
			t.Fatalf("tree hash lost mandatory env envelope: %q", line)
		}
		if fields[2] != "" {
			t.Fatalf("tree hash inherited GIT_DIR unexpectedly: %q", line)
		}
	}
}

func TestWorktreeTreeHashTracksUncommittedChanges(t *testing.T) {
	root := treeHashRepo(t)
	base, err := WorktreeTreeHash(root)
	if err != nil {
		t.Fatalf("WorktreeTreeHash: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("alpha v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	editHash, err := WorktreeTreeHash(root)
	if err != nil {
		t.Fatalf("WorktreeTreeHash after edit: %v", err)
	}
	if editHash == base {
		t.Fatal("worktree tree hash did not change after file edit")
	}

	if err := os.WriteFile(filepath.Join(root, "beta.txt"), []byte("beta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	addHash, err := WorktreeTreeHash(root)
	if err != nil {
		t.Fatalf("WorktreeTreeHash after new file: %v", err)
	}
	if addHash == editHash {
		t.Fatal("worktree tree hash did not change after adding a file")
	}

	if err := os.Remove(filepath.Join(root, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	deleteHash, err := WorktreeTreeHash(root)
	if err != nil {
		t.Fatalf("WorktreeTreeHash after delete: %v", err)
	}
	if deleteHash == addHash {
		t.Fatal("worktree tree hash did not change after deletion")
	}

	gamma := filepath.Join(root, "gamma.sh")
	if err := os.WriteFile(gamma, []byte("#!/bin/sh\necho gamma\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	modeHash, err := WorktreeTreeHash(root)
	if err != nil {
		t.Fatalf("WorktreeTreeHash after mode change: %v", err)
	}
	if modeHash == deleteHash {
		t.Fatal("worktree tree hash did not change after mode-bearing new file")
	}
}

func TestWorktreeTreeHashDoesNotMutateLiveIndexOrRefs(t *testing.T) {
	root := treeHashRepo(t)
	beforeIndex := treeHashGit(t, root, "write-tree")
	beforeHead := treeHashGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "scratch.txt"), []byte("scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WorktreeTreeHash(root); err != nil {
		t.Fatalf("WorktreeTreeHash: %v", err)
	}
	afterIndex := treeHashGit(t, root, "write-tree")
	afterHead := treeHashGit(t, root, "rev-parse", "HEAD")
	if afterIndex != beforeIndex {
		t.Fatalf("WorktreeTreeHash mutated live index: before=%s after=%s", beforeIndex, afterIndex)
	}
	if afterHead != beforeHead {
		t.Fatalf("WorktreeTreeHash mutated HEAD: before=%s after=%s", beforeHead, afterHead)
	}
}

func TestWorktreeTreeHashHonorsHostileGitEnvIsolation(t *testing.T) {
	target := treeHashRepo(t)
	foreign := treeHashRepo(t)
	if err := os.WriteFile(filepath.Join(target, "alpha.txt"), []byte("target\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "alpha.txt"), []byte("foreign\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	targetHash, err := WorktreeTreeHash(target)
	if err != nil {
		t.Fatalf("WorktreeTreeHash baseline: %v", err)
	}
	foreignBefore := treeHashGit(t, foreign, "write-tree")

	t.Setenv("GIT_DIR", filepath.Join(foreign, ".git"))
	t.Setenv("GIT_WORK_TREE", foreign)
	t.Setenv("GIT_COMMON_DIR", filepath.Join(foreign, ".git"))
	t.Setenv("GIT_INDEX_FILE", filepath.Join(foreign, ".git", "index"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "alias.add")
	t.Setenv("GIT_CONFIG_VALUE_0", "!false")

	hash, err := WorktreeTreeHash(target)
	if err != nil {
		t.Fatalf("WorktreeTreeHash under hostile env: %v", err)
	}
	if hash != targetHash {
		t.Fatalf("hostile env redirected tree hash away from target repo: got %s want %s", hash, targetHash)
	}
	foreignAfter := treeHashGit(t, foreign, "write-tree")
	if foreignAfter != foreignBefore {
		t.Fatalf("foreign index mutated under hostile env: before=%s after=%s", foreignBefore, foreignAfter)
	}
}
