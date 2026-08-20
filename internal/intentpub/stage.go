package intentpub

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"unicode/utf8"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
)

func Stage(authority *intentlock.WorkspaceAuthority, slug string, inputs []StageInput, options Options) (StageResult, error) {
	if authority == nil || !validSlug(slug) || len(inputs) == 0 {
		return StageResult{}, transactionError(CodeInvalidPlan, "", "stage-input", "staging requires a valid slug and at least one file", 5)
	}
	options, err := options.withScratch()
	if err != nil {
		return StageResult{}, err
	}
	if err := ValidateStageInputs(inputs); err != nil {
		return StageResult{}, err
	}
	suffix, err := options.randomHex12()
	if err != nil || !validHex(suffix, 12) {
		return StageResult{}, transactionError(CodeRootedWrite, "", "stage-name", "the staging directory name could not be created", 5)
	}
	stageRel := laneRel(slug) + "/stage-" + suffix
	result := StageResult{StageRel: stageRel, Files: make([]StagedFile, 0, len(inputs))}
	if err := authority.WithRoot(func(root *os.Root) error {
		return mkdirChain(options.rootOps(root), stageRel)
	}); err != nil {
		return result, transactionError(CodeRootedWrite, "", "stage-directory", "the staging directory could not be prepared", 5)
	}

	seen := make(map[ArtifactID]bool, len(inputs))
	for _, input := range inputs {
		if artifactIndex(input.ArtifactID) < 0 || seen[input.ArtifactID] ||
			input.Rel != stagedBase(input.ArtifactID) {
			return result, transactionError(CodeInvalidPlan, input.ArtifactID, "stage-entry", "the staging input is invalid or duplicated", 5)
		}
		seen[input.ArtifactID] = true
		mode := input.Mode.Perm()
		if mode == 0 {
			mode = fs.FileMode(0o644)
		}
		if mode != input.Mode.Perm() || input.Mode&^fs.ModePerm != 0 {
			return result, transactionError(CodeInvalidPlan, input.ArtifactID, "stage-mode", "the final canonical mode is invalid", 5)
		}
		newImage, identityErr := identityForBytes(input.Data, mode)
		if identityErr != nil {
			if len(input.Data) > MaxArtifactBytes {
				return result, stagedValidationError(input.ArtifactID, "v2-size")
			}
			return result, attachArtifact(identityErr, input.ArtifactID)
		}
		rel := stageRel + "/" + input.Rel
		writeResult, writeErr := DurableWrite(authority, WriteRequest{
			Rel:        rel,
			Data:       input.Data,
			Mode:       0o600,
			ArtifactID: input.ArtifactID,
			Role:       WriteRoleControl,
		}, options)
		if writeErr != nil {
			return result, writeErr
		}
		file := StagedFile{
			ArtifactID: input.ArtifactID,
			Rel:        rel,
			Identity:   writeResult.Identity,
			NewImage:   newImage,
		}
		result.Files = append(result.Files, file)
		if err := validateStagedBytes(input.ArtifactID, input.Data); err != nil {
			return result, err
		}

		var captured capturedFile
		if err := authority.WithRoot(func(root *os.Root) error {
			var captureErr error
			captured, captureErr = options.captureBytes(options.rootOps(root), rel)
			return captureErr
		}); err != nil || !captured.Identity.Equal(file.Identity) {
			return result, transactionError(CodeEntryChanged, input.ArtifactID, "v6-staged-identity", "the staged file changed after synchronization", 5)
		}
		if err := validateStagedBytes(input.ArtifactID, captured.Bytes); err != nil {
			return result, err
		}
	}
	return result, nil
}

// ValidateStageInputs applies V1-V5 without creating staging state.
func ValidateStageInputs(inputs []StageInput) error {
	if len(inputs) == 0 {
		return transactionError(CodeInvalidPlan, "", "stage-input", "staging requires at least one file", 5)
	}
	seen := make(map[ArtifactID]bool, len(inputs))
	for _, input := range inputs {
		if artifactIndex(input.ArtifactID) < 0 || seen[input.ArtifactID] ||
			input.Rel != stagedBase(input.ArtifactID) {
			return transactionError(CodeInvalidPlan, input.ArtifactID, "stage-entry", "the staging input is invalid or duplicated", 5)
		}
		seen[input.ArtifactID] = true
		mode := input.Mode.Perm()
		if mode == 0 {
			mode = fs.FileMode(0o644)
		}
		if mode != input.Mode.Perm() || input.Mode&^fs.ModePerm != 0 {
			return transactionError(CodeInvalidPlan, input.ArtifactID, "stage-mode", "the final canonical mode is invalid", 5)
		}
		if _, err := identityForBytes(input.Data, mode); err != nil {
			if len(input.Data) > MaxArtifactBytes {
				return stagedValidationError(input.ArtifactID, "v2-size")
			}
			return attachArtifact(err, input.ArtifactID)
		}
		if err := validateStagedBytes(input.ArtifactID, input.Data); err != nil {
			return err
		}
	}
	return nil
}

func validateStagedBytes(id ArtifactID, data []byte) error {
	switch id {
	case ArtifactAnalysis, ArtifactSpec, ArtifactExploration:
		if len(bytes.TrimSpace(data)) == 0 {
			return stagedValidationError(id, "v1-nonempty")
		}
	}
	if len(data) > MaxArtifactBytes {
		return stagedValidationError(id, "v2-size")
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return stagedValidationError(id, "v3-nul")
	}
	if !utf8.Valid(data) {
		return stagedValidationError(id, "v4-utf8")
	}
	if id == ArtifactAnalysisSidecar {
		if err := validateJSONObject(data); err != nil {
			return stagedValidationError(id, "v5-json-object")
		}
	}
	return nil
}

func stagedValidationError(id ArtifactID, class string) *Error {
	return transactionError(CodeStagedOutputInvalid, id, class, "the staged output failed structural validation", 2)
}

func validateJSONObject(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var object map[string]json.RawMessage
	if err := decoder.Decode(&object); err != nil || object == nil {
		return errors.New("not an object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return err
	}
	return nil
}

func WriteRawPreimage(authority *intentlock.WorkspaceAuthority, slug string, id ArtifactID, data []byte, options Options) (string, error) {
	var name string
	switch id {
	case ArtifactArchiveIndex:
		name = "index.preimage.json"
	case ArtifactStatus:
		name = "status.preimage.json"
	default:
		return "", transactionError(CodeInvalidPlan, id, "raw-preimage", "only metadata entries use raw preimages", 5)
	}
	rel := laneRel(slug) + "/" + name
	_, err := DurableWrite(authority, WriteRequest{
		Rel:          rel,
		Data:         data,
		Mode:         0o600,
		Expected:     identityPointer(AbsentIdentity()),
		MismatchCode: CodeEntryAppeared,
		ArtifactID:   id,
		Role:         WriteRoleControl,
	}, options)
	if err != nil {
		return rel, err
	}
	return rel, nil
}

func PersistJournal(authority *intentlock.WorkspaceAuthority, journal Journal, options Options) (WriteResult, error) {
	encoded, err := EncodeJournal(journal)
	if err != nil {
		return WriteResult{}, err
	}
	options, err = options.withScratch()
	if err != nil {
		return WriteResult{}, err
	}
	var marker Identity
	err = authority.WithRoot(func(root *os.Root) error {
		var captureErr error
		marker, captureErr = options.capture(options.rootOps(root), JournalMarkerRel(journal.Slug))
		return captureErr
	})
	if err != nil {
		return WriteResult{}, journalBindError(CodeJournalCorrupt, "clearing-marker-kind")
	}
	if marker.Exists {
		return WriteResult{}, transactionError(CodeJournalPending, "", "clearing-marker", "a transaction clearing marker is already pending", 6)
	}
	rel := JournalRel(journal.Slug)
	if beforeJournalWrite != nil {
		beforeJournalWrite(rel)
	}
	result, err := DurableWrite(authority, WriteRequest{
		Rel:          rel,
		Data:         encoded,
		Mode:         0o600,
		Expected:     identityPointer(AbsentIdentity()),
		MismatchCode: CodeJournalPending,
		Role:         WriteRoleControl,
	}, options)
	if result.Committed && afterJournalWrite != nil {
		afterJournalWrite(rel)
	}
	return result, err
}

func identityPointer(identity Identity) *Identity {
	return &identity
}

func stagedBase(id ArtifactID) string {
	switch id {
	case ArtifactAnalysis:
		return "analysis.md"
	case ArtifactSpec:
		return "spec.md"
	case ArtifactExploration:
		return "exploration.md"
	case ArtifactAnalysisSidecar:
		return "analysis.json"
	case ArtifactArchiveIndex:
		return "index.json"
	case ArtifactStatus:
		return "status.json"
	default:
		return path.Base("")
	}
}
