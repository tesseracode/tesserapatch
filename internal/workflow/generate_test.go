package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

type scriptedGeneratorProvider struct {
	responses []string
	errs      []error
	calls     int
	prompts   []provider.GenerateRequest
	ignoreCtx bool
	onCall    func(int)
}

func (p *scriptedGeneratorProvider) Check(context.Context, provider.Config) (*provider.Health, error) {
	return &provider.Health{}, nil
}

func (p *scriptedGeneratorProvider) Generate(ctx context.Context, _ provider.Config, req provider.GenerateRequest) (string, error) {
	p.prompts = append(p.prompts, req)
	index := p.calls
	p.calls++
	if p.onCall != nil {
		p.onCall(index)
	}
	if !p.ignoreCtx {
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}
	var response string
	if index < len(p.responses) {
		response = p.responses[index]
	}
	if index < len(p.errs) {
		return response, p.errs[index]
	}
	return response, nil
}

func configuredGeneratorConfig() provider.Config {
	return provider.Config{Type: "test", BaseURL: "memory://provider", Model: "test-model"}
}

func validAnalysisResponse(summary string) string {
	return fmt.Sprintf(`{
  "summary": %q,
  "compatibility": {"status": "compatible", "reasoning": "safe"},
  "affected_areas": ["internal/workflow"],
  "acceptance_criteria": ["works"],
  "implementation_notes": ["small"],
  "unresolved_questions": []
}`, summary)
}

func TestPureGeneratorsProviderSuccess(t *testing.T) {
	t.Run("analysis", func(t *testing.T) {
		prov := &scriptedGeneratorProvider{responses: []string{validAnalysisResponse("provider summary")}}
		result, note, err := GenerateAnalysis(context.Background(), AnalysisInput{
			Slug: "demo", Request: "request", FileTree: "a.go\n", Guidance: "guide",
			Provider: prov, Config: configuredGeneratorConfig(), MaxRetries: 2,
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.Summary != "provider summary" || result.HeuristicMode {
			t.Fatalf("unexpected result: %+v", result)
		}
		assertProviderNote(t, note, 1, 3)
		if len(prov.prompts) != 1 ||
			!strings.Contains(prov.prompts[0].UserPrompt, "request") ||
			!strings.Contains(prov.prompts[0].UserPrompt, "a.go") ||
			!strings.Contains(prov.prompts[0].UserPrompt, "guide") {
			t.Fatalf("analysis context not forwarded: %+v", prov.prompts)
		}
	})

	t.Run("spec", func(t *testing.T) {
		prov := &scriptedGeneratorProvider{responses: []string{"## Acceptance Criteria\n\n1. exact"}}
		got, note, err := GenerateSpec(context.Background(), DefineInput{
			Slug: "demo", Request: "request", AnalysisMD: "analysis-md", AnalysisJSON: "analysis-json",
			Provider: prov, Config: configuredGeneratorConfig(),
		})
		if err != nil {
			t.Fatal(err)
		}
		want := "# Specification: demo\n\n## Acceptance Criteria\n\n1. exact\n"
		if got != want {
			t.Fatalf("spec bytes differ\nwant: %q\n got: %q", want, got)
		}
		assertProviderNote(t, note, 1, 1)
		if prompt := prov.prompts[0].UserPrompt; !strings.Contains(prompt, "analysis-md") || !strings.Contains(prompt, "analysis-json") {
			t.Fatalf("effective analysis context missing: %q", prompt)
		}
	})

	t.Run("exploration", func(t *testing.T) {
		prov := &scriptedGeneratorProvider{responses: []string{"## Relevant Files\n\n- a.go"}}
		got, note, err := GenerateExploration(context.Background(), ExploreInput{
			Slug: "demo", Request: "request", AnalysisMD: "analysis", SpecMD: "spec", FileTree: "a.go\n",
			Provider: prov, Config: configuredGeneratorConfig(),
		})
		if err != nil {
			t.Fatal(err)
		}
		want := "# Exploration: demo\n\n## Relevant Files\n\n- a.go\n"
		if got != want {
			t.Fatalf("exploration bytes differ\nwant: %q\n got: %q", want, got)
		}
		assertProviderNote(t, note, 1, 1)
		if prompt := prov.prompts[0].UserPrompt; !strings.Contains(prompt, "analysis") || !strings.Contains(prompt, "spec") || !strings.Contains(prompt, "a.go") {
			t.Fatalf("effective exploration context missing: %q", prompt)
		}
	})
}

func TestPureGeneratorsDefaultHeuristicWithoutProvider(t *testing.T) {
	result, analysisNote, err := GenerateAnalysis(context.Background(), AnalysisInput{Slug: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.HeuristicMode {
		t.Fatal("analysis did not use heuristic mode")
	}
	assertHeuristicNote(t, analysisNote, GenAdvisoryProviderNotConfigured)

	spec, specNote, err := GenerateSpec(context.Background(), DefineInput{
		Slug:   "demo",
		Config: provider.Config{BaseURL: "configured-without-model"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec != heuristicDefine("demo") {
		t.Fatalf("unexpected heuristic spec: %q", spec)
	}
	assertHeuristicNote(t, specNote, GenAdvisoryProviderNotConfigured)

	exploration, explorationNote, err := GenerateExploration(context.Background(), ExploreInput{
		Slug: "demo", FileTree: "a.go\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if exploration != heuristicExplore("demo", "a.go\n") {
		t.Fatalf("unexpected heuristic exploration: %q", exploration)
	}
	assertHeuristicNote(t, explorationNote, GenAdvisoryProviderNotConfigured)
}

func TestPureGeneratorTransportAndValidationFallback(t *testing.T) {
	t.Run("transport", func(t *testing.T) {
		prov := &scriptedGeneratorProvider{
			responses: []string{"SECRET_RESPONSE"},
			errs:      []error{errors.New("SECRET_TRANSPORT_ERROR")},
		}
		got, note, err := GenerateSpec(context.Background(), DefineInput{
			Slug: "demo", Provider: prov, Config: configuredGeneratorConfig(), MaxRetries: 4,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got != heuristicDefine("demo") {
			t.Fatalf("transport failure did not fall back: %q", got)
		}
		assertHeuristicNote(t, note, GenAdvisoryProviderFallbackHeuristic)
		if note.ErrorClass != GenErrorProviderFailure || note.Attempts != 1 || note.MaxAttempts != 5 {
			t.Fatalf("unexpected transport note: %+v", note)
		}
		assertNoSensitiveText(t, note, err, "SECRET_RESPONSE", "SECRET_TRANSPORT_ERROR")
	})

	t.Run("validation-retries", func(t *testing.T) {
		prov := &scriptedGeneratorProvider{responses: []string{"", " ", "## Acceptance Criteria\n\n1. valid"}}
		got, note, err := GenerateSpec(context.Background(), DefineInput{
			Slug: "demo", Provider: prov, Config: configuredGeneratorConfig(), MaxRetries: 2,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got != "# Specification: demo\n\n## Acceptance Criteria\n\n1. valid\n" {
			t.Fatalf("unexpected retried spec: %q", got)
		}
		assertProviderNote(t, note, 3, 3)
		if !strings.Contains(prov.prompts[1].UserPrompt, "previous response was invalid") {
			t.Fatalf("corrective retry prompt missing: %q", prov.prompts[1].UserPrompt)
		}
	})

	t.Run("validation-exhausted", func(t *testing.T) {
		prov := &scriptedGeneratorProvider{responses: []string{"", ""}}
		got, note, err := GenerateSpec(context.Background(), DefineInput{
			Slug: "demo", Provider: prov, Config: configuredGeneratorConfig(), MaxRetries: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got != heuristicDefine("demo") {
			t.Fatalf("validation exhaustion did not fall back: %q", got)
		}
		if note.ErrorClass != GenErrorValidation || note.Attempts != 2 {
			t.Fatalf("unexpected validation note: %+v", note)
		}
	})
}

func TestPureGeneratorNoRetryContext(t *testing.T) {
	prov := &scriptedGeneratorProvider{responses: []string{"", "valid but must not be reached"}}
	ctx := WithDisableRetry(context.Background(), true)
	got, note, err := GenerateSpec(ctx, DefineInput{
		Slug: "demo", Provider: prov, Config: configuredGeneratorConfig(), MaxRetries: 9,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != heuristicDefine("demo") || prov.calls != 1 {
		t.Fatalf("--no-retry mismatch: calls=%d output=%q", prov.calls, got)
	}
	if note.Attempts != 1 || note.MaxAttempts != 1 || note.ErrorClass != GenErrorValidation {
		t.Fatalf("unexpected no-retry note: %+v", note)
	}
}

func TestPureGeneratorRegenerateAuthority(t *testing.T) {
	required := GeneratorAuthorityPolicy{Authority: GeneratorAuthorityRegenerate}

	t.Run("no-provider-refuses-without-call-or-output", func(t *testing.T) {
		prov := &scriptedGeneratorProvider{}
		got, note, err := GenerateSpec(context.Background(), DefineInput{
			Slug: "demo", Provider: prov, Config: provider.Config{}, Authority: required,
		})
		if err == nil || got != "" || prov.calls != 0 {
			t.Fatalf("required-provider refusal: output=%q calls=%d err=%v", got, prov.calls, err)
		}
		if note.Generator != GeneratorNone || note.ErrorClass != GenErrorProviderRequired {
			t.Fatalf("unexpected required-provider note: %+v", note)
		}
		assertNoSensitiveText(t, note, err, "request", "provider-body")
	})

	t.Run("provider-failure-refuses-without-partial-output", func(t *testing.T) {
		prov := &scriptedGeneratorProvider{
			responses: []string{"SECRET_PARTIAL"},
			errs:      []error{errors.New("SECRET_PROVIDER_FAILURE")},
		}
		got, note, err := GenerateExploration(context.Background(), ExploreInput{
			Slug: "demo", Provider: prov, Config: configuredGeneratorConfig(), Authority: required,
		})
		if err == nil || got != "" {
			t.Fatalf("regenerate published partial output: output=%q err=%v", got, err)
		}
		if note.ErrorClass != GenErrorProviderFailure || note.Attempts != 1 {
			t.Fatalf("unexpected refusal note: %+v", note)
		}
		assertNoSensitiveText(t, note, err, "SECRET_PARTIAL", "SECRET_PROVIDER_FAILURE")
	})

	t.Run("validation-failure-refuses-without-partial-output", func(t *testing.T) {
		prov := &scriptedGeneratorProvider{responses: []string{"", ""}}
		got, note, err := GenerateSpec(context.Background(), DefineInput{
			Slug: "demo", Provider: prov, Config: configuredGeneratorConfig(),
			MaxRetries: 1, Authority: required,
		})
		if err == nil || got != "" || note.ErrorClass != GenErrorValidation || note.Attempts != 2 {
			t.Fatalf("validation refusal: output=%q note=%+v err=%v", got, note, err)
		}
	})

	t.Run("explicit-heuristic-opt-in", func(t *testing.T) {
		prov := &scriptedGeneratorProvider{errs: []error{errors.New("unreachable")}}
		got, note, err := GenerateSpec(context.Background(), DefineInput{
			Slug: "demo", Provider: prov, Config: configuredGeneratorConfig(),
			Authority: GeneratorAuthorityPolicy{Authority: GeneratorAuthorityRegenerate, AllowHeuristic: true},
		})
		if err != nil || got != heuristicDefine("demo") {
			t.Fatalf("allowed heuristic failed: output=%q err=%v", got, err)
		}
		want := []GenAdvisory{
			GenAdvisoryProviderFallbackHeuristic,
			GenAdvisoryRegenerateHeuristicAllowed,
		}
		if !reflect.DeepEqual(note.Advisories, want) {
			t.Fatalf("advisories: want %v, got %v", want, note.Advisories)
		}
	})

	t.Run("explicit-heuristic-opt-in-without-provider", func(t *testing.T) {
		got, note, err := GenerateSpec(context.Background(), DefineInput{
			Slug: "demo",
			Authority: GeneratorAuthorityPolicy{
				Authority:      GeneratorAuthorityRegenerate,
				AllowHeuristic: true,
			},
		})
		if err != nil || got != heuristicDefine("demo") {
			t.Fatalf("allowed no-provider heuristic: output=%q err=%v", got, err)
		}
		want := []GenAdvisory{
			GenAdvisoryProviderNotConfigured,
			GenAdvisoryRegenerateHeuristicAllowed,
		}
		if !reflect.DeepEqual(note.Advisories, want) {
			t.Fatalf("advisories: want %v, got %v", want, note.Advisories)
		}
	})
}

func TestPureGeneratorDeadlineAndCancellationClassification(t *testing.T) {
	t.Run("pre-expired-deadline-makes-zero-calls", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
		defer cancel()
		prov := &scriptedGeneratorProvider{responses: []string{"must-not-publish"}}
		got, note, err := GenerateSpec(ctx, DefineInput{
			Slug: "demo", Provider: prov, Config: configuredGeneratorConfig(),
		})
		if err != nil || got != heuristicDefine("demo") || prov.calls != 0 {
			t.Fatalf("deadline fallback: output=%q calls=%d err=%v", got, prov.calls, err)
		}
		if note.ErrorClass != GenErrorDeadline || note.DeadlineClass != GenDeadlineExceeded ||
			note.Attempts != 0 || !reflect.DeepEqual(note.Advisories, []GenAdvisory{GenAdvisoryProviderDeadlineHeuristic}) {
			t.Fatalf("unexpected deadline note: %+v", note)
		}
	})

	t.Run("pre-canceled-makes-zero-calls", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		prov := &scriptedGeneratorProvider{responses: []string{"must-not-publish"}}
		got, note, err := GenerateSpec(ctx, DefineInput{
			Slug: "demo", Provider: prov, Config: configuredGeneratorConfig(),
		})
		if err != nil || got != heuristicDefine("demo") || prov.calls != 0 {
			t.Fatalf("cancellation fallback: output=%q calls=%d err=%v", got, prov.calls, err)
		}
		if note.ErrorClass != GenErrorCanceled || note.DeadlineClass != GenDeadlineCanceled ||
			note.Attempts != 0 || !reflect.DeepEqual(note.Advisories, []GenAdvisory{GenAdvisoryProviderFallbackHeuristic}) {
			t.Fatalf("unexpected cancellation note: %+v", note)
		}
	})

	t.Run("context-ignoring-success-after-deadline-is-rejected", func(t *testing.T) {
		ctx, expire := context.WithCancelCause(context.Background())
		prov := &scriptedGeneratorProvider{
			responses: []string{"must-not-publish"},
			ignoreCtx: true,
			onCall: func(int) {
				expire(context.DeadlineExceeded)
			},
		}
		got, note, err := GenerateSpec(ctx, DefineInput{
			Slug: "demo", Provider: prov, Config: configuredGeneratorConfig(),
		})
		if err != nil || got != heuristicDefine("demo") || prov.calls != 1 {
			t.Fatalf("post-call deadline fallback: output=%q calls=%d err=%v", got, prov.calls, err)
		}
		if note.ErrorClass != GenErrorDeadline || note.DeadlineClass != GenDeadlineExceeded ||
			note.Generator != GeneratorHeuristic {
			t.Fatalf("expired provider success was accepted: %+v", note)
		}
	})

	t.Run("prepare-style-default-strict-policy-rejects-canceled-success", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		prov := &scriptedGeneratorProvider{
			responses: []string{"must-not-publish"},
			ignoreCtx: true,
			onCall: func(int) {
				cancel()
			},
		}
		got, note, err := GenerateSpec(ctx, DefineInput{
			Slug: "demo", Provider: prov, Config: configuredGeneratorConfig(),
			Authority: GeneratorAuthorityPolicy{Authority: GeneratorAuthorityRegenerate},
		})
		if err == nil || got != "" || prov.calls != 1 {
			t.Fatalf("strict prepare-style generation accepted canceled success: output=%q calls=%d err=%v", got, prov.calls, err)
		}
		if note.ErrorClass != GenErrorCanceled || note.DeadlineClass != GenDeadlineCanceled ||
			note.Generator != GeneratorNone {
			t.Fatalf("unexpected strict cancellation note: %+v", note)
		}
	})

	t.Run("live-context-provider-timeout-is-provider-failure", func(t *testing.T) {
		prov := &scriptedGeneratorProvider{errs: []error{context.DeadlineExceeded}}
		got, note, err := GenerateSpec(context.Background(), DefineInput{
			Slug: "demo", Provider: prov, Config: configuredGeneratorConfig(),
		})
		if err != nil || got != heuristicDefine("demo") {
			t.Fatalf("provider timeout fallback: output=%q err=%v", got, err)
		}
		if note.ErrorClass != GenErrorProviderFailure || note.DeadlineClass != GenDeadlineNone ||
			!reflect.DeepEqual(note.Advisories, []GenAdvisory{GenAdvisoryProviderFallbackHeuristic}) {
			t.Fatalf("provider timeout misclassified: %+v", note)
		}
	})

	t.Run("canceled-between-retries-stops-next-call", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		prov := &scriptedGeneratorProvider{responses: []string{"", "must-not-be-called"}}
		validator := func(string) error {
			cancel()
			return errors.New("invalid")
		}
		_, err := GenerateWithRetryInMemory(ctx, prov, configuredGeneratorConfig(),
			provider.GenerateRequest{UserPrompt: "request"},
			RetryOptions{MaxRetries: 1, Validate: validator})
		if !errors.Is(err, context.Canceled) || prov.calls != 1 {
			t.Fatalf("retry ignored cancellation: calls=%d err=%v", prov.calls, err)
		}
	})

	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer cancel()
	got, note, err := GenerateSpec(ctx, DefineInput{
		Slug: "demo", Provider: &scriptedGeneratorProvider{}, Config: configuredGeneratorConfig(),
		Authority: GeneratorAuthorityPolicy{Authority: GeneratorAuthorityRegenerate},
	})
	if err == nil || got != "" || note.ErrorClass != GenErrorDeadline || note.DeadlineClass != GenDeadlineExceeded {
		t.Fatalf("required deadline refusal: output=%q note=%+v err=%v", got, note, err)
	}
}

func TestPureGeneratorDoesNotPersistProviderOrPromptText(t *testing.T) {
	dir := t.TempDir()
	markerPath := filepath.Join(dir, "SECRET_SOURCE_MARKER")
	responseMarker := "SECRET_PROVIDER_RESPONSE"
	promptMarker := "SECRET_PROMPT_SOURCE"
	prov := &scriptedGeneratorProvider{responses: []string{validAnalysisResponse(responseMarker)}}

	result, note, err := GenerateAnalysis(context.Background(), AnalysisInput{
		Slug: markerPath, Request: promptMarker, FileTree: markerPath, Guidance: promptMarker,
		Provider: prov, Config: configuredGeneratorConfig(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary != responseMarker {
		t.Fatalf("canonical provider output was not returned: %+v", result)
	}
	assertNoSensitiveText(t, note, err, responseMarker, promptMarker, markerPath)

	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("pure generator wrote filesystem entries: %v", entries)
	}
}

func TestGenerateWithRetryInMemoryForcesQuietNoSinkPolicy(t *testing.T) {
	type retryRoute func(context.Context, provider.Provider, provider.Config, provider.GenerateRequest, RetryOptions) (string, error)
	routes := []struct {
		name string
		run  retryRoute
	}{
		{name: "strict", run: GenerateWithRetryInMemory},
		{name: "legacy-context", run: generateWithRetryInMemoryLegacyContext},
	}

	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			s := newGeneratorStore(t, 0)
			prov := &scriptedGeneratorProvider{responses: []string{"not-json", `{"ok":true}`}}
			var target map[string]any

			readStderr, writeStderr, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			originalStderr := os.Stderr
			os.Stderr = writeStderr
			defer func() {
				os.Stderr = originalStderr
				_ = readStderr.Close()
			}()

			response, retryErr := route.run(
				context.Background(),
				prov,
				configuredGeneratorConfig(),
				provider.GenerateRequest{UserPrompt: "request"},
				RetryOptions{
					MaxRetries: 1,
					Validate:   JSONObjectValidator(&target),
					LogPrefix:  "future-json",
					Slug:       "demo",
					Store:      s,
				},
			)
			if closeErr := writeStderr.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			stderr, readErr := io.ReadAll(readStderr)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if retryErr != nil || response != `{"ok":true}` {
				t.Fatalf("in-memory retry failed: response=%q err=%v", response, retryErr)
			}
			if len(stderr) != 0 {
				t.Fatalf("in-memory retry wrote global stderr: %q", stderr)
			}
			assertArtifactAbsent(t, s, "raw-future-json-response-1.txt")
			assertArtifactAbsent(t, s, "raw-future-json-response-2.txt")
		})
	}
}

func TestRenderAnalysisMDDeterministic(t *testing.T) {
	result := heuristicAnalysis("demo")
	first := RenderAnalysisMD(result, "demo")
	for i := 0; i < 10; i++ {
		if got := RenderAnalysisMD(result, "demo"); got != first {
			t.Fatalf("render changed at iteration %d", i)
		}
	}
	if !strings.HasSuffix(first, "\n") {
		t.Fatalf("render lacks terminal newline: %q", first)
	}

	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := "{\n" +
		"  \"summary\": \"Heuristic analysis for feature 'demo'. Manual review recommended.\",\n" +
		"  \"compatibility\": {\n"
	if !strings.HasPrefix(string(encoded), wantPrefix) {
		t.Fatalf("analysis JSON field order changed: %s", encoded)
	}
}

func TestRunWrappersPreserveLegacyRawArtifacts(t *testing.T) {
	t.Run("provider-success", func(t *testing.T) {
		s := newGeneratorStore(t, 2)
		response := validAnalysisResponse("analysis")
		result, err := RunAnalysis(context.Background(), s, "demo",
			&scriptedGeneratorProvider{responses: []string{response}}, configuredGeneratorConfig())
		if err != nil {
			t.Fatal(err)
		}
		if result.Summary != "analysis" || result.HeuristicMode {
			t.Fatalf("unexpected analysis result: %+v", result)
		}
		assertArtifactBytes(t, s, "raw-analyze-response-1.txt", response)
		assertArtifactAbsent(t, s, "raw-analysis-response.txt")
	})

	t.Run("retry-validation", func(t *testing.T) {
		s := newGeneratorStore(t, 1)
		first := "not-json"
		second := validAnalysisResponse("retried")
		result, err := RunAnalysis(context.Background(), s, "demo",
			&scriptedGeneratorProvider{responses: []string{first, second}}, configuredGeneratorConfig())
		if err != nil {
			t.Fatal(err)
		}
		if result.Summary != "retried" || result.HeuristicMode {
			t.Fatalf("unexpected retried result: %+v", result)
		}
		assertArtifactBytes(t, s, "raw-analyze-response-1.txt", first)
		assertArtifactBytes(t, s, "raw-analyze-response-2.txt", second)
		assertArtifactAbsent(t, s, "raw-analysis-response.txt")
	})

	t.Run("transport-with-response", func(t *testing.T) {
		s := newGeneratorStore(t, 2)
		const response = "partial-provider-body"
		providerErr := errors.New("transport exploded")
		result, err := RunAnalysis(context.Background(), s, "demo", &scriptedGeneratorProvider{
			responses: []string{response},
			errs:      []error{providerErr},
		}, configuredGeneratorConfig())
		if err != nil {
			t.Fatal(err)
		}

		want := heuristicAnalysis("demo")
		want.UnresolvedQuestions = append(want.UnresolvedQuestions, "Provider error: transport exploded")
		if !reflect.DeepEqual(*result, want) {
			t.Fatalf("legacy fallback result differs\nwant: %+v\n got: %+v", want, *result)
		}
		assertArtifactAbsent(t, s, "raw-analyze-response-1.txt")
		assertArtifactBytes(t, s, "raw-analysis-response.txt", response)
		assertAnalysisArtifacts(t, s, want)
	})

	t.Run("final-validation-failure", func(t *testing.T) {
		s := newGeneratorStore(t, 1)
		prov := &scriptedGeneratorProvider{responses: []string{"first-invalid", "last-invalid"}}
		result, err := RunAnalysis(context.Background(), s, "demo", prov, configuredGeneratorConfig())
		if err != nil {
			t.Fatal(err)
		}

		want := heuristicAnalysis("demo")
		want.UnresolvedQuestions = append(want.UnresolvedQuestions,
			"Provider error: validation failed after 2 attempt(s): could not locate JSON object in response: no JSON object or array found in response")
		if !reflect.DeepEqual(*result, want) {
			t.Fatalf("legacy validation fallback differs\nwant: %+v\n got: %+v", want, *result)
		}
		assertArtifactBytes(t, s, "raw-analyze-response-1.txt", "first-invalid")
		assertArtifactBytes(t, s, "raw-analyze-response-2.txt", "last-invalid")
		assertArtifactBytes(t, s, "raw-analysis-response.txt", "last-invalid")
		assertAnalysisArtifacts(t, s, want)
	})

	t.Run("define-and-explore", func(t *testing.T) {
		s := newGeneratorStore(t, 0)
		prov := &scriptedGeneratorProvider{responses: []string{
			"## Acceptance Criteria\n\n1. spec",
			"## Relevant Files\n\n- file.go",
		}}
		if err := RunDefine(context.Background(), s, "demo", prov, configuredGeneratorConfig()); err != nil {
			t.Fatal(err)
		}
		if err := RunExplore(context.Background(), s, "demo", prov, configuredGeneratorConfig()); err != nil {
			t.Fatal(err)
		}
		assertArtifactBytes(t, s, "raw-define-response-1.txt", "## Acceptance Criteria\n\n1. spec")
		assertArtifactBytes(t, s, "raw-explore-response-1.txt", "## Relevant Files\n\n- file.go")
	})
}

func TestRunWrappersPreserveLegacyCanceledContextSuccess(t *testing.T) {
	t.Run("analysis", func(t *testing.T) {
		s := newGeneratorStore(t, 0)
		ctx, cancel := context.WithCancel(context.Background())
		response := validAnalysisResponse("canceled analysis")
		prov := &scriptedGeneratorProvider{
			responses: []string{response},
			ignoreCtx: true,
			onCall: func(int) {
				cancel()
			},
		}

		result, err := RunAnalysis(ctx, s, "demo", prov, configuredGeneratorConfig())
		if err != nil {
			t.Fatal(err)
		}
		if result.Summary != "canceled analysis" || result.HeuristicMode {
			t.Fatalf("wrapper rejected legacy provider output: %+v", result)
		}
		assertArtifactBytes(t, s, "raw-analyze-response-1.txt", response)
		assertArtifactAbsent(t, s, "raw-analysis-response.txt")
		assertAnalysisArtifacts(t, s, *result)
	})

	t.Run("define", func(t *testing.T) {
		s := newGeneratorStore(t, 0)
		ctx, cancel := context.WithCancel(context.Background())
		response := "## Acceptance Criteria\n\n1. canceled spec"
		prov := &scriptedGeneratorProvider{
			responses: []string{response},
			ignoreCtx: true,
			onCall: func(int) {
				cancel()
			},
		}

		if err := RunDefine(ctx, s, "demo", prov, configuredGeneratorConfig()); err != nil {
			t.Fatal(err)
		}
		spec, err := s.ReadFeatureFile("demo", "spec.md")
		if err != nil {
			t.Fatal(err)
		}
		if want := "# Specification: demo\n\n" + response + "\n"; spec != want {
			t.Fatalf("wrapper rejected legacy provider output\nwant: %q\n got: %q", want, spec)
		}
		assertArtifactBytes(t, s, "raw-define-response-1.txt", response)
	})

	t.Run("explore", func(t *testing.T) {
		s := newGeneratorStore(t, 0)
		ctx, cancel := context.WithCancel(context.Background())
		response := "## Relevant Files\n\n- canceled.go"
		prov := &scriptedGeneratorProvider{
			responses: []string{response},
			ignoreCtx: true,
			onCall: func(int) {
				cancel()
			},
		}

		if err := RunExplore(ctx, s, "demo", prov, configuredGeneratorConfig()); err != nil {
			t.Fatal(err)
		}
		exploration, err := s.ReadFeatureFile("demo", "exploration.md")
		if err != nil {
			t.Fatal(err)
		}
		if want := "# Exploration: demo\n\n" + response + "\n"; exploration != want {
			t.Fatalf("wrapper rejected legacy provider output\nwant: %q\n got: %q", want, exploration)
		}
		assertArtifactBytes(t, s, "raw-explore-response-1.txt", response)
	})
}

func TestGenerateWithRetryPreservesProviderErrorIdentity(t *testing.T) {
	sentinel := errors.New("sentinel provider error")
	prov := &scriptedGeneratorProvider{errs: []error{sentinel}}
	_, err := GenerateWithRetry(
		context.Background(),
		prov,
		configuredGeneratorConfig(),
		provider.GenerateRequest{UserPrompt: "request"},
		RetryOptions{MaxRetries: 3, Store: nil},
	)
	if err != sentinel {
		t.Fatalf("legacy retry error identity changed: want %p, got %p (%v)", sentinel, err, err)
	}
}

func TestRetryRoutesPreserveLegacyContextCompatibility(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(cause.Error(), func(t *testing.T) {
			legacyCtx, cancelLegacy := context.WithCancelCause(context.Background())
			legacyProvider := &scriptedGeneratorProvider{
				responses: []string{"valid response"},
				ignoreCtx: true,
				onCall: func(int) {
					cancelLegacy(cause)
				},
			}
			response, err := GenerateWithRetry(
				legacyCtx,
				legacyProvider,
				configuredGeneratorConfig(),
				provider.GenerateRequest{UserPrompt: "request"},
				RetryOptions{Validate: NonEmptyValidator()},
			)
			if err != nil || response != "valid response" || legacyProvider.calls != 1 {
				t.Fatalf("legacy route rejected context-ignoring success: response=%q calls=%d err=%v", response, legacyProvider.calls, err)
			}

			memoryCtx, cancelMemory := context.WithCancelCause(context.Background())
			memoryProvider := &scriptedGeneratorProvider{
				responses: []string{"valid response"},
				ignoreCtx: true,
				onCall: func(int) {
					cancelMemory(cause)
				},
			}
			response, err = GenerateWithRetryInMemory(
				memoryCtx,
				memoryProvider,
				configuredGeneratorConfig(),
				provider.GenerateRequest{UserPrompt: "request"},
				RetryOptions{Validate: NonEmptyValidator()},
			)
			if err != cause || response != "valid response" || memoryProvider.calls != 1 {
				t.Fatalf("in-memory route accepted post-provider context expiry: response=%q calls=%d err=%v", response, memoryProvider.calls, err)
			}

			legacyMemoryCtx, cancelLegacyMemory := context.WithCancelCause(context.Background())
			legacyMemoryProvider := &scriptedGeneratorProvider{
				responses: []string{"valid response"},
				ignoreCtx: true,
				onCall: func(int) {
					cancelLegacyMemory(cause)
				},
			}
			response, err = generateWithRetryInMemoryLegacyContext(
				legacyMemoryCtx,
				legacyMemoryProvider,
				configuredGeneratorConfig(),
				provider.GenerateRequest{UserPrompt: "request"},
				RetryOptions{Validate: NonEmptyValidator()},
			)
			if err != nil || response != "valid response" || legacyMemoryProvider.calls != 1 {
				t.Fatalf("legacy in-memory route rejected context-ignoring success: response=%q calls=%d err=%v", response, legacyMemoryProvider.calls, err)
			}
		})
	}
}

func TestLegacyInMemoryRetryDoesNotSubstituteContext(t *testing.T) {
	t.Run("pre-canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		prov := &scriptedGeneratorProvider{
			responses: []string{"valid response"},
			ignoreCtx: true,
		}
		response, err := generateWithRetryInMemoryLegacyContext(
			ctx,
			prov,
			configuredGeneratorConfig(),
			provider.GenerateRequest{UserPrompt: "request"},
			RetryOptions{Validate: NonEmptyValidator()},
		)
		if err != nil || response != "valid response" || prov.calls != 1 {
			t.Fatalf("legacy route applied a pre-call context check: response=%q calls=%d err=%v", response, prov.calls, err)
		}
	})

	t.Run("post-canceled-validation-error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		prov := &scriptedGeneratorProvider{
			responses: []string{""},
			ignoreCtx: true,
			onCall: func(int) {
				cancel()
			},
		}
		response, err := generateWithRetryInMemoryLegacyContext(
			ctx,
			prov,
			configuredGeneratorConfig(),
			provider.GenerateRequest{UserPrompt: "request"},
			RetryOptions{Validate: NonEmptyValidator()},
		)
		if response != "" || err == nil || errors.Is(err, context.Canceled) ||
			!strings.Contains(err.Error(), "validation failed after 1 attempt(s)") {
			t.Fatalf("legacy route substituted post-call context: response=%q err=%v", response, err)
		}
	})
}

func TestRetryRoutesHandleWrappedProviderTimeoutByPolicy(t *testing.T) {
	wrappedTimeout := fmt.Errorf("provider timeout: %w", context.DeadlineExceeded)

	legacyCtx, expireLegacy := context.WithCancelCause(context.Background())
	legacyProvider := &scriptedGeneratorProvider{
		errs:      []error{wrappedTimeout},
		ignoreCtx: true,
		onCall: func(int) {
			expireLegacy(context.DeadlineExceeded)
		},
	}
	_, err := GenerateWithRetry(
		legacyCtx,
		legacyProvider,
		configuredGeneratorConfig(),
		provider.GenerateRequest{UserPrompt: "request"},
		RetryOptions{},
	)
	if err != wrappedTimeout {
		t.Fatalf("legacy route changed wrapped provider timeout: want %p, got %p (%v)", wrappedTimeout, err, err)
	}

	memoryCtx, expireMemory := context.WithCancelCause(context.Background())
	memoryProvider := &scriptedGeneratorProvider{
		errs:      []error{wrappedTimeout},
		ignoreCtx: true,
		onCall: func(int) {
			expireMemory(context.DeadlineExceeded)
		},
	}
	_, err = GenerateWithRetryInMemory(
		memoryCtx,
		memoryProvider,
		configuredGeneratorConfig(),
		provider.GenerateRequest{UserPrompt: "request"},
		RetryOptions{},
	)
	if err != context.DeadlineExceeded {
		t.Fatalf("in-memory route did not enforce expired context: got %p (%v)", err, err)
	}
}

func TestGenerateWithRetryPreservesLegacyStoreLogging(t *testing.T) {
	s := newGeneratorStore(t, 0)
	prov := &scriptedGeneratorProvider{responses: []string{"invalid", `{"ok":true}`}}
	var target map[string]any
	response, err := GenerateWithRetry(
		context.Background(),
		prov,
		configuredGeneratorConfig(),
		provider.GenerateRequest{UserPrompt: "request"},
		RetryOptions{
			MaxRetries: 1,
			Validate:   JSONObjectValidator(&target),
			LogPrefix:  "legacy",
			Slug:       "demo",
			Store:      s,
		},
	)
	if err != nil || response != `{"ok":true}` {
		t.Fatalf("legacy retry failed: response=%q err=%v", response, err)
	}
	assertArtifactBytes(t, s, "raw-legacy-response-1.txt", "invalid")
	assertArtifactBytes(t, s, "raw-legacy-response-2.txt", `{"ok":true}`)
}

func newGeneratorStore(t *testing.T, maxRetries int) *store.Store {
	t.Helper()
	s, err := store.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "demo", Request: "request"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxRetries = maxRetries
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	return s
}

func assertArtifactBytes(t *testing.T, s *store.Store, name, want string) {
	t.Helper()
	got, err := s.ReadFeatureFile("demo", filepath.Join("artifacts", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if got != want {
		t.Fatalf("%s bytes differ\nwant: %q\n got: %q", name, want, got)
	}
}

func assertArtifactAbsent(t *testing.T, s *store.Store, name string) {
	t.Helper()
	if _, err := s.ReadFeatureFile("demo", filepath.Join("artifacts", name)); !os.IsNotExist(err) {
		t.Fatalf("%s unexpectedly exists (err=%v)", name, err)
	}
}

func assertAnalysisArtifacts(t *testing.T, s *store.Store, want AnalysisResult) {
	t.Helper()
	encoded, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	assertArtifactBytes(t, s, "analysis.json", string(encoded)+"\n")
	gotMD, err := s.ReadFeatureFile("demo", "analysis.md")
	if err != nil {
		t.Fatal(err)
	}
	wantMD := RenderAnalysisMD(want, "demo")
	if gotMD != wantMD {
		t.Fatalf("analysis.md bytes differ\nwant: %q\n got: %q", wantMD, gotMD)
	}
}

func assertProviderNote(t *testing.T, note GenNote, attempts, maxAttempts int) {
	t.Helper()
	if note.Generator != GeneratorProvider || len(note.Advisories) != 0 ||
		note.ErrorClass != GenErrorNone || note.DeadlineClass != GenDeadlineNone ||
		note.Attempts != attempts || note.MaxAttempts != maxAttempts {
		t.Fatalf("unexpected provider note: %+v", note)
	}
}

func assertHeuristicNote(t *testing.T, note GenNote, advisory GenAdvisory) {
	t.Helper()
	if note.Generator != GeneratorHeuristic || !reflect.DeepEqual(note.Advisories, []GenAdvisory{advisory}) {
		t.Fatalf("unexpected heuristic note: %+v", note)
	}
}

func assertNoSensitiveText(t *testing.T, note GenNote, err error, markers ...string) {
	t.Helper()
	text := fmt.Sprintf("%+v %v", note, err)
	for _, marker := range markers {
		if marker != "" && strings.Contains(text, marker) {
			t.Fatalf("sensitive marker %q leaked through note/error: %q", marker, text)
		}
	}
}
