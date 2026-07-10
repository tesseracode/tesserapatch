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

func buildReverseApplyConfirmationFixture(t *testing.T) (*store.Store, string) {
	t.Helper()
	secret := "SECRET_REVISION_METADATA_DO_NOT_LEAK"
	dir := filepath.Join(t.TempDir(), secret+"-repo-root")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	setupGitRepo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, "feature.txt")
	patch := gitOutput(t, dir, "diff", "--cached", "--no-color", "HEAD")
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	feature, err := s.AddFeature(store.AddFeatureInput{Title: secret + " reverse apply confirmation", Request: "confirm upstreamed"})
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
	status.Apply.BaseCommit = gitRevParse(t, dir, "HEAD")
	status.Apply.HasPatch = true
	if err := s.SaveFeatureStatus(status); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact(feature.Slug, "post-apply.patch", patch); err != nil {
		t.Fatal(err)
	}
	return s, feature.Slug
}

func buildOperationUpstreamedCandidateFixture(t *testing.T) (*store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	setupGitRepo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "ops.txt"), []byte("already-present\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, "ops.txt")
	gitCommit(t, dir, "add ops")
	baseCommit := gitRevParse(t, dir, "HEAD")
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	feature, err := s.AddFeature(store.AddFeatureInput{Title: "SECRET_GATE_DO_NOT_LEAK", Request: "operation upstreamed candidate"})
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
	return s, feature.Slug
}

func buildHunkOverlapFixture(t *testing.T, overlap bool) (*store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	setupGitRepo(t, dir)
	base := "line1\nline2\nline3\n"
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, "existing.txt")
	gitCommit(t, dir, "base")
	baseCommit := gitRevParse(t, dir, "HEAD")
	featureContent := "line1\nSECRET_OVERLAP_DO_NOT_LEAK\nline3\n"
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte(featureContent), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := gitOutput(t, dir, "diff", "--no-color", "HEAD")
	upstreamContent := "line1\nupstream-change\nline3\n"
	if !overlap {
		upstreamContent = "line1\nline2\nline3\nupstream-tail\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte(upstreamContent), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, "existing.txt")
	gitCommit(t, dir, "upstream change")
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
	return s, feature.Slug
}

func hasEvidenceKind(entries []store.ReconcileEvidence, kind store.ReconcileEvidenceKind) bool {
	for _, e := range entries {
		if e.EvidenceKind == kind {
			return true
		}
	}
	return false
}

func hasEvidenceKindReason(entries []store.ReconcileEvidence, kind store.ReconcileEvidenceKind, reason string) bool {
	for _, e := range entries {
		if e.EvidenceKind == kind && e.ReasonCode == reason {
			return true
		}
	}
	return false
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
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

func TestUpstreamedConfirmationGateKeepsConfirmedReverseApply(t *testing.T) {
	s, slug := buildReverseApplyConfirmationFixture(t)

	results, err := RunReconcile(context.Background(), s, []string{slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Outcome != store.ReconcileUpstreamed || results[0].ReviewVerdict != "confirmed-upstreamed" {
		t.Fatalf("expected confirmed upstreamed verdict, got outcome=%s review=%q", results[0].Outcome, results[0].ReviewVerdict)
	}
	entries, err := store.LoadReconcileEvidence(s, slug)
	if err != nil {
		t.Fatal(err)
	}
	var gateEvidence store.ReconcileEvidence
	for _, entry := range entries {
		if entry.EvidenceKind == store.EvidenceKindManualReview && entry.ReasonCode == "confirmed-upstreamed" {
			gateEvidence = entry
			break
		}
	}
	if gateEvidence.AttemptID == "" {
		t.Fatalf("confirmation gate evidence not found: %+v", entries)
	}
	revisions, err := store.LoadReconcileRevisions(s, slug)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 1 || revisions[0].EvidenceAttemptID != gateEvidence.AttemptID {
		t.Fatalf("revision entry does not link to gate evidence attempt %q: %+v", gateEvidence.AttemptID, revisions)
	}
	wantUpstreamCommit := gitRevParse(t, s.Root, "HEAD")
	if gateEvidence.UpstreamCommit != wantUpstreamCommit {
		t.Fatalf("gate evidence upstream commit %q does not match HEAD", gateEvidence.UpstreamCommit)
	}
	foundUpstreamRef := false
	for _, ref := range revisions[0].ValidationRefs {
		if ref.Kind == "upstream-commit" && ref.Value == wantUpstreamCommit && ref.Result == "referenced" {
			foundUpstreamRef = true
		}
	}
	if !foundUpstreamRef {
		t.Fatalf("revision entry does not include upstream commit ref %q: %+v", wantUpstreamCommit, revisions[0].ValidationRefs)
	}
}

func TestUpstreamedConfirmationGateBlocksUnconfirmedOperationMatch(t *testing.T) {
	s, slug := buildOperationUpstreamedCandidateFixture(t)

	results, err := RunReconcile(context.Background(), s, []string{slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Outcome != store.ReconcileBlocked || results[0].ReviewVerdict != "rejected-upstreamed" {
		t.Fatalf("expected blocked rejected upstreamed candidate, got outcome=%s review=%q", results[0].Outcome, results[0].ReviewVerdict)
	}
	entries, err := store.LoadReconcileEvidence(s, slug)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEvidenceKindReason(entries, store.EvidenceKindManualReview, "missing-upstream-commit-ref") {
		t.Fatalf("confirmation gate rejection evidence not found: %+v", entries)
	}
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatal(err)
	}
	if status.State == store.StateUpstreamMerged || status.State != store.StateBlocked {
		t.Fatalf("rejected upstreamed candidate must persist blocked state, got %q", status.State)
	}
	if strings.Contains(mustReadFile(t, s.ReconcileEvidencePath(slug)), "SECRET_GATE_DO_NOT_LEAK") {
		t.Fatal("confirmation gate evidence leaked secret metadata")
	}
}

func TestRevisionPassLogAppendedForConfirmationGate(t *testing.T) {
	s, slug := buildReverseApplyConfirmationFixture(t)

	results, err := RunReconcile(context.Background(), s, []string{slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results[0].Revisions) != 1 {
		t.Fatalf("expected one surfaced revision entry, got %+v", results[0].Revisions)
	}
	revisions, err := store.LoadReconcileRevisions(s, slug)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 1 || revisions[0].EntryID == "" || revisions[0].EvidenceAttemptID == "" || revisions[0].ReviewVerdict != store.ReviewVerdictConfirmed {
		t.Fatalf("unexpected revision entries: %+v", revisions)
	}
	if got := store.ComputeRevisionID(revisions[0]); got != revisions[0].EntryID {
		t.Fatalf("revision entry_id not stable: got %s want %s", got, revisions[0].EntryID)
	}
	secret := "SECRET_REVISION_METADATA_DO_NOT_LEAK"
	if strings.Contains(mustReadFile(t, s.ReconcileRevisionsPath(slug)), secret) {
		t.Fatal("revision entry leaked secret metadata")
	}
	if strings.Contains(mustReadFile(t, s.ReconcileEvidencePath(slug)), secret) {
		t.Fatal("evidence entry leaked secret metadata")
	}
}

func TestHunkOverlapEvidenceForModifiedPath(t *testing.T) {
	s, slug := buildHunkOverlapFixture(t, true)

	results, err := RunReconcile(context.Background(), s, []string{slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result")
	}
	entries, err := store.LoadReconcileEvidence(s, slug)
	if err != nil {
		t.Fatal(err)
	}
	var hunkEvidence store.ReconcileEvidence
	for _, entry := range entries {
		if entry.EvidenceKind == store.EvidenceKindHunkOverlap && entry.ReasonCode == string(HunkOverlapEditOverlap) {
			hunkEvidence = entry
			break
		}
	}
	if hunkEvidence.AttemptID == "" {
		t.Fatalf("hunk-overlap edit-overlap evidence not found: %+v", entries)
	}
	hunkJSON, err := json.Marshal(hunkEvidence)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(hunkJSON, []byte("nearby-window=3")) {
		t.Fatalf("expected default nearby-window=3 in hunk-overlap JSON: %s", hunkJSON)
	}
	if strings.Contains(mustReadFile(t, s.ReconcileEvidencePath(slug)), "SECRET_OVERLAP_DO_NOT_LEAK") {
		t.Fatal("hunk-overlap evidence leaked source body")
	}
}

func TestHunkOverlapSkippedForAllNewFiles(t *testing.T) {
	s, slug := buildFileNoveltyFixture(t, "all-new-no-overlap", map[string]string{"new.txt": "brand new\n"})

	_, err := RunReconcile(context.Background(), s, []string{slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := store.LoadReconcileEvidence(s, slug)
	if err != nil {
		t.Fatal(err)
	}
	if hasEvidenceKind(entries, store.EvidenceKindHunkOverlap) {
		t.Fatalf("did not expect hunk-overlap evidence for all-new files: %+v", entries)
	}
}

func TestReconcileResultJSONOmitsWaveBetaFieldsWhenNoGateRevisionOrOverlap(t *testing.T) {
	s, slug := buildFileNoveltyFixture(t, "byte identity beta", map[string]string{"new.txt": "brand new\n"})

	results, err := RunReconcile(context.Background(), s, []string{slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(results[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"review_verdict"`, `"revisions"`, `"hunk-overlap"`, `"confirmation-gate"`} {
		if bytes.Contains(data, []byte(forbidden)) {
			t.Fatalf("unexpected Wave beta no-op field %s in JSON: %s", forbidden, data)
		}
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

func TestReconcileWritesPathRestructureEvidenceAndBlockedCategory(t *testing.T) {
	s, slug, sourceSecret := buildPathRestructureFixture(t, nil)

	results, err := RunReconcile(context.Background(), s, []string{slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	if results[0].Outcome != store.ReconcileBlocked || results[0].BlockedCategory != string(BlockedCategoryStructuralConflict) {
		t.Fatalf("expected structural blocked result, got outcome=%s category=%q result=%+v", results[0].Outcome, results[0].BlockedCategory, results[0])
	}

	entries, err := store.LoadReconcileEvidence(s, slug)
	if err != nil {
		t.Fatal(err)
	}
	var pathEvidence store.ReconcileEvidence
	for _, entry := range entries {
		if entry.EvidenceKind == store.EvidenceKindPathRestructure {
			pathEvidence = entry
			break
		}
	}
	if pathEvidence.AttemptID == "" {
		t.Fatalf("path-restructure evidence not found: %+v", entries)
	}
	if pathEvidence.ReasonCode != string(PathRestructurePrefixSplit) || pathEvidence.Confidence != store.EvidenceConfidenceHigh {
		t.Fatalf("unexpected path-restructure evidence: %+v", pathEvidence)
	}
	ops := strings.Join(pathEvidence.MatchedOperations, "\n")
	for _, want := range []string{"old_prefix=src/", "candidate_prefixes=app/|backend/", "prefix_split_min_files=3", "prefix_split_min_prefixes=2", "prefix_move_min_files=5"} {
		if !strings.Contains(ops, want) {
			t.Fatalf("path-restructure operations missing %q: %s", want, ops)
		}
	}
	if got := strings.Join(pathEvidence.MatchedPaths, ","); got != "src/SECRET_PATH_COMPONENT_ALLOWED/feature.go" {
		t.Fatalf("affected paths got %q", got)
	}
	if !hasEvidenceKindReason(entries, store.EvidenceKindBlockedClassification, string(BlockedCategoryStructuralConflict)) {
		t.Fatalf("blocked-classification did not consume path-restructure evidence: %+v", entries)
	}
	if strings.Contains(mustReadFile(t, s.ReconcileEvidencePath(slug)), sourceSecret) {
		t.Fatal("path-restructure evidence leaked source content")
	}
}

func TestReconcilePathRestructureThresholdOverrideSuppressesEvidence(t *testing.T) {
	s, slug, _ := buildPathRestructureFixture(t, func(cfg *store.Config) {
		cfg.PathRestructurePrefixSplitMinFiles = 4
		cfg.PathRestructurePrefixSplitMinPrefixes = 2
		cfg.PathRestructurePrefixMoveMinFiles = 5
	})

	results, err := RunReconcile(context.Background(), s, []string{slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	if hasEvidenceKind(results[0].Evidence, store.EvidenceKindPathRestructure) {
		t.Fatalf("path-restructure evidence should respect config threshold override: %+v", results[0].Evidence)
	}
	entries, err := store.LoadReconcileEvidence(s, slug)
	if err != nil {
		t.Fatal(err)
	}
	if hasEvidenceKind(entries, store.EvidenceKindPathRestructure) {
		t.Fatalf("persisted path-restructure evidence should be suppressed by threshold override: %+v", entries)
	}
}

func buildPathRestructureFixture(t *testing.T, configure func(*store.Config)) (*store.Store, string, string) {
	t.Helper()
	dir := t.TempDir()
	setupGitRepo(t, dir)
	baseFiles := map[string]string{
		"src/SECRET_PATH_COMPONENT_ALLOWED/feature.go": "package feature\n\nconst Value = \"base\"\n",
		"src/a.go": "package feature\n\nconst A = \"a\"\n",
		"src/b.go": "package feature\n\nconst B = \"b\"\n",
	}
	for path, content := range baseFiles {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		gitAdd(t, dir, path)
	}
	gitCommit(t, dir, "base source tree")
	baseCommit := gitRevParse(t, dir, "HEAD")

	sourceSecret := "SECRET_PATH_RESTRUCTURE_SOURCE_DO_NOT_LEAK"
	featurePath := filepath.Join(dir, "src/SECRET_PATH_COMPONENT_ALLOWED/feature.go")
	if err := os.WriteFile(featurePath, []byte("package feature\n\nconst Value = \""+sourceSecret+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := gitOutput(t, dir, "diff", "--no-color", "HEAD")

	gitRun(t, dir, "reset", "--hard", "HEAD")
	if err := os.MkdirAll(filepath.Join(dir, "app/SECRET_PATH_COMPONENT_ALLOWED"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "backend"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "mv", "src/SECRET_PATH_COMPONENT_ALLOWED/feature.go", "app/SECRET_PATH_COMPONENT_ALLOWED/feature.go")
	gitRun(t, dir, "mv", "src/a.go", "app/a.go")
	gitRun(t, dir, "mv", "src/b.go", "backend/b.go")
	gitCommit(t, dir, "split source tree")

	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	if configure != nil {
		cfg, err := s.LoadConfig()
		if err != nil {
			t.Fatal(err)
		}
		configure(&cfg)
		if err := s.SaveConfig(cfg); err != nil {
			t.Fatal(err)
		}
	}
	feature, err := s.AddFeature(store.AddFeatureInput{Title: "path restructure", Request: "detect path restructure"})
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
	return s, feature.Slug, sourceSecret
}
