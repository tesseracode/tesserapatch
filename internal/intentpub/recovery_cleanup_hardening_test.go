package intentpub

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
)

func TestRecoveryAcceptsArbitraryKnownMixtures(t *testing.T) {
	patterns := [][]ArtifactID{
		{ArtifactSpec, ArtifactAnalysisSidecar},
		{ArtifactAnalysis, ArtifactStatus},
		{ArtifactExploration, ArtifactAnalysisSidecar, ArtifactStatus},
	}
	for _, pattern := range patterns {
		t.Run(artifactSignatureFromIDs(pattern), func(t *testing.T) {
			_, authority := acquireWorkspace(t)
			plan := stageCreatePlan(t, authority)
			_, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
				RandomHex12: sequenceHex(),
				Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
					if point == PointAfterJournalDurable {
						return errors.New("crash")
					}
					return nil
				},
			})
			assertCode(t, err, CodeCrashInjected)
			entries := plan.Entries()
			for _, id := range pattern {
				entry, _ := findEntry(entries, id)
				rootWrite(t, authority, entry.Rel, rootRead(t, authority, entry.StagedRel), fs.FileMode(entry.NewImage.Mode))
			}

			recovered, err := Recover(authority, testSlug, Options{RandomHex12: sequenceHex()})
			if err != nil {
				t.Fatal(err)
			}
			wantRestored := reverseArtifactIDs(pattern)
			if !reflect.DeepEqual(recovered.Restored, wantRestored) {
				t.Fatalf("restored=%v, want reverse publication order %v", recovered.Restored, wantRestored)
			}
			for _, entry := range entries {
				if entry.Action == ActionCreate && rootExists(t, authority, entry.Rel) {
					t.Fatalf("%s survived recovery", entry.ArtifactID)
				}
			}
			if string(rootRead(t, authority, canonicalRel(testSlug, ArtifactStatus))) != `{"state":"requested"}` {
				t.Fatal("status preimage was not restored")
			}
		})
	}
}

func TestRecoveryRollbackUsesJournalBoundCanonicalTemp(t *testing.T) {
	_, authority := acquireWorkspace(t)
	plan := stageCreatePlan(t, authority)
	_, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
		RandomHex12: sequenceHex(),
		Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
			if point == PointAfterJournalDurable {
				return errors.New("crash")
			}
			return nil
		},
	})
	assertCode(t, err, CodeCrashInjected)
	status, _ := findEntry(plan.Entries(), ArtifactStatus)
	rootWrite(t, authority, status.Rel, rootRead(t, authority, status.StagedRel), fs.FileMode(status.NewImage.Mode))

	var renames [][2]string
	recovered, err := Recover(authority, testSlug, Options{
		RootOpsFactory: func(root *os.Root) RootOps {
			return &renamePairOps{RootOps: NewRootOps(root), pairs: &renames}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(recovered.Restored, []ArtifactID{ArtifactStatus}) {
		t.Fatalf("recovery = %#v", recovered)
	}
	want := [2]string{canonicalTempRel("0123456789abcdef", status), status.Rel}
	found := false
	for _, pair := range renames {
		if pair == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("recovery renames %v do not contain %v", renames, want)
	}
}

func TestRecoveryTreatsEveryIdenticalReplaceAsDualMatch(t *testing.T) {
	_, authority := acquireWorkspace(t)
	plan := stageSemanticNoOpRegeneratePlan(t, authority)
	hookReached := false
	_, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
		RandomHex12: sequenceHex(),
		Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
			if point == PointAfterJournalDurable {
				hookReached = true
				return errors.New("crash")
			}
			return nil
		},
	})
	assertCode(t, err, CodeCrashInjected)
	if !hookReached {
		t.Fatal("journal-durable crash hook was not reached")
	}
	recovered, err := Recover(authority, testSlug, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !recovered.Completed || len(recovered.Restored) != 0 {
		t.Fatalf("dual-match recovery = %#v", recovered)
	}
	for _, entry := range plan.Entries() {
		if current := captureForTest(t, authority, entry.Rel); !current.Equal(entry.Preimage) {
			t.Fatalf("%s changed during dual-match recovery", entry.ArtifactID)
		}
	}
}

func TestRecoveryRemovesOnlyJournalBoundCanonicalTemps(t *testing.T) {
	_, authority := acquireWorkspace(t)
	plan := stageCreatePlan(t, authority)
	first := plan.Entries()[0]
	state := &strandedCanonicalTempState{target: first.Rel}
	result, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
		RandomHex12: sequenceHex(),
		RootOpsFactory: func(root *os.Root) RootOps {
			return &strandedCanonicalTempOps{RootOps: NewRootOps(root), state: state}
		},
	})
	assertCode(t, err, CodeCleanupFailed)
	assertResultErrorExitAgreement(t, result, err, 6)
	if state.temp != canonicalTempRel("0123456789abcdef", first) ||
		!rootExists(t, authority, state.temp) {
		t.Fatalf("stranded temp = %q", state.temp)
	}

	for _, entry := range plan.Entries()[1:] {
		rootWrite(t, authority, canonicalTempRel("0123456789abcdef", entry), []byte("crash-residue"), 0o600)
	}
	foreign := pathDir(first.Rel) + "/." + path.Base(first.Rel) + ".tmp-ffffffffffff"
	rootWrite(t, authority, foreign, []byte("foreign"), 0o600)

	cleanupState := &canonicalTempCleanupState{
		expected: make(map[string]bool, len(plan.Entries())),
		synced:   make(map[string]bool),
	}
	for _, entry := range plan.Entries() {
		cleanupState.expected[canonicalTempRel("0123456789abcdef", entry)] = true
	}
	recovered, err := Recover(authority, testSlug, Options{
		RootOpsFactory: func(root *os.Root) RootOps {
			return &canonicalTempCleanupOps{RootOps: NewRootOps(root), state: cleanupState}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Outcome != OutcomeRecovered || recovered.Completed {
		t.Fatalf("recovery = %#v", recovered)
	}
	for _, entry := range plan.Entries() {
		if rootExists(t, authority, canonicalTempRel("0123456789abcdef", entry)) {
			t.Fatalf("journal-bound temp survived for %s", entry.ArtifactID)
		}
	}
	for directory, synced := range cleanupState.synced {
		if !synced {
			t.Fatalf("canonical temp parent was not synchronized: %s", directory)
		}
	}
	if string(rootRead(t, authority, foreign)) != "foreign" {
		t.Fatal("recovery removed a foreign canonical-directory temp")
	}
}

func TestRecoveryUsesExactLoadedJournalEvidence(t *testing.T) {
	t.Run("reordered-whitespace-empty-optionals-and-marker-restore", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		plan := stageCreatePlan(t, authority)
		_, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
			RandomHex12: sequenceHex(),
			Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
				if point == PointAfterJournalDurable {
					return errors.New("crash")
				}
				return nil
			},
		})
		assertCode(t, err, CodeCrashInjected)

		exact := reorderedJournalEvidence(t, rootRead(t, authority, JournalRel(testSlug)))
		rootWrite(t, authority, JournalRel(testSlug), exact, 0o600)
		state := &cleanupFaultState{fault: cleanupFinalMarkerSync, slug: testSlug}
		result, err := Recover(authority, testSlug, Options{
			RootOpsFactory: func(root *os.Root) RootOps {
				return &cleanupFaultOps{RootOps: NewRootOps(root), state: state}
			},
		})
		assertCode(t, err, CodeCleanupFailed)
		assertResultErrorExitAgreement(t, result, err, 6)
		if got := rootRead(t, authority, JournalMarkerRel(testSlug)); string(got) != string(exact) {
			t.Fatalf("marker restore changed loaded bytes:\n got %q\nwant %q", got, exact)
		}

		recovered, err := Recover(authority, testSlug, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if recovered.Outcome != OutcomeRecovered ||
			rootExists(t, authority, JournalRel(testSlug)) ||
			rootExists(t, authority, JournalMarkerRel(testSlug)) {
			t.Fatalf("exact-evidence recovery = %#v", recovered)
		}
	})

	t.Run("same-semantics-byte-change-fails-cas", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		plan := stageCreatePlan(t, authority)
		_, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
			RandomHex12: sequenceHex(),
			Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
				if point == PointAfterJournalDurable {
					return errors.New("crash")
				}
				return nil
			},
		})
		assertCode(t, err, CodeCrashInjected)
		loaded := rootRead(t, authority, JournalRel(testSlug))
		result, err := Recover(authority, testSlug, Options{
			Hook: func(point CrashPoint, root *os.Root, _ *Entry) error {
				if point != PointBeforeRecoveryClear {
					return nil
				}
				return writeRootFile(root, JournalRel(testSlug), reorderedJournalEvidence(t, loaded), 0o600)
			},
		})
		assertCode(t, err, CodeCleanupFailed)
		assertResultErrorExitAgreement(t, result, err, 6)
		if !rootExists(t, authority, JournalRel(testSlug)) {
			t.Fatal("exact-byte CAS mismatch removed journal evidence")
		}
	})
}

func TestRecoveryCP5CP6CP8Fixtures(t *testing.T) {
	tests := []struct {
		name        string
		afterRename int
		wantRestore int
	}{
		{"cp5-artifacts-new-index-old", 4, 4},
		{"cp6-index-new-status-old", 5, 5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, authority := acquireWorkspace(t)
			plan := stageRegeneratePlan(t, authority)
			count := 0
			_, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
				RandomHex12: sequenceHex(),
				Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
					if point == PointAfterEntryRename {
						count++
						if count == test.afterRename {
							return errors.New("crash")
						}
					}
					return nil
				},
			})
			assertCode(t, err, CodeCrashInjected)
			recovered, err := Recover(authority, testSlug, Options{RandomHex12: sequenceHex()})
			if err != nil {
				t.Fatal(err)
			}
			if len(recovered.Restored) != test.wantRestore || recovered.Completed {
				t.Fatalf("recovered = %#v", recovered)
			}
			assertRegeneratePreimages(t, authority)
		})
	}

	t.Run("cp8-journal-cleared", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		plan := stageCreatePlan(t, authority)
		rootWrite(t, authority, laneRel(testSlug)+"/stage-abcdefabcdef/extra", []byte("owned"), 0o600)
		hookReached := false
		earlyAssertionPassed := false
		var stateErr error
		result, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
			RandomHex12: sequenceHex(),
			Hook: func(point CrashPoint, root *os.Root, _ *Entry) error {
				switch point {
				case PointAfterJournalDurable:
					earlyAssertionPassed = inspectCP8State(root, testSlug) == nil
				case PointAfterJournalClear:
					hookReached = true
					if authority.Released() || root == nil {
						stateErr = errors.New("CP8 ran without held rooted authority")
					} else {
						stateErr = inspectCP8State(root, testSlug)
					}
					return errors.New("process died after clear")
				}
				return nil
			},
		})
		if err != nil || result.Outcome != OutcomePublished {
			t.Fatalf("result=%#v err=%v", result, err)
		}
		if !hookReached {
			t.Fatal("CP8 hook was not reached")
		}
		if earlyAssertionPassed {
			t.Fatal("CP8 state assertion passed before transaction cleanup")
		}
		if stateErr != nil {
			t.Fatalf("CP8 state: %v", stateErr)
		}
		for _, rel := range []string{
			JournalRel(testSlug),
			JournalMarkerRel(testSlug),
			plan.StageRel(),
			laneRel(testSlug) + "/index.preimage.json",
			laneRel(testSlug) + "/status.preimage.json",
		} {
			if rootExists(t, authority, rel) {
				t.Fatalf("CP8 retained transaction residue %s", rel)
			}
		}
		for _, entry := range plan.Entries() {
			if rootExists(t, authority, canonicalTempRel("0123456789abcdef", entry)) {
				t.Fatalf("CP8 retained canonical temp for %s", entry.ArtifactID)
			}
		}
		recovered, err := Recover(authority, testSlug, Options{})
		if err != nil || recovered.Outcome != OutcomeRecoveryAbsent {
			t.Fatalf("recovery=%#v err=%v", recovered, err)
		}
	})
}

func TestCP8SubprocessCrashLeavesNoRecoveryResidue(t *testing.T) {
	const childEnv = "INTENTPUB_CP8_CRASH_CHILD"
	if os.Getenv(childEnv) == "1" {
		workspace := os.Getenv("INTENTPUB_CP8_WORKSPACE")
		authority, err := intentlock.Acquire(workspace)
		if err != nil {
			t.Fatal(err)
		}
		plan := stageCreatePlan(t, authority)
		rootWrite(t, authority, laneRel(testSlug)+"/stage-abcdefabcdef/extra", []byte("owned"), 0o600)
		_, _ = Execute(authority, plan, "0123456789abcdef", nil, Options{
			RandomHex12: sequenceHex(),
			Hook: func(point CrashPoint, root *os.Root, _ *Entry) error {
				if point != PointAfterJournalClear {
					return nil
				}
				if authority.Released() || root == nil || inspectCP8State(root, testSlug) != nil {
					os.Exit(89)
				}
				os.Exit(88)
				return nil
			},
		})
		t.Fatal("child execution returned before CP8 crash")
	}

	workspace := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestCP8SubprocessCrashLeavesNoRecoveryResidue$")
	command.Env = append(os.Environ(), childEnv+"=1", "INTENTPUB_CP8_WORKSPACE="+workspace)
	output, err := command.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 88 {
		t.Fatalf("CP8 child exit=%v output=%s", err, output)
	}

	authority, err := intentlock.Acquire(workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer authority.Release()
	recovered, err := Recover(authority, testSlug, Options{})
	if err != nil || recovered.Outcome != OutcomeRecoveryAbsent {
		t.Fatalf("post-CP8 recovery=%#v err=%v", recovered, err)
	}
	err = authority.WithRoot(func(root *os.Root) error {
		return inspectCP8State(root, testSlug)
	})
	if err != nil {
		t.Fatalf("post-CP8 residue: %v", err)
	}
}

func inspectCP8State(root *os.Root, slug string) error {
	if root == nil {
		return errors.New("root is nil")
	}
	for _, rel := range []string{
		JournalRel(slug),
		JournalMarkerRel(slug),
		laneRel(slug) + "/index.preimage.json",
		laneRel(slug) + "/status.preimage.json",
	} {
		if _, err := root.Lstat(rel); err == nil {
			return fmt.Errorf("%s still exists", rel)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("inspect %s: %w", rel, err)
		}
	}
	lane, err := root.Open(laneRel(slug))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	names, readErr := lane.Readdirnames(-1)
	closeErr := lane.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	for _, name := range names {
		if ownedStageName(name) {
			return fmt.Errorf("owned stage %s still exists", name)
		}
	}
	return nil
}

func TestUndoAndRecoveryNormalizeNonregularEvidence(t *testing.T) {
	t.Run("undo-directory", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		plan := stageCreatePlan(t, authority)
		state := &renameFailureState{failCanonicalAt: 2}
		changed := false
		result, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
			RandomHex12: sequenceHex(),
			RootOpsFactory: func(root *os.Root) RootOps {
				return &failingRenameOps{RootOps: NewRootOps(root), state: state}
			},
			Hook: func(point CrashPoint, root *os.Root, entry *Entry) error {
				if point == PointAfterEntryRename && entry != nil &&
					entry.ArtifactID == ArtifactAnalysis && !changed {
					changed = true
					if err := root.Remove(entry.Rel); err != nil {
						return err
					}
					return root.Mkdir(entry.Rel, 0o755)
				}
				return nil
			},
		})
		assertCode(t, err, CodeUndoCASMismatch)
		assertResultErrorExitAgreement(t, result, err, 6)
		if !rootExists(t, authority, JournalRel(testSlug)) {
			t.Fatal("undo mismatch removed journal evidence")
		}
	})

	for _, kind := range []string{"directory", "symlink"} {
		t.Run("recovery-"+kind, func(t *testing.T) {
			_, authority := acquireWorkspace(t)
			plan := stageCreatePlan(t, authority)
			_, _ = Execute(authority, plan, "0123456789abcdef", nil, Options{
				RandomHex12: sequenceHex(),
				Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
					if point == PointAfterJournalDurable {
						return errors.New("crash")
					}
					return nil
				},
			})
			entry := plan.Entries()[0]
			err := authority.WithRoot(func(root *os.Root) error {
				switch kind {
				case "directory":
					return root.Mkdir(entry.Rel, 0o755)
				default:
					return root.Symlink("spec.md", entry.Rel)
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			result, err := Recover(authority, testSlug, Options{})
			assertCode(t, err, CodeRecoveryDivergent)
			assertResultErrorExitAgreement(t, result, err, 6)
			if !rootExists(t, authority, JournalRel(testSlug)) {
				t.Fatal("divergent recovery removed journal evidence")
			}
		})
	}
}

func TestRecoveryFinalSetDetectsEarlierRestoreRewrittenAfterLaterUndo(t *testing.T) {
	_, authority := acquireWorkspace(t)
	plan := stageCreatePlan(t, authority)
	renameCount := 0
	_, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
		RandomHex12: sequenceHex(),
		Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
			if point == PointAfterEntryRename {
				renameCount++
				if renameCount == 2 {
					return errors.New("crash after two renames")
				}
			}
			return nil
		},
	})
	assertCode(t, err, CodeCrashInjected)

	undoCount := 0
	result, err := Recover(authority, testSlug, Options{
		Hook: func(point CrashPoint, root *os.Root, entry *Entry) error {
			if point != PointAfterUndo || entry == nil {
				return nil
			}
			undoCount++
			if undoCount == 2 {
				return writeRootFile(root, canonicalRel(testSlug, ArtifactSpec), []byte("rewritten-after-recovery"), 0o644)
			}
			return nil
		},
	})
	assertCode(t, err, CodeRecoveryDivergent)
	assertResultErrorExitAgreement(t, result, err, 6)
	if !reflect.DeepEqual(result.Restored, []ArtifactID{ArtifactAnalysis}) {
		t.Fatalf("final-set divergent entry reported restored: %v", result.Restored)
	}
	if string(rootRead(t, authority, canonicalRel(testSlug, ArtifactSpec))) != "rewritten-after-recovery" {
		t.Fatal("recovery final set verification changed divergent bytes")
	}
	if !rootExists(t, authority, JournalRel(testSlug)) {
		t.Fatal("recovery final set divergence removed evidence")
	}
}

func TestRootReplacementIsRevalidatedBeforeEvidenceCleanup(t *testing.T) {
	t.Run("execution-rollback", func(t *testing.T) {
		workspace, authority := acquireWorkspace(t)
		plan := stageCreatePlan(t, authority)
		state := &renameFailureState{failCanonicalAt: 2}
		var restore func()
		result, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
			RandomHex12: sequenceHex(),
			RootOpsFactory: func(root *os.Root) RootOps {
				return &failingRenameOps{RootOps: NewRootOps(root), state: state}
			},
			Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
				if point == PointBeforeUndo && restore == nil {
					var replaceErr error
					restore, replaceErr = replaceWorkspacePath(workspace)
					return replaceErr
				}
				return nil
			},
		})
		if restore != nil {
			defer restore()
		}
		assertCode(t, err, CodeWorkspaceRootReplacedAfterPublication)
		assertResultErrorExitAgreement(t, result, err, 6)
		if !rootExists(t, authority, JournalRel(testSlug)) {
			t.Fatal("root replacement during rollback removed evidence")
		}
	})

	t.Run("recovery-before-clear", func(t *testing.T) {
		workspace, authority := acquireWorkspace(t)
		plan := stageCreatePlan(t, authority)
		_, _ = Execute(authority, plan, "0123456789abcdef", nil, Options{
			RandomHex12: sequenceHex(),
			Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
				if point == PointAfterJournalDurable {
					return errors.New("crash")
				}
				return nil
			},
		})
		var restore func()
		result, err := Recover(authority, testSlug, Options{
			Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
				if point == PointBeforeRecoveryClear && restore == nil {
					var replaceErr error
					restore, replaceErr = replaceWorkspacePath(workspace)
					return replaceErr
				}
				return nil
			},
		})
		if restore != nil {
			defer restore()
		}
		assertCode(t, err, CodeWorkspaceRootReplacedAfterPublication)
		assertResultErrorExitAgreement(t, result, err, 6)
		if !rootExists(t, authority, JournalRel(testSlug)) {
			t.Fatal("root replacement during recovery removed evidence")
		}
	})

	t.Run("recovery-during-undo", func(t *testing.T) {
		workspace, authority := acquireWorkspace(t)
		plan := stageCreatePlan(t, authority)
		_, _ = Execute(authority, plan, "0123456789abcdef", nil, Options{
			RandomHex12: sequenceHex(),
			Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
				if point == PointAfterJournalDurable {
					return errors.New("crash")
				}
				return nil
			},
		})
		entry := plan.Entries()[0]
		rootWrite(t, authority, entry.Rel, rootRead(t, authority, entry.StagedRel), fs.FileMode(entry.NewImage.Mode))
		var restore func()
		result, err := Recover(authority, testSlug, Options{
			Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
				if point == PointAfterUndo && restore == nil {
					var replaceErr error
					restore, replaceErr = replaceWorkspacePath(workspace)
					return replaceErr
				}
				return nil
			},
		})
		if restore != nil {
			defer restore()
		}
		assertCode(t, err, CodeWorkspaceRootReplacedAfterPublication)
		assertResultErrorExitAgreement(t, result, err, 6)
		if !rootExists(t, authority, JournalRel(testSlug)) {
			t.Fatal("root replacement during recovery undo removed evidence")
		}
	})
}

func TestCleanupMarkerRetainsRecoverableEvidenceOnEveryCleanupPhase(t *testing.T) {
	for _, fault := range []cleanupFault{
		cleanupMarkerSync,
		cleanupStageRemove,
		cleanupFinalMarkerSync,
	} {
		t.Run(string(fault), func(t *testing.T) {
			_, authority := acquireWorkspace(t)
			plan := stageCreatePlan(t, authority)
			state := &cleanupFaultState{fault: fault, slug: testSlug}
			result, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
				RandomHex12: sequenceHex(),
				RootOpsFactory: func(root *os.Root) RootOps {
					return &cleanupFaultOps{RootOps: NewRootOps(root), state: state}
				},
			})
			assertCode(t, err, CodeCleanupFailed)
			assertResultErrorExitAgreement(t, result, err, 6)
			if !rootExists(t, authority, JournalMarkerRel(testSlug)) {
				t.Fatalf("%s removed all durable transaction evidence", fault)
			}
			assertMode(t, authority, JournalMarkerRel(testSlug), 0o600)

			recovered, err := Recover(authority, testSlug, Options{RandomHex12: sequenceHex()})
			if err != nil {
				t.Fatal(err)
			}
			if recovered.Outcome != OutcomeRecovered || !recovered.Completed ||
				rootExists(t, authority, JournalMarkerRel(testSlug)) {
				t.Fatalf("marker recovery = %#v", recovered)
			}
		})
	}
}

func TestRecoveryCleanupRemovesEveryOwnedStageAndNothingElse(t *testing.T) {
	_, authority := acquireWorkspace(t)
	plan := stageCreatePlan(t, authority)
	_, _ = Execute(authority, plan, "0123456789abcdef", nil, Options{
		RandomHex12: sequenceHex(),
		Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
			if point == PointAfterJournalDurable {
				return errors.New("crash")
			}
			return nil
		},
	})
	extraStage := laneRel(testSlug) + "/stage-abcdefabcdef/value"
	nonmatchingStage := laneRel(testSlug) + "/stage-not-owned/value"
	abandoned := laneRel(testSlug) + "/abandoned-keep/value"
	otherSlug := laneRel("other") + "/stage-abcdefabcdef/value"
	blob := featureRel(testSlug) + "/artifacts/intent-archive/blobs/" + strings.Repeat("a", 64) + ".blob"
	for _, rel := range []string{extraStage, nonmatchingStage, abandoned, otherSlug, blob} {
		rootWrite(t, authority, rel, []byte("keep"), 0o600)
	}

	if _, err := Recover(authority, testSlug, Options{RandomHex12: sequenceHex()}); err != nil {
		t.Fatal(err)
	}
	if rootExists(t, authority, plan.StageRel()) || rootExists(t, authority, pathDir(extraStage)) {
		t.Fatal("owned stage tree survived recovery cleanup")
	}
	for _, rel := range []string{nonmatchingStage, abandoned, otherSlug, blob} {
		if string(rootRead(t, authority, rel)) != "keep" {
			t.Fatalf("cleanup touched unowned path %s", rel)
		}
	}
}

func TestRecoverUsesExactlyOneScratchBacking(t *testing.T) {
	_, authority := acquireWorkspace(t)
	plan := stageCreatePlan(t, authority)
	_, _ = Execute(authority, plan, "0123456789abcdef", nil, Options{
		RandomHex12: sequenceHex(),
		Hook: func(point CrashPoint, _ *os.Root, _ *Entry) error {
			if point == PointAfterJournalDurable {
				return errors.New("crash")
			}
			return nil
		},
	})
	allocations := 0
	backings := make(map[uintptr]struct{})
	_, err := Recover(authority, testSlug, Options{
		RandomHex12: sequenceHex(),
		ScratchFactory: func(size int) []byte {
			allocations++
			return make([]byte, size)
		},
		RootOpsFactory: func(root *os.Root) RootOps {
			return &scratchRenameSpyOps{RootOps: NewRootOps(root), backings: backings, renames: new([][2]string)}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if allocations != 1 || len(backings) != 1 {
		t.Fatalf("recovery scratch allocations=%d backings=%d", allocations, len(backings))
	}
}

func stageRegeneratePlan(t *testing.T, workspaceAuthority *intentlock.WorkspaceAuthority) Plan {
	t.Helper()
	oldIntent := regenerateOldIntent()
	for id, data := range oldIntent {
		rootWrite(t, workspaceAuthority, canonicalRel(testSlug, id), data, regenerateOldMode(id))
	}
	oldIndex := []byte(`{"old":"index"}`)
	oldStatus := []byte(`{"old":"status"}`)
	rootWrite(t, workspaceAuthority, canonicalRel(testSlug, ArtifactArchiveIndex), oldIndex, 0o644)
	rootWrite(t, workspaceAuthority, canonicalRel(testSlug, ArtifactStatus), oldStatus, 0o644)
	stage, err := Stage(workspaceAuthority, testSlug, []StageInput{
		{ArtifactID: ArtifactAnalysis, Rel: "analysis.md", Data: []byte("new-analysis"), Mode: 0o600},
		{ArtifactID: ArtifactSpec, Rel: "spec.md", Data: []byte("new-spec"), Mode: 0o644},
		{ArtifactID: ArtifactExploration, Rel: "exploration.md", Data: []byte("new-exploration"), Mode: 0o644},
		{ArtifactID: ArtifactAnalysisSidecar, Rel: "analysis.json", Data: []byte(`{"new":"analysis"}`), Mode: 0o644},
		{ArtifactID: ArtifactArchiveIndex, Rel: "index.json", Data: []byte(`{"new":"index"}`), Mode: 0o644},
		{ArtifactID: ArtifactStatus, Rel: "status.json", Data: []byte(`{"new":"status"}`), Mode: 0o644},
	}, Options{RandomHex12: sequenceHex()})
	if err != nil {
		t.Fatal(err)
	}
	byID := stagedByID(stage)
	entries := make([]Entry, 0, 6)
	for _, id := range []ArtifactID{ArtifactAnalysis, ArtifactSpec, ArtifactExploration, ArtifactAnalysisSidecar} {
		preimage := captureForTest(t, workspaceAuthority, canonicalRel(testSlug, id))
		blobRel := featureRel(testSlug) + "/artifacts/intent-archive/blobs/" + preimage.SHA256 + ".blob"
		if _, err := DurableWrite(workspaceAuthority, WriteRequest{
			Rel: blobRel, Data: oldIntent[id], Mode: 0o644, Role: WriteRoleOrdinaryCanonical,
		}, Options{RandomHex12: sequenceHex()}); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, Entry{
			ArtifactID:      id,
			Rel:             canonicalRel(testSlug, id),
			Action:          ActionReplace,
			Preimage:        preimage,
			PreimageBlob:    preimage.SHA256,
			PreimageBlobRel: blobRel,
			NewImage:        byID[id].NewImage,
			StagedRel:       byID[id].Rel,
		})
	}
	indexRaw, err := WriteRawPreimage(workspaceAuthority, testSlug, ArtifactArchiveIndex, oldIndex, Options{RandomHex12: sequenceHex()})
	if err != nil {
		t.Fatal(err)
	}
	statusRaw, err := WriteRawPreimage(workspaceAuthority, testSlug, ArtifactStatus, oldStatus, Options{RandomHex12: sequenceHex()})
	if err != nil {
		t.Fatal(err)
	}
	entries = append(entries,
		Entry{ArtifactID: ArtifactArchiveIndex, Rel: canonicalRel(testSlug, ArtifactArchiveIndex), Action: ActionReplace, Preimage: captureForTest(t, workspaceAuthority, canonicalRel(testSlug, ArtifactArchiveIndex)), PreimageRawRel: indexRaw, NewImage: byID[ArtifactArchiveIndex].NewImage, StagedRel: byID[ArtifactArchiveIndex].Rel},
		Entry{ArtifactID: ArtifactStatus, Rel: canonicalRel(testSlug, ArtifactStatus), Action: ActionReplace, Preimage: captureForTest(t, workspaceAuthority, canonicalRel(testSlug, ArtifactStatus)), PreimageRawRel: statusRaw, NewImage: byID[ArtifactStatus].NewImage, StagedRel: byID[ArtifactStatus].Rel},
	)
	plan, err := NewPlan(testSlug, ModeRegenerate, stage.StageRel, entries)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func stageSemanticNoOpRegeneratePlan(t *testing.T, authority *intentlock.WorkspaceAuthority) Plan {
	t.Helper()
	base := stageRegeneratePlan(t, authority)
	entries := base.Entries()
	for index := range entries {
		data := rootRead(t, authority, entries[index].Rel)
		rootWrite(t, authority, entries[index].StagedRel, data, 0o600)
		entries[index].NewImage = entries[index].Preimage
	}
	plan, err := NewPlan(testSlug, ModeRegenerate, base.StageRel(), entries)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func regenerateOldIntent() map[ArtifactID][]byte {
	return map[ArtifactID][]byte{
		ArtifactAnalysis:        []byte("old-analysis"),
		ArtifactSpec:            []byte("old-spec"),
		ArtifactExploration:     []byte("old-exploration"),
		ArtifactAnalysisSidecar: []byte(`{"old":"analysis"}`),
	}
}

func regenerateOldMode(id ArtifactID) fs.FileMode {
	if id == ArtifactAnalysis {
		return 0o600
	}
	return 0o644
}

func assertRegeneratePreimages(t *testing.T, authority *intentlock.WorkspaceAuthority) {
	t.Helper()
	for id, data := range regenerateOldIntent() {
		if string(rootRead(t, authority, canonicalRel(testSlug, id))) != string(data) {
			t.Fatalf("%s preimage was not restored", id)
		}
		assertMode(t, authority, canonicalRel(testSlug, id), regenerateOldMode(id))
	}
	if string(rootRead(t, authority, canonicalRel(testSlug, ArtifactArchiveIndex))) != `{"old":"index"}` ||
		string(rootRead(t, authority, canonicalRel(testSlug, ArtifactStatus))) != `{"old":"status"}` {
		t.Fatal("metadata preimages were not restored")
	}
}

func artifactSignatureFromIDs(ids []ArtifactID) string {
	parts := make([]string, len(ids))
	for index, id := range ids {
		parts[index] = string(id)
	}
	return strings.Join(parts, "-")
}

func reverseArtifactIDs(ids []ArtifactID) []ArtifactID {
	result := make([]ArtifactID, len(ids))
	for index := range ids {
		result[len(ids)-1-index] = ids[index]
	}
	return result
}

func reorderedJournalEvidence(t *testing.T, canonical []byte) []byte {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(canonical, &top); err != nil {
		t.Fatal(err)
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(top["entries"], &entries); err != nil {
		t.Fatal(err)
	}
	entries[0]["preimage_blob"] = json.RawMessage(`""`)
	entries[0]["preimage_blob_rel"] = json.RawMessage(`""`)
	entries[0]["preimage_raw_rel"] = json.RawMessage(`""`)
	entryBytes, err := json.MarshalIndent(entries, "    ", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return []byte(fmt.Sprintf(
		"{\n  \"entries\": %s,\n  \"stage_rel\": %s,\n  \"plan_digest\": %s,\n  \"run_nonce\": %s,\n  \"mode\": %s,\n  \"slug\": %s,\n  \"version\": %s\n}\n",
		entryBytes, top["stage_rel"], top["plan_digest"], top["run_nonce"],
		top["mode"], top["slug"], top["version"],
	))
}

func assertResultErrorExitAgreement(t *testing.T, result Result, err error, want int) {
	t.Helper()
	var typed *Error
	if !errors.As(err, &typed) {
		t.Fatalf("error = %#v", err)
	}
	if result.ExitClass != want || typed.ExitClass != want {
		t.Fatalf("result exit=%d error exit=%d, want %d", result.ExitClass, typed.ExitClass, want)
	}
}

func replaceWorkspacePath(workspace string) (func(), error) {
	moved := workspace + "-held"
	if err := os.Rename(workspace, moved); err != nil {
		return nil, err
	}
	if err := os.Mkdir(workspace, 0o755); err != nil {
		_ = os.Rename(moved, workspace)
		return nil, err
	}
	return func() {
		_ = os.Remove(workspace)
		_ = os.Rename(moved, workspace)
	}, nil
}

type cleanupFault string

const (
	cleanupMarkerSync      cleanupFault = "marker-sync"
	cleanupStageRemove     cleanupFault = "stage-remove"
	cleanupFinalMarkerSync cleanupFault = "final-marker-sync"
)

type cleanupFaultState struct {
	fault         cleanupFault
	slug          string
	markerRenamed bool
	markerRemoved bool
	fired         bool
}

type cleanupFaultOps struct {
	RootOps
	state *cleanupFaultState
}

func (ops *cleanupFaultOps) Open(name string) (RootFile, error) {
	file, err := ops.RootOps.Open(name)
	if err != nil {
		return nil, err
	}
	if name == laneRel(ops.state.slug) {
		return &cleanupFaultDir{RootFile: file, state: ops.state}, nil
	}
	return file, nil
}

func (ops *cleanupFaultOps) Rename(oldName, newName string) error {
	err := ops.RootOps.Rename(oldName, newName)
	if err == nil && oldName == JournalRel(ops.state.slug) && newName == JournalMarkerRel(ops.state.slug) {
		ops.state.markerRenamed = true
	}
	return err
}

func (ops *cleanupFaultOps) Remove(name string) error {
	if ops.state.fault == cleanupStageRemove && ops.state.markerRenamed &&
		strings.Contains(name, "/stage-") && !ops.state.fired {
		ops.state.fired = true
		return errors.New("stage remove fault")
	}
	err := ops.RootOps.Remove(name)
	if err == nil && name == JournalMarkerRel(ops.state.slug) {
		ops.state.markerRemoved = true
	}
	return err
}

type cleanupFaultDir struct {
	RootFile
	state *cleanupFaultState
}

type strandedCanonicalTempState struct {
	target     string
	temp       string
	tempClosed bool
}

type renamePairOps struct {
	RootOps
	pairs *[][2]string
}

func (ops *renamePairOps) Rename(oldName, newName string) error {
	*ops.pairs = append(*ops.pairs, [2]string{oldName, newName})
	return ops.RootOps.Rename(oldName, newName)
}

type strandedCanonicalTempOps struct {
	RootOps
	state *strandedCanonicalTempState
}

func (ops *strandedCanonicalTempOps) Lstat(name string) (fs.FileInfo, error) {
	if name == ops.state.target && ops.state.tempClosed {
		return nil, errors.New("crash after canonical temp close")
	}
	return ops.RootOps.Lstat(name)
}

func (ops *strandedCanonicalTempOps) OpenFile(name string, flag int, mode fs.FileMode) (RootFile, error) {
	file, err := ops.RootOps.OpenFile(name, flag, mode)
	if err != nil {
		return nil, err
	}
	if strings.Contains(name, ".tmp-") && pathDir(name) == pathDir(ops.state.target) {
		ops.state.temp = name
		return &strandedCanonicalTempFile{RootFile: file, state: ops.state}, nil
	}
	return file, nil
}

func (ops *strandedCanonicalTempOps) Remove(name string) error {
	if name == ops.state.temp && ops.state.tempClosed {
		return errors.New("process died before temp cleanup")
	}
	return ops.RootOps.Remove(name)
}

type strandedCanonicalTempFile struct {
	RootFile
	state *strandedCanonicalTempState
}

func (file *strandedCanonicalTempFile) Close() error {
	err := file.RootFile.Close()
	file.state.tempClosed = true
	return err
}

type canonicalTempCleanupState struct {
	expected map[string]bool
	synced   map[string]bool
}

type canonicalTempCleanupOps struct {
	RootOps
	state *canonicalTempCleanupState
}

func (ops *canonicalTempCleanupOps) Remove(name string) error {
	err := ops.RootOps.Remove(name)
	if err == nil && ops.state.expected[name] {
		ops.state.synced[path.Dir(name)] = false
	}
	return err
}

func (ops *canonicalTempCleanupOps) Open(name string) (RootFile, error) {
	file, err := ops.RootOps.Open(name)
	if err != nil {
		return nil, err
	}
	if _, tracked := ops.state.synced[name]; tracked {
		return &canonicalTempCleanupDir{RootFile: file, directory: name, state: ops.state}, nil
	}
	return file, nil
}

type canonicalTempCleanupDir struct {
	RootFile
	directory string
	state     *canonicalTempCleanupState
}

func (file *canonicalTempCleanupDir) Sync() error {
	if err := file.RootFile.Sync(); err != nil {
		return err
	}
	file.state.synced[file.directory] = true
	return nil
}

func (file *cleanupFaultDir) Sync() error {
	if file.state.fault == cleanupMarkerSync && file.state.markerRenamed && !file.state.fired {
		file.state.fired = true
		return errors.New("marker sync fault")
	}
	if file.state.fault == cleanupFinalMarkerSync && file.state.markerRemoved && !file.state.fired {
		file.state.fired = true
		return errors.New("final marker sync fault")
	}
	return file.RootFile.Sync()
}
