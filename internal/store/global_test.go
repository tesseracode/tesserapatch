package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withXDG(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

func TestGlobalConfigRoundtrip(t *testing.T) {
	withXDG(t)
	cfg := Config{}
	cfg.Provider.Type = "openai-compatible"
	cfg.Provider.BaseURL = "http://localhost:4141"
	cfg.Provider.Model = "gpt-4o"
	cfg.Provider.AuthEnv = "COPILOT_API_KEY"
	cfg.MergeStrategy = "3way"
	cfg.MaxRetries = 2
	cfg.CopilotAUPAckAt = "2026-04-17T10:00:00Z"
	if err := SaveGlobalConfig(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadGlobalConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Provider.BaseURL != cfg.Provider.BaseURL {
		t.Errorf("baseurl: got %q want %q", got.Provider.BaseURL, cfg.Provider.BaseURL)
	}
	if got.CopilotAUPAckAt != cfg.CopilotAUPAckAt {
		t.Errorf("aup: got %q want %q", got.CopilotAUPAckAt, cfg.CopilotAUPAckAt)
	}
}

func TestLoadGlobalConfigMissing(t *testing.T) {
	withXDG(t)
	cfg, err := LoadGlobalConfig()
	if err != nil {
		t.Fatalf("missing should not error: %v", err)
	}
	if cfg.Provider.BaseURL != "" {
		t.Errorf("expected zero cfg, got %+v", cfg)
	}
}

func TestCopilotAUPAcknowledge(t *testing.T) {
	withXDG(t)
	if CopilotAUPAcknowledged() {
		t.Fatal("fresh config must not be acknowledged")
	}
	if err := AcknowledgeCopilotAUP(); err != nil {
		t.Fatalf("ack: %v", err)
	}
	if !CopilotAUPAcknowledged() {
		t.Fatal("expected ack after AcknowledgeCopilotAUP")
	}
	// Idempotent — second call must not change timestamp.
	before, _ := LoadGlobalConfig()
	if err := AcknowledgeCopilotAUP(); err != nil {
		t.Fatalf("second ack: %v", err)
	}
	after, _ := LoadGlobalConfig()
	if before.CopilotAUPAckAt != after.CopilotAUPAckAt {
		t.Errorf("ack must be idempotent: before=%q after=%q",
			before.CopilotAUPAckAt, after.CopilotAUPAckAt)
	}
}

func TestLoadMergedConfigPrecedence(t *testing.T) {
	dir := withXDG(t)
	// Global says openai, gpt-4o, retries=5.
	global := Config{}
	global.Provider.Type = "openai-compatible"
	global.Provider.BaseURL = "https://api.openai.com/v1"
	global.Provider.Model = "gpt-4o"
	global.Provider.AuthEnv = "OPENAI_API_KEY"
	global.MaxRetries = 5
	global.MergeStrategy = "3way"
	if err := SaveGlobalConfig(global); err != nil {
		t.Fatalf("save global: %v", err)
	}

	// Verify file landed in the XDG dir (sanity).
	if _, err := os.Stat(filepath.Join(dir, "tpatch", "config.yaml")); err != nil {
		t.Fatalf("global file not created: %v", err)
	}

	root := t.TempDir()
	s, err := Init(root)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	// Repo overrides base URL + model (points at local copilot-api proxy).
	repo := Config{}
	repo.Provider.BaseURL = "http://localhost:4141"
	repo.Provider.Model = "claude-sonnet-4"
	if err := s.SaveConfig(repo); err != nil {
		t.Fatalf("save repo: %v", err)
	}
	merged, err := s.LoadMergedConfig()
	if err != nil {
		t.Fatalf("merged: %v", err)
	}
	if merged.Provider.BaseURL != "http://localhost:4141" {
		t.Errorf("baseurl should be repo override: %q", merged.Provider.BaseURL)
	}
	if merged.Provider.Model != "claude-sonnet-4" {
		t.Errorf("model should be repo override: %q", merged.Provider.Model)
	}
	// Fields the repo left blank must come from global.
	if merged.Provider.Type != "openai-compatible" {
		t.Errorf("type should inherit from global: %q", merged.Provider.Type)
	}
	if merged.Provider.AuthEnv != "OPENAI_API_KEY" {
		t.Errorf("auth env should inherit from global: %q", merged.Provider.AuthEnv)
	}
	if merged.MaxRetries != 5 {
		t.Errorf("max retries should inherit from global (got %d)", merged.MaxRetries)
	}
}

func TestMergeConfigDoesNotClearWithZero(t *testing.T) {
	global := Config{}
	global.Provider.BaseURL = "https://api.openai.com/v1"
	global.MaxRetries = 3
	merged := mergeConfig(global, Config{})
	if merged.Provider.BaseURL != global.Provider.BaseURL {
		t.Errorf("empty repo must not clear global BaseURL: %q", merged.Provider.BaseURL)
	}
	if merged.MaxRetries != 3 {
		t.Errorf("empty repo must not clear global MaxRetries (got %d)", merged.MaxRetries)
	}
}

func TestSaveGlobalConfigCreatesDir(t *testing.T) {
	dir := withXDG(t)
	cfg := Config{}
	cfg.Provider.Type = "openai-compatible"
	cfg.Provider.BaseURL = "http://localhost:4141"
	if err := SaveGlobalConfig(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	path := filepath.Join(dir, "tpatch", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "localhost:4141") {
		t.Errorf("expected base url in file, got: %s", string(data))
	}
}
