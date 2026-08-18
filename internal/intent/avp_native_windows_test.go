package intent

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// mustCreateJunction creates a real NTFS junction with `cmd /c mklink /J`.
//
// AVP-199: this helper calls t.Fatal — never t.Skip — when the command is
// unavailable or returns non-zero. A capability skip would silently turn the
// native Windows rows into unrun rows, which is the exact defect the row
// exists to prevent. The only permitted guard anywhere in this file is
// `runtime.GOOS != "windows"`, which is a platform guard, not a capability
// guard.
func mustCreateJunction(t *testing.T, link, target string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Fatalf("mustCreateJunction called on %s", runtime.GOOS)
	}
	output, err := exec.Command("cmd", "/c", "mklink", "/J",
		filepath.Clean(link), filepath.Clean(target)).CombinedOutput()
	if err != nil {
		t.Fatalf("mklink /J %s %s failed: %v\n%s", link, target, err, output)
	}
	info, statErr := os.Lstat(link)
	if statErr != nil {
		t.Fatalf("lstat junction: %v", statErr)
	}
	if info.Mode()&os.ModeIrregular == 0 {
		t.Fatalf("mklink /J produced mode %v, want ModeIrregular", info.Mode())
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("a junction reported ModeSymlink (%v); the winsymlink=1 mapping regressed", info.Mode())
	}
}

func windowsFixture(t *testing.T) (string, *os.Root) {
	t.Helper()
	dir := t.TempDir()
	feature := filepath.Join(dir, ".tpatch", "features", testSlug)
	if err := os.MkdirAll(filepath.Join(feature, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"status.json":             `{"state":"defined"}`,
		"analysis.md":             "analysis\n",
		"spec.md":                 "spec\n",
		"exploration.md":          "exploration\n",
		"artifacts/analysis.json": `{"summary":"x"}`,
	} {
		path := filepath.Join(feature, filepath.FromSlash(name))
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return dir, root
}

func windowsInspect(t *testing.T, root *os.Root) Report {
	t.Helper()
	return Inspect(NewRootOps(root), testSlug, make([]byte, MaxArtifactBytes+1))
}

// TestAVPNativeWindows is AVP-176 and AVP-199: the native-runner rows.
//
// The identity half rests on version-pinned unexported behavior (claims-audit
// G7, G8, G16) — Windows file identity is derived from a handle, not from a
// documented contract — so this is a Go-upgrade tripwire rather than a public
// contract test (PRD §7.4.4).
func TestAVPNativeWindows(t *testing.T) {
	// This file is name-constrained to GOOS=windows, so there is no platform
	// skip here at all: on a Windows runner every subtest below executes, and
	// on any other target the file is not compiled. AVP-199's source half
	// asserts the file contains no t.Skip of any kind.

	t.Run("AVP-176", func(t *testing.T) {
		t.Run("symlink-spec-is-symlink-refused", func(t *testing.T) {
			dir, root := windowsFixture(t)
			spec := filepath.Join(dir, ".tpatch", "features", testSlug, "spec.md")
			target := filepath.Join(dir, "target.md")
			if err := os.WriteFile(target, []byte("body\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(spec); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, spec); err != nil {
				t.Fatalf("symlink creation failed on the native runner: %v", err)
			}
			report := windowsInspect(t, root)
			if got := artifactByID(t, report, "spec").State; got != StateSymlinkRefused {
				t.Fatalf("spec = %q, want symlink-refused", got)
			}
		})

		t.Run("junction-artifacts-is-symlink-refused", func(t *testing.T) {
			dir, root := windowsFixture(t)
			artifacts := filepath.Join(dir, ".tpatch", "features", testSlug, "artifacts")
			target := filepath.Join(dir, "junction-target")
			if err := os.MkdirAll(target, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(artifacts); err != nil {
				t.Fatal(err)
			}
			mustCreateJunction(t, artifacts, target)
			report := windowsInspect(t, root)
			sidecar := artifactByID(t, report, "analysis_sidecar")
			if sidecar.State != StateSymlinkRefused {
				t.Fatalf("sidecar = %q, want symlink-refused via ModeIrregular", sidecar.State)
			}
		})

		t.Run("status-reparse-point-aborts", func(t *testing.T) {
			dir, root := windowsFixture(t)
			feature := filepath.Join(dir, ".tpatch", "features", testSlug)
			status := filepath.Join(feature, "status.json")
			target := filepath.Join(dir, "status-target.json")
			if err := os.WriteFile(target, []byte(`{"state":"defined"}`), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(status); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, status); err != nil {
				t.Fatalf("symlink creation failed on the native runner: %v", err)
			}
			report := windowsInspect(t, root)
			if report.AbortCode() != AbortStatusSymlink {
				t.Fatalf("abort = %q, want status-symlink-refused", report.AbortCode())
			}
		})

		t.Run("samefile-identity-over-root-lstat-and-file-stat", func(t *testing.T) {
			dir, root := windowsFixture(t)
			name := ".tpatch/features/" + testSlug + "/spec.md"
			pre, err := root.Lstat(name)
			if err != nil {
				t.Fatal(err)
			}
			file, err := root.OpenFile(name, os.O_RDONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			post, err := file.Stat()
			if err != nil {
				_ = file.Close()
				t.Fatal(err)
			}
			if !os.SameFile(pre, post) {
				_ = file.Close()
				t.Fatal("os.SameFile is false for an unchanged file on this runner")
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			replacement := filepath.Join(dir, ".tpatch", "features", testSlug, "spec.md")
			if err := os.Remove(replacement); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(replacement, []byte("replaced\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			replaced, err := root.Lstat(name)
			if err != nil {
				t.Fatal(err)
			}
			if os.SameFile(pre, replaced) {
				t.Fatal("os.SameFile is true across a replacement: identity detection regressed")
			}
		})

		t.Run("char-device-handle-is-not-regular", func(t *testing.T) {
			// The production gate is two ordered predicates in `capture`:
			// `refused(pre)` (symlink/reparse) and then
			// `!pre.Mode().IsRegular()` (not-regular). A character device is
			// refused by the *second* one, so the assertions below pin that
			// branch: NUL must not look like a reparse point, and it must not
			// be regular.
			info, err := os.Lstat("NUL")
			if err != nil {
				t.Fatalf("stat NUL: %v", err)
			}
			if refused(info) {
				t.Fatalf("a FILE_TYPE_CHAR handle matched the reparse predicate (mode %v); it must reach the not-regular gate instead", info.Mode())
			}
			if info.Mode().IsRegular() {
				t.Fatalf("a FILE_TYPE_CHAR handle reported regular (mode %v)", info.Mode())
			}
			if info.Mode()&os.ModeCharDevice == 0 {
				t.Fatalf("NUL is not reported as a character device on this runner (mode %v)", info.Mode())
			}
		})
	})

	t.Run("AVP-199", func(t *testing.T) {
		t.Run("junction-helper-fails-never-skips", func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "real-target")
			if err := os.MkdirAll(target, 0o755); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(dir, "link")
			mustCreateJunction(t, link, target)
			info, err := os.Lstat(link)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode()&os.ModeIrregular == 0 {
				t.Fatalf("the created object is not a real junction: mode %v", info.Mode())
			}
			if info.Mode()&os.ModeSymlink != 0 {
				t.Fatal("the junction carries ModeSymlink")
			}
		})

	})
}
