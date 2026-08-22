package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestS7AQRecoverabilityClaimsAreEnvironmentallyQualified(t *testing.T) {
	// PIB-496: the shipped, nonhistorical claim inventory is closed, and every
	// accepted claim carries the environmental route qualification.
	sources := s7AQOwnedRecoverabilitySources(t)
	if err := validateS7AQRecoverabilityClaims(sources); err != nil {
		t.Fatal(err)
	}
	mutated := cloneS7AQSources(sources)
	const rel = "docs/prds/PRD-prepare-intent-bundle.md"
	mutated[rel] += "\nIntent publication is always recoverable.\n"
	if err := validateS7AQRecoverabilityClaims(mutated); err == nil {
		t.Fatal("PIB-496 same validator accepted unconditional recoverability")
	}
}

func TestS7AQStepReferencesResolveToDescribedSemantics(t *testing.T) {
	// PIB-505: every PRD/ADR step citation resolves against the mechanically
	// parsed ordered algorithm and the semantics in its surrounding clause.
	sources := map[string]string{}
	root := avpRepoRoot(t)
	for _, rel := range []string{
		"docs/prds/PRD-prepare-intent-bundle.md",
		"docs/adrs/ADR-035-intent-bundle-publication-and-history.md",
	} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		sources[rel] = string(data)
	}
	if err := validateS7AQStepReferences(sources); err != nil {
		t.Fatal(err)
	}
	for _, sensitivity := range []struct {
		name string
		old  string
		new  string
	}{
		{
			name: "pending-hash-cited-as-journal-recovery",
			old:  "per-hash machine has exactly one owner and it is\n    `feature intent-archive purge --yes` (§7.8 step 5",
			new:  "per-hash machine has exactly one owner and it is\n    `feature intent-archive purge --yes` (§7.8 step 4",
		},
		{
			name: "pending-hash-precedence-cited-as-terminal-recovery",
			old:  "§7.8 step 5, §7.11, §10.5 step 13):",
			new:  "§7.8 step 5, §7.11, §10.5 step 12):",
		},
		{
			name: "read-platform-cell-cited-as-mutating-platform",
			old:  "read-boundary platform allowlist (§10.5 step 5)",
			new:  "read-boundary platform allowlist (§10.5 step 8)",
		},
		{
			name: "abandon-branch-cited-as-flock",
			old:  "the branch is at §7.8 step 2 / §10.5 step 10",
			new:  "the branch is at §7.8 step 2 / §10.5 step 9",
		},
		{
			name: "abandon-publication-step-cited-as-git-gate",
			old:  "the branch is at §7.8 step 2 / §10.5 step 10",
			new:  "the branch is at §7.8 step 3 / §10.5 step 10",
		},
		{
			name: "publication-lock-negated",
			old:  "1. Acquire the one held-root directory lock",
			new:  "1. Never acquire the one held-root directory lock",
		},
		{
			name: "publication-terminal-recovery-negated",
			old:  "**Journal recovery (§7.11), and it is terminal.**",
			new:  "**Journal recovery (§7.11), and it is not terminal.**",
		},
		{
			name: "precedence-flock-negated",
			old:  "9. One directory-flock acquisition",
			new:  "9. No directory-flock acquisition",
		},
		{
			name: "precedence-terminal-recovery-negated",
			old:  "**on success it is terminal**",
			new:  "**on success it is not terminal**",
		},
	} {
		mutated := cloneS7AQSources(sources)
		const rel = "docs/prds/PRD-prepare-intent-bundle.md"
		before := mutated[rel]
		mutated[rel] = strings.Replace(before, sensitivity.old, sensitivity.new, 1)
		if mutated[rel] == before {
			t.Fatalf("PIB-505 %s mutation anchor missing", sensitivity.name)
		}
		if err := validateS7AQStepReferences(mutated); err == nil {
			t.Fatalf("PIB-505 same validator accepted %s", sensitivity.name)
		}
	}
}

func s7AQOwnedRecoverabilitySources(t *testing.T) map[string]string {
	t.Helper()
	sources := map[string]string{}
	repoRoot := avpRepoRoot(t)
	roots := []string{"internal", "assets"}
	for _, root := range roots {
		err := filepath.WalkDir(filepath.Join(repoRoot, root), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			name := entry.Name()
			if (!strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, ".md")) ||
				strings.HasSuffix(name, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return err
			}
			sources[filepath.ToSlash(relative)] = string(data)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range []string{
		"SPEC.md",
		"docs/prds/PRD-prepare-intent-bundle.md",
		"docs/adrs/ADR-035-intent-bundle-publication-and-history.md",
	} {
		data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		sources[rel] = string(data)
	}
	return sources
}

func validateS7AQRecoverabilityClaims(sources map[string]string) error {
	accepted := map[string]bool{
		"59144be083985bcd8ff2f62e0f8efa8be0500581a781ed6f273caafb4f120403": true,
		"16121ce26e84d048756f330117253bdd50f112352be7f6da4c16e38a4a18feb8": true,
		"43f689e94291b8897e17d2ebc7919a9e4556e2475f43967bf7b3629bcd8f453b": true,
		"55f4781cfe533333fcc9adfe59d61e1fd7d94f01c058c1bad2e2835594b8a26f": true,
		"05aaaf3e14b4f3774222928a4dada91c36a122dac4929b260d937f880c683575": true,
		"6333d0fe60eef82cb95f613286030092d0051f009e28458fa337e18ec469a578": true,
		"2b882f50b49e4a80b7613e3026622b4eef85219f7898ca75ff6b5280dda58436": true,
		"21c45bc6417f34db5d8ce71c9923261b43f197206e400865a04ebf52e0b4b5ed": true,
		"0e1f96786d40505dd1713e68d82376c92a9b0bb6972a7a505d4fdd873e533e80": true,
		"af01d5f58da48aaca0f6071c28fb7ce18d0d613ab4c52b84ee11224ef66e9719": true,
	}
	observed := map[string]string{}
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		content := sources[name]
		switch name {
		case "docs/prds/PRD-prepare-intent-bundle.md":
			content = s7AQAfterMarker(content, "## 1. Problem")
		case "docs/adrs/ADR-035-intent-bundle-publication-and-history.md":
			content = s7AQAfterMarker(content, "## Context")
		}
		for _, clause := range s7AQCompleteClauses(content) {
			normalized := s7AQNormalizeClause(clause)
			if !strings.Contains(normalized, "always recoverable") &&
				!strings.Contains(normalized, "exit 6 is never terminal") {
				continue
			}
			sum := sha256.Sum256([]byte(normalized))
			hash := hex.EncodeToString(sum[:])
			if !accepted[hash] {
				return fmt.Errorf("%s has an unclassified recoverability claim %q", name, normalized)
			}
			observed[hash] = name
		}
	}
	if len(observed) != len(accepted) {
		var missing []string
		for hash := range accepted {
			if observed[hash] == "" {
				missing = append(missing, hash)
			}
		}
		sort.Strings(missing)
		return fmt.Errorf("recoverability claim inventory drift: got %d want %d missing %v",
			len(observed), len(accepted), missing)
	}
	return nil
}

func s7AQAfterMarker(content, marker string) string {
	index := strings.Index(content, marker)
	if index < 0 {
		return content
	}
	return content[index:]
}

func s7AQCompleteClauses(content string) []string {
	var units []string
	var prose []string
	flush := func() {
		if len(prose) == 0 {
			return
		}
		units = append(units, strings.Join(prose, " "))
		prose = nil
	}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "|"):
			flush()
			units = append(units, trimmed)
		case trimmed == "":
			flush()
		default:
			prose = append(prose, trimmed)
		}
	}
	flush()
	var clauses []string
	for _, unit := range units {
		clauses = append(clauses, s7AQSplitSentences(unit)...)
	}
	return clauses
}

func s7AQSplitSentences(unit string) []string {
	var sentences []string
	start := 0
	for index := 0; index < len(unit); index++ {
		switch unit[index] {
		case '.', '!', '?':
		default:
			continue
		}
		next := index + 1
		for next < len(unit) && (unit[next] == ' ' || unit[next] == '\t') {
			next++
		}
		if next == index+1 || next >= len(unit) ||
			!(unit[next] >= 'A' && unit[next] <= 'Z') &&
				!strings.ContainsRune(`*"|`, rune(unit[next])) {
			continue
		}
		sentences = append(sentences, unit[start:index+1])
		start = next
		index = next - 1
	}
	if start < len(unit) {
		sentences = append(sentences, unit[start:])
	}
	return sentences
}

func s7AQNormalizeClause(clause string) string {
	clause = strings.ReplaceAll(clause, "`", "")
	clause = strings.ReplaceAll(clause, "**", "")
	return strings.ToLower(strings.Join(strings.Fields(clause), " "))
}

type s7AQStepReference struct {
	source  string
	kind    string
	section string
	step    string
	context string
}

var s7AQStepReferencePattern = regexp.MustCompile(
	`§(7\.8|10\.5)\s+steps?\s+([0-9]+[a-z]?(?:\s*(?:,|and|–|-)\s*[0-9]+[a-z]?){0,4})`,
)
var s7AQStepNumberPattern = regexp.MustCompile(`[0-9]+[a-z]?`)

func validateS7AQStepReferences(sources map[string]string) error {
	prd := sources["docs/prds/PRD-prepare-intent-bundle.md"]
	if prd == "" || sources["docs/adrs/ADR-035-intent-bundle-publication-and-history.md"] == "" {
		return errors.New("PRD or ADR-035 source is missing")
	}
	steps := map[string]string{}
	for _, section := range []string{"7.8", "10.5"} {
		parsed, err := s7AQParseOrderedSteps(prd, section)
		if err != nil {
			return err
		}
		for step, text := range parsed {
			steps[section+"/"+step] = text
		}
	}
	var stepInventory []string
	for key, text := range steps {
		stepInventory = append(stepInventory, key+"\x1f"+text)
	}
	sort.Strings(stepInventory)
	stepSum := sha256.Sum256([]byte(strings.Join(stepInventory, "\n")))
	const wantStepInventory = "92d04e92e7870201e2f27a8ea76ebc487f2e60c949b183b140122c58563bb551"
	if got := hex.EncodeToString(stepSum[:]); got != wantStepInventory {
		return fmt.Errorf("canonical step-definition inventory hash = %s, want %s",
			got, wantStepInventory)
	}
	references := s7AQExtractStepReferences(sources)
	if len(references) != 95 {
		return fmt.Errorf("step reference inventory = %d, want 95", len(references))
	}
	var clauseInventory []string
	for _, reference := range references {
		key := reference.section + "/" + reference.step
		stepText := steps[key]
		if stepText == "" {
			return fmt.Errorf("reference to missing §%s step %s", reference.section, reference.step)
		}
		clauseInventory = append(clauseInventory, strings.Join([]string{
			reference.source, reference.kind, reference.section,
			reference.step, reference.context,
		}, "\x1f"))
	}
	sort.Strings(clauseInventory)
	sum := sha256.Sum256([]byte(strings.Join(clauseInventory, "\n")))
	const wantClauseInventory = "4bd4a87a5462cd22475e6125677e04337a8e260f3c48979b66e0d35cbab2d1f9"
	if got := hex.EncodeToString(sum[:]); got != wantClauseInventory {
		return fmt.Errorf("step citation semantic-inventory hash = %s, want %s",
			got, wantClauseInventory)
	}
	return nil
}

func s7AQParseOrderedSteps(prd, section string) (map[string]string, error) {
	startMarker := "### " + section + " "
	start := strings.Index(prd, startMarker)
	if start < 0 {
		return nil, fmt.Errorf("section %s is missing", section)
	}
	body := prd[start+len(startMarker):]
	if end := strings.Index(body, "\n### "); end >= 0 {
		body = body[:end]
	}
	stepPattern := regexp.MustCompile(`(?m)^\s*([0-9]+[a-z]?)\.\s`)
	matches := stepPattern.FindAllStringSubmatchIndex(body, -1)
	steps := map[string]string{}
	order := make([]string, 0, len(matches))
	for index, match := range matches {
		step := body[match[2]:match[3]]
		order = append(order, step)
		textStart := match[1]
		textEnd := len(body)
		if index+1 < len(matches) {
			textEnd = matches[index+1][0]
		}
		steps[step] = s7AQNormalizeClause(body[textStart:textEnd])
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("section %s has no parsed steps", section)
	}
	wantOrder := map[string][]string{
		"7.8": {
			"1", "2", "3", "4", "5", "6", "7", "8", "9", "10",
			"11", "12", "13", "14", "15",
		},
		"10.5": {
			"1", "1a", "2", "3", "4", "5", "6", "7", "8", "9", "10",
			"11", "12", "13", "14", "15", "16", "17", "18", "19", "20",
			"21", "22", "23", "24", "25",
		},
	}
	if fmt.Sprint(order) != fmt.Sprint(wantOrder[section]) {
		return nil, fmt.Errorf("section %s ordered steps = %v, want %v",
			section, order, wantOrder[section])
	}
	return steps, nil
}

func s7AQExtractStepReferences(sources map[string]string) []s7AQStepReference {
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	var references []s7AQStepReference
	for _, name := range names {
		content, err := s7AQNonHistoricalStepDocument(name, sources[name])
		if err != nil {
			return []s7AQStepReference{{
				source: name, kind: "document-error", context: err.Error(),
			}}
		}
		for _, unit := range s7AQMarkdownSemanticUnits(content) {
			for _, clause := range s7AQImmediateSemanticClauses(unit.text) {
				for _, match := range s7AQStepReferencePattern.FindAllStringSubmatch(clause, -1) {
					for _, step := range s7AQStepNumberPattern.FindAllString(match[2], -1) {
						references = append(references, s7AQStepReference{
							source: name, kind: unit.kind + "-clause",
							section: match[1], step: step,
							context: s7AQNormalizeClause(clause),
						})
					}
				}
			}
		}
	}
	return references
}

func s7AQNonHistoricalStepDocument(name, content string) (string, error) {
	marker := ""
	wantExcluded := 0
	switch name {
	case "docs/prds/PRD-prepare-intent-bundle.md":
		marker = "## 1. Problem"
		wantExcluded = 9
	case "docs/adrs/ADR-035-intent-bundle-publication-and-history.md":
		marker = "## Context"
	default:
		return "", fmt.Errorf("unclassified step-reference document")
	}
	index := strings.Index(content, marker)
	if index < 0 {
		return "", fmt.Errorf("bounded nonhistorical marker %q is missing", marker)
	}
	excluded := len(s7AQStepReferencePattern.FindAllString(content[:index], -1))
	if excluded != wantExcluded {
		return "", fmt.Errorf("historical step-reference exclusion = %d, want %d",
			excluded, wantExcluded)
	}
	return content[index:], nil
}

type s7AQMarkdownUnit struct {
	kind string
	text string
}

var s7AQMarkdownListStart = regexp.MustCompile(
	`^\s*(?:[-+*]|\d+[a-z]?\.)\s+`,
)

func s7AQMarkdownSemanticUnits(content string) []s7AQMarkdownUnit {
	var units []s7AQMarkdownUnit
	var current []string
	currentKind := ""
	flush := func() {
		if len(current) == 0 {
			return
		}
		text := strings.Join(current, " ")
		if currentKind == "sentence" {
			for _, sentence := range s7AQMarkdownSentences(text) {
				units = append(units, s7AQMarkdownUnit{kind: "sentence", text: sentence})
			}
		} else {
			units = append(units, s7AQMarkdownUnit{kind: currentKind, text: text})
		}
		current = nil
		currentKind = ""
	}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			flush()
		case strings.HasPrefix(trimmed, "|"):
			flush()
			for _, cell := range strings.Split(trimmed, "|") {
				cell = strings.TrimSpace(cell)
				if cell != "" && !regexp.MustCompile(`^:?-+:?$`).MatchString(cell) {
					units = append(units, s7AQMarkdownUnit{kind: "table-cell", text: cell})
				}
			}
		case s7AQMarkdownListStart.MatchString(line):
			flush()
			currentKind = "list-item"
			current = append(current, trimmed)
		case strings.HasPrefix(trimmed, "#"):
			flush()
			units = append(units, s7AQMarkdownUnit{kind: "heading", text: trimmed})
		default:
			if currentKind == "" {
				currentKind = "sentence"
			}
			current = append(current, trimmed)
		}
	}
	flush()
	return units
}

func s7AQMarkdownSentences(text string) []string {
	var sentences []string
	start := 0
	inCode := false
	for index := 0; index < len(text); index++ {
		if text[index] == '`' {
			inCode = !inCode
			continue
		}
		if inCode || !strings.ContainsRune(".!?", rune(text[index])) {
			continue
		}
		next := index + 1
		if next < len(text) && text[next] != ' ' && text[next] != '\t' {
			continue
		}
		for next < len(text) && (text[next] == ' ' || text[next] == '\t') {
			next++
		}
		sentence := strings.TrimSpace(text[start : index+1])
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
		start = next
		index = next - 1
	}
	if tail := strings.TrimSpace(text[start:]); tail != "" {
		sentences = append(sentences, tail)
	}
	return sentences
}

func s7AQImmediateSemanticClauses(text string) []string {
	var clauses []string
	start := 0
	inCode := false
	flush := func(end int) {
		if clause := strings.TrimSpace(text[start:end]); clause != "" {
			clauses = append(clauses, clause)
		}
	}
	for index := 0; index < len(text); index++ {
		if text[index] == '`' {
			inCode = !inCode
			continue
		}
		if inCode {
			continue
		}
		width := 0
		switch {
		case text[index] == ';':
			width = 1
		case text[index] == ':' &&
			index+1 < len(text) && (text[index+1] == ' ' || text[index+1] == '\t'):
			width = 1
		case strings.ContainsRune(".!?", rune(text[index])):
			next := index + 1
			if next == len(text) || text[next] == ' ' || text[next] == '\t' {
				width = 1
			}
		}
		if width == 0 {
			continue
		}
		flush(index + 1)
		index += width - 1
		start = index + 1
		for start < len(text) && (text[start] == ' ' || text[start] == '\t') {
			start++
			index = start - 1
		}
	}
	flush(len(text))
	return clauses
}

func cloneS7AQSources(sources map[string]string) map[string]string {
	clone := make(map[string]string, len(sources))
	for name, source := range sources {
		clone[name] = source
	}
	return clone
}
