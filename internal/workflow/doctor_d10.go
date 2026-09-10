package workflow

import (
	"fmt"
	"time"
)

// D10 is diagnostic only, including under --fix. Neither assessment nor the
// shared future-producer plan allocates a temporary index or emits an event.
func runDoctorD10(ctx *doctorContext) {
	inventory, err := buildInventory(ctx.store)
	if err != nil {
		ctx.addFinding(DoctorFinding{CheckID: "D10", Code: "check-error", Severity: "warning",
			Message: fmt.Sprintf("feature inventory cannot be read: %v", err), Fixable: false,
			Remediation: "recipe-generation-no-truthful-regeneration: review feature inventory manually"})
		return
	}
	for _, slug := range inventory.Order {
		feature := inventory.Entry(slug)
		if feature.Status == nil || feature.Err != nil {
			continue
		}
		snap := inventoryCoverageSnapshot(feature)
		a := AssessRecipeCoverage(ctx.root, feature.Slug, snap, nil)
		if a.Rung == 6 {
			continue
		}
		if a.Rung == 5 && !snap.Patch.Exists && snap.Patch.Err == nil && !snap.Recipe.Exists && snap.Recipe.Err == nil {
			continue
		}
		plan := PlanDefaultRecord(ctx.store, *feature.Status, snap, time.Now().UTC(), inventory)
		ctx.addFinding(DoctorFinding{CheckID: "D10", Code: a.Code, Severity: "warning",
			Feature: feature.Slug, Path: a.Path, Message: a.explanation(), Fixable: false,
			Remediation: plan.Remediation(feature.Slug)})
	}
}
