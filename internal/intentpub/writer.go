package intentpub

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
)

type WriteRequest struct {
	Rel           string
	Data          []byte
	Mode          fs.FileMode
	TempSuffix    string
	Expected      *Identity
	MismatchCode  Code
	ArtifactID    ArtifactID
	RequireParent bool
}

func DurableWrite(authority *intentlock.WorkspaceAuthority, request WriteRequest, options Options) (WriteResult, error) {
	if authority == nil || !validRootRel(request.Rel) {
		return WriteResult{}, transactionError(CodeInvalidPlan, request.ArtifactID, "write-path", "the write target is not root-relative", 5)
	}
	if request.Mode.Perm() != request.Mode || request.Mode.Perm() == 0 {
		return WriteResult{}, transactionError(CodeInvalidPlan, request.ArtifactID, "write-mode", "the write mode is invalid", 5)
	}
	intended, err := identityForBytes(request.Data, request.Mode)
	if err != nil {
		return WriteResult{}, attachArtifact(err, request.ArtifactID)
	}
	options, err = options.withScratch()
	if err != nil {
		return WriteResult{}, err
	}

	var result WriteResult
	err = authority.WithRoot(func(root *os.Root) error {
		var writeErr error
		result, writeErr = durableWriteRoot(options.rootOps(root), request, intended, options)
		return writeErr
	})
	return result, err
}

func durableWriteRoot(ops RootOps, request WriteRequest, intended Identity, options Options) (result WriteResult, err error) {
	result.Identity = intended
	directory := path.Dir(request.Rel)
	if request.RequireParent {
		if err := validateDirectoryChain(ops, directory); err != nil {
			return result, writerError(CodeRootedWrite, request.ArtifactID, "parent", "the rooted destination directory is unavailable", 5, false, result.Phase)
		}
	} else if err := mkdirChain(ops, directory); err != nil {
		return result, writerError(CodeRootedWrite, request.ArtifactID, "mkdir", "the rooted destination directory could not be prepared", 5, false, result.Phase)
	}
	result.Phase = WritePhaseParentReady

	suffix := request.TempSuffix
	if suffix == "" {
		var err error
		suffix, err = options.randomHex12()
		if err != nil {
			return result, writerError(CodeRootedWrite, request.ArtifactID, "temp-name", "a rooted temporary name could not be created", 5, false, result.Phase)
		}
	}
	if !validHex(suffix, 12) {
		return result, writerError(CodeInvalidPlan, request.ArtifactID, "temp-name", "the rooted temporary suffix is invalid", 5, false, result.Phase)
	}
	base := path.Base(request.Rel)
	tempRel := path.Join(directory, "."+base+".tmp-"+suffix)
	file, err := ops.OpenFile(tempRel, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return result, writerError(CodeRootedWrite, request.ArtifactID, "open-temp", "the rooted temporary file could not be created", 5, false, result.Phase)
	}
	result.Phase = WritePhaseTempOpened
	tempPresent := true
	fileOpen := true

	cleanup := func() error {
		var cleanupErr error
		if fileOpen {
			if closeErr := file.Close(); closeErr != nil {
				cleanupErr = closeErr
			}
			fileOpen = false
		}
		if tempPresent {
			if removeErr := ops.Remove(tempRel); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
				cleanupErr = errors.Join(cleanupErr, removeErr)
			} else if syncErr := syncDir(ops, directory); syncErr != nil {
				cleanupErr = errors.Join(cleanupErr, syncErr)
			}
			tempPresent = false
		}
		return cleanupErr
	}
	failBeforeCommit := func(code Code, class, detail string, exitClass int) (WriteResult, error) {
		if cleanupErr := cleanup(); cleanupErr != nil {
			return result, writerError(CodeCleanupFailed, request.ArtifactID, "temp-cleanup", "the rooted temporary file could not be removed durably", 6, false, result.Phase)
		}
		return result, writerError(code, request.ArtifactID, class, detail, exitClass, false, result.Phase)
	}

	if err := file.Chmod(request.Mode); err != nil {
		return failBeforeCommit(CodeRootedWrite, "chmod-temp", "the rooted temporary file mode could not be set", 5)
	}
	tempInfo, err := file.Stat()
	if err != nil || !tempInfo.Mode().IsRegular() || tempInfo.Mode().Perm() != request.Mode {
		return failBeforeCommit(CodeRootedWrite, "verify-temp-mode", "the rooted temporary file mode could not be verified", 5)
	}
	if err := writeAll(file, request.Data); err != nil {
		return failBeforeCommit(CodeRootedWrite, "write-temp", "the rooted temporary file could not be written", 5)
	}
	result.Phase = WritePhaseTempWritten
	if err := file.Sync(); err != nil {
		return failBeforeCommit(CodeRootedWrite, "sync-temp", "the rooted temporary file could not be synchronized", 5)
	}
	result.Phase = WritePhaseTempSynced
	if err := file.Close(); err != nil {
		fileOpen = false
		return failBeforeCommit(CodeRootedWrite, "close-temp", "the rooted temporary file could not be closed", 5)
	}
	fileOpen = false
	result.Phase = WritePhaseTempClosed

	if request.Expected != nil {
		current, captureErr := options.capture(ops, request.Rel)
		if captureErr != nil || !current.Equal(*request.Expected) {
			code := request.MismatchCode
			if code == "" {
				code = CodeEntryChanged
			}
			return failBeforeCommit(code, "write-cas", "the destination changed before the rooted rename", exitClassForCode(code))
		}
	}
	result.Phase = WritePhaseCASValidated
	if err := ops.Rename(tempRel, request.Rel); err != nil {
		return failBeforeCommit(CodeRootedWrite, "rename", "the rooted temporary file could not be published", 5)
	}
	tempPresent = false
	result.Committed = true
	result.Phase = WritePhaseRenamed

	if err := syncDir(ops, directory); err != nil {
		return result, writerError(CodePostPublicationDivergence, request.ArtifactID, "sync-directory", "the committed destination directory could not be synchronized", 6, true, result.Phase)
	}
	result.Phase = WritePhaseDirectorySynced
	current, captureErr := options.capture(ops, request.Rel)
	if captureErr != nil || !current.Equal(intended) {
		return result, writerError(CodePostPublicationDivergence, request.ArtifactID, "writer-post-cas", "the committed rooted write could not be verified", 6, true, result.Phase)
	}
	result.Phase = WritePhaseVerified
	return result, nil
}

func writerError(code Code, id ArtifactID, class, detail string, exitClass int, committed bool, phase WritePhase) *Error {
	err := transactionError(code, id, class, detail, exitClass)
	err.Committed = committed
	err.WritePhase = phase
	return err
}

func mkdirChain(ops RootOps, directory string) error {
	if directory == "." {
		return nil
	}
	components := strings.Split(directory, "/")
	current := ""
	for _, component := range components {
		if current == "" {
			current = component
		} else {
			current += "/" + component
		}
		info, err := ops.Lstat(current)
		if err == nil {
			if refusedFileInfo(info) || !info.IsDir() {
				return fs.ErrInvalid
			}
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		mode := directoryMode(current)
		if err := ops.Mkdir(current, mode); err != nil {
			if !errors.Is(err, fs.ErrExist) {
				return err
			}
			info, statErr := ops.Lstat(current)
			if statErr != nil || refusedFileInfo(info) || !info.IsDir() {
				return fs.ErrInvalid
			}
			continue
		}
		file, err := ops.Open(current)
		if err != nil {
			return err
		}
		if err := file.Chmod(mode); err != nil {
			_ = file.Close()
			return err
		}
		info, statErr := file.Stat()
		if statErr != nil || !info.IsDir() || info.Mode().Perm() != mode {
			_ = file.Close()
			return fs.ErrInvalid
		}
		if err := file.Sync(); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		if err := syncDir(ops, path.Dir(current)); err != nil {
			return err
		}
	}
	return nil
}

func validateDirectoryChain(ops RootOps, directory string) error {
	if directory == "." {
		return nil
	}
	current := ""
	for _, component := range strings.Split(directory, "/") {
		if current == "" {
			current = component
		} else {
			current += "/" + component
		}
		info, err := ops.Lstat(current)
		if err != nil || refusedFileInfo(info) || !info.IsDir() {
			return fs.ErrInvalid
		}
	}
	return nil
}

func directoryMode(rel string) fs.FileMode {
	if rel == ".tpatch/local" || strings.HasPrefix(rel, ".tpatch/local/") {
		return 0o700
	}
	return 0o755
}

func syncDir(ops RootOps, directory string) error {
	file, err := ops.Open(directory)
	if err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func writeAll(file RootFile, data []byte) error {
	for len(data) > 0 {
		count, err := file.Write(data)
		if err != nil {
			return err
		}
		if count <= 0 {
			return fs.ErrInvalid
		}
		data = data[count:]
	}
	return nil
}

func exitClassForCode(code Code) int {
	switch code {
	case CodeUndoCASMismatch, CodePostPublicationDivergence, CodeRecoveryDivergent,
		CodeJournalCorrupt, CodeJournalVersionMismatch, CodeJournalForeign,
		CodeJournalPathEscape, CodeJournalForged, CodeJournalPending,
		CodeWorkspaceRootReplacedAfterPublication, CodeCleanupFailed:
		return 6
	default:
		return 5
	}
}
