package cli

// Producer observation wiring (GH #15 / ADR-036 D2,
// PRD-recipe-generation-authority §6.2).
//
// Every governed producer captures its immutable observation BEFORE its
// first bound write. The helpers here keep that ordering visible at the
// call site: each one is invoked on the last line of a producer's
// discovery window, immediately above the write it is about to bind.
//
// The recorder is a pre-write diagnostic seam, not publication. Producers
// retain its immutable input and publish separately after their owned writes.

import (
	"errors"
	"fmt"

	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
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

// coverageFinalizer is armed only after a successful bound write. Explicit
// completion precedes the success message; the deferred path preserves primary
// errors when a later owned write fails. Each event publishes at most once.
func coverageFinalizer(s *store.Store, in *workflow.CoveragePublicationInput) func(error) error {
	finished := false
	return func(primary error) error {
		if finished {
			return primary
		}
		finished = true
		_, err := workflow.PublishCoverage(s, *in)
		return errors.Join(primary, err)
	}
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
	checkpoint ...bool,
) (patchobs.Observation, error) {
	mode := observeCaptureMode(captureModeLabel)
	patchPresent := true
	isCheckpoint := len(checkpoint) != 0 && checkpoint[0]
	if isCheckpoint && producer != patchobs.ProducerFeaturePatch {
		return patchobs.Observation{}, fmt.Errorf("%s: no contracted re-observation checkpoint", producer)
	}
	if isCheckpoint {
		// P2 will write nothing. Bind the actual canonical artifact, not
		// the capture's matching generation hash if that artifact was
		// independently removed or edited.
		var err error
		patch, err = s.ReadFeatureFile(slug, "artifacts/post-apply.patch")
		patchPresent = err == nil
		if !patchPresent {
			patch = ""
		}
	}
	obs := patchobs.ObserveAndEmit(patchobs.Input{
		Producer:     producer,
		RepoRoot:     s.Root,
		Slug:         slug,
		Patch:        patch,
		PatchPresent: patchPresent,
		Capture: patchobs.CaptureDescriptor{
			Mode:      mode,
			Pathspecs: pathspecs,
			ClaimIDs:  claimIDs,
		},
		PreimageRef:  preimageRefForCaptureMode(mode, fromRef),
		PostimageRef: postimageRefForCaptureMode(mode, toRef),
	})
	if err := obs.PreflightError(); err != nil && !isCheckpoint {
		return obs, fmt.Errorf("%s: captured patch is unreadable, refusing to bind it: %w", producer, err)
	}
	return obs, nil
}
