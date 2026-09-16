package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

type rgaS6Soak struct {
	t         *testing.T
	root      string
	store     *store.Store
	producers map[patchobs.ProducerID]int
}

func (f *rgaS6Soak) run(args ...string) string {
	f.t.Helper()
	args = append(args, "--path", f.root)
	out, stderr, code := runCmdExit(args...)
	if code != 0 {
		f.t.Fatalf("tpatch %v: exit=%d\nstdout=%s\nstderr=%s", args, code, out, stderr)
	}
	return out + stderr
}

func (f *rgaS6Soak) add(slug string) {
	f.t.Helper()
	f.run("add", "--slug", slug, "Cumulative downstream "+slug)
	for _, name := range []string{"spec.md", "exploration.md"} {
		if err := f.store.WriteFeatureFile(slug, name, "Maintain the downstream behavior of "+slug+".\n"); err != nil {
			f.t.Fatal(err)
		}
	}
}

func (f *rgaS6Soak) publication(slug string, producer patchobs.ProducerID, status string) workflow.RecipeCoverageSnapshot {
	f.t.Helper()
	snapshot := workflow.SnapshotRecipeCoverage(f.root, slug)
	assessment := workflow.AssessRecipeCoverage(f.root, slug, snapshot, nil)
	if assessment.BindingFailure() || (assessment.Rung != 3 && assessment.Rung != 4 && assessment.Rung != 6) {
		f.t.Fatalf("%s left missing, stale or unprovable coverage: %+v", producer, assessment)
	}
	if assessment.Coverage.Producer != producer || assessment.Coverage.CoverageStatus != status {
		f.t.Fatalf("publication %s: producer=%s status=%s, want %s/%s",
			slug, assessment.Coverage.Producer, assessment.Coverage.CoverageStatus, producer, status)
	}
	rgaS4CLICapturePair(f.t, f.root, slug)
	f.producers[producer]++
	return snapshot
}

func (f *rgaS6Soak) verify(slug, coverageCode string) workflow.VerifyReport {
	f.t.Helper()
	before := rgaS4FeatureBytes(f.t, f.root, slug)
	out, stderr, code := runCmdExit("verify", slug, "--path", f.root, "--json", "--no-write", "--quiet")
	var report workflow.VerifyReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		f.t.Fatalf("verify %s: invalid JSON: %v\n%s\n%s", slug, err, out, stderr)
	}
	if code != 0 || report.ExitCode != 0 || report.Verdict != "passed" {
		f.t.Fatalf("verify %s did not remain green: exit=%d\n%s\n%s", slug, code, out, stderr)
	}
	found := false
	for _, check := range report.Checks {
		if check.ID != workflow.CheckRecipeGenerationCoverage {
			continue
		}
		found = true
		if coverageCode == "" {
			if !check.Passed {
				f.t.Fatalf("%s complete coverage did not verify: %+v", slug, check)
			}
		} else if check.Passed || check.Severity != workflow.SeverityWarn ||
			!strings.Contains(check.Remediation, coverageCode) {
			f.t.Fatalf("%s lost explicit warning-class coverage: %+v", slug, check)
		}
	}
	if !found {
		f.t.Fatalf("verify %s omitted coverage instead of reporting its authority boundary", slug)
	}
	if after := rgaS4FeatureBytes(f.t, f.root, slug); !maps.Equal(before, after) {
		f.t.Fatalf("verify --no-write changed feature %s", slug)
	}
	return report
}

func (f *rgaS6Soak) repair(slug, path string) workflow.RecipeCoverageSnapshot {
	f.t.Helper()
	f.run("record", slug, "--files", path, "--regenerate-recipe", "--lenient")
	snapshot := f.publication(slug, patchobs.ProducerRecord, "complete")
	f.verify(slug, "")
	return snapshot
}

func rgaS6ValidateProducerLedger(seen map[patchobs.ProducerID]int) error {
	required := []patchobs.ProducerID{
		patchobs.ProducerRecord, patchobs.ProducerFeaturePatch, patchobs.ProducerReconcileAccept,
		patchobs.ProducerCycle, patchobs.ProducerApplyDone, patchobs.ProducerImplement, patchobs.ProducerEdit,
	}
	if len(seen) != len(required) {
		return fmt.Errorf("soak producer inventory differs from P1-P7")
	}
	for _, producer := range required {
		if seen[producer] < 1 {
			return fmt.Errorf("soak did not observe a validated %s publication", producer)
		}
	}
	return nil
}

// Bootstrap with real record/land commands, then install the pre-v0.17 wire
// cohort: ungated recipe, no C/E artifacts, and optionally the historical marker.
// This models an existing repository; it does not claim to execute an old binary.
func (f *rgaS6Soak) legacy(slug string, stale bool) {
	f.t.Helper()
	f.add(slug)
	path := "src/" + slug + ".txt"
	body := "legacy customization\n"
	modesWriteFile(f.t, f.root, path, body)
	recipe, err := workflow.EncodeRecipe(workflow.ApplyRecipe{Feature: slug, Operations: []workflow.RecipeOperation{
		{Type: "write-file", Path: path, Content: body},
	}})
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.store.WriteArtifact(slug, "apply-recipe.json", string(recipe)); err != nil {
		f.t.Fatal(err)
	}
	f.run("record", slug, "--files", path, "--lenient")
	f.run("land", slug, "--message", "Preserve "+slug)
	for _, name := range []string{"recipe-coverage.json", "recipe-capture-event.json"} {
		if err := os.Remove(filepath.Join(featureDirPath(f.store, slug), "artifacts", name)); err != nil {
			f.t.Fatal(err)
		}
	}
	marker := filepath.Join(featureDirPath(f.store, slug), "artifacts", "recipe-stale.json")
	if stale {
		if err := os.WriteFile(marker, []byte("{\"stale\":true,\"reason\":\"patch touches files absent from recipe: src/legacy.txt\",\"detected_at\":\"2026-06-01T00:00:00Z\"}\n"), 0o644); err != nil {
			f.t.Fatal(err)
		}
	} else if err := os.Remove(marker); err != nil && !os.IsNotExist(err) {
		f.t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(featureDirPath(f.store, slug), "artifacts", "apply-recipe.json")); err != nil || !bytes.Equal(got, recipe) {
		f.t.Fatalf("legacy bootstrap changed manual recipe bytes: %v", err)
	}
	rgaS0CommitAll(f.t, f.root)
	f.verify(slug, "recipe-coverage-missing")
}

// RGA-360 keeps one populated fork throughout: two landed legacy features,
// a landed generated customization, a repeatedly maintained feature and an
// unsupported deletion. P1 repairs are explicit operator actions, never replay.
func TestRGAS6CumulativeDownstreamSoak(t *testing.T) {
	root := modesFixture(t, "generated-flags")
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	f := &rgaS6Soak{t: t, root: root, store: s, producers: map[patchobs.ProducerID]int{}}
	modesWriteFile(t, root, "src/server.txt", "serve --port 8080 --timeout 30\n")
	modesWriteFile(t, root, "src/route.txt", "mode=upstream\n")
	modesWriteFile(t, root, "src/obsolete.txt", "remove in downstream\n")
	rgaS0CommitAll(t, root)
	gitRun(t, root, "branch", "upstream-main")

	f.legacy("legacy-clean", false)
	f.legacy("legacy-stale", true)
	legacyBytes := map[string]map[string]string{
		"legacy-clean": rgaS4FeatureBytes(t, root, "legacy-clean"),
		"legacy-stale": rgaS4FeatureBytes(t, root, "legacy-stale"),
	}
	for _, name := range []string{"spec.md", "exploration.md"} {
		if err := s.WriteFeatureFile("generated-flags", name, "Preserve the adjacent port and timeout arguments.\n"); err != nil {
			t.Fatal(err)
		}
	}
	modesWriteFile(t, root, "src/server.txt", "serve --port 8080 --timeout 30 --downstream-cache\n")
	f.repair("generated-flags", "src/server.txt")
	f.run("land", "generated-flags", "--message", "Add downstream cache without changing adjacent arguments")
	generated := f.publication("generated-flags", patchobs.ProducerRecord, "complete")
	c, err := workflow.DecodeRecipeCoverage(generated.Coverage.Bytes)
	if err != nil || c.CrossBaseStatus != workflow.CrossBaseConsumerDerivationRequired {
		t.Fatalf("whole-file capture claimed cross-base safety: %+v %v", c, err)
	}
	f.verify("generated-flags", "")

	const slug, route = "maintained-route", "src/route.txt"
	f.add(slug)
	modesWriteFile(t, root, route, "mode=fork-v1\n")
	first := f.repair(slug, route)

	// P2's changed patch must replace the prior complete envelope, even though
	// the preserved recipe no longer explains it. Then explicitly regenerate.
	modesWriteFile(t, root, route, "mode=fork-v2\n")
	f.run("feature", "patch", "refresh", slug, "--reason", "downstream route revision")
	changed := f.publication(slug, patchobs.ProducerFeaturePatch, "incomplete")
	assessment := workflow.AssessRecipeCoverage(root, slug, changed, nil)
	for _, reason := range []string{"producer-patch-rewrite", "recipe-not-regenerated"} {
		if !slices.Contains(assessment.Reasons, reason) {
			t.Fatalf("P2 lost semantic reason %s: %+v", reason, assessment)
		}
	}
	stale := changed
	stale.Coverage, stale.Event = first.Coverage, first.Event
	if a := workflow.AssessRecipeCoverage(root, slug, stale, nil); !a.BindingFailure() {
		t.Fatalf("same reader accepted stale complete coverage over new patch bytes: %+v", a)
	}
	f.repair(slug, route)

	// A resolved shadow represents the human/provider handoff. Acceptance
	// itself goes through the shipped CLI and its actual P3 finalizer.
	head := gitHead(t, root)
	shadow, err := gitutil.CreateShadow(root, slug, head)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := gitutil.PruneShadow(root, slug); err != nil {
			t.Errorf("clean up resolved soak shadow: %v", err)
		}
	})
	modesWriteFile(t, shadow, route, "mode=fork-v3\n")
	state, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatal(err)
	}
	state.State = store.StateReconcilingShadow
	state.Reconcile.ShadowPath = shadow
	state.Reconcile.UpstreamCommit = head
	state.Reconcile.ResolveSession = "s6-resolved-route"
	if err := s.SaveFeatureStatus(state); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact(slug, "resolution-session.json", `{"outcomes":[{"path":"src/route.txt","status":"resolved"}]}`); err != nil {
		t.Fatal(err)
	}
	f.run("reconcile", "--accept", slug)
	f.publication(slug, patchobs.ProducerReconcileAccept, "incomplete")
	f.repair(slug, route)

	modesWriteFile(t, root, route, "mode=fork-v4\n")
	f.run("cycle", slug)
	f.publication(slug, patchobs.ProducerCycle, "incomplete")
	f.repair(slug, route)

	modesWriteFile(t, root, route, "mode=fork-v5\n")
	f.run("apply", slug, "--mode", "done")
	f.publication(slug, patchobs.ProducerApplyDone, "incomplete")
	f.repair(slug, route)

	f.run("implement", slug, "--manual")
	manual := f.publication(slug, patchobs.ProducerImplement, "incomplete")
	manualCoverage, err := workflow.DecodeRecipeCoverage(manual.Coverage.Bytes)
	if err != nil || manualCoverage.Capture.Mode != patchobs.CaptureModeNoCapture {
		t.Fatalf("manual checkpoint invented a capture: %+v %v", manualCoverage, err)
	}
	f.repair(slug, route)

	recipe := rgaS0ReadRecipe(t, root, slug)
	if len(recipe.Operations) != 1 {
		t.Fatalf("maintained-route fixture lost its single concrete write: %+v", recipe.Operations)
	}
	recipe.Operations[0].Content = "mode=unrecorded-editor-change\n"
	edited, err := workflow.EncodeRecipe(recipe)
	if err != nil {
		t.Fatal(err)
	}
	rgaS4SetEditor(t, "write", string(edited), false)
	f.run("edit", slug, "artifacts/apply-recipe.json")
	f.publication(slug, patchobs.ProducerEdit, "incomplete")
	f.repair(slug, route)
	f.run("land", slug, "--message", "Land maintained downstream route")
	f.publication(slug, patchobs.ProducerRecord, "complete")
	f.verify(slug, "")

	// A deletion is captured truthfully, not hidden behind a partial generated
	// recipe or a complete label. Verify warnings do not make it replay eligible.
	f.add("unsupported-delete")
	if err := os.Remove(filepath.Join(root, "src", "obsolete.txt")); err != nil {
		t.Fatal(err)
	}
	f.run("record", "unsupported-delete", "--files", "src/obsolete.txt", "--lenient")
	unsupported := f.publication("unsupported-delete", patchobs.ProducerRecord, "incomplete")
	a := workflow.AssessRecipeCoverage(root, "unsupported-delete", unsupported, nil)
	if !slices.Contains(a.Reasons, "effect-delete-unsupported") {
		t.Fatalf("unsupported cohort lost its concrete reason: %+v", a)
	}
	f.verify("unsupported-delete", "recipe-coverage-incomplete")

	for _, legacy := range []string{"legacy-clean", "legacy-stale"} {
		f.verify(legacy, "recipe-coverage-missing")
		if !maps.Equal(legacyBytes[legacy], rgaS4FeatureBytes(t, root, legacy)) {
			t.Fatalf("later producers rewrote the untouched legacy cohort %s", legacy)
		}
	}
	f.verify("generated-flags", "")
	f.verify(slug, "")
	if err := rgaS6ValidateProducerLedger(f.producers); err != nil {
		t.Fatal(err)
	}
	missing := make(map[patchobs.ProducerID]int, len(f.producers))
	for producer, count := range f.producers {
		missing[producer] = count
	}
	delete(missing, patchobs.ProducerEdit)
	if err := rgaS6ValidateProducerLedger(missing); err == nil {
		t.Fatal("producer ledger accepted a soak that never exercised P7")
	}

	// The legacy green verdict is not a fixture constant: the same CLI must
	// refuse when its canonical patch is corrupted, then recover after restore.
	path := filepath.Join(featureDirPath(s, "legacy-stale"), "artifacts", "post-apply.patch")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not a canonical patch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, code := runCmdExit("verify", "legacy-stale", "--path", root, "--json", "--no-write", "--quiet"); code == 0 {
		t.Fatal("legacy verifier stayed green when its canonical patch was corrupted")
	}
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	f.verify("legacy-stale", "recipe-coverage-missing")
}
