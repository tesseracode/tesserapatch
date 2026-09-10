package cli

import (
	"bytes"
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

const rgaS5GoldenAddPatch = "diff --git a/src/golden.txt b/src/golden.txt\nnew file mode 100644\nindex 0000000..236c5dd\n--- /dev/null\n+++ b/src/golden.txt\n@@ -0,0 +1 @@\n+recorded change\n"

const rgaS5GoldenRecordedRecipe = `{
  "feature": "pib-golden",
  "operations": [
    {
      "type": "write-file",
      "path": "src/golden.txt",
      "content": "recorded change\n",
      "search": "",
      "replace": "",
      "preimage_hash": ""
    }
  ]
}
`

const rgaS5GoldenAutoRecipe = `{
  "feature": "pib-golden",
  "operations": [
    {
      "type": "ensure-directory",
      "path": "src/",
      "content": "",
      "search": "",
      "replace": ""
    }
  ]
}
`

const rgaS5GoldenManualRecipe = `{
  "feature": "pib-golden",
  "operations": [
    {"type": "ensure-directory", "path": "generated"}
  ]
}
`

func rgaS5GoldenHash(body string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(body)))
}

func rgaS5GoldenFrame(relative, body string) string {
	return fmt.Sprintf("--- .tpatch/features/pib-golden/%s (%d bytes) ---\n%s", relative, len(body), body)
}

// Only frozen fixture facts and the preceding independently expected S4 bytes
// enter this delta. Workflow types provide wire layout only, never an encoder,
// publisher, observation or validation oracle.
func rgaS5ExpectedCaptureEventGolden(name string, previous []byte) ([]byte, error) {
	recorded := false
	wantRecipe := rgaS5GoldenRecordedRecipe
	switch name {
	case "compat-record.txt", "compat-land.txt", "compat-verify.txt", "compat-reconcile.txt":
		recorded = true
	case "phase-auto-implement.txt":
		wantRecipe = rgaS5GoldenAutoRecipe
	case "phase-manual-implement.txt":
		wantRecipe = rgaS5GoldenManualRecipe
	default:
		return previous, nil
	}
	old := string(previous)
	if strings.Contains(old, "recipe-capture-event.json") {
		return nil, fmt.Errorf("S5 expected stage already contains capture evidence")
	}
	coverageBody, err := rgaS2GoldenSection(old, "artifacts/recipe-coverage.json")
	if err != nil {
		return nil, err
	}
	// Pair the exact raw expected C, including its LF, before any rendering.
	coverageHash := rgaS5GoldenHash(coverageBody)
	recipe, err := rgaS2GoldenSection(old, "artifacts/apply-recipe.json")
	if err != nil {
		return nil, err
	}
	if recipe != wantRecipe {
		return nil, fmt.Errorf("S5 expected stage lost its fixed recipe facts")
	}
	e := workflow.RecipeCaptureEvent{
		SchemaVersion: 1, Feature: "pib-golden",
		RecipePresent: true, RecipeSHA256: rgaS5GoldenHash(recipe),
		Reference:          workflow.CoverageReference{Kind: "unavailable", PreimageSetSHA256: rgaS5GoldenHash("[]")},
		Capture:            workflow.CoverageCapture{Mode: "no-capture", Pathspecs: []string{}, ClaimIDs: []string{}},
		ParentCreatedPaths: []string{},
		Observations:       []workflow.RecipeCaptureObservation{},
		CoverageSHA256:     coverageHash,
	}
	c := workflow.RecipeCoverage{
		SchemaVersion: 1, Feature: "pib-golden", Producer: "implement",
		RecipePresent: true, RecipeDecodable: true, RecipeSHA256: e.RecipeSHA256,
		CoverageStatus: "incomplete", CrossBaseStatus: "unsupported",
		Effects: []workflow.CoverageEffect{},
		Reasons: []string{"canonical-patch-missing", "operation-surplus", "reference-not-durable"},
	}
	if recorded {
		patch, err := rgaS2GoldenSection(old, "artifacts/post-apply.patch")
		if err != nil {
			return nil, err
		}
		if patch != rgaS5GoldenAddPatch {
			return nil, fmt.Errorf("S5 expected stage lost its fixed creation patch")
		}
		statusBody, err := rgaS2GoldenSection(old, "status.json")
		if err != nil {
			return nil, err
		}
		var status struct {
			Apply struct {
				BaseCommit string `json:"base_commit"`
			} `json:"apply"`
		}
		if err := json.Unmarshal([]byte(statusBody), &status); err != nil {
			return nil, err
		}
		base := "c920fdc71d5627cdfcbad47c4e2553364ae57a13"
		e.Capture.Mode = "committed-range"
		if name == "compat-record.txt" {
			base = "92fcf51ca3b4173cc9e5aa589d0f357ebed42ddf"
			e.Capture.Mode = "working-tree-all"
		}
		if status.Apply.BaseCommit != base {
			return nil, fmt.Errorf("S5 expected stage lost its fixed resolved lower commit")
		}
		post := rgaS5GoldenHash("recorded change\n")
		set := fmt.Sprintf(`[{"ordinal":1,"path_hex":%q,"old_path_hex":"","change_kind":"add","content_kind":"text","object_kind":"regular","old_mode":"","new_mode":"100644","preimage_observed":true,"preimage_present":false,"preimage_sha256":"","postimage_observed":true,"postimage_present":true,"postimage_sha256":%q}]`,
			hex.EncodeToString([]byte("src/golden.txt")), post)
		e.PatchPresent, e.PatchSHA256 = true, rgaS5GoldenHash(patch)
		e.Reference = workflow.CoverageReference{Kind: "commit", Commit: base, PreimageSetSHA256: rgaS5GoldenHash(set)}
		e.Event.PatchRewritten, e.Event.RecipeRegenerated = true, true
		e.Observations = []workflow.RecipeCaptureObservation{{
			Ordinal: 1, ChangeKind: "add", ContentKind: "text", ObjectKind: "regular",
			Path: "src/golden.txt", NewMode: "100644",
			PreimageObserved: true, PostimageObserved: true, PostimagePresent: true,
			PostimageSHA256: post, PatchFragmentSHA256: e.PatchSHA256,
		}}
		descriptor, err := json.Marshal(e.Observations[0])
		if err != nil {
			return nil, err
		}
		c.Producer, c.PatchPresent, c.PatchSHA256 = "record", true, e.PatchSHA256
		c.CoverageStatus, c.CrossBaseStatus, c.Reasons = "complete", "reference-tree-only", []string{}
		c.Effects = []workflow.CoverageEffect{{
			Ordinal: 1, ChangeKind: "add", ContentKind: "text", ObjectKind: "regular",
			Path: "src/golden.txt", NewMode: "100644",
			PreimageObserved: true, PostimageObserved: true, PostimagePresent: true,
			PostimageSHA256: post, PatchFragmentSHA256: e.PatchSHA256,
			EffectSHA256: rgaS5GoldenHash(string(descriptor)), OperationIndexes: []int{1},
			Disposition: "represented", ReasonCodes: []string{}, ContextualHint: "none",
		}}
	} else if strings.Contains(old, "--- .tpatch/features/pib-golden/artifacts/post-apply.patch (") {
		return nil, fmt.Errorf("S5 implement fixture unexpectedly contains a canonical patch")
	}
	c.Reference, c.Capture = e.Reference, e.Capture
	canonicalCoverage, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	if coverageBody != string(canonicalCoverage)+"\n" {
		return nil, fmt.Errorf("S5 prior expected coverage differs from the closed fixture facts")
	}
	body, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return nil, err
	}
	body = append(body, '\n')
	// These fixed fields contain no workspace/time tokens: normalization is
	// inert, and the serializer's post-normalization byte count is len(body).
	coverageFrame := rgaS5GoldenFrame("artifacts/recipe-coverage.json", coverageBody)
	if strings.Count(old, coverageFrame) != 1 {
		return nil, fmt.Errorf("S5 coverage frame is missing, ambiguous or noncanonical")
	}
	out := strings.Replace(old, coverageFrame,
		rgaS5GoldenFrame("artifacts/recipe-capture-event.json", string(body))+coverageFrame, 1)
	const stagedC = "   M .tpatch/features/pib-golden/artifacts/recipe-coverage.json\n"
	if name == "compat-land.txt" {
		if strings.Count(out, stagedC) != 1 {
			return nil, fmt.Errorf("S5 land staging point is missing or ambiguous")
		}
		const stagedE = "   M .tpatch/features/pib-golden/artifacts/recipe-capture-event.json\n"
		out = strings.Replace(out, stagedC, stagedE+stagedC, 1)
	} else if strings.Contains(out, stagedC) {
		return nil, fmt.Errorf("S5 non-land fixture unexpectedly stages coverage")
	}
	return []byte(out), nil
}

const rgaS5GoldenVerifyRow = `    {
      "id": "recipe_generation_coverage",
      "severity": "block",
      "passed": true
    }`

func rgaS5ExpectedVerifyGolden(name string, previous []byte) ([]byte, error) {
	if name != "compat-verify.txt" {
		return previous, nil
	}
	old := string(previous)
	const prefix = "$ tpatch --path <workspace> verify pib-golden --json --quiet --no-write\nexit 2\nstdout:\n"
	if !strings.HasPrefix(old, prefix) || strings.Contains(old, `"id": "recipe_generation_coverage"`) {
		return nil, fmt.Errorf("S5 verify fixture lost its original command/result or already contains the new row")
	}
	body, _, found := strings.Cut(strings.TrimPrefix(old, prefix), "\nstderr:\n")
	if !found {
		return nil, fmt.Errorf("S5 verify report boundary is missing")
	}
	var report struct {
		SchemaVersion string `json:"schema_version"`
		Slug          string `json:"slug"`
		Verdict       string `json:"verdict"`
		ExitCode      int    `json:"exit_code"`
		Checks        []struct {
			ID       string `json:"id"`
			Severity string `json:"severity"`
			Passed   bool   `json:"passed"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(body), &report); err != nil {
		return nil, err
	}
	ids := []string{"status_loaded", "intent_files_present", "recipe_parses", "recipe_op_targets_resolve",
		"dep_metadata_valid", "satisfied_by_reachable", "dependency_gate_satisfied", "recipe_replay_clean",
		"post_apply_patch_replay_clean", "reconcile_outcome_consistent", "write_file_preimage_fresh"}
	if report.SchemaVersion != "1.1" || report.Slug != "pib-golden" || report.Verdict != "failed" ||
		report.ExitCode != 2 || len(report.Checks) != len(ids) {
		return nil, fmt.Errorf("S5 verify fixture lost its original report shape/verdict")
	}
	for i, id := range ids {
		if report.Checks[i].ID != id {
			return nil, fmt.Errorf("S5 verify fixture changed original check %d", i)
		}
	}
	if report.Checks[8].Passed || report.Checks[8].Severity != "block" {
		return nil, fmt.Errorf("S5 verify fixture lost its independent V8 failure")
	}
	coverageBody, err := rgaS2GoldenSection(old, "artifacts/recipe-coverage.json")
	if err != nil {
		return nil, err
	}
	eventBody, err := rgaS2GoldenSection(old, "artifacts/recipe-capture-event.json")
	if err != nil {
		return nil, err
	}
	var coverage struct {
		Feature string `json:"feature"`
		Status  string `json:"coverage_status"`
	}
	var event struct {
		Feature string `json:"feature"`
		Hash    string `json:"coverage_sha256"`
	}
	if err := json.Unmarshal([]byte(coverageBody), &coverage); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(eventBody), &event); err != nil {
		return nil, err
	}
	if coverage.Feature != "pib-golden" || coverage.Status != "complete" ||
		event.Feature != coverage.Feature || event.Hash != rgaS5GoldenHash(coverageBody) {
		return nil, fmt.Errorf("S5 verify expected stage lacks its independently derived complete pair")
	}
	const footer = "    }\n  ],\n  \"lifecycle_state\": \"applied\","
	if strings.Count(old, footer) != 1 {
		return nil, fmt.Errorf("S5 verify check-list boundary is missing or ambiguous")
	}
	return []byte(strings.Replace(old, footer, "    },\n"+rgaS5GoldenVerifyRow+"\n  ],\n  \"lifecycle_state\": \"applied\",", 1)), nil
}

func rgaS5ReplaceGoldenSection(t *testing.T, transcript, relative, body string) string {
	t.Helper()
	oldBody, err := rgaS2GoldenSection(transcript, relative)
	if err != nil {
		t.Fatal(err)
	}
	oldFrame := rgaS5GoldenFrame(relative, oldBody)
	if strings.Count(transcript, oldFrame) != 1 {
		t.Fatalf("mutation lacks one canonical frame for %s", relative)
	}
	return strings.Replace(transcript, oldFrame, rgaS5GoldenFrame(relative, body), 1)
}

func TestRGAS5GoldenCaptureEventDeltaAndSensitivities(t *testing.T) {
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
			previous, err = rgaS4ExpectedPublicationGolden(name, previous)
			if err != nil {
				t.Fatal(err)
			}
			expected, err := rgaS5ExpectedCaptureEventGolden(name, previous)
			if err != nil {
				t.Fatal(err)
			}
			expected, err = rgaS5ExpectedVerifyGolden(name, expected)
			if err != nil {
				t.Fatal(err)
			}
			if err := preparePIBGoldenDelta(name, string(expected)); err != nil {
				t.Fatalf("S2/S4/S5 composed expectation refused: %v", err)
			}
			eventBody, err := rgaS2GoldenSection(string(expected), "artifacts/recipe-capture-event.json")
			if err != nil {
				t.Fatal(err)
			}
			coverageBody, err := rgaS2GoldenSection(string(previous), "artifacts/recipe-coverage.json")
			if err != nil {
				t.Fatal(err)
			}
			eventFrame := rgaS5GoldenFrame("artifacts/recipe-capture-event.json", eventBody)
			coverageFrame := rgaS5GoldenFrame("artifacts/recipe-coverage.json", coverageBody)
			if !strings.Contains(string(expected), eventFrame+coverageFrame) {
				t.Fatal("E must be immediately before the complete C frame")
			}
			var event workflow.RecipeCaptureEvent
			if err := json.Unmarshal([]byte(eventBody), &event); err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(coverageBody, "\n") || event.CoverageSHA256 != rgaS5GoldenHash(coverageBody) ||
				event.CoverageSHA256 == rgaS5GoldenHash(strings.TrimSuffix(coverageBody, "\n")) {
				t.Fatal("E does not bind the exact raw expected C including its LF")
			}
			mutations := map[string]func(*workflow.RecipeCaptureEvent){
				"pair-hash":          func(e *workflow.RecipeCaptureEvent) { e.CoverageSHA256 = strings.Repeat("a", 64) },
				"schema":             func(e *workflow.RecipeCaptureEvent) { e.SchemaVersion = 2 },
				"owner":              func(e *workflow.RecipeCaptureEvent) { e.Feature = "other-feature" },
				"patch-presence":     func(e *workflow.RecipeCaptureEvent) { e.PatchPresent = !e.PatchPresent },
				"recipe-presence":    func(e *workflow.RecipeCaptureEvent) { e.RecipePresent = false },
				"patch-hash":         func(e *workflow.RecipeCaptureEvent) { e.PatchSHA256 = strings.Repeat("a", 64) },
				"recipe-hash":        func(e *workflow.RecipeCaptureEvent) { e.RecipeSHA256 = strings.Repeat("a", 64) },
				"reference-digest":   func(e *workflow.RecipeCaptureEvent) { e.Reference.PreimageSetSHA256 = strings.Repeat("a", 64) },
				"reference-kind":     func(e *workflow.RecipeCaptureEvent) { e.Reference.Kind = "index-snapshot" },
				"reference-commit":   func(e *workflow.RecipeCaptureEvent) { e.Reference.Commit = strings.Repeat("a", 40) },
				"capture-mode":       func(e *workflow.RecipeCaptureEvent) { e.Capture.Mode = "staged-index" },
				"patch-event":        func(e *workflow.RecipeCaptureEvent) { e.Event.PatchRewritten = !e.Event.PatchRewritten },
				"regeneration-event": func(e *workflow.RecipeCaptureEvent) { e.Event.RecipeRegenerated = !e.Event.RecipeRegenerated },
				"edit-event":         func(e *workflow.RecipeCaptureEvent) { e.Event.BoundArtifactEdited = true },
				"marker-event":       func(e *workflow.RecipeCaptureEvent) { e.Event.StaleMarkerPresent = true },
				"parent-set":         func(e *workflow.RecipeCaptureEvent) { e.ParentCreatedPaths = []string{"src/golden.txt"} },
				"null-parent-set":    func(e *workflow.RecipeCaptureEvent) { e.ParentCreatedPaths = nil },
				"pathspecs":          func(e *workflow.RecipeCaptureEvent) { e.Capture.Pathspecs = []string{"src/golden.txt"} },
				"null-pathspecs":     func(e *workflow.RecipeCaptureEvent) { e.Capture.Pathspecs = nil },
				"claim-ids":          func(e *workflow.RecipeCaptureEvent) { e.Capture.ClaimIDs = []string{"claim"} },
				"null-claim-ids":     func(e *workflow.RecipeCaptureEvent) { e.Capture.ClaimIDs = nil },
				"null-observations":  func(e *workflow.RecipeCaptureEvent) { e.Observations = nil },
				"observation-count": func(e *workflow.RecipeCaptureEvent) {
					e.Observations = append(e.Observations, workflow.RecipeCaptureObservation{Ordinal: 2})
				},
			}
			if len(event.Observations) != 0 {
				for field, mutate := range map[string]func(*workflow.RecipeCaptureObservation){
					"pre-observed":  func(o *workflow.RecipeCaptureObservation) { o.PreimageObserved = false },
					"pre-present":   func(o *workflow.RecipeCaptureObservation) { o.PreimagePresent = true },
					"post-observed": func(o *workflow.RecipeCaptureObservation) { o.PostimageObserved = false },
					"post-present":  func(o *workflow.RecipeCaptureObservation) { o.PostimagePresent = false },
					"old-mode":      func(o *workflow.RecipeCaptureObservation) { o.OldMode = "100644" },
					"new-mode":      func(o *workflow.RecipeCaptureObservation) { o.NewMode = "100755" },
					"pre-digest":    func(o *workflow.RecipeCaptureObservation) { o.PreimageSHA256 = strings.Repeat("a", 64) },
					"post-digest":   func(o *workflow.RecipeCaptureObservation) { o.PostimageSHA256 = strings.Repeat("a", 64) },
					"fragment":      func(o *workflow.RecipeCaptureObservation) { o.PatchFragmentSHA256 = strings.Repeat("a", 64) },
					"path":          func(o *workflow.RecipeCaptureObservation) { o.Path = "other.txt" },
					"old-path":      func(o *workflow.RecipeCaptureObservation) { o.OldPath = "old.txt" },
					"ordinal":       func(o *workflow.RecipeCaptureObservation) { o.Ordinal = 2 },
					"change-kind":   func(o *workflow.RecipeCaptureObservation) { o.ChangeKind = "modify" },
					"content-kind":  func(o *workflow.RecipeCaptureObservation) { o.ContentKind = "binary" },
					"object-kind":   func(o *workflow.RecipeCaptureObservation) { o.ObjectKind = "executable" },
				} {
					mutations[field] = func(e *workflow.RecipeCaptureEvent) { mutate(&e.Observations[0]) }
				}
			}
			for mutation, mutate := range mutations {
				t.Run(mutation, func(t *testing.T) {
					var changed workflow.RecipeCaptureEvent
					if err := json.Unmarshal([]byte(eventBody), &changed); err != nil {
						t.Fatal(err)
					}
					mutate(&changed)
					body, err := json.MarshalIndent(changed, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					body = append(body, '\n')
					wrong := rgaS5ReplaceGoldenSection(t, string(expected), "artifacts/recipe-capture-event.json", string(body))
					if bytes.Equal(body, []byte(eventBody)) || preparePIBGoldenDelta(name, wrong) == nil {
						t.Fatal("shared final comparator accepted changed E with a correct frame length")
					}
				})
			}
			withoutE := strings.Replace(string(expected), eventFrame, "", 1)
			wrongPlacement := strings.Replace(withoutE, coverageFrame, coverageFrame+eventFrame, 1)
			for mutation, wrong := range map[string]string{
				"missing-E":          withoutE,
				"missing-S5-stage":   string(previous),
				"duplicate-E":        strings.Replace(string(expected), eventFrame, eventFrame+eventFrame, 1),
				"wrong-placement":    wrongPlacement,
				"wrong-frame-length": strings.Replace(string(expected), fmt.Sprintf("recipe-capture-event.json (%d bytes)", len(eventBody)), "recipe-capture-event.json (0 bytes)", 1),
				"extra-artifact":     string(expected) + rgaS5GoldenFrame("artifacts/unrelated.json", "{}\n"),
				"unrelated-output":   strings.Replace(string(expected), "stdout:\n", "stdout:\nunrelated change\n", 1),
			} {
				t.Run(mutation, func(t *testing.T) {
					if wrong == string(expected) || preparePIBGoldenDelta(name, wrong) == nil {
						t.Fatal("shared final comparator accepted missing/misframed/extra evidence or unrelated output")
					}
				})
			}
			for _, relative := range []string{"status.json", "artifacts/apply-recipe.json"} {
				body, err := rgaS2GoldenSection(string(expected), relative)
				if err != nil {
					t.Fatal(err)
				}
				old, replacement := `"state": "`, `"state": "changed-`
				if relative == "artifacts/apply-recipe.json" {
					old, replacement = `"path": "`, `"path": "changed-`
				}
				changed := strings.Replace(body, old, replacement, 1)
				wrong := rgaS5ReplaceGoldenSection(t, string(expected), relative, changed)
				if body == changed || preparePIBGoldenDelta(name, wrong) == nil {
					t.Fatalf("shared final comparator accepted unrelated change to %s", relative)
				}
			}
			if name == "compat-land.txt" {
				const stagedE = "   M .tpatch/features/pib-golden/artifacts/recipe-capture-event.json\n"
				const stagedC = "   M .tpatch/features/pib-golden/artifacts/recipe-coverage.json\n"
				if strings.Count(string(expected), stagedE) != 1 || !strings.Contains(string(expected), stagedE+stagedC) {
					t.Fatal("land must add exactly one staged E immediately before C")
				}
				for _, wrong := range []string{
					strings.Replace(string(expected), stagedE, "", 1),
					strings.Replace(string(expected), stagedE, stagedE+stagedE, 1),
					strings.Replace(string(expected), stagedE+stagedC, stagedC+stagedE, 1),
				} {
					if preparePIBGoldenDelta(name, wrong) == nil {
						t.Fatal("shared final comparator accepted missing/duplicate/reordered staged E")
					}
				}
			}
			recipeBody, err := rgaS2GoldenSection(string(previous), "artifacts/apply-recipe.json")
			if err != nil {
				t.Fatal(err)
			}
			invalidInputs := map[string]string{
				"already-E":       string(expected),
				"missing-C":       strings.Replace(string(previous), coverageFrame, "", 1),
				"duplicate-C":     string(previous) + coverageFrame,
				"wrong-C-length":  strings.Replace(string(previous), fmt.Sprintf("recipe-coverage.json (%d bytes)", len(coverageBody)), "recipe-coverage.json (0 bytes)", 1),
				"changed-C-facts": rgaS5ReplaceGoldenSection(t, string(previous), "artifacts/recipe-coverage.json", strings.Replace(coverageBody, `"recipe_present": true`, `"recipe_present": false`, 1)),
				"changed-C-LF":    rgaS5ReplaceGoldenSection(t, string(previous), "artifacts/recipe-coverage.json", coverageBody+"\n"),
				"changed-recipe":  rgaS5ReplaceGoldenSection(t, string(previous), "artifacts/apply-recipe.json", strings.Replace(recipeBody, `"path": "`, `"path": "changed-`, 1)),
			}
			if event.PatchPresent {
				statusBody, err := rgaS2GoldenSection(string(previous), "status.json")
				if err != nil {
					t.Fatal(err)
				}
				invalidInputs["changed-patch"] = rgaS5ReplaceGoldenSection(t, string(previous), "artifacts/post-apply.patch",
					strings.Replace(rgaS5GoldenAddPatch, "+recorded change", "+unrelated change", 1))
				invalidInputs["changed-reference"] = rgaS5ReplaceGoldenSection(t, string(previous), "status.json",
					strings.Replace(statusBody, event.Reference.Commit, strings.Repeat("a", 40), 1))
			} else {
				invalidInputs["unexpected-patch"] = string(previous) + rgaS5GoldenFrame("artifacts/post-apply.patch", rgaS5GoldenAddPatch)
			}
			if name == "compat-land.txt" {
				const stagedC = "   M .tpatch/features/pib-golden/artifacts/recipe-coverage.json\n"
				invalidInputs["missing-staged-C"] = strings.Replace(string(previous), stagedC, "", 1)
				invalidInputs["duplicate-staged-C"] = strings.Replace(string(previous), stagedC, stagedC+stagedC, 1)
			}
			for mutation, invalid := range invalidInputs {
				t.Run("expected-input-"+mutation, func(t *testing.T) {
					if _, err := rgaS5ExpectedCaptureEventGolden(name, []byte(invalid)); err == nil {
						t.Fatal("S5 expected adapter accepted changed fixture facts or ambiguous framing")
					}
				})
			}
		})
	}
}

func TestRGAS5VerifyGoldenDeltaAndSensitivities(t *testing.T) {
	const name = "compat-verify.txt"
	frozen, err := os.ReadFile(filepath.Join(preparePIBGoldenDir, name))
	if err != nil {
		t.Fatal(err)
	}
	previous, err := rgaS2ExpectedProducerGolden(name, frozen)
	if err != nil {
		t.Fatal(err)
	}
	previous, err = rgaS4ExpectedPublicationGolden(name, previous)
	if err != nil {
		t.Fatal(err)
	}
	previous, err = rgaS5ExpectedCaptureEventGolden(name, previous)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := rgaS5ExpectedVerifyGolden(name, previous)
	if err != nil || preparePIBGoldenDelta(name, string(expected)) != nil {
		t.Fatalf("exact new verify row was rejected: %v", err)
	}
	current := string(expected)
	for name, wrong := range map[string]string{
		"missing-row":           string(previous),
		"wrong-id":              strings.Replace(current, `"id": "recipe_generation_coverage"`, `"id": "coverage"`, 1),
		"wrong-severity":        strings.Replace(current, rgaS5GoldenVerifyRow, strings.Replace(rgaS5GoldenVerifyRow, `"block"`, `"warn"`, 1), 1),
		"failed-new-row":        strings.Replace(current, rgaS5GoldenVerifyRow, strings.Replace(rgaS5GoldenVerifyRow, `"passed": true`, `"passed": false`, 1), 1),
		"duplicate-row":         strings.Replace(current, rgaS5GoldenVerifyRow, rgaS5GoldenVerifyRow+",\n"+rgaS5GoldenVerifyRow, 1),
		"gratuitous-mode":       strings.Replace(current, rgaS5GoldenVerifyRow, strings.Replace(rgaS5GoldenVerifyRow, `"passed": true`, "\"passed\": true,\n      \"mode\": \"forward\"", 1), 1),
		"false-overall-green":   strings.NewReplacer("exit 2\n", "exit 0\n", `"verdict": "failed"`, `"verdict": "passed"`, `"exit_code": 2`, `"exit_code": 0`).Replace(current),
		"old-V8-failure-erased": strings.Replace(current, "\"id\": \"post_apply_patch_replay_clean\",\n      \"severity\": \"block\",\n      \"passed\": false", "\"id\": \"post_apply_patch_replay_clean\",\n      \"severity\": \"block\",\n      \"passed\": true", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if wrong == current || preparePIBGoldenDelta("compat-verify.txt", wrong) == nil {
				t.Fatal("same final comparator accepted the wrong verify report")
			}
		})
	}
	if _, err := rgaS5ExpectedVerifyGolden(name, expected); err == nil {
		t.Fatal("verify adapter accepted an already-applied row delta")
	}
	if _, err := rgaS5ExpectedVerifyGolden(name, frozen); err == nil {
		t.Fatal("verify adapter claimed a passing coverage row without expected pair evidence")
	}
	unaffected := []byte("unrelated fixture bytes\n")
	if got, err := rgaS5ExpectedVerifyGolden("compat-record.txt", unaffected); err != nil || !bytes.Equal(got, unaffected) {
		t.Fatal("verify-only delta changed an unrelated fixture")
	}
}

func TestRGAS5GoldenUnaffectedFixtureStaysExact(t *testing.T) {
	const name = "phase-auto-explore.txt"
	frozen, err := os.ReadFile(filepath.Join(preparePIBGoldenDir, name))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := rgaS5ExpectedCaptureEventGolden(name, frozen)
	if err != nil || !bytes.Equal(expected, frozen) {
		t.Fatalf("S5 changed an unaffected fixture: %v", err)
	}
	if err := preparePIBGoldenDelta(name, string(expected)); err != nil {
		t.Fatal(err)
	}
	if preparePIBGoldenDelta(name, string(expected)+"unrelated output\n") == nil {
		t.Fatal("shared final comparator stopped enforcing unaffected fixture bytes")
	}
}
