package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tesseracode/tesserapatch/internal/store"
)

const patchGenerationsRemediationPrefix = "run tpatch feature patch refresh "

func runDoctorD2(ctx *doctorContext) {
	for _, feature := range ctx.features {
		manifestPath := ctx.store.PatchGenerationsPath(feature.Slug)
		postApplyPath := filepath.Join(feature.Dir, "artifacts", "post-apply.patch")
		manifestExists := fileExistsDoctor(manifestPath)
		needsManifest := fileExistsDoctor(postApplyPath)
		if feature.Status != nil && feature.Status.Apply.HasPatch {
			needsManifest = true
		}
		if needsManifest && !manifestExists {
			ctx.addFinding(DoctorFinding{
				CheckID:     "D2",
				Code:        "patch-generations-missing",
				Severity:    "drift",
				Feature:     feature.Slug,
				Path:        relOrAbs(ctx.root, manifestPath),
				Message:     "patch-generations.json missing for feature with captured patch",
				Fixable:     false,
				Remediation: patchGenerationsRemediationPrefix + feature.Slug,
			})
			continue
		}
		if !manifestExists {
			continue
		}
		if _, err := store.LoadPatchGenerations(ctx.store, feature.Slug); err != nil {
			severity := "error"
			code := "patch-generations-unreadable"
			if errors.Is(err, store.ErrMalformedManifest) {
				severity = "drift"
				code = "patch-generations-malformed"
			}
			ctx.addFinding(DoctorFinding{
				CheckID:     "D2",
				Code:        code,
				Severity:    severity,
				Feature:     feature.Slug,
				Path:        relOrAbs(ctx.root, manifestPath),
				Line:        lineForJSONError(manifestPath, err),
				Message:     fmt.Sprintf("patch-generations.json failed production validation: %v", err),
				Fixable:     false,
				Remediation: patchGenerationsRemediationPrefix + feature.Slug,
			})
		}
	}
}

func fileExistsDoctor(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
