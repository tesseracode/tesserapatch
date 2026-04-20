package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ManualArtifact describes the artifact a given phase expects on disk when
// the user advances feature state manually (--manual / --skip-llm).
type ManualArtifact struct {
	// Phase is the canonical phase name (analyze|define|explore|implement).
	Phase string
	// Path is the feature-relative path to the artifact.
	Path string
	// State is the feature state the phase advances to.
	State FeatureState
	// ValidateJSON causes the helper to parse the artifact as JSON and reject
	// syntax errors before advancing state. Currently only set for implement.
	ValidateJSON bool
}

// manualPhaseMap is the single source of truth for --manual behaviour.
// Keep in sync with internal/workflow/*.go provider-driven phases.
var manualPhaseMap = map[string]ManualArtifact{
	"analyze":   {Phase: "analyze", Path: "analysis.md", State: StateAnalyzed},
	"define":    {Phase: "define", Path: "spec.md", State: StateDefined},
	"explore":   {Phase: "explore", Path: "exploration.md", State: StateDefined},
	"implement": {Phase: "implement", Path: filepath.Join("artifacts", "apply-recipe.json"), State: StateImplementing, ValidateJSON: true},
}

// ManualPhase returns the manual-advance contract for a phase, or false if
// the phase does not support --manual.
func ManualPhase(phase string) (ManualArtifact, bool) {
	m, ok := manualPhaseMap[phase]
	return m, ok
}

// AdvanceStateManually validates that the expected artifact for a phase
// exists under the feature directory and advances feature state WITHOUT
// invoking the provider. It records the manual transition in the feature's
// notes field so downstream tools and humans can see the artifact was
// authored by an agent rather than the LLM provider.
//
// Errors:
//   - phase not recognised (only analyze/define/explore/implement supported)
//   - artifact file does not exist at the expected path
//   - for implement, artifact is not valid JSON
func (s *Store) AdvanceStateManually(slug, phase string) error {
	m, ok := ManualPhase(phase)
	if !ok {
		return fmt.Errorf("--manual is not supported for phase %q", phase)
	}
	fullPath := filepath.Join(s.featureDir(slug), m.Path)
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("expected artifact not found: %s\n\nAuthor the artifact first, then re-run with --manual. See docs/agent-as-provider.md for the schema.", fullPath)
		}
		return fmt.Errorf("stat %s: %w", fullPath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("expected artifact is a directory, not a file: %s", fullPath)
	}
	if m.ValidateJSON {
		data, rerr := os.ReadFile(fullPath)
		if rerr != nil {
			return fmt.Errorf("read %s: %w", fullPath, rerr)
		}
		if len(strings.TrimSpace(string(data))) == 0 {
			return fmt.Errorf("artifact is empty: %s", fullPath)
		}
		if !json.Valid(data) {
			return fmt.Errorf("artifact is not valid JSON: %s\n\nFix the JSON syntax and re-run with --manual.", fullPath)
		}
	}
	notes := fmt.Sprintf("Phase advanced manually (--manual); artifact authored at %s", m.Path)
	return s.MarkFeatureState(slug, m.State, phase, notes)
}
