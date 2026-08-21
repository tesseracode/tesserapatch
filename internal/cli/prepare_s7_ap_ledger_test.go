//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"fmt"
	"strings"
	"testing"
)

func TestS7APCoverageLedger(t *testing.T) {
	manifest := parseS7RowManifest(t)
	rows := s7APCoverageLedger(t)
	if len(rows) != 34 {
		t.Fatalf("AP ledger rows = %d, want 34", len(rows))
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
		wantID := fmt.Sprintf("PIB-%03d", 449+index)
		authority := manifestByID[row.id]
		if row.id != wantID || authority.category != "AP" || row.kind != authority.kind {
			t.Fatalf("AP ledger row %d = %+v manifest=%+v", index, row, authority)
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
		t.Fatalf("AP exact-target resolution failed:\n%s", strings.Join(resolutionErrors, "\n"))
	}
	wantKinds := map[string]int{"I": 9, "C": 12, "G": 9, "U": 2, "S": 2}
	if fmt.Sprint(kindCounts) != fmt.Sprint(wantKinds) {
		t.Fatalf("AP kind counts = %v, want %v", kindCounts, wantKinds)
	}
}

func TestS7APCoverageLedgerRejectsEmptyTarget(t *testing.T) {
	rows := s7APCoverageLedger(t)
	rows[0].targets = nil
	if err := s7ValidateLedgerTargetPresence(rows); err == nil {
		t.Fatal("AP ledger validator accepted an empty exact target")
	}
}

func s7APCoverageLedger(t *testing.T) []s7AMLedgerRow {
	t.Helper()
	target := func(dir, pkg, test, subtest string) s7FixtureTarget {
		return s7FixtureTarget{dir: dir, pkg: pkg, test: test, subtest: subtest}
	}
	rows := []s7AMLedgerRow{
		{id: "PIB-449", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7APAbandonContracts", "PIB-449")}},
		{id: "PIB-450", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7APAbandonContracts", "PIB-450")}},
		{id: "PIB-451", targets: []s7FixtureTarget{
			target("internal/cli", "cli", "TestS7APAbandonContracts", "PIB-451"),
		}},
		{id: "PIB-452", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7APAbandonContracts", "PIB-452")}},
		{id: "PIB-453", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7APAbandonContracts", "PIB-453")}},
		{id: "PIB-454", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7APRootedWriterGuards", "PIB-454")}},
		{id: "PIB-455", targets: []s7FixtureTarget{target("internal/intentpub", "intentpub", "TestS7APDurableWriterContracts", "PIB-455")}},
		{id: "PIB-456", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7APRootedWriterGuards", "PIB-456")}},
		{id: "PIB-457", targets: []s7FixtureTarget{
			target("internal/cli", "cli", "TestS7APDanglingPublicWorkflows", "PIB-457"),
			target("internal/store", "store", "TestS7APPurgeContracts", "PIB-457"),
		}},
		{id: "PIB-458", targets: []s7FixtureTarget{
			target("internal/cli", "cli", "TestS7APDanglingPublicWorkflows", "PIB-458"),
			target("internal/store", "store", "TestS7APPurgeContracts", "PIB-458"),
		}},
		{id: "PIB-459", targets: []s7FixtureTarget{
			target("internal/cli", "cli", "TestS7APDanglingPublicWorkflows", "PIB-459"),
			target("internal/store", "store", "TestS7APPurgeContracts", "PIB-459"),
		}},
		{id: "PIB-460", targets: []s7FixtureTarget{
			target("internal/cli", "cli", "TestS7APDanglingPublicWorkflows", "PIB-460"),
			target("internal/store", "store", "TestS7APPurgeContracts", "PIB-460"),
		}},
		{id: "PIB-461", targets: []s7FixtureTarget{
			target("internal/cli", "cli", "TestS7APDryRunContracts", "PIB-461"),
		}},
		{id: "PIB-462", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7APDryRunContracts", "PIB-462")}},
		{id: "PIB-463", targets: []s7FixtureTarget{
			target("internal/cli", "cli", "TestS7APDryRunContracts", "PIB-463"),
			target("internal/cli", "cli", "TestS7APDryRunWindowsNotEvaluatedPlatform", "PIB-463"),
			target("internal/cli", "cli", "TestS7APWindowsDryRunBlockingGuard", "PIB-463"),
		}},
		{id: "PIB-464", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7APDryRunContracts", "PIB-464")}},
		{id: "PIB-465", targets: []s7FixtureTarget{
			target("internal/store", "store", "TestS7APPurgeContracts", "PIB-465"),
			target("internal/store", "store", "TestIntentArchivePurgeSelectorValidationAndSharedGeneration", ""),
			target("internal/store", "store", "TestIntentArchiveGlobalRefusalPrecedesSharedAndPopulatesPlan", ""),
			target("internal/store", "store", "TestIntentArchiveSequentialTwoClassRepair", ""),
			target("internal/store", "store", "TestIntentArchiveCorruptFirstStageArithmeticAndBlocking", ""),
		}},
		{id: "PIB-466", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7APPurgeCLIContracts", "PIB-466")}},
		{id: "PIB-467", targets: []s7FixtureTarget{
			target("internal/cli", "cli", "TestS7APPurgeCLIContracts", "PIB-467"),
			target("internal/store", "store", "TestS7APPurgeContracts", "PIB-467"),
		}},
		{id: "PIB-468", targets: []s7FixtureTarget{
			target("internal/cli", "cli", "TestS7APPurgeCLIContracts", "PIB-468"),
			target("internal/store", "store", "TestS7APPurgeContracts", "PIB-468"),
		}},
		{id: "PIB-469", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7APPurgeControlFlowGuard", "PIB-469")}},
		{id: "PIB-470", targets: []s7FixtureTarget{target("internal/workflow", "workflow", "TestS7APDoctorD9Contracts", "PIB-470")}},
		{id: "PIB-471", targets: []s7FixtureTarget{
			target("internal/workflow", "workflow", "TestS7APDoctorD9Contracts", "PIB-471"),
			target("internal/cli", "cli", "TestS7APTransactionInProgressTruth", "PIB-471"),
		}},
		{id: "PIB-472", targets: []s7FixtureTarget{target("internal/gitutil", "gitutil", "TestS7APGitContracts", "PIB-472")}},
		{id: "PIB-473", targets: []s7FixtureTarget{
			target("internal/gitutil", "gitutil", "TestS7APGitContracts", "PIB-473"),
			target("internal/rescap", "rescap", "TestS7APCompatibilityWrappersRuntime", "PIB-473"),
		}},
		{id: "PIB-474", targets: []s7FixtureTarget{target("internal/gitutil", "gitutil", "TestS7APGitContracts", "PIB-474")}},
		{id: "PIB-475", targets: []s7FixtureTarget{
			target("internal/gitutil", "gitutil", "TestS7APGitContracts", "PIB-475"),
			target("internal/cli", "cli", "TestS7APPrepareGitRuntime", "PIB-475"),
		}},
		{id: "PIB-476", targets: []s7FixtureTarget{
			target("internal/gitutil", "gitutil", "TestS7APGitContracts", "PIB-476"),
			target("internal/cli", "cli", "TestS7APPrepareGitRuntime", "PIB-476"),
			target("internal/workflow", "workflow", "TestS7APDoctorD9Contracts", "PIB-476"),
		}},
		{id: "PIB-477", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7APProvenanceGuard", "PIB-477")}},
		{id: "PIB-478", targets: []s7FixtureTarget{target("internal/intentlock", "intentlock", "TestS7APAuthorityContracts", "PIB-478")}},
		{id: "PIB-479", targets: []s7FixtureTarget{
			target("internal/intentlock", "intentlock", "TestS7APAuthorityContracts", "PIB-479"),
			target("internal/intentlock", "intentlock", "TestS7APDarwinFilesystemPredicate", "PIB-479"),
			target("internal/intentlock", "intentlock", "TestS7APLinuxFilesystemMagic", "PIB-479"),
		}},
		{id: "PIB-480", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7APFilesystemCLIContract", "PIB-480")}},
		{id: "PIB-481", targets: []s7FixtureTarget{target("internal/intentlock", "intentlock", "TestS7APAuthorityContracts", "PIB-481")}},
		{id: "PIB-482", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7APReferenceTruthGuard", "PIB-482")}},
	}
	manifestKinds := map[string]string{}
	for _, row := range parseS7RowManifest(t) {
		if row.category == "AP" {
			manifestKinds[row.id] = row.kind
		}
	}
	for index := range rows {
		rows[index].kind = manifestKinds[rows[index].id]
		rows[index].status = "exact"
	}
	return rows
}
