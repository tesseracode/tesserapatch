package cli

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/candidate"
	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

type rocS8CLIFixture struct {
	s                         *store.Store
	slug, directory, upstream string
}

func rocS8CLIRead(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func newROCS8CLIFixture(t *testing.T) rocS8CLIFixture {
	t.Helper()
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	gitInitTestRepo(t, root)
	gitRun(t, root, "config", "core.autocrlf", "false")
	base := gitCommitFile(t, root, "a.txt", "head\nremove\nend\n", "candidate base")
	status, err := s.AddFeature(store.AddFeatureInput{Title: "S8 CLI real candidate"})
	if err != nil {
		t.Fatal(err)
	}
	status.State, status.Apply.BaseCommit = store.StateBlocked, base
	if err := s.SaveFeatureStatus(status); err != nil {
		t.Fatal(err)
	}
	cfg, _ := s.LoadConfig()
	cfg.OperationCandidateEnabled = true
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("head\nremove\nfeature\nend\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "diff", "--binary", base, "--", "a.txt")
	command.Dir = root
	command.Env = gitutil.HardenedNoLazyGitEnv()
	patch, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	obs := patchobs.Observe(patchobs.Input{
		Producer: patchobs.ProducerRecord, RepoRoot: root, Slug: status.Slug,
		Patch: string(patch), PatchPresent: true, PreimageRef: base,
		Capture: patchobs.CaptureDescriptor{Mode: patchobs.CaptureModeWorkingTreeAll, Pathspecs: []string{}, ClaimIDs: []string{}},
	})
	derived, err := workflow.DeriveRecipe(obs)
	if err != nil {
		t.Fatal(err)
	}
	recipe := derived.CanonicalBytes()
	if err := s.WriteArtifact(status.Slug, "post-apply.patch", string(patch)); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact(status.Slug, "apply-recipe.json", string(recipe)); err != nil {
		t.Fatal(err)
	}
	_, err = workflow.PublishCoverage(s, workflow.CoveragePublicationInput{
		Observation: obs, Recipe: workflow.CoverageArtifact{Present: true, Bytes: recipe},
		Events: workflow.CoverageEvents{RecipeRegenerated: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	upstream := gitCommitFile(t, root, "a.txt", "head\nend\n", "upstream context deletion")
	results, err := workflow.RunReconcile(context.Background(), s, []string{status.Slug}, "HEAD", nil, provider.Config{}, workflow.ReconcileOptions{})
	if err != nil || len(results) != 1 || results[0].Phase != "phase-2-operation-candidate" {
		t.Fatalf("real CLI candidate fixture: %+v %v", results, err)
	}
	status, err = s.LoadFeatureStatus(status.Slug)
	if err != nil || status.Reconcile.OperationCandidate == nil {
		t.Fatalf("candidate pointer: %+v %v", status, err)
	}
	return rocS8CLIFixture{s, status.Slug, status.Reconcile.OperationCandidate.Path, upstream}
}

func TestROCS8CLIAcceptsRealCandidateHumanAndJSON(t *testing.T) {
	for _, format := range []string{"human", "json"} {
		t.Run(format, func(t *testing.T) {
			f := newROCS8CLIFixture(t)
			out, stderr, code := runCmdExit("reconcile", "--path", f.s.Root, "--format", format, "--accept-candidate", f.slug)
			if code != 0 || stderr != "" || strings.Contains(out, "warning") {
				t.Fatalf("accepted terminal: exit=%d out=%q stderr=%q", code, out, stderr)
			}
			if format == "json" {
				var value map[string]string
				if err := json.Unmarshal([]byte(out), &value); err != nil || value["slug"] != f.slug || value["status"] != "accepted" {
					t.Fatalf("accepted JSON %q: %v", out, err)
				}
			} else if !strings.Contains(out, "Accepted operation candidate for "+f.slug) {
				t.Fatalf("missing accepted terminal line: %q", out)
			}
			status, err := f.s.LoadFeatureStatus(f.slug)
			if err != nil || status.Apply.BaseCommit != f.upstream || status.State != store.StateApplied || status.Reconcile.OperationCandidate != nil {
				t.Fatalf("terminal did not commit acceptance: %+v %v", status, err)
			}
		})
	}
}

func TestROCS8CLIRealRefusalExitCodesHaveNoSuccessOutput(t *testing.T) {
	for _, name := range []string{"cas", "state", "test", "binding", "journal", "bad-resume"} {
		t.Run(name, func(t *testing.T) {
			f := newROCS8CLIFixture(t)
			flag, code, message := "--accept-candidate", 2, "candidate-cas-failed"
			switch name {
			case "cas":
				if err := os.WriteFile(filepath.Join(f.s.Root, "a.txt"), []byte("foreign edit\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "state":
				status, _ := f.s.LoadFeatureStatus(f.slug)
				status.State = store.StateApplied
				if err := f.s.SaveFeatureStatus(status); err != nil {
					t.Fatal(err)
				}
				code, message = 3, "candidate-feature-state-changed"
			case "test":
				cfg, _ := f.s.LoadConfig()
				cfg.TestCommand = "exit 7"
				if err := f.s.SaveConfig(cfg); err != nil {
					t.Fatal(err)
				}
				message = "candidate-test-failed"
			case "binding":
				if err := f.s.WriteArtifact(f.slug, "apply-recipe.json", "{}"); err != nil {
					t.Fatal(err)
				}
				message = "candidate-binding-changed"
			case "journal", "bad-resume":
				journal := filepath.Join(f.s.Root, filepath.FromSlash(f.directory), candidate.CandidateJournalName)
				if err := os.WriteFile(journal, []byte("{}"), 0o600); err != nil {
					t.Fatal(err)
				}
				code, message = 3, "recovery-required"
				if name == "bad-resume" {
					flag, code, message = "--resume-candidate-recovery", 1, "malformed journal"
				}
			}
			out, stderr, got := runCmdExit("reconcile", "--path", f.s.Root, flag, f.slug)
			if got != code || !strings.Contains(stderr, message) || out != "" || strings.Contains(stderr, "warning:") {
				t.Fatalf("%s exit=%d/%d out=%q stderr=%q", name, got, code, out, stderr)
			}
		})
	}
}

type rocS8CLIImage struct {
	present bool
	mode    os.FileMode
	data    string
}

func rocS8CLIImageAt(t *testing.T, name string) rocS8CLIImage {
	t.Helper()
	info, err := os.Lstat(name)
	if os.IsNotExist(err) {
		return rocS8CLIImage{}
	}
	if err != nil {
		t.Fatal(err)
	}
	return rocS8CLIImage{true, info.Mode(), string(rocS8CLIRead(t, name))}
}

func rocS8CLIReadOnlyImages(t *testing.T, f *rocS7CandidateFixture) map[string]rocS8CLIImage {
	t.Helper()
	paths := []string{
		filepath.Join(f.root, "dirty-journal.txt"),
		filepath.Join(f.root, ".tpatch", "features", f.slug, "status.json"),
		f.store.ReconcileEvidencePath(f.slug),
	}
	for _, name := range []string{candidate.CandidateMetadataName, candidate.CandidateStateName, candidate.CandidatePatchName, candidate.CandidateJournalName} {
		paths = append(paths, filepath.Join(f.root, filepath.FromSlash(f.pointerPath), name))
	}
	for _, name := range []string{"post-apply.patch", "apply-recipe.json", "recipe-provenance.json",
		"recipe-capture-event.json", "recipe-coverage.json", "patch-generations.json"} {
		paths = append(paths, filepath.Join(f.root, ".tpatch", "features", f.slug, "artifacts", name))
	}
	images := map[string]rocS8CLIImage{}
	for _, name := range paths {
		images[name] = rocS8CLIImageAt(t, name)
	}
	return images
}

func TestROCS8CLIJournalProbePreservesCleanPreflight(t *testing.T) {
	root, _, slug, tip := autoDropFixture(t, "probe-clean", false)
	if got := gitOut(t, root, "status", "--porcelain", "--untracked-files=all"); got != "" {
		t.Fatalf("fixture is not initially clean: %s", got)
	}
	locks := filepath.Join(root, ".tpatch", "locks")
	if _, err := os.Lstat(locks); !os.IsNotExist(err) {
		t.Fatalf("clean fixture unexpectedly has persistent locks: %v", err)
	}
	for _, slugs := range [][]string{{slug}, nil} {
		args := []string{"reconcile", "--path", root, "--upstream-ref", tip, "--allow-stale-lock", "--preflight"}
		args = append(args, slugs...)
		out, stderr, code := runCmdExit(args...)
		if code != 0 || !strings.Contains(out, "Preflight: clean.") {
			t.Fatalf("journal absence dirtied/refused clean preflight for %v: exit=%d out=%q stderr=%q", slugs, code, out, stderr)
		}
		if got := gitOut(t, root, "status", "--porcelain", "--untracked-files=all"); got != "" {
			t.Fatalf("read-only preflight changed clean-tree behavior: %s", got)
		}
		if _, err := os.Lstat(locks); !os.IsNotExist(err) {
			t.Fatalf("read-only CLI probe created persistent lock files: %v", err)
		}
	}
}

func TestROCS8CLIJournalPreflightPrecedesEveryEarlyReturn(t *testing.T) {
	for _, test := range []struct {
		name                  string
		flags                 []string
		defaultSlugs          bool
		missingStatus         bool
		missingCandidateState bool
		rejected              bool
	}{
		{name: "dirty"},
		{name: "preflight", flags: []string{"--preflight", "--allow-dirty"}},
		{name: "check-applied-only", flags: []string{"--check-applied-only"}},
		{name: "default-slugs", defaultSlugs: true},
		{name: "default-missing-status", defaultSlugs: true, missingStatus: true},
		{name: "default-missing-state", defaultSlugs: true, missingCandidateState: true},
		{name: "preflight-default-missing-both", flags: []string{"--preflight", "--allow-dirty"}, defaultSlugs: true, missingStatus: true, missingCandidateState: true},
		{name: "check-default-missing-both", flags: []string{"--check-applied-only"}, defaultSlugs: true, missingStatus: true, missingCandidateState: true},
		{name: "rejected-slug-filter", rejected: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newROCS7CandidateFixture(t)
			base := filepath.Join(f.root, filepath.FromSlash(f.pointerPath))
			if err := os.WriteFile(filepath.Join(base, candidate.CandidateJournalName), []byte("{malformed but recovery-required"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(f.root, "dirty-journal.txt"), []byte("foreign dirty bytes\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			statusPath := filepath.Join(f.root, ".tpatch", "features", f.slug, "status.json")
			if test.rejected {
				status, err := f.store.LoadFeatureStatus(f.slug)
				if err != nil {
					t.Fatal(err)
				}
				status.State = store.StateRejected
				if err := f.store.SaveFeatureStatus(status); err != nil {
					t.Fatal(err)
				}
			}
			if test.missingStatus {
				if err := os.Remove(statusPath); err != nil {
					t.Fatal(err)
				}
			}
			if test.missingCandidateState {
				if err := os.Remove(filepath.Join(base, candidate.CandidateStateName)); err != nil {
					t.Fatal(err)
				}
			}
			before := rocS8CLIReadOnlyImages(t, f)
			args := []string{"reconcile", "--path", f.root, "--upstream-ref", "HEAD"}
			args = append(args, test.flags...)
			if !test.defaultSlugs {
				args = append(args, f.slug)
			}
			out, stderr, code := runCmdExit(args...)
			if code != 3 || out != "" || !strings.Contains(stderr, "recovery-required") ||
				strings.Contains(stderr, "warning:") || strings.Contains(stderr, "provider") {
				t.Fatalf("journal lost early priority: exit=%d out=%q stderr=%q", code, out, stderr)
			}
			for name, image := range before {
				if got := rocS8CLIImageAt(t, name); got != image {
					t.Errorf("journal preflight mutated %s: got=%+v want=%+v", name, got, image)
				}
			}
		})
	}
}

func TestROCS8CLIPendingWithoutJournalRetainsReadOnlyFlagBehavior(t *testing.T) {
	f := newROCS7CandidateFixture(t)
	before := rocS8CLIReadOnlyImages(t, f)
	for _, flag := range []string{"--preflight", "--check-applied-only"} {
		args := []string{"reconcile", "--path", f.root, "--resolve", "--upstream-ref", "HEAD", flag, f.slug}
		if flag == "--preflight" {
			args = append(args, "--allow-dirty")
		}
		_, stderr, code := runCmdExit(args...)
		if (flag == "--preflight" && code != 0) || strings.Contains(stderr, "recovery-required") || strings.Contains(stderr, "candidate-pending") {
			t.Fatalf("pending/no-journal read-only behavior changed: %s exit=%d stderr=%q", flag, code, stderr)
		}
		for name, image := range before {
			if got := rocS8CLIImageAt(t, name); got != image {
				t.Errorf("read-only reconcile mutated %s: got=%+v want=%+v", name, got, image)
			}
		}
	}
}

func TestROCS8CLIRealRecordSameByteGenerationRebasesAcceptance(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	gitInitTestRepo(t, root)
	gitRun(t, root, "config", "core.autocrlf", "false")
	gitRun(t, root, "config", "core.abbrev", "40")
	baseText := "head\nremove\nend\n"
	base := gitCommitFile(t, root, "a.txt", baseText, "record base B")
	status, err := s.AddFeature(store.AddFeatureInput{Title: "S8 real same-byte record flow"})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.OperationCandidateEnabled = true
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("head\nremove\nfeature\nend\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, stderr, code := runCmdExit("record", "--path", root, "--files", "a.txt", status.Slug)
	if code != 0 {
		t.Fatalf("real record producer failed: exit=%d out=%q stderr=%q", code, out, stderr)
	}
	artifactDir := filepath.Join(root, ".tpatch", "features", status.Slug, "artifacts")
	recorded := rocS8CLIRead(t, filepath.Join(artifactDir, "post-apply.patch"))
	manifest, err := store.LoadPatchGenerations(s, status.Slug)
	if err != nil || len(manifest.Generations) != 1 || manifest.Generations[0].BaseCommit != base {
		t.Fatalf("record did not produce the real B-bound generation: %+v %v", manifest, err)
	}
	var oldRows struct {
		Generations []json.RawMessage `json:"generations"`
	}
	if err := json.Unmarshal(rocS8CLIRead(t, s.PatchGenerationsPath(status.Slug)), &oldRows); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte(baseText), 0o644); err != nil {
		t.Fatal(err)
	}
	upstream := gitCommitFile(t, root, "unrelated.txt", "unrelated U change\n", "upstream U changes another path")
	head := gitCommitFile(t, root, "a.txt", "head\nend\n", "descendant H removes insertion context")
	if base == upstream || upstream == head {
		t.Fatal("fixture did not create distinct B, U and H commits")
	}
	results, err := workflow.RunReconcile(context.Background(), s, []string{status.Slug}, upstream, nil, provider.Config{}, workflow.ReconcileOptions{})
	if err != nil || len(results) != 1 || results[0].Phase != "phase-2-operation-candidate" {
		t.Fatalf("real ancestor-U/HEAD-H reconciliation did not generate a candidate: %+v %v", results, err)
	}
	status, err = s.LoadFeatureStatus(status.Slug)
	if err != nil || status.Reconcile.OperationCandidate == nil {
		t.Fatalf("real candidate pointer is absent: %+v %v", status, err)
	}
	directory := filepath.Join(root, filepath.FromSlash(status.Reconcile.OperationCandidate.Path))
	residual := rocS8CLIRead(t, filepath.Join(directory, candidate.CandidatePatchName))
	if string(residual) != string(recorded) {
		t.Fatalf("normal producer flow did not reproduce raw patch equality:\nrecorded:\n%s\nresidual:\n%s", recorded, residual)
	}
	metadata, err := candidate.DecodeCandidateArtifact(rocS8CLIRead(t, filepath.Join(directory, candidate.CandidateMetadataName)))
	if err != nil || metadata.ReferenceCommit != base || metadata.UpstreamCommit != upstream {
		t.Fatalf("candidate lost the B-to-U base distinction: %+v %v", metadata, err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte(baseText), 0o644); err != nil {
		t.Fatal(err)
	}
	before := map[string]rocS8CLIImage{}
	for _, file := range []string{
		filepath.Join(root, ".git", "index"),
		filepath.Join(directory, candidate.CandidateMetadataName),
		filepath.Join(directory, candidate.CandidatePatchName),
	} {
		before[file] = rocS8CLIImageAt(t, file)
	}
	out, stderr, code = runCmdExit("reconcile", "--path", root, "--accept-candidate", status.Slug)
	if code != 0 || stderr != "" || !strings.Contains(out, "Accepted operation candidate") {
		t.Fatalf("approved same-byte candidate acceptance failed: exit=%d out=%q stderr=%q", code, out, stderr)
	}
	for file, image := range before {
		if got := rocS8CLIImageAt(t, file); got != image {
			t.Errorf("acceptance changed immutable Git/audit data %s: got=%+v want=%+v", file, got, image)
		}
	}
	manifest, err = store.LoadPatchGenerations(s, status.Slug)
	if err != nil || len(manifest.Generations) != 2 || manifest.CurrentGeneration != 2 {
		t.Fatalf("same-byte acceptance did not append: %+v %v", manifest, err)
	}
	var newRows struct {
		Generations []json.RawMessage `json:"generations"`
	}
	if err := json.Unmarshal(rocS8CLIRead(t, s.PatchGenerationsPath(status.Slug)), &newRows); err != nil {
		t.Fatal(err)
	}
	if len(oldRows.Generations) != 1 || len(newRows.Generations) != 2 || string(oldRows.Generations[0]) != string(newRows.Generations[0]) {
		t.Fatal("same-byte acceptance rewrote the record generation")
	}
	status, err = s.LoadFeatureStatus(status.Slug)
	if err != nil {
		t.Fatal(err)
	}
	coverage, err := workflow.DecodeRecipeCoverage(rocS8CLIRead(t, filepath.Join(artifactDir, "recipe-coverage.json")))
	if err != nil {
		t.Fatal(err)
	}
	latest := manifest.Generations[1]
	if status.State != store.StateApplied || status.Reconcile.OperationCandidate != nil || status.Apply.BaseCommit != upstream ||
		coverage.Reference.Commit != upstream || latest.BaseCommit != upstream || latest.Kind != "reconcile" ||
		latest.Capture.Mode != "reconcile" || latest.Upper != (store.GenerationUpper{Kind: "reconcile-result", Ref: upstream, Commit: upstream}) ||
		latest.PatchSHA256 != store.SHA256HexBytes(recorded) || latest.RecipeSHA256 != coverage.RecipeSHA256 {
		t.Fatalf("accepted base/binding disagreement: status=%+v C=%+v generation=%+v", status, coverage, latest)
	}
	if gitHead(t, root) != head || string(rocS8CLIRead(t, filepath.Join(root, "a.txt"))) != "head\nremove\nfeature\nend\n" {
		t.Fatal("ancestor acceptance changed HEAD or failed to install the candidate result")
	}
	if _, err := os.Lstat(filepath.Join(directory, candidate.CandidateJournalName)); !os.IsNotExist(err) {
		t.Fatalf("same-byte acceptance retained a journal: %v", err)
	}
}

// This external-package wire fixture models a crash boundary; its images are
// captured from a real generated candidate, not synthetic eligibility proofs.
func rocS8CLIRecoveryJournal(t *testing.T, f rocS8CLIFixture) []byte {
	t.Helper()
	type image struct {
		Scope   string `json:"scope"`
		Path    string `json:"path"`
		Present bool   `json:"present"`
		Mode    uint32 `json:"mode"`
		Data    []byte `json:"data"`
		SHA256  string `json:"sha256"`
	}
	status, err := f.s.LoadFeatureStatus(f.slug)
	if err != nil {
		t.Fatal(err)
	}
	var images []image
	for _, name := range []string{"a.txt", "artifacts/post-apply.patch", "artifacts/apply-recipe.json",
		"artifacts/recipe-provenance.json", "artifacts/patch-generations.json", "artifacts/recipe-capture-event.json",
		"artifacts/recipe-coverage.json", "status.json"} {
		scope, file := "feature", filepath.Join(f.s.Root, ".tpatch", "features", f.slug, filepath.FromSlash(name))
		if name == "a.txt" {
			scope, file = "worktree", filepath.Join(f.s.Root, name)
		}
		entry := image{Scope: scope, Path: name, Data: []byte{}, SHA256: store.SHA256HexString("")}
		if info, err := os.Stat(file); err == nil {
			entry.Present, entry.Mode, entry.Data = true, uint32(info.Mode()), rocS8CLIRead(t, file)
			entry.SHA256 = store.SHA256HexBytes(entry.Data)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		images = append(images, entry)
	}
	base := filepath.Join(f.s.Root, filepath.FromSlash(f.directory))
	metadata := rocS8CLIRead(t, filepath.Join(base, candidate.CandidateMetadataName))
	pendingInfo, err := os.Stat(filepath.Join(base, candidate.CandidateStateName))
	if err != nil {
		t.Fatal(err)
	}
	journal := struct {
		Version            int                            `json:"version"`
		Feature            string                         `json:"feature"`
		CandidateID        string                         `json:"candidate_id"`
		MetadataSHA256     string                         `json:"metadata_sha256"`
		PreparedSteps      []int                          `json:"prepared_steps"`
		Snapshots          []image                        `json:"snapshots"`
		MissingDirectories []string                       `json:"missing_directories"`
		PendingState       string                         `json:"pending_state"`
		PendingStateMode   uint32                         `json:"pending_state_mode"`
		ApplyBefore        store.ApplySummary             `json:"apply_before"`
		StateBefore        store.FeatureState             `json:"state_before"`
		CommitMarker       string                         `json:"commit_marker"`
		PruneOwnership     *candidate.AcceptanceOwnership `json:"prune_ownership"`
	}{
		Version: 1, Feature: f.slug, CandidateID: status.Reconcile.OperationCandidate.CandidateID,
		MetadataSHA256: store.SHA256HexBytes(metadata), PreparedSteps: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14},
		Snapshots: images, MissingDirectories: []string{},
		PendingState:     string(rocS8CLIRead(t, filepath.Join(base, candidate.CandidateStateName))),
		PendingStateMode: uint32(pendingInfo.Mode()),
		ApplyBefore:      status.Apply, StateBefore: status.State,
	}
	raw, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func TestROCS8CLIExplicitRecoverySuccessAndFailure(t *testing.T) {
	f := newROCS8CLIFixture(t)
	journal := rocS8CLIRecoveryJournal(t, f)
	journalPath := filepath.Join(f.s.Root, filepath.FromSlash(f.directory), candidate.CandidateJournalName)
	if err := os.WriteFile(journalPath, journal, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.s.Root, "a.txt"), []byte("partially accepted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifacts := filepath.Join(f.s.Root, ".tpatch", "features", f.slug, "artifacts")
	if err := os.Rename(artifacts, artifacts+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifacts, []byte("foreign blocker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, stderr, code := runCmdExit("reconcile", "--path", f.s.Root, "--resume-candidate-recovery", f.slug)
	if code != 1 || out != "" || !strings.Contains(stderr, "journal retained") ||
		string(rocS8CLIRead(t, artifacts)) != "foreign blocker\n" || string(rocS8CLIRead(t, journalPath)) != string(journal) {
		t.Fatalf("failed resume: exit=%d out=%q stderr=%q", code, out, stderr)
	}
	if err := os.Remove(artifacts); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(artifacts+".saved", artifacts); err != nil {
		t.Fatal(err)
	}
	out, stderr, code = runCmdExit("reconcile", "--path", f.s.Root, "--format", "json", "--resume-candidate-recovery", f.slug)
	var value map[string]string
	if code != 0 || stderr != "" || json.Unmarshal([]byte(out), &value) != nil ||
		value["slug"] != f.slug || value["candidate_recovery"] != "complete" {
		t.Fatalf("recovered terminal: exit=%d out=%q stderr=%q", code, out, stderr)
	}
	if got := string(rocS8CLIRead(t, filepath.Join(f.s.Root, "a.txt"))); got != "head\nend\n" {
		t.Fatalf("recovery did not restore live preimage: %q", got)
	}
	if _, err := os.Stat(journalPath); !os.IsNotExist(err) {
		t.Fatal("successful explicit recovery retained its journal")
	}
}
