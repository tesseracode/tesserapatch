//go:build windows

package candidate

import (
	"fmt"
	"os"
	"syscall"
)

func readAcceptanceDirectoryIdentity(root *os.Root) (AcceptanceDirectoryIdentity, error) {
	file, err := root.Open(".")
	if err != nil {
		return AcceptanceDirectoryIdentity{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return AcceptanceDirectoryIdentity{}, err
	}
	expected, err := root.Stat(".")
	if err != nil || !info.IsDir() || !os.SameFile(info, expected) {
		return AcceptanceDirectoryIdentity{}, fmt.Errorf("acceptance identity requires its held directory")
	}
	var native syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(file.Fd()), &native); err != nil {
		return AcceptanceDirectoryIdentity{}, err
	}
	identity := AcceptanceDirectoryIdentity{
		Platform: "windows",
		Volume:   fmt.Sprintf("%016x", uint64(native.VolumeSerialNumber)),
		File:     fmt.Sprintf("%016x", uint64(native.FileIndexHigh)<<32|uint64(native.FileIndexLow)),
	}
	return identity, identity.validate()
}
