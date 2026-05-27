package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestReconcileHumanOutputEvidenceHint(t *testing.T) {
	dir, slug, _ := cliEvidenceFixture(t, "human evidence", map[string]string{
		"existing.txt": "base\nfeature\n",
		"new.txt":      "brand new\n",
	})

	out, errOut, err := runCLIForEvidence("reconcile", "--path", dir, "--allow-dirty", "--upstream-ref", "HEAD", slug)
	if err != nil {
		t.Fatalf("reconcile failed: %v\nstderr=%s\nstdout=%s", err, errOut, out)
	}
	verdictLine := "  - " + slug + " ["
	evidenceLine := "    evidence: phase-4 forward-apply"
	verdictIdx := strings.Index(out, verdictLine)
	evidenceIdx := strings.Index(out, evidenceLine)
	if verdictIdx < 0 || evidenceIdx < 0 {
		t.Fatalf("expected verdict and evidence hint lines in output:\n%s", out)
	}
	if evidenceIdx < verdictIdx {
		t.Fatalf("evidence hint must appear after verdict line:\n%s", out)
	}
	if !strings.Contains(out, "    evidence: file-novelty mixed-additive") {
		t.Fatalf("expected file-novelty evidence hint in output:\n%s", out)
	}
}

func TestStatusJSONIncludesEvidenceArtifact(t *testing.T) {
	dir, slug, _ := cliEvidenceFixture(t, "status evidence", map[string]string{
		"new.txt": "brand new\n",
	})

	out, errOut, err := runCLIForEvidence("reconcile", "--path", dir, "--allow-dirty", "--upstream-ref", "HEAD", slug)
	if err != nil {
		t.Fatalf("reconcile failed: %v\nstderr=%s\nstdout=%s", err, errOut, out)
	}
	statusOut, statusErr, err := runCLIForEvidence("status", "--path", dir, "--json")
	if err != nil {
		t.Fatalf("status failed: %v\nstderr=%s\nstdout=%s", err, statusErr, statusOut)
	}
	want := `"evidence_artifact": ".tpatch/features/` + slug + `/artifacts/reconcile-evidence.jsonl"`
	if !strings.Contains(statusOut, want) {
		t.Fatalf("expected status JSON evidence artifact %s in:\n%s", want, statusOut)
	}
}

func TestReconcileCLIEvidenceOutputPrivacyNoSourceLeak(t *testing.T) {
	secretSource := "SECRET_SOURCE_BODY_DO_NOT_LEAK_CLI"
	dir, slug, _ := cliEvidenceFixture(t, "privacy cli", map[string]string{
		"new.txt": secretSource + "\n",
	})

	humanOut, errOut, err := runCLIForEvidence("reconcile", "--path", dir, "--allow-dirty", "--upstream-ref", "HEAD", slug)
	if err != nil {
		t.Fatalf("reconcile failed: %v\nstderr=%s\nstdout=%s", err, errOut, humanOut)
	}
	statusOut, statusErr, err := runCLIForEvidence("status", "--path", dir, "--json")
	if err != nil {
		t.Fatalf("status failed: %v\nstderr=%s\nstdout=%s", err, statusErr, statusOut)
	}
	if strings.Contains(humanOut, secretSource) {
		t.Fatalf("human output leaked source body:\n%s", humanOut)
	}
	if strings.Contains(statusOut, secretSource) {
		t.Fatalf("status JSON leaked source body:\n%s", statusOut)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(statusOut), &payload); err != nil {
		t.Fatalf("status output is not JSON: %v\n%s", err, statusOut)
	}
}

func runCLIForEvidence(args ...string) (string, string, error) {
	root := buildRootCmd()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), errOut.String(), err
}

func cliEvidenceFixture(t *testing.T, title string, files map[string]string) (string, string, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	gitInitTestRepo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "existing.txt")
	gitRun(t, dir, "commit", "-m", "add existing")
	baseCommit := gitHead(t, dir)

	for path, content := range files {
		if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		gitRun(t, dir, "add", path)
	}
	patch := gitOut(t, dir, "diff", "--cached", "HEAD")
	gitRun(t, dir, "reset", "--hard", "HEAD")
	gitRun(t, dir, "clean", "-fd")

	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	feature, err := s.AddFeature(store.AddFeatureInput{Title: title, Request: "classify file novelty"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MarkFeatureState(feature.Slug, store.StateApplied, "apply", ""); err != nil {
		t.Fatal(err)
	}
	status, err := s.LoadFeatureStatus(feature.Slug)
	if err != nil {
		t.Fatal(err)
	}
	status.Apply.BaseCommit = baseCommit
	status.Apply.HasPatch = true
	if err := s.SaveFeatureStatus(status); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact(feature.Slug, "post-apply.patch", patch); err != nil {
		t.Fatal(err)
	}
	return dir, feature.Slug, s
}
