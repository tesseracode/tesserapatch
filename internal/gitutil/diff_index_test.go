package gitutil

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDiffFromCommitForPathsPreservesRacyIndexTimestamp(t *testing.T) {
	for _, redirected := range []bool{false, true} {
		name := "real-index"
		if redirected {
			name = "redirected-index"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			gitInit(t, dir)
			t.Setenv("GIT_OPTIONAL_LOCKS", "0")
			runGitTest(t, dir, "config", "core.trustctime", "false")
			runGitTest(t, dir, "config", "core.checkStat", "minimal")
			realIndex := filepath.Join(dir, ".git", "index")
			index := realIndex
			if redirected {
				index = filepath.Join(t.TempDir(), "alternate index")
				data, err := os.ReadFile(realIndex)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(index, data, 0o600); err != nil {
					t.Fatal(err)
				}
				t.Setenv("GIT_INDEX_FILE", index)
			}

			path := filepath.Join(dir, "racy.txt")
			stamp := time.Unix(946684800, 0)
			write := func(content string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(path, stamp, stamp); err != nil {
					t.Fatal(err)
				}
			}
			write("old\n")
			runGitTest(t, dir, "add", "racy.txt")
			runGitTest(t, dir, "commit", "-q", "-m", "racy baseline")
			// Equal timestamps require Git's content check despite matching stat data.
			if err := os.Chtimes(index, stamp, stamp); err != nil {
				t.Fatal(err)
			}
			write("new\n")
			before, err := os.ReadFile(index)
			if err != nil {
				t.Fatal(err)
			}
			realBefore, err := os.ReadFile(realIndex)
			if err != nil {
				t.Fatal(err)
			}
			want := strings.TrimSpace(runGitTest(t, dir, "diff", "HEAD", "--", "racy.txt"))
			if !strings.Contains(want, "-old\n+new") {
				t.Fatalf("fixture did not expose a real same-stat content change:\n%s", want)
			}

			diff, err := DiffFromCommitForPaths(dir, "HEAD", []string{"racy.txt"})
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(diff) != want {
				t.Fatalf("copied index hid a same-stat content change:\ngot:\n%s\nwant:\n%s", diff, want)
			}
			after, err := os.ReadFile(index)
			if err != nil || !bytes.Equal(after, before) {
				t.Fatalf("effective index changed: %v", err)
			}
			realAfter, err := os.ReadFile(realIndex)
			if err != nil || !bytes.Equal(realAfter, realBefore) {
				t.Fatalf("real index changed: %v", err)
			}
			info, err := os.Stat(index)
			if err != nil {
				t.Fatal(err)
			}
			if !info.ModTime().Equal(stamp) {
				t.Fatalf("effective index timestamp changed: %v", info.ModTime())
			}
		})
	}
}
