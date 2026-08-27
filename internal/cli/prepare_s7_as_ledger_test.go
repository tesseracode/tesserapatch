//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"fmt"
	"strings"
	"testing"
)

func TestS7ASCoverageLedger(t *testing.T) {
	manifest := parseS7RowManifest(t)
	rows := s7ASCoverageLedger(t)
	if len(rows) != 10 {
		t.Fatalf("AS ledger rows = %d, want 10", len(rows))
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
		wantID := fmt.Sprintf("PIB-%03d", 521+index)
		authority := manifestByID[row.id]
		if row.id != wantID || authority.category != "AS" || row.kind != authority.kind {
			t.Fatalf("AS ledger row %d = %+v manifest=%+v", index, row, authority)
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
		t.Fatalf("AS exact-target resolution failed:\n%s", strings.Join(resolutionErrors, "\n"))
	}
	wantKinds := map[string]int{"I": 7, "G": 3}
	if fmt.Sprint(kindCounts) != fmt.Sprint(wantKinds) {
		t.Fatalf("AS kind counts = %v, want %v", kindCounts, wantKinds)
	}
	if len(s7ObservedAMThroughAOTargets(t)) != 54 ||
		len(s7PIB443ObservedLeaves(t)) != 12 ||
		len(s7ObservedAPTargets(t)) != 34 ||
		len(s7ObservedAQTargets(t)) != 23 ||
		len(s7ObservedARTargets(t)) != 15 {
		t.Fatal("AS ledger extension weakened the accepted AM-AR target partitions")
	}
	if len(manifest) != 173 {
		t.Fatalf("S7 manifest rows = %d, want 173", len(manifest))
	}
}

func TestS7ASCoverageLedgerRejectsEmptyTarget(t *testing.T) {
	rows := s7ASCoverageLedger(t)
	rows[0].targets = nil
	if err := s7ValidateLedgerTargetPresence(rows); err == nil {
		t.Fatal("AS ledger validator accepted an empty exact target")
	}
}

func s7ASCoverageLedger(t *testing.T) []s7AMLedgerRow {
	t.Helper()
	target := func(test string) s7FixtureTarget {
		return s7FixtureTarget{dir: "internal/cli", pkg: "cli", test: test}
	}
	rows := []s7AMLedgerRow{
		{id: "PIB-521", targets: []s7FixtureTarget{target("TestS7ASUnreferencedResidueContracts")}},
		{id: "PIB-522", targets: []s7FixtureTarget{target("TestS7ASOrphanRepairContracts")}},
		{id: "PIB-523", targets: []s7FixtureTarget{target("TestS7ASOrphanPreviewContracts")}},
		{id: "PIB-524", targets: []s7FixtureTarget{target("TestS7ASX11ClassificationGuard")}},
		{id: "PIB-525", targets: []s7FixtureTarget{target("TestS7ASPendingRecoveryRefusalContracts")}},
		{id: "PIB-526", targets: []s7FixtureTarget{target("TestS7ASRecoverPendingPurgeCallGraphGuard")}},
		{id: "PIB-527", targets: []s7FixtureTarget{target("TestS7ASPendingOrphanRecoveryContracts")}},
		{id: "PIB-528", targets: []s7FixtureTarget{target("TestS7ASSelectorTotalityGuard")}},
		{id: "PIB-529", targets: []s7FixtureTarget{target("TestS7ASPendingPurgeSchemaContracts")}},
		{id: "PIB-530", targets: []s7FixtureTarget{target("TestS7ASAuthorityProcessSpyContracts")}},
	}
	manifestKinds := map[string]string{}
	for _, row := range parseS7RowManifest(t) {
		if row.category == "AS" {
			manifestKinds[row.id] = row.kind
		}
	}
	for index := range rows {
		rows[index].kind = manifestKinds[rows[index].id]
		rows[index].status = "exact"
	}
	return rows
}

func s7ObservedASTargets(t *testing.T) []s7ObservedRegistrationTarget {
	t.Helper()
	rows := s7ASCoverageLedger(t)
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
