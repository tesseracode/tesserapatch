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
	"fmt"
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

// ─── GH #7 rev-1: CLI-level fail-closed regression ──────────────────

// installFailingWorktreeListGit prepends a `git` wrapper to PATH that
// fails ONLY for `git worktree list` and execs the real git for
// everything else. It returns a restore func that removes the wrapper
// again, so the same test can prove both the refusal and the recovery.
//
// The simulated failure is a `fatal:` error, not an unknown-switch
// usage error, so it exercises the "genuine repository failure" branch
// rather than the pre-2.36 legacy fallback. Production code is
// untouched: the seam is the `git` executable itself.
func installFailingWorktreeListGit(t *testing.T) (restore func()) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate real git: %v", err)
	}
	binDir := t.TempDir()
	script := "#!/bin/sh\nprev=\"\"\nfor a in \"$@\"; do\n" +
		"  if [ \"$prev\" = \"worktree\" ] && [ \"$a\" = \"list\" ]; then\n" +
		"    echo \"fatal: simulated worktree discovery failure\" >&2\n" +
		"    exit 128\n" +
		"  fi\n" +
		"  prev=\"$a\"\n" +
		"done\n" +
		"exec " + realGit + " \"$@\"\n"
	shim := filepath.Join(binDir, "git")
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	original := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+original)

	// Sanity: the wrapper must actually break `worktree list` and
	// leave every other git subcommand working.
	probe := exec.Command("git", "worktree", "list", "--porcelain", "-z")
	probe.Dir = binDir
	if err := probe.Run(); err == nil {
		t.Fatal("git wrapper did not break `worktree list`")
	}
	if out, err := exec.Command("git", "--version").CombinedOutput(); err != nil {
		t.Fatalf("git wrapper broke unrelated subcommands: %s: %v", out, err)
	}
	return func() { os.Setenv("PATH", original) }
}

// snapshotDir returns a path → content map for every regular file
// under dir, used to prove a refusal left state byte-identical.
func snapshotDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		body, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = string(body)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func assertSnapshotUnchanged(t *testing.T, label string, before, after map[string]string) {
	t.Helper()
	for k, v := range before {
		got, ok := after[k]
		if !ok {
			t.Errorf("%s: %s was removed by a refusal", label, k)
			continue
		}
		if got != v {
			t.Errorf("%s: %s changed across a refusal", label, k)
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			t.Errorf("%s: %s was created by a refusal", label, k)
		}
	}
}

// apply --mode done must refuse and write nothing when worktree
// discovery itself fails, then succeed once discovery is restored.
func TestNestedWorktree_DiscoveryFailure_ApplyDoneRefusesThenRecovers(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	featureDir := filepath.Join(tmpDir, ".tpatch", "features", slug)
	before := snapshotDir(t, featureDir)

	restore := installFailingWorktreeListGit(t)
	stdout, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done")
	if code == 0 {
		t.Fatalf("apply --mode done must refuse when worktree discovery fails: stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "Refusing to capture") {
		t.Errorf("refusal missing the fail-closed guidance: %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(featureDir, "artifacts", "post-apply.patch")); err == nil {
		t.Error("apply --mode done wrote a patch despite refusing")
	}
	assertSnapshotUnchanged(t, "apply --mode done", before, snapshotDir(t, featureDir))

	restore()
	if stdout, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed after discovery was restored: stdout=%q stderr=%q", stdout, stderr)
	}
	canonical := readArtifact(t, tmpDir, slug, "post-apply.patch")
	assertNoNestedWorktree(t, "canonical patch after recovery", canonical)
	if !strings.Contains(canonical, "+changed") {
		t.Errorf("recovered capture missing the intended change:\n%s", canonical)
	}
}

// record must refuse and leave the feature directory byte-identical.
func TestNestedWorktree_DiscoveryFailure_RecordRefusesThenRecovers(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	featureDir := filepath.Join(tmpDir, ".tpatch", "features", slug)
	before := snapshotDir(t, featureDir)

	restore := installFailingWorktreeListGit(t)
	for _, mode := range [][]string{nil, {"--all"}, {"--unstaged"}, {"--files", "README.md"}} {
		args := append([]string{"record", "--path", tmpDir, slug}, mode...)
		stdout, stderr, code := runCmdWithError(args...)
		if code == 0 {
			t.Fatalf("record %v must refuse when worktree discovery fails: stdout=%q", mode, stdout)
		}
		if !strings.Contains(stderr, "Refusing to capture") {
			t.Errorf("record %v refusal missing the fail-closed guidance: %q", mode, stderr)
		}
		assertSnapshotUnchanged(t, "record "+strings.Join(mode, " "), before, snapshotDir(t, featureDir))
	}

	restore()
	if stdout, stderr, code := runCmdWithError("record", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("record failed after discovery was restored: stdout=%q stderr=%q", stdout, stderr)
	}
	assertNoNestedWorktree(t, "post-apply.patch after recovery", readArtifact(t, tmpDir, slug, "post-apply.patch"))
	assertNoNestedWorktree(t, "post-apply-diff.txt after recovery", readArtifact(t, tmpDir, slug, "post-apply-diff.txt"))
}

// land --dry-run must refuse without printing a staging plan, and land
// must not commit; both recover once discovery works again.
func TestNestedWorktree_DiscoveryFailure_LandRefusesThenRecovers(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	headBefore := gitHead(t, tmpDir)

	restore := installFailingWorktreeListGit(t)
	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--dry-run")
	if code == 0 {
		t.Fatalf("land --dry-run must refuse when worktree discovery fails: stdout=%q", stdout)
	}
	if strings.Contains(stdout, "Staging (path set):") || strings.Contains(stdout, "Outside path set") {
		t.Errorf("land --dry-run emitted a misleading plan despite refusing:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Refusing to capture") {
		t.Errorf("land --dry-run refusal missing the fail-closed guidance: %q", stderr)
	}

	stdout, stderr, code = runCmdWithError("land", "--path", tmpDir, slug, "--files", "README.md,internal/example.go")
	if code == 0 {
		t.Fatalf("land must refuse when worktree discovery fails: stdout=%q stderr=%q", stdout, stderr)
	}
	if got := gitHead(t, tmpDir); got != headBefore {
		t.Errorf("land advanced HEAD despite refusing: %s -> %s", headBefore, got)
	}

	restore()
	stdout, stderr, code = runCmdWithError("land", "--path", tmpDir, slug, "--dry-run",
		"--files", "README.md,internal/example.go")
	if code != 0 {
		t.Fatalf("land --dry-run failed after discovery was restored: stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "Staging (path set):") {
		t.Errorf("recovered dry-run missing the staging plan:\n%s", stdout)
	}
	assertNoNestedWorktree(t, "recovered dry-run plan", stdout)
	if stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go"); code != 0 {
		t.Fatalf("land failed after discovery was restored: stdout=%q stderr=%q", stdout, stderr)
	}
	if gitHead(t, tmpDir) == headBefore {
		t.Error("recovered land did not advance HEAD")
	}
	assertNoNestedWorktree(t, "recovered landing commit",
		nwtGit(t, tmpDir, "show", "--pretty=format:", "--name-only", "HEAD"))
}

// GH #7 rev-1 residual 3, CLI level: pre-existing staged gitlink
// residue must be absent from post-apply.patch AND post-apply-diff.txt.
func TestNestedWorktree_StagedGitlinkResidue_AbsentFromPatchAndDiffstat(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"default record", nil},
		{"scoped record", []string{"--files", "README.md,internal/example.go"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, slug := setupNestedWorktreeFixture(t)
			// Residue a pre-fix tpatch run (or an operator `git add`)
			// leaves behind in the index.
			nwtGit(t, tmpDir, "-c", "advice.addEmbeddedRepo=false",
				"--literal-pathspecs", "add", "--intent-to-add", "--", nestedWorktreeRel)

			if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
				t.Fatalf("apply --mode done failed: %s", stderr)
			}
			args := append([]string{"record", "--path", tmpDir, slug}, tc.args...)
			if stdout, stderr, code := runCmdWithError(args...); code != 0 {
				t.Fatalf("record failed: stdout=%q stderr=%q", stdout, stderr)
			}

			patch := readArtifact(t, tmpDir, slug, "post-apply.patch")
			diffstat := readArtifact(t, tmpDir, slug, "post-apply-diff.txt")
			assertNoNestedWorktree(t, "post-apply.patch", patch)
			assertNoNestedWorktree(t, "post-apply-diff.txt", diffstat)
			for _, want := range []string{"README.md", "internal/example.go"} {
				if !strings.Contains(patch, want) {
					t.Errorf("patch missing %q:\n%s", want, patch)
				}
				if !strings.Contains(diffstat, want) {
					t.Errorf("diffstat missing %q:\n%s", want, diffstat)
				}
			}
		})
	}
}

// A nested worktree whose directory name carries significant trailing
// whitespace must stay out of the CLI artifacts and out of land's
// plan, while the same name without the trailing space stays in.
func TestNestedWorktree_TrailingWhitespaceName_ExcludedEndToEnd(t *testing.T) {
	tmpDir := t.TempDir()
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
	if _, _, code := runCmd("add", "--path", tmpDir, "Trailing space guard"); code != 0 {
		t.Fatalf("tpatch add failed")
	}
	slug := "trailing-space-guard"

	wtRel := "wt/agent "
	addNestedWorktree(t, tmpDir, wtRel, "agent-trailing")
	// Prefix-boundary control: same name, no trailing space.
	control := filepath.Join(tmpDir, "wt", "agent")
	if err := os.MkdirAll(control, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(control, "keep.txt"), []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if stdout, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: stdout=%q stderr=%q", stdout, stderr)
	}
	patch := readArtifact(t, tmpDir, slug, "post-apply.patch")
	diffstat := readArtifact(t, tmpDir, slug, "post-apply-diff.txt")
	for label, body := range map[string]string{"post-apply.patch": patch, "post-apply-diff.txt": diffstat} {
		if strings.Contains(body, "160000") {
			t.Errorf("%s captured the trailing-space worktree as a gitlink:\n%s", label, body)
		}
		if strings.Contains(body, "wt/agent ") {
			t.Errorf("%s referenced the trailing-space worktree:\n%s", label, body)
		}
	}
	// The prefix-boundary control is untracked, so it shows up in the
	// patch (which intent-to-adds untracked files) but not in the
	// plain `git diff --stat` diffstat — that asymmetry predates this
	// fix and is unchanged by it.
	if !strings.Contains(patch, "wt/agent/keep.txt") {
		t.Errorf("post-apply.patch dropped the prefix-boundary control wt/agent/keep.txt:\n%s", patch)
	}
	if !strings.Contains(patch, "+changed") {
		t.Errorf("intended change missing:\n%s", patch)
	}

	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--dry-run")
	if code != 0 {
		t.Fatalf("land --dry-run failed: stdout=%q stderr=%q", stdout, stderr)
	}
	if strings.Contains(stdout, "wt/agent ") {
		t.Errorf("land plan listed the trailing-space worktree:\n%s", stdout)
	}
	if !strings.Contains(stdout, "wt/agent/keep.txt") {
		t.Errorf("land plan dropped the prefix-boundary control:\n%s", stdout)
	}
}

// ─── GH #7 rev-1: actionable empty-capture diagnostic ───────────────

// nestedWorktreeDiagnosticMarkers are the load-bearing phrases the
// actionable refusal must contain.
var nestedWorktreeDiagnosticMarkers = []string{
	"registered nested Git worktree",
	"intentionally excluded from capture",
	"mode-160000 gitlink",
	"git worktree remove",
}

// The misleading speculation the generic zero-byte diagnostic emits;
// it must NOT appear when the cause is the nested-worktree guard.
const genericEmptyCaptureSpeculation = "possibly mode-only or binary changes"

// `record --files` naming ONLY nested linked worktrees must say so
// instead of speculating about mode-only or binary changes.
func TestNestedWorktree_ScopedRecord_WorktreeOnlyFiles_ActionableDiagnostic(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)

	stdout, stderr, code := runCmdWithError("record", "--path", tmpDir, slug, "--files", nestedWorktreeRel)
	if code == 0 {
		t.Fatalf("record --files <worktree> must refuse: stdout=%q", stdout)
	}
	for _, want := range nestedWorktreeDiagnosticMarkers {
		if !strings.Contains(stderr, want) {
			t.Errorf("diagnostic missing %q:\n%s", want, stderr)
		}
	}
	if !strings.Contains(stderr, "every requested path is a registered nested Git worktree") {
		t.Errorf("worktree-only case should say every requested path is a worktree:\n%s", stderr)
	}
	if !strings.Contains(stderr, nestedWorktreeRel) {
		t.Errorf("diagnostic must name the offending pathspec %q:\n%s", nestedWorktreeRel, stderr)
	}
	if strings.Contains(stderr, genericEmptyCaptureSpeculation) {
		t.Errorf("misleading generic speculation still emitted:\n%s", stderr)
	}
	if strings.Contains(stderr, "--from <base-commit-or-ref>") {
		t.Errorf("misleading --from recovery hint still emitted:\n%s", stderr)
	}
	// A refusal must not advance the feature or write artifacts.
	if _, err := os.Stat(filepath.Join(tmpDir, ".tpatch", "features", slug, "artifacts", "post-apply.patch")); err == nil {
		t.Error("refused record wrote a canonical patch")
	}
}

// A subdirectory INSIDE the nested worktree is the same class.
func TestNestedWorktree_ScopedRecord_WorktreeDescendant_ActionableDiagnostic(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	inner := filepath.Join(tmpDir, filepath.FromSlash(nestedWorktreeRel), "src")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inner, "agent.go"), []byte("package agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runCmdWithError("record", "--path", tmpDir, slug,
		"--files", nestedWorktreeRel+"/src")
	if code == 0 {
		t.Fatalf("record --files <worktree>/src must refuse: stdout=%q", stdout)
	}
	for _, want := range nestedWorktreeDiagnosticMarkers {
		if !strings.Contains(stderr, want) {
			t.Errorf("diagnostic missing %q:\n%s", want, stderr)
		}
	}
	if strings.Contains(stderr, genericEmptyCaptureSpeculation) {
		t.Errorf("misleading generic speculation still emitted:\n%s", stderr)
	}
}

// Mixed control A: intended path HAS changes → capture succeeds and no
// refusal is emitted even though a worktree was also named.
func TestNestedWorktree_ScopedRecord_MixedWithRealChanges_Succeeds(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)

	stdout, stderr, code := runCmdWithError("record", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go,"+nestedWorktreeRel)
	if code != 0 {
		t.Fatalf("mixed record should succeed: stdout=%q stderr=%q", stdout, stderr)
	}
	if strings.Contains(stderr, "registered nested Git worktree") {
		t.Errorf("no refusal diagnostic expected for a non-empty capture:\n%s", stderr)
	}
	patch := readArtifact(t, tmpDir, slug, "post-apply.patch")
	assertNoNestedWorktree(t, "post-apply.patch", patch)
	if !strings.Contains(patch, "+changed") {
		t.Errorf("mixed record dropped the intended change:\n%s", patch)
	}
}

// Mixed control B: intended path has NO changes → the refusal names
// the excluded worktree AND separates the no-diff remainder, without
// claiming the whole request was worktrees.
func TestNestedWorktree_ScopedRecord_MixedWithoutChanges_PartitionedDiagnostic(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	// Revert the intended edits so the non-worktree pathspec is clean.
	nwtGit(t, tmpDir, "checkout", "--", "README.md", "internal/example.go")

	stdout, stderr, code := runCmdWithError("record", "--path", tmpDir, slug,
		"--files", "README.md,"+nestedWorktreeRel)
	if code == 0 {
		t.Fatalf("mixed record with no real changes must refuse: stdout=%q", stdout)
	}
	for _, want := range nestedWorktreeDiagnosticMarkers {
		if !strings.Contains(stderr, want) {
			t.Errorf("diagnostic missing %q:\n%s", want, stderr)
		}
	}
	if strings.Contains(stderr, "every requested path") {
		t.Errorf("mixed case must not claim every requested path is a worktree:\n%s", stderr)
	}
	if !strings.Contains(stderr, "Requested and not excluded (no diff):") ||
		!strings.Contains(stderr, "    - README.md") {
		t.Errorf("mixed diagnostic must partition the non-worktree remainder:\n%s", stderr)
	}
	if strings.Contains(stderr, genericEmptyCaptureSpeculation) {
		t.Errorf("misleading generic speculation still emitted:\n%s", stderr)
	}
}

// The staged/unstaged targeted refusals are equally misleading for a
// worktree-only scope, so they route through the new diagnostic too.
func TestNestedWorktree_ScopedRecord_WorktreeOnlyFiles_CaptureModes(t *testing.T) {
	for _, mode := range []string{"--all", "--staged", "--unstaged"} {
		t.Run(strings.TrimPrefix(mode, "--"), func(t *testing.T) {
			tmpDir, slug := setupNestedWorktreeFixture(t)
			stdout, stderr, code := runCmdWithError("record", "--path", tmpDir, slug,
				mode, "--files", nestedWorktreeRel)
			if code == 0 {
				t.Fatalf("record %s --files <worktree> must refuse: stdout=%q", mode, stdout)
			}
			if !strings.Contains(stderr, "registered nested Git worktree") {
				t.Errorf("record %s missing the actionable diagnostic:\n%s", mode, stderr)
			}
			for _, unwanted := range []string{
				genericEmptyCaptureSpeculation,
				"nothing staged for capture",
				"no unstaged worktree edits to capture",
			} {
				if strings.Contains(stderr, unwanted) {
					t.Errorf("record %s still emitted the misleading %q:\n%s", mode, unwanted, stderr)
				}
			}
		})
	}
}

// Non-regression: a genuinely empty capture that has nothing to do
// with nested worktrees keeps its existing diagnostics verbatim, both
// with and without --files, and in a repo that HAS a nested worktree.
func TestNestedWorktree_GenericEmptyCaptureDiagnosticPreserved(t *testing.T) {
	t.Run("dirty tree, scoped to an unchanged path", func(t *testing.T) {
		tmpDir, slug := setupNestedWorktreeFixture(t)
		if err := os.WriteFile(filepath.Join(tmpDir, "untouched.txt"), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		nwtGit(t, tmpDir, "add", "untouched.txt")
		nwtGit(t, tmpDir, "-c", "commit.gpgsign=false", "commit", "-qm", "add untouched")

		stdout, stderr, code := runCmdWithError("record", "--path", tmpDir, slug, "--files", "untouched.txt")
		if code == 0 {
			t.Fatalf("record on an unchanged path should refuse: stdout=%q", stdout)
		}
		if strings.Contains(stderr, "registered nested Git worktree") {
			t.Errorf("nested-worktree diagnostic leaked into an unrelated empty capture:\n%s", stderr)
		}
		if !strings.Contains(stderr, "tpatch record captured 0 bytes") {
			t.Errorf("generic empty-capture diagnostic missing:\n%s", stderr)
		}
		if !strings.Contains(stderr, genericEmptyCaptureSpeculation) {
			t.Errorf("generic dirty-tree speculation must be preserved:\n%s", stderr)
		}
	})

	t.Run("clean tree, unscoped", func(t *testing.T) {
		tmpDir, slug := setupNestedWorktreeFixture(t)
		nwtGit(t, tmpDir, "add", "README.md", "internal/example.go")
		nwtGit(t, tmpDir, "-c", "commit.gpgsign=false", "commit", "-qm", "commit the feature edits")
		nwtGit(t, tmpDir, "add", ".tpatch")
		nwtGit(t, tmpDir, "-c", "commit.gpgsign=false", "commit", "-qm", "commit tpatch state")

		stdout, stderr, code := runCmdWithError("record", "--path", tmpDir, slug)
		if code == 0 {
			t.Fatalf("record on a clean tree should refuse: stdout=%q", stdout)
		}
		if strings.Contains(stderr, "registered nested Git worktree") {
			t.Errorf("nested-worktree diagnostic leaked into an unrelated empty capture:\n%s", stderr)
		}
		if !strings.Contains(stderr, "--from <base-commit-or-ref>") {
			t.Errorf("generic --from recovery hint must be preserved:\n%s", stderr)
		}
	})
}

// ─── GH #7 rev-2 F4: transactional discovery ordering ───────────────

// installNthCallFailingWorktreeListGit prepends a `git` wrapper to PATH
// that succeeds for the first `failAfter` `git worktree list`
// invocations and fails every one after that, execing the real git for
// all other subcommands. The counter lives in a file so it survives the
// many short-lived `git` processes a single tpatch command spawns.
//
// This is the deterministic seam for "a first discovery succeeds, a
// later one fails": the command must then leave the feature directory
// byte-for-byte unchanged.
func installNthCallFailingWorktreeListGit(t *testing.T, failAfter int) (restore func()) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate real git: %v", err)
	}
	binDir := t.TempDir()
	counter := filepath.Join(binDir, "worktree-list-count")
	script := fmt.Sprintf(`#!/bin/sh
list=0
prev=""
for a in "$@"; do
  if [ "$prev" = "worktree" ] && [ "$a" = "list" ]; then list=1; fi
  prev="$a"
done
if [ "$list" = 1 ]; then
  n=0
  if [ -f %[1]q ]; then n=$(cat %[1]q); fi
  n=$((n+1))
  printf '%%s' "$n" > %[1]q
  if [ "$n" -gt %[2]d ]; then
    echo "fatal: simulated worktree discovery failure on call $n" >&2
    exit 128
  fi
fi
exec %[3]q "$@"
`, counter, failAfter, realGit)
	shim := filepath.Join(binDir, "git")
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	original := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+original)
	return func() { os.Setenv("PATH", original) }
}

// apply --mode done: the canonical patch and the diffstat are the only
// two discovery-dependent reads. A failure on the SECOND must leave the
// feature directory byte-identical and the state unadvanced.
func TestNestedWorktree_SecondDiscoveryFailure_ApplyDoneNoMutation(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	featureDir := filepath.Join(tmpDir, ".tpatch", "features", slug)
	before := snapshotDir(t, featureDir)
	stateBefore := readArtifact(t, tmpDir, slug, "status.json")

	restore := installNthCallFailingWorktreeListGit(t, 1)
	stdout, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done")
	if code == 0 {
		t.Fatalf("apply --mode done must refuse when the second discovery fails: stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "Refusing to capture") {
		t.Errorf("refusal missing the fail-closed guidance: %q", stderr)
	}
	assertSnapshotUnchanged(t, "apply --mode done (2nd discovery fails)", before, snapshotDir(t, featureDir))
	if got := readArtifact(t, tmpDir, slug, "status.json"); got != stateBefore {
		t.Errorf("status.json advanced despite refusing:\nbefore=%s\nafter=%s", stateBefore, got)
	}

	restore()
	if stdout, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed after recovery: stdout=%q stderr=%q", stdout, stderr)
	}
	canonical := readArtifact(t, tmpDir, slug, "post-apply.patch")
	assertNoNestedWorktree(t, "canonical patch after recovery", canonical)
	if !strings.Contains(canonical, "+changed") {
		t.Errorf("recovered capture missing the intended change:\n%s", canonical)
	}
}

// record: same contract, across the numbered-snapshot branch and the
// same-feature duplicate branch (which skips the numbered snapshot).
func TestNestedWorktree_SecondDiscoveryFailure_RecordNoMutation(t *testing.T) {
	for _, tc := range []struct {
		name        string
		preRecord   bool
		recordArgs  []string
		description string
	}{
		{name: "first record (numbered snapshot branch)"},
		{name: "re-record (same-feature duplicate branch)", preRecord: true},
		{name: "scoped record", recordArgs: []string{"--files", "README.md,internal/example.go"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, slug := setupNestedWorktreeFixture(t)
			if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
				t.Fatalf("apply --mode done failed: %s", stderr)
			}
			if tc.preRecord {
				if _, stderr, code := runCmdWithError("record", "--path", tmpDir, slug); code != 0 {
					t.Fatalf("priming record failed: %s", stderr)
				}
			}
			featureDir := filepath.Join(tmpDir, ".tpatch", "features", slug)
			before := snapshotDir(t, featureDir)

			restore := installNthCallFailingWorktreeListGit(t, 1)
			args := append([]string{"record", "--path", tmpDir, slug}, tc.recordArgs...)
			stdout, stderr, code := runCmdWithError(args...)
			if code == 0 {
				t.Fatalf("record must refuse when a later discovery fails: stdout=%q", stdout)
			}
			if !strings.Contains(stderr, "Refusing to capture") {
				t.Errorf("refusal missing the fail-closed guidance: %q", stderr)
			}
			assertSnapshotUnchanged(t, "record (2nd discovery fails)", before, snapshotDir(t, featureDir))

			restore()
			if stdout, stderr, code := runCmdWithError(args...); code != 0 {
				t.Fatalf("record failed after recovery: stdout=%q stderr=%q", stdout, stderr)
			}
			assertNoNestedWorktree(t, "post-apply.patch after recovery",
				readArtifact(t, tmpDir, slug, "post-apply.patch"))
			assertNoNestedWorktree(t, "post-apply-diff.txt after recovery",
				readArtifact(t, tmpDir, slug, "post-apply-diff.txt"))
		})
	}
}

// land delegates capture to the embedded record; a later discovery
// failure must still leave the index and HEAD untouched.
func TestNestedWorktree_SecondDiscoveryFailure_LandDoesNotStageOrCommit(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	headBefore := gitHead(t, tmpDir)

	restore := installNthCallFailingWorktreeListGit(t, 1)
	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go")
	if code == 0 {
		t.Fatalf("land must refuse when a later discovery fails: stdout=%q stderr=%q", stdout, stderr)
	}
	if got := gitHead(t, tmpDir); got != headBefore {
		t.Errorf("land advanced HEAD despite refusing: %s -> %s", headBefore, got)
	}
	if staged := nwtGit(t, tmpDir, "diff", "--cached", "--name-only"); strings.TrimSpace(staged) != "" {
		t.Errorf("land staged paths despite refusing: %q", staged)
	}

	restore()
	if stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go"); code != 0 {
		t.Fatalf("land failed after recovery: stdout=%q stderr=%q", stdout, stderr)
	}
	assertNoNestedWorktree(t, "landing commit after recovery",
		nwtGit(t, tmpDir, "show", "--pretty=format:", "--name-only", "HEAD"))
}

// `cycle` performs exactly ONE discovery-dependent read — the [6/6]
// capture — and its record-phase artifacts are written only after that
// read completes. The earlier phases write analysis/spec/exploration/
// recipe artifacts, which are not discovery-dependent, so the
// invariant proven here is: a discovery failure leaves NO record-phase
// artifact behind and does not advance the feature to `applied`.
func TestNestedWorktree_CycleSingleDiscovery(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	featureDir := filepath.Join(tmpDir, ".tpatch", "features", slug)

	restore := installNthCallFailingWorktreeListGit(t, 0)
	stdout, stderr, code := runCmdWithError("cycle", "--path", tmpDir, slug)
	restore()
	if code == 0 {
		t.Fatalf("cycle must refuse when discovery fails: stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "Refusing to capture") {
		t.Errorf("refusal missing the fail-closed guidance: %q", stderr)
	}
	for _, recordPhase := range []string{
		filepath.Join("artifacts", "post-apply.patch"),
		filepath.Join("artifacts", "post-apply-diff.txt"),
	} {
		if _, err := os.Stat(filepath.Join(featureDir, recordPhase)); err == nil {
			t.Errorf("cycle wrote %s despite refusing", recordPhase)
		}
	}
	if _, err := os.ReadDir(filepath.Join(featureDir, "patches")); err == nil {
		entries, _ := os.ReadDir(filepath.Join(featureDir, "patches"))
		if len(entries) > 0 {
			t.Errorf("cycle wrote %d numbered patch(es) despite refusing", len(entries))
		}
	}
	statusBody := readArtifact(t, tmpDir, slug, "status.json")
	if strings.Contains(statusBody, `"state": "applied"`) {
		t.Errorf("cycle advanced the feature to applied despite refusing:\n%s", statusBody)
	}

	// A single successful discovery completes the whole run: the
	// capture is the only discovery-dependent read in `cycle`.
	restoreOne := installNthCallFailingWorktreeListGit(t, 1)
	stdout, stderr, code = runCmdWithError("cycle", "--path", tmpDir, slug)
	restoreOne()
	if code != 0 {
		t.Fatalf("cycle needs more than one discovery call: stdout=%q stderr=%q", stdout, stderr)
	}
	assertNoNestedWorktree(t, "cycle post-apply.patch", readArtifact(t, tmpDir, slug, "post-apply.patch"))
}

// ─── GH #7 rev-3 F1: land discovery transaction ─────────────────────

// installCountingWorktreeListGit prepends a `git` wrapper that counts
// `git worktree list` invocations in a file (so the count survives the
// many short-lived `git` processes one tpatch command spawns) and
// optionally fails every call after `failAfter`. failAfter < 0 means
// "never fail". Returns the counter path and a restore func.
func installCountingWorktreeListGit(t *testing.T, failAfter int) (counterPath string, restore func()) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate real git: %v", err)
	}
	binDir := t.TempDir()
	counterPath = filepath.Join(binDir, "worktree-list-count")
	script := fmt.Sprintf(`#!/bin/sh
list=0
prev=""
for a in "$@"; do
  if [ "$prev" = "worktree" ] && [ "$a" = "list" ]; then list=1; fi
  prev="$a"
done
if [ "$list" = 1 ]; then
  n=0
  if [ -f %[1]q ]; then n=$(cat %[1]q); fi
  n=$((n+1))
  printf '%%s' "$n" > %[1]q
  if [ %[2]d -ge 0 ] && [ "$n" -gt %[2]d ]; then
    echo "fatal: simulated worktree discovery failure on call $n" >&2
    exit 128
  fi
fi
exec %[3]q "$@"
`, counterPath, failAfter, realGit)
	shim := filepath.Join(binDir, "git")
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	original := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+original)
	return counterPath, func() { os.Setenv("PATH", original) }
}

func worktreeListCount(t *testing.T, counterPath string) int {
	t.Helper()
	body, err := os.ReadFile(counterPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatal(err)
	}
	n := 0
	if _, err := fmt.Sscanf(strings.TrimSpace(string(body)), "%d", &n); err != nil {
		t.Fatalf("parse counter %q: %v", body, err)
	}
	return n
}

// measureRecordDiscoveryCalls returns how many `git worktree list`
// invocations a standalone `record` performs on an identical fixture.
// The land test calibrates against this instead of hardcoding internals.
func measureRecordDiscoveryCalls(t *testing.T) int {
	t.Helper()
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	counter, restore := installCountingWorktreeListGit(t, -1)
	if _, stderr, code := runCmdWithError("record", "--path", tmpDir, slug); code != 0 {
		restore()
		t.Fatalf("calibration record failed: %s", stderr)
	}
	restore()
	n := worktreeListCount(t, counter)
	if n < 1 {
		t.Fatalf("calibration measured %d record discovery calls", n)
	}
	return n
}

// GH #7 rev-3 F1 + rev-4 F2 + rev-5: land's discovery budget is exactly
// FIVE of its own calls — the pre-record entry gate, the pre-stage
// revalidation, the post-stage index audit, the final pre-commit audit
// and the post-commit re-audit (which catches a hook that staged a
// nested worktree) — plus whatever the embedded record needs. Calibrated against a standalone record so the
// assertion does not hardcode internals. Anything above this budget
// means a stray rediscovery crept back into the path.
func TestNestedWorktree_Land_DiscoveryBudget(t *testing.T) {
	recordCalls := measureRecordDiscoveryCalls(t)

	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}

	budget := 5 + recordCalls
	counter, restore := installCountingWorktreeListGit(t, budget)
	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go")
	total := worktreeListCount(t, counter)
	restore()
	if code != 0 {
		t.Fatalf("land exceeded its discovery budget (%d, observed %d): stdout=%q stderr=%q",
			budget, total, stdout, stderr)
	}
	if total != budget {
		t.Errorf("expected exactly %d worktree-list invocations (1 entry gate + %d embedded record + 1 pre-stage revalidation + 3 index audits), got %d",
			budget, recordCalls, total)
	}
	committed := nwtGit(t, tmpDir, "show", "--pretty=format:", "--name-only", "HEAD")
	assertNoNestedWorktree(t, "landing commit", committed)
	for _, want := range []string{"README.md", "internal/example.go"} {
		if !strings.Contains(committed, want) {
			t.Errorf("landing commit missing %q:\n%s", want, committed)
		}
	}
}

// An entry-discovery failure must refuse before the embedded record
// runs: no record artifact, no status mutation, no index or HEAD change.
func TestNestedWorktree_Land_EntryDiscoveryFailureLeavesEverythingUntouched(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	featureDir := filepath.Join(tmpDir, ".tpatch", "features", slug)
	before := snapshotDir(t, featureDir)
	headBefore := gitHead(t, tmpDir)
	statusBefore := nwtGit(t, tmpDir, "status", "--porcelain", "-z", "--untracked-files=all")

	_, restore := installCountingWorktreeListGit(t, 0)
	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go")
	restore()
	if code == 0 {
		t.Fatalf("land must refuse when the entry discovery fails: stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "Refusing to capture") {
		t.Errorf("refusal missing the fail-closed guidance: %q", stderr)
	}
	assertSnapshotUnchanged(t, "land (entry discovery fails)", before, snapshotDir(t, featureDir))
	if got := gitHead(t, tmpDir); got != headBefore {
		t.Errorf("HEAD advanced despite refusing: %s -> %s", headBefore, got)
	}
	if got := nwtGit(t, tmpDir, "status", "--porcelain", "-z", "--untracked-files=all"); got != statusBefore {
		t.Errorf("index/worktree mutated despite refusing")
	}
}

// --no-record spends only land's own five calls (entry gate +
// pre-stage revalidation + three index audits); --dry-run runs no
// record, stages nothing, and revalidates at plan time, so it spends
// one.
func TestNestedWorktree_Land_NoRecordAndDryRunDiscoveryBudget(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}

	t.Run("dry-run", func(t *testing.T) {
		counter, restore := installCountingWorktreeListGit(t, 1)
		stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--dry-run",
			"--files", "README.md,internal/example.go")
		total := worktreeListCount(t, counter)
		restore()
		if code != 0 {
			t.Fatalf("dry-run exceeded its discovery budget (observed %d calls): stdout=%q stderr=%q", total, stdout, stderr)
		}
		if total != 1 {
			t.Errorf("dry-run should discover exactly once, at plan time, got %d", total)
		}
		if !strings.Contains(stdout, "Staging (path set):") {
			t.Errorf("dry-run missing the staging plan:\n%s", stdout)
		}
		assertNoNestedWorktree(t, "dry-run plan", stdout)
	})

	t.Run("--no-record", func(t *testing.T) {
		counter, restore := installCountingWorktreeListGit(t, 5)
		stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--no-record")
		total := worktreeListCount(t, counter)
		restore()
		if code != 0 {
			t.Fatalf("--no-record exceeded its discovery budget (observed %d calls): stdout=%q stderr=%q", total, stdout, stderr)
		}
		if total != 5 {
			t.Errorf("--no-record should spend exactly 5 discoveries (entry gate + pre-stage revalidation + 3 index audits), got %d", total)
		}
		assertNoNestedWorktree(t, "--no-record landing commit",
			nwtGit(t, tmpDir, "show", "--pretty=format:", "--name-only", "HEAD"))
	})
}

// A stale pre-fix canonical patch whose gitlink header is C-quoted by
// Git must be parsed, filtered, and absent from the staging plan.
func TestNestedWorktree_Land_StaleQuotedGitlinkPatchFilteredFromPlan(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	wtRel := ".claude/worktrees/new\nline"
	abs := filepath.Join(tmpDir, filepath.FromSlash(wtRel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	add := exec.Command("git", "worktree", "add", "-q", abs, "-b", "wt-quoted")
	add.Dir = tmpDir
	if out, err := add.CombinedOutput(); err != nil {
		t.Skipf("platform/Git refuses a newline worktree path: %s: %v", out, err)
	}
	t.Cleanup(func() {
		rm := exec.Command("git", "worktree", "remove", "--force", abs)
		rm.Dir = tmpDir
		_ = rm.Run()
		prune := exec.Command("git", "worktree", "prune")
		prune.Dir = tmpDir
		_ = prune.Run()
	})

	// Build the stale artifact from REAL Git output.
	nwtGit(t, tmpDir, "-c", "advice.addEmbeddedRepo=false",
		"--literal-pathspecs", "add", "--", wtRel)
	c := exec.Command("git", "diff", "--cached", "HEAD")
	c.Dir = tmpDir
	gitlinkPatch, err := c.Output()
	if err != nil {
		t.Fatal(err)
	}
	nwtGit(t, tmpDir, "--literal-pathspecs", "reset", "-q", "--", wtRel)
	if !strings.Contains(string(gitlinkPatch), `diff --git "a/`) {
		t.Fatalf("fixture header is not C-quoted:\n%q", gitlinkPatch)
	}

	nwtGit(t, tmpDir, "add", "README.md")
	c2 := exec.Command("git", "diff", "--cached", "HEAD", "--", "README.md")
	c2.Dir = tmpDir
	intended, err := c2.Output()
	if err != nil {
		t.Fatal(err)
	}
	nwtGit(t, tmpDir, "reset", "-q", "--", "README.md")

	stale := string(gitlinkPatch) + string(intended)
	artifact := filepath.Join(tmpDir, ".tpatch", "features", slug, "artifacts", "post-apply.patch")
	if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--dry-run", "--no-record")
	if code != 0 {
		t.Fatalf("dry-run failed: stdout=%q stderr=%q", stdout, stderr)
	}
	if strings.Contains(stdout, "worktrees/new") || strings.Contains(stdout, "160000") {
		t.Errorf("stale quoted gitlink survived into the plan:\n%s", stdout)
	}
	if !strings.Contains(stdout, "README.md") {
		t.Errorf("intended path missing from the plan:\n%s", stdout)
	}
}

// A malformed canonical patch must refuse before status.json, the index
// or HEAD is touched. --no-record is used so the failure is attributable
// to land's own strict parse rather than the embedded record.
func TestNestedWorktree_Land_MalformedPatchFailsBeforeMutation(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	artifact := filepath.Join(tmpDir, ".tpatch", "features", slug, "artifacts", "post-apply.patch")
	malformed := "diff --git \"a/broken.txt b/broken.txt\nindex 111..222 100644\n--- a/broken.txt\n+++ b/broken.txt\n@@ -1 +1 @@\n-a\n+b\n"
	if err := os.WriteFile(artifact, []byte(malformed), 0o644); err != nil {
		t.Fatal(err)
	}

	featureDir := filepath.Join(tmpDir, ".tpatch", "features", slug)
	before := snapshotDir(t, featureDir)
	headBefore := gitHead(t, tmpDir)

	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--no-record")
	if code == 0 {
		t.Fatalf("land must refuse on a malformed canonical patch: stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "which paths post-apply.patch touches") {
		t.Errorf("refusal should name the parse failure: %q", stderr)
	}
	assertSnapshotUnchanged(t, "land (malformed patch)", before, snapshotDir(t, featureDir))
	if got := gitHead(t, tmpDir); got != headBefore {
		t.Errorf("HEAD advanced despite refusing: %s -> %s", headBefore, got)
	}
	if staged := nwtGit(t, tmpDir, "diff", "--cached", "--name-only"); strings.TrimSpace(staged) != "" {
		t.Errorf("index mutated despite refusing: %q", staged)
	}
}

// The extras contract is unchanged by the reorder: an ordinary
// unrelated dirty path still refuses, still names only itself, and
// --allow-extra-paths still stages it.
func TestNestedWorktree_Land_ExtrasSemanticsUnchangedAfterReorder(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "unrelated.txt"), []byte("dirt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(tmpDir, ".tpatch", "features", slug, "status.json")

	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go")
	if code == 0 {
		t.Fatalf("land should refuse on an unrelated dirty path: stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "unrelated.txt") {
		t.Errorf("extras refusal must name the ordinary dirty path:\n%s", stderr)
	}
	assertNoNestedWorktree(t, "extras refusal", stderr)
	// F1: land's own status-notes write must sit AFTER the last
	// refusal. (The embedded record legitimately rewrites status.json
	// as part of its own transaction; what must NOT appear is land's
	// "landed at ..." note.)
	statusAfter, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(statusAfter), `"notes": "landed at`) {
		t.Errorf("extras refusal wrote land's status notes:\n%s", statusAfter)
	}

	stdout, stderr, code = runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go", "--allow-extra-paths")
	if code != 0 {
		t.Fatalf("land --allow-extra-paths failed: stdout=%q stderr=%q", stdout, stderr)
	}
	committed := nwtGit(t, tmpDir, "show", "--pretty=format:", "--name-only", "HEAD")
	if !strings.Contains(committed, "unrelated.txt") {
		t.Errorf("--allow-extra-paths must still stage the extra path:\n%s", committed)
	}
	if !strings.Contains(committed, "status.json") {
		t.Errorf("status.json must still be staged by land:\n%s", committed)
	}
	assertNoNestedWorktree(t, "landing commit", committed)
	if got := gitPorcelain(t, tmpDir); strings.Contains(got, "status.json") {
		t.Errorf("status.json left dirty after land: %q", got)
	}
}

// ─── GH #7 rev-4 F2: land pre-stage revalidation ────────────────────

// installWorktreeRegisteringHookGit prepends a `git` wrapper that, on
// the `hookAt`-th `git worktree list` invocation, registers a NEW
// linked worktree at `wtRel` BEFORE delegating to the real git — then
// behaves normally forever after.
//
// This is the deterministic seam for "a worktree appears after land's
// entry discovery". Call 1 is land's pre-record gate, so hooking at
// call 2 (the embedded record's first discovery) guarantees the
// worktree exists before land's pre-stage revalidation and did NOT
// exist when the entry gate ran.
//
// The hook shells out to the real git directly, so it cannot recurse.
func installWorktreeRegisteringHookGit(t *testing.T, repoRoot, wtRel, branch string, hookAt int) (counterPath string, restore func()) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate real git: %v", err)
	}
	binDir := t.TempDir()
	counterPath = filepath.Join(binDir, "worktree-list-count")
	wtAbs := filepath.Join(repoRoot, filepath.FromSlash(wtRel))
	script := fmt.Sprintf(`#!/bin/sh
list=0
prev=""
for a in "$@"; do
  if [ "$prev" = "worktree" ] && [ "$a" = "list" ]; then list=1; fi
  prev="$a"
done
if [ "$list" = 1 ]; then
  n=0
  if [ -f %[1]q ]; then n=$(cat %[1]q); fi
  n=$((n+1))
  printf '%%s' "$n" > %[1]q
  if [ "$n" -eq %[2]d ]; then
    %[3]q -C %[4]q worktree add -q %[5]q -b %[6]q >/dev/null 2>&1 || true
  fi
fi
exec %[3]q "$@"
`, counterPath, hookAt, realGit, repoRoot, wtAbs, branch)
	shim := filepath.Join(binDir, "git")
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	original := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+original)
	t.Cleanup(func() {
		rm := exec.Command(realGit, "-C", repoRoot, "worktree", "remove", "--force", wtAbs)
		_ = rm.Run()
		prune := exec.Command(realGit, "-C", repoRoot, "worktree", "prune")
		_ = prune.Run()
	})
	return counterPath, func() { os.Setenv("PATH", original) }
}

// A linked worktree registered AFTER land's entry discovery must never
// be staged — not by the ordinary path set and not by the
// --allow-extra-paths fold, which is the branch that would otherwise
// sweep it in as a gitlink.
func TestNestedWorktree_Land_RevalidatesBeforeStaging(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"broad land with --allow-extra-paths", []string{"--allow-extra-paths"}},
		{"scoped land with --allow-extra-paths", []string{"--files", "README.md,internal/example.go", "--allow-extra-paths"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, slug := setupNestedWorktreeFixture(t)
			if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
				t.Fatalf("apply --mode done failed: %s", stderr)
			}
			lateRel := "wt/late agent"
			_, restore := installWorktreeRegisteringHookGit(t, tmpDir, lateRel, "late-agent", 2)

			args := append([]string{"land", "--path", tmpDir, slug}, tc.args...)
			stdout, stderr, code := runCmdWithError(args...)
			restore()
			if code != 0 {
				t.Fatalf("land failed: stdout=%q stderr=%q", stdout, stderr)
			}

			// The late worktree must exist (the hook ran) and must be
			// absent from index and commit.
			if _, err := os.Stat(filepath.Join(tmpDir, filepath.FromSlash(lateRel))); err != nil {
				t.Fatalf("hook did not register the late worktree: %v", err)
			}
			committed := nwtGit(t, tmpDir, "show", "--pretty=format:", "--name-only", "HEAD")
			if strings.Contains(committed, "late agent") || strings.Contains(committed, "160000") {
				t.Errorf("late-registered worktree entered the commit:\n%s", committed)
			}
			if strings.Contains(stderr, "staging extra path wt/late agent") {
				t.Errorf("--allow-extra-paths tried to stage the late worktree:\n%s", stderr)
			}
			tree := nwtGit(t, tmpDir, "ls-tree", "-r", "HEAD", "--name-only")
			if strings.Contains(tree, "late agent") {
				t.Errorf("late worktree is present in the committed tree:\n%s", tree)
			}
			for _, want := range []string{"README.md", "internal/example.go"} {
				if !strings.Contains(committed, want) {
					t.Errorf("landing commit missing %q:\n%s", want, committed)
				}
			}
			if staged := nwtGit(t, tmpDir, "diff", "--cached", "--name-only"); strings.TrimSpace(staged) != "" {
				t.Errorf("index not empty after land: %q", staged)
			}
		})
	}
}

// installFailAtNthWorktreeListGit fails ONLY the nth `git worktree
// list` invocation onwards, so a test can target the pre-stage
// revalidation specifically.
func installFailAtNthWorktreeListGit(t *testing.T, failFrom int) (counterPath string, restore func()) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate real git: %v", err)
	}
	binDir := t.TempDir()
	counterPath = filepath.Join(binDir, "worktree-list-count")
	script := fmt.Sprintf(`#!/bin/sh
list=0
prev=""
for a in "$@"; do
  if [ "$prev" = "worktree" ] && [ "$a" = "list" ]; then list=1; fi
  prev="$a"
done
if [ "$list" = 1 ]; then
  n=0
  if [ -f %[1]q ]; then n=$(cat %[1]q); fi
  n=$((n+1))
  printf '%%s' "$n" > %[1]q
  if [ "$n" -ge %[2]d ]; then
    echo "fatal: simulated worktree discovery failure on call $n" >&2
    exit 128
  fi
fi
exec %[3]q "$@"
`, counterPath, failFrom, realGit)
	shim := filepath.Join(binDir, "git")
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	original := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+original)
	return counterPath, func() { os.Setenv("PATH", original) }
}

// The documented boundary when the PRE-STAGE revalidation fails: the
// embedded record's artifacts persist (record's own completed
// transaction), but land's status note, the index and HEAD are all
// untouched.
func TestNestedWorktree_Land_RevalidationFailureBoundary(t *testing.T) {
	recordCalls := measureRecordDiscoveryCalls(t)

	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	headBefore := gitHead(t, tmpDir)
	statusPath := filepath.Join(tmpDir, ".tpatch", "features", slug, "status.json")

	// Entry gate = call 1, embedded record = calls 2..1+recordCalls,
	// pre-stage revalidation = call 2+recordCalls. Fail exactly there.
	_, restore := installFailAtNthWorktreeListGit(t, 2+recordCalls)
	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go")
	restore()
	if code == 0 {
		t.Fatalf("land must refuse when the pre-stage revalidation fails: stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "Refusing to capture") {
		t.Errorf("refusal missing the fail-closed guidance: %q", stderr)
	}

	// Documented boundary, asserted explicitly.
	if got := gitHead(t, tmpDir); got != headBefore {
		t.Errorf("HEAD advanced despite refusing: %s -> %s", headBefore, got)
	}
	if staged := nwtGit(t, tmpDir, "diff", "--cached", "--name-only"); strings.TrimSpace(staged) != "" {
		t.Errorf("index mutated despite refusing: %q", staged)
	}
	body, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), `"notes": "landed at`) {
		t.Errorf("land wrote its status note despite refusing:\n%s", body)
	}
	// The embedded record's artifacts DO persist — this is record's own
	// completed transaction, identical to `tpatch record` followed by a
	// failing `tpatch land`.
	if _, err := os.Stat(filepath.Join(tmpDir, ".tpatch", "features", slug, "artifacts", "post-apply.patch")); err != nil {
		t.Errorf("embedded record artifacts should persist at this boundary: %v", err)
	}

	// Recovery.
	if stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go"); code != 0 {
		t.Fatalf("land failed after recovery: stdout=%q stderr=%q", stdout, stderr)
	}
	if gitHead(t, tmpDir) == headBefore {
		t.Error("recovered land did not advance HEAD")
	}
}

// The unchanged-prefix control: when nothing registers in between, the
// revalidation is a no-op and the success output is byte-identical to
// a run without any wrapper.
func TestNestedWorktree_Land_RevalidationNoOpPreservesOutput(t *testing.T) {
	run := func(t *testing.T) (string, string) {
		t.Helper()
		tmpDir, slug := setupNestedWorktreeFixture(t)
		if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
			t.Fatalf("apply --mode done failed: %s", stderr)
		}
		stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
			"--files", "README.md,internal/example.go")
		if code != 0 {
			t.Fatalf("land failed: stdout=%q stderr=%q", stdout, stderr)
		}
		// The landing SHA differs per run; strip it.
		lines := strings.Split(stdout, "\n")
		for i, l := range lines {
			if strings.HasPrefix(l, "Landed ") {
				lines[i] = "Landed <sha>"
			}
		}
		return strings.Join(lines, "\n"), stderr
	}
	firstOut, firstErr := run(t)
	secondOut, secondErr := run(t)
	if firstOut != secondOut {
		t.Errorf("land stdout is not stable across runs:\n%q\nvs\n%q", firstOut, secondOut)
	}
	if firstErr != secondErr {
		t.Errorf("land stderr is not stable across runs:\n%q\nvs\n%q", firstErr, secondErr)
	}
	if !strings.Contains(firstOut, "Landed <sha>") {
		t.Errorf("unexpected success output:\n%s", firstOut)
	}
}

// A worktree registered before the dry-run's plan step must be
// reflected in the printed plan, without any mutation.
func TestNestedWorktree_LandDryRun_ReflectsLatestSetWithoutMutation(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	// Register a second worktree after apply, before the dry-run.
	addNestedWorktree(t, tmpDir, "wt/late agent", "late-agent-dry")

	headBefore := gitHead(t, tmpDir)
	porcelainBefore := gitPorcelain(t, tmpDir)

	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--dry-run")
	if code != 0 {
		t.Fatalf("dry-run failed: stdout=%q stderr=%q", stdout, stderr)
	}
	if strings.Contains(stdout, "late agent") {
		t.Errorf("dry-run plan listed the newly registered worktree:\n%s", stdout)
	}
	assertNoNestedWorktree(t, "dry-run plan", stdout)
	if got := gitHead(t, tmpDir); got != headBefore {
		t.Errorf("dry-run advanced HEAD")
	}
	if got := gitPorcelain(t, tmpDir); got != porcelainBefore {
		t.Errorf("dry-run mutated the working tree:\n pre=%q\npost=%q", porcelainBefore, got)
	}
}

// ─── GH #7 rev-5: land staging transaction ──────────────────────────

// installStageTimeWorktreeHookGit prepends a `git` wrapper that, on the
// `hookAt`-th `git add` invocation, registers a linked worktree at
// `wtRel` AND stages its gitlink, before delegating.
//
// This is deliberate fault injection, and the comment says so: because
// land stages explicit file paths, a bare stage-time registration does
// not by itself put the worktree in the index. The hook therefore
// creates the exact contaminated-index state that a widening `git add`,
// a future refactor, or a concurrent third party could produce, and the
// test asserts that the post-stage audit detects it and restores the
// pre-land index byte-for-byte. Proving the backstop works is the
// point; assuming `git add` will never widen is what rev-4 did.
//
// hookAt counts `git add` invocations across the whole process tree,
// which is deterministic for a single land run.
func installStageTimeWorktreeHookGit(t *testing.T, repoRoot, wtRel, branch string, hookAt int) (counterPath string, restore func()) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate real git: %v", err)
	}
	binDir := t.TempDir()
	counterPath = filepath.Join(binDir, "add-count")
	wtAbs := filepath.Join(repoRoot, filepath.FromSlash(wtRel))
	script := fmt.Sprintf(`#!/bin/sh
isadd=0
for a in "$@"; do
  if [ "$a" = "add" ]; then isadd=1; break; fi
done
if [ "$isadd" = 1 ]; then
  n=0
  if [ -f %[1]q ]; then n=$(cat %[1]q); fi
  n=$((n+1))
  printf '%%s' "$n" > %[1]q
  if [ "$n" -eq %[2]d ]; then
    %[3]q -C %[4]q worktree add -q %[5]q -b %[6]q >/dev/null 2>&1 || true
    %[3]q -C %[4]q -c advice.addEmbeddedRepo=false --literal-pathspecs add -- %[7]q >/dev/null 2>&1 || true
  fi
fi
exec %[3]q "$@"
`, counterPath, hookAt, realGit, repoRoot, wtAbs, branch, wtRel)
	shim := filepath.Join(binDir, "git")
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	original := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+original)
	t.Cleanup(func() {
		rm := exec.Command(realGit, "-C", repoRoot, "worktree", "remove", "--force", wtAbs)
		_ = rm.Run()
		prune := exec.Command(realGit, "-C", repoRoot, "worktree", "prune")
		_ = prune.Run()
	})
	return counterPath, func() { os.Setenv("PATH", original) }
}

// countGitAddInvocations measures how many `git add` calls a land run
// makes, so a hook can be aimed at the staging step specifically
// without hardcoding internals.
func countGitAddInvocations(t *testing.T) int {
	t.Helper()
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	counter := filepath.Join(binDir, "add-count")
	script := fmt.Sprintf(`#!/bin/sh
isadd=0
for a in "$@"; do
  if [ "$a" = "add" ]; then isadd=1; break; fi
done
if [ "$isadd" = 1 ]; then
  n=0
  if [ -f %[1]q ]; then n=$(cat %[1]q); fi
  n=$((n+1))
  printf '%%s' "$n" > %[1]q
fi
exec %[2]q "$@"
`, counter, realGit)
	shim := filepath.Join(binDir, "git")
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	original := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+original)
	_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go", "--allow-extra-paths")
	os.Setenv("PATH", original)
	if code != 0 {
		t.Fatalf("calibration land failed: %s", stderr)
	}
	n := worktreeListCount(t, counter)
	if n < 2 {
		t.Fatalf("calibration measured %d `git add` invocations", n)
	}
	return n
}

// The rev-5 regression: a worktree registered DURING the staging step
// (after the pre-stage revalidation) means `git add` records a gitlink
// in land's private index. The post-stage audit must catch it and
// refuse, leaving HEAD, the live index and the landed-at note alone.
func TestNestedWorktree_Land_StageTimeRegistrationIsAuditedAndRolledBack(t *testing.T) {
	addCalls := countGitAddInvocations(t)

	for _, tc := range []struct {
		name   string
		args   []string
		hookAt int
	}{
		{"broad land, hook on the first add", []string{"--allow-extra-paths"}, 1},
		{"scoped land, hook on the first add", []string{"--files", "README.md,internal/example.go", "--allow-extra-paths"}, 1},
		{"hook on the final status-staging add", []string{"--allow-extra-paths"}, addCalls - 1},
		{"hook on the very last add", []string{"--allow-extra-paths"}, addCalls},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.hookAt < 1 {
				t.Skipf("calibrated hook position %d is not reachable", tc.hookAt)
			}
			tmpDir, slug := setupNestedWorktreeFixture(t)
			if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
				t.Fatalf("apply --mode done failed: %s", stderr)
			}
			// Give the soon-to-be worktree directory a presence in the
			// dirty set beforehand is impossible (git worktree add
			// needs a free path), so the harness registers it fresh.
			lateRel := "wt/stage time"
			headBefore := gitHead(t, tmpDir)
			indexTreeBefore := nwtGit(t, tmpDir, "write-tree")

			_, restore := installStageTimeWorktreeHookGit(t, tmpDir, lateRel, "stage-time", tc.hookAt)
			args := append([]string{"land", "--path", tmpDir, slug}, tc.args...)
			stdout, stderr, code := runCmdWithError(args...)
			restore()

			if _, err := os.Stat(filepath.Join(tmpDir, filepath.FromSlash(lateRel))); err != nil {
				t.Fatalf("hook did not register the stage-time worktree: %v", err)
			}

			if code == 0 {
				t.Fatalf("the post-stage audit must catch the injected gitlink; land returned success\nstdout=%q stderr=%q", stdout, stderr)
			}

			// Refusal path: the audit fired. Index restored, nothing
			// committed, no landed-at note.
			if !strings.Contains(stderr, "nested Git worktree") {
				t.Errorf("refusal should name the contamination: %q", stderr)
			}
			if got := gitHead(t, tmpDir); got != headBefore {
				t.Errorf("HEAD advanced despite refusing: %s -> %s (stdout=%q)", headBefore, got, stdout)
			}
			if got := nwtGit(t, tmpDir, "write-tree"); got != indexTreeBefore {
				t.Errorf("index not restored:\nbefore=%s\nafter=%s", indexTreeBefore, got)
			}
			cached := nwtGit(t, tmpDir, "diff", "--cached", "--name-only")
			if strings.Contains(cached, "stage time") {
				t.Errorf("worktree still present in the cached diff:\n%s", cached)
			}
			statusBody, err := os.ReadFile(filepath.Join(tmpDir, ".tpatch", "features", slug, "status.json"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(statusBody), `"notes": "landed at`) {
				t.Errorf("land wrote its status note despite refusing:\n%s", statusBody)
			}

			// Recovery: remove the worktree, land succeeds.
			rm := exec.Command("git", "worktree", "remove", "--force",
				filepath.Join(tmpDir, filepath.FromSlash(lateRel)))
			rm.Dir = tmpDir
			if out, rerr := rm.CombinedOutput(); rerr != nil {
				t.Fatalf("cleanup worktree: %s: %v", out, rerr)
			}
			if stdout, stderr, code := runCmdWithError(args...); code != 0 {
				t.Fatalf("land failed after recovery: stdout=%q stderr=%q", stdout, stderr)
			}
			assertNoNestedWorktree(t, "landing commit after recovery",
				nwtGit(t, tmpDir, "show", "--pretty=format:", "--name-only", "HEAD"))
		})
	}
}

// An operator's own staged work is never touched: land stages in a
// private temp index, so an abort leaves the live index exactly as the
// operator last set it (GH #7 rev-6 — there is nothing to "roll back").
func TestNestedWorktree_Land_AbortLeavesOperatorStagedStateUntouched(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	// Operator-staged content that land has no business touching.
	if err := os.WriteFile(filepath.Join(tmpDir, "operator.txt"), []byte("operator staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "intent.txt"), []byte("intent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nwtGit(t, tmpDir, "add", "operator.txt")
	nwtGit(t, tmpDir, "add", "--intent-to-add", "intent.txt")

	indexPath := strings.TrimSpace(nwtGit(t, tmpDir, "rev-parse", "--git-path", "index"))
	if !filepath.IsAbs(indexPath) {
		indexPath = filepath.Join(tmpDir, indexPath)
	}
	rawBefore, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	treeBefore := nwtGit(t, tmpDir, "write-tree")
	headBefore := gitHead(t, tmpDir)

	// Fail the post-stage audit's discovery: entry gate = 1, embedded
	// record = recordCalls, pre-stage revalidation = 2+recordCalls,
	// post-stage audit = 3+recordCalls.
	recordCalls := measureRecordDiscoveryCalls(t)
	_, restore := installFailAtNthWorktreeListGit(t, 3+recordCalls)
	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go", "--allow-extra-paths")
	restore()
	if code == 0 {
		t.Fatalf("land must refuse when the post-stage audit's discovery fails: stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "Refusing to capture") {
		t.Errorf("refusal missing the fail-closed guidance: %q", stderr)
	}

	rawAfter, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(rawAfter) != string(rawBefore) {
		t.Error("the live index was modified even though staging is isolated")
	}
	if got := nwtGit(t, tmpDir, "write-tree"); got != treeBefore {
		t.Errorf("write-tree differs after the abort:\nbefore=%s\nafter=%s", treeBefore, got)
	}
	if got := gitHead(t, tmpDir); got != headBefore {
		t.Errorf("HEAD advanced despite refusing: %s -> %s", headBefore, got)
	}
	cached := nwtGit(t, tmpDir, "diff", "--cached", "--name-only")
	if !strings.Contains(cached, "operator.txt") {
		t.Errorf("operator's staged file lost:\n%s", cached)
	}
	if strings.Contains(cached, "README.md") {
		t.Errorf("land's staging leaked into the live index:\n%s", cached)
	}
	statusBody, err := os.ReadFile(filepath.Join(tmpDir, ".tpatch", "features", slug, "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(statusBody), `"notes": "landed at`) {
		t.Errorf("land wrote its status note despite refusing:\n%s", statusBody)
	}
}

// The commit-hook failure contract is unchanged: once the transaction
// has passed audit and the commit is attempted, a failing hook leaves
// the audited index staged for a `--no-record` retry.
func TestNestedWorktree_Land_CommitHookFailureLeavesAuditedIndexStaged(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	hookDir := filepath.Join(tmpDir, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(hookDir, "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho 'hook says no' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	headBefore := gitHead(t, tmpDir)

	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go")
	if code == 0 {
		t.Fatalf("land should fail when the pre-commit hook rejects: stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "--no-record") {
		t.Errorf("hook-failure guidance should mention the --no-record retry: %q", stderr)
	}
	if got := gitHead(t, tmpDir); got != headBefore {
		t.Errorf("HEAD advanced despite the hook failure")
	}
	cached := nwtGit(t, tmpDir, "diff", "--cached", "--name-only")
	for _, want := range []string{"README.md", "internal/example.go", "status.json"} {
		if !strings.Contains(cached, want) {
			t.Errorf("audited index should remain staged for retry, %q missing:\n%s", want, cached)
		}
	}
	assertNoNestedWorktree(t, "staged index after hook failure", cached)

	// The retry path works once the hook is removed.
	if err := os.Remove(hook); err != nil {
		t.Fatal(err)
	}
	if stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--no-record"); code != 0 {
		t.Fatalf("--no-record retry failed: stdout=%q stderr=%q", stdout, stderr)
	}
	if gitHead(t, tmpDir) == headBefore {
		t.Error("retry did not advance HEAD")
	}
}

// Clean success is unchanged by the transaction: the same paths land,
// the tree is clean, and the landed-at note is committed.
func TestNestedWorktree_Land_CleanSuccessUnchangedByTransaction(t *testing.T) {
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
	for _, want := range []string{"README.md", "internal/example.go", "status.json"} {
		if !strings.Contains(committed, want) {
			t.Errorf("landing commit missing %q:\n%s", want, committed)
		}
	}
	if staged := nwtGit(t, tmpDir, "diff", "--cached", "--name-only"); strings.TrimSpace(staged) != "" {
		t.Errorf("index not empty after a clean land: %q", staged)
	}
	if got := gitPorcelain(t, tmpDir); strings.Contains(got, "status.json") {
		t.Errorf("status.json left dirty after land: %q", got)
	}
	body, err := os.ReadFile(filepath.Join(tmpDir, ".tpatch", "features", slug, "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"notes": "landed at`) {
		t.Errorf("landed-at note missing on the success path:\n%s", body)
	}
}

// ─── GH #7 rev-6: isolated temp-index land transaction ──────────────

// liveIndexPath returns the repo's effective index path.
func liveIndexPath(t *testing.T, repoRoot string) string {
	t.Helper()
	p := strings.TrimSuffix(nwtGit(t, repoRoot, "rev-parse", "--git-path", "index"), "\n")
	if !filepath.IsAbs(p) {
		p = filepath.Join(repoRoot, p)
	}
	return p
}

// Land must not touch the live index at any point before publication.
// A concurrent operator `git add` performed while land is staging is
// therefore preserved, and detected at publish time rather than
// overwritten.
func TestNestedWorktree_Land_ConcurrentOperatorAddIsDetectedNotOverwritten(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "operator.txt"), []byte("operator\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nwtGit(t, tmpDir, "add", "operator.txt")

	// A `git` wrapper that, on the first `git add` land performs,
	// stages an operator file into the LIVE index (no GIT_INDEX_FILE),
	// simulating a concurrent `git add` mid-land.
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	counter := filepath.Join(binDir, "add-count")
	if err := os.WriteFile(filepath.Join(tmpDir, "concurrent.txt"), []byte("concurrent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf(`#!/bin/sh
isadd=0
for a in "$@"; do
  if [ "$a" = "add" ]; then isadd=1; break; fi
done
if [ "$isadd" = 1 ]; then
  n=0
  if [ -f %[1]q ]; then n=$(cat %[1]q); fi
  n=$((n+1))
  printf '%%s' "$n" > %[1]q
  if [ "$n" -eq 1 ]; then
    env -u GIT_INDEX_FILE %[2]q -C %[3]q add concurrent.txt >/dev/null 2>&1 || true
  fi
fi
exec %[2]q "$@"
`, counter, realGit, tmpDir)
	shim := filepath.Join(binDir, "git")
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	original := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+original)

	headBefore := gitHead(t, tmpDir)
	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go", "--allow-extra-paths")
	os.Setenv("PATH", original)

	if code == 0 {
		t.Fatalf("land must refuse when the live index changed mid-flight: stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "changed while") {
		t.Errorf("refusal should explain the concurrent mutation: %q", stderr)
	}
	if got := gitHead(t, tmpDir); got != headBefore {
		t.Errorf("HEAD advanced despite refusing")
	}
	// The operator's concurrent work is intact and land's staging is
	// nowhere to be seen.
	cached := nwtGit(t, tmpDir, "diff", "--cached", "--name-only")
	for _, want := range []string{"operator.txt", "concurrent.txt"} {
		if !strings.Contains(cached, want) {
			t.Errorf("operator's staged path %q lost:\n%s", want, cached)
		}
	}
	if strings.Contains(cached, "README.md") {
		t.Errorf("land's staging leaked into the live index:\n%s", cached)
	}
	statusBody, err := os.ReadFile(filepath.Join(tmpDir, ".tpatch", "features", slug, "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(statusBody), `"notes": "landed at`) {
		t.Errorf("land wrote its status note despite refusing:\n%s", statusBody)
	}
	if _, err := os.Stat(liveIndexPath(t, tmpDir) + ".lock"); !os.IsNotExist(err) {
		t.Errorf("index lock residue left behind: %v", err)
	}
}

// A symlinked effective index is refused before anything is staged or
// written.
func TestNestedWorktree_Land_RefusesSymlinkedIndexBeforeMutation(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	indexPath := liveIndexPath(t, tmpDir)
	moved := filepath.Join(t.TempDir(), "real-index")
	if err := os.Rename(indexPath, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(moved, indexPath); err != nil {
		t.Skipf("platform refuses symlinks: %v", err)
	}
	headBefore := gitHead(t, tmpDir)
	statusBefore, err := os.ReadFile(filepath.Join(tmpDir, ".tpatch", "features", slug, "status.json"))
	if err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--no-record")
	if code == 0 {
		t.Fatalf("land must refuse a symlinked index: stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "symlink") {
		t.Errorf("refusal should name the symlink: %q", stderr)
	}
	info, err := os.Lstat(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("the symlink was replaced despite the refusal")
	}
	if got := gitHead(t, tmpDir); got != headBefore {
		t.Errorf("HEAD advanced despite refusing")
	}
	statusAfter, err := os.ReadFile(filepath.Join(tmpDir, ".tpatch", "features", slug, "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(statusAfter) != string(statusBefore) {
		t.Error("status.json changed despite the refusal")
	}
}

// A pre-existing index lock is somebody else's: land refuses, leaves it
// alone, and mutates nothing.
func TestNestedWorktree_Land_RefusesOnLiveIndexLockContention(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	lockPath := liveIndexPath(t, tmpDir) + ".lock"
	if err := os.WriteFile(lockPath, []byte("someone else\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(lockPath)
	headBefore := gitHead(t, tmpDir)

	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go")
	if code == 0 {
		t.Fatalf("land must refuse while another process holds the index lock: stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "another Git process") {
		t.Errorf("refusal should explain the contention: %q", stderr)
	}
	body, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("the foreign lock was removed: %v", err)
	}
	if string(body) != "someone else\n" {
		t.Error("the foreign lock was overwritten")
	}
	if got := gitHead(t, tmpDir); got != headBefore {
		t.Errorf("HEAD advanced despite refusing")
	}
	statusBody, err := os.ReadFile(filepath.Join(tmpDir, ".tpatch", "features", slug, "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(statusBody), `"notes": "landed at`) {
		t.Errorf("land wrote its status note despite refusing:\n%s", statusBody)
	}
}

// Land works when the effective index is redirected through a
// whitespace-bearing GIT_INDEX_FILE, and leaves no residue.
func TestNestedWorktree_Land_WhitespaceBearingGitIndexFile(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	// Seed the redirected index from the real one so the repository
	// state is coherent.
	redirect := filepath.Join(t.TempDir(), " odd\tindex ")
	realIndex := liveIndexPath(t, tmpDir)
	if body, err := os.ReadFile(realIndex); err == nil {
		if werr := os.WriteFile(redirect, body, 0o644); werr != nil {
			t.Fatal(werr)
		}
	}
	t.Setenv("GIT_INDEX_FILE", redirect)

	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go")
	if code != 0 {
		t.Fatalf("land failed with a redirected index: stdout=%q stderr=%q", stdout, stderr)
	}
	committed := nwtGit(t, tmpDir, "show", "--pretty=format:", "--name-only", "HEAD")
	assertNoNestedWorktree(t, "landing commit", committed)
	for _, want := range []string{"README.md", "internal/example.go", "status.json"} {
		if !strings.Contains(committed, want) {
			t.Errorf("landing commit missing %q:\n%s", want, committed)
		}
	}
	if _, err := os.Stat(redirect); err != nil {
		t.Errorf("the redirected index should exist after land: %v", err)
	}
	if _, err := os.Stat(redirect + ".lock"); !os.IsNotExist(err) {
		t.Errorf("lock residue at the redirected path: %v", err)
	}
}

// A clean land leaves no temp-index or lock residue, and the live index
// agrees with the new HEAD.
func TestNestedWorktree_Land_LeavesNoIndexResidue(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go")
	if code != 0 {
		t.Fatalf("land failed: stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "Landed ") || !strings.Contains(stdout, "trailer: Tpatch-Feature: "+slug) {
		t.Errorf("success output changed:\n%s", stdout)
	}
	if _, err := os.Stat(liveIndexPath(t, tmpDir) + ".lock"); !os.IsNotExist(err) {
		t.Errorf("index lock residue: %v", err)
	}
	if staged := nwtGit(t, tmpDir, "diff", "--cached", "--name-only"); strings.TrimSpace(staged) != "" {
		t.Errorf("live index disagrees with the new HEAD: %q", staged)
	}
	msg := gitLastCommitMsg(t, tmpDir)
	for _, want := range []string{"Tpatch-Feature: " + slug, "Tpatch-Patch-SHA: ", "Co-authored-by: Copilot"} {
		if !strings.Contains(msg, want) {
			t.Errorf("commit trailers changed, missing %q:\n%s", want, msg)
		}
	}
}

// fileState.restore must report failures rather than swallow them —
// a silently failed restore leaves a false `landed at` note behind.
func TestFileStateRestoreReportsFailures(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "status.json")
	if err := os.WriteFile(target, []byte("preimage\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := captureFileState(target)
	if err != nil {
		t.Fatal(err)
	}
	if !st.existed || string(st.data) != "preimage\n" || st.mode != 0o600 {
		t.Fatalf("capture = %+v", st)
	}

	// Happy path restores bytes and mode exactly.
	if err := os.WriteFile(target, []byte("mutated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := st.restore(); err != nil {
		t.Fatalf("restore: %v", err)
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "preimage\n" {
		t.Errorf("restore did not put the preimage back: %q", body)
	}
	if info, err := os.Stat(target); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0o600 {
		t.Errorf("mode not restored: %v", info.Mode().Perm())
	}

	// A restore into an unwritable directory must return an error.
	roDir := filepath.Join(dir, "readonly")
	if err := os.Mkdir(roDir, 0o755); err != nil {
		t.Fatal(err)
	}
	roTarget := filepath.Join(roDir, "status.json")
	if err := os.WriteFile(roTarget, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	roState, err := captureFileState(roTarget)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(roDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o755) })
	if err := roState.restore(); err == nil {
		t.Error("restore into an unwritable directory must return an error")
	}

	// An absent preimage restores to absent.
	missing := filepath.Join(dir, "never-existed.json")
	absentState, err := captureFileState(missing)
	if err != nil {
		t.Fatal(err)
	}
	if absentState.existed {
		t.Fatal("capture of a missing file should report absent")
	}
	if err := os.WriteFile(missing, []byte("created\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := absentState.restore(); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Errorf("restore should have removed the file, got %v", err)
	}
}
