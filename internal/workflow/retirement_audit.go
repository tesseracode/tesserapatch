package workflow

import (
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

type RetirementAuditFinding struct {
	Check    string `json:"check"`
	Severity string `json:"severity"`
	Feature  string `json:"feature"`
	RefKind  string `json:"ref_kind,omitempty"`
	Ref      string `json:"ref,omitempty"`
	Message  string `json:"message"`
	Action   string `json:"action"`
}

type RetirementAuditReport struct {
	Slug             string                   `json:"slug"`
	FeatureState     store.FeatureState       `json:"feature_state"`
	ReconcileOutcome store.ReconcileOutcome   `json:"reconcile_outcome,omitempty"`
	ReviewVerdict    string                   `json:"review_verdict,omitempty"`
	Findings         []RetirementAuditFinding `json:"findings"`
}

func AuditRetirement(s *store.Store, slug string) (RetirementAuditReport, error) {
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return RetirementAuditReport{}, err
	}
	report := RetirementAuditReport{Slug: slug, FeatureState: status.State, ReconcileOutcome: status.Reconcile.Outcome, ReviewVerdict: status.Reconcile.ReviewVerdict, Findings: []RetirementAuditFinding{}}
	add := func(check, feature, kind, ref, msg string) {
		report.Findings = append(report.Findings, RetirementAuditFinding{Check: check, Severity: "cleanup-needed", Feature: feature, RefKind: kind, Ref: ref, Message: msg, Action: "cleanup-needed"})
	}
	if status.State != store.StateUpstreamMerged || status.Reconcile.Outcome != store.ReconcileUpstreamed || (status.Reconcile.ReviewVerdict != "" && status.Reconcile.ReviewVerdict != "confirmed-upstreamed") {
		add("retirement-confirmed", slug, "state", string(status.State), "feature state and reconcile evidence do not agree that retirement was confirmed")
	}
	if status.Apply.BaseCommit != "" && !isReachableFromHEAD(s, status.Apply.BaseCommit) {
		add("reachable-sha", slug, store.FeatureRefKindBaseCommit, abbrevRef(status.Apply.BaseCommit), "retired feature base_commit is no longer reachable from HEAD")
	}
	features, err := s.ListFeatures()
	if err != nil {
		return report, err
	}
	broken, _ := store.CollectBrokenRefs(s)
	for _, child := range features {
		if child.Slug == slug {
			continue
		}
		for _, dep := range child.DependsOn {
			if dep.Slug != slug {
				continue
			}
			add("children-affected", child.Slug, "dependency", slug, "child depends on retired parent and may need dependency cleanup")
			if dep.SatisfiedBy == "" {
				add("child-labels", child.Slug, store.FeatureRefKindSatisfiedBy, "", "child dependency on retired parent has no satisfied_by SHA")
			} else if !isReachableFromHEAD(s, dep.SatisfiedBy) {
				add("reachable-sha", child.Slug, store.FeatureRefKindSatisfiedBy, abbrevRef(dep.SatisfiedBy), "child satisfied_by SHA is no longer reachable from HEAD")
			}
			if labels, lerr := ComposeLabels(s, child.Slug); lerr == nil && hasLabel(labels, store.LabelDependentBroken) {
				add("dependent-broken", child.Slug, "label", string(store.LabelDependentBroken), "dependent-broken remains after parent retirement and needs review")
			}
		}
	}
	for feature, refs := range broken {
		for _, ref := range refs {
			if ref.Feature == slug || dependsOn(features, feature, slug) {
				add("dependent-broken", feature, ref.Kind, abbrevRef(ref.SHA), "dependent-broken is justified by unreachable recorded SHA")
			}
		}
	}
	revs, _ := store.LoadReconcileRevisions(s, slug)
	hasRetirementRevision := false
	for _, r := range revs {
		if r.ActionTaken == store.ReconcileActionConfirmedRetired || r.ActionTaken == store.ReconcileActionCleanupNeeded {
			hasRetirementRevision = true
			break
		}
	}
	if !hasRetirementRevision {
		add("revision-log", slug, "reconcile-revisions", "", "retired feature has no retirement revision-pass entry")
	}
	sort.Slice(report.Findings, func(i, j int) bool {
		a, b := report.Findings[i], report.Findings[j]
		if a.Check != b.Check {
			return a.Check < b.Check
		}
		if a.Feature != b.Feature {
			return a.Feature < b.Feature
		}
		if a.RefKind != b.RefKind {
			return a.RefKind < b.RefKind
		}
		return a.Ref < b.Ref
	})
	return report, nil
}

func dependsOn(features []store.FeatureStatus, child, parent string) bool {
	for _, f := range features {
		if f.Slug != child {
			continue
		}
		for _, d := range f.DependsOn {
			if d.Slug == parent {
				return true
			}
		}
	}
	return false
}
func isReachableFromHEAD(s *store.Store, sha string) bool {
	ok, err := gitutil.IsAncestor(s.Root, sha, "HEAD")
	return err == nil && ok
}
func abbrevRef(ref string) string {
	if len(ref) > 12 {
		return ref[:12]
	}
	return ref
}
func RetirementAuditLines(report RetirementAuditReport) []string {
	lines := []string{"retirement audit: " + report.Slug, "  feature state: " + string(report.FeatureState)}
	if len(report.Findings) == 0 {
		return append(lines, "  findings: none")
	}
	for _, f := range report.Findings {
		parts := []string{f.Check + ":"}
		if f.Feature != "" {
			parts = append(parts, f.Feature)
		}
		if f.RefKind != "" {
			parts = append(parts, f.RefKind)
		}
		if f.Ref != "" {
			parts = append(parts, f.Ref)
		}
		parts = append(parts, f.Action)
		lines = append(lines, "  "+strings.Join(parts, " "))
	}
	return lines
}

func AppendRetirementCleanupRevisions(s *store.Store, report RetirementAuditReport) ([]store.ReconcileRevision, error) {
	status, err := s.LoadFeatureStatus(report.Slug)
	if err != nil {
		return nil, err
	}
	out := make([]store.ReconcileRevision, 0, len(report.Findings))
	for _, f := range report.Findings {
		entry := store.ReconcileRevision{SchemaVersion: store.ReconcileRevisionSchemaVersion, FeatureSlug: report.Slug, EvidenceAttemptID: "", RawReconcileVerdict: string(status.Reconcile.Outcome), ReviewVerdict: store.ReviewVerdictDeferred, FinalFeatureState: status.State, ActionTaken: store.ReconcileActionCleanupNeeded, ReasonCode: "retirement-audit-" + f.Check, ValidationRefs: []store.ValidationRef{{Kind: "retirement-audit", Value: f.Feature + ":" + f.RefKind + ":" + f.Ref, Result: "cleanup-needed"}}}
		entry.EntryID = store.ComputeRevisionID(entry)
		if err := store.AppendReconcileRevision(s, report.Slug, entry); err != nil {
			return out, err
		}
		out = append(out, entry)
	}
	return out, nil
}
