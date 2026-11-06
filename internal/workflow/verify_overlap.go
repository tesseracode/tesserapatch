package workflow

// Read-only overlap assessment — ADR-013 Amendment 2 (D20–D27), GH #25
// slices C1–C2.
//
// When a landed feature's anchor-C ladder blocks, verify explains WHY in
// a separate report field. The assessment never changes the verdict, the
// exit code, `failed_at`, any check's pass/skip state, the advisories or
// the persisted record; the only text it may change is the V8
// remediation (D26). It uses the run's immutable inventory and captured
// bytes, the run's captured HEAD commit and an isolated temp index only.
//
// The one exception is Amendment 3 (D28–D35, GH #25 A1): RunVerify may
// promote V8's current half when a recorded-descendant-evolution
// assessment names a single direct hard child and every D30 condition
// holds (promoteExplainedCurrentAnchor).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// Overlap assessment classes (D22). The vocabulary is closed.
const (
	OverlapRecordedDescendantEvolution = "recorded-descendant-evolution"
	OverlapContextOnlyDrift            = "context-only-drift"
	OverlapUnexplainedContentLoss      = "unexplained-content-loss"
	OverlapInvalidOrMissingEvidence    = "invalid-or-missing-evidence"
)

// `verdict_effect` values: `none` (D20) unless Amendment 3 promoted V8's
// current half (D31).
const (
	OverlapVerdictEffectNone                = "none"
	OverlapVerdictEffectCurrentAnchorPassed = "current-anchor-passed"
)

// Procedure steps that can decide a class (D20 `failed_step`).
const (
	OverlapStepTargetPaths = "target-paths"
	OverlapStepCandidates  = "candidates"
	OverlapStepEligibility = "eligibility"
	OverlapStepOrder       = "order"
	OverlapStepBaseline    = "baseline"
	OverlapStepReplay      = "replay"
	OverlapStepCompare     = "compare"
)

// Eligibility rules (PRD §6.1 E1–E6). E3 decides candidacy and is never
// reported as a failure.
const (
	OverlapRuleE1 = "E1"
	OverlapRuleE2 = "E2"
	OverlapRuleE4 = "E4"
	OverlapRuleE5 = "E5"
	OverlapRuleE6 = "E6"
)

// VerifyOverlapAssessment is the explanation-only report field (D20).
type VerifyOverlapAssessment struct {
	Class         string                    `json:"class"`
	VerdictEffect string                    `json:"verdict_effect"`
	Detail        string                    `json:"detail"`
	FailedStep    string                    `json:"failed_step,omitempty"`
	Descendants   []VerifyOverlapDescendant `json:"descendants,omitempty"`
}

// VerifyOverlapDescendant is one candidate the assessment considered.
type VerifyOverlapDescendant struct {
	Slug              string `json:"slug"`
	AttestationCommit string `json:"attestation_commit,omitempty"`
	Evidence          string `json:"evidence"`
	FailedRule        string `json:"failed_rule,omitempty"`
	Detail            string `json:"detail,omitempty"`
}

// overlapNestedVerify is one memoized same-run descendant verify (D23).
type overlapNestedVerify struct {
	passed bool
	detail string
	// currentClean records that the descendant's own V8 current anchor
	// matched with full context (`materialized-clean`), so every one of
	// its paths is plainly present at HEAD (Amendment 3 D30 P9).
	currentClean bool
}

// overlapCandidate is the working state for one candidate descendant.
type overlapCandidate struct {
	slug  string
	entry *inventoryEntry
	out   VerifyOverlapDescendant
}

func newOverlapAssessment(class, step, detail string) *VerifyOverlapAssessment {
	return &VerifyOverlapAssessment{
		Class:         class,
		VerdictEffect: OverlapVerdictEffectNone,
		Detail:        detail,
		FailedStep:    step,
	}
}

// contextOnlyDriftAssessment is the D21 report for a ladder that matched
// with reduced but non-zero context. No candidate is considered.
func contextOnlyDriftAssessment(path string) *VerifyOverlapAssessment {
	return newOverlapAssessment(OverlapContextOnlyDrift, "",
		fmt.Sprintf("the recorded postimage is present at HEAD with drifted surrounding context at %s; no descendant was considered", path))
}

// assessBlockedOverlap runs the D22 procedure for a landed target whose
// anchor-C ladder blocked.
func assessBlockedOverlap(in anchoredInput) *VerifyOverlapAssessment {
	ctx := in.ctx
	target := in.slug

	// 1. Target paths, from the captured canonical patch, strict parser.
	targetPaths, err := gitutil.PathsAffectedByPatchStrict(string(in.entry.Patch.Bytes))
	if err != nil || len(targetPaths) == 0 {
		return newOverlapAssessment(OverlapUnexplainedContentLoss, OverlapStepTargetPaths,
			"the feature's canonical patch paths could not be read with the strict parser, so no descendant can be matched to them")
	}
	targetSet := map[string]bool{}
	for _, p := range targetPaths {
		targetSet[p] = true
	}

	// 2. Candidates: hard descendants (E3) whose canonical patch touches a
	//    target path (E2). An unparseable descendant patch is a candidate
	//    that fails E2.
	chain := hardDescendantChains(ctx, target)
	var cands []*overlapCandidate
	for _, slug := range chainSlugs(chain) {
		entry := ctx.inv.Entry(slug)
		if entry == nil || entry.Err != nil {
			continue
		}
		c := &overlapCandidate{slug: slug, entry: entry}
		c.out.Slug = slug
		if entry.Patch.Err != nil {
			c.fail(OverlapRuleE2, "its canonical patch could not be read")
			cands = append(cands, c)
			continue
		}
		if entry.Patch.Presence != PresenceNonEmpty {
			continue
		}
		paths, perr := gitutil.PathsAffectedByPatchStrict(string(entry.Patch.Bytes))
		if perr != nil {
			c.fail(OverlapRuleE2, "its canonical patch paths could not be read with the strict parser")
			cands = append(cands, c)
			continue
		}
		overlaps := false
		for _, p := range paths {
			if targetSet[p] {
				overlaps = true
				break
			}
		}
		if !overlaps {
			continue
		}
		cands = append(cands, c)
	}
	if len(cands) == 0 {
		return newOverlapAssessment(OverlapUnexplainedContentLoss, OverlapStepCandidates,
			"no feature that depends on this one through hard edges touches its paths")
	}

	// 3. Eligibility, E1 → E2 → E4 → E5 → E6, every candidate.
	targetAttestation := in.evidence.Evidence.AttestationCommit
	bindingMemo := map[string]string{}
	for _, c := range cands {
		ev := ctx.classifyEvidence(c.slug)
		c.out.Evidence = ev.Evidence.State
		c.out.AttestationCommit = ev.Evidence.AttestationCommit
		if c.out.FailedRule != "" {
			continue
		}
		if ev.ArtifactsAbsent || ev.Evidence.State != EvidenceExact {
			c.fail(OverlapRuleE1, fmt.Sprintf("its landing evidence is %s, not exact", evidenceStateLabel(ev)))
			continue
		}
		if detail := overlapChainBinding(ctx, chain, target, c.slug, bindingMemo); detail != "" {
			c.fail(OverlapRuleE4, detail)
			continue
		}
		detail, execErr := overlapLandingOrder(ctx, targetAttestation, c.out.AttestationCommit)
		if execErr != "" {
			// Git could not answer: the step could not run, so no
			// descendant is judged and none is named (D22, D26).
			return newOverlapAssessment(OverlapInvalidOrMissingEvidence, OverlapStepEligibility,
				fmt.Sprintf("%s for descendant %s", execErr, c.slug))
		}
		if detail != "" {
			c.fail(OverlapRuleE5, detail)
			continue
		}
	}
	for _, c := range cands {
		if c.out.FailedRule != "" {
			continue
		}
		if nested := ctx.sameRunDescendantVerify(in.store, c.slug); !nested.passed {
			c.fail(OverlapRuleE6, nested.detail)
		}
	}
	for _, c := range cands {
		if c.out.FailedRule != "" {
			a := newOverlapAssessment(OverlapInvalidOrMissingEvidence, OverlapStepEligibility,
				fmt.Sprintf("descendant %s cannot be used as an explanation (%s: %s)", c.slug, c.out.FailedRule, c.out.Detail))
			a.Descendants = candidateOutputs(cands)
			return a
		}
	}

	// 4. Total order by attestation-commit ancestry.
	ordered, step, class, detail := orderCandidatesByLanding(ctx, cands)
	if class != "" {
		a := newOverlapAssessment(class, step, detail)
		a.Descendants = candidateOutputs(cands)
		return a
	}

	// 5–7. Baseline, replay and compare in one isolated temp index.
	if class, step, detail := replayOverlap(ctx, in, ordered, targetPaths); class != "" {
		a := newOverlapAssessment(class, step, detail)
		a.Descendants = candidateOutputs(ordered)
		return a
	}

	names := make([]string, 0, len(ordered))
	for _, c := range ordered {
		names = append(names, c.slug)
	}
	a := newOverlapAssessment(OverlapRecordedDescendantEvolution, "",
		fmt.Sprintf("this feature's landing commit %s, evolved by its recorded descendants in landing order (%s), reproduces HEAD on every one of its paths",
			targetAttestation, strings.Join(names, ", ")))
	a.Descendants = candidateOutputs(ordered)
	return a
}

func (c *overlapCandidate) fail(rule, detail string) {
	c.out.FailedRule = rule
	c.out.Detail = detail
}

func candidateOutputs(cands []*overlapCandidate) []VerifyOverlapDescendant {
	out := make([]VerifyOverlapDescendant, 0, len(cands))
	for _, c := range cands {
		out = append(out, c.out)
	}
	return out
}

func evidenceStateLabel(ev *landingEvidenceResult) string {
	if ev.ArtifactsAbsent {
		return "landed-artifacts-absent"
	}
	if ev.Evidence.State == "" {
		return "none"
	}
	return ev.Evidence.State
}

// hardDescendantChains returns every feature that reaches target through
// a chain of `hard` depends_on edges in the run inventory, mapped to its
// direct hard parents that lie on such a chain (the target itself or
// another chain member). The target is never its own descendant.
func hardDescendantChains(ctx *verifyRunContext, target string) map[string][]string {
	children := map[string][]string{}
	hardParents := map[string][]string{}
	for _, st := range ctx.inv.Statuses() {
		for _, d := range st.DependsOn {
			if d.Kind != store.DependencyKindHard {
				continue
			}
			children[d.Slug] = append(children[d.Slug], st.Slug)
			hardParents[st.Slug] = append(hardParents[st.Slug], d.Slug)
		}
	}
	onChain := map[string]bool{target: true}
	queue := []string{target}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, ch := range children[cur] {
			if onChain[ch] {
				continue
			}
			onChain[ch] = true
			queue = append(queue, ch)
		}
	}
	out := map[string][]string{}
	for slug := range onChain {
		if slug == target {
			continue
		}
		seen := map[string]bool{}
		var parents []string
		for _, p := range hardParents[slug] {
			if onChain[p] && !seen[p] {
				seen[p] = true
				parents = append(parents, p)
			}
		}
		sort.Strings(parents)
		out[slug] = parents
	}
	return out
}

func chainSlugs(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// overlapChainBinding is E4 (D22 step 3): every hard edge on every chain
// from the candidate to the target is bound, including edges through
// intermediates that do not touch the target's paths. memo caches each
// member's own result for the whole assessment. Returns "" when bound.
func overlapChainBinding(ctx *verifyRunContext, chain map[string][]string, target, start string, memo map[string]string) string {
	seen := map[string]bool{start: true}
	queue := []string{start}
	var members []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		members = append(members, cur)
		for _, p := range chain[cur] {
			if p == target || seen[p] {
				continue
			}
			seen[p] = true
			queue = append(queue, p)
		}
	}
	sort.Strings(members[1:])
	for _, m := range members {
		detail, ok := memo[m]
		if !ok {
			detail = overlapMemberBinding(ctx, m, chain[m])
			memo[m] = detail
		}
		if detail == "" {
			continue
		}
		if m == start {
			return "its " + detail
		}
		return fmt.Sprintf("intermediate feature %s's %s", m, detail)
	}
	return ""
}

// overlapMemberBinding checks one chain member: its captured
// patch-generations.json binds its own captured canonical patch and, for
// every hard parent on a chain to the target, that parent's captured
// canonical patch. The returned phrase reads after "its " or "<slug>'s ".
func overlapMemberBinding(ctx *verifyRunContext, slug string, parents []string) string {
	entry := ctx.inv.Entry(slug)
	if entry == nil || entry.Err != nil {
		return "status could not be read from the run inventory"
	}
	if entry.GenerationsErr != nil {
		return "patch-generations.json could not be read"
	}
	if len(entry.Generations) == 0 {
		return "patch-generations.json is missing (legacy feature), so the hard edges cannot be checked"
	}
	var m store.PatchGenerationsManifest
	dec := json.NewDecoder(bytes.NewReader(entry.Generations))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return "patch-generations.json does not parse"
	}
	if dec.Decode(new(any)) != io.EOF {
		return "patch-generations.json has trailing data"
	}
	if err := store.ValidatePatchGenerations(slug, m); err != nil {
		return "patch-generations.json is not valid"
	}
	latest, ok := store.LatestPatchGeneration(m)
	if !ok {
		return "patch-generations.json records no generation"
	}
	if entry.Patch.Err != nil || entry.Patch.Presence != PresenceNonEmpty {
		return "canonical patch is absent or unreadable, so no generation can bind it"
	}
	if latest.PatchSHA256 != store.SHA256HexBytes(entry.Patch.Bytes) {
		return "latest patch generation does not record the current canonical patch"
	}
	for _, parent := range parents {
		var dep *store.GenerationDependency
		for i := range latest.Dependencies {
			if latest.Dependencies[i].Slug == parent && latest.Dependencies[i].Kind == store.DependencyKindHard {
				dep = &latest.Dependencies[i]
				break
			}
		}
		if dep == nil {
			return fmt.Sprintf("latest patch generation records no hard dependency on %s", parent)
		}
		pe := ctx.inv.Entry(parent)
		if pe == nil || pe.Err != nil || pe.Patch.Err != nil || pe.Patch.Presence != PresenceNonEmpty {
			return fmt.Sprintf("latest patch generation cannot be bound to %s, which has no canonical patch", parent)
		}
		if dep.ParentPatchSHA256 != store.SHA256HexBytes(pe.Patch.Bytes) {
			return fmt.Sprintf("latest patch generation is bound to a different canonical patch of %s", parent)
		}
	}
	return ""
}

// overlapLandingOrder is E5: the candidate's attestation commit differs
// from the target's, descends from it, and is reachable from the run's
// HEAD. detail is the rule failure ("" when satisfied); execErr is set
// instead when Git could not answer an ancestry query, which is an
// execution failure of the step, never a judgement of the descendant.
func overlapLandingOrder(ctx *verifyRunContext, targetAttestation, attestation string) (detail, execErr string) {
	if attestation == "" || targetAttestation == "" {
		return "its landing commit is unknown", ""
	}
	if attestation == targetAttestation {
		return "it shares this feature's landing commit", ""
	}
	after, err := ctx.isAncestorChecked(targetAttestation, attestation)
	if err != nil {
		return "", "git could not determine whether the landing commit descends from this feature's landing commit"
	}
	if !after {
		return fmt.Sprintf("its landing commit %s does not descend from this feature's landing commit %s", attestation, targetAttestation), ""
	}
	reachable, err := ctx.isAncestorChecked(attestation, ctx.head)
	if err != nil {
		return "", "git could not determine whether the landing commit is reachable from HEAD"
	}
	if !reachable {
		return fmt.Sprintf("its landing commit %s is not reachable from HEAD", attestation), ""
	}
	return "", ""
}

// orderCandidatesByLanding sorts the candidates by attestation-commit
// ancestry and refuses any pair that ancestry does not order (D22 step 4).
func orderCandidatesByLanding(ctx *verifyRunContext, cands []*overlapCandidate) ([]*overlapCandidate, string, string, string) {
	for i := 0; i < len(cands); i++ {
		for j := i + 1; j < len(cands); j++ {
			a, b := cands[i], cands[j]
			if a.out.AttestationCommit == b.out.AttestationCommit {
				return nil, OverlapStepOrder, OverlapUnexplainedContentLoss,
					fmt.Sprintf("descendants %s and %s share landing commit %s, so their landing order is undefined", a.slug, b.slug, a.out.AttestationCommit)
			}
			ab, err1 := ctx.isAncestorChecked(a.out.AttestationCommit, b.out.AttestationCommit)
			ba, err2 := ctx.isAncestorChecked(b.out.AttestationCommit, a.out.AttestationCommit)
			if err1 != nil || err2 != nil {
				return nil, OverlapStepOrder, OverlapInvalidOrMissingEvidence,
					fmt.Sprintf("the landing order of descendants %s and %s could not be determined", a.slug, b.slug)
			}
			if !ab && !ba {
				return nil, OverlapStepOrder, OverlapUnexplainedContentLoss,
					fmt.Sprintf("descendants %s and %s landed on parallel histories; merge topologies are not supported", a.slug, b.slug)
			}
		}
	}
	ordered := append([]*overlapCandidate(nil), cands...)
	sort.SliceStable(ordered, func(i, j int) bool {
		ok, _ := ctx.isAncestorChecked(ordered[i].out.AttestationCommit, ordered[j].out.AttestationCommit)
		return ok
	})
	return ordered, "", "", ""
}

// overlapPatchFiles stages captured patch bytes in run-owned temporary
// files inside the D11 directory, so git applies exactly the bytes the
// run captured and never rereads an artifact file (D24).
type overlapPatchFiles struct {
	dir   string
	paths []string
}

func (f *overlapPatchFiles) write(data []byte) (string, error) {
	if err := os.MkdirAll(f.dir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(f.dir, "overlap-patch-*")
	if err != nil {
		return "", err
	}
	f.paths = append(f.paths, tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", err
	}
	return tmp.Name(), tmp.Close()
}

func (f *overlapPatchFiles) cleanup() {
	for _, p := range f.paths {
		_ = os.Remove(p)
	}
}

// replayOverlap is D22 steps 5–7: seed an isolated index from the
// target's landing commit, prove the target's postimage was present
// there, replay every candidate on the target's paths in landing order,
// and require the result to equal the run's captured HEAD on those
// paths. Returns an empty class on success.
func replayOverlap(ctx *verifyRunContext, in anchoredInput, ordered []*overlapCandidate, targetPaths []string) (string, string, string) {
	landing := in.evidence.Evidence.AttestationCommit
	if ctx.headErr != nil || ctx.head == "" || landing == "" {
		return OverlapInvalidOrMissingEvidence, OverlapStepBaseline,
			"the run's HEAD commit or this feature's landing commit is unknown"
	}
	files := &overlapPatchFiles{dir: ctx.tempIndexDir()}
	defer files.cleanup()
	idx, err := ctx.newTempIndex()
	if err != nil {
		return OverlapInvalidOrMissingEvidence, OverlapStepBaseline,
			"the isolated index for the replay could not be allocated"
	}
	defer func() { _ = idx.Close() }()
	if err := idx.ReadTree(landing); err != nil {
		return OverlapInvalidOrMissingEvidence, OverlapStepBaseline,
			fmt.Sprintf("landing commit %s could not be read into the isolated index", landing)
	}

	targetPatch, err := files.write(in.entry.Patch.Bytes)
	if err != nil {
		return OverlapInvalidOrMissingEvidence, OverlapStepBaseline,
			"this feature's captured canonical patch could not be staged for git"
	}
	check := idx.ApplyCheck(gitutil.ApplyCheckOptions{PatchPath: targetPatch, Reverse: true})
	if !check.OK {
		if !check.ApplyAnswered() {
			return OverlapInvalidOrMissingEvidence, OverlapStepBaseline,
				fmt.Sprintf("git could not check this feature's canonical patch at its landing commit %s", landing)
		}
		return OverlapUnexplainedContentLoss, OverlapStepBaseline,
			fmt.Sprintf("this feature's recorded postimage is not present at its own landing commit %s, so no descendant can explain its absence", landing)
	}

	for _, c := range ordered {
		patch, err := files.write(c.entry.Patch.Bytes)
		if err != nil {
			return OverlapInvalidOrMissingEvidence, OverlapStepReplay,
				fmt.Sprintf("descendant %s's captured canonical patch could not be staged for git", c.slug)
		}
		res := idx.ApplyCached(patch, targetPaths)
		if !res.OK {
			if !res.ApplyAnswered() {
				return OverlapInvalidOrMissingEvidence, OverlapStepReplay,
					fmt.Sprintf("git could not apply descendant %s's canonical patch during the replay", c.slug)
			}
			return OverlapUnexplainedContentLoss, OverlapStepReplay,
				fmt.Sprintf("descendant %s's canonical patch does not apply to this feature's paths in landing order from landing commit %s", c.slug, landing)
		}
	}

	diff := idx.DiffCachedQuiet(ctx.head, targetPaths)
	if !diff.Answered {
		return OverlapInvalidOrMissingEvidence, OverlapStepCompare,
			"git could not compare the replay with HEAD"
	}
	if !diff.Equal {
		return OverlapUnexplainedContentLoss, OverlapStepCompare,
			fmt.Sprintf("this feature's paths at HEAD differ from landing commit %s evolved by its recorded descendants, so a change outside those descendants touched them", landing)
	}
	return "", "", ""
}

// sameRunDescendantVerify is E6: the descendant's own single-feature
// verify in THIS run context (same inventory, enumeration and probe
// memos), without persisting, with HEAD pinned to the captured commit,
// run-owned shadow names, a private copy of the landing-evidence memo,
// the assessment disabled, and the run's pending arbitration state saved
// and restored around it (D23). Memoized per slug for the rest of the
// run.
func (ctx *verifyRunContext) sameRunDescendantVerify(s *store.Store, slug string) overlapNestedVerify {
	if ctx.overlapVerifyMemo == nil {
		ctx.overlapVerifyMemo = map[string]overlapNestedVerify{}
	}
	if cached, ok := ctx.overlapVerifyMemo[slug]; ok {
		return cached
	}
	report, err := ctx.runNestedVerify(s, slug)

	res := overlapNestedVerify{passed: true}
	switch {
	case IsRefused(err):
		res = overlapNestedVerify{detail: "its own same-run verify refused its lifecycle state"}
	case err != nil || report == nil:
		res = overlapNestedVerify{detail: "its own same-run verify could not complete"}
	case report.Verdict != "passed":
		res = overlapNestedVerify{detail: "its own same-run verify failed" + nestedFailureSuffix(report)}
	default:
		for _, c := range report.Checks {
			if c.ID == CheckPostApplyPatchReplayClean {
				res.currentClean = c.AnchorResults["current"] == CurrentMaterializedClean
			}
		}
	}
	ctx.overlapVerifyMemo[slug] = res
	return res
}

// runNestedVerify runs the ordinary pipeline for slug in nested mode and
// restores every piece of mutable run state on return, including after a
// panic.
func (ctx *verifyRunContext) runNestedVerify(s *store.Store, slug string) (*VerifyReport, error) {
	savedAdvisories := ctx.pendingAdvisories
	savedBaselines := ctx.pendingMemberBaselines
	savedNested := ctx.overlapNested
	savedEvidence := ctx.evidenceMemo
	defer func() {
		ctx.pendingAdvisories = savedAdvisories
		ctx.pendingMemberBaselines = savedBaselines
		ctx.overlapNested = savedNested
		ctx.evidenceMemo = savedEvidence
	}()
	ctx.pendingAdvisories = nil
	ctx.pendingMemberBaselines = nil
	ctx.overlapNested = true
	ctx.evidenceMemo = cloneEvidenceMemo(savedEvidence)
	return runVerifyWithContext(s, slug, VerifyOptions{NoWrite: true}, ctx)
}

// cloneEvidenceMemo deep-copies the landing-evidence memo. The landed
// pipeline rewrites a memoized result's state on terminal probe outcomes,
// so a nested verify must never write through to the run's own entries.
func cloneEvidenceMemo(m map[string]*landingEvidenceResult) map[string]*landingEvidenceResult {
	out := make(map[string]*landingEvidenceResult, len(m))
	for k, v := range m {
		if v == nil {
			out[k] = nil
			continue
		}
		c := *v
		c.Evidence.PatchSHAMatch = cloneBoolPtr(v.Evidence.PatchSHAMatch)
		c.Evidence.RecipeSHAMatch = cloneBoolPtr(v.Evidence.RecipeSHAMatch)
		c.Evidence.BaseCommitMatch = cloneBoolPtr(v.Evidence.BaseCommitMatch)
		c.Evidence.BaseCommitReachable = cloneBoolPtr(v.Evidence.BaseCommitReachable)
		c.wellFormed = append([]gitutil.CommitRecord(nil), v.wellFormed...)
		c.Advisories = append([]VerifyAdvisory(nil), v.Advisories...)
		out[k] = &c
	}
	return out
}

func cloneBoolPtr(b *bool) *bool {
	if b == nil {
		return nil
	}
	v := *b
	return &v
}

func nestedFailureSuffix(r *VerifyReport) string {
	if r.FailedAt != "" {
		return " at " + r.FailedAt
	}
	for _, c := range r.Checks {
		if !c.Passed && !c.Skipped && c.Severity != SeverityWarn {
			return " at " + c.ID
		}
	}
	return ""
}

// overlapRemediation picks the V8 remediation for a blocked ladder by
// assessment class (D26). R1/R2 stay the answer for every other class.
// reason says why a recorded-descendant-evolution explanation cannot pass
// the check (Amendment 3 D32).
func overlapRemediation(a *VerifyOverlapAssessment, slug, attestation, fallback, reason string) string {
	if a == nil {
		return fallback
	}
	switch a.Class {
	case OverlapRecordedDescendantEvolution:
		parts := make([]string, 0, len(a.Descendants))
		for _, d := range a.Descendants {
			parts = append(parts, d.Slug+" @ "+d.AttestationCommit)
		}
		return remediationR25(slug, attestation, strings.Join(parts, ", "), reason)
	case OverlapInvalidOrMissingEvidence:
		for _, d := range a.Descendants {
			if d.FailedRule != "" {
				return remediationR26(slug, d.Slug, d.FailedRule, d.Detail)
			}
		}
		return remediationR26Execution(slug, a.FailedStep, a.Detail)
	}
	return fallback
}

func remediationR25(slug, sha, descendants, reason string) string {
	return fmt.Sprintf("landed feature: post-apply.patch postimage is not present at HEAD because recorded descendant(s) %s rewrote it after landing commit %s (overlap assessment: recorded-descendant-evolution) — the explanation cannot pass this check because %s, so it stays failed; do NOT re-land %s or run tpatch reconcile to restore it, which would undo the descendants' recorded changes", descendants, sha, reason, slug)
}

// remediationR27 is the `explained-by-descendant` advisory message
// (Amendment 3 D31).
func remediationR27(slug, sha, child, childSHA string) string {
	return fmt.Sprintf("landed feature: post-apply.patch postimage is not present at HEAD, but HEAD on every one of its paths is exactly landing commit %s evolved by its recorded hard child %s @ %s (overlap assessment: recorded-descendant-evolution), so the current anchor passes — do NOT re-land %s or run tpatch reconcile to restore it, which would undo %s; change these paths through %s", sha, child, childSHA, slug, child, child)
}

// Reasons a recorded-descendant-evolution explanation cannot pass V8's
// current half (Amendment 3 D30, D32).
const overlapReasonOtherFailure = "another block-severity check failed in this run"

// explainedPromotionBlocker checks D30's P1, P4–P7 and P9 for an
// assessment computed by the anchor-C fold and returns why it cannot
// promote V8's current half, or "" when it can. P10 needs Git and is
// checked next by mergeFreeHistoryBlocker; P2 and P8 are checked after
// the whole run by promoteExplainedCurrentAnchor; P3 holds because a
// nested verify never assesses.
func explainedPromotionBlocker(a *VerifyOverlapAssessment, directHardChild bool, targetEvidence, historical string, childCurrentClean bool) string {
	switch {
	case a == nil || a.Class != OverlapRecordedDescendantEvolution:
		return "the overlap assessment did not find a recorded descendant evolution"
	case len(a.Descendants) != 1:
		return fmt.Sprintf("%d recorded descendants rewrote these paths and only a single direct hard child can explain them (GH #25 A1)", len(a.Descendants))
	case !directHardChild:
		return fmt.Sprintf("descendant %s reaches this feature only through intermediate features", a.Descendants[0].Slug)
	case targetEvidence != EvidenceExact:
		return fmt.Sprintf("this feature's own landing evidence is %s, not exact", targetEvidence)
	case historical != "passed":
		return "the historical half of this check did not pass"
	case !childCurrentClean:
		return fmt.Sprintf("descendant %s's own current anchor did not match with full context, so its own paths are not proven present at HEAD", a.Descendants[0].Slug)
	}
	return ""
}

// mergeFreeHistoryBlocker is D30 P10: merge topologies are refused in
// v1 (GH #25 Q2), so no merge commit may sit between this feature's
// landing commit and the captured HEAD. An unanswerable query refuses.
func mergeFreeHistoryBlocker(ctx *verifyRunContext, landing string) string {
	free, err := ctx.mergeFreeSince(landing)
	if err != nil {
		return fmt.Sprintf("git could not determine whether the history from landing commit %s to HEAD is free of merge commits", landing)
	}
	if !free {
		return fmt.Sprintf("the history from landing commit %s to HEAD contains a merge commit, and merge topologies are refused (GH #25 Q2)", landing)
	}
	return ""
}

// hasDirectHardEdge reports whether child declares a `hard` depends_on
// edge naming parent in the run inventory (D30 P5).
func hasDirectHardEdge(ctx *verifyRunContext, child, parent string) bool {
	for _, st := range ctx.inv.Statuses() {
		if st.Slug != child {
			continue
		}
		for _, d := range st.DependsOn {
			if d.Slug == parent && d.Kind == store.DependencyKindHard {
				return true
			}
		}
	}
	return false
}

// promoteExplainedCurrentAnchor passes V8's current half on a recorded
// single-child explanation (Amendment 3 D30/D31). It runs after the D21
// drop checks, so a dropped assessment (nil) never promotes (P2), and it
// requires that anchor C's block is the run's only failure (P8). It
// reports whether it promoted.
func promoteExplainedCurrentAnchor(report *VerifyReport, phase dynamicPhase) bool {
	a := report.OverlapAssessment
	if a == nil || !phase.overlapPromotable || !phase.anchorCSetFailedAt {
		return false
	}
	v8 := -1
	for i, c := range report.Checks {
		if c.ID == CheckPostApplyPatchReplayClean {
			v8 = i
			continue
		}
		if !c.Skipped && !c.Passed && (c.Severity == SeverityBlock || c.Severity == SeverityBlockAbort) {
			return false
		}
	}
	if v8 < 0 {
		return false
	}
	c := &report.Checks[v8]
	results := make(map[string]string, len(c.AnchorResults))
	for k, v := range c.AnchorResults {
		results[k] = v
	}
	results["current"] = CurrentExplainedByDescendant
	c.AnchorResults = results
	c.Passed = true
	c.Skipped = false
	c.Remediation = ""
	report.FailedAt = ""
	report.ParentSlug = ""
	a.VerdictEffect = OverlapVerdictEffectCurrentAnchorPassed
	report.Advisories = append(report.Advisories, phase.explainedAdvisory)
	return true
}

// remediationR26Execution is R26 when the assessment could not run a
// step: no descendant was judged, so none is named (D26).
func remediationR26Execution(slug, step, detail string) string {
	return fmt.Sprintf("landed feature: post-apply.patch postimage is not present at HEAD and the overlap assessment could not complete its %s step (%s) — no descendant was judged; fix the repository or tool failure and re-run tpatch verify %s before deciding whether to re-land %s", step, detail, slug, slug)
}

func remediationR26(slug, descendant, rule, detail string) string {
	return fmt.Sprintf("landed feature: post-apply.patch postimage is not present at HEAD; recorded descendant %s may explain it but cannot be used as evidence (%s: %s) — verify or repair %s first, then re-run tpatch verify %s; do NOT re-land %s yet", descendant, rule, detail, descendant, slug, slug)
}

// writeOverlapAssessment renders the D26 human lines.
func writeOverlapAssessment(w io.Writer, a *VerifyOverlapAssessment) {
	if a == nil {
		return
	}
	effect := "explanation only; verdict unchanged"
	if a.VerdictEffect == OverlapVerdictEffectCurrentAnchorPassed {
		effect = "explains the current anchor; V8 passes"
	}
	fmt.Fprintf(w, "  ~ overlap assessment: %s (%s) — %s\n", a.Class, effect, a.Detail)
	for _, d := range a.Descendants {
		line := "    - " + d.Slug
		if d.AttestationCommit != "" {
			line += " @ " + shortSHA(d.AttestationCommit)
		}
		if d.FailedRule != "" {
			line += fmt.Sprintf(" [%s: %s]", d.FailedRule, d.Detail)
		}
		fmt.Fprintln(w, line)
	}
}
