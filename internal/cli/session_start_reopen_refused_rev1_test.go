package cli

import (
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// v0.12.0 Wave γ rev-1 Slice R5 (F-EXT-γ-5 HIGH). PRD-active-feature-
// session §3 D4 verbatim: "reopen is out of scope and valid transitions
// do not include `closed → active`." Content-addressing per §3 D3 is
// intentional: same identity inputs → same cs_ id. Rev-0 only counted
// active sessions on start, so `session stop` + `session start` at the
// same base commit + capture mode silently reopened the closed manifest
// (mutating its state to active). Rev-1 refuses instead.

// TestSessionStartAfterCloseRefusesReopen closes a session and then
// re-invokes `session start` under identical conditions. The refusal
// MUST fire and cite PRD §3 D4.
func TestSessionStartAfterCloseRefusesReopen(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "R5 reopen refused")
	if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
		t.Fatalf("first start: %s", stderr)
	}
	if _, stderr, code := runSessionCmd("session", "stop", "--path", tmp, slug); code != 0 {
		t.Fatalf("stop: %s", stderr)
	}
	// Re-start at the same base commit + capture mode → same cs_ id.
	_, errMsg, code := runSessionCmd("session", "start", "--path", tmp, slug)
	if code == 0 {
		t.Fatalf("expected session start to refuse after close; got success")
	}
	if !strings.Contains(errMsg, "PRD §3 D4") {
		t.Fatalf("expected refusal to cite PRD §3 D4; got %q", errMsg)
	}
	if !strings.Contains(errMsg, "closed") {
		t.Fatalf("expected refusal to name closed state; got %q", errMsg)
	}
}

// TestSessionStartAfterPromoteRefusesReopen mirrors the above for the
// `promoted` terminal state. Promoting via `session summarize --write
// --promote` should also block a same-id restart.
func TestSessionStartAfterPromoteRefusesReopen(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "R5 reopen refused after promote")
	if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
		t.Fatalf("start: %s", stderr)
	}
	// Seed a safe observation so redaction has content to promote.
	s, err := store.Open(tmp)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	entries, err := s.ListSessions(slug)
	if err != nil || len(entries) != 1 {
		t.Fatalf("list: err=%v n=%d", err, len(entries))
	}
	sess := *entries[0].Session
	sess.Observations = []store.SessionObservation{
		{Seq: 1, SymbolicRef: "safe", Summary: "reviewed apply-recipe.json"},
	}
	if err := s.SaveSession(sess); err != nil {
		t.Fatalf("save: %v", err)
	}
	if out, stderr, code := runSessionCmd("session", "summarize", "--path", tmp, slug, "--write", "--promote"); code != 0 {
		t.Fatalf("promote: out=%q err=%q", out, stderr)
	}
	// Re-start at the same content-addressed id → refusal.
	_, errMsg, code := runSessionCmd("session", "start", "--path", tmp, slug)
	if code == 0 {
		t.Fatalf("expected refusal after promote; got success")
	}
	if !strings.Contains(errMsg, "PRD §3 D4") {
		t.Fatalf("expected PRD §3 D4 citation; got %q", errMsg)
	}
	if !strings.Contains(errMsg, "promoted") {
		t.Fatalf("expected refusal to name promoted state; got %q", errMsg)
	}
}
