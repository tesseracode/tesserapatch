package workflow

// ADR-013 Amendment 2 (D20–D27) — the read-only overlap assessment for
// GH #25 slices C1–C2, against PRD-overlap-verification-consolidation
// §9 (AC-1, AC-3, AC-5, AC-6 and the N-matrix). Each behavior row states
// the mutation it catches; the source guards carry their own failing
// fixtures.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

const ovPath = "src/app.txt"

// overlapFixture lands features with real four-trailer commits, each with
// its canonical patch, a write-file recipe and (optionally) a `record`
// patch generation bound to its hard parents.
type overlapFixture struct {
	t    *testing.T
	s    *store.Store
	root string
}

func newOverlapFixture(t *testing.T) *overlapFixture {
	t.Helper()
	root := t.TempDir()
	gitInitVerifyTest(t, root)
	mustWriteFile(t, filepath.Join(root, ovPath), ladderBody(nil))
	mustWriteFile(t, filepath.Join(root, "src", "mid.txt"), "mid-seed\n")
	mustGit(t, root, "add", "-A")
	mustGit(t, root, "commit", "-q", "-m", "seed")
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	mustGit(t, root, "add", "-A")
	mustGit(t, root, "commit", "-q", "-m", "tpatch scaffolding")
	return &overlapFixture{t: t, s: s, root: root}
}

func (f *overlapFixture) feature(slug string, hardDeps ...string) {
	f.t.Helper()
	if _, err := f.s.AddFeature(store.AddFeatureInput{Title: slug, Slug: slug, Request: "x"}); err != nil {
		f.t.Fatalf("AddFeature %s: %v", slug, err)
	}
	if err := f.s.MarkFeatureState(slug, store.StateApplied, "apply", ""); err != nil {
		f.t.Fatalf("MarkFeatureState %s: %v", slug, err)
	}
	writeIntentFiles(f.t, f.s, slug)
	if len(hardDeps) == 0 {
		return
	}
	st, err := f.s.LoadFeatureStatus(slug)
	if err != nil {
		f.t.Fatal(err)
	}
	for _, d := range hardDeps {
		st.DependsOn = append(st.DependsOn, store.Dependency{Slug: d, Kind: store.DependencyKindHard})
	}
	if err := f.s.SaveFeatureStatus(st); err != nil {
		f.t.Fatal(err)
	}
}

// land rewrites path, records the feature against the current HEAD and
// commits everything with the feature's trailer block.
func (f *overlapFixture) land(slug, path, body string, generation bool) string {
	f.t.Helper()
	mustWriteFile(f.t, filepath.Join(f.root, path), body)
	patch, err := gitutil.CapturePatchScoped(f.root, []string{path})
	if err != nil || strings.TrimSpace(patch) == "" {
		f.t.Fatalf("capture %s: %v", slug, err)
	}
	if err := f.s.WriteArtifact(slug, "post-apply.patch", patch); err != nil {
		f.t.Fatal(err)
	}
	recipe := mustJSON(f.t, ApplyRecipe{Feature: slug, Operations: []RecipeOperation{{Type: "write-file", Path: path, Content: body}}})
	if err := f.s.WriteArtifact(slug, "apply-recipe.json", recipe); err != nil {
		f.t.Fatal(err)
	}
	head := gitHeadOf(f.t, f.root)
	st, err := f.s.LoadFeatureStatus(slug)
	if err != nil {
		f.t.Fatal(err)
	}
	st.Apply.BaseCommit = head
	st.Apply.HasPatch = true
	if err := f.s.SaveFeatureStatus(st); err != nil {
		f.t.Fatal(err)
	}
	if generation {
		if _, err := AppendPatchGenerationForFeature(f.s, slug, PatchGenerationInput{
			Kind: store.PatchGenerationKindRecord, Patch: patch, AuditPatch: "artifacts/post-apply.patch",
			BaseCommit: head, Reason: "overlap fixture",
			Upper:   store.GenerationUpper{Kind: "working-tree", Ref: "working-tree"},
			Capture: store.GenerationCapture{Mode: "working-tree-all", Pathspecs: []string{}, ClaimIDs: []string{}},
		}); err != nil {
			f.t.Fatalf("generation %s: %v", slug, err)
		}
	}
	block := trailerBlockFor(f.t, f.s, f.root, slug)
	mustGit(f.t, f.root, "add", "-A")
	mustGit(f.t, f.root, "commit", "-q", "-m", "feat: land "+slug, "-m", block)
	return gitHeadOf(f.t, f.root)
}

// edit is a commit that no feature records.
func (f *overlapFixture) edit(path, body string) string {
	f.t.Helper()
	mustWriteFile(f.t, filepath.Join(f.root, path), body)
	mustGit(f.t, f.root, "add", "-A")
	mustGit(f.t, f.root, "commit", "-q", "-m", "unrecorded edit")
	return gitHeadOf(f.t, f.root)
}

func (f *overlapFixture) verify(slug string) *VerifyReport {
	f.t.Helper()
	r, err := RunVerify(f.s, slug, VerifyOptions{NoWrite: true})
	if r == nil {
		f.t.Fatalf("RunVerify %s: %v", slug, err)
	}
	return r
}

// rebindGeneration rewrites the latest generation's hard-parent binding.
func (f *overlapFixture) rebindGeneration(slug, parent, sha string) {
	f.t.Helper()
	m, err := store.LoadPatchGenerations(f.s, slug)
	if err != nil {
		f.t.Fatal(err)
	}
	latest := &m.Generations[len(m.Generations)-1]
	for i := range latest.Dependencies {
		if latest.Dependencies[i].Slug == parent {
			latest.Dependencies[i].ParentPatchSHA256 = sha
		}
	}
	if err := store.SavePatchGenerations(f.s, m); err != nil {
		f.t.Fatal(err)
	}
}

func childBody() string { return featureBody(func(l []string) { l[29] = "CHILD30" }) }

// overlapAC1 is the PRD §2.1 minimal case: parent lands, its hard child
// rewrites one of the parent's lines and lands.
func overlapAC1(t *testing.T) (f *overlapFixture, parentLanding, childLanding string) {
	t.Helper()
	f = newOverlapFixture(t)
	f.feature("parent")
	parentLanding = f.land("parent", ovPath, featureBody(nil), true)
	f.feature("child", "parent")
	childLanding = f.land("child", ovPath, childBody(), true)
	return f, parentLanding, childLanding
}

func requireBlocked(t *testing.T, r *VerifyReport) {
	t.Helper()
	if r.Verdict != "failed" || r.ExitCode != 2 || r.FailedAt != FailedAtLandedContentAbsent {
		t.Fatalf("verdict=%s exit=%d failed_at=%q — the assessment must never change the block", r.Verdict, r.ExitCode, r.FailedAt)
	}
}

func requireClass(t *testing.T, r *VerifyReport, class, step string) *VerifyOverlapAssessment {
	t.Helper()
	a := r.OverlapAssessment
	if a == nil {
		t.Fatalf("no overlap assessment; v8=%q", checkByID(t, r, CheckPostApplyPatchReplayClean).Remediation)
	}
	if a.Class != class || a.FailedStep != step || a.VerdictEffect != OverlapVerdictEffectNone {
		t.Fatalf("assessment=%+v want class=%s step=%q", a, class, step)
	}
	return a
}

// AC-1 + AC-3: the minimal case is explained, the verdict is unchanged
// and the child still passes. Mutation caught: any class/remediation or
// verdict change in the assessment path.
func TestOverlapAC1RecordedDescendantEvolution(t *testing.T) {
	f, parentLanding, childLanding := overlapAC1(t)
	r := f.verify("parent")
	requireBlocked(t, r)
	a := requireClass(t, r, OverlapRecordedDescendantEvolution, "")
	if len(a.Descendants) != 1 || a.Descendants[0].Slug != "child" ||
		a.Descendants[0].AttestationCommit != childLanding || a.Descendants[0].Evidence != EvidenceExact ||
		a.Descendants[0].FailedRule != "" {
		t.Fatalf("descendants=%+v", a.Descendants)
	}
	if !strings.Contains(a.Detail, parentLanding) {
		t.Errorf("detail must name the landing commit: %q", a.Detail)
	}
	v8 := checkByID(t, r, CheckPostApplyPatchReplayClean)
	if v8.Passed || v8.Skipped || !strings.Contains(v8.Remediation, "child @ "+childLanding) ||
		!strings.Contains(v8.Remediation, "do NOT re-land parent") {
		t.Fatalf("V8 must stay failed with R25: %+v", v8)
	}
	var human strings.Builder
	r.WriteHumanReport(&human)
	if !strings.Contains(human.String(), "overlap assessment: recorded-descendant-evolution (explanation only; verdict unchanged)") {
		t.Errorf("human report lacks the assessment:\n%s", human.String())
	}

	child := f.verify("child")
	if child.Verdict != "passed" || child.ExitCode != 0 || child.OverlapAssessment != nil {
		t.Fatalf("AC-3: child verdict=%s exit=%d assessment=%+v", child.Verdict, child.ExitCode, child.OverlapAssessment)
	}
}

// N2 — a non-tpatch edit far from every hunk. Mutation caught: the rev-1
// HEAD round-trip, which cannot see edits outside the hunks.
func TestOverlapN2UnrecordedEditOutsideHunksIsUnexplained(t *testing.T) {
	f, _, _ := overlapAC1(t)
	f.edit(ovPath, featureBody(func(l []string) { l[29] = "CHILD30"; l[4] = "UNRECORDED" }))
	r := f.verify("parent")
	requireBlocked(t, r)
	requireClass(t, r, OverlapUnexplainedContentLoss, OverlapStepCompare)
	if v8 := checkByID(t, r, CheckPostApplyPatchReplayClean); strings.Contains(v8.Remediation, "recorded-descendant-evolution") {
		t.Fatalf("an unexplained block must keep R1/R2: %q", v8.Remediation)
	}
}

// N1 — an unrecorded revert of another of the parent's lines after the
// child landed.
func TestOverlapN1UnrecordedRevertIsUnexplained(t *testing.T) {
	f, _, _ := overlapAC1(t)
	f.edit(ovPath, featureBody(func(l []string) { l[29] = "CHILD30"; l[9] = "l10" }))
	r := f.verify("parent")
	requireBlocked(t, r)
	requireClass(t, r, OverlapUnexplainedContentLoss, OverlapStepCompare)
}

// N3 — an unrecorded revert of the child restores the parent: the plain
// ladder passes and no assessment exists.
func TestOverlapN3CleanLadderHasNoAssessment(t *testing.T) {
	f, _, _ := overlapAC1(t)
	f.edit(ovPath, featureBody(nil))
	r := f.verify("parent")
	if r.Verdict != "passed" || r.OverlapAssessment != nil {
		t.Fatalf("verdict=%s assessment=%+v", r.Verdict, r.OverlapAssessment)
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "overlap_assessment") {
		t.Fatal("overlap_assessment must be omitted when absent")
	}
}

// N5 — the child's recipe changed after landing: evidence is no longer
// exact (E1).
func TestOverlapN5ChangedChildRecipeIsInvalid(t *testing.T) {
	f, _, _ := overlapAC1(t)
	recipe := mustJSON(t, ApplyRecipe{Feature: "child", Operations: []RecipeOperation{{Type: "write-file", Path: ovPath, Content: "other\n"}}})
	if err := f.s.WriteArtifact("child", "apply-recipe.json", recipe); err != nil {
		t.Fatal(err)
	}
	r := f.verify("parent")
	requireBlocked(t, r)
	a := requireClass(t, r, OverlapInvalidOrMissingEvidence, OverlapStepEligibility)
	if len(a.Descendants) != 1 || a.Descendants[0].FailedRule != OverlapRuleE1 {
		t.Fatalf("descendants=%+v", a.Descendants)
	}
	if v8 := checkByID(t, r, CheckPostApplyPatchReplayClean); !strings.Contains(v8.Remediation, "recorded descendant child may explain it but cannot be used as evidence (E1:") {
		t.Fatalf("V8 must carry R26 naming the child: %q", v8.Remediation)
	}
}

// N6 — the child's generation is bound to a different parent patch (E4).
func TestOverlapN6BindingMismatchIsInvalid(t *testing.T) {
	f, _, _ := overlapAC1(t)
	f.rebindGeneration("child", "parent", strings.Repeat("a", 64))
	r := f.verify("parent")
	a := requireClass(t, r, OverlapInvalidOrMissingEvidence, OverlapStepEligibility)
	if d := a.Descendants[0]; d.FailedRule != OverlapRuleE4 || !strings.HasPrefix(d.Detail, "its latest patch generation is bound to a different canonical patch of parent") {
		t.Fatalf("descendant=%+v", d)
	}
}

// A legacy child without patch-generations.json cannot bind its edge (E4).
func TestOverlapLegacyChildWithoutManifestIsInvalid(t *testing.T) {
	f := newOverlapFixture(t)
	f.feature("parent")
	f.land("parent", ovPath, featureBody(nil), true)
	f.feature("child", "parent")
	f.land("child", ovPath, childBody(), false)
	r := f.verify("parent")
	a := requireClass(t, r, OverlapInvalidOrMissingEvidence, OverlapStepEligibility)
	if d := a.Descendants[0]; d.FailedRule != OverlapRuleE4 || !strings.Contains(d.Detail, "missing (legacy feature)") {
		t.Fatalf("descendant=%+v", d)
	}
}

// intermediateChain is parent ← mid ← child, where mid touches only
// src/mid.txt and child rewrites one of the parent's lines.
func intermediateChain(t *testing.T) *overlapFixture {
	t.Helper()
	f := newOverlapFixture(t)
	f.feature("parent")
	f.land("parent", ovPath, featureBody(nil), true)
	f.feature("mid", "parent")
	f.land("mid", "src/mid.txt", "mid-seed\nMID\n", true)
	f.feature("child", "mid")
	f.land("child", ovPath, childBody(), true)
	return f
}

// Finding 4 — a bound intermediate that does not touch the paths is not
// a candidate, and the chain through it explains the block.
func TestOverlapIntermediateChainExplains(t *testing.T) {
	f := intermediateChain(t)
	a := requireClass(t, f.verify("parent"), OverlapRecordedDescendantEvolution, "")
	if len(a.Descendants) != 1 || a.Descendants[0].Slug != "child" {
		t.Fatalf("only the overlapping descendant is a candidate: %+v", a.Descendants)
	}
}

// Finding 4 — E4 covers every chain edge. Mutation caught: checking only
// the candidate's own edge, which is still bound here.
func TestOverlapIntermediateBindingIsChecked(t *testing.T) {
	f := intermediateChain(t)
	f.rebindGeneration("mid", "parent", strings.Repeat("b", 64))
	a := requireClass(t, f.verify("parent"), OverlapInvalidOrMissingEvidence, OverlapStepEligibility)
	if d := a.Descendants[0]; d.Slug != "child" || d.FailedRule != OverlapRuleE4 ||
		!strings.HasPrefix(d.Detail, "intermediate feature mid's latest patch generation is bound to a different canonical patch of parent") {
		t.Fatalf("descendant=%+v", d)
	}
}

// N19 — the child fails a block-severity check in its own same-run
// verify (E6).
func TestOverlapN19ChildFailingItsOwnVerifyIsInvalid(t *testing.T) {
	f, _, _ := overlapAC1(t)
	mustRemove(t, filepath.Join(f.root, ".tpatch", "features", "child", "spec.md"))
	r := f.verify("parent")
	requireBlocked(t, r)
	a := requireClass(t, r, OverlapInvalidOrMissingEvidence, OverlapStepEligibility)
	if d := a.Descendants[0]; d.FailedRule != OverlapRuleE6 || !strings.Contains(d.Detail, CheckIntentFilesPresent) {
		t.Fatalf("descendant=%+v", d)
	}
}

// N12 — a feature with exact evidence but no hard edge rewrote the file.
func TestOverlapN12EdgeLessFeatureIsUnexplained(t *testing.T) {
	f := newOverlapFixture(t)
	f.feature("parent")
	f.land("parent", ovPath, featureBody(nil), true)
	f.feature("other")
	f.land("other", ovPath, childBody(), true)
	requireClass(t, f.verify("parent"), OverlapUnexplainedContentLoss, OverlapStepCandidates)
}

// Two hard descendants landed on parallel histories cannot be ordered.
func TestOverlapParallelLandingsAreUnexplained(t *testing.T) {
	f := newOverlapFixture(t)
	f.feature("parent")
	f.land("parent", ovPath, featureBody(nil), true)
	mustGit(t, f.root, "branch", "side")
	f.feature("left", "parent")
	f.land("left", ovPath, featureBody(func(l []string) { l[29] = "LEFT30" }), true)
	mustGit(t, f.root, "checkout", "-q", "side")
	f.feature("right", "parent")
	f.land("right", ovPath, featureBody(func(l []string) { l[49] = "RIGHT50" }), true)
	mustGit(t, f.root, "checkout", "-q", "main")
	mustGit(t, f.root, "merge", "-q", "--no-ff", "-m", "merge side", "side")
	r := f.verify("parent")
	requireBlocked(t, r)
	a := requireClass(t, r, OverlapUnexplainedContentLoss, OverlapStepOrder)
	if !strings.Contains(a.Detail, "parallel histories") {
		t.Fatalf("detail=%q", a.Detail)
	}
}

// A ladder that matches with drifted context is reported, with no
// descendant considered and the verdict untouched.
func TestOverlapContextOnlyDrift(t *testing.T) {
	f := newOverlapFixture(t)
	f.feature("parent")
	f.land("parent", ovPath, featureBody(nil), true)
	f.edit(ovPath, featureBody(func(l []string) { l[11] = "UNRELATED" }))
	r := f.verify("parent")
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%s", r.Verdict)
	}
	if _, ok := advisoryByCode(r, AdvisoryContextDrift); !ok {
		t.Fatalf("expected the context-drift advisory: %+v", r.Advisories)
	}
	a := requireClass(t, r, OverlapContextOnlyDrift, "")
	if len(a.Descendants) != 0 {
		t.Fatalf("descendants=%+v", a.Descendants)
	}
}

// E5 against real ancestry: same commit, wrong order (N9) and an
// unreachable landing (N7). Mutation caught: dropping any of the three
// clauses.
func TestOverlapLandingOrderRule(t *testing.T) {
	f, parentLanding, childLanding := overlapAC1(t)
	mustGit(t, f.root, "checkout", "-q", "-b", "detour", parentLanding)
	f.edit("src/mid.txt", "detour\n")
	detour := gitHeadOf(t, f.root)
	mustGit(t, f.root, "checkout", "-q", "main")
	ctx := newVerifyRunContext(f.s)
	for _, tc := range []struct{ target, cand, want string }{
		{parentLanding, childLanding, ""},
		{parentLanding, parentLanding, "it shares this feature's landing commit"},
		{childLanding, parentLanding, "does not descend from this feature's landing commit"},
		{parentLanding, detour, "is not reachable from HEAD"},
		{parentLanding, "", "its landing commit is unknown"},
	} {
		got, execErr := overlapLandingOrder(ctx, tc.target, tc.cand)
		if execErr != "" {
			t.Errorf("overlapLandingOrder(%s, %s) execErr=%q, want a rule answer", tc.target, tc.cand, execErr)
		}
		if (tc.want == "") != (got == "") || !strings.Contains(got, tc.want) {
			t.Errorf("overlapLandingOrder(%s, %s)=%q want %q", tc.target, tc.cand, got, tc.want)
		}
	}
}

// installFailingAncestryGit makes exactly `git merge-base --is-ancestor
// <ancestor> <descendant>` exit 128, as a broken object store would; every
// other git call is real.
func installFailingAncestryGit(t *testing.T, ancestor, descendant string) {
	t.Helper()
	realGit := mustLookGit(t)
	dir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
case " $* " in *" merge-base --is-ancestor %s %s "*)
  echo "fatal: simulated object store failure" >&2; exit 128;;
esac
exec %q "$@"
`, ancestor, descendant, realGit)
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// E5 execution failure (review rev-1 F3): when Git cannot answer either
// ancestry query, E5 reports an execution error and never a rule failure.
// Mutation caught: folding either query's error back into the rule detail.
func TestOverlapLandingOrderExecutionFailureIsNotARuleFailure(t *testing.T) {
	f, parentLanding, childLanding := overlapAC1(t)
	head := gitHeadOf(t, f.root)
	for _, tc := range []struct{ name, failA, failD, want string }{
		{"descends", parentLanding, childLanding, "descends from this feature's landing commit"},
		{"reachable", childLanding, head, "reachable from HEAD"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			installFailingAncestryGit(t, tc.failA, tc.failD)
			ctx := newVerifyRunContext(f.s)
			detail, execErr := overlapLandingOrder(ctx, parentLanding, childLanding)
			if detail != "" || !strings.Contains(execErr, tc.want) {
				t.Fatalf("detail=%q execErr=%q want an execution error about %q", detail, execErr, tc.want)
			}
		})
	}
}

// E5 execution failure end to end: the assessment takes R26's execution
// form, names no descendant and leaves the verdict alone. Mutation caught:
// recording the failure as the descendant's failed_rule E5.
func TestOverlapLandingOrderExecutionFailureUsesR26ExecutionForm(t *testing.T) {
	f, parentLanding, childLanding := overlapAC1(t)
	installFailingAncestryGit(t, parentLanding, childLanding)
	r := f.verify("parent")
	requireBlocked(t, r)
	a := requireClass(t, r, OverlapInvalidOrMissingEvidence, OverlapStepEligibility)
	for _, d := range a.Descendants {
		if d.FailedRule != "" {
			t.Fatalf("an execution failure must not judge a descendant: %+v", a.Descendants)
		}
	}
	if !strings.Contains(a.Detail, "for descendant child") {
		t.Errorf("detail=%q", a.Detail)
	}
	v8 := checkByID(t, r, CheckPostApplyPatchReplayClean)
	if !strings.Contains(v8.Remediation, "could not complete its eligibility step") ||
		!strings.Contains(v8.Remediation, "no descendant was judged") ||
		strings.Contains(v8.Remediation, "cannot be used as evidence") {
		t.Fatalf("V8 must carry R26's execution form: %q", v8.Remediation)
	}
}

// AC-5 + AC-6: identical state gives identical JSON, and the assessment
// leaves the working tree, the real index, `.tpatch/`, the temp-index
// directory and the worktree list untouched.
func TestOverlapIsDeterministicAndReadOnly(t *testing.T) {
	f, _, _ := overlapAC1(t)
	tree, index := snapshotTree(t, f.root), readIndexBytes(t, f.root)
	worktrees := mustGit(t, f.root, "worktree", "list", "--porcelain")
	a, b := f.verify("parent"), f.verify("parent")
	requireClass(t, a, OverlapRecordedDescendantEvolution, "")
	a.VerifiedAt, b.VerifiedAt = "", ""
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatalf("non-deterministic report:\n%s\n%s", ja, jb)
	}
	if d := diffTrees(tree, snapshotTree(t, f.root)); d != "" {
		t.Fatalf("the run changed files: %s", d)
	}
	if string(index) != string(readIndexBytes(t, f.root)) {
		t.Fatal("the run changed the real index")
	}
	if got := mustGit(t, f.root, "worktree", "list", "--porcelain"); got != worktrees {
		t.Fatalf("worktrees changed:\n%s", got)
	}
	if left := listTempIndexes(t, f.root); len(left) != 0 {
		t.Fatalf("temp indexes left behind: %v", left)
	}
	entries, _ := os.ReadDir(filepath.Join(f.root, ".git", "tpatch-verify"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "overlap-patch-") {
			t.Fatalf("staged patch left behind: %s", e.Name())
		}
	}
}

// The persisted record never carries the assessment.
func TestOverlapIsNeverPersisted(t *testing.T) {
	f, _, _ := overlapAC1(t)
	r, _ := RunVerify(f.s, "parent", VerifyOptions{})
	requireClass(t, r, OverlapRecordedDescendantEvolution, "")
	raw, err := os.ReadFile(filepath.Join(f.root, ".tpatch", "features", "parent", "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(raw)), "overlap") || strings.Contains(string(raw), "recorded-descendant") {
		t.Fatalf("status.json carries the assessment:\n%s", raw)
	}
}

// Finding 5 — the nested verify must not prune the descendant's own
// shadow. Mutation caught: shadowOwner returning the bare slug.
func TestOverlapNestedVerifyKeepsDescendantShadow(t *testing.T) {
	f, _, childLanding := overlapAC1(t)
	own, err := gitutil.CreateShadow(f.root, "child", childLanding)
	if err != nil {
		t.Fatalf("CreateShadow: %v", err)
	}
	requireClass(t, f.verify("parent"), OverlapRecordedDescendantEvolution, "")
	if _, err := os.Stat(own); err != nil {
		t.Fatalf("the child's own shadow was pruned by the nested verify: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Join(f.root, ".tpatch", "shadow"))
	for _, e := range entries {
		if strings.Contains(e.Name(), ".overlap-") {
			t.Fatalf("nested shadow left behind: %s", e.Name())
		}
	}
}

// Review rev-1 F2 (Amendment 2 D24): every git probe of verify reads a
// staged copy of the run's captured patch bytes, so no git argv names a
// live feature artifact — on single runs of the target and its descendant
// and under --all. Mutation caught: staging captured bytes under
// .tpatch/features, or passing any live artifact path to git again.
func TestOverlapGitNeverReadsLiveArtifacts(t *testing.T) {
	f, _, _ := overlapAC1(t)
	w := installGitWrapper(t)
	w.Reset()
	requireBlocked(t, f.verify("parent"))
	if r := f.verify("child"); r.Verdict != "passed" {
		t.Fatalf("child verdict=%s", r.Verdict)
	}
	if _, err := RunVerifyAll(f.s, VerifyOptions{NoWrite: true}); err != nil {
		t.Fatal(err)
	}
	calls := w.Calls()
	if len(callsWithSubcommand(calls, "apply")) == 0 {
		t.Fatal("the wrapper saw no git apply; the guard proves nothing")
	}
	live := string(filepath.Separator) + filepath.Join(".tpatch", "features") + string(filepath.Separator)
	for _, c := range calls {
		for _, a := range c.Args {
			if strings.Contains(a, live) {
				t.Fatalf("git read a live feature artifact: %q", c.Args)
			}
		}
	}
}

// Review rev-1 F2: a transient rewrite of a descendant's live patch while
// the run resolves its anchor cannot change the resolution, or poison the
// anchor memo the nested E6 verify and later --all entries share. Mutation
// caught: forward qualification reading the live artifact.
func TestOverlapAnchorResolutionUsesCapturedPatch(t *testing.T) {
	f, _, childLanding := overlapAC1(t)
	want := newVerifyRunContext(f.s).resolveAnchor("child")
	if !want.Available || want.ReplayFrom != childLanding {
		t.Fatalf("baseline resolution=%+v; the fixture must qualify the child's landing", want)
	}
	ctx := newVerifyRunContext(f.s)
	live := artifactPath(f.root, "child", "post-apply.patch")
	orig, err := os.ReadFile(live)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(live, []byte("not a patch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ctx.resolveAnchor("child")
	if err := os.WriteFile(live, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	if *got != *want {
		t.Fatalf("a transient live rewrite changed the anchor: got %+v want %+v", got, want)
	}
	if again := ctx.resolveAnchor("child"); *again != *want {
		t.Fatalf("the shared anchor memo was poisoned: %+v", again)
	}
}

// Finding 3 — the nested verify reads the run's captured HEAD. Mutation
// caught: any literal "HEAD" left on the nested path (the live HEAD has
// moved to a tree without the child's change).
func TestOverlapNestedVerifyIsPinnedToCapturedHead(t *testing.T) {
	f, _, _ := overlapAC1(t)
	ctx := newVerifyRunContext(f.s)
	f.edit(ovPath, featureBody(nil))
	if got := ctx.sameRunDescendantVerify(f.s, "child"); !got.passed {
		t.Fatalf("nested verify followed the live HEAD: %s", got.detail)
	}
	if ctx.headRev() != "HEAD" || ctx.overlapNested {
		t.Fatalf("nested mode leaked out of the nested run: headRev=%s nested=%v", ctx.headRev(), ctx.overlapNested)
	}
}

// Finding 6 — the nested verify restores the run's mutable state and
// never writes through to the run's evidence entries. Mutation caught:
// removing the restore, or a shallow memo copy.
func TestOverlapNestedVerifyIsIsolated(t *testing.T) {
	f, _, _ := overlapAC1(t)
	ctx := newVerifyRunContext(f.s)
	ev := ctx.classifyEvidence("child")
	sentinel := []VerifyAdvisory{{Code: "sentinel"}}
	ctx.pendingAdvisories = sentinel
	ctx.pendingMemberBaselines = map[string]string{"x": "y"}
	memo := ctx.evidenceMemo
	if _, err := ctx.runNestedVerify(f.s, "child"); err != nil {
		t.Fatalf("nested verify: %v", err)
	}
	if len(ctx.pendingAdvisories) != 1 || ctx.pendingAdvisories[0].Code != "sentinel" ||
		ctx.pendingMemberBaselines["x"] != "y" || ctx.overlapNested {
		t.Fatalf("run state not restored: adv=%v baselines=%v nested=%v", ctx.pendingAdvisories, ctx.pendingMemberBaselines, ctx.overlapNested)
	}
	if fmt.Sprintf("%p", ctx.evidenceMemo) != fmt.Sprintf("%p", memo) || ctx.evidenceMemo["child"] != ev {
		t.Fatal("the run's evidence memo was replaced")
	}

	clone := cloneEvidenceMemo(memo)
	clone["child"].Evidence.State = EvidenceUnavailable
	if clone["child"].Evidence.PatchSHAMatch != nil {
		*clone["child"].Evidence.PatchSHAMatch = !*clone["child"].Evidence.PatchSHAMatch
	}
	if ev.Evidence.State != EvidenceExact || (ev.Evidence.PatchSHAMatch != nil && !*ev.Evidence.PatchSHAMatch) {
		t.Fatalf("a clone write reached the run's entry: %+v", ev.Evidence)
	}

	// A nested verify whose probes cannot run rewrites the descendant's
	// evidence state in place (the landed pipeline's terminal family). It
	// must rewrite its private copy, never the entry E1 already classified.
	installFailingApplyGit(t, "fatal: simulated apply failure")
	report, err := ctx.runNestedVerify(f.s, "child")
	if err != nil {
		t.Fatalf("nested verify under the failing shim: %v", err)
	}
	if report.LandingEvidence == nil || report.LandingEvidence.State == EvidenceExact {
		t.Fatalf("precondition: the shim did not make the nested verify rewrite its evidence: %+v", report.LandingEvidence)
	}
	if ctx.evidenceMemo["child"] != ev || ev.Evidence.State != EvidenceExact {
		t.Fatalf("the nested verify wrote through to the run's evidence entry: %+v", ev.Evidence)
	}
}

// `verify --all` gives every feature the same report it gets on its own,
// with the same counts. Mutation caught: nested state leaking between
// features of one run.
func TestOverlapVerifyAllMatchesSingleRuns(t *testing.T) {
	f, _, _ := overlapAC1(t)
	agg, err := RunVerifyAll(f.s, VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerifyAll: %v", err)
	}
	if agg.Summary.Failed != 1 || agg.Summary.Passed != 1 {
		t.Fatalf("summary=%+v", agg.Summary)
	}
	for _, e := range agg.Features {
		single := f.verify(e.Slug)
		e.Report.VerifiedAt, single.VerifiedAt = "", ""
		ja, _ := json.Marshal(e.Report)
		jb, _ := json.Marshal(single)
		if string(ja) != string(jb) {
			t.Errorf("%s differs under --all:\n%s\n%s", e.Slug, ja, jb)
		}
	}
}

// D21 — HEAD moving during the run drops the assessment and restores the
// plain remediation. Mutation caught: removing the headStillCaptured drop.
func TestOverlapDroppedWhenHeadMoves(t *testing.T) {
	f, _, _ := overlapAC1(t)
	moved := strings.TrimSpace(mustGit(t, f.root, "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "moved"))
	realGit := mustLookGit(t)
	dir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
%q "$@"; rc=$?
case " $* " in *" diff-index "*)
  if [ ! -f %q ]; then : > %q; %q -C %q update-ref refs/heads/main %s; fi;;
esac
exit $rc
`, realGit, filepath.Join(dir, "done"), filepath.Join(dir, "done"), realGit, f.root, moved)
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	r := f.verify("parent")
	if _, err := os.Stat(filepath.Join(dir, "done")); err != nil {
		t.Fatal("the shim never saw the compare step")
	}
	requireBlocked(t, r)
	if r.OverlapAssessment != nil {
		t.Fatalf("an assessment computed against a HEAD that moved must be dropped: %+v", r.OverlapAssessment)
	}
	if v8 := checkByID(t, r, CheckPostApplyPatchReplayClean); strings.Contains(v8.Remediation, "overlap assessment") ||
		r.LandingEvidence == nil || r.LandingEvidence.AttestationCommit == "" ||
		!strings.Contains(v8.Remediation, "inspect with git diff "+r.LandingEvidence.AttestationCommit+" HEAD") {
		t.Fatalf("the R25 remediation must be restored to R1/R2: %q", v8.Remediation)
	}
}

// D21 drop for a context-only-drift assessment (review rev-1 F4): the
// assessment replaced no remediation, so dropping it must leave V8's own
// remediation — here a historical failure's — in place, while a blocked
// ladder's drop still restores the plain R1/R2. Mutation caught: restoring
// plainV8Remediation for every class, which blanks the historical text.
func TestOverlapDropRestoresOnlyAReplacedRemediation(t *testing.T) {
	const historical = "landed feature: HISTORICAL R5"
	report := func(rem string) *VerifyReport {
		return &VerifyReport{
			OverlapAssessment: contextOnlyDriftAssessment(ovPath),
			Checks:            []store.VerifyCheckResult{{ID: CheckPostApplyPatchReplayClean, Remediation: rem}},
		}
	}
	r := report(historical)
	dropOverlapAssessment(r, dynamicPhase{overlap: r.OverlapAssessment})
	if r.OverlapAssessment != nil || r.Checks[0].Remediation != historical {
		t.Fatalf("context-only drop: assessment=%+v remediation=%q", r.OverlapAssessment, r.Checks[0].Remediation)
	}
	r = report("R25 text")
	dropOverlapAssessment(r, dynamicPhase{overlap: r.OverlapAssessment, overlapReplacedV8: true, plainV8Remediation: "R1 text"})
	if r.OverlapAssessment != nil || r.Checks[0].Remediation != "R1 text" {
		t.Fatalf("blocked drop: assessment=%+v remediation=%q", r.OverlapAssessment, r.Checks[0].Remediation)
	}
}

// D21 — an unstable inventory drops the assessment and keeps R20.
func TestOverlapDroppedWhenInventoryUnstable(t *testing.T) {
	f, _, _ := overlapAC1(t)
	installMutatingGitWrapper(t, filepath.Join(f.root, ".tpatch", "features", "child", "artifacts", "post-apply.patch"))
	r := f.verify("parent")
	if r.FailedAt != FailedAtSnapshotUnstable {
		t.Fatalf("failed_at=%q want snapshot-unstable", r.FailedAt)
	}
	if r.OverlapAssessment != nil {
		t.Fatalf("an unstable run must not carry an assessment: %+v", r.OverlapAssessment)
	}
	if v8 := checkByID(t, r, CheckPostApplyPatchReplayClean); !strings.HasPrefix(v8.Remediation, "verify aborted: child/") {
		t.Fatalf("R20 must stay V8's remediation on an unstable run: %q", v8.Remediation)
	}
}

// D26 — remediation by class, including the R26 execution form (finding 7).
func TestOverlapRemediationByClass(t *testing.T) {
	const fb = "FALLBACK"
	cases := []struct {
		a    *VerifyOverlapAssessment
		want string
	}{
		{nil, fb},
		{&VerifyOverlapAssessment{Class: OverlapUnexplainedContentLoss}, fb},
		{&VerifyOverlapAssessment{Class: OverlapContextOnlyDrift}, fb},
		{&VerifyOverlapAssessment{Class: OverlapRecordedDescendantEvolution,
			Descendants: []VerifyOverlapDescendant{{Slug: "c", AttestationCommit: "abc"}}}, "recorded descendant(s) c @ abc rewrote it"},
		{&VerifyOverlapAssessment{Class: OverlapInvalidOrMissingEvidence,
			Descendants: []VerifyOverlapDescendant{{Slug: "ok"}, {Slug: "bad", FailedRule: "E5", Detail: "d"}}}, "recorded descendant bad may explain it but cannot be used as evidence (E5: d)"},
		{&VerifyOverlapAssessment{Class: OverlapInvalidOrMissingEvidence, FailedStep: OverlapStepReplay, Detail: "git failed"},
			"could not complete its replay step (git failed) — no descendant was judged"},
	}
	for _, tc := range cases {
		got := overlapRemediation(tc.a, "p", "sha", fb)
		if (tc.want == fb && got != fb) || !strings.Contains(got, tc.want) {
			t.Errorf("overlapRemediation(%+v)=%q want %q", tc.a, got, tc.want)
		}
	}
}

// ── Source guards ────────────────────────────────────────────────────────

func readWorkflowSource(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(docsRootForTest(t), "internal", "workflow", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// overlapSourceViolations lists forbidden reads in the assessment source:
// it must apply captured bytes (D24) against the captured HEAD (D23).
func overlapSourceViolations(src string) []string {
	var out []string
	for _, bad := range []string{"artifactPath(", "os.ReadFile(", "os.Open(", "ReadArtifact(", `"HEAD"`} {
		if strings.Contains(src, bad) {
			out = append(out, bad)
		}
	}
	return out
}

func TestOverlapSourceUsesCapturedBytesAndHead(t *testing.T) {
	for _, mutant := range []string{
		"x := artifactPath(ctx.root, c.slug, \"post-apply.patch\")",
		"data, _ := os.ReadFile(p)",
		"idx.ReadTree(\"HEAD\")",
	} {
		if len(overlapSourceViolations(mutant)) == 0 {
			t.Fatalf("the guard cannot see %q", mutant)
		}
	}
	if v := overlapSourceViolations(readWorkflowSource(t, "verify_overlap.go")); len(v) != 0 {
		t.Fatalf("verify_overlap.go reads outside the run's captures: %v", v)
	}
}

// overlapConsumers lists production files outside verify that reference
// the assessment: it is explanation-only and nothing may act on it.
func overlapConsumers(files map[string]string) []string {
	allowed := map[string]bool{
		"internal/workflow/verify.go":          true,
		"internal/workflow/verify_overlap.go":  true,
		"internal/workflow/verify_anchored.go": true,
	}
	re := regexp.MustCompile(`\bOverlapAssessment\b|\bVerifyOverlapAssessment\b|\boverlap_assessment\b`)
	var out []string
	for name, src := range files {
		if !allowed[name] && re.MatchString(src) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func TestOverlapAssessmentHasNoConsumers(t *testing.T) {
	mutant := map[string]string{"internal/workflow/status_view.go": "if r.OverlapAssessment != nil { ok = true }"}
	if got := overlapConsumers(mutant); len(got) != 1 {
		t.Fatalf("the guard cannot see a consumer: %v", got)
	}
	root := docsRootForTest(t)
	files := map[string]string{}
	for _, dir := range []string{"internal", "cmd"} {
		_ = filepath.Walk(filepath.Join(root, dir), func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return err
			}
			data, rerr := os.ReadFile(p)
			if rerr != nil {
				return rerr
			}
			rel, _ := filepath.Rel(root, p)
			files[filepath.ToSlash(rel)] = string(data)
			return nil
		})
	}
	if len(files) < 50 {
		t.Fatalf("scanned only %d files", len(files))
	}
	if got := overlapConsumers(files); len(got) != 0 {
		t.Fatalf("production code outside verify consumes the overlap assessment: %v", got)
	}
}
