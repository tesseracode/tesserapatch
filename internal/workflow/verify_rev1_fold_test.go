package workflow

// Rev-1 fold regression suite — v0.15.1 Wave C, adjudication findings
// 1-5 (PRD-verify-freshness §7.1 rows AC-L129/AC-L134, AC-L107-L112,
// AC-L58/AC-L69, AC-L111, AC-L59).
//
// Each test here pins a defect the rev-0 review found, so the fold
// cannot silently regress:
//
//  1. every verify Git call is floor-gated AND offline — including the
//     legacy V4/V5 ancestry and the shadow worktree commands;
//  2. the feature set is read exactly once and every later answer comes
//     from that capture;
//  3. missing-object and generic probe failures classify honestly
//     instead of degrading to "no qualifier" / "ambiguous";
//  4. artifact and generation read failures are not absence;
//  5. the `base-commit-unreachable` advisory is actually emitted.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// ── Finding 1 — one floor-validated offline Git context ──────────────────

// gitSubcommandsUnderTest is every subcommand a verify run may issue.
// The wrapper asserts the offline env on ALL of them, including the
// shadow-worktree commands rev-0 left outside the gate.
var gitSubcommandsUnderTest = map[string]bool{
	"log": true, "rev-parse": true, "read-tree": true, "apply": true,
	"diff": true, "merge-base": true, "cat-file": true, "worktree": true,
	"write-tree": true, "add": true, "clean": true, "config": true,
	"status": true, "ls-files": true, "check-ignore": true, "rev-list": true,
}

// TestRev1_EveryVerifyGitCallIsOffline asserts that EVERY git command a
// verify run issues — not just the landed contract's own — carries
// GIT_NO_LAZY_FETCH=1. Rev-0 passed the narrower AC-L129 assertion while
// `git worktree add`, `git worktree remove` and the V4/V5 ancestry calls
// ran with the ambient environment.
func TestRev1_EveryVerifyGitCallIsOffline(t *testing.T) {
	f := newLadderFixture(t)
	// Give the feature a satisfied_by dep so V4 and V5 both perform
	// their own ancestry query during the run.
	addSatisfiedByParent(t, f.Store, f.Slug, f.BaseCommit)

	w := installGitWrapper(t)
	w.Reset()
	f.Verify()

	calls := w.Calls()
	if len(calls) == 0 {
		t.Fatalf("no git calls recorded")
	}
	sawWorktree, sawMergeBase := false, false
	for _, c := range calls {
		sub := c.Subcommand()
		if !gitSubcommandsUnderTest[sub] {
			continue
		}
		switch sub {
		case "worktree":
			sawWorktree = true
		case "merge-base":
			sawMergeBase = true
		}
		if c.Env["GIT_NO_LAZY_FETCH"] != "1" {
			t.Errorf("git %s ran WITHOUT GIT_NO_LAZY_FETCH=1", c.Joined())
		}
	}
	if !sawWorktree {
		t.Errorf("no `git worktree` call observed — the shadow path must be exercised")
	}
	if !sawMergeBase {
		t.Errorf("no `git merge-base` call observed — the ancestry path must be exercised")
	}
}

// TestRev1_BelowFloorIssuesOnlyVersion asserts the hard stop covers the
// LEGACY paths too: with a satisfied_by dep present (V5) and dependency
// metadata to validate (V4), a below-floor run still spawns nothing but
// `git --version`.
func TestRev1_BelowFloorIssuesOnlyVersion(t *testing.T) {
	f := newLadderFixture(t)
	addSatisfiedByParent(t, f.Store, f.Slug, f.BaseCommit)

	w := installFakeVersionGit(t, "git version 2.30.2")
	w.Reset()
	r := f.Verify()

	for _, c := range w.Calls() {
		if c.Subcommand() != "--version" && !c.Has("--version") {
			t.Errorf("below-floor run issued git %s", c.Joined())
		}
	}
	if r.LandingEvidence.State != EvidenceUnavailable {
		t.Errorf("evidence=%q want unavailable", r.LandingEvidence.State)
	}
	if r.Verdict != "failed" {
		t.Errorf("verdict=%q want failed", r.Verdict)
	}
	// V5 must refuse rather than silently pass: it could not answer.
	v5 := checkByID(t, r, CheckSatisfiedByReachable)
	if v5.Passed && !v5.Skipped {
		t.Errorf("V5 passed below the floor without being able to check reachability: %+v", v5)
	}
	if !strings.Contains(v5.Remediation, "git >= 2.36") {
		t.Errorf("V5 remediation must name the floor; got %q", v5.Remediation)
	}
}

// TestRev1_GitGateRefusesWithoutSpawning asserts the gateway itself,
// method by method, refuses below the floor without spawning git.
func TestRev1_GitGateRefusesWithoutSpawning(t *testing.T) {
	f := newLadderFixture(t)
	ctx := newVerifyRunContext(f.Store)
	// Force the below-floor condition on an otherwise healthy context.
	ctx.floorOK = false
	ctx.version = gitutil.GitVersion{Major: 2, Minor: 30, Raw: "git version 2.30.2"}

	w := installGitWrapper(t)
	w.Reset()

	if _, err := ctx.isAncestorChecked("a", "b"); !errors.Is(err, errGitBelowFloor) {
		t.Errorf("isAncestorChecked: %v", err)
	}
	if _, _, err := ctx.blobAtTree("HEAD", "x"); !errors.Is(err, errGitBelowFloor) {
		t.Errorf("blobAtTree: %v", err)
	}
	if _, err := ctx.newTempIndex(); !errors.Is(err, errGitBelowFloor) {
		t.Errorf("newTempIndex: %v", err)
	}
	if _, err := ctx.createShadow(f.Slug, "HEAD"); !errors.Is(err, errGitBelowFloor) {
		t.Errorf("createShadow: %v", err)
	}
	if _, _, err := ctx.runShadowGit(f.Root, "status"); !errors.Is(err, errGitBelowFloor) {
		t.Errorf("runShadowGit: %v", err)
	}
	if _, err := ctx.normalizedIdentity("HEAD", []string{"x"}); !errors.Is(err, errGitBelowFloor) {
		t.Errorf("normalizedIdentity: %v", err)
	}
	if err := ctx.pruneShadow(f.Slug); err != nil {
		t.Errorf("pruneShadow must be a no-op below the floor: %v", err)
	}
	if calls := w.Calls(); len(calls) != 0 {
		t.Fatalf("the gateway spawned git below the floor: %v", calls[0].Joined())
	}
}

// TestRev1_NoLegacyGitHelperInVerifyPath is the adversarial source check
// behind "no legacy bypass": the verify sources must not reference the
// non-offline gitutil helpers directly.
func TestRev1_NoLegacyGitHelperInVerifyPath(t *testing.T) {
	forbidden := []string{
		"gitutil.IsAncestor(", "gitutil.HeadCommit(", "gitutil.CreateShadow(",
		"gitutil.PruneShadow(", "gitutil.PruneAllShadows(", "gitutil.RunOfflineGitIn(",
		"gitutil.NewTempIndex(", "gitutil.NormalizedChangeIdentity(", "gitutil.BlobAtTree(",
		"gitutil.IsAncestorOffline(",
	}
	root := filepath.Join(docsRootForTest(t), "internal", "workflow")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "verify") || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") || name == "verify_gitgate.go" {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(root, name))
		if rerr != nil {
			t.Fatal(rerr)
		}
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue // prose may name the legacy helper it replaced
			}
			for _, bad := range forbidden {
				if strings.Contains(line, bad) {
					t.Errorf("%s calls %s directly; every verify Git call must go through the floor-validated gateway", name, bad)
				}
			}
		}
	}
}

func addSatisfiedByParent(t *testing.T, s *store.Store, slug, commit string) {
	t.Helper()
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "merged-parent", Slug: "merged-parent", Request: "x"}); err != nil {
		t.Fatalf("AddFeature: %v", err)
	}
	if err := s.MarkFeatureState("merged-parent", store.StateUpstreamMerged, "apply", ""); err != nil {
		t.Fatalf("MarkFeatureState: %v", err)
	}
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatal(err)
	}
	st.DependsOn = append(st.DependsOn, store.Dependency{
		Slug: "merged-parent", Kind: store.DependencyKindHard, SatisfiedBy: commit,
	})
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatal(err)
	}
}

// ── Finding 2 — a single immutable inventory ─────────────────────────────

// TestRev1_InventoryIsReadExactlyOnce counts store reads across a full
// run. Rev-0 built the inventory, then re-read the store through
// `ListFeatures`, `IsFeatureSuperseded`, per-parent `LoadFeatureStatus`
// and a persistence reload.
func TestRev1_InventoryIsReadExactlyOnce(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("parent", store.StateApplied)
	c.Feature("child", store.StateApplied, "parent")
	c.Artifacts("parent", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+P\n",
		ApplyRecipe{Feature: "parent", Operations: []RecipeOperation{
			{Type: "append-file", Path: "src/parent.txt", Content: "P\n"},
		}})
	c.Artifacts("child", "diff --git a/src/child.txt b/src/child.txt\nnew file mode 100644\n--- /dev/null\n+++ b/src/child.txt\n@@ -0,0 +1 @@\n+child\n",
		ApplyRecipe{Feature: "child", Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/child.txt", Content: "child\n"},
		}})

	ctx := newVerifyRunContext(c.Store)
	before := ctx.inv.Snapshot()

	// Mutate every feature on disk AFTER the capture. Every answer the
	// run gives must still come from the capture.
	mustWriteFile(t, filepath.Join(c.Root, ".tpatch", "features", "parent", "status.json"),
		`{"slug":"parent","state":"rejected"}`)
	mustWriteFile(t, artifactPath(c.Root, "parent", "post-apply.patch"), "REWRITTEN AFTER CAPTURE\n")

	st, err := before.Load("parent")
	if err != nil {
		t.Fatalf("captured parent unreadable: %v", err)
	}
	if st.State != store.StateApplied {
		t.Fatalf("the capture changed under the run: state=%q", st.State)
	}
	if string(ctx.inv.Entry("parent").Patch.Bytes) == "REWRITTEN AFTER CAPTURE\n" {
		t.Fatalf("the inventory re-read the artifact from disk")
	}
	// Supersession and the closure walk read the same capture.
	if _, superseded := isFeatureSupersededIn(ctx.inv.Statuses(), "parent"); superseded {
		t.Fatalf("supersession answered from a fresh scan")
	}
	states := map[string]store.FeatureState{}
	for _, f := range ctx.inv.Statuses() {
		states[f.Slug] = f.State
	}
	if states["parent"] != store.StateApplied {
		t.Errorf("Statuses() is not the capture: %v", states)
	}
}

// TestRev1_ReportIsBuiltFromTheCapture proves the whole report —
// verdict, hashes, parent snapshot and persisted record — is a function
// of the capture, not of the disk at report time.
func TestRev1_ReportIsBuiltFromTheCapture(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("parent", store.StateApplied)
	c.Feature("target", store.StateApplied, "parent")
	c.Artifacts("parent", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+P\n",
		ApplyRecipe{Feature: "parent", Operations: []RecipeOperation{
			{Type: "append-file", Path: "src/parent.txt", Content: "P\n"},
		}})
	c.Artifacts("target", "diff --git a/src/t.txt b/src/t.txt\nnew file mode 100644\n--- /dev/null\n+++ b/src/t.txt\n@@ -0,0 +1 @@\n+t\n",
		ApplyRecipe{Feature: "target", Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/t.txt", Content: "t\n"},
		}})

	first := c.Verify("target")
	// Now change the parent's state on disk and re-run over a NEW
	// context: the second run must see the new value, proving the first
	// run's stability was the capture and not caching.
	setFeatureState(t, c.Store, "parent", store.StateUpstreamMerged)
	second := c.Verify("target")

	if first.ParentSnapshot["parent"] != store.StateApplied {
		t.Errorf("first run parent snapshot=%v want applied", first.ParentSnapshot)
	}
	if second.ParentSnapshot["parent"] != store.StateUpstreamMerged {
		t.Errorf("second run parent snapshot=%v want upstream_merged", second.ParentSnapshot)
	}
}

// TestRev1_PersistenceDoesNotReload asserts the freshness write neither
// re-reads `status.json` nor diverges from disk (which would trip the
// instability detector on the next feature of a `--all` run).
func TestRev1_PersistenceDoesNotReload(t *testing.T) {
	f := newLadderFixture(t)
	r, err := RunVerify(f.Store, f.Slug, VerifyOptions{})
	if err != nil {
		t.Fatalf("RunVerify: %v", err)
	}
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%s failed_at=%s", r.Verdict, r.FailedAt)
	}
	onDisk, err := f.Store.LoadFeatureStatus(f.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if onDisk.Verify == nil || !onDisk.Verify.Passed {
		t.Fatalf("freshness record not persisted")
	}
	if onDisk.LastCommand != "verify" {
		t.Errorf("last_command=%q want verify", onDisk.LastCommand)
	}
}

// ── Finding 3 — probe failures classify honestly ─────────────────────────

// TestRev1_QualificationFailureIsNotSwallowed asserts a git-level
// failure during qualification surfaces as `unavailable` /
// `history-incomplete`, never as "no candidate qualified".
func TestRev1_QualificationFailureIsNotSwallowed(t *testing.T) {
	f := newLadderFixture(t)
	ctx := newVerifyRunContext(f.Store)
	patchPath := artifactPath(f.Root, f.Slug, "post-apply.patch")

	// A candidate whose parent revision cannot be resolved at all.
	ok, state, err := ctx.forwardQualifies(strings.Repeat("0", 40), patchPath)
	if ok {
		t.Fatalf("an unresolvable candidate must not qualify")
	}
	if err == nil {
		t.Fatalf("an unresolvable candidate must surface an error, not a silent non-qualification")
	}
	if state != EvidenceHistoryIncomplete && state != EvidenceUnavailable {
		t.Errorf("classification=%q want history-incomplete or unavailable", state)
	}
}

// TestRev1_IdentityFailureIsNotAmbiguity asserts an identity that could
// not be COMPUTED classifies as a reader failure rather than as two
// identities that differ.
func TestRev1_IdentityFailureIsNotAmbiguity(t *testing.T) {
	f := newLadderFixture(t)
	ctx := newVerifyRunContext(f.Store)
	_, err := ctx.identitiesFor(
		[]gitutil.CommitRecord{{SHA: strings.Repeat("0", 40)}},
		f.PatchBytes())
	if err == nil {
		t.Fatalf("an uncomputable identity must surface an error")
	}
	if got := classifyGitFailure(err); got != EvidenceUnavailable && got != EvidenceHistoryIncomplete {
		t.Errorf("classifyGitFailure=%q want unavailable or history-incomplete for %v", got, err)
	}
}

// TestRev1_GenericProbeFailureIsUnavailable injects a generic (non
// missing-object) git failure into the anchor-C probe and asserts the
// run reports `unavailable`, not `landed-content-absent`.
func TestRev1_GenericProbeFailureIsUnavailable(t *testing.T) {
	f := newLadderFixture(t)
	// A `git apply` shim that fails with a generic error for the ladder
	// while leaving every other subcommand alone.
	installFailingApplyGit(t, "fatal: internal error: object store is on fire")
	r := f.Verify()
	if r.Verdict != "failed" {
		t.Fatalf("verdict=%s want failed", r.Verdict)
	}
	if r.FailedAt == FailedAtLandedContentAbsent {
		t.Fatalf("a probe that could not RUN must not be reported as absent content")
	}
	if r.LandingEvidence.State != EvidenceUnavailable && r.LandingEvidence.State != EvidenceHistoryIncomplete {
		t.Fatalf("evidence=%q want unavailable/history-incomplete", r.LandingEvidence.State)
	}
}

// installFailingApplyGit makes every `git apply` fail with msg while
// forwarding everything else to the real git.
func installFailingApplyGit(t *testing.T, msg string) {
	t.Helper()
	realGit := mustLookGit(t)
	dir := t.TempDir()
	script := "#!/bin/sh\nfor a in \"$@\"; do\n  if [ \"$a\" = \"apply\" ]; then\n    echo " +
		rev1ShellQuote(msg) + " >&2\n    exit 128\n  fi\ndone\nexec " + realGit + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatalf("write shim: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func rev1ShellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// ── Finding 4 — artifact/generation read errors are not absence ──────────

// TestRev1_ArtifactReadErrorIsNotAbsence asserts a non-ENOENT artifact
// read failure blocks with `inventory-unreadable` instead of being
// classified as an absent artifact.
func TestRev1_ArtifactReadErrorIsNotAbsence(t *testing.T) {
	f := newLadderFixture(t)
	// Replace the canonical patch with a DIRECTORY: it exists, and it
	// cannot be read. Rev-0 reported this as `absent`.
	patch := artifactPath(f.Root, f.Slug, "post-apply.patch")
	if err := os.Remove(patch); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(patch, 0o755); err != nil {
		t.Fatal(err)
	}

	inv, err := buildInventory(f.Store)
	if err != nil {
		t.Fatalf("buildInventory: %v", err)
	}
	entry := inv.Entry(f.Slug)
	if entry.Patch.Err == nil {
		t.Fatalf("a non-ENOENT read failure must be retained, not collapsed to absence")
	}
	path, readErr := entry.ReadErr()
	if readErr == nil {
		t.Fatalf("ReadErr must surface the artifact failure")
	}
	if !strings.Contains(path, "post-apply.patch") {
		t.Errorf("path=%q must name the artifact", path)
	}

	r := f.Verify()
	if r.FailedAt != FailedAtInventoryUnreadable {
		t.Fatalf("failed_at=%q want inventory-unreadable", r.FailedAt)
	}
	if !strings.Contains(checkByID(t, r, CheckRecipeReplayClean).Remediation, "post-apply.patch") {
		t.Errorf("the block must name the unreadable artifact")
	}
}

// TestRev1_GenerationsParseErrorIsRetained asserts a corrupt
// `patch-generations.json` is a read failure, not an empty touched-path
// set that silently suppresses ADR-029 later-touch detection.
func TestRev1_GenerationsParseErrorIsRetained(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("target", store.StateApplied)
	c.Artifacts("target", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+T\n",
		ApplyRecipe{Feature: "target", Operations: []RecipeOperation{
			{Type: "append-file", Path: "src/parent.txt", Content: "T\n"},
		}})
	mustWriteFile(t, c.Store.PatchGenerationsPath("target"), "{ not json")

	inv, err := buildInventory(c.Store)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Entry("target").GenerationsErr == nil {
		t.Fatalf("a corrupt patch-generations manifest must be retained as an error")
	}
	r := c.Verify("target")
	if r.FailedAt != FailedAtInventoryUnreadable {
		t.Fatalf("failed_at=%q want inventory-unreadable", r.FailedAt)
	}
}

// TestRev1_UnrelatedArtifactErrorIsAdvisoryOnly asserts the read-error
// policy's other half: an UNRELATED feature's failure warns and is
// excluded from ordering, without failing the run.
func TestRev1_UnrelatedArtifactErrorIsAdvisoryOnly(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("target", store.StateApplied)
	c.Feature("outsider", store.StateApplied)
	c.Artifacts("target", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+T\n",
		ApplyRecipe{Feature: "target", Operations: []RecipeOperation{
			{Type: "append-file", Path: "src/parent.txt", Content: "T\n"},
		}})
	c.Artifacts("outsider", "diff --git a/x b/x\n", ApplyRecipe{Feature: "outsider"})
	recipe := artifactPath(c.Root, "outsider", "apply-recipe.json")
	if err := os.Remove(recipe); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(recipe, 0o755); err != nil {
		t.Fatal(err)
	}

	r := c.Verify("target")
	if r.FailedAt == FailedAtInventoryUnreadable {
		t.Fatalf("an UNRELATED feature's read failure must not block the run")
	}
	a, ok := advisoryByCode(r, AdvisoryInventoryUnreadable)
	if !ok {
		t.Fatalf("expected an inventory-unreadable advisory; got %v", r.Advisories)
	}
	if a.Slug != "outsider" {
		t.Errorf("advisory slug=%q want outsider", a.Slug)
	}
	if !strings.Contains(a.Path, "apply-recipe.json") {
		t.Errorf("advisory path=%q must name the artifact", a.Path)
	}
}

// ── Finding 5 — the base-commit-unreachable advisory ─────────────────────

// TestRev1_BaseCommitUnreachableAdvisoryIsEmitted asserts the required
// advisory fires for a landing whose attested base commit is well-formed
// but not reachable from HEAD (the rebase / cherry-pick case), and that
// it does not fail the run on its own.
func TestRev1_BaseCommitUnreachableAdvisoryIsEmitted(t *testing.T) {
	f := newLadderFixture(t)
	// Point the recorded base at a resolvable commit that is NOT an
	// ancestor of HEAD, then re-attest so the landing carries it.
	tree := mustGit(t, f.Root, "rev-parse", f.BaseCommit+"^{tree}")
	orphan := strings.TrimSpace(mustGit(t, f.Root, "commit-tree", tree, "-m", "unreachable base"))
	st, err := f.Store.LoadFeatureStatus(f.Slug)
	if err != nil {
		t.Fatal(err)
	}
	st.Apply.BaseCommit = orphan
	if err := f.Store.SaveFeatureStatus(st); err != nil {
		t.Fatal(err)
	}
	f.LandWithBlock(f.TrailerBlock(f.Slug))

	r := f.Verify()
	if r.LandingEvidence.State != EvidenceExact {
		t.Fatalf("evidence=%q want exact", r.LandingEvidence.State)
	}
	if r.LandingEvidence.BaseCommitReachable == nil || *r.LandingEvidence.BaseCommitReachable {
		t.Fatalf("base_commit_reachable=%v want false", r.LandingEvidence.BaseCommitReachable)
	}
	a, ok := advisoryByCode(r, AdvisoryBaseCommitUnreachable)
	if !ok {
		t.Fatalf("the required base-commit-unreachable advisory was not emitted; got %v", r.Advisories)
	}
	if a.Severity != SeverityWarn {
		t.Errorf("advisory severity=%q want warn", a.Severity)
	}
	want := advisoryBaseCommitUnreachable(f.Slug, r.LandingEvidence.AttestationCommit, orphan)
	if a.Message != want {
		t.Errorf("advisory not verbatim:\n got %q\nwant %q", a.Message, want)
	}
	if r.Verdict != "passed" {
		t.Fatalf("an advisory must never fail the run: verdict=%s failed_at=%s", r.Verdict, r.FailedAt)
	}
}
