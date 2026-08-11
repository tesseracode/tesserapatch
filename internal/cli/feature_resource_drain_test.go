// Rev-2: matrix row 180 driven through the REAL `tpatch feature
// resource capture` command.
//
// Rev-1 proved the drain timeout at `Engine.Stage`, which bypasses the
// CLI's own lock acquisition and publication orchestration — so its
// "publishes nothing" and "lock released" claims were about a layer
// that never holds the lock or publishes in the first place. This
// drives the actual command.

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/rescap"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// escapingDoltFixture is a fake adapter binary that can be switched
// between two behaviours WITHOUT changing its bytes, so the trust pin
// stays valid across both phases of the test:
//
//   - while the escape marker exists, it prints a valid body and then
//     forks a copy of itself with SysProcAttr{Setsid: true} holding the
//     inherited stdout, so the parent's read end never reaches EOF even
//     after the whole original process group is killed and the leader
//     reaped;
//   - once the marker is removed, it just prints the body and exits.
//
// Switching by marker rather than by rewriting the binary is what lets
// the final "a subsequent normal capture succeeds" assertion run
// against the same pinned digest.
type escapingDoltFixture struct {
	Path         string
	EscapeMarker string
	ReleaseFile  string
}

func buildEscapingDoltCLIFixture(t *testing.T, hold time.Duration) escapingDoltFixture {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH to build the escaping adapter fixture")
	}
	srcDir := t.TempDir()
	binDir := t.TempDir()
	ctlDir := t.TempDir()
	bin := filepath.Join(binDir, "dolt")
	escapeMarker := filepath.Join(ctlDir, "escape.on")
	releaseFile := filepath.Join(ctlDir, "release.now")

	src := `package main

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

const (
	selfPath     = ` + strconv.Quote(bin) + `
	escapeMarker = ` + strconv.Quote(escapeMarker) + `
	releaseFile  = ` + strconv.Quote(releaseFile) + `
	holdFor      = ` + strconv.FormatInt(int64(hold), 10) + ` * time.Nanosecond
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--hold-stdout" {
		deadline := time.Now().Add(holdFor)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(releaseFile); err == nil {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		return
	}
	_, _ = os.Stdout.WriteString(` + "`" + `{"rows":[]}` + "`" + `)
	if _, err := os.Stat(escapeMarker); err != nil {
		// Normal mode: exit cleanly so the pipe reaches EOF.
		os.Exit(0)
	}
	child := exec.Command(selfPath, "--hold-stdout")
	child.Stdout = os.Stdout
	// Setsid is applied by the child between fork and exec, so the
	// escape is complete before Start returns and cannot race the
	// finalizer's group signal.
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := child.Start(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
`
	if err := os.WriteFile(filepath.Join(srcDir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "go.mod"), []byte("module escapingdolt\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("write fixture go.mod: %v", err)
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = srcDir
	cmd.Env = append(os.Environ(), "GOFLAGS=", "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("could not build the escaping adapter fixture: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		// Let any escaped grandchild exit promptly once the test is done.
		_ = os.WriteFile(releaseFile, []byte("x"), 0o644)
	})
	return escapingDoltFixture{Path: bin, EscapeMarker: escapeMarker, ReleaseFile: releaseFile}
}

// TestCaptureCLIDrainTimeoutFromEscapedWriter covers matrix row 180 at
// the real CLI boundary: `tpatch feature resource capture` must refuse
// `adapter-drain-timeout` (exit 3), publish no batch and no pointer,
// release the per-slug lock, and leave the feature capturable again.
func TestCaptureCLIDrainTimeoutFromEscapedWriter(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a helper binary and waits on real process bounds; skipped under -short")
	}
	dir := resourceTestRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "data", "dolt-db"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	fixture := buildEscapingDoltCLIFixture(t, 30*time.Second)
	restore := rescap.SetLookPathForTest(func(string) (string, error) { return fixture.Path, nil })
	defer restore()

	// Declare the Dolt resource; --trust-current-dolt pins the fixture.
	if _, stderr, code := runCmdExit(doltAddArgs(dir, "data/dolt-db", nil)...); code != 0 {
		t.Fatalf("add: %d %s", code, stderr)
	}

	// Phase 1: escaping mode. The grandchild holds the inherited
	// stdout, so the bounded drain genuinely expires.
	if err := os.WriteFile(fixture.EscapeMarker, []byte("on"), 0o644); err != nil {
		t.Fatalf("arm escape marker: %v", err)
	}

	started := time.Now()
	_, stderr, code := runCmdExit("feature", "resource", "capture", "--path", dir, "model-picker")
	elapsed := time.Since(started)

	if code != rescap.ExitRefusal {
		t.Fatalf("exit = %d after %v, want 3 (stderr: %s)", code, elapsed, stderr)
	}
	if !strings.Contains(stderr, rescap.ReasonAdapterDrainTimeout) {
		t.Fatalf("stderr = %q, want %s", stderr, rescap.ReasonAdapterDrainTimeout)
	}

	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	if _, statErr := os.Stat(s.ResourceCurrentPath("model-picker")); !os.IsNotExist(statErr) {
		t.Fatal("a drain timeout must publish no pointer")
	}
	if entries, readErr := os.ReadDir(s.ResourceBatchesDir("model-picker")); readErr == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".json") {
				t.Fatalf("a drain timeout must publish no batch, found %s", e.Name())
			}
		}
	}
	// The ephemeral scratch is cleaned up even on this failure path.
	assertNoEphemeralScratch(t, dir)

	// The per-slug lock was released by the refusal: it is acquirable
	// from outside, and — more importantly — the command itself can run
	// again without hitting capture-in-progress.
	lock, err := rescap.AcquireLock(rescap.ScratchRoot(dir, "model-picker"), dir)
	if err != nil {
		t.Fatalf("the CLI did not release the per-slug lock: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}

	// Phase 2: disarm the escape. The SAME binary (same pinned digest)
	// now exits cleanly, and a normal capture must succeed.
	if err := os.Remove(fixture.EscapeMarker); err != nil {
		t.Fatalf("disarm escape marker: %v", err)
	}
	stdout, stderr, code := runCmdExit("feature", "resource", "capture", "--path", dir, "model-picker", "--json")
	if code != 0 {
		t.Fatalf("a subsequent normal capture must succeed: %d %s", code, stderr)
	}
	if !strings.Contains(stdout, `"wrote_batch": true`) {
		t.Fatalf("the recovery capture should have published a batch: %s", stdout)
	}
	if _, statErr := os.Stat(s.ResourceCurrentPath("model-picker")); statErr != nil {
		t.Fatalf("the recovery capture must publish the pointer: %v", statErr)
	}
}
