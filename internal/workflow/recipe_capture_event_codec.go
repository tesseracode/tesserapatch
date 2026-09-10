package workflow

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/tesseracode/tesserapatch/internal/patchobs"
)

func DecodeRecipeCaptureEvent(raw []byte) (RecipeCaptureEvent, error) {
	var e RecipeCaptureEvent
	node, err := coverageJSONNode(raw)
	if err != nil {
		return e, fmt.Errorf("capture event: %w", err)
	}
	if err := coverageRequiredShape(node, reflect.TypeOf(e), "capture event"); err != nil {
		return e, err
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return RecipeCaptureEvent{}, err
	}
	if err := ValidateRecipeCaptureEventSchema(e); err != nil {
		return RecipeCaptureEvent{}, err
	}
	return e, nil
}

// EncodeRecipeCaptureEvent validates without repairing input and emits the
// declared field order, S3's escaping, two-space indentation and one final LF.
func EncodeRecipeCaptureEvent(e RecipeCaptureEvent) ([]byte, error) {
	if err := ValidateRecipeCaptureEventSchema(e); err != nil {
		return nil, err
	}
	raw, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func RecipeCaptureEventIdentity(e RecipeCaptureEvent) (string, error) {
	raw, err := EncodeRecipeCaptureEvent(e)
	if err != nil {
		return "", err
	}
	return CoverageSHA256(raw), nil
}

func ValidateRecipeCaptureEventSchema(e RecipeCaptureEvent) error {
	if err := coverageUTF8Value(reflect.ValueOf(e)); err != nil {
		return err
	}
	if e.SchemaVersion != 1 || e.Feature == "" {
		return fmt.Errorf("capture event: invalid schema version or feature")
	}
	for _, b := range []struct {
		present bool
		hash    string
	}{{e.PatchPresent, e.PatchSHA256}, {e.RecipePresent, e.RecipeSHA256}} {
		if (b.present && !coverageHash(b.hash)) || (!b.present && b.hash != "") {
			return fmt.Errorf("capture event: readable presence and hash contradict")
		}
	}
	switch e.Reference.Kind {
	case patchobs.ReferenceKindCommit:
		if !fullRecipeBaseCommit(e.Reference.Commit) {
			return fmt.Errorf("capture event: invalid reference commit")
		}
	case patchobs.ReferenceKindUnavailable, patchobs.ReferenceKindIndexSnapshot:
		if e.Reference.Commit != "" {
			return fmt.Errorf("capture event: non-durable reference carries a commit")
		}
	default:
		return fmt.Errorf("capture event: unknown reference kind")
	}
	if !coverageHash(e.Reference.PreimageSetSHA256) || !coverageHash(e.CoverageSHA256) {
		return fmt.Errorf("capture event: invalid preimage-set or coverage pairing hash")
	}
	if !patchobs.KnownCaptureMode(e.Capture.Mode) ||
		!coverageSorted(e.Capture.Pathspecs) || !coverageSorted(e.Capture.ClaimIDs) {
		return fmt.Errorf("capture event: invalid capture descriptor")
	}
	if e.Capture.Mode == patchobs.CaptureModeNoCapture &&
		(len(e.Capture.Pathspecs) != 0 || len(e.Capture.ClaimIDs) != 0) {
		return fmt.Errorf("capture event: no-capture carries selectors")
	}
	if e.Event.RecipeRegenerated && !e.RecipePresent {
		return fmt.Errorf("capture event: recipe regeneration without a readable recipe")
	}
	if !coverageSorted(e.ParentCreatedPaths) {
		return fmt.Errorf("capture event: parent exclusions must be non-null, sorted and unique")
	}
	for _, path := range e.ParentCreatedPaths {
		if coverageEffectPathUnsafe(CoverageEffect{Path: path}) {
			return fmt.Errorf("capture event: parent exclusion is not a canonical repository-relative path")
		}
	}
	if e.Observations == nil || (!e.PatchPresent && len(e.Observations) != 0) {
		return fmt.Errorf("capture event: null observations or absent patch with observations")
	}
	effects := make([]patchobs.EffectObservation, 0, len(e.Observations))
	seen := make(map[string]bool)
	for i, o := range e.Observations {
		if o.Ordinal != i+1 || seen[o.Path] {
			return fmt.Errorf("capture event: observations must be unique in gapless ordinal order")
		}
		seen[o.Path] = true
		effect := coverageEffect(o.patchEffect())
		// Reuse S3's observation grammar without claiming an assignment or
		// supplying synthetic source bytes to its simulation validator.
		effect.ReasonCodes = coverageLocalReasons(effect, false, false, coverageEffectPathUnsafe(effect))
		var err error
		effect.Disposition, err = coverageDisposition(effect.ReasonCodes)
		if err != nil {
			return err
		}
		effect.EffectSHA256, err = CoverageEffectSHA256(effect)
		if err != nil {
			return err
		}
		if err := coverageValidateEffect(effect); err != nil {
			return fmt.Errorf("capture event: observation %d: %w", i+1, err)
		}
		effects = append(effects, patchobs.EffectObservation{Effect: o.patchEffect()})
	}
	if patchobs.PreimageSetDigest(effects) != e.Reference.PreimageSetSHA256 {
		return fmt.Errorf("capture event: preimage-set digest contradicts observation projection")
	}
	return nil
}
