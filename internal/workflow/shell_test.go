package workflow

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestUserShellForUnix(t *testing.T) {
	name, flag := userShellFor("linux")
	if name != "sh" || flag != "-c" {
		t.Fatalf("linux: got (%q, %q), want (sh, -c)", name, flag)
	}
	name, flag = userShellFor("darwin")
	if name != "sh" || flag != "-c" {
		t.Fatalf("darwin: got (%q, %q), want (sh, -c)", name, flag)
	}
}

func TestUserShellForWindows(t *testing.T) {
	name, flag := userShellFor("windows")
	if name != "cmd" || flag != "/C" {
		t.Fatalf("windows: got (%q, %q), want (cmd, /C)", name, flag)
	}
}

func TestUserShellMatchesRuntime(t *testing.T) {
	name, flag := UserShell()
	wantName, wantFlag := userShellFor(runtime.GOOS)
	if name != wantName || flag != wantFlag {
		t.Fatalf("UserShell() = (%q, %q); want (%q, %q)", name, flag, wantName, wantFlag)
	}
}

// Smoke: on Unix the helper actually invokes `sh -c` and the command runs.
// Skipped on Windows runners because /bin/sh is absent there.
func TestUserShellSmokeRunsOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no POSIX shell on Windows")
	}
	name, flag := UserShell()
	out, err := exec.Command(name, flag, "echo tpatch-shell-smoke").Output()
	if err != nil {
		t.Fatalf("exec %s %s: %v", name, flag, err)
	}
	if !strings.Contains(string(out), "tpatch-shell-smoke") {
		t.Fatalf("unexpected output: %q", out)
	}
}
