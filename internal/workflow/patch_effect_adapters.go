package workflow

// Thin adapters over the one authoritative strict effect grammar
// (GH #15 / ADR-036 D1, PRD-recipe-generation-authority §6.1).
//
// Every adapter here PROJECTS gitutil's normalized effect set. None of
// them re-implements header splitting, dequoting or `a/`/`b/` stripping,
// and none of them swallows the strict error to preserve a shorter legacy
// result. That is the entire acceptance condition for an adapter: derive
// from the normalized effects, or do not exist.
//
// The functions these replaced dropped every Git C-quoted path on the
// floor, silently. A path that vanished from `touched_paths`, from
// novelty classification or from hunk attribution now either appears or
// produces an error.

import (
	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

// patchEffectView is the per-path projection the derivation and novelty
// consumers need: the canonical path plus the change axis, both read from
// the strict record header rather than guessed from a field split.
type patchEffectView struct {
	Path       string
	OldPath    string
	ChangeKind gitutil.ChangeKind
}

// patchEffectViews projects the strict normalized effect set. The strict
// error is returned, never absorbed.
func patchEffectViews(patch string) ([]patchEffectView, error) {
	effects, err := gitutil.NormalizePatchEffects(patch)
	if err != nil {
		return nil, err
	}
	out := make([]patchEffectView, 0, len(effects))
	for _, effect := range effects {
		out = append(out, patchEffectView{
			Path:       effect.Path,
			OldPath:    effect.OldPath,
			ChangeKind: effect.ChangeKind,
		})
	}
	return out, nil
}

// strictTouchedPaths is the b-side path projection PI-3 and PI-4 consume.
// It is the same projection PI-12's five shipped callers receive, so a
// `touched_paths` audit list and a landed-patch file set cannot disagree
// about rename semantics.
func strictTouchedPaths(patch string) ([]string, error) {
	return gitutil.FilesInPatchStrict(patch)
}

// noveltyActionForChangeKind maps the strict change axis onto the novelty
// vocabulary. The mapping is total: every change kind the grammar can
// produce has an action, so a new arm cannot silently fall through to
// "modify".
func noveltyActionForChangeKind(kind gitutil.ChangeKind) (FileNoveltyAction, bool) {
	switch kind {
	case gitutil.ChangeKindAdd:
		return FileNoveltyActionCreate, true
	case gitutil.ChangeKindModify:
		return FileNoveltyActionModify, true
	case gitutil.ChangeKindDelete:
		return FileNoveltyActionDelete, true
	case gitutil.ChangeKindRename:
		return FileNoveltyActionRename, true
	case gitutil.ChangeKindCopy:
		// A copy creates its destination; its source is untouched.
		return FileNoveltyActionCreate, true
	default:
		return "", false
	}
}
