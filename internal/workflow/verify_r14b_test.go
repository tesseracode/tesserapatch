package workflow

// ADR-013 Amendment 4 slice N2 (GH #26): D42 splits R14 on ancestry
// (R14b) and D43 never reads an ancestry query that could not answer as
// an answer. Each test names the mutation it catches.

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

// a4T3 is A4-E4: the child lands on the parent's first landing; the
// parent is unapplied and re-lands with a changed patch; the child is not
// re-landed, so its only landing's parent does not contain the parent's
// current landing.
func a4T3(t *testing.T) (f *overlapFixture, childLanding, parentRelanding string) {
	t.Helper()
	f = newOverlapFixture(t)
	f.edit(a4ParentPath, ladderBody(nil))
	f.feature("parent")
	f.land("parent", a4ParentPath, a4ParentBody("PARENT-V1"), true)
	f.feature("child", "parent")
	childLanding = f.land("child", ovPath, a4ChildBody(), true)
	f.edit(a4ParentPath, ladderBody(nil))
	parentRelanding = f.land("parent", a4ParentPath, a4ParentBody("PARENT-V2"), true)
	return f, childLanding, parentRelanding
}

// requireParentLandingDrift asserts what D42 leaves unchanged (verdict,
// exit code, failed_at, the named parent) and returns V7's remediation.
func requireParentLandingDrift(t *testing.T, r *VerifyReport) string {
	t.Helper()
	v7 := checkByID(t, r, CheckRecipeReplayClean)
	if r.Verdict != "failed" || r.ExitCode != 2 || r.FailedAt != FailedAtParentLandingDrift || r.ParentSlug != "parent" {
		t.Fatalf("verdict=%s exit=%d failed_at=%q parent=%q v7=%q want failed/2/parent-landing-drift/parent",
			r.Verdict, r.ExitCode, r.FailedAt, r.ParentSlug, v7.Remediation)
	}
	return v7.Remediation
}

// requireReplayAnchor asserts a landed-mode run whose replay anchor is
// landing, and returns the anchor commit: the landing's parent in root.
func requireReplayAnchor(t *testing.T, root string, r *VerifyReport, landing string) string {
	t.Helper()
	if r.Baseline == nil {
		t.Fatalf("precondition: landed mode with replay anchor %s; the report has no baseline (evidence=%s)", landing, r.LandingEvidence.State)
	}
	ha := r.Baseline.HistoricalAnchor
	if r.LandingEvidence.State != EvidenceExact || ha == nil || ha.ReplayAnchorCommit != landing {
		t.Fatalf("precondition: landed mode with replay anchor %s; evidence=%s anchor=%+v", landing, r.LandingEvidence.State, ha)
	}
	parent := mustGit(t, root, "rev-parse", landing+"^")
	if ha.Commit != parent {
		t.Fatalf("precondition: anchor commit %s want the landing's parent %s", ha.Commit, parent)
	}
	return parent
}

// requireR14bUndetermined asserts R14b's undetermined form for parent and
// target, naming the replay anchor, with a detail that contains detail.
func requireR14bUndetermined(t *testing.T, got, target, replayAnchor, detail string) {
	t.Helper()
	parts := strings.SplitN(remediationR14bUndetermined("parent", target, replayAnchor, "\x00"), "\x00", 2)
	if len(got) < len(parts[0])+len(parts[1]) || !strings.HasPrefix(got, parts[0]) || !strings.HasSuffix(got, parts[1]) {
		t.Fatalf("want R14b's undetermined form; got %q", got)
	}
	if mid := got[len(parts[0]) : len(got)-len(parts[1])]; !strings.Contains(mid, detail) {
		t.Fatalf("the undetermined detail %q does not contain %q", mid, detail)
	}
}

// requireAncestry asserts `git merge-base --is-ancestor` answers want (a
// determined yes or no) for the pair in root.
func requireAncestry(t *testing.T, root, ancestor, descendant string, want bool) {
	t.Helper()
	_, err := tryGit(root, "merge-base", "--is-ancestor", ancestor, descendant)
	var exitErr *exec.ExitError
	switch {
	case err == nil && want, errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && !want:
	default:
		t.Fatalf("precondition: is-ancestor %s %s want %v, got %v", ancestor, descendant, want, err)
	}
}

// T3 (D42): the parent re-landed and the child did not. Mutation caught:
// the ancestry query dropped (R14 always), or the replay anchor never
// passed to arbitration (forward-mode R14).
func TestA4T3RelandedParentGivesR14b(t *testing.T) {
	f, childLanding, parentRelanding := a4T3(t)
	r := f.verify("child")
	anchor := requireReplayAnchor(t, f.root, r, childLanding)
	requireAncestry(t, f.root, parentRelanding, anchor, false)
	got := requireParentLandingDrift(t, r)
	if want := remediationR14b("parent", parentRelanding, "child", childLanding); got != want {
		t.Fatalf("want R14b naming both commits:\n got %q\nwant %q", got, want)
	}
	requireAggregateMatchesSingle(t, f.s)
}

// T4 (D42, A4-E5): a later edit rewrites the parent's line before the
// child lands, so the parent's attestation is in the anchor's history and
// its content is absent there. Mutation caught: R14b always.
func TestA4T4AncestorAttestationKeepsR14(t *testing.T) {
	f := newOverlapFixture(t)
	f.edit(a4ParentPath, ladderBody(nil))
	f.feature("parent")
	parentLanding := f.land("parent", a4ParentPath, a4ParentBody("PARENT-V1"), true)
	f.feature("child", "parent")
	f.edit(a4ParentPath, a4ParentBody("REWRITTEN"))
	childLanding := f.land("child", ovPath, a4ChildBody(), true)

	r := f.verify("child")
	anchor := requireReplayAnchor(t, f.root, r, childLanding)
	requireAncestry(t, f.root, parentLanding, anchor, true)
	if got, want := requireParentLandingDrift(t, r), remediationR14("parent", parentLanding); got != want {
		t.Fatalf("want R14:\n got %q\nwant %q", got, want)
	}
}

// T9 (D43): T3 with the ancestry query failing. Mutation caught: the
// failure read as "not an ancestor" (R14b's claim) or as "ancestor" (R14).
func TestA4T9FailedAncestryQueryIsUndetermined(t *testing.T) {
	skipWithoutPOSIXGitShim(t)
	f, childLanding, parentRelanding := a4T3(t)
	anchor := requireReplayAnchor(t, f.root, f.verify("child"), childLanding)
	installFailingAncestryGit(t, parentRelanding, anchor)
	got := requireParentLandingDrift(t, f.verify("child"))
	requireR14bUndetermined(t, got, "child", childLanding, "simulated object store failure")
}

// T10 (D42): the child is applied and landed on a branch that never
// contained the parent's current landing, then that branch merges main,
// so the parent's landing is reachable only through the merge's
// non-first parent. R14b claims only that no exact landing is in the
// baseline's history. The expected text is spelled out here, not built
// by remediationR14b. Mutation caught: R14b's text restored to a claim
// that the baseline predates the landing.
func TestA4T10DivergentBranchR14bClaimsOnlyAncestry(t *testing.T) {
	f := newOverlapFixture(t)
	f.edit(a4ParentPath, ladderBody(nil))
	f.feature("parent")
	f.land("parent", a4ParentPath, a4ParentBody("PARENT-V1"), true)
	f.feature("child", "parent")
	mustGit(t, f.root, "add", "-A")
	mustGit(t, f.root, "commit", "-q", "-m", "register child")
	mustGit(t, f.root, "switch", "-q", "-c", "side")
	childLanding := f.land("child", ovPath, a4ChildBody(), true)
	mustGit(t, f.root, "switch", "-q", "main")
	f.edit(a4ParentPath, ladderBody(nil))
	parentRelanding := f.land("parent", a4ParentPath, a4ParentBody("PARENT-V2"), true)
	mustGit(t, f.root, "switch", "-q", "side")
	mustGit(t, f.root, "merge", "-q", "--no-ff", "--no-edit", "-m", "merge main", "main")

	requireAncestry(t, f.root, parentRelanding, "HEAD", true)
	if firstParent := mustGit(t, f.root, "rev-list", "--first-parent", "HEAD"); strings.Contains(firstParent, parentRelanding) {
		t.Fatalf("precondition: the parent's landing must be reachable only through the merge's non-first parent")
	}
	r := f.verify("child")
	anchor := requireReplayAnchor(t, f.root, r, childLanding)
	requireAncestry(t, f.root, parentRelanding, anchor, false)
	requireAncestry(t, f.root, anchor, parentRelanding, false)
	got := requireParentLandingDrift(t, r)
	want := "hard parent parent landed at " + parentRelanding + " but its canonical patch is not present at the verification baseline, " +
		"and no exact landing of parent is in the history of that baseline, the parent of child's replay anchor " + childLanding +
		": child was applied before that landing, or on a branch that never contained it, so verifying parent will not help — " +
		"genuinely unapply child (tpatch feature unapply child), re-apply it on parent's current landing (tpatch apply child), " +
		"then run tpatch record child and tpatch land child"
	if got != want {
		t.Fatalf("R14b not verbatim:\n got %q\nwant %q", got, want)
	}
	for _, claim := range []string{"predate", "older"} {
		if strings.Contains(strings.ToLower(got), claim) {
			t.Fatalf("R14b claims more than ancestry (%q): %q", claim, got)
		}
	}
}

// a4Dup is the duplicate-equivalent fixture of T12 and T15. The parent
// lands twice from the same base with the same change and trailer block:
// aMain on main and aSide on a side line. On main a later edit rewrites
// the parent's line and the child lands. The child's anchor contains
// aMain and not aSide; mergeAt joins the side line in either order.
type a4Dup struct {
	f                                  *overlapFixture
	aMain, aSide, anchor, childLanding string
}

func newA4Dup(t *testing.T) *a4Dup {
	t.Helper()
	f := newOverlapFixture(t)
	f.edit(a4ParentPath, ladderBody(nil))
	f.feature("parent")
	aMain := f.land("parent", a4ParentPath, a4ParentBody("PARENT-V1"), true)
	msg := mustGit(t, f.root, "log", "-1", "--format=%B", aMain)
	sideMsg := strings.Replace(msg, "feat: land parent", "feat: land parent on a side line", 1)
	if sideMsg == msg {
		t.Fatalf("precondition: the side landing's subject was not rewritten")
	}
	aSide := mustGit(t, f.root, "commit-tree", aMain+"^{tree}", "-p", aMain+"^", "-m", sideMsg)
	f.feature("child", "parent")
	anchor := f.edit(a4ParentPath, a4ParentBody("REWRITTEN"))
	childLanding := f.land("child", ovPath, a4ChildBody(), true)
	requireAncestry(t, f.root, aMain, anchor, true)
	requireAncestry(t, f.root, aSide, anchor, false)
	return &a4Dup{f: f, aMain: aMain, aSide: aSide, anchor: anchor, childLanding: childLanding}
}

// mergeAt points HEAD at a merge of the child's landing and aSide, with
// aSide as the first parent when sideFirst. Git's topological order, and
// so the duplicates' enumeration order, follows the merge's parent order.
func (d *a4Dup) mergeAt(t *testing.T, sideFirst bool) {
	t.Helper()
	first, second := d.childLanding, d.aSide
	if sideFirst {
		first, second = second, first
	}
	m := mustGit(t, d.f.root, "commit-tree", d.childLanding+"^{tree}", "-p", first, "-p", second, "-m", "merge the side line")
	mustGit(t, d.f.root, "reset", "-q", "--hard", m)
}

// reported classifies the parent at HEAD, requires duplicate-equivalent
// evidence over both landings and returns the reported attestation.
func (d *a4Dup) reported(t *testing.T) string {
	t.Helper()
	ev := newVerifyRunContext(d.f.s).classifyEvidence("parent").Evidence
	if ev.State != EvidenceDuplicateEquivalent {
		t.Fatalf("precondition: parent evidence=%s (%s) want duplicate-equivalent", ev.State, ev.Reason)
	}
	return ev.AttestationCommit
}

// T12 (D42): every exact attestation is queried. In one merge order the
// reported attestation is aSide, outside the anchor's history; aMain is
// inside it. Mutation caught: only attestation_commit queried (R14b).
func TestA4T12AnyExactAttestationInHistoryKeepsR14(t *testing.T) {
	d := newA4Dup(t)
	sawSide := false
	for _, sideFirst := range []bool{true, false} {
		d.mergeAt(t, sideFirst)
		rep := d.reported(t)
		sawSide = sawSide || rep == d.aSide
		r := d.f.verify("child")
		requireReplayAnchor(t, d.f.root, r, d.childLanding)
		if got, want := requireParentLandingDrift(t, r), remediationR14("parent", rep); got != want {
			t.Fatalf("sideFirst=%v: want R14:\n got %q\nwant %q", sideFirst, got, want)
		}
	}
	if !sawSide {
		t.Fatalf("precondition: no merge order reports %s, the attestation outside the anchor's history", d.aSide)
	}
}

// T15 (D42, D43): two exact attestations whose queries answer {failure,
// yes} and {failure, no}, each in both enumeration orders. Mutation
// caught: the first answer decides, or querying stops at a failure.
func TestA4T15AggregateIgnoresEnumerationOrder(t *testing.T) {
	skipWithoutPOSIXGitShim(t)
	d := newA4Dup(t)
	var reports []string
	for _, sideFirst := range []bool{true, false} {
		d.mergeAt(t, sideFirst)
		rep := d.reported(t)
		reports = append(reports, rep)
		for _, tc := range []struct {
			name, fail string
			wantR14    bool
		}{
			{"failure-and-yes", d.aSide, true},
			{"failure-and-no", d.aMain, false},
		} {
			t.Run(fmt.Sprintf("sideFirst=%v/%s", sideFirst, tc.name), func(t *testing.T) {
				installFailingAncestryGit(t, tc.fail, d.anchor)
				got := requireParentLandingDrift(t, d.f.verify("child"))
				if tc.wantR14 {
					if want := remediationR14("parent", rep); got != want {
						t.Fatalf("want R14:\n got %q\nwant %q", got, want)
					}
					return
				}
				requireR14bUndetermined(t, got, "child", d.childLanding, "simulated object store failure")
			})
		}
	}
	if reports[0] == reports[1] {
		t.Fatalf("precondition: both merge orders enumerate the duplicates alike (reported %s)", reports[0])
	}
}

// T13 (D42): a forward-mode target (evidence none) whose landed hard
// parent is reverted at HEAD keeps R14 and runs no ancestry query; the
// shim makes any such query visible as R14b's undetermined form.
// Mutation caught: the split applied in forward mode.
func TestA4T13ForwardModeKeepsR14WithoutAQuery(t *testing.T) {
	skipWithoutPOSIXGitShim(t)
	c, parent, child := buildLandedParentChain(t, poisonRecipe("parent"), "parent-seed\nPARENT-CHANGE\n")
	parentLanding := gitHeadOf(t, c.Root)
	mustWriteFile(t, filepath.Join(c.Root, "src", "parent.txt"), "parent-seed\n")
	mustGit(t, c.Root, "add", "-A")
	mustGit(t, c.Root, "commit", "-q", "-m", "revert the parent")
	installFailingAncestryGit(t, parentLanding, gitHeadOf(t, c.Root))

	r := c.Verify(child)
	if r.LandingEvidence.State != EvidenceNone {
		t.Fatalf("precondition: forward mode needs evidence none; got %s", r.LandingEvidence.State)
	}
	if r.FailedAt != FailedAtParentLandingDrift || r.ParentSlug != parent || r.ExitCode != 2 {
		t.Fatalf("failed_at=%q parent=%q exit=%d", r.FailedAt, r.ParentSlug, r.ExitCode)
	}
	if got, want := checkByID(t, r, CheckRecipeReplayClean).Remediation, remediationR14(parent, parentLanding); got != want {
		t.Fatalf("want R14:\n got %q\nwant %q", got, want)
	}
}

// T16 (D43): in a shallow clone the parent's landing and the child's
// anchor are present with their parents, but the history joining them is
// cut (a merge reaches the landing by a shorter path), so Git answers no.
// Control: the same history unshallowed. Mutation caught: a shallow no
// read as determined (R14b's claim).
func TestA4T16ShallowNoIsUndetermined(t *testing.T) {
	f := newOverlapFixture(t)
	f.edit(a4ParentPath, ladderBody(nil))
	f.feature("parent")
	parentLanding := f.land("parent", a4ParentPath, a4ParentBody("PARENT-V1"), true)
	f.feature("child", "parent")
	f.edit(a4ParentPath, a4ParentBody("REWRITTEN"))
	for i := 1; i <= 5; i++ {
		f.edit(fmt.Sprintf("src/filler-%d.txt", i), "filler\n")
	}
	childLanding := f.land("child", ovPath, a4ChildBody(), true)
	m := mustGit(t, f.root, "commit-tree", childLanding+"^{tree}", "-p", childLanding, "-p", parentLanding, "-m", "merge the parent's landing")
	mustGit(t, f.root, "reset", "-q", "--hard", m)

	clone := t.TempDir()
	if out, err := tryGit(clone, "clone", "-q", "--depth", "4", "file://"+f.root, "."); err != nil {
		t.Fatalf("shallow clone: %v\n%s", err, out)
	}
	if facts, err := gitutil.ReadRepoFacts(clone); err != nil || !facts.Shallow {
		t.Fatalf("precondition: the clone must be shallow (%v)", err)
	}
	anchor := mustGit(t, clone, "rev-parse", childLanding+"^")
	for _, c := range []string{parentLanding + "^", anchor + "^"} {
		mustGit(t, clone, "cat-file", "-e", c+"^{commit}")
	}
	requireAncestry(t, clone, parentLanding, anchor, false)
	s, err := storeOpen(clone)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	verifyClone := func() *VerifyReport {
		r, _ := RunVerify(s, "child", VerifyOptions{NoWrite: true})
		if r == nil {
			t.Fatalf("no report")
		}
		requireReplayAnchor(t, clone, r, childLanding)
		return r
	}
	requireR14bUndetermined(t, requireParentLandingDrift(t, verifyClone()), "child", childLanding, "shallow clone")

	mustGit(t, clone, "fetch", "-q", "--unshallow")
	requireAncestry(t, clone, parentLanding, anchor, true)
	if got, want := requireParentLandingDrift(t, verifyClone()), remediationR14("parent", parentLanding); got != want {
		t.Fatalf("control: want R14 once unshallowed:\n got %q\nwant %q", got, want)
	}
}

// D42/D43 aggregation over every permutation of each answer set. Mutation
// caught: an order-dependent rule (the first answer decides, a failure
// stops the queries), a failure read as an answer, a shallow no read as
// determined, or the query issued against something other than the anchor.
func TestA4AncestryAggregationDependsOnlyOnTheAnswerSet(t *testing.T) {
	cases := []struct {
		answers []string
		shallow bool
		want    ancestryOutcome
	}{
		{nil, false, ancestryUndetermined},
		{[]string{"yes"}, false, ancestryInHistory},
		{[]string{"no"}, false, ancestryNotInHistory},
		{[]string{"fail"}, false, ancestryUndetermined},
		{[]string{"no", "no"}, false, ancestryNotInHistory},
		{[]string{"no", "yes"}, false, ancestryInHistory},
		{[]string{"fail", "yes"}, false, ancestryInHistory},
		{[]string{"fail", "no"}, false, ancestryUndetermined},
		{[]string{"fail", "no", "yes"}, false, ancestryInHistory},
		{[]string{"fail", "no", "no"}, false, ancestryUndetermined},
		{[]string{"no"}, true, ancestryUndetermined},
		{[]string{"yes"}, true, ancestryInHistory},
		{[]string{"no", "yes"}, true, ancestryInHistory},
		{[]string{"no", "no"}, true, ancestryUndetermined},
	}
	for _, tc := range cases {
		for _, perm := range permutations(tc.answers) {
			atts := make([]string, len(perm))
			answer := map[string]string{}
			for i, a := range perm {
				atts[i] = fmt.Sprintf("att%d", i)
				answer[atts[i]] = a
			}
			query := func(ancestor, descendant string) (bool, error) {
				if descendant != "anchor" {
					t.Errorf("queried %s against %q, not the anchor", ancestor, descendant)
				}
				switch answer[ancestor] {
				case "yes":
					return true, nil
				case "no":
					return false, nil
				}
				return false, fmt.Errorf("simulated failure")
			}
			got, detail := memberLandingInAnchorHistory(atts, "anchor", tc.shallow, query)
			if got != tc.want {
				t.Errorf("answers=%v shallow=%v: outcome=%d want %d", perm, tc.shallow, got, tc.want)
			}
			if (got == ancestryUndetermined) != (detail != "") {
				t.Errorf("answers=%v shallow=%v: outcome=%d detail=%q", perm, tc.shallow, got, detail)
			}
		}
	}
}

func permutations(xs []string) [][]string {
	if len(xs) <= 1 {
		return [][]string{append([]string(nil), xs...)}
	}
	var out [][]string
	for i := range xs {
		rest := append(append([]string(nil), xs[:i]...), xs[i+1:]...)
		for _, p := range permutations(rest) {
			out = append(out, append([]string{xs[i]}, p...))
		}
	}
	return out
}
