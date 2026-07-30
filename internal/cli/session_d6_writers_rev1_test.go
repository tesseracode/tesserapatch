package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

// v0.12.0 Wave γ rev-1 Slice R1 (F-EXT-γ-1 CRITICAL): PRD-active-
// feature-session §4 D6 mandate 4 verbatim: "Writers must refuse when
// Git is unavailable or the path is not ignored." Plural, unqualified.
// Rev-0 shipped only session start with the check; every later writer
// bypassed it. Rev-1 enforces the check inside Store.SaveSession so
// every session-state writer routes through the same bottleneck.
//
// These regression tests reproduce, per writer surface, the refusal
// path when .gitignore is removed after `tpatch init` so `git
// check-ignore` reports the local capture path is not ignored.

// removeGitignoreForD6 removes .gitignore so git no longer reports the
// local capture path as effectively ignored. Used to trigger mandate 4
// refusal on every writer surface.
func removeGitignoreForD6(t *testing.T, tmp string) {
	t.Helper()
	if err := os.Remove(filepath.Join(tmp, ".gitignore")); err != nil {
		t.Fatalf("rm .gitignore: %v", err)
	}
}

// assertD6RefusalMessage asserts a refusal message enumerates all six
// D6 mandates so users see the complete safety contract in one place.
func assertD6RefusalMessage(t *testing.T, stderr string) {
	t.Helper()
	if !strings.Contains(stderr, ".tpatch/local/") {
		t.Fatalf("expected refusal to name .tpatch/local/; got %q", stderr)
	}
	for i := 1; i <= 6; i++ {
		needle := "  " + itoa(i) + "."
		if !strings.Contains(stderr, needle) {
			t.Fatalf("refusal message must enumerate mandate %d; got:\n%s", i, stderr)
		}
	}
}

// TestD6MandateWriter_SessionStopRefusesWithoutGitignore proves that
// `tpatch session stop` — a Session-state WRITER that transitions
// state to `closed` and re-persists the manifest — refuses when
// .gitignore is removed after the session was started. Rev-0 bug:
// the D6 check was only wired to `session start`, so stop silently
// wrote the manifest even after the ignore rule vanished.
func TestD6MandateWriter_SessionStopRefusesWithoutGitignore(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "D6 later writer stop")
	if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
		t.Fatalf("start failed: %s", stderr)
	}
	removeGitignoreForD6(t, tmp)
	_, stderr, code := runSessionCmd("session", "stop", "--path", tmp, slug)
	if code == 0 {
		t.Fatalf("expected session stop to refuse (D6 mandate 4); got success")
	}
	assertD6RefusalMessage(t, stderr)
}

// TestD6MandateWriter_SessionSummarizeRefusesWithoutGitignore proves
// that `tpatch session summarize --write --promote` — a Session-state
// WRITER that flips the source session to `promoted` and re-persists
// the manifest — refuses when .gitignore is removed. The --promote
// half writes back a Session; the --write half writes a committed
// summary under features/<slug>/artifacts/. Rev-0 bug: the promote
// re-save happened without re-checking the ignore contract.
func TestD6MandateWriter_SessionSummarizeRefusesWithoutGitignore(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "D6 later writer summarize")
	if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
		t.Fatalf("start failed: %s", stderr)
	}
	// Seed a safe observation so redaction has something to promote.
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
		{Seq: 1, SymbolicRef: "safe", Summary: "reviewed apply-recipe.json for parity"},
	}
	if err := s.SaveSession(sess); err != nil {
		t.Fatalf("save: %v", err)
	}
	removeGitignoreForD6(t, tmp)
	_, stderr, code := runSessionCmd("session", "summarize", "--path", tmp, slug, "--write", "--promote")
	if code == 0 {
		t.Fatalf("expected session summarize --write --promote to refuse (D6 mandate 4); got success")
	}
	assertD6RefusalMessage(t, stderr)
}

// TestD6_AllWritersRefuse is the table-driven safety-margin proof for
// PRD §4 D6 mandate 4. Every Session-state-writing entry point in the
// CLI is enumerated; each one MUST refuse when the ignore contract is
// violated. Adding a NEW session-state writer without routing it
// through Store.SaveSession will fail here — that is the intended
// forcing function.
//
// Enumerated writers (rev-1 audit):
//   - session start        — new manifest at active state
//   - session stop         — active → closed transition write
//   - session summarize --write --promote — session state → promoted write
//   - record --with-session — same summarize-writer path (Slice 4)
//
// Every row prepares a fixture matching that writer's minimum
// arguments, invokes it with .gitignore removed, and asserts D6 refusal.
func TestD6_AllWritersRefuse(t *testing.T) {
	type writerCase struct {
		name    string
		prepare func(t *testing.T) (tmp, slug string)
		args    func(tmp, slug string) []string
	}
	cases := []writerCase{
		{
			name: "session-start",
			prepare: func(t *testing.T) (string, string) {
				tmp, slug := setupSessionRepo(t, "D6 all writers start")
				removeGitignoreForD6(t, tmp)
				return tmp, slug
			},
			args: func(tmp, slug string) []string {
				return []string{"session", "start", "--path", tmp, slug}
			},
		},
		{
			name: "session-stop",
			prepare: func(t *testing.T) (string, string) {
				tmp, slug := setupSessionRepo(t, "D6 all writers stop")
				if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
					t.Fatalf("start: %s", stderr)
				}
				removeGitignoreForD6(t, tmp)
				return tmp, slug
			},
			args: func(tmp, slug string) []string {
				return []string{"session", "stop", "--path", tmp, slug}
			},
		},
		{
			name: "session-summarize-write-promote",
			prepare: func(t *testing.T) (string, string) {
				tmp, slug := setupSessionRepo(t, "D6 all writers summarize")
				if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
					t.Fatalf("start: %s", stderr)
				}
				s, err := store.Open(tmp)
				if err != nil {
					t.Fatalf("open: %v", err)
				}
				entries, err := s.ListSessions(slug)
				if err != nil || len(entries) != 1 {
					t.Fatalf("list: %v (n=%d)", err, len(entries))
				}
				sess := *entries[0].Session
				sess.Observations = []store.SessionObservation{
					{Seq: 1, SymbolicRef: "safe", Summary: "reviewed feature.yaml"},
				}
				if err := s.SaveSession(sess); err != nil {
					t.Fatalf("save: %v", err)
				}
				removeGitignoreForD6(t, tmp)
				return tmp, slug
			},
			args: func(tmp, slug string) []string {
				return []string{"session", "summarize", "--path", tmp, slug, "--write", "--promote"}
			},
		},
		{
			name: "record-with-session",
			prepare: func(t *testing.T) (string, string) {
				tmp, slug := setupSessionRepo(t, "D6 all writers record")
				if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slug); code != 0 {
					t.Fatalf("start: %s", stderr)
				}
				s, err := store.Open(tmp)
				if err != nil {
					t.Fatalf("open: %v", err)
				}
				entries, err := s.ListSessions(slug)
				if err != nil || len(entries) != 1 {
					t.Fatalf("list: %v (n=%d)", err, len(entries))
				}
				sess := *entries[0].Session
				sess.Observations = []store.SessionObservation{
					{Seq: 1, SymbolicRef: "safe", Summary: "reviewed apply-recipe.json"},
				}
				if err := s.SaveSession(sess); err != nil {
					t.Fatalf("save: %v", err)
				}
				writeStagedFileForRecord(t, tmp, "README.txt", "record d6 all-writers\n")
				removeGitignoreForD6(t, tmp)
				return tmp, slug
			},
			args: func(tmp, slug string) []string {
				return []string{"record", "--path", tmp, slug, "--with-session"}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp, slug := tc.prepare(t)
			_, stderr, code := runSessionCmd(tc.args(tmp, slug)...)
			if code == 0 {
				t.Fatalf("[%s] expected D6 mandate-4 refusal; got success", tc.name)
			}
			if !strings.Contains(stderr, ".tpatch/local/") {
				t.Fatalf("[%s] refusal must name .tpatch/local/; got %q", tc.name, stderr)
			}
			// Every writer routes the D6 refusal through the same
			// sentinel enumerating all six mandates.
			_ = workflow.ErrLocalIgnoreRefusal
			for i := 1; i <= 6; i++ {
				needle := "  " + itoa(i) + "."
				if !strings.Contains(stderr, needle) {
					t.Fatalf("[%s] refusal must enumerate mandate %d; got:\n%s", tc.name, i, stderr)
				}
			}
		})
	}
}
