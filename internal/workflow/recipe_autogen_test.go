package workflow

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// setupAutogenStore creates a tpatch store at a tempdir, registers the
// slug as a feature, and writes the given files into the working tree
// (post-state). Returns the store and a representative patch string
// covering those files.
func setupAutogenStore(t *testing.T, slug string, files map[string]string) *store.Store {
	t.Helper()
	tmp := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"}, {"-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "base"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmp
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".tpatch", "features", slug, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range files {
		full := filepath.Join(tmp, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := &store.Store{Root: tmp}
	return s
}

func captureRecipeForTest(root, slug, patch string) patchobs.Observation {
	return patchobs.Observe(patchobs.Input{
		Producer: patchobs.ProducerRecord, RepoRoot: root, Slug: slug,
		Patch: patch, PatchPresent: true, PreimageRef: "HEAD",
		Capture: patchobs.CaptureDescriptor{Mode: patchobs.CaptureModeWorkingTreeAll},
	})
}

func recipeFromWorktreeForTest(root, slug, patch string) (ApplyRecipe, []string, error) {
	return RecipeFromPatch(captureRecipeForTest(root, slug, patch))
}

func autogenForTest(s *store.Store, slug, patch string, autogen, regenerate bool) (AutogenOutcome, error) {
	return AutogenRecipeForRecord(s, captureRecipeForTest(s.Root, slug, patch), autogen, regenerate)
}

func newFilePatch(paths ...string) string {
	var b strings.Builder
	for _, p := range paths {
		b.WriteString("diff --git a/" + p + " b/" + p + "\n")
		b.WriteString("new file mode 100644\n")
		b.WriteString("--- /dev/null\n")
		b.WriteString("+++ b/" + p + "\n")
		b.WriteString("@@ -0,0 +1 @@\n+x\n")
	}
	return b.String()
}

func deletePatch(path string) string {
	return "diff --git a/" + path + " b/" + path + "\n" +
		"deleted file mode 100644\n" +
		"--- a/" + path + "\n" +
		"+++ /dev/null\n" +
		"@@ -1 +0,0 @@\n-old\n"
}

// TestPatchEffectViews replaces TestParsePatchTouchedFiles: GH #15 S1
// removed `parsePatchTouchedFiles` (PI-1) and the derivation now projects
// the strict normalized effect set. The three rows below are the same
// three the removed splitter was measured on, so the migration is a
// like-for-like comparison; the fourth row is the behaviour that is new,
// and deliberately so.
func TestPatchEffectViews(t *testing.T) {
	patch := newFilePatch("a.txt", "sub/b.go") + deletePatch("c.md")
	got, err := patchEffectViews(patch)
	if err != nil {
		t.Fatalf("patchEffectViews: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 entries, got %d: %+v", len(got), got)
	}
	if got[0].Path != "a.txt" || got[0].ChangeKind != gitutil.ChangeKindAdd {
		t.Errorf("entry 0: %+v", got[0])
	}
	if got[1].Path != "sub/b.go" || got[1].ChangeKind != gitutil.ChangeKindAdd {
		t.Errorf("entry 1: %+v", got[1])
	}
	if got[2].Path != "c.md" || got[2].ChangeKind != gitutil.ChangeKindDelete {
		t.Errorf("entry 2: %+v", got[2])
	}

	// The removed splitter silently dropped a header it could not read.
	// The adapter refuses instead, and it may not return a partial view.
	views, err := patchEffectViews("diff --git a/x.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n")
	if err == nil {
		t.Fatalf("a truncated header must be refused, got %+v", views)
	}
	if views != nil {
		t.Fatalf("a refusal must not return a partial view: %+v", views)
	}
}

func TestRecipeFromPatch_NewAndModifiedFiles(t *testing.T) {
	slug := "feat-x"
	s := setupAutogenStore(t, slug, map[string]string{
		"a.txt":    "alpha\n",
		"sub/b.go": "package main\n",
	})
	patch := newFilePatch("a.txt", "sub/b.go")

	recipe, skipped, err := recipeFromWorktreeForTest(s.Root, slug, patch)
	if err != nil {
		t.Fatalf("RecipeFromPatch: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("unexpected skipped: %v", skipped)
	}
	if len(recipe.Operations) != 2 {
		t.Fatalf("want 2 ops, got %d", len(recipe.Operations))
	}
	// Determinism: alphabetical.
	if recipe.Operations[0].Path != "a.txt" || recipe.Operations[1].Path != "sub/b.go" {
		t.Errorf("ops not sorted: %+v", recipe.Operations)
	}
	for _, op := range recipe.Operations {
		if op.Type != "write-file" {
			t.Errorf("op %s: type=%q want write-file", op.Path, op.Type)
		}
	}
	if recipe.Operations[0].Content != "alpha\n" {
		t.Errorf("a.txt content mismatch: %q", recipe.Operations[0].Content)
	}
}

func TestRecipeFromPatch_DeletedFileSkipped(t *testing.T) {
	slug := "feat-y"
	s := setupAutogenStore(t, slug, map[string]string{"keeper.txt": "k\n"})
	patch := newFilePatch("keeper.txt") + deletePatch("gone.md")

	recipe, skipped, err := recipeFromWorktreeForTest(s.Root, slug, patch)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(recipe.Operations) != 0 {
		t.Errorf("partial recipe must be withheld, got %+v", recipe.Operations)
	}
	if len(skipped) != 1 || !strings.Contains(skipped[0], "gone.md") || !strings.Contains(skipped[0], "effect-delete-unsupported") {
		t.Errorf("expected skipped deletion, got %v", skipped)
	}
}

func TestAutogenRecipeForRecord_GeneratesWhenMissing(t *testing.T) {
	slug := "feat-gen"
	s := setupAutogenStore(t, slug, map[string]string{"a.txt": "A\n"})
	patch := newFilePatch("a.txt")

	outcome, err := autogenForTest(s, slug, patch, true, false)
	action, skipped := outcome.Action, outcome.SkippedPaths
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if action != AutogenGenerated {
		t.Fatalf("action=%s want generated", action)
	}
	if len(skipped) != 0 {
		t.Errorf("skipped: %v", skipped)
	}
	got, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "apply-recipe.json"))
	if err != nil {
		t.Fatalf("recipe not written: %v", err)
	}
	var r ApplyRecipe
	if jerr := json.Unmarshal([]byte(got), &r); jerr != nil {
		t.Fatalf("recipe invalid JSON: %v", jerr)
	}
	if len(r.Operations) != 1 || r.Operations[0].Path != "a.txt" {
		t.Errorf("recipe contents: %+v", r)
	}
}

func TestAutogenRecipeForRecord_SkipsWhenAutogenOff(t *testing.T) {
	slug := "feat-off"
	s := setupAutogenStore(t, slug, map[string]string{"a.txt": "A\n"})
	patch := newFilePatch("a.txt")

	outcome, err := autogenForTest(s, slug, patch, false, false)
	action := outcome.Action
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if action != AutogenSkipped {
		t.Fatalf("action=%s want skipped", action)
	}
	if _, rerr := s.ReadFeatureFile(slug, filepath.Join("artifacts", "apply-recipe.json")); rerr == nil {
		t.Errorf("recipe should not have been written when --no-recipe-autogen")
	}
}

func TestAutogenRecipeForRecord_NoopWhenRecipeMatches(t *testing.T) {
	slug := "feat-match"
	s := setupAutogenStore(t, slug, map[string]string{"a.txt": "A\n"})
	patch := newFilePatch("a.txt")
	// Pre-existing recipe with the same file.
	pre := ApplyRecipe{Feature: slug, Operations: []RecipeOperation{
		{Type: "replace-in-file", Path: "a.txt", Search: "B", Replace: "A"},
	}}
	data, _ := json.MarshalIndent(pre, "", "  ")
	if err := s.WriteArtifact(slug, "apply-recipe.json", string(data)+"\n"); err != nil {
		t.Fatal(err)
	}

	outcome, err := autogenForTest(s, slug, patch, true, false)
	action := outcome.Action
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if action != AutogenStale {
		t.Fatalf("action=%s want preserved/stale without origin proof", action)
	}
	got, _ := s.ReadFeatureFile(slug, filepath.Join("artifacts", "apply-recipe.json"))
	if !strings.Contains(got, "replace-in-file") {
		t.Errorf("existing richer recipe was overwritten: %s", got)
	}
}

func TestAutogenRecipeForRecord_StaleWhenRecipeDrifts(t *testing.T) {
	slug := "feat-drift"
	s := setupAutogenStore(t, slug, map[string]string{"a.txt": "A\n", "b.txt": "B\n"})
	// Recipe only mentions a.txt; patch touches a.txt + b.txt.
	pre := ApplyRecipe{Feature: slug, Operations: []RecipeOperation{
		{Type: "write-file", Path: "a.txt", Content: "A\n"},
	}}
	data, _ := json.MarshalIndent(pre, "", "  ")
	s.WriteArtifact(slug, "apply-recipe.json", string(data)+"\n")
	patch := newFilePatch("a.txt", "b.txt")

	outcome, err := autogenForTest(s, slug, patch, true, false)
	action, reason := outcome.Action, outcome.DriftReason
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if action != AutogenStale {
		t.Fatalf("action=%s want stale", action)
	}
	if !strings.Contains(reason, "recipe bytes differ") {
		t.Errorf("reason should report total-byte drift: %q", reason)
	}
	// Sidecar must be written and the recipe must NOT be overwritten.
	got, _ := s.ReadFeatureFile(slug, filepath.Join("artifacts", "apply-recipe.json"))
	if strings.Contains(got, "b.txt") {
		t.Errorf("recipe was overwritten despite regenerate=false: %s", got)
	}
	side, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "recipe-stale.json"))
	if err != nil {
		t.Fatalf("recipe-stale.json missing: %v", err)
	}
	if !strings.Contains(side, "\"stale\": true") {
		t.Errorf("sidecar missing stale flag: %s", side)
	}
}

func TestAutogenRecipeForRecord_RegenerateOverwrites(t *testing.T) {
	slug := "feat-regen"
	s := setupAutogenStore(t, slug, map[string]string{"a.txt": "A\n", "b.txt": "B\n"})
	pre := ApplyRecipe{Feature: slug, Operations: []RecipeOperation{
		{Type: "write-file", Path: "a.txt", Content: "A\n"},
	}}
	data, _ := json.MarshalIndent(pre, "", "  ")
	s.WriteArtifact(slug, "apply-recipe.json", string(data)+"\n")
	// Pre-existing stale sidecar to verify it gets cleared.
	s.WriteArtifact(slug, "recipe-stale.json", `{"stale": true}`+"\n")

	patch := newFilePatch("a.txt", "b.txt")
	outcome, err := autogenForTest(s, slug, patch, true, true)
	action := outcome.Action
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if action != AutogenRegenerated {
		t.Fatalf("action=%s want regenerated", action)
	}
	got, _ := s.ReadFeatureFile(slug, filepath.Join("artifacts", "apply-recipe.json"))
	if !strings.Contains(got, "b.txt") {
		t.Errorf("recipe should now reference b.txt: %s", got)
	}
	if _, rerr := s.ReadFeatureFile(slug, filepath.Join("artifacts", "recipe-stale.json")); rerr == nil {
		t.Errorf("stale sidecar should have been cleared after regenerate")
	}
}

func TestAutogenRecipeForRecord_ClearsStaleWhenAligned(t *testing.T) {
	slug := "feat-clear"
	s := setupAutogenStore(t, slug, map[string]string{"a.txt": "A\n"})
	pre := ApplyRecipe{Feature: slug, Operations: []RecipeOperation{
		{Type: "write-file", Path: "a.txt", Content: "A\n", PreimageHash: new(string)},
	}}
	data, _ := json.MarshalIndent(pre, "", "  ")
	s.WriteArtifact(slug, "apply-recipe.json", string(data)+"\n")
	s.WriteArtifact(slug, "recipe-stale.json", `{"stale": true}`+"\n")

	patch := newFilePatch("a.txt")
	outcome, err := autogenForTest(s, slug, patch, true, false)
	action := outcome.Action
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if action != AutogenNoop {
		t.Fatalf("action=%s want noop", action)
	}
	if _, rerr := s.ReadFeatureFile(slug, filepath.Join("artifacts", "recipe-stale.json")); rerr == nil {
		t.Errorf("stale sidecar should be cleared once recipe matches captured patch again")
	}
}

func TestAutogenRecipeOpsValidateAgainstSchema(t *testing.T) {
	// Belt-and-braces: confirm autogen-produced ops use only types
	// recognised by the parity-guard schema (write-file).
	slug := "feat-schema"
	s := setupAutogenStore(t, slug, map[string]string{"a.txt": "A\n", "b.txt": "B\n"})
	patch := newFilePatch("a.txt", "b.txt")

	recipe, _, err := recipeFromWorktreeForTest(s.Root, slug, patch)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"write-file":       true,
		"replace-in-file":  true,
		"append-file":      true,
		"ensure-directory": true,
	}
	for _, op := range recipe.Operations {
		if !allowed[op.Type] {
			t.Errorf("op type %q not in parity-guard allowlist", op.Type)
		}
	}
}
