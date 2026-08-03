package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// v0.12.0 Wave γ rev-1.5 (F-EXT-γ-1 residual, CRITICAL): rev-1
// rearchitected D6 enforcement to a bottleneck inside Store.SaveSession
// via SessionIgnoreVerifier — that covers every SaveSession call site
// (session start/stop/summarize/record). However, runSessionSummarize
// wrote the COMMITTED-lane ctx_<12hex>.json via SaveContextSummary
// BEFORE calling SaveSession. On D6 refusal SaveSession would refuse,
// but the orphan ctx artifact would already sit in
// .tpatch/features/<slug>/artifacts/context/ — a violation of PRD §5
// D9→D10 promotion atomicity.
//
// Rev-1.5 preflights workflow.EnsureLocalIgnoreContract at the top of
// runSessionSummarize when opts.Write is set, ahead of any writer
// surface. This regression asserts:
//   1. exit non-zero,
//   2. the committed context directory is empty (or absent),
//   3. the D6 six-mandate refusal message is emitted,
//   4. the source session state remains `active` (no partial promotion).

// TestSessionSummarize_D6RefusalLeavesNoOrphanCtxArtifact drives the
// full `session summarize --write --json` path with .gitignore removed
// so mandate 5 fails. The invariant under test is the atomicity of
// D9→D10: after refusal, the committed lane must contain zero
// ctx_<12hex>.json artifacts and the source session's state must not
// have transitioned to `promoted`.
func TestSessionSummarize_D6RefusalLeavesNoOrphanCtxArtifact(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "R1.5 D6 preflight no orphan ctx")

	if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
		t.Fatalf("start: %s", stderr)
	}

	// Seed a safe observation so the redactor produces a non-empty
	// committed body — otherwise the D11 empty-summary refusal would
	// fire first and mask the D6 preflight we are exercising.
	s, err := store.Open(tmp)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	entries, err := s.ListSessions(slug)
	if err != nil || len(entries) != 1 {
		t.Fatalf("list sessions: %v (n=%d)", err, len(entries))
	}
	sess := *entries[0].Session
	sess.Observations = []store.SessionObservation{
		{Seq: 1, SymbolicRef: "safe", Summary: "reviewed apply-recipe.json for D6 preflight"},
	}
	if err := s.SaveSession(sess); err != nil {
		t.Fatalf("save observation: %v", err)
	}

	// Break mandate 5 by removing .gitignore so `git check-ignore`
	// reports the local capture path is not ignored.
	if err := os.Remove(filepath.Join(tmp, ".gitignore")); err != nil {
		t.Fatalf("rm .gitignore: %v", err)
	}

	_, stderr, code := runSessionCmd("session", "summarize", "--path", tmp, slug, "--write", "--json")
	if code == 0 {
		t.Fatalf("expected D6 preflight refusal; got success (stderr: %q)", stderr)
	}

	// Six-mandate refusal message must reach the user.
	if !strings.Contains(stderr, ".tpatch/local/") {
		t.Fatalf("refusal must name .tpatch/local/; got %q", stderr)
	}
	for i := 1; i <= 6; i++ {
		needle := "  " + itoa(i) + "."
		if !strings.Contains(stderr, needle) {
			t.Fatalf("refusal must enumerate mandate %d; got:\n%s", i, stderr)
		}
	}

	// Committed context lane MUST be empty. The whole point of the
	// preflight is that no ctx_<12hex>.json ever lands.
	ctxDir := filepath.Join(tmp, ".tpatch", "features", slug, "artifacts", "context")
	if ents, err := os.ReadDir(ctxDir); err == nil {
		if len(ents) != 0 {
			names := make([]string, 0, len(ents))
			for _, e := range ents {
				names = append(names, e.Name())
			}
			t.Fatalf("expected empty committed context lane after D6 refusal; found %d entries: %v", len(ents), names)
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat committed context dir: %v", err)
	}

	// Source session state must NOT have transitioned to `promoted`.
	// Reopen the store to defeat any stale in-memory optimism.
	s2, err := store.Open(tmp)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	entries2, err := s2.ListSessions(slug)
	if err != nil {
		t.Fatalf("list after refusal: %v", err)
	}
	if len(entries2) != 1 {
		t.Fatalf("expected 1 session after refusal, got %d", len(entries2))
	}
	if got := entries2[0].Session.State; got != store.SessionActive {
		t.Fatalf("expected session state to remain %q after D6 refusal, got %q", store.SessionActive, got)
	}
	if entries2[0].Session.PromotedCtxID != "" {
		t.Fatalf("expected PromotedCtxID to remain empty after D6 refusal, got %q", entries2[0].Session.PromotedCtxID)
	}
}

// TestSessionSummarize_D6RefusalEmitsJSONEnvelope is the Wave γ
// LOW-γr15-N1 regression. Before this fix, `session summarize --json
// --write` on a D6 ignore-rule violation printed the plaintext
// six-mandate refusal message ONLY via the returned error (surfaced
// as "error: ...\n" on real stderr by cli.Execute) -- stdout stayed
// empty. In --json mode ALL output, including refusals, must be a
// JSON envelope on stdout. This test asserts stdout is valid JSON
// carrying an "error" field naming the violation, a "message" field
// with the human-readable detail, a "citation" field, and that the
// process exits non-zero.
func TestSessionSummarize_D6RefusalEmitsJSONEnvelope(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "N1 D6 JSON envelope")

	if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
		t.Fatalf("start: %s", stderr)
	}

	s, err := store.Open(tmp)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	entries, err := s.ListSessions(slug)
	if err != nil || len(entries) != 1 {
		t.Fatalf("list sessions: %v (n=%d)", err, len(entries))
	}
	sess := *entries[0].Session
	sess.Observations = []store.SessionObservation{
		{Seq: 1, SymbolicRef: "safe", Summary: "reviewed apply-recipe.json for D6 JSON envelope"},
	}
	if err := s.SaveSession(sess); err != nil {
		t.Fatalf("save observation: %v", err)
	}

	// Break mandate 5 by removing .gitignore so `git check-ignore`
	// reports the local capture path is not ignored.
	if err := os.Remove(filepath.Join(tmp, ".gitignore")); err != nil {
		t.Fatalf("rm .gitignore: %v", err)
	}

	stdout, _, code := runSessionCmd("session", "summarize", "--path", tmp, slug, "--write", "--json")
	if code == 0 {
		t.Fatalf("expected non-zero exit for D6 refusal under --json")
	}

	var envelope struct {
		Error    string `json:"error"`
		Message  string `json:"message"`
		Citation string `json:"citation"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &envelope); err != nil {
		t.Fatalf("stdout must be valid JSON envelope, got parse error %v; stdout=%q", err, stdout)
	}
	if envelope.Error != "d6_ignore_rule_violation" {
		t.Fatalf("expected error=%q, got %q", "d6_ignore_rule_violation", envelope.Error)
	}
	if !strings.Contains(envelope.Message, ".tpatch/local/") {
		t.Fatalf("expected message to name .tpatch/local/, got %q", envelope.Message)
	}
	if envelope.Citation == "" {
		t.Fatalf("expected non-empty citation field")
	}
}
