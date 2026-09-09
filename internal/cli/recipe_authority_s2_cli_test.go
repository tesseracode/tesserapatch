package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

type rgaS2RecorderFunc func(patchobs.Observation)

func (f rgaS2RecorderFunc) Record(obs patchobs.Observation) { f(obs) }

func TestRGAS2RecordUsesPreWriteObservation(t *testing.T) {
	const slug = "s2-frozen-record"
	root := modesFixture(t, slug)
	rgaS0CommitAll(t, root)
	modesWriteFile(t, root, "new.txt", "captured\r\nno newline")

	captured := 0
	var frozen patchobs.Observation
	restore := patchobs.SetRecorder(rgaS2RecorderFunc(func(obs patchobs.Observation) {
		if obs.Producer != patchobs.ProducerRecord {
			return
		}
		captured++
		frozen = obs
		// Changing the worktree at the capture/publication seam makes a
		// post-write re-read observably wrong, not merely source-shaped.
		if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("late mutation"), 0o644); err != nil {
			t.Fatal(err)
		}
	}))
	defer restore()
	_, stderr, code := runRecord(t, "record", "--path", root, slug, "--lenient")
	if code != 0 || captured != 1 {
		t.Fatalf("record capture: code=%d count=%d stderr=%s", code, captured, stderr)
	}
	recipe := rgaS0ReadRecipe(t, root, slug)
	if len(recipe.Operations) != 1 {
		t.Fatalf("operations = %+v", recipe.Operations)
	}
	op := recipe.Operations[0]
	if op.Content != "captured\r\nno newline" || op.PreimageHash == nil || *op.PreimageHash != "" {
		t.Fatalf("producer did not consume the pre-write observation: %+v", op)
	}
	live, err := os.ReadFile(filepath.Join(root, "new.txt"))
	if err != nil || string(live) != "late mutation" {
		t.Fatalf("mutation fixture did not change the guarded input: %q / %v", live, err)
	}
	rawRecipe, err := os.ReadFile(filepath.Join(root, ".tpatch", "features", slug, "artifacts", "apply-recipe.json"))
	if err != nil {
		t.Fatal(err)
	}
	rawCoverage, err := os.ReadFile(filepath.Join(root, ".tpatch", "features", slug, "artifacts", "recipe-coverage.json"))
	if err != nil {
		t.Fatal(err)
	}
	coverage, err := workflow.DecodeRecipeCoverage(rawCoverage)
	if err != nil || coverage.CoverageStatus != workflow.CoverageComplete || coverage.Producer != patchobs.ProducerRecord {
		t.Fatalf("S4 did not publish the captured complete record: %+v / %v", coverage, err)
	}
	if err := workflow.ValidateRecipeCoverage(coverage, workflow.RecipeCoverageInput{
		Observation: frozen,
		Recipe:      workflow.CoverageArtifact{Present: true, Bytes: rawRecipe},
		Events:      workflow.CoverageEvents{PatchRewritten: true, RecipeRegenerated: true},
	}); err != nil {
		t.Fatalf("published coverage followed live bytes instead of the captured observation: %v", err)
	}
}
