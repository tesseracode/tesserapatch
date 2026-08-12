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
	retSum := sha256.Sum256(retainedBytes)

	nonce := "deadbeefdeadbeefdeadbeefdeadbeef"
	lockPath := livePath + ".lock"
	if withLock {
		if err := os.WriteFile(lockPath, []byte("tpatch-land-lock "+nonce+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	liveRel, liveAbs := relOrAbs(tmpDir, livePath)
	lockRel, lockAbs := relOrAbs(tmpDir, lockPath)
	j := landJournal{
		Version:          landJournalVersion,
		Slug:             slug,
		CreatedAt:        "2026-08-12T00:00:00Z",
		Phase:            landPhasePreCommit,
		PreHead:          preHead,
		LiveIndexRel:     liveRel,
		LiveIndexAbs:     liveAbs,
		LiveIndex:        landJournalFileState{Exists: len(liveBytes) > 0, SHA256: hex.EncodeToString(liveSum[:]), Mode: 0o644},
		RetainedIndexRel: retainedIndexRel(slug),
		RetainedIndex:    landJournalFileState{Exists: true, SHA256: hex.EncodeToString(retSum[:]), Mode: 0o600},
		LockRel:          lockRel,
		LockAbs:          lockAbs,
		LockNonce:        nonce,
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
	retained, err := os.ReadFile(cs.retainedPath)
	if err != nil {
		t.Fatal(err)
	}

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
	// Refresh the retained checksum, since committing rewrote it.
	newRetained, err := os.ReadFile(cs.retainedPath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(newRetained)
	mutateJournal(t, cs, func(j *landJournal) {
		j.RetainedIndex.SHA256 = hex.EncodeToString(sum[:])
	})

	if err := recoverLand(cs.repoRoot, cs.slug, noWarn); err != nil {
		t.Fatalf("recovery: %v", err)
	}
	if got := gitHead(t, cs.repoRoot); got != head {
		t.Errorf("recovery moved HEAD: %s -> %s", head, got)
	}
	live, err := os.ReadFile(cs.livePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(live) == string(retained) {
		t.Error("recovery published the pre-commit index instead of the committed one")
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
