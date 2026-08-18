//go:build (linux && !android) || (darwin && !ios)

package intentlock

import (
	"os"
	"syscall"
)

func identityFromFile(file *os.File) (nativeIdentity, error) {
	info, err := file.Stat()
	if err != nil {
		return nativeIdentity{}, err
	}
	return identityFromInfo(info)
}

func identityFromPath(path string) (nativeIdentity, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nativeIdentity{}, err
	}
	return identityFromInfo(info)
}

func identityFromInfo(info os.FileInfo) (nativeIdentity, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nativeIdentity{}, syscall.EINVAL
	}
	return nativeIdentity{device: uint64(stat.Dev), inode: uint64(stat.Ino)}, nil
}
