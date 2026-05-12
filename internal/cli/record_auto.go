package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// autoBaseSource labels where the resolved baseline came from. Used in
// the user-facing decision line and in record.md provenance.
type autoBaseSource string

const (
	srcUpstreamLock          autoBaseSource = "upstream.lock"
	srcMergeBaseFromLock     autoBaseSource = "merge-base(toRef, upstream.lock)"
	srcUpstreamLockRemote    autoBaseSource = "upstream.lock(remote/branch)"
	srcMergeBaseLockRemote   autoBaseSource = "merge-base(toRef, upstream.lock remote/branch)"
	srcDefaultRemoteHead     autoBaseSource = "default-remote-head"
	srcMergeBaseDefaultHead  autoBaseSource = "merge-base(toRef, default-remote-head)"
	srcConventionalRef       autoBaseSource = "conventional-upstream-ref"
	srcMergeBaseConventional autoBaseSource = "merge-base(toRef, conventional-upstream-ref)"
)

// autoBaseResolution captures the outcome of record --auto inference.
type autoBaseResolution struct {
	Base         string         // resolved commit SHA (lower bound)
	BaseShort    string         // 8-char abbreviation for messages
	ToRef        string         // resolved upper bound (commit SHA)
	ToLabel      string         // original symbol used for upper bound (e.g. "HEAD")
	Source       autoBaseSource // provenance label
	SourceRef    string         // human-friendly source ref (e.g. "origin/main")
	AheadCount   int            // commits in <base>..<toRef>
	FromFallback bool           // true when Base came from merge-base fallback
}

// resolveAutoBase implements PRD §3.2.
//
// `toRef` is taken verbatim from the operator (defaults to "HEAD"). The
// function returns a fully-populated resolution or an error already
// formatted for the user (no further wrapping needed by callers).
//
// `errOut` receives one-line warnings (e.g. when the lock is populated
// but unusable and we fall back to discovery). May be nil.
func resolveAutoBase(s *store.Store, toRef string, errOut io.Writer) (*autoBaseResolution, error) {
	repo := s.Root
	if toRef == "" {
		toRef = "HEAD"
	}
	toCommit, err := gitutil.ResolveRef(repo, toRef)
	if err != nil || toCommit == "" {
		return nil, fmt.Errorf("cannot resolve --to ref %q: %v", toRef, err)
	}

	// Step 1: read upstream.lock.
	lock, lockErr := store.LoadUpstreamLock(s)
	haveLock := lockErr == nil
	if lockErr != nil && !errors.Is(lockErr, fs.ErrNotExist) {
		// Unexpected I/O error — surface to the user.
		return nil, fmt.Errorf("cannot read .tpatch/upstream.lock: %v", lockErr)
	}

	// Track why the lock did not yield a resolution. If non-empty
	// after exhausting steps 2-4 we fall back to discovery (PRD
	// §3.2 step 5 says lock that is "empty or unusable" → discover)
	// rather than hard-refusing.
	var lockReason string

	// Step 2 / 3: lock.Commit present.
	if haveLock && strings.TrimSpace(lock.Commit) != "" {
		lockCommit, rerr := gitutil.ResolveRef(repo, lock.Commit)
		if rerr != nil || lockCommit == "" {
			lockReason = fmt.Sprintf("lock.commit %q does not resolve in this repo", lock.Commit)
		} else {
			anc, _ := gitutil.IsAncestor(repo, lockCommit, toCommit)
			if anc {
				return buildResolution(repo, lockCommit, toCommit, toRef,
					srcUpstreamLock, "upstream.lock commit "+abbrev(lockCommit), false)
			}
			// Merge-base fallback against the lock commit.
			mb, mberr := gitutil.MergeBase(repo, toCommit, lockCommit)
			if mberr == nil && mb != "" && mb != toCommit {
				return buildResolutionWithGate(repo, mb, toCommit, toRef,
					srcMergeBaseFromLock, "upstream.lock commit "+abbrev(lockCommit))
			}
			lockReason = fmt.Sprintf("lock.commit %s is not reachable from %s", abbrev(lockCommit), toRef)
		}
	}

	// Step 4: lock has remote/branch.
	if haveLock && lock.Remote != "" && lock.Branch != "" {
		ref := lock.Remote + "/" + lock.Branch
		commit, rerr := gitutil.ResolveRef(repo, ref)
		if rerr != nil || commit == "" {
			if lockReason == "" {
				lockReason = fmt.Sprintf("lock ref %s does not exist locally", ref)
			}
		} else {
			anc, _ := gitutil.IsAncestor(repo, commit, toCommit)
			if anc {
				return buildResolution(repo, commit, toCommit, toRef,
					srcUpstreamLockRemote, ref, false)
			}
			mb, mberr := gitutil.MergeBase(repo, toCommit, commit)
			if mberr == nil && mb != "" && mb != toCommit {
				return buildResolutionWithGate(repo, mb, toCommit, toRef,
					srcMergeBaseLockRemote, ref)
			}
			if lockReason == "" {
				lockReason = fmt.Sprintf("lock ref %s is not reachable from %s", ref, toRef)
			}
		}
	}

	// Step 5: discover a default upstream candidate. PRD §3.2 step 5
	// says we discover when the lock is "empty or unusable" — that
	// includes the case where the lock has fields but none of them
	// resolved to a usable base above (lockReason != "").
	if !haveLock || lockReason != "" || (strings.TrimSpace(lock.Commit) == "" &&
		(lock.Remote == "" || lock.Branch == "")) {
		if haveLock && lockReason != "" && errOut != nil {
			fmt.Fprintf(errOut,
				"record --auto: upstream.lock unusable (%s); falling back to discovery\n",
				lockReason)
		}
		res, derr := resolveAutoBaseDiscovery(repo, toCommit, toRef)
		if derr == nil {
			return res, nil
		}
		// Discovery failed too. If the lock was populated, give the
		// historical "populated but no field resolves" diagnostic so
		// operators understand both inputs were exhausted; otherwise
		// surface the discovery error verbatim.
		if haveLock && lockReason != "" {
			return nil, autoBaseRefuse(
				fmt.Sprintf("record --auto: .tpatch/upstream.lock is populated but no field resolves to a commit reachable from %s, and no upstream candidate could be discovered (%s).",
					toRef, lockReason),
				toRef)
		}
		return nil, derr
	}

	// Step 7: no candidate resolved.
	return nil, autoBaseRefuse(
		fmt.Sprintf("record --auto: .tpatch/upstream.lock is populated but no field resolves to a commit reachable from %s.", toRef),
		toRef)
}

// resolveAutoBaseDiscovery handles step 5 of the algorithm: discovering
// a default upstream candidate when the lock is empty or unusable.
func resolveAutoBaseDiscovery(repo, toCommit, toRef string) (*autoBaseResolution, error) {
	// Priority A: refs/remotes/upstream/HEAD
	if ref, ok := resolveSymbolicHead(repo, "upstream"); ok {
		return tryAutoCandidate(repo, ref, toCommit, toRef,
			srcDefaultRemoteHead, srcMergeBaseDefaultHead)
	}
	// Priority B: refs/remotes/origin/HEAD
	if ref, ok := resolveSymbolicHead(repo, "origin"); ok {
		return tryAutoCandidate(repo, ref, toCommit, toRef,
			srcDefaultRemoteHead, srcMergeBaseDefaultHead)
	}
	// Priority C: conventional refs — only when exactly one resolves.
	candidates := []string{
		"upstream/main", "upstream/master",
		"origin/main", "origin/master",
	}
	var resolved []string
	for _, c := range candidates {
		if commit, err := gitutil.ResolveRef(repo, c); err == nil && commit != "" {
			resolved = append(resolved, c)
		}
	}
	if len(resolved) == 0 {
		return nil, autoBaseRefuse(
			"record --auto: no upstream candidate could be discovered (.tpatch/upstream.lock is empty, and neither `upstream/*` nor `origin/*` HEAD symrefs resolve).",
			toRef)
	}
	if len(resolved) > 1 {
		var b strings.Builder
		b.WriteString("record --auto: multiple upstream candidates resolve and .tpatch/upstream.lock is empty.\n")
		b.WriteString("  Candidates:\n")
		for _, r := range resolved {
			b.WriteString("    ")
			b.WriteString(r)
			b.WriteString("\n")
		}
		b.WriteString("  Pick one explicitly:\n")
		b.WriteString("    tpatch record <slug> --from <base>\n")
		b.WriteString("  Or populate .tpatch/upstream.lock and rerun:\n")
		b.WriteString("    tpatch record <slug> --auto --to ")
		b.WriteString(toRef)
		b.WriteString("\n")
		return nil, errors.New(b.String())
	}
	return tryAutoCandidate(repo, resolved[0], toCommit, toRef,
		srcConventionalRef, srcMergeBaseConventional)
}

// tryAutoCandidate attempts a single candidate ref. Direct-ancestor wins;
// otherwise merge-base fallback (subject to the safety gate). Returns
// an error if neither path succeeds.
func tryAutoCandidate(repo, ref, toCommit, toRef string,
	directSrc, mbSrc autoBaseSource) (*autoBaseResolution, error) {
	commit, err := gitutil.ResolveRef(repo, ref)
	if err != nil || commit == "" {
		return nil, autoBaseRefuse(
			fmt.Sprintf("record --auto: candidate %s cannot be resolved.", ref),
			toRef)
	}
	anc, _ := gitutil.IsAncestor(repo, commit, toCommit)
	if anc {
		return buildResolution(repo, commit, toCommit, toRef, directSrc, ref, false)
	}
	mb, mberr := gitutil.MergeBase(repo, toCommit, commit)
	if mberr != nil || mb == "" || mb == toCommit {
		return nil, autoBaseRefuse(
			fmt.Sprintf("record --auto: candidate %s is not an ancestor of %s and has no usable merge-base.", ref, toRef),
			toRef)
	}
	return buildResolutionWithGate(repo, mb, toCommit, toRef, mbSrc, ref)
}

// resolveSymbolicHead returns the branch pointed at by
// refs/remotes/<remote>/HEAD, formatted as "<remote>/<branch>". The
// boolean is false when the symref is missing or unresolvable.
func resolveSymbolicHead(repo, remote string) (string, bool) {
	target, err := gitutil.SymbolicRef(repo, "refs/remotes/"+remote+"/HEAD")
	if err != nil || target == "" {
		return "", false
	}
	// target is e.g. "refs/remotes/origin/main"
	const prefix = "refs/remotes/"
	if !strings.HasPrefix(target, prefix) {
		return "", false
	}
	return strings.TrimPrefix(target, prefix), true
}

// buildResolution constructs a resolution for a direct-ancestor outcome.
func buildResolution(repo, base, toCommit, toRef string,
	src autoBaseSource, sourceRef string, fromFallback bool) (*autoBaseResolution, error) {
	if base == toCommit {
		return nil, autoBaseRefuse(
			fmt.Sprintf("record --auto: inferred base %s equals --to %s (no commits to record).",
				abbrev(base), toRef),
			toRef)
	}
	count, err := gitutil.CommitCountInRange(repo, base, toCommit)
	if err != nil {
		return nil, fmt.Errorf("record --auto: cannot count commits in %s..%s: %v",
			abbrev(base), toRef, err)
	}
	if count == 0 {
		return nil, autoBaseRefuse(
			fmt.Sprintf("record --auto: %s..%s contains zero commits.", abbrev(base), toRef),
			toRef)
	}
	return &autoBaseResolution{
		Base:         base,
		BaseShort:    abbrev(base),
		ToRef:        toCommit,
		ToLabel:      toRef,
		Source:       src,
		SourceRef:    sourceRef,
		AheadCount:   count,
		FromFallback: fromFallback,
	}, nil
}

// buildResolutionWithGate is buildResolution + the merge-base safety
// gate (refuse if the range contains more than one commit).
func buildResolutionWithGate(repo, base, toCommit, toRef string,
	src autoBaseSource, sourceRef string) (*autoBaseResolution, error) {
	res, err := buildResolution(repo, base, toCommit, toRef, src, sourceRef, true)
	if err != nil {
		return nil, err
	}
	if res.AheadCount > 1 {
		return nil, fmt.Errorf(
			"record --auto inferred merge-base %s against %s, but the range contains %d commits.\n"+
				"This is too broad to trust automatically; it may include multiple feature commits.\n"+
				"Inspect with:\n"+
				"  git log --oneline %s..%s\n"+
				"Then rerun with one of:\n"+
				"  tpatch record <slug> --from <precise-base> --to <feature-tip>\n"+
				"  tpatch record <slug> --from %s --to <feature-tip> --files <feature-paths>",
			res.BaseShort, sourceRef, res.AheadCount, res.BaseShort, toRef, res.BaseShort)
	}
	return res, nil
}

// autoBaseRefuse formats a generic refusal that points the operator at
// explicit-flag recovery paths.
func autoBaseRefuse(headline, toRef string) error {
	return fmt.Errorf("%s\n  Recover with an explicit base:\n    tpatch record <slug> --from <base> --to %s\n    tpatch record <slug> --from <base> --files <paths>",
		headline, toRef)
}

func abbrev(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

// isTrackedTreeDirty reports whether the working tree has uncommitted
// changes to tracked files. It deliberately ignores untracked paths
// (notably `.tpatch/` artifacts that are added to .gitignore in many
// downstream repos, and the freshly-scaffolded `.tpatch/` in tests).
// `record --auto` only cares whether tracked content has drifted away
// from HEAD — untracked files are not part of the committed-range
// diff it would emit anyway.
func isTrackedTreeDirty(repoRoot string) bool {
	out, err := gitStatusTrackedOnly(repoRoot)
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != ""
}

func gitStatusTrackedOnly(repoRoot string) (string, error) {
	// `--untracked-files=no` filters the porcelain output to tracked
	// changes (modified, deleted, staged), which is exactly the set
	// the --auto contract refuses.
	c := exec.Command("git", "status", "--porcelain", "--untracked-files=no")
	c.Dir = repoRoot
	out, err := c.Output()
	return string(out), err
}
