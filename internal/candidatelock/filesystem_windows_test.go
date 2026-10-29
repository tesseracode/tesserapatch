//go:build windows

package candidatelock

import "testing"

func TestClassifyWindowsDriveType(t *testing.T) {
	for _, drive := range []uint32{driveFixed, driveRemovable, driveRAMDisk} {
		if err := classifyWindowsDriveType(drive); err != nil {
			t.Fatalf("drive type %d rejected: %v", drive, err)
		}
	}
	for _, drive := range []uint32{driveRemote, driveUnknown, driveNoRootDir, driveCDROM, 999} {
		if err := classifyWindowsDriveType(drive); !IsCode(err, CodeLockFilesystemUnsupported) {
			t.Fatalf("drive type %d err=%v, want %s", drive, err, CodeLockFilesystemUnsupported)
		}
	}
}
