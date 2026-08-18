package intent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// nativeWindowsFixture is the GOOS-constrained file that carries the native
// rows. It is referenced by path (not by a hard-coded suffix assertion) so the
// guards below can ask the toolchain what the constraint actually resolves to.
const nativeWindowsFixture = "internal/intent/avp_native_windows_test.go"

// TestAVPWindowsSourceGuards runs on every target so the source half of the
// native rows cannot rot between Windows CI runs.
func TestAVPWindowsSourceGuards(t *testing.T) {
	source := repoFile(t, nativeWindowsFixture)

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
		// The GOOS constraint is asserted against the toolchain, not against
		// the file's own name: `strings.HasSuffix("<literal>",
		// "_windows_test.go")` is a tautology that holds even after the file
		// is deleted, renamed on disk, or made to compile everywhere.
		base := filepath.Base(nativeWindowsFixture)
		windowsFiles := testGoFiles(t, "windows")
		if !windowsFiles[base] {
			t.Fatalf("%s is not compiled into the internal/intent test binary for GOOS=windows (files: %v)",
				base, sortedKeys(windowsFiles))
		}
		if runtime.GOOS != "windows" {
			if testGoFiles(t, runtime.GOOS)[base] {
				t.Fatalf("%s is compiled on GOOS=%s; the native rows must be Windows-only", base, runtime.GOOS)
			}
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

// testGoFiles asks `go list` which test files the toolchain compiles into the
// internal/intent test binary for a given GOOS. That is the authoritative
// answer to "is this file GOOS-constrained?": it evaluates the real filename
// and build-tag constraints instead of restating them in a string literal.
func testGoFiles(t *testing.T, goos string) map[string]bool {
	t.Helper()
	cmd := exec.Command("go", "list", "-f",
		"{{range .TestGoFiles}}{{.}}\n{{end}}{{range .XTestGoFiles}}{{.}}\n{{end}}",
		"./internal/intent")
	cmd.Dir = repoRootDir(t)
	cmd.Env = append(os.Environ(), "GOOS="+goos, "CGO_ENABLED=0")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list for GOOS=%s: %v", goos, err)
	}
	files := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			files[name] = true
		}
	}
	if len(files) == 0 {
		t.Fatalf("go list reported no test files for GOOS=%s", goos)
	}
	return files
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
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
