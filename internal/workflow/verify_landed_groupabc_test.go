package workflow

// Acceptance rows AC-L1 – AC-L28 plus AC-L129 / AC-L134 —
// PRD-verify-freshness §7.1 Groups A (defect + schema), B (anchor-C
// isolation) and C (the hardened ladder).

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

// ladderBody renders the 60-line file the §3.6.5 measured table uses.
func ladderBody(mut func(lines []string)) string {
	lines := make([]string, 0, 60)
	for i := 1; i <= 60; i++ {
		lines = append(lines, fmt.Sprintf("l%d", i))
	}
	if mut != nil {
		mut(lines)
	}
	return strings.Join(lines, "\n") + "\n"
}

func featureBody(extra func([]string)) string {
	return ladderBody(func(l []string) {
		l[9] = "FEATURE10"
		l[29] = "FEATURE30"
		l[49] = "FEATURE50"
		if extra != nil {
			extra(l)
		}
	})
}

// newLadderFixture lands a 3-hunk feature into a 60-line file and
// returns the fixture positioned at the landing commit.
func newLadderFixture(t *testing.T) *landedFixture {
	t.Helper()
	f := newLandedFixture(t, "ladder-feature")
	mustWriteFile(t, filepath.Join(f.Root, f.FilePath), ladderBody(nil))
	mustGit(t, f.Root, "add", "-A")
	mustGit(t, f.Root, "commit", "-q", "-m", "60-line seed")
	f.BaseCommit = gitHeadOf(t, f.Root)
	f.LandUserChange(featureBody(nil))
	return f
}

// mutateHead commits a post-landing edit on top of the feature.
func mutateHead(t *testing.T, f *landedFixture, extra func([]string)) {
	t.Helper()
	f.EditTracked(featureBody(extra), "post-landing edit")
}

func currentAnchor(t *testing.T, r *VerifyReport) string {
	t.Helper()
	return checkByID(t, r, CheckPostApplyPatchReplayClean).AnchorResults["current"]
}

// ── Group A ──────────────────────────────────────────────────────────────

// AC-L2 — the same feature after `land` passes: target_mode landed,
// evidence exact, baseline dual-anchor, eleven rows.
func TestACL2_LandedFeaturePassesWithExactEvidence(t *testing.T) {
	f := newLadderFixture(t)
	r := f.Verify()
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%s failed_at=%s", r.Verdict, r.FailedAt)
	}
	if r.TargetMode != TargetModeLanded {
		t.Errorf("target_mode=%q want landed", r.TargetMode)
	}
	if r.LandingEvidence.State != EvidenceExact {
		t.Errorf("evidence=%q reason=%q want exact", r.LandingEvidence.State, r.LandingEvidence.Reason)
	}
	if r.Baseline.Mode != BaselineModeDual {
		t.Errorf("baseline.mode=%q want dual-anchor", r.Baseline.Mode)
	}
	if len(r.Checks) != 11 {
		t.Errorf("checks=%d want 11", len(r.Checks))
	}
}

// AC-L4 — a landed LEAF with no dependencies passes.
func TestACL4_LandedLeafPasses(t *testing.T) {
	f := newLandedFixture(t, "leaf")
	f.LandUserChange("l1\nl2\nLEAF\nl4\nl5\nl6\nl7\nl8\nl9\nl10\nl11\nl12\nl13\nl14\nl15\n")
	r := f.Verify()
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%s failed_at=%s v8=%+v", r.Verdict, r.FailedAt, checkByID(t, r, CheckPostApplyPatchReplayClean))
	}
	st, err := f.Store.LoadFeatureStatus(f.Slug)
	if err != nil {
		t.Fatalf("load status: %v", err)
	}
	if len(st.DependsOn) != 0 {
		t.Fatalf("fixture is not a leaf")
	}
}

// AC-L5 — every report emits exactly eleven check rows whose ids equal
// the shipped constants, in order. Golden assertion.
func TestACL5_ElevenCheckRowsInOrder(t *testing.T) {
	want := []string{
		CheckStatusLoaded, CheckIntentFilesPresent, CheckRecipeParses,
		CheckRecipeOpTargetsResolve, CheckDepMetadataValid, CheckSatisfiedByReachable,
		CheckDependencyGateSatisfied, CheckRecipeReplayClean, CheckPostApplyPatchReplayClean,
		CheckReconcileOutcomeConsistent, CheckWriteFilePreimageFresh,
	}
	f := newLadderFixture(t)
	for _, phase := range []string{"landed-pass", "landed-fail"} {
		if phase == "landed-fail" {
			mutateHead(t, f, func(l []string) { l[9] = "l10" }) // full revert of hunk 1
		}
		r := f.Verify()
		if len(r.Checks) != len(want) {
			t.Fatalf("%s: %d checks, want 11", phase, len(r.Checks))
		}
		for i, id := range want {
			if r.Checks[i].ID != id {
				t.Fatalf("%s: checks[%d]=%q want %q", phase, i, r.Checks[i].ID, id)
			}
		}
	}
}

// AC-L6 — `--no-write` leaves `.tpatch/`, the real index and the worktree
// byte-identical.
func TestACL6_NoWriteLeavesEverythingByteIdentical(t *testing.T) {
	f := newLadderFixture(t)
	before := snapshotTree(t, f.Root)
	beforeIndex := readIndexBytes(t, f.Root)
	beforeStatus := mustGit(t, f.Root, "status", "--porcelain")

	if r := f.Verify(); r.Verdict != "passed" {
		t.Fatalf("verdict=%s", r.Verdict)
	}

	if diff := diffTrees(before, snapshotTree(t, f.Root)); diff != "" {
		t.Errorf("--no-write mutated the tree: %s", diff)
	}
	if string(beforeIndex) != string(readIndexBytes(t, f.Root)) {
		t.Errorf("--no-write mutated the real index")
	}
	if got := mustGit(t, f.Root, "status", "--porcelain"); got != beforeStatus {
		t.Errorf("git status changed: %q -> %q", beforeStatus, got)
	}
}

// ── Group B — anchor C isolation ─────────────────────────────────────────

// AC-L7 — every anchor-C and qualification `apply` call carries
// `--cached` against a `read-tree`-seeded temp index.
func TestACL7_EveryApplyCallIsCached(t *testing.T) {
	f := newLadderFixture(t)
	w := installGitWrapper(t)
	w.Reset()
	f.Verify()
	applies := callsWithSubcommand(w.Calls(), "apply")
	if len(applies) == 0 {
		t.Fatalf("no apply calls recorded")
	}
	for _, c := range applies {
		// The shadow's forward V8 check runs inside a worktree, not the
		// isolated index; every OTHER apply must be --cached.
		if c.Has("--check") && !c.Has("--cached") && !c.Has("--reverse") && !c.Has("-C1") {
			continue // the anchor-H forward check inside the shadow
		}
		if !c.Has("--cached") {
			t.Errorf("apply without --cached: %s", c.Joined())
		}
	}
}

// AC-L8 — landed target with a DIRTY WORKTREE (feature reverted in the
// worktree only) still passes. The rev-1 worktree-based check false-reds.
func TestACL8_DirtyWorktreeStillPasses(t *testing.T) {
	f := newLadderFixture(t)
	mustWriteFile(t, filepath.Join(f.Root, f.FilePath), ladderBody(nil)) // revert in the worktree only
	r := f.Verify()
	if r.Verdict != "passed" {
		t.Fatalf("dirty worktree false-red: verdict=%s v8=%+v", r.Verdict, checkByID(t, r, CheckPostApplyPatchReplayClean))
	}
	if currentAnchor(t, r) != CurrentMaterializedClean {
		t.Errorf("current=%q want materialized-clean", currentAnchor(t, r))
	}
}

// AC-L9 — landed target with a DIRTY INDEX passes; the real index is
// never read.
func TestACL9_DirtyIndexStillPasses(t *testing.T) {
	f := newLadderFixture(t)
	mustWriteFile(t, filepath.Join(f.Root, "unrelated.txt"), "staged\n")
	mustGit(t, f.Root, "add", "unrelated.txt")
	mustWriteFile(t, filepath.Join(f.Root, f.FilePath), ladderBody(nil))
	mustGit(t, f.Root, "add", f.FilePath)
	r := f.Verify()
	if r.Verdict != "passed" {
		t.Fatalf("dirty index false-red: verdict=%s", r.Verdict)
	}
}

// AC-L10 — worktree contains the feature but HEAD does not ⇒ anchor C
// blocks.
func TestACL10_WorktreeOnlyFeatureBlocksAtAnchorC(t *testing.T) {
	f := newLadderFixture(t)
	// Remove the feature from HEAD, keep it in the worktree.
	f.EditTracked(ladderBody(nil), "revert at HEAD")
	mustWriteFile(t, filepath.Join(f.Root, f.FilePath), featureBody(nil))
	r := f.Verify()
	if r.Verdict != "failed" {
		t.Fatalf("expected anchor C to block; verdict=%s current=%s", r.Verdict, currentAnchor(t, r))
	}
	if currentAnchor(t, r) != CurrentAbsent {
		t.Errorf("current=%q want absent", currentAnchor(t, r))
	}
}

// AC-L11 — after any landed run the real index and worktree are
// byte-identical, `git status --porcelain -z` is unchanged, and the temp
// index is not an untracked entry.
func TestACL11_RunIsReadOnlyOnIndexAndWorktree(t *testing.T) {
	f := newLadderFixture(t)
	beforeIndex := readIndexBytes(t, f.Root)
	beforeTree := snapshotTree(t, f.Root)
	beforeStatus := mustGit(t, f.Root, "status", "--porcelain", "-z")

	f.Verify()

	if string(beforeIndex) != string(readIndexBytes(t, f.Root)) {
		t.Errorf("real index changed")
	}
	if d := diffTrees(beforeTree, snapshotTree(t, f.Root)); d != "" {
		t.Errorf("worktree changed: %s", d)
	}
	after := mustGit(t, f.Root, "status", "--porcelain", "-z")
	if after != beforeStatus {
		t.Errorf("git status changed: %q -> %q", beforeStatus, after)
	}
	if strings.Contains(after, "verify-index") {
		t.Errorf("temp index leaked into git status: %q", after)
	}
}

// AC-L12 — the temp index is removed on EVERY exit path, including each
// terminal state.
func TestACL12_TempIndexRemovedOnEveryExitPath(t *testing.T) {
	cases := []struct {
		name    string
		arrange func(t *testing.T, f *landedFixture)
	}{
		{"pass", func(t *testing.T, f *landedFixture) {}},
		{"landed-content-absent", func(t *testing.T, f *landedFixture) {
			mutateHead(t, f, func(l []string) { l[9] = "l10" })
		}},
		{"stale-evidence", func(t *testing.T, f *landedFixture) {
			f.WritePatch(string(f.PatchBytes()) + "\n")
		}},
		{"historical-anchor-unavailable", func(t *testing.T, f *landedFixture) {
			// Rewrite the canonical patch so no parent tree accepts it.
			f.WritePatch(unrelatedPatch())
			relandWithCurrentArtifacts(t, f)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newLadderFixture(t)
			tc.arrange(t, f)
			f.Verify()
			leftovers := listTempIndexes(t, f.Root)
			if len(leftovers) != 0 {
				t.Errorf("temp index leaked on %s: %v", tc.name, leftovers)
			}
		})
	}
}

// AC-L13 — the temp index is created OUTSIDE the tracked working tree
// (inside the gitignored `.tpatch/local/` root).
func TestACL13_TempIndexOutsideTrackedTree(t *testing.T) {
	f := newLadderFixture(t)
	ctx := newVerifyRunContext(f.Store)
	dir := ctx.tempIndexDir()
	gitDir := mustGit(t, f.Root, "rev-parse", "--path-format=absolute", "--git-dir")
	// D11: the isolated index lives under the git dir (measured E24:
	// invisible to `git status`, never an untracked entry) — never
	// inside the tracked working tree.
	if !strings.HasPrefix(dir, gitDir) {
		t.Fatalf("temp index dir=%q is not under the git dir %q", dir, gitDir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	idx, err := gitutil.NewTempIndex(f.Root, dir)
	if err != nil {
		t.Fatalf("NewTempIndex: %v", err)
	}
	if err := idx.ReadTree("HEAD"); err != nil {
		t.Fatalf("ReadTree: %v", err)
	}
	if out := mustGit(t, f.Root, "status", "--porcelain"); strings.Contains(out, "tpatch-verify") {
		t.Errorf("temp index visible to git status: %q", out)
	}
	if err := idx.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// AC-L129 — EVERY git invocation issued by verify carries
// GIT_NO_LAZY_FETCH=1.
func TestACL129_EveryGitCallCarriesNoLazyFetch(t *testing.T) {
	f := newLadderFixture(t)
	w := installGitWrapper(t)
	w.Reset()
	f.Verify()
	calls := w.Calls()
	if len(calls) == 0 {
		t.Fatalf("no git calls recorded")
	}
	interesting := map[string]bool{
		"log": true, "read-tree": true, "apply": true, "diff": true,
		"merge-base": true, "cat-file": true, "rev-parse": true,
	}
	seen := 0
	for _, c := range calls {
		if !interesting[c.Subcommand()] {
			continue
		}
		seen++
		if c.Env["GIT_NO_LAZY_FETCH"] != "1" {
			t.Errorf("git %s without GIT_NO_LAZY_FETCH=1", c.Joined())
		}
	}
	if seen == 0 {
		t.Fatalf("no object/enumeration calls observed")
	}
}

// AC-L134 — Git-floor preflight. Below 2.36 the run reports
// `unavailable` with R10 verbatim and issues ONLY `--version`.
func TestACL134_GitFloorPreflight(t *testing.T) {
	t.Run("below-floor", func(t *testing.T) {
		f := newLadderFixture(t)
		w := installFakeVersionGit(t, "git version 2.30.2")
		w.Reset()
		r := f.Verify()
		if r.LandingEvidence.State != EvidenceUnavailable {
			t.Fatalf("evidence=%q want unavailable", r.LandingEvidence.State)
		}
		v7 := checkByID(t, r, CheckRecipeReplayClean)
		want := remediationR10(f.Slug, fmt.Sprintf("git %s is below the required floor 2.36", "git version 2.30.2"))
		if v7.Remediation != want {
			t.Errorf("R10 not verbatim:\n got %q\nwant %q", v7.Remediation, want)
		}
		for _, c := range w.Calls() {
			if c.Subcommand() != "--version" && !c.Has("--version") {
				t.Errorf("below-floor run issued git %s", c.Joined())
			}
		}
	})
	t.Run("at-floor-proceeds", func(t *testing.T) {
		f := newLadderFixture(t)
		if r := f.Verify(); r.Verdict != "passed" {
			t.Fatalf("real git (>= 2.36) run did not proceed: %s", r.Verdict)
		}
	})
}

// ── Group C — the hardened ladder ────────────────────────────────────────

// AC-L14 — step 1 passes ⇒ materialized-clean, no advisory.
func TestACL14_LadderStep1CleanNoAdvisory(t *testing.T) {
	f := newLadderFixture(t)
	r := f.Verify()
	if currentAnchor(t, r) != CurrentMaterializedClean {
		t.Fatalf("current=%q", currentAnchor(t, r))
	}
	if hasAdvisory(r, AdvisoryContextDrift) {
		t.Errorf("unexpected context-drift advisory")
	}
}

// AC-L15 — offset shifts and an unrelated edit far from any hunk keep
// step 1 passing.
func TestACL15_OffsetAndFarEditsStayClean(t *testing.T) {
	for _, tc := range []struct {
		name string
		mut  func([]string)
	}{
		{"far-edit", func(l []string) { l[58] = "UNRELATED" }},
		{"outside-context", func(l []string) { l[14] = "UNRELATED" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newLadderFixture(t)
			mutateHead(t, f, tc.mut)
			r := f.Verify()
			if currentAnchor(t, r) != CurrentMaterializedClean {
				t.Errorf("current=%q want materialized-clean", currentAnchor(t, r))
			}
			if r.Verdict != "passed" {
				t.Errorf("verdict=%s", r.Verdict)
			}
		})
	}
}

// AC-L16 — unrelated edit 2 lines from a hunk ⇒ step 2 passes with zero
// `(0/0)` ⇒ PASS + context-drift advisory carrying R3.
func TestACL16_TwoLinesAwayIsContextDriftWarn(t *testing.T) {
	f := newLadderFixture(t)
	mutateHead(t, f, func(l []string) { l[11] = "UNRELATED" })
	r := f.Verify()
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%s want passed", r.Verdict)
	}
	if currentAnchor(t, r) != CurrentMaterializedContextDrif {
		t.Fatalf("current=%q want materialized-context-drift", currentAnchor(t, r))
	}
	a, ok := advisoryByCode(r, AdvisoryContextDrift)
	if !ok {
		t.Fatalf("no context-drift advisory; got %v", r.Advisories)
	}
	want := remediationR3(r.LandingEvidence.AttestationCommit, a.Path)
	if a.Message != want {
		t.Errorf("R3 not verbatim:\n got %q\nwant %q", a.Message, want)
	}
	if a.Severity != SeverityWarn {
		t.Errorf("advisory severity=%q want warn", a.Severity)
	}
}

// AC-L17 — unrelated edit 1 line from a hunk ⇒ `(0/0)` ⇒ BLOCK with R2.
// The accepted false red, pinned deliberately.
func TestACL17_OneLineAwayBlocksWithR2(t *testing.T) {
	f := newLadderFixture(t)
	mutateHead(t, f, func(l []string) { l[10] = "UNRELATED" })
	r := f.Verify()
	if r.Verdict != "failed" {
		t.Fatalf("verdict=%s want failed (accepted false red)", r.Verdict)
	}
	if r.FailedAt != FailedAtLandedContentAbsent {
		t.Errorf("failed_at=%q want landed-content-absent", r.FailedAt)
	}
	v8 := checkByID(t, r, CheckPostApplyPatchReplayClean)
	if !strings.Contains(v8.Remediation, "only with all context discarded at") {
		t.Errorf("expected R2; got %q", v8.Remediation)
	}
}

// AC-L18 — revert-in-place + identical postimage text elsewhere in the
// file ⇒ `(0/0)` ⇒ BLOCK. The rev-1 false-green regression guard.
func TestACL18_RevertInPlacePlusPastedTextBlocks(t *testing.T) {
	f := newLadderFixture(t)
	f.EditTracked(ladderBody(func(l []string) {
		// hunk 1 reverted in place, its postimage text pasted at EOF
		l[29] = "FEATURE30"
		l[49] = "FEATURE50"
	})+"FEATURE10\n", "revert in place, paste at EOF")
	r := f.Verify()
	if r.Verdict != "failed" {
		t.Fatalf("false green: verdict=%s current=%s", r.Verdict, currentAnchor(t, r))
	}
	if currentAnchor(t, r) != CurrentAbsent {
		t.Errorf("current=%q want absent", currentAnchor(t, r))
	}
}

// AC-L19/20/21/22 — partial reverts of hunk 1, hunk 2, hunk 3 and the
// non-adjacent pair 1+3 all BLOCK with R1.
func TestACL19_PartialRevertHunk1Blocks(t *testing.T) {
	assertPartialRevertBlocks(t, func(l []string) { l[9] = "l10" })
}

func TestACL20_PartialRevertHunk2Blocks(t *testing.T) {
	assertPartialRevertBlocks(t, func(l []string) { l[29] = "l30" })
}

func TestACL21_PartialRevertHunk3Blocks(t *testing.T) {
	assertPartialRevertBlocks(t, func(l []string) { l[49] = "l50" })
}

func TestACL22_PartialRevertHunks1And3Blocks(t *testing.T) {
	assertPartialRevertBlocks(t, func(l []string) { l[9] = "l10"; l[49] = "l50" })
}

func assertPartialRevertBlocks(t *testing.T, mut func([]string)) {
	t.Helper()
	f := newLadderFixture(t)
	mutateHead(t, f, mut)
	r := f.Verify()
	if r.Verdict != "failed" {
		t.Fatalf("verdict=%s want failed", r.Verdict)
	}
	v8 := checkByID(t, r, CheckPostApplyPatchReplayClean)
	if !strings.Contains(v8.Remediation, "postimage is not present at HEAD") {
		t.Errorf("expected R1; got %q", v8.Remediation)
	}
}

// AC-L23 — full revert ⇒ BLOCK.
func TestACL23_FullRevertBlocks(t *testing.T) {
	assertPartialRevertBlocks(t, func(l []string) { l[9] = "l10"; l[29] = "l30"; l[49] = "l50" })
}

// AC-L24 — a patched file DELETED at HEAD ⇒ BLOCK.
func TestACL24_DeletedFileBlocks(t *testing.T) {
	f := newLadderFixture(t)
	mustGit(t, f.Root, "rm", "-q", f.FilePath)
	mustGit(t, f.Root, "commit", "-q", "-m", "delete patched file")
	r := f.Verify()
	if r.Verdict != "failed" {
		t.Fatalf("verdict=%s want failed", r.Verdict)
	}
	if currentAnchor(t, r) != CurrentAbsent {
		t.Errorf("current=%q want absent", currentAnchor(t, r))
	}
}

// AC-L25 — degenerate whole-file hunk with header/footer added ⇒ step 2
// passes with zero `(0/0)` ⇒ PASS + context-drift.
func TestACL25_DegenerateWholeFileHunkIsContextDrift(t *testing.T) {
	f := newLadderFixture(t)
	// Re-capture the canonical patch with whole-file context so its
	// single hunk spans the entire file — the §3.6.5 "degenerate
	// whole-file hunk" row — then re-attest it.
	wide := mustGit(t, f.Root, "diff", "-U100", f.BaseCommit, "HEAD", "--", f.FilePath)
	f.WritePatch(wide + "\n")
	f.LandWithBlock(f.TrailerBlock(f.Slug))
	f.EditTracked("HEADER\n"+featureBody(nil)+"FOOTER\n", "wrap with header/footer")
	r := f.Verify()
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%s current=%s", r.Verdict, currentAnchor(t, r))
	}
	if currentAnchor(t, r) != CurrentMaterializedContextDrif {
		t.Errorf("current=%q want materialized-context-drift", currentAnchor(t, r))
	}
	if !hasAdvisory(r, AdvisoryContextDrift) {
		t.Errorf("expected context-drift advisory")
	}
}

// AC-L26 — the `-C0` step runs with LC_ALL=C.
func TestACL26_C0StepRunsUnderLCAllC(t *testing.T) {
	f := newLadderFixture(t)
	mutateHead(t, f, func(l []string) { l[11] = "UNRELATED" })
	w := installGitWrapper(t)
	w.Reset()
	f.Verify()
	found := false
	for _, c := range callsWithSubcommand(w.Calls(), "apply") {
		if !c.Has("-C0") {
			continue
		}
		found = true
		if c.Env["LC_ALL"] != "C" {
			t.Errorf("-C0 apply without LC_ALL=C: %s (env %v)", c.Joined(), c.Env)
		}
		if !c.Has("--verbose") {
			t.Errorf("-C0 apply without --verbose: %s", c.Joined())
		}
	}
	if !found {
		t.Fatalf("no -C0 ladder call observed")
	}
}

// AC-L27 — ladder results are memoised per (tree, patch, direction,
// context): probing the same tree twice issues ONE pair of calls.
func TestACL27_LadderIsMemoised(t *testing.T) {
	f := newLadderFixture(t)
	ctx := newVerifyRunContext(f.Store)
	patchPath := artifactPath(f.Root, f.Slug, "post-apply.patch")
	w := installGitWrapper(t)
	w.Reset()
	first := ctx.runLadder("HEAD", patchPath, f.PatchBytes())
	afterFirst := len(callsWithSubcommand(w.Calls(), "apply"))
	second := ctx.runLadder("HEAD", patchPath, f.PatchBytes())
	afterSecond := len(callsWithSubcommand(w.Calls(), "apply"))
	if first.Result != second.Result {
		t.Fatalf("memo returned a different result: %v vs %v", first, second)
	}
	if afterSecond != afterFirst {
		t.Errorf("second probe issued %d extra apply calls", afterSecond-afterFirst)
	}
}

// AC-L28 — R1 contains the literal `Do NOT run tpatch reconcile`, and no
// landed-mode remediation contains `reconcile` outside that negation.
func TestACL28_LandedRemediationsNeverRouteToReconcile(t *testing.T) {
	if !strings.Contains(remediationR1("abc"), "Do NOT run tpatch reconcile") {
		t.Fatalf("R1 lost its negation")
	}
	landed := []string{
		remediationR1("sha"), remediationR2("sha", "p"), remediationR3("sha", "p"),
		remediationR4(1, "sha", fmt.Errorf("x"), "slug"), remediationR5("sha"),
		remediationR6("s", "c", "p", "r", "b"), remediationR7("s", 2, []string{"a", "b"}),
		remediationR8("s", "c"), remediationR9("s", "c", 2), remediationR10("s", "err"),
		remediationR11("s"), remediationR12(1, "p", "e", "b", "o", "s"),
		remediationR13("later", "p", "s"), remediationR14("p", "c"),
		remediationR15("p", "stale", "t"), remediationR16("p", "t"),
		remediationR17("p", "t"), remediationR18("p"), remediationR19("s"),
		remediationR20("s", "p", "t"), remediationR21("s", "c"), remediationR22("s"),
		remediationR24(1, "p", "absent", "s"),
	}
	for _, msg := range landed {
		idx := strings.Index(msg, "reconcile")
		for idx >= 0 {
			window := msg[max0(idx-30):idx]
			if !strings.Contains(window, "Do NOT run tpatch") {
				t.Errorf("landed remediation routes to reconcile: %q", msg)
				break
			}
			next := strings.Index(msg[idx+1:], "reconcile")
			if next < 0 {
				break
			}
			idx = idx + 1 + next
		}
	}
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// ── local helpers ────────────────────────────────────────────────────────

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		if info.IsDir() {
			if rel == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		out[rel] = sha256Hex(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}

func diffTrees(before, after map[string]string) string {
	var diffs []string
	for k, v := range before {
		if av, ok := after[k]; !ok {
			diffs = append(diffs, "removed "+k)
		} else if av != v {
			diffs = append(diffs, "changed "+k)
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			diffs = append(diffs, "added "+k)
		}
	}
	return strings.Join(diffs, ", ")
}

func readIndexBytes(t *testing.T, root string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		return nil
	}
	return data
}

func listTempIndexes(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	for _, dir := range []string{
		filepath.Join(root, ".tpatch", "local", "verify-index"),
		filepath.Join(root, ".git"),
	} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "verify-index-") {
				out = append(out, filepath.Join(dir, e.Name()))
			}
		}
	}
	return out
}

// unrelatedPatch returns a syntactically valid patch that no tree in the
// fixture can accept, used to force `historical-anchor-unavailable`.
func unrelatedPatch() string {
	return "diff --git a/src/app.txt b/src/app.txt\n" +
		"--- a/src/app.txt\n" +
		"+++ b/src/app.txt\n" +
		"@@ -1,3 +1,3 @@\n" +
		"-NOT-PRESENT-ANYWHERE\n" +
		"+ALSO-NOT-PRESENT\n" +
		" trailing\n"
}

// relandWithCurrentArtifacts re-attests the CURRENT artifacts so evidence
// stays `exact` while the anchor search still fails.
func relandWithCurrentArtifacts(t *testing.T, f *landedFixture) {
	t.Helper()
	f.LandWithBlock(f.TrailerBlock(f.Slug))
}
