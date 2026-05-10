package workflow

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// gitInitVerifyAllRepo provisions a minimal git repo at dir with one
// committed file. Mirrors `gitInitVerifyTest` from verify_test.go;
// duplicated here so the Slice D test file does not depend on test
// helpers from a different test concern (Slice C). V7's hard-parent
// closure replay needs a real worktree.
func gitInitVerifyAllRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "tpatch-test@example.com"},
		{"config", "user.name", "tpatch test"},
		{"config", "commit.gpgsign", "false"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "hello.txt"},
		{"commit", "-q", "-m", "init"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

// seedFeature builds a feature in the given state with the supplied
// dependencies. Optionally seeds intent + recipe so RunVerify will
// pass V0–V3.
func seedFeature(t *testing.T, s *store.Store, slug string, state store.FeatureState, deps []store.Dependency, withRecipe bool) {
	t.Helper()
	if _, err := s.AddFeature(store.AddFeatureInput{Title: slug, Slug: slug, Request: "x"}); err != nil {
		t.Fatalf("AddFeature %s: %v", slug, err)
	}
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatalf("LoadFeatureStatus %s: %v", slug, err)
	}
	st.State = state
	st.DependsOn = deps
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatalf("SaveFeatureStatus %s: %v", slug, err)
	}
	if !withRecipe {
		return
	}
	if err := s.WriteFeatureFile(slug, "spec.md", "intent for "+slug); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	if err := s.WriteFeatureFile(slug, "exploration.md", "exploration for "+slug); err != nil {
		t.Fatalf("write exploration.md: %v", err)
	}
	recipe := ApplyRecipe{Feature: slug, Operations: []RecipeOperation{
		{Type: "ensure-directory", Path: "src"},
	}}
	data, err := json.Marshal(recipe)
	if err != nil {
		t.Fatalf("marshal recipe: %v", err)
	}
	if err := s.WriteArtifact(slug, "apply-recipe.json", string(data)); err != nil {
		t.Fatalf("write recipe: %v", err)
	}
}

func newVerifyAllRepo(t *testing.T) *store.Store {
	t.Helper()
	tmp := t.TempDir()
	gitInitVerifyAllRepo(t, tmp)
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	return s
}

// indexOf returns the position of slug in the aggregate report's
// features array; -1 if not present.
func indexOf(r *AggregateReport, slug string) int {
	for i, e := range r.Features {
		if e.Slug == slug {
			return i
		}
	}
	return -1
}

// TestRunVerifyAll_TopoOrdering — 3-feature hard-dep chain (A→B→C) plus
// independent D. Insert in reverse order; expect topological output:
// A, B, C with D positioned at the lex-earliest ready point. The
// Kahn primitive breaks ties lexicographically by slug, so the expected
// order is [A, D, B, C].
func TestRunVerifyAll_TopoOrdering(t *testing.T) {
	s := newVerifyAllRepo(t)
	// Insert in reverse to prove topo wins over insertion order.
	seedFeature(t, s, "feat-c", store.StateApplied, []store.Dependency{{Slug: "feat-b", Kind: store.DependencyKindHard}}, true)
	seedFeature(t, s, "feat-b", store.StateApplied, []store.Dependency{{Slug: "feat-a", Kind: store.DependencyKindHard}}, true)
	seedFeature(t, s, "feat-a", store.StateApplied, nil, true)
	seedFeature(t, s, "feat-d", store.StateApplied, nil, true)

	report, err := RunVerifyAll(s, VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerifyAll: %v", err)
	}
	if len(report.Features) != 4 {
		t.Fatalf("expected 4 feature rows, got %d (%+v)", len(report.Features), report.Features)
	}
	got := []string{
		report.Features[0].Slug,
		report.Features[1].Slug,
		report.Features[2].Slug,
		report.Features[3].Slug,
	}
	// Kahn lex tie-break: ready set after each round —
	// Round 1: {feat-a, feat-d} → emit feat-a (lex first).
	// Round 2: {feat-d, feat-b} → emit feat-b? No — feat-b's parent
	//   feat-a was just satisfied; feat-d was already ready. Lex
	//   first → feat-b? feat-b < feat-d lexicographically. So
	//   feat-b is emitted next.
	// Round 3: {feat-d, feat-c} → feat-c < feat-d → feat-c.
	// Round 4: {feat-d}.
	want := []string{"feat-a", "feat-b", "feat-c", "feat-d"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("topo order mismatch: got %v want %v", got, want)
		}
	}
	// Parent must always precede child.
	if indexOf(report, "feat-a") > indexOf(report, "feat-b") {
		t.Errorf("feat-a must precede feat-b")
	}
	if indexOf(report, "feat-b") > indexOf(report, "feat-c") {
		t.Errorf("feat-b must precede feat-c")
	}
}

// TestRunVerifyAll_PreApplySkip — a feature in state `defined` is
// skipped at its topo position. RunVerify is NOT invoked (verified by
// inspecting that no Verify record was written to status.json) and the
// skip alone does NOT flip the aggregate exit code.
func TestRunVerifyAll_PreApplySkip(t *testing.T) {
	s := newVerifyAllRepo(t)
	seedFeature(t, s, "ready", store.StateApplied, nil, true)
	// Pre-apply: "defined" — without recipe, since RunVerify must not run.
	seedFeature(t, s, "draft", store.StateDefined, nil, false)

	report, err := RunVerifyAll(s, VerifyOptions{})
	if err != nil {
		t.Fatalf("RunVerifyAll: %v", err)
	}
	if len(report.Features) != 2 {
		t.Fatalf("expected 2 feature rows, got %d", len(report.Features))
	}
	idx := indexOf(report, "draft")
	if idx == -1 {
		t.Fatalf("draft feature missing from report")
	}
	if report.Features[idx].Status != AggregateStatusSkipped {
		t.Errorf("draft status=%q want %q", report.Features[idx].Status, AggregateStatusSkipped)
	}
	if !strings.Contains(report.Features[idx].Reason, "pre-apply") {
		t.Errorf("draft reason=%q want substring 'pre-apply'", report.Features[idx].Reason)
	}
	if report.Features[idx].LifecycleState != store.StateDefined {
		t.Errorf("draft lifecycle=%q want %q", report.Features[idx].LifecycleState, store.StateDefined)
	}
	// V0 must NOT have run for the skipped feature: status.json
	// has no Verify field.
	st, err := s.LoadFeatureStatus("draft")
	if err != nil {
		t.Fatal(err)
	}
	if st.Verify != nil {
		t.Errorf("pre-apply skip must not write Verify record; got %+v", st.Verify)
	}
	// Skip alone does not flip the exit gate.
	if report.HasFailures() {
		t.Errorf("pre-apply skip alone must not produce non-zero exit; got summary %+v", report.Summary)
	}
	if report.Summary.Skipped != 1 {
		t.Errorf("Summary.Skipped=%d want 1", report.Summary.Skipped)
	}
}

// TestRunVerifyAll_TopoPositionForSkip — pre-apply slugs participate in
// topo order; their skip row appears at their topo position, NOT at the
// end (CURRENT.md "deterministic and ordered first in topo").
func TestRunVerifyAll_TopoPositionForSkip(t *testing.T) {
	s := newVerifyAllRepo(t)
	// alpha (defined, parent of beta), beta (applied, child of alpha).
	// alpha is pre-apply; should still appear FIRST in topo, even
	// though it is skipped.
	seedFeature(t, s, "beta", store.StateApplied, []store.Dependency{{Slug: "alpha", Kind: store.DependencyKindHard}}, true)
	seedFeature(t, s, "alpha", store.StateDefined, nil, false)

	report, err := RunVerifyAll(s, VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerifyAll: %v", err)
	}
	if len(report.Features) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(report.Features))
	}
	if report.Features[0].Slug != "alpha" {
		t.Errorf("alpha must appear first in topo despite skip; got %q", report.Features[0].Slug)
	}
	if report.Features[0].Status != AggregateStatusSkipped {
		t.Errorf("alpha status=%q want skipped", report.Features[0].Status)
	}
	if report.Features[1].Slug != "beta" {
		t.Errorf("beta must appear second; got %q", report.Features[1].Slug)
	}
}

// TestRunVerifyAll_ExitCodeOnFailure — at least one failed feature
// flips HasFailures(). Use a feature whose recipe is malformed so V2
// fails; the aggregate must report it as failed without poisoning the
// other features.
//
// CARRYOVER LESSON from Slice C external supervisor: artifact-presence
// gates require a malformed-but-present case in the standard matrix.
// The malformed-but-present cell is exercised here: `apply-recipe.json`
// is present but unparseable; `post-apply.patch` is present but
// zero-byte. Both are presence-gated probes inside RunVerify; the
// aggregate must report this feature as failed (V2 first, V8 if V2
// is somehow demoted) WITHOUT cross-contaminating the healthy
// neighbours.
func TestRunVerifyAll_MalformedButPresent_FeatureFailsWithoutPoison(t *testing.T) {
	s := newVerifyAllRepo(t)

	// Healthy neighbours — must remain green.
	seedFeature(t, s, "healthy-a", store.StateApplied, nil, true)
	seedFeature(t, s, "healthy-b", store.StateApplied, nil, true)

	// Malformed feature — recipe is present but garbage; patch is
	// present but zero bytes (mirrors the Slice C rev2 zero-byte
	// presence semantics).
	seedFeature(t, s, "broken", store.StateApplied, nil, false)
	if err := s.WriteFeatureFile("broken", "spec.md", "ok"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteFeatureFile("broken", "exploration.md", "ok"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact("broken", "apply-recipe.json", "{not valid json"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact("broken", "post-apply.patch", ""); err != nil {
		t.Fatal(err)
	}

	report, err := RunVerifyAll(s, VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerifyAll: %v", err)
	}
	if len(report.Features) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(report.Features))
	}
	// Aggregate exit gate fires.
	if !report.HasFailures() {
		t.Errorf("expected HasFailures() true; summary=%+v", report.Summary)
	}
	// Healthy neighbours unchanged.
	for _, slug := range []string{"healthy-a", "healthy-b"} {
		idx := indexOf(report, slug)
		if idx == -1 {
			t.Fatalf("missing %s", slug)
		}
		if report.Features[idx].Status != AggregateStatusPassed {
			t.Errorf("%s status=%q want passed (broken neighbour must not poison run); reason=%q",
				slug, report.Features[idx].Status, report.Features[idx].Reason)
		}
	}
	// Broken feature reports failed.
	idx := indexOf(report, "broken")
	if idx == -1 {
		t.Fatalf("missing broken")
	}
	if report.Features[idx].Status != AggregateStatusFailed {
		t.Errorf("broken status=%q want failed", report.Features[idx].Status)
	}
	if report.Features[idx].Report == nil {
		t.Errorf("broken should carry a per-feature report")
	}
}

// TestRunVerifyAll_AllPassing_ExitGateZero — every feature passes →
// HasFailures() false → caller emits exit 0.
func TestRunVerifyAll_AllPassing_ExitGateZero(t *testing.T) {
	s := newVerifyAllRepo(t)
	seedFeature(t, s, "alpha", store.StateApplied, nil, true)
	seedFeature(t, s, "beta", store.StateApplied, nil, true)

	report, err := RunVerifyAll(s, VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerifyAll: %v", err)
	}
	if report.HasFailures() {
		t.Errorf("expected no failures, got %+v", report.Summary)
	}
	if report.Summary.Passed != 2 {
		t.Errorf("Summary.Passed=%d want 2", report.Summary.Passed)
	}
}

// TestRunVerifyAll_JSONShape — JSON envelope matches the documented
// `{schema_version, features: [...], summary: {passed, failed,
// skipped, error}}` contract.
func TestRunVerifyAll_JSONShape(t *testing.T) {
	s := newVerifyAllRepo(t)
	seedFeature(t, s, "alpha", store.StateApplied, nil, true)
	seedFeature(t, s, "beta", store.StateDefined, nil, false)

	report, err := RunVerifyAll(s, VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerifyAll: %v", err)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		t.Fatalf("encode: %v", err)
	}

	// Re-decode into a generic map so we assert on shape, not on
	// strict struct identity (harness consumers will do the same).
	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := parsed["schema_version"]; !ok {
		t.Errorf("missing schema_version")
	}
	feats, ok := parsed["features"].([]any)
	if !ok {
		t.Fatalf("features array missing or wrong type")
	}
	if len(feats) != 2 {
		t.Errorf("features len=%d want 2", len(feats))
	}
	summary, ok := parsed["summary"].(map[string]any)
	if !ok {
		t.Fatalf("summary missing or wrong type")
	}
	for _, key := range []string{"passed", "failed", "skipped", "error"} {
		if _, ok := summary[key]; !ok {
			t.Errorf("summary missing %q", key)
		}
	}
}

// TestRunVerifyAll_UnreadableStatusJSON_SurfacesAsErrorRow pins the
// revision-1 contract: a feature whose status.json is malformed must
// appear in the aggregate output as an `error` row, must NOT call
// RunVerify, must increment summary.error, and must flip
// HasFailures(). Silent omission (the bug at 19271f7) is a contract
// violation per PRD §9.
func TestRunVerifyAll_UnreadableStatusJSON_SurfacesAsErrorRow(t *testing.T) {
	s := newVerifyAllRepo(t)
	seedFeature(t, s, "good", store.StateApplied, nil, true)
	seedFeature(t, s, "bad", store.StateApplied, nil, true)
	// Corrupt bad's status.json to malformed JSON.
	badStatusPath := filepath.Join(s.Root, ".tpatch", "features", "bad", "status.json")
	if err := os.WriteFile(badStatusPath, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := RunVerifyAll(s, VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerifyAll: %v", err)
	}
	if len(report.Features) != 2 {
		t.Fatalf("expected 2 rows, got %d (%+v)", len(report.Features), report.Features)
	}
	idx := indexOf(report, "bad")
	if idx == -1 {
		t.Fatalf("bad feature missing from aggregate output (the revision-1 bug)")
	}
	if report.Features[idx].Status != AggregateStatusError {
		t.Errorf("bad status=%q want error", report.Features[idx].Status)
	}
	if !strings.Contains(report.Features[idx].Reason, "failed to load status.json") {
		t.Errorf("bad reason=%q must mention status.json load failure", report.Features[idx].Reason)
	}
	if report.Features[idx].Report != nil {
		t.Errorf("bad must not carry a per-feature report (RunVerify must not be called)")
	}
	if report.Summary.Error != 1 {
		t.Errorf("Summary.Error=%d want 1", report.Summary.Error)
	}
	if !report.HasFailures() {
		t.Errorf("HasFailures() must be true when error row present; summary=%+v", report.Summary)
	}
	// Healthy neighbour preserved.
	gIdx := indexOf(report, "good")
	if gIdx == -1 || report.Features[gIdx].Status != AggregateStatusPassed {
		t.Errorf("good must remain passed; got %+v", report.Features[gIdx])
	}
}

// TestRunVerifyAll_EmptyFeatureDir_SilentlyDropped pins the chosen
// semantic for a directory under features/ with NO status.json: it is
// treated as a non-feature directory and silently dropped (matches
// today's behavior for non-feature dirs and keeps `ListFeatureEntries`
// scoped to "looks like a feature" = "has a status.json entry").
func TestRunVerifyAll_EmptyFeatureDir_SilentlyDropped(t *testing.T) {
	s := newVerifyAllRepo(t)
	seedFeature(t, s, "real", store.StateApplied, nil, true)
	if err := os.MkdirAll(filepath.Join(s.Root, ".tpatch", "features", "stray"), 0o755); err != nil {
		t.Fatal(err)
	}

	report, err := RunVerifyAll(s, VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerifyAll: %v", err)
	}
	if len(report.Features) != 1 {
		t.Fatalf("expected 1 row (stray empty dir dropped), got %d (%+v)", len(report.Features), report.Features)
	}
	if report.Features[0].Slug != "real" {
		t.Errorf("expected real, got %q", report.Features[0].Slug)
	}
	if indexOf(report, "stray") != -1 {
		t.Errorf("empty feature dir must not appear in aggregate")
	}
}

// TestRunVerifyAll_StatusJSONIsDirectory_SurfacesAsErrorRow pins the
// chosen semantic for a feature directory whose `status.json` entry is
// itself a directory: presence of the entry signals an attempted
// feature, so it surfaces as an `error` row (not silently dropped).
func TestRunVerifyAll_StatusJSONIsDirectory_SurfacesAsErrorRow(t *testing.T) {
	s := newVerifyAllRepo(t)
	seedFeature(t, s, "ok", store.StateApplied, nil, true)
	weirdDir := filepath.Join(s.Root, ".tpatch", "features", "weird")
	if err := os.MkdirAll(filepath.Join(weirdDir, "status.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	report, err := RunVerifyAll(s, VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerifyAll: %v", err)
	}
	idx := indexOf(report, "weird")
	if idx == -1 {
		t.Fatalf("weird feature must appear as error row, not be dropped")
	}
	if report.Features[idx].Status != AggregateStatusError {
		t.Errorf("weird status=%q want error", report.Features[idx].Status)
	}
	if report.Summary.Error < 1 {
		t.Errorf("Summary.Error must include the directory-as-status case")
	}
	if !report.HasFailures() {
		t.Errorf("HasFailures() must trip when status.json is a directory")
	}
}

// TestRunVerifyAll_EmptyRepo — a tpatch-init repo with no features
// produces an empty features array and a zero-summary aggregate; exit
// gate stays clean.
func TestRunVerifyAll_EmptyRepo(t *testing.T) {
	s := newVerifyAllRepo(t)
	report, err := RunVerifyAll(s, VerifyOptions{NoWrite: true})
	if err != nil {
		t.Fatalf("RunVerifyAll: %v", err)
	}
	if len(report.Features) != 0 {
		t.Errorf("expected 0 features in empty repo, got %d", len(report.Features))
	}
	if report.HasFailures() {
		t.Errorf("empty repo must not flip exit gate")
	}
}
