//go:build linux || darwin

package candidate

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
)

func readAcceptanceDirectoryIdentity(root *os.Root) (AcceptanceDirectoryIdentity, error) {
	info, err := root.Stat(".")
	if err != nil {
		return AcceptanceDirectoryIdentity{}, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() {
		return AcceptanceDirectoryIdentity{}, fmt.Errorf("acceptance identity requires a native directory")
	}
	identity := AcceptanceDirectoryIdentity{
		Platform: runtime.GOOS,
		Volume:   fmt.Sprintf("%016x", uint64(stat.Dev)),
		File:     fmt.Sprintf("%016x", uint64(stat.Ino)),
	}
	return identity, identity.validate()
}
