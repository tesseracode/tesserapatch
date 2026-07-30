package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// writeStagedFileForRecord creates a simple staged change so
// `tpatch record` has something to capture. Returns after the file is
// staged.
func writeStagedFileForRecord(t *testing.T, root, relPath, content string) {
	t.Helper()
	full := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// TestRecordFromSessionRequiresWithSession proves PRD §8.8:
// `--from-session` without `--with-session` is a refusal.
func TestRecordFromSessionRequiresWithSession(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "Record from-session guard")
	// Attempt record with --from-session but not --with-session.
	// Provide a dummy cs_ id — the refusal must fire BEFORE any
	// session lookup happens.
	_, errMsg, code := runSessionCmd(
		"record", "--path", tmp, slug,
		"--from-session", "cs_deadbeef0011",
	)
	if code == 0 {
		t.Fatalf("expected refusal for --from-session without --with-session; got success")
	}
	if !strings.Contains(errMsg, "requires --with-session") {
		t.Fatalf("expected 'requires --with-session' in stderr; got %q", errMsg)
	}
	if !strings.Contains(errMsg, "§8.8") {
		t.Fatalf("expected PRD §8.8 citation in stderr; got %q", errMsg)
	}
}

// TestRecordWithSessionCrossFeatureIsolationRefused proves PRD §7 D18:
// `record <slug-B> --with-session --from-session <cs_id_of_A>` MUST
// refuse — session A's manifest lives under feature A's directory,
// LoadSession(slugB, csA) fails because the manifest's feature field
// disagrees with the directory path.
func TestRecordWithSessionCrossFeatureIsolationRefused(t *testing.T) {
	tmp := t.TempDir()
	initGitRepoForCLI(t, tmp)
	if _, stderr, code := runSessionCmd("init", "--path", tmp); code != 0 {
		t.Fatalf("init failed: %s", stderr)
	}
	if _, stderr, code := runSessionCmd("add", "--path", tmp, "Cross iso A"); code != 0 {
		t.Fatalf("add A failed: %s", stderr)
	}
	if _, stderr, code := runSessionCmd("add", "--path", tmp, "Cross iso B"); code != 0 {
		t.Fatalf("add B failed: %s", stderr)
	}
	s, err := store.Open(tmp)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	features, err := s.ListFeatures()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(features) < 2 {
		t.Fatalf("expected 2 features")
	}
	slugA, slugB := features[0].Slug, features[1].Slug

	// Start a session under slug A.
	if _, stderr, code := runSessionCmd("session", "start", "--path", tmp, slugA); code != 0 {
		t.Fatalf("start A failed: %s", stderr)
	}
	entriesA, _ := s.ListSessions(slugA)
	if len(entriesA) != 1 {
		t.Fatalf("expected 1 session under A")
	}
	csA := entriesA[0].SessionID

	// Attempt `record slugB --with-session --from-session csA`.
	_, errMsg, code := runSessionCmd(
		"record", "--path", tmp, slugB,
		"--with-session", "--from-session", csA,
	)
	if code == 0 {
		t.Fatalf("expected cross-feature refusal; got success")
	}
	if !strings.Contains(errMsg, "record --with-session") {
		t.Fatalf("expected 'record --with-session' scope in stderr; got %q", errMsg)
	}
}

// TestRecordWithSessionPromotesRedactedSummary proves the happy path:
// after starting a session and appending a safe observation, `record
// <slug> --with-session` produces a committed summary under the
// feature's artifacts/context/ directory AND flips the source session
// to `promoted`. Because record needs an actual patch to capture,
// this test stages a simple file first so record does not bail on
// "no changes".
func TestRecordWithSessionPromotesRedactedSummary(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "Record with session promotes")

	// Start session and append a safe observation.
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
	sess.Observations = []store.SessionObservation{
		{Seq: 1, SymbolicRef: "safe-1", Summary: "reviewed feature.yaml scope for record test"},
	}
	if err := s.SaveSession(sess); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Stage a file so record has something to capture.
	writeStagedFileForRecord(t, tmp, "README.txt", "hello wave gamma\n")

	// Run record with --with-session (exactly one eligible session
	// → auto-selected per PRD §8.7).
	out, errMsg, code := runSessionCmd(
		"record", "--path", tmp, slug,
		"--with-session",
	)
	if code != 0 {
		t.Fatalf("record --with-session failed: out=%q err=%q", out, errMsg)
	}

	// A committed context summary MUST now exist under the feature.
	contextDir := filepath.Join(tmp, ".tpatch", "features", slug, "artifacts", "context")
	dirEntries, err := os.ReadDir(contextDir)
	if err != nil {
		t.Fatalf("read context dir: %v", err)
	}
	if len(dirEntries) != 1 {
		t.Fatalf("expected exactly 1 committed summary; got %d", len(dirEntries))
	}
	body, err := os.ReadFile(filepath.Join(contextDir, dirEntries[0].Name()))
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	if !strings.Contains(string(body), "feature.yaml") {
		t.Fatalf("expected safe observation body in committed summary; got:\n%s", string(body))
	}

	// Session state must have flipped to promoted with a ctx_id.
	listOut, _, _ := runSessionCmd("session", "list", "--path", tmp, "--json")
	var lst SessionListJSON
	if err := json.Unmarshal([]byte(listOut), &lst); err != nil {
		t.Fatalf("list JSON: %v", err)
	}
	found := false
	for _, item := range lst.Sessions {
		if item.Feature == slug &&
			item.State == string(store.SessionPromoted) &&
			strings.HasPrefix(item.PromotedCtxID, "ctx_") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected session promoted with ctx_ id after record --with-session; got %+v", lst.Sessions)
	}
}

// TestRecordWithSessionRefusesAmbiguousWithoutFromSession proves PRD
// §8.7: when multiple eligible same-feature sessions exist,
// `record --with-session` without `--from-session` MUST refuse.
// (We can only construct this state by hand — normal usage never
// produces two active sessions because start is idempotent — but the
// refusal protects the invariant against future writers that produce
// closed-plus-active or closed-plus-closed states.)
func TestRecordWithSessionRefusesAmbiguousWithoutFromSession(t *testing.T) {
	tmp, slug := setupSessionRepo(t, "Ambiguous session refusal")
	s, err := store.Open(tmp)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Manually build two closed sessions for the same feature by
	// writing two manifests with different content-addressed IDs.
	for i, mode := range []store.SessionCaptureMode{store.SessionCaptureSummary, store.SessionCaptureLocalEvents} {
		id := store.ComputeSessionID(store.SessionIdentityInputs{
			SchemaVersion:          store.SessionSchemaVersion,
			RepositoryIdentity:     "test-repo",
			Feature:                slug,
			BaseCommit:             "deadbeef",
			CaptureMode:            string(mode),
			WorkspaceDiscriminator: "test-workspace",
		})
		sess := store.Session{
			SchemaVersion: store.SessionSchemaVersion,
			ID:            id,
			Feature:       slug,
			State:         store.SessionClosed,
			CaptureMode:   mode,
			BaseCommit:    "deadbeef",
			Observations: []store.SessionObservation{
				{Seq: i + 1, SymbolicRef: "safe", Summary: "safe body"},
			},
		}
		if err := s.SaveSession(sess); err != nil {
			t.Fatalf("save session %d: %v", i, err)
		}
	}

	writeStagedFileForRecord(t, tmp, "README.txt", "hello ambiguous\n")

	_, errMsg, code := runSessionCmd(
		"record", "--path", tmp, slug,
		"--with-session",
	)
	if code == 0 {
		t.Fatalf("expected refusal on ambiguous session selection; got success")
	}
	if !strings.Contains(errMsg, "multiple eligible sessions") {
		t.Fatalf("expected 'multiple eligible sessions' in stderr; got %q", errMsg)
	}
	// v0.12.0 Wave γ rev-1 Slice R6 (F-INT-γ-2 LOW). The record surface
	// disambiguates via `--from-session`, not `--session` — rewrite the
	// helper's canonical hint so the operator's remediation matches
	// the actual flag they need to pass.
	if !strings.Contains(errMsg, "--from-session") {
		t.Fatalf("expected '--from-session' remediation hint in stderr; got %q", errMsg)
	}
	if strings.Contains(errMsg, "pass --session <cs_id>") {
		t.Fatalf("record ambiguity message must not surface the raw `--session <cs_id>` hint (that flag belongs to session subcommands, not record); got %q", errMsg)
	}
}
