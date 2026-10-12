package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func rocS0CLIResult(stdout, stderr string, code int, wantOut, wantErr string, wantCode int) error {
	if stdout != wantOut || stderr != wantErr || code != wantCode {
		return fmt.Errorf("frozen CLI result changed:\nstdout=%q\nstderr=%q\nexit=%d\nwant stdout=%q stderr=%q exit=%d",
			stdout, stderr, code, wantOut, wantErr, wantCode)
	}
	return nil
}

func rocS0CLIShadow(t *testing.T) (*store.Store, string) {
	t.Helper()
	s := rgaS5CLISetup(t, "complete", false)
	head, err := gitutil.HeadCommit(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Root, "a.txt"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	shadow, err := gitutil.CreateShadow(s.Root, "s5", head)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := gitutil.PruneShadow(s.Root, "s5"); err != nil {
			t.Errorf("prune S0 shadow: %v", err)
		}
	})
	if err := os.WriteFile(filepath.Join(shadow, "a.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := s.LoadFeatureStatus("s5")
	if err != nil {
		t.Fatal(err)
	}
	st.State = store.StateReconcilingShadow
	st.Apply.BaseCommit = head
	st.Reconcile.ShadowPath = shadow
	st.Reconcile.UpstreamCommit = head
	st.Reconcile.ResolveSession = "roc-s0-shadow"
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact("s5", "resolution-session.json", `{"outcomes":[{"path":"a.txt","status":"resolved"}]}`); err != nil {
		t.Fatal(err)
	}
	return s, shadow
}

func TestROCS0ExistingShadowTerminalBytes(t *testing.T) {
	for _, action := range []string{"accept", "reject", "shadow-diff"} {
		t.Run(action, func(t *testing.T) {
			s, shadow := rocS0CLIShadow(t)
			want := ""
			switch action {
			case "accept":
				want = "Accepted 1 file(s) for s5; state → applied\n"
			case "reject":
				want = "Rejected shadow for s5; state → applied\n"
			case "shadow-diff":
				// Git owns platform-specific absolute-path quoting; the CLI must
				// forward this exact native diff without changing its bytes.
				cmd := exec.Command("git", "diff", "--no-index", "--",
					filepath.Join(s.Root, "a.txt"), filepath.Join(shadow, "a.txt"))
				cmd.Dir = s.Root
				body, err := cmd.Output()
				if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
					t.Fatalf("changed-file diff oracle failed: %v", err)
				}
				want = string(body)
			}
			before, err := s.ReadFeatureFile("s5", "status.json")
			if err != nil {
				t.Fatal(err)
			}
			stdout, stderr, code := runCmdExit("reconcile", "--path", s.Root, "--"+action, "s5")
			if err := rocS0CLIResult(stdout, stderr, code, want, "", 0); err != nil {
				t.Fatal(err)
			}
			state, err := s.LoadFeatureStatus("s5")
			if err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(filepath.Join(s.Root, "a.txt"))
			if err != nil {
				t.Fatal(err)
			}
			switch action {
			case "accept":
				if string(body) != "new\n" || state.State != store.StateApplied || state.Reconcile.ShadowPath != "" {
					t.Fatalf("accept did not preserve its existing transition: body=%q status=%+v", body, state)
				}
			case "reject":
				if string(body) != "old\n" || state.State != store.StateApplied || state.Reconcile.ShadowPath != "" {
					t.Fatalf("reject changed source or retained shadow state: body=%q status=%+v", body, state)
				}
			case "shadow-diff":
				after, err := s.ReadFeatureFile("s5", "status.json")
				if err != nil || after != before || string(body) != "old\n" {
					t.Fatalf("shadow diff mutated status/source: %v", err)
				}
			}
		})
	}
}

func TestROCS0ShadowEmptyOutputBytes(t *testing.T) {
	for _, noFiles := range []bool{false, true} {
		s, shadow := rocS0CLIShadow(t)
		want := "(shadow matches real tree — nothing to review)\n"
		if noFiles {
			want = "(no resolved files recorded in resolution-session.json)\n"
			if err := s.WriteArtifact("s5", "resolution-session.json", `{"outcomes":[]}`); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(filepath.Join(shadow, "a.txt"), []byte("old\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := runCmdExit("reconcile", "--path", s.Root, "--shadow-diff", "s5")
		if err := rocS0CLIResult(stdout, stderr, code, want, "", 0); err != nil {
			t.Fatal(err)
		}
	}
}

func TestROCS0ExistingReconcileFlagErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"accept-reject", []string{"--accept", "s5", "--reject", "s5"}, "reconcile: --accept, --reject, --shadow-diff are mutually exclusive"},
		{"accept-diff", []string{"--accept", "s5", "--shadow-diff", "s5"}, "reconcile: --accept, --reject, --shadow-diff are mutually exclusive"},
		{"reject-diff", []string{"--reject", "s5", "--shadow-diff", "s5"}, "reconcile: --accept, --reject, --shadow-diff are mutually exclusive"},
		{"apply-without-resolve", []string{"--apply"}, "reconcile: --apply requires --resolve"},
	}
	for _, terminal := range []string{"--accept", "--reject", "--shadow-diff"} {
		for _, pipeline := range []string{"--resolve", "--apply"} {
			cases = append(cases, struct {
				name string
				args []string
				want string
			}{terminal + pipeline, []string{terminal, "s5", pipeline}, "reconcile: --accept/--reject/--shadow-diff cannot be combined with --resolve/--apply"})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runCmdExit(append([]string{"reconcile"}, tc.args...)...)
			if err := rocS0CLIResult(stdout, stderr, code, "", "error: "+tc.want+"\n", 1); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestROCS0CLIByteGuardSensitivity(t *testing.T) {
	const out = "Accepted 1 file(s) for s5; state → applied\n"
	if err := rocS0CLIResult(out, "", 0, out, "", 0); err != nil {
		t.Fatal(err)
	}
	for _, err := range []error{
		rocS0CLIResult("", "", 0, out, "", 0),
		rocS0CLIResult(out+"\n", "", 0, out, "", 0),
		rocS0CLIResult(out, "unexpected warning\n", 0, out, "", 0),
		rocS0CLIResult(out, "", 1, out, "", 0),
	} {
		if err == nil {
			t.Fatal("CLI baseline comparison accepted changed bytes or exit status")
		}
	}
}
