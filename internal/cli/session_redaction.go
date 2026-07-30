package cli

import (
	"regexp"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// redactedSession is the intermediate value produced by
// redactSessionForCommit.
type redactedSession struct {
	Summary        string
	ScrubbedFields []string
}

// forbiddenContentClass enumerates PRD-active-feature-session §5 D11
// forbidden content classes. Each class has a finding code (used as
// the audit-trail entry in ContextSummaryRedaction.FindingCodes) and
// a matcher function that returns true if the class is present in the
// given string.
type forbiddenContentClass struct {
	code    string
	matcher func(string) bool
}

// forbiddenContentClasses is the ordered list applied to every
// observation summary and the session label. Order is stable so audit
// finding-code lists are deterministic (PRD §8.15).
var forbiddenContentClasses = []forbiddenContentClass{
	{"secret-like-string", matchSecretLikeString},
	{"absolute-home-path", matchAbsoluteHomePath},
	{"prompt-text-marker", matchPromptTextMarker},
	{"tool-call-argument", matchToolCallArgument},
	{"command-output-marker", matchCommandOutputMarker},
	{"stack-trace-marker", matchStackTraceMarker},
	{"ide-buffer-marker", matchIDEBufferMarker},
	{"clipboard-marker", matchClipboardMarker},
	{"vector-embedding-payload", matchVectorEmbeddingPayload},
	{"source-snippet-marker", matchSourceSnippetMarker},
}

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
	for _, cls := range forbiddenContentClasses {
		if cls.matcher(obs.Summary) {
			findings = append(findings, cls.code)
		}
	}
	if len(findings) > 0 {
		return "", findings
	}
	return obs.Summary, nil
}

// ─── D11 matchers ────────────────────────────────────────────────────────────

// PRD §5 D11 forbidden classes each have a matcher below. The matchers
// are heuristic: they aim to catch the well-known shapes without
// dependencies. A future PRD may layer a stricter policy (see the
// deferred PRD-record-context-summary spec).

var (
	// Secrets: common API-key prefixes, generic "Bearer <token>",
	// AWS access keys, GitHub personal access tokens.
	reAPIKeyPrefix = regexp.MustCompile(`(?i)\b(sk-[A-Za-z0-9]{20,}|ghp_[A-Za-z0-9]{20,}|gho_[A-Za-z0-9]{20,}|ghu_[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}|xox[baprs]-[A-Za-z0-9-]{10,})\b`)
	reBearerToken  = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9\-_\.]{20,}\b`)
	reEnvAssign    = regexp.MustCompile(`(?i)\b(secret|token|password|api[_-]?key|access[_-]?key)\b\s*[:=]\s*['"]?[A-Za-z0-9\-_\.]{16,}`)

	// Absolute home paths on unix or windows user directories.
	reAbsHomePath = regexp.MustCompile(`(^|[^A-Za-z0-9])((/Users/|/home/|/root/|C:\\Users\\)[^\s"']+)`)

	// Prompt / provider text markers.
	rePromptMarker = regexp.MustCompile(`(?i)\b(system prompt|user prompt|assistant response|chain[- ]of[- ]thought|prompt bytes|<\|im_start\|>|<\|im_end\|>)\b`)

	// Tool-call arguments — sample JSON-like structure.
	reToolCallArgs = regexp.MustCompile(`(?i)"(arguments|tool_input|input|args)"\s*:\s*(\{|\[)`)

	// Command output markers.
	reCommandOutput = regexp.MustCompile(`(?i)\b(stdout|stderr|command output|shell output)\s*[:=]`)

	// Stack traces (Go / Python / JS).
	reStackTrace = regexp.MustCompile(`(?m)^(\s*at\s+[A-Za-z_][A-Za-z0-9_\.]*\s*\(|Traceback \(most recent call last\)|goroutine \d+ \[)`)

	// IDE buffer / selection markers.
	reIDEBuffer = regexp.MustCompile(`(?i)\b(editor buffer|editor selection|active buffer|open file contents|lsp payload|textDocument\/(didChange|didOpen))\b`)

	// Clipboard markers.
	reClipboard = regexp.MustCompile(`(?i)\b(clipboard contents|from clipboard|paste buffer)\b`)

	// Vector-like embeddings: a JSON array of many floats.
	reVectorArray = regexp.MustCompile(`\[\s*-?\d+\.\d+(?:\s*,\s*-?\d+\.\d+){15,}\s*\]`)

	// Source snippet markers: fenced code blocks with language tags
	// commonly attached to raw source excerpts.
	reSourceSnippet = regexp.MustCompile("(?m)^```(go|python|py|ts|tsx|js|jsx|rust|rs|c|cpp|h|hpp|java)\\b")
)

func matchSecretLikeString(s string) bool {
	if s == "" {
		return false
	}
	return reAPIKeyPrefix.MatchString(s) || reBearerToken.MatchString(s) || reEnvAssign.MatchString(s)
}

func matchAbsoluteHomePath(s string) bool {
	return reAbsHomePath.MatchString(s)
}

func matchPromptTextMarker(s string) bool {
	return rePromptMarker.MatchString(s)
}

func matchToolCallArgument(s string) bool {
	return reToolCallArgs.MatchString(s)
}

func matchCommandOutputMarker(s string) bool {
	return reCommandOutput.MatchString(s)
}

func matchStackTraceMarker(s string) bool {
	return reStackTrace.MatchString(s)
}

func matchIDEBufferMarker(s string) bool {
	return reIDEBuffer.MatchString(s)
}

func matchClipboardMarker(s string) bool {
	return reClipboard.MatchString(s)
}

func matchVectorEmbeddingPayload(s string) bool {
	return reVectorArray.MatchString(s)
}

func matchSourceSnippetMarker(s string) bool {
	return reSourceSnippet.MatchString(s)
}
