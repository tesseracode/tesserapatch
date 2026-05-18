package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Patch generation manifest constants (ADR-024 / PRD-feature-patch-identity-metadata v1).
const (
	PatchGenerationsManifestVersion = 1
	PatchIDAlgorithmStable          = "git-patch-id-stable"
)

const patchGenerationsFileName = "patch-generations.json"

type PatchGenerationsManifest struct {
	Version           int               `json:"version"`
	Feature           string            `json:"feature"`
	CurrentGeneration int               `json:"current_generation"`
	Generations       []PatchGeneration `json:"generations"`
}

type PatchGeneration struct {
	Generation          int                    `json:"generation"`
	GenerationID        string                 `json:"generation_id"`
	Kind                string                 `json:"kind"`
	PatchSHA256         string                 `json:"patch_sha256"`
	GitPatchID          string                 `json:"git_patch_id"`
	GitPatchIDAlgorithm string                 `json:"git_patch_id_algorithm"`
	RecipeSHA256        string                 `json:"recipe_sha256"`
	CanonicalPatch      string                 `json:"canonical_patch"`
	AuditPatch          string                 `json:"audit_patch"`
	BaseCommit          string                 `json:"base_commit"`
	Upper               GenerationUpper        `json:"upper"`
	Capture             GenerationCapture      `json:"capture"`
	TouchedPaths        []string               `json:"touched_paths"`
	Dependencies        []GenerationDependency `json:"dependencies"`
	Refs                GenerationRefs         `json:"refs"`
}

type GenerationUpper struct {
	Kind   string `json:"kind"`
	Ref    string `json:"ref"`
	Commit string `json:"commit"`
}

type GenerationCapture struct {
	Mode      string   `json:"mode"`
	Pathspecs []string `json:"pathspecs"`
	ClaimIDs  []string `json:"claim_ids"`
}

type GenerationDependency struct {
	Slug               string `json:"slug"`
	Kind               string `json:"kind"`
	SatisfiedBy        string `json:"satisfied_by"`
	SatisfiedPatchID   string `json:"satisfied_patch_id,omitempty"`
	ParentGeneration   int    `json:"parent_generation"`
	ParentPatchSHA256  string `json:"parent_patch_sha256"`
	ParentRecipeSHA256 string `json:"parent_recipe_sha256,omitempty"`
}

type GenerationRefs struct {
	Anchors        string `json:"anchors"`
	Fingerprints   string `json:"fingerprints"`
	Relations      string `json:"relations"`
	VectorManifest string `json:"vector_manifest"`
}

func (s *Store) PatchGenerationsPath(slug string) string {
	return filepath.Join(s.featureArtifactsDir(slug), patchGenerationsFileName)
}

func LoadPatchGenerations(s *Store, slug string) (PatchGenerationsManifest, error) {
	path := s.PatchGenerationsPath(slug)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return PatchGenerationsManifest{Version: PatchGenerationsManifestVersion, Feature: slug, Generations: []PatchGeneration{}}, nil
		}
		return PatchGenerationsManifest{}, err
	}
	var m PatchGenerationsManifest
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return PatchGenerationsManifest{}, fmt.Errorf("patch-generations.json: %w", err)
	}
	if err := ValidatePatchGenerations(slug, m); err != nil {
		return PatchGenerationsManifest{}, err
	}
	return m, nil
}

func ValidatePatchGenerations(slug string, m PatchGenerationsManifest) error {
	if m.Version != PatchGenerationsManifestVersion {
		return fmt.Errorf("patch-generations.json: unsupported schema version %d (expected %d)", m.Version, PatchGenerationsManifestVersion)
	}
	if m.Feature != slug {
		return fmt.Errorf("patch-generations.json: feature mismatch (file says %q, expected %q)", m.Feature, slug)
	}
	if m.Generations == nil {
		return fmt.Errorf("patch-generations.json: generations field is required")
	}
	if len(m.Generations) == 0 {
		if m.CurrentGeneration != 0 {
			return fmt.Errorf("patch-generations.json: current_generation %d does not match empty generations", m.CurrentGeneration)
		}
		return nil
	}
	seenGen := map[int]struct{}{}
	seenID := map[string]PatchGeneration{}
	for i, g := range m.Generations {
		want := i + 1
		if g.Generation != want {
			return fmt.Errorf("patch-generations.json: generations[%d].generation=%d is not contiguous from 1", i, g.Generation)
		}
		if _, ok := seenGen[g.Generation]; ok {
			return fmt.Errorf("patch-generations.json: duplicate generation %d", g.Generation)
		}
		seenGen[g.Generation] = struct{}{}
		if err := validatePatchGeneration(g); err != nil {
			return fmt.Errorf("patch-generations.json: generations[%d]: %w", i, err)
		}
		if prev, ok := seenID[g.GenerationID]; ok {
			if !patchGenerationPayloadEqual(prev, g) {
				return fmt.Errorf("patch-generations.json: generation_id %q has differing payload (hash/schema collision)", g.GenerationID)
			}
		} else {
			seenID[g.GenerationID] = g
		}
	}
	if m.CurrentGeneration != len(m.Generations) {
		return fmt.Errorf("patch-generations.json: current_generation %d does not match latest generation %d", m.CurrentGeneration, len(m.Generations))
	}
	return nil
}

func validatePatchGeneration(g PatchGeneration) error {
	if g.GenerationID == "" {
		return fmt.Errorf("generation_id is required")
	}
	switch g.Kind {
	case "record", "reconcile", "amend-refresh", "amend-fixup", "import", "manual-metadata":
	default:
		return fmt.Errorf("kind %q is not recognized", g.Kind)
	}
	if g.GitPatchIDAlgorithm != PatchIDAlgorithmStable {
		return fmt.Errorf("git_patch_id_algorithm %q is not supported (expected %q)", g.GitPatchIDAlgorithm, PatchIDAlgorithmStable)
	}
	switch g.Upper.Kind {
	case "working-tree", "index", "commit", "range", "reconcile-result":
	default:
		return fmt.Errorf("upper.kind %q is not recognized", g.Upper.Kind)
	}
	if g.Refs.Anchors != "" || g.Refs.Fingerprints != "" || g.Refs.Relations != "" || g.Refs.VectorManifest != "" {
		return fmt.Errorf("refs must be empty strings in schema version 1")
	}
	return nil
}

func SavePatchGenerations(s *Store, m PatchGenerationsManifest) error {
	if m.Version == 0 {
		m.Version = PatchGenerationsManifestVersion
	}
	if m.Version != PatchGenerationsManifestVersion {
		return fmt.Errorf("patch-generations.json: refusing to write unsupported schema version %d", m.Version)
	}
	if m.Feature == "" {
		return fmt.Errorf("patch-generations.json: feature slug is required")
	}
	if m.Generations == nil {
		m.Generations = []PatchGeneration{}
	}
	if err := ValidatePatchGenerations(m.Feature, m); err != nil {
		return err
	}
	target := s.PatchGenerationsPath(m.Feature)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := target + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp)
		return err
	}
	if d, err := os.Open(filepath.Dir(target)); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// ComputeGenerationID mirrors ComputeClaimID's NUL-separated SHA-256 prefix style.
// ADR-024 D2 pins the hash input set and requires pathspecs/claim_ids to be
// sorted before hashing; NUL separators make boundary collisions impossible.
func ComputeGenerationID(feature string, generation int, patchSHA256, recipeSHA256, baseCommit string, upper GenerationUpper, capture GenerationCapture) string {
	pathspecs := append([]string(nil), capture.Pathspecs...)
	claimIDs := append([]string(nil), capture.ClaimIDs...)
	sort.Strings(pathspecs)
	sort.Strings(claimIDs)
	h := sha256.New()
	writeHashPart := func(s string) {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	writeHashPart(feature)
	writeHashPart(fmt.Sprintf("%d", generation))
	writeHashPart(patchSHA256)
	writeHashPart(recipeSHA256)
	writeHashPart(baseCommit)
	writeHashPart(upper.Commit)
	writeHashPart(upper.Kind)
	writeHashPart(upper.Ref)
	writeHashPart(capture.Mode)
	writeHashPart(strings.Join(pathspecs, "\n"))
	writeHashPart(strings.Join(claimIDs, "\n"))
	return "pg_" + hex.EncodeToString(h.Sum(nil))[:12]
}

func AppendPatchGeneration(m *PatchGenerationsManifest, g PatchGeneration) (bool, error) {
	if m.Version == 0 {
		m.Version = PatchGenerationsManifestVersion
	}
	if m.Generations == nil {
		m.Generations = []PatchGeneration{}
	}
	if g.Kind != "record" && g.Kind != "reconcile" {
		return false, fmt.Errorf("patch-generations.json: kind %q is reserved and not writable in v1", g.Kind)
	}
	if g.Generation == 0 {
		g.Generation = len(m.Generations) + 1
	}
	if g.Generation != len(m.Generations)+1 {
		return false, fmt.Errorf("patch-generations.json: generation %d is not the next contiguous value %d", g.Generation, len(m.Generations)+1)
	}
	if g.GenerationID == "" {
		g.GenerationID = ComputeGenerationID(m.Feature, g.Generation, g.PatchSHA256, g.RecipeSHA256, g.BaseCommit, g.Upper, g.Capture)
	}
	if g.GitPatchIDAlgorithm == "" {
		g.GitPatchIDAlgorithm = PatchIDAlgorithmStable
	}
	for _, existing := range m.Generations {
		if existing.GenerationID == g.GenerationID {
			if patchGenerationPayloadEqual(existing, g) {
				return false, nil
			}
			return false, fmt.Errorf("patch-generations.json: generation_id %q has differing payload (hash/schema collision)", g.GenerationID)
		}
	}
	for i := range m.Generations {
		m.Generations[i].CanonicalPatch = ""
	}
	g.CanonicalPatch = "artifacts/post-apply.patch"
	m.Generations = append(m.Generations, g)
	m.CurrentGeneration = g.Generation
	return true, ValidatePatchGenerations(m.Feature, *m)
}

func LatestPatchGeneration(m PatchGenerationsManifest) (PatchGeneration, bool) {
	if len(m.Generations) == 0 {
		return PatchGeneration{}, false
	}
	return m.Generations[len(m.Generations)-1], true
}

func SHA256HexString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func SHA256HexBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func patchGenerationPayloadEqual(a, b PatchGeneration) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return bytes.Equal(aj, bj)
}
