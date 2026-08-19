package intentpub

import (
	"errors"
	"io/fs"
	"os"
	"path"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
)

type undoContext uint8

const (
	undoExecution undoContext = iota
	undoRecovery
)

type setExpectation uint8

const (
	expectPreimage setExpectation = iota
	expectNewImage
)

func Execute(authority *intentlock.WorkspaceAuthority, plan Plan, runNonce string, orphans []string, options Options) (Result, error) {
	result := Result{
		Outcome:   OutcomeFailed,
		ExitClass: 5,
		Published: []ArtifactID{},
		Restored:  []ArtifactID{},
		Orphans:   append([]string(nil), orphans...),
	}
	if authority == nil {
		return result, transactionError(CodeInvalidPlan, "", "authority", "workspace authority is required", 5)
	}
	if err := validatePlanShape(plan.slug, plan.mode, plan.stageRel, plan.entries); err != nil {
		return resultWithError(result, err)
	}
	options, err := options.withScratch()
	if err != nil {
		return resultWithError(result, err)
	}
	journal, err := BuildJournal(plan, runNonce)
	if err != nil {
		return resultWithError(result, err)
	}
	journalBytes, err := EncodeJournal(journal)
	if err != nil {
		return resultWithError(result, err)
	}

	if err := callHook(options, PointBeforeSetValidation, nil, nil); err != nil {
		return resultWithError(result, err)
	}
	if err := validateOriginalRoot(authority, false); err != nil {
		return resultWithError(result, err)
	}
	err = authority.WithRoot(func(root *os.Root) error {
		ops := options.rootOps(root)
		if err := validateCanonicalParents(ops, plan.entries); err != nil {
			return err
		}
		return validatePrePublication(ops, plan, options)
	})
	if err != nil {
		return resultWithError(result, err)
	}

	journalWrite, err := PersistJournal(authority, journal, options)
	if err != nil {
		if journalWrite.Committed {
			result.ExitClass = 6
			if rootErr := validateOriginalRoot(authority, true); rootErr != nil {
				return resultWithError(result, rootErr)
			}
		}
		return resultWithError(result, err)
	}
	evidence := journalEvidence{
		Bytes:    journalBytes,
		Identity: journalWrite.Identity,
	}

	var rollbackCause error
	err = authority.WithRoot(func(root *os.Root) error {
		ops := options.rootOps(root)
		if err := callHook(options, PointAfterJournalDurable, root, nil); err != nil {
			return err
		}
		for index := range plan.entries {
			entry := plan.entries[index]
			if err := callHook(options, PointBeforeEntryCAS, root, &entry); err != nil {
				return err
			}
			stagedBytes, err := readValidatedStaged(ops, entry, options)
			if err != nil {
				rollbackCause = err
				break
			}
			if entry.Preimage.Equal(entry.NewImage) {
				current, captureErr := options.capture(ops, entry.Rel)
				if captureErr != nil || !current.Equal(entry.Preimage) {
					rollbackCause = transactionError(CodeEntryChanged, entry.ArtifactID, "no-op-cas", "the semantic no-op entry changed before publication", 5)
					break
				}
				continue
			}
			mismatchCode := CodeEntryChanged
			if entry.Action == ActionCreate {
				mismatchCode = CodeEntryAppeared
			}
			writeResult, writeErr := durableWriteRoot(ops, WriteRequest{
				Rel:           entry.Rel,
				Data:          stagedBytes,
				Mode:          fs.FileMode(entry.NewImage.Mode),
				TempSuffix:    canonicalTempSuffix(journal.RunNonce, entry),
				Expected:      identityPointer(entry.Preimage),
				MismatchCode:  mismatchCode,
				ArtifactID:    entry.ArtifactID,
				RequireParent: true,
			}, entry.NewImage, options)
			if writeResult.Committed {
				result.Published = append(result.Published, entry.ArtifactID)
			}
			if writeErr != nil {
				var typed *Error
				if writeResult.Committed || (errors.As(writeErr, &typed) && typed.ExitClass == 6) {
					return writeErr
				}
				rollbackCause = writeErr
				break
			}
			if !writeResult.Committed {
				return transactionError(CodePostPublicationDivergence, entry.ArtifactID, "publish-commit-state", "the publication writer returned without a committed rename", 6)
			}
			if err := callHook(options, PointAfterEntryRename, root, &entry); err != nil {
				return err
			}
		}

		if rollbackCause != nil {
			if err := validateOriginalRoot(authority, true); err != nil {
				return err
			}
			restored, rollbackErr := rollbackRoot(root, ops, plan.entries, result.Published, journal.RunNonce, options, undoExecution)
			result.Restored = restored
			if rollbackErr != nil {
				return rollbackErr
			}
			verified, verifyErr := verifyFinalSet(authority, ops, plan.entries, expectPreimage, result.Restored, options, undoExecution)
			result.Restored = verified
			if verifyErr != nil {
				return verifyErr
			}
			if err := cleanupTransactionRoot(ops, journal, evidence, options); err != nil {
				return err
			}
			result.Outcome = OutcomeRolledBack
			result.ExitClass = 5
			return rollbackCause
		}

		if err := callHook(options, PointAfterAllRenames, root, nil); err != nil {
			return err
		}
		if err := validateOriginalRoot(authority, true); err != nil {
			return err
		}
		if err := callHook(options, PointBeforeFinalVerify, root, nil); err != nil {
			return err
		}
		for _, entry := range plan.entries {
			current, captureErr := options.capture(ops, entry.Rel)
			if captureErr != nil || !current.Equal(entry.NewImage) {
				return transactionError(CodePostPublicationDivergence, entry.ArtifactID, "final-verification", "the final publication identity diverged", 6)
			}
		}
		if err := cleanupTransactionRoot(ops, journal, evidence, options); err != nil {
			return err
		}
		result.Outcome = OutcomePublished
		result.ExitClass = 0
		result.Completed = true
		if err := callHook(options, PointAfterJournalClear, root, nil); err != nil {
			// CP8 is after durable evidence removal. A real crash returns no
			// process result; the in-process seam therefore keeps the already
			// committed success rather than manufacturing an evidence-less 6.
			return nil
		}
		return nil
	})
	if err != nil {
		if result.Outcome == OutcomeRolledBack {
			return result, rollbackCause
		}
		if rootErr := validateOriginalRoot(authority, true); rootErr != nil {
			return resultWithError(result, rootErr)
		}
		return resultWithError(result, err)
	}
	return result, nil
}

func validateCanonicalParents(ops RootOps, entries []Entry) error {
	seen := make(map[string]struct{})
	for _, entry := range entries {
		directory := path.Dir(entry.Rel)
		if _, ok := seen[directory]; ok {
			continue
		}
		seen[directory] = struct{}{}
		if err := validateDirectoryChain(ops, directory); err != nil {
			return transactionError(CodeRootedWrite, entry.ArtifactID, "canonical-parent", "a canonical destination parent is absent or unsafe", 5)
		}
	}
	return nil
}

func validatePrePublication(ops RootOps, plan Plan, options Options) error {
	for _, entry := range plan.entries {
		current, err := options.capture(ops, entry.Rel)
		if err != nil {
			return attachArtifact(err, entry.ArtifactID)
		}
		if !current.Equal(entry.Preimage) {
			code := CodeEntryChanged
			if entry.Action == ActionCreate && current.Exists {
				code = CodeEntryAppeared
			}
			return transactionError(code, entry.ArtifactID, "set-revalidation", "the canonical entry changed before publication was armed", 5)
		}
		if _, err := readValidatedStaged(ops, entry, options); err != nil {
			return err
		}
		if entry.Action == ActionReplace {
			if _, err := readValidatedPreimage(ops, entry, options); err != nil {
				return err
			}
		}
	}
	return nil
}

func readValidatedStaged(ops RootOps, entry Entry, options Options) ([]byte, error) {
	captured, err := options.captureBytes(ops, entry.StagedRel)
	if err != nil || !captured.Identity.Equal(stagedIdentity(entry.NewImage)) {
		return nil, transactionError(CodeEntryChanged, entry.ArtifactID, "v6-staged-identity", "the staged entry changed after synchronization", 5)
	}
	if err := validateStagedBytes(entry.ArtifactID, captured.Bytes); err != nil {
		return nil, err
	}
	return captured.Bytes, nil
}

func rollbackRoot(root *os.Root, ops RootOps, entries []Entry, published []ArtifactID, runNonce string, options Options, context undoContext) ([]ArtifactID, error) {
	restored := []ArtifactID{}
	for index := len(published) - 1; index >= 0; index-- {
		id := published[index]
		entry, ok := findEntry(entries, id)
		if !ok {
			return restored, transactionError(CodeInvalidPlan, id, "rollback-entry", "the published entry is absent from the frozen plan", 6)
		}
		if err := callHook(options, PointBeforeUndo, root, &entry); err != nil {
			return restored, err
		}
		current, err := options.capture(ops, entry.Rel)
		if err != nil || !current.Equal(entry.NewImage) {
			return restored, undoMismatch(context, entry.ArtifactID, "undo-cas")
		}
		if entry.Preimage.Equal(entry.NewImage) {
			continue
		}
		switch entry.Action {
		case ActionCreate:
			if err := ops.Remove(entry.Rel); err != nil {
				return restored, transactionError(CodeRootedWrite, entry.ArtifactID, "undo-remove", "the created entry could not be removed", 6)
			}
			if err := syncDir(ops, path.Dir(entry.Rel)); err != nil {
				return restored, transactionError(CodeRootedWrite, entry.ArtifactID, "undo-sync", "the undo directory could not be synchronized", 6)
			}
			after, err := options.capture(ops, entry.Rel)
			if err != nil || after.Exists {
				return restored, undoMismatch(context, entry.ArtifactID, "undo-remove-postcondition")
			}
		case ActionReplace:
			data, err := readValidatedPreimage(ops, entry, options)
			if err != nil {
				if context == undoRecovery {
					return restored, recoveryDivergence(entry.ArtifactID, "preimage-source")
				}
				return restored, err
			}
			mismatchCode := CodeUndoCASMismatch
			if context == undoRecovery {
				mismatchCode = CodeRecoveryDivergent
			}
			_, err = durableWriteRoot(ops, WriteRequest{
				Rel:           entry.Rel,
				Data:          data,
				Mode:          fs.FileMode(entry.Preimage.Mode),
				TempSuffix:    canonicalTempSuffix(runNonce, entry),
				Expected:      identityPointer(entry.NewImage),
				MismatchCode:  mismatchCode,
				ArtifactID:    entry.ArtifactID,
				RequireParent: true,
			}, entry.Preimage, options)
			if err != nil {
				var typed *Error
				if errors.As(err, &typed) && (typed.Code == CodeNonRegular || typed.Code == CodeIdentityUnstable ||
					typed.Code == CodeEntryChanged || typed.Code == CodeUndoCASMismatch || typed.Code == CodeRecoveryDivergent) {
					return restored, undoMismatch(context, entry.ArtifactID, "undo-write-cas")
				}
				return restored, err
			}
		}
		restored = append(restored, entry.ArtifactID)
		if err := callHook(options, PointAfterUndo, root, &entry); err != nil {
			return restored, err
		}
	}
	return restored, nil
}

func verifyFinalSet(
	authority *intentlock.WorkspaceAuthority,
	ops RootOps,
	entries []Entry,
	expected setExpectation,
	restored []ArtifactID,
	options Options,
	context undoContext,
) ([]ArtifactID, error) {
	if err := validateOriginalRoot(authority, true); err != nil {
		return nil, err
	}

	matches := make(map[ArtifactID]bool, len(entries))
	var firstMismatch ArtifactID
	for _, entry := range entries {
		want := entry.Preimage
		if expected == expectNewImage {
			want = entry.NewImage
		}
		current, err := options.capture(ops, entry.Rel)
		matches[entry.ArtifactID] = err == nil && current.Equal(want)
		if !matches[entry.ArtifactID] && firstMismatch == "" {
			firstMismatch = entry.ArtifactID
		}
	}

	verified := make([]ArtifactID, 0, len(restored))
	for _, id := range restored {
		if matches[id] {
			verified = append(verified, id)
		}
	}
	if err := validateOriginalRoot(authority, true); err != nil {
		return verified, err
	}
	if firstMismatch != "" {
		return verified, undoMismatch(context, firstMismatch, "undo-final-set")
	}
	return verified, nil
}

func Recover(authority *intentlock.WorkspaceAuthority, slug string, options Options) (Result, error) {
	result := Result{
		Outcome:   OutcomeRecoveryAbsent,
		ExitClass: 0,
		Published: []ArtifactID{},
		Restored:  []ArtifactID{},
		Orphans:   []string{},
	}
	if authority == nil || !validSlug(slug) {
		return resultWithError(result, transactionError(CodeInvalidPlan, "", "recovery-input", "recovery requires an authority and valid slug", 6))
	}
	options, err := options.withScratch()
	if err != nil {
		return resultWithError(result, err)
	}
	evidence, found, err := loadJournalBytes(authority, slug, options)
	if err != nil {
		return resultWithError(result, err)
	}
	if !found {
		return result, nil
	}
	journal, err := DecodeJournal(evidence.Bytes, slug)
	if err != nil {
		return resultWithError(result, err)
	}
	if err := validateOriginalRoot(authority, true); err != nil {
		return resultWithError(result, err)
	}

	err = authority.WithRoot(func(root *os.Root) error {
		ops := options.rootOps(root)
		if err := validateJournalEvidence(ops, journal, evidence, options); err != nil {
			return err
		}
		if err := removeCanonicalTemps(ops, journal); err != nil {
			return transactionError(CodeCleanupFailed, "", "canonical-temp", "a journal-bound canonical temporary file could not be removed durably", 6)
		}
		newEntries := make([]ArtifactID, 0, len(journal.Entries))
		preimageCount := 0
		dualCount := 0
		for _, entry := range journal.Entries {
			current, captureErr := options.capture(ops, entry.Rel)
			if captureErr != nil {
				return recoveryDivergence(entry.ArtifactID, "identity-capture")
			}
			switch {
			case current.Equal(entry.NewImage) && current.Equal(entry.Preimage):
				dualCount++
			case current.Equal(entry.NewImage):
				newEntries = append(newEntries, entry.ArtifactID)
			case current.Equal(entry.Preimage):
				preimageCount++
			default:
				return recoveryDivergence(entry.ArtifactID, "identity")
			}
		}

		expected := expectPreimage
		switch {
		case len(newEntries)+dualCount == len(journal.Entries):
			result.Completed = true
			expected = expectNewImage
		case preimageCount+dualCount == len(journal.Entries):
			// The journal was durable, but no entry was published.
		default:
			for _, id := range newEntries {
				entry, _ := findEntry(journal.Entries, id)
				if entry.Action == ActionReplace {
					if _, err := readValidatedPreimage(ops, entry, options); err != nil {
						return recoveryDivergence(entry.ArtifactID, "preimage-source")
					}
				}
			}
			restored, err := rollbackRoot(root, ops, journal.Entries, newEntries, journal.RunNonce, options, undoRecovery)
			result.Restored = restored
			if err != nil {
				return err
			}
		}
		if err := callHook(options, PointBeforeRecoveryClear, root, nil); err != nil {
			return err
		}
		verified, verifyErr := verifyFinalSet(authority, ops, journal.Entries, expected, result.Restored, options, undoRecovery)
		result.Restored = verified
		if verifyErr != nil {
			return verifyErr
		}
		if err := cleanupTransactionRoot(ops, journal, evidence, options); err != nil {
			return err
		}
		result.Outcome = OutcomeRecovered
		result.ExitClass = 0
		return nil
	})
	if err != nil {
		return resultWithError(result, err)
	}
	return result, nil
}

func loadJournalBytes(authority *intentlock.WorkspaceAuthority, slug string, options Options) (journalEvidence, bool, error) {
	var evidence journalEvidence
	found := false
	err := authority.WithRoot(func(root *os.Root) error {
		ops := options.rootOps(root)
		lane, err := ops.Lstat(laneRel(slug))
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil || refusedFileInfo(lane) || !lane.IsDir() {
			return journalBindError(CodeJournalCorrupt, "lane-kind")
		}
		journalExists, err := regularControlExists(ops, JournalRel(slug))
		if err != nil {
			return journalBindError(CodeJournalCorrupt, "journal-kind")
		}
		markerExists, err := regularControlExists(ops, JournalMarkerRel(slug))
		if err != nil {
			return journalBindError(CodeJournalCorrupt, "clearing-marker-kind")
		}
		if journalExists && markerExists {
			return journalBindError(CodeJournalCorrupt, "duplicate-control-evidence")
		}
		if !journalExists && !markerExists {
			return nil
		}
		rel := JournalRel(slug)
		if markerExists {
			rel = JournalMarkerRel(slug)
			evidence.MarkerPresent = true
		}
		captured, captureErr := options.captureBytes(ops, rel)
		if captureErr != nil || !captured.Identity.Exists {
			return journalBindError(CodeJournalCorrupt, "journal-read")
		}
		evidence.Bytes = append([]byte(nil), captured.Bytes...)
		evidence.Identity = captured.Identity
		found = true
		return nil
	})
	return evidence, found, err
}

func regularControlExists(ops RootOps, rel string) (bool, error) {
	info, err := ops.Lstat(rel)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil || refusedFileInfo(info) || !info.Mode().IsRegular() {
		return false, fs.ErrInvalid
	}
	return true, nil
}

func readValidatedPreimage(ops RootOps, entry Entry, options Options) ([]byte, error) {
	rel := entry.PreimageRawRel
	if rel == "" {
		rel = entry.PreimageBlobRel
	}
	captured, err := options.captureBytes(ops, rel)
	if err != nil || !captured.Identity.Exists || captured.Identity.Size != entry.Preimage.Size {
		return nil, transactionError(CodeSourceInvalid, entry.ArtifactID, "source-kind", "the durable preimage source is absent or invalid", 6)
	}
	expectedSourceMode := uint32(0o644)
	if entry.PreimageRawRel != "" {
		expectedSourceMode = 0o600
	}
	if captured.Identity.Mode != expectedSourceMode {
		return nil, transactionError(CodeSourceInvalid, entry.ArtifactID, "source-mode", "the durable preimage source mode is invalid", 6)
	}
	identity, err := identityForBytes(captured.Bytes, fs.FileMode(entry.Preimage.Mode))
	if err != nil || identity.SHA256 != entry.Preimage.SHA256 || identity.Size != entry.Preimage.Size {
		return nil, transactionError(CodeSourceInvalid, entry.ArtifactID, "source-identity", "the durable preimage source does not match the recorded preimage", 6)
	}
	if entry.PreimageBlob != "" && identity.SHA256 != entry.PreimageBlob {
		return nil, transactionError(CodeSourceInvalid, entry.ArtifactID, "source-hash", "the archive source does not match its content address", 6)
	}
	return captured.Bytes, nil
}

func findEntry(entries []Entry, id ArtifactID) (Entry, bool) {
	for _, entry := range entries {
		if entry.ArtifactID == id {
			return entry, true
		}
	}
	return Entry{}, false
}

func undoMismatch(context undoContext, id ArtifactID, class string) *Error {
	if context == undoRecovery {
		return recoveryDivergence(id, class)
	}
	return transactionError(CodeUndoCASMismatch, id, class, "the published entry no longer matches its intended image", 6)
}

func recoveryDivergence(id ArtifactID, class string) *Error {
	return transactionError(CodeRecoveryDivergent, id, class, "the recovery evidence does not match a safe expected image", 6)
}

func attachArtifact(err error, id ArtifactID) error {
	var typed *Error
	if errors.As(err, &typed) {
		copy := *typed
		copy.ArtifactID = id
		return &copy
	}
	return transactionError(CodeIdentityUnstable, id, "identity", "the entry identity could not be captured", 5)
}

func validateOriginalRoot(authority *intentlock.WorkspaceAuthority, afterPublication bool) error {
	if err := authority.ValidateOriginalPath(afterPublication); err != nil {
		code := CodeWorkspaceRootChanged
		exitClass := 5
		if afterPublication {
			code = CodeWorkspaceRootReplacedAfterPublication
			exitClass = 6
		}
		return transactionError(code, "", "root-identity", "the live workspace root no longer matches the held authority", exitClass)
	}
	return nil
}

func callHook(options Options, point CrashPoint, root *os.Root, entry *Entry) error {
	if options.Hook == nil {
		return nil
	}
	if err := options.Hook(point, root, entry); err != nil {
		id := ArtifactID("")
		if entry != nil {
			id = entry.ArtifactID
		}
		return transactionError(CodeCrashInjected, id, string(point), "execution stopped at an injected crash point", 6)
	}
	return nil
}

func resultWithError(result Result, err error) (Result, error) {
	result.Outcome = OutcomeFailed
	var typed *Error
	if errors.As(err, &typed) {
		result.ExitClass = typed.ExitClass
	} else if result.ExitClass == 0 {
		result.ExitClass = 5
	}
	return result, err
}
