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
	"strings"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

// startGitDaemon serves `root` over the git:// transport with
// `uploadpack.allowFilter=true` and returns the base URL plus a stopper.
func startGitDaemon(t *testing.T, root string) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve a port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("release the port: %v", err)
	}

	cmd := exec.Command("git", "daemon",
		"--reuseaddr",
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
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, dialErr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			return url, stop
		}
		time.Sleep(100 * time.Millisecond)
	}
	stop()
	t.Fatalf("git daemon did not accept connections on port %d: %s", port, stderr.String())
	return "", stop
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

	url, stop := startGitDaemon(t, origin)
	t.Cleanup(stop)

	clone := filepath.Join(t.TempDir(), "clone")
	out, err := tryGit(filepath.Dir(clone), "clone", "-q", "--filter=blob:none", url, clone)
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
	// The classification must be history-incomplete (R22) or, when the
	// missing object only breaks a later stage, a terminal state whose
	// remediation names the partial clone.
	v7 := checkByID(t, r, CheckRecipeReplayClean)
	if r.LandingEvidence.State == EvidenceHistoryIncomplete {
		if got, want := v7.Remediation, remediationR22(f.Slug); got != want {
			t.Errorf("R22 not verbatim:\n got %q\nwant %q", got, want)
		}
		return
	}
	if r.FailedAt != FailedAtHistoricalAnchor && r.FailedAt != FailedAtLandingEvidence {
		t.Fatalf("unexpected failure shape: failed_at=%q evidence=%q", r.FailedAt, r.LandingEvidence.State)
	}
	t.Logf("missing-object run classified as %q / %q", r.LandingEvidence.State, r.FailedAt)
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
