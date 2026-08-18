package intent

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Acceptance-matrix parsing (shared by AVP-139, AVP-202 and the ledger)
// ---------------------------------------------------------------------------

type matrixRow struct {
	ID       string
	Kind     string
	Case     string
	Observed string
	Category string
}

var matrixRowRE = regexp.MustCompile(`^\| (AVP-\d{3}) \| ([A-Z+]+) \| (.*?) \| (.*) \|$`)
var matrixSectionRE = regexp.MustCompile(`^### 18\.\d+ ([A-Z]) — `)

func acceptancePRDPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRootDir(t), "docs", "prds", "PRD-artifact-validation-and-provenance.md")
}

func parseAcceptanceMatrix(t *testing.T) []matrixRow {
	t.Helper()
	data, err := os.ReadFile(acceptancePRDPath(t))
	if err != nil {
		t.Fatalf("read PRD: %v", err)
	}
	var rows []matrixRow
	category := ""
	for _, line := range strings.Split(string(data), "\n") {
		if match := matrixSectionRE.FindStringSubmatch(line); match != nil {
			category = match[1]
			continue
		}
		if match := matrixRowRE.FindStringSubmatch(strings.TrimRight(line, " ")); match != nil {
			rows = append(rows, matrixRow{
				ID: match[1], Kind: match[2], Case: match[3],
				Observed: match[4], Category: category,
			})
		}
	}
	if len(rows) == 0 {
		t.Fatal("no acceptance rows parsed — every matrix guard would be vacuous")
	}
	return rows
}

// ---------------------------------------------------------------------------
// Shipped-string inventory
// ---------------------------------------------------------------------------

var skillSurfaces = []string{
	"assets/skills/claude/tessera-patch/SKILL.md",
	"assets/skills/copilot/tessera-patch/SKILL.md",
	"assets/skills/cursor/tessera-patch.mdc",
	"assets/skills/windsurf/windsurfrules",
	"assets/workflows/tessera-patch-generic.md",
	"assets/prompts/copilot/tessera-patch-apply.prompt.md",
}

// shippedStrings collects every byte this command can put in front of an
// operator: help text, both report renderers over the full matrix, every
// advisory and abort message, every `error:` line, and the six skill surfaces.
func shippedStrings(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, report := range guardReportMatrix(t) {
		out = append(out, renderAllSurfaces(report))
	}
	for _, code := range AbortCodes() {
		out = append(out, abortMessage(code, testSlug))
		out = append(out, lifecycleAnnotation(NewAbortReport(testSlug, code)))
	}
	for _, state := range ArtifactStates() {
		if advisory := sidecarAdvisory(state); advisory != nil {
			out = append(out, advisory.Message)
		}
		out = append(out, remediation(artifactSpecs[3], testSidecar, testSlug, state))
		if state != StateInvalidStructured {
			out = append(out, remediation(artifactSpecs[1], testSpec, testSlug, state))
		}
	}
	out = append(out, disclaimer)
	prepareSource := repoFile(t, "internal/cli/prepare.go")
	for _, literal := range extractStringLiterals(prepareSource) {
		out = append(out, literal)
	}
	for _, rel := range skillSurfaces {
		out = append(out, repoFile(t, rel))
	}
	return out
}

var stringLiteralRE = regexp.MustCompile("`[^`]*`|\"(?:[^\"\\\\]|\\\\.)*\"")

func extractStringLiterals(source string) []string {
	var out []string
	for _, match := range stringLiteralRE.FindAllString(source, -1) {
		if strings.HasPrefix(match, "`") {
			out = append(out, strings.Trim(match, "`"))
			continue
		}
		if unquoted, err := strconv.Unquote(match); err == nil {
			out = append(out, unquoted)
		}
	}
	return out
}

var withdrawalMarkers = []string{
	"withdrawn", "withdraws", "withdrawal", "rejected", "removed", "must not",
	"may not", "may claim", "no document may", "forbidden", "no acceptance row",
	"does not claim", "never claims", "erratum", "errata",
	// Guard-describing prose is a quotation of the forbidden phrasing, not
	// an assertion of it. These markers are the ones the two documents
	// actually use when they state what a guard rejects.
	"no sentence", "finds no", "is a defect", "that claims", "which claims",
	"never asserts", "does not assert", "sensitivity fixture", "fails the guard",
	"prohibited", "claiming", "quote", "quoted", "reinsert", "reinserting",
}

func splitSentences(text string) []string {
	replaced := strings.NewReplacer("\n", " ", "\r", " ").Replace(text)
	parts := regexp.MustCompile(`(?m)[.!?;]\s+|\|`).Split(replaced, -1)
	return parts
}

// checkNoOverClaim fails when a forbidden phrasing is *asserted*. In document
// sources a sentence carrying a withdrawal marker is a quotation, not an
// assertion, and is exempt; shipped strings admit no exemption.
func checkNoOverClaim(sources []string, phrases []string, allowWithdrawalContext bool) error {
	for _, source := range sources {
		for _, sentence := range splitSentences(source) {
			lower := strings.ToLower(sentence)
			for _, phrase := range phrases {
				if !strings.Contains(lower, phrase) {
					continue
				}
				if allowWithdrawalContext {
					quoted := false
					for _, marker := range withdrawalMarkers {
						if strings.Contains(lower, marker) {
							quoted = true
							break
						}
					}
					if quoted {
						continue
					}
				}
				return fmt.Errorf("over-claim asserted: %q", strings.TrimSpace(sentence))
			}
		}
	}
	return nil
}

var confinementOverClaims = []string{
	"no path outside the repository is ever opened",
	"no file outside the repository can be read",
	"bytes read are physically inside the repository",
	"the root is a filesystem boundary",
	"the root is a device boundary",
	"mount points are prevented",
	"bind mounts are prevented",
}

var boundedRuntimeOverClaims = []string{
	"has no unbounded wait anywhere",
	"so nothing hangs",
	"no leaf kind can hang it",
	"the command cannot hang",
	"always terminates",
	"has bounded runtime",
	"has predictable runtime",
	"a read cannot block",
	"safe to run in a non-cancellable preflight",
}

var honestyOverClaims = []string{
	"every symlink race is detected",
	"the final open never follows a link",
	"a same-identity alias is refused",
	"a hard-link alias is detected",
	"an inode reuse is detected",
	"a swap-and-restore is defeated",
}

func contractDocuments(t *testing.T) []string {
	t.Helper()
	return []string{
		repoFile(t, "docs/prds/PRD-artifact-validation-and-provenance.md"),
		repoFile(t, "docs/adrs/ADR-034-rooted-filesystem-inspection-boundary.md"),
	}
}

func registerDocumentGuards() {
	registerGuard("AVP-152", guardSpec{
		Run: func(t *testing.T) error {
			return checkNoOverClaim(shippedStrings(t), honestyOverClaims, false)
		},
		Sensitivity: func(t *testing.T) error {
			return checkNoOverClaim(
				[]string{"prepare --check: every symlink race is detected before the open."},
				honestyOverClaims, false)
		},
	})

	registerGuard("AVP-189", guardSpec{
		Run: func(t *testing.T) error {
			if err := checkNoOverClaim(shippedStrings(t), confinementOverClaims, false); err != nil {
				return err
			}
			return checkNoOverClaim(contractDocuments(t), confinementOverClaims, true)
		},
		Sensitivity: func(t *testing.T) error {
			bare := "The inspector guarantees that no path outside the repository is ever opened, read, or named."
			if err := checkNoOverClaim([]string{bare}, confinementOverClaims, true); err == nil {
				return errors.New("a bare confinement claim passed the document half")
			}
			return checkNoOverClaim([]string{bare}, confinementOverClaims, false)
		},
	})

	registerGuard("AVP-207", guardSpec{
		Run: func(t *testing.T) error {
			if err := checkNoOverClaim(shippedStrings(t), boundedRuntimeOverClaims, false); err != nil {
				return err
			}
			// The inverse assertion: the documents as written must be green,
			// proving the quotation exemption is doing real work rather than
			// the guard being weakened to nothing.
			if err := checkNoOverClaim(contractDocuments(t), boundedRuntimeOverClaims, true); err != nil {
				return err
			}
			quoted := 0
			for _, document := range contractDocuments(t) {
				for _, phrase := range boundedRuntimeOverClaims {
					if strings.Contains(strings.ToLower(document), phrase) {
						quoted++
					}
				}
			}
			if quoted == 0 {
				return errors.New("no withdrawn phrasing is quoted anywhere; the exemption is untested")
			}
			return nil
		},
		Sensitivity: func(t *testing.T) error {
			for _, bare := range []string{
				"The command has no unbounded wait anywhere.",
				"Every capture is bounded, so nothing hangs.",
				"The kind gate means no leaf kind can hang it.",
			} {
				if err := checkNoOverClaim([]string{bare}, boundedRuntimeOverClaims, true); err == nil {
					return fmt.Errorf("a bare bounded-runtime claim passed: %q", bare)
				}
			}
			return checkNoOverClaim(
				[]string{"The command has no unbounded wait anywhere."},
				boundedRuntimeOverClaims, false)
		},
	})

	registerGuard("AVP-202", guardSpec{
		Run: func(t *testing.T) error {
			return checkMatrixArithmetic(t, parseAcceptanceMatrix(t), repoFile(t, "docs/prds/PRD-artifact-validation-and-provenance.md"))
		},
		Sensitivity: func(t *testing.T) error {
			prd := repoFile(t, "docs/prds/PRD-artifact-validation-and-provenance.md")
			// A prose citation beyond the declared maximum must fail.
			return checkMatrixArithmetic(t, parseAcceptanceMatrix(t), prd+"\n\nSee AVP-999 for the rest.\n")
		},
	})
}

func checkMatrixArithmetic(t *testing.T, rows []matrixRow, prose string) error {
	t.Helper()
	// (b) contiguity, no duplicates, no gaps.
	seen := map[string]bool{}
	for index, row := range rows {
		if seen[row.ID] {
			return fmt.Errorf("duplicate acceptance row %s", row.ID)
		}
		seen[row.ID] = true
		want := fmt.Sprintf("AVP-%03d", index+1)
		if row.ID != want {
			return fmt.Errorf("row %d is %s, want the contiguous %s", index+1, row.ID, want)
		}
	}
	if len(rows) != 208 {
		return fmt.Errorf("matrix declares %d rows, want 208", len(rows))
	}
	// (a) every AVP token in the prose resolves to a declared row.
	for _, match := range regexp.MustCompile(`AVP-\d{3}`).FindAllString(prose, -1) {
		if !seen[match] {
			return fmt.Errorf("prose cites %s, which is not a declared row", match)
		}
	}
	// (c) category counts.
	declaredCategories := map[string]int{
		"A": 10, "B": 20, "C": 8, "D": 14, "E": 6, "F": 5, "G": 9, "H": 4,
		"I": 6, "J": 4, "K": 6, "L": 3, "M": 6, "N": 5, "O": 12, "P": 10,
		"Q": 1, "R": 9, "S": 2, "T": 12, "V": 17, "W": 5, "X": 6, "Y": 8, "Z": 20,
	}
	actual := map[string]int{}
	for _, row := range rows {
		actual[row.Category]++
	}
	total := 0
	categories := make([]string, 0, len(declaredCategories))
	for category := range declaredCategories {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	for _, category := range categories {
		want := declaredCategories[category]
		if actual[category] != want {
			return fmt.Errorf("category %s has %d rows, §18.27 declares %d", category, actual[category], want)
		}
		total += want
	}
	if total != 208 {
		return fmt.Errorf("category table sums to %d, want 208", total)
	}
	// (d) kind counts and the guard predicate.
	declaredKinds := map[string]int{"U": 61, "I": 96, "S": 6, "G": 31, "S+G": 5, "U+G": 6, "I+G": 1, "S+I": 2}
	actualKinds := map[string]int{}
	guards := 0
	for _, row := range rows {
		actualKinds[row.Kind]++
		if strings.Contains(row.Kind, "G") {
			guards++
		}
	}
	kinds := make([]string, 0, len(declaredKinds))
	for kind := range declaredKinds {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		if actualKinds[kind] != declaredKinds[kind] {
			return fmt.Errorf("kind %s has %d rows, §18.27 declares %d", kind, actualKinds[kind], declaredKinds[kind])
		}
	}
	if guards != 43 {
		return fmt.Errorf("the guard predicate selects %d rows, §18.27 declares 43", guards)
	}
	if 31+5+6+1 != guards {
		return errors.New("the stated guard arithmetic no longer reproduces the derived set")
	}
	if 61+96+6+2 != 208-guards {
		return errors.New("the non-guard complement no longer sums to 165")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Cross-build guard
// ---------------------------------------------------------------------------

func checkCrossBuild(t *testing.T, targets []string) error {
	t.Helper()
	arch := map[string]string{"linux": "amd64", "darwin": "arm64", "windows": "amd64"}
	for _, goos := range targets {
		goarch, ok := arch[goos]
		if !ok {
			goarch = "amd64"
		}
		cmd := exec.Command("go", "build", "-o", filepath.Join(t.TempDir(), "out"), "./cmd/tpatch")
		cmd.Dir = repoRootDir(t)
		cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("GOOS=%s build failed: %v\n%s", goos, err, output)
		}
	}
	return nil
}
