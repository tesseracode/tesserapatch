package workflow

import (
	"context"
	"errors"

	"github.com/tesseracode/tesserapatch/internal/provider"
)

// GeneratorKind identifies the authority that produced a canonical artifact.
type GeneratorKind string

const (
	GeneratorNone      GeneratorKind = "none"
	GeneratorProvider  GeneratorKind = "provider"
	GeneratorHeuristic GeneratorKind = "heuristic"
)

// GenAdvisory is a closed, non-sensitive generation advisory.
type GenAdvisory string

const (
	GenAdvisoryProviderNotConfigured      GenAdvisory = "provider-not-configured"
	GenAdvisoryProviderFallbackHeuristic  GenAdvisory = "provider-fallback-heuristic"
	GenAdvisoryProviderDeadlineHeuristic  GenAdvisory = "provider-deadline-heuristic"
	GenAdvisoryRegenerateHeuristicAllowed GenAdvisory = "regenerate-heuristic-allowed"
)

// GenErrorClass is a closed classification that never contains provider text.
type GenErrorClass string

const (
	GenErrorNone             GenErrorClass = ""
	GenErrorProviderRequired GenErrorClass = "provider-required"
	GenErrorProviderFailure  GenErrorClass = "provider-failure"
	GenErrorValidation       GenErrorClass = "validation-failure"
	GenErrorDeadline         GenErrorClass = "deadline-exceeded"
	GenErrorCanceled         GenErrorClass = "context-canceled"
	GenErrorInvalidPolicy    GenErrorClass = "invalid-authority-policy"
)

// GenDeadlineClass records context termination without retaining error text.
type GenDeadlineClass string

const (
	GenDeadlineNone     GenDeadlineClass = ""
	GenDeadlineExceeded GenDeadlineClass = "deadline"
	GenDeadlineCanceled GenDeadlineClass = "canceled"
)

// GenNote contains only bounded control metadata. Canonical returned output is
// deliberately not duplicated here.
type GenNote struct {
	Generator     GeneratorKind
	Advisories    []GenAdvisory
	ErrorClass    GenErrorClass
	DeadlineClass GenDeadlineClass
	Attempts      int
	MaxAttempts   int
}

// GeneratorAuthority selects the default fallback policy or provider-required
// regeneration policy.
type GeneratorAuthority string

const (
	GeneratorAuthorityDefault    GeneratorAuthority = ""
	GeneratorAuthorityRegenerate GeneratorAuthority = "regenerate"
)

// GeneratorAuthorityPolicy is explicit so prepare can require provider
// authority without changing the behavior of existing phase commands.
type GeneratorAuthorityPolicy struct {
	Authority              GeneratorAuthority
	AllowHeuristic         bool
	LegacyContextSemantics bool
}

// GenerationError exposes only closed classifications.
type GenerationError struct {
	Class         GenErrorClass
	DeadlineClass GenDeadlineClass
	Attempts      int
}

func (e *GenerationError) Error() string {
	return "generation failed: " + string(e.Class)
}

type countingProvider struct {
	provider.Provider
	attempts int
	lastErr  error
}

func (p *countingProvider) Generate(ctx context.Context, cfg provider.Config, req provider.GenerateRequest) (string, error) {
	p.attempts++
	response, err := p.Provider.Generate(ctx, cfg, req)
	p.lastErr = err
	return response, err
}

func generationConfigured(prov provider.Provider, cfg provider.Config) bool {
	return prov != nil && cfg.Configured()
}

func generationFallbackAllowed(policy GeneratorAuthorityPolicy) (bool, bool) {
	switch policy.Authority {
	case GeneratorAuthorityDefault:
		return true, true
	case GeneratorAuthorityRegenerate:
		return policy.AllowHeuristic, true
	default:
		return false, false
	}
}

func generationFailureNote(ctx context.Context, _ error, providerErr error, attempts, maxAttempts int, enforceContext bool) GenNote {
	note := GenNote{
		Generator:   GeneratorNone,
		ErrorClass:  GenErrorValidation,
		Attempts:    attempts,
		MaxAttempts: maxAttempts,
	}
	if providerErr != nil {
		note.ErrorClass = GenErrorProviderFailure
	}
	if !enforceContext {
		return note
	}

	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded) ||
		errors.Is(context.Cause(ctx), context.DeadlineExceeded):
		note.ErrorClass = GenErrorDeadline
		note.DeadlineClass = GenDeadlineExceeded
	case errors.Is(ctx.Err(), context.Canceled) ||
		errors.Is(context.Cause(ctx), context.Canceled):
		note.ErrorClass = GenErrorCanceled
		note.DeadlineClass = GenDeadlineCanceled
	}
	return note
}

func generationFailureAdvisory(note GenNote) GenAdvisory {
	if note.DeadlineClass == GenDeadlineExceeded {
		return GenAdvisoryProviderDeadlineHeuristic
	}
	return GenAdvisoryProviderFallbackHeuristic
}

func heuristicNote(policy GeneratorAuthorityPolicy, advisory GenAdvisory, maxAttempts int) GenNote {
	advisories := []GenAdvisory{advisory}
	if policy.Authority == GeneratorAuthorityRegenerate && policy.AllowHeuristic {
		advisories = append(advisories, GenAdvisoryRegenerateHeuristicAllowed)
	}
	return GenNote{
		Generator:   GeneratorHeuristic,
		Advisories:  advisories,
		MaxAttempts: maxAttempts,
	}
}

func generationError(note GenNote) error {
	return &GenerationError{
		Class:         note.ErrorClass,
		DeadlineClass: note.DeadlineClass,
		Attempts:      note.Attempts,
	}
}
