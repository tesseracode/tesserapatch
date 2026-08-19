//go:build unix

package intentpub

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestCaptureRacedWriterlessFIFODoesNotBlockOpen(t *testing.T) {
	workspace, authority := acquireWorkspace(t)
	rel := ".tpatch/features/fifo/value"
	rootWrite(t, authority, rel, []byte("regular"), 0o644)
	state := &fifoRaceState{
		absolute: filepath.Join(workspace, filepath.FromSlash(rel)),
	}

	done := make(chan error, 1)
	go func() {
		_, err := CaptureIdentity(authority, rel, Options{
			RootOpsFactory: func(root *os.Root) RootOps {
				return &fifoRaceOps{RootOps: NewRootOps(root), root: root, rel: rel, state: state}
			},
		})
		done <- err
	}()

	select {
	case err := <-done:
		assertCode(t, err, CodeIdentityUnstable)
	case <-time.After(2 * time.Second):
		// Unblock a regressed blocking FIFO open so the test process can cleanly
		// release its authority before failing.
		go func() {
			file, _ := os.OpenFile(state.absolute, os.O_WRONLY, 0)
			if file != nil {
				_ = file.Close()
			}
		}()
		t.Fatal("raced writer-less FIFO wedged the rooted open; O_NONBLOCK was not effective")
	}
}

func TestFIFOReplacementsNormalizeUndoAndRecoveryErrors(t *testing.T) {
	t.Run("undo", func(t *testing.T) {
		workspace, authority := acquireWorkspace(t)
		plan := stageCreatePlan(t, authority)
		state := &renameFailureState{failCanonicalAt: 2}
		changed := false
		result, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
			RandomHex12: sequenceHex(),
			RootOpsFactory: func(root *os.Root) RootOps {
				return &failingRenameOps{RootOps: NewRootOps(root), state: state}
			},
			Hook: func(point CrashPoint, root *os.Root, entry *Entry) error {
				if point != PointAfterEntryRename || entry == nil ||
					entry.ArtifactID != ArtifactAnalysis || changed {
					return nil
				}
				changed = true
				if err := root.Remove(entry.Rel); err != nil {
					return err
				}
				return syscall.Mkfifo(filepath.Join(workspace, filepath.FromSlash(entry.Rel)), 0o600)
			},
		})
		assertCode(t, err, CodeUndoCASMismatch)
		assertResultErrorExitAgreement(t, result, err, 6)
	})

	t.Run("recovery", func(t *testing.T) {
		workspace, authority := acquireWorkspace(t)
		plan := stageCreatePlan(t, authority)
		_, _ = Execute(authority, plan, "0123456789abcdef", nil, Options{
			RandomHex12: sequenceHex(),
			Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
				if point == PointAfterJournalDurable {
					return errors.New("crash")
				}
				return nil
			},
		})
		entry := plan.Entries()[0]
		if err := syscall.Mkfifo(filepath.Join(workspace, filepath.FromSlash(entry.Rel)), 0o600); err != nil {
			t.Fatal(err)
		}
		result, err := Recover(authority, testSlug, Options{})
		assertCode(t, err, CodeRecoveryDivergent)
		assertResultErrorExitAgreement(t, result, err, 6)
		if !rootExists(t, authority, JournalRel(testSlug)) {
			t.Fatal("FIFO recovery divergence removed evidence")
		}
	})
}

type fifoRaceState struct {
	once     sync.Once
	absolute string
	err      error
}

type fifoRaceOps struct {
	RootOps
	root  *os.Root
	rel   string
	state *fifoRaceState
}

func (ops *fifoRaceOps) OpenFile(name string, flag int, mode fs.FileMode) (RootFile, error) {
	if name == ops.rel && flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) == 0 {
		ops.state.once.Do(func() {
			if err := ops.root.Remove(name); err != nil {
				ops.state.err = err
				return
			}
			ops.state.err = syscall.Mkfifo(ops.state.absolute, 0o600)
		})
		if ops.state.err != nil {
			return nil, ops.state.err
		}
	}
	return ops.RootOps.OpenFile(name, flag, mode)
}
