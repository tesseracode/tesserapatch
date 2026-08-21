//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type s7ManifestRow struct {
	id       string
	kind     string
	category string
}

type s7FixtureTarget struct {
	dir     string
	pkg     string
	test    string
	subtest string
}

type s7AMLedgerRow struct {
	id      string
	kind    string
	status  string
	targets []s7FixtureTarget
}

func TestS7RowManifestAndAMCoverageLedger(t *testing.T) {
	manifest := parseS7RowManifest(t)
	if len(manifest) != 173 {
		t.Fatalf("S7 manifest rows = %d, want 173", len(manifest))
	}
	categoryCounts := map[string]int{}
	kindCounts := map[string]int{}
	for index, row := range manifest {
		wantID := fmt.Sprintf("PIB-%03d", 395+index)
		if row.id != wantID {
			t.Fatalf("S7 manifest row %d = %s, want %s", index, row.id, wantID)
		}
		categoryCounts[row.category]++
		kindCounts[row.kind]++
	}
	expectedCategories := parseS7CategoryCountsFromSection1852(t)
	if fmt.Sprint(categoryCounts) != fmt.Sprint(expectedCategories) {
		t.Fatalf("S7 category counts = %v, §18.52 says %v", categoryCounts, expectedCategories)
	}
	expectedKinds := map[string]int{"I": 69, "C": 43, "G": 51, "U": 6, "S": 4}
	if fmt.Sprint(kindCounts) != fmt.Sprint(expectedKinds) {
		t.Fatalf("S7 kind counts = %v, want %v", kindCounts, expectedKinds)
	}

	ledger := s7AMCoverageLedger()
	if len(ledger) != 15 {
		t.Fatalf("AM ledger rows = %d, want 15", len(ledger))
	}
	if err := s7ValidateLedgerTargetPresence(ledger); err != nil {
		t.Fatal(err)
	}
	manifestByID := map[string]s7ManifestRow{}
	for _, row := range manifest {
		manifestByID[row.id] = row
	}
	statusCounts := map[string]int{}
	var resolutionErrors []string
	for index, row := range ledger {
		wantID := fmt.Sprintf("PIB-%03d", 395+index)
		if row.id != wantID || manifestByID[row.id].category != "AM" ||
			manifestByID[row.id].kind != row.kind {
			t.Fatalf("AM ledger row %d = %+v manifest=%+v", index, row, manifestByID[row.id])
		}
		if len(row.targets) == 0 {
			t.Fatalf("%s has no exact target", row.id)
		}
		statusCounts[row.status]++
		for _, target := range row.targets {
			if err := resolveS7FixtureTarget(t, target); err != nil {
				resolutionErrors = append(resolutionErrors, fmt.Sprintf("%s target %+v: %v", row.id, target, err))
			}
		}
	}
	if len(resolutionErrors) != 0 {
		t.Fatalf("AM exact-target resolution failed:\n%s", strings.Join(resolutionErrors, "\n"))
	}
	if statusCounts["exact"] != 15 || len(statusCounts) != 1 {
		t.Fatalf("AM coverage status = %v, want 15 exact", statusCounts)
	}
}

func TestS7FixtureTargetResolverSensitivity(t *testing.T) {
	validNamed := `package fixture
import "testing"
func TestCase(t *testing.T) { t.Run("PIB-409", case409) }
func case409(t *testing.T) { t.Fatal("binding") }
`
	if err := resolveS7FixtureSource(validNamed, "fixture", "TestCase", "PIB-409"); err != nil {
		t.Fatalf("named literal callback rejected: %v", err)
	}
	validTable := `package fixture
import "testing"
func TestCase(t *testing.T) {
	for _, test := range []struct{name string}{{name:"PIB-409"}} {
		t.Run(test.name, func(t *testing.T) { t.Fatal("binding") })
	}
}`
	if err := resolveS7FixtureSource(validTable, "fixture", "TestCase", "PIB-409"); err != nil {
		t.Fatalf("table-bound literal callback rejected: %v", err)
	}

	fixtures := []struct {
		name   string
		source string
		pkg    string
		test   string
		sub    string
	}{
		{"comment", "package fixture\n// func TestCase(t *testing.T){ t.Fatal(\"x\") }\n", "fixture", "TestCase", ""},
		{"string", "package fixture\nconst x = `func TestCase(t *testing.T){ t.Fatal(\"x\") }`\n", "fixture", "TestCase", ""},
		{"wrong-package", "package other\nimport \"testing\"\nfunc TestCase(t *testing.T){ t.Fatal(\"x\") }\n", "fixture", "TestCase", ""},
		{"wrong-signature", "package fixture\nfunc TestCase(t string){}\n", "fixture", "TestCase", ""},
		{"receiver-method", "package fixture\nimport \"testing\"\ntype X struct{}\nfunc (X) TestCase(t *testing.T){ t.Fatal(\"x\") }\n", "fixture", "TestCase", ""},
		{"shadow-receiver", "package fixture\nimport \"testing\"\ntype runner struct{}\nfunc (runner) Run(string, func(*testing.T)){}\nfunc TestCase(t *testing.T){ t2:=runner{}; t2.Run(\"PIB-409\", func(t *testing.T){t.Fatal(\"x\")}) }\n", "fixture", "TestCase", "PIB-409"},
		{"nonliteral", "package fixture\nimport \"testing\"\nfunc TestCase(t *testing.T){ name:=\"PIB-409\"; t.Run(name, func(t *testing.T){t.Fatal(\"x\")}) }\n", "fixture", "TestCase", "PIB-409"},
		{"missing", "package fixture\nimport \"testing\"\nfunc TestCase(t *testing.T){ t.Run(\"other\", func(t *testing.T){t.Fatal(\"x\")}) }\n", "fixture", "TestCase", "PIB-409"},
		{"ambiguous", "package fixture\nimport \"testing\"\nfunc TestCase(t *testing.T){ t.Run(\"PIB-409\", func(t *testing.T){t.Fatal(\"a\")}); t.Run(\"PIB-409\", func(t *testing.T){t.Fatal(\"b\")}) }\n", "fixture", "TestCase", "PIB-409"},
		{"empty", "package fixture\nimport \"testing\"\nfunc TestCase(t *testing.T){}\n", "fixture", "TestCase", ""},
		{"skipped", "package fixture\nimport \"testing\"\nfunc TestCase(t *testing.T){ t.Skip(\"not run\") }\n", "fixture", "TestCase", ""},
		{"unrelated-wrapper", "package fixture\nimport \"testing\"\nfunc TestCase(t *testing.T){ broad(t) }\nfunc broad(t *testing.T){ t.Fatal(\"broad\") }\n", "fixture", "TestCase", ""},
		{"nested-unrelated-assertion", "package fixture\nimport \"testing\"\nfunc TestCase(t *testing.T){ t.Run(\"other\", func(t *testing.T){ t.Fatal(\"not binding\") }) }\n", "fixture", "TestCase", ""},
		{"selected-nested-unrelated-assertion", "package fixture\nimport \"testing\"\nfunc TestCase(t *testing.T){ t.Run(\"PIB-409\", func(t *testing.T){ t.Run(\"other\", func(t *testing.T){ t.Fatal(\"not binding\") }) }) }\n", "fixture", "TestCase", "PIB-409"},
		{"broad-helper-selected", "package fixture\nimport \"testing\"\nfunc TestCase(t *testing.T){ t.Run(\"PIB-409\", func(t *testing.T){ broad(t) }) }\nfunc broad(t *testing.T){ t.Fatal(\"broad\") }\n", "fixture", "TestCase", "PIB-409"},
		{"dead-assertion", "package fixture\nimport \"testing\"\nfunc TestCase(t *testing.T){ t.Run(\"PIB-409\", func(t *testing.T){ if false { t.Fatal(\"dead\") } }) }\n", "fixture", "TestCase", "PIB-409"},
		{"post-return-assertion", "package fixture\nimport \"testing\"\nfunc TestCase(t *testing.T){ t.Run(\"PIB-409\", func(t *testing.T){ return; t.Fatal(\"dead\") }) }\n", "fixture", "TestCase", "PIB-409"},
		{"aggregate-parent-assertion", "package fixture\nimport \"testing\"\nfunc TestCase(t *testing.T){ t.Run(\"PIB-409\", func(t *testing.T){ t.Fatal(\"parent\"); t.Run(\"leaf\", func(t *testing.T){ t.Fatal(\"leaf\") }) }) }\n", "fixture", "TestCase", "PIB-409"},
		{"aggregate-test-wrapper", "package fixture\nimport \"testing\"\nfunc TestCase(t *testing.T){ t.Fatal(\"parent\"); t.Run(\"PIB-409\", func(t *testing.T){ t.Fatal(\"leaf\") }) }\n", "fixture", "TestCase", ""},
		{"missing-table-case", strings.Replace(validTable, `"PIB-409"`, `"PIB-410"`, 1), "fixture", "TestCase", "PIB-409"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			if err := resolveS7FixtureSource(
				fixture.source, fixture.pkg, fixture.test, fixture.sub,
			); err == nil {
				t.Fatalf("resolver accepted %s false positive", fixture.name)
			}
		})
	}
}

func TestS7ANCoverageLedger(t *testing.T) {
	manifest := parseS7RowManifest(t)
	manifestByID := map[string]s7ManifestRow{}
	for _, row := range manifest {
		manifestByID[row.id] = row
	}
	ledger := s7ANCoverageLedger()
	if len(ledger) != 23 {
		t.Fatalf("AN ledger rows = %d, want 23", len(ledger))
	}
	if err := s7ValidateLedgerTargetPresence(ledger); err != nil {
		t.Fatal(err)
	}
	var resolutionErrors []string
	for index, row := range ledger {
		wantID := fmt.Sprintf("PIB-%03d", 410+index)
		normative := manifestByID[row.id]
		if row.id != wantID || row.status != "exact" ||
			normative.category != "AN" || normative.kind != row.kind {
			t.Fatalf("AN ledger row %d = %+v normative=%+v", index, row, normative)
		}
		if len(row.targets) == 0 {
			t.Fatalf("%s has no exact targets", row.id)
		}
		for _, target := range row.targets {
			if err := resolveS7FixtureTarget(t, target); err != nil {
				resolutionErrors = append(resolutionErrors, fmt.Sprintf("%s target %+v: %v", row.id, target, err))
			}
		}
	}
	if len(resolutionErrors) != 0 {
		t.Fatalf("AN exact-target resolution failed:\n%s", strings.Join(resolutionErrors, "\n"))
	}
}

func s7ANCoverageLedger() []s7AMLedgerRow {
	target := func(dir, pkg, test string) s7FixtureTarget {
		return s7FixtureTarget{dir: dir, pkg: pkg, test: test}
	}
	subtest := func(dir, pkg, test, sub string) s7FixtureTarget {
		return s7FixtureTarget{dir: dir, pkg: pkg, test: test, subtest: sub}
	}
	return []s7AMLedgerRow{
		{id: "PIB-410", kind: "C", status: "exact", targets: []s7FixtureTarget{
			target("internal/intentlock", "intentlock", "TestS7PIB410OneRetainedDirectorySuppliesAuthorityAndRootedIO"),
		}},
		{id: "PIB-411", kind: "C", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/intentlock", "intentlock", "TestS7PIB411OnlyWouldBlockAndAgainAreContention", "would-block"),
			subtest("internal/intentlock", "intentlock", "TestS7PIB411OnlyWouldBlockAndAgainAreContention", "again"),
			subtest("internal/intentlock", "intentlock", "TestS7PIB411OnlyWouldBlockAndAgainAreContention", "other"),
		}},
		{id: "PIB-412", kind: "I", status: "exact", targets: []s7FixtureTarget{
			target("internal/cli", "cli", "TestS7PIB412NestedMutatorThreadsOneAuthorityAndSerializesSlugs"),
		}},
		{id: "PIB-413", kind: "I", status: "exact", targets: []s7FixtureTarget{
			target("internal/intentlock", "intentlock", "TestPIB413RenameRetainsInodeContentionAndInvalidatesOriginalAlias"),
		}},
		{id: "PIB-414", kind: "C", status: "exact", targets: []s7FixtureTarget{
			target("internal/cli", "cli", "TestS7PIB414RootReplacementBeforePublicationRefusesAndPreservesReplacement"),
		}},
		{id: "PIB-415", kind: "C", status: "exact", targets: []s7FixtureTarget{
			target("internal/cli", "cli", "TestS7PIB415RootReplacementAfterPublicationIsExitSixWithEvidence"),
		}},
		{id: "PIB-416", kind: "U", status: "exact", targets: []s7FixtureTarget{
			target("internal/intentlock", "intentlock", "TestS7PIB416LinuxRootFilesystemClassificationTable"),
			target("internal/intentlock", "intentlock", "TestS7PIB416DarwinRootFilesystemClassificationTable"),
			target("internal/cli", "cli", "TestS7PIB416DeniedFilesystemClassesReachPublicCLIReports"),
		}},
		{id: "PIB-417", kind: "U", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/cli", "cli", "TestS7WindowsPlatformRows", "PIB-417/check-ready-human.txt"),
			subtest("internal/cli", "cli", "TestS7WindowsPlatformRows", "PIB-417/check-ready-json.txt"),
			subtest("internal/cli", "cli", "TestS7BSDPlatformRows", "PIB-417/check-ready-human.txt"),
			subtest("internal/cli", "cli", "TestS7BSDPlatformRows", "PIB-417/check-ready-json.txt"),
			subtest("internal/cli", "cli", "TestS7BSDPlatformPredicateSeamRuntime", "PIB-417/check-ready-human.txt"),
			subtest("internal/cli", "cli", "TestS7BSDPlatformPredicateSeamRuntime", "PIB-417/check-ready-json.txt"),
			target("internal/cli", "cli", "TestS7PIB417WindowsBlockingLeafGuard"),
			subtest("internal/intentlock", "intentlock", "TestS7UnsupportedPlatformRows", "PIB-417"),
		}},
		{id: "PIB-418", kind: "G", status: "exact", targets: []s7FixtureTarget{
			target("internal/intentlock", "intentlock", "TestS7PIB418AuthoritySourceAndDocsRejectLegacyLockPrimitives"),
		}},
		{id: "PIB-419", kind: "C", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/workflow", "workflow", "TestS7PIB419ProviderSuccessFailureRetryHaveNoRawSink", "retry-then-success"),
			subtest("internal/workflow", "workflow", "TestS7PIB419ProviderSuccessFailureRetryHaveNoRawSink", "provider-failure"),
			subtest("internal/workflow", "workflow", "TestS7PIB419ProviderSuccessFailureRetryHaveNoRawSink", "success-allows-final-provider-content"),
			subtest("internal/workflow", "workflow", "TestS7PIB419ProviderSuccessFailureRetryHaveNoRawSink", "temp-root-write-bites-observation"),
		}},
		{id: "PIB-420", kind: "G", status: "exact", targets: []s7FixtureTarget{
			target("internal/workflow", "workflow", "TestS7PIB420PureGeneratorRetryConstructionGuardAndWrongInput"),
		}},
		{id: "PIB-421", kind: "C", status: "exact", targets: []s7FixtureTarget{
			target("internal/store", "store", "TestIntentArchiveNewSelectionAbsentRecaptureRejectsInsertionPIB421"),
			subtest("internal/store", "store", "TestRecoverPendingPurgeFreshClassificationAfterPreflightErrors", "blob-identity-change-remains-owned"),
		}},
		{id: "PIB-422", kind: "C", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/store", "store", "TestIntentArchivePurgeExternalWindowsAndRemovalResidual", "replacement-after-revalidation-is-disclosed-residual"),
		}},
		{id: "PIB-423", kind: "I", status: "exact", targets: []s7FixtureTarget{
			target("internal/store", "store", "TestIntentArchiveConfirmedPurgeClaimsEveryReferenceThenRemovesThenTombstones"),
		}},
		{id: "PIB-424", kind: "I", status: "exact", targets: []s7FixtureTarget{
			target("internal/store", "store", "TestIntentArchivePurgeSelectorValidationAndSharedGeneration"),
			subtest("internal/cli", "cli", "TestS7PIB424GenerationPurgeRefusesSharedHashWithoutMutation", "shared-refusal-json-human"),
			subtest("internal/cli", "cli", "TestS7PIB424GenerationPurgeRefusesSharedHashWithoutMutation", "blob-confirmed"),
			subtest("internal/cli", "cli", "TestS7PIB424GenerationPurgeRefusesSharedHashWithoutMutation", "all-preview"),
			subtest("internal/cli", "cli", "TestS7PIB424GenerationPurgeRefusesSharedHashWithoutMutation", "all-confirmed"),
		}},
		{id: "PIB-425", kind: "C", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/store", "store", "TestS7PIB425RehydrateExcludesPendingAndRefusesDanglingRetained", "multiple-tombstones-one-CAS"),
			subtest("internal/store", "store", "TestS7PIB425RehydrateExcludesPendingAndRefusesDanglingRetained", "pending-hash-routes-to-owner"),
			subtest("internal/store", "store", "TestS7PIB425RehydrateExcludesPendingAndRefusesDanglingRetained", "dangling-retained-still-refuses"),
		}},
		{id: "PIB-426", kind: "I", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/cli", "cli", "TestS7PIB426GitCleanCanRemoveUntrackedIntentArchive", "fd"),
			subtest("internal/cli", "cli", "TestS7PIB426GitCleanCanRemoveUntrackedIntentArchive", "xfd"),
		}},
		{id: "PIB-427", kind: "S", status: "exact", targets: []s7FixtureTarget{
			target("internal/gitutil", "gitutil", "TestS7PIB427GitStateExactArgvEnvironmentAndProcessCounts"),
		}},
		{id: "PIB-428", kind: "I", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/cli", "cli", "TestS7PIB428DanglingAndCorruptArchiveTruthAcrossSurfaces", "dangling"),
			subtest("internal/cli", "cli", "TestS7PIB428DanglingAndCorruptArchiveTruthAcrossSurfaces", "hash-wrong-corrupt"),
			subtest("internal/store", "store", "TestIntentArchiveNonRegularKindsShareClosedRoutes", "symlink"),
			subtest("internal/store", "store", "TestIntentArchiveNonRegularKindsShareClosedRoutes", "directory"),
			subtest("internal/store", "store", "TestIntentArchiveNonRegularKindsShareClosedRoutes", "fifo"),
			subtest("internal/store", "store", "TestIntentArchiveNonRegularKindsShareClosedRoutes", "device"),
			subtest("internal/store", "store", "TestIntentArchiveNonRegularKindsShareClosedRoutes", "other-non-regular"),
		}},
		{id: "PIB-429", kind: "C", status: "exact", targets: []s7FixtureTarget{
			target("internal/store", "store", "TestS7PIB429OrphanRevalidationProtectsNewlyReferencedBlob"),
		}},
		{id: "PIB-430", kind: "C", status: "exact", targets: []s7FixtureTarget{
			target("internal/store", "store", "TestS7PIB430ExternalReplacementAfterRevalidationIsDisclosedResidual"),
		}},
		{id: "PIB-431", kind: "G", status: "exact", targets: []s7FixtureTarget{
			target("internal/cli", "cli", "TestS7PIB431RefusalCatalogRendererParityAndWrongInput"),
		}},
		{id: "PIB-432", kind: "G", status: "exact", targets: []s7FixtureTarget{
			target("internal/intentlock", "intentlock", "TestS7PIB432PrepareOwnedAuthorityAndFrozenRescapParity"),
		}},
	}
}

func TestS7AOCoverageLedger(t *testing.T) {
	manifest := parseS7RowManifest(t)
	manifestByID := map[string]s7ManifestRow{}
	for _, row := range manifest {
		manifestByID[row.id] = row
	}
	ledger := s7AOCoverageLedger()
	if len(ledger) != 16 {
		t.Fatalf("AO ledger rows = %d, want 16", len(ledger))
	}
	if err := s7ValidateLedgerTargetPresence(ledger); err != nil {
		t.Fatal(err)
	}
	t.Run("empty-target-sensitivity", func(t *testing.T) {
		wrong := append([]s7AMLedgerRow(nil), ledger...)
		wrong[0].targets = nil
		if err := s7ValidateLedgerTargetPresence(wrong); err == nil {
			t.Fatal("AO ledger accepted an exact row with no targets")
		}
	})
	var resolutionErrors []string
	for index, row := range ledger {
		wantID := fmt.Sprintf("PIB-%03d", 433+index)
		normative := manifestByID[row.id]
		if row.id != wantID || row.status != "exact" ||
			normative.category != "AO" || normative.kind != row.kind {
			t.Fatalf("AO ledger row %d = %+v normative=%+v", index, row, normative)
		}
		if len(row.targets) == 0 {
			t.Fatalf("%s has no exact targets", row.id)
		}
		for _, target := range row.targets {
			if err := resolveS7FixtureTarget(t, target); err != nil {
				resolutionErrors = append(resolutionErrors, fmt.Sprintf("%s target %+v: %v", row.id, target, err))
			}
		}
	}
	if len(resolutionErrors) != 0 {
		t.Fatalf("AO exact-target resolution failed:\n%s", strings.Join(resolutionErrors, "\n"))
	}
}

func s7ValidateLedgerTargetPresence(ledger []s7AMLedgerRow) error {
	for _, row := range ledger {
		if row.status == "exact" && len(row.targets) == 0 {
			return fmt.Errorf("%s exact row has no targets", row.id)
		}
	}
	return nil
}

func s7AOCoverageLedger() []s7AMLedgerRow {
	target := func(dir, pkg, test string) s7FixtureTarget {
		return s7FixtureTarget{dir: dir, pkg: pkg, test: test}
	}
	subtest := func(dir, pkg, test, sub string) s7FixtureTarget {
		return s7FixtureTarget{dir: dir, pkg: pkg, test: test, subtest: sub}
	}
	return []s7AMLedgerRow{
		{id: "PIB-433", kind: "C", status: "exact", targets: []s7FixtureTarget{
			target("internal/intentlock", "intentlock", "TestS7PIB433RealHolderSurvivesGCUntilExplicitRelease"),
		}},
		{id: "PIB-434", kind: "G", status: "exact", targets: []s7FixtureTarget{
			target("internal/intentlock", "intentlock", "TestS7PIB434AuthorityLifetimeGuardAndWrongInput"),
		}},
		{id: "PIB-435", kind: "C", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/cli", "cli", "TestS7PIB435NestedRecoveryPublicationAndPurgeAcquireOnce", "publication"),
			subtest("internal/cli", "cli", "TestS7PIB435NestedRecoveryPublicationAndPurgeAcquireOnce", "recovery"),
			subtest("internal/cli", "cli", "TestS7PIB435NestedRecoveryPublicationAndPurgeAcquireOnce", "purge"),
		}},
		{id: "PIB-436", kind: "I", status: "exact", targets: []s7FixtureTarget{
			target("internal/cli", "cli", "TestS7PIB436RealRootRenameBeforeAndAfterPublicationBoundary"),
		}},
		{id: "PIB-437", kind: "G", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/cli", "cli", "TestS7PIB437AuthorityDurationDocsGuardAndWrongInput", "baseline"),
			subtest("internal/cli", "cli", "TestS7PIB437AuthorityDurationDocsGuardAndWrongInput", "command-total-bound"),
			subtest("internal/cli", "cli", "TestS7PIB437AuthorityDurationDocsGuardAndWrongInput", "authority-total-bound"),
			subtest("internal/cli", "cli", "TestS7PIB437AuthorityDurationDocsGuardAndWrongInput", "adr-filesystem-bound"),
		}},
		{id: "PIB-438", kind: "G", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/gitutil", "gitutil", "TestS7PIB438CentralGitExecutorGuardAndWrongInput", "baseline"),
			subtest("internal/gitutil", "gitutil", "TestS7PIB438CentralGitExecutorGuardAndWrongInput", "rev-parse-show-toplevel"),
			subtest("internal/gitutil", "gitutil", "TestS7PIB438CentralGitExecutorGuardAndWrongInput", "second-G1-probe"),
			subtest("internal/gitutil", "gitutil", "TestS7PIB438CentralGitExecutorGuardAndWrongInput", "absolute-lane"),
			subtest("internal/gitutil", "gitutil", "TestS7PIB438CentralGitExecutorGuardAndWrongInput", "duplicate-privacy-gate"),
		}},
		{id: "PIB-439", kind: "I", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/cli", "cli", "TestS7PIB439GitProcessTableUsesExactArgvAndPinnedEnvironment", "worktree-non-regenerate"),
			subtest("internal/cli", "cli", "TestS7PIB439GitProcessTableUsesExactArgvAndPinnedEnvironment", "worktree-regenerate"),
			subtest("internal/cli", "cli", "TestS7PIB439GitProcessTableUsesExactArgvAndPinnedEnvironment", "established-non-worktree"),
		}},
		{id: "PIB-440", kind: "C", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/cli", "cli", "TestS7PIB440DryRunModesAndRefusalsSkipAllMutatingEffects", "generate"),
			subtest("internal/cli", "cli", "TestS7PIB440DryRunModesAndRefusalsSkipAllMutatingEffects", "manual"),
			subtest("internal/cli", "cli", "TestS7PIB440DryRunModesAndRefusalsSkipAllMutatingEffects", "regenerate"),
			subtest("internal/cli", "cli", "TestS7PIB440DryRunModesAndRefusalsSkipAllMutatingEffects", "missing-feature"),
			subtest("internal/cli", "cli", "TestS7PIB440DryRunModesAndRefusalsSkipAllMutatingEffects", "malformed-status"),
			subtest("internal/cli", "cli", "TestS7PIB440DryRunModesAndRefusalsSkipAllMutatingEffects", "pending-journal-not-evaluated"),
		}},
		{id: "PIB-441", kind: "U", status: "exact", targets: []s7FixtureTarget{
			target("internal/intentlock", "intentlock", "TestS7PIB441LinuxRootFilesystemPolicyFixtures"),
			target("internal/intentlock", "intentlock", "TestS7PIB441DarwinRootFilesystemPolicyFixtures"),
		}},
		{id: "PIB-442", kind: "I", status: "exact", targets: []s7FixtureTarget{
			target("internal/intentlock", "intentlock", "TestRealProcessContentionAndExplicitRelease"),
		}},
		{id: "PIB-443", kind: "C", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/store", "store", "TestS7PIB443CrashMatrixRecoversEverySelectedHash", "all-before-removal-hash-0"),
			subtest("internal/store", "store", "TestS7PIB443CrashMatrixRecoversEverySelectedHash", "all-before-removal-hash-1"),
			subtest("internal/store", "store", "TestS7PIB443CrashMatrixRecoversEverySelectedHash", "all-after-removal-hash-0"),
			subtest("internal/store", "store", "TestS7PIB443CrashMatrixRecoversEverySelectedHash", "all-after-removal-hash-1"),
			subtest("internal/store", "store", "TestS7PIB443CrashMatrixRecoversEverySelectedHash", "all-after-tombstone-hash-0"),
			subtest("internal/store", "store", "TestS7PIB443CrashMatrixRecoversEverySelectedHash", "all-after-tombstone-hash-1"),
			subtest("internal/store", "store", "TestS7PIB443CrashMatrixRecoversEverySelectedHash", "generation-before-removal-hash-0"),
			subtest("internal/store", "store", "TestS7PIB443CrashMatrixRecoversEverySelectedHash", "generation-before-removal-hash-1"),
			subtest("internal/store", "store", "TestS7PIB443CrashMatrixRecoversEverySelectedHash", "generation-after-removal-hash-0"),
			subtest("internal/store", "store", "TestS7PIB443CrashMatrixRecoversEverySelectedHash", "generation-after-removal-hash-1"),
			subtest("internal/store", "store", "TestS7PIB443CrashMatrixRecoversEverySelectedHash", "generation-after-tombstone-hash-0"),
			subtest("internal/store", "store", "TestS7PIB443CrashMatrixRecoversEverySelectedHash", "generation-after-tombstone-hash-1"),
		}},
		{id: "PIB-444", kind: "I", status: "exact", targets: []s7FixtureTarget{
			target("internal/cli", "cli", "TestS7PIB444AvailableBytesDoNotBypassDanglingRepairRoute"),
		}},
		{id: "PIB-445", kind: "G", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/cli", "cli", "TestS7PIB445DoctorD9SourceGuardAndWrongInput", "baseline"),
			subtest("internal/cli", "cli", "TestS7PIB445DoctorD9SourceGuardAndWrongInput", "reachable-root-open"),
			subtest("internal/cli", "cli", "TestS7PIB445DoctorD9SourceGuardAndWrongInput", "live-authority-output-claim"),
			subtest("internal/cli", "cli", "TestS7PIB445DoctorD9SourceGuardAndWrongInput", "prechange-golden-byte"),
			target("internal/cli", "cli", "TestS7PIB445LiveMutatingPrepareIsUnchangedByD9"),
		}},
		{id: "PIB-446", kind: "G", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "baseline"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "legacy-retry-store"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "forbidden-legacy-retry-identity"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "forbidden-retry-store-field"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "forbidden-central-legacy-identities"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "forbidden-write-artifact-method"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "cross-file-report-history-sink"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "cross-file-stdout-writer"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "cross-file-fmt-file-writer"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "cross-file-create-temp-file-method"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "forbidden-identity-helper-parameter"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "forbidden-identity-helper-return"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "forbidden-interface-package-variable"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "forbidden-package-variable-alias-chain"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "forbidden-selector-method-dispatch"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "forbidden-init-assigned-package-aliases"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "forbidden-init-assigned-legacy-sink"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "forbidden-imported-interface-local-method"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "forbidden-imported-interface-writefile-method"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "forbidden-module-provider-implementation"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "forbidden-generic-writer-dispatch"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "fail-closed-unresolved-module-import"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "fail-closed-type-error"),
			subtest("internal/workflow", "workflow", "TestS7PIB446RawResponseStructuralGuardAndWrongInput", "cross-file-create-temp-alias-write-unlink"),
		}},
		{id: "PIB-447", kind: "G", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/store", "store", "TestS7PIB447ArchiveStateMachineSemanticParityAndWrongInput", "baseline"),
			subtest("internal/store", "store", "TestS7PIB447ArchiveStateMachineSemanticParityAndWrongInput", "prepare-recovers-pending"),
			subtest("internal/store", "store", "TestS7PIB447ArchiveStateMachineSemanticParityAndWrongInput", "pending-must-still-exist"),
			subtest("internal/store", "store", "TestS7PIB447ArchiveStateMachineSemanticParityAndWrongInput", "rehydration-dangling-repair"),
			subtest("internal/store", "store", "TestS7PIB447ArchiveStateMachineSemanticParityAndWrongInput", "prepare-finalizes-pending"),
			subtest("internal/store", "store", "TestS7PIB447ArchiveStateMachineSemanticParityAndWrongInput", "present-tombstone-divergence"),
			subtest("internal/store", "store", "TestS7PIB447ArchiveStateMachineSemanticParityAndWrongInput", "present-tombstone-always-orphan"),
			subtest("internal/store", "store", "TestS7PIB447ArchiveStateMachineSemanticParityAndWrongInput", "recovery-leaves-dangling"),
			subtest("internal/store", "store", "TestS7PIB447ArchiveStateMachineSemanticParityAndWrongInput", "selector-scoped-X11"),
		}},
		{id: "PIB-448", kind: "G", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/cli", "cli", "TestS7PIB448RefusalPrecedenceHelpAndReadOnlyListGuard", "baseline"),
			subtest("internal/cli", "cli", "TestS7PIB448RefusalPrecedenceHelpAndReadOnlyListGuard", "partial-exit-collapse"),
			subtest("internal/cli", "cli", "TestS7PIB448RefusalPrecedenceHelpAndReadOnlyListGuard", "distinct-index-code-collapse"),
			subtest("internal/cli", "cli", "TestS7PIB448RefusalPrecedenceHelpAndReadOnlyListGuard", "list-acquires-authority"),
			subtest("internal/cli", "cli", "TestS7PIB448RefusalPrecedenceHelpAndReadOnlyListGuard", "stale-section-anchor"),
			subtest("internal/cli", "cli", "TestS7PIB448RefusalPrecedenceHelpAndReadOnlyListGuard", "row-catalog-drift"),
		}},
	}
}

func s7AMCoverageLedger() []s7AMLedgerRow {
	target := func(dir, pkg, test string) s7FixtureTarget {
		return s7FixtureTarget{dir: dir, pkg: pkg, test: test}
	}
	subtest := func(dir, pkg, test, sub string) s7FixtureTarget {
		return s7FixtureTarget{dir: dir, pkg: pkg, test: test, subtest: sub}
	}
	return []s7AMLedgerRow{
		{id: "PIB-395", kind: "C", status: "exact", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7PIB395RealCLIContenderExitsThreeUnderHeldRoot")}},
		{id: "PIB-396", kind: "I", status: "exact", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7PIB396KilledHolderReleasesAuthorityForTerminalRecovery")}},
		{id: "PIB-397", kind: "I", status: "exact", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7PIB397RealDifferentSlugCLIProcessesSerialize")}},
		{id: "PIB-398", kind: "G", status: "exact", targets: []s7FixtureTarget{target("internal/intentlock", "intentlock", "TestS7PIB398DirectoryAuthorityConstructionGuardAndWrongInput")}},
		{id: "PIB-399", kind: "C", status: "exact", targets: []s7FixtureTarget{
			target("internal/cli", "cli", "TestS7PIB399ManualStatusUsesRootedDurableWriterOnly"),
			target("internal/intentpub", "intentpub", "TestProductionSourceHasNoPathWritersOrForbiddenJournalFields"),
		}},
		{id: "PIB-400", kind: "C", status: "exact", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7PIB400ManualStatusConcurrentEditHasNoRenameOrFeaturesRefresh")}},
		{id: "PIB-401", kind: "U", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/store", "store", "TestS7PIB401PurgedGenerationRetainsAndValidatesImmutableIdentity", "purged-identity-roundtrip"),
			subtest("internal/store", "store", "TestS7PIB401PurgedGenerationRetainsAndValidatesImmutableIdentity", "missing-content-digest"),
			subtest("internal/store", "store", "TestS7PIB401PurgedGenerationRetainsAndValidatesImmutableIdentity", "mismatched-content-digest"),
			subtest("internal/store", "store", "TestS7PIB401PurgedGenerationRetainsAndValidatesImmutableIdentity", "blob-purged-inconsistency"),
			subtest("internal/store", "store", "TestS7PIB401PurgedGenerationRetainsAndValidatesImmutableIdentity", "altered-immutable-path"),
		}},
		{id: "PIB-402", kind: "I", status: "exact", targets: []s7FixtureTarget{
			target("internal/store", "store", "TestS7PIB402TombstoneRehydrateAndPendingOwnershipConflict"),
			{dir: "internal/cli", pkg: "cli", test: "TestS7PIB402RegenerationRehydratesTombstonesAndRefusesPendingOwner", subtest: "tombstone-multireference-rehydrates"},
			{dir: "internal/cli", pkg: "cli", test: "TestS7PIB402RegenerationRehydratesTombstonesAndRefusesPendingOwner", subtest: "pending-owner-refuses-zero-write"},
			subtest("internal/cli", "cli", "TestS7Rev16PendingOwnerErratumGuardAndSensitivities", "baseline"),
			subtest("internal/cli", "cli", "TestS7Rev16PendingOwnerErratumGuardAndSensitivities", "old-pending-rehydration-token"),
			subtest("internal/cli", "cli", "TestS7Rev16PendingOwnerErratumGuardAndSensitivities", "fourth-matrix-row-change"),
			subtest("internal/cli", "cli", "TestS7Rev16PendingOwnerErratumGuardAndSensitivities", "omitted-amendment-ledger-row"),
			subtest("internal/cli", "cli", "TestS7Rev16PendingOwnerErratumGuardAndSensitivities", "section-1852-count-drift"),
			subtest("internal/cli", "cli", "TestS7Rev16PendingOwnerErratumGuardAndSensitivities", "adr-revision-mismatch"),
			subtest("internal/cli", "cli", "TestS7Rev16PendingOwnerErratumGuardAndSensitivities", "unrelated-prd-normative-edit"),
			subtest("internal/cli", "cli", "TestS7Rev16PendingOwnerErratumGuardAndSensitivities", "unrelated-adr-normative-edit"),
		}},
		{id: "PIB-403", kind: "I", status: "exact", targets: []s7FixtureTarget{
			target("internal/store", "store", "TestS7PIB403RepeatedPurgeRehydratePreservesIDsOrderAndReferenceCount"),
			target("internal/cli", "cli", "TestS7PIB403PendingOwnerRecoversBeforeLaterRegeneration"),
		}},
		{id: "PIB-404", kind: "C", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/store", "store", "TestS7PIB404RehydrateRedactionAndCP13ResidueClassification", "redaction-preserves-tombstone-and-absent-blob"),
			subtest("internal/store", "store", "TestS7PIB404RehydrateRedactionAndCP13ResidueClassification", "unreferenced-cp13-blob-routes-to-orphans"),
			subtest("internal/store", "store", "TestS7PIB404RehydrateRedactionAndCP13ResidueClassification", "live-cp13-blob-routes-to-confirmed-hash"),
			subtest("internal/cli", "cli", "TestS7PIB404CP13PrepareRefusalRendersExactRepairRoutes", "globally-unreferenced"),
			subtest("internal/cli", "cli", "TestS7PIB404CP13PrepareRefusalRendersExactRepairRoutes", "live-retained-reference"),
			{dir: "internal/cli", pkg: "cli", test: "TestFeatureIntentArchivePurgeAuthorityAndPendingJournal", subtest: "ownership-is-global-over-every-reference"},
		}},
		{id: "PIB-405", kind: "C", status: "exact", targets: []s7FixtureTarget{target("internal/store", "store", "TestS7PIB405ConcurrentIndexEditBeforeClaimRenamePreservesIndexAndBlobs")}},
		{id: "PIB-406", kind: "C", status: "exact", targets: []s7FixtureTarget{
			{dir: "internal/intentpub", pkg: "intentpub", test: "TestS7PIB406HeldRootRedirectsNeverEscapeAuthority", subtest: "stable-in-root-symlink-is-refused-without-following"},
			{dir: "internal/intentpub", pkg: "intentpub", test: "TestS7PIB406HeldRootRedirectsNeverEscapeAuthority", subtest: "outside-root-symlink-is-refused-without-following"},
			{dir: "internal/intentpub", pkg: "intentpub", test: "TestS7PIB406HeldRootRedirectsNeverEscapeAuthority", subtest: "ancestor-substitution-is-detected-after-rooted-open"},
		}},
		{id: "PIB-407", kind: "I", status: "exact", targets: []s7FixtureTarget{
			{dir: "internal/cli", pkg: "cli", test: "TestS7PIB407AnalyzeAndDefinePartialBundlesAreCleanForD9", subtest: "analyze"},
			{dir: "internal/cli", pkg: "cli", test: "TestS7PIB407AnalyzeAndDefinePartialBundlesAreCleanForD9", subtest: "define"},
		}},
		{id: "PIB-408", kind: "I", status: "exact", targets: []s7FixtureTarget{target("internal/cli", "cli", "TestS7PIB408LinkedWorktreeSubmoduleAndNonWorktreeGitGate")}},
		{id: "PIB-409", kind: "G", status: "exact", targets: []s7FixtureTarget{
			subtest("internal/intentlock", "intentlock", "TestS7PIB409MutationPlatformMatrixGuardAndWrongInput", "baseline"),
			subtest("internal/intentlock", "intentlock", "TestS7PIB409MutationPlatformMatrixGuardAndWrongInput", "windows-mutation-enabled"),
			subtest("internal/intentlock", "intentlock", "TestS7PIB409MutationPlatformMatrixGuardAndWrongInput", "plan9-check-enabled"),
			target("internal/cli", "cli", "TestS7PIB409WindowsBlockingSelectorGuard"),
			subtest("internal/cli", "cli", "TestS7BSDPlatformSourceGuardSensitivity", "baseline"),
			subtest("internal/cli", "cli", "TestS7BSDPlatformSourceGuardSensitivity", "missing-openbsd-build-lane"),
			subtest("internal/cli", "cli", "TestS7BSDPlatformSourceGuardSensitivity", "computed-ledger-leaf"),
			subtest("internal/cli", "cli", "TestS7BSDPlatformSourceGuardSensitivity", "predicate-bypassed"),
			subtest("internal/cli", "cli", "TestS7BSDPlatformSourceGuardSensitivity", "json-golden-omitted"),
			subtest("internal/cli", "cli", "TestS7BSDPlatformSourceGuardSensitivity", "workflow-freebsd-cross-compile-omitted"),
			subtest("internal/cli", "cli", "TestS7BSDPlatformSourceGuardSensitivity", "workflow-openbsd-cross-compile-omitted"),
			subtest("internal/cli", "cli", "TestS7BSDPlatformSourceGuardSensitivity", "workflow-netbsd-cross-compile-omitted"),
			subtest("internal/cli", "cli", "TestS7BSDPlatformSourceGuardSensitivity", "workflow-dragonfly-cross-compile-omitted"),
			{dir: "internal/cli", pkg: "cli", test: "TestS7WindowsPlatformRows", subtest: "PIB-409"},
			{dir: "internal/cli", pkg: "cli", test: "TestS7BSDPlatformRows", subtest: "PIB-409"},
			{dir: "internal/intentlock", pkg: "intentlock", test: "TestS7UnsupportedPlatformRows", subtest: "PIB-409"},
		}},
	}
}

func parseS7RowManifest(t *testing.T) []s7ManifestRow {
	t.Helper()
	path := filepath.Join(avpRepoRoot(t), "docs", "prds", "PRD-prepare-intent-bundle.md")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	heading := regexp.MustCompile(`^### 18\.(4[0-9]|5[01]) ([A-Z]{2}) —`)
	rowPattern := regexp.MustCompile(`^\| (PIB-[0-9]{3}) \| ([ICGUS]) \|`)
	category := ""
	rows := []s7ManifestRow{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if match := heading.FindStringSubmatch(line); match != nil {
			category = match[2]
			continue
		}
		if strings.HasPrefix(line, "### 18.52 ") {
			category = ""
		}
		if category == "" {
			continue
		}
		if match := rowPattern.FindStringSubmatch(line); match != nil {
			rows = append(rows, s7ManifestRow{id: match[1], kind: match[2], category: category})
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

func parseS7CategoryCountsFromSection1852(t *testing.T) map[string]int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(avpRepoRoot(t), "docs", "prds", "PRD-prepare-intent-bundle.md"))
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(data), "### 18.52 Counts, kinds and slice partition")
	end := strings.Index(string(data), "### 18.53 Sensitivity requirement")
	if start < 0 || end <= start {
		t.Fatal("PRD §18.52 was not found")
	}
	section := string(data[start:end])
	pattern := regexp.MustCompile(`\b(A[M-N]|A[O-X]) ([0-9]+)\b`)
	counts := map[string]int{}
	for _, match := range pattern.FindAllStringSubmatch(section, -1) {
		value, err := strconv.Atoi(match[2])
		if err != nil {
			t.Fatal(err)
		}
		counts[match[1]] = value
	}
	if len(counts) != 12 || !strings.Contains(section, "| S7 rev-3…rev-13 cross-cutting hardening | AM, AN, AO, AP, AQ, AR, AS, AT, AU, AV, AW, AX | 173 |") {
		t.Fatalf("§18.52 S7 partition was not parsed exactly: %v", counts)
	}
	return counts
}

func resolveS7FixtureTarget(t *testing.T, target s7FixtureTarget) error {
	t.Helper()
	dir := filepath.Join(avpRepoRoot(t), filepath.FromSlash(target.dir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	files := []*ast.File{}
	fileset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileset, filepath.Join(dir, entry.Name()), nil, 0)
		if err != nil {
			return err
		}
		files = append(files, file)
	}
	return resolveS7FixtureAST(files, target.pkg, target.test, target.subtest)
}

func resolveS7FixtureSource(source, pkg, test, subtest string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "fixture_test.go", source, 0)
	if err != nil {
		return err
	}
	return resolveS7FixtureAST([]*ast.File{file}, pkg, test, subtest)
}

func resolveS7FixtureAST(files []*ast.File, pkg, test, subtest string) error {
	functions := map[string][]*ast.FuncDecl{}
	testingAliases := map[string]bool{}
	for _, file := range files {
		if file.Name.Name != pkg {
			continue
		}
		for _, imported := range file.Imports {
			pathValue, err := strconv.Unquote(imported.Path.Value)
			if err != nil || pathValue != "testing" {
				continue
			}
			alias := "testing"
			if imported.Name != nil {
				alias = imported.Name.Name
			}
			if alias != "_" && alias != "." {
				testingAliases[alias] = true
			}
		}
		for _, declaration := range file.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok {
				functions[function.Name.Name] = append(functions[function.Name.Name], function)
			}
		}
	}
	candidates := functions[test]
	if len(candidates) != 1 {
		return fmt.Errorf("%s resolves to %d declarations", test, len(candidates))
	}
	function := candidates[0]
	receiver, err := s7TestingReceiver(function, testingAliases)
	if err != nil {
		return err
	}
	body := function.Body
	bodyReceiver := receiver
	if subtest != "" {
		for _, segment := range strings.Split(subtest, "/") {
			bodies := s7SelectSubtestBodies(
				body, bodyReceiver, functions, testingAliases, segment,
			)
			if len(bodies) != 1 {
				return fmt.Errorf("%s/%s resolves segment %q to %d runnable bodies", test, subtest, segment, len(bodies))
			}
			body = bodies[0].body
			bodyReceiver = bodies[0].receiver
		}
	}
	if err := s7AssertRunnableBody(body, bodyReceiver); err != nil {
		return fmt.Errorf("%s/%s: %w", test, subtest, err)
	}
	return nil
}

type s7SelectedBody struct {
	body              *ast.BlockStmt
	receiver          *ast.Ident
	registration      ast.Node
	innerRegistration ast.Node
}

func s7SelectSubtestBodies(
	body *ast.BlockStmt,
	receiver *ast.Ident,
	functions map[string][]*ast.FuncDecl,
	testingAliases map[string]bool,
	subtest string,
) []s7SelectedBody {
	var bodies []s7SelectedBody
	s7InspectRegistrationSyntax(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !s7RunCallOnReceiver(call, receiver) {
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, unquoteErr := strconv.Unquote(literal.Value)
		if unquoteErr == nil && value == subtest {
			bodies = append(bodies, s7CallbackBodies(call.Args[1], functions, testingAliases)...)
		}
		return true
	})
	s7InspectRegistrationSyntax(body, func(node ast.Node) bool {
		rangeStmt, ok := node.(*ast.RangeStmt)
		if !ok {
			return true
		}
		valueIdent, ok := rangeStmt.Value.(*ast.Ident)
		if !ok || valueIdent.Obj == nil ||
			!s7TableContainsCase(s7ResolveTableExpression(rangeStmt.X), subtest) {
			return true
		}
		s7InspectRegistrationSyntax(rangeStmt.Body, func(rangeNode ast.Node) bool {
			call, ok := rangeNode.(*ast.CallExpr)
			if !ok || !s7RunCallOnReceiver(call, receiver) {
				return true
			}
			selector, ok := call.Args[0].(*ast.SelectorExpr)
			if ok && selector.Sel.Name == "name" {
				base, baseOK := selector.X.(*ast.Ident)
				if !baseOK || base.Obj != valueIdent.Obj {
					return true
				}
			} else {
				direct, directOK := call.Args[0].(*ast.Ident)
				if !directOK || direct.Obj != valueIdent.Obj {
					return true
				}
			}
			if len(s7CallbackBodies(call.Args[1], functions, testingAliases)) == 0 {
				return true
			}
			bodies = append(bodies, s7CallbackBodies(call.Args[1], functions, testingAliases)...)
			return true
		})
		return true
	})
	return bodies
}

func s7InspectRegistrationSyntax(body *ast.BlockStmt, visit func(ast.Node) bool) {
	ast.Inspect(body, func(node ast.Node) bool {
		if literal, ok := node.(*ast.FuncLit); ok && literal != nil {
			return false
		}
		return visit(node)
	})
}

func s7RunCallOnReceiver(call *ast.CallExpr, receiver *ast.Ident) bool {
	if len(call.Args) != 2 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Run" {
		return false
	}
	actual, ok := selector.X.(*ast.Ident)
	return ok && actual.Obj == receiver.Obj
}

func s7CallbackBodies(
	callback ast.Expr,
	functions map[string][]*ast.FuncDecl,
	testingAliases map[string]bool,
) []s7SelectedBody {
	var bodies []s7SelectedBody
	switch typed := callback.(type) {
	case *ast.FuncLit:
		receiver, err := s7TestingFuncLiteralReceiver(typed, testingAliases)
		if err == nil {
			bodies = append(bodies, s7SelectedBody{body: typed.Body, receiver: receiver})
		}
	case *ast.Ident:
		for _, named := range functions[typed.Name] {
			receiver, err := s7TestingReceiver(named, testingAliases)
			if err == nil {
				bodies = append(bodies, s7SelectedBody{body: named.Body, receiver: receiver})
			}
		}
	}
	return bodies
}

const (
	s7PathUnregistered uint8 = 1 << iota
	s7PathRegistered
)

type s7RegistrationFlow struct {
	falls uint8
	exits uint8
}

func s7RegistrationDominatesParent(
	body *ast.BlockStmt,
	receiver *ast.Ident,
	selected s7SelectedBody,
) bool {
	if selected.registration == nil {
		return false
	}
	if selected.innerRegistration != nil {
		rangeStatement, ok := selected.registration.(*ast.RangeStmt)
		if !ok {
			return false
		}
		inner := s7AnalyzeRegistrationBlock(
			rangeStatement.Body,
			s7PathUnregistered,
			selected.innerRegistration,
			receiver,
		)
		innerPaths := inner.falls | inner.exits
		if innerPaths == 0 || innerPaths&s7PathUnregistered != 0 {
			return false
		}
	}
	flow := s7AnalyzeRegistrationBlock(
		body,
		s7PathUnregistered,
		selected.registration,
		receiver,
	)
	paths := flow.falls | flow.exits
	return paths != 0 && paths&s7PathUnregistered == 0 &&
		paths&s7PathRegistered != 0
}

func s7AnalyzeRegistrationBlock(
	body *ast.BlockStmt,
	paths uint8,
	target ast.Node,
	receiver *ast.Ident,
) s7RegistrationFlow {
	flow := s7RegistrationFlow{falls: paths}
	if body == nil {
		return flow
	}
	for _, statement := range body.List {
		if flow.falls == 0 {
			break
		}
		next := s7AnalyzeRegistrationStatement(
			statement,
			flow.falls,
			target,
			receiver,
		)
		flow.falls = next.falls
		flow.exits |= next.exits
	}
	return flow
}

func s7AnalyzeRegistrationStatement(
	statement ast.Stmt,
	paths uint8,
	target ast.Node,
	receiver *ast.Ident,
) s7RegistrationFlow {
	if paths == 0 {
		return s7RegistrationFlow{}
	}
	mark := func(node ast.Node) uint8 {
		return s7MarkRegistrationPaths(paths, node, target)
	}
	switch typed := statement.(type) {
	case *ast.BlockStmt:
		return s7AnalyzeRegistrationBlock(typed, paths, target, receiver)
	case *ast.ExprStmt:
		paths = mark(typed.X)
		if s7ExpressionTerminatesTest(typed.X, receiver) {
			return s7RegistrationFlow{exits: paths}
		}
		return s7RegistrationFlow{falls: paths}
	case *ast.ReturnStmt:
		for _, result := range typed.Results {
			paths = s7MarkRegistrationPaths(paths, result, target)
		}
		return s7RegistrationFlow{exits: paths}
	case *ast.IfStmt:
		flow := s7RegistrationFlow{falls: paths}
		if typed.Init != nil {
			flow = s7AnalyzeRegistrationStatement(
				typed.Init, flow.falls, target, receiver,
			)
			if flow.falls == 0 {
				return flow
			}
		}
		flow.falls = s7MarkRegistrationPaths(flow.falls, typed.Cond, target)
		if value, known := s7StaticBool(typed.Cond); known {
			var branch s7RegistrationFlow
			if value {
				branch = s7AnalyzeRegistrationBlock(
					typed.Body, flow.falls, target, receiver,
				)
			} else if typed.Else != nil {
				branch = s7AnalyzeRegistrationElse(
					typed.Else, flow.falls, target, receiver,
				)
			} else {
				branch.falls = flow.falls
			}
			branch.exits |= flow.exits
			return branch
		}
		thenFlow := s7AnalyzeRegistrationBlock(
			typed.Body, flow.falls, target, receiver,
		)
		elseFlow := s7RegistrationFlow{falls: flow.falls}
		if typed.Else != nil {
			elseFlow = s7AnalyzeRegistrationElse(
				typed.Else, flow.falls, target, receiver,
			)
		}
		return s7RegistrationFlow{
			falls: thenFlow.falls | elseFlow.falls,
			exits: flow.exits | thenFlow.exits | elseFlow.exits,
		}
	case *ast.ForStmt:
		flow := s7RegistrationFlow{falls: paths}
		if typed.Init != nil {
			flow = s7AnalyzeRegistrationStatement(
				typed.Init, flow.falls, target, receiver,
			)
		}
		if typed.Cond != nil {
			flow.falls = s7MarkRegistrationPaths(
				flow.falls, typed.Cond, target,
			)
			if value, known := s7StaticBool(typed.Cond); known && !value {
				return flow
			}
		}
		bodyFlow := s7AnalyzeRegistrationBlock(
			typed.Body, flow.falls, target, receiver,
		)
		return s7RegistrationFlow{
			falls: flow.falls | bodyFlow.falls,
			exits: flow.exits | bodyFlow.exits,
		}
	case *ast.RangeStmt:
		paths = s7MarkRegistrationPaths(paths, typed.X, target)
		if typed == target {
			paths = s7RegisterPaths(paths)
		}
		bodyFlow := s7AnalyzeRegistrationBlock(
			typed.Body, paths, target, receiver,
		)
		return s7RegistrationFlow{
			falls: paths | bodyFlow.falls,
			exits: bodyFlow.exits,
		}
	case *ast.AssignStmt:
		for _, expression := range typed.Rhs {
			paths = s7MarkRegistrationPaths(paths, expression, target)
		}
		return s7RegistrationFlow{falls: paths}
	case *ast.DeclStmt:
		paths = s7MarkRegistrationPaths(paths, typed.Decl, target)
		return s7RegistrationFlow{falls: paths}
	case *ast.LabeledStmt:
		return s7AnalyzeRegistrationStatement(typed.Stmt, paths, target, receiver)
	case *ast.BranchStmt:
		return s7RegistrationFlow{exits: paths}
	case *ast.SwitchStmt:
		flow := s7RegistrationFlow{falls: paths}
		if typed.Init != nil {
			flow = s7AnalyzeRegistrationStatement(
				typed.Init, flow.falls, target, receiver,
			)
		}
		if typed.Tag != nil {
			flow.falls = s7MarkRegistrationPaths(
				flow.falls, typed.Tag, target,
			)
		}
		return s7AnalyzeRegistrationCases(
			typed.Body.List, flow, target, receiver,
		)
	case *ast.TypeSwitchStmt:
		flow := s7RegistrationFlow{falls: paths}
		if typed.Init != nil {
			flow = s7AnalyzeRegistrationStatement(
				typed.Init, flow.falls, target, receiver,
			)
		}
		if typed.Assign != nil {
			assigned := s7AnalyzeRegistrationStatement(
				typed.Assign, flow.falls, target, receiver,
			)
			flow.falls = assigned.falls
			flow.exits |= assigned.exits
		}
		return s7AnalyzeRegistrationCases(
			typed.Body.List, flow, target, receiver,
		)
	case *ast.SelectStmt:
		var combined s7RegistrationFlow
		hasDefault := false
		for _, statement := range typed.Body.List {
			clause, ok := statement.(*ast.CommClause)
			if !ok {
				continue
			}
			clausePaths := paths
			if clause.Comm == nil {
				hasDefault = true
			} else {
				comm := s7AnalyzeRegistrationStatement(
					clause.Comm, clausePaths, target, receiver,
				)
				clausePaths = comm.falls
				combined.exits |= comm.exits
			}
			clauseFlow := s7AnalyzeRegistrationBlock(
				&ast.BlockStmt{List: clause.Body},
				clausePaths,
				target,
				receiver,
			)
			combined.falls |= clauseFlow.falls
			combined.exits |= clauseFlow.exits
		}
		if !hasDefault {
			combined.exits |= paths
		}
		return combined
	case *ast.DeferStmt, *ast.GoStmt:
		return s7RegistrationFlow{falls: paths}
	default:
		paths = mark(statement)
		return s7RegistrationFlow{falls: paths}
	}
}

func s7AnalyzeRegistrationElse(
	statement ast.Stmt,
	paths uint8,
	target ast.Node,
	receiver *ast.Ident,
) s7RegistrationFlow {
	if block, ok := statement.(*ast.BlockStmt); ok {
		return s7AnalyzeRegistrationBlock(block, paths, target, receiver)
	}
	return s7AnalyzeRegistrationStatement(statement, paths, target, receiver)
}

func s7AnalyzeRegistrationCases(
	statements []ast.Stmt,
	initial s7RegistrationFlow,
	target ast.Node,
	receiver *ast.Ident,
) s7RegistrationFlow {
	combined := s7RegistrationFlow{exits: initial.exits}
	hasDefault := false
	for _, statement := range statements {
		clause, ok := statement.(*ast.CaseClause)
		if !ok {
			continue
		}
		if len(clause.List) == 0 {
			hasDefault = true
		}
		paths := initial.falls
		for _, expression := range clause.List {
			paths = s7MarkRegistrationPaths(paths, expression, target)
		}
		flow := s7AnalyzeRegistrationBlock(
			&ast.BlockStmt{List: clause.Body}, paths, target, receiver,
		)
		combined.falls |= flow.falls
		combined.exits |= flow.exits
	}
	if !hasDefault {
		combined.falls |= initial.falls
	}
	return combined
}

func s7NodeContainsRegistration(node ast.Node, target ast.Node) bool {
	found := false
	ast.Inspect(node, func(current ast.Node) bool {
		if current == target {
			found = true
			return false
		}
		if _, nested := current.(*ast.FuncLit); nested {
			return false
		}
		return !found
	})
	return found
}

func s7MarkRegistrationPaths(paths uint8, node ast.Node, target ast.Node) uint8 {
	if !s7NodeContainsRegistration(node, target) {
		return paths
	}
	return s7RegisterPaths(paths)
}

func s7RegisterPaths(paths uint8) uint8 {
	if paths&s7PathUnregistered != 0 {
		paths &^= s7PathUnregistered
		paths |= s7PathRegistered
	}
	return paths
}

func s7ExpressionTerminatesTest(expression ast.Expr, receiver *ast.Ident) bool {
	call, ok := expression.(*ast.CallExpr)
	if !ok {
		return false
	}
	if identifier, ok := call.Fun.(*ast.Ident); ok {
		return identifier.Obj == nil && identifier.Name == "panic"
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	actual, ok := selector.X.(*ast.Ident)
	if !ok || actual.Obj != receiver.Obj {
		return false
	}
	switch selector.Sel.Name {
	case "Fatal", "Fatalf", "FailNow", "Skip", "Skipf", "SkipNow":
		return true
	}
	return false
}

func s7ResolveTableExpression(expression ast.Expr) ast.Expr {
	seen := map[*ast.Object]bool{}
	for {
		identifier, ok := expression.(*ast.Ident)
		if !ok || identifier.Obj == nil || seen[identifier.Obj] {
			return expression
		}
		seen[identifier.Obj] = true
		switch declaration := identifier.Obj.Decl.(type) {
		case *ast.AssignStmt:
			for index, left := range declaration.Lhs {
				bound, ok := left.(*ast.Ident)
				if !ok || bound.Obj != identifier.Obj {
					continue
				}
				switch {
				case len(declaration.Rhs) == len(declaration.Lhs):
					expression = declaration.Rhs[index]
				case len(declaration.Rhs) == 1:
					expression = declaration.Rhs[0]
				default:
					return expression
				}
				goto resolved
			}
			return expression
		case *ast.ValueSpec:
			for index, bound := range declaration.Names {
				if bound.Obj != identifier.Obj {
					continue
				}
				switch {
				case len(declaration.Values) == len(declaration.Names):
					expression = declaration.Values[index]
				case len(declaration.Values) == 1:
					expression = declaration.Values[0]
				default:
					return expression
				}
				goto resolved
			}
			return expression
		default:
			return expression
		}
	resolved:
	}
}

func s7TableContainsCase(expression ast.Expr, subtest string) bool {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return false
	}
	matches := 0
	for _, element := range literal.Elts {
		if value, ok := element.(*ast.BasicLit); ok && value.Kind == token.STRING {
			decoded, err := strconv.Unquote(value.Value)
			if err == nil && decoded == subtest {
				matches++
			}
			continue
		}
		row, ok := element.(*ast.CompositeLit)
		if !ok {
			continue
		}
		for _, field := range row.Elts {
			kv, ok := field.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, _ := kv.Key.(*ast.Ident)
			value, _ := kv.Value.(*ast.BasicLit)
			if key == nil || key.Name != "name" || value == nil || value.Kind != token.STRING {
				continue
			}
			decoded, err := strconv.Unquote(value.Value)
			if err == nil && decoded == subtest {
				matches++
			}
		}
	}
	return matches == 1
}

func s7TestingReceiver(
	function *ast.FuncDecl,
	testingAliases map[string]bool,
) (*ast.Ident, error) {
	if function.Recv != nil || function.Body == nil || function.Type.Results != nil ||
		function.Type.Params == nil || len(function.Type.Params.List) != 1 {
		return nil, fmt.Errorf("%s is not a test-shaped function", function.Name.Name)
	}
	parameter := function.Type.Params.List[0]
	if len(parameter.Names) != 1 {
		return nil, fmt.Errorf("%s does not bind one testing receiver", function.Name.Name)
	}
	pointer, ok := parameter.Type.(*ast.StarExpr)
	if !ok {
		return nil, fmt.Errorf("%s parameter is not *testing.T", function.Name.Name)
	}
	selector, ok := pointer.X.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "T" {
		return nil, fmt.Errorf("%s parameter is not *testing.T", function.Name.Name)
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || !testingAliases[pkg.Name] {
		return nil, fmt.Errorf("%s uses the wrong testing package", function.Name.Name)
	}
	if parameter.Names[0].Obj == nil {
		return nil, fmt.Errorf("%s testing receiver has no exact object", function.Name.Name)
	}
	return parameter.Names[0], nil
}

func s7TestingFuncLiteralReceiver(
	function *ast.FuncLit,
	testingAliases map[string]bool,
) (*ast.Ident, error) {
	synthetic := &ast.FuncDecl{
		Name: ast.NewIdent("literal"),
		Type: function.Type,
		Body: function.Body,
	}
	return s7TestingReceiver(synthetic, testingAliases)
}

func s7InspectLexicalBody(body *ast.BlockStmt, visit func(ast.Node) bool) {
	if body == nil {
		return
	}
	ast.Inspect(body, func(node ast.Node) bool {
		if literal, ok := node.(*ast.FuncLit); ok && literal.Body != body {
			return false
		}
		return visit(node)
	})
}

func s7AssertRunnableBody(body *ast.BlockStmt, receiver *ast.Ident) error {
	if body == nil || len(body.List) == 0 {
		return fmt.Errorf("empty body")
	}
	assertions, skips, subtests := 0, 0, 0
	observe := func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		actual, ok := selector.X.(*ast.Ident)
		if !ok || actual.Obj != receiver.Obj {
			return true
		}
		switch selector.Sel.Name {
		case "Run":
			subtests++
		case "Skip", "Skipf", "SkipNow":
			skips++
		case "Fatal", "Fatalf", "Error", "Errorf", "Fail", "FailNow":
			assertions++
		}
		return true
	}
	s7InspectReachableBody(body, observe)
	if skips != 0 {
		return fmt.Errorf("selected body can skip")
	}
	if subtests != 0 {
		return fmt.Errorf("selected body is an aggregate wrapper with %d nested subtest(s)", subtests)
	}
	if assertions == 0 {
		return fmt.Errorf("selected body has no reachable binding assertion")
	}
	return nil
}

func s7InspectReachableBody(body *ast.BlockStmt, visit func(ast.Node) bool) bool {
	if body == nil {
		return true
	}
	fallsThrough := true
	for _, statement := range body.List {
		if !fallsThrough {
			break
		}
		fallsThrough = s7InspectReachableStatement(statement, visit)
	}
	return fallsThrough
}

func s7InspectReachableStatement(statement ast.Stmt, visit func(ast.Node) bool) bool {
	if !visit(statement) {
		return true
	}
	inspectExpression := func(expression ast.Expr) {
		ast.Inspect(expression, func(node ast.Node) bool {
			if _, nested := node.(*ast.FuncLit); nested {
				return false
			}
			return visit(node)
		})
	}
	switch typed := statement.(type) {
	case *ast.BlockStmt:
		return s7InspectReachableBody(typed, visit)
	case *ast.ExprStmt:
		inspectExpression(typed.X)
		if call, ok := typed.X.(*ast.CallExpr); ok {
			if identifier, ok := call.Fun.(*ast.Ident); ok &&
				identifier.Obj == nil && identifier.Name == "panic" {
				return false
			}
		}
		return true
	case *ast.ReturnStmt:
		for _, result := range typed.Results {
			inspectExpression(result)
		}
		return false
	case *ast.IfStmt:
		if typed.Init != nil {
			s7InspectReachableStatement(typed.Init, visit)
		}
		inspectExpression(typed.Cond)
		if value, known := s7StaticBool(typed.Cond); known {
			if value {
				return s7InspectReachableBody(typed.Body, visit)
			}
			if typed.Else == nil {
				return true
			}
			return s7InspectReachableElse(typed.Else, visit)
		}
		bodyFalls := s7InspectReachableBody(typed.Body, visit)
		elseFalls := true
		if typed.Else != nil {
			elseFalls = s7InspectReachableElse(typed.Else, visit)
		}
		return bodyFalls || elseFalls
	case *ast.ForStmt:
		if typed.Init != nil {
			s7InspectReachableStatement(typed.Init, visit)
		}
		if typed.Cond != nil {
			inspectExpression(typed.Cond)
			if value, known := s7StaticBool(typed.Cond); known && !value {
				return true
			}
		}
		s7InspectReachableBody(typed.Body, visit)
		if typed.Post != nil {
			s7InspectReachableStatement(typed.Post, visit)
		}
		return true
	case *ast.RangeStmt:
		inspectExpression(typed.X)
		s7InspectReachableBody(typed.Body, visit)
		return true
	case *ast.DeferStmt:
		inspectExpression(typed.Call)
		return true
	case *ast.GoStmt:
		inspectExpression(typed.Call)
		return true
	case *ast.AssignStmt:
		for _, expression := range typed.Rhs {
			inspectExpression(expression)
		}
		return true
	case *ast.DeclStmt:
		ast.Inspect(typed.Decl, func(node ast.Node) bool {
			if _, nested := node.(*ast.FuncLit); nested {
				return false
			}
			return visit(node)
		})
		return true
	case *ast.LabeledStmt:
		return s7InspectReachableStatement(typed.Stmt, visit)
	case *ast.BranchStmt:
		return false
	case *ast.SwitchStmt:
		if typed.Init != nil {
			s7InspectReachableStatement(typed.Init, visit)
		}
		if typed.Tag != nil {
			inspectExpression(typed.Tag)
		}
		for _, statement := range typed.Body.List {
			clause, ok := statement.(*ast.CaseClause)
			if !ok {
				continue
			}
			for _, expression := range clause.List {
				inspectExpression(expression)
			}
			s7InspectReachableBody(&ast.BlockStmt{List: clause.Body}, visit)
		}
		return true
	case *ast.TypeSwitchStmt:
		if typed.Init != nil {
			s7InspectReachableStatement(typed.Init, visit)
		}
		if typed.Assign != nil {
			s7InspectReachableStatement(typed.Assign, visit)
		}
		for _, statement := range typed.Body.List {
			if clause, ok := statement.(*ast.CaseClause); ok {
				s7InspectReachableBody(&ast.BlockStmt{List: clause.Body}, visit)
			}
		}
		return true
	case *ast.SelectStmt:
		for _, statement := range typed.Body.List {
			if clause, ok := statement.(*ast.CommClause); ok {
				if clause.Comm != nil {
					s7InspectReachableStatement(clause.Comm, visit)
				}
				s7InspectReachableBody(&ast.BlockStmt{List: clause.Body}, visit)
			}
		}
		return true
	default:
		ast.Inspect(statement, func(node ast.Node) bool {
			if _, nested := node.(*ast.FuncLit); nested {
				return false
			}
			return visit(node)
		})
		return true
	}
}

func s7InspectReachableElse(statement ast.Stmt, visit func(ast.Node) bool) bool {
	switch typed := statement.(type) {
	case *ast.BlockStmt:
		return s7InspectReachableBody(typed, visit)
	case *ast.IfStmt:
		return s7InspectReachableStatement(typed, visit)
	default:
		return s7InspectReachableStatement(statement, visit)
	}
}

func s7StaticBool(expression ast.Expr) (bool, bool) {
	switch typed := expression.(type) {
	case *ast.Ident:
		if typed.Obj == nil && typed.Name == "true" {
			return true, true
		}
		if typed.Obj == nil && typed.Name == "false" {
			return false, true
		}
	case *ast.ParenExpr:
		return s7StaticBool(typed.X)
	case *ast.UnaryExpr:
		if typed.Op == token.NOT {
			value, known := s7StaticBool(typed.X)
			return !value, known
		}
	case *ast.BinaryExpr:
		left, leftKnown := s7StaticBool(typed.X)
		right, rightKnown := s7StaticBool(typed.Y)
		switch typed.Op {
		case token.LAND:
			if leftKnown && !left {
				return false, true
			}
			if rightKnown && !right {
				return false, true
			}
			if leftKnown && rightKnown {
				return left && right, true
			}
		case token.LOR:
			if leftKnown && left {
				return true, true
			}
			if rightKnown && right {
				return true, true
			}
			if leftKnown && rightKnown {
				return left || right, true
			}
		case token.EQL:
			if leftKnown && rightKnown {
				return left == right, true
			}
		case token.NEQ:
			if leftKnown && rightKnown {
				return left != right, true
			}
		}
	}
	return false, false
}

func sortedS7Keys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
