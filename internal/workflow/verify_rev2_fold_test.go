package workflow

// Rev-2 fold regression suite — v0.15.1 Wave C, adjudication findings
// 1-5. Every test here drives the REAL branch end-to-end (black box
// through `RunVerify`), because rev-1's helper-level assertions passed
// while the shipped paths still had the defect.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// ── Finding 1 — V2 parses the CAPTURED recipe bytes ──────────────────────

// TestRev2_V2ParsesCapturedRecipeBytes is the black-box mutation-after-
// capture regression: the recipe is rewritten on disk between the
// capture and V2, and the run must still describe the ORIGINAL bytes
// while the final re-statement reports `snapshot-unstable`.
func TestRev2_V2ParsesCapturedRecipeBytes(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("target", store.StateApplied)
	original := ApplyRecipe{Feature: "target", Operations: []RecipeOperation{
		{Type: "append-file", Path: "src/parent.txt", Content: "ORIGINAL\n"},
	}}
	c.Artifacts("target", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+ORIGINAL\n", original)
	originalBytes := readFileString(t, artifactPath(c.Root, "target", "apply-recipe.json"))

	// A PATH wrapper rewrites the recipe with UNPARSEABLE bytes on the
	// third git call — after the capture, before V2 runs. No production
	// hook of any kind.
	installMutatingGitWrapperWithBody(t,
		artifactPath(c.Root, "target", "apply-recipe.json"),
		"{ this is not valid json at all")

	r := c.Verify("target")

	// V2 must have parsed the CAPTURED bytes: the mutation is
	// unparseable, so a disk read would have failed the check.
	v2 := checkByID(t, r, CheckRecipeParses)
	if !v2.Passed || v2.Skipped {
		t.Fatalf("V2 read the mutated file instead of the capture: %+v", v2)
	}
	// The persisted hash describes the captured bytes, not the mutation.
	if r.RecipeHashAtVerify != sha256Hex([]byte(originalBytes)) {
		t.Errorf("recipe_hash_at_verify does not describe the captured bytes")
	}
	// And the run still FAILS, because the re-statement caught the write.
	if r.FailedAt != FailedAtSnapshotUnstable {
		t.Fatalf("failed_at=%q want snapshot-unstable", r.FailedAt)
	}
}

// installMutatingGitWrapperWithBody rewrites `path` with `body` on the
// Nth git invocation (after the inventory capture, which spawns no git).
func installMutatingGitWrapperWithBody(t *testing.T, path, body string) {
	t.Helper()
	realGit := mustLookGit(t)
	dir := t.TempDir()
	stamp := filepath.Join(dir, "count")
	payload := filepath.Join(dir, "payload")
	if err := os.WriteFile(payload, []byte(body), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	script := "#!/bin/sh\nn=0\nif [ -f " + stamp + " ]; then n=$(cat " + stamp + "); fi\n" +
		"n=$((n+1))\nprintf '%s' \"$n\" > " + stamp + "\n" +
		"if [ \"$n\" = \"3\" ]; then cp " + payload + " " + path + "; fi\n" +
		"exec " + realGit + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatalf("write shim: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TestRev2_VerifyPathHasNoLiveArtifactReads is the purity/source guard:
// within the verify sources, `os.ReadFile` may appear ONLY inside the
// capture itself and inside the shadow replayer.
func TestRev2_VerifyPathHasNoLiveArtifactReads(t *testing.T) {
	allowed := map[string]string{
		"snapshotArtifact":     "the capture itself",
		"buildInventory":       "the capture itself",
		"inventoryInstability": "the documented re-statement",
		"replayOpInShadow":     "reads inside the shadow worktree, not the store",
	}
	root := filepath.Join(docsRootForTest(t), "internal", "workflow")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "verify") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(root, name)
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if !ok || pkg.Name != "os" {
					return true
				}
				if sel.Sel.Name != "ReadFile" && sel.Sel.Name != "Open" {
					return true
				}
				if _, permitted := allowed[fn.Name.Name]; !permitted {
					t.Errorf("%s: %s reads the filesystem directly (os.%s); the verify path must consume the immutable capture",
						name, fn.Name.Name, sel.Sel.Name)
				}
				return true
			})
		}
	}
}

// ── Finding 2 — duplicate-attestation identity failure ───────────────────

// newDuplicateAttestationFixture produces TWO reachable landings that
// both attest the current artifacts, so classification reaches the
// identity comparison (`len(allMatch) >= 2`).
func newDuplicateAttestationFixture(t *testing.T) *landedFixture {
	t.Helper()
	f := newLadderFixture(t)
	f.RelandIdentical()
	// Sanity: without any injected failure this is duplicate-equivalent.
	if got := f.Verify().LandingEvidence.State; got != EvidenceDuplicateEquivalent {
		t.Fatalf("fixture precondition: evidence=%q want duplicate-equivalent", got)
	}
	return f
}

// TestRev2_DuplicateAttestationGenericDiffFailureIsUnavailable drives the
// real multi-attestation branch with a `git diff` that exits 128 and
// asserts `unavailable` + R10 — never R7's "resolve the history".
func TestRev2_DuplicateAttestationGenericDiffFailureIsUnavailable(t *testing.T) {
	f := newDuplicateAttestationFixture(t)
	installFailingSubcommandGit(t, "diff", "fatal: internal error: diff machinery unavailable")

	r := f.Verify()
	if r.LandingEvidence.State != EvidenceUnavailable {
		t.Fatalf("evidence=%q want unavailable", r.LandingEvidence.State)
	}
	if r.FailedAt != FailedAtLandingEvidence {
		t.Errorf("failed_at=%q want landing-evidence", r.FailedAt)
	}
	v7 := checkByID(t, r, CheckRecipeReplayClean)
	want := remediationR10(f.Slug, r.LandingEvidence.Reason)
	if v7.Remediation != want {
		t.Errorf("R10 not verbatim:\n got %q\nwant %q", v7.Remediation, want)
	}
	assertNoR7(t, r)
}

// TestRev2_DuplicateAttestationMissingObjectIsHistoryIncomplete drives the
// same branch with a genuinely missing object and asserts
// `history-incomplete` + R22 — never R7.
func TestRev2_DuplicateAttestationMissingObjectIsHistoryIncomplete(t *testing.T) {
	f := newDuplicateAttestationFixture(t)
	// Delete the blob the identity diff must read at the older landing's
	// parent. The object is genuinely gone; under GIT_NO_LAZY_FETCH=1
	// git fails locally and immediately.
	blob := mustGit(t, f.Root, "rev-parse", f.LandingCommit+"^:"+f.FilePath)
	deleteLooseOrPackedObject(t, f.Root, blob)

	r := f.Verify()
	if r.LandingEvidence.State != EvidenceHistoryIncomplete {
		t.Fatalf("evidence=%q want history-incomplete (reason=%q)", r.LandingEvidence.State, r.LandingEvidence.Reason)
	}
	if r.FailedAt != FailedAtLandingEvidence {
		t.Errorf("failed_at=%q want landing-evidence", r.FailedAt)
	}
	v7 := checkByID(t, r, CheckRecipeReplayClean)
	if got, want := v7.Remediation, remediationR22(f.Slug); got != want {
		t.Errorf("R22 not verbatim:\n got %q\nwant %q", got, want)
	}
	assertNoR7(t, r)
}

// TestRev2_DuplicateAttestationTrueAmbiguityStillUsesR7 pins the other
// half: identities that were successfully COMPUTED and differ remain
// `ambiguous` with R7.
func TestRev2_DuplicateAttestationTrueAmbiguityStillUsesR7(t *testing.T) {
	f := newLadderFixture(t)
	// A second landing that attests the SAME artifacts but introduces a
	// different change: two all-match candidates, differing identities.
	block := f.TrailerBlock(f.Slug)
	mustWriteFile(t, filepath.Join(f.Root, f.FilePath), featureBody(func(l []string) { l[19] = "DIFFERENT" }))
	mustGit(t, f.Root, "add", "-A")
	mustGit(t, f.Root, "commit", "-q", "-m", "second attestation, different change", "-m", block)

	r := f.Verify()
	if r.LandingEvidence.State != EvidenceAmbiguous {
		t.Fatalf("evidence=%q want ambiguous", r.LandingEvidence.State)
	}
	v7 := checkByID(t, r, CheckRecipeReplayClean)
	if !strings.Contains(v7.Remediation, "is ambiguous:") {
		t.Errorf("a successfully computed disagreement must still emit R7; got %q", v7.Remediation)
	}
}

func assertNoR7(t *testing.T, r *VerifyReport) {
	t.Helper()
	for _, c := range r.Checks {
		if strings.Contains(c.Remediation, "is ambiguous:") ||
			strings.Contains(c.Remediation, "resolve the history") {
			t.Errorf("%s emitted the R7 ambiguity remediation for a probe FAILURE: %q", c.ID, c.Remediation)
		}
	}
}

// installFailingSubcommandGit makes `git <sub>` fail with msg (exit 128)
// while forwarding every other subcommand to the real git.
func installFailingSubcommandGit(t *testing.T, sub, msg string) {
	t.Helper()
	realGit := mustLookGit(t)
	dir := t.TempDir()
	script := "#!/bin/sh\nfor a in \"$@\"; do\n  case \"$a\" in\n    -*) continue ;;\n    " + sub + ")\n" +
		"      echo " + rev1ShellQuote(msg) + " >&2\n      exit 128 ;;\n" +
		"    *) break ;;\n  esac\ndone\nexec " + realGit + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatalf("write shim: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// ── Finding 3 — historical V8 apply failure ──────────────────────────────

// TestRev2_HistoricalV8GenuineNonApplyIsR5 pins the ANSWER branch: a
// patch that genuinely does not apply at the landing baseline is still
// exactly R5.
func TestRev2_HistoricalV8GenuineNonApplyIsR5(t *testing.T) {
	f := newLadderFixture(t)
	// Rewrite the canonical patch so it still parses and still qualifies
	// nothing at the anchor: a hunk whose context is not in the baseline.
	anchor := mustGit(t, f.Root, "rev-parse", f.LandingCommit+"^")
	bad := "diff --git a/" + f.FilePath + " b/" + f.FilePath + "\n" +
		"--- a/" + f.FilePath + "\n+++ b/" + f.FilePath + "\n" +
		"@@ -1,3 +1,3 @@\n l1\n-NOT-IN-THE-BASELINE\n+X\n l3\n"
	f.WritePatch(bad)
	f.LandWithBlock(f.TrailerBlock(f.Slug))
	_ = anchor

	r := f.Verify()
	if r.LandingEvidence.State == EvidenceUnavailable || r.LandingEvidence.State == EvidenceHistoryIncomplete {
		t.Fatalf("a patch that does not apply must not be reported as a reader failure: %q (%s)",
			r.LandingEvidence.State, r.LandingEvidence.Reason)
	}
	// Either the anchor search honestly finds no qualifier (R11) or V8
	// reports R5; both are ANSWERS, neither is R10/R22.
	v7 := checkByID(t, r, CheckRecipeReplayClean)
	v8 := checkByID(t, r, CheckPostApplyPatchReplayClean)
	joined := v7.Remediation + v8.Remediation
	if strings.Contains(joined, "verify requires git >= 2.36") ||
		strings.Contains(joined, "missing from this partial clone") {
		t.Fatalf("a patch-level answer was reported as an execution failure: %q", joined)
	}
}

// TestRev2_HistoricalV8MalformedPatchStaysAnAnswer pins that a corrupt
// artifact — which makes `git apply --check` exit 128 with
// "No valid patches in input" — is a patch-level answer, not a reader
// failure.
func TestRev2_HistoricalV8MalformedPatchStaysAnAnswer(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("target", store.StateApplied)
	c.Artifacts("target", "this is not a diff at all\n", ApplyRecipe{Feature: "target", Operations: []RecipeOperation{
		{Type: "append-file", Path: "src/parent.txt", Content: "T\n"},
	}})
	r := c.Verify("target")
	if r.LandingEvidence.State == EvidenceUnavailable || r.LandingEvidence.State == EvidenceHistoryIncomplete {
		t.Fatalf("a malformed patch must stay a patch-level answer; evidence=%q", r.LandingEvidence.State)
	}
	v8 := checkByID(t, r, CheckPostApplyPatchReplayClean)
	if v8.Passed {
		t.Fatalf("a malformed patch must fail V8")
	}
	if !strings.Contains(v8.Remediation, "no longer applies to closure-replayed baseline") {
		t.Errorf("expected the shipped forward-mode V8 remediation; got %q", v8.Remediation)
	}
}

// TestRev2_HistoricalV8ExecutionFailureIsUnavailable drives the FAILURE
// branch: `git apply` exits 128 with a repository-level fatal, and the
// run must report `unavailable` + R10 rather than claiming the patch and
// the attestation disagree. Shadow cleanup is asserted too.
func TestRev2_HistoricalV8ExecutionFailureIsUnavailable(t *testing.T) {
	f := newLadderFixture(t)
	installShadowApplyFailureGit(t, "fatal: internal error: object database offline")

	r := f.Verify()
	if r.LandingEvidence.State != EvidenceUnavailable {
		t.Fatalf("evidence=%q want unavailable (reason=%q)", r.LandingEvidence.State, r.LandingEvidence.Reason)
	}
	v8 := checkByID(t, r, CheckPostApplyPatchReplayClean)
	if strings.Contains(v8.Remediation, "the patch and the landing attestation disagree") {
		t.Fatalf("an unrunnable check must not claim a patch/attestation disagreement: %q", v8.Remediation)
	}
	v7 := checkByID(t, r, CheckRecipeReplayClean)
	if !strings.Contains(v7.Remediation, "verify requires git >= 2.36") {
		t.Errorf("expected R10; got %q", v7.Remediation)
	}
	assertNoShadowLeak(t, f.Root, f.Slug)
}

// TestRev2_HistoricalV8MissingObjectIsHistoryIncomplete drives the same
// branch with a missing-object diagnostic.
func TestRev2_HistoricalV8MissingObjectIsHistoryIncomplete(t *testing.T) {
	f := newLadderFixture(t)
	installShadowApplyFailureGit(t, "error: unable to read sha1 file of src/app.txt")

	r := f.Verify()
	if r.LandingEvidence.State != EvidenceHistoryIncomplete {
		t.Fatalf("evidence=%q want history-incomplete (reason=%q)", r.LandingEvidence.State, r.LandingEvidence.Reason)
	}
	if got, want := checkByID(t, r, CheckRecipeReplayClean).Remediation, remediationR22(f.Slug); got != want {
		t.Errorf("R22 not verbatim:\n got %q\nwant %q", got, want)
	}
	assertNoShadowLeak(t, f.Root, f.Slug)
}

// installShadowApplyFailureGit fails ONLY the shadow-side
// `git apply --check <patch>` (no `--cached`), leaving the isolated-index
// ladder and the qualifier untouched.
func installShadowApplyFailureGit(t *testing.T, msg string) {
	t.Helper()
	realGit := mustLookGit(t)
	dir := t.TempDir()
	script := "#!/bin/sh\nisapply=0\ncached=0\nfor a in \"$@\"; do\n" +
		"  [ \"$a\" = \"apply\" ] && isapply=1\n" +
		"  [ \"$a\" = \"--cached\" ] && cached=1\ndone\n" +
		"if [ \"$isapply\" = \"1\" ] && [ \"$cached\" = \"0\" ]; then\n" +
		"  echo " + rev1ShellQuote(msg) + " >&2\n  exit 128\nfi\n" +
		"exec " + realGit + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatalf("write shim: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func assertNoShadowLeak(t *testing.T, root, slug string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, ".tpatch", "shadow"))
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), slug+"-") {
			t.Errorf("shadow leaked: %s", e.Name())
		}
	}
}

// ── Finding 4 — unlanded parent presence probe ───────────────────────────

// buildUnlandedParentChain returns a chain whose hard parent is applied,
// unlanded, and whose recipe would FAIL loudly if it were ever replayed.
func buildUnlandedParentChain(t *testing.T) (*chainFixture, string) {
	t.Helper()
	c := newChainFixture(t)
	c.Feature("parent", store.StateApplied)
	c.Feature("child", store.StateApplied, "parent")
	// The parent's content IS present in the tree, so the presence probe
	// has something to answer about.
	mustWriteFile(t, filepath.Join(c.Root, "src", "parent.txt"), "parent-seed\nPARENT-CHANGE\n")
	patch := mustGit(t, c.Root, "diff", "--", "src/parent.txt") + "\n"
	c.Artifacts("parent", patch, poisonRecipe("parent"))
	mustGit(t, c.Root, "add", "-A")
	mustGit(t, c.Root, "commit", "-q", "-m", "materialize the parent, no trailer")

	mustWriteFile(t, filepath.Join(c.Root, "src", "app.txt"), featureBody(nil))
	childPatch := mustGit(t, c.Root, "diff", "--", "src/app.txt") + "\n"
	c.Artifacts("child", childPatch, ApplyRecipe{Feature: "child", Operations: []RecipeOperation{
		{Type: "write-file", Path: "src/app.txt", Content: featureBody(nil)},
	}})
	return c, "child"
}

// TestRev2_UnlandedParentProbeFailureNeverReplays is the core finding-4
// regression: the parent's recipe is a POISON recipe that fails loudly
// if executed, so a fall-through to replay is observable.
func TestRev2_UnlandedParentProbeFailureNeverReplays(t *testing.T) {
	t.Run("generic-execution-failure", func(t *testing.T) {
		c, child := buildUnlandedParentChain(t)
		installFailingCachedApplyGit(t, "fatal: internal error: index machinery offline")
		r := c.Verify(child)
		assertParentProbeTerminal(t, r, "verify requires git >= 2.36")
	})
	t.Run("missing-object", func(t *testing.T) {
		c, child := buildUnlandedParentChain(t)
		installFailingCachedApplyGit(t, "error: unable to read sha1 file of src/parent.txt")
		r := c.Verify(child)
		assertParentProbeTerminal(t, r, "missing from this partial clone")
	})
}

func assertParentProbeTerminal(t *testing.T, r *VerifyReport, wantFragment string) {
	t.Helper()
	if r.Verdict != "failed" {
		t.Fatalf("an unanswerable presence probe must fail the run: verdict=%s", r.Verdict)
	}
	v7 := checkByID(t, r, CheckRecipeReplayClean)
	if strings.Contains(v7.Remediation, "search text not found") {
		t.Fatalf("the parent's recipe was REPLAYED after an unanswerable probe: %q", v7.Remediation)
	}
	if !strings.Contains(v7.Remediation, wantFragment) {
		t.Errorf("expected the closed remediation %q; got %q", wantFragment, v7.Remediation)
	}
	if r.FailedAt != FailedAtLandingEvidence {
		t.Errorf("failed_at=%q want landing-evidence", r.FailedAt)
	}
}

// TestRev2_UnlandedParentAnsweredAbsenceStillReplays pins the other half:
// an ANSWERED absence (`ladder.Blocked`) still means replay, and a clean
// probe still means skip + R18.
func TestRev2_UnlandedParentAnsweredAbsenceStillReplays(t *testing.T) {
	t.Run("answered-absence-replays", func(t *testing.T) {
		c := newChainFixture(t)
		c.Feature("parent", store.StateApplied)
		c.Feature("child", store.StateApplied, "parent")
		// The parent's patch is NOT materialized: an answered absence.
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
			t.Fatalf("an answered absence must be replayed: verdict=%s failed_at=%s v7=%q",
				r.Verdict, r.FailedAt, checkByID(t, r, CheckRecipeReplayClean).Remediation)
		}
		if hasAdvisory(r, AdvisoryUnattributedMaterialized) {
			t.Errorf("an absent parent must not be reported as unattributed-materialized")
		}
	})
	t.Run("clean-probe-skips-with-R18", func(t *testing.T) {
		c, child := buildUnlandedParentChain(t)
		r := c.Verify(child)
		if r.Verdict != "passed" {
			t.Fatalf("verdict=%s failed_at=%s", r.Verdict, r.FailedAt)
		}
		a, ok := advisoryByCode(r, AdvisoryUnattributedMaterialized)
		if !ok {
			t.Fatalf("expected the R18 advisory; got %v", r.Advisories)
		}
		if a.Message != remediationR18("parent") {
			t.Errorf("R18 not verbatim: %q", a.Message)
		}
	})
}

// installFailingCachedApplyGit fails ONLY the isolated-index probes
// (`git apply ... --cached`), so the presence ladder is unanswerable
// while the shadow-side check is untouched.
func installFailingCachedApplyGit(t *testing.T, msg string) {
	t.Helper()
	realGit := mustLookGit(t)
	dir := t.TempDir()
	script := "#!/bin/sh\nisapply=0\ncached=0\nfor a in \"$@\"; do\n" +
		"  [ \"$a\" = \"apply\" ] && isapply=1\n" +
		"  [ \"$a\" = \"--cached\" ] && cached=1\ndone\n" +
		"if [ \"$isapply\" = \"1\" ] && [ \"$cached\" = \"1\" ]; then\n" +
		"  echo " + rev1ShellQuote(msg) + " >&2\n  exit 128\nfi\n" +
		"exec " + realGit + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatalf("write shim: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// ── Finding 5 — instability readability transitions ──────────────────────

// TestRev2_ReadabilityTransitionsAreUnstable is the table test over all
// four captured surfaces, in BOTH directions. Rev-1 compared presence
// and bytes only, so `unreadable → absent` (and back) kept the same
// `absent` label with no bytes and evaded detection.
func TestRev2_ReadabilityTransitionsAreUnstable(t *testing.T) {
	surfaces := []struct {
		name string
		path func(root, slug string) string
	}{
		{"artifacts/apply-recipe.json", func(root, slug string) string { return artifactPath(root, slug, "apply-recipe.json") }},
		{"artifacts/post-apply.patch", func(root, slug string) string { return artifactPath(root, slug, "post-apply.patch") }},
		{"artifacts/recipe-provenance.json", func(root, slug string) string { return artifactPath(root, slug, "recipe-provenance.json") }},
		{"patch-generations.json", func(root, slug string) string { return "" }}, // resolved below
	}
	// The last two directions are the ones rev-1 could not see: BOTH
	// sides carry `Presence == absent` with no bytes, so only the
	// readability flag distinguishes them.
	directions := []string{
		"readable-to-unreadable", "unreadable-to-readable",
		"absent-to-unreadable", "unreadable-to-absent",
	}

	for _, surface := range surfaces {
		for _, dir := range directions {
			t.Run(surface.name+"/"+dir, func(t *testing.T) {
				c := newChainFixture(t)
				c.Feature("target", store.StateApplied)
				c.Artifacts("target", "diff --git a/x b/x\n", ApplyRecipe{Feature: "target"})
				writeProvenance(t, c.Store, "target", c.Base, true)
				mustWriteFile(t, c.Store.PatchGenerationsPath("target"),
					`{"version":1,"feature":"target","generations":[]}`)

				path := surface.path(c.Root, "target")
				if path == "" {
					path = c.Store.PatchGenerationsPath("target")
				}

				body := `{"version":1,"feature":"target","generations":[]}`
				switch dir {
				case "unreadable-to-readable", "unreadable-to-absent":
					makeUnreadable(t, path)
				case "absent-to-unreadable":
					if err := os.RemoveAll(path); err != nil {
						t.Fatal(err)
					}
				}
				before, err := buildInventory(c.Store)
				if err != nil {
					t.Fatal(err)
				}
				switch dir {
				case "readable-to-unreadable", "absent-to-unreadable":
					makeUnreadable(t, path)
				case "unreadable-to-readable":
					makeReadable(t, path, body)
				case "unreadable-to-absent":
					if err := os.RemoveAll(path); err != nil {
						t.Fatal(err)
					}
				}

				got := inventoryInstability(c.Store, before)
				if got == "" {
					t.Fatalf("a %s transition on %s was not detected", dir, surface.name)
				}
				if !strings.Contains(got, "target") {
					t.Errorf("instability must name the slug; got %q", got)
				}
				if !strings.Contains(got, filepath.Base(surface.name)) {
					t.Errorf("instability must name the path %q; got %q", surface.name, got)
				}
			})
		}
	}
}

// TestRev2_StableUnreadableFeatureIsNotUnstable pins the other half: a
// feature that is unreadable for the WHOLE run keeps its existing warn
// and never becomes perpetual instability.
func TestRev2_StableUnreadableFeatureIsNotUnstable(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("target", store.StateApplied)
	c.Feature("outsider", store.StateApplied)
	c.Artifacts("target", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+T\n",
		ApplyRecipe{Feature: "target", Operations: []RecipeOperation{
			{Type: "append-file", Path: "src/parent.txt", Content: "T\n"},
		}})
	c.Artifacts("outsider", "diff --git a/x b/x\n", ApplyRecipe{Feature: "outsider"})
	makeUnreadable(t, artifactPath(c.Root, "outsider", "apply-recipe.json"))

	for i := 0; i < 3; i++ {
		r := c.Verify("target")
		if r.FailedAt == FailedAtSnapshotUnstable {
			t.Fatalf("run %d: a stably-unreadable unrelated feature became instability", i)
		}
		if !hasAdvisory(r, AdvisoryInventoryUnreadable) {
			t.Fatalf("run %d: the unreadable advisory disappeared", i)
		}
	}
}

// TestRev2_WorkspaceMutationDuringRunIsUnstable is the workflow-level
// fixture: a PATH wrapper flips an artifact's readability between git
// calls, with no production hook.
func TestRev2_WorkspaceMutationDuringRunIsUnstable(t *testing.T) {
	c := newChainFixture(t)
	c.Feature("target", store.StateApplied)
	c.Artifacts("target", "diff --git a/src/parent.txt b/src/parent.txt\n--- a/src/parent.txt\n+++ b/src/parent.txt\n@@ -1 +1,2 @@\n parent-seed\n+T\n",
		ApplyRecipe{Feature: "target", Operations: []RecipeOperation{
			{Type: "append-file", Path: "src/parent.txt", Content: "T\n"},
		}})
	installReadabilityFlippingGitWrapper(t, artifactPath(c.Root, "target", "recipe-provenance.json"))

	r := c.Verify("target")
	if r.FailedAt != FailedAtSnapshotUnstable {
		t.Fatalf("failed_at=%q want snapshot-unstable", r.FailedAt)
	}
}

// installReadabilityFlippingGitWrapper creates `path` as an UNREADABLE
// entry (a directory) on the third git invocation — after the capture
// recorded it as absent.
func installReadabilityFlippingGitWrapper(t *testing.T, path string) {
	t.Helper()
	realGit := mustLookGit(t)
	dir := t.TempDir()
	stamp := filepath.Join(dir, "count")
	script := "#!/bin/sh\nn=0\nif [ -f " + stamp + " ]; then n=$(cat " + stamp + "); fi\n" +
		"n=$((n+1))\nprintf '%s' \"$n\" > " + stamp + "\n" +
		"if [ \"$n\" = \"3\" ]; then mkdir -p " + path + "; fi\n" +
		"exec " + realGit + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatalf("write shim: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// makeUnreadable replaces path with a directory: it exists, and reading
// it fails with EISDIR — a non-absence read error.
func makeUnreadable(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func makeReadable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, path, body)
}
