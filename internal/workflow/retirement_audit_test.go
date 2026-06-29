package workflow

import (
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestAuditRetirementChecks(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	parent, _ := s.AddFeature(store.AddFeatureInput{Title: "parent", Slug: "parent", Request: "parent"})
	child, _ := s.AddFeature(store.AddFeatureInput{Title: "child", Slug: "child", Request: "child"})
	parent.State = store.StateUpstreamMerged
	parent.Reconcile.Outcome = store.ReconcileUpstreamed
	parent.Reconcile.ReviewVerdict = "confirmed-upstreamed"
	parent.Apply.BaseCommit = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	_ = s.SaveFeatureStatus(parent)
	child.State = store.StateApplied
	child.DependsOn = []store.Dependency{{Slug: "parent", Kind: store.DependencyKindHard, SatisfiedBy: "badbadbadbadbadbadbadbadbadbadbadbadbadb"}}
	_ = s.SaveFeatureStatus(child)
	report, err := AuditRetirement(s, "parent")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"reachable-sha", "children-affected", "revision-log"} {
		if !hasAuditCheck(report, want) {
			t.Fatalf("missing %s in %#v", want, report.Findings)
		}
	}
	before, _ := s.LoadFeatureStatus("parent")
	_, _ = AuditRetirement(s, "parent")
	after, _ := s.LoadFeatureStatus("parent")
	if before.State != after.State || before.Reconcile.Outcome != after.Reconcile.Outcome {
		t.Fatalf("audit mutated status")
	}
}

func TestAppendRetirementCleanupRevisions(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	s, _ := store.Init(dir)
	f, _ := s.AddFeature(store.AddFeatureInput{Title: "parent", Slug: "parent", Request: "parent"})
	f.State = store.StateUpstreamMerged
	f.Reconcile.Outcome = store.ReconcileUpstreamed
	_ = s.SaveFeatureStatus(f)
	report := RetirementAuditReport{Slug: "parent", Findings: []RetirementAuditFinding{{Check: "children-affected", Feature: "child", Action: "cleanup-needed"}}}
	if _, err := AppendRetirementCleanupRevisions(s, report); err != nil {
		t.Fatal(err)
	}
	revs, err := store.LoadReconcileRevisions(s, "parent")
	if err != nil {
		t.Fatal(err)
	}
	if len(revs) != 1 || revs[0].ActionTaken != store.ReconcileActionCleanupNeeded {
		t.Fatalf("bad revs %#v", revs)
	}
}

func hasAuditCheck(r RetirementAuditReport, check string) bool {
	for _, f := range r.Findings {
		if f.Check == check {
			return true
		}
	}
	return false
}
