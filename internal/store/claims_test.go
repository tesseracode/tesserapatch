package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newClaimsStore(t *testing.T, slug string) *Store {
	t.Helper()
	root := t.TempDir()
	s, err := Init(root)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	if _, err := s.AddFeature(AddFeatureInput{Title: slug, Request: "claims-test", Slug: slug}); err != nil {
		t.Fatalf("AddFeature: %v", err)
	}
	return s
}

func TestComputeClaimIDIsDeterministicAnd12Hex(t *testing.T) {
	a := ComputeClaimID("feat", "path", "src/", "advisory")
	b := ComputeClaimID("feat", "path", "src/", "advisory")
	if a != b {
		t.Errorf("claim_id not deterministic: %q vs %q", a, b)
	}
	if len(a) != 12 {
		t.Errorf("claim_id length = %d, want 12 (got %q)", len(a), a)
	}
	for _, r := range a {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Errorf("claim_id %q contains non-hex rune %q", a, r)
		}
	}
	// Different inputs produce different IDs.
	if ComputeClaimID("feat", "path", "src/", "advisory") == ComputeClaimID("other", "path", "src/", "advisory") {
		t.Error("claim_id should differ when feature changes")
	}
	if ComputeClaimID("feat", "path", "src/", "advisory") == ComputeClaimID("feat", "path", "src", "advisory") {
		t.Error("claim_id should differ between 'src' and 'src/'")
	}
}

func TestNormalizeClaimPathHappyPaths(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "models"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"src/models":  "src/models/", // directory on disk → trailing slash forced
		"src/models/": "src/models/",
		"README.md":   "README.md",
		"./README.md": "README.md",
	}
	for in, want := range cases {
		got, err := NormalizeClaimPath(root, in)
		if err != nil {
			t.Errorf("NormalizeClaimPath(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeClaimPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeClaimPathRejects(t *testing.T) {
	root := t.TempDir()
	cases := map[string]string{
		"":                          "empty",
		"/etc/passwd":               "absolute",
		"../escape":                 "escape",
		".tpatch/secret":            "reserved",
		".tpatch/":                  "reserved",
		".claude/skills/foo":        "reserved",
		".github/skills/anything":   "reserved",
		".github/prompts/x.md":      "reserved",
		".cursor/rules/tessera.mdc": "reserved",
		".windsurfrules":            "reserved",
		".":                         "empty",
	}
	for in, why := range cases {
		if _, err := NormalizeClaimPath(root, in); err == nil {
			t.Errorf("NormalizeClaimPath(%q) should fail (%s)", in, why)
		}
	}
}

func TestValidateClaimKindAndModeInput(t *testing.T) {
	if err := ValidateClaimKindInput("path"); err != nil {
		t.Errorf("path should be accepted: %v", err)
	}
	for _, k := range []string{"glob", "symbol", "anchor"} {
		if err := ValidateClaimKindInput(k); err == nil {
			t.Errorf("kind %q should be rejected as reserved", k)
		}
	}
	if err := ValidateClaimKindInput("nonsense"); err == nil {
		t.Error("unknown kind should be rejected")
	}
	if err := ValidateClaimModeInput("advisory"); err != nil {
		t.Errorf("advisory should be accepted: %v", err)
	}
	if err := ValidateClaimModeInput("strict"); err == nil || !strings.Contains(err.Error(), "deferred") {
		t.Errorf("strict mode rejection should mention 'deferred', got %v", err)
	}
	if err := ValidateClaimModeInput("bogus"); err == nil {
		t.Error("unknown mode should be rejected")
	}
}

func TestSaveLoadClaimsRoundTripAndSortStability(t *testing.T) {
	s := newClaimsStore(t, "feat-x")
	m := ClaimsManifest{Version: ClaimsManifestVersion, Feature: "feat-x"}
	// Add in arbitrary order; SaveClaims must stable-sort by claim_id.
	for _, v := range []string{"src/c", "src/a", "src/b"} {
		AddClaim(&m, v)
	}
	if err := SaveClaims(s, m); err != nil {
		t.Fatalf("SaveClaims: %v", err)
	}
	got, err := LoadClaims(s, "feat-x")
	if err != nil {
		t.Fatalf("LoadClaims: %v", err)
	}
	if got.Version != ClaimsManifestVersion || got.Feature != "feat-x" {
		t.Errorf("unexpected header: %+v", got)
	}
	if len(got.Claims) != 3 {
		t.Fatalf("expected 3 claims, got %d", len(got.Claims))
	}
	for i := 1; i < len(got.Claims); i++ {
		if got.Claims[i-1].ClaimID > got.Claims[i].ClaimID {
			t.Errorf("claims not stable-sorted: %s > %s", got.Claims[i-1].ClaimID, got.Claims[i].ClaimID)
		}
	}
	// Saving again with the same claims (different input order) must
	// produce identical bytes on disk.
	first, _ := os.ReadFile(s.ClaimsPath("feat-x"))
	m2 := ClaimsManifest{Version: ClaimsManifestVersion, Feature: "feat-x"}
	for _, v := range []string{"src/b", "src/c", "src/a"} {
		AddClaim(&m2, v)
	}
	if err := SaveClaims(s, m2); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(s.ClaimsPath("feat-x"))
	if string(first) != string(second) {
		t.Errorf("manifest bytes differ between insert orders:\n--- first ---\n%s--- second ---\n%s", first, second)
	}
}

func TestSaveClaimsLeavesNoTmpFile(t *testing.T) {
	s := newClaimsStore(t, "feat-y")
	m := ClaimsManifest{Version: ClaimsManifestVersion, Feature: "feat-y"}
	AddClaim(&m, "src/")
	if err := SaveClaims(s, m); err != nil {
		t.Fatal(err)
	}
	tmp := s.ClaimsPath("feat-y") + ".tmp"
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("expected no .tmp file left behind, stat err = %v", err)
	}
	entries, _ := os.ReadDir(filepath.Dir(s.ClaimsPath("feat-y")))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("found stray tmp file: %s", e.Name())
		}
	}
}

func TestAddClaimIsIdempotent(t *testing.T) {
	m := ClaimsManifest{Version: ClaimsManifestVersion, Feature: "feat"}
	c1, added1 := AddClaim(&m, "src/")
	if !added1 {
		t.Fatal("first add should report added=true")
	}
	c2, added2 := AddClaim(&m, "src/")
	if added2 {
		t.Error("duplicate add should report added=false")
	}
	if c1.ClaimID != c2.ClaimID {
		t.Errorf("claim_id mismatch: %s vs %s", c1.ClaimID, c2.ClaimID)
	}
	if len(m.Claims) != 1 {
		t.Errorf("manifest should still have 1 claim, has %d", len(m.Claims))
	}
}

func TestMatchClaimByPrefixAndPath(t *testing.T) {
	m := ClaimsManifest{Version: ClaimsManifestVersion, Feature: "feat"}
	AddClaim(&m, "src/a/")
	AddClaim(&m, "src/b/")
	first := m.Claims[0]

	// Exact path
	got, ok, err := MatchClaim(&m, first.Value)
	if err != nil || !ok || got.ClaimID != first.ClaimID {
		t.Fatalf("path match failed: %+v ok=%v err=%v", got, ok, err)
	}

	// Full claim_id
	got, ok, err = MatchClaim(&m, first.ClaimID)
	if err != nil || !ok || got.ClaimID != first.ClaimID {
		t.Fatalf("full-id match failed: %+v ok=%v err=%v", got, ok, err)
	}

	// 7-char prefix that is unique to first
	prefix := first.ClaimID[:ClaimIDPrefixMin]
	// If the prefix is by accident shared with the other claim, skip the
	// uniqueness assertion and try a longer one.
	hits := 0
	for _, c := range m.Claims {
		if strings.HasPrefix(c.ClaimID, prefix) {
			hits++
		}
	}
	if hits == 1 {
		got, ok, err = MatchClaim(&m, prefix)
		if err != nil || !ok || got.ClaimID != first.ClaimID {
			t.Fatalf("prefix match failed: %+v ok=%v err=%v", got, ok, err)
		}
	}

	// Too-short prefix should not match by id.
	if _, ok, _ := MatchClaim(&m, "ab"); ok {
		t.Error("short prefix should not match")
	}

	// Missing path → not found, not error.
	if _, ok, err := MatchClaim(&m, "no/such/path"); err != nil || ok {
		t.Errorf("missing path should return ok=false nil err, got ok=%v err=%v", ok, err)
	}
}

func TestMatchClaimAmbiguousPrefix(t *testing.T) {
	// Force ambiguity by handcrafting two claims that share a prefix.
	m := ClaimsManifest{
		Version: ClaimsManifestVersion,
		Feature: "feat",
		Claims: []Claim{
			{ClaimID: "abcdef0123ab", Kind: "path", Value: "a", Mode: "advisory", Source: "manual"},
			{ClaimID: "abcdef0124cd", Kind: "path", Value: "b", Mode: "advisory", Source: "manual"},
		},
	}
	if _, ok, err := MatchClaim(&m, "abcdef0"); err == nil {
		t.Errorf("expected ambiguity error; ok=%v", ok)
	} else if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("error should mention 'ambiguous': %v", err)
	}
}

func TestLoadClaimsMissingFileIsEmpty(t *testing.T) {
	s := newClaimsStore(t, "feat-z")
	m, err := LoadClaims(s, "feat-z")
	if err != nil {
		t.Fatalf("LoadClaims on missing: %v", err)
	}
	if len(m.Claims) != 0 || m.Version != ClaimsManifestVersion || m.Feature != "feat-z" {
		t.Errorf("unexpected empty manifest: %+v", m)
	}
}

func TestLoadClaimsRejectsBadVersion(t *testing.T) {
	s := newClaimsStore(t, "feat-bad")
	bad := struct {
		Version int     `json:"version"`
		Feature string  `json:"feature"`
		Claims  []Claim `json:"claims"`
	}{Version: 99, Feature: "feat-bad"}
	data, _ := json.Marshal(bad)
	if err := os.MkdirAll(filepath.Dir(s.ClaimsPath("feat-bad")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.ClaimsPath("feat-bad"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadClaims(s, "feat-bad"); err == nil {
		t.Error("expected error on unsupported schema version")
	}
}

func TestLoadClaimsRejectsFeatureMismatch(t *testing.T) {
	s := newClaimsStore(t, "feat-mm")
	bad := ClaimsManifest{Version: ClaimsManifestVersion, Feature: "different-feature", Claims: []Claim{}}
	data, _ := json.Marshal(bad)
	os.MkdirAll(filepath.Dir(s.ClaimsPath("feat-mm")), 0o755)
	os.WriteFile(s.ClaimsPath("feat-mm"), data, 0o644)
	if _, err := LoadClaims(s, "feat-mm"); err == nil {
		t.Error("expected error on feature-slug mismatch")
	}
}
