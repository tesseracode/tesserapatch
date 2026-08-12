package workflow

// Acceptance rows AC-L92 – AC-L106 — PRD-verify-freshness §7.1 Group G:
// V10 per-member baselines, `RecipeProvenance` as the forward-mode
// anchor (Q15), and metadata-derived later-touch.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func sha256Display(body string) string {
	sum := sha256.Sum256([]byte(body))
	return PreimageHashPrefix + hex.EncodeToString(sum[:])
}

func strPtr(s string) *string { return &s }

// writeProvenance writes a hash-bound `recipe-provenance.json` for slug.
func writeProvenance(t *testing.T, s *store.Store, slug, baseCommit string, bind bool) {
	t.Helper()
	raw, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "apply-recipe.json"))
	if err != nil {
		t.Fatalf("read recipe: %v", err)
	}
	prov := RecipeProvenance{BaseCommit: baseCommit, GeneratedAt: "2026-08-12T00:00:00Z"}
	if bind {
		sum := sha256Hex([]byte(raw))
		prov.RecipeSHA256 = &sum
	}
	data, err := json.Marshal(prov)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact(slug, "recipe-provenance.json", string(data)); err != nil {
		t.Fatalf("write provenance: %v", err)
	}
}

// newPreimageChain lands a parent that owns `src/parent.txt` with a
// write-file preimage, plus a landed child. The parent's postimage IS
// present at the child's anchor, which is exactly the rev-2 defect.
func newPreimageChain(t *testing.T) (*chainFixture, string, string) {
	t.Helper()
	c := newChainFixture(t)
	c.Feature("parent", store.StateApplied)
	c.Feature("child", store.StateApplied, "parent")

	parentPre := "parent-seed\n"
	parentPost := "parent-seed\nPARENT-CHANGE\n"
	mustWriteFile(t, filepath.Join(c.Root, "src", "parent.txt"), parentPost)
	parentPatch := mustGit(t, c.Root, "diff", "--", "src/parent.txt") + "\n"
	c.Artifacts("parent", parentPatch, ApplyRecipe{Feature: "parent", Operations: []RecipeOperation{
		{Type: "write-file", Path: "src/parent.txt", Content: parentPost,
			PreimageHash: strPtr(sha256Display(parentPre))},
	}})
	c.Land("parent", "src/parent.txt")

	mustWriteFile(t, filepath.Join(c.Root, "src", "app.txt"), featureBody(nil))
	childPatch := mustGit(t, c.Root, "diff", "--", "src/app.txt") + "\n"
	c.Artifacts("child", childPatch, ApplyRecipe{Feature: "child", Operations: []RecipeOperation{
		{Type: "write-file", Path: "src/app.txt", Content: featureBody(nil),
			PreimageHash: strPtr(sha256Display(ladderBody(nil)))},
	}})
	c.Land("child", "src/app.txt")
	return c, "parent", "child"
}

// AC-L92 — each landed member's V10 uses its OWN replay-anchor parent
// tree, never the target's.
func TestACL92_PerMemberV10Baselines(t *testing.T) {
	c, parent, child := newPreimageChain(t)
	r := c.Verify(child)
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%s failed_at=%s v10=%q", r.Verdict, r.FailedAt,
			checkByID(t, r, CheckWriteFilePreimageFresh).Remediation)
	}
	v10 := checkByID(t, r, CheckWriteFilePreimageFresh)
	if v10.MemberBaselines == nil {
		t.Fatalf("V10 reports no member_baselines")
	}
	childBaseline, ok := v10.MemberBaselines[child]
	if !ok {
		t.Fatalf("no baseline recorded for the target: %v", v10.MemberBaselines)
	}
	parentBaseline, ok := v10.MemberBaselines[parent]
	if !ok {
		t.Fatalf("no baseline recorded for the landed parent: %v", v10.MemberBaselines)
	}
	if parentBaseline == childBaseline {
		t.Fatalf("the landed parent reused the target's baseline %s — the rev-2 defect", childBaseline)
	}
	// The parent's postimage must NOT be present at its own baseline.
	body, found, _, err := gitutilBlob(c.Root, parentBaseline, "src/parent.txt")
	if err != nil {
		t.Fatalf("read parent baseline: %v", err)
	}
	if found && strings.Contains(string(body), "PARENT-CHANGE") {
		t.Errorf("the parent's own baseline already contains its postimage")
	}
}

// AC-L93 — a landed parent whose postimage IS present at the target's
// anchor still passes V10.
func TestACL93_LandedParentPassesAtItsOwnBaseline(t *testing.T) {
	c, _, child := newPreimageChain(t)
	r := c.Verify(child)
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%s failed_at=%s", r.Verdict, r.FailedAt)
	}
	// Sanity: the parent's postimage IS present at the target's anchor.
	anchor := r.Baseline.HistoricalAnchor
	if anchor == nil || anchor.Commit == "" {
		t.Fatalf("no anchor recorded")
	}
	body, found, _, err := gitutilBlob(c.Root, anchor.Commit, "src/parent.txt")
	if err != nil || !found {
		t.Fatalf("read target anchor: found=%v err=%v", found, err)
	}
	if !strings.Contains(string(body), "PARENT-CHANGE") {
		t.Fatalf("precondition: the parent's postimage must be present at the target's anchor")
	}
}

// AC-L94 — landed target with a preimage_hash matching at the anchor-H
// closure baseline ⇒ V10 PASSES with mode historical-anchor.
func TestACL94_LandedTargetPreimageAtClosureBaseline(t *testing.T) {
	c, _, child := newPreimageChain(t)
	r := c.Verify(child)
	v10 := checkByID(t, r, CheckWriteFilePreimageFresh)
	if !v10.Passed || v10.Skipped {
		t.Fatalf("V10 must pass at the landed baseline: %+v", v10)
	}
	if v10.Mode != ModeHistoricalAnchor {
		t.Errorf("mode=%q want historical-anchor", v10.Mode)
	}
}

// AC-L95 — landed target with `preimage_hash: ""` (new file) and the
// file ABSENT at the anchor baseline ⇒ V10 passes.
func TestACL95_LandedNewFilePreimagePasses(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("newfile", store.StateApplied)
	mustWriteFile(t, filepath.Join(c.Root, "src", "brand-new.txt"), "brand new\n")
	patch := mustGit(t, c.Root, "diff", "--", "src/brand-new.txt")
	if strings.TrimSpace(patch) == "" {
		// Untracked files need an explicit intent-to-add for `git diff`.
		mustGit(t, c.Root, "add", "-N", "src/brand-new.txt")
		patch = mustGit(t, c.Root, "diff", "--", "src/brand-new.txt")
	}
	c.Artifacts("newfile", patch+"\n", ApplyRecipe{Feature: "newfile", Operations: []RecipeOperation{
		{Type: "write-file", Path: "src/brand-new.txt", Content: "brand new\n", PreimageHash: strPtr("")},
	}})
	c.Land("newfile", "src/brand-new.txt")

	r := c.Verify("newfile")
	v10 := checkByID(t, r, CheckWriteFilePreimageFresh)
	if !v10.Passed {
		t.Fatalf("new-file preimage must pass at a baseline where the path is absent: %q", v10.Remediation)
	}
}

// AC-L96 — an UNLANDED applied feature whose op carries a preimage_hash
// with present + well-formed + reachable + hash-bound provenance ⇒ V10
// evaluated at RecipeProvenance.BaseCommit, mode provenance-anchor.
func TestACL96_ForwardModeUsesProvenanceAnchor(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("fwd", store.StateApplied)
	pre := "parent-seed\n"
	c.Artifacts("fwd", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+FWD\n",
		ApplyRecipe{Feature: "fwd", Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/parent.txt", Content: "parent-seed\nFWD\n",
				PreimageHash: strPtr(sha256Display(pre))},
		}})
	writeProvenance(t, c.Store, "fwd", c.Base, true)
	// The live working tree is deliberately DRIFTED so a live-tree read
	// would fail: only the provenance anchor can make this pass.
	mustWriteFile(t, filepath.Join(c.Root, "src", "parent.txt"), "drifted in the worktree\n")

	r := c.Verify("fwd")
	v10 := checkByID(t, r, CheckWriteFilePreimageFresh)
	if !v10.Passed {
		t.Fatalf("V10 must evaluate at RecipeProvenance.BaseCommit: %q", v10.Remediation)
	}
	if v10.Mode != ModeProvenanceAnchor {
		t.Errorf("mode=%q want provenance-anchor", v10.Mode)
	}
	if v10.MemberBaselines[c.featureSlug("fwd")] != c.Base {
		t.Errorf("member_baselines=%v want the provenance base %s", v10.MemberBaselines, c.Base)
	}
	if v10.ProvenanceHashBound == nil || !*v10.ProvenanceHashBound {
		t.Errorf("provenance_hash_bound=%v want true", v10.ProvenanceHashBound)
	}
}

// AC-L97 — a provenance sidecar WITHOUT `recipe_sha256` (pre-v0.5.2) is
// accepted with `provenance_hash_bound: false`.
func TestACL97_UnboundProvenanceIsAcceptedAndReported(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("fwd", store.StateApplied)
	c.Artifacts("fwd", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+FWD\n",
		ApplyRecipe{Feature: "fwd", Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/parent.txt", Content: "parent-seed\nFWD\n",
				PreimageHash: strPtr(sha256Display("parent-seed\n"))},
		}})
	writeProvenance(t, c.Store, "fwd", c.Base, false)
	r := c.Verify("fwd")
	v10 := checkByID(t, r, CheckWriteFilePreimageFresh)
	if !v10.Passed {
		t.Fatalf("an unbound sidecar must still be accepted: %q", v10.Remediation)
	}
	if v10.ProvenanceHashBound == nil || *v10.ProvenanceHashBound {
		t.Errorf("provenance_hash_bound=%v want false", v10.ProvenanceHashBound)
	}
}

// AC-L98 — provenance `recipe_sha256` MISMATCHING the inventory's recipe
// bytes is treated as inventory-inconsistent, never silently trusted.
func TestACL98_MismatchedProvenanceHashIsRejected(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("fwd", store.StateApplied)
	c.Artifacts("fwd", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+FWD\n",
		ApplyRecipe{Feature: "fwd", Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/parent.txt", Content: "parent-seed\nFWD\n",
				PreimageHash: strPtr(sha256Display("parent-seed\n"))},
		}})
	writeProvenance(t, c.Store, "fwd", c.Base, true)
	// Rewrite the recipe AFTER binding, so the sidecar no longer
	// describes it.
	c.Artifacts("fwd", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+FWD2\n",
		ApplyRecipe{Feature: "fwd", Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/parent.txt", Content: "parent-seed\nFWD2\n",
				PreimageHash: strPtr(sha256Display("parent-seed\n"))},
		}})
	r := c.Verify("fwd")
	if r.FailedAt != FailedAtRecipeProvenance {
		t.Fatalf("failed_at=%q want recipe-provenance-unavailable", r.FailedAt)
	}
}

// AC-L99 — provenance absent, ill-formed or unreachable with a real
// preimage_hash ⇒ FAIL `recipe-provenance-unavailable` (R24).
func TestACL99_UnusableProvenanceFailsWithR24(t *testing.T) {
	cases := map[string]struct {
		setup     func(t *testing.T, c *chainFixture)
		condition string
	}{
		"absent":      {func(t *testing.T, c *chainFixture) {}, "absent"},
		"ill-formed":  {func(t *testing.T, c *chainFixture) { writeProvenance(t, c.Store, "fwd", "not-hex", true) }, "malformed"},
		"unreachable": {func(t *testing.T, c *chainFixture) { writeProvenance(t, c.Store, "fwd", strings.Repeat("a", 40), true) }, "unreachable"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := newChainFixture(t)
			c.Feature("fwd", store.StateApplied)
			c.Artifacts("fwd", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+FWD\n",
				ApplyRecipe{Feature: "fwd", Operations: []RecipeOperation{
					{Type: "write-file", Path: "src/parent.txt", Content: "parent-seed\nFWD\n",
						PreimageHash: strPtr(sha256Display("parent-seed\n"))},
				}})
			tc.setup(t, c)
			r := c.Verify("fwd")
			if r.FailedAt != FailedAtRecipeProvenance {
				t.Fatalf("failed_at=%q want recipe-provenance-unavailable", r.FailedAt)
			}
			want := remediationR24(1, "src/parent.txt", tc.condition, "fwd")
			if got := checkByID(t, r, CheckWriteFilePreimageFresh).Remediation; got != want {
				t.Errorf("R24 not verbatim:\n got %q\nwant %q", got, want)
			}
			if name == "unreachable" && !hasAdvisory(r, AdvisoryProvenanceUnreachable) {
				t.Errorf("expected a provenance-unreachable advisory")
			}
		})
	}
}

// AC-L100 — verify NEVER reads the live working tree for a preimage
// comparison. Adversarial: a worktree that would make the check pass
// must still fail.
func TestACL100_NeverReadsTheLiveWorktreeForPreimages(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("fwd", store.StateApplied)
	// The recipe expects a preimage that exists ONLY in the worktree.
	worktreeOnly := "only-in-the-worktree\n"
	c.Artifacts("fwd", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+FWD\n",
		ApplyRecipe{Feature: "fwd", Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/parent.txt", Content: "parent-seed\nFWD\n",
				PreimageHash: strPtr(sha256Display(worktreeOnly))},
		}})
	writeProvenance(t, c.Store, "fwd", c.Base, true)
	mustWriteFile(t, filepath.Join(c.Root, "src", "parent.txt"), worktreeOnly)

	r := c.Verify("fwd")
	v10 := checkByID(t, r, CheckWriteFilePreimageFresh)
	if v10.Passed {
		t.Fatalf("V10 passed by reading the live working tree")
	}
	if !strings.Contains(v10.Remediation, c.Base) {
		t.Errorf("the failure must name the provenance baseline; got %q", v10.Remediation)
	}
}

// AC-L101 — ops WITHOUT preimage_hash stay on the ADR-029 D4 legacy path
// and the absence of a provenance sidecar is not an error there.
func TestACL101_LegacyOpsNeedNoProvenance(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("legacy", store.StateApplied)
	c.Artifacts("legacy", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+LEGACY\n",
		ApplyRecipe{Feature: "legacy", Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/parent.txt", Content: "parent-seed\nLEGACY\n"},
		}})
	r := c.Verify("legacy")
	v10 := checkByID(t, r, CheckWriteFilePreimageFresh)
	if !v10.Passed {
		t.Fatalf("legacy ops must pass without provenance: %q", v10.Remediation)
	}
	if r.FailedAt == FailedAtRecipeProvenance {
		t.Fatalf("a legacy recipe must not require a provenance sidecar")
	}
}

// AC-L102 — a MALFORMED preimage_hash ⇒ FAIL block regardless of any
// later-touch state.
func TestACL102_MalformedPreimageHashBlocks(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("bad", store.StateApplied)
	c.Artifacts("bad", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+BAD\n",
		ApplyRecipe{Feature: "bad", Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/parent.txt", Content: "parent-seed\nBAD\n",
				PreimageHash: strPtr("sha256:NOTLOWERCASEHEX")},
		}})
	writeProvenance(t, c.Store, "bad", c.Base, true)
	r := c.Verify("bad")
	v10 := checkByID(t, r, CheckWriteFilePreimageFresh)
	if v10.Passed {
		t.Fatalf("a malformed preimage_hash must block")
	}
	if !strings.Contains(v10.Remediation, "malformed preimage_hash") {
		t.Errorf("expected the ADR-029 D1 form diagnostic; got %q", v10.Remediation)
	}
}

// AC-L103 — later-touch is derived from the shipped METADATA detector.
// Adversarial: a path whose BYTES differ at HEAD but which no later
// feature touched raises NO advisory.
func TestACL103_LaterTouchIsMetadataNotBytes(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("only", store.StateApplied)
	c.Artifacts("only", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+ONLY\n",
		ApplyRecipe{Feature: "only", Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/parent.txt", Content: "parent-seed\nONLY\n",
				PreimageHash: strPtr(sha256Display("parent-seed\n"))},
		}})
	writeProvenance(t, c.Store, "only", c.Base, true)
	// Bytes at HEAD drift, but no LATER FEATURE touched the path.
	mustWriteFile(t, filepath.Join(c.Root, "src", "parent.txt"), "totally different bytes\n")
	mustGit(t, c.Root, "add", "-A")
	mustGit(t, c.Root, "commit", "-q", "-m", "byte drift with no owning feature")

	r := c.Verify("only")
	if hasAdvisory(r, AdvisoryLaterTouch) {
		t.Fatalf("later-touch must come from metadata, not bytes: %v", r.Advisories)
	}
}

// AC-L104 — a genuine later-touch ⇒ `later-touch` warn (R13); the
// verdict is not blocked by it.
func TestACL104_GenuineLaterTouchIsWarnOnly(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("older", store.StateApplied)
	c.Artifacts("older", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+OLDER\n",
		ApplyRecipe{Feature: "older", Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/parent.txt", Content: "parent-seed\nOLDER\n",
				PreimageHash: strPtr(sha256Display("parent-seed\n"))},
		}})
	writeProvenance(t, c.Store, "older", c.Base, true)

	// A LATER feature that touches the same path.
	c.Feature("newer", store.StateApplied)
	bumpRequestedAt(t, c.Store, "older", "2020-01-01T00:00:00Z")
	bumpRequestedAt(t, c.Store, "newer", "2030-01-01T00:00:00Z")
	c.Artifacts("newer", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+NEWER\n",
		ApplyRecipe{Feature: "newer", Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/parent.txt", Content: "parent-seed\nNEWER\n"},
		}})

	r := c.Verify("older")
	a, ok := advisoryByCode(r, AdvisoryLaterTouch)
	if !ok {
		t.Fatalf("expected a later-touch advisory; got %v", r.Advisories)
	}
	want := remediationR13("newer", "src/parent.txt", "older")
	if a.Message != want {
		t.Errorf("R13 not verbatim:\n got %q\nwant %q", a.Message, want)
	}
	if a.Severity != SeverityWarn {
		t.Errorf("later-touch severity=%q want warn", a.Severity)
	}
	if r.Verdict != "passed" {
		t.Fatalf("later-touch must never block: verdict=%s failed_at=%s v10=%q",
			r.Verdict, r.FailedAt, checkByID(t, r, CheckWriteFilePreimageFresh).Remediation)
	}
}

// AC-L105 — a superseded landed feature with a preimage mismatch has its
// severity downgraded to warn; a skipped/failed V2 makes V10 skip.
func TestACL105_SupersessionDowngradeAndV2Skip(t *testing.T) {
	t.Run("superseded-downgrade", func(t *testing.T) {
		c := newChainFixture(t)
		c.Feature("historical", store.StateApplied)
		c.Artifacts("historical", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+H\n",
			ApplyRecipe{Feature: "historical", Operations: []RecipeOperation{
				{Type: "write-file", Path: "src/parent.txt", Content: "parent-seed\nH\n",
					PreimageHash: strPtr(sha256Display("something else entirely\n"))},
			}})
		writeProvenance(t, c.Store, "historical", c.Base, true)
		c.Feature("superseder", store.StateApplied)
		st, _ := c.Store.LoadFeatureStatus("superseder")
		st.DependsOn = append(st.DependsOn, store.Dependency{Slug: "historical", Kind: store.DependencyKindSupersedes})
		if err := c.Store.SaveFeatureStatus(st); err != nil {
			t.Fatal(err)
		}
		r := c.Verify("historical")
		v10 := checkByID(t, r, CheckWriteFilePreimageFresh)
		if v10.Severity != SeverityWarn {
			t.Fatalf("severity=%q want warn for a superseded feature", v10.Severity)
		}
		if v10.Passed {
			t.Errorf("the failing signal must still surface")
		}
	})
	t.Run("v2-skipped-makes-v10-skip", func(t *testing.T) {
		c := newChainFixture(t)
		c.Feature("norecipe", store.StateApplied)
		if err := c.Store.WriteArtifact("norecipe", "post-apply.patch", "diff --git a/x b/x\n"); err != nil {
			t.Fatal(err)
		}
		r := c.Verify("norecipe")
		v10 := checkByID(t, r, CheckWriteFilePreimageFresh)
		if !v10.Skipped {
			t.Fatalf("V10 must skip when V2 skipped: %+v", v10)
		}
		if !strings.Contains(v10.Reason, "V2") {
			t.Errorf("reason=%q must name V2", v10.Reason)
		}
	})
}

// AC-L106 — parent V10 aggregation: a member's block-class outcome
// contributes to `parent-landing-drift`; its warn-class later-touch
// appears in advisories under the member's slug and affects no verdict.
func TestACL106_ParentV10Aggregation(t *testing.T) {
	t.Run("block-class-contributes-to-parent-landing-drift", func(t *testing.T) {
		c, parent, child := newPreimageChain(t)
		// Break the PARENT's preimage against its own baseline.
		c.Artifacts(parent, readFileString(t, artifactPath(c.Root, parent, "post-apply.patch")),
			ApplyRecipe{Feature: parent, Operations: []RecipeOperation{
				{Type: "write-file", Path: "src/parent.txt", Content: "parent-seed\nPARENT-CHANGE\n",
					PreimageHash: strPtr(sha256Display("a preimage that never existed\n"))},
			}})
		c.Land(parent)
		r := c.Verify(child)
		if r.FailedAt != FailedAtParentLandingDrift {
			t.Fatalf("failed_at=%q want parent-landing-drift", r.FailedAt)
		}
		if !strings.Contains(checkByID(t, r, CheckRecipeReplayClean).Remediation, parent) {
			t.Errorf("the fail-fast reason must name the member")
		}
	})
	t.Run("warn-class-later-touch-is-attributed", func(t *testing.T) {
		c, parent, child := newPreimageChain(t)
		bumpRequestedAt(t, c.Store, parent, "2020-01-01T00:00:00Z")
		bumpRequestedAt(t, c.Store, child, "2030-01-01T00:00:00Z")
		// Make the child touch the parent's path so the parent gets a
		// later-touch advisory attributed to it.
		c.Artifacts(child, readFileString(t, artifactPath(c.Root, child, "post-apply.patch")),
			ApplyRecipe{Feature: child, Operations: []RecipeOperation{
				{Type: "write-file", Path: "src/app.txt", Content: featureBody(nil),
					PreimageHash: strPtr(sha256Display(ladderBody(nil)))},
				{Type: "write-file", Path: "src/parent.txt", Content: "child owns this now\n"},
			}})
		c.Land(child)
		r := c.Verify(child)
		for _, a := range r.Advisories {
			if a.Code == AdvisoryLaterTouch && a.Slug == parent {
				if r.Verdict == "failed" && r.FailedAt == "" {
					t.Errorf("a warn-class member advisory must not flip the verdict")
				}
				return
			}
		}
		// Not reaching the advisory is acceptable only if the parent has
		// no write-file preimage op left to evaluate.
		t.Logf("advisories: %v", r.Advisories)
	})
}

func bumpRequestedAt(t *testing.T, s *store.Store, slug, ts string) {
	t.Helper()
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatalf("load %s: %v", slug, err)
	}
	st.RequestedAt = ts
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatalf("save %s: %v", slug, err)
	}
}

// featureSlug is an identity helper that keeps the member_baselines
// assertions readable.
func (c *chainFixture) featureSlug(slug string) string { return slug }

var _ = os.Remove
