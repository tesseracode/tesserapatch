package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

type PatchGenerationInput struct {
	Kind                   string
	Intent                 string
	Reason                 string
	FixupOfGeneration      string
	Patch                  string
	AuditPatch             string
	BaseCommit             string
	Upper                  store.GenerationUpper
	Capture                store.GenerationCapture
	AllowMalformedManifest bool
}

// AppendPatchGenerationForFeature appends ADR-024 patch identity metadata when
// canonical patch bytes change. Missing manifests start at generation 1; latest
// same-byte captures (including recipe-only record refreshes) are no-ops.
func AppendPatchGenerationForFeature(s *store.Store, slug string, in PatchGenerationInput) (bool, error) {
	manifest, err := store.LoadPatchGenerations(s, slug)
	if err != nil {
		if in.AllowMalformedManifest && errors.Is(err, store.ErrMalformedManifest) {
			return false, nil
		}
		return false, err
	}
	patchSHA := store.SHA256HexString(in.Patch)
	kind := in.Kind
	intent := in.Intent
	if intent == "" {
		switch in.Kind {
		case store.PatchGenerationKindRecord:
			intent = store.PatchGenerationIntentPlainRecord
		case store.PatchGenerationKindAmendRefresh:
			intent = store.PatchGenerationIntentRefresh
		case store.PatchGenerationKindAmendFixup:
			intent = store.PatchGenerationIntentFixup
		}
	}
	if intent != "" {
		classification, err := store.ClassifyPatchGenerationKind(manifest.Generations, patchSHA, intent)
		if err != nil {
			return false, err
		}
		if !classification.Append {
			return false, nil
		}
		kind = classification.Kind
	} else if latest, ok := store.LatestPatchGeneration(manifest); ok && latest.PatchSHA256 == patchSHA {
		return false, nil
	}
	recipeSHA, err := recipeSHA256(s, slug)
	if err != nil {
		return false, err
	}
	gitPatchID := ""
	if strings.TrimSpace(in.Patch) != "" {
		if pid, perr := gitutil.PatchID(s.Root, in.Patch); perr == nil {
			gitPatchID = pid
		} else if !strings.Contains(perr.Error(), "empty patch") && !strings.Contains(perr.Error(), "no output") {
			return false, fmt.Errorf("patch-generations.json: git_patch_id: %w", perr)
		}
	}
	touched := gitutil.FilesInPatch(in.Patch)
	sort.Strings(touched)
	deps := snapshotGenerationDependencies(s, slug)
	g := store.PatchGeneration{
		Kind:                kind,
		Reason:              in.Reason,
		FixupOfGeneration:   in.FixupOfGeneration,
		PatchSHA256:         patchSHA,
		GitPatchID:          gitPatchID,
		GitPatchIDAlgorithm: store.PatchIDAlgorithmStable,
		RecipeSHA256:        recipeSHA,
		AuditPatch:          in.AuditPatch,
		BaseCommit:          in.BaseCommit,
		Upper:               in.Upper,
		Capture: store.GenerationCapture{
			Mode:      in.Capture.Mode,
			Pathspecs: sortedCopy(in.Capture.Pathspecs),
			ClaimIDs:  sortedCopy(in.Capture.ClaimIDs),
		},
		TouchedPaths: touched,
		Dependencies: deps,
		Refs:         &store.GenerationRefs{},
	}
	changed, err := store.AppendPatchGeneration(&manifest, g)
	if err != nil || !changed {
		return changed, err
	}
	if err := store.SavePatchGenerations(s, manifest); err != nil {
		return false, err
	}
	return true, nil
}

func recipeSHA256(s *store.Store, slug string) (string, error) {
	path := filepath.Join(s.Root, ".tpatch", "features", slug, "artifacts", "apply-recipe.json")
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return store.SHA256HexBytes(b), nil
}

func snapshotGenerationDependencies(s *store.Store, slug string) []store.GenerationDependency {
	st, err := s.LoadFeatureStatus(slug)
	if err != nil || len(st.DependsOn) == 0 {
		return []store.GenerationDependency{}
	}
	out := make([]store.GenerationDependency, 0, len(st.DependsOn))
	for _, dep := range st.DependsOn {
		gd := store.GenerationDependency{
			Slug:               dep.Slug,
			Kind:               dep.Kind,
			SatisfiedBy:        dep.SatisfiedBy,
			ParentGeneration:   0,
			ParentPatchSHA256:  "",
			ParentRecipeSHA256: "",
		}
		if pm, perr := store.LoadPatchGenerations(s, dep.Slug); perr == nil {
			if latest, ok := store.LatestPatchGeneration(pm); ok {
				gd.ParentGeneration = latest.Generation
				gd.ParentPatchSHA256 = latest.PatchSHA256
				gd.ParentRecipeSHA256 = latest.RecipeSHA256
			}
		}
		out = append(out, gd)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

func sortedCopy(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
