package gitutil

import (
	"crypto/sha256"
	"errors"
	"fmt"
)

var (
	ErrInsertionFragment    = errors.New("fragment-unparseable")
	ErrInsertionBoundary    = errors.New("fragment-boundary-underivable")
	ErrInsertionReplacement = errors.New("effect-replacement-unsupported")
)

// PatchInsertionRun is an ephemeral, byte-exact projection of a strict hunk.
type PatchInsertionRun struct {
	Ordinal  int
	Boundary int
	Records  [][]byte
}

// ExtractPatchInsertionRuns shares the exact reconstruction walker. The
// returned digest proves replay without allocating a second full postimage.
func ExtractPatchInsertionRuns(patch string, effect PatchEffect, pre []byte) ([]PatchInsertionRun, string, error) {
	effects, err := NormalizePatchEffects(patch)
	if err != nil || effect.Ordinal < 1 || effect.Ordinal > len(effects) || effects[effect.Ordinal-1] != effect {
		return nil, "", ErrInsertionFragment
	}
	if effect.BinaryStanza || (effect.ChangeKind != ChangeKindAdd && effect.ChangeKind != ChangeKindModify) {
		return nil, "", ErrInsertionFragment
	}
	var runs []PatchInsertionRun
	continuing, replacement := false, false
	visit := func(kind byte, boundary int, body string) {
		if kind == '-' {
			replacement = true
		}
		if kind != '+' {
			continuing = false
			return
		}
		if !continuing {
			runs = append(runs, PatchInsertionRun{Ordinal: len(runs) + 1, Boundary: boundary})
		}
		runs[len(runs)-1].Records = append(runs[len(runs)-1].Records, []byte(body))
		continuing = true
	}
	post, err := reconstructTextRecordWithVisitor(patch[effect.FragmentStart:effect.FragmentEnd], pre, 0, visit)
	if replacement {
		return nil, "", ErrInsertionReplacement
	}
	if err != nil {
		var proof *PatchImageBudgetProof
		if !errors.As(err, &proof) {
			return nil, "", ErrInsertionBoundary
		}
		return runs, proof.SHA256, nil
	}
	sum := sha256.Sum256(post)
	return runs, fmt.Sprintf("%x", sum[:]), nil
}
