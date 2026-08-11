package cli

import (
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/redact"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// redactedSession is the intermediate value produced by
// redactSessionForCommit.
type redactedSession struct {
	Summary        string
	ScrubbedFields []string
}

// forbiddenContentClasses is the ordered list applied to every
// observation summary and the session label. Order is stable so audit
// finding-code lists are deterministic (PRD §8.15).
//
// The matchers themselves live in internal/redact (ADR-033 D4 /
// PRD-feature-resource-claims-and-capture-adapters §8.2 extracted them
// so resource capture can reuse the same byte patterns). The session
// policy is unchanged by that move: the same ten classes, in the same
// order, with the same drop-the-line-and-continue semantics.
var forbiddenContentClasses = redact.SessionClasses

// redactSessionForCommit applies PRD-active-feature-session §5 D11 to
// produce a committed-safe snapshot of a session. Every observation
// summary and the (dropped) label is checked against the forbidden
// content classes; any positive match is recorded as a finding code
// and the offending line is DROPPED from the committed body.
//
// If EVERY observation is dropped and no clean lines survive, the
// caller (runSessionSummarize) treats it as a redaction refusal and
// declines to write — preserving PRD §5 D11's "raw bodies NEVER cross
// the boundary" invariant.
//
// Labels are LOCAL-ONLY per PRD §6 D14 and are dropped unconditionally
// with a `label` entry in ScrubbedFields.
func redactSessionForCommit(sess store.Session) (redactedSession, []string) {
	findingSet := map[string]struct{}{}
	var scrubbed []string

	if sess.Label != "" {
		scrubbed = append(scrubbed, "label")
	}

	var lines []string
	for _, obs := range sess.Observations {
		body, obsFindings := redactObservation(obs)
		for _, f := range obsFindings {
			findingSet[f] = struct{}{}
		}
		if len(obsFindings) > 0 {
			// PRD §5 D11: drop the offending line rather than write
			// a partially-scrubbed body. Callers get a
			// "scrubbed" redaction status and a machine-readable
			// finding-code list.
			scrubbed = append(scrubbed, "observation."+obs.SymbolicRef)
			continue
		}
		if body != "" {
			lines = append(lines, body)
		}
	}

	findings := make([]string, 0, len(findingSet))
	for k := range findingSet {
		findings = append(findings, k)
	}
	sort.Strings(findings)

	return redactedSession{
		Summary:        strings.Join(lines, "\n"),
		ScrubbedFields: scrubbed,
	}, findings
}

// redactObservation returns the observation body plus any finding
// codes matched against PRD §5 D11 forbidden content classes.
func redactObservation(obs store.SessionObservation) (string, []string) {
	var findings []string
	findings = append(findings, redact.MatchAny(forbiddenContentClasses, obs.Summary)...)
	if len(findings) > 0 {
		return "", findings
	}
	return obs.Summary, nil
}

// RedactSessionLabelForStore applies the PRD §5 D11 forbidden-content
// matchers to a `--label` value before it lands in a session manifest.
// PRD §7 D16 + ADR-027 D3 forbid raw secret values / prompt-like
// content in local buffers; the label field is a rare exception (local
// display only), but rev-0 persisted it verbatim so a mistyped label
// containing an API key was written to disk before any redaction ran.
//
// v0.12.0 Wave γ rev-1 Slice R3 (F-EXT-γ-3 HIGH). Reuses the same D11
// forbidden-content classes runSessionSummarize applies to observations.
// Returns the safe label (empty string when ANY finding matched, since
// dropping the whole label is safer than a partially-scrubbed variant)
// and the ordered finding-code list for user-facing diagnostics.
func RedactSessionLabelForStore(label string) (safe string, findings []string) {
	if label == "" {
		return "", nil
	}
	codes := redact.MatchAny(forbiddenContentClasses, label)
	if len(codes) > 0 {
		return "", codes
	}
	return label, nil
}
