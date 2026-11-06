package workflow

// ADR-013 Amendment 3 (D28–D37) — GH #25 A1: a recorded single direct
// hard child may pass V8's current half. Each test names the mutation it
// catches; every promotion condition has a fixture that promotes when the
// condition is removed.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// A1-T: two direct hard children explain the block together (P4). The
// first edits the parent's path away from its hunks, so the second still
// finds the parent present at its own anchor and passes its own verify.
// Mutation caught: dropping the single-descendant condition.
func TestA1TwoDirectChildrenAreRefused(t *testing.T) {
	f := newOverlapFixture(t)
	f.feature("parent")
	f.land("parent", ovPath, featureBody(nil), true)
	f.feature("first", "parent")
	f.land("first", ovPath, featureBody(func(l []string) { l[1] = "FIRST2" }), true)
	f.feature("second", "parent")
	f.land("second", ovPath, featureBody(func(l []string) { l[1] = "FIRST2"; l[29] = "SECOND30" }), true)
	r := f.verify("parent")
	requireRefused(t, r, "2 recorded descendants rewrote these paths")
	if a := r.OverlapAssessment; len(a.Descendants) != 2 || a.Descendants[0].Slug != "first" || a.Descendants[1].Slug != "second" {
		t.Fatalf("descendants=%+v", a.Descendants)
	}
}

// A1-T: a nested E6 verify never promotes (P3). The nested run of a
// feature its top-level run promotes must stay blocked, so an E6 answer
// can never rest on another explanation. Mutation caught: letting a
// nested verify assess and promote.
func TestA1NestedVerifyNeverPromotes(t *testing.T) {
	f, _, _ := overlapAC1(t)
	requirePromoted(t, f.verify("parent"))
	nested, err := newVerifyRunContext(f.s).runNestedVerify(f.s, "parent")
	if err != nil || nested == nil {
		t.Fatalf("nested verify: %v", err)
	}
	requireBlocked(t, nested)
	if nested.OverlapAssessment != nil {
		t.Fatalf("a nested verify assessed: %+v", nested.OverlapAssessment)
	}
}

// A1-T: another block-severity failure keeps the block (P8). A malformed
// coverage envelope fails V11 at block severity after anchor C ran.
// Mutation caught: promoting without checking the rest of the run.
func TestA1AnotherBlockFailureKeepsTheBlock(t *testing.T) {
	f, _, _ := overlapAC1(t)
	if err := f.s.WriteArtifact("parent", "recipe-coverage.json", "not json\n"); err != nil {
		t.Fatal(err)
	}
	r := f.verify("parent")
	if c := checkByID(t, r, CheckRecipeGenerationCoverage); c.Passed || c.Severity != SeverityBlock {
		t.Fatalf("precondition: coverage must fail at block severity: %+v", c)
	}
	requireRefused(t, r, overlapReasonOtherFailure)
}

// A1-T (review F3): the child drifted in context on a path the target
// never touched. The target's composition only covers its own paths, so
// the child's own full-context anchor must cover the rest (P9). Mutation
// caught: dropping P9, which promotes on a child whose own change is only
// present with reduced context.
func TestA1ChildContextDriftOnItsOwnPathIsRefused(t *testing.T) {
	const second = "src/second.txt"
	f := newOverlapFixture(t)
	f.edit(second, ladderBody(nil))
	f.feature("parent")
	f.land("parent", ovPath, featureBody(nil), true)
	f.feature("child", "parent")
	f.landFiles("child", true, ovPath, childBody(), second, ladderBody(func(l []string) { l[19] = "CHILD20" }))
	f.edit(second, ladderBody(func(l []string) { l[19] = "CHILD20"; l[21] = "DRIFT22" }))

	child := f.verify("child")
	if child.Verdict != "passed" || checkByID(t, child, CheckPostApplyPatchReplayClean).AnchorResults["current"] != CurrentMaterializedContextDrif {
		t.Fatalf("precondition: the child passes with context drift: verdict=%s v8=%+v", child.Verdict, checkByID(t, child, CheckPostApplyPatchReplayClean))
	}
	requireRefused(t, f.verify("parent"), "descendant child's own current anchor did not match with full context")
}

// A1-T (review F2): the child reaches HEAD only through a merge's second
// parent (P10). Mutation caught: dropping P10, which E5's ancestry checks
// alone satisfy.
func TestA1ChildThroughSecondParentIsRefused(t *testing.T) {
	f := newOverlapFixture(t)
	f.feature("parent")
	landing := f.land("parent", ovPath, featureBody(nil), true)
	mustGit(t, f.root, "branch", "side")
	f.edit("src/mid.txt", "main-only\n")
	mustGit(t, f.root, "checkout", "-q", "side")
	f.feature("child", "parent")
	childLanding := f.land("child", ovPath, childBody(), true)
	mustGit(t, f.root, "checkout", "-q", "main")
	mustGit(t, f.root, "merge", "-q", "--no-ff", "-m", "merge side", "side")
	if parents := strings.Fields(mustGit(t, f.root, "rev-list", "--parents", "-n", "1", "HEAD")); len(parents) != 3 || parents[2] != childLanding {
		t.Fatalf("precondition: the child must be HEAD's second parent: %v", parents)
	}
	requireRefused(t, f.verify("parent"), "the history from landing commit "+landing+" to HEAD contains a merge commit")
}

// A1-T (review F2): an unrelated merge after the child also refuses (P10).
func TestA1UnrelatedMergeIsRefused(t *testing.T) {
	f, landing, _ := overlapAC1(t)
	mustGit(t, f.root, "checkout", "-q", "-b", "side")
	f.edit("src/mid.txt", "side\n")
	mustGit(t, f.root, "checkout", "-q", "main")
	mustGit(t, f.root, "merge", "-q", "--no-ff", "-m", "merge side", "side")
	requireRefused(t, f.verify("parent"), "the history from landing commit "+landing+" to HEAD contains a merge commit")
}

// A1-T: an unanswerable merge probe refuses (P10 fails closed). Mutation
// caught: treating a Git failure as a merge-free history.
func TestA1MergeProbeFailureRefuses(t *testing.T) {
	f, landing, _ := overlapAC1(t)
	realGit := mustLookGit(t)
	dir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
case " $* " in *" rev-list --merges "*)
  echo "fatal: simulated object store failure" >&2; exit 128;;
esac
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	requireRefused(t, f.verify("parent"), "git could not determine whether the history from landing commit "+landing)
}

// A1-T: each D30 condition refuses on its own, in a fixed order, and the
// baseline passes. Mutation caught: dropping or reordering any clause.
func TestA1PromotionBlockerConditions(t *testing.T) {
	one := &VerifyOverlapAssessment{Class: OverlapRecordedDescendantEvolution,
		Descendants: []VerifyOverlapDescendant{{Slug: "c"}}}
	two := &VerifyOverlapAssessment{Class: OverlapRecordedDescendantEvolution,
		Descendants: []VerifyOverlapDescendant{{Slug: "c"}, {Slug: "d"}}}
	for _, tc := range []struct {
		name       string
		a          *VerifyOverlapAssessment
		direct     bool
		evidence   string
		historical string
		clean      bool
		want       string
	}{
		{"baseline", one, true, EvidenceExact, "passed", true, ""},
		{"P1 nil", nil, true, EvidenceExact, "passed", true, "did not find a recorded descendant evolution"},
		{"P1 class", &VerifyOverlapAssessment{Class: OverlapUnexplainedContentLoss}, true, EvidenceExact, "passed", true, "did not find a recorded descendant evolution"},
		{"P4", two, true, EvidenceExact, "passed", true, "2 recorded descendants"},
		{"P5", one, false, EvidenceExact, "passed", true, "only through intermediate features"},
		{"P6", one, true, EvidenceUnavailable, "passed", true, "own landing evidence is " + EvidenceUnavailable},
		{"P7", one, true, EvidenceExact, "failed", true, "historical half"},
		{"P9", one, true, EvidenceExact, "passed", false, "did not match with full context"},
		{"P4 before P5", two, false, EvidenceExact, "passed", true, "2 recorded descendants"},
		{"P6 before P9", one, true, EvidenceUnavailable, "passed", false, "own landing evidence"},
	} {
		got := explainedPromotionBlocker(tc.a, tc.direct, tc.evidence, tc.historical, tc.clean)
		if (tc.want == "") != (got == "") || !strings.Contains(got, tc.want) {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

// A1-T: the promotion itself (P2, P8 and the D31 effects). Mutation
// caught: promoting a dropped assessment, a fold that did not set
// failed_at, or a run with another block or block-abort failure; and not
// promoting past a warn or skipped check.
func TestA1PromoteExplainedCurrentAnchorConditions(t *testing.T) {
	type mut func(r *VerifyReport, p *dynamicPhase)
	build := func(m mut) (*VerifyReport, dynamicPhase) {
		r := &VerifyReport{
			FailedAt:          FailedAtLandedContentAbsent,
			OverlapAssessment: &VerifyOverlapAssessment{Class: OverlapRecordedDescendantEvolution, VerdictEffect: OverlapVerdictEffectNone},
			Checks: []store.VerifyCheckResult{
				{ID: CheckIntentFilesPresent, Severity: SeverityBlock, Passed: true},
				{ID: CheckPostApplyPatchReplayClean, Severity: SeverityBlock, Remediation: "R25",
					AnchorResults: map[string]string{"historical": "passed", "current": CurrentAbsent}},
			},
		}
		p := dynamicPhase{overlapPromotable: true, anchorCSetFailedAt: true,
			explainedAdvisory: warnAdvisory(AdvisoryExplainedByDescendant, "p", "", "R27")}
		if m != nil {
			m(r, &p)
		}
		return r, p
	}
	other := func(c store.VerifyCheckResult) mut {
		return func(r *VerifyReport, _ *dynamicPhase) { r.Checks = append(r.Checks, c) }
	}
	for _, tc := range []struct {
		name string
		m    mut
		want bool
	}{
		{"baseline", nil, true},
		{"P2 dropped", func(r *VerifyReport, _ *dynamicPhase) { r.OverlapAssessment = nil }, false},
		{"fold refused", func(_ *VerifyReport, p *dynamicPhase) { p.overlapPromotable = false }, false},
		{"P8 earlier failed_at", func(_ *VerifyReport, p *dynamicPhase) { p.anchorCSetFailedAt = false }, false},
		{"P8 block", other(store.VerifyCheckResult{ID: "x", Severity: SeverityBlock}), false},
		{"P8 block-abort", other(store.VerifyCheckResult{ID: "x", Severity: SeverityBlockAbort}), false},
		{"warn failure", other(store.VerifyCheckResult{ID: "x", Severity: SeverityWarn}), true},
		{"skipped block", other(store.VerifyCheckResult{ID: "x", Severity: SeverityBlock, Skipped: true}), true},
		{"no V8", func(r *VerifyReport, _ *dynamicPhase) { r.Checks = r.Checks[:1] }, false},
	} {
		r, p := build(tc.m)
		if got := promoteExplainedCurrentAnchor(r, p); got != tc.want {
			t.Errorf("%s: promoted=%v want %v", tc.name, got, tc.want)
			continue
		}
		if !tc.want {
			if r.FailedAt != FailedAtLandedContentAbsent || len(r.Advisories) != 0 {
				t.Errorf("%s: a refused promotion changed the report: %+v", tc.name, r)
			}
			continue
		}
		v8 := r.Checks[1]
		if !v8.Passed || v8.Remediation != "" || v8.AnchorResults["current"] != CurrentExplainedByDescendant ||
			v8.AnchorResults["historical"] != "passed" || r.FailedAt != "" ||
			r.OverlapAssessment.VerdictEffect != OverlapVerdictEffectCurrentAnchorPassed ||
			len(r.Advisories) != 1 || r.Advisories[0].Code != AdvisoryExplainedByDescendant {
			t.Errorf("%s: promotion effects wrong: v8=%+v report=%+v", tc.name, v8, r)
		}
	}
}

// A1-T: the promotion is persisted as an ordinary pass (D34). Mutation
// caught: persisting before the promotion.
func TestA1PromotionPersistsPassed(t *testing.T) {
	f, _, _ := overlapAC1(t)
	if _, err := RunVerify(f.s, "parent", VerifyOptions{}); err != nil {
		t.Fatalf("RunVerify: %v", err)
	}
	st, err := f.s.LoadFeatureStatus("parent")
	if err != nil {
		t.Fatal(err)
	}
	if st.Verify == nil || !st.Verify.Passed {
		t.Fatalf("persisted verify=%+v", st.Verify)
	}
}

// A1-T (review F1): V4 and V5 answer at the run's captured HEAD even when
// the live HEAD moved after the capture — the ABA window the end-of-run
// re-statement cannot see. Mutation caught: either check reading the live
// HEAD again.
func TestA1HeadDependentChecksUseCapturedHead(t *testing.T) {
	f := newOverlapFixture(t)
	f.feature("up")
	if err := f.s.MarkFeatureState("up", store.StateUpstreamMerged, "test", ""); err != nil {
		t.Fatal(err)
	}
	f.feature("child")
	h := gitHeadOf(t, f.root)
	k := strings.TrimSpace(mustGit(t, f.root, "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "temporary"))
	st, err := f.s.LoadFeatureStatus("child")
	if err != nil {
		t.Fatal(err)
	}
	st.DependsOn = []store.Dependency{{Slug: "up", Kind: store.DependencyKindHard, SatisfiedBy: k}}
	if err := f.s.SaveFeatureStatus(st); err != nil {
		t.Fatal(err)
	}
	captured := newVerifyRunContext(f.s)
	mustGit(t, f.root, "update-ref", "refs/heads/main", k)
	defer mustGit(t, f.root, "update-ref", "refs/heads/main", h)

	live := newVerifyRunContext(f.s)
	if c := checkSatisfiedByReachable(live, f.s, "child", st); !c.Passed {
		t.Fatalf("control: V5 at the moved HEAD must pass: %+v", c)
	}
	if c := checkDepMetadataValid(live, f.s, "child", st); !c.Passed {
		t.Fatalf("control: V4 at the moved HEAD must pass: %+v", c)
	}
	if captured.headRev() != h {
		t.Fatalf("headRev=%s want the captured %s", captured.headRev(), h)
	}
	if c := checkSatisfiedByReachable(captured, f.s, "child", st); c.Passed {
		t.Fatalf("V5 followed the live HEAD: %+v", c)
	}
	if c := checkDepMetadataValid(captured, f.s, "child", st); c.Passed || !strings.Contains(c.Remediation, k) {
		t.Fatalf("V4 followed the live HEAD: %+v", c)
	}
}

// A1-T (review F1): the trailer enumeration reads the captured HEAD, so a
// HEAD that moves between the capture and the enumeration cannot add a
// commit the run never captured. Mutation caught: enumerating the live
// HEAD.
func TestA1EnumerationUsesCapturedHead(t *testing.T) {
	f := newOverlapFixture(t)
	h := gitHeadOf(t, f.root)
	k := strings.TrimSpace(mustGit(t, f.root, "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "moved"))
	realGit := mustLookGit(t)
	dir := t.TempDir()
	done := filepath.Join(dir, "done")
	script := fmt.Sprintf(`#!/bin/sh
%q "$@"; rc=$?
case " $* " in *" rev-parse HEAD "*)
  if [ ! -f %q ]; then : > %q; %q -C %q update-ref refs/heads/main %s; fi;;
esac
exit $rc
`, realGit, done, done, realGit, f.root, k)
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx := newVerifyRunContext(f.s)
	if _, err := os.Stat(done); err != nil || ctx.head != h {
		t.Fatalf("precondition: the shim must move HEAD right after the capture: head=%s err=%v", ctx.head, err)
	}
	if ctx.commitsErr != nil || len(ctx.commits) == 0 {
		t.Fatalf("enumeration: %v (%d records)", ctx.commitsErr, len(ctx.commits))
	}
	for _, c := range ctx.commits {
		if c.SHA == k {
			t.Fatal("the enumeration followed the live HEAD past the captured commit")
		}
	}
}

// A1-T (review F4, folds W2): an arbitration fail-fast must not leave its
// member advisories and baselines on the run context for the next
// `verify --all` entry. mid-x's closure reaches its landed member base
// (context drift: an advisory) before its unapplied member gone fails
// fast; zz verifies next. Mutation caught: draining the pending state only
// on the success path.
func TestA1VerifyAllArbitrationFailFastDoesNotLeak(t *testing.T) {
	f := newOverlapFixture(t)
	f.feature("base")
	f.land("base", ovPath, featureBody(nil), true)
	f.edit(ovPath, featureBody(func(l []string) { l[11] = "DRIFT12" }))
	f.feature("gone")
	f.feature("mid-x", "base", "gone")
	f.land("mid-x", "src/mid.txt", "mid-seed\nX\n", true)
	f.feature("zz")
	f.land("zz", "src/zz.txt", "zz\n", true)
	if err := f.s.MarkFeatureState("gone", store.StateUnapplied, "test", ""); err != nil {
		t.Fatal(err)
	}

	mid := f.verify("mid-x")
	if mid.FailedAt != FailedAtParentUnapplied {
		t.Fatalf("precondition: mid-x must fail fast on gone: failed_at=%q", mid.FailedAt)
	}
	if adv, ok := advisoryByCode(mid, AdvisoryContextDrift); !ok || adv.Slug != "base" {
		t.Fatalf("precondition: mid-x's arbitration must record base's drift before failing: %+v", mid.Advisories)
	}

	agg, err := RunVerifyAll(f.s, VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerifyAll: %v", err)
	}
	order := map[string]int{}
	for i, e := range agg.Features {
		order[e.Slug] = i
	}
	if mi, ok := order["mid-x"]; !ok || order["zz"] != mi+1 {
		t.Fatalf("precondition: zz must verify right after mid-x: %v", order)
	}
	for _, e := range agg.Features {
		if e.Report == nil {
			continue
		}
		single := f.verify(e.Slug)
		e.Report.VerifiedAt, single.VerifiedAt = "", ""
		ja, _ := json.Marshal(e.Report)
		jb, _ := json.Marshal(single)
		if string(ja) != string(jb) {
			t.Errorf("%s differs under --all:\n%s\n%s", e.Slug, ja, jb)
		}
	}
}
