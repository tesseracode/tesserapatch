package cli

import (
	"strings"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// redactedSession is the intermediate value produced by
// redactSessionForCommit. Slice 2 keeps a minimal shape; Slice 3
// expands the enumeration of scrubbed field categories and the
// D11-forbidden content classes.
type redactedSession struct {
	Summary        string
	ScrubbedFields []string
}

// redactSessionForCommit applies PRD-active-feature-session §5 D11 to
// produce a committed-safe snapshot of a session. Slice 2 lands the
// contract skeleton: it drops labels and enumerates observation
// summaries as the committed body. Slice 3 wires in the forbidden-
// content matchers (secrets, prompts, IDE buffers, absolute home
// paths, etc.) and reports finding codes for the redaction audit.
//
// The returned findings slice is the list of PRD §5 D11 finding codes
// (e.g. `secret-like-string`, `absolute-home-path`) that fired against
// the session. An empty slice + non-empty Summary means the redactor
// found no forbidden content; a non-empty slice means one or more
// classes were scrubbed OR the write must refuse.
func redactSessionForCommit(sess store.Session) (redactedSession, []string) {
	var findings []string
	var scrubbed []string

	// Slice 2: synthesize a minimal committed body from the observation
	// summaries. Labels are LOCAL-ONLY per PRD §6 D14 — never promoted
	// without a further explicit user redaction — so the label is
	// dropped unconditionally.
	if sess.Label != "" {
		scrubbed = append(scrubbed, "label")
	}

	var lines []string
	for _, obs := range sess.Observations {
		body, obsFindings := redactObservation(obs)
		findings = append(findings, obsFindings...)
		if body != "" {
			lines = append(lines, body)
		}
	}
	return redactedSession{
		Summary:        strings.Join(lines, "\n"),
		ScrubbedFields: scrubbed,
	}, findings
}

// redactObservation is the per-observation D11 filter. Slice 2 keeps
// the summary text as-is (observations are already required to be
// redacted-only per PRD §7 D16 write contract) and reports no
// findings. Slice 3 layers the forbidden-content matchers on top so
// mis-behaving writers who slipped raw bodies into observations are
// caught at the boundary.
func redactObservation(obs store.SessionObservation) (string, []string) {
	return obs.Summary, nil
}
