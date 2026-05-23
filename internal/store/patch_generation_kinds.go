package store

import "fmt"

const (
	PatchGenerationKindRecord        = "record"
	PatchGenerationKindReconcile     = "reconcile"
	PatchGenerationKindAmendRefresh  = "amend-refresh"
	PatchGenerationKindAmendFixup    = "amend-fixup"
	PatchGenerationKindImport        = "import"
	PatchGenerationKindManualMeta    = "manual-metadata"
	PatchGenerationIntentPlainRecord = "plain-record"
	PatchGenerationIntentRefresh     = "refresh"
	PatchGenerationIntentFixup       = "fixup"
)

// PatchGenerationClassification is the single write-path decision for whether
// a patch-byte capture appends a generation and which v1 kind it emits.
type PatchGenerationClassification struct {
	Kind   string
	Append bool
}

// IsWritablePatchGenerationKind returns true for v1 generation kinds that may
// be emitted by current writers. import/manual-metadata remain read-only.
func IsWritablePatchGenerationKind(kind string) bool {
	switch kind {
	case PatchGenerationKindRecord, PatchGenerationKindReconcile, PatchGenerationKindAmendRefresh, PatchGenerationKindAmendFixup:
		return true
	default:
		return false
	}
}

// ClassifyPlainRecordKind implements ADR-026 D1's hybrid rule for the backward-
// compatible `tpatch record <slug>` path: first patch-byte generation is a
// record; subsequent bytes-changing plain records are amend-refresh; same bytes
// append nothing.
func ClassifyPlainRecordKind(prior []PatchGeneration, patchSHA256 string) PatchGenerationClassification {
	c, _ := ClassifyPatchGenerationKind(prior, patchSHA256, PatchGenerationIntentPlainRecord)
	return c
}

// ClassifyPatchGenerationKind classifies all Wave γ patch-content write intents
// through one helper so record, refresh, and fixup cannot drift.
func ClassifyPatchGenerationKind(prior []PatchGeneration, patchSHA256, intent string) (PatchGenerationClassification, error) {
	if latest, ok := LatestPatchGeneration(PatchGenerationsManifest{Generations: prior}); ok && latest.PatchSHA256 == patchSHA256 {
		return PatchGenerationClassification{Append: false}, nil
	}

	switch intent {
	case PatchGenerationIntentPlainRecord:
		if len(prior) == 0 {
			return PatchGenerationClassification{Kind: PatchGenerationKindRecord, Append: true}, nil
		}
		return PatchGenerationClassification{Kind: PatchGenerationKindAmendRefresh, Append: true}, nil
	case PatchGenerationIntentRefresh:
		return PatchGenerationClassification{Kind: PatchGenerationKindAmendRefresh, Append: true}, nil
	case PatchGenerationIntentFixup:
		return PatchGenerationClassification{Kind: PatchGenerationKindAmendFixup, Append: true}, nil
	default:
		return PatchGenerationClassification{}, fmt.Errorf("patch-generations.json: unknown generation intent %q", intent)
	}
}
