package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// RecipeStaleness is the recipe-stale.json sidecar written when
// `tpatch record` detects drift between the existing apply-recipe.json
// and the patch it just captured (typically after a `--manual`
// implement flow). The recipe itself is left untouched so a richer
// provider-generated recipe is not destroyed; the sidecar tells the
// user the recipe no longer matches the recorded patch and how to
// resolve it.
//
// Held as a sidecar (not a field on ApplyRecipe) so the skill-parity
// guard's DisallowUnknownFields check stays passing without touching
// the 6 skill assets.
type RecipeStaleness struct {
	Stale      bool   `json:"stale"`
	Reason     string `json:"reason"`
	DetectedAt string `json:"detected_at"`
}

// RecipeFromPatch returns only a complete recipe from an immutable capture.
// Unsupported effects are reported without returning a lossy partial recipe.
func RecipeFromPatch(obs patchobs.Observation) (ApplyRecipe, []string, error) {
	derived, err := DeriveRecipe(obs)
	if err != nil {
		return ApplyRecipe{}, nil, err
	}
	return derived.recipe, derived.skippedPaths(), nil
}

// AutogenAction enumerates the outcomes of AutogenRecipeForRecord.
type AutogenAction string

const (
	AutogenGenerated   AutogenAction = "generated"   // no recipe → wrote a derived one
	AutogenRegenerated AutogenAction = "regenerated" // existing recipe overwritten (regenerate=true)
	AutogenStale       AutogenAction = "stale"       // existing recipe drifted → sidecar written
	AutogenNoop        AutogenAction = "noop"        // existing recipe matches captured patch
	AutogenSkipped     AutogenAction = "skipped"     // autogen disabled and no recipe present
)

// AutogenOutcome is what AutogenRecipeForRecord decided, together with
// the operation count of the recipe it actually derived.
//
// Operations exists because a `diff --git` prefix counter cannot compute
// one: it counts file records, so it double-counts a path with two
// records and counts a line that never produced an operation
// (PRD §6.1.3, PI-10). The producer that derived the recipe is the only
// caller holding a real operation count, so it reports it here.
type AutogenOutcome struct {
	Action             AutogenAction
	SkippedPaths       []string
	DriftReason        string
	Operations         int
	OriginProved       bool
	ProvenanceWritten  bool
	Recipe             CoverageArtifact
	RecipeWritten      bool
	StaleMarkerPresent bool
	plan               *recordRecipePlan
}

type recordRecipePlan struct {
	recipe     []byte
	provenance []byte
	stale      []byte
	clearStale bool
}

// AutogenRecipeForRecord consumes the producer's pre-write observation.
// Exact canonical equality, not the action or file set, licenses provenance.
// Explicit regeneration may replace a recipe only with a complete derivation.
func AutogenRecipeForRecord(s *store.Store, obs patchobs.Observation, autogen, regenerate bool, captured ...CoveragePublicationInput) (outcome AutogenOutcome, retErr error) {
	if s.Root != obs.RepoRoot {
		return AutogenOutcome{}, fmt.Errorf("recipe observation belongs to a different repository")
	}
	var publication CoveragePublicationInput
	if len(captured) != 0 {
		publication = captured[0]
	} else {
		publication = ObserveCoveragePublication(s, obs)
	}
	if publication.observationErr != nil {
		return AutogenOutcome{}, publication.observationErr
	}
	existing := string(publication.Recipe.Bytes)
	recipeErr := publication.Recipe.ReadError
	if recipeErr != nil && !errors.Is(recipeErr, fs.ErrNotExist) {
		return AutogenOutcome{Recipe: publication.Recipe, StaleMarkerPresent: publication.Events.StaleMarkerPresent}, fmt.Errorf("read existing recipe: %w", recipeErr)
	}
	haveExisting := publication.Recipe.Present
	outcome = AutogenOutcome{Recipe: publication.Recipe, StaleMarkerPresent: publication.Events.StaleMarkerPresent, plan: &recordRecipePlan{}}
	defer func() {
		if retErr == nil && !publication.DeferRecipeWrites {
			retErr = publishRecordRecipePlan(s, obs.Slug, &outcome)
		}
	}()
	// This only narrows derivation. A declared parent is never a substitute
	// for a missing captured preimage, even during explicit regeneration.
	var prior ApplyRecipe
	if haveExisting && json.Unmarshal([]byte(existing), &prior) == nil {
		obs.ParentCreatedPaths = slices.Clone(obs.ParentCreatedPaths)
		for _, op := range prior.Operations {
			if op.CreatedBy != "" {
				obs.ParentCreatedPaths = append(obs.ParentCreatedPaths, op.Path)
			}
		}
	}
	derived, err := DeriveRecipe(obs)
	if err != nil {
		return outcome, err
	}
	outcome.SkippedPaths, outcome.Operations = derived.skippedPaths(), len(derived.recipe.Operations)
	switch {
	case len(derived.canonical) == 0:
		outcome.DriftReason = "captured effects cannot produce a complete recipe; existing recipe preserved"
		if haveExisting {
			return planRecipeStale(outcome)
		}
		outcome.Action = AutogenSkipped
		return outcome, nil
	case !haveExisting:
		if !autogen && !regenerate {
			outcome.Action = AutogenSkipped
			return outcome, nil
		}
		outcome.plan.recipe = derived.CanonicalBytes()
		existing = string(derived.canonical)
		outcome.Recipe = CoverageArtifact{Present: true, Bytes: derived.CanonicalBytes(), Path: publication.Recipe.Path}
		outcome.RecipeWritten = true
		outcome.Action = AutogenGenerated
	case derived.ProvesOrigin([]byte(existing)):
		outcome.Action = AutogenNoop
	case regenerate:
		outcome.plan.recipe = derived.CanonicalBytes()
		existing = string(derived.canonical)
		outcome.Recipe = CoverageArtifact{Present: true, Bytes: derived.CanonicalBytes(), Path: publication.Recipe.Path}
		outcome.RecipeWritten = true
		outcome.Action = AutogenRegenerated
	default:
		outcome.DriftReason = "recipe bytes differ from the complete captured derivation; preserved without origin proof (GH #19 owns historical/manual adoption)"
		return planRecipeStale(outcome)
	}
	outcome.OriginProved = derived.ProvesOrigin([]byte(existing))
	outcome.plan.provenance, err = convergeRecipeProvenance(derived, []byte(existing), publication.Provenance)
	if err != nil {
		return outcome, err
	}
	outcome.plan.clearStale = true
	outcome.StaleMarkerPresent = false
	return outcome, nil
}

func planRecipeStale(outcome AutogenOutcome) (AutogenOutcome, error) {
	sb := RecipeStaleness{
		Stale:      true,
		Reason:     outcome.DriftReason,
		DetectedAt: time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.MarshalIndent(sb, "", "  ")
	outcome.plan.stale = append(data, '\n')
	outcome.Action = AutogenStale
	outcome.StaleMarkerPresent = true
	return outcome, nil
}

func writeRecipe(s *store.Store, slug string, data []byte) error {
	return s.WriteArtifactAtomic(slug, "apply-recipe.json", string(data))
}

func convergeRecipeProvenance(derived RecipeDerivation, existing []byte, priorArtifact CoverageArtifact) ([]byte, error) {
	if !derived.ProvesOrigin(existing) || !fullRecipeBaseCommit(derived.baseCommit) {
		return nil, nil
	}
	hasPreimage := false
	for _, op := range derived.recipe.Operations {
		if op.PreimageHash != nil {
			hasPreimage = true
			break
		}
	}
	if !hasPreimage {
		return nil, nil
	}
	hash := store.SHA256HexString(string(existing))
	if priorArtifact.ReadError != nil {
		return nil, fmt.Errorf("read recipe provenance: %w", priorArtifact.ReadError)
	}
	var prior RecipeProvenance
	if priorArtifact.Present && json.Unmarshal(priorArtifact.Bytes, &prior) == nil &&
		prior.BaseCommit == derived.baseCommit && prior.RecipeSHA256 != nil && *prior.RecipeSHA256 == hash {
		if _, timeErr := time.Parse(time.RFC3339, prior.GeneratedAt); timeErr == nil {
			return nil, nil
		}
	}
	prov := RecipeProvenance{
		BaseCommit: derived.baseCommit, GeneratedAt: time.Now().UTC().Format(time.RFC3339), RecipeSHA256: &hash,
	}
	data, err := json.MarshalIndent(prov, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func publishRecordRecipePlan(s *store.Store, slug string, outcome *AutogenOutcome) error {
	plan := outcome.plan
	if plan.recipe != nil {
		if err := writeRecipe(s, slug, plan.recipe); err != nil {
			return err
		}
	}
	if plan.provenance != nil {
		if err := s.WriteArtifactAtomic(slug, "recipe-provenance.json", string(plan.provenance)); err != nil {
			return fmt.Errorf("write recipe provenance: %w", err)
		}
		outcome.ProvenanceWritten = true
	}
	if plan.stale != nil {
		if err := s.WriteArtifactAtomic(slug, "recipe-stale.json", string(plan.stale)); err != nil {
			return err
		}
	}
	if plan.clearStale {
		if err := clearStaleMarker(s, slug); err != nil {
			return err
		}
	}
	outcome.plan = nil
	return nil
}

func clearStaleMarker(s *store.Store, slug string) error {
	// Path layout is fixed by the store: .tpatch/features/<slug>/artifacts/.
	// Same convention used by recipe-provenance.json reads in cobra.go.
	stalePath := filepath.Join(s.Root, ".tpatch", "features", slug, "artifacts", "recipe-stale.json")
	if err := os.Remove(stalePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove stale recipe marker: %w", err)
	}
	return nil
}
