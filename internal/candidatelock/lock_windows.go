//go:build windows

package candidatelock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// Supported reports whether this build target has a native advisory lock.
const Supported = true

var acquireFilesystemClassifier = classifyLockFilesystem

const (
	driveUnknown   = 0
	driveNoRootDir = 1
	driveRemovable = 2
	driveFixed     = 3
	driveRemote    = 4
	driveCDROM     = 5
	driveRAMDisk   = 6
)

const (
	lockfileFailImmediately = 0x00000001
	lockfileExclusive       = 0x00000002
)

const windowsErrLockViolation syscall.Errno = 33

var (
	kernel32DLL      = syscall.NewLazyDLL("kernel32.dll")
	lockFileExProc   = kernel32DLL.NewProc("LockFileEx")
	getDriveTypeProc = kernel32DLL.NewProc("GetDriveTypeW")
)

// TryAcquire takes a nonblocking exclusive advisory lock on path.
func TryAcquire(path string) (*Handle, error) {
	lockPath, err := canonicalLockPath(path)
	if err != nil {
		return nil, err
	}
	ancestor, err := nearestExistingAncestor(filepath.Dir(lockPath))
	if err != nil {
		return nil, err
	}
	if err := acquireFilesystemClassifier(ancestor); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := ensureLockSentinelByte(file); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := lockFileEx(file); err != nil {
		_ = file.Close()
		if errors.Is(err, windowsErrLockViolation) {
			return nil, &Error{
				Code:   CodeCandidateLockHeld,
				Detail: "another process already holds this candidate lock",
			}
		}
		return nil, fmt.Errorf("lock file: %w", err)
	}
	return &Handle{file: file, path: lockPath}, nil
}

func ensureLockSentinelByte(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat lock file: %w", err)
	}
	if info.Size() > 0 {
		return nil
	}
	if _, err := file.WriteAt([]byte{0}, 0); err != nil {
		return fmt.Errorf("seed lock file: %w", err)
	}
	return nil
}

func lockFileEx(file *os.File) error {
	var overlapped syscall.Overlapped
	handle := syscall.Handle(file.Fd())
	r1, _, e1 := lockFileExProc.Call(
		uintptr(handle),
		uintptr(lockfileExclusive|lockfileFailImmediately),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if r1 != 0 {
		return nil
	}
	if e1 != syscall.Errno(0) {
		return e1
	}
	return syscall.EINVAL
}

func classifyLockFilesystem(path string) error {
	abs := filepath.Clean(path)
	volume := filepath.VolumeName(abs)
	if strings.HasPrefix(volume, `\\`) {
		return &Error{
			Code:   CodeLockFilesystemUnsupported,
			Detail: "UNC paths are not supported for candidate locks",
		}
	}
	if volume == "" {
		return &Error{
			Code:   CodeLockFilesystemUnsupported,
			Detail: "path has no drive volume",
		}
	}
	root := volume + `\`
	ptr, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return fmt.Errorf("encode volume path: %w", err)
	}
	r1, _, e1 := getDriveTypeProc.Call(uintptr(unsafe.Pointer(ptr)))
	if r1 == 0 && e1 != syscall.Errno(0) {
		return fmt.Errorf("GetDriveTypeW(%s): %w", root, e1)
	}
	switch uint32(r1) {
	case driveFixed, driveRemovable, driveRAMDisk:
		return nil
	case driveRemote, driveUnknown, driveNoRootDir, driveCDROM:
		return &Error{
			Code:   CodeLockFilesystemUnsupported,
			Detail: fmt.Sprintf("drive type %d is not supported for candidate locks", r1),
		}
	default:
		return &Error{
			Code:   CodeLockFilesystemUnsupported,
			Detail: fmt.Sprintf("unrecognized drive type %d", r1),
		}
	}
}
