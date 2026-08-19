package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/assets"
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
	if !strings.Contains(out, "summary: 1 drift findings, 1 warnings, 0 fixed, 0 errors") {
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
	root.SetArgs([]string{"--path", rootDir, "doctor", "--check", "D10"})
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

func TestDoctorCLICheckIDsAreCaseSensitive(t *testing.T) {
	rootDir := t.TempDir()
	if _, err := store.Init(rootDir); err != nil {
		t.Fatal(err)
	}
	_, err := runDoctorCLI(t, rootDir, "doctor", "--check", "d3")
	if err == nil {
		t.Fatal("expected lowercase check ID to be rejected")
	}
	if !strings.Contains(err.Error(), "unknown doctor check") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDoctorCLID3FixEndToEndAndExitCodes(t *testing.T) {
	rootDir := t.TempDir()
	if _, err := store.Init(rootDir); err != nil {
		t.Fatal(err)
	}
	src := "skills/claude/tessera-patch/SKILL.md"
	dst := filepath.Join(rootDir, ".claude", "skills", "tessera-patch", "SKILL.md")
	bundled, err := assets.Skills.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	drifted := append([]byte("# tpatch drift\n"), bundled...)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, drifted, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runDoctorCLI(t, rootDir, "doctor", "--check", "D3")
	if err == nil || asExitCodeError(err).ExitCode() != 1 {
		t.Fatalf("D3 dry-run exit = %v stdout=%s", err, out)
	}
	if backups := findOrigBackups(t, rootDir); len(backups) != 0 {
		t.Fatalf("dry-run created backups: %#v", backups)
	}

	out, err = runDoctorCLI(t, rootDir, "doctor", "--fix", "--check", "D3")
	if err != nil {
		t.Fatalf("D3 --fix should exit 0: %v stdout=%s", err, out)
	}
	if !strings.Contains(out, "summary: 0 drift findings, 0 warnings, 1 fixed, 0 errors") {
		t.Fatalf("missing fixed summary: %s", out)
	}
	if got, err := os.ReadFile(dst); err != nil || !bytes.Equal(got, bundled) {
		t.Fatalf("installed asset not replaced: err=%v", err)
	}
	if got, err := os.ReadFile(dst + ".orig"); err != nil || !bytes.Equal(got, drifted) {
		t.Fatalf("backup not written: err=%v", err)
	}

	out, err = runDoctorCLI(t, rootDir, "doctor", "--fix", "--check", "D3")
	if err != nil {
		t.Fatalf("second D3 --fix should be clean: %v stdout=%s", err, out)
	}
	if backups := findOrigBackups(t, rootDir); len(backups) != 1 {
		t.Fatalf("second --fix created extra backups: %#v", backups)
	}
}

func TestDoctorCLID3FixRefusalExitCode2(t *testing.T) {
	rootDir := t.TempDir()
	if _, err := store.Init(rootDir); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(rootDir, ".github", "skills", "tessera-patch", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("personal notes without marker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runDoctorCLI(t, rootDir, "doctor", "--fix", "--check", "D3")
	if err == nil || asExitCodeError(err).ExitCode() != 2 {
		t.Fatalf("D3 refused --fix exit = %v stdout=%s", err, out)
	}
	if !strings.Contains(out, "skill-asset-unrecognized") || !strings.Contains(out, "summary: 0 drift findings, 0 warnings, 0 fixed, 1 errors") {
		t.Fatalf("missing refusal output: %s", out)
	}
}

func TestDoctorCLID4D5EndToEnd(t *testing.T) {
	rootDir := t.TempDir()
	s, err := store.Init(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.UpstreamLockPath(), []byte("remote: origin\nbranch: main\ncommit: abc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runDoctorCLI(t, rootDir, "doctor", "--check", "D4")
	if err == nil || asExitCodeError(err).ExitCode() != 1 {
		t.Fatalf("D4 dry-run exit = %v stdout=%s", err, out)
	}
	if !strings.Contains(out, "D4 lock-malformed-sha") {
		t.Fatalf("missing D4 malformed lock output: %s", out)
	}

	st, err := s.AddFeature(store.AddFeatureInput{Title: "Needs Evidence", Slug: "needs-evidence"})
	if err != nil {
		t.Fatal(err)
	}
	st.State = store.StateApplied
	st.Reconcile.AttemptedAt = "2026-07-28T00:00:00Z"
	st.Reconcile.Outcome = store.ReconcileReapplied
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatal(err)
	}
	out, err = runDoctorCLI(t, rootDir, "doctor", "--check", "D5")
	if err == nil || asExitCodeError(err).ExitCode() != 1 {
		t.Fatalf("D5 dry-run exit = %v stdout=%s", err, out)
	}
	if !strings.Contains(out, "D5 reconcile-evidence-missing") || !strings.Contains(out, "run tpatch reconcile needs-evidence") {
		t.Fatalf("missing D5 evidence output/remediation: %s", out)
	}
}

func TestDoctorCLID6ReleaseMetadataEndToEndAndFlagScope(t *testing.T) {
	rootDir := t.TempDir()
	gitInitTestRepo(t, rootDir)
	gitRun(t, rootDir, "tag", "v9.9.9")
	if _, err := store.Init(rootDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "CHANGELOG.md"), []byte("# Changelog\n\n## v9.9.9 — 2026-07-28 — Test\n\n- Test.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "release-metadata.json"), []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}

	root := buildRootCmd()
	if root.PersistentFlags().Lookup("release-metadata") != nil {
		t.Fatal("--release-metadata must not be a root persistent flag")
	}
	doctor, _, err := root.Find([]string{"doctor"})
	if err != nil {
		t.Fatal(err)
	}
	if doctor.Flags().Lookup("release-metadata") == nil {
		t.Fatal("--release-metadata must be local to the doctor subcommand")
	}

	out, err := runDoctorCLI(t, rootDir, "doctor", "--check", "D6", "--release-metadata", "release-metadata.json", "--json")
	if err == nil || asExitCodeError(err).ExitCode() != 1 {
		t.Fatalf("D6 missing GH release exit = %v stdout=%s", err, out)
	}
	if !strings.Contains(out, `"code": "release-missing-gh-release"`) || !strings.Contains(out, `"tag": "v9.9.9"`) {
		t.Fatalf("missing D6 JSON finding: %s", out)
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
