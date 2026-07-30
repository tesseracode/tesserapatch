package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// v0.12.0 Wave γ rev-1 Slice R3 (F-EXT-γ-3 HIGH). ADR-027 D3
// ("redaction before persistence") + PRD-active-feature-session §7 D16
// (no raw secret values in local buffers). Rev-0 persisted the
// `--label` verbatim into session.json — a mistyped label containing
// an API key or system-prompt marker landed on disk raw. Rev-1 pipes
// the label through the D11 forbidden-content matchers and drops it
// entirely if any class matches; safe labels are preserved.

// TestSessionStartLabelRedactsSecretShapedTokens proves the D11 scrub
// runs on --label. The persisted session.json MUST NOT contain the
// leaked token; stderr MUST record that the label was scrubbed with
// the matched finding codes.
func TestSessionStartLabelRedactsSecretShapedTokens(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "Label secret scrubbed")
	const leaked = "sk-labelabcdef01234567890abcdef"
	_, stderr, code := runSessionCmd(
		"session", "start", "--path", tmp, slug,
		"--label", "debug run "+leaked,
	)
	if code != 0 {
		t.Fatalf("start with label failed: %s", stderr)
	}
	if !strings.Contains(stderr, "--label scrubbed") {
		t.Fatalf("expected scrub notice in stderr; got %q", stderr)
	}
	if !strings.Contains(stderr, "secret-like-string") {
		t.Fatalf("expected 'secret-like-string' finding in stderr; got %q", stderr)
	}
	// The persisted manifest must not contain the raw token.
	listOut, _, listCode := runSessionCmd("session", "list", "--path", tmp, "--json", slug)
	if listCode != 0 {
		t.Fatalf("list failed")
	}
	if strings.Contains(listOut, leaked) {
		t.Fatalf("RAW LABEL LEAKED into persisted session.json: %s", listOut)
	}
	var payload SessionListJSON
	if err := json.Unmarshal([]byte(listOut), &payload); err != nil {
		t.Fatalf("list JSON parse: %v (%s)", err, listOut)
	}
	if len(payload.Sessions) != 1 {
		t.Fatalf("expected 1 session under slug %q; got %+v", slug, payload.Sessions)
	}
}

// TestSessionStartLabelHappyPathPreserved proves safe labels are not
// clobbered by the redaction pass. A benign human note should round-
// trip byte-identically into session.json.
func TestSessionStartLabelHappyPathPreserved(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "Label happy path")
	const safe = "reviewer notes for the WIP fix"
	if _, stderr, code := runSessionCmd(
		"session", "start", "--path", tmp, slug,
		"--label", safe,
	); code != 0 {
		t.Fatalf("start with safe label failed: %s", stderr)
	}
	// Direct unit-level verification through the redactor.
	got, findings := RedactSessionLabelForStore(safe)
	if len(findings) != 0 {
		t.Fatalf("safe label produced findings: %v", findings)
	}
	if got != safe {
		t.Fatalf("safe label mangled by redactor: got %q want %q", got, safe)
	}
}

// TestRedactSessionLabelForStore_ForbiddenClasses table-checks each
// D11 forbidden class against a canonical example landed as a
// --label value. Every match must trigger a drop (safe == "") and
// the finding code must appear in the returned list.
func TestRedactSessionLabelForStore_ForbiddenClasses(t *testing.T) {
	cases := []struct {
		name  string
		label string
		want  string
	}{
		{"openai-key", "temp label sk-abc123def456ghi789jklmno debug", "secret-like-string"},
		{"github-pat", "trace ghp_bogusabcdefghijklmnopqrstuvwxyz01", "secret-like-string"},
		{"aws-access-key", "aws creds AKIAABCDEFGHIJKLMNOP", "secret-like-string"},
		{"absolute-home-path", "note about /Users/alice/.env", "absolute-home-path"},
		{"prompt-marker", "captured system prompt marker", "prompt-text-marker"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, findings := RedactSessionLabelForStore(tc.label)
			if got != "" {
				t.Fatalf("expected empty safe label; got %q", got)
			}
			if !containsString(findings, tc.want) {
				t.Fatalf("expected finding %q; got %v", tc.want, findings)
			}
		})
	}
}
