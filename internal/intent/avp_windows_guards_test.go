package intent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestAVPWindowsSourceGuards runs on every target so the source half of the
// native rows cannot rot between Windows CI runs.
func TestAVPWindowsSourceGuards(t *testing.T) {
	source := repoFile(t, "internal/intent/avp_native_windows_test.go")

	t.Run("AVP-199-source-half", func(t *testing.T) {
		if !strings.Contains(source, `exec.Command("cmd", "/c", "mklink", "/J",`) {
			t.Fatal("the junction helper no longer shells out to mklink /J")
		}
		if !strings.Contains(source, "t.Fatalf(\"mklink /J %s %s failed") {
			t.Fatal("the junction helper no longer fails hard when mklink is unavailable")
		}
		// The file is name-constrained to GOOS=windows, so it needs no
		// platform guard at all and must contain no skip of any kind: on a
		// windows-latest runner every assertion below executes.
		for _, forbidden := range []string{"t.Skip(", "t.Skipf(", "t.SkipNow("} {
			if occurrences := strings.Count(source, forbidden); occurrences != 0 {
				t.Fatalf("%s appears %d times in the junction fixture path; capability skips are forbidden", forbidden, occurrences)
			}
		}
		if !strings.HasSuffix("internal/intent/avp_native_windows_test.go", "_windows_test.go") {
			t.Fatal("the native rows are no longer in a GOOS-constrained file")
		}
	})

	t.Run("AVP-175-ci-half", func(t *testing.T) {
		workflow := repoFile(t, ".github/workflows/ci.yml")
		if err := checkCIMatrix(workflow); err != nil {
			t.Fatal(err)
		}
		// The Windows row must actually run the test suite, not just build.
		if !strings.Contains(workflow, "go test") {
			t.Fatal("the CI job does not run go test")
		}
	})

	t.Run("AVP-198-source-half", func(t *testing.T) {
		if err := checkWinsymlinkDirective(repoFile(t, "cmd/tpatch/main.go")); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("AVP-178-cross-build-half", func(t *testing.T) {
		if testing.Short() {
			t.Skip("cross-build guard is covered by TestAVPGuards/AVP-178")
		}
		if err := crossBuildWindows(t); err != nil {
			t.Fatal(err)
		}
	})
}

func crossBuildWindows(t *testing.T) error {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", filepath.Join(t.TempDir(), "tpatch.exe"), "./cmd/tpatch")
	cmd.Dir = repoRootDir(t)
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("windows cross-build failed: %v\n%s", err, output)
	}
	return nil
}
