package workflow

import (
	"bytes"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
)

// BuildRecipeCoverage derives a record from immutable observations and raw
// artifacts only. It neither publishes a sidecar nor reads a live tree.
func BuildRecipeCoverage(in RecipeCoverageInput) (RecipeCoverage, error) {
	patchReason, err := coverageObservation(in.Observation)
	if err != nil {
		return RecipeCoverage{}, err
	}
	obs := in.Observation
	if in.Recipe.Present && in.Recipe.ReadError != nil {
		return RecipeCoverage{}, fmt.Errorf("coverage: readable recipe has a read failure at %s: %w", in.Recipe.Path, in.Recipe.ReadError)
	}
	if !in.Recipe.Present && len(in.Recipe.Bytes) != 0 {
		return RecipeCoverage{}, fmt.Errorf("coverage: unreadable/absent recipe carries bytes")
	}
	if in.Events.RecipeRegenerated && !in.Recipe.Present {
		return RecipeCoverage{}, fmt.Errorf("coverage: regeneration without a readable recipe")
	}
	if in.Events.BoundArtifactEdited && obs.Producer != patchobs.ProducerEdit {
		return RecipeCoverage{}, fmt.Errorf("coverage: artifact-edit fact on another producer")
	}
	c := RecipeCoverage{
		SchemaVersion: 1, Feature: obs.Slug, Producer: obs.Producer,
		PatchPresent: obs.PatchPresent, PatchSHA256: obs.PatchSHA256,
		RecipePresent: in.Recipe.Present,
		Reference:     CoverageReference{obs.Reference.Kind, obs.Reference.Commit, obs.Reference.PreimageSetSHA256},
		Capture:       CoverageCapture{obs.Capture.Mode, coverageSortedCopy(obs.Capture.Pathspecs), coverageSortedCopy(obs.Capture.ClaimIDs)},
		Effects:       []CoverageEffect{}, Reasons: []string{},
		CoverageStatus: CoverageIncomplete, CrossBaseStatus: CrossBaseUnsupported,
	}
	if patchReason != "" {
		c.Reasons = append(c.Reasons, patchReason)
	}
	if c.Reference.Kind != patchobs.ReferenceKindCommit {
		c.Reasons = append(c.Reasons, "reference-not-durable")
	}
	var recipe ApplyRecipe
	if in.Recipe.Present {
		c.RecipeSHA256 = CoverageSHA256(in.Recipe.Bytes)
		recipe, err = decodeCoverageRecipe(in.Recipe.Bytes)
		c.RecipeDecodable = err == nil
		if err != nil {
			c.Reasons = append(c.Reasons, "recipe-undecodable")
		}
	}
	if c.RecipeDecodable && recipe.Feature != c.Feature {
		c.Reasons = append(c.Reasons, "recipe-owner-mismatch")
	}
	if in.Events.StaleMarkerPresent {
		c.Reasons = append(c.Reasons, "recipe-stale-marker-present")
	}
	parents, err := coverageParents(obs)
	if err != nil {
		return RecipeCoverage{}, err
	}
	assignments, surplus, err := coverageAssignments(obs, recipe)
	if err != nil {
		return RecipeCoverage{}, err
	}
	simulation := simulateCoverage(obs, recipe, assignments, surplus)
	if c.RecipeDecodable && surplus {
		c.Reasons = append(c.Reasons, "operation-surplus")
	}
	if c.RecipeDecodable && patchReason == "" && !simulation.ExactPostimage {
		c.Reasons = append(c.Reasons, "simulation-mismatch")
	}
	effectsRepresented := len(obs.Effects) != 0
	for i, entry := range obs.Effects {
		e := coverageEffect(entry.Effect)
		e.OperationIndexes = assignments[i]
		normalizedPath, pathErr := coveragePath(obs.RepoRoot, e.Path)
		if e.OldPath != "" {
			if _, oldErr := coveragePath(obs.RepoRoot, e.OldPath); oldErr != nil {
				pathErr = oldErr
			}
		}
		unreclassifiable := false
		for _, index := range assignments[i] {
			unreclassifiable = unreclassifiable || slices.Contains(simulation.UnreclassifiableOperations, index)
		}
		e.ReasonCodes = coverageLocalReasons(e, parents[normalizedPath] && !e.PreimagePresent, unreclassifiable, pathErr != nil)
		e.Disposition, err = coverageDisposition(e.ReasonCodes)
		if err != nil {
			return RecipeCoverage{}, err
		}
		e.ContextualHint = coverageContextualHint(entry)
		e.EffectSHA256, err = CoverageEffectSHA256(e)
		if err != nil {
			return RecipeCoverage{}, err
		}
		c.Effects = append(c.Effects, e)
		effectsRepresented = effectsRepresented && e.Disposition == "represented"
	}
	// Event diagnostics depend on the actual explanation/simulation, never on
	// producer identity, an origin label, or the fact that a write succeeded.
	explains := patchReason == "" && c.RecipeDecodable && recipe.Feature == c.Feature &&
		effectsRepresented && !surplus && simulation.ExactPostimage && simulation.AllAlreadyPresent
	if in.Events.PatchRewritten && in.Recipe.Present && !in.Events.RecipeRegenerated && !explains {
		c.Reasons = append(c.Reasons, "producer-patch-rewrite", "recipe-not-regenerated")
	}
	if in.Events.BoundArtifactEdited && (!explains || obs.Reference.Kind != patchobs.ReferenceKindCommit) {
		c.Reasons = append(c.Reasons, "manual-bound-artifact-edit")
	}
	c.Reasons = coverageSortedCopy(c.Reasons)
	// These are D3's ten predicates, grouped only where construction already
	// proves them: observation validation proves 1/4/8; assignment proves 5/6;
	// exact simulation and its no-write reclassification prove 9/10.
	complete := explains && obs.Reference.Kind == patchobs.ReferenceKindCommit && len(c.Reasons) == 0
	if complete {
		c.CoverageStatus = CoverageComplete
		c.CrossBaseStatus, err = coverageCrossBase(c, recipe)
		if err != nil {
			return RecipeCoverage{}, err
		}
	}
	if err := ValidateRecipeCoverageSchema(c); err != nil {
		return RecipeCoverage{}, err
	}
	return c, nil
}

// ValidateRecipeCoverage recomputes every binding, assignment, reason and
// completeness predicate. A structurally valid stored "complete" is not proof.
func ValidateRecipeCoverage(c RecipeCoverage, in RecipeCoverageInput) error {
	if err := ValidateRecipeCoverageSchema(c); err != nil {
		return err
	}
	want, err := BuildRecipeCoverage(in)
	if err != nil {
		return err
	}
	// Diagnostic producer labels and contextual hints cannot become a
	// permission gate. Recompute authority from the supplied facts regardless
	// of either advisory value; schema validation still enforces their enums.
	want.Producer = c.Producer
	for i := range want.Effects {
		if i < len(c.Effects) {
			want.Effects[i].ContextualHint = c.Effects[i].ContextualHint
		}
	}
	if !coverageEqual(c, want) {
		return fmt.Errorf("coverage: record does not match recomputed immutable inputs")
	}
	return nil
}

func coverageObservation(obs patchobs.Observation) (string, error) {
	if !filepath.IsAbs(obs.RepoRoot) {
		return "", fmt.Errorf("coverage: observation requires an absolute repository root")
	}
	if obs.Slug == "" || !utf8.ValidString(obs.Slug) || !patchobs.KnownProducer(obs.Producer) ||
		!patchobs.KnownCaptureMode(obs.Capture.Mode) {
		return "", fmt.Errorf("coverage: invalid observation identity/capture")
	}
	if obs.Capture.Mode == patchobs.CaptureModeNoCapture &&
		(len(obs.Capture.Pathspecs) != 0 || len(obs.Capture.ClaimIDs) != 0) {
		return "", fmt.Errorf("coverage: no-capture observation has selectors")
	}
	if obs.Reference.Kind == patchobs.ReferenceKindCommit {
		if !fullRecipeBaseCommit(obs.Reference.Commit) {
			return "", fmt.Errorf("coverage: invalid captured commit")
		}
	} else if (obs.Reference.Kind != patchobs.ReferenceKindUnavailable &&
		obs.Reference.Kind != patchobs.ReferenceKindIndexSnapshot) || obs.Reference.Commit != "" {
		return "", fmt.Errorf("coverage: invalid captured reference")
	}
	reason := ""
	if !obs.PatchPresent {
		if obs.PatchSHA256 != "" || len(obs.PatchBytes) != 0 || len(obs.Effects) != 0 || obs.ParseRefusal != "" {
			return "", fmt.Errorf("coverage: missing patch carries fabricated evidence")
		}
		reason = "canonical-patch-missing"
	} else {
		if CoverageSHA256(obs.PatchBytes) != obs.PatchSHA256 {
			return "", fmt.Errorf("coverage: captured patch bytes changed")
		}
		parsed, parseErr := gitutil.NormalizePatchEffects(string(obs.PatchBytes))
		if (parseErr != nil) != (obs.ParseRefusal != "") {
			return "", fmt.Errorf("coverage: captured parse result changed")
		}
		switch {
		case parseErr != nil:
			if len(obs.Effects) != 0 {
				return "", fmt.Errorf("coverage: unparseable patch carries effects")
			}
			reason = "canonical-patch-unparseable"
		case len(parsed) == 0:
			if len(obs.Effects) != 0 {
				return "", fmt.Errorf("coverage: empty patch carries effects")
			}
			reason = "canonical-patch-empty"
		default:
			if len(parsed) != len(obs.Effects) {
				return "", fmt.Errorf("coverage: captured effect count changed")
			}
			for i, entry := range obs.Effects {
				e := entry.Effect
				if !utf8.ValidString(e.Path) || !utf8.ValidString(e.OldPath) {
					return "", fmt.Errorf("coverage: effect path cannot be encoded losslessly")
				}
				pre, post := parsed[i].ExtantSides()
				if err := validateRecipeSide(e.PreimageObserved, e.PreimagePresent, pre,
					e.OldMode, parsed[i].HeaderOldMode, e.PreimageSHA256, entry.Bytes.Preimage); err != nil {
					return "", fmt.Errorf("coverage: %s preimage: %w", e.Path, err)
				}
				if err := validateRecipeSide(e.PostimageObserved, e.PostimagePresent, post,
					e.NewMode, parsed[i].HeaderNewMode, e.PostimageSHA256, entry.Bytes.Postimage); err != nil {
					return "", fmt.Errorf("coverage: %s postimage: %w", e.Path, err)
				}
				if (e.PreimagePresent && !coverageHash(e.PreimageSHA256)) ||
					(e.PostimagePresent && !coverageHash(e.PostimageSHA256)) {
					return "", fmt.Errorf("coverage: invalid observed content hash")
				}
				if (e.OldMode == gitutil.ModeGitlink && len(entry.Bytes.Preimage) != 0) ||
					(e.NewMode == gitutil.ModeGitlink && len(entry.Bytes.Postimage) != 0) {
					return "", fmt.Errorf("coverage: gitlink carries a file body")
				}
				want := parsed[i]
				want.PreimageObserved, want.PreimagePresent, want.PreimageSHA256 = e.PreimageObserved, e.PreimagePresent, e.PreimageSHA256
				want.PostimageObserved, want.PostimagePresent, want.PostimageSHA256 = e.PostimageObserved, e.PostimagePresent, e.PostimageSHA256
				want.OldMode, want.NewMode = e.OldMode, e.NewMode
				want.ContentKind, want.ObjectKind = coverageKinds(entry)
				if e != want {
					return "", fmt.Errorf("coverage: effect %d does not bind the full normalized observation", i+1)
				}
			}
		}
	}
	if !coverageHash(obs.Reference.PreimageSetSHA256) ||
		patchobs.PreimageSetDigest(obs.Effects) != obs.Reference.PreimageSetSHA256 {
		return "", fmt.Errorf("coverage: captured preimage-set digest changed")
	}
	return reason, nil
}

func coverageKinds(entry patchobs.EffectObservation) (gitutil.ContentKind, gitutil.ObjectKind) {
	return patchobs.ClassifyEffectObservation(entry.Effect, entry.Bytes)
}

func coverageEffect(e gitutil.PatchEffect) CoverageEffect {
	return CoverageEffect{
		Ordinal: e.Ordinal, ChangeKind: e.ChangeKind, ContentKind: e.ContentKind, ObjectKind: e.ObjectKind,
		Path: e.Path, OldPath: e.OldPath, OldMode: e.OldMode, NewMode: e.NewMode,
		PreimageObserved: e.PreimageObserved, PreimagePresent: e.PreimagePresent, PreimageSHA256: e.PreimageSHA256,
		PostimageObserved: e.PostimageObserved, PostimagePresent: e.PostimagePresent, PostimageSHA256: e.PostimageSHA256,
		PatchFragmentSHA256: e.FragmentSHA256,
		OperationIndexes:    []int{}, ReasonCodes: []string{}, ContextualHint: "none",
	}
}

func coverageLocalReasons(e CoverageEffect, parent, unreclassifiable, unsafe bool) []string {
	reasons := []string{}
	for code, condition := range map[string]bool{
		"effect-delete-unsupported": e.ChangeKind == gitutil.ChangeKindDelete,
		"effect-rename-unsupported": e.ChangeKind == gitutil.ChangeKindRename,
		"effect-copy-unsupported":   e.ChangeKind == gitutil.ChangeKindCopy,
		"effect-binary-unsupported": e.ContentKind == gitutil.ContentKindBinary,
		"effect-mode-only-unsupported": e.ChangeKind == gitutil.ChangeKindModify &&
			e.PreimageObserved && e.PostimageObserved && e.PreimagePresent && e.PostimagePresent &&
			e.PreimageSHA256 == e.PostimageSHA256 && e.OldMode != e.NewMode,
		"effect-symlink-unsupported":        e.OldMode == gitutil.ModeSymlink || e.NewMode == gitutil.ModeSymlink,
		"effect-gitlink-unsupported":        e.OldMode == gitutil.ModeGitlink || e.NewMode == gitutil.ModeGitlink,
		"effect-executable-unsupported":     e.OldMode == gitutil.ModeExecutable || e.NewMode == gitutil.ModeExecutable,
		"preimage-unavailable":              !e.PreimageObserved,
		"postimage-unavailable":             !e.PostimageObserved,
		"parent-created-target-unsupported": parent,
		"operation-not-reclassifiable":      unreclassifiable,
		"path-unsafe":                       unsafe,
	} {
		if condition {
			reasons = append(reasons, code)
		}
	}
	if len(reasons) == 0 && len(e.OperationIndexes) == 0 {
		reasons = append(reasons, "operation-missing")
	}
	sort.Strings(reasons)
	return reasons
}

func coverageSortedCopy(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return slices.Compact(out)
}

func coveragePath(root, target string) (string, error) {
	if target == "" || strings.ContainsRune(target, 0) {
		return "", fmt.Errorf("coverage: empty or NUL path")
	}
	// Exactly execution's lexical Join, without Abs, stat, symlink resolution,
	// whitespace trimming or a worktree read. Root is already absolute.
	relative, err := filepath.Rel(root, filepath.Join(root, target))
	if err != nil {
		return "", err
	}
	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("coverage: target does not name a file inside the repository")
	}
	return filepath.ToSlash(relative), nil
}

func coverageEffectPathUnsafe(e CoverageEffect) bool {
	for i, path := range []string{e.Path, e.OldPath} {
		if i == 1 && path == "" {
			continue
		}
		// Normalized effect paths obey the strict grammar's repo-relative
		// spelling. Operation aliases are normalized separately using Join.
		if path == "" || strings.ContainsRune(path, 0) || strings.HasPrefix(path, "/") ||
			strings.HasPrefix(path, `\`) {
			return true
		}
		if len(path) >= 3 && path[1] == ':' && (path[2] == '/' || path[2] == '\\') &&
			((path[0] >= 'a' && path[0] <= 'z') || (path[0] >= 'A' && path[0] <= 'Z')) {
			return true
		}
		for _, segment := range strings.Split(path, "/") {
			if segment == "" || segment == "." || segment == ".." {
				return true
			}
		}
		if _, err := coveragePath(filepath.Join(string(filepath.Separator), "coverage-schema-root"), path); err != nil {
			return true
		}
	}
	return false
}

func coverageParents(obs patchobs.Observation) (map[string]bool, error) {
	out := map[string]bool{}
	for _, path := range obs.ParentCreatedPaths {
		normalized, err := coveragePath(obs.RepoRoot, path)
		if err != nil {
			return nil, fmt.Errorf("coverage: invalid parent target: %w", err)
		}
		out[normalized] = true
	}
	return out, nil
}

func coverageContextualHint(entry patchobs.EffectObservation) string {
	e := entry.Effect
	if e.ChangeKind != gitutil.ChangeKindModify || e.ContentKind != gitutil.ContentKindText ||
		e.OldMode != gitutil.ModeRegular || e.NewMode != gitutil.ModeRegular ||
		!e.PreimageObserved || !e.PostimageObserved || bytes.Equal(entry.Bytes.Preimage, entry.Bytes.Postimage) {
		return "none"
	}
	// A byte-exact line subsequence is only a deterministic hint. No anchor,
	// uniqueness, replay eligibility or authority is derived or persisted.
	before := bytes.SplitAfter(entry.Bytes.Preimage, []byte("\n"))
	after := bytes.SplitAfter(entry.Bytes.Postimage, []byte("\n"))
	at := 0
	for _, line := range after {
		if at < len(before) && bytes.Equal(line, before[at]) {
			at++
		}
	}
	if at == len(before) {
		return "additive-text"
	}
	return "none"
}

func coverageCrossBase(c RecipeCoverage, recipe ApplyRecipe) (string, error) {
	creationOnly := true
	wholeExisting := false
	for _, e := range c.Effects {
		for _, index := range e.OperationIndexes {
			op := recipe.Operations[index-1]
			wholeExisting = wholeExisting || (e.PreimagePresent && op.Type == "write-file")
			creationOnly = creationOnly && !e.PreimagePresent && op.Type == "write-file" &&
				op.PreimageHash != nil && *op.PreimageHash == ""
		}
	}
	if wholeExisting {
		return CrossBaseConsumerDerivationRequired, nil
	}
	if creationOnly {
		return CrossBaseReferenceTreeOnly, nil
	}
	// D3 supplies no fourth branch. Do not invent replay scope or an
	// incompleteness reason for a recipe that meets the ten predicates.
	return "", fmt.Errorf("coverage: exact recipe has no defined D3 cross-base branch (requires contract adjudication)")
}
