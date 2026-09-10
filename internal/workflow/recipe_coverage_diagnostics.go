package workflow

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func recipeCoverageReasons(c RecipeCoverage) ([]string, []string) {
	reasons := append([]string{}, c.Reasons...)
	paths := []string{}
	for _, effect := range c.Effects {
		reasons = append(reasons, effect.ReasonCodes...)
		paths = append(paths, effect.Path)
		if effect.OldPath != "" {
			paths = append(paths, effect.OldPath)
		}
	}
	return coverageSortedCopy(reasons), coverageSortedCopy(paths)
}

func (a RecipeCoverageAssessment) explanation() string {
	text := a.Diagnostic()
	if len(a.Reasons) != 0 {
		text += "; reasons: " + strings.Join(a.Reasons, ", ")
	}
	if len(a.Paths) != 0 {
		text += "; affected paths: " + strings.Join(a.Paths, ", ")
	}
	if len(a.Limitations) != 0 {
		text += "; reconstruction limitations: " + strings.Join(a.Limitations, "; ")
	}
	return text
}

func inventoryCoverageSnapshot(entry *inventoryEntry) RecipeCoverageSnapshot {
	read := func(a artifactSnapshot) RecipeArtifactRead {
		return RecipeArtifactRead{Exists: a.Presence != "" && a.Presence != PresenceAbsent || a.Err != nil, Bytes: a.Bytes, Err: a.Err}
	}
	return RecipeCoverageSnapshot{Coverage: read(entry.Coverage), Event: read(entry.CaptureEvent),
		Patch: read(entry.Patch), Recipe: read(entry.Recipe), Marker: read(entry.StaleMarker)}
}

func checkRecipeGenerationCoverage(ctx *verifyRunContext, s *store.Store, slug string) store.VerifyCheckResult {
	entry := inventoryEntryOrEmpty(ctx, slug)
	snapshot := inventoryCoverageSnapshot(entry)
	a := AssessRecipeCoverage(ctx.root, slug, snapshot, ctx.gitGate)
	row := store.VerifyCheckResult{ID: CheckRecipeGenerationCoverage, Severity: SeverityBlock, Passed: a.Rung == 6}
	if a.Rung >= 3 && a.Rung <= 5 {
		row.Severity = SeverityWarn
	}
	if !row.Passed {
		row.Remediation = a.explanation()
		if entry.Status != nil && ctx.gitGate() == nil {
			plan := PlanDefaultRecord(s, *entry.Status, snapshot, time.Now().UTC(), ctx.inv)
			row.Remediation += "; " + plan.Remediation(slug)
		} else {
			row.Remediation += "; recipe-generation-no-truthful-regeneration: local record planning unavailable; manual review required"
		}
	}
	return row
}

// RecipeExecutePreflight returns only the recipe from the assessed snapshot.
// A nonempty refusal is a named exit-2 condition. Legacy errors remain errors
// so the CLI retains their existing exit code 1.
func RecipeExecutePreflight(root string, status store.FeatureStatus, snap RecipeCoverageSnapshot) (recipe ApplyRecipe, warning, refusal string, err error) {
	a := AssessRecipeCoverage(root, status.Slug, snap, nil)
	if a.BindingFailure() {
		return recipe, "", a.explanation(), nil
	}
	if a.Rung == 3 && (!a.Coverage.RecipePresent || !a.Coverage.RecipeDecodable) {
		artifact := coverageReadArtifact(status.Slug, "apply-recipe.json", snap.Recipe)
		condition := "no readable apply-recipe.json is present"
		if artifact.Present {
			condition = "apply-recipe.json was read and does not decode"
		}
		text := fmt.Sprintf("apply --mode execute refuses: the recipe coverage for %q binds no\nreadable executable recipe — %s.\n  recipe coverage: incomplete (%s)\n  recipe path: %s\n  recipe status: %s\n  affected paths: %s\n  feature state: %s",
			status.Slug, condition, strings.Join(a.Reasons, ", "), artifact.Path, recipeReadStatus(artifact), strings.Join(a.Paths, ", "), status.State)
		if artifact.Present {
			text += "\n  There is no decodable recipe to execute, and tpatch will not guess at\n  partially readable operations."
		} else {
			text += "\n  There is no readable recipe to execute, and tpatch will not synthesize a\n  partial one."
		}
		text += "\n  code: recipe-generation-incomplete"
		if status.State == store.StateApplied {
			text += fmt.Sprintf("\n  This feature is already applied: its effects are materialized in the working\n  tree, so nothing needs to be reapplied. To confirm that:\n    tpatch verify %s\n    tpatch status %s\n  To make the recipe executable later, author a complete apply-recipe.json and\n  checkpoint it with `tpatch implement %s --manual` — note that the\n  checkpoint moves this feature from applied to implementing.", status.Slug, status.Slug, status.Slug)
		} else {
			path := ".tpatch/features/" + status.Slug + "/artifacts/post-apply.patch"
			text += fmt.Sprintf("\n  The canonical patch is the authority for this feature. Review it, then apply\n  it yourself — tpatch will not do this silently on your behalf:\n    %s\n    git apply --check %s\n    git apply %s\n  Or author a complete apply-recipe.json and checkpoint it:\n    tpatch implement %s --manual\n  The checkpoint moves this feature to state implementing.",
				path, path, path, status.Slug)
		}
		if len(a.Limitations) != 0 {
			text += "\n  reconstruction limitations: " + strings.Join(a.Limitations, "; ")
		}
		return recipe, "", text, nil
	}
	if a.Rung == 3 {
		warning = a.explanation()
	}
	readErr := snap.Recipe.Err
	if !snap.Recipe.Exists && readErr == nil {
		readErr = os.ErrNotExist
	}
	recipe, err = LoadRecipeBytes(status.Slug, snap.Recipe.Bytes, readErr)
	return recipe, warning, "", err
}
