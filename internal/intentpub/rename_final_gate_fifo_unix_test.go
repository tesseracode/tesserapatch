//go:build !windows

package intentpub

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestPIB150RenameFinalGateRefusesFIFOWithoutOpeningIt(t *testing.T) {
	t.Run("PIB-150-final-fifo-no-open", func(t *testing.T) {
		workspace, authority := acquireWorkspace(t)
		rel := ".tpatch/features/rename-gate/artifacts/fifo.json"
		if err := authority.WithRoot(func(root *os.Root) error {
			return mkdirChain(NewRootOps(root), filepath.ToSlash(filepath.Dir(rel)))
		}); err != nil {
			t.Fatal(err)
		}
		var hookErr error
		gateCalls := 0
		restoreBeforeRename(t, func(index int) {
			gateCalls++
			if index != 1 {
				hookErr = errors.New("beforeRename received the wrong entry index")
				return
			}
			hookErr = syscall.Mkfifo(filepath.Join(workspace, filepath.FromSlash(rel)), 0o600)
		})

		done := make(chan struct{})
		var result WriteResult
		var writeErr error
		spy := &fifoFinalGateOpenSpy{target: rel}
		go func() {
			result, writeErr = DurableWrite(authority, WriteRequest{
				Rel:        rel,
				Data:       []byte("ours\n"),
				Mode:       0o644,
				Expected:   identityPointer(AbsentIdentity()),
				Indexed:    true,
				EntryIndex: 1,
				Role:       WriteRoleOrdinaryCanonical,
			}, Options{
				RandomHex12: fixedHex("fedcfedcfedc"),
				RootOpsFactory: func(root *os.Root) RootOps {
					spy.RootOps = NewRootOps(root)
					return spy
				},
			})
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("rename-time FIFO was opened and blocked the writer")
		}
		if hookErr != nil {
			t.Fatal(hookErr)
		}
		if gateCalls != 1 {
			t.Fatalf("beforeRename gate calls = %d, want 1", gateCalls)
		}
		if spy.targetOpens != 0 {
			t.Fatalf("canonical FIFO target was opened %d time(s)", spy.targetOpens)
		}
		var typed *Error
		if writeErr == nil || !errors.As(writeErr, &typed) || typed.ExitClass != 5 || result.Committed {
			t.Fatalf("FIFO refusal = result=%+v err=%v", result, writeErr)
		}
		info, err := os.Lstat(filepath.Join(workspace, filepath.FromSlash(rel)))
		if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
			t.Fatalf("FIFO changed: %v %v", info, err)
		}
	})
}

type fifoFinalGateOpenSpy struct {
	RootOps
	target      string
	targetOpens int
}

func (ops *fifoFinalGateOpenSpy) Open(name string) (RootFile, error) {
	if name == ops.target {
		ops.targetOpens++
		return nil, errors.New("canonical FIFO target must not be opened")
	}
	return ops.RootOps.Open(name)
}

func (ops *fifoFinalGateOpenSpy) OpenFile(name string, flag int, mode fs.FileMode) (RootFile, error) {
	if name == ops.target {
		ops.targetOpens++
		return nil, errors.New("canonical FIFO target must not be opened")
	}
	return ops.RootOps.OpenFile(name, flag, mode)
}
