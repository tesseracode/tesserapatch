package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitReportsGitignoreStatusHonestly enforces the F-INT-γ-4 LOW
// closure. Rev-0 `tpatch init` always printed
//
//	gitignore: appended ".tpatch/local/" to <path> (PRD-active-feature-session §4 D6)
//
// regardless of whether the workflow ACTUALLY appended anything.
// Operators re-running init (a common troubleshooting pattern) would
// be told the file had just been amended when it hadn't. Rev-1
// distinguishes the three outcomes surfaced by
// workflow.LocalIgnoreStatus:
//
//   - `created`         — .gitignore did not exist beforehand
//   - `appended`        — .gitignore existed but was missing the rule
//   - `already present` — the rule was already present (idempotent no-op)
//
// store.Init refuses a second init in the same repo, so each subcase
// gets its own fresh workspace fixture.
func TestInitReportsGitignoreStatusHonestly(t *testing.T) {
	cases := []struct {
		name         string
		preGitignore string
		wantSubstr   string
		mustNotHave  string
	}{
		{
			name:         "created-when-no-gitignore",
			preGitignore: "",
			wantSubstr:   "created",
		},
		{
			name:         "already-present-when-rule-preexists",
			preGitignore: "# preexisting\n.tpatch/local/\n",
			wantSubstr:   "already present",
			mustNotHave:  "appended",
		},
		{
			name:         "appended-when-gitignore-missing-rule",
			preGitignore: "# preexisting\nnode_modules/\n",
			wantSubstr:   "appended",
			mustNotHave:  "already present",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			runInitTestCmd(t, tmp, "git", "init", "-q")
			runInitTestCmd(t, tmp, "git", "config", "user.email", "test@example.com")
			runInitTestCmd(t, tmp, "git", "config", "user.name", "test")

			gitignore := filepath.Join(tmp, ".gitignore")
			if tc.preGitignore != "" {
				if err := os.WriteFile(gitignore, []byte(tc.preGitignore), 0o644); err != nil {
					t.Fatalf("seed .gitignore: %v", err)
				}
			}

			out, _, code := runSessionCmd("init", "--path", tmp)
			if code != 0 {
				t.Fatalf("init failed: %s", out)
			}
			if !strings.Contains(out, "gitignore:") {
				t.Fatalf("expected `gitignore:` status line in output; got %q", out)
			}
			if !strings.Contains(out, tc.wantSubstr) {
				t.Fatalf("expected `%s` in output (F-INT-γ-4); got %q", tc.wantSubstr, out)
			}
			if tc.mustNotHave != "" && strings.Contains(out, tc.mustNotHave) {
				t.Fatalf("output MUST NOT contain `%s` in this case (F-INT-γ-4); got %q", tc.mustNotHave, out)
			}
		})
	}
}

// runInitTestCmd is a small test helper that runs a shell command in dir and
// fails the test on non-zero exit. Kept local so this test doesn't
// depend on any specific helper the other rev-1 tests use.
func runInitTestCmd(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}
