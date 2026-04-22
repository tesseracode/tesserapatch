package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// WarnWriter receives non-fatal warnings emitted by workflow phases (e.g.
// when the implement phase falls back to a heuristic recipe because the
// LLM call failed validation). Defaults to os.Stderr; tests override it
// to capture output.
var WarnWriter io.Writer = os.Stderr

// ApplyRecipe is the deterministic operation format for applying changes.
type ApplyRecipe struct {
	Feature    string            `json:"feature"`
	Operations []RecipeOperation `json:"operations"`
}

// RecipeOperation is a single operation in an apply recipe.
type RecipeOperation struct {
	Type    string `json:"type"`    // write-file, replace-in-file, append-file, ensure-directory
	Path    string `json:"path"`    // target file path (relative to repo root)
	Content string `json:"content"` // for write-file, append-file
	Search  string `json:"search"`  // for replace-in-file: text to find
	Replace string `json:"replace"` // for replace-in-file: replacement text
}

// RunImplement generates a deterministic apply recipe for a feature.
func RunImplement(ctx context.Context, s *store.Store, slug string, prov provider.Provider, cfg provider.Config) error {
	request, err := s.ReadFeatureFile(slug, "request.md")
	if err != nil {
		return fmt.Errorf("cannot read feature request: %w", err)
	}

	spec, _ := s.ReadFeatureFile(slug, "spec.md")
	exploration, _ := s.ReadFeatureFile(slug, "exploration.md")

	var recipeContent string
	if prov != nil && cfg.Configured() {
		systemPrompt := `You are a senior software engineer. Generate an apply recipe (a JSON array of file operations) to implement the requested feature.

Each operation has:
- "type": one of "write-file", "replace-in-file", "append-file", "ensure-directory"
- "path": target file path relative to repo root
- "content": file content (for write-file, append-file)
- "search": text to find (for replace-in-file)
- "replace": replacement text (for replace-in-file)

Output ONLY valid JSON: {"feature": "<slug>", "operations": [...]}`

		userPrompt := fmt.Sprintf("# Feature: %s\n\n## Request\n%s\n\n## Spec\n%s\n\n## Exploration\n%s",
			slug, request, spec, exploration)

		storeCfg, _ := s.LoadConfig()
		maxTokens := storeCfg.MaxTokensImplement
		if maxTokens <= 0 {
			maxTokens = store.DefaultMaxTokensImplement
		}
		var tmp ApplyRecipe
		response, err := GenerateWithRetry(ctx, prov, cfg, provider.GenerateRequest{
			SystemPrompt: systemPrompt,
			UserPrompt:   userPrompt,
			MaxTokens:    maxTokens,
			Temperature:  0.1,
		}, RetryOptions{
			MaxRetries: storeCfg.MaxRetries,
			Validate:   JSONObjectValidator(&tmp),
			LogPrefix:  "implement",
			Slug:       slug,
			Store:      s,
		})
		if err != nil {
			fmt.Fprintf(WarnWriter,
				"warning: implement LLM call failed after %d retries (%v); "+
					"falling back to a 1-operation heuristic recipe.\n"+
					"  Inspect raw responses at .tpatch/features/%s/artifacts/raw-implement-response-*.txt\n"+
					"  Retry with a larger budget: tpatch config set max_tokens_implement 32768\n",
				storeCfg.MaxRetries, err, slug)
			recipeContent = heuristicRecipe(slug)
		} else {
			recipeContent = response
		}
	} else {
		recipeContent = heuristicRecipe(slug)
	}

	// Try to parse and re-serialize for clean formatting
	var recipe ApplyRecipe
	if err := json.Unmarshal([]byte(mustExtractJSON(recipeContent)), &recipe); err != nil {
		// Save raw content if not valid JSON
		if err := s.WriteArtifact(slug, "apply-recipe.json", recipeContent); err != nil {
			return err
		}
	} else {
		data, _ := json.MarshalIndent(recipe, "", "  ")
		if err := s.WriteArtifact(slug, "apply-recipe.json", string(data)+"\n"); err != nil {
			return err
		}
	}

	// State advances to "implementing" — the recipe is ready but the
	// code has not been executed/applied yet. The `apply` command moves
	// it the rest of the way through implementing → applied.
	return s.MarkFeatureState(slug, store.StateImplementing, "implement", "Apply recipe generated")
}

func heuristicRecipe(slug string) string {
	recipe := ApplyRecipe{
		Feature: slug,
		Operations: []RecipeOperation{
			{
				Type:    "ensure-directory",
				Path:    "src/",
				Content: "",
			},
		},
	}
	data, _ := json.MarshalIndent(recipe, "", "  ")
	return string(data)
}

func extractJSON(s string) string { return mustExtractJSON(s) }

// mustExtractJSON is a thin adapter over ExtractJSONObject that
// preserves the old "best-effort, never panic" contract of the legacy
// helper: on parse failure it still returns a non-empty string so the
// downstream json.Unmarshal path produces its own structured error
// (which is what the retry loop keys off of).
func mustExtractJSON(s string) string {
	out, err := ExtractJSONObject(s)
	if err != nil {
		return out
	}
	return out
}

func findIndex(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
