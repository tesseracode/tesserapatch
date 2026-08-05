package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// ─── reopen ──────────────────────────────────────────────────────────────────

// verifyHistoricalEvidence re-runs the path-safety check and recomputes
// the SHA-256 of every historical evidence entry (PRD §6 "Reopen-time
// integrity check"). It runs UNCONDITIONALLY on every reopen, whether or
// not the operator attached new `--evidence` (PRD §9 tests 26 + 26b).
//
// The check never blocks the reopen: it returns the integrity verdict
// and the per-element divergence detail so the new history entry can
// record it durably.
func verifyHistoricalEvidence(root, slug string, historical []store.EvidenceRef) (integrity string, detail []store.DivergenceDetail) {
	for _, e := range historical {
		normalized, nerr := normalizeEvidencePath(e.Path)
		if nerr != nil {
			detail = append(detail, store.DivergenceDetail{
				Path:            e.Path,
				DivergentReason: store.DivergentReasonPathSafetyFailed,
			})
			continue
		}
		abs, reason := resolveEvidence(root, slug, normalized)
		if reason != "" {
			// Path-safety failures short-circuit here: no hash is ever
			// attempted, so an escaping symlink's target bytes are never
			// read (PRD §6, F-INT-3).
			detail = append(detail, store.DivergenceDetail{Path: e.Path, DivergentReason: reason})
			continue
		}
		sum, herr := evidenceHashFn(abs)
		if herr != nil {
			detail = append(detail, store.DivergenceDetail{Path: e.Path, DivergentReason: store.DivergentReasonUnreadable})
			continue
		}
		if sum != e.SHA256 {
			detail = append(detail, store.DivergenceDetail{Path: e.Path, DivergentReason: store.DivergentReasonHashMismatch})
		}
	}
	if len(detail) == 0 {
		// Clean verification: the field is OMITTED from the persisted
		// record and from the JSON envelope (ADR-031 D3 addendum).
		return "", nil
	}
	sort.Slice(detail, func(i, j int) bool { return detail[i].Path < detail[j].Path })
	return store.EvidenceIntegrityDivergent, detail
}

func reopenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reopen <slug>",
		Short: "Reopen a rejected feature (rejected → requested, append-only)",
		Long: "Reopen a rejected feature, transitioning it back to `requested`.\n\n" +
			"The prior rejection record is NEVER deleted: the live rejection is folded into an\n" +
			"append-only history entry recording the completed reject→reopen cycle, so the reason,\n" +
			"evidence, actor and timestamp all survive. Reject/reopen cycles are unbounded.\n\n" +
			"`--note` is mandatory; `--evidence` is optional (attach it when new artifacts motivated\n" +
			"the reopen). Regardless of whether new evidence is attached, EVERY historical evidence\n" +
			"reference is re-verified against its recorded SHA-256. A divergence never blocks the\n" +
			"reopen — it is recorded durably as `evidence_integrity: divergent` with a per-element\n" +
			"`divergent_reason` (hash-mismatch, missing, non-regular, path-safety-failed-at-reopen,\n" +
			"unreadable).\n\n" +
			"Exit codes: 0 success, 1 unexpected error, 2 validation error, 3 state-machine refusal.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReopen(cmd, args[0])
		},
	}
	cmd.Flags().String("note", "", "Human-readable rationale for reopening (required, non-empty)")
	cmd.Flags().StringArray("evidence", nil, "Additional evidence file path, repeatable (optional)")
	cmd.Flags().String("actor", "", "Override the resolved actor (default: $TPATCH_ACTOR, then `git config user.email`, then \"unknown\")")
	cmd.Flags().Bool("json", false, "Output as JSON")
	return cmd
}

func runReopen(cmd *cobra.Command, slug string) error {
	note, _ := cmd.Flags().GetString("note")
	rawEvidence, _ := cmd.Flags().GetStringArray("evidence")
	actorFlag, _ := cmd.Flags().GetString("actor")
	asJSON, _ := cmd.Flags().GetBool("json")

	emit := func(payload map[string]any, err error) error {
		if asJSON {
			payload["slug"] = slug
			payload["exit_code"] = exitCodeOf(err)
			if err != nil {
				payload["error"] = err.Error()
			}
			data, _ := json.MarshalIndent(payload, "", "  ")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", data)
		}
		return err
	}

	note = strings.TrimSpace(note)
	if note == "" {
		return emit(map[string]any{}, validationError("note required: --note must be a non-empty rationale string"))
	}

	s, err := openStoreFromCmd(cmd)
	if err != nil {
		return err
	}

	// New evidence is optional (REOPEN-EVIDENCE-OPTIONAL, PRD §5 rev-3)
	// but is validated exactly like reject's when supplied — and, like
	// reject's, BEFORE any check against current store state, so a
	// doubly-invalid invocation reports exit 2 rather than exit 3
	// (Cluster F' rev-1, F-INT-3; ADR-031 D4).
	newEvidence, eerr := collectEvidence(s.Root, slug, rawEvidence)
	if eerr != nil {
		return emit(map[string]any{}, eerr)
	}

	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return emit(map[string]any{}, validationError("feature %q not found: %v", slug, err))
	}
	if status.State != store.StateRejected {
		return emit(map[string]any{"state": string(status.State)},
			stateRefusalError("cannot reopen feature %q from state %q: reopen is only valid from %q", slug, status.State, store.StateRejected))
	}
	if status.Rejection == nil {
		return emit(map[string]any{"state": string(status.State)},
			stateRefusalError("cannot reopen feature %q: state is %q but no rejection record is present in status.json", slug, store.StateRejected))
	}

	// Historical-evidence verification runs unconditionally — it is
	// orthogonal to whether new evidence was attached (tests 26 + 26b).
	integrity, divergence := verifyHistoricalEvidence(s.Root, slug, status.Rejection.Evidence)

	actor := store.ResolveActorIn(actorFlag, s.Root)
	now := time.Now().UTC().Truncate(time.Second)
	// One history entry per COMPLETED reject→reopen cycle (PRD §6,
	// ADR-031 D5): the live Rejection record supplies the reject half,
	// this call supplies the reopen half, and Rejection is cleared
	// afterwards so the cycle is never double-counted.
	rejected := status.Rejection
	entry := store.RejectionHistoryEntry{
		RejectedAt:        rejected.RejectedAt,
		RejectedBy:        rejected.Actor,
		Reason:            rejected.Reason,
		RejectNote:        rejected.Note,
		PriorState:        rejected.PriorState,
		Related:           rejected.Related,
		RejectEvidence:    rejected.Evidence,
		ReopenedAt:        now,
		ReopenedBy:        actor,
		ReopenNote:        note,
		ReopenEvidence:    newEvidence,
		EvidenceIntegrity: integrity,
		DivergenceDetail:  divergence,
	}
	status.RejectionHistory = append(status.RejectionHistory, entry)
	status.Rejection = nil
	status.State = store.StateRequested
	status.LastCommand = "reopen"
	status.UpdatedAt = now.Format(time.RFC3339)
	if err := s.SaveFeatureStatus(status); err != nil {
		return err
	}

	if asJSON {
		payload := map[string]any{
			"state":           string(store.StateRequested),
			"reopened_at":     now.Format(time.RFC3339),
			"reopened_by":     actor,
			"reopen_note":     note,
			"reopen_evidence": nonNilEvidence(newEvidence),
			"history_entries": len(status.RejectionHistory),
		}
		if integrity != "" {
			payload["evidence_integrity"] = integrity
			payload["divergence_detail"] = divergence
		}
		return emit(payload, nil)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Reopened %s; state rejected → requested\n", slug)
	if integrity == store.EvidenceIntegrityDivergent {
		fmt.Fprintf(out, "  evidence integrity: divergent\n")
		for _, d := range divergence {
			fmt.Fprintf(out, "    - %s: %s\n", d.Path, d.DivergentReason)
		}
	}
	fmt.Fprintf(out, "History entries: %d (append-only; the prior rejection record is preserved)\n", len(status.RejectionHistory))
	return nil
}
