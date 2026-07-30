package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/tesseracode/tesserapatch/internal/store"
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
	// per-feature artifact path. When false, the call is dry-run only.
	Write bool
	// Promote, when true, transitions the source session to `promoted`
	// after a successful write. Requires Write.
	Promote bool
	// AsJSON toggles between deterministic JSON and human output.
	AsJSON bool
}

// runSessionSummarize is the shared entry point invoked by
// `tpatch session summarize` (this file) and by `tpatch record
// --with-session` (Slice 4). It runs the D11 redaction contract
// against the session buffer, computes the ctx_<12hex> ID, and either
// dry-runs or writes the committed summary. Slice 3 adds the redaction
// contract and the write path; Slice 2 landed the wiring stub.
func runSessionSummarize(out io.Writer, s *store.Store, target store.Session, opts sessionSummarizeOpts) error {
	if opts.Promote && !opts.Write {
		return fmt.Errorf("session summarize: --promote requires --write")
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
		if opts.Promote {
			// Transition the source session to promoted so future
			// listings/audit see the boundary was crossed.
			promoted := target
			promoted.State = store.SessionPromoted
			promoted.PromotedCtxID = ctxID
			if err := s.SaveSession(promoted); err != nil {
				return fmt.Errorf("session summarize: promote: %w", err)
			}
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
		if opts.Promote {
			fmt.Fprintf(out, "  session state -> promoted\n")
		}
	} else {
		fmt.Fprintf(out, "  would write %s -> %s (pass --write to commit)\n", ctxID, committedPath)
	}
	return nil
}
