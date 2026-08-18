//go:build (linux && !android) || (darwin && !ios)

package intentlock

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestAcquireFailureStagesStopBeforeLaterWork(t *testing.T) {
	sentinel := errors.New("injected")
	tests := []struct {
		name       string
		mutate     func(*authorityOps, *[]string)
		wantEvents []string
		wantCode   Code
		wantClass  string
	}{
		{
			name: "OpenRoot",
			mutate: func(ops *authorityOps, events *[]string) {
				ops.openRoot = func(string) (*os.Root, error) {
					*events = append(*events, "open-root")
					return nil, sentinel
				}
			},
			wantEvents: []string{"open-root"},
			wantCode:   CodeDirectoryFlockUnavailable,
			wantClass:  "open-root",
		},
		{
			name: "OpenDot",
			mutate: func(ops *authorityOps, events *[]string) {
				ops.openDir = func(*os.Root) (*os.File, error) {
					*events = append(*events, "open-directory")
					return nil, sentinel
				}
			},
			wantEvents: []string{"open-root", "open-directory", "close-root"},
			wantCode:   CodeDirectoryFlockUnavailable,
			wantClass:  "open-directory",
		},
		{
			name: "Identity",
			mutate: func(ops *authorityOps, events *[]string) {
				ops.fileIdentity = func(*os.File) (nativeIdentity, error) {
					*events = append(*events, "identity")
					return nativeIdentity{}, sentinel
				}
			},
			wantEvents: []string{
				"open-root", "open-directory", "identity",
				"close-directory", "close-root",
			},
			wantCode:  CodeDirectoryFlockUnavailable,
			wantClass: "identity-capture",
		},
		{
			name: "FstatfsOrControl",
			mutate: func(ops *authorityOps, events *[]string) {
				ops.classify = func(*os.File) (string, bool, error) {
					*events = append(*events, "fstatfs-control")
					return "", false, sentinel
				}
			},
			wantEvents: []string{
				"open-root", "open-directory", "identity", "fstatfs-control",
				"close-directory", "close-root",
			},
			wantCode:  CodeDirectoryFlockUnavailable,
			wantClass: "filesystem-classification",
		},
		{
			name: "DeniedClass",
			mutate: func(ops *authorityOps, events *[]string) {
				ops.classify = func(*os.File) (string, bool, error) {
					*events = append(*events, "fstatfs-control")
					return "nfs", true, nil
				}
			},
			wantEvents: []string{
				"open-root", "open-directory", "identity", "fstatfs-control",
				"close-directory", "close-root",
			},
			wantCode:  CodeLockFilesystemUnsupported,
			wantClass: "nfs",
		},
		{
			name: "FlockNonContention",
			mutate: func(ops *authorityOps, events *[]string) {
				ops.lock = func(*os.File) error {
					*events = append(*events, "flock-control")
					return syscall.EPERM
				}
			},
			wantEvents: []string{
				"open-root", "open-directory", "identity", "fstatfs-control",
				"flock-control", "close-directory", "close-root",
			},
			wantCode:  CodeDirectoryFlockUnavailable,
			wantClass: "flock",
		},
		{
			name: "FlockContention",
			mutate: func(ops *authorityOps, events *[]string) {
				ops.lock = func(*os.File) error {
					*events = append(*events, "flock-control")
					return syscall.EWOULDBLOCK
				}
			},
			wantEvents: []string{
				"open-root", "open-directory", "identity", "fstatfs-control",
				"flock-control", "close-directory", "close-root",
			},
			wantCode:  CodeTransactionInProgress,
			wantClass: "workspace",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			events := []string{}
			ops := instrumentAuthorityOps(defaultAuthorityOps, &events)
			test.mutate(&ops, &events)
			authority, err := acquireWithOps(t.TempDir(), ops)
			if authority != nil {
				_ = authority.Release()
				t.Fatal("failure injection acquired authority")
			}
			assertCode(t, err, test.wantCode)
			var typed *Error
			if !errors.As(err, &typed) || typed.Class != test.wantClass {
				t.Fatalf("error class = %q, want %q (%v)", typed.Class, test.wantClass, err)
			}
			if !equalStrings(events, test.wantEvents) {
				t.Fatalf("events = %v, want %v", events, test.wantEvents)
			}
		})
	}
}

func TestTypedErrorsDoNotLeakWorkspacePath(t *testing.T) {
	workspace := t.TempDir()
	ops := defaultAuthorityOps
	ops.openRoot = func(string) (*os.Root, error) {
		return nil, &os.PathError{Op: "open", Path: workspace, Err: syscall.EACCES}
	}
	_, err := acquireWithOps(workspace, ops)
	var typed *Error
	if !errors.As(err, &typed) {
		t.Fatalf("expected typed error, got %T", err)
	}
	for _, value := range []string{err.Error(), typed.Class, typed.Detail} {
		if strings.Contains(value, workspace) {
			t.Fatalf("error leaked workspace path: %q", value)
		}
	}
}

func TestDeniedFilesystemClassIsSanitized(t *testing.T) {
	workspace := t.TempDir()
	ops := defaultAuthorityOps
	ops.classify = func(*os.File) (string, bool, error) {
		return workspace, true, nil
	}
	_, err := acquireWithOps(workspace, ops)
	var typed *Error
	if !errors.As(err, &typed) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Code != CodeLockFilesystemUnsupported || typed.Class != "unknown" {
		t.Fatalf("denied error = %#v", typed)
	}
	if strings.Contains(err.Error(), workspace) {
		t.Fatalf("denied error leaked workspace path: %v", err)
	}
}

func TestAcquireOrdersIdentityBeforeClassificationAndFlockOnSameRetainedFile(t *testing.T) {
	ops := defaultAuthorityOps
	var events []string
	var identified *os.File
	var classified *os.File
	var locked *os.File
	realIdentity := ops.fileIdentity
	realClassify := ops.classify
	realLock := ops.lock
	ops.fileIdentity = func(file *os.File) (nativeIdentity, error) {
		events = append(events, "identity")
		identified = file
		return realIdentity(file)
	}
	ops.classify = func(file *os.File) (string, bool, error) {
		events = append(events, "fstatfs-control")
		classified = file
		return realClassify(file)
	}
	ops.lock = func(file *os.File) error {
		events = append(events, "flock-control")
		locked = file
		return realLock(file)
	}
	authority, err := acquireWithOps(t.TempDir(), ops)
	if err != nil {
		t.Fatalf("acquireWithOps: %v", err)
	}
	defer func() { _ = authority.Release() }()

	if !equalStrings(events, []string{"identity", "fstatfs-control", "flock-control"}) {
		t.Fatalf("events = %v", events)
	}
	if identified == nil || identified != classified || classified != locked || locked != authority.directory {
		t.Fatal("identity, classification, and flock did not use the retained directory file")
	}
}

func TestAcquireSuccessRevalidatesUnchangedOriginalPath(t *testing.T) {
	var events []string
	ops := instrumentAuthorityOps(defaultAuthorityOps, &events)
	authority, err := acquireWithOps(t.TempDir(), ops)
	if err != nil {
		t.Fatalf("acquireWithOps: %v", err)
	}
	if err := authority.ValidateOriginalPath(false); err != nil {
		_ = authority.Release()
		t.Fatalf("ValidateOriginalPath unchanged: %v", err)
	}
	if err := authority.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	want := []string{
		"open-root", "open-directory", "identity", "fstatfs-control",
		"flock-control", "path-identity", "unlock-control",
		"close-directory", "close-root",
	}
	if !equalStrings(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestReleaseOrderUsesRetainedFileControlThenCloses(t *testing.T) {
	authority, err := Acquire(t.TempDir())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	var events []string
	realUnlock := authority.ops.unlock
	realCloseFile := authority.ops.closeFile
	realCloseRoot := authority.ops.closeRoot
	authority.ops.unlock = func(file *os.File) error {
		events = append(events, "unlock-control")
		if file != authority.directory {
			t.Fatal("unlock did not use retained directory file")
		}
		return realUnlock(file)
	}
	authority.ops.closeFile = func(file *os.File) error {
		events = append(events, "close-directory")
		return realCloseFile(file)
	}
	authority.ops.closeRoot = func(root *os.Root) error {
		events = append(events, "close-root")
		return realCloseRoot(root)
	}
	if err := authority.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	want := []string{"unlock-control", "close-directory", "close-root"}
	if !equalStrings(events, want) {
		t.Fatalf("release events = %v, want %v", events, want)
	}
}

func TestReleaseContinuesCleanupAndReturnsFirstStableError(t *testing.T) {
	sentinel := errors.New("injected cleanup failure")
	tests := []struct {
		name          string
		failUnlock    bool
		failCloseFile bool
		failCloseRoot bool
		wantClass     string
	}{
		{name: "unlock", failUnlock: true, wantClass: "unlock"},
		{name: "close-file", failCloseFile: true, wantClass: "close-directory"},
		{name: "close-root", failCloseRoot: true, wantClass: "close-root"},
		{
			name:          "all-first-error-wins",
			failUnlock:    true,
			failCloseFile: true,
			failCloseRoot: true,
			wantClass:     "unlock",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authority, err := Acquire(t.TempDir())
			if err != nil {
				t.Fatalf("Acquire: %v", err)
			}
			var events []string
			realUnlock := authority.ops.unlock
			realCloseFile := authority.ops.closeFile
			realCloseRoot := authority.ops.closeRoot
			authority.ops.unlock = func(file *os.File) error {
				events = append(events, "unlock-control")
				realErr := realUnlock(file)
				if realErr != nil {
					return realErr
				}
				if test.failUnlock {
					return sentinel
				}
				return nil
			}
			authority.ops.closeFile = func(file *os.File) error {
				events = append(events, "close-directory")
				realErr := realCloseFile(file)
				if realErr != nil {
					return realErr
				}
				if test.failCloseFile {
					return sentinel
				}
				return nil
			}
			authority.ops.closeRoot = func(root *os.Root) error {
				events = append(events, "close-root")
				realErr := realCloseRoot(root)
				if realErr != nil {
					return realErr
				}
				if test.failCloseRoot {
					return sentinel
				}
				return nil
			}

			err = authority.Release()
			assertCode(t, err, CodeDirectoryFlockUnavailable)
			var typed *Error
			if !errors.As(err, &typed) || typed.Class != test.wantClass {
				t.Fatalf("error class = %q, want %q (%v)", typed.Class, test.wantClass, err)
			}
			want := []string{"unlock-control", "close-directory", "close-root"}
			if !equalStrings(events, want) {
				t.Fatalf("events = %v, want %v", events, want)
			}
		})
	}
}

func instrumentAuthorityOps(ops authorityOps, events *[]string) authorityOps {
	realOpenRoot := ops.openRoot
	realOpenDir := ops.openDir
	realIdentity := ops.fileIdentity
	realClassify := ops.classify
	realLock := ops.lock
	realUnlock := ops.unlock
	realPathIdentity := ops.pathIdentity
	realCloseFile := ops.closeFile
	realCloseRoot := ops.closeRoot
	ops.openRoot = func(path string) (*os.Root, error) {
		*events = append(*events, "open-root")
		return realOpenRoot(path)
	}
	ops.openDir = func(root *os.Root) (*os.File, error) {
		*events = append(*events, "open-directory")
		return realOpenDir(root)
	}
	ops.fileIdentity = func(file *os.File) (nativeIdentity, error) {
		*events = append(*events, "identity")
		return realIdentity(file)
	}
	ops.classify = func(file *os.File) (string, bool, error) {
		*events = append(*events, "fstatfs-control")
		return realClassify(file)
	}
	ops.lock = func(file *os.File) error {
		*events = append(*events, "flock-control")
		return realLock(file)
	}
	ops.unlock = func(file *os.File) error {
		*events = append(*events, "unlock-control")
		return realUnlock(file)
	}
	ops.pathIdentity = func(path string) (nativeIdentity, error) {
		*events = append(*events, "path-identity")
		return realPathIdentity(path)
	}
	ops.closeFile = func(file *os.File) error {
		*events = append(*events, "close-directory")
		return realCloseFile(file)
	}
	ops.closeRoot = func(root *os.Root) error {
		*events = append(*events, "close-root")
		return realCloseRoot(root)
	}
	return ops
}
