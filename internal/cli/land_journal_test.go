// Crash-recovery tests for `tpatch land` (GH #7 rev-7 F2).
//
// Crash phases are exercised by CONSTRUCTING the durable state a crash
// would have left — a journal, a retained alternate index, an optional
// stale lock, and a HEAD at one of the possible positions — and then
// invoking recovery. That is deterministic and needs no process
// killing, and it tests exactly the evidence recovery reasons about.

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

func jGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %s: %v", args, dir, out, err)
	}
	return string(out)
}

func noWarn(string, ...any) {}

// crashState is the durable residue a crashed land leaves behind.
type crashState struct {
	repoRoot string
	slug     string
	preHead  string
	// retained is the audited alternate index we pretend land had
	// built; live is the operator index bytes at Begin time.
	retainedPath string
	livePath     string
	journalPath  string
	lockPath     string
	nonce        string
	// livePre/liveMode are the exact preimage bytes and mode, so a test
	// can restore them without going through git.
	livePre  []byte
	liveMode os.FileMode
}

// buildCrashState produces a repo whose live index differs from a
// retained alternate index, plus a valid journal describing it.
func buildCrashState(t *testing.T, withLock bool) *crashState {
	t.Helper()
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	preHead := gitHead(t, tmpDir)

	livePath := liveIndexPath(t, tmpDir)
	// Build the "audited" alternate index: everything land would stage.
	tx, err := gitutil.BeginIndexTransaction(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Close()
	jGit(t, tmpDir, "-c", "core.quotePath=false", "status", "--porcelain")
	addCmd := exec.Command("git", "add", "--", "README.md", "internal/example.go", ".tpatch")
	addCmd.Dir = tmpDir
	addCmd.Env = tx.Env()
	if out, aerr := addCmd.CombinedOutput(); aerr != nil {
		t.Fatalf("stage into the alternate index: %s: %v", out, aerr)
	}
	retainedBytes, err := os.ReadFile(tx.TempPath)
	if err != nil {
		t.Fatal(err)
	}

	dir := landJournalDir(tmpDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	retainedPath := filepath.Join(tmpDir, filepath.FromSlash(retainedIndexRel(slug)))
	if err := os.WriteFile(retainedPath, retainedBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	liveBytes, _ := os.ReadFile(livePath)
	liveSum := sha256.Sum256(liveBytes)
	liveMode := os.FileMode(0o644)
	if info, serr := os.Stat(livePath); serr == nil {
		liveMode = info.Mode().Perm()
	}
	// write-tree rewrites the index, so the tree is taken BEFORE the
	// checksum — mirroring production.
	retainedTree, err := indexTree(tmpDir, retainedPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(retainedPath, 0o600); err != nil {
		t.Fatal(err)
	}
	retainedBytes, err = os.ReadFile(retainedPath)
	if err != nil {
		t.Fatal(err)
	}
	retSum := sha256.Sum256(retainedBytes)

	nonce := "deadbeefdeadbeefdeadbeefdeadbeef"
	lockPath := livePath + ".lock"
	if withLock {
		if err := os.WriteFile(lockPath, []byte("tpatch-land-lock "+nonce+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	liveRel, liveAbs := relOrAbs(tmpDir, livePath)
	lockIno, _ := gitutil.FileIno(lockPath)
	j := landJournal{
		Version:          landJournalVersion,
		Slug:             slug,
		CreatedAt:        "2026-08-12T00:00:00Z",
		Phase:            landPhasePreCommit,
		PreHead:          preHead,
		LiveIndexRel:     liveRel,
		LiveIndexAbs:     liveAbs,
		LiveIndexPre:     landJournalFileState{Exists: len(liveBytes) > 0, SHA256: hex.EncodeToString(liveSum[:]), Mode: uint32(liveMode)},
		RetainedIndexRel: retainedIndexRel(slug),
		RetainedPre:      landJournalFileState{Exists: true, SHA256: hex.EncodeToString(retSum[:]), Mode: 0o600},
		RetainedPreTree:  retainedTree,
		LockNonce:        nonce,
		LockIno:          lockIno,
	}
	body, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	journalPath := landJournalPath(tmpDir, slug)
	if err := os.WriteFile(journalPath, append(body, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return &crashState{
		repoRoot: tmpDir, slug: slug, preHead: preHead,
		retainedPath: retainedPath, livePath: livePath,
		journalPath: journalPath, lockPath: lockPath, nonce: nonce,
		livePre: liveBytes, liveMode: liveMode,
	}
}

// mutateJournal rewrites one field of an existing journal.
func mutateJournal(t *testing.T, cs *crashState, f func(*landJournal)) {
	t.Helper()
	body, err := os.ReadFile(cs.journalPath)
	if err != nil {
		t.Fatal(err)
	}
	var j landJournal
	if err := json.Unmarshal(body, &j); err != nil {
		t.Fatal(err)
	}
	f(&j)
	out, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cs.journalPath, append(out, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

// No journal at all: recovery is a no-op.
func TestRecoverLandNoJournalIsNoOp(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if err := recoverLand(tmpDir, slug, noWarn); err != nil {
		t.Fatalf("recovery with no journal must be a no-op: %v", err)
	}
}

// Crash BEFORE the commit: HEAD is still pre-HEAD, so the retained
// audited index is published as the staged-retry contract, and the
// journal plus our stale lock are cleaned.
func TestRecoverLandCrashBeforeCommitPublishesStagedRetry(t *testing.T) {
	cs := buildCrashState(t, true)
	retained, err := os.ReadFile(cs.retainedPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := recoverLand(cs.repoRoot, cs.slug, noWarn); err != nil {
		t.Fatalf("recovery: %v", err)
	}
	live, err := os.ReadFile(cs.livePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(live) != string(retained) {
		t.Error("the retained audited index was not published")
	}
	cached := jGit(t, cs.repoRoot, "diff", "--cached", "--name-only")
	for _, want := range []string{"README.md", "internal/example.go"} {
		if !strings.Contains(cached, want) {
			t.Errorf("staged retry state missing %q:\n%s", want, cached)
		}
	}
	if got := gitHead(t, cs.repoRoot); got != cs.preHead {
		t.Errorf("recovery advanced HEAD: %s -> %s", cs.preHead, got)
	}
	if _, err := os.Stat(cs.journalPath); !os.IsNotExist(err) {
		t.Errorf("journal residue: %v", err)
	}
	if _, err := os.Stat(cs.retainedPath); !os.IsNotExist(err) {
		t.Errorf("retained index residue: %v", err)
	}
	if _, err := os.Stat(cs.lockPath); !os.IsNotExist(err) {
		t.Errorf("our stale lock was not removed: %v", err)
	}

	// Idempotent: a second pass finds nothing to do.
	if err := recoverLand(cs.repoRoot, cs.slug, noWarn); err != nil {
		t.Fatalf("second recovery must be a no-op: %v", err)
	}
}

// Crash AFTER HEAD advanced but before the journal phase was updated:
// recovery must use evidence (parent + binding trailer + tree match),
// not the stale `pre-commit` phase, and publish the committed index.
func TestRecoverLandCrashAfterHeadAdvanceBeforePhaseUpdate(t *testing.T) {
	cs := buildCrashState(t, true)

	// Create the landing commit from the retained index, exactly as
	// land would have, leaving the live index stale.
	env := append(os.Environ(), "GIT_INDEX_FILE="+cs.retainedPath)
	msg := "feat: land\n\nTpatch-Feature: " + cs.slug + "\n"
	commit := exec.Command("git", "-c", "commit.gpgsign=false", "commit", "-m", msg)
	commit.Dir = cs.repoRoot
	commit.Env = env
	if out, cerr := commit.CombinedOutput(); cerr != nil {
		t.Fatalf("construct the landing commit: %s: %v", out, cerr)
	}
	head := gitHead(t, cs.repoRoot)
	if head == cs.preHead {
		t.Fatal("fixture assumption broken: HEAD did not advance")
	}
	// The journal still says pre-commit; recovery must not trust it.
	if j, rerr := readLandJournal(cs.repoRoot, cs.slug); rerr != nil || j.Phase != landPhasePreCommit {
		t.Fatalf("fixture assumption broken: phase=%v err=%v", j, rerr)
	}
	// Deliberately DO NOT refresh the journal: this is exactly the
	// "crashed after HEAD advanced, before the journal was updated"
	// case, which recovery must resolve from the tree evidence.

	if err := recoverLand(cs.repoRoot, cs.slug, noWarn); err != nil {
		t.Fatalf("recovery: %v", err)
	}
	if got := gitHead(t, cs.repoRoot); got != head {
		t.Errorf("recovery moved HEAD: %s -> %s", head, got)
	}
	if staged := strings.TrimSpace(jGit(t, cs.repoRoot, "diff", "--cached", "--name-only")); staged != "" {
		t.Errorf("the published index should agree with the new HEAD, got: %q", staged)
	}
	if _, err := os.Stat(cs.journalPath); !os.IsNotExist(err) {
		t.Errorf("journal residue: %v", err)
	}
	if _, err := os.Stat(cs.lockPath); !os.IsNotExist(err) {
		t.Errorf("our stale lock was not removed: %v", err)
	}
	if err := recoverLand(cs.repoRoot, cs.slug, noWarn); err != nil {
		t.Fatalf("second recovery must be a no-op: %v", err)
	}
}

// HEAD advanced to something that is NOT this feature's landing commit:
// refuse, preserve every artifact, change nothing.
func TestRecoverLandRefusesUnrelatedHeadAdvance(t *testing.T) {
	// No stale lock here: the fixture itself needs to run `git add`,
	// which our own lock would (correctly) block.
	cs := buildCrashState(t, false)
	if err := os.WriteFile(filepath.Join(cs.repoRoot, "unrelated.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jGit(t, cs.repoRoot, "add", "unrelated.txt")
	jGit(t, cs.repoRoot, "-c", "commit.gpgsign=false", "commit", "-qm", "unrelated commit")
	head := gitHead(t, cs.repoRoot)
	liveBefore, err := os.ReadFile(cs.livePath)
	if err != nil {
		t.Fatal(err)
	}

	err = recoverLand(cs.repoRoot, cs.slug, noWarn)
	if err == nil {
		t.Fatal("an unrelated HEAD advance must refuse")
	}
	for _, want := range []string{"cannot be recovered automatically", landJournalDirRel, "git log --grep"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal missing actionable guidance %q: %v", want, err)
		}
	}
	if got := gitHead(t, cs.repoRoot); got != head {
		t.Error("recovery moved HEAD despite refusing")
	}
	if live, _ := os.ReadFile(cs.livePath); string(live) != string(liveBefore) {
		t.Error("recovery modified the live index despite refusing")
	}
	for _, p := range []string{cs.journalPath, cs.retainedPath} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("evidence removed despite refusing: %s: %v", p, err)
		}
	}
}

// Schema, containment and checksum validation all refuse rather than
// operate on untrusted state.
func TestRecoverLandValidatesJournal(t *testing.T) {
	t.Run("unsupported version", func(t *testing.T) {
		cs := buildCrashState(t, false)
		mutateJournal(t, cs, func(j *landJournal) { j.Version = 99 })
		if err := recoverLand(cs.repoRoot, cs.slug, noWarn); err == nil {
			t.Fatal("an unsupported version must refuse")
		} else if !strings.Contains(err.Error(), "not supported") {
			t.Errorf("refusal should name the version: %v", err)
		}
	})

	t.Run("retained index escapes the journal directory", func(t *testing.T) {
		cs := buildCrashState(t, false)
		mutateJournal(t, cs, func(j *landJournal) { j.RetainedIndexRel = "../../etc/passwd" })
		err := recoverLand(cs.repoRoot, cs.slug, noWarn)
		if err == nil {
			t.Fatal("a path escaping the journal directory must refuse")
		}
		if !strings.Contains(err.Error(), "escapes") {
			t.Errorf("refusal should name the containment violation: %v", err)
		}
	})

	t.Run("retained checksum mismatch", func(t *testing.T) {
		cs := buildCrashState(t, false)
		if err := os.WriteFile(cs.retainedPath, []byte("tampered\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		err := recoverLand(cs.repoRoot, cs.slug, noWarn)
		if err == nil {
			t.Fatal("a checksum mismatch must refuse")
		}
		if !strings.Contains(err.Error(), "checksum") {
			t.Errorf("refusal should name the checksum: %v", err)
		}
	})

	t.Run("effective index identity changed", func(t *testing.T) {
		cs := buildCrashState(t, false)
		mutateJournal(t, cs, func(j *landJournal) {
			j.LiveIndexRel = ""
			j.LiveIndexAbs = filepath.Join(t.TempDir(), "someone-elses-index")
		})
		err := recoverLand(cs.repoRoot, cs.slug, noWarn)
		if err == nil {
			t.Fatal("a changed effective index must refuse")
		}
		if !strings.Contains(err.Error(), "effective index") {
			t.Errorf("refusal should name the index mismatch: %v", err)
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		cs := buildCrashState(t, false)
		if err := os.WriteFile(cs.journalPath, []byte("{not json"), 0o600); err != nil {
			t.Fatal(err)
		}
		err := recoverLand(cs.repoRoot, cs.slug, noWarn)
		if err == nil {
			t.Fatal("a malformed journal must refuse")
		}
		if !strings.Contains(err.Error(), "cannot be read") {
			t.Errorf("refusal should say the journal is unreadable: %v", err)
		}
	})
}

// A stale lock carrying OUR nonce is removed; a foreign lock is not,
// and the failure is reported rather than ignored.
func TestRecoverLandLockOwnership(t *testing.T) {
	t.Run("our stale lock is removed", func(t *testing.T) {
		cs := buildCrashState(t, true)
		if err := recoverLand(cs.repoRoot, cs.slug, noWarn); err != nil {
			t.Fatalf("recovery: %v", err)
		}
		if _, err := os.Stat(cs.lockPath); !os.IsNotExist(err) {
			t.Errorf("our stale lock survived: %v", err)
		}
	})

	t.Run("a foreign lock is preserved and reported", func(t *testing.T) {
		cs := buildCrashState(t, false)
		if err := os.WriteFile(cs.lockPath, []byte("someone else\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(cs.lockPath)
		err := recoverLand(cs.repoRoot, cs.slug, noWarn)
		if err == nil {
			t.Fatal("a foreign lock must be reported")
		}
		if !strings.Contains(err.Error(), "belongs to another process") {
			t.Errorf("diagnostic should name the foreign lock: %v", err)
		}
		body, rerr := os.ReadFile(cs.lockPath)
		if rerr != nil {
			t.Fatalf("the foreign lock was removed: %v", rerr)
		}
		if string(body) != "someone else\n" {
			t.Error("the foreign lock was modified")
		}
	})
}

// Recovery runs at land entry, before any record/status/staging
// mutation, and a refusal therefore leaves the feature untouched.
func TestLandRunsRecoveryBeforeAnyMutation(t *testing.T) {
	cs := buildCrashState(t, false)
	// Make recovery refuse by advancing HEAD with an unrelated commit.
	if err := os.WriteFile(filepath.Join(cs.repoRoot, "unrelated.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jGit(t, cs.repoRoot, "add", "unrelated.txt")
	jGit(t, cs.repoRoot, "-c", "commit.gpgsign=false", "commit", "-qm", "unrelated commit")

	featureDir := filepath.Join(cs.repoRoot, ".tpatch", "features", cs.slug)
	before := snapshotDir(t, featureDir)
	head := gitHead(t, cs.repoRoot)

	stdout, stderr, code := runCmdWithError("land", "--path", cs.repoRoot, cs.slug, "--no-record")
	if code == 0 {
		t.Fatalf("land must refuse while an unrecoverable journal exists: stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "cannot be recovered automatically") {
		t.Errorf("land should surface the recovery refusal: %q", stderr)
	}
	assertSnapshotUnchanged(t, "land (recovery refusal)", before, snapshotDir(t, featureDir))
	if got := gitHead(t, cs.repoRoot); got != head {
		t.Error("land advanced HEAD despite the recovery refusal")
	}
}

// A successful land leaves no journal, retained index or lock behind.
func TestLandLeavesNoJournalResidue(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	if stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go"); code != 0 {
		t.Fatalf("land failed: stdout=%q stderr=%q", stdout, stderr)
	}
	if _, err := os.Stat(landJournalPath(tmpDir, slug)); !os.IsNotExist(err) {
		t.Errorf("journal residue after a clean land: %v", err)
	}
	retained := filepath.Join(tmpDir, filepath.FromSlash(retainedIndexRel(slug)))
	if _, err := os.Stat(retained); !os.IsNotExist(err) {
		t.Errorf("retained index residue after a clean land: %v", err)
	}
	if _, err := os.Stat(liveIndexPath(t, tmpDir) + ".lock"); !os.IsNotExist(err) {
		t.Errorf("lock residue after a clean land: %v", err)
	}
	// The journal directory lives under the gitignored local root, so
	// it never shows up as dirty.
	if got := gitPorcelain(t, tmpDir); strings.Contains(got, "land-journal") {
		t.Errorf("journal directory leaked into git status:\n%s", got)
	}
}

// The commit-hook failure path clears the journal and keeps the audited
// staged retry state.
func TestLandCommitFailureClearsJournalAndKeepsStagedRetry(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	hookDir := filepath.Join(tmpDir, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(hookDir, "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, _, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go"); code == 0 {
		t.Fatal("land should fail when the pre-commit hook rejects")
	}
	if _, err := os.Stat(landJournalPath(tmpDir, slug)); !os.IsNotExist(err) {
		t.Errorf("journal residue after an ordinary commit failure: %v", err)
	}
	cached := jGit(t, tmpDir, "diff", "--cached", "--name-only")
	for _, want := range []string{"README.md", "internal/example.go", "status.json"} {
		if !strings.Contains(cached, want) {
			t.Errorf("audited staged retry state missing %q:\n%s", want, cached)
		}
	}
	// Status note stays consistent with the staged retry.
	body, err := os.ReadFile(filepath.Join(tmpDir, ".tpatch", "features", slug, "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"notes": "landed at`) {
		t.Errorf("the staged retry state should keep the landed-at note:\n%s", body)
	}
	if err := os.Remove(hook); err != nil {
		t.Fatal(err)
	}
	if stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--no-record"); code != 0 {
		t.Fatalf("--no-record retry failed: stdout=%q stderr=%q", stdout, stderr)
	}
}

// ─── GH #7 rev-8: locked state comparison ───────────────────────────

// The live index must be compared under the lock, not merely by path.
// An operator `git add` after the crash makes it DIVERGENT, and
// recovery must refuse rather than overwrite their work.
func TestRecoverLandRefusesDivergentLiveIndex(t *testing.T) {
	cs := buildCrashState(t, false)
	// The operator stages something after the crash.
	if err := os.WriteFile(filepath.Join(cs.repoRoot, "operator.txt"), []byte("operator\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jGit(t, cs.repoRoot, "add", "operator.txt")
	liveBefore, err := os.ReadFile(cs.livePath)
	if err != nil {
		t.Fatal(err)
	}
	modeBefore := os.FileMode(0)
	if info, serr := os.Stat(cs.livePath); serr == nil {
		modeBefore = info.Mode().Perm()
	}

	err = recoverLand(cs.repoRoot, cs.slug, noWarn)
	if err == nil {
		t.Fatal("a divergent live index must refuse")
	}
	if !strings.Contains(err.Error(), "no longer matches the state recorded") {
		t.Errorf("refusal should explain the divergence: %v", err)
	}
	live, err := os.ReadFile(cs.livePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(live) != string(liveBefore) {
		t.Error("the operator's index was overwritten despite the refusal")
	}
	if info, serr := os.Stat(cs.livePath); serr != nil || info.Mode().Perm() != modeBefore {
		t.Errorf("index mode changed despite the refusal")
	}
	if !strings.Contains(jGit(t, cs.repoRoot, "diff", "--cached", "--name-only"), "operator.txt") {
		t.Error("the operator's staged file was lost")
	}
	for _, p := range []string{cs.journalPath, cs.retainedPath} {
		if _, serr := os.Stat(p); serr != nil {
			t.Errorf("evidence removed despite refusing: %s: %v", p, serr)
		}
	}
	if _, serr := os.Stat(cs.livePath + ".lock"); !os.IsNotExist(serr) {
		t.Errorf("recovery left a lock behind: %v", serr)
	}

	// Restore the exact preimage bytes and mode; recovery then
	// succeeds. (`git reset` would produce a semantically equivalent
	// but not byte-identical index, which recovery correctly still
	// treats as divergent.)
	if err := os.WriteFile(cs.livePath, cs.livePre, cs.liveMode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cs.livePath, cs.liveMode); err != nil {
		t.Fatal(err)
	}
	if err := recoverLand(cs.repoRoot, cs.slug, noWarn); err != nil {
		t.Fatalf("recovery after removing the divergence: %v", err)
	}
	if _, serr := os.Stat(cs.journalPath); !os.IsNotExist(serr) {
		t.Errorf("journal residue: %v", serr)
	}
}

// If the retained index was already published, recovery must recognise
// the postimage and clean up idempotently WITHOUT rewriting the index.
func TestRecoverLandPostimageCleansWithoutRewrite(t *testing.T) {
	cs := buildCrashState(t, false)
	retained, err := os.ReadFile(cs.retainedPath)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate "a previous recovery already published it": the live
	// index is byte-identical to the retained one, with its mode.
	if err := os.WriteFile(cs.livePath, retained, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cs.livePath, 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(cs.livePath)
	if err != nil {
		t.Fatal(err)
	}
	mtimeBefore := info.ModTime()

	if err := recoverLand(cs.repoRoot, cs.slug, noWarn); err != nil {
		t.Fatalf("postimage recovery must succeed: %v", err)
	}
	after, err := os.Stat(cs.livePath)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(mtimeBefore) {
		t.Error("an already-published index was rewritten instead of left alone")
	}
	body, err := os.ReadFile(cs.livePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != string(retained) {
		t.Error("the published index changed during cleanup")
	}
	if _, err := os.Stat(cs.journalPath); !os.IsNotExist(err) {
		t.Errorf("journal residue: %v", err)
	}
	// Idempotent.
	if err := recoverLand(cs.repoRoot, cs.slug, noWarn); err != nil {
		t.Fatalf("second recovery must be a no-op: %v", err)
	}
}

// A foreign lock blocks recovery entirely and is preserved; an owned
// stale lock is validated by nonce AND inode before removal.
func TestRecoverLandLockValidation(t *testing.T) {
	t.Run("owned stale lock with a wrong inode is preserved", func(t *testing.T) {
		cs := buildCrashState(t, true)
		mutateJournal(t, cs, func(j *landJournal) { j.LockIno = 999999999 })
		err := recoverLand(cs.repoRoot, cs.slug, noWarn)
		if err == nil {
			t.Fatal("an inode mismatch must refuse")
		}
		if !strings.Contains(err.Error(), "recreated by another process") {
			t.Errorf("refusal should name the inode mismatch: %v", err)
		}
		if _, serr := os.Stat(cs.lockPath); serr != nil {
			t.Errorf("the lock was removed despite the mismatch: %v", serr)
		}
	})

	t.Run("a lock held by another process blocks recovery", func(t *testing.T) {
		cs := buildCrashState(t, false)
		if err := os.WriteFile(cs.lockPath, []byte("someone else\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(cs.lockPath)
		err := recoverLand(cs.repoRoot, cs.slug, noWarn)
		if err == nil {
			t.Fatal("a foreign lock must refuse")
		}
		if !strings.Contains(err.Error(), "belongs to another process") {
			t.Errorf("refusal should name the foreign lock: %v", err)
		}
		body, rerr := os.ReadFile(cs.lockPath)
		if rerr != nil || string(body) != "someone else\n" {
			t.Error("the foreign lock was removed or modified")
		}
	})
}

// A pre-commit hook that stages an ALLOWED extra file mutates the
// retained index. The commit must include it, and the journal's
// post-commit evidence must describe the mutated index.
func TestLandHookStagedFileIsCommittedAndJournalled(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "hook-added.txt"), []byte("hook\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hookDir := filepath.Join(tmpDir, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The hook stages an extra allowed file into whatever index Git is
	// using — which is land's retained alternate index.
	if err := os.WriteFile(filepath.Join(hookDir, "pre-commit"),
		[]byte("#!/bin/sh\ngit add hook-added.txt\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go", "--allow-extra-paths")
	if code != 0 {
		t.Fatalf("land failed: stdout=%q stderr=%q", stdout, stderr)
	}
	committed := jGit(t, tmpDir, "show", "--pretty=format:", "--name-only", "HEAD")
	if !strings.Contains(committed, "hook-added.txt") {
		t.Errorf("the hook's staged file is missing from the commit:\n%s", committed)
	}
	if staged := strings.TrimSpace(jGit(t, tmpDir, "diff", "--cached", "--name-only")); staged != "" {
		t.Errorf("the published index disagrees with HEAD: %q", staged)
	}
	if _, err := os.Stat(landJournalPath(tmpDir, slug)); !os.IsNotExist(err) {
		t.Errorf("journal residue after a successful land: %v", err)
	}
}

// A hook that stages a nested worktree must be caught by the
// post-commit re-audit, and the evidence preserved.
func TestLandHookNestedWorktreeContaminationRefuses(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	hookDir := filepath.Join(tmpDir, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The hook stages the nested worktree, then rejects the commit.
	script := "#!/bin/sh\ngit -c advice.addEmbeddedRepo=false --literal-pathspecs add -- '" +
		nestedWorktreeRel + "' >/dev/null 2>&1\nexit 1\n"
	if err := os.WriteFile(filepath.Join(hookDir, "pre-commit"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	headBefore := gitHead(t, tmpDir)

	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go")
	if code == 0 {
		t.Fatalf("land must refuse when a hook stages a nested worktree: stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "nested Git worktree") {
		t.Errorf("refusal should name the contamination: %q", stderr)
	}
	if got := gitHead(t, tmpDir); got != headBefore {
		t.Errorf("HEAD advanced despite refusing")
	}
	cached := jGit(t, tmpDir, "diff", "--cached", "--name-only")
	if strings.Contains(cached, nestedWorktreeDirName) {
		t.Errorf("the contaminated index reached the live index:\n%s", cached)
	}
}

// When the durable publish of the staged-retry index fails after a
// commit failure, the journal and retained index MUST be kept — they
// are the only copy of the retry evidence — and a later recovery must
// publish them and then clear.
func TestLandCommitFailurePublishFailureRetainsEvidenceThenRecovers(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	hookDir := filepath.Join(tmpDir, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(hookDir, "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	live := liveIndexPath(t, tmpDir)
	gitutil.SetPublishRenameHookForTest(func(target string) error {
		if target != live {
			return nil
		}
		return fmt.Errorf("injected publish failure")
	})
	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go")
	gitutil.SetPublishRenameHookForTest(nil)
	if code == 0 {
		t.Fatalf("land should fail: stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "commit failed, staged retry recovery pending") {
		t.Errorf("diagnostic should distinguish the staged-retry pending case: %q", stderr)
	}
	// Evidence retained.
	for _, p := range []string{landJournalPath(tmpDir, slug), retainedIndexAbs(tmpDir, slug)} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("evidence deleted despite the publish failure: %s: %v", p, err)
		}
	}
	if _, err := os.Stat(liveIndexPath(t, tmpDir) + ".lock"); !os.IsNotExist(err) {
		t.Errorf("lock residue: %v", err)
	}

	// A later land recovers: publishes the staged retry, then clears.
	if err := os.Remove(hook); err != nil {
		t.Fatal(err)
	}
	if err := recoverLand(tmpDir, slug, noWarn); err != nil {
		t.Fatalf("recovery after the retained publish failure: %v", err)
	}
	cached := jGit(t, tmpDir, "diff", "--cached", "--name-only")
	for _, want := range []string{"README.md", "internal/example.go"} {
		if !strings.Contains(cached, want) {
			t.Errorf("staged retry state missing %q:\n%s", want, cached)
		}
	}
	if _, err := os.Stat(landJournalPath(tmpDir, slug)); !os.IsNotExist(err) {
		t.Errorf("journal residue after a successful recovery: %v", err)
	}
}

// When the post-commit publish fails, HEAD has advanced: the diagnostic
// must say so precisely, the journal must be kept, and a later recovery
// must finish the publication.
func TestLandPostCommitPublishFailureIsRecoveryPending(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	headBefore := gitHead(t, tmpDir)

	live := liveIndexPath(t, tmpDir)
	gitutil.SetPublishRenameHookForTest(func(target string) error {
		if target != live {
			return nil
		}
		return fmt.Errorf("injected publish failure")
	})
	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go")
	gitutil.SetPublishRenameHookForTest(nil)
	if code == 0 {
		t.Fatalf("land should report the publish failure: stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "commit succeeded, recovery pending") {
		t.Errorf("diagnostic should say the commit succeeded and recovery is pending: %q", stderr)
	}
	if strings.Contains(stderr, "rolled back") {
		t.Errorf("the diagnostic must never claim a rollback of HEAD: %q", stderr)
	}
	if got := gitHead(t, tmpDir); got == headBefore {
		t.Fatal("HEAD should have advanced")
	}
	for _, p := range []string{landJournalPath(tmpDir, slug), retainedIndexAbs(tmpDir, slug)} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("evidence deleted despite the publish failure: %s: %v", p, err)
		}
	}

	if err := recoverLand(tmpDir, slug, noWarn); err != nil {
		t.Fatalf("recovery after the post-commit publish failure: %v", err)
	}
	if staged := strings.TrimSpace(jGit(t, tmpDir, "diff", "--cached", "--name-only")); staged != "" {
		t.Errorf("the recovered index disagrees with HEAD: %q", staged)
	}
	if _, err := os.Stat(landJournalPath(tmpDir, slug)); !os.IsNotExist(err) {
		t.Errorf("journal residue after a successful recovery: %v", err)
	}
	if err := recoverLand(tmpDir, slug, noWarn); err != nil {
		t.Fatalf("second recovery must be a no-op: %v", err)
	}
}

// ─── GH #7 rev-9 F1: contaminated successful commit ─────────────────

// writeHook installs an executable hook.
func writeHook(t *testing.T, repoRoot, name, body string) string {
	t.Helper()
	dir := filepath.Join(repoRoot, ".git", "hooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// A pre-commit hook that stages a registered nested worktree and then
// lets the commit SUCCEED already put the gitlink in a commit. Refusing
// to publish is not enough: the commit must be rolled back.
func TestLandSuccessfulHookContaminationRollsBackHead(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	writeHook(t, tmpDir, "pre-commit",
		"#!/bin/sh\ngit -c advice.addEmbeddedRepo=false --literal-pathspecs add -- '"+
			nestedWorktreeRel+"' >/dev/null 2>&1\nexit 0\n")

	headBefore := gitHead(t, tmpDir)
	livePath := liveIndexPath(t, tmpDir)
	liveBefore, err := os.ReadFile(livePath)
	if err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(tmpDir, ".tpatch", "features", slug, "status.json")

	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go")
	if code == 0 {
		t.Fatalf("land must refuse a contaminated commit: stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "nested Git worktree") {
		t.Errorf("refusal should name the contamination: %q", stderr)
	}
	if !strings.Contains(stderr, "rolled back") {
		t.Errorf("refusal should say the commit was rolled back: %q", stderr)
	}
	if strings.Contains(stdout, "Landed ") {
		t.Errorf("a contaminated commit must never be presented as landed:\n%s", stdout)
	}

	// HEAD is exactly the pre-land commit.
	if got := gitHead(t, tmpDir); got != headBefore {
		t.Fatalf("HEAD was not rolled back: %s (want %s)", got, headBefore)
	}
	// No binding commit reachable from the branch.
	bound := jGit(t, tmpDir, "log", "--grep", "^Tpatch-Feature: "+slug+"$", "--pretty=%H")
	if strings.TrimSpace(bound) != "" {
		t.Errorf("a binding landing commit is still reachable:\n%s", bound)
	}
	// The live index and status preimage are untouched.
	liveAfter, err := os.ReadFile(livePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(liveAfter) != string(liveBefore) {
		t.Error("the live index was modified despite the refusal")
	}
	// The embedded record legitimately rewrote status.json (its own
	// completed transaction, as on every other refusal path). What must
	// be gone is land's own landed-at note.
	statusAfter, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(statusAfter), `"notes": "landed at`) {
		t.Errorf("the landed-at note survived the rollback:\n%s", statusAfter)
	}
	// A completed rollback cleans the evidence.
	for _, p := range []string{landJournalPath(tmpDir, slug), retainedIndexAbs(tmpDir, slug)} {
		if _, serr := os.Stat(p); !os.IsNotExist(serr) {
			t.Errorf("evidence residue after a completed rollback: %s: %v", p, serr)
		}
	}
	if _, serr := os.Stat(livePath + ".lock"); !os.IsNotExist(serr) {
		t.Errorf("lock residue: %v", serr)
	}
	if err := recoverLand(tmpDir, slug, noWarn); err != nil {
		t.Fatalf("recovery after a completed rollback must be a no-op: %v", err)
	}

	// Removing the hook lets the feature land normally.
	if err := os.Remove(filepath.Join(tmpDir, ".git", "hooks", "pre-commit")); err != nil {
		t.Fatal(err)
	}
	if stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go"); code != 0 {
		t.Fatalf("land failed after removing the hook: stdout=%q stderr=%q", stdout, stderr)
	}
	assertNoNestedWorktree(t, "landing commit after recovery",
		jGit(t, tmpDir, "show", "--pretty=format:", "--name-only", "HEAD"))
}

// If the compare-and-swap rollback cannot complete — here because a
// concurrent process advanced the branch — the evidence is preserved
// and the diagnostic is a manual-recovery one that never claims a
// landing.
func TestLandContaminatedRollbackFailureRetainsEvidence(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	// The hook stages the nested worktree AND, via post-commit, moves
	// the branch on so the CAS rollback sees an unexpected old value.
	writeHook(t, tmpDir, "pre-commit",
		"#!/bin/sh\ngit -c advice.addEmbeddedRepo=false --literal-pathspecs add -- '"+
			nestedWorktreeRel+"' >/dev/null 2>&1\nexit 0\n")
	// `commit-tree` needs no index, so this deterministically advances
	// the branch past the landing commit while land is still running.
	writeHook(t, tmpDir, "post-commit",
		"#!/bin/sh\n"+
			"n=$(git commit-tree 'HEAD^{tree}' -p HEAD -m 'concurrent advance') || exit 0\n"+
			"git update-ref HEAD \"$n\" || true\n")

	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go")
	if code == 0 {
		t.Fatalf("land must refuse: stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "manual recovery pending") {
		t.Errorf("diagnostic should be the manual-recovery one: %q", stderr)
	}
	if strings.Contains(stdout, "Landed ") {
		t.Errorf("a contaminated commit must never be presented as landed:\n%s", stdout)
	}
	for _, p := range []string{landJournalPath(tmpDir, slug), retainedIndexAbs(tmpDir, slug)} {
		if _, serr := os.Stat(p); serr != nil {
			t.Errorf("evidence deleted despite the rollback failure: %s: %v", p, serr)
		}
	}
	if _, serr := os.Stat(liveIndexPath(t, tmpDir) + ".lock"); !os.IsNotExist(serr) {
		t.Errorf("lock residue: %v", serr)
	}
	// Recovery must refuse rather than publish anything: either the
	// unexpected HEAD or the contaminated retained index is enough.
	livePre, err := os.ReadFile(liveIndexPath(t, tmpDir))
	if err != nil {
		t.Fatal(err)
	}
	rerr := recoverLand(tmpDir, slug, noWarn)
	if rerr == nil {
		t.Fatal("recovery must refuse to publish after a failed rollback")
	}
	if !strings.Contains(rerr.Error(), "cannot be recovered automatically") {
		t.Errorf("recovery should refuse with manual guidance: %v", rerr)
	}
	liveAfter, err := os.ReadFile(liveIndexPath(t, tmpDir))
	if err != nil {
		t.Fatal(err)
	}
	if string(liveAfter) != string(livePre) {
		t.Error("recovery modified the live index despite refusing")
	}
	for _, p := range []string{landJournalPath(tmpDir, slug), retainedIndexAbs(tmpDir, slug)} {
		if _, serr := os.Stat(p); serr != nil {
			t.Errorf("evidence removed by the refusing recovery: %s: %v", p, serr)
		}
	}
}

// Recovery must never publish a retained index containing a nested
// worktree, even when HEAD is exactly the pre-land commit and every
// other check passes.
func TestRecoverLandRefusesContaminatedRetainedIndex(t *testing.T) {
	cs := buildCrashState(t, false)
	// Contaminate the retained index in place, then re-journal its
	// identity so only the contamination guard can refuse.
	env := append(os.Environ(), "GIT_INDEX_FILE="+cs.retainedPath)
	add := exec.Command("git", "-c", "advice.addEmbeddedRepo=false",
		"--literal-pathspecs", "add", "--", nestedWorktreeRel)
	add.Dir = cs.repoRoot
	add.Env = env
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("contaminate the retained index: %s: %v", out, err)
	}
	body, err := os.ReadFile(cs.retainedPath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	info, err := os.Stat(cs.retainedPath)
	if err != nil {
		t.Fatal(err)
	}
	mutateJournal(t, cs, func(j *landJournal) {
		j.RetainedPre.SHA256 = hex.EncodeToString(sum[:])
		j.RetainedPre.Mode = uint32(info.Mode().Perm())
	})
	livePre, err := os.ReadFile(cs.livePath)
	if err != nil {
		t.Fatal(err)
	}

	err = recoverLand(cs.repoRoot, cs.slug, noWarn)
	if err == nil {
		t.Fatal("recovery must refuse to publish a contaminated retained index")
	}
	if !strings.Contains(err.Error(), "nested Git worktree") {
		t.Errorf("refusal should name the contamination: %v", err)
	}
	liveAfter, err := os.ReadFile(cs.livePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(liveAfter) != string(livePre) {
		t.Error("recovery published the contaminated index")
	}
	for _, p := range []string{cs.journalPath, cs.retainedPath} {
		if _, serr := os.Stat(p); serr != nil {
			t.Errorf("evidence removed despite refusing: %s: %v", p, serr)
		}
	}
}

// commit-msg and prepare-commit-msg hooks are controls: they run in the
// same environment and must not be treated as contamination when they
// touch nothing.
func TestLandBenignMessageHooksStillLand(t *testing.T) {
	tmpDir, slug := setupNestedWorktreeFixture(t)
	if _, stderr, code := runCmdWithError("apply", "--path", tmpDir, slug, "--mode", "done"); code != 0 {
		t.Fatalf("apply --mode done failed: %s", stderr)
	}
	writeHook(t, tmpDir, "prepare-commit-msg", "#!/bin/sh\nexit 0\n")
	writeHook(t, tmpDir, "commit-msg", "#!/bin/sh\nexit 0\n")

	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug,
		"--files", "README.md,internal/example.go")
	if code != 0 {
		t.Fatalf("benign message hooks must not block landing: stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "Landed ") {
		t.Errorf("expected a normal landing:\n%s", stdout)
	}
	if _, err := os.Stat(landJournalPath(tmpDir, slug)); !os.IsNotExist(err) {
		t.Errorf("journal residue: %v", err)
	}
}

// ─── GH #7 rev-9 F2: journal cannot direct filesystem operations ────

// A journal carrying a lock path field is rejected outright by the
// strict decoder, and the victim file it names is never touched.
func TestRecoverLandRejectsJournalSuppliedLockPaths(t *testing.T) {
	for _, field := range []string{"lock_abs", "lock_rel", "some_other_path"} {
		t.Run(field, func(t *testing.T) {
			cs := buildCrashState(t, false)
			victimDir := t.TempDir()
			victim := filepath.Join(victimDir, "victim.lock")
			// A victim that even carries a matching nonce sentinel.
			if err := os.WriteFile(victim, []byte("tpatch-land-lock "+cs.nonce+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(cs.journalPath)
			if err != nil {
				t.Fatal(err)
			}
			var raw map[string]any
			if err := json.Unmarshal(body, &raw); err != nil {
				t.Fatal(err)
			}
			raw[field] = victim
			tampered, err := json.MarshalIndent(raw, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(cs.journalPath, append(tampered, '\n'), 0o600); err != nil {
				t.Fatal(err)
			}

			rerr := recoverLand(cs.repoRoot, cs.slug, noWarn)
			if rerr == nil {
				t.Fatal("a journal with an unknown field must be refused")
			}
			if !strings.Contains(rerr.Error(), "not a valid journal document") {
				t.Errorf("refusal should come from the strict decoder: %v", rerr)
			}
			if _, serr := os.Stat(victim); serr != nil {
				t.Fatalf("the victim file was removed: %v", serr)
			}
			body2, err := os.ReadFile(victim)
			if err != nil || !strings.Contains(string(body2), cs.nonce) {
				t.Error("the victim file was modified")
			}
		})
	}
}

// Schema v2 journals are refused: the version gate is the contract.
func TestRecoverLandRefusesPreviousSchemaVersion(t *testing.T) {
	cs := buildCrashState(t, false)
	mutateJournal(t, cs, func(j *landJournal) { j.Version = 2 })
	err := recoverLand(cs.repoRoot, cs.slug, noWarn)
	if err == nil {
		t.Fatal("a v2 journal must be refused")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("refusal should name the version: %v", err)
	}
}

// A retained-index reference reached through a symlinked component is
// refused before it is read.
func TestRecoverLandRefusesSymlinkedRetainedPath(t *testing.T) {
	cs := buildCrashState(t, false)
	dir := landJournalDir(cs.repoRoot)
	realDir := filepath.Join(cs.repoRoot, ".tpatch", "local", "elsewhere")
	if err := os.MkdirAll(realDir, 0o700); err != nil {
		t.Fatal(err)
	}
	linkDir := filepath.Join(dir, "sub")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Skipf("platform refuses symlinks: %v", err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "x.index"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	mutateJournal(t, cs, func(j *landJournal) {
		j.RetainedIndexRel = landJournalDirRel + "/sub/x.index"
	})
	err := recoverLand(cs.repoRoot, cs.slug, noWarn)
	if err == nil {
		t.Fatal("a symlinked retained-index component must be refused")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("refusal should name the symlink: %v", err)
	}
}
