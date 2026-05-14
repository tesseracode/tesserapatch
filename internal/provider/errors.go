package provider

import (
	"errors"
	"fmt"
	"strings"
)

// ProxyUpstreamAbortedError signals that the local copilot-api proxy
// returned a 500 with the body "This operation was aborted" — i.e.
// the upstream Copilot fetch inside the proxy was cancelled before a
// usable response arrived.
//
// Two known triggers in the current proxy version:
//
//  1. A Claude model routed to /v1/chat/completions resolves to
//     /v1/messages internally, but `routes/chat-completions/handler.ts`
//     is missing the dispatch branch for it and falls through to the
//     OpenAI-format upstream call, which upstream Copilot aborts.
//     PickProvider transparently dodges this by selecting the Anthropic
//     provider (hits /v1/messages directly) when the model advertises
//     /v1/messages support.
//
//  2. The experimental ResponsesProvider is enabled against a proxy
//     build that advertises /responses but does not reliably serve it.
//     Pending an upstream proxy fix, keep the responses provider gate
//     off so GPT-5.x models fall back to /v1/chat/completions.
//
// CLI callers may use IsProxyUpstreamAborted to detect this case and
// print a richer remediation prompt; the multi-line Error() string is
// safe to surface verbatim too.
type ProxyUpstreamAbortedError struct {
	Model    string
	Endpoint string
	Body     string
}

// Error returns a multi-line message that is safe to print directly:
// callers don't need to print extra hints — the hint is embedded.
func (e *ProxyUpstreamAbortedError) Error() string {
	return fmt.Sprintf(
		"provider proxy aborted upstream call (model=%q, endpoint=%s)\n"+
			"\n"+
			"  The local copilot-api proxy received the request but its\n"+
			"  upstream call to Copilot was cancelled before a response\n"+
			"  arrived. Common causes:\n"+
			"    • Claude model routed to /v1/chat/completions on a proxy\n"+
			"      version that's missing the /v1/messages dispatch branch.\n"+
			"      Workaround: `tpatch provider set --type anthropic ...`,\n"+
			"      or upgrade the local copilot-api proxy.\n"+
			"    • experimental /responses routing is enabled against a\n"+
			"      proxy build that advertises /responses but does not\n"+
			"      reliably serve it. Unset TPATCH_ENABLE_RESPONSES_PROVIDER\n"+
			"      so GPT-5.x models use /v1/chat/completions.",
		e.Model, e.Endpoint,
	)
}

// IsProxyUpstreamAborted reports whether err wraps a
// ProxyUpstreamAbortedError. Workflow / CLI code can use this to
// short-circuit retries (the cause is configuration, not transient).
func IsProxyUpstreamAborted(err error) bool {
	var pe *ProxyUpstreamAbortedError
	return errors.As(err, &pe)
}

// detectProxyAbort inspects a non-2xx response from the copilot-api
// proxy and returns a typed ProxyUpstreamAbortedError when it matches
// the abort signature. Returns nil for any non-matching combination.
func detectProxyAbort(cfg Config, endpoint string, status int, body string) error {
	if status != 500 || !IsCopilotProxyEndpoint(cfg) {
		return nil
	}
	if !strings.Contains(body, "This operation was aborted") {
		return nil
	}
	return &ProxyUpstreamAbortedError{
		Model:    cfg.Model,
		Endpoint: endpoint,
		Body:     body,
	}
}
