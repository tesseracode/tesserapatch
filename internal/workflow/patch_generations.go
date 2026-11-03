package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
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
	candidateStage         *operationCandidateGenerationStage
}

// The acceptance variant uses the same appender, but retains its output for
// the journaled publication rather than changing the canonical manifest early.
type operationCandidateGenerationStage struct {
	recipe        []byte
	priorPresent  bool
	priorManifest []byte
	manifest      []byte
}

// AppendPatchGenerationForFeature appends ADR-024 patch identity metadata when
// canonical patch bytes change. Missing manifests start at generation 1; latest
// same-byte captures are no-ops except the operator-approved ADR-051 candidate
// acceptance variant when its complete normalized binding tuple changes.
func AppendPatchGenerationForFeature(s *store.Store, slug string, in PatchGenerationInput) (bool, error) {
	if in.candidateStage != nil && (in.Kind != "reconcile" || in.Intent != "") {
		return false, fmt.Errorf("candidate generation staging requires the reconcile acceptance kind")
	}
	var manifest store.PatchGenerationsManifest
	var err error
	if in.candidateStage == nil {
		manifest, err = store.LoadPatchGenerations(s, slug)
	} else {
		manifest = store.PatchGenerationsManifest{Version: store.PatchGenerationsManifestVersion, Feature: slug, Generations: []store.PatchGeneration{}}
		if in.candidateStage.priorPresent {
			decoder := json.NewDecoder(bytes.NewReader(in.candidateStage.priorManifest))
			decoder.DisallowUnknownFields()
			err = decoder.Decode(&manifest)
			if err == nil && decoder.Decode(new(any)) != io.EOF {
				err = fmt.Errorf("patch generation snapshot has trailing input")
			}
		}
		if err == nil {
			err = store.ValidatePatchGenerations(slug, manifest)
		}
	}
	if err != nil {
		if in.AllowMalformedManifest && errors.Is(err, store.ErrMalformedManifest) {
			return false, nil
		}
		return false, err
	}
	patchSHA := store.SHA256HexString(in.Patch)
	preserveHistory := false
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
		if in.candidateStage == nil {
			return false, nil
		}
		if operationCandidateGenerationBindingsMatch(latest, in) {
			in.candidateStage.manifest = bytes.Clone(in.candidateStage.priorManifest)
			return false, nil
		}
		preserveHistory = true
	}
	var recipeSHA string
	if in.candidateStage != nil {
		recipeSHA = store.SHA256HexBytes(in.candidateStage.recipe)
	} else {
		recipeSHA, err = recipeSHA256(s, slug)
	}
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
	touched, terr := strictTouchedPaths(in.Patch)
	if terr != nil {
		// PI-3: this reparse is the LAST line of defence, not the first.
		// Every producer that reaches this function has already run the
		// same bytes through the strict grammar in its own discovery
		// window and refused before its first bound write, so a refusal
		// here means a caller skipped that preflight — a bug in the
		// caller, surfaced rather than degraded into a silently short
		// `touched_paths` audit list.
		//
		// It is deliberately NOT treated as infallible. The earlier claim
		// that "every caller already tolerates an error" was wrong: two
		// of the three callers return it, and by the time control
		// reaches here the canonical patch and the numbered snapshot
		// have already been written. That is precisely why the preflight
		// exists upstream.
		return false, fmt.Errorf("patch-generations.json: touched_paths: %w", terr)
	}
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
	var previous []store.PatchGeneration
	if preserveHistory {
		previous = slices.Clone(manifest.Generations)
	}
	changed, err := store.AppendPatchGeneration(&manifest, g)
	if err != nil || !changed {
		return changed, err
	}
	if in.candidateStage != nil {
		if preserveHistory {
			// The canonical bytes still match the old generation. Preserve its
			// truthful canonical pointer as well as every historical payload.
			copy(manifest.Generations, previous)
		}
		data, err := encodeOperationCandidateGenerationManifest(manifest, in.candidateStage.priorManifest, preserveHistory)
		if err != nil {
			return false, err
		}
		in.candidateStage.manifest = data
		return true, nil
	}
	if err := store.SavePatchGenerations(s, manifest); err != nil {
		return false, err
	}
	return true, nil
}

func operationCandidateGenerationBindingsMatch(latest store.PatchGeneration, in PatchGenerationInput) bool {
	return latest.BaseCommit == in.BaseCommit &&
		latest.RecipeSHA256 == store.SHA256HexBytes(in.candidateStage.recipe) &&
		latest.Upper == in.Upper && latest.Capture.Mode == in.Capture.Mode &&
		slices.Equal(sortedCopy(latest.Capture.Pathspecs), sortedCopy(in.Capture.Pathspecs)) &&
		slices.Equal(sortedCopy(latest.Capture.ClaimIDs), sortedCopy(in.Capture.ClaimIDs))
}

func encodeOperationCandidateGenerationManifest(manifest store.PatchGenerationsManifest, prior []byte, preserveHistory bool) ([]byte, error) {
	if err := store.ValidatePatchGenerations(manifest.Feature, manifest); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	if preserveHistory {
		var original struct {
			Generations []json.RawMessage `json:"generations"`
		}
		if err := json.Unmarshal(prior, &original); err != nil {
			return nil, err
		}
		if len(original.Generations)+1 != len(manifest.Generations) {
			return nil, fmt.Errorf("candidate generation append lost its prior entry inventory")
		}
		for i, raw := range original.Generations {
			canonical, err := json.MarshalIndent(manifest.Generations[i], "    ", "  ")
			if err != nil {
				return nil, err
			}
			// Generation numbers make these complete object encodings unique.
			// Preserve even a previously reformatted row, not just its values.
			if bytes.Count(data, canonical) != 1 {
				return nil, fmt.Errorf("candidate generation append cannot identify historical entry %d", i+1)
			}
			data = bytes.Replace(data, canonical, raw, 1)
		}
	}
	return append(data, '\n'), nil
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
