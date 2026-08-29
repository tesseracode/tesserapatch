//go:build (linux && !android) || (darwin && !ios)

package cli

// Bite proofs for the GH #23 aggregate acceptance ledger.
//
// Two families live here:
//
//   - TestPIBLedgerResolverSensitivity drives the resolver against hand-built
//     sources that are valid Go and textually mention the wanted identity, and
//     requires every one of them to be REJECTED. A byte scan or a name match
//     passes all of them; only body-sensitive resolution fails them.
//   - TestPIBAggregateLedgerMetaMutations mutates the ledger and the sensitivity
//     registry themselves and requires the validators to notice. A ledger that
//     cannot detect a removed, duplicated, re-kinded, re-sliced or downgraded row
//     is not a gate.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestPIBLedgerResolverSensitivity(t *testing.T) {
	valid := `package fixture
import "testing"
func TestCase(t *testing.T) {
	t.Run("row", func(t *testing.T) { t.Fatal("binding") })
}`
	if err := pibResolveFixtureSource(valid, "fixture", "TestCase", "row"); err != nil {
		t.Fatalf("resolver rejected a valid literal subtest: %v", err)
	}
	validHelper := `package fixture
import "testing"
func assertSomething(t *testing.T, value int) { if value == 0 { t.Fatalf("bad") } }
func TestCase(t *testing.T) { assertSomething(t, 1) }`
	if err := pibResolveFixtureSource(validHelper, "fixture", "TestCase", ""); err != nil {
		t.Fatalf("resolver rejected a real helper-delegated binding: %v", err)
	}
	validKeyed := `package fixture
import "testing"
var rows = map[string][]string{"PIB-198": {"a"}, "PIB-199": {"b"}}
func TestCase(t *testing.T) {
	for row, selectors := range rows {
		row, selectors := row, selectors
		t.Run(row, func(t *testing.T) {
			if len(selectors) == 0 { t.Fatalf("%s selected nothing", row) }
		})
	}
}`
	if err := pibResolveFixtureSource(validKeyed, "fixture", "TestCase", "PIB-198"); err != nil {
		t.Fatalf("resolver rejected a map-keyed literal subtest: %v", err)
	}
	validOwnAssertionWithChildren := `package fixture
import "testing"
func TestCase(t *testing.T) {
	t.Run("outer", func(t *testing.T) {
		if !vetPasses() { t.Fatalf("vet failed") }
		for _, name := range []string{"a", "b"} {
			t.Run(name, func(t *testing.T) { t.Fatal("child") })
		}
	})
}
func vetPasses() bool { return true }`
	if err := pibResolveFixtureSource(
		validOwnAssertionWithChildren, "fixture", "TestCase", "outer",
	); err != nil {
		t.Fatalf("resolver rejected a body that asserts and also registers children: %v", err)
	}

	fixtures := []struct {
		name    string
		source  string
		pkg     string
		test    string
		subtest string
	}{
		{
			name:   "comment",
			source: "package fixture\n// func TestCase(t *testing.T) { t.Fatal(\"binding\") }\n",
			pkg:    "fixture", test: "TestCase",
		},
		{
			name:   "string",
			source: "package fixture\nconst source = `func TestCase(t *testing.T) { t.Fatal(\"binding\") }`\n",
			pkg:    "fixture", test: "TestCase",
		},
		{
			name: "wrong-package",
			source: `package other
import "testing"
func TestCase(t *testing.T) { t.Fatal("binding") }`,
			pkg: "fixture", test: "TestCase",
		},
		{
			name: "wrong-testing-import",
			source: `package fixture
import testing "example.invalid/testing"
func TestCase(t *testing.T) { t.Fatal("binding") }`,
			pkg: "fixture", test: "TestCase",
		},
		{
			name: "aliased-testing-import-not-used",
			source: `package fixture
import unit "testing"
type shim struct{}
func (shim) Fatal(...any) {}
func TestCase(t *shim) { t.Fatal("binding") }
var _ = unit.Short`,
			pkg: "fixture", test: "TestCase",
		},
		{
			name: "wrong-signature",
			source: `package fixture
import "testing"
func TestCase(name string) { _ = name }
var _ = testing.Short`,
			pkg: "fixture", test: "TestCase",
		},
		{
			name: "dead-after-return",
			source: `package fixture
import "testing"
func TestCase(t *testing.T) {
	return
	t.Fatal("unreachable binding")
}`,
			pkg: "fixture", test: "TestCase",
		},
		{
			name: "dead-after-panic",
			source: `package fixture
import "testing"
func TestCase(t *testing.T) {
	panic("stop")
	t.Fatal("unreachable binding")
}`,
			pkg: "fixture", test: "TestCase",
		},
		{
			name: "dead-after-fatal-terminator",
			source: `package fixture
import "testing"
func TestCase(t *testing.T) {
	t.Run("row", func(t *testing.T) {
		return
		t.Fatal("unreachable binding")
	})
}`,
			pkg: "fixture", test: "TestCase", subtest: "row",
		},
		{
			name: "short-circuit-unreachable",
			source: `package fixture
import "testing"
func TestCase(t *testing.T) {
	if false {
		t.Fatal("unreachable binding")
	}
}`,
			pkg: "fixture", test: "TestCase",
		},
		{
			name: "infinite-loop-before-assertion",
			source: `package fixture
import "testing"
func TestCase(t *testing.T) {
	for {
		spin()
	}
	t.Fatal("unreachable binding")
}
func spin() {}`,
			pkg: "fixture", test: "TestCase",
		},
		{
			name: "parent-only-aggregate-wrapper",
			source: `package fixture
import "testing"
func TestCase(t *testing.T) {
	t.Run("a", func(t *testing.T) { t.Fatal("child binds") })
	t.Run("b", func(t *testing.T) { t.Fatal("child binds") })
}`,
			pkg: "fixture", test: "TestCase",
		},
		{
			name: "nested-function-assertion",
			source: `package fixture
import "testing"
func TestCase(t *testing.T) {
	run := func(t *testing.T) { t.Fatal("binding") }
	_ = run
}`,
			pkg: "fixture", test: "TestCase",
		},
		{
			name: "shadow-receiver",
			source: `package fixture
import "testing"
type shim struct{}
func (shim) Fatal(...any) {}
func TestCase(other *testing.T) {
	t := shim{}
	t.Fatal("binding")
	_ = other
}`,
			pkg: "fixture", test: "TestCase",
		},
		{
			name: "subtest-shadow-receiver",
			source: `package fixture
import "testing"
func TestCase(other *testing.T) {
	t := other
	t.Run("row", func(t *testing.T) { t.Fatal("binding") })
}`,
			pkg: "fixture", test: "TestCase", subtest: "row",
		},
		{
			name: "nonliteral-subtest",
			source: `package fixture
import "testing"
func TestCase(t *testing.T) {
	name := "row"
	t.Run(name, func(t *testing.T) { t.Fatal("binding") })
}`,
			pkg: "fixture", test: "TestCase", subtest: "row",
		},
		{
			name: "missing-subtest-with-surviving-siblings",
			source: `package fixture
import "testing"
func TestCase(t *testing.T) {
	t.Run("sibling-a", func(t *testing.T) { t.Fatal("binding") })
	t.Run("sibling-b", func(t *testing.T) { t.Fatal("binding") })
}`,
			pkg: "fixture", test: "TestCase", subtest: "row",
		},
		{
			name: "skipping-body",
			source: `package fixture
import "testing"
func TestCase(t *testing.T) {
	if !supported() { t.Skip("unsupported") }
	t.Fatal("binding")
}
func supported() bool { return true }`,
			pkg: "fixture", test: "TestCase",
		},
		{
			name: "helper-that-never-asserts",
			source: `package fixture
import "testing"
func observe(t *testing.T, value int) { t.Helper(); _ = value }
func TestCase(t *testing.T) { observe(t, 1) }`,
			pkg: "fixture", test: "TestCase",
		},
		{
			name: "helper-not-given-the-receiver",
			source: `package fixture
import "testing"
func assertSomething(t *testing.T, value int) { if value == 0 { t.Fatalf("bad") } }
func TestCase(t *testing.T) {
	other := &testing.T{}
	assertSomething(other, 0)
}`,
			pkg: "fixture", test: "TestCase",
		},
		{
			name: "duplicate-declaration",
			source: `package fixture
import "testing"
func TestCase(t *testing.T) { t.Fatal("binding") }
func TestCase(t *testing.T) { t.Fatal("binding") }`,
			pkg: "fixture", test: "TestCase",
		},
		{
			name: "duplicate-map-key-subtest",
			source: `package fixture
import "testing"
var rows = map[string][]string{"row": {"a"}}
var more = map[string][]string{"row": {"b"}}
func TestCase(t *testing.T) {
	for row := range rows {
		t.Run(row, func(t *testing.T) { t.Fatal("binding") })
	}
	for row := range more {
		t.Run(row, func(t *testing.T) { t.Fatal("binding") })
	}
}`,
			pkg: "fixture", test: "TestCase", subtest: "row",
		},
	}
	for _, fixture := range fixtures {
		if err := pibResolveFixtureSource(
			fixture.source, fixture.pkg, fixture.test, fixture.subtest,
		); err == nil {
			t.Fatalf("aggregate resolver accepted the %s false positive", fixture.name)
		}
	}
}

// pibDynamicSubtestIdentities are the run-time `t.Run` labels this repository
// actually ships. None of them is a literal the sources state: they are the
// `name`/`id` field of a table row, a loop-local shadow of a `[]string` element,
// or a nested positional table's field, produced only while the test runs.
//
// A ledger target may never name one. They are listed here so the proof below
// runs against the real shapes rather than against synthetic look-alikes; if a
// future change converts one of these owners to literal subtests, delete its
// entry here at the same time.
func pibDynamicSubtestIdentities() []s7FixtureTarget {
	return []s7FixtureTarget{
		{dir: "internal/cli", pkg: "cli", test: "TestAVPGrammarAndSurface", subtest: "--manual"},
		{dir: "internal/cli", pkg: "cli", test: "TestAVPGrammarAndSurface", subtest: "--regenerate"},
		{dir: "internal/cli", pkg: "cli", test: "TestAVPReadinessAndOutput", subtest: "AVP-052/sidecar-empty"},
		{dir: "internal/cli", pkg: "cli", test: "TestS6ArchiveRefusalMappings", subtest: "archive-index-corrupt"},
		{dir: "internal/cli", pkg: "cli", test: "TestS6IntentpubRefusalMappings", subtest: "undo-cas-mismatch"},
		{dir: "internal/intentpub", pkg: "intentpub", test: "TestStageV1ThroughV6AndArtifactSensitivity", subtest: "v1-empty"},
		{dir: "internal/intentpub", pkg: "intentpub", test: "TestRecoveryEvidenceTableAndIdempotency", subtest: "cp3-all-preimages"},
		{dir: "internal/intentpub", pkg: "intentpub", test: "TestRecoveryCP5CP6CP8Fixtures", subtest: "cp5-artifacts-new-index-old"},
		{dir: "internal/intentpub", pkg: "intentpub", test: "TestDecodeJournalJ1ThroughJ10", subtest: "trailing-value"},
		{dir: "internal/store", pkg: "store", test: "TestIntentArchiveStrictDecoderSensitivity", subtest: "unknown-top"},
	}
}

// TestPIBLedgerRejectsDynamicSubtestLabels is the standing guard against the
// aggregate ledger drifting back into treating a run-time `t.Run` label as a
// literal acceptance path.
//
// It proves three things:
//
//  1. Shape. Each of the four run-time label shapes this repository ships — a
//     positional table's `name` field, a keyed table's non-`name` field, a
//     loop-local shadow of a `[]string` element, and a nested positional table
//     inside a literal parent — is rejected, while the *literal* sibling in the
//     very same fixture still resolves. The rejection is therefore about the
//     label, not about the surrounding shape.
//  2. Reality. Every identity in pibDynamicSubtestIdentities, taken from the
//     shipped tests themselves, is unresolvable through the real resolver.
//  3. Ledger mutation. Swapping a live ledger row's target for one of those
//     identities makes that row resolve none of its targets, which is exactly
//     what TestPIBAggregateAcceptanceLedger records as a failure.
func TestPIBLedgerRejectsDynamicSubtestLabels(t *testing.T) {
	shapes := []struct {
		name    string
		source  string
		dynamic string
		literal string
	}{
		{
			name: "positional-table-name-field",
			source: `package fixture
import "testing"
func TestCase(t *testing.T) {
	tests := []struct {
		name  string
		class string
	}{
		{"v1-empty", "v1-nonempty"},
		{"v1-whitespace", "v1-nonempty"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.class == "" {
				t.Fatalf("%s has no class", test.name)
			}
		})
	}
	t.Run("v6-identity", func(t *testing.T) { t.Fatal("literal sibling binds") })
}`,
			dynamic: "v1-empty",
			literal: "v6-identity",
		},
		{
			name: "keyed-table-non-name-field",
			source: `package fixture
import "testing"
func TestCase(t *testing.T) {
	for _, flag := range []struct{ id, name string }{
		{id: "AVP-004", name: "--manual"},
		{id: "AVP-005", name: "--regenerate"},
	} {
		t.Run(flag.id, func(t *testing.T) {
			if flag.name == "" {
				t.Fatalf("%s carries no flag", flag.id)
			}
		})
	}
	t.Run("AVP-001", func(t *testing.T) { t.Fatal("literal sibling binds") })
}`,
			dynamic: "--manual",
			literal: "AVP-001",
		},
		{
			name: "shadowed-range-value",
			source: `package fixture
import "testing"
func TestCase(t *testing.T) {
	for _, code := range []string{"archive-index-corrupt", "archive-index-foreign"} {
		code := code
		t.Run(code, func(t *testing.T) {
			if code == "" {
				t.Fatal("empty refusal code")
			}
		})
	}
	t.Run("baseline", func(t *testing.T) { t.Fatal("literal sibling binds") })
}`,
			dynamic: "archive-index-corrupt",
			literal: "baseline",
		},
		{
			name: "nested-positional-table",
			source: `package fixture
import "testing"
func TestCase(t *testing.T) {
	t.Run("AVP-052", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			files string
		}{
			{"sidecar-empty", "  "},
			{"sidecar-absent", ""},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if tc.name == "" {
					t.Fatalf("%s has no fixture", tc.files)
				}
			})
		}
	})
	t.Run("AVP-031", func(t *testing.T) { t.Fatal("literal sibling binds") })
}`,
			dynamic: "AVP-052/sidecar-empty",
			literal: "AVP-031",
		},
	}
	for _, shape := range shapes {
		if err := pibResolveFixtureSource(
			shape.source, "fixture", "TestCase", shape.literal,
		); err != nil {
			t.Fatalf("%s: the literal sibling %q was rejected, so the shape proves nothing: %v",
				shape.name, shape.literal, err)
		}
		if err := pibResolveFixtureSource(
			shape.source, "fixture", "TestCase", shape.dynamic,
		); err == nil {
			t.Fatalf("%s: the resolver accepted the run-time label %q as a literal path",
				shape.name, shape.dynamic)
		}
	}

	cache := pibNewPackageCache()
	dynamic := pibDynamicSubtestIdentities()
	if len(dynamic) == 0 {
		t.Fatal("no shipped run-time label was named, so the guard proves nothing")
	}
	forbidden := map[string]bool{}
	for _, target := range dynamic {
		if err := pibResolveTarget(cache, target); err == nil {
			t.Fatalf("the aggregate resolver accepted the shipped run-time label %s",
				pibTargetKey(target))
		}
		forbidden[pibTargetKey(target)] = true
	}

	ledger := pibAggregateLedger(t)
	rows := map[string]pibLedgerRow{}
	for _, row := range ledger {
		for _, target := range row.targets {
			if forbidden[pibTargetKey(target)] {
				t.Fatalf("%s names the run-time label %s", row.id, pibTargetKey(target))
			}
		}
		rows[row.id] = row
	}
	for id, entry := range pibGuardSensitivityRegistry() {
		if forbidden[pibTargetKey(entry.sensitivity)] {
			t.Fatalf("%s sensitivity names the run-time label %s",
				id, pibTargetKey(entry.sensitivity))
		}
	}

	for _, mutation := range []struct {
		id     string
		target s7FixtureTarget
	}{
		{id: "PIB-005", target: dynamic[0]},
		{id: "PIB-082", target: dynamic[5]},
		{id: "PIB-298", target: dynamic[8]},
		{id: "PIB-331", target: dynamic[9]},
	} {
		row, ok := rows[mutation.id]
		if !ok || len(row.targets) == 0 {
			t.Fatalf("the aggregate ledger no longer carries %s", mutation.id)
		}
		for _, target := range row.targets {
			if err := pibResolveTarget(cache, target); err != nil {
				t.Fatalf("shipped %s target %s does not resolve: %v",
					mutation.id, pibTargetKey(target), err)
			}
		}
		resolved := 0
		for _, target := range []s7FixtureTarget{mutation.target} {
			if err := pibResolveTarget(cache, target); err == nil {
				resolved++
			}
		}
		if resolved != 0 {
			t.Fatalf("%s still resolved %d target(s) after its evidence was swapped for %s",
				mutation.id, resolved, pibTargetKey(mutation.target))
		}
	}
}

// TestPIBAggregateLedgerMetaMutations proves the ledger's own validators bite.
func TestPIBAggregateLedgerMetaMutations(t *testing.T) {
	matrix := parsePIBMatrix(t)
	ledger := pibAggregateLedger(t)

	// A removed row breaks contiguity against the matrix.
	if err := pibValidateLedgerShape(matrix, append(append([]pibLedgerRow{}, ledger[:10]...), ledger[11:]...)); err == nil {
		t.Fatal("ledger shape validator accepted a removed row")
	}
	// A duplicated row does too.
	duplicated := append([]pibLedgerRow{}, ledger...)
	duplicated[11] = duplicated[10]
	if err := pibValidateLedgerShape(matrix, duplicated); err == nil {
		t.Fatal("ledger shape validator accepted a duplicated row")
	}
	// A row moved to another position (and therefore another category/slice)
	// no longer lines up with the matrix authority.
	moved := append([]pibLedgerRow{}, ledger...)
	moved[10], moved[400] = moved[400], moved[10]
	if err := pibValidateLedgerShape(matrix, moved); err == nil {
		t.Fatal("ledger shape validator accepted a row moved into another category/slice")
	}
	// A short ledger is rejected outright.
	if err := pibValidateLedgerShape(matrix, ledger[:566]); err == nil {
		t.Fatal("ledger shape validator accepted 566 rows")
	}

	// Mis-kinding is caught by recomputing the kind totals from the matrix.
	misKinded := append([]pibMatrixRow{}, matrix...)
	for index := range misKinded {
		if misKinded[index].kind == "G" {
			misKinded[index].kind = "I"
			break
		}
	}
	if err := pibValidateMatrixTotals(misKinded); err == nil {
		t.Fatal("matrix arithmetic accepted a re-kinded row")
	}
	// So is moving a row to another category.
	reCategorised := append([]pibMatrixRow{}, matrix...)
	reCategorised[0].category = "AX"
	if err := pibValidateMatrixTotals(reCategorised); err == nil {
		t.Fatal("matrix arithmetic accepted a row moved to another category")
	}

	// Mis-slicing is caught by recomputing the slice partition from the matrix
	// against the shipped §18.52 slice table.
	misSliced := append([]pibMatrixRow{}, matrix...)
	for index := range misSliced {
		if misSliced[index].category == "AA" {
			misSliced[index].category = "AB"
			break
		}
	}
	if err := pibValidateSliceArithmetic(parsePIBSlicePartition(t), misSliced); err == nil {
		t.Fatal("slice arithmetic accepted a row moved into another slice's category")
	}
	if err := pibValidateSliceArithmetic(parsePIBSlicePartition(t), matrix); err != nil {
		t.Fatalf("slice arithmetic rejected the shipped matrix: %v", err)
	}
	doubled := parsePIBSlicePartition(t)
	doubled[0].categories = append(append([]string(nil), doubled[0].categories...), "AA")
	if err := pibValidateSliceArithmetic(doubled, matrix); err == nil {
		t.Fatal("slice arithmetic accepted a double-assigned category")
	}

	// Substituting a parent-only aggregate wrapper for a real target must fail
	// resolution even though the wrapper's children do assert.
	cache := pibNewPackageCache()
	wrapper := s7FixtureTarget{
		dir: "internal/cli", pkg: "cli",
		test: "TestS6PrepareContractRows",
	}
	if err := pibResolveTarget(cache, wrapper); err == nil {
		t.Fatal("resolver accepted a broad parent-only wrapper as an acceptance target")
	}

	// The sensitivity registry must reject a baseline-only target: the row's own
	// acceptance body is not automatically a wrong-input fixture. The declared
	// validator is a symbol this body genuinely calls, so what fails is the proof
	// shape and not a name lookup.
	baselineOnly := s7FixtureTarget{
		dir: "internal/cli", pkg: "cli",
		test: "TestPrepareS4StagesBeforeRevalidationAndPublishesStatusLast",
	}
	if err := pibResolveTarget(cache, baselineOnly); err != nil {
		t.Fatalf("baseline probe is not a runnable body: %v", err)
	}
	if err := pibAssertSensitivityShape(cache, "PIB-231", pibSensitivityEntry{
		sensitivity: baselineOnly, validator: "runPrepare",
	}); err == nil {
		t.Fatal("sensitivity shape check accepted a baseline-only body")
	}

	// A registry that drops a derived G row, gains an undeclared row, or points
	// two rows at one identity must be caught.
	registry := pibGuardSensitivityRegistry()
	derived := []string{}
	for _, row := range matrix {
		if strings.Contains(row.kind, "G") {
			derived = append(derived, row.id)
		}
	}
	sort.Strings(derived)
	if err := pibValidateRegistryKeys(derived, pibRegistryWithout(registry, derived[0])); err == nil {
		t.Fatal("registry key check accepted a missing G row")
	}
	extra := pibRegistryWithout(registry, "")
	extra["PIB-001"] = pibSensitivityEntry{
		sensitivity: s7FixtureTarget{
			dir: "internal/cli", pkg: "cli", test: "TestPIBRowPIB180RefusalPointerGuardAndSensitivity",
		},
	}
	if err := pibValidateRegistryKeys(derived, extra); err == nil {
		t.Fatal("registry key check accepted a row the Kind column does not derive")
	}
	shared := pibRegistryWithout(registry, "")
	var donor, receiver string
	for _, id := range derived {
		if _, ok := shared[id]; !ok {
			continue
		}
		if donor == "" {
			donor = id
			continue
		}
		receiver = id
		break
	}
	if donor == "" || receiver == "" {
		t.Fatal("registry has fewer than two live sensitivity entries")
	}
	shared[receiver] = shared[donor]
	if err := pibValidateRegistryUniqueness(shared); err == nil {
		t.Fatal("registry uniqueness check accepted two G rows sharing one fixture identity")
	}
	emptied := pibRegistryWithout(registry, "")
	emptied[derived[0]] = pibSensitivityEntry{}
	if err := pibValidateRegistryUniqueness(emptied); err == nil {
		t.Fatal("registry uniqueness check accepted an empty (blocked-shaped) sensitivity identity")
	}
}

func pibRegistryWithout(
	registry map[string]pibSensitivityEntry, drop string,
) map[string]pibSensitivityEntry {
	copied := make(map[string]pibSensitivityEntry, len(registry))
	for id, entry := range registry {
		if id == drop {
			continue
		}
		copied[id] = entry
	}
	return copied
}

func pibValidateLedgerShape(matrix []pibMatrixRow, ledger []pibLedgerRow) error {
	if len(ledger) != len(matrix) {
		return fmt.Errorf("ledger has %d rows, matrix has %d", len(ledger), len(matrix))
	}
	for index, row := range ledger {
		if row.id != matrix[index].id {
			return fmt.Errorf("ledger row %d = %s, matrix says %s", index, row.id, matrix[index].id)
		}
	}
	return nil
}

func pibValidateMatrixTotals(matrix []pibMatrixRow) error {
	categoryCounts := map[string]int{}
	kindCounts := map[string]int{}
	for _, row := range matrix {
		categoryCounts[row.category]++
		kindCounts[row.kind]++
	}
	wantKinds := map[string]int{"I": 248, "C": 122, "G": 123, "U": 49, "S": 25}
	if fmt.Sprint(kindCounts) != fmt.Sprint(wantKinds) {
		return fmt.Errorf("kind totals = %v, want %v", kindCounts, wantKinds)
	}
	wantCategories := map[string]int{
		"A": 20, "B": 24, "C": 15, "D": 12, "E": 9, "F": 19, "G": 13, "H": 14,
		"I": 13, "J": 8, "K": 12, "L": 10, "M": 14, "N": 14, "O": 10, "P": 7,
		"Q": 6, "R": 3, "S": 9, "T": 2, "U": 10, "V": 12, "W": 5, "X": 6,
		"Y": 7, "Z": 4, "AA": 15, "AB": 7, "AC": 10, "AD": 8, "AE": 10, "AF": 5,
		"AG": 14, "AH": 17, "AI": 6, "AJ": 10, "AK": 10, "AL": 4, "AM": 15,
		"AN": 23, "AO": 16, "AP": 34, "AQ": 23, "AR": 15, "AS": 10, "AT": 6,
		"AU": 9, "AV": 6, "AW": 9, "AX": 7,
	}
	if len(categoryCounts) != len(wantCategories) {
		return fmt.Errorf("category count = %d, want %d", len(categoryCounts), len(wantCategories))
	}
	for category, count := range wantCategories {
		if categoryCounts[category] != count {
			return fmt.Errorf("category %s = %d, want %d", category, categoryCounts[category], count)
		}
	}
	return nil
}

func pibValidateRegistryKeys(derived []string, registry map[string]pibSensitivityEntry) error {
	if len(registry) != len(derived) {
		return fmt.Errorf("registry has %d rows, the Kind column derives %d", len(registry), len(derived))
	}
	for _, id := range derived {
		if _, ok := registry[id]; !ok {
			return fmt.Errorf("registry is missing derived G row %s", id)
		}
	}
	for id := range registry {
		if !pibContains(derived, id) {
			return fmt.Errorf("registry carries %s, which is not derived as G", id)
		}
	}
	return nil
}

func pibValidateRegistryUniqueness(registry map[string]pibSensitivityEntry) error {
	owners := map[string]string{}
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		entry := registry[id]
		if entry.sensitivity == (s7FixtureTarget{}) {
			return fmt.Errorf("%s carries an empty sensitivity identity", id)
		}
		if entry.validator == "" {
			return fmt.Errorf("%s names no expected validator family", id)
		}
		key := pibTargetKey(entry.sensitivity)
		if owner, reused := owners[key]; reused {
			return fmt.Errorf("%s reuses %s's sensitivity identity %s", id, owner, key)
		}
		owners[key] = id
	}
	return nil
}

// pibValidateSliceArithmetic recomputes the §18.52 slice partition from the
// matrix and compares it with the shipped table: a row moved into another
// slice's category, a double-assigned category, an unassigned category and a
// wrong per-slice total all fail here.
func pibValidateSliceArithmetic(slices []pibSliceSpec, matrix []pibMatrixRow) error {
	categoryCounts := map[string]int{}
	for _, row := range matrix {
		categoryCounts[row.category]++
	}
	assigned := map[string]string{}
	total := 0
	for _, slice := range slices {
		derived := 0
		for _, category := range slice.categories {
			if owner, duplicated := assigned[category]; duplicated {
				return fmt.Errorf("category %s is assigned to both %s and %s", category, owner, slice.name)
			}
			assigned[category] = slice.name
			derived += categoryCounts[category]
		}
		if derived != slice.rows {
			return fmt.Errorf(
				"slice %s: categories sum to %d rows, the shipped table claims %d",
				slice.name, derived, slice.rows,
			)
		}
		total += slice.rows
	}
	if total != pibMatrixRowCount {
		return fmt.Errorf("slice totals sum to %d, want %d", total, pibMatrixRowCount)
	}
	for category := range categoryCounts {
		if assigned[category] == "" {
			return fmt.Errorf("category %s is unassigned by the slice partition", category)
		}
	}
	for category := range assigned {
		if categoryCounts[category] == 0 {
			return fmt.Errorf("slice partition assigns category %s, which has no matrix rows", category)
		}
	}
	return nil
}

// TestPIBMatrixParserAuthority drives the matrix parser against synthetic
// documents. Each one is valid Markdown that textually contains matrix-shaped
// rows; only an authority-scoped parser tells them apart.
func TestPIBMatrixParserAuthority(t *testing.T) {
	const header = "| ID | Kind | Case | Asserted observable |\n|---|---|---|---|\n"
	good := "### 18.2 A — first\n\n" + header +
		"| PIB-001 | I | case | observable |\n" +
		"| PIB-003 | G | case | observable |\n" +
		"\n### 18.3 B — second\n\n" + header +
		"| PIB-002 | C | case | observable |\n"
	rows, err := pibParseMatrixDocument(good, 3)
	if err != nil {
		t.Fatalf("parser rejected a valid out-of-position matrix: %v", err)
	}
	if len(rows) != 3 || rows[0].id != "PIB-001" || rows[1].id != "PIB-002" || rows[2].id != "PIB-003" {
		t.Fatalf("parser did not return numeric order: %#v", rows)
	}
	if rows[1].category != "B" || rows[2].category != "A" || rows[2].kind != "G" {
		t.Fatalf("parser lost the enclosing category or the Kind column: %#v", rows)
	}

	for _, fixture := range []struct {
		name string
		body string
		want int
	}{
		{
			name: "revision-history-row-outside-a-table",
			body: "### 18.2 A — first\n\n" + header +
				"| PIB-001 | I | case | observable |\n" +
				"\nRevision history:\n\n" +
				"| PIB-002 | G | restated | restated |\n",
			want: 2,
		},
		{
			name: "row-in-the-counts-summary-section",
			body: "### 18.2 A — first\n\n" + header +
				"| PIB-001 | I | case | observable |\n" +
				"\n### 18.52 Counts, kinds and slice partition\n\n" + header +
				"| PIB-002 | G | restated | restated |\n",
			want: 2,
		},
		{
			name: "row-under-a-non-matrix-heading-level",
			body: "### 18.2 A — first\n\n" + header +
				"| PIB-001 | I | case | observable |\n" +
				"\n#### Notes\n\n" + header +
				"| PIB-002 | G | restated | restated |\n",
			want: 2,
		},
		{
			name: "duplicate-authoritative-row",
			body: "### 18.2 A — first\n\n" + header +
				"| PIB-001 | I | case | observable |\n" +
				"\n### 18.3 B — second\n\n" + header +
				"| PIB-001 | I | case | observable |\n",
			want: 1,
		},
		{
			name: "duplicate-with-disagreeing-kind",
			body: "### 18.2 A — first\n\n" + header +
				"| PIB-001 | I | case | observable |\n" +
				"\n### 18.3 B — second\n\n" + header +
				"| PIB-001 | G | case | observable |\n",
			want: 1,
		},
		{
			name: "gapped-id-sequence",
			body: "### 18.2 A — first\n\n" + header +
				"| PIB-001 | I | case | observable |\n" +
				"| PIB-003 | I | case | observable |\n",
			want: 2,
		},
		{
			name: "out-of-order-inside-one-section",
			body: "### 18.2 A — first\n\n" + header +
				"| PIB-002 | I | case | observable |\n" +
				"| PIB-001 | I | case | observable |\n",
			want: 2,
		},
		{
			name: "short-matrix",
			body: "### 18.2 A — first\n\n" + header +
				"| PIB-001 | I | case | observable |\n",
			want: 2,
		},
	} {
		if _, err := pibParseMatrixDocument(fixture.body, fixture.want); err == nil {
			t.Fatalf("matrix parser accepted the %s false positive", fixture.name)
		}
	}
}

// TestPIBLedgerCarriesNoBlockedEscape is the structural meta-gate for the
// zero-blocked contract. The ledger and the sensitivity registry must not
// reintroduce a `blocked` field, a declared-blocked list, or any other opt-out:
// a row without a resolvable target must fail, never be excused.
func TestPIBLedgerCarriesNoBlockedEscape(t *testing.T) {
	root := avpRepoRoot(t)
	sources := map[string]string{}
	for _, rel := range []string{
		"internal/cli/prepare_pib_acceptance_ledger_test.go",
		"internal/cli/prepare_pib_ledger_rows_test.go",
		"internal/cli/prepare_pib_ledger_sensitivity_test.go",
	} {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		sources[rel] = string(body)
	}
	if err := pibValidateNoBlockedEscape(sources); err != nil {
		t.Fatalf("the aggregate ledger reintroduced a blocked escape: %v", err)
	}

	mutated := map[string]string{}
	for rel, body := range sources {
		mutated[rel] = body
	}
	const declaration = "internal/cli/prepare_pib_ledger_rows_test.go"
	mutated[declaration] = strings.Replace(
		mutated[declaration],
		"func pibEarlySliceLedger() map[string]pibLedgerRow {",
		"var pibDeclaredBlockedRows = []string{\"PIB-004\"}\n\n"+
			"func pibEarlySliceLedger() map[string]pibLedgerRow {",
		1,
	)
	if mutated[declaration] == sources[declaration] {
		t.Fatal("the blocked-escape mutation anchor is missing")
	}
	if err := pibValidateNoBlockedEscape(mutated); err == nil {
		t.Fatal("the structural gate accepted a reintroduced declared-blocked list")
	}

	fielded := map[string]string{}
	for rel, body := range sources {
		fielded[rel] = body
	}
	const typeFile = "internal/cli/prepare_pib_acceptance_ledger_test.go"
	fielded[typeFile] = strings.Replace(
		fielded[typeFile],
		"type pibLedgerRow struct {\n\tid      string",
		"type pibLedgerRow struct {\n\tblocked string\n\tid      string",
		1,
	)
	if fielded[typeFile] == sources[typeFile] {
		t.Fatal("the blocked-field mutation anchor is missing")
	}
	if err := pibValidateNoBlockedEscape(fielded); err == nil {
		t.Fatal("the structural gate accepted a reintroduced blocked field")
	}
}

// pibValidateNoBlockedEscape rejects any declaration that would let a row opt
// out of resolution. The scan is structural over the AST — a struct field named
// `blocked`, a `blocked:` element in a composite literal, a `.blocked` selector,
// or a top-level `pibDeclaredBlocked…` identifier — so string literals that
// merely mention those spellings (including this file's own fixtures) cannot
// trip it, and a renamed-but-equivalent escape still can.
func pibValidateNoBlockedEscape(sources map[string]string) error {
	if len(sources) != 3 {
		return fmt.Errorf("the escape scan covers %d ledger files, want 3", len(sources))
	}
	rels := make([]string, 0, len(sources))
	for rel := range sources {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		file, err := parser.ParseFile(token.NewFileSet(), rel, sources[rel], 0)
		if err != nil {
			return err
		}
		var found string
		ast.Inspect(file, func(node ast.Node) bool {
			if found != "" {
				return false
			}
			switch typed := node.(type) {
			case *ast.StructType:
				if typed.Fields == nil {
					return true
				}
				for _, field := range typed.Fields.List {
					for _, name := range field.Names {
						if name.Name == "blocked" {
							found = "a struct field named blocked"
						}
					}
				}
			case *ast.KeyValueExpr:
				if key, ok := typed.Key.(*ast.Ident); ok && key.Name == "blocked" {
					found = "a blocked: element in a composite literal"
				}
			case *ast.SelectorExpr:
				if typed.Sel.Name == "blocked" {
					found = "a .blocked selector"
				}
			case *ast.ValueSpec:
				for _, name := range typed.Names {
					if strings.HasPrefix(name.Name, "pibDeclaredBlocked") {
						found = "the declared-blocked list " + name.Name
					}
				}
			case *ast.FuncDecl:
				if strings.HasPrefix(typed.Name.Name, "pibDeclaredBlocked") {
					found = "the declared-blocked accessor " + typed.Name.Name
				}
			}
			return true
		})
		if found != "" {
			return fmt.Errorf("%s reintroduces the blocked escape: %s", rel, found)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// PIB-231 proof-gate bites.
//
// Every fixture below is a *complete, parseable* test body that the rev-3 gate
// accepted and that proves nothing. The gate must reject each of them, and must
// still accept the two genuine shapes at the end — otherwise the strengthening
// would have been bought by breaking real rows.
// ---------------------------------------------------------------------------

// pibMetaFixture wraps a body in a package whose helpers have the shapes the
// gate reasons about. Nothing here is executed; the gate is pure AST work.
func pibMetaFixture(body string) string {
	return `package fixture

import (
	"fmt"
	"strings"
	"testing"
)

type pibMetaReport struct{ Refusal *string }

func shippedSources() map[string]string { return map[string]string{"a.go": "shipped"} }
func shippedSource() string             { return "shipped" }
func cloneSources(sources map[string]string) map[string]string {
	cloned := map[string]string{}
	for name, body := range sources {
		cloned[name] = body
	}
	return cloned
}
func runProduct() pibMetaReport                        { return pibMetaReport{} }
func validateGuard(input any) error                    { return fmt.Errorf("%v", input) }
func validateOther(input any) error                    { return fmt.Errorf("%v", input) }
func _pibMetaUnused()                                  { _ = strings.TrimSpace("") }

func TestCase(t *testing.T) {
` + body + `
}
`
}

func TestPIBSensitivityProofGateBites(t *testing.T) {
	rejected := []struct {
		name    string
		subtest string
		body    string
	}{
		{
			// A body that only proves the shipped input is accepted.
			name: "baseline-only",
			body: `	sources := shippedSources()
	if err := validateGuard(sources); err != nil {
		t.Fatal(err)
	}`,
		},
		{
			// The mutation lives in a sibling subtest. rev-3's gate fell back to
			// the enclosing function and passed this; the leaf proves nothing.
			name:    "sibling-subtest-mutation",
			subtest: "target",
			body: `	sources := shippedSources()
	t.Run("sibling", func(t *testing.T) {
		wrong := cloneSources(sources)
		wrong["a.go"] = strings.Replace(wrong["a.go"], "shipped", "wrong", 1)
		if err := validateGuard(wrong); err == nil {
			t.Fatal("the guard accepted a mutated input")
		}
	})
	t.Run("target", func(t *testing.T) {
		if err := validateGuard(sources); err == nil {
			t.Fatal("the guard accepted the shipped input")
		}
	})`,
		},
		{
			// Ordinary setup: an append, a format and a field write, none of
			// which reaches the asserted call.
			name: "common-append-and-format-setup",
			body: `	sources := shippedSources()
	names := append([]string{}, "a.go")
	sources["log"] = fmt.Sprintf("case %d", len(names))
	if err := validateGuard(shippedSources()); err == nil {
		t.Fatal("the guard accepted the shipped input")
	}`,
		},
		{
			// A real mutation is built and then never handed to the validator.
			name: "mutation-never-reaches-the-validator",
			body: `	sources := shippedSources()
	wrong := cloneSources(sources)
	wrong["a.go"] = strings.Replace(wrong["a.go"], "shipped", "wrong", 1)
	if len(wrong) == 0 {
		t.Fatal("the fixture built no wrong input")
	}
	if err := validateGuard(sources); err == nil {
		t.Fatal("the guard accepted the shipped input")
	}`,
		},
		{
			// The mutation reaches a *different* validator; the declared one is
			// still only asked about the shipped input.
			name: "another-validator-receives-the-mutation",
			body: `	sources := shippedSources()
	wrong := cloneSources(sources)
	wrong["a.go"] = strings.Replace(wrong["a.go"], "shipped", "wrong", 1)
	if err := validateOther(wrong); err == nil {
		t.Fatal("the other guard accepted a mutated input")
	}
	if err := validateGuard(sources); err == nil {
		t.Fatal("the guard accepted the shipped input")
	}`,
		},
		{
			// The validator is asked the right question and the answer is
			// discarded, so nothing fails when the wrong input is accepted.
			name: "validator-result-ignored",
			body: `	sources := shippedSources()
	wrong := cloneSources(sources)
	wrong["a.go"] = strings.Replace(wrong["a.go"], "shipped", "wrong", 1)
	if err := validateGuard(wrong); err == nil {
		t.Logf("the guard accepted a mutated input")
	}
	if err := validateGuard(sources); err != nil {
		t.Fatal(err)
	}`,
		},
		{
			// `report.Refusal == nil` is a baseline observation about the product,
			// not a claim that a validator rejected a wrong input.
			name: "baseline-nil-check-is-not-a-rejection",
			body: `	report := runProduct()
	if report.Refusal == nil {
		t.Fatal("the shipped input was not refused")
	}
	wrong := cloneSources(shippedSources())
	wrong["a.go"] = strings.Replace(wrong["a.go"], "shipped", "wrong", 1)
	_ = wrong`,
		},
		{
			// A table-registered leaf whose own element passes the baseline
			// straight through carries no wrong input, even though a sibling
			// element in the same table does.
			name:    "fixture-table-element-passes-the-baseline-through",
			subtest: "passthrough",
			body: `	source := shippedSource()
	fixtures := []struct {
		name   string
		source string
	}{
		{name: "widened", source: strings.Replace(source, "shipped", "wrong", 1)},
		{name: "passthrough", source: source},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			if err := validateGuard(fixture.source); err == nil {
				t.Fatal("the guard accepted a mutated input")
			}
		})
	}`,
		},
	}
	for _, bite := range rejected {
		bite := bite
		t.Run(bite.name, func(t *testing.T) {
			entry := pibSensitivityEntry{
				sensitivity: s7FixtureTarget{
					dir: "fixture", pkg: "fixture", test: "TestCase", subtest: bite.subtest,
				},
				validator: "validateGuard",
			}
			if err := pibAssertSensitivityShapeSource(
				pibMetaFixture(bite.body), "PIB-231", entry,
			); err == nil {
				t.Fatal("the proof gate accepted a fixture that proves nothing")
			}
		})
	}

	accepted := []struct {
		name    string
		subtest string
		body    string
	}{
		{
			// The genuine shape: one validator, the shipped input accepted and
			// the mutated input rejected.
			name: "same-validator-baseline-and-mutation",
			body: `	sources := shippedSources()
	if err := validateGuard(sources); err != nil {
		t.Fatalf("the shipped input failed its own guard: %v", err)
	}
	wrong := cloneSources(sources)
	wrong["a.go"] = strings.Replace(wrong["a.go"], "shipped", "wrong", 1)
	if err := validateGuard(wrong); err == nil {
		t.Fatal("the guard accepted a mutated input")
	}`,
		},
		{
			// The table shape: the leaf's own element injects the wrong input.
			name:    "fixture-table-element-injects-the-wrong-input",
			subtest: "widened",
			body: `	source := shippedSource()
	t.Run("baseline", func(t *testing.T) {
		if err := validateGuard(source); err != nil {
			t.Fatal(err)
		}
	})
	fixtures := []struct {
		name   string
		source string
	}{
		{name: "widened", source: strings.Replace(source, "shipped", "wrong", 1)},
		{name: "passthrough", source: source},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			if err := validateGuard(fixture.source); err == nil {
				t.Fatal("the guard accepted a mutated input")
			}
		})
	}`,
		},
	}
	for _, control := range accepted {
		control := control
		t.Run(control.name, func(t *testing.T) {
			entry := pibSensitivityEntry{
				sensitivity: s7FixtureTarget{
					dir: "fixture", pkg: "fixture", test: "TestCase", subtest: control.subtest,
				},
				validator: "validateGuard",
			}
			if err := pibAssertSensitivityShapeSource(
				pibMetaFixture(control.body), "PIB-231", entry,
			); err != nil {
				t.Fatalf("the proof gate rejected a genuine wrong-input fixture: %v", err)
			}
		})
	}

	t.Run("declared-validator-is-load-bearing", func(t *testing.T) {
		body := `	sources := shippedSources()
	if err := validateOther(sources); err != nil {
		t.Fatal(err)
	}
	wrong := cloneSources(sources)
	wrong["a.go"] = strings.Replace(wrong["a.go"], "shipped", "wrong", 1)
	if err := validateOther(wrong); err == nil {
		t.Fatal("the other guard accepted a mutated input")
	}`
		target := s7FixtureTarget{dir: "fixture", pkg: "fixture", test: "TestCase"}
		if err := pibAssertSensitivityShapeSource(pibMetaFixture(body), "PIB-231",
			pibSensitivityEntry{sensitivity: target, validator: "validateOther"}); err != nil {
			t.Fatalf("the gate rejected a fixture that proves its own validator: %v", err)
		}
		if err := pibAssertSensitivityShapeSource(pibMetaFixture(body), "PIB-231",
			pibSensitivityEntry{sensitivity: target, validator: "validateGuard"}); err == nil {
			t.Fatal("the gate accepted another validator's fixture as this row's proof")
		}
		if err := pibAssertSensitivityShapeSource(pibMetaFixture(body), "PIB-231",
			pibSensitivityEntry{sensitivity: target}); err == nil {
			t.Fatal("the gate accepted a row that declares no validator")
		}
	})

	t.Run("witness-pair-is-bound-to-the-row-id", func(t *testing.T) {
		source := `package fixture

import "testing"

type pibMetaSpec struct {
	run         func(*testing.T) error
	fixture     string
	sensitivity func(*testing.T) error
}

var pibMetaSpecs = map[string]pibMetaSpec{}

func TestCase(t *testing.T) {
	spec, registered := pibMetaSpecs["PIB-155"]
	if !registered || spec.run == nil || spec.sensitivity == nil || spec.fixture == "" {
		t.Fatal("PIB-155 has no registered guard spec")
	}
	if err := spec.run(t); err != nil {
		t.Fatalf("PIB-155 guard rejected the shipped input: %v", err)
	}
	if err := spec.sensitivity(t); err == nil {
		t.Fatalf("PIB-155 sensitivity fixture %q was accepted by its own validator", spec.fixture)
	}
}
`
		target := s7FixtureTarget{dir: "fixture", pkg: "fixture", test: "TestCase"}
		if err := pibAssertSensitivityShapeSource(source, "PIB-155",
			pibSensitivityEntry{sensitivity: target, validator: "pibMetaSpecs"}); err != nil {
			t.Fatalf("the gate rejected a witness pair keyed by its own row: %v", err)
		}
		if err := pibAssertSensitivityShapeSource(source, "PIB-158",
			pibSensitivityEntry{sensitivity: target, validator: "pibMetaSpecs"}); err == nil {
			t.Fatal("the gate accepted a witness pair keyed by another row")
		}
	})
}

// TestPIBSensitivityRotationsFailTheProofGate replays the seven rev-3 rows whose
// registry entry pointed at a *sibling's* wrong-input body. Each target exists,
// resolves, mutates a real input and asserts a rejection — it is simply not this
// row's claim. Binding the row to its own validator is what fails them.
func TestPIBSensitivityRotationsFailTheProofGate(t *testing.T) {
	registry := pibGuardSensitivityRegistry()
	cache := pibNewPackageCache()
	rotations := map[string]s7FixtureTarget{
		"PIB-142": {dir: "internal/cli", pkg: "cli",
			test: "TestPrepareS5ProvenanceBoundaryRows", subtest: "PIB-143"},
		"PIB-144": {dir: "internal/cli", pkg: "cli",
			test: "TestPrepareS5ProvenanceBoundaryRows", subtest: "PIB-145"},
		"PIB-145": {dir: "internal/cli", pkg: "cli",
			test: "TestPrepareS5ProvenanceBoundaryRows", subtest: "PIB-147"},
		"PIB-146": {dir: "internal/cli", pkg: "cli",
			test: "TestPrepareS5ProvenanceBoundaryRows", subtest: "PIB-378"},
		"PIB-326": {dir: "internal/gitutil", pkg: "gitutil",
			test: "TestS7APGitContracts", subtest: "PIB-473"},
		"PIB-344": {dir: "internal/cli", pkg: "cli", test: "TestS6PrepareContractRows",
			subtest: "PIB-154-write-target-constructor-closure/approved-slug-constructor-contract"},
		"PIB-473": {dir: "internal/gitutil", pkg: "gitutil",
			test: "TestS7APGitContracts", subtest: "PIB-476"},
	}
	ids := make([]string, 0, len(rotations))
	for id := range rotations {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		id := id
		t.Run(id, func(t *testing.T) {
			entry, registered := registry[id]
			if !registered || entry.validator == "" {
				t.Fatalf("%s is not a registered G row with a declared validator", id)
			}
			if pibTargetKey(entry.sensitivity) == pibTargetKey(rotations[id]) {
				t.Fatalf("%s still points at the rotated body %s", id, pibTargetKey(rotations[id]))
			}
			if err := pibAssertSensitivityShape(cache, id, entry); err != nil {
				t.Fatalf("%s's own fixture does not prove its claim: %v", id, err)
			}
			rotated := pibSensitivityEntry{
				sensitivity: rotations[id],
				validator:   entry.validator,
			}
			if err := pibAssertSensitivityShape(cache, id, rotated); err == nil {
				t.Fatalf("the proof gate accepted %s's rotated target %s",
					id, pibTargetKey(rotations[id]))
			}
		})
	}
}
