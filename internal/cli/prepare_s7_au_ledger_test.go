//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"fmt"
	"strings"
	"testing"
)

func TestS7AUCoverageLedger(t *testing.T) {
	manifest := parseS7RowManifest(t)
	rows := s7AUCoverageLedger(t)
	if len(rows) != 9 {
		t.Fatalf("AU ledger rows = %d, want 9", len(rows))
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
		wantID := fmt.Sprintf("PIB-%03d", 537+index)
		authority := manifestByID[row.id]
		if row.id != wantID || authority.category != "AU" || row.kind != authority.kind {
			t.Fatalf("AU ledger row %d = %+v manifest=%+v", index, row, authority)
		}
		if row.status != "exact" || len(row.targets) != 1 {
			t.Fatalf("%s status/targets = %q/%d, want exact/1", row.id, row.status, len(row.targets))
		}
		if row.targets[0].subtest != "" {
			t.Fatalf("%s target subtest = %q, want top-level body", row.id, row.targets[0].subtest)
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
		t.Fatalf("AU exact-target resolution failed:\n%s", strings.Join(resolutionErrors, "\n"))
	}
	wantKinds := map[string]int{"I": 6, "C": 2, "G": 1}
	if fmt.Sprint(kindCounts) != fmt.Sprint(wantKinds) {
		t.Fatalf("AU kind counts = %v, want %v", kindCounts, wantKinds)
	}
	// Verify prior partitions are unchanged
	if len(s7ObservedAMThroughAOTargets(t)) != 54 ||
		len(s7PIB443ObservedLeaves(t)) != 12 ||
		len(s7ObservedAPTargets(t)) != 34 ||
		len(s7ObservedAQTargets(t)) != 23 ||
		len(s7ObservedARTargets(t)) != 15 ||
		len(s7ObservedASTargets(t)) != 10 ||
		len(s7ObservedATTargets(t)) != 6 {
		t.Fatal("AU ledger extension weakened the accepted AM-AT target partitions")
	}
	if len(manifest) != 173 {
		t.Fatalf("S7 manifest rows = %d, want 173", len(manifest))
	}
}

func TestS7AUCoverageLedgerRejectsEmptyTarget(t *testing.T) {
	rows := s7AUCoverageLedger(t)
	rows[0].targets = nil
	if err := s7ValidateLedgerTargetPresence(rows); err == nil {
		t.Fatal("AU ledger validator accepted an empty exact target")
	}
}

func s7AUCoverageLedger(t *testing.T) []s7AMLedgerRow {
	t.Helper()
	target := func(test string) s7FixtureTarget {
		return s7FixtureTarget{dir: "internal/cli", pkg: "cli", test: test}
	}
	rows := []s7AMLedgerRow{
		{id: "PIB-537", targets: []s7FixtureTarget{target("TestS7AUAbandonBooleanDomainContracts")}},
		{id: "PIB-538", targets: []s7FixtureTarget{target("TestS7AUManualModeRefusalContracts")}},
		{id: "PIB-539", targets: []s7FixtureTarget{target("TestS7AURetainedPendingSameHashContracts")}},
		{id: "PIB-540", targets: []s7FixtureTarget{target("TestS7AUCrashInjectionContracts")}},
		{id: "PIB-541", targets: []s7FixtureTarget{target("TestS7AUListDoctorMultiObservationContracts")}},
		{id: "PIB-542", targets: []s7FixtureTarget{target("TestS7AUDisjointSelectorRefusalContracts")}},
		{id: "PIB-543", targets: []s7FixtureTarget{target("TestS7AUCorruptBlobRouteContracts")}},
		{id: "PIB-544", targets: []s7FixtureTarget{target("TestS7AUSameHashInsertionWindowContracts")}},
		{id: "PIB-545", targets: []s7FixtureTarget{target("TestS7AUClassificationAuthorityGuard")}},
	}
	manifestKinds := map[string]string{}
	for _, row := range parseS7RowManifest(t) {
		if row.category == "AU" {
			manifestKinds[row.id] = row.kind
		}
	}
	for index := range rows {
		rows[index].kind = manifestKinds[rows[index].id]
		rows[index].status = "exact"
	}
	return rows
}

func s7ObservedAUTargets(t *testing.T) []s7ObservedRegistrationTarget {
	t.Helper()
	rows := s7AUCoverageLedger(t)
	observed := make([]s7ObservedRegistrationTarget, 0, len(rows))
	seen := map[string]string{}
	for _, row := range rows {
		selected := row.targets[0]
		key := s7ObservedTargetKey(selected)
		if previous := seen[key]; previous != "" {
			t.Fatalf("%s and %s share observed-registration target %s", previous, row.id, key)
		}
		seen[key] = row.id
		observed = append(observed, s7ObservedRegistrationTarget{row: row.id, target: selected})
	}
	return observed
}
