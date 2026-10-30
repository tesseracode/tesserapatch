package workflow

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func validateROCS5LifecycleSource(source string) error {
	for _, needle := range []string{
		"acquireOperationCandidateFeatureLock",
		"findOperationCandidateRecoveryJournal",
		"stalePendingOperationCandidate(s, slug, status, current, lock)",
		"candidate.CandidateNeedsStaleMark",
		"pruneOperationCandidateWorktree",
		"appendRejectedOperationCandidate(s.Root, slug, artifact, lock)",
		"clearOperationCandidateRejectionsLocked",
	} {
		if !strings.Contains(source, needle) {
			return fmt.Errorf("missing lifecycle authority %s", needle)
		}
	}
	lockIdx := strings.Index(source, "acquireOperationCandidateFeatureLock")
	recoveryIdx := strings.Index(source, "findOperationCandidateRecoveryJournal")
	staleIdx := strings.Index(source, "stalePendingOperationCandidate(s, slug, status, current, lock)")
	if lockIdx < 0 || recoveryIdx < 0 || staleIdx < 0 || !(lockIdx < recoveryIdx && recoveryIdx < staleIdx) {
		return fmt.Errorf("preflight order changed: lock -> recovery -> stale")
	}
	if strings.Contains(source, "OpenCandidateRoot(repoRoot") || strings.Contains(source, "CreateCandidateRoot(s.Root") {
		return fmt.Errorf("lifecycle reintroduced bare-root candidate constructor forwarding")
	}
	return nil
}

func TestROCS5LifecycleSourceGuardAndSensitivity(t *testing.T) {
	body, err := os.ReadFile("operation_candidate_lifecycle.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	if err := validateROCS5LifecycleSource(source); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range [][2]string{
		{"acquireOperationCandidateFeatureLock", "skipLockAcquire"},
		{"findOperationCandidateRecoveryJournal", "readRecoveryJournalAfterPrune"},
		{"stalePendingOperationCandidate(s, slug, status, current, lock)", "noopStaleCandidate(s, slug, status, current, lock)"},
		{"appendRejectedOperationCandidate(s.Root, slug, artifact, lock)", "writeRejectedTimestamped(s.Root, slug, artifact, lock)"},
	} {
		if strings.Count(source, mutation[0]) == 0 {
			t.Fatalf("missing mutation anchor %q", mutation[0])
		}
		mutant := strings.Replace(source, mutation[0], mutation[1], 1)
		if err := validateROCS5LifecycleSource(mutant); err == nil {
			t.Fatalf("lifecycle guard accepted mutation %q", mutation[1])
		}
	}
}
