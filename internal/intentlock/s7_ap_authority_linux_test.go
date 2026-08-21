//go:build linux && !android

package intentlock

import "testing"

func TestS7APLinuxFilesystemMagic(t *testing.T) {
	t.Run("PIB-479", func(t *testing.T) {
		for magic, want := range map[uint32]string{
			0x6969: "nfs", 0x517B: "smb", 0xFF534D42: "cifs",
			0xFE534D42: "smb2", 0x65735546: "fuse",
		} {
			class, denied := classifyLinuxFilesystem(magic)
			if class != want || !denied {
				t.Fatalf("Linux magic %#x = (%q,%t), want (%q,true)",
					magic, class, denied, want)
			}
		}
		for _, magic := range []uint32{0x794C7630, 0, 0x6968, 0x696A, 0xDEADBEEF} {
			if class, denied := classifyLinuxFilesystem(magic); denied {
				t.Fatalf("Linux unknown/local magic %#x denied as %q", magic, class)
			}
		}
	})
}
