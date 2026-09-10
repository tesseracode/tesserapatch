package workflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/patchobs"
)

func rgaS5EventPair(t *testing.T, in RecipeCoverageInput) (RecipeCaptureEvent, []byte, RecipeCaptureBindings) {
	t.Helper()
	in, err := recipeCaptureInput(in)
	if err != nil {
		t.Fatal(err)
	}
	c, err := BuildRecipeCoverage(in)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeRecipeCoverage(c)
	if err != nil {
		t.Fatal(err)
	}
	e, err := BuildRecipeCaptureEvent(in, raw)
	if err != nil {
		t.Fatal(err)
	}
	bindings := RecipeCaptureBindings{
		RepoRoot: in.Observation.RepoRoot, Feature: in.Observation.Slug,
		Patch:  CoverageArtifact{Present: in.Observation.PatchPresent, Bytes: in.Observation.PatchBytes},
		Recipe: in.Recipe,
	}
	if err := ValidateRecipeCaptureEventSource(e, raw, in); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRecipeCaptureEventPair(e, raw, bindings); err != nil {
		t.Fatal(err)
	}
	return e, raw, bindings
}

func rgaS5CloneEvent(e RecipeCaptureEvent) RecipeCaptureEvent {
	raw, err := json.Marshal(e)
	if err != nil {
		panic(err)
	}
	var out RecipeCaptureEvent
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	return out
}

func rgaS5EventInput(t *testing.T) RecipeCoverageInput {
	t.Helper()
	return rgaS3Inputs(t, rgaS3Observe(t, rgaS3ModifyPatch, rgaS3Image{pre: "old\n", post: "new\n"}),
		rgaS3Write("a.txt", "old\n", "new\n", false))
}

func TestRGAS5CaptureEventCanonicalIdentityAndIndependentSource(t *testing.T) {
	in := rgaS5EventInput(t)
	event, rawC, bindings := rgaS5EventPair(t, in)
	first, err := EncodeRecipeCaptureEvent(event)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildRecipeCaptureEvent(in, rawC)
	if err != nil {
		t.Fatal(err)
	}
	secondRaw, err := EncodeRecipeCaptureEvent(second)
	if err != nil || !bytes.Equal(first, secondRaw) || !bytes.HasSuffix(first, []byte("}\n")) || bytes.HasSuffix(first, []byte("\n\n")) {
		t.Fatal("identical frozen inputs do not encode canonically")
	}
	identity, err := RecipeCaptureEventIdentity(event)
	want := sha256.Sum256(first)
	if err != nil || identity != hex.EncodeToString(want[:]) {
		t.Fatalf("identity is not the full canonical-byte hash: %s %v", identity, err)
	}
	wantPair := sha256.Sum256(rawC)
	if event.CoverageSHA256 != hex.EncodeToString(wantPair[:]) {
		t.Fatal("pairing does not include exact canonical C bytes")
	}
	decoded, err := DecodeRecipeCaptureEvent(first)
	if err != nil || !reflect.DeepEqual(decoded, event) {
		t.Fatalf("strict canonical round trip: %v", err)
	}
	for _, forbidden := range []string{`"producer"`, `"coverage_status"`, `"reasons"`, `"operation_indexes"`, `"event_id"`, `"generated_at"`, `"repo_root"`, `"content"`, `"preimage"`} {
		if bytes.Contains(first, []byte(forbidden)) {
			t.Fatalf("evidence gained a non-contract field %s", forbidden)
		}
	}
	changed := in
	changed.Observation.Capture.Mode = patchobs.CaptureModeStagedIndex
	newEvent, newC, _ := rgaS5EventPair(t, changed)
	newIdentity, err := RecipeCaptureEventIdentity(newEvent)
	if err != nil || identity == newIdentity || bytes.Equal(rawC, newC) ||
		newEvent.PatchSHA256 != event.PatchSHA256 || newEvent.RecipeSHA256 != event.RecipeSHA256 {
		t.Fatal("same artifacts with a different capture lost their independent event identity")
	}
	if err := ValidateRecipeCaptureEventSource(event, rawC, changed); err == nil {
		t.Fatal("source validator accepted a capture copied from the old coverage")
	}
	inert := rgaS5CloneEvent(event)
	inert.Event.PatchRewritten = true
	if err := ValidateRecipeCaptureEventPair(inert, rawC, bindings); err != nil {
		t.Fatalf("pair consistency pretended to authenticate an inert historical event fact: %v", err)
	}
	if err := ValidateRecipeCaptureEventSource(inert, rawC, in); err == nil {
		t.Fatal("producer source validation ignored the actual frozen event facts")
	}
	changed = in
	changed.Observation.Effects = append([]patchobs.EffectObservation{}, in.Observation.Effects...)
	changed.Observation.Effects[0].Bytes.Postimage = []byte("late mutated body\n")
	if _, err := BuildRecipeCaptureEvent(changed, rawC); err == nil {
		t.Fatal("builder accepted changed retained bytes without their captured binding")
	}
}

func TestRGAS5CaptureEventStrictWireMutations(t *testing.T) {
	e, _, _ := rgaS5EventPair(t, rgaS5EventInput(t))
	raw, err := EncodeRecipeCaptureEvent(e)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	var required [][]string
	var walk func(map[string]any, []string)
	walk = func(node map[string]any, prefix []string) {
		for key, value := range node {
			path := append(append([]string{}, prefix...), key)
			required = append(required, path)
			if nested, ok := value.(map[string]any); ok {
				walk(nested, path)
			}
		}
	}
	walk(object, nil)
	for _, path := range required {
		for _, mode := range []string{"missing", "null"} {
			t.Run(strings.Join(path, ".")+"/"+mode, func(t *testing.T) {
				var mutation map[string]any
				if err := json.Unmarshal(raw, &mutation); err != nil {
					t.Fatal(err)
				}
				node := mutation
				for _, key := range path[:len(path)-1] {
					node = node[key].(map[string]any)
				}
				key := path[len(path)-1]
				if mode == "missing" {
					delete(node, key)
				} else {
					node[key] = nil
				}
				bad, err := json.Marshal(mutation)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := DecodeRecipeCaptureEvent(bad); err == nil {
					t.Fatal("strict decoder accepted a missing/null required field")
				}
			})
		}
	}
	for key := range object["observations"].([]any)[0].(map[string]any) {
		for _, mode := range []string{"missing", "null"} {
			t.Run("observation."+key+"/"+mode, func(t *testing.T) {
				var mutation map[string]any
				if err := json.Unmarshal(raw, &mutation); err != nil {
					t.Fatal(err)
				}
				observation := mutation["observations"].([]any)[0].(map[string]any)
				if mode == "missing" {
					delete(observation, key)
				} else {
					observation[key] = nil
				}
				bad, err := json.Marshal(mutation)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := DecodeRecipeCaptureEvent(bad); err == nil {
					t.Fatal("strict decoder accepted a missing/null observation field")
				}
			})
		}
	}
	for name, bad := range map[string][]byte{
		"unknown-field":      bytes.Replace(raw, []byte(`"schema_version": 1`), []byte(`"schema_version": 1, "nonce": ""`), 1),
		"unknown-version":    bytes.Replace(raw, []byte(`"schema_version": 1`), []byte(`"schema_version": 2`), 1),
		"fractional-integer": bytes.Replace(raw, []byte(`"schema_version": 1`), []byte(`"schema_version": 1.0`), 1),
		"duplicate":          bytes.Replace(raw, []byte(`"feature": "s3"`), []byte(`"feature": "s3", "feature": "s3"`), 1),
		"trailing":           append(append([]byte{}, raw...), []byte("{}")...),
		"surrogate":          bytes.Replace(raw, []byte(`"feature": "s3"`), []byte(`"feature": "\ud800"`), 1),
		"utf8":               bytes.Replace(raw, []byte("s3"), []byte{0xff}, 1),
		"observation-field":  bytes.Replace(raw, []byte(`"ordinal": 1`), []byte(`"ordinal": 1, "source": ""`), 1),
		"observation-null":   bytes.Replace(raw, []byte(`"old_path": ""`), []byte(`"old_path": null`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if bytes.Equal(raw, bad) {
				t.Fatal("wire mutation did not change the input")
			}
			if _, err := DecodeRecipeCaptureEvent(bad); err == nil {
				t.Fatal("the real strict decoder accepted its wrong-input mutation")
			}
		})
	}
}

func TestRGAS5CaptureEventEncoderDoesNotRepair(t *testing.T) {
	event, _, _ := rgaS5EventPair(t, rgaS5EventInput(t))
	for _, mutate := range []func(*RecipeCaptureEvent){
		func(e *RecipeCaptureEvent) { e.Capture.Mode = "invented" },
		func(e *RecipeCaptureEvent) { e.Capture.Pathspecs = []string{"z", "a"} },
		func(e *RecipeCaptureEvent) { e.Capture.ClaimIDs = []string{"same", "same"} },
		func(e *RecipeCaptureEvent) {
			e.Capture.Mode = patchobs.CaptureModeNoCapture
			e.Capture.Pathspecs = []string{"a"}
		},
		func(e *RecipeCaptureEvent) {
			e.RecipePresent = false
			e.RecipeSHA256 = ""
			e.Event.RecipeRegenerated = true
		},
		func(e *RecipeCaptureEvent) { e.Reference.Kind = patchobs.ReferenceKindUnavailable },
		func(e *RecipeCaptureEvent) { e.Observations = nil },
	} {
		bad := rgaS5CloneEvent(event)
		mutate(&bad)
		if _, err := EncodeRecipeCaptureEvent(bad); err == nil {
			t.Fatal("encoder repaired contradictory inputs")
		}
		raw, err := json.Marshal(bad)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeRecipeCaptureEvent(raw); err == nil {
			t.Fatal("decoder repaired the same contradictory input")
		}
	}
}

func TestRGAS5CaptureEventShapeAndPairMutations(t *testing.T) {
	in := rgaS5EventInput(t)
	e, c, bindings := rgaS5EventPair(t, in)
	for name, mutate := range map[string]func(*RecipeCaptureEvent){
		"owner": func(e *RecipeCaptureEvent) { e.Feature = "other" },
		"patch-presence": func(e *RecipeCaptureEvent) {
			e.PatchPresent = false
			e.PatchSHA256 = ""
			e.Observations = []RecipeCaptureObservation{}
		},
		"recipe-presence": func(e *RecipeCaptureEvent) { e.RecipePresent = false; e.RecipeSHA256 = "" },
		"patch-hash":      func(e *RecipeCaptureEvent) { e.PatchSHA256 = strings.Repeat("b", 64) },
		"recipe-hash":     func(e *RecipeCaptureEvent) { e.RecipeSHA256 = strings.Repeat("b", 64) },
		"capture-mode":    func(e *RecipeCaptureEvent) { e.Capture.Mode = patchobs.CaptureModeStagedIndex },
		"pathspec":        func(e *RecipeCaptureEvent) { e.Capture.Pathspecs = []string{"a.txt"} },
		"claim":           func(e *RecipeCaptureEvent) { e.Capture.ClaimIDs = []string{"claim"} },
		"reference":       func(e *RecipeCaptureEvent) { e.Reference.Commit = strings.Repeat("b", 40) },
		"preimage-set":    func(e *RecipeCaptureEvent) { e.Reference.PreimageSetSHA256 = strings.Repeat("b", 64) },
		"pair":            func(e *RecipeCaptureEvent) { e.CoverageSHA256 = strings.Repeat("b", 64) },
		"ordinal":         func(e *RecipeCaptureEvent) { e.Observations[0].Ordinal = 2 },
		"fragment":        func(e *RecipeCaptureEvent) { e.Observations[0].PatchFragmentSHA256 = strings.Repeat("b", 64) },
		"observed":        func(e *RecipeCaptureEvent) { e.Observations[0].PreimageObserved = false },
		"observed-extant-absence": func(e *RecipeCaptureEvent) {
			e.Observations[0].PreimagePresent = false
			e.Observations[0].PreimageSHA256 = ""
			e.Observations[0].OldMode = ""
		},
		"mode":               func(e *RecipeCaptureEvent) { e.Observations[0].NewMode = "100755" },
		"axis":               func(e *RecipeCaptureEvent) { e.Observations[0].ContentKind = "unknown" },
		"stale-marker-event": func(e *RecipeCaptureEvent) { e.Event.StaleMarkerPresent = true },
	} {
		t.Run(name, func(t *testing.T) {
			bad := rgaS5CloneEvent(e)
			mutate(&bad)
			if err := ValidateRecipeCaptureEventPair(bad, c, bindings); err == nil {
				t.Fatal("pair validator accepted changed evidence")
			}
			if err := ValidateRecipeCaptureEventSource(bad, c, in); err == nil {
				t.Fatal("source validator accepted changed evidence")
			}
		})
	}
	for _, mutation := range []RecipeCaptureBindings{
		{RepoRoot: bindings.RepoRoot, Feature: "other", Patch: bindings.Patch, Recipe: bindings.Recipe},
		{RepoRoot: bindings.RepoRoot, Feature: bindings.Feature, Patch: CoverageArtifact{}, Recipe: bindings.Recipe},
		{RepoRoot: bindings.RepoRoot, Feature: bindings.Feature, Patch: bindings.Patch, Recipe: CoverageArtifact{ReadError: errors.New("injected EIO")}},
		{RepoRoot: bindings.RepoRoot, Feature: bindings.Feature, Patch: bindings.Patch, Recipe: CoverageArtifact{Present: true, Bytes: []byte("{}")}},
	} {
		if err := ValidateRecipeCaptureEventPair(e, c, mutation); err == nil {
			t.Fatal("stored flags or hashes overrode independently supplied artifact reads")
		}
	}
	whitespaceC := append(append([]byte{}, c...), '\n')
	if err := ValidateRecipeCaptureEventPair(e, whitespaceC, bindings); err == nil {
		t.Fatal("pair validator hashed reserialized C rather than its actual bytes")
	}
}

func TestRGAS5CaptureEventEffectiveParentExclusions(t *testing.T) {
	obs := rgaS3Observe(t, rgaS3AddPatch, rgaS3Image{post: "new\n"})
	obs.ParentCreatedPaths = []string{"./a.txt", "folder/../a.txt", "outside-patch.txt"}
	in := rgaS3Inputs(t, obs, rgaS3Write("a.txt", "", "new\n", true))
	e, c, bindings := rgaS5EventPair(t, in)
	if !slices.Equal(e.ParentCreatedPaths, []string{"a.txt", "outside-patch.txt"}) {
		t.Fatalf("effective exclusions were not normalized/preserved: %v", e.ParentCreatedPaths)
	}
	if !slices.Equal(obs.ParentCreatedPaths, []string{"./a.txt", "folder/../a.txt", "outside-patch.txt"}) {
		t.Fatal("builder mutated the captured discovery input")
	}
	dropped := rgaS5CloneEvent(e)
	dropped.ParentCreatedPaths = []string{}
	if err := ValidateRecipeCaptureEventPair(dropped, c, bindings); err == nil {
		t.Fatal("pair validator inferred the dropped exclusion from coverage reasons")
	}
	coverage, err := DecodeRecipeCoverage(c)
	if err != nil {
		t.Fatal(err)
	}
	coverage.Effects[0].ReasonCodes = []string{}
	coverage.Effects[0].Disposition = "represented"
	coverage.CoverageStatus = CoverageComplete
	coverage.CrossBaseStatus = CrossBaseReferenceTreeOnly
	mutatedC, err := EncodeRecipeCoverage(coverage)
	if err != nil {
		t.Fatal(err)
	}
	changed := rgaS5CloneEvent(e)
	changed.CoverageSHA256 = CoverageSHA256(mutatedC)
	if err := ValidateRecipeCaptureEventPair(changed, mutatedC, bindings); err == nil {
		t.Fatal("refreshed pairing hash hid a coverage-side exclusion deletion")
	}
	for _, paths := range [][]string{nil, {"a.txt", "a.txt"}, {"z.txt", "a.txt"}, {"./a.txt"}, {"folder/../a.txt"}, {"../a.txt"}, {"/absolute"}, {""}} {
		bad := rgaS5CloneEvent(e)
		bad.ParentCreatedPaths = paths
		if _, err := EncodeRecipeCaptureEvent(bad); err == nil {
			t.Fatalf("encoder repaired invalid parent paths: %v", paths)
		}
		raw, _ := json.Marshal(bad)
		if _, err := DecodeRecipeCaptureEvent(raw); err == nil {
			t.Fatalf("decoder repaired invalid parent paths: %v", paths)
		}
	}
	obs.ParentCreatedPaths = nil
	op := rgaS3Write("./a.txt", "", "new\n", true)
	op.CreatedBy = "parent"
	augmented, augmentedC, augmentedBindings := rgaS5EventPair(t, rgaS3Inputs(t, obs, op))
	if !slices.Equal(augmented.ParentCreatedPaths, []string{"a.txt"}) {
		t.Fatal("final-recipe created_by was not added to effective exclusions")
	}
	augmented.ParentCreatedPaths = []string{}
	if err := ValidateRecipeCaptureEventPair(augmented, augmentedC, augmentedBindings); err == nil {
		t.Fatal("recipe-derived exclusion was omitted from the independent evidence")
	}
}

func TestRGAS5CaptureEventPatchReadableExistence(t *testing.T) {
	obs := rgaS3Observe(t, "")
	obs.PatchPresent, obs.PatchBytes, obs.PatchSHA256 = false, nil, ""
	in := RecipeCoverageInput{Observation: obs}
	e, c, bindings := rgaS5EventPair(t, in)
	bindings.Patch.ReadError = errors.New("injected patch EIO")
	if err := ValidateRecipeCaptureEventPair(e, c, bindings); err != nil {
		t.Fatalf("truthfully unreadable and absent patch bindings must agree: %v", err)
	}
	bindings.Patch = CoverageArtifact{Present: true, Bytes: []byte{}}
	if err := ValidateRecipeCaptureEventPair(e, c, bindings); err == nil {
		t.Fatal("a restored readable empty patch escaped the false-to-true binding check")
	}
	readable := rgaS5EventInput(t)
	e, c, bindings = rgaS5EventPair(t, readable)
	bindings.Patch = CoverageArtifact{ReadError: errors.New("injected patch EIO")}
	if err := ValidateRecipeCaptureEventPair(e, c, bindings); err == nil {
		t.Fatal("a patch readability loss escaped the true-to-false binding check")
	}
}

func TestRGAS5CaptureEventIncompleteObservationsAndEventFacts(t *testing.T) {
	binary := "diff --git a/a.txt b/a.txt\nindex 1111111..2222222 100644\nBinary files a/a.txt and b/a.txt differ\n"
	for _, obs := range []patchobs.Observation{
		rgaS3Observe(t, rgaS3ModifyPatch),
		rgaS3Observe(t, binary, rgaS3Image{pre: "old\x00", post: "new\x00"}),
		rgaS3Observe(t, ""),
		rgaS3Observe(t, "not a patch"),
	} {
		obs.Reference.Kind, obs.Reference.Commit = patchobs.ReferenceKindUnavailable, ""
		obs.Capture = patchobs.CaptureDescriptor{Mode: patchobs.CaptureModeNoCapture}
		in := RecipeCoverageInput{Observation: obs, Recipe: CoverageArtifact{ReadError: errors.New("injected unreadable recipe")}}
		e, rawC, bindings := rgaS5EventPair(t, in)
		if e.RecipePresent || e.RecipeSHA256 != "" {
			t.Fatal("unreadable recipe became a fabricated empty file")
		}
		bindings.Recipe = CoverageArtifact{Present: true, Bytes: []byte("restored")}
		if err := ValidateRecipeCaptureEventPair(e, rawC, bindings); err == nil {
			t.Fatal("false-to-readable transition escaped the real pair validator")
		}
		wire, err := EncodeRecipeCaptureEvent(e)
		if err != nil || bytes.Contains(wire, []byte("injected")) || bytes.Contains(wire, []byte(`old\u0000`)) {
			t.Fatal("I/O diagnostics or source bodies leaked into evidence")
		}
	}
	in := rgaS5EventInput(t)
	in.Observation.Producer = patchobs.ProducerEdit
	in.Recipe.Bytes = []byte(`{"feature":"s3","operations":[{"type":"write-file","path":"a.txt","content":"wrong"}]}`)
	in.Events = CoverageEvents{PatchRewritten: true, BoundArtifactEdited: true, StaleMarkerPresent: true}
	e, c, bindings := rgaS5EventPair(t, in)
	for _, mutation := range []func(*RecipeCaptureEvent){
		func(e *RecipeCaptureEvent) { e.Event.PatchRewritten = false },
		func(e *RecipeCaptureEvent) { e.Event.BoundArtifactEdited = false },
		func(e *RecipeCaptureEvent) { e.Event.RecipeRegenerated = true },
		func(e *RecipeCaptureEvent) { e.Event.StaleMarkerPresent = false },
	} {
		bad := rgaS5CloneEvent(e)
		mutation(&bad)
		if err := ValidateRecipeCaptureEventPair(bad, c, bindings); err == nil {
			t.Fatal("changed event facts escaped semantic-condition recomputation")
		}
	}
}

func TestRGAS5CaptureEventPureBoundaryMutations(t *testing.T) {
	for _, path := range []string{"internal/workflow/recipe_capture_event.go", "internal/workflow/recipe_capture_event_codec.go"} {
		src := rgaS0ReadRepoFile(t, path)
		if err := rgaS0CoveragePhaseSource(path, src); err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{
			src + "\nfunc plantedEventWriter() { PublishCoverage(nil, CoveragePublicationInput{}) }\n",
			src + "\nfunc plantedEventRead() { patchobs.Observe(patchobs.Input{}) }\n",
			src + "\nfunc plantedEventReplay() { ExecuteRecipe(nil, ApplyRecipe{}) }\n",
		} {
			if err := rgaS0CoveragePhaseSource(path, bad); err == nil {
				t.Fatal("new pure event surface admitted a producer, live read or execution")
			}
		}

		if err := rgaS0CoveragePhaseSource("internal/workflow/unregistered_event.go", src); err == nil {
			t.Fatal("new evidence policy escaped its designated pure files")
		}
	}

}

func TestRGAS5EvidenceOnlySourceBoundaryMutations(t *testing.T) {
	path := "internal/workflow/unregistered_event.go"
	positive := "package workflow\nfunc unrelated() { _ = \"unrelated.json\" }\n"
	if err := rgaS0CoveragePhaseSource(path, positive); err != nil {
		t.Fatal(err)
	}
	for name, source := range map[string]string{
		"only-event-symbol":      "package workflow\nfunc planted() { _ = RecipeCaptureEvent{} }\n",
		"only-event-helper":      "package workflow\nfunc planted() { recipeCaptureInput(input) }\n",
		"only-event-writer":      "package workflow\nfunc planted() { s.WriteArtifactAtomic(\"slug\", \"recipe-capture-event.json\", \"{}\") }\n",
		"escaped-event-writer":   `package workflow; func planted() { s.WriteArtifactAtomic("slug", "\x72ecipe-capture-event.json", "{}") }`,
		"split-event-writer":     `package workflow; func planted() { s.WriteArtifactAtomic("slug", "recipe-capture-"+"event.json", "{}") }`,
		"parenthesized-literals": `package workflow; func planted() { s.WriteArtifactAtomic("slug", ("recipe-" + ("capture-event." + "json")), "{}") }`,
		"escaped-full-path":      `package workflow; func planted() { s.WriteFeatureFile("slug", "artifacts/\x72ecipe-capture-event.json", "{}") }`,
		"constant-aliases":       `package workflow; const prefix = "recipe-"; const suffix = "capture-event.json"; func planted() { s.WriteArtifactAtomic("slug", prefix+suffix, "{}") }`,
	} {
		t.Run(name, func(t *testing.T) {
			if name == "only-event-writer" && strings.Contains(source, "Coverage") {
				t.Fatal("E-only control accidentally acquired an old coverage token")
			}
			if err := rgaS0CoveragePhaseSource(path, source); err == nil {
				t.Fatal("the same phase validator accepted an E-only mutation")
			}
		})
	}
	publisher := rgaS0ReadRepoFile(t, "internal/workflow/recipe_coverage_publish.go")
	if err := rgaS0CoveragePhaseSource(path, publisher); err == nil {
		t.Fatal("copying the publisher into an unregistered file evaded the phase guard")
	}
}
