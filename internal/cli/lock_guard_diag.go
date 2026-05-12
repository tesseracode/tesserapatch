package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

// printEmptyLockWarning formats the warning for an upstream.lock that
// exists but has no recorded remote/branch/commit triple — typically
// the scaffolded lock left by `tpatch init`. Reconcile continues.
func printEmptyLockWarning(w io.Writer) {
	fmt.Fprintln(w, "warning: .tpatch/upstream.lock is empty (no baseline recorded).")
	fmt.Fprintln(w, "  Reconcile verdicts cannot be cross-checked against a recorded baseline.")
	fmt.Fprintln(w, "  Run `tpatch record --auto` after applying a feature to populate the lock.")
}

// printMissingLockWarning formats the warning for an absent
// upstream.lock. Reconcile continues — older repositories never
// produced one.
func printMissingLockWarning(w io.Writer) {
	fmt.Fprintln(w, "warning: .tpatch/upstream.lock not found.")
	fmt.Fprintln(w, "  No baseline is recorded; reconcile verdicts will be computed without cross-checks.")
	fmt.Fprintln(w, "  Run `tpatch record --auto` to start tracking baselines.")
}

// printSkippedLockNote formats the informational note shown when the
// operator passed `--upstream-ref` and the override resolves to a ref
// different from the one recorded in the lock. PRD §3.1.
func printSkippedLockNote(w io.Writer, d gitutil.LockDiagnostic) {
	fmt.Fprintf(w, "note: --upstream-ref %s does not match locked ref %s; skipping lock validation.\n",
		d.OverrideRef, d.LockRefName)
}

// printStaleLockOverrideWarning formats the warning shown when the
// operator passes `--allow-stale-lock` and proceeds despite a stale
// lock. PRD §3.2.
func printStaleLockOverrideWarning(w io.Writer, d gitutil.LockDiagnostic) {
	fmt.Fprintln(w, "warning: proceeding past a stale upstream.lock (--allow-stale-lock set).")
	fmt.Fprintln(w, staleLockReasonLine(d))
	fmt.Fprintln(w, "  Reconcile verdicts may compare against a baseline that no longer exists in upstream.")
}

// formatStaleLockRefusal renders the refusal block printed when the
// upstream lock is stale and the operator did not pass
// `--allow-stale-lock`. The shape and field order are part of the
// public CLI contract — PRD §3.2.
func formatStaleLockRefusal(d gitutil.LockDiagnostic) string {
	var b strings.Builder
	b.WriteString("error: reconcile refused — .tpatch/upstream.lock is stale.\n")
	fmt.Fprintf(&b, "  recorded commit:    %s\n", d.LockCommit)
	fmt.Fprintf(&b, "  recorded branch:    %s\n", d.LockRefName)
	if d.HeadSHA != "" {
		fmt.Fprintf(&b, "  current ref HEAD:   %s\n", d.HeadSHA)
	} else {
		fmt.Fprintln(&b, "  current ref HEAD:   (could not resolve)")
	}
	b.WriteString(staleLockReasonLine(d) + "\n")
	b.WriteString("\n")
	b.WriteString("  remediation:\n")
	b.WriteString("    1. `git fetch <remote>` to refresh the remote-tracking branch.\n")
	b.WriteString("    2. Re-run `tpatch record --auto` against an applied feature to refresh the lock.\n")
	b.WriteString("    3. Or, if you know the baseline drift is intentional, pass\n")
	b.WriteString("       `--allow-stale-lock` to bypass this guard.\n")
	return b.String()
}

func staleLockReasonLine(d gitutil.LockDiagnostic) string {
	switch d.SubCause {
	case gitutil.StaleSubCauseCommit:
		return fmt.Sprintf("  reason:             %s — recorded commit is not an ancestor of %s HEAD",
			d.SubCause, d.LockRefName)
	case gitutil.StaleSubCauseResolve:
		return fmt.Sprintf("  reason:             %s — locked ref %s does not resolve locally (try `git fetch`)",
			d.SubCause, d.LockRefName)
	case gitutil.StaleSubCauseRef:
		return fmt.Sprintf("  reason:             %s — recorded commit %s is no longer present locally",
			d.SubCause, d.LockCommit)
	default:
		return "  reason:             (stale — sub-cause not classified)"
	}
}
