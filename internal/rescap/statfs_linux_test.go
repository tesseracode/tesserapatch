//go:build linux

// Linux statfs normalization tests (PRD §7.2, ADR-033 D9).
//
// The design does not require actually cross-compiling and running on
// four architectures in CI: it requires the normalization logic to be
// architecture-agnostic *by construction* and unit-tested against
// fixture values representative of each width/signedness class.

package rescap

import "testing"

// TestStatfsTypeNormalizationAcrossWidths feeds a fixture value for
// each of Linux's architecture-dependent Statfs_t.Type widths —
// int64 (amd64/arm64), int32 (386/arm) and uint32 (s390x) — and
// confirms the same allow/deny outcome regardless.
func TestStatfsTypeNormalizationAcrossWidths(t *testing.T) {
	cases := []struct {
		name    string
		got     uint32
		allowed bool
	}{
		{"ext4", magicEXT, true},
		{"xfs", magicXFS, true},
		{"btrfs", magicBTRFS, true},
		{"tmpfs", magicTMPFS, true},
		{"overlayfs", magicOVERLAY, true},
		{"nfs", magicNFS, false},
		{"cifs", magicCIFS, false},
		{"smb2", magicSMB2, false},
		{"fuse", magicFUSE, false},
		{"unrecognized", 0xDEADBEEF, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// int64 (amd64/arm64): the magic may be sign-extended.
			asInt64 := int64(int32(tc.got))
			if normalizeStatfsType(asInt64) != tc.got {
				t.Fatalf("int64 normalization = %#x, want %#x", normalizeStatfsType(asInt64), tc.got)
			}
			// int32 (386/arm).
			if normalizeStatfsType(int32(tc.got)) != tc.got {
				t.Fatalf("int32 normalization = %#x, want %#x", normalizeStatfsType(int32(tc.got)), tc.got)
			}
			// uint32 (s390x).
			if normalizeStatfsType(tc.got) != tc.got {
				t.Fatalf("uint32 normalization = %#x, want %#x", normalizeStatfsType(tc.got), tc.got)
			}
			// The wide, unsign-extended form.
			if normalizeStatfsType(uint64(tc.got)) != tc.got {
				t.Fatalf("uint64 normalization = %#x, want %#x", normalizeStatfsType(uint64(tc.got)), tc.got)
			}
			if _, allowed := ClassifyFilesystemMagic(tc.got); allowed != tc.allowed {
				t.Fatalf("allowed = %v, want %v", allowed, tc.allowed)
			}
		})
	}
}

// TestNoexecFlagBit pins Linux's ST_NOEXEC value.
func TestNoexecFlagBit(t *testing.T) {
	if !NoexecFlagSet(0x8) {
		t.Fatal("ST_NOEXEC is bit 0x8")
	}
	if NoexecFlagSet(0x4) {
		t.Fatal("0x4 is not ST_NOEXEC on linux")
	}
}
