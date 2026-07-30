package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// v0.12.0 Wave γ rev-1 Slice R2 (F-EXT-γ-2 HIGH). PRD-active-feature-
// session §5 D11 verbatim: "Redaction failure is a hard failure."
//
// Rev-0 emitted a refusal payload but returned nil (exit 0). External
// review flagged this as a contract violation. Rev-1 wraps the refusal
// in ErrSessionRedactionRefusal so callers can errors.Is off ONE
// sentinel — mirrors the Wave β F-M1 sentinel-wrap pattern.

// TestSessionSummarizeHardFailureReturnsSentinel proves the returned
// error wraps ErrSessionRedactionRefusal so downstream code
// (`record --with-session`, tests, doctor gates) can classify the
// refusal by identity via errors.Is.
func TestSessionSummarizeHardFailureReturnsSentinel(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "D11 hard failure sentinel")
	if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
		t.Fatalf("start: %s", stderr)
	}
	s, err := store.Open(tmp)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	entries, err := s.ListSessions(slug)
	if err != nil || len(entries) != 1 {
		t.Fatalf("list: err=%v n=%d", err, len(entries))
	}
	sess := *entries[0].Session
	// Poison-only session — every observation drops on redaction so
	// the safe body is empty and D11 fires.
	sess.Observations = []store.SessionObservation{
		{Seq: 1, SymbolicRef: "poison", Summary: "leaked sk-poisonaaaaaaaaaaaaaaaaaaaaaaaa in log"},
	}
	if err := s.SaveSession(sess); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Drive runSessionSummarize DIRECTLY so we can errors.Is the
	// returned error — the CLI wrapper flattens error identity when
	// bouncing through cobra.
	target := sess
	var sink strings.Builder
	runErr := runSessionSummarize(&sink, s, target, sessionSummarizeOpts{Write: true})
	if runErr == nil {
		t.Fatalf("D11 hard failure MUST return non-nil error; got nil")
	}
	if !errors.Is(runErr, ErrSessionRedactionRefusal) {
		t.Fatalf("expected errors.Is(err, ErrSessionRedactionRefusal); got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "PRD §5 D11") {
		t.Fatalf("refusal error must cite PRD §5 D11; got %q", runErr.Error())
	}
}

// TestSessionSummarizeDryRunRefusalStillExitsZero proves the D11
// hard-failure conversion only fires when opts.Write is set. A
// `--dry-run` (or default) summarize that produces an empty body must
// still exit 0 with the refusal payload so operators can preview
// before running with --write.
func TestSessionSummarizeDryRunRefusalStillExitsZero(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "D11 dry-run refusal exits zero")
	if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
		t.Fatalf("start: %s", stderr)
	}
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
		{Seq: 1, SymbolicRef: "poison", Summary: "leaked sk-poisonaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	if err := s.SaveSession(sess); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Default (no --write) — must exit 0 even though redaction dropped
	// everything.
	out, errMsg, code := runSessionCmd("session", "summarize", "--path", tmp, slug)
	if code != 0 {
		t.Fatalf("dry-run must exit 0; got code=%d out=%q err=%q", code, out, errMsg)
	}
}
