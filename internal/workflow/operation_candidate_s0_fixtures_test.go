package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/provider"
)

type rocS0AdjacentExpectation struct {
	Name                  string   `json:"name"`
	BaseBoundary          int      `json:"base_boundary"`
	UpstreamBoundary      int      `json:"upstream_boundary"`
	DeletedArguments      []string `json:"deleted_arguments"`
	SurvivingArgument     string   `json:"surviving_argument"`
	ExpectedPhase         string   `json:"expected_phase"`
	ExpectedOutcome       string   `json:"expected_outcome"`
	ApplicableBoundaries  int      `json:"applicable_boundaries"`
	PresentBoundaries     int      `json:"present_boundaries"`
	ConflictingBoundaries int      `json:"conflicting_boundaries"`
}

type rocS0LegacyExpectation struct {
	GateOffPhase             string `json:"gate_off_phase"`
	GateOffOutcome           string `json:"gate_off_outcome"`
	GateOffReviewVerdict     string `json:"gate_off_review_verdict"`
	GateOnPhase              string `json:"gate_on_phase"`
	GateOnOutcome            string `json:"gate_on_outcome"`
	GateOnReviewVerdict      string `json:"gate_on_review_verdict"`
	GateOnCandidatePersisted bool   `json:"gate_on_candidate_persisted"`
	GateOnPhase2Reason       string `json:"gate_on_phase2_reason"`
}

type rocS0Expectations struct {
	BaselineCommit   string                     `json:"baseline_commit"`
	RuntimeStatus    string                     `json:"runtime_status"`
	LegacyAllPresent rocS0LegacyExpectation     `json:"legacy_all_present"`
	Adjacent         []rocS0AdjacentExpectation `json:"adjacent_arguments"`
}

func rocS0LoadExpectations(t *testing.T) rocS0Expectations {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "roc-s0", "candidate-expectations.json"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := rocS0DecodeExpectations(body)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func rocS0DecodeExpectations(body []byte) (rocS0Expectations, error) {
	var data rocS0Expectations
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&data); err != nil {
		return data, err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return data, fmt.Errorf("fixture has trailing JSON or invalid bytes: %v", err)
	}
	return data, nil
}

func rocS0ValidateExpectations(data rocS0Expectations) error {
	if data.BaselineCommit != rocS0Baseline || data.RuntimeStatus != "future-expectations-only" {
		return fmt.Errorf("fixture provenance or future-only boundary changed")
	}
	wantLegacy := rocS0LegacyExpectation{
		GateOffPhase: "phase-2-operation-level", GateOffOutcome: "blocked", GateOffReviewVerdict: "rejected-upstreamed",
		GateOnPhase: "phase-4-blocked", GateOnOutcome: "blocked", GateOnPhase2Reason: "coverage-missing",
	}
	if data.LegacyAllPresent != wantLegacy {
		return fmt.Errorf("legacy gate-on/off distinction changed: %+v", data.LegacyAllPresent)
	}
	if len(data.Adjacent) != 2 {
		return fmt.Errorf("both motivating adjacent-argument variants are required")
	}
	for i, row := range data.Adjacent {
		want := rocS0AdjacentExpectation{
			UpstreamBoundary: 4, ExpectedPhase: "phase-2-operation-candidate", ExpectedOutcome: "still_needed",
			ApplicableBoundaries: 1,
		}
		if i == 0 {
			want.Name, want.BaseBoundary = "delete-all", 6
			want.DeletedArguments = []string{"--old-a", "--old-b"}
		} else {
			want.Name, want.BaseBoundary, want.SurvivingArgument = "between-delete-first", 5, "--old-b"
			want.DeletedArguments = []string{"--old-a"}
		}
		if !reflect.DeepEqual(row, want) {
			return fmt.Errorf("candidate expectation changed: %+v", row)
		}
	}
	return nil
}

func rocS0InsertFixture(body string, boundary int) (string, error) {
	records := strings.SplitAfter(body, "\n")
	if boundary < 0 || boundary >= len(records) {
		return "", fmt.Errorf("invalid fixture boundary %d", boundary)
	}
	insertion := "\t\t\"--feature-x\",\n\t\t\"--feature-y\",\n"
	return strings.Join(records[:boundary], "") + insertion + strings.Join(records[boundary:], ""), nil
}

func rocS0ValidateAdjacentBytes(row rocS0AdjacentExpectation, base, feature, upstream, candidate string) error {
	wantFeature, err := rocS0InsertFixture(base, row.BaseBoundary)
	if err != nil || feature != wantFeature {
		return fmt.Errorf("feature is not the single required insertion: %v", err)
	}
	wantUpstream := base
	for _, arg := range row.DeletedArguments {
		wantUpstream = strings.ReplaceAll(wantUpstream, "\t\t\""+arg+"\",\n", "")
	}
	if upstream != wantUpstream {
		return fmt.Errorf("upstream deletion fixture changed")
	}
	wantCandidate, err := rocS0InsertFixture(upstream, row.UpstreamBoundary)
	if err != nil || candidate != wantCandidate {
		return fmt.Errorf("candidate does not preserve upstream with one insertion: %v", err)
	}
	return nil
}

func TestROCS0CandidateExpectations(t *testing.T) {
	data := rocS0LoadExpectations(t)
	if err := rocS0ValidateExpectations(data); err != nil {
		t.Fatal(err)
	}
	for _, row := range data.Adjacent {
		t.Run(row.Name, func(t *testing.T) {
			rocS0PinGit(t)
			base := rgaS0Fixture(t, "base.command.go.txt")
			feature := rgaS0Fixture(t, "feature.command.go.txt")
			upstream := rgaS0Fixture(t, "upstream.command.go.txt")
			candidate := rgaS0Fixture(t, "resolved.command.go.txt")
			if row.Name == "between-delete-first" {
				for name, target := range map[string]*string{
					"between-feature.command.go.txt":   &feature,
					"between-upstream.command.go.txt":  &upstream,
					"between-candidate.command.go.txt": &candidate,
				} {
					body, err := os.ReadFile(filepath.Join("testdata", "roc-s0", name))
					if err != nil {
						t.Fatal(err)
					}
					*target = string(body)
				}
			}
			if err := rocS0ValidateAdjacentBytes(row, base, feature, upstream, candidate); err != nil {
				t.Fatal(err)
			}
			for _, mutant := range []string{
				upstream,
				feature,
				strings.Replace(candidate, "\t\t\"--feature-x\",\n", "", 1),
				strings.Replace(candidate, "\t\t\"--feature-y\",\n", "\t\t\"--old-a\",\n", 1),
				candidate + "\n",
			} {
				if err := rocS0ValidateAdjacentBytes(row, base, feature, upstream, mutant); err == nil {
					t.Fatal("candidate fixture guard accepted a corrupted desired result")
				}
			}
			s := rocS0FreshStore(t)
			path := filepath.Join(s.Root, "command.go")
			rocS0WriteFile(t, path, base)
			gitAdd(t, s.Root, "command.go")
			gitCommit(t, s.Root, "adjacent base")
			rocS0WriteFile(t, path, feature)
			patch := gitOutput(t, s.Root, "diff", "HEAD", "--", "command.go")
			rocS0WriteFile(t, path, upstream)
			gitAdd(t, s.Root, "command.go")
			gitCommit(t, s.Root, "upstream removes old arguments")
			preview, err := gitutil.PreviewForwardApply(s.Root, patch)
			if err != nil {
				t.Fatal(err)
			}
			if preview.Verdict != gitutil.ForwardApply3WayConflicts {
				t.Fatalf("motivating fixture no longer conflicts textually: %+v", preview)
			}
		})
	}
}

func TestROCS0LegacyGateOnExpectationHasLaterPhaseOracle(t *testing.T) {
	rocS0PinGit(t)
	data := rocS0LoadExpectations(t)
	if err := rocS0ValidateExpectations(data); err != nil {
		t.Fatal(err)
	}
	for _, omitRecipe := range []bool{false, true} {
		s, slug := buildOperationUpstreamedCandidateFixture(t)
		if omitRecipe {
			if err := os.Remove(filepath.Join(s.Root, ".tpatch", "features", slug, "artifacts", "apply-recipe.json")); err != nil {
				t.Fatal(err)
			}
		}
		results, err := RunReconcile(context.Background(), s, []string{slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
		if err != nil || len(results) != 1 {
			t.Fatalf("legacy fixture: %v %+v", err, results)
		}
		want := data.LegacyAllPresent
		phase, outcome, review := want.GateOffPhase, want.GateOffOutcome, want.GateOffReviewVerdict
		if omitRecipe {
			// Removing the legacy early-return input is only a later-phase oracle,
			// not an implementation or test execution of the future config gate.
			phase, outcome, review = want.GateOnPhase, want.GateOnOutcome, want.GateOnReviewVerdict
		}
		if results[0].Phase != phase || string(results[0].Outcome) != outcome || results[0].ReviewVerdict != review {
			t.Fatalf("legacy/fallback oracle changed: %+v", results[0])
		}
	}
}

func TestROCS0CandidateExpectationSensitivity(t *testing.T) {
	good := rocS0LoadExpectations(t)
	if err := rocS0ValidateExpectations(good); err != nil {
		t.Fatal(err)
	}

	mutations := []func(*rocS0Expectations){
		func(d *rocS0Expectations) { d.RuntimeStatus = "implemented" },
		func(d *rocS0Expectations) { d.BaselineCommit = "unknown" },
		func(d *rocS0Expectations) { d.LegacyAllPresent.GateOffReviewVerdict = "confirmed-upstreamed" },
		func(d *rocS0Expectations) { d.LegacyAllPresent.GateOnPhase = "phase-2-operation-level" },
		func(d *rocS0Expectations) { d.LegacyAllPresent.GateOnCandidatePersisted = true },
		func(d *rocS0Expectations) { d.Adjacent = d.Adjacent[:1] },
		func(d *rocS0Expectations) { d.Adjacent[0].ExpectedOutcome = "blocked" },
		func(d *rocS0Expectations) { d.Adjacent[1].SurvivingArgument = "" },
		func(d *rocS0Expectations) { d.Adjacent[0].ApplicableBoundaries = 0 },
	}
	for i, mutate := range mutations {
		copy := good
		copy.Adjacent = append([]rocS0AdjacentExpectation(nil), good.Adjacent...)
		mutate(&copy)
		if err := rocS0ValidateExpectations(copy); err == nil {
			t.Fatalf("expectation mutation %d was accepted", i)
		}
	}
}

func TestROCS0ExpectationDecoderSensitivity(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "roc-s0", "candidate-expectations.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rocS0DecodeExpectations(body); err != nil {
		t.Fatal(err)
	}
	for _, mutant := range []string{
		"",
		"{",
		string(body) + "{}",
		strings.Replace(string(body), "{", `{"unknown":true,`, 1),
	} {
		if _, err := rocS0DecodeExpectations([]byte(mutant)); err == nil {
			t.Fatalf("fixture decoder accepted %q", mutant)
		}
	}
}
