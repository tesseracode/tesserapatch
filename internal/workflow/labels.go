// Composable reconcile labels — M14.3 / ADR-011 D3 + D6.
//
// ComposeLabels reads a child feature's hard-dependency declarations,
// loads each parent's FeatureStatus, and derives the overlay labels per
// PRD-feature-dependencies §3.5. Labels are computed at READ time and
// overlay on top of the child's intrinsic Reconcile.Outcome — they are
// NOT persisted as new ReconcileOutcome enum values.
//
// AUTHORITATIVE SOURCE GUARD (ADR-010 D5, ADR-011 D6):
//
//   This function reads parent verdicts via store.LoadFeatureStatus —
//   specifically status.Reconcile.Outcome. It MUST NEVER consult
//   artifacts/reconcile-session.json. The session artifact is an audit
//   record of one RunReconcile invocation; status.json is the source of
//   current truth post-accept. Any future change here that adds a path
//   reading session artifacts is a behavioural regression.
//
// Soft dependencies never produce labels (ADR-011 D4). Multiple labels
// can stack (e.g. one parent waiting + another stale → two labels).
// Output is deduplicated and sorted alphabetically for deterministic
// JSON serialization.

package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// hasLabel reports whether labels contains the target label. Linear scan
// — label slices are typically 0-3 elements so a map would be overkill.
func hasLabel(labels []store.ReconcileLabel, target store.ReconcileLabel) bool {
	for _, l := range labels {
		if l == target {
			return true
		}
	}
	return false
}

// LabelWaitingOnParent. The set covers every pre-applied lifecycle state
// plus the in-flight reconcile states.
var transientStates = map[store.FeatureState]struct{}{
	store.StateRequested:         {},
	store.StateAnalyzed:          {},
	store.StateDefined:           {},
	store.StateImplementing:      {},
	store.StateReconciling:       {},
	store.StateReconcilingShadow: {},
}

// appliedSatisfyingStates: hard parents in any of these states are
// considered "applied" for label purposes — only stale and
// blocked-by-parent labels apply, never waiting-on-parent.
var appliedSatisfyingStates = map[store.FeatureState]struct{}{
	store.StateApplied:        {},
	store.StateActive:         {},
	store.StateUpstreamMerged: {},
}

// blockedReconcileOutcomes: a hard parent in StateApplied/Active whose
// last reconcile produced one of these verdicts is treated as blocked
// for label-composition purposes. The parent's working tree may be
// usable, but the operator has unresolved upstream work owed.
var blockedReconcileOutcomes = map[store.ReconcileOutcome]struct{}{
	store.ReconcileBlockedRequiresHuman:    {},
	store.ReconcileBlockedTooManyConflicts: {},
	store.ReconcileBlocked:                 {},
	store.ReconcileShadowAwaiting:          {},
}

// ComposeLabels returns the M14.3 overlay labels for a child feature
// based on its hard-parent set. Soft parents are skipped per ADR-011 D4.
//
// Returns an empty (nil) slice when:
//   - Config.DAGEnabled() is false (gate per ADR-011 D9).
//   - The child has no dependencies.
//   - The child's own Reconcile.Outcome marks it as retired (currently
//     ReconcileUpstreamed — per ADR-011, once a child is absorbed
//     upstream the parent context is irrelevant; surfacing
//     waiting-on-parent / blocked-by-parent on a retiring child is
//     misleading).
//   - No hard parent's state warrants a label.
//
// Errors only when the child's own status cannot be loaded — parent
// load failures are silently treated as LabelBlockedByParent (a missing
// parent is, by definition, not satisfying the child's dependency).
func ComposeLabels(s *store.Store, slug string) ([]store.ReconcileLabel, error) {
	cfg, err := s.LoadConfig()
	if err != nil {
		return nil, err
	}
	if !cfg.DAGEnabled() {
		return nil, nil
	}

	child, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return nil, err
	}
	return composeLabelsFromStatus(s, child), nil
}

// composeLabelsAt is like ComposeLabels but uses asOf as the
// child's effective reconcile baseline for the staleness check, instead
// of the on-disk child.Reconcile.AttemptedAt. Callers inside
// RunReconcile use this to ensure persisted Labels reflect the
// AttemptedAt about to be written, not the previous run's value
// (M14 fix-pass F2).
func composeLabelsAt(s *store.Store, slug string, asOf string) ([]store.ReconcileLabel, error) {
	cfg, err := s.LoadConfig()
	if err != nil {
		return nil, err
	}
	if !cfg.DAGEnabled() {
		return nil, nil
	}
	child, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return nil, err
	}
	if asOf != "" {
		child.Reconcile.AttemptedAt = asOf
	}
	return composeLabelsFromStatus(s, child), nil
}

// childRetiredOutcomes lists the child's own reconcile outcomes that
// suppress all parent-derived labels. M14 fix-pass F3 / ADR-011: once a
// child is absorbed upstream, parent state is irrelevant — the child is
// being retired. Currently only ReconcileUpstreamed qualifies; other
// outcomes (Reapplied, StillNeeded, Blocked, ShadowAwaiting,
// BlockedTooManyConflicts, BlockedRequiresHuman) keep the child live.
var childRetiredOutcomes = map[store.ReconcileOutcome]struct{}{
	store.ReconcileUpstreamed: {},
}

// composeLabelsFromStatus is the body shared by ComposeLabels and
// composeLabelsAt. It accepts an already-loaded FeatureStatus so the
// caller can override fields (e.g. AttemptedAt) prior to label
// composition without round-tripping through disk.
//
// Slice B (ADR-013 D5, PRD-verify-freshness §3.4.2): the function ALSO
// derives exactly one of the four freshness labels (`never-verified` /
// `verified-fresh` / `verified-stale` / `verify-failed`) and merges it
// into the returned set. This is read-time computation only — the
// freshness labels are NEVER persisted (D4 byte-identity contract).
// Callers that persist the result MUST first call StripFreshnessLabels
// (and StripSupersessionLabels for the Wave α overlay, see below).
//
// v0.12.0 Wave α (ADR-028 D4, PRD-feature-supersession §4.3): the
// function ALSO derives up to four supersession overlay labels
// (`superseded-by`, `active-superseder`, `stale-superseder`,
// `orphan-superseder`) from the read-time graph. Like freshness, these
// are NEVER persisted to `Reconcile.Labels` — persistence call sites
// must strip them via StripSupersessionLabels alongside the freshness
// strip. See composeSupersessionLabels for the direction-aware rules.
//
// PURITY (D5): no writes of any kind. The function reads only the
// `Verify` sub-record on `child`, parent FeatureStatus via
// `s.LoadFeatureStatus`, and the recipe / patch file bytes for hash
// comparison. It MUST NOT consult `artifacts/reconcile-session.json`
// or any other artifact (D6).
func composeLabelsFromStatus(s *store.Store, child store.FeatureStatus) []store.ReconcileLabel {
	set := make(map[store.ReconcileLabel]struct{})

	// M14 fix-pass F3 (preserved by Slice B): retired children surface
	// NO labels — neither M14.3 nor freshness. Once a child is absorbed
	// upstream the verify history is moot; surfacing `verified-fresh`
	// or `never-verified` on a retired feature is misleading.
	if _, retired := childRetiredOutcomes[child.Reconcile.Outcome]; retired {
		return nil
	}

	// M14.3 labels (skipped for features with no hard deps —
	// preserves pre-Slice-B label-set semantics).
	if len(child.DependsOn) > 0 {
		composeM143Labels(s, child, set)
	}

	// v0.12.0 Wave α — supersession overlay (ADR-028 D4,
	// PRD-feature-supersession §4.3). Composed independently of the
	// M14.3 set so a feature that is BOTH a superseder AND a target
	// (double role) surfaces both directional labels. Runs even when
	// `child.DependsOn` is empty because a feature can be the passive
	// target of someone else's `supersedes` edge.
	composeSupersessionLabels(s, child, set)

	// Slice B — freshness overlay. Exactly one freshness label is
	// always derived for non-retired features; it composes
	// orthogonally with the M14.3 set.
	set[deriveFreshnessLabel(s, child)] = struct{}{}

	out := make([]store.ReconcileLabel, 0, len(set))
	for l := range set {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// composeM143Labels populates `set` with the M14.3 dependency-graph
// overlay labels (`waiting-on-parent`, `blocked-by-parent`,
// `stale-parent-applied`). Unchanged from the pre-Slice-B inline body;
// extracted only so composeLabelsFromStatus can compose its result with
// the freshness overlay.
func composeM143Labels(s *store.Store, child store.FeatureStatus, set map[store.ReconcileLabel]struct{}) {
	for _, dep := range child.DependsOn {
		if dep.Kind != store.DependencyKindHard {
			continue // ADR-011 D4: soft deps never contribute to labels.
		}
		// CRITICAL — read parent verdict from status.json (ADR-010 D5).
		// Do NOT read artifacts/reconcile-session.json from any code
		// path reachable here; the adversarial test enforces this.
		parent, perr := s.LoadFeatureStatus(dep.Slug)
		if perr != nil {
			// Missing parent acts as a hard blocker.
			set[store.LabelBlockedByParent] = struct{}{}
			continue
		}

		// State-level classification first.
		if parent.State == store.StateBlocked {
			set[store.LabelBlockedByParent] = struct{}{}
			continue
		}
		if _, transient := transientStates[parent.State]; transient {
			set[store.LabelWaitingOnParent] = struct{}{}
			continue
		}
		if _, applied := appliedSatisfyingStates[parent.State]; applied {
			// upstream_merged with valid satisfied_by is fully retired:
			// the parent's changes are part of upstream, so neither
			// blocked-by-parent nor stale-parent-applied apply (the
			// child has no live local parent to drift against).
			if parent.State == store.StateUpstreamMerged {
				continue
			}
			// Reconcile-level overlay on applied/active parents.
			if _, blocked := blockedReconcileOutcomes[parent.Reconcile.Outcome]; blocked {
				set[store.LabelBlockedByParent] = struct{}{}
				continue
			}
			// Stale check: parent has been updated since the child's
			// last reconcile. We only flag this when the child has a
			// prior AttemptedAt — without a baseline timestamp there is
			// nothing to be "stale" against.
			if child.Reconcile.AttemptedAt != "" && parent.UpdatedAt != "" &&
				parent.UpdatedAt > child.Reconcile.AttemptedAt {
				set[store.LabelStaleParentApplied] = struct{}{}
			}
			continue
		}
		// Unknown / unhandled state — be conservative and treat as
		// blocked rather than silently dropping a label.
		set[store.LabelBlockedByParent] = struct{}{}
	}
}

// freshnessLabelSet lists the four read-time freshness labels. Used by
// StripFreshnessLabels so persistence call sites can remove freshness
// before writing Reconcile.Labels (D4: never persisted).
var freshnessLabelSet = map[store.ReconcileLabel]struct{}{
	store.LabelNeverVerified: {},
	store.LabelVerifiedFresh: {},
	store.LabelVerifiedStale: {},
	store.LabelVerifyFailed:  {},
}

// supersessionLabelSet lists the four v0.12.0 Wave α supersession
// overlay labels (ADR-028 D4, PRD-feature-supersession §4.3). Like
// freshness, these are derived at read time from the graph and MUST
// NOT be persisted to `Reconcile.Labels` — they must be recomputed on
// every render. Persistence call sites strip them via
// StripSupersessionLabels alongside the freshness strip.
var supersessionLabelSet = map[store.ReconcileLabel]struct{}{
	store.LabelSupersededBy:     {},
	store.LabelActiveSuperseder: {},
	store.LabelStaleSuperseder:  {},
	store.LabelOrphanSuperseder: {},
}

// IsFreshnessLabel reports whether l is one of the four Slice B
// freshness labels (ADR-013 D5).
func IsFreshnessLabel(l store.ReconcileLabel) bool {
	_, ok := freshnessLabelSet[l]
	return ok
}

// IsSupersessionLabel reports whether l is one of the four v0.12.0
// Wave α supersession overlay labels (ADR-028 D4). Mirror of
// IsFreshnessLabel — provided for symmetric strip / render logic.
//
// v0.12.0 rev-1 (F-SEXT-1): the `superseded-by` label is rendered as
// the composite `superseded-by <slug>` per PRD-feature-supersession §4.1
// binding label-value contract. The prefix check recognizes the
// composite so persistence strip and read-time diagnostics stay sound
// regardless of which superseder slug is appended.
func IsSupersessionLabel(l store.ReconcileLabel) bool {
	if _, ok := supersessionLabelSet[l]; ok {
		return true
	}
	return strings.HasPrefix(string(l), string(store.LabelSupersededBy)+" ")
}

// StripFreshnessLabels returns a copy of `in` with every freshness
// label removed. Persistence call sites (saveReconcileArtifacts, accept)
// MUST run their composed label slice through this before writing it
// to Reconcile.Labels — freshness is read-time only (D4 byte-identity).
//
// Returns nil when the result would be empty so `omitempty` keeps the
// JSON output absent rather than `"labels": []`.
func StripFreshnessLabels(in []store.ReconcileLabel) []store.ReconcileLabel {
	if len(in) == 0 {
		return nil
	}
	out := make([]store.ReconcileLabel, 0, len(in))
	for _, l := range in {
		if IsFreshnessLabel(l) {
			continue
		}
		out = append(out, l)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// StripSupersessionLabels returns a copy of `in` with every supersession
// overlay label removed (ADR-028 D4). Persistence call sites MUST run
// their composed label slice through this alongside StripFreshnessLabels
// so `Reconcile.Labels` never carries the four supersession labels — they
// are derived on every render from the graph.
//
// Returns nil when the result would be empty so `omitempty` keeps the
// JSON output absent rather than `"labels": []`.
func StripSupersessionLabels(in []store.ReconcileLabel) []store.ReconcileLabel {
	if len(in) == 0 {
		return nil
	}
	out := make([]store.ReconcileLabel, 0, len(in))
	for _, l := range in {
		if IsSupersessionLabel(l) {
			continue
		}
		out = append(out, l)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// stripDerivedLabels chains the two derived-label strips. Internal
// helper used by persist call sites so a future third derived-label
// group only needs one edit.
func stripDerivedLabels(in []store.ReconcileLabel) []store.ReconcileLabel {
	return StripSupersessionLabels(StripFreshnessLabels(in))
}

// supersederHealthyStates lists the FeatureStates that qualify a
// superseder feature as "healthy for default replay" (ADR-028 §
// Consequences, PRD §5). A superseder outside this set combined with a
// non-blocked reconcile outcome still filters the historical target out
// of the effective set, but surfaces `stale-superseder` so operators
// see the replacement itself needs repair before the effective stack is
// clean.
var supersederHealthyStates = map[store.FeatureState]struct{}{
	store.StateApplied:        {},
	store.StateActive:         {},
	store.StateUpstreamMerged: {},
}

// supersederIsHealthy reports whether the given feature is a valid
// active superseder for default replay purposes. Healthy = state in
// {applied, active, upstream_merged} AND reconcile outcome is NOT one
// of the terminal-blocked verdicts. This mirrors the pattern used by
// blockedReconcileOutcomes so a superseder that reconcile-blocks
// automatically demotes to `stale-superseder` without extra plumbing.
func supersederIsHealthy(f store.FeatureStatus) bool {
	if _, ok := supersederHealthyStates[f.State]; !ok {
		return false
	}
	if _, blocked := blockedReconcileOutcomes[f.Reconcile.Outcome]; blocked {
		return false
	}
	return true
}

// IsFeatureSuperseded reports whether `slug` is the target of a
// `supersedes` edge somewhere else in the store. When true, the
// returned superseder slug is the slug of one such superseder (per
// ADR-028 D5 there is at most one healthy candidate; when multiple
// stale-only candidates exist, the first-encountered peer is
// returned). Callers (reconcile default-set filtering, verify V7
// hard-parent closure) use this to exclude the historical target from
// the effective replay set (ADR-028 D6, PRD §4.5.3).
//
// v0.12.0 rev-1 Internal F1 (runtime flip): supersession EXCLUDES the
// historical target whether the superseder is healthy or stale. PRD
// §4.5.3 clause 3 + the docstring on composeSupersessionLabels both
// state the historical does NOT automatically re-enter the effective
// set when the superseder is stale; the flip aligns the runtime with
// the accepted PRD/ADR contract. Operators see `stale-superseder` on
// the graph as the visible signal that the replacement needs repair;
// the exclusion behavior itself matches the active-superseder case
// so drift on the historical stays warning-class per ADR-028 D8.
//
// The ORPHAN case (superseder's supersedes edge names a missing
// target) never surfaces here because the check scans peers claiming
// to supersede `slug`; if a peer exists and names `slug`, `slug`
// exists too. Orphan-superseder is a label on the SUPERSEDER (not the
// target) and does not participate in the exclusion decision.
//
// Purity: reads status.json via s.ListFeatures. Never touches
// artifacts/reconcile-session.json (ADR-010 D5). Never writes.
func IsFeatureSuperseded(s *store.Store, slug string) (string, bool) {
	all, err := s.ListFeatures()
	if err != nil {
		return "", false
	}
	return isFeatureSupersededIn(all, slug)
}

// isFeatureSupersededIn is the pure form of IsFeatureSuperseded that
// accepts a pre-loaded feature list. Used by hot loops (V7 closure
// replay, default-set filtering) to avoid repeated ListFeatures scans.
//
// v0.12.0 rev-1 Internal F1: the function returns `true` for both
// healthy AND stale superseders (see the IsFeatureSuperseded docstring
// for the PRD §4.5.3 / ADR-028 D6-D8 flip rationale). To keep the
// returned superseder slug deterministic when multiple candidates
// exist (a case ValidateDependencies now rejects at write time when >1
// is healthy, but stale peers can still stack), prefer the first
// healthy peer if any exist; otherwise fall back to the first stale
// peer encountered.
func isFeatureSupersededIn(all []store.FeatureStatus, slug string) (string, bool) {
	var firstStale string
	haveStale := false
	for _, f := range all {
		if f.Slug == slug {
			continue
		}
		for _, dep := range f.DependsOn {
			if dep.Kind != store.DependencyKindSupersedes {
				continue
			}
			if dep.Slug != slug {
				continue
			}
			if supersederIsHealthy(f) {
				return f.Slug, true
			}
			if !haveStale {
				firstStale = f.Slug
				haveStale = true
			}
		}
	}
	if haveStale {
		return firstStale, true
	}
	return "", false
}

// composeSupersessionLabels populates `set` with the four v0.12.0
// Wave α supersession overlay labels for `child`, using ADR-028 D4's
// direction-aware rules:
//
//   - Superseder side (child owns one or more `supersedes` edges):
//     `orphan-superseder` when any named target is missing from the
//     store; `active-superseder` when at least one named target
//     exists. Both may co-appear when child has mixed valid and
//     dangling edges. `stale-superseder` is added when child itself
//     is not healthy for default replay (state not in
//     {applied, active, upstream_merged} OR reconcile outcome is
//     terminally blocked).
//
//   - Target side (some OTHER feature declares
//     `supersedes -> child.slug`): `superseded-by` when the
//     superseder is healthy (default replay excludes child, and the
//     replacement is trustworthy); `stale-superseder` when the
//     superseder exists but is unhealthy (default replay still
//     excludes child per ADR-028 D6/D8 — the historical feature
//     does NOT automatically re-enter the effective set — but the
//     replacement itself needs repair).
//
// Purity: reads status.json via s.LoadFeatureStatus and
// s.ListFeatures. Never touches artifacts/reconcile-session.json
// (ADR-010 D5). Never writes.
//
// Complexity: one full store scan via ListFeatures. Same order of
// magnitude as the existing dependent/broken-refs render paths.
func composeSupersessionLabels(s *store.Store, child store.FeatureStatus, set map[store.ReconcileLabel]struct{}) {
	all, err := s.ListFeatures()
	if err != nil {
		return
	}
	index := make(map[string]store.FeatureStatus, len(all))
	for _, f := range all {
		index[f.Slug] = f
	}

	// Superseder side — child's own supersedes edges.
	childIsSuperseder := false
	for _, dep := range child.DependsOn {
		if dep.Kind != store.DependencyKindSupersedes {
			continue
		}
		childIsSuperseder = true
		if _, exists := index[dep.Slug]; !exists {
			set[store.LabelOrphanSuperseder] = struct{}{}
			continue
		}
		set[store.LabelActiveSuperseder] = struct{}{}
	}
	if childIsSuperseder && !supersederIsHealthy(child) {
		set[store.LabelStaleSuperseder] = struct{}{}
	}

	// Target side — reverse scan for peers that supersede child.
	// v0.12.0 rev-1 (F-SEXT-1, PRD §4.1 binding label-value contract):
	// the `superseded-by` label carries the healthy superseder slug as
	// a composite `superseded-by <slug>` token. When multiple healthy
	// superseders exist (a corruption path that Slice R3 rejects at
	// write time), pick the first by sorted slug for determinism. The
	// stale-path (unhealthy superseder) keeps emitting the bare
	// `stale-superseder` label — no slug suffix, matches PRD §4.3.
	var healthyPeers []string
	stalePeerSeen := false
	for _, peer := range all {
		if peer.Slug == child.Slug {
			continue
		}
		for _, dep := range peer.DependsOn {
			if dep.Kind != store.DependencyKindSupersedes {
				continue
			}
			if dep.Slug != child.Slug {
				continue
			}
			// peer supersedes child. Healthy vs stale determines
			// which label lands on the target.
			if supersederIsHealthy(peer) {
				healthyPeers = append(healthyPeers, peer.Slug)
			} else {
				stalePeerSeen = true
			}
		}
	}
	if len(healthyPeers) > 0 {
		sort.Strings(healthyPeers)
		set[store.ReconcileLabel(string(store.LabelSupersededBy)+" "+healthyPeers[0])] = struct{}{}
	}
	if stalePeerSeen {
		set[store.LabelStaleSuperseder] = struct{}{}
	}
}

// DeriveFreshnessLabel returns the single freshness label for `child`
// per the §3.4.2 truth table. Exported wrapper around
// `deriveFreshnessLabel` for render-layer callers (status / status --dag
// / status --json) that want the freshness label without the M14.3 set.
func DeriveFreshnessLabel(s *store.Store, child store.FeatureStatus) store.ReconcileLabel {
	return deriveFreshnessLabel(s, child)
}

// DeriveSupersessionLabels returns the sorted list of supersession
// overlay labels for `child` (0-4 entries). Exported wrapper around
// `composeSupersessionLabels` for render-layer callers (status /
// status --dag / status --json) that want to surface supersession
// state without inheriting the entire ComposeLabels set (which also
// carries the M14.3 hard-parent overlay + the freshness label — those
// already have their own render-time hooks).
//
// See ADR-028 D4 for the four label semantics. Purity guarantee mirrors
// `composeSupersessionLabels`: reads only status.json via
// s.LoadFeatureStatus / s.ListFeatures.
//
// v0.12.0 rev-1 (F-SEXT-2): the returned slice preserves the locked
// severity-first order from PRD §4.3:184-188 + ADR-028 D4:63-67:
//
//	[stale-superseder] [orphan-superseder] [superseded-by <slug>] [active-superseder]
//
// Callers (status text + JSON render) MUST NOT re-sort — a plain
// alphabetical sort would invert `active-superseder` and
// `stale-superseder` and violate the binding order contract.
func DeriveSupersessionLabels(s *store.Store, child store.FeatureStatus) []store.ReconcileLabel {
	set := make(map[store.ReconcileLabel]struct{})
	composeSupersessionLabels(s, child, set)
	if len(set) == 0 {
		return nil
	}
	// Severity-first emit. `superseded-by` is a composite token
	// (`superseded-by <slug>` per F-SEXT-1) so we search the set for
	// any entry starting with `superseded-by ` in addition to the
	// bare literal for defensive compatibility.
	out := make([]store.ReconcileLabel, 0, len(set))
	if _, ok := set[store.LabelStaleSuperseder]; ok {
		out = append(out, store.LabelStaleSuperseder)
	}
	if _, ok := set[store.LabelOrphanSuperseder]; ok {
		out = append(out, store.LabelOrphanSuperseder)
	}
	// superseded-by: emit the composite token if present, else the
	// bare literal (defensive — production compose always sets the
	// composite when a healthy superseder exists).
	for l := range set {
		if l == store.LabelSupersededBy || strings.HasPrefix(string(l), string(store.LabelSupersededBy)+" ") {
			out = append(out, l)
			break
		}
	}
	if _, ok := set[store.LabelActiveSuperseder]; ok {
		out = append(out, store.LabelActiveSuperseder)
	}
	return out
}

// readArtifactBytesForFreshness is the file reader used by
// deriveFreshnessLabel to recompute recipe / patch hashes. Hookable-var
// pattern: tests that need to stub the file system (e.g. simulate a
// concurrent rewrite mid-derivation) can override this. Production
// callers go through it transparently.
var readArtifactBytesForFreshness = func(s *store.Store, slug, name string) []byte {
	p := filepath.Join(s.Root, ".tpatch", "features", slug, "artifacts", name)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	return data
}

// deriveFreshnessLabel implements the §3.4.2 truth table. Exactly one
// of `LabelNeverVerified` / `LabelVerifyFailed` / `LabelVerifiedFresh` /
// `LabelVerifiedStale` is returned for every FeatureStatus.
//
// Read-only on `child.Verify`, on parent FeatureStatus, and on the
// recipe / patch file bytes for hash recomputation. NO reads of
// `reconcile-session.json` or any other artifact (D6). NO writes.
func deriveFreshnessLabel(s *store.Store, child store.FeatureStatus) store.ReconcileLabel {
	if child.Verify == nil {
		return store.LabelNeverVerified
	}
	if !child.Verify.Passed {
		return store.LabelVerifyFailed
	}

	// Verify.Passed == true — check freshness invariants.
	if !hashMatchesCurrent(s, child.Slug, "apply-recipe.json", child.Verify.RecipeHashAtVerify) {
		return store.LabelVerifiedStale
	}
	if !hashMatchesCurrent(s, child.Slug, "post-apply.patch", child.Verify.PatchHashAtVerify) {
		return store.LabelVerifiedStale
	}

	for parentSlug, snapshotState := range child.Verify.ParentSnapshot {
		ps, err := s.LoadFeatureStatus(parentSlug)
		if err != nil {
			// Parent missing now but recorded at verify time — drift.
			return store.LabelVerifiedStale
		}
		if !satisfiesStateOrBetter(ps.State, snapshotState) {
			return store.LabelVerifiedStale
		}
	}

	return store.LabelVerifiedFresh
}

// hashMatchesCurrent recomputes sha256 of the artifact at
// `.tpatch/features/<slug>/artifacts/<name>` and compares to `recorded`.
// Both-absent (recorded == "" AND file missing/empty) is a match: this
// mirrors the verify writer's behaviour, which records "" for absent
// artifacts (see workflow/verify.go sha256Hex(nil) → "").
func hashMatchesCurrent(s *store.Store, slug, name, recorded string) bool {
	current := readArtifactBytesForFreshness(s, slug, name)
	currentHash := ""
	if len(current) > 0 {
		h := sha256.Sum256(current)
		currentHash = hex.EncodeToString(h[:])
	}
	return currentHash == recorded
}

// satisfiesStateOrBetter implements the state-or-better invariant from
// §3.4.2 line 251.
//
//   - Snapshot `applied`: current `applied` or `upstream_merged` (both
//     satisfy the apply gate; the structural guarantee verify leaned on
//     is preserved). `active` is treated as `applied` per
//     appliedSatisfyingStates.
//   - Snapshot `upstream_merged`: only current `upstream_merged` is
//     acceptable (terminal-by-design; transitioning out is a
//     manual-edit anomaly).
//   - Snapshot pre-apply (`requested` / `analyzed` / `defined` /
//     `implementing`): current `applied` or `upstream_merged` accepted
//     (parent has only become more healthy).
//   - Snapshot `blocked` / `reconciling` / `reconciling-shadow`: only
//     exact match satisfies; any transition invalidates freshness.
func satisfiesStateOrBetter(current, snapshot store.FeatureState) bool {
	appliedOrMerged := current == store.StateApplied ||
		current == store.StateActive ||
		current == store.StateUpstreamMerged

	switch snapshot {
	case store.StateApplied, store.StateActive:
		return appliedOrMerged
	case store.StateUpstreamMerged:
		return current == store.StateUpstreamMerged
	case store.StateRequested, store.StateAnalyzed,
		store.StateDefined, store.StateImplementing:
		return appliedOrMerged
	case store.StateBlocked, store.StateReconciling, store.StateReconcilingShadow:
		return current == snapshot
	default:
		// Unknown snapshot state — be conservative; require exact match.
		return current == snapshot
	}
}
