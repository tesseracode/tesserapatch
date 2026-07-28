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

func TestReconcileEvidenceHintsPathRestructure(t *testing.T) {
	hints := reconcileEvidenceHints([]store.ReconcileEvidence{{
		EvidenceKind: store.EvidenceKindPathRestructure,
		ReasonCode:   "prefix-split",
		Confidence:   store.EvidenceConfidenceHigh,
		MatchedOperations: []string{
			"old_prefix=src/",
			"candidate_prefixes=app/|backend/",
		},
	}})
	if len(hints) != 1 {
		t.Fatalf("expected one hint, got %#v", hints)
	}
	for _, want := range []string{"path-restructure prefix-split", "old_prefix=src/", "candidate_prefixes=app/|backend/", "confidence=high"} {
		if !strings.Contains(hints[0], want) {
			t.Fatalf("path-restructure hint missing %q: %#v", want, hints)
		}
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

func TestReconcileJSONSurfacesConfirmationGateAndRevision(t *testing.T) {
	dir, slug := cliOperationUpstreamedCandidateFixture(t)

	out, errOut, err := runCLIForEvidence("reconcile", "--path", dir, "--allow-dirty", "--upstream-ref", "HEAD", "--format", "json", slug)
	if err != nil {
		t.Fatalf("reconcile failed: %v\nstderr=%s\nstdout=%s", err, errOut, out)
	}
	for _, want := range []string{`"outcome": "blocked"`, `"review_verdict": "rejected-upstreamed"`, `"evidence_kind": "manual-review"`, `"confirmation-gate"`, `"revisions"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %s in JSON output:\n%s", want, out)
		}
	}
	if strings.Contains(out, `"outcome": "upstreamed-candidate"`) {
		t.Fatalf("JSON output must preserve outcome=blocked and not invent a persisted outcome:\n%s", out)
	}
	if strings.Contains(out, "SECRET_CLI_GATE_DO_NOT_LEAK") {
		t.Fatalf("confirmation gate JSON leaked metadata secret:\n%s", out)
	}
}

func TestReconcileHumanOutputDisplaysUpstreamedCandidate(t *testing.T) {
	dir, slug := cliOperationUpstreamedCandidateFixture(t)

	out, errOut, err := runCLIForEvidence("reconcile", "--path", dir, "--allow-dirty", "--upstream-ref", "HEAD", slug)
	if err != nil {
		t.Fatalf("reconcile failed: %v\nstderr=%s\nstdout=%s", err, errOut, out)
	}
	if !strings.Contains(out, "[upstreamed-candidate]") {
		t.Fatalf("expected human output to display upstreamed-candidate for rejected gate:\n%s", out)
	}
	if strings.Contains(out, "[blocked]") {
		t.Fatalf("human output should not collapse rejected upstreamed candidate to generic blocked:\n%s", out)
	}
}

func TestReconcileReviewAddListJSON(t *testing.T) {
	dir, slug, _ := cliEvidenceFixture(t, "review list", map[string]string{"new.txt": "brand new\n"})

	out, errOut, err := runCLIForEvidence("reconcile", "review", "add", "--path", dir, slug, "--raw-verdict", "blocked", "--verdict", "false-negative", "--action", "reapplied", "--reason-code", "false-negative-blocked", "--final-state", "applied")
	if err != nil {
		t.Fatalf("review add failed: %v\nstderr=%s\nstdout=%s", err, errOut, out)
	}
	listOut, listErr, err := runCLIForEvidence("reconcile", "review", "list", "--path", dir, "--json", slug)
	if err != nil {
		t.Fatalf("review list failed: %v\nstderr=%s\nstdout=%s", err, listErr, listOut)
	}
	if !strings.Contains(listOut, `"review_verdict": "false-negative"`) || !strings.Contains(listOut, `"reason_code": "false-negative-blocked"`) {
		t.Fatalf("expected revision in list JSON:\n%s", listOut)
	}
	if !strings.Contains(listOut, `"corrupt_entries": []`) {
		t.Fatalf("expected JSON envelope with empty corrupt_entries:\n%s", listOut)
	}
}

func TestReconcileReviewListReportsCorruptEntries(t *testing.T) {
	dir, slug, s := cliEvidenceFixture(t, "review corrupt", map[string]string{"new.txt": "brand new\n"})
	first := cliSampleRevision(slug, "re_111111111111")
	second := cliSampleRevision(slug, "re_222222222222")
	if err := store.AppendReconcileRevision(s, slug, first); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendReconcileRevision(s, slug, second); err != nil {
		t.Fatal(err)
	}
	path := s.ReconcileRevisionsPath(slug)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected two seed revision lines, got %d: %s", len(lines), data)
	}
	corruptBody := lines[0] + "\n" + `{"schema_version":` + "\n" + lines[1] + "\n"
	if err := os.WriteFile(path, []byte(corruptBody), 0o644); err != nil {
		t.Fatal(err)
	}

	jsonOut, jsonErr, err := runCLIForEvidence("reconcile", "review", "list", "--path", dir, "--json", slug)
	if err == nil {
		t.Fatalf("review list --json should exit non-zero for corrupt entries; stderr=%s stdout=%s", jsonErr, jsonOut)
	}
	for _, want := range []string{`"corrupt_entries"`, `"line": 2`, first.EntryID, second.EntryID} {
		if !strings.Contains(jsonOut, want) {
			t.Fatalf("expected %s in corrupt JSON envelope:\n%s", want, jsonOut)
		}
	}
	var payload struct {
		Revisions      []store.ReconcileRevision `json:"revisions"`
		CorruptEntries []store.CorruptEntry      `json:"corrupt_entries"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &payload); err != nil {
		t.Fatalf("review list corrupt output is not valid JSON: %v\n%s", err, jsonOut)
	}
	if len(payload.Revisions) != 2 || len(payload.CorruptEntries) != 1 || payload.CorruptEntries[0].Line != 2 {
		t.Fatalf("unexpected corrupt JSON payload: %+v", payload)
	}

	humanOut, humanErr, err := runCLIForEvidence("reconcile", "review", "list", "--path", dir, slug)
	if err == nil {
		t.Fatalf("review list human should exit non-zero for corrupt entries; stderr=%s stdout=%s", humanErr, humanOut)
	}
	for _, want := range []string{first.EntryID, second.EntryID, "corrupted entries: line 2:"} {
		if !strings.Contains(humanOut, want) {
			t.Fatalf("expected %s in corrupt human output:\n%s", want, humanOut)
		}
	}
}

func TestReconcileReviewListReportsNonNewlineTerminatedFinalRevision(t *testing.T) {
	dir, slug, s := cliEvidenceFixture(t, "review non newline", map[string]string{"new.txt": "brand new\n"})
	first := cliSampleRevision(slug, "re_111111111111")
	second := cliSampleRevision(slug, "re_222222222222")
	if err := store.AppendReconcileRevision(s, slug, first); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendReconcileRevision(s, slug, second); err != nil {
		t.Fatal(err)
	}
	path := s.ReconcileRevisionsPath(slug)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		t.Fatalf("seed revisions should be newline-terminated: %q", data)
	}
	if err := os.WriteFile(path, bytes.TrimSuffix(data, []byte("\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	jsonOut, jsonErr, err := runCLIForEvidence("reconcile", "review", "list", "--path", dir, "--json", slug)
	if err == nil {
		t.Fatalf("review list --json should exit non-zero for non-newline-terminated final line; stderr=%s stdout=%s", jsonErr, jsonOut)
	}
	var payload struct {
		Revisions      []store.ReconcileRevision `json:"revisions"`
		CorruptEntries []store.CorruptEntry      `json:"corrupt_entries"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &payload); err != nil {
		t.Fatalf("review list non-newline output is not valid JSON: %v\n%s", err, jsonOut)
	}
	if len(payload.CorruptEntries) != 1 {
		t.Fatalf("expected exactly one corrupt entry, got %+v", payload.CorruptEntries)
	}
	if got := payload.CorruptEntries[0]; got.Line != 2 || got.Error != "final object is not newline-terminated" {
		t.Fatalf("unexpected corrupt entry: %+v", got)
	}
	if len(payload.Revisions) != 2 || payload.Revisions[0].EntryID != first.EntryID {
		t.Fatalf("valid preceding revision was not preserved: %+v", payload.Revisions)
	}
}

func TestReconcileHumanOutputHunkOverlapHint(t *testing.T) {
	dir, slug := cliHunkOverlapFixture(t)

	out, errOut, err := runCLIForEvidence("reconcile", "--path", dir, "--allow-dirty", "--upstream-ref", "HEAD", slug)
	if err != nil {
		t.Fatalf("reconcile failed: %v\nstderr=%s\nstdout=%s", err, errOut, out)
	}
	if !strings.Contains(out, "evidence: hunk-overlap edit-overlap") {
		t.Fatalf("expected hunk-overlap evidence hint in output:\n%s", out)
	}
	if strings.Contains(out, "SECRET_CLI_OVERLAP_DO_NOT_LEAK") {
		t.Fatalf("hunk-overlap output leaked source body:\n%s", out)
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

func cliOperationUpstreamedCandidateFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	gitInitTestRepo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "ops.txt"), []byte("already-present\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "ops.txt")
	gitRun(t, dir, "commit", "-m", "add ops")
	baseCommit := gitHead(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	feature, err := s.AddFeature(store.AddFeatureInput{Title: "operation upstreamed candidate", Request: "operation upstreamed candidate"})
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
	patch := "diff --git a/other.txt b/other.txt\n--- a/other.txt\n+++ b/other.txt\n@@ -1 +1 @@\n-a\n+b\n"
	if err := s.WriteArtifact(feature.Slug, "post-apply.patch", patch); err != nil {
		t.Fatal(err)
	}
	recipe := `{"version":1,"operations":[{"type":"replace-in-file","path":"ops.txt","search":"missing","replace":"already-present"}]}`
	if err := s.WriteArtifact(feature.Slug, "apply-recipe.json", recipe); err != nil {
		t.Fatal(err)
	}
	return dir, feature.Slug
}

func cliHunkOverlapFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	gitInitTestRepo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "existing.txt")
	gitRun(t, dir, "commit", "-m", "base")
	baseCommit := gitHead(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("line1\nSECRET_CLI_OVERLAP_DO_NOT_LEAK\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := gitOut(t, dir, "diff", "--no-color", "HEAD")
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("line1\nupstream-change\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "existing.txt")
	gitRun(t, dir, "commit", "-m", "upstream change")
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	feature, err := s.AddFeature(store.AddFeatureInput{Title: "hunk overlap", Request: "detect overlap"})
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
	return dir, feature.Slug
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

func cliSampleRevision(slug, evidence string) store.ReconcileRevision {
	entry := store.ReconcileRevision{
		SchemaVersion:       store.ReconcileRevisionSchemaVersion,
		FeatureSlug:         slug,
		EvidenceAttemptID:   evidence,
		RawReconcileVerdict: string(store.ReconcileBlocked),
		ReviewVerdict:       store.ReviewVerdictFalseNegative,
		FinalFeatureState:   store.StateApplied,
		ActionTaken:         store.ReconcileActionReapplied,
		ReasonCode:          "false-negative-blocked",
		ValidationRefs:      []store.ValidationRef{},
	}
	entry.EntryID = store.ComputeRevisionID(entry)
	return entry
}
