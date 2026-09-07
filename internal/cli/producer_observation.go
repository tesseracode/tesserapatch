package cli

// Producer observation wiring (GH #15 / ADR-036 D2,
// PRD-recipe-generation-authority §6.2).
//
// Every governed producer captures its immutable observation BEFORE its
// first bound write. The helpers here keep that ordering visible at the
// call site: each one is invoked on the last line of a producer's
// discovery window, immediately above the write it is about to bind.
//
// The observation itself is in-memory only. Nothing here writes a file or
// changes public output; the Recorder seam in internal/patchobs is the
// single hand-off point a later slice replaces with the shared
// publication step.

import (
	"fmt"

	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// observeCaptureMode maps a CLI capture-mode label onto the observation
// vocabulary. An unmapped label falls to no-capture rather than being
// silently treated as a working-tree capture, because a producer that
// cannot name its capture has not established a reference.
func observeCaptureMode(label string) patchobs.CaptureMode {
	mode := patchobs.CaptureMode(label)
	if !patchobs.KnownCaptureMode(mode) {
		return patchobs.CaptureModeNoCapture
	}
	return mode
}

// preimageRefForCaptureMode names the ref the preimage is reconstructed
// from: the resolved lower bound for a committed range, and HEAD for
// every working-tree mode.
//
// `--unstaged` is commit-kind HEAD, not an index snapshot: record refuses
// a capture whose staged and unstaged edits touch the same path before
// any capture runs, so every path that reaches an accepted `--unstaged`
// patch has an index entry identical to HEAD.
func preimageRefForCaptureMode(mode patchobs.CaptureMode, fromRef string) string {
	switch mode {
	case patchobs.CaptureModeCommittedRange,
		patchobs.CaptureModeAutoCommittedRange,
		patchobs.CaptureModeExplicitCommittedRange:
		return fromRef
	case patchobs.CaptureModeNoCapture:
		return ""
	default:
		return "HEAD"
	}
}

// postimageRefForCaptureMode names the ref postimages are read from. A
// committed range describes two commits, so its postimage is the upper
// bound rather than whatever the working tree holds now; every other mode
// reads the working tree it just captured.
//
// The empty-upper default mirrors recordGenerationUpper: a range with no
// explicit upper bound means HEAD.
func postimageRefForCaptureMode(mode patchobs.CaptureMode, toRef string) string {
	switch mode {
	case patchobs.CaptureModeCommittedRange,
		patchobs.CaptureModeAutoCommittedRange,
		patchobs.CaptureModeExplicitCommittedRange:
		if toRef == "" {
			return "HEAD"
		}
		return toRef
	default:
		return ""
	}
}

// observePatchProducer takes and emits the immutable observation for a
// patch-writing producer, and returns the PREFLIGHT error the observation
// establishes.
//
// Callers invoke it immediately before their first bound write and MUST
// return on a non-nil error. That ordering is the whole point (PI-3): the
// strict grammar decides whether the producer can describe the bytes it
// is about to bind, and it decides that BEFORE post-apply.patch, the
// numbered snapshot, the recipe and the generation append have landed.
// The downstream `AppendPatchGenerationForFeature` reparse is the last
// line of defence behind this check, not a substitute for it.
//
// The observation is taken and handed to the seam even when the preflight
// fails: "the producer looked and the grammar refused" is itself the
// record S1 owes, and discarding it would lose the refusal message.
func observePatchProducer(
	producer patchobs.ProducerID,
	s *store.Store,
	slug, patch, captureModeLabel, fromRef, toRef string,
	pathspecs, claimIDs []string,
) (patchobs.Observation, error) {
	mode := observeCaptureMode(captureModeLabel)
	obs := patchobs.ObserveAndEmit(patchobs.Input{
		Producer:     producer,
		RepoRoot:     s.Root,
		Slug:         slug,
		Patch:        patch,
		PatchPresent: true,
		Capture: patchobs.CaptureDescriptor{
			Mode:      mode,
			Pathspecs: pathspecs,
			ClaimIDs:  claimIDs,
		},
		PreimageRef:  preimageRefForCaptureMode(mode, fromRef),
		PostimageRef: postimageRefForCaptureMode(mode, toRef),
	})
	if err := obs.PreflightError(); err != nil {
		return obs, fmt.Errorf("%s: captured patch is unreadable, refusing to bind it: %w", producer, err)
	}
	return obs, nil
}
