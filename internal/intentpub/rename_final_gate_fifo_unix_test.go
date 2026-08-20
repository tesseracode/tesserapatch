//go:build !windows

package intentpub

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestPIB150RenameFinalGateRefusesFIFOWithoutOpeningIt(t *testing.T) {
	workspace, authority := acquireWorkspace(t)
	rel := ".tpatch/features/rename-gate/artifacts/fifo.json"
	if err := authority.WithRoot(func(root *os.Root) error {
		return mkdirChain(NewRootOps(root), filepath.ToSlash(filepath.Dir(rel)))
	}); err != nil {
		t.Fatal(err)
	}
	var hookErr error
	restoreBeforeRename(t, func(index int) {
		if index != 1 {
			hookErr = errors.New("beforeRename received the wrong entry index")
			return
		}
		hookErr = syscall.Mkfifo(filepath.Join(workspace, filepath.FromSlash(rel)), 0o600)
	})

	done := make(chan struct{})
	var result WriteResult
	var writeErr error
	go func() {
		result, writeErr = DurableWrite(authority, WriteRequest{
			Rel:        rel,
			Data:       []byte("ours\n"),
			Mode:       0o644,
			Expected:   identityPointer(AbsentIdentity()),
			Indexed:    true,
			EntryIndex: 1,
			Role:       WriteRoleOrdinaryCanonical,
		}, Options{RandomHex12: fixedHex("fedcfedcfedc")})
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
	var typed *Error
	if writeErr == nil || !errors.As(writeErr, &typed) || typed.ExitClass != 5 || result.Committed {
		t.Fatalf("FIFO refusal = result=%+v err=%v", result, writeErr)
	}
	info, err := os.Lstat(filepath.Join(workspace, filepath.FromSlash(rel)))
	if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("FIFO changed: %v %v", info, err)
	}
}
