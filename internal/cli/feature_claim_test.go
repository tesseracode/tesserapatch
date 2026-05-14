package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// claimFixture creates a tpatch repo with one feature, returns its
// absolute path and the feature slug. Tests use the --path flag rather
// than chdir-ing the process so they can run in parallel.
func claimFixture(t *testing.T, slug string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Init(dir)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: slug, Request: "claims-cli", Slug: slug}); err != nil {
		t.Fatalf("AddFeature: %v", err)
	}
	return dir, slug
}

// runClaim invokes the root command with --path <dir> prepended so
// tests don't rely on the process working directory.
func runClaim(t *testing.T, dir string, extraArgs ...string) (string, string, error) {
	t.Helper()
	var out, errBuf bytes.Buffer
	root := buildRootCmd()
	root.SetOut(&out)
	root.SetErr(&errBuf)
	args := append([]string{"--path", dir}, extraArgs...)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), errBuf.String(), err
}

func TestFeatureClaimAddWritesManifest(t *testing.T) {
	dir, slug := claimFixture(t, "model-picker")
	os.MkdirAll(filepath.Join(dir, "src", "models"), 0o755)
	os.WriteFile(filepath.Join(dir, "docs.md"), []byte("x"), 0o644)

	out, _, err := runClaim(t, dir, "feature", "claim", "add", slug, "src/models", "docs.md")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !strings.Contains(out, "added claim ") {
		t.Errorf("expected 'added claim' in stdout, got: %s", out)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".tpatch", "features", slug, "claims.json"))
	if err != nil {
		t.Fatalf("read claims.json: %v", err)
	}
	var m store.ClaimsManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Version != 1 || m.Feature != slug || len(m.Claims) != 2 {
		t.Fatalf("unexpected manifest: %+v", m)
	}
	// Validate every persisted claim is path/advisory/manual.
	for _, c := range m.Claims {
		if c.Kind != "path" || c.Mode != "advisory" || c.Source != "manual" {
			t.Errorf("claim has unexpected fields: %+v", c)
		}
		if len(c.ClaimID) != 12 {
			t.Errorf("claim_id length = %d, want 12", len(c.ClaimID))
		}
	}
	// The directory input must carry a trailing slash on disk.
	var found bool
	for _, c := range m.Claims {
		if c.Value == "src/models/" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected normalized 'src/models/' value in manifest, got: %+v", m.Claims)
	}
}

func TestFeatureClaimAddIdempotent(t *testing.T) {
	dir, slug := claimFixture(t, "feat")
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)

	if _, _, err := runClaim(t, dir, "feature", "claim", "add", slug, "src/"); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(dir, ".tpatch", "features", slug, "claims.json"))
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := runClaim(t, dir, "feature", "claim", "add", slug, "src/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "already claimed") {
		t.Errorf("expected 'already claimed' on duplicate add, got: %s", out)
	}
	second, _ := os.ReadFile(filepath.Join(dir, ".tpatch", "features", slug, "claims.json"))
	if string(first) != string(second) {
		t.Errorf("manifest changed on duplicate add\nfirst:%s\nsecond:%s", first, second)
	}
}

func TestFeatureClaimListHumanAndJSON(t *testing.T) {
	dir, slug := claimFixture(t, "feat")
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	os.WriteFile(filepath.Join(dir, "x.md"), []byte("x"), 0o644)

	// Empty case
	out, _, err := runClaim(t, dir, "feature", "claim", "list", slug)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "(none)") {
		t.Errorf("empty list should say (none): %s", out)
	}

	if _, _, err := runClaim(t, dir, "feature", "claim", "add", slug, "src/", "x.md"); err != nil {
		t.Fatal(err)
	}

	out, _, err = runClaim(t, dir, "feature", "claim", "list", slug)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Claims for "+slug) {
		t.Errorf("missing header: %s", out)
	}
	if !strings.Contains(out, "advisory  path") {
		t.Errorf("expected '<mode>  <kind>' columns, got: %s", out)
	}

	// JSON output stable across insertion orders.
	out1, _, err := runClaim(t, dir, "feature", "claim", "list", slug, "--json")
	if err != nil {
		t.Fatal(err)
	}

	// Rebuild manifest with a different add order on a second feature
	// and compare the per-claim_id ordering.
	dir2, slug2 := claimFixture(t, "feat")
	os.MkdirAll(filepath.Join(dir2, "src"), 0o755)
	os.WriteFile(filepath.Join(dir2, "x.md"), []byte("x"), 0o644)
	if _, _, err := runClaim(t, dir2, "feature", "claim", "add", slug2, "x.md", "src/"); err != nil {
		t.Fatal(err)
	}
	out2, _, err := runClaim(t, dir2, "feature", "claim", "list", slug2, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if out1 != out2 {
		t.Errorf("JSON output not order-stable:\n---a---\n%s---b---\n%s", out1, out2)
	}
}

func TestFeatureClaimRemoveByIDAndPath(t *testing.T) {
	dir, slug := claimFixture(t, "feat")
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	os.WriteFile(filepath.Join(dir, "y.md"), []byte("y"), 0o644)
	if _, _, err := runClaim(t, dir, "feature", "claim", "add", slug, "src/", "y.md"); err != nil {
		t.Fatal(err)
	}

	// Find the y.md claim_id.
	data, _ := os.ReadFile(filepath.Join(dir, ".tpatch", "features", slug, "claims.json"))
	var m store.ClaimsManifest
	json.Unmarshal(data, &m)
	var ymdID string
	for _, c := range m.Claims {
		if c.Value == "y.md" {
			ymdID = c.ClaimID
		}
	}
	if ymdID == "" {
		t.Fatal("could not find y.md claim_id")
	}

	// Remove by full id.
	out, _, err := runClaim(t, dir, "feature", "claim", "remove", slug, ymdID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "removed claim") {
		t.Errorf("expected 'removed claim', got: %s", out)
	}

	// Remove by path value.
	out, _, err = runClaim(t, dir, "feature", "claim", "remove", slug, "src/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "removed claim") {
		t.Errorf("expected 'removed claim' for path remove, got: %s", out)
	}

	// Manifest should now be empty (file still present).
	data2, err := os.ReadFile(filepath.Join(dir, ".tpatch", "features", slug, "claims.json"))
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(data2, &m)
	if len(m.Claims) != 0 {
		t.Errorf("expected empty claims, got %+v", m.Claims)
	}
}

func TestFeatureClaimRemoveMissingClaim(t *testing.T) {
	dir, slug := claimFixture(t, "feat")
	out, _, err := runClaim(t, dir, "feature", "claim", "remove", slug, "nope/path")
	if err != nil {
		t.Fatalf("missing-claim remove should not error: %v", err)
	}
	if !strings.Contains(out, "no such claim") {
		t.Errorf("expected 'no such claim' message, got: %s", out)
	}
}

func TestFeatureClaimClear(t *testing.T) {
	dir, slug := claimFixture(t, "feat")
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	if _, _, err := runClaim(t, dir, "feature", "claim", "add", slug, "src/"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runClaim(t, dir, "feature", "claim", "clear", slug); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".tpatch", "features", slug, "claims.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m store.ClaimsManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Claims) != 0 || m.Version != 1 || m.Feature != slug {
		t.Errorf("clear should leave version=1, feature=%s, claims=[]; got %+v", slug, m)
	}
}

func TestFeatureClaimRejectsBadPaths(t *testing.T) {
	dir, slug := claimFixture(t, "feat")
	cases := map[string][]string{
		"absolute":     {"/etc/passwd"},
		"escape":       {"../escape"},
		"tpatch":       {".tpatch/secret"},
		"skill-claude": {".claude/skills/foo"},
		"windsurf":     {".windsurfrules"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			full := append([]string{"feature", "claim", "add", slug}, args...)
			_, _, err := runClaim(t, dir, full...)
			if err == nil {
				t.Errorf("expected error for %s (%v)", name, args)
			}
		})
	}
}

func TestFeatureClaimUnknownSlugRejected(t *testing.T) {
	dir, _ := claimFixture(t, "feat")
	for _, verb := range []string{"add", "list", "remove", "clear"} {
		args := []string{"feature", "claim", verb, "does-not-exist"}
		if verb == "add" || verb == "remove" {
			args = append(args, "src/")
		}
		_, _, err := runClaim(t, dir, args...)
		if err == nil {
			t.Errorf("%s should reject unknown slug", verb)
			continue
		}
		if !strings.Contains(err.Error(), "no such feature") {
			t.Errorf("%s error should mention 'no such feature', got: %v", verb, err)
		}
	}
}

func TestFeatureClaimListJSONIsValidAndContainsHeader(t *testing.T) {
	dir, slug := claimFixture(t, "feat")
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	if _, _, err := runClaim(t, dir, "feature", "claim", "add", slug, "src/"); err != nil {
		t.Fatal(err)
	}
	out, _, err := runClaim(t, dir, "feature", "claim", "list", slug, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var m store.ClaimsManifest
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if m.Version != 1 || m.Feature != slug || len(m.Claims) != 1 {
		t.Errorf("unexpected JSON manifest: %+v", m)
	}
	// No timestamps anywhere in the JSON payload.
	for _, banned := range []string{"updated_at", "created_at", "timestamp"} {
		if strings.Contains(out, banned) {
			t.Errorf("claim JSON must not contain %q: %s", banned, out)
		}
	}
}

// TestFeatureClaim_RemoveByUnnormalizedDirectoryPath is the exact
// external-supervisor repro for rev-1 F1: `add src/models` (with
// `src/models/` existing as a real directory) stores `src/models/` and
// must round-trip through `remove src/models` (no trailing slash).
// This test MUST fail against commit dcd9bf0 (where MatchClaim did a
// literal compare of c.Value == arg) and pass against rev-1.
func TestFeatureClaim_RemoveByUnnormalizedDirectoryPath(t *testing.T) {
	dir, slug := claimFixture(t, "model-picker")
	if err := os.MkdirAll(filepath.Join(dir, "src", "models"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, _, err := runClaim(t, dir, "feature", "claim", "add", slug, "src/models"); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Sanity: the stored value carries the trailing slash because the
	// dir exists on disk. Without rev-1, removing by the unnormalized
	// form below would print "no such claim".
	data, err := os.ReadFile(filepath.Join(dir, ".tpatch", "features", slug, "claims.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m store.ClaimsManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(m.Claims) != 1 || m.Claims[0].Value != "src/models/" {
		t.Fatalf("setup precondition failed; expected one claim with value src/models/, got %+v", m.Claims)
	}

	out, _, err := runClaim(t, dir, "feature", "claim", "remove", slug, "src/models")
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !strings.Contains(out, "removed claim ") {
		t.Errorf("expected 'removed claim' in stdout; got: %q", out)
	}
	if strings.Contains(out, "no such claim") {
		t.Errorf("rev-1 F1 regression: remove printed 'no such claim'; got: %q", out)
	}

	data, err = os.ReadFile(filepath.Join(dir, ".tpatch", "features", slug, "claims.json"))
	if err != nil {
		t.Fatalf("re-read manifest: %v", err)
	}
	var after store.ClaimsManifest
	if err := json.Unmarshal(data, &after); err != nil {
		t.Fatalf("unmarshal after remove: %v", err)
	}
	if len(after.Claims) != 0 {
		t.Errorf("claim should be gone from manifest after remove, still have: %+v", after.Claims)
	}
}
