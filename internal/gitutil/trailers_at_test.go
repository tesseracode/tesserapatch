package gitutil

// ADR-013 Amendment 3 D36 — EnumerateCommitTrailersAt reads exactly the
// commits reachable from the revision verify captured. These rows use no
// Git shim, so they also run on Windows.

import (
	"strings"
	"testing"
)

func enumeratedSHAs(t *testing.T, repo, rev string) map[string]bool {
	t.Helper()
	recs, err := EnumerateCommitTrailersAt(repo, rev)
	if err != nil {
		t.Fatalf("enumeration: %v", err)
	}
	out := map[string]bool{}
	for _, r := range recs {
		out[r.SHA] = true
	}
	return out
}

// Mutation caught: ignoring rev and enumerating the live HEAD.
func TestEnumerateCommitTrailersAtFollowsTheGivenRev(t *testing.T) {
	repo := newApplyFixtureRepo(t)
	h := strings.TrimSpace(runGitTest(t, repo, "rev-parse", "HEAD"))
	k := strings.TrimSpace(runGitTest(t, repo, "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "off HEAD"))

	atK := enumeratedSHAs(t, repo, k)
	if !atK[k] || !atK[h] {
		t.Fatalf("enumeration at %s must contain it and its parent %s: %v", k, h, atK)
	}
	runGitTest(t, repo, "update-ref", "refs/heads/main", k)
	atH := enumeratedSHAs(t, repo, h)
	if atH[k] || !atH[h] {
		t.Fatalf("enumeration at %s must not follow the live HEAD to %s: %v", h, k, atH)
	}
	live := enumeratedSHAs(t, repo, "")
	if !live[k] {
		t.Fatalf("the live-HEAD form must still follow HEAD: %v", live)
	}
}

// Mutation caught: applying the empty-repository tolerance to an
// explicit revision, which would read an unresolvable captured commit as
// a history with no landings.
func TestEnumerateCommitTrailersAtUnknownRevFails(t *testing.T) {
	repo := newApplyFixtureRepo(t)
	// A missing ref draws the very diagnostic the empty-repository
	// tolerance matches; a missing object id draws "bad object".
	for _, rev := range []string{"refs/heads/missing", strings.Repeat("d", 40)} {
		recs, err := EnumerateCommitTrailersAt(repo, rev)
		if err == nil {
			t.Fatalf("an unresolvable explicit rev %q must fail, got %d records", rev, len(recs))
		}
	}
	empty := t.TempDir()
	runGitTest(t, empty, "init", "-q", "-b", "main", ".")
	if recs, err := EnumerateCommitTrailers(empty); err != nil || len(recs) != 0 {
		t.Fatalf("an empty repository stays a clean empty enumeration: %d records, %v", len(recs), err)
	}
}
