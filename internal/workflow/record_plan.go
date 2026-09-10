package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/safety"
	"github.com/tesseracode/tesserapatch/internal/store"
)

type RecordCollisionMatch struct {
	Slug, Path, SHA256 string
	Bytes, Files       int
}

type RecordCollisionScan struct {
	NewSHA256    string
	NewBytes     int
	SameFeature  bool
	CrossFeature []RecordCollisionMatch
}

// ScanRecordCollisions is the existing record scanner, shared without changing
// its historical unreadable-other-feature tolerance or byte comparison ladder.
func ScanRecordCollisions(s *store.Store, currentSlug, patch string, inventories ...*featureInventory) (RecordCollisionScan, error) {
	result := RecordCollisionScan{}
	result.NewSHA256, result.NewBytes = gitutil.PatchSignature(patch)
	var features []store.FeatureStatus
	var err error
	if len(inventories) != 0 {
		features = inventories[0].Statuses()
	} else {
		features, err = s.ListFeatures()
	}
	if err != nil {
		return result, err
	}
	sort.Slice(features, func(i, j int) bool { return features[i].Slug < features[j].Slug })
	for _, feature := range features {
		path := filepath.Join(s.Root, ".tpatch", "features", feature.Slug, "artifacts", "post-apply.patch")
		var raw []byte
		if len(inventories) != 0 {
			captured := inventories[0].Entry(feature.Slug).Patch
			if captured.Err != nil || captured.Presence == PresenceAbsent {
				continue
			}
			raw = captured.Bytes
		} else {
			info, err := os.Stat(path)
			if err != nil || info.Size() != int64(result.NewBytes) {
				continue
			}
			var readErr error
			raw, readErr = os.ReadFile(path)
			if readErr != nil {
				continue
			}
		}
		if len(raw) != result.NewBytes {
			continue
		}
		hash, _ := gitutil.PatchSignature(string(raw))
		if hash != result.NewSHA256 || string(raw) != patch {
			continue
		}
		if feature.Slug == currentSlug {
			result.SameFeature = true
			continue
		}
		files := 0
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(line, "diff --git") {
				files++
			}
		}
		result.CrossFeature = append(result.CrossFeature, RecordCollisionMatch{
			Slug: feature.Slug, Path: filepath.Join(".tpatch", "features", feature.Slug, "artifacts", "post-apply.patch"),
			SHA256: hash, Bytes: len(raw), Files: files,
		})
	}
	return result, nil
}

func CaptureRecordDiffStat(root string, pathspecs []string) (string, error) {
	text, err := gitutil.CaptureDiffStatScoped(root, pathspecs)
	if err != nil && !errors.Is(err, gitutil.ErrNestedWorktreeDiscovery) {
		return "", nil
	}
	return text, err
}

func RecordPendingBaselineCandidate(status store.FeatureStatus) bool {
	return status.State == store.StateUnapplied || status.LastCommand == "feature unapply" ||
		strings.Contains(status.Notes, "artifacts/unapply/")
}

// DetectRecordAmendOrphans preserves the real producer's missing-signal policy.
func DetectRecordAmendOrphans(s *store.Store, inventories ...*featureInventory) ([]store.FeatureRef, string, bool) {
	read := func(ref string) (string, error) {
		out, _, err := gitutil.RunOfflineGitIn(s.Root, "rev-parse", ref)
		return strings.TrimSpace(out), err
	}
	prev, err := read("HEAD@{1}")
	if err != nil || prev == "" {
		return nil, "", false
	}
	parent, err := read("HEAD@{1}^")
	if err != nil || parent == "" {
		return nil, "", false
	}
	headParent, err := read("HEAD^")
	if err != nil || headParent == "" {
		return nil, "", false
	}
	if parent != headParent {
		return nil, "", true
	}
	head, err := read("HEAD")
	if err != nil || head == "" {
		return nil, "", false
	}
	if prev == head {
		return nil, "", true
	}
	var features []store.FeatureStatus
	if len(inventories) != 0 {
		features = inventories[0].Statuses()
	} else {
		features, err = s.ListFeatures()
		if err != nil {
			return nil, "", false
		}
	}
	deps := make(map[string][]store.FeatureRef)
	for _, feature := range features {
		if sha := feature.Apply.BaseCommit; sha != "" {
			deps[sha] = append(deps[sha], store.FeatureRef{Feature: feature.Slug, Kind: store.FeatureRefKindBaseCommit, SHA: sha})
		}
		for _, dep := range feature.DependsOn {
			if sha := dep.SatisfiedBy; sha != "" {
				deps[sha] = append(deps[sha], store.FeatureRef{Feature: feature.Slug, Kind: store.FeatureRefKindSatisfiedBy, SHA: sha})
			}
		}
	}
	return store.IsAmendBreaking(prev, deps), prev, true
}

type RecordPlan struct {
	Observation  patchobs.Observation
	Publication  CoveragePublicationInput
	Autogen      AutogenOutcome
	RecipeError  error
	Blockers     []string
	complete     bool
	gateFailures []recordGateFailure
}

type recordGateFailure struct{ kind, message string }

// RecordGateError applies only the existing producer's explicit override
// policy. Diagnostics never request these overrides to manufacture feasibility.
func (plan RecordPlan) RecordGateError(forceAmend bool, collisionReason string, lenient, staged bool) error {
	for _, failure := range plan.gateFailures {
		if failure.kind == "amend" && forceAmend ||
			failure.kind == "collision" && strings.TrimSpace(collisionReason) != "" ||
			failure.kind == "roundtrip" && (lenient || staged) {
			continue
		}
		return fmt.Errorf("record refuses: %s", failure.message)
	}
	return nil
}

// PlanRecord is the single read-only record planner. Actual record supplies
// its immutable pre-write observation/publication; diagnostics supply a fresh
// capture, not historical C/E descriptors. Blockers concern the named default
// command without bypass flags. RecipeError is preserved at the producer's
// existing autogen error boundary.
func PlanRecord(s *store.Store, status store.FeatureStatus, obs patchobs.Observation, publication CoveragePublicationInput, autogen, regenerate bool, now time.Time, inventories ...*featureInventory) RecordPlan {
	plan := RecordPlan{Observation: obs, Publication: publication}
	block := func(text string) { plan.Blockers = append(plan.Blockers, text) }
	gate := func(kind, text string) {
		block(text)
		plan.gateFailures = append(plan.gateFailures, recordGateFailure{kind, text})
	}
	if s.Root != obs.RepoRoot || status.Slug != obs.Slug || obs.Producer != patchobs.ProducerRecord {
		gate("identity", "record identity differs from captured inputs")
	}
	if RecordPendingBaselineCandidate(status) {
		// The producer's legacy pending check may use an isolated index.
		// A readonly recommendation cannot run it or assume its answer.
		block("pending-unapply baseline requires manual review; readonly record feasibility is not established")
	}
	if refs, _, ok := DetectRecordAmendOrphans(s, inventories...); ok && len(refs) != 0 {
		gate("amend", "amend would orphan downstream features")
	}
	if !obs.PatchPresent || len(obs.PatchBytes) == 0 {
		gate("empty", "empty capture has no producer event")
	}
	if err := obs.PreflightError(); err != nil {
		gate("capture", "strict capture preflight failed")
	}
	if obs.Capture.Mode != patchobs.CaptureModeWorkingTreeAll {
		block("named default record command does not select this capture mode")
	}
	if len(obs.Capture.Pathspecs) != 0 || len(obs.Capture.ClaimIDs) != 0 {
		block("named default record command does not select this scoped capture")
	}
	if err := recordGenerationGate(s, status.Slug, inventories...); err != nil {
		gate("generations", "patch-generations manifest cannot be read")
	}
	collision, err := ScanRecordCollisions(s, status.Slug, string(obs.PatchBytes), inventories...)
	if err != nil {
		gate("collision-read", "collision scan failed")
	} else if len(collision.CrossFeature) != 0 {
		gate("collision", "cross-feature canonical patch collision")
	}
	if err := gitutil.ValidatePatchReverse(s.Root, string(obs.PatchBytes)); err != nil {
		gate("roundtrip", "captured patch does not round-trip against the working tree")
	}
	if _, err := CaptureRecordDiffStat(s.Root, obs.Capture.Pathspecs); err != nil {
		gate("diffstat", "diffstat nested-worktree discovery failed")
	}
	plan.Autogen, plan.RecipeError = PlanRecipeForRecord(obs, autogen, regenerate, publication, now)
	if plan.RecipeError != nil {
		block("recipe planning failed")
		return plan
	}
	core := RecipeCoverageInput{Observation: obs, Recipe: plan.Autogen.Recipe, Events: CoverageEvents{
		PatchRewritten: true, RecipeRegenerated: plan.Autogen.RecipeWritten,
		StaleMarkerPresent: plan.Autogen.StaleMarkerPresent,
	}}
	c, err := BuildRecipeCoverage(core)
	if err != nil || c.CoverageStatus != CoverageComplete {
		block("current capture cannot publish complete coverage")
	} else {
		raw, encodeErr := EncodeRecipeCoverage(c)
		event, eventErr := BuildRecipeCaptureEvent(core, raw)
		if encodeErr != nil || eventErr != nil {
			block("coverage/capture evidence cannot be encoded")
		} else if err := ValidateRecipeCaptureEventSource(event, raw, core); err != nil {
			block("capture evidence does not prove the planned publication")
		} else if _, err := EncodeRecipeCaptureEvent(event); err != nil {
			block("capture evidence cannot be encoded")
		} else {
			derived, err := DeriveRecipe(obs)
			if err != nil || !derived.ProvesOrigin(plan.Autogen.Recipe.Bytes) {
				block("D16 canonical-byte origin is not established")
			} else {
				plan.complete = true
			}
		}
	}
	for _, rel := range []string{
		"status.json", "record.md", "patch-generations.json", "patches",
		"artifacts/post-apply.patch", "artifacts/post-apply-diff.txt", "artifacts/apply-recipe.json",
		"artifacts/recipe-provenance.json", "artifacts/recipe-stale.json",
		"artifacts/recipe-capture-event.json", "artifacts/recipe-coverage.json",
	} {
		if err := recordPublicationPath(s.Root, filepath.Join(s.Root, ".tpatch", "features", status.Slug, rel)); err != nil {
			block(filepath.ToSlash(rel) + ": " + err.Error())
		}
	}
	plan.Blockers = coverageSortedCopy(plan.Blockers)
	return plan
}

func recordPublicationPath(root, path string) error {
	if err := safety.EnsureSafeRepoPath(root, path); err != nil {
		return fmt.Errorf("unsafe publication path")
	}
	for p := path; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("publication path unreadable")
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("publication path is a symlink")
		}
		if p == path && info.IsDir() && filepath.Base(path) != "patches" {
			return fmt.Errorf("publication artifact is a directory")
		}
		if p == path && !info.Mode().IsRegular() && !info.IsDir() {
			return fmt.Errorf("publication target is not a regular file")
		}
		if p != path && !info.IsDir() {
			return fmt.Errorf("publication parent is not a directory")
		}
		if info.Mode().Perm()&0222 == 0 {
			return fmt.Errorf("publication path is not writable")
		}
		if info.IsDir() && info.Mode().Perm()&0111 == 0 {
			return fmt.Errorf("publication parent is not searchable")
		}
		if p == root {
			return nil
		}
		if filepath.Dir(p) == p {
			return fmt.Errorf("publication path escapes repository")
		}
	}
}

// PlanDefaultRecord captures exactly the named working-tree command. It never
// selects a committed range or emits a phantom observation event.
func PlanDefaultRecord(s *store.Store, status store.FeatureStatus, snap RecipeCoverageSnapshot, now time.Time, inventories ...*featureInventory) RecordPlan {
	patch, err := gitutil.CapturePatchScopedReadOnly(s.Root, nil)
	if err != nil {
		return RecordPlan{Blockers: []string{fmt.Sprintf("default working-tree capture failed: %v", err)}}
	}
	obs := patchobs.Observe(patchobs.Input{
		Producer: patchobs.ProducerRecord, RepoRoot: s.Root, Slug: status.Slug,
		Patch: patch, PatchPresent: true, PreimageRef: "HEAD",
		Capture: patchobs.CaptureDescriptor{Mode: patchobs.CaptureModeWorkingTreeAll},
	})
	var prov RecipeArtifactRead
	if len(inventories) != 0 {
		a := inventories[0].Entry(status.Slug).Provenance
		prov = RecipeArtifactRead{Exists: a.Presence != PresenceAbsent || a.Err != nil, Bytes: a.Bytes, Err: a.Err}
	} else {
		prov = ReadRecipeArtifact(filepath.Join(s.Root, ".tpatch", "features", status.Slug, "artifacts", "recipe-provenance.json"))
	}
	publication := CoveragePublicationInput{
		Observation: obs, Recipe: coverageReadArtifact(status.Slug, "apply-recipe.json", snap.Recipe),
		Provenance:     coverageReadArtifact(status.Slug, "recipe-provenance.json", prov),
		Events:         CoverageEvents{StaleMarkerPresent: snap.Marker.Exists || snap.Marker.Err != nil},
		observationErr: snap.Marker.Err, DeferRecipeWrites: true,
	}
	return PlanRecord(s, status, obs, publication, true, true, now, inventories...)
}

func recordGenerationGate(s *store.Store, slug string, inventories ...*featureInventory) error {
	if len(inventories) == 0 {
		_, err := store.LoadPatchGenerations(s, slug)
		return err
	}
	entry := inventories[0].Entry(slug)
	if entry.GenerationsErr != nil {
		return entry.GenerationsErr
	}
	if entry.Generations == nil {
		return nil
	}
	var manifest store.PatchGenerationsManifest
	decoder := json.NewDecoder(bytes.NewReader(entry.Generations))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return err
	}
	return store.ValidatePatchGenerations(slug, manifest)
}

func (plan RecordPlan) Remediation(slug string) string {
	if plan.complete && plan.Observation.Slug == slug && len(plan.Blockers) == 0 && plan.RecipeError == nil &&
		plan.RecordGateError(false, "", false, false) == nil {
		return "tpatch record " + slug + " --regenerate-recipe"
	}
	if len(plan.Blockers) == 0 {
		plan.Blockers = []string{"complete producer plan is not established"}
	}
	return "recipe-generation-no-truthful-regeneration: no automatic truthful regeneration; blockers: " +
		strings.Join(plan.Blockers, "; ") + "; review the canonical patch and capture/state manually"
}
