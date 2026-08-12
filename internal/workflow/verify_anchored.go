package workflow

// Anchored dynamic phase — v0.15.1 Wave C / GH #8.
//
// V7 / V8 / V10 under the ADR-013 Amendment 1 dual-anchor model:
//
//   - Anchor H (historical): a shadow rooted at the replay anchor's
//     single parent tree. Closure arbitration runs there, then V7
//     replays the target recipe, the shadow is RESET to
//     `closureBaselineTree` (the GH #2 / v0.11.3 invariant), V8 runs the
//     forward `git apply --check`, and V10 evaluates preimages at each
//     member's OWN baseline.
//   - Anchor C (current): an index-isolated hardened ladder at HEAD. It
//     mutates nothing — no shadow, no worktree read, no real-index read.
//
// Forward mode (evidence `none`) keeps today's semantics for V7/V8
// byte-for-byte; only V10's reference tree moves (Q15).

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// dynamicPhase is the V7/V8/V10 triple plus the run-level fields the
// report needs.
type dynamicPhase struct {
	v7          store.VerifyCheckResult
	v8          store.VerifyCheckResult
	v10         store.VerifyCheckResult
	failedAt    string
	parentSlug  string
	advisories  []VerifyAdvisory
	baseline    VerifyBaseline
	memberNotes map[string]string
}

// anchoredInput carries everything the dynamic phase needs. Nothing here
// is re-read from disk: artifact bytes come from the immutable
// inventory.
type anchoredInput struct {
	ctx           *verifyRunContext
	store         *store.Store
	slug          string
	status        store.FeatureStatus
	recipe        ApplyRecipe
	recipePresent bool
	entry         *inventoryEntry
	evidence      *landingEvidenceResult
	staticFailed  bool
}

// runDynamicPhase dispatches V7/V8/V10 by target mode.
func runDynamicPhase(in anchoredInput) dynamicPhase {
	ctx := in.ctx
	landed := in.evidence.Landed()
	terminal := in.evidence.Terminal()
	artifactsAbsent := in.evidence.ArtifactsAbsent

	baseline := VerifyBaseline{
		Mode:          BaselineModeHead,
		CurrentCommit: ctx.head,
	}
	if landed || terminal || artifactsAbsent {
		baseline.Mode = BaselineModeDual
		baseline.CurrentProbe = CurrentProbeIsolated
	}

	v7Mode, v8Mode, v10Mode := ModeForward, ModeForward, ModeProvenanceAnchor
	if landed || terminal || artifactsAbsent {
		v7Mode, v8Mode, v10Mode = ModeHistoricalAnchor, ModeDualAnchor, ModeHistoricalAnchor
	}

	// The TARGET's own inventory read failure outranks every other
	// outcome: a feature whose artifacts could not be read cannot be
	// classified, replayed or asserted (D17, rev-1 finding 4).
	if path, readErr := in.entry.ReadErr(); readErr != nil {
		rem := fmt.Sprintf("verify aborted: %s/%s could not be read from the feature inventory: %v; repair the artifact before verifying %s",
			in.slug, path, readErr, in.slug)
		return dynamicPhase{
			v7:       withMode(store.VerifyCheckResult{ID: CheckRecipeReplayClean, Severity: SeverityBlock, Passed: false, Remediation: rem}, v7Mode),
			v8:       withMode(store.VerifyCheckResult{ID: CheckPostApplyPatchReplayClean, Severity: SeverityBlock, Passed: false, Remediation: rem}, v8Mode),
			v10:      withMode(store.VerifyCheckResult{ID: CheckWriteFilePreimageFresh, Severity: SeverityBlock, Passed: false, Remediation: rem}, v10Mode),
			failedAt: FailedAtInventoryUnreadable,
			baseline: baseline,
		}
	}

	// The two CLASSIFICATION terminals are decided before the static
	// short-circuit: they are properties of the evidence and the captured
	// artifacts, not of the dynamic phase, and neither allocates a shadow.
	if terminal {
		return terminalEvidencePhase(in, baseline, v7Mode, v8Mode, v10Mode)
	}
	if artifactsAbsent {
		return artifactsAbsentPhase(in, baseline, v7Mode, v8Mode, v10Mode)
	}

	// Static block-severity failure upstream: V7/V8 skip exactly as
	// today, and V10 keeps its own skip/execute contract. `mode` is
	// still present on all three rows (AC-L116).
	if in.staticFailed {
		out := dynamicPhase{
			v7: withMode(store.VerifyCheckResult{ID: CheckRecipeReplayClean, Severity: SeverityBlock, Passed: true, Skipped: true,
				Reason: "skipped: an earlier block-severity static check failed"}, v7Mode),
			v8: withMode(store.VerifyCheckResult{ID: CheckPostApplyPatchReplayClean, Severity: SeverityBlock, Passed: true, Skipped: true,
				Reason: "skipped: V7 (recipe_replay_clean) skipped"}, v8Mode),
			baseline: baseline,
		}
		v10 := evaluateV10(in, nil, "", nil)
		out.v10 = withMode(v10.result, v10Mode)
		out.advisories = append(out.advisories, v10.advisories...)
		if out.failedAt == "" {
			out.failedAt = v10.failedAt
		}
		return out
	}

	if landed {
		return landedPhase(in, baseline)
	}
	return forwardPhase(in, baseline)
}

func withMode(c store.VerifyCheckResult, mode string) store.VerifyCheckResult {
	c.Mode = mode
	return c
}

// ── Terminal families ────────────────────────────────────────────────────

// terminalEvidencePhase handles the eight terminal non-`exact` states.
// V7, V8 and V10 all report `passed: false` with `mode` present —
// failed-because-unanchored, never skipped.
func terminalEvidencePhase(in anchoredInput, baseline VerifyBaseline, v7Mode, v8Mode, v10Mode string) dynamicPhase {
	ev := in.evidence.Evidence
	attested := recordBySHA(in.ctx.commits, ev.AttestationCommit)
	dupes := attestationSHAs(in.ctx, in.slug)
	rem := evidenceRemediation(in.slug, ev, in.entry, attested, dupes)

	baseline.HistoricalAnchor = &VerifyHistoricalAnchor{
		State:  AnchorStateUnavailable,
		Reason: fmt.Sprintf("landing evidence is %s", ev.State),
	}
	return dynamicPhase{
		v7: withMode(store.VerifyCheckResult{ID: CheckRecipeReplayClean, Severity: SeverityBlock, Passed: false, Remediation: rem}, v7Mode),
		v8: withMode(store.VerifyCheckResult{
			ID: CheckPostApplyPatchReplayClean, Severity: SeverityBlock, Passed: false,
			AnchorResults: map[string]string{"historical": "skipped", "current": CurrentSkipped},
			Remediation:   fmt.Sprintf("landing evidence for %s is %s: the historical half of this check could not run; see recipe_replay_clean", in.slug, ev.State),
		}, v8Mode),
		v10: withMode(store.VerifyCheckResult{
			ID: CheckWriteFilePreimageFresh, Severity: SeverityBlock, Passed: false,
			Remediation: fmt.Sprintf("landing evidence for %s is %s: preimage freshness cannot be evaluated without a landing baseline; see recipe_replay_clean", in.slug, ev.State),
		}, v10Mode),
		failedAt: FailedAtLandingEvidence,
		baseline: baseline,
	}
}

// artifactsAbsentPhase handles the D10 presence short-circuit.
func artifactsAbsentPhase(in anchoredInput, baseline VerifyBaseline, v7Mode, v8Mode, v10Mode string) dynamicPhase {
	rem := remediationR19(in.slug)
	baseline.HistoricalAnchor = &VerifyHistoricalAnchor{
		State:  AnchorStateUnavailable,
		Reason: "the canonical patch is absent or empty, so no landing baseline can be derived",
	}
	return dynamicPhase{
		v7: withMode(store.VerifyCheckResult{ID: CheckRecipeReplayClean, Severity: SeverityBlock, Passed: false, Remediation: rem}, v7Mode),
		v8: withMode(store.VerifyCheckResult{
			ID: CheckPostApplyPatchReplayClean, Severity: SeverityBlock, Passed: false,
			AnchorResults: map[string]string{"historical": "skipped", "current": CurrentSkipped},
			Remediation:   rem,
		}, v8Mode),
		v10:      withMode(store.VerifyCheckResult{ID: CheckWriteFilePreimageFresh, Severity: SeverityBlock, Passed: false, Remediation: rem}, v10Mode),
		failedAt: FailedAtLandedArtifacts,
		baseline: baseline,
	}
}

func attestationSHAs(ctx *verifyRunContext, slug string) []string {
	var out []string
	for _, rec := range ctx.commits {
		vals := rec.TrailerValues(gitutil.TrailerFeature)
		if len(vals) == 1 && vals[0] == slug {
			out = append(out, rec.SHA)
		}
	}
	return out
}

// ── Landed mode ──────────────────────────────────────────────────────────

func landedPhase(in anchoredInput, baseline VerifyBaseline) dynamicPhase {
	ctx := in.ctx
	anchor := ctx.resolveAnchor(in.slug)
	patchPath := artifactPath(ctx.root, in.slug, "post-apply.patch")

	// Anchor C runs regardless of the historical half — the report says
	// what the tree looks like even when the run fails for other reasons.
	ladder := ctx.runLadder("HEAD", patchPath, in.entry.Patch.Bytes)

	// D16: an object that is missing LOCALLY is `history-incomplete`,
	// never "the content is absent". The distinction is only detectable
	// because every command runs under GIT_NO_LAZY_FETCH=1 (E47).
	if ladder.MissingObject {
		in.evidence.Evidence.State = EvidenceHistoryIncomplete
		in.evidence.Evidence.Reason = "an object required to read the landing baseline is missing from this partial clone"
		return terminalEvidencePhase(in, baseline, ModeHistoricalAnchor, ModeDualAnchor, ModeHistoricalAnchor)
	}
	// A probe that could not RUN is `unavailable` — the reader failed,
	// which is not the same claim as "the content is absent"
	// (rev-1 adjudication finding 3).
	if ladder.Err != nil {
		in.evidence.Evidence.State = EvidenceUnavailable
		in.evidence.Evidence.Reason = fmt.Sprintf("the current-anchor probe could not run: %v", ladder.Err)
		return terminalEvidencePhase(in, baseline, ModeHistoricalAnchor, ModeDualAnchor, ModeHistoricalAnchor)
	}

	// A qualification or identity probe that FAILED classifies on its own
	// terms rather than degrading to "no usable landing baseline".
	if anchor.FailState != "" {
		in.evidence.Evidence.State = anchor.FailState
		in.evidence.Evidence.Reason = anchor.FailDetail
		return terminalEvidencePhase(in, baseline, ModeHistoricalAnchor, ModeDualAnchor, ModeHistoricalAnchor)
	}

	if !anchor.Available {
		baseline.HistoricalAnchor = &VerifyHistoricalAnchor{
			State:               AnchorStateUnavailable,
			CandidatesCollected: anchor.Collected,
			CandidatesQualified: anchor.Qualified,
			Reason:              anchor.Reason,
		}
		rem := remediationR11(in.slug)
		return dynamicPhase{
			v7: withMode(store.VerifyCheckResult{ID: CheckRecipeReplayClean, Severity: SeverityBlock, Passed: false, Remediation: rem}, ModeHistoricalAnchor),
			v8: withMode(store.VerifyCheckResult{
				ID: CheckPostApplyPatchReplayClean, Severity: SeverityBlock, Passed: false,
				AnchorResults: map[string]string{"historical": "failed", "current": ladder.Result},
				Remediation:   fmt.Sprintf("landed feature %s has no usable landing baseline: the historical half of this check could not run; see recipe_replay_clean", in.slug),
			}, ModeDualAnchor),
			v10: withMode(store.VerifyCheckResult{
				ID: CheckWriteFilePreimageFresh, Severity: SeverityBlock, Passed: false,
				Remediation: fmt.Sprintf("landed feature %s has no usable landing baseline: preimage freshness cannot be evaluated without one; see recipe_replay_clean", in.slug),
			}, ModeHistoricalAnchor),
			failedAt: FailedAtHistoricalAnchor,
			baseline: baseline,
		}
	}

	baseline.HistoricalAnchor = &VerifyHistoricalAnchor{
		State:               AnchorStateAvailable,
		Commit:              anchor.Anchor,
		ReplayAnchorCommit:  anchor.ReplayFrom,
		CandidatesCollected: anchor.Collected,
		CandidatesQualified: anchor.Qualified,
	}

	out := runAnchoredClosure(in, anchor.Anchor, baseline)
	out.baseline = baseline

	// Fold the anchor-C half into V8.
	foldAnchorC(&out, in, ladder)
	return out
}

// foldAnchorC merges the anchor-C ladder outcome into the V8 row and the
// run's advisories (D12 / R1 / R2 / R3).
func foldAnchorC(out *dynamicPhase, in anchoredInput, ladder ladderOutcome) {
	attestation := in.evidence.Evidence.AttestationCommit
	if out.v8.AnchorResults == nil {
		out.v8.AnchorResults = map[string]string{}
	}
	historical := "passed"
	if out.v8.Skipped {
		historical = "skipped"
	} else if !out.v8.Passed {
		historical = "failed"
	}
	out.v8.AnchorResults["historical"] = historical
	out.v8.AnchorResults["current"] = ladder.Result

	switch {
	case ladder.Err != nil:
		// Defensive: landedPhase already routes a failed probe to
		// `unavailable` before this fold runs.
		out.v8.Passed = false
		out.v8.Skipped = false
		out.v8.Remediation = appendRemediation(out.v8.Remediation,
			fmt.Sprintf("landed feature: the current-anchor probe could not run: %v", ladder.Err))
		if out.failedAt == "" {
			out.failedAt = FailedAtLandingEvidence
		}
	case ladder.Blocked:
		out.v8.Passed = false
		out.v8.Skipped = false
		path := ladder.Path
		if path == "" && in.recipePresent {
			// Diagnostics only (D13): localise which op and path the
			// failure concerns. This never decides presence.
			path = diagnosePathForBlockedLadder(in.ctx, "HEAD", in.recipe)
		}
		rem := remediationR1(attestation)
		if ladder.ZeroContextHunks > 0 {
			rem = remediationR2(attestation, path)
		}
		out.v8.Remediation = appendRemediation(out.v8.Remediation, rem)
		if out.failedAt == "" {
			out.failedAt = FailedAtLandedContentAbsent
		}
	case ladder.ContextDrift:
		out.advisories = append(out.advisories, warnAdvisory(
			AdvisoryContextDrift, in.slug, ladder.Path,
			remediationR3(attestation, ladder.Path)))
	}
}

func appendRemediation(existing, add string) string {
	if existing == "" {
		return add
	}
	return existing + "; " + add
}

// ── Forward mode ─────────────────────────────────────────────────────────

func forwardPhase(in anchoredInput, baseline VerifyBaseline) dynamicPhase {
	out := runAnchoredClosure(in, "", baseline)
	out.baseline = baseline
	return out
}

// ── Shared anchored closure ──────────────────────────────────────────────

// runAnchoredClosure is the generalisation of the shipped closure
// replay. anchorCommit == "" means forward mode (shadow at HEAD).
func runAnchoredClosure(in anchoredInput, anchorCommit string, baseline VerifyBaseline) dynamicPhase {
	ctx := in.ctx
	s := in.store
	slug := in.slug
	landed := anchorCommit != ""

	// Presence for V7/V8 preconditions is "the file exists", byte-for-byte
	// as shipped — a zero-byte patch is PRESENT and V8 fails on it. The
	// D10 three-state presence model governs landed CLASSIFICATION, and a
	// landed target with a non-nonempty patch has already short-circuited
	// to `landed-artifacts-absent` before reaching here.
	patchPresent := in.entry != nil && in.entry.Patch.Presence != PresenceAbsent
	recipePresent := in.recipePresent

	v7Mode, v8Mode, v10Mode := ModeForward, ModeForward, ModeProvenanceAnchor
	if landed {
		v7Mode, v8Mode, v10Mode = ModeHistoricalAnchor, ModeDualAnchor, ModeHistoricalAnchor
	}

	// Both artifacts absent — skip V7/V8 and allocate no shadow.
	if !recipePresent && !patchPresent {
		out := dynamicPhase{
			v7: withMode(store.VerifyCheckResult{ID: CheckRecipeReplayClean, Severity: SeverityBlock, Passed: true, Skipped: true,
				Reason: "no apply-recipe.json (precondition not met)"}, v7Mode),
			v8: withMode(store.VerifyCheckResult{ID: CheckPostApplyPatchReplayClean, Severity: SeverityBlock, Passed: true, Skipped: true,
				Reason: "no post-apply.patch (precondition not met)"}, v8Mode),
		}
		v10 := evaluateV10(in, nil, "", nil)
		out.v10 = withMode(v10.result, v10Mode)
		out.advisories = append(out.advisories, v10.advisories...)
		out.failedAt = v10.failedAt
		return out
	}

	// 1. Hard-parent closure (unchanged BFS + supersession filtering).
	//    The feature set comes from the ONE immutable inventory, never a
	//    second `ListFeatures()` scan (rev-1 adjudication finding 2).
	allFeatures := ctx.inv.Statuses()
	closure := map[string][]store.Dependency{}
	closure[slug] = filterHardDeps(in.status.DependsOn)
	queue := append([]string(nil), depSlugsHard(in.status.DependsOn)...)
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if _, seen := closure[curr]; seen {
			continue
		}
		if _, superseded := isFeatureSupersededIn(allFeatures, curr); superseded {
			continue
		}
		entry := ctx.inv.Entry(curr)
		if entry == nil || entry.Err != nil || entry.Status == nil {
			closure[curr] = nil
			continue
		}
		hd := filterHardDeps(entry.Status.DependsOn)
		closure[curr] = hd
		for _, d := range hd {
			queue = append(queue, d.Slug)
		}
	}

	order, err := store.TopologicalOrder(closure)
	if err != nil {
		return dynamicPhase{
			v7: withMode(store.VerifyCheckResult{
				ID: CheckRecipeReplayClean, Severity: SeverityBlock, Passed: false,
				Remediation: fmt.Sprintf("hard-parent closure topology failed: %v; investigate or re-run tpatch implement %s", err, slug),
			}, v7Mode),
			v8:  withMode(skipV8Because("V7 (recipe_replay_clean) failed: topology"), v8Mode),
			v10: withMode(skipV10Because("skipped: V7 (recipe_replay_clean) failed: topology"), v10Mode),
		}
	}

	// Inventory read-error policy (D17): the target or any closure member
	// that could not be read is a block. "Could not be read" covers the
	// status AND every artifact/metadata read failure that is not plain
	// absence (rev-1 adjudication finding 4).
	for _, member := range order {
		e := ctx.inv.Entry(member)
		if e == nil {
			continue
		}
		if path, readErr := e.ReadErr(); readErr != nil {
			rem := fmt.Sprintf("verify aborted: %s/%s could not be read from the feature inventory: %v; repair or remove the feature before verifying %s", member, path, readErr, slug)
			return dynamicPhase{
				v7:       withMode(store.VerifyCheckResult{ID: CheckRecipeReplayClean, Severity: SeverityBlock, Passed: false, Remediation: rem}, v7Mode),
				v8:       withMode(store.VerifyCheckResult{ID: CheckPostApplyPatchReplayClean, Severity: SeverityBlock, Passed: false, Remediation: rem}, v8Mode),
				v10:      withMode(store.VerifyCheckResult{ID: CheckWriteFilePreimageFresh, Severity: SeverityBlock, Passed: false, Remediation: rem}, v10Mode),
				failedAt: FailedAtInventoryUnreadable,
			}
		}
	}

	// 2. Shadow allocation — at the anchor commit in landed mode, at
	//    HEAD in forward mode.
	shadowRoot := anchorCommit
	if shadowRoot == "" {
		head, headErr := ctx.head, ctx.headErr
		if headErr != nil {
			return dynamicPhase{
				v7: withMode(store.VerifyCheckResult{
					ID: CheckRecipeReplayClean, Severity: SeverityBlock, Passed: false,
					Remediation: fmt.Sprintf("cannot resolve HEAD for shadow allocation: %v", headErr),
				}, v7Mode),
				v8:  withMode(skipV8Because("V7 (recipe_replay_clean) failed: HEAD unresolved"), v8Mode),
				v10: withMode(skipV10Because("skipped: V7 (recipe_replay_clean) failed: HEAD unresolved"), v10Mode),
			}
		}
		shadowRoot = head
	}
	shadowPath, err := ctx.createShadow(slug, shadowRoot)
	if err != nil {
		return dynamicPhase{
			v7: withMode(store.VerifyCheckResult{
				ID: CheckRecipeReplayClean, Severity: SeverityBlock, Passed: false,
				Remediation: fmt.Sprintf("cannot allocate shadow worktree: %v", err),
			}, v7Mode),
			v8:  withMode(skipV8Because("V7 (recipe_replay_clean) failed: shadow allocation"), v8Mode),
			v10: withMode(skipV10Because("skipped: V7 (recipe_replay_clean) failed: shadow allocation"), v10Mode),
		}
	}
	defer func() { _ = ctx.pruneShadow(slug) }()

	// 3. Closure arbitration (D13).
	arb := arbitrateClosure(in, order, shadowRoot, shadowPath, v7Mode, v8Mode, v10Mode)
	if arb != nil {
		return *arb
	}
	advisories := in.ctx.pendingAdvisories
	in.ctx.pendingAdvisories = nil
	memberBaselines := in.ctx.pendingMemberBaselines
	in.ctx.pendingMemberBaselines = nil

	closureBaselineTree, err := snapshotShadowTree(ctx, shadowPath)
	if err != nil {
		return dynamicPhase{
			v7: withMode(store.VerifyCheckResult{
				ID: CheckRecipeReplayClean, Severity: SeverityBlock, Passed: false,
				Remediation: fmt.Sprintf("cannot snapshot closure-replayed baseline: %v", err),
			}, v7Mode),
			v8:         withMode(skipV8Because("V7 (recipe_replay_clean) failed: closure baseline snapshot"), v8Mode),
			v10:        withMode(skipV10Because("skipped: V7 (recipe_replay_clean) failed: closure baseline snapshot"), v10Mode),
			advisories: advisories,
		}
	}

	out := dynamicPhase{advisories: advisories}

	// 4. V10 FIRST for the target when landed: it is evaluated at the
	//    closure baseline, before the target's own recipe replays.
	v10ReferenceTree := ""
	if landed {
		// Landed: the target's own V10 baseline is the anchor-H closure
		// baseline — the shadow AFTER arbitration and BEFORE the target's
		// recipe replays (D15).
		v10ReferenceTree = closureBaselineTree
	}
	v10 := evaluateV10(in, memberBaselines, v10ReferenceTree, ctxAnchorCommit(anchorCommit))
	out.v10 = withMode(v10.result, v10Mode)
	out.advisories = append(out.advisories, v10.advisories...)
	if v10.failedAt != "" {
		out.failedAt = v10.failedAt
	}

	// 5. V7 — replay the target's recipe in the shadow.
	var v7 store.VerifyCheckResult
	switch {
	case recipePresent && landed && len(in.recipe.Operations) == 0:
		// A parsing recipe with zero operations cannot attest anything;
		// the canonical patch is the sole authority (D10).
		v7 = store.VerifyCheckResult{ID: CheckRecipeReplayClean, Severity: SeverityBlock, Passed: true, Skipped: true,
			Reason: "apply-recipe.json declares 0 op(s); nothing to replay — the canonical patch is the sole authority"}
	case recipePresent:
		if opIdx, rerr := replayRecipeOpsInShadow(shadowPath, in.recipe.Operations); rerr != nil {
			rem := fmt.Sprintf("recipe op #%d failed in shadow replay: %v; investigate or re-run tpatch implement %s", opIdx, rerr, slug)
			failedAt := ""
			if landed {
				rem = remediationR4(opIdx, anchorCommit, rerr, slug)
				failedAt = FailedAtLandedBaseline
			}
			out.v7 = withMode(store.VerifyCheckResult{ID: CheckRecipeReplayClean, Severity: SeverityBlock, Passed: false, Remediation: rem}, v7Mode)
			out.v8 = withMode(skipV8Because("V7 (recipe_replay_clean) failed"), v8Mode)
			if out.failedAt == "" {
				out.failedAt = failedAt
			}
			return out
		}
		v7 = store.VerifyCheckResult{ID: CheckRecipeReplayClean, Severity: SeverityBlock, Passed: true}
	default:
		v7 = store.VerifyCheckResult{ID: CheckRecipeReplayClean, Severity: SeverityBlock, Passed: true, Skipped: true,
			Reason: "no apply-recipe.json (precondition not met)"}
	}
	out.v7 = withMode(v7, v7Mode)

	// 6. V8 — forward `git apply --check` at the SAME baseline, after
	//    resetting the shadow (GH #2 / v0.11.3 invariant).
	if !patchPresent {
		out.v8 = withMode(store.VerifyCheckResult{
			ID: CheckPostApplyPatchReplayClean, Severity: SeverityBlock, Passed: true, Skipped: true,
			Reason: "no post-apply.patch (precondition not met)",
		}, v8Mode)
		return out
	}
	if recipePresent && len(in.recipe.Operations) > 0 {
		if err := resetShadowToTree(ctx, shadowPath, closureBaselineTree); err != nil {
			out.v8 = withMode(store.VerifyCheckResult{
				ID: CheckPostApplyPatchReplayClean, Severity: SeverityBlock, Passed: false,
				Remediation: fmt.Sprintf("cannot reset shadow to closure-replayed baseline before V8: %v", err),
			}, v8Mode)
			return out
		}
	}
	patchPath := artifactPath(s.Root, slug, "post-apply.patch")
	if _, _, err := ctx.runShadowGit(shadowPath, "apply", "--check", patchPath); err != nil {
		rem := fmt.Sprintf("post-apply.patch no longer applies to closure-replayed baseline; run tpatch reconcile %s", slug)
		if landed {
			rem = remediationR5(anchorCommit)
			if out.failedAt == "" {
				out.failedAt = FailedAtLandedBaseline
			}
		}
		out.v8 = withMode(store.VerifyCheckResult{
			ID: CheckPostApplyPatchReplayClean, Severity: SeverityBlock, Passed: false, Remediation: rem,
		}, v8Mode)
		return out
	}
	out.v8 = withMode(store.VerifyCheckResult{ID: CheckPostApplyPatchReplayClean, Severity: SeverityBlock, Passed: true}, v8Mode)
	return out
}

func ctxAnchorCommit(anchor string) *string {
	if anchor == "" {
		return nil
	}
	a := anchor
	return &a
}

func artifactPath(root, slug, name string) string {
	return filepath.Join(root, ".tpatch", "features", slug, "artifacts", name)
}

func skipV10Because(reason string) store.VerifyCheckResult {
	return store.VerifyCheckResult{
		ID:       CheckWriteFilePreimageFresh,
		Severity: SeverityBlock,
		Passed:   true,
		Skipped:  true,
		Reason:   reason,
	}
}

// ── Closure arbitration (D13) ────────────────────────────────────────────

// arbitrateClosure walks the closure in topological order and either
// skips, replays, or fail-fasts each member. It returns a non-nil
// dynamicPhase ONLY on a fail-fast; otherwise it mutates the shadow with
// the members that must be replayed and records advisories and member
// baselines on the run context.
func arbitrateClosure(in anchoredInput, order []string, anchorTreeish, shadowPath, v7Mode, v8Mode, v10Mode string) *dynamicPhase {
	ctx := in.ctx
	slug := in.slug
	if anchorTreeish == "" {
		anchorTreeish = "HEAD"
	}

	fail := func(failedAt, parent, remediation string) *dynamicPhase {
		return &dynamicPhase{
			v7:         withMode(store.VerifyCheckResult{ID: CheckRecipeReplayClean, Severity: SeverityBlock, Passed: false, Remediation: remediation}, v7Mode),
			v8:         withMode(skipV8Because("skipped: parent-replay aborted before V8"), v8Mode),
			v10:        withMode(skipV10Because("skipped: parent-replay aborted before V10"), v10Mode),
			failedAt:   failedAt,
			parentSlug: parent,
			advisories: ctx.pendingAdvisories,
		}
	}

	for _, member := range order {
		if member == slug {
			continue
		}
		entry := ctx.inv.Entry(member)
		if entry == nil || entry.Err != nil || entry.Status == nil {
			err := fmt.Errorf("cannot load parent status")
			if entry != nil && entry.Err != nil {
				err = entry.Err
			}
			return fail(FailedAtParentReplay, member,
				fmt.Sprintf("hard parent %s failed to replay in shadow: %v; re-run tpatch verify %s on the parent first", member, err, member))
		}

		switch entry.Status.State {
		case store.StateUpstreamMerged:
			continue
		case store.StateUnapplied:
			return fail(FailedAtParentUnapplied, member, remediationR16(member, slug))
		case store.StateRejected:
			return fail(FailedAtParentRejected, member, remediationR17(member, slug))
		case store.StateApplied, store.StateActive:
			// `active` is total: treated identically to `applied`.
		default:
			return fail(FailedAtParentReplay, member,
				fmt.Sprintf("hard parent %s failed to replay in shadow: parent state is %q (need applied or upstream_merged); re-run tpatch verify %s on the parent first",
					member, entry.Status.State, member))
		}

		mev := ctx.classifyEvidence(member)

		// Terminal evidence on a member is never rescued.
		if mev.Terminal() {
			return fail(FailedAtParentEvidence, member, remediationR15(member, mev.Evidence.State, slug))
		}
		// D10 presence short-circuit for a slug-bearing member.
		if mev.ArtifactsAbsent {
			return fail(FailedAtLandedArtifacts, member, remediationR19(member))
		}

		memberPatchPresent := entry.Patch.Presence == PresenceNonEmpty
		if mev.Landed() {
			if !memberPatchPresent {
				return fail(FailedAtLandedArtifacts, member, remediationR19(member))
			}
			ladder := ctx.runLadder(anchorTreeish, artifactPath(ctx.root, member, "post-apply.patch"), entry.Patch.Bytes)
			if ladder.MissingObject {
				return fail(FailedAtLandingEvidence, member, remediationR22(member))
			}
			if ladder.Err != nil {
				// The probe could not run: report the reader failure,
				// not a drift claim (rev-1 finding 3).
				return fail(FailedAtLandingEvidence, member,
					remediationR10(member, fmt.Sprintf("the presence probe for hard parent %s could not run: %v", member, ladder.Err)))
			}
			if ladder.Blocked {
				return fail(FailedAtParentLandingDrift, member, remediationR14(member, mev.Evidence.AttestationCommit))
			}
			if ladder.ContextDrift {
				ctx.pendingAdvisories = append(ctx.pendingAdvisories, warnAdvisory(
					AdvisoryContextDrift, member, ladder.Path,
					remediationR3(mev.Evidence.AttestationCommit, ladder.Path)))
			}
			// A landed member contributes its own V10, evaluated at its
			// OWN replay-anchor parent tree (D15) — never the target's.
			mres := evaluateMemberV10(ctx, member, entry, slug)
			if mres.blocked {
				return fail(FailedAtParentLandingDrift, member,
					fmt.Sprintf("hard parent %s landed but its write-file preimages are stale at its own landing baseline: %s; verify %s first", member, mres.detail, member))
			}
			ctx.pendingAdvisories = append(ctx.pendingAdvisories, mres.advisories...)
			if mres.baseline != "" {
				if ctx.pendingMemberBaselines == nil {
					ctx.pendingMemberBaselines = map[string]string{}
				}
				ctx.pendingMemberBaselines[member] = mres.baseline
			}
			// Skipped: never replayed, so an append-file parent is not
			// duplicated at the anchor.
			continue
		}

		// evidence `none`
		if memberPatchPresent {
			ladder := ctx.runLadder(anchorTreeish, artifactPath(ctx.root, member, "post-apply.patch"), entry.Patch.Bytes)
			if ladder.Err == nil && !ladder.Blocked {
				ctx.pendingAdvisories = append(ctx.pendingAdvisories, warnAdvisory(
					AdvisoryUnattributedMaterialized, member, "", remediationR18(member)))
				continue
			}
		}

		// Replay — byte-identical to the shipped path.
		pr, prerr := loadParentRecipeFromInventory(entry)
		if prerr != nil {
			return fail(FailedAtParentReplay, member,
				fmt.Sprintf("hard parent %s failed to replay in shadow: %v; re-run tpatch verify %s on the parent first", member, prerr, member))
		}
		if _, rerr := replayRecipeOpsInShadow(shadowPath, pr.Operations); rerr != nil {
			return fail(FailedAtParentReplay, member,
				fmt.Sprintf("hard parent %s failed to replay in shadow: %v; re-run tpatch verify %s on the parent first", member, rerr, member))
		}
	}
	return nil
}

func loadParentRecipeFromInventory(entry *inventoryEntry) (ApplyRecipe, error) {
	if entry.Recipe.Presence == PresenceAbsent {
		return ApplyRecipe{}, fmt.Errorf("read parent recipe: no apply-recipe.json")
	}
	r, ok := entry.ParsedRecipe()
	if !ok {
		return ApplyRecipe{}, fmt.Errorf("parse parent recipe: apply-recipe.json is empty or unparseable")
	}
	return r, nil
}

// ── V10 (D15) ────────────────────────────────────────────────────────────

type v10Outcome struct {
	result     store.VerifyCheckResult
	advisories []VerifyAdvisory
	failedAt   string
}

type memberV10Outcome struct {
	blocked    bool
	detail     string
	baseline   string
	advisories []VerifyAdvisory
}

// evaluateV10 produces the TARGET's V10 row. `closureTree` is the anchor-H
// closure baseline (landed) and `anchorCommit` the commit reported in
// `member_baselines`; both are empty in forward mode, where the
// reference tree is `RecipeProvenance.BaseCommit` (Q15).
func evaluateV10(in anchoredInput, memberBaselines map[string]string, closureTree string, anchorCommit *string) v10Outcome {
	ctx := in.ctx
	slug := in.slug
	if !in.recipePresent {
		return v10Outcome{result: skipV10Because("skipped: V2 (recipe_parses) skipped or failed")}
	}
	// Supersession is answered from the capture, not a fresh
	// `ListFeatures()` scan (rev-1 adjudication finding 2).
	superseder, superseded := isFeatureSupersededIn(ctx.inv.Statuses(), slug)
	severity := SeverityBlock
	if superseded {
		severity = SeverityWarn
	}

	baselines := map[string]string{}
	for k, v := range memberBaselines {
		baselines[k] = v
	}

	var failures []string
	var advisories []VerifyAdvisory
	failedAt := ""

	// Reference tree for the target.
	refTree := closureTree
	if anchorCommit != nil {
		baselines[slug] = *anchorCommit
	}
	provenanceUnusable := false
	if refTree == "" && needsProvenance(in.recipe) {
		// Forward mode: the reference tree is `RecipeProvenance.BaseCommit`
		// (Q15). Resolved lazily so a recipe with no preimage op never
		// requires a sidecar (ADR-029 D4 legacy path, AC-L101).
		prov := ctx.resolveProvenance(in.entry)
		if !prov.OK {
			provenanceUnusable = true
			for i, op := range in.recipe.Operations {
				if op.Type != "write-file" || op.PreimageHash == nil {
					continue
				}
				failures = append(failures, remediationR24(i+1, op.Path, prov.Reason, slug))
				break
			}
			if prov.Reason == "unreachable" {
				advisories = append(advisories, warnAdvisory(AdvisoryProvenanceUnreachable, slug, "",
					fmt.Sprintf("provenance-unreachable: artifacts/recipe-provenance.json for %s records base_commit %s, which is not reachable from HEAD", slug, prov.BaseCommit)))
			}
			failedAt = FailedAtRecipeProvenance
		} else {
			refTree = prov.BaseCommit
			baselines[slug] = prov.BaseCommit
		}
	}

	later := laterTouchIndexFromInventory(ctx.inv, slug)
	for i, op := range in.recipe.Operations {
		if op.Type != "write-file" {
			continue
		}
		if op.PreimageHash == nil {
			// ADR-029 D4 legacy path: pass, no re-warn.
			continue
		}
		if refTree == "" || provenanceUnusable {
			// Without a usable reference tree the op cannot be evaluated.
			// Verify NEVER falls back to the live working tree — the
			// measured E11/E12 false block.
			continue
		}
		ok, msg, observed := ctx.preimageAtTree(refTree, slug, i+1, op)
		if !ok {
			if observed != "" {
				failures = append(failures, remediationR12(i+1, op.Path, *op.PreimageHash, refTree, observed, slug))
			} else {
				failures = append(failures, msg)
			}
		}
		if laterSlug, hit := later[op.Path]; hit {
			advisories = append(advisories, warnAdvisory(AdvisoryLaterTouch, slug, op.Path,
				remediationR13(laterSlug, op.Path, slug)))
		}
	}

	mode := ModeProvenanceAnchor
	if anchorCommit != nil {
		mode = ModeHistoricalAnchor
	} else if !needsProvenance(in.recipe) {
		mode = ModeForward
	}

	if len(failures) == 0 {
		res := store.VerifyCheckResult{ID: CheckWriteFilePreimageFresh, Severity: severity, Passed: true, Mode: mode}
		if len(baselines) > 0 {
			res.MemberBaselines = baselines
		}
		if mode == ModeProvenanceAnchor {
			prov := ctx.resolveProvenance(in.entry)
			res.ProvenanceHashBound = boolPtr(prov.HashBound)
		}
		return v10Outcome{result: res, advisories: advisories}
	}
	remediation := strings.Join(failures, "; ")
	if superseded {
		remediation = fmt.Sprintf("%s (downgraded to warn: superseded by %q per ADR-029 D7 + PRD-feature-supersession §4.5 \"Reconcile interaction with write-file safety\")",
			remediation, superseder)
	}
	res := store.VerifyCheckResult{ID: CheckWriteFilePreimageFresh, Severity: severity, Passed: false, Mode: mode, Remediation: remediation}
	if len(baselines) > 0 {
		res.MemberBaselines = baselines
	}
	return v10Outcome{result: res, advisories: advisories, failedAt: failedAt}
}

// needsProvenance reports whether any write-file op carries a real
// preimage_hash, which is the only case that requires a reference tree.
func needsProvenance(recipe ApplyRecipe) bool {
	for _, op := range recipe.Operations {
		if op.Type == "write-file" && op.PreimageHash != nil {
			return true
		}
	}
	return false
}

// evaluateMemberV10 evaluates a LANDED closure member's V10 at that
// member's OWN replay-anchor parent tree (D15). Block-class outcomes
// contribute to `parent-landing-drift`; warn-class later-touch outcomes
// are aggregated into the run's advisories under the member's slug.
func evaluateMemberV10(ctx *verifyRunContext, member string, entry *inventoryEntry, target string) memberV10Outcome {
	recipe, ok := entry.ParsedRecipe()
	if !ok {
		return memberV10Outcome{}
	}
	if !needsProvenance(recipe) {
		// Nothing to evaluate; still record the member's baseline when
		// one is resolvable so the report proves per-member anchoring.
		anchor := ctx.resolveAnchor(member)
		if anchor.Available {
			return memberV10Outcome{baseline: anchor.Anchor}
		}
		return memberV10Outcome{}
	}
	anchor := ctx.resolveAnchor(member)
	if !anchor.Available {
		return memberV10Outcome{blocked: true, detail: remediationR11(member)}
	}
	var failures []string
	var advisories []VerifyAdvisory
	later := laterTouchIndexFromInventory(ctx.inv, member)
	for i, op := range recipe.Operations {
		if op.Type != "write-file" || op.PreimageHash == nil {
			continue
		}
		ok, msg, observed := ctx.preimageAtTree(anchor.Anchor, member, i+1, op)
		if !ok {
			if observed != "" {
				failures = append(failures, remediationR12(i+1, op.Path, *op.PreimageHash, anchor.Anchor, observed, member))
			} else {
				failures = append(failures, msg)
			}
		}
		if laterSlug, hit := later[op.Path]; hit {
			advisories = append(advisories, warnAdvisory(AdvisoryLaterTouch, member, op.Path,
				remediationR13(laterSlug, op.Path, member)))
		}
	}
	out := memberV10Outcome{baseline: anchor.Anchor, advisories: advisories}
	if len(failures) > 0 {
		out.blocked = true
		out.detail = strings.Join(failures, "; ")
	}
	return out
}

// preimageAtTree evaluates one write-file op's preimage_hash against a
// TREE, never the live working tree (the measured E11/E12 false block).
func (ctx *verifyRunContext) preimageAtTree(treeish, slug string, opIndex int, op RecipeOperation) (ok bool, msg string, observed string) {
	expected := *op.PreimageHash

	if expected == "" {
		_, found, err := ctx.blobAtTree(treeish, op.Path)
		if err != nil {
			return false, fmt.Sprintf("recipe drift: [%s] op %d %s: cannot read the baseline tree %s: %v", slug, opIndex, op.Path, treeish, err), ""
		}
		if found {
			return false, fmt.Sprintf("recipe drift: [%s] op %d %s: new-file collision — the path already exists at baseline %s but the recipe expected an empty preimage; regenerate the recipe against the current tree or reconcile before replay",
				slug, opIndex, op.Path, treeish), ""
		}
		return true, "", ""
	}

	// ADR-029 D1 form validation.
	if !strings.HasPrefix(expected, PreimageHashPrefix) ||
		len(expected) != len(PreimageHashPrefix)+PreimageHashHexLen ||
		!isLowercaseHex(expected[len(PreimageHashPrefix):]) {
		return false, fmt.Sprintf("recipe drift: [%s] op %d %s: malformed preimage_hash %q — expected 'sha256:<64 lowercase hex>' per ADR-029 D1",
			slug, opIndex, op.Path, expected), ""
	}

	data, found, err := ctx.blobAtTree(treeish, op.Path)
	if err != nil {
		return false, fmt.Sprintf("recipe drift: [%s] op %d %s: cannot read the baseline tree %s: %v", slug, opIndex, op.Path, treeish, err), ""
	}
	if !found {
		return false, fmt.Sprintf("recipe drift: [%s] op %d %s: expected preimage %s but the path is missing at baseline %s; regenerate the recipe or reconcile before replay",
			slug, opIndex, op.Path, expected, treeish), ""
	}
	got := PreimageHashPrefix + sha256Hex(data)
	if got != expected {
		return false, "", got
	}
	return true, "", ""
}

// ── Human report helpers ─────────────────────────────────────────────────

// writeLandedHeader renders the two §3.6.9 lines above the check list.
func writeLandedHeader(w io.StringWriter, r *VerifyReport) {
	if r.Baseline == nil {
		return
	}
	b := r.Baseline
	line := "  baseline: "
	if b.HistoricalAnchor != nil && b.HistoricalAnchor.State == AnchorStateAvailable {
		line += fmt.Sprintf("historical-anchor @ %s", shortSHA(b.HistoricalAnchor.Commit))
		if b.HistoricalAnchor.ReplayAnchorCommit != "" &&
			b.HistoricalAnchor.ReplayAnchorCommit != b.HistoricalAnchor.Commit {
			line += fmt.Sprintf(" (replay anchor %s)", shortSHA(b.HistoricalAnchor.ReplayAnchorCommit))
		}
		line += " · "
	} else if b.HistoricalAnchor != nil {
		line += fmt.Sprintf("historical-anchor unavailable (%s) · ", b.HistoricalAnchor.Reason)
	}
	line += fmt.Sprintf("current @ %s", shortSHA(b.CurrentCommit))
	if b.CurrentProbe != "" {
		line += " (isolated index)"
	}
	_, _ = w.WriteString(line + "\n")

	if r.LandingEvidence != nil {
		ev := r.LandingEvidence
		state := ev.State
		if state == "" {
			state = FailedAtLandedArtifacts
		}
		ln := fmt.Sprintf("  landing evidence: %s", state)
		if ev.AttestationCommit != "" {
			ln += fmt.Sprintf(" @ %s", shortSHA(ev.AttestationCommit))
		}
		if ev.PatchSHAMatch != nil && ev.RecipeSHAMatch != nil && ev.BaseCommitMatch != nil {
			ln += fmt.Sprintf(" (patch %s recipe %s base %s)",
				tick(*ev.PatchSHAMatch), tick(*ev.RecipeSHAMatch), tick(*ev.BaseCommitMatch))
		}
		_, _ = w.WriteString(ln + "\n")
	}
}

func tick(b bool) string {
	if b {
		return "✓"
	}
	return "✗"
}

func shortSHA(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}
