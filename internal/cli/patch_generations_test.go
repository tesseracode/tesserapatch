package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func loadPatchGenerationsForTest(t *testing.T, tmpDir, slug string) store.PatchGenerationsManifest {
	t.Helper()
	p := filepath.Join(tmpDir, ".tpatch", "features", slug, "artifacts", "patch-generations.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read patch-generations.json: %v", err)
	}
	var m store.PatchGenerationsManifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal patch-generations.json: %v", err)
	}
	return m
}

func TestPatchGenerations_FirstRecordCreatesGeneration1(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "Identity")
	if _, stderr, code := recordSameContent(t, tmpDir, "identity", baseSha); code != 0 {
		t.Fatalf("record failed: %s", stderr)
	}
	m := loadPatchGenerationsForTest(t, tmpDir, "identity")
	if m.CurrentGeneration != 1 || len(m.Generations) != 1 || m.Generations[0].Generation != 1 {
		t.Fatalf("expected generation 1, got %+v", m)
	}
	g := m.Generations[0]
	if g.GitPatchIDAlgorithm != store.PatchIDAlgorithmStable || g.CanonicalPatch != "artifacts/post-apply.patch" || g.AuditPatch != "patches/001-record.patch" {
		t.Fatalf("unexpected generation fields: %+v", g)
	}
	if len(g.TouchedPaths) == 0 || g.Capture.Mode == "" || g.BaseCommit == "" {
		t.Fatalf("missing required metadata: %+v", g)
	}
}

func TestPatchGenerations_ChangedRerecordAppends(t *testing.T) {
	tmpDir, baseSha, shaA, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "Evolve identity")
	slug := "evolve-identity"
	if _, stderr, code := recordSameContent(t, tmpDir, slug, baseSha); code != 0 {
		t.Fatalf("first record failed: %s", stderr)
	}
	if _, stderr, code := runCmdWithError("record", "--path", tmpDir, slug, "--from", baseSha, "--to", shaA, "--lenient"); code != 0 {
		t.Fatalf("second record failed: %s", stderr)
	}
	m := loadPatchGenerationsForTest(t, tmpDir, slug)
	if m.CurrentGeneration != 2 || len(m.Generations) != 2 || m.Generations[1].Generation != 2 {
		t.Fatalf("expected two monotonic generations, got %+v", m)
	}
	if m.Generations[0].CanonicalPatch != "" || m.Generations[1].CanonicalPatch != "artifacts/post-apply.patch" {
		t.Fatalf("canonical path should move to latest: %+v", m.Generations)
	}
}

func TestPatchGenerations_HasMonotonicAndContentAddressedIDs(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "IDs")
	if _, stderr, code := recordSameContent(t, tmpDir, "ids", baseSha); code != 0 {
		t.Fatalf("record failed: %s", stderr)
	}
	g := loadPatchGenerationsForTest(t, tmpDir, "ids").Generations[0]
	if g.Generation != 1 {
		t.Fatalf("generation missing: %+v", g)
	}
	if ok, _ := regexp.MatchString(`^pg_[0-9a-f]{12}$`, g.GenerationID); !ok {
		t.Fatalf("bad generation_id: %q", g.GenerationID)
	}
}

func TestPatchGenerations_ClaimedOnlyRecordsClaimIDs(t *testing.T) {
	tmp := modesFixture(t, "claim-gen")
	modesWriteFile(t, tmp, "src/keep.go", "keep\n")
	modesWriteFile(t, tmp, "src/drop.go", "drop\n")
	if _, _, code := runRecord(t, "feature", "claim", "add", "--path", tmp, "claim-gen", "src/keep.go"); code != 0 {
		t.Fatalf("claim add failed")
	}
	if _, stderr, code := runRecord(t, "record", "--path", tmp, "claim-gen", "--claimed-only", "--lenient"); code != 0 {
		t.Fatalf("record --claimed-only failed: %s", stderr)
	}
	g := loadPatchGenerationsForTest(t, tmp, "claim-gen").Generations[0]
	if len(g.Capture.ClaimIDs) != 1 || g.Capture.ClaimIDs[0] == "" {
		t.Fatalf("expected one claim id in generation: %+v", g.Capture)
	}
	md := readRecordMD(t, tmp, "claim-gen")
	if !strings.Contains(md, g.Capture.ClaimIDs[0]) {
		t.Fatalf("generation claim id should match record.md provenance: id=%s md=%s", g.Capture.ClaimIDs[0], md)
	}
}

func TestPatchGenerations_DependencySnapshot(t *testing.T) {
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
	st, err := s.LoadFeatureStatus("child")
	if err != nil {
		t.Fatal(err)
	}
	st.DependsOn = []store.Dependency{{Slug: "parent", Kind: store.DependencyKindHard}}
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runCmdWithError("record", "--path", tmpDir, "child", "--from", baseSha, "--lenient", "--files", "noise.txt"); code != 0 {
		t.Fatalf("child record failed: %s", stderr)
	}
	child := loadPatchGenerationsForTest(t, tmpDir, "child").Generations[0]
	parent := loadPatchGenerationsForTest(t, tmpDir, "parent").Generations[0]
	if len(child.Dependencies) != 1 {
		t.Fatalf("expected dependency snapshot: %+v", child.Dependencies)
	}
	dep := child.Dependencies[0]
	if dep.Slug != "parent" || dep.ParentGeneration != 1 || dep.ParentPatchSHA256 != parent.PatchSHA256 {
		t.Fatalf("bad dependency snapshot: %+v parent=%+v", dep, parent)
	}
}

func TestPatchGenerations_SameBytesRerecordSkipsGeneration(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "Solo generation")
	slug := "solo-generation"
	if _, stderr, code := recordSameContent(t, tmpDir, slug, baseSha); code != 0 {
		t.Fatalf("first record failed: %s", stderr)
	}
	if _, stderr, code := recordSameContent(t, tmpDir, slug, baseSha); code != 0 {
		t.Fatalf("second record failed: %s", stderr)
	}
	m := loadPatchGenerationsForTest(t, tmpDir, slug)
	if len(m.Generations) != 1 {
		t.Fatalf("same bytes should not append generation: %+v", m)
	}
}

func TestPatchGenerations_RecipeOnlySkipsGeneration(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "Recipe only")
	slug := "recipe-only"
	if _, stderr, code := recordSameContent(t, tmpDir, slug, baseSha); code != 0 {
		t.Fatalf("first record failed: %s", stderr)
	}
	recipePath := filepath.Join(tmpDir, ".tpatch", "features", slug, "artifacts", "apply-recipe.json")
	if err := os.WriteFile(recipePath, []byte(`{"version":1,"operations":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := recordSameContent(t, tmpDir, slug, baseSha, "--regenerate-recipe"); code != 0 {
		t.Fatalf("recipe-only rerecord failed: %s", stderr)
	}
	m := loadPatchGenerationsForTest(t, tmpDir, slug)
	if len(m.Generations) != 1 {
		t.Fatalf("recipe-only change should not append generation: %+v", m)
	}
}

func TestPatchGenerations_MalformedManifestRefusesRecord(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "Bad manifest")
	path := filepath.Join(tmpDir, ".tpatch", "features", "bad-manifest", "artifacts", "patch-generations.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"feature":"bad-manifest","current_generation":0,"generations":[],"foo":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := recordSameContent(t, tmpDir, "bad-manifest", baseSha)
	if code == 0 {
		t.Fatalf("expected malformed manifest refusal")
	}
	if !strings.Contains(stderr, "patch-generations.json") || !strings.Contains(stderr, "foo") {
		t.Fatalf("expected clear manifest error, got %s", stderr)
	}
	if got := readArtifact(t, tmpDir, "bad-manifest", "post-apply.patch"); got != "" {
		t.Fatalf("record should refuse before writing post-apply.patch")
	}
}

func TestPatchGenerations_RecordReconcileRecordSmoke(t *testing.T) {
	tmpDir, baseSha, shaA, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "Smoke")
	slug := "smoke"
	if _, stderr, code := recordSameContent(t, tmpDir, slug, baseSha); code != 0 {
		t.Fatalf("first record failed: %s", stderr)
	}
	if _, stderr, code := runCmdWithError("reconcile", "--path", tmpDir, "--preflight", "--allow-dirty", slug); code != 0 {
		t.Fatalf("reconcile preflight failed: %s", stderr)
	}
	if _, stderr, code := runCmdWithError("record", "--path", tmpDir, slug, "--from", baseSha, "--to", shaA, "--lenient"); code != 0 {
		t.Fatalf("second record failed: %s", stderr)
	}
	m := loadPatchGenerationsForTest(t, tmpDir, slug)
	if m.Version != 1 || m.Feature != slug || m.CurrentGeneration != 2 || len(m.Generations) != 2 {
		t.Fatalf("unexpected final manifest shape: %+v", m)
	}
	if m.Generations[0].Generation != 1 || m.Generations[1].Generation != 2 || m.Generations[1].Refs != (store.GenerationRefs{}) {
		t.Fatalf("unexpected generation sequence/refs: %+v", m.Generations)
	}
}

func TestPatchGenerations_NoManifestBackwardsCompatible(t *testing.T) {
	tmpDir, baseSha, _, _ := setupRecordRangeFixture(t)
	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "Legacy")
	legacyPatch := filepath.Join(tmpDir, ".tpatch", "features", "legacy", "patches", "001-record.patch")
	if err := os.MkdirAll(filepath.Dir(legacyPatch), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPatch, []byte("legacy audit only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := recordSameContent(t, tmpDir, "legacy", baseSha); code != 0 {
		t.Fatalf("record should create manifest despite legacy patches: %s", stderr)
	}
	m := loadPatchGenerationsForTest(t, tmpDir, "legacy")
	if len(m.Generations) != 1 || m.Generations[0].AuditPatch != "patches/002-record.patch" {
		t.Fatalf("expected no backfill and one live generation, got %+v", m)
	}
}
