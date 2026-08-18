//go:build darwin && !ios

package intentlock

import "testing"

func TestDarwinDeniedFilesystemNamesAreExact(t *testing.T) {
	for _, name := range []string{"nfs", "smbfs", "webdav", "macfuse", "osxfuse"} {
		got, denied := classifyDarwinFilesystem(name)
		if !denied || got != name {
			t.Fatalf("%q = (%q, %v), want exact denial", name, got, denied)
		}
	}
	for _, name := range []string{
		"sshfs", "gocryptfs", "fuse", "nfs4", "smbfs-extra",
		"prefix-webdav", "overlay", "apfs", "zfs", "",
	} {
		if _, denied := classifyDarwinFilesystem(name); denied {
			t.Fatalf("%q was fuzzily denied", name)
		}
	}
}

func TestDarwinFilesystemNameTrimsInt8AtFirstNUL(t *testing.T) {
	raw := []int8{'m', 'a', 'c', 'f', 'u', 's', 'e', 0, 'x'}
	if got := darwinFilesystemName(raw); got != "macfuse" {
		t.Fatalf("darwinFilesystemName = %q, want macfuse", got)
	}
	raw = []int8{-61, -87, 0}
	if got := []byte(darwinFilesystemName(raw)); len(got) != 2 || got[0] != 0xC3 || got[1] != 0xA9 {
		t.Fatalf("signed-byte conversion = %v, want [195 169]", got)
	}
}
