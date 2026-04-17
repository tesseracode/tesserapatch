package provider

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ProbeTimeout is the default timeout for reachability probes. Kept short
// because a probe runs before every first workflow call; any value over
// ~2s starts to feel sluggish.
const ProbeTimeout = 2 * time.Second

// Reachable performs a fast best-effort health check against cfg's endpoint.
// It returns nil when the provider's /v1/models (or Anthropic equivalent)
// responds with a 200, or a descriptive error otherwise.
//
// Call-site pattern:
//
//	if err := provider.Reachable(ctx, cfg); err != nil {
//	    // warn-and-continue on init, or hard-fail on workflow commands
//	}
//
// Honors the caller-supplied deadline but enforces an upper bound of
// ProbeTimeout so a mistakenly-passed background context cannot hang.
func Reachable(ctx context.Context, cfg Config) error {
	if !cfg.Configured() {
		return fmt.Errorf("provider is not configured")
	}
	probeCtx, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()
	prov := NewFromConfig(cfg)
	if _, err := prov.Check(probeCtx, cfg); err != nil {
		return err
	}
	return nil
}

// IsLocalEndpoint reports whether cfg points at a local address. Callers
// use this to decide whether to probe (local proxies benefit from a
// reachability check; remote endpoints we trust the user's network for).
func IsLocalEndpoint(cfg Config) bool {
	u := strings.ToLower(strings.TrimSpace(cfg.BaseURL))
	return strings.HasPrefix(u, "http://localhost:") ||
		strings.HasPrefix(u, "http://127.0.0.1:") ||
		strings.HasPrefix(u, "http://[::1]:") ||
		strings.HasPrefix(u, "https://localhost:") ||
		strings.HasPrefix(u, "https://127.0.0.1:")
}

// IsCopilotProxyEndpoint reports whether cfg looks like it points at a
// local copilot-api proxy (default port 4141). Used to scope Copilot-
// specific install hints and AUP warnings.
func IsCopilotProxyEndpoint(cfg Config) bool {
	if cfg.Type != "openai-compatible" {
		return false
	}
	u := strings.ToLower(strings.TrimSpace(cfg.BaseURL))
	return strings.Contains(u, ":4141")
}
