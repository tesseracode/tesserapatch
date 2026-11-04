package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/candidate"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

// Preserve the historical AP/S6 oracles; only the six reviewed S9 skills move.
func rocS9ExpectedDanglingSurfaces(previous map[string]string) (map[string]string, error) {
	delta := []struct{ path, before, after string }{
		{
			"assets/prompts/copilot/tessera-patch-apply.prompt.md",
			"1fabc947282019dd1676997a62318ac1466d53239ced82851eab8c7658e116a8",
			"c39b71cfefbc31723ffa8bb1a5cf18177225bc7fb8e6195e07e4ae512e5c84e4",
		},
		{
			"assets/skills/claude/tessera-patch/SKILL.md",
			"35299f5b3caa450df48b819c812d994a7d1d651bb048a2e8f9ed6331c2a2d6d4",
			"14518245fc0358362e1e9119f71862c0f10f5f35a8db49f30b58e40162b36cc8",
		},
		{
			"assets/skills/copilot/tessera-patch/SKILL.md",
			"c51e23147a46ac749bf75149b84c33f6b7921fe14dee171f9d0c75ca83a559eb",
			"4d16007703040fd03cf1d3815f8b0bc1789ffeee062455d818d91444e9262373",
		},
		{
			"assets/skills/cursor/tessera-patch.mdc",
			"963ab3be368e34a4fc05f1fcc11e7f7f7c7ffe38bb38725d328e63191400bbd8",
			"8f8d6e9db50f66eb5ed9f37f7e5c6a4626f64ba01cf2d7270025bd0147cca50f",
		},
		{
			"assets/skills/windsurf/windsurfrules",
			"c82f2abc4db356ba849f7f56765f8d3146b9e26e84d97175593521c5153c697e",
			"afb6fb77f0873531739e4cf4de38b8df7da6463a34680b142388c67a846f30de",
		},
		{
			"assets/workflows/tessera-patch-generic.md",
			"70becebf81c99fbe59e4bc175c5f13e9028639e6040bf39d137dd8f2623dfed1",
			"8dc1c2b15e9041ef1a075eb6156e0d386b7615b7df9bf0a4c94e8ccb72773a28",
		},
		{
			"docs/feature-layout.md",
			"01d2bdd5b4fbf863cb2759c999a7255b95d9190b5250f94a42e11bc891a3bd8d",
			"01d2bdd5b4fbf863cb2759c999a7255b95d9190b5250f94a42e11bc891a3bd8d",
		},
		{
			"docs/adrs/ADR-035-intent-bundle-publication-and-history.md",
			"edbf901f42e7b7df24ecac8c2b6befae0a5a16136322ccee19634ce4f1af17c8",
			"edbf901f42e7b7df24ecac8c2b6befae0a5a16136322ccee19634ce4f1af17c8",
		},
		{
			"docs/prds/PRD-prepare-intent-bundle.md",
			"3dd28b88f34f5f5f2e523a8738681d473a6255c8bf03d9b80d3a165ab49e3cc9",
			"3dd28b88f34f5f5f2e523a8738681d473a6255c8bf03d9b80d3a165ab49e3cc9",
		},
	}
	if len(previous) != len(delta) {
		return nil, fmt.Errorf("S9 dangling-doc projection requires the unchanged S6 inventory")
	}
	expected := make(map[string]string, len(delta))
	for _, item := range delta {
		if previous[item.path] != item.before {
			return nil, fmt.Errorf("S9 dangling-doc projection refuses S6 drift at %s", item.path)
		}
		expected[item.path] = item.after
	}
	return expected, nil
}

func TestROCS9DanglingDocProjectionAndSensitivity(t *testing.T) {
	// This fixture pins S6 independently without importing its platform-tagged
	// AP harness. The real AP validator chains S6 into the same S9 projection.
	before := map[string]string{
		"assets/prompts/copilot/tessera-patch-apply.prompt.md":       "1fabc947282019dd1676997a62318ac1466d53239ced82851eab8c7658e116a8",
		"assets/skills/claude/tessera-patch/SKILL.md":                "35299f5b3caa450df48b819c812d994a7d1d651bb048a2e8f9ed6331c2a2d6d4",
		"assets/skills/copilot/tessera-patch/SKILL.md":               "c51e23147a46ac749bf75149b84c33f6b7921fe14dee171f9d0c75ca83a559eb",
		"assets/skills/cursor/tessera-patch.mdc":                     "963ab3be368e34a4fc05f1fcc11e7f7f7c7ffe38bb38725d328e63191400bbd8",
		"assets/skills/windsurf/windsurfrules":                       "c82f2abc4db356ba849f7f56765f8d3146b9e26e84d97175593521c5153c697e",
		"assets/workflows/tessera-patch-generic.md":                  "70becebf81c99fbe59e4bc175c5f13e9028639e6040bf39d137dd8f2623dfed1",
		"docs/feature-layout.md":                                     "01d2bdd5b4fbf863cb2759c999a7255b95d9190b5250f94a42e11bc891a3bd8d",
		"docs/adrs/ADR-035-intent-bundle-publication-and-history.md": "edbf901f42e7b7df24ecac8c2b6befae0a5a16136322ccee19634ce4f1af17c8",
		"docs/prds/PRD-prepare-intent-bundle.md":                     "3dd28b88f34f5f5f2e523a8738681d473a6255c8bf03d9b80d3a165ab49e3cc9",
	}
	reviewed := map[string]string{
		"assets/prompts/copilot/tessera-patch-apply.prompt.md": "c39b71cfefbc31723ffa8bb1a5cf18177225bc7fb8e6195e07e4ae512e5c84e4",
		"assets/skills/claude/tessera-patch/SKILL.md":          "14518245fc0358362e1e9119f71862c0f10f5f35a8db49f30b58e40162b36cc8",
		"assets/skills/copilot/tessera-patch/SKILL.md":         "4d16007703040fd03cf1d3815f8b0bc1789ffeee062455d818d91444e9262373",
		"assets/skills/cursor/tessera-patch.mdc":               "8f8d6e9db50f66eb5ed9f37f7e5c6a4626f64ba01cf2d7270025bd0147cca50f",
		"assets/skills/windsurf/windsurfrules":                 "afb6fb77f0873531739e4cf4de38b8df7da6463a34680b142388c67a846f30de",
		"assets/workflows/tessera-patch-generic.md":            "8dc1c2b15e9041ef1a075eb6156e0d386b7615b7df9bf0a4c94e8ccb72773a28",
	}
	original, want := maps.Clone(before), maps.Clone(before)
	for path, digest := range reviewed {
		want[path] = digest
	}
	after, err := rocS9ExpectedDanglingSurfaces(before)
	if err != nil || !maps.Equal(after, want) {
		t.Fatalf("S9 projection differs from the reviewed bodies: got=%v want=%v err=%v", after, want, err)
	}
	if !maps.Equal(before, original) {
		t.Fatal("S9 projection mutated its S6 input")
	}
	changed, unchanged := 0, 0
	for path, digest := range before {
		if after[path] != digest {
			changed++
		} else {
			unchanged++
		}
	}
	if changed != 6 || unchanged != 3 || len(after) != 9 {
		t.Fatalf("S9 projection changed=%d unchanged=%d paths=%d, want 6/3/9", changed, unchanged, len(after))
	}
	reject := func(t *testing.T, wrong map[string]string) {
		t.Helper()
		snapshot := maps.Clone(wrong)
		if got, err := rocS9ExpectedDanglingSurfaces(wrong); err == nil || got != nil {
			t.Fatalf("same S9 projection accepted invalid input: got=%v err=%v", got, err)
		}
		if !maps.Equal(wrong, snapshot) {
			t.Fatal("rejected S9 projection mutated its input")
		}
	}
	extra := maps.Clone(before)
	extra["docs/unowned.md"] = strings.Repeat("a", 64)
	replaced := maps.Clone(extra)
	delete(replaced, "docs/feature-layout.md")
	for name, wrong := range map[string]map[string]string{
		"nil": nil, "empty": {}, "extra-path": extra,
		"replaced-path": replaced, "already-projected": after,
	} {
		t.Run(name, func(t *testing.T) { reject(t, wrong) })
	}
	for path := range before {
		t.Run("missing/"+path, func(t *testing.T) {
			wrong := maps.Clone(before)
			delete(wrong, path)
			reject(t, wrong)
		})
		t.Run("drifted/"+path, func(t *testing.T) {
			wrong := maps.Clone(before)
			wrong[path] = strings.Repeat("a", 64)
			reject(t, wrong)
		})
	}
	for path, digest := range reviewed {
		t.Run("partially-projected/"+path, func(t *testing.T) {
			wrong := maps.Clone(before)
			wrong[path] = digest
			reject(t, wrong)
		})
	}
}

func rocS9FrozenAdjacent(t *testing.T, variant string) (base, feature, upstream, want string) {
	t.Helper()
	frozen := filepath.Join("..", "workflow", "testdata")
	original := filepath.Join(frozen, "rga-s0", "adjacent-cli-args-conflict-2026-08")
	base = string(rocS8CLIRead(t, filepath.Join(original, "base.command.go.txt")))
	if variant == "delete-all" {
		return base,
			string(rocS8CLIRead(t, filepath.Join(original, "feature.command.go.txt"))),
			string(rocS8CLIRead(t, filepath.Join(original, "upstream.command.go.txt"))),
			string(rocS8CLIRead(t, filepath.Join(original, "resolved.command.go.txt")))
	}
	if variant != "between-delete-first" {
		t.Fatalf("unknown frozen adjacent fixture %q", variant)
	}
	return base,
		string(rocS8CLIRead(t, filepath.Join(frozen, "roc-s0", "between-feature.command.go.txt"))),
		string(rocS8CLIRead(t, filepath.Join(frozen, "roc-s0", "between-upstream.command.go.txt"))),
		string(rocS8CLIRead(t, filepath.Join(frozen, "roc-s0", "between-candidate.command.go.txt")))
}

func rocS9CLIHistory(t *testing.T, root string) map[string]rocS8CLIImage {
	t.Helper()
	gitdir := filepath.Join(root, ".git")
	images := map[string]rocS8CLIImage{}
	for _, name := range []string{"HEAD", "index", "packed-refs"} {
		images[name] = rocS8CLIImageAt(t, filepath.Join(gitdir, name))
	}
	for _, name := range []string{"refs", "logs"} {
		err := filepath.WalkDir(filepath.Join(gitdir, name), func(file string, entry fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				relative, err := filepath.Rel(gitdir, file)
				if err != nil {
					return err
				}
				images[filepath.ToSlash(relative)] = rocS8CLIImageAt(t, file)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return images
}

func rocS9AssertCLIImages(t *testing.T, images map[string]rocS8CLIImage) {
	t.Helper()
	for file, want := range images {
		if got := rocS8CLIImageAt(t, file); got != want {
			t.Fatalf("unexpected mutation at %s: got %+v, want %+v", file, got, want)
		}
	}
}

func TestROCS9PendingCandidateSuggestionsExecute(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for i, action := range []struct{ name, flag, state string }{
		{"reject", "--reject-candidate", candidate.CandidateStatusRejected},
		{"accept", "--accept-candidate", candidate.CandidateStatusAccepted},
	} {
		t.Run(action.name, func(t *testing.T) {
			f := newROCS8CLIFixture(t)
			before, err := f.s.LoadFeatureStatus(f.slug)
			if err != nil || before.Reconcile.OperationCandidate == nil {
				t.Fatalf("real pending candidate: %+v %v", before, err)
			}
			directory := filepath.Join(f.s.Root, filepath.FromSlash(f.directory))
			live := filepath.Join(f.s.Root, "a.txt")
			liveBefore := rocS8CLIImageAt(t, live)
			want := string(rocS8CLIRead(t, filepath.Join(directory, candidate.CandidateWorktreeName, "a.txt")))
			featureBefore := rgaS4FeatureBytes(t, f.s.Root, f.slug)
			out, stderr, code := runCmdExit("reconcile", "--path", f.s.Root, "--shadow-diff", f.slug)
			if code != 3 || out != "" || !strings.Contains(stderr, "candidate-pending") ||
				!maps.Equal(featureBefore, rgaS4FeatureBytes(t, f.s.Root, f.slug)) ||
				rocS8CLIImageAt(t, live) != liveBefore {
				t.Fatalf("resolver refusal changed code/state: exit=%d out=%q stderr=%q", code, out, stderr)
			}
			quoted := strings.Split(stderr, "`")
			if len(quoted) != 5 {
				t.Fatalf("expected two copyable suggestions in %q", stderr)
			}
			printed := quoted[2*i+1]
			// Execute the printed words unchanged, apart from selecting this
			// fixture's repository; do not repair a missing subcommand or slug.
			args := append(strings.Fields(strings.TrimPrefix(printed, "tpatch ")), "--path", f.s.Root)
			out, stderr, code = runCmdExit(args...)
			wantOutput := "Accepted operation candidate for " + f.slug + "\n"
			if action.name == "reject" {
				wantOutput = "Rejected operation candidate " + before.Reconcile.OperationCandidate.CandidateID + " for " + f.slug + "\n"
			}
			if code != 0 || stderr != "" || out != wantOutput ||
				printed != "tpatch reconcile "+action.flag+" "+f.slug {
				t.Fatalf("printed suggestion %q is not executable: exit=%d out=%q stderr=%q", printed, code, out, stderr)
			}
			after, err := f.s.LoadFeatureStatus(f.slug)
			if err != nil || after.Reconcile.OperationCandidate != nil {
				t.Fatalf("suggestion did not clear the pending pointer: %+v %v", after, err)
			}
			state, err := candidate.DecodeCandidateState(rocS8CLIRead(t, filepath.Join(directory, candidate.CandidateStateName)))
			if err != nil || state.Status != action.state {
				t.Fatalf("suggestion did not perform %s: %+v %v", action.name, state, err)
			}
			if action.name == "reject" {
				if after.State != before.State || after.Apply.BaseCommit != before.Apply.BaseCommit ||
					rocS8CLIImageAt(t, live) != liveBefore {
					t.Fatal("printed rejection altered the feature's applied base or live tree")
				}
			} else if after.State != store.StateApplied || after.Apply.BaseCommit != f.upstream ||
				string(rocS8CLIRead(t, live)) != want {
				t.Fatal("printed acceptance did not apply the exact candidate and rebind its base")
			}
		})
	}
}

func TestROCS9AdjacentArgumentsDownstreamSoak(t *testing.T) {
	for _, variant := range []string{"delete-all", "between-delete-first"} {
		t.Run(variant, func(t *testing.T) {
			for key, value := range map[string]string{
				"GIT_CONFIG_GLOBAL": os.DevNull, "GIT_CONFIG_NOSYSTEM": "1",
				"GIT_AUTHOR_DATE": "2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE": "2000-01-01T00:00:00Z",
				"GIT_DEFAULT_HASH": "sha1",
			} {
				t.Setenv(key, value)
			}
			base, feature, upstream, want := rocS9FrozenAdjacent(t, variant)
			root := t.TempDir()
			s, err := store.Init(root)
			if err != nil {
				t.Fatal(err)
			}
			gitInitTestRepo(t, root)
			gitRun(t, root, "config", "core.autocrlf", "false")
			baseCommit := gitCommitFile(t, root, "command.go", base, "frozen adjacent base")
			status, err := s.AddFeature(store.AddFeatureInput{Title: "Frozen adjacent arguments", Slug: "add-feature-args"})
			if err != nil {
				t.Fatal(err)
			}
			slug := status.Slug
			cfg, err := s.LoadConfig()
			if err != nil {
				t.Fatal(err)
			}
			cfg.OperationCandidateEnabled, cfg.TestCommand = true, "exit 7"
			cfg.Provider.Type, cfg.Provider.BaseURL, cfg.Provider.Model = "", "", ""
			if err := s.SaveConfig(cfg); err != nil {
				t.Fatal(err)
			}
			live := filepath.Join(root, "command.go")
			if err := os.WriteFile(live, []byte(feature), 0o644); err != nil {
				t.Fatal(err)
			}
			out, stderr, code := runCmdExit("record", "--path", root, "--files", "command.go", slug)
			if code != 0 {
				t.Fatalf("real record: exit=%d out=%q stderr=%q", code, out, stderr)
			}
			status, err = s.LoadFeatureStatus(slug)
			if err != nil || status.State != store.StateApplied || status.Apply.BaseCommit != baseCommit {
				t.Fatalf("recorded status=%+v err=%v", status, err)
			}
			notes := status.Notes
			coverage := rgaS4CLICoverage(t, root, slug)
			rgaS4CLICapturePair(t, root, slug)
			if coverage.CoverageStatus != workflow.CoverageComplete || coverage.Producer != patchobs.ProducerRecord {
				t.Fatalf("frozen insertion did not record complete coverage: %+v", coverage)
			}
			artifacts := filepath.Join(root, ".tpatch", "features", slug, "artifacts")
			inputs := map[string]rocS8CLIImage{}
			for _, name := range []string{"post-apply.patch", "apply-recipe.json", "recipe-provenance.json",
				"patch-generations.json", "recipe-capture-event.json", "recipe-coverage.json"} {
				file := filepath.Join(artifacts, name)
				inputs[file] = rocS8CLIImageAt(t, file)
			}
			upstreamCommit := gitCommitFile(t, root, "command.go", upstream, "intentional upstream argument deletion")
			history := rocS9CLIHistory(t, root)
			reconcile := func(t *testing.T) workflow.ReconcileResult {
				t.Helper()
				out, stderr, code := runCmdExit("reconcile", "--path", root, "--allow-dirty",
					"--upstream-ref", "HEAD", "--format", "json", slug)
				var results []workflow.ReconcileResult
				if code != 0 || json.Unmarshal([]byte(out), &results) != nil || len(results) != 1 {
					t.Fatalf("real reconcile: exit=%d out=%q stderr=%q", code, out, stderr)
				}
				result := results[0]
				if result.Outcome != store.ReconcileStillNeeded || result.Phase != "phase-2-operation-candidate" {
					t.Fatalf("textual-conflict arm did not finalize the candidate: %+v", result)
				}
				if len(result.Evidence) < 2 || result.Evidence[0].Phase != store.EvidencePhase2 ||
					result.Evidence[0].RawReconcileVerdict != string(store.ReconcileStillNeeded) ||
					result.Evidence[1].Phase != store.EvidencePhase4 ||
					result.Evidence[1].RawReconcileVerdict != string(store.ReconcileBlocked) {
					t.Fatalf("missing phase-local candidate/textual-conflict evidence: %+v", result.Evidence)
				}
				return result
			}
			reconcile(t)
			status, err = s.LoadFeatureStatus(slug)
			if err != nil || status.State != store.StateApplied || status.Notes != notes ||
				status.Reconcile.OperationCandidate == nil {
				t.Fatalf("generation accepted/retired or lost the candidate: %+v %v", status, err)
			}
			pointer := *status.Reconcile.OperationCandidate
			directory := filepath.Join(root, filepath.FromSlash(pointer.Path))
			candidateFile := filepath.Join(directory, candidate.CandidateWorktreeName, "command.go")
			metadata, err := candidate.DecodeCandidateArtifact(rocS8CLIRead(t, filepath.Join(directory, candidate.CandidateMetadataName)))
			if err != nil || metadata.UpstreamCommit != upstreamCommit || metadata.ReferenceCommit != baseCommit ||
				metadata.Derivation.EffectCount != 1 || metadata.Derivation.CandidateApplicableCount != 1 ||
				metadata.Derivation.ConflictCount != 0 || len(metadata.Derivation.Operations) != 1 ||
				metadata.Result.Validated != candidate.CandidateValidationStructural {
				t.Fatalf("candidate is not a complete structurally proved insertion: %+v %v", metadata, err)
			}
			if got := string(rocS8CLIRead(t, candidateFile)); got != want {
				t.Fatalf("candidate resurrected upstream deletions or lost arguments:\ngot:\n%s\nwant:\n%s", got, want)
			}
			if got := string(rocS8CLIRead(t, live)); got != upstream {
				t.Fatal("generation changed live bytes before explicit acceptance")
			}
			repeatImages := map[string]rocS8CLIImage{}
			for _, name := range []string{candidate.CandidateMetadataName, candidate.CandidatePatchName, candidate.CandidateStateName} {
				file := filepath.Join(directory, name)
				repeatImages[file] = rocS8CLIImageAt(t, file)
			}
			repeatImages[s.ReconcileEvidencePath(slug)] = rocS8CLIImageAt(t, s.ReconcileEvidencePath(slug))
			reconcile(t)
			status, err = s.LoadFeatureStatus(slug)
			if err != nil || status.Reconcile.OperationCandidate == nil || *status.Reconcile.OperationCandidate != pointer {
				t.Fatalf("identical inputs did not reuse the candidate: %+v %v", status, err)
			}
			rocS9AssertCLIImages(t, repeatImages)
			rocS9AssertCLIImages(t, inputs)
			patch := string(rocS8CLIRead(t, filepath.Join(directory, candidate.CandidatePatchName)))
			out, stderr, code = runCmdExit("reconcile", "--path", root, "--candidate-diff", slug)
			if code != 0 || stderr != "" || out != patch {
				t.Fatalf("review diff: exit=%d out=%q stderr=%q", code, out, stderr)
			}

			unchanged := maps.Clone(inputs)
			for file, image := range repeatImages {
				unchanged[file] = image
			}
			statusPath := filepath.Join(root, ".tpatch", "features", slug, "status.json")
			unchanged[statusPath] = rocS8CLIImageAt(t, statusPath)
			refuse := func(t *testing.T, reason string) {
				t.Helper()
				out, stderr, code := runCmdExit("reconcile", "--path", root, "--accept-candidate", slug)
				if code != 2 || out != "" || !strings.Contains(stderr, reason) {
					t.Fatalf("explicit acceptance must refuse %s: exit=%d out=%q stderr=%q", reason, code, out, stderr)
				}
				rocS9AssertCLIImages(t, unchanged)
				if _, err := os.Lstat(filepath.Join(directory, candidate.CandidateJournalName)); !os.IsNotExist(err) {
					t.Fatalf("pre-transaction refusal left journal: %v", err)
				}
			}
			t.Run("whole-file-resurrection-refused", func(t *testing.T) {
				if err := os.WriteFile(candidateFile, []byte(feature), 0o644); err != nil {
					t.Fatal(err)
				}
				refuse(t, "candidate-binding-changed")
				if got := string(rocS8CLIRead(t, live)); got != upstream {
					t.Fatal("wrong candidate changed the live upstream tree")
				}
				if err := os.WriteFile(candidateFile, []byte(want), 0o644); err != nil {
					t.Fatal(err)
				}
			})
			t.Run("configured-tests-refused", func(t *testing.T) {
				refuse(t, "candidate-test-failed")
				if got := string(rocS8CLIRead(t, live)); got != upstream {
					t.Fatal("failed configured tests changed live bytes")
				}
			})
			cfg.TestCommand = "exit 0"
			if err := s.SaveConfig(cfg); err != nil {
				t.Fatal(err)
			}
			t.Run("live-cas-refused", func(t *testing.T) {
				if err := os.WriteFile(live, []byte(feature), 0o644); err != nil {
					t.Fatal(err)
				}
				refuse(t, "candidate-cas-failed")
				if got := string(rocS8CLIRead(t, live)); got != feature {
					t.Fatal("failed CAS erased the foreign edit")
				}
				if err := os.WriteFile(live, []byte(upstream), 0o644); err != nil {
					t.Fatal(err)
				}
			})

			format, expectedOutput := "human", "Accepted operation candidate for "+slug+"\n"
			if variant == "between-delete-first" {
				format = "json"
				expectedOutput = "{\n  \"slug\": \"" + slug + "\",\n  \"status\": \"accepted\"\n}\n"
			}
			out, stderr, code = runCmdExit("reconcile", "--path", root, "--format", format, "--accept-candidate", slug)
			if code != 0 || stderr != "" || out != expectedOutput {
				t.Fatalf("explicit acceptance: exit=%d out=%q stderr=%q", code, out, stderr)
			}
			if got := string(rocS8CLIRead(t, live)); got != want {
				t.Fatalf("accepted bytes differ from the frozen intent-preserving result:\ngot:\n%s\nwant:\n%s", got, want)
			}
			if !maps.Equal(history, rocS9CLIHistory(t, root)) {
				t.Fatal("candidate generation/review/refusal/acceptance rewrote live index, refs or reflogs")
			}
			status, err = s.LoadFeatureStatus(slug)
			if err != nil || status.State != store.StateApplied || status.Apply.BaseCommit != upstreamCommit ||
				status.Apply.CompletedAt == "" || !status.Apply.HasPatch || !status.Apply.HasRecipe ||
				status.Reconcile.OperationCandidate != nil {
				t.Fatalf("accepted status=%+v err=%v", status, err)
			}
			state, err := candidate.DecodeCandidateState(rocS8CLIRead(t, filepath.Join(directory, candidate.CandidateStateName)))
			if err != nil || state.Status != candidate.CandidateStatusAccepted ||
				state.AcceptValidation != candidate.CandidateAcceptValidationWithRun {
				t.Fatalf("accepted validation=%+v err=%v", state, err)
			}
			coverage = rgaS4CLICoverage(t, root, slug)
			event := rgaS4CLICapturePair(t, root, slug)
			if coverage.CoverageStatus != workflow.CoverageComplete || len(coverage.Reasons) != 0 ||
				coverage.Reference.Commit != upstreamCommit || coverage.Producer != patchobs.ProducerReconcileAccept ||
				coverage.Capture.Mode != patchobs.CaptureModeReconcile || event.Reference.Commit != upstreamCommit {
				t.Fatalf("accepted residual is not completely rebound: C=%+v E=%+v", coverage, event)
			}
			var recipe workflow.ApplyRecipe
			if err := json.Unmarshal(rocS8CLIRead(t, filepath.Join(artifacts, "apply-recipe.json")), &recipe); err != nil {
				t.Fatal(err)
			}
			if len(recipe.Operations) != 1 || recipe.Operations[0].Type != "write-file" ||
				recipe.Operations[0].Path != "command.go" || recipe.Operations[0].Content != want ||
				recipe.Operations[0].PreimageHash == nil ||
				*recipe.Operations[0].PreimageHash != "sha256:"+store.SHA256HexString(upstream) {
				t.Fatalf("accepted recipe is not the exact upstream-to-result residual: %+v", recipe)
			}
			manifest, err := store.LoadPatchGenerations(s, slug)
			if err != nil || len(manifest.Generations) != 2 {
				t.Fatalf("accepted generation=%+v err=%v", manifest, err)
			}
			latest := manifest.Generations[1]
			if latest.Kind != "reconcile" || latest.BaseCommit != upstreamCommit ||
				latest.PatchSHA256 != coverage.PatchSHA256 || latest.RecipeSHA256 != coverage.RecipeSHA256 {
				t.Fatalf("accepted generation disagrees with complete residual: %+v", latest)
			}
			for _, name := range []string{candidate.CandidateWorktreeName, candidate.CandidateJournalName, "staged", "snapshots", ".accept-write", ".accept-prune"} {
				if _, err := os.Lstat(filepath.Join(directory, name)); !os.IsNotExist(err) {
					t.Fatalf("accepted transaction retained %s: %v", name, err)
				}
			}
			for _, name := range []string{candidate.CandidateMetadataName, candidate.CandidatePatchName} {
				file := filepath.Join(directory, name)
				rocS9AssertCLIImages(t, map[string]rocS8CLIImage{file: repeatImages[file]})
			}
			accepted := rgaS4FeatureBytes(t, root, slug)
			out, stderr, code = runCmdExit("reconcile", "--path", root, "--accept-candidate", slug)
			if code != 2 || out != "" || !strings.Contains(stderr, "candidate-absent") ||
				!maps.Equal(accepted, rgaS4FeatureBytes(t, root, slug)) ||
				string(rocS8CLIRead(t, live)) != want {
				t.Fatalf("second acceptance mutated or duplicated the completed transaction: exit=%d out=%q stderr=%q", code, out, stderr)
			}
		})
	}
}
