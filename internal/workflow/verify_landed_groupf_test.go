package workflow

// Acceptance rows AC-L72 – AC-L91 — PRD-verify-freshness §7.1 Group F:
// closure arbitration. The presence test for every member is the §3.6.5
// patch ladder at the anchor tree; recipe replay is what arbitration
// DECIDES about, never how it decides.

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// chainFixture builds a repo with several features and explicit hard
// dependencies so the arbitration table can be exercised member by
// member.
type chainFixture struct {
	t     *testing.T
	Store *store.Store
	Root  string
	Base  string
}

func newChainFixture(t *testing.T) *chainFixture {
	t.Helper()
	root := t.TempDir()
	gitInitVerifyTest(t, root)
	mustWriteFile(t, filepath.Join(root, "src", "app.txt"), ladderBody(nil))
	mustWriteFile(t, filepath.Join(root, "src", "parent.txt"), "parent-seed\n")
	mustGit(t, root, "add", "-A")
	mustGit(t, root, "commit", "-q", "-m", "seed")
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	mustGit(t, root, "add", "-A")
	mustGit(t, root, "commit", "-q", "-m", "tpatch scaffolding")
	return &chainFixture{t: t, Store: s, Root: root, Base: gitHeadOf(t, root)}
}

// Feature registers a feature in the given state with the given hard
// dependencies and intent files.
func (c *chainFixture) Feature(slug string, state store.FeatureState, hardDeps ...string) {
	c.t.Helper()
	if _, err := c.Store.AddFeature(store.AddFeatureInput{Title: slug, Slug: slug, Request: "x"}); err != nil {
		c.t.Fatalf("AddFeature %s: %v", slug, err)
	}
	if err := c.Store.MarkFeatureState(slug, state, "apply", ""); err != nil {
		c.t.Fatalf("MarkFeatureState %s: %v", slug, err)
	}
	writeIntentFiles(c.t, c.Store, slug)
	st, err := c.Store.LoadFeatureStatus(slug)
	if err != nil {
		c.t.Fatalf("load %s: %v", slug, err)
	}
	for _, dep := range hardDeps {
		st.DependsOn = append(st.DependsOn, store.Dependency{Slug: dep, Kind: store.DependencyKindHard})
	}
	st.Apply.BaseCommit = c.Base
	st.Apply.HasPatch = true
	if err := c.Store.SaveFeatureStatus(st); err != nil {
		c.t.Fatalf("save %s: %v", slug, err)
	}
}

// Artifacts writes a feature's canonical patch and recipe.
func (c *chainFixture) Artifacts(slug, patch string, recipe ApplyRecipe) {
	c.t.Helper()
	if err := c.Store.WriteArtifact(slug, "post-apply.patch", patch); err != nil {
		c.t.Fatalf("write patch %s: %v", slug, err)
	}
	if recipe.Feature == "" {
		recipe.Feature = slug
	}
	data := mustJSON(c.t, recipe)
	if err := c.Store.WriteArtifact(slug, "apply-recipe.json", data); err != nil {
		c.t.Fatalf("write recipe %s: %v", slug, err)
	}
}

// PatchFor captures the working-tree diff for the given paths.
func (c *chainFixture) PatchFor(paths ...string) string {
	c.t.Helper()
	return mustGit(c.t, c.Root, "diff", "--", strings.Join(paths, " ")) + "\n"
}

// Land commits `.tpatch/` plus the EXPLICITLY named paths with the
// feature's four-trailer block, producing a real landing. Explicit paths
// keep one feature's landing from sweeping in another's worktree state.
func (c *chainFixture) Land(slug string, paths ...string) string {
	c.t.Helper()
	block := trailerBlockFor(c.t, c.Store, c.Root, slug)
	landingCounter++
	mustWriteFile(c.t, filepath.Join(c.Root, ".tpatch", "features", slug,
		fmt.Sprintf("landing-marker-%d.txt", landingCounter)), "landed\n")
	args := append([]string{"add", "--", ".tpatch"}, paths...)
	mustGit(c.t, c.Root, args...)
	mustGit(c.t, c.Root, "commit", "-q", "-m", "feat: land "+slug, "-m", block)
	return gitHeadOf(c.t, c.Root)
}

func (c *chainFixture) Verify(slug string) *VerifyReport {
	c.t.Helper()
	r, err := RunVerify(c.Store, slug, VerifyOptions{NoWrite: true})
	if r == nil {
		c.t.Fatalf("RunVerify %s: %v", slug, err)
	}
	return r
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := jsonMarshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

// poisonRecipe is a recipe whose replay ALWAYS fails. A member that is
// correctly skipped by the patch ladder never executes it, so a passing
// run proves the skip and a failing run proves a replay happened.
func poisonRecipe(slug string) ApplyRecipe {
	return ApplyRecipe{Feature: slug, Operations: []RecipeOperation{
		{Type: "replace-in-file", Path: "src/parent.txt", Search: "TEXT-THAT-DOES-NOT-EXIST", Replace: "x"},
	}}
}

// buildLandedParentChain lands `parent` (patch materialized in history)
// and leaves `child` applied-and-unlanded on top of it.
func buildLandedParentChain(t *testing.T, parentRecipe ApplyRecipe, parentBody string) (*chainFixture, string, string) {
	t.Helper()
	c := newChainFixture(t)
	c.Feature("parent", store.StateApplied)
	c.Feature("child", store.StateApplied, "parent")

	// The parent's user-code change, captured as its canonical patch.
	mustWriteFile(t, filepath.Join(c.Root, "src", "parent.txt"), parentBody)
	patch := mustGit(t, c.Root, "diff", "--", "src/parent.txt") + "\n"
	c.Artifacts("parent", patch, parentRecipe)
	c.Land("parent", "src/parent.txt")

	// The child's change on top.
	mustWriteFile(t, filepath.Join(c.Root, "src", "app.txt"), featureBody(nil))
	childPatch := mustGit(t, c.Root, "diff", "--", "src/app.txt") + "\n"
	c.Artifacts("child", childPatch, ApplyRecipe{Feature: "child", Operations: []RecipeOperation{
		{Type: "write-file", Path: "src/app.txt", Content: featureBody(nil)},
	}})
	return c, "parent", "child"
}

// AC-L72 / AC-L73 — the presence test is the patch ladder; a landed
// member whose ladder is clean is SKIPPED and its recipe never executes.
func TestACL72_PresenceTestIsThePatchLadder(t *testing.T) {
	c, _, child := buildLandedParentChain(t, poisonRecipe("parent"), "parent-seed\nPARENT-CHANGE\n")
	r := c.Verify(child)
	if r.Verdict != "passed" {
		t.Fatalf("a landed parent's poison recipe was executed: verdict=%s v7=%q",
			r.Verdict, checkByID(t, r, CheckRecipeReplayClean).Remediation)
	}
}

func TestACL73_LandedMemberWithCleanLadderIsSkipped(t *testing.T) {
	c, _, child := buildLandedParentChain(t, poisonRecipe("parent"), "parent-seed\nPARENT-CHANGE\n")
	if r := c.Verify(child); r.Verdict != "passed" {
		t.Fatalf("verdict=%s — the parent's recipe must never be executed", r.Verdict)
	}
}

// AC-L74 — a landed member with an `append-file` recipe is skipped and
// the anchor tree contains its payload exactly ONCE.
func TestACL74_LandedAppendFileParentIsNotDoubleApplied(t *testing.T) {
	appendRecipe := ApplyRecipe{Feature: "parent", Operations: []RecipeOperation{
		{Type: "append-file", Path: "src/parent.txt", Content: "PARENT-APPENDED\n"},
	}}
	c, _, child := buildLandedParentChain(t, appendRecipe, "parent-seed\nPARENT-APPENDED\n")
	// The child's canonical patch is captured against a tree that has the
	// parent's payload exactly once; a double-append would break it.
	r := c.Verify(child)
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%s failed_at=%s — the append-file parent was replayed into the baseline",
			r.Verdict, r.FailedAt)
	}
}

// AC-L75 — a landed member with a `replace-in-file` recipe is skipped
// without `search text not found`.
func TestACL75_LandedReplaceInFileParentIsSkipped(t *testing.T) {
	replaceRecipe := ApplyRecipe{Feature: "parent", Operations: []RecipeOperation{
		{Type: "replace-in-file", Path: "src/parent.txt", Search: "parent-seed", Replace: "PARENT-CHANGE"},
	}}
	c, _, child := buildLandedParentChain(t, replaceRecipe, "PARENT-CHANGE\n")
	r := c.Verify(child)
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%s", r.Verdict)
	}
	if strings.Contains(checkByID(t, r, CheckRecipeReplayClean).Remediation, "search text not found") {
		t.Fatalf("the parent's replace-in-file recipe was replayed")
	}
}

// AC-L76 — a landed member whose ladder BLOCKS ⇒ fail-fast
// `parent-landing-drift` (R14) before the target is judged.
func TestACL76_LandedParentDriftFailsFast(t *testing.T) {
	c, parent, child := buildLandedParentChain(t, poisonRecipe("parent"), "parent-seed\nPARENT-CHANGE\n")
	// Revert the parent's content at HEAD.
	mustWriteFile(t, filepath.Join(c.Root, "src", "parent.txt"), "parent-seed\n")
	mustGit(t, c.Root, "add", "-A")
	mustGit(t, c.Root, "commit", "-q", "-m", "revert the parent")
	r := c.Verify(child)
	if r.FailedAt != FailedAtParentLandingDrift {
		t.Fatalf("failed_at=%q want parent-landing-drift", r.FailedAt)
	}
	if r.ParentSlug != parent {
		t.Errorf("parent_slug=%q want %q", r.ParentSlug, parent)
	}
	v7 := checkByID(t, r, CheckRecipeReplayClean)
	if !strings.HasPrefix(v7.Remediation, "hard parent parent landed at ") {
		t.Errorf("expected R14; got %q", v7.Remediation)
	}
}

// AC-L77 — an applied hard parent with evidence `none` whose patch
// ladder-passes is SKIPPED with a mandatory unattributed-materialized
// advisory (R18).
func TestACL77_UnattributedMaterializedParentIsSkipped(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("parent", store.StateApplied)
	c.Feature("child", store.StateApplied, "parent")
	mustWriteFile(t, filepath.Join(c.Root, "src", "parent.txt"), "parent-seed\nPARENT-CHANGE\n")
	patch := mustGit(t, c.Root, "diff", "--", "src/parent.txt") + "\n"
	c.Artifacts("parent", patch, poisonRecipe("parent"))
	// Commit the content with NO trailer at all: materialized, unattributed.
	mustGit(t, c.Root, "add", "-A")
	mustGit(t, c.Root, "commit", "-q", "-m", "someone else committed this")

	mustWriteFile(t, filepath.Join(c.Root, "src", "app.txt"), featureBody(nil))
	childPatch := mustGit(t, c.Root, "diff", "--", "src/app.txt") + "\n"
	c.Artifacts("child", childPatch, ApplyRecipe{Feature: "child", Operations: []RecipeOperation{
		{Type: "write-file", Path: "src/app.txt", Content: featureBody(nil)},
	}})

	r := c.Verify("child")
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%s failed_at=%s — the parent must be skipped, not replayed", r.Verdict, r.FailedAt)
	}
	a, ok := advisoryByCode(r, AdvisoryUnattributedMaterialized)
	if !ok {
		t.Fatalf("missing unattributed-materialized advisory; got %v", r.Advisories)
	}
	if a.Message != remediationR18("parent") {
		t.Errorf("R18 not verbatim:\n got %q\nwant %q", a.Message, remediationR18("parent"))
	}
	if a.Severity != SeverityWarn {
		t.Errorf("advisory severity=%q want warn", a.Severity)
	}
}

// AC-L78 — an applied hard parent with evidence `none` whose ladder
// BLOCKS, or which has no patch, is REPLAYED byte-identically to today.
func TestACL78_UnmaterializedParentIsReplayed(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("parent", store.StateApplied)
	c.Feature("child", store.StateApplied, "parent")
	// The parent's patch is NOT present in history.
	c.Artifacts("parent", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+PARENT-CHANGE\n",
		ApplyRecipe{Feature: "parent", Operations: []RecipeOperation{
			{Type: "append-file", Path: "src/parent.txt", Content: "PARENT-CHANGE\n"},
		}})
	// The child's patch is captured against the tree the parent's replay
	// produces.
	mustWriteFile(t, filepath.Join(c.Root, "src", "parent.txt"), "parent-seed\nPARENT-CHANGE\n")
	mustWriteFile(t, filepath.Join(c.Root, "src", "app.txt"), featureBody(nil))
	childPatch := mustGit(t, c.Root, "diff", "--", "src/app.txt") + "\n"
	c.Artifacts("child", childPatch, ApplyRecipe{Feature: "child", Operations: []RecipeOperation{
		{Type: "write-file", Path: "src/app.txt", Content: featureBody(nil)},
	}})
	mustGit(t, c.Root, "checkout", "--", "src/parent.txt", "src/app.txt")

	r := c.Verify("child")
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%s failed_at=%s v7=%q", r.Verdict, r.FailedAt,
			checkByID(t, r, CheckRecipeReplayClean).Remediation)
	}
	if hasAdvisory(r, AdvisoryUnattributedMaterialized) {
		t.Errorf("an unmaterialized parent must be replayed, not skipped-as-unattributed")
	}
}

// AC-L79 — a landed member with patch absent or present-empty ⇒
// `landed-artifacts-absent`, regardless of recipe shape.
func TestACL79_LandedMemberWithoutPatchIsTerminal(t *testing.T) {
	shapes := map[string]func(t *testing.T, c *chainFixture){
		"recipe-absent": func(t *testing.T, c *chainFixture) {
			mustRemove(t, artifactPath(c.Root, "parent", "apply-recipe.json"))
		},
		"recipe-empty": func(t *testing.T, c *chainFixture) {
			mustWriteFile(t, artifactPath(c.Root, "parent", "apply-recipe.json"), "  \n")
		},
		"recipe-zero-op": func(t *testing.T, c *chainFixture) {
			c.Artifacts("parent", "", ApplyRecipe{Feature: "parent"})
		},
		"recipe-with-ops": func(t *testing.T, c *chainFixture) {},
	}
	for _, patchState := range []string{"absent", "empty"} {
		for name, shape := range shapes {
			t.Run(patchState+"/"+name, func(t *testing.T) {
				c, _, child := buildLandedParentChain(t, poisonRecipe("parent"), "parent-seed\nPARENT-CHANGE\n")
				shape(t, c)
				if patchState == "absent" {
					mustRemove(t, artifactPath(c.Root, "parent", "post-apply.patch"))
				} else {
					mustWriteFile(t, artifactPath(c.Root, "parent", "post-apply.patch"), "")
				}
				r := c.Verify(child)
				if r.FailedAt != FailedAtLandedArtifacts {
					t.Fatalf("failed_at=%q want landed-artifacts-absent", r.FailedAt)
				}
				if got := checkByID(t, r, CheckRecipeReplayClean).Remediation; got != remediationR19("parent") {
					t.Errorf("R19 not verbatim:\n got %q\nwant %q", got, remediationR19("parent"))
				}
			})
		}
	}
}

// AC-L80 — a landed member with patch present-nonempty and recipe
// absent / present-empty / zero-op ⇒ the patch ladder is sole authority
// and the member is still skipped.
func TestACL80_PatchIsSoleAuthorityWhenRecipeIsUnusable(t *testing.T) {
	for _, name := range []string{"absent", "empty", "zero-op"} {
		t.Run(name, func(t *testing.T) {
			c, _, child := buildLandedParentChain(t, poisonRecipe("parent"), "parent-seed\nPARENT-CHANGE\n")
			switch name {
			case "absent":
				mustRemove(t, artifactPath(c.Root, "parent", "apply-recipe.json"))
			case "empty":
				mustWriteFile(t, artifactPath(c.Root, "parent", "apply-recipe.json"), "   \n")
			case "zero-op":
				mustWriteFile(t, artifactPath(c.Root, "parent", "apply-recipe.json"),
					mustJSON(t, ApplyRecipe{Feature: "parent"}))
			}
			// Re-attest: the recipe shape (not a stale digest) is what
			// this row exercises. Only `.tpatch/` is staged, so the
			// child's un-landed worktree change stays out of history.
			c.Land("parent")
			r := c.Verify(child)
			if r.Verdict != "passed" {
				t.Fatalf("verdict=%s failed_at=%s — the patch ladder is the sole authority here",
					r.Verdict, r.FailedAt)
			}
		})
	}
}

// AC-L81 — a landed member with NO usable artifact ⇒ FAIL
// `landed-artifacts-absent` (R19); never skipped, never replayed.
func TestACL81_LandedMemberWithNoUsableArtifact(t *testing.T) {
	c, _, child := buildLandedParentChain(t, poisonRecipe("parent"), "parent-seed\nPARENT-CHANGE\n")
	mustRemove(t, artifactPath(c.Root, "parent", "post-apply.patch"))
	mustRemove(t, artifactPath(c.Root, "parent", "apply-recipe.json"))
	r := c.Verify(child)
	if r.FailedAt != FailedAtLandedArtifacts {
		t.Fatalf("failed_at=%q want landed-artifacts-absent", r.FailedAt)
	}
	if checkByID(t, r, CheckRecipeReplayClean).Skipped {
		t.Errorf("the member must not be skipped")
	}
}

// AC-L82 — a hard parent with any terminal non-`exact` evidence state ⇒
// fail-fast `parent-evidence-integrity` (R15).
func TestACL82_ParentEvidenceIntegrityFailsFast(t *testing.T) {
	c, _, child := buildLandedParentChain(t, poisonRecipe("parent"), "parent-seed\nPARENT-CHANGE\n")
	// Drift the parent's artifacts ⇒ its evidence becomes `stale`.
	mustWriteFile(t, artifactPath(c.Root, "parent", "post-apply.patch"),
		readFileString(t, artifactPath(c.Root, "parent", "post-apply.patch"))+"\n")
	r := c.Verify(child)
	if r.FailedAt != FailedAtParentEvidence {
		t.Fatalf("failed_at=%q want parent-evidence-integrity", r.FailedAt)
	}
	want := remediationR15("parent", EvidenceStale, child)
	if got := checkByID(t, r, CheckRecipeReplayClean).Remediation; got != want {
		t.Errorf("R15 not verbatim:\n got %q\nwant %q", got, want)
	}
}

// AC-L83 — a hard parent in `unapplied` ⇒ `parent-unapplied` (R16).
func TestACL83_UnappliedParentFailsWithR16(t *testing.T) {
	c, _, child := buildLandedParentChain(t, poisonRecipe("parent"), "parent-seed\nPARENT-CHANGE\n")
	setFeatureState(t, c.Store, "parent", store.StateUnapplied)
	r := c.Verify(child)
	if r.FailedAt != FailedAtParentUnapplied {
		t.Fatalf("failed_at=%q want parent-unapplied", r.FailedAt)
	}
	want := remediationR16("parent", child)
	if got := checkByID(t, r, CheckRecipeReplayClean).Remediation; got != want {
		t.Errorf("R16 not verbatim:\n got %q\nwant %q", got, want)
	}
}

// AC-L84 — a hard parent in `rejected` ⇒ `parent-rejected` (R17).
func TestACL84_RejectedParentFailsWithR17(t *testing.T) {
	c, _, child := buildLandedParentChain(t, poisonRecipe("parent"), "parent-seed\nPARENT-CHANGE\n")
	setFeatureState(t, c.Store, "parent", store.StateRejected)

	// Shipped precedence, unchanged by this amendment: V4
	// (dep_metadata_valid) already blocks a rejected hard parent
	// (ADR-031), so the run fails before the dynamic phase.
	r := c.Verify(child)
	if r.Verdict != "failed" {
		t.Fatalf("verdict=%s want failed", r.Verdict)
	}
	if v4 := checkByID(t, r, CheckDepMetadataValid); v4.Passed {
		t.Errorf("V4 must keep its pre-existing rejected-parent block")
	}

	// The arbitration branch itself is exercised directly, because the
	// static precedence above means it is otherwise unreachable.
	ctx := newVerifyRunContext(c.Store)
	status, err := c.Store.LoadFeatureStatus(child)
	if err != nil {
		t.Fatal(err)
	}
	in := anchoredInput{ctx: ctx, store: c.Store, slug: child, status: status,
		entry: inventoryEntryOrEmpty(ctx, child), evidence: ctx.classifyEvidence(child)}
	got := arbitrateClosure(in, []string{"parent", child}, "HEAD", c.Root, ModeForward, ModeForward, ModeForward)
	if got == nil {
		t.Fatalf("arbitration did not fail-fast on a rejected hard parent")
	}
	if got.failedAt != FailedAtParentRejected {
		t.Fatalf("failed_at=%q want parent-rejected", got.failedAt)
	}
	if got.v7.Remediation != remediationR17("parent", child) {
		t.Errorf("R17 not verbatim:\n got %q\nwant %q", got.v7.Remediation, remediationR17("parent", child))
	}
}

// AC-L85 — a hard parent in `upstream_merged` is skipped byte-identically
// to today; a superseded parent stays excluded by the existing filter.
func TestACL85_UpstreamMergedAndSupersededParentsAreSkipped(t *testing.T) {
	t.Run("upstream-merged", func(t *testing.T) {
		c, _, child := buildLandedParentChain(t, poisonRecipe("parent"), "parent-seed\nPARENT-CHANGE\n")
		setFeatureState(t, c.Store, "parent", store.StateUpstreamMerged)
		if r := c.Verify(child); r.Verdict != "passed" {
			t.Fatalf("verdict=%s failed_at=%s", r.Verdict, r.FailedAt)
		}
	})
	t.Run("superseded", func(t *testing.T) {
		c, _, child := buildLandedParentChain(t, poisonRecipe("parent"), "parent-seed\nPARENT-CHANGE\n")
		c.Feature("superseder", store.StateApplied)
		st, _ := c.Store.LoadFeatureStatus("superseder")
		st.DependsOn = append(st.DependsOn, store.Dependency{Slug: "parent", Kind: store.DependencyKindSupersedes})
		if err := c.Store.SaveFeatureStatus(st); err != nil {
			t.Fatal(err)
		}
		if r := c.Verify(child); r.Verdict != "passed" {
			t.Fatalf("verdict=%s failed_at=%s", r.Verdict, r.FailedAt)
		}
	})
}

// AC-L86 — a hard parent in `active` is treated EXACTLY as `applied`,
// including for a NON-landed target, where it changes today's verdict.
func TestACL86_ActiveParentIsTreatedAsApplied(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("parent", store.StateActive)
	c.Feature("child", store.StateApplied, "parent")
	c.Artifacts("parent", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+PARENT-CHANGE\n",
		ApplyRecipe{Feature: "parent", Operations: []RecipeOperation{
			{Type: "append-file", Path: "src/parent.txt", Content: "PARENT-CHANGE\n"},
		}})
	mustWriteFile(t, filepath.Join(c.Root, "src", "parent.txt"), "parent-seed\nPARENT-CHANGE\n")
	mustWriteFile(t, filepath.Join(c.Root, "src", "app.txt"), featureBody(nil))
	childPatch := mustGit(t, c.Root, "diff", "--", "src/app.txt") + "\n"
	c.Artifacts("child", childPatch, ApplyRecipe{Feature: "child", Operations: []RecipeOperation{
		{Type: "write-file", Path: "src/app.txt", Content: featureBody(nil)},
	}})
	mustGit(t, c.Root, "checkout", "--", "src/parent.txt", "src/app.txt")

	r := c.Verify("child")
	if r.Verdict != "passed" {
		t.Fatalf("an `active` hard parent must arbitrate exactly like `applied`: verdict=%s v7=%q",
			r.Verdict, checkByID(t, r, CheckRecipeReplayClean).Remediation)
	}
	if r.TargetMode != TargetModeForward {
		t.Fatalf("this row must exercise a NON-landed target; target_mode=%s", r.TargetMode)
	}
}

// AC-L87 — after AC-L86 all four `active` sites agree.
func TestACL87_AllFourActiveSitesAgree(t *testing.T) {
	// 1 + 2 — the two state-set predicates.
	if !postApplyVerifyStates()[store.StateActive] {
		t.Errorf("postApplyVerifyStates rejects active (verify.go)")
	}
	if !isPostApplyState(store.StateActive) {
		t.Errorf("isPostApplyState rejects active (verify_all.go)")
	}

	// 3 — CheckDependencyGate, behaviourally.
	c := newChainFixture(t)
	c.Feature("parent", store.StateActive)
	c.Feature("child", store.StateApplied, "parent")
	if err := CheckDependencyGate(c.Store, "child"); err != nil {
		t.Errorf("CheckDependencyGate rejects an active hard parent: %v", err)
	}

	// 4 — the closure arbitration switch, behaviourally: an `active`
	// parent must never reach the fail-fast default.
	c.Artifacts("parent", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+PARENT-CHANGE\n",
		ApplyRecipe{Feature: "parent", Operations: []RecipeOperation{
			{Type: "append-file", Path: "src/parent.txt", Content: "PARENT-CHANGE\n"},
		}})
	c.Artifacts("child", "diff --git a/src/child.txt b/src/child.txt\nnew file mode 100644\n--- /dev/null\n+++ b/src/child.txt\n@@ -0,0 +1 @@\n+child\n",
		ApplyRecipe{Feature: "child", Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/child.txt", Content: "child\n"},
		}})
	r := c.Verify("child")
	if r.FailedAt == FailedAtParentReplay &&
		strings.Contains(checkByID(t, r, CheckRecipeReplayClean).Remediation, "need applied or upstream_merged") {
		t.Errorf("the closure switch still fail-fasts on active: %q",
			checkByID(t, r, CheckRecipeReplayClean).Remediation)
	}
}

// AC-L88 — a revert landing AFTER the anchor is invisible at anchor H
// and caught at anchor C; one predating the anchor is caught at anchor H.
func TestACL88_RevertTimingIsReportedAtBothAnchors(t *testing.T) {
	t.Run("revert-after-the-anchor", func(t *testing.T) {
		f := newLadderFixture(t)
		f.EditTracked(ladderBody(nil), "revert after the landing")
		r := f.Verify()
		if currentAnchor(t, r) != CurrentAbsent {
			t.Fatalf("anchor C must catch a later revert; current=%q", currentAnchor(t, r))
		}
		if got := checkByID(t, r, CheckPostApplyPatchReplayClean).AnchorResults["historical"]; got != "passed" {
			t.Errorf("anchor H must be blind to a LATER revert; historical=%q", got)
		}
	})
	t.Run("revert-before-the-anchor", func(t *testing.T) {
		c, parent, child := buildLandedParentChain(t, poisonRecipe("parent"), "parent-seed\nPARENT-CHANGE\n")
		mustWriteFile(t, filepath.Join(c.Root, "src", "parent.txt"), "parent-seed\n")
		mustGit(t, c.Root, "add", "-A")
		mustGit(t, c.Root, "commit", "-q", "-m", "revert the parent before the child's anchor")
		r := c.Verify(child)
		if r.FailedAt != FailedAtParentLandingDrift || r.ParentSlug != parent {
			t.Fatalf("failed_at=%q parent=%q want parent-landing-drift/%s", r.FailedAt, r.ParentSlug, parent)
		}
	})
}

// AC-L89 — a parent landed AFTER vs BEFORE the target yields identical
// verdicts; closure ordering for an all-unlanded closure is unchanged.
func TestACL89_ParentLandingOrderDoesNotChangeTheVerdict(t *testing.T) {
	// The parent's CONTENT is materialized before the child is recorded
	// in both arms; only the position of the parent's ATTESTATION commit
	// relative to the child's varies.
	build := func(t *testing.T, parentAttestsFirst bool) *VerifyReport {
		c := newChainFixture(t)
		c.Feature("parent", store.StateApplied)
		c.Feature("child", store.StateApplied, "parent")

		mustWriteFile(t, filepath.Join(c.Root, "src", "parent.txt"), "parent-seed\nPARENT-CHANGE\n")
		parentPatch := mustGit(t, c.Root, "diff", "--", "src/parent.txt") + "\n"
		c.Artifacts("parent", parentPatch, poisonRecipe("parent"))
		mustGit(t, c.Root, "add", "-A")
		mustGit(t, c.Root, "commit", "-q", "-m", "materialize the parent")

		mustWriteFile(t, filepath.Join(c.Root, "src", "app.txt"), featureBody(nil))
		childPatch := mustGit(t, c.Root, "diff", "--", "src/app.txt") + "\n"
		c.Artifacts("child", childPatch, ApplyRecipe{Feature: "child", Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/app.txt", Content: featureBody(nil)},
		}})
		if parentAttestsFirst {
			c.Land("parent")
			c.Land("child", "src/app.txt")
		} else {
			c.Land("child", "src/app.txt")
			c.Land("parent")
		}
		return c.Verify("child")
	}
	a := build(t, true)
	b := build(t, false)
	if a.Verdict != b.Verdict {
		t.Fatalf("verdicts differ by attestation order: %s (failed_at=%s) vs %s (failed_at=%s)",
			a.Verdict, a.FailedAt, b.Verdict, b.FailedAt)
	}
	if a.Verdict != "passed" {
		t.Fatalf("both arms should pass; got %s / %s", a.FailedAt, b.FailedAt)
	}
}

// AC-L90 — mixed chain: target unlanded, P1 landed, P2 applied-unlanded
// ⇒ anchor HEAD, P1 ladder-skipped, P2 replayed, target forward-verified.
func TestACL90_MixedChainUnlandedTarget(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("p1", store.StateApplied)
	c.Feature("p2", store.StateApplied)
	c.Feature("target", store.StateApplied, "p1", "p2")

	mustWriteFile(t, filepath.Join(c.Root, "src", "parent.txt"), "parent-seed\nP1\n")
	p1Patch := mustGit(t, c.Root, "diff", "--", "src/parent.txt") + "\n"
	c.Artifacts("p1", p1Patch, poisonRecipe("p1"))
	c.Land("p1", "src/parent.txt")

	// P2 is applied but unlanded and NOT materialized ⇒ replayed.
	c.Artifacts("p2", "diff --git a/src/p2.txt b/src/p2.txt\nnew file mode 100644\n--- /dev/null\n+++ b/src/p2.txt\n@@ -0,0 +1 @@\n+p2\n",
		ApplyRecipe{Feature: "p2", Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/p2.txt", Content: "p2\n"},
		}})

	mustWriteFile(t, filepath.Join(c.Root, "src", "app.txt"), featureBody(nil))
	targetPatch := mustGit(t, c.Root, "diff", "--", "src/app.txt") + "\n"
	c.Artifacts("target", targetPatch, ApplyRecipe{Feature: "target", Operations: []RecipeOperation{
		{Type: "write-file", Path: "src/app.txt", Content: featureBody(nil)},
	}})
	mustGit(t, c.Root, "checkout", "--", "src/app.txt")

	r := c.Verify("target")
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%s failed_at=%s v7=%q", r.Verdict, r.FailedAt,
			checkByID(t, r, CheckRecipeReplayClean).Remediation)
	}
	if r.TargetMode != TargetModeForward {
		t.Errorf("target_mode=%q want forward", r.TargetMode)
	}
	if r.Baseline.Mode != BaselineModeHead {
		t.Errorf("baseline.mode=%q want head-anchored", r.Baseline.Mode)
	}
}

// AC-L91 — mixed chain: target landed, P1 applied-unlanded ⇒ the anchor
// is the target's replay-anchor parent tree, P1 is replayed there, and
// the target is judged at both anchors.
func TestACL91_MixedChainLandedTarget(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("p1", store.StateApplied)
	c.Feature("target", store.StateApplied, "p1")
	// P1 is unlanded and not materialized ⇒ replayed into the shadow.
	c.Artifacts("p1", "diff --git a/src/p1.txt b/src/p1.txt\nnew file mode 100644\n--- /dev/null\n+++ b/src/p1.txt\n@@ -0,0 +1 @@\n+p1\n",
		ApplyRecipe{Feature: "p1", Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/p1.txt", Content: "p1\n"},
		}})

	mustWriteFile(t, filepath.Join(c.Root, "src", "app.txt"), featureBody(nil))
	targetPatch := mustGit(t, c.Root, "diff", "--", "src/app.txt") + "\n"
	c.Artifacts("target", targetPatch, ApplyRecipe{Feature: "target", Operations: []RecipeOperation{
		{Type: "write-file", Path: "src/app.txt", Content: featureBody(nil)},
	}})
	c.Land("target", "src/app.txt")

	r := c.Verify("target")
	if r.TargetMode != TargetModeLanded {
		t.Fatalf("target_mode=%q want landed", r.TargetMode)
	}
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%s failed_at=%s v7=%q", r.Verdict, r.FailedAt,
			checkByID(t, r, CheckRecipeReplayClean).Remediation)
	}
	ar := checkByID(t, r, CheckPostApplyPatchReplayClean).AnchorResults
	if ar["historical"] != "passed" || ar["current"] != CurrentMaterializedClean {
		t.Errorf("anchor_results=%v — both anchors must be reported", ar)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────

func setFeatureState(t *testing.T, s *store.Store, slug string, state store.FeatureState) {
	t.Helper()
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatalf("load %s: %v", slug, err)
	}
	st.State = state
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatalf("save %s: %v", slug, err)
	}
}

func mustRemove(t *testing.T, path string) {
	t.Helper()
	if err := removeIfExists(path); err != nil {
		t.Fatalf("remove %s: %v", path, err)
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := readFileBytes(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
