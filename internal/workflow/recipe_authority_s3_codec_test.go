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

func TestRGAS3StrictJSONAllNestingLevels(t *testing.T) {
	c := rgaS3Build(t, rgaS3Inputs(t,
		rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"}),
		rgaS3Write("a.txt", "old\n", "new\n", false)))
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	objects := []struct {
		name string
		node map[string]any
	}{
		{"record", doc},
		{"reference", doc["reference"].(map[string]any)},
		{"capture", doc["capture"].(map[string]any)},
		{"effect", doc["effects"].([]any)[0].(map[string]any)},
	}
	for _, object := range objects {
		// Snapshot the keys; mutations never alter the iteration's inventory.
		keys := []string{}
		for key := range object.node {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			t.Run(object.name+"/"+key, func(t *testing.T) {
				value := object.node[key]
				for _, kind := range []string{"missing", "null", "wrong-type"} {
					if kind == "missing" {
						delete(object.node, key)
					} else if kind == "null" {
						object.node[key] = nil
					} else {
						object.node[key] = map[string]any{"unexpected": true}
					}
					bad, err := json.Marshal(doc)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := DecodeRecipeCoverage(bad); err == nil {
						t.Fatalf("accepted %s %s.%s", kind, object.name, key)
					}
					object.node[key] = value
				}
			})
		}
		object.node["invented-authority"] = true
		bad, _ := json.Marshal(doc)
		delete(object.node, "invented-authority")
		if _, err := DecodeRecipeCoverage(bad); err == nil {
			t.Fatalf("unknown field accepted at %s", object.name)
		}
	}
	for _, mutation := range []struct {
		name string
		raw  []byte
	}{
		{"trailing-object", append(bytes.Clone(raw), []byte("{}")...)},
		{"trailing-null", append(bytes.Clone(raw), []byte("null")...)},
		{"top-null", []byte("null")},
		{"duplicate-record", bytes.Replace(raw, []byte(`"feature":"s3"`), []byte(`"feature":"s3","feature":"s3"`), 1)},
		{"duplicate-reference", bytes.Replace(raw, []byte(`"kind":"commit"`), []byte(`"kind":"commit","kind":"commit"`), 1)},
		{"duplicate-capture", bytes.Replace(raw, []byte(`"mode":"working-tree-all"`), []byte(`"mode":"working-tree-all","mode":"working-tree-all"`), 1)},
		{"duplicate-effect", bytes.Replace(raw, []byte(`"ordinal":1`), []byte(`"ordinal":1,"ordinal":1`), 1)},
		{"duplicate-escaped-key", bytes.Replace(raw, []byte(`"feature":"s3"`), []byte(`"feature":"s3","\u0066eature":"s3"`), 1)},
		{"case-folded-key", bytes.Replace(raw, []byte(`"feature":`), []byte(`"Feature":`), 1)},
		{"fractional-integer", bytes.Replace(raw, []byte(`"ordinal":1`), []byte(`"ordinal":1.0`), 1)},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			if bytes.Equal(mutation.raw, raw) {
				t.Fatal("dead mutation")
			}
			if _, err := DecodeRecipeCoverage(mutation.raw); err == nil {
				t.Fatal("strict decoder accepted wrong JSON")
			}
		})
	}
}

func TestRGAS3CodecRejectsLossyUnicode(t *testing.T) {
	c := rgaS3Build(t, rgaS3Inputs(t, rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"}), rgaS3Write("a.txt", "old\n", "new\n", false)))
	raw, _ := json.Marshal(c)
	for _, replacement := range []string{`"\ud800"`, `"\udfff"`, `"\ud800\u0041"`, "\"\xff\""} {
		bad := bytes.Replace(raw, []byte(`"feature":"s3"`), []byte(`"feature":`+replacement), 1)
		if bytes.Equal(raw, bad) {
			t.Fatal("dead Unicode mutation")
		}
		if _, err := DecodeRecipeCoverage(bad); err == nil {
			t.Fatalf("lossy Unicode accepted: %q", replacement)
		}
	}
	paired := bytes.Replace(raw, []byte(`"feature":"s3"`), []byte(`"feature":"\ud83d\ude00"`), 1)
	decoded, err := DecodeRecipeCoverage(paired)
	if err != nil || decoded.Feature != "😀" {
		t.Fatalf("valid Unicode pair refused: %v", err)
	}
	c.Feature = "\xff"
	if _, err := EncodeRecipeCoverage(c); err == nil {
		t.Fatal("encoder replaced invalid UTF-8")
	}
	e := c.Effects[0]
	e.Path = "\xfe"
	if _, err := CoverageEffectSHA256(e); err == nil {
		t.Fatal("effect digest normalized invalid filename bytes")
	}
}

func TestRGAS3ClosedEnumsAndFlagRelations(t *testing.T) {
	base := rgaS3Build(t, rgaS3Inputs(t, rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"}), rgaS3Write("a.txt", "old\n", "new\n", false)))
	for _, mutate := range []struct {
		name string
		fn   func(*RecipeCoverage)
	}{
		{"version", func(c *RecipeCoverage) { c.SchemaVersion = 2 }},
		{"producer", func(c *RecipeCoverage) { c.Producer = "trusted-generator" }},
		{"reference", func(c *RecipeCoverage) { c.Reference.Kind = "head" }},
		{"capture", func(c *RecipeCoverage) { c.Capture.Mode = "worktree" }},
		{"status", func(c *RecipeCoverage) { c.CoverageStatus = "eligible" }},
		{"cross-base", func(c *RecipeCoverage) { c.CrossBaseStatus = "safe" }},
		{"change", func(c *RecipeCoverage) { c.Effects[0].ChangeKind = "unknown" }},
		{"content", func(c *RecipeCoverage) { c.Effects[0].ContentKind = "utf8" }},
		{"object", func(c *RecipeCoverage) { c.Effects[0].ObjectKind = "file" }},
		{"mode", func(c *RecipeCoverage) { c.Effects[0].NewMode = "100600" }},
		{"hint", func(c *RecipeCoverage) { c.Effects[0].ContextualHint = "unique-anchor" }},
		{"disposition", func(c *RecipeCoverage) { c.Effects[0].Disposition = "eligible" }},
		{"recipe-absent", func(c *RecipeCoverage) { c.RecipePresent = false }},
		{"recipe-undecodable", func(c *RecipeCoverage) { c.RecipeDecodable = false }},
		{"patch-absent", func(c *RecipeCoverage) { c.PatchPresent = false }},
		{"patch-hash-empty", func(c *RecipeCoverage) { c.PatchSHA256 = "" }},
		{"recipe-hash-uppercase", func(c *RecipeCoverage) { c.RecipeSHA256 = strings.Repeat("A", 64) }},
		{"commit-short", func(c *RecipeCoverage) { c.Reference.Commit = "abcdef1" }},
		{"commit-on-unavailable", func(c *RecipeCoverage) { c.Reference.Kind = patchobs.ReferenceKindUnavailable }},
		{"reference-digest", func(c *RecipeCoverage) { c.Reference.PreimageSetSHA256 = "sha256:" + strings.Repeat("a", 64) }},
		{"reference-digest-does-not-bind", func(c *RecipeCoverage) { c.Reference.PreimageSetSHA256 = strings.Repeat("a", 64) }},
		{"no-capture-selectors", func(c *RecipeCoverage) {
			c.Capture.Mode = patchobs.CaptureModeNoCapture
			c.Capture.Pathspecs = []string{"a.txt"}
		}},
		{"nonnull-selector-order", func(c *RecipeCoverage) { c.Capture.Pathspecs = []string{"z", "a"} }},
		{"unknown-record-code", func(c *RecipeCoverage) { c.Reasons = []string{"source-trusted"} }},
		{"record-code-on-effect", func(c *RecipeCoverage) { c.Effects[0].ReasonCodes = []string{"simulation-mismatch"} }},
		{"effect-code-on-record", func(c *RecipeCoverage) { c.Reasons = []string{"operation-missing"} }},
		{"both-arrays", func(c *RecipeCoverage) {
			c.Reasons = []string{"operation-missing"}
			c.Effects[0].ReasonCodes = []string{"operation-missing"}
		}},
		{"not-contiguous", func(c *RecipeCoverage) { c.Effects[0].Ordinal = 2 }},
		{"zero-index", func(c *RecipeCoverage) { c.Effects[0].OperationIndexes = []int{0} }},
		{"negative-index", func(c *RecipeCoverage) { c.Effects[0].OperationIndexes = []int{-1} }},
		{"duplicate-index", func(c *RecipeCoverage) { c.Effects[0].OperationIndexes = []int{1, 1} }},
		{"unsorted-indexes", func(c *RecipeCoverage) { c.Effects[0].OperationIndexes = []int{2, 1} }},
		{"old-path-on-modify", func(c *RecipeCoverage) { c.Effects[0].OldPath = "old.txt" }},
		{"observed-absent-extant", func(c *RecipeCoverage) {
			e := &c.Effects[0]
			e.PreimagePresent = false
			e.OldMode = ""
			e.PreimageSHA256 = ""
		}},
		{"unobserved-present", func(c *RecipeCoverage) { c.Effects[0].PreimageObserved = false }},
		{"observed-present-no-mode", func(c *RecipeCoverage) { c.Effects[0].OldMode = "" }},
		{"observed-present-no-hash", func(c *RecipeCoverage) { c.Effects[0].PreimageSHA256 = "" }},
		{"none-on-regular", func(c *RecipeCoverage) { c.Effects[0].ContentKind = "none" }},
		{"unknown-on-observed", func(c *RecipeCoverage) { c.Effects[0].ContentKind = "unknown" }},
		{"improper-object-selection", func(c *RecipeCoverage) { c.Effects[0].ObjectKind = "executable" }},
		{"omitted-operation-missing", func(c *RecipeCoverage) { c.Effects[0].OperationIndexes = []int{} }},
		{"invented-operation-missing", func(c *RecipeCoverage) {
			c.Effects[0].ReasonCodes = []string{"operation-missing"}
			c.Effects[0].Disposition = "mismatch"
		}},
		{"invented-path-unsafe", func(c *RecipeCoverage) {
			c.Effects[0].ReasonCodes = []string{"path-unsafe"}
			c.Effects[0].Disposition = "unsupported"
			c.CoverageStatus, c.CrossBaseStatus = CoverageIncomplete, CrossBaseUnsupported
		}},
		{"disposition-both-directions", func(c *RecipeCoverage) { c.Effects[0].Disposition = "unsupported" }},
		{"complete-with-reasons", func(c *RecipeCoverage) { c.Reasons = []string{"recipe-stale-marker-present"} }},
		{"incomplete-without-cause", func(c *RecipeCoverage) {
			c.CoverageStatus = CoverageIncomplete
			c.CrossBaseStatus = CrossBaseUnsupported
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			bad := rgaS3Clone(base)
			mutate.fn(&bad)
			rgaS3Rehash(t, &bad.Effects[0])
			rgaS3RejectSchema(t, bad)
		})
	}
}

func TestRGAS3DispositionMappingBothDirections(t *testing.T) {
	cases := []RecipeCoverage{
		rgaS3Build(t, rgaS3Inputs(t, rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"}))),
		rgaS3Build(t, rgaS3Inputs(t, rgaS3Observe(t, rgaS3DeletePatch, rgaS3Image{pre: "old\n"}))),
		rgaS3Build(t, rgaS3Inputs(t, rgaS3Observe(t, rgaS3RenamePatch))),
	}
	for _, c := range cases {
		for _, disposition := range []string{"represented", "unsupported", "ambiguous", "mismatch"} {
			bad := rgaS3Clone(c)
			if disposition == bad.Effects[0].Disposition {
				continue
			}
			bad.Effects[0].Disposition = disposition
			rgaS3RejectSchema(t, bad)
		}
		if c.Effects[0].Disposition != "mismatch" {
			bad := rgaS3Clone(c)
			bad.Effects[0].ReasonCodes = coverageSortedCopy(append(bad.Effects[0].ReasonCodes, "operation-missing"))
			bad.Effects[0].Disposition = "mismatch"
			rgaS3RejectSchema(t, bad)
		}
	}
}

func TestRGAS3RecipeStrictnessAndExactRawHashes(t *testing.T) {
	obs := rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"})
	valid := `{"feature":"s3","operations":[{"type":"write-file","path":"a.txt","content":"new\n"}]}`
	for _, raw := range []string{
		valid + `{}`, strings.Replace(valid, `"feature":"s3"`, `"feature":"s3","feature":"s3"`, 1),
		strings.Replace(valid, `"path":"a.txt"`, `"path":"a.txt","path":"a.txt"`, 1),
		strings.Replace(valid, `"path":"a.txt"`, `"path":null`, 1),
		strings.Replace(valid, `"path":"a.txt"`, `"path":"a.txt","preimage_hash":null`, 1),
		strings.Replace(valid, `"path":"a.txt"`, `"path":"a.txt","unlisted":true`, 1),
		strings.Replace(valid, `"path":"a.txt"`, `"path":"a.txt","preimage_hash":"sha256:no"`, 1),
		strings.Replace(valid, `"content":"new\n"`, `"content":"\ud800"`, 1),
		strings.Replace(valid, `"content":"new\n"`, `"Content":"new\n"`, 1),
		`{"feature":"s3","operations":null}`,
	} {
		if raw == valid {
			t.Fatal("dead recipe JSON mutation")
		}
		in := RecipeCoverageInput{Observation: obs, Recipe: CoverageArtifact{Present: true, Bytes: []byte(raw)}}
		c := rgaS3Build(t, in)
		if !c.RecipePresent || c.RecipeDecodable || c.RecipeSHA256 != CoverageSHA256([]byte(raw)) ||
			!slices.Contains(c.Reasons, "recipe-undecodable") {
			t.Fatalf("dishonest corrupt recipe: %+v", c)
		}
	}
	in := RecipeCoverageInput{Observation: obs, Recipe: CoverageArtifact{Present: true, Bytes: []byte(valid)}}
	c := rgaS3Build(t, in)
	if !c.RecipePresent || !c.RecipeDecodable || c.RecipeSHA256 != CoverageSHA256([]byte(valid)) ||
		c.CoverageStatus != CoverageIncomplete || c.CrossBaseStatus != CrossBaseUnsupported ||
		!slices.Equal(c.Effects[0].ReasonCodes, []string{"operation-not-reclassifiable"}) || len(c.Reasons) != 0 {
		t.Fatal("valid legacy recipe must remain byte-bound and decodable but lack a v1 reclassification proof")
	}
	if CoverageSHA256(nil) != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatal("hash helper is not exact SHA-256")
	}
}

func TestRGAS3SchemaDoesNotPretendToProveSimulation(t *testing.T) {
	in := rgaS3Inputs(t, rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"}), rgaS3Write("a.txt", "old\n", "new\n", false))
	c := rgaS3Build(t, in)
	wrong := in
	wrong.Recipe.Bytes = bytes.Replace(in.Recipe.Bytes, []byte(`new\n`), []byte(`unrelated\n`), 1)
	// A matching raw hash by itself cannot establish simulation.
	forged := rgaS3Clone(c)
	forged.RecipeSHA256 = CoverageSHA256(wrong.Recipe.Bytes)
	raw, err := EncodeRecipeCoverage(forged)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeRecipeCoverage(raw); err != nil {
		t.Fatal("schema should not claim access to recipe bytes")
	}
	if err := ValidateRecipeCoverage(forged, wrong); err == nil {
		t.Fatal("bare JSON was accepted as simulation authority")
	}
	truth := rgaS3Build(t, wrong)
	if truth.CoverageStatus != CoverageIncomplete || !reflect.DeepEqual(truth.Reasons, []string{"simulation-mismatch"}) {
		t.Fatalf("same-file-set difference was not simulated: %+v", truth)
	}
}
