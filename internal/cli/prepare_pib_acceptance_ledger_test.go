//go:build (linux && !android) || (darwin && !ios)

package cli

// GH #23 aggregate acceptance closure — PRD-prepare-intent-bundle §18.52/§18.53,
// driven by PIB-230 (mechanical ledger derivation) and PIB-231 (mechanical
// sensitivity derivation from the Kind column).
//
// This file is the *aggregate* ledger over the whole 567-row matrix. It does not
// replace the per-category S7 ledgers (`s7AMCoverageLedger` … `s7AXCoverageLedger`)
// or the 31-row S6 guard subset (`s6AcceptanceLedger`); for `PIB-395`…`PIB-567` it
// *reuses* the accepted S7 ledgers verbatim rather than restating them, so an S7
// row cannot be weakened here without failing its own category gate first.
//
// Three properties are enforced:
//
//  1. Totality and arithmetic. Every number in §18.52 is recomputed from the
//     shipped matrix table — not read from the prose — and then compared against
//     the prose. Row IDs must be contiguous `PIB-001`…`PIB-567` with no
//     duplicates, no gaps and no retired IDs; the 50 category counts, the five
//     kind totals and the nine slice totals must all agree.
//
//  2. Body-sensitive resolution. Every ledger row names an exact
//     `(dir, package, test, subtest)` identity that must resolve, by static AST
//     analysis, to a *runnable body that binds*. Comment text, string literals,
//     the wrong package, the wrong signature, a wrong `testing` alias, a shadowed
//     receiver, a non-literal subtest name, a missing subtest whose siblings
//     survive, an assertion that is only reachable inside a nested function
//     literal, an assertion after an unconditional terminator, a
//     short-circuit-unreachable assertion, and a parent that only dispatches to
//     children are all rejected. `TestPIBLedgerResolverSensitivity` proves each.
//
//  3. G-row sensitivity. The guard set is derived from the Kind column of the
//     shipped table — never from a hand-kept list — and every derived `G` row must
//     carry a *sensitivity* target: a body that mutates a guarded input and then
//     asserts the same validator rejects it. A baseline-only body, an empty/no-op
//     mutation, and a body that merely scans row names do not qualify.
//
// Runtime is bounded: resolution is pure AST work over a per-package parse cache.
// The ledger never executes an acceptance target, and it is a normal
// non-observer gate — it does not register itself with the S7 registration
// observer and does not re-run the 567 rows.

import (
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
	"sync"
	"testing"
)

const (
	pibMatrixRowCount = 567
	pibCategoryCount  = 50
	// pibMaxRowsPerIdentity bounds how many matrix rows may lean on a single
	// acceptance identity, so the ledger cannot degenerate into a broad proxy.
	pibMaxRowsPerIdentity = 6
)

// ---------------------------------------------------------------------------
// Matrix parsing — the authority for id, kind, category and slice.
// ---------------------------------------------------------------------------

type pibMatrixRow struct {
	id       string
	kind     string
	category string
}

type pibSliceSpec struct {
	name       string
	categories []string
	rows       int
}

var (
	pibHeadingPattern = regexp.MustCompile(`^### 18\.(\d+) ([A-Z]{1,2}) — `)
	pibAnyHeading     = regexp.MustCompile(`^#{1,6} `)
	pibRowPattern     = regexp.MustCompile(`^\| (PIB-[0-9]{3}) \| ([ICGUS]+) \| `)
	pibSliceRow       = regexp.MustCompile(`^\| (S[0-9][a-z]?) ([^|]*?)\s*\| ([A-Z, ]+) \| ([0-9]+) \|$`)
	pibProseCategory  = regexp.MustCompile(`\b(A[A-X]|[A-Z]) ([0-9]+)[,.]`)
	pibProseKind      = regexp.MustCompile("`(I|C|G|U|S)` ([0-9]+)")
)

// pibMatrixHeader is the exact header line every §18.N acceptance matrix table
// starts with. Scoping row extraction to a table that opens with this header is
// what keeps revision-history bullets, prose sentences and the §18.52/§18.53
// summary tables out of the authoritative row set.
const (
	pibMatrixHeader     = "| ID | Kind | Case | Asserted observable |"
	pibMatrixSeparator  = "|---|---|---|---|"
	pibFirstMatrixIndex = 2
	pibLastMatrixIndex  = 51
)

func pibPRDBody(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(
		avpRepoRoot(t), "docs", "prds", "PRD-prepare-intent-bundle.md",
	))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// parsePIBMatrix returns the one authoritative acceptance-matrix row per PIB ID,
// in numeric order. It is a thin wrapper around pibParseMatrixDocument so the
// parser itself can be driven by TestPIBMatrixParserAuthority against synthetic
// documents that a byte scan would accept.
func parsePIBMatrix(t *testing.T) []pibMatrixRow {
	t.Helper()
	rows, err := pibParseMatrixDocument(pibPRDBody(t), pibMatrixRowCount)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

// pibParseMatrixDocument walks §18.2…§18.51 and returns one row per shipped
// matrix line, sorted by numeric ID.
//
// Four things make the extraction authoritative rather than textual:
//
//   - Section scoping. Only `### 18.N X — ` headings with 2 ≤ N ≤ 51 open a
//     category. Any other heading (including `### 18.1`, `### 18.52`,
//     `### 18.53` and every `##`/`####` level) closes the current section, so a
//     `| PIB-NNN | G |`-shaped line in revision history or in the counts summary
//     is never read as a row.
//   - Table scoping. Inside a section, rows are only collected while the reader
//     is inside a table opened by the exact matrix header line. A blank line or
//     any non-`|` line closes the table.
//   - Duplicate rejection with disagreement reporting. A second authoritative
//     row for an ID is an error, and the error names the disagreement when the
//     duplicate's kind or category differs.
//   - Contiguity. The sorted IDs must be exactly `PIB-001`…`PIB-<want>`: a gap,
//     a missing ID, an out-of-range ID or a short/long matrix all fail.
//
// Physical order is deliberately *not* required to be numeric: §18.28 AA carries
// `PIB-392`…`PIB-394` after `PIB-290` because those rows were added to that
// category later, and §18.1 fixes the ID rather than the position. Sorting is
// therefore part of the contract, and the sorted sequence is what must be gapless.
func pibParseMatrixDocument(body string, want int) ([]pibMatrixRow, error) {
	category := ""
	section := 0
	inTable := false
	previous := 0
	seen := map[string]pibMatrixRow{}
	rows := []pibMatrixRow{}
	for number, line := range strings.Split(body, "\n") {
		if match := pibHeadingPattern.FindStringSubmatch(line); match != nil {
			index, err := strconv.Atoi(match[1])
			if err != nil {
				return nil, err
			}
			section, category, inTable, previous = index, match[2], false, 0
			if index < pibFirstMatrixIndex || index > pibLastMatrixIndex {
				section, category = 0, ""
			}
			continue
		}
		if pibAnyHeading.MatchString(line) {
			section, category, inTable, previous = 0, "", false, 0
			continue
		}
		if category == "" {
			continue
		}
		if strings.TrimSpace(line) == pibMatrixHeader {
			inTable = true
			continue
		}
		if strings.TrimSpace(line) == pibMatrixSeparator {
			continue
		}
		if !strings.HasPrefix(line, "|") {
			inTable = false
			continue
		}
		match := pibRowPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if !inTable {
			return nil, fmt.Errorf(
				"line %d: %s appears in §18.%d outside the acceptance matrix table",
				number+1, match[1], section,
			)
		}
		row := pibMatrixRow{id: match[1], kind: match[2], category: category}
		if existing, duplicated := seen[row.id]; duplicated {
			if existing.kind != row.kind || existing.category != row.category {
				return nil, fmt.Errorf(
					"line %d: %s is claimed twice and the claims disagree: %s/%s versus %s/%s",
					number+1, row.id, existing.category, existing.kind, row.category, row.kind,
				)
			}
			return nil, fmt.Errorf("line %d: %s has two authoritative matrix rows", number+1, row.id)
		}
		value, err := strconv.Atoi(strings.TrimPrefix(row.id, "PIB-"))
		if err != nil {
			return nil, err
		}
		if value <= previous {
			return nil, fmt.Errorf(
				"line %d: %s is out of order inside §18.%d %s (previous row was PIB-%03d)",
				number+1, row.id, section, category, previous,
			)
		}
		previous = value
		seen[row.id] = row
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
	if len(rows) != want {
		return nil, fmt.Errorf("matrix carries %d authoritative rows, want %d", len(rows), want)
	}
	for index, row := range rows {
		expected := fmt.Sprintf("PIB-%03d", index+1)
		if row.id != expected {
			return nil, fmt.Errorf(
				"sorted matrix position %d is %s, want %s (the ID sequence is gapped)",
				index, row.id, expected,
			)
		}
	}
	return rows, nil
}

// parsePIBSlicePartition reads the shipped §18.52 slice table rather than the
// prose sentence beside it.
func parsePIBSlicePartition(t *testing.T) []pibSliceSpec {
	t.Helper()
	section := pibSection1852(t)
	slices := []pibSliceSpec{}
	for _, line := range strings.Split(section, "\n") {
		match := pibSliceRow.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		count, err := strconv.Atoi(match[4])
		if err != nil {
			t.Fatal(err)
		}
		categories := []string{}
		for _, raw := range strings.Split(match[3], ",") {
			if trimmed := strings.TrimSpace(raw); trimmed != "" {
				categories = append(categories, trimmed)
			}
		}
		slices = append(slices, pibSliceSpec{
			name: match[1], categories: categories, rows: count,
		})
	}
	return slices
}

func pibSection1852(t *testing.T) string {
	t.Helper()
	body := pibPRDBody(t)
	start := strings.Index(body, "### 18.52 Counts, kinds and slice partition")
	end := strings.Index(body, "### 18.53 Sensitivity requirement")
	if start < 0 || end <= start {
		t.Fatal("PRD §18.52 was not found")
	}
	return body[start:end]
}

// parsePIBProseCategoryCounts reads the "**50 categories**: A 20, B 24, …"
// sentence. It exists only so the recomputed table can be *compared* with the
// prose; the table always wins.
func parsePIBProseCategoryCounts(t *testing.T) map[string]int {
	t.Helper()
	section := pibSection1852(t)
	start := strings.Index(section, "**50 categories**:")
	end := strings.Index(section, "**50 categories; sum")
	if start < 0 || end <= start {
		t.Fatal("§18.52 category prose was not found")
	}
	counts := map[string]int{}
	for _, match := range pibProseCategory.FindAllStringSubmatch(section[start:end], -1) {
		value, err := strconv.Atoi(match[2])
		if err != nil {
			t.Fatal(err)
		}
		counts[match[1]] = value
	}
	return counts
}

// parsePIBProseKindCounts reads only the leading "**By kind**: `I` 248, …"
// clause. The remainder of that bullet is revision history that restates older
// totals, so it is deliberately excluded.
func parsePIBProseKindCounts(t *testing.T) map[string]int {
	t.Helper()
	section := pibSection1852(t)
	start := strings.Index(section, "**By kind**:")
	if start < 0 {
		t.Fatal("§18.52 kind prose was not found")
	}
	end := strings.Index(section[start:], "Sum = 567.")
	if end < 0 {
		t.Fatal("§18.52 kind sum sentence was not found")
	}
	counts := map[string]int{}
	for _, match := range pibProseKind.FindAllStringSubmatch(section[start:start+end], -1) {
		value, err := strconv.Atoi(match[2])
		if err != nil {
			t.Fatal(err)
		}
		counts[match[1]] = value
	}
	return counts
}

// ---------------------------------------------------------------------------
// PIB-230 / PIB-231 arithmetic: recomputed from the table, then compared.
// ---------------------------------------------------------------------------

func TestPIBAggregateMatrixArithmetic(t *testing.T) {
	matrix := parsePIBMatrix(t)
	if len(matrix) != pibMatrixRowCount {
		t.Fatalf("matrix rows = %d, want %d", len(matrix), pibMatrixRowCount)
	}

	// pibParseMatrixDocument already rejects duplicates, gaps and short/long
	// matrices; this restates the contiguity contract at the call site so a
	// future parser change cannot silently drop it.
	seen := map[string]int{}
	for index, row := range matrix {
		want := fmt.Sprintf("PIB-%03d", index+1)
		if row.id != want {
			t.Fatalf("matrix row %d = %s, want %s (contiguity broken)", index, row.id, want)
		}
		seen[row.id]++
	}
	for id, count := range seen {
		if count != 1 {
			t.Fatalf("%s appears %d times in the matrix", id, count)
		}
	}

	categoryCounts := map[string]int{}
	kindCounts := map[string]int{}
	for _, row := range matrix {
		categoryCounts[row.category]++
		kindCounts[row.kind]++
	}
	if len(categoryCounts) != pibCategoryCount {
		t.Fatalf("matrix has %d categories, want %d", len(categoryCounts), pibCategoryCount)
	}
	proseCategories := parsePIBProseCategoryCounts(t)
	if len(proseCategories) != pibCategoryCount {
		t.Fatalf("§18.52 prose lists %d categories, want %d", len(proseCategories), pibCategoryCount)
	}
	for category, count := range categoryCounts {
		if proseCategories[category] != count {
			t.Fatalf("category %s: table %d, §18.52 prose %d", category, count, proseCategories[category])
		}
	}
	categorySum := 0
	for _, count := range categoryCounts {
		categorySum += count
	}
	if categorySum != pibMatrixRowCount {
		t.Fatalf("category sum = %d, want %d", categorySum, pibMatrixRowCount)
	}

	proseKinds := parsePIBProseKindCounts(t)
	wantKinds := map[string]int{"I": 248, "C": 122, "G": 123, "U": 49, "S": 25}
	if fmt.Sprint(kindCounts) != fmt.Sprint(wantKinds) {
		t.Fatalf("kind counts recomputed from the table = %v, want %v", kindCounts, wantKinds)
	}
	if fmt.Sprint(proseKinds) != fmt.Sprint(wantKinds) {
		t.Fatalf("§18.52 kind prose = %v, table says %v", proseKinds, wantKinds)
	}
	kindSum := 0
	for _, count := range kindCounts {
		kindSum += count
	}
	if kindSum != pibMatrixRowCount {
		t.Fatalf("kind sum = %d, want %d", kindSum, pibMatrixRowCount)
	}

	slices := parsePIBSlicePartition(t)
	wantSlices := []pibSliceSpec{
		{name: "S1", rows: 75}, {name: "S1b", rows: 15}, {name: "S2", rows: 24},
		{name: "S3", rows: 42}, {name: "S4", rows: 142}, {name: "S4b", rows: 17},
		{name: "S5", rows: 48}, {name: "S6", rows: 31}, {name: "S7", rows: 173},
	}
	if len(slices) != len(wantSlices) {
		t.Fatalf("§18.52 slice table has %d rows, want %d", len(slices), len(wantSlices))
	}
	assigned := map[string]string{}
	sliceSum := 0
	for index, slice := range slices {
		if slice.name != wantSlices[index].name || slice.rows != wantSlices[index].rows {
			t.Fatalf("slice row %d = %s/%d, want %s/%d",
				index, slice.name, slice.rows, wantSlices[index].name, wantSlices[index].rows)
		}
		derived := 0
		for _, category := range slice.categories {
			if owner, duplicated := assigned[category]; duplicated {
				t.Fatalf("category %s is double-assigned to %s and %s", category, owner, slice.name)
			}
			assigned[category] = slice.name
			derived += categoryCounts[category]
		}
		if derived != slice.rows {
			t.Fatalf("slice %s: categories sum to %d rows, table claims %d",
				slice.name, derived, slice.rows)
		}
		sliceSum += slice.rows
	}
	if sliceSum != pibMatrixRowCount {
		t.Fatalf("slice sum = %d, want %d", sliceSum, pibMatrixRowCount)
	}
	for category := range categoryCounts {
		if assigned[category] == "" {
			t.Fatalf("category %s is unassigned by the §18.52 slice partition", category)
		}
	}
	for category := range assigned {
		if categoryCounts[category] == 0 {
			t.Fatalf("slice partition assigns category %s, which has no matrix rows", category)
		}
	}
}

// ---------------------------------------------------------------------------
// The aggregate ledger.
// ---------------------------------------------------------------------------

type pibLedgerRow struct {
	id      string
	targets []s7FixtureTarget
}

// pibAggregateLedger returns all 567 rows in matrix order. `PIB-395`…`PIB-567`
// are taken verbatim from the accepted S7 category ledgers, so this file cannot
// restate, duplicate or weaken them.
func pibAggregateLedger(t *testing.T) []pibLedgerRow {
	t.Helper()
	rows := make([]pibLedgerRow, 0, pibMatrixRowCount)
	early := pibEarlySliceLedger()
	for number := 1; number <= 394; number++ {
		id := fmt.Sprintf("PIB-%03d", number)
		row, ok := early[id]
		if !ok {
			t.Fatalf("aggregate ledger omits %s", id)
		}
		row.id = id
		rows = append(rows, row)
	}
	for _, row := range pibS7LedgerRows(t) {
		rows = append(rows, row)
	}
	return rows
}

// pibS7LedgerRows flattens the twelve accepted S7 category ledgers in ID order.
func pibS7LedgerRows(t *testing.T) []pibLedgerRow {
	t.Helper()
	source := [][]s7AMLedgerRow{
		s7AMCoverageLedger(),
		s7ANCoverageLedger(),
		s7AOCoverageLedger(),
		s7APCoverageLedger(t),
		s7AQCoverageLedger(t),
		s7ARCoverageLedger(t),
		s7ASCoverageLedger(t),
		s7ATCoverageLedger(t),
		s7AUCoverageLedger(t),
		s7AVCoverageLedger(t),
		s7AWCoverageLedger(t),
		s7AXCoverageLedger(t),
	}
	byID := map[string][]s7FixtureTarget{}
	for _, ledger := range source {
		for _, row := range ledger {
			if _, duplicated := byID[row.id]; duplicated {
				t.Fatalf("%s is claimed by two S7 category ledgers", row.id)
			}
			byID[row.id] = row.targets
		}
	}
	if len(byID) != 173 {
		t.Fatalf("S7 category ledgers cover %d rows, want 173", len(byID))
	}
	rows := make([]pibLedgerRow, 0, 173)
	for number := 395; number <= 567; number++ {
		id := fmt.Sprintf("PIB-%03d", number)
		targets, ok := byID[id]
		if !ok || len(targets) == 0 {
			t.Fatalf("S7 category ledgers do not carry %s", id)
		}
		rows = append(rows, pibLedgerRow{id: id, targets: targets})
	}
	return rows
}

func TestPIBAggregateAcceptanceLedger(t *testing.T) {
	matrix := parsePIBMatrix(t)
	ledger := pibAggregateLedger(t)
	if len(ledger) != pibMatrixRowCount {
		t.Fatalf("aggregate ledger rows = %d, want %d", len(ledger), pibMatrixRowCount)
	}
	slices := parsePIBSlicePartition(t)
	sliceOf := map[string]string{}
	for _, slice := range slices {
		for _, category := range slice.categories {
			sliceOf[category] = slice.name
		}
	}

	cache := pibNewPackageCache()
	kindCounts := map[string]int{}
	categoryCounts := map[string]int{}
	sliceCounts := map[string]int{}
	identities := map[string]string{}
	resolvedRows := 0
	var failures []string

	for index, row := range ledger {
		authority := matrix[index]
		if row.id != authority.id {
			t.Fatalf("ledger row %d = %s, matrix says %s", index, row.id, authority.id)
		}
		kindCounts[authority.kind]++
		categoryCounts[authority.category]++
		slice := sliceOf[authority.category]
		if slice == "" {
			t.Fatalf("%s is in category %s, which no slice owns", row.id, authority.category)
		}
		sliceCounts[slice]++

		if len(row.targets) == 0 {
			t.Fatalf("%s names no acceptance target; the ledger has no blocked escape", row.id)
		}
		resolved := 0
		for _, target := range row.targets {
			key := pibTargetKey(target)
			identities[key] = identities[key] + "," + row.id
			if err := pibResolveTarget(cache, target); err != nil {
				failures = append(failures, fmt.Sprintf("%s target %s: %v", row.id, key, err))
				continue
			}
			resolved++
		}
		if resolved == 0 {
			failures = append(failures, fmt.Sprintf("%s resolved none of its %d target(s)", row.id, len(row.targets)))
			continue
		}
		resolvedRows++
	}

	// One integration body legitimately proves a handful of neighbouring rows,
	// but a ledger that funnels many rows into one broad body is exactly the
	// failure this gate exists to stop. The fan-out is therefore bounded.
	for key, owners := range identities {
		claimants := strings.Split(strings.Trim(owners, ","), ",")
		if len(claimants) > pibMaxRowsPerIdentity {
			t.Fatalf("acceptance identity %s is the evidence for %d rows (%v); the bound is %d",
				key, len(claimants), claimants, pibMaxRowsPerIdentity)
		}
	}

	if len(failures) != 0 {
		sort.Strings(failures)
		t.Fatalf("aggregate ledger resolution failed:\n%s", strings.Join(failures, "\n"))
	}

	wantKinds := map[string]int{"I": 248, "C": 122, "G": 123, "U": 49, "S": 25}
	if fmt.Sprint(kindCounts) != fmt.Sprint(wantKinds) {
		t.Fatalf("ledger kind coverage = %v, want %v", kindCounts, wantKinds)
	}
	if len(categoryCounts) != pibCategoryCount {
		t.Fatalf("ledger covers %d categories, want %d", len(categoryCounts), pibCategoryCount)
	}
	wantSlices := map[string]int{
		"S1": 75, "S1b": 15, "S2": 24, "S3": 42, "S4": 142,
		"S4b": 17, "S5": 48, "S6": 31, "S7": 173,
	}
	if fmt.Sprint(sliceCounts) != fmt.Sprint(wantSlices) {
		t.Fatalf("ledger slice coverage = %v, want %v", sliceCounts, wantSlices)
	}

	if resolvedRows != pibMatrixRowCount {
		t.Fatalf("resolved ledger rows = %d, want %d with zero blocked", resolvedRows, pibMatrixRowCount)
	}
}

func pibTargetKey(target s7FixtureTarget) string {
	return strings.Join([]string{target.dir, target.pkg, target.test, target.subtest}, "|")
}

// ---------------------------------------------------------------------------
// Resolution. The strict S7 predicate is tried first and is never relaxed for
// the S7 rows; only when it cannot bind does the aggregate resolver consider the
// two additional *binding proofs* documented below.
// ---------------------------------------------------------------------------

type pibPackageCache struct {
	mu    sync.Mutex
	root  string
	files map[string][]*ast.File
}

var pibRepoRootOnce struct {
	sync.Once
	root string
}

func pibNewPackageCache() *pibPackageCache {
	return &pibPackageCache{files: map[string][]*ast.File{}}
}

func (c *pibPackageCache) parse(dir string) ([]*ast.File, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cached, ok := c.files[dir]; ok {
		return cached, nil
	}
	if c.root == "" {
		root, err := pibRepoRoot()
		if err != nil {
			return nil, err
		}
		c.root = root
	}
	full := filepath.Join(c.root, filepath.FromSlash(dir))
	entries, err := os.ReadDir(full)
	if err != nil {
		return nil, err
	}
	fileset := token.NewFileSet()
	files := []*ast.File{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fileset, filepath.Join(full, entry.Name()), nil, 0)
		if parseErr != nil {
			return nil, parseErr
		}
		files = append(files, file)
	}
	c.files[dir] = files
	return files, nil
}

// pibRepoRoot walks up from the working directory to the module root. It does
// not take a *testing.T so the cache can be shared by helpers that return errors.
func pibRepoRoot() (string, error) {
	pibRepoRootOnce.Do(func() {
		dir, err := os.Getwd()
		if err != nil {
			return
		}
		for {
			if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
				pibRepoRootOnce.root = dir
				return
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				return
			}
			dir = parent
		}
	})
	if pibRepoRootOnce.root == "" {
		return "", fmt.Errorf("module root containing go.mod was not found")
	}
	return pibRepoRootOnce.root, nil
}

func pibResolveTarget(cache *pibPackageCache, target s7FixtureTarget) error {
	files, err := cache.parse(target.dir)
	if err != nil {
		return err
	}
	return pibResolveFixtureAST(files, target.pkg, target.test, target.subtest)
}

func pibResolveFixtureSource(source, pkg, test, subtest string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "fixture_test.go", source, 0)
	if err != nil {
		return err
	}
	return pibResolveFixtureAST([]*ast.File{file}, pkg, test, subtest)
}

// pibResolveFixtureAST is the aggregate resolver. It reproduces the accepted S7
// selection rules exactly — package match, a single declaration, a real
// `*testing.T` receiver bound to an object, literal-only subtest names — and then
// applies pibAssertBindingBody, which keeps every rejection s7AssertRunnableBody
// makes and differs in exactly three documented ways:
//
//   - helper delegation (looser, and a real binding): a body that calls a
//     package-local helper, passing this body's own receiver into a
//     `*testing.T`/`testing.TB` parameter, where the helper itself binds.
//   - own-assertion parents (looser): a body that owns a reachable assertion and
//     also registers children is not a parent-only wrapper. A parent that only
//     dispatches is still rejected.
//   - unconditional infinite loops (stricter): statements after `for { … }` with
//     no exit are unreachable, which the accepted S7 walk does not model.
//
// Because the third difference is stricter, the S7 result is not short-circuited:
// every target — S7's included — is put through this predicate.
func pibResolveFixtureAST(files []*ast.File, pkg, test, subtest string) error {
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
		return fmt.Errorf("%s resolves to %d declarations in package %s", test, len(candidates), pkg)
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
			bodies := s7SelectSubtestBodies(body, bodyReceiver, functions, testingAliases, segment)
			if len(bodies) == 0 {
				// Only an *absent* accepted selection may fall back to the
				// map-keyed form; an ambiguous accepted selection stays ambiguous.
				bodies = pibSelectKeyedSubtestBodies(
					body, bodyReceiver, functions, testingAliases, segment,
				)
			}
			if len(bodies) != 1 {
				return fmt.Errorf(
					"%s/%s resolves segment %q to %d runnable bodies",
					test, subtest, segment, len(bodies),
				)
			}
			body = bodies[0].body
			bodyReceiver = bodies[0].receiver
		}
	}
	if err := pibAssertBindingBody(body, bodyReceiver, functions, testingAliases); err != nil {
		return fmt.Errorf("%s/%s: %w", test, subtest, err)
	}
	return nil
}

// pibSelectKeyedSubtestBodies handles `for id := range <map literal> { t.Run(id, …) }`,
// the shape the frozen pre-change golden suite uses. The case name still has to
// be a *string literal key* of a composite literal that the range expression
// resolves to, and it must occur exactly once, so a computed or duplicated name
// is still rejected.
func pibSelectKeyedSubtestBodies(
	body *ast.BlockStmt,
	receiver *ast.Ident,
	functions map[string][]*ast.FuncDecl,
	testingAliases map[string]bool,
	subtest string,
) []s7SelectedBody {
	var bodies []s7SelectedBody
	s7InspectRegistrationSyntax(body, func(node ast.Node) bool {
		rangeStmt, ok := node.(*ast.RangeStmt)
		if !ok {
			return true
		}
		keyIdent, ok := rangeStmt.Key.(*ast.Ident)
		if !ok || keyIdent.Name == "_" {
			return true
		}
		if !pibCompositeContainsKey(s7ResolveTableExpression(rangeStmt.X), subtest) {
			return true
		}
		s7InspectRegistrationSyntax(rangeStmt.Body, func(inner ast.Node) bool {
			call, ok := inner.(*ast.CallExpr)
			if !ok || !s7RunCallOnReceiver(call, receiver) {
				return true
			}
			named, ok := call.Args[0].(*ast.Ident)
			if !ok || named.Name != keyIdent.Name {
				return true
			}
			bodies = append(bodies, s7CallbackBodies(call.Args[1], functions, testingAliases)...)
			return true
		})
		return true
	})
	return bodies
}

func pibCompositeContainsKey(expression ast.Expr, subtest string) bool {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return false
	}
	matches := 0
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := pair.Key.(*ast.BasicLit)
		if !ok || key.Kind != token.STRING {
			continue
		}
		decoded, err := strconv.Unquote(key.Value)
		if err == nil && decoded == subtest {
			matches++
		}
	}
	return matches == 1
}

// pibAssertBindingBody keeps every rejection s7AssertRunnableBody makes and adds
// the helper-delegation binding proof.
func pibAssertBindingBody(
	body *ast.BlockStmt,
	receiver *ast.Ident,
	functions map[string][]*ast.FuncDecl,
	testingAliases map[string]bool,
) error {
	if body == nil || len(body.List) == 0 {
		return fmt.Errorf("empty body")
	}
	assertions, skips, subtests := pibCountReceiverCalls(body, receiver)
	if skips != 0 {
		return fmt.Errorf("selected body can skip")
	}
	if assertions == 0 {
		if !pibBodyBindsThroughHelper(body, receiver, functions, testingAliases, map[string]bool{}, 0) {
			if subtests != 0 {
				return fmt.Errorf(
					"selected body is a parent-only aggregate wrapper with %d nested subtest(s)", subtests,
				)
			}
			return fmt.Errorf("selected body has no reachable binding assertion")
		}
		if subtests != 0 {
			return fmt.Errorf(
				"selected body is a parent-only aggregate wrapper with %d nested subtest(s)", subtests,
			)
		}
	}
	return nil
}

func pibCountReceiverCalls(body *ast.BlockStmt, receiver *ast.Ident) (int, int, int) {
	assertions, skips, subtests := 0, 0, 0
	s7InspectReachableBody(pibTruncateAtInfiniteLoop(body), func(node ast.Node) bool {
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
	})
	return assertions, skips, subtests
}

// pibTruncateAtInfiniteLoop closes the one reachability gap the accepted S7 walk
// leaves open: `for { … }` with no condition and no way out never falls through,
// so nothing after it is reachable. Statements past such a loop are dropped
// before the walk, which is why the "infinite loop before assertion" fixture bites.
func pibTruncateAtInfiniteLoop(body *ast.BlockStmt) *ast.BlockStmt {
	if body == nil {
		return nil
	}
	for index, statement := range body.List {
		loop, ok := statement.(*ast.ForStmt)
		if !ok || loop.Cond != nil || loop.Init != nil || loop.Post != nil {
			continue
		}
		if pibLoopCanExit(loop.Body) {
			continue
		}
		return &ast.BlockStmt{List: body.List[:index+1]}
	}
	return body
}

func pibLoopCanExit(body *ast.BlockStmt) bool {
	if body == nil {
		return true
	}
	escapes := false
	ast.Inspect(body, func(node ast.Node) bool {
		if escapes {
			return false
		}
		switch typed := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			escapes = true
		case *ast.BranchStmt:
			if typed.Tok == token.BREAK || typed.Tok == token.GOTO {
				escapes = true
			}
		case *ast.CallExpr:
			selector, ok := typed.Fun.(*ast.SelectorExpr)
			if ok {
				switch selector.Sel.Name {
				case "Fatal", "Fatalf", "FailNow", "Skip", "Skipf", "SkipNow", "Exit":
					escapes = true
				}
			}
		}
		return true
	})
	return escapes
}

// pibBodyBindsThroughHelper proves a binding through a package-local helper.
// The helper must (a) be declared exactly once in this package, (b) receive this
// body's own receiver identifier at a parameter position whose type is
// `*testing.T` or `testing.TB` under an imported `testing` alias, and (c) itself
// bind. Depth is bounded and cycles are cut, so a helper that never asserts, a
// helper in another package, and a call that passes something other than the
// receiver are all still rejected.
func pibBodyBindsThroughHelper(
	body *ast.BlockStmt,
	receiver *ast.Ident,
	functions map[string][]*ast.FuncDecl,
	testingAliases map[string]bool,
	visiting map[string]bool,
	depth int,
) bool {
	if depth > 4 {
		return false
	}
	bound := false
	s7InspectReachableBody(body, func(node ast.Node) bool {
		if bound {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		callee, ok := call.Fun.(*ast.Ident)
		if !ok {
			return true
		}
		declarations := functions[callee.Name]
		if len(declarations) != 1 {
			return true
		}
		positions := map[int]bool{}
		for index, argument := range call.Args {
			identifier, ok := argument.(*ast.Ident)
			if ok && identifier.Obj == receiver.Obj {
				positions[index] = true
			}
		}
		if len(positions) == 0 {
			return true
		}
		if visiting[callee.Name] {
			return true
		}
		visiting[callee.Name] = true
		defer delete(visiting, callee.Name)
		if pibHelperBinds(declarations[0], positions, functions, testingAliases, visiting, depth+1) {
			bound = true
			return false
		}
		return true
	})
	return bound
}

func pibHelperBinds(
	function *ast.FuncDecl,
	positions map[int]bool,
	functions map[string][]*ast.FuncDecl,
	testingAliases map[string]bool,
	visiting map[string]bool,
	depth int,
) bool {
	if function.Body == nil || function.Type.Params == nil {
		return false
	}
	index := 0
	for _, field := range function.Type.Params.List {
		names := field.Names
		if len(names) == 0 {
			index++
			continue
		}
		for _, name := range names {
			if positions[index] && pibIsTestingParameter(field.Type, testingAliases) && name.Obj != nil {
				assertions, _, _ := pibCountReceiverCalls(function.Body, name)
				if assertions > 0 {
					return true
				}
				if pibBodyBindsThroughHelper(
					function.Body, name, functions, testingAliases, visiting, depth,
				) {
					return true
				}
			}
			index++
		}
	}
	return false
}

func pibIsTestingParameter(expression ast.Expr, testingAliases map[string]bool) bool {
	switch typed := expression.(type) {
	case *ast.StarExpr:
		selector, ok := typed.X.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "T" {
			return false
		}
		alias, ok := selector.X.(*ast.Ident)
		return ok && testingAliases[alias.Name]
	case *ast.SelectorExpr:
		if typed.Sel.Name != "TB" {
			return false
		}
		alias, ok := typed.X.(*ast.Ident)
		return ok && testingAliases[alias.Name]
	}
	return false
}

// ---------------------------------------------------------------------------
// PIB-231: the G-row sensitivity registry, derived from the Kind column.
// ---------------------------------------------------------------------------

type pibSensitivityEntry struct {
	// sensitivity is the body that mutates the guarded input and asserts the
	// *same* validator that accepts the shipped input rejects the mutation.
	// There is no blocked escape: every derived `G` row must name one.
	sensitivity s7FixtureTarget
	// validator names the symbol whose result the fixture must assert. For a
	// direct fixture it is the row's own validator function — `validateFoo`,
	// `pkg.Decode…`, a package-local `s7XXValidate…`. For the S6 witness pairs it
	// is the registry the row's guard/sensitivity closures are drawn from
	// (`s6GuardSpecs`), and the gate additionally requires the lookup key to be
	// the row's own ID.
	//
	// Structural uniqueness is not semantic ownership: without this field a row
	// can be pointed at a *sibling's* wrong-input body, which resolves, carries a
	// mutation and asserts a rejection — but proves the sibling's claim. The gate
	// binds the row to this symbol, so a rotated target fails even though its
	// body is a perfectly good fixture for somebody else.
	validator string
}

func TestPIBGuardSensitivityRegistry(t *testing.T) {
	matrix := parsePIBMatrix(t)
	derived := []string{}
	for _, row := range matrix {
		if strings.Contains(row.kind, "G") {
			derived = append(derived, row.id)
		}
	}
	if len(derived) != 123 {
		t.Fatalf("Kind column derives %d G rows, §18.52 says 123", len(derived))
	}

	registry := pibGuardSensitivityRegistry()
	if err := pibValidateRegistryKeys(derived, registry); err != nil {
		t.Fatal(err)
	}
	if err := pibValidateRegistryUniqueness(registry); err != nil {
		t.Fatal(err)
	}

	cache := pibNewPackageCache()
	live := 0
	var failures []string
	for _, id := range derived {
		entry := registry[id]
		if entry.sensitivity == (s7FixtureTarget{}) {
			failures = append(failures, fmt.Sprintf("%s names no sensitivity target", id))
			continue
		}
		key := pibTargetKey(entry.sensitivity)
		if err := pibResolveTarget(cache, entry.sensitivity); err != nil {
			failures = append(failures, fmt.Sprintf("%s sensitivity %s: %v", id, key, err))
			continue
		}
		if err := pibAssertSensitivityShape(cache, id, entry); err != nil {
			failures = append(failures, fmt.Sprintf("%s sensitivity %s: %v", id, key, err))
			continue
		}
		live++
	}
	if len(failures) != 0 {
		sort.Strings(failures)
		t.Fatalf("G sensitivity registry failed:\n%s", strings.Join(failures, "\n"))
	}
	if live != 123 {
		t.Fatalf("live sensitivity entries = %d, want 123 with zero blocked", live)
	}
}

func pibContains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// The §18.53 proof gate.
//
// rev-3's gate asked two things of a sensitivity body: that *some* `if` whose
// condition mentioned `== nil` reached an assertion, and that *somewhere* in the
// body — or anywhere in the enclosing test — an `append`, a `copy`, a
// `fmt.Sprintf` or any index/field write appeared. Both are satisfied by code
// that proves nothing: `if report.Refusal == nil { t.Fatal(…) }` is a *baseline*
// check, ordinary setup uses `append`, and a subtest inherited its parent's
// siblings' mutations.
//
// The gate now requires a data-flow link instead:
//
//  1. Leaf-only. A subtest target is judged on the selected leaf body alone —
//     never on a sibling or on the enclosing function. A top-level target is
//     judged on its own body (its own closures included).
//  2. Wrong input reaches the validator. A value produced by a recognized
//     wrong-input derivation — a text substitution over real bytes, a clone
//     followed by a specific key/field write, a truncation, a concatenated
//     literal, a `…Mutate…`/`…Inject…` helper, or the fixture-table field the
//     leaf was registered with — must flow into the very call whose result is
//     asserted. Common setup `append`/`fmt.Sprintf`/field writes prove nothing
//     on their own.
//  3. Rejecting direction, dominating a failure. That call must be the row's
//     declared validator, its result must be compared `== nil`, and the branch
//     that comparison guards must fail the test. A `!= nil` baseline check, an
//     `== nil` check on something the validator did not produce, and an ignored
//     error are all rejected.
//  4. Row-bound identity. The validator is named per row by the registry, so a
//     structurally-perfect fixture that belongs to a *different* row does not
//     satisfy this row.
//
// The S6 witness pairs are recognized as a fifth shape: a row that draws its
// baseline and wrong-input closures from one registry entry keyed by its own ID
// executes both through the same validator, which is exactly what the direct
// shape demands — and the key literal binds the row.
// ---------------------------------------------------------------------------

// pibSelectedSensitivity is the leaf the gate judges, plus the fixture-table
// element the leaf was registered with (when it was registered from a table).
type pibSelectedSensitivity struct {
	body     *ast.BlockStmt
	receiver *ast.Ident
	// injected maps a range-variable name to the composite literal element that
	// produced *this* leaf. `fixture.command` is a wrong input only when this
	// element sets `command` to a derived expression, not to a baseline variable.
	injected map[string]*ast.CompositeLit
}

func pibAssertSensitivityShape(cache *pibPackageCache, id string, entry pibSensitivityEntry) error {
	files, err := cache.parse(entry.sensitivity.dir)
	if err != nil {
		return err
	}
	return pibAssertSensitivityShapeFiles(files, id, entry)
}

// pibAssertSensitivityShapeSource runs the same gate over a synthetic source, so
// the meta fixtures can prove each rejection without shipping a fake row.
func pibAssertSensitivityShapeSource(source, id string, entry pibSensitivityEntry) error {
	file, err := parser.ParseFile(token.NewFileSet(), "fixture_test.go", source, 0)
	if err != nil {
		return err
	}
	return pibAssertSensitivityShapeFiles([]*ast.File{file}, id, entry)
}

func pibAssertSensitivityShapeFiles(files []*ast.File, id string, entry pibSensitivityEntry) error {
	if entry.validator == "" {
		return fmt.Errorf("%s names no expected validator family", id)
	}
	selected, err := pibSelectSensitivityBody(files, entry.sensitivity)
	if err != nil {
		return err
	}
	if pibWitnessPairProves(selected, entry.validator, id) {
		return nil
	}
	rejections := pibValidatorRejections(selected, entry.validator)
	if len(rejections) == 0 {
		return fmt.Errorf(
			"no %s result is compared == nil in a branch that fails the test", entry.validator)
	}
	tainted := pibMutatedValues(selected)
	for _, call := range rejections {
		if pibCallCarriesWrongInput(call, selected, tainted) {
			return nil
		}
	}
	return fmt.Errorf(
		"the asserted %s rejection receives no mutated input: no wrong-input derivation reaches it",
		entry.validator)
}

func pibPackageDeclarations(
	files []*ast.File, pkg string,
) (map[string][]*ast.FuncDecl, map[string]bool) {
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
	return functions, testingAliases
}

// pibSelectSensitivityBody resolves the target to the leaf body the gate judges.
// Every subtest segment narrows the selection; nothing outside the final leaf is
// carried forward except the fixture-table element that produced it, which is
// one of the leaf's own inputs rather than a sibling's mutation.
func pibSelectSensitivityBody(
	files []*ast.File, target s7FixtureTarget,
) (pibSelectedSensitivity, error) {
	functions, testingAliases := pibPackageDeclarations(files, target.pkg)
	candidates := functions[target.test]
	if len(candidates) != 1 {
		return pibSelectedSensitivity{}, fmt.Errorf(
			"%s resolves to %d declarations", target.test, len(candidates))
	}
	function := candidates[0]
	receiver, err := s7TestingReceiver(function, testingAliases)
	if err != nil {
		return pibSelectedSensitivity{}, err
	}
	selected := pibSelectedSensitivity{
		body:     function.Body,
		receiver: receiver,
		injected: map[string]*ast.CompositeLit{},
	}
	if target.subtest == "" {
		return selected, nil
	}
	for _, segment := range strings.Split(target.subtest, "/") {
		parent := selected
		bodies := s7SelectSubtestBodies(
			parent.body, parent.receiver, functions, testingAliases, segment)
		if len(bodies) == 0 {
			bodies = pibSelectKeyedSubtestBodies(
				parent.body, parent.receiver, functions, testingAliases, segment)
		}
		if len(bodies) != 1 {
			return pibSelectedSensitivity{}, fmt.Errorf(
				"%s/%s resolves segment %q to %d bodies",
				target.test, target.subtest, segment, len(bodies))
		}
		selected = pibSelectedSensitivity{
			body:     bodies[0].body,
			receiver: bodies[0].receiver,
			injected: pibFixtureInjection(parent.body, parent.receiver, segment),
		}
	}
	return selected, nil
}

// pibFixtureInjection binds a table-registered leaf to the element it was
// registered with: `for _, fixture := range fixtures { t.Run(fixture.name, …) }`
// makes `fixture` the leaf's injected input, and the element whose `name` is this
// subtest is the one the leaf actually receives.
func pibFixtureInjection(
	parent *ast.BlockStmt, receiver *ast.Ident, subtest string,
) map[string]*ast.CompositeLit {
	injected := map[string]*ast.CompositeLit{}
	if parent == nil {
		return injected
	}
	s7InspectRegistrationSyntax(parent, func(node ast.Node) bool {
		rangeStmt, ok := node.(*ast.RangeStmt)
		if !ok {
			return true
		}
		element := pibTableElement(s7ResolveTableExpression(rangeStmt.X), subtest)
		if element == nil {
			return true
		}
		registers := false
		s7InspectRegistrationSyntax(rangeStmt.Body, func(inner ast.Node) bool {
			call, ok := inner.(*ast.CallExpr)
			if !ok || !s7RunCallOnReceiver(call, receiver) {
				return true
			}
			registers = true
			return true
		})
		if !registers {
			return true
		}
		if value, ok := rangeStmt.Value.(*ast.Ident); ok && value.Name != "_" {
			injected[value.Name] = element
		}
		if key, ok := rangeStmt.Key.(*ast.Ident); ok && key.Name != "_" {
			injected[key.Name] = element
		}
		return true
	})
	return injected
}

// pibTableElement returns the composite-literal element a table registers under
// `subtest`, either as a `name:` field or as a string map key.
func pibTableElement(expression ast.Expr, subtest string) *ast.CompositeLit {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	var matched *ast.CompositeLit
	matches := 0
	for _, element := range literal.Elts {
		switch typed := element.(type) {
		case *ast.KeyValueExpr:
			key, ok := typed.Key.(*ast.BasicLit)
			if ok && key.Kind == token.STRING {
				if decoded, err := strconv.Unquote(key.Value); err == nil && decoded == subtest {
					matches++
					if value, ok := typed.Value.(*ast.CompositeLit); ok {
						matched = value
					}
				}
				continue
			}
			if value, ok := typed.Value.(*ast.CompositeLit); ok && pibElementNames(value, subtest) {
				matches++
				matched = value
			}
		case *ast.CompositeLit:
			if pibElementNames(typed, subtest) {
				matches++
				matched = typed
			}
		}
	}
	if matches != 1 {
		return nil
	}
	return matched
}

func pibElementNames(element *ast.CompositeLit, subtest string) bool {
	for _, field := range element.Elts {
		pair, ok := field.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := pair.Key.(*ast.Ident)
		if !ok || key.Name != "name" {
			continue
		}
		literal, ok := pair.Value.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			continue
		}
		if decoded, err := strconv.Unquote(literal.Value); err == nil && decoded == subtest {
			return true
		}
	}
	return false
}

// pibValidatorRejections returns every call to the declared validator whose
// result is compared `== nil` in a branch that fails the test. A `!= nil`
// baseline check, an `== nil` check on a report field, and a comparison whose
// branch never fails are all excluded, so "the validator accepted the wrong
// input" is the only shape that counts.
func pibValidatorRejections(selected pibSelectedSensitivity, validator string) []*ast.CallExpr {
	produced := pibProducedValues(selected.body, validator)
	found := []*ast.CallExpr{}
	ast.Inspect(selected.body, func(node ast.Node) bool {
		statement, ok := node.(*ast.IfStmt)
		if !ok {
			return true
		}
		if !pibBodyFailsTest(statement.Body, selected.receiver) {
			return true
		}
		for _, compared := range pibNilComparisons(statement.Cond, token.EQL) {
			switch typed := compared.(type) {
			case *ast.CallExpr:
				if pibCalleeMatches(typed, validator) {
					found = append(found, typed)
				}
			case *ast.Ident:
				if assignment, ok := pibInitAssigns(statement.Init, typed.Name, validator); ok {
					found = append(found, assignment)
					continue
				}
				found = append(found, produced[typed.Name]...)
			}
		}
		return true
	})
	return found
}

// pibProducedValues maps every variable the body assigns from a call to the
// declared validator to that call, so `err := validate(x)` followed by
// `if err == nil { t.Fatal(…) }` is understood as one rejection assertion.
func pibProducedValues(body *ast.BlockStmt, validator string) map[string][]*ast.CallExpr {
	produced := map[string][]*ast.CallExpr{}
	ast.Inspect(body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, right := range assignment.Rhs {
			call, ok := right.(*ast.CallExpr)
			if !ok || !pibCalleeMatches(call, validator) {
				continue
			}
			for _, left := range assignment.Lhs {
				if identifier, ok := left.(*ast.Ident); ok && identifier.Name != "_" {
					produced[identifier.Name] = append(produced[identifier.Name], call)
				}
			}
		}
		return true
	})
	return produced
}

func pibInitAssigns(init ast.Stmt, name, validator string) (*ast.CallExpr, bool) {
	assignment, ok := init.(*ast.AssignStmt)
	if !ok {
		return nil, false
	}
	bound := false
	for _, left := range assignment.Lhs {
		if identifier, ok := left.(*ast.Ident); ok && identifier.Name == name {
			bound = true
		}
	}
	if !bound {
		return nil, false
	}
	for _, right := range assignment.Rhs {
		if call, ok := right.(*ast.CallExpr); ok && pibCalleeMatches(call, validator) {
			return call, true
		}
	}
	return nil, false
}

// pibNilComparisons returns the expressions a condition compares to nil with the
// given operator.
func pibNilComparisons(condition ast.Expr, operator token.Token) []ast.Expr {
	compared := []ast.Expr{}
	ast.Inspect(condition, func(node ast.Node) bool {
		binary, ok := node.(*ast.BinaryExpr)
		if !ok || binary.Op != operator {
			return true
		}
		if identifier, ok := binary.Y.(*ast.Ident); ok && identifier.Name == "nil" {
			compared = append(compared, binary.X)
		}
		if identifier, ok := binary.X.(*ast.Ident); ok && identifier.Name == "nil" {
			compared = append(compared, binary.Y)
		}
		return true
	})
	return compared
}

// pibBodyFailsTest reports whether a branch fails the test through the leaf's own
// receiver. An `== nil` comparison that only logs, or that hands the result to
// something else, does not count as an assertion.
func pibBodyFailsTest(body *ast.BlockStmt, receiver *ast.Ident) bool {
	if body == nil || receiver == nil {
		return false
	}
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if found {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch selector.Sel.Name {
		case "Fatal", "Fatalf", "Error", "Errorf", "Fail", "FailNow":
		default:
			return true
		}
		base, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		if base.Obj == receiver.Obj || base.Name == receiver.Name {
			found = true
		}
		return true
	})
	return found
}

func pibCalleeMatches(call *ast.CallExpr, validator string) bool {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name == validator
	case *ast.SelectorExpr:
		if fun.Sel.Name == validator {
			return true
		}
		if base, ok := fun.X.(*ast.Ident); ok {
			return base.Name+"."+fun.Sel.Name == validator
		}
	}
	return false
}

// pibMutationSelectors is the closed set of package-qualified derivations that
// turn real bytes into a deliberately wrong version of themselves.
var pibMutationSelectors = map[string]map[string]bool{
	"strings": {
		"Replace": true, "ReplaceAll": true, "NewReplacer": true, "Repeat": true,
		"TrimPrefix": true, "TrimSuffix": true, "Join": true, "ToUpper": true,
		"ToLower": true, "Map": true,
	},
	"bytes": {
		"Replace": true, "ReplaceAll": true, "TrimPrefix": true, "TrimSuffix": true,
		"Join": true, "Repeat": true,
	},
	"fmt": {"Sprintf": true, "Sprint": true},
}

// pibMutationVerbs are the name fragments a wrong-input helper carries. They are
// matched on the callee's own identifier, never on a comment or a row name.
var pibMutationVerbs = []string{"utat", "nject", "eplace", "orrupt", "ensitivity"}

func pibIsMutationCallee(callee ast.Expr) bool {
	switch fun := callee.(type) {
	case *ast.Ident:
		switch fun.Name {
		case "append", "delete", "copy":
			return true
		}
		return pibNameCarriesMutationVerb(fun.Name)
	case *ast.SelectorExpr:
		if base, ok := fun.X.(*ast.Ident); ok && pibMutationSelectors[base.Name][fun.Sel.Name] {
			return true
		}
		return pibNameCarriesMutationVerb(fun.Sel.Name)
	}
	return false
}

func pibNameCarriesMutationVerb(name string) bool {
	for _, verb := range pibMutationVerbs {
		if strings.Contains(name, verb) {
			return true
		}
	}
	return false
}

// pibMutatedValues computes, over the leaf body alone, the set of value names
// that carry a wrong-input derivation. Writes into an existing value
// (`clone[key] = …`, `wrong.field = …`, `body += …`), text substitutions,
// truncations, literal concatenations, `…Mutate…`-shaped helpers and in-body
// fixture tables all seed the set; ordinary assignments propagate it.
func pibMutatedValues(selected pibSelectedSensitivity) map[string]bool {
	tainted := map[string]bool{}
	for round := 0; round < 4; round++ {
		changed := false
		mark := func(name string) {
			if name == "" || name == "_" || tainted[name] {
				return
			}
			tainted[name] = true
			changed = true
		}
		ast.Inspect(selected.body, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.AssignStmt:
				derived := false
				for _, right := range typed.Rhs {
					if pibIsDerivedExpression(right, selected, tainted) {
						derived = true
					}
				}
				for _, left := range typed.Lhs {
					if root := pibWriteRoot(left); root != "" {
						mark(root)
						continue
					}
					if identifier, ok := left.(*ast.Ident); ok && derived {
						mark(identifier.Name)
					}
				}
			case *ast.IncDecStmt:
				if root := pibWriteRoot(typed.X); root != "" {
					mark(root)
				}
			case *ast.ExprStmt:
				call, ok := typed.X.(*ast.CallExpr)
				if !ok || !pibIsMutationCallee(call.Fun) {
					return true
				}
				for _, argument := range call.Args {
					mark(pibValueRoot(argument))
				}
			case *ast.RangeStmt:
				if _, ok := s7ResolveTableExpression(typed.X).(*ast.CompositeLit); !ok {
					return true
				}
				if value, ok := typed.Value.(*ast.Ident); ok {
					mark(value.Name)
				}
			}
			return true
		})
		if !changed {
			break
		}
	}
	return tainted
}

// pibWriteRoot returns the value a statement writes *into* — the map behind
// `clone[key] = …`, the struct behind `wrong.field = …`, the pointee behind
// `*target = …`. A plain identifier is not a write into an existing value.
func pibWriteRoot(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.IndexExpr, *ast.SelectorExpr, *ast.StarExpr:
		return pibValueRoot(typed)
	}
	return ""
}

func pibValueRoot(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.IndexExpr:
		return pibValueRoot(typed.X)
	case *ast.SelectorExpr:
		return pibValueRoot(typed.X)
	case *ast.StarExpr:
		return pibValueRoot(typed.X)
	case *ast.UnaryExpr:
		return pibValueRoot(typed.X)
	case *ast.SliceExpr:
		return pibValueRoot(typed.X)
	case *ast.ParenExpr:
		return pibValueRoot(typed.X)
	}
	return ""
}

// pibIsDerivedExpression reports whether an expression produces a deliberately
// wrong version of a real input.
func pibIsDerivedExpression(
	expression ast.Expr, selected pibSelectedSensitivity, tainted map[string]bool,
) bool {
	derived := false
	ast.Inspect(expression, func(node ast.Node) bool {
		if derived {
			return false
		}
		switch typed := node.(type) {
		case *ast.CallExpr:
			if pibIsMutationCallee(typed.Fun) {
				derived = true
			}
		case *ast.BinaryExpr:
			if typed.Op != token.ADD {
				return true
			}
			for _, side := range []ast.Expr{typed.X, typed.Y} {
				if literal, ok := side.(*ast.BasicLit); ok && literal.Kind == token.STRING {
					derived = true
				}
			}
		case *ast.SliceExpr:
			derived = true
		case *ast.KeyValueExpr:
			// A composite-literal key is a field name, not a value reference.
			if pibIsDerivedExpression(typed.Value, selected, tainted) {
				derived = true
			}
			return false
		case *ast.SelectorExpr:
			if pibInjectedFieldIsDerived(typed, selected) {
				derived = true
				return false
			}
			// The selected field name is not a value reference either; only the
			// base of the selector is.
			if pibIsDerivedExpression(typed.X, selected, tainted) {
				derived = true
			}
			return false
		case *ast.Ident:
			if tainted[typed.Name] {
				derived = true
			}
		}
		return true
	})
	return derived
}

// pibInjectedFieldIsDerived reports whether `fixture.field` is a wrong input:
// the leaf must have been registered from a fixture table, and *this* leaf's
// element must set that field to a derived expression rather than to the
// baseline variable every other element passes through.
func pibInjectedFieldIsDerived(
	selector *ast.SelectorExpr, selected pibSelectedSensitivity,
) bool {
	base, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	element, registered := selected.injected[base.Name]
	if !registered || element == nil {
		return false
	}
	for _, field := range element.Elts {
		pair, ok := field.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := pair.Key.(*ast.Ident)
		if !ok || key.Name != selector.Sel.Name {
			continue
		}
		switch pair.Value.(type) {
		case *ast.Ident, *ast.SelectorExpr:
			// The element passes the baseline value straight through.
			return false
		}
		return true
	}
	return false
}

// pibCallCarriesWrongInput reports whether the asserted validator call actually
// receives a wrong input. This is the data-flow link the gate exists for: a body
// may mutate whatever it likes, but unless the mutated value reaches *this* call
// the assertion proves nothing.
func pibCallCarriesWrongInput(
	call *ast.CallExpr, selected pibSelectedSensitivity, tainted map[string]bool,
) bool {
	for _, argument := range call.Args {
		if pibIsDerivedExpression(argument, selected, tainted) {
			return true
		}
	}
	return false
}

// pibWitnessPairProves recognizes the standardized witness pair: the row looks
// its guard up in a registry *by its own ID*, requires the registry's baseline
// closure to be accepted, and requires the registry's wrong-input closure to be
// rejected. Both closures run the row's validator over the row's own inputs, so
// this is the direct shape with the mutation moved inside the registry entry —
// and the key literal is what binds the row to it.
func pibWitnessPairProves(selected pibSelectedSensitivity, registry, id string) bool {
	holders := map[string]bool{}
	ast.Inspect(selected.body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, right := range assignment.Rhs {
			index, ok := right.(*ast.IndexExpr)
			if !ok {
				continue
			}
			base, ok := index.X.(*ast.Ident)
			if !ok || base.Name != registry {
				continue
			}
			key, ok := index.Index.(*ast.BasicLit)
			if !ok || key.Kind != token.STRING {
				continue
			}
			decoded, err := strconv.Unquote(key.Value)
			if err != nil || decoded != id {
				continue
			}
			for _, left := range assignment.Lhs {
				if identifier, ok := left.(*ast.Ident); ok && identifier.Name != "_" {
					holders[identifier.Name] = true
				}
			}
		}
		return true
	})
	if len(holders) == 0 {
		return false
	}
	for holder := range holders {
		if pibWitnessAssertion(selected, holder, "sensitivity", token.EQL) &&
			pibWitnessAssertion(selected, holder, "run", token.NEQ) {
			return true
		}
	}
	return false
}

// pibWitnessAssertion reports whether the body compares `holder.method(…)`'s
// result to nil with the given operator in a branch that fails the test.
func pibWitnessAssertion(
	selected pibSelectedSensitivity, holder, method string, operator token.Token,
) bool {
	qualified := holder + "." + method
	produced := pibProducedValues(selected.body, qualified)
	found := false
	ast.Inspect(selected.body, func(node ast.Node) bool {
		if found {
			return false
		}
		statement, ok := node.(*ast.IfStmt)
		if !ok || !pibBodyFailsTest(statement.Body, selected.receiver) {
			return true
		}
		for _, compared := range pibNilComparisons(statement.Cond, operator) {
			switch typed := compared.(type) {
			case *ast.CallExpr:
				if pibCalleeMatches(typed, qualified) {
					found = true
				}
			case *ast.Ident:
				if _, ok := pibInitAssigns(statement.Init, typed.Name, qualified); ok {
					found = true
					continue
				}
				if len(produced[typed.Name]) != 0 {
					found = true
				}
			}
		}
		return true
	})
	return found
}
