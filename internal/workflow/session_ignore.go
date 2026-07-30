package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// LocalIgnoreRule is the exact `.gitignore` line `tpatch init` writes
// and every session writer must verify is effective before the first
// local buffer write. Kept in ONE place so mandates 1, 2, 3, and 5
// share the same literal string.
const LocalIgnoreRule = ".tpatch/local/"

// PrePRDWorkspaceFallbackPath is the D6 mandate 6 fallback surfaced to
// the user when the effective-ignore check refuses. The user can move
// their local buffers here (outside the tracked worktree) or run
// `tpatch init` to install the .gitignore rule and retry. The path is
// display-only — writers never auto-create it (D6 mandate 6 says
// "defined", not "materialized"); pre-PRD workspaces are valid.
const PrePRDWorkspaceFallbackPath = ".git/tpatch/capture/"

// ErrLocalIgnoreRefusal is the single sentinel every D6 refusal case
// wraps. Callers key off `errors.Is` and print the actionable message
// verbatim to the user. Each concrete refusal wraps this sentinel
// so downstream tests can assert on ONE error identity and route six
// different mandate reasons through the same refusal class.
var ErrLocalIgnoreRefusal = errors.New("refusing to write local capture buffer: .tpatch/local/ ignore contract violated")

// LocalIgnoreRefusalReason enumerates the six D6 mandate failure modes.
// Every LocalIgnoreRefusal error carries one of these — the message
// prefix uniquely identifies which mandate short-circuited so users
// know EXACTLY which of the six is unsatisfied without reading source.
type LocalIgnoreRefusalReason string

const (
	// LocalIgnoreGitUnavailable — D6 mandate 4: git is missing OR the
	// directory is not a working tree. The whole ignore contract
	// cannot be evaluated safely.
	LocalIgnoreGitUnavailable LocalIgnoreRefusalReason = "git-unavailable"
	// LocalIgnoreGitignoreUnwritable — D6 mandate 2: `.gitignore`
	// exists but cannot be edited (permission denied, read-only FS,
	// unresolvable path). The `tpatch init` amendment refuses and
	// prints the rule verbatim; session writers refuse until Git
	// reports the path ignored.
	LocalIgnoreGitignoreUnwritable LocalIgnoreRefusalReason = "gitignore-unwritable"
	// LocalIgnorePathNotIgnored — D6 mandates 3+5: the concrete
	// resolved path is NOT effectively ignored by git. This can
	// happen when `.gitignore` was edited manually with the wrong
	// line, when a nested `.gitignore` un-ignores the path with a
	// negation rule (`!`), or when the user has not run `tpatch init`.
	LocalIgnorePathNotIgnored LocalIgnoreRefusalReason = "path-not-ignored"
	// LocalIgnorePathOutsideWorktree — D6 mandate 3 corollary: the
	// concrete resolved path is not under the working tree (e.g. a
	// symlink escape). Refuses rather than write outside the tree.
	LocalIgnorePathOutsideWorktree LocalIgnoreRefusalReason = "path-outside-worktree"
	// LocalIgnoreEffectiveCheckFailed — D6 mandate 5: git could not
	// answer the check-ignore query (128 exit or similar).
	LocalIgnoreEffectiveCheckFailed LocalIgnoreRefusalReason = "effective-check-failed"
)

// LocalIgnoreRefusal wraps ErrLocalIgnoreRefusal with a specific
// mandate reason and the actionable message enumerating all six D6
// mandates. Every field is display-only after construction —
// production paths log the .Error() form directly.
type LocalIgnoreRefusal struct {
	Reason LocalIgnoreRefusalReason
	// Path is the concrete resolved local buffer path the writer
	// tried to verify. Empty when the refusal fired before any
	// specific path was chosen (mandate 4 git-unavailable).
	Path string
	// Detail is the reason-specific short message (e.g. the git
	// stderr for effective-check-failed).
	Detail string
}

// Error renders the refusal exactly as it must appear to the user.
// The rendered message MUST enumerate all six mandates so the user
// sees the complete safety contract in one place per PRD §4 D6.
func (r *LocalIgnoreRefusal) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s]", ErrLocalIgnoreRefusal.Error(), r.Reason)
	if r.Path != "" {
		fmt.Fprintf(&b, "\n  path: %s", r.Path)
	}
	if r.Detail != "" {
		fmt.Fprintf(&b, "\n  detail: %s", r.Detail)
	}
	b.WriteString("\n\nThe local buffer lane at .tpatch/local/capture/ MUST satisfy all six D6\n")
	b.WriteString("mandates from PRD-active-feature-session §4 D6 + ADR-027 D1 before any\n")
	b.WriteString("session writer runs. All six:\n")
	b.WriteString("  1. tpatch init installs an effective .gitignore rule for .tpatch/local/.\n")
	b.WriteString("  2. If .gitignore cannot be edited, refuse and print the rule.\n")
	b.WriteString("  3. session start verifies the concrete resolved path is ignored before first write.\n")
	b.WriteString("  4. Refuse when Git is unavailable OR the path is not ignored.\n")
	b.WriteString("  5. Verification uses `git check-ignore` (effective), NOT textual .gitignore matching.\n")
	b.WriteString("  6. Pre-PRD workspaces are valid: the first writer prompts/refuses until (1)-(5) hold.\n")
	b.WriteString("\nRemediation:\n")
	fmt.Fprintf(&b, "  • Run `tpatch init` to install the rule automatically.\n")
	fmt.Fprintf(&b, "  • Or append this exact line to your .gitignore:\n\n      %s\n\n", LocalIgnoreRule)
	fmt.Fprintf(&b, "  • Fallback: move local buffers to %s (outside the worktree).\n", PrePRDWorkspaceFallbackPath)
	return b.String()
}

// Unwrap lets errors.Is(err, ErrLocalIgnoreRefusal) match every refusal
// class. The single sentinel is intentional — tests, downstream doctor
// checks, and JSON error routing all key off ONE identity.
func (r *LocalIgnoreRefusal) Unwrap() error { return ErrLocalIgnoreRefusal }

// EnsureLocalIgnoreContract runs the D6 mandates 3+4+5 verification
// used at session-start time and before every local buffer write. It
// returns nil when the contract holds; otherwise a *LocalIgnoreRefusal
// wrapping ErrLocalIgnoreRefusal with the specific mandate reason.
//
// Concretely, the mandates checked here are:
//   - Mandate 4: git must be available and repoRoot must be a working
//     tree. Failure → LocalIgnoreGitUnavailable.
//   - Mandate 3 corollary: the resolved path must sit inside the
//     working tree. Failure → LocalIgnorePathOutsideWorktree.
//   - Mandate 5: the effective ignore status is queried via
//     `git check-ignore`. A "not ignored" answer → LocalIgnorePathNotIgnored.
//     A "check itself failed" answer → LocalIgnoreEffectiveCheckFailed.
//
// Mandates 1 and 2 belong to `tpatch init` (see EnsureLocalGitignoreRule)
// and mandate 6 is the fallback string constant PrePRDWorkspaceFallbackPath.
//
// v0.12.0 Wave γ rev-1 (F-EXT-γ-1 fix, PRD §4 D6 mandate 4 verbatim
// "Writers must refuse when Git is unavailable or the path is not
// ignored."): this function is also registered at package init as
// store.SessionIgnoreVerifier so EVERY caller of Store.SaveSession —
// present and future — routes through the same bottleneck check. See
// the init() below.
func EnsureLocalIgnoreContract(repoRoot, resolvedPath string) error {
	if !gitutil.IsGitAvailable(repoRoot) {
		return &LocalIgnoreRefusal{
			Reason: LocalIgnoreGitUnavailable,
			Path:   resolvedPath,
			Detail: "git executable missing or " + repoRoot + " is not a working tree",
		}
	}
	// Mandate 3 corollary: reject paths outside the worktree BEFORE
	// asking git — avoids leaking absolute paths from other trees.
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return &LocalIgnoreRefusal{
			Reason: LocalIgnorePathOutsideWorktree,
			Path:   resolvedPath,
			Detail: err.Error(),
		}
	}
	absPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		return &LocalIgnoreRefusal{
			Reason: LocalIgnorePathOutsideWorktree,
			Path:   resolvedPath,
			Detail: err.Error(),
		}
	}
	if absPath != absRoot && !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) {
		return &LocalIgnoreRefusal{
			Reason: LocalIgnorePathOutsideWorktree,
			Path:   resolvedPath,
			Detail: "resolved path is not under repository root " + absRoot,
		}
	}
	ignored, err := gitutil.IsPathIgnored(repoRoot, absPath)
	if err != nil {
		if errors.Is(err, gitutil.ErrGitUnavailable) {
			return &LocalIgnoreRefusal{
				Reason: LocalIgnoreGitUnavailable,
				Path:   resolvedPath,
				Detail: err.Error(),
			}
		}
		return &LocalIgnoreRefusal{
			Reason: LocalIgnoreEffectiveCheckFailed,
			Path:   resolvedPath,
			Detail: err.Error(),
		}
	}
	if !ignored {
		return &LocalIgnoreRefusal{
			Reason: LocalIgnorePathNotIgnored,
			Path:   resolvedPath,
			Detail: "git check-ignore reports " + absPath + " is not ignored",
		}
	}
	return nil
}

// LocalIgnoreStatus is the outcome of EnsureLocalGitignoreRuleStatus,
// exposed so callers (notably `tpatch init`) can distinguish "we
// created / amended `.gitignore`" from "the rule was already present
// and preserved". Rev-0 of `tpatch init` always printed `appended`
// regardless of whether it actually wrote anything, which lied to
// operators re-running the command. v0.12.0 Wave γ rev-1 Slice R6
// (F-INT-γ-4 LOW).
type LocalIgnoreStatus int

const (
	// LocalIgnoreAlreadyPresent — an equivalent rule line was
	// already in `.gitignore`; the file was left untouched.
	LocalIgnoreAlreadyPresent LocalIgnoreStatus = iota
	// LocalIgnoreAppended — `.gitignore` existed but was missing
	// the rule; the rule was appended.
	LocalIgnoreAppended
	// LocalIgnoreCreated — `.gitignore` did not exist and was
	// created containing the rule.
	LocalIgnoreCreated
)

// EnsureLocalGitignoreRule is the D6 mandates 1+2 half of the contract.
// Called by `tpatch init`. Behavior:
//   - If `.gitignore` at repoRoot does not exist, create it with the
//     rule (D6 mandate 1 — install the effective rule).
//   - If it exists and already contains the rule (any equivalent line),
//     do nothing.
//   - If it exists but is missing the rule, append the rule line
//     preceded by a `# tpatch` comment for traceability.
//   - If the file cannot be edited (permission denied, etc.), return a
//     LocalIgnoreRefusal(gitignore-unwritable) which prints the rule
//     verbatim per D6 mandate 2.
//
// This function does NOT run `git check-ignore` — that is mandate 5
// and belongs to EnsureLocalIgnoreContract. `tpatch init` is the
// one-shot amendment; ongoing verification is a session-start-time
// concern.
//
// Kept for backward compatibility with existing tests / callers that
// don't need the status distinction. New callers (see `tpatch init`)
// should prefer EnsureLocalGitignoreRuleStatus.
func EnsureLocalGitignoreRule(repoRoot string) error {
	_, err := EnsureLocalGitignoreRuleStatus(repoRoot)
	return err
}

// EnsureLocalGitignoreRuleStatus is the status-returning form. See
// EnsureLocalGitignoreRule for the mandate-1+2 semantics.
func EnsureLocalGitignoreRuleStatus(repoRoot string) (LocalIgnoreStatus, error) {
	gitignorePath := filepath.Join(repoRoot, ".gitignore")
	existing, err := os.ReadFile(gitignorePath)
	switch {
	case err == nil:
		// Already contains the rule?
		lines := strings.Split(string(existing), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == LocalIgnoreRule {
				return LocalIgnoreAlreadyPresent, nil
			}
			if strings.TrimSpace(line) == strings.TrimSuffix(LocalIgnoreRule, "/") {
				// A non-trailing-slash form (e.g. `.tpatch/local`) is
				// also honored by git.
				return LocalIgnoreAlreadyPresent, nil
			}
		}
		out := string(existing)
		if !strings.HasSuffix(out, "\n") && out != "" {
			out += "\n"
		}
		out += "# tpatch: keep local capture buffers out of commits (PRD-active-feature-session §4 D6, ADR-027 D1)\n"
		out += LocalIgnoreRule + "\n"
		if writeErr := os.WriteFile(gitignorePath, []byte(out), 0o644); writeErr != nil {
			return LocalIgnoreAlreadyPresent, &LocalIgnoreRefusal{
				Reason: LocalIgnoreGitignoreUnwritable,
				Path:   gitignorePath,
				Detail: writeErr.Error(),
			}
		}
		return LocalIgnoreAppended, nil
	case os.IsNotExist(err):
		content := "# tpatch: keep local capture buffers out of commits (PRD-active-feature-session §4 D6, ADR-027 D1)\n" + LocalIgnoreRule + "\n"
		if writeErr := os.WriteFile(gitignorePath, []byte(content), 0o644); writeErr != nil {
			return LocalIgnoreAlreadyPresent, &LocalIgnoreRefusal{
				Reason: LocalIgnoreGitignoreUnwritable,
				Path:   gitignorePath,
				Detail: writeErr.Error(),
			}
		}
		return LocalIgnoreCreated, nil
	default:
		return LocalIgnoreAlreadyPresent, &LocalIgnoreRefusal{
			Reason: LocalIgnoreGitignoreUnwritable,
			Path:   gitignorePath,
			Detail: err.Error(),
		}
	}
}

// StoreLocalCaptureDir is a small forwarder so cli and workflow can
// both talk about the same D5 path without needing to import store
// from workflow-test code. Kept internal.
func StoreLocalCaptureDir(s *store.Store) string { return s.LocalCaptureDir() }

// init wires EnsureLocalIgnoreContract into store as the D6 bottleneck
// verifier every session-state writer must satisfy. PRD-active-feature-
// session §4 D6 mandate 4 verbatim: "Writers must refuse when Git is
// unavailable or the path is not ignored." Plural, unqualified — every
// present + future session-state writer routes through Store.SaveSession
// which in turn calls this verifier. Rev-0 shipped a bug where the
// mandate-3 check was only wired to `session start`; every later
// writer (session stop, session summarize --promote, record
// --with-session) bypassed it. Bottleneck enforcement fixes that by
// construction.
func init() {
	store.SetSessionIgnoreVerifier(func(repoRoot, resolvedPath string) error {
		return EnsureLocalIgnoreContract(repoRoot, resolvedPath)
	})
}
