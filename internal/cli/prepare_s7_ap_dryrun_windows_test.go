//go:build windows

package cli

import (
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
)

func TestS7APDryRunWindowsEvaluatedPlatform(t *testing.T) {
	t.Run("PIB-461", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S7 AP Windows dry-run")
		before := snapshotTreeMetadata(t, "workspace", root)
		oldAcquire := prepareAcquireAuthority
		acquires := 0
		prepareAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
			acquires++
			return oldAcquire(path)
		}
		t.Cleanup(func() { prepareAcquireAuthority = oldAcquire })

		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--dry-run", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 3 || stderr != "" || report.Refusal == nil ||
			report.Refusal.Code != "workspace-unsupported-platform" ||
			!report.DryRun || report.ExecutionPreflight != "not_evaluated" ||
			acquires != 0 {
			t.Fatalf(
				"Windows evaluated platform refusal = exit:%d stderr:%q authority:%d report:%+v",
				code, stderr, acquires, report,
			)
		}
		if after := snapshotTreeMetadata(t, "workspace", root); after != before {
			t.Fatalf("Windows dry-run platform refusal mutated workspace\nbefore:\n%s\nafter:\n%s",
				before, after)
		}
	})
}
