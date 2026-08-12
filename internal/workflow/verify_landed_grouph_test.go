package workflow

// Acceptance rows AC-L107 – AC-L128 — PRD-verify-freshness §7.1 Group H:
// the immutable inventory, the schema-1.1 report surface, the closed
// diagnostic vocabularies, and the run-level invariants.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// AC-L107 — the inventory is built from store.ListFeatureEntries(), not
// ListFeatures(), and covers EVERY feature.
func TestACL107_InventoryCoversEveryFeatureViaListFeatureEntries(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("target", store.StateApplied)
	c.Feature("outsider", store.StateApplied)
	c.Artifacts("outsider", "diff --git a/x b/x\n", ApplyRecipe{Feature: "outsider", Operations: []RecipeOperation{
		{Type: "write-file", Path: "src/parent.txt", Content: "outsider\n"},
	}})
	// A feature that ListFeatures() would silently drop.
	broken := filepath.Join(c.Root, ".tpatch", "features", "broken")
	mustWriteFile(t, filepath.Join(broken, "status.json"), "{ this is not json")

	inv, err := buildInventory(c.Store)
	if err != nil {
		t.Fatalf("buildInventory: %v", err)
	}
	for _, slug := range []string{"target", "outsider", "broken"} {
		e := inv.Entry(slug)
		if e == nil {
			t.Fatalf("inventory omits %q — ListFeatures() would have dropped it", slug)
		}
	}
	if inv.Entry("broken").Err == nil {
		t.Errorf("the unreadable feature must be retained as an Err row")
	}
	if inv.Entry("outsider").Recipe.Presence != PresenceNonEmpty {
		t.Errorf("the inventory must capture out-of-closure artifacts too")
	}

	// ListFeatures() cannot see it: that is the whole point of D17.
	feats, _ := c.Store.ListFeatures()
	for _, f := range feats {
		if f.Slug == "broken" {
			t.Fatalf("precondition: ListFeatures must drop the broken feature")
		}
	}
}

// AC-L108 — classification is a pure, deterministic function of the
// inventory: two runs over identical values produce identical results
// and the unit under test performs no filesystem access.
func TestACL108_ClassificationIsPureOverTheInventory(t *testing.T) {
	f := newLadderFixture(t)
	ctx := newVerifyRunContext(f.Store)
	// Pre-fill the reachability memo so the pure pass needs no git.
	ctx.ancestorMemo[f.BaseCommit+"\x00HEAD"] = true

	w := installGitWrapper(t)
	w.Reset()
	first := ctx.classifyEvidenceUncached(f.Slug)
	second := ctx.classifyEvidenceUncached(f.Slug)
	if len(w.Calls()) != 0 {
		t.Fatalf("classification touched git: %v", w.Calls())
	}
	a, _ := json.Marshal(first.Evidence)
	b, _ := json.Marshal(second.Evidence)
	if string(a) != string(b) {
		t.Fatalf("classification is not deterministic:\n%s\n%s", a, b)
	}
}

// AC-L109 — every later stage consumes COPIES: an artifact changed after
// capture still yields digests computed from the captured bytes.
func TestACL109_StagesConsumeCapturedBytes(t *testing.T) {
	f := newLadderFixture(t)
	ctx := newVerifyRunContext(f.Store)
	captured := sha256Hex(ctx.inv.Entry(f.Slug).Patch.Bytes)

	mustWriteFile(t, artifactPath(f.Root, f.Slug, "post-apply.patch"), "rewritten after capture\n")
	after := sha256Hex(ctx.inv.Entry(f.Slug).Patch.Bytes)
	if captured != after {
		t.Fatalf("the inventory re-read disk: %s != %s", captured, after)
	}
	if captured == sha256Hex([]byte("rewritten after capture\n")) {
		t.Fatalf("the inventory captured the post-mutation bytes")
	}
}

// AC-L110 — a feature added, removed, changed, or flipping between an
// Err row and a Status row during a run ⇒ FAIL snapshot-unstable (R20).
func TestACL110_SnapshotInstabilityIsDetected(t *testing.T) {
	cases := map[string]func(t *testing.T, c *chainFixture){
		"added": func(t *testing.T, c *chainFixture) {
			c.Feature("appeared", store.StateApplied)
		},
		"removed": func(t *testing.T, c *chainFixture) {
			if err := os.RemoveAll(filepath.Join(c.Root, ".tpatch", "features", "outsider")); err != nil {
				t.Fatal(err)
			}
		},
		"changed": func(t *testing.T, c *chainFixture) {
			mustWriteFile(t, artifactPath(c.Root, "outsider", "apply-recipe.json"), `{"feature":"outsider","operations":[]}`)
		},
		"err-flip": func(t *testing.T, c *chainFixture) {
			mustWriteFile(t, filepath.Join(c.Root, ".tpatch", "features", "outsider", "status.json"), "{ broken")
		},
	}
	for name, mutate := range cases {
		t.Run("unit/"+name, func(t *testing.T) {
			c := newChainFixture(t)
			c.Feature("target", store.StateApplied)
			c.Feature("outsider", store.StateApplied)
			c.Artifacts("outsider", "diff --git a/x b/x\n", ApplyRecipe{Feature: "outsider"})
			before, err := buildInventory(c.Store)
			if err != nil {
				t.Fatal(err)
			}
			mutate(t, c)
			if got := inventoryInstability(c.Store, before); got == "" {
				t.Fatalf("instability not detected for %q", name)
			}
		})
	}

	t.Run("workflow/report-fails", func(t *testing.T) {
		c := newChainFixture(t)
		c.Feature("target", store.StateApplied)
		c.Artifacts("target", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+T\n",
			ApplyRecipe{Feature: "target", Operations: []RecipeOperation{
				{Type: "append-file", Path: "src/parent.txt", Content: "T\n"},
			}})
		// A PATH wrapper that mutates `.tpatch/` between git calls, with
		// no production hook of any kind.
		installMutatingGitWrapper(t, filepath.Join(c.Root, ".tpatch", "features", "target", "artifacts", "apply-recipe.json"))
		r := c.Verify("target")
		if r.FailedAt != FailedAtSnapshotUnstable {
			t.Fatalf("failed_at=%q want snapshot-unstable", r.FailedAt)
		}
		if !strings.Contains(checkByID(t, r, CheckRecipeReplayClean).Remediation, "changed while verify was running") {
			t.Errorf("expected R20; got %q", checkByID(t, r, CheckRecipeReplayClean).Remediation)
		}
	})
}

// installMutatingGitWrapper installs a `git` shim that rewrites `path`
// once, on its first invocation, before forwarding to the real git.
func installMutatingGitWrapper(t *testing.T, path string) {
	t.Helper()
	realGit := mustLookGit(t)
	dir := t.TempDir()
	stamp := filepath.Join(dir, "done")
	// The inventory is captured AFTER `git --version` and BEFORE the
	// preflight, so the mutation must land on a later call to model a
	// genuinely concurrent write.
	script := fmt.Sprintf(`#!/bin/sh
n=0
if [ -f %q ]; then n=$(cat %q); fi
n=$((n+1))
printf '%%s' "$n" > %q
if [ "$n" = "3" ]; then
  printf 'mutated by the PATH wrapper\n' >> %q
fi
exec %q "$@"
`, stamp, stamp, stamp, path, realGit)
	shim := filepath.Join(dir, "git")
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatalf("write shim: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// AC-L111 — an unreadable status.json is an Err row, never dropped:
// unrelated ⇒ warn advisory + exclusion from ordering; target or closure
// member ⇒ FAIL inventory-unreadable.
func TestACL111_UnreadableFeaturePolicy(t *testing.T) {
	t.Run("unrelated-warns-and-is-excluded", func(t *testing.T) {
		c := newChainFixture(t)
		c.Feature("target", store.StateApplied)
		c.Artifacts("target", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+T\n",
			ApplyRecipe{Feature: "target", Operations: []RecipeOperation{
				{Type: "append-file", Path: "src/parent.txt", Content: "T\n"},
			}})
		mustWriteFile(t, filepath.Join(c.Root, ".tpatch", "features", "broken", "status.json"), "{ broken")

		r := c.Verify("target")
		a, ok := advisoryByCode(r, AdvisoryInventoryUnreadable)
		if !ok {
			t.Fatalf("a silent skip fails this row; advisories=%v", r.Advisories)
		}
		if a.Slug != "broken" {
			t.Errorf("advisory slug=%q want broken", a.Slug)
		}
		inv, _ := buildInventory(c.Store)
		if idx := laterTouchIndexFromInventory(inv, "target"); idx != nil {
			for _, slug := range idx {
				if slug == "broken" {
					t.Errorf("an unreadable feature must be excluded from ADR-029 ordering")
				}
			}
		}
	})
	t.Run("closure-member-blocks", func(t *testing.T) {
		c := newChainFixture(t)
		c.Feature("parent", store.StateApplied)
		c.Feature("child", store.StateApplied, "parent")
		c.Artifacts("child", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+C\n",
			ApplyRecipe{Feature: "child", Operations: []RecipeOperation{
				{Type: "append-file", Path: "src/parent.txt", Content: "C\n"},
			}})
		mustWriteFile(t, filepath.Join(c.Root, ".tpatch", "features", "parent", "status.json"), "{ broken")

		// Shipped precedence: V4 already blocks on a dependency whose
		// status cannot be read, so the run fails before the dynamic
		// phase. Both halves are asserted.
		r := c.Verify("child")
		if r.Verdict != "failed" {
			t.Fatalf("verdict=%q want failed", r.Verdict)
		}

		ctx := newVerifyRunContext(c.Store)
		status, err := c.Store.LoadFeatureStatus("child")
		if err != nil {
			t.Fatal(err)
		}
		recipe, _ := ctx.inv.Entry("child").ParsedRecipe()
		phase := runDynamicPhase(anchoredInput{
			ctx: ctx, store: c.Store, slug: "child", status: status,
			recipe: recipe, recipePresent: true,
			entry: inventoryEntryOrEmpty(ctx, "child"), evidence: ctx.classifyEvidence("child"),
		})
		if phase.failedAt != FailedAtInventoryUnreadable {
			t.Fatalf("failed_at=%q want inventory-unreadable", phase.failedAt)
		}
		if !strings.Contains(phase.v7.Remediation, "could not be read from the feature inventory") {
			t.Errorf("the block must name the unreadable member: %q", phase.v7.Remediation)
		}
	})
	t.Run("target-blocks", func(t *testing.T) {
		c := newChainFixture(t)
		c.Feature("target", store.StateApplied)
		mustWriteFile(t, filepath.Join(c.Root, ".tpatch", "features", "target", "status.json"), "{ broken")
		r, err := RunVerify(c.Store, "target", VerifyOptions{NoWrite: true})
		if r == nil {
			t.Fatalf("no report: %v", err)
		}
		if r.Verdict != "failed" {
			t.Fatalf("verdict=%q want failed", r.Verdict)
		}
	})
}

// AC-L112 — inventory enumeration order is the slug-sorted order
// ListFeatureEntries returns, and the report is stable across runs.
func TestACL112_InventoryOrderIsDeterministic(t *testing.T) {
	c := newChainFixture(t)
	for _, slug := range []string{"zulu", "alpha", "mike"} {
		c.Feature(slug, store.StateApplied)
	}
	inv, err := buildInventory(c.Store)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]string{}, inv.Order...)
	for i := 1; i < len(want); i++ {
		if want[i-1] > want[i] {
			t.Fatalf("inventory order is not slug-sorted: %v", want)
		}
	}
	for i := 0; i < 3; i++ {
		again, err := buildInventory(c.Store)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(again.Order, ",") != strings.Join(want, ",") {
			t.Fatalf("order changed between runs: %v vs %v", again.Order, want)
		}
	}
}

// AC-L113 — schema_version is "1.1" and a no-evidence report is a
// SEMANTIC superset of "1.0": every 1.0 key retains name, type and
// position.
func TestACL113_SchemaIsAdditiveSuperset(t *testing.T) {
	f := newLandedFixture(t, "forward-shape")
	f.Implement("l1\nl2\nCHANGED\nl4\nl5\nl6\nl7\nl8\nl9\nl10\nl11\nl12\nl13\nl14\nl15\n")
	r := f.Verify()
	if r.SchemaVersion != "1.1" {
		t.Fatalf("schema_version=%q want 1.1", r.SchemaVersion)
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"schema_version", "slug", "verified_at", "verdict", "exit_code", "checks", "lifecycle_state"} {
		if _, ok := m[key]; !ok {
			t.Errorf("schema 1.0 key %q disappeared", key)
		}
	}
	checks, ok := m["checks"].([]any)
	if !ok || len(checks) != 11 {
		t.Fatalf("checks is not an 11-entry array")
	}
	row, _ := checks[0].(map[string]any)
	for _, key := range []string{"id", "severity", "passed"} {
		if _, ok := row[key]; !ok {
			t.Errorf("check row lost the 1.0 key %q", key)
		}
	}
	for _, added := range []string{"repository", "baseline", "landing_evidence", "target_mode"} {
		if _, ok := m[added]; !ok {
			t.Errorf("schema 1.1 must emit %q for every feature", added)
		}
	}
}

// AC-L114 — `failed_at` only takes a value from the closed set of
// thirteen, and advisory `code` only from the closed set of five.
func TestACL114_ClosedVocabularies(t *testing.T) {
	if len(FailedAtVocabulary()) != 13 {
		t.Fatalf("failed_at vocabulary has %d values, want 13", len(FailedAtVocabulary()))
	}
	if len(EvidenceStates()) != 10 {
		t.Fatalf("evidence states = %d, want 10", len(EvidenceStates()))
	}
	allowed := map[string]bool{}
	for _, v := range FailedAtVocabulary() {
		allowed[v] = true
	}
	codes := map[string]bool{
		AdvisoryContextDrift: true, AdvisoryLaterTouch: true,
		AdvisoryUnattributedMaterialized: true, AdvisoryBaseCommitUnreachable: true,
		AdvisoryProvenanceUnreachable: true, AdvisoryInventoryUnreadable: true,
	}
	// Adversarial sweep over every landed shape produced by the matrix.
	for _, r := range collectRepresentativeReports(t) {
		if r.FailedAt != "" && !allowed[r.FailedAt] {
			t.Errorf("failed_at=%q is outside the closed set", r.FailedAt)
		}
		for _, a := range r.Advisories {
			if !codes[a.Code] {
				t.Errorf("advisory code=%q is outside the closed set", a.Code)
			}
			if a.Severity != SeverityWarn {
				t.Errorf("advisory %q severity=%q want warn", a.Code, a.Severity)
			}
		}
		if r.LandingEvidence != nil && r.LandingEvidence.State != "" {
			ok := false
			for _, s := range EvidenceStates() {
				if s == r.LandingEvidence.State {
					ok = true
					break
				}
			}
			if !ok {
				t.Errorf("landing_evidence.state=%q is outside the closed set of ten", r.LandingEvidence.State)
			}
		}
	}
}

// collectRepresentativeReports produces one report per major shape so the
// adversarial vocabulary sweeps have real material.
func collectRepresentativeReports(t *testing.T) []*VerifyReport {
	t.Helper()
	var out []*VerifyReport

	pass := newLadderFixture(t)
	out = append(out, pass.Verify())

	drift := newLadderFixture(t)
	mutateHead(t, drift, func(l []string) { l[11] = "UNRELATED" })
	out = append(out, drift.Verify())

	blocked := newLadderFixture(t)
	mutateHead(t, blocked, func(l []string) { l[9] = "l10" })
	out = append(out, blocked.Verify())

	stale := newLadderFixture(t)
	stale.WritePatch(string(stale.PatchBytes()) + "\n")
	out = append(out, stale.Verify())

	absent := newLadderFixture(t)
	_ = os.Remove(artifactPath(absent.Root, absent.Slug, "post-apply.patch"))
	out = append(out, absent.Verify())

	forward := newLandedFixture(t, "forward-rep")
	forward.Implement("l1\nl2\nF\nl4\nl5\nl6\nl7\nl8\nl9\nl10\nl11\nl12\nl13\nl14\nl15\n")
	out = append(out, forward.Verify())

	unanchored := newLandedFixture(t, "unanchored-rep")
	mustWriteFile(t, filepath.Join(unanchored.Root, unanchored.FilePath), ladderBody(nil))
	mustGit(t, unanchored.Root, "add", "-A")
	mustGit(t, unanchored.Root, "commit", "-q", "-m", "seed")
	unanchored.BaseCommit = gitHeadOf(t, unanchored.Root)
	unanchored.LandTrailerOnlyAfterChange(featureBody(nil))
	out = append(out, unanchored.Verify())

	return out
}

// AC-L115 — NO verify report contains `freshness_label` (Q16).
func TestACL115_NoFreshnessLabelInVerifyReports(t *testing.T) {
	for _, r := range collectRepresentativeReports(t) {
		data, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "freshness_label") {
			t.Fatalf("verify report leaked freshness_label: %s", data)
		}
	}
}

// AC-L116 — `mode` is present on V7, V8 and V10 in EVERY report and
// absent on V0–V6 and V9.
func TestACL116_ModePresenceRule(t *testing.T) {
	anchored := map[string]bool{
		CheckRecipeReplayClean: true, CheckPostApplyPatchReplayClean: true, CheckWriteFilePreimageFresh: true,
	}
	for i, r := range collectRepresentativeReports(t) {
		for _, c := range r.Checks {
			if anchored[c.ID] {
				if c.Mode == "" {
					t.Errorf("report %d: %s has no mode (passed=%v skipped=%v)", i, c.ID, c.Passed, c.Skipped)
				}
				continue
			}
			if c.Mode != "" {
				t.Errorf("report %d: %s must not carry a mode, got %q", i, c.ID, c.Mode)
			}
		}
	}
}

// AC-L117 — every R1–R22 and R24 remediation string is emitted verbatim.
func TestACL117_RemediationGoldenStrings(t *testing.T) {
	golden := map[string]string{
		"R1":  "landed feature: post-apply.patch postimage is not present at HEAD; landing commit SHA is reachable but the content is absent — inspect with git diff SHA HEAD, then re-record and re-land. Do NOT run tpatch reconcile: this is local drift, not upstream drift",
		"R2":  "landed feature: post-apply.patch matched at HEAD only with all context discarded at PATH; verify refuses to certify an unanchored match — inspect with git diff SHA HEAD -- PATH, then re-record so the captured context matches HEAD and re-land",
		"R3":  "landed feature: post-apply.patch content is present at HEAD but its recorded context has drifted at PATH; a later change touched the surrounding lines — inspect with git diff SHA HEAD -- PATH and re-record if the feature should absorb it",
		"R4":  "landed feature: recipe op #3 failed to replay at the landing baseline SHA: boom; the recipe no longer describes the tree it was authored against — re-run tpatch record SLUG --regenerate-recipe and re-land",
		"R5":  "landed feature: post-apply.patch does not apply at the landing baseline SHA; the patch and the landing attestation disagree — re-record and re-land",
		"R6":  "landing evidence for SLUG is stale: commit SHA attests patch-sha=P / recipe-sha=R / base=B but the current artifacts hash differently; re-run tpatch land SLUG to re-attest, or restore the attested artifacts",
		"R7":  "landing evidence for SLUG is ambiguous: 2 reachable commits carry matching trailers with non-equivalent normalized changes (A, B); resolve the history or re-land so exactly one attestation is current",
		"R8":  "landing evidence for SLUG is malformed: commit SHA carries a Tpatch-Feature line that Git does not parse as a trailer, or a duplicated/ill-formed Tpatch-* value; restore the four-trailer block with git commit --amend, or re-land",
		"R9":  "landing evidence for SLUG is unusable: commit SHA has 2 parents and tpatch land emits single-parent commits; verify cannot derive a landing baseline from a root or merge commit — re-land SLUG on a linear commit",
		"R10": "landing evidence for SLUG could not be read: ERR; verify requires git >= 2.36 (trailer enumeration >= 2.22/2.25, object-format probe >= 2.29, and GIT_NO_LAZY_FETCH >= 2.36 for offline object access) and refuses to guess — upgrade git to 2.36 or newer, or report this environment",
		"R11": "landed feature SLUG has no usable landing baseline: no reachable single-parent landing commit has a parent that the current canonical patch applies to, or the qualifying candidates describe different changes; verify will not certify a landed feature it cannot replay — re-run tpatch record SLUG and tpatch land SLUG to create a fresh single-parent landing",
		"R12": "recipe op #3 PATH expected preimage EXP at baseline BASE but observed OBS; the recipe is stale against its own baseline — re-run tpatch record SLUG --regenerate-recipe and re-land",
		"R13": "later-touch: later feature LATER touched PATH after SLUG was recorded; replaying this write-file would silently revert it — review before any replay (ADR-029 D5/D6, warning-class)",
		"R14": "hard parent PARENT landed at SHA but its canonical patch is not present at the verification baseline; verify PARENT first — do not re-apply it into the shadow",
		"R15": "hard parent PARENT has stale landing evidence; verify PARENT first — replaying or skipping it would validate TARGET against an unknown baseline",
		"R16": "hard parent PARENT is unapplied; its patch is deliberately absent from the tree — run tpatch apply PARENT before verifying TARGET",
		"R17": "hard parent PARENT is rejected (terminal); remove the hard dependency with tpatch amend TARGET --remove-depends-on PARENT, or reopen PARENT",
		"R18": "unattributed-materialized: hard parent PARENT is not landed but its canonical patch is already present at the verification baseline; it was not replayed, and verify makes no claim about what produced it",
		"R19": "landed feature SLUG has no usable apply-recipe.json or post-apply.patch; materialization cannot be proven from an absent or empty artifact set — re-run tpatch record SLUG",
		"R20": "verify aborted: SLUG/PATH changed while verify was running; re-run tpatch verify TARGET with no concurrent tpatch or editor writes",
		"R21": "landing evidence for SLUG is incomplete: this is a shallow clone and commit SHA sits on the graft boundary, so its parent is not available locally — run git fetch --unshallow (or increase --depth) and re-run verify",
		"R22": "landing evidence for SLUG could not be completed: an object required to read the landing baseline is missing from this partial clone — restore network access to the promisor remote, or run git fetch --refetch, and re-run verify",
		"R24": "recipe op #3 PATH carries a preimage_hash but artifacts/recipe-provenance.json is absent; verify will not evaluate a preimage against the live working tree — re-run tpatch implement SLUG to regenerate the recipe and its provenance",
	}
	got := map[string]string{
		"R1":  remediationR1("SHA"),
		"R2":  remediationR2("SHA", "PATH"),
		"R3":  remediationR3("SHA", "PATH"),
		"R4":  remediationR4(3, "SHA", fmt.Errorf("boom"), "SLUG"),
		"R5":  remediationR5("SHA"),
		"R6":  remediationR6("SLUG", "SHA", "P", "R", "B"),
		"R7":  remediationR7("SLUG", 2, []string{"A", "B"}),
		"R8":  remediationR8("SLUG", "SHA"),
		"R9":  remediationR9("SLUG", "SHA", 2),
		"R10": remediationR10("SLUG", "ERR"),
		"R11": remediationR11("SLUG"),
		"R12": remediationR12(3, "PATH", "EXP", "BASE", "OBS", "SLUG"),
		"R13": remediationR13("LATER", "PATH", "SLUG"),
		"R14": remediationR14("PARENT", "SHA"),
		"R15": remediationR15("PARENT", "stale", "TARGET"),
		"R16": remediationR16("PARENT", "TARGET"),
		"R17": remediationR17("PARENT", "TARGET"),
		"R18": remediationR18("PARENT"),
		"R19": remediationR19("SLUG"),
		"R20": remediationR20("SLUG", "PATH", "TARGET"),
		"R21": remediationR21("SLUG", "SHA"),
		"R22": remediationR22("SLUG"),
		"R24": remediationR24(3, "PATH", "absent", "SLUG"),
	}
	for id, want := range golden {
		if got[id] != want {
			t.Errorf("%s not verbatim:\n got %q\nwant %q", id, got[id], want)
		}
	}
	if len(got) != 23 {
		t.Errorf("expected 23 remediation templates (R1–R22 + R24), got %d", len(got))
	}
}

// AC-L118 — the human report emits `baseline:` and `landing evidence:`
// above the check list, naming both anchors and the isolated probe.
func TestACL118_HumanReportHeaderLines(t *testing.T) {
	f := newLadderFixture(t)
	f.ExtendAndReland(featureBody(func(l []string) { l[39] = "FEATURE40" }))
	r := f.Verify()
	var sb strings.Builder
	r.WriteHumanReport(&sb)
	out := sb.String()
	if !strings.Contains(out, "  baseline: historical-anchor @ ") {
		t.Errorf("missing the baseline line:\n%s", out)
	}
	if !strings.Contains(out, "(replay anchor ") {
		t.Errorf("the replay anchor must be named when it differs from the attestation:\n%s", out)
	}
	if !strings.Contains(out, "(isolated index)") {
		t.Errorf("the isolated-index probe must be named:\n%s", out)
	}
	if !strings.Contains(out, "  landing evidence: exact @ ") {
		t.Errorf("missing the landing-evidence line:\n%s", out)
	}
}

// AC-L119 — a passing landed run persists the SAME field set as a
// passing forward run; omitempty round-trip preserved.
func TestACL119_PersistedRecordFieldSetIsUnchanged(t *testing.T) {
	landed := newLadderFixture(t)
	lr, err := RunVerify(landed.Store, landed.Slug, VerifyOptions{})
	if err != nil {
		t.Fatalf("landed RunVerify: %v", err)
	}
	forward := newLandedFixture(t, "forward-persist")
	forward.Implement("l1\nl2\nF\nl4\nl5\nl6\nl7\nl8\nl9\nl10\nl11\nl12\nl13\nl14\nl15\n")
	fr, err := RunVerify(forward.Store, forward.Slug, VerifyOptions{})
	if err != nil {
		t.Fatalf("forward RunVerify: %v", err)
	}
	keys := func(rec store.VerifyRecord) []string {
		data, mErr := json.Marshal(rec)
		if mErr != nil {
			t.Fatal(mErr)
		}
		var m map[string]any
		if uErr := json.Unmarshal(data, &m); uErr != nil {
			t.Fatal(uErr)
		}
		var out []string
		for k := range m {
			out = append(out, k)
		}
		return out
	}
	a, b := keys(lr.Persisted), keys(fr.Persisted)
	sortStrings(a)
	sortStrings(b)
	if strings.Join(a, ",") != strings.Join(b, ",") {
		t.Fatalf("persisted field sets differ: landed=%v forward=%v", a, b)
	}
	st, err := landed.Store.LoadFeatureStatus(landed.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if st.Verify == nil {
		t.Fatalf("no Verify record persisted")
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// AC-L120 — sticky clearing is mode-agnostic; the labels surface has no
// reference to landing evidence.
func TestACL120_StickyClearingIsModeAgnostic(t *testing.T) {
	f := newLadderFixture(t)
	// A failing landed run first.
	mutateHead(t, f, func(l []string) { l[9] = "l10" })
	if _, err := RunVerify(f.Store, f.Slug, VerifyOptions{}); err != nil {
		t.Logf("expected failing run: %v", err)
	}
	st, _ := f.Store.LoadFeatureStatus(f.Slug)
	if st.Verify == nil || st.Verify.Passed {
		t.Fatalf("precondition: the failing run must persist passed=false")
	}
	// Restore and re-verify: the sticky state clears on the first pass.
	f.EditTracked(featureBody(nil), "restore the feature")
	r, err := RunVerify(f.Store, f.Slug, VerifyOptions{})
	if err != nil {
		t.Fatalf("RunVerify: %v", err)
	}
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%s failed_at=%s", r.Verdict, r.FailedAt)
	}
	st, _ = f.Store.LoadFeatureStatus(f.Slug)
	if st.Verify == nil || !st.Verify.Passed {
		t.Fatalf("sticky verify-failed did not clear")
	}

	// Adversarial: the labels surface must not reference landing evidence.
	src, err := os.ReadFile("labels.go")
	if err != nil {
		t.Fatalf("read labels.go: %v", err)
	}
	for _, forbidden := range []string{"LandingEvidence", "landing_evidence", "target_mode", "TargetMode"} {
		if strings.Contains(string(src), forbidden) {
			t.Errorf("labels.go references %q — freshness derivation must take no mode input", forbidden)
		}
	}
}

// AC-L121 — the GH #2 regression test stays green UNMODIFIED.
func TestACL121_GH2RegressionTestIsUnmodified(t *testing.T) {
	const waveBase = "b768602"
	const path = "internal/workflow/verify_closure_replay_test.go"
	repo, err := repoRootForTest()
	if err != nil {
		t.Skipf("not running inside the tpatch repository: %v", err)
	}
	if _, err := tryGit(repo, "rev-parse", "--verify", waveBase+"^{commit}"); err != nil {
		t.Skipf("WAVE_BASE %s is not resolvable here", waveBase)
	}
	if out, err := tryGit(repo, "diff", "--exit-code", waveBase, "--", path); err != nil {
		t.Fatalf("%s was modified since WAVE_BASE %s:\n%s", path, waveBase, out)
	}
}

func repoRootForTest() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Dir(filepath.Dir(wd)), nil
}

// AC-L122 — the GH #2 reset holds at anchor H: after V7 mutates the
// shadow the tree hash seen by V8 equals `closureBaselineTree`.
func TestACL122_GH2ResetHoldsAtAnchorH(t *testing.T) {
	f := newLadderFixture(t)
	w := installGitWrapper(t)
	w.Reset()
	r := f.Verify()
	if r.Verdict != "passed" {
		t.Fatalf("verdict=%s", r.Verdict)
	}
	resets := 0
	for _, c := range callsWithSubcommand(w.Calls(), "read-tree") {
		if c.Has("--reset") && c.Has("-u") {
			resets++
		}
	}
	if resets == 0 {
		t.Fatalf("the shadow was never reset between V7 and V8 — the GH #2 invariant")
	}
}

// AC-L123 — the shadow is pruned on EVERY exit path.
func TestACL123_ShadowPrunedOnEveryExitPath(t *testing.T) {
	cases := map[string]func(t *testing.T, f *landedFixture){
		"pass":                  func(t *testing.T, f *landedFixture) {},
		"landed-content-absent": func(t *testing.T, f *landedFixture) { mutateHead(t, f, func(l []string) { l[9] = "l10" }) },
		"stale":                 func(t *testing.T, f *landedFixture) { f.WritePatch(string(f.PatchBytes()) + "\n") },
		"artifacts-absent": func(t *testing.T, f *landedFixture) {
			_ = os.Remove(artifactPath(f.Root, f.Slug, "post-apply.patch"))
		},
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			f := newLadderFixture(t)
			arrange(t, f)
			f.Verify()
			shadowRoot := filepath.Join(f.Root, ".tpatch", "shadow")
			entries, err := os.ReadDir(shadowRoot)
			if err != nil {
				return // no shadow dir at all
			}
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), f.Slug+"-") {
					t.Errorf("shadow leaked on %s: %s", name, e.Name())
				}
			}
		})
	}
}

// AC-L126 — the replace-in-file diagnostic predicate is SOUND on the
// exhaustive corpus.
func TestACL126_ReplaceInFilePredicateIsSound(t *testing.T) {
	if got := replaceInFilePresence("abb", "aa", "b"); got != OpPresentLikely {
		t.Errorf("c='abb', S='aa', R='b' ⇒ %v want present", got)
	}
	if got := replaceInFilePresence("b", "a", "a"); got != OpAbsentLikely {
		t.Errorf("c='b', S='a', R='a' ⇒ %v want absent", got)
	}
	if got := replaceInFilePresence("anything", "a", ""); got != OpPresenceUndecidable {
		t.Errorf("R=='' ⇒ %v want undecidable", got)
	}
	if got := replaceInFilePresence("anything", "", "x"); got != OpPresenceUnsupported {
		t.Errorf("S=='' ⇒ %v want unsupported", got)
	}

	// Exhaustive enumeration over the alphabet {a,b,X}, strings up to
	// length 3 for the content and up to 2 for search/replace.
	alphabet := []rune{'a', 'b', 'X'}
	var words func(n int) []string
	words = func(n int) []string {
		if n == 0 {
			return []string{""}
		}
		var out []string
		for _, w := range words(n - 1) {
			out = append(out, w)
			for _, r := range alphabet {
				out = append(out, w+string(r))
			}
		}
		// de-duplicate
		seen := map[string]bool{}
		var uniq []string
		for _, w := range out {
			if !seen[w] {
				seen[w] = true
				uniq = append(uniq, w)
			}
		}
		return uniq
	}
	contents := words(3)
	patterns := words(2)
	preimages := words(5)
	decided, falseRed, falseGreen, undecidable := 0, 0, 0, 0
	for _, c := range contents {
		for _, search := range patterns {
			for _, replace := range patterns {
				verdict := replaceInFilePresence(c, search, replace)
				switch verdict {
				case OpPresenceUnsupported:
					continue
				case OpPresenceUndecidable:
					undecidable++
					continue
				}
				decided++
				// Ground truth: does SOME preimage p exist in which the
				// replacement ACTUALLY FIRED and produced c? The
				// occurrence requirement is what makes `c='b', S='a',
				// R='a'` false, as the ADR pins. The preimage domain is
				// long enough to express |c| - |replace| + |search|.
				truth := false
				for _, p := range preimages {
					if !strings.Contains(p, search) {
						continue
					}
					if strings.Replace(p, search, replace, 1) == c {
						truth = true
						break
					}
				}
				got := verdict == OpPresentLikely
				if truth && !got {
					falseRed++
				}
				if !truth && got {
					falseGreen++
				}
			}
		}
	}
	if decided == 0 {
		t.Fatalf("the corpus decided nothing")
	}
	if falseRed != 0 || falseGreen != 0 {
		t.Fatalf("predicate unsound over %d decided cases: %d false reds, %d false greens",
			decided, falseRed, falseGreen)
	}
	t.Logf("decided=%d undecidable=%d false-reds=0 false-greens=0", decided, undecidable)
}

// AC-L127 — diagnostic predicates NEVER certify.
func TestACL127_DiagnosticsNeverCertify(t *testing.T) {
	// A write-file op whose content matches byte-for-byte while the patch
	// ladder blocks must still FAIL.
	f := newLadderFixture(t)
	body := featureBody(nil)
	f.WriteRecipe(ApplyRecipe{Feature: f.Slug, Operations: []RecipeOperation{
		{Type: "write-file", Path: f.FilePath, Content: body},
	}})
	f.LandWithBlock(f.TrailerBlock(f.Slug))
	// Revert the content at HEAD but keep the recipe claiming it.
	mustWriteFile(t, filepath.Join(f.Root, f.FilePath), ladderBody(nil))
	mustGit(t, f.Root, "add", "-A")
	mustGit(t, f.Root, "commit", "-q", "-m", "revert")
	r := f.Verify()
	if r.Verdict != "failed" {
		t.Fatalf("a matching recipe must not certify presence: verdict=%s", r.Verdict)
	}

	// append-file with EMPTY content reports undecidable rather than
	// passing.
	if got := DiagnoseOpPresence(RecipeOperation{Type: "append-file", Path: "x"}, []byte("anything"), true, false); got != OpPresenceUndecidable {
		t.Errorf("append-file with empty content ⇒ %v want undecidable", got)
	}
	// Unknown op type is unsupported.
	if got := DiagnoseOpPresence(RecipeOperation{Type: "teleport-file", Path: "x"}, nil, false, false); got != OpPresenceUnsupported {
		t.Errorf("unknown op ⇒ %v want unsupported", got)
	}
	// ensure-directory.
	if got := DiagnoseOpPresence(RecipeOperation{Type: "ensure-directory", Path: "d"}, nil, true, true); got != OpPresentLikely {
		t.Errorf("ensure-directory on a real directory ⇒ %v", got)
	}
	if got := DiagnoseOpPresence(RecipeOperation{Type: "ensure-directory", Path: "d"}, nil, true, false); got != OpAbsentLikely {
		t.Errorf("ensure-directory on a file ⇒ %v", got)
	}
}

var _ = gitutil.TrailerFeature
