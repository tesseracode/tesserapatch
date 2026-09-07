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

// ManualCheckpoint describes the artifact a manual advance has just
// validated, handed to the optional checkpoint hook BEFORE the feature
// state moves.
//
// It exists so a caller can act on exactly the bytes the store validated
// — `implement --manual` is a governed producer event (ADR-036 D15 P6)
// and must checkpoint the bytes it accepted, not a re-read of the file
// that could have changed in between. The type is deliberately plain
// data: the store knows nothing about who consumes it.
type ManualCheckpoint struct {
	// Slug and Phase name the advance being checkpointed.
	Slug  string
	Phase string
	// Path is the resolved path the store validated.
	Path string
	// Data is the exact validated bytes. It is non-nil only for a phase
	// whose contract reads the artifact's content (today: implement).
	Data []byte
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
	return s.AdvanceStateManuallyWithCheckpoint(slug, phase, nil)
}

// AdvanceStateManuallyWithCheckpoint is AdvanceStateManually with an
// optional hook invoked AFTER the artifact has passed every validation
// the phase requires and BEFORE the state transition.
//
// That position is the contract: a hook never sees an artifact the store
// refused, and a hook that itself fails aborts the advance, so a failed
// checkpoint leaves the feature exactly where it was. A nil hook makes
// this identical to AdvanceStateManually.
func (s *Store) AdvanceStateManuallyWithCheckpoint(slug, phase string, checkpoint func(ManualCheckpoint) error) error {
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
	var validated []byte
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
		validated = data
	}
	if checkpoint != nil {
		if cerr := checkpoint(ManualCheckpoint{Slug: slug, Phase: m.Phase, Path: fullPath, Data: validated}); cerr != nil {
			return cerr
		}
	}
	notes := fmt.Sprintf("Phase advanced manually (--manual); artifact authored at %s", m.Path)
	return s.MarkFeatureState(slug, m.State, phase, notes)
}
