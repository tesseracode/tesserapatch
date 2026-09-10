package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tesseracode/tesserapatch/internal/patchobs"
)

// RecipeArtifactRead retains directory-entry existence separately from readable
// existence. In particular a dangling symlink is not a legacy absent artifact.
type RecipeArtifactRead struct {
	Bytes  []byte
	Exists bool
	Err    error
}

// ReadRecipeArtifact is the shared inventory/apply/doctor injection seam.
// Tests replace it only for serial invocations, restoring it afterwards.
var ReadRecipeArtifact = readRecipeArtifact

func readRecipeArtifact(path string) RecipeArtifactRead {
	data, err := os.ReadFile(path)
	if err == nil {
		return RecipeArtifactRead{Bytes: data, Exists: true}
	}
	if !errors.Is(err, os.ErrNotExist) {
		return RecipeArtifactRead{Exists: true, Err: err}
	}
	for p := path; ; p = filepath.Dir(p) {
		info, statErr := os.Lstat(p)
		if statErr == nil {
			if p == path || !info.IsDir() {
				return RecipeArtifactRead{Exists: true, Err: err}
			}
			return RecipeArtifactRead{}
		}
		if !errors.Is(statErr, os.ErrNotExist) || filepath.Dir(p) == p {
			return RecipeArtifactRead{Exists: true, Err: statErr}
		}
	}
}

// RecipeCoverageSnapshot is one immutable five-artifact capture. Assessments
// consume it without rereading; execution also loads these exact recipe bytes.
type RecipeCoverageSnapshot struct {
	Coverage, Event, Patch, Recipe, Marker RecipeArtifactRead
}

func SnapshotRecipeCoverage(root, slug string) RecipeCoverageSnapshot {
	read := func(name string) RecipeArtifactRead {
		return ReadRecipeArtifact(filepath.Join(root, ".tpatch", "features", slug, "artifacts", name))
	}
	return RecipeCoverageSnapshot{
		Coverage: read("recipe-coverage.json"), Event: read("recipe-capture-event.json"),
		Patch: read("post-apply.patch"), Recipe: read("apply-recipe.json"), Marker: read("recipe-stale.json"),
	}
}

func coverageReadArtifact(slug, name string, r RecipeArtifactRead) CoverageArtifact {
	return CoverageArtifact{
		Present: r.Exists && r.Err == nil, Bytes: r.Bytes, ReadError: r.Err,
		Path: filepath.ToSlash(filepath.Join(".tpatch", "features", slug, "artifacts", name)),
	}
}

type RecipeCoverageAssessment struct {
	Rung        int
	Code        string
	Path        string
	Detail      string
	Coverage    RecipeCoverage
	Reasons     []string
	Paths       []string
	Limitations []string
}

// AssessRecipeCoverage implements D13's six first-match rungs. gate admits all
// object reads for verify through its existing >=2.36 offline gateway.
func AssessRecipeCoverage(root, slug string, snap RecipeCoverageSnapshot, gate func() error) RecipeCoverageAssessment {
	result := RecipeCoverageAssessment{}
	fail := func(rung int, code, name, detail string) RecipeCoverageAssessment {
		result.Rung, result.Code, result.Detail = rung, code, detail
		result.Path = coverageReadArtifact(slug, name, RecipeArtifactRead{}).Path
		return result
	}
	if !snap.Coverage.Exists && len(snap.Coverage.Bytes) != 0 {
		return fail(1, "recipe-coverage-malformed", "recipe-coverage.json", "coverage snapshot contradicts genuine absence")
	}
	if !snap.Coverage.Exists && snap.Coverage.Err == nil {
		return fail(5, "recipe-coverage-missing", "recipe-coverage.json", "coverage is absent; absence is not replay authority")
	}
	if snap.Coverage.Err != nil {
		return fail(1, "recipe-coverage-malformed", "recipe-coverage.json", snap.Coverage.Err.Error())
	}
	c, err := DecodeRecipeCoverage(snap.Coverage.Bytes)
	if err != nil {
		// Decoder errors can include JSON source tokens; do not echo bodies.
		return fail(1, "recipe-coverage-malformed", "recipe-coverage.json", "strict coverage decoding failed")
	}
	result.Coverage = c
	if c.Feature != slug {
		return fail(2, "recipe-coverage-owner-mismatch", "recipe-coverage.json", "coverage envelope belongs to another feature")
	}
	patch := coverageReadArtifact(slug, "post-apply.patch", snap.Patch)
	recipe := coverageReadArtifact(slug, "apply-recipe.json", snap.Recipe)
	for _, bound := range []struct {
		artifact         CoverageArtifact
		present          bool
		hash, code, name string
	}{
		{patch, c.PatchPresent, c.PatchSHA256, "recipe-coverage-patch-changed", "post-apply.patch"},
		{recipe, c.RecipePresent, c.RecipeSHA256, "recipe-coverage-recipe-changed", "apply-recipe.json"},
	} {
		if !bound.artifact.Present && len(bound.artifact.Bytes) != 0 {
			return fail(2, bound.code, bound.name, "artifact snapshot contradicts readable existence")
		}
		hash := ""
		if bound.artifact.Present {
			hash = CoverageSHA256(bound.artifact.Bytes)
		}
		if bound.present != bound.artifact.Present || bound.hash != hash {
			detail := "readable presence or exact raw-byte hash differs"
			if !bound.artifact.Present {
				detail += ": " + recipeReadStatus(bound.artifact)
			}
			return fail(2, bound.code, bound.name, detail)
		}
	}
	_, decodeErr := decodeCoverageRecipe(recipe.Bytes)
	if c.RecipeDecodable != (recipe.Present && decodeErr == nil) {
		return fail(2, "recipe-coverage-recipe-changed", "apply-recipe.json", "strict recipe decodability differs")
	}
	const eventCode = "recipe-coverage-capture-evidence-invalid"
	if snap.Event.Err != nil {
		return fail(2, eventCode, "recipe-capture-event.json", snap.Event.Err.Error())
	}
	if !snap.Event.Exists {
		return fail(2, eventCode, "recipe-capture-event.json", "required capture evidence is absent")
	}
	e, err := DecodeRecipeCaptureEvent(snap.Event.Bytes)
	if err != nil {
		return fail(2, eventCode, "recipe-capture-event.json", "strict capture-evidence decoding failed")
	}
	if err := ValidateRecipeCaptureEventPair(e, snap.Coverage.Bytes, RecipeCaptureBindings{
		RepoRoot: root, Feature: slug, Patch: patch, Recipe: recipe,
	}); err != nil {
		return fail(2, eventCode, "recipe-capture-event.json", err.Error())
	}
	limitations, err := reconstructRecipeCoverage(root, c, e, patch, recipe, gate)
	if err != nil {
		return fail(2, "recipe-coverage-reference-stale", "recipe-coverage.json", err.Error())
	}
	result.Limitations = limitations
	result.Reasons, result.Paths = recipeCoverageReasons(c)
	if c.CoverageStatus == CoverageIncomplete {
		return fail(3, "recipe-coverage-incomplete", "recipe-coverage.json", "incomplete coverage is not replay authority")
	}
	if snap.Marker.Exists || snap.Marker.Err != nil {
		detail := "current stale marker is present; coverage is not replay authority"
		if snap.Marker.Err != nil {
			detail += ": " + snap.Marker.Err.Error()
		}
		return fail(4, "recipe-coverage-stale-marker", "recipe-stale.json", detail)
	}
	result.Rung = 6
	return result
}

func recipeReadStatus(a CoverageArtifact) string {
	if a.ReadError != nil {
		return "unreadable: " + a.ReadError.Error()
	}
	if !a.Present {
		return "absent"
	}
	return "readable but undecodable"
}

func reconstructedObservation(root string, c RecipeCoverage, e RecipeCaptureEvent, patch CoverageArtifact) patchobs.Observation {
	obs := patchobs.Observation{
		RepoRoot: root, Slug: e.Feature, Producer: c.Producer,
		PatchPresent: patch.Present, PatchBytes: patch.Bytes, PatchSHA256: e.PatchSHA256,
		Reference:          patchobs.ReferenceDescriptor{Kind: e.Reference.Kind, Commit: e.Reference.Commit, PreimageSetSHA256: e.Reference.PreimageSetSHA256},
		Capture:            patchobs.CaptureDescriptor{Mode: e.Capture.Mode, Pathspecs: e.Capture.Pathspecs, ClaimIDs: e.Capture.ClaimIDs},
		ParentCreatedPaths: e.ParentCreatedPaths,
	}
	return obs
}

func (a RecipeCoverageAssessment) BindingFailure() bool { return a.Rung == 1 || a.Rung == 2 }

func (a RecipeCoverageAssessment) Diagnostic() string {
	return fmt.Sprintf("%s: %s: %s", a.Code, a.Path, a.Detail)
}
