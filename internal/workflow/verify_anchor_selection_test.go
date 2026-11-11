package workflow

// ADR-013 Amendment 4 slice N1 (GH #26): D14 step 4 prefers the newest
// qualifying exact attestation (D40). Each test names the mutation it
// catches. T2, T6, T6b and T7 are the existing rows AC-L33, AC-L35,
// TestRev2_DuplicateAttestationTrueAmbiguityStillUsesR7 and AC-L34.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

const a4ParentPath = "src/parent.txt"

func a4ParentBody(tag string) string { return ladderBody(func(l []string) { l[9] = tag }) }

func a4ChildBody() string { return ladderBody(func(l []string) { l[29] = "CHILD30" }) }

func (f *overlapFixture) patchBytes(slug string) string {
	f.t.Helper()
	data, err := os.ReadFile(artifactPath(f.root, slug, "post-apply.patch"))
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

// relandSame lands slug again over path with body, as a re-application on
// the current HEAD, and requires the canonical patch bytes not to change.
func (f *overlapFixture) relandSame(slug, path, body string) string {
	f.t.Helper()
	before := f.patchBytes(slug)
	landing := f.land(slug, path, body, false)
	if after := f.patchBytes(slug); after != before {
		f.t.Fatalf("precondition: %s's re-land changed its patch bytes", slug)
	}
	return landing
}

// a4GH26 is A4-E1: the child lands on the parent; the parent is
// unapplied and re-lands with a changed patch; the child is genuinely
// unapplied, re-applied on the parent's new landing and re-lands with
// identical patch and recipe bytes. Its new base makes the second landing
// its only exact attestation, and both landings forward-qualify.
func a4GH26(t *testing.T) (f *overlapFixture, first, second string) {
	t.Helper()
	f = newOverlapFixture(t)
	f.edit(a4ParentPath, ladderBody(nil))
	f.feature("parent")
	f.land("parent", a4ParentPath, a4ParentBody("PARENT-V1"), true)
	f.feature("child", "parent")
	first = f.land("child", ovPath, a4ChildBody(), true)
	f.edit(a4ParentPath, ladderBody(nil))
	f.land("parent", a4ParentPath, a4ParentBody("PARENT-V2"), true)
	f.edit(ovPath, ladderBody(nil))
	second = f.relandSame("child", ovPath, a4ChildBody())
	return f, first, second
}

// requireAggregateMatchesSingle runs `verify --all` and requires every
// embedded report to equal its single run (Amendment 4 tests, T1–T4).
func requireAggregateMatchesSingle(t *testing.T, s *store.Store) {
	t.Helper()
	agg, err := RunVerifyAll(s, VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerifyAll: %v", err)
	}
	if len(agg.Features) == 0 {
		t.Fatalf("verify --all reported no features")
	}
	for _, e := range agg.Features {
		if e.Report == nil {
			t.Fatalf("%s has no report under --all", e.Slug)
		}
		single, _ := RunVerify(s, e.Slug, VerifyOptions{NoWrite: true})
		if single == nil {
			t.Fatalf("RunVerify %s returned no report", e.Slug)
		}
		e.Report.VerifiedAt, single.VerifiedAt = "", ""
		ja, _ := json.Marshal(e.Report)
		jb, _ := json.Marshal(single)
		if string(ja) != string(jb) {
			t.Errorf("%s differs under --all:\n%s\n%s", e.Slug, ja, jb)
		}
	}
}

// T1 (D40): GH #26 passes and replays at the exact attestation. Mutation
// caught: oldest-first restored, which replays at the first landing,
// where the parent's current patch is absent (R14).
func TestA4T1GenuineRelandReplaysAtTheAttestation(t *testing.T) {
	f, first, second := a4GH26(t)
	r := f.verify("child")
	if r.Verdict != "passed" || r.ExitCode != 0 || r.FailedAt != "" {
		t.Fatalf("verdict=%s exit=%d failed_at=%q v7=%q", r.Verdict, r.ExitCode, r.FailedAt,
			checkByID(t, r, CheckRecipeReplayClean).Remediation)
	}
	if r.LandingEvidence.State != EvidenceExact || r.LandingEvidence.AttestationCommit != second {
		t.Fatalf("evidence=%s attestation=%s want exact at %s", r.LandingEvidence.State, r.LandingEvidence.AttestationCommit, second)
	}
	ha := r.Baseline.HistoricalAnchor
	if ha == nil || ha.CandidatesCollected != 2 || ha.CandidatesQualified != 2 {
		t.Fatalf("precondition: both landings (%s, %s) must qualify: %+v", first, second, ha)
	}
	if ha.ReplayAnchorCommit != second {
		t.Fatalf("replay anchor=%s want the exact attestation %s", ha.ReplayAnchorCommit, second)
	}
	v8 := checkByID(t, r, CheckPostApplyPatchReplayClean)
	if v8.AnchorResults["historical"] != "passed" || v8.AnchorResults["current"] != CurrentMaterializedClean {
		t.Fatalf("v8 anchors=%v", v8.AnchorResults)
	}
	requireAggregateMatchesSingle(t, f.s)
}

// T5 (D45): T1 plus a later unrecorded edit at HEAD on the child's path
// still fails at the current anchor. Mutation caught: skipping anchor C
// when the historical anchor passes.
func TestA4T5LaterEditFailsAtTheCurrentAnchor(t *testing.T) {
	f, _, _ := a4GH26(t)
	f.edit(ovPath, ladderBody(func(l []string) { l[29] = "EDITED30" }))
	r := f.verify("child")
	if r.ExitCode != 2 || r.FailedAt != FailedAtLandedContentAbsent {
		t.Fatalf("exit=%d failed_at=%q want 2/landed-content-absent", r.ExitCode, r.FailedAt)
	}
	if v7 := checkByID(t, r, CheckRecipeReplayClean); !v7.Passed {
		t.Fatalf("V7 must pass at the attestation: %q", v7.Remediation)
	}
	v8 := checkByID(t, r, CheckPostApplyPatchReplayClean)
	if v8.Passed || v8.AnchorResults["historical"] != "passed" || v8.AnchorResults["current"] != CurrentAbsent {
		t.Fatalf("v8=%+v", v8)
	}
	requireClass(t, r, OverlapUnexplainedContentLoss, OverlapStepCandidates)
}

// T6b (D40): two exact attestations whose identities differ stop at
// landing evidence with R7, and no replay anchor is resolved. Mutation
// caught: D40 run before (or instead of) evidence classification, which
// resolves an anchor and fails at the historical anchor (R11) instead.
func TestA4T6bAmbiguousAttestationsNeverReachD40(t *testing.T) {
	f := newLadderFixture(t)
	block := f.TrailerBlock(f.Slug)
	mustWriteFile(t, filepath.Join(f.Root, f.FilePath), featureBody(func(l []string) { l[19] = "DIFFERENT" }))
	mustGit(t, f.Root, "add", "-A")
	mustGit(t, f.Root, "commit", "-q", "-m", "second attestation, different change", "-m", block)

	r := f.Verify()
	if r.LandingEvidence.State != EvidenceAmbiguous || r.FailedAt != FailedAtLandingEvidence || r.ExitCode != 2 {
		t.Fatalf("evidence=%s failed_at=%q exit=%d want ambiguous/landing-evidence/2",
			r.LandingEvidence.State, r.FailedAt, r.ExitCode)
	}
	if v7 := checkByID(t, r, CheckRecipeReplayClean); !strings.Contains(v7.Remediation, "is ambiguous:") {
		t.Fatalf("V7 must carry R7: %q", v7.Remediation)
	}
	// The terminal phase reports the anchor as unavailable without
	// collecting a candidate: D14 (and D40 with it) never ran.
	ha := r.Baseline.HistoricalAnchor
	if ha == nil || ha.State != AnchorStateUnavailable || ha.CandidatesCollected != 0 ||
		ha.CandidatesQualified != 0 || ha.ReplayAnchorCommit != "" {
		t.Fatalf("anchor resolution ran for ambiguous evidence: %+v", ha)
	}
}

// T8 (D40): the forward check cannot run on the attestation candidate.
// Resolution aborts with a reader-failure state. Mutation caught: the
// error read as "does not qualify", which silently selects the older
// landing (oldest-first fallback).
func TestA4T8AttestationProbeFailureIsNotANonQualifier(t *testing.T) {
	skipWithoutPOSIXGitShim(t)
	f, _, second := a4GH26(t)
	realGit := mustLookGit(t)
	dir := t.TempDir()
	script := "#!/bin/sh\nsub=no\nfor a in \"$@\"; do\n  [ \"$a\" = read-tree ] && sub=yes\ndone\n" +
		"if [ $sub = yes ]; then\n  for a in \"$@\"; do\n    case \"$a\" in\n      " + second + "^*)\n" +
		"        echo 'fatal: injected read-tree failure' >&2\n        exit 128 ;;\n    esac\n  done\nfi\n" +
		"exec " + realGit + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	r := f.verify("child")
	if r.LandingEvidence.State != EvidenceUnavailable || !strings.Contains(r.LandingEvidence.Reason, "injected read-tree failure") {
		t.Fatalf("evidence=%s reason=%q want unavailable from the injected failure", r.LandingEvidence.State, r.LandingEvidence.Reason)
	}
	if r.FailedAt != FailedAtLandingEvidence || r.ExitCode != 2 {
		t.Fatalf("failed_at=%q exit=%d", r.FailedAt, r.ExitCode)
	}
	if ha := r.Baseline.HistoricalAnchor; ha != nil && ha.ReplayAnchorCommit != "" {
		t.Fatalf("a replay anchor was selected despite the failed probe: %+v", ha)
	}
}

// T11 (D40): only exact attestations are preferred. The exact attestation
// is the older qualifier; a later landing whose recipe trailer is stale
// forward-qualifies with an equal identity. Mutation caught: the newest
// qualifier selected whenever an exact attestation qualifies.
func TestA4T11LaterNonAttestationNeverDisplacesTheAttestation(t *testing.T) {
	f := newLadderFixture(t)
	exact := f.LandingCommit
	block := f.TrailerBlock(f.Slug)
	staleRecipe := strings.Replace(block, "Tpatch-Recipe-SHA: "+sha256Hex(f.RecipeBytes()),
		"Tpatch-Recipe-SHA: "+strings.Repeat("b", 64), 1)
	if staleRecipe == block {
		t.Fatalf("precondition: the recipe trailer was not rewritten")
	}
	f.EditTracked(ladderBody(nil), "unapply "+f.Slug)
	mustWriteFile(t, filepath.Join(f.Root, f.FilePath), featureBody(nil))
	mustGit(t, f.Root, "add", "-A")
	mustGit(t, f.Root, "commit", "-q", "-m", "re-land with a stale recipe trailer", "-m", staleRecipe)
	later := gitHeadOf(t, f.Root)

	r := f.Verify()
	if r.LandingEvidence.State != EvidenceExact || r.LandingEvidence.AttestationCommit != exact {
		t.Fatalf("evidence=%s attestation=%s want exact at %s", r.LandingEvidence.State, r.LandingEvidence.AttestationCommit, exact)
	}
	ha := r.Baseline.HistoricalAnchor
	if ha == nil || ha.CandidatesQualified != 2 {
		t.Fatalf("precondition: both landings (%s, %s) must qualify: %+v", exact, later, ha)
	}
	if ha.ReplayAnchorCommit != exact {
		t.Fatalf("replay anchor=%s want the exact attestation %s, not the later non-attestation %s", ha.ReplayAnchorCommit, exact, later)
	}
}

// a4T14 is Amendment 4 T14: hard chain p → t → c, with t the target and
// c its one recorded child. p, t and c land; c rewrites a line t added.
// c and t are unapplied, p re-lands with a changed patch on its own file,
// t re-lands, then c re-lands. The patch bytes of t and c never change,
// and c's first generation still binds t's patch (E4). In the control, c
// is re-applied by a commit without a trailer and c's landing is a
// trailer-only commit on top, so that landing does not qualify (A4-E3)
// and c's anchor stays at its first landing, where p's patch is absent.
func a4T14(t *testing.T, control bool) (f *overlapFixture, t2, c2 string) {
	t.Helper()
	f = newOverlapFixture(t)
	f.edit(a4ParentPath, ladderBody(nil))
	f.feature("p")
	f.land("p", a4ParentPath, a4ParentBody("P-V1"), true)
	f.feature("t", "p")
	f.land("t", ovPath, featureBody(nil), true)
	f.feature("c", "t")
	f.land("c", ovPath, childBody(), true)
	f.edit(ovPath, ladderBody(nil))
	f.edit(a4ParentPath, ladderBody(nil))
	f.land("p", a4ParentPath, a4ParentBody("P-V2"), true)
	t2 = f.relandSame("t", ovPath, featureBody(nil))
	if !control {
		return f, t2, f.relandSame("c", ovPath, childBody())
	}
	base := gitHeadOf(t, f.root)
	f.edit(ovPath, childBody())
	st, err := f.s.LoadFeatureStatus("c")
	if err != nil {
		t.Fatal(err)
	}
	st.Apply.BaseCommit = base
	if err := f.s.SaveFeatureStatus(st); err != nil {
		t.Fatal(err)
	}
	mustGit(t, f.root, "add", "-A")
	mustGit(t, f.root, "commit", "-q", "-m", "feat: land c", "-m", trailerBlockFor(t, f.s, f.root, "c"))
	return f, t2, gitHeadOf(t, f.root)
}

// requireT14Preconditions asserts what both T14 cases share: c is exact
// at its second landing, the target replays cleanly at its attestation
// (P7), and c's own verify finds its current anchor clean.
func requireT14Preconditions(t *testing.T, f *overlapFixture, target *VerifyReport, t2, c2 string) {
	t.Helper()
	child := f.verify("c")
	if child.LandingEvidence.State != EvidenceExact || child.LandingEvidence.AttestationCommit != c2 {
		t.Fatalf("precondition E1: c evidence=%s attestation=%s want exact at %s",
			child.LandingEvidence.State, child.LandingEvidence.AttestationCommit, c2)
	}
	if got := checkByID(t, child, CheckPostApplyPatchReplayClean).AnchorResults["current"]; got != CurrentMaterializedClean {
		t.Fatalf("precondition: c's current anchor=%q want materialized-clean", got)
	}
	if ha := target.Baseline.HistoricalAnchor; ha == nil || ha.ReplayAnchorCommit != t2 {
		t.Fatalf("precondition: the target must replay at its attestation %s: %+v", t2, ha)
	}
	if got := checkByID(t, target, CheckPostApplyPatchReplayClean).AnchorResults["historical"]; got != "passed" {
		t.Fatalf("precondition P7: the target's historical half=%q", got)
	}
}

// T14 main (D40, D45): the child's nested verify selects its re-landing,
// passes, and the target is promoted. Mutation caught: the nested verify
// selecting oldest-first, which keeps the target red.
func TestA4T14NestedVerifyFollowsD40(t *testing.T) {
	f, t2, c2 := a4T14(t, false)
	r := f.verify("t")
	requireT14Preconditions(t, f, r, t2, c2)
	requirePromoted(t, r)
}

// T14 control: c's re-landing does not qualify, so its nested verify
// fails and the target stays red with E6 the child's only failing rule.
// Mutation caught: E6 dropped from eligibility, which turns the class into
// recorded-descendant-evolution. P9 keeps that run red too, so the class
// and failing rule are asserted, not the exit code.
func TestA4T14ControlKeepsTheBlockOnE6(t *testing.T) {
	f, t2, c2 := a4T14(t, true)
	r := f.verify("t")
	requireT14Preconditions(t, f, r, t2, c2)
	requireBlocked(t, r)
	a := requireClass(t, r, OverlapInvalidOrMissingEvidence, OverlapStepEligibility)
	if len(a.Descendants) != 1 || a.Descendants[0].Slug != "c" || a.Descendants[0].FailedRule != OverlapRuleE6 {
		t.Fatalf("descendants=%+v want c failing only E6", a.Descendants)
	}
}
