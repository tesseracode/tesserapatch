// CLI-level regressions for GH #7: registered linked Git worktrees
// nested beneath the target repository must never enter apply/record
// capture artifacts nor `land`'s staging / outside-path plan.
//
// These exercise the real commands against real Git fixtures with a
// real `git worktree add`, because the bug lived in the seam between
// Git's untracked-path reporting and tpatch's capture/staging surfaces
// — a parser-only test cannot observe it.
//
// Every fixture worktree is detached with `git worktree remove --force`
// plus a prune via t.Cleanup, which runs before t.TempDir() teardown.

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// nestedWorktreeDirName is deliberately space-bearing: the porcelain
// parser and every downstream pathspec must survive it.
const nestedWorktreeDirName = "agent review"

// nestedWorktreeRel is the repo-relative path of the fixture worktree.
var nestedWorktreeRel = filepath.ToSlash(filepath.Join(".claude", "worktrees", nestedWorktreeDirName))

func nwtGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %s: %v", args, dir, out, err)
	}
	return string(out)
}

// addNestedWorktree registers a linked worktree at repoRoot/rel and
// schedules its removal before the temp root is torn down.
func addNestedWorktree(t *testing.T, repoRoot, rel, branch string) string {
	t.Helper()
	abs := filepath.Join(repoRoot, filepath.FromSlash(rel))
	nwtGit(t, repoRoot, "worktree", "add", "-q", abs, "-b", branch)
	t.Cleanup(func() {
		rm := exec.Command("git", "worktree", "remove", "--force", abs)
		rm.Dir = repoRoot
		_ = rm.Run()
		prune := exec.Command("git", "worktree", "prune")
		prune.Dir = repoRoot
		_ = prune.Run()
	})
	return abs
}

// setupNestedWorktreeFixture builds a committed repo with tpatch
// scaffolding, a requested feature, a registered nested linked
// worktree, and intended edits to two tracked files.
func setupNestedWorktreeFixture(t *testing.T) (tmpDir, slug string) {
	t.Helper()
	tmpDir = t.TempDir()
	gitInitTestRepo(t, tmpDir)
	nwtGit(t, tmpDir, "config", "commit.gpgsign", "false")
	if err := os.MkdirAll(filepath.Join(tmpDir, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "internal", "example.go"), []byte("package example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nwtGit(t, tmpDir, "add", "internal/example.go")
	nwtGit(t, tmpDir, "-c", "commit.gpgsign=false", "commit", "-qm", "seed")

	if _, _, code := runCmd("init", "--path", tmpDir); code != 0 {
		t.Fatalf("tpatch init failed")
	}
	nwtGit(t, tmpDir, "add", ".")
	nwtGit(t, tmpDir, "-c", "commit.gpgsign=false", "commit", "-qm", "tpatch scaffolding")

	if _, _, code := runCmd("add", "--path", tmpDir, "Nested worktree guard"); code != 0 {
		t.Fatalf("tpatch add failed")
	}
	slug = "nested-worktree-guard"

	addNestedWorktree(t, tmpDir, nestedWorktreeRel, "agent-review")

	// Intended product changes, made OUTSIDE the nested worktree.
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "internal", "example.go"), []byte("package example\n\n// feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return tmpDir, slug
}

// assertNoNestedWorktree fails when a blob mentions the nested worktree
// path or a gitlink mode entry.
func assertNoNestedWorktree(t *testing.T, label, body string) {
	t.Helper()
	if strings.Contains(body, nestedWorktreeDirName) {
		t.Errorf("%s references the nested linked worktree:\n%s", label, body)
	}
	if strings.Contains(body, "160000") {
		t.Errorf("%s contains a mode-160000 gitlink entry:\n%s", label, body)
	}
}

// GH #7 reproduction step 4: manual Path B completion must not fold the
// nested worktree into the canonical or numbered patch.
func TestNestedWorktree_ApplyDone_NotCaptured(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)

	stdout, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done")
	if code != 0 {
		t.Fatalf("apply --mode done failed: stdout=%q stderr=%q", stdout, stderr)
	}

	canonical := readArtifact(t, tmpDir, slug, "post-apply.patch")
	assertNoNestedWorktree(t, "canonical post-apply.patch", canonical)
	for _, want := range []string{"README.md", "internal/example.go", "+changed", "+// feature"} {
		if !strings.Contains(canonical, want) {
			t.Errorf("canonical patch missing intended change %q:\n%s", want, canonical)
		}
	}

	patchesDir := filepath.Join(tmpDir, ".tpatch", "features", slug, "patches")
	entries, err := os.ReadDir(patchesDir)
	if err != nil {
		t.Fatalf("read patches dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one numbered patch")
	}
	for _, e := range entries {
		body, err := os.ReadFile(filepath.Join(patchesDir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		assertNoNestedWorktree(t, "numbered patch "+e.Name(), string(body))
	}
}

// Default record + recipe autogen must neither capture the nested
// worktree nor attempt to read it as a recipe write-file source.
func TestNestedWorktree_DefaultRecord_NotCapturedNorRecipeRead(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)

	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	stdout, stderr, code := runCmdWithError("record", "--path", tmpDir, slug)
	if code != 0 {
		t.Fatalf("record failed: stdout=%q stderr=%q", stdout, stderr)
	}
	if strings.Contains(stderr, "recipe autogen failed") {
		t.Errorf("recipe autogen tried to read the nested worktree: %s", stderr)
	}
	assertNoNestedWorktree(t, "record stdout", stdout)
	assertNoNestedWorktree(t, "record stderr", stderr)

	assertNoNestedWorktree(t, "canonical post-apply.patch", readArtifact(t, tmpDir, slug, "post-apply.patch"))
	assertNoNestedWorktree(t, "apply-recipe.json", readArtifact(t, tmpDir, slug, "apply-recipe.json"))
	assertNoNestedWorktree(t, "post-apply-diff.txt", readArtifact(t, tmpDir, slug, "post-apply-diff.txt"))
	if got := readArtifact(t, tmpDir, slug, "post-apply.patch"); !strings.Contains(got, "+changed") {
		t.Errorf("intended change missing from default record:\n%s", got)
	}
}

// `record --all` / `--staged` / `--unstaged` share the untracked
// discovery seam; none may admit the nested worktree.
func TestNestedWorktree_RecordCaptureModes_NotCaptured(t *testing.T) {
	for _, mode := range []string{"--all", "--staged", "--unstaged"} {
		t.Run(strings.TrimPrefix(mode, "--"), func(t *testing.T) {
			tmpDir, slug := setupNestedWorktreeFixture(t)
			if mode == "--staged" {
				nwtGit(t, tmpDir, "add", "README.md", "internal/example.go")
			}
			stdout, stderr, code := runCmdWithError("record", "--path", tmpDir, slug, mode)
			if code != 0 {
				t.Fatalf("record %s failed: stdout=%q stderr=%q", mode, stdout, stderr)
			}
			got := readArtifact(t, tmpDir, slug, "post-apply.patch")
			assertNoNestedWorktree(t, "post-apply.patch for record "+mode, got)
			if !strings.Contains(got, "+changed") {
				t.Errorf("record %s dropped the intended change:\n%s", mode, got)
			}
		})
	}
}

// Scoped record stays clean even when the worktree is named explicitly
// or implicitly through a parent directory pathspec.
func TestNestedWorktree_ScopedRecord_StaysClean(t *testing.T) {
	for _, files := range []string{
		"README.md,internal/example.go",
		"README.md,internal/example.go," + nestedWorktreeRel,
		"README.md,internal/example.go,.claude",
	} {
		t.Run(files, func(t *testing.T) {
			tmpDir, slug := setupNestedWorktreeFixture(t)
			stdout, stderr, code := runCmdWithError("record", "--path", tmpDir, slug, "--files", files)
			if code != 0 {
				t.Fatalf("record --files %q failed: stdout=%q stderr=%q", files, stdout, stderr)
			}
			got := readArtifact(t, tmpDir, slug, "post-apply.patch")
			assertNoNestedWorktree(t, "post-apply.patch for --files "+files, got)
			if !strings.Contains(got, "+changed") {
				t.Errorf("scoped record dropped the intended change:\n%s", got)
			}
		})
	}
}

// GH #7 reproduction step 5: the dry-run plan must omit the nested
// worktree from BOTH the staging list and the outside-path/refusal
// list, while an ordinary unrelated dirty path still shows up there.
func TestNestedWorktree_LandDryRun_AbsentFromPlan(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "unrelated.txt"), []byte("dirt\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--dry-run",
		"--files", "README.md,internal/example.go")
	if code != 0 {
		t.Fatalf("land --dry-run failed: stdout=%q stderr=%q", stdout, stderr)
	}
	assertNoNestedWorktree(t, "land --dry-run plan", stdout)
	if !strings.Contains(stdout, "Outside path set") || !strings.Contains(stdout, "unrelated.txt") {
		t.Errorf("ordinary unrelated dirty path must still be reported as outside the path set:\n%s", stdout)
	}
	for _, want := range []string{"README.md", "internal/example.go"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("dry-run staging plan missing %q:\n%s", want, stdout)
		}
	}
}

// Real land: the nested worktree is never staged nor committed, the
// intended paths land, and the worktree survives as untracked state.
func TestNestedWorktree_Land_NotStagedNorCommitted(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}

	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go")
	if code != 0 {
		t.Fatalf("land failed: stdout=%q stderr=%q", stdout, stderr)
	}

	committed := nwtGit(t, tmpDir, "show", "--pretty=format:", "--name-only", "HEAD")
	assertNoNestedWorktree(t, "landing commit", committed)
	for _, want := range []string{"README.md", "internal/example.go"} {
		if !strings.Contains(committed, want) {
			t.Errorf("landing commit missing %q:\n%s", want, committed)
		}
	}
	if staged := nwtGit(t, tmpDir, "diff", "--cached", "--name-only"); strings.TrimSpace(staged) != "" {
		t.Errorf("index not empty after land: %q", staged)
	}
	// The worktree itself must survive, untouched, as untracked state.
	if _, err := os.Stat(filepath.Join(tmpDir, filepath.FromSlash(nestedWorktreeRel))); err != nil {
		t.Errorf("land removed the nested worktree: %v", err)
	}
}

// Broad/default land must not sweep the gitlink in either, while
// `--allow-extra-paths` keeps its ordinary semantics for an unrelated
// dirty path.
func TestNestedWorktree_BroadLandAllowExtraPaths_DoesNotStageGitlink(t *testing.T) {
	t.Run("scoped land refuses on the ordinary path only", func(t *testing.T) {
		tmpDir, slug := setupNestedWorktreeFixture(t)
		if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
			t.Fatalf("apply --mode done failed: %s", stderr)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "unrelated.txt"), []byte("dirt\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
			"--files", "README.md,internal/example.go")
		if code == 0 {
			t.Fatalf("land should refuse on an unrelated dirty path: stdout=%q", stdout)
		}
		assertNoNestedWorktree(t, "land extras refusal", stderr)
		if !strings.Contains(stderr, "unrelated.txt") {
			t.Errorf("extras refusal must still name the ordinary dirty path:\n%s", stderr)
		}

		stdout, stderr, code = runCmdWithError("land", "--path", tmpDir, slug,
			"--files", "README.md,internal/example.go", "--allow-extra-paths")
		if code != 0 {
			t.Fatalf("land --allow-extra-paths failed: stdout=%q stderr=%q", stdout, stderr)
		}
		committed := nwtGit(t, tmpDir, "show", "--pretty=format:", "--name-only", "HEAD")
		assertNoNestedWorktree(t, "landing commit", committed)
		if !strings.Contains(committed, "unrelated.txt") {
			t.Errorf("--allow-extra-paths must still stage the ordinary extra path:\n%s", committed)
		}
	})

	t.Run("broad land stages everything except the gitlink", func(t *testing.T) {
		tmpDir, slug := setupNestedWorktreeFixture(t)
		if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
			t.Fatalf("apply --mode done failed: %s", stderr)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "unrelated.txt"), []byte("dirt\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug)
		if code != 0 {
			t.Fatalf("broad land failed: stdout=%q stderr=%q", stdout, stderr)
		}
		committed := nwtGit(t, tmpDir, "show", "--pretty=format:", "--name-only", "HEAD")
		assertNoNestedWorktree(t, "broad landing commit", committed)
		for _, want := range []string{"README.md", "internal/example.go", "unrelated.txt"} {
			if !strings.Contains(committed, want) {
				t.Errorf("broad landing commit missing %q:\n%s", want, committed)
			}
		}
		if staged := nwtGit(t, tmpDir, "diff", "--cached", "--name-only"); strings.TrimSpace(staged) != "" {
			t.Errorf("index not empty after broad land: %q", staged)
		}
	})
}

// A linked worktree registered OUTSIDE the target root is unrelated and
// must not be filtered from anything; an intentionally tracked gitlink
// must keep flowing through capture.
func TestNestedWorktree_ExternalWorktreeAndTrackedGitlinkUnaffected(t *testing.T) {
	tmpDir := t.TempDir()
	gitInitTestRepo(t, tmpDir)
	nwtGit(t, tmpDir, "config", "commit.gpgsign", "false")

	// External registered linked worktree — outside the target root.
	external := filepath.Join(t.TempDir(), "external")
	nwtGit(t, tmpDir, "worktree", "add", "-q", external, "-b", "external-wt")
	t.Cleanup(func() {
		rm := exec.Command("git", "worktree", "remove", "--force", external)
		rm.Dir = tmpDir
		_ = rm.Run()
		prune := exec.Command("git", "worktree", "prune")
		prune.Dir = tmpDir
		_ = prune.Run()
	})

	if _, _, code := runCmd("init", "--path", tmpDir); code != 0 {
		t.Fatalf("tpatch init failed")
	}
	nwtGit(t, tmpDir, "add", ".")
	nwtGit(t, tmpDir, "-c", "commit.gpgsign=false", "commit", "-qm", "tpatch scaffolding")
	if _, _, code := runCmd("add", "--path", tmpDir, "Track a submodule"); code != 0 {
		t.Fatalf("tpatch add failed")
	}
	slug := "track-a-submodule"

	// Intentionally tracked gitlink (submodule shape), created AFTER
	// the scaffolding commit so it is genuinely new in the index.
	sub := filepath.Join(tmpDir, "vendor", "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	nwtGit(t, sub, "init", "-q", "-b", "main", ".")
	nwtGit(t, sub, "config", "user.email", "t@example.com")
	nwtGit(t, sub, "config", "user.name", "T")
	if err := os.WriteFile(filepath.Join(sub, "s.txt"), []byte("s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nwtGit(t, sub, "add", "s.txt")
	nwtGit(t, sub, "-c", "commit.gpgsign=false", "commit", "-qm", "sub seed")
	nwtGit(t, tmpDir, "-c", "advice.addEmbeddedRepo=false", "add", "vendor/sub")

	stdout, stderr, code := runCmdWithError("record", "--path", tmpDir, slug, "--staged")
	if code != 0 {
		t.Fatalf("record --staged failed: stdout=%q stderr=%q", stdout, stderr)
	}
	got := readArtifact(t, tmpDir, slug, "post-apply.patch")
	if !strings.Contains(got, "vendor/sub") || !strings.Contains(got, "160000") {
		t.Errorf("intentionally tracked gitlink was filtered out of capture:\n%s", got)
	}

	// The external worktree must not be classified as nested, so an
	// ordinary untracked file in the target root still gets captured.
	if err := os.WriteFile(filepath.Join(tmpDir, "brand-new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runCmdWithError("record", "--path", tmpDir, slug, "--all"); code != 0 {
		t.Fatalf("record --all failed: %s", stderr)
	}
	if got := readArtifact(t, tmpDir, slug, "post-apply.patch"); !strings.Contains(got, "brand-new.txt") {
		t.Errorf("ordinary untracked file dropped while an external worktree is registered:\n%s", got)
	}
}
