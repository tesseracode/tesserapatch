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
	Action            AutogenAction
	SkippedPaths      []string
	DriftReason       string
	Operations        int
	OriginProved      bool
	ProvenanceWritten bool
}

// AutogenRecipeForRecord consumes the producer's pre-write observation.
// Exact canonical equality, not the action or file set, licenses provenance.
// Explicit regeneration may replace a recipe only with a complete derivation.
func AutogenRecipeForRecord(s *store.Store, obs patchobs.Observation, autogen, regenerate bool) (AutogenOutcome, error) {
	if s.Root != obs.RepoRoot {
		return AutogenOutcome{}, fmt.Errorf("recipe observation belongs to a different repository")
	}
	slug := obs.Slug
	existing, recipeErr := s.ReadFeatureFile(slug, filepath.Join("artifacts", "apply-recipe.json"))
	if recipeErr != nil && !errors.Is(recipeErr, fs.ErrNotExist) {
		return AutogenOutcome{}, fmt.Errorf("read existing recipe: %w", recipeErr)
	}
	haveExisting := recipeErr == nil
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
		return AutogenOutcome{}, err
	}
	outcome := AutogenOutcome{SkippedPaths: derived.skippedPaths(), Operations: len(derived.recipe.Operations)}
	switch {
	case len(derived.canonical) == 0:
		outcome.DriftReason = "captured effects cannot produce a complete recipe; existing recipe preserved"
		if haveExisting {
			return markRecipeStale(s, slug, outcome)
		}
		outcome.Action = AutogenSkipped
		return outcome, nil
	case !haveExisting:
		if !autogen && !regenerate {
			outcome.Action = AutogenSkipped
			return outcome, nil
		}
		if err := writeRecipe(s, slug, derived.recipe); err != nil {
			return outcome, err
		}
		existing = string(derived.canonical)
		outcome.Action = AutogenGenerated
	case derived.ProvesOrigin([]byte(existing)):
		outcome.Action = AutogenNoop
	case regenerate:
		if err := writeRecipe(s, slug, derived.recipe); err != nil {
			return outcome, err
		}
		existing = string(derived.canonical)
		outcome.Action = AutogenRegenerated
	default:
		outcome.DriftReason = "recipe bytes differ from the complete captured derivation; preserved without origin proof (GH #19 owns historical/manual adoption)"
		return markRecipeStale(s, slug, outcome)
	}
	outcome.OriginProved = derived.ProvesOrigin([]byte(existing))
	outcome.ProvenanceWritten, err = convergeRecipeProvenance(s, slug, derived, []byte(existing))
	if err != nil {
		return outcome, err
	}
	if err := clearStaleMarker(s, slug); err != nil {
		return outcome, err
	}
	return outcome, nil
}

func markRecipeStale(s *store.Store, slug string, outcome AutogenOutcome) (AutogenOutcome, error) {
	sb := RecipeStaleness{
		Stale:      true,
		Reason:     outcome.DriftReason,
		DetectedAt: time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.MarshalIndent(sb, "", "  ")
	if err := s.WriteArtifact(slug, "recipe-stale.json", string(data)+"\n"); err != nil {
		return outcome, err
	}
	outcome.Action = AutogenStale
	return outcome, nil
}

func writeRecipe(s *store.Store, slug string, recipe ApplyRecipe) error {
	data, err := EncodeRecipe(recipe)
	if err != nil {
		return err
	}
	return s.WriteArtifact(slug, "apply-recipe.json", string(data))
}

func convergeRecipeProvenance(s *store.Store, slug string, derived RecipeDerivation, existing []byte) (bool, error) {
	if !derived.ProvesOrigin(existing) || !fullRecipeBaseCommit(derived.baseCommit) {
		return false, nil
	}
	hasPreimage := false
	for _, op := range derived.recipe.Operations {
		if op.PreimageHash != nil {
			hasPreimage = true
			break
		}
	}
	if !hasPreimage {
		return false, nil
	}
	hash := store.SHA256HexString(string(existing))
	raw, err := s.ReadFeatureFile(slug, "artifacts/recipe-provenance.json")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("read recipe provenance: %w", err)
	}
	var prior RecipeProvenance
	if err == nil && json.Unmarshal([]byte(raw), &prior) == nil &&
		prior.BaseCommit == derived.baseCommit && prior.RecipeSHA256 != nil && *prior.RecipeSHA256 == hash {
		if _, timeErr := time.Parse(time.RFC3339, prior.GeneratedAt); timeErr == nil {
			return false, nil
		}
	}
	prov := RecipeProvenance{
		BaseCommit: derived.baseCommit, GeneratedAt: time.Now().UTC().Format(time.RFC3339), RecipeSHA256: &hash,
	}
	data, err := json.MarshalIndent(prov, "", "  ")
	if err != nil {
		return false, err
	}
	if err := s.WriteArtifact(slug, "recipe-provenance.json", string(data)+"\n"); err != nil {
		return false, fmt.Errorf("write recipe provenance: %w", err)
	}
	return true, nil
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
