//go:build android || ios || (!linux && !darwin)

package intentlock

import (
	"os"
	"testing"
)

func TestAcquireWithFilesystemClassifierUnsupportedDoesNotInvokeCallback(t *testing.T) {
	called := false
	authority, err := AcquireWithFilesystemClassifier(
		"path-that-must-not-be-opened",
		func(*os.File) (string, bool, error) {
			called = true
			return "nfs", true, nil
		},
	)
	if authority != nil {
		t.Fatal("unsupported target returned authority")
	}
	if called {
		t.Fatal("unsupported target invoked filesystem classifier")
	}
	assertUnsupportedCode(t, err)
}
