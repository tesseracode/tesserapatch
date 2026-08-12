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
	"fmt"
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

// ─── layer 1: NUL parser (byte exactness) ───────────────────────────

// nulRecord renders one NUL-delimited porcelain record.
func nulRecord(fields ...string) string {
	var b strings.Builder
	for _, f := range fields {
		b.WriteString(f)
		b.WriteByte(0)
	}
	b.WriteByte(0)
	return b.String()
}

func TestParseWorktreeListNUL_PreservesPathBytes(t *testing.T) {
	// Every one of these is a legal directory name whose bytes MUST
	// survive parsing untouched. Trailing whitespace is the rev-0
	// regression: TrimSpace turned `agent ` into `agent`, which then
	// stopped matching the path Git reports elsewhere for the same
	// directory, letting the worktree back into capture.
	paths := []string{
		"/repo",
		"/repo/wt/trailing space ",
		"/repo/wt/trailing tab\t",
		"/repo/wt/ leading space",
		"/repo/wt/internal\ttab and  double  spaces",
		"/repo/wt/new\nline",
		"/repo/wt/quote\"and\\backslash",
	}
	var out strings.Builder
	for i, p := range paths {
		out.WriteString(nulRecord(worktreeKey+p, "HEAD abc", fmt.Sprintf("branch refs/heads/b%d", i)))
	}

	got, err := parseWorktreeListNUL(out.String())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != len(paths) {
		t.Fatalf("got %d paths %q, want %d", len(got), got, len(paths))
	}
	for i := range paths {
		if got[i] != paths[i] {
			t.Errorf("path[%d] = %q, want %q (byte-for-byte)", i, got[i], paths[i])
		}
	}
}

func TestParseWorktreeListNUL_Malformed(t *testing.T) {
	cases := []struct {
		name string
		out  string
	}{
		{"empty output", ""},
		{"no worktree records", nulRecord("HEAD abc", "branch refs/heads/main")},
		{"empty path", nulRecord("worktree ", "HEAD abc")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseWorktreeListNUL(tc.out); err == nil {
				t.Fatalf("expected error for malformed output %q", tc.out)
			}
		})
	}
}

// ─── layer 1: no legacy fallback ────────────────────────────────────

// GH #7 rev-2 F1/F2: the newline-delimited fallback is REMOVED, not
// merely stricter. Any `-z` failure is fail-closed with the Git 2.36
// guidance; tpatch never runs plain `git worktree list --porcelain`.
func TestListRegisteredWorktreePaths_NoLegacyFallback(t *testing.T) {
	notARepo := t.TempDir()
	_, err := listRegisteredWorktreePaths(notARepo)
	if err == nil {
		t.Fatal("expected an error outside a Git repository")
	}
	for _, want := range []string{"--porcelain -z", "2.36", "will not fall back"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("failure message missing %q: %v", want, err)
		}
	}
	wrapped := NestedWorktreeDiscoveryError(notARepo, err)
	if !errors.Is(wrapped, ErrNestedWorktreeDiscovery) {
		t.Errorf("failure not in the fail-closed class: %v", wrapped)
	}
}

// The production source must contain no plain-`--porcelain`
// invocation and no legacy parser symbols: a reviewer should be able
// to prove the fallback is gone by grep alone.
func TestNoLegacyPorcelainParserInProduction(t *testing.T) {
	body, err := os.ReadFile("worktrees.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	for _, banned := range []string{
		"parseWorktreeListLegacy",
		"isUnknownSwitchError",
		"unquoteCStyle",
		`"--porcelain")`,
	} {
		if strings.Contains(src, banned) {
			t.Errorf("legacy fallback residue %q still present in production source", banned)
		}
	}
	// Every `git worktree list` argv in production must carry -z.
	for _, line := range strings.Split(src, "\n") {
		if strings.Contains(line, `runGitStreams(`) && strings.Contains(line, `"worktree"`) {
			if !strings.Contains(line, `"-z"`) {
				t.Errorf("worktree list invocation without -z: %s", strings.TrimSpace(line))
			}
		}
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

// ─── layer 2: real fixtures with byte-exact exotic names ────────────

// exoticWorktreeNames are legal directory names whose bytes must
// survive discovery, prefix generation and pathspec exclusion. Each is
// paired with a plain branch name (branch refs cannot carry these
// bytes).
var exoticWorktreeNames = []struct {
	dir    string
	branch string
}{
	{"trailing space ", "wt-trailing-space"},
	{"trailing tab\t", "wt-trailing-tab"},
	{" leading space", "wt-leading-space"},
	{"internal\ttab and  double  spaces", "wt-internal"},
}

// TestNestedWorktreePrefixes_ExoticNamesRealRepo is the rev-0
// regression: `TrimSpace` on the parsed path produced the prefix
// `wt/trailing space` for the directory `wt/trailing space `, which
// then failed to match the path Git reports in `ls-files --others`,
// so the worktree flowed straight back into capture.
func TestNestedWorktreePrefixes_ExoticNamesRealRepo(t *testing.T) {
	root := nestedWTRepo(t)

	var want []string
	for _, n := range exoticWorktreeNames {
		rel := "wt/" + n.dir
		addWorktree(t, root, filepath.Join(root, "wt", n.dir), n.branch)
		want = append(want, rel)
	}
	// Prefix-boundary controls: ordinary directories whose names are
	// the worktree names minus the significant whitespace, plus a
	// classic sibling. None may be excluded.
	controls := []string{"wt/trailing space", "wt/trailing tab", "wt/leading space", "wt/internal"}
	for _, c := range controls {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(c)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(c), "keep.txt"), []byte("keep\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := NestedWorktreePrefixes(root)
	if err != nil {
		t.Fatalf("NestedWorktreePrefixes: %v", err)
	}
	gotSet := map[string]bool{}
	for _, g := range got {
		gotSet[g] = true
	}
	for _, w := range want {
		if !gotSet[w] {
			t.Errorf("prefix %q missing (byte-for-byte) from %q", w, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("prefixes = %q, want exactly %q", got, want)
	}
	for _, c := range controls {
		if PathUnderNestedWorktree(c+"/keep.txt", got) {
			t.Errorf("prefix-boundary control %q was excluded by %q", c, got)
		}
	}

	// Untracked discovery drops every worktree and keeps every control.
	files, err := listUntrackedFiles(root, nil)
	if err != nil {
		t.Fatalf("listUntrackedFiles: %v", err)
	}
	joined := strings.Join(files, "\n")
	for _, w := range want {
		if strings.Contains(joined, w+"/") || contains(files, w) {
			t.Errorf("nested worktree %q leaked into untracked discovery:\n%s", w, joined)
		}
	}
	for _, c := range controls {
		if !contains(files, c+"/keep.txt") {
			t.Errorf("prefix-boundary control %q dropped from untracked discovery:\n%s", c, joined)
		}
	}

	// Capture: the exclude pathspecs must survive the exotic bytes.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# seed\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch, err := CapturePatchScoped(root, nil)
	if err != nil {
		t.Fatalf("CapturePatchScoped: %v", err)
	}
	if strings.Contains(patch, "160000") {
		t.Errorf("nested worktree captured as a gitlink:\n%s", patch)
	}
	for _, w := range want {
		if strings.Contains(patch, w) {
			t.Errorf("nested worktree %q captured:\n%s", w, patch)
		}
	}
	if !strings.Contains(patch, "+changed") {
		t.Errorf("intended change missing:\n%s", patch)
	}
	for _, c := range controls {
		if !strings.Contains(patch, c+"/keep.txt") {
			t.Errorf("prefix-boundary control %q dropped from capture:\n%s", c, patch)
		}
	}
	stat, err := CaptureDiffStat(root)
	if err != nil {
		t.Fatalf("CaptureDiffStat: %v", err)
	}
	for _, w := range want {
		if strings.Contains(stat, w) {
			t.Errorf("nested worktree %q leaked into the diffstat:\n%s", w, stat)
		}
	}
}

// A worktree whose directory name contains a newline is the case only
// the NUL shape can express. Skipped when the platform refuses the
// name.
func TestNestedWorktreePrefixes_NewlineNameRealRepo(t *testing.T) {
	root := nestedWTRepo(t)
	rel := "wt/new\nline"
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	probe := exec.Command("git", "worktree", "add", "-q", abs, "-b", "wt-newline")
	probe.Dir = root
	if out, err := probe.CombinedOutput(); err != nil {
		t.Skipf("platform/Git refuses a newline in a worktree path: %s: %v", out, err)
	}
	t.Cleanup(func() {
		rm := exec.Command("git", "worktree", "remove", "--force", abs)
		rm.Dir = root
		_ = rm.Run()
		prune := exec.Command("git", "worktree", "prune")
		prune.Dir = root
		_ = prune.Run()
	})

	got, err := NestedWorktreePrefixes(root)
	if err != nil {
		t.Fatalf("NestedWorktreePrefixes: %v", err)
	}
	if len(got) != 1 || got[0] != rel {
		t.Fatalf("prefixes = %q, want exactly [%q]", got, rel)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# seed\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch, err := CapturePatchScoped(root, nil)
	if err != nil {
		t.Fatalf("CapturePatchScoped: %v", err)
	}
	if strings.Contains(patch, "160000") || strings.Contains(patch, "new\nline") {
		t.Errorf("newline-named nested worktree captured:\n%q", patch)
	}
	if !strings.Contains(patch, "+changed") {
		t.Errorf("intended change missing:\n%s", patch)
	}
}

// GH #7 rev-1 residual 3: a pre-existing intent-to-add / staged gitlink
// for a nested registered worktree must be filtered out of the
// DIFFSTAT as well as the patch.
func TestCaptureDiffStatExcludesStagedNestedWorktreeResidue(t *testing.T) {
	root := nestedWTRepo(t)
	rel := ".claude/worktrees/agent review"
	addWorktree(t, root, filepath.Join(root, filepath.FromSlash(rel)), "agent-review")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# seed\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Residue a pre-fix tpatch run (or an operator `git add`) leaves.
	nestedWTGit(t, root, "-c", "advice.addEmbeddedRepo=false", "--literal-pathspecs", "add", "--intent-to-add", "--", rel)

	patch, err := CapturePatchScoped(root, nil)
	if err != nil {
		t.Fatalf("CapturePatchScoped: %v", err)
	}
	if strings.Contains(patch, "agent review") || strings.Contains(patch, "160000") {
		t.Errorf("staged gitlink residue leaked into the patch:\n%s", patch)
	}
	stat, err := CaptureDiffStat(root)
	if err != nil {
		t.Fatalf("CaptureDiffStat: %v", err)
	}
	if strings.Contains(stat, "agent review") {
		t.Errorf("staged gitlink residue leaked into the diffstat:\n%s", stat)
	}
	if !strings.Contains(stat, "README.md") {
		t.Errorf("intended file missing from the diffstat:\n%s", stat)
	}
	// Scoped diffstat inherits the same exclusions.
	scoped, err := CaptureDiffStatScoped(root, []string{"README.md", rel})
	if err != nil {
		t.Fatalf("CaptureDiffStatScoped: %v", err)
	}
	if strings.Contains(scoped, "agent review") {
		t.Errorf("staged gitlink residue leaked into the scoped diffstat:\n%s", scoped)
	}
	if !strings.Contains(scoped, "README.md") {
		t.Errorf("intended file missing from the scoped diffstat:\n%s", scoped)
	}
}

func contains(items []string, want string) bool {
	for _, s := range items {
		if s == want {
			return true
		}
	}
	return false
}

// installNoZWorktreeListGit prepends a `git` wrapper to PATH that
// rejects `worktree list ... -z` exactly the way a pre-2.36 Git does,
// records every `worktree list` invocation, and execs the real git for
// everything else. Production code is untouched: the seam is the `git`
// executable. The returned path is the invocation log.
func installNoZWorktreeListGit(t *testing.T) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate real git: %v", err)
	}
	binDir := t.TempDir()
	invocationLog := filepath.Join(binDir, "worktree-list-calls.log")
	script := "#!/bin/sh\nlist=0\nz=0\nprev=\"\"\nfor a in \"$@\"; do\n" +
		"  if [ \"$prev\" = \"worktree\" ] && [ \"$a\" = \"list\" ]; then list=1; fi\n" +
		"  if [ \"$a\" = \"-z\" ]; then z=1; fi\n" +
		"  prev=\"$a\"\n" +
		"done\n" +
		"if [ \"$list\" = 1 ]; then echo \"$@\" >> " + invocationLog + "; fi\n" +
		"if [ \"$list\" = 1 ] && [ \"$z\" = 1 ]; then\n" +
		"  echo \"error: unknown switch \\`z'\" >&2\n" +
		"  echo \"usage: git worktree list [-v | --porcelain]\" >&2\n" +
		"  exit 129\n" +
		"fi\n" +
		"exec " + realGit + " \"$@\"\n"
	shim := filepath.Join(binDir, "git")
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	probe := exec.Command("git", "worktree", "list", "--porcelain", "-z")
	probe.Dir = binDir
	if err := probe.Run(); err == nil {
		t.Fatal("git wrapper did not reject -z")
	}
	_ = os.Remove(invocationLog)
	return invocationLog
}

// worktreeListInvocations returns the recorded `git worktree list`
// argv lines.
func worktreeListInvocations(t *testing.T, logPath string) []string {
	t.Helper()
	body, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// GH #7 rev-2 F1/F2: when `-z` is rejected, discovery must fail closed
// and must NOT retry with plain `--porcelain`.
func TestNestedWorktreePrefixes_NoZRejectionFailsClosedWithoutRetry(t *testing.T) {
	root := nestedWTRepo(t)
	rel := ".claude/worktrees/agent review"
	addWorktree(t, root, filepath.Join(root, filepath.FromSlash(rel)), "agent-review")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# seed\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	logPath := installNoZWorktreeListGit(t)

	got, err := NestedWorktreePrefixes(root)
	if err == nil {
		t.Fatalf("a -z rejection must fail closed, got prefixes %q", got)
	}
	if !errors.Is(err, ErrNestedWorktreeDiscovery) {
		t.Errorf("failure not in the fail-closed class: %v", err)
	}
	for _, want := range []string{"2.36", "--porcelain -z", "will not fall back"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("guidance missing %q: %v", want, err)
		}
	}

	calls := worktreeListInvocations(t, logPath)
	if len(calls) != 1 {
		t.Fatalf("expected exactly one `git worktree list` invocation, got %d: %q", len(calls), calls)
	}
	if !strings.Contains(calls[0], "-z") {
		t.Errorf("the single invocation must be the -z form: %q", calls[0])
	}

	// Every discovery-dependent entry point refuses rather than
	// operating blind, and none of them retries the plain form.
	if _, err := CapturePatchScoped(root, nil); !errors.Is(err, ErrNestedWorktreeDiscovery) {
		t.Errorf("CapturePatchScoped must fail closed on a -z rejection, got %v", err)
	}
	if _, err := CaptureDiffStat(root); !errors.Is(err, ErrNestedWorktreeDiscovery) {
		t.Errorf("CaptureDiffStat must fail closed on a -z rejection, got %v", err)
	}
	if _, err := DiffFromCommitForPaths(root, "HEAD", []string{"README.md"}); !errors.Is(err, ErrNestedWorktreeDiscovery) {
		t.Errorf("DiffFromCommitForPaths must fail closed on a -z rejection, got %v", err)
	}
	if _, err := FilterPathsExcludingNestedWorktrees(root, []string{"README.md"}); !errors.Is(err, ErrNestedWorktreeDiscovery) {
		t.Errorf("FilterPathsExcludingNestedWorktrees must fail closed on a -z rejection, got %v", err)
	}
	for _, call := range worktreeListInvocations(t, logPath) {
		if !strings.Contains(call, "-z") {
			t.Errorf("plain --porcelain was invoked: %q", call)
		}
	}
}

// A repository whose ONLY dirt is a nested linked worktree must read
// as clean, so the record empty-capture diagnostic routes to the
// correct `--from` guidance instead of speculating about mode-only or
// binary changes (GH #7 rev-0 user-external note).
func TestIsWorkingTreeDirtyIgnoresNestedWorktree(t *testing.T) {
	root := nestedWTRepo(t)
	if IsWorkingTreeDirty(root) {
		t.Fatal("freshly committed repo should read clean")
	}
	addWorktree(t, root, filepath.Join(root, ".claude", "worktrees", "agent review"), "agent-review")
	if IsWorkingTreeDirty(root) {
		t.Error("a nested linked worktree must not count as working-tree dirt")
	}

	// Genuine dirt still counts, in every shape.
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsWorkingTreeDirty(root) {
		t.Error("an untracked file must count as dirt")
	}
	if err := os.Remove(filepath.Join(root, "untracked.txt")); err != nil {
		t.Fatal(err)
	}
	if IsWorkingTreeDirty(root) {
		t.Fatal("removing the untracked file should restore clean")
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# seed\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsWorkingTreeDirty(root) {
		t.Error("a tracked modification must count as dirt")
	}
	nestedWTGit(t, root, "add", "README.md")
	if !IsWorkingTreeDirty(root) {
		t.Error("a staged modification must count as dirt")
	}
}

// ─── GH #7 rev-2 F3: DiffFromCommitForPaths filtering ───────────────

// A stale nested-worktree gitlink path in the caller's list must be
// dropped before the intent-to-add pass and excluded from the diff,
// while intended paths still flow through.
func TestDiffFromCommitForPathsExcludesNestedWorktree(t *testing.T) {
	root := nestedWTRepo(t)
	rel := ".claude/worktrees/agent review"
	addWorktree(t, root, filepath.Join(root, filepath.FromSlash(rel)), "agent-review")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# seed\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "new.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	statusBefore := nestedWTGit(t, root, "status", "--porcelain", "-z", "--untracked-files=all")

	diff, err := DiffFromCommitForPaths(root, "HEAD", []string{"README.md", "new.go", rel})
	if err != nil {
		t.Fatalf("DiffFromCommitForPaths: %v", err)
	}
	if strings.Contains(diff, "agent review") || strings.Contains(diff, "160000") {
		t.Errorf("nested worktree leaked into the refreshed diff:\n%s", diff)
	}
	for _, want := range []string{"README.md", "new.go", "+changed"} {
		if !strings.Contains(diff, want) {
			t.Errorf("intended path %q missing from the refreshed diff:\n%s", want, diff)
		}
	}
	// The real index must be untouched (temp-index contract).
	if got := nestedWTGit(t, root, "status", "--porcelain", "-z", "--untracked-files=all"); got != statusBefore {
		t.Errorf("real index/worktree mutated:\nbefore=%q\nafter=%q", statusBefore, got)
	}
}

// A caller list consisting ONLY of nested-worktree paths must produce
// an EMPTY diff, never a broadened full-tree diff.
func TestDiffFromCommitForPathsWorktreeOnlyScopeReturnsEmpty(t *testing.T) {
	root := nestedWTRepo(t)
	rel := ".claude/worktrees/agent review"
	addWorktree(t, root, filepath.Join(root, filepath.FromSlash(rel)), "agent-review")
	// Real, unrelated change that a broadened full-tree diff WOULD pick up.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# seed\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	diff, err := DiffFromCommitForPaths(root, "HEAD", []string{rel, rel + "/src"})
	if err != nil {
		t.Fatalf("DiffFromCommitForPaths: %v", err)
	}
	if diff != "" {
		t.Fatalf("worktree-only scope must yield an empty diff, got:\n%s", diff)
	}
}

// An empty caller list still means "full diff", minus nested worktrees.
func TestDiffFromCommitForPathsEmptyScopeStillExcludesNestedWorktree(t *testing.T) {
	root := nestedWTRepo(t)
	rel := ".claude/worktrees/agent review"
	addWorktree(t, root, filepath.Join(root, filepath.FromSlash(rel)), "agent-review")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# seed\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Residue a pre-fix run leaves in the real index.
	nestedWTGit(t, root, "-c", "advice.addEmbeddedRepo=false",
		"--literal-pathspecs", "add", "--intent-to-add", "--", rel)

	diff, err := DiffFromCommitForPaths(root, "HEAD", nil)
	if err != nil {
		t.Fatalf("DiffFromCommitForPaths: %v", err)
	}
	if strings.Contains(diff, "agent review") || strings.Contains(diff, "160000") {
		t.Errorf("nested worktree leaked into the full diff:\n%s", diff)
	}
	if !strings.Contains(diff, "+changed") {
		t.Errorf("intended change missing from the full diff:\n%s", diff)
	}
}

// The linked-worktree effective-index behavior must survive the
// pathspec-magic change (`--literal-pathspecs` → `:(literal)`).
func TestDiffFromCommitForPathsPreservesLinkedWorktreeIndexAfterFiltering(t *testing.T) {
	mainDir := nestedWTRepo(t)
	linkedDir := filepath.Join(t.TempDir(), "linked")
	addWorktree(t, mainDir, linkedDir, "linked-rev2")

	if err := os.WriteFile(filepath.Join(linkedDir, "hello.txt"), []byte("linked change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nestedWTGit(t, linkedDir, "add", "hello.txt")
	statusBefore := nestedWTGit(t, linkedDir, "status", "--porcelain=v1")
	indexBefore := nestedWTGit(t, linkedDir, "write-tree")

	diff, err := DiffFromCommitForPaths(linkedDir, "HEAD", []string{"hello.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "linked change") {
		t.Fatalf("linked-worktree diff missing staged change:\n%s", diff)
	}
	if got := nestedWTGit(t, linkedDir, "status", "--porcelain=v1"); got != statusBefore {
		t.Fatalf("status changed:\nbefore=%q\nafter=%q", statusBefore, got)
	}
	if got := nestedWTGit(t, linkedDir, "write-tree"); got != indexBefore {
		t.Fatalf("index changed:\nbefore=%s\nafter=%s", indexBefore, got)
	}
}

// A path containing pathspec-magic-looking bytes must still be treated
// literally now that the global --literal-pathspecs flag is gone.
func TestDiffFromCommitForPathsKeepsLiteralPathSemantics(t *testing.T) {
	root := nestedWTRepo(t)
	name := "weird[1]*name.txt"
	if err := os.WriteFile(filepath.Join(root, name), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nestedWTGit(t, root, "--literal-pathspecs", "add", "--", name)
	nestedWTGit(t, root, "commit", "-qm", "add weird name")
	if err := os.WriteFile(filepath.Join(root, name), []byte("v1\nv2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	diff, err := DiffFromCommitForPaths(root, "HEAD", []string{name})
	if err != nil {
		t.Fatalf("DiffFromCommitForPaths: %v", err)
	}
	if !strings.Contains(diff, "+v2") {
		t.Errorf("literal pathspec semantics broken for %q:\n%s", name, diff)
	}
}
