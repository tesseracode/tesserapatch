package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

func TestFeaturePatchRefreshAndFixupWriteAmendGenerations(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "Patch amend")
	slug := "patch-amend"
	if _, stderr, code := recordSameContent(t, tmpDir, slug, baseSha); code != 0 {
		t.Fatalf("first record failed: %s", stderr)
	}
	first := loadPatchGenerationsForTest(t, tmpDir, slug).Generations[0]
	s, err := store.Open(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatal(err)
	}
	st.Verify = &store.VerifyRecord{
		VerifiedAt:         "2026-05-22T00:00:00Z",
		Passed:             true,
		RecipeHashAtVerify: featureArtifactHash(t, tmpDir, slug, "apply-recipe.json"),
		PatchHashAtVerify:  featureArtifactHash(t, tmpDir, slug, "post-apply.patch"),
	}
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "src", "a.txt"), []byte("a refresh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runCmdWithError("feature", "patch", "refresh", "--path", tmpDir, slug, "--reason", "complete case"); code != 0 {
		t.Fatalf("feature patch refresh failed: %s", stderr)
	}
	m := loadPatchGenerationsForTest(t, tmpDir, slug)
	if len(m.Generations) != 2 || m.Generations[1].Kind != store.PatchGenerationKindAmendRefresh || m.Generations[1].Reason != "complete case" {
		t.Fatalf("refresh generation mismatch: %+v", m.Generations)
	}
	if m.Generations[0].GenerationID != first.GenerationID || m.Generations[0].CanonicalPatch != "" || m.Generations[1].CanonicalPatch != "artifacts/post-apply.patch" {
		t.Fatalf("canonical generation history mismatch: %+v", m.Generations)
	}
	st, err = s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatal(err)
	}
	if got := workflow.DeriveFreshnessLabel(s, st); got != store.LabelVerifiedStale {
		t.Fatalf("patch-content amend should stale verify by patch/recipe hash input, got %q", got)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "src", "a.txt"), []byte("a fixup\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := m.Generations[1].GenerationID
	if _, stderr, code := runCmdWithError("feature", "patch", "fixup", "--path", tmpDir, slug, "--target", target, "--reason", "cover empty response"); code != 0 {
		t.Fatalf("feature patch fixup failed: %s", stderr)
	}
	m = loadPatchGenerationsForTest(t, tmpDir, slug)
	if len(m.Generations) != 3 {
		t.Fatalf("expected three generations: %+v", m)
	}
	fixup := m.Generations[2]
	if fixup.Kind != store.PatchGenerationKindAmendFixup || fixup.Reason != "cover empty response" || fixup.FixupOfGeneration != target {
		t.Fatalf("fixup generation mismatch: %+v", fixup)
	}
}

func featureArtifactHash(t *testing.T, root, slug, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".tpatch", "features", slug, "artifacts", name))
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestFeaturePatchFixupRequiresReasonAndTargetBeforeCapture(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "Fixup validation")
	slug := "fixup-validation"
	if _, stderr, code := recordSameContent(t, tmpDir, slug, baseSha); code != 0 {
		t.Fatalf("first record failed: %s", stderr)
	}
	before := loadPatchGenerationsForTest(t, tmpDir, slug)
	if err := os.WriteFile(filepath.Join(tmpDir, "src", "a.txt"), []byte("a fixup\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := runCmdWithError("feature", "patch", "fixup", "--path", tmpDir, slug, "--target", before.Generations[0].GenerationID)
	if code == 0 || !strings.Contains(stderr, "requires --reason") {
		t.Fatalf("expected missing reason refusal, code=%d stderr=%q", code, stderr)
	}
	_, stderr, code = runCmdWithError("feature", "patch", "fixup", "--path", tmpDir, slug, "--reason", "why")
	if code == 0 || !strings.Contains(stderr, "requires --target") {
		t.Fatalf("expected missing target refusal, code=%d stderr=%q", code, stderr)
	}
	_, stderr, code = runCmdWithError("feature", "patch", "fixup", "--path", tmpDir, slug, "--target", "pg_missing", "--reason", "why")
	if code == 0 || !strings.Contains(stderr, "does not exist") {
		t.Fatalf("expected missing target refusal, code=%d stderr=%q", code, stderr)
	}
	after := loadPatchGenerationsForTest(t, tmpDir, slug)
	if len(after.Generations) != len(before.Generations) {
		t.Fatalf("fixup refusals must not append generations: before=%+v after=%+v", before, after)
	}
}

func TestFeaturePatchRefreshNoByteChangeSkips(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "No byte refresh")
	slug := "no-byte-refresh"
	if _, stderr, code := runCmdWithError("record", "--path", tmpDir, slug, "--from", baseSha, "--to", "HEAD", "--lenient"); code != 0 {
		t.Fatalf("first record failed: %s", stderr)
	}
	before := loadPatchGenerationsForTest(t, tmpDir, slug)
	_, stderr, code := runCmdWithError("feature", "patch", "refresh", "--path", tmpDir, slug)
	if code != 0 || !strings.Contains(stderr, "refresh skipped") {
		t.Fatalf("expected no-op refresh success, code=%d stderr=%q", code, stderr)
	}
	after := loadPatchGenerationsForTest(t, tmpDir, slug)
	if len(after.Generations) != len(before.Generations) {
		t.Fatalf("no-byte refresh appended: before=%+v after=%+v", before, after)
	}
}

func TestStatusJSONSurfacesParentGenerationStale(t *testing.T) {
	tmpDir, baseSha, shaA, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "Parent")
	if _, stderr, code := runCmdWithError("record", "--path", tmpDir, "parent", "--from", baseSha, "--to", shaA, "--lenient"); code != 0 {
		t.Fatalf("parent record failed: %s", stderr)
	}
	runCmd("add", "--path", tmpDir, "Child")
	s, err := store.Open(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	child, err := s.LoadFeatureStatus("child")
	if err != nil {
		t.Fatal(err)
	}
	child.DependsOn = []store.Dependency{{Slug: "parent", Kind: store.DependencyKindSoft}}
	if err := s.SaveFeatureStatus(child); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runCmdWithError("record", "--path", tmpDir, "child", "--from", baseSha, "--lenient", "--files", "noise.txt"); code != 0 {
		t.Fatalf("child record failed: %s", stderr)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "src", "a.txt"), []byte("parent refresh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runCmdWithError("feature", "patch", "refresh", "--path", tmpDir, "parent"); code != 0 {
		t.Fatalf("parent refresh failed: %s", stderr)
	}
	stdout, stderr, code := runCmdWithError("status", "--path", tmpDir, "--json")
	if code != 0 {
		t.Fatalf("status --json failed: %s", stderr)
	}
	var payload struct {
		Features []struct {
			Slug                  string                 `json:"slug"`
			ParentGenerationStale bool                   `json:"parent_generation_stale"`
			RenderedLabels        []store.ReconcileLabel `json:"labels_rendered"`
		} `json:"features"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("status json parse: %v\n%s", err, stdout)
	}
	for _, f := range payload.Features {
		if f.Slug != "child" {
			continue
		}
		if !f.ParentGenerationStale {
			t.Fatalf("child missing parent_generation_stale: %+v", f)
		}
		for _, label := range f.RenderedLabels {
			if label == store.LabelParentGenerationStale {
				return
			}
		}
		t.Fatalf("child missing rendered parent-generation-stale label: %+v", f)
	}
	t.Fatalf("child not found in status payload: %+v", payload.Features)
}
