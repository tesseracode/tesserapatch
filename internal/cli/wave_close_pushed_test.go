package cli

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// HK-D2: wave-close-check [3/8] passes when HEAD is contained in
// origin/main, so a gate validating a pushed commit survives a later push to
// main. HEAD ahead of or diverged from origin/main, an ancestry query that
// cannot answer, and an unreachable origin all fail. The Makefile's [3/8]
// recipe text is executed as written against a fixture repository whose
// origin is advanced by a second clone, so origin/main is stale until the
// recipe fetches.
func TestWaveCloseHeadPushedAncestry(t *testing.T) {
	gh9SkipWithoutPOSIXGate(t)
	makefile := gh9ReadFile(t, filepath.Join(avpRepoRoot(t), "Makefile"))
	if err := wcPushedCheck(t, makefile); err != nil {
		t.Fatal(err)
	}
	const ancestry = "git merge-base --is-ancestor HEAD origin/main"
	for _, mutation := range []struct{ name, old, new string }{
		{"equality-restored", ancestry, `[ "$$local_head" = "$$remote_head" ]`},
		{"ancestry-reversed", ancestry, "git merge-base --is-ancestor origin/main HEAD"},
		{"always-contained", ancestry, "true"},
		{"fetch-skipped", "if ! git fetch --quiet origin main 2>/dev/null; then", "if false; then"},
		{"only-no-means-unpushed", `elif [ "$$pushed_status" -eq 0 ]; then`, `elif [ "$$pushed_status" -ne 1 ]; then`},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			mutated := strings.Replace(makefile, mutation.old, mutation.new, 1)
			if mutated == makefile {
				t.Fatal("mutation changed nothing")
			}
			if err := wcPushedCheck(t, mutated); err == nil {
				t.Fatal("mutated [3/8] recipe was accepted")
			}
		})
	}
}

func wcPushedCheck(t *testing.T, makefile string) error {
	t.Helper()
	snippet, err := waveCloseRecipeBlock(makefile, "[3/8]", "[4/8]")
	if err != nil {
		return err
	}
	for _, c := range []struct {
		name  string
		setup func(t *testing.T, repo, origin string) string
		flags string
		line  string
	}{
		{"equal", nil, "fail=0", "  OK ("},
		{"behind-advanced-main", func(t *testing.T, repo, origin string) string {
			wcAdvanceOrigin(t, origin, "b.txt")
			return repo
		}, "fail=0", "  NOTE: origin/main has advanced; this gate validates "},
		{"ahead-unpushed", func(t *testing.T, repo, origin string) string {
			gh9Commit(t, repo, "a.txt", "a\n")
			return repo
		}, "fail=1", "  FAIL: HEAD ("},
		{"diverged", func(t *testing.T, repo, origin string) string {
			gh9Commit(t, repo, "a.txt", "a\n")
			wcAdvanceOrigin(t, origin, "b.txt")
			return repo
		}, "fail=1", "  FAIL: HEAD ("},
		{"undetermined-unborn-head", func(t *testing.T, repo, origin string) string {
			unborn := t.TempDir()
			gh9Git(t, unborn, "init", "-q")
			gh9Git(t, unborn, "remote", "add", "origin", origin)
			return unborn
		}, "fail=1", "  FAIL: cannot tell whether HEAD is in origin/main (git merge-base exited 128)"},
		{"fetch-fails", func(t *testing.T, repo, origin string) string {
			gh9Git(t, repo, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
			return repo
		}, "fail=1", "  FAIL: could not fetch origin/main"},
	} {
		origin := t.TempDir()
		gh9Git(t, origin, "init", "-q", "--bare")
		repo := gh9NewRepo(t)
		gh9Git(t, repo, "remote", "add", "origin", origin)
		gh9Git(t, repo, "push", "-q", "origin", "HEAD:refs/heads/main")
		dir := repo
		if c.setup != nil {
			dir = c.setup(t, repo, origin)
		}
		cmd := exec.Command("sh", "-c", "fail=0; warn=0;\n"+snippet+"\necho \"fail=$fail warn=$warn\"")
		cmd.Dir = dir
		cmd.Env = gh9GitEnv(t)
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("%s: recipe snippet failed: %v\n%s", c.name, err, output)
		}
		lines := strings.Split(strings.TrimRight(string(output), "\n"), "\n")
		if got, want := lines[len(lines)-1], c.flags+" warn=0"; got != want {
			return fmt.Errorf("%s: gate flags = %q, want %q\n%s", c.name, got, want, output)
		}
		found := false
		for _, line := range lines {
			found = found || strings.HasPrefix(line, c.line)
		}
		if !found {
			return fmt.Errorf("%s: no line starting %q\n%s", c.name, c.line, output)
		}
		if c.name == "equal" && strings.Contains(string(output), "NOTE:") {
			return fmt.Errorf("%s: unexpected advanced-main note\n%s", c.name, output)
		}
	}
	return nil
}

// wcAdvanceOrigin pushes a new commit to origin's main from a separate
// clone, leaving the fixture repository's origin/main stale until it fetches.
func wcAdvanceOrigin(t *testing.T, origin, rel string) {
	t.Helper()
	other := filepath.Join(t.TempDir(), "other")
	gh9Git(t, filepath.Dir(other), "clone", "-q", "-b", "main", origin, other)
	gh9Commit(t, other, rel, rel+"\n")
	gh9Git(t, other, "push", "-q", "origin", "HEAD:refs/heads/main")
}

// waveCloseRecipeBlock returns the wave-close-check recipe text from the
// check banner up to the next check's banner, with make's escaping removed.
func waveCloseRecipeBlock(makefile, check, next string) (string, error) {
	start := strings.Index(makefile, "\nwave-close-check:\n")
	if start < 0 {
		return "", errors.New("wave-close-check target not found")
	}
	recipe := makefile[start:]
	begin := strings.Index(recipe, "\techo \""+check+" ")
	end := strings.Index(recipe, "\techo \""+next+" ")
	if begin < 0 || end < begin {
		return "", fmt.Errorf("%s recipe block not found", check)
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimRight(recipe[begin:end], "\n"), "\n") {
		line = strings.TrimLeft(line, "\t")
		if !strings.HasSuffix(line, " \\") {
			return "", fmt.Errorf("%s recipe line lacks continuation: %q", check, line)
		}
		lines = append(lines, strings.ReplaceAll(strings.TrimSuffix(line, " \\"), "$$", "$"))
	}
	return strings.Join(lines, "\n"), nil
}
