package store

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestPatchGenerationsStoreRoundTrip(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	m := PatchGenerationsManifest{Version: 1, Feature: "demo", Generations: []PatchGeneration{}}
	g := sampleGeneration("demo", 1)
	if changed, err := AppendPatchGeneration(&m, g); err != nil || !changed {
		t.Fatalf("AppendPatchGeneration changed=%v err=%v", changed, err)
	}
	if err := SavePatchGenerations(s, m); err != nil {
		t.Fatalf("SavePatchGenerations: %v", err)
	}
	got, err := LoadPatchGenerations(s, "demo")
	if err != nil {
		t.Fatalf("LoadPatchGenerations: %v", err)
	}
	if got.CurrentGeneration != 1 || len(got.Generations) != 1 || got.Generations[0].CanonicalPatch != "artifacts/post-apply.patch" {
		t.Fatalf("unexpected roundtrip: %+v", got)
	}
}

func TestPatchGenerations_SchemaVersionMismatchRefuses(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	path := s.PatchGenerationsPath("demo")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":2,"feature":"demo","current_generation":0,"generations":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPatchGenerations(s, "demo")
	if err == nil || !strings.Contains(err.Error(), "unsupported schema version") {
		t.Fatalf("expected schema error, got %v", err)
	}
}

func TestPatchGenerations_StrictSchema_RefusesUnknownField(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	path := s.PatchGenerationsPath("demo")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"feature":"demo","current_generation":0,"generations":[],"foo":"bar"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPatchGenerations(s, "demo")
	if err == nil || !strings.Contains(err.Error(), "foo") {
		t.Fatalf("expected unknown field named in error, got %v", err)
	}
}

func TestPatchGenerations_StrictSchema_RefusesDeepUnknownField(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	path := s.PatchGenerationsPath("demo")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"version":1,"feature":"demo","current_generation":1,"generations":[{"generation":1,"generation_id":"pg_abc123def456","kind":"record","patch_sha256":"p","git_patch_id":"g","git_patch_id_algorithm":"git-patch-id-stable","recipe_sha256":"r","canonical_patch":"artifacts/post-apply.patch","audit_patch":"patches/001-record.patch","base_commit":"b","upper":{"kind":"working-tree","ref":"working-tree","commit":"","extra":"nope"},"capture":{"mode":"working-tree-all","pathspecs":[],"claim_ids":[]},"touched_paths":[],"dependencies":[],"refs":{"anchors":"","fingerprints":"","relations":"","vector_manifest":""}}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPatchGenerations(s, "demo")
	if err == nil || !strings.Contains(err.Error(), "extra") {
		t.Fatalf("expected deep unknown field named in error, got %v", err)
	}
}

func TestComputeGenerationID_Determinism(t *testing.T) {
	u := GenerationUpper{Kind: "commit", Ref: "HEAD", Commit: "abc"}
	c1 := GenerationCapture{Mode: "committed-range", Pathspecs: []string{"b", "a"}, ClaimIDs: []string{"z", "m"}}
	c2 := GenerationCapture{Mode: "committed-range", Pathspecs: []string{"a", "b"}, ClaimIDs: []string{"m", "z"}}
	id1 := ComputeGenerationID("demo", 1, strings.Repeat("a", 64), "", "base", u, c1)
	id2 := ComputeGenerationID("demo", 1, strings.Repeat("a", 64), "", "base", u, c2)
	if id1 != id2 {
		t.Fatalf("order should not affect generation_id: %s != %s", id1, id2)
	}
	if ok, _ := regexp.MatchString(`^pg_[0-9a-f]{12}$`, id1); !ok {
		t.Fatalf("generation_id has wrong shape: %s", id1)
	}
}

func TestComputeGenerationID_EmptyClaimIDs(t *testing.T) {
	id1 := ComputeGenerationID("demo", 1, "p", "r", "b", GenerationUpper{Kind: "working-tree", Ref: "working-tree"}, GenerationCapture{Mode: "working-tree-all"})
	id2 := ComputeGenerationID("demo", 1, "p", "r", "b", GenerationUpper{Kind: "working-tree", Ref: "working-tree"}, GenerationCapture{Mode: "working-tree-all", ClaimIDs: []string{}})
	if id1 != id2 {
		t.Fatalf("nil and empty claim_ids should hash identically")
	}
}

func TestComputeGenerationID_CollisionRefuse(t *testing.T) {
	m := PatchGenerationsManifest{Version: 1, Feature: "demo", Generations: []PatchGeneration{}}
	g1 := sampleGeneration("demo", 1)
	if _, err := AppendPatchGeneration(&m, g1); err != nil {
		t.Fatalf("append 1: %v", err)
	}
	g2 := sampleGeneration("demo", 2)
	g2.GenerationID = m.Generations[0].GenerationID
	g2.PatchSHA256 = "different"
	if _, err := AppendPatchGeneration(&m, g2); err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("expected collision refusal, got %v", err)
	}
}

func TestPatchGenerations_PinsAlgorithmMarker(t *testing.T) {
	m := PatchGenerationsManifest{Version: 1, Feature: "demo", CurrentGeneration: 1, Generations: []PatchGeneration{sampleGeneration("demo", 1)}}
	m.Generations[0].GitPatchIDAlgorithm = "other"
	if err := ValidatePatchGenerations("demo", m); err == nil || !strings.Contains(err.Error(), "git_patch_id_algorithm") {
		t.Fatalf("expected algorithm refusal, got %v", err)
	}
}

func TestLoadPatchGenerations_RejectsMissingRefs(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	path := s.PatchGenerationsPath("demo")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"version":1,"feature":"demo","current_generation":1,"generations":[{"generation":1,"generation_id":"pg_abc123def456","kind":"record","patch_sha256":"p","git_patch_id":"g","git_patch_id_algorithm":"git-patch-id-stable","recipe_sha256":"r","canonical_patch":"artifacts/post-apply.patch","audit_patch":"patches/001-record.patch","base_commit":"b","upper":{"kind":"working-tree","ref":"working-tree","commit":""},"capture":{"mode":"working-tree-all","pathspecs":[],"claim_ids":[]},"touched_paths":[],"dependencies":[]}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPatchGenerations(s, "demo")
	if err == nil || !strings.Contains(err.Error(), "refs") || !strings.Contains(err.Error(), "generations[0]") {
		t.Fatalf("expected missing refs error with generation index, got %v", err)
	}
}

func TestErrMalformedManifest_Classification(t *testing.T) {
	t.Run("json syntax", func(t *testing.T) {
		s := &Store{Root: t.TempDir()}
		path := s.PatchGenerationsPath("demo")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"version":1,"feature":"demo","current_generation":0,"generations":[,]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadPatchGenerations(s, "demo")
		if !errors.Is(err, ErrMalformedManifest) {
			t.Fatalf("expected ErrMalformedManifest for JSON syntax error, got %v", err)
		}
	})

	t.Run("schema validation", func(t *testing.T) {
		s := &Store{Root: t.TempDir()}
		path := s.PatchGenerationsPath("demo")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"version":2,"feature":"demo","current_generation":0,"generations":[]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadPatchGenerations(s, "demo")
		if !errors.Is(err, ErrMalformedManifest) {
			t.Fatalf("expected ErrMalformedManifest for validation error, got %v", err)
		}
	})

	t.Run("io error", func(t *testing.T) {
		s := &Store{Root: t.TempDir()}
		m := PatchGenerationsManifest{Version: PatchGenerationsManifestVersion, Feature: "demo", Generations: []PatchGeneration{sampleGeneration("demo", 1)}}
		m.CurrentGeneration = 1
		if err := SavePatchGenerations(s, m); err != nil {
			t.Fatalf("SavePatchGenerations: %v", err)
		}
		path := s.PatchGenerationsPath("demo")
		if err := os.Chmod(path, 0); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(path, 0o644)
		_, err := LoadPatchGenerations(s, "demo")
		if err == nil {
			t.Fatal("expected unreadable manifest error, got nil")
		}
		if errors.Is(err, ErrMalformedManifest) {
			t.Fatalf("I/O error must not classify as ErrMalformedManifest: %v", err)
		}
	})
}

func sampleGeneration(feature string, n int) PatchGeneration {
	g := PatchGeneration{
		Generation:          n,
		Kind:                "record",
		PatchSHA256:         strings.Repeat("a", 64),
		GitPatchID:          strings.Repeat("b", 40),
		GitPatchIDAlgorithm: PatchIDAlgorithmStable,
		RecipeSHA256:        "",
		CanonicalPatch:      "artifacts/post-apply.patch",
		AuditPatch:          "patches/001-record.patch",
		BaseCommit:          "base",
		Upper:               GenerationUpper{Kind: "working-tree", Ref: "working-tree", Commit: ""},
		Capture:             GenerationCapture{Mode: "working-tree-all", Pathspecs: []string{}, ClaimIDs: []string{}},
		TouchedPaths:        []string{"README.md"},
		Dependencies:        []GenerationDependency{},
		Refs:                &GenerationRefs{},
	}
	g.GenerationID = ComputeGenerationID(feature, n, g.PatchSHA256, g.RecipeSHA256, g.BaseCommit, g.Upper, g.Capture)
	return g
}
