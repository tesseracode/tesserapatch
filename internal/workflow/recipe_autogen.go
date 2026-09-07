package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/safety"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// RecipeStaleness is the recipe-stale.json sidecar written when
// `tpatch record` detects drift between the existing apply-recipe.json
// and the patch it just captured (typically after a `--manual`
// implement flow). The recipe itself is left untouched so a richer
// provider-generated recipe is not destroyed; the sidecar tells the
// user the recipe no longer matches the recorded patch and how to
// resolve it.
//
// Held as a sidecar (not a field on ApplyRecipe) so the skill-parity
// guard's DisallowUnknownFields check stays passing without touching
// the 6 skill assets.
type RecipeStaleness struct {
	Stale      bool   `json:"stale"`
	Reason     string `json:"reason"`
	DetectedAt string `json:"detected_at"`
}

// RecipeFromPatch derives a minimal ApplyRecipe from a captured unified
// diff by emitting a `write-file` op for each non-deleted file using
// the post-image content read from the working tree at repoRoot.
//
// The touched-file set comes from the strict normalized effect grammar
// through `patchEffectViews` (PI-1). A patch the grammar refuses returns
// its error rather than a silently short operation list: a derivation
// that cannot read the patch has nothing to derive. That includes a patch
// naming one destination path twice — the authority refuses it, so this
// derivation no longer keeps a first-seen `seen` map that would have
// dropped the second record's effect without telling anybody.
//
// Deleted files are returned in the `skipped` slice with a reason
// message: the current recipe schema has no delete-file op (a known
// gap surfaced to the user as a warning, not silently extended).
//
// The recipe is intended for replay/inspection — `artifacts/post-apply.patch`
// remains the reconcile source of truth.
func RecipeFromPatch(repoRoot, slug, patch string) (ApplyRecipe, []string, error) {
	files, err := patchEffectViews(patch)
	if err != nil {
		return ApplyRecipe{}, nil, err
	}
	// Determinism: alphabetical by path so two captures of the same
	// patch produce byte-identical recipes.
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	recipe := ApplyRecipe{Feature: slug}
	var skipped []string
	for _, fc := range files {
		if fc.Path == "" {
			continue
		}
		if fc.ChangeKind == gitutil.ChangeKindDelete {
			skipped = append(skipped, fmt.Sprintf("%s (deleted — recipe schema has no delete-file op)", fc.Path))
			continue
		}
		target := filepath.Join(repoRoot, fc.Path)
		if err := safety.EnsureSafeRepoPath(repoRoot, target); err != nil {
			skipped = append(skipped, fmt.Sprintf("%s (path safety: %v)", fc.Path, err))
			continue
		}
		data, err := os.ReadFile(target)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s (read: %v)", fc.Path, err))
			continue
		}
		recipe.Operations = append(recipe.Operations, RecipeOperation{
			Type:    "write-file",
			Path:    fc.Path,
			Content: string(data),
		})
	}
	return recipe, skipped, nil
}

// AutogenAction enumerates the outcomes of AutogenRecipeForRecord.
type AutogenAction string

const (
	AutogenGenerated   AutogenAction = "generated"   // no recipe → wrote a derived one
	AutogenRegenerated AutogenAction = "regenerated" // existing recipe overwritten (regenerate=true)
	AutogenStale       AutogenAction = "stale"       // existing recipe drifted → sidecar written
	AutogenNoop        AutogenAction = "noop"        // existing recipe matches captured patch
	AutogenSkipped     AutogenAction = "skipped"     // autogen disabled and no recipe present
)

// AutogenOutcome is what AutogenRecipeForRecord decided, together with
// the operation count of the recipe it actually derived.
//
// Operations exists because a `diff --git` prefix counter cannot compute
// one: it counts file records, so it double-counts a path with two
// records and counts a line that never produced an operation
// (PRD §6.1.3, PI-10). The producer that derived the recipe is the only
// caller holding a real operation count, so it reports it here.
type AutogenOutcome struct {
	Action       AutogenAction
	SkippedPaths []string
	DriftReason  string
	Operations   int
}

// AutogenRecipeForRecord materialises or stale-marks apply-recipe.json
// after `tpatch record` captures a patch.
//
// Behaviour matrix:
//
//	no recipe + autogen=true  → write derived recipe → AutogenGenerated
//	no recipe + autogen=false → leave alone          → AutogenSkipped
//	recipe exists, no drift   → clear stale sidecar   → AutogenNoop
//	recipe exists, drifted, regenerate=true  → overwrite recipe → AutogenRegenerated
//	recipe exists, drifted, regenerate=false → write recipe-stale.json sidecar → AutogenStale
//
// Drift is currently file-set based: if the existing recipe references
// a file the patch does not, or vice-versa, the recipe is stale. The
// sidecar approach is deliberate — a richer provider-generated recipe
// (with replace-in-file ops, search/replace context, created_by edges)
// is preserved; the sidecar tells the operator to regenerate when they
// are ready, without silent data loss.
func AutogenRecipeForRecord(s *store.Store, slug, patch string, autogen, regenerate bool) (AutogenOutcome, error) {
	derived, skipped, err := RecipeFromPatch(s.Root, slug, patch)
	if err != nil {
		return AutogenOutcome{SkippedPaths: skipped}, err
	}
	outcome := AutogenOutcome{SkippedPaths: skipped, Operations: len(derived.Operations)}

	existing, recipeErr := s.ReadFeatureFile(slug, filepath.Join("artifacts", "apply-recipe.json"))
	haveExisting := recipeErr == nil && strings.TrimSpace(existing) != ""

	if !haveExisting {
		if !autogen {
			outcome.Action = AutogenSkipped
			return outcome, nil
		}
		if err := writeRecipe(s, slug, derived); err != nil {
			return outcome, err
		}
		outcome.Action = AutogenGenerated
		return outcome, nil
	}

	var existingRecipe ApplyRecipe
	if jerr := json.Unmarshal([]byte(existing), &existingRecipe); jerr != nil {
		return resolveStale(s, slug, derived, "existing apply-recipe.json is unparseable JSON", regenerate, outcome)
	}
	drift, reason := compareRecipeFileSets(existingRecipe, derived)
	if !drift {
		_ = clearStaleMarker(s, slug)
		outcome.Action = AutogenNoop
		return outcome, nil
	}
	return resolveStale(s, slug, derived, reason, regenerate, outcome)
}

func resolveStale(s *store.Store, slug string, derived ApplyRecipe, reason string, regenerate bool, outcome AutogenOutcome) (AutogenOutcome, error) {
	outcome.DriftReason = reason
	if regenerate {
		if err := writeRecipe(s, slug, derived); err != nil {
			return outcome, err
		}
		_ = clearStaleMarker(s, slug)
		outcome.Action = AutogenRegenerated
		return outcome, nil
	}
	sb := RecipeStaleness{
		Stale:      true,
		Reason:     reason,
		DetectedAt: time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.MarshalIndent(sb, "", "  ")
	if err := s.WriteArtifact(slug, "recipe-stale.json", string(data)+"\n"); err != nil {
		return outcome, err
	}
	outcome.Action = AutogenStale
	return outcome, nil
}

func writeRecipe(s *store.Store, slug string, recipe ApplyRecipe) error {
	data, _ := json.MarshalIndent(recipe, "", "  ")
	return s.WriteArtifact(slug, "apply-recipe.json", string(data)+"\n")
}

// compareRecipeFileSets reports drift between two recipes by
// comparing the set of file paths each touches. ensure-directory ops
// are excluded because they describe directories, not files. Returns
// (false, "") when the two recipes target the same file set.
func compareRecipeFileSets(existing, derived ApplyRecipe) (bool, string) {
	e := recipePathSet(existing)
	d := recipePathSet(derived)
	var missing []string
	for p := range d {
		if !e[p] {
			missing = append(missing, p)
		}
	}
	var extra []string
	for p := range e {
		if !d[p] {
			extra = append(extra, p)
		}
	}
	if len(missing) == 0 && len(extra) == 0 {
		return false, ""
	}
	sort.Strings(missing)
	sort.Strings(extra)
	var parts []string
	if len(missing) > 0 {
		parts = append(parts, "patch touches files absent from recipe: "+strings.Join(missing, ", "))
	}
	if len(extra) > 0 {
		parts = append(parts, "recipe references files absent from patch: "+strings.Join(extra, ", "))
	}
	return true, strings.Join(parts, "; ")
}

func recipePathSet(r ApplyRecipe) map[string]bool {
	m := map[string]bool{}
	for _, op := range r.Operations {
		if op.Type == "ensure-directory" {
			continue
		}
		m[op.Path] = true
	}
	return m
}

func clearStaleMarker(s *store.Store, slug string) error {
	// Path layout is fixed by the store: .tpatch/features/<slug>/artifacts/.
	// Same convention used by recipe-provenance.json reads in cobra.go.
	stalePath := filepath.Join(s.Root, ".tpatch", "features", slug, "artifacts", "recipe-stale.json")
	if _, err := os.Stat(stalePath); err != nil {
		return nil
	}
	return os.Remove(stalePath)
}
