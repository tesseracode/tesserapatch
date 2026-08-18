//go:build android || ios || (!linux && !darwin)

package intentlock

import "testing"

func TestUnsupportedTargetFailsClosed(t *testing.T) {
	if AuthoritySupported {
		t.Fatal("unsupported build reports authority support")
	}
	authority, err := Acquire("path-that-must-not-be-opened")
	if authority != nil {
		t.Fatal("unsupported build returned an authority")
	}
	assertUnsupportedCode(t, err)
}

func assertUnsupportedCode(t *testing.T, err error) {
	t.Helper()
	typed, ok := err.(*Error)
	if !ok {
		t.Fatalf("error = %T, want *Error", err)
	}
	if typed.Code != CodePrepareUnsupportedPlatform {
		t.Fatalf("code = %q, want %q", typed.Code, CodePrepareUnsupportedPlatform)
	}
}
