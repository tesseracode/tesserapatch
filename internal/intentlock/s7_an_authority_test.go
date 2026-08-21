//go:build (linux && !android) || (darwin && !ios)

package intentlock

import (
	"errors"
	"os"
	"syscall"
	"testing"
)

func TestS7PIB410OneRetainedDirectorySuppliesAuthorityAndRootedIO(t *testing.T) {
	workspace := t.TempDir()
	ops := defaultAuthorityOps
	realOpenRoot := ops.openRoot
	realOpenDir := ops.openDir
	realIdentity := ops.fileIdentity
	realClassify := ops.classify
	realLock := ops.lock
	openRoots := 0
	openDirs := 0
	var identified, classified, locked *os.File
	ops.openRoot = func(path string) (*os.Root, error) {
		openRoots++
		return realOpenRoot(path)
	}
	ops.openDir = func(root *os.Root) (*os.File, error) {
		openDirs++
		return realOpenDir(root)
	}
	ops.fileIdentity = func(file *os.File) (nativeIdentity, error) {
		identified = file
		return realIdentity(file)
	}
	ops.classify = func(file *os.File) (string, bool, error) {
		classified = file
		return realClassify(file)
	}
	ops.lock = func(file *os.File) error {
		locked = file
		return realLock(file)
	}
	authority, err := acquireWithOps(workspace, ops)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = authority.Release() }()
	var rooted *os.Root
	if err := authority.WithRoot(func(root *os.Root) error {
		rooted = root
		_, statErr := root.Stat(".")
		return statErr
	}); err != nil {
		t.Fatal(err)
	}
	if openRoots != 1 || openDirs != 1 || rooted != authority.root ||
		identified == nil || identified != classified || classified != locked ||
		locked != authority.directory {
		t.Fatalf(
			"authority construction roots=%d dirs=%d rooted=%p authority=%p identified=%p classified=%p locked=%p held=%p",
			openRoots, openDirs, rooted, authority.root, identified, classified, locked, authority.directory,
		)
	}
	contender, err := Acquire(workspace)
	if contender != nil {
		_ = contender.Release()
		t.Fatal("retained rooted authority did not hold the directory flock")
	}
	assertCode(t, err, CodeTransactionInProgress)
}

func TestS7PIB411OnlyWouldBlockAndAgainAreContention(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		code Code
	}{
		{name: "would-block", err: syscall.EWOULDBLOCK, code: CodeTransactionInProgress},
		{name: "again", err: syscall.EAGAIN, code: CodeTransactionInProgress},
		{name: "other", err: syscall.EPERM, code: CodeDirectoryFlockUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			workspace := t.TempDir()
			before := directoryNames(t, workspace)
			ops := defaultAuthorityOps
			lockCalls := 0
			ops.lock = func(*os.File) error {
				lockCalls++
				return test.err
			}
			authority, err := acquireWithOps(workspace, ops)
			if authority != nil {
				_ = authority.Release()
				t.Fatal("failed flock returned an unlocked authority")
			}
			assertCode(t, err, test.code)
			var typed *Error
			if !errors.As(err, &typed) || lockCalls != 1 {
				t.Fatalf("flock failure = calls:%d err:%v", lockCalls, err)
			}
			if got := directoryNames(t, workspace); !equalStrings(got, before) {
				t.Fatalf("flock failure mutated workspace: before=%v after=%v", before, got)
			}
		})
	}
}
