package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{
					{"id": "gpt-4o"},
					{"id": "claude-opus-4.6"},
				},
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	p := New()
	cfg := Config{BaseURL: srv.URL, Model: "gpt-4o"}
	health, err := p.Check(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if health.Endpoint != srv.URL {
		t.Errorf("endpoint = %q", health.Endpoint)
	}
	if len(health.Models) != 2 {
		t.Errorf("models count = %d, want 2", len(health.Models))
	}
}

func TestCheckFailure(t *testing.T) {
	p := New()
	cfg := Config{BaseURL: "http://localhost:1", Model: "test"}
	_, err := p.Check(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestGenerateSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{"message": map[string]string{"content": "Test response"}},
				},
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	p := New()
	cfg := Config{BaseURL: srv.URL, Model: "gpt-4o"}
	result, err := p.Generate(context.Background(), cfg, GenerateRequest{
		SystemPrompt: "You are a helpful assistant.",
		UserPrompt:   "Say hello",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(result, "Test response") {
		t.Errorf("result = %q, want 'Test response'", result)
	}
}

func TestGenerateWithAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "ok"}},
			},
		})
	}))
	defer srv.Close()

	t.Setenv("TEST_TOKEN", "my-secret-token")
	p := New()
	cfg := Config{BaseURL: srv.URL, Model: "gpt-4o", AuthEnv: "TEST_TOKEN"}
	_, err := p.Generate(context.Background(), cfg, GenerateRequest{UserPrompt: "test"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if gotAuth != "Bearer my-secret-token" {
		t.Errorf("auth = %q, want 'Bearer my-secret-token'", gotAuth)
	}
}
