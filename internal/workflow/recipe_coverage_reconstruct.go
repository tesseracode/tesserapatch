package workflow

import (
	"fmt"
	"slices"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
)

func reconstructRecipeCoverage(root string, c RecipeCoverage, e RecipeCaptureEvent, patch, recipe CoverageArtifact, gate func() error) ([]string, error) {
	obs := reconstructedObservation(root, c, e, patch)
	parsed, parseErr := gitutil.NormalizePatchEffects(string(patch.Bytes))
	if parseErr != nil {
		obs.ParseRefusal = "strict patch grammar refused"
	}
	var reconstructed []patchobs.ReconstructedEffect
	limitations := []string{}
	if e.Reference.Kind == patchobs.ReferenceKindCommit {
		if gate != nil {
			if err := gate(); err != nil {
				return nil, err
			}
		}
		// Even a missing/empty/unparseable patch must prove commit object type.
		text := string(patch.Bytes)
		if parseErr != nil {
			text = ""
		}
		var err error
		reconstructed, err = patchobs.Reconstruct(root, e.Reference.Commit, text)
		if err != nil {
			return nil, err
		}
	} else if len(e.Observations) != 0 {
		limitations = append(limitations, "historical images have no durable reference")
	}
	full := len(limitations) == 0
	for i, observed := range e.Observations {
		effect := parsed[i]
		projection := observed.patchEffect()
		projection.HeaderOldMode, projection.HeaderNewMode = effect.HeaderOldMode, effect.HeaderNewMode
		projection.FragmentStart, projection.FragmentEnd = effect.FragmentStart, effect.FragmentEnd
		projection.BinaryStanza = effect.BinaryStanza
		row := patchobs.EffectObservation{Effect: projection}
		for _, side := range []struct {
			name              string
			observed, present bool
			mode, hash        string
			post              bool
		}{
			{"preimage", observed.PreimageObserved, observed.PreimagePresent, observed.OldMode, observed.PreimageSHA256, false},
			{"postimage", observed.PostimageObserved, observed.PostimagePresent, observed.NewMode, observed.PostimageSHA256, true},
		} {
			if !side.observed {
				full = false
				limitations = append(limitations, fmt.Sprintf("%s: %s was not observed at publication", effect.Path, side.name))
			}
			if len(reconstructed) == 0 {
				full = false
				continue
			}
			actual := reconstructed[i].Pre
			if side.post {
				actual = reconstructed[i].Post
			}
			if !actual.Available {
				full = false
				dependentBudget := side.post && !observed.PreimageObserved &&
					actual.Limitation == "preimage unavailable within image retention budget"
				if side.observed && !actual.PayloadAbsent && !dependentBudget {
					return nil, fmt.Errorf("%s: required %s cannot be reconstructed: %s", effect.Path, side.name, actual.Limitation)
				}
				limitations = append(limitations, fmt.Sprintf("%s: %s: %s; body hash has pair consistency only", effect.Path, side.name, actual.Limitation))
				continue
			}
			if side.observed && (side.present != actual.Present || side.mode != actual.Mode || side.hash != actual.SHA256) {
				return nil, fmt.Errorf("%s: independently reconstructed %s presence/mode/hash differs", effect.Path, side.name)
			}
			if side.observed {
				if side.post {
					row.Bytes.Postimage = actual.Bytes
				} else {
					row.Bytes.Preimage = actual.Bytes
				}
			}
		}
		if len(reconstructed) != 0 && reconstructed[i].Pre.Available && reconstructed[i].Post.Available {
			content, object := patchobs.ClassifyEffectObservation(row.Effect, row.Bytes)
			if content != observed.ContentKind || object != observed.ObjectKind {
				return nil, fmt.Errorf("%s: independently reconstructed object/content kind differs", effect.Path)
			}
		}
		obs.Effects = append(obs.Effects, row)
	}
	if full {
		if err := ValidateRecipeCoverage(c, RecipeCoverageInput{Observation: obs, Recipe: recipe, Events: e.events()}); err != nil {
			return nil, fmt.Errorf("coverage differs from independently reconstructed S3 semantics")
		}
	} else {
		if c.CoverageStatus != CoverageIncomplete || c.CrossBaseStatus != CrossBaseUnsupported {
			return nil, fmt.Errorf("complete coverage lacks full independent content proof")
		}
		// Unsupported or historically unobserved effects force S3's simulation
		// to fail independently of the unavailable payload's bytes.
		for _, effect := range c.Effects {
			if c.RecipeDecodable && (effect.ContentKind != gitutil.ContentKindText ||
				!effect.PreimageObserved || !effect.PostimageObserved ||
				(effect.ChangeKind != gitutil.ChangeKindAdd && effect.ChangeKind != gitutil.ChangeKindModify)) &&
				!slices.Contains(c.Reasons, "simulation-mismatch") {
				return nil, fmt.Errorf("limited observation omits required simulation-mismatch")
			}
		}
	}
	return coverageSortedCopy(limitations), nil
}
