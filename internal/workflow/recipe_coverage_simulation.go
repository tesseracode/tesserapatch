package workflow

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
)

// CoverageSimulation is ephemeral evidence, not another wire schema or a
// consumer authorization. It holds no body, anchor or live-tree reference.
type CoverageSimulation struct {
	ExactPostimage             bool
	AllAlreadyPresent          bool
	MismatchPaths              []string
	UnreclassifiableOperations []int
}

// SimulateRecipeCoverage reproduces operations over the captured image set.
// Every returned present-state classification is a comparison, not a second
// execution. The existing append/replace execution implementations are untouched.
func SimulateRecipeCoverage(obs patchobs.Observation, recipe ApplyRecipe) (CoverageSimulation, error) {
	_, err := coverageObservation(obs)
	if err != nil {
		return CoverageSimulation{}, err
	}
	raw, err := EncodeRecipe(recipe)
	if err != nil {
		return CoverageSimulation{}, err
	}
	if _, err := decodeCoverageRecipe(raw); err != nil {
		return CoverageSimulation{}, err
	}
	assignments, surplus, err := coverageAssignments(obs, recipe)
	if err != nil {
		return CoverageSimulation{}, err
	}
	return simulateCoverage(obs, recipe, assignments, surplus), nil
}

func coverageAssignments(obs patchobs.Observation, recipe ApplyRecipe) ([][]int, bool, error) {
	assignments := make([][]int, len(obs.Effects))
	byPath := map[string]int{}
	for i, entry := range obs.Effects {
		assignments[i] = []int{}
		path, err := coveragePath(obs.RepoRoot, entry.Effect.Path)
		if err != nil {
			continue
		}
		if _, duplicate := byPath[path]; duplicate {
			return nil, false, fmt.Errorf("coverage: duplicate normalized effect target %q", path)
		}
		byPath[path] = i
	}
	surplus := false
	claimed := map[string]bool{}
	for i, op := range recipe.Operations {
		path, err := coveragePath(obs.RepoRoot, op.Path)
		if err != nil {
			surplus = true
			continue
		}
		if claimed[path] {
			return nil, false, fmt.Errorf("coverage: duplicate operation target %q", path)
		}
		claimed[path] = true
		effect, found := byPath[path]
		if !found || op.Type == "ensure-directory" {
			surplus = true
			continue
		}
		assignments[effect] = append(assignments[effect], i+1)
	}
	return assignments, surplus, nil
}

func simulateCoverage(obs patchobs.Observation, recipe ApplyRecipe, assignments [][]int, surplus bool) CoverageSimulation {
	out := CoverageSimulation{
		ExactPostimage:    len(obs.Effects) != 0 && len(recipe.Operations) != 0 && !surplus,
		AllAlreadyPresent: len(recipe.Operations) != 0 && !surplus,
		MismatchPaths:     []string{}, UnreclassifiableOperations: []int{},
	}
	assigned := map[int]bool{}
	for i, entry := range obs.Effects {
		e := entry.Effect
		supported := (e.ChangeKind == gitutil.ChangeKindAdd || e.ChangeKind == gitutil.ChangeKindModify) &&
			e.PreimageObserved && e.PostimageObserved && e.ContentKind == gitutil.ContentKindText &&
			e.NewMode == gitutil.ModeRegular && (!e.PreimagePresent || e.OldMode == gitutil.ModeRegular)
		exact := supported && len(assignments[i]) == 1
		for _, index := range assignments[i] {
			assigned[index] = true
			op := recipe.Operations[index-1]
			opExact, reclassifiable := simulateCoverageOperation(entry, op, supported)
			exact = exact && opExact
			if !reclassifiable {
				out.AllAlreadyPresent = false
				if op.Type == "append-file" || op.Type == "replace-in-file" {
					out.UnreclassifiableOperations = append(out.UnreclassifiableOperations, index)
				}
			}
		}
		if !exact {
			out.ExactPostimage = false
			out.MismatchPaths = append(out.MismatchPaths, e.Path)
			if e.OldPath != "" {
				out.MismatchPaths = append(out.MismatchPaths, e.OldPath)
			}
		}
	}
	for i, op := range recipe.Operations {
		if !assigned[i+1] {
			out.ExactPostimage, out.AllAlreadyPresent = false, false
			out.MismatchPaths = append(out.MismatchPaths, op.Path)
		}
	}
	out.MismatchPaths = coverageSortedCopy(out.MismatchPaths)
	slices.Sort(out.UnreclassifiableOperations)
	return out
}

func simulateCoverageOperation(entry patchobs.EffectObservation, op RecipeOperation, supported bool) (exact, reclassifiable bool) {
	if op.Type == "append-file" {
		if !supported || !entry.Effect.PreimagePresent {
			return false, false
		}
		// Compare segments without allocating another retained image.
		pre, post := entry.Bytes.Preimage, entry.Bytes.Postimage
		return len(post) == len(pre)+len(op.Content) &&
			bytes.Equal(post[:min(len(pre), len(post))], pre) &&
			bytes.Equal(post[min(len(pre), len(post)):], []byte(op.Content)), false
	}
	if !supported {
		return false, false
	}
	switch op.Type {
	case "write-file":
		if op.PreimageHash != nil {
			want := *op.PreimageHash
			already := entry.Effect.PreimagePresent && bytes.Equal(entry.Bytes.Preimage, []byte(op.Content))
			if !already && ((want == "" && entry.Effect.PreimagePresent) ||
				(want != "" && (!entry.Effect.PreimagePresent || want != "sha256:"+entry.Effect.PreimageSHA256))) {
				return false, false
			}
		}
		// A fixed write's result is exactly its content and mode 100644.
		// Reclassification compares that result with the operation postimage,
		// without writing again, regardless of manual/provider formatting.
		return bytes.Equal(entry.Bytes.Postimage, []byte(op.Content)), true
	case "replace-in-file":
		if !entry.Effect.PreimagePresent {
			return false, false
		}
		pre, post := entry.Bytes.Preimage, entry.Bytes.Postimage
		at := bytes.Index(pre, []byte(op.Search))
		if at < 0 {
			return false, false
		}
		end := at + len(op.Search)
		length := len(pre) - len(op.Search) + len(op.Replace)
		exact := len(post) == length && at <= len(post) && at+len(op.Replace) <= len(post) &&
			bytes.Equal(post[:at], pre[:at]) &&
			bytes.Equal(post[at:at+len(op.Replace)], []byte(op.Replace)) &&
			bytes.Equal(post[at+len(op.Replace):], pre[end:])
		// This is D5's narrow, in-process exact-postimage exception. It
		// derives no contextual anchor and never calls replace on the result.
		reclassifiable := exact && op.Search != "" && op.Replace != "" &&
			!bytes.Contains(post, []byte(op.Search))
		return exact, reclassifiable
	default:
		return false, false
	}
}
