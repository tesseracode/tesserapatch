package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/tesseracode/tesserapatch/internal/candidate"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func operationCandidatePublication(s *store.Store, transaction *operationCandidateTransaction, upstream *candidate.ReferenceRoot, execution candidate.ExecutionResult, now time.Time) (CoveragePublicationInput, error) {
	rows, err := upstream.ReconstructPatch(execution.Diff)
	if err != nil {
		return CoveragePublicationInput{}, err
	}
	paths := operationCandidateArtifactPaths(transaction.artifact)
	changes := make(map[string]candidate.AcceptanceChange, len(transaction.changes))
	for _, change := range transaction.changes {
		changes[change.Path] = change
	}
	obs := patchobs.Observation{
		Producer: patchobs.ProducerReconcileAccept, RepoRoot: s.Root, Slug: transaction.slug,
		PatchPresent: true, PatchBytes: []byte(execution.Diff), PatchSHA256: store.SHA256HexString(execution.Diff),
		Capture:            patchobs.CaptureDescriptor{Mode: patchobs.CaptureModeReconcile, Pathspecs: paths, ClaimIDs: []string{}},
		ParentCreatedPaths: []string{}, Effects: []patchobs.EffectObservation{}, Reasons: []string{},
	}
	if len(rows) != len(transaction.changes) {
		return CoveragePublicationInput{}, fmt.Errorf("residual patch path set differs from candidate operations")
	}
	for _, row := range rows {
		if !row.Pre.Available || !row.Post.Available || !row.Post.Present {
			return CoveragePublicationInput{}, fmt.Errorf("residual images are unavailable")
		}
		effect := row.Effect
		effect.PreimageObserved, effect.PreimagePresent, effect.PreimageSHA256 = true, row.Pre.Present, row.Pre.SHA256
		effect.PostimageObserved, effect.PostimagePresent, effect.PostimageSHA256 = true, row.Post.Present, row.Post.SHA256
		effect.OldMode, effect.NewMode = row.Pre.Mode, row.Post.Mode
		change, exists := changes[effect.Path]
		if !exists || row.Pre.Present != change.Present ||
			!bytes.Equal(row.Pre.Bytes, change.Before) || !bytes.Equal(row.Post.Bytes, change.After) {
			return CoveragePublicationInput{}, fmt.Errorf("residual patch does not reproduce the re-derived operations")
		}
		delete(changes, effect.Path)
		entry := patchobs.EffectObservation{Effect: effect, ReasonCodes: []string{}}
		entry.Bytes.Preimage, entry.Bytes.Postimage = row.Pre.Bytes, row.Post.Bytes
		entry.Effect.ContentKind, entry.Effect.ObjectKind = patchobs.ClassifyEffectObservation(effect, entry.Bytes)
		obs.Effects = append(obs.Effects, entry)
	}
	obs.Reference = patchobs.ReferenceDescriptor{
		Kind: patchobs.ReferenceKindCommit, Commit: upstream.Commit(),
		PreimageSetSHA256: patchobs.PreimageSetDigest(obs.Effects),
	}
	derived, err := DeriveRecipe(obs)
	if err != nil {
		return CoveragePublicationInput{}, err
	}
	recipe := derived.CanonicalBytes()
	if len(recipe) == 0 {
		return CoveragePublicationInput{}, fmt.Errorf("residual recipe is not completely representable: %v %v", derived.Reasons, derived.Exclusions)
	}
	provenance, err := convergeRecipeProvenance(derived, recipe, CoverageArtifact{}, now)
	if err != nil {
		return CoveragePublicationInput{}, fmt.Errorf("residual recipe has no truthful provenance: %w", err)
	}
	if len(provenance) == 0 {
		return CoveragePublicationInput{}, fmt.Errorf("residual recipe has no truthful provenance")
	}
	return CoveragePublicationInput{
		Observation: obs, Recipe: CoverageArtifact{Present: true, Bytes: recipe},
		Provenance: CoverageArtifact{Present: true, Bytes: provenance},
		Events:     CoverageEvents{PatchRewritten: true, RecipeRegenerated: true},
		Generation: &PatchGenerationInput{
			Kind: "reconcile", Patch: execution.Diff, BaseCommit: upstream.Commit(),
			Upper:   store.GenerationUpper{Kind: "reconcile-result", Ref: upstream.Commit(), Commit: upstream.Commit()},
			Capture: store.GenerationCapture{Mode: "reconcile", Pathspecs: slices.Clone(paths), ClaimIDs: []string{}},
		},
		candidateAcceptance: transaction,
	}, nil
}

func (t *operationCandidateTransaction) publishInputs(s *store.Store, in CoveragePublicationInput, coverage RecipeCoverage) error {
	if in.candidateAcceptance != t || s.Root != t.disk.path || in.Observation.Slug != t.slug ||
		in.Autogen != nil || in.Generation == nil ||
		coverage.Producer != patchobs.ProducerReconcileAccept || coverage.Capture.Mode != patchobs.CaptureModeReconcile ||
		coverage.CoverageStatus != CoverageComplete || !in.Events.PatchRewritten || !in.Events.RecipeRegenerated ||
		!in.Provenance.Present {
		return fmt.Errorf("operation-candidate-accept publication lacks its complete bound input")
	}
	if t.step != candidateStepStage && t.step != candidateStepPublish {
		return fmt.Errorf("operation-candidate-accept publication outside steps 7/11")
	}
	if err := t.validatePaths(); err != nil {
		return err
	}
	generation := t.staged["artifacts/patch-generations.json"]
	if t.step == candidateStepStage {
		// Pass the confined pre-write observation into the shared appender; it
		// must not reopen the historical manifest through an unheld pathname.
		prior, err := t.disk.read(t.featureDir + "/artifacts/patch-generations.json")
		if err != nil {
			return err
		}
		input := *in.Generation
		input.candidateStage = &operationCandidateGenerationStage{
			recipe: bytes.Clone(in.Recipe.Bytes), priorPresent: prior.Present, priorManifest: bytes.Clone(prior.Data),
		}
		if _, err := AppendPatchGenerationForFeature(s, t.slug, input); err != nil {
			return err
		}
		generation = input.candidateStage.manifest
	}
	var manifest store.PatchGenerationsManifest
	if err := json.Unmarshal(generation, &manifest); err != nil {
		return err
	}
	if err := validateOperationCandidateBaseConsistency(t.artifact.UpstreamCommit, coverage, manifest); err != nil {
		return err
	}
	for _, file := range []struct {
		name string
		data []byte
	}{
		{"artifacts/post-apply.patch", in.Observation.PatchBytes},
		{"artifacts/apply-recipe.json", in.Recipe.Bytes},
		{"artifacts/recipe-provenance.json", in.Provenance.Bytes},
		{"artifacts/patch-generations.json", generation},
	} {
		if err := t.writePublicationFile(file.name, file.data); err != nil {
			return err
		}
	}
	return nil
}

func validateOperationCandidateBaseConsistency(base string, coverage RecipeCoverage, manifest store.PatchGenerationsManifest) error {
	if err := store.ValidatePatchGenerations(coverage.Feature, manifest); err != nil {
		return err
	}
	latest, ok := store.LatestPatchGeneration(manifest)
	if !ok || coverage.Reference.Kind != patchobs.ReferenceKindCommit || coverage.Reference.Commit != base ||
		latest.BaseCommit != base || latest.PatchSHA256 != coverage.PatchSHA256 || latest.RecipeSHA256 != coverage.RecipeSHA256 ||
		latest.Capture.Mode != "reconcile" || !slices.Equal(sortedCopy(latest.Capture.Pathspecs), sortedCopy(coverage.Capture.Pathspecs)) ||
		!slices.Equal(sortedCopy(latest.Capture.ClaimIDs), sortedCopy(coverage.Capture.ClaimIDs)) ||
		latest.Upper != (store.GenerationUpper{Kind: "reconcile-result", Ref: base, Commit: base}) {
		return fmt.Errorf("acceptance base/patch/recipe/capture consistency failed")
	}
	return nil
}

func (t *operationCandidateTransaction) validatePublishedBase() error {
	statusImage, err := t.disk.read(t.featureDir + "/status.json")
	if err != nil {
		return err
	}
	var status store.FeatureStatus
	if !statusImage.Present || json.Unmarshal(statusImage.Data, &status) != nil ||
		status.State != store.StateApplied || status.Apply.BaseCommit != t.artifact.UpstreamCommit ||
		status.Apply.CompletedAt == "" || !status.Apply.HasPatch || !status.Apply.HasRecipe {
		return fmt.Errorf("accepted status does not name the bound base and completion")
	}
	for _, name := range operationCandidateFeatureFiles[:len(operationCandidateFeatureFiles)-1] {
		image, err := t.disk.read(t.featureDir + "/" + name)
		if err != nil || !image.Present || !bytes.Equal(image.Data, t.staged[name]) {
			return fmt.Errorf("published artifact differs from staged input: %s", name)
		}
	}
	coverage, err := DecodeRecipeCoverage(t.staged["artifacts/recipe-coverage.json"])
	if err != nil {
		return err
	}
	var manifest store.PatchGenerationsManifest
	if err := json.Unmarshal(t.staged["artifacts/patch-generations.json"], &manifest); err != nil {
		return err
	}
	return validateOperationCandidateBaseConsistency(status.Apply.BaseCommit, coverage, manifest)
}
