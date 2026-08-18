//go:build unix

package intent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// fifoOpenTripwire is AVP-200: a Go-upgrade tripwire, not a contract test.
//
// It verifies an UNEXPORTED implementation detail of `os.Root` —
// `rootOpenFileNolog` forwarding the caller's flags to `openat` (claims audit
// G17) — by opening a real writer-less FIFO through a real `*os.Root` under a
// hard deadline. Nothing in `os`'s documentation promises this. If a Go
// release stops forwarding the flag, this turns red at upgrade time instead of
// turning `prepare --check` into a field hang.
//
// The property is scoped to the *open*. It asserts nothing about read time
// (PRD §7.4.2, AVP-207).
func fifoOpenTripwire(t *testing.T, withNonblock bool) error {
	t.Helper()
	dir := t.TempDir()
	name := "pipe"
	if err := syscall.Mkfifo(filepath.Join(dir, name), 0o600); err != nil {
		return fmt.Errorf("mkfifo: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("open root: %w", err)
	}
	defer root.Close()

	flags := os.O_RDONLY
	if withNonblock {
		flags |= syscall.O_NONBLOCK
	}

	done := make(chan error, 1)
	go func() {
		file, openErr := root.OpenFile(name, flags, 0)
		if file != nil {
			_ = file.Close()
		}
		done <- openErr
	}()

	select {
	case <-done:
		if !withNonblock {
			// The sensitivity arm is expected to block; returning here means
			// the fixture no longer reproduces the blocking open.
			return errors.New("a blocking open returned, so the tripwire proves nothing")
		}
		return nil
	case <-time.After(3 * time.Second):
		if withNonblock {
			return errors.New("O_NONBLOCK no longer reaches openat: the FIFO open wedged")
		}
		return errors.New("the open without O_NONBLOCK blocked, as the tripwire predicts")
	}
}
