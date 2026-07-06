package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestAuditRetirementCLIHumanJSONReadOnly(t *testing.T) {
	dir := t.TempDir()
	setupCLIGit(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := s.AddFeature(store.AddFeatureInput{Title: "parent", Slug: "parent", Request: "parent"})
	f.State = store.StateUpstreamMerged
	f.Reconcile.Outcome = store.ReconcileUpstreamed
	f.Reconcile.ReviewVerdict = "confirmed-upstreamed"
	_ = s.SaveFeatureStatus(f)
	before, _ := s.LoadFeatureStatus("parent")
	out, _, code := runCmd("reconcile", "audit-retirement", "--path", dir, "parent")
	if code != 0 {
		t.Fatalf("human failed: %s", out)
	}
	if !strings.Contains(out, "retirement audit: parent") || !strings.Contains(out, "feature state: upstream_merged") {
		t.Fatalf("unexpected human output %q", out)
	}
	jsonOut, _, code := runCmd("reconcile", "audit-retirement", "--path", dir, "--json", "parent")
	if code != 0 {
		t.Fatalf("json failed: %s", jsonOut)
	}
	var report struct {
		Slug     string `json:"slug"`
		Findings []any  `json:"findings"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &report); err != nil {
		t.Fatalf("bad json: %v\n%s", err, jsonOut)
	}
	after, _ := s.LoadFeatureStatus("parent")
	if before.State != after.State || before.Reconcile.Outcome != after.Reconcile.Outcome || before.Reconcile.ReviewVerdict != after.Reconcile.ReviewVerdict {
		t.Fatalf("audit mutated status")
	}
}

func TestReconcileConfirmedUpstreamedRunsRetirementAudit(t *testing.T) {
	dir, slug := cliConfirmedUpstreamedFixture(t)
	out, errOut, err := runCLIForEvidence("reconcile", "--path", dir, "--allow-dirty", "--upstream-ref", "HEAD", slug)
	if err != nil {
		t.Fatalf("reconcile failed: %v\nstderr=%s\nstdout=%s", err, errOut, out)
	}
	if !strings.Contains(out, "retirement audit: "+slug) {
		t.Fatalf("missing auto audit output:\n%s", out)
	}
	s, _ := store.Open(dir)
	revs, err := store.LoadReconcileRevisions(s, slug)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range revs {
		if r.ActionTaken == store.ReconcileActionCleanupNeeded {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected cleanup-needed revision, got %#v", revs)
	}
}

func TestReconcileConfirmedUpstreamedJSONRunsRetirementAudit(t *testing.T) {
	dir, slug := cliConfirmedUpstreamedFixture(t)
	out, errOut, err := runCLIForEvidence("reconcile", "--path", dir, "--allow-dirty", "--upstream-ref", "HEAD", "--format", "json", slug)
	if err != nil {
		t.Fatalf("json reconcile failed: %v\nstderr=%s\nstdout=%s", err, errOut, out)
	}
	var results []struct {
		Slug            string `json:"slug"`
		ReviewVerdict   string `json:"review_verdict"`
		RetirementAudit struct {
			Findings []any `json:"findings"`
		} `json:"retirement_audit"`
		Revisions []struct {
			ActionTaken string `json:"action_taken"`
		} `json:"revisions"`
	}
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	if len(results) != 1 || results[0].Slug != slug || results[0].ReviewVerdict != "confirmed-upstreamed" {
		t.Fatalf("bad result envelope: %#v", results)
	}
	if len(results[0].RetirementAudit.Findings) == 0 {
		t.Fatalf("expected retirement audit findings in json output: %#v", results[0])
	}
	foundRuntimeRevision := false
	for _, rev := range results[0].Revisions {
		if rev.ActionTaken == string(store.ReconcileActionCleanupNeeded) {
			foundRuntimeRevision = true
		}
	}
	if !foundRuntimeRevision {
		t.Fatalf("expected cleanup-needed runtime revision in json output: %#v", results[0].Revisions)
	}
	s, _ := store.Open(dir)
	revs, err := store.LoadReconcileRevisions(s, slug)
	if err != nil {
		t.Fatal(err)
	}
	foundPersistedRevision := false
	for _, r := range revs {
		if r.ActionTaken == store.ReconcileActionCleanupNeeded {
			foundPersistedRevision = true
		}
	}
	if !foundPersistedRevision {
		t.Fatalf("expected persisted cleanup-needed revision, got %#v", revs)
	}
}

func TestConfirmUpstreamedRunsRetirementAudit(t *testing.T) {
	dir := t.TempDir()
	setupCLIGit(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := s.AddFeature(store.AddFeatureInput{Title: "parent", Slug: "parent", Request: "parent"})
	f.State = store.StateUpstreamMerged
	f.Reconcile.Outcome = store.ReconcileUpstreamed
	f.Reconcile.ReviewVerdict = "confirmed-upstreamed"
	if err := s.SaveFeatureStatus(f); err != nil {
		t.Fatal(err)
	}
	out, errOut, err := runCLIForEvidence("reconcile", "confirm-upstreamed", "--path", dir, "--format", "json", "parent")
	if err != nil {
		t.Fatalf("confirm-upstreamed failed: %v\nstderr=%s\nstdout=%s", err, errOut, out)
	}
	var result struct {
		Slug            string `json:"slug"`
		ReviewVerdict   string `json:"review_verdict"`
		RetirementAudit struct {
			Findings []any `json:"findings"`
		} `json:"retirement_audit"`
		Revisions []struct {
			ActionTaken string `json:"action_taken"`
		} `json:"revisions"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	if result.Slug != "parent" || result.ReviewVerdict != "confirmed-upstreamed" || len(result.RetirementAudit.Findings) == 0 {
		t.Fatalf("bad confirm-upstreamed result: %#v", result)
	}
	if len(result.Revisions) == 0 || result.Revisions[0].ActionTaken != string(store.ReconcileActionCleanupNeeded) {
		t.Fatalf("expected cleanup-needed revision in result: %#v", result.Revisions)
	}
	jsonAliasOut, errOut, err := runCLIForEvidence("reconcile", "confirm-upstreamed", "--path", dir, "--json", "parent")
	if err != nil {
		t.Fatalf("confirm-upstreamed --json failed: %v\nstderr=%s\nstdout=%s", err, errOut, jsonAliasOut)
	}
	if !strings.Contains(jsonAliasOut, `"retirement_audit"`) {
		t.Fatalf("expected --json alias to emit audit payload:\n%s", jsonAliasOut)
	}
	revs, err := store.LoadReconcileRevisions(s, "parent")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range revs {
		if r.ActionTaken == store.ReconcileActionCleanupNeeded {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected persisted cleanup-needed revision, got %#v", revs)
	}
}

func cliConfirmedUpstreamedFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	gitInitTestRepo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "existing.txt")
	gitRun(t, dir, "commit", "-m", "base")
	baseCommit := gitHead(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("base\nfeature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := gitOut(t, dir, "diff", "--no-color", "HEAD") + "\n"
	gitRun(t, dir, "add", "existing.txt")
	gitRun(t, dir, "commit", "-m", "upstream has feature")
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	feature, err := s.AddFeature(store.AddFeatureInput{Title: "confirmed upstreamed", Slug: "confirmed-upstreamed", Request: "confirmed"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MarkFeatureState(feature.Slug, store.StateApplied, "apply", ""); err != nil {
		t.Fatal(err)
	}
	st, _ := s.LoadFeatureStatus(feature.Slug)
	st.Apply.BaseCommit = baseCommit
	st.Apply.HasPatch = true
	_ = s.SaveFeatureStatus(st)
	if err := s.WriteArtifact(feature.Slug, "post-apply.patch", patch); err != nil {
		t.Fatal(err)
	}
	child, err := s.AddFeature(store.AddFeatureInput{Title: "child", Slug: "child", Request: "child"})
	if err != nil {
		t.Fatal(err)
	}
	child.State = store.StateApplied
	child.DependsOn = []store.Dependency{{Slug: feature.Slug, Kind: store.DependencyKindHard}}
	if err := s.SaveFeatureStatus(child); err != nil {
		t.Fatal(err)
	}
	return dir, feature.Slug
}

func setupCLIGit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{{"init"}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v %s", err, out)
	}
}
