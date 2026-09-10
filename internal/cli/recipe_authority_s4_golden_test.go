package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/workflow"
)

func rgaS4GoldenHash(body string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(body)))
}

// The input is the frozen historical transcript with its independently
// specified S2 delta. Only six fixtures execute a governed producer.
// Wire types supply layout, not the oracle: no publisher, builder, observation
// capture or production hash/encoder helper computes these expected facts.
func rgaS4ExpectedPublicationGolden(name string, previous []byte) ([]byte, error) {
	recorded := false
	switch name {
	case "compat-record.txt", "compat-land.txt", "compat-verify.txt", "compat-reconcile.txt":
		recorded = true
	case "phase-auto-implement.txt", "phase-manual-implement.txt":
	default:
		return previous, nil
	}
	old := string(previous)
	recipe, err := rgaS2GoldenSection(old, "artifacts/apply-recipe.json")
	if err != nil {
		return nil, err
	}
	var shape struct {
		Feature    string `json:"feature"`
		Operations []struct {
			Type         string  `json:"type"`
			Path         string  `json:"path"`
			Content      string  `json:"content"`
			PreimageHash *string `json:"preimage_hash"`
		} `json:"operations"`
	}
	if err := json.Unmarshal([]byte(recipe), &shape); err != nil ||
		shape.Feature != "pib-golden" || len(shape.Operations) != 1 {
		return nil, fmt.Errorf("S4 golden recipe is not the fixed single-operation fixture: %v", err)
	}
	c := workflow.RecipeCoverage{
		SchemaVersion: 1, Feature: "pib-golden", Producer: "implement",
		RecipePresent: true, RecipeDecodable: true, RecipeSHA256: rgaS4GoldenHash(recipe),
		Reference:      workflow.CoverageReference{Kind: "unavailable", PreimageSetSHA256: rgaS4GoldenHash("[]")},
		Capture:        workflow.CoverageCapture{Mode: "no-capture", Pathspecs: []string{}, ClaimIDs: []string{}},
		CoverageStatus: "incomplete", CrossBaseStatus: "unsupported",
		Effects: []workflow.CoverageEffect{},
		Reasons: []string{"canonical-patch-missing", "operation-surplus", "reference-not-durable"},
	}
	if !recorded && shape.Operations[0].Type != "ensure-directory" {
		return nil, fmt.Errorf("S4 implement golden no longer has its surplus directory operation")
	}
	if recorded {
		op := shape.Operations[0]
		if op.Type != "write-file" || op.Path != "src/golden.txt" || op.Content != "recorded change\n" ||
			op.PreimageHash == nil || *op.PreimageHash != "" {
			return nil, fmt.Errorf("S4 recorded golden no longer has its fixed gated creation")
		}
		patch, err := rgaS2GoldenSection(old, "artifacts/post-apply.patch")
		if err != nil {
			return nil, err
		}
		statusBytes, err := rgaS2GoldenSection(old, "status.json")
		if err != nil {
			return nil, err
		}
		var status struct {
			Apply struct {
				BaseCommit string `json:"base_commit"`
			} `json:"apply"`
		}
		if err := json.Unmarshal([]byte(statusBytes), &status); err != nil {
			return nil, err
		}
		base := status.Apply.BaseCommit
		if len(base) != 40 || strings.Trim(base, "0123456789abcdef") != "" {
			return nil, fmt.Errorf("S4 golden lacks its resolved preimage commit")
		}
		c.Producer, c.PatchPresent, c.PatchSHA256 = "record", true, rgaS4GoldenHash(patch)
		c.Reference.Kind, c.Reference.Commit = "commit", base
		c.Capture.Mode = "committed-range"
		if name == "compat-record.txt" {
			c.Capture.Mode = "working-tree-all"
		}
		c.CoverageStatus, c.CrossBaseStatus, c.Reasons = "complete", "reference-tree-only", []string{}
		post := rgaS4GoldenHash(op.Content)
		// ADR-036 D2's exact ordered preimage-set projection, including
		// byte-encoded paths. This is independent of patchobs's helper.
		set := fmt.Sprintf(`[{"ordinal":1,"path_hex":%q,"old_path_hex":"","change_kind":"add","content_kind":"text","object_kind":"regular","old_mode":"","new_mode":"100644","preimage_observed":true,"preimage_present":false,"preimage_sha256":"","postimage_observed":true,"postimage_present":true,"postimage_sha256":%q}]`,
			hex.EncodeToString([]byte(op.Path)), post)
		c.Reference.PreimageSetSHA256 = rgaS4GoldenHash(set)
		// D3's effect descriptor excludes operation assignment and advisory
		// fields; both final hashes derive only from frozen fixture data.
		descriptor := fmt.Sprintf(`{"ordinal":1,"change_kind":"add","content_kind":"text","object_kind":"regular","path":%q,"old_path":"","old_mode":"","new_mode":"100644","preimage_observed":true,"preimage_present":false,"preimage_sha256":"","postimage_observed":true,"postimage_present":true,"postimage_sha256":%q,"patch_fragment_sha256":%q}`,
			op.Path, post, c.PatchSHA256)
		c.Effects = []workflow.CoverageEffect{{
			Ordinal: 1, ChangeKind: "add", ContentKind: "text", ObjectKind: "regular",
			Path: op.Path, NewMode: "100644",
			PreimageObserved: true, PostimageObserved: true, PostimagePresent: true,
			PostimageSHA256: post, PatchFragmentSHA256: c.PatchSHA256,
			EffectSHA256: rgaS4GoldenHash(descriptor), OperationIndexes: []int{1},
			Disposition: "represented", ReasonCodes: []string{}, ContextualHint: "none",
		}}
	}
	body, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	body = append(body, '\n')
	section := fmt.Sprintf("--- .tpatch/features/pib-golden/artifacts/recipe-coverage.json (%d bytes) ---\n%s", len(body), body)
	anchor := "--- .tpatch/features/pib-golden/artifacts/recipe-provenance.json ("
	if name == "phase-manual-implement.txt" {
		anchor = "--- .tpatch/features/pib-golden/exploration.md ("
	}
	if strings.Count(old, anchor) != 1 || strings.Contains(old, "recipe-coverage.json") {
		return nil, fmt.Errorf("S4 coverage insertion point missing, ambiguous or already populated")
	}
	out := strings.Replace(old, anchor, section+anchor, 1)
	if name == "compat-land.txt" {
		const staged = "   M .tpatch/features/pib-golden/artifacts/recipe-provenance.json\n"
		if strings.Count(out, staged) != 1 {
			return nil, fmt.Errorf("S4 land golden staging point changed")
		}
		out = strings.Replace(out, staged, "   M .tpatch/features/pib-golden/artifacts/recipe-coverage.json\n"+staged, 1)
	}
	switch name {
	case "compat-record.txt", "phase-auto-implement.txt", "phase-manual-implement.txt":
		line := "recipe coverage: complete\n"
		if !recorded {
			line = "recipe coverage: incomplete (canonical-patch-missing, operation-surplus, reference-not-durable)\n"
		}
		const stderr = "stderr:\n"
		if strings.Count(out, stderr) != 1 || strings.Contains(out, "recipe coverage: ") {
			return nil, fmt.Errorf("S4 producer diagnostic insertion point changed")
		}
		out = strings.Replace(out, stderr, stderr+line, 1)
	}
	return []byte(out), nil
}

func rgaS4ExpectedRoutingGolden(name string, frozen []byte) ([]byte, error) {
	if name != "cycle-skip-execute-transcript.txt" {
		return frozen, nil
	}
	const anchor = "[4/6] Generating apply recipe...\n"
	const status = "recipe coverage: incomplete (canonical-patch-missing, operation-surplus, reference-not-durable)\n"
	before := string(frozen)
	if strings.Count(before, anchor) != 1 || strings.Contains(before, "recipe coverage: ") {
		return nil, fmt.Errorf("frozen cycle producer boundary changed")
	}
	return []byte(strings.Replace(before, anchor, anchor+status, 1)), nil
}

func rgaS4RoutingGoldenDelta(name, got string, frozen []byte) error {
	want, err := rgaS4ExpectedRoutingGolden(name, frozen)
	if err != nil {
		return err
	}
	if got != string(want) {
		return fmt.Errorf("%s drifted beyond the S4 producer diagnostic\n--- expected ---\n%s\n--- current ---\n%s", name, want, got)
	}
	return nil
}

func TestRGAS4CycleGoldenDeltaAndSensitivities(t *testing.T) {
	const name = "cycle-skip-execute-transcript.txt"
	frozen, err := os.ReadFile(filepath.Join(routingGoldenDir, name))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := rgaS4ExpectedRoutingGolden(name, frozen)
	if err != nil || rgaS4RoutingGoldenDelta(name, string(expected), frozen) != nil {
		t.Fatalf("exact cycle diagnostic delta rejected: %v", err)
	}
	for _, wrong := range []string{
		string(frozen),
		strings.Replace(string(expected), "reference-not-durable", "invented-reason", 1),
		strings.Replace(string(expected), "recipe coverage: incomplete", "recipe coverage: complete", 1),
		string(expected) + "\n",
	} {
		if rgaS4RoutingGoldenDelta(name, wrong, frozen) == nil {
			t.Fatal("same cycle comparator accepted missing/wrong/extra output")
		}
	}
	if _, err := rgaS4ExpectedRoutingGolden(name, []byte(strings.Replace(string(frozen), "[4/6]", "[9/9]", 1))); err == nil {
		t.Fatal("cycle expected-delta adapter accepted a changed frozen boundary")
	}
}

func TestRGAS4GoldenPublicationDeltaAndSensitivities(t *testing.T) {
	for _, name := range []string{
		"compat-record.txt", "compat-land.txt", "compat-verify.txt", "compat-reconcile.txt",
		"phase-auto-implement.txt", "phase-manual-implement.txt",
	} {
		t.Run(name, func(t *testing.T) {
			frozen, err := os.ReadFile(filepath.Join(preparePIBGoldenDir, name))
			if err != nil {
				t.Fatal(err)
			}
			previous, err := rgaS2ExpectedProducerGolden(name, frozen)
			if err != nil {
				t.Fatal(err)
			}
			expected, err := rgaS4ExpectedPublicationGolden(name, previous)
			if err != nil {
				t.Fatal(err)
			}
			expected, err = rgaS5ExpectedCaptureEventGolden(name, expected)
			if err != nil {
				t.Fatal(err)
			}
			expected, err = rgaS5ExpectedVerifyGolden(name, expected)
			if err != nil || preparePIBGoldenDelta(name, string(expected)) != nil {
				t.Fatalf("S4/S5 composed expected delta rejected: %v", err)
			}
			coverageBody, err := rgaS2GoldenSection(string(expected), "artifacts/recipe-coverage.json")
			if err != nil {
				t.Fatal(err)
			}
			for _, mutation := range []struct{ old, replacement string }{
				{`"recipe_present": true`, `"recipe_present": false`},
				{`"recipe_decodable": true`, `"recipe_decodable": false`},
				{`"recipe_sha256": "`, `"recipe_sha256": "wrong`},
				{`"preimage_set_sha256": "`, `"preimage_set_sha256": "wrong`},
				{`"schema_version": 1`, `"schema_version": 2`},
			} {
				wrongBody := strings.Replace(coverageBody, mutation.old, mutation.replacement, 1)
				wrong := strings.Replace(string(expected), coverageBody, wrongBody, 1)
				if wrongBody == coverageBody || preparePIBGoldenDelta(name, wrong) == nil {
					t.Fatalf("same comparator accepted coverage drift %q", mutation.old)
				}
			}
			for _, wrong := range []string{string(previous), string(expected) + "\n", ""} {
				if preparePIBGoldenDelta(name, wrong) == nil {
					t.Fatal("same comparator accepted missing publication or unrelated drift")
				}
			}
			if strings.Contains(string(expected), "recipe coverage: ") {
				wrong := strings.Replace(string(expected), "recipe coverage: ", "missing coverage diagnostic: ", 1)
				if preparePIBGoldenDelta(name, wrong) == nil {
					t.Fatal("same comparator accepted loss of common producer status")
				}
			}
			invalid := strings.Replace(string(previous), `"type":`, `"wrong_type":`, 1)
			if _, err := rgaS4ExpectedPublicationGolden(name, []byte(invalid)); err == nil {
				t.Fatal("expected-delta adapter accepted a changed historical fixture shape")
			}
		})
	}
}
