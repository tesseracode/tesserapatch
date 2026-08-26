//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestS7ARCoverageLedger(t *testing.T) {
	manifest := parseS7RowManifest(t)
	rows := s7ARCoverageLedger(t)
	if len(rows) != 15 {
		t.Fatalf("AR ledger rows = %d, want 15", len(rows))
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
		wantID := fmt.Sprintf("PIB-%03d", 506+index)
		authority := manifestByID[row.id]
		if row.id != wantID || authority.category != "AR" || row.kind != authority.kind {
			t.Fatalf("AR ledger row %d = %+v manifest=%+v", index, row, authority)
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
		t.Fatalf("AR exact-target resolution failed:\n%s", strings.Join(resolutionErrors, "\n"))
	}
	wantKinds := map[string]int{"I": 6, "C": 4, "G": 4, "S": 1}
	if fmt.Sprint(kindCounts) != fmt.Sprint(wantKinds) {
		t.Fatalf("AR kind counts = %v, want %v", kindCounts, wantKinds)
	}

	if len(s7ObservedAMThroughAOTargets(t)) != 54 ||
		len(s7PIB443ObservedLeaves(t)) != 12 ||
		len(s7ObservedAPTargets(t)) != 34 ||
		len(s7ObservedAQTargets(t)) != 23 {
		t.Fatal("AR ledger extension weakened the accepted AM-AQ target partitions")
	}
	if len(manifest) != 173 {
		t.Fatalf("S7 manifest rows = %d, want 173", len(manifest))
	}
	prd := s6RepoFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
	section, err := s7ARSectionBetween(
		prd,
		"| Slice | Categories | Rows |",
		"\n\nSum = 567;",
	)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, match := range regexp.MustCompile(`(?m)^\| S[0-7][^|]*\|[^|]*\| ([0-9]+) \|$`).FindAllStringSubmatch(section, -1) {
		count, convertErr := strconv.Atoi(match[1])
		if convertErr != nil {
			t.Fatal(convertErr)
		}
		total += count
	}
	if total != 567 || !strings.Contains(prd, "zero unassigned, zero double-assigned") {
		t.Fatalf("slice partition total = %d, want 567 with zero overlap/gaps", total)
	}
}

func TestS7ARCoverageLedgerRejectsEmptyTarget(t *testing.T) {
	rows := s7ARCoverageLedger(t)
	rows[0].targets = nil
	if err := s7ValidateLedgerTargetPresence(rows); err == nil {
		t.Fatal("AR ledger validator accepted an empty exact target")
	}
}

func s7ARCoverageLedger(t *testing.T) []s7AMLedgerRow {
	t.Helper()
	target := func(test, subtest string) s7FixtureTarget {
		return s7FixtureTarget{
			dir: "internal/cli", pkg: "cli", test: test, subtest: subtest,
		}
	}
	rows := []s7AMLedgerRow{
		{id: "PIB-506", targets: []s7FixtureTarget{
			target("TestS7ARDivergenceContracts", "PIB-506/regular"),
			target("TestS7ARDivergenceContracts", "PIB-506/symlink"),
			target("TestS7ARDivergenceContracts", "PIB-506/directory"),
			target("TestS7ARDivergenceContracts", "PIB-506/fifo"),
			target("TestS7ARDivergenceContracts", "PIB-506/device-seam"),
			target("TestS7ARDivergenceContracts", "PIB-506/index-strict-decode-stop"),
			target("TestS7ARDivergenceContracts", "PIB-506/non-owned-tombstone-live-false"),
			target("TestS7ARDivergenceContracts", "PIB-506/non-owned-tombstone-live-true"),
		}},
		{id: "PIB-507", targets: []s7FixtureTarget{
			target("TestS7ARDivergenceContracts", "PIB-507/regular"),
			target("TestS7ARDivergenceContracts", "PIB-507/symlink"),
			target("TestS7ARDivergenceContracts", "PIB-507/directory"),
			target("TestS7ARDivergenceContracts", "PIB-507/fifo"),
			target("TestS7ARDivergenceContracts", "PIB-507/device-seam"),
		}},
		{id: "PIB-508", targets: []s7FixtureTarget{target("TestS7ARExitSixRouteGuard", "PIB-508")}},
		{id: "PIB-509", targets: []s7FixtureTarget{
			target("TestS7ARAbandonContracts", "PIB-509/absent-feature"),
			target("TestS7ARAbandonContracts", "PIB-509/malformed-status"),
			target("TestS7ARAbandonContracts", "PIB-509/unreadable-status"),
		}},
		{id: "PIB-510", targets: []s7FixtureTarget{target("TestS7ARAbandonContracts", "PIB-510")}},
		{id: "PIB-511", targets: []s7FixtureTarget{target("TestS7ARAbandonGateTableGuard", "PIB-511")}},
		{id: "PIB-512", targets: []s7FixtureTarget{target("TestS7ARAbandonContracts", "PIB-512")}},
		{id: "PIB-513", targets: []s7FixtureTarget{target("TestS7ARAbandonContracts", "PIB-513")}},
		{id: "PIB-514", targets: []s7FixtureTarget{target("TestS7ARArchiveControlContracts", "PIB-514")}},
		{id: "PIB-515", targets: []s7FixtureTarget{target("TestS7ARArchiveControlContracts", "PIB-515")}},
		{id: "PIB-516", targets: []s7FixtureTarget{target("TestS7ARArchiveControlContracts", "PIB-516")}},
		{id: "PIB-517", targets: []s7FixtureTarget{target("TestS7ARArchiveControlContracts", "PIB-517")}},
		{id: "PIB-518", targets: []s7FixtureTarget{target("TestS7ARPurgeProgressGuard", "PIB-518")}},
		{id: "PIB-519", targets: []s7FixtureTarget{target("TestS7ARPermanentBlockClaimsGuard", "PIB-519")}},
		{id: "PIB-520", targets: []s7FixtureTarget{target("TestS7ARPrepareGrammarGuard", "PIB-520")}},
	}
	manifestKinds := map[string]string{}
	for _, row := range parseS7RowManifest(t) {
		if row.category == "AR" {
			manifestKinds[row.id] = row.kind
		}
	}
	for index := range rows {
		rows[index].kind = manifestKinds[rows[index].id]
		rows[index].status = "exact"
	}
	return rows
}
