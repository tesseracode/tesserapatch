//go:build windows

package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func operationCandidateSyncDirectory(root *os.Root) error {
	identity, err := root.Stat(".")
	if err != nil {
		return err
	}
	directory, err := filepath.Abs(root.Name())
	if err != nil {
		return err
	}
	if !strings.HasPrefix(directory, `\\?\`) {
		if strings.HasPrefix(directory, `\\`) {
			directory = `\\?\UNC\` + directory[2:]
		} else {
			directory = `\\?\` + directory
		}
	}
	name, err := syscall.UTF16PtrFromString(directory)
	if err != nil {
		return err
	}
	// FlushFileBuffers needs write access, unlike the read-only directory
	// handle returned by os.Root.Open. Never ignore a failed durability call.
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	dir := os.NewFile(uintptr(handle), root.Name())
	actual, err := dir.Stat()
	if err != nil || actual == nil || !os.SameFile(identity, actual) || actual.Mode()&os.ModeSymlink != 0 {
		_ = dir.Close()
		return errors.Join(fmt.Errorf("acceptance directory changed before flush"), err)
	}
	return errors.Join(syscall.FlushFileBuffers(handle), dir.Close())
}
