package workflow

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// CoveragePublicationInput binds one event to its immutable pre-write capture
// and exact final recipe bytes. Events describe successful writes/checkpoints,
// not intended writes or producer labels.
type CoveragePublicationInput struct {
	Observation       patchobs.Observation
	Recipe            CoverageArtifact
	Provenance        CoverageArtifact
	Events            CoverageEvents
	Autogen           *AutogenOutcome
	Generation        *PatchGenerationInput
	DeferRecipeWrites bool
	priorCoverage     []byte
	priorPatchHash    string
	priorRecipeHash   string
	observationErr    error
}

// ErrCoveragePublication distinguishes an owed publication failure from
// pre-event/best-effort refresh diagnostics, without inspecting error text.
var ErrCoveragePublication = errors.New("coverage publication failed")

// ObserveCoveragePublication snapshots the other bound input before a producer
// starts writing. It does not emit an event or publish anything.
func ObserveCoveragePublication(s *store.Store, obs patchobs.Observation) CoveragePublicationInput {
	path := filepath.Join(s.TpatchDir(), "features", obs.Slug, "artifacts", "apply-recipe.json")
	data, err := s.ReadFeatureFile(obs.Slug, "artifacts/apply-recipe.json")
	recipe := CoverageArtifact{Path: path, Present: err == nil, ReadError: err}
	if err == nil {
		recipe.Bytes = []byte(data)
	} else if errors.Is(err, fs.ErrNotExist) {
		recipe.ReadError = nil
	}
	stalePath := filepath.Join(s.TpatchDir(), "features", obs.Slug, "artifacts", "recipe-stale.json")
	_, staleErr := os.Lstat(stalePath)
	var observationErr error
	if staleErr != nil && !errors.Is(staleErr, fs.ErrNotExist) {
		observationErr = fmt.Errorf("observe recipe stale marker: %w", staleErr)
	}
	provenancePath := filepath.Join(s.TpatchDir(), "features", obs.Slug, "artifacts", "recipe-provenance.json")
	provenanceBytes, provenanceErr := s.ReadFeatureFile(obs.Slug, "artifacts/recipe-provenance.json")
	provenance := CoverageArtifact{Path: provenancePath, Present: provenanceErr == nil, ReadError: provenanceErr}
	if provenanceErr == nil {
		provenance.Bytes = []byte(provenanceBytes)
	} else if errors.Is(provenanceErr, fs.ErrNotExist) {
		provenance.ReadError = nil
	}
	prior, _ := s.ReadFeatureFile(obs.Slug, "artifacts/recipe-coverage.json")
	return CoveragePublicationInput{
		Observation: obs, Recipe: recipe,
		Provenance:    provenance,
		Events:        CoverageEvents{StaleMarkerPresent: staleErr == nil},
		priorCoverage: []byte(prior), priorPatchHash: obs.PatchSHA256,
		priorRecipeHash: CoverageSHA256(recipe.Bytes),
		observationErr:  observationErr,
	}
}

// ReconstructEditedCoverage carries a commit forward only when the pre-edit
// bindings and independently reconstructed preimage set both validate. This is
// observation I/O, outside the four pure S3 files, using S1's retained-image cap.
func ReconstructEditedCoverage(s *store.Store, in CoveragePublicationInput) CoveragePublicationInput {
	prior, err := DecodeRecipeCoverage(in.priorCoverage)
	if err != nil || prior.Feature != in.Observation.Slug || !prior.PatchPresent || !prior.RecipePresent ||
		prior.Reference.Kind != patchobs.ReferenceKindCommit ||
		prior.PatchSHA256 != in.priorPatchHash || prior.RecipeSHA256 != in.priorRecipeHash {
		return in
	}
	obs := patchobs.Observe(patchobs.Input{
		Producer: in.Observation.Producer, RepoRoot: s.Root, Slug: in.Observation.Slug,
		Patch: string(in.Observation.PatchBytes), PatchPresent: in.Observation.PatchPresent,
		Capture:     patchobs.CaptureDescriptor{Mode: patchobs.CaptureModeWorkingTreeAll},
		PreimageRef: prior.Reference.Commit,
	})
	if obs.Reference.Kind != patchobs.ReferenceKindCommit ||
		obs.Reference.PreimageSetSHA256 != prior.Reference.PreimageSetSHA256 {
		return in
	}
	obs.Capture = in.Observation.Capture
	obs.ArtifactBefore, obs.ArtifactAfter = in.Observation.ArtifactBefore, in.Observation.ArtifactAfter
	in.Observation = obs
	return in
}

// PublishCoverage is the single publication boundary for P1-P7. Call it last,
// after the producer's writes and state attempt. Recipe, provenance, generation
// and coverage outputs are planned from immutable inputs before this function
// publishes them in that order. Atomicity is single-file, not cross-file.
func PublishCoverage(s *store.Store, in CoveragePublicationInput) (coverage RecipeCoverage, retErr error) {
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("%w: %w", ErrCoveragePublication, retErr)
		}
	}()
	if s.Root != in.Observation.RepoRoot {
		return RecipeCoverage{}, fmt.Errorf("publish coverage: observation belongs to a different repository")
	}
	if in.observationErr != nil {
		return RecipeCoverage{}, fmt.Errorf("publish coverage: %w", in.observationErr)
	}
	if in.Autogen != nil {
		if in.Observation.Producer != patchobs.ProducerRecord && in.Observation.Producer != patchobs.ProducerFeaturePatch {
			return RecipeCoverage{}, fmt.Errorf("publish coverage: recipe derivation is not owned by this producer")
		}
		if in.Autogen.plan != nil && in.Autogen.plan.recipe != nil &&
			!bytes.Equal(in.Autogen.plan.recipe, in.Autogen.Recipe.Bytes) {
			return RecipeCoverage{}, fmt.Errorf("publish coverage: planned recipe bytes changed after derivation")
		}
		in.Recipe = in.Autogen.Recipe
		in.Events.RecipeRegenerated = in.Autogen.RecipeWritten
		in.Events.StaleMarkerPresent = in.Autogen.StaleMarkerPresent
	}
	if in.Generation != nil && in.Generation.Patch != string(in.Observation.PatchBytes) {
		return RecipeCoverage{}, fmt.Errorf("publish coverage: generation patch differs from immutable observation")
	}
	if recipe, err := decodeCoverageRecipe(in.Recipe.Bytes); in.Recipe.Present && err == nil {
		in.Observation.ParentCreatedPaths = slices.Clone(in.Observation.ParentCreatedPaths)
		for _, op := range recipe.Operations {
			if op.CreatedBy != "" {
				in.Observation.ParentCreatedPaths = append(in.Observation.ParentCreatedPaths, op.Path)
			}
		}
	}
	core := RecipeCoverageInput{Observation: in.Observation, Recipe: in.Recipe, Events: in.Events}
	c, err := BuildRecipeCoverage(core)
	if err != nil {
		return RecipeCoverage{}, fmt.Errorf("publish coverage: %w", err)
	}
	if in.Observation.Producer == patchobs.ProducerFeaturePatch && in.Events.PatchRewritten && c.CoverageStatus == CoverageComplete {
		derived, derr := DeriveRecipe(in.Observation)
		if derr != nil {
			return RecipeCoverage{}, fmt.Errorf("publish coverage: derive P2 origin: %w", derr)
		}
		if !derived.ProvesOrigin(in.Recipe.Bytes) {
			// D15's extra origin requirement governs the patch P2 wrote.
			// Its coverage-only checkpoint makes no origin/provenance claim.
			return RecipeCoverage{}, fmt.Errorf("publish coverage: P2 patch-writing event cannot publish complete coverage without D16 canonical-byte origin")
		}
	}
	data, err := EncodeRecipeCoverage(c)
	if err != nil {
		return RecipeCoverage{}, fmt.Errorf("publish coverage: encode: %w", err)
	}
	if in.Autogen != nil && in.Autogen.plan != nil {
		if err := publishRecordRecipePlan(s, in.Observation.Slug, in.Autogen); err != nil {
			return RecipeCoverage{}, err
		}
	}
	if in.Generation != nil {
		if _, err := AppendPatchGenerationForFeature(s, in.Observation.Slug, *in.Generation); err != nil {
			return RecipeCoverage{}, fmt.Errorf("record patch generation: %w", err)
		}
	}
	if err := s.WriteArtifactAtomic(in.Observation.Slug, "recipe-coverage.json", string(data)); err != nil {
		return RecipeCoverage{}, fmt.Errorf("publish coverage: %w", err)
	}
	return c, nil
}

// ReportCoverageStatus is the common completion path for a publication. Status
// is a stderr diagnostic, not part of a command's JSON/stdout result. The input
// is the successfully published record; failed publications emit no status.
func ReportCoverageStatus(w io.Writer, coverage RecipeCoverage, publicationErr error) error {
	if publicationErr != nil {
		return publicationErr
	}
	if w == nil {
		w = os.Stderr
	}
	line := "recipe coverage: " + coverage.CoverageStatus
	switch coverage.CoverageStatus {
	case CoverageComplete:
	case CoverageIncomplete:
		reasons := append([]string{}, coverage.Reasons...)
		for _, effect := range coverage.Effects {
			reasons = append(reasons, effect.ReasonCodes...)
		}
		line += " (" + strings.Join(coverageSortedCopy(reasons), ", ") + ")"
	default:
		return fmt.Errorf("%w: cannot report unknown coverage status %q", ErrCoveragePublication, coverage.CoverageStatus)
	}
	fmt.Fprintln(w, line)
	return nil
}
