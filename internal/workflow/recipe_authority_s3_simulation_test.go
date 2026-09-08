package workflow

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/patchobs"
)

func TestRGAS3SimulationAssignmentsAndCompleteSet(t *testing.T) {
	patch := rgaS3ModifyPatch + strings.ReplaceAll(rgaS3AddPatch, "a.txt", "z.txt")
	obs := rgaS3Observe(t, patch, rgaS3Image{pre: "old\n", post: "new\n"}, rgaS3Image{post: "new\n"})
	ops := []RecipeOperation{rgaS3Write("z.txt", "", "new\n", true), rgaS3Write("a.txt", "old\n", "new\n", false)}
	in := rgaS3Inputs(t, obs, ops...)
	c := rgaS3Build(t, in)
	if !slices.Equal(c.Effects[0].OperationIndexes, []int{2}) || !slices.Equal(c.Effects[1].OperationIndexes, []int{1}) ||
		c.Effects[0].Ordinal != 1 || c.Effects[1].Ordinal != 2 {
		t.Fatalf("recipe order replaced normalized patch ordinals: %+v", c.Effects)
	}
	simulation, err := SimulateRecipeCoverage(obs, ApplyRecipe{Feature: "s3", Operations: ops})
	if err != nil || !simulation.ExactPostimage || !simulation.AllAlreadyPresent || len(simulation.MismatchPaths) != 0 {
		t.Fatalf("exact simulation: %+v %v", simulation, err)
	}
	for _, kind := range []string{"omitted", "surplus", "same-path-wrong-bytes", "unsafe", "duplicate", "alias-duplicate", "directory"} {
		t.Run(kind, func(t *testing.T) {
			wrong := slices.Clone(ops)
			switch kind {
			case "omitted":
				wrong = wrong[:1]
			case "surplus":
				wrong = append(wrong, rgaS3Write("extra.txt", "", "extra\n", true))
			case "same-path-wrong-bytes":
				wrong[1].Content = "wrong\n"
			case "unsafe":
				wrong = append(wrong, rgaS3Write("../outside", "", "x", true))
			case "duplicate":
				wrong = append(wrong, wrong[0])
			case "alias-duplicate":
				op := wrong[0]
				op.Path = "./z.txt"
				wrong = append(wrong, op)
			case "directory":
				wrong = append(wrong, RecipeOperation{Type: "ensure-directory", Path: "empty-dir"})
			}
			input := rgaS3Inputs(t, obs, wrong...)
			got, err := BuildRecipeCoverage(input)
			if kind == "duplicate" || kind == "alias-duplicate" {
				if err == nil {
					t.Fatal("duplicate target assignment accepted")
				}
				if _, err := SimulateRecipeCoverage(obs, ApplyRecipe{Feature: "s3", Operations: wrong}); err == nil {
					t.Fatal("simulator accepted duplicate paths")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.CoverageStatus != CoverageIncomplete || !slices.Contains(got.Reasons, "simulation-mismatch") {
				t.Fatalf("full-set comparison missed %s: %+v", kind, got)
			}
			if kind == "omitted" && !slices.Equal(got.Effects[0].ReasonCodes, []string{"operation-missing"}) {
				t.Fatalf("omitted representable effect: %+v", got.Effects[0])
			}
			if (kind == "surplus" || kind == "unsafe" || kind == "directory") && !slices.Contains(got.Reasons, "operation-surplus") {
				t.Fatal("unassigned operation was not surplus")
			}
			forged := rgaS3Clone(c)
			forged.RecipeSHA256 = CoverageSHA256(input.Recipe.Bytes)
			if err := ValidateRecipeCoverage(forged, input); err == nil {
				t.Fatal("same input-aware validator accepted file-set-only proof")
			}
		})
	}
	for _, mutation := range []func(*RecipeCoverage){
		func(c *RecipeCoverage) { c.Effects = c.Effects[:1] },
		func(c *RecipeCoverage) { c.Effects = append(c.Effects, c.Effects[0]) },
		func(c *RecipeCoverage) { c.Effects[0], c.Effects[1] = c.Effects[1], c.Effects[0] },
		func(c *RecipeCoverage) { c.Effects[0].OperationIndexes = []int{1} },
		func(c *RecipeCoverage) { c.Effects[0].OperationIndexes = []int{3} },
		func(c *RecipeCoverage) { c.Effects[0].OperationIndexes = []int{} },
	} {
		wrong := rgaS3Clone(c)
		mutation(&wrong)
		if err := ValidateRecipeCoverage(wrong, in); err == nil {
			t.Fatal("effect/assignment omission or invention accepted")
		}
	}
}

func TestRGAS3ModeExistenceAndPreconditionDomain(t *testing.T) {
	for _, tc := range []struct {
		name, patch string
		image       rgaS3Image
		op          RecipeOperation
		exact       bool
	}{
		{"regular-create", rgaS3AddPatch, rgaS3Image{post: "new\n"}, rgaS3Write("a.txt", "", "new\n", true), true},
		{"regular-modify", rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"}, rgaS3Write("a.txt", "old\n", "new\n", false), true},
		{"explicit-empty-collision", rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"}, rgaS3Write("a.txt", "", "new\n", true), false},
		{"wrong-preimage-hash", rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"}, rgaS3Write("a.txt", "wrong\n", "new\n", false), false},
		{"expected-file-is-absent", rgaS3AddPatch, rgaS3Image{post: "new\n"}, rgaS3Write("a.txt", "old\n", "new\n", false), false},
		{"mode-cannot-be-written", strings.Replace(rgaS3AddPatch, "100644", "100755", 1), rgaS3Image{post: "new\n", newMode: "100755"}, rgaS3Write("a.txt", "", "new\n", true), false},
		{"symlink-is-not-regular", strings.Replace(rgaS3AddPatch, "100644", "120000", 1), rgaS3Image{post: "new\n", newMode: "120000"}, rgaS3Write("a.txt", "", "new\n", true), false},
		{"delete-is-not-write", rgaS3DeletePatch, rgaS3Image{pre: "old\n"}, rgaS3Write("a.txt", "old\n", "", false), false},
		{"rename-loses-source", rgaS3RenamePatch, rgaS3Image{pre: "old\n", post: "old\n"}, rgaS3Write("a.txt", "old\n", "old\n", false), false},
		{"copy-is-not-write", rgaS3CopyPatch, rgaS3Image{pre: "old\n", post: "old\n"}, rgaS3Write("a.txt", "old\n", "old\n", false), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obs := rgaS3Observe(t, tc.patch, tc.image)
			in := rgaS3Inputs(t, obs, tc.op)
			c := rgaS3Build(t, in)
			result, err := SimulateRecipeCoverage(obs, ApplyRecipe{Feature: "s3", Operations: []RecipeOperation{tc.op}})
			if err != nil || result.ExactPostimage != tc.exact || (c.CoverageStatus == CoverageComplete) != tc.exact {
				t.Fatalf("mode/existence simulation = %+v, %s, %v", result, c.CoverageStatus, err)
			}
			if tc.exact && !result.AllAlreadyPresent {
				t.Fatal("exact simulated state did not reclassify without writes")
			}
			if !tc.exact && result.AllAlreadyPresent {
				t.Fatal("failed precondition or unsupported mode produced an already-present proof")
			}
			if !tc.exact && len(result.MismatchPaths) == 0 {
				t.Fatal("mismatch diagnostic omitted affected paths")
			}
		})
	}
}

func TestRGAS3PreservedOperationsReclassification(t *testing.T) {
	for _, tc := range []struct {
		name, pre, post string
		op              RecipeOperation
		exact, reclass  bool
	}{
		{"append", "old\n", "old\nnew\n", RecipeOperation{Type: "append-file", Path: "a.txt", Content: "new\n"}, true, false},
		{"empty-append", "old\n", "old\n", RecipeOperation{Type: "append-file", Path: "a.txt", Content: ""}, true, false},
		{"append-wrong-result", "old\n", "new\n", RecipeOperation{Type: "append-file", Path: "a.txt", Content: "new\n"}, false, false},
		{"replace-first-only", "old old\n", "new old\n", RecipeOperation{Type: "replace-in-file", Path: "a.txt", Search: "old", Replace: "new"}, true, false},
		{"replace-does-not-replace-all", "old old\n", "new new\n", RecipeOperation{Type: "replace-in-file", Path: "a.txt", Search: "old", Replace: "new"}, false, false},
		{"replace-empty-search", "old\n", "newold\n", RecipeOperation{Type: "replace-in-file", Path: "a.txt", Search: "", Replace: "new"}, true, false},
		{"replace-exact-postimage-exception", "old\n", "new\n", RecipeOperation{Type: "replace-in-file", Path: "a.txt", Search: "old", Replace: "new"}, true, true},
		{"replace-missing-search", "old\n", "new\n", RecipeOperation{Type: "replace-in-file", Path: "a.txt", Search: "absent", Replace: "new"}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obs := rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: tc.pre, post: tc.post})
			recipe := ApplyRecipe{Feature: "s3", Operations: []RecipeOperation{tc.op}}
			raw, err := EncodeRecipe(recipe)
			if err != nil {
				t.Fatal(err)
			}
			before := bytes.Clone(raw)
			result, err := SimulateRecipeCoverage(obs, recipe)
			if err != nil || result.ExactPostimage != tc.exact || result.AllAlreadyPresent != tc.reclass {
				t.Fatalf("preserved semantics/reclassification = %+v %v", result, err)
			}
			if !tc.reclass && !slices.Equal(result.UnreclassifiableOperations, []int{1}) {
				t.Fatal("unreclassifiable operation missing")
			}
			if tc.reclass {
				// The explicit exception is implemented, but the accepted
				// contract does not assign this complete shape a cross-base
				// branch. No invented scope or reason may hide that conflict.
				_, err := BuildRecipeCoverage(RecipeCoverageInput{Observation: obs, Recipe: CoverageArtifact{Present: true, Bytes: raw}})
				if err == nil || !strings.Contains(err.Error(), "D3 cross-base branch") {
					t.Fatalf("undefined cross-base scope invented: %v", err)
				}
			} else {
				c := rgaS3Build(t, RecipeCoverageInput{Observation: obs, Recipe: CoverageArtifact{Present: true, Bytes: raw}})
				if c.CoverageStatus != CoverageIncomplete || !slices.Equal(c.Effects[0].ReasonCodes, []string{"operation-not-reclassifiable"}) {
					t.Fatalf("preserved op misclassified: %+v", c)
				}
			}
			after, err := EncodeRecipe(recipe)
			if err != nil || !bytes.Equal(before, after) || !bytes.Equal(raw, before) {
				t.Fatal("simulation mutated preserved recipe")
			}
		})
	}
}

func TestRGAS3ReclassificationIsNotSecondExecution(t *testing.T) {
	obs := rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"})
	recipe := ApplyRecipe{Feature: "s3", Operations: []RecipeOperation{rgaS3Write("a.txt", "old\n", "new\n", false)}}
	before, _ := json.Marshal(obs)
	result, err := SimulateRecipeCoverage(obs, recipe)
	after, _ := json.Marshal(obs)
	if err != nil || !result.AllAlreadyPresent || !result.ExactPostimage || !bytes.Equal(before, after) {
		t.Fatalf("simulation mutated its state: %+v %v", result, err)
	}
	// A second real append would change the result. The pure classifier
	// therefore cannot report it already-present even with exact output.
	appendObs := rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "old\nnew\n"})
	appendRecipe := ApplyRecipe{Feature: "s3", Operations: []RecipeOperation{{Type: "append-file", Path: "a.txt", Content: "new\n"}}}
	appended, err := SimulateRecipeCoverage(appendObs, appendRecipe)
	if err != nil || !appended.ExactPostimage || appended.AllAlreadyPresent {
		t.Fatalf("append recognized by suffix alone: %+v %v", appended, err)
	}
}

func TestRGAS3ContextHintsNeverAuthorizeAndCapturesAreCopied(t *testing.T) {
	obs := rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "old\nextra\n"})
	obs.Capture.Pathspecs = []string{"z", "a", "a"}
	obs.Capture.ClaimIDs = []string{"claim-z", "claim-a"}
	in := rgaS3Inputs(t, obs, rgaS3Write("a.txt", "old\n", "old\nextra\n", false))
	c := rgaS3Build(t, in)
	if c.Effects[0].ContextualHint != "additive-text" || c.CrossBaseStatus != CrossBaseConsumerDerivationRequired {
		t.Fatalf("additive hint: %+v", c)
	}
	bad := rgaS3Clone(c)
	bad.Effects[0].ContextualHint = "none"
	if err := ValidateRecipeCoverageSchema(bad); err != nil {
		t.Fatalf("advice changed structural authority: %v", err)
	}
	bad.Producer = patchobs.ProducerImplement
	if err := ValidateRecipeCoverage(bad, in); err != nil {
		t.Fatalf("advisory producer/hint became input-aware authority: %v", err)
	}
	if bad.CoverageStatus != c.CoverageStatus || bad.CrossBaseStatus != c.CrossBaseStatus || bad.Effects[0].Disposition != c.Effects[0].Disposition {
		t.Fatal("context hint changed authoritative fields")
	}
	c.Capture.Pathspecs[0] = "changed"
	c.Capture.ClaimIDs[0] = "changed"
	if !reflect.DeepEqual(obs.Capture.Pathspecs, []string{"z", "a", "a"}) || !reflect.DeepEqual(obs.Capture.ClaimIDs, []string{"claim-z", "claim-a"}) {
		t.Fatal("builder sorted or aliased caller selectors")
	}
	c.Effects[0].OperationIndexes[0] = 99
	if rgaS3Build(t, in).Effects[0].OperationIndexes[0] != 1 {
		t.Fatal("returned assignments share mutable internal state")
	}
	unobserved := obs
	unobserved.Effects = slices.Clone(obs.Effects)
	unobserved.Effects[0] = patchobs.EffectObservation{Effect: obs.Effects[0].Effect}
	e := &unobserved.Effects[0].Effect
	e.PreimageObserved, e.PostimageObserved, e.PreimagePresent, e.PostimagePresent = false, false, false, false
	e.PreimageSHA256, e.PostimageSHA256, e.OldMode, e.NewMode = "", "", "", ""
	e.ContentKind, e.ObjectKind = "unknown", "unknown"
	unobserved.Reference.PreimageSetSHA256 = patchobs.PreimageSetDigest(unobserved.Effects)
	u := rgaS3Build(t, rgaS3Inputs(t, unobserved))
	if u.Effects[0].ContextualHint != "none" || u.CrossBaseStatus != CrossBaseUnsupported {
		t.Fatal("unobserved source gained contextual authority")
	}
}

func TestRGAS3LiteralFragmentsAndTypechangeOrdinals(t *testing.T) {
	embedded := "diff --git a/a.txt b/a.txt\nnew file mode 100644\n--- /dev/null\n+++ b/a.txt\n@@ -0,0 +1 @@\n+diff --git a/not-a-record b/not-a-record\n"
	last := strings.ReplaceAll(rgaS3AddPatch, "a.txt", "last.txt")
	obs := rgaS3Observe(t, embedded+last, rgaS3Image{post: "diff --git a/not-a-record b/not-a-record\n"}, rgaS3Image{post: "new\n"})
	c := rgaS3Build(t, rgaS3Inputs(t, obs, rgaS3Write("a.txt", "", "diff --git a/not-a-record b/not-a-record\n", true), rgaS3Write("last.txt", "", "new\n", true)))
	if len(c.Effects) != 2 || c.Effects[0].PatchFragmentSHA256 != CoverageSHA256([]byte(embedded)) || c.Effects[1].PatchFragmentSHA256 != CoverageSHA256([]byte(last)) {
		t.Fatal("fragment boundaries were a substring scan")
	}
	crlf := "diff --git a/a.txt b/a.txt\nnew file mode 100644\n--- /dev/null\n+++ b/a.txt\n@@ -0,0 +1 @@\n+new\r\n"
	crlfObs := rgaS3Observe(t, crlf, rgaS3Image{post: "new\r\n"})
	crlfCoverage := rgaS3Build(t, rgaS3Inputs(t, crlfObs, rgaS3Write("a.txt", "", "new\r\n", true)))
	wrong := rgaS3Clone(crlfCoverage)
	wrong.Effects[0].PatchFragmentSHA256 = CoverageSHA256([]byte(strings.ReplaceAll(crlf, "\r\n", "\n")))
	rgaS3Rehash(t, &wrong.Effects[0])
	if err := ValidateRecipeCoverage(wrong, rgaS3Inputs(t, crlfObs, rgaS3Write("a.txt", "", "new\r\n", true))); err == nil {
		t.Fatal("normalized fragment was accepted")
	}
	typechange := rgaS3DeletePatch + strings.Replace(rgaS3AddPatch, "100644", "120000", 1)
	tcObs := rgaS3Observe(t, typechange+last, rgaS3Image{pre: "old\n", post: "new\n", newMode: "120000"}, rgaS3Image{post: "new\n"})
	tc := rgaS3Build(t, rgaS3Inputs(t, tcObs))
	if len(tc.Effects) != 2 || tc.Effects[0].ChangeKind != "modify" || tc.Effects[0].Ordinal != 1 || tc.Effects[1].Ordinal != 2 ||
		tc.Effects[0].PatchFragmentSHA256 != CoverageSHA256([]byte(typechange)) {
		t.Fatalf("typechange pair failed canonical normalized order: %+v", tc.Effects)
	}
}
