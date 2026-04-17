package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeGitHubServer mocks the three endpoints that CopilotLoginOptions
// addresses: device-code, access-token, session-token. The poll handler
// scripts a response sequence so tests can assert slow_down / pending
// handling without real timing dependencies.
type fakeGitHubServer struct {
	t            *testing.T
	deviceCodeFn func(r *http.Request) (int, string)
	pollScript   []string // each entry is a JSON body, consumed in order
	pollIdx      int
	sessionFn    func(r *http.Request) (int, string)

	acceptHeaders []string // captured from every incoming request
}

func (f *fakeGitHubServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/login/device/code", func(w http.ResponseWriter, r *http.Request) {
		f.acceptHeaders = append(f.acceptHeaders, r.Header.Get("Accept"))
		code, body := f.deviceCodeFn(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = io.WriteString(w, body)
	})
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		f.acceptHeaders = append(f.acceptHeaders, r.Header.Get("Accept"))
		if f.pollIdx >= len(f.pollScript) {
			http.Error(w, "script exhausted", http.StatusInternalServerError)
			return
		}
		body := f.pollScript[f.pollIdx]
		f.pollIdx++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	})
	mux.HandleFunc("/copilot_internal/v2/token", func(w http.ResponseWriter, r *http.Request) {
		f.acceptHeaders = append(f.acceptHeaders, r.Header.Get("Accept"))
		if got := r.Header.Get("Authorization"); got != "token gho_fake_oauth_token" {
			http.Error(w, "bad auth: "+got, http.StatusUnauthorized)
			return
		}
		code, body := f.sessionFn(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = io.WriteString(w, body)
	})
	return mux
}

// TestCopilotLogin_HappyPath drives the full device-code → access-token
// → session-token roundtrip against a local httptest server and
// asserts the returned CopilotAuth matches what the mock emitted.
func TestCopilotLogin_HappyPath(t *testing.T) {
	fake := &fakeGitHubServer{
		t: t,
		deviceCodeFn: func(r *http.Request) (int, string) {
			return http.StatusOK, `{
				"device_code": "dev-123",
				"user_code": "WXYZ-1234",
				"verification_uri": "https://github.com/login/device",
				"interval": 1,
				"expires_in": 900
			}`
		},
		pollScript: []string{
			// First poll: pending. Second: success.
			`{"error":"authorization_pending","error_description":"waiting"}`,
			`{"access_token":"gho_fake_oauth_token","token_type":"bearer","scope":"read:user"}`,
		},
		sessionFn: func(r *http.Request) (int, string) {
			return http.StatusOK, `{
				"token": "tid=abc;sess=xyz",
				"expires_at": 9999999999,
				"endpoints": {"api": "https://api.githubcopilot.com"}
			}`
		},
	}
	ts := httptest.NewServer(fake.handler())
	defer ts.Close()

	opts := CopilotLoginOptions{
		OverrideBaseURL: ts.URL,
		HTTPClient:      ts.Client(),
		Prompt:          io.Discard,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dev, err := RequestDeviceCode(ctx, opts)
	if err != nil {
		t.Fatalf("RequestDeviceCode: %v", err)
	}
	if dev.DeviceCode != "dev-123" || dev.UserCode != "WXYZ-1234" {
		t.Fatalf("unexpected device response: %+v", dev)
	}

	token, err := PollAccessToken(ctx, opts, dev)
	if err != nil {
		t.Fatalf("PollAccessToken: %v", err)
	}
	if token != "gho_fake_oauth_token" {
		t.Fatalf("wrong token: %q", token)
	}

	auth := &CopilotAuth{
		Version: 1,
		OAuth: CopilotOAuthBlock{
			AccessToken: token,
			ObtainedAt:  time.Now().UTC().Format(time.RFC3339),
		},
	}
	if err := ExchangeSessionToken(ctx, opts, auth); err != nil {
		t.Fatalf("ExchangeSessionToken: %v", err)
	}
	if auth.Session.Token != "tid=abc;sess=xyz" {
		t.Fatalf("session token not stored: %+v", auth.Session)
	}
	if auth.Session.Endpoints["api"] != "https://api.githubcopilot.com" {
		t.Fatalf("endpoints map not captured: %+v", auth.Session.Endpoints)
	}
	if auth.Session.ExpiresAt == "" {
		t.Fatal("ExpiresAt not set")
	}
	// Round-trip through disk to exercise SaveCopilotAuth / LoadCopilotAuth.
	tmp := t.TempDir()
	t.Setenv("TPATCH_COPILOT_AUTH_FILE", filepath.Join(tmp, "copilot-auth.json"))
	if err := SaveCopilotAuth(auth); err != nil {
		t.Fatalf("SaveCopilotAuth: %v", err)
	}
	loaded, err := LoadCopilotAuth()
	if err != nil {
		t.Fatalf("LoadCopilotAuth: %v", err)
	}
	if loaded.Session.Token != auth.Session.Token {
		t.Fatalf("roundtrip mismatch: %+v vs %+v", loaded.Session, auth.Session)
	}

	// Per rubber-duck #1: every call must advertise JSON.
	for i, h := range fake.acceptHeaders {
		if h != "application/json" {
			t.Fatalf("call %d sent Accept=%q, want application/json", i, h)
		}
	}
}

// TestCopilotLogin_SlowDownBump verifies that a slow_down response
// extends the poll interval without surfacing an error. We don't
// assert the exact timing — only that the second (successful) poll
// is reached after a slow_down without failing.
func TestCopilotLogin_SlowDownBump(t *testing.T) {
	fake := &fakeGitHubServer{
		t: t,
		deviceCodeFn: func(r *http.Request) (int, string) {
			return http.StatusOK, `{
				"device_code": "dev-slow",
				"user_code": "SLOW-0001",
				"verification_uri": "https://github.com/login/device",
				"interval": 1,
				"expires_in": 900
			}`
		},
		pollScript: []string{
			`{"error":"slow_down","error_description":"too fast"}`,
			`{"access_token":"gho_ok","token_type":"bearer"}`,
		},
	}
	ts := httptest.NewServer(fake.handler())
	defer ts.Close()

	opts := CopilotLoginOptions{
		OverrideBaseURL: ts.URL,
		HTTPClient:      ts.Client(),
		Prompt:          io.Discard,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dev, err := RequestDeviceCode(ctx, opts)
	if err != nil {
		t.Fatalf("RequestDeviceCode: %v", err)
	}
	token, err := PollAccessToken(ctx, opts, dev)
	if err != nil {
		t.Fatalf("PollAccessToken: %v", err)
	}
	if token != "gho_ok" {
		t.Fatalf("wrong token: %q", token)
	}
}

// TestCopilotLogin_AccessDenied verifies the terminal error path.
func TestCopilotLogin_AccessDenied(t *testing.T) {
	fake := &fakeGitHubServer{
		t: t,
		deviceCodeFn: func(r *http.Request) (int, string) {
			return http.StatusOK, `{"device_code":"d","user_code":"U","interval":1,"expires_in":60}`
		},
		pollScript: []string{
			`{"error":"access_denied","error_description":"user said no"}`,
		},
	}
	ts := httptest.NewServer(fake.handler())
	defer ts.Close()

	opts := CopilotLoginOptions{
		OverrideBaseURL: ts.URL,
		HTTPClient:      ts.Client(),
		Prompt:          io.Discard,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dev, err := RequestDeviceCode(ctx, opts)
	if err != nil {
		t.Fatalf("RequestDeviceCode: %v", err)
	}
	_, err = PollAccessToken(ctx, opts, dev)
	if err == nil {
		t.Fatal("expected access_denied error, got nil")
	}
	if !strings.Contains(err.Error(), "denied") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCopilotLogin_ContextCancel ensures Poll exits promptly when the
// parent context is cancelled, rather than spinning until device-code
// expiry.
func TestCopilotLogin_ContextCancel(t *testing.T) {
	fake := &fakeGitHubServer{
		t: t,
		deviceCodeFn: func(r *http.Request) (int, string) {
			return http.StatusOK, `{"device_code":"d","user_code":"U","interval":1,"expires_in":900}`
		},
		// Return pending forever; we rely on ctx cancel to exit.
		pollScript: func() []string {
			s := make([]string, 50)
			for i := range s {
				s[i] = `{"error":"authorization_pending"}`
			}
			return s
		}(),
	}
	ts := httptest.NewServer(fake.handler())
	defer ts.Close()

	opts := CopilotLoginOptions{
		OverrideBaseURL: ts.URL,
		HTTPClient:      ts.Client(),
		Prompt:          io.Discard,
	}
	ctx, cancel := context.WithCancel(context.Background())
	dev, err := RequestDeviceCode(ctx, opts)
	if err != nil {
		t.Fatalf("RequestDeviceCode: %v", err)
	}
	go func() {
		time.Sleep(1500 * time.Millisecond)
		cancel()
	}()
	_, err = PollAccessToken(ctx, opts, dev)
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
	if err != context.Canceled && !strings.Contains(err.Error(), "context") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestExchangeSessionToken_Unauthorized verifies we surface the typed
// errCopilotUnauthorized so the provider can decide to force re-login.
func TestExchangeSessionToken_Unauthorized(t *testing.T) {
	fake := &fakeGitHubServer{
		t: t,
		sessionFn: func(r *http.Request) (int, string) {
			// Bad token path is exercised by the 401 handler above,
			// but we want a 401 even when auth matches. Use a custom
			// handler instead.
			return http.StatusUnauthorized, `{"message":"bad"}`
		},
	}
	// Override the real handler to bypass the matching-token check.
	mux := http.NewServeMux()
	mux.HandleFunc("/copilot_internal/v2/token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"message":"bad"}`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	_ = fake

	opts := CopilotLoginOptions{
		OverrideBaseURL: ts.URL,
		HTTPClient:      ts.Client(),
	}
	auth := &CopilotAuth{OAuth: CopilotOAuthBlock{AccessToken: "gho_whatever"}}
	err := ExchangeSessionToken(context.Background(), opts, auth)
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if !IsCopilotAuthError(err) {
		t.Fatalf("expected errCopilotUnauthorized, got %T: %v", err, err)
	}
}

// TestCopilotAuth_SaveLoadRoundtrip exercises the auth-file safety
// rails: 0600 perms, atomic write, and the parent-dir perm check.
func TestCopilotAuth_SaveLoadRoundtrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TPATCH_COPILOT_AUTH_FILE", filepath.Join(tmp, "sub", "copilot-auth.json"))

	auth := &CopilotAuth{
		Version: 1,
		OAuth:   CopilotOAuthBlock{AccessToken: "gho_x", ObtainedAt: "2026-04-18T00:00:00Z"},
		Session: CopilotSessionBlock{
			Token:     "sess",
			ExpiresAt: time.Now().Add(25 * time.Minute).UTC().Format(time.RFC3339),
			Endpoints: map[string]string{"api": "https://api.githubcopilot.com"},
		},
	}
	if err := SaveCopilotAuth(auth); err != nil {
		t.Fatalf("SaveCopilotAuth: %v", err)
	}
	loaded, err := LoadCopilotAuth()
	if err != nil {
		t.Fatalf("LoadCopilotAuth: %v", err)
	}
	if loaded.OAuth.AccessToken != auth.OAuth.AccessToken {
		t.Fatalf("mismatch: %+v", loaded)
	}
	// Make sure JSON we wrote is the schema we promised (no extra nesting,
	// version at the top level).
	path, _ := CopilotAuthFilePath()
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(buf, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{"version", "oauth", "session"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("missing top-level key %q in %s", k, string(buf))
		}
	}

	if err := DeleteCopilotAuth(); err != nil {
		t.Fatalf("DeleteCopilotAuth: %v", err)
	}
	if _, err := LoadCopilotAuth(); err == nil {
		t.Fatal("expected load to fail after delete")
	}
}

// end of tests
