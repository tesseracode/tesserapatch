package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/safety"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// PreimageHashPrefix is the mandatory literal prefix on the display form
// of `preimage_hash` for existing-file writes. PRD-write-file-recipe-safety
// §3.1 + ADR-029 D1 pin the display format to `sha256:<64 lowercase hex>`;
// the raw `sha256:` prefix (not a `pg_/re_/rr_` 12-hex ID) is a deliberate
// distinction between preconditions and record identity per ADR-029 D1.
const PreimageHashPrefix = "sha256:"

// PreimageHashHexLen is the number of hex characters after the `sha256:`
// prefix — 64 chars = 256 bits.
const PreimageHashHexLen = 64

// Sentinel errors for write-file preimage/later-touch prechecks. Callers
// (execute path, tests, downstream Wave γ session gates) match with
// errors.Is. All apply-time refusal cases from ADR-029 D3 route through
// this pair — ErrWriteFilePreimageMismatch covers the hash/exists/legacy
// PRD §3.3 table refusal rows; ErrWriteFileLaterTouch covers PRD §4.2
// path-level later-touch drift (Slice 3).
var (
	// ErrWriteFilePreimageMismatch fires when a `write-file` operation's
	// `preimage_hash` precondition fails: the current target hash differs
	// from the expected hash, the expected file is missing, or the
	// empty-preimage new-file case collides with an existing file. PRD
	// §3.3 table + ADR-029 D3.
	ErrWriteFilePreimageMismatch = errors.New("write-file preimage precondition failed")

	// ErrWriteFileLaterTouch fires when a `write-file` operation targets
	// a path that has been touched by a feature recorded LATER than the
	// operation's owning feature (PRD-write-file-recipe-safety §4.2, ADR-029
	// D5). Signals cross-feature ownership drift where replaying the older
	// whole-file recipe would silently revert a later fix.
	ErrWriteFileLaterTouch = errors.New("write-file later-touch drift detected")
)

// preimageCheckOutcome enumerates the four resolutions of a single
// write-file preimage precondition check. Kept internal — Slice 4 will
// use this to route between hard-reject and downgrade-to-warning based
// on the caller feature's supersession status.
type preimageCheckOutcome int

const (
	// preimageOK — precondition satisfied; op may execute.
	preimageOK preimageCheckOutcome = iota
	// preimageRejected — precondition failed; execute must refuse (ADR-029 D3).
	preimageRejected
	// preimageLegacyWarn — recipe omits `preimage_hash`; v1 accepts with
	// warning per ADR-029 D4 ("A v1 reader accepts existing `write-file`
	// recipes that lack `preimage_hash`, emits a warning, and applies
	// using current behavior").
	preimageLegacyWarn
	// preimageSkip — op is not a write-file (or does not target a file).
	// No precheck applies.
	preimageSkip
)

// computeFileSHA256 returns the sha256-over-exact-bytes digest of `path`
// in the `sha256:<64 lowercase hex>` display form specified by ADR-029
// D2 ("SHA-256 over exact preimage bytes... Writers do not normalize
// line endings, encodings, permissions, or JSON string escaping before
// hashing"). Bytes are read verbatim; no normalization.
//
// Callers must catch os.ErrNotExist before invoking (a missing file is
// PRD §3.3's own refusal row, distinct from a hash mismatch).
func computeFileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return PreimageHashPrefix + hex.EncodeToString(sum[:]), nil
}

// checkWriteFilePreimage evaluates the preimage precondition for one
// `write-file` operation and returns an outcome + a human-readable
// remediation message.
//
// The four table rows from PRD §3.3 map to outcomes:
//
//   - preimage_hash: "sha256:<h>" AND file exists with matching hash → preimageOK
//   - preimage_hash: "sha256:<h>" AND file missing → preimageRejected
//   - preimage_hash: "sha256:<h>" AND file exists with differing hash → preimageRejected
//   - preimage_hash: ""            AND file missing → preimageOK
//   - preimage_hash: ""            AND file exists → preimageRejected
//   - preimage_hash: absent        AND any state → preimageLegacyWarn
//   - non-write-file op            AND any state → preimageSkip
//
// The refusal message follows the ADR-020 inline-remediation contract:
// it names the feature, operation index, path, expected hash, observed
// state, and remediation category (regenerate / reconcile). ADR-029 D8
// forbids embedding file bodies in diagnostics; only paths and hashes
// appear.
func checkWriteFilePreimage(repoRoot, slug string, opIndex int, op RecipeOperation) (preimageCheckOutcome, string) {
	if op.Type != "write-file" {
		return preimageSkip, ""
	}
	target := filepath.Join(repoRoot, op.Path)

	if op.PreimageHash == nil {
		// ADR-029 D4: legacy recipe path. Warn but proceed.
		msg := fmt.Sprintf("warning: [%s] op %d %s: recipe lacks preimage_hash precondition (legacy); regenerate via 'tpatch record %s' to lock preimage safety",
			slug, opIndex, op.Path, slug)
		return preimageLegacyWarn, msg
	}

	expected := *op.PreimageHash

	// Empty preimage: PRD §3.3 new-file semantics — target must NOT exist.
	if expected == "" {
		_, err := os.Stat(target)
		if os.IsNotExist(err) {
			return preimageOK, ""
		}
		if err != nil {
			// Any other stat error (permission, etc.) is treated as a
			// precondition failure per ADR-029 D3 ("unreadable target
			// needed for a precondition").
			return preimageRejected, fmt.Sprintf("recipe drift: [%s] op %d %s: cannot stat target for new-file check: %v; verify permissions before replay",
				slug, opIndex, op.Path, err)
		}
		return preimageRejected, fmt.Sprintf("recipe drift: [%s] op %d %s: new-file collision — target already exists but recipe expected an empty preimage; regenerate the recipe against the current tree or reconcile before replay",
			slug, opIndex, op.Path)
	}

	// Non-empty preimage: validate format then compare against target.
	// Format check follows ADR-029 D1: `sha256:<64 lowercase hex>`.
	if !strings.HasPrefix(expected, PreimageHashPrefix) {
		return preimageRejected, fmt.Sprintf("recipe drift: [%s] op %d %s: malformed preimage_hash %q — expected 'sha256:<64 lowercase hex>' per ADR-029 D1",
			slug, opIndex, op.Path, expected)
	}
	if len(expected) != len(PreimageHashPrefix)+PreimageHashHexLen {
		return preimageRejected, fmt.Sprintf("recipe drift: [%s] op %d %s: malformed preimage_hash %q — expected 'sha256:<64 lowercase hex>' per ADR-029 D1",
			slug, opIndex, op.Path, expected)
	}
	hexPart := expected[len(PreimageHashPrefix):]
	if !isLowercaseHex(hexPart) {
		return preimageRejected, fmt.Sprintf("recipe drift: [%s] op %d %s: malformed preimage_hash %q — hex portion must be lowercase per ADR-029 D1",
			slug, opIndex, op.Path, expected)
	}

	observed, err := computeFileSHA256(target)
	if err != nil {
		if os.IsNotExist(err) {
			return preimageRejected, fmt.Sprintf("recipe drift: [%s] op %d %s: expected preimage %s but target file is missing; regenerate the recipe or reconcile before replay",
				slug, opIndex, op.Path, expected)
		}
		// ADR-029 D3 refusal case: unreadable target.
		return preimageRejected, fmt.Sprintf("recipe drift: [%s] op %d %s: expected preimage %s but target is unreadable: %v",
			slug, opIndex, op.Path, expected, err)
	}
	if observed != expected {
		return preimageRejected, fmt.Sprintf("recipe drift: [%s] op %d %s: expected preimage %s, observed %s; regenerate the recipe against the current tree or reconcile before replay",
			slug, opIndex, op.Path, expected, observed)
	}
	return preimageOK, ""
}

// isLowercaseHex reports whether s is a run of lowercase hex nibbles.
// A separate helper (not hex.DecodeString) so uppercase hex — which would
// decode but violates the ADR-029 D1 canonical form — is rejected.
func isLowercaseHex(s string) bool {
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}

// PreimagePrecheckResult is the aggregated verdict of the ExecuteRecipe
// precheck pass over every `write-file` operation. It carries deterministic
// per-op verdicts so callers can log warnings (legacy path) and hard-reject
// with all failing reasons in one shot (ADR-029 D3 all-or-nothing).
type PreimagePrecheckResult struct {
	// Errors are precondition failures that must block execution per
	// ADR-029 D3. Populated in operation-index order.
	Errors []string
	// Warnings are ADR-029 D4 legacy-recipe advisories emitted for
	// operations that omit `preimage_hash`. Never block execution.
	Warnings []string
}

// runWriteFilePreimagePrecheck evaluates every `write-file` operation
// in a recipe against its preimage precondition BEFORE any file
// mutation. ADR-029 D3 mandates all-or-nothing: if any precondition
// fails, no operation from the recipe is written.
//
// v0.12.0 Wave β Slice 3: the same pass also runs a path-level later-
// touch scan (PRD-write-file-recipe-safety §4.2). If any later feature
// (per RequestedAt ordering) has recorded a touch on the same path,
// the op is refused with an actionable message naming the later
// feature. This is a Wave β tightening over ADR-029 D6's "warn-only"
// baseline: per the Wave β dispatch, apply-time later-touch is
// refusal-class so silent-revert scenarios (GH #1) are blocked even
// when the operator regenerated the preimage against a stale base.
// Slice 4 downgrades both severities to warning-with-note when the
// current feature is superseded (PRD §PRD-1-interaction, ADR-029 D7).
//
// Slice 4 will layer supersession-aware severity downgrade on top of
// this (superseded features flip Errors → Warnings per PRD §PRD-1-
// interaction / ADR-029 D7). The `superseded` bool is a parameter so
// the caller controls the store lookup once per recipe.
func runWriteFilePreimagePrecheck(s *store.Store, recipe ApplyRecipe) PreimagePrecheckResult {
	var out PreimagePrecheckResult
	repoRoot := s.Root
	laterIdx := loadLaterFeatureTouches(s, recipe.Feature)
	for i, op := range recipe.Operations {
		if op.Type != "write-file" {
			continue
		}
		target := filepath.Join(repoRoot, op.Path)
		if err := safety.EnsureSafeRepoPath(repoRoot, target); err != nil {
			out.Errors = append(out.Errors, fmt.Sprintf("recipe drift: [%s] op %d %s: path safety: %v",
				recipe.Feature, i, op.Path, err))
			continue
		}
		outcome, msg := checkWriteFilePreimage(repoRoot, recipe.Feature, i, op)
		switch outcome {
		case preimageOK, preimageSkip:
			// nothing to report
		case preimageLegacyWarn:
			if msg != "" {
				out.Warnings = append(out.Warnings, msg)
			}
		case preimageRejected:
			if msg != "" {
				out.Errors = append(out.Errors, msg)
			}
		}
		// Later-touch detection runs regardless of the preimage
		// outcome so the operator sees BOTH classes of drift in one
		// shot rather than needing multiple apply attempts.
		if lt := checkLaterTouch(recipe.Feature, i, op, laterIdx); lt != "" {
			out.Errors = append(out.Errors, lt)
		}
	}
	return out
}

// laterTouchIndex maps a repo-relative path to the slug of the FIRST
// later feature (per lexicographic slug order for determinism, with
// ties broken by earliest RequestedAt) that touched that path. Missing
// entries mean no later feature touched the path.
//
// Built once per precheck invocation to keep the write-file loop O(N)
// in ops rather than O(N * F * paths_per_F) when a repo has many
// features.
type laterTouchIndex map[string]string

// loadLaterFeatureTouches inventories every feature recorded LATER
// than `currentSlug` (per RequestedAt) and returns a path→slug lookup
// covering every path any such feature touched.
//
// "Touched" per PRD §4.2 v1 detection = path-level union of:
//   - patch-generations.json.touched_paths (preferred: deterministic
//     artifact recorded at capture time),
//   - apply-recipe.json operation `path` values (fallback for features
//     that lack a patch-generations manifest).
//
// If the current slug cannot be resolved OR carries no RequestedAt
// timestamp, the index is empty — callers must not treat "cannot
// determine order" as a failure (Slice 3 is best-effort against
// available metadata; the primary defense is the preimage hash).
//
// Determinism (PRD §5 note 4 "Sort path warnings by path then feature
// slug for deterministic output"): when multiple later features
// touched the same path, the returned index carries the
// alphabetically-first slug so error messages are stable across runs.
func loadLaterFeatureTouches(s *store.Store, currentSlug string) laterTouchIndex {
	features, err := s.ListFeatures()
	if err != nil || len(features) == 0 {
		return nil
	}
	var currentReq string
	for _, f := range features {
		if f.Slug == currentSlug {
			currentReq = f.RequestedAt
			break
		}
	}
	if currentReq == "" {
		return nil
	}
	// Collect candidate later features, sorted by slug for determinism
	// (features already alphabetized by ListFeatures).
	idx := laterTouchIndex{}
	for _, f := range features {
		if f.Slug == currentSlug {
			continue
		}
		if f.RequestedAt == "" || f.RequestedAt <= currentReq {
			continue
		}
		for _, p := range collectFeatureTouchedPaths(s, f.Slug) {
			if _, seen := idx[p]; seen {
				continue
			}
			idx[p] = f.Slug
		}
	}
	return idx
}

// collectFeatureTouchedPaths returns the union of paths that `slug`
// touched, drawn from patch-generations manifests + the feature's
// apply-recipe. Best-effort: unreadable/missing artifacts yield no
// contribution rather than an error (PRD §4.2 v1 detection is
// deterministic-artifact-first with a recipe-scan fallback).
func collectFeatureTouchedPaths(s *store.Store, slug string) []string {
	seen := map[string]struct{}{}
	// Primary source: patch-generations.json.touched_paths across all
	// generations. Union covers both fresh records and later
	// amendments/fixups.
	if m, err := store.LoadPatchGenerations(s, slug); err == nil {
		for _, g := range m.Generations {
			for _, p := range g.TouchedPaths {
				seen[p] = struct{}{}
			}
		}
	}
	// Fallback / augmentation: recipe op paths. Covers features that
	// have no patch-generations yet (early-lifecycle features).
	if data, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "apply-recipe.json")); err == nil {
		var recipe ApplyRecipe
		if uerr := json.Unmarshal([]byte(data), &recipe); uerr == nil {
			for _, op := range recipe.Operations {
				if op.Path != "" {
					seen[op.Path] = struct{}{}
				}
			}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	return out
}

// checkLaterTouch returns an ADR-020-style refusal message if `op`'s
// path was touched by a later feature per `idx`, or "" when no drift
// is detected. Only `write-file` ops trigger a message — non-write
// ops flow through without checking (Slice 3 targets the silent-
// whole-file-revert class that PRD-write-file-recipe-safety §1
// isolates).
func checkLaterTouch(currentSlug string, opIndex int, op RecipeOperation, idx laterTouchIndex) string {
	if op.Type != "write-file" || idx == nil {
		return ""
	}
	laterSlug, ok := idx[op.Path]
	if !ok {
		return ""
	}
	return fmt.Sprintf("recipe drift: [%s] op %d %s: later feature %q touched this path; replaying this write-file would silently revert %q — regenerate this recipe against the current tree, or run 'tpatch reconcile %s' to plan the merge",
		currentSlug, opIndex, op.Path, laterSlug, laterSlug, currentSlug)
}
