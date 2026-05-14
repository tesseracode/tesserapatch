package provider_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	provider "github.com/tesseracode/tesserapatch/internal/provider"
)

// TestModelEndpointDetection verifies that /v1/models responses with
// supported_endpoints are correctly parsed and used for routing.
func TestModelEndpointDetection(t *testing.T) {
	mockProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{
				"data": [
					{"id": "claude-sonnet-4.6", "supported_endpoints": ["/v1/messages"]},
					{"id": "gpt-5.5", "supported_endpoints": ["/responses"]},
					{"id": "gpt-4o", "supported_endpoints": ["/chat/completions"]}
				]
			}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockProxy.Close()

	cfg := provider.Config{
		Type:    "openai-compatible",
		BaseURL: mockProxy.URL,
		Model:   "gpt-5.5",
	}

	p := provider.New()
	health, err := p.Check(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if health == nil || len(health.ModelInfo) == 0 {
		t.Fatal("ModelInfo not populated from /v1/models")
	}

	// Verify all models were parsed with endpoints
	for _, m := range health.ModelInfo {
		if len(m.SupportedEndpoints) == 0 {
			t.Errorf("model %s has no supported_endpoints", m.ID)
		}
	}

	// Check specific model mapping
	found := false
	for _, m := range health.ModelInfo {
		if m.ID == "gpt-5.5" {
			found = true
			if len(m.SupportedEndpoints) != 1 || m.SupportedEndpoints[0] != "/responses" {
				t.Errorf("gpt-5.5 should support /responses, got %v", m.SupportedEndpoints)
			}
		}
	}
	if !found {
		t.Error("gpt-5.5 not found in parsed models")
	}
}

// TestPickProviderClaudeOnCopilotProxy verifies Claude models
// are routed to AnthropicProvider when /v1/messages is available.
func TestPickProviderClaudeOnCopilotProxy(t *testing.T) {
	cfg := provider.Config{
		Type:    "openai-compatible",
		BaseURL: "http://localhost:4141",
		Model:   "claude-sonnet-4.6",
	}
	health := &provider.Health{
		Endpoint: "http://localhost:4141",
		Models: []string{
			"claude-sonnet-4.6",
			"gpt-4o",
		},
		ModelInfo: []provider.ModelInfo{
			{ID: "claude-sonnet-4.6", SupportedEndpoints: []string{"/v1/messages", "/chat/completions"}},
			{ID: "gpt-4o", SupportedEndpoints: []string{"/chat/completions"}},
		},
	}

	got := provider.PickProvider(cfg, health)
	if _, ok := got.(*provider.AnthropicProvider); !ok {
		t.Fatalf("PickProvider for Claude on copilot proxy = %T, want *AnthropicProvider", got)
	}
}

// TestPickProviderGPT5DefaultFallsThrough verifies the production default:
// GPT-5.x models that advertise /responses still use OpenAICompatible unless
// the experimental ResponsesProvider gate is explicitly enabled.
func TestPickProviderGPT5DefaultFallsThrough(t *testing.T) {
	t.Setenv("TPATCH_ENABLE_RESPONSES_PROVIDER", "")

	cfg := provider.Config{
		Type:    "openai-compatible",
		BaseURL: "http://localhost:4141",
		Model:   "gpt-5.5",
	}
	health := &provider.Health{
		Endpoint: "http://localhost:4141",
		Models: []string{
			"gpt-5.5",
			"gpt-4o",
		},
		ModelInfo: []provider.ModelInfo{
			{ID: "gpt-5.5", SupportedEndpoints: []string{"/responses"}},
			{ID: "gpt-4o", SupportedEndpoints: []string{"/chat/completions"}},
		},
	}

	got := provider.PickProvider(cfg, health)
	if _, ok := got.(*provider.OpenAICompatible); !ok {
		t.Fatalf("PickProvider for gpt-5.5 with /responses endpoint = %T, want *OpenAICompatible", got)
	}
}

// TestPickProviderGPT5ResponsesEndpoint verifies GPT-5.x models
// with /responses endpoint trigger ResponsesProvider when enabled.
func TestPickProviderGPT5ResponsesEndpoint(t *testing.T) {
	t.Setenv("TPATCH_ENABLE_RESPONSES_PROVIDER", "1")

	cfg := provider.Config{
		Type:    "openai-compatible",
		BaseURL: "http://localhost:4141",
		Model:   "gpt-5.5",
	}
	health := &provider.Health{
		Endpoint: "http://localhost:4141",
		Models: []string{
			"gpt-5.5",
			"gpt-4o",
		},
		ModelInfo: []provider.ModelInfo{
			{ID: "gpt-5.5", SupportedEndpoints: []string{"/responses"}},
			{ID: "gpt-4o", SupportedEndpoints: []string{"/chat/completions"}},
		},
	}

	got := provider.PickProvider(cfg, health)
	if _, ok := got.(*provider.ResponsesProvider); !ok {
		t.Fatalf("PickProvider for gpt-5.5 with /responses endpoint = %T, want *ResponsesProvider", got)
	}
}

// TestPickProviderGPT4FallsThrough verifies standard GPT models
// without /v1/messages use OpenAICompatible for /chat/completions.
func TestPickProviderGPT4FallsThrough(t *testing.T) {
	cfg := provider.Config{
		Type:    "openai-compatible",
		BaseURL: "http://localhost:4141",
		Model:   "gpt-4o",
	}
	health := &provider.Health{
		Endpoint: "http://localhost:4141",
		Models:   []string{"gpt-4o"},
		ModelInfo: []provider.ModelInfo{
			{ID: "gpt-4o", SupportedEndpoints: []string{"/chat/completions"}},
		},
	}

	got := provider.PickProvider(cfg, health)
	if _, ok := got.(*provider.OpenAICompatible); !ok {
		t.Fatalf("PickProvider for gpt-4o = %T, want *OpenAICompatible", got)
	}
}

// TestPickProviderMultiEndpointPriority verifies endpoint priority:
// /v1/messages > /responses > /chat/completions
func TestPickProviderMultiEndpointPriority(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		endpoints    []string
		expectedType string
	}{
		{
			name:         "messages preferred over responses",
			model:        "multi1",
			endpoints:    []string{"/v1/messages", "/responses", "/chat/completions"},
			expectedType: "AnthropicProvider",
		},
		{
			name:         "responses preferred over completions when gate enabled",
			model:        "multi2",
			endpoints:    []string{"/responses", "/chat/completions"},
			expectedType: "ResponsesProvider",
		},
		{
			name:         "only completions available",
			model:        "multi3",
			endpoints:    []string{"/chat/completions"},
			expectedType: "OpenAICompatible",
		},
	}

	t.Setenv("TPATCH_ENABLE_RESPONSES_PROVIDER", "1")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := provider.Config{
				Type:    "openai-compatible",
				BaseURL: "http://localhost:4141",
				Model:   tt.model,
			}
			health := &provider.Health{
				Endpoint: "http://localhost:4141",
				Models:   []string{tt.model},
				ModelInfo: []provider.ModelInfo{
					{ID: tt.model, SupportedEndpoints: tt.endpoints},
				},
			}

			got := provider.PickProvider(cfg, health)
			gotType := providerTypeName(got)
			if gotType != tt.expectedType {
				t.Errorf("expected %s provider, got %s", tt.expectedType, gotType)
			}
		})
	}
}

// TestLiveLocalProxy runs against localhost:4141 (if available)
// and validates the actual model routing matrix.
func TestLiveLocalProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live proxy test in short mode")
	}
	if os.Getenv("TPATCH_LIVE_PROVIDER") != "1" {
		t.Skip("set TPATCH_LIVE_PROVIDER=1 to hit localhost:4141")
	}

	// Try to connect to localhost:4141
	testURL := "http://localhost:4141"
	p := provider.New()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := provider.Config{
		Type:    "openai-compatible",
		BaseURL: testURL,
		Model:   "claude-sonnet-4.6",
	}
	health, err := p.Check(ctx, cfg)
	if err != nil {
		t.Skipf("localhost:4141 not reachable: %v", err)
	}

	t.Logf("Live proxy endpoint: %s", health.Endpoint)
	t.Logf("Available models: %d", len(health.Models))

	// Verify ModelInfo was populated
	if len(health.ModelInfo) == 0 {
		t.Error("ModelInfo is empty — proxy may not be returning supported_endpoints")
	}

	// Print routing matrix for manual inspection
	t.Log("\n=== Model Routing Matrix ===")
	for _, m := range health.ModelInfo {
		endpoints := strings.Join(m.SupportedEndpoints, ", ")
		if endpoints == "" {
			endpoints = "(unknown)"
		}
		t.Logf("%-20s → %s", m.ID, endpoints)

		cfg.Model = m.ID
		chosen := provider.PickProvider(cfg, health)
		t.Logf("  → Picked provider: %s\n", providerTypeName(chosen))
	}

	smokeModels := []string{
		"claude-sonnet-4.6",
		"gpt-5.5",
		"gpt-5.4",
		"gpt-5-mini",
		"gemini-2.5-pro",
		"gpt-4.1",
		"gpt-4o",
	}
	for _, model := range smokeModels {
		if !hasModel(health.Models, model) {
			t.Logf("skipping %s: not present in live catalog", model)
			continue
		}
		t.Run("generate/"+model, func(t *testing.T) {
			cfg.Model = model
			chosen := provider.PickProvider(cfg, health)
			callCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			out, err := chosen.Generate(callCtx, cfg, provider.GenerateRequest{
				SystemPrompt: "You are terse.",
				UserPrompt:   "Reply exactly: TPATCH_OK",
				MaxTokens:    512,
				Temperature:  0.1,
			})
			if err != nil {
				t.Fatalf("Generate(%s) via %s failed: %v", model, providerTypeName(chosen), err)
			}
			if strings.TrimSpace(out) == "" {
				t.Fatalf("Generate(%s) via %s returned empty text", model, providerTypeName(chosen))
			}
			t.Logf("%s via %s => %q", model, providerTypeName(chosen), out)
		})
	}
}

// Helper: get provider type name for logging
func providerTypeName(p provider.Provider) string {
	switch p.(type) {
	case *provider.OpenAICompatible:
		return "OpenAICompatible"
	case *provider.AnthropicProvider:
		return "AnthropicProvider"
	case *provider.ResponsesProvider:
		return "ResponsesProvider"
	default:
		return fmt.Sprintf("unknown(%T)", p)
	}
}

func hasModel(models []string, want string) bool {
	for _, model := range models {
		if model == want {
			return true
		}
	}
	return false
}
