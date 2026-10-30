package candidate

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
)

type operation struct {
	kind                              string
	path                              string
	effectOrdinal                     int
	runOrdinals                       []int
	boundary, left, right, byteOffset int
	payload                           []byte
}

type effectClassification struct {
	ordinal int
	path    string
	state   classification
	reason  string
}

// Plan is ephemeral. It contains execution coordinates/payloads, not an artifact.
type Plan struct {
	operations []operation
	effects    []effectClassification
}

func (p Plan) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("derivation plan is ephemeral and cannot be serialized")
}

func (p Plan) Counts() (presentCount, applicableCount, conflictCount int) {
	for _, effect := range p.effects {
		switch effect.state {
		case present:
			presentCount++
		case applicable:
			applicableCount++
		case conflict:
			conflictCount++
		}
	}
	return
}

func derivationDomain(effect gitutil.PatchEffect) error {
	if effect.ChangeKind != gitutil.ChangeKindAdd && effect.ChangeKind != gitutil.ChangeKindModify {
		return &refusal{code: "effect-change-kind-unsupported", path: effect.Path}
	}
	if effect.ContentKind != gitutil.ContentKindText {
		return &refusal{code: "effect-content-kind-unsupported", path: effect.Path}
	}
	if effect.ObjectKind != gitutil.ObjectKindRegular {
		return &refusal{code: "effect-object-kind-unsupported", path: effect.Path}
	}
	if effect.NewMode != gitutil.ModeRegular ||
		(effect.ChangeKind == gitutil.ChangeKindAdd && effect.OldMode != "") ||
		(effect.ChangeKind == gitutil.ChangeKindModify && effect.OldMode != gitutil.ModeRegular) {
		return &refusal{code: "effect-mode-change-unsupported", path: effect.Path}
	}
	if !effect.PreimageObserved || !effect.PostimageObserved {
		return &refusal{code: "effect-binding-drift", path: effect.Path}
	}
	if !effect.PostimagePresent || effect.PreimagePresent != (effect.ChangeKind == gitutil.ChangeKindModify) ||
		(effect.ChangeKind == gitutil.ChangeKindAdd && effect.PreimageSHA256 != "") {
		return &refusal{code: "effect-binding-drift", path: effect.Path}
	}
	return nil
}

// Derive creates no worktree and writes nothing. Effects must be the validated
// paired E/C projections; their observation ceiling is never promoted.
func Derive(reference, upstream *ReferenceRoot, patch string, effects []gitutil.PatchEffect) (Plan, error) {
	normalized, err := gitutil.NormalizePatchEffects(patch)
	if err != nil || len(normalized) == 0 || len(normalized) != len(effects) {
		return Plan{}, &refusal{code: "fragment-unparseable"}
	}
	order := make([]int, len(effects))
	paths := make([]string, len(effects))
	for i, effect := range effects {
		parsed := normalized[i]
		if effect.Ordinal != parsed.Ordinal || effect.Path != parsed.Path || effect.OldPath != parsed.OldPath ||
			effect.ChangeKind != parsed.ChangeKind || effect.FragmentSHA256 != parsed.FragmentSHA256 {
			return Plan{}, &refusal{code: "effect-binding-drift", path: effect.Path}
		}
		order[i], paths[i] = i, effect.Path
	}
	sort.Slice(order, func(i, j int) bool { return effects[order[i]].Path < effects[order[j]].Path })
	for _, index := range order {
		if err := derivationDomain(effects[index]); err != nil {
			return Plan{}, err
		}
	}
	baseSnapshot, err := reference.snapshotPaths(paths)
	if err != nil {
		return Plan{}, &refusal{code: "reference-not-reconstructable"}
	}
	// Freeze every upstream identity before the first classification. Body reads
	// below use these immutable object IDs, never a later path/ref lookup.
	snapshot, err := upstream.snapshotPaths(paths)
	if err != nil {
		return Plan{}, &refusal{code: "reference-not-reconstructable"}
	}
	var plan Plan
	for _, index := range order {
		effect := effects[index]
		baseEntry := baseSnapshot.entries[effect.Path]
		if baseEntry.Present != effect.PreimagePresent || baseEntry.Mode != effect.OldMode || !effect.PostimagePresent {
			return Plan{}, &refusal{code: "effect-binding-drift", path: effect.Path}
		}
		entry := snapshot.entries[effect.Path]
		if effect.ChangeKind == gitutil.ChangeKindAdd {
			post, available, err := gitutil.ReconstructPatchPostimage(patch, normalized[index], nil, int64(len(patch)))
			if err != nil || !available {
				return Plan{}, &refusal{code: "fragment-boundary-underivable", path: effect.Path}
			}
			if digestBytes(post) != effect.PostimageSHA256 {
				return Plan{}, &refusal{code: "derivation-postimage-mismatch", path: effect.Path}
			}
			if bytes.IndexByte(post, 0) >= 0 {
				return Plan{}, &refusal{code: "effect-binding-drift", path: effect.Path}
			}
			state := applicable
			if entry.Present {
				state = conflict
				if entry.Mode == gitutil.ModeRegular && entry.Size == int64(len(post)) {
					body, err := snapshot.read(effect.Path, int64(len(post)))
					if err != nil {
						return Plan{}, &refusal{code: "reference-not-reconstructable", path: effect.Path}
					}
					if bytes.Equal(body, post) {
						state = present
					}
				}
			}
			classified := effectClassification{ordinal: effect.Ordinal, path: effect.Path, state: state}
			if state == conflict {
				classified.reason = "creation-conflict"
			}
			plan.effects = append(plan.effects, classified)
			if state == applicable {
				plan.operations = append(plan.operations, operation{kind: "create-file", path: effect.Path,
					effectOrdinal: effect.Ordinal, payload: post})
			}
			continue
		}
		if baseEntry.Size > alignmentMaxBytes || entry.Size > alignmentMaxBytes {
			return Plan{}, &refusal{code: "alignment-limit", path: effect.Path}
		}
		pre, err := baseSnapshot.read(effect.Path, alignmentMaxBytes)
		if err != nil || digestBytes(pre) != effect.PreimageSHA256 || bytes.IndexByte(pre, 0) >= 0 {
			return Plan{}, &refusal{code: "effect-binding-drift", path: effect.Path}
		}
		if recordCount(pre) > alignmentMaxRecords {
			return Plan{}, &refusal{code: "alignment-limit", path: effect.Path}
		}
		runs, postHash, err := gitutil.ExtractPatchInsertionRuns(patch, normalized[index], pre)
		if err != nil {
			code := "fragment-boundary-underivable"
			if errors.Is(err, gitutil.ErrInsertionReplacement) {
				code = "effect-replacement-unsupported"
			} else if errors.Is(err, gitutil.ErrInsertionFragment) {
				code = "fragment-unparseable"
			}
			return Plan{}, &refusal{code: code, path: effect.Path}
		}
		if postHash != effect.PostimageSHA256 {
			return Plan{}, &refusal{code: "derivation-postimage-mismatch", path: effect.Path}
		}
		projected := make([]insertionRun, len(runs))
		for i, run := range runs {
			projected[i] = insertionRun{effectOrdinal: effect.Ordinal, runOrdinal: run.Ordinal,
				boundary: run.Boundary, records: run.Records}
		}
		replayed, err := replayInsertionDigest(pre, projected)
		if err != nil || replayed != effect.PostimageSHA256 {
			return Plan{}, &refusal{code: "derivation-postimage-mismatch", path: effect.Path}
		}
		if !entry.Present || entry.Mode != gitutil.ModeRegular {
			plan.effects = append(plan.effects, effectClassification{ordinal: effect.Ordinal, path: effect.Path,
				state: conflict, reason: "boundary-conflict"})
			continue
		}
		body, err := snapshot.read(effect.Path, alignmentMaxBytes)
		if err != nil {
			code := "reference-not-reconstructable"
			if errors.Is(err, patchobs.ErrCommitObjectLimit) {
				code = "alignment-limit"
			}
			return Plan{}, &refusal{code: code, path: effect.Path}
		}
		boundaries, err := alignInsertionRuns(pre, body, projected)
		if err != nil {
			var reason *refusal
			if errors.As(err, &reason) {
				return Plan{}, &refusal{code: reason.code, path: effect.Path}
			}
			return Plan{}, err
		}
		classified := effectClassification{ordinal: effect.Ordinal, path: effect.Path, state: present}
		for _, boundary := range boundaries {
			switch boundary.state {
			case conflict:
				classified.state, classified.reason = conflict, "boundary-conflict"
			case applicable:
				if classified.state != conflict {
					classified.state = applicable
				}
				plan.operations = append(plan.operations, operation{kind: "insert-by-boundary", path: effect.Path,
					effectOrdinal: effect.Ordinal, runOrdinals: boundary.runOrdinals, boundary: boundary.boundary,
					left: boundary.left, right: boundary.right, byteOffset: boundary.byteOffset,
					payload: bytes.Join(boundary.records, nil)})
			}
		}
		plan.effects = append(plan.effects, classified)
	}
	for _, effect := range plan.effects {
		if effect.state == conflict {
			plan.operations = nil
			break
		}
	}
	sort.Slice(plan.operations, func(i, j int) bool {
		if plan.operations[i].path != plan.operations[j].path {
			return plan.operations[i].path < plan.operations[j].path
		}
		return plan.operations[i].byteOffset > plan.operations[j].byteOffset
	})
	return plan, nil
}

func digestBytes(body []byte) string {
	sum := sha256.Sum256(body)
	return fmt.Sprintf("%x", sum[:])
}

func replayInsertionDigest(base []byte, runs []insertionRun) (string, error) {
	records := lineRecords(base)
	boundaries, err := combineInsertionRuns(runs, len(records))
	if err != nil {
		return "", err
	}
	digest := sha256.New()
	cursor := 0
	for _, boundary := range boundaries {
		for _, record := range records[cursor:boundary.boundary] {
			_, _ = digest.Write(record)
		}
		for _, record := range boundary.records {
			_, _ = digest.Write(record)
		}
		cursor = boundary.boundary
	}
	for _, record := range records[cursor:] {
		_, _ = digest.Write(record)
	}
	return fmt.Sprintf("%x", digest.Sum(nil)), nil
}
