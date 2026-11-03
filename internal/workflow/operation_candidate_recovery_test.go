package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/candidate"
	"github.com/tesseracode/tesserapatch/internal/candidatelock"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestROCS8NativeDirectoryDurability(t *testing.T) {
	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.WriteFile(filepath.Join(directory, "retained"), []byte("durable directory entry\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := operationCandidateSyncDirectory(root); err != nil {
		t.Fatalf("native %s directory durability failed: %v", runtime.GOOS, err)
	}
	if got := rocS8ReadImage(t, filepath.Join(directory, "retained")).data; got != "durable directory entry\n" {
		t.Fatal("directory sync changed file contents")
	}
}

func TestROCS8WindowsDirectoryFlushRejectsReplacedPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("exercises Windows CreateFile path reopening before FlushFileBuffers")
	}
	base := t.TempDir()
	directory := filepath.Join(base, "held")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.Rename(directory, filepath.Join(base, "original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(directory, "foreign.txt")
	if err := os.WriteFile(foreign, []byte("foreign\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := rocS8ReadImage(t, foreign)
	if err := operationCandidateSyncDirectory(root); err == nil {
		t.Fatal("Windows directory flush accepted a replacement of its held path")
	}
	if rocS8ReadImage(t, foreign) != before {
		t.Fatal("failed directory flush changed foreign contents")
	}
}

func TestROCS8CrashChild(t *testing.T) {
	root := os.Getenv("TPATCH_ROC_S8_CRASH_ROOT")
	if root == "" {
		return
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	step, err := strconv.Atoi(os.Getenv("TPATCH_ROC_S8_CRASH_STEP"))
	if err != nil {
		t.Fatal(err)
	}
	point, name := os.Getenv("TPATCH_ROC_S8_CRASH_POINT"), os.Getenv("TPATCH_ROC_S8_CRASH_NAME")
	operationCandidateAcceptanceHook = func(actual operationCandidateAcceptStep, where, path string) error {
		if int(actual) == step && where == point && path == name {
			// Terminate without running a single defer, including lock release.
			os.Exit(87)
		}
		return nil
	}
	if err := AcceptOperationCandidate(s, os.Getenv("TPATCH_ROC_S8_CRASH_SLUG")); err != nil {
		t.Fatal(err)
	}
	t.Fatal("crash point was not reached")
}

func rocS8Crash(t *testing.T, f *rocS8Fixture, step operationCandidateAcceptStep, point, name string) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-test.run=^TestROCS8CrashChild$")
	cmd.Env = append(os.Environ(),
		"TPATCH_ROC_S8_CRASH_ROOT="+f.s.Root,
		"TPATCH_ROC_S8_CRASH_SLUG="+f.slug,
		"TPATCH_ROC_S8_CRASH_STEP="+strconv.Itoa(int(step)),
		"TPATCH_ROC_S8_CRASH_POINT="+point,
		"TPATCH_ROC_S8_CRASH_NAME="+name,
	)
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 87 {
		t.Fatalf("child did not crash at step %d/%s/%s: %v\n%s", step, point, name, err, output)
	}
	for _, path := range []string{filepath.Join("reconcile", f.slug+".lock"), "worktree-mutation.lock"} {
		probe, err := candidatelock.TryAcquire(filepath.Join(f.s.Root, ".tpatch", "locks", path))
		if err != nil {
			t.Fatalf("crashed process retained %s: %v", path, err)
		}
		if err := probe.Release(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestROCS8CrashNoMarkerRestoresExactly(t *testing.T) {
	for _, point := range []struct {
		step        operationCandidateAcceptStep
		point, name string
	}{
		{candidateStepApply, "before", ""},
		{candidateStepApply, "written", "a.txt"},
		{candidateStepPublish, "published", "artifacts/recipe-capture-event.json"},
		{candidateStepAccepted, "accepted-status", "status.json"},
	} {
		t.Run(fmt.Sprintf("%d-%s", point.step, point.point), func(t *testing.T) {
			f := rocS8NewFixture(t, false)
			before := rocS8Snapshot(t, f)
			rocS8Crash(t, f, point.step, point.point, point.name)
			rocS8AssertRecoveryRefusal(t, f)
			if err := ResumeOperationCandidateRecovery(f.s, f.slug); err != nil {
				t.Fatal(err)
			}
			rocS8AssertImages(t, before)
			rocS8NoTransaction(t, f)
			if err := AcceptOperationCandidate(f.s, f.slug); err != nil {
				t.Fatalf("pending candidate was not usable after recovery: %v", err)
			}
		})
	}
}

func TestROCS8CommittedCrashCleanupNeverRollsBack(t *testing.T) {
	for _, point := range []string{"before", "worktree-relocated", "worktree-unregistered", "pruned", "snapshots-removed", "before-journal-removal"} {
		t.Run(point, func(t *testing.T) {
			f := rocS8NewFixture(t, false)
			name := map[string]string{"pruned": "worktree", "snapshots-removed": "snapshots",
				"worktree-relocated": operationCandidatePruneDirectory, "worktree-unregistered": operationCandidatePruneDirectory,
				"before-journal-removal": candidate.CandidateJournalName}[point]
			rocS8Crash(t, f, candidateStepCleanup, point, name)
			if point == "worktree-unregistered" {
				for _, name := range []string{".git", "a.txt"} {
					if err := os.Remove(f.candidatePath(filepath.Join(operationCandidatePruneDirectory, name))); err != nil {
						t.Fatal(err)
					}
				}
			}
			rocS8AssertRecoveryRefusal(t, f)
			before := map[string]rocS8Image{}
			for _, name := range []string{"a.txt", filepath.ToSlash(filepath.Join(f.directory, candidate.CandidateMetadataName)),
				filepath.ToSlash(filepath.Join(f.directory, candidate.CandidatePatchName)),
				filepath.ToSlash(filepath.Join(f.directory, candidate.CandidateStateName))} {
				full := filepath.Join(f.s.Root, filepath.FromSlash(name))
				before[full] = rocS8ReadImage(t, full)
			}
			for _, name := range []string{"post-apply.patch", "apply-recipe.json", "recipe-provenance.json",
				"recipe-capture-event.json", "recipe-coverage.json", "patch-generations.json"} {
				full := f.featurePath("artifacts/" + name)
				before[full] = rocS8ReadImage(t, full)
			}
			if err := ResumeOperationCandidateRecovery(f.s, f.slug); err != nil {
				t.Fatal(err)
			}
			rocS8AssertImages(t, before)
			if got := rocS8ReadImage(t, filepath.Join(f.s.Root, "a.txt")).data; got != "head\nfeature\nend\n" {
				t.Fatal("committed recovery rolled live changes back")
			}
			status, err := f.s.LoadFeatureStatus(f.slug)
			if err != nil || status.Reconcile.OperationCandidate != nil || status.State != store.StateApplied {
				t.Fatalf("cleanup status: %+v %v", status, err)
			}
			rocS8NoTransaction(t, f)
		})
	}
}

func TestROCS8NoMarkerCleanupGapRetainsInlineSnapshots(t *testing.T) {
	f := rocS8NewFixture(t, true)
	before := rocS8Snapshot(t, f)
	rocS8Hook(t, func(step operationCandidateAcceptStep, point, _ string) error {
		if step == candidateStepApply && (point == "written" || point == "snapshots-removed") {
			return errors.New("interrupted rollback cleanup")
		}
		return nil
	})
	rocS8Code(t, AcceptOperationCandidate(f.s, f.slug), "acceptance-rollback-failed")
	if _, err := os.Stat(f.candidatePath("snapshots")); !os.IsNotExist(err) {
		t.Fatal("fixture did not reach the snapshot/journal cleanup gap")
	}
	if _, err := os.Stat(f.candidatePath(candidate.CandidateJournalName)); err != nil {
		t.Fatal("cleanup gap lost the journal")
	}
	operationCandidateAcceptanceHook = func(operationCandidateAcceptStep, string, string) error { return nil }
	if err := ResumeOperationCandidateRecovery(f.s, f.slug); err != nil {
		t.Fatalf("inline snapshot recovery failed after snapshots/ removal: %v", err)
	}
	rocS8AssertImages(t, before)
	rocS8NoTransaction(t, f)
}

func TestROCS8RecoveryRestoresIndependentEventCoverageAbsences(t *testing.T) {
	for _, missing := range []struct{ event, coverage bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		t.Run(fmt.Sprintf("E-absent-%v-C-absent-%v", missing.event, missing.coverage), func(t *testing.T) {
			f := rocS8NewFixture(t, false)
			var before map[string]rocS8Image
			rocS8Hook(t, func(step operationCandidateAcceptStep, point, name string) error {
				if step == candidateStepSnapshot && point == "before" {
					for artifact, absent := range map[string]bool{"recipe-capture-event.json": missing.event, "recipe-coverage.json": missing.coverage} {
						if absent {
							if err := os.Remove(f.featurePath("artifacts/" + artifact)); err != nil {
								t.Fatal(err)
							}
						}
					}
					before = rocS8Snapshot(t, f)
				}
				if (step == candidateStepPublish && point == "published" && name == "artifacts/recipe-capture-event.json") || point == "restore" {
					return errors.New("forced interrupted publication and rollback")
				}
				return nil
			})
			rocS8Code(t, AcceptOperationCandidate(f.s, f.slug), "acceptance-rollback-failed")
			rocS8AssertRecoveryRefusal(t, f)
			operationCandidateAcceptanceHook = func(operationCandidateAcceptStep, string, string) error { return nil }
			if err := ResumeOperationCandidateRecovery(f.s, f.slug); err != nil {
				t.Fatalf("recovery improperly required a valid old E/C pair: %v", err)
			}
			rocS8AssertImages(t, before)
			rocS8NoTransaction(t, f)
		})
	}
}

func TestROCS8FailedResumeRetainsEvidence(t *testing.T) {
	f := rocS8NewFixture(t, false)
	before := rocS8Snapshot(t, f)
	rocS8Crash(t, f, candidateStepPublish, "published", "artifacts/recipe-capture-event.json")
	journal := rocS8ReadImage(t, f.candidatePath(candidate.CandidateJournalName))
	mixedEvent := rocS8ReadImage(t, f.featurePath("artifacts/recipe-capture-event.json"))
	rocS8Hook(t, func(_ operationCandidateAcceptStep, point, name string) error {
		if point == "restore" && strings.HasSuffix(name, "/recipe-capture-event.json") {
			return errors.New("forced resume write failure")
		}
		return nil
	})
	err := ResumeOperationCandidateRecovery(f.s, f.slug)
	if err == nil {
		t.Fatal("failed explicit recovery reported success")
	}
	if _, recognized := OperationCandidateActionCode(err); recognized {
		t.Fatalf("internal recovery failure acquired a validation/refusal exit class: %v", err)
	}
	if rocS8ReadImage(t, f.candidatePath(candidate.CandidateJournalName)) != journal ||
		rocS8ReadImage(t, f.featurePath("artifacts/recipe-capture-event.json")) != mixedEvent {
		t.Fatal("failed recovery discarded its journal or normalized the mixed pair")
	}
	operationCandidateAcceptanceHook = func(operationCandidateAcceptStep, string, string) error { return nil }
	if err := ResumeOperationCandidateRecovery(f.s, f.slug); err != nil {
		t.Fatal(err)
	}
	rocS8AssertImages(t, before)
}

func TestROCS8RecoveryRejectsUntrustedJournalTargets(t *testing.T) {
	f := rocS8NewFixture(t, false)
	before := rocS8Snapshot(t, f)
	rocS8Crash(t, f, candidateStepApply, "before", "")
	journalFile := f.candidatePath(candidate.CandidateJournalName)
	raw, err := os.ReadFile(journalFile)
	if err != nil {
		t.Fatal(err)
	}
	var reordered map[string]json.RawMessage
	if err := json.Unmarshal(raw, &reordered); err != nil {
		t.Fatal(err)
	}
	reformatted, err := json.Marshal(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeOperationCandidateJournal(reformatted); err != nil {
		t.Fatalf("strict journal decoder incorrectly requires object-key order: %v", err)
	}
	for _, mutate := range []func(*operationCandidateJournal){
		func(j *operationCandidateJournal) { j.Snapshots[0].Path = "../escape.txt" },
		func(j *operationCandidateJournal) { j.Snapshots[0].Path = ".git/config" },
		func(j *operationCandidateJournal) { j.Snapshots[1].Path = "artifacts/unregistered.json" },
		func(j *operationCandidateJournal) { j.Snapshots = j.Snapshots[:len(j.Snapshots)-1] },
		func(j *operationCandidateJournal) { j.Snapshots[0].SHA256 = strings.Repeat("0", 64) },
		func(j *operationCandidateJournal) { j.PreparedSteps = j.PreparedSteps[:13] },
		func(j *operationCandidateJournal) { j.MissingDirectories = []string{"unrelated"} },
		func(j *operationCandidateJournal) { j.CommitMarker = "maybe" },
		func(j *operationCandidateJournal) { j.PendingStateMode = uint32(os.ModeSymlink) },
		func(j *operationCandidateJournal) { j.CandidateID = "rc_111111111111" },
		func(j *operationCandidateJournal) { j.PruneOwnership.RegistrationName = "../foreign" },
		func(j *operationCandidateJournal) { j.PruneOwnership.Head = f.artifact.ReferenceCommit + "\n" },
		func(j *operationCandidateJournal) {
			j.PruneOwnership.GitDir = filepath.Join(f.s.Root, "foreign", ".git")
		},
		func(j *operationCandidateJournal) { j.PruneOwnership.CommonDir = "../foreign" },
		func(j *operationCandidateJournal) { j.PruneOwnership.Worktree.Platform = "foreign-platform" },
		func(j *operationCandidateJournal) { j.PruneOwnership.Worktree.Volume = "000000000000000A" },
		func(j *operationCandidateJournal) { j.PruneOwnership.Worktree.File = "0000000000000000" },
		func(j *operationCandidateJournal) { j.PruneOwnership.Worktree = j.PruneOwnership.Registration },
		func(j *operationCandidateJournal) { j.CommitMarker, j.PruneOwnership = "committed", nil },
	} {
		journal, err := decodeOperationCandidateJournal(raw)
		if err != nil {
			t.Fatal(err)
		}
		mutate(&journal)
		mutant, err := json.Marshal(journal)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(journalFile, mutant, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := ResumeOperationCandidateRecovery(f.s, f.slug); err == nil {
			t.Fatalf("recovery accepted forged journal: %s", mutant)
		}
		if got := rocS8ReadImage(t, journalFile).data; got != string(mutant) {
			t.Fatal("failed recovery removed or rewrote its evidence")
		}
		rocS8AssertImages(t, before)
	}
	for _, mutant := range [][]byte{
		[]byte(`{"version":1,"version":1}`),
		[]byte(strings.TrimSuffix(strings.TrimSpace(string(raw)), "}") + `,"unrecognized":true}`),
		[]byte(strings.Replace(string(raw), `"commit_marker": ""`, `"commit_marker": null`, 1)),
	} {
		if _, err := decodeOperationCandidateJournal(mutant); err == nil {
			t.Fatal("strict journal decoder accepted duplicate/unknown/null member")
		}
	}
	if err := os.WriteFile(journalFile, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ResumeOperationCandidateRecovery(f.s, f.slug); err != nil {
		t.Fatal(err)
	}
}

func rocS8ProbeImages(t *testing.T, root string) map[string]rocS8Image {
	t.Helper()
	images := map[string]rocS8Image{}
	err := filepath.WalkDir(root, func(name string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		image := rocS8Image{present: true, mode: info.Mode()}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			image.data, err = os.Readlink(name)
		case info.Mode().IsRegular():
			var data []byte
			data, err = os.ReadFile(name)
			image.data = string(data)
		}
		images[name] = image
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return images
}

func TestROCS8JournalProbeIsReadOnlyAndFailClosed(t *testing.T) {
	for _, test := range []struct {
		name, target, kind string
		refusal            bool
	}{
		{name: "namespace-absent"},
		{name: "namespace-empty", target: "namespace", kind: "directory"},
		{name: "slug-empty", target: "slug", kind: "directory"},
		{name: "candidate-without-journal", target: "candidate", kind: "directory"},
		{name: "malformed-journal", target: "journal", kind: "file", refusal: true},
		{name: "directory-journal", target: "journal", kind: "directory", refusal: true},
		{name: "symlink-journal", target: "journal", kind: "symlink", refusal: true},
		{name: "dangling-journal", target: "journal", kind: "dangling", refusal: true},
		{name: "namespace-file", target: "namespace", kind: "file", refusal: true},
		{name: "slug-file", target: "slug", kind: "file", refusal: true},
		{name: "candidate-file", target: "candidate", kind: "file", refusal: true},
		{name: "namespace-symlink", target: "namespace", kind: "symlink", refusal: true},
		{name: "slug-symlink", target: "slug", kind: "symlink", refusal: true},
		{name: "candidate-symlink", target: "candidate", kind: "symlink", refusal: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, err := store.Init(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			const slug = "probe"
			directory, err := candidate.CandidateDirectoryRelative(slug, "rc_111111111111")
			if err != nil {
				t.Fatal(err)
			}
			base := filepath.Join(s.Root, filepath.FromSlash(directory))
			targets := map[string]string{
				"namespace": filepath.Join(s.Root, ".tpatch", "reconcile-candidates"),
				"slug":      filepath.Dir(base), "candidate": base,
				"journal": filepath.Join(base, candidate.CandidateJournalName),
			}
			foreign := filepath.Join(s.Root, "foreign")
			if err := os.Mkdir(foreign, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(foreign, "retained"), []byte("foreign bytes\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if target := targets[test.target]; target != "" {
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					t.Fatal(err)
				}
				switch test.kind {
				case "directory":
					err = os.Mkdir(target, 0o755)
				case "file":
					err = os.WriteFile(target, []byte("{malformed and preserved\n"), 0o600)
				case "symlink", "dangling":
					destination := foreign
					if test.kind == "dangling" {
						destination = filepath.Join(foreign, "absent")
					}
					err = os.Symlink(destination, target)
					if err != nil && runtime.GOOS == "windows" {
						t.Skipf("symlinks unavailable: %v", err)
					}
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			locks := filepath.Join(s.Root, ".tpatch", "locks")
			if _, err := os.Lstat(locks); !os.IsNotExist(err) {
				t.Fatalf("probe fixture must start without lock files: %v", err)
			}
			before := rocS8ProbeImages(t, s.Root)
			for _, slugs := range [][]string{{slug}, nil} {
				err := RefuseReconcileCandidateRecovery(s, slugs)
				if test.refusal {
					if !errors.Is(err, ErrOperationCandidateRecoveryRequired) {
						t.Fatalf("unsafe/present evidence did not fail closed for %v: %v", slugs, err)
					}
				} else if err != nil {
					t.Fatalf("absence/no-journal probe refused %v: %v", slugs, err)
				}
				if !maps.Equal(before, rocS8ProbeImages(t, s.Root)) {
					t.Fatalf("read-only probe changed filesystem bytes, modes or entries for %v", slugs)
				}
				if _, err := os.Lstat(locks); !os.IsNotExist(err) {
					t.Fatalf("read-only probe created persistent lock files: %v", err)
				}
			}
		})
	}
}

func TestROCS8JournalProbeDoesNotReplaceHeldRecheck(t *testing.T) {
	s, err := store.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	const slug = "probe"
	if err := RefuseReconcileCandidateRecovery(s, []string{slug}); err != nil {
		t.Fatal(err)
	}
	lock, refusal, err := acquireOperationCandidateFeatureLock(s.Root, slug)
	if err != nil || refusal.Reason != "" {
		t.Fatalf("acquire held recheck capability: %+v %v", refusal, err)
	}
	defer lock.Release()
	other, refusal, err := acquireOperationCandidateFeatureLock(s.Root, "other-probe")
	if err != nil || refusal.Reason != "" {
		t.Fatalf("acquire distinct capability: %+v %v", refusal, err)
	}
	defer other.Release()
	directory, err := candidate.CandidateDirectoryRelative(slug, "rc_111111111111")
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(s.Root, filepath.FromSlash(directory))
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, candidate.CandidateJournalName), []byte("{appeared after the early probe"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := rocS8ProbeImages(t, s.Root)
	rocS8Code(t, operationCandidateJournalPreflightHeld(s, slug, lock), "recovery-required")
	for _, invalid := range []*candidatelock.Handle{nil, other} {
		err := operationCandidateJournalPreflightHeld(s, slug, invalid)
		if code, _ := OperationCandidateActionCode(err); err == nil || code == "recovery-required" {
			t.Fatalf("held recheck inspected evidence without the correct ownership capability: %v", err)
		}
	}
	if err := RefuseReconcileCandidateRecovery(s, nil); !errors.Is(err, ErrOperationCandidateRecoveryRequired) {
		t.Fatalf("held lock hid evidence from the default-slug read-only probe: %v", err)
	}
	if !maps.Equal(before, rocS8ProbeImages(t, s.Root)) {
		t.Fatal("presence probing or held recheck mutated recovery evidence/lock files")
	}
}

func TestROCS8MalformedJournalBlocksWithoutStatusOrCoverage(t *testing.T) {
	f := rocS8NewFixture(t, false)
	if err := os.WriteFile(f.candidatePath(candidate.CandidateJournalName), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(f.featurePath("artifacts/recipe-coverage.json")); err != nil {
		t.Fatal(err)
	}
	rocS8AssertRecoveryRefusal(t, f)
	if err := os.Remove(f.featurePath("status.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(f.candidatePath(candidate.CandidateStateName)); err != nil {
		t.Fatal(err)
	}
	for _, action := range []func() error{
		func() error { return AcceptOperationCandidate(f.s, f.slug) },
		func() error { _, err := DiffOperationCandidate(f.s, f.slug); return err },
		func() error { _, err := RejectOperationCandidate(f.s, f.slug); return err },
		func() error { return ClearOperationCandidateRejections(f.s, f.slug) },
	} {
		rocS8Code(t, action(), "recovery-required")
	}
	for _, slugs := range [][]string{{f.slug}, nil} {
		_, err := RunReconcile(context.Background(), f.s, slugs, "missing-ref", nil, provider.Config{}, ReconcileOptions{})
		if !errors.Is(err, ErrOperationCandidateRecoveryRequired) {
			t.Fatalf("journal lost priority over missing status/upstream: %v", err)
		}
	}
}

func TestROCS8UnjournaledStagingCrashCanBeRetried(t *testing.T) {
	f := rocS8NewFixture(t, false)
	before := rocS8Snapshot(t, f)
	rocS8Crash(t, f, candidateStepStage, "stage", "artifacts/recipe-capture-event.json")
	rocS8AssertImages(t, before)
	if _, err := os.Stat(f.candidatePath(candidate.CandidateJournalName)); !os.IsNotExist(err) {
		t.Fatal("preparation unexpectedly installed a journal")
	}
	if err := AcceptOperationCandidate(f.s, f.slug); err != nil {
		t.Fatalf("reserved unjournaled staging residue stranded the pending candidate: %v", err)
	}
	rocS8NoTransaction(t, f)
}

func TestROCS8HeldFeatureLockCannotHideRecoveryJournal(t *testing.T) {
	f := rocS8NewFixture(t, false)
	if err := os.WriteFile(f.candidatePath(candidate.CandidateJournalName), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	lock, err := candidatelock.TryAcquire(filepath.Join(f.s.Root, ".tpatch", "locks", "reconcile", f.slug+".lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	before := rocS8ReadImage(t, f.s.ReconcileEvidencePath(f.slug))
	_, err = RunReconcile(context.Background(), f.s, []string{f.slug}, "HEAD", nil, provider.Config{}, ReconcileOptions{})
	if !errors.Is(err, ErrOperationCandidateRecoveryRequired) {
		t.Fatalf("held lock converted recovery-required into a nonterminal attempt: %v", err)
	}
	if rocS8ReadImage(t, f.s.ReconcileEvidencePath(f.slug)) != before {
		t.Fatal("journal refusal appended evidence while acceptance owned the lock")
	}
}

func TestROCS8CleanupRefusesUnknownTransactionEntries(t *testing.T) {
	f := rocS8NewFixture(t, false)
	rocS8Crash(t, f, candidateStepCleanup, "before", "")
	foreign := f.candidatePath(filepath.Join("snapshots", "foreign.txt"))
	if err := os.WriteFile(foreign, []byte("operator evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	journal := rocS8ReadImage(t, f.candidatePath(candidate.CandidateJournalName))
	live := rocS8ReadImage(t, filepath.Join(f.s.Root, "a.txt"))
	if err := ResumeOperationCandidateRecovery(f.s, f.slug); err == nil {
		t.Fatal("cleanup removed an unknown transaction entry")
	}
	if rocS8ReadImage(t, foreign).data != "operator evidence\n" ||
		rocS8ReadImage(t, f.candidatePath(candidate.CandidateJournalName)) != journal ||
		rocS8ReadImage(t, filepath.Join(f.s.Root, "a.txt")) != live {
		t.Fatal("failed committed cleanup destroyed evidence or rolled back live bytes")
	}
	if err := os.Remove(foreign); err != nil {
		t.Fatal(err)
	}
	if err := ResumeOperationCandidateRecovery(f.s, f.slug); err != nil {
		t.Fatal(err)
	}
	rocS8NoTransaction(t, f)
}

func TestROCS8ReservedPruneNeverTraversesSymlink(t *testing.T) {
	f := rocS8NewFixture(t, false)
	probe := f.candidatePath("symlink-probe")
	if err := os.Symlink(f.s.Root, probe); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Remove(probe); err != nil {
		t.Fatal(err)
	}
	rocS8Crash(t, f, candidateStepCleanup, "worktree-relocated", operationCandidatePruneDirectory)
	pruned := f.candidatePath(operationCandidatePruneDirectory)
	saved := f.candidatePath("held-prune-evidence")
	if err := os.Rename(pruned, saved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(f.s.Root, pruned); err != nil {
		t.Fatal(err)
	}
	journal := rocS8ReadImage(t, f.candidatePath(candidate.CandidateJournalName))
	live := rocS8ReadImage(t, filepath.Join(f.s.Root, "a.txt"))
	if err := ResumeOperationCandidateRecovery(f.s, f.slug); err == nil {
		t.Fatal("reserved prune traversed an unrelated symlink")
	}
	if rocS8ReadImage(t, f.candidatePath(candidate.CandidateJournalName)) != journal ||
		rocS8ReadImage(t, filepath.Join(f.s.Root, "a.txt")) != live {
		t.Fatal("symlink refusal changed live data or recovery evidence")
	}
	if err := os.Remove(pruned); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(saved, pruned); err != nil {
		t.Fatal(err)
	}
	if err := ResumeOperationCandidateRecovery(f.s, f.slug); err != nil {
		t.Fatal(err)
	}
	rocS8NoTransaction(t, f)
}

func TestROCS8ReservedPruneRefusesOrdinaryDirectoryReplacement(t *testing.T) {
	for _, point := range []string{"before", "worktree-relocated"} {
		t.Run(point, func(t *testing.T) {
			f := rocS8NewFixture(t, false)
			name, leaf := operationCandidatePruneDirectory, operationCandidatePruneDirectory
			if point == "before" {
				name, leaf = "", candidate.CandidateWorktreeName
			}
			rocS8Crash(t, f, candidateStepCleanup, point, name)
			pruned := f.candidatePath(leaf)
			saved := f.candidatePath("original-prune-held-aside")
			if err := os.Rename(pruned, saved); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(pruned, 0o755); err != nil {
				t.Fatal(err)
			}
			foreign := filepath.Join(pruned, "foreign.txt")
			if err := os.WriteFile(foreign, []byte("foreign ordinary directory\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			// A fresh typed reopen would accept this copied Git indirection.
			gitfile := rocS8ReadImage(t, filepath.Join(saved, ".git"))
			if err := os.WriteFile(filepath.Join(pruned, ".git"), []byte(gitfile.data), 0o644); err != nil {
				t.Fatal(err)
			}
			before := rocS8Snapshot(t, f)
			for _, file := range []string{foreign, filepath.Join(pruned, ".git"), filepath.Join(saved, "a.txt"), f.candidatePath(candidate.CandidateJournalName)} {
				before[file] = rocS8ReadImage(t, file)
			}
			for i := 0; i < 2; i++ {
				err := ResumeOperationCandidateRecovery(f.s, f.slug)
				if err == nil || !strings.Contains(err.Error(), "original reservation") {
					t.Fatalf("ordinary-directory replacement was not refused: %v", err)
				}
				if _, recognized := OperationCandidateActionCode(err); recognized {
					t.Fatalf("failed resume must retain exit 1, not a refusal envelope: %v", err)
				}
				rocS8AssertImages(t, before)
			}
			for _, file := range []string{foreign, filepath.Join(pruned, ".git"), pruned} {
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Rename(saved, pruned); err != nil {
				t.Fatal(err)
			}
			if err := ResumeOperationCandidateRecovery(f.s, f.slug); err != nil {
				t.Fatal(err)
			}
			rocS8NoTransaction(t, f)
		})
	}
}

func TestROCS8RegistrationPruneRequiresOriginalDirectoryIdentity(t *testing.T) {
	f := rocS8NewFixture(t, false)
	rocS8Crash(t, f, candidateStepCleanup, "worktree-relocated", operationCandidatePruneDirectory)
	journalFile := f.candidatePath(candidate.CandidateJournalName)
	journal, err := decodeOperationCandidateJournal([]byte(rocS8ReadImage(t, journalFile).data))
	if err != nil || journal.PruneOwnership == nil {
		t.Fatalf("crash lost original cleanup identity: %+v %v", journal, err)
	}
	registration := filepath.Join(f.s.Root, ".git", "worktrees", journal.PruneOwnership.RegistrationName)
	saved := filepath.Join(f.s.Root, "original-registration-held-aside")
	if err := os.Rename(registration, saved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(registration, 0o755); err != nil {
		t.Fatal(err)
	}
	before := rocS8Snapshot(t, f)
	before[journalFile] = rocS8ReadImage(t, journalFile)
	for name, body := range map[string]string{"HEAD": journal.PruneOwnership.Head, "gitdir": journal.PruneOwnership.GitDir,
		"commondir": journal.PruneOwnership.CommonDir, "foreign.txt": "not the original registration\n"} {
		file := filepath.Join(registration, name)
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		before[file] = rocS8ReadImage(t, file)
	}
	err = ResumeOperationCandidateRecovery(f.s, f.slug)
	if err == nil || !strings.Contains(err.Error(), "original reservation") {
		t.Fatalf("byte-identical replacement Git registration was not refused: %v", err)
	}
	rocS8AssertImages(t, before)
	for _, name := range []string{"HEAD", "gitdir", "commondir", "foreign.txt"} {
		if err := os.Remove(filepath.Join(registration, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(registration); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(saved, registration); err != nil {
		t.Fatal(err)
	}
	if err := ResumeOperationCandidateRecovery(f.s, f.slug); err != nil {
		t.Fatal(err)
	}
	rocS8NoTransaction(t, f)
}

func TestROCS8PartiallyDeletedOriginalRegistrationCanFinish(t *testing.T) {
	f := rocS8NewFixture(t, false)
	rocS8Crash(t, f, candidateStepCleanup, "worktree-relocated", operationCandidatePruneDirectory)
	journal, err := decodeOperationCandidateJournal([]byte(rocS8ReadImage(t, f.candidatePath(candidate.CandidateJournalName)).data))
	if err != nil || journal.PruneOwnership == nil {
		t.Fatalf("crash lost original cleanup identity: %+v %v", journal, err)
	}
	before := rocS8Snapshot(t, f)
	registration := filepath.Join(f.s.Root, ".git", "worktrees", journal.PruneOwnership.RegistrationName)
	for _, name := range []string{"HEAD", "gitdir", "commondir"} {
		if err := os.Remove(filepath.Join(registration, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := ResumeOperationCandidateRecovery(f.s, f.slug); err != nil {
		t.Fatal(err)
	}
	delete(before, f.featurePath("status.json"))
	rocS8AssertImages(t, before)
	rocS8NoTransaction(t, f)
	if _, err := os.Lstat(registration); !os.IsNotExist(err) {
		t.Fatalf("owned partial registration survived cleanup: %v", err)
	}
}
