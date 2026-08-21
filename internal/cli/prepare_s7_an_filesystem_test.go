//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
)

func TestS7PIB416DeniedFilesystemClassesReachPublicCLIReports(t *testing.T) {
	originalAcquire := prepareAcquireAuthority
	t.Cleanup(func() { prepareAcquireAuthority = originalAcquire })
	classes := []struct {
		platform string
		class    string
	}{
		{platform: "linux", class: "nfs"},
		{platform: "linux", class: "smb"},
		{platform: "linux", class: "cifs"},
		{platform: "linux", class: "smb2"},
		{platform: "linux", class: "fuse"},
		{platform: "darwin", class: "nfs"},
		{platform: "darwin", class: "smbfs"},
		{platform: "darwin", class: "webdav"},
		{platform: "darwin", class: "macfuse"},
		{platform: "darwin", class: "osxfuse"},
	}
	for _, fixture := range classes {
		for _, asJSON := range []bool{true, false} {
			root, slug := s7PlatformWorkspace(t)
			lane := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug)
			if err := os.MkdirAll(lane, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(lane, "journal.json"), []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			before := readTree(t, filepath.Join(root, ".tpatch"))
			classifierCalls := 0
			prepareAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
				return intentlock.AcquireWithFilesystemClassifier(
					path,
					func(*os.File) (string, bool, error) {
						classifierCalls++
						return fixture.class, true, nil
					},
				)
			}
			args := []string{"--path", root, "prepare", slug}
			if asJSON {
				args = append(args, "--json", "--quiet")
			}
			code, stdout, stderr, _ := runPrepare(t, args...)
			prepareAcquireAuthority = originalAcquire

			if code != 3 || classifierCalls != 1 {
				t.Fatalf("%s/%s json=%t code=%d classifier-calls=%d stderr=%q",
					fixture.platform, fixture.class, asJSON, code, classifierCalls, stderr)
			}
			if asJSON {
				report := prepareS4Report(t, stdout)
				if report.Refusal == nil ||
					report.Refusal.Code != string(intentlock.CodeLockFilesystemUnsupported) ||
					!strings.Contains(report.Refusal.Message, "("+fixture.class+")") ||
					!strings.Contains(report.Refusal.Remediation, ".tpatch/local/intent-prepare/"+slug+"/") {
					t.Fatalf("%s/%s JSON refusal = %#v", fixture.platform, fixture.class, report.Refusal)
				}
			} else {
				rendered := stdout + stderr
				for _, want := range []string{
					string(intentlock.CodeLockFilesystemUnsupported),
					"(" + fixture.class + ")",
					"rm -rf .tpatch/local/intent-prepare/" + slug + "/",
				} {
					if !strings.Contains(rendered, want) {
						t.Fatalf("%s/%s human refusal missing %q:\n%s", fixture.platform, fixture.class, want, rendered)
					}
				}
			}
			if strings.Contains(stdout+stderr, root) {
				t.Fatalf("%s/%s report leaked absolute workspace path", fixture.platform, fixture.class)
			}
			after := readTree(t, filepath.Join(root, ".tpatch"))
			if !bytes.Equal(before, after) {
				t.Fatalf("%s/%s json=%t refusal mutated .tpatch", fixture.platform, fixture.class, asJSON)
			}
		}
	}
}
