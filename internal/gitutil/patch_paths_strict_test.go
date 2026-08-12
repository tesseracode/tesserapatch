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

// ─── GH #7 rev-4 F1: grammar hardening ──────────────────────────────

// Non-blank input with no `diff --git` header used to yield an empty,
// error-free scope — the same "empty means everything" hole reached
// through a truncated artifact or a plain `diff -u` patch.
func TestFilesInPatchStrictRefusesHeaderlessNonBlankPatch(t *testing.T) {
	cases := []struct {
		name  string
		patch string
	}{
		{
			"plain diff -u output",
			"--- a/foo.txt\t2026-01-01\n+++ b/foo.txt\t2026-01-02\n@@ -1 +1 @@\n-a\n+b\n",
		},
		{
			"truncated artifact",
			"index 1111111..2222222 100644\n--- a/foo.txt\n+++ b/foo.txt\n",
		},
		{"prose", "this is not a patch at all\n"},
		{"lone hunk", "@@ -1 +1 @@\n-a\n+b\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FilesInPatchStrict(tc.patch)
			if err == nil {
				t.Fatalf("expected refusal, got %q", got)
			}
			if got != nil {
				t.Errorf("a refusal must return a nil scope, got %q", got)
			}
			if !strings.Contains(err.Error(), "no `diff --git` header") {
				t.Errorf("refusal should name the missing header: %v", err)
			}
		})
	}
}

// Whitespace-only input is the control: it legitimately touches
// nothing and must NOT be an error.
func TestFilesInPatchStrictWhitespaceOnlyIsNotAnError(t *testing.T) {
	for _, blank := range []string{"", "\n", "   \n\t\n", "\r\n"} {
		got, err := FilesInPatchStrict(blank)
		if err != nil {
			t.Errorf("blank input %q must not be an error: %v", blank, err)
		}
		if len(got) != 0 {
			t.Errorf("blank input %q produced %q", blank, got)
		}
	}
}

// The a-side must be validated too: a malformed a-side with a
// perfectly good b-side used to pass.
func TestFilesInPatchStrictValidatesASide(t *testing.T) {
	cases := []struct {
		name  string
		patch string
	}{
		{"a-side bad escape", "diff --git \"a/o\\qops.txt\" \"b/fine.txt\"\nindex 1..2 100644\n"},
		{"a-side unterminated", "diff --git \"a/oops.txt \"b/fine.txt\"\nindex 1..2 100644\n"},
		{"a-side wrong prefix", "diff --git \"x/oops.txt\" \"b/fine.txt\"\nindex 1..2 100644\n"},
		{"a-side empty payload", "diff --git \"a/\" \"b/fine.txt\"\nindex 1..2 100644\n"},
		{"b-side wrong prefix", "diff --git \"a/fine.txt\" \"z/oops.txt\"\nindex 1..2 100644\n"},
		{"b-side empty payload", "diff --git \"a/fine.txt\" \"b/\"\nindex 1..2 100644\n"},
		{"unquoted a-side without a/ prefix", "diff --git x/foo.txt b/foo.txt\nindex 1..2 100644\n"},
		{"unquoted header without a b-side", "diff --git a/foo.txt\nindex 1..2 100644\n"},
		{"trailing third operand", "diff --git \"a/x.txt\" \"b/x.txt\" \"c/x.txt\"\nindex 1..2 100644\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FilesInPatchStrict(tc.patch)
			if err == nil {
				t.Fatalf("expected refusal, got %q", got)
			}
			if got != nil {
				t.Errorf("a refusal must return a nil scope, got %q", got)
			}
		})
	}
}

// Go-literal escapes Git never emits must be refused on EITHER side,
// even though strconv.Unquote would happily decode them.
func TestFilesInPatchStrictRefusesGoOnlyEscapes(t *testing.T) {
	forbidden := []string{
		`\x41`,
		`\u0041`,
		`\U0001F600`,
		`\'`,
		`\e`,
		`\3`,
		`\30`,
		`\400`,
	}
	for _, esc := range forbidden {
		for _, side := range []string{"a", "b"} {
			t.Run(esc+"/"+side, func(t *testing.T) {
				aPath, bPath := `a/ok.txt`, `b/ok.txt`
				if side == "a" {
					aPath = `a/x` + esc + `y.txt`
				} else {
					bPath = `b/x` + esc + `y.txt`
				}
				patch := "diff --git \"" + aPath + "\" \"" + bPath + "\"\nindex 1..2 100644\n"
				got, err := FilesInPatchStrict(patch)
				if err == nil {
					t.Fatalf("escape %q on the %s-side must be refused, got %q", esc, side, got)
				}
				if got != nil {
					t.Errorf("a refusal must return a nil scope, got %q", got)
				}
			})
		}
	}
}

// The escapes Git DOES emit must decode byte-correctly.
func TestUnquoteGitCStyleAcceptsExactlyGitsEscapes(t *testing.T) {
	cases := map[string]string{
		`"a\ab"`:        "a\ab",
		`"a\bb"`:        "a\bb",
		`"a\fb"`:        "a\fb",
		`"a\nb"`:        "a\nb",
		`"a\rb"`:        "a\rb",
		`"a\tb"`:        "a\tb",
		`"a\vb"`:        "a\vb",
		`"a\"b"`:        `a"b`,
		`"a\\b"`:        `a\b`,
		`"caf\303\251"`: "café",
		`"\000end"`:     "\x00end",
		`"\377"`:        "\xff",
		`"plain"`:       "plain",
	}
	for in, want := range cases {
		got, err := unquoteGitCStyle(in)
		if err != nil {
			t.Errorf("unquoteGitCStyle(%s): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("unquoteGitCStyle(%s) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{
		`"unterminated`,
		`"trailing"x`,
		`"\q"`,
		`"\x41"`,
		`"\u0041"`,
		`"\"`,
		`"\12"`,
		`"\1"`,
		`"\800"`,
		`plain`,
		`"`,
	} {
		if got, err := unquoteGitCStyle(bad); err == nil {
			t.Errorf("unquoteGitCStyle(%s) should have failed, got %q", bad, got)
		}
	}
}

// Unquoted operands keep whitespace bytes Git permits unquoted.
func TestFilesInPatchStrictPreservesUnquotedWhitespace(t *testing.T) {
	patch := "diff --git a/dir/sp  ace.txt b/dir/sp  ace.txt\nindex 1..2 100644\n--- a/dir/sp  ace.txt\n+++ b/dir/sp  ace.txt\n@@ -1 +1 @@\n-a\n+b\n"
	got, err := FilesInPatchStrict(patch)
	if err != nil {
		t.Fatalf("FilesInPatchStrict: %v", err)
	}
	if len(got) != 1 || got[0] != "dir/sp  ace.txt" {
		t.Fatalf("got %q, want [dir/sp  ace.txt]", got)
	}
}

// A copy entry (real Git, -C) resolves from `copy to`.
func TestFilesInPatchStrictHandlesRealGitCopyEntry(t *testing.T) {
	dir := exoticDiffFixture(t)
	body, err := os.ReadFile(filepath.Join(dir, "base.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "copied to.txt"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	strictGit(t, dir, "add", "-A")
	patch := strictGitOut(t, dir, "diff", "--cached", "-C", "--find-copies-harder", "HEAD")
	got, err := FilesInPatchStrict(patch)
	if err != nil {
		t.Fatalf("FilesInPatchStrict: %v\npatch:\n%s", err, patch)
	}
	found := false
	for _, p := range got {
		if p == "copied to.txt" {
			found = true
		}
	}
	if !found {
		t.Errorf("copy entry not resolved; got %q\npatch:\n%s", got, patch)
	}
}
