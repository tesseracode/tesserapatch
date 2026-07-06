package studyvalidator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateReportsMalformedJSONLWithFileAndLine(t *testing.T) {
	dir := studyFixture(t)
	appendFile(t, filepath.Join(dir, "features.jsonl"), "{bad\n")
	r, _ := Validate(dir)
	if !hasIssue(r.Errors, "features.jsonl", 2, "invalid") {
		t.Fatalf("missing malformed line issue: %#v", r.Errors)
	}
}
func TestValidateMismatchedStudyIDAndFeatureCount(t *testing.T) {
	dir := studyFixture(t)
	write(t, filepath.Join(dir, "metrics.json"), `{"study_id":"other","totals":{"features_total":2,"hunks_total":1,"patches_total":1},"ground_truth_distribution":{"reapplies_clean":1}}`)
	r, _ := Validate(dir)
	if !hasIssue(r.Errors, "metrics.json", 0, "study_id") || !hasIssue(r.Errors, "metrics.json", 0, "features_total") {
		t.Fatalf("issues %#v", r.Errors)
	}
}
func TestValidateAggregateCountContradiction(t *testing.T) {
	dir := studyFixture(t)
	write(t, filepath.Join(dir, "metrics.json"), `{"study_id":"s","totals":{"features_total":1,"hunks_total":9,"patches_total":1},"ground_truth_distribution":{"reapplies_clean":2}}`)
	r, _ := Validate(dir)
	if !hasIssue(r.Errors, "metrics.json", 0, "hunks_total") || !hasIssue(r.Errors, "metrics.json", 0, "ground_truth_distribution") {
		t.Fatalf("issues %#v", r.Errors)
	}
}
func TestValidateRawPostReviewFinalPhaseDistinction(t *testing.T) {
	dir := studyFixture(t)
	r, _ := Validate(dir)
	if !hasIssue(r.Warnings, "metrics.json", 0, "verdicts_phase") {
		t.Fatalf("expected phase warning %#v", r.Warnings)
	}
}
func TestValidateMissingNotesWarningOldErrorNew(t *testing.T) {
	dir := studyFixture(t)
	if err := os.Remove(filepath.Join(dir, "local-notes.md")); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "features.jsonl"), `{"study_id":"s","slug":"a","ground_truth":"false_positive_upstreamed"}`+"\n")
	r, _ := Validate(dir)
	if !hasIssue(r.Warnings, "local-notes.md", 0, "old study") {
		t.Fatalf("warnings %#v errors %#v", r.Warnings, r.Errors)
	}
	dir = studyFixture(t)
	if err := os.Remove(filepath.Join(dir, "local-notes.md")); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "features.jsonl"), `{"study_id":"s","slug":"a","ground_truth":"false_positive_upstreamed"}`+"\n")
	write(t, filepath.Join(dir, "study.json"), `{"study_id":"s","feature_count":1,"tpatch_version":"0.10.1"}`)
	write(t, filepath.Join(dir, "metrics.json"), `{"study_id":"s","totals":{"features_total":1,"hunks_total":1,"patches_total":1},"verdict_post_review":{"false_positive_upstreamed":1}}`)
	r, _ = Validate(dir)
	if !hasIssue(r.Errors, "local-notes.md", 0, "slug=a") || !hasIssue(r.Errors, "local-notes.md", 0, "false_positive_upstreamed") {
		t.Fatalf("expected new-study error %#v", r.Errors)
	}
}

func TestValidateCorrectionLinksAllRevisions(t *testing.T) {
	dir := correctedStudyFixture(t)
	if err := os.Remove(filepath.Join(dir, "local-notes.md")); err != nil {
		t.Fatal(err)
	}
	writeRevisionRefs(t, dir, "alpha", "beta", "gamma")
	r, _ := Validate(dir)
	if len(r.Errors) != 0 {
		t.Fatalf("expected revision-linked corrected verdicts to pass, got %#v", r.Errors)
	}
}

func TestValidateCorrectionLinksRevisionsAndNotes(t *testing.T) {
	dir := correctedStudyFixture(t)
	writeRevisionRefs(t, dir, "alpha", "beta")
	write(t, filepath.Join(dir, "local-notes.md"), "## gamma\nreview notes for gamma\n")
	r, _ := Validate(dir)
	if len(r.Errors) != 0 {
		t.Fatalf("expected mixed revision/notes links to pass, got %#v", r.Errors)
	}
}

func TestValidateCorrectionLinksReportsEachUnlinkedFeature(t *testing.T) {
	dir := correctedStudyFixture(t)
	if err := os.Remove(filepath.Join(dir, "local-notes.md")); err != nil {
		t.Fatal(err)
	}
	writeRevisionRefs(t, dir, "alpha")
	r, _ := Validate(dir)
	if len(r.Errors) != 2 {
		t.Fatalf("expected exactly 2 unlinked correction errors, got %#v", r.Errors)
	}
	if !hasIssue(r.Errors, "local-notes.md", 0, "slug=beta") || !hasIssue(r.Errors, "local-notes.md", 0, "false_negative") {
		t.Fatalf("missing beta false_negative error: %#v", r.Errors)
	}
	if !hasIssue(r.Errors, "local-notes.md", 0, "slug=gamma") || !hasIssue(r.Errors, "local-notes.md", 0, "false_positive") {
		t.Fatalf("missing gamma false_positive error: %#v", r.Errors)
	}
}

func TestValidateCorrectionLinksZeroCorrectedNoNotesNoError(t *testing.T) {
	dir := studyFixture(t)
	if err := os.Remove(filepath.Join(dir, "local-notes.md")); err != nil {
		t.Fatal(err)
	}
	r, _ := Validate(dir)
	if len(r.Errors) != 0 {
		t.Fatalf("expected zero corrected verdicts to need no links, got %#v", r.Errors)
	}
}

func TestValidateRunsOnT3CodeStudyArtifacts(t *testing.T) {
	r, err := Validate(filepath.Join("..", "..", "..", "docs", "state-of-the-art", "case-studies", "t3code-upstream-v0.0.23-2026-05"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Counts.Features == 0 || r.Counts.Hunks == 0 || r.Counts.Patches == 0 {
		t.Fatalf("bad counts %#v", r.Counts)
	}
}

func studyFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "study.json"), `{"study_id":"s","feature_count":1,"tpatch_version":"0.6.1"}`)
	write(t, filepath.Join(dir, "features.jsonl"), `{"study_id":"s","slug":"a","ground_truth":"reapplies_clean","final_state":"applied","reconcile_verdict":"blocked"}`+"\n")
	write(t, filepath.Join(dir, "hunks.jsonl"), `{"study_id":"s","hunk_id":"h1"}`+"\n")
	write(t, filepath.Join(dir, "patches.jsonl"), `{"study_id":"s","patch_id":"p1"}`+"\n")
	write(t, filepath.Join(dir, "metrics.json"), `{"study_id":"s","totals":{"features_total":1,"hunks_total":1,"patches_total":1},"verdicts":{"blocked":1},"ground_truth_distribution":{"reapplies_clean":1}}`)
	write(t, filepath.Join(dir, "summary.md"), "summary")
	write(t, filepath.Join(dir, "local-notes.md"), "notes")
	return dir
}

func correctedStudyFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "study.json"), `{"study_id":"s","feature_count":3,"tpatch_version":"0.10.1"}`)
	write(t, filepath.Join(dir, "features.jsonl"),
		`{"study_id":"s","slug":"alpha","ground_truth":"false_positive","final_state":"applied","reconcile_verdict":"upstreamed"}`+"\n"+
			`{"study_id":"s","slug":"beta","ground_truth":"false_negative","final_state":"upstream_merged","reconcile_verdict":"blocked"}`+"\n"+
			`{"study_id":"s","slug":"gamma","ground_truth":"false_positive","final_state":"applied","reconcile_verdict":"upstreamed"}`+"\n")
	write(t, filepath.Join(dir, "hunks.jsonl"), `{"study_id":"s","hunk_id":"h1"}`+"\n")
	write(t, filepath.Join(dir, "patches.jsonl"), `{"study_id":"s","patch_id":"p1"}`+"\n")
	write(t, filepath.Join(dir, "metrics.json"), `{"study_id":"s","totals":{"features_total":3,"hunks_total":1,"patches_total":1},"ground_truth_distribution":{"false_positive":2,"false_negative":1}}`)
	write(t, filepath.Join(dir, "summary.md"), "summary")
	write(t, filepath.Join(dir, "local-notes.md"), "notes")
	return dir
}

func writeRevisionRefs(t *testing.T, dir string, slugs ...string) {
	t.Helper()
	var b strings.Builder
	for _, slug := range slugs {
		b.WriteString(`{"entry_id":"rr_`)
		b.WriteString(slug)
		b.WriteString(`","feature_slug":"`)
		b.WriteString(slug)
		b.WriteString(`","evidence_attempt_id":"re_`)
		b.WriteString(slug)
		b.WriteString(`"}`)
		b.WriteByte('\n')
	}
	write(t, filepath.Join(dir, "reconcile-revisions.jsonl"), b.String())
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
func appendFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
}
func hasIssue(xs []Issue, file string, line int, sub string) bool {
	for _, x := range xs {
		if x.File == file && x.Line == line && strings.Contains(x.Message, sub) {
			return true
		}
	}
	return false
}
