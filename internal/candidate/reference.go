package candidate

import (
	"fmt"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/patchobs"
)

// CheckTree proves the bound commit's tree is available without fetching it.
func (r *ReferenceRoot) CheckTree() error {
	if err := r.ensureAvailable(); err != nil {
		return err
	}
	_, stderr, err := r.runGitWithOutput("cat-file", "-e", r.commit+"^{tree}")
	if err != nil {
		return fmt.Errorf("reference tree unavailable: %v: %s", err, strings.TrimSpace(stderr))
	}
	return nil
}

// ReconstructPatch obtains bounded immutable pre/postimages, never live files.
func (r *ReferenceRoot) ReconstructPatch(patch string) ([]patchobs.ReconstructedEffect, error) {
	if err := r.ensureAvailable(); err != nil {
		return nil, err
	}
	effects, err := patchobs.ReconstructOffline(r.repo.path, r.commit, patch)
	if identityErr := r.ensureAvailable(); identityErr != nil {
		return nil, identityErr
	}
	return effects, err
}
