package workflow

import (
	"fmt"
	"regexp"
	"slices"

	"github.com/tesseracode/tesserapatch/internal/candidate"
	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
)

type operationCandidateInvocation struct {
	slugs            []string
	upstreamCommit   string
	cumulativeLegacy bool
}

// Eligibility is not an all-present proof, an executable recipe or a wire type.
type operationCandidateEligibility struct {
	enabled, eligible                  bool
	gate                               int
	reason, detail                     string
	gatesPassed                        [15]bool
	coverageSHA256, captureEventSHA256 string
	patchSHA256, recipeSHA256          string
	referenceCommit, upstreamCommit    string
	sourceFeatureState                 store.FeatureState
	inputs                             *operationCandidateValidatedInputs
}

type operationCandidateValidatedInputs struct {
	patch        string
	effects      []gitutil.PatchEffect
	effectHashes map[int]string
}

type operationEligibilityContext struct {
	store       *store.Store
	slug        string
	invocation  operationCandidateInvocation
	status      store.FeatureStatus
	snapshot    RecipeCoverageSnapshot
	coverage    RecipeCoverage
	event       RecipeCaptureEvent
	recipe      ApplyRecipe
	observation patchobs.Observation
	result      operationCandidateEligibility
}

func assessOperationCandidateEligibility(s *store.Store, slug string, invocation operationCandidateInvocation) (operationCandidateEligibility, error) {
	cfg, err := s.LoadConfig()
	if err != nil {
		return operationCandidateEligibility{}, err
	}
	if !cfg.OperationCandidateEnabled {
		return operationCandidateEligibility{}, nil
	}
	if !regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`).MatchString(slug) {
		return operationCandidateEligibility{}, fmt.Errorf("invalid candidate feature slug")
	}
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return operationCandidateEligibility{}, err
	}
	c := operationEligibilityContext{
		store: s, slug: slug, status: status, invocation: invocation,
		result: operationCandidateEligibility{enabled: true, sourceFeatureState: status.State},
	}
	if status.State != store.StateApplied && status.State != store.StateBlocked {
		c.refuse("candidate-feature-state-unsupported", string(status.State))
		return c.result, nil
	}
	c.snapshot = SnapshotRecipeCoverage(s.Root, slug)
	gates := [15]func() bool{
		c.coverageUsable, c.coverageOwner, c.coverageComplete, c.captureEvent,
		c.bindingPresence, c.bindingHashes, c.recipeDecodable, c.recipeOwner,
		c.references, c.effectBindings, c.recipeWitnesses, c.supersession,
		c.parents, c.invocationMode, c.shadow,
	}
	for i, gate := range gates {
		c.result.gate = i + 1
		if !gate() {
			return c.result, nil
		}
		c.result.gatesPassed[i] = true
	}
	c.result.gate = 0
	c.result.eligible = true
	inputs := &operationCandidateValidatedInputs{
		patch:        string(c.snapshot.Patch.Bytes),
		effects:      make([]gitutil.PatchEffect, len(c.observation.Effects)),
		effectHashes: make(map[int]string, len(c.coverage.Effects)),
	}
	for i, effect := range c.observation.Effects {
		inputs.effects[i] = effect.Effect
		inputs.effectHashes[effect.Effect.Ordinal] = c.coverage.Effects[i].EffectSHA256
	}
	c.result.inputs = inputs
	return c.result, nil
}

func snapshotOperationCandidateBindings(s *store.Store, slug string) (candidate.CandidateBindings, error) {
	snapshot := SnapshotRecipeCoverage(s.Root, slug)
	event, err := DecodeRecipeCaptureEvent(snapshot.Event.Bytes)
	if err != nil {
		return candidate.CandidateBindings{}, fmt.Errorf("capture-event-unusable: %w", err)
	}
	digest, err := RecipeCaptureEventIdentity(event)
	if err != nil {
		return candidate.CandidateBindings{}, err
	}
	return candidate.CandidateBindings{CaptureEventSHA256: digest,
		CoverageSHA256: CoverageSHA256(snapshot.Coverage.Bytes), PatchSHA256: CoverageSHA256(snapshot.Patch.Bytes),
		RecipeSHA256: CoverageSHA256(snapshot.Recipe.Bytes)}, nil
}

func (c *operationEligibilityContext) refuse(reason, detail string) bool {
	c.result.reason, c.result.detail = reason, detail
	return false
}

func (c *operationEligibilityContext) coverageUsable() bool {
	read := c.snapshot.Coverage
	if read.Err != nil {
		return c.refuse("coverage-unusable", read.Err.Error())
	}
	if !read.Exists {
		return c.refuse("coverage-unusable", "coverage is absent")
	}
	coverage, err := DecodeRecipeCoverage(read.Bytes)
	if err != nil {
		return c.refuse("coverage-unusable", "strict coverage decoding failed")
	}
	c.coverage = coverage
	c.result.coverageSHA256 = CoverageSHA256(read.Bytes)
	return true
}

func (c *operationEligibilityContext) coverageOwner() bool {
	if c.coverage.Feature != c.slug {
		return c.refuse("coverage-owner-mismatch", "coverage.feature differs from requested feature")
	}
	return true
}

func (c *operationEligibilityContext) coverageComplete() bool {
	if c.coverage.CoverageStatus != CoverageComplete {
		return c.refuse("coverage-incomplete", "coverage is not complete")
	}
	return true
}

func (c *operationEligibilityContext) captureEvent() bool {
	read := c.snapshot.Event
	if read.Err != nil {
		return c.refuse("capture-event-unusable", read.Err.Error())
	}
	if !read.Exists {
		return c.refuse("capture-event-unusable", "capture event is absent")
	}
	event, err := DecodeRecipeCaptureEvent(read.Bytes)
	if err != nil {
		return c.refuse("capture-event-unusable", "strict capture event decoding failed")
	}
	digest, err := RecipeCaptureEventIdentity(event)
	if err != nil {
		return c.refuse("capture-event-unusable", "canonical capture event encoding failed")
	}
	if event.Feature != c.slug {
		return c.refuse("capture-event-owner-mismatch", "event.feature differs from requested feature")
	}
	for _, field := range []struct {
		name string
		same bool
	}{
		{"coverage_sha256", event.CoverageSHA256 == c.result.coverageSHA256},
		{"capture.mode", event.Capture.Mode == c.coverage.Capture.Mode},
		{"capture.pathspecs", slices.Equal(event.Capture.Pathspecs, c.coverage.Capture.Pathspecs)},
		{"capture.claim_ids", slices.Equal(event.Capture.ClaimIDs, c.coverage.Capture.ClaimIDs)},
	} {
		if !field.same {
			return c.refuse("capture-event-pair-mismatch", field.name+" differs")
		}
	}
	c.event, c.result.captureEventSHA256 = event, digest
	return true
}

func (c *operationEligibilityContext) bindingPresence() bool {
	for _, artifact := range []struct {
		name            string
		read            RecipeArtifactRead
		coverage, event bool
	}{
		{"patch", c.snapshot.Patch, c.coverage.PatchPresent, c.event.PatchPresent},
		{"recipe", c.snapshot.Recipe, c.coverage.RecipePresent, c.event.RecipePresent},
	} {
		present := artifact.read.Exists && artifact.read.Err == nil
		if artifact.coverage != present || artifact.event != present || (!present && len(artifact.read.Bytes) != 0) {
			detail := artifact.name + " readable presence differs"
			if artifact.read.Err != nil {
				detail += ": " + artifact.read.Err.Error()
			}
			return c.refuse("binding-presence-drift", detail)
		}
	}
	return true
}

func (c *operationEligibilityContext) bindingHashes() bool {
	c.result.patchSHA256 = CoverageSHA256(c.snapshot.Patch.Bytes)
	c.result.recipeSHA256 = CoverageSHA256(c.snapshot.Recipe.Bytes)
	if c.result.patchSHA256 != c.coverage.PatchSHA256 || c.result.patchSHA256 != c.event.PatchSHA256 {
		return c.refuse("binding-hash-drift", "patch raw-byte hash differs")
	}
	if c.result.recipeSHA256 != c.coverage.RecipeSHA256 || c.result.recipeSHA256 != c.event.RecipeSHA256 {
		return c.refuse("binding-hash-drift", "recipe raw-byte hash differs")
	}
	return true
}

func (c *operationEligibilityContext) recipeDecodable() bool {
	recipe, err := decodeCoverageRecipe(c.snapshot.Recipe.Bytes)
	if err != nil {
		return c.refuse("recipe-undecodable", "strict on-disk recipe decoding failed")
	}
	c.recipe = recipe
	return true
}

func (c *operationEligibilityContext) recipeOwner() bool {
	if c.recipe.Feature != c.coverage.Feature {
		return c.refuse("recipe-owner-mismatch", "recipe.feature differs")
	}
	return true
}

func (c *operationEligibilityContext) references() bool {
	reference := c.event.Reference
	if reference != c.coverage.Reference || reference.Kind != patchobs.ReferenceKindCommit {
		return c.refuse("reference-not-reconstructable", "paired durable reference differs")
	}
	base, err := candidate.OpenReferenceRoot(c.store.Root, reference.Commit)
	if err != nil {
		return c.refuse("reference-not-reconstructable", err.Error())
	}
	defer base.Close()
	upstream, err := candidate.OpenReferenceRoot(c.store.Root, c.invocation.upstreamCommit)
	if err != nil {
		return c.refuse("reference-not-reconstructable", "upstream: "+err.Error())
	}
	defer upstream.Close()
	if err := upstream.CheckTree(); err != nil {
		return c.refuse("reference-not-reconstructable", "upstream: "+err.Error())
	}
	rows, err := base.ReconstructPatch(string(c.snapshot.Patch.Bytes))
	if err != nil {
		return c.refuse("reference-not-reconstructable", err.Error())
	}
	obs := reconstructedObservation(c.store.Root, c.coverage, c.event,
		coverageReadArtifact(c.slug, "post-apply.patch", c.snapshot.Patch))
	for _, row := range rows {
		if !row.Pre.Available || !row.Post.Available {
			return c.refuse("reference-not-reconstructable", "required image is unavailable within the reconstruction budget")
		}
		e := row.Effect
		e.PreimageObserved, e.PreimagePresent, e.PreimageSHA256 = true, row.Pre.Present, row.Pre.SHA256
		e.PostimageObserved, e.PostimagePresent, e.PostimageSHA256 = true, row.Post.Present, row.Post.SHA256
		e.OldMode, e.NewMode = row.Pre.Mode, row.Post.Mode
		entry := patchobs.EffectObservation{Effect: e}
		entry.Bytes.Preimage, entry.Bytes.Postimage = row.Pre.Bytes, row.Post.Bytes
		entry.Effect.ContentKind, entry.Effect.ObjectKind = patchobs.ClassifyEffectObservation(e, entry.Bytes)
		obs.Effects = append(obs.Effects, entry)
	}
	if patchobs.PreimageSetDigest(obs.Effects) != reference.PreimageSetSHA256 {
		return c.refuse("reference-not-reconstructable", "recomputed preimage-set digest differs")
	}
	paths := make([]string, 0, len(rows))
	for _, row := range rows {
		paths = append(paths, row.Effect.Path)
	}
	if err := upstream.CheckPaths(paths); err != nil {
		return c.refuse("reference-not-reconstructable", "upstream: "+err.Error())
	}
	c.observation = obs
	c.result.referenceCommit, c.result.upstreamCommit = base.Commit(), upstream.Commit()
	return true
}

func (c *operationEligibilityContext) effectBindings() bool {
	if len(c.observation.Effects) != len(c.coverage.Effects) || len(c.event.Observations) != len(c.coverage.Effects) {
		return c.refuse("effect-binding-drift", "effect count differs")
	}
	for i, row := range c.observation.Effects {
		projection := recipeCaptureProjection(row.Effect)
		if projection != c.event.Observations[i] || projection != recipeCaptureProjectionFromCoverage(c.coverage.Effects[i]) {
			return c.refuse("effect-binding-drift", fmt.Sprintf("effect %d projection differs", i+1))
		}
	}
	event := c.event.Event
	// Event semantics depend on current explanation, not advisory producer labels.
	if event.StaleMarkerPresent ||
		((event.PatchRewritten && !event.RecipeRegenerated) || event.BoundArtifactEdited) && !c.witnessesSupported() {
		return c.refuse("effect-binding-drift", "capture event facts contradict complete coverage")
	}
	return true
}

func (c *operationEligibilityContext) recipeWitnesses() bool {
	if c.snapshot.Marker.Exists || c.snapshot.Marker.Err != nil {
		detail := "stale marker is present"
		if c.snapshot.Marker.Err != nil {
			detail += ": " + c.snapshot.Marker.Err.Error()
		}
		return c.refuse("recipe-stale-marker-present", detail)
	}
	if !c.witnessesSupported() {
		return c.refuse("recipe-unsupported", "recipe assignments, witnesses or captured parent exclusions differ")
	}
	return true
}

func (c *operationEligibilityContext) witnessesSupported() bool {
	parents, err := coverageParents(c.observation)
	if err != nil {
		return false
	}
	for _, op := range c.recipe.Operations {
		if op.CreatedBy != "" || !coverageAdmissibleReclassification(op) {
			return false
		}
	}
	assignments, surplus, err := coverageAssignments(c.observation, c.recipe)
	if err != nil || surplus {
		return false
	}
	for i, row := range c.observation.Effects {
		if !slices.Equal(assignments[i], c.coverage.Effects[i].OperationIndexes) {
			return false
		}
		path, err := coveragePath(c.store.Root, row.Effect.Path)
		if err != nil || (parents[path] && !row.Effect.PreimagePresent) {
			return false
		}
	}
	simulation := simulateCoverage(c.observation, c.recipe, assignments, surplus)
	return simulation.ExactPostimage && simulation.AllAlreadyPresent
}

func (c *operationEligibilityContext) supersession() bool {
	features, err := store.NewFeatureSnapshot(c.store)
	if err != nil || len(features.Unreadable()) != 0 {
		return c.refuse("supersession-not-authority", "supersession metadata is unreadable")
	}
	if _, superseded := isFeatureSupersededIn(features.List(), c.slug); superseded {
		return c.refuse("supersession-not-authority", "feature is superseded")
	}
	return true
}

func (c *operationEligibilityContext) parents() bool {
	labels := map[store.ReconcileLabel]struct{}{}
	composeM143Labels(c.store, c.status, labels)
	if _, blocked := labels[store.LabelBlockedByParent]; blocked {
		return c.refuse("parent-blocked", "hard parent is blocked")
	}
	return true
}

func (c *operationEligibilityContext) invocationMode() bool {
	if c.invocation.cumulativeLegacy || len(c.invocation.slugs) != 1 || c.invocation.slugs[0] != c.slug {
		return c.refuse("derivation-mode-unsupported", "candidate derivation requires exactly the requested slug and no cumulative legacy mode")
	}
	return true
}

func (c *operationEligibilityContext) shadow() bool {
	if c.status.Reconcile.ShadowPath != "" {
		return c.refuse("shadow-pending", "resolver shadow is already staged")
	}
	return true
}
