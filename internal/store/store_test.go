package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Change button color", "change-button-color"},
		{"Add payment provider", "add-payment-provider"},
		{"fix model ID translation [1m] bug", "fix-model-id-translation-1m-bug"},
		{"  spaces everywhere  ", "spaces-everywhere"},
		{"UPPERCASE", "uppercase"},
		{"special!@#chars$%^here", "special-chars-here"},
		{"multiple---dashes", "multiple-dashes"},
		{"", ""},
		{"a", "a"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Slugify(tt.input)
			if got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSlugifyTruncation(t *testing.T) {
	long := "this-is-a-very-long-feature-name-that-exceeds-the-maximum-slug-length-limit-and-should-be-truncated"
	slug := Slugify(long)
	if len(slug) > maxSlugLen {
		t.Errorf("slug length %d exceeds max %d: %q", len(slug), maxSlugLen, slug)
	}
	if slug == "" {
		t.Error("long input should produce non-empty slug")
	}
}

func TestInitAndOpen(t *testing.T) {
	tmpDir := t.TempDir()

	// Init
	s, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if s.Root != tmpDir {
		t.Fatalf("Root = %q, want %q", s.Root, tmpDir)
	}

	// Verify files exist
	checkExists(t, filepath.Join(tmpDir, ".tpatch", "config.yaml"))
	checkExists(t, filepath.Join(tmpDir, ".tpatch", "FEATURES.md"))
	checkExists(t, filepath.Join(tmpDir, ".tpatch", "upstream.lock"))
	checkExists(t, filepath.Join(tmpDir, ".tpatch", "steering", "local.md"))
	checkExists(t, filepath.Join(tmpDir, ".tpatch", "steering", "upstream.md"))

	// Double init should fail
	_, err = Init(tmpDir)
	if err == nil {
		t.Fatal("double Init should fail")
	}

	// Open
	s2, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if s2.Root != tmpDir {
		t.Fatalf("Open Root = %q, want %q", s2.Root, tmpDir)
	}
}

func TestFindProjectRoot(t *testing.T) {
	tmpDir := t.TempDir()

	// Init at root
	_, err := Init(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create nested dir
	nested := filepath.Join(tmpDir, "a", "b", "c")
	os.MkdirAll(nested, 0o755)

	// Find from nested dir
	root, err := FindProjectRoot(nested)
	if err != nil {
		t.Fatalf("FindProjectRoot: %v", err)
	}
	if root != tmpDir {
		t.Fatalf("FindProjectRoot = %q, want %q", root, tmpDir)
	}
}

func TestAddFeature(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := Init(tmpDir)

	status, err := s.AddFeature(AddFeatureInput{
		Title:   "Fix model translation bug",
		Request: "The [1m] suffix gets stripped from Claude model IDs",
	})
	if err != nil {
		t.Fatalf("AddFeature: %v", err)
	}

	if status.Slug != "fix-model-translation-bug" {
		t.Fatalf("slug = %q", status.Slug)
	}
	if status.State != StateRequested {
		t.Fatalf("state = %q, want requested", status.State)
	}

	// Verify files
	checkExists(t, filepath.Join(tmpDir, ".tpatch", "features", "fix-model-translation-bug", "request.md"))
	checkExists(t, filepath.Join(tmpDir, ".tpatch", "features", "fix-model-translation-bug", "status.json"))
	checkExists(t, filepath.Join(tmpDir, ".tpatch", "features", "fix-model-translation-bug", "artifacts"))

	// Duplicate should fail
	_, err = s.AddFeature(AddFeatureInput{Title: "Fix model translation bug"})
	if err == nil {
		t.Fatal("duplicate add should fail")
	}
}

func TestListFeatures(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := Init(tmpDir)

	// Empty
	features, err := s.ListFeatures()
	if err != nil {
		t.Fatal(err)
	}
	if len(features) != 0 {
		t.Fatalf("expected 0 features, got %d", len(features))
	}

	// Add two
	s.AddFeature(AddFeatureInput{Title: "Feature B"})
	s.AddFeature(AddFeatureInput{Title: "Feature A"})

	features, err = s.ListFeatures()
	if err != nil {
		t.Fatal(err)
	}
	if len(features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(features))
	}
	// Should be sorted by slug
	if features[0].Slug != "feature-a" {
		t.Fatalf("first feature should be feature-a, got %q", features[0].Slug)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := Init(tmpDir)

	// Load default config
	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider.Type != "openai-compatible" {
		t.Fatalf("default type = %q, want openai-compatible", cfg.Provider.Type)
	}

	// Save updated config
	cfg.Provider.BaseURL = "http://localhost:4141"
	cfg.Provider.Model = "gpt-4o"
	cfg.Provider.AuthEnv = "GITHUB_TOKEN"
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// Reload
	cfg2, err := s.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.Provider.BaseURL != "http://localhost:4141" {
		t.Fatalf("base_url = %q", cfg2.Provider.BaseURL)
	}
	if cfg2.Provider.Model != "gpt-4o" {
		t.Fatalf("model = %q", cfg2.Provider.Model)
	}
	if cfg2.Provider.AuthEnv != "GITHUB_TOKEN" {
		t.Fatalf("auth_env = %q", cfg2.Provider.AuthEnv)
	}
}

func TestMarkFeatureState(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := Init(tmpDir)
	s.AddFeature(AddFeatureInput{Title: "Test feature"})

	// Mark analyzed
	if err := s.MarkFeatureState("test-feature", StateAnalyzed, "analyze", "analysis complete"); err != nil {
		t.Fatal(err)
	}

	status, err := s.LoadFeatureStatus("test-feature")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateAnalyzed {
		t.Fatalf("state = %q, want analyzed", status.State)
	}
}

// TestSaveFeatureStatusRefreshesIndex locks in the v0.5.0 fix:
// FEATURES.md must reflect every state transition, not just AddFeature.
// Previously the index only got rewritten in AddFeature, so a feature
// authored via Path B (add → apply --mode started → --mode done → record)
// left FEATURES.md stuck on "requested" while status.json said "applied".
func TestSaveFeatureStatusRefreshesIndex(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := Init(tmpDir)
	s.AddFeature(AddFeatureInput{Title: "Test feature"})

	indexPath := filepath.Join(tmpDir, ".tpatch", "FEATURES.md")
	initial, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(initial), "requested") {
		t.Fatalf("initial index missing requested state:\n%s", initial)
	}

	if err := s.MarkFeatureState("test-feature", StateApplied, "apply", ""); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "applied") {
		t.Fatalf("FEATURES.md not refreshed after MarkFeatureState; still:\n%s", after)
	}
	if strings.Contains(string(after), "requested") {
		t.Fatalf("FEATURES.md still shows stale state; got:\n%s", after)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func checkExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected %s to exist", path)
	}
}
