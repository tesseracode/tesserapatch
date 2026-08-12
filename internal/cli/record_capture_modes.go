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
	"io"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
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

	var pathClaims []store.Claim
	var values []string
	var ids []string
	for _, c := range manifest.Claims {
		if c.Kind != store.ClaimKindPath {
			continue
		}
		pathClaims = append(pathClaims, c)
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

	intersection, contribIDs := intersectExplicitAndClaimsWithIDs(explicit, pathClaims)
	if len(intersection) == 0 {
		return nil, nil, fmt.Errorf("record --claimed-only refuses: --files %v has empty intersection with claims %v", explicit, values)
	}
	contribIDs = sortDedupe(contribIDs)
	return intersection, contribIDs, nil
}

// sortDedupe returns a stably sorted copy of in with duplicate
// entries removed. Returns nil when in is empty so callers see a
// zero-length slice that JSON/Markdown formatters treat as "none".
func sortDedupe(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	cp := make([]string, len(in))
	copy(cp, in)
	sort.Strings(cp)
	out := cp[:0]
	var prev string
	for i, s := range cp {
		if i == 0 || s != prev {
			out = append(out, s)
			prev = s
		}
	}
	return out
}

// intersectExplicitAndClaimsWithIDs returns the intersection of
// explicit pathspecs and claim values, plus the claim_id slice of
// claims that actually contributed to the match. Order in the
// returned IDs is the observation order in the explicit-iteration
// loop; the caller is expected to sort and dedupe.
//
// Match shapes mirror the alpha-2 PRD §3.5 semantics:
//  1. File-claim exact match — explicit pathspec equals a file claim.
//  2. Dir-claim coverage — a dir claim (trailing-slash) covers the
//     explicit pathspec (explicit may be a file or a dir under the
//     claim).
//  3. Converse — a dir-shape explicit covers a file claim.
//
// Each branch records the contributing claim's ID so the provenance
// block lists only the subset that actually scoped the capture.
func intersectExplicitAndClaimsWithIDs(explicit []string, claims []store.Claim) (paths []string, ids []string) {
	var fileClaims []store.Claim
	var dirClaims []store.Claim
	for _, c := range claims {
		if strings.HasSuffix(c.Value, "/") {
			dirClaims = append(dirClaims, c)
		} else {
			fileClaims = append(fileClaims, c)
		}
	}
	fileClaimsByValue := map[string]store.Claim{}
	for _, fc := range fileClaims {
		fileClaimsByValue[fc.Value] = fc
	}
	for _, raw := range explicit {
		bare := strings.TrimSuffix(raw, "/")
		matched := false
		// Branch 1: file-claim exact match.
		if fc, ok := fileClaimsByValue[bare]; ok {
			paths = append(paths, raw)
			ids = append(ids, fc.ClaimID)
			matched = true
		}
		// Branch 2: dir-claim coverage of the explicit pathspec.
		// Iterate every dir claim so multi-overlap (e.g. `src/` AND
		// `src/a.go` both claimed with `--files src/a.go`) records
		// both contributing IDs.
		for _, d := range dirClaims {
			if bare == strings.TrimSuffix(d.Value, "/") || strings.HasPrefix(bare+"/", d.Value) {
				if !matched {
					paths = append(paths, raw)
					matched = true
				}
				ids = append(ids, d.ClaimID)
			}
		}
		if matched {
			continue
		}
		// Branch 3 (converse): dir-shape explicit covering a file
		// claim. Only reached when neither file-claim exact match
		// nor dir-claim coverage hit.
		if strings.HasSuffix(raw, "/") {
			for _, fc := range fileClaims {
				if strings.HasPrefix(fc.Value, bare+"/") {
					if !matched {
						paths = append(paths, raw)
						matched = true
					}
					ids = append(ids, fc.ClaimID)
				}
			}
		}
	}
	return paths, ids
}

// nestedWorktreeScopedPathspecs partitions the caller's pathspecs into
// the ones that name (or sit inside) a registered nested linked Git
// worktree and the ones that do not. Both slices preserve input order.
//
// Pathspecs may carry git magic (`:(exclude)…`, `:/…`) or a trailing
// slash; only the plain path portion is classified, and a pathspec
// tpatch cannot interpret as a plain path is conservatively treated as
// "not a worktree" so the caller never invents a misleading refusal.
func nestedWorktreeScopedPathspecs(repoRoot string, pathspecs []string) (worktreeSpecs, otherSpecs []string, err error) {
	if len(pathspecs) == 0 {
		return nil, nil, nil
	}
	prefixes, err := gitutil.NestedWorktreePrefixes(repoRoot)
	if err != nil {
		return nil, nil, err
	}
	for _, spec := range pathspecs {
		if len(prefixes) > 0 && !strings.HasPrefix(spec, ":") &&
			gitutil.PathUnderNestedWorktree(spec, prefixes) {
			worktreeSpecs = append(worktreeSpecs, spec)
			continue
		}
		otherSpecs = append(otherSpecs, spec)
	}
	return worktreeSpecs, otherSpecs, nil
}

// nestedWorktreeEmptyCaptureRefusal produces the actionable refusal for
// an empty scoped capture whose pathspecs named registered nested
// linked worktrees (GH #7, rev-0 user-external note).
//
// Without it, `record --files '.claude/worktrees/agent review'` fell
// through to the generic zero-byte diagnostic, which speculates about
// "mode-only or binary changes" — actively misleading, because the
// paths were deliberately filtered out by the nested-worktree guard.
//
// Returns nil when no requested pathspec is a nested worktree, so
// genuinely empty non-worktree captures keep their existing
// diagnostics byte-for-byte.
func nestedWorktreeEmptyCaptureRefusal(w io.Writer, repoRoot, slug string, pathspecs []string) error {
	worktreeSpecs, otherSpecs, err := nestedWorktreeScopedPathspecs(repoRoot, pathspecs)
	if err != nil {
		return err
	}
	if len(worktreeSpecs) == 0 {
		return nil
	}

	noun := "path"
	if len(worktreeSpecs) > 1 {
		noun = "paths"
	}
	if len(otherSpecs) == 0 {
		fmt.Fprintf(w, "tpatch record captured 0 bytes: every requested %s is a registered nested Git worktree and %s intentionally excluded from capture.\n",
			noun, plural(len(worktreeSpecs), "is", "are"))
	} else {
		fmt.Fprintf(w, "tpatch record captured 0 bytes. %d requested %s %s a registered nested Git worktree and %s intentionally excluded from capture; the remaining requested path(s) produced no textual diff.\n",
			len(worktreeSpecs), noun, plural(len(worktreeSpecs), "is", "are"), plural(len(worktreeSpecs), "was", "were"))
	}
	fmt.Fprintln(w, "  Excluded as nested linked worktrees:")
	for _, p := range worktreeSpecs {
		fmt.Fprintf(w, "    - %s\n", p)
	}
	if len(otherSpecs) > 0 {
		fmt.Fprintln(w, "  Requested and not excluded (no diff):")
		for _, p := range otherSpecs {
			fmt.Fprintf(w, "    - %s\n", p)
		}
	}
	fmt.Fprintln(w, "  A nested linked worktree is another checkout's working directory, not feature content:")
	fmt.Fprintln(w, "  capturing it would record a mode-160000 gitlink and pull unrelated state into the patch.")
	fmt.Fprintln(w, "  Resolve with one of:")
	fmt.Fprintf(w, "    - name the files you actually changed: tpatch record %s --files <path>[,<path>...]\n", slug)
	fmt.Fprintln(w, "    - to capture work done INSIDE that worktree, run tpatch with --path set to the worktree root")
	fmt.Fprintln(w, "    - if the directory should not be a worktree at all: git worktree remove <path>")
	return fmt.Errorf("empty capture: requested path(s) are registered nested Git worktrees — see diagnostic above")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
