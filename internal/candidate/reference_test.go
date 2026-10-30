//go:build !windows

package candidate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestS2ReferenceReconstructionUsesHardenedOfflineEnvelope(t *testing.T) {
	root, head := initCandidateRepo(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shim := t.TempDir()
	log := filepath.Join(shim, "env.log")
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\037%%s\037%%s\037%%s\037%%s\037%%s\037%%s\n' \
  "${GIT_NO_LAZY_FETCH-}" "${GIT_NO_REPLACE_OBJECTS-}" "${LC_ALL-}" \
  "${GIT_DIR-}" "${GIT_CONFIG_PARAMETERS-}" "${GIT_CONFIG_GLOBAL-}" "$*" >> %q
exec %q "$@"
`, log, realGit)
	if err := os.WriteFile(filepath.Join(shim, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GIT_DIR", filepath.Join(shim, "not-a-repository"))
	t.Setenv("GIT_NO_LAZY_FETCH", "0")
	t.Setenv("GIT_NO_REPLACE_OBJECTS", "0")
	t.Setenv("GIT_CONFIG_PARAMETERS", "'core.worktree'='/untrusted-root'")
	ref, err := OpenReferenceRoot(root, head)
	if err != nil {
		t.Fatal(err)
	}
	defer ref.Close()
	if err := ref.CheckTree(); err != nil {
		t.Fatal(err)
	}
	if err := ref.CheckPaths([]string{"tracked.txt", "absent.txt"}); err != nil {
		t.Fatal(err)
	}
	patch := "diff --git a/tracked.txt b/tracked.txt\nindex 1111111..2222222 100644\n--- a/tracked.txt\n+++ b/tracked.txt\n@@ -1 +1 @@\n-v1\n+v2\n"
	rows, err := ref.ReconstructPatch(patch)
	if err != nil || len(rows) != 1 || string(rows[0].Pre.Bytes) != "v1\n" || string(rows[0].Post.Bytes) != "v2\n" {
		t.Fatalf("offline reconstruction: %+v %v", rows, err)
	}
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	validate := func(fields []string) error {
		if len(fields) != 7 || fields[0] != "1" || fields[1] != "1" || fields[2] != "C" ||
			fields[3] != "" || fields[4] != "" || fields[5] != os.DevNull {
			return fmt.Errorf("unsafe reconstruction environment")
		}
		return nil
	}
	batchSeen := false
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		fields := strings.Split(line, "\x1f")
		if err := validate(fields); err != nil {
			t.Fatalf("%v: %q", err, line)
		}
		batchSeen = batchSeen || fields[6] == "cat-file --batch"
		for index := 0; index < 6; index++ {
			mutant := append([]string(nil), fields...)
			mutant[index] = "unsafe"
			if err := validate(mutant); err == nil {
				t.Fatalf("envelope guard accepted field %d", index)
			}
		}
	}
	if !batchSeen {
		t.Fatal("fixture did not exercise the streaming blob reader")
	}
}
