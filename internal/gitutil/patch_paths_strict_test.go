// Tests for FilesInPatchStrict (GH #7 rev-3 F2).
//
// The fixtures are produced by REAL Git so the quoting is whatever the
// installed Git actually emits, not a hand-written approximation of it.

package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func strictGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %s: %v", args, dir, out, err)
	}
	return string(out)
}

// strictGitOut runs git capturing stdout only (diffs must not be
// polluted by stderr).
func strictGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.Output()
	if err != nil {
		t.Fatalf("git %v in %s: %v", args, dir, err)
	}
	return string(out)
}

// exoticDiffFixture builds a repo containing paths Git must C-quote and
// returns the repo root plus a diff that exercises new files, a rename,
// a copy, a binary entry, a mode-only entry and a deletion.
func exoticDiffFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	strictGit(t, dir, "init", "-q", "-b", "main", ".")
	strictGit(t, dir, "config", "user.email", "t@example.com")
	strictGit(t, dir, "config", "user.name", "T")
	strictGit(t, dir, "config", "commit.gpgsign", "false")
	strictGit(t, dir, "config", "diff.renames", "true")

	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("base.txt", "base\n")
	write("mode only.sh", "#!/bin/sh\necho hi\n")
	write("to delete.txt", "bye\n")
	strictGit(t, dir, "add", "-A")
	strictGit(t, dir, "commit", "-qm", "seed")
	return dir
}

// TestFilesInPatchStrictRealGitQuotedHeaders is the F2 regression: a
// path Git C-quotes must be recovered byte-for-byte, and the fail-soft
// scanner must be shown to miss it.
func TestFilesInPatchStrictRealGitQuotedHeaders(t *testing.T) {
	dir := exoticDiffFixture(t)

	exotic := map[string]string{
		"sp ace.txt":     "space\n",
		"tab\tin.txt":    "tab\n",
		"new\nline.txt":  "newline\n",
		"qu\"ote.txt":    "quote\n",
		"back\\slash.md": "backslash\n",
		"café.txt":       "octal\n",
	}
	for name, body := range exotic {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %q: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "bin ary.dat"), []byte{0x00, 0x01, 0x02, 'b', 'i', 'n', '\n'}, 0o644); err != nil {
		t.Fatal(err)
	}
	// Rename, deletion and a mode-only change round out the shapes.
	strictGit(t, dir, "mv", "base.txt", "renamed to.txt")
	strictGit(t, dir, "rm", "-q", "to delete.txt")
	if err := os.Chmod(filepath.Join(dir, "mode only.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	strictGit(t, dir, "add", "-A")

	patch := strictGitOut(t, dir, "diff", "--cached", "-M", "--binary", "HEAD")
	if !strings.Contains(patch, `diff --git "a/new\nline.txt"`) {
		t.Fatalf("fixture did not produce a C-quoted newline header:\n%s", patch)
	}

	got, err := FilesInPatchStrict(patch)
	if err != nil {
		t.Fatalf("FilesInPatchStrict: %v\npatch:\n%s", err, patch)
	}
	gotSet := map[string]bool{}
	for _, p := range got {
		gotSet[p] = true
	}
	want := []string{
		"sp ace.txt", "tab\tin.txt", "new\nline.txt", "qu\"ote.txt",
		"back\\slash.md", "café.txt", "bin ary.dat",
		"renamed to.txt", "to delete.txt", "mode only.sh",
	}
	for _, w := range want {
		if !gotSet[w] {
			t.Errorf("strict parse missed %q; got %q", w, got)
		}
	}

	// The fail-soft scanner demonstrably misses the quoted entries —
	// this is why the write-scope callers had to move.
	soft := map[string]bool{}
	for _, p := range FilesInPatch(patch) {
		soft[p] = true
	}
	for _, quoted := range []string{"new\nline.txt", "tab\tin.txt", "qu\"ote.txt", "café.txt"} {
		if soft[quoted] {
			t.Errorf("FilesInPatch unexpectedly resolved %q; the strict/fail-soft split may be obsolete", quoted)
		}
	}
}

// A worktree-only stale patch whose single entry is C-quoted must parse
// to exactly that one path — never to an empty scope.
func TestFilesInPatchStrictQuotedGitlinkOnlyPatch(t *testing.T) {
	dir := exoticDiffFixture(t)
	wt := filepath.Join(dir, ".claude", "worktrees", "new\nline")
	if err := os.MkdirAll(filepath.Dir(wt), 0o755); err != nil {
		t.Fatal(err)
	}
	add := exec.Command("git", "worktree", "add", "-q", wt, "-b", "wt-newline")
	add.Dir = dir
	if out, err := add.CombinedOutput(); err != nil {
		t.Skipf("platform/Git refuses a newline in a worktree path: %s: %v", out, err)
	}
	t.Cleanup(func() {
		rm := exec.Command("git", "worktree", "remove", "--force", wt)
		rm.Dir = dir
		_ = rm.Run()
		prune := exec.Command("git", "worktree", "prune")
		prune.Dir = dir
		_ = prune.Run()
	})
	strictGit(t, dir, "-c", "advice.addEmbeddedRepo=false",
		"--literal-pathspecs", "add", "--", ".claude/worktrees/new\nline")

	patch := strictGitOut(t, dir, "diff", "--cached", "HEAD")
	if !strings.Contains(patch, "160000") {
		t.Fatalf("fixture did not produce a gitlink entry:\n%q", patch)
	}

	got, err := FilesInPatchStrict(patch)
	if err != nil {
		t.Fatalf("FilesInPatchStrict: %v\npatch:\n%q", err, patch)
	}
	if len(got) != 1 || got[0] != ".claude/worktrees/new\nline" {
		t.Fatalf("got %q, want exactly [.claude/worktrees/new\\nline]", got)
	}
	if len(FilesInPatch(patch)) != 0 {
		t.Logf("note: fail-soft scanner resolved %q for this shape", FilesInPatch(patch))
	}
}

func TestFilesInPatchStrictRefusesMalformedHeaders(t *testing.T) {
	cases := []struct {
		name  string
		patch string
	}{
		{"unterminated quote", "diff --git \"a/foo.txt b/foo.txt\nindex 1..2 100644\n"},
		{"single quoted operand", "diff --git \"a/foo.txt\"\nindex 1..2 100644\n"},
		{"three quoted operands", "diff --git \"a/x\" \"b/x\" \"c/x\"\nindex 1..2 100644\n"},
		{"bad escape", "diff --git \"a/o\\qops\" \"b/o\\qops\"\nindex 1..2 100644\n"},
		{"empty operand", "diff --git \nindex 1..2 100644\n"},
		{"unresolvable rename without corroboration", "diff --git a/old name b/new name\nindex 1..2 100644\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FilesInPatchStrict(tc.patch)
			if err == nil {
				t.Fatalf("expected refusal, got %q", got)
			}
			if got != nil {
				t.Errorf("a refusal must not return a partial scope: %q", got)
			}
		})
	}
}

// An empty patch is not malformed — it legitimately touches nothing.
func TestFilesInPatchStrictEmptyPatch(t *testing.T) {
	got, err := FilesInPatchStrict("")
	if err != nil {
		t.Fatalf("empty patch must not be an error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no paths, got %q", got)
	}
}

// For every input the fail-soft scanner CAN parse, the strict parser
// must agree — the strict API is a superset, not a different contract.
func TestFilesInPatchStrictAgreesWithFailSoftOnPlainPatches(t *testing.T) {
	dir := exoticDiffFixture(t)
	if err := os.WriteFile(filepath.Join(dir, "plain.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\nv2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	strictGit(t, dir, "add", "-A")
	patch := strictGitOut(t, dir, "diff", "--cached", "HEAD")

	soft := FilesInPatch(patch)
	strict, err := FilesInPatchStrict(patch)
	if err != nil {
		t.Fatalf("FilesInPatchStrict: %v", err)
	}
	if strings.Join(soft, "\x00") != strings.Join(strict, "\x00") {
		t.Errorf("strict and fail-soft disagree on a plain patch:\n soft  =%q\n strict=%q", soft, strict)
	}
}
