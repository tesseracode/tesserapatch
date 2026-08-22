//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"fmt"
	"strings"
	"testing"
)

func TestS7AQCoverageLedger(t *testing.T) {
	manifest := parseS7RowManifest(t)
	rows := s7AQCoverageLedger(t)
	if len(rows) != 23 {
		t.Fatalf("AQ ledger rows = %d, want 23", len(rows))
	}
	if err := s7ValidateLedgerTargetPresence(rows); err != nil {
		t.Fatal(err)
	}

	manifestByID := map[string]s7ManifestRow{}
	for _, row := range manifest {
		manifestByID[row.id] = row
	}
	kindCounts := map[string]int{}
	var resolutionErrors []string
	for index, row := range rows {
		wantID := fmt.Sprintf("PIB-%03d", 483+index)
		authority := manifestByID[row.id]
		if row.id != wantID || authority.category != "AQ" || row.kind != authority.kind {
			t.Fatalf("AQ ledger row %d = %+v manifest=%+v", index, row, authority)
		}
		if row.status != "exact" || len(row.targets) == 0 {
			t.Fatalf("%s status/targets = %q/%d, want exact/nonempty", row.id, row.status, len(row.targets))
		}
		kindCounts[row.kind]++
		for _, target := range row.targets {
			if err := resolveS7FixtureTarget(t, target); err != nil {
				resolutionErrors = append(
					resolutionErrors,
					fmt.Sprintf("%s target %+v: %v", row.id, target, err),
				)
			}
		}
	}
	if len(resolutionErrors) != 0 {
		t.Fatalf("AQ exact-target resolution failed:\n%s", strings.Join(resolutionErrors, "\n"))
	}
	wantKinds := map[string]int{"I": 13, "C": 3, "G": 7}
	if fmt.Sprint(kindCounts) != fmt.Sprint(wantKinds) {
		t.Fatalf("AQ kind counts = %v, want %v", kindCounts, wantKinds)
	}

	if len(s7ObservedAMThroughAOTargets(t)) != 54 ||
		len(s7PIB443ObservedLeaves(t)) != 12 ||
		len(s7ObservedAPTargets(t)) != 34 {
		t.Fatal("AQ ledger extension weakened the accepted AM-AP target partitions")
	}
}

func TestS7AQCoverageLedgerRejectsEmptyTarget(t *testing.T) {
	rows := s7AQCoverageLedger(t)
	rows[0].targets = nil
	if err := s7ValidateLedgerTargetPresence(rows); err == nil {
		t.Fatal("AQ ledger validator accepted an empty exact target")
	}
}

func s7AQCoverageLedger(t *testing.T) []s7AMLedgerRow {
	t.Helper()
	target := func(dir, pkg, test, subtest string) s7FixtureTarget {
		return s7FixtureTarget{dir: dir, pkg: pkg, test: test, subtest: subtest}
	}
	rows := []s7AMLedgerRow{
		{id: "PIB-483", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQTerminalRecoveryContracts", "PIB-483")}},
		{id: "PIB-484", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQTerminalRecoveryContracts", "PIB-484")}},
		{id: "PIB-485", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQTerminalRecoveryContracts", "PIB-485")}},
		{id: "PIB-486", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQPendingPurgeContracts", "PIB-486")}},
		{id: "PIB-487", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQPostRecoveryControlFlowGuard", "PIB-487")}},
		{id: "PIB-488", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQTerminalRecoveryContracts", "PIB-488")}},
		{id: "PIB-489", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQPendingPurgeContracts", "PIB-489")}},
		{id: "PIB-490", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQPendingPurgeContracts", "PIB-490")}},
		{id: "PIB-491", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQPendingPurgeContracts", "PIB-491")}},
		{id: "PIB-492", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQAbandonContracts", "PIB-492")}},
		{id: "PIB-493", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQAbandonContracts", "PIB-493")}},
		{id: "PIB-494", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQAbandonContracts", "PIB-494")}},
		{id: "PIB-495", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQAbandonContracts", "PIB-495")}},
		{id: "PIB-496", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQRecoverabilityClaimsAreEnvironmentallyQualified", "")}},
		{id: "PIB-497", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQAbsoluteRootNeverReachesReportsOrProse", "")}},
		{id: "PIB-498", targets: []s7FixtureTarget{
			target("internal/cli", "cli", "TestS7AQRerunEmittersAreDerivedAndCanonical", ""),
			target("internal/cli", "cli", "TestFeatureIntentArchivePartialBranchesAndDivergenceReports", "pending-recovery-then-completion"),
			target("internal/cli", "cli", "TestFeatureIntentArchivePartialBranchesAndDivergenceReports", "completion-only"),
			target("internal/cli", "cli", "TestFeatureIntentArchivePartialBranchesAndDivergenceReports", "orphan-scan"),
			target("internal/cli", "cli", "TestFeatureIntentArchiveCorruptClassRemediationUsesPredictedClasses", "complete-class-then-one-total-dangling-retry"),
			target("internal/cli", "cli", "TestFeatureIntentArchiveListDanglingCorruptAndOwnedTruth", "dangling"),
			target("internal/cli", "cli", "TestFeatureIntentArchiveListDanglingCorruptAndOwnedTruth", "corrupt"),
			target("internal/cli", "cli", "TestFeatureIntentArchiveListGlobalAvailabilityAndCompleteObservations", ""),
		}},
		{id: "PIB-499", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQAbandonContracts", "PIB-499")}},
		{id: "PIB-500", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQFlagContracts", "PIB-500")}},
		{id: "PIB-501", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQFlagContracts", "PIB-501")}},
		{id: "PIB-502", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQFlagContracts", "PIB-502")}},
		{id: "PIB-503", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQYesSourceGuard", "PIB-503")}},
		{id: "PIB-504", targets: []s7FixtureTarget{target("internal/intentlock", "intentlock", "TestS7AQAuthorityUsesRetainedDescriptorControl", "")}},
		{id: "PIB-505", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7AQStepReferencesResolveToDescribedSemantics", "")}},
	}
	manifestKinds := map[string]string{}
	for _, row := range parseS7RowManifest(t) {
		if row.category == "AQ" {
			manifestKinds[row.id] = row.kind
		}
	}
	for index := range rows {
		rows[index].kind = manifestKinds[rows[index].id]
		rows[index].status = "exact"
	}
	return rows
}
