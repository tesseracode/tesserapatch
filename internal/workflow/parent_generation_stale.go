package workflow

import "github.com/tesseracode/tesserapatch/internal/store"

// ParentGenerationStaleDependency describes one child dependency snapshot that
// no longer matches the parent's current patch-generation metadata.
type ParentGenerationStaleDependency struct {
	Slug                      string
	Kind                      string
	SnapshotParentGeneration  int
	SnapshotParentPatchSHA256 string
	CurrentGeneration         int
	CurrentGenerationID       string
	CurrentPatchSHA256        string
}

// ParentGenerationStale reports ADR-026 D5's read-time overlay for children
// whose latest patch generation was captured against an older parent generation
// or parent patch SHA than the parent's current patch-generations manifest.
func ParentGenerationStale(s *store.Store, slug string) bool {
	return len(ParentGenerationStaleDependencies(s, slug)) > 0
}

// ParentGenerationStaleDependencies returns every stale parent-generation
// snapshot for slug's latest patch generation. Legacy parents without
// patch-generations.json are ignored, matching ADR-026 D5.
func ParentGenerationStaleDependencies(s *store.Store, slug string) []ParentGenerationStaleDependency {
	childManifest, err := store.LoadPatchGenerations(s, slug)
	if err != nil {
		return nil
	}
	childLatest, ok := store.LatestPatchGeneration(childManifest)
	if !ok {
		return nil
	}
	var stale []ParentGenerationStaleDependency
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
			stale = append(stale, ParentGenerationStaleDependency{
				Slug:                      dep.Slug,
				Kind:                      dep.Kind,
				SnapshotParentGeneration:  dep.ParentGeneration,
				SnapshotParentPatchSHA256: dep.ParentPatchSHA256,
				CurrentGeneration:         parentLatest.Generation,
				CurrentGenerationID:       parentLatest.GenerationID,
				CurrentPatchSHA256:        parentLatest.PatchSHA256,
			})
		}
	}
	return stale
}
