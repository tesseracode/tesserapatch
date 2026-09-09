package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

func rgaS4CLICoverage(t *testing.T, root, slug string) workflow.RecipeCoverage {
	t.Helper()
	path := filepath.Join(root, ".tpatch", "features", slug, "artifacts")
	raw, err := os.ReadFile(filepath.Join(path, "recipe-coverage.json"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := workflow.DecodeRecipeCoverage(raw)
	if err != nil {
		t.Fatal(err)
	}
	patch, patchErr := os.ReadFile(filepath.Join(path, "post-apply.patch"))
	recipe, recipeErr := os.ReadFile(filepath.Join(path, "apply-recipe.json"))
	if c.PatchPresent != (patchErr == nil) || (c.PatchPresent && c.PatchSHA256 != workflow.CoverageSHA256(patch)) ||
		c.RecipePresent != (recipeErr == nil) || (c.RecipePresent && c.RecipeSHA256 != workflow.CoverageSHA256(recipe)) {
		t.Fatalf("coverage does not bind disk: %+v", c)
	}
	return c
}

func rgaS4CLIFixture(t *testing.T, slug string, dirty bool) string {
	t.Helper()
	root := modesFixture(t, slug)
	rgaS0CommitAll(t, root)
	if dirty {
		modesWriteFile(t, root, "a.txt", "new\n")
	}
	return root
}

func rgaS4FeatureBytes(t *testing.T, root, slug string, excludeCoverage bool) map[string]string {
	t.Helper()
	base := filepath.Join(root, ".tpatch", "features", slug)
	result := map[string]string{}
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		if excludeCoverage && filepath.ToSlash(rel) == "artifacts/recipe-coverage.json" {
			return nil
		}
		raw, err := os.ReadFile(path)
		result[filepath.ToSlash(rel)] = string(raw)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestRGAS4RecordNoopRepairsCoverageAndProvenance(t *testing.T) {
	slug := "s4-record"
	root := rgaS4CLIFixture(t, slug, true)
	args := []string{"record", "--path", root, slug, "--lenient"}
	if _, stderr, code := runRecord(t, args...); code != 0 {
		t.Fatal(stderr)
	}
	before := rgaS4CLICoverage(t, root, slug)
	if before.Producer != patchobs.ProducerRecord || before.CoverageStatus != workflow.CoverageComplete {
		t.Fatalf("P1 initial publication: %+v", before)
	}
	generation := loadPatchGenerationsForTest(t, root, slug)
	artifacts := filepath.Join(root, ".tpatch", "features", slug, "artifacts")
	for _, name := range []string{"recipe-provenance.json", "recipe-coverage.json"} {
		if err := os.Remove(filepath.Join(artifacts, name)); err != nil {
			t.Fatal(err)
		}
	}
	if _, stderr, code := runRecord(t, args...); code != 0 {
		t.Fatal(stderr)
	}
	after := rgaS4CLICoverage(t, root, slug)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("noop recovery changed immutable coverage: before=%+v after=%+v", before, after)
	}
	if got := loadPatchGenerationsForTest(t, root, slug); !reflect.DeepEqual(generation, got) {
		t.Fatal("noop repair appended a duplicate generation")
	}
	if _, err := os.ReadFile(filepath.Join(artifacts, "recipe-provenance.json")); err != nil {
		t.Fatalf("D6 noop did not repair provenance: %v", err)
	}
}

func TestRGAS4P2CheckpointCoverageOnlyAndEmptyCaptureNoEvent(t *testing.T) {
	for _, verb := range []string{"refresh", "fixup"} {
		t.Run(verb, func(t *testing.T) {
			slug := "s4-checkpoint-" + verb
			root := rgaS4CLIFixture(t, slug, true)
			if _, stderr, code := runRecord(t, "record", "--path", root, slug, "--lenient"); code != 0 {
				t.Fatal(stderr)
			}
			path := filepath.Join(root, ".tpatch", "features", slug, "artifacts", "recipe-coverage.json")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			before := rgaS4FeatureBytes(t, root, slug, true)
			args := []string{"feature", "patch", verb, "--path", root, slug, "--reason", "checkpoint"}
			stdout, stderr, code := runCmdWithError(args...)
			if code != 0 || !strings.Contains(stderr, "no patch byte change; "+verb+" skipped") ||
				!strings.Contains(stderr, "recipe coverage: complete\n") {
				t.Fatalf("checkpoint output: code=%d out=%s err=%s", code, stdout, stderr)
			}
			if after := rgaS4FeatureBytes(t, root, slug, true); !reflect.DeepEqual(before, after) {
				t.Fatal("P2 checkpoint changed artifacts other than coverage")
			}
			c := rgaS4CLICoverage(t, root, slug)
			rgaS4AssertReportedCoverageStatus(t, stderr, c)
			if c.Producer != patchobs.ProducerFeaturePatch || slices.Contains(c.Reasons, "producer-patch-rewrite") ||
				slices.Contains(c.Reasons, "recipe-not-regenerated") {
				t.Fatalf("checkpoint borrowed rewrite reasons: %+v", c)
			}
			rgaS0CommitAll(t, root)
			untouched := rgaS4FeatureBytes(t, root, slug, false)
			_, stderr, code = runCmdWithError(args...)
			if verb == "refresh" && code != 0 {
				t.Fatal(stderr)
			}
			if verb == "fixup" && code == 0 {
				t.Fatal("empty fixup must refuse")
			}
			if after := rgaS4FeatureBytes(t, root, slug, false); !reflect.DeepEqual(untouched, after) {
				t.Fatal("P2 empty capture invented an event")
			}
		})
	}
}

func TestRGAS4P2WritingPreservesDriftedRecipeAndPairedReasons(t *testing.T) {
	slug := "s4-amend"
	root := rgaS4CLIFixture(t, slug, true)
	if _, stderr, code := runRecord(t, "record", "--path", root, slug, "--lenient"); code != 0 {
		t.Fatal(stderr)
	}
	recipePath := filepath.Join(root, ".tpatch", "features", slug, "artifacts", "apply-recipe.json")
	before, err := os.ReadFile(recipePath)
	if err != nil {
		t.Fatal(err)
	}
	modesWriteFile(t, root, "a.txt", "amended\n")
	_, stderr, code := runCmdWithError("feature", "patch", "refresh", "--path", root, slug)
	if code != 0 {
		t.Fatal(stderr)
	}
	c := rgaS4CLICoverage(t, root, slug)
	rgaS4AssertReportedCoverageStatus(t, stderr, c)
	for _, reason := range []string{"producer-patch-rewrite", "recipe-not-regenerated", "recipe-stale-marker-present"} {
		if !slices.Contains(c.Reasons, reason) {
			t.Fatalf("P2 omitted applicable reason %s: %+v", reason, c)
		}
	}
	after, err := os.ReadFile(recipePath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("non-regenerating amendment rewrote the recipe")
	}
}

type rgaS4CLIRecorder func(patchobs.Observation)

func (f rgaS4CLIRecorder) Record(obs patchobs.Observation) { f(obs) }

func TestRGAS4CLIProducerPublicationFailuresAreNonzero(t *testing.T) {
	for _, event := range []string{"P1", "P1-noop", "P2-write", "P2-fixup", "P2-checkpoint", "P3", "P4", "P5", "P6", "P6-manual", "P7"} {
		t.Run(event, func(t *testing.T) {
			slug := "s4-failure"
			root := rgaS4CLIFixture(t, slug, true)
			artifacts := filepath.Join(root, ".tpatch", "features", slug, "artifacts")
			args := []string{"record", "--path", root, slug, "--lenient"}
			var invoke func() (string, string, int)
			switch event {
			case "P1-noop":
				if _, stderr, code := runRecord(t, args...); code != 0 {
					t.Fatal(stderr)
				}
			case "P2-write", "P2-fixup", "P2-checkpoint":
				if _, stderr, code := runRecord(t, args...); code != 0 {
					t.Fatal(stderr)
				}
				if event != "P2-checkpoint" {
					modesWriteFile(t, root, "a.txt", "changed\n")
				}
				args = []string{"feature", "patch", "refresh", "--path", root, slug}
				if event == "P2-fixup" {
					args = []string{"feature", "patch", "fixup", "--path", root, slug, "--reason", "failure fixture"}
				}
			case "P3":
				s, err := store.Open(root)
				if err != nil {
					t.Fatal(err)
				}
				head, err := gitutil.HeadCommit(root)
				if err != nil {
					t.Fatal(err)
				}
				patch, err := gitutil.CapturePatch(root)
				if err != nil {
					t.Fatal(err)
				}
				invoke = func() (string, string, int) {
					if err := workflow.RefreshAfterAccept(s, slug, head, patch); err != nil {
						return "", err.Error(), 1
					}
					return "", "", 0
				}
			case "P4":
				args = []string{"cycle", "--path", root, slug}
			case "P5":
				args = []string{"apply", "--mode", "done", "--path", root, slug}
			case "P6":
				args = []string{"implement", "--path", root, slug}
			case "P6-manual":
				args = []string{"implement", "--manual", "--path", root, slug}
			case "P7":
				args = []string{"edit", "--path", root, slug, "artifacts/apply-recipe.json"}
				rgaS4SetEditor(t, "write", "changed raw recipe\n", false)
			}
			if event == "P6-manual" || event == "P7" {
				body := fmt.Sprintf("{\"feature\":%q,\"operations\":[]}\n", slug)
				if err := os.WriteFile(filepath.Join(artifacts, "apply-recipe.json"), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			blockCoverage := func() {
				path := filepath.Join(artifacts, "recipe-coverage.json")
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if event == "P4" {
				restore := patchobs.SetRecorder(rgaS4CLIRecorder(func(obs patchobs.Observation) {
					if obs.Producer == patchobs.ProducerCycle {
						c := rgaS4CLICoverage(t, root, slug)
						if c.Producer != patchobs.ProducerImplement {
							t.Fatal("P6 must publish before P4's observation")
						}
						blockCoverage()
					}
				}))
				t.Cleanup(restore)
			} else {
				blockCoverage()
			}
			if invoke == nil {
				invoke = func() (string, string, int) { return runCmdWithError(args...) }
			}
			stdout, stderr, code := invoke()
			if code == 0 || !strings.Contains(stderr, "publish coverage") {
				t.Fatalf("%s hid publication failure: code=%d stdout=%s stderr=%s", event, code, stdout, stderr)
			}
			for _, success := range []string{"Recorded patch for", "Amended patch for", "marked as applied", "Phase implement advanced manually", "is now in state: applied"} {
				if strings.Contains(stdout, success) {
					t.Fatalf("%s reported success over failed publication: %s", event, stdout)
				}
			}
			if event == "P1" || event == "P1-noop" || event == "P2-write" || event == "P2-fixup" {
				generation := loadPatchGenerationsForTest(t, root, slug)
				if len(generation.Generations) == 0 {
					t.Fatal("coverage failure happened before generation publication")
				}
				for _, name := range []string{"apply-recipe.json", "recipe-provenance.json"} {
					if _, err := os.ReadFile(filepath.Join(artifacts, name)); err != nil {
						t.Fatalf("coverage was not last after %s: %v", name, err)
					}
				}
			}
		})
	}
}

func TestRGAS4P2CompactRecipeCheckpointChangesCoverageOnly(t *testing.T) {
	for _, verb := range []string{"refresh", "fixup"} {
		for _, keepProvenance := range []bool{false, true} {
			name := verb + "/" + map[bool]string{false: "no-provenance", true: "preserved-provenance"}[keepProvenance]
			t.Run(name, func(t *testing.T) {
				slug := "s4-compact-checkpoint"
				root := rgaS4CLIFixture(t, slug, true)
				if _, stderr, code := runRecord(t, "record", "--path", root, slug, "--lenient"); code != 0 {
					t.Fatal(stderr)
				}
				if c := rgaS4CLICoverage(t, root, slug); c.CoverageStatus != workflow.CoverageComplete {
					t.Fatalf("fixture did not start from complete gated coverage: %+v", c)
				}
				artifacts := filepath.Join(root, ".tpatch", "features", slug, "artifacts")
				recipePath := filepath.Join(artifacts, "apply-recipe.json")
				canonical, err := os.ReadFile(recipePath)
				if err != nil {
					t.Fatal(err)
				}
				var recipe workflow.ApplyRecipe
				if err := json.Unmarshal(canonical, &recipe); err != nil || len(recipe.Operations) == 0 {
					t.Fatalf("fixture recipe: %+v %v", recipe, err)
				}
				for _, op := range recipe.Operations {
					if op.Type != "write-file" || op.PreimageHash == nil {
						t.Fatalf("fixture widened ADR-039's admissible operation domain: %+v", op)
					}
				}
				var compact bytes.Buffer
				if err := json.Compact(&compact, canonical); err != nil {
					t.Fatal(err)
				}
				if bytes.Equal(compact.Bytes(), canonical) {
					t.Fatal("fixture must actually change canonical formatting")
				}
				if err := os.WriteFile(recipePath, compact.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
				provenancePath := filepath.Join(artifacts, "recipe-provenance.json")
				if !keepProvenance {
					if err := os.Remove(provenancePath); err != nil {
						t.Fatal(err)
					}
				}
				before := rgaS4FeatureBytes(t, root, slug, true)
				stdout, stderr, code := runCmdWithError("feature", "patch", verb, "--path", root, slug, "--reason", "semantic checkpoint")
				if code != 0 || !strings.Contains(stderr, "no patch byte change; "+verb+" skipped") ||
					!strings.Contains(stderr, "recipe coverage: complete\n") {
					t.Fatalf("compact checkpoint failed: code=%d out=%q err=%q", code, stdout, stderr)
				}
				c := rgaS4CLICoverage(t, root, slug)
				rgaS4AssertReportedCoverageStatus(t, stderr, c)
				if c.Producer != patchobs.ProducerFeaturePatch || c.CoverageStatus != workflow.CoverageComplete ||
					c.RecipeSHA256 != workflow.CoverageSHA256(compact.Bytes()) ||
					slices.Contains(c.Reasons, "producer-patch-rewrite") || slices.Contains(c.Reasons, "recipe-not-regenerated") {
					t.Fatalf("checkpoint imposed origin or borrowed rewrite reasons: %+v", c)
				}
				after := rgaS4FeatureBytes(t, root, slug, true)
				if !reflect.DeepEqual(before, after) {
					t.Fatal("compact checkpoint changed recipe, provenance, patch, generation, state or another artifact")
				}
				if !keepProvenance {
					if _, err := os.Stat(provenancePath); !os.IsNotExist(err) {
						t.Fatalf("checkpoint fabricated provenance without an origin claim: %v", err)
					}
				}
			})
		}
	}
}

func rgaS4AssertReportedCoverageStatus(t *testing.T, output string, coverage workflow.RecipeCoverage) {
	t.Helper()
	want := "recipe coverage: " + coverage.CoverageStatus
	if coverage.CoverageStatus == workflow.CoverageIncomplete {
		set := map[string]bool{}
		for _, reason := range coverage.Reasons {
			set[reason] = true
		}
		for _, effect := range coverage.Effects {
			for _, reason := range effect.ReasonCodes {
				set[reason] = true
			}
		}
		var reasons []string
		for reason := range set {
			reasons = append(reasons, reason)
		}
		sort.Strings(reasons)
		want += " (" + strings.Join(reasons, ", ") + ")"
	}
	matches := 0
	for _, line := range strings.Split(output, "\n") {
		if line == want {
			matches++
		}
	}
	if matches != 1 || strings.Count(output, "recipe coverage:") != 1 {
		t.Fatalf("expected one exact common coverage line %q, got %q", want, output)
	}
}

func TestRGAS4RecordReportsAllAutogenOutcomes(t *testing.T) {
	for _, outcome := range []string{"generated", "regenerated", "noop", "preserved", "skipped", "incomplete-regeneration"} {
		t.Run(outcome, func(t *testing.T) {
			slug := "s4-status"
			root := rgaS4CLIFixture(t, slug, true)
			args := []string{"record", "--path", root, slug, "--lenient"}
			if outcome == "regenerated" || outcome == "noop" || outcome == "incomplete-regeneration" {
				if _, stderr, code := runRecord(t, args...); code != 0 {
					t.Fatal(stderr)
				}
			}
			switch outcome {
			case "regenerated":
				modesWriteFile(t, root, "a.txt", "regenerated\n")
				args = append(args, "--regenerate-recipe")
			case "preserved":
				recipe := fmt.Sprintf(`{"feature":%q,"operations":[{"type":"write-file","path":"a.txt","content":"manual\n","preimage_hash":""}]}`, slug)
				if err := os.WriteFile(filepath.Join(root, ".tpatch", "features", slug, "artifacts", "apply-recipe.json"), []byte(recipe), 0o644); err != nil {
					t.Fatal(err)
				}
			case "skipped":
				args = append(args, "--no-recipe-autogen")
			case "incomplete-regeneration":
				if err := os.Remove(filepath.Join(root, "README.md")); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--regenerate-recipe")
			}
			stdout, stderr, code := runRecord(t, args...)
			if code != 0 {
				t.Fatalf("record %s: %s", outcome, stderr)
			}
			coverage := rgaS4CLICoverage(t, root, slug)
			rgaS4AssertReportedCoverageStatus(t, stderr, coverage)
			if strings.Contains(stdout, "recipe coverage:") {
				t.Fatal("coverage diagnostics must not pollute command stdout")
			}
			incomplete := outcome == "preserved" || outcome == "skipped" || outcome == "incomplete-regeneration"
			if (coverage.CoverageStatus == workflow.CoverageIncomplete) != incomplete {
				t.Fatalf("fixture did not reach %s: %+v", outcome, coverage)
			}
			if outcome == "skipped" && strings.Contains(stdout, "Recipe generated:") {
				t.Fatal("withheld recipe reported a generation instead of the common incomplete status")
			}
		})
	}
}

func TestRGAS4P2WritingReasonsFollowSemanticsNotFormatting(t *testing.T) {
	for _, verb := range []string{"refresh", "fixup"} {
		for _, changedSemantics := range []bool{false, true} {
			name := verb + "/" + map[bool]string{false: "formatting-only", true: "changed-semantics"}[changedSemantics]
			t.Run(name, func(t *testing.T) {
				slug := "s4-semantic-reasons"
				root := rgaS4CLIFixture(t, slug, false)
				preimage := "one\ntwo\nthree\nold\nfive\nsix\nseven\n"
				postimage := strings.Replace(preimage, "old\n", "new\n", 1)
				modesWriteFile(t, root, "context.txt", preimage)
				gitRun(t, root, "add", "context.txt")
				gitRun(t, root, "commit", "-qm", "context fixture base")
				gitRun(t, root, "config", "diff.context", "3")
				modesWriteFile(t, root, "context.txt", postimage)
				if _, stderr, code := runRecord(t, "record", "--path", root, slug, "--lenient"); code != 0 {
					t.Fatal(stderr)
				}
				if initial := rgaS4CLICoverage(t, root, slug); initial.CoverageStatus != workflow.CoverageComplete {
					t.Fatalf("fixture must start from a complete gated recipe: %+v", initial)
				}
				artifacts := filepath.Join(root, ".tpatch", "features", slug, "artifacts")
				recipePath := filepath.Join(artifacts, "apply-recipe.json")
				patchPath := filepath.Join(artifacts, "post-apply.patch")
				provenancePath := filepath.Join(artifacts, "recipe-provenance.json")
				canonical, err := os.ReadFile(recipePath)
				if err != nil {
					t.Fatal(err)
				}
				provenance, err := os.ReadFile(provenancePath)
				if err != nil {
					t.Fatal(err)
				}
				oldPatch, err := os.ReadFile(patchPath)
				if err != nil {
					t.Fatal(err)
				}
				var compact bytes.Buffer
				if err := json.Compact(&compact, canonical); err != nil || bytes.Equal(canonical, compact.Bytes()) {
					t.Fatalf("fixture must change the on-disk recipe formatting: %v", err)
				}
				if err := os.WriteFile(recipePath, compact.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}

				// A real Git-rendering change forces P2's writing branch
				// without changing either image in the formatting-only row.
				gitRun(t, root, "config", "diff.context", "0")
				if changedSemantics {
					postimage = strings.Replace(postimage, "new\n", "different\n", 1)
					modesWriteFile(t, root, "context.txt", postimage)
				}
				captured, err := gitutil.CapturePatchScoped(root, nil)
				if err != nil || captured == "" || captured == string(oldPatch) {
					t.Fatalf("fixture must capture different nonempty patch bytes: %q %v", captured, err)
				}
				beforeGenerations := loadPatchGenerationsForTest(t, root, slug)
				var observations []patchobs.Observation
				restore := patchobs.SetRecorder(rgaS4CLIRecorder(func(observation patchobs.Observation) {
					if observation.Producer != patchobs.ProducerFeaturePatch {
						t.Fatalf("unexpected producer: %s", observation.Producer)
					}
					beforeWrite, err := os.ReadFile(patchPath)
					if err != nil || !bytes.Equal(beforeWrite, oldPatch) {
						t.Fatal("P2 observation must precede its canonical patch write")
					}
					observations = append(observations, observation)
				}))
				t.Cleanup(restore)
				stdout, stderr, code := runCmdWithError("feature", "patch", verb, "--path", root, slug, "--reason", "semantic reason fixture")
				if code != 0 || !strings.Contains(stdout, "Amended patch for ") || strings.Contains(stderr, "no patch byte change") {
					t.Fatalf("fixture did not take P2's writing branch: code=%d out=%q err=%q", code, stdout, stderr)
				}
				if len(observations) != 1 || string(observations[0].PatchBytes) != captured {
					t.Fatalf("P2 did not bind its actual capture: %+v", observations)
				}
				derived, err := workflow.DeriveRecipe(observations[0])
				if err != nil || len(derived.CanonicalBytes()) == 0 || derived.ProvesOrigin(compact.Bytes()) {
					t.Fatalf("fixture must be a complete derivation without raw D16 origin: %v", err)
				}
				if bytes.Equal(derived.CanonicalBytes(), canonical) == changedSemantics {
					t.Fatal("fixture did not distinguish formatting from changed semantics")
				}
				writtenPatch, err := os.ReadFile(patchPath)
				if err != nil || string(writtenPatch) != captured {
					t.Fatalf("P2 did not publish the newly captured canonical patch: %v", err)
				}
				afterGenerations := loadPatchGenerationsForTest(t, root, slug)
				if len(afterGenerations.Generations) != len(beforeGenerations.Generations)+1 {
					t.Fatal("P2 writing event did not append its generation")
				}
				for path, before := range map[string][]byte{recipePath: compact.Bytes(), provenancePath: provenance} {
					after, err := os.ReadFile(path)
					if err != nil || !bytes.Equal(before, after) {
						t.Fatalf("non-D16 writing event rewrote %s: %v", filepath.Base(path), err)
					}
				}
				rawMarker, err := os.ReadFile(filepath.Join(artifacts, "recipe-stale.json"))
				if err != nil {
					t.Fatal(err)
				}
				var marker workflow.RecipeStaleness
				if err := json.Unmarshal(rawMarker, &marker); err != nil || !marker.Stale ||
					!strings.Contains(marker.Reason, "recipe bytes differ") || marker.DetectedAt == "" {
					t.Fatalf("missing truthful raw-byte stale marker: %+v %v", marker, err)
				}
				coverage := rgaS4CLICoverage(t, root, slug)
				wantReasons := []string{"recipe-stale-marker-present"}
				if changedSemantics {
					wantReasons = []string{"producer-patch-rewrite", "recipe-not-regenerated", "recipe-stale-marker-present", "simulation-mismatch"}
				}
				if coverage.Producer != patchobs.ProducerFeaturePatch ||
					coverage.CoverageStatus != workflow.CoverageIncomplete || !slices.Equal(coverage.Reasons, wantReasons) {
					t.Fatalf("rewrite reasons must follow semantic explanation, not the marker: got=%v want=%v", coverage.Reasons, wantReasons)
				}
				for _, effect := range coverage.Effects {
					if len(effect.ReasonCodes) != 0 {
						t.Fatalf("supported gated fixture gained inapplicable effect reasons: %+v", effect)
					}
				}
				rgaS4AssertReportedCoverageStatus(t, stderr, coverage)
			})
		}
	}
}
