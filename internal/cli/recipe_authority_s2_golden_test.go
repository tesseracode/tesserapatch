package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The prepare-era golden files stay byte-frozen. This derives only ADR-036
// D5/D6's intended delta from their old bytes, never from current CLI output
// or the production recipe encoder. Every other transcript byte still compares.
func rgaS2ExpectedProducerGolden(name string, frozen []byte) ([]byte, error) {
	if name != "compat-record.txt" && name != "compat-land.txt" {
		return frozen, nil
	}
	old := string(frozen)
	recipe, err := rgaS2GoldenSection(old, "artifacts/apply-recipe.json")
	if err != nil {
		return nil, err
	}
	const replaceField = "      \"replace\": \"\"\n"
	if strings.Count(recipe, replaceField) != 1 || strings.Contains(recipe, "preimage_hash") {
		return nil, fmt.Errorf("S2 expected delta needs the frozen legacy recipe")
	}
	updatedRecipe := strings.Replace(recipe, replaceField, "      \"replace\": \"\",\n      \"preimage_hash\": \"\"\n", 1)
	oldHash := fmt.Sprintf("%x", sha256.Sum256([]byte(recipe)))
	newHash := fmt.Sprintf("%x", sha256.Sum256([]byte(updatedRecipe)))

	manifest, err := rgaS2GoldenSection(old, "artifacts/patch-generations.json")
	if err != nil {
		return nil, err
	}
	var record struct {
		Feature     string `json:"feature"`
		Generations []struct {
			Number     int                                `json:"generation"`
			ID         string                             `json:"generation_id"`
			PatchHash  string                             `json:"patch_sha256"`
			RecipeHash string                             `json:"recipe_sha256"`
			Base       string                             `json:"base_commit"`
			Upper      struct{ Kind, Ref, Commit string } `json:"upper"`
			Capture    struct {
				Mode      string
				Pathspecs []string
				ClaimIDs  []string `json:"claim_ids"`
			} `json:"capture"`
		} `json:"generations"`
	}
	if err := json.Unmarshal([]byte(manifest), &record); err != nil || len(record.Generations) != 1 {
		return nil, fmt.Errorf("S2 golden needs exactly one decoded generation: %v", err)
	}
	g := record.Generations[0]
	if g.RecipeHash != oldHash || len(g.Capture.Pathspecs) != 0 || len(g.Capture.ClaimIDs) != 0 {
		return nil, fmt.Errorf("S2 golden generation does not bind the frozen fixture")
	}
	// ADR-024 D2's NUL-separated identity, independently of the current
	// production helper: only recipe_sha256 changes in this generation.
	generationID := func(recipeHash string) string {
		parts := []string{record.Feature, fmt.Sprint(g.Number), g.PatchHash, recipeHash,
			g.Base, g.Upper.Commit, g.Upper.Kind, g.Upper.Ref, g.Capture.Mode, "", ""}
		sum := sha256.Sum256([]byte(strings.Join(parts, "\x00") + "\x00"))
		return fmt.Sprintf("pg_%x", sum[:6])
	}
	if generationID(oldHash) != g.ID {
		return nil, fmt.Errorf("frozen generation ID does not match ADR-024")
	}
	base := g.Base
	if name == "compat-land.txt" {
		const prefix = "    Tpatch-Base-Commit: "
		_, suffix, found := strings.Cut(old, prefix)
		if !found {
			return nil, fmt.Errorf("land golden lacks its resolved base")
		}
		base, _, _ = strings.Cut(suffix, "\n")
	}
	if len(base) != 40 || strings.Trim(base, "0123456789abcdef") != "" {
		return nil, fmt.Errorf("golden provenance base is not a resolved commit")
	}
	provenanceBody := fmt.Sprintf("{\n  \"base_commit\": %q,\n  \"generated_at\": \"<wall-clock>\",\n  \"recipe_sha256\": %q\n}\n", base, newHash)
	provenance := fmt.Sprintf("--- .tpatch/features/pib-golden/artifacts/recipe-provenance.json (%d bytes) ---\n%s",
		len(provenanceBody), provenanceBody)
	out := old
	replacements := [][2]string{
		{recipe, updatedRecipe},
		{fmt.Sprintf("apply-recipe.json (%d bytes)", len(recipe)), fmt.Sprintf("apply-recipe.json (%d bytes)", len(updatedRecipe))},
		{g.ID, generationID(newHash)},
	}
	for _, replacement := range replacements {
		if strings.Count(out, replacement[0]) != 1 {
			return nil, fmt.Errorf("S2 golden replacement is missing or ambiguous: %q", replacement[0])
		}
		out = strings.Replace(out, replacement[0], replacement[1], 1)
	}
	hashes := 1
	if name == "compat-land.txt" {
		hashes = 2 // generation binding plus the commit trailer
		const staged = "   M .tpatch/features/pib-golden/artifacts/post-apply.patch\n"
		if strings.Count(out, staged) != 1 {
			return nil, fmt.Errorf("land staging anchor is missing or ambiguous")
		}
		out = strings.Replace(out, staged, staged+"   M .tpatch/features/pib-golden/artifacts/recipe-provenance.json\n", 1)
	}
	if strings.Count(out, oldHash) != hashes {
		return nil, fmt.Errorf("S2 golden has unexpected recipe hash bindings")
	}
	out = strings.ReplaceAll(out, oldHash, newHash)
	const patchHeader = "--- .tpatch/features/pib-golden/artifacts/post-apply.patch (155 bytes) ---\n"
	if strings.Count(out, patchHeader) != 1 {
		return nil, fmt.Errorf("S2 golden canonical patch section changed")
	}
	start := strings.Index(out, patchHeader) + len(patchHeader)
	patchBody, err := rgaS2GoldenSection(old, "artifacts/post-apply.patch")
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(out[start:], patchBody) {
		return nil, fmt.Errorf("S2 golden canonical patch body changed")
	}
	at := start + len(patchBody)
	return []byte(out[:at] + provenance + out[at:]), nil
}

func rgaS2GoldenSection(transcript, relative string) (string, error) {
	prefix := "--- .tpatch/features/pib-golden/" + relative + " ("
	if strings.Count(transcript, prefix) != 1 {
		return "", fmt.Errorf("missing or ambiguous frozen section %s", relative)
	}
	_, tail, _ := strings.Cut(transcript, prefix)
	header, body, found := strings.Cut(tail, " ---\n")
	if !found {
		return "", fmt.Errorf("malformed frozen section %s", relative)
	}
	sizeText, found := strings.CutSuffix(header, " bytes)")
	size, err := strconv.Atoi(sizeText)
	if !found || err != nil || size < 0 || size > len(body) {
		return "", fmt.Errorf("invalid frozen section size for %s", relative)
	}
	if len(body) != size && !strings.HasPrefix(body[size:], "--- .tpatch/") {
		return "", fmt.Errorf("frozen section size does not end at a record boundary: %s", relative)
	}
	return body[:size], nil
}

func TestRGAS2GoldenDeltaIsExactAndMutationSensitive(t *testing.T) {
	for _, name := range []string{"compat-record.txt", "compat-land.txt"} {
		t.Run(name, func(t *testing.T) {
			frozen, err := os.ReadFile(filepath.Join(preparePIBGoldenDir, name))
			if err != nil {
				t.Fatal(err)
			}
			expected, err := rgaS2ExpectedProducerGolden(name, frozen)
			if err != nil || preparePIBGoldenDelta(name, string(expected)) != nil {
				t.Fatalf("independent S2 delta refused: %v", err)
			}
			if !strings.Contains(string(expected), "+recorded change\n--- .tpatch/features/pib-golden/artifacts/recipe-provenance.json") {
				t.Fatal("provenance was inserted inside the canonical patch instead of after its complete body")
			}
			for _, mutation := range []struct{ old, new string }{
				{`"preimage_hash": ""`, `"preimage_hash": "sha256:wrong"`},
				{`"generated_at": "<wall-clock>"`, `"generated_at": "backdated"`},
				{`"recipe_sha256": "559078db6a81e4530a844e37fbfb0ef4023acc9ab032a95c1fd243de7568c62f"`, `"recipe_sha256": "wrong"`},
				{`"kind": "record"`, `"kind": "unrelated"`},
				{"artifacts/recipe-provenance.json (185 bytes)", "artifacts/recipe-provenance.json (0 bytes)"},
				{"artifacts/recipe-provenance.json (185 bytes)", "artifacts/recipe-provenance.json (193 bytes)"},
			} {
				wrong := strings.Replace(string(expected), mutation.old, mutation.new, 1)
				if wrong == string(expected) || preparePIBGoldenDelta(name, wrong) == nil {
					t.Fatalf("same comparator accepted drift %q", mutation.old)
				}
			}
			for _, wrong := range []string{string(frozen), string(expected) + "\n", ""} {
				if preparePIBGoldenDelta(name, wrong) == nil {
					t.Fatal("same comparator accepted a legacy, expanded, or missing capture")
				}
			}
			invalidFrozen := strings.Replace(string(frozen), `"replace": ""`, `"replace": "not the baseline"`, 1)
			if _, err := rgaS2ExpectedProducerGolden(name, []byte(invalidFrozen)); err == nil {
				t.Fatal("delta adapter silently accepted a changed baseline shape")
			}
			for _, wrongSize := range []string{"154", "156", "-1", "huge"} {
				bad := strings.Replace(string(frozen), "post-apply.patch (155 bytes)", "post-apply.patch ("+wrongSize+" bytes)", 1)
				if _, err := rgaS2GoldenSection(bad, "artifacts/post-apply.patch"); err == nil {
					t.Fatalf("section parser accepted incorrect body size %s", wrongSize)
				}
			}
		})
	}
}
