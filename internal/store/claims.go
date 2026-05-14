// Feature claim manifest (PRD-feature-file-claims v1).
//
// Claims are scope metadata declared per feature: "this feature
// expects to touch these paths." They are advisory-only in v1; they do
// not gate `record`, `land`, or any other lifecycle verb. The manifest
// is deterministic (stable-sorted by claim_id, no wall-clock
// timestamps) and atomic on write (.tmp + rename).
//
// The reserved kinds (`glob`, `symbol`, `anchor`) and reserved mode
// (`strict`) and reserved sources (`agent`, `imported`, `generated`)
// are NOT writable in v1: they exist in the schema so future PRDs can
// introduce them without redefining `claim_id` derivation.

package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/safety"
)

// ClaimsManifestVersion is the on-disk schema version this package
// reads and writes. Older or newer versions are rejected by LoadClaims.
const ClaimsManifestVersion = 1

// ClaimKind enumerates the values that may appear in Claim.Kind on disk.
// Only ClaimKindPath is accepted as input in v1; the rest are reserved
// schema values that LoadClaims will tolerate on read but AddClaim
// rejects on write.
type ClaimKind = string

const (
	ClaimKindPath   ClaimKind = "path"
	ClaimKindGlob   ClaimKind = "glob"   // reserved
	ClaimKindSymbol ClaimKind = "symbol" // reserved
	ClaimKindAnchor ClaimKind = "anchor" // reserved
)

// ClaimMode enumerates v1 + reserved modes. v1 writes only "advisory".
type ClaimMode = string

const (
	ClaimModeAdvisory ClaimMode = "advisory"
	ClaimModeStrict   ClaimMode = "strict" // reserved; rejected on input
)

// ClaimSource enumerates v1 + reserved sources. v1 writes only "manual".
type ClaimSource = string

const (
	ClaimSourceManual    ClaimSource = "manual"
	ClaimSourceAgent     ClaimSource = "agent"     // reserved
	ClaimSourceImported  ClaimSource = "imported"  // reserved
	ClaimSourceGenerated ClaimSource = "generated" // reserved
)

// Claim is one row of a feature claims manifest.
type Claim struct {
	ClaimID string `json:"claim_id"`
	Kind    string `json:"kind"`
	Value   string `json:"value"`
	Mode    string `json:"mode"`
	Source  string `json:"source"`
}

// ClaimsManifest is the full on-disk shape of claims.json.
type ClaimsManifest struct {
	Version int     `json:"version"`
	Feature string  `json:"feature"`
	Claims  []Claim `json:"claims"`
}

// ClaimsPath returns the absolute path of a feature's claims.json.
func (s *Store) ClaimsPath(slug string) string {
	return filepath.Join(s.featureDir(slug), "claims.json")
}

// FeatureExists reports whether the given slug has a status.json on
// disk. Used by the claim CLI surface to validate <slug> before any
// manifest read/write so we can emit a uniform "no such feature: X"
// error rather than a deep file-not-found from the manifest layer.
func (s *Store) FeatureExists(slug string) bool {
	return fileExists(s.featureStatusPath(slug))
}

// ComputeClaimID derives the deterministic 12-hex-char claim
// identifier from (feature, kind, normalized value, mode). The hash
// inputs are NUL-separated so no separator collision is possible. The
// 12-char prefix matches the example IDs in PRD-feature-file-claims §4.
func ComputeClaimID(feature, kind, value, mode string) string {
	h := sha256.New()
	h.Write([]byte(feature))
	h.Write([]byte{0})
	h.Write([]byte(kind))
	h.Write([]byte{0})
	h.Write([]byte(value))
	h.Write([]byte{0})
	h.Write([]byte(mode))
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// installedSkillPathPrefixes lists the repo-relative prefixes that the
// `init` skill installer writes into. We block claims on these so a
// user can't accidentally claim shipped skill assets (which are not
// product code). Keep this list in sync with installSkills() in
// internal/cli/cobra.go — there is no shared constant today because
// the installer writes to absolute paths.
var installedSkillPathPrefixes = []string{
	".tpatch/",
	".claude/skills/",
	".github/skills/",
	".github/prompts/",
	".cursor/rules/",
}

// installedSkillExactFiles lists single-file skill surfaces (no
// trailing slash) that should be rejected as exact matches.
var installedSkillExactFiles = []string{
	".windsurfrules",
}

// NormalizeClaimPath validates and canonicalizes a user-supplied path
// for a `path`-kind claim. Returns the cleaned, repo-relative,
// forward-slash form. Trailing-slash semantics: if the caller's input
// ended with "/" or the path resolves to a directory on disk, the
// returned form ends with "/". Empty / absolute / repo-escaping /
// .tpatch / skill-surface paths are rejected.
func NormalizeClaimPath(repoRoot, input string) (string, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", fmt.Errorf("claim path is empty")
	}
	if filepath.IsAbs(raw) {
		return "", fmt.Errorf("claim path %q must be repo-relative (no absolute paths)", input)
	}
	wantTrailingSlash := strings.HasSuffix(raw, "/") || strings.HasSuffix(raw, string(filepath.Separator))

	// Use filepath.Clean for normalization; reject ".." escapes.
	cleaned := filepath.Clean(raw)
	cleaned = filepath.ToSlash(cleaned)
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("claim path %q normalizes to empty", input)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("claim path %q escapes the repository root", input)
	}

	// Path-safety check (defense in depth): resolve against repoRoot
	// and confirm it stays inside. EnsureSafeRepoPath also catches
	// symlink-resolution surprises that Clean alone would miss.
	resolved := filepath.Join(repoRoot, cleaned)
	if err := safety.EnsureSafeRepoPath(repoRoot, resolved); err != nil {
		return "", fmt.Errorf("claim path %q: %w", input, err)
	}

	// If the path exists on disk and is a directory, force trailing slash.
	if info, err := os.Stat(resolved); err == nil && info.IsDir() {
		wantTrailingSlash = true
	}

	// Reject .tpatch/ and installed skill surfaces.
	probe := cleaned
	if wantTrailingSlash && !strings.HasSuffix(probe, "/") {
		probe += "/"
	}
	for _, exact := range installedSkillExactFiles {
		if cleaned == exact {
			return "", fmt.Errorf("claim path %q is an installed skill surface and cannot be claimed", input)
		}
	}
	for _, prefix := range installedSkillPathPrefixes {
		if probe == prefix || strings.HasPrefix(probe, prefix) {
			return "", fmt.Errorf("claim path %q is under a reserved area (%s) and cannot be claimed", input, prefix)
		}
	}

	if wantTrailingSlash && !strings.HasSuffix(cleaned, "/") {
		cleaned += "/"
	}
	return cleaned, nil
}

// ValidateClaimKindInput rejects reserved kinds at the input boundary.
// LoadClaims is more permissive (it tolerates unknown kinds so a future
// schema bump can be read by an older binary without losing data), but
// AddClaim only ever writes ClaimKindPath in v1.
func ValidateClaimKindInput(kind string) error {
	switch kind {
	case ClaimKindPath:
		return nil
	case ClaimKindGlob, ClaimKindSymbol, ClaimKindAnchor:
		return fmt.Errorf("claim kind %q is reserved for a future PRD and not accepted in v1", kind)
	default:
		return fmt.Errorf("claim kind %q is not recognized (v1 accepts only %q)", kind, ClaimKindPath)
	}
}

// ValidateClaimModeInput rejects reserved modes at the input boundary.
func ValidateClaimModeInput(mode string) error {
	switch mode {
	case ClaimModeAdvisory:
		return nil
	case ClaimModeStrict:
		return fmt.Errorf("strict mode is deferred; see PRD-feature-file-claims §3.4")
	default:
		return fmt.Errorf("claim mode %q is not recognized (v1 accepts only %q)", mode, ClaimModeAdvisory)
	}
}

// LoadClaims reads the claims manifest for a feature. A missing file
// is reported as an empty manifest (no error) so callers can treat
// "feature has no claims yet" as the natural empty state. The slug is
// NOT validated here — call FeatureExists first if you need that.
func LoadClaims(s *Store, slug string) (ClaimsManifest, error) {
	path := s.ClaimsPath(slug)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ClaimsManifest{Version: ClaimsManifestVersion, Feature: slug, Claims: []Claim{}}, nil
		}
		return ClaimsManifest{}, err
	}
	var m ClaimsManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return ClaimsManifest{}, fmt.Errorf("claims.json: %w", err)
	}
	if m.Version != ClaimsManifestVersion {
		return ClaimsManifest{}, fmt.Errorf("claims.json: unsupported schema version %d (expected %d)", m.Version, ClaimsManifestVersion)
	}
	if m.Feature != "" && m.Feature != slug {
		return ClaimsManifest{}, fmt.Errorf("claims.json: feature mismatch (file says %q, expected %q)", m.Feature, slug)
	}
	m.Feature = slug
	if m.Claims == nil {
		m.Claims = []Claim{}
	}
	sortClaims(m.Claims)
	return m, nil
}

// SaveClaims writes the manifest atomically: marshal → write to
// claims.json.tmp in the same directory → fsync the file → rename
// over claims.json. The rename is atomic on POSIX filesystems, which
// is what every supported tpatch host uses today.
func SaveClaims(s *Store, m ClaimsManifest) error {
	if m.Version == 0 {
		m.Version = ClaimsManifestVersion
	}
	if m.Version != ClaimsManifestVersion {
		return fmt.Errorf("claims.json: refusing to write unsupported schema version %d", m.Version)
	}
	if m.Feature == "" {
		return fmt.Errorf("claims.json: feature slug is required")
	}
	if m.Claims == nil {
		m.Claims = []Claim{}
	}
	sortClaims(m.Claims)

	target := s.ClaimsPath(m.Feature)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp := target + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// sortClaims orders claims by claim_id ascending. Stable so that
// identical claim_id collisions (impossible under SHA-256 truncation in
// practice but cheap insurance) keep their relative input order.
func sortClaims(cs []Claim) {
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].ClaimID < cs[j].ClaimID })
}

// AddClaim appends a new path-kind, advisory, manual claim to the
// manifest if one with the same (kind, value, mode) tuple isn't
// already present. Returns the resulting claim plus a boolean
// indicating whether the manifest actually changed. Idempotent.
func AddClaim(m *ClaimsManifest, normalizedValue string) (Claim, bool) {
	c := Claim{
		ClaimID: ComputeClaimID(m.Feature, ClaimKindPath, normalizedValue, ClaimModeAdvisory),
		Kind:    ClaimKindPath,
		Value:   normalizedValue,
		Mode:    ClaimModeAdvisory,
		Source:  ClaimSourceManual,
	}
	for _, existing := range m.Claims {
		if existing.ClaimID == c.ClaimID {
			return existing, false
		}
	}
	m.Claims = append(m.Claims, c)
	sortClaims(m.Claims)
	return c, true
}

// MatchClaim resolves a remove-argument to exactly one Claim in the
// manifest. The argument may be:
//   - a full claim_id (12 hex chars) — exact match,
//   - a claim_id prefix of >= ClaimIDPrefixMin chars — must be unambiguous,
//   - a normalized path value — exact match against Claim.Value.
//
// Returns (claim, true, nil) on a unique hit, (Claim{}, false, nil)
// when no claim matches (caller decides whether that's an error), and
// (Claim{}, false, err) on ambiguity.
const ClaimIDPrefixMin = 7

func MatchClaim(m *ClaimsManifest, arg string) (Claim, bool, error) {
	// Exact path-value match wins first so users can remove by the
	// same string they used to add.
	for _, c := range m.Claims {
		if c.Value == arg {
			return c, true, nil
		}
	}
	// claim_id (full or prefix). Only treat as a prefix when the arg
	// looks like a hex digest of plausible length; that keeps a path
	// like "src/a" from being scanned as a prefix.
	if isHexDigest(arg) && len(arg) >= ClaimIDPrefixMin {
		var hits []Claim
		for _, c := range m.Claims {
			if strings.HasPrefix(c.ClaimID, arg) {
				hits = append(hits, c)
			}
		}
		if len(hits) == 1 {
			return hits[0], true, nil
		}
		if len(hits) > 1 {
			ids := make([]string, len(hits))
			for i, h := range hits {
				ids[i] = h.ClaimID
			}
			return Claim{}, false, fmt.Errorf("ambiguous claim prefix %q matches %d claims: %s", arg, len(hits), strings.Join(ids, ", "))
		}
	}
	return Claim{}, false, nil
}

// RemoveClaim drops the claim with the matching claim_id from the
// manifest. Returns true if a claim was actually removed.
func RemoveClaim(m *ClaimsManifest, claimID string) bool {
	for i, c := range m.Claims {
		if c.ClaimID == claimID {
			m.Claims = append(m.Claims[:i], m.Claims[i+1:]...)
			return true
		}
	}
	return false
}

func isHexDigest(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}
