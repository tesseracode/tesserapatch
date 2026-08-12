package workflow

// The ONE floor-validated offline Git gateway for `tpatch verify`
// (v0.15.1 Wave C rev-1, adjudication finding 1).
//
// Rev-0 routed the landed contract's own commands through
// `GIT_NO_LAZY_FETCH=1` but left three classes of call outside the gate:
// the legacy `gitutil.IsAncestor` used by V5 and by
// `store.ValidateDependencies` (V4), and the shadow worktree
// create/prune commands. Those bypassed both the offline discipline and
// the below-floor hard stop, so a git older than 2.36 still issued
// object commands and a promisor remote could still be reached.
//
// Every Git operation the verify path performs now goes through a method
// on `verifyRunContext`. Each method:
//
//  1. calls `gitGate()` first — below the 2.36 floor it returns
//     `errGitBelowFloor` WITHOUT spawning anything, so the only command a
//     below-floor run ever issues is the `git --version` probe itself;
//  2. runs against `ctx.root` with `GIT_NO_LAZY_FETCH=1` applied.
//
// There is no legacy bypass left in the verify path: `grep` for
// `gitutil.` in internal/workflow/verify*.go returns only pure helpers
// (record parsing, hex validation, patch path extraction) and the
// gateway below.

import (
	"errors"
	"fmt"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

// errGitBelowFloor is returned by every gateway method when the
// repository's git is older than the D17 floor. It is never converted
// into a "no result" answer: callers surface it as `unavailable`.
var errGitBelowFloor = errors.New("git is below the required floor 2.36; no object command may be issued")

// offlineEnv is the mandatory environment for every verify Git call:
// offline by construction, and C-locale so diagnostics are classifiable
// (rev-3). `internal/gitutil` applies both to the commands it issues
// itself; this is the copy handed to helpers that build their own
// command, such as the shadow worktree operations.
func (ctx *verifyRunContext) offlineEnv() []string {
	return []string{gitutil.NoLazyFetchEnv, gitutil.CLocaleEnv}
}

// gitGate is the single admission check. Every gateway method calls it
// BEFORE spawning git.
func (ctx *verifyRunContext) gitGate() error {
	if ctx.versionErr != nil {
		return fmt.Errorf("git --version could not be read: %w", ctx.versionErr)
	}
	if !ctx.floorOK {
		return fmt.Errorf("%w (found %s)", errGitBelowFloor, ctx.version.String())
	}
	return nil
}

// GitGateSatisfied reports whether object commands are permitted. Used
// by the static checks that own a Git operation of their own (V4/V5).
func (ctx *verifyRunContext) GitGateSatisfied() bool { return ctx.gitGate() == nil }

// ── Ancestry ─────────────────────────────────────────────────────────────

// isAncestorChecked is the gateway form of `git merge-base --is-ancestor`.
// It returns the answer AND the error, so callers can distinguish "not
// reachable" from "could not be determined" (rev-1 finding 3).
func (ctx *verifyRunContext) isAncestorChecked(ancestor, descendant string) (bool, error) {
	if err := ctx.gitGate(); err != nil {
		return false, err
	}
	key := ancestor + "\x00" + descendant
	if v, ok := ctx.ancestorMemo[key]; ok {
		return v, nil
	}
	v, err := gitutil.IsAncestorOffline(ctx.root, ancestor, descendant)
	if err != nil {
		return false, err
	}
	ctx.ancestorMemo[key] = v
	return v, nil
}

// isAncestor is the advisory form: an unanswerable query is reported as
// "not reachable", which only ever raises a warn-severity advisory.
func (ctx *verifyRunContext) isAncestor(ancestor, descendant string) bool {
	v, err := ctx.isAncestorChecked(ancestor, descendant)
	if err != nil {
		return false
	}
	return v
}

// ── Object reads ─────────────────────────────────────────────────────────

// blobAtTree is the gateway form of `git cat-file blob <tree>:<path>`.
func (ctx *verifyRunContext) blobAtTree(treeish, path string) (data []byte, found bool, err error) {
	if gErr := ctx.gitGate(); gErr != nil {
		return nil, false, gErr
	}
	data, found, _, err = gitutil.BlobAtTree(ctx.root, treeish, path)
	return data, found, err
}

// ── Temp index ───────────────────────────────────────────────────────────

// newTempIndex is the gateway form of the isolated-index allocator.
func (ctx *verifyRunContext) newTempIndex() (*gitutil.TempIndex, error) {
	if err := ctx.gitGate(); err != nil {
		return nil, err
	}
	return gitutil.NewTempIndex(ctx.root, ctx.tempIndexDir())
}

// ── Shadow worktrees ─────────────────────────────────────────────────────

// createShadow is the gateway form of `git worktree add`. Rev-0 called
// `gitutil.CreateShadow`, which inherited the ambient environment and
// could therefore lazy-fetch from a promisor remote.
func (ctx *verifyRunContext) createShadow(slug, commitish string) (string, error) {
	if err := ctx.gitGate(); err != nil {
		return "", err
	}
	return gitutil.CreateShadowEnv(ctx.root, slug, commitish, ctx.offlineEnv())
}

// pruneShadow is the gateway form of `git worktree remove` + `prune`.
// It is safe to call below the floor: no shadow can exist in that case,
// so the gate short-circuits without spawning git.
func (ctx *verifyRunContext) pruneShadow(slug string) error {
	if err := ctx.gitGate(); err != nil {
		return nil
	}
	return gitutil.PruneShadowEnv(ctx.root, slug, ctx.offlineEnv())
}

// runShadowGit runs a git command INSIDE a shadow worktree under the
// offline discipline (`read-tree --reset -u`, `add -A -f`, `clean -fdx`,
// `write-tree`, `apply --check`).
func (ctx *verifyRunContext) runShadowGit(shadowPath string, args ...string) (string, string, error) {
	if err := ctx.gitGate(); err != nil {
		return "", "", err
	}
	return gitutil.RunOfflineGitIn(shadowPath, args...)
}

// ── Enumeration + preflight (performed once, in newVerifyRunContext) ─────

// shadowApplyCheck is the gateway form of `git apply --check <patch>`
// INSIDE a shadow worktree, with the exit code retained so the caller
// can tell "the patch does not apply" (exit 1 — the answer V8 wants)
// from "git could not run the check" (rev-2 adjudication finding 3).
func (ctx *verifyRunContext) shadowApplyCheck(shadowPath, patchPath string) gitutil.OfflineGitResult {
	if err := ctx.gitGate(); err != nil {
		return gitutil.OfflineGitResult{ExitCode: -1, Err: err}
	}
	return gitutil.RunOfflineGitInResult(shadowPath, "apply", "--check", patchPath)
}

// normalizedIdentity is the gateway form of the D18 identity diff.
func (ctx *verifyRunContext) normalizedIdentity(commit string, paths []string) (string, error) {
	if err := ctx.gitGate(); err != nil {
		return "", err
	}
	return gitutil.NormalizedChangeIdentity(ctx.root, commit, paths)
}
