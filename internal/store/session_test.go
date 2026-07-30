package store_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestComputeSessionIDDeterministic(t *testing.T) {
	in := store.SessionIdentityInputs{
		SchemaVersion:          store.SessionSchemaVersion,
		RepositoryIdentity:     "abc123def456",
		Feature:                "add-cool-feature",
		BaseCommit:             "0123456789abcdef0123456789abcdef01234567",
		CaptureMode:            "summary",
		WorkspaceDiscriminator: "/repo/worktree",
	}
	id1 := store.ComputeSessionID(in)
	id2 := store.ComputeSessionID(in)
	if id1 != id2 {
		t.Fatalf("session ID not deterministic: %q != %q", id1, id2)
	}
	if !store.IsValidSessionID(id1) {
		t.Fatalf("session ID %q does not match cs_<12hex> shape", id1)
	}
	// Different inputs must yield different IDs (basic collision check).
	in2 := in
	in2.Feature = "add-different-feature"
	if store.ComputeSessionID(in2) == id1 {
		t.Fatalf("session ID collides across different features")
	}
}

func TestSessionIDShapeValidation(t *testing.T) {
	cases := []struct {
		name string
		id   string
		want bool
	}{
		{"cs prefix + 12 hex", "cs_a1b2c3d4e5f6", true},
		{"cs prefix + 11 hex", "cs_a1b2c3d4e5f", false},
		{"cs prefix + 13 hex", "cs_a1b2c3d4e5f61", false},
		{"cs prefix uppercase hex", "cs_A1B2C3D4E5F6", false},
		{"cs prefix + non-hex", "cs_gggggggggggg", false},
		{"missing prefix", "a1b2c3d4e5f6", false},
		{"ctx prefix not cs", "ctx_a1b2c3d4e5f6", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := store.IsValidSessionID(tc.id)
			if got != tc.want {
				t.Fatalf("IsValidSessionID(%q) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}

func TestContextSummaryIDShapeValidation(t *testing.T) {
	if !store.IsValidContextSummaryID("ctx_deadbeef1234") {
		t.Fatalf("ctx_deadbeef1234 should validate")
	}
	if store.IsValidContextSummaryID("cs_deadbeef1234") {
		t.Fatalf("cs_ prefix must not satisfy ctx_ shape")
	}
}

// TestSessionRoundTrip covers save/load of a v1 manifest and the
// feature-mismatch refusal per PRD §7 D18.
func TestSessionRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	sess := store.Session{
		SchemaVersion: store.SessionSchemaVersion,
		ID:            "cs_abcdef012345",
		Feature:       "some-feature",
		State:         store.SessionActive,
		CaptureMode:   store.SessionCaptureSummary,
		BaseCommit:    "0123456789abcdef0123456789abcdef01234567",
	}
	if err := s.SaveSession(sess); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := s.LoadSession("some-feature", sess.ID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.ID != sess.ID || loaded.State != store.SessionActive {
		t.Fatalf("round trip mismatch: %+v", loaded)
	}
}

// TestLoadSessionCrossFeatureIsolation reproduces PRD §7 D18 — a
// session under `.tpatch/local/capture/<slug>/` MUST refuse to load
// when the manifest's feature disagrees with the directory feature.
// This is the write-time invariant that keeps feature A from reading
// feature B's buffer even if paths overlap.
func TestLoadSessionCrossFeatureIsolation(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	// Manually stitch a manifest whose Feature field disagrees with
	// the directory slug — mimics a corrupted / hand-edited local
	// buffer or a copy-paste attack across features.
	sess := store.Session{
		SchemaVersion: store.SessionSchemaVersion,
		ID:            "cs_abcdef012345",
		Feature:       "feature-b",
		State:         store.SessionActive,
		CaptureMode:   store.SessionCaptureSummary,
	}
	dir := filepath.Join(s.LocalCaptureDir(), "feature-a", sess.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	raw, _ := json.MarshalIndent(sess, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "session.json"), raw, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err = s.LoadSession("feature-a", sess.ID)
	if err == nil {
		t.Fatalf("expected cross-feature refusal, got nil error")
	}
	if !strings.Contains(err.Error(), "D18") {
		t.Fatalf("cross-feature refusal must cite PRD §7 D18; got: %v", err)
	}
}

func TestValidateSessionRejectsMalformed(t *testing.T) {
	cases := []struct {
		name string
		sess store.Session
	}{
		{"bad schema", store.Session{SchemaVersion: "vNever", ID: "cs_abcdef012345", Feature: "x", State: store.SessionActive, CaptureMode: store.SessionCaptureSummary}},
		{"bad id", store.Session{SchemaVersion: store.SessionSchemaVersion, ID: "not-a-cs-id", Feature: "x", State: store.SessionActive, CaptureMode: store.SessionCaptureSummary}},
		{"empty feature", store.Session{SchemaVersion: store.SessionSchemaVersion, ID: "cs_abcdef012345", Feature: "  ", State: store.SessionActive, CaptureMode: store.SessionCaptureSummary}},
		{"bad state", store.Session{SchemaVersion: store.SessionSchemaVersion, ID: "cs_abcdef012345", Feature: "x", State: store.SessionState("bogus"), CaptureMode: store.SessionCaptureSummary}},
		{"bad capture mode", store.Session{SchemaVersion: store.SessionSchemaVersion, ID: "cs_abcdef012345", Feature: "x", State: store.SessionActive, CaptureMode: store.SessionCaptureMode("raw")}},
		{"bad promoted ctx", store.Session{SchemaVersion: store.SessionSchemaVersion, ID: "cs_abcdef012345", Feature: "x", State: store.SessionPromoted, CaptureMode: store.SessionCaptureSummary, PromotedCtxID: "not-a-ctx-id"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.ValidateSession(tc.sess); err == nil {
				t.Fatalf("ValidateSession should reject %+v", tc.sess)
			}
		})
	}
}

// TestPurgeSessionIdempotent covers PRD §8.11 — purge on a missing
// directory returns nil.
func TestPurgeSessionIdempotent(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := s.PurgeSession("some-feature", "cs_abcdef012345"); err != nil {
		t.Fatalf("purge on missing session should be nil, got %v", err)
	}
}

func TestPurgeSessionRejectsBadID(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := s.PurgeSession("some-feature", "not-a-cs-id"); err == nil {
		t.Fatalf("purge with bad id should refuse")
	}
}

// TestPurgeSessionSymlinkRefusal covers PRD §8.19 — purge refuses paths
// that escape .tpatch/local/capture/ after symlink evaluation.
func TestPurgeSessionSymlinkRefusal(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	outside := t.TempDir()
	slugDir := filepath.Join(s.LocalCaptureDir(), "some-feature")
	if err := os.MkdirAll(slugDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	link := filepath.Join(slugDir, "cs_abcdef012345")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("cannot create symlink:", err)
	}
	err = s.PurgeSession("some-feature", "cs_abcdef012345")
	if err == nil {
		t.Fatalf("expected refusal on symlinked session directory")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("refusal message should mention symlink; got: %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside directory must survive; got: %v", err)
	}
}

func TestListSessionsSorted(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	sessions := []store.Session{
		{SchemaVersion: store.SessionSchemaVersion, ID: "cs_bbbbbbbbbbbb", Feature: "b-feature", State: store.SessionActive, CaptureMode: store.SessionCaptureSummary},
		{SchemaVersion: store.SessionSchemaVersion, ID: "cs_aaaaaaaaaaaa", Feature: "a-feature", State: store.SessionActive, CaptureMode: store.SessionCaptureSummary},
		{SchemaVersion: store.SessionSchemaVersion, ID: "cs_cccccccccccc", Feature: "a-feature", State: store.SessionActive, CaptureMode: store.SessionCaptureSummary},
	}
	for _, sess := range sessions {
		if err := s.SaveSession(sess); err != nil {
			t.Fatalf("save %s: %v", sess.ID, err)
		}
	}
	got, err := s.ListSessions("")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 sessions, got %d", len(got))
	}
	wantOrder := []struct{ slug, id string }{
		{"a-feature", "cs_aaaaaaaaaaaa"},
		{"a-feature", "cs_cccccccccccc"},
		{"b-feature", "cs_bbbbbbbbbbbb"},
	}
	for i, w := range wantOrder {
		if got[i].Slug != w.slug || got[i].SessionID != w.id {
			t.Fatalf("order[%d] = (%s,%s), want (%s,%s)", i, got[i].Slug, got[i].SessionID, w.slug, w.id)
		}
	}
	// Slug filter narrows.
	filtered, err := s.ListSessions("a-feature")
	if err != nil {
		t.Fatalf("list a: %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("want 2 filtered, got %d", len(filtered))
	}
}

// TestListSessionsMalformedIsolation covers PRD §8.16 — one bad
// manifest MUST NOT abort the listing of unrelated sessions.
func TestListSessionsMalformedIsolation(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	good := store.Session{
		SchemaVersion: store.SessionSchemaVersion,
		ID:            "cs_aaaaaaaaaaaa",
		Feature:       "a-feature",
		State:         store.SessionActive,
		CaptureMode:   store.SessionCaptureSummary,
	}
	if err := s.SaveSession(good); err != nil {
		t.Fatalf("save good: %v", err)
	}
	// Write a broken manifest in a valid-id directory.
	brokenDir := filepath.Join(s.LocalCaptureDir(), "b-feature", "cs_bbbbbbbbbbbb")
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatalf("mkdir broken: %v", err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, "session.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write broken: %v", err)
	}
	entries, err := s.ListSessions("")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	var seenGood, seenBad bool
	for _, e := range entries {
		if e.Session != nil && e.Session.ID == "cs_aaaaaaaaaaaa" {
			seenGood = true
		}
		if e.Err != nil && e.SessionID == "cs_bbbbbbbbbbbb" {
			seenBad = true
		}
	}
	if !seenGood || !seenBad {
		t.Fatalf("list must surface good=%v and bad=%v", seenGood, seenBad)
	}
}
