package workflow_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/workflow"
)

// initGitRepo makes `dir` a working tree with a default identity so
// `git check-ignore` can answer. Skips the enclosing test if git is
// missing (CI without git → test is not meaningful).
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH:", err)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, string(out))
		}
	}
	run("init", "-q")
	run("config", "user.email", "test@example.invalid")
	run("config", "user.name", "test")
	run("config", "commit.gpgsign", "false")
}

// mustBeRefusal asserts err is a *LocalIgnoreRefusal wrapping
// ErrLocalIgnoreRefusal with the expected reason, AND that the
// rendered message enumerates all six D6 mandates.
func mustBeRefusal(t *testing.T, err error, want workflow.LocalIgnoreRefusalReason) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected LocalIgnoreRefusal, got nil")
	}
	if !errors.Is(err, workflow.ErrLocalIgnoreRefusal) {
		t.Fatalf("expected errors.Is(err, ErrLocalIgnoreRefusal); got %v", err)
	}
	var ref *workflow.LocalIgnoreRefusal
	if !errors.As(err, &ref) {
		t.Fatalf("expected errors.As to *LocalIgnoreRefusal; got %v", err)
	}
	if ref.Reason != want {
		t.Fatalf("refusal reason %q, want %q", ref.Reason, want)
	}
	msg := err.Error()
	// The rendered message MUST enumerate all six mandates — that is
	// the entire safety-margin contract for Wave γ.
	for i := 1; i <= 6; i++ {
		if !strings.Contains(msg, fmt.Sprintf("%d.", i)) {
			t.Errorf("refusal message must enumerate mandate %d; got:\n%s", i, msg)
		}
	}
	if !strings.Contains(msg, workflow.LocalIgnoreRule) {
		t.Errorf("refusal message must print the exact rule %q; got:\n%s", workflow.LocalIgnoreRule, msg)
	}
	if !strings.Contains(msg, workflow.PrePRDWorkspaceFallbackPath) {
		t.Errorf("refusal message must name the pre-PRD workspace fallback path; got:\n%s", msg)
	}
}

// ─────────── D6 mandate 4: git unavailable ───────────

// TestD6MandateGitUnavailable_NotAWorkTree is the refusal-path fixture
// for the "user unpacked a tarball" scenario — .tpatch/ exists but the
// enclosing directory is not a git working tree. Session start MUST
// refuse with the mandate-4 error and cite all six mandates.
func TestD6MandateGitUnavailable_NotAWorkTree(t *testing.T) {
	// A brand-new tmp dir is NOT a git working tree — git rev-parse
	// --is-inside-work-tree fails.
	tmp := t.TempDir()
	// Ensure git is not going to accidentally find an ancestor repo
	// (which happens in CI when running tests inside a nested repo).
	// A `.git` file with `gitdir: /dev/null` short-circuits the ancestor
	// walk so IsGitAvailable returns false deterministically.
	if err := os.WriteFile(filepath.Join(tmp, ".git"), []byte("gitdir: /nonexistent\n"), 0o644); err != nil {
		t.Fatalf("seed .git: %v", err)
	}
	// The path we would want to write.
	target := filepath.Join(tmp, ".tpatch", "local", "capture", "some-feature", "cs_abcdef012345")
	err := workflow.EnsureLocalIgnoreContract(tmp, target)
	mustBeRefusal(t, err, workflow.LocalIgnoreGitUnavailable)
}

// ─────────── D6 mandate 5: effective ignore check ───────────

// TestD6MandateEffectiveIgnoreFails_NoGitignore reproduces a fresh git
// repo where NO .gitignore rule for .tpatch/local/ exists. Session
// start MUST refuse with LocalIgnorePathNotIgnored — the textual
// contents of .gitignore are irrelevant; only git's own effective
// answer matters (D6 mandate 5).
func TestD6MandateEffectiveIgnoreFails_NoGitignore(t *testing.T) {
	tmp := t.TempDir()
	initGitRepo(t, tmp)
	// No .gitignore at all — check-ignore returns exit 1.
	target := filepath.Join(tmp, ".tpatch", "local", "capture", "some-feature", "cs_abcdef012345")
	err := workflow.EnsureLocalIgnoreContract(tmp, target)
	mustBeRefusal(t, err, workflow.LocalIgnorePathNotIgnored)
}

// TestD6MandateEffectiveIgnoreFails_NegationRule_DefeatsTextualMatch
// is the load-bearing fixture proving mandate 5 is not just "grep the
// .gitignore file". The workspace has a `.tpatch/local/` line AND a
// nested `!.tpatch/local/capture/` negation. Textual matching would
// mistakenly conclude "the rule is present". Git's effective answer is
// "not ignored" — that is what the writer honors.
func TestD6MandateEffectiveIgnoreFails_NegationRule_DefeatsTextualMatch(t *testing.T) {
	tmp := t.TempDir()
	initGitRepo(t, tmp)
	// Textual rule present …
	if err := os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte(".tpatch/local/\n"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	// … but nested .gitignore negates it under capture/.
	nested := filepath.Join(tmp, ".tpatch")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, ".gitignore"), []byte("!local/\n"), 0o644); err != nil {
		t.Fatalf("write nested .gitignore: %v", err)
	}
	target := filepath.Join(tmp, ".tpatch", "local", "capture", "some-feature", "cs_abcdef012345")
	err := workflow.EnsureLocalIgnoreContract(tmp, target)
	mustBeRefusal(t, err, workflow.LocalIgnorePathNotIgnored)
}

// ─────────── D6 mandate 3 corollary: path outside worktree ───────────

// TestD6MandatePathOutsideWorktree covers the symlink-escape / caller-
// bug case where a path is under a totally different directory. The
// writer refuses BEFORE asking git so a malicious slug cannot leak
// paths outside the tree.
func TestD6MandatePathOutsideWorktree(t *testing.T) {
	tmp := t.TempDir()
	initGitRepo(t, tmp)
	// Even with the ignore rule present, a target OUTSIDE the tree
	// must refuse with mandate 3 corollary.
	if err := os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte(".tpatch/local/\n"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	outside := t.TempDir()
	target := filepath.Join(outside, "somewhere")
	err := workflow.EnsureLocalIgnoreContract(tmp, target)
	mustBeRefusal(t, err, workflow.LocalIgnorePathOutsideWorktree)
}

// ─────────── Happy path: contract satisfied ───────────

// TestD6ContractHappyPath verifies that when init installed the rule
// AND git reports the path ignored AND git is available AND the path
// stays inside the worktree, EnsureLocalIgnoreContract returns nil.
func TestD6ContractHappyPath(t *testing.T) {
	tmp := t.TempDir()
	initGitRepo(t, tmp)
	if err := workflow.EnsureLocalGitignoreRule(tmp); err != nil {
		t.Fatalf("install .gitignore rule: %v", err)
	}
	target := filepath.Join(tmp, ".tpatch", "local", "capture", "some-feature", "cs_abcdef012345")
	if err := workflow.EnsureLocalIgnoreContract(tmp, target); err != nil {
		t.Fatalf("happy path should hold; got: %v", err)
	}
}

// ─────────── D6 mandates 1+2: init installs / refuses ───────────

// TestD6MandateInitCreatesGitignore covers mandate 1 — no prior
// .gitignore, init creates it with the exact rule.
func TestD6MandateInitCreatesGitignore(t *testing.T) {
	tmp := t.TempDir()
	if err := workflow.EnsureLocalGitignoreRule(tmp); err != nil {
		t.Fatalf("install: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(data), workflow.LocalIgnoreRule) {
		t.Fatalf("created .gitignore missing rule %q; got:\n%s", workflow.LocalIgnoreRule, string(data))
	}
}

// TestD6MandateInitAppendsWithoutDuplication covers mandate 1's
// re-run behavior — a pre-existing .gitignore with the rule is left
// untouched.
func TestD6MandateInitAppendsWithoutDuplication(t *testing.T) {
	tmp := t.TempDir()
	seed := "*.log\n" + workflow.LocalIgnoreRule + "\nbuild/\n"
	if err := os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte(seed), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := workflow.EnsureLocalGitignoreRule(tmp); err != nil {
		t.Fatalf("install: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Count(string(data), workflow.LocalIgnoreRule) != 1 {
		t.Fatalf("rule must appear exactly once; got:\n%s", string(data))
	}
}

// TestD6MandateInitAppendsWhenMissing covers mandate 1 — a pre-existing
// .gitignore without the rule gets the rule appended.
func TestD6MandateInitAppendsWhenMissing(t *testing.T) {
	tmp := t.TempDir()
	seed := "*.log\nbuild/\n"
	if err := os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte(seed), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := workflow.EnsureLocalGitignoreRule(tmp); err != nil {
		t.Fatalf("install: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), workflow.LocalIgnoreRule) {
		t.Fatalf("rule not appended; got:\n%s", string(data))
	}
}

// TestD6MandateInitRefusesUnwritableGitignore covers mandate 2 — when
// .gitignore cannot be written, init refuses with the exact rule
// printed and the six mandates enumerated.
func TestD6MandateInitRefusesUnwritableGitignore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based unwritable check is POSIX-only")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses read-only file mode")
	}
	tmp := t.TempDir()
	gp := filepath.Join(tmp, ".gitignore")
	if err := os.WriteFile(gp, []byte("*.log\n"), 0o444); err != nil {
		t.Fatalf("seed unwritable: %v", err)
	}
	// Make the parent dir writable but the file read-only so the
	// os.WriteFile attempt fails with EACCES.
	if err := os.Chmod(gp, 0o400); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	err := workflow.EnsureLocalGitignoreRule(tmp)
	mustBeRefusal(t, err, workflow.LocalIgnoreGitignoreUnwritable)
}
