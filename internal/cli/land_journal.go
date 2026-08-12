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
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

// landJournalVersion is bumped whenever the on-disk shape changes.
// Recovery refuses any other version rather than guessing.
const landJournalVersion = 3

// landJournalDirRel is the gitignored home for journals and the durable
// retained index.
const landJournalDirRel = ".tpatch/local/land-journal"

// landJournalFileState records a file's identity without its content.
type landJournalFileState struct {
	Exists bool   `json:"exists"`
	SHA256 string `json:"sha256,omitempty"`
	Mode   uint32 `json:"mode,omitempty"`
}

func (a landJournalFileState) matches(b landJournalFileState) bool {
	if a.Exists != b.Exists {
		return false
	}
	if !a.Exists {
		return true
	}
	return a.SHA256 == b.SHA256 && a.Mode == b.Mode
}

// landJournal is the versioned, owner-only transaction record.
//
// EVIDENCE MATRIX (GH #7 rev-8). Recovery decides from two independent
// axes, never from `Phase`, which is advisory and kept for forensics:
//
//	LIVE INDEX (compared under the live lock)
//	  preimage    live == LiveIndexPre           → safe to publish retained
//	  postimage   live == the retained identity  → already published; clean only
//	  divergent   neither                        → operator activity or tamper; REFUSE
//
//	HEAD
//	  HEAD == PreHead                            → commit never happened; retained
//	                                               is the audited staged-retry index
//	  HEAD is a direct child of PreHead carrying
//	  `Tpatch-Feature: <slug>` AND the retained
//	  index's write-tree == HEAD's tree          → commit completed; publish
//	  anything else                              → REFUSE
//
// The retained index is the SAME file Git staged, audited, hooked and
// committed against, so a hook's edits are part of the evidence. Its
// bytes may therefore legitimately differ from the pre-commit checksum
// when a crash landed after a hook ran but before the journal was
// updated. That transition is accepted ONLY when path containment holds
// and the retained tree matches the child HEAD tree exactly; arbitrary
// tampering fails both checks.
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
	LiveIndexRel string `json:"live_index_rel,omitempty"`
	LiveIndexAbs string `json:"live_index_abs,omitempty"`
	// LiveIndexPre is the canonical preimage identity: only an index
	// matching it byte-for-byte may be overwritten.
	LiveIndexPre landJournalFileState `json:"live_index_pre"`
	// RetainedIndexRel is always repo-relative, inside landJournalDirRel.
	RetainedIndexRel string `json:"retained_index_rel"`
	// RetainedPre is the retained index identity as journalled before
	// the commit; RetainedPost is refreshed after the commit (and any
	// hook) returns, and is what a later publish must match.
	RetainedPre      landJournalFileState  `json:"retained_pre"`
	RetainedPreTree  string                `json:"retained_pre_tree,omitempty"`
	RetainedPost     *landJournalFileState `json:"retained_post,omitempty"`
	RetainedPostTree string                `json:"retained_post_tree,omitempty"`
	// Lock OWNERSHIP evidence only. GH #7 rev-9 F2: the journal must
	// never name a filesystem path that cleanup will act on. The lock
	// path is DERIVED at use time as the validated effective index path
	// plus ".lock"; a tampered journal therefore cannot point lock
	// removal at an arbitrary nonce-bearing victim file.
	LockNonce string `json:"lock_nonce"`
	LockIno   uint64 `json:"lock_ino,omitempty"`
}

// retainedIdentity returns the identity a publish must match: the
// post-commit one when it exists, otherwise the pre-commit one.
func (j *landJournal) retainedIdentity() landJournalFileState {
	if j.RetainedPost != nil {
		return *j.RetainedPost
	}
	return j.RetainedPre
}

const (
	landPhasePreCommit = "pre-commit"
	landPhaseCommitted = "committed"
	landPhaseFailed    = "commit-failed"
)

func landJournalDir(repoRoot string) string {
	return filepath.Join(repoRoot, filepath.FromSlash(landJournalDirRel))
}

func landJournalPath(repoRoot, slug string) string {
	return filepath.Join(landJournalDir(repoRoot), slug+".json")
}

// retainedIndexRel is the repo-relative retained-index reference. This
// file IS the alternate index land stages and commits against.
func retainedIndexRel(slug string) string {
	return landJournalDirRel + "/" + slug + ".index"
}

func retainedIndexAbs(repoRoot, slug string) string {
	return filepath.Join(repoRoot, filepath.FromSlash(retainedIndexRel(slug)))
}

// journalFileIdentity hashes a file and reports its identity.
func journalFileIdentity(path string) (landJournalFileState, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return landJournalFileState{}, nil
		}
		return landJournalFileState{}, err
	}
	if !info.Mode().IsRegular() {
		return landJournalFileState{}, fmt.Errorf("%s is not a regular file (mode %s)", path, info.Mode())
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return landJournalFileState{}, err
	}
	sum := sha256.Sum256(body)
	return landJournalFileState{Exists: true, SHA256: hex.EncodeToString(sum[:]), Mode: uint32(info.Mode().Perm())}, nil
}

// indexTree returns `write-tree` for an alternate index file.
//
// IMPORTANT: `git write-tree` rewrites the index in place to store the
// cache-tree extension, so it MUST run before the file is hashed.
// Hashing first and computing the tree afterwards silently records a
// checksum the file no longer has.
func indexTree(repoRoot, indexPath string) (string, error) {
	env := append(os.Environ(), "GIT_INDEX_FILE="+indexPath)
	out, err := runGitEnvOut(repoRoot, env, "write-tree")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
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

func resolveJournalPath(repoRoot, rel, abs string) string {
	if rel != "" {
		return filepath.Join(repoRoot, filepath.FromSlash(rel))
	}
	return abs
}

// writeLandJournal durably persists the pre-commit journal describing
// the live preimage and the retained alternate index (which the caller
// has already built in place).
func writeLandJournal(repoRoot, slug string, tx *gitutil.IndexTransaction, preHead string) error {
	dir := landJournalDir(repoRoot)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create land journal directory: %w", err)
	}
	// Tree first: write-tree rewrites the index, so the hash must be
	// taken afterwards or it will not describe the file on disk.
	tree := ""
	if _, statErr := os.Lstat(tx.TempPath); statErr == nil {
		var terr error
		if tree, terr = indexTree(repoRoot, tx.TempPath); terr != nil {
			return fmt.Errorf("compute the retained index tree: %w", terr)
		}
	}
	if err := tx.SyncAlternateIndex(); err != nil {
		return err
	}
	retained, err := journalFileIdentity(tx.TempPath)
	if err != nil {
		return fmt.Errorf("hash the retained index: %w", err)
	}

	liveState := tx.LiveState()
	liveRel, liveAbs := relOrAbs(repoRoot, tx.LivePath)
	lockIno, _ := gitutil.FileIno(tx.LockPath())
	j := landJournal{
		Version:          landJournalVersion,
		Slug:             slug,
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		Phase:            landPhasePreCommit,
		PreHead:          preHead,
		LiveIndexRel:     liveRel,
		LiveIndexAbs:     liveAbs,
		RetainedIndexRel: retainedIndexRel(slug),
		RetainedPre:      retained,
		RetainedPreTree:  tree,
		LockNonce:        tx.LockNonce,
		LockIno:          lockIno,
	}
	if liveState != nil {
		j.LiveIndexPre = landJournalFileState{Exists: liveState.Existed, Mode: uint32(liveState.Mode)}
		if liveState.Existed {
			sum := sha256.Sum256(liveState.Data)
			j.LiveIndexPre.SHA256 = hex.EncodeToString(sum[:])
		}
	}
	return writeJournalStruct(repoRoot, slug, &j)
}

// refreshLandJournalAfterCommit re-reads the retained index AFTER the
// commit (and any hook that mutated it) and durably records the
// post-commit identity and tree BEFORE any live publish is attempted.
func refreshLandJournalAfterCommit(repoRoot, slug string, tx *gitutil.IndexTransaction, phase string) error {
	j, err := readLandJournal(repoRoot, slug)
	if err != nil {
		return err
	}
	if j == nil {
		return fmt.Errorf("land journal disappeared before it could be refreshed")
	}
	// Tree first (write-tree rewrites the index), then fsync, then hash.
	tree := ""
	if _, statErr := os.Lstat(tx.TempPath); statErr == nil {
		var terr error
		if tree, terr = indexTree(repoRoot, tx.TempPath); terr != nil {
			return fmt.Errorf("compute the retained index tree: %w", terr)
		}
	}
	if err := tx.SyncAlternateIndex(); err != nil {
		return err
	}
	post, err := journalFileIdentity(tx.TempPath)
	if err != nil {
		return fmt.Errorf("hash the retained index: %w", err)
	}
	j.RetainedPost = &post
	j.RetainedPostTree = tree
	j.Phase = phase
	return writeJournalStruct(repoRoot, slug, j)
}

func writeJournalStruct(repoRoot, slug string, j *landJournal) error {
	body, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Errorf("encode land journal: %w", err)
	}
	body = append(body, '\n')
	dir := landJournalDir(repoRoot)
	if err := gitutil.DurableWriteFile(dir, landJournalPath(repoRoot, slug), body, 0o600); err != nil {
		return fmt.Errorf("write land journal: %w", err)
	}
	return nil
}

// clearLandJournal removes the journal and the retained index durably.
// Callers MUST only invoke it once the live index has been published
// (or is already the published postimage): the retained index is the
// only copy of the staged-retry evidence.
func clearLandJournal(repoRoot, slug string) error {
	dir := landJournalDir(repoRoot)
	var problems []string
	for _, p := range []string{landJournalPath(repoRoot, slug), retainedIndexAbs(repoRoot, slug)} {
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
	// Strict decoding: an unknown field means the file is not a journal
	// this build wrote. Refusing beats silently ignoring a `lock_abs`
	// somebody added hoping cleanup would act on it.
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var j landJournal
	if err := dec.Decode(&j); err != nil {
		return nil, fmt.Errorf("land journal is not a valid journal document: %w", err)
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
	if err := validateContainedRelPath(repoRoot, j.RetainedIndexRel); err != nil {
		return nil, err
	}
	return &j, nil
}

// validateContainedRelPath refuses any journal-supplied relative path
// that escapes the journal directory, is not a regular file, or reaches
// it through a symlinked component. The journal is untrusted input.
func validateContainedRelPath(repoRoot, rel string) error {
	if rel == "" {
		return fmt.Errorf("land journal is missing its retained-index reference")
	}
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") {
		return fmt.Errorf("land journal reference %q must be repo-relative", rel)
	}
	clean := filepath.ToSlash(filepath.Clean(rel))
	if !strings.HasPrefix(clean, landJournalDirRel+"/") || strings.Contains(clean, "..") {
		return fmt.Errorf("land journal retained-index reference %q escapes %s", rel, landJournalDirRel)
	}
	abs := filepath.Join(repoRoot, filepath.FromSlash(clean))
	// No component between the journal root and the file may be a
	// symlink, and the file itself must be a regular file when present.
	root := landJournalDir(repoRoot)
	cur := abs
	for {
		info, err := os.Lstat(cur)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("land journal reference %q passes through a symlink (%s); refusing", rel, cur)
			}
			if cur == abs && !info.Mode().IsRegular() && !info.IsDir() {
				return fmt.Errorf("land journal reference %q is not a regular file (mode %s)", rel, info.Mode())
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur || cur == root {
			return nil
		}
		cur = parent
	}
}

// liveClass is how recovery classifies the live index under the lock.
type liveClass int

const (
	liveDivergent liveClass = iota
	livePreimage
	livePostimage
)

func classifyLive(j *landJournal, current landJournalFileState) liveClass {
	switch {
	case current.matches(j.LiveIndexPre):
		return livePreimage
	case current.matches(j.retainedIdentity()):
		return livePostimage
	default:
		return liveDivergent
	}
}

// recoverLand completes or refuses an interrupted land BEFORE the
// caller mutates anything. It returns nil when there was nothing to do.
//
// Everything happens under the live index lock, and the live index is
// compared to the journalled preimage byte-for-byte before it can be
// overwritten. A live index that is neither the preimage nor the
// already-published postimage means the operator (or something else)
// touched it: recovery refuses and preserves every artifact.
func recoverLand(repoRoot, slug string, warn func(string, ...any)) error {
	j, err := readLandJournal(repoRoot, slug)
	if err != nil {
		return fmt.Errorf("land refuses: a previous land left a journal that cannot be read: %v.\n"+
			"Inspect %s and remove it once you have confirmed your index and HEAD are correct", err, landJournalDirRel)
	}
	if j == nil {
		return nil
	}

	livePath, err := gitutil.EffectiveIndexPath(repoRoot)
	if err != nil {
		return err
	}
	journalLive := resolveJournalPath(repoRoot, j.LiveIndexRel, j.LiveIndexAbs)
	if journalLive != "" && journalLive != livePath {
		return landRecoveryRefusal(j, fmt.Sprintf(
			"the journal was written for index %q but the effective index is now %q", journalLive, livePath))
	}

	// Take the live lock before reading, comparing or publishing
	// anything. The path is DERIVED from the validated effective index,
	// never read from the journal (rev-9 F2). A stale lock of OURS is
	// removed first, but only after nonce and (where the platform
	// exposes one) inode both match.
	lockPath := livePath + ".lock"
	if _, statErr := os.Lstat(lockPath); statErr == nil {
		if err := removeOwnedLandLock(lockPath, j); err != nil {
			return landRecoveryRefusal(j, err.Error())
		}
	}
	// NOTE: recovery deliberately does NOT open an IndexTransaction —
	// that would re-seed the alternate index from the live one and
	// destroy the retained evidence. It uses its own minimal lock.
	return recoverUnderLock(repoRoot, slug, j, livePath, warn)
}

// recoverUnderLock performs the locked half of recovery.
func recoverUnderLock(repoRoot, slug string, j *landJournal, livePath string, warn func(string, ...any)) error {
	lock, err := acquireRecoveryLock(livePath)
	if err != nil {
		return landRecoveryRefusal(j, err.Error())
	}
	release := func() error { return lock.release() }

	fail := func(why string) error {
		relErr := release()
		out := landRecoveryRefusal(j, why)
		if relErr != nil {
			return fmt.Errorf("%w\nadditionally: %v", out, relErr)
		}
		return out
	}

	retainedAbs := retainedIndexAbs(repoRoot, slug)
	retainedNow, err := journalFileIdentity(retainedAbs)
	if err != nil {
		return fail(fmt.Sprintf("the retained index could not be read: %v", err))
	}

	liveNow, err := journalFileIdentity(livePath)
	if err != nil {
		return fail(fmt.Sprintf("the live index could not be read: %v", err))
	}
	class := classifyLive(j, liveNow)
	if class == liveDivergent {
		return fail("your Git index no longer matches the state recorded before the interrupted land " +
			"(it is neither the pre-land index nor the index that land was about to publish), " +
			"so something else has staged or reset since the crash")
	}

	head, herr := gitutil.HeadCommit(repoRoot)
	if herr != nil {
		return fail(fmt.Sprintf("HEAD could not be read: %v", herr))
	}

	// Evidence axis 2: HEAD.
	switch {
	case head == j.PreHead:
		if !retainedNow.Exists {
			return fail("the commit did not complete and no retained index survives")
		}
		// The retained bytes may differ from the pre-commit checksum
		// when a hook mutated the alternate index; that is legitimate
		// only if the identity matches what the journal last recorded.
		if !retainedNow.matches(j.retainedIdentity()) {
			return fail("the retained index checksum no longer matches the journal (it was changed after the crash)")
		}
		warn("recovering an interrupted `tpatch land %s`: the commit did not complete; restoring the audited staged index for retry\n", slug)
	case landCommitBindsSlug(repoRoot, head, j.PreHead, slug):
		if !retainedNow.Exists {
			return fail("HEAD advanced but no retained index survives")
		}
		// write-tree rewrites the index in place, so the identity is
		// re-taken afterwards.
		retainedTree, terr := indexTree(repoRoot, retainedAbs)
		if terr != nil {
			return fail(fmt.Sprintf("the retained index tree could not be computed: %v", terr))
		}
		headTree, terr := runGit(repoRoot, "rev-parse", head+"^{tree}")
		if terr != nil || strings.TrimSpace(headTree) != retainedTree {
			return fail("HEAD advanced but the retained index does not describe the committed tree")
		}
		if retainedNow, err = journalFileIdentity(retainedAbs); err != nil {
			return fail(fmt.Sprintf("the retained index could not be re-read: %v", err))
		}
		if !retainedNow.matches(j.retainedIdentity()) {
			// A crash after HEAD advanced but before the journal was
			// refreshed. The tree check above is the authority; persist
			// the observed identity so the state is self-consistent.
			j.RetainedPost = &retainedNow
			j.RetainedPostTree = retainedTree
			j.Phase = landPhaseCommitted
			if werr := writeJournalStruct(repoRoot, slug, j); werr != nil {
				return fail(fmt.Sprintf("recording the post-commit retained identity failed: %v", werr))
			}
		}
		warn("recovering an interrupted `tpatch land %s`: the commit completed as %s; publishing its index\n", slug, abbrevSHA(head))
	default:
		return fail(fmt.Sprintf(
			"HEAD is %s, which is neither the pre-land commit %s nor a land commit for this feature",
			abbrevSHA(head), abbrevSHA(j.PreHead)))
	}

	// GH #7 rev-9: never publish a retained index that contains a
	// nested worktree. This is reachable when a hook contaminated the
	// commit and the automatic rollback could not complete.
	if retainedNow.Exists {
		env := append(os.Environ(), "GIT_INDEX_FILE="+retainedAbs)
		contaminated, aerr := gitutil.AuditIndexEntriesForNestedWorktreesEnv(repoRoot, env)
		if aerr != nil {
			return fail(fmt.Sprintf("the retained index could not be audited for nested worktrees: %v", aerr))
		}
		if len(contaminated) > 0 {
			return fail(fmt.Sprintf(
				"the retained index contains path(s) inside a registered nested Git worktree (%s); publishing it would stage a gitlink",
				strings.Join(contaminated, ", ")))
		}
	}

	// Publish only when the live index is still the preimage. A
	// postimage means a previous recovery already published it, so this
	// pass is pure cleanup — idempotent, with no rewrite.
	if class == livePreimage {
		body, rerr := os.ReadFile(retainedAbs)
		if rerr != nil {
			return fail(fmt.Sprintf("the retained index could not be read: %v", rerr))
		}
		mode := os.FileMode(j.LiveIndexPre.Mode)
		if mode == 0 {
			mode = 0o644
		}
		liveDir := filepath.Dir(livePath)
		if resolved, eerr := filepath.EvalSymlinks(liveDir); eerr == nil {
			liveDir = resolved
		}
		if perr := gitutil.DurableWriteFile(liveDir, livePath, body, mode); perr != nil {
			return fail(fmt.Sprintf("publishing the retained index failed: %v", perr))
		}
	} else {
		warn("the index was already published by an earlier recovery of `tpatch land %s`; clearing the journal\n", slug)
	}

	// Clear only after a successful publish (or a confirmed postimage).
	var problems []string
	if err := clearLandJournal(repoRoot, slug); err != nil {
		problems = append(problems, err.Error())
	}
	if err := release(); err != nil {
		problems = append(problems, err.Error())
	}
	if len(problems) > 0 {
		return fmt.Errorf("land recovery completed but cleanup failed: %s", strings.Join(problems, "; "))
	}
	return nil
}

// recoveryLock is a minimal owned lock used by recovery.
type recoveryLock struct {
	path string
	f    *os.File
	dir  string
}

func acquireRecoveryLock(livePath string) (*recoveryLock, error) {
	lockPath := livePath + ".lock"
	dir := filepath.Dir(livePath)
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("the Git index lock %q is held by another process", lockPath)
		}
		return nil, fmt.Errorf("acquiring the Git index lock %q failed: %v", lockPath, err)
	}
	nonce, nerr := recoveryNonce()
	if nerr == nil {
		_, _ = f.WriteString("tpatch-land-lock " + nonce + "\n")
		_ = f.Sync()
	}
	return &recoveryLock{path: lockPath, f: f, dir: dir}, nil
}

func (l *recoveryLock) release() error {
	if l == nil {
		return nil
	}
	if l.f != nil {
		_ = l.f.Close()
		l.f = nil
	}
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("releasing the Git index lock %q failed: %v", l.path, err)
	}
	return syncDirPath(l.dir)
}

func recoveryNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// removeOwnedLandLock removes a stale `<index>.lock` only when it
// carries this transaction's nonce AND, where the platform exposes one,
// the recorded inode. A foreign lock is left alone and reported.
func removeOwnedLandLock(lockPath string, j *landJournal) error {
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
	if j.LockIno != 0 {
		if ino, ok := gitutil.FileIno(lockPath); ok && ino != j.LockIno {
			return fmt.Errorf("the index lock %q was recreated by another process since the crash; it was left untouched", lockPath)
		}
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

// contaminatedRollbackPending renders the state where a commit hook
// staged a nested worktree, the commit SUCCEEDED, and the automatic
// rollback could not be completed.
//
// Nothing is guessed at: the journal and retained index stay in place so
// a later recovery (or the operator) can resolve it, and only a lock we
// safely own is released. The diagnostic never claims the feature
// landed, because the commit that exists contains a gitlink tpatch
// refuses to publish.
func contaminatedRollbackPending(cmd *cobra.Command, tx *gitutil.IndexTransaction, slug string, contaminated []string, why string) error {
	var extra []string
	if cerr := tx.Close(); cerr != nil {
		extra = append(extra, fmt.Sprintf("cleaning up the alternate index failed: %v", cerr))
	}
	msg := fmt.Sprintf(
		"land: contaminated commit, manual recovery pending — a commit hook staged path(s) inside a registered nested Git worktree "+
			"and the commit succeeded, but %s.\n"+
			"Contaminated path(s): %s\n"+
			"The feature has NOT landed: nothing was published to your index. HEAD may still point at the contaminated commit.\n"+
			"Resolve by hand: inspect `git log -1`, and if the top commit is the contaminated landing, run `git reset --soft HEAD^`.\n"+
			"Then remove the nested worktree (`git worktree remove <path>`), delete %s/%s.json and its .index sibling, and re-run `tpatch land %s`",
		why, strings.Join(contaminated, ", "), landJournalDirRel, slug, slug)
	if len(extra) > 0 {
		return fmt.Errorf("%s\nadditionally: %s", msg, strings.Join(extra, "; "))
	}
	return fmt.Errorf("%s", msg)
}
