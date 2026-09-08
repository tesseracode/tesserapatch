package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// RecipeExclusion describes an effect for which S2 cannot derive an operation.
// It is an in-process result, not a persisted completeness or replay claim.
type RecipeExclusion struct {
	Ordinal     int
	Path        string
	ReasonCodes []string
}

// RecipeDerivation keeps its proof bytes private: callers cannot turn a
// partial recipe or a stored origin label into a successful D16 comparison.
type RecipeDerivation struct {
	recipe     ApplyRecipe
	canonical  []byte
	baseCommit string
	Exclusions []RecipeExclusion
	Reasons    []string
}

func (d RecipeDerivation) CanonicalBytes() []byte { return bytes.Clone(d.canonical) }

// ProvesOrigin compares raw on-disk bytes, not decoded or re-encoded JSON.
func (d RecipeDerivation) ProvesOrigin(existing []byte) bool {
	return len(d.canonical) > 0 && bytes.Equal(d.canonical, existing)
}

// EncodeRecipe is the single canonical encoder for derived recipes. Operation
// order is meaningful; the derivation sorts independent whole-file writes,
// but this encoder never reorders arbitrary manual/provider operations.
func EncodeRecipe(recipe ApplyRecipe) ([]byte, error) {
	if !utf8.ValidString(recipe.Feature) {
		return nil, fmt.Errorf("recipe feature is not UTF-8")
	}
	for _, op := range recipe.Operations {
		for _, field := range []string{op.Type, op.Path, op.Content, op.Search, op.Replace, op.CreatedBy} {
			if !utf8.ValidString(field) {
				return nil, fmt.Errorf("recipe operation cannot be encoded without changing bytes")
			}
		}
		if op.PreimageHash != nil && !utf8.ValidString(*op.PreimageHash) {
			return nil, fmt.Errorf("recipe preimage hash is not UTF-8")
		}
	}
	if recipe.Operations == nil {
		recipe.Operations = []RecipeOperation{}
	}
	data, err := json.MarshalIndent(recipe, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// DeriveRecipe is pure. It consumes the patch and exact image bytes captured
// before the producer's first write; it never opens the worktree or consults
// a clock, a stored recipe label, or historical provenance.
func DeriveRecipe(obs patchobs.Observation) (RecipeDerivation, error) {
	d := RecipeDerivation{recipe: ApplyRecipe{Feature: obs.Slug}}
	if err := obs.PreflightError(); err != nil {
		return d, err
	}
	if !obs.PatchPresent {
		d.Reasons = append(d.Reasons, patchobs.ReasonPatchMissing)
		return d, nil
	}
	if store.SHA256HexString(string(obs.PatchBytes)) != obs.PatchSHA256 {
		return d, fmt.Errorf("recipe derivation: captured patch bytes changed")
	}
	effects, err := gitutil.NormalizePatchEffects(string(obs.PatchBytes))
	if err != nil {
		return d, err
	}
	if len(effects) != len(obs.Effects) || patchobs.PreimageSetDigest(obs.Effects) != obs.Reference.PreimageSetSHA256 {
		return d, fmt.Errorf("recipe derivation: captured effect set changed")
	}
	if len(effects) == 0 {
		d.Reasons = append(d.Reasons, patchobs.ReasonPatchEmpty)
	}
	if obs.Reference.Kind != patchobs.ReferenceKindCommit {
		d.Reasons = append(d.Reasons, patchobs.ReasonReferenceNotDurable)
	} else if !fullRecipeBaseCommit(obs.Reference.Commit) {
		return d, fmt.Errorf("recipe derivation: invalid captured commit")
	}
	parents := make(map[string]bool, len(obs.ParentCreatedPaths))
	for _, target := range obs.ParentCreatedPaths {
		// Match execution's Join semantics without statting or trimming
		// filenames. Claim-path helpers trim whitespace and inspect disk.
		relative, err := filepath.Rel(obs.RepoRoot, filepath.Join(obs.RepoRoot, target))
		if err != nil {
			return d, fmt.Errorf("recipe derivation: parent target: %w", err)
		}
		parents[filepath.ToSlash(relative)] = true
	}
	for i, entry := range obs.Effects {
		e, parsed := entry.Effect, effects[i]
		if e.Ordinal != parsed.Ordinal || e.Path != parsed.Path || e.OldPath != parsed.OldPath ||
			e.ChangeKind != parsed.ChangeKind || e.HeaderOldMode != parsed.HeaderOldMode ||
			e.HeaderNewMode != parsed.HeaderNewMode || e.BinaryStanza != parsed.BinaryStanza ||
			e.FragmentStart != parsed.FragmentStart || e.FragmentEnd != parsed.FragmentEnd ||
			e.FragmentSHA256 != parsed.FragmentSHA256 {
			return d, fmt.Errorf("recipe derivation: effect %d does not bind the captured patch", i+1)
		}
		preExtant, postExtant := e.ExtantSides()
		if err := validateRecipeSide(e.PreimageObserved, e.PreimagePresent, preExtant,
			e.OldMode, parsed.HeaderOldMode, e.PreimageSHA256, entry.Bytes.Preimage); err != nil {
			return d, fmt.Errorf("recipe derivation: %s preimage: %w", e.Path, err)
		}
		if err := validateRecipeSide(e.PostimageObserved, e.PostimagePresent, postExtant,
			e.NewMode, parsed.HeaderNewMode, e.PostimageSHA256, entry.Bytes.Postimage); err != nil {
			return d, fmt.Errorf("recipe derivation: %s postimage: %w", e.Path, err)
		}
		reasons := recipeEffectExclusions(entry, parents[e.Path])
		if len(reasons) != 0 {
			d.Exclusions = append(d.Exclusions, RecipeExclusion{e.Ordinal, e.Path, reasons})
			continue
		}
		preimage := ""
		if e.PreimagePresent {
			preimage = "sha256:" + e.PreimageSHA256
		}
		d.recipe.Operations = append(d.recipe.Operations, RecipeOperation{
			Type: "write-file", Path: e.Path, Content: string(entry.Bytes.Postimage),
			PreimageHash: &preimage,
		})
	}
	sort.Strings(d.Reasons)
	if len(d.Reasons) != 0 || len(d.Exclusions) != 0 {
		// A partial derivation is diagnostic only; never offer its bytes
		// as a recipe or as origin proof.
		d.recipe.Operations = nil
		return d, nil
	}
	sort.Slice(d.recipe.Operations, func(i, j int) bool {
		return d.recipe.Operations[i].Path < d.recipe.Operations[j].Path
	})
	d.canonical, err = EncodeRecipe(d.recipe)
	if err != nil {
		return RecipeDerivation{}, err
	}
	d.baseCommit = obs.Reference.Commit
	return d, nil
}

func fullRecipeBaseCommit(commit string) bool {
	return len(commit) == 40 && strings.Trim(commit, "0123456789abcdef") == ""
}

func validateRecipeSide(observed, present, extant bool, mode, headerMode, hash string, body []byte) error {
	if !observed {
		if present || mode != "" || hash != "" || len(body) != 0 {
			return fmt.Errorf("unobserved side carries fabricated evidence")
		}
		return nil
	}
	if present != extant {
		return fmt.Errorf("observed presence contradicts the patch change kind")
	}
	if !present {
		if mode != "" || hash != "" || len(body) != 0 {
			return fmt.Errorf("absent side carries object evidence")
		}
		return nil
	}
	if gitutil.ObjectKindForMode(mode) == gitutil.ObjectKindUnknown {
		return fmt.Errorf("observed side has an unsupported mode")
	}
	if headerMode != "" && mode != headerMode {
		return fmt.Errorf("observed mode contradicts the captured patch header")
	}
	// Gitlinks bind commit identities, not file bodies.
	if mode != gitutil.ModeGitlink && store.SHA256HexString(string(body)) != hash {
		return fmt.Errorf("captured image bytes changed or were released")
	}
	return nil
}

func recipeEffectExclusions(entry patchobs.EffectObservation, parentCreated bool) []string {
	e := entry.Effect
	var reasons []string
	switch e.ChangeKind {
	case gitutil.ChangeKindDelete:
		reasons = append(reasons, "effect-delete-unsupported")
	case gitutil.ChangeKindRename:
		reasons = append(reasons, "effect-rename-unsupported")
	case gitutil.ChangeKindCopy:
		reasons = append(reasons, "effect-copy-unsupported")
	}
	for _, mode := range []string{e.OldMode, e.NewMode} {
		switch mode {
		case gitutil.ModeExecutable:
			reasons = append(reasons, "effect-executable-unsupported")
		case gitutil.ModeSymlink:
			reasons = append(reasons, "effect-symlink-unsupported")
		case gitutil.ModeGitlink:
			reasons = append(reasons, "effect-gitlink-unsupported")
		}
	}
	_, postExtant := e.ExtantSides()
	gitlinkContent := (e.PostimageObserved && e.PostimagePresent && e.NewMode == gitutil.ModeGitlink) ||
		(!postExtant && e.PreimageObserved && e.PreimagePresent && e.OldMode == gitutil.ModeGitlink)
	if !gitlinkContent && (e.BinaryStanza ||
		bytes.IndexByte(entry.Bytes.Preimage, 0) >= 0 || bytes.IndexByte(entry.Bytes.Postimage, 0) >= 0) {
		reasons = append(reasons, "effect-binary-unsupported")
	}
	if e.ChangeKind == gitutil.ChangeKindModify && e.PreimageObserved && e.PostimageObserved &&
		e.PreimageSHA256 == e.PostimageSHA256 && e.OldMode != e.NewMode {
		reasons = append(reasons, "effect-mode-only-unsupported")
	}
	if !e.PreimageObserved {
		reasons = append(reasons, patchobs.ReasonPreimageUnavailable)
	}
	if !e.PostimageObserved {
		reasons = append(reasons, patchobs.ReasonPostimageUnavailable)
	}
	if parentCreated && !e.PreimagePresent {
		reasons = append(reasons, "parent-created-target-unsupported")
	}
	sort.Strings(reasons)
	return slices.Compact(reasons)
}

func (d RecipeDerivation) skippedPaths() []string {
	var skipped []string
	for _, exclusion := range d.Exclusions {
		skipped = append(skipped, fmt.Sprintf("%s (%s)", exclusion.Path, strings.Join(exclusion.ReasonCodes, ", ")))
	}
	skipped = append(skipped, d.Reasons...)
	return skipped
}
