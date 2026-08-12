// Crash-recovery journal for `tpatch land` (GH #7 rev-7 F2).
//
// `git commit` advances HEAD before land can publish the resulting
// index. A crash in that gap leaves a new HEAD, the OLD live index and
// possibly a stale lock — a state the operator cannot distinguish from
// "the commit never happened" without help.
//
// So land writes a durable journal before it commits, and every land
// recovers from any journal it finds BEFORE touching record, status or
// the index. Recovery is decided by EVIDENCE, never by the recorded
// phase alone, because the crash can land between HEAD advancing and
// the phase being updated:
//
//	HEAD == pre_head           → the commit did not happen. Publish the
//	                             retained audited pre-commit index, which
//	                             is exactly the staged-retry contract.
//	HEAD advanced, and it is a
//	child of pre_head carrying
//	this slug's binding trailer,
//	and the retained index tree
//	== HEAD's tree              → the commit happened and only the index
//	                             publish is outstanding. Publish it.
//	anything else               → refuse with manual guidance. The
//	                             journal and retained index are kept; no
//	                             guess is made and nothing is
//	                             overwritten.
//
// Recovery is idempotent: a successful pass removes the journal, so the
// next land finds nothing to do.
//
// Storage lives under the already-gitignored `.tpatch/local/` root. The
// journal records repo-relative references wherever possible and never
// records source content or secrets. An absolute path appears only when
// `GIT_INDEX_FILE` genuinely points outside the repository, which
// cannot be expressed relatively.

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

// landJournalVersion is bumped whenever the on-disk shape changes.
// Recovery refuses any other version rather than guessing.
const landJournalVersion = 1

// landJournalDirRel is the gitignored home for journals and retained
// indexes.
const landJournalDirRel = ".tpatch/local/land-journal"

// landJournalFileState records a file's identity without its content.
type landJournalFileState struct {
	Exists bool   `json:"exists"`
	SHA256 string `json:"sha256,omitempty"`
	Mode   uint32 `json:"mode,omitempty"`
}

// landJournal is the versioned, owner-only transaction record.
type landJournal struct {
	Version   int    `json:"version"`
	Slug      string `json:"slug"`
	CreatedAt string `json:"created_at"`
	// Phase is advisory only. Recovery decides from evidence.
	Phase string `json:"phase"`
	// PreHead is HEAD immediately before the commit was attempted.
	PreHead string `json:"pre_head"`
	// LiveIndexRel is the effective index as a repo-relative path when
	// it lives inside the repository; LiveIndexAbs is populated only
	// when it genuinely does not (a redirected GIT_INDEX_FILE).
	LiveIndexRel string               `json:"live_index_rel,omitempty"`
	LiveIndexAbs string               `json:"live_index_abs,omitempty"`
	LiveIndex    landJournalFileState `json:"live_index"`
	// RetainedIndexRel is always repo-relative, inside landJournalDirRel.
	RetainedIndexRel string               `json:"retained_index_rel"`
	RetainedIndex    landJournalFileState `json:"retained_index"`
	// LockRel/LockAbs and LockNonce identify the lock this transaction
	// owned, so a stale lock of ours can be told apart from a foreign
	// one during recovery.
	LockRel   string `json:"lock_rel,omitempty"`
	LockAbs   string `json:"lock_abs,omitempty"`
	LockNonce string `json:"lock_nonce"`
}

const (
	landPhasePreCommit = "pre-commit"
	landPhaseCommitted = "committed"
)

// landJournalDir returns the absolute journal directory for repoRoot.
func landJournalDir(repoRoot string) string {
	return filepath.Join(repoRoot, filepath.FromSlash(landJournalDirRel))
}

// landJournalPath returns the journal file for a slug.
func landJournalPath(repoRoot, slug string) string {
	return filepath.Join(landJournalDir(repoRoot), slug+".json")
}

// retainedIndexRel is the repo-relative retained-index reference.
func retainedIndexRel(slug string) string {
	return landJournalDirRel + "/" + slug + ".index"
}

func journalFileDigest(path string) (string, bool, os.FileMode, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, 0, nil
		}
		return "", false, 0, err
	}
	if !info.Mode().IsRegular() {
		return "", false, 0, fmt.Errorf("%s is not a regular file (mode %s)", path, info.Mode())
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "", false, 0, err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), true, info.Mode().Perm(), nil
}

// relOrAbs renders p as a repo-relative slash path when it lives inside
// repoRoot, so the journal avoids recording machine-specific absolute
// paths unnecessarily.
func relOrAbs(repoRoot, p string) (rel string, abs string) {
	r, err := filepath.Rel(repoRoot, p)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", p
	}
	return filepath.ToSlash(r), ""
}

// resolveJournalPath turns a rel/abs pair back into an absolute path.
func resolveJournalPath(repoRoot, rel, abs string) string {
	if rel != "" {
		return filepath.Join(repoRoot, filepath.FromSlash(rel))
	}
	return abs
}

// writeLandJournal durably persists the journal plus a retained copy of
// the alternate index: retained index first (so a journal never
// references bytes that are not on disk), then the journal, then the
// directory. Every file is owner-only.
func writeLandJournal(repoRoot, slug string, tx *gitutil.IndexTransaction, preHead string) error {
	dir := landJournalDir(repoRoot)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create land journal directory: %w", err)
	}

	retainedAbs := filepath.Join(repoRoot, filepath.FromSlash(retainedIndexRel(slug)))
	tempBody, tempExists, tempMode, err := journalFileBody(tx.TempPath)
	if err != nil {
		return fmt.Errorf("read the alternate index for retention: %w", err)
	}
	retained := landJournalFileState{}
	if tempExists {
		if err := durableWriteOwnerFile(dir, retainedAbs, tempBody, 0o600); err != nil {
			return fmt.Errorf("retain the alternate index: %w", err)
		}
		sum := sha256.Sum256(tempBody)
		retained = landJournalFileState{Exists: true, SHA256: hex.EncodeToString(sum[:]), Mode: uint32(tempMode)}
	} else if err := os.Remove(retainedAbs); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear a stale retained index: %w", err)
	}

	liveState := tx.LiveState()
	liveRel, liveAbs := relOrAbs(repoRoot, tx.LivePath)
	lockRel, lockAbs := relOrAbs(repoRoot, tx.LockPath())
	j := landJournal{
		Version:          landJournalVersion,
		Slug:             slug,
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		Phase:            landPhasePreCommit,
		PreHead:          preHead,
		LiveIndexRel:     liveRel,
		LiveIndexAbs:     liveAbs,
		RetainedIndexRel: retainedIndexRel(slug),
		RetainedIndex:    retained,
		LockRel:          lockRel,
		LockAbs:          lockAbs,
		LockNonce:        tx.LockNonce,
	}
	if liveState != nil {
		j.LiveIndex = landJournalFileState{Exists: liveState.Existed, Mode: uint32(liveState.Mode)}
		if liveState.Existed {
			sum := sha256.Sum256(liveState.Data)
			j.LiveIndex.SHA256 = hex.EncodeToString(sum[:])
		}
	}
	body, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Errorf("encode land journal: %w", err)
	}
	body = append(body, '\n')
	if err := durableWriteOwnerFile(dir, landJournalPath(repoRoot, slug), body, 0o600); err != nil {
		return fmt.Errorf("write land journal: %w", err)
	}
	return nil
}

// updateLandJournalPhase records that the commit succeeded. It is
// advisory: recovery never trusts it alone.
func updateLandJournalPhase(repoRoot, slug, phase string) error {
	path := landJournalPath(repoRoot, slug)
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var j landJournal
	if err := json.Unmarshal(body, &j); err != nil {
		return err
	}
	j.Phase = phase
	out, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return durableWriteOwnerFile(landJournalDir(repoRoot), path, out, 0o600)
}

// clearLandJournal removes the journal and its retained index durably.
// Errors are returned so a caller can surface them.
func clearLandJournal(repoRoot, slug string) error {
	dir := landJournalDir(repoRoot)
	var problems []string
	for _, p := range []string{
		landJournalPath(repoRoot, slug),
		filepath.Join(repoRoot, filepath.FromSlash(retainedIndexRel(slug))),
	} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			problems = append(problems, fmt.Sprintf("removing %s failed: %v", filepath.Base(p), err))
		}
	}
	if _, err := os.Stat(dir); err == nil {
		if err := syncDirPath(dir); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}

// readLandJournal loads and structurally validates a journal. A missing
// journal is (nil, nil): there is nothing to recover.
func readLandJournal(repoRoot, slug string) (*landJournal, error) {
	body, err := os.ReadFile(landJournalPath(repoRoot, slug))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var j landJournal
	if err := json.Unmarshal(body, &j); err != nil {
		return nil, fmt.Errorf("land journal is not valid JSON: %w", err)
	}
	if j.Version != landJournalVersion {
		return nil, fmt.Errorf("land journal version %d is not supported (this build expects %d)", j.Version, landJournalVersion)
	}
	if j.Slug != slug {
		return nil, fmt.Errorf("land journal names slug %q but was read for %q", j.Slug, slug)
	}
	if j.PreHead == "" || j.LockNonce == "" {
		return nil, fmt.Errorf("land journal is missing required fields")
	}
	// Path containment: the retained index must live inside the
	// gitignored journal directory. Anything else is refused rather
	// than read.
	clean := filepath.ToSlash(filepath.Clean(j.RetainedIndexRel))
	if !strings.HasPrefix(clean, landJournalDirRel+"/") || strings.Contains(clean, "..") {
		return nil, fmt.Errorf("land journal retained-index reference %q escapes %s", j.RetainedIndexRel, landJournalDirRel)
	}
	return &j, nil
}

// recoverLand completes or refuses an interrupted land BEFORE the
// caller mutates anything. It returns nil when there was nothing to do.
func recoverLand(repoRoot, slug string, warn func(string, ...any)) error {
	j, err := readLandJournal(repoRoot, slug)
	if err != nil {
		return fmt.Errorf("land refuses: a previous land left a journal that cannot be read: %v.\n"+
			"Inspect %s and remove it once you have confirmed your index and HEAD are correct", err, landJournalDirRel)
	}
	if j == nil {
		return nil
	}

	// The effective index must still be the one the journal describes;
	// otherwise the retained bytes belong to a different index.
	livePath, err := gitutil.EffectiveIndexPath(repoRoot)
	if err != nil {
		return err
	}
	journalLive := resolveJournalPath(repoRoot, j.LiveIndexRel, j.LiveIndexAbs)
	if journalLive != "" && journalLive != livePath {
		return landRecoveryRefusal(j, fmt.Sprintf(
			"the journal was written for index %q but the effective index is now %q", journalLive, livePath))
	}

	retainedAbs := filepath.Join(repoRoot, filepath.FromSlash(j.RetainedIndexRel))
	if j.RetainedIndex.Exists {
		sum, exists, _, herr := journalFileDigest(retainedAbs)
		if herr != nil {
			return landRecoveryRefusal(j, fmt.Sprintf("the retained index could not be read: %v", herr))
		}
		if !exists || sum != j.RetainedIndex.SHA256 {
			return landRecoveryRefusal(j, "the retained index is missing or its checksum does not match the journal")
		}
	}

	head, herr := gitutil.HeadCommit(repoRoot)
	if herr != nil {
		return landRecoveryRefusal(j, fmt.Sprintf("HEAD could not be read: %v", herr))
	}

	switch {
	case head == j.PreHead:
		// The commit never advanced HEAD. The retained index is the
		// audited pre-commit state: publish it, which is exactly the
		// existing staged-retry contract. The landed-at note written
		// before the commit stays, consistent with that contract.
		warn("recovering an interrupted `tpatch land %s`: the commit did not complete; restoring the audited staged index for retry\n", slug)
	case landCommitBindsSlug(repoRoot, head, j.PreHead, slug):
		if !j.RetainedIndex.Exists {
			return landRecoveryRefusal(j, "HEAD advanced but no retained index was recorded")
		}
		if !retainedTreeMatchesHead(repoRoot, retainedAbs, head) {
			return landRecoveryRefusal(j, "HEAD advanced but the retained index does not describe the committed tree")
		}
		warn("recovering an interrupted `tpatch land %s`: the commit completed as %s; publishing its index\n", slug, abbrevSHA(head))
	default:
		return landRecoveryRefusal(j, fmt.Sprintf(
			"HEAD is %s, which is neither the pre-land commit %s nor a land commit for this feature", abbrevSHA(head), abbrevSHA(j.PreHead)))
	}

	// Publish the retained index and clean up. Both branches publish
	// the same file; only the diagnostic above differs.
	if j.RetainedIndex.Exists {
		body, err := os.ReadFile(retainedAbs)
		if err != nil {
			return landRecoveryRefusal(j, fmt.Sprintf("the retained index could not be read: %v", err))
		}
		mode := os.FileMode(j.LiveIndex.Mode)
		if mode == 0 {
			mode = 0o644
		}
		if err := durableWriteOwnerFile(filepath.Dir(livePath), livePath, body, mode); err != nil {
			return landRecoveryRefusal(j, fmt.Sprintf("publishing the retained index failed: %v", err))
		}
	}
	var problems []string
	if err := removeOwnedLandLock(repoRoot, j); err != nil {
		problems = append(problems, err.Error())
	}
	if err := clearLandJournal(repoRoot, slug); err != nil {
		problems = append(problems, err.Error())
	}
	if len(problems) > 0 {
		return fmt.Errorf("land recovery completed but cleanup failed: %s", strings.Join(problems, "; "))
	}
	return nil
}

// removeOwnedLandLock removes a stale `<index>.lock` only when it
// carries this transaction's nonce. A foreign lock is left alone and
// reported.
func removeOwnedLandLock(repoRoot string, j *landJournal) error {
	lockPath := resolveJournalPath(repoRoot, j.LockRel, j.LockAbs)
	if lockPath == "" {
		return nil
	}
	nonce, ours, err := gitutil.LockNonceAt(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading the index lock %q failed: %v", lockPath, err)
	}
	if !ours || nonce != j.LockNonce {
		return fmt.Errorf("the index lock %q belongs to another process; it was left untouched", lockPath)
	}
	if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing our stale index lock %q failed: %v", lockPath, err)
	}
	return nil
}

// landCommitBindsSlug reports whether `head` is exactly one commit past
// `preHead` and carries this feature's binding trailer.
func landCommitBindsSlug(repoRoot, head, preHead, slug string) bool {
	if head == "" || head == preHead {
		return false
	}
	parent, err := runGit(repoRoot, "rev-parse", head+"^")
	if err != nil || strings.TrimSpace(parent) != preHead {
		return false
	}
	msg, err := runGit(repoRoot, "log", "-1", "--pretty=%B", head)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(msg, "\n") {
		if strings.TrimSpace(line) == "Tpatch-Feature: "+slug {
			return true
		}
	}
	return false
}

// retainedTreeMatchesHead reports whether the retained alternate index
// describes exactly the tree that `head` committed.
func retainedTreeMatchesHead(repoRoot, retainedAbs, head string) bool {
	env := append(os.Environ(), "GIT_INDEX_FILE="+retainedAbs)
	indexTree, err := runGitEnvOut(repoRoot, env, "write-tree")
	if err != nil {
		return false
	}
	headTree, err := runGit(repoRoot, "rev-parse", head+"^{tree}")
	if err != nil {
		return false
	}
	return strings.TrimSpace(indexTree) == strings.TrimSpace(headTree)
}

// landRecoveryRefusal renders the manual-recovery guidance and keeps
// every artifact in place: nothing is guessed at or overwritten.
func landRecoveryRefusal(j *landJournal, why string) error {
	return fmt.Errorf(
		"land refuses: a previous `tpatch land %s` was interrupted and cannot be recovered automatically (%s).\n"+
			"Nothing has been changed. The evidence is preserved:\n"+
			"  journal        : %s/%s.json\n"+
			"  retained index : %s\n"+
			"Recover by hand: confirm whether the landing commit exists (`git log --grep '^Tpatch-Feature: %s$'`),\n"+
			"reset or keep your index as appropriate, then delete those two files to clear the journal",
		j.Slug, why, landJournalDirRel, j.Slug, j.RetainedIndexRel, j.Slug)
}

// sha256FileBody reads a file returning its bytes, existence and mode.
func journalFileBody(path string) ([]byte, bool, os.FileMode, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, 0, nil
		}
		return nil, false, 0, err
	}
	if !info.Mode().IsRegular() {
		return nil, false, 0, fmt.Errorf("%s is not a regular file (mode %s)", path, info.Mode())
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, false, 0, err
	}
	return body, true, info.Mode().Perm(), nil
}

// durableWriteOwnerFile writes `data` at `target` via an O_EXCL temp in
// `dir`, fsyncing the file and the directory, so the result survives a
// crash and is never observed partially written.
func durableWriteOwnerFile(dir, target string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tpatch-durable-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	fail := func(e error) error {
		_ = tmp.Close()
		if rerr := os.Remove(tmpPath); rerr != nil && !os.IsNotExist(rerr) {
			return fmt.Errorf("%w; additionally removing %q failed: %v", e, tmpPath, rerr)
		}
		return e
	}
	if _, err := tmp.Write(data); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, target); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return syncDirPath(dir)
}

// syncDirPath fsyncs a directory so a rename inside it is durable.
func syncDirPath(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open directory %q for fsync: %w", dir, err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("fsync directory %q: %w", dir, err)
	}
	return nil
}
