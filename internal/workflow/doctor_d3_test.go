package workflow

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/tesseracode/tesserapatch/assets"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestDoctorD3CleanAndDetectsAllSixStaleSkillAssets(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	installDoctorSkillFixtures(t, root, false)
	clean, err := RunDoctor(s, DoctorOptions{Checks: []string{"D3"}})
	if err != nil {
		t.Fatal(err)
	}
	if clean.Summary.Findings != 0 {
		t.Fatalf("clean installed skills reported findings: %#v", clean.Findings)
	}

	installDoctorSkillFixtures(t, root, true)
	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D3"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Findings != 6 {
		t.Fatalf("D3 findings = %d, want 6: %#v", report.Summary.Findings, report.Findings)
	}
	for _, asset := range doctorSkillAssets(root) {
		found := false
		for _, f := range report.Findings {
			if f.Code == "stale-skill-asset" && f.Path == relOrAbs(root, asset.Dst) && f.Fixable && f.BackupPath != "" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing stale finding for %s in %#v", asset.Dst, report.Findings)
		}
	}
}

func TestDoctorD3FixReplacesWithBackupAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	asset := doctorSkillAssets(root)[0]
	bundled := mustSkillBytes(t, asset.Src)
	drifted := append([]byte("# tessera-patch local drift\n"), bundled...)
	writeFile(t, asset.Dst, drifted)

	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D3"}, Fix: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Fixed != 1 || report.Summary.Findings != 0 || report.Summary.Errors != 0 {
		t.Fatalf("fix summary = %#v findings=%#v", report.Summary, report.Findings)
	}
	if got := DoctorExitCode(report); got != 0 {
		t.Fatalf("successful --fix exit = %d", got)
	}
	if got := readFile(t, asset.Dst); !bytes.Equal(got, bundled) {
		t.Fatal("D3 did not replace installed asset with bundled bytes")
	}
	if got := readFile(t, BackupPathForOverwrite(asset.Dst)); !bytes.Equal(got, drifted) {
		t.Fatal("D3 backup does not match pre-fix bytes")
	}

	second, err := RunDoctor(s, DoctorOptions{Checks: []string{"D3"}, Fix: true})
	if err != nil {
		t.Fatal(err)
	}
	if second.Summary != (DoctorSummary{ChecksRun: 1}) {
		t.Fatalf("second --fix should be clean no-op, got %#v %#v", second.Summary, second.Findings)
	}
	if got := readFile(t, BackupPathForOverwrite(asset.Dst)); !bytes.Equal(got, drifted) {
		t.Fatal("idempotent second run modified backup")
	}
}

func TestDoctorD3RefusesUnrecognizedUserContent(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	asset := doctorSkillAssets(root)[1]
	userContent := []byte("personal instructions without marker\n")
	writeFile(t, asset.Dst, userContent)

	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D3"}, Fix: true})
	if err != nil {
		t.Fatal(err)
	}
	f := assertFinding(t, report, "D3", "skill-asset-unrecognized", "")
	if f.Severity != "error" || f.Fixable {
		t.Fatalf("unrecognized finding = %#v", f)
	}
	if got := DoctorExitCode(report); got != 2 {
		t.Fatalf("refused --fix exit = %d, want 2", got)
	}
	if got := readFile(t, asset.Dst); !bytes.Equal(got, userContent) {
		t.Fatal("D3 overwrote unrecognized user content")
	}
	if _, err := os.Stat(BackupPathForOverwrite(asset.Dst)); !os.IsNotExist(err) {
		t.Fatalf("unrecognized content should not create backup, stat err=%v", err)
	}
}

func TestDoctorD3RefusesBackupCollision(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	asset := doctorSkillAssets(root)[2]
	drifted := append([]byte("# tpatch drift\n"), mustSkillBytes(t, asset.Src)...)
	writeFile(t, asset.Dst, drifted)
	writeFile(t, BackupPathForOverwrite(asset.Dst), []byte("different prior backup\n"))

	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D3"}, Fix: true})
	if err != nil {
		t.Fatal(err)
	}
	f := assertFinding(t, report, "D3", "skill-asset-backup-collision", "")
	if f.Severity != "error" {
		t.Fatalf("backup collision finding = %#v", f)
	}
	if got := readFile(t, asset.Dst); !bytes.Equal(got, drifted) {
		t.Fatal("D3 overwrote drifted asset despite backup collision")
	}
}

func TestDoctorD3ExistingMatchingBackupAllowsFix(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	asset := doctorSkillAssets(root)[3]
	drifted := append([]byte("# tpatch drift\n"), mustSkillBytes(t, asset.Src)...)
	writeFile(t, asset.Dst, drifted)
	writeFile(t, BackupPathForOverwrite(asset.Dst), drifted)

	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D3"}, Fix: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Fixed != 1 || report.Summary.Errors != 0 {
		t.Fatalf("matching backup should allow fix: %#v %#v", report.Summary, report.Findings)
	}
}

func installDoctorSkillFixtures(t *testing.T, root string, drift bool) {
	t.Helper()
	for _, asset := range doctorSkillAssets(root) {
		data := mustSkillBytes(t, asset.Src)
		if drift {
			data = append([]byte("# tpatch drift\n"), data...)
		}
		writeFile(t, asset.Dst, data)
	}
}

func mustSkillBytes(t *testing.T, src string) []byte {
	t.Helper()
	data, err := assets.Skills.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
