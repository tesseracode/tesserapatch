package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestDoctorD6CleanTagChangelogAndReleaseSnapshot(t *testing.T) {
	root, s := doctorD6Fixture(t, "v0.1.0")
	writeDoctorD6Changelog(t, root, "## v0.1.0 — 2026-07-28 — Test\n\n- Test release.\n")
	writeDoctorD6ReleaseMetadata(t, root, `[{"tagName":"v0.1.0","url":"https://example.invalid/v0.1.0","publishedAt":"2026-07-28T00:00:00Z"}]`)

	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D6"}, ReleaseMetadata: "release-metadata.json"})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected clean D6 report, got %#v", report.Findings)
	}
}

func TestDoctorD6ReportsTagChangelogAndGHReleaseDrift(t *testing.T) {
	root, s := doctorD6Fixture(t, "v1.0.0", "v1.1.0")
	writeDoctorD6Changelog(t, root, "## v1.0.0 — 2026-07-28 — One\n\n- One.\n\n## v1.2.0 — 2026-07-28 — Two\n\n- Two.\n")
	writeDoctorD6ReleaseMetadata(t, root, `{"releases":[{"tag":"v1.0.0","url":"https://example.invalid/v1.0.0","published_at":"2026-07-28T00:00:00Z"}]}`)

	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D6"}, ReleaseMetadata: "release-metadata.json"})
	if err != nil {
		t.Fatal(err)
	}
	assertDoctorD6Finding(t, report, "release-tag-missing-changelog", "v1.1.0")
	assertDoctorD6Finding(t, report, "release-changelog-missing-tag", "v1.2.0")
	assertDoctorD6Finding(t, report, "release-missing-gh-release", "v1.1.0")
}

func TestDoctorD6ReportsGHReleaseUnknownWithoutMetadata(t *testing.T) {
	root, s := doctorD6Fixture(t, "v2.0.0")
	writeDoctorD6Changelog(t, root, "## v2.0.0 — 2026-07-28 — Two\n\n- Two.\n")

	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D6"}})
	if err != nil {
		t.Fatal(err)
	}
	f := assertDoctorD6Finding(t, report, "release-gh-release-unknown", "v2.0.0")
	if f.Severity != "warning" {
		t.Fatalf("unknown GH Release status severity = %q, want warning", f.Severity)
	}
	if report.Summary.Warnings != 1 || report.Summary.Findings != 0 {
		t.Fatalf("unexpected summary for unknown metadata: %#v", report.Summary)
	}
}

func TestDoctorD6SkipsUpstreamNonTpatchContext(t *testing.T) {
	root, s := doctorD6Fixture(t, "v1.0.0", "v1.2.0")
	writeDoctorD6Changelog(t, root, "## 1.2.0 (2024-01-01)\n\n- upstream feature\n")

	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D6"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range report.Findings {
		if f.Code == "release-tag-missing-changelog" || f.Code == "release-changelog-missing-tag" {
			t.Fatalf("unexpected tag/CHANGELOG drift in non-tpatch context: %#v", report.Findings)
		}
	}
	if report.Summary.Findings != 0 {
		t.Fatalf("non-tpatch context produced drift findings: %#v", report.Summary)
	}
}

func TestDoctorD6MissingChangelogIsWarning(t *testing.T) {
	_, s := doctorD6Fixture(t, "v1.0.0")

	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D6"}})
	if err != nil {
		t.Fatal(err)
	}
	f := assertDoctorD6Finding(t, report, "release-changelog-unreadable", "")
	if f.Severity != "warning" {
		t.Fatalf("missing CHANGELOG severity = %q, want warning", f.Severity)
	}
}

func TestDoctorD6RemediationHasNoRepoDocRefs(t *testing.T) {
	root, s := doctorD6Fixture(t, "v1.0.0", "v1.1.0")
	writeDoctorD6Changelog(t, root, "## v1.0.0 — 2026-07-28 — One\n\n- One.\n\n## v1.2.0 — 2026-07-28 — Two\n\n- Two.\n")
	writeDoctorD6ReleaseMetadata(t, root, `{"releases":[{"tag":"v1.0.0","url":"https://example.invalid/v1.0.0","published_at":"2026-07-28T00:00:00Z"}]}`)

	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D6"}, ReleaseMetadata: "release-metadata.json"})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) == 0 {
		t.Fatal("expected D6 findings for remediation guard")
	}
	for _, f := range report.Findings {
		if strings.Contains(f.Remediation, "RELEASING.md") {
			t.Fatalf("remediation references repo doc: %#v", f)
		}
	}
}

func TestDoctorD6MalformedReleaseMetadataReportsLine(t *testing.T) {
	root, s := doctorD6Fixture(t, "v3.0.0")
	writeDoctorD6Changelog(t, root, "## v3.0.0 — 2026-07-28 — Three\n\n- Three.\n")
	writeDoctorD6ReleaseMetadata(t, root, "{\n  \"releases\": [\n")

	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D6"}, ReleaseMetadata: "release-metadata.json"})
	if err != nil {
		t.Fatal(err)
	}
	f := assertDoctorD6Finding(t, report, "release-metadata-malformed", "")
	if f.Line < 1 {
		t.Fatalf("malformed metadata line = %d, want positive line", f.Line)
	}
}

func TestDoctorD6ParsesGHReleaseListJSONShape(t *testing.T) {
	snapshot, err := doctorD6ParseReleaseMetadata([]byte(`[{"tagName":"v4.0.0","url":"https://example.invalid/v4.0.0","publishedAt":"2026-07-28T00:00:00Z"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Tags["v4.0.0"] {
		t.Fatalf("gh release list tagName shape did not decode: %#v", snapshot.Tags)
	}
}

func doctorD6Fixture(t *testing.T, tags ...string) (string, *store.Store) {
	t.Helper()
	root := t.TempDir()
	setupGitRepo(t, root)
	for _, tag := range tags {
		gitRun(t, root, "tag", tag)
	}
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, s
}

func writeDoctorD6Changelog(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("# Changelog\n\n"+body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeDoctorD6ReleaseMetadata(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "release-metadata.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertDoctorD6Finding(t *testing.T, report DoctorReport, code, tag string) DoctorFinding {
	t.Helper()
	for _, f := range report.Findings {
		if f.CheckID == "D6" && f.Code == code && (tag == "" || f.Tag == tag) {
			return f
		}
	}
	t.Fatalf("missing D6 finding %s/%s in %#v", code, tag, report.Findings)
	return DoctorFinding{}
}
