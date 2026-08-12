package workflow

// Landed-feature verification — v0.15.1 Wave C / GH #8.
//
// Binding contract: ADR-013 Amendment 1 rev-7 (D8–D19),
// PRD-verify-freshness §3.6 / §4.3.6–4.3.9 / §7.1, PRD-tpatch-land §3.8.
//
// This file holds the POLICY half of the landed contract (ADR-013 D7
// keeps policy in internal/workflow); the git primitives it drives live
// in internal/gitutil/trailers.go.
//
// The shape of a run:
//
//  1. Git floor preflight (≥ 2.36). Below the floor the run reports
//     evidence `unavailable` with R10 and issues NO further git command.
//  2. Repository preflight — object format, shallowness, promisor config
//     — BEFORE any parent-count or topology classification (D16).
//  3. ONE immutable inventory over store.ListFeatureEntries(), retaining
//     unreadable rows, plus ONE `git log` enumeration. Everything later
//     consumes copies from these two.
//  4. Evidence classification for the target and every closure member.
//  5. Landed mode: anchor H (shadow at the replay anchor's parent tree)
//     for V7 / V8-historical / V10, and anchor C (index-isolated ladder
//     at HEAD) for V8-current.
//  6. Instability re-statement of the inventory before the report is
//     finalised.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// ── Vocabularies (closed sets) ───────────────────────────────────────────

// Landing-evidence states — the closed set of ten (D10). Only
// EvidenceNone degrades to forward mode; the other eight non-`exact`
// states are terminal.
const (
	EvidenceNone                = "none"
	EvidenceExact               = "exact"
	EvidenceDuplicateEquivalent = "duplicate-equivalent"
	EvidenceStale               = "stale"
	EvidenceAmbiguous           = "ambiguous"
	EvidenceMalformed           = "malformed"
	EvidenceUnsupportedTopology = "unsupported-topology"
	EvidenceShallowHistory      = "shallow-history"
	EvidenceHistoryIncomplete   = "history-incomplete"
	EvidenceUnavailable         = "unavailable"
)

// EvidenceStates is the authoritative closed set of ten (AC-L114
// companion). No other value may be emitted in
// `landing_evidence.state`.
func EvidenceStates() []string {
	return []string{
		EvidenceNone, EvidenceExact, EvidenceDuplicateEquivalent, EvidenceStale,
		EvidenceAmbiguous, EvidenceMalformed, EvidenceUnsupportedTopology,
		EvidenceShallowHistory, EvidenceHistoryIncomplete, EvidenceUnavailable,
	}
}

// `failed_at` — the closed set of thirteen (§4.3.9, AC-L114).
const (
	FailedAtParentReplay        = "parent-replay"
	FailedAtLandingEvidence     = "landing-evidence"
	FailedAtHistoricalAnchor    = "historical-anchor-unavailable"
	FailedAtRecipeProvenance    = "recipe-provenance-unavailable"
	FailedAtLandedContentAbsent = "landed-content-absent"
	FailedAtLandedArtifacts     = "landed-artifacts-absent"
	FailedAtLandedBaseline      = "landed-baseline-incoherent"
	FailedAtParentLandingDrift  = "parent-landing-drift"
	FailedAtParentEvidence      = "parent-evidence-integrity"
	FailedAtParentUnapplied     = "parent-unapplied"
	FailedAtParentRejected      = "parent-rejected"
	FailedAtSnapshotUnstable    = "snapshot-unstable"
	FailedAtInventoryUnreadable = "inventory-unreadable"
)

// FailedAtVocabulary is the closed set of thirteen (AC-L114).
func FailedAtVocabulary() []string {
	return []string{
		FailedAtParentReplay, FailedAtLandingEvidence, FailedAtHistoricalAnchor,
		FailedAtRecipeProvenance, FailedAtLandedContentAbsent, FailedAtLandedArtifacts,
		FailedAtLandedBaseline, FailedAtParentLandingDrift, FailedAtParentEvidence,
		FailedAtParentUnapplied, FailedAtParentRejected, FailedAtSnapshotUnstable,
		FailedAtInventoryUnreadable,
	}
}

// Advisory codes — the closed set of five (D12 / §4.3.9). All are
// `warn`; none flips `passed`.
const (
	AdvisoryContextDrift             = "context-drift"
	AdvisoryLaterTouch               = "later-touch"
	AdvisoryUnattributedMaterialized = "unattributed-materialized"
	AdvisoryBaseCommitUnreachable    = "base-commit-unreachable"
	AdvisoryProvenanceUnreachable    = "provenance-unreachable"
	// AdvisoryInventoryUnreadable is the §3.6.9 read-error advisory for
	// an UNRELATED feature. It is named in §3.6.9 and R-less by design
	// (the closed set of five covers the check-level advisories); it is
	// reported under the same `warn` severity.
	AdvisoryInventoryUnreadable = "inventory-unreadable"
)

// Check `mode` values (§4.3.6 field table).
const (
	ModeForward          = "forward"
	ModeHistoricalAnchor = "historical-anchor"
	ModeCurrentAnchor    = "current-anchor"
	ModeDualAnchor       = "dual-anchor"
	ModeProvenanceAnchor = "provenance-anchor"
)

// Artifact presence states (D10, closed and mutually exclusive).
const (
	PresenceAbsent          = "absent"
	PresenceEmpty           = "present-empty"
	PresenceNonEmpty        = "present-nonempty"
	RecipeShapeZeroOp       = "present-nonempty-zero-op"
	RecipeShapeWithOps      = "present-nonempty-with-ops"
	TargetModeForward       = "forward"
	TargetModeLanded        = "landed"
	BaselineModeHead        = "head-anchored"
	BaselineModeDual        = "dual-anchor"
	CurrentProbeIsolated    = "isolated-index"
	AnchorStateAvailable    = "available"
	AnchorStateUnavailable  = "unavailable"
	AnchorStateNotApplicabl = "not-applicable"
)

// Anchor-C ladder outcomes reported in `checks[].anchor_results.current`.
const (
	CurrentMaterializedClean       = "materialized-clean"
	CurrentMaterializedContextDrif = "materialized-context-drift"
	CurrentAbsent                  = "absent"
	CurrentSkipped                 = "skipped"
)

// ── Report sub-objects (schema 1.1, additive) ────────────────────────────

// VerifyRepositoryInfo is the §4.3.6 `repository` block.
type VerifyRepositoryInfo struct {
	ObjectFormat    string `json:"object_format"`
	CommitIDHexLen  int    `json:"commit_id_hex_len"`
	Shallow         bool   `json:"shallow"`
	PartialClone    bool   `json:"partial_clone"`
	GitVersion      string `json:"git_version,omitempty"`
	GitFloorSatisfd bool   `json:"git_floor_satisfied"`
}

// VerifyHistoricalAnchor is the §4.3.6 `baseline.historical_anchor`
// block. `commit` is the anchor TREE's commit — the replay anchor's
// single parent — and may differ from `replay_anchor_commit`, which may
// itself differ from `landing_evidence.attestation_commit` (D14).
type VerifyHistoricalAnchor struct {
	State               string `json:"state"`
	Commit              string `json:"commit,omitempty"`
	ReplayAnchorCommit  string `json:"replay_anchor_commit,omitempty"`
	CandidatesCollected int    `json:"candidates_collected"`
	CandidatesQualified int    `json:"candidates_qualified"`
	Reason              string `json:"reason,omitempty"`
}

// VerifyBaseline is the §4.3.6 `baseline` block.
type VerifyBaseline struct {
	Mode             string                  `json:"mode"`
	CurrentCommit    string                  `json:"current_commit,omitempty"`
	CurrentProbe     string                  `json:"current_probe,omitempty"`
	HistoricalAnchor *VerifyHistoricalAnchor `json:"historical_anchor,omitempty"`
}

// VerifyLandingEvidence is the §4.3.6 `landing_evidence` block.
//
// `State` carries one of the ten closed states. It is deliberately
// OMITTED in exactly one situation: when the D10 artifact-presence
// short-circuit fires (`landed-artifacts-absent`), classification never
// happens, so no classification state exists — emitting one would either
// invent an eleventh value or misreport one of the ten. `Reason` and the
// report's `failed_at` carry the outcome instead.
type VerifyLandingEvidence struct {
	State               string `json:"state,omitempty"`
	AttestationCommit   string `json:"attestation_commit,omitempty"`
	Candidates          int    `json:"candidates"`
	Duplicates          int    `json:"duplicates,omitempty"`
	ParentCount         int    `json:"parent_count,omitempty"`
	PatchPresence       string `json:"patch_presence,omitempty"`
	RecipePresence      string `json:"recipe_presence,omitempty"`
	PatchSHAMatch       *bool  `json:"patch_sha_match,omitempty"`
	RecipeSHAMatch      *bool  `json:"recipe_sha_match,omitempty"`
	BaseCommitMatch     *bool  `json:"base_commit_match,omitempty"`
	BaseCommitReachable *bool  `json:"base_commit_reachable,omitempty"`
	Reason              string `json:"reason,omitempty"`
}

// VerifyAdvisory is one `advisories[]` entry. Every advisory is `warn`
// severity and none flips `passed`.
type VerifyAdvisory struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Slug     string `json:"slug,omitempty"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

func warnAdvisory(code, slug, path, message string) VerifyAdvisory {
	return VerifyAdvisory{Code: code, Severity: SeverityWarn, Slug: slug, Path: path, Message: message}
}

// ── Immutable inventory (D17) ────────────────────────────────────────────

// artifactSnapshot is one captured artifact: its D10 presence state and
// its raw bytes. Consumers read the BYTES, never the disk (AC-L109).
type artifactSnapshot struct {
	Presence string
	Bytes    []byte
	// Err is a NON-absence read failure — permission denied, EIO, a
	// directory in place of the file. Rev-0 collapsed every read error
	// into `absent`, which is the same false-green class the presence
	// short-circuit exists to close (rev-1 adjudication finding 4).
	// Only `os.ErrNotExist` means absent.
	Err error
	// Path is the repo-relative artifact path, named in the block
	// diagnostic when Err is set.
	Path string
}

func snapshotArtifact(root, slug, name string, whitespaceIsEmpty bool) artifactSnapshot {
	rel := filepath.ToSlash(filepath.Join("artifacts", name))
	p := filepath.Join(root, ".tpatch", "features", slug, "artifacts", name)
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return artifactSnapshot{Presence: PresenceAbsent, Path: rel}
		}
		// Permission denied, EIO, EISDIR, … — the artifact EXISTS as far
		// as we can tell and we could not read it. Never absence.
		return artifactSnapshot{Presence: PresenceAbsent, Err: err, Path: rel}
	}
	if len(data) == 0 || (whitespaceIsEmpty && strings.TrimSpace(string(data)) == "") {
		return artifactSnapshot{Presence: PresenceEmpty, Bytes: data, Path: rel}
	}
	return artifactSnapshot{Presence: PresenceNonEmpty, Bytes: data, Path: rel}
}

// inventoryEntry is one feature's immutable capture. An entry with
// Err != nil is an explicit `unreadable` row and is NEVER omitted (D17).
type inventoryEntry struct {
	Slug        string
	Status      *store.FeatureStatus
	Err         error
	Recipe      artifactSnapshot
	Patch       artifactSnapshot
	Provenance  artifactSnapshot
	Generations []byte
	// GenerationsErr is a NON-absence read failure on
	// `patch-generations.json`. Rev-0 discarded it, so a corrupt or
	// unreadable manifest silently produced an empty touched-path set
	// and suppressed ADR-029 later-touch detection (rev-1 finding 4).
	GenerationsErr error
	TouchedPaths   []string
}

// ReadErr returns the first NON-absence artifact/metadata read failure
// captured for this feature, or nil. A non-nil result makes the feature
// `inventory-unreadable` for the target and every closure member (D17),
// and a warn advisory for an unrelated feature.
func (e *inventoryEntry) ReadErr() (string, error) {
	if e == nil {
		return "", nil
	}
	if e.Err != nil {
		return "status.json", e.Err
	}
	for _, a := range []artifactSnapshot{e.Recipe, e.Patch, e.Provenance} {
		if a.Err != nil {
			return a.Path, a.Err
		}
	}
	if e.GenerationsErr != nil {
		return "patch-generations.json", e.GenerationsErr
	}
	return "", nil
}

// RecipeShape returns the four-way recipe classification of D10.
func (e *inventoryEntry) RecipeShape() string {
	switch e.Recipe.Presence {
	case PresenceAbsent:
		return PresenceAbsent
	case PresenceEmpty:
		return PresenceEmpty
	}
	var r ApplyRecipe
	if err := json.Unmarshal(e.Recipe.Bytes, &r); err != nil {
		// Unparseable but non-empty bytes: the recipe cannot contribute
		// operations, which is the zero-op shape for arbitration
		// purposes. V2 reports the parse failure separately.
		return RecipeShapeZeroOp
	}
	if len(r.Operations) == 0 {
		return RecipeShapeZeroOp
	}
	return RecipeShapeWithOps
}

// ParsedRecipe decodes the captured recipe bytes. ok is false when the
// recipe is absent, empty or unparseable.
func (e *inventoryEntry) ParsedRecipe() (ApplyRecipe, bool) {
	if e.Recipe.Presence != PresenceNonEmpty {
		return ApplyRecipe{}, false
	}
	var r ApplyRecipe
	if err := json.Unmarshal(e.Recipe.Bytes, &r); err != nil {
		return ApplyRecipe{}, false
	}
	return r, true
}

// ExpectedRecipeSHA mirrors `readRecipeSHA` (internal/cli/land.go): the
// sha256 of the raw recipe bytes, or the literal `none` for an absent or
// whitespace-only recipe (D10 / E19).
func (e *inventoryEntry) ExpectedRecipeSHA() string {
	if e.Recipe.Presence != PresenceNonEmpty {
		return "none"
	}
	return sha256Hex(e.Recipe.Bytes)
}

// featureInventory is the whole-repository capture taken ONCE per run.
type featureInventory struct {
	Order   []string
	Entries map[string]*inventoryEntry

	// snap memoises the store-level view; the inventory is immutable
	// for the run, so one derivation is enough.
	snap *store.FeatureSnapshot
}

func (inv *featureInventory) Entry(slug string) *inventoryEntry {
	if inv == nil {
		return nil
	}
	return inv.Entries[slug]
}

// Snapshot is the store-level view of this inventory. Every downstream
// reader that used to call `LoadFeatureStatus` / `ListFeatures` takes
// this instead, so a run answers every feature question from ONE
// capture (D17, rev-1 adjudication finding 2).
func (inv *featureInventory) Snapshot() *store.FeatureSnapshot {
	if inv == nil {
		return nil
	}
	if inv.snap != nil {
		return inv.snap
	}
	snap := &store.FeatureSnapshot{
		Status: map[string]store.FeatureStatus{},
		Errs:   map[string]error{},
	}
	for _, slug := range inv.Order {
		e := inv.Entries[slug]
		snap.Order = append(snap.Order, slug)
		if e == nil {
			continue
		}
		if e.Err != nil {
			snap.Errs[slug] = e.Err
			continue
		}
		if e.Status != nil {
			snap.Status[slug] = *e.Status
		}
	}
	inv.snap = snap
	return snap
}

// Statuses is the `ListFeatures()` equivalent over the capture: every
// readable feature, slug-sorted. Unreadable entries are EXCLUDED here
// (they cannot contribute a status) but are never dropped from the
// inventory itself — the caller reports them.
func (inv *featureInventory) Statuses() []store.FeatureStatus {
	return inv.Snapshot().List()
}

// buildInventory captures every feature via store.ListFeatureEntries —
// NOT ListFeatures, which silently drops unreadable features
// (internal/store/store.go:226) and cannot represent the very
// false-green class this contract closes.
func buildInventory(s *store.Store) (*featureInventory, error) {
	entries, err := s.ListFeatureEntries()
	if err != nil {
		return nil, err
	}
	inv := &featureInventory{Entries: map[string]*inventoryEntry{}}
	for _, fe := range entries {
		ie := &inventoryEntry{Slug: fe.Slug, Err: fe.Err}
		if fe.Err == nil && fe.Status != nil {
			st := *fe.Status
			ie.Status = &st
			ie.Recipe = snapshotArtifact(s.Root, fe.Slug, "apply-recipe.json", true)
			ie.Patch = snapshotArtifact(s.Root, fe.Slug, "post-apply.patch", false)
			ie.Provenance = snapshotArtifact(s.Root, fe.Slug, "recipe-provenance.json", true)
			gen, genErr := os.ReadFile(s.PatchGenerationsPath(fe.Slug))
			if genErr != nil && !errors.Is(genErr, os.ErrNotExist) {
				ie.GenerationsErr = genErr
			}
			ie.Generations = gen
			ie.TouchedPaths = touchedPathsFromCapture(ie)
		}
		inv.Order = append(inv.Order, fe.Slug)
		inv.Entries[fe.Slug] = ie
	}
	return inv, nil
}

// touchedPathsFromCapture reproduces collectFeatureTouchedPaths
// (internal/workflow/writefile_safety.go:449-481) over CAPTURED bytes:
// the union of patch-generations touched_paths and recipe op paths.
func touchedPathsFromCapture(e *inventoryEntry) []string {
	seen := map[string]struct{}{}
	if len(e.Generations) > 0 {
		var m store.PatchGenerationsManifest
		if err := json.Unmarshal(e.Generations, &m); err != nil {
			// A manifest that exists but does not parse is a read
			// failure, not an empty touched-path set (rev-1 finding 4).
			e.GenerationsErr = err
		} else {
			for _, g := range m.Generations {
				for _, p := range g.TouchedPaths {
					seen[p] = struct{}{}
				}
			}
		}
	}
	if r, ok := e.ParsedRecipe(); ok {
		for _, op := range r.Operations {
			if op.Path != "" {
				seen[op.Path] = struct{}{}
			}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// inventoryInstability re-states the inventory and returns a non-empty
// description when a feature was added, removed, or changed — including
// a slug flipping between an `Err` row and a `Status` row (D17).
func inventoryInstability(s *store.Store, before *featureInventory) string {
	after, err := buildInventory(s)
	if err != nil {
		return fmt.Sprintf("<inventory>/features: re-statement failed: %v", err)
	}
	beforeSet := map[string]bool{}
	for _, slug := range before.Order {
		beforeSet[slug] = true
	}
	for _, slug := range after.Order {
		if !beforeSet[slug] {
			return fmt.Sprintf("%s/status.json: feature added during the run", slug)
		}
	}
	afterSet := map[string]bool{}
	for _, slug := range after.Order {
		afterSet[slug] = true
	}
	for _, slug := range before.Order {
		if !afterSet[slug] {
			return fmt.Sprintf("%s/status.json: feature removed during the run", slug)
		}
	}
	for _, slug := range before.Order {
		b := before.Entries[slug]
		a := after.Entries[slug]
		if (b.Err == nil) != (a.Err == nil) {
			return fmt.Sprintf("%s/status.json: readability changed during the run", slug)
		}
		if b.Err != nil {
			continue
		}
		if !statusEquivalent(b.Status, a.Status) {
			return fmt.Sprintf("%s/status.json: changed during the run", slug)
		}
		for _, pair := range []struct {
			name string
			b, a artifactSnapshot
		}{
			{"artifacts/apply-recipe.json", b.Recipe, a.Recipe},
			{"artifacts/post-apply.patch", b.Patch, a.Patch},
			{"artifacts/recipe-provenance.json", b.Provenance, a.Provenance},
		} {
			// rev-2 adjudication finding 5: READABILITY is part of the
			// captured state. An artifact that flips between readable and
			// unreadable keeps its presence label (`absent`) and its
			// bytes (none), so comparing only those let
			// `unreadable → absent` evade `snapshot-unstable`.
			if (pair.b.Err == nil) != (pair.a.Err == nil) {
				return fmt.Sprintf("%s/%s: readability changed during the run", slug, pair.name)
			}
			if pair.b.Presence != pair.a.Presence || !bytesEqual(pair.b.Bytes, pair.a.Bytes) {
				return fmt.Sprintf("%s/%s: changed during the run", slug, pair.name)
			}
		}
		if (b.GenerationsErr == nil) != (a.GenerationsErr == nil) {
			return fmt.Sprintf("%s/patch-generations.json: readability changed during the run", slug)
		}
		if !bytesEqual(b.Generations, a.Generations) {
			return fmt.Sprintf("%s/patch-generations.json: changed during the run", slug)
		}
	}
	return ""
}

func statusEquivalent(a, b *store.FeatureStatus) bool {
	if a == nil || b == nil {
		return a == b
	}
	ab, err1 := json.Marshal(a)
	bb, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return string(ab) == string(bb)
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ── Run context ──────────────────────────────────────────────────────────

// verifyRunContext holds the once-per-run captures shared by every
// feature of a `verify --all` run (D10/D17): the git floor probe, the
// repository preflight, the immutable inventory, the single commit
// enumeration, and the memo tables for tree/apply/identity probes.
type verifyRunContext struct {
	root string

	version    gitutil.GitVersion
	versionErr error
	floorOK    bool

	facts    gitutil.RepoFacts
	factsErr error

	head    string
	headErr error

	commits    []gitutil.CommitRecord
	commitsErr error

	inv    *featureInventory
	invErr error

	// Memo tables. Keys are deliberately explicit so a `PATH` wrapper
	// test can count invocations (AC-L27, AC-L63).
	ladderMemo   map[string]ladderOutcome
	forwardMemo  map[string]bool
	identityMemo map[string]string
	ancestorMemo map[string]bool
	anchorMemo   map[string]*anchorResolution
	evidenceMemo map[string]*landingEvidenceResult

	// pendingAdvisories / pendingMemberBaselines are the arbitration
	// pass's outputs, drained by the caller immediately after the walk.
	pendingAdvisories      []VerifyAdvisory
	pendingMemberBaselines map[string]string
}

func newVerifyRunContext(s *store.Store) *verifyRunContext {
	ctx := &verifyRunContext{
		root:         s.Root,
		ladderMemo:   map[string]ladderOutcome{},
		forwardMemo:  map[string]bool{},
		identityMemo: map[string]string{},
		ancestorMemo: map[string]bool{},
		anchorMemo:   map[string]*anchorResolution{},
		evidenceMemo: map[string]*landingEvidenceResult{},
	}

	// 1. Git floor FIRST. Below the floor no object, log, read-tree,
	//    apply or diff command may be issued (D17).
	ctx.version, ctx.versionErr = gitutil.ReadGitVersion(s.Root)
	ctx.floorOK = ctx.versionErr == nil && ctx.version.AtLeast(gitutil.GitFloorMajor, gitutil.GitFloorMinor)

	// 2. Inventory — no git process at all, so it is safe below the floor.
	ctx.inv, ctx.invErr = buildInventory(s)

	if !ctx.floorOK {
		return ctx
	}

	// 3. Repository preflight, BEFORE any topology classification (D16).
	ctx.facts, ctx.factsErr = gitutil.ReadRepoFacts(s.Root)
	// 4. The single enumeration.
	ctx.head, ctx.headErr = gitutil.HeadCommitOffline(s.Root)
	ctx.commits, ctx.commitsErr = gitutil.EnumerateCommitTrailers(s.Root)
	return ctx
}

// tempIndexDir is the home for isolated indexes. D11 allows the git dir
// or the gitignored `.tpatch/local/` root; the git dir is preferred
// because it is outside the tracked tree unconditionally, whereas the
// `.tpatch/local/` ignore rule is installed by `tpatch init` and can be
// absent in a hand-made workspace (measured E24).
func (ctx *verifyRunContext) tempIndexDir() string {
	if ctx.facts.GitDir != "" {
		return filepath.Join(ctx.facts.GitDir, "tpatch-verify")
	}
	return filepath.Join(ctx.root, ".tpatch", "local", "verify-index")
}

// readerUnavailableReason returns the R10 detail when the reader cannot
// operate at all, or "" when it can.
func (ctx *verifyRunContext) readerUnavailableReason() string {
	if ctx.versionErr != nil {
		return ctx.versionErr.Error()
	}
	if !ctx.floorOK {
		return fmt.Sprintf("git %s is below the required floor 2.36", ctx.version.String())
	}
	if ctx.factsErr != nil {
		return ctx.factsErr.Error()
	}
	if ctx.commitsErr != nil {
		return ctx.commitsErr.Error()
	}
	if ctx.headErr != nil {
		return ctx.headErr.Error()
	}
	return ""
}

// ── Evidence classification (D10) ────────────────────────────────────────

type landingEvidenceResult struct {
	Evidence VerifyLandingEvidence
	// ArtifactsAbsent marks the D10 presence short-circuit: a
	// slug-bearing candidate exists but the canonical patch is absent or
	// empty, so no digest comparison is attempted and neither `exact`
	// nor `stale` is reachable.
	ArtifactsAbsent bool
	// Candidates that are well-formed, exact-slug and single-parent.
	// Retained so arbitration and the anchor search share one pass.
	wellFormed []gitutil.CommitRecord

	// Advisories raised by classification itself — currently the
	// `base-commit-unreachable` note (D12 closed vocabulary).
	Advisories []VerifyAdvisory
}

// Landed reports whether the target/member is in landed mode.
func (r *landingEvidenceResult) Landed() bool {
	return r != nil && !r.ArtifactsAbsent &&
		(r.Evidence.State == EvidenceExact || r.Evidence.State == EvidenceDuplicateEquivalent)
}

// Terminal reports whether the evidence state is one of the eight
// terminal non-`exact` classifications.
func (r *landingEvidenceResult) Terminal() bool {
	if r == nil {
		return false
	}
	switch r.Evidence.State {
	case EvidenceStale, EvidenceAmbiguous, EvidenceMalformed,
		EvidenceUnsupportedTopology, EvidenceShallowHistory,
		EvidenceHistoryIncomplete, EvidenceUnavailable:
		return true
	}
	return false
}

func boolPtr(b bool) *bool { return &b }

// classifyEvidence implements D10 for one slug over the cached
// enumeration and the immutable inventory. Memoised per run.
func (ctx *verifyRunContext) classifyEvidence(slug string) *landingEvidenceResult {
	if cached, ok := ctx.evidenceMemo[slug]; ok {
		return cached
	}
	res := ctx.classifyEvidenceUncached(slug)
	ctx.evidenceMemo[slug] = res
	return res
}

func (ctx *verifyRunContext) classifyEvidenceUncached(slug string) *landingEvidenceResult {
	out := &landingEvidenceResult{}

	if reason := ctx.readerUnavailableReason(); reason != "" {
		out.Evidence = VerifyLandingEvidence{State: EvidenceUnavailable, Reason: reason}
		if ctx.floorOK && ctx.commitsErr != nil && gitutil.IsMissingObjectError(ctx.commitsErr.Error()) {
			out.Evidence.State = EvidenceHistoryIncomplete
		}
		return out
	}

	entry := ctx.inv.Entry(slug)
	if entry == nil || entry.Err != nil {
		out.Evidence = VerifyLandingEvidence{State: EvidenceNone}
		return out
	}

	var (
		wellFormed []gitutil.CommitRecord
		nonLinear  []gitutil.CommitRecord
		malformed  []gitutil.CommitRecord
		rawOnly    []gitutil.CommitRecord
	)

	for _, rec := range ctx.commits {
		features := rec.TrailerValues(gitutil.TrailerFeature)
		parsedHasSlug := false
		for _, v := range features {
			if v == slug {
				parsedHasSlug = true
				break
			}
		}
		rawHasSlug := gitutil.RawBodyHasTrailerLine(rec.RawBody, gitutil.TrailerFeature, slug)

		if !parsedHasSlug {
			// Conservative raw precedence (D10): a raw line that git does
			// not parse as a trailer is `malformed`, never `none`.
			if rawHasSlug {
				rawOnly = append(rawOnly, rec)
			}
			continue
		}
		// Cardinality: exactly one Tpatch-Feature value (E6).
		if len(features) != 1 {
			malformed = append(malformed, rec)
			continue
		}
		if !ctx.trailerGrammarOK(rec) {
			malformed = append(malformed, rec)
			continue
		}
		if rec.ParentCount() != 1 {
			nonLinear = append(nonLinear, rec)
			continue
		}
		wellFormed = append(wellFormed, rec)
	}

	if len(malformed) > 0 || len(rawOnly) > 0 {
		bad := append(append([]gitutil.CommitRecord{}, malformed...), rawOnly...)
		out.wellFormed = wellFormed
		out.Evidence = VerifyLandingEvidence{
			State:             EvidenceMalformed,
			AttestationCommit: bad[0].SHA,
			Candidates:        len(bad) + len(wellFormed) + len(nonLinear),
			ParentCount:       bad[0].ParentCount(),
			PatchPresence:     entry.Patch.Presence,
			RecipePresence:    entry.RecipeShape(),
			Reason:            fmt.Sprintf("commit %s carries a Tpatch-Feature line that git does not parse as a terminal trailer, or a duplicated/ill-formed Tpatch-* value", bad[0].SHA),
		}
		return out
	}

	if len(wellFormed) == 0 && len(nonLinear) == 0 {
		out.Evidence = VerifyLandingEvidence{State: EvidenceNone}
		return out
	}

	// Topology classification — only AFTER the D16 preflight facts were
	// captured, because a shallow graft boundary reports 0 parents
	// exactly like a true root (E38).
	if len(wellFormed) == 0 {
		rec := nonLinear[0]
		state := EvidenceUnsupportedTopology
		reason := fmt.Sprintf("commit %s has %d parents; tpatch land emits single-parent commits", rec.SHA, rec.ParentCount())
		if rec.ParentCount() == 0 && (ctx.facts.Shallow || ctx.facts.ShallowBoundary[rec.SHA]) {
			state = EvidenceShallowHistory
			reason = fmt.Sprintf("commit %s sits on the graft boundary of a shallow clone; its parent is not available locally", rec.SHA)
		}
		out.Evidence = VerifyLandingEvidence{
			State:             state,
			AttestationCommit: rec.SHA,
			Candidates:        len(nonLinear),
			ParentCount:       rec.ParentCount(),
			PatchPresence:     entry.Patch.Presence,
			RecipePresence:    entry.RecipeShape(),
			Reason:            reason,
		}
		return out
	}

	out.wellFormed = wellFormed

	// D10 presence short-circuit — evaluated BEFORE any digest
	// comparison, so `exact` and `stale` are reachable only from
	// `present-nonempty`.
	if entry.Patch.Presence != PresenceNonEmpty {
		out.ArtifactsAbsent = true
		out.Evidence = VerifyLandingEvidence{
			AttestationCommit: wellFormed[0].SHA,
			Candidates:        len(wellFormed) + len(nonLinear),
			ParentCount:       wellFormed[0].ParentCount(),
			PatchPresence:     entry.Patch.Presence,
			RecipePresence:    entry.RecipeShape(),
			Reason: fmt.Sprintf("post-apply.patch is %s; a landed feature cannot be attested from an absent or empty canonical patch, and no digest comparison is attempted",
				entry.Patch.Presence),
		}
		return out
	}

	// Digest comparison over the well-formed single-parent candidates.
	expectedPatchSHA := sha256Hex(entry.Patch.Bytes)
	expectedRecipeSHA := entry.ExpectedRecipeSHA()
	expectedBase := ""
	if entry.Status != nil {
		expectedBase = entry.Status.Apply.BaseCommit
	}

	var allMatch []gitutil.CommitRecord
	lastPatchMatch, lastRecipeMatch, lastBaseMatch := false, false, false
	for _, rec := range wellFormed {
		patchOK := firstValue(rec, gitutil.TrailerPatchSHA) == expectedPatchSHA
		recipeOK := firstValue(rec, gitutil.TrailerRecipeSHA) == expectedRecipeSHA
		baseOK := firstValue(rec, gitutil.TrailerBaseCommit) == expectedBase
		lastPatchMatch, lastRecipeMatch, lastBaseMatch = patchOK, recipeOK, baseOK
		if patchOK && recipeOK && baseOK {
			allMatch = append(allMatch, rec)
		}
	}

	ev := VerifyLandingEvidence{
		Candidates:     len(wellFormed) + len(nonLinear),
		PatchPresence:  entry.Patch.Presence,
		RecipePresence: entry.RecipeShape(),
	}

	switch {
	case len(allMatch) == 1:
		ev.State = EvidenceExact
		ev.AttestationCommit = allMatch[0].SHA
		ev.ParentCount = allMatch[0].ParentCount()
		ev.PatchSHAMatch = boolPtr(true)
		ev.RecipeSHAMatch = boolPtr(true)
		ev.BaseCommitMatch = boolPtr(true)
	case len(allMatch) >= 2:
		identities, err := ctx.identitiesFor(allMatch, entry.Patch.Bytes)
		if err != nil {
			ev.AttestationCommit = allMatch[0].SHA
			ev.ParentCount = allMatch[0].ParentCount()
			ev.Reason = err.Error()
			// rev-2 adjudication finding 2: an identity that could not be
			// COMPUTED is a reader failure, not two identities that
			// differ. Only a successful comparison may yield `ambiguous`
			// (and R7's "resolve the history" remediation); a missing
			// object is `history-incomplete` (R22) and any other command
			// failure is `unavailable` (R10).
			if state := classifyGitFailure(err); state != "" {
				ev.State = state
			} else {
				ev.State = EvidenceAmbiguous
			}
			break
		}
		equal := true
		for _, id := range identities[1:] {
			if id != identities[0] {
				equal = false
				break
			}
		}
		ev.AttestationCommit = allMatch[0].SHA
		ev.ParentCount = allMatch[0].ParentCount()
		ev.PatchSHAMatch = boolPtr(true)
		ev.RecipeSHAMatch = boolPtr(true)
		ev.BaseCommitMatch = boolPtr(true)
		if equal {
			ev.State = EvidenceDuplicateEquivalent
			ev.Duplicates = len(allMatch)
		} else {
			ev.State = EvidenceAmbiguous
			shas := make([]string, 0, len(allMatch))
			for _, r := range allMatch {
				shas = append(shas, r.SHA)
			}
			ev.Reason = fmt.Sprintf("%d reachable commits carry matching trailers with non-equivalent normalized changes (%s)",
				len(allMatch), strings.Join(shas, ", "))
		}
	default:
		ev.State = EvidenceStale
		ev.AttestationCommit = wellFormed[len(wellFormed)-1].SHA
		ev.ParentCount = wellFormed[len(wellFormed)-1].ParentCount()
		ev.PatchSHAMatch = boolPtr(lastPatchMatch)
		ev.RecipeSHAMatch = boolPtr(lastRecipeMatch)
		ev.BaseCommitMatch = boolPtr(lastBaseMatch)
		ev.Reason = "no reachable landing attests the current artifacts"
	}

	// Advisory-only reachability of the attested base commit. Rev-0
	// recorded the boolean but never emitted the advisory the closed
	// vocabulary requires (rev-1 adjudication finding 5).
	if ev.AttestationCommit != "" {
		base := firstValue(recordBySHA(wellFormed, ev.AttestationCommit), gitutil.TrailerBaseCommit)
		if base != "" {
			reachable := ctx.isAncestor(base, "HEAD")
			ev.BaseCommitReachable = boolPtr(reachable)
			if !reachable {
				out.Advisories = append(out.Advisories, warnAdvisory(
					AdvisoryBaseCommitUnreachable, slug, "",
					advisoryBaseCommitUnreachable(slug, ev.AttestationCommit, base)))
			}
		}
	}

	out.Evidence = ev
	return out
}

// classifyGitFailure maps a git execution failure to the evidence state
// that honestly describes it: a locally missing object is
// `history-incomplete`; anything else the reader could not complete is
// `unavailable`. A nil error, or an error that is a genuine contract
// answer (e.g. "the canonical patch declares no paths"), returns "".
func classifyGitFailure(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if gitutil.IsMissingObjectError(msg) {
		return EvidenceHistoryIncomplete
	}
	if gitutil.IsNetworkFetchError(msg) {
		return EvidenceUnavailable
	}
	if strings.Contains(msg, "git diff") || strings.Contains(msg, "git read-tree") ||
		strings.Contains(msg, "git apply") || strings.Contains(msg, "git cat-file") ||
		strings.Contains(msg, "isolated index") || strings.Contains(msg, "normalized identity") ||
		errors.Is(err, errGitBelowFloor) {
		return EvidenceUnavailable
	}
	return ""
}

func recordBySHA(recs []gitutil.CommitRecord, sha string) gitutil.CommitRecord {
	for _, r := range recs {
		if r.SHA == sha {
			return r
		}
	}
	return gitutil.CommitRecord{}
}

func firstValue(rec gitutil.CommitRecord, key string) string {
	vals := rec.TrailerValues(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// trailerGrammarOK enforces the D10 grammar: exactly one value for each
// of the three SHA trailers, 64 lowercase hex for the patch digest,
// 64 lowercase hex or the literal `none` for the recipe digest, and
// `N` lowercase hex for the base commit where N is DERIVED from
// `git rev-parse --show-object-format` (40 sha1 / 64 sha256, E41).
func (ctx *verifyRunContext) trailerGrammarOK(rec gitutil.CommitRecord) bool {
	patch := rec.TrailerValues(gitutil.TrailerPatchSHA)
	recipe := rec.TrailerValues(gitutil.TrailerRecipeSHA)
	base := rec.TrailerValues(gitutil.TrailerBaseCommit)
	if len(patch) != 1 || len(recipe) != 1 || len(base) != 1 {
		return false
	}
	if !gitutil.IsLowercaseHexOfLen(patch[0], 64) {
		return false
	}
	if recipe[0] != "none" && !gitutil.IsLowercaseHexOfLen(recipe[0], 64) {
		return false
	}
	if !gitutil.IsLowercaseHexOfLen(base[0], ctx.facts.CommitIDHexLen) {
		return false
	}
	return true
}

// identitiesFor computes the D18 normalized change identity of every
// record over the canonical patch's declared path set. An empty path set
// makes candidates incomparable (⇒ `ambiguous`).
func (ctx *verifyRunContext) identitiesFor(recs []gitutil.CommitRecord, patchBytes []byte) ([]string, error) {
	paths, err := gitutil.FilesInPatchStrict(string(patchBytes))
	if err != nil {
		return nil, fmt.Errorf("canonical patch path set could not be derived: %v", err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("canonical patch declares no paths, so candidates are not comparable")
	}
	sort.Strings(paths)
	out := make([]string, 0, len(recs))
	for _, rec := range recs {
		key := rec.SHA + "\x00" + strings.Join(paths, "\x00")
		if cached, ok := ctx.identityMemo[key]; ok {
			out = append(out, cached)
			continue
		}
		id, idErr := ctx.normalizedIdentity(rec.SHA, paths)
		if idErr != nil {
			return nil, idErr
		}
		ctx.identityMemo[key] = id
		out = append(out, id)
	}
	return out, nil
}

// ── Anchor C ladder (D12) ────────────────────────────────────────────────

type ladderOutcome struct {
	// Result is the `anchor_results.current` value.
	Result string
	// Blocked is true when step 3 of the ladder fired.
	Blocked bool
	// ContextDrift is true when step 2 passed with zero `(0/0)`.
	ContextDrift bool
	// ZeroContextHunks is the measured `(0/0)` count.
	ZeroContextHunks int
	// Path names the file the diagnostic concerns, when derivable.
	Path string
	// MissingObject is set when the probe failed because an object is
	// missing LOCALLY — the D16 `history-incomplete` signal, detectable
	// only because every command runs under GIT_NO_LAZY_FETCH=1 (E47).
	MissingObject bool
	// Err is a git-level failure (never a patch-level verdict).
	Err error
}

// runLadder probes whether the patch's postimage is materialized in
// treeish, using the D12 hardened ladder through an isolated temp index
// (D11). Memoised per `(tree, patch, reverse, context)`.
func (ctx *verifyRunContext) runLadder(treeish string, patchPath string, patchBytes []byte) ladderOutcome {
	key := treeish + "\x00" + patchPath + "\x00reverse"
	if cached, ok := ctx.ladderMemo[key]; ok {
		return cached
	}
	out := ctx.runLadderUncached(treeish, patchPath, patchBytes)
	ctx.ladderMemo[key] = out
	return out
}

func (ctx *verifyRunContext) runLadderUncached(treeish, patchPath string, patchBytes []byte) ladderOutcome {
	idx, err := ctx.newTempIndex()
	if err != nil {
		return ladderOutcome{Result: CurrentSkipped, Err: err}
	}
	defer func() { _ = idx.Close() }()

	if err := idx.ReadTree(treeish); err != nil {
		return ladderOutcome{Result: CurrentSkipped, Err: err, MissingObject: gitutil.IsMissingObjectError(err.Error())}
	}

	// Step 1 — plain reverse check.
	step1 := idx.ApplyCheck(gitutil.ApplyCheckOptions{PatchPath: patchPath, Reverse: true})
	if step1.OK {
		return ladderOutcome{Result: CurrentMaterializedClean}
	}
	if !step1.ApplyAnswered() {
		return ladderOutcome{
			Result:        CurrentSkipped,
			MissingObject: gitutil.IsMissingObjectError(step1.Stderr),
			Err:           fmt.Errorf("git apply --check --reverse --cached exited %d: %s", step1.ExitCode, strings.TrimSpace(step1.Stderr)),
		}
	}

	// Step 2 — `-C0 --verbose` under LC_ALL=C (mandatory).
	step2 := idx.ApplyCheck(gitutil.ApplyCheckOptions{
		PatchPath:    patchPath,
		Reverse:      true,
		Context:      gitutil.IntPtr(0),
		Verbose:      true,
		ForceCLocale: true,
	})
	if !step2.ApplyAnswered() {
		return ladderOutcome{
			Result:        CurrentSkipped,
			MissingObject: gitutil.IsMissingObjectError(step2.Stderr),
			Err:           fmt.Errorf("git apply --check --reverse --cached -C0 exited %d: %s", step2.ExitCode, strings.TrimSpace(step2.Stderr)),
		}
	}
	checked, zeroPaths, offsetPaths := parseApplyVerbose(step2.Stderr)
	fallbackPath := firstPatchPath(patchBytes, checked)

	if !step2.OK {
		return ladderOutcome{
			Result: CurrentAbsent, Blocked: true, Path: fallbackPath,
			MissingObject: gitutil.IsMissingObjectError(step1.Stderr) || gitutil.IsMissingObjectError(step2.Stderr),
		}
	}
	if step2.ZeroContextHunks > 0 {
		p := fallbackPath
		if len(zeroPaths) > 0 {
			p = zeroPaths[0]
		}
		return ladderOutcome{
			Result:           CurrentAbsent,
			Blocked:          true,
			ZeroContextHunks: step2.ZeroContextHunks,
			Path:             p,
		}
	}
	p := fallbackPath
	if len(offsetPaths) > 0 {
		p = offsetPaths[0]
	}
	return ladderOutcome{Result: CurrentMaterializedContextDrif, ContextDrift: true, Path: p}
}

// parseApplyVerbose extracts the paths git reported on, the paths where
// a hunk needed all context discarded, and the paths where a hunk moved.
func parseApplyVerbose(stderr string) (checked, zeroContext, offset []string) {
	current := ""
	for _, line := range strings.Split(stderr, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Checking patch ") {
			current = strings.TrimSuffix(strings.TrimPrefix(trimmed, "Checking patch "), "...")
			current = strings.TrimSpace(strings.TrimSuffix(current, "..."))
			if current != "" {
				checked = append(checked, current)
			}
			continue
		}
		if strings.Contains(trimmed, "Context reduced to (0/0)") && current != "" {
			zeroContext = append(zeroContext, current)
			continue
		}
		if strings.Contains(trimmed, "offset ") && current != "" {
			offset = append(offset, current)
		}
	}
	return checked, zeroContext, offset
}

func firstPatchPath(patchBytes []byte, checked []string) string {
	if paths, err := gitutil.FilesInPatchStrict(string(patchBytes)); err == nil && len(paths) > 0 {
		sorted := append([]string{}, paths...)
		sort.Strings(sorted)
		return sorted[0]
	}
	if len(checked) > 0 {
		return checked[0]
	}
	return ""
}

// ── Anchor collection, forward qualification, selection (D14) ────────────

type anchorResolution struct {
	Available  bool
	Anchor     string // the anchor tree's commit — the replay anchor's single parent
	ReplayFrom string // the selected replay anchor commit
	Collected  int
	Qualified  int
	Reason     string

	// FailState classifies a git-level failure during collection,
	// qualification or identity comparison. Rev-0 swallowed every such
	// error into "no candidate qualified" / "ambiguous", which reports a
	// history problem as a contract violation (rev-1 adjudication
	// finding 3). "" means the resolution failed on its own terms —
	// genuinely no qualifier, or genuinely non-equivalent qualifiers.
	FailState  string // "" | EvidenceHistoryIncomplete | EvidenceUnavailable
	FailDetail string
}

// resolveAnchor runs D14 steps 1–4 for slug. Memoised per run so a
// landed closure member and the target share one resolution.
func (ctx *verifyRunContext) resolveAnchor(slug string) *anchorResolution {
	if cached, ok := ctx.anchorMemo[slug]; ok {
		return cached
	}
	res := ctx.resolveAnchorUncached(slug)
	ctx.anchorMemo[slug] = res
	return res
}

func (ctx *verifyRunContext) resolveAnchorUncached(slug string) *anchorResolution {
	entry := ctx.inv.Entry(slug)
	if entry == nil || entry.Err != nil || entry.Patch.Presence != PresenceNonEmpty {
		return &anchorResolution{Reason: "no canonical post-apply.patch to qualify a candidate against"}
	}
	patchPath := filepath.Join(ctx.root, ".tpatch", "features", slug, "artifacts", "post-apply.patch")

	// 1. Collect EVERY reachable single-parent exact-slug landing. No
	//    stop-at-first: a second, non-equivalent qualifier can only be
	//    observed by collecting all of them.
	var collected []gitutil.CommitRecord
	for _, rec := range ctx.commits {
		features := rec.TrailerValues(gitutil.TrailerFeature)
		if len(features) != 1 || features[0] != slug {
			continue
		}
		if rec.ParentCount() != 1 {
			continue
		}
		collected = append(collected, rec)
	}
	res := &anchorResolution{Collected: len(collected)}
	if len(collected) == 0 {
		res.Reason = "no reachable single-parent landing commit carries this feature's trailer"
		return res
	}

	// 2. Qualify by FORWARD apply at -C1 against `C^`.
	var qualified []gitutil.CommitRecord
	for _, rec := range collected {
		ok, failState, qErr := ctx.forwardQualifies(rec.SHA, patchPath)
		if qErr != nil {
			res.FailState = failState
			res.FailDetail = qErr.Error()
			res.Reason = fmt.Sprintf("candidate %s could not be qualified: %v", rec.SHA, qErr)
			return res
		}
		if ok {
			qualified = append(qualified, rec)
		}
	}
	res.Qualified = len(qualified)
	if len(qualified) == 0 {
		res.Reason = "no collected candidate's parent tree accepts a forward apply of the current canonical patch"
		return res
	}

	// 3. Compare normalized identities when more than one qualifies.
	if len(qualified) > 1 {
		identities, err := ctx.identitiesFor(qualified, entry.Patch.Bytes)
		if err != nil {
			// An identity that could not be COMPUTED is not the same as
			// two identities that differ (rev-1 finding 3).
			if state := classifyGitFailure(err); state != "" {
				res.FailState = state
				res.FailDetail = err.Error()
			}
			res.Reason = err.Error()
			return res
		}
		for _, id := range identities[1:] {
			if id != identities[0] {
				res.Reason = "the qualifying candidates describe different changes"
				return res
			}
		}
	}

	// 4. Select: first in the enumeration's oldest-first order; final
	//    tie-break, the lexicographically smallest commit id.
	best := qualified[0]
	for _, rec := range qualified[1:] {
		if rec.SHA < best.SHA && sameEnumerationPosition(ctx.commits, rec.SHA, best.SHA) {
			best = rec
		}
	}
	res.Available = true
	res.ReplayFrom = best.SHA
	res.Anchor = best.Parents[0]
	return res
}

// sameEnumerationPosition is the guard on D14's final tie-break: the
// lexicographic rule only applies between records that the enumeration
// could not order, which cannot happen for distinct commits. It is kept
// explicit so the selection rule reads exactly as specified.
func sameEnumerationPosition(commits []gitutil.CommitRecord, a, b string) bool {
	ia, ib := -1, -1
	for i, rec := range commits {
		if rec.SHA == a {
			ia = i
		}
		if rec.SHA == b {
			ib = i
		}
	}
	return ia == ib
}

// forwardQualifies answers D14 step 2: seed a temp index from `C^` and
// run a FORWARD `git apply --check --cached -C1`. Never `--reverse`;
// never the invalid `C^{tree}^` revision (E43).
//
// A git-level FAILURE is returned separately from "did not qualify"
// (rev-1 adjudication finding 3). A tree or blob that is missing
// locally is `history-incomplete`; any other execution failure is
// `unavailable`. Neither is ever reported as "no candidate qualified".
func (ctx *verifyRunContext) forwardQualifies(commit, patchPath string) (bool, string, error) {
	key := commit + "^\x00" + patchPath + "\x00forward-C1"
	if v, ok := ctx.forwardMemo[key]; ok {
		return v, "", nil
	}
	idx, err := ctx.newTempIndex()
	if err != nil {
		return false, EvidenceUnavailable, fmt.Errorf("isolated index for %s^: %w", commit, err)
	}
	defer func() { _ = idx.Close() }()

	if rtErr := idx.ReadTree(commit + "^"); rtErr != nil {
		if gitutil.IsMissingObjectError(rtErr.Error()) {
			return false, EvidenceHistoryIncomplete, rtErr
		}
		// A candidate whose parent tree cannot be read at all is not a
		// silent non-qualifier: surface it.
		return false, EvidenceUnavailable, rtErr
	}
	res := idx.ApplyCheck(gitutil.ApplyCheckOptions{
		PatchPath: patchPath,
		Context:   gitutil.IntPtr(1),
	})
	if !res.ApplyAnswered() {
		// git could not carry the probe out: never a silent
		// non-qualification (rev-1 finding 3).
		state := EvidenceUnavailable
		if gitutil.IsMissingObjectError(res.Stderr) {
			state = EvidenceHistoryIncomplete
		}
		return false, state, fmt.Errorf("git apply --check --cached -C1 at %s^ exited %d: %s",
			commit, res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	if !res.OK && gitutil.IsMissingObjectError(res.Stderr) {
		return false, EvidenceHistoryIncomplete, fmt.Errorf("git apply --check --cached -C1 at %s^: %s", commit, strings.TrimSpace(res.Stderr))
	}
	if !res.OK && gitutil.IsNetworkFetchError(res.Stderr) {
		return false, EvidenceUnavailable, fmt.Errorf("git apply --check --cached -C1 at %s^ attempted the network: %s", commit, strings.TrimSpace(res.Stderr))
	}
	ctx.forwardMemo[key] = res.OK
	return res.OK, "", nil
}

// ── Provenance (D15) ─────────────────────────────────────────────────────

type provenanceResolution struct {
	OK         bool
	BaseCommit string
	HashBound  bool
	Reason     string // "absent" | "malformed" | "unreachable"
}

// resolveProvenance implements the four-condition test of D15 over the
// captured `artifacts/recipe-provenance.json` bytes.
func (ctx *verifyRunContext) resolveProvenance(entry *inventoryEntry) provenanceResolution {
	if entry == nil || entry.Provenance.Presence != PresenceNonEmpty {
		return provenanceResolution{Reason: "absent"}
	}
	var prov RecipeProvenance
	if err := json.Unmarshal(entry.Provenance.Bytes, &prov); err != nil {
		return provenanceResolution{Reason: "malformed"}
	}
	if !gitutil.IsLowercaseHexOfLen(prov.BaseCommit, ctx.facts.CommitIDHexLen) {
		return provenanceResolution{Reason: "malformed"}
	}
	reachable := prov.BaseCommit == ctx.head || ctx.isAncestor(prov.BaseCommit, "HEAD")
	if !reachable {
		return provenanceResolution{Reason: "unreachable", BaseCommit: prov.BaseCommit}
	}
	res := provenanceResolution{OK: true, BaseCommit: prov.BaseCommit}
	if prov.RecipeSHA256 != nil {
		// Inventory-consistency: the sidecar must describe the captured
		// recipe bytes. A mismatch is inventory-inconsistent, never
		// silently trusted.
		if *prov.RecipeSHA256 != sha256Hex(entry.Recipe.Bytes) {
			return provenanceResolution{Reason: "malformed", BaseCommit: prov.BaseCommit}
		}
		res.HashBound = true
	}
	return res
}

// ── Later-touch index over the inventory (D15) ───────────────────────────

// laterTouchIndexFromInventory reproduces loadLaterFeatureTouches
// (internal/workflow/writefile_safety.go:409-442) over the immutable
// inventory: `RequestedAt` ordering, touched-path union, first-later-slug
// in slug order. Unreadable entries are excluded — and, unlike the
// shipped detector, that exclusion is REPORTED by the caller.
func laterTouchIndexFromInventory(inv *featureInventory, currentSlug string) map[string]string {
	cur := inv.Entry(currentSlug)
	if cur == nil || cur.Status == nil || cur.Status.RequestedAt == "" {
		return nil
	}
	idx := map[string]string{}
	for _, slug := range inv.Order {
		if slug == currentSlug {
			continue
		}
		e := inv.Entries[slug]
		if e == nil || e.Err != nil || e.Status == nil {
			continue
		}
		if _, readErr := e.ReadErr(); readErr != nil {
			// Excluded from ADR-029 ordering — and REPORTED as an
			// advisory rather than silently skipped (D17).
			continue
		}
		if e.Status.RequestedAt == "" || e.Status.RequestedAt <= cur.Status.RequestedAt {
			continue
		}
		for _, p := range e.TouchedPaths {
			if _, seen := idx[p]; seen {
				continue
			}
			idx[p] = slug
		}
	}
	if len(idx) == 0 {
		return nil
	}
	return idx
}

// ── Remediation strings (§3.6.9 R1–R22, R24) ─────────────────────────────
//
// Emitted VERBATIM. Every template below is transcribed from the PRD
// table; AC-L117 pins them as golden strings.

func remediationR1(sha string) string {
	return fmt.Sprintf("landed feature: post-apply.patch postimage is not present at HEAD; landing commit %s is reachable but the content is absent — inspect with git diff %s HEAD, then re-record and re-land. Do NOT run tpatch reconcile: this is local drift, not upstream drift", sha, sha)
}

func remediationR2(sha, path string) string {
	return fmt.Sprintf("landed feature: post-apply.patch matched at HEAD only with all context discarded at %s; verify refuses to certify an unanchored match — inspect with git diff %s HEAD -- %s, then re-record so the captured context matches HEAD and re-land", path, sha, path)
}

func remediationR3(sha, path string) string {
	return fmt.Sprintf("landed feature: post-apply.patch content is present at HEAD but its recorded context has drifted at %s; a later change touched the surrounding lines — inspect with git diff %s HEAD -- %s and re-record if the feature should absorb it", path, sha, path)
}

func remediationR4(opIndex int, baseline string, err error, slug string) string {
	return fmt.Sprintf("landed feature: recipe op #%d failed to replay at the landing baseline %s: %v; the recipe no longer describes the tree it was authored against — re-run tpatch record %s --regenerate-recipe and re-land", opIndex, baseline, err, slug)
}

func remediationR5(baseline string) string {
	return fmt.Sprintf("landed feature: post-apply.patch does not apply at the landing baseline %s; the patch and the landing attestation disagree — re-record and re-land", baseline)
}

func remediationR6(slug, sha, patchSHA, recipeSHA, base string) string {
	return fmt.Sprintf("landing evidence for %s is stale: commit %s attests patch-sha=%s / recipe-sha=%s / base=%s but the current artifacts hash differently; re-run tpatch land %s to re-attest, or restore the attested artifacts", slug, sha, patchSHA, recipeSHA, base, slug)
}

func remediationR7(slug string, n int, shas []string) string {
	return fmt.Sprintf("landing evidence for %s is ambiguous: %d reachable commits carry matching trailers with non-equivalent normalized changes (%s); resolve the history or re-land so exactly one attestation is current", slug, n, strings.Join(shas, ", "))
}

func remediationR8(slug, sha string) string {
	return fmt.Sprintf("landing evidence for %s is malformed: commit %s carries a Tpatch-Feature line that Git does not parse as a trailer, or a duplicated/ill-formed Tpatch-* value; restore the four-trailer block with git commit --amend, or re-land", slug, sha)
}

func remediationR9(slug, sha string, parents int) string {
	return fmt.Sprintf("landing evidence for %s is unusable: commit %s has %d parents and tpatch land emits single-parent commits; verify cannot derive a landing baseline from a root or merge commit — re-land %s on a linear commit", slug, sha, parents, slug)
}

func remediationR10(slug string, err string) string {
	return fmt.Sprintf("landing evidence for %s could not be read: %s; verify requires git >= 2.36 (trailer enumeration >= 2.22/2.25, object-format probe >= 2.29, and GIT_NO_LAZY_FETCH >= 2.36 for offline object access) and refuses to guess — upgrade git to 2.36 or newer, or report this environment", slug, err)
}

func remediationR11(slug string) string {
	return fmt.Sprintf("landed feature %s has no usable landing baseline: no reachable single-parent landing commit has a parent that the current canonical patch applies to, or the qualifying candidates describe different changes; verify will not certify a landed feature it cannot replay — re-run tpatch record %s and tpatch land %s to create a fresh single-parent landing", slug, slug, slug)
}

func remediationR12(opIndex int, path, expected, baseline, observed, slug string) string {
	return fmt.Sprintf("recipe op #%d %s expected preimage %s at baseline %s but observed %s; the recipe is stale against its own baseline — re-run tpatch record %s --regenerate-recipe and re-land", opIndex, path, expected, baseline, observed, slug)
}

func remediationR13(laterSlug, path, slug string) string {
	return fmt.Sprintf("later-touch: later feature %s touched %s after %s was recorded; replaying this write-file would silently revert it — review before any replay (ADR-029 D5/D6, warning-class)", laterSlug, path, slug)
}

func remediationR14(parent, sha string) string {
	return fmt.Sprintf("hard parent %s landed at %s but its canonical patch is not present at the verification baseline; verify %s first — do not re-apply it into the shadow", parent, sha, parent)
}

func remediationR15(parent, state, target string) string {
	return fmt.Sprintf("hard parent %s has %s landing evidence; verify %s first — replaying or skipping it would validate %s against an unknown baseline", parent, state, parent, target)
}

func remediationR16(parent, target string) string {
	return fmt.Sprintf("hard parent %s is unapplied; its patch is deliberately absent from the tree — run tpatch apply %s before verifying %s", parent, parent, target)
}

func remediationR17(parent, target string) string {
	return fmt.Sprintf("hard parent %s is rejected (terminal); remove the hard dependency with tpatch amend %s --remove-depends-on %s, or reopen %s", parent, target, parent, parent)
}

func remediationR18(parent string) string {
	return fmt.Sprintf("unattributed-materialized: hard parent %s is not landed but its canonical patch is already present at the verification baseline; it was not replayed, and verify makes no claim about what produced it", parent)
}

func remediationR19(slug string) string {
	return fmt.Sprintf("landed feature %s has no usable apply-recipe.json or post-apply.patch; materialization cannot be proven from an absent or empty artifact set — re-run tpatch record %s", slug, slug)
}

func remediationR20(changedSlug, changedPath, targetSlug string) string {
	return fmt.Sprintf("verify aborted: %s/%s changed while verify was running; re-run tpatch verify %s with no concurrent tpatch or editor writes", changedSlug, changedPath, targetSlug)
}

func remediationR21(slug, sha string) string {
	return fmt.Sprintf("landing evidence for %s is incomplete: this is a shallow clone and commit %s sits on the graft boundary, so its parent is not available locally — run git fetch --unshallow (or increase --depth) and re-run verify", slug, sha)
}

func remediationR22(slug string) string {
	return fmt.Sprintf("landing evidence for %s could not be completed: an object required to read the landing baseline is missing from this partial clone — restore network access to the promisor remote, or run git fetch --refetch, and re-run verify", slug)
}

// advisoryBaseCommitUnreachable is the D12 `base-commit-unreachable`
// warn advisory. It is emitted whenever a landing attests a well-formed
// base commit that is NOT reachable from HEAD — the measured
// cherry-pick / rebase case — and it never fails the run on its own.
func advisoryBaseCommitUnreachable(slug, attestation, base string) string {
	return fmt.Sprintf("base-commit-unreachable: landing %s for %s attests base commit %s, which is not reachable from HEAD; the landing was most likely rebased or cherry-picked — this is advisory only and does not affect the verdict", attestation, slug, base)
}

func remediationR24(opIndex int, path, condition, slug string) string {
	return fmt.Sprintf("recipe op #%d %s carries a preimage_hash but artifacts/recipe-provenance.json is %s; verify will not evaluate a preimage against the live working tree — re-run tpatch implement %s to regenerate the recipe and its provenance", opIndex, path, condition, slug)
}

// evidenceRemediation maps a terminal evidence state to its exact
// remediation string.
func evidenceRemediation(slug string, ev VerifyLandingEvidence, entry *inventoryEntry, attested gitutil.CommitRecord, duplicateSHAs []string) string {
	switch ev.State {
	case EvidenceStale:
		return remediationR6(slug, ev.AttestationCommit,
			firstValue(attested, gitutil.TrailerPatchSHA),
			firstValue(attested, gitutil.TrailerRecipeSHA),
			firstValue(attested, gitutil.TrailerBaseCommit))
	case EvidenceAmbiguous:
		return remediationR7(slug, len(duplicateSHAs), duplicateSHAs)
	case EvidenceMalformed:
		return remediationR8(slug, ev.AttestationCommit)
	case EvidenceUnsupportedTopology:
		return remediationR9(slug, ev.AttestationCommit, ev.ParentCount)
	case EvidenceShallowHistory:
		return remediationR21(slug, ev.AttestationCommit)
	case EvidenceHistoryIncomplete:
		return remediationR22(slug)
	case EvidenceUnavailable:
		return remediationR10(slug, ev.Reason)
	}
	return ""
}

// ── Run-context helpers used by the pipeline ─────────────────────────────

// repositoryInfo renders the §4.3.6 `repository` block.
func (ctx *verifyRunContext) repositoryInfo() *VerifyRepositoryInfo {
	return &VerifyRepositoryInfo{
		ObjectFormat:    ctx.facts.ObjectFormat,
		CommitIDHexLen:  ctx.facts.CommitIDHexLen,
		Shallow:         ctx.facts.Shallow,
		PartialClone:    ctx.facts.PartialClone,
		GitVersion:      ctx.version.Raw,
		GitFloorSatisfd: ctx.floorOK,
	}
}

// inventoryAdvisories emits one `warn` per UNRELATED unreadable feature
// (D17). The target and closure members are handled as blocks by the
// dynamic phase; this surface exists so an exclusion from ADR-029
// later-touch ordering is REPORTED rather than invisible.
func (ctx *verifyRunContext) inventoryAdvisories(targetSlug string) []VerifyAdvisory {
	if ctx.inv == nil {
		return nil
	}
	var out []VerifyAdvisory
	for _, slug := range ctx.inv.Order {
		e := ctx.inv.Entries[slug]
		if e == nil || slug == targetSlug {
			continue
		}
		path, readErr := e.ReadErr()
		if readErr == nil {
			continue
		}
		out = append(out, warnAdvisory(AdvisoryInventoryUnreadable, slug, path,
			fmt.Sprintf("inventory-unreadable: feature %s could not be read at %s (%v); it is excluded from ADR-029 later-touch ordering for this run", slug, path, readErr)))
	}
	return out
}

// refreshAfterOwnWrite folds verify's OWN freshness write into the
// capture. Rev-0 re-read `status.json` from disk here, which is a
// persistence reload the immutable-inventory contract forbids (rev-1
// adjudication finding 2). The value written is already known, so the
// capture is updated in memory and stays byte-consistent with disk.
func (ctx *verifyRunContext) refreshAfterOwnWrite(slug string, persisted store.FeatureStatus) {
	e := ctx.inv.Entry(slug)
	if e == nil || e.Err != nil || e.Status == nil {
		return
	}
	updated := persisted
	e.Status = &updated
	if ctx.inv.snap != nil {
		ctx.inv.snap.SetStatus(slug, updated)
	}
}

// inventoryEntryOrEmpty returns the captured entry for slug, or an empty
// entry so downstream code never nil-checks.
func inventoryEntryOrEmpty(ctx *verifyRunContext, slug string) *inventoryEntry {
	if e := ctx.inv.Entry(slug); e != nil {
		return e
	}
	return &inventoryEntry{Slug: slug, Recipe: artifactSnapshot{Presence: PresenceAbsent},
		Patch: artifactSnapshot{Presence: PresenceAbsent}, Provenance: artifactSnapshot{Presence: PresenceAbsent}}
}

// inventoryPatchBytes returns the CAPTURED canonical-patch bytes. There
// is no live-read fallback: a feature that is not in the capture cannot
// contribute a hash, and the instability re-statement fails the run
// (rev-2 finding 1).
func inventoryPatchBytes(ctx *verifyRunContext, slug string) []byte {
	if e := ctx.inv.Entry(slug); e != nil && e.Err == nil {
		return e.Patch.Bytes
	}
	return nil
}

// markSnapshotUnstable rewrites the dynamic rows as failed with R20 and
// sets `failed_at: snapshot-unstable` (D17).
func markSnapshotUnstable(report *VerifyReport, slug, detail string) {
	parts := strings.SplitN(detail, ":", 2)
	loc := strings.SplitN(parts[0], "/", 2)
	changedSlug, changedPath := slug, parts[0]
	if len(loc) == 2 {
		changedSlug, changedPath = loc[0], loc[1]
	}
	rem := remediationR20(changedSlug, changedPath, slug)
	for i := range report.Checks {
		switch report.Checks[i].ID {
		case CheckRecipeReplayClean, CheckPostApplyPatchReplayClean, CheckWriteFilePreimageFresh:
			report.Checks[i].Passed = false
			report.Checks[i].Skipped = false
			report.Checks[i].Reason = ""
			report.Checks[i].Remediation = rem
		}
	}
	report.FailedAt = FailedAtSnapshotUnstable
}

// inventoryEntries re-exposes the ONE immutable capture in the
// `store.FeatureEntry` shape `verify --all` already consumes, so the
// aggregate walks the same inventory as every per-feature run instead of
// re-reading the store (rev-1 adjudication finding 2).
func (ctx *verifyRunContext) inventoryEntries() ([]store.FeatureEntry, error) {
	if ctx.invErr != nil {
		return nil, ctx.invErr
	}
	if ctx.inv == nil {
		return nil, nil
	}
	out := make([]store.FeatureEntry, 0, len(ctx.inv.Order))
	for _, slug := range ctx.inv.Order {
		e := ctx.inv.Entries[slug]
		if e == nil {
			continue
		}
		entry := store.FeatureEntry{Slug: slug, Err: e.Err}
		if e.Err == nil && e.Status != nil {
			st := *e.Status
			entry.Status = &st
		}
		out = append(out, entry)
	}
	return out, nil
}
