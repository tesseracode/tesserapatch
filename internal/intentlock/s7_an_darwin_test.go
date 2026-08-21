//go:build darwin && !ios

package intentlock

import "testing"

func TestS7PIB416DarwinRootFilesystemClassificationTable(t *testing.T) {
	attempts := s7InstallLockAttemptCounter(t)
	for _, want := range []string{"nfs", "smbfs", "webdav", "macfuse", "osxfuse"} {
		if class, blocked := classifyDarwinFilesystem(want); !blocked || class != want {
			t.Fatalf("%q = (%q,%t), want exact denial", want, class, blocked)
		}
		s7AssertDeniedClassStopsBeforeFlock(t, want, attempts)
	}
	for _, name := range []string{"overlay", "apfs", ""} {
		if class, blocked := classifyDarwinFilesystem(name); blocked {
			t.Fatalf("local/unknown class %q denied as %q", name, class)
		}
		s7AssertNonDeniedClassStillRequiresRealFlock(t, name, attempts)
	}
}

func TestS7PIB441DarwinRootFilesystemPolicyFixtures(t *testing.T) {
	attempts := s7InstallLockAttemptCounter(t)
	for _, name := range []string{"nfs", "smbfs", "webdav", "macfuse", "osxfuse"} {
		if class, denied := classifyDarwinFilesystem(name); !denied || class != name {
			t.Fatalf("%q = (%q,%t), want exact denial", name, class, denied)
		}
	}
	for _, name := range []string{"overlay", "apfs", ""} {
		class, denied := classifyDarwinFilesystem(name)
		if denied {
			t.Fatalf("local/unknown %q denied as %q", name, class)
		}
		s7AssertNonDeniedClassStillRequiresRealFlock(t, class, attempts)
	}
}
