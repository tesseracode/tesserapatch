//go:build linux

package candidatelock

import "testing"

func TestClassifyLinuxFilesystemType(t *testing.T) {
	for magic := range linuxAllowedFilesystems {
		if err := classifyLinuxFilesystemType(magic); err != nil {
			t.Fatalf("allowed magic 0x%x rejected: %v", magic, err)
		}
	}
	if err := classifyLinuxFilesystemType(0xdeadbeef); !IsCode(err, CodeLockFilesystemUnsupported) {
		t.Fatalf("unknown filesystem err=%v, want %s", err, CodeLockFilesystemUnsupported)
	}
}
