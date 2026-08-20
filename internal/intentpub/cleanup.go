package intentpub

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
)

type journalEvidence struct {
	Bytes         []byte
	Identity      Identity
	MarkerPresent bool
}

func cleanupTransactionRoot(ops RootOps, journal Journal, evidence journalEvidence, options Options) error {
	lane := laneRel(journal.Slug)
	markerRel := JournalMarkerRel(journal.Slug)
	if err := validateJournalEvidence(ops, journal, evidence, options); err != nil {
		return err
	}
	if beforeJournalClear != nil {
		beforeJournalClear(markerRel)
	}
	if !evidence.MarkerPresent {
		marker, err := options.captureBytes(ops, markerRel)
		if err != nil || marker.Identity.Exists {
			return transactionError(CodeCleanupFailed, "", "marker-cas", "the transaction clearing marker is not absent", 6)
		}
		if beforeControlWriteRename != nil {
			beforeControlWriteRename(markerRel)
		}
		if failRename != nil {
			if err := failRename(markerRel); err != nil {
				return transactionError(CodeCleanupFailed, "", "marker-rename", "the transaction journal could not enter the clearing state", 6)
			}
		}
		if err := revalidateRenameTarget(ops, marker); err != nil {
			return transactionError(CodeCleanupFailed, "", "marker-final-gate", "the transaction clearing marker appeared before rename", 6)
		}
		if err := ops.Rename(JournalRel(journal.Slug), markerRel); err != nil {
			return transactionError(CodeCleanupFailed, "", "marker-rename", "the transaction journal could not enter the clearing state", 6)
		}
		if err := syncDir(ops, lane); err != nil {
			return transactionError(CodeCleanupFailed, "", "marker-sync", "the transaction clearing marker could not be synchronized", 6)
		}
		evidence.MarkerPresent = true
	}
	if err := validateJournalEvidence(ops, journal, evidence, options); err != nil {
		return err
	}

	if err := removeCanonicalTemps(ops, journal); err != nil {
		return transactionError(CodeCleanupFailed, "", "canonical-temp", "a journal-bound canonical temporary file could not be removed durably", 6)
	}
	if err := removeOwnedStages(ops, journal.Slug); err != nil {
		return transactionError(CodeCleanupFailed, "", "stage", "an owned staging tree could not be removed", 6)
	}
	for _, raw := range []struct {
		id  ArtifactID
		rel string
	}{
		{id: ArtifactArchiveIndex, rel: lane + "/index.preimage.json"},
		{id: ArtifactStatus, rel: lane + "/status.preimage.json"},
	} {
		if err := removeIfPresent(ops, raw.rel); err != nil {
			return transactionError(CodeCleanupFailed, raw.id, "raw-preimage", "an owned raw preimage could not be removed", 6)
		}
	}
	if err := syncDir(ops, lane); err != nil {
		return transactionError(CodeCleanupFailed, "", "lane-sync", "the transaction lane could not be synchronized", 6)
	}

	if err := validateJournalEvidence(ops, journal, evidence, options); err != nil {
		return transactionError(CodeCleanupFailed, "", "marker-final-cas", "the transaction clearing marker changed before final removal", 6)
	}
	if err := ops.Remove(markerRel); err != nil {
		return transactionError(CodeCleanupFailed, "", "marker-remove", "the transaction clearing marker could not be removed", 6)
	}
	if err := syncDir(ops, lane); err != nil {
		if restoreErr := restoreClearingMarker(ops, journal, evidence, options); restoreErr != nil {
			return transactionError(CodeCleanupFailed, "", "marker-final-sync", "the clearing marker removal could not be synchronized or safely re-evidenced", 6)
		}
		return transactionError(CodeCleanupFailed, "", "marker-final-sync", "the clearing marker removal could not be synchronized; durable evidence was restored", 6)
	}
	return nil
}

func validateJournalEvidence(ops RootOps, journal Journal, evidence journalEvidence, options Options) error {
	rel := JournalRel(journal.Slug)
	class := "journal-cas"
	if evidence.MarkerPresent {
		rel = JournalMarkerRel(journal.Slug)
		class = "marker-cas"
	}
	captured, err := options.captureBytes(ops, rel)
	if err != nil || !captured.Identity.Equal(evidence.Identity) || !bytes.Equal(captured.Bytes, evidence.Bytes) {
		if class == "journal-cas" {
			return transactionError(CodeCleanupFailed, "", "journal-cas", "the transaction journal changed before cleanup", 6)
		}
		return transactionError(CodeCleanupFailed, "", "marker-cas", "the transaction clearing marker changed before evidence cleanup", 6)
	}
	return nil
}

func restoreClearingMarker(ops RootOps, journal Journal, evidence journalEvidence, options Options) error {
	_, err := durableWriteRoot(ops, WriteRequest{
		Rel:           JournalMarkerRel(journal.Slug),
		Data:          evidence.Bytes,
		Mode:          0o600,
		TempSuffix:    controlTempSuffix(journal.RunNonce, "marker-restore"),
		Expected:      identityPointer(AbsentIdentity()),
		MismatchCode:  CodeCleanupFailed,
		RequireParent: true,
		Role:          WriteRoleControl,
	}, evidence.Identity, options)
	return err
}

func removeCanonicalTemps(ops RootOps, journal Journal) error {
	synced := make(map[string]bool)
	for _, entry := range journal.Entries {
		rel := canonicalTempRel(journal.RunNonce, entry)
		if err := ops.Remove(rel); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return err
		}
		synced[path.Dir(rel)] = true
	}
	for directory := range synced {
		if err := syncDir(ops, directory); err != nil {
			return err
		}
	}
	return nil
}

func CleanupStaleStages(authority *intentlock.WorkspaceAuthority, slug string, options Options) ([]string, error) {
	return CleanupUnarmedLane(authority, slug, options)
}

// CleanupUnarmedLane removes only owned stage-* trees, lane-writer temps, and
// the two exact raw metadata preimages when no transaction evidence exists.
func CleanupUnarmedLane(authority *intentlock.WorkspaceAuthority, slug string, options Options) ([]string, error) {
	if authority == nil || !validSlug(slug) {
		return nil, transactionError(CodeInvalidPlan, "", "slug", "the feature slug is invalid", 5)
	}
	options, err := options.withScratch()
	if err != nil {
		return nil, err
	}
	removed := []string{}
	err = authority.WithRoot(func(root *os.Root) error {
		ops := options.rootOps(root)
		laneInfo, err := ops.Lstat(laneRel(slug))
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil || refusedFileInfo(laneInfo) || !laneInfo.IsDir() {
			return transactionError(CodeCleanupFailed, "", "lane-open", "the transaction lane could not be inspected", 6)
		}
		for _, controlRel := range []string{JournalRel(slug), JournalMarkerRel(slug)} {
			identity, captureErr := options.capture(ops, controlRel)
			if captureErr != nil {
				return transactionError(CodeJournalPending, "", "stale-cleanup", "transaction evidence owns the staging lane", 6)
			}
			if identity.Exists {
				return transactionError(CodeJournalPending, "", "stale-cleanup", "transaction evidence owns the staging lane", 6)
			}
		}
		names, err := readDirectoryNames(ops, laneRel(slug))
		if err != nil {
			return transactionError(CodeCleanupFailed, "", "lane-read", "the transaction lane could not be inspected", 6)
		}
		for _, name := range names {
			rel := laneRel(slug) + "/" + name
			switch {
			case ownedStageName(name):
				if err := removeTree(ops, rel); err != nil {
					return transactionError(CodeCleanupFailed, "", "stale-stage", "an owned stale staging tree could not be removed", 6)
				}
			case ownedLaneTempName(name):
				if err := removeIfPresent(ops, rel); err != nil {
					return transactionError(CodeCleanupFailed, "", "unarmed-temp", "an owned unarmed temporary file could not be removed", 6)
				}
			default:
				continue
			}
			removed = append(removed, rel)
		}
		for _, name := range []string{"index.preimage.json", "status.preimage.json"} {
			rel := laneRel(slug) + "/" + name
			info, err := ops.Lstat(rel)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil || refusedFileInfo(info) || !info.Mode().IsRegular() {
				return transactionError(CodeCleanupFailed, "", "unarmed-preimage", "an owned unarmed preimage could not be classified", 6)
			}
			if err := removeIfPresent(ops, rel); err != nil {
				return transactionError(CodeCleanupFailed, "", "unarmed-preimage", "an owned unarmed preimage could not be removed", 6)
			}
			removed = append(removed, rel)
		}
		return syncDir(ops, laneRel(slug))
	})
	return removed, err
}

func removeOwnedStages(ops RootOps, slug string) error {
	names, err := readDirectoryNames(ops, laneRel(slug))
	if err != nil {
		return err
	}
	for _, name := range names {
		if ownedStageName(name) {
			if err := removeTree(ops, laneRel(slug)+"/"+name); err != nil {
				return err
			}
		}
	}
	return nil
}

func ownedStageName(name string) bool {
	return strings.HasPrefix(name, "stage-") && validHex(strings.TrimPrefix(name, "stage-"), 12)
}

func ownedLaneTempName(name string) bool {
	for _, base := range []string{
		"journal.json",
		"journal.clearing.json",
		"index.preimage.json",
		"status.preimage.json",
	} {
		prefix := "." + base + ".tmp-"
		if strings.HasPrefix(name, prefix) && validHex(strings.TrimPrefix(name, prefix), 12) {
			return true
		}
	}
	return false
}

func readDirectoryNames(ops RootOps, rel string) ([]string, error) {
	directory, err := ops.Open(rel)
	if err != nil {
		return nil, err
	}
	names, readErr := directory.Readdirnames(-1)
	closeErr := directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return names, nil
}

func removeTree(ops RootOps, rel string) error {
	info, err := ops.Lstat(rel)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.IsDir() || refusedFileInfo(info) {
		if err := ops.Remove(rel); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return syncDir(ops, path.Dir(rel))
	}
	names, err := readDirectoryNames(ops, rel)
	if err != nil {
		return err
	}
	for _, name := range names {
		if name == "." || name == ".." || strings.Contains(name, "/") {
			return fs.ErrInvalid
		}
		if err := removeTree(ops, rel+"/"+name); err != nil {
			return err
		}
	}
	if err := ops.Remove(rel); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return syncDir(ops, path.Dir(rel))
}

func removeIfPresent(ops RootOps, rel string) error {
	if err := ops.Remove(rel); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return syncDir(ops, path.Dir(rel))
}
