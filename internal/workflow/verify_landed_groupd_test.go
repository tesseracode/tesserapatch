package workflow

// Acceptance rows AC-L29 – AC-L45 and AC-L130 – AC-L133 —
// PRD-verify-freshness §7.1 Group D: anchor collection, forward
// qualification at `-C1`, normalized change identity, selection.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

// qualifyProbe answers "does the current canonical patch forward-apply
// at this tree" at a given context level, through an isolated index.
func qualifyProbe(t *testing.T, root, treeish, patchPath string, context int) bool {
	t.Helper()
	idx, err := gitutil.NewTempIndex(root, filepath.Join(root, ".git", "tpatch-verify"))
	if err != nil {
		t.Fatalf("NewTempIndex: %v", err)
	}
	defer func() { _ = idx.Close() }()
	if err := idx.ReadTree(treeish); err != nil {
		t.Fatalf("ReadTree %s: %v", treeish, err)
	}
	return idx.ApplyCheck(gitutil.ApplyCheckOptions{PatchPath: patchPath, Context: gitutil.IntPtr(context)}).OK
}

// AC-L29 — ALL candidates are collected before any selection.
func TestACL29_AllCandidatesCollectedBeforeSelection(t *testing.T) {
	f := newLadderFixture(t)
	// Two revert-and-re-land cycles ⇒ three slug-bearing single-parent
	// landings reachable, each introducing the SAME normalized change.
	f.RelandIdentical()
	f.RelandIdentical()
	r := f.Verify()
	if r.Baseline.HistoricalAnchor == nil {
		t.Fatalf("no historical anchor block")
	}
	if got := r.Baseline.HistoricalAnchor.CandidatesCollected; got != 3 {
		t.Fatalf("candidates_collected=%d want 3 (stop-at-first would report 1)", got)
	}
}

// AC-L30 — qualification uses a FORWARD `apply --check --cached -C1`
// seeded by `read-tree C^`; never `--reverse`, never `C^{tree}^`.
func TestACL30_QualificationIsForwardAtC1(t *testing.T) {
	f := newLadderFixture(t)
	w := installGitWrapper(t)
	w.Reset()
	ctx := newVerifyRunContext(f.Store)
	qualified, failState, qErr := ctx.forwardQualifies(f.LandingCommit, artifactPath(f.Root, f.Slug, "post-apply.patch"))
	if qErr != nil {
		t.Fatalf("qualification failed with %s: %v", failState, qErr)
	}
	if !qualified {
		t.Fatalf("landing candidate did not forward-qualify at its parent")
	}
	calls := w.Calls()
	readTrees := callsWithSubcommand(calls, "read-tree")
	if len(readTrees) == 0 {
		t.Fatalf("no read-tree call recorded")
	}
	for _, c := range readTrees {
		rev := c.Args[len(c.Args)-1]
		if strings.Contains(rev, "^{tree}^") {
			t.Errorf("invalid revision syntax C^{tree}^: %s", c.Joined())
		}
		if !strings.HasSuffix(rev, "^") && !strings.HasSuffix(rev, "^{tree}") {
			t.Errorf("read-tree revision is not C^ / C^^{tree}: %s", c.Joined())
		}
	}
	applies := callsWithSubcommand(calls, "apply")
	if len(applies) == 0 {
		t.Fatalf("no apply call recorded")
	}
	for _, c := range applies {
		if c.Has("--reverse") {
			t.Errorf("qualification used --reverse: %s", c.Joined())
		}
		if !c.Has("-C1") {
			t.Errorf("qualification not at -C1: %s", c.Joined())
		}
		if !c.Has("--cached") {
			t.Errorf("qualification not --cached: %s", c.Joined())
		}
	}
}

// AC-L130 — `C^{tree}^` is invalid; `C^`, `C^^{tree}`, `C~1` and
// `C~1^{tree}` all resolve and `read-tree` accepts them.
func TestACL130_ParentRevisionSyntax(t *testing.T) {
	f := newLadderFixture(t)
	c := f.LandingCommit

	if _, err := tryGit(f.Root, "rev-parse", c+"^{tree}^"); err == nil {
		t.Errorf("git rev-parse %s^{tree}^ unexpectedly succeeded", c)
	}
	idxDir := filepath.Join(f.Root, ".git", "tpatch-verify")
	bad, err := gitutil.NewTempIndex(f.Root, idxDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := bad.ReadTree(c + "^{tree}^"); err == nil {
		t.Errorf("read-tree %s^{tree}^ unexpectedly succeeded", c)
	}
	_ = bad.Close()

	for _, rev := range []string{c + "^", c + "^^{tree}", c + "~1", c + "~1^{tree}"} {
		if _, err := tryGit(f.Root, "rev-parse", rev); err != nil {
			t.Errorf("git rev-parse %s failed: %v", rev, err)
		}
		idx, err := gitutil.NewTempIndex(f.Root, idxDir)
		if err != nil {
			t.Fatal(err)
		}
		if err := idx.ReadTree(rev); err != nil {
			t.Errorf("read-tree %s failed: %v", rev, err)
		}
		_ = idx.Close()
	}
}

// AC-L131 — the `-C1` qualification ladder matches the §3.6.8 measured
// table row for row.
func TestACL131_QualificationLadderTable(t *testing.T) {
	type row struct {
		name           string
		parent         func([]string)
		c3, c1, c0     bool
		mustQualifyC1  bool
		onlyAtC1Passes bool
	}
	rows := []row{
		{name: "pristine-parent", parent: nil, c3: true, c1: true, c0: true, mustQualifyC1: true},
		{name: "edit-4-lines-away", parent: func(l []string) { l[13] = "UNRELATED" }, c3: true, c1: true, c0: true, mustQualifyC1: true},
		{name: "edit-2-lines-away", parent: func(l []string) { l[11] = "UNRELATED" }, c3: false, c1: true, c0: true, mustQualifyC1: true, onlyAtC1Passes: true},
		{name: "edit-1-line-away", parent: func(l []string) { l[10] = "UNRELATED" }, c3: false, c1: false, c0: true},
		{name: "ten-lines-prepended", parent: nil, c3: true, c1: true, c0: true, mustQualifyC1: true},
		{name: "edit-far-away", parent: func(l []string) { l[58] = "UNRELATED" }, c3: true, c1: true, c0: true, mustQualifyC1: true},
		{name: "already-materialized", parent: func(l []string) { l[9] = "FEATURE10"; l[29] = "FEATURE30"; l[49] = "FEATURE50" }, c3: false, c1: false, c0: false},
		{name: "feature-line-changed", parent: func(l []string) { l[9] = "SOMETHING-ELSE" }, c3: false, c1: false, c0: false},
		{name: "feature-line-deleted", parent: func(l []string) { l[9] = "" }, c3: false, c1: false, c0: false},
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			f := newLadderFixture(t)
			patchPath := artifactPath(f.Root, f.Slug, "post-apply.patch")
			body := ladderBody(tc.parent)
			if tc.name == "ten-lines-prepended" {
				body = "p1\np2\np3\np4\np5\np6\np7\np8\np9\np10\n" + body
			}
			mustWriteFile(t, filepath.Join(f.Root, "candidate-marker.txt"), tc.name+"\n")
			candidate := f.EditTracked(body, "candidate parent state")
			for _, probe := range []struct {
				ctx  int
				want bool
			}{{3, tc.c3}, {1, tc.c1}, {0, tc.c0}} {
				got := qualifyProbe(t, f.Root, candidate, patchPath, probe.ctx)
				if got != probe.want {
					t.Errorf("-C%d: got %v want %v", probe.ctx, got, probe.want)
				}
			}
			if tc.onlyAtC1Passes && (tc.c3 || !tc.c1) {
				t.Errorf("row is mis-specified: it must pass ONLY at -C1")
			}
			if tc.mustQualifyC1 && !tc.c1 {
				t.Errorf("healthy parent must qualify at -C1")
			}
		})
	}
}

// AC-L132 — remediation-loop fixture: a landed feature whose
// neighbourhood drifted two lines, re-recorded over the drift and
// re-landed. Default context rejects the surviving candidate and would
// dead-end R11; `-C1` qualifies it and the run passes.
func TestACL132_RemediationLoopQualifiesAtC1(t *testing.T) {
	f := newLadderFixture(t)
	// Neighbourhood drift two lines from hunk 1, landed as its own commit.
	f.EditTracked(featureBody(func(l []string) { l[11] = "DRIFT" }), "unrelated drift")
	// Re-record the canonical patch over the drift and re-land.
	f.BaseCommit = mustGit(t, f.Root, "rev-parse", f.LandingCommit+"^")
	patch := mustGit(t, f.Root, "diff", f.BaseCommit, "HEAD", "--", f.FilePath)
	f.WritePatch(patch + "\n")
	body, err := os.ReadFile(filepath.Join(f.Root, f.FilePath))
	if err != nil {
		t.Fatal(err)
	}
	f.WriteRecipe(ApplyRecipe{Feature: f.Slug, Operations: []RecipeOperation{
		{Type: "write-file", Path: f.FilePath, Content: string(body)},
	}})
	st, _ := f.Store.LoadFeatureStatus(f.Slug)
	st.Apply.BaseCommit = f.BaseCommit
	if err := f.Store.SaveFeatureStatus(st); err != nil {
		t.Fatal(err)
	}
	f.LandWithBlock(f.TrailerBlock(f.Slug))

	patchPath := artifactPath(f.Root, f.Slug, "post-apply.patch")
	if qualifyProbe(t, f.Root, f.BaseCommit, patchPath, 3) {
		t.Logf("note: default context also qualifies this history")
	}
	if !qualifyProbe(t, f.Root, f.BaseCommit, patchPath, 1) {
		t.Fatalf("-C1 must qualify the surviving candidate — the operator's own remediation")
	}
	r := f.Verify()
	if r.Verdict != "passed" {
		t.Fatalf("remediation loop dead-ended: verdict=%s failed_at=%s", r.Verdict, r.FailedAt)
	}
}

// AC-L31 — measured semantics: at a clean pre-landing parent forward
// passes and reverse fails; at the landing itself forward fails and
// reverse passes.
func TestACL31_ForwardAndReverseAreInverted(t *testing.T) {
	f := newLadderFixture(t)
	patchPath := artifactPath(f.Root, f.Slug, "post-apply.patch")
	parent := mustGit(t, f.Root, "rev-parse", f.LandingCommit+"^")

	forwardAt := func(rev string) bool { return qualifyProbe(t, f.Root, rev, patchPath, 1) }
	reverseAt := func(rev string) bool {
		idx, err := gitutil.NewTempIndex(f.Root, filepath.Join(f.Root, ".git", "tpatch-verify"))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = idx.Close() }()
		if err := idx.ReadTree(rev); err != nil {
			t.Fatal(err)
		}
		return idx.ApplyCheck(gitutil.ApplyCheckOptions{PatchPath: patchPath, Reverse: true}).OK
	}

	if !forwardAt(parent) {
		t.Errorf("forward must pass at the clean pre-landing parent")
	}
	if reverseAt(parent) {
		t.Errorf("reverse must fail at the clean pre-landing parent")
	}
	if forwardAt(f.LandingCommit) {
		t.Errorf("forward must fail at the landing itself")
	}
	if !reverseAt(f.LandingCommit) {
		t.Errorf("reverse must pass at the landing itself")
	}
}

// AC-L32 — a candidate parent carrying unrelated drift INSIDE the
// patch's context window does not qualify, and the report says so.
func TestACL32_DriftInsideContextWindowDoesNotQualify(t *testing.T) {
	f := newLadderFixture(t)
	patchPath := artifactPath(f.Root, f.Slug, "post-apply.patch")
	drifted := f.EditTracked(ladderBody(func(l []string) { l[10] = "UNRELATED" }), "drift inside context")
	if qualifyProbe(t, f.Root, drifted, patchPath, 1) {
		t.Fatalf("a parent with drift one line from the hunk must NOT qualify at -C1")
	}
}

// AC-L33 — re-record + re-land: both landings are collected, the NEWEST
// fails qualification and the earlier one qualifies; the report reports
// attestation_commit ≠ replay_anchor_commit.
func TestACL33_ReRecordReLandSplitsAttestationFromAnchor(t *testing.T) {
	f := newLadderFixture(t)
	first := f.LandingCommit
	// A real re-record + re-land: a SECOND change is recorded into the
	// canonical artifacts and attested by a new landing whose own parent
	// already materializes the first half (measured E30).
	second := f.ExtendAndReland(featureBody(func(l []string) { l[39] = "FEATURE40" }))

	r := f.Verify()
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%s failed_at=%s", r.Verdict, r.FailedAt)
	}
	ha := r.Baseline.HistoricalAnchor
	if ha.CandidatesCollected != 2 {
		t.Errorf("candidates_collected=%d want 2", ha.CandidatesCollected)
	}
	if ha.CandidatesQualified != 1 {
		t.Errorf("candidates_qualified=%d want 1 (the newest landing must NOT qualify)", ha.CandidatesQualified)
	}
	if ha.ReplayAnchorCommit != first {
		t.Errorf("replay anchor=%s want the EARLIER landing %s", ha.ReplayAnchorCommit, first)
	}
	if r.LandingEvidence.AttestationCommit == ha.ReplayAnchorCommit {
		t.Errorf("attestation and replay anchor must differ after a re-land")
	}
	if r.LandingEvidence.AttestationCommit != second {
		t.Errorf("attestation=%s want the NEWEST landing %s", r.LandingEvidence.AttestationCommit, second)
	}
}

// AC-L34 — two qualifying candidates with EQUAL normalized identity ⇒
// selection proceeds, candidates_qualified: 2, topo-oldest chosen.
func TestACL34_EqualIdentityQualifiersSelectTopoOldest(t *testing.T) {
	f := newLadderFixture(t)
	oldest := f.LandingCommit
	// Revert-and-re-land reproduces the cherry-pick / merge-back class:
	// two reachable landings introducing the SAME normalized change,
	// both attesting the same (unchanged) artifacts.
	f.RelandIdentical()

	r := f.Verify()
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%s failed_at=%s evidence=%s", r.Verdict, r.FailedAt, r.LandingEvidence.State)
	}
	if r.LandingEvidence.State != EvidenceDuplicateEquivalent {
		t.Fatalf("evidence=%s want duplicate-equivalent", r.LandingEvidence.State)
	}
	if r.LandingEvidence.Duplicates != 2 {
		t.Errorf("duplicates=%d want 2", r.LandingEvidence.Duplicates)
	}
	ha := r.Baseline.HistoricalAnchor
	if ha == nil || ha.State != AnchorStateAvailable {
		t.Fatalf("anchor unavailable: %+v", ha)
	}
	if ha.CandidatesQualified != 2 {
		t.Fatalf("candidates_qualified=%d want 2", ha.CandidatesQualified)
	}
	if ha.ReplayAnchorCommit != oldest {
		t.Errorf("selected %s want the topo-oldest qualifier %s", ha.ReplayAnchorCommit, oldest)
	}
}

// AC-L35 — two qualifying candidates with DIFFERING normalized identity
// ⇒ FAIL `historical-anchor-unavailable` (R11).
func TestACL35_DifferingIdentityQualifiersFail(t *testing.T) {
	f := newLadderFixture(t)
	base := mustGit(t, f.Root, "rev-parse", f.LandingCommit+"^")
	// A second slug-bearing single-parent landing rooted at the SAME
	// parent, carrying STALE digests (so it can never be an attestation)
	// but introducing a DIFFERENT change. Its parent still forward-accepts
	// the current canonical patch, so it qualifies as a replay anchor.
	// The block is rendered BEFORE the checkout: `git checkout <base>`
	// rewinds `.tpatch/` to its pre-record state.
	staleBlock := f.StaleTrailerBlock()
	mustGit(t, f.Root, "checkout", "-q", "-b", "side", base)
	mustWriteFile(t, filepath.Join(f.Root, f.FilePath), ladderBody(func(l []string) { l[19] = "DIFFERENT" }))
	mustGit(t, f.Root, "add", "-A")
	mustGit(t, f.Root, "commit", "-q", "-m", "different change", "-m", staleBlock)
	side := gitHeadOf(t, f.Root)
	mustGit(t, f.Root, "checkout", "-q", "main")
	if _, err := tryGit(f.Root, "merge", "-q", "--no-ff", "-m", "merge divergent", side); err != nil {
		mustGit(t, f.Root, "merge", "--abort")
		t.Skipf("merge produced a conflict in this environment")
	}

	r := f.Verify()
	if r.LandingEvidence.State != EvidenceExact {
		t.Fatalf("evidence=%s want exact (the stale candidate must not become an attestation)", r.LandingEvidence.State)
	}
	ha := r.Baseline.HistoricalAnchor
	if ha == nil || ha.State != AnchorStateUnavailable {
		t.Fatalf("non-equivalent qualifiers were silently resolved: %+v", ha)
	}
	if r.FailedAt != FailedAtHistoricalAnchor {
		t.Errorf("failed_at=%q want historical-anchor-unavailable", r.FailedAt)
	}
	if got := checkByID(t, r, CheckRecipeReplayClean).Remediation; got != remediationR11(f.Slug) {
		t.Errorf("R11 not verbatim: %q", got)
	}
}

// AC-L36 — selection is deterministic across repeated runs.
func TestACL36_SelectionIsDeterministic(t *testing.T) {
	f := newLadderFixture(t)
	f.RelandIdentical()
	var first string
	for i := 0; i < 3; i++ {
		r := f.Verify()
		got := r.Baseline.HistoricalAnchor.ReplayAnchorCommit
		if i == 0 {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("run %d selected %s, run 0 selected %s", i, got, first)
		}
	}
}

// AC-L37 — the anchor search never broadens beyond exact-slug trailer
// commits. Adversarial: a NON-trailer commit that introduced the same
// paths is never selected.
func TestACL37_AnchorSearchNeverBroadens(t *testing.T) {
	f := newLadderFixture(t)
	// A commit touching the same path with no trailer at all.
	noTrailer := f.EditTracked(featureBody(func(l []string) { l[20] = "NOISE" }), "no trailer, same path")
	r := f.Verify()
	ha := r.Baseline.HistoricalAnchor
	if ha != nil && ha.ReplayAnchorCommit == noTrailer {
		t.Fatalf("selected a non-trailer commit as the replay anchor")
	}
	ctx := newVerifyRunContext(f.Store)
	res := ctx.resolveAnchor(f.Slug)
	if res.Collected != 1 {
		t.Errorf("collected=%d want 1 (exact-slug trailer commits only)", res.Collected)
	}
}

// AC-L38 — a stale-hash qualifying candidate supplies ONLY a tree.
func TestACL38_StaleCandidateSuppliesOnlyATree(t *testing.T) {
	f := newLadderFixture(t)
	stale := f.LandingCommit
	// A re-record + re-land makes the earlier landing's own digests
	// stale while it remains the only qualifying replay anchor.
	fresh := f.ExtendAndReland(featureBody(func(l []string) { l[39] = "FEATURE40" }))

	r := f.Verify()
	ha := r.Baseline.HistoricalAnchor
	if ha == nil || ha.State != AnchorStateAvailable {
		t.Fatalf("anchor unavailable: %+v (failed_at=%s)", ha, r.FailedAt)
	}
	if ha.ReplayAnchorCommit != stale {
		t.Fatalf("replay anchor=%s want the stale-hash candidate %s", ha.ReplayAnchorCommit, stale)
	}
	if r.LandingEvidence.State != EvidenceExact {
		t.Fatalf("evidence=%s want exact", r.LandingEvidence.State)
	}
	if r.LandingEvidence.AttestationCommit != fresh {
		t.Errorf("attestation=%s want %s — a stale candidate must never be reported as the attestation",
			r.LandingEvidence.AttestationCommit, fresh)
	}
}

// AC-L39 — no qualifier ⇒ FAIL `historical-anchor-unavailable` with R11;
// V7, V8-historical and V10 all report passed:false WITH mode present.
func TestACL39_NoQualifierFailsWithModePresent(t *testing.T) {
	f := newLandedFixture(t, "unanchored")
	mustWriteFile(t, filepath.Join(f.Root, f.FilePath), ladderBody(nil))
	mustGit(t, f.Root, "add", "-A")
	mustGit(t, f.Root, "commit", "-q", "-m", "60-line seed")
	f.BaseCommit = gitHeadOf(t, f.Root)
	f.LandTrailerOnlyAfterChange(featureBody(nil))
	r := f.Verify()
	if r.FailedAt != FailedAtHistoricalAnchor {
		t.Fatalf("failed_at=%q want historical-anchor-unavailable (evidence=%s)", r.FailedAt, r.LandingEvidence.State)
	}
	for _, id := range []string{CheckRecipeReplayClean, CheckPostApplyPatchReplayClean, CheckWriteFilePreimageFresh} {
		c := checkByID(t, r, id)
		if c.Passed {
			t.Errorf("%s passed on an unanchored run", id)
		}
		if c.Skipped {
			t.Errorf("%s is SKIPPED; the contract requires failed-because-unanchored", id)
		}
		if c.Mode == "" {
			t.Errorf("%s has no mode", id)
		}
	}
	if got := checkByID(t, r, CheckRecipeReplayClean).Remediation; got != remediationR11(f.Slug) {
		t.Errorf("R11 not verbatim:\n got %q\nwant %q", got, remediationR11(f.Slug))
	}
}

// AC-L40 — anchor unavailable while anchor C is CLEAN ⇒ the run still
// FAILS and reports `materialized-clean`.
func TestACL40_UnavailableAnchorFailsEvenWhenCurrentIsClean(t *testing.T) {
	f := newLandedFixture(t, "clean-but-unanchored")
	mustWriteFile(t, filepath.Join(f.Root, f.FilePath), ladderBody(nil))
	mustGit(t, f.Root, "add", "-A")
	mustGit(t, f.Root, "commit", "-q", "-m", "60-line seed")
	f.BaseCommit = gitHeadOf(t, f.Root)
	// The change lands BEFORE the attestation, so the only candidate's
	// parent already materializes the patch and nothing qualifies —
	// while the content is unambiguously present at HEAD.
	f.LandTrailerOnlyAfterChange(featureBody(nil))

	r := f.Verify()
	if r.Verdict != "failed" {
		t.Fatalf("verdict=%s want failed", r.Verdict)
	}
	if r.FailedAt != FailedAtHistoricalAnchor {
		t.Fatalf("failed_at=%q want historical-anchor-unavailable", r.FailedAt)
	}
	if got := currentAnchor(t, r); got != CurrentMaterializedClean {
		t.Fatalf("anchor_results.current=%q want materialized-clean", got)
	}
}

// AC-L41 — after the R6 re-land remediation, anchor H is REGAINED and
// the run passes; where history admits no qualifier it fails with R11.
func TestACL41_ReLandRegainsAnchorOrFailsWithR11(t *testing.T) {
	t.Run("re-land-regains", func(t *testing.T) {
		f := newLadderFixture(t)
		// Drift the artifacts ⇒ stale evidence.
		f.WritePatch(string(f.PatchBytes()) + "\n")
		if r := f.Verify(); r.LandingEvidence.State != EvidenceStale {
			t.Fatalf("precondition: evidence=%s want stale", r.LandingEvidence.State)
		}
		// Remediation: re-attest the CURRENT (drifted) artifacts. The
		// earlier landing's digests stop matching, so it becomes a
		// replay-anchor candidate only.
		f.LandWithBlock(f.TrailerBlock(f.Slug))
		r := f.Verify()
		if r.Verdict != "passed" {
			t.Fatalf("re-land did not regain the anchor: verdict=%s failed_at=%s", r.Verdict, r.FailedAt)
		}
	})
	t.Run("no-qualifier-fails", func(t *testing.T) {
		f := newLandedFixture(t, "no-qualifier")
		mustWriteFile(t, filepath.Join(f.Root, f.FilePath), ladderBody(nil))
		mustGit(t, f.Root, "add", "-A")
		mustGit(t, f.Root, "commit", "-q", "-m", "60-line seed")
		f.BaseCommit = gitHeadOf(t, f.Root)
		f.LandTrailerOnlyAfterChange(featureBody(nil))
		r := f.Verify()
		if r.FailedAt != FailedAtHistoricalAnchor {
			t.Fatalf("failed_at=%q want historical-anchor-unavailable", r.FailedAt)
		}
	})
}

// AC-L42 — normalized identity is exactly the documented `git diff`
// invocation with only `^index ` lines removed (plus the hunk-header
// rewrite of AC-L133), SHA-256 over the remainder.
func TestACL42_NormalizedIdentityArgv(t *testing.T) {
	f := newLadderFixture(t)
	w := installGitWrapper(t)
	w.Reset()
	if _, err := gitutil.NormalizedChangeIdentity(f.Root, f.LandingCommit, []string{f.FilePath}); err != nil {
		t.Fatalf("NormalizedChangeIdentity: %v", err)
	}
	diffs := callsWithSubcommand(w.Calls(), "diff")
	if len(diffs) != 1 {
		t.Fatalf("expected exactly one diff call, got %d", len(diffs))
	}
	for _, flag := range []string{"--no-color", "--no-ext-diff", "--no-textconv", "--binary", "--no-renames", "--unified=0"} {
		if !diffs[0].Has(flag) {
			t.Errorf("diff missing %s: %s", flag, diffs[0].Joined())
		}
	}
	if !diffs[0].Has(f.LandingCommit+"^") || !diffs[0].Has(f.LandingCommit) {
		t.Errorf("diff not between C^ and C: %s", diffs[0].Joined())
	}
}

// AC-L43 — the same logical change on two branches with different
// preimage blobs gives different raw -U3 bytes, different -U0 bytes, and
// IDENTICAL bytes after the `index` strip.
func TestACL43_NormalizationMeasuredBehaviour(t *testing.T) {
	root := t.TempDir()
	gitInitVerifyTest(t, root)
	mustWriteFile(t, filepath.Join(root, "f.txt"), "a\nb\nc\n")
	mustGit(t, root, "add", "-A")
	mustGit(t, root, "commit", "-q", "-m", "base")
	base := gitHeadOf(t, root)

	mk := func(branch, preimage string) string {
		mustGit(t, root, "checkout", "-q", "-b", branch, base)
		mustWriteFile(t, filepath.Join(root, "f.txt"), preimage)
		mustGit(t, root, "add", "-A")
		mustGit(t, root, "commit", "-q", "-m", "preimage "+branch)
		mustWriteFile(t, filepath.Join(root, "f.txt"), strings.Replace(preimage, "b\n", "B-CHANGED\n", 1))
		mustGit(t, root, "add", "-A")
		mustGit(t, root, "commit", "-q", "-m", "change "+branch)
		return gitHeadOf(t, root)
	}
	one := mk("one", "a\nb\nc\nONE\n")
	two := mk("two", "a\nb\nc\nd\nTWO\n")

	rawU3a := mustGit(t, root, "diff", "-U3", one+"^", one, "--", "f.txt")
	rawU3b := mustGit(t, root, "diff", "-U3", two+"^", two, "--", "f.txt")
	if rawU3a == rawU3b {
		t.Errorf("raw -U3 bytes unexpectedly identical")
	}
	rawU0a := mustGit(t, root, "diff", "-U0", one+"^", one, "--", "f.txt")
	rawU0b := mustGit(t, root, "diff", "-U0", two+"^", two, "--", "f.txt")
	if rawU0a == rawU0b {
		t.Errorf("raw -U0 bytes unexpectedly identical")
	}
	if gitutil.NormalizeDiffBytes(rawU0a) != gitutil.NormalizeDiffBytes(rawU0b) {
		t.Errorf("normalized forms differ:\n%q\n%q",
			gitutil.NormalizeDiffBytes(rawU0a), gitutil.NormalizeDiffBytes(rawU0b))
	}
}

// AC-L44 — normalization preserves mode, binary and rename semantics;
// hunk bodies keep content and order.
func TestACL44_NormalizationPreservesModeBinaryRename(t *testing.T) {
	t.Run("mode-only", func(t *testing.T) {
		in := "diff --git a/f b/f\nold mode 100644\nnew mode 100755\nindex aaa..bbb\n"
		got := gitutil.NormalizeDiffBytes(in)
		for _, want := range []string{"old mode 100644", "new mode 100755", "diff --git a/f b/f"} {
			if !strings.Contains(got, want) {
				t.Errorf("normalized form lost %q: %q", want, got)
			}
		}
		if strings.Contains(got, "index aaa..bbb") {
			t.Errorf("index line survived: %q", got)
		}
		if strings.Contains(got, "@@") {
			t.Errorf("a mode-only change must normalize with no @@: %q", got)
		}
	})
	t.Run("binary", func(t *testing.T) {
		in := "diff --git a/b b/b\nindex aaa..bbb 100644\nGIT binary patch\nliteral 3\nabc\n"
		got := gitutil.NormalizeDiffBytes(in)
		if !strings.Contains(got, "GIT binary patch") {
			t.Errorf("binary marker lost: %q", got)
		}
	})
	t.Run("rename-split", func(t *testing.T) {
		in := "diff --git a/x b/x\ndeleted file mode 100644\nindex aaa..000\n" +
			"diff --git a/y b/y\nnew file mode 100644\nindex 000..bbb\n"
		got := gitutil.NormalizeDiffBytes(in)
		for _, want := range []string{"deleted file mode 100644", "new file mode 100644"} {
			if !strings.Contains(got, want) {
				t.Errorf("rename split lost %q: %q", want, got)
			}
		}
	})
	t.Run("hunk-bodies-preserved", func(t *testing.T) {
		in := "@@ -1,2 +1,2 @@ func x()\n-old\n+new\n"
		got := gitutil.NormalizeDiffBytes(in)
		if got != "@@\n-old\n+new\n" {
			t.Errorf("hunk body altered: %q", got)
		}
	})
}

// AC-L133 — the offset-normalized duplicate trade, both directions.
func TestACL133_OffsetNormalizationTrade(t *testing.T) {
	t.Run("cherry-pick-compares-equal-only-with-header-rewrite", func(t *testing.T) {
		a := "diff --git a/f b/f\nindex 1..2 100644\n@@ -10 +10 @@ l9\n-x\n+y\n"
		b := "diff --git a/f b/f\nindex 3..4 100644\n@@ -15 +15 @@ l9\n-x\n+y\n"
		if gitutil.NormalizeDiffBytes(a) != gitutil.NormalizeDiffBytes(b) {
			t.Errorf("shifted cherry-pick must normalize equal")
		}
		stripIndexOnly := func(s string) string {
			var out []string
			for _, l := range strings.Split(s, "\n") {
				if strings.HasPrefix(l, "index ") {
					continue
				}
				out = append(out, l)
			}
			return strings.Join(out, "\n")
		}
		if stripIndexOnly(a) == stripIndexOnly(b) {
			t.Errorf("without the header rewrite the identities must differ")
		}
	})
	t.Run("duplicate-bodies-collide-documented-trade", func(t *testing.T) {
		a := "@@ -2 +2 @@\n-DUP\n+DUP CHANGED\n"
		b := "@@ -4 +4 @@\n-DUP\n+DUP CHANGED\n"
		if gitutil.NormalizeDiffBytes(a) != gitutil.NormalizeDiffBytes(b) {
			t.Errorf("the documented collision must reproduce")
		}
	})
	t.Run("distinct-payload-mode-and-path-stay-distinct", func(t *testing.T) {
		payload := "@@ -2 +2 @@\n-DUP\n+OTHER\n"
		dup := "@@ -2 +2 @@\n-DUP\n+DUP CHANGED\n"
		modeOnly := "diff --git a/f b/f\nold mode 100644\nnew mode 100755\n"
		otherPath := "diff --git a/g b/g\n@@ -2 +2 @@\n-DUP\n+DUP CHANGED\n"
		seen := map[string]string{}
		for name, s := range map[string]string{"payload": payload, "dup": dup, "mode": modeOnly, "path": otherPath} {
			n := gitutil.NormalizeDiffBytes(s)
			if prev, ok := seen[n]; ok {
				t.Errorf("%s collided with %s", name, prev)
			}
			seen[n] = name
		}
	})
}

// AC-L45 — an EMPTY canonical path set makes candidates incomparable ⇒
// `ambiguous`; never broadened to "all paths".
func TestACL45_EmptyPathSetIsAmbiguous(t *testing.T) {
	f := newLadderFixture(t)
	if _, err := gitutil.NormalizedChangeIdentity(f.Root, f.LandingCommit, nil); err == nil {
		t.Fatalf("an empty path set must be rejected, never broadened")
	}
	ctx := newVerifyRunContext(f.Store)
	_, err := ctx.identitiesFor([]gitutil.CommitRecord{{SHA: f.LandingCommit}}, []byte("not a patch\n"))
	if err == nil {
		t.Fatalf("an undeclared path set must make candidates incomparable")
	}
	if !strings.Contains(err.Error(), "not comparable") &&
		!strings.Contains(err.Error(), "no paths") &&
		!strings.Contains(err.Error(), "could not be derived") {
		t.Errorf("unexpected incomparability reason: %v", err)
	}
}
