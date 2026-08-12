// Tests for nested linked-worktree discovery and filtering (GH #7).
//
// Two layers:
//
//  1. Pure parser / membership tests — porcelain shapes, paths with
//     spaces, prefix-boundary safety, malformed output.
//  2. Real Git fixtures — `git worktree add` under and outside the
//     target root, plus non-regression coverage for tracked gitlinks
//     and unregistered nested Git repositories.
//
// Every fixture worktree is detached with `git worktree remove --force`
// (and pruned) via t.Cleanup, which runs before t.TempDir() teardown.

package gitutil

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// nestedWTGit runs git in dir and fails the test on error.
func nestedWTGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %s: %v", args, dir, out, err)
	}
	return string(out)
}

// nestedWTRepo initializes a repo with one commit and returns its root.
func nestedWTRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	nestedWTGit(t, dir, "init", "-q", "-b", "main", ".")
	nestedWTGit(t, dir, "config", "user.email", "t@example.com")
	nestedWTGit(t, dir, "config", "user.name", "T")
	nestedWTGit(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nestedWTGit(t, dir, "add", "README.md")
	nestedWTGit(t, dir, "commit", "-qm", "seed")
	return dir
}

// addWorktree registers a linked worktree at path (absolute) and
// schedules `git worktree remove --force` + prune before temp teardown.
func addWorktree(t *testing.T, repoRoot, path, branch string) {
	t.Helper()
	nestedWTGit(t, repoRoot, "worktree", "add", "-q", path, "-b", branch)
	t.Cleanup(func() {
		rm := exec.Command("git", "worktree", "remove", "--force", path)
		rm.Dir = repoRoot
		_ = rm.Run()
		prune := exec.Command("git", "worktree", "prune")
		prune.Dir = repoRoot
		_ = prune.Run()
	})
}

// ─── layer 1: parser ────────────────────────────────────────────────

func TestParseWorktreeListPorcelain_NUL(t *testing.T) {
	out := strings.Join([]string{
		"worktree /repo\x00",
		"HEAD abc\x00",
		"branch refs/heads/main\x00",
		"\x00",
		"worktree /repo/.claude/worktrees/agent review\x00",
		"HEAD def\x00",
		"branch refs/heads/agent-review\x00",
		"\x00",
	}, "")
	got, err := parseWorktreeListPorcelain(out, true)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []string{"/repo", "/repo/.claude/worktrees/agent review"}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("path[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseWorktreeListPorcelain_Newline(t *testing.T) {
	out := "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n" +
		"worktree /repo/nested wt with  spaces\nHEAD def\ndetached\n\n"
	got, err := parseWorktreeListPorcelain(out, false)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []string{"/repo", "/repo/nested wt with  spaces"}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("path[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseWorktreeListPorcelain_Malformed(t *testing.T) {
	cases := []struct {
		name string
		out  string
		nul  bool
	}{
		{"empty output", "", false},
		{"empty output nul", "", true},
		{"no worktree records", "HEAD abc\nbranch refs/heads/main\n\n", false},
		{"empty path", "worktree \nHEAD abc\n\n", false},
		{"empty path nul", "worktree \x00HEAD abc\x00\x00", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseWorktreeListPorcelain(tc.out, tc.nul); err == nil {
				t.Fatalf("expected error for malformed output %q", tc.out)
			}
		})
	}
}

// ─── layer 1: classification + membership ───────────────────────────

func TestNestedWorktreePrefixes_Classification(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, ".claude", "worktrees", "agent review")
	deeper := filepath.Join(root, "tools", "wt")
	outside := filepath.Join(t.TempDir(), "external")

	got := nestedWorktreePrefixes(root, []string{
		root,                 // the target root itself → never a prefix
		nested,               // nested, contains a space
		deeper,               // nested, plain
		outside,              // registered but outside the target root
		root + "-sibling/wt", // prefix-boundary sibling of the root
		"",                   // defensive: blank record
	})
	want := []string{".claude/worktrees/agent review", "tools/wt"}
	if len(got) != len(want) {
		t.Fatalf("prefixes = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("prefix[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPathUnderNestedWorktree_PrefixBoundary(t *testing.T) {
	prefixes := []string{".claude/worktrees/agent", "tools/wt"}
	cases := []struct {
		path string
		want bool
	}{
		{".claude/worktrees/agent", true},
		{".claude/worktrees/agent/", true},
		{"./.claude/worktrees/agent", true},
		{".claude/worktrees/agent/src/main.go", true},
		{".claude/worktrees/agent-other", false},
		{".claude/worktrees/agent-other/f.txt", false},
		{".claude/worktrees/agentx", false},
		{".claude/worktrees", false},
		{"tools/wt", true},
		{"tools/wtx/f.txt", false},
		{"tools/wt-helper.go", false},
		{"README.md", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := PathUnderNestedWorktree(tc.path, prefixes); got != tc.want {
			t.Errorf("PathUnderNestedWorktree(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
	if PathUnderNestedWorktree("anything", nil) {
		t.Error("nil prefixes must never match")
	}
}

func TestFilterNestedWorktreePaths(t *testing.T) {
	prefixes := []string{"wt"}
	got := FilterNestedWorktreePaths([]string{"a.go", "wt/b.go", "wt", "wt-other/c.go"}, prefixes)
	want := []string{"a.go", "wt-other/c.go"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("filtered = %q, want %q", got, want)
	}
	// Empty prefix set is an identity transform.
	in := []string{"a.go", "wt/b.go"}
	if out := FilterNestedWorktreePaths(in, nil); strings.Join(out, ",") != strings.Join(in, ",") {
		t.Fatalf("nil prefixes changed the slice: %q", out)
	}
}

// ─── layer 2: real Git fixtures ─────────────────────────────────────

func TestNestedWorktreePrefixes_RealRepo(t *testing.T) {
	root := nestedWTRepo(t)
	addWorktree(t, root, filepath.Join(root, ".claude", "worktrees", "agent review"), "agent-review")
	external := filepath.Join(t.TempDir(), "external")
	addWorktree(t, root, external, "external-wt")

	// An unregistered nested Git repository and an ordinary directory
	// whose name is a strict prefix of the worktree must both survive.
	plainRepo := filepath.Join(root, "vendor", "plain")
	if err := os.MkdirAll(plainRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	nestedWTGit(t, plainRepo, "init", "-q", "-b", "main", ".")
	if err := os.MkdirAll(filepath.Join(root, ".claude", "worktrees", "agent-other"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := NestedWorktreePrefixes(root)
	if err != nil {
		t.Fatalf("NestedWorktreePrefixes: %v", err)
	}
	if len(got) != 1 || got[0] != ".claude/worktrees/agent review" {
		t.Fatalf("prefixes = %q, want exactly [.claude/worktrees/agent review]", got)
	}
	if PathUnderNestedWorktree("vendor/plain/x", got) {
		t.Error("unregistered nested Git repository must not be excluded")
	}
	if PathUnderNestedWorktree(".claude/worktrees/agent-other/f.txt", got) {
		t.Error("prefix-boundary sibling directory must not be excluded")
	}
}

// The historical "run tpatch from a linked worktree" flow must keep
// working: from inside a linked worktree, the main worktree is OUTSIDE
// the effective root and must not be reported as nested.
func TestNestedWorktreePrefixes_FromLinkedWorktree(t *testing.T) {
	root := nestedWTRepo(t)
	linked := filepath.Join(t.TempDir(), "linked")
	addWorktree(t, root, linked, "linked-wt")

	got, err := NestedWorktreePrefixes(linked)
	if err != nil {
		t.Fatalf("NestedWorktreePrefixes: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("prefixes from a linked worktree = %q, want none", got)
	}
}

func TestNestedWorktreePrefixes_FailsClosed(t *testing.T) {
	notARepo := t.TempDir()
	_, err := NestedWorktreePrefixes(notARepo)
	if err == nil {
		t.Fatal("discovery outside a Git repository must return an error, not an empty prefix set")
	}
	wrapped := NestedWorktreeDiscoveryError(notARepo, err)
	if !errors.Is(wrapped, ErrNestedWorktreeDiscovery) {
		t.Errorf("wrapped error must match ErrNestedWorktreeDiscovery: %v", wrapped)
	}
	for _, want := range []string{"Refusing to capture", "git worktree prune"} {
		if !strings.Contains(wrapped.Error(), want) {
			t.Errorf("discovery error missing actionable guidance %q: %v", want, wrapped)
		}
	}
}

func TestListUntrackedFilesExcludesNestedWorktree(t *testing.T) {
	root := nestedWTRepo(t)
	addWorktree(t, root, filepath.Join(root, ".claude", "worktrees", "agent review"), "agent-review")
	if err := os.MkdirAll(filepath.Join(root, ".claude", "worktrees", "agent-other"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "worktrees", "agent-other", "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "new.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := listUntrackedFiles(root, nil)
	if err != nil {
		t.Fatalf("listUntrackedFiles: %v", err)
	}
	joined := strings.Join(files, "\n")
	if strings.Contains(joined, "agent review") {
		t.Errorf("nested worktree leaked into untracked discovery:\n%s", joined)
	}
	for _, want := range []string{"new.go", ".claude/worktrees/agent-other/f.txt"} {
		if !strings.Contains(joined, want) {
			t.Errorf("untracked discovery dropped %q:\n%s", want, joined)
		}
	}

	// Explicitly naming the worktree as a pathspec must not re-admit it.
	scoped, err := listUntrackedFiles(root, []string{".claude/worktrees/agent review"})
	if err != nil {
		t.Fatalf("listUntrackedFiles scoped: %v", err)
	}
	if len(scoped) != 0 {
		t.Errorf("explicit --files pathspec re-admitted the nested worktree: %q", scoped)
	}
}

func TestCapturePatchScopedExcludesNestedWorktree(t *testing.T) {
	root := nestedWTRepo(t)
	addWorktree(t, root, filepath.Join(root, ".claude", "worktrees", "agent review"), "agent-review")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# seed\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name      string
		pathspecs []string
	}{
		{"default", nil},
		{"scoped to the worktree", []string{".claude/worktrees/agent review"}},
		{"scoped to the worktree parent", []string{".claude"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			patch, err := CapturePatchScoped(root, tc.pathspecs)
			if err != nil {
				t.Fatalf("CapturePatchScoped: %v", err)
			}
			if strings.Contains(patch, "160000") || strings.Contains(patch, "agent review") {
				t.Errorf("nested worktree captured:\n%s", patch)
			}
		})
	}

	patch, err := CapturePatchScoped(root, nil)
	if err != nil {
		t.Fatalf("CapturePatchScoped: %v", err)
	}
	if !strings.Contains(patch, "+changed") {
		t.Errorf("intended source change missing from capture:\n%s", patch)
	}

	// The capture must leave no intent-to-add residue behind.
	if out := nestedWTGit(t, root, "diff", "--cached", "--name-only"); strings.TrimSpace(out) != "" {
		t.Errorf("capture left staged residue: %q", out)
	}
}

// A tracked gitlink (the submodule shape) must keep flowing through
// capture untouched — the filter is scoped to REGISTERED linked
// worktrees, not to gitlinks in general.
func TestCaptureKeepsTrackedGitlink(t *testing.T) {
	root := nestedWTRepo(t)
	sub := filepath.Join(root, "vendor", "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	nestedWTGit(t, sub, "init", "-q", "-b", "main", ".")
	nestedWTGit(t, sub, "config", "user.email", "t@example.com")
	nestedWTGit(t, sub, "config", "user.name", "T")
	nestedWTGit(t, sub, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(sub, "s.txt"), []byte("s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nestedWTGit(t, sub, "add", "s.txt")
	nestedWTGit(t, sub, "commit", "-qm", "sub seed")

	// Intentionally track the nested repository as a gitlink.
	nestedWTGit(t, root, "-c", "advice.addEmbeddedRepo=false", "add", "vendor/sub")

	prefixes, err := NestedWorktreePrefixes(root)
	if err != nil {
		t.Fatalf("NestedWorktreePrefixes: %v", err)
	}
	if PathUnderNestedWorktree("vendor/sub", prefixes) {
		t.Fatalf("tracked gitlink was classified as a nested linked worktree (prefixes=%q)", prefixes)
	}
	patch, _, err := CaptureStagedPatch(root, nil)
	if err != nil {
		t.Fatalf("CaptureStagedPatch: %v", err)
	}
	if !strings.Contains(patch, "vendor/sub") || !strings.Contains(patch, "160000") {
		t.Errorf("intentionally tracked gitlink dropped from staged capture:\n%s", patch)
	}
}

func TestCaptureUnstagedPatchExcludesNestedWorktree(t *testing.T) {
	root := nestedWTRepo(t)
	addWorktree(t, root, filepath.Join(root, ".claude", "worktrees", "agent review"), "agent-review")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# seed\nunstaged\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	patch, summary, err := CaptureUnstagedPatch(root, nil)
	if err != nil {
		t.Fatalf("CaptureUnstagedPatch: %v", err)
	}
	if strings.Contains(patch, "agent review") || strings.Contains(patch, "160000") {
		t.Errorf("nested worktree captured by --unstaged:\n%s", patch)
	}
	if !strings.Contains(patch, "+unstaged") {
		t.Errorf("intended unstaged change missing:\n%s", patch)
	}
	if summary.UnstagedPaths != 1 {
		t.Errorf("UnstagedPaths = %d, want 1 (nested worktree must not be counted)", summary.UnstagedPaths)
	}

	_, _, unrelatedUnstaged, err := StagedUnstagedOverlap(root, nil)
	if err != nil {
		t.Fatalf("StagedUnstagedOverlap: %v", err)
	}
	for _, p := range unrelatedUnstaged {
		if strings.Contains(p, "agent review") {
			t.Errorf("nested worktree leaked into overlap diagnostics: %q", unrelatedUnstaged)
		}
	}
}
