package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
)

const rgaS3ModifyPatch = "diff --git a/a.txt b/a.txt\nindex 3367afd..3e75765 100644\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-old\n+new\n"
const rgaS3AddPatch = "diff --git a/a.txt b/a.txt\nnew file mode 100644\nindex 0000000..3e75765\n--- /dev/null\n+++ b/a.txt\n@@ -0,0 +1 @@\n+new\n"
const rgaS3DeletePatch = "diff --git a/a.txt b/a.txt\ndeleted file mode 100644\nindex 3367afd..0000000\n--- a/a.txt\n+++ /dev/null\n@@ -1 +0,0 @@\n-old\n"
const rgaS3RenamePatch = "diff --git a/old.txt b/a.txt\nsimilarity index 100%\nrename from old.txt\nrename to a.txt\n"
const rgaS3CopyPatch = "diff --git a/old.txt b/a.txt\nsimilarity index 100%\ncopy from old.txt\ncopy to a.txt\n"

type rgaS3Image struct {
	pre, post        string
	oldMode, newMode string
}

func rgaS3Root() string {
	if runtime.GOOS == "windows" {
		return `C:\rga-s3-nonexistent-root`
	}
	return "/rga-s3-nonexistent-root"
}

func rgaS3Observe(t *testing.T, patch string, images ...rgaS3Image) patchobs.Observation {
	t.Helper()
	obs := patchobs.Observation{
		Producer: patchobs.ProducerRecord, RepoRoot: rgaS3Root(), Slug: "s3",
		Capture:      patchobs.CaptureDescriptor{Mode: patchobs.CaptureModeWorkingTreeAll, Pathspecs: []string{}, ClaimIDs: []string{}},
		Reference:    patchobs.ReferenceDescriptor{Kind: patchobs.ReferenceKindCommit, Commit: strings.Repeat("a", 40)},
		PatchPresent: true, PatchBytes: []byte(patch), PatchSHA256: CoverageSHA256([]byte(patch)),
		Effects: []patchobs.EffectObservation{},
	}
	parsed, err := gitutil.NormalizePatchEffects(patch)
	if err != nil {
		obs.ParseRefusal = err.Error()
	} else {
		if len(images) != 0 && len(images) != len(parsed) {
			t.Fatalf("fixture image count %d, effects %d", len(images), len(parsed))
		}
		for i, e := range parsed {
			entry := patchobs.EffectObservation{Effect: e}
			if len(images) != 0 {
				image := images[i]
				entry.Effect.PreimageObserved, entry.Effect.PostimageObserved = true, true
				pre, post := e.ExtantSides()
				entry.Effect.PreimagePresent, entry.Effect.PostimagePresent = pre, post
				if pre {
					entry.Effect.OldMode = image.oldMode
					if image.oldMode == "" {
						entry.Effect.OldMode = gitutil.ModeRegular
					}
					entry.Effect.PreimageSHA256 = CoverageSHA256([]byte(image.pre))
					if entry.Effect.OldMode != gitutil.ModeGitlink {
						entry.Bytes.Preimage = []byte(image.pre)
					}
				}
				if post {
					entry.Effect.NewMode = image.newMode
					if image.newMode == "" {
						entry.Effect.NewMode = gitutil.ModeRegular
					}
					entry.Effect.PostimageSHA256 = CoverageSHA256([]byte(image.post))
					if entry.Effect.NewMode != gitutil.ModeGitlink {
						entry.Bytes.Postimage = []byte(image.post)
					}
				}
			}
			entry.Effect.ContentKind, entry.Effect.ObjectKind = patchobs.ClassifyEffectObservation(entry.Effect, entry.Bytes)
			obs.Effects = append(obs.Effects, entry)
		}
	}
	obs.Reference.PreimageSetSHA256 = patchobs.PreimageSetDigest(obs.Effects)
	return obs
}

func rgaS3Inputs(t *testing.T, obs patchobs.Observation, ops ...RecipeOperation) RecipeCoverageInput {
	t.Helper()
	in := RecipeCoverageInput{Observation: obs}
	if len(ops) != 0 {
		raw, err := EncodeRecipe(ApplyRecipe{Feature: obs.Slug, Operations: ops})
		if err != nil {
			t.Fatal(err)
		}
		in.Recipe = CoverageArtifact{Present: true, Bytes: raw, Path: "artifacts/apply-recipe.json"}
	}
	return in
}

func rgaS3Write(path, pre, post string, created bool) RecipeOperation {
	hash := "sha256:" + CoverageSHA256([]byte(pre))
	if created {
		hash = ""
	}
	return RecipeOperation{Type: "write-file", Path: path, Content: post, PreimageHash: &hash}
}

func rgaS3Build(t *testing.T, in RecipeCoverageInput) RecipeCoverage {
	t.Helper()
	c, err := BuildRecipeCoverage(in)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateRecipeCoverage(c, in); err != nil {
		t.Fatalf("builder and input-aware validator disagree: %v", err)
	}
	raw, err := EncodeRecipeCoverage(c)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeRecipeCoverage(raw)
	if err != nil || !reflect.DeepEqual(decoded, c) {
		t.Fatalf("wire round trip: %v", err)
	}
	return c
}

func rgaS3Clone(c RecipeCoverage) RecipeCoverage {
	raw, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	var out RecipeCoverage
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	return out
}

func rgaS3Rehash(t *testing.T, e *CoverageEffect) {
	t.Helper()
	var err error
	e.EffectSHA256, err = CoverageEffectSHA256(*e)
	if err != nil {
		t.Fatal(err)
	}
}

func rgaS3RejectSchema(t *testing.T, c RecipeCoverage) {
	t.Helper()
	if err := ValidateRecipeCoverageSchema(c); err == nil {
		t.Fatal("structural validator accepted wrong input")
	}
	if _, err := EncodeRecipeCoverage(c); err == nil {
		t.Fatal("encoder accepted wrong input")
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeRecipeCoverage(raw); err == nil {
		t.Fatal("decoder accepted wrong input")
	}
}

func TestRGAS3ExactBindingsAndDeterminism(t *testing.T) {
	obs := rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"})
	in := rgaS3Inputs(t, obs, rgaS3Write("a.txt", "old\n", "new\n", false))
	c := rgaS3Build(t, in)
	if c.CoverageStatus != CoverageComplete || c.CrossBaseStatus != CrossBaseConsumerDerivationRequired ||
		len(c.Reasons) != 0 || c.Effects[0].Disposition != "represented" ||
		!slices.Equal(c.Effects[0].OperationIndexes, []int{1}) {
		t.Fatalf("ordinary modify: %+v", c)
	}
	if c.PatchSHA256 != CoverageSHA256(obs.PatchBytes) || c.RecipeSHA256 != CoverageSHA256(in.Recipe.Bytes) ||
		c.Reference.PreimageSetSHA256 != obs.Reference.PreimageSetSHA256 ||
		c.Effects[0].PatchFragmentSHA256 != CoverageSHA256([]byte(rgaS3ModifyPatch)) {
		t.Fatal("not bound to exact raw inputs")
	}
	first, _ := EncodeRecipeCoverage(c)
	for i := 0; i < 5; i++ {
		again := rgaS3Build(t, in)
		raw, _ := EncodeRecipeCoverage(again)
		if !bytes.Equal(first, raw) {
			t.Fatal("same immutable inputs produced different bytes")
		}
	}
	if bytes.Contains(first, []byte("old\\n")) || bytes.Contains(first, []byte("new\\n")) {
		t.Fatal("coverage persisted source bodies")
	}
}

func TestRGAS3ArtifactRealityAndIff(t *testing.T) {
	for _, patch := range []struct {
		name, raw, reason string
		present           bool
	}{
		{"missing", "", "canonical-patch-missing", false},
		{"empty", "", "canonical-patch-empty", true},
		{"whitespace", " \t\r\n", "canonical-patch-empty", true},
		{"unicode-whitespace", "\u2003\n", "canonical-patch-empty", true},
		{"unparseable", "not a canonical patch\n", "canonical-patch-unparseable", true},
	} {
		t.Run(patch.name, func(t *testing.T) {
			obs := rgaS3Observe(t, patch.raw)
			if !patch.present {
				obs.PatchPresent, obs.PatchSHA256, obs.PatchBytes = false, "", nil
			}
			c := rgaS3Build(t, rgaS3Inputs(t, obs))
			if c.CoverageStatus != CoverageIncomplete || c.CrossBaseStatus != CrossBaseUnsupported ||
				len(c.Effects) != 0 || !slices.Equal(c.Reasons, []string{patch.reason}) {
				t.Fatalf("untruthful artifact state: %+v", c)
			}
			if patch.present && c.PatchSHA256 != CoverageSHA256([]byte(patch.raw)) {
				t.Fatal("missing raw hash for readable patch")
			}
			bad := rgaS3Clone(c)
			bad.CoverageStatus, bad.CrossBaseStatus = CoverageComplete, CrossBaseReferenceTreeOnly
			rgaS3RejectSchema(t, bad)
			if err := ValidateRecipeCoverage(RecipeCoverage{}, rgaS3Inputs(t, obs)); err == nil {
				t.Fatal("absence of a record accepted")
			}
		})
	}
	obs := rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"})
	for _, recipe := range []struct {
		name        string
		artifact    CoverageArtifact
		undecodable bool
	}{
		{"absent", CoverageArtifact{}, false},
		{"unreadable", CoverageArtifact{Path: "artifacts/apply-recipe.json", ReadError: errors.New("permission denied")}, false},
		{"malformed", CoverageArtifact{Present: true, Bytes: []byte("{malformed")}, true},
		{"null", CoverageArtifact{Present: true, Bytes: []byte("null")}, true},
		{"empty-operations", CoverageArtifact{Present: true, Bytes: []byte(`{"feature":"s3","operations":[]}`)}, true},
	} {
		t.Run(recipe.name, func(t *testing.T) {
			in := RecipeCoverageInput{Observation: obs, Recipe: recipe.artifact}
			c := rgaS3Build(t, in)
			if c.RecipePresent != recipe.artifact.Present || c.RecipeDecodable || c.CoverageStatus != CoverageIncomplete ||
				len(c.Effects[0].OperationIndexes) != 0 ||
				!slices.Equal(c.Effects[0].ReasonCodes, []string{"operation-missing"}) ||
				slices.Contains(c.Reasons, "recipe-undecodable") != recipe.undecodable {
				t.Fatalf("recipe state conflation: %+v", c)
			}
			if recipe.artifact.Present && c.RecipeSHA256 != CoverageSHA256(recipe.artifact.Bytes) {
				t.Fatal("corrupt recipe lost its raw hash")
			}
		})
	}
}

func TestRGAS3ClosedExclusionCohorts(t *testing.T) {
	// Literal headers and image bytes; the expected axes/reasons are not
	// computed with the builder or its classification/reason helpers.
	cases := []struct {
		name, patch string
		image       rgaS3Image
		content     gitutil.ContentKind
		object      gitutil.ObjectKind
		reasons     []string
	}{
		{"delete", rgaS3DeletePatch, rgaS3Image{pre: "old\n"}, "text", "regular", []string{"effect-delete-unsupported"}},
		{"rename", rgaS3RenamePatch, rgaS3Image{pre: "old\n", post: "old\n"}, "text", "regular", []string{"effect-rename-unsupported"}},
		{"copy", rgaS3CopyPatch, rgaS3Image{pre: "old\n", post: "old\n"}, "text", "regular", []string{"effect-copy-unsupported"}},
		{"binary-rename", rgaS3RenamePatch + "Binary files a/old.txt and b/a.txt differ\n", rgaS3Image{pre: "\x00old", post: "\x00new"}, "binary", "regular", []string{"effect-binary-unsupported", "effect-rename-unsupported"}},
		{"executable-add", strings.Replace(rgaS3AddPatch, "100644", "100755", 1), rgaS3Image{post: "new\n", newMode: "100755"}, "text", "executable", []string{"effect-executable-unsupported"}},
		{"executable-rename", rgaS3RenamePatch, rgaS3Image{pre: "old\n", post: "old\n", oldMode: "100755", newMode: "100755"}, "text", "executable", []string{"effect-executable-unsupported", "effect-rename-unsupported"}},
		{"symlink-delete", strings.Replace(rgaS3DeletePatch, "100644", "120000", 1), rgaS3Image{pre: "old\n", oldMode: "120000"}, "text", "symlink", []string{"effect-delete-unsupported", "effect-symlink-unsupported"}},
		{"mode-only", "diff --git a/a.txt b/a.txt\nold mode 100644\nnew mode 100755\n", rgaS3Image{pre: "same", post: "same", newMode: "100755"}, "text", "executable", []string{"effect-executable-unsupported", "effect-mode-only-unsupported"}},
		{"symlink-transition", "diff --git a/a.txt b/a.txt\nold mode 100644\nnew mode 120000\n", rgaS3Image{pre: "old", post: "link", newMode: "120000"}, "text", "symlink", []string{"effect-symlink-unsupported"}},
		{"gitlink", "diff --git a/a.txt b/a.txt\nindex 1111111..2222222 160000\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-Subproject commit 1111111111111111111111111111111111111111\n+Subproject commit 2222222222222222222222222222222222222222\n", rgaS3Image{pre: strings.Repeat("1", 40), post: strings.Repeat("2", 40), oldMode: "160000", newMode: "160000"}, "none", "gitlink", []string{"effect-gitlink-unsupported"}},
		{"binary-by-NUL", rgaS3ModifyPatch, rgaS3Image{pre: "old\x00", post: "new\x00"}, "binary", "regular", []string{"effect-binary-unsupported"}},
		{"regular-to-gitlink", "diff --git a/a.txt b/a.txt\nold mode 100644\nnew mode 160000\n", rgaS3Image{pre: "old", post: strings.Repeat("2", 40), newMode: "160000"}, "none", "gitlink", []string{"effect-gitlink-unsupported"}},
		{"gitlink-to-regular", "diff --git a/a.txt b/a.txt\nold mode 160000\nnew mode 100644\n", rgaS3Image{pre: strings.Repeat("1", 40), post: "new", oldMode: "160000"}, "text", "regular", []string{"effect-gitlink-unsupported"}},
	}
	for _, tc := range cases {
		for _, state := range []string{"absent", "unreadable", "undecodable"} {
			t.Run(tc.name+"/"+state, func(t *testing.T) {
				in := rgaS3Inputs(t, rgaS3Observe(t, tc.patch, tc.image))
				if state == "unreadable" {
					in.Recipe.ReadError = errors.New("real read failure")
				} else if state == "undecodable" {
					in.Recipe = CoverageArtifact{Present: true, Bytes: []byte("broken")}
				}
				c := rgaS3Build(t, in)
				if len(c.Effects) != 1 {
					t.Fatalf("literal cohort did not parse: %+v", c)
				}
				e := c.Effects[0]
				if e.ContentKind != tc.content || e.ObjectKind != tc.object || e.Disposition != "unsupported" ||
					!slices.Equal(e.ReasonCodes, tc.reasons) || c.CoverageStatus != CoverageIncomplete {
					t.Fatalf("cohort collapsed: %+v", e)
				}
				for _, wrong := range []string{"omitted", "invented", "operation-missing"} {
					bad := rgaS3Clone(c)
					switch wrong {
					case "omitted":
						bad.Effects[0].ReasonCodes = bad.Effects[0].ReasonCodes[1:]
					case "invented":
						bad.Effects[0].ReasonCodes = coverageSortedCopy(append(bad.Effects[0].ReasonCodes, "preimage-unavailable"))
					default:
						bad.Effects[0].ReasonCodes = coverageSortedCopy(append(bad.Effects[0].ReasonCodes, "operation-missing"))
					}
					rgaS3RejectSchema(t, bad)
				}
			})
		}
	}
}

func TestRGAS3AvailabilityIsNotClassification(t *testing.T) {
	for _, tc := range []struct {
		name, patch  string
		image        rgaS3Image
		unobservePre bool
		kind         gitutil.ChangeKind
		reasons      []string
	}{
		{"half-add", rgaS3AddPatch, rgaS3Image{post: "new\n"}, true, gitutil.ChangeKindAdd, []string{"preimage-unavailable"}},
		{"half-delete", rgaS3DeletePatch, rgaS3Image{pre: "old\n"}, false, gitutil.ChangeKindDelete, []string{"effect-delete-unsupported", "postimage-unavailable"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obs := rgaS3Observe(t, tc.patch, tc.image)
			e := &obs.Effects[0].Effect
			if tc.unobservePre {
				e.PreimageObserved = false
			} else {
				e.PostimageObserved = false
			}
			obs.Reference.PreimageSetSHA256 = patchobs.PreimageSetDigest(obs.Effects)
			in := rgaS3Inputs(t, obs)
			c := rgaS3Build(t, in)
			if c.CoverageStatus != CoverageIncomplete || c.Effects[0].ContentKind != "text" || c.Effects[0].ObjectKind != "regular" ||
				c.Effects[0].ChangeKind != tc.kind || c.Effects[0].Disposition != "ambiguous" ||
				!slices.Equal(c.Effects[0].ReasonCodes, tc.reasons) {
				t.Fatalf("unavailable non-extant side erased known classification: %+v", c)
			}
			for _, axis := range []string{"object", "content"} {
				bad := rgaS3Clone(c)
				if axis == "object" {
					bad.Effects[0].ObjectKind = "unknown"
				} else {
					bad.Effects[0].ContentKind = "unknown"
				}
				rgaS3Rehash(t, &bad.Effects[0])
				rgaS3RejectSchema(t, bad)
			}
			// Predicate 8 fails in its own right, not only because delete is
			// unsupported: the pure simulator independently refuses this side.
			result, err := SimulateRecipeCoverage(obs, ApplyRecipe{Feature: "s3", Operations: []RecipeOperation{rgaS3Write("a.txt", tc.image.pre, tc.image.post, tc.unobservePre)}})
			if err != nil || result.ExactPostimage {
				t.Fatalf("half-observed cohort simulated as exact: %+v %v", result, err)
			}
		})
	}
	for _, patch := range []string{rgaS3RenamePatch, "diff --git a/a.txt b/a.txt\nold mode 100644\nnew mode 100755\n",
		"diff --git a/a.txt b/a.txt\nold mode 100644\nnew mode 120000\n"} {
		obs := rgaS3Observe(t, patch)
		obs.Capture.Mode, obs.Reference.Kind, obs.Reference.Commit = patchobs.CaptureModeNoCapture, patchobs.ReferenceKindUnavailable, ""
		c := rgaS3Build(t, rgaS3Inputs(t, obs))
		e := c.Effects[0]
		if e.ContentKind != "unknown" || e.ObjectKind != "unknown" || e.OldMode != "" || e.NewMode != "" || e.Disposition != "ambiguous" ||
			!slices.Contains(e.ReasonCodes, "preimage-unavailable") || !slices.Contains(e.ReasonCodes, "postimage-unavailable") ||
			slices.Contains(e.ReasonCodes, "operation-missing") || slices.Contains(e.ReasonCodes, "effect-mode-only-unsupported") ||
			slices.Contains(e.ReasonCodes, "effect-symlink-unsupported") {
			t.Fatalf("header inferred evidence never observed: %+v", e)
		}
	}
}

func TestRGAS3RejectsContradictoryObservationsAndMutation(t *testing.T) {
	obs := rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"})
	in := rgaS3Inputs(t, obs, rgaS3Write("a.txt", "old\n", "new\n", false))
	c := rgaS3Build(t, in)
	for _, name := range []string{"pre-body", "post-body", "patch", "count", "ordinal", "mode", "header", "axis", "observed-absence", "released-body", "reference"} {
		t.Run(name, func(t *testing.T) {
			bad := in
			bad.Observation.Effects = slices.Clone(obs.Effects)
			e := &bad.Observation.Effects[0]
			switch name {
			case "pre-body":
				e.Bytes.Preimage = []byte("changed")
			case "post-body":
				e.Bytes.Postimage = []byte("changed")
			case "patch":
				bad.Observation.PatchBytes = append(bytes.Clone(obs.PatchBytes), '\n')
			case "count":
				bad.Observation.Effects = nil
			case "ordinal":
				e.Effect.Ordinal++
			case "mode":
				e.Effect.NewMode = ""
			case "header":
				e.Effect.HeaderNewMode = "100755"
			case "axis":
				e.Effect.ObjectKind = "symlink"
			case "observed-absence":
				e.Effect.PreimagePresent, e.Effect.OldMode, e.Effect.PreimageSHA256, e.Bytes.Preimage = false, "", "", nil
			case "released-body":
				e.Bytes.Preimage = nil
			case "reference":
				bad.Observation.Reference.PreimageSetSHA256 = strings.Repeat("0", 64)
			}
			if _, err := BuildRecipeCoverage(bad); err == nil {
				t.Fatal("builder accepted contradictory observation")
			}
			if err := ValidateRecipeCoverage(c, bad); err == nil {
				t.Fatal("input-aware validator accepted changed observation")
			}
		})
	}
	// Root names never get opened. Even a nonexistent alternate root is
	// immaterial when all normalized lexical target relations are unchanged.
	before, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	in.Observation.RepoRoot = filepath.Join(rgaS3Root(), "another-missing-root")
	second := rgaS3Build(t, in)
	if !coverageEqual(c, second) {
		t.Fatal("core used live repository state")
	}
	in.Observation.RepoRoot = obs.RepoRoot
	after, err := json.Marshal(in)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("core mutated inputs: %v", err)
	}
}

func TestRGAS3ExtantSideAxisSelectionAndUnavailableCompany(t *testing.T) {
	for _, side := range []string{"pre", "post"} {
		obs := rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"})
		entry := &obs.Effects[0]
		if side == "pre" {
			entry.Effect.PreimageObserved, entry.Effect.PreimagePresent = false, false
			entry.Effect.OldMode, entry.Effect.PreimageSHA256, entry.Bytes.Preimage = "", "", nil
		} else {
			entry.Effect.PostimageObserved, entry.Effect.PostimagePresent = false, false
			entry.Effect.NewMode, entry.Effect.PostimageSHA256, entry.Bytes.Postimage = "", "", nil
		}
		entry.Effect.ContentKind, entry.Effect.ObjectKind = patchobs.ClassifyEffectObservation(entry.Effect, entry.Bytes)
		obs.Reference.PreimageSetSHA256 = patchobs.PreimageSetDigest(obs.Effects)
		c := rgaS3Build(t, rgaS3Inputs(t, obs))
		wantObject := "unknown"
		if side == "pre" {
			wantObject = "regular"
		}
		if string(c.Effects[0].ObjectKind) != wantObject || c.Effects[0].ContentKind != "unknown" ||
			!slices.Equal(c.Effects[0].ReasonCodes, []string{side + "image-unavailable"}) {
			t.Fatalf("extant-side classification: %+v", c.Effects[0])
		}
		bad := rgaS3Clone(c)
		bad.Effects[0].ReasonCodes = []string{}
		bad.Effects[0].Disposition = "represented"
		rgaS3RejectSchema(t, bad)
	}
	obs := rgaS3Observe(t, "diff --git a/a.txt b/a.txt\nBinary files a/a.txt and b/a.txt differ\n")
	c := rgaS3Build(t, rgaS3Inputs(t, obs))
	if c.Effects[0].ContentKind != "binary" || c.Effects[0].ObjectKind != "unknown" ||
		!slices.Equal(c.Effects[0].ReasonCodes, []string{"effect-binary-unsupported", "postimage-unavailable", "preimage-unavailable"}) {
		t.Fatalf("stanza's positive binary evidence was lost: %+v", c.Effects[0])
	}
}

func TestRGAS3ObservationModesNeverComeFromMissingHeaders(t *testing.T) {
	patch := strings.Replace(rgaS3ModifyPatch, "index 3367afd..3e75765 100644\n", "", 1)
	obs := rgaS3Observe(t, patch, rgaS3Image{pre: "old\n", post: "new\n"})
	if obs.Effects[0].Effect.HeaderOldMode != "" || obs.Effects[0].Effect.HeaderNewMode != "" {
		t.Fatal("fixture still contains a mode header")
	}
	in := rgaS3Inputs(t, obs, rgaS3Write("a.txt", "old\n", "new\n", false))
	c := rgaS3Build(t, in)
	if c.Effects[0].OldMode != "100644" || c.Effects[0].NewMode != "100644" {
		t.Fatal("omitted header erased observed modes")
	}
	for _, field := range []string{"OldMode", "NewMode"} {
		bad := rgaS3Clone(c)
		reflect.ValueOf(&bad.Effects[0]).Elem().FieldByName(field).SetString("")
		rgaS3Rehash(t, &bad.Effects[0])
		rgaS3RejectSchema(t, bad)
	}
}

func TestRGAS3RecordReasonsAndRawManualBytes(t *testing.T) {
	obs := rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"})
	good := rgaS3Inputs(t, obs, rgaS3Write("a.txt", "old\n", "new\n", false))
	var compact bytes.Buffer
	if err := json.Compact(&compact, good.Recipe.Bytes); err != nil {
		t.Fatal(err)
	}
	good.Recipe.Bytes = append([]byte(" \n"), compact.Bytes()...)
	c := rgaS3Build(t, good)
	if c.RecipeSHA256 != CoverageSHA256(good.Recipe.Bytes) {
		t.Fatal("manual bytes recanonicalized for binding")
	}
	for _, tc := range []struct {
		name    string
		change  func(*RecipeCoverageInput)
		reasons []string
	}{
		{"non-durable", func(in *RecipeCoverageInput) {
			in.Observation.Reference.Kind = patchobs.ReferenceKindIndexSnapshot
			in.Observation.Reference.Commit = ""
		}, []string{"reference-not-durable"}},
		{"owner", func(in *RecipeCoverageInput) {
			in.Recipe.Bytes = bytes.Replace(in.Recipe.Bytes, []byte(`"s3"`), []byte(`"other"`), 1)
		}, []string{"recipe-owner-mismatch"}},
		{"stale", func(in *RecipeCoverageInput) { in.Events.StaleMarkerPresent = true }, []string{"recipe-stale-marker-present"}},
		{"rewrite-still-exact", func(in *RecipeCoverageInput) { in.Events.PatchRewritten = true }, []string{}},
		{"rewrite-mismatch", func(in *RecipeCoverageInput) {
			in.Events.PatchRewritten = true
			in.Recipe.Bytes = bytes.Replace(in.Recipe.Bytes, []byte(`new\n`), []byte(`wrong\n`), 1)
		}, []string{"producer-patch-rewrite", "recipe-not-regenerated", "simulation-mismatch"}},
		{"regenerated-but-wrong", func(in *RecipeCoverageInput) {
			in.Events.PatchRewritten = true
			in.Events.RecipeRegenerated = true
			in.Recipe.Bytes = bytes.Replace(in.Recipe.Bytes, []byte(`new\n`), []byte(`wrong\n`), 1)
		}, []string{"simulation-mismatch"}},
		{"manual-edited-exact", func(in *RecipeCoverageInput) {
			in.Observation.Producer = patchobs.ProducerEdit
			in.Events.BoundArtifactEdited = true
		}, []string{}},
		{"manual-edited-wrong", func(in *RecipeCoverageInput) {
			in.Observation.Producer = patchobs.ProducerEdit
			in.Events.BoundArtifactEdited = true
			in.Recipe.Bytes = bytes.Replace(in.Recipe.Bytes, []byte(`new\n`), []byte(`wrong\n`), 1)
		}, []string{"manual-bound-artifact-edit", "simulation-mismatch"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := good
			tc.change(&in)
			got := rgaS3Build(t, in)
			if !slices.Equal(got.Reasons, tc.reasons) {
				t.Fatalf("exact reason set = %v, want %v", got.Reasons, tc.reasons)
			}
			for _, change := range []string{"omission", "invention", "status"} {
				bad := rgaS3Clone(got)
				switch change {
				case "omission":
					if len(bad.Reasons) == 0 {
						continue
					}
					bad.Reasons = bad.Reasons[1:]
				case "invention":
					bad.Reasons = coverageSortedCopy(append(bad.Reasons, "recipe-stale-marker-present"))
					if slices.Equal(bad.Reasons, got.Reasons) {
						continue
					}
					bad.CoverageStatus, bad.CrossBaseStatus = CoverageIncomplete, CrossBaseUnsupported
				case "status":
					bad.CoverageStatus = CoverageComplete
					if got.CoverageStatus == CoverageComplete {
						bad.CoverageStatus = CoverageIncomplete
					}
				}
				if err := ValidateRecipeCoverage(bad, in); err == nil {
					t.Fatalf("same validator accepted %s", change)
				}
			}
		})
	}
	for _, mutate := range []func(*RecipeCoverageInput){
		func(in *RecipeCoverageInput) { in.Recipe.Present = false; in.Recipe.Bytes = nil },
		func(in *RecipeCoverageInput) { in.Recipe.Bytes = bytes.Clone(compact.Bytes()) },
		func(in *RecipeCoverageInput) { in.Recipe.Bytes = []byte("invalid") },
		func(in *RecipeCoverageInput) { in.Observation.Slug = "other" },
		func(in *RecipeCoverageInput) { in.Observation.Reference.Commit = strings.Repeat("b", 40) },
		func(in *RecipeCoverageInput) { in.Observation.Capture.ClaimIDs = []string{"claim-other"} },
	} {
		wrong := good
		mutate(&wrong)
		if err := ValidateRecipeCoverage(c, wrong); err == nil {
			t.Fatal("stored-label trust accepted binding drift")
		}
	}
}

func TestRGAS3InputDiagnosticsAreNotWireCodes(t *testing.T) {
	obs := rgaS3Observe(t, rgaS3ModifyPatch)
	obs.Effects[0].ReasonCodes = []string{patchobs.ReasonPreimageModeContradiction, patchobs.ReasonPostimageModeContradiction}
	obs.Effects[0].Contradictions = []string{"header disagreement: body must not be persisted"}
	c := rgaS3Build(t, rgaS3Inputs(t, obs))
	if !slices.Equal(c.Effects[0].ReasonCodes, []string{"postimage-unavailable", "preimage-unavailable"}) {
		t.Fatalf("S1 diagnostics became wire reasons: %v", c.Effects[0].ReasonCodes)
	}
	raw, _ := EncodeRecipeCoverage(c)
	if strings.Contains(string(raw), "contradiction") || strings.Contains(string(raw), "body must") {
		t.Fatal("diagnostic persisted")
	}
	bad := rgaS3Clone(c)
	bad.Effects[0].ReasonCodes = coverageSortedCopy(append(bad.Effects[0].ReasonCodes, patchobs.ReasonPreimageModeContradiction))
	rgaS3RejectSchema(t, bad)
}

func TestRGAS3ParentExclusionsUseExecutionLexicalTargets(t *testing.T) {
	obs := rgaS3Observe(t, rgaS3AddPatch, rgaS3Image{post: "new\n"})
	for _, parent := range []string{"a.txt", "./a.txt", "folder/../a.txt"} {
		t.Run(parent, func(t *testing.T) {
			in := rgaS3Inputs(t, obs, rgaS3Write("./a.txt", "", "new\n", true))
			in.Observation.ParentCreatedPaths = []string{parent}
			c := rgaS3Build(t, in)
			if c.CoverageStatus != CoverageIncomplete || !slices.Equal(c.Effects[0].ReasonCodes, []string{"parent-created-target-unsupported"}) {
				t.Fatalf("parent alias escaped exclusion: %+v", c)
			}
			bad := rgaS3Clone(c)
			bad.Effects[0].ReasonCodes = []string{}
			bad.Effects[0].Disposition = "represented"
			bad.CoverageStatus, bad.CrossBaseStatus = CoverageComplete, CrossBaseReferenceTreeOnly
			if err := ValidateRecipeCoverage(bad, in); err == nil {
				t.Fatal("parent-body reuse accepted")
			}
		})
	}
	obs = rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"})
	obs.ParentCreatedPaths = []string{"a.txt"}
	c := rgaS3Build(t, rgaS3Inputs(t, obs, rgaS3Write("a.txt", "old\n", "new\n", false)))
	if c.CoverageStatus != CoverageComplete {
		t.Fatal("historical parent origin erased an independently observed preimage")
	}
}

func TestRGAS3EveryEffectFieldBinds(t *testing.T) {
	c := rgaS3Build(t, rgaS3Inputs(t, rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"}), rgaS3Write("a.txt", "old\n", "new\n", false)))
	e := c.Effects[0]
	base, err := CoverageEffectSHA256(e)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"Ordinal", "ChangeKind", "ContentKind", "ObjectKind", "Path", "OldPath", "OldMode", "NewMode",
		"PreimageObserved", "PreimagePresent", "PreimageSHA256", "PostimageObserved", "PostimagePresent", "PostimageSHA256", "PatchFragmentSHA256"} {
		t.Run(field, func(t *testing.T) {
			bad := e
			v := reflect.ValueOf(&bad).Elem().FieldByName(field)
			switch v.Kind() {
			case reflect.String:
				v.SetString(v.String() + "x")
			case reflect.Bool:
				v.SetBool(!v.Bool())
			case reflect.Int:
				v.SetInt(v.Int() + 1)
			}
			hash, err := CoverageEffectSHA256(bad)
			if err != nil || hash == base {
				t.Fatalf("unbound descriptor field %s: %v", field, err)
			}
		})
	}
	for _, field := range []string{"EffectSHA256", "Disposition", "ContextualHint"} {
		bad := e
		reflect.ValueOf(&bad).Elem().FieldByName(field).SetString("advisory")
		hash, err := CoverageEffectSHA256(bad)
		if err != nil || hash != base {
			t.Fatalf("hidden digest input %s: %v", field, err)
		}
	}
	bad := e
	bad.OperationIndexes = []int{99}
	bad.ReasonCodes = []string{"operation-missing"}
	hash, err := CoverageEffectSHA256(bad)
	if err != nil || hash != base {
		t.Fatal(fmt.Sprintf("assignment/reason entered descriptor hash: %v", err))
	}
}
