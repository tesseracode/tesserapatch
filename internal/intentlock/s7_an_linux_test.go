//go:build linux && !android

package intentlock

import "testing"

func TestS7PIB416LinuxRootFilesystemClassificationTable(t *testing.T) {
	attempts := s7InstallLockAttemptCounter(t)
	denied := []struct {
		magic uint32
		class string
	}{
		{magic: 0x00006969, class: "nfs"},
		{magic: 0x0000517B, class: "smb"},
		{magic: 0xFF534D42, class: "cifs"},
		{magic: 0xFE534D42, class: "smb2"},
		{magic: 0x65735546, class: "fuse"},
	}
	for _, fixture := range denied {
		if class, blocked := classifyLinuxFilesystem(fixture.magic); !blocked || class != fixture.class {
			t.Fatalf("magic %#x = (%q,%t), want (%q,true)", fixture.magic, class, blocked, fixture.class)
		}
		s7AssertDeniedClassStopsBeforeFlock(t, fixture.class, attempts)
	}
	for _, magic := range []uint32{0x794C7630, 0xDEADBEEF} {
		if class, blocked := classifyLinuxFilesystem(magic); blocked {
			t.Fatalf("overlay/unknown magic %#x denied as %q", magic, class)
		}
		class, _ := classifyLinuxFilesystem(magic)
		s7AssertNonDeniedClassStillRequiresRealFlock(t, class, attempts)
	}
}

func TestS7PIB441LinuxRootFilesystemPolicyFixtures(t *testing.T) {
	attempts := s7InstallLockAttemptCounter(t)
	for magic, want := range map[uint32]string{
		0x00006969: "nfs",
		0x0000517B: "smb",
		0xFF534D42: "cifs",
		0xFE534D42: "smb2",
		0x65735546: "fuse",
	} {
		if class, denied := classifyLinuxFilesystem(magic); !denied || class != want {
			t.Fatalf("magic %#x = (%q,%t), want (%q,true)", magic, class, denied, want)
		}
	}
	for _, magic := range []uint32{0x794C7630, 0xDEADBEEF} {
		class, denied := classifyLinuxFilesystem(magic)
		if denied {
			t.Fatalf("overlay/unknown %#x denied as %q", magic, class)
		}
		s7AssertNonDeniedClassStillRequiresRealFlock(t, class, attempts)
	}
}
