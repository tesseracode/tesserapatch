package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// AnalysisResult is the structured output of the analysis phase.
type AnalysisResult struct {
	Summary             string              `json:"summary"`
	Compatibility       CompatibilityResult `json:"compatibility"`
	AffectedAreas       []string            `json:"affected_areas"`
	AcceptanceCriteria  []string            `json:"acceptance_criteria"`
	ImplementationNotes []string            `json:"implementation_notes"`
	UnresolvedQuestions []string            `json:"unresolved_questions"`
	HeuristicMode       bool                `json:"heuristic_mode"`
}

// CompatibilityResult describes compatibility assessment.
type CompatibilityResult struct {
	Status    store.CompatibilityStatus `json:"status"`
	Reasoning string                    `json:"reasoning"`
}

// AnalysisInput contains only in-memory generation context.
type AnalysisInput struct {
	Slug       string
	Request    string
	FileTree   string
	Guidance   string
	Provider   provider.Provider
	Config     provider.Config
	MaxRetries int
	Authority  GeneratorAuthorityPolicy
}

// GenerateAnalysis generates analysis without reading or writing the filesystem.
func GenerateAnalysis(ctx context.Context, in AnalysisInput) (AnalysisResult, GenNote, error) {
	maxAttempts := retryMaxAttempts(ctx, in.MaxRetries)
	fallbackAllowed, validPolicy := generationFallbackAllowed(in.Authority)
	if !validPolicy {
		note := GenNote{
			Generator:   GeneratorNone,
			ErrorClass:  GenErrorInvalidPolicy,
			MaxAttempts: maxAttempts,
		}
		return AnalysisResult{}, note, generationError(note)
	}

	if !generationConfigured(in.Provider, in.Config) {
		note := heuristicNote(in.Authority, GenAdvisoryProviderNotConfigured, maxAttempts)
		if !fallbackAllowed {
			note.Generator = GeneratorNone
			note.ErrorClass = GenErrorProviderRequired
			return AnalysisResult{}, note, generationError(note)
		}
		return heuristicAnalysis(in.Slug), note, nil
	}

	var validated AnalysisResult
	counted := &countingProvider{Provider: in.Provider}
	response, err := generateWithRetryForAuthority(ctx, counted, in.Config, provider.GenerateRequest{
		SystemPrompt: buildAnalysisSystemPrompt(),
		UserPrompt:   buildAnalysisUserPrompt(in.Request, in.FileTree, in.Guidance),
		MaxTokens:    4096,
		Temperature:  0.1,
	}, RetryOptions{
		MaxRetries: in.MaxRetries,
		Validate:   JSONObjectValidator(&validated),
		LogPrefix:  "analyze",
	}, in.Authority)
	if err != nil {
		note := generationFailureNote(ctx, err, counted.lastErr, counted.attempts, maxAttempts, !in.Authority.LegacyContextSemantics)
		if !fallbackAllowed {
			return AnalysisResult{}, note, generationError(note)
		}
		note.Generator = GeneratorHeuristic
		note.Advisories = []GenAdvisory{generationFailureAdvisory(note)}
		if in.Authority.Authority == GeneratorAuthorityRegenerate {
			note.Advisories = append(note.Advisories, GenAdvisoryRegenerateHeuristicAllowed)
		}
		return heuristicAnalysis(in.Slug), note, nil
	}

	result, err := parseAnalysisResponse(response)
	if err != nil {
		note := GenNote{
			Generator:   GeneratorNone,
			ErrorClass:  GenErrorValidation,
			Attempts:    maxAttempts,
			MaxAttempts: maxAttempts,
		}
		if !fallbackAllowed {
			return AnalysisResult{}, note, generationError(note)
		}
		note.Generator = GeneratorHeuristic
		note.Advisories = []GenAdvisory{GenAdvisoryProviderFallbackHeuristic}
		if in.Authority.Authority == GeneratorAuthorityRegenerate {
			note.Advisories = append(note.Advisories, GenAdvisoryRegenerateHeuristicAllowed)
		}
		return heuristicAnalysis(in.Slug), note, nil
	}
	return result, GenNote{
		Generator:   GeneratorProvider,
		Attempts:    counted.attempts,
		MaxAttempts: maxAttempts,
	}, nil
}

func heuristicAnalysis(slug string) AnalysisResult {
	return AnalysisResult{
		Summary:       fmt.Sprintf("Heuristic analysis for feature '%s'. Manual review recommended.", slug),
		HeuristicMode: true,
		Compatibility: CompatibilityResult{
			Status:    store.CompatibilityUnknown,
			Reasoning: "Heuristic mode — no LLM available for compatibility assessment",
		},
		AffectedAreas:       []string{"(manual identification needed)"},
		AcceptanceCriteria:  []string{"Feature works as described in request", "Existing tests pass", "No regressions introduced", "Changes documented"},
		ImplementationNotes: []string{"Analysis generated in heuristic mode — connect a provider for detailed analysis"},
		UnresolvedQuestions: []string{"Detailed compatibility assessment pending provider connection"},
	}
}

func buildAnalysisSystemPrompt() string {
	return `You are a senior software engineer analyzing a feature request for a forked open-source project.

Analyze the request and produce a JSON response with these fields:
{
  "summary": "one-paragraph summary of the feature",
  "compatibility": {
    "status": "compatible|conflict|unclear",
    "reasoning": "why this assessment"
  },
  "affected_areas": ["list of code areas/files affected"],
  "acceptance_criteria": ["testable criteria that must be met"],
  "implementation_notes": ["key implementation considerations"],
  "unresolved_questions": ["questions that need answers"]
}

Be specific about file paths when you can identify them from the file tree. Output ONLY valid JSON.`
}

func buildAnalysisUserPrompt(request, fileTree, guidance string) string {
	var b strings.Builder
	b.WriteString("# Feature Request\n\n")
	b.WriteString(request)
	b.WriteString("\n\n# Project File Tree\n\n```\n")
	b.WriteString(fileTree)
	b.WriteString("```\n")
	if guidance != "" {
		b.WriteString("\n# Project Guidance Files\n\n")
		b.WriteString(guidance)
	}
	return b.String()
}

func parseAnalysisResponse(response string) (AnalysisResult, error) {
	cleaned, _ := ExtractJSONObject(response)
	cleaned = strings.TrimSpace(cleaned)

	var result AnalysisResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return AnalysisResult{}, fmt.Errorf("cannot parse analysis response as JSON: %w", err)
	}
	return result, nil
}

// RenderAnalysisMD deterministically renders an analysis artifact.
func RenderAnalysisMD(result AnalysisResult, slug string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Analysis: %s\n\n", slug))

	if result.HeuristicMode {
		b.WriteString("*Generated in heuristic mode (no provider). Connect a provider for detailed analysis.*\n\n")
	}

	b.WriteString("## Summary\n\n")
	b.WriteString(result.Summary + "\n\n")

	b.WriteString("## Compatibility\n\n")
	b.WriteString(fmt.Sprintf("**Status**: %s\n\n", result.Compatibility.Status))
	if result.Compatibility.Reasoning != "" {
		b.WriteString(result.Compatibility.Reasoning + "\n\n")
	}

	if len(result.AffectedAreas) > 0 {
		b.WriteString("## Affected Areas\n\n")
		for _, area := range result.AffectedAreas {
			b.WriteString("- " + area + "\n")
		}
		b.WriteString("\n")
	}

	if len(result.AcceptanceCriteria) > 0 {
		b.WriteString("## Acceptance Criteria\n\n")
		for i, criterion := range result.AcceptanceCriteria {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, criterion))
		}
		b.WriteString("\n")
	}

	if len(result.ImplementationNotes) > 0 {
		b.WriteString("## Implementation Notes\n\n")
		for _, note := range result.ImplementationNotes {
			b.WriteString("- " + note + "\n")
		}
		b.WriteString("\n")
	}

	if len(result.UnresolvedQuestions) > 0 {
		b.WriteString("## Unresolved Questions\n\n")
		for _, question := range result.UnresolvedQuestions {
			b.WriteString("- " + question + "\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}
