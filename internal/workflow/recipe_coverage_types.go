package workflow

import (
	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
)

// RecipeCoverage is ADR-036 D3's complete, fixed-field v1 wire schema.
// Decoding it establishes structural consistency, never simulation authority.
type RecipeCoverage struct {
	SchemaVersion   int                 `json:"schema_version"`
	Feature         string              `json:"feature"`
	Producer        patchobs.ProducerID `json:"producer"`
	PatchPresent    bool                `json:"patch_present"`
	RecipePresent   bool                `json:"recipe_present"`
	RecipeDecodable bool                `json:"recipe_decodable"`
	PatchSHA256     string              `json:"patch_sha256"`
	RecipeSHA256    string              `json:"recipe_sha256"`
	Reference       CoverageReference   `json:"reference"`
	Capture         CoverageCapture     `json:"capture"`
	CoverageStatus  string              `json:"coverage_status"`
	CrossBaseStatus string              `json:"cross_base_status"`
	Effects         []CoverageEffect    `json:"effects"`
	Reasons         []string            `json:"reasons"`
}

type CoverageReference struct {
	Kind              patchobs.ReferenceKind `json:"kind"`
	Commit            string                 `json:"commit"`
	PreimageSetSHA256 string                 `json:"preimage_set_sha256"`
}

type CoverageCapture struct {
	Mode      patchobs.CaptureMode `json:"mode"`
	Pathspecs []string             `json:"pathspecs"`
	ClaimIDs  []string             `json:"claim_ids"`
}

type CoverageEffect struct {
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
	EffectSHA256        string              `json:"effect_sha256"`
	OperationIndexes    []int               `json:"operation_indexes"`
	Disposition         string              `json:"disposition"`
	ReasonCodes         []string            `json:"reason_codes"`
	ContextualHint      string              `json:"contextual_hint"`
}

// CoverageArtifact supplies readable existence and exact, unmodified bytes.
// A read failure is not a decoding failure; its path and cause stay diagnostic.
type CoverageArtifact struct {
	Present   bool
	Bytes     []byte
	Path      string
	ReadError error
}

// CoverageEvents are facts about the current event, not labels authorizing it.
// Whether a rewrite or edit left coverage truthful is recomputed by the core.
type CoverageEvents struct {
	PatchRewritten      bool
	RecipeRegenerated   bool
	BoundArtifactEdited bool
	StaleMarkerPresent  bool
}

// RecipeCoverageInput contains only already-captured, immutable inputs. It is
// not a publication API and performs no artifact, tree, Git or filesystem read.
type RecipeCoverageInput struct {
	Observation patchobs.Observation
	Recipe      CoverageArtifact
	Events      CoverageEvents
}

const (
	CoverageComplete   = "complete"
	CoverageIncomplete = "incomplete"

	CrossBaseReferenceTreeOnly          = "reference-tree-only"
	CrossBaseConsumerDerivationRequired = "consumer-derivation-required"
	CrossBaseUnsupported                = "unsupported"
)

var coverageRecordReasons = []string{
	"canonical-patch-empty",
	"canonical-patch-missing",
	"canonical-patch-unparseable",
	"manual-bound-artifact-edit",
	"operation-surplus",
	"producer-patch-rewrite",
	"recipe-not-regenerated",
	"recipe-owner-mismatch",
	"recipe-stale-marker-present",
	"recipe-undecodable",
	"reference-not-durable",
	"simulation-mismatch",
}

var coverageEffectReasons = []string{
	"effect-binary-unsupported",
	"effect-copy-unsupported",
	"effect-delete-unsupported",
	"effect-executable-unsupported",
	"effect-gitlink-unsupported",
	"effect-mode-only-unsupported",
	"effect-rename-unsupported",
	"effect-symlink-unsupported",
	"operation-missing",
	"operation-not-reclassifiable",
	"parent-created-target-unsupported",
	"path-unsafe",
	"postimage-unavailable",
	"preimage-unavailable",
}
