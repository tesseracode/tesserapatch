package store_test

import (
	"github.com/tesseracode/tesserapatch/internal/store"
)

// init registers a permissive Session-ignore verifier for unit tests in
// the store package. Production wiring lives in internal/workflow, which
// registers EnsureLocalIgnoreContract for the CLI binary and every
// workflow-level test. These store-only tests never run git, so a
// pass-through keeps the bottleneck contract-visible without dragging
// in a working tree.
//
// v0.12.0 Wave γ rev-1 (F-EXT-γ-1): Store.SaveSession now REQUIRES a
// verifier per PRD-active-feature-session §4 D6 mandate 4 verbatim
// ("Writers must refuse when Git is unavailable or the path is not
// ignored.") — the bypass here mirrors what internal/workflow.init()
// installs in the production binary.
func init() {
	store.SetSessionIgnoreVerifier(func(repoRoot, resolvedPath string) error {
		return nil
	})
}
