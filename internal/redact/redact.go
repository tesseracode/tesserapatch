// Package redact hosts the shared, content-agnostic byte-pattern
// matchers used by every tpatch surface that must refuse to persist
// secret-shaped content.
//
// It exists because ADR-033 D4 (PRD-feature-resource-claims-and-capture-adapters
// §8.2) requires the matchers previously private to
// internal/cli/session_redaction.go to be reusable by resource capture
// without duplicating the patterns. Two distinct policies share one
// matcher inventory:
//
//   - SessionClasses — the ten PRD-active-feature-session §5 D11
//     classes, applied with drop-the-line-and-continue semantics by the
//     session summarize path. Behaviour is byte-identical to the
//     pre-extraction implementation.
//   - Scan — the six closed resource-capture classes of ADR-033 D4,
//     applied as a hard refusal of the whole invocation.
//
// Scan deliberately takes an in-memory []byte and never a path: the
// "raw bytes are never written to disk before scanning" property is
// then structural rather than a call-site discipline a later change
// could quietly violate.
package redact

import (
	"regexp"
	"sort"
)

// Class is one named content class plus the matcher that detects it.
type Class struct {
	Code  string
	Match func(string) bool
}

// SessionClasses is the ordered list applied to every session
// observation summary and session label. Order is stable so audit
// finding-code lists stay deterministic.
var SessionClasses = []Class{
	{"secret-like-string", MatchSecretLikeString},
	{"absolute-home-path", MatchAbsoluteHomePath},
	{"prompt-text-marker", MatchPromptTextMarker},
	{"tool-call-argument", MatchToolCallArgument},
	{"command-output-marker", MatchCommandOutputMarker},
	{"stack-trace-marker", MatchStackTraceMarker},
	{"ide-buffer-marker", MatchIDEBufferMarker},
	{"clipboard-marker", MatchClipboardMarker},
	{"vector-embedding-payload", MatchVectorEmbeddingPayload},
	{"source-snippet-marker", MatchSourceSnippetMarker},
}

// Resource-capture finding codes (ADR-033 D4 / PRD §8.2's six closed
// classes). These are the only codes Scan can ever return.
const (
	ClassPrivateKey           = "private-key"
	ClassConnectionURL        = "connection-url"
	ClassEmailPII             = "email-pii"
	ClassCredentialAssignment = "credential-assignment"
	ClassBearerOrKeyToken     = "bearer-or-key-token"
	ClassHomeAbsolutePath     = "home-absolute-path"
)

// ResourceClasses is the closed six-class set resource capture applies.
// Unlike SessionClasses this list is not extended by future session
// policy work: ADR-033 D4 fixes it at six, and any match hard-refuses
// the whole capture invocation.
var ResourceClasses = []Class{
	{ClassPrivateKey, MatchPrivateKeyMaterial},
	{ClassConnectionURL, MatchConnectionURL},
	{ClassEmailPII, MatchEmailPII},
	{ClassCredentialAssignment, MatchCredentialAssignment},
	{ClassBearerOrKeyToken, MatchBearerOrKeyToken},
	{ClassHomeAbsolutePath, MatchAbsoluteHomePath},
}

// Scan reports every resource-capture class matched by content. The
// returned codes are sorted and deduplicated so a caller's diagnostic
// output is deterministic. An empty result means the content may be
// hashed and its structural summary published.
//
// Scan never mutates content and never retains a reference to it.
func Scan(content []byte) []string {
	if len(content) == 0 {
		return nil
	}
	return ScanString(string(content))
}

// ScanString is Scan for an already-decoded string (selectors, args,
// resolved Git metadata values).
func ScanString(s string) []string {
	if s == "" {
		return nil
	}
	seen := map[string]struct{}{}
	for _, cls := range ResourceClasses {
		if cls.Match(s) {
			seen[cls.Code] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	codes := make([]string, 0, len(seen))
	for code := range seen {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

// MatchAny reports the finding codes for an arbitrary class list. It is
// the shared primitive behind both policies.
func MatchAny(classes []Class, s string) []string {
	var codes []string
	for _, cls := range classes {
		if cls.Match(s) {
			codes = append(codes, cls.Code)
		}
	}
	return codes
}

// ─── session matchers (moved verbatim from internal/cli/session_redaction.go) ──
//
// The patterns below are unchanged from the pre-extraction
// implementation. Session behaviour must stay byte-identical; any
// tightening belongs in the resource-only matchers further down.

var (
	// Secrets: common API-key prefixes, generic "******",
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

// MatchSecretLikeString reports well-known API-key/bearer/credential
// shapes.
func MatchSecretLikeString(s string) bool {
	if s == "" {
		return false
	}
	return reAPIKeyPrefix.MatchString(s) || reBearerToken.MatchString(s) || reEnvAssign.MatchString(s)
}

// MatchAbsoluteHomePath reports an absolute path rooted in a user home
// directory.
func MatchAbsoluteHomePath(s string) bool { return reAbsHomePath.MatchString(s) }

// MatchPromptTextMarker reports provider prompt/response markers.
func MatchPromptTextMarker(s string) bool { return rePromptMarker.MatchString(s) }

// MatchToolCallArgument reports serialized tool-call argument payloads.
func MatchToolCallArgument(s string) bool { return reToolCallArgs.MatchString(s) }

// MatchCommandOutputMarker reports captured shell output markers.
func MatchCommandOutputMarker(s string) bool { return reCommandOutput.MatchString(s) }

// MatchStackTraceMarker reports Go/Python/JS stack-trace shapes.
func MatchStackTraceMarker(s string) bool { return reStackTrace.MatchString(s) }

// MatchIDEBufferMarker reports IDE buffer/selection payload markers.
func MatchIDEBufferMarker(s string) bool { return reIDEBuffer.MatchString(s) }

// MatchClipboardMarker reports clipboard-content markers.
func MatchClipboardMarker(s string) bool { return reClipboard.MatchString(s) }

// MatchVectorEmbeddingPayload reports a dense float array.
func MatchVectorEmbeddingPayload(s string) bool { return reVectorArray.MatchString(s) }

// MatchSourceSnippetMarker reports fenced source excerpts.
func MatchSourceSnippetMarker(s string) bool { return reSourceSnippet.MatchString(s) }

// ─── resource-capture-only matchers (ADR-033 D4 classes 1-3) ─────────────────
//
// Classes 4/5/6 reuse the session matchers above verbatim
// (credential assignments, bearer/key tokens, home absolute paths).
// Classes 1/2/3 are new: session redaction never had dedicated
// PEM/OpenSSH, connection-URL, or email/PII patterns.

var (
	// PEM and OpenSSH private-key envelopes. Matches the BEGIN header
	// for any key type, plus PuTTY's distinct format.
	rePrivateKey = regexp.MustCompile(`(?i)(-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----|-----BEGIN OPENSSH PRIVATE KEY-----|PuTTY-User-Key-File-\d)`)

	// Connection URLs carrying embedded userinfo. The first alternative
	// pins the well-known database/broker schemes even when the
	// credential half is absent; the second is the generalized
	// scheme://user:pass@host shape for anything else.
	reConnectionURLScheme   = regexp.MustCompile(`(?i)\b(postgres(?:ql)?|mysql|mariadb|mongodb(?:\+srv)?|rediss?|amqps?|clickhouse|cockroachdb|mssql|sqlserver|jdbc:[a-z0-9]+)://[^\s"']+`)
	reConnectionURLUserinfo = regexp.MustCompile(`\b[a-zA-Z][a-zA-Z0-9+.\-]*://[^\s/:@"']+:[^\s/@"']+@[^\s/"']+`)

	// Email addresses. Deliberately broad: the resource-capture policy
	// hard-refuses, so a false positive costs an explicit operator
	// decision rather than a silent leak.
	reEmailPII = regexp.MustCompile(`(?i)\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)
)

// MatchPrivateKeyMaterial reports a PEM/OpenSSH/PuTTY private-key
// envelope (ADR-033 D4 class 1).
func MatchPrivateKeyMaterial(s string) bool { return rePrivateKey.MatchString(s) }

// MatchConnectionURL reports a database/broker connection URL or any
// URL carrying embedded userinfo credentials (class 2).
func MatchConnectionURL(s string) bool {
	return reConnectionURLScheme.MatchString(s) || reConnectionURLUserinfo.MatchString(s)
}

// MatchEmailPII reports an email address (class 3).
func MatchEmailPII(s string) bool { return reEmailPII.MatchString(s) }

// MatchCredentialAssignment reports a secret/token/password/api_key
// assignment (class 4). This is the session reEnvAssign pattern used
// on its own, so the resource policy can name the class precisely.
func MatchCredentialAssignment(s string) bool { return reEnvAssign.MatchString(s) }

// MatchBearerOrKeyToken reports a bearer token or a well-known API-key
// prefix (class 5).
func MatchBearerOrKeyToken(s string) bool {
	return reBearerToken.MatchString(s) || reAPIKeyPrefix.MatchString(s)
}
