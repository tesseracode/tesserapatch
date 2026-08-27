//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

type s7ATOrphanPredicateObservation struct {
	name              string
	code              int
	stderr            string
	report            intentArchivePurgeReport
	expectedRemoved   []string
	removed           []string
	blobPresent       map[string]bool
	indexBefore       []byte
	indexAfter        []byte
	dangling          bool
	mixedHash         string
	mixedBlobBefore   []byte
	mixedBlobAfter    []byte
	mixedStatesBefore []store.IntentArchiveWireState
	mixedStatesAfter  []store.IntentArchiveWireState
}

func s7ATObserveOrphanPredicate(
	t *testing.T,
	withMixed bool,
) s7ATOrphanPredicateObservation {
	t.Helper()
	root, slug := intentArchiveCLIWorkspace(t)
	residue := s7ASWriteResidueFixture(
		t, root, slug, []byte("PIB-533 indexed residue\n"),
	)
	unindexedData := []byte("PIB-533 unindexed orphan\n")
	unindexed := intentArchiveCLIReplacement(
		t,
		store.IntentArchiveArtifactSpec,
		unindexedData,
		store.IntentArchiveWireRetained,
	)
	unindexedRel, err := store.IntentArchiveBlobRel(slug, unindexed.ContentSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(root, filepath.FromSlash(unindexedRel)),
		unindexedData,
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	mixedHash := ""
	mixedRel := ""
	var mixedBlobBefore []byte
	if withMixed {
		mixedData := []byte("PIB-533 mixed reference\n")
		retained := intentArchiveCLIReplacement(
			t,
			store.IntentArchiveArtifactExploration,
			mixedData,
			store.IntentArchiveWireRetained,
		)
		tombstoned := intentArchiveCLIReplacement(
			t,
			store.IntentArchiveArtifactSpec,
			mixedData,
			store.IntentArchiveWireTombstoned,
		)
		_, index := readIntentArchiveCLIIndex(t, root, slug)
		index.Generations = append(
			index.Generations,
			intentArchiveCLIGeneration(t, slug, retained),
			intentArchiveCLIGeneration(t, slug, tombstoned),
		)
		encoded, err := store.EncodeIntentArchiveIndex(index)
		if err != nil {
			t.Fatal(err)
		}
		indexRel, err := store.IntentArchiveIndexRel(slug)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(root, filepath.FromSlash(indexRel)),
			encoded,
			0o644,
		); err != nil {
			t.Fatal(err)
		}
		mixedHash = retained.ContentSHA256
		mixedRel, err = store.IntentArchiveBlobRel(slug, mixedHash)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(root, filepath.FromSlash(mixedRel)),
			mixedData,
			0o644,
		); err != nil {
			t.Fatal(err)
		}
		mixedBlobBefore = append([]byte(nil), mixedData...)
	}

	indexBefore, parsedBefore := readIntentArchiveCLIIndex(t, root, slug)
	code, stdout, stderr, _ := runPrepare(
		t,
		"--path", root, "feature", "intent-archive", "purge", slug,
		"--orphans", "--yes", "--json", "--quiet",
	)
	report := decodeIntentArchivePurgeReport(t, stdout)
	indexAfter, parsedAfter := readIntentArchiveCLIIndex(t, root, slug)

	expectedRemoved := []string{residue.hash, unindexed.ContentSHA256}
	sort.Strings(expectedRemoved)
	removed := []string{}
	for _, blob := range report.Blobs {
		if blob.Removed {
			removed = append(removed, blob.Hash)
		}
	}
	sort.Strings(removed)
	blobPresent := map[string]bool{}
	for hash, rel := range map[string]string{
		residue.hash:            residue.blobRel,
		unindexed.ContentSHA256: unindexedRel,
	} {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		blobPresent[hash] = err == nil
	}
	if mixedHash != "" {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(mixedRel)))
		blobPresent[mixedHash] = err == nil
	}
	dangling := false
	for _, generation := range parsedAfter.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.WireState() != store.IntentArchiveWireRetained {
				continue
			}
			blobRel, err := store.IntentArchiveBlobRel(
				slug, replacement.ContentSHA256,
			)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(
				filepath.Join(root, filepath.FromSlash(blobRel)),
			); os.IsNotExist(err) {
				dangling = true
			}
		}
	}
	var mixedBlobAfter []byte
	if mixedHash != "" {
		mixedBlobAfter, err = os.ReadFile(
			filepath.Join(root, filepath.FromSlash(mixedRel)),
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	name := "residue-only"
	if withMixed {
		name = "residue-plus-mixed"
	}
	return s7ATOrphanPredicateObservation{
		name:              name,
		code:              code,
		stderr:            stderr,
		report:            report,
		expectedRemoved:   expectedRemoved,
		removed:           removed,
		blobPresent:       blobPresent,
		indexBefore:       indexBefore,
		indexAfter:        indexAfter,
		dangling:          dangling,
		mixedHash:         mixedHash,
		mixedBlobBefore:   mixedBlobBefore,
		mixedBlobAfter:    mixedBlobAfter,
		mixedStatesBefore: s7ATWireStates(parsedBefore, mixedHash),
		mixedStatesAfter:  s7ATWireStates(parsedAfter, mixedHash),
	}
}

func validateS7ATGlobalOrphanPredicate(
	observations []s7ATOrphanPredicateObservation,
) error {
	if len(observations) != 2 {
		return fmt.Errorf("orphan observations = %d, want 2", len(observations))
	}
	seen := map[string]bool{}
	for _, observation := range observations {
		if seen[observation.name] {
			return fmt.Errorf("duplicate orphan observation %q", observation.name)
		}
		seen[observation.name] = true
		if observation.code != 0 || observation.stderr != "" ||
			observation.report.Outcome != string(store.IntentArchivePurgePurged) {
			return fmt.Errorf(
				"%s outcome = exit:%d stderr:%q report:%#v",
				observation.name,
				observation.code,
				observation.stderr,
				observation.report,
			)
		}
		if !reflect.DeepEqual(
			observation.removed, observation.expectedRemoved,
		) {
			return fmt.Errorf(
				"%s removed = %v, want %v",
				observation.name,
				observation.removed,
				observation.expectedRemoved,
			)
		}
		for _, hash := range observation.expectedRemoved {
			if observation.blobPresent[hash] {
				return fmt.Errorf("%s left removable orphan %s", observation.name, hash)
			}
		}
		if !bytes.Equal(observation.indexBefore, observation.indexAfter) {
			return fmt.Errorf("%s rewrote index.json", observation.name)
		}
		if observation.dangling {
			return fmt.Errorf("%s left a dangling retained reference", observation.name)
		}
		hasRemainingAdvisory := false
		for _, advisory := range observation.report.Advisories {
			if advisory.Code == "archive-repairs-remaining" {
				hasRemainingAdvisory = true
			}
		}
		switch observation.name {
		case "residue-only":
			if observation.mixedHash != "" ||
				observation.report.RemainingRepairs != nil ||
				hasRemainingAdvisory {
				return errors.New("residue-only purge reported a remaining repair")
			}
		case "residue-plus-mixed":
			if observation.mixedHash == "" ||
				!observation.blobPresent[observation.mixedHash] ||
				!bytes.Equal(
					observation.mixedBlobBefore, observation.mixedBlobAfter,
				) ||
				!reflect.DeepEqual(
					observation.mixedStatesBefore,
					[]store.IntentArchiveWireState{
						store.IntentArchiveWireRetained,
						store.IntentArchiveWireTombstoned,
					},
				) ||
				!reflect.DeepEqual(
					observation.mixedStatesAfter,
					observation.mixedStatesBefore,
				) {
				return fmt.Errorf(
					"mixed evidence changed: present=%t states=%v/%v",
					observation.blobPresent[observation.mixedHash],
					observation.mixedStatesBefore,
					observation.mixedStatesAfter,
				)
			}
			remaining := observation.report.RemainingRepairs
			if remaining == nil || !remaining.RerunRequired ||
				remaining.StagesRemaining != 1 ||
				len(remaining.Stages) != 1 ||
				!hasRemainingAdvisory {
				return fmt.Errorf("mixed repair disclosure = %#v", remaining)
			}
			stage := remaining.Stages[0]
			wantRepair := "tpatch feature intent-archive purge " +
				observation.report.Slug + " --blob " +
				observation.mixedHash + " --yes"
			if stage.Class != string(store.IntentArchiveRepairMixedReference) ||
				!reflect.DeepEqual(stage.Hashes, []string{observation.mixedHash}) ||
				stage.Repair != wantRepair ||
				stage.RepairCWD != store.IntentArchiveRepairCWD {
				return fmt.Errorf("mixed repair stage = %#v", stage)
			}
		default:
			return fmt.Errorf("unknown orphan observation %q", observation.name)
		}
	}
	if !seen["residue-only"] || !seen["residue-plus-mixed"] {
		return fmt.Errorf("orphan observation set = %v", seen)
	}
	return nil
}

func s7ATCloneOrphanObservations(
	source []s7ATOrphanPredicateObservation,
) []s7ATOrphanPredicateObservation {
	cloned := make([]s7ATOrphanPredicateObservation, len(source))
	for index, observation := range source {
		cloned[index] = observation
		cloned[index].expectedRemoved = append(
			[]string(nil), observation.expectedRemoved...,
		)
		cloned[index].removed = append([]string(nil), observation.removed...)
		cloned[index].blobPresent = make(
			map[string]bool, len(observation.blobPresent),
		)
		for hash, present := range observation.blobPresent {
			cloned[index].blobPresent[hash] = present
		}
		if observation.report.RemainingRepairs != nil {
			remaining := *observation.report.RemainingRepairs
			remaining.Stages = append(
				[]intentArchiveRepairStageReport(nil), remaining.Stages...,
			)
			cloned[index].report.RemainingRepairs = &remaining
		}
	}
	return cloned
}

func s7ATMutatedOrphanObservations(
	source []s7ATOrphanPredicateObservation,
	mutate func([]s7ATOrphanPredicateObservation),
) ([]s7ATOrphanPredicateObservation, error) {
	mutated := s7ATCloneOrphanObservations(source)
	mutate(mutated)
	if reflect.DeepEqual(mutated, source) {
		return nil, errors.New("orphan sensitivity mutation changed nothing")
	}
	return mutated, nil
}

func s7ATProductionOrphanObservations(
	t *testing.T,
) []s7ATOrphanPredicateObservation {
	t.Helper()
	return []s7ATOrphanPredicateObservation{
		s7ATObserveOrphanPredicate(t, false),
		s7ATObserveOrphanPredicate(t, true),
	}
}

func TestS7ATGlobalOrphanPredicateContracts(t *testing.T) {
	if err := validateS7ATGlobalOrphanPredicate(
		s7ATProductionOrphanObservations(t),
	); err != nil {
		t.Fatal(err)
	}
}

func TestS7ATGlobalOrphanPredicateSensitivityPerReference(t *testing.T) {
	observations, err := s7ATMutatedOrphanObservations(
		s7ATProductionOrphanObservations(t),
		func(observations []s7ATOrphanPredicateObservation) {
			observations[1].removed = append(
				observations[1].removed, observations[1].mixedHash,
			)
			sort.Strings(observations[1].removed)
			observations[1].blobPresent[observations[1].mixedHash] = false
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateS7ATGlobalOrphanPredicate(observations); err == nil {
		t.Fatal("orphan validator accepted a per-reference mixed-hash removal")
	}
}

func TestS7ATGlobalOrphanPredicateSensitivitySilentMixed(t *testing.T) {
	observations, err := s7ATMutatedOrphanObservations(
		s7ATProductionOrphanObservations(t),
		func(observations []s7ATOrphanPredicateObservation) {
			observations[1].report.RemainingRepairs = nil
			observations[1].report.Advisories = []prepareAdvisoryReport{}
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateS7ATGlobalOrphanPredicate(observations); err == nil {
		t.Fatal("orphan validator accepted silent mixed-repair omission")
	}
}

func TestS7ATGlobalOrphanPredicateSensitivityPartialCleanup(t *testing.T) {
	observations, err := s7ATMutatedOrphanObservations(
		s7ATProductionOrphanObservations(t),
		func(observations []s7ATOrphanPredicateObservation) {
			omitted := observations[0].expectedRemoved[1]
			observations[0].removed = observations[0].removed[:1]
			observations[0].blobPresent[omitted] = true
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateS7ATGlobalOrphanPredicate(observations); err == nil {
		t.Fatal("orphan validator accepted partial intra-class cleanup")
	}
}

func TestS7ATGlobalOrphanPredicateUnchangedMutationFatal(t *testing.T) {
	observations := s7ATProductionOrphanObservations(t)
	if _, err := s7ATMutatedOrphanObservations(
		observations,
		func([]s7ATOrphanPredicateObservation) {},
	); err == nil {
		t.Fatal("unchanged orphan mutation was accepted")
	}
}
