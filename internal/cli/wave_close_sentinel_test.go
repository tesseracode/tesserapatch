package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// GH #9: the wave-close [2/8] forgotten-`git add` sentinel must see source
// inside untracked nested Git repositories, which Git reports only as an
// opaque `dir/` entry.

const gh9SentinelScript = "scripts/wave-close-untracked-sentinel.sh"

var gh9SentinelPathspecs = []string{
	"*.go", "internal/**", "cmd/**", "assets/**", "scripts/**", "docs/*.md",
	"docs/adrs/*.md", "docs/prds/*.md", "docs/milestones/*.md",
	"docs/whitepapers/*.md", "docs/state-of-the-art/**", "Makefile", "go.mod",
	"go.sum", "AGENTS.md", "SPEC.md", "CLAUDE.md", ".wave-close-allowlist",
}

type gh9SentinelCase struct {
	name     string
	setup    func(t *testing.T, repo string)
	subdir   string
	wantExit int
	wantOut  string
}

func gh9SentinelCases() []gh9SentinelCase {
	return []gh9SentinelCase{
		{name: "clean", wantExit: 0, wantOut: "  OK\n"},
		{
			name: "top-level-source",
			setup: func(t *testing.T, repo string) {
				gh9Write(t, repo, "a.go", "package a\n")
			},
			wantExit: 3,
			wantOut:  "  WARN: 1 untracked files not in allowlist (0 allowlisted):\n    a.go\n",
		},
		{
			name:     "issue-9-nested-repository",
			setup:    gh9IssueReproduction,
			wantExit: 3,
			wantOut: "  WARN: 1 untracked files not in allowlist (0 allowlisted):\n" +
				"    scratch-review/repo/internal/example.go\n",
		},
		{
			name: "nested-allowlist-literal",
			setup: func(t *testing.T, repo string) {
				gh9IssueReproduction(t, repo)
				gh9Commit(t, repo, ".wave-close-allowlist", "# fixture\nscratch-review/repo/internal/example.go\n")
			},
			wantExit: 0,
			wantOut:  "  OK (1 entries allowlisted)\n",
		},
		{
			name: "nested-allowlist-glob",
			setup: func(t *testing.T, repo string) {
				gh9IssueReproduction(t, repo)
				gh9Commit(t, repo, ".wave-close-allowlist", "scratch-review/repo/**\n")
			},
			wantExit: 0,
			wantOut:  "  OK (1 entries allowlisted)\n",
		},
		{
			// Allowlist entries are Git pathspecs: `*.go` must match the nested
			// file too, not only what the shell would glob at the top level.
			name: "allowlist-is-pathspec-not-shell-glob",
			setup: func(t *testing.T, repo string) {
				gh9IssueReproduction(t, repo)
				gh9Write(t, repo, "a.go", "package a\n")
				gh9Commit(t, repo, ".wave-close-allowlist", "*.go\n")
			},
			wantExit: 0,
			wantOut:  "  OK (2 entries allowlisted)\n",
		},
		{
			name: "nested-path-ignored-by-parent",
			setup: func(t *testing.T, repo string) {
				gh9IssueReproduction(t, repo)
				gh9Commit(t, repo, ".gitignore", "scratch-review/repo/internal/\n")
			},
			wantExit: 0,
			wantOut:  "  OK\n",
		},
		{
			name: "nested-path-ignored-by-nested-gitignore",
			setup: func(t *testing.T, repo string) {
				gh9IssueReproduction(t, repo)
				gh9Write(t, repo, "scratch-review/repo/.gitignore", "internal/\n")
			},
			wantExit: 0,
			wantOut:  "  OK\n",
		},
		{
			name: "git-internals-never-read",
			setup: func(t *testing.T, repo string) {
				gh9NestedRepo(t, repo, "scratch-review/repo")
				gh9Write(t, repo, "scratch-review/repo/README", "notes\n")
				gh9Write(t, repo, "scratch-review/repo/.git/objects/zz/hidden.go", "package hidden\n")
				gh9Write(t, repo, "scratch-review/repo/sub/.git/hidden.go", "package hidden\n")
			},
			wantExit: 0,
			wantOut:  "  OK\n",
		},
		{
			// An unreadable directory inside a nested `.git` makes any
			// traversal of Git internals a hard find error.
			name: "git-internals-never-traversed",
			setup: func(t *testing.T, repo string) {
				gh9NestedRepo(t, repo, "scratch-review/repo")
				gh9Write(t, repo, "scratch-review/repo/README", "notes\n")
				gh9Unreadable(t, repo, "scratch-review/repo/.git/locked")
			},
			wantExit: 0,
			wantOut:  "  OK\n",
		},
		{
			name: "repository-nested-in-nested-repository",
			setup: func(t *testing.T, repo string) {
				gh9NestedRepo(t, repo, "scratch-review/repo")
				gh9NestedRepo(t, repo, "scratch-review/repo/inner")
				gh9Write(t, repo, "scratch-review/repo/inner/cmd/main.go", "package main\n")
			},
			wantExit: 3,
			wantOut: "  WARN: 1 untracked files not in allowlist (0 allowlisted):\n" +
				"    scratch-review/repo/inner/cmd/main.go\n",
		},
		{
			// Git reports a nested repository under a source pathspec as an
			// opaque `internal/repo/` record; it is replaced by its files.
			name:     "nested-repository-under-source-pathspec",
			setup:    gh9SourceNestedRepo,
			wantExit: 3,
			wantOut:  "  WARN: 1 untracked files not in allowlist (0 allowlisted):\n    internal/repo/x.go\n",
		},
		{
			name: "nested-under-source-allowlist-literal",
			setup: func(t *testing.T, repo string) {
				gh9SourceNestedRepo(t, repo)
				gh9Commit(t, repo, ".wave-close-allowlist", "internal/repo/x.go\n")
			},
			wantExit: 0,
			wantOut:  "  OK (1 entries allowlisted)\n",
		},
		{
			// Directory-style entries written for the old opaque record
			// keep covering the files inside.
			name: "nested-under-source-allowlist-directory",
			setup: func(t *testing.T, repo string) {
				gh9SourceNestedRepo(t, repo)
				gh9Commit(t, repo, ".wave-close-allowlist", "internal/repo/\n")
			},
			wantExit: 0,
			wantOut:  "  OK (1 entries allowlisted)\n",
		},
		{
			name: "nested-under-source-contents-ignored",
			setup: func(t *testing.T, repo string) {
				gh9SourceNestedRepo(t, repo)
				gh9Commit(t, repo, ".gitignore", "internal/repo/x.go\n")
			},
			wantExit: 0,
			wantOut:  "  OK\n",
		},
		{
			// Git C-quotes non-ASCII names by default: the opaque record
			// is `"internal/caf\303\251/"`.
			name: "quoted-boundary-record-replaced",
			setup: func(t *testing.T, repo string) {
				gh9NestedRepo(t, repo, "internal/café")
				gh9Write(t, repo, "internal/café/x.go", "package x\n")
			},
			wantExit: 3,
			wantOut:  "  WARN: 1 untracked files not in allowlist (0 allowlisted):\n    \"internal/caf\\303\\251/x.go\"\n",
		},
		{
			name: "not-at-top-level-fails-closed",
			setup: func(t *testing.T, repo string) {
				gh9Write(t, repo, "sub/README", "x\n")
			},
			subdir:   "sub",
			wantExit: 2,
			wantOut:  "  FAIL: not running at the top level of a Git work tree\n",
		},
		{
			name: "rejected-allowlist-pattern-fails-closed",
			setup: func(t *testing.T, repo string) {
				gh9Write(t, repo, "a.go", "package a\n")
				gh9Commit(t, repo, ".wave-close-allowlist", ":(bogus)a.go\n")
			},
			wantExit: 2,
			wantOut:  "  FAIL: git rejected a .wave-close-allowlist pattern\n",
		},
		{
			name: "quoted-nested-repository-name-fails-closed",
			setup: func(t *testing.T, repo string) {
				gh9NestedRepo(t, repo, `we"ird`)
				gh9Write(t, repo, `we"ird/x.go`, "package x\n")
			},
			wantExit: 2,
			wantOut:  "  FAIL: untracked nested repository \"we\\\"ird/\" has a name the sentinel cannot inspect\n",
		},
		{
			name: "newline-file-name-fails-closed",
			setup: func(t *testing.T, repo string) {
				gh9NestedRepo(t, repo, "nl")
				gh9Write(t, repo, "nl/a\nb.go", "package x\n")
			},
			wantExit: 2,
			wantOut:  "  FAIL: untracked nested repository nl/ contains a file name with a newline\n",
		},
	}
}

func TestGH9WaveCloseUntrackedSentinel(t *testing.T) {
	gh9SkipWithoutPOSIXGate(t)
	root := avpRepoRoot(t)
	makefile := gh9ReadFile(t, filepath.Join(root, "Makefile"))
	specs, err := gh9MakefilePathspecs(makefile)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(specs, gh9SentinelPathspecs) {
		t.Fatalf("Makefile sentinel pathspecs = %q, want %q", specs, gh9SentinelPathspecs)
	}
	script := filepath.Join(root, gh9SentinelScript)
	for _, c := range gh9SentinelCases() {
		t.Run(c.name, func(t *testing.T) {
			if err := gh9CheckCase(t, script, specs, c); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Every guarded behavior must be load-bearing: each mutation of the real
// script must make its corresponding case fail.
func TestGH9WaveCloseUntrackedSentinelMutations(t *testing.T) {
	gh9SkipWithoutPOSIXGate(t)
	root := avpRepoRoot(t)
	body := gh9ReadFile(t, filepath.Join(root, gh9SentinelScript))
	cases := map[string]gh9SentinelCase{}
	for _, c := range gh9SentinelCases() {
		cases[c.name] = c
	}
	for _, mutation := range []struct {
		name, old, new, breaks string
	}{
		{
			name:   "nested-supplement-disabled",
			old:    `done <"$work/all"`,
			new:    `done </dev/null`,
			breaks: "issue-9-nested-repository",
		},
		{
			name:   "git-dir-prune-removed",
			old:    `find "./$dir" -name .git -prune -o \( -type f -o -type l \) -print`,
			new:    `find "./$dir" \( -type f -o -type l \) -print`,
			breaks: "git-internals-never-read",
		},
		{
			name:   "git-dir-prune-removed-from-enumeration-traversal",
			old:    `find "./$dir" -name .git -prune -o \( -type f -o -type l \) -print`,
			new:    `find "./$dir" \( -type f -o -type l \) -print`,
			breaks: "git-internals-never-traversed",
		},
		{
			name:   "git-dir-prune-removed-from-newline-scan",
			old:    `find "./$dir" -name .git -prune -o -name "*$newline*" -print`,
			new:    `find "./$dir" -name "*$newline*" -print`,
			breaks: "git-internals-never-traversed",
		},
		{
			name:   "ignore-rules-not-applied",
			old:    `if [ -s "$work/ignored" ]; then`,
			new:    `if false; then`,
			breaks: "nested-path-ignored-by-parent",
		},
		{
			name:   "inspected-boundary-record-kept",
			old:    `grep -v -e '/$' -e '/"$' "$work/top" >"$work/candidates"`,
			new:    `cp "$work/top" "$work/candidates"`,
			breaks: "nested-under-source-allowlist-literal",
		},
		{
			name:   "inspected-boundary-record-kept-when-contents-ignored",
			old:    `grep -v -e '/$' -e '/"$' "$work/top" >"$work/candidates"`,
			new:    `cp "$work/top" "$work/candidates"`,
			breaks: "nested-under-source-contents-ignored",
		},
		{
			name:   "quoted-boundary-record-kept",
			old:    `grep -v -e '/$' -e '/"$' "$work/top" >"$work/candidates"`,
			new:    `grep -v -e '/$' "$work/top" >"$work/candidates"`,
			breaks: "quoted-boundary-record-replaced",
		},
		{
			name:   "allowlist-not-applied-to-nested-paths",
			old:    `scratch_git ls-files --cached -- $allow_patterns >"$work/nested-allowed"`,
			new:    `: >"$work/nested-allowed"`,
			breaks: "nested-allowlist-literal",
		},
		{
			name:   "allowlist-shell-globbing-restored",
			old:    "set -f\n",
			new:    "\n",
			breaks: "allowlist-is-pathspec-not-shell-glob",
		},
		{
			name:   "rejected-allowlist-fails-open",
			old:    `fail "git rejected a .wave-close-allowlist pattern"`,
			new:    `true`,
			breaks: "rejected-allowlist-pattern-fails-closed",
		},
		{
			name:   "quoted-boundary-skipped",
			old:    "\t\\\"*/\\\") fail \"untracked nested repository $entry has a name the sentinel cannot inspect\" ;;\n",
			new:    "",
			breaks: "quoted-nested-repository-name-fails-closed",
		},
		{
			name:   "newline-guard-removed",
			old:    `if [ -n "$odd" ]; then`,
			new:    `if false; then`,
			breaks: "newline-file-name-fails-closed",
		},
		{
			name:   "top-level-check-removed",
			old:    `if ! cdup=$(git rev-parse --show-cdup 2>/dev/null) || [ -n "$cdup" ]; then`,
			new:    `if false; then`,
			breaks: "not-at-top-level-fails-closed",
		},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			mutated := strings.Replace(body, mutation.old, mutation.new, 1)
			if mutated == body {
				t.Fatal("mutation changed nothing")
			}
			c, ok := cases[mutation.breaks]
			if !ok {
				t.Fatalf("unknown case %q", mutation.breaks)
			}
			script := filepath.Join(t.TempDir(), "sentinel.sh")
			if err := os.WriteFile(script, []byte(mutated), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := gh9CheckCase(t, script, gh9SentinelPathspecs, c); err == nil {
				t.Fatalf("mutated sentinel still satisfied %q", c.name)
			}
		})
	}
}

// The Makefile's [2/8] recipe text is executed as written: WARN must raise
// the gate's warn flag and any other non-zero status must fail the gate.
func TestGH9WaveCloseSentinelMakefileWiring(t *testing.T) {
	gh9SkipWithoutPOSIXGate(t)
	root := avpRepoRoot(t)
	makefile := gh9ReadFile(t, filepath.Join(root, "Makefile"))
	script := gh9ReadFile(t, filepath.Join(root, gh9SentinelScript))
	if err := gh9CheckMakefileWiring(t, makefile, script); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct{ name, old, new string }{
		{"warn-not-raised", "\t\twarn=1; \\\n", "\t\t:; \\\n"},
		{"failure-swallowed", `elif [ "$$sentinel_status" -ne 0 ]; then`, "elif false; then"},
		{"git-only-enumeration-restored", "sh " + gh9SentinelScript, "git ls-files --others --exclude-standard --"},
		{"source-pathspec-dropped", " '*.go' 'internal/**'", " 'internal/**'"},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			mutated := strings.Replace(makefile, mutation.old, mutation.new, 1)
			if mutated == makefile {
				t.Fatal("mutation changed nothing")
			}
			if err := gh9CheckMakefileWiring(t, mutated, script); err == nil {
				t.Fatal("mutated Makefile wiring was accepted")
			}
		})
	}
}

func gh9CheckMakefileWiring(t *testing.T, makefile, script string) error {
	t.Helper()
	snippet, err := gh9MakefileSentinelSnippet(makefile)
	if err != nil {
		return err
	}
	for _, c := range []struct {
		name  string
		setup func(t *testing.T, repo string)
		want  string
	}{
		{"clean", nil, "fail=0 warn=0"},
		{"issue-9-nested-repository", gh9IssueReproduction, "fail=0 warn=1"},
		{"rejected-allowlist", func(t *testing.T, repo string) {
			gh9Write(t, repo, "a.go", "package a\n")
			gh9Commit(t, repo, ".wave-close-allowlist", ":(bogus)a.go\n")
		}, "fail=1 warn=0"},
	} {
		repo := gh9NewRepo(t)
		gh9Commit(t, repo, gh9SentinelScript, script)
		if c.setup != nil {
			c.setup(t, repo)
		}
		cmd := exec.Command("sh", "-c", "fail=0; warn=0;\n"+snippet+"\necho \"fail=$fail warn=$warn\"")
		cmd.Dir = repo
		cmd.Env = gh9GitEnv(t)
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("%s: recipe snippet failed: %v\n%s", c.name, err, output)
		}
		lines := strings.Split(strings.TrimRight(string(output), "\n"), "\n")
		if got := lines[len(lines)-1]; got != c.want {
			return fmt.Errorf("%s: gate flags = %q, want %q\n%s", c.name, got, c.want, output)
		}
	}
	return nil
}

// gh9MakefileSentinelSnippet returns the wave-close-check recipe text from
// the [2/8] banner up to the [3/8] banner, with make's escaping removed.
func gh9MakefileSentinelSnippet(makefile string) (string, error) {
	start := strings.Index(makefile, "\nwave-close-check:\n")
	if start < 0 {
		return "", errors.New("wave-close-check target not found")
	}
	recipe := makefile[start:]
	begin := strings.Index(recipe, "\techo \"[2/8] ")
	end := strings.Index(recipe, "\techo \"[3/8] ")
	if begin < 0 || end < begin {
		return "", errors.New("[2/8] recipe block not found")
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimRight(recipe[begin:end], "\n"), "\n") {
		line = strings.TrimLeft(line, "\t")
		if !strings.HasSuffix(line, " \\") {
			return "", fmt.Errorf("[2/8] recipe line lacks continuation: %q", line)
		}
		lines = append(lines, strings.ReplaceAll(strings.TrimSuffix(line, " \\"), "$$", "$"))
	}
	return strings.Join(lines, "\n"), nil
}

func gh9MakefilePathspecs(makefile string) ([]string, error) {
	call := "\tsh " + gh9SentinelScript + " "
	if strings.Count(makefile, call) != 1 {
		return nil, fmt.Errorf("sentinel invocation count = %d, want 1", strings.Count(makefile, call))
	}
	start := strings.Index(makefile, call) + len(call)
	end := strings.Index(makefile[start:], "; \\\n")
	if end < 0 {
		return nil, errors.New("sentinel invocation is not a recipe line")
	}
	args := makefile[start : start+end]
	words := regexp.MustCompile(`'([^']*)'`).FindAllStringSubmatch(args, -1)
	var specs []string
	for _, word := range words {
		specs = append(specs, word[1])
	}
	if strings.Join(strings.Fields(args), " ") != "'"+strings.Join(specs, "' '")+"'" {
		return nil, fmt.Errorf("sentinel arguments are not plain single-quoted pathspecs: %q", args)
	}
	return specs, nil
}

func gh9CheckCase(t *testing.T, script string, specs []string, c gh9SentinelCase) error {
	t.Helper()
	repo := gh9NewRepo(t)
	if c.setup != nil {
		c.setup(t, repo)
	}
	cmd := exec.Command("sh", append([]string{script}, specs...)...)
	cmd.Dir = filepath.Join(repo, c.subdir)
	cmd.Env = gh9GitEnv(t)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	exit := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exit = exitErr.ExitCode()
	} else if err != nil {
		return fmt.Errorf("%s: run sentinel: %v", c.name, err)
	}
	if exit != c.wantExit || stdout.String() != c.wantOut {
		return fmt.Errorf("%s: exit %d output %q, want exit %d output %q (stderr %q)",
			c.name, exit, stdout.String(), c.wantExit, c.wantOut, stderr.String())
	}
	// An answered gate (OK or WARN) must be silent on stderr; Git only warns
	// there, e.g. "Ignoring path" when Git internals reach the scratch index.
	if c.wantExit != 2 && stderr.Len() != 0 {
		return fmt.Errorf("%s: unexpected stderr %q", c.name, stderr.String())
	}
	return nil
}

// gh9IssueReproduction recreates the GH #9 report and proves Git alone
// cannot see the nested file, so the fixture is meaningful.
func gh9IssueReproduction(t *testing.T, repo string) {
	t.Helper()
	gh9NestedRepo(t, repo, "scratch-review/repo")
	gh9Write(t, repo, "scratch-review/repo/internal/example.go", "package example\n")
	if got := gh9Git(t, repo, "ls-files", "--others", "--exclude-standard", "--",
		"scratch-review/repo/internal/example.go"); got != "" {
		t.Fatalf("git enumerated the nested file directly (%q); fixture no longer reproduces GH #9", got)
	}
	if got := gh9Git(t, repo, "ls-files", "--others", "--exclude-standard"); !strings.Contains(got, "scratch-review/repo/\n") {
		t.Fatalf("git did not report the opaque nested repository boundary: %q", got)
	}
}

// gh9SourceNestedRepo places an untracked nested repository inside a source
// pathspec, where Git reports it as the opaque record `internal/repo/`.
func gh9SourceNestedRepo(t *testing.T, repo string) {
	t.Helper()
	gh9NestedRepo(t, repo, "internal/repo")
	gh9Write(t, repo, "internal/repo/x.go", "package x\n")
	if got := gh9Git(t, repo, "ls-files", "--others", "--exclude-standard", "--", "internal/**"); got != "internal/repo/\n" {
		t.Fatalf("git source query = %q, want the opaque record %q", got, "internal/repo/\n")
	}
}

func gh9SkipWithoutPOSIXGate(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("wave-close-check is a POSIX make/sh gate that is not run on Windows (Windows portability: GH #17)")
	}
}

func gh9NewRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gh9Git(t, repo, "init", "-q")
	gh9Git(t, repo, "-c", "user.name=gh9", "-c", "user.email=gh9@example.invalid",
		"commit", "-q", "--allow-empty", "-m", "init")
	return repo
}

func gh9NestedRepo(t *testing.T, repo, rel string) {
	t.Helper()
	dir := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gh9Git(t, dir, "init", "-q")
}

func gh9Write(t *testing.T, repo, rel, body string) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// gh9Unreadable creates an empty directory that cannot be listed. It skips
// the test where permissions are not enforced (e.g. running as root).
func gh9Unreadable(t *testing.T, repo, rel string) {
	t.Helper()
	dir := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if f, err := os.Open(dir); err == nil {
		_, err = f.Readdirnames(1)
		_ = f.Close()
		if err == nil || errors.Is(err, io.EOF) {
			t.Skip("directory permissions are not enforced for this user")
		}
	}
}

func gh9Commit(t *testing.T, repo, rel, body string) {
	t.Helper()
	gh9Write(t, repo, rel, body)
	gh9Git(t, repo, "add", "--", rel)
	gh9Git(t, repo, "-c", "user.name=gh9", "-c", "user.email=gh9@example.invalid",
		"commit", "-q", "-m", "fixture "+rel)
}

func gh9Git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gh9GitEnv(t)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

// gh9GitEnv isolates fixture Git processes from the caller's repository,
// configuration and index.
func gh9GitEnv(t *testing.T) []string {
	t.Helper()
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			env = append(env, kv)
		}
	}
	return append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
}

func gh9ReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
