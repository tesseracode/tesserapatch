package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

// The test executable is a portable synchronous editor, avoiding shell/script
// differences. Normal invocations never set this dedicated helper variable.
func init() {
	mode := os.Getenv("TPATCH_RGA_S4_EDITOR_HELPER")
	if mode == "" {
		return
	}
	target := os.Args[len(os.Args)-1]
	switch mode {
	case "write":
		if err := os.WriteFile(target, []byte(os.Getenv("TPATCH_RGA_S4_EDITOR_BYTES")), 0o644); err != nil {
			os.Exit(92)
		}
	case "directory":
		if os.Remove(target) != nil || os.Mkdir(target, 0o755) != nil {
			os.Exit(93)
		}
	}
	if os.Getenv("TPATCH_RGA_S4_EDITOR_FAIL") != "" {
		os.Exit(37)
	}
	os.Exit(0)
}

func rgaS4SetEditor(t *testing.T, mode, body string, fail bool) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", executable)
	t.Setenv("TPATCH_RGA_S4_EDITOR_HELPER", mode)
	t.Setenv("TPATCH_RGA_S4_EDITOR_BYTES", body)
	if fail {
		t.Setenv("TPATCH_RGA_S4_EDITOR_FAIL", "1")
	} else {
		t.Setenv("TPATCH_RGA_S4_EDITOR_FAIL", "")
	}
}

func TestRGAS4EditEventBoundaryAndEditorErrors(t *testing.T) {
	for _, tc := range []struct {
		name, artifact, mode        string
		decoy, fail, event, unknown bool
	}{
		{name: "recipe", artifact: "apply-recipe.json", mode: "write", event: true},
		{name: "patch", artifact: "post-apply.patch", mode: "write", event: true},
		{name: "root-decoy", artifact: "apply-recipe.json", mode: "write", decoy: true},
		{name: "explicit-canonical-with-decoy", artifact: "artifacts/apply-recipe.json", mode: "write", decoy: true, event: true},
		{name: "unrelated", artifact: "request.md", mode: "write"},
		{name: "no-change", artifact: "apply-recipe.json", mode: "noop"},
		{name: "unset-editor", artifact: "apply-recipe.json"},
		{name: "error-after-write", artifact: "apply-recipe.json", mode: "write", fail: true, event: true},
		{name: "error-without-write", artifact: "apply-recipe.json", mode: "noop", fail: true},
		{name: "unknown-after-snapshot", artifact: "apply-recipe.json", mode: "directory", event: true, unknown: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			slug := "s4-edit"
			root := rgaS4CLIFixture(t, slug, false)
			base := filepath.Join(root, ".tpatch", "features", slug)
			artifacts := filepath.Join(base, "artifacts")
			for _, name := range []string{"apply-recipe.json", "post-apply.patch"} {
				if err := os.WriteFile(filepath.Join(artifacts, name), []byte("old bytes\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			coveragePath := filepath.Join(artifacts, "recipe-coverage.json")
			legacy := []byte("pre-event coverage sentinel\n")
			if err := os.WriteFile(coveragePath, legacy, 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.decoy {
				if err := os.WriteFile(filepath.Join(base, "apply-recipe.json"), []byte("decoy\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.mode == "" {
				t.Setenv("EDITOR", "")
			} else {
				rgaS4SetEditor(t, tc.mode, "new bytes\n", tc.fail)
			}
			_, stderr, code := runCmdWithError("edit", "--path", root, slug, tc.artifact)
			if (code != 0) != (tc.fail || tc.unknown) {
				t.Fatalf("editor outcome code=%d stderr=%s", code, stderr)
			}
			raw, err := os.ReadFile(coveragePath)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.event {
				if !bytes.Equal(raw, legacy) {
					t.Fatal("non-event changed coverage")
				}
				return
			}
			c := rgaS4CLICoverage(t, root, slug)
			if c.Producer != patchobs.ProducerEdit || c.CoverageStatus != workflow.CoverageIncomplete ||
				!slices.Contains(c.Reasons, "manual-bound-artifact-edit") ||
				!slices.Contains(c.Reasons, "reference-not-durable") {
				t.Fatalf("P7 mutation not honestly bound: %+v", c)
			}
		})
	}
}

func TestRGAS4EditReconstructsOnlyValidatedReference(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(map[bool]string{false: "validated", true: "stale-preimage-digest"}[corrupt], func(t *testing.T) {
			slug := "s4-edit-reference"
			root := rgaS4CLIFixture(t, slug, true)
			if _, stderr, code := runRecord(t, "record", "--path", root, slug, "--lenient"); code != 0 {
				t.Fatal(stderr)
			}
			before := rgaS4CLICoverage(t, root, slug)
			artifacts := filepath.Join(root, ".tpatch", "features", slug, "artifacts")
			if corrupt {
				before.Reference.PreimageSetSHA256 = strings.Repeat("b", 64)
				// This is deliberately tampered input; bypass the encoder.
				raw, _ := json.Marshal(before)
				if err := os.WriteFile(filepath.Join(artifacts, "recipe-coverage.json"), raw, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			recipe, err := os.ReadFile(filepath.Join(artifacts, "apply-recipe.json"))
			if err != nil {
				t.Fatal(err)
			}
			var compact bytes.Buffer
			if err := json.Compact(&compact, recipe); err != nil {
				t.Fatal(err)
			}
			rgaS4SetEditor(t, "write", compact.String(), false)
			var observed []patchobs.Observation
			restore := patchobs.SetRecorder(rgaS4CLIRecorder(func(obs patchobs.Observation) {
				observed = append(observed, obs)
			}))
			t.Cleanup(restore)
			if _, stderr, code := runCmdWithError("edit", "--path", root, slug, "artifacts/apply-recipe.json"); code != 0 {
				t.Fatal(stderr)
			}
			after := rgaS4CLICoverage(t, root, slug)
			if len(observed) != 1 || observed[0].Reference.Kind != after.Reference.Kind ||
				observed[0].Reference.Commit != after.Reference.Commit ||
				observed[0].Reference.PreimageSetSHA256 != after.Reference.PreimageSetSHA256 {
				t.Fatal("P7 publication changed the emitted immutable observation")
			}
			if !corrupt && (after.Reference.Kind != patchobs.ReferenceKindCommit || after.CoverageStatus != workflow.CoverageComplete) {
				t.Fatalf("validated reference was lost: %+v", after)
			}
			if corrupt && (after.Reference.Kind != patchobs.ReferenceKindUnavailable || after.CoverageStatus != workflow.CoverageIncomplete) {
				t.Fatalf("unvalidated reference was carried forward: %+v", after)
			}
		})
	}
}

func TestRGAS4EditPrimaryAndPublicationErrorOrdering(t *testing.T) {
	slug := "s4-edit-both"
	root := rgaS4CLIFixture(t, slug, false)
	artifacts := filepath.Join(root, ".tpatch", "features", slug, "artifacts")
	if err := os.WriteFile(filepath.Join(artifacts, "apply-recipe.json"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(artifacts, "recipe-coverage.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	rgaS4SetEditor(t, "write", "changed", true)
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err = runEditWithObservation(cmd, s, slug, filepath.Join(artifacts, "apply-recipe.json"))
	if err == nil {
		t.Fatal("editor and publication failures disappeared")
	}
	editor, publication := strings.Index(err.Error(), "exit status 37"), strings.Index(err.Error(), "publish coverage")
	if editor < 0 || publication <= editor {
		t.Fatalf("editor must precede chained publication failure: %v", err)
	}
}

func TestRGAS4CycleSeparatesImplementAndPatchEvents(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		dirty, skip, failPatch bool
	}{
		{"skip-execute", true, true, false},
		{"empty-capture", false, false, false},
		{"patch-event", true, false, false},
		{"failed-patch-write", true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			slug := "s4-cycle"
			root := rgaS4CLIFixture(t, slug, tc.dirty)
			args := []string{"cycle", "--path", root, slug}
			if tc.skip {
				args = append(args, "--skip-execute")
			}
			if tc.failPatch {
				if err := os.Mkdir(filepath.Join(root, ".tpatch", "features", slug, "artifacts", "post-apply.patch"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			_, stderr, code := runCmdWithError(args...)
			if (code != 0) != tc.failPatch {
				t.Fatalf("cycle code=%d stderr=%s", code, stderr)
			}
			c := rgaS4CLICoverage(t, root, slug)
			want := patchobs.ProducerImplement
			if tc.dirty && !tc.skip && !tc.failPatch {
				want = patchobs.ProducerCycle
			}
			if c.Producer != want {
				t.Fatalf("event producer=%s want=%s", c.Producer, want)
			}
		})
	}
}

func TestRGAS4ApplyDoneEmptyAndReapplyDoNotPublish(t *testing.T) {
	for _, reapply := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty", true: "reapply"}[reapply], func(t *testing.T) {
			var s *store.Store
			var root, slug string
			if reapply {
				fx := newUnapplyFixture(t, store.StateUnapplied)
				s, root, slug = fx.store, fx.dir, fx.slug
			} else {
				slug = "s4-empty"
				root = rgaS4CLIFixture(t, slug, false)
				var err error
				s, err = store.Open(root)
				if err != nil {
					t.Fatal(err)
				}

			}
			sentinel := "unchanged no-event coverage\n"
			if err := s.WriteArtifact(slug, "recipe-coverage.json", sentinel); err != nil {
				t.Fatal(err)
			}
			cmd := &cobra.Command{}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			if _, _, err := runApplyDone(cmd, s, slug); err != nil {
				t.Fatal(err)
			}
			after, err := s.ReadFeatureFile(slug, "artifacts/recipe-coverage.json")
			if err != nil || after != sentinel {
				t.Fatal("no-event apply changed coverage")
			}
		})
	}
}

func TestRGAS4ManualCheckpointPublishesAfterAcceptanceOnValidatedBytes(t *testing.T) {
	for _, phase := range []string{"implement", "analyze", "define", "explore"} {
		t.Run(phase, func(t *testing.T) {
			slug := "s4-manual"
			root := rgaS4CLIFixture(t, slug, false)
			s, err := store.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			contract, ok := store.ManualPhase(phase)
			if !ok {
				t.Fatal("manual phase missing")
			}
			path := filepath.Join(featureDirPath(s, slug), contract.Path)
			body := "manually authored\n"
			if phase == "implement" {
				body = "{\"feature\":\"" + slug + "\",\"operations\":[]}\n"
			}
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			var observations []patchobs.Observation
			restore := patchobs.SetRecorder(rgaS4CLIRecorder(func(obs patchobs.Observation) {
				observations = append(observations, obs)
				state, err := s.LoadFeatureStatus(slug)
				if err != nil || state.State == store.StateImplementing {
					t.Fatal("checkpoint observation must precede state transition")
				}
				if _, err := s.ReadFeatureFile(slug, "artifacts/recipe-coverage.json"); !os.IsNotExist(err) {
					t.Fatal("pre-write observation seam published coverage")
				}
			}))
			t.Cleanup(restore)
			cmd := &cobra.Command{}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			if err := runManualPhase(cmd, s, slug, phase); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != body {
				t.Fatal("manual checkpoint rewrote validated bytes")
			}
			if phase != "implement" {
				rgaS0AssertNoCLICoverageArtifact(t, root, slug)
				if len(observations) != 0 {
					t.Fatal("unrelated manual phase emitted a producer event")
				}
				return
			}
			c := rgaS4CLICoverage(t, root, slug)
			if len(observations) != 1 || c.Producer != patchobs.ProducerImplement ||
				c.RecipeSHA256 != observations[0].ArtifactAfter.SHA256 {
				t.Fatalf("manual checkpoint lost exact validated bytes: %+v", c)
			}
		})
	}
}

func TestRGAS4ManualRefusalAndStateFailureAreNotAcceptedCheckpoints(t *testing.T) {
	for _, failure := range []string{"missing", "invalid", "state"} {
		t.Run(failure, func(t *testing.T) {
			slug := "s4-manual-refusal"
			root := rgaS4CLIFixture(t, slug, false)
			s, err := store.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			if failure != "missing" {
				body := "{}\n"
				if failure == "invalid" {
					body = "not json\n"
				}
				if err := s.WriteArtifact(slug, "apply-recipe.json", body); err != nil {
					t.Fatal(err)
				}
			}
			if failure == "state" {
				restore := patchobs.SetRecorder(rgaS4CLIRecorder(func(patchobs.Observation) {
					path := filepath.Join(featureDirPath(s, slug), "status.json")
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(path, 0o755); err != nil {
						t.Fatal(err)
					}
				}))
				t.Cleanup(restore)
			}
			cmd := &cobra.Command{}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			if err := runManualPhase(cmd, s, slug, "implement"); err == nil {
				t.Fatal("failed checkpoint reported success")
			}
			rgaS0AssertNoCLICoverageArtifact(t, root, slug)
		})
	}
}

func TestRGAS4CycleDeclinedPromptsLeaveP6Coverage(t *testing.T) {
	for _, answers := range []string{"y\ny\ny\nn\n", "y\ny\ny\ny\nn\n"} {
		t.Run(strings.ReplaceAll(answers, "\n", "-"), func(t *testing.T) {
			slug := "s4-cycle-decline"
			root := rgaS4CLIFixture(t, slug, true)
			cmd := buildRootCmd()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetIn(strings.NewReader(answers))
			cmd.SetArgs([]string{"cycle", "--path", root, slug, "--interactive"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			c := rgaS4CLICoverage(t, root, slug)
			if c.Producer != patchobs.ProducerImplement || c.PatchPresent {
				t.Fatalf("declined prompt invented P4 event: %+v", c)
			}
		})
	}
}

func TestRGAS4P2CheckpointBindsActualTamperedCanonicalArtifact(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "unparseable", true: "missing"}[missing], func(t *testing.T) {
			slug := "s4-checkpoint-actual"
			root := rgaS4CLIFixture(t, slug, true)
			if _, stderr, code := runRecord(t, "record", "--path", root, slug, "--lenient"); code != 0 {
				t.Fatal(stderr)
			}
			path := filepath.Join(root, ".tpatch", "features", slug, "artifacts", "post-apply.patch")
			if missing {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte("malformed canonical bytes"), 0o644); err != nil {
				t.Fatal(err)
			}
			before := rgaS4FeatureBytes(t, root, slug, true)
			if _, stderr, code := runCmdWithError("feature", "patch", "refresh", "--path", root, slug); code != 0 {
				t.Fatal(stderr)
			}
			c := rgaS4CLICoverage(t, root, slug)
			if c.PatchPresent == missing || c.CoverageStatus != workflow.CoverageIncomplete {
				t.Fatalf("checkpoint bound captured bytes that it did not write: %+v", c)
			}
			if after := rgaS4FeatureBytes(t, root, slug, true); !reflect.DeepEqual(before, after) {
				t.Fatal("checkpoint repaired a non-coverage artifact")
			}
		})
	}
}

func TestRGAS4ReobservationCheckpointProducerIsClosed(t *testing.T) {
	slug := "s4-closed-checkpoint"
	root := rgaS4CLIFixture(t, slug, false)
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, producer := range []patchobs.ProducerID{
		patchobs.ProducerRecord, patchobs.ProducerFeaturePatch, patchobs.ProducerReconcileAccept,
		patchobs.ProducerCycle, patchobs.ProducerApplyDone, patchobs.ProducerImplement, patchobs.ProducerEdit,
	} {
		_, err := observePatchProducer(producer, s, slug, "", string(captureModeWorkingTreeAll), "", "", nil, nil, true)
		if (err == nil) != (producer == patchobs.ProducerFeaturePatch) {
			t.Fatalf("checkpoint producer=%s err=%v", producer, err)
		}
	}
	rgaS0AssertNoCLICoverageArtifact(t, root, slug)
}

func TestRGAS4ReconcileAcceptDoesNotReportFalseCompletion(t *testing.T) {
	slug := "s4-accept-error"
	root := rgaS4CLIFixture(t, slug, false)
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	head, err := gitutil.HeadCommit(root)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "a.txt")
	if err := os.WriteFile(target, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch, err := gitutil.CapturePatch(root)
	if err != nil || patch == "" {
		t.Fatalf("capture fixture: %q %v", patch, err)
	}
	if err := s.WriteArtifact(slug, "post-apply.patch", patch); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	shadow, err := gitutil.CreateShadow(root, slug, head)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gitutil.PruneShadow(root, slug) })
	if err := os.WriteFile(filepath.Join(shadow, "a.txt"), []byte("resolved\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatal(err)
	}
	state.State = store.StateReconcilingShadow
	state.Reconcile.ShadowPath = shadow
	state.Reconcile.UpstreamCommit = head
	state.Reconcile.ResolveSession = "s4-accept"
	if err := s.SaveFeatureStatus(state); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact(slug, "resolution-session.json", `{"outcomes":[{"path":"a.txt","status":"resolved"}]}`); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(featureDirPath(s, slug), "artifacts", "recipe-coverage.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	err = runReconcileAccept(cmd, s, slug)
	if !errors.Is(err, workflow.ErrCoveragePublication) {
		t.Fatalf("CLI lost publication failure: %v", err)
	}
	if strings.Contains(out.String(), "Accepted ") || strings.Contains(out.String(), "state → applied") ||
		strings.Contains(stderr.String(), "warning:") {
		t.Fatalf("failed acceptance reported success/warning: out=%q err=%q", out.String(), stderr.String())
	}
	after, err := s.LoadFeatureStatus(slug)
	if err != nil || after.State != store.StateReconcilingShadow || after.Reconcile.ShadowPath != shadow {
		t.Fatalf("failed CLI acceptance lost recovery state: %+v %v", after, err)
	}
	if _, err := os.Stat(shadow); err != nil {
		t.Fatalf("failed CLI acceptance pruned its shadow: %v", err)
	}
}

type rgaS4OfflineTransport func(*http.Request) (*http.Response, error)

func (f rgaS4OfflineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRGAS4AutoAcceptPublicationFailureReachesCLIAndJSON(t *testing.T) {
	for _, format := range []string{"text", "json"} {
		t.Run(format, func(t *testing.T) {
			slug := "s4-auto-accept"
			root := rgaS4CLIFixture(t, slug, false)
			for _, change := range []struct{ body, message string }{
				{"a\nb\nc\n", "shared base"},
				{"a\nB-local\nc\n", "feature applied"},
				{"a\nB-upstream\nc\n", "upstream diverged"},
			} {
				modesWriteFile(t, root, "shared.txt", change.body)
				gitRun(t, root, "add", "shared.txt")
				if change.message == "feature applied" {
					patch := gitOut(t, root, "diff", "--cached", "HEAD") + "\n"
					s, err := store.Open(root)
					if err != nil {
						t.Fatal(err)
					}
					if err := s.WriteArtifact(slug, "post-apply.patch", patch); err != nil {
						t.Fatal(err)
					}
				}
				gitRun(t, root, "commit", "-qm", change.message)
			}
			s, err := store.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = gitutil.PruneShadow(root, slug) })
			if err := s.MarkFeatureState(slug, store.StateApplied, "apply", "fixture"); err != nil {
				t.Fatal(err)
			}
			cfg, err := s.LoadConfig()
			if err != nil {
				t.Fatal(err)
			}
			cfg.Provider.Type = "openai-compatible"
			cfg.Provider.BaseURL = "https://rga-s4.invalid"
			cfg.Provider.Model = "s4"
			cfg.Provider.AuthEnv = "TPATCH_S4_OFFLINE_AUTH"
			cfg.TestCommand = ""
			if err := s.SaveConfig(cfg); err != nil {
				t.Fatal(err)
			}
			t.Setenv("TPATCH_S4_OFFLINE_AUTH", "fixture")
			calls := 0
			oldTransport := http.DefaultTransport
			http.DefaultTransport = rgaS4OfflineTransport(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host != "rga-s4.invalid" || req.URL.Path != "/v1/chat/completions" || req.Method != http.MethodPost {
					return nil, fmt.Errorf("unexpected offline fixture request: %s %s", req.Method, req.URL)
				}
				defer req.Body.Close()
				var input struct {
					Messages []struct {
						Content string `json:"content"`
					} `json:"messages"`
				}
				if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
					return nil, err
				}
				content := `{"verdict":"unclear"}`
				for _, message := range input.Messages {
					if strings.Contains(message.Content, "# File: shared.txt") {
						content = "a\nB-merged\nc\n"
					}
				}
				calls++
				body, err := json.Marshal(map[string]any{
					"choices": []any{map[string]any{"message": map[string]string{"content": content}}},
				})
				if err != nil {
					return nil, err
				}
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
					Body: io.NopCloser(bytes.NewReader(body)), Request: req}, nil
			})
			t.Cleanup(func() { http.DefaultTransport = oldTransport })
			if err := os.Mkdir(filepath.Join(featureDirPath(s, slug), "artifacts", "recipe-coverage.json"), 0o755); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			cmd := buildRootCmd()
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"reconcile", slug, "--path", root, "--upstream-ref", "HEAD", "--resolve", "--apply", "--allow-dirty", "--format", format})
			err = cmd.Execute()
			if !errors.Is(err, workflow.ErrCoveragePublication) || exitCodeFor(err) == 0 {
				t.Fatalf("%s route swallowed publication failure: err=%v out=%q stderr=%q", format, err, stdout.String(), stderr.String())
			}
			if calls < 2 {
				t.Fatalf("fixture did not reach provider-assisted auto-accept: calls=%d", calls)
			}
			if strings.Contains(stdout.String(), "Reconciled ") || strings.Contains(stdout.String(), `"outcome": "reapplied"`) {
				t.Fatalf("%s route reported false completion: %q", format, stdout.String())
			}
			state, stateErr := s.LoadFeatureStatus(slug)
			if stateErr != nil || state.State != store.StateBlocked || state.Reconcile.ShadowPath == "" {
				t.Fatalf("%s route lost blocked/shadow recovery: %+v %v", format, state, stateErr)
			}
			if _, err := os.Stat(state.Reconcile.ShadowPath); err != nil {
				t.Fatalf("%s route pruned the failed accept's shadow: %v", format, err)
			}
		})
	}
}
