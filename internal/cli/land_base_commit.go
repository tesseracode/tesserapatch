package cli

// `land` producer contract for `Tpatch-Base-Commit` — v0.15.1 Wave C /
// GH #8, ADR-013 Amendment 1 D19 and PRD-tpatch-land §3.8.6.
//
// `land` used to interpolate `status.Apply.BaseCommit` into the trailer
// block with no validation (internal/cli/land.go:394), so a legacy or
// corrupt status could emit `Tpatch-Base-Commit: ` with an empty value —
// evidence the §3.8.2 grammar is REQUIRED to classify as malformed at
// every future read. A producer must not create evidence the reader must
// reject.
//
// This file adds EXACTLY ONE refusal (R23) and changes nothing else:
// every pre-existing refusal, its ordering, its message and the entire
// successful path are byte-unchanged.
//
// The guarantee is stated by invocation mode, honestly:
//
//   - Mode A (`--no-record`): validation is inserted immediately AFTER
//     `recoverLand` returns and before every land-owned mutation. With
//     no pending journal that means no mutation whatsoever; with a
//     pending journal `recoverLand` may already have finished a PRIOR
//     interrupted transaction, and the refusal names it.
//   - Mode B (embedded `record`): the field is `record`'s output, so
//     `land` re-validates the reloaded value immediately after `record`
//     returns and before any land-owned mutation. `record`'s artifacts
//     persist — it completed as an independent transaction — and the
//     refusal says so.

import (
	"fmt"
	"os"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// baseCommitDefect enumerates the three invalid shapes named in R23.
type baseCommitDefect string

const (
	baseCommitEmpty        baseCommitDefect = "empty"
	baseCommitMalformed    baseCommitDefect = "malformed"
	baseCommitUnresolvable baseCommitDefect = "unresolvable"
)

// landBaseCommitNote is the extra sentence R23 carries per mode.
type landBaseCommitNote string

const (
	noteNone              landBaseCommitNote = ""
	noteRecordArtifacts   landBaseCommitNote = "note: the embedded record step already completed; its artifacts under .tpatch/features/%s/artifacts/ are retained and are not rolled back"
	noteRecoveryCompleted landBaseCommitNote = "note: an interrupted previous land was recovered before this refusal; that recovery is complete and is not rolled back"
)

// validateLandBaseCommit implements the D19 validity predicate:
//
//  1. non-empty;
//  2. well-formed — `N` lowercase hex where N DERIVES from
//     `git rev-parse --show-object-format` (40 sha1 / 64 sha256);
//  3. resolvable — `GIT_NO_LAZY_FETCH=1 git rev-parse --verify <base>^{commit}`;
//  4. reachable from HEAD — unless the repository is shallow or a
//     partial clone, where unreachability is a one-line warn and the
//     landing proceeds.
//
// It returns a non-nil error ONLY for conditions 1–3. `warn` is emitted
// for the shallow/partial unreachable case.
func validateLandBaseCommit(repoRoot, slug string, status store.FeatureStatus, note landBaseCommitNote, warn func(string, ...any)) error {
	base := strings.TrimSpace(status.Apply.BaseCommit)
	if base == "" {
		return landBaseCommitRefusal(slug, baseCommitEmpty, status.Apply.BaseCommit, note)
	}

	facts, err := gitutil.ReadRepoFacts(repoRoot)
	if err != nil {
		return fmt.Errorf("land refuses: cannot read the repository object format to validate status.apply.base_commit: %v", err)
	}
	if !gitutil.IsLowercaseHexOfLen(base, facts.CommitIDHexLen) {
		return landBaseCommitRefusal(slug, baseCommitMalformed, status.Apply.BaseCommit, note)
	}
	if _, _, err := gitutil.ResolveRevOffline(repoRoot, base+"^{commit}"); err != nil {
		return landBaseCommitRefusal(slug, baseCommitUnresolvable, status.Apply.BaseCommit, note)
	}

	reachable, ancErr := gitutil.IsAncestorOffline(repoRoot, base, "HEAD")
	if ancErr == nil && !reachable {
		// Unreachability alone NEVER refuses. In a shallow or partial
		// clone the object may simply be outside local history.
		if warn != nil {
			warn("warning: status.apply.base_commit %s is not reachable from HEAD (shallow=%v partial=%v); landing proceeds\n",
				base, facts.Shallow, facts.PartialClone)
		}
	}
	return nil
}

// landBaseCommitRefusal renders R23 verbatim.
func landBaseCommitRefusal(slug string, defect baseCommitDefect, value string, note landBaseCommitNote) error {
	msg := fmt.Sprintf("land refuses: status.apply.base_commit is %s (%s); the Tpatch-Base-Commit trailer would be unreadable. Run tpatch record %s --auto (or --from <base>) to repopulate it, then re-run tpatch land %s",
		defect, value, slug, slug)
	switch note {
	case noteRecordArtifacts:
		msg += "\n" + fmt.Sprintf(string(noteRecordArtifacts), slug)
	case noteRecoveryCompleted:
		msg += "\n" + string(noteRecoveryCompleted)
	}
	// Plain error: `land` refusals collapse to exit 1 exactly like every
	// pre-existing one (PRD-tpatch-land §3.2).
	return fmt.Errorf("%s", msg)
}

// landJournalPending reports whether a land journal exists for slug at
// command entry. Used only to select R23's note; recovery itself is
// unchanged and still runs first.
func landJournalPending(repoRoot, slug string) bool {
	_, err := os.Stat(landJournalPath(repoRoot, slug))
	return err == nil
}
