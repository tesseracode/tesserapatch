package workflow

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The verify_anchored.go cite ledger. Every cite of
// internal/workflow/verify_anchored.go in the design docs is listed here,
// counted per document and range, and is exactly one of:
//
//   - live: Anchor must appear inside the cited lines of the current
//     source. When an edit moves the construct, move the doc cite to its
//     new lines and update the row.
//   - historical: Pin names the commit at which the cite was verified;
//     the range is frozen and never refreshed.
//
// docs/handoff/ and docs/supervisor/ are dated records and are not
// scanned. A new cite fails until it is classified here.
type verifyAnchoredCite struct {
	Doc    string
	Range  string
	N      int
	Anchor string
	Pin    string
}

const (
	citeADR013       = "docs/adrs/ADR-013-verify-freshness-overlay.md"
	citeADR036       = "docs/adrs/ADR-036-recipe-coverage-authority.md"
	citePRDAnchor    = "docs/prds/PRD-historical-anchor-relanding.md"
	citePRDOverlap   = "docs/prds/PRD-overlap-verification-consolidation.md"
	citePRDRecipe    = "docs/prds/PRD-recipe-generation-authority.md"
	citePRDFreshness = "docs/prds/PRD-verify-freshness.md"
)

var verifyAnchoredCiteLedger = []verifyAnchoredCite{
	// ADR-013-verify-freshness-overlay.md
	{citeADR013, "276-280", 1, "", "2354df9"},
	{citeADR013, "311-328", 1, "", "2354df9"},
	{citeADR013, "342-590", 1, "", "15560af"},
	{citeADR013, "389-391", 1, "", "15560af"},
	{citeADR013, "404", 1, "", "15560af"},
	{citeADR013, "432-434", 1, "if _, superseded := isFeatureSupersededIn(allFeatures, curr); superseded {", ""},
	{citeADR013, "439-464", 2, "", "15560af"},
	{citeADR013, "464", 1, "", "15560af"},
	{citeADR013, "476", 1, "", "15560af"},
	{citeADR013, "481-495", 1, "shadowRoot := anchorCommit", ""},
	{citeADR013, "513-519", 1, "", "2354df9"},
	{citeADR013, "526", 2, "closureBaselineTree, err := snapshotShadowTree(ctx, shadowPath)", ""},
	{citeADR013, "547-553", 1, "", "15560af"},
	{citeADR013, "556-580", 1, "", "15560af"},
	{citeADR013, "576", 1, "", "15560af"},
	{citeADR013, "596-604", 2, "resetShadowToTree(ctx, shadowPath, closureBaselineTree)", ""},
	{citeADR013, "605-637", 1, "applyRes := ctx.shadowApplyCheck(shadowPath, in.entry.Patch.Bytes)", ""},
	{citeADR013, "621-756", 2, "", "15560af"},
	{citeADR013, "653-666", 2, "", "15560af"},
	{citeADR013, "669-674", 1, "", "2354df9"},
	{citeADR013, "700-701", 1, "case store.StateUpstreamMerged:", ""},
	{citeADR013, "740-742", 1, "memberLandingDriftRemediation(ctx, slug, member, entry, mev, anchorTreeish, replayAnchor)", ""},
	{citeADR013, "768-800", 1, "pr, prerr := loadParentRecipeFromInventory(entry)", ""},
	{citeADR013, "791-792", 1, "", "15560af"},
	{citeADR013, "796-800", 1, "", "15560af"},
	{citeADR013, "847-850", 1, "", "15560af"},
	{citeADR013, "889-893", 1, "", "15560af"},
	{citeADR013, "913-915", 1, "skipV10Because(\"skipped: V2 (recipe_parses) skipped or failed\")", ""},
	{citeADR013, "918-922", 1, "severity = SeverityWarn", ""},
	{citeADR013, "969-972", 1, "// ADR-029 D4 legacy path: pass, no re-warn.", ""},
	{citeADR013, "1011-1015", 1, "downgraded to warn: superseded by %q", ""},
	// ADR-036-recipe-coverage-authority.md
	{citeADR036, "910-1129", 1, "func evaluateV10(", ""},
	{citeADR036, "938-962", 1, "if refTree == \"\" && needsProvenance(in.recipe) {", ""},
	{citeADR036, "1090-1129", 1, "if found && string(data) != op.Content {", ""},
	// PRD-historical-anchor-relanding.md
	{citePRDAnchor, "110-111", 1, "if terminal {", ""},
	{citePRDAnchor, "110-111", 1, "return terminalEvidencePhase(in, baseline, v7Mode, v8Mode, v10Mode)", ""},
	{citePRDAnchor, "174", 2, "failedAt: FailedAtLandingEvidence,", ""},
	{citePRDAnchor, "483", 2, "", "2e44569"},
	{citePRDAnchor, "695", 2, "", "2e44569"},
	{citePRDAnchor, "705-706", 3, "", "2e44569"},
	// PRD-overlap-verification-consolidation.md
	{citePRDOverlap, "122-124", 2, "", "7f5f97c"},
	{citePRDOverlap, "205", 2, "", "7f5f97c"},
	{citePRDOverlap, "217", 1, "ladder := ctx.runLadder(ctx.headRev(), in.entry.Patch.Bytes)", ""},
	{citePRDOverlap, "232-253", 2, "", "7f5f97c"},
	{citePRDOverlap, "264", 1, "", "7f5f97c"},
	{citePRDOverlap, "264-268", 1, "", "7f5f97c"},
	{citePRDOverlap, "286-365", 1, "case ladder.Blocked:", ""},
	{citePRDOverlap, "299-315", 2, "", "7f5f97c"},
	{citePRDOverlap, "311", 1, "case ladder.Blocked:", ""},
	{citePRDOverlap, "316-320", 1, "", "7f5f97c"},
	{citePRDOverlap, "357-363", 1, "case ladder.ContextDrift:", ""},
	{citePRDOverlap, "376-401", 2, "", "7f5f97c"},
	{citePRDOverlap, "423-445", 1, "closure[slug] = filterHardDeps(in.status.DependsOn)", ""},
	{citePRDOverlap, "432-434", 1, "isFeatureSupersededIn(allFeatures, curr)", ""},
	{citePRDOverlap, "566", 1, "replayRecipeOpsInShadow(shadowPath, in.recipe.Operations)", ""},
	{citePRDOverlap, "596-605", 1, "applyRes := ctx.shadowApplyCheck(shadowPath, in.entry.Patch.Bytes)", ""},
	{citePRDOverlap, "654-655", 1, "", "7f5f97c"},
	{citePRDOverlap, "680-721", 1, "", "7f5f97c"},
	{citePRDOverlap, "684", 1, "", "7f5f97c"},
	{citePRDOverlap, "694-696", 1, "", "7f5f97c"},
	{citePRDOverlap, "700-701", 1, "case store.StateUpstreamMerged:", ""},
	{citePRDOverlap, "726-765", 1, "ladder := ctx.runLadder(anchorTreeish, entry.Patch.Bytes)", ""},
	{citePRDOverlap, "730-742", 1, "ladder := ctx.runLadder(anchorTreeish, entry.Patch.Bytes)", ""},
	{citePRDOverlap, "796-800", 1, "", "7f5f97c"},
	{citePRDOverlap, "864-867", 1, "", "7f5f97c"},
	{citePRDOverlap, "910-1021", 1, "ctx.preimageAtTree(refTree, slug, i+1, op)", ""},
	{citePRDOverlap, "918-922", 1, "severity = SeverityWarn", ""},
	{citePRDOverlap, "987-990", 1, "warnAdvisory(AdvisoryLaterTouch, slug, op.Path,", ""},
	// PRD-recipe-generation-authority.md
	{citePRDRecipe, "918-922", 1, "severity = SeverityWarn", ""},
	{citePRDRecipe, "938-962", 1, "if refTree == \"\" && needsProvenance(in.recipe) {", ""},
	{citePRDRecipe, "938-962", 2, "prov := ctx.resolveProvenance(in.entry)", ""},
	{citePRDRecipe, "964-981", 1, "", "e76e0f7"},
	{citePRDRecipe, "1011-1015", 1, "downgraded to warn: superseded by %q", ""},
	{citePRDRecipe, "1090-1129", 1, "if found && string(data) != op.Content {", ""},
	// PRD-verify-freshness.md
	{citePRDFreshness, "432-434", 1, "isFeatureSupersededIn(allFeatures, curr)", ""},
	{citePRDFreshness, "439-464", 1, "", "15560af"},
	{citePRDFreshness, "481-507", 1, "shadowRoot := anchorCommit", ""},
	{citePRDFreshness, "507", 1, "defer func() { _ = ctx.pruneShadow(slug) }()", ""},
	{citePRDFreshness, "526", 1, "closureBaselineTree, err := snapshotShadowTree(ctx, shadowPath)", ""},
	{citePRDFreshness, "596-604", 2, "resetShadowToTree(ctx, shadowPath, closureBaselineTree)", ""},
	{citePRDFreshness, "621-756", 1, "", "15560af"},
	{citePRDFreshness, "625", 1, "post-apply.patch no longer applies to closure-replayed baseline; run tpatch reconcile %s", ""},
	{citePRDFreshness, "653-666", 2, "", "15560af"},
	{citePRDFreshness, "700-701", 1, "case store.StateUpstreamMerged:", ""},
	{citePRDFreshness, "768-800", 1, "pr, prerr := loadParentRecipeFromInventory(entry)", ""},
	{citePRDFreshness, "910-1021", 1, "ok, msg, observed := ctx.preimageAtTree(refTree, slug, i+1, op)", ""},
	{citePRDFreshness, "913-915", 1, "skipV10Because(\"skipped: V2 (recipe_parses) skipped or failed\")", ""},
	{citePRDFreshness, "918-922", 1, "severity = SeverityWarn", ""},
	{citePRDFreshness, "969-972", 1, "// ADR-029 D4 legacy path: pass, no re-warn.", ""},
	{citePRDFreshness, "1011-1015", 2, "downgraded to warn: superseded by %q", ""},
	{citePRDFreshness, "1038-1082", 1, "ctx.preimageAtTree(anchor.Anchor, member, i+1, op)", ""},
}

const citeRangeList = `[0-9]+(?:-[0-9]+)?(?:,[0-9]+(?:-[0-9]+)?)*`

// A cite is "<path>:<ranges>"; a backticked "`:<ranges>`" continues the
// most recent path in the same block, which may be on a wrapped line or a
// previous table row. A blank line ends the block.
var (
	citeTokenRE = regexp.MustCompile("`:(" + citeRangeList + `)|([A-Za-z0-9_./-]*[A-Za-z][A-Za-z0-9_./-]*):(` + citeRangeList + `)`)
	citePinRE   = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
)

func verifyAnchoredCitesIn(text string) []string {
	var ranges []string
	current := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			current = ""
		}
		for _, m := range citeTokenRE.FindAllStringSubmatch(line, -1) {
			list := m[1]
			if m[2] != "" {
				current = filepath.Base(m[2])
				list = m[3]
			}
			if current == "verify_anchored.go" {
				ranges = append(ranges, strings.Split(list, ",")...)
			}
		}
	}
	return ranges
}

func parseCiteRange(r string) (start, end int, ok bool) {
	a, b, isRange := strings.Cut(r, "-")
	start, err := strconv.Atoi(a)
	if err != nil {
		return 0, 0, false
	}
	end = start
	if isRange {
		if end, err = strconv.Atoi(b); err != nil {
			return 0, 0, false
		}
	}
	return start, end, start > 0 && end >= start
}

func checkVerifyAnchoredCites(docs map[string]string, source []byte, ledger []verifyAnchoredCite) []string {
	lines := strings.Split(string(source), "\n")
	lineCount := acceptanceLedgerSourceLineCount(source)
	found := map[[2]string]int{}
	for doc, text := range docs {
		for _, r := range verifyAnchoredCitesIn(text) {
			found[[2]string{doc, r}]++
		}
	}
	var problems []string
	want := map[[2]string]int{}
	for _, row := range ledger {
		key := [2]string{row.Doc, row.Range}
		want[key] += row.N
		start, end, ok := parseCiteRange(row.Range)
		switch {
		case row.N < 1:
			problems = append(problems, fmt.Sprintf("%s:%s: ledger count %d is not positive", row.Doc, row.Range, row.N))
		case (row.Anchor == "") == (row.Pin == ""):
			problems = append(problems, fmt.Sprintf("%s:%s: a row needs exactly one of a live anchor or a historical pin", row.Doc, row.Range))
		case row.Pin != "" && !citePinRE.MatchString(row.Pin):
			problems = append(problems, fmt.Sprintf("%s:%s: pin %q is not a commit id", row.Doc, row.Range, row.Pin))
		case row.Anchor == "":
		case !ok || end > lineCount:
			problems = append(problems, fmt.Sprintf("%s cites verify_anchored.go:%s, outside the file's %d lines", row.Doc, row.Range, lineCount))
		case !strings.Contains(strings.Join(lines[start-1:end], "\n"), row.Anchor):
			problems = append(problems, fmt.Sprintf("%s cites verify_anchored.go:%s for %q, which is no longer in those lines; move the cite to the construct's current lines and update the ledger row", row.Doc, row.Range, row.Anchor))
		}
	}
	for key, n := range found {
		if want[key] != n {
			problems = append(problems, fmt.Sprintf("%s cites verify_anchored.go:%s %d time(s); the ledger lists %d — classify the cite as live or historical", key[0], key[1], n, want[key]))
		}
	}
	for key, n := range want {
		if found[key] == 0 {
			problems = append(problems, fmt.Sprintf("%s: ledger lists verify_anchored.go:%s %d time(s), but the doc no longer cites it", key[0], key[1], n))
		}
	}
	sort.Strings(problems)
	return problems
}

func verifyAnchoredCiteDocs(t *testing.T, root string) map[string]string {
	t.Helper()
	docs := map[string]string{}
	add := func(path string) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		docs[filepath.ToSlash(rel)] = string(data)
	}
	err := filepath.WalkDir(filepath.Join(root, "docs"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "handoff" || d.Name() == "supervisor") {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			add(path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk docs: %v", err)
	}
	top, err := filepath.Glob(filepath.Join(root, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range top {
		add(path)
	}
	return docs
}

func TestVerifyAnchoredCiteLedger(t *testing.T) {
	root := docsRootForTest(t)
	source, err := os.ReadFile(filepath.Join(root, "internal", "workflow", "verify_anchored.go"))
	if err != nil {
		t.Fatalf("read verify_anchored.go: %v", err)
	}
	for _, problem := range checkVerifyAnchoredCites(verifyAnchoredCiteDocs(t, root), source, verifyAnchoredCiteLedger) {
		t.Error(problem)
	}
}

func TestVerifyAnchoredCiteLedgerIsSensitive(t *testing.T) {
	source := "package x\n\nfunc alpha() {\n\treturn\n}\n"
	doc := "See `internal/workflow/verify_anchored.go:3-5`, `:4`; `ADR-013:9`, `:2`.\n" +
		"Frozen `verify_anchored.go:40` and live `verify_anchored.go:1,3`.\n" +
		"Unrelated `other.go:7`, `:5`.\n"
	ledger := []verifyAnchoredCite{
		{"d.md", "1", 1, "package x", ""},
		{"d.md", "3", 1, "func alpha", ""},
		{"d.md", "3-5", 1, "func alpha", ""},
		{"d.md", "4", 1, "return", ""},
		{"d.md", "40", 1, "", "abc1234"},
	}
	if got, want := verifyAnchoredCitesIn(doc), []string{"3-5", "4", "40", "1", "3"}; strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("parsed %v, want %v", got, want)
	}
	blocks := "Wrapped `verify_anchored.go:3`,\n  `:1`.\n| a | `verify_anchored.go:4` |\n| b | `:5` |\n \t\n`:2` after a blank line; `ADR-013:9`, `:2`.\n"
	if got, want := verifyAnchoredCitesIn(blocks), []string{"3", "1", "4", "5"}; strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("parsed %v across lines, want %v", got, want)
	}
	if problems := checkVerifyAnchoredCites(map[string]string{"d.md": doc}, []byte(source), ledger); len(problems) != 0 {
		t.Fatalf("clean fixture reported %v", problems)
	}
	with := func(rows ...verifyAnchoredCite) []verifyAnchoredCite {
		return append(append([]verifyAnchoredCite{}, ledger...), rows...)
	}
	// Each rule-specific case keeps the counts equal and breaks one rule
	// of an otherwise valid row, so only that rule can reject it.
	swap := func(i int, row verifyAnchoredCite) []verifyAnchoredCite {
		out := append([]verifyAnchoredCite{}, ledger...)
		out[i] = row
		return out
	}
	frozen := func(row verifyAnchoredCite) []verifyAnchoredCite { return swap(len(ledger)-1, row) }
	for _, tc := range []struct {
		name   string
		doc    string
		source string
		ledger []verifyAnchoredCite
	}{
		{"construct-moved", doc, "\n" + source, ledger},
		{"source-shrank", doc, "package x\n\nfunc alpha() {\n", ledger},
		{"unclassified-cite", doc + "`verify_anchored.go:2`\n", source, ledger},
		{"repeated-cite", doc + "`verify_anchored.go:4`\n", source, ledger},
		{"continuation-cite", doc + "`verify_anchored.go:1`, `:2`\n", source, with(verifyAnchoredCite{"d.md", "1", 1, "package x", ""})},
		{"wrapped-continuation", doc + "`verify_anchored.go:1`,\n`:2`\n", source, with(verifyAnchoredCite{"d.md", "1", 1, "package x", ""})},
		{"table-row-continuation", doc + "| a | `verify_anchored.go:1` |\n| b | `:2` |\n", source, with(verifyAnchoredCite{"d.md", "1", 1, "package x", ""})},
		{"stale-row", doc, source, with(verifyAnchoredCite{"d.md", "5", 1, "}", ""})},
		{"anchor-and-pin", doc, source, swap(0, verifyAnchoredCite{"d.md", "1", 1, "package x", "abc1234"})},
		{"neither", doc, source, frozen(verifyAnchoredCite{"d.md", "40", 1, "", ""})},
		{"pin-not-commit", doc, source, frozen(verifyAnchoredCite{"d.md", "40", 1, "", "HEAD"})},
		{"zero-count", doc, source, with(verifyAnchoredCite{"d.md", "40", 0, "", "abc1234"})},
		{"reversed-range", doc + "`verify_anchored.go:5-3`\n", source, with(verifyAnchoredCite{"d.md", "5-3", 1, "x", ""})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if problems := checkVerifyAnchoredCites(map[string]string{"d.md": tc.doc}, []byte(tc.source), tc.ledger); len(problems) == 0 {
				t.Fatal("the guard accepted a broken fixture")
			}
		})
	}
}
