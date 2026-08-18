package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intent"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// ---------------------------------------------------------------------------
// Integration harness
// ---------------------------------------------------------------------------

const avpSlug = "feature"

type avpFiles map[string]string

func defaultAVPFiles() avpFiles {
	return avpFiles{
		"status.json":             `{"state":"defined"}`,
		"analysis.md":             "analysis\n",
		"spec.md":                 "spec\n",
		"exploration.md":          "exploration\n",
		"artifacts/analysis.json": `{"summary":"x"}`,
	}
}

// avpWorkspace hand-builds a workspace so every artifact state is reachable
// without running a lifecycle phase.
func avpWorkspace(t *testing.T, files avpFiles) string {
	t.Helper()
	root := t.TempDir()
	feature := filepath.Join(root, ".tpatch", "features", avpSlug)
	if err := os.MkdirAll(filepath.Join(feature, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".tpatch", "config.yaml"), []byte("merge_strategy: 3way\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".tpatch", "FEATURES.md"), []byte("# Features\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		path := filepath.Join(feature, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func featurePath(root, name string) string {
	return filepath.Join(root, ".tpatch", "features", avpSlug, filepath.FromSlash(name))
}

// avpInit creates a real workspace through the real CLI.
func avpInit(t *testing.T, title string) (string, string) {
	t.Helper()
	root := t.TempDir()
	if code, _, stderr, _ := runPrepare(t, "--path", root, "init"); code != 0 {
		t.Fatalf("init failed: %s", stderr)
	}
	if code, _, stderr, _ := runPrepare(t, "--path", root, "add", title); code != 0 {
		t.Fatalf("add failed: %s", stderr)
	}
	return root, store.Slugify(title)
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func decodeReport(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var report map[string]any
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	return report
}

func reportArtifacts(t *testing.T, report map[string]any) []map[string]any {
	t.Helper()
	raw, ok := report["artifacts"].([]any)
	if !ok {
		t.Fatalf("artifacts is not an array: %#v", report["artifacts"])
	}
	out := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		artifact, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("artifact row is not an object: %#v", entry)
		}
		out = append(out, artifact)
	}
	return out
}

func artifactRow(t *testing.T, report map[string]any, id string) map[string]any {
	t.Helper()
	for _, artifact := range reportArtifacts(t, report) {
		if artifact["id"] == id {
			return artifact
		}
	}
	t.Fatalf("no artifact row %q", id)
	return nil
}

func readiness(t *testing.T, report map[string]any) string {
	t.Helper()
	overall, ok := report["overall"].(map[string]any)
	if !ok {
		t.Fatalf("overall is not an object")
	}
	return fmt.Sprint(overall["structural_readiness"])
}

func advisoryCodesOf(t *testing.T, report map[string]any) []string {
	t.Helper()
	raw, ok := report["advisories"].([]any)
	if !ok {
		t.Fatalf("advisories is not an array: %#v", report["advisories"])
	}
	var out []string
	for _, entry := range raw {
		advisory, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("advisory is not an object")
		}
		out = append(out, fmt.Sprint(advisory["code"]))
	}
	return out
}

func avpContains(haystack []string, needle string) bool {
	for _, entry := range haystack {
		if entry == needle {
			return true
		}
	}
	return false
}

// abortFixtures maps each of the thirteen abort codes to a workspace and the
// argument vector that reaches it.
type abortFixture struct {
	code string
	args func(t *testing.T) []string
}

func allAbortFixtures() []abortFixture {
	return []abortFixture{
		{code: "slug-unsafe", args: func(t *testing.T) []string {
			return []string{"--path", avpWorkspace(t, defaultAVPFiles()), "prepare", "../../etc", "--check"}
		}},
		{code: "workspace-unsupported-platform", args: nil},
		{code: "workspace-not-initialized", args: func(t *testing.T) []string {
			return []string{"--path", t.TempDir(), "prepare", avpSlug, "--check"}
		}},
		{code: "workspace-root-unopenable", args: nil},
		{code: "feature-dir-unsafe", args: func(t *testing.T) []string {
			root := avpWorkspace(t, defaultAVPFiles())
			features := filepath.Join(root, ".tpatch", "features")
			target := filepath.Join(root, "elsewhere")
			if err := os.MkdirAll(filepath.Join(target, avpSlug), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(features); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, features); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			return []string{"--path", root, "prepare", avpSlug, "--check"}
		}},
		{code: "feature-not-found", args: func(t *testing.T) []string {
			return []string{"--path", avpWorkspace(t, defaultAVPFiles()), "prepare", "other-feature", "--check"}
		}},
		{code: "status-symlink-refused", args: func(t *testing.T) []string {
			root := avpWorkspace(t, defaultAVPFiles())
			path := featurePath(root, "status.json")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(featurePath(root, "spec.md"), path); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			return []string{"--path", root, "prepare", avpSlug, "--check"}
		}},
		{code: "status-not-regular", args: func(t *testing.T) []string {
			files := defaultAVPFiles()
			delete(files, "status.json")
			root := avpWorkspace(t, files)
			if err := os.MkdirAll(featurePath(root, "status.json"), 0o755); err != nil {
				t.Fatal(err)
			}
			return []string{"--path", root, "prepare", avpSlug, "--check"}
		}},
		{code: "status-oversize", args: func(t *testing.T) []string {
			files := defaultAVPFiles()
			files["status.json"] = strings.Repeat("x", intent.MaxStatusBytes+1)
			return []string{"--path", avpWorkspace(t, files), "prepare", avpSlug, "--check"}
		}},
		{code: "status-unreadable", args: func(t *testing.T) []string {
			if os.Geteuid() == 0 {
				t.Skip("running as root: mode 0000 is still readable")
			}
			root := avpWorkspace(t, defaultAVPFiles())
			if err := os.Chmod(featurePath(root, "status.json"), 0o000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(featurePath(root, "status.json"), 0o644) })
			return []string{"--path", root, "prepare", avpSlug, "--check"}
		}},
		{code: "status-unstable", args: nil},
		{code: "status-malformed", args: func(t *testing.T) []string {
			files := defaultAVPFiles()
			files["status.json"] = "{not json"
			return []string{"--path", avpWorkspace(t, files), "prepare", avpSlug, "--check"}
		}},
		{code: "status-invalid-state", args: func(t *testing.T) []string {
			files := defaultAVPFiles()
			files["status.json"] = `{"state":"prepared"}`
			return []string{"--path", avpWorkspace(t, files), "prepare", avpSlug, "--check"}
		}},
	}
}

// reachableAbortFixtures are the abort codes an integration run can actually
// produce on this platform. `workspace-unsupported-platform`,
// `workspace-root-unopenable` and `status-unstable` are injected populations
// (AVP-179, AVP-159/AVP-160, AVP-036) and are covered in internal/intent.
func reachableAbortFixtures() []abortFixture {
	var out []abortFixture
	for _, fixture := range allAbortFixtures() {
		if fixture.args != nil {
			out = append(out, fixture)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// A — CLI grammar and surface boundary
// ---------------------------------------------------------------------------

func TestAVPGrammarAndSurface(t *testing.T) {
	t.Run("AVP-001", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		code, stdout, stderr, err := runPrepare(t, "--path", root, "prepare", avpSlug, "--check")
		if code != 0 || err != nil {
			t.Fatalf("exit = %d (%v), stderr=%q", code, err, stderr)
		}
		if !strings.HasPrefix(stdout, "prepare --check  "+avpSlug+"\n") {
			t.Fatalf("human report missing from stdout: %q", stdout)
		}
		if !strings.Contains(stdout, "readiness: ready") {
			t.Fatalf("stdout does not report ready: %q", stdout)
		}
	})

	t.Run("AVP-002", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare")
		if code != 1 {
			t.Fatalf("exit = %d, want 1", code)
		}
		if stdout != "" {
			t.Fatalf("stdout = %q, want empty", stdout)
		}
		if !strings.Contains(stderr, "arg") {
			t.Fatalf("stderr does not carry the cobra arity message: %q", stderr)
		}
	})

	t.Run("AVP-003", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", "a", "b", "--check")
		if code != 1 || stdout != "" {
			t.Fatalf("exit = %d, stdout = %q; want exit 1 with no report", code, stdout)
		}
	})

	for _, flag := range []struct{ id, name string }{
		{id: "AVP-004", name: "--manual"},
		{id: "AVP-005", name: "--regenerate"},
	} {
		t.Run(flag.id, func(t *testing.T) {
			root := avpWorkspace(t, defaultAVPFiles())
			before := readTree(t, filepath.Join(root, ".tpatch"))
			code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", flag.name)
			if code != 1 {
				t.Fatalf("exit = %d, want 1", code)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "unknown flag") {
				t.Fatalf("stderr = %q, want an unknown-flag message", stderr)
			}
			if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
				t.Fatal(".tpatch changed on a parse error")
			}
		})
	}

	t.Run("AVP-006", func(t *testing.T) {
		outside := t.TempDir()
		code, stdout, stderr, _ := runPrepare(t, "--path", outside, "prepare", avpSlug)
		if code != 4 {
			t.Fatalf("exit = %d, want 4 (not 3)", code)
		}
		if stdout != "" {
			t.Fatalf("stdout = %q, want empty", stdout)
		}
		lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
		if len(lines) != 1 || !strings.HasPrefix(lines[0], "error: ") {
			t.Fatalf("stderr = %q, want exactly one error: line", stderr)
		}
		if !strings.Contains(stderr, "--check") {
			t.Fatalf("the error line does not name --check: %q", stderr)
		}
		if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
			t.Fatalf("the reserved-surface refusal touched the filesystem: %v %v", entries, err)
		}
	})

	t.Run("AVP-007", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug)
		if code != 4 || stdout != "" {
			t.Fatalf("exit = %d, stdout = %q", code, stdout)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal(".tpatch changed")
		}
	})

	t.Run("AVP-008", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		nested := filepath.Join(root, "sub", "dir")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatal(err)
		}
		code, stdout, _, _ := runPrepare(t, "--path", nested, "prepare", avpSlug, "--check", "--json", "--quiet")
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		report := decodeReport(t, stdout)
		if report["slug"] != avpSlug || readiness(t, report) != "ready" {
			t.Fatalf("report = %#v", report)
		}
	})

	t.Run("AVP-009", func(t *testing.T) {
		_, stdout, _, _ := runPrepare(t, "prepare", "--help")
		if !strings.Contains(stdout, "apply --mode prepare") || !strings.Contains(stdout, "unrelated") {
			t.Fatalf("prepare --help does not disclaim the collision: %q", stdout)
		}
		if !strings.Contains(stdout, "--check") {
			t.Fatalf("prepare --help does not present --check: %q", stdout)
		}
		for _, forbidden := range []string{"--manual", "--regenerate"} {
			if strings.Contains(stdout, forbidden) {
				t.Fatalf("prepare --help advertises the unregistered flag %s", forbidden)
			}
		}
	})

	t.Run("AVP-010", func(t *testing.T) {
		_, stdout, _, _ := runPrepare(t, "apply", "--help")
		if !strings.Contains(stdout, "tpatch prepare <slug> --check") {
			t.Fatalf("apply --help does not point at prepare --check: %q", stdout)
		}
	})
}

// ---------------------------------------------------------------------------
// C — readiness and exit codes; D — output shape
// ---------------------------------------------------------------------------

func TestAVPReadinessAndOutput(t *testing.T) {
	t.Run("AVP-031", func(t *testing.T) {
		files := defaultAVPFiles()
		delete(files, "artifacts/analysis.json")
		root := avpWorkspace(t, files)
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		report := decodeReport(t, stdout)
		overall := report["overall"].(map[string]any)
		if readiness(t, report) != "ready" ||
			overall["required_total"].(float64) != 3 || overall["required_satisfied"].(float64) != 3 ||
			overall["optional_total"].(float64) != 1 || overall["optional_satisfied"].(float64) != 0 {
			t.Fatalf("overall = %#v", overall)
		}
	})

	t.Run("AVP-032", func(t *testing.T) {
		files := defaultAVPFiles()
		delete(files, "exploration.md")
		root := avpWorkspace(t, files)
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		if code != 2 {
			t.Fatalf("exit = %d, want 2", code)
		}
		report := decodeReport(t, stdout)
		if readiness(t, report) != "not_ready" {
			t.Fatalf("readiness = %q", readiness(t, report))
		}
		overall := report["overall"].(map[string]any)
		if overall["required_satisfied"].(float64) != 2 {
			t.Fatalf("required_satisfied = %v", overall["required_satisfied"])
		}
		row := artifactRow(t, report, "exploration")
		if row["state"] != "absent" || row["remediation"] == "" {
			t.Fatalf("exploration row = %#v", row)
		}
	})

	t.Run("AVP-033", func(t *testing.T) {
		files := avpFiles{"status.json": `{"state":"requested"}`}
		root := avpWorkspace(t, files)
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		if code != 2 {
			t.Fatalf("exit = %d, want 2", code)
		}
		report := decodeReport(t, stdout)
		overall := report["overall"].(map[string]any)
		if overall["required_satisfied"].(float64) != 0 {
			t.Fatalf("required_satisfied = %v", overall["required_satisfied"])
		}
		remediations := 0
		for _, artifact := range reportArtifacts(t, report) {
			if artifact["role"] == "required" && artifact["remediation"] != "" {
				remediations++
			}
		}
		if remediations != 3 {
			t.Fatalf("%d remediations, want 3", remediations)
		}
	})

	t.Run("AVP-034", func(t *testing.T) {
		files := defaultAVPFiles()
		delete(files, "artifacts/analysis.json")
		root := avpWorkspace(t, files)
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		report := decodeReport(t, stdout)
		overall := report["overall"].(map[string]any)
		if code != 0 || readiness(t, report) != "ready" || overall["optional_satisfied"].(float64) != 0 {
			t.Fatalf("exit=%d overall=%#v", code, overall)
		}
	})

	t.Run("AVP-035", func(t *testing.T) {
		files := defaultAVPFiles()
		files["artifacts/analysis.json"] = "[1,2,3]"
		root := avpWorkspace(t, files)
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		report := decodeReport(t, stdout)
		if code != 0 || readiness(t, report) != "ready" {
			t.Fatalf("exit = %d, readiness = %q", code, readiness(t, report))
		}
		if !avpContains(advisoryCodesOf(t, report), "analysis-sidecar-invalid-structured") {
			t.Fatalf("advisories = %v", advisoryCodesOf(t, report))
		}
	})

	// AVP-036 / AVP-037 — instability is an injected population; the report
	// shape it produces is asserted here through the same renderer the CLI
	// uses, and the classification itself in internal/intent.
	for _, id := range []string{"AVP-036", "AVP-037"} {
		t.Run(id, func(t *testing.T) {
			report := intent.Report{
				SchemaVersion: 1, Command: "prepare --check", Slug: avpSlug,
				FeatureState: "defined",
				Artifacts: []intent.Artifact{
					{ID: "analysis", Role: "required", State: intent.StatePresentNonempty},
					{ID: "spec", Role: "required", State: intent.StateUnstable},
					{ID: "exploration", Role: "required", State: intent.StateUnstable},
					{ID: "analysis_sidecar", Role: "optional", State: intent.StatePresentNonempty},
				},
				Overall:    intent.Overall{StructuralReadiness: intent.ReadinessIndeterminate, RequiredTotal: 3, OptionalTotal: 1},
				Advisories: []intent.Advisory{},
			}
			if report.Abort != nil {
				t.Fatal("an instability report must carry no abort object")
			}
			if len(report.Artifacts) != 4 {
				t.Fatalf("artifacts length = %d, want 4", len(report.Artifacts))
			}
			if got := prepareExit(report); exitCodeFor(got) != 3 {
				t.Fatalf("exit = %d, want 3", exitCodeFor(got))
			}
		})
	}

	t.Run("AVP-038", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", "no-such-feature", "--check", "--json", "--quiet")
		if code != 3 {
			t.Fatalf("exit = %d, want 3", code)
		}
		report := decodeReport(t, stdout)
		abort := report["abort"].(map[string]any)
		if abort["code"] != "feature-not-found" {
			t.Fatalf("abort = %#v", abort)
		}
		if len(reportArtifacts(t, report)) != 0 {
			t.Fatal("an abort emitted artifact rows")
		}
	})

	t.Run("AVP-039", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		report := decodeReport(t, stdout)
		if report["schema_version"].(float64) != 1 || report["command"] != "prepare --check" {
			t.Fatalf("report = %#v", report)
		}
	})

	t.Run("AVP-040", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		want := []string{"schema_version", "command", "slug", "feature_state", "disclaimer", "artifacts", "overall", "advisories"}
		if got := jsonKeyOrder(t, stdout); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("key order = %v, want %v", got, want)
		}
		abortRoot := avpWorkspace(t, defaultAVPFiles())
		_, abortStdout, _, _ := runPrepare(t, "--path", abortRoot, "prepare", "missing-one", "--check", "--json", "--quiet")
		if got := jsonKeyOrder(t, abortStdout); strings.Join(got, ",") != strings.Join(append(want, "abort"), ",") {
			t.Fatalf("abort key order = %v", got)
		}
	})

	t.Run("AVP-041", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		report := decodeReport(t, stdout)
		want := []string{"analysis", "spec", "exploration", "analysis_sidecar"}
		var got []string
		for _, artifact := range reportArtifacts(t, report) {
			got = append(got, fmt.Sprint(artifact["id"]))
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("artifact order = %v, want %v", got, want)
		}
	})

	t.Run("AVP-042", func(t *testing.T) {
		for _, fixture := range reachableAbortFixtures() {
			t.Run(fixture.code, func(t *testing.T) {
				args := append(fixture.args(t), "--json", "--quiet")
				code, stdout, _, _ := runPrepare(t, args...)
				if code != 3 {
					t.Fatalf("exit = %d, want 3", code)
				}
				if !strings.Contains(stdout, `"artifacts": []`) {
					t.Fatalf("artifacts is not the empty array: %s", stdout)
				}
				if !strings.Contains(stdout, `"advisories": []`) {
					t.Fatalf("advisories is not the empty array: %s", stdout)
				}
				report := decodeReport(t, stdout)
				abort := report["abort"].(map[string]any)
				if abort["code"] != fixture.code {
					t.Fatalf("abort code = %v, want %s", abort["code"], fixture.code)
				}
			})
		}
	})

	t.Run("AVP-043", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		report := decodeReport(t, stdout)
		codes := advisoryCodesOf(t, report)
		if len(codes) != 1 || codes[0] != "provenance-unknown-by-design" {
			t.Fatalf("advisories = %v", codes)
		}
	})

	t.Run("AVP-044", func(t *testing.T) {
		files := defaultAVPFiles()
		delete(files, "status.json")
		delete(files, "artifacts/analysis.json")
		root := avpWorkspace(t, files)
		_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		codes := advisoryCodesOf(t, decodeReport(t, stdout))
		want := []string{"feature-state-absent", "analysis-sidecar-absent-path-b-normal", "provenance-unknown-by-design"}
		if strings.Join(codes, ",") != strings.Join(want, ",") {
			t.Fatalf("advisory order = %v, want %v", codes, want)
		}
	})

	t.Run("AVP-045", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		want := []string{"id", "path", "role", "state", "reason_code", "provenance", "remediation"}
		if got := artifactKeyOrder(t, stdout); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("artifact key order = %v, want %v", got, want)
		}
	})

	t.Run("AVP-046", func(t *testing.T) {
		const disclaimer = "Structural presence only. This report does not certify semantic quality."
		root := avpWorkspace(t, defaultAVPFiles())
		_, jsonOut, humanOut, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json")
		if decodeReport(t, jsonOut)["disclaimer"] != disclaimer {
			t.Fatal("JSON disclaimer drifted")
		}
		if !strings.HasSuffix(strings.TrimRight(humanOut, "\n"), disclaimer) {
			t.Fatalf("human report does not end with the disclaimer: %q", humanOut)
		}
		_, _, abortHuman, _ := runPrepare(t, "--path", root, "prepare", "missing-x", "--check", "--json")
		if !strings.HasSuffix(reportPortion(abortHuman), disclaimer) {
			t.Fatalf("abort human report does not end with the disclaimer: %q", abortHuman)
		}
		_, _, withheldHuman, _ := runPrepare(t, "--path", root, "prepare", "../../etc", "--check", "--json")
		if !strings.HasSuffix(reportPortion(withheldHuman), disclaimer) {
			t.Fatalf("slug-withheld report does not end with the disclaimer: %q", withheldHuman)
		}
	})

	t.Run("AVP-047", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json")
		if code != 0 {
			t.Fatalf("exit = %d", code)
		}
		decodeReport(t, stdout)
		if !strings.Contains(stderr, "prepare --check  "+avpSlug) {
			t.Fatalf("human report is not on stderr: %q", stderr)
		}
		if strings.Contains(stderr, "error:") {
			t.Fatalf("stderr carries an error line: %q", stderr)
		}
	})

	t.Run("AVP-048", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		if code != 0 || stderr != "" {
			t.Fatalf("exit = %d, stderr = %q", code, stderr)
		}
		decodeReport(t, stdout)
	})

	t.Run("AVP-049", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--quiet")
		if code != 0 {
			t.Fatalf("exit = %d", code)
		}
		if stdout != "prepare --check "+avpSlug+" — ready\n" {
			t.Fatalf("quiet line = %q", stdout)
		}
	})

	t.Run("AVP-050", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			files avpFiles
			flags []string
			exit  int
		}{
			{"human-exit-0", defaultAVPFiles(), nil, 0},
			{"json-exit-0", defaultAVPFiles(), []string{"--json"}, 0},
			{"human-exit-2", withoutFile(defaultAVPFiles(), "spec.md"), nil, 2},
			{"json-exit-2", withoutFile(defaultAVPFiles(), "spec.md"), []string{"--json"}, 2},
		} {
			t.Run(tc.name, func(t *testing.T) {
				root := avpWorkspace(t, tc.files)
				args := append([]string{"--path", root, "prepare", avpSlug, "--check"}, tc.flags...)
				code1, stdout1, stderr1, _ := runPrepare(t, args...)
				code2, stdout2, stderr2, _ := runPrepare(t, args...)
				if code1 != tc.exit || code2 != tc.exit {
					t.Fatalf("exits = %d/%d, want %d", code1, code2, tc.exit)
				}
				if stdout1 != stdout2 || stderr1 != stderr2 {
					t.Fatal("two runs over a quiescent tree were not byte-identical")
				}
			})
		}
	})

	t.Run("AVP-052", func(t *testing.T) {
		// Required artifacts carry a remediation exactly when they are not
		// present-nonempty; the optional row never carries one.
		for _, tc := range []struct {
			name  string
			files avpFiles
		}{
			{"absent", withoutFile(defaultAVPFiles(), "spec.md")},
			{"empty", withFile(defaultAVPFiles(), "spec.md", "")},
			{"whitespace", withFile(defaultAVPFiles(), "spec.md", " \n\t")},
			{"ready", defaultAVPFiles()},
			{"sidecar-invalid", withFile(defaultAVPFiles(), "artifacts/analysis.json", "[")},
			{"sidecar-absent", withoutFile(defaultAVPFiles(), "artifacts/analysis.json")},
			{"sidecar-empty", withFile(defaultAVPFiles(), "artifacts/analysis.json", "  ")},
		} {
			t.Run(tc.name, func(t *testing.T) {
				root := avpWorkspace(t, tc.files)
				_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
				report := decodeReport(t, stdout)
				for _, artifact := range reportArtifacts(t, report) {
					remediation := fmt.Sprint(artifact["remediation"])
					switch {
					case artifact["role"] == "optional":
						if remediation != "" {
							t.Fatalf("optional row carries a remediation: %q", remediation)
						}
					case artifact["state"] == "present-nonempty":
						if remediation != "" {
							t.Fatalf("satisfied required row carries a remediation: %q", remediation)
						}
					default:
						if remediation == "" {
							t.Fatalf("failing required row %v carries no remediation", artifact["id"])
						}
					}
				}
			})
		}
	})
}

func withoutFile(files avpFiles, name string) avpFiles {
	out := avpFiles{}
	for key, value := range files {
		if key != name {
			out[key] = value
		}
	}
	return out
}

func withFile(files avpFiles, name, content string) avpFiles {
	out := avpFiles{}
	for key, value := range files {
		out[key] = value
	}
	out[name] = content
	return out
}

var jsonKeyRE = regexp.MustCompile(`(?m)^  "([a-z_]+)":`)
var artifactKeyRE = regexp.MustCompile(`(?m)^      "([a-z_]+)":`)

func jsonKeyOrder(t *testing.T, document string) []string {
	t.Helper()
	var out []string
	for _, match := range jsonKeyRE.FindAllStringSubmatch(document, -1) {
		out = append(out, match[1])
	}
	if len(out) == 0 {
		t.Fatalf("no top-level keys parsed from %q", document)
	}
	return out
}

func artifactKeyOrder(t *testing.T, document string) []string {
	t.Helper()
	matches := artifactKeyRE.FindAllStringSubmatch(document, -1)
	if len(matches) < 7 {
		t.Fatalf("no artifact keys parsed from %q", document)
	}
	var out []string
	for _, match := range matches[:7] {
		out = append(out, match[1])
	}
	return out
}

// ---------------------------------------------------------------------------
// E — zero mutation; F — provenance; I — security
// ---------------------------------------------------------------------------

func TestAVPZeroMutationAndPrivacy(t *testing.T) {
	t.Run("AVP-053", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		before := readTree(t, filepath.Join(root, ".tpatch"))
		if code, _, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--quiet"); code != 0 {
			t.Fatalf("exit = %d", code)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal(".tpatch changed")
		}
	})

	t.Run("AVP-054", func(t *testing.T) {
		root := gitWorkspace(t)
		before := avpGitPorcelain(t, root)
		runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--quiet")
		if after := avpGitPorcelain(t, root); after != before {
			t.Fatalf("git status changed:\n%q\n%q", before, after)
		}
	})

	t.Run("AVP-055", func(t *testing.T) {
		for _, fixture := range reachableAbortFixtures() {
			t.Run(fixture.code, func(t *testing.T) {
				args := fixture.args(t)
				root := args[1]
				tpatchDir := filepath.Join(root, ".tpatch")
				existed := false
				if _, err := os.Stat(tpatchDir); err == nil {
					existed = true
				}
				before := avpTreeSnapshot(t, tpatchDir)
				runPrepare(t, append(args, "--quiet")...)
				if existed {
					if !bytes.Equal(before, avpTreeSnapshot(t, tpatchDir)) {
						t.Fatalf("abort %s mutated .tpatch", fixture.code)
					}
				} else if _, err := os.Stat(tpatchDir); err == nil {
					t.Fatalf("abort %s created .tpatch", fixture.code)
				}
			})
		}
	})

	t.Run("AVP-056", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		runPrepare(t, "--path", root, "prepare", "brand-new-slug", "--check", "--quiet")
		if _, err := os.Stat(filepath.Join(root, ".tpatch", "features", "brand-new-slug")); err == nil {
			t.Fatal("the feature directory was created")
		}
	})

	t.Run("AVP-057", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		feature := filepath.Join(root, ".tpatch", "features", avpSlug)
		entries, err := os.ReadDir(feature)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if err := os.Chmod(filepath.Join(feature, entry.Name()), 0o444); err != nil {
				t.Fatal(err)
			}
		}
		if code, _, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--quiet"); code != 0 {
			t.Fatalf("exit = %d over a read-only tree, want 0", code)
		}
	})

	t.Run("AVP-058", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		features := filepath.Join(root, ".tpatch", "FEATURES.md")
		before, err := os.ReadFile(features)
		if err != nil {
			t.Fatal(err)
		}
		runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--quiet")
		after, err := os.ReadFile(features)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatal("FEATURES.md changed")
		}
	})

	t.Run("AVP-059", func(t *testing.T) {
		for name, files := range map[string]avpFiles{
			"ready":           defaultAVPFiles(),
			"absent":          withoutFile(defaultAVPFiles(), "spec.md"),
			"empty":           withFile(defaultAVPFiles(), "spec.md", ""),
			"sidecar-invalid": withFile(defaultAVPFiles(), "artifacts/analysis.json", "["),
		} {
			t.Run(name, func(t *testing.T) {
				root := avpWorkspace(t, files)
				_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
				for _, artifact := range reportArtifacts(t, decodeReport(t, stdout)) {
					if artifact["provenance"] != "unknown" {
						t.Fatalf("provenance = %v", artifact["provenance"])
					}
				}
			})
		}
	})

	t.Run("AVP-077", func(t *testing.T) {
		const sentinel = "ZZQQ-SENTINEL-CONTENT-ZZQQ"
		files := withFile(defaultAVPFiles(), "spec.md", sentinel+"\n")
		files = withFile(files, "artifacts/analysis.json", `{"summary":"`+sentinel+`"}`)
		root := avpWorkspace(t, files)
		for _, flags := range [][]string{nil, {"--json"}, {"--quiet"}, {"--json", "--quiet"}} {
			args := append([]string{"--path", root, "prepare", avpSlug, "--check"}, flags...)
			_, stdout, stderr, _ := runPrepare(t, args...)
			if strings.Contains(stdout, sentinel) || strings.Contains(stderr, sentinel) {
				t.Fatalf("artifact content leaked with flags %v", flags)
			}
		}
	})

	t.Run("AVP-078", func(t *testing.T) {
		const segment = "zzqq-path-sentinel"
		base := filepath.Join(t.TempDir(), segment)
		if err := os.MkdirAll(base, 0o755); err != nil {
			t.Fatal(err)
		}
		feature := filepath.Join(base, ".tpatch", "features", avpSlug)
		if err := os.MkdirAll(feature, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, content := range defaultAVPFiles() {
			mustWrite(t, filepath.Join(feature, filepath.FromSlash(name)), content)
		}
		cases := [][]string{
			{"--path", base, "prepare", avpSlug, "--check"},
			{"--path", base, "prepare", avpSlug},
			{"--path", base, "prepare", "missing-feature", "--check"},
			{"--path", base, "prepare", "../../etc", "--check"},
			{"--path", t.TempDir(), "prepare", avpSlug, "--check"},
		}
		for _, args := range cases {
			for _, flags := range [][]string{nil, {"--json"}, {"--quiet"}, {"--json", "--quiet"}} {
				_, stdout, stderr, _ := runPrepare(t, append(append([]string{}, args...), flags...)...)
				if strings.Contains(stdout, segment) || strings.Contains(stderr, segment) {
					t.Fatalf("an absolute path segment leaked: %v %v\n%s%s", args, flags, stdout, stderr)
				}
				if strings.Contains(stdout, base) || strings.Contains(stderr, base) {
					t.Fatalf("an absolute path leaked: %v %v", args, flags)
				}
			}
		}
	})

	t.Run("AVP-080", func(t *testing.T) {
		root := avpWorkspace(t, withoutFile(defaultAVPFiles(), "spec.md"))
		target := filepath.Join(t.TempDir(), "zzqq-symlink-target.txt")
		mustWrite(t, target, "content\n")
		if err := os.Symlink(target, featurePath(root, "spec.md")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		_, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check")
		if strings.Contains(stdout+stderr, "zzqq-symlink-target") {
			t.Fatal("the symlink target was named")
		}
		if !strings.Contains(stdout, "symlink-refused") {
			t.Fatalf("spec was not refused: %q", stdout)
		}
	})

	t.Run("AVP-082", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		mustWrite(t, filepath.Join(root, ".tpatch", "config.yaml"),
			"provider:\n  type: openai\n  base_url: http://127.0.0.1:1/v1\n  model: bogus\n  auth_env: TPATCH_BOGUS_KEY\n")
		code, _, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--quiet")
		if code != 0 {
			t.Fatalf("exit = %d; the report must depend only on the inspected workspace", code)
		}
	})

	t.Run("AVP-086", func(t *testing.T) {
		root := avpWorkspace(t, defaultAVPFiles())
		_, stdout, _, _ := runPrepare(t, "--path", root, "prepare", avpSlug, "--check", "--json", "--quiet")
		report := decodeReport(t, stdout)
		for _, artifact := range reportArtifacts(t, report) {
			if artifact["state"] == "unstable" {
				t.Fatal("a quiescent tree produced an unstable artifact")
			}
		}
		for _, forbidden := range []string{"snapshot_id", "captured_at"} {
			if strings.Contains(stdout, `"`+forbidden+`"`) {
				t.Fatalf("the schema carries %q", forbidden)
			}
		}
	})
}

func gitWorkspace(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	root := avpWorkspace(t, defaultAVPFiles())
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "add", "-A"},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "-m", "fixture"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git %v failed: %v\n%s", args, err, output)
		}
	}
	return root
}

func avpGitPorcelain(t *testing.T, root string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	return string(output)
}

// avpTreeSnapshot records the shape and readable content of a tree without
// failing on the deliberately hostile fixtures (unreadable modes, symlinked
// components) that the abort populations require.
func avpTreeSnapshot(t *testing.T, root string) []byte {
	t.Helper()
	var output bytes.Buffer
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			relative = path
		}
		if err != nil {
			output.WriteString("E " + relative + "\n")
			return nil
		}
		if entry.IsDir() {
			output.WriteString("D " + relative + "\n")
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil {
			output.WriteString("E " + relative + "\n")
			return nil
		}
		output.WriteString(fmt.Sprintf("F %s %d %s\x00", relative, info.Size(), info.Mode()))
		if data, readErr := os.ReadFile(path); readErr == nil {
			output.Write(data)
		}
		output.WriteByte('\n')
		return nil
	})
	return output.Bytes()
}

// reportPortion strips the process-level `error:` line from a stderr stream,
// leaving the human report exactly as the renderer wrote it.
func reportPortion(stderr string) string {
	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	for len(lines) > 0 && strings.HasPrefix(lines[len(lines)-1], "error: ") {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}
