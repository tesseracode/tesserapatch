//go:build linux && !android

package intentlock

import "testing"

func TestLinuxDeniedFilesystemMagicIsExact(t *testing.T) {
	denied := map[uint32]string{
		0x00006969: "nfs",
		0x0000517B: "smb",
		0xFF534D42: "cifs",
		0xFE534D42: "smb2",
		0x65735546: "fuse",
	}
	for magic, wantName := range denied {
		name, gotDenied := classifyLinuxFilesystem(magic)
		if !gotDenied || name != wantName {
			t.Fatalf("magic %#x = (%q, %v), want (%q, true)", magic, name, gotDenied, wantName)
		}
		for _, nearby := range []uint32{magic - 1, magic + 1} {
			if _, fuzzyDenied := classifyLinuxFilesystem(nearby); fuzzyDenied {
				t.Fatalf("nearby magic %#x was fuzzily denied", nearby)
			}
		}
	}

	for _, allowedByClass := range []uint32{
		0x794C7630, // overlayfs
		0x2011BAB0, // exFAT
		0x2FC12FC1, // ZFS
		0xF2F52010, // f2fs
		0xDEADBEEF, // unknown
	} {
		if _, denied := classifyLinuxFilesystem(allowedByClass); denied {
			t.Fatalf("non-denied magic %#x was rejected", allowedByClass)
		}
	}
}

func TestLinuxStatfsTypeNormalizationAcrossWidths(t *testing.T) {
	const want uint32 = 0xFF534D42
	if got := normalizeLinuxStatfsType(int32(-11317950)); got != want {
		t.Fatalf("int32 normalization = %#x, want %#x", got, want)
	}
	if got := normalizeLinuxStatfsType(int64(want)); got != want {
		t.Fatalf("int64 normalization = %#x, want %#x", got, want)
	}
	if got := normalizeLinuxStatfsType(uint32(want)); got != want {
		t.Fatalf("uint32 normalization = %#x, want %#x", got, want)
	}
	if got := normalizeLinuxStatfsType(uint64(want)); got != want {
		t.Fatalf("uint64 normalization = %#x, want %#x", got, want)
	}
}
