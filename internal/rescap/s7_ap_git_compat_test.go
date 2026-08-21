//go:build (linux && !android) || (darwin && !ios)

package rescap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestS7APCompatibilityWrappersRuntime(t *testing.T) {
	t.Run("PIB-473", func(t *testing.T) {
		repo := t.TempDir()
		bin := t.TempDir()
		logPath := filepath.Join(t.TempDir(), "git.log")
		script := `#!/bin/sh
printf '%s|%s|%s|%s|%s\n' "$*" "$S7_AP_COMPAT" "$GIT_CONFIG_GLOBAL" "$GIT_CONFIG_SYSTEM" "$HOME" >> "$S7_AP_COMPAT_LOG"
case "$*" in
  "check-ignore -q --no-index -- lane") exit 0 ;;
  "--literal-pathspecs ls-files --error-unmatch -- tracked.txt") exit 0 ;;
  "--literal-pathspecs ls-files -- .tpatch/local/") printf '.tpatch/local/entry\n'; exit 0 ;;
  "status --short") printf 'compat-run\n'; exit 0 ;;
esac
printf 'unexpected argv: %s\n' "$*" >&2
exit 88
`
		if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin)
		t.Setenv("S7_AP_COMPAT_LOG", logPath)
		t.Setenv("S7_AP_COMPAT", "preserved-sentinel")
		t.Setenv("GIT_CONFIG_GLOBAL", "/compat/global")
		t.Setenv("GIT_CONFIG_SYSTEM", "/compat/system")
		t.Setenv("HOME", "/compat/home")

		if ignored, err := IsIgnored(repo, "lane"); err != nil || !ignored {
			t.Fatalf("compat IsIgnored = %t %v", ignored, err)
		}
		if tracked, err := IsTracked(repo, "tracked.txt"); err != nil || !tracked {
			t.Fatalf("compat IsTracked = %t %v", tracked, err)
		}
		if tracked, err := AnythingTrackedUnder(repo, ".tpatch/local/"); err != nil || !tracked {
			t.Fatalf("compat AnythingTrackedUnder = %t %v", tracked, err)
		}
		if output, err := RunGit(repo, "status", "--short"); err != nil || output != "compat-run\n" {
			t.Fatalf("compat RunGit = %q %v", output, err)
		}

		raw, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		got := strings.Split(strings.TrimSpace(string(raw)), "\n")
		want := []string{
			"check-ignore -q --no-index -- lane|preserved-sentinel|/compat/global|/compat/system|/compat/home",
			"--literal-pathspecs ls-files --error-unmatch -- tracked.txt|preserved-sentinel|/compat/global|/compat/system|/compat/home",
			"--literal-pathspecs ls-files -- .tpatch/local/|preserved-sentinel|/compat/global|/compat/system|/compat/home",
			"status --short|preserved-sentinel|/compat/global|/compat/system|/compat/home",
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("compatibility wrapper golden drift\n got: %q\nwant: %q", got, want)
		}
	})
}
