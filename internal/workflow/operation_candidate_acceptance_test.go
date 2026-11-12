package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/candidate"
	"github.com/tesseracode/tesserapatch/internal/candidatelock"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

type rocS8Fixture struct {
	*rocS2Fixture
	artifact  candidate.CandidateArtifact
	directory string
}

func rocS8Pending(t *testing.T, f *rocS2Fixture) *rocS8Fixture {
	t.Helper()
	git(t, f.s.Root, "config", "core.autocrlf", "false")
	prepared, err := prepareOperationCandidate(f.s, f.slug, f.invocation)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := finalizeOperationCandidate(f.s, f.slug, prepared)
	if err != nil {
		t.Fatal(err)
	}
	directory, err := candidate.CandidateDirectoryRelative(f.slug, artifact.CandidateID)
	if err != nil {
		t.Fatal(err)
	}
	return &rocS8Fixture{rocS2Fixture: f, artifact: artifact, directory: directory}
}

// rocS8NewFixture gives each row a private copy of a per-process template
// (operation_candidate_template_test.go), or a fresh build on Windows.
func rocS8NewFixture(t *testing.T, creation bool) *rocS8Fixture {
	t.Helper()
	if !rocS8TemplateSupported() {
		return rocS8BuildFixture(t, creation)
	}
	return rocS8TemplateFor(t, creation).instantiate(t)
}

func rocS8BuildFixture(t *testing.T, creation bool) *rocS8Fixture {
	t.Helper()
	if creation {
		return rocS8Pending(t, rocS2NewFixture(t, true))
	}
	f := rocS6PhaseFixture(t, rocS6UpstreamDeletion)
	git(t, f.s.Root, "config", "core.autocrlf", "false")
	result := rocS6Run(t, f, nil, provider.Config{}, ReconcileOptions{})
	if result.Phase != "phase-2-operation-candidate" {
		t.Fatalf("fixture did not reach actual candidate generation: %+v", result)
	}
	status, err := f.s.LoadFeatureStatus(f.slug)
	if err != nil || status.Reconcile.OperationCandidate == nil {
		t.Fatalf("candidate pointer: %+v %v", status, err)
	}
	directory := status.Reconcile.OperationCandidate.Path
	raw, err := os.ReadFile(filepath.Join(f.s.Root, filepath.FromSlash(directory), candidate.CandidateMetadataName))
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := candidate.DecodeCandidateArtifact(raw)
	if err != nil {
		t.Fatal(err)
	}
	return &rocS8Fixture{rocS2Fixture: f, artifact: artifact, directory: directory}
}

func (f *rocS8Fixture) candidatePath(name string) string {
	return filepath.Join(f.s.Root, filepath.FromSlash(f.directory), name)
}

func (f *rocS8Fixture) featurePath(name string) string {
	return filepath.Join(f.s.Root, ".tpatch", "features", f.slug, filepath.FromSlash(name))
}

type rocS8Image struct {
	present bool
	mode    os.FileMode
	data    string
}

func rocS8ReadImage(t *testing.T, name string) rocS8Image {
	t.Helper()
	info, err := os.Lstat(name)
	if os.IsNotExist(err) {
		return rocS8Image{}
	}
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() {
		return rocS8Image{present: true, mode: info.Mode()}
	}
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return rocS8Image{true, info.Mode(), string(raw)}
}

func rocS8Snapshot(t *testing.T, f *rocS8Fixture) map[string]rocS8Image {
	t.Helper()
	out := map[string]rocS8Image{}
	for _, name := range []string{
		"artifacts/post-apply.patch", "artifacts/apply-recipe.json",
		"artifacts/recipe-provenance.json", "artifacts/recipe-capture-event.json",
		"artifacts/recipe-coverage.json", "artifacts/patch-generations.json", "status.json",
	} {
		file := f.featurePath(name)
		out[file] = rocS8ReadImage(t, file)
	}
	for _, name := range operationCandidateArtifactPaths(f.artifact) {
		file := filepath.Join(f.s.Root, filepath.FromSlash(name))
		out[file] = rocS8ReadImage(t, file)
	}
	for _, name := range []string{candidate.CandidateMetadataName, candidate.CandidatePatchName, candidate.CandidateStateName} {
		file := f.candidatePath(name)
		out[file] = rocS8ReadImage(t, file)
	}
	return out
}

func rocS8AssertImages(t *testing.T, before map[string]rocS8Image) {
	t.Helper()
	for file, want := range before {
		if got := rocS8ReadImage(t, file); got != want {
			t.Errorf("%s was not restored exactly:\n got %+v\nwant %+v", file, got, want)
		}
	}
}

func rocS8Hook(t *testing.T, hook func(operationCandidateAcceptStep, string, string) error) {
	t.Helper()
	old := operationCandidateAcceptanceHook
	operationCandidateAcceptanceHook = hook
	t.Cleanup(func() { operationCandidateAcceptanceHook = old })
}

func rocS8Code(t *testing.T, err error, want string) {
	t.Helper()
	code, _ := OperationCandidateActionCode(err)
	if err == nil || code != want {
		t.Fatalf("error=%v code=%q; want %q", err, code, want)
	}
}

func rocS8NoTransaction(t *testing.T, f *rocS8Fixture) {
	t.Helper()
	for _, name := range []string{candidate.CandidateJournalName, "staged", "snapshots", ".accept-write", operationCandidatePruneDirectory} {
		if _, err := os.Lstat(f.candidatePath(name)); !os.IsNotExist(err) {
			t.Errorf("transaction residue %s: %v", name, err)
		}
	}
}

func rocS8GitIdentity(t *testing.T, root string) map[string]rocS8Image {
	t.Helper()
	identity := map[string]rocS8Image{
		"logical refs": {present: true, data: gitOutput(t, root, "for-each-ref", "--format=%(refname) %(objectname)")},
	}
	gitdir := filepath.Join(root, ".git")
	for _, name := range []string{"HEAD", "index", "packed-refs"} {
		identity[name] = rocS8ReadImage(t, filepath.Join(gitdir, name))
	}
	// Read the actual shared/main-worktree logs, not `reflog --all`: that
	// pseudo-option also expands detached candidate HEADs unless the
	// --single-worktree switch precedes it. Raw images also catch changes
	// hidden by the old %H/%gs projection (author, date, old OID and mode).
	for _, directory := range []string{"refs", "logs"} {
		err := filepath.WalkDir(filepath.Join(gitdir, directory), func(name string, entry fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				relative, err := filepath.Rel(gitdir, name)
				if err != nil {
					return err
				}
				identity[filepath.ToSlash(relative)] = rocS8ReadImage(t, name)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return identity
}

func rocS8GitIdentityChanges(before, after map[string]rocS8Image) []string {
	var names, changes []string
	for name := range before {
		names = append(names, name)
	}
	for name := range after {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range slices.Compact(names) {
		a, b := before[name], after[name]
		if a != b {
			changes = append(changes, fmt.Sprintf("%s: before present=%t mode=%s bytes=%d sha256=%s; after present=%t mode=%s bytes=%d sha256=%s",
				name, a.present, a.mode, len(a.data), store.SHA256HexString(a.data),
				b.present, b.mode, len(b.data), store.SHA256HexString(b.data)))
		}
	}
	return changes
}

func TestROCS8LiveGitIdentityDetectsIndexRefsAndReflogChanges(t *testing.T) {
	f := rocS8NewFixture(t, false)
	git(t, f.s.Root, "config", "core.logAllRefUpdates", "true")
	head := strings.TrimSpace(gitOutput(t, f.s.Root, "rev-parse", "HEAD"))
	git(t, f.s.Root, "update-ref", "--create-reflog", "-m", "identity fixture", "HEAD", f.artifact.ReferenceCommit)
	git(t, f.s.Root, "update-ref", "--create-reflog", "-m", "identity fixture restore", "HEAD", head)
	before := rocS8GitIdentity(t, f.s.Root)
	for _, name := range []string{"index", "logs/HEAD"} {
		file := filepath.Join(f.s.Root, ".git", filepath.FromSlash(name))
		original := rocS8ReadImage(t, file)
		if err := os.WriteFile(file, []byte(original.data+"\n"), original.mode.Perm()); err != nil {
			t.Fatal(err)
		}
		changes := rocS8GitIdentityChanges(before, rocS8GitIdentity(t, f.s.Root))
		if len(changes) == 0 || !strings.Contains(strings.Join(changes, "\n"), name+":") {
			t.Fatalf("same Git invariant validator missed changed %s: %v", name, changes)
		}
		if err := os.WriteFile(file, []byte(original.data), original.mode.Perm()); err != nil {
			t.Fatal(err)
		}
	}
	git(t, f.s.Root, "update-ref", "refs/heads/s8-identity-negative", "HEAD")
	changes := rocS8GitIdentityChanges(before, rocS8GitIdentity(t, f.s.Root))
	if len(changes) == 0 || !strings.Contains(strings.Join(changes, "\n"), "logical refs:") {
		t.Fatalf("same Git invariant validator missed a real ref mutation: %v", changes)
	}
	git(t, f.s.Root, "update-ref", "-d", "refs/heads/s8-identity-negative")
	if changes := rocS8GitIdentityChanges(before, rocS8GitIdentity(t, f.s.Root)); len(changes) != 0 {
		t.Fatalf("identity mutation fixture did not restore Git state: %v", changes)
	}
}

func TestROCS8AcceptAncestorDirtyTreeAndReeligibility(t *testing.T) {
	f := rocS8NewFixture(t, false)
	for _, text := range []string{"one\n", "two\n"} {
		writeRepoFile(t, f.s, "unrelated.txt", []byte(text))
		git(t, f.s.Root, "add", "unrelated.txt")
		git(t, f.s.Root, "commit", "-qm", "unrelated descendant")
	}
	writeRepoFile(t, f.s, "unrelated.txt", []byte("unrelated dirty bytes\n"))
	writeRepoFile(t, f.s, "untracked.txt", []byte("untracked dirt\n"))
	cfg, _ := f.s.LoadConfig()
	cfg.TestCommand = "exit 0"
	if err := f.s.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	audit := rocS8ReadImage(t, f.candidatePath(candidate.CandidateMetadataName))
	reviewDiff := rocS8ReadImage(t, f.candidatePath(candidate.CandidatePatchName))
	gitBefore := rocS8GitIdentity(t, f.s.Root)
	var publications []string
	var prepared operationCandidateJournal
	rocS8Hook(t, func(step operationCandidateAcceptStep, point, name string) error {
		if step == candidateStepApply && point == "before" {
			for _, name := range []string{"post-apply.patch", "apply-recipe.json", "recipe-provenance.json",
				"patch-generations.json", "recipe-capture-event.json", "recipe-coverage.json"} {
				if _, err := os.Stat(f.candidatePath(filepath.Join("staged", name))); err != nil {
					t.Errorf("staged %s not ready before first live write: %v", name, err)
				}
			}
			raw, err := os.ReadFile(f.candidatePath(candidate.CandidateJournalName))
			if err != nil {
				t.Fatal(err)
			}
			prepared, err = decodeOperationCandidateJournal(raw)
			if err != nil {
				t.Fatal(err)
			}
			if got := rocS8ReadImage(t, filepath.Join(f.s.Root, "a.txt")).data; got != rocS6UpstreamDeletion {
				t.Fatal("live content changed before step 10")
			}
		}
		if step == candidateStepPublish && point == "published" {
			publications = append(publications, name)
		}
		if point == "before" || point == "held" {
			for _, lockPath := range []string{filepath.Join("reconcile", f.slug+".lock"), "worktree-mutation.lock"} {
				probe, err := candidatelock.TryAcquire(filepath.Join(f.s.Root, ".tpatch", "locks", lockPath))
				if err == nil {
					_ = probe.Release()
					t.Errorf("transaction temporarily released %s at step %d", lockPath, step)
				} else if !candidatelock.IsCode(err, candidatelock.CodeCandidateLockHeld) {
					t.Error(err)
				}
			}
		}
		return nil
	})
	if err := AcceptOperationCandidate(f.s, f.slug); err != nil {
		t.Fatal(err)
	}
	if changes := rocS8GitIdentityChanges(gitBefore, rocS8GitIdentity(t, f.s.Root)); len(changes) != 0 {
		t.Fatalf("acceptance mutated live Git state:\n%s", strings.Join(changes, "\n"))
	}
	if got := rocS8ReadImage(t, filepath.Join(f.s.Root, "a.txt")).data; got != "head\nfeature\nend\n" {
		t.Fatalf("live candidate result %q", got)
	}
	if got := rocS8ReadImage(t, filepath.Join(f.s.Root, "unrelated.txt")).data; got != "unrelated dirty bytes\n" {
		t.Fatal("acceptance changed unrelated tracked dirt")
	}
	if got := rocS8ReadImage(t, filepath.Join(f.s.Root, "untracked.txt")).data; got != "untracked dirt\n" {
		t.Fatal("acceptance changed unrelated untracked dirt")
	}
	wantPublications := []string{"artifacts/post-apply.patch", "artifacts/apply-recipe.json", "artifacts/recipe-provenance.json",
		"artifacts/patch-generations.json", "artifacts/recipe-capture-event.json", "artifacts/recipe-coverage.json"}
	if !slices.Equal(publications, wantPublications) || len(prepared.Snapshots) != 8 ||
		prepared.CommitMarker != "" || prepared.PruneOwnership == nil {
		t.Fatalf("publication/snapshot inventory: %v %+v", publications, prepared)
	}
	if rocS8ReadImage(t, f.candidatePath(candidate.CandidateMetadataName)) != audit ||
		rocS8ReadImage(t, f.candidatePath(candidate.CandidatePatchName)) != reviewDiff {
		t.Fatal("accepted immutable audit changed")
	}
	stateRaw := rocS8ReadImage(t, f.candidatePath(candidate.CandidateStateName)).data
	state, err := candidate.DecodeCandidateState([]byte(stateRaw))
	if err != nil || state.Status != candidate.CandidateStatusAccepted || state.AcceptValidation != candidate.CandidateAcceptValidationWithRun {
		t.Fatalf("accepted state: %+v %v", state, err)
	}
	status, err := f.s.LoadFeatureStatus(f.slug)
	if err != nil || status.State != store.StateApplied || status.Apply.BaseCommit != f.artifact.UpstreamCommit ||
		status.Apply.CompletedAt == "" || !status.Apply.HasPatch || !status.Apply.HasRecipe || status.Reconcile.OperationCandidate != nil {
		t.Fatalf("accepted status: %+v %v", status, err)
	}
	rocS8NoTransaction(t, f)
	if _, err := os.Stat(f.candidatePath("worktree")); !os.IsNotExist(err) {
		t.Fatalf("accepted worktree was not pruned: %v", err)
	}
	invocation := f.invocation
	invocation.upstreamCommit = strings.TrimSpace(gitOutput(t, f.s.Root, "rev-parse", "HEAD"))
	eligibility, err := assessOperationCandidateEligibility(f.s, f.slug, invocation)
	if err != nil || !eligibility.eligible || eligibility.referenceCommit != f.artifact.UpstreamCommit {
		t.Fatalf("new pair not eligible: %+v %v", eligibility, err)
	}
	coverage, err := DecodeRecipeCoverage([]byte(rocS8ReadImage(t, f.featurePath("artifacts/recipe-coverage.json")).data))
	if err != nil || coverage.Producer != patchobs.ProducerReconcileAccept || coverage.Capture.Mode != patchobs.CaptureModeReconcile ||
		coverage.CoverageStatus != CoverageComplete || len(coverage.Reasons) != 0 {
		t.Fatalf("P3 publication: %+v %v", coverage, err)
	}
	manifest, err := store.LoadPatchGenerations(f.s, f.slug)
	if err != nil || validateOperationCandidateBaseConsistency(status.Apply.BaseCommit, coverage, manifest) != nil {
		t.Fatalf("base agreement: %+v %v", manifest, err)
	}
	for _, wrong := range []string{"", f.artifact.ReferenceCommit, strings.Repeat("f", 40)} {
		if wrong != status.Apply.BaseCommit && validateOperationCandidateBaseConsistency(wrong, coverage, manifest) == nil {
			t.Fatalf("same base validator accepted stale base %q", wrong)
		}
	}
}

func TestROCS8AcceptanceRefusalsHaveNoTransactionWrites(t *testing.T) {
	for _, name := range []string{"content", "mode", "directory", "diverged", "source-state", "event", "coverage", "patch", "recipe",
		"forged-operation", "forged-tree", "candidate-bytes", "candidate-diff", "rejected", "accepted", "rematerialization",
		"bindings-during-tests", "state-during-tests", "test-fails", "test-mutates"} {
		t.Run(name, func(t *testing.T) {
			f := rocS8NewFixture(t, false)
			code := "candidate-binding-changed"
			switch name {
			case "content":
				writeRepoFile(t, f.s, "a.txt", []byte("foreign edit\n"))
				code = "candidate-cas-failed"
			case "mode":
				mode := os.FileMode(0o755)
				if runtime.GOOS == "windows" {
					mode = 0o444
					t.Cleanup(func() { _ = os.Chmod(filepath.Join(f.s.Root, "a.txt"), 0o666) })
				}
				if err := os.Chmod(filepath.Join(f.s.Root, "a.txt"), mode); err != nil {
					t.Fatal(err)
				}
				code = "candidate-cas-failed"
			case "directory":
				if err := os.Remove(filepath.Join(f.s.Root, "a.txt")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(f.s.Root, "a.txt"), 0o755); err != nil {
					t.Fatal(err)
				}
				code = "candidate-cas-failed"
			case "diverged":
				git(t, f.s.Root, "checkout", "--detach", f.artifact.ReferenceCommit)
				writeRepoFile(t, f.s, "a.txt", []byte(rocS6UpstreamDeletion))
				code = "head-diverged"
			case "source-state":
				f.changeStatus(t, func(s *store.FeatureStatus) { s.State = store.StateApplied })
				code = "candidate-feature-state-changed"
			case "event", "coverage", "patch", "recipe":
				files := map[string]string{"event": "recipe-capture-event.json", "coverage": "recipe-coverage.json",
					"patch": "post-apply.patch", "recipe": "apply-recipe.json"}
				if name == "event" {
					f.event.Capture.Mode = patchobs.CaptureModeNoCapture
					raw, _ := json.Marshal(f.event)
					f.write(t, files[name], raw)
				} else {
					file := f.featurePath("artifacts/" + files[name])
					before := rocS8ReadImage(t, file)
					if err := os.WriteFile(file, []byte(before.data+"\n"), before.mode); err != nil {
						t.Fatal(err)
					}
				}
			case "forged-operation", "forged-tree":
				artifact := f.artifact
				if name == "forged-operation" {
					artifact.Derivation.Operations[0].InsertionSHA256 = strings.Repeat("d", 64)
				} else {
					artifact.Result.CandidateTreeSHA256 = strings.Repeat("e", 64)
				}
				raw, err := candidate.EncodeCandidateArtifact(artifact)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(f.candidatePath(candidate.CandidateMetadataName), raw, 0o644); err != nil {
					t.Fatal(err)
				}
			case "candidate-bytes":
				if err := os.WriteFile(f.candidatePath(filepath.Join("worktree", "a.txt")), []byte("forged\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "candidate-diff":
				if err := os.WriteFile(f.candidatePath(candidate.CandidatePatchName), []byte("forged diff\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "rejected", "accepted":
				state, err := candidate.DecodeCandidateState([]byte(rocS8ReadImage(t, f.candidatePath(candidate.CandidateStateName)).data))
				if err != nil {
					t.Fatal(err)
				}
				state.Status = name
				raw, _ := candidate.EncodeCandidateState(state)
				if err := os.WriteFile(f.candidatePath(candidate.CandidateStateName), raw, 0o644); err != nil {
					t.Fatal(err)
				}
				code = "candidate-not-pending"
				if name == "accepted" {
					code = "candidate-already-accepted"
				}
			case "rematerialization":
				rocS8Hook(t, func(step operationCandidateAcceptStep, point, _ string) error {
					if step == candidateStepRematerialize && point == "before" {
						return fmt.Errorf("forced rematerialization refusal")
					}
					return nil
				})
			case "bindings-during-tests":
				rocS8Hook(t, func(step operationCandidateAcceptStep, point, _ string) error {
					if step == candidateStepRematerialize && point == "before" {
						f.write(t, "apply-recipe.json", append(bytes.Clone(f.recipe), '\n'))
					}
					return nil
				})
			case "state-during-tests":
				code = "candidate-feature-state-changed"
				rocS8Hook(t, func(step operationCandidateAcceptStep, point, _ string) error {
					if step == candidateStepRematerialize && point == "before" {
						f.changeStatus(t, func(status *store.FeatureStatus) { status.State = store.StateApplied })
					}
					return nil
				})
			case "test-fails", "test-mutates":
				cfg, _ := f.s.LoadConfig()
				cfg.TestCommand = "exit 29"
				code = "candidate-test-failed"
				if name == "test-mutates" {
					cfg.TestCommand = "echo changed > a.txt"
					code = "candidate-binding-changed"
				}
				if err := f.s.SaveConfig(cfg); err != nil {
					t.Fatal(err)
				}
			}
			before := rocS8Snapshot(t, f)
			if name == "bindings-during-tests" {
				delete(before, f.featurePath("artifacts/apply-recipe.json"))
			}
			if name == "state-during-tests" {
				delete(before, f.featurePath("status.json"))
			}
			if name == "directory" {
				// The occupied directory is deliberately not a regular-file image.
				delete(before, filepath.Join(f.s.Root, "a.txt"))
			}
			rocS8Code(t, AcceptOperationCandidate(f.s, f.slug), code)
			rocS8AssertImages(t, before)
			rocS8NoTransaction(t, f)
		})
	}
}

func TestROCS8CreationCollisionsAndSecondCAS(t *testing.T) {
	for _, name := range []string{"untracked", "directory", "symlink", "between-checks"} {
		t.Run(name, func(t *testing.T) {
			f := rocS8NewFixture(t, true)
			target := filepath.Join(f.s.Root, "a.txt")
			switch name {
			case "untracked":
				writeRepoFile(t, f.s, "a.txt", []byte("foreign creation\n"))
			case "directory":
				if err := os.Mkdir(target, 0o755); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				writeRepoFile(t, f.s, "outside.txt", []byte("outside\n"))
				if err := os.Symlink(filepath.Join(f.s.Root, "outside.txt"), target); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			case "between-checks":
				rocS8Hook(t, func(step operationCandidateAcceptStep, point, _ string) error {
					if step == candidateStepApply && point == "before" {
						writeRepoFile(t, f.s, "a.txt", []byte("intervening creation\n"))
					}
					return nil
				})
			}
			code := "candidate-path-occupied"
			if name == "between-checks" {
				code = "candidate-cas-failed"
			}
			rocS8Code(t, AcceptOperationCandidate(f.s, f.slug), code)
			if name != "between-checks" {
				rocS8NoTransaction(t, f)
				if _, err := os.Lstat(target); err != nil {
					t.Fatal("collision was destroyed")
				}
				return
			}
			if got := rocS8ReadImage(t, target).data; got != "intervening creation\n" {
				t.Fatal("no-write CAS refusal eagerly erased foreign creation")
			}
			rocS8Code(t, AcceptOperationCandidate(f.s, f.slug), "recovery-required")
			if err := ResumeOperationCandidateRecovery(f.s, f.slug); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatal("explicit recovery did not restore creation absence")
			}
			rocS8NoTransaction(t, f)
		})
	}
}

func TestROCS8ContentCASRecheckLeavesForeignEditAndJournal(t *testing.T) {
	f := rocS8NewFixture(t, false)
	before := rocS8Snapshot(t, f)
	rocS8Hook(t, func(step operationCandidateAcceptStep, point, _ string) error {
		if step == candidateStepApply && point == "before" {
			writeRepoFile(t, f.s, "a.txt", []byte("intervening edit\n"))
		}
		return nil
	})
	rocS8Code(t, AcceptOperationCandidate(f.s, f.slug), "candidate-cas-failed")
	if got := rocS8ReadImage(t, filepath.Join(f.s.Root, "a.txt")).data; got != "intervening edit\n" {
		t.Fatal("step-10 refusal rolled back a foreign edit without a write")
	}
	if err := ResumeOperationCandidateRecovery(f.s, f.slug); err != nil {
		t.Fatal(err)
	}
	rocS8AssertImages(t, before)
	rocS8NoTransaction(t, f)
}

func TestROCS8RollbackRestoresEveryRegisteredImage(t *testing.T) {
	for _, creation := range []bool{false, true} {
		for _, failure := range []struct {
			step              operationCandidateAcceptStep
			point, name, code string
		}{
			{candidateStepApply, "written", "a.txt", "acceptance-rolled-back"},
			{candidateStepPublish, "published", "artifacts/post-apply.patch", "coverage-publication-failed"},
			{candidateStepPublish, "published", "artifacts/recipe-capture-event.json", "coverage-publication-failed"},
			{candidateStepPublish, "publish", "artifacts/recipe-coverage.json", "coverage-publication-failed"},
			{candidateStepAccepted, "accepted-state", "state.json", "acceptance-rolled-back"},
			{candidateStepAccepted, "accepted-status", "status.json", "acceptance-rolled-back"},
			{candidateStepCommit, "before", "", "acceptance-rolled-back"},
		} {
			t.Run(fmt.Sprintf("create-%v-step-%d-%s", creation, failure.step, failure.point+failure.name), func(t *testing.T) {
				f := rocS8NewFixture(t, creation)
				f.write(t, "recipe-provenance.json", []byte{0xff, 0, 'o', 'l', 'd'})
				if runtime.GOOS != "windows" {
					if err := os.Chmod(f.featurePath("artifacts/recipe-provenance.json"), 0o640); err != nil {
						t.Fatal(err)
					}
				}
				before := rocS8Snapshot(t, f)
				rocS8Hook(t, func(step operationCandidateAcceptStep, point, name string) error {
					if step == failure.step && point == failure.point && name == failure.name {
						return errors.New("forced transaction failure")
					}
					return nil
				})
				err := AcceptOperationCandidate(f.s, f.slug)
				rocS8Code(t, err, failure.code)
				if !strings.Contains(err.Error(), fmt.Sprintf("step %d", failure.step)) {
					t.Fatalf("failure omitted step: %v", err)
				}
				rocS8AssertImages(t, before)
				rocS8NoTransaction(t, f)
			})
		}
	}
}

func TestROCS8PreparationFailureIsNotReportedAsRollback(t *testing.T) {
	f := rocS8NewFixture(t, false)
	before := rocS8Snapshot(t, f)
	rocS8Hook(t, func(step operationCandidateAcceptStep, point, name string) error {
		if step == candidateStepStage && point == "stage" && name == "artifacts/recipe-capture-event.json" {
			return errors.New("forced staging write failure")
		}
		return nil
	})
	err := AcceptOperationCandidate(f.s, f.slug)
	if err == nil || !strings.Contains(err.Error(), "before journal preparation") || strings.Contains(err.Error(), "rolled-back") {
		t.Fatalf("dishonest preparation outcome: %v", err)
	}
	rocS8AssertImages(t, before)
	rocS8NoTransaction(t, f)
}

func TestROCS8FailedRollbackRequiresExplicitRecovery(t *testing.T) {
	f := rocS8NewFixture(t, false)
	before := rocS8Snapshot(t, f)
	rocS8Hook(t, func(step operationCandidateAcceptStep, point, _ string) error {
		if (step == candidateStepAccepted && point == "accepted-status") || point == "restore" {
			return errors.New("forced failure")
		}
		return nil
	})
	rocS8Code(t, AcceptOperationCandidate(f.s, f.slug), "acceptance-rollback-failed")
	rocS8AssertRecoveryRefusal(t, f)
	operationCandidateAcceptanceHook = func(operationCandidateAcceptStep, string, string) error { return nil }
	if err := ResumeOperationCandidateRecovery(f.s, f.slug); err != nil {
		t.Fatal(err)
	}
	rocS8AssertImages(t, before)
	rocS8NoTransaction(t, f)
}

func rocS8AssertRecoveryRefusal(t *testing.T, f *rocS8Fixture) {
	t.Helper()
	for _, action := range []func() error{
		func() error { return AcceptOperationCandidate(f.s, f.slug) },
		func() error { _, err := DiffOperationCandidate(f.s, f.slug); return err },
		func() error { _, err := RejectOperationCandidate(f.s, f.slug); return err },
		func() error { return ClearOperationCandidateRejections(f.s, f.slug) },
		func() error { return RefuseResolverActionWhenCandidatePending(f.s, f.slug) },
	} {
		rocS8Code(t, action(), "recovery-required")
	}
	evidencePath := f.s.ReconcileEvidencePath(f.slug)
	before := rocS8ReadImage(t, evidencePath)
	for _, opts := range []ReconcileOptions{{}, {CumulativeLegacy: true}, {Resolve: true}} {
		_, err := RunReconcile(context.Background(), f.s, []string{f.slug}, "HEAD", nil, provider.Config{}, opts)
		if !errors.Is(err, ErrOperationCandidateRecoveryRequired) {
			t.Fatalf("ordinary reconcile did not refuse before mutation: %v", err)
		}
	}
	if rocS8ReadImage(t, evidencePath) != before {
		t.Fatal("refusal appended reconcile evidence")
	}
}

func TestROCS8SecondLockFailureReleasesFeatureHold(t *testing.T) {
	f := rocS8NewFixture(t, false)
	lock, err := candidatelock.TryAcquire(filepath.Join(f.s.Root, ".tpatch", "locks", "worktree-mutation.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	rocS8Code(t, AcceptOperationCandidate(f.s, f.slug), "candidate-lock-held")
	probe, err := candidatelock.TryAcquire(filepath.Join(f.s.Root, ".tpatch", "locks", "reconcile", f.slug+".lock"))
	if err != nil {
		t.Fatalf("feature hold leaked after failed second acquisition: %v", err)
	}
	if err := probe.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestROCS8CanonicalEventReformatDoesNotChangeAcceptanceIdentity(t *testing.T) {
	f := rocS8NewFixture(t, false)
	file := f.featurePath("artifacts/recipe-capture-event.json")
	raw := rocS8ReadImage(t, file).data
	var indented bytes.Buffer
	if err := json.Indent(&indented, []byte(raw), "", "    "); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, append(indented.Bytes(), '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AcceptOperationCandidate(f.s, f.slug); err != nil {
		t.Fatalf("format-only E change affected acceptance identity: %v", err)
	}
}

func TestROCS8ResidualPublicationExcludesAbsorbedEffects(t *testing.T) {
	f := rocS6PhaseFixture(t, rocS6UpstreamDeletion)
	patch := string(f.patch) + strings.ReplaceAll(rgaS3AddPatch, "a.txt", "absorbed.txt")
	obs := rgaS3Observe(t, patch, rgaS3Image{pre: rocS6Base, post: rocS6Feature}, rgaS3Image{post: "new\n"})
	obs.RepoRoot, obs.Slug, obs.Reference.Commit = f.s.Root, f.slug, f.coverage.Reference.Commit
	input := rgaS3Inputs(t, obs, rgaS3Write("a.txt", rocS6Base, rocS6Feature, false), rgaS3Write("absorbed.txt", "", "new\n", true))
	input.Events.RecipeRegenerated = true
	event, raw, bindings := rgaS5EventPair(t, input)
	var err error
	f.coverage, err = DecodeRecipeCoverage(raw)
	if err != nil {
		t.Fatal(err)
	}
	f.event, f.patch, f.recipe = event, bindings.Patch.Bytes, bindings.Recipe.Bytes
	f.save(t)
	writeRepoFile(t, f.s, "absorbed.txt", []byte("new\n"))
	git(t, f.s.Root, "add", "absorbed.txt")
	git(t, f.s.Root, "commit", "-qm", "absorb independent creation")
	f.invocation.upstreamCommit = strings.TrimSpace(gitOutput(t, f.s.Root, "rev-parse", "HEAD"))
	pending := rocS8Pending(t, f)
	if pending.artifact.Derivation.AbsorbedPresentCount != 1 {
		t.Fatalf("fixture did not absorb its second effect: %+v", pending.artifact.Derivation)
	}
	if err := AcceptOperationCandidate(f.s, f.slug); err != nil {
		t.Fatal(err)
	}
	residual := rocS8ReadImage(t, pending.featurePath("artifacts/post-apply.patch")).data
	if strings.Contains(residual, "absorbed.txt") || !strings.Contains(residual, "+feature") {
		t.Fatalf("residual patch did not exclude absorbed effect: %s", residual)
	}
	coverage, err := DecodeRecipeCoverage([]byte(rocS8ReadImage(t, pending.featurePath("artifacts/recipe-coverage.json")).data))
	if err != nil || len(coverage.Effects) != 1 || coverage.CoverageStatus != CoverageComplete || len(coverage.Reasons) != 0 {
		t.Fatalf("residual coverage: %+v %v", coverage, err)
	}
	invocation := f.invocation
	git(t, f.s.Root, "commit", "--allow-empty", "-qm", "later upstream")
	invocation.upstreamCommit = strings.TrimSpace(gitOutput(t, f.s.Root, "rev-parse", "HEAD"))
	eligibility, err := assessOperationCandidateEligibility(f.s, f.slug, invocation)
	if err != nil || !eligibility.eligible {
		t.Fatalf("accepted residual became ineligible: %+v %v", eligibility, err)
	}
	results, err := RunReconcile(context.Background(), f.s, []string{f.slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if err != nil || len(results) != 1 {
		t.Fatalf("next reconcile: %+v %v", results, err)
	}
	for _, entry := range results[0].Evidence {
		if entry.BaseCommit != pending.artifact.UpstreamCommit {
			t.Errorf("next reconcile stamped stale base: %+v", entry)
		}
	}
}

func TestROCS8AllSixShippedBaseReadersUseAcceptedBase(t *testing.T) {
	f := rocS8NewFixture(t, false)
	if err := AcceptOperationCandidate(f.s, f.slug); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, f.s, "a.txt", []byte("head\nupstream replacement\nend\n"))
	git(t, f.s.Root, "add", "a.txt")
	git(t, f.s.Root, "commit", "-qm", "next upstream overlap")
	result := ReconcileResult{Slug: f.slug, UpstreamRef: "HEAD",
		UpstreamCommit: strings.TrimSpace(gitOutput(t, f.s.Root, "rev-parse", "HEAD")),
		Outcome:        store.ReconcileBlocked, Phase: "phase-4-forward-apply-conflicts", FailedFiles: []string{"a.txt"}}
	assert := func(name string, entries []store.ReconcileEvidence) {
		t.Helper()
		if len(entries) == 0 {
			t.Fatalf("%s fixture did not exercise the shipped reader", name)
		}
		for _, entry := range entries {
			if entry.BaseCommit != f.artifact.UpstreamCommit {
				t.Fatalf("%s retained pre-accept base: %+v", name, entry)
			}
		}
	}
	entries, _, err := persistReconcileEvidence(f.s, f.slug, &result, false)
	if err != nil {
		t.Fatal(err)
	}
	assert("persistReconcileEvidence", entries)
	assert("file-novelty", persistFileNoveltyEvidence(f.s, f.slug, &result))
	assert("hunk-overlap", persistHunkOverlapEvidence(f.s, f.slug, &result))
	assert("blocked-classification", persistBlockedClassificationEvidence(f.s, f.slug, &result))
	git(t, f.s.Root, "rm", "a.txt")
	git(t, f.s.Root, "commit", "-qm", "next upstream deletion")
	result.UpstreamCommit = strings.TrimSpace(gitOutput(t, f.s.Root, "rev-parse", "HEAD"))
	assert("path-restructure", persistPathRestructureEvidence(f.s, f.slug, &result))
	result.Outcome = store.ReconcileUpstreamed
	assert("confirmation-gate", applyUpstreamedConfirmationGate(f.s, f.slug, &result, nil))
}

func TestROCS8BaseMismatchAfterStateWriteRollsBack(t *testing.T) {
	f := rocS8NewFixture(t, false)
	before := rocS8Snapshot(t, f)
	rocS8Hook(t, func(step operationCandidateAcceptStep, point, _ string) error {
		if step == candidateStepAccepted && point == "accepted-status" {
			raw := rocS8ReadImage(t, f.featurePath("status.json")).data
			var status store.FeatureStatus
			if err := json.Unmarshal([]byte(raw), &status); err != nil {
				return err
			}
			status.Apply.BaseCommit = f.artifact.ReferenceCommit
			body, err := json.Marshal(status)
			if err != nil {
				return err
			}
			return os.WriteFile(f.featurePath("status.json"), body, 0o644)
		}
		return nil
	})
	rocS8Code(t, AcceptOperationCandidate(f.s, f.slug), "acceptance-rolled-back")
	rocS8AssertImages(t, before)
	rocS8NoTransaction(t, f)
}

func TestROCS8RollbackRemovesCreatedParentDirectories(t *testing.T) {
	f := rocS2NewFixture(t, true)
	patch := strings.ReplaceAll(string(f.patch), "a.txt", "new/nested/a.txt")
	obs := rgaS3Observe(t, patch, rgaS3Image{post: "new\n"})
	obs.RepoRoot, obs.Slug, obs.Reference.Commit = f.s.Root, f.slug, f.coverage.Reference.Commit
	input := rgaS3Inputs(t, obs, rgaS3Write("new/nested/a.txt", "", "new\n", true))
	input.Events.RecipeRegenerated = true
	event, raw, bindings := rgaS5EventPair(t, input)
	var err error
	f.coverage, err = DecodeRecipeCoverage(raw)
	if err != nil {
		t.Fatal(err)
	}
	f.event, f.patch, f.recipe = event, bindings.Patch.Bytes, bindings.Recipe.Bytes
	f.save(t)
	pending := rocS8Pending(t, f)
	before := rocS8Snapshot(t, pending)
	rocS8Hook(t, func(step operationCandidateAcceptStep, point, _ string) error {
		if step == candidateStepApply && point == "written" {
			return errors.New("created path rollback")
		}
		return nil
	})
	rocS8Code(t, AcceptOperationCandidate(f.s, f.slug), "acceptance-rolled-back")
	rocS8AssertImages(t, before)
	if _, err := os.Stat(filepath.Join(f.s.Root, "new")); !os.IsNotExist(err) {
		t.Fatalf("new directory ancestor survived rollback: %v", err)
	}
}

func TestROCS8SameByteGenerationAppendsWithoutRewritingHistory(t *testing.T) {
	f := rocS8NewFixture(t, false)
	var staged []byte
	rocS8Hook(t, func(step operationCandidateAcceptStep, point, _ string) error {
		if step == candidateStepSnapshot && point == "before" {
			staged = []byte(rocS8ReadImage(t, f.candidatePath(filepath.Join("staged", "patch-generations.json"))).data)
			return errors.New("retain generation plan for same-byte historical-base fixture")
		}
		return nil
	})
	if err := AcceptOperationCandidate(f.s, f.slug); err == nil || len(staged) == 0 {
		t.Fatalf("generation plan capture failed: %v", err)
	}
	var manifest store.PatchGenerationsManifest
	if err := json.Unmarshal(staged, &manifest); err != nil {
		t.Fatal(err)
	}
	latest := &manifest.Generations[len(manifest.Generations)-1]
	latest.BaseCommit = f.artifact.ReferenceCommit
	latest.GenerationID = store.ComputeGenerationID(manifest.Feature, latest.Generation, latest.PatchSHA256,
		latest.RecipeSHA256, latest.BaseCommit, latest.Upper, latest.Capture)
	if err := store.SavePatchGenerations(f.s, manifest); err != nil {
		t.Fatal(err)
	}
	operationCandidateAcceptanceHook = func(operationCandidateAcceptStep, string, string) error { return nil }
	before := rocS8GenerationRows(t, []byte(rocS8ReadImage(t, f.featurePath("artifacts/patch-generations.json")).data))
	err := AcceptOperationCandidate(f.s, f.slug)
	if err != nil {
		t.Fatal(err)
	}
	after := rocS8GenerationRows(t, []byte(rocS8ReadImage(t, f.featurePath("artifacts/patch-generations.json")).data))
	if len(after) != len(before)+1 {
		t.Fatalf("metadata-aware candidate append: rows before=%d after=%d", len(before), len(after))
	}
	for i := range before {
		if !bytes.Equal(before[i], after[i]) {
			t.Fatalf("candidate append rewrote historical generation %d", i+1)
		}
	}
	current, err := store.LoadPatchGenerations(f.s, f.slug)
	if err != nil {
		t.Fatal(err)
	}
	last, _ := store.LatestPatchGeneration(current)
	status, err := f.s.LoadFeatureStatus(f.slug)
	if err != nil || last.BaseCommit != f.artifact.UpstreamCommit || status.Apply.BaseCommit != last.BaseCommit {
		t.Fatalf("metadata-aware generation disagrees with accepted base: %+v %+v %v", last, status, err)
	}
	rocS8NoTransaction(t, f)
}

func rocS8GenerationRows(t *testing.T, data []byte) []json.RawMessage {
	t.Helper()
	var manifest struct {
		Generations []json.RawMessage `json:"generations"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest.Generations
}

func TestROCS8GenerationBindingTupleIsCandidateOnly(t *testing.T) {
	f := rocS6PhaseFixture(t, rocS6UpstreamDeletion)
	input := PatchGenerationInput{
		Kind: "reconcile", Patch: string(f.patch), BaseCommit: f.invocation.upstreamCommit,
		Upper:          store.GenerationUpper{Kind: "reconcile-result", Ref: f.invocation.upstreamCommit, Commit: f.invocation.upstreamCommit},
		Capture:        store.GenerationCapture{Mode: "reconcile", Pathspecs: []string{"a.txt", "unused.txt"}, ClaimIDs: []string{}},
		candidateStage: &operationCandidateGenerationStage{recipe: bytes.Clone(f.recipe)},
	}
	if changed, err := AppendPatchGenerationForFeature(f.s, f.slug, input); err != nil || !changed {
		t.Fatalf("seed generation staging: changed=%v err=%v", changed, err)
	}
	seed := bytes.Clone(input.candidateStage.manifest)
	for _, invalid := range []struct{ kind, intent string }{
		{kind: "record"},
		{kind: "reconcile", intent: store.PatchGenerationIntentPlainRecord},
	} {
		t.Run("refuse-private-stage-"+invalid.kind+"-"+invalid.intent, func(t *testing.T) {
			ordinary := input
			ordinary.Kind, ordinary.Intent = invalid.kind, invalid.intent
			ordinary.candidateStage = &operationCandidateGenerationStage{
				recipe: bytes.Clone(f.recipe), priorPresent: true, priorManifest: bytes.Clone(seed),
			}
			before := rocS8ReadImage(t, f.s.PatchGenerationsPath(f.slug))
			if _, err := AppendPatchGenerationForFeature(f.s, f.slug, ordinary); err == nil {
				t.Fatal("candidate staging authority leaked into another producer or intent")
			}
			if len(ordinary.candidateStage.manifest) != 0 || rocS8ReadImage(t, f.s.PatchGenerationsPath(f.slug)) != before {
				t.Fatal("invalid private stage published a generation")
			}
		})
	}
	for _, test := range []struct {
		name     string
		change   func(*store.PatchGeneration)
		append   bool
		compact  bool
		claimIDs []string
	}{
		{name: "identical"},
		{name: "base", append: true, change: func(g *store.PatchGeneration) { g.BaseCommit = f.coverage.Reference.Commit }},
		{name: "recipe", append: true, change: func(g *store.PatchGeneration) { g.RecipeSHA256 = strings.Repeat("f", 64) }},
		{name: "upper-kind", append: true, change: func(g *store.PatchGeneration) { g.Upper.Kind = "commit" }},
		{name: "upper-ref", append: true, change: func(g *store.PatchGeneration) { g.Upper.Ref = "HEAD" }},
		{name: "upper-commit", append: true, change: func(g *store.PatchGeneration) { g.Upper.Commit = f.coverage.Reference.Commit }},
		{name: "capture-mode", append: true, change: func(g *store.PatchGeneration) { g.Capture.Mode = "working-tree-all" }},
		{name: "pathspecs", append: true, change: func(g *store.PatchGeneration) { g.Capture.Pathspecs = []string{"a.txt"} }},
		{name: "claim-ids", append: true, change: func(g *store.PatchGeneration) { g.Capture.ClaimIDs = []string{"historical-claim"} }},
		{name: "selector-order", change: func(g *store.PatchGeneration) { slices.Reverse(g.Capture.Pathspecs) }},
		{name: "claim-order", claimIDs: []string{"claim-a", "claim-b"}, change: func(g *store.PatchGeneration) { g.Capture.ClaimIDs = []string{"claim-b", "claim-a"} }},
		{name: "nil-empty-selectors", change: func(g *store.PatchGeneration) { g.Capture.ClaimIDs = nil }},
		{name: "noncanonical-history-bytes", append: true, compact: true, change: func(g *store.PatchGeneration) { g.BaseCommit = f.coverage.Reference.Commit }},
	} {
		t.Run(test.name, func(t *testing.T) {
			var history store.PatchGenerationsManifest
			if err := json.Unmarshal(seed, &history); err != nil {
				t.Fatal(err)
			}
			older := history.Generations[0]
			older.BaseCommit = f.coverage.Reference.Commit
			older.GenerationID = store.ComputeGenerationID(f.slug, older.Generation, older.PatchSHA256, older.RecipeSHA256, older.BaseCommit, older.Upper, older.Capture)
			history.Generations[0] = older
			latest := history.Generations[0]
			latest.Generation, latest.GenerationID, latest.BaseCommit = 0, "", input.BaseCommit
			if changed, err := store.AppendPatchGeneration(&history, latest); err != nil || !changed {
				t.Fatalf("two-entry history fixture: changed=%v err=%v", changed, err)
			}
			last := &history.Generations[len(history.Generations)-1]
			if test.change != nil {
				test.change(last)
			}
			last.GenerationID = store.ComputeGenerationID(f.slug, last.Generation, last.PatchSHA256, last.RecipeSHA256, last.BaseCommit, last.Upper, last.Capture)
			if err := store.SavePatchGenerations(f.s, history); err != nil {
				t.Fatal(err)
			}
			prior := []byte(rocS8ReadImage(t, f.s.PatchGenerationsPath(f.slug)).data)
			if test.compact {
				var compact bytes.Buffer
				if err := json.Compact(&compact, prior); err != nil {
					t.Fatal(err)
				}
				prior = append(compact.Bytes(), '\n')
				if err := os.WriteFile(f.s.PatchGenerationsPath(f.slug), prior, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			before := rocS8ReadImage(t, f.s.PatchGenerationsPath(f.slug))
			staged := input
			if test.claimIDs != nil {
				staged.Capture.ClaimIDs = slices.Clone(test.claimIDs)
			}
			staged.candidateStage = &operationCandidateGenerationStage{
				recipe: bytes.Clone(f.recipe), priorPresent: true, priorManifest: bytes.Clone(prior),
			}
			changed, err := AppendPatchGenerationForFeature(f.s, f.slug, staged)
			if err != nil || changed != test.append {
				t.Fatalf("candidate tuple mutation: append=%v want=%v err=%v", changed, test.append, err)
			}
			if rocS8ReadImage(t, f.s.PatchGenerationsPath(f.slug)) != before {
				t.Fatal("candidate staging wrote the canonical manifest before publication")
			}
			if !changed {
				if !bytes.Equal(staged.candidateStage.manifest, prior) {
					t.Fatal("equal normalized candidate tuple rewrote the manifest")
				}
			} else {
				oldRows, rows := rocS8GenerationRows(t, prior), rocS8GenerationRows(t, staged.candidateStage.manifest)
				if len(rows) != len(oldRows)+1 {
					t.Fatalf("metadata-aware append changed the history length: %d -> %d", len(oldRows), len(rows))
				}
				for i := range oldRows {
					if !bytes.Equal(rows[i], oldRows[i]) {
						t.Fatalf("metadata-aware append rewrote prior row %d", i+1)
					}
				}
				var actual store.PatchGenerationsManifest
				if err := json.Unmarshal(staged.candidateStage.manifest, &actual); err != nil {
					t.Fatal(err)
				}
				next, _ := store.LatestPatchGeneration(actual)
				if next.Kind != "reconcile" || next.BaseCommit != input.BaseCommit || next.Upper != input.Upper ||
					next.RecipeSHA256 != store.SHA256HexBytes(f.recipe) || next.Capture.Mode != input.Capture.Mode ||
					!slices.Equal(next.Capture.Pathspecs, input.Capture.Pathspecs) || !slices.Equal(next.Capture.ClaimIDs, input.Capture.ClaimIDs) {
					t.Fatalf("appended generation did not carry the full candidate tuple: %+v", next)
				}
			}
			for _, kind := range []string{"reconcile", "record"} {
				ordinary := input
				ordinary.Kind, ordinary.candidateStage = kind, nil
				if changed, err := AppendPatchGenerationForFeature(f.s, f.slug, ordinary); err != nil || changed {
					t.Fatalf("candidate exception leaked into ordinary %s: changed=%v err=%v", kind, changed, err)
				}
				if rocS8ReadImage(t, f.s.PatchGenerationsPath(f.slug)) != before {
					t.Fatalf("ordinary %s same-patch capture rewrote history", kind)
				}
			}
		})
	}
}

func TestROCS8RollbackPreservesPendingStateBytesAndMode(t *testing.T) {
	f := rocS8NewFixture(t, false)
	name := f.candidatePath(candidate.CandidateStateName)
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, []byte(rocS8ReadImage(t, name).data), "", "    "); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, append(formatted.Bytes(), '\n', '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(name, 0o600); err != nil {
		t.Fatal(err)
	}
	before := rocS8Snapshot(t, f)
	rocS8Hook(t, func(step operationCandidateAcceptStep, point, _ string) error {
		if step == candidateStepAccepted && point == "accepted-state" {
			return errors.New("restore the original noncanonical pending representation")
		}
		return nil
	})
	rocS8Code(t, AcceptOperationCandidate(f.s, f.slug), "acceptance-rolled-back")
	rocS8AssertImages(t, before)
	rocS8NoTransaction(t, f)
}

func TestROCS8MultipleLivePathsAreAllOrNothing(t *testing.T) {
	for _, failAfter := range []string{"", "a.txt", "b.txt"} {
		t.Run("fail-after-"+failAfter, func(t *testing.T) {
			f := rocS6PhaseFixture(t, rocS6UpstreamDeletion)
			patch := string(f.patch) + strings.ReplaceAll(rgaS3AddPatch, "a.txt", "b.txt")
			obs := rgaS3Observe(t, patch, rgaS3Image{pre: rocS6Base, post: rocS6Feature}, rgaS3Image{post: "new\n"})
			obs.RepoRoot, obs.Slug, obs.Reference.Commit = f.s.Root, f.slug, f.coverage.Reference.Commit
			input := rgaS3Inputs(t, obs, rgaS3Write("a.txt", rocS6Base, rocS6Feature, false), rgaS3Write("b.txt", "", "new\n", true))
			input.Events.RecipeRegenerated = true
			event, raw, bindings := rgaS5EventPair(t, input)
			var err error
			f.coverage, err = DecodeRecipeCoverage(raw)
			if err != nil {
				t.Fatal(err)
			}
			f.event, f.patch, f.recipe = event, bindings.Patch.Bytes, bindings.Recipe.Bytes
			f.save(t)
			pending := rocS8Pending(t, f)
			before := rocS8Snapshot(t, pending)
			rocS8Hook(t, func(step operationCandidateAcceptStep, point, name string) error {
				if failAfter != "" && step == candidateStepApply && point == "written" && name == failAfter {
					return errors.New("stop a multi-path acceptance")
				}
				return nil
			})
			err = AcceptOperationCandidate(f.s, f.slug)
			if failAfter != "" {
				rocS8Code(t, err, "acceptance-rolled-back")
				rocS8AssertImages(t, before)
			} else {
				if err != nil {
					t.Fatal(err)
				}
				for name, want := range map[string]string{"a.txt": "head\nfeature\nend\n", "b.txt": "new\n"} {
					if got := rocS8ReadImage(t, filepath.Join(f.s.Root, name)).data; got != want {
						t.Fatalf("incomplete multi-path acceptance %s: %q", name, got)
					}
				}
			}
			rocS8NoTransaction(t, pending)
		})
	}
}
