package workflow

import (
	"context"
	"fmt"

	"github.com/tesseracode/tesserapatch/internal/provider"
)

// ExploreInput contains only in-memory exploration context.
type ExploreInput struct {
	Slug       string
	Request    string
	AnalysisMD string
	SpecMD     string
	FileTree   string
	Provider   provider.Provider
	Config     provider.Config
	MaxRetries int
	Authority  GeneratorAuthorityPolicy
}

// GenerateExploration generates exploration markdown without filesystem access.
func GenerateExploration(ctx context.Context, in ExploreInput) (string, GenNote, error) {
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
		return heuristicExplore(in.Slug, in.FileTree), note, nil
	}

	counted := &countingProvider{Provider: in.Provider}
	response, err := generateWithRetryForAuthority(ctx, counted, in.Config, provider.GenerateRequest{
		SystemPrompt: "You are a senior software engineer exploring a codebase. Identify the specific files and code sections relevant to implementing the requested feature. Output as markdown with ## Relevant Files (list with paths and descriptions) and ## Minimal Changeset (what needs to change).",
		UserPrompt:   fmt.Sprintf("# Feature\n%s\n\n# Analysis\n%s\n\n# Spec\n%s\n\n# File Tree\n```\n%s\n```", in.Request, in.AnalysisMD, in.SpecMD, in.FileTree),
		MaxTokens:    4096,
	}, RetryOptions{
		MaxRetries: in.MaxRetries,
		Validate:   NonEmptyValidator(),
		LogPrefix:  "explore",
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
		return heuristicExplore(in.Slug, in.FileTree), note, nil
	}

	return fmt.Sprintf("# Exploration: %s\n\n%s\n", in.Slug, response), GenNote{
		Generator:   GeneratorProvider,
		Attempts:    counted.attempts,
		MaxAttempts: maxAttempts,
	}, nil
}

func heuristicExplore(slug, fileTree string) string {
	return fmt.Sprintf(`# Exploration: %s

## File Tree

`+"```"+`
%s
`+"```"+`

## Relevant Files

*(Manual identification needed — connect a provider for automated exploration.)*

## Minimal Changeset

*(Pending detailed analysis.)*
`, slug, fileTree)
}
