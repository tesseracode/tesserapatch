package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

func TestDoctorCLIJSONCheckFilteringAndExitCode(t *testing.T) {
	rootDir := t.TempDir()
	s, err := store.Init(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "Bad Metadata", Slug: "bad-metadata"}); err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(rootDir, ".tpatch", "features", "bad-metadata", "status.json")
	raw, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statusPath, append(bytes.TrimSuffix(raw, []byte("\n")), []byte(" extra")...), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	root := buildRootCmd()
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"--path", rootDir, "doctor", "--json", "--check", "D1"})
	execErr := root.Execute()
	if execErr == nil {
		t.Fatal("expected drift exit error")
	}
	if ec := asExitCodeError(execErr); ec == nil || ec.ExitCode() != 1 {
		t.Fatalf("expected exit code 1 for dry-run drift/errors, got %T %v", execErr, execErr)
	}
	var report workflow.DoctorReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("bad JSON report: %v\n%s", err, out.String())
	}
	if report.Summary.ChecksRun != 1 || len(report.Checks) != 1 || report.Checks[0].CheckID != "D1" {
		t.Fatalf("--check did not limit execution: %#v", report)
	}
}

func TestDoctorCLIDryRunDefaultNoBackupsAndIdempotentFixNoop(t *testing.T) {
	rootDir := t.TempDir()
	s, err := store.Init(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "Needs Manifest", Slug: "needs-manifest"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, ".tpatch", "features", "needs-manifest", "artifacts", "post-apply.patch"), []byte("diff --git a/a b/a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runDoctorCLI(t, rootDir, "doctor")
	if err == nil || asExitCodeError(err).ExitCode() != 1 {
		t.Fatalf("doctor dry-run exit = %v, stdout=%s", err, out)
	}
	if !strings.Contains(out, "summary: 1 drift findings, 0 warnings, 0 fixed, 0 errors") {
		t.Fatalf("missing summary counts: %s", out)
	}
	if backups := findOrigBackups(t, rootDir); len(backups) != 0 {
		t.Fatalf("dry-run created backups: %#v", backups)
	}

	if _, err := runDoctorCLI(t, rootDir, "doctor", "--fix"); err == nil || asExitCodeError(err).ExitCode() != 1 {
		t.Fatalf("doctor --fix read-only drift exit = %v", err)
	}
	if backups := findOrigBackups(t, rootDir); len(backups) != 0 {
		t.Fatalf("--fix for read-only Wave alpha classes created backups: %#v", backups)
	}
	if _, err := runDoctorCLI(t, rootDir, "doctor", "--fix"); err == nil || asExitCodeError(err).ExitCode() != 1 {
		t.Fatalf("second doctor --fix read-only drift exit = %v", err)
	}
	if backups := findOrigBackups(t, rootDir); len(backups) != 0 {
		t.Fatalf("second --fix created backups: %#v", backups)
	}
}

func TestDoctorCLIUnknownCheckFailsBeforeRun(t *testing.T) {
	rootDir := t.TempDir()
	if _, err := store.Init(rootDir); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	root := buildRootCmd()
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"--path", rootDir, "doctor", "--check", "D9"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected unknown check error")
	}
	if !strings.Contains(err.Error(), "unknown doctor check") {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("unknown check should not emit report: %s", out.String())
	}
}

func runDoctorCLI(t *testing.T, rootDir string, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	root := buildRootCmd()
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(append([]string{"--path", rootDir}, args...))
	err := root.Execute()
	return out.String(), err
}

func findOrigBackups(t *testing.T, rootDir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".orig") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
