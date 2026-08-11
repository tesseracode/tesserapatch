//go:build darwin

// Darwin statfs tests (PRD §7.2, ADR-033 D9).
//
// Darwin's Statfs_t.Fstypename is an array of *signed* 8-bit integers,
// so converting it to a comparable Go string needs an explicit
// per-element cast and a trim at the first NUL. These tests exercise
// that conversion and the exact allow/deny lists without mounting
// anything.

package rescap

import (
	"os"
	"testing"
)

// TestFstypenameSignedConversion covers the [16]int8 handling.
func TestFstypenameSignedConversion(t *testing.T) {
	raw := make([]int8, 16)
	for i, c := range []byte("apfs") {
		raw[i] = int8(c)
	}
	if got := FstypenameString(raw); got != "apfs" {
		t.Fatalf("Fstypename = %q, want apfs", got)
	}
	// A high-bit byte round-trips through the signed representation.
	raw2 := []int8{int8(byte('h')), int8(byte('f')), int8(byte('s')), 0, int8(byte('X'))}
	if got := FstypenameString(raw2); got != "hfs" {
		t.Fatalf("Fstypename = %q, want hfs (trimmed at the first NUL)", got)
	}
	if got := FstypenameString(make([]int8, 4)); got != "" {
		t.Fatalf("all-NUL Fstypename = %q, want empty", got)
	}
}

// TestDarwinFilesystemAllowDenyLists pins the exact lists.
func TestDarwinFilesystemAllowDenyLists(t *testing.T) {
	cases := []struct {
		name    string
		denied  bool
		allowed bool
	}{
		{"apfs", false, true},
		{"hfs", false, true},
		{"tmpfs", false, true},
		{"nfs", true, false},
		{"smbfs", true, false},
		{"webdav", true, false},
		{"osxfuse", true, false},
		{"macfuse", true, false},
		{"somethingelse", false, false},
	}
	for _, tc := range cases {
		denied, allowed := ClassifyFilesystemName(tc.name)
		if denied != tc.denied || allowed != tc.allowed {
			t.Fatalf("%s: denied=%v allowed=%v, want denied=%v allowed=%v",
				tc.name, denied, allowed, tc.denied, tc.allowed)
		}
	}
}

// TestNoexecFlagBit pins Darwin's MNT_NOEXEC value.
func TestNoexecFlagBit(t *testing.T) {
	if !NoexecFlagSet(0x00000004) {
		t.Fatal("MNT_NOEXEC is bit 0x4")
	}
	if NoexecFlagSet(0x8) {
		t.Fatal("0x8 is not MNT_NOEXEC on darwin")
	}
}

// TestCheckFilesystemSupportedOnRealTempDir proves the preflight
// accepts an ordinary local temp directory on this host.
func TestCheckFilesystemSupportedOnRealTempDir(t *testing.T) {
	dir := t.TempDir()
	if err := CheckFilesystemSupported(dir); err != nil {
		t.Fatalf("a local temp dir should pass the preflight: %v", err)
	}
	if err := CheckScratchExecutable(dir); err != nil {
		t.Fatalf("a local temp dir should not be noexec: %v", err)
	}
	if err := CheckFilesystemSupported(dir + "/definitely/missing"); err == nil {
		t.Fatal("statfs on a missing path must refuse")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("the preflight must not disturb the directory: %v", err)
	}
}
