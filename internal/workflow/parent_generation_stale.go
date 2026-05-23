package workflow

import "github.com/tesseracode/tesserapatch/internal/store"

// ParentGenerationStale reports ADR-026 D5's read-time overlay for children
// whose latest patch generation was captured against an older parent generation
// or parent patch SHA than the parent's current patch-generations manifest.
func ParentGenerationStale(s *store.Store, slug string) bool {
	childManifest, err := store.LoadPatchGenerations(s, slug)
	if err != nil {
		return false
	}
	childLatest, ok := store.LatestPatchGeneration(childManifest)
	if !ok {
		return false
	}
	for _, dep := range childLatest.Dependencies {
		if dep.ParentGeneration == 0 && dep.ParentPatchSHA256 == "" {
			continue
		}
		parentManifest, perr := store.LoadPatchGenerations(s, dep.Slug)
		if perr != nil {
			continue
		}
		parentLatest, pok := store.LatestPatchGeneration(parentManifest)
		if !pok {
			continue
		}
		if dep.ParentGeneration != parentLatest.Generation || dep.ParentPatchSHA256 != parentLatest.PatchSHA256 {
			return true
		}
	}
	return false
}
