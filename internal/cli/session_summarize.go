package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

// ErrSessionRedactionRefusal fires when a session summarize --write
// call cannot produce a safe committed body (every observation was
// dropped by D11 redaction). PRD-active-feature-session §5 D11
// verbatim: "Redaction failure is a hard failure." Callers use
// errors.Is to distinguish the D11 refusal from unrelated I/O errors.
//
// v0.12.0 Wave γ rev-1 Slice R2 (F-EXT-γ-2 fix). Rev-0 emitted a
// refusal payload but returned nil so the process exited 0. External
// review flagged this as a contract-authority violation. The sentinel
// wraps the human-visible refusal string so tests can key off ONE
// error identity — mirrors the Wave β F-M1 sentinel-wrap pattern for
// ErrWriteFilePreimageMismatch.
var ErrSessionRedactionRefusal = errors.New("session summarize: redaction refused (PRD §5 D11 hard failure)")

// sessionSummarizeOpts groups the flags for runSessionSummarize.
type sessionSummarizeOpts struct {
	// Write, when true, writes the redacted committed summary to the
	// per-feature artifact path AND transitions the source session to
	// `promoted` (PRD §5 D9 rule 3 verbatim: "`--write` is the
	// mutating mode."). When false, the call is dry-run only.
	//
	// v0.12.0 Wave γ rev-1 Slice R6 (F-EXT-γ-6 MEDIUM): rev-0 split
	// the write and the promote transition across two flags. External
	// review flagged this as a D9 semantic mismatch — the PRD lists
	// `--write` as the sole mutating trigger. Collapsing keeps
	// intuition: `--write` writes and marks the source promoted.
	Write bool
	// AsJSON toggles between deterministic JSON and human output.
	AsJSON bool
}

// runSessionSummarize is the shared entry point invoked by
// `tpatch session summarize` (this file) and by `tpatch record
// --with-session` (Slice 4). It runs the D11 redaction contract
// against the session buffer, computes the ctx_<12hex> ID, and either
// dry-runs or writes the committed summary + promotes the source
// session in ONE step.
//
// v0.12.0 Wave γ rev-1 Slice R6 (F-EXT-γ-6 fix). Rev-0 had two flags:
// `--write` for the committed summary + a separate `--promote` for
// the state transition. PRD §5 D9 rule 3 verbatim ("`--write` is the
// mutating mode.") and the D9 command listing show `--write` as the
// singular mutating trigger. Rev-1 collapses the two into `--write`.
func runSessionSummarize(out io.Writer, s *store.Store, target store.Session, opts sessionSummarizeOpts) error {
	// v0.12.0 Wave γ rev-1.5 (F-EXT-γ-1 residual): the D6 bottleneck
	// inside Store.SaveSession covers the promoted-manifest write at
	// the tail of this function, but SaveContextSummary below runs
	// BEFORE SaveSession and would leave an orphan ctx_<12hex>.json in
	// the COMMITTED lane (.tpatch/features/<slug>/artifacts/context/)
	// when D6 is violated. Preflighting workflow.EnsureLocalIgnoreContract
	// here — before any writer surface — enforces D9→D10 promotion
	// atomicity: either both the ctx artifact AND the promoted manifest
	// land, or neither. Dry-run (opts.Write == false) intentionally
	// bypasses this preflight because it reads but never writes.
	//
	// PRD-active-feature-session §4 D6 mandate 4 verbatim: "Writers
	// must refuse when Git is unavailable or the path is not ignored."
	// PRD §5 D9→D10 mandates the promotion is a single atomic hop from
	// the local buffer to the committed lane; a partial write breaks
	// the atomicity contract.
	if opts.Write {
		if err := workflow.EnsureLocalIgnoreContract(s.Root, s.LocalCaptureDir()); err != nil {
			return fmt.Errorf("session summarize: D6 preflight refused (PRD §4 D6 mandate 4 + PRD §5 D9→D10 atomicity — enforced upstream of SaveContextSummary): %w", err)
		}
	}

	redacted, findings := redactSessionForCommit(target)
	summaryBody := redacted.Summary
	ctxID := store.ComputeContextSummaryID(store.ContextSummaryIDInputs{
		SchemaVersion: store.SessionSchemaVersion,
		Feature:       target.Feature,
		SessionID:     target.ID,
		CaptureMode:   string(target.CaptureMode),
		SummaryHash:   store.HashSummaryBody(summaryBody),
	})
	redactionStatus := "clean"
	if len(findings) > 0 {
		redactionStatus = "scrubbed"
	}
	// PRD §5 D11: redaction failure prevents committed writes and
	// leaves existing summaries unchanged (PRD §8.12). "Failure" here
	// means the redactor could not produce a safe payload — an empty
	// summary body after scrubbing is treated as failure so we do not
	// write an empty ctx_<12hex> artifact.
	refusalReason := ""
	if opts.Write && summaryBody == "" {
		refusalReason = "empty summary after redaction (PRD §5 D11 refusal — nothing safe to commit)"
	}

	cs := store.ContextSummary{
		SchemaVersion: store.SessionSchemaVersion,
		ID:            ctxID,
		Feature:       target.Feature,
		SessionID:     target.ID,
		CaptureMode:   string(target.CaptureMode),
		Summary:       summaryBody,
		Redaction: store.ContextSummaryRedaction{
			Status:         redactionStatus,
			FindingCodes:   findings,
			ScrubbedFields: redacted.ScrubbedFields,
		},
	}

	committedPath := s.FeatureContextSummaryPath(target.Feature, ctxID)
	wouldWrite := opts.Write && refusalReason == ""

	if wouldWrite {
		if err := s.SaveContextSummary(cs); err != nil {
			return fmt.Errorf("session summarize: %w", err)
		}
		// v0.12.0 Wave γ rev-1 Slice R6 (F-EXT-γ-6): `--write` is now
		// the SINGLE mutating trigger per PRD §5 D9. Every successful
		// committed-summary write also transitions the source session
		// to `promoted` so audit + `session list` reflect the boundary
		// crossing.
		promoted := target
		promoted.State = store.SessionPromoted
		promoted.PromotedCtxID = ctxID
		if err := s.SaveSession(promoted); err != nil {
			return fmt.Errorf("session summarize: promote: %w", err)
		}
	}

	if opts.AsJSON {
		payload := SessionSummarizeJSON{
			SchemaVersion:             store.SessionSchemaVersion,
			Feature:                   target.Feature,
			SessionID:                 target.ID,
			WouldWrite:                wouldWrite,
			SummaryID:                 ctxID,
			RedactionStatus:           redactionStatus,
			ForbiddenContentFindings:  findings,
			PromotionRefusalReasonMsg: refusalReason,
		}
		if wouldWrite {
			payload.CommittedSummaryPath = committedPath
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
		// PRD §5 D11 verbatim: "Redaction failure is a hard failure."
		// Emit the JSON payload (so downstream parsers still see the
		// full refusal record) and THEN return a wrapped sentinel so
		// the process exits non-zero when the caller asked to write.
		// Slice R2 (F-EXT-γ-2 fix).
		if refusalReason != "" && opts.Write {
			return fmt.Errorf("%w: %s", ErrSessionRedactionRefusal, refusalReason)
		}
		return nil
	}

	fmt.Fprintf(out, "session %s (feature %s):\n", target.ID, target.Feature)
	fmt.Fprintf(out, "  redaction: %s\n", redactionStatus)
	if len(findings) > 0 {
		fmt.Fprintf(out, "  scrubbed:  %v\n", findings)
	}
	if refusalReason != "" {
		fmt.Fprintf(out, "  refused:   %s\n", refusalReason)
		// PRD §5 D11 verbatim: "Redaction failure is a hard failure."
		// Only convert to a hard failure when the caller asked to
		// write; dry-run should still exit 0 with the refusal payload
		// visible. See Slice R2 (F-EXT-γ-2 fix).
		if opts.Write {
			return fmt.Errorf("%w: %s", ErrSessionRedactionRefusal, refusalReason)
		}
		return nil
	}
	if wouldWrite {
		fmt.Fprintf(out, "  wrote %s -> %s\n", ctxID, committedPath)
		fmt.Fprintf(out, "  session state -> promoted\n")
	} else {
		fmt.Fprintf(out, "  would write %s -> %s (pass --write to commit)\n", ctxID, committedPath)
	}
	return nil
}
