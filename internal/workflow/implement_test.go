package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesserabox/tesserapatch/internal/provider"
	"github.com/tesserabox/tesserapatch/internal/store"
)

// alwaysInvalidProvider returns the same un-parseable response every call,
// forcing GenerateWithRetry to exhaust its budget and surface an error.
type alwaysInvalidProvider struct{ calls int }

func (p *alwaysInvalidProvider) Check(ctx context.Context, cfg provider.Config) (*provider.Health, error) {
	return &provider.Health{}, nil
}

func (p *alwaysInvalidProvider) Generate(ctx context.Context, cfg provider.Config, req provider.GenerateRequest) (string, error) {
	p.calls++
	return "this is not json at all", nil
}

func TestRunImplement_FallbackEmitsWarning(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "demo", Request: "demo request"}); err != nil {
		t.Fatal(err)
	}

	// Capture warnings.
	var buf bytes.Buffer
	prev := WarnWriter
	WarnWriter = &buf
	defer func() { WarnWriter = prev }()

	prov := &alwaysInvalidProvider{}
	cfg := provider.Config{Type: "openai-compatible", BaseURL: "http://x", Model: "m", AuthEnv: "TPATCH_TEST_KEY"}
	t.Setenv("TPATCH_TEST_KEY", "stub")

	if err := RunImplement(context.Background(), s, "demo", prov, cfg); err != nil {
		t.Fatalf("RunImplement returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "implement LLM call failed") {
		t.Fatalf("expected fallback warning on stderr, got %q", out)
	}
	if !strings.Contains(out, "raw-implement-response") {
		t.Fatalf("expected pointer to raw response artifacts, got %q", out)
	}

	// And the heuristic recipe should have been written.
	body, err := os.ReadFile(filepath.Join(s.TpatchDir(), "features", "demo", "artifacts", "apply-recipe.json"))
	if err != nil {
		t.Fatalf("apply-recipe.json missing: %v", err)
	}
	var recipe ApplyRecipe
	if err := json.Unmarshal(body, &recipe); err != nil {
		t.Fatalf("recipe not valid JSON: %v", err)
	}
	if len(recipe.Operations) != 1 || recipe.Operations[0].Type != "ensure-directory" {
		t.Fatalf("expected heuristic recipe, got %+v", recipe.Operations)
	}

	// Regression for bug-cycle-state-mismatch: even on heuristic
	// fallback the feature state must advance to `implementing` so
	// status.json and last_command agree.
	st, err := s.LoadFeatureStatus("demo")
	if err != nil {
		t.Fatalf("LoadFeatureStatus: %v", err)
	}
	if st.State != store.StateImplementing {
		t.Fatalf("state after RunImplement: want %q, got %q (last_command=%q)",
			store.StateImplementing, st.State, st.LastCommand)
	}
	if st.LastCommand != "implement" {
		t.Fatalf("last_command after RunImplement: want %q, got %q",
			"implement", st.LastCommand)
	}
}

func TestConfig_DefaultMaxTokensImplement(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxTokensImplement != store.DefaultMaxTokensImplement {
		t.Fatalf("default max_tokens_implement: want %d, got %d",
			store.DefaultMaxTokensImplement, cfg.MaxTokensImplement)
	}
}
