//go:build darwin

package candidatelock

import "testing"

func TestClassifyDarwinFilesystem(t *testing.T) {
	if err := classifyDarwinFilesystem("apfs", true); err != nil {
		t.Fatalf("apfs local rejected: %v", err)
	}
	if err := classifyDarwinFilesystem("apfs", false); !IsCode(err, CodeLockFilesystemUnsupported) {
		t.Fatalf("non-local apfs err=%v, want %s", err, CodeLockFilesystemUnsupported)
	}
	if err := classifyDarwinFilesystem("nfs", true); !IsCode(err, CodeLockFilesystemUnsupported) {
		t.Fatalf("nfs local err=%v, want %s", err, CodeLockFilesystemUnsupported)
	}
}
