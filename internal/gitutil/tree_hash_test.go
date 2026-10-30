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

func TestTreeHashIgnoresReplaceObjects(t *testing.T) {
	root := treeHashRepo(t)
	originalHead := treeHashGit(t, root, "rev-parse", "HEAD")
	originalTree := treeHashGit(t, root, "rev-parse", "--verify", "HEAD^{tree}")

	treeHashGit(t, root, "checkout", "-q", "-b", "replacement-branch")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("replacement-tree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	treeHashGit(t, root, "add", "tracked.txt")
	treeHashGit(t, root, "commit", "-qm", "replacement")
	replacementHead := treeHashGit(t, root, "rev-parse", "HEAD")
	treeHashGit(t, root, "checkout", "-q", "main")

	treeHashGit(t, root, "replace", originalHead, replacementHead)
	t.Cleanup(func() {
		_ = exec.Command("git", "-C", root, "replace", "-d", originalHead).Run()
	})

	sanity := treeHashGit(t, root, "rev-parse", "--verify", "HEAD^{tree}")
	if sanity == originalTree {
		t.Skip("git rev-parse did not follow replace objects in this environment")
	}

	got, err := TreeHash(root, "HEAD")
	if err != nil {
		t.Fatalf("TreeHash: %v", err)
	}
	if got != originalTree {
		t.Fatalf("TreeHash honored replace objects: got %s want %s", got, originalTree)
	}
}

func TestHardenedGitEnvScrubsCaseInsensitiveRouting(t *testing.T) {
	names := []string{
		"git_dir", "git_work_tree", "git_index_file", "git_common_dir",
		"git_config_parameters", "git_config_count", "git_config_key_0", "git_config_value_0",
	}
	for _, name := range names {
		t.Setenv(name, "untrusted")
	}
	for _, entry := range HardenedNoLazyGitEnv() {
		name, _, _ := strings.Cut(entry, "=")
		for _, poisoned := range names {
			if strings.EqualFold(name, poisoned) {
				t.Fatalf("case-insensitive Git routing/config survived: %s", name)
			}
		}
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
printf '%%s\037%%s\037%%s\037%%s\037%%s\037%%s\037%%s\037%%s\037%%s\037%%s\037%%s\037%%s\037%%s\n' \
  "${GIT_NO_LAZY_FETCH-}" "${LC_ALL-}" "${GIT_NO_REPLACE_OBJECTS-}" \
  "${GIT_DIR-}" "${GIT_WORK_TREE-}" "${GIT_COMMON_DIR-}" "${GIT_INDEX_FILE-}" \
  "${GIT_CONFIG_GLOBAL-}" "${GIT_CONFIG_SYSTEM-}" "${GIT_CONFIG_NOSYSTEM-}" "${GIT_REPLACE_REF_BASE-}" "${GIT_CONFIG_PARAMETERS-}" "$*" >> %q
exec %q "$@"
`, logPath, realGit)
	if err := os.WriteFile(filepath.Join(shimDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GIT_NO_LAZY_FETCH", "0")
	t.Setenv("LC_ALL", "fr_FR.UTF-8")
	t.Setenv("GIT_NO_REPLACE_OBJECTS", "0")
	t.Setenv("GIT_DIR", filepath.Join(root, ".git"))
	t.Setenv("GIT_WORK_TREE", root)
	t.Setenv("GIT_COMMON_DIR", filepath.Join(root, ".git"))
	t.Setenv("GIT_INDEX_FILE", filepath.Join(root, "untrusted-index"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(root, ".gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(root, "system.gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_PARAMETERS", "'core.autocrlf'='false'")
	t.Setenv("GIT_REPLACE_REF_BASE", "refs/replace-foreign/")

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
		if len(fields) != 13 {
			t.Fatalf("unexpected env log %q", line)
		}
		if fields[0] != "1" || fields[1] != "C" || fields[2] != "1" {
			t.Fatalf("tree hash lost mandatory env envelope: %q", line)
		}
		temporaryIndex := fields[6]
		switch fields[12] {
		case "read-tree HEAD", "write-tree",
			"-c core.fsmonitor=false -c core.ignoreStat=false add -A -- .",
			`-c core.fsmonitor=false -c core.ignoreStat=false config --get-regexp --null ^(filter\.|diff\.external$|diff\..*\.(command|textconv)$|diff\.renames$|core\.autocrlf$)`,
			"-c core.fsmonitor=false -c core.ignoreStat=false ls-files --cached --stage -z",
			"-c core.fsmonitor=false -c core.ignoreStat=false ls-files --others --exclude-standard -z",
			"-c core.fsmonitor=false -c core.ignoreStat=false ls-files --cached -z -- .gitattributes :(glob)**/.gitattributes",
			"check-attr -z --all --stdin":
			if filepath.Dir(temporaryIndex) != filepath.Join(root, ".git") ||
				!strings.HasPrefix(filepath.Base(temporaryIndex), ".tpatch-tree-index-") {
				t.Fatalf("tree hash used an index outside its owned temporary namespace: %q", line)
			}
		case "rev-parse --verify HEAD^{tree}", "rev-parse --git-path index":
			if temporaryIndex != "" {
				t.Fatalf("read-only tree lookup inherited an index: %q", line)
			}
		default:
			t.Fatalf("unregistered tree-hash command: %q", line)
		}
		for index, value := range []struct{ name, want string }{
			{"GIT_DIR", ""}, {"GIT_WORK_TREE", ""}, {"GIT_COMMON_DIR", ""}, {"GIT_INDEX_FILE", temporaryIndex},
			{"GIT_CONFIG_GLOBAL", os.DevNull}, {"GIT_CONFIG_SYSTEM", os.DevNull},
			{"GIT_CONFIG_NOSYSTEM", "1"}, {"GIT_REPLACE_REF_BASE", ""},
			{"GIT_CONFIG_PARAMETERS", ""},
		} {
			if fields[3+index] != value.want {
				t.Fatalf("tree hash has unsafe %s: %q", value.name, line)
			}
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
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(foreign, ".gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(foreign, "system.gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_REPLACE_REF_BASE", "refs/replace-foreign/")
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

func TestWorktreeTreeHashIgnoresDefaultGlobalWorktreeOverride(t *testing.T) {
	target, foreign := treeHashRepo(t), treeHashRepo(t)
	if err := os.WriteFile(filepath.Join(target, "alpha.txt"), []byte("target\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "alpha.txt"), []byte("foreign\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	want, err := WorktreeTreeHash(target)
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	configPath := filepath.Join(home, ".gitconfig")
	treeHashGit(t, target, "config", "--file", configPath, "core.worktree", foreign)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", configPath)
	got, err := WorktreeTreeHash(target)
	if err != nil || got != want {
		t.Fatalf("global configuration redirected worktree hashing: got=%s want=%s err=%v", got, want, err)
	}
}
