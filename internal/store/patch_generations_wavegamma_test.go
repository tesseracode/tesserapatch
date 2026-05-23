package store

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const waveGammaGoldenManifest = `{
  "version": 1,
  "feature": "amend-demo",
  "current_generation": 3,
  "generations": [
    {
      "generation": 1,
      "generation_id": "pg_record000001",
      "kind": "record",
      "patch_sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "git_patch_id": "1111111111111111111111111111111111111111",
      "git_patch_id_algorithm": "git-patch-id-stable",
      "recipe_sha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      "canonical_patch": "",
      "audit_patch": "patches/001-record.patch",
      "base_commit": "base-1",
      "upper": {
        "kind": "working-tree",
        "ref": "working-tree",
        "commit": ""
      },
      "capture": {
        "mode": "working-tree-all",
        "pathspecs": [],
        "claim_ids": []
      },
      "touched_paths": [
        "README.md"
      ],
      "dependencies": [],
      "refs": {
        "anchors": "",
        "fingerprints": "",
        "relations": "",
        "vector_manifest": ""
      }
    },
    {
      "generation": 2,
      "generation_id": "pg_refresh0002",
      "kind": "amend-refresh",
      "reason": "complete README coverage",
      "patch_sha256": "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
      "git_patch_id": "2222222222222222222222222222222222222222",
      "git_patch_id_algorithm": "git-patch-id-stable",
      "recipe_sha256": "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
      "canonical_patch": "",
      "audit_patch": "patches/002-record.patch",
      "base_commit": "base-1",
      "upper": {
        "kind": "working-tree",
        "ref": "working-tree",
        "commit": ""
      },
      "capture": {
        "mode": "working-tree-all",
        "pathspecs": [],
        "claim_ids": []
      },
      "touched_paths": [
        "README.md"
      ],
      "dependencies": [],
      "refs": {
        "anchors": "",
        "fingerprints": "",
        "relations": "",
        "vector_manifest": ""
      }
    },
    {
      "generation": 3,
      "generation_id": "pg_fixup000003",
      "kind": "amend-fixup",
      "reason": "cover empty response",
      "fixup_of_generation": "pg_refresh0002",
      "patch_sha256": "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
      "git_patch_id": "3333333333333333333333333333333333333333",
      "git_patch_id_algorithm": "git-patch-id-stable",
      "recipe_sha256": "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
      "canonical_patch": "artifacts/post-apply.patch",
      "audit_patch": "patches/003-record.patch",
      "base_commit": "base-1",
      "upper": {
        "kind": "working-tree",
        "ref": "working-tree",
        "commit": ""
      },
      "capture": {
        "mode": "working-tree-all",
        "pathspecs": [],
        "claim_ids": []
      },
      "touched_paths": [
        "README.md"
      ],
      "dependencies": [],
      "refs": {
        "anchors": "",
        "fingerprints": "",
        "relations": "",
        "vector_manifest": ""
      }
    }
  ]
}
`

const waveBetaManifestFixture = `{
  "version": 1,
  "feature": "beta-demo",
  "current_generation": 1,
  "generations": [
    {
      "generation": 1,
      "generation_id": "pg_beta0000001",
      "kind": "record",
      "patch_sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "git_patch_id": "1111111111111111111111111111111111111111",
      "git_patch_id_algorithm": "git-patch-id-stable",
      "recipe_sha256": "",
      "canonical_patch": "artifacts/post-apply.patch",
      "audit_patch": "patches/001-record.patch",
      "base_commit": "base-1",
      "upper": {
        "kind": "working-tree",
        "ref": "working-tree",
        "commit": ""
      },
      "capture": {
        "mode": "working-tree-all",
        "pathspecs": [],
        "claim_ids": []
      },
      "touched_paths": [
        "README.md"
      ],
      "dependencies": [],
      "refs": {
        "anchors": "",
        "fingerprints": "",
        "relations": "",
        "vector_manifest": ""
      }
    }
  ]
}
`

func TestPatchGenerationsWaveGammaGoldenRoundTrip(t *testing.T) {
	s := storeWithManifestFixture(t, "amend-demo", waveGammaGoldenManifest)
	m, err := LoadPatchGenerations(s, "amend-demo")
	if err != nil {
		t.Fatalf("LoadPatchGenerations: %v", err)
	}
	if err := SavePatchGenerations(s, m); err != nil {
		t.Fatalf("SavePatchGenerations: %v", err)
	}
	got, err := os.ReadFile(s.PatchGenerationsPath("amend-demo"))
	if err != nil {
		t.Fatalf("read saved fixture: %v", err)
	}
	if !bytes.Equal(got, []byte(waveGammaGoldenManifest)) {
		t.Fatalf("round-trip changed bytes\nwant:\n%s\ngot:\n%s", waveGammaGoldenManifest, string(got))
	}
}

func TestPatchGenerationsWaveGammaStrictOnUnknown(t *testing.T) {
	fixture := strings.Replace(waveGammaGoldenManifest, "\"reason\": \"complete README coverage\",", "\"reason\": \"complete README coverage\",\n      \"unexpected_wave_gamma_field\": true,", 1)
	s := storeWithManifestFixture(t, "amend-demo", fixture)
	_, err := LoadPatchGenerations(s, "amend-demo")
	if err == nil || !strings.Contains(err.Error(), "unexpected_wave_gamma_field") {
		t.Fatalf("expected strict unknown-field error, got %v", err)
	}
}

func TestPatchGenerationsWaveGammaFixupRequiresReason(t *testing.T) {
	for name, replacement := range map[string]string{
		"missing": "",
		"empty":   "\"reason\": \"\",\n      ",
	} {
		t.Run(name, func(t *testing.T) {
			fixture := strings.Replace(waveGammaGoldenManifest, "\"reason\": \"cover empty response\",\n      ", replacement, 1)
			s := storeWithManifestFixture(t, "amend-demo", fixture)
			_, err := LoadPatchGenerations(s, "amend-demo")
			if err == nil || !strings.Contains(err.Error(), "reason") {
				t.Fatalf("expected reason validation error, got %v", err)
			}
		})
	}
}

func TestPatchGenerationsWaveGammaFixupTargetMustResolveToPriorGeneration(t *testing.T) {
	fixture := strings.Replace(waveGammaGoldenManifest, "\"fixup_of_generation\": \"pg_refresh0002\"", "\"fixup_of_generation\": \"pg_missing0000\"", 1)
	s := storeWithManifestFixture(t, "amend-demo", fixture)
	_, err := LoadPatchGenerations(s, "amend-demo")
	if err == nil || !strings.Contains(err.Error(), "fixup_of_generation") {
		t.Fatalf("expected fixup target validation error, got %v", err)
	}
}

func TestPatchGenerationsWaveGammaExistingWaveBetaManifestRoundTrip(t *testing.T) {
	s := storeWithManifestFixture(t, "beta-demo", waveBetaManifestFixture)
	m, err := LoadPatchGenerations(s, "beta-demo")
	if err != nil {
		t.Fatalf("LoadPatchGenerations: %v", err)
	}
	if err := SavePatchGenerations(s, m); err != nil {
		t.Fatalf("SavePatchGenerations: %v", err)
	}
	got, err := os.ReadFile(s.PatchGenerationsPath("beta-demo"))
	if err != nil {
		t.Fatalf("read saved fixture: %v", err)
	}
	if !bytes.Equal(got, []byte(waveBetaManifestFixture)) {
		t.Fatalf("Wave beta fixture changed bytes\nwant:\n%s\ngot:\n%s", waveBetaManifestFixture, string(got))
	}
}

func storeWithManifestFixture(t *testing.T, slug, fixture string) *Store {
	t.Helper()
	s := &Store{Root: t.TempDir()}
	path := s.PatchGenerationsPath(slug)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	return s
}
