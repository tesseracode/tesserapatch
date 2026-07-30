package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/safety"
)

// SessionSchemaVersion is the schema version stamped into every session
// manifest. Bumping this string bumps every content-addressed cs_<12hex>
// derived from it — treat it as a compatibility break (ADR-027 D6 pattern).
const SessionSchemaVersion = "session/v1"

// SessionState is the local state machine defined in
// PRD-active-feature-session §3 D4.
type SessionState string

const (
	// SessionActive — same-feature local observations may be appended.
	SessionActive SessionState = "active"
	// SessionClosed — no more observations; may be summarized or purged.
	SessionClosed SessionState = "closed"
	// SessionPromoted — a redacted committed summary exists.
	SessionPromoted SessionState = "promoted"
	// SessionPurged — local buffer removed or tombstoned.
	SessionPurged SessionState = "purged"
)

// ValidSessionState reports whether s is one of the four defined states.
func ValidSessionState(s SessionState) bool {
	switch s {
	case SessionActive, SessionClosed, SessionPromoted, SessionPurged:
		return true
	default:
		return false
	}
}

// SessionCaptureMode enumerates the two v1 accepted `--capture-context`
// values per PRD §6 D13/D14. `off` is not stored — no session is created.
type SessionCaptureMode string

const (
	// SessionCaptureSummary is the default. Local buffer stores only
	// redacted short summaries; promotion is straightforward.
	SessionCaptureSummary SessionCaptureMode = "summary"
	// SessionCaptureLocalEvents allows richer local-only observations
	// (still no raw prompts, bodies, or IDE buffers per PRD §7 D16).
	SessionCaptureLocalEvents SessionCaptureMode = "local-events"
)

// ValidSessionCaptureMode reports whether m is one of the two v1 modes.
func ValidSessionCaptureMode(m SessionCaptureMode) bool {
	switch m {
	case SessionCaptureSummary, SessionCaptureLocalEvents:
		return true
	default:
		return false
	}
}

// Session is the persisted session manifest at
// .tpatch/local/capture/<slug>/<cs_id>/session.json.
//
// The `ID` field is the content-addressed cs_<12hex> derived from the
// identity fields per PRD §3 D3 (repo identity, feature slug, base
// commit at start, capture mode, schema version, workspace discriminator).
// Wall-clock timestamps are explicitly NOT identity inputs (ADR-027 D6).
type Session struct {
	SchemaVersion string             `json:"schema_version"`
	ID            string             `json:"id"`
	Feature       string             `json:"feature"`
	State         SessionState       `json:"state"`
	CaptureMode   SessionCaptureMode `json:"capture_mode"`
	BaseCommit    string             `json:"base_commit"`
	Label         string             `json:"label,omitempty"`
	// PromotedCtxID is the committed-summary ID once promotion succeeds
	// (D3 ctx_<12hex>). Empty until promotion.
	PromotedCtxID string `json:"promoted_ctx_id,omitempty"`
	// Observations is an ordered, append-scoped list of redacted-only
	// short observations. Kept intentionally small — the local raw
	// buffer archive is out of scope per PRD §7 D16.
	Observations []SessionObservation `json:"observations,omitempty"`
}

// SessionObservation is a single append entry stored in the manifest.
// PRD §7 D16 allowed content: redacted short summaries, symbolic
// references (no auto-dereference), operation/claim IDs, local sequence
// numbers. Forbidden: raw prompts, transcripts, tool bodies, IDE buffers,
// selections, secret values.
type SessionObservation struct {
	Seq     int    `json:"seq"`
	Summary string `json:"summary"`
	// SymbolicRef is opaque and does NOT auto-dereference per PRD D16.
	SymbolicRef string `json:"symbolic_ref,omitempty"`
}

// SessionIdentityInputs is the set of fields hashed to produce the
// content-addressed cs_<12hex> ID per PRD §3 D3.2. Excludes the ID
// itself, wall-clock timestamps, PIDs, adapter IDs, and local sequence
// numbers (D3.3).
type SessionIdentityInputs struct {
	SchemaVersion          string `json:"schema_version"`
	RepositoryIdentity     string `json:"repository_identity"`
	Feature                string `json:"feature"`
	BaseCommit             string `json:"base_commit"`
	CaptureMode            string `json:"capture_mode"`
	WorkspaceDiscriminator string `json:"workspace_discriminator"`
}

// ComputeSessionID returns the `cs_<12hex>` ID for the given identity
// inputs. The 12 lowercase hex characters are the prefix of
// sha256(canonical-json(inputs)). Deterministic — same inputs always
// yield the same ID (D3.1).
func ComputeSessionID(in SessionIdentityInputs) string {
	// Canonical JSON: json.Marshal already produces stable field order
	// for a struct with fixed field ordering. We deliberately do NOT
	// sort or normalize further — a struct is the identity contract.
	raw, err := json.Marshal(in)
	if err != nil {
		// Struct with only string fields never fails to marshal.
		return "cs_" + strings.Repeat("0", 12)
	}
	sum := sha256.Sum256(raw)
	return "cs_" + hex.EncodeToString(sum[:])[:12]
}

// SessionIDPrefix is the reserved cs_<12hex> prefix per ADR-027 D6 and
// PRD §3 D3.
const SessionIDPrefix = "cs_"

// ContextSummaryIDPrefix is the reserved ctx_<12hex> committed-summary
// prefix per ADR-027 D6 and PRD §3 D3.
const ContextSummaryIDPrefix = "ctx_"

// IsValidSessionID reports whether s matches the `cs_` + 12 lowercase
// hex characters shape.
func IsValidSessionID(s string) bool {
	return isPrefixedHex12(s, SessionIDPrefix)
}

// IsValidContextSummaryID reports whether s matches the `ctx_` + 12
// lowercase hex characters shape.
func IsValidContextSummaryID(s string) bool {
	return isPrefixedHex12(s, ContextSummaryIDPrefix)
}

func isPrefixedHex12(s, prefix string) bool {
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	rest := s[len(prefix):]
	if len(rest) != 12 {
		return false
	}
	for _, r := range rest {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

// LocalCaptureDir returns the absolute path to `.tpatch/local/capture/`
// under this store. This is the D5-locked path.
func (s *Store) LocalCaptureDir() string {
	return filepath.Join(s.tpatchDir(), "local", "capture")
}

// LocalDir returns the absolute path to `.tpatch/local/` — the root of
// the private buffer lane that must be gitignore-covered before writes.
func (s *Store) LocalDir() string {
	return filepath.Join(s.tpatchDir(), "local")
}

// SessionDir returns the absolute path to the session directory for the
// given feature slug and cs_<12hex> ID. Callers MUST verify the returned
// path stays inside LocalCaptureDir via safety.EnsureSafeRepoPath before
// writing (PRD §8.19 + ADR-027 D9 cross-feature isolation).
func (s *Store) SessionDir(slug, sessionID string) string {
	return filepath.Join(s.LocalCaptureDir(), slug, sessionID)
}

// SessionManifestPath returns the `session.json` path for the given
// slug and cs_<12hex> ID.
func (s *Store) SessionManifestPath(slug, sessionID string) string {
	return filepath.Join(s.SessionDir(slug, sessionID), "session.json")
}

// FeatureContextDir returns the committed-summary directory for a
// feature per PRD §5 D10.
func (s *Store) FeatureContextDir(slug string) string {
	return filepath.Join(s.featureArtifactsDir(slug), "context")
}

// FeatureContextSummaryPath returns the committed-summary JSON path
// for the given feature slug and ctx_<12hex> ID.
func (s *Store) FeatureContextSummaryPath(slug, ctxID string) string {
	return filepath.Join(s.FeatureContextDir(slug), ctxID+".json")
}

// SaveSession writes the manifest to the on-disk location for its
// (feature, ID) coordinates. The caller MUST have already verified
// the ignore contract via IgnoreCheck (D6). The manifest is validated
// via ValidateSession before write.
func (s *Store) SaveSession(sess Session) error {
	if err := ValidateSession(sess); err != nil {
		return err
	}
	dir := s.SessionDir(sess.Feature, sess.ID)
	// PRD §8.19 + D18: refuse paths that escape .tpatch/local/capture/.
	if err := safety.EnsureSafeRepoPath(s.LocalCaptureDir(), dir); err != nil {
		return fmt.Errorf("session directory escapes local capture root: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.SessionManifestPath(sess.Feature, sess.ID), append(data, '\n'), 0o644)
}

// LoadSession reads and validates a session manifest.
func (s *Store) LoadSession(slug, sessionID string) (Session, error) {
	path := s.SessionManifestPath(slug, sessionID)
	raw, err := os.ReadFile(path)
	if err != nil {
		return Session{}, err
	}
	var sess Session
	if err := json.Unmarshal(raw, &sess); err != nil {
		return Session{}, fmt.Errorf("session %s/%s: parse: %w", slug, sessionID, err)
	}
	if err := ValidateSession(sess); err != nil {
		return Session{}, fmt.Errorf("session %s/%s: %w", slug, sessionID, err)
	}
	// PRD §7 D18 cross-feature isolation: refuse if manifest feature
	// mismatches directory feature.
	if sess.Feature != slug {
		return Session{}, fmt.Errorf("session %s/%s: manifest feature %q disagrees with directory %q (PRD §7 D18)", slug, sessionID, sess.Feature, slug)
	}
	return sess, nil
}

// ValidateSession checks the invariants a session manifest must satisfy
// on the wire. Kept internal to store so callers key off a single
// sentinel for downstream error routing.
func ValidateSession(sess Session) error {
	if sess.SchemaVersion != SessionSchemaVersion {
		return fmt.Errorf("%w: schema %q not %q", ErrSessionMalformed, sess.SchemaVersion, SessionSchemaVersion)
	}
	if !IsValidSessionID(sess.ID) {
		return fmt.Errorf("%w: id %q not cs_<12hex>", ErrSessionMalformed, sess.ID)
	}
	if strings.TrimSpace(sess.Feature) == "" {
		return fmt.Errorf("%w: feature slug empty", ErrSessionMalformed)
	}
	if !ValidSessionState(sess.State) {
		return fmt.Errorf("%w: state %q not one of active|closed|promoted|purged", ErrSessionMalformed, sess.State)
	}
	if !ValidSessionCaptureMode(sess.CaptureMode) {
		return fmt.Errorf("%w: capture mode %q not one of summary|local-events", ErrSessionMalformed, sess.CaptureMode)
	}
	if sess.PromotedCtxID != "" && !IsValidContextSummaryID(sess.PromotedCtxID) {
		return fmt.Errorf("%w: promoted_ctx_id %q not ctx_<12hex>", ErrSessionMalformed, sess.PromotedCtxID)
	}
	return nil
}

// ErrSessionMalformed is returned by ValidateSession / LoadSession when
// a manifest fails wire validation. Read-only callers may still list
// the entry with warnings; writers MUST refuse per PRD §8.17.
var ErrSessionMalformed = errors.New("session manifest malformed")

// SessionEntry pairs a discovered session with either the loaded
// manifest or the load error, mirroring the FeatureEntry pattern for
// `session list`.
type SessionEntry struct {
	Slug      string
	SessionID string
	Session   *Session
	Err       error
}

// ListSessions returns every session under `.tpatch/local/capture/`
// for the given slug filter. If slug == "", every slug's sessions are
// returned. Sorted lexicographically by (slug, sessionID) for
// deterministic output (PRD §8.15 + §8.16).
//
// Per PRD §8.16, per-session parse/redaction errors do NOT abort
// listing unrelated sessions — malformed manifests surface as entries
// with Err set.
func (s *Store) ListSessions(slugFilter string) ([]SessionEntry, error) {
	root := s.LocalCaptureDir()
	rootEntries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []SessionEntry
	for _, slugEntry := range rootEntries {
		if !slugEntry.IsDir() {
			continue
		}
		slug := slugEntry.Name()
		if slugFilter != "" && slug != slugFilter {
			continue
		}
		slugDir := filepath.Join(root, slug)
		sessEntries, err := os.ReadDir(slugDir)
		if err != nil {
			out = append(out, SessionEntry{Slug: slug, Err: err})
			continue
		}
		for _, se := range sessEntries {
			if !se.IsDir() {
				continue
			}
			sessionID := se.Name()
			if !IsValidSessionID(sessionID) {
				continue
			}
			sess, lerr := s.LoadSession(slug, sessionID)
			if lerr != nil {
				out = append(out, SessionEntry{Slug: slug, SessionID: sessionID, Err: lerr})
				continue
			}
			cp := sess
			out = append(out, SessionEntry{Slug: slug, SessionID: sessionID, Session: &cp})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Slug != out[j].Slug {
			return out[i].Slug < out[j].Slug
		}
		return out[i].SessionID < out[j].SessionID
	})
	return out, nil
}

// PurgeSession removes the local session directory for (slug, sessionID)
// after verifying the resolved path stays inside .tpatch/local/capture/.
// Idempotent per PRD §8.11 — a missing directory returns nil.
func (s *Store) PurgeSession(slug, sessionID string) error {
	if !IsValidSessionID(sessionID) {
		return fmt.Errorf("purge: %q is not a cs_<12hex> id", sessionID)
	}
	dir := s.SessionDir(slug, sessionID)
	if err := safety.EnsureSafeRepoPath(s.LocalCaptureDir(), dir); err != nil {
		return fmt.Errorf("purge refuses unsafe path: %w", err)
	}
	// Follow symlinks: PRD §8.19 requires purge to refuse paths that
	// escape after symlink evaluation.
	if info, err := os.Lstat(dir); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("purge refuses symlinked session directory %q (PRD §8.19)", dir)
	}
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	// Evaluate symlinks on the resolved path — a nested symlink escape
	// is caught here.
	resolved, err := filepath.EvalSymlinks(dir)
	if err == nil {
		if err := safety.EnsureSafeRepoPath(s.LocalCaptureDir(), resolved); err != nil {
			return fmt.Errorf("purge refuses symlink-escape path: %w", err)
		}
	}
	return os.RemoveAll(dir)
}

// LoadContextSummary reads a committed context summary JSON.
func (s *Store) LoadContextSummary(slug, ctxID string) (ContextSummary, error) {
	path := s.FeatureContextSummaryPath(slug, ctxID)
	raw, err := os.ReadFile(path)
	if err != nil {
		return ContextSummary{}, err
	}
	var cs ContextSummary
	if err := json.Unmarshal(raw, &cs); err != nil {
		return ContextSummary{}, fmt.Errorf("context summary %s/%s: parse: %w", slug, ctxID, err)
	}
	return cs, nil
}

// SaveContextSummary writes a committed context summary JSON under
// .tpatch/features/<slug>/artifacts/context/<ctx_id>.json.
//
// Path safety is enforced against featureArtifactsDir(slug). Callers
// MUST have already run the D11 redaction contract; this method does
// not re-run redaction. Determinism: JSON is marshalled with sorted
// keys via the struct definition order.
func (s *Store) SaveContextSummary(cs ContextSummary) error {
	if !IsValidContextSummaryID(cs.ID) {
		return fmt.Errorf("context summary id %q not ctx_<12hex>", cs.ID)
	}
	if strings.TrimSpace(cs.Feature) == "" {
		return fmt.Errorf("context summary feature slug empty")
	}
	dir := s.FeatureContextDir(cs.Feature)
	if err := safety.EnsureSafeRepoPath(s.featureArtifactsDir(cs.Feature), dir); err != nil {
		return fmt.Errorf("context summary directory escapes feature artifacts: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.FeatureContextSummaryPath(cs.Feature, cs.ID), append(data, '\n'), 0o644)
}

// ContextSummary is the committed-summary JSON shape per PRD §5 D10.
// The broader schema is deferred to PRD-record-context-summary; v1
// commits only the minimum fields required for identity + audit.
type ContextSummary struct {
	SchemaVersion string `json:"schema_version"`
	ID            string `json:"id"`
	Feature       string `json:"feature"`
	SessionID     string `json:"session_id"`
	CaptureMode   string `json:"capture_mode"`
	Summary       string `json:"summary"`
	// Redaction is the machine-readable proof the D11 contract ran.
	Redaction ContextSummaryRedaction `json:"redaction"`
}

// ContextSummaryRedaction captures the D11 redaction contract's
// audit fields. Individual findings are enumerated as reason codes,
// not raw text.
type ContextSummaryRedaction struct {
	Status         string   `json:"status"` // "clean" | "scrubbed"
	FindingCodes   []string `json:"finding_codes,omitempty"`
	ScrubbedFields []string `json:"scrubbed_fields,omitempty"`
}

// ContextSummaryIDInputs is the identity input struct for ctx_<12hex>
// content addressing (PRD D3). Wall-clock is excluded.
type ContextSummaryIDInputs struct {
	SchemaVersion string `json:"schema_version"`
	Feature       string `json:"feature"`
	SessionID     string `json:"session_id"`
	CaptureMode   string `json:"capture_mode"`
	SummaryHash   string `json:"summary_hash"`
}

// ComputeContextSummaryID returns the `ctx_<12hex>` ID for the given
// inputs.
func ComputeContextSummaryID(in ContextSummaryIDInputs) string {
	raw, err := json.Marshal(in)
	if err != nil {
		return "ctx_" + strings.Repeat("0", 12)
	}
	sum := sha256.Sum256(raw)
	return "ctx_" + hex.EncodeToString(sum[:])[:12]
}

// HashSummaryBody returns the sha256 hex digest of a summary body — used
// as one of the identity inputs for the ctx_<12hex> ID (D3).
func HashSummaryBody(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}
