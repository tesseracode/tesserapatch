package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func runDoctorD1(ctx *doctorContext) {
	for _, feature := range ctx.features {
		if feature.StatusErr != nil {
			ctx.addFinding(DoctorFinding{
				CheckID:     "D1",
				Code:        "feature-metadata-malformed",
				Severity:    "error",
				Feature:     feature.Slug,
				Path:        relOrAbs(ctx.root, feature.StatusPath),
				Line:        lineForJSONError(feature.StatusPath, feature.StatusErr),
				Message:     fmt.Sprintf("status.json is malformed or unreadable: %v", feature.StatusErr),
				Fixable:     false,
				Remediation: "inspect and repair the feature metadata manually",
			})
		} else if feature.Status != nil {
			validateDoctorStatus(ctx, feature)
		}
		featureYAML := filepath.Join(feature.Dir, "feature.yaml")
		if _, err := os.Stat(featureYAML); err == nil {
			ctx.addFinding(DoctorFinding{
				CheckID:     "D1",
				Code:        "legacy-feature-yaml-unsupported",
				Severity:    "drift",
				Feature:     feature.Slug,
				Path:        relOrAbs(ctx.root, featureYAML),
				Field:       "feature.yaml",
				Message:     "legacy feature.yaml metadata is not part of the current feature metadata contract",
				Fixable:     false,
				Remediation: "inspect the legacy feature.yaml manually; status.json remains the current metadata source",
			})
		} else if err != nil && !os.IsNotExist(err) {
			ctx.addFinding(DoctorFinding{
				CheckID:  "D1",
				Code:     "feature-metadata-unreadable",
				Severity: "error",
				Feature:  feature.Slug,
				Path:     relOrAbs(ctx.root, featureYAML),
				Message:  fmt.Sprintf("cannot stat feature.yaml: %v", err),
				Fixable:  false,
			})
		}
	}
}

func validateDoctorStatus(ctx *doctorContext, feature doctorFeature) {
	data, err := os.ReadFile(feature.StatusPath)
	if err != nil {
		ctx.addFinding(DoctorFinding{
			CheckID:  "D1",
			Code:     "feature-metadata-unreadable",
			Severity: "error",
			Feature:  feature.Slug,
			Path:     relOrAbs(ctx.root, feature.StatusPath),
			Message:  fmt.Sprintf("cannot read status.json: %v", err),
			Fixable:  false,
		})
		return
	}
	var strict store.FeatureStatus
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&strict); err != nil {
		ctx.addFinding(DoctorFinding{
			CheckID:     "D1",
			Code:        "feature-metadata-unsupported-field",
			Severity:    "error",
			Feature:     feature.Slug,
			Path:        relOrAbs(ctx.root, feature.StatusPath),
			Line:        lineForJSONErrorBytes(data, err),
			Message:     fmt.Sprintf("status.json does not match the current FeatureStatus schema: %v", err),
			Fixable:     false,
			Remediation: "inspect and repair the feature metadata manually",
		})
		return
	}
	required := []struct {
		field string
		value string
	}{
		{"id", strict.ID},
		{"slug", strict.Slug},
		{"title", strict.Title},
		{"state", string(strict.State)},
		{"compatibility", string(strict.Compatibility)},
		{"requested_at", strict.RequestedAt},
		{"updated_at", strict.UpdatedAt},
		{"last_command", strict.LastCommand},
	}
	for _, req := range required {
		if strings.TrimSpace(req.value) == "" {
			ctx.addFinding(DoctorFinding{
				CheckID:     "D1",
				Code:        "feature-metadata-missing-field",
				Severity:    "error",
				Feature:     feature.Slug,
				Path:        relOrAbs(ctx.root, feature.StatusPath),
				Line:        1,
				Field:       req.field,
				Message:     fmt.Sprintf("status.json missing required field %q", req.field),
				Fixable:     false,
				Remediation: "inspect and repair the feature metadata manually",
			})
		}
	}
	if strict.Slug != "" && strict.Slug != feature.Slug {
		ctx.addFinding(DoctorFinding{
			CheckID:     "D1",
			Code:        "feature-metadata-slug-mismatch",
			Severity:    "error",
			Feature:     feature.Slug,
			Path:        relOrAbs(ctx.root, feature.StatusPath),
			Line:        1,
			Field:       "slug",
			Message:     fmt.Sprintf("status.json slug %q does not match feature directory %q", strict.Slug, feature.Slug),
			Fixable:     false,
			Remediation: "inspect and repair the feature metadata manually",
		})
	}
	if !store.ValidFeatureState(strict.State) {
		ctx.addFinding(DoctorFinding{
			CheckID:     "D1",
			Code:        "feature-metadata-invalid-state",
			Severity:    "error",
			Feature:     feature.Slug,
			Path:        relOrAbs(ctx.root, feature.StatusPath),
			Line:        1,
			Field:       "state",
			Message:     fmt.Sprintf("status.json state %q is not supported by this tpatch binary", strict.State),
			Fixable:     false,
			Remediation: "inspect and repair the feature metadata manually",
		})
	}
}
