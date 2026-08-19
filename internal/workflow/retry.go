package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// spinnerMessage maps an internal LogPrefix (analyze/define/explore/
// implement/...) to the user-facing phase label printed alongside the
// braille spinner. Any prefix not in the table falls back to a
// Title-cased "<prefix>..." string.
func spinnerMessage(prefix string) string {
	switch prefix {
	case "analyze":
		return "Analyzing..."
	case "define":
		return "Defining..."
	case "explore":
		return "Exploring..."
	case "implement":
		return "Implementing..."
	case "":
		return "Generating..."
	default:
		if len(prefix) == 0 {
			return "Generating..."
		}
		return strings.ToUpper(prefix[:1]) + prefix[1:] + "..."
	}
}

// Validator inspects a raw LLM response and returns an error if it cannot be
// used as-is. The error message is fed back into the retry prompt.
type Validator func(string) error

// contextKey is a private type for workflow context values.
type contextKey int

const (
	ctxKeyDisableRetry contextKey = iota
)

// WithDisableRetry returns a context that forces MaxRetries=0 for any
// GenerateWithRetry call down-stream. Used by the `--no-retry` CLI flag.
func WithDisableRetry(ctx context.Context, disable bool) context.Context {
	return context.WithValue(ctx, ctxKeyDisableRetry, disable)
}

func retryDisabled(ctx context.Context) bool {
	v, _ := ctx.Value(ctxKeyDisableRetry).(bool)
	return v
}

// RetryOptions controls retry behavior for GenerateWithRetry.
type RetryOptions struct {
	MaxRetries int       // 0 disables retry
	Validate   Validator // may be nil
	LogPrefix  string    // artifact filename prefix (e.g. "analyze")
	Slug       string    // feature slug, for artifact writes
	Store      *store.Store
}

type retryIOPolicy struct {
	startAttempt             func() func()
	recordSuccessfulResponse func(int, string)
	enforceContext           bool
}

// GenerateWithRetry calls prov.Generate up to (1 + MaxRetries) times. It
// preserves the legacy spinner and optional raw-response artifact logging.
// Pure generators must use GenerateWithRetryInMemory instead.
//
// Returns the first response that passes validation. If every attempt fails,
// returns the last response together with the final validator error so the
// caller can decide whether to fall back to heuristics.
func GenerateWithRetry(ctx context.Context, prov provider.Provider, cfg provider.Config, req provider.GenerateRequest, opts RetryOptions) (string, error) {
	policy := retryIOPolicy{
		startAttempt: func() func() {
			spinner := NewSpinnerIfTTY(os.Stderr, spinnerMessage(opts.LogPrefix))
			return spinner.Stop
		},
		enforceContext: false,
	}
	if opts.Store != nil && opts.Slug != "" {
		policy.recordSuccessfulResponse = func(attempt int, response string) {
			name := fmt.Sprintf("raw-%s-response-%d.txt", opts.safePrefix(), attempt)
			_ = opts.Store.WriteArtifact(opts.Slug, name, response)
		}
	}
	return generateWithRetry(ctx, prov, cfg, req, opts, policy)
}

// GenerateWithRetryInMemory applies retry and validation without spinner,
// filesystem, Store, or global stderr access. Store and Slug are cleared even
// when a caller accidentally supplies them.
func GenerateWithRetryInMemory(ctx context.Context, prov provider.Provider, cfg provider.Config, req provider.GenerateRequest, opts RetryOptions) (string, error) {
	return generateWithRetryInMemory(ctx, prov, cfg, req, opts, true)
}

// generateWithRetryInMemoryLegacyContext preserves the legacy Run* context
// semantics while retaining the pure in-memory retry policy.
func generateWithRetryInMemoryLegacyContext(ctx context.Context, prov provider.Provider, cfg provider.Config, req provider.GenerateRequest, opts RetryOptions) (string, error) {
	return generateWithRetryInMemory(ctx, prov, cfg, req, opts, false)
}

func generateWithRetryInMemory(ctx context.Context, prov provider.Provider, cfg provider.Config, req provider.GenerateRequest, opts RetryOptions, enforceContext bool) (string, error) {
	opts.Store = nil
	opts.Slug = ""
	return generateWithRetry(ctx, prov, cfg, req, opts, retryIOPolicy{enforceContext: enforceContext})
}

func generateWithRetryForAuthority(ctx context.Context, prov provider.Provider, cfg provider.Config, req provider.GenerateRequest, opts RetryOptions, authority GeneratorAuthorityPolicy) (string, error) {
	if authority.LegacyContextSemantics {
		return generateWithRetryInMemoryLegacyContext(ctx, prov, cfg, req, opts)
	}
	return GenerateWithRetryInMemory(ctx, prov, cfg, req, opts)
}

func generateWithRetry(ctx context.Context, prov provider.Provider, cfg provider.Config, req provider.GenerateRequest, opts RetryOptions, ioPolicy retryIOPolicy) (string, error) {
	if prov == nil || !cfg.Configured() {
		return "", fmt.Errorf("provider not configured")
	}
	if ioPolicy.enforceContext {
		if err := retryContextError(ctx); err != nil {
			return "", err
		}
	}

	attempts := retryMaxAttempts(ctx, opts.MaxRetries)

	var lastResp string
	var lastErr error
	currentReq := req

	for i := 0; i < attempts; i++ {
		if ioPolicy.enforceContext {
			if err := retryContextError(ctx); err != nil {
				return lastResp, err
			}
		}

		stopAttempt := func() {}
		if ioPolicy.startAttempt != nil {
			stopAttempt = ioPolicy.startAttempt()
		}
		resp, err := prov.Generate(ctx, cfg, currentReq)
		stopAttempt()

		if err == nil && ioPolicy.recordSuccessfulResponse != nil {
			ioPolicy.recordSuccessfulResponse(i+1, resp)
		}
		if ioPolicy.enforceContext {
			if contextErr := retryContextError(ctx); contextErr != nil {
				return resp, contextErr
			}
		}
		if err != nil {
			// Transport / provider-level error: don't retry with corrective prompt,
			// surface it immediately.
			return resp, err
		}
		lastResp = resp

		if opts.Validate == nil {
			return resp, nil
		}
		if err := opts.Validate(resp); err == nil {
			return resp, nil
		} else {
			lastErr = err
		}

		// Prepare corrective follow-up for next attempt.
		if i < attempts-1 {
			currentReq = req
			currentReq.UserPrompt = fmt.Sprintf(
				"%s\n\n---\n\nYour previous response was invalid: %s\n\nPlease output ONLY the response in the exact format requested. Do not include explanations, prose, or markdown fences.",
				req.UserPrompt, lastErr.Error(),
			)
		}
	}

	return lastResp, fmt.Errorf("validation failed after %d attempt(s): %w", attempts, lastErr)
}

func retryContextError(ctx context.Context) error {
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}
	return ctx.Err()
}

func retryMaxAttempts(ctx context.Context, maxRetries int) int {
	attempts := maxRetries + 1
	if retryDisabled(ctx) {
		attempts = 1
	}
	if attempts < 1 {
		return 1
	}
	return attempts
}

func (o RetryOptions) safePrefix() string {
	if o.LogPrefix == "" {
		return "generate"
	}
	return o.LogPrefix
}

// JSONObjectValidator returns a Validator that tries to parse the response
// (stripping markdown fences AND trailing prose) into the provided target
// type. Used for analyze/implement phases where strict JSON is required.
// Delegates span detection to ExtractJSONObject so prose after the closing
// brace (a common LLM mistake) is tolerated instead of triggering a retry.
func JSONObjectValidator(target any) Validator {
	return func(resp string) error {
		cleaned, err := ExtractJSONObject(resp)
		if err != nil {
			return fmt.Errorf("could not locate JSON object in response: %v", err)
		}
		cleaned = strings.TrimSpace(cleaned)
		if cleaned == "" {
			return fmt.Errorf("empty response")
		}
		if err := json.Unmarshal([]byte(cleaned), target); err != nil {
			return fmt.Errorf("response is not valid JSON: %v", err)
		}
		return nil
	}
}

// NonEmptyValidator accepts any response containing non-whitespace text.
func NonEmptyValidator() Validator {
	return func(resp string) error {
		if strings.TrimSpace(resp) == "" {
			return fmt.Errorf("empty response")
		}
		return nil
	}
}

// (stripJSONFences was removed after bug-extract-json-robustness — all
// call sites now go through ExtractJSONObject, which subsumes fence
// stripping and brace-balanced span detection.)
