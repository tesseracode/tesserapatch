package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// CoverageSHA256 binds exact bytes, including noncanonical or undecodable JSON.
func CoverageSHA256(raw []byte) string { return store.SHA256HexString(string(raw)) }

// CoverageEffectSHA256 hashes only the exhaustive D3 descriptor, in its
// declared field order as compact encoding/json JSON (no final newline).
func CoverageEffectSHA256(e CoverageEffect) (string, error) {
	descriptor := struct {
		Ordinal             int                 `json:"ordinal"`
		ChangeKind          gitutil.ChangeKind  `json:"change_kind"`
		ContentKind         gitutil.ContentKind `json:"content_kind"`
		ObjectKind          gitutil.ObjectKind  `json:"object_kind"`
		Path                string              `json:"path"`
		OldPath             string              `json:"old_path"`
		OldMode             string              `json:"old_mode"`
		NewMode             string              `json:"new_mode"`
		PreimageObserved    bool                `json:"preimage_observed"`
		PreimagePresent     bool                `json:"preimage_present"`
		PreimageSHA256      string              `json:"preimage_sha256"`
		PostimageObserved   bool                `json:"postimage_observed"`
		PostimagePresent    bool                `json:"postimage_present"`
		PostimageSHA256     string              `json:"postimage_sha256"`
		PatchFragmentSHA256 string              `json:"patch_fragment_sha256"`
	}{
		e.Ordinal, e.ChangeKind, e.ContentKind, e.ObjectKind, e.Path, e.OldPath,
		e.OldMode, e.NewMode, e.PreimageObserved, e.PreimagePresent, e.PreimageSHA256,
		e.PostimageObserved, e.PostimagePresent, e.PostimageSHA256, e.PatchFragmentSHA256,
	}
	if err := coverageUTF8Value(reflect.ValueOf(descriptor)); err != nil {
		return "", err
	}
	raw, err := json.Marshal(descriptor)
	if err != nil {
		return "", err
	}
	return CoverageSHA256(raw), nil
}

// DecodeRecipeCoverage rejects malformed or internally contradictory records.
// Call ValidateRecipeCoverage with captured inputs before trusting any binding.
func DecodeRecipeCoverage(raw []byte) (RecipeCoverage, error) {
	var c RecipeCoverage
	node, err := coverageJSONNode(raw)
	if err != nil {
		return c, err
	}
	if err := coverageRequiredShape(node, reflect.TypeOf(c), "coverage"); err != nil {
		return c, err
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return RecipeCoverage{}, err
	}
	if err := ValidateRecipeCoverageSchema(c); err != nil {
		return RecipeCoverage{}, err
	}
	return c, nil
}

// EncodeRecipeCoverage never sorts or repairs contradictory input. Builders
// establish ordering; this encoder emits fixed fields, two spaces and one LF.
func EncodeRecipeCoverage(c RecipeCoverage) ([]byte, error) {
	if err := ValidateRecipeCoverageSchema(c); err != nil {
		return nil, err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func coverageJSONNode(raw []byte) (store.CanonNode, error) {
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return store.CanonNode{}, fmt.Errorf("coverage: invalid UTF-8 or JSON")
	}
	// encoding/json replaces lone UTF-16 surrogates with U+FFFD. Refuse
	// those escapes before decoding, just as we refuse invalid raw UTF-8.
	for i := 0; i < len(raw); i++ {
		if raw[i] != '"' {
			continue
		}
		for i++; raw[i] != '"'; i++ {
			if raw[i] != '\\' {
				continue
			}
			i++
			if raw[i] != 'u' {
				continue
			}
			n, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
			if err != nil {
				return store.CanonNode{}, err
			}
			i += 4
			if n >= 0xdc00 && n <= 0xdfff {
				return store.CanonNode{}, fmt.Errorf("coverage: unpaired UTF-16 surrogate")
			}
			if n >= 0xd800 && n <= 0xdbff {
				if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
					return store.CanonNode{}, fmt.Errorf("coverage: unpaired UTF-16 surrogate")
				}
				low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
				if err != nil || low < 0xdc00 || low > 0xdfff {
					return store.CanonNode{}, fmt.Errorf("coverage: unpaired UTF-16 surrogate")
				}
				i += 6
			}
		}
	}
	var node store.CanonNode
	if err := node.UnmarshalJSON(raw); err != nil {
		return store.CanonNode{}, err
	}
	return node, nil
}

func coverageRequiredShape(n store.CanonNode, typ reflect.Type, at string) error {
	switch typ.Kind() {
	case reflect.Struct:
		if n.Kind != store.CanonKindObject || len(n.Object) != typ.NumField() {
			return fmt.Errorf("%s: every declared field is required, no others permitted", at)
		}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			name := f.Tag.Get("json")
			value, ok := n.Field(name)
			if !ok {
				return fmt.Errorf("%s: missing field %s", at, name)
			}
			if err := coverageRequiredShape(value, f.Type, at+"."+name); err != nil {
				return err
			}
		}
	case reflect.Slice:
		if n.Kind != store.CanonKindArray {
			return fmt.Errorf("%s: expected non-null array", at)
		}
		for _, item := range n.Array {
			if err := coverageRequiredShape(item, typ.Elem(), at+"[]"); err != nil {
				return err
			}
		}
	case reflect.String:
		if n.Kind != store.CanonKindString {
			return fmt.Errorf("%s: expected string", at)
		}
	case reflect.Bool:
		if n.Kind != store.CanonKindBool {
			return fmt.Errorf("%s: expected boolean", at)
		}
	case reflect.Int:
		if n.Kind != store.CanonKindUint {
			return fmt.Errorf("%s: expected nonnegative integer", at)
		}
	default:
		return fmt.Errorf("%s: unsupported schema type", at)
	}
	return nil
}

func coverageUTF8Value(v reflect.Value) error {
	switch v.Kind() {
	case reflect.String:
		if !utf8.ValidString(v.String()) {
			return fmt.Errorf("coverage: string cannot be encoded without changing bytes")
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if err := coverageUTF8Value(v.Field(i)); err != nil {
				return err
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if err := coverageUTF8Value(v.Index(i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func coverageHash(s string) bool {
	return len(s) == 64 && strings.Trim(s, "0123456789abcdef") == ""
}

func coverageSorted(values []string) bool {
	if values == nil {
		return false
	}
	for i := 1; i < len(values); i++ {
		if values[i-1] >= values[i] {
			return false
		}
	}
	return true
}

func coverageReasonArray(values, allowed []string) error {
	if !coverageSorted(values) {
		return fmt.Errorf("coverage: reasons must be non-null, sorted and unique")
	}
	for _, code := range values {
		if !slices.Contains(allowed, code) {
			return fmt.Errorf("coverage: unknown or misallocated reason %q", code)
		}
	}
	return nil
}

func coverageDisposition(reasons []string) (string, error) {
	if len(reasons) == 0 {
		return "represented", nil
	}
	if slices.Contains(reasons, "operation-missing") {
		if len(reasons) != 1 {
			return "", fmt.Errorf("coverage: operation-missing cannot accompany another effect reason")
		}
		return "mismatch", nil
	}
	if slices.Contains(reasons, "preimage-unavailable") || slices.Contains(reasons, "postimage-unavailable") {
		return "ambiguous", nil
	}
	return "unsupported", nil
}

// ValidateRecipeCoverageSchema checks all properties decidable from the wire.
// Conditions requiring bytes, operations or event facts are additionally
// recomputed by ValidateRecipeCoverage; their stored labels are not evidence.
func ValidateRecipeCoverageSchema(c RecipeCoverage) error {
	if err := coverageUTF8Value(reflect.ValueOf(c)); err != nil {
		return err
	}
	if c.SchemaVersion != 1 || c.Feature == "" || !patchobs.KnownProducer(c.Producer) {
		return fmt.Errorf("coverage: invalid version, feature or producer")
	}
	switch c.Reference.Kind {
	case patchobs.ReferenceKindCommit:
		if !fullRecipeBaseCommit(c.Reference.Commit) {
			return fmt.Errorf("coverage: invalid reference commit")
		}
	case patchobs.ReferenceKindUnavailable, patchobs.ReferenceKindIndexSnapshot:
		if c.Reference.Commit != "" {
			return fmt.Errorf("coverage: non-durable reference carries a commit")
		}
	default:
		return fmt.Errorf("coverage: unknown reference kind")
	}
	if !coverageHash(c.Reference.PreimageSetSHA256) {
		return fmt.Errorf("coverage: invalid preimage-set hash")
	}
	if !patchobs.KnownCaptureMode(c.Capture.Mode) ||
		!coverageSorted(c.Capture.Pathspecs) || !coverageSorted(c.Capture.ClaimIDs) {
		return fmt.Errorf("coverage: invalid capture descriptor")
	}
	if c.Capture.Mode == patchobs.CaptureModeNoCapture &&
		(len(c.Capture.Pathspecs) != 0 || len(c.Capture.ClaimIDs) != 0) {
		return fmt.Errorf("coverage: no-capture cannot carry capture selectors")
	}
	for _, binding := range []struct {
		present bool
		hash    string
	}{
		{c.PatchPresent, c.PatchSHA256}, {c.RecipePresent, c.RecipeSHA256},
	} {
		if (binding.present && !coverageHash(binding.hash)) || (!binding.present && binding.hash != "") {
			return fmt.Errorf("coverage: readable presence and raw hash contradict")
		}
	}
	if c.Effects == nil || (c.RecipeDecodable && !c.RecipePresent) {
		return fmt.Errorf("coverage: null effects or impossible recipe decodability")
	}
	if err := coverageReasonArray(c.Reasons, coverageRecordReasons); err != nil {
		return err
	}
	has := func(code string) bool { return slices.Contains(c.Reasons, code) }
	for code, condition := range map[string]bool{
		"canonical-patch-missing": !c.PatchPresent,
		"reference-not-durable":   c.Reference.Kind != patchobs.ReferenceKindCommit,
		"recipe-undecodable":      c.RecipePresent && !c.RecipeDecodable,
	} {
		if has(code) != condition {
			return fmt.Errorf("coverage: reason %s contradicts its condition", code)
		}
	}
	empty, unparseable := has("canonical-patch-empty"), has("canonical-patch-unparseable")
	if empty && unparseable || (!c.PatchPresent && (empty || unparseable || len(c.Effects) != 0)) ||
		(c.PatchPresent && ((len(c.Effects) == 0) != (empty || unparseable))) {
		return fmt.Errorf("coverage: patch state and effect set contradict")
	}
	if has("producer-patch-rewrite") != has("recipe-not-regenerated") ||
		(!c.RecipePresent && has("producer-patch-rewrite")) ||
		(!c.RecipeDecodable && (has("recipe-owner-mismatch") || has("operation-surplus") || has("simulation-mismatch"))) ||
		(len(c.Effects) == 0 && has("simulation-mismatch")) {
		return fmt.Errorf("coverage: impossible recipe/event reason combination")
	}
	complete := c.PatchPresent && len(c.Effects) > 0 &&
		c.Reference.Kind == patchobs.ReferenceKindCommit && c.RecipePresent &&
		c.RecipeDecodable && len(c.Reasons) == 0
	assigned := map[int]bool{}
	paths := map[string]bool{}
	bindingEffects := make([]patchobs.EffectObservation, 0, len(c.Effects))
	for i, e := range c.Effects {
		if e.Ordinal != i+1 || paths[e.Path] {
			return fmt.Errorf("coverage: effects must be unique in normalized ordinal order")
		}
		paths[e.Path] = true
		if err := coverageValidateEffect(e); err != nil {
			return fmt.Errorf("coverage effect %d: %w", i+1, err)
		}
		for _, index := range e.OperationIndexes {
			if !c.RecipeDecodable || assigned[index] {
				return fmt.Errorf("coverage: impossible or duplicate operation assignment %d", index)
			}
			assigned[index] = true
		}
		complete = complete && e.Disposition == "represented" &&
			e.PreimageObserved && e.PostimageObserved &&
			e.ContentKind != gitutil.ContentKindUnknown && e.ObjectKind != gitutil.ObjectKindUnknown
		bindingEffects = append(bindingEffects, patchobs.EffectObservation{Effect: gitutil.PatchEffect{
			Ordinal: e.Ordinal, ChangeKind: e.ChangeKind, ContentKind: e.ContentKind, ObjectKind: e.ObjectKind,
			Path: e.Path, OldPath: e.OldPath, OldMode: e.OldMode, NewMode: e.NewMode,
			PreimageObserved: e.PreimageObserved, PreimagePresent: e.PreimagePresent, PreimageSHA256: e.PreimageSHA256,
			PostimageObserved: e.PostimageObserved, PostimagePresent: e.PostimagePresent, PostimageSHA256: e.PostimageSHA256,
		}})
	}
	if patchobs.PreimageSetDigest(bindingEffects) != c.Reference.PreimageSetSHA256 {
		return fmt.Errorf("coverage: preimage-set hash contradicts the descriptors")
	}
	want := CoverageIncomplete
	if complete {
		want = CoverageComplete
	}
	if c.CoverageStatus != want {
		return fmt.Errorf("coverage: status contradicts the completeness iff")
	}
	if !complete {
		if c.CrossBaseStatus != CrossBaseUnsupported {
			return fmt.Errorf("coverage: incomplete record has cross-base scope")
		}
	} else {
		existing := false
		for _, e := range c.Effects {
			existing = existing || e.PreimagePresent
		}
		wantCross := CrossBaseReferenceTreeOnly
		if existing {
			wantCross = CrossBaseConsumerDerivationRequired
		}
		if c.CrossBaseStatus != wantCross {
			return fmt.Errorf("coverage: cross-base scope contradicts effect existence")
		}
	}
	return nil
}

func coverageValidateEffect(e CoverageEffect) error {
	switch e.ChangeKind {
	case gitutil.ChangeKindAdd, gitutil.ChangeKindModify, gitutil.ChangeKindDelete, gitutil.ChangeKindRename, gitutil.ChangeKindCopy:
	default:
		return fmt.Errorf("unknown change kind")
	}
	if e.Path == "" || ((e.ChangeKind == gitutil.ChangeKindRename || e.ChangeKind == gitutil.ChangeKindCopy) != (e.OldPath != "")) {
		return fmt.Errorf("path/old_path contradict change kind")
	}
	switch e.ContentKind {
	case gitutil.ContentKindText, gitutil.ContentKindBinary, gitutil.ContentKindNone, gitutil.ContentKindUnknown:
	default:
		return fmt.Errorf("unknown content kind")
	}
	preExtant, postExtant := (gitutil.PatchEffect{ChangeKind: e.ChangeKind}).ExtantSides()
	for _, side := range []struct {
		observed, present, extant bool
		mode, hash                string
	}{
		{e.PreimageObserved, e.PreimagePresent, preExtant, e.OldMode, e.PreimageSHA256},
		{e.PostimageObserved, e.PostimagePresent, postExtant, e.NewMode, e.PostimageSHA256},
	} {
		if !side.observed {
			if side.present || side.mode != "" || side.hash != "" {
				return fmt.Errorf("unobserved side carries object evidence")
			}
		} else if side.present != side.extant {
			return fmt.Errorf("observed presence contradicts change kind")
		} else if side.present {
			if !coverageHash(side.hash) || gitutil.ObjectKindForMode(side.mode) == gitutil.ObjectKindUnknown {
				return fmt.Errorf("present side lacks an exact mode/hash")
			}
		} else if side.mode != "" || side.hash != "" {
			return fmt.Errorf("absent side carries object evidence")
		}
	}
	object := gitutil.ObjectKindUnknown
	if e.PostimageObserved && e.PostimagePresent {
		object = gitutil.ObjectKindForMode(e.NewMode)
	} else if !postExtant && e.PreimageObserved && e.PreimagePresent {
		object = gitutil.ObjectKindForMode(e.OldMode)
	}
	if e.ObjectKind != object || (e.ContentKind == gitutil.ContentKindNone) != (object == gitutil.ObjectKindGitlink) {
		return fmt.Errorf("object/content kind contradict observed modes")
	}
	allExtant := (!preExtant || e.PreimageObserved) && (!postExtant || e.PostimageObserved)
	if (e.ContentKind == gitutil.ContentKindText && !allExtant) ||
		(e.ContentKind == gitutil.ContentKindUnknown && allExtant) {
		return fmt.Errorf("content kind contradicts extant-side observation")
	}
	if !coverageHash(e.PatchFragmentSHA256) || !coverageHash(e.EffectSHA256) {
		return fmt.Errorf("invalid fragment/effect hash")
	}
	digest, err := CoverageEffectSHA256(e)
	if err != nil || digest != e.EffectSHA256 {
		return fmt.Errorf("effect descriptor hash mismatch")
	}
	if e.OperationIndexes == nil {
		return fmt.Errorf("null operation indexes")
	}
	for i, n := range e.OperationIndexes {
		if n < 1 || (i > 0 && n <= e.OperationIndexes[i-1]) {
			return fmt.Errorf("operation indexes must be positive, sorted and unique")
		}
	}
	if e.ContextualHint != "none" && e.ContextualHint != "additive-text" {
		return fmt.Errorf("unknown contextual hint")
	}
	if err := coverageReasonArray(e.ReasonCodes, coverageEffectReasons); err != nil {
		return err
	}
	// These two predicates require supplied recipe/parent facts. Structural
	// validation preserves their assertions; input-aware validation proves them.
	parent := slices.Contains(e.ReasonCodes, "parent-created-target-unsupported")
	reclass := slices.Contains(e.ReasonCodes, "operation-not-reclassifiable")
	unsafe := slices.Contains(e.ReasonCodes, "path-unsafe")
	if coverageEffectPathUnsafe(e) != unsafe {
		return fmt.Errorf("path-unsafe does not match effect path safety")
	}
	if reclass && len(e.OperationIndexes) == 0 {
		return fmt.Errorf("unreclassifiable operation was not assigned")
	}
	if parent && e.PreimageObserved && e.PreimagePresent {
		return fmt.Errorf("parent body claimed despite an established preimage")
	}
	expected := coverageLocalReasons(e, parent, reclass, unsafe)
	if !slices.Equal(expected, e.ReasonCodes) {
		return fmt.Errorf("effect reasons are not the exact applicable set")
	}
	disposition, err := coverageDisposition(e.ReasonCodes)
	if err != nil {
		return err
	}
	if disposition != e.Disposition {
		return fmt.Errorf("disposition contradicts reason codes")
	}
	return nil
}

// decodeCoverageRecipe strengthens the existing recipe decoder locally, without
// changing the legacy producer/consumer contract outside this pure core.
func decodeCoverageRecipe(raw []byte) (ApplyRecipe, error) {
	node, err := coverageJSONNode(raw)
	if err != nil {
		return ApplyRecipe{}, err
	}
	if node.Kind != store.CanonKindObject || len(node.Object) != 2 {
		return ApplyRecipe{}, fmt.Errorf("recipe requires exactly feature and operations")
	}
	feature, f := node.Field("feature")
	ops, o := node.Field("operations")
	if !f || !o || feature.Kind != store.CanonKindString || ops.Kind != store.CanonKindArray {
		return ApplyRecipe{}, fmt.Errorf("recipe requires feature and non-null operations")
	}
	for _, op := range ops.Array {
		if op.Kind != store.CanonKindObject {
			return ApplyRecipe{}, fmt.Errorf("recipe operation is not an object")
		}
		for _, field := range op.Object {
			if !slices.Contains([]string{"type", "path", "content", "search", "replace", "created_by", "preimage_hash"}, field.Key) ||
				field.Value.Kind != store.CanonKindString {
				return ApplyRecipe{}, fmt.Errorf("invalid recipe operation field %q", field.Key)
			}
		}
		kind, k := op.Field("type")
		path, p := op.Field("path")
		if !k || !p || kind.Kind != store.CanonKindString || path.Kind != store.CanonKindString {
			return ApplyRecipe{}, fmt.Errorf("recipe operation requires type and path")
		}
		required := []string{}
		switch kind.Str {
		case "write-file", "append-file":
			required = []string{"content"}
		case "replace-in-file":
			required = []string{"search", "replace"}
		}
		for _, field := range required {
			if _, ok := op.Field(field); !ok {
				return ApplyRecipe{}, fmt.Errorf("recipe %s requires %s", kind.Str, field)
			}
		}
		if hash, ok := op.Field("preimage_hash"); ok &&
			hash.Str != "" && !(strings.HasPrefix(hash.Str, "sha256:") && coverageHash(strings.TrimPrefix(hash.Str, "sha256:"))) {
			return ApplyRecipe{}, fmt.Errorf("recipe preimage_hash is invalid")
		}
	}
	return DecodeApplyRecipeStrict(raw)
}

func coverageEqual(a, b RecipeCoverage) bool {
	// Both operands have passed the strict validator; canonical bytes compare
	// the complete descriptors, assignments and reasons, not just file sets.
	x, err := EncodeRecipeCoverage(a)
	if err != nil {
		return false
	}
	y, err := EncodeRecipeCoverage(b)
	return err == nil && bytes.Equal(x, y)
}
