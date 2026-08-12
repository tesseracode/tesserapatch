package workflow

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestFilesInPatch(t *testing.T) {
	patch := `diff --git a/foo.txt b/foo.txt
index 111..222 100644
--- a/foo.txt
+++ b/foo.txt
@@ -1 +1 @@
-hi
+hello
diff --git a/bar/baz.go b/bar/baz.go
new file mode 100644
--- /dev/null
+++ b/bar/baz.go
@@ -0,0 +1 @@
+package baz
diff --git a/foo.txt b/foo.txt
`
	got := gitutil.FilesInPatch(patch)
	want := []string{"foo.txt", "bar/baz.go"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("at %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestFilesInPatchEmpty(t *testing.T) {
	if got := gitutil.FilesInPatch(""); len(got) != 0 {
		t.Errorf("expected no files, got %v", got)
	}
}

func TestRefreshAfterAcceptRegeneratesArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	s.AddFeature(store.AddFeatureInput{Title: "Demo", Request: "demo"})
	slug := "demo"

	// Record the upstream commit (the single "initial commit" setupGitRepo makes).
	upstream, err := gitutil.HeadCommit(tmpDir)
	if err != nil {
		t.Fatalf("head: %v", err)
	}

	// Simulate the post-accept working tree: feature touched a new file
	// and modified README.md. The ORIGINAL patch (pre-accept) is what
	// determines which paths the refresh will regenerate against.
	if err := os.WriteFile(filepath.Join(tmpDir, "feature.txt"), []byte("feature content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test\nupdated line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Also create an unrelated untracked file — refresh must NOT pull it in.
	if err := os.WriteFile(filepath.Join(tmpDir, "unrelated.log"), []byte("noise\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	originalPatch := `diff --git a/feature.txt b/feature.txt
new file mode 100644
--- /dev/null
+++ b/feature.txt
@@ -0,0 +1 @@
+feature content
diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1 +1,2 @@
 # Test
+updated line
`
	if err := s.WriteArtifact(slug, "post-apply.patch", originalPatch); err != nil {
		t.Fatal(err)
	}

	if err := RefreshAfterAccept(s, slug, upstream, originalPatch); err != nil {
		t.Fatalf("RefreshAfterAccept: %v", err)
	}

	// 1. post-apply.patch was rewritten and reflects both touched files.
	newPatch, err := s.ReadFeatureFile(slug, "artifacts/post-apply.patch")
	if err != nil {
		t.Fatalf("read new patch: %v", err)
	}
	if !strings.Contains(newPatch, "feature.txt") {
		t.Errorf("refreshed patch missing feature.txt:\n%s", newPatch)
	}
	if !strings.Contains(newPatch, "README.md") {
		t.Errorf("refreshed patch missing README.md:\n%s", newPatch)
	}
	if strings.Contains(newPatch, "unrelated.log") {
		t.Errorf("refreshed patch leaked untracked file:\n%s", newPatch)
	}

	// 2. A numbered reconcile patch exists in patches/.
	entries, err := os.ReadDir(filepath.Join(tmpDir, ".tpatch", "features", slug, "patches"))
	if err != nil {
		t.Fatalf("readdir patches: %v", err)
	}
	found := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "-reconcile.patch") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a patches/NNN-reconcile.patch snapshot; got %v", entries)
	}

	manifest, err := store.LoadPatchGenerations(s, slug)
	if err != nil {
		t.Fatalf("LoadPatchGenerations: %v", err)
	}
	if len(manifest.Generations) != 1 || manifest.Generations[0].Kind != "reconcile" || manifest.Generations[0].AuditPatch != "patches/001-reconcile.patch" {
		t.Fatalf("expected reconcile generation append, got %+v", manifest)
	}
}

func TestRefreshAfterAccept_WarnsOnAppendFailure(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	s.AddFeature(store.AddFeatureInput{Title: "Demo", Request: "demo"})
	slug := "demo"

	upstream, err := gitutil.HeadCommit(tmpDir)
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test\nupdated line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	originalPatch := `diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1 +1,2 @@
 # Test
+old line
`
	if err := s.WriteArtifact(slug, "post-apply.patch", originalPatch); err != nil {
		t.Fatal(err)
	}
	recipePath := filepath.Join(tmpDir, ".tpatch", "features", slug, "artifacts", "apply-recipe.json")
	if err := os.WriteFile(recipePath, []byte(`{"operations":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(recipePath, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(recipePath, 0o644)

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	err = RefreshAfterAccept(s, slug, upstream, originalPatch)
	_ = w.Close()
	os.Stderr = oldStderr
	stderrBytes, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		t.Fatalf("read stderr: %v", readErr)
	}
	if err != nil {
		t.Fatalf("RefreshAfterAccept: %v", err)
	}

	newPatch, err := s.ReadFeatureFile(slug, "artifacts/post-apply.patch")
	if err != nil {
		t.Fatalf("read new patch: %v", err)
	}
	if newPatch == originalPatch || !strings.Contains(newPatch, "updated line") {
		t.Fatalf("post-apply.patch was not refreshed:\n%s", newPatch)
	}
	if _, err := os.Stat(s.PatchGenerationsPath(slug)); !os.IsNotExist(err) {
		t.Fatalf("patch-generations.json should not exist after append failure, err=%v", err)
	}
	stderr := string(stderrBytes)
	if !strings.Contains(stderr, "warning") || !strings.Contains(stderr, "patch-generations.json") {
		t.Fatalf("expected patch-generations warning on stderr, got %q", stderr)
	}
}

func TestRefreshAfterAccept_WarnsOnUnreadableManifest(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	s.AddFeature(store.AddFeatureInput{Title: "Demo", Request: "demo"})
	slug := "demo"

	upstream, err := gitutil.HeadCommit(tmpDir)
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test\nupdated line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	originalPatch := `diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1 +1,2 @@
 # Test
+old line
`
	if err := s.WriteArtifact(slug, "post-apply.patch", originalPatch); err != nil {
		t.Fatal(err)
	}
	patchSHA := store.SHA256HexString(originalPatch)
	gen := store.PatchGeneration{
		Generation:          1,
		Kind:                "record",
		PatchSHA256:         patchSHA,
		GitPatchID:          strings.Repeat("b", 40),
		GitPatchIDAlgorithm: store.PatchIDAlgorithmStable,
		RecipeSHA256:        "",
		CanonicalPatch:      "artifacts/post-apply.patch",
		AuditPatch:          "patches/001-record.patch",
		BaseCommit:          upstream,
		Upper:               store.GenerationUpper{Kind: "working-tree", Ref: "working-tree", Commit: ""},
		Capture:             store.GenerationCapture{Mode: "working-tree-all", Pathspecs: []string{"README.md"}, ClaimIDs: []string{}},
		TouchedPaths:        []string{"README.md"},
		Dependencies:        []store.GenerationDependency{},
		Refs:                &store.GenerationRefs{},
	}
	gen.GenerationID = store.ComputeGenerationID(slug, gen.Generation, gen.PatchSHA256, gen.RecipeSHA256, gen.BaseCommit, gen.Upper, gen.Capture)
	manifest := store.PatchGenerationsManifest{Version: store.PatchGenerationsManifestVersion, Feature: slug, CurrentGeneration: 1, Generations: []store.PatchGeneration{gen}}
	if err := store.SavePatchGenerations(s, manifest); err != nil {
		t.Fatalf("SavePatchGenerations: %v", err)
	}
	manifestPath := s.PatchGenerationsPath(slug)
	if err := os.Chmod(manifestPath, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(manifestPath, 0o644)

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	err = RefreshAfterAccept(s, slug, upstream, originalPatch)
	_ = w.Close()
	os.Stderr = oldStderr
	stderrBytes, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		t.Fatalf("read stderr: %v", readErr)
	}
	if err != nil {
		t.Fatalf("RefreshAfterAccept: %v", err)
	}

	newPatch, err := s.ReadFeatureFile(slug, "artifacts/post-apply.patch")
	if err != nil {
		t.Fatalf("read new patch: %v", err)
	}
	if newPatch == originalPatch || !strings.Contains(newPatch, "updated line") {
		t.Fatalf("post-apply.patch was not refreshed:\n%s", newPatch)
	}
	stderr := string(stderrBytes)
	if !strings.Contains(stderr, "warning") || !strings.Contains(stderr, "patch-generations.json") {
		t.Fatalf("expected patch-generations warning on stderr, got %q", stderr)
	}
}

// TestRefreshAfterAcceptLeavesIndexClean guards the v0.5.2 fix for
// finding #2: DiffFromCommitForPaths (used by RefreshAfterAccept) used
// to run `git add -N` against the REAL .git/index, leaving intent-to-add
// entries in the user's working state after reconcile --accept. The
// fix routes intent-to-add through GIT_INDEX_FILE to a throwaway index.
// `git status --porcelain` must be byte-identical before and after.

func TestRefreshAfterAcceptLeavesIndexClean(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	s.AddFeature(store.AddFeatureInput{Title: "IdxClean", Request: "demo"})
	slug := "idxclean"

	upstream, err := gitutil.HeadCommit(tmpDir)
	if err != nil {
		t.Fatalf("head: %v", err)
	}

	// Simulate post-accept working tree: new untracked file + modified
	// tracked file. The untracked file is what forced the old code to
	// call `git add -N` on the real index.
	if err := os.WriteFile(filepath.Join(tmpDir, "new.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test\nmore\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	originalPatch := `diff --git a/new.txt b/new.txt
new file mode 100644
--- /dev/null
+++ b/new.txt
@@ -0,0 +1 @@
+hi
diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1 +1,2 @@
 # Test
+more
`
	if err := s.WriteArtifact(slug, "post-apply.patch", originalPatch); err != nil {
		t.Fatal(err)
	}

	statusBefore := gitStatusPorcelain(t, tmpDir)

	if err := RefreshAfterAccept(s, slug, upstream, originalPatch); err != nil {
		t.Fatalf("RefreshAfterAccept: %v", err)
	}

	statusAfter := gitStatusPorcelain(t, tmpDir)
	if statusBefore != statusAfter {
		t.Errorf("index dirtied by refresh.\nbefore:\n%s\nafter:\n%s", statusBefore, statusAfter)
	}

	// Second safeguard: no intent-to-add marker ('A ' with hollow content)
	// should appear. An `ls-files --stage` check catches the specific
	// regression: intent-to-add entries show the all-zeroes SHA.
	lsFiles := gitLsFilesStage(t, tmpDir)
	if strings.Contains(lsFiles, "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391") && strings.Contains(lsFiles, "new.txt") {
		// empty-blob sha is fine; intent-to-add uses all-zero sha.
	}
	if strings.Contains(lsFiles, "0000000000000000000000000000000000000000\t") {
		t.Errorf("intent-to-add entry leaked into real index:\n%s", lsFiles)
	}
}

func gitStatusPorcelain(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	return string(out)
}

func gitLsFilesStage(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "--stage")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	return string(out)
}

func TestForwardApplyExcluding(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	// Two-file patch: one we'll exclude, one should land.
	patch := `diff --git a/a.txt b/a.txt
new file mode 100644
--- /dev/null
+++ b/a.txt
@@ -0,0 +1 @@
+A
diff --git a/b.txt b/b.txt
new file mode 100644
--- /dev/null
+++ b/b.txt
@@ -0,0 +1 @@
+B
`
	if err := gitutil.ForwardApplyExcluding(tmpDir, patch, []string{"a.txt"}); err != nil {
		t.Fatalf("ForwardApplyExcluding: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "a.txt")); !os.IsNotExist(err) {
		t.Errorf("a.txt should have been excluded, but it exists (err=%v)", err)
	}
	if content, err := os.ReadFile(filepath.Join(tmpDir, "b.txt")); err != nil || strings.TrimSpace(string(content)) != "B" {
		t.Errorf("b.txt should have been applied; got content=%q err=%v", content, err)
	}
}

// ─── GH #7 rev-2 F3: nested-worktree filtering in the refresh ───────

// refreshAddWorktree registers a linked worktree under repoRoot and
// removes it (plus prune) before temp-root teardown.
func refreshAddWorktree(t *testing.T, repoRoot, rel, branch string) string {
	t.Helper()
	abs := filepath.Join(repoRoot, filepath.FromSlash(rel))
	c := exec.Command("git", "worktree", "add", "-q", abs, "-b", branch)
	c.Dir = repoRoot
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %s: %v", out, err)
	}
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

// A stale nested-worktree gitlink recorded in the ORIGINAL patch must
// not survive into the refreshed canonical patch, the numbered
// reconcile snapshot, or the generation metadata.
func TestRefreshAfterAcceptExcludesStaleNestedWorktreePaths(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	s.AddFeature(store.AddFeatureInput{Title: "Demo", Request: "demo"})
	slug := "demo"

	upstream, err := gitutil.HeadCommit(tmpDir)
	if err != nil {
		t.Fatalf("head: %v", err)
	}

	wtRel := ".claude/worktrees/agent review"
	refreshAddWorktree(t, tmpDir, wtRel, "agent-review")

	if err := os.WriteFile(filepath.Join(tmpDir, "feature.txt"), []byte("feature content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test\nupdated line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The original patch carries a stale mode-160000 gitlink for the
	// nested worktree, exactly as a pre-fix tpatch would have recorded.
	originalPatch := `diff --git a/feature.txt b/feature.txt
new file mode 100644
--- /dev/null
+++ b/feature.txt
@@ -0,0 +1 @@
+feature content
diff --git a/.claude/worktrees/agent review b/.claude/worktrees/agent review
new file mode 160000
--- /dev/null
+++ b/.claude/worktrees/agent review
@@ -0,0 +1 @@
+Subproject commit 0000000000000000000000000000000000000000
diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1 +1,2 @@
 # Test
+updated line
`
	if err := s.WriteArtifact(slug, "post-apply.patch", originalPatch); err != nil {
		t.Fatal(err)
	}

	if err := RefreshAfterAccept(s, slug, upstream, originalPatch); err != nil {
		t.Fatalf("RefreshAfterAccept: %v", err)
	}

	newPatch, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(newPatch, "agent review") || strings.Contains(newPatch, "160000") {
		t.Errorf("refreshed canonical patch kept the stale nested-worktree gitlink:\n%s", newPatch)
	}
	for _, want := range []string{"feature.txt", "README.md", "+updated line"} {
		if !strings.Contains(newPatch, want) {
			t.Errorf("refreshed canonical patch dropped %q:\n%s", want, newPatch)
		}
	}

	// Numbered reconcile snapshot mirrors the canonical patch.
	entries, err := os.ReadDir(filepath.Join(tmpDir, ".tpatch", "features", slug, "patches"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), "-reconcile.patch") {
			continue
		}
		found = true
		body, rerr := os.ReadFile(filepath.Join(tmpDir, ".tpatch", "features", slug, "patches", e.Name()))
		if rerr != nil {
			t.Fatal(rerr)
		}
		if strings.Contains(string(body), "agent review") || strings.Contains(string(body), "160000") {
			t.Errorf("numbered reconcile patch %s kept the nested worktree:\n%s", e.Name(), body)
		}
	}
	if !found {
		t.Error("no numbered reconcile patch written")
	}

	// Generation metadata records the FILTERED pathspec set.
	gens, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "patch-generations.json"))
	if err != nil {
		t.Fatalf("read patch-generations.json: %v", err)
	}
	if strings.Contains(gens, "agent review") {
		t.Errorf("generation metadata kept the nested-worktree pathspec:\n%s", gens)
	}
	for _, want := range []string{"feature.txt", "README.md"} {
		if !strings.Contains(gens, want) {
			t.Errorf("generation metadata dropped %q:\n%s", want, gens)
		}
	}
}

// If EVERY path in the original patch was a nested worktree, the
// refresh must regenerate nothing rather than broadening to a
// full-tree diff that would sweep unrelated working-tree state in.
func TestRefreshAfterAcceptWorktreeOnlyOriginalPatchRegeneratesNothing(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	s.AddFeature(store.AddFeatureInput{Title: "Demo", Request: "demo"})
	slug := "demo"
	upstream, err := gitutil.HeadCommit(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	wtRel := ".claude/worktrees/agent review"
	refreshAddWorktree(t, tmpDir, wtRel, "agent-review")
	// Unrelated real change a broadened full-tree diff WOULD capture.
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test\nunrelated\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	originalPatch := `diff --git a/.claude/worktrees/agent review b/.claude/worktrees/agent review
new file mode 160000
--- /dev/null
+++ b/.claude/worktrees/agent review
@@ -0,0 +1 @@
+Subproject commit 0000000000000000000000000000000000000000
`
	if err := s.WriteArtifact(slug, "post-apply.patch", originalPatch); err != nil {
		t.Fatal(err)
	}
	if err := RefreshAfterAccept(s, slug, upstream, originalPatch); err != nil {
		t.Fatalf("RefreshAfterAccept: %v", err)
	}
	newPatch, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(newPatch) != "" {
		t.Fatalf("worktree-only original patch must regenerate nothing, got:\n%s", newPatch)
	}
}

// Discovery failure must be fail-closed and must happen before any
// artifact mutation.
func TestRefreshAfterAcceptFailsClosedBeforeMutation(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	s.AddFeature(store.AddFeatureInput{Title: "Demo", Request: "demo"})
	slug := "demo"
	upstream, err := gitutil.HeadCommit(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	originalPatch := `diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1 +1,2 @@
 # Test
+updated line
`
	if err := s.WriteArtifact(slug, "post-apply.patch", originalPatch); err != nil {
		t.Fatal(err)
	}
	featureDir := filepath.Join(tmpDir, ".tpatch", "features", slug)
	before := snapshotTreeForRefresh(t, featureDir)

	installFailingWorktreeListGitForRefresh(t)

	err = RefreshAfterAccept(s, slug, upstream, originalPatch)
	if err == nil {
		t.Fatal("RefreshAfterAccept must fail closed when worktree discovery fails")
	}
	if !errors.Is(err, gitutil.ErrNestedWorktreeDiscovery) {
		t.Errorf("failure not in the fail-closed class: %v", err)
	}
	after := snapshotTreeForRefresh(t, featureDir)
	if len(before) != len(after) {
		t.Fatalf("feature directory changed across a refusal: %d -> %d files", len(before), len(after))
	}
	for k, v := range before {
		if after[k] != v {
			t.Errorf("%s changed across a refusal", k)
		}
	}
}

func snapshotTreeForRefresh(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
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
	return out
}

func installFailingWorktreeListGitForRefresh(t *testing.T) {
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
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// ─── GH #7 rev-3 F2: strict header parsing in the refresh ───────────

// gitCapture runs git capturing stdout only.
func gitCapture(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

func gitMust(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}

// realQuotedGitlinkPatch registers a newline-named nested worktree,
// stages it, and returns the REAL Git-generated diff — whose header is
// C-quoted. Skips when the platform refuses the name.
func realQuotedGitlinkPatch(t *testing.T, repoRoot, rel string) string {
	t.Helper()
	abs := filepath.Join(repoRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	add := exec.Command("git", "worktree", "add", "-q", abs, "-b", "wt-quoted")
	add.Dir = repoRoot
	if out, err := add.CombinedOutput(); err != nil {
		t.Skipf("platform/Git refuses this worktree path: %s: %v", out, err)
	}
	t.Cleanup(func() {
		rm := exec.Command("git", "worktree", "remove", "--force", abs)
		rm.Dir = repoRoot
		_ = rm.Run()
		prune := exec.Command("git", "worktree", "prune")
		prune.Dir = repoRoot
		_ = prune.Run()
	})
	gitMust(t, repoRoot, "-c", "advice.addEmbeddedRepo=false",
		"--literal-pathspecs", "add", "--", rel)
	patch := gitCapture(t, repoRoot, "diff", "--cached", "HEAD")
	gitMust(t, repoRoot, "--literal-pathspecs", "reset", "-q", "--", rel)
	if !strings.Contains(patch, "160000") {
		t.Fatalf("fixture did not produce a gitlink entry:\n%q", patch)
	}
	return patch
}

// The F2 regression: a stale, REAL Git-generated, C-quoted
// worktree-only patch must regenerate NOTHING. Before strict parsing
// the header was skipped, the scope became empty, and an empty scope
// means "diff everything" — sweeping unrelated working-tree dirt into
// post-apply.patch.
func TestRefreshAfterAcceptQuotedWorktreeOnlyPatchDoesNotCaptureUnrelatedDirt(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	s.AddFeature(store.AddFeatureInput{Title: "Demo", Request: "demo"})
	slug := "demo"
	upstream, err := gitutil.HeadCommit(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	originalPatch := realQuotedGitlinkPatch(t, tmpDir, ".claude/worktrees/new\nline")
	if !strings.Contains(originalPatch, `diff --git "a/`) {
		t.Fatalf("fixture header is not C-quoted:\n%q", originalPatch)
	}

	// Unrelated working-tree dirt an over-broad diff WOULD capture.
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test\nunrelated dirt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "stray.txt"), []byte("stray\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := s.WriteArtifact(slug, "post-apply.patch", originalPatch); err != nil {
		t.Fatal(err)
	}
	if err := RefreshAfterAccept(s, slug, upstream, originalPatch); err != nil {
		t.Fatalf("RefreshAfterAccept: %v", err)
	}

	newPatch, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(newPatch) != "" {
		t.Fatalf("quoted worktree-only patch must regenerate nothing, got:\n%s", newPatch)
	}
	for _, dirt := range []string{"unrelated dirt", "stray.txt"} {
		if strings.Contains(newPatch, dirt) {
			t.Errorf("refresh captured unrelated dirt %q", dirt)
		}
	}
	// Numbered snapshot and generation metadata must be equally clean.
	entries, _ := os.ReadDir(filepath.Join(tmpDir, ".tpatch", "features", slug, "patches"))
	for _, e := range entries {
		body, _ := os.ReadFile(filepath.Join(tmpDir, ".tpatch", "features", slug, "patches", e.Name()))
		for _, dirt := range []string{"unrelated dirt", "stray.txt", "160000"} {
			if strings.Contains(string(body), dirt) {
				t.Errorf("numbered patch %s contains %q", e.Name(), dirt)
			}
		}
	}
	if gens, gerr := s.ReadFeatureFile(slug, filepath.Join("artifacts", "patch-generations.json")); gerr == nil {
		for _, dirt := range []string{"stray.txt", "worktrees"} {
			if strings.Contains(gens, dirt) {
				t.Errorf("generation metadata contains %q:\n%s", dirt, gens)
			}
		}
	}
}

// Mixed: a quoted worktree entry plus an intended path retains only the
// intended path.
func TestRefreshAfterAcceptQuotedWorktreePlusIntendedPathKeepsOnlyIntended(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	s.AddFeature(store.AddFeatureInput{Title: "Demo", Request: "demo"})
	slug := "demo"
	upstream, err := gitutil.HeadCommit(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	gitlinkPatch := realQuotedGitlinkPatch(t, tmpDir, ".claude/worktrees/new\nline")

	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test\nintended\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitMust(t, tmpDir, "add", "README.md")
	intendedPatch := gitCapture(t, tmpDir, "diff", "--cached", "HEAD")
	gitMust(t, tmpDir, "reset", "-q", "--", "README.md")
	if err := os.WriteFile(filepath.Join(tmpDir, "stray.txt"), []byte("stray\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	originalPatch := gitlinkPatch + intendedPatch
	if err := s.WriteArtifact(slug, "post-apply.patch", originalPatch); err != nil {
		t.Fatal(err)
	}
	if err := RefreshAfterAccept(s, slug, upstream, originalPatch); err != nil {
		t.Fatalf("RefreshAfterAccept: %v", err)
	}

	newPatch, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(newPatch, "README.md") || !strings.Contains(newPatch, "+intended") {
		t.Errorf("refreshed patch dropped the intended path:\n%s", newPatch)
	}
	if strings.Contains(newPatch, "160000") || strings.Contains(newPatch, "worktrees") {
		t.Errorf("refreshed patch kept the nested worktree:\n%s", newPatch)
	}
	if strings.Contains(newPatch, "stray.txt") {
		t.Errorf("refreshed patch swept in unrelated dirt:\n%s", newPatch)
	}
}

// A malformed header must refuse before ANY write.
func TestRefreshAfterAcceptMalformedHeaderFailsBeforeWrites(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	s.AddFeature(store.AddFeatureInput{Title: "Demo", Request: "demo"})
	slug := "demo"
	upstream, err := gitutil.HeadCommit(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	malformed := "diff --git \"a/broken.txt b/broken.txt\nindex 111..222 100644\n--- a/broken.txt\n+++ b/broken.txt\n@@ -1 +1 @@\n-a\n+b\n"
	if err := s.WriteArtifact(slug, "post-apply.patch", malformed); err != nil {
		t.Fatal(err)
	}
	featureDir := filepath.Join(tmpDir, ".tpatch", "features", slug)
	before := snapshotTreeForRefresh(t, featureDir)

	err = RefreshAfterAccept(s, slug, upstream, malformed)
	if err == nil {
		t.Fatal("a malformed diff header must refuse")
	}
	if !strings.Contains(err.Error(), "which paths") {
		t.Errorf("refusal should name the parse failure: %v", err)
	}
	after := snapshotTreeForRefresh(t, featureDir)
	if len(before) != len(after) {
		t.Fatalf("feature directory changed across a refusal: %d -> %d files", len(before), len(after))
	}
	for k, v := range before {
		if after[k] != v {
			t.Errorf("%s changed across a refusal", k)
		}
	}
}
