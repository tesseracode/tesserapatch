package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// TestRedactSessionForCommitLabelDropped proves the session label
// (LOCAL-ONLY per PRD §6 D14) is always scrubbed.
func TestRedactSessionForCommitLabelDropped(t *testing.T) {
	sess := store.Session{
		Label: "local reviewer note that must never cross",
		Observations: []store.SessionObservation{
			{Seq: 1, Summary: "safe redacted line about apply-recipe.json"},
		},
	}
	got, findings := redactSessionForCommit(sess)
	if len(findings) != 0 {
		t.Fatalf("expected no forbidden findings for safe body; got %v", findings)
	}
	if got.Summary == "" {
		t.Fatalf("expected non-empty safe summary; got empty")
	}
	if strings.Contains(got.Summary, "local reviewer") {
		t.Fatalf("label leaked into committed summary: %q", got.Summary)
	}
	if !containsString(got.ScrubbedFields, "label") {
		t.Fatalf("expected 'label' in ScrubbedFields; got %v", got.ScrubbedFields)
	}
}

// TestRedactSessionForCommitForbiddenClasses exercises every PRD §5
// D11 forbidden content class. Each fixture is a session with a single
// observation carrying a canonical example of the class, and the test
// asserts:
//   - the class code appears in the finding list
//   - the offending body does NOT appear in the committed summary
func TestRedactSessionForCommitForbiddenClasses(t *testing.T) {
	cases := []struct {
		name       string
		summary    string
		wantCode   string
		mustNotHit string
	}{
		{
			name:       "secret-like-string/openai-key",
			summary:    "generated sk-abc123def456ghi789jklmno while debugging",
			wantCode:   "secret-like-string",
			mustNotHit: "sk-abc123def456ghi789jklmno",
		},
		{
			name:       "secret-like-string/bearer-header",
			summary:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			wantCode:   "secret-like-string",
			mustNotHit: "Bearer eyJhbGciOiJIUzI",
		},
		{
			name:       "secret-like-string/aws-access-key",
			summary:    "aws creds: AKIAABCDEFGHIJKLMNOP",
			wantCode:   "secret-like-string",
			mustNotHit: "AKIAABCDEFGHIJKLMNOP",
		},
		{
			name:       "secret-like-string/env-assignment",
			summary:    `token = "abcdef0123456789abcdef" saved`,
			wantCode:   "secret-like-string",
			mustNotHit: "abcdef0123456789abcdef",
		},
		{
			name:       "absolute-home-path/mac",
			summary:    "user opened /Users/alice/.env for review",
			wantCode:   "absolute-home-path",
			mustNotHit: "/Users/alice/.env",
		},
		{
			name:       "absolute-home-path/linux",
			summary:    "wrote /home/bob/secrets.yml earlier",
			wantCode:   "absolute-home-path",
			mustNotHit: "/home/bob/secrets.yml",
		},
		{
			name:       "prompt-text-marker",
			summary:    "captured full system prompt for later replay",
			wantCode:   "prompt-text-marker",
			mustNotHit: "system prompt",
		},
		{
			name:       "tool-call-argument",
			summary:    `stored {"name":"apply","arguments":{"path":"a"}}`,
			wantCode:   "tool-call-argument",
			mustNotHit: `"arguments":{`,
		},
		{
			name:       "command-output-marker",
			summary:    "stderr: fatal panic in reconcile phase",
			wantCode:   "command-output-marker",
			mustNotHit: "stderr:",
		},
		{
			name:       "stack-trace-marker/go",
			summary:    "goroutine 42 [runnable]:\nmain.foo()",
			wantCode:   "stack-trace-marker",
			mustNotHit: "goroutine 42",
		},
		{
			name:       "stack-trace-marker/py",
			summary:    "Traceback (most recent call last):\n  File \"a.py\"",
			wantCode:   "stack-trace-marker",
			mustNotHit: "Traceback",
		},
		{
			name:       "ide-buffer-marker",
			summary:    "captured editor buffer contents for review",
			wantCode:   "ide-buffer-marker",
			mustNotHit: "editor buffer",
		},
		{
			name:       "clipboard-marker",
			summary:    "read from clipboard on user paste",
			wantCode:   "clipboard-marker",
			mustNotHit: "from clipboard",
		},
		{
			name:       "vector-embedding-payload",
			summary:    "embedding [0.12, 0.34, 0.56, 0.78, 0.90, 0.11, 0.22, 0.33, 0.44, 0.55, 0.66, 0.77, 0.88, 0.99, 0.10, 0.20]",
			wantCode:   "vector-embedding-payload",
			mustNotHit: "0.44, 0.55, 0.66",
		},
		{
			name:       "source-snippet-marker",
			summary:    "fenced snippet:\n```go\nfunc main() {}\n```",
			wantCode:   "source-snippet-marker",
			mustNotHit: "```go",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sess := store.Session{
				Observations: []store.SessionObservation{
					{Seq: 1, Summary: tc.summary, SymbolicRef: tc.name},
				},
			}
			got, findings := redactSessionForCommit(sess)
			if !containsString(findings, tc.wantCode) {
				t.Fatalf("expected finding code %q; got %v", tc.wantCode, findings)
			}
			if strings.Contains(got.Summary, tc.mustNotHit) {
				t.Fatalf("forbidden content %q leaked into committed summary %q", tc.mustNotHit, got.Summary)
			}
		})
	}
}

// TestSessionSummarizeRefusesWhenAllScrubbed proves PRD §5 D11 refusal:
// if EVERY observation is dropped by redaction and no safe body
// survives, `session summarize --write` MUST refuse and MUST NOT write
// a committed summary file. Existing committed summaries remain
// unchanged (PRD §8.12).
func TestSessionSummarizeRefusesWhenAllScrubbed(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "Redaction refuses on empty")
	if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
		t.Fatalf("start failed: %s", stderr)
	}

	// Poison the manifest with a forbidden-only observation so redaction
	// drops it and produces an empty body.
	s, err := store.Open(tmp)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	entries, err := s.ListSessions(slug)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 session, got %d", len(entries))
	}
	sess := *entries[0].Session
	sess.Observations = []store.SessionObservation{
		{Seq: 1, SymbolicRef: "poisoned", Summary: "leaked sk-poisoned0123456789abcdef into log"},
	}
	if err := s.SaveSession(sess); err != nil {
		t.Fatalf("save session: %v", err)
	}

	// Attempt --write summarize and expect refusal.
	out, _, code := runSessionCmd("session", "summarize", "--path", tmp, slug, "--write", "--json")
	if code != 0 {
		t.Fatalf("summarize command should exit 0 with refusal payload; got err. out=%q", out)
	}
	var payload SessionSummarizeJSON
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("summarize JSON parse: %v (%s)", err, out)
	}
	if payload.WouldWrite {
		t.Fatalf("expected would_write=false when all observations redacted; got %+v", payload)
	}
	if !strings.Contains(payload.PromotionRefusalReasonMsg, "empty summary after redaction") {
		t.Fatalf("expected D11 refusal reason in payload; got %q", payload.PromotionRefusalReasonMsg)
	}
	if !containsString(payload.ForbiddenContentFindings, "secret-like-string") {
		t.Fatalf("expected 'secret-like-string' in findings; got %v", payload.ForbiddenContentFindings)
	}

	// No committed summary must exist on disk.
	contextDir := filepath.Join(tmp, ".tpatch", "features", slug, "artifacts", "context")
	if entries, err := os.ReadDir(contextDir); err == nil && len(entries) > 0 {
		t.Fatalf("no committed summary should exist on disk after refusal; found %d entries", len(entries))
	}
}

// TestSessionSummarizePromoteWritesRedactedCopy proves the happy path
// end-to-end: session with a safe observation + a secret-leak
// observation is summarized with --write --promote. The committed
// file:
//   - contains the safe body,
//   - does NOT contain the raw leaked secret,
//   - lists the leak finding-code in redaction.finding_codes.
//
// This is the PRD §5 D11 "boundary invariant" regression test.
func TestSessionSummarizePromoteWritesRedactedCopy(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "Promote redacted copy")
	if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
		t.Fatalf("start failed: %s", stderr)
	}
	s, err := store.Open(tmp)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	entries, err := s.ListSessions(slug)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	sess := *entries[0].Session
	const rawSecret = "ghp_bogusabcdefghijklmnopqrstuvwxyz01"
	sess.Observations = []store.SessionObservation{
		{Seq: 1, SymbolicRef: "safe-note", Summary: "reviewed apply-recipe.json for feature parity"},
		{Seq: 2, SymbolicRef: "poison", Summary: "leaked " + rawSecret + " during debug"},
	}
	if err := s.SaveSession(sess); err != nil {
		t.Fatalf("save: %v", err)
	}

	out, _, code := runSessionCmd("session", "summarize", "--path", tmp, slug, "--write", "--promote", "--json")
	if code != 0 {
		t.Fatalf("summarize --write --promote failed: %s", out)
	}
	var payload SessionSummarizeJSON
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("parse: %v (%s)", err, out)
	}
	if !payload.WouldWrite {
		t.Fatalf("expected would_write=true; got %+v", payload)
	}
	if !containsString(payload.ForbiddenContentFindings, "secret-like-string") {
		t.Fatalf("expected secret finding to be recorded even though we wrote; got %v", payload.ForbiddenContentFindings)
	}

	// Committed summary must exist and NOT contain the raw secret.
	body, err := os.ReadFile(payload.CommittedSummaryPath)
	if err != nil {
		t.Fatalf("read committed summary: %v", err)
	}
	if strings.Contains(string(body), rawSecret) {
		t.Fatalf("RAW SECRET LEAKED into committed summary %s", payload.CommittedSummaryPath)
	}
	if !strings.Contains(string(body), "apply-recipe.json") {
		t.Fatalf("expected safe observation body to appear in committed summary; got:\n%s", string(body))
	}

	// The source session state must have transitioned to promoted.
	listOut, _, _ := runSessionCmd("session", "list", "--path", tmp, "--json")
	var lst SessionListJSON
	if err := json.Unmarshal([]byte(listOut), &lst); err != nil {
		t.Fatalf("list JSON: %v", err)
	}
	found := false
	for _, item := range lst.Sessions {
		if item.Feature == slug && item.State == string(store.SessionPromoted) && item.PromotedCtxID != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected session state=promoted with a ctx_id after --promote; got %+v", lst.Sessions)
	}
}

// TestSessionSummarizeDoesNotOverwriteOnRefusal proves PRD §8.12: an
// existing committed summary is left untouched when a later summarize
// attempt refuses due to redaction failure.
func TestSessionSummarizeDoesNotOverwriteOnRefusal(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "Refusal preserves prior")
	if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
		t.Fatalf("start: %s", stderr)
	}
	s, err := store.Open(tmp)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	entries, err := s.ListSessions(slug)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	sess := *entries[0].Session
	sess.Observations = []store.SessionObservation{
		{Seq: 1, SymbolicRef: "clean-1", Summary: "initial safe observation"},
	}
	if err := s.SaveSession(sess); err != nil {
		t.Fatalf("save: %v", err)
	}
	// First write - success.
	firstOut, _, code := runSessionCmd("session", "summarize", "--path", tmp, slug, "--write", "--json")
	if code != 0 {
		t.Fatalf("first summarize failed: %s", firstOut)
	}
	var first SessionSummarizeJSON
	_ = json.Unmarshal([]byte(firstOut), &first)
	priorPath := first.CommittedSummaryPath
	priorBytes, err := os.ReadFile(priorPath)
	if err != nil {
		t.Fatalf("read prior: %v", err)
	}

	// Poison the session so redaction refuses on the next summarize.
	entries2, _ := s.ListSessions(slug)
	sess2 := *entries2[0].Session
	sess2.Observations = []store.SessionObservation{
		{Seq: 2, SymbolicRef: "poison-only", Summary: "leaked sk-poisonedxxxxxxxxxxxxxxxxxx into log"},
	}
	if err := s.SaveSession(sess2); err != nil {
		t.Fatalf("save2: %v", err)
	}

	// Second write - refusal.
	out, _, code := runSessionCmd("session", "summarize", "--path", tmp, slug, "--write", "--json")
	if code != 0 {
		t.Fatalf("second summarize errored: %s", out)
	}
	var second SessionSummarizeJSON
	if err := json.Unmarshal([]byte(out), &second); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if second.WouldWrite {
		t.Fatalf("expected would_write=false on refusal; got %+v", second)
	}

	// Prior committed summary content must be byte-identical.
	nowBytes, err := os.ReadFile(priorPath)
	if err != nil {
		t.Fatalf("re-read prior: %v", err)
	}
	if string(nowBytes) != string(priorBytes) {
		t.Fatalf("refusal must not overwrite existing committed summary; diff detected")
	}
}

// (containsString is provided by internal/cli/cobra.go; do not
// re-declare it here.)
