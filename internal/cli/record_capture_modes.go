// Capture-mode dispatch for `tpatch record` (PRD-record-capture-modes v1).
//
// This file owns:
//   - mutex validation for the new + existing capture-mode flags
//     (`--all`, `--staged`, `--unstaged`, `--auto`, `--from`,
//     `--commit-range`) plus the `--to` requires-from rule.
//   - the recordCaptureMode enum used to label provenance in
//     record.md without leaking implementation detail.
//   - claim filtering for `--claimed-only` (refusal when no claims,
//     intersection with `--files`, set arithmetic against the
//     advisory claim manifest from alpha-1).
//
// The dispatch entry points (CaptureStagedPatch / CaptureUnstagedPatch)
// live in internal/gitutil; this file only orchestrates them and
// surfaces refusal diagnostics.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// recordCaptureMode is the normalized capture-mode label written to
// record.md provenance. The wire-form strings come from PRD §4.
type recordCaptureMode string

const (
	captureModeWorkingTreeAll         recordCaptureMode = "working-tree-all"
	captureModeStagedIndex            recordCaptureMode = "staged-index"
	captureModeUnstagedWorktree       recordCaptureMode = "unstaged-worktree"
	captureModeAutoCommittedRange     recordCaptureMode = "auto-committed-range"
	captureModeCommittedRange         recordCaptureMode = "committed-range"
	captureModeExplicitCommittedRange recordCaptureMode = "explicit-committed-range"
)

// recordCaptureFlags is the post-parse view of the capture-mode flag
// set. Booleans are independent of toRef/commitRange/fromRef; the
// mutex validator examines them together.
type recordCaptureFlags struct {
	All         bool
	Staged      bool
	Unstaged    bool
	Auto        bool
	From        string
	To          string
	CommitRange string
	ClaimedOnly bool
}

// validateRecordCaptureMode enforces PRD §3.7's mutex matrix. The
// helper returns a clear, single-line error on the first violation so
// the CLI surface refuses BEFORE patch capture runs. Empty input is
// valid (it represents the default working-tree-all path).
//
// Diagnostic shape preserves the legacy messages for the four
// pre-existing pairs (--auto+--from, --auto+--commit-range,
// --commit-range+--from, --commit-range+--to) and uses a uniform
// "X and Y are mutually exclusive" shape for the new pairs introduced
// by this PRD. The --to-requires-from precondition keeps its legacy
// shape too so existing tests that assert on it stay green.
func validateRecordCaptureMode(f recordCaptureFlags) error {
	from := strings.TrimSpace(f.From)
	to := strings.TrimSpace(f.To)
	commitRange := strings.TrimSpace(f.CommitRange)

	// Legacy mutex pairs — preserve historical messages verbatim.
	if f.Auto && from != "" {
		return fmt.Errorf("--auto is mutually exclusive with --from")
	}
	if f.Auto && commitRange != "" {
		return fmt.Errorf("--auto is mutually exclusive with --commit-range")
	}
	if commitRange != "" && from != "" {
		return fmt.Errorf("--commit-range is mutually exclusive with --from")
	}
	if commitRange != "" && to != "" {
		return fmt.Errorf("--commit-range is mutually exclusive with --to")
	}

	// New mutex pairs — single uniform shape for the three new flags.
	// Iterate in declaration order so messages are deterministic.
	type modeEntry struct {
		name string
		set  bool
	}
	newModes := []modeEntry{
		{"--all", f.All},
		{"--staged", f.Staged},
		{"--unstaged", f.Unstaged},
	}
	existing := []modeEntry{
		{"--auto", f.Auto},
		{"--from", from != ""},
		{"--commit-range", commitRange != ""},
	}
	// New-vs-new collisions.
	var activeNew []string
	for _, m := range newModes {
		if m.set {
			activeNew = append(activeNew, m.name)
		}
	}
	if len(activeNew) > 1 {
		return fmt.Errorf("record refuses: %s are mutually exclusive (choose one capture mode)", strings.Join(activeNew, " and "))
	}
	// New-vs-existing collisions.
	for _, n := range newModes {
		if !n.set {
			continue
		}
		for _, e := range existing {
			if e.set {
				return fmt.Errorf("record refuses: %s is mutually exclusive with %s", n.name, e.name)
			}
		}
	}

	if to != "" {
		// --to requires --from or --auto. Keep legacy shape so
		// pre-existing tests still match; this preserves the
		// "--to requires --from" substring.
		if !f.Auto && from == "" {
			return fmt.Errorf("--to requires --from (or use --commit-range <a>..<b>)")
		}
		if f.All || f.Staged || f.Unstaged {
			return fmt.Errorf("record refuses: --to is not valid with worktree/staged/unstaged capture modes")
		}
	}
	return nil
}

// resolveClaimedOnly loads the active claims for the feature slug and
// returns the claim-derived pathspec set the capture call should use,
// plus the list of claim_ids whose values contributed. Behavior:
//
//   - Refuse when the feature has no claims.
//   - When no explicit --files: return the claim values verbatim as
//     pathspecs.
//   - When --files is supplied: intersect against claims; refuse when
//     the intersection is empty.
//
// `explicit` is the post-parse `--files` slice (may be empty/nil).
func resolveClaimedOnly(s *store.Store, slug string, explicit []string) ([]string, []string, error) {
	manifest, err := store.LoadClaims(s, slug)
	if err != nil {
		return nil, nil, fmt.Errorf("--claimed-only: %w", err)
	}
	if len(manifest.Claims) == 0 {
		return nil, nil, fmt.Errorf("record --claimed-only refuses: feature %q has no active claims (run `tpatch feature claim add %s <path>...` first)", slug, slug)
	}

	var values []string
	var ids []string
	for _, c := range manifest.Claims {
		if c.Kind != store.ClaimKindPath {
			continue
		}
		values = append(values, c.Value)
		ids = append(ids, c.ClaimID)
	}
	if len(values) == 0 {
		return nil, nil, fmt.Errorf("record --claimed-only refuses: feature %q has no path-kind claims (reserved-kind claims cannot scope capture in v1)", slug)
	}

	if len(explicit) == 0 {
		sort.Strings(ids)
		return values, ids, nil
	}

	intersection := intersectExplicitAndClaims(explicit, values)
	if len(intersection) == 0 {
		return nil, nil, fmt.Errorf("record --claimed-only refuses: --files %v has empty intersection with claims %v", explicit, values)
	}
	// We can't trivially tell which subset of claim_ids actually
	// contributed to the intersection (a claim like `src/` covers any
	// `src/...` file); list all path-kind claims as a conservative
	// witness set.
	sort.Strings(ids)
	return intersection, ids, nil
}

func intersectExplicitAndClaims(explicit, claims []string) []string {
	var fileClaims []string
	var dirClaims []string
	for _, c := range claims {
		if strings.HasSuffix(c, "/") {
			dirClaims = append(dirClaims, c)
		} else {
			fileClaims = append(fileClaims, c)
		}
	}
	fileSet := map[string]bool{}
	for _, f := range fileClaims {
		fileSet[f] = true
	}
	var out []string
	for _, raw := range explicit {
		bare := strings.TrimSuffix(raw, "/")
		if fileSet[bare] {
			out = append(out, raw)
			continue
		}
		matched := false
		for _, d := range dirClaims {
			if bare == strings.TrimSuffix(d, "/") || strings.HasPrefix(bare+"/", d) {
				out = append(out, raw)
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		// converse: explicit is a dir-shape that contains a file claim.
		if strings.HasSuffix(raw, "/") {
			for _, fc := range fileClaims {
				if strings.HasPrefix(fc, bare+"/") {
					out = append(out, raw)
					break
				}
			}
		}
	}
	return out
}
