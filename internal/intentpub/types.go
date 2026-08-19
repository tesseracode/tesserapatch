package intentpub

import (
	"fmt"
	"io/fs"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
)

const (
	JournalVersion   = 1
	MaxArtifactBytes = 4 << 20
)

type Mode string

const (
	ModeGenerate   Mode = "generate"
	ModeManual     Mode = "manual"
	ModeRegenerate Mode = "regenerate"
)

type ArtifactID string

const (
	ArtifactAnalysis        ArtifactID = "analysis"
	ArtifactSpec            ArtifactID = "spec"
	ArtifactExploration     ArtifactID = "exploration"
	ArtifactAnalysisSidecar ArtifactID = "analysis_sidecar"
	ArtifactArchiveIndex    ArtifactID = "archive_index"
	ArtifactStatus          ArtifactID = "status"
)

var artifactOrder = []ArtifactID{
	ArtifactAnalysis,
	ArtifactSpec,
	ArtifactExploration,
	ArtifactAnalysisSidecar,
	ArtifactArchiveIndex,
	ArtifactStatus,
}

type Action string

const (
	ActionCreate  Action = "create"
	ActionReplace Action = "replace"
)

// Identity is the complete semantic compare-and-swap identity of a file.
type Identity struct {
	Exists bool   `json:"exists"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	Mode   uint32 `json:"mode"`
}

func (i Identity) Equal(other Identity) bool {
	if i.Exists != other.Exists {
		return false
	}
	if !i.Exists {
		return true
	}
	return i.SHA256 == other.SHA256 &&
		i.Size == other.Size &&
		i.Mode == other.Mode
}

func AbsentIdentity() Identity {
	return Identity{}
}

// Entry describes one frozen publication step. Recovery sources contain only
// durable root-relative references, never file content.
type Entry struct {
	ArtifactID      ArtifactID `json:"artifact_id"`
	Rel             string     `json:"rel"`
	Action          Action     `json:"action"`
	Preimage        Identity   `json:"preimage"`
	PreimageBlob    string     `json:"preimage_blob,omitempty"`
	PreimageBlobRel string     `json:"preimage_blob_rel,omitempty"`
	PreimageRawRel  string     `json:"preimage_raw_rel,omitempty"`
	NewImage        Identity   `json:"new_image"`
	StagedRel       string     `json:"staged_rel"`
}

type ArchivePreimage struct {
	SHA256 string
	Rel    string
}

// PreimageReferences is the S3/S4 boundary. It supplies references to
// already-durable preimages; intentpub does not classify or create archives.
type PreimageReferences interface {
	CanonicalPreimage(ArtifactID, Identity) (ArchivePreimage, error)
	MetadataPreimage(ArtifactID) (string, error)
}

func BindPreimageReference(entry Entry, references PreimageReferences) (Entry, error) {
	if entry.Action == ActionCreate {
		return entry, nil
	}
	if references == nil || entry.Action != ActionReplace {
		return entry, transactionError(CodeInvalidPlan, entry.ArtifactID, "preimage-reference", "a replacement requires a durable preimage reference", 5)
	}
	switch entry.ArtifactID {
	case ArtifactArchiveIndex, ArtifactStatus:
		rel, err := references.MetadataPreimage(entry.ArtifactID)
		if err != nil {
			return entry, transactionError(CodeSourceInvalid, entry.ArtifactID, "metadata-reference", "the metadata preimage reference is unavailable", 5)
		}
		entry.PreimageRawRel = rel
	default:
		source, err := references.CanonicalPreimage(entry.ArtifactID, entry.Preimage)
		if err != nil {
			return entry, transactionError(CodeSourceInvalid, entry.ArtifactID, "archive-reference", "the archive preimage reference is unavailable", 5)
		}
		entry.PreimageBlob = source.SHA256
		entry.PreimageBlobRel = source.Rel
	}
	return entry, nil
}

type Journal struct {
	Version    int     `json:"version"`
	Slug       string  `json:"slug"`
	Mode       Mode    `json:"mode"`
	RunNonce   string  `json:"run_nonce"`
	PlanDigest string  `json:"plan_digest"`
	StageRel   string  `json:"stage_rel"`
	Entries    []Entry `json:"entries"`
}

type Outcome string

const (
	OutcomePublished      Outcome = "published"
	OutcomeRolledBack     Outcome = "rolled-back"
	OutcomeRecovered      Outcome = "recovered"
	OutcomeRecoveryAbsent Outcome = "recovery-absent"
	OutcomeFailed         Outcome = "failed"
)

type Result struct {
	Outcome   Outcome
	ExitClass int
	Published []ArtifactID
	Restored  []ArtifactID
	Orphans   []string
	Completed bool
}

type WritePhase string

const (
	WritePhaseNone            WritePhase = ""
	WritePhaseParentReady     WritePhase = "parent-ready"
	WritePhaseTempOpened      WritePhase = "temp-opened"
	WritePhaseTempWritten     WritePhase = "temp-written"
	WritePhaseTempSynced      WritePhase = "temp-synced"
	WritePhaseTempClosed      WritePhase = "temp-closed"
	WritePhaseCASValidated    WritePhase = "cas-validated"
	WritePhaseRenamed         WritePhase = "renamed"
	WritePhaseDirectorySynced WritePhase = "directory-synced"
	WritePhaseVerified        WritePhase = "verified"
)

type WriteResult struct {
	Identity  Identity
	Committed bool
	Phase     WritePhase
}

type Transaction struct {
	Authority *intentlock.WorkspaceAuthority
	Plan      Plan
	RunNonce  string
	Orphans   []string
	Options   Options
}

func (transaction Transaction) Execute() (Result, error) {
	return Execute(transaction.Authority, transaction.Plan, transaction.RunNonce, transaction.Orphans, transaction.Options)
}

type Code string

const (
	CodeInvalidPlan                           Code = "invalid-plan"
	CodeEntryAppeared                         Code = "entry-appeared"
	CodeEntryChanged                          Code = "entry-changed"
	CodeUndoCASMismatch                       Code = "undo-cas-mismatch"
	CodePostPublicationDivergence             Code = "post-publication-divergence"
	CodeWorkspaceRootChanged                  Code = "workspace-root-changed"
	CodeWorkspaceRootReplacedAfterPublication Code = "workspace-root-replaced-after-publication"
	CodeJournalCorrupt                        Code = "journal-corrupt"
	CodeJournalVersionMismatch                Code = "journal-version-mismatch"
	CodeJournalForeign                        Code = "journal-foreign"
	CodeJournalPathEscape                     Code = "journal-path-escape"
	CodeJournalForged                         Code = "journal-forged"
	CodeJournalPending                        Code = "journal-pending"
	CodeRecoveryDivergent                     Code = "recovery-divergent"
	CodeNonRegular                            Code = "non-regular-file"
	CodeIdentityUnstable                      Code = "identity-unstable"
	CodeFileOversize                          Code = "file-oversize"
	CodeSourceInvalid                         Code = "preimage-source-invalid"
	CodeRootedWrite                           Code = "rooted-write-failed"
	CodeCleanupFailed                         Code = "cleanup-failed"
	CodeCrashInjected                         Code = "crash-injected"
	CodeStagedOutputInvalid                   Code = "staged-output-invalid"
)

// Error is path-safe: it identifies only a stable code and artifact ID.
type Error struct {
	Code       Code
	ArtifactID ArtifactID
	Class      string
	Detail     string
	ExitClass  int
	Committed  bool
	WritePhase WritePhase
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	prefix := string(e.Code)
	if e.ArtifactID != "" {
		prefix += " (" + string(e.ArtifactID) + ")"
	}
	if e.Class != "" {
		prefix += " [" + e.Class + "]"
	}
	if e.Detail == "" {
		return prefix
	}
	return fmt.Sprintf("%s: %s", prefix, e.Detail)
}

func transactionError(code Code, id ArtifactID, class, detail string, exitClass int) *Error {
	return &Error{
		Code:       code,
		ArtifactID: id,
		Class:      class,
		Detail:     detail,
		ExitClass:  exitClass,
	}
}

type StageInput struct {
	ArtifactID ArtifactID
	Rel        string
	Data       []byte
	Mode       fs.FileMode
}

type StagedFile struct {
	ArtifactID ArtifactID
	Rel        string
	Identity   Identity
	NewImage   Identity
}

type StageResult struct {
	StageRel string
	Files    []StagedFile
}

type CrashPoint string

const (
	PointBeforeSetValidation CrashPoint = "before-set-validation"
	PointAfterJournalDurable CrashPoint = "cp3-after-journal-durable"
	PointBeforeEntryCAS      CrashPoint = "before-entry-cas"
	PointAfterEntryRename    CrashPoint = "after-entry-rename"
	PointBeforeFinalVerify   CrashPoint = "before-final-verification"
	PointAfterAllRenames     CrashPoint = "cp7-after-all-renames"
	PointAfterJournalClear   CrashPoint = "cp8-after-journal-clear"
	PointBeforeUndo          CrashPoint = "before-undo"
	PointAfterUndo           CrashPoint = "after-undo"
	PointBeforeRecoveryClear CrashPoint = "before-recovery-clear"
)
