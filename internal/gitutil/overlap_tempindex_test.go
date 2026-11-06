package gitutil

// ADR-013 Amendment 2 D22/D24 — the isolated-index replay primitives of
// the overlap assessment. Every row names the mutation it catches.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// globFixture is a repository with two files whose names collide under
// glob matching: `a*.txt` (a literal star) and `ab.txt`. A patch changing
// both is written to patchPath; the working tree and real index are left
// at the seed commit.
func globFixture(t *testing.T) (repo, patchPath string) {
	t.Helper()
	repo = newApplyFixtureRepo(t)
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return string(out)
	}
	write("a*.txt", "star-1\nstar-2\n")
	write("ab.txt", "ab-1\nab-2\n")
	run("add", "-A")
	run("commit", "-q", "-m", "glob seed")
	write("a*.txt", "star-1\nSTAR-CHANGED\n")
	write("ab.txt", "ab-1\nAB-CHANGED\n")
	patch := run("-c", "core.quotepath=false", "diff", "--", ":(literal)a*.txt", ":(literal)ab.txt")
	run("checkout", "-q", "--", ".")
	patchPath = filepath.Join(t.TempDir(), "both.patch")
	if err := os.WriteFile(patchPath, []byte(patch), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo, patchPath
}

func seededIndex(t *testing.T, repo string) *TempIndex {
	t.Helper()
	idx, err := NewTempIndex(repo, filepath.Join(repo, ".git", "tpatch-verify"))
	if err != nil {
		t.Fatalf("NewTempIndex: %v", err)
	}
	t.Cleanup(func() { _ = idx.Close() })
	if err := idx.ReadTree("HEAD"); err != nil {
		t.Fatalf("ReadTree: %v", err)
	}
	return idx
}

func TestWildmatchLiteralEscapesEveryMetacharacter(t *testing.T) {
	for in, want := range map[string]string{
		"src/app.txt":  "src/app.txt",
		"a*.txt":       `a\*.txt`,
		"q?.go":        `q\?.go`,
		"[x].md":       `\[x].md`,
		`back\slash`:   `back\\slash`,
		"*?[\\":        `\*\?\[\\`,
		"dir/sub/f.go": "dir/sub/f.go",
	} {
		if got := WildmatchLiteral(in); got != want {
			t.Errorf("WildmatchLiteral(%q)=%q want %q", in, got, want)
		}
	}
}

// Mutation caught: dropping WildmatchLiteral from ApplyCached makes
// `--include=a*.txt` match `ab.txt` too, so ab.txt would change.
func TestApplyCachedIncludeIsLiteral(t *testing.T) {
	repo, patch := globFixture(t)
	idx := seededIndex(t, repo)
	if res := idx.ApplyCached(patch, []string{"a*.txt"}); !res.OK {
		t.Fatalf("ApplyCached: exit %d: %s", res.ExitCode, res.Stderr)
	}
	if d := idx.DiffCachedQuiet("HEAD", []string{"ab.txt"}); !d.Answered || !d.Equal {
		t.Fatalf("ab.txt changed through a literal include of a*.txt: %+v", d)
	}
	if d := idx.DiffCachedQuiet("HEAD", []string{"a*.txt"}); !d.Answered || d.Equal {
		t.Fatalf("a*.txt was not applied: %+v", d)
	}
}

// Mutation caught: without GIT_LITERAL_PATHSPECS the pathspec `a*.txt`
// globs onto the changed `ab.txt` and reports a difference.
func TestDiffCachedQuietPathsAreLiteral(t *testing.T) {
	repo, patch := globFixture(t)
	idx := seededIndex(t, repo)
	if res := idx.ApplyCached(patch, []string{"ab.txt"}); !res.OK {
		t.Fatalf("ApplyCached: exit %d: %s", res.ExitCode, res.Stderr)
	}
	if d := idx.DiffCachedQuiet("HEAD", []string{"a*.txt"}); !d.Answered || !d.Equal {
		t.Fatalf("the literal path a*.txt is unchanged, got %+v", d)
	}
	if d := idx.DiffCachedQuiet("HEAD", []string{"ab.txt"}); !d.Answered || d.Equal {
		t.Fatalf("ab.txt changed, got %+v", d)
	}
}

func TestDiffCachedQuietRefusesAndFailsClosed(t *testing.T) {
	repo, _ := globFixture(t)
	idx := seededIndex(t, repo)
	if d := idx.DiffCachedQuiet("HEAD", nil); d.Answered {
		t.Fatalf("an empty path list must be refused, got %+v", d)
	}
	if d := idx.DiffCachedQuiet("0123456789012345678901234567890123456789", []string{"ab.txt"}); d.Answered {
		t.Fatalf("an unresolvable revision is an execution failure, not an answer: %+v", d)
	}
}

// Review rev-1 F1: a `submodule.<name>.ignore=all` setting, in either
// `.git/config` or `.gitmodules`, must not hide a changed gitlink.
// Mutation caught: dropping `--ignore-submodules=none`, which answers
// "equal" in both variants.
func TestDiffCachedQuietComparesIgnoredSubmodules(t *testing.T) {
	for _, variant := range []string{"config", "gitmodules"} {
		t.Run(variant, func(t *testing.T) {
			repo := newApplyFixtureRepo(t)
			run := func(args ...string) string {
				cmd := exec.Command("git", args...)
				cmd.Dir = repo
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v: %s", args, err, out)
				}
				return string(bytes.TrimSpace(out))
			}
			gitmodules := "[submodule \"sub\"]\n\tpath = sub\n\turl = ./nowhere\n"
			if variant == "gitmodules" {
				gitmodules += "\tignore = all\n"
			}
			if err := os.WriteFile(filepath.Join(repo, ".gitmodules"), []byte(gitmodules), 0o644); err != nil {
				t.Fatal(err)
			}
			first := run("rev-parse", "HEAD")
			run("add", ".gitmodules")
			run("update-index", "--add", "--cacheinfo", "160000,"+first+",sub")
			run("commit", "-q", "-m", "gitlink at first")
			old := run("rev-parse", "HEAD")
			run("commit", "-q", "--allow-empty", "-m", "second")
			second := run("rev-parse", "HEAD")
			run("update-index", "--cacheinfo", "160000,"+second+",sub")
			run("commit", "-q", "-m", "gitlink at second")
			if variant == "config" {
				run("config", "submodule.sub.ignore", "all")
			}

			idx, err := NewTempIndex(repo, filepath.Join(repo, ".git", "tpatch-verify"))
			if err != nil {
				t.Fatalf("NewTempIndex: %v", err)
			}
			t.Cleanup(func() { _ = idx.Close() })
			if err := idx.ReadTree(old); err != nil {
				t.Fatalf("ReadTree: %v", err)
			}
			if d := idx.DiffCachedQuiet("HEAD", []string{"sub"}); !d.Answered || d.Equal {
				t.Fatalf("the gitlink moved from %s to %s but compared equal: %+v", first, second, d)
			}
		})
	}
}

func TestApplyCachedAnswersDoesNotApply(t *testing.T) {
	repo, patch := globFixture(t)
	idx := seededIndex(t, repo)
	if res := idx.ApplyCached(patch, nil); !res.OK {
		t.Fatalf("first apply: %s", res.Stderr)
	}
	res := idx.ApplyCached(patch, nil)
	if res.OK || !res.ApplyAnswered() {
		t.Fatalf("re-applying the same patch must be an answered refusal, got %+v", res)
	}
	missing := idx.ApplyCached(filepath.Join(t.TempDir(), "absent.patch"), nil)
	if missing.OK || missing.ApplyAnswered() {
		t.Fatalf("an unreadable patch file is an execution failure, got %+v", missing)
	}
}

// AC-6: the replay touches only the isolated index.
func TestApplyCachedLeavesRealIndexAndWorktree(t *testing.T) {
	repo, patch := globFixture(t)
	realIndex := func() []byte {
		b, err := os.ReadFile(filepath.Join(repo, ".git", "index"))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	before := realIndex()
	idx := seededIndex(t, repo)
	if res := idx.ApplyCached(patch, nil); !res.OK {
		t.Fatalf("ApplyCached: %s", res.Stderr)
	}
	if !bytes.Equal(before, realIndex()) {
		t.Fatal("ApplyCached modified the real index")
	}
	for name, want := range map[string]string{"a*.txt": "star-1\nstar-2\n", "ab.txt": "ab-1\nab-2\n"} {
		got, err := os.ReadFile(filepath.Join(repo, name))
		if err != nil || string(got) != want {
			t.Fatalf("ApplyCached modified the working tree file %s: %q", name, got)
		}
	}
}
