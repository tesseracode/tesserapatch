package workflow

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestDoctorD4CleanAndMalformedLockClasses(t *testing.T) {
	root, s, sha := initDoctorD4GitRepo(t)
	writeDoctorD4Lock(t, s, string(canonicalDoctorUpstreamLock(store.UpstreamLock{Remote: "origin", Branch: "main", Commit: sha})))
	clean, err := RunDoctor(s, DoctorOptions{Checks: []string{"D4"}})
	if err != nil {
		t.Fatal(err)
	}
	if clean.Summary.Findings != 0 || clean.Summary.Errors != 0 || clean.Summary.Warnings != 0 {
		t.Fatalf("clean lock findings: %#v %#v", clean.Summary, clean.Findings)
	}

	cases := []struct {
		name  string
		body  string
		code  string
		field string
	}{
		{"unknown", "remote: origin\nbranch: main\ncommit: \"" + sha + "\"\nextra: true\n", "lock-unknown-field", "extra"},
		{"wrong-type", "remote: [origin]\nbranch: main\ncommit: \"" + sha + "\"\n", "lock-field-wrong-type", "remote"},
		{"missing", "remote: origin\ncommit: \"" + sha + "\"\n", "lock-missing-field", "branch"},
		{"bad-sha", "remote: origin\nbranch: main\ncommit: abc\n", "lock-malformed-sha", "commit"},
		{"malformed", "remote origin\nbranch: main\ncommit: \"" + sha + "\"\n", "lock-malformed", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeDoctorD4Lock(t, s, tc.body)
			report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D4"}})
			if err != nil {
				t.Fatal(err)
			}
			f := assertFinding(t, report, "D4", tc.code, "")
			if tc.field != "" && f.Field != tc.field {
				t.Fatalf("field = %q, want %q in %#v", f.Field, tc.field, f)
			}
		})
	}
	_ = root
}

func TestDoctorD4StaleRefAndUnreachableCommit(t *testing.T) {
	_, s, sha := initDoctorD4GitRepo(t)
	writeDoctorD4Lock(t, s, "remote: \"origin\"\nbranch: \"missing\"\ncommit: \""+sha+"\"\nurl: \"\"\n")
	stale, err := RunDoctor(s, DoctorOptions{Checks: []string{"D4"}})
	if err != nil {
		t.Fatal(err)
	}
	assertFinding(t, stale, "D4", "lock-stale-ref", "")

	orphan := strings.TrimSpace(runGitTest(t, s.Root, "commit-tree", "HEAD^{tree}", "-m", "orphan"))
	writeDoctorD4Lock(t, s, "remote: \"origin\"\nbranch: \"main\"\ncommit: \""+orphan+"\"\nurl: \"\"\n")
	unreachable, err := RunDoctor(s, DoctorOptions{Checks: []string{"D4"}})
	if err != nil {
		t.Fatal(err)
	}
	assertFinding(t, unreachable, "D4", "lock-unreachable-commit", "")
}

func TestDoctorD4FixNormalizesLegacyBranchAndIsIdempotent(t *testing.T) {
	_, s, sha := initDoctorD4GitRepo(t)
	before := []byte("branch: \"origin/main\"\ncommit: \"" + sha + "\"\nremote: \"origin\"\nurl: \"\"\n")
	writeDoctorD4Lock(t, s, string(before))
	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D4"}, Fix: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Fixed != 1 || report.Summary.Errors != 0 {
		t.Fatalf("fix summary = %#v findings=%#v", report.Summary, report.Findings)
	}
	after := readFile(t, s.UpstreamLockPath())
	if !bytes.Contains(after, []byte("branch: \"main\"")) || bytes.Contains(after, []byte("branch: \"origin/main\"")) {
		t.Fatalf("legacy branch was not normalized:\n%s", after)
	}
	if got := readFile(t, BackupPathForOverwrite(s.UpstreamLockPath())); !bytes.Equal(got, before) {
		t.Fatal("backup does not match pre-fix lock bytes")
	}
	second, err := RunDoctor(s, DoctorOptions{Checks: []string{"D4"}, Fix: true})
	if err != nil {
		t.Fatal(err)
	}
	if second.Summary != (DoctorSummary{ChecksRun: 1}) {
		t.Fatalf("second D4 fix not clean: %#v %#v", second.Summary, second.Findings)
	}
}

func TestDoctorD4FixRefusesCommitAdvanceOrBranchGuess(t *testing.T) {
	_, s, sha := initDoctorD4GitRepo(t)
	for _, tc := range []struct {
		name string
		body string
	}{
		{"missing-commit", "remote: origin\nbranch: main\n"},
		{"missing-branch", "remote: origin\ncommit: \"" + sha + "\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeDoctorD4Lock(t, s, tc.body)
			report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D4"}, Fix: true})
			if err != nil {
				t.Fatal(err)
			}
			f := assertFinding(t, report, "D4", "lock-missing-field", "")
			if f.Severity != "error" || report.Summary.Fixed != 0 {
				t.Fatalf("expected refusal error with no fixed item: %#v summary=%#v", f, report.Summary)
			}
			if got := string(readFile(t, s.UpstreamLockPath())); got != tc.body {
				t.Fatalf("D4 --fix rewrote refused lock:\n%s", got)
			}
			if _, err := os.Stat(BackupPathForOverwrite(s.UpstreamLockPath())); err == nil {
				t.Fatal("refused D4 --fix created a backup")
			}
		})
	}
}

func TestDoctorD4ImplementationUsesNoRemoteGitState(t *testing.T) {
	data := string(readFile(t, filepath.Join(".", "doctor_d4.go")))
	if strings.Contains(data, `"fetch"`) || strings.Contains(data, `"ls-remote"`) || strings.Contains(data, `"remote update"`) {
		t.Fatalf("D4 implementation must not fetch remote git state:\n%s", data)
	}
}

func initDoctorD4GitRepo(t *testing.T) (string, *store.Store, string) {
	t.Helper()
	root := t.TempDir()
	runGitTest(t, root, "init", "-b", "main")
	runGitTest(t, root, "config", "user.email", "doctor@example.test")
	runGitTest(t, root, "config", "user.name", "Doctor Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, root, "add", "README.md")
	runGitTest(t, root, "commit", "-m", "initial")
	sha := strings.TrimSpace(runGitTest(t, root, "rev-parse", "HEAD"))
	runGitTest(t, root, "update-ref", "refs/remotes/origin/main", sha)
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, s, sha
}

func writeDoctorD4Lock(t *testing.T, s *store.Store, body string) {
	t.Helper()
	if err := os.WriteFile(s.UpstreamLockPath(), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGitTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}
