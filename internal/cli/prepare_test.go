package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func prepareWorkspace(t *testing.T, complete bool) string {
	t.Helper()
	root := t.TempDir()
	feature := filepath.Join(root, ".tpatch", "features", "feature")
	if err := os.MkdirAll(filepath.Join(feature, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"status.json":             `{"state":"defined"}`,
		"request.md":              "request\n",
		"analysis.md":             "analysis\n",
		"spec.md":                 "spec\n",
		"exploration.md":          "exploration\n",
		"artifacts/analysis.json": `{}`,
	}
	if !complete {
		delete(files, "exploration.md")
	}
	for name, content := range files {
		path := filepath.Join(feature, filepath.FromSlash(name))
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// runPrepare drives the real root command through the real root error
// printer, so the returned stderr is byte-for-byte what the process emits
// (including the single `error:` line) and the returned code is the real
// process exit code.
func runPrepare(t *testing.T, args ...string) (int, string, string, error) {
	t.Helper()
	root := buildRootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	code := execute(root, &stderr)
	var err error
	if code != 0 {
		err = errors.New(strings.TrimSpace(stderr.String()))
	}
	return code, stdout.String(), stderr.String(), err
}

func TestPrepareCheckJSONAndNoMutation(t *testing.T) {
	workspace := prepareWorkspace(t, true)
	before := readTree(t, filepath.Join(workspace, ".tpatch"))

	code, stdout, stderr, err := runPrepare(t, "--path", workspace, "prepare", "feature", "--check", "--json", "--quiet")
	if err != nil || code != 0 {
		t.Fatalf("prepare = (%d, %v)", code, err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	var report struct {
		Slug      string `json:"slug"`
		Artifacts []struct {
			ID         string `json:"id"`
			State      string `json:"state"`
			Provenance string `json:"provenance"`
		} `json:"artifacts"`
		Overall struct {
			Readiness string `json:"structural_readiness"`
		} `json:"overall"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout)
	}
	if report.Slug != "feature" || report.Overall.Readiness != "ready" || len(report.Artifacts) != 4 {
		t.Fatalf("unexpected report: %#v", report)
	}
	for _, artifact := range report.Artifacts {
		if artifact.State != "present-nonempty" || artifact.Provenance != "unknown" {
			t.Fatalf("artifact = %#v", artifact)
		}
	}
	after := readTree(t, filepath.Join(workspace, ".tpatch"))
	if !bytes.Equal(before, after) {
		t.Fatal("prepare --check mutated .tpatch")
	}
}

func TestPrepareCheckPrecedenceAndExitEnvelope(t *testing.T) {
	workspace := prepareWorkspace(t, false)
	readyWorkspace := prepareWorkspace(t, true)

	code, stdout, _, err := runPrepare(t, "--path", readyWorkspace, "prepare", "feature")
	if code != 0 || err != nil || !strings.Contains(stdout, "Mode:    generate") ||
		strings.Contains(stdout, "Refusal:") {
		t.Fatalf("plain prepare = (%d, %q, %v)", code, stdout, err)
	}

	code, stdout, _, err = runPrepare(t, "--path", workspace, "prepare", "../../etc", "--check", "--quiet")
	if code != 3 || err == nil {
		t.Fatalf("unsafe slug = (%d, %v)", code, err)
	}
	if stdout != "prepare --check — indeterminate (slug-unsafe)\n" {
		t.Fatalf("unsafe slug output = %q", stdout)
	}

	code, stdout, _, err = runPrepare(t, "--path", workspace, "prepare", "feature", "--check", "--json", "--quiet")
	if code != 2 || err == nil {
		t.Fatalf("not-ready = (%d, %v)", code, err)
	}
	var report struct {
		Overall struct {
			Readiness string `json:"structural_readiness"`
		} `json:"overall"`
	}
	if json.Unmarshal([]byte(stdout), &report) != nil || report.Overall.Readiness != "not_ready" {
		t.Fatalf("not-ready report = %q", stdout)
	}
}

func readTree(t *testing.T, root string) []byte {
	t.Helper()
	var output bytes.Buffer
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			output.WriteString("D " + relative + "\n")
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		output.WriteString("F " + relative + "\x00")
		output.Write(data)
		output.WriteByte('\n')
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
