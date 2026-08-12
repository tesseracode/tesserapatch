// CLI-side guards for registered nested linked Git worktrees (GH #7).
//
// The classification itself lives in internal/gitutil; this file holds
// the CLI-layer consequences of it:
//
//   - captureDiffStatFailClosed — diffstat capture that surfaces the
//     fail-closed discovery class and swallows nothing else, used to
//     hoist every discovery-dependent read above the first artifact
//     write (GH #7 rev-2 F4);
//   - nestedWorktreeScopedPathspecs / nestedWorktreeEmptyCaptureRefusal
//     — the actionable refusal for a scoped capture whose pathspecs
//     were filtered away by the guard.

package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

// captureDiffStatFailClosed wraps CaptureDiffStatScoped with the
// historical error tolerance of its call sites: a transient Git
// failure degrades to an empty diffstat (the artifact is then skipped),
// but a nested-worktree discovery failure is returned so the caller
// refuses.
//
// GH #7 rev-2 (F4): callers MUST invoke this before their first
// artifact write. Previously `record` wrote post-apply.patch and the
// numbered snapshot, and `apply --mode done` wrote both plus the patch
// generation, before the diffstat's discovery ran — so a discovery
// failure at that point reported an error while leaving mutated
// artifacts behind.
func captureDiffStatFailClosed(repoRoot string, pathspecs []string) (string, error) {
	diffStat, err := gitutil.CaptureDiffStatScoped(repoRoot, pathspecs)
	if err != nil {
		if errors.Is(err, gitutil.ErrNestedWorktreeDiscovery) {
			return "", err
		}
		return "", nil
	}
	return diffStat, nil
}

// nestedWorktreeScopedPathspecs partitions the caller's pathspecs into
// the ones that name (or sit inside) a registered nested linked Git
// worktree and the ones that do not. Both slices preserve input order.
//
// Pathspecs may carry git magic (`:(exclude)…`, `:/…`) or a trailing
// slash; only the plain path portion is classified, and a pathspec
// tpatch cannot interpret as a plain path is conservatively treated as
// "not a worktree" so the caller never invents a misleading refusal.
func nestedWorktreeScopedPathspecs(repoRoot string, pathspecs []string) (worktreeSpecs, otherSpecs []string, err error) {
	if len(pathspecs) == 0 {
		return nil, nil, nil
	}
	prefixes, err := gitutil.NestedWorktreePrefixes(repoRoot)
	if err != nil {
		return nil, nil, err
	}
	for _, spec := range pathspecs {
		if len(prefixes) > 0 && !strings.HasPrefix(spec, ":") &&
			gitutil.PathUnderNestedWorktree(spec, prefixes) {
			worktreeSpecs = append(worktreeSpecs, spec)
			continue
		}
		otherSpecs = append(otherSpecs, spec)
	}
	return worktreeSpecs, otherSpecs, nil
}

// nestedWorktreeEmptyCaptureRefusal produces the actionable refusal for
// an empty scoped capture whose pathspecs named registered nested
// linked worktrees (GH #7, rev-0 user-external note).
//
// Without it, `record --files '.claude/worktrees/agent review'` fell
// through to the generic zero-byte diagnostic, which speculates about
// "mode-only or binary changes" — actively misleading, because the
// paths were deliberately filtered out by the nested-worktree guard.
//
// Returns nil when no requested pathspec is a nested worktree, so
// genuinely empty non-worktree captures keep their existing
// diagnostics byte-for-byte.
func nestedWorktreeEmptyCaptureRefusal(w io.Writer, repoRoot, slug string, pathspecs []string) error {
	worktreeSpecs, otherSpecs, err := nestedWorktreeScopedPathspecs(repoRoot, pathspecs)
	if err != nil {
		return err
	}
	if len(worktreeSpecs) == 0 {
		return nil
	}

	noun := "path"
	if len(worktreeSpecs) > 1 {
		noun = "paths"
	}
	if len(otherSpecs) == 0 {
		fmt.Fprintf(w, "tpatch record captured 0 bytes: every requested %s is a registered nested Git worktree and %s intentionally excluded from capture.\n",
			noun, plural(len(worktreeSpecs), "is", "are"))
	} else {
		fmt.Fprintf(w, "tpatch record captured 0 bytes. %d requested %s %s a registered nested Git worktree and %s intentionally excluded from capture; the remaining requested path(s) produced no textual diff.\n",
			len(worktreeSpecs), noun, plural(len(worktreeSpecs), "is", "are"), plural(len(worktreeSpecs), "was", "were"))
	}
	fmt.Fprintln(w, "  Excluded as nested linked worktrees:")
	for _, p := range worktreeSpecs {
		fmt.Fprintf(w, "    - %s\n", p)
	}
	if len(otherSpecs) > 0 {
		fmt.Fprintln(w, "  Requested and not excluded (no diff):")
		for _, p := range otherSpecs {
			fmt.Fprintf(w, "    - %s\n", p)
		}
	}
	fmt.Fprintln(w, "  A nested linked worktree is another checkout's working directory, not feature content:")
	fmt.Fprintln(w, "  capturing it would record a mode-160000 gitlink and pull unrelated state into the patch.")
	fmt.Fprintln(w, "  Resolve with one of:")
	fmt.Fprintf(w, "    - name the files you actually changed: tpatch record %s --files <path>[,<path>...]\n", slug)
	fmt.Fprintln(w, "    - to capture work done INSIDE that worktree, run tpatch with --path set to the worktree root")
	fmt.Fprintln(w, "    - if the directory should not be a worktree at all: git worktree remove <path>")
	return fmt.Errorf("empty capture: requested path(s) are registered nested Git worktrees — see diagnostic above")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
