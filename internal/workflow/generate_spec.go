package workflow

import (
	"context"
	"fmt"

	"github.com/tesseracode/tesserapatch/internal/provider"
)

// DefineInput contains only in-memory specification context.
type DefineInput struct {
	Slug         string
	Request      string
	AnalysisMD   string
	AnalysisJSON string
	Provider     provider.Provider
	Config       provider.Config
	MaxRetries   int
	Authority    GeneratorAuthorityPolicy
}

// GenerateSpec generates specification markdown without filesystem access.
func GenerateSpec(ctx context.Context, in DefineInput) (string, GenNote, error) {
	maxAttempts := retryMaxAttempts(ctx, in.MaxRetries)
	fallbackAllowed, validPolicy := generationFallbackAllowed(in.Authority)
	if !validPolicy {
		note := GenNote{Generator: GeneratorNone, ErrorClass: GenErrorInvalidPolicy, MaxAttempts: maxAttempts}
		return "", note, generationError(note)
	}

	if !generationConfigured(in.Provider, in.Config) {
		note := heuristicNote(in.Authority, GenAdvisoryProviderNotConfigured, maxAttempts)
		if !fallbackAllowed {
			note.Generator = GeneratorNone
			note.ErrorClass = GenErrorProviderRequired
			return "", note, generationError(note)
		}
		return heuristicDefine(in.Slug), note, nil
	}

	counted := &countingProvider{Provider: in.Provider}
	response, err := generateWithRetryForAuthority(ctx, counted, in.Config, provider.GenerateRequest{
		SystemPrompt: "You are a senior software engineer. Generate acceptance criteria and an implementation plan for the following feature request. Output as markdown with ## Acceptance Criteria (numbered list) and ## Implementation Plan sections.",
		UserPrompt:   fmt.Sprintf("# Feature Request\n\n%s\n\n# Analysis\n\n%s\n%s", in.Request, in.AnalysisMD, in.AnalysisJSON),
		MaxTokens:    4096,
	}, RetryOptions{
		MaxRetries: in.MaxRetries,
		Validate:   NonEmptyValidator(),
		LogPrefix:  "define",
	}, in.Authority)
	if err != nil {
		note := generationFailureNote(ctx, err, counted.lastErr, counted.attempts, maxAttempts, !in.Authority.LegacyContextSemantics)
		if !fallbackAllowed {
			return "", note, generationError(note)
		}
		note.Generator = GeneratorHeuristic
		note.Advisories = []GenAdvisory{generationFailureAdvisory(note)}
		if in.Authority.Authority == GeneratorAuthorityRegenerate {
			note.Advisories = append(note.Advisories, GenAdvisoryRegenerateHeuristicAllowed)
		}
		return heuristicDefine(in.Slug), note, nil
	}

	return fmt.Sprintf("# Specification: %s\n\n%s\n", in.Slug, response), GenNote{
		Generator:   GeneratorProvider,
		Attempts:    counted.attempts,
		MaxAttempts: maxAttempts,
	}, nil
}

func heuristicDefine(slug string) string {
	return fmt.Sprintf(`# Specification: %s

## Acceptance Criteria

1. Feature works as described in the request
2. All existing tests continue to pass
3. No regressions introduced
4. Changes are documented

## Implementation Plan

1. Review the feature request and analysis
2. Identify affected files and code areas
3. Implement the changes
4. Run tests and verify acceptance criteria
5. Record the changes

*Generated in heuristic mode — connect a provider for detailed analysis.*
`, slug)
}
