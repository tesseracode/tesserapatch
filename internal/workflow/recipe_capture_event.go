package workflow

import (
	"bytes"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
)

// RecipeCaptureEvent is ADR-041's independent, unkeyed event consistency
// evidence. It contains no source bodies, origin assertion or replay decision.
type RecipeCaptureEvent struct {
	SchemaVersion      int                        `json:"schema_version"`
	Feature            string                     `json:"feature"`
	PatchPresent       bool                       `json:"patch_present"`
	PatchSHA256        string                     `json:"patch_sha256"`
	RecipePresent      bool                       `json:"recipe_present"`
	RecipeSHA256       string                     `json:"recipe_sha256"`
	Reference          CoverageReference          `json:"reference"`
	Capture            CoverageCapture            `json:"capture"`
	Event              RecipeCaptureEventFacts    `json:"event"`
	ParentCreatedPaths []string                   `json:"parent_created_paths"`
	Observations       []RecipeCaptureObservation `json:"observations"`
	CoverageSHA256     string                     `json:"coverage_sha256"`
}

type RecipeCaptureEventFacts struct {
	PatchRewritten      bool `json:"patch_rewritten"`
	RecipeRegenerated   bool `json:"recipe_regenerated"`
	BoundArtifactEdited bool `json:"bound_artifact_edited"`
	StaleMarkerPresent  bool `json:"stale_marker_present"`
}

type RecipeCaptureObservation struct {
	Ordinal             int                 `json:"ordinal"`
	ChangeKind          gitutil.ChangeKind  `json:"change_kind"`
	ContentKind         gitutil.ContentKind `json:"content_kind"`
	ObjectKind          gitutil.ObjectKind  `json:"object_kind"`
	Path                string              `json:"path"`
	OldPath             string              `json:"old_path"`
	OldMode             string              `json:"old_mode"`
	NewMode             string              `json:"new_mode"`
	PreimageObserved    bool                `json:"preimage_observed"`
	PreimagePresent     bool                `json:"preimage_present"`
	PreimageSHA256      string              `json:"preimage_sha256"`
	PostimageObserved   bool                `json:"postimage_observed"`
	PostimagePresent    bool                `json:"postimage_present"`
	PostimageSHA256     string              `json:"postimage_sha256"`
	PatchFragmentSHA256 string              `json:"patch_fragment_sha256"`
}

// RecipeCaptureBindings contains separately captured artifacts, not fields
// recovered from either member of a stored pair.
type RecipeCaptureBindings struct {
	RepoRoot string
	Feature  string
	Patch    CoverageArtifact
	Recipe   CoverageArtifact
}

func recipeCaptureInput(in RecipeCoverageInput) (RecipeCoverageInput, error) {
	parents := slices.Clone(in.Observation.ParentCreatedPaths)
	if recipe, err := decodeCoverageRecipe(in.Recipe.Bytes); in.Recipe.Present && err == nil {
		for _, op := range recipe.Operations {
			if op.CreatedBy != "" {
				parents = append(parents, op.Path)
			}
		}
	}
	normalized := make([]string, 0, len(parents))
	for _, path := range parents {
		if filepath.IsAbs(path) || strings.HasPrefix(path, "/") || strings.HasPrefix(path, `\`) ||
			(len(path) >= 3 && path[1] == ':' && (path[2] == '/' || path[2] == '\\')) {
			return RecipeCoverageInput{}, fmt.Errorf("capture event: absolute parent target is forbidden")
		}
		target, err := coveragePath(in.Observation.RepoRoot, path)
		if err != nil || coverageEffectPathUnsafe(CoverageEffect{Path: target}) {
			return RecipeCoverageInput{}, fmt.Errorf("capture event: invalid parent target %q", path)
		}
		normalized = append(normalized, target)
	}
	in.Observation.ParentCreatedPaths = coverageSortedCopy(normalized)
	return in, nil
}

func recipeCaptureProjection(effect gitutil.PatchEffect) RecipeCaptureObservation {
	return RecipeCaptureObservation{
		Ordinal: effect.Ordinal, ChangeKind: effect.ChangeKind,
		ContentKind: effect.ContentKind, ObjectKind: effect.ObjectKind,
		Path: effect.Path, OldPath: effect.OldPath, OldMode: effect.OldMode, NewMode: effect.NewMode,
		PreimageObserved: effect.PreimageObserved, PreimagePresent: effect.PreimagePresent, PreimageSHA256: effect.PreimageSHA256,
		PostimageObserved: effect.PostimageObserved, PostimagePresent: effect.PostimagePresent, PostimageSHA256: effect.PostimageSHA256,
		PatchFragmentSHA256: effect.FragmentSHA256,
	}
}

func (o RecipeCaptureObservation) patchEffect() gitutil.PatchEffect {
	return gitutil.PatchEffect{
		Ordinal: o.Ordinal, ChangeKind: o.ChangeKind, ContentKind: o.ContentKind, ObjectKind: o.ObjectKind,
		Path: o.Path, OldPath: o.OldPath, OldMode: o.OldMode, NewMode: o.NewMode,
		PreimageObserved: o.PreimageObserved, PreimagePresent: o.PreimagePresent, PreimageSHA256: o.PreimageSHA256,
		PostimageObserved: o.PostimageObserved, PostimagePresent: o.PostimagePresent, PostimageSHA256: o.PostimageSHA256,
		FragmentSHA256: o.PatchFragmentSHA256,
	}
}

func (e RecipeCaptureEvent) events() CoverageEvents {
	return CoverageEvents{
		PatchRewritten: e.Event.PatchRewritten, RecipeRegenerated: e.Event.RecipeRegenerated,
		BoundArtifactEdited: e.Event.BoundArtifactEdited, StaleMarkerPresent: e.Event.StaleMarkerPresent,
	}
}

// BuildRecipeCaptureEvent projects independent frozen inputs. Coverage bytes
// contribute only their raw pairing digest; no descriptor is copied from C.
func BuildRecipeCaptureEvent(in RecipeCoverageInput, coverageBytes []byte) (RecipeCaptureEvent, error) {
	in, err := recipeCaptureInput(in)
	if err != nil {
		return RecipeCaptureEvent{}, err
	}
	if _, err := coverageObservation(in.Observation); err != nil {
		return RecipeCaptureEvent{}, fmt.Errorf("capture event: %w", err)
	}
	if err := validateCaptureArtifact(in.Recipe); err != nil {
		return RecipeCaptureEvent{}, err
	}
	obs := in.Observation
	e := RecipeCaptureEvent{
		SchemaVersion: 1, Feature: obs.Slug,
		PatchPresent: obs.PatchPresent, PatchSHA256: obs.PatchSHA256,
		RecipePresent: in.Recipe.Present,
		Reference:     CoverageReference{obs.Reference.Kind, obs.Reference.Commit, obs.Reference.PreimageSetSHA256},
		Capture:       CoverageCapture{obs.Capture.Mode, coverageSortedCopy(obs.Capture.Pathspecs), coverageSortedCopy(obs.Capture.ClaimIDs)},
		Event: RecipeCaptureEventFacts{
			PatchRewritten: in.Events.PatchRewritten, RecipeRegenerated: in.Events.RecipeRegenerated,
			BoundArtifactEdited: in.Events.BoundArtifactEdited, StaleMarkerPresent: in.Events.StaleMarkerPresent,
		},
		ParentCreatedPaths: slices.Clone(obs.ParentCreatedPaths),
		Observations:       make([]RecipeCaptureObservation, 0, len(obs.Effects)),
		CoverageSHA256:     CoverageSHA256(coverageBytes),
	}
	if in.Recipe.Present {
		e.RecipeSHA256 = CoverageSHA256(in.Recipe.Bytes)
	}
	for _, entry := range obs.Effects {
		e.Observations = append(e.Observations, recipeCaptureProjection(entry.Effect))
	}
	if err := ValidateRecipeCaptureEventSchema(e); err != nil {
		return RecipeCaptureEvent{}, err
	}
	return e, nil
}

func validateCaptureArtifact(a CoverageArtifact) error {
	if (a.Present && a.ReadError != nil) || (!a.Present && len(a.Bytes) != 0) {
		return fmt.Errorf("capture event: contradictory readable artifact at %s", a.Path)
	}
	return nil
}

// ValidateRecipeCaptureEventSource is a producer-side proof against actual
// retained images. Unlike pair consistency, this runs the unchanged S3 core.
func ValidateRecipeCaptureEventSource(e RecipeCaptureEvent, coverageBytes []byte, in RecipeCoverageInput) error {
	in, err := recipeCaptureInput(in)
	if err != nil {
		return err
	}
	want, err := BuildRecipeCaptureEvent(in, coverageBytes)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(e, want) {
		return fmt.Errorf("capture event: evidence differs from independently captured inputs")
	}
	c, err := BuildRecipeCoverage(in)
	if err != nil {
		return fmt.Errorf("capture event: source coverage: %w", err)
	}
	canonical, err := EncodeRecipeCoverage(c)
	if err != nil {
		return err
	}
	if !bytes.Equal(canonical, coverageBytes) {
		return fmt.Errorf("capture event: paired coverage differs from canonical source derivation")
	}
	return nil
}

// ValidateRecipeCaptureEventPair proves strict consistency and exact raw-byte
// bindings only. It is not independent reconstruction of historical bodies.
func ValidateRecipeCaptureEventPair(e RecipeCaptureEvent, coverageBytes []byte, bindings RecipeCaptureBindings) error {
	if !filepath.IsAbs(bindings.RepoRoot) {
		return fmt.Errorf("capture event: repository root must be absolute")
	}
	if err := ValidateRecipeCaptureEventSchema(e); err != nil {
		return err
	}
	c, err := DecodeRecipeCoverage(coverageBytes)
	if err != nil {
		return fmt.Errorf("capture event: paired coverage is invalid: %w", err)
	}
	if c.Feature != bindings.Feature || e.Feature != bindings.Feature {
		return fmt.Errorf("capture event: requested feature owner mismatch")
	}
	for _, bound := range []struct {
		name               string
		actual             CoverageArtifact
		ePresent, cPresent bool
		eHash, cHash       string
	}{
		{"patch", bindings.Patch, e.PatchPresent, c.PatchPresent, e.PatchSHA256, c.PatchSHA256},
		{"recipe", bindings.Recipe, e.RecipePresent, c.RecipePresent, e.RecipeSHA256, c.RecipeSHA256},
	} {
		if err := validateCaptureArtifact(bound.actual); err != nil {
			return err
		}
		hash := ""
		if bound.actual.Present {
			hash = CoverageSHA256(bound.actual.Bytes)
		}
		if bound.ePresent != bound.actual.Present || bound.cPresent != bound.actual.Present ||
			bound.eHash != hash || bound.cHash != hash {
			return fmt.Errorf("capture event: %s readable presence or exact hash differs", bound.name)
		}
	}
	recipe, decodeErr := decodeCoverageRecipe(bindings.Recipe.Bytes)
	if c.RecipeDecodable != (bindings.Recipe.Present && decodeErr == nil) {
		return fmt.Errorf("capture event: strict recipe decodability differs")
	}
	if !reflect.DeepEqual(e.Capture, c.Capture) {
		return fmt.Errorf("capture event: capture descriptor differs")
	}
	if e.Reference != c.Reference {
		return fmt.Errorf("capture event: reference descriptor differs")
	}
	if len(e.Observations) != len(c.Effects) {
		return fmt.Errorf("capture event: observation count differs")
	}
	projected := patchobs.Observation{RepoRoot: bindings.RepoRoot, ParentCreatedPaths: e.ParentCreatedPaths}
	for i, observed := range e.Observations {
		if observed != recipeCaptureProjectionFromCoverage(c.Effects[i]) {
			return fmt.Errorf("capture event: observation %d differs", i+1)
		}
		projected.Effects = append(projected.Effects, patchobs.EffectObservation{Effect: observed.patchEffect()})
	}
	parsed, parseErr := gitutil.NormalizePatchEffects(string(bindings.Patch.Bytes))
	switch {
	case !bindings.Patch.Present || parseErr != nil:
		if len(e.Observations) != 0 {
			return fmt.Errorf("capture event: absent or unparseable patch carries observations")
		}
	default:
		if len(parsed) != len(e.Observations) {
			return fmt.Errorf("capture event: strict patch effect count differs")
		}
		for i, p := range parsed {
			o := e.Observations[i]
			if p.Ordinal != o.Ordinal || p.ChangeKind != o.ChangeKind || p.Path != o.Path || p.OldPath != o.OldPath ||
				p.FragmentSHA256 != o.PatchFragmentSHA256 ||
				(o.PreimageObserved && o.PreimagePresent && p.HeaderOldMode != "" && p.HeaderOldMode != o.OldMode) ||
				(o.PostimageObserved && o.PostimagePresent && p.HeaderNewMode != "" && p.HeaderNewMode != o.NewMode) {
				return fmt.Errorf("capture event: observation %d differs from the strict patch", i+1)
			}
		}
	}
	has := func(reason string) bool { return slices.Contains(c.Reasons, reason) }
	if has("canonical-patch-unparseable") != (bindings.Patch.Present && parseErr != nil) ||
		has("canonical-patch-empty") != (bindings.Patch.Present && parseErr == nil && len(parsed) == 0) ||
		has("recipe-owner-mismatch") != (c.RecipeDecodable && recipe.Feature != c.Feature) {
		return fmt.Errorf("capture event: artifact-derived coverage reasons differ")
	}
	parents, err := coverageParents(projected)
	if err != nil {
		return err
	}
	if c.RecipeDecodable {
		for _, op := range recipe.Operations {
			if op.CreatedBy != "" {
				path, err := coveragePath(bindings.RepoRoot, op.Path)
				if err != nil || !parents[path] {
					return fmt.Errorf("capture event: parent exclusion omits final recipe created_by target")
				}
			}
		}
	}
	assignments, surplus, err := coverageAssignments(projected, recipe)
	if err != nil {
		return err
	}
	explains := bindings.Patch.Present && parseErr == nil && len(parsed) != 0 &&
		c.RecipeDecodable && recipe.Feature == c.Feature && !surplus && !has("simulation-mismatch")
	for i, effect := range c.Effects {
		if !slices.Equal(assignments[i], effect.OperationIndexes) {
			return fmt.Errorf("capture event: operation assignments differ at observation %d", i+1)
		}
		path, _ := coveragePath(bindings.RepoRoot, effect.Path)
		unreclassifiable := false
		for _, n := range assignments[i] {
			unreclassifiable = unreclassifiable || !coverageAdmissibleReclassification(recipe.Operations[n-1])
		}
		reasons := coverageLocalReasons(effect, parents[path] && !effect.PreimagePresent,
			unreclassifiable, coverageEffectPathUnsafe(effect))
		if !slices.Equal(reasons, effect.ReasonCodes) {
			return fmt.Errorf("capture event: independent operation or parent-exclusion reasons differ at observation %d", i+1)
		}
		explains = explains && effect.Disposition == "represented"
	}
	if has("operation-surplus") != (c.RecipeDecodable && surplus) ||
		has("recipe-stale-marker-present") != e.Event.StaleMarkerPresent ||
		has("producer-patch-rewrite") != (e.Event.PatchRewritten && e.RecipePresent && !e.Event.RecipeRegenerated && !explains) ||
		has("manual-bound-artifact-edit") != (e.Event.BoundArtifactEdited && (!explains || e.Reference.Kind != patchobs.ReferenceKindCommit)) ||
		(e.Event.BoundArtifactEdited && c.Producer != patchobs.ProducerEdit) {
		return fmt.Errorf("capture event: event-derived semantic conditions differ")
	}
	if e.CoverageSHA256 != CoverageSHA256(coverageBytes) {
		return fmt.Errorf("capture event: exact raw coverage pairing hash differs")
	}
	return nil
}

func recipeCaptureProjectionFromCoverage(e CoverageEffect) RecipeCaptureObservation {
	return RecipeCaptureObservation{
		Ordinal: e.Ordinal, ChangeKind: e.ChangeKind, ContentKind: e.ContentKind, ObjectKind: e.ObjectKind,
		Path: e.Path, OldPath: e.OldPath, OldMode: e.OldMode, NewMode: e.NewMode,
		PreimageObserved: e.PreimageObserved, PreimagePresent: e.PreimagePresent, PreimageSHA256: e.PreimageSHA256,
		PostimageObserved: e.PostimageObserved, PostimagePresent: e.PostimagePresent, PostimageSHA256: e.PostimageSHA256,
		PatchFragmentSHA256: e.PatchFragmentSHA256,
	}
}
