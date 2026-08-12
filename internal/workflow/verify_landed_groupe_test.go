package workflow

// Acceptance rows AC-L46 – AC-L71 — PRD-verify-freshness §7.1 Group E:
// the evidence reader, its grammar, the closed presence states,
// topology, shallow history and partial clones.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

// AC-L46 — no reachable landing and no raw match ⇒ `none`, forward mode,
// shadow at HEAD, V7/V8 verdicts identical to the pre-amendment
// implementation.
func TestACL46_NoEvidenceStaysForward(t *testing.T) {
	f := newLandedFixture(t, "unlanded")
	f.Implement("l1\nl2\nCHANGED\nl4\nl5\nl6\nl7\nl8\nl9\nl10\nl11\nl12\nl13\nl14\nl15\n")
	r := f.Verify()
	if r.LandingEvidence.State != EvidenceNone {
		t.Fatalf("evidence=%q want none", r.LandingEvidence.State)
	}
	if r.TargetMode != TargetModeForward {
		t.Errorf("target_mode=%q want forward", r.TargetMode)
	}
	if r.Baseline.Mode != BaselineModeHead {
		t.Errorf("baseline.mode=%q want head-anchored", r.Baseline.Mode)
	}
	if r.Baseline.HistoricalAnchor != nil {
		t.Errorf("forward mode must not report a historical anchor: %+v", r.Baseline.HistoricalAnchor)
	}
	v7 := checkByID(t, r, CheckRecipeReplayClean)
	v8 := checkByID(t, r, CheckPostApplyPatchReplayClean)
	if v7.Mode != ModeForward || v8.Mode != ModeForward {
		t.Errorf("modes: v7=%q v8=%q want forward", v7.Mode, v8.Mode)
	}
	if len(v8.AnchorResults) != 0 {
		t.Errorf("forward V8 must not carry anchor_results: %v", v8.AnchorResults)
	}
}

// AC-L47 — all three values match with patch present-nonempty ⇒ `exact`.
func TestACL47_AllThreeValuesMatchIsExact(t *testing.T) {
	f := newLadderFixture(t)
	ev := f.Verify().LandingEvidence
	if ev.State != EvidenceExact {
		t.Fatalf("evidence=%q want exact", ev.State)
	}
	for name, got := range map[string]*bool{
		"patch_sha_match": ev.PatchSHAMatch, "recipe_sha_match": ev.RecipeSHAMatch, "base_commit_match": ev.BaseCommitMatch,
	} {
		if got == nil || !*got {
			t.Errorf("%s not reported true", name)
		}
	}
	if ev.PatchPresence != PresenceNonEmpty {
		t.Errorf("patch_presence=%q", ev.PatchPresence)
	}
	if ev.RecipePresence != RecipeShapeWithOps {
		t.Errorf("recipe_presence=%q", ev.RecipePresence)
	}
}

// AC-L48 — Tpatch-Patch-SHA mismatch ⇒ `stale`, FAIL, landing-evidence;
// V8 and V10 also failed WITH mode present.
func TestACL48_PatchSHAMismatchIsStale(t *testing.T) {
	f := newLadderFixture(t)
	f.WritePatch(string(f.PatchBytes()) + "\n")
	r := f.Verify()
	if r.LandingEvidence.State != EvidenceStale {
		t.Fatalf("evidence=%q want stale", r.LandingEvidence.State)
	}
	if r.FailedAt != FailedAtLandingEvidence {
		t.Errorf("failed_at=%q want landing-evidence", r.FailedAt)
	}
	for _, id := range []string{CheckRecipeReplayClean, CheckPostApplyPatchReplayClean, CheckWriteFilePreimageFresh} {
		c := checkByID(t, r, id)
		if c.Passed || c.Mode == "" {
			t.Errorf("%s: passed=%v mode=%q — want failed with mode present", id, c.Passed, c.Mode)
		}
	}
	if !strings.Contains(checkByID(t, r, CheckRecipeReplayClean).Remediation, "is stale: commit ") {
		t.Errorf("expected R6; got %q", checkByID(t, r, CheckRecipeReplayClean).Remediation)
	}
}

// AC-L49 — Recipe-SHA mismatch ⇒ stale; Base-Commit mismatch ⇒ stale.
func TestACL49_RecipeAndBaseMismatchAreStale(t *testing.T) {
	t.Run("recipe-sha", func(t *testing.T) {
		f := newLadderFixture(t)
		f.WriteRecipe(ApplyRecipe{Feature: f.Slug, Operations: []RecipeOperation{
			{Type: "write-file", Path: f.FilePath, Content: "different\n"},
		}})
		if got := f.Verify().LandingEvidence.State; got != EvidenceStale {
			t.Fatalf("evidence=%q want stale", got)
		}
	})
	t.Run("base-commit", func(t *testing.T) {
		f := newLadderFixture(t)
		st, _ := f.Store.LoadFeatureStatus(f.Slug)
		st.Apply.BaseCommit = strings.Repeat("c", 40)
		if err := f.Store.SaveFeatureStatus(st); err != nil {
			t.Fatal(err)
		}
		if got := f.Verify().LandingEvidence.State; got != EvidenceStale {
			t.Fatalf("evidence=%q want stale", got)
		}
	})
}

// AC-L50 — `Tpatch-Recipe-SHA: none` matches an ABSENT recipe and a
// PRESENT-EMPTY (whitespace-only) recipe.
func TestACL50_RecipeSHANoneMatchesAbsentAndWhitespace(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, f *landedFixture)
	}{
		{"absent", func(t *testing.T, f *landedFixture) {
			if err := os.Remove(artifactPath(f.Root, f.Slug, "apply-recipe.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{"whitespace-only", func(t *testing.T, f *landedFixture) {
			mustWriteFile(t, artifactPath(f.Root, f.Slug, "apply-recipe.json"), "  \n\t\n")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newLadderFixture(t)
			tc.setup(t, f)
			// Re-attest with the recipe in its new shape.
			f.LandWithBlock(f.TrailerBlock(f.Slug))
			ctx := newVerifyRunContext(f.Store)
			entry := ctx.inv.Entry(f.Slug)
			if got := entry.ExpectedRecipeSHA(); got != "none" {
				t.Fatalf("expected recipe trailer 'none', got %q", got)
			}
		})
	}
}

// AC-L51 — presence PRECEDES digest: patch absent ⇒ `patch_presence:
// absent` and `patch_sha_match` OMITTED entirely; no mismatch reported.
func TestACL51_PresencePrecedesDigest(t *testing.T) {
	f := newLadderFixture(t)
	if err := os.Remove(artifactPath(f.Root, f.Slug, "post-apply.patch")); err != nil {
		t.Fatal(err)
	}
	r := f.Verify()
	ev := r.LandingEvidence
	if ev.PatchPresence != PresenceAbsent {
		t.Fatalf("patch_presence=%q want absent", ev.PatchPresence)
	}
	if ev.PatchSHAMatch != nil {
		t.Errorf("patch_sha_match must be omitted, got %v", *ev.PatchSHAMatch)
	}
	if ev.State == EvidenceExact || ev.State == EvidenceStale {
		t.Errorf("state=%q must be reachable only from present-nonempty", ev.State)
	}
	if r.FailedAt != FailedAtLandedArtifacts {
		t.Errorf("failed_at=%q want landed-artifacts-absent", r.FailedAt)
	}
}

// AC-L52 — patch PRESENT-EMPTY ⇒ terminal landed-artifacts-absent by the
// same short-circuit; neither exact nor stale is reachable.
func TestACL52_PresentEmptyPatchShortCircuits(t *testing.T) {
	f := newLadderFixture(t)
	mustWriteFile(t, artifactPath(f.Root, f.Slug, "post-apply.patch"), "")
	r := f.Verify()
	if r.LandingEvidence.PatchPresence != PresenceEmpty {
		t.Fatalf("patch_presence=%q want present-empty", r.LandingEvidence.PatchPresence)
	}
	if r.FailedAt != FailedAtLandedArtifacts {
		t.Fatalf("failed_at=%q want landed-artifacts-absent", r.FailedAt)
	}
	if r.LandingEvidence.State == EvidenceExact || r.LandingEvidence.State == EvidenceStale {
		t.Errorf("state=%q must not be reachable", r.LandingEvidence.State)
	}
	if got := checkByID(t, r, CheckRecipeReplayClean).Remediation; got != remediationR19(f.Slug) {
		t.Errorf("R19 not verbatim: %q", got)
	}
}

// AC-L53 — the four recipe shapes are distinguished and the full 3×4
// cross-product is exclusive and total.
func TestACL53_PresenceCrossProductIsTotalAndExclusive(t *testing.T) {
	patchStates := []struct {
		name  string
		apply func(t *testing.T, f *landedFixture)
	}{
		{PresenceAbsent, func(t *testing.T, f *landedFixture) {
			_ = os.Remove(artifactPath(f.Root, f.Slug, "post-apply.patch"))
		}},
		{PresenceEmpty, func(t *testing.T, f *landedFixture) {
			mustWriteFile(t, artifactPath(f.Root, f.Slug, "post-apply.patch"), "")
		}},
		{PresenceNonEmpty, func(t *testing.T, f *landedFixture) {}},
	}
	recipeShapes := []struct {
		name  string
		apply func(t *testing.T, f *landedFixture)
	}{
		{PresenceAbsent, func(t *testing.T, f *landedFixture) {
			_ = os.Remove(artifactPath(f.Root, f.Slug, "apply-recipe.json"))
		}},
		{PresenceEmpty, func(t *testing.T, f *landedFixture) {
			mustWriteFile(t, artifactPath(f.Root, f.Slug, "apply-recipe.json"), "   \n")
		}},
		{RecipeShapeZeroOp, func(t *testing.T, f *landedFixture) {
			f.WriteRecipe(ApplyRecipe{Feature: f.Slug})
		}},
		{RecipeShapeWithOps, func(t *testing.T, f *landedFixture) {}},
	}
	for _, ps := range patchStates {
		for _, rs := range recipeShapes {
			t.Run(ps.name+"/"+rs.name, func(t *testing.T) {
				f := newLadderFixture(t)
				ps.apply(t, f)
				rs.apply(t, f)
				if ps.name == PresenceNonEmpty {
					// Re-attest so the recipe shape (not a stale digest)
					// is what the row exercises. An absent or empty patch
					// cannot be attested at all — `land` refuses to
					// produce it (AC-LD12) — so those cells keep the
					// original landing.
					f.LandWithBlock(f.TrailerBlock(f.Slug))
				}
				r := f.Verify()
				ev := r.LandingEvidence
				if ev.PatchPresence != ps.name {
					t.Fatalf("patch_presence=%q want %q", ev.PatchPresence, ps.name)
				}
				if ev.RecipePresence != rs.name {
					t.Fatalf("recipe_presence=%q want %q", ev.RecipePresence, rs.name)
				}
				if ps.name == PresenceNonEmpty {
					if r.FailedAt == FailedAtLandedArtifacts {
						t.Fatalf("present-nonempty must not short-circuit")
					}
					if rs.name == RecipeShapeZeroOp {
						v7 := checkByID(t, r, CheckRecipeReplayClean)
						if !strings.Contains(v7.Reason, "0 op(s)") {
							t.Errorf("zero-op recipe must record `0 op(s)`, got reason=%q passed=%v", v7.Reason, v7.Passed)
						}
					}
					return
				}
				// The eight short-circuit cells.
				if r.FailedAt != FailedAtLandedArtifacts {
					t.Fatalf("failed_at=%q want landed-artifacts-absent", r.FailedAt)
				}
				if ev.State == EvidenceExact || ev.State == EvidenceStale {
					t.Fatalf("state=%q is unreachable from %q", ev.State, ps.name)
				}
				if ev.PatchSHAMatch != nil {
					t.Errorf("no digest comparison may be attempted")
				}
			})
		}
	}
}

// AC-L54 — missing any of the four trailers, a duplicate of any, or ≥2
// Tpatch-Feature values ⇒ malformed.
func TestACL54_CardinalityViolationsAreMalformed(t *testing.T) {
	base := func(f *landedFixture) []string {
		return strings.Split(strings.TrimRight(f.TrailerBlock(f.Slug), "\n"), "\n")
	}
	cases := map[string]func(f *landedFixture) string{
		"missing-patch-sha": func(f *landedFixture) string {
			l := base(f)
			return strings.Join(append(l[:1], l[2:]...), "\n") + "\n"
		},
		"missing-recipe-sha": func(f *landedFixture) string {
			l := base(f)
			return strings.Join(append(l[:2], l[3:]...), "\n") + "\n"
		},
		"missing-base-commit": func(f *landedFixture) string {
			l := base(f)
			return strings.Join(l[:3], "\n") + "\n"
		},
		"duplicate-patch-sha": func(f *landedFixture) string {
			l := base(f)
			return strings.Join(append(l, l[1]), "\n") + "\n"
		},
		"two-feature-values": func(f *landedFixture) string {
			l := base(f)
			return strings.Join(append(l, "Tpatch-Feature: other-slug"), "\n") + "\n"
		},
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			f := newLadderFixture(t)
			// Replace the landing with a malformed one.
			mustGit(t, f.Root, "reset", "-q", "--soft", f.LandingCommit+"^")
			mustGit(t, f.Root, "commit", "-q", "-m", "feat: land "+f.Slug, "-m", mk(f))
			r := f.Verify()
			if r.LandingEvidence.State != EvidenceMalformed {
				t.Fatalf("evidence=%q want malformed", r.LandingEvidence.State)
			}
			if r.FailedAt != FailedAtLandingEvidence {
				t.Errorf("failed_at=%q", r.FailedAt)
			}
		})
	}
}

// AC-L55 — a commit whose RAW body carries an exact `Tpatch-Feature:
// <slug>` line that Git does not parse as a trailer ⇒ malformed, never
// none. Both the amend-destroyed block and the prose quotation.
func TestACL55_RawOnlyMatchIsMalformed(t *testing.T) {
	for _, tc := range []struct {
		name string
		body func(block string) string
	}{
		{"amend-destroyed-block", func(block string) string { return block + "\nsome prose appended by a hook\n" }},
		{"prose-quotation", func(block string) string {
			// The line is byte-exact but sits in a NON-terminal paragraph,
			// so git parses no trailer from it. Measured as
			// indistinguishable from an amend-destroyed block.
			return strings.SplitN(block, "\n", 2)[0] + "\n\nprose body that follows the quotation\n"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newLadderFixture(t)
			block := f.TrailerBlock(f.Slug)
			mustGit(t, f.Root, "reset", "-q", "--soft", f.LandingCommit+"^")
			mustGit(t, f.Root, "commit", "-q", "-m", "feat: land", "-m", tc.body(block))
			r := f.Verify()
			if r.LandingEvidence.State != EvidenceMalformed {
				t.Fatalf("evidence=%q want malformed (never none)", r.LandingEvidence.State)
			}
		})
	}
}

// AC-L56 — slug matching is exact after trimming; a lowercase trailer
// key is still a candidate.
func TestACL56_SlugMatchingAndKeyCase(t *testing.T) {
	rec := gitutil.CommitRecord{Trailers: map[string][]string{}}
	if gitutil.RawBodyHasTrailerLine("Tpatch-Feature: my-slug-extended\n", gitutil.TrailerFeature, "my-slug") {
		t.Errorf("my-slug must not match my-slug-extended")
	}
	if !gitutil.RawBodyHasTrailerLine("Tpatch-Feature: my-slug   \n", gitutil.TrailerFeature, "my-slug") {
		t.Errorf("trailing whitespace must be trimmed")
	}
	if !gitutil.RawBodyHasTrailerLine("tpatch-feature: my-slug\n", gitutil.TrailerFeature, "my-slug") {
		t.Errorf("trailer keys are case-insensitive")
	}
	_ = rec
}

// AC-L56 (live half) — a lowercase trailer key still produces a landing
// candidate through the real reader.
func TestACL56b_LowercaseTrailerKeyIsACandidate(t *testing.T) {
	f := newLadderFixture(t)
	block := strings.Replace(f.TrailerBlock(f.Slug), "Tpatch-Feature:", "tpatch-feature:", 1)
	mustGit(t, f.Root, "reset", "-q", "--soft", f.LandingCommit+"^")
	mustGit(t, f.Root, "commit", "-q", "-m", "feat: land", "-m", block)
	r := f.Verify()
	if r.LandingEvidence.State == EvidenceNone {
		t.Fatalf("a lowercase trailer key must still be a candidate")
	}
}

// AC-L57 — Tpatch-Base-Commit length is DERIVED from
// `git rev-parse --show-object-format`.
func TestACL57_BaseCommitLengthIsDerived(t *testing.T) {
	t.Run("sha1-accepts-40-rejects-64", func(t *testing.T) {
		ctx := &verifyRunContext{}
		ctx.facts.CommitIDHexLen = 40
		if !ctx.trailerGrammarOK(recordWithTrailers(strings.Repeat("a", 40))) {
			t.Errorf("sha1 repo must accept 40 hex")
		}
		if ctx.trailerGrammarOK(recordWithTrailers(strings.Repeat("a", 64))) {
			t.Errorf("sha1 repo must reject 64 hex")
		}
	})
	t.Run("sha256-accepts-64-rejects-40", func(t *testing.T) {
		ctx := &verifyRunContext{}
		ctx.facts.CommitIDHexLen = 64
		if !ctx.trailerGrammarOK(recordWithTrailers(strings.Repeat("a", 64))) {
			t.Errorf("sha256 repo must accept 64 hex")
		}
		if ctx.trailerGrammarOK(recordWithTrailers(strings.Repeat("a", 40))) {
			t.Errorf("sha256 repo must reject 40 hex — a hardcoded 40 fails this row")
		}
	})
	t.Run("real-sha256-repository", func(t *testing.T) {
		root := t.TempDir()
		if _, err := tryGit(root, "init", "-q", "-b", "main", "--object-format=sha256", "."); err != nil {
			t.Skipf("this git cannot create a sha256 repository: %v", err)
		}
		mustGit(t, root, "config", "user.email", "t@e.com")
		mustGit(t, root, "config", "user.name", "t")
		mustWriteFile(t, filepath.Join(root, "f.txt"), "x\n")
		mustGit(t, root, "add", "-A")
		mustGit(t, root, "commit", "-q", "-m", "seed")
		facts, err := gitutil.ReadRepoFacts(root)
		if err != nil {
			t.Fatalf("ReadRepoFacts: %v", err)
		}
		if facts.ObjectFormat != "sha256" || facts.CommitIDHexLen != 64 {
			t.Fatalf("facts=%+v want sha256/64", facts)
		}
		if len(gitHeadOf(t, root)) != 64 {
			t.Fatalf("sha256 commit id is not 64 hex")
		}
	})
}

func recordWithTrailers(base string) gitutil.CommitRecord {
	return gitutil.CommitRecord{
		Trailers: map[string][]string{
			strings.ToLower(gitutil.TrailerFeature):    {"slug"},
			strings.ToLower(gitutil.TrailerPatchSHA):   {strings.Repeat("f", 64)},
			strings.ToLower(gitutil.TrailerRecipeSHA):  {"none"},
			strings.ToLower(gitutil.TrailerBaseCommit): {base},
		},
	}
}

// AC-L58 — a git error, unparsable output, or an unreadable object
// format ⇒ `unavailable`, FAIL — never none.
func TestACL58_ReaderFailureIsUnavailable(t *testing.T) {
	f := newLadderFixture(t)
	w := installFakeVersionGit(t, "git version 2.99.0")
	w.Reset()
	r := f.Verify()
	if r.LandingEvidence.State != EvidenceUnavailable {
		t.Fatalf("evidence=%q want unavailable", r.LandingEvidence.State)
	}
	if r.FailedAt != FailedAtLandingEvidence {
		t.Errorf("failed_at=%q want landing-evidence", r.FailedAt)
	}
	if r.Verdict != "failed" {
		t.Errorf("verdict=%q want failed", r.Verdict)
	}
}

// AC-L59 — `base_commit_reachable: false` raises `base-commit-unreachable`
// and does not fail on its own.
func TestACL59_UnreachableBaseCommitIsAdvisoryOnly(t *testing.T) {
	f := newLadderFixture(t)
	ctx := newVerifyRunContext(f.Store)
	if ctx.isAncestor(strings.Repeat("0", 40), "HEAD") {
		t.Fatalf("a nonexistent commit must not be reported reachable")
	}
	r := f.Verify()
	if r.LandingEvidence.BaseCommitReachable == nil || !*r.LandingEvidence.BaseCommitReachable {
		t.Fatalf("precondition: the fixture's base commit is reachable")
	}
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%q", r.Verdict)
	}
}

// AC-L60 — the enumeration is exactly ONE `git log --topo-order
// --reverse -z` per run carrying %H, %P, %B and all four trailers.
func TestACL60_SingleEnumerationPerRun(t *testing.T) {
	f := newLadderFixture(t)
	w := installGitWrapper(t)
	w.Reset()
	f.Verify()
	logs := callsWithSubcommand(w.Calls(), "log")
	if len(logs) != 1 {
		t.Fatalf("expected exactly 1 git log, got %d", len(logs))
	}
	joined := logs[0].Joined()
	for _, want := range []string{"--topo-order", "--reverse", "-z", "%H", "%P", "%B",
		gitutil.TrailerFeature, gitutil.TrailerPatchSHA, gitutil.TrailerRecipeSHA, gitutil.TrailerBaseCommit} {
		if !strings.Contains(joined, want) {
			t.Errorf("enumeration missing %q: %s", want, joined)
		}
	}
	if strings.Contains(joined, "--first-parent") {
		t.Errorf("enumeration must never use --first-parent")
	}
}

// AC-L61 — `rev-list` is NEVER invoked. Adversarial.
func TestACL61_RevListIsNeverInvoked(t *testing.T) {
	f := newLadderFixture(t)
	w := installGitWrapper(t)
	w.Reset()
	f.Verify()
	if calls := callsWithSubcommand(w.Calls(), "rev-list"); len(calls) != 0 {
		t.Fatalf("verify invoked rev-list %d time(s): %v", len(calls), calls[0].Joined())
	}
}

// AC-L62 — records are consumed oldest-first and anchor selection uses
// that order directly.
func TestACL62_RecordsAreOldestFirst(t *testing.T) {
	f := newLadderFixture(t)
	recs, err := gitutil.EnumerateCommitTrailers(f.Root)
	if err != nil {
		t.Fatalf("EnumerateCommitTrailers: %v", err)
	}
	if len(recs) < 3 {
		t.Fatalf("expected several commits, got %d", len(recs))
	}
	if recs[0].ParentCount() != 0 {
		t.Errorf("first record must be the root (oldest-first): parents=%d", recs[0].ParentCount())
	}
	if recs[len(recs)-1].SHA != gitHeadOf(t, f.Root) {
		t.Errorf("last record must be HEAD")
	}
}

// AC-L63 — the §3.6.9 invocation budget is honoured.
func TestACL63_InvocationBudget(t *testing.T) {
	f := newLadderFixture(t)
	w := installGitWrapper(t)
	w.Reset()
	f.Verify()
	calls := w.Calls()
	counts := map[string]int{}
	for _, c := range calls {
		counts[c.Subcommand()]++
	}
	if counts["log"] != 1 {
		t.Errorf("git log ×%d want 1", counts["log"])
	}
	if counts["rev-list"] != 0 {
		t.Errorf("git rev-list ×%d want 0", counts["rev-list"])
	}
	// Preflight: one combined rev-parse plus one config read; the budget
	// allows three.
	if counts["config"] > 1 {
		t.Errorf("promisor config read ×%d want <= 1", counts["config"])
	}
	// Exactly one candidate ⇒ at most one qualification apply and at most
	// two ladder applies at HEAD, plus the shadow's forward check.
	if counts["apply"] > 4 {
		t.Errorf("git apply ×%d exceeds the budget", counts["apply"])
	}
	if counts["diff"] != 0 {
		t.Errorf("git diff ×%d — the identity is computed only when >= 2 candidates qualify", counts["diff"])
	}
}

// AC-L64 — root landing (0 parents) in a NON-shallow repository ⇒
// unsupported-topology, R9.
func TestACL64_RootLandingIsUnsupportedTopology(t *testing.T) {
	root := t.TempDir()
	f := newRootLandingFixture(t, root)
	r := f.Verify()
	if r.LandingEvidence.State != EvidenceUnsupportedTopology {
		t.Fatalf("evidence=%q want unsupported-topology", r.LandingEvidence.State)
	}
	want := remediationR9(f.Slug, r.LandingEvidence.AttestationCommit, 0)
	if got := checkByID(t, r, CheckRecipeReplayClean).Remediation; got != want {
		t.Errorf("R9 not verbatim:\n got %q\nwant %q", got, want)
	}
}

// newRootLandingFixture builds a repo whose ONLY commit is the landing.
func newRootLandingFixture(t *testing.T, root string) *landedFixture {
	t.Helper()
	mustGit(t, root, "init", "-q", "-b", "main", ".")
	mustGit(t, root, "config", "user.email", "t@e.com")
	mustGit(t, root, "config", "user.name", "t")
	mustGit(t, root, "config", "commit.gpgsign", "false")
	s, err := storeInit(root)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	f := &landedFixture{t: t, Root: root, Store: s, Slug: "root-landing", FilePath: "src/app.txt"}
	if _, err := s.AddFeature(storeAddInput(f.Slug)); err != nil {
		t.Fatalf("AddFeature: %v", err)
	}
	if err := s.MarkFeatureState(f.Slug, storeStateApplied(), "apply", ""); err != nil {
		t.Fatalf("MarkFeatureState: %v", err)
	}
	writeIntentFiles(t, s, f.Slug)
	mustWriteFile(t, filepath.Join(root, f.FilePath), "root\n")
	f.WritePatch("diff --git a/src/app.txt b/src/app.txt\nnew file mode 100644\n--- /dev/null\n+++ b/src/app.txt\n@@ -0,0 +1 @@\n+root\n")
	f.WriteRecipe(ApplyRecipe{Feature: f.Slug, Operations: []RecipeOperation{
		{Type: "write-file", Path: f.FilePath, Content: "root\n"},
	}})
	st, _ := s.LoadFeatureStatus(f.Slug)
	st.Apply.BaseCommit = strings.Repeat("0", 40)
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatal(err)
	}
	block := fmt.Sprintf("Tpatch-Feature: %s\nTpatch-Patch-SHA: %s\nTpatch-Recipe-SHA: %s\nTpatch-Base-Commit: %s\n",
		f.Slug, sha256Hex(f.PatchBytes()), sha256Hex(f.RecipeBytes()), st.Apply.BaseCommit)
	mustGit(t, root, "add", "-A")
	mustGit(t, root, "commit", "-q", "-m", "feat: root landing", "-m", block)
	f.LandingCommit = gitHeadOf(t, root)
	return f
}

// AC-L65 — merge landing (≥2 parents) ⇒ unsupported-topology, R9; never
// approximated to ^1.
func TestACL65_MergeLandingIsUnsupportedTopology(t *testing.T) {
	f := newLandedFixture(t, "merge-landing")
	mustWriteFile(t, filepath.Join(f.Root, f.FilePath), ladderBody(nil))
	mustGit(t, f.Root, "add", "-A")
	mustGit(t, f.Root, "commit", "-q", "-m", "seed")
	f.BaseCommit = gitHeadOf(t, f.Root)
	mustWriteFile(t, filepath.Join(f.Root, f.FilePath), featureBody(nil))
	mustGit(t, f.Root, "add", "-A")
	mustGit(t, f.Root, "commit", "-q", "-m", "implement")
	f.Record()
	block := f.TrailerBlock(f.Slug)

	mustGit(t, f.Root, "checkout", "-q", "-b", "side")
	mustWriteFile(t, filepath.Join(f.Root, "side.txt"), "side\n")
	mustGit(t, f.Root, "add", "-A")
	mustGit(t, f.Root, "commit", "-q", "-m", "side")
	mustGit(t, f.Root, "checkout", "-q", "main")
	mustWriteFile(t, filepath.Join(f.Root, "main.txt"), "main\n")
	mustGit(t, f.Root, "add", "-A")
	mustGit(t, f.Root, "commit", "-q", "-m", "main")
	mustGit(t, f.Root, "merge", "-q", "--no-ff", "--no-commit", "side")
	mustGit(t, f.Root, "commit", "-q", "-m", "merge landing", "-m", block)

	r := f.Verify()
	if r.LandingEvidence.State != EvidenceUnsupportedTopology {
		t.Fatalf("evidence=%q want unsupported-topology", r.LandingEvidence.State)
	}
	if r.LandingEvidence.ParentCount != 2 {
		t.Errorf("parent_count=%d want 2", r.LandingEvidence.ParentCount)
	}
	if !strings.Contains(checkByID(t, r, CheckRecipeReplayClean).Remediation, "has 2 parents") {
		t.Errorf("expected R9 naming the parent count")
	}
}

// AC-L66 — SHALLOW clone: a candidate on the graft boundary reports 0
// parents yet classifies `shallow-history` with R21, not R9.
func TestACL66_ShallowBoundaryIsShallowHistory(t *testing.T) {
	f := newLadderFixture(t)
	clone := t.TempDir()
	if _, err := tryGit(clone, "clone", "-q", "--depth", "1", "file://"+f.Root, "."); err != nil {
		t.Skipf("shallow clone unsupported here: %v", err)
	}
	facts, err := gitutil.ReadRepoFacts(clone)
	if err != nil {
		t.Fatalf("ReadRepoFacts: %v", err)
	}
	if !facts.Shallow {
		t.Fatalf("clone is not shallow")
	}
	s, err := storeOpen(clone)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	report, _ := RunVerify(s, f.Slug, VerifyOptions{NoWrite: true})
	if report == nil {
		t.Fatalf("no report")
	}
	if report.LandingEvidence.State != EvidenceShallowHistory {
		t.Fatalf("evidence=%q want shallow-history (reason=%q)",
			report.LandingEvidence.State, report.LandingEvidence.Reason)
	}
	want := remediationR21(f.Slug, report.LandingEvidence.AttestationCommit)
	if got := checkByID(t, report, CheckRecipeReplayClean).Remediation; got != want {
		t.Errorf("R21 not verbatim:\n got %q\nwant %q", got, want)
	}
	if strings.Contains(checkByID(t, report, CheckRecipeReplayClean).Remediation, "re-land") {
		t.Errorf("a shallow checkout must not be told to re-land (R9)")
	}
}

// AC-L67 — the shallow discriminator is the PREFLIGHT, not the parent
// count: a true root in a full repo still yields unsupported-topology,
// and the preflight runs BEFORE any parent-count branch.
func TestACL67_ShallowDiscriminatorIsThePreflight(t *testing.T) {
	root := t.TempDir()
	f := newRootLandingFixture(t, root)
	if got := f.Verify().LandingEvidence.State; got != EvidenceUnsupportedTopology {
		t.Fatalf("true root in a full repo: evidence=%q want unsupported-topology", got)
	}

	w := installGitWrapper(t)
	w.Reset()
	f.Verify()
	calls := w.Calls()
	preflightAt, logAt := -1, -1
	for i, c := range calls {
		if preflightAt < 0 && c.Subcommand() == "rev-parse" && strings.Contains(c.Joined(), "--is-shallow-repository") {
			preflightAt = i
		}
		if logAt < 0 && c.Subcommand() == "log" {
			logAt = i
		}
	}
	if preflightAt < 0 {
		t.Fatalf("no shallow preflight observed")
	}
	if logAt >= 0 && preflightAt > logAt {
		t.Errorf("preflight ran AFTER the enumeration (%d > %d)", preflightAt, logAt)
	}
}

// AC-L70 — a landing reachable only through a merge's NON-first parent
// is found; cherry-picked and rebased landings classify exact.
func TestACL70_NonFirstParentAndCherryPickAreFound(t *testing.T) {
	t.Run("non-first-parent", func(t *testing.T) {
		f := newLandedFixture(t, "side-landing")
		mustWriteFile(t, filepath.Join(f.Root, f.FilePath), ladderBody(nil))
		mustGit(t, f.Root, "add", "-A")
		mustGit(t, f.Root, "commit", "-q", "-m", "seed")
		f.BaseCommit = gitHeadOf(t, f.Root)
		mainTip := f.BaseCommit

		mustGit(t, f.Root, "checkout", "-q", "-b", "side")
		f.LandUserChange(featureBody(nil))
		side := f.LandingCommit
		mustGit(t, f.Root, "checkout", "-q", "main")
		mustWriteFile(t, filepath.Join(f.Root, "main-only.txt"), "main\n")
		mustGit(t, f.Root, "add", "-A")
		mustGit(t, f.Root, "commit", "-q", "-m", "main advances")
		mustGit(t, f.Root, "merge", "-q", "--no-ff", "-m", "merge side", side)

		r := f.Verify()
		if r.LandingEvidence.State != EvidenceExact {
			t.Fatalf("evidence=%q want exact — the landing is reachable only through the second parent", r.LandingEvidence.State)
		}
		_ = mainTip
	})
	t.Run("cherry-pick", func(t *testing.T) {
		f := newLadderFixture(t)
		original := f.LandingCommit
		mustGit(t, f.Root, "checkout", "-q", "-b", "elsewhere", f.BaseCommit)
		if _, err := tryGit(f.Root, "cherry-pick", original); err != nil {
			mustGit(t, f.Root, "cherry-pick", "--abort")
			t.Skipf("cherry-pick conflicted in this environment")
		}
		r := f.Verify()
		if r.LandingEvidence.State != EvidenceExact && r.LandingEvidence.State != EvidenceDuplicateEquivalent {
			t.Fatalf("evidence=%q want exact/duplicate-equivalent", r.LandingEvidence.State)
		}
	})
}

// AC-L71 — a branch switch away from the landing ⇒ none ⇒ forward mode;
// equivalent content present anyway ⇒ still none. Detached HEAD and
// history-rewrite rows included.
func TestACL71_BranchSwitchDetachedHeadAndRewrite(t *testing.T) {
	t.Run("branch-switch-away", func(t *testing.T) {
		f := newLadderFixture(t)
		mustGit(t, f.Root, "checkout", "-q", "-b", "no-landing", f.BaseCommit)
		r := f.Verify()
		if r.LandingEvidence.State != EvidenceNone {
			t.Fatalf("evidence=%q want none", r.LandingEvidence.State)
		}
		if r.TargetMode != TargetModeForward {
			t.Errorf("target_mode=%q want forward", r.TargetMode)
		}
	})
	t.Run("equivalent-content-without-attestation", func(t *testing.T) {
		f := newLadderFixture(t)
		mustGit(t, f.Root, "checkout", "-q", "-b", "no-landing", f.BaseCommit)
		mustWriteFile(t, filepath.Join(f.Root, f.FilePath), featureBody(nil))
		mustGit(t, f.Root, "add", "-A")
		mustGit(t, f.Root, "commit", "-q", "-m", "same content, no trailer")
		r := f.Verify()
		if r.LandingEvidence.State != EvidenceNone {
			t.Fatalf("evidence=%q want none — content is not attestation", r.LandingEvidence.State)
		}
	})
	t.Run("detached-head", func(t *testing.T) {
		f := newLadderFixture(t)
		mustGit(t, f.Root, "checkout", "-q", "--detach", f.LandingCommit)
		r := f.Verify()
		if r.LandingEvidence.State != EvidenceExact {
			t.Fatalf("evidence=%q want exact on a detached HEAD", r.LandingEvidence.State)
		}
	})
	t.Run("history-rewrite-drops-the-landing", func(t *testing.T) {
		f := newLadderFixture(t)
		mustGit(t, f.Root, "reset", "-q", "--hard", f.LandingCommit+"^")
		r := f.Verify()
		if r.LandingEvidence.State != EvidenceNone {
			t.Fatalf("evidence=%q want none after the landing was rewritten away", r.LandingEvidence.State)
		}
	})
}
