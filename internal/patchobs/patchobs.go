// Package patchobs holds the immutable observation a governed producer
// captures before its first bound write (GH #15 / ADR-036 D2,
// PRD-recipe-generation-authority §6.2).
//
// The rule this package exists to make implementable:
//
//	Before a governed producer writes or checkpoints a bound artifact, it
//	captures the canonical patch bytes it is about to bind, the reference
//	descriptor, the capture descriptor, and the ordered pre/postimage
//	observations for every effect it will describe. Coverage is derived
//	from that immutable observation and from nothing re-read afterwards.
//
// Two properties are load-bearing and are enforced by the types rather
// than by convention:
//
//   - an observation records whether each side was LOOKED AT, not only
//     what was found. A producer that could not read a side records
//     `observed: false` with its availability reason, never the same
//     `false` a producer uses to prove a path absent;
//   - an extant side — the postimage of an add/modify/rename/copy, the
//     preimage of a modify/rename/copy/delete — is never recorded as
//     observed-and-absent. That shape contradicts the change kind the
//     same record carries, so it is demoted to unobserved with its
//     availability reason instead.
//
// Nothing here writes to disk and nothing here changes public output.
// Source bodies stay in memory within a per-observation budget (ADR-038).
// The Recorder seam is the single hand-off point a later slice replaces
// with the shared publication step; until
// then the default recorder discards, so wiring a producer is observable
// only through the seam.
//
// The observation runs BEFORE a producer's first bound write, so its cost
// is on the critical path of every record, amend, accept, cycle and apply.
// Every Git read is therefore batched per REFERENCE rather than per
// effect — see gitread.go — and working-tree postimages are read straight
// from the filesystem with no subprocess at all.
package patchobs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/safety"
)

// ProducerID names the governed producer that took an observation. It is
// diagnostic context, never authority.
//
// The values are the CLOSED vocabulary of ADR-036's producer registry
// (D15, §6.15 inventory table). They are wire-visible identifiers a later
// slice publishes verbatim, so they are spelled exactly as the registry
// spells them — `feature-patch-amend`, not `feature-patch`; `artifact-edit`,
// not `edit`.
type ProducerID string

const (
	ProducerRecord          ProducerID = "record"              // P1
	ProducerFeaturePatch    ProducerID = "feature-patch-amend" // P2
	ProducerReconcileAccept ProducerID = "reconcile-accept"    // P3
	ProducerCycle           ProducerID = "cycle"               // P4
	ProducerApplyDone       ProducerID = "apply-done"          // P5
	ProducerImplement       ProducerID = "implement"           // P6
	ProducerEdit            ProducerID = "artifact-edit"       // P7
)

// KnownProducer reports whether id is one of the seven registered
// producers. An unregistered producer is a bug, not a default.
func KnownProducer(id ProducerID) bool {
	switch id {
	case ProducerRecord, ProducerFeaturePatch, ProducerReconcileAccept,
		ProducerCycle, ProducerApplyDone, ProducerImplement, ProducerEdit:
		return true
	}
	return false
}

// ReferenceKind is the typed reference descriptor of ADR-036 D2.
type ReferenceKind string

const (
	// ReferenceKindCommit means a resolved commit supplied the preimage.
	ReferenceKindCommit ReferenceKind = "commit"
	// ReferenceKindIndexSnapshot means the preimage came from a captured
	// index state, which is not durable unless another committed
	// authority binds it. No shipped record mode emits it; it exists for
	// future or internal callers whose preimage genuinely is an
	// uncommitted index state.
	ReferenceKindIndexSnapshot ReferenceKind = "index-snapshot"
	// ReferenceKindUnavailable means no trustworthy preimage was captured.
	ReferenceKindUnavailable ReferenceKind = "unavailable"
)

// ReferenceDescriptor identifies what supplied the preimage.
type ReferenceDescriptor struct {
	Kind ReferenceKind
	// Commit is a 40-character lowercase hex commit for
	// ReferenceKindCommit and "" for every other kind.
	Commit string
	// PreimageSetSHA256 binds the ordered path/kind/mode/existence/hash
	// observation set. It proves observation identity, not durable
	// reconstructability.
	PreimageSetSHA256 string
}

// CaptureMode is the capture a producer ran, in the vocabulary
// `patch-generations.json` already uses.
type CaptureMode string

const (
	CaptureModeWorkingTreeAll         CaptureMode = "working-tree-all"
	CaptureModeStagedIndex            CaptureMode = "staged-index"
	CaptureModeUnstagedWorktree       CaptureMode = "unstaged-worktree"
	CaptureModeCommittedRange         CaptureMode = "committed-range"
	CaptureModeAutoCommittedRange     CaptureMode = "auto-committed-range"
	CaptureModeExplicitCommittedRange CaptureMode = "explicit-committed-range"
	CaptureModeReconcile              CaptureMode = "reconcile"
	// CaptureModeNoCapture is the mode of the two producers that write or
	// checkpoint a bound artifact without running a patch capture at all:
	// `implement` (P6) and `tpatch edit` (P7). It carries empty pathspecs
	// and claim IDs, and it licenses nothing.
	CaptureModeNoCapture CaptureMode = "no-capture"
)

// KnownCaptureMode reports whether mode is one this package maps to a
// reference kind. An unmapped mode is a bug, not a default.
func KnownCaptureMode(mode CaptureMode) bool {
	switch mode {
	case CaptureModeWorkingTreeAll, CaptureModeStagedIndex, CaptureModeUnstagedWorktree,
		CaptureModeCommittedRange, CaptureModeAutoCommittedRange, CaptureModeExplicitCommittedRange,
		CaptureModeReconcile, CaptureModeNoCapture:
		return true
	}
	return false
}

// ReferenceKindForMode is the ADR-036 D2 capture-mode map. Every shipped
// record capture mode maps to `commit` or `unavailable`; none maps to
// `index-snapshot`.
//
// `unstaged-worktree` is commit-kind HEAD, not an index snapshot: record
// refuses a capture whose staged and unstaged edits touch the same path
// before any capture runs, so every path that reaches an accepted
// `--unstaged` patch has an index entry identical to HEAD.
func ReferenceKindForMode(mode CaptureMode) ReferenceKind {
	switch mode {
	case CaptureModeWorkingTreeAll, CaptureModeStagedIndex, CaptureModeUnstagedWorktree,
		CaptureModeCommittedRange, CaptureModeAutoCommittedRange, CaptureModeExplicitCommittedRange,
		CaptureModeReconcile:
		return ReferenceKindCommit
	default:
		return ReferenceKindUnavailable
	}
}

// CaptureDescriptor records what the producer captured.
type CaptureDescriptor struct {
	Mode      CaptureMode
	Pathspecs []string
	ClaimIDs  []string
}

// normalized returns the descriptor with its slices non-nil and sorted,
// and with the no-capture invariant applied: a no-capture record carries
// empty pathspecs and claim IDs.
func (c CaptureDescriptor) normalized() CaptureDescriptor {
	out := CaptureDescriptor{
		Mode:      c.Mode,
		Pathspecs: sortedCopy(c.Pathspecs),
		ClaimIDs:  sortedCopy(c.ClaimIDs),
	}
	if out.Mode == CaptureModeNoCapture {
		out.Pathspecs = []string{}
		out.ClaimIDs = []string{}
	}
	return out
}

// Record-level reasons. The wire vocabulary is owned by a later slice;
// these are the in-memory subset S1 can establish truthfully.
const (
	ReasonPatchMissing        = "canonical-patch-missing"
	ReasonPatchEmpty          = "canonical-patch-empty"
	ReasonPatchUnparseable    = "canonical-patch-unparseable"
	ReasonReferenceNotDurable = "reference-not-durable"
)

// Effect-local reasons.
const (
	ReasonPreimageUnavailable  = "preimage-unavailable"
	ReasonPostimageUnavailable = "postimage-unavailable"
	// ReasonPreimageModeContradiction and its postimage twin separate the
	// two ways a side ends up unestablished. Ordinary unavailability means
	// the producer could not READ the side; a contradiction means it read
	// the side and found an object whose mode the patch header says it is
	// not. Collapsing the second into the first would report a
	// wrong-object observation as an IO problem, and an operator reading
	// only `*-unavailable` would look for a missing file that is right
	// there.
	//
	// Both are S1 in-memory reason codes. The published wire vocabulary is
	// a closed set a later slice owns; nothing here invents an entry in
	// it, and the pair below is mapped at publication time rather than
	// emitted raw.
	ReasonPreimageModeContradiction  = "preimage-header-mode-contradiction"
	ReasonPostimageModeContradiction = "postimage-header-mode-contradiction"
)

// SideBytes holds the exact bytes of one observed side. Bodies stay in
// memory and are never persisted. They remain available for derivation
// and simulation until the observation is discarded. Callers must treat
// them as immutable; repeated Git objects may share a backing array.
type SideBytes struct {
	Preimage  []byte
	Postimage []byte
}

// EffectObservation pairs a normalized effect with its effect-local
// reasons. The per-side observation flags, hashes and modes live on the
// effect itself, which is what makes an effect self-describing to a
// consumer that never saw the producer.
type EffectObservation struct {
	Effect      gitutil.PatchEffect
	Bytes       SideBytes
	ReasonCodes []string
	// Contradictions carries human-readable details for header-mode
	// contradictions and bounded-capture refusals. It is diagnostic
	// text, never authority; retention refusals use the existing
	// preimage-unavailable/postimage-unavailable reason codes.
	Contradictions []string
}

// ArtifactSnapshot is one observation of a bound artifact's exact bytes,
// taken at a named moment. P6 checkpoints externally authored bytes with
// one; P7 takes a matching before/after pair around `$EDITOR`.
type ArtifactSnapshot struct {
	// Path is the RESOLVED path the producer actually opened.
	Path     string
	Observed bool
	Present  bool
	SHA256   string
	Bytes    []byte
}

// Observation is the complete immutable capture of one producer event.
type Observation struct {
	Producer  ProducerID
	RepoRoot  string
	Slug      string
	Capture   CaptureDescriptor
	Reference ReferenceDescriptor

	// ParentCreatedPaths is the sorted, pre-captured exclusion set of
	// paths whose required preimage depends on parent-created bytes.
	// It makes no ownership or history claim and never authorizes reuse
	// of a parent's body.
	ParentCreatedPaths []string

	// PatchPresent is true exactly when a canonical patch existed and was
	// readable. An absent patch and an unreadable one collapse here
	// deliberately; the read-error diagnostic keeps the real cause.
	PatchPresent bool
	// PatchSHA256 is the digest of the exact raw patch bytes whenever
	// PatchPresent is true — including the empty-byte-string digest, and
	// including a patch the grammar refused.
	PatchSHA256 string
	PatchBytes  []byte

	// Effects is empty when the grammar refused the patch or when the
	// patch is semantically empty. No effect list is ever invented.
	Effects []EffectObservation
	// ParseRefusal carries the strict grammar's own message when it
	// refused. It is never swallowed into an empty effect list.
	ParseRefusal string

	// ArtifactBefore and ArtifactAfter are the P6/P7 bound-artifact
	// snapshots. Both are nil for producers that bind through a patch
	// capture instead.
	ArtifactBefore *ArtifactSnapshot
	ArtifactAfter  *ArtifactSnapshot

	// Reasons are record-level, sorted and de-duplicated.
	Reasons []string
}

// ArtifactMutated reports whether the before/after pair proves the bound
// artifact's bytes changed. A pair that was not fully observed proves
// nothing and returns false.
func (o Observation) ArtifactMutated() bool {
	if o.ArtifactBefore == nil || o.ArtifactAfter == nil {
		return false
	}
	if !o.ArtifactBefore.Observed || !o.ArtifactAfter.Observed {
		return false
	}
	if o.ArtifactBefore.Present != o.ArtifactAfter.Present {
		return true
	}
	return o.ArtifactBefore.SHA256 != o.ArtifactAfter.SHA256
}

// PreflightError reports the strict grammar's refusal of the patch this
// observation binds, or nil when there was none.
//
// A producer calls it immediately after taking its observation and BEFORE
// its first bound write. An effect set nobody could derive means the
// producer cannot say what it is about to write; letting the write happen
// anyway is exactly how a strict refusal ends up surfacing after the
// artifacts have already landed.
//
// A patch that is absent, or present and semantically empty, is NOT a
// preflight failure: neither is a parse refusal, and both are recorded as
// record-level reasons instead.
func (o Observation) PreflightError() error {
	if o.ParseRefusal == "" {
		return nil
	}
	return errors.New(o.ParseRefusal)
}

// Input is what a producer knows at the moment it is about to bind.
type Input struct {
	Producer ProducerID
	RepoRoot string
	Slug     string

	// Patch is the canonical patch bytes the producer is about to bind.
	// PatchPresent distinguishes "no readable patch" from "a patch that
	// happens to be empty", which is a real and different state.
	Patch        string
	PatchPresent bool

	Capture CaptureDescriptor

	// ParentCreatedPaths is supplied during producer discovery, before
	// capture. It names targets whose required preimage depends on
	// parent-created bytes; it is an exclusion input, not an ownership
	// or history claim. Observe copies it without reading parent bodies.
	ParentCreatedPaths []string

	// PreimageRef is the ref the preimage is reconstructed from: HEAD for
	// the working-tree modes, the resolved lower commit for a range, the
	// accepted upstream commit for reconcile. Empty means the producer
	// has no reference, which yields ReferenceKindUnavailable.
	PreimageRef string
	// PostimageRef, when non-empty, reads postimages from that commit
	// instead of the working tree. Committed-range captures set it so the
	// observation describes the range, not whatever the tree holds now.
	PostimageRef string
}

// Observe takes the immutable observation. It never fails: an observation
// that could not be taken is recorded as flags and reasons, because a
// producer that fabricates a side is worse than one that admits it did
// not look.
func Observe(in Input) Observation {
	return observeWithImageBudget(in, imageRetentionLimit)
}

// The internal limit parameter permits small boundary fixtures without a
// mutable process-global override or a new producer configuration surface.
func observeWithImageBudget(in Input, limit int64) Observation {
	obs := Observation{
		Producer:           in.Producer,
		RepoRoot:           in.RepoRoot,
		Slug:               in.Slug,
		Capture:            in.Capture.normalized(),
		PatchPresent:       in.PatchPresent,
		Effects:            []EffectObservation{},
		ParentCreatedPaths: sortedCopy(in.ParentCreatedPaths),
	}

	if in.PatchPresent {
		obs.PatchBytes = []byte(in.Patch)
		obs.PatchSHA256 = sha256Hex(obs.PatchBytes)
	} else {
		obs.Reasons = append(obs.Reasons, ReasonPatchMissing)
	}

	obs.Reference = resolveReference(in)
	if obs.Reference.Kind != ReferenceKindCommit {
		obs.Reasons = append(obs.Reasons, ReasonReferenceNotDurable)
	}

	if in.PatchPresent {
		effects, err := gitutil.NormalizePatchEffects(in.Patch)
		switch {
		case err != nil:
			obs.ParseRefusal = err.Error()
			obs.Reasons = append(obs.Reasons, ReasonPatchUnparseable)
		case len(effects) == 0:
			obs.Reasons = append(obs.Reasons, ReasonPatchEmpty)
		default:
			obs.Effects = observeEffects(in, obs.Reference, effects, &imageBudget{limit: limit})
		}
	}

	obs.Reference.PreimageSetSHA256 = PreimageSetDigest(obs.Effects)
	obs.Reasons = sortedUnique(obs.Reasons)
	return obs
}

// resolveReference maps the capture mode onto a typed reference. An
// unmapped mode and an unresolvable commit both land on `unavailable`;
// neither is silently upgraded.
func resolveReference(in Input) ReferenceDescriptor {
	kind := ReferenceKindForMode(in.Capture.Mode)
	if kind != ReferenceKindCommit || strings.TrimSpace(in.PreimageRef) == "" {
		return ReferenceDescriptor{Kind: ReferenceKindUnavailable}
	}
	commit, ok := resolveCommit(in.RepoRoot, in.PreimageRef)
	if !ok {
		return ReferenceDescriptor{Kind: ReferenceKindUnavailable}
	}
	return ReferenceDescriptor{Kind: ReferenceKindCommit, Commit: commit}
}

// sideSource names which reader answers one side of one effect. A side
// with no source is one nothing could establish, and it is recorded as
// unobserved rather than guessed from the header.
type sideSource int

const (
	sourceNone sideSource = iota
	sourceTree
	sourceWorktree
)

// sidePlan is the decision taken for one side BEFORE any process runs, so
// the paths of every effect can be read in one batch per reference.
type sidePlan struct {
	source sideSource
	path   string
}

// observeEffects takes the ordered per-effect observation.
//
// The whole effect set is planned first and read second: one `rev-parse`
// per reference, one `ls-tree` per commit, one `cat-file --batch` for
// every blob body of every reference, and one `ls-files --stage` only if
// a worktree postimage turns out to be a gitlink directory. Nothing in
// that budget scales with the number of effects.
func observeEffects(in Input, ref ReferenceDescriptor, effects []gitutil.PatchEffect, budget *imageBudget) []EffectObservation {
	preCommit := ""
	if ref.Kind == ReferenceKindCommit {
		preCommit = ref.Commit
	}

	// The postimage reference is resolved ONCE for the whole observation.
	// Resolving it per effect re-ran `rev-parse` for every record and, on
	// failure, re-learned the same negative answer N times.
	postCommit := ""
	postRefUnresolvable := false
	if strings.TrimSpace(in.PostimageRef) != "" {
		commit, ok := resolveCommit(in.RepoRoot, in.PostimageRef)
		if !ok {
			postRefUnresolvable = true
		}
		postCommit = commit
	}

	prePlans := make([]sidePlan, len(effects))
	postPlans := make([]sidePlan, len(effects))
	var preTreePaths, postTreePaths, worktreePaths []string

	for i, effect := range effects {
		prePath := effect.Path
		if effect.OldPath != "" {
			prePath = effect.OldPath
		}
		if preCommit != "" && ensureInRepo(in.RepoRoot, prePath) == nil {
			prePlans[i] = sidePlan{source: sourceTree, path: prePath}
			preTreePaths = append(preTreePaths, prePath)
		}

		switch {
		case postRefUnresolvable:
			// The capture describes a range whose upper bound does not
			// resolve, so no postimage of it was established.
		case postCommit != "":
			if ensureInRepo(in.RepoRoot, effect.Path) == nil {
				postPlans[i] = sidePlan{source: sourceTree, path: effect.Path}
				postTreePaths = append(postTreePaths, effect.Path)
			}
		default:
			if ensureInRepo(in.RepoRoot, effect.Path) == nil {
				postPlans[i] = sidePlan{source: sourceWorktree, path: effect.Path}
				worktreePaths = append(worktreePaths, effect.Path)
			}
		}
	}

	preTree := readTreeEntries(in.RepoRoot, preCommit, preTreePaths)
	postTree := readTreeEntries(in.RepoRoot, postCommit, postTreePaths)
	worktree := readWorktreeSides(in.RepoRoot, worktreePaths, budget)
	blobs := readBlobs(in.RepoRoot, neededBlobIDs(preTree, postTree), budget)

	out := make([]EffectObservation, 0, len(effects))
	for i, effect := range effects {
		entry := EffectObservation{Effect: effect, ReasonCodes: []string{}}
		preExtant, postExtant := effect.ExtantSides()

		pre := sideFromPlan(prePlans[i], preTree, worktree, blobs)
		if pre.diagnostic != "" {
			entry.Contradictions = append(entry.Contradictions,
				fmt.Sprintf("preimage of %q: %s", prePlans[i].path, pre.diagnostic))
		}
		pre, preContradiction := reconcileHeaderMode(pre, effect.HeaderOldMode)
		if preContradiction {
			entry.ReasonCodes = append(entry.ReasonCodes, ReasonPreimageModeContradiction)
			entry.Contradictions = append(entry.Contradictions,
				fmt.Sprintf("preimage of %q: patch header declares mode %s, the observed object is not that mode",
					prePlans[i].path, effect.HeaderOldMode))
		}
		pre = demoteImpossibleAbsence(pre, preExtant)

		post := sideFromPlan(postPlans[i], postTree, worktree, blobs)
		if post.diagnostic != "" {
			entry.Contradictions = append(entry.Contradictions,
				fmt.Sprintf("postimage of %q: %s", postPlans[i].path, post.diagnostic))
		}
		post, postContradiction := reconcileHeaderMode(post, effect.HeaderNewMode)
		if postContradiction {
			entry.ReasonCodes = append(entry.ReasonCodes, ReasonPostimageModeContradiction)
			entry.Contradictions = append(entry.Contradictions,
				fmt.Sprintf("postimage of %q: patch header declares mode %s, the observed object is not that mode",
					postPlans[i].path, effect.HeaderNewMode))
		}
		post = demoteImpossibleAbsence(post, postExtant)

		entry.Effect.PreimageObserved = pre.observed
		entry.Effect.PreimagePresent = pre.present
		entry.Effect.PreimageSHA256 = pre.sha256
		entry.Effect.OldMode = pre.mode
		entry.Bytes.Preimage = pre.bytes

		entry.Effect.PostimageObserved = post.observed
		entry.Effect.PostimagePresent = post.present
		entry.Effect.PostimageSHA256 = post.sha256
		entry.Effect.NewMode = post.mode
		entry.Bytes.Postimage = post.bytes

		if !pre.observed {
			entry.ReasonCodes = append(entry.ReasonCodes, ReasonPreimageUnavailable)
		}
		if !post.observed {
			entry.ReasonCodes = append(entry.ReasonCodes, ReasonPostimageUnavailable)
		}

		entry.Effect.ObjectKind = resolveObjectKind(entry.Effect, postExtant)
		entry.Effect.ContentKind = resolveContentKind(entry.Effect, entry.Bytes, preExtant, postExtant)
		entry.ReasonCodes = sortedUnique(entry.ReasonCodes)
		out = append(out, entry)
	}
	return out
}

// sideFromPlan folds one planned side onto the batched read that answers
// it.
func sideFromPlan(plan sidePlan, tree treeSnapshot, worktree map[string]sideObservation, blobs map[string]blobObservation) sideObservation {
	switch plan.source {
	case sourceTree:
		return sideFromTree(tree, blobs, plan.path)
	case sourceWorktree:
		return worktree[plan.path]
	default:
		return unobserved()
	}
}

// sideFromTree reads one side out of a batched tree snapshot.
//
// A batch that did not run establishes nothing, so every side it would
// have answered is unobserved. A batch that DID run and does not carry
// the path proves absence: the producer looked in that tree and the path
// was not there.
func sideFromTree(tree treeSnapshot, blobs map[string]blobObservation, path string) sideObservation {
	entry, found, read := tree.lookup(path)
	if !read {
		return unobserved()
	}
	if !found {
		return sideObservation{observed: true}
	}
	if entry.mode == gitutil.ModeGitlink {
		return gitlinkSide(entry)
	}
	blob, ok := blobs[entry.objectSHA]
	if !ok {
		return unobserved()
	}
	if blob.diagnostic != "" {
		return sideObservation{diagnostic: blob.diagnostic}
	}
	body := blob.bytes
	return sideObservation{observed: true, present: true, mode: entry.mode, sha256: sha256Hex(body), bytes: body}
}

// gitlinkSide is the identity of a submodule pointer. A gitlink has no
// file content on either side, so its identity binds the referenced
// commit rather than a blob nobody can read.
func gitlinkSide(entry treeEntryRecord) sideObservation {
	body := []byte(entry.objectSHA)
	return sideObservation{observed: true, present: true, mode: gitutil.ModeGitlink, sha256: sha256Hex(body)}
}

// neededBlobIDs collects every non-gitlink object id the batched tree
// reads produced, across BOTH references, so one `cat-file --batch`
// answers all of them.
func neededBlobIDs(snapshots ...treeSnapshot) []string {
	ids := map[string]bool{}
	for _, snapshot := range snapshots {
		if !snapshot.read {
			continue
		}
		for _, entry := range snapshot.entries {
			if entry.mode == gitutil.ModeGitlink || entry.objectSHA == "" {
				continue
			}
			ids[entry.objectSHA] = true
		}
	}
	return sortedObjectIDs(ids)
}

// readWorktreeSides reads every working-tree postimage directly — a
// working tree is a filesystem, so no Git process is needed for it — and
// then resolves, in ONE batched index read, the one shape the filesystem
// cannot answer: a directory the index records as a gitlink.
func readWorktreeSides(repoRoot string, paths []string, budget *imageBudget) map[string]sideObservation {
	out := make(map[string]sideObservation, len(paths))
	var gitlinkCandidates []string
	for _, path := range sortedUnique(paths) {
		side, isDirectory := observeWorktreePath(repoRoot, path, budget)
		if isDirectory {
			gitlinkCandidates = append(gitlinkCandidates, path)
			continue
		}
		out[path] = side
	}
	if len(gitlinkCandidates) == 0 {
		return out
	}
	index := readIndexEntries(repoRoot, gitlinkCandidates)
	for _, path := range gitlinkCandidates {
		entry, found, read := index.lookup(path)
		if !read || !found || entry.mode != gitutil.ModeGitlink {
			// A directory the index does not record as a gitlink is not
			// a submodule pointer, and nothing here can say what it is.
			out[path] = unobserved()
			continue
		}
		out[path] = gitlinkSide(entry)
	}
	return out
}

// observeWorktreePath reads one path from the working tree. The second
// result reports "this is a directory", which is the one case the
// filesystem cannot decide alone: the index holds the submodule commit.
func observeWorktreePath(repoRoot, path string, budget *imageBudget) (side sideObservation, isDirectory bool) {
	abs := filepath.Join(repoRoot, filepath.FromSlash(path))
	info, err := os.Lstat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return sideObservation{observed: true}, false
		}
		return unobserved(), false
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		if diagnostic := budget.refusal(info.Size()); diagnostic != "" {
			return sideObservation{diagnostic: diagnostic}, false
		}
		target, rerr := os.Readlink(abs)
		if rerr != nil {
			return unobserved(), false
		}
		body, rerr := readImage(strings.NewReader(target), int64(len(target)), budget)
		if rerr != nil {
			return sideObservation{diagnostic: rerr.Error()}, false
		}
		return sideObservation{observed: true, present: true, mode: gitutil.ModeSymlink, sha256: sha256Hex(body), bytes: body}, false
	case info.IsDir():
		return unobserved(), true
	case !info.Mode().IsRegular():
		return unobserved(), false
	}
	if diagnostic := budget.refusal(info.Size()); diagnostic != "" {
		return sideObservation{diagnostic: diagnostic}, false
	}
	file, rerr := os.Open(abs)
	if rerr != nil {
		return unobserved(), false
	}
	defer file.Close()
	info, rerr = file.Stat()
	if rerr != nil || !info.Mode().IsRegular() {
		return unobserved(), false
	}
	body, rerr := readWorktreeImage(file, info.Size(), budget)
	if rerr != nil {
		return sideObservation{diagnostic: rerr.Error()}, false
	}
	mode := worktreeGitMode(info.Mode())
	return sideObservation{observed: true, present: true, mode: mode, sha256: sha256Hex(body), bytes: body}, false
}

func worktreeGitMode(mode fs.FileMode) string {
	// Git canonicalizes executable mode from the owner execute bit
	// (`ce_permissions`), not from group/other execute bits.
	if mode.Perm()&0o100 != 0 {
		return gitutil.ModeExecutable
	}
	return gitutil.ModeRegular
}

// sideObservation is the internal per-side result before it is folded
// onto the effect.
type sideObservation struct {
	observed   bool
	present    bool
	mode       string
	sha256     string
	bytes      []byte
	diagnostic string
}

// unobserved is the honest record of "nobody established this": no hash,
// no mode, and the caller raises the matching availability reason.
func unobserved() sideObservation { return sideObservation{} }

// demoteImpossibleAbsence enforces the contradictory-observation rule. For
// a side the change kind requires to exist, observed-and-absent asserts
// that the producer looked and found the extant side missing, which
// contradicts the change kind the same record carries. Such a side is
// recorded as unobserved instead, which is what actually happened: the
// producer could not establish its bytes.
func demoteImpossibleAbsence(side sideObservation, extant bool) sideObservation {
	if extant && side.observed && !side.present {
		return unobserved()
	}
	return side
}

// reconcileHeaderMode refuses a contradiction between the observed mode
// and the mode the patch header declared. The observation is the mode
// authority and the header only corroborates it, so a conflict means the
// producer did not observe the object the patch describes.
//
// The second result reports THAT this happened, so the caller can record
// a contradiction rather than an availability problem. Demoting silently
// would file a wrong-object observation under "could not read it".
func reconcileHeaderMode(side sideObservation, headerMode string) (sideObservation, bool) {
	if !side.observed || !side.present || headerMode == "" {
		return side, false
	}
	if gitutil.ObjectKindForMode(headerMode) == gitutil.ObjectKindUnknown {
		// The header carries a mode outside the permitted set, so it
		// corroborates nothing and cannot contradict anything.
		return side, false
	}
	if side.mode != headerMode {
		return unobserved(), true
	}
	return side, false
}

// resolveObjectKind selects the kind from one named EXTANT side, and is
// unknown when the side the selection needs was not observed. No kind is
// inferred across an unobserved transition.
func resolveObjectKind(effect gitutil.PatchEffect, postExtant bool) gitutil.ObjectKind {
	if effect.PostimageObserved && effect.PostimagePresent {
		return gitutil.ObjectKindForMode(effect.NewMode)
	}
	if !postExtant && effect.PreimageObserved && effect.PreimagePresent {
		return gitutil.ObjectKindForMode(effect.OldMode)
	}
	return gitutil.ObjectKindUnknown
}

// resolveContentKind evaluates ADR-036 D3's ordered rule: the first branch
// that holds wins.
func resolveContentKind(effect gitutil.PatchEffect, body SideBytes, preExtant, postExtant bool) gitutil.ContentKind {
	if effect.ObjectKind == gitutil.ObjectKindGitlink {
		return gitutil.ContentKindNone
	}
	if effect.BinaryStanza {
		return gitutil.ContentKindBinary
	}
	if effect.PreimageObserved && effect.PreimagePresent && bytes.IndexByte(body.Preimage, 0) >= 0 {
		return gitutil.ContentKindBinary
	}
	if effect.PostimageObserved && effect.PostimagePresent && bytes.IndexByte(body.Postimage, 0) >= 0 {
		return gitutil.ContentKindBinary
	}
	if preExtant && !(effect.PreimageObserved && effect.PreimagePresent) {
		return gitutil.ContentKindUnknown
	}
	if postExtant && !(effect.PostimageObserved && effect.PostimagePresent) {
		return gitutil.ContentKindUnknown
	}
	return gitutil.ContentKindText
}

// ClassifyEffectObservation reuses capture's D3 kind rules without reading a
// tree or mutating the observation. The caller validates presence, modes and
// body hashes before using this projection as a binding.
func ClassifyEffectObservation(effect gitutil.PatchEffect, body SideBytes) (gitutil.ContentKind, gitutil.ObjectKind) {
	pre, post := effect.ExtantSides()
	effect.ObjectKind = resolveObjectKind(effect, post)
	return resolveContentKind(effect, body, pre, post), effect.ObjectKind
}

// SnapshotArtifact observes one bound artifact's exact bytes at the
// RESOLVED path the producer opened. An unreadable path is recorded as
// unobserved, never as proven absence.
func SnapshotArtifact(path string) ArtifactSnapshot {
	snap := ArtifactSnapshot{Path: path}
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			snap.Observed = true
			return snap
		}
		return snap
	}
	if !info.Mode().IsRegular() {
		return snap
	}
	body, rerr := os.ReadFile(path)
	if rerr != nil {
		return snap
	}
	snap.Observed = true
	snap.Present = true
	snap.Bytes = body
	snap.SHA256 = sha256Hex(body)
	return snap
}

// preimageSetEntry is the canonical, field-ordered projection the
// preimage-set digest hashes. Its shape is deliberately explicit so the
// digest cannot drift when the effect struct gains a field.
//
// The two path fields are HEX, not text. A repo-relative path is a byte
// string, and Git C-quoting can carry bytes that are not valid UTF-8;
// `encoding/json` replaces every such byte with U+FFFD, so the paths
// "\xff" and "\xfe" would encode identically and two different
// observation sets would share one digest. Hex-encoding them makes the
// digest byte-exact while keeping the surrounding encoding canonical
// JSON.
type preimageSetEntry struct {
	Ordinal           int    `json:"ordinal"`
	PathHex           string `json:"path_hex"`
	OldPathHex        string `json:"old_path_hex"`
	ChangeKind        string `json:"change_kind"`
	ContentKind       string `json:"content_kind"`
	ObjectKind        string `json:"object_kind"`
	OldMode           string `json:"old_mode"`
	NewMode           string `json:"new_mode"`
	PreimageObserved  bool   `json:"preimage_observed"`
	PreimagePresent   bool   `json:"preimage_present"`
	PreimageSHA256    string `json:"preimage_sha256"`
	PostimageObserved bool   `json:"postimage_observed"`
	PostimagePresent  bool   `json:"postimage_present"`
	PostimageSHA256   string `json:"postimage_sha256"`
}

// PreimageSetDigest is the deterministic digest of the ordered
// path/kind/mode/existence/content-hash observation set. It proves
// observation identity, not durable reconstructability.
func PreimageSetDigest(effects []EffectObservation) string {
	entries := make([]preimageSetEntry, 0, len(effects))
	for _, entry := range effects {
		e := entry.Effect
		entries = append(entries, preimageSetEntry{
			Ordinal:           e.Ordinal,
			PathHex:           hex.EncodeToString([]byte(e.Path)),
			OldPathHex:        hex.EncodeToString([]byte(e.OldPath)),
			ChangeKind:        string(e.ChangeKind),
			ContentKind:       string(e.ContentKind),
			ObjectKind:        string(e.ObjectKind),
			OldMode:           e.OldMode,
			NewMode:           e.NewMode,
			PreimageObserved:  e.PreimageObserved,
			PreimagePresent:   e.PreimagePresent,
			PreimageSHA256:    e.PreimageSHA256,
			PostimageObserved: e.PostimageObserved,
			PostimagePresent:  e.PostimagePresent,
			PostimageSHA256:   e.PostimageSHA256,
		})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Ordinal < entries[j].Ordinal })
	encoded, err := json.Marshal(entries)
	if err != nil {
		// The projection contains only ASCII strings, ints and bools, so
		// this is unreachable; returning the empty-input digest keeps the
		// function total without inventing a value.
		return sha256Hex(nil)
	}
	return sha256Hex(encoded)
}

// ── the publication seam ─────────────────────────────────────────────────

// Recorder receives an immutable producer observation. The default
// recorder discards: S1's obligation is that every governed producer
// hands its observation over before its first bound write, and a later
// slice owns what happens next.
type Recorder interface {
	Record(Observation)
}

type discardRecorder struct{}

func (discardRecorder) Record(Observation) {}

var (
	recorderMu sync.Mutex
	recorder   Recorder = discardRecorder{}
)

// SetRecorder installs a recorder and returns a function restoring the
// previous one. Tests use it to assert capture ordering; production code
// installs the real publication step through the same seam.
func SetRecorder(r Recorder) func() {
	recorderMu.Lock()
	defer recorderMu.Unlock()
	previous := recorder
	if r == nil {
		r = discardRecorder{}
	}
	recorder = r
	return func() {
		recorderMu.Lock()
		defer recorderMu.Unlock()
		recorder = previous
	}
}

// Emit hands an independently owned observation to the installed recorder.
func Emit(obs Observation) {
	recorderMu.Lock()
	current := recorder
	recorderMu.Unlock()
	if _, discard := current.(discardRecorder); discard {
		return
	}
	current.Record(cloneObservation(obs))
}

// ObserveAndEmit is the one-call form producers use immediately before
// their first bound write.
func ObserveAndEmit(in Input) Observation {
	obs := Observe(in)
	Emit(obs)
	return obs
}

// ── small helpers ────────────────────────────────────────────────────────

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func isFullCommitHex(s string) bool {
	if len(s) != 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// ensureInRepo runs a repo-relative patch path through the shared path
// safety check before anything opens it.
func ensureInRepo(repoRoot, path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	return safety.EnsureSafeRepoPath(repoRoot, filepath.Join(repoRoot, filepath.FromSlash(path)))
}

func sortedCopy(in []string) []string {
	out := make([]string, 0, len(in))
	out = append(out, in...)
	sort.Strings(out)
	return out
}

func sortedUnique(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
