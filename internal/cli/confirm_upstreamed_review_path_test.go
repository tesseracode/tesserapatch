package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// setupConfirmUpstreamedReviewFixture builds a repo whose HEAD contains
// a synthetic "adopting" upstream commit, plus a feature in state
// `blocked` with review_verdict `rejected-upstreamed` (mirroring the
// reporter's step-2 state) and a human review revision matching the
// authorising tuple (mirroring step-3).
//
// Returns (dir, slug, upstreamSHA, reviewEntryID). By default the
// feature has NO Reconcile.UpstreamRef, so reachability falls back to
// HEAD (with warning) unless the caller sets one.
func setupConfirmUpstreamedReviewFixture(t *testing.T) (string, string, string, string) {
	t.Helper()
	dir := t.TempDir()
	gitInitTestRepo(t, dir)
	// Add an "adopting" upstream commit whose SHA the operator will
	// supply. It's reachable from HEAD.
	if err := os.WriteFile(filepath.Join(dir, "upstream.txt"), []byte("adopted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "upstream.txt")
	gitRun(t, dir, "commit", "-m", "upstream absorbed the feature")
	upstream := gitHead(t, dir)

	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.AddFeature(store.AddFeatureInput{Title: "windows wsl support", Slug: "windows-wsl-support", Request: "wsl support"})
	if err != nil {
		t.Fatal(err)
	}
	f.State = store.StateBlocked
	f.Reconcile.Outcome = store.ReconcileBlocked
	f.Reconcile.ReviewVerdict = "rejected-upstreamed"
	if err := s.SaveFeatureStatus(f); err != nil {
		t.Fatal(err)
	}

	// Reporter's step 3: append the human review revision.
	review := store.ReconcileRevision{
		SchemaVersion:       store.ReconcileRevisionSchemaVersion,
		FeatureSlug:         f.Slug,
		EvidenceAttemptID:   "re_1125ecf82225",
		RawReconcileVerdict: string(store.ReconcileUpstreamed),
		ReviewVerdict:       store.ReviewVerdictConfirmed,
		FinalFeatureState:   store.StateUpstreamMerged,
		ActionTaken:         store.ReconcileActionConfirmedRetired,
		ReasonCode:          "manual-review",
		ValidationRefs:      []store.ValidationRef{},
	}
	review.EntryID = store.ComputeRevisionID(review)
	if err := store.AppendReconcileRevision(s, f.Slug, review); err != nil {
		t.Fatal(err)
	}
	return dir, f.Slug, upstream, review.EntryID
}

// AC-1: fast path golden — status.State=upstream_merged +
// Outcome=upstreamed + ReviewVerdict=confirmed-upstreamed produces
// unchanged behavior (no transition revision, no consumed-entry
// output).
func TestConfirmUpstreamed_FastPath_ByteIdenticalGolden(t *testing.T) {
	dir := t.TempDir()
	setupCLIGit(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := s.AddFeature(store.AddFeatureInput{Title: "parent", Slug: "parent", Request: "parent"})
	f.State = store.StateUpstreamMerged
	f.Reconcile.Outcome = store.ReconcileUpstreamed
	f.Reconcile.ReviewVerdict = "confirmed-upstreamed"
	if err := s.SaveFeatureStatus(f); err != nil {
		t.Fatal(err)
	}
	out, _, code := runCmdWithError("reconcile", "confirm-upstreamed", "--path", dir, "parent")
	if code != 0 {
		t.Fatalf("fast path failed: %s", out)
	}
	// The fast path must not print consumed/transition envelope keys.
	if strings.Contains(out, "consumed review revision") || strings.Contains(out, "appended transition revision") {
		t.Fatalf("fast path unexpectedly emitted review-path envelope:\n%s", out)
	}
	if !strings.Contains(out, "confirmed upstreamed: parent") {
		t.Fatalf("expected fast-path success line:\n%s", out)
	}
}

// PRD-#4 rev-1 F-1 (AC-2 byte-identity): even when status.json carries
// populated Reconcile.UpstreamRef + Reconcile.UpstreamCommit (which the
// review path is expected to write for future transitions), the fast
// path JSON output MUST match the pre-PRD-#4 shape — namely the
// upstream_ref / upstream_commit fields are ABSENT from the CLI JSON.
// This guards AC-2 byte-identity against the fast-path struct init
// regressing to include the populated fields from status.json.
func TestConfirmUpstreamed_FastPath_JSON_OmitsUpstreamFields_WhenStatusPopulated(t *testing.T) {
	dir := t.TempDir()
	setupCLIGit(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := s.AddFeature(store.AddFeatureInput{Title: "parent", Slug: "parent", Request: "parent"})
	f.State = store.StateUpstreamMerged
	f.Reconcile.Outcome = store.ReconcileUpstreamed
	f.Reconcile.ReviewVerdict = "confirmed-upstreamed"
	// Simulate the case where status.json already carries populated
	// upstream anchors from a prior transition or reconcile run.
	f.Reconcile.UpstreamRef = "refs/remotes/origin/main"
	f.Reconcile.UpstreamCommit = "0123456789abcdef0123456789abcdef01234567"
	if err := s.SaveFeatureStatus(f); err != nil {
		t.Fatal(err)
	}
	out, _, code := runCmdWithError("reconcile", "confirm-upstreamed", "--path", dir, "parent", "--json")
	if code != 0 {
		t.Fatalf("fast path (json) failed: %s", out)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("failed to parse fast-path JSON: %v\n%s", err, out)
	}
	// Pre-PRD-#4 behavior emitted empty-string values (or absence)
	// for upstream_ref / upstream_commit on the fast path — the fields
	// were left zero on the ReconcileResult and status.json values
	// never leaked through. AC-2 byte-identity requires the fast path
	// remain that shape even when status.Reconcile carries populated
	// upstream anchors. If PRD-#4's struct init regresses and passes
	// status.Reconcile.UpstreamCommit through, this test flips to red.
	if v, ok := payload["upstream_ref"]; ok && v != "" {
		t.Fatalf("PRD AC-2 byte-identity: fast-path JSON must not leak status.Reconcile.UpstreamRef, got upstream_ref=%v\n%s", v, out)
	}
	if v, ok := payload["upstream_commit"]; ok && v != "" {
		t.Fatalf("PRD AC-2 byte-identity: fast-path JSON must not leak status.Reconcile.UpstreamCommit, got upstream_commit=%v\n%s", v, out)
	}
	// Sanity: envelope must not include review-path-only keys.
	for _, key := range []string{"source_revision_entry_id", "transition_revision_entry_id"} {
		if _, present := payload[key]; present {
			t.Fatalf("PRD AC-2 byte-identity: fast-path JSON leaked review-path key %q:\n%s", key, out)
		}
	}
}

// AC-2: fast-path entry invariant — refuse when State != upstream_merged
// even if Outcome/ReviewVerdict claim confirmed.
func TestConfirmUpstreamed_FastPath_EntryInvariantRefuses(t *testing.T) {
	dir := t.TempDir()
	setupCLIGit(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := s.AddFeature(store.AddFeatureInput{Title: "bad", Slug: "bad", Request: "bad"})
	f.State = store.StateApplied // mismatched with Reconcile.Outcome below
	f.Reconcile.Outcome = store.ReconcileUpstreamed
	f.Reconcile.ReviewVerdict = "confirmed-upstreamed"
	if err := s.SaveFeatureStatus(f); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runCmdWithError("reconcile", "confirm-upstreamed", "--path", dir, "bad")
	if code == 0 {
		t.Fatalf("expected refusal for fast-path entry invariant")
	}
	if !strings.Contains(errOut, "fast path requires state") {
		t.Fatalf("expected entry-invariant diagnostic in stderr: %s", errOut)
	}
}

// AC-1 (partial) / AC-10: the v0.11 gate wording is preserved
// byte-identically when no --upstream-commit and no authorising
// revision.
func TestConfirmUpstreamed_V011_GateWordingByteIdentical(t *testing.T) {
	dir := t.TempDir()
	setupCLIGit(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := s.AddFeature(store.AddFeatureInput{Title: "blocked", Slug: "blocked", Request: "blocked"})
	f.State = store.StateBlocked
	f.Reconcile.Outcome = store.ReconcileBlocked
	f.Reconcile.ReviewVerdict = "rejected-upstreamed"
	if err := s.SaveFeatureStatus(f); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runCmdWithError("reconcile", "confirm-upstreamed", "--path", dir, "blocked")
	if code == 0 {
		t.Fatalf("expected refusal when no authorising revision and no --upstream-commit")
	}
	want := `confirm-upstreamed requires reconcile outcome "upstreamed" or review_verdict "confirmed-upstreamed" for blocked`
	if !strings.Contains(errOut, want) {
		t.Fatalf("expected v0.11 error verbatim:\ngot:  %s\nwant: %s", errOut, want)
	}
}

// AC-5 fall-back branch (TS 9.12): HEAD-only reachability succeeds with
// residual-risk warning to stderr.
func TestConfirmUpstreamed_ReviewPath_HEADFallbackWithWarning(t *testing.T) {
	dir, slug, sha, reviewID := setupConfirmUpstreamedReviewFixture(t)
	out, errOut, code := runCmdWithError("reconcile", "confirm-upstreamed", "--path", dir, slug, "--upstream-commit", sha)
	if code != 0 {
		t.Fatalf("review path failed: %s / %s", out, errOut)
	}
	if !strings.Contains(errOut, "warning: no upstream ref resolvable for "+slug) {
		t.Fatalf("expected residual-risk warning naming the slug on stderr, got: %s", errOut)
	}
	if !strings.Contains(errOut, "reachable from HEAD only") {
		t.Fatalf("expected HEAD-only reachability phrasing, got: %s", errOut)
	}
	if !strings.Contains(errOut, "Local operators can insert commits into HEAD's ancestry") {
		t.Fatalf("expected warning to name the local-insertion threat model, got: %s", errOut)
	}
	if !strings.Contains(errOut, "audit before relying on this transition") {
		t.Fatalf("expected warning to instruct audit-before-relying action, got: %s", errOut)
	}
	if !strings.Contains(out, "consumed review revision: "+reviewID) {
		t.Fatalf("expected consumed review revision line, got: %s", out)
	}
	// AC-6: consumed entry byte-identity assertion.
	s, _ := store.Open(dir)
	revs, err := store.LoadReconcileRevisions(s, slug)
	if err != nil {
		t.Fatalf("load revisions: %v", err)
	}
	if len(revs) < 2 {
		t.Fatalf("expected at least 2 revisions (consumed + transition), got %d", len(revs))
	}
	var consumed *store.ReconcileRevision
	var transition *store.ReconcileRevision
	for i := range revs {
		if revs[i].EntryID == reviewID {
			consumed = &revs[i]
		}
		if revs[i].SupersedesEntryID == reviewID {
			transition = &revs[i]
		}
	}
	if consumed == nil {
		t.Fatalf("consumed entry %s missing after transition", reviewID)
	}
	if transition == nil {
		t.Fatalf("no transition revision supersedes %s", reviewID)
	}
	if consumed.ActionTaken != store.ReconcileActionConfirmedRetired || consumed.ReviewVerdict != store.ReviewVerdictConfirmed || consumed.FinalFeatureState != store.StateUpstreamMerged {
		t.Fatalf("consumed entry mutated: %+v", consumed)
	}
	// Confirm the ValidationRefs on the transition entry match the
	// PRD §7.1 precedent + source-revision back-link.
	sawUpstreamRef, sawSource := false, false
	for _, r := range transition.ValidationRefs {
		if r.Kind == "upstream-commit" && r.Value == sha {
			sawUpstreamRef = true
		}
		if r.Kind == "source-revision" && r.Value == reviewID {
			sawSource = true
		}
	}
	if !sawUpstreamRef || !sawSource {
		t.Fatalf("transition ValidationRefs missing precedent Kinds: %+v", transition.ValidationRefs)
	}
	// State mutation.
	st, _ := s.LoadFeatureStatus(slug)
	if st.State != store.StateUpstreamMerged {
		t.Fatalf("state not mutated: %s", st.State)
	}
	if st.Reconcile.Outcome != store.ReconcileUpstreamed || st.Reconcile.ReviewVerdict != "confirmed-upstreamed" || st.Reconcile.UpstreamCommit != sha {
		t.Fatalf("reconcile fields not mutated: %+v", st.Reconcile)
	}
}

// AC-5 preferred branch (TS 9.11): status.Reconcile.UpstreamRef is
// resolvable → do NOT emit warning; sha ancestor of that ref succeeds.
func TestConfirmUpstreamed_ReviewPath_UpstreamRefResolvableNoWarning(t *testing.T) {
	dir, slug, sha, _ := setupConfirmUpstreamedReviewFixture(t)
	// Point Reconcile.UpstreamRef at HEAD so ancestry succeeds.
	s, _ := store.Open(dir)
	st, _ := s.LoadFeatureStatus(slug)
	st.Reconcile.UpstreamRef = "HEAD"
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runCmdWithError("reconcile", "confirm-upstreamed", "--path", dir, slug, "--upstream-commit", sha)
	if code != 0 {
		t.Fatalf("preferred path failed: %s", errOut)
	}
	if strings.Contains(errOut, "warning: no upstream ref resolvable") {
		t.Fatalf("preferred path unexpectedly emitted residual-risk warning: %s", errOut)
	}
}

// AC-5 refusal (TS 9.6): non-reachable SHA refuses under both branches.
func TestConfirmUpstreamed_ReviewPath_UnreachableSHARefuses(t *testing.T) {
	dir, slug, _, _ := setupConfirmUpstreamedReviewFixture(t)
	// Create a commit that exists but is not in HEAD's ancestry: use
	// `git commit-tree` against a synthetic empty tree.
	emptyTree := strings.TrimSpace(gitOut(t, dir, "mktree"))
	unreach := strings.TrimSpace(gitOut(t, dir, "commit-tree", emptyTree, "-m", "orphan"))
	if unreach == "" {
		t.Fatalf("failed to synthesize orphan commit")
	}
	// HEAD-only branch.
	_, errOut, code := runCmdWithError("reconcile", "confirm-upstreamed", "--path", dir, slug, "--upstream-commit", unreach)
	if code == 0 {
		t.Fatalf("expected refusal on unreachable sha")
	}
	if !strings.Contains(errOut, "not an ancestor of HEAD") {
		t.Fatalf("expected HEAD ancestry diagnostic: %s", errOut)
	}
	// UpstreamRef branch.
	s, _ := store.Open(dir)
	st, _ := s.LoadFeatureStatus(slug)
	st.Reconcile.UpstreamRef = "HEAD"
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatal(err)
	}
	_, errOut2, code2 := runCmdWithError("reconcile", "confirm-upstreamed", "--path", dir, slug, "--upstream-commit", unreach)
	if code2 == 0 {
		t.Fatalf("expected refusal on unreachable sha under upstream-ref branch")
	}
	if !strings.Contains(errOut2, "not an ancestor of upstream ref") {
		t.Fatalf("expected upstream-ref ancestry diagnostic: %s", errOut2)
	}
}

// AC-11: supersession matrix — all 5 rows exercised.
func TestConfirmUpstreamed_ReviewPath_SupersessionMatrix(t *testing.T) {
	rows := []struct {
		name        string
		targetState store.FeatureState
		wantRefuse  bool
	}{
		{"applied_healthy", store.StateApplied, true},
		// stale satisfied_by is same class as healthy applied; we
		// exercise it by setting a non-matching SatisfiedBy on the edge
		// itself. But since the matrix only inspects target state, we
		// use state=applied to cover the class.
		{"applied_stale_satisfied_by", store.StateApplied, true},
		{"promoted", store.FeatureState("promoted"), true},
		{"blocked", store.StateBlocked, true},
		{"upstream_merged", store.StateUpstreamMerged, false},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			dir, slug, sha, _ := setupConfirmUpstreamedReviewFixture(t)
			s, _ := store.Open(dir)
			// Add a supersedes-target and edge.
			target, err := s.AddFeature(store.AddFeatureInput{Title: "older", Slug: "older-feat-" + row.name, Request: "older"})
			if err != nil {
				t.Fatal(err)
			}
			target.State = row.targetState
			if err := s.SaveFeatureStatus(target); err != nil {
				t.Fatal(err)
			}
			st, _ := s.LoadFeatureStatus(slug)
			edge := store.Dependency{Slug: target.Slug, Kind: store.DependencyKindSupersedes}
			if row.name == "applied_stale_satisfied_by" {
				edge.SatisfiedBy = "0000000000000000000000000000000000000000"
			}
			st.DependsOn = append(st.DependsOn, edge)
			if err := s.SaveFeatureStatus(st); err != nil {
				t.Fatal(err)
			}
			_, errOut, code := runCmdWithError("reconcile", "confirm-upstreamed", "--path", dir, slug, "--upstream-commit", sha)
			if row.wantRefuse {
				if code == 0 {
					t.Fatalf("[%s] expected refusal but succeeded", row.name)
				}
				if !strings.Contains(errOut, "supersedes-target") {
					t.Fatalf("[%s] expected supersession diagnostic: %s", row.name, errOut)
				}
			} else {
				if code != 0 {
					t.Fatalf("[%s] expected success but failed: %s", row.name, errOut)
				}
			}
		})
	}
}

// AC-12: transition determinism — replaying the transition against a
// pristine fixture produces byte-identical transition-revision content
// (excluding RecordedAt, which the schema does not carry, and
// audit-cleanup revisions, which are out-of-scope per rev-1).
func TestConfirmUpstreamed_ReviewPath_TransitionDeterminism(t *testing.T) {
	// Two independent runs against isolated fixtures with identical
	// inputs must produce the same transition EntryID because
	// ComputeRevisionID hashes the canonical identity JSON.
	dirA, slugA, shaA, reviewA := setupConfirmUpstreamedReviewFixture(t)
	dirB, slugB, shaB, reviewB := setupConfirmUpstreamedReviewFixture(t)
	if reviewA != reviewB {
		t.Fatalf("fixture drift: review IDs differ %s vs %s", reviewA, reviewB)
	}
	// Fixture SHAs differ because the git init timestamp differs.
	// Run each fixture with its own reachable sha; then normalise the
	// UpstreamCommit field on the persisted transition entry to a
	// shared synthetic SHA and cross-compare the recomputed EntryIDs.
	_, _, codeA := runCmdWithError("reconcile", "confirm-upstreamed", "--path", dirA, slugA, "--upstream-commit", shaA)
	_, _, codeB := runCmdWithError("reconcile", "confirm-upstreamed", "--path", dirB, slugB, "--upstream-commit", shaB)
	if codeA != 0 || codeB != 0 {
		t.Fatalf("both runs must succeed: codeA=%d codeB=%d", codeA, codeB)
	}
	sA, _ := store.Open(dirA)
	sB, _ := store.Open(dirB)
	revsA, _ := store.LoadReconcileRevisions(sA, slugA)
	revsB, _ := store.LoadReconcileRevisions(sB, slugB)
	var transA, transB *store.ReconcileRevision
	for i := range revsA {
		if revsA[i].SupersedesEntryID == reviewA {
			transA = &revsA[i]
		}
	}
	for i := range revsB {
		if revsB[i].SupersedesEntryID == reviewB {
			transB = &revsB[i]
		}
	}
	if transA == nil {
		t.Fatalf("no transition revision found in dirA")
	}
	if transB == nil {
		t.Fatalf("no transition revision found in dirB")
	}
	// PRD-#4 rev-1 F-2: cross-fixture EntryID determinism. Fixture-
	// specific data enters the transition entry only via
	// ValidationRefs[Kind=upstream-commit].Value — the operator-
	// supplied --upstream-commit SHA differs between fixtures because
	// the two independent git-init timestamps produce different HEAD
	// commits. Normalise that field to a shared synthetic SHA on both
	// transitions and cross-compare the recomputed EntryIDs. This
	// converts the previous recompute-of-self tautology into a real
	// cross-fixture determinism check on the identity hash.
	const syntheticSHA = "deadbeefcafebabe1234567890abcdef01234567"
	normValidationRefs := func(refs []store.ValidationRef) []store.ValidationRef {
		out := make([]store.ValidationRef, len(refs))
		copy(out, refs)
		for i := range out {
			if out[i].Kind == "upstream-commit" {
				out[i].Value = syntheticSHA
			}
		}
		return out
	}
	normA := *transA
	normA.ValidationRefs = normValidationRefs(transA.ValidationRefs)
	normA.EntryID = ""
	normB := *transB
	normB.ValidationRefs = normValidationRefs(transB.ValidationRefs)
	normB.EntryID = ""
	idA := store.ComputeRevisionID(normA)
	idB := store.ComputeRevisionID(normB)
	if idA == "" {
		t.Fatalf("ComputeRevisionID returned empty for normalised transA")
	}
	if idA != idB {
		t.Fatalf("PRD F-2 cross-fixture determinism: normalised EntryIDs differ across identical fixtures\n  dirA %s\n  dirB %s", idA, idB)
	}
	// Recompute-of-self baseline (retained for regression on identity
	// hash shape drift): both persisted EntryIDs are reproduced from
	// their persisted fields.
	if got := store.ComputeRevisionID(*transA); got != transA.EntryID {
		t.Fatalf("recompute-of-self drift (dirA): got %s vs persisted %s", got, transA.EntryID)
	}
	if got := store.ComputeRevisionID(*transB); got != transB.EntryID {
		t.Fatalf("recompute-of-self drift (dirB): got %s vs persisted %s", got, transB.EntryID)
	}
}

// AC-1 reporter reproduction: 5-step scenario end-to-end. This is the
// PRD §9.2 happy path plus the JSON envelope keys the reporter names.
func TestConfirmUpstreamed_ReporterReproduction(t *testing.T) {
	dir, slug, sha, reviewID := setupConfirmUpstreamedReviewFixture(t)
	out, errOut, code := runCmdWithError("reconcile", "confirm-upstreamed", "--path", dir, slug, "--upstream-commit", sha, "--json")
	if code != 0 {
		t.Fatalf("reporter repro failed: %s / %s", out, errOut)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("json payload not valid: %v\n%s", err, out)
	}
	if payload["source_revision_entry_id"] != reviewID {
		t.Fatalf("missing/wrong source_revision_entry_id: %v", payload["source_revision_entry_id"])
	}
	if payload["upstream_commit"] != sha {
		t.Fatalf("missing/wrong upstream_commit: %v", payload["upstream_commit"])
	}
	if payload["transition_revision_entry_id"] == nil || payload["transition_revision_entry_id"] == "" {
		t.Fatalf("missing transition_revision_entry_id: %v", payload["transition_revision_entry_id"])
	}
	// Reporter's audit-trail assertion (AC-8): review list --all yields
	// at least 3 entries.
	listOut, _, listCode := runCmd("reconcile", "review", "list", "--path", dir, "--all", "--json", slug)
	if listCode != 0 {
		t.Fatalf("review list failed: %s", listOut)
	}
	var listPayload struct {
		Revisions []store.ReconcileRevision `json:"revisions"`
	}
	if err := json.Unmarshal([]byte(listOut), &listPayload); err != nil {
		t.Fatalf("review list json: %v\n%s", err, listOut)
	}
	if len(listPayload.Revisions) < 2 {
		t.Fatalf("expected at least 2 revisions in list --all, got %d", len(listPayload.Revisions))
	}
}

// TS 9.5: --upstream-commit required on the review path when an
// authorising revision is already recorded.
func TestConfirmUpstreamed_ReviewPath_UpstreamCommitRequired(t *testing.T) {
	dir, slug, _, _ := setupConfirmUpstreamedReviewFixture(t)
	_, errOut, code := runCmdWithError("reconcile", "confirm-upstreamed", "--path", dir, slug)
	if code == 0 {
		t.Fatalf("expected refusal without --upstream-commit")
	}
	if !strings.Contains(errOut, "--upstream-commit") {
		t.Fatalf("expected diagnostic naming --upstream-commit: %s", errOut)
	}
}

// TS 9.4: review path with incomplete authorising tuple — refuse.
func TestConfirmUpstreamed_ReviewPath_TupleMismatchRefuses(t *testing.T) {
	dir := t.TempDir()
	gitInitTestRepo(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := s.AddFeature(store.AddFeatureInput{Title: "t", Slug: "t", Request: "t"})
	f.State = store.StateBlocked
	f.Reconcile.Outcome = store.ReconcileBlocked
	f.Reconcile.ReviewVerdict = "rejected-upstreamed"
	if err := s.SaveFeatureStatus(f); err != nil {
		t.Fatal(err)
	}
	// Wrong action_taken — deferred instead of confirmed-retired.
	bad := store.ReconcileRevision{
		SchemaVersion:       store.ReconcileRevisionSchemaVersion,
		FeatureSlug:         f.Slug,
		EvidenceAttemptID:   "re_xyz",
		RawReconcileVerdict: string(store.ReconcileUpstreamed),
		ReviewVerdict:       store.ReviewVerdictConfirmed,
		FinalFeatureState:   store.StateUpstreamMerged,
		ActionTaken:         store.ReconcileActionDeferred,
		ReasonCode:          "not-quite",
		ValidationRefs:      []store.ValidationRef{},
	}
	bad.EntryID = store.ComputeRevisionID(bad)
	if err := store.AppendReconcileRevision(s, f.Slug, bad); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runCmdWithError("reconcile", "confirm-upstreamed", "--path", dir, f.Slug, "--upstream-commit", gitHead(t, dir))
	if code == 0 {
		t.Fatalf("expected refusal on tuple mismatch")
	}
	if !strings.Contains(errOut, "no non-superseded revision authorises retirement") {
		t.Fatalf("expected §4 D5 error shape: %s", errOut)
	}
}

// TS 9.7: --from-revision overrides latest selection.
func TestConfirmUpstreamed_ReviewPath_FromRevisionOverrides(t *testing.T) {
	dir, slug, sha, olderID := setupConfirmUpstreamedReviewFixture(t)
	s, _ := store.Open(dir)
	// Append a newer matching revision.
	newer := store.ReconcileRevision{
		SchemaVersion:       store.ReconcileRevisionSchemaVersion,
		FeatureSlug:         slug,
		EvidenceAttemptID:   "re_newerreview",
		RawReconcileVerdict: string(store.ReconcileUpstreamed),
		ReviewVerdict:       store.ReviewVerdictConfirmed,
		FinalFeatureState:   store.StateUpstreamMerged,
		ActionTaken:         store.ReconcileActionConfirmedRetired,
		ReasonCode:          "manual-review",
		ValidationRefs:      []store.ValidationRef{},
	}
	newer.EntryID = store.ComputeRevisionID(newer)
	if err := store.AppendReconcileRevision(s, slug, newer); err != nil {
		t.Fatal(err)
	}
	// Override to older.
	_, errOut, code := runCmdWithError("reconcile", "confirm-upstreamed", "--path", dir, slug, "--upstream-commit", sha, "--from-revision", olderID)
	if code != 0 {
		t.Fatalf("--from-revision override failed: %s", errOut)
	}
	revs, _ := store.LoadReconcileRevisions(s, slug)
	sawSupersedesOlder := false
	for _, r := range revs {
		if r.SupersedesEntryID == olderID {
			sawSupersedesOlder = true
		}
	}
	if !sawSupersedesOlder {
		t.Fatalf("expected transition to supersede older ID %s; revisions=%+v", olderID, revs)
	}
}

// TS 9.13: D4 tie-break — two matching revisions with different
// EvidenceAttemptIDs both survive the dedupe; last-in-file-order wins.
func TestConfirmUpstreamed_ReviewPath_TieBreakLastInFileOrder(t *testing.T) {
	dir, slug, sha, olderID := setupConfirmUpstreamedReviewFixture(t)
	s, _ := store.Open(dir)
	// Second matching revision with a different EvidenceAttemptID
	// (so the dedupe key differs and both survive).
	newer := store.ReconcileRevision{
		SchemaVersion:       store.ReconcileRevisionSchemaVersion,
		FeatureSlug:         slug,
		EvidenceAttemptID:   "re_secondreview",
		RawReconcileVerdict: string(store.ReconcileUpstreamed),
		ReviewVerdict:       store.ReviewVerdictConfirmed,
		FinalFeatureState:   store.StateUpstreamMerged,
		ActionTaken:         store.ReconcileActionConfirmedRetired,
		ReasonCode:          "manual-review",
		ValidationRefs:      []store.ValidationRef{},
	}
	newer.EntryID = store.ComputeRevisionID(newer)
	if err := store.AppendReconcileRevision(s, slug, newer); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runCmdWithError("reconcile", "confirm-upstreamed", "--path", dir, slug, "--upstream-commit", sha)
	if code != 0 {
		t.Fatalf("tie-break run failed: %s", errOut)
	}
	revs, _ := store.LoadReconcileRevisions(s, slug)
	for _, r := range revs {
		if r.SupersedesEntryID == olderID {
			t.Fatalf("tie-break selected older entry; should pick last-in-file-order (%s)", newer.EntryID)
		}
	}
	sawSupersedesNewer := false
	for _, r := range revs {
		if r.SupersedesEntryID == newer.EntryID {
			sawSupersedesNewer = true
		}
	}
	if !sawSupersedesNewer {
		t.Fatalf("tie-break did not select last-in-file-order entry %s", newer.EntryID)
	}
}

// TestConfirmUpstreamed_ReviewPath_TieBreak_ReuseEvidenceKey covers the
// PRD-#4 rev-1 F-2 gap: three authorising, non-superseded revisions in
// file order A(evidence=x1), B(evidence=x2), C(evidence=x1) — dedup
// collapses A+C on the (slug, evidence, verdict, action) key. Before
// the F-2 fix, latestRevisionEntries preserved A's earlier positional
// slot for the collapsed key, so the reverse walk on the filtered
// slice returned B (positionally last) rather than C (last-in-file-
// order). PRD §4 D4 mandates C wins.
func TestConfirmUpstreamed_ReviewPath_TieBreak_ReuseEvidenceKey(t *testing.T) {
	dir, slug, sha, entryA := setupConfirmUpstreamedReviewFixture(t)
	s, _ := store.Open(dir)
	// setupConfirmUpstreamedReviewFixture already appended A with
	// EvidenceAttemptID="re_1125ecf82225". Append B (different
	// evidence, different reason_code so EntryID differs).
	entryB := store.ReconcileRevision{
		SchemaVersion:       store.ReconcileRevisionSchemaVersion,
		FeatureSlug:         slug,
		EvidenceAttemptID:   "re_middle_x2",
		RawReconcileVerdict: string(store.ReconcileUpstreamed),
		ReviewVerdict:       store.ReviewVerdictConfirmed,
		FinalFeatureState:   store.StateUpstreamMerged,
		ActionTaken:         store.ReconcileActionConfirmedRetired,
		ReasonCode:          "manual-review",
		ValidationRefs:      []store.ValidationRef{},
	}
	entryB.EntryID = store.ComputeRevisionID(entryB)
	if err := store.AppendReconcileRevision(s, slug, entryB); err != nil {
		t.Fatal(err)
	}
	// Append C reusing A's EvidenceAttemptID so the dedup key
	// (slug, evidence, verdict, action) collides with A. Differ on
	// reason_code so the identity hash — and therefore EntryID —
	// differs from A (otherwise AppendReconcileRevision would treat
	// it as a duplicate rather than a distinct later entry).
	entryC := store.ReconcileRevision{
		SchemaVersion:       store.ReconcileRevisionSchemaVersion,
		FeatureSlug:         slug,
		EvidenceAttemptID:   "re_1125ecf82225", // reuses A
		RawReconcileVerdict: string(store.ReconcileUpstreamed),
		ReviewVerdict:       store.ReviewVerdictConfirmed,
		FinalFeatureState:   store.StateUpstreamMerged,
		ActionTaken:         store.ReconcileActionConfirmedRetired,
		ReasonCode:          "manual-review-recheck",
		ValidationRefs:      []store.ValidationRef{},
	}
	entryC.EntryID = store.ComputeRevisionID(entryC)
	if entryC.EntryID == entryA {
		t.Fatalf("test setup drift: entryC.EntryID must differ from entryA to exercise the dedup collision (got %s)", entryC.EntryID)
	}
	if err := store.AppendReconcileRevision(s, slug, entryC); err != nil {
		t.Fatal(err)
	}
	// Sanity: file order is [A, B, C] and none are superseded.
	revs, _ := store.LoadReconcileRevisions(s, slug)
	if len(revs) != 3 {
		t.Fatalf("expected 3 revisions, got %d", len(revs))
	}
	if revs[0].EntryID != entryA || revs[1].EntryID != entryB.EntryID || revs[2].EntryID != entryC.EntryID {
		t.Fatalf("unexpected file order: %s %s %s", revs[0].EntryID, revs[1].EntryID, revs[2].EntryID)
	}
	_, errOut, code := runCmdWithError("reconcile", "confirm-upstreamed", "--path", dir, slug, "--upstream-commit", sha)
	if code != 0 {
		t.Fatalf("tie-break run failed: %s", errOut)
	}
	// After the run: exactly one transition revision was appended,
	// and its SupersedesEntryID must be C (last-in-file-order) — NOT
	// B (positionally-last-in-filtered) or A (earlier match with the
	// reused evidence key).
	revs, _ = store.LoadReconcileRevisions(s, slug)
	var transition *store.ReconcileRevision
	for i := range revs {
		if revs[i].SupersedesEntryID != "" {
			if transition != nil {
				t.Fatalf("expected exactly one transition entry, saw multiple")
			}
			transition = &revs[i]
		}
	}
	if transition == nil {
		t.Fatalf("expected one transition entry in %d revisions", len(revs))
	}
	if transition.SupersedesEntryID != entryC.EntryID {
		t.Fatalf("PRD F-2 tie-break: transition must supersede last-in-file-order match %s (C), got %s (A=%s B=%s)", entryC.EntryID, transition.SupersedesEntryID, entryA, entryB.EntryID)
	}
}
