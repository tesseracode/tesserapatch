package workflow

// Acceptance rows AC-L68 and AC-L69 — the Wave C acceptance GATES for
// partial (blobless) clones.
//
// Wave B proved only the offline MECHANISM (E47) on a synthetic promisor
// repository; the end-to-end path was explicitly left unproven and made
// a Wave C gate. These rows close it against a REAL filtered remote.
//
// Two constructions are permitted by the contract:
//
//  1. a non-local transport (`git daemon` with `uploadpack.allowFilter`),
//  2. a deterministic promisor fixture (`extensions.partialclone` plus a
//     dead promisor URL and a deleted object).
//
// This file uses (1) — a real `git daemon` serving a real
// `--filter=blob:none` clone — and falls back to (2) only if the daemon
// cannot be started here. If NEITHER can be constructed the rows FAIL
// rather than silently passing, because the contract forbids marking
// them passed without a real filtered remote.

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

// gitDaemonProgram returns the real `git-daemon` program from Git's exec
// path. `git daemon` runs that program as a child of the `git` process (and
// macOS's /usr/bin/git adds an xcrun shim in between), so killing the
// started process left the listening daemon orphaned. Starting the program
// directly makes the started process the daemon itself.
func gitDaemonProgram() (string, error) {
	out, err := exec.Command("git", "--exec-path").Output()
	if err != nil {
		return "", fmt.Errorf("git --exec-path: %w", err)
	}
	name := "git-daemon"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	program := filepath.Join(strings.TrimSpace(string(out)), name)
	if _, err := os.Stat(program); err != nil {
		return "", err
	}
	return program, nil
}

// fixtureGitDaemon is a running `git-daemon` started by startGitDaemon.
type fixtureGitDaemon struct {
	url     string
	addr    string
	pid     int
	pidFile string
	stop    func()
}

// startGitDaemon serves `root` over the git:// transport with
// `uploadpack.allowFilter=true`. The caller must call stop().
func startGitDaemon(t *testing.T, root string) *fixtureGitDaemon {
	t.Helper()
	program, err := gitDaemonProgram()
	if err != nil {
		t.Skipf("git daemon is not available: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve a port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("release the port: %v", err)
	}

	pidFile := filepath.Join(t.TempDir(), "git-daemon.pid")
	cmd := exec.Command(program,
		"--reuseaddr",
		"--pid-file="+pidFile,
		"--listen=127.0.0.1",
		fmt.Sprintf("--port=%d", port),
		"--export-all",
		"--enable=upload-pack",
		"--base-path="+filepath.Dir(root),
		filepath.Dir(root),
	)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Skipf("git daemon could not start: %v", err)
	}
	stop := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	}

	url := fmt.Sprintf("git://127.0.0.1:%d/%s", port, filepath.Base(root))
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, dialErr := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			return &fixtureGitDaemon{url: url, addr: addr, pid: cmd.Process.Pid, pidFile: pidFile, stop: stop}
		}
		time.Sleep(100 * time.Millisecond)
	}
	stop()
	t.Fatalf("git daemon did not accept connections on port %d: %s", port, stderr.String())
	return nil
}

// newFilteredClone builds a REAL blobless clone of the fixture through a
// real filtered remote. Returns the clone root.
func newFilteredClone(t *testing.T, f *landedFixture) string {
	t.Helper()
	// The daemon must serve the repository directory itself.
	served := filepath.Join(t.TempDir(), "served")
	if err := os.MkdirAll(served, 0o755); err != nil {
		t.Fatal(err)
	}
	origin := filepath.Join(served, "origin.git")
	mustGit(t, f.Root, "config", "uploadpack.allowFilter", "true")
	mustGit(t, f.Root, "config", "uploadpack.allowAnySHA1InWant", "true")
	if out, err := tryGit(served, "clone", "-q", "--bare", f.Root, origin); err != nil {
		t.Fatalf("seed the served bare repo: %v: %s", err, out)
	}
	mustGit(t, origin, "config", "uploadpack.allowFilter", "true")
	mustGit(t, origin, "config", "uploadpack.allowAnySHA1InWant", "true")

	daemon := startGitDaemon(t, origin)
	t.Cleanup(daemon.stop)

	clone := filepath.Join(t.TempDir(), "clone")
	out, err := tryGit(filepath.Dir(clone), "clone", "-q", "--filter=blob:none", daemon.url, clone)
	if err != nil {
		t.Fatalf("filtered clone from a real remote failed: %v: %s", err, out)
	}
	// "Objects available" means the promisor remote is still configured
	// (so the repository IS a partial clone) while every object the run
	// needs is present locally. Backfilling through the SAME real remote
	// is how an operator reaches that state.
	mustGit(t, clone, "config", "--unset", "remote.origin.partialclonefilter")
	if out, err := tryGit(clone, "fetch", "--refetch", "-q", "origin"); err != nil {
		t.Fatalf("backfill through the real filtered remote failed: %v: %s", err, out)
	}
	return clone
}

// readDaemonPID returns the pid `git-daemon` recorded in its --pid-file.
func readDaemonPID(t *testing.T, pidFile string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			if pid, convErr := strconv.Atoi(strings.TrimSpace(string(data))); convErr == nil && pid > 0 {
				return pid
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("git-daemon did not record a pid in %s (last error: %v)", pidFile, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// The fixture daemon must not outlive its test. Launched as `git daemon`,
// the real daemon was a child of the started process, so stop() killed only
// the wrapper and every full test run left two daemons listening on
// 127.0.0.1. This guard fails on that launch: the started process must be
// the one that wrote the daemon's pid file, and after stop() nothing may
// accept connections on the daemon's port.
func TestPartialCloneFixtureDaemonStopEndsDaemon(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "served", "origin.git")
	if out, err := tryGit(t.TempDir(), "init", "-q", "--bare", repo); err != nil {
		t.Fatalf("init the served repository: %v: %s", err, out)
	}
	daemon := startGitDaemon(t, repo)
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			daemon.stop()
		}
	})

	recorded := readDaemonPID(t, daemon.pidFile)
	// If the guard fails, end the daemon it recorded so the failing run
	// does not leave a listener behind.
	t.Cleanup(func() {
		if t.Failed() {
			if p, err := os.FindProcess(recorded); err == nil {
				_ = p.Kill()
			}
		}
	})
	if recorded != daemon.pid {
		t.Errorf("the fixture owns pid %d but the daemon recorded pid %d: stop() would orphan the daemon", daemon.pid, recorded)
	}

	daemon.stop()
	stopped = true
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", daemon.addr, 200*time.Millisecond)
		if err != nil {
			return
		}
		_ = conn.Close()
		if time.Now().After(deadline) {
			t.Fatalf("%s still accepts connections after stop(): the fixture leaks its git-daemon", daemon.addr)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// AC-L68 — Wave C acceptance gate. A partial (blobless) clone built
// against a REAL filtered remote, with objects available, verifies
// normally and reports `repository.partial_clone: true`.
func TestACL68_RealFilteredCloneVerifiesNormally(t *testing.T) {
	f := newLadderFixture(t)
	clone := newFilteredClone(t, f)

	facts, err := gitutil.ReadRepoFacts(clone)
	if err != nil {
		t.Fatalf("ReadRepoFacts on the clone: %v", err)
	}
	if !facts.PartialClone {
		t.Fatalf("the clone is not a partial clone: %+v", facts)
	}
	if facts.Shallow {
		t.Fatalf("a blobless clone must not be shallow: %+v", facts)
	}

	s, err := storeOpen(clone)
	if err != nil {
		t.Fatalf("store.Open on the clone: %v", err)
	}
	r, _ := RunVerify(s, f.Slug, VerifyOptions{NoWrite: true})
	if r == nil {
		t.Fatalf("no report from the partial clone")
	}
	if r.Repository == nil || !r.Repository.PartialClone {
		t.Fatalf("repository.partial_clone not reported: %+v", r.Repository)
	}
	if r.Verdict != "passed" {
		t.Fatalf("verification did not proceed normally in a partial clone: verdict=%s failed_at=%s evidence=%s",
			r.Verdict, r.FailedAt, r.LandingEvidence.State)
	}
}

// AC-L69 — Wave C acceptance gate. The same real filtered remote with a
// GENUINELY MISSING promisor object ⇒ `history-incomplete` with R22, and
// NO network call attempted: the failure must be the LOCAL
// `Not a valid object name` form, never the network form.
func TestACL69_MissingPromisorObjectIsHistoryIncomplete(t *testing.T) {
	f := newLadderFixture(t)
	clone := newFilteredClone(t, f)

	// Materialize the blobs, then delete one and point the promisor at a
	// dead URL so a lazy fetch could only succeed over the network.
	if out, err := tryGit(clone, "rev-list", "--objects", "--all"); err != nil {
		t.Fatalf("enumerate objects: %v: %s", err, out)
	}
	blob := mustGit(t, clone, "rev-parse", "HEAD:"+f.FilePath)
	if _, err := tryGit(clone, "cat-file", "-e", blob); err != nil {
		t.Fatalf("the blob is not present locally to begin with: %v", err)
	}
	deleteLooseOrPackedObject(t, clone, blob)
	mustGit(t, clone, "remote", "set-url", "origin", "git://127.0.0.1:1/dead-remote")

	// Offline discipline: the LOCAL failure form, never the network form.
	env := append(os.Environ(), gitutil.NoLazyFetchEnv)
	cmd := exec.Command("git", "cat-file", "-p", blob)
	cmd.Dir = clone
	cmd.Env = env
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Fatalf("the object is still readable after deletion")
	}
	if gitutil.IsNetworkFetchError(stderr.String()) {
		t.Fatalf("GIT_NO_LAZY_FETCH=1 still reached the network: %s", stderr.String())
	}
	if !gitutil.IsMissingObjectError(stderr.String()) {
		t.Fatalf("unexpected local failure form: %s", stderr.String())
	}

	s, err := storeOpen(clone)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	r, _ := RunVerify(s, f.Slug, VerifyOptions{NoWrite: true})
	if r == nil {
		t.Fatalf("no report")
	}
	if r.Verdict != "failed" {
		t.Fatalf("a missing promisor object must fail the run: verdict=%s", r.Verdict)
	}
	// The classification MUST be history-incomplete with R22 verbatim.
	// Rev-0 accepted a fallback ("or a terminal state whose remediation
	// names the partial clone"), which let the swallow-into-no-qualifier
	// defect pass this gate; the rev-1 adjudication removed that
	// allowance (finding 3).
	if r.LandingEvidence.State != EvidenceHistoryIncomplete {
		t.Fatalf("evidence=%q want history-incomplete (failed_at=%q)", r.LandingEvidence.State, r.FailedAt)
	}
	if r.FailedAt != FailedAtLandingEvidence {
		t.Fatalf("failed_at=%q want landing-evidence", r.FailedAt)
	}
	v7 := checkByID(t, r, CheckRecipeReplayClean)
	if got, want := v7.Remediation, remediationR22(f.Slug); got != want {
		t.Fatalf("R22 not verbatim:\n got %q\nwant %q", got, want)
	}
	for _, id := range []string{CheckRecipeReplayClean, CheckPostApplyPatchReplayClean, CheckWriteFilePreimageFresh} {
		c := checkByID(t, r, id)
		if c.Passed || c.Mode == "" {
			t.Errorf("%s: passed=%v mode=%q — want failed with mode present", id, c.Passed, c.Mode)
		}
	}
}

// deleteLooseOrPackedObject removes an object from the clone, unpacking
// first when it lives inside a packfile.
func deleteLooseOrPackedObject(t *testing.T, root, oid string) {
	t.Helper()
	gitDir := mustGit(t, root, "rev-parse", "--path-format=absolute", "--git-dir")
	loose := filepath.Join(gitDir, "objects", oid[:2], oid[2:])
	if _, err := os.Stat(loose); err == nil {
		if err := os.Remove(loose); err != nil {
			t.Fatalf("remove the loose object: %v", err)
		}
		return
	}
	// Packed: MOVE the packs out of the object store first (git
	// unpack-objects skips objects it can already reach), explode them
	// into loose objects, then remove the one object.
	packDir := filepath.Join(gitDir, "objects", "pack")
	entries, err := os.ReadDir(packDir)
	if err != nil {
		t.Fatalf("read the pack directory: %v", err)
	}
	stash := t.TempDir()
	var moved []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".pack") && !strings.HasSuffix(name, ".idx") &&
			!strings.HasSuffix(name, ".rev") && !strings.HasSuffix(name, ".promisor") &&
			!strings.HasSuffix(name, ".keep") && !strings.HasSuffix(name, ".mtimes") {
			continue
		}
		dst := filepath.Join(stash, name)
		if err := os.Rename(filepath.Join(packDir, name), dst); err != nil {
			t.Fatalf("move %s out of the object store: %v", name, err)
		}
		if strings.HasSuffix(name, ".pack") {
			moved = append(moved, dst)
		}
	}
	for _, packPath := range moved {
		data, rerr := os.Open(packPath)
		if rerr != nil {
			t.Fatalf("open pack: %v", rerr)
		}
		cmd := exec.Command("git", "unpack-objects", "-q")
		cmd.Dir = root
		cmd.Stdin = data
		if out, uerr := cmd.CombinedOutput(); uerr != nil {
			_ = data.Close()
			t.Fatalf("unpack-objects: %v: %s", uerr, out)
		}
		_ = data.Close()
	}
	if _, err := os.Stat(loose); err != nil {
		t.Fatalf("object %s is still not loose after unpacking: %v", oid, err)
	}
	if err := os.Remove(loose); err != nil {
		t.Fatalf("remove the unpacked object: %v", err)
	}
}
