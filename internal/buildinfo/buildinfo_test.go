package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestResolvePrefersInjectedVersion(t *testing.T) {
	got := resolve("v1.2.3", func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9"}}, true
	})
	if got != "v1.2.3" {
		t.Fatalf("expected ldflags-injected version to win, got %q", got)
	}
}

func TestResolveUsesModuleVersionWhenInjectedIsDev(t *testing.T) {
	got := resolve("dev", func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v0.6.2"}}, true
	})
	if got != "v0.6.2" {
		t.Fatalf("expected BuildInfo fallback, got %q", got)
	}
}

func TestResolveKeepsDevForDevelBuild(t *testing.T) {
	got := resolve("dev", func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true
	})
	if got != "dev" {
		t.Fatalf("expected (devel) to be ignored, got %q", got)
	}
}

func TestResolveKeepsDevWithoutBuildInfo(t *testing.T) {
	got := resolve("dev", func() (*debug.BuildInfo, bool) {
		return nil, false
	})
	if got != "dev" {
		t.Fatalf("expected dev fallback when BuildInfo unavailable, got %q", got)
	}
}

func TestStringDoesNotPanic(t *testing.T) {
	// Smoke test: in the test binary, debug.ReadBuildInfo() returns a
	// real BuildInfo, so this should always produce a non-empty string.
	if got := String(); got == "" {
		t.Fatal("String() returned empty")
	}
}
