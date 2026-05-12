package gitutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LockState classifies the state of `.tpatch/upstream.lock` as observed
// by the reconcile preflight. See PRD-reconcile-lock-guard §3.1 for the
// 5-state taxonomy. LockStateUnknown is the zero value and signals that
// classification was not attempted (e.g. by the single-arg
// PreflightReconcile entry point).
type LockState int

const (
	LockStateUnknown LockState = iota
	LockStateValid
	LockStateEmpty
	LockStateMissing
	LockStateStale
	LockStateSkipped
)

func (s LockState) String() string {
	switch s {
	case LockStateValid:
		return "valid"
	case LockStateEmpty:
		return "empty"
	case LockStateMissing:
		return "missing"
	case LockStateStale:
		return "stale"
	case LockStateSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

// Stale sub-cause identifiers. PRD-reconcile-lock-guard §4 step 4.
const (
	StaleSubCauseCommit  = "STALE-COMMIT"
	StaleSubCauseResolve = "STALE-RESOLVE"
	StaleSubCauseRef     = "STALE-REF"
)

// LockDiagnostic carries the data needed to render a refusal block or a
// warning note for a non-Valid LockState. Fields are zero-valued when
// not applicable to the current state.
type LockDiagnostic struct {
	// SubCause is one of the StaleSubCause* constants when LockState
	// is Stale. Empty otherwise.
	SubCause string
	// LockCommit is the commit recorded in upstream.lock (truncated
	// shown to the user, but stored full here).
	LockCommit string
	// LockRefName is the `<remote>/<branch>` ref reconstructed from
	// the recorded lock fields.
	LockRefName string
	// HeadSHA is the SHA the lock-ref currently resolves to on disk
	// (empty when STALE-RESOLVE).
	HeadSHA string
	// OverrideRef is the symbolic ref the operator passed via
	// `--upstream-ref` (populated for LockStateSkipped).
	OverrideRef string
}

// PreflightReconcileWithOverride runs the standard reconcile preflight
// AND classifies the upstream lock against the operator-supplied
// `--upstream-ref`. See PRD-reconcile-lock-guard §3, §4, §7.3.
//
// The lock check is independent of the working-tree check: callers
// gate the two with separate override flags (`--allow-dirty` vs
// `--allow-stale-lock`). `Clean()` continues to report the working-tree
// state only; lock state is inspected via the LockState field.
func PreflightReconcileWithOverride(repoRoot, upstreamRefOverride string) (ReconcilePreflight, error) {
	p, err := PreflightReconcile(repoRoot)
	if err != nil {
		return p, err
	}
	p.LockState, p.LockDiagnostic = classifyUpstreamLock(repoRoot, upstreamRefOverride)
	return p, nil
}

// classifyUpstreamLock implements PRD-reconcile-lock-guard §4. It is
// pure-read: it never modifies the lock file or the git index. It does
// not return errors — every recoverable failure maps to a documented
// LockState (Missing / Empty / Stale).
//
// NOTE: This implements a small inline lock scanner rather than calling
// store.LoadUpstreamLock because internal/store imports internal/gitutil
// (see store/validation.go, store/dependents.go) and we cannot create
// the reverse import. The scanner mirrors store.ParseUpstreamLock for
// the four scalar keys this guard cares about. See CURRENT.md for the
// follow-up cleanup PRD that promotes the parser to a leaf package.
func classifyUpstreamLock(repoRoot, overrideRef string) (LockState, LockDiagnostic) {
	lockPath := filepath.Join(repoRoot, ".tpatch", "upstream.lock")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return LockStateMissing, LockDiagnostic{}
		}
		// Unreadable for any other reason — treat as Missing per
		// PRD §4 step 1 (degrade-to-warn, never refuse on infra
		// glitches).
		return LockStateMissing, LockDiagnostic{}
	}

	remote, branch, commit := scanUpstreamLockBytes(data)
	if remote == "" || branch == "" || commit == "" {
		return LockStateEmpty, LockDiagnostic{}
	}

	// Legacy normalization: pre-v0.8 writer stored the full ref
	// inside `branch:`. If branch begins with `<remote>/`, strip it.
	if strings.HasPrefix(branch, remote+"/") {
		branch = strings.TrimPrefix(branch, remote+"/")
	}

	lockRef := remote + "/" + branch
	diag := LockDiagnostic{
		LockCommit:  commit,
		LockRefName: lockRef,
	}

	// PRD §3.1 / §4 step 3: --upstream-ref override → Skipped when
	// the override resolves to the same full ref as the lock.
	if overrideRef != "" {
		lockFull, lerr := SymbolicFullRefName(repoRoot, lockRef)
		overFull, oerr := SymbolicFullRefName(repoRoot, overrideRef)
		if lerr == nil && oerr == nil && lockFull != "" && overFull != "" && lockFull != overFull {
			diag.OverrideRef = overrideRef
			return LockStateSkipped, diag
		}
	}

	// PRD §4 step 4: resolve the lock ref against the local git
	// remote-tracking branch. Failure to resolve → STALE-RESOLVE.
	head, err := runGit(repoRoot, "rev-parse", "--verify", lockRef+"^{commit}")
	head = strings.TrimSpace(head)
	if err != nil || head == "" {
		diag.SubCause = StaleSubCauseResolve
		return LockStateStale, diag
	}
	diag.HeadSHA = head

	// PRD §4 step 4b: lock commit must exist locally. Failure →
	// STALE-REF (typically commit was force-pushed away).
	if _, err := runGit(repoRoot, "rev-parse", "--verify", commit+"^{commit}"); err != nil {
		diag.SubCause = StaleSubCauseRef
		return LockStateStale, diag
	}

	// PRD §4 step 4c: lock commit must be an ancestor of the lock
	// ref HEAD (or equal). Otherwise STALE-COMMIT.
	if commit != head {
		// `git merge-base --is-ancestor A B` exits 0 if A is an
		// ancestor of B.
		if _, err := runGit(repoRoot, "merge-base", "--is-ancestor", commit, head); err != nil {
			diag.SubCause = StaleSubCauseCommit
			return LockStateStale, diag
		}
	}

	return LockStateValid, LockDiagnostic{}
}

// scanUpstreamLockBytes extracts the three scalar keys that classify
// upstream.lock. It mirrors store.ParseUpstreamLock for the keys we
// inspect here. URL is intentionally not surfaced.
func scanUpstreamLockBytes(data []byte) (remote, branch, commit string) {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.IndexByte(trimmed, ':')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		val := strings.TrimSpace(trimmed[idx+1:])
		if i := strings.Index(val, " #"); i >= 0 {
			val = strings.TrimSpace(val[:i])
		}
		if n := len(val); n >= 2 {
			if (val[0] == '"' && val[n-1] == '"') || (val[0] == '\'' && val[n-1] == '\'') {
				val = val[1 : n-1]
			}
		}
		switch key {
		case "remote":
			remote = val
		case "branch":
			branch = val
		case "commit":
			commit = val
		}
	}
	return
}

// SymbolicFullRefName resolves `ref` to its symbolic full form (e.g.
// `refs/remotes/upstream/main`). Returns empty string on failure. Used
// by the lock-guard to decide whether `--upstream-ref` truly diverges
// from the locked ref.
func SymbolicFullRefName(repoRoot, ref string) (string, error) {
	out, err := runGit(repoRoot, "rev-parse", "--symbolic-full-name", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// SplitUpstreamRef splits an `<remote>/<branch>` ref. It requires
// exactly one slash; refs with zero slashes (no remote) or more than
// one slash (ambiguous — could be a remote name with `/` in it, but
// tpatch doesn't support that) are rejected by returning ok=false.
//
// Used by both the lock-guard (read side) and the reconcile writer
// (write side, PRD-reconcile-lock-guard §5.3).
func SplitUpstreamRef(ref string) (remote, branch string, ok bool) {
	if ref == "" {
		return "", "", false
	}
	i := strings.IndexByte(ref, '/')
	if i <= 0 || i >= len(ref)-1 {
		return "", "", false
	}
	if strings.IndexByte(ref[i+1:], '/') >= 0 {
		return "", "", false
	}
	return ref[:i], ref[i+1:], true
}

// GitRemoteURL returns the URL configured for the named remote.
// Returns empty string and a wrapped error when the remote is unknown
// or git fails. Used by the reconcile writer to populate
// `upstream.lock` (PRD §5.3).
func GitRemoteURL(repoRoot, remote string) (string, error) {
	if remote == "" {
		return "", fmt.Errorf("empty remote name")
	}
	out, err := runGit(repoRoot, "remote", "get-url", remote)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
