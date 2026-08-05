package workflow

import (
	"os"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/testutil"
)

// TestMain pins gc.auto=0 process-wide for every `git` subprocess this
// package's tests spawn (Cluster E F2 rev-1 fold, E-EXT-1). Without
// this, `git commit` in tests can fork a detached `git maintenance
// --auto` background writer that races `t.TempDir()` teardown under
// full-suite load, producing a transient macOS
// `unlinkat ...: directory not empty` failure.
func TestMain(m *testing.M) {
	testutil.PinGitAutoGCOff()
	os.Exit(m.Run())
}
