//go:build (linux && !android) || (darwin && !ios)

package intentpub

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

type tempCleanupIdentity struct {
	created syscall.Stat_t
	valid   bool
}

func descriptorTempCleanupSupported() bool {
	return canRemoveAtHeldDirectory()
}

func canRemoveAtHeldDirectory() bool {
	return true
}

func retainAndVerifyTemp(
	directory RootFile,
	file RootFile,
	name string,
) (RootFile, tempCleanupIdentity, error) {
	held, err := duplicateRootFile(file)
	if err != nil {
		return nil, tempCleanupIdentity{}, err
	}
	identity, err := verifyTempAtHeldDirectory(directory, held, name, tempCleanupIdentity{})
	if err != nil {
		_ = held.Close()
		return nil, tempCleanupIdentity{}, err
	}
	return held, identity, nil
}

func verifyTempAtHeldDirectory(
	directory RootFile,
	file RootFile,
	name string,
	identity tempCleanupIdentity,
) (tempCleanupIdentity, error) {
	var captured tempCleanupIdentity
	err := withRootFileDescriptor(file, func(fileDescriptor uintptr) error {
		var fileStat syscall.Stat_t
		if err := syscall.Fstat(int(fileDescriptor), &fileStat); err != nil {
			return err
		}
		if !regularSingleLink(fileStat) {
			return fs.ErrInvalid
		}
		if identity.valid && !sameCreatedTemp(identity.created, fileStat) {
			return fs.ErrInvalid
		}
		return withRootFileDescriptor(directory, func(directoryDescriptor uintptr) error {
			pathStat, err := platformStatAt(directoryDescriptor, name)
			if err != nil {
				return err
			}
			if !sameTempMutationMetadata(fileStat, pathStat) {
				return fs.ErrInvalid
			}
			captured.created = fileStat
			captured.valid = true
			return nil
		})
	})
	return captured, err
}

func removeVerifiedTempAtHeldDirectory(
	directory RootFile,
	file RootFile,
	name string,
	identity tempCleanupIdentity,
) error {
	return withRootFileDescriptor(file, func(fileDescriptor uintptr) error {
		var fileStat syscall.Stat_t
		if err := syscall.Fstat(int(fileDescriptor), &fileStat); err != nil {
			return err
		}
		if !regularSingleLink(fileStat) || !sameCreatedTemp(identity.created, fileStat) {
			return fs.ErrInvalid
		}
		return withRootFileDescriptor(directory, func(directoryDescriptor uintptr) error {
			pathStat, err := platformStatAt(directoryDescriptor, name)
			if err != nil {
				return err
			}
			if !sameTempMutationMetadata(fileStat, pathStat) {
				return fs.ErrInvalid
			}
			return unlinkAt(directoryDescriptor, name)
		})
	})
}

func verifyTempContentAtHeldDirectory(
	directory RootFile,
	file RootFile,
	name string,
	identity tempCleanupIdentity,
	intended Identity,
	scratch []byte,
) error {
	return withRootFileDescriptor(file, func(fileDescriptor uintptr) error {
		var before syscall.Stat_t
		if err := syscall.Fstat(int(fileDescriptor), &before); err != nil {
			return err
		}
		if !sameCreatedTemp(identity.created, before) ||
			before.Size != intended.Size ||
			uint32(before.Mode&0o777) != intended.Mode ||
			intended.Size < 0 ||
			intended.Size > MaxArtifactBytes ||
			len(scratch) < int(intended.Size)+1 {
			return fs.ErrInvalid
		}

		limit := int(intended.Size) + 1
		count, err := readTempContent(
			int(fileDescriptor), scratch, limit, syscall.Pread,
		)
		if err != nil {
			return err
		}
		if count != int(intended.Size) {
			return fs.ErrInvalid
		}
		sum := sha256.Sum256(scratch[:count])
		if hex.EncodeToString(sum[:]) != intended.SHA256 {
			return fs.ErrInvalid
		}
		if afterTempContentRead != nil {
			afterTempContentRead(name)
		}

		var after syscall.Stat_t
		if err := syscall.Fstat(int(fileDescriptor), &after); err != nil {
			return err
		}
		if !sameCreatedTemp(identity.created, after) ||
			after.Size != intended.Size ||
			uint32(after.Mode&0o777) != intended.Mode ||
			!sameTempMutationMetadata(before, after) {
			return fs.ErrInvalid
		}
		return withRootFileDescriptor(directory, func(directoryDescriptor uintptr) error {
			pathStat, err := platformStatAt(directoryDescriptor, name)
			if err != nil {
				return err
			}
			if !sameTempMutationMetadata(after, pathStat) {
				return fs.ErrInvalid
			}
			return nil
		})
	})
}

func readTempContent(
	fileDescriptor int,
	scratch []byte,
	limit int,
	pread func(int, []byte, int64) (int, error),
) (int, error) {
	count := 0
	for count < limit {
		read, err := pread(fileDescriptor, scratch[count:limit], int64(count))
		if read > 0 {
			count += read
		} else if read < 0 && err == nil {
			return count, fs.ErrInvalid
		}
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			return count, err
		}
		if read == 0 {
			break
		}
	}
	return count, nil
}

func duplicateRootFile(file RootFile) (RootFile, error) {
	var duplicate int
	err := withRootFileDescriptor(file, func(descriptor uintptr) error {
		var err error
		duplicate, err = syscall.Dup(int(descriptor))
		if err == nil {
			syscall.CloseOnExec(duplicate)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	held := os.NewFile(uintptr(duplicate), "tpatch-intentpub-temp")
	if held == nil {
		_ = syscall.Close(duplicate)
		return nil, fs.ErrInvalid
	}
	return held, nil
}

func withRootFileDescriptor(file RootFile, action func(uintptr) error) error {
	provider, ok := file.(interface {
		SyscallConn() (syscall.RawConn, error)
	})
	if !ok {
		return fs.ErrInvalid
	}
	connection, err := provider.SyscallConn()
	if err != nil {
		return err
	}
	var actionErr error
	if err := connection.Control(func(descriptor uintptr) {
		actionErr = action(descriptor)
	}); err != nil {
		return err
	}
	runtime.KeepAlive(file)
	return actionErr
}

func rawStatAt(directory uintptr, name string, trap, noFollow uintptr) (syscall.Stat_t, error) {
	var stat syscall.Stat_t
	pointer, err := syscall.BytePtrFromString(name)
	if err != nil {
		return stat, err
	}
	_, _, errno := syscall.Syscall6(
		trap,
		directory,
		uintptr(unsafe.Pointer(pointer)),
		uintptr(unsafe.Pointer(&stat)),
		noFollow,
		0,
		0,
	)
	if errno != 0 {
		return stat, errno
	}
	return stat, nil
}

func unlinkAt(directory uintptr, name string) error {
	pointer, err := syscall.BytePtrFromString(name)
	if err != nil {
		return err
	}
	_, _, errno := syscall.Syscall6(
		unlinkatTrap,
		directory,
		uintptr(unsafe.Pointer(pointer)),
		0,
		0,
		0,
		0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func regularSingleLink(stat syscall.Stat_t) bool {
	return stat.Mode&syscall.S_IFMT == syscall.S_IFREG && stat.Nlink == 1
}

func sameCreatedTemp(created, current syscall.Stat_t) bool {
	return regularSingleLink(created) &&
		regularSingleLink(current) &&
		created.Dev == current.Dev &&
		created.Ino == current.Ino
}
