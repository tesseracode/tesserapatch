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
