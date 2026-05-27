package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestReconcileWritesFileNoveltyEvidenceMixedAdditive(t *testing.T) {
	s, slug := buildFileNoveltyFixture(t, "mixed-additive", map[string]string{
		"existing.txt": "base\nfeature\n",
		"new.txt":      "brand new\n",
	})

	results, err := RunReconcile(context.Background(), s, []string{slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Outcome == "" {
		t.Fatalf("expected non-empty reconcile result, got %#v", results)
	}

	entry := readFileNoveltyEvidenceFromDisk(t, s, slug)
	if entry.ReasonCode != string(FileNoveltyMixedAdditive) {
		t.Fatalf("expected classification %q, got %q", FileNoveltyMixedAdditive, entry.ReasonCode)
	}
	if got := strings.Join(entry.MatchedPaths, ","); got != "existing.txt,new.txt" {
		t.Fatalf("expected sorted matched paths existing.txt,new.txt, got %q", got)
	}
}

func TestReconcileWritesFileNoveltyEvidenceAllNewFiles(t *testing.T) {
	s, slug := buildFileNoveltyFixture(t, "all-new-files", map[string]string{
		"new-a.txt": "a\n",
		"new-b.txt": "b\n",
	})

	results, err := RunReconcile(context.Background(), s, []string{slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Outcome == "" {
		t.Fatalf("expected non-empty reconcile result, got %#v", results)
	}

	entry := readFileNoveltyEvidenceFromDisk(t, s, slug)
	if entry.ReasonCode != string(FileNoveltyAllNewFiles) {
		t.Fatalf("expected classification %q, got %q", FileNoveltyAllNewFiles, entry.ReasonCode)
	}
	if entry.PreReconcilePresence != store.EvidencePresenceAbsent {
		t.Fatalf("expected absent pre-reconcile presence, got %q", entry.PreReconcilePresence)
	}
}

func TestReconcileWarnsOnMalformedEvidenceArtifact(t *testing.T) {
	s, slug := buildFileNoveltyFixture(t, "malformed-evidence", map[string]string{
		"new.txt": "brand new\n",
	})
	malformed := []byte(`{"schema_version":`)
	if err := os.MkdirAll(filepath.Dir(s.ReconcileEvidencePath(slug)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.ReconcileEvidencePath(slug), malformed, 0o644); err != nil {
		t.Fatal(err)
	}

	oldWarn := warnReconcileEvidence
	var warnings bytes.Buffer
	warnReconcileEvidence = func(format string, args ...any) {
		fmt.Fprintf(&warnings, format, args...)
	}
	t.Cleanup(func() { warnReconcileEvidence = oldWarn })

	results, err := RunReconcile(context.Background(), s, []string{slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatalf("reconcile should preserve verdict semantics despite malformed evidence: %v", err)
	}
	if len(results) != 1 || results[0].Outcome == "" {
		t.Fatalf("expected non-error verdict, got %#v", results)
	}

	warningText := warnings.String()
	if !strings.Contains(warningText, "reconcile evidence artifact malformed") || !strings.Contains(warningText, slug) {
		t.Fatalf("expected malformed evidence warning mentioning slug %q, got %q", slug, warningText)
	}
	got, err := os.ReadFile(s.ReconcileEvidencePath(slug))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, malformed) {
		t.Fatalf("malformed artifact was overwritten: got %q want %q", string(got), string(malformed))
	}
}

func buildFileNoveltyFixture(t *testing.T, title string, files map[string]string) (*store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	setupGitRepo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, "existing.txt")
	gitCommit(t, dir, "add existing")
	baseCommit := gitRevParse(t, dir, "HEAD")

	for path, content := range files {
		if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		gitAdd(t, dir, path)
	}
	patch := gitOutput(t, dir, "diff", "--cached", "HEAD")
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
	slug := feature.Slug
	if err := s.MarkFeatureState(slug, store.StateApplied, "apply", ""); err != nil {
		t.Fatal(err)
	}
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatal(err)
	}
	status.Apply.BaseCommit = baseCommit
	status.Apply.HasPatch = true
	if err := s.SaveFeatureStatus(status); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact(slug, "post-apply.patch", patch); err != nil {
		t.Fatal(err)
	}
	return s, slug
}

func readFileNoveltyEvidenceFromDisk(t *testing.T, s *store.Store, slug string) store.ReconcileEvidence {
	t.Helper()
	data, err := os.ReadFile(s.ReconcileEvidencePath(slug))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		var entry store.ReconcileEvidence
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("decode evidence line %q: %v", string(line), err)
		}
		if entry.EvidenceKind == store.EvidenceKindFileNovelty {
			return entry
		}
	}
	t.Fatalf("file-novelty evidence not found in %s: %s", s.ReconcileEvidencePath(slug), string(data))
	return store.ReconcileEvidence{}
}

func gitRevParse(t *testing.T, dir, rev string) string {
	t.Helper()
	return strings.TrimSpace(gitOutput(t, dir, "rev-parse", rev))
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}

func TestReconcileResultJSONExposesEvidence(t *testing.T) {
	s, slug := buildFileNoveltyFixture(t, "json evidence", map[string]string{
		"existing.txt": "base\nfeature\n",
		"new.txt":      "brand new\n",
	})

	results, err := RunReconcile(context.Background(), s, []string{slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	data, err := json.Marshal(results[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"evidence"`)) {
		t.Fatalf("expected evidence field in result JSON: %s", data)
	}
	if !bytes.Contains(data, []byte(`"evidence_kind":"forward-apply"`)) {
		t.Fatalf("expected phase evidence in result JSON: %s", data)
	}
	if !bytes.Contains(data, []byte(`"evidence_kind":"file-novelty"`)) {
		t.Fatalf("expected file-novelty evidence in result JSON: %s", data)
	}
}

func TestReconcileResultJSONOmitsEvidenceWhenNoArtifactWritten(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	feature, err := s.AddFeature(store.AddFeatureInput{Title: "missing patch", Request: "no patch artifact"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MarkFeatureState(feature.Slug, store.StateApplied, "apply", ""); err != nil {
		t.Fatal(err)
	}

	results, err := RunReconcile(context.Background(), s, []string{feature.Slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Outcome != store.ReconcileBlocked || results[0].Phase != "error" {
		t.Fatalf("expected caught pre-artifact error result, got %#v", results)
	}
	data, err := json.Marshal(results[0])
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"evidence"`)) || bytes.Contains(data, []byte(`"evidence_artifact"`)) {
		t.Fatalf("expected no evidence fields when saveReconcileArtifacts did not run: %s", data)
	}
}

func TestReconcileEvidenceReaderOutputPrivacyNoSourceLeak(t *testing.T) {
	secretSource := "SECRET_SOURCE_BODY_DO_NOT_LEAK"
	s, slug := buildFileNoveltyFixture(t, "privacy evidence", map[string]string{
		"new.txt": secretSource + "\n",
	})

	results, err := RunReconcile(context.Background(), s, []string{slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	data, err := json.Marshal(results[0])
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(secretSource)) {
		t.Fatalf("result JSON leaked source body: %s", data)
	}
	var human strings.Builder
	for _, entry := range results[0].Evidence {
		if entry.EvidenceKind == store.EvidenceKindFileNovelty {
			fmt.Fprintf(&human, "evidence: %s %s\n", entry.EvidenceKind, entry.ReasonCode)
		} else {
			fmt.Fprintf(&human, "evidence: %s %s\n", entry.Phase, entry.EvidenceKind)
		}
	}
	if strings.Contains(human.String(), secretSource) {
		t.Fatalf("human evidence hints leaked source body: %s", human.String())
	}
}
