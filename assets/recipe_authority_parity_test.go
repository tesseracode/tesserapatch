package assets_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/assets"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
)

// RGA-359 checks installed bytes and current public guidance with the same
// validator used by the wrong-input cases below. Changelog releases older
// than the selected contract are not current guidance; no other file is exempted.
func TestRecipeAuthorityPublicParity(t *testing.T) {
	for _, sf := range skillFiles {
		t.Run(sf.name, func(t *testing.T) {
			body := rgaEmbeddedSurface(t, sf.path)
			if err := validateRecipeAuthoritySurface(body, true); err != nil {
				t.Fatalf("%s: %v", sf.path, err)
			}
		})
	}
	t.Run("template", func(t *testing.T) {
		if err := validateRecipeAuthoritySurface(rgaEmbeddedSurface(t, "templates/README.md"), true); err != nil {
			t.Fatal(err)
		}
	})
	for _, path := range []string{
		"README.md", "SPEC.md", "CHANGELOG.md", "docs/feature-layout.md",
		"docs/record.md", "docs/path-b-operator-guide.md", "docs/faq.md",
		"docs/agent-as-provider.md", "docs/reconcile.md", "docs/land.md",
		"docs/commits.md", "docs/dependencies.md",
	} {
		t.Run(path, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(path)))
			if err != nil {
				t.Fatal(err)
			}
			if err := rgaValidatePublicSurface(path, string(raw)); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
		})
	}
}

func rgaValidatePublicSurface(path, body string) error {
	if path == "CHANGELOG.md" {
		current, err := rgaChangelogSection(body, "0.17.0")
		if err != nil {
			return err
		}
		body = current
	}
	if err := validateRecipeAuthoritySurface(body, false); err != nil {
		return err
	}
	if path == "SPEC.md" || path == "CHANGELOG.md" {
		return rgaRequireEditorFailureDisclosure(body)
	}
	return nil
}

func rgaEmbeddedSurface(t *testing.T, path string) string {
	t.Helper()
	body, err := assets.Skills.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

var rgaChangelogHeading = regexp.MustCompile(`^ {0,3}##(?:[ \t]+|$)`)
var rgaCurrentHeading = regexp.MustCompile(`(?i)^ {0,3}##[ \t]+\[?unreleased\b`)
var rgaUnreleasedHeading = regexp.MustCompile(`(?i)^ {0,3}##[ \t]+(?:\[unreleased\]|unreleased)(?:[ \t]+[—–-][ \t]+[^\r\n]+|[ \t]+\([^()\r\n]+\))?[ \t]*$`)
var rgaReleaseHeading = regexp.MustCompile(`^ {0,3}##[ \t]+(?:\[v?[0-9]+\.[0-9]+\.[0-9]+\]|v?[0-9]+\.[0-9]+\.[0-9]+)(?:[ \t]+[—–-][ \t]+[0-9]{4}-[0-9]{2}-[0-9]{2}(?:[ \t]+[—–-][ \t]+[^\r\n]+)?)?[ \t]*$`)
var rgaVersionHeadingCandidate = regexp.MustCompile(`^ {0,3}##[ \t]+\[?v?([0-9]+\.[0-9]+\.[0-9]+)(?:[^0-9.]|$)`)
var rgaUnshippedQualifier = regexp.MustCompile(`(?i)\b(?:planned|unreleased|upcoming|draft|unshipped|not released)\b`)

func rgaUnreleasedSection(body string) (string, error) {
	return rgaChangelogSection(body, "")
}

// An empty release selects Unreleased for the retained pre-release controls.
// Published/candidate contracts pin an exact version instead of dropping into
// the historical exemption when their heading graduates from Unreleased.
func rgaChangelogSection(body, release string) (string, error) {
	currentHeading, validCurrentHeading := rgaCurrentHeading, rgaUnreleasedHeading
	currentName := "Unreleased"
	if release != "" {
		currentHeading = regexp.MustCompile(`^ {0,3}##[ \t]+\[?v?` + regexp.QuoteMeta(release) + `(?:[^0-9.]|$)`)
		validCurrentHeading = rgaReleaseHeading
		currentName = "v" + release
	}
	currentCount, offset, historyStart := 0, 0, len(body)
	var fence byte
	fenceLength := 0
	for _, raw := range strings.SplitAfter(body, "\n") {
		lineStart := offset
		offset += len(raw)
		line := strings.TrimRight(raw, "\r\n")
		trimmed := strings.TrimLeft(line, " ")
		if len(line)-len(trimmed) <= 3 && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")) {
			length := 0
			for length < len(trimmed) && trimmed[length] == trimmed[0] {
				length++
			}
			if fence == 0 {
				fence, fenceLength = trimmed[0], length
			} else if trimmed[0] == fence && length >= fenceLength && strings.TrimSpace(trimmed[length:]) == "" {
				fence, fenceLength = 0, 0
			}
			continue
		}
		if fence != 0 || !rgaChangelogHeading.MatchString(line) {
			continue
		}
		switch {
		case currentHeading.MatchString(line):
			currentCount++
			if currentCount != 1 || !validCurrentHeading.MatchString(line) || historyStart != len(body) ||
				(release != "" && rgaUnshippedQualifier.MatchString(line)) {
				return "", fmt.Errorf("duplicate, misplaced or unsupported current heading: %q", line)
			}
		case rgaCurrentHeading.MatchString(line):
			return "", fmt.Errorf("unexpected Unreleased section beside selected %s contract", currentName)
		case rgaReleaseHeading.MatchString(line):
			if release != "" {
				match := rgaVersionHeadingCandidate.FindStringSubmatch(line)
				if len(match) != 2 || !rgaOlderVersion(match[1], release) {
					return "", fmt.Errorf("release is not older than selected %s: %q", currentName, line)
				}
			}
			if historyStart == len(body) {
				if rgaUnshippedQualifier.MatchString(line) {
					return "", fmt.Errorf("unshipped version is not a historical release: %q", line)
				}
				if currentCount != 1 {
					return "", fmt.Errorf("historical release precedes %s guidance: %q", currentName, line)
				}
				historyStart = lineStart
			}
		default:
			if release != "" {
				if match := rgaVersionHeadingCandidate.FindStringSubmatch(line); len(match) == 2 &&
					!rgaOlderVersion(match[1], release) {
					return "", fmt.Errorf("unsupported current or newer version heading: %q", line)
				}
			}
			if historyStart == len(body) {
				return "", fmt.Errorf("unknown current changelog heading: %q", line)
			}
		}
	}
	if fence != 0 && historyStart == len(body) {
		return "", fmt.Errorf("unterminated changelog code fence")
	}
	if currentCount != 1 {
		return "", fmt.Errorf("want exactly one %s heading, got %d", currentName, currentCount)
	}
	// Keep preamble and additional current notes. Only a release boundary,
	// never an arbitrary heading (or a heading inside a fence), ends guidance.
	return body[:historyStart], nil
}

func rgaOlderVersion(candidate, selected string) bool {
	parse := func(version string) ([]uint64, bool) {
		parts := strings.Split(version, ".")
		if len(parts) != 3 {
			return nil, false
		}
		numbers := make([]uint64, 3)
		for i, part := range parts {
			number, err := strconv.ParseUint(part, 10, 64)
			if err != nil {
				return nil, false
			}
			numbers[i] = number
		}
		return numbers, true
	}
	left, leftOK := parse(candidate)
	right, rightOK := parse(selected)
	if !leftOK || !rightOK {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return left[i] < right[i]
		}
	}
	return false
}

func rgaPlain(body string) string {
	body = strings.NewReplacer("`", "", "*", "", "–", "-", "—", "-", "\u00a0", " ").Replace(body)
	return strings.ToLower(strings.Join(strings.Fields(body), " "))
}

var rgaRequiredDisclaimers = []struct {
	name string
	re   *regexp.Regexp
}{
	{"necessary-not-sufficient", regexp.MustCompile(`\bcoverage is necessary,?\s+(?:but\s+)?not sufficient,?\s+for future replay eligibility\b`)},
	{"not-cross-base-safety", regexp.MustCompile(`\b(?:it|coverage)\s+is not cross[- ]base safety\b`)},
	{"warning-not-eligibility", regexp.MustCompile(`\bwarn\s*/\s*exit\s*0\s+coverage row is not eligibility\b`)},
	{"warning-no-replay-permission", regexp.MustCompile(`\bnever grants replay permission\b`)},
}

// Capture group 1 is the authorizing predicate. Negation is checked at that
// predicate, not anywhere in the document or line: a surviving disclaimer
// cannot hide a contradictory second assertion.
const rgaEditorFailureCondition = `(?:editor(?:[- ]process)? failures?|editor(?: process)? fail(?:s|ed)?)`
const rgaEditorSuccessAction = `((?:(?:returns?|exits?|reports?|return|exit)\s+(?:(?:a|an|with|exit|status|code)\s+){0,3}(?:zero|0|success)(?:\s+(?:exit|status|code)){0,2}|succeeds?))`

var rgaOverclaims = []struct {
	name string
	re   *regexp.Regexp
}{
	{"replay-or-cross-base-authority", regexp.MustCompile(
		`\b(?:coverage(?:[_ -]status)?|complete|incomplete|recipes?|warn\s*/\s*exit\s*0|it|the pair)\b[^.!?;]{0,100}?\b` +
			`(is|are|means?|makes?|proves?|guarantees?|certifies?|establishes?|grants?|permits?|authorizes?|allows?)\s+` +
			`(?:(?:automatically|therefore|now|always|fully|inherently|a|the|it|them|future|replay)\s+){0,4}` +
			`(?:replay[- ]safe|safe (?:for|to) replay|cross[- ]base[- ]safe(?:ty)?|cross[- ]base safety|eligible(?: for replay)?|eligibility|replay eligibility|replay permission|permission to replay|replay(?:ing)?\b)`,
	)},
	{"arrow-authority", regexp.MustCompile(
		`\b(?:complete(?: coverage)?|coverage|warn\s*/\s*exit\s*0(?: coverage row)?)\s*(=>|→|implies)\s*` +
			`(?:replay[- ]safe|cross[- ]base[- ]safe(?:ty)?|eligible|eligibility|replay permission)`,
	)},
	{"coverage-is-sufficient", regexp.MustCompile(
		`\b(?:coverage|complete|it)\b[^.!?;]{0,60}?\b(is|are|provides?)\s+` +
			`(?:alone\s+)?(?:sufficient|enough)(?: by itself)?\s+(?:for|to permit)\s+(?:future\s+)?replay(?: eligibility| permission)?\b`,
	)},
	{"coverage-as-authentication", regexp.MustCompile(
		`\b(?:coverage|capture[- ]event(?: evidence)?|capture evidence|companion|the pair|c(?: and e)?|e|it)\b[^.!?;]{0,100}?\b` +
			`(is|are|proves?|authenticates?|certifies?|establishes?|guarantees?|provides?)\s+` +
			`(?:(?:the|its|a|an|independent|cryptographic|authenticated|historical)\s+){0,4}` +
			`(?:authentication|authorship|authenticity|history|historical proof|historical origin|provenance|producer identity|producer|proof of origin)`,
	)},
	{"broad-operation-completeness", regexp.MustCompile(
		`\b(?:append-file|replace-in-file|ungated (?:write-file|writes?)|(?:all|any|every) (?:recipe )?operations?)\b[^.!?;]{0,80}?\b` +
			`(is|are|can|may|satisfies|satisfy|yields?|achieves?|supports?|qualif(?:y|ies))\s+` +
			`(?:(?:also|now|always|still|satisfy|achieve|yield|support|qualify for|for|v1)\s+){0,3}` +
			`(?:complete(?: coverage)?|completeness|coverage[- ]complete)`,
	)},
	{"complete-includes-broad-operations", regexp.MustCompile(
		`\b(?:complete coverage|v1 completeness|coverage[- ]complete)\b[^.!?;]{0,40}?\b` +
			`(includes?|supports?|admits?|allows?)\s+(?:all\s+)?(?:append-file|replace-in-file|ungated writes?|operations?)\b`,
	)},
	{"coverage-transaction", regexp.MustCompile(
		`\b(?:coverage|capture[- ]event|e and c|the pair|publication)\b[^.!?;]{0,60}?\b` +
			`(is|are|provides?|guarantees?)\s+(?:(?:a|an|one|single|atomic|multi-file)\s+){0,3}(?:transaction|transactional publication)\b`,
	)},
	{"doctor-writes-evidence", regexp.MustCompile(
		`\b(?:doctor|d10)\b[^.!?;]{0,45}?\b(writes?|repairs?|regenerates?|backfills?|publishes?)\s+` +
			`(?:(?:the|missing|stale|recipe|capture)\s+){0,3}(?:coverage|evidence|pair|recipe-capture-event)\b`,
	)},
	{"reversed-pair-publication", regexp.MustCompile(
		`\bc(?:\s+atomically)?\s+(before)\s+e\b`,
	)},
	{"editor-failure-success", regexp.MustCompile(
		`\b(?:tpatch edit|the command)(?: now)?\s+` + rgaEditorSuccessAction +
			`\s+(?:even\s+)?(?:when|if|though|despite|after)\s+(?:(?:the|an|a)\s+)?` + rgaEditorFailureCondition + `\b`,
	)},
	{"editor-failure-success", regexp.MustCompile(
		`\b` + rgaEditorFailureCondition + `(?:\s+(?:makes?|causes?)\s+|,\s*)` +
			`(?:tpatch edit|the command)(?:\s+to)?\s+` + rgaEditorSuccessAction + `\b`,
	)},
}

var rgaParagraphBoundary = regexp.MustCompile(`\r?\n[ \t]*\r?\n`)
var rgaSentenceBoundary = regexp.MustCompile(`[.!?;]\s+`)
var rgaClauseBoundary = regexp.MustCompile(`(?:[.!?;]\s+|\s+\b(?:but|however|yet|whereas|although)\b\s+)`)
var rgaCoordinationBoundary = regexp.MustCompile(`\s+(?:and|or|but|however|yet|whereas|although)\s+`)
var rgaNegatedBefore = regexp.MustCompile(`\b(?:not|never|cannot|can't|neither)\s*$`)
var rgaNegatedAfter = regexp.MustCompile(`^\s*(?:not|never|no)\b`)
var rgaNeitherSubject = regexp.MustCompile(`^\s*neither (?:recipe )?coverage nor (?:a )?producer label\s*$`)

const rgaClaimSubject = `(?:c and e|e and c|(?:recipe |complete |incomplete )?coverage(?:[_ -]status| record| row| pair)?|complete|incomplete|recipes?|warn\s*/\s*exit\s*0(?: coverage row)?|capture[- ]event(?: evidence)?|capture evidence|companion|the pair|publication|doctor|d10|append-file|replace-in-file|ungated (?:write-file|writes?)|(?:all|any|every) (?:recipe )?operations?|it|c|e)`
const rgaFinitePredicate = `(?:is|are|means?|makes?|proves?|guarantees?|certifies?|establishes?|grants?|permits?|authorizes?|allows?|authenticates?|provides?|can|may|satisfies|satisfy|yields?|achieves?|supports?|qualif(?:y|ies)|includes?|admits?|writes?|repairs?|regenerates?|backfills?|publishes?|cannot|can't)`
const rgaPredicatePhrase = `(?:(?:do|does|must|will|would|should)\s+)?(?:(?:not|never|also|still|now|always|automatically|therefore|alone)\s+)*` + rgaFinitePredicate + `\b`

var rgaSubjectPredicate = regexp.MustCompile(`\b(` + rgaClaimSubject + `)\s+` + rgaPredicatePhrase)
var rgaLeadingPredicate = regexp.MustCompile(`^` + rgaPredicatePhrase)
var rgaAnyPredicate = regexp.MustCompile(`\b` + rgaFinitePredicate + `\b`)

// Expand elided subjects across coordinated predicates, not their polarity.
// Explicit new subjects replace the owner; an unrecognized subject with its
// own predicate clears it instead of attributing that subject's claims to C.
func rgaCoordinatedClaims(sentence string) []string {
	claims := []string{sentence}
	subject, previous := "", 0
	boundaries := rgaCoordinationBoundary.FindAllStringIndex(sentence, -1)
	for i, boundary := range boundaries {
		before := strings.TrimSpace(sentence[previous:boundary[0]])
		explicit := rgaSubjectPredicate.FindAllStringSubmatch(before, -1)
		if len(explicit) != 0 {
			subject = explicit[len(explicit)-1][1]
		} else if previous == 0 || (!rgaLeadingPredicate.MatchString(before) && rgaAnyPredicate.MatchString(before)) {
			subject = ""
		}
		end := len(sentence)
		if i+1 < len(boundaries) {
			end = boundaries[i+1][0]
		}
		next := strings.TrimSpace(sentence[boundary[1]:end])
		if subject != "" && rgaLeadingPredicate.MatchString(next) {
			claims = append(claims, subject+" "+next)
		}
		previous = boundary[1]
	}
	return claims
}

func rgaNegatedPredicate(clause string, start, end int) bool {
	before, after := clause[:start], clause[end:]
	return rgaNegatedBefore.MatchString(before) ||
		rgaNegatedAfter.MatchString(after) ||
		rgaNeitherSubject.MatchString(before)
}

func validateRecipeAuthoritySurface(body string, installed bool) error {
	var clauses []string
	for _, paragraph := range rgaParagraphBoundary.Split(body, -1) {
		for _, sentence := range rgaSentenceBoundary.Split(rgaPlain(paragraph), -1) {
			for _, claim := range rgaCoordinatedClaims(sentence) {
				clauses = append(clauses, rgaClauseBoundary.Split(claim, -1)...)
			}
		}
	}
	for _, clause := range clauses {
		for _, rule := range rgaOverclaims {
			for _, match := range rule.re.FindAllStringSubmatchIndex(clause, -1) {
				if !rgaNegatedPredicate(clause, match[2], match[3]) {
					return fmt.Errorf("%s: %q", rule.name, clause[match[0]:match[1]])
				}
			}
		}
	}
	plain := rgaPlain(body)
	for _, required := range rgaRequiredDisclaimers {
		if !required.re.MatchString(plain) {
			return fmt.Errorf("missing %s disclaimer", required.name)
		}
	}
	if !installed {
		return nil
	}
	if err := rgaValidateProducers(body); err != nil {
		return err
	}
	return rgaValidateInstalledContract(body)
}

var rgaProducers = []struct {
	id       string
	identity patchobs.ProducerID
	commands []string
}{
	{"P1", patchobs.ProducerRecord, []string{"tpatch record <slug>", "tpatch land <slug>", "--auto", "--from", "--to", "--staged", "--unstaged"}},
	{"P2", patchobs.ProducerFeaturePatch, []string{"tpatch feature patch refresh <slug>", "tpatch feature patch fixup <slug>", "--reason"}},
	{"P3", patchobs.ProducerReconcileAccept, []string{"tpatch reconcile --accept <slug>", "--resolve --apply"}},
	{"P4", patchobs.ProducerCycle, []string{"tpatch cycle <slug>", "P6"}},
	{"P5", patchobs.ProducerApplyDone, []string{"tpatch apply <slug> --mode done"}},
	{"P6", patchobs.ProducerImplement, []string{"tpatch implement <slug>", "tpatch implement <slug> --manual", "no-capture"}},
	{"P7", patchobs.ProducerEdit, []string{"tpatch edit <slug> artifacts/apply-recipe.json", "tpatch edit <slug> artifacts/post-apply.patch"}},
}

var rgaProducerRow = regexp.MustCompile(`^\|[ \t]*P[0-9]+[ \t]*\|`)

func rgaValidateProducers(body string) error {
	rows := make(map[string][]string)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !rgaProducerRow.MatchString(line) {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) != 3 {
			return fmt.Errorf("producer row has %d cells, want 3: %s", len(cells), line)
		}
		id := strings.TrimSpace(cells[0])
		if _, found := rows[id]; found {
			return fmt.Errorf("duplicate producer %s", id)
		}
		rows[id] = cells
	}
	if len(rows) != len(rgaProducers) || len(rgaProducers) != 7 {
		return fmt.Errorf("producer inventory has %d rows, want exactly P1-P7", len(rows))
	}
	for _, producer := range rgaProducers {
		cells, exists := rows[producer.id]
		if !exists || rgaPlain(cells[1]) != string(producer.identity) || !patchobs.KnownProducer(producer.identity) {
			return fmt.Errorf("%s identity must be %q", producer.id, producer.identity)
		}
		for _, command := range producer.commands {
			present := strings.Contains(cells[2], command)
			if strings.HasPrefix(command, "tpatch ") {
				present = containsExactCommand(cells[2], command)
			}
			if !present {
				return fmt.Errorf("%s missing command/event alias %q", producer.id, command)
			}
		}
	}
	return nil
}

func rgaValidateInstalledContract(body string) error {
	// Scope the numbered checklist to the authority section, not unrelated
	// phase ordering or older numbered examples elsewhere in the skill.
	const heading = "## Recipe generation and coverage authority"
	start := strings.Index(body, heading)
	if start < 0 {
		return fmt.Errorf("missing installed authority section")
	}
	section := body[start+len(heading):]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	if err := rgaRequireEditorFailureDisclosure(section); err != nil {
		return err
	}
	for _, required := range []string{
		"recipe-coverage.json", "recipe-capture-event.json", "not a transaction",
		"unkeyed consistency", "not authenticated authorship or historical",
		"independently reconstruct content and trees", "--no-recipe-autogen",
		"D16", "canonical recipe", "raw bytes", "provenance",
		"--regenerate-recipe", "complete derivation", "preserve",
		"producer-patch-rewrite", "recipe-not-regenerated", "Formatting-only",
		"ADR-040", "ADR-039", "present, non-null", `explicit ` + "`\"\"`",
		"operation-not-reclassifiable", "all ten predicates",
		"reference-tree-only", "consumer-derivation-required", "unsupported",
		"recipe_generation_coverage", "recipe-coverage-capture-evidence-invalid",
		"recipe-generation-incomplete", "warn/verify-green absent other",
		"ADR-042", "no-write exemption", "rechecked", "Applied", "Skipped",
		"[write-file] <path>: already present (exact postimage), no write",
		"8 MiB", "supersession severity", "created_by",
		"doctor --check D10", "read-only", "warning-only", "Fixable:false",
		"even with", "--fix", "dry feasibility", "pair publication",
		"recipe-generation-no-truthful-regeneration",
	} {
		if !strings.Contains(rgaPlain(section), rgaPlain(required)) {
			return fmt.Errorf("installed contract missing %q", required)
		}
	}
	order := regexp.MustCompile(`\be(?: is published)? atomically(?:,? then| before) c atomically last\b`)
	if !order.MatchString(rgaPlain(section)) {
		return fmt.Errorf("installed contract missing atomic E-before-C final publication")
	}
	predicates := regexp.MustCompile(`(?m)^([0-9]+)\. (.+)$`).FindAllStringSubmatch(section, -1)
	if len(predicates) != 10 {
		return fmt.Errorf("installed completeness checklist has %d predicates, want 10", len(predicates))
	}
	for i, predicate := range predicates {
		if predicate[1] != fmt.Sprint(i+1) {
			return fmt.Errorf("completeness predicate %d is missing or out of order", i+1)
		}
	}
	for index, required := range [][]string{
		{"present canonical patch", "strict-parsed", "at least one effect"},
		{"durable", "reference.kind: commit"},
		{"readable", "decodable", "owned"},
		{"every normalized patch effect", "exactly once", "no extra effects"},
		{"every operation", "assigned", "no surplus operations"},
		{"repository-safe", "no two operations"},
		{"every effect", "represented", "reason arrays", "empty"},
		{"required sides", "observed", "modes/hashes", "present sides"},
		{"immutable-preimage simulation", "exact postimage bytes", "existence", "supported modes", "unmodeled changes"},
		{"reclassification", "every operation", "already-present", "writes no byte"},
	} {
		for _, phrase := range required {
			if !strings.Contains(rgaPlain(predicates[index][2]), phrase) {
				return fmt.Errorf("completeness predicate %d missing %q", index+1, phrase)
			}
		}
	}
	if strings.Contains(body, "--mode reapply") {
		return fmt.Errorf("invented --mode reapply")
	}
	return nil
}

var rgaEditorFailureRequirements = []struct {
	name string
	re   *regexp.Regexp
}{
	{"editor failure is non-zero despite publication success", regexp.MustCompile(
		`\btpatch edit (?:now )?returns a non[- ]zero exit when the editor process fails, even if coverage publication succeeds\b`)},
	{"inspection still occurs", regexp.MustCompile(`\bstill inspects bound[- ]artifact changes\b`)},
	{"publication is attempted before returning the editor error", regexp.MustCompile(
		`\battempts coverage publication for changed canonical recipe/patch bytes before returning the editor error\b`)},
	{"non-zero does not imply rollback", regexp.MustCompile(
		`\ba non[- ]zero exit does not imply that saved edits were rolled back\b`)},
	{"no numeric editor exit passthrough", regexp.MustCompile(
		`\ban editor[- ]only failure normally returns tpatch exit 1, not the editor's exact exit code\b`)},
}

func rgaRequireEditorFailureDisclosure(body string) error {
	plain := rgaPlain(body)
	for _, required := range rgaEditorFailureRequirements {
		if !required.re.MatchString(plain) {
			return fmt.Errorf("missing editor-failure disclosure: %s", required.name)
		}
	}
	return nil
}

func TestRecipeAuthorityEditorFailureDisclosureSensitivity(t *testing.T) {
	block := regexp.MustCompile(`(?ms)^(?:\*\*Editor failures:\*\*|- \*\*Editor failure exit status:\*\*).*?(?:\r?\n\r?\n|\z)`)
	for _, surface := range []string{"installed", "SPEC.md", "CHANGELOG.md"} {
		t.Run(surface, func(t *testing.T) {
			good := rgaEmbeddedSurface(t, skillFiles[0].path)
			if surface != "installed" {
				raw, err := os.ReadFile(filepath.Join("..", surface))
				if err != nil {
					t.Fatal(err)
				}
				good = string(raw)
			}
			validate := func(body string) error {
				if surface == "installed" {
					return validateRecipeAuthoritySurface(body, true)
				}
				return rgaValidatePublicSurface(surface, body)
			}
			if err := validate(good); err != nil {
				t.Fatalf("actual disclosure baseline: %v", err)
			}
			spans := block.FindAllStringIndex(good, -1)
			if len(spans) != 1 {
				t.Fatalf("want one editor disclosure mutation target, got %d", len(spans))
			}
			start, end := spans[0][0], spans[0][1]
			disclosure := good[start:end]
			without := good[:start] + good[end:]
			if err := validate(without); err == nil || !strings.Contains(err.Error(), "editor-failure disclosure") {
				t.Fatalf("same surface validator accepted disclosure removal: %v", err)
			}
			zero := strings.Replace(disclosure, "a non-zero exit", "a zero exit", 1)
			if zero == disclosure {
				t.Fatal("zero-exit mutation did not change its target")
			}
			appendClaim := func(claim string) string {
				return good[:end] + "\n" + claim + "\n\n" + good[end:]
			}
			for name, wrong := range map[string]string{
				"zero-exit-replacement":  good[:start] + zero + good[end:],
				"appended-zero-exit":     appendClaim("tpatch edit exits zero even when the editor process fails."),
				"appended-failure-first": appendClaim("An editor-process failure makes tpatch edit exit zero."),
			} {
				t.Run(name, func(t *testing.T) {
					if err := validate(wrong); err == nil || !strings.Contains(err.Error(), "editor-failure-success") {
						t.Fatalf("same surface validator accepted a zero-exit claim: %v", err)
					}
				})
			}
			for _, truthful := range []string{
				"tpatch edit never exits zero when the editor process fails.",
				"Coverage publication succeeds even when the editor process fails.",
				"tpatch edit exits zero when no editor process fails.",
			} {
				if err := validate(appendClaim(truthful)); err != nil {
					t.Fatalf("truthful distinction rejected: %q: %v", truthful, err)
				}
			}
		})
	}
}

func TestRecipeAuthorityOverclaimSensitivity(t *testing.T) {
	good := rgaEmbeddedSurface(t, skillFiles[0].path)
	if err := validateRecipeAuthoritySurface(good, true); err != nil {
		t.Fatalf("positive installed baseline: %v", err)
	}
	for _, mutation := range []struct {
		name  string
		claim string
		code  string
	}{
		{"skill-line-claims-replay-safe", "Recipe coverage is replay-safe.", "replay-or-cross-base-authority"},
		{"complete-implies-cross-base-safe", "Complete coverage means cross-base safety.", "replay-or-cross-base-authority"},
		{"complete-arrow-cross-base-safe", "complete => cross-base-safe", "arrow-authority"},
		{"coverage-alone-sufficient", "Complete coverage is sufficient for future replay eligibility.", "coverage-is-sufficient"},
		{"warn-exit0-implies-eligible", "A warn/exit0 coverage row is eligible for replay.", "replay-or-cross-base-authority"},
		{"warn-arrow-eligible", "warn/exit0 => eligible", "arrow-authority"},
		{"coverage-as-authentication", "Coverage authenticates its producer.", "coverage-as-authentication"},
		{"coverage-as-history", "C and E prove historical origin.", "coverage-as-authentication"},
		{"broad-operation-completeness", "Append-file can satisfy v1 completeness.", "broad-operation-completeness"},
		{"exact-replacement-completeness", "Replace-in-file satisfies complete coverage.", "broad-operation-completeness"},
		{"ungated-write-completeness", "Ungated writes qualify for complete coverage.", "broad-operation-completeness"},
		{"reverse-broad-domain", "Complete coverage includes append-file.", "complete-includes-broad-operations"},
		{"pair-as-transaction", "E and C are one atomic transaction.", "coverage-transaction"},
		{"doctor-manufactures-pair", "Doctor --fix repairs the coverage pair.", "doctor-writes-evidence"},
		{"reversed-publication", "Publish C atomically before E.", "reversed-pair-publication"},
		{"same-line-good-then-bad", "Coverage is not cross-base safety, but coverage is replay-safe.", "replay-or-cross-base-authority"},
		{"unrelated-negation", "Coverage is not authenticated history and complete coverage is cross-base-safe.", "replay-or-cross-base-authority"},
		{"neither-is-not-global-exemption", "Neither coverage nor a producer label authorizes replay and coverage is replay-safe.", "replay-or-cross-base-authority"},
		{"good-token-block-survives", "Coverage is necessary, not sufficient, for future replay eligibility; coverage grants replay permission.", "replay-or-cross-base-authority"},
		{"formatted-overclaim", "**Complete coverage** is\n`replay-safe`.", "replay-or-cross-base-authority"},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			for _, installed := range []bool{false, true} {
				err := validateRecipeAuthoritySurface(good+"\n\n"+mutation.claim+"\n", installed)
				if err == nil || !strings.Contains(err.Error(), mutation.code) {
					t.Fatalf("appended contradiction not rejected by %s (installed=%v): %v", mutation.code, installed, err)
				}
			}
		})
	}
	for _, disclaimer := range []string{
		"Recipe coverage is not replay-safe.",
		"Complete coverage does not mean cross-base safety.",
		"Complete coverage is not sufficient for future replay eligibility.",
		"A warn / exit 0 coverage row is not eligible for replay.",
		"Coverage never grants replay permission.",
		"Coverage does not authenticate its producer.",
		"C and E do not prove historical origin.",
		"Append-file cannot satisfy v1 completeness.",
		"Replace-in-file does not satisfy complete coverage.",
		"Ungated writes do not qualify for complete coverage.",
		"Complete coverage does not include append-file.",
		"E and C are not one atomic transaction.",
		"Doctor --fix never repairs the coverage pair.",
		"Neither coverage nor a producer label authorizes replay.",
	} {
		t.Run("negation/"+disclaimer, func(t *testing.T) {
			if err := validateRecipeAuthoritySurface(good+"\n\n"+disclaimer, true); err != nil {
				t.Fatalf("explicit negation rejected: %v", err)
			}
		})
	}
}

func TestRecipeAuthorityContractSensitivity(t *testing.T) {
	good := rgaEmbeddedSurface(t, skillFiles[0].path)
	if err := validateRecipeAuthoritySurface(good, true); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct {
		name string
		from string
		to   string
		code string
	}{
		{"missing-necessary-not-sufficient", "Recipe coverage is necessary, not sufficient, for future replay eligibility;", "Recipe coverage is a diagnostic;", "necessary-not-sufficient"},
		{"missing-warning-limit", "A warn/exit0 coverage row is not eligibility and never grants replay permission.", "", "warning-not-eligibility"},
		{"missing-event-before-coverage-order", "then C atomically last", "then C eventually", "atomic E-before-C"},
		{"predicate-effect-totality", "Every normalized patch effect appears exactly once", "Some patch effects appear", "predicate 4"},
		{"predicate-surplus", "with no surplus operations", "with surplus operations allowed", "predicate 5"},
		{"predicate-no-write", "and writes no byte", "and may write bytes", "predicate 10"},
		{"command-suffix", "`tpatch record <slug>` (including", "`tpatch record <slug>-invented` (including", "alias"},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			bad := strings.Replace(good, mutation.from, mutation.to, 1)
			if bad == good {
				t.Fatalf("mutation target absent: %q", mutation.from)
			}
			if err := validateRecipeAuthoritySurface(bad, true); err == nil || !strings.Contains(err.Error(), mutation.code) {
				t.Fatalf("mutation did not fail its contract check %s: %v", mutation.code, err)
			}
		})
	}
}

func TestRecipeAuthorityProducerSensitivity(t *testing.T) {
	good := rgaEmbeddedSurface(t, skillFiles[0].path)
	if err := validateRecipeAuthoritySurface(good, true); err != nil {
		t.Fatal(err)
	}
	for _, producer := range rgaProducers {
		var row string
		for _, line := range strings.Split(good, "\n") {
			if strings.HasPrefix(line, "| "+producer.id+" |") {
				row = line
				break
			}
		}
		if row == "" {
			t.Fatalf("cannot locate %s mutation target", producer.id)
		}
		t.Run("missing-one-producer/"+producer.id, func(t *testing.T) {
			bad := strings.Replace(good, row+"\n", "", 1)
			if err := validateRecipeAuthoritySurface(bad, true); err == nil || !strings.Contains(err.Error(), "producer inventory") {
				t.Fatalf("removed producer was accepted: %v", err)
			}
		})
		t.Run("bad-producer-identity/"+producer.id, func(t *testing.T) {
			badRow := strings.Replace(row, "`"+string(producer.identity)+"`", "`invented-producer`", 1)
			bad := strings.Replace(good, row, badRow, 1)
			if err := validateRecipeAuthoritySurface(bad, true); err == nil || !strings.Contains(err.Error(), "identity") {
				t.Fatalf("wrong producer identity was accepted: %v", err)
			}
		})
		t.Run("wrong-command-alias/"+producer.id, func(t *testing.T) {
			badRow := strings.ReplaceAll(row, producer.commands[0], "tpatch invented <slug>")
			bad := strings.Replace(good, row, badRow, 1)
			if err := validateRecipeAuthoritySurface(bad, true); err == nil || !strings.Contains(err.Error(), "alias") {
				t.Fatalf("wrong command alias was accepted: %v", err)
			}
		})
	}
}

func TestRecipeAuthorityCurrentChangelogScope(t *testing.T) {
	history := "\n## [0.16.0]\nHistorical release prose need not assert future coverage behavior.\n"
	compact := "Recipe coverage is necessary, not sufficient, for future replay eligibility; it is not cross-base safety.\n" +
		"A warn/exit0 coverage row is not eligibility and never grants replay permission.\n"
	for _, heading := range []string{
		"## [Unreleased]", "## Unreleased",
		"## Unreleased — v0.17 planned — recipe generation authority",
		"## Unreleased (v0.17 planned)", "## [Unreleased] (v0.17 planned)",
	} {
		current, err := rgaUnreleasedSection("# Changelog\n\n" + heading + "\n" + compact + history)
		if err != nil {
			t.Fatalf("current-section heading %q: %v", heading, err)
		}
		if err := validateRecipeAuthoritySurface(current, false); err != nil {
			t.Fatalf("current-section positive control: %v", err)
		}
	}

	bad, err := rgaUnreleasedSection("# Changelog\n\n## [Unreleased]\n" + compact +
		"\n### Added\nCoverage is replay-safe.\n" + history)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRecipeAuthoritySurface(bad, false); err == nil || !strings.Contains(err.Error(), "replay-or-cross-base-authority") {
		t.Fatalf("Unreleased subsection accidentally exempted: %v", err)
	}
	for _, malformed := range []string{
		"# Changelog\n## [0.16.0]\n" + compact,
		"## [Unreleased]\n" + compact + "\n## [Unreleased]\n" + compact,
		"## Unreleased archive\n" + compact,
		"## Unreleased — planned\n" + compact + "\n## [Unreleased]\n" + compact,
	} {
		if _, err := rgaUnreleasedSection(malformed); err == nil {
			t.Fatal("missing/duplicate Unreleased heading accepted")
		}
	}
}

func TestRecipeAuthorityVersionedChangelogScope(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	good := string(raw)
	const heading = "## v0.17.0 — 2026-09-19 — recipe generation authority"
	boundary := strings.Index(good, "\n## v0.16.0 ")
	if strings.Count(good, heading) != 1 || boundary < 0 {
		t.Fatal("versioned mutation fixture must pin the actual current and historical headings")
	}
	current, history := good[:boundary], good[boundary:]
	validate := func(body string) error { return rgaValidatePublicSurface("CHANGELOG.md", body) }
	if err := validate(good); err != nil {
		t.Fatalf("actual versioned baseline: %v", err)
	}
	for _, alternative := range []string{
		"## v0.17.0", "## [0.17.0]", "## [v0.17.0] - 2026-09-19",
	} {
		if err := validate(strings.Replace(good, heading, alternative, 1)); err != nil {
			t.Fatalf("supported version heading %q: %v", alternative, err)
		}
	}
	for name, wrong := range map[string]string{
		"missing-current":                 "# Changelog\n" + history,
		"wrong-version":                   strings.Replace(good, heading, "## v0.16.9", 1),
		"unreleased-only-is-not-v017":     strings.Replace(good, heading, "## Unreleased", 1),
		"unexpected-unreleased-before":    strings.Replace(good, heading, "## Unreleased\n\n"+heading, 1),
		"unexpected-unreleased-after":     good + "\n## Unreleased\n",
		"duplicate-current":               current + "\n" + heading + "\n" + history,
		"duplicate-current-after-history": good + "\n## [v0.17.0]\n",
		"linked-current-after-history":    good + "\n## [v0.17.0](#release)\nCoverage is replay-safe.\n",
		"unsupported-current-suffix":      good + "\n## v0.17.0-rc1\nCoverage is replay-safe.\n",
		"newer-minor-false-history":       current + "\n## v0.18.0\nCoverage is replay-safe.\n" + history,
		"newer-patch-false-history":       current + "\n## v0.17.1\nCoverage is replay-safe.\n" + history,
		"newer-major-false-history":       current + "\n## v1.0.0\nCoverage is replay-safe.\n" + history,
		"newer-after-history":             good + "\n## v0.18.0\nCoverage is replay-safe.\n",
		"linked-newer-after-history":      good + "\n## [v0.18.0](#release)\nCoverage is replay-safe.\n",
		"numeric-version-overflow":        current + "\n## v999999999999999999999999.0.0\n" + history,
		"equal-version-leading-zero":      good + "\n## v0.017.0\n",
		"current-after-older-version":     strings.Replace(good, heading, "## v0.16.9\n\n"+heading, 1),
		"unknown-current-boundary":        current + "\n## Release notes\nCoverage is replay-safe.\n" + history,
		"fenced-current-heading":          strings.Replace(good, heading, "```markdown\n"+heading+"\n```", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := validate(wrong); err == nil {
				t.Fatal("same public validator accepted an invalid current-version boundary")
			}
		})
	}
	for name, addition := range map[string]string{
		"versioned-replay-overclaim": "Coverage is replay-safe.",
		"versioned-editor-overclaim": "tpatch edit exits zero even when the editor process fails.",
		"fenced-false-history":       "```markdown\n## v0.16.0\nCoverage is replay-safe.\n```",
	} {
		t.Run(name, func(t *testing.T) {
			if err := validate(current + "\n\n" + addition + "\n" + history); err == nil {
				t.Fatal("graduating the heading silently exempted the current contract")
			}
		})
	}
	// A historical copy cannot substitute for the current version's disclosure.
	const notice = "even if coverage publication succeeds"
	if strings.Count(current, notice) != 1 {
		t.Fatal("editor-disclosure mutation target absent or ambiguous")
	}
	withoutDisclosure := strings.Replace(current, notice, "only if publication fails", 1)
	if err := validate(withoutDisclosure + history + "\n" + current[strings.Index(current, heading)+len(heading):]); err == nil ||
		!strings.Contains(err.Error(), "editor-failure disclosure") {
		t.Fatalf("old release prose supplied missing current disclosure: %v", err)
	}
	if err := validate(current + history + "\nCoverage is replay-safe.\n"); err != nil {
		t.Fatalf("genuinely older release prose became current authority: %v", err)
	}
	for _, older := range []string{"0.16.99", "0.9.0", "0.0.0"} {
		if err := validate(current + "\n## v" + older + "\nCoverage is replay-safe.\n"); err != nil {
			t.Fatalf("numeric older-version boundary %s rejected: %v", older, err)
		}
	}
	for _, olderHeading := range []string{
		"## v0.5.3 — Shadow Accept Accounting Fixes (Tranche C3)",
		"## v0.4.1 and earlier",
	} {
		if err := validate(current + history + "\n" + olderHeading + "\nCoverage is replay-safe.\n"); err != nil {
			t.Fatalf("existing older notes after a real history boundary rejected: %v", err)
		}
	}
}

func TestRecipeAuthorityCoordinatedPredicateSensitivity(t *testing.T) {
	good := rgaEmbeddedSurface(t, skillFiles[0].path)
	for _, installed := range []bool{false, true} {
		t.Run(fmt.Sprintf("installed=%v", installed), func(t *testing.T) {
			if err := validateRecipeAuthoritySurface(good, installed); err != nil {
				t.Fatalf("positive baseline: %v", err)
			}
			for _, fixture := range []struct {
				name  string
				claim string
				code  string
			}{
				{"negative-then-replay-safe", "Coverage never grants replay permission and is replay-safe.", "replay-or-cross-base-authority"},
				{"negative-then-cross-base-safe", "Coverage does not grant replay permission and is cross-base-safe.", "replay-or-cross-base-authority"},
				{"negative-then-replay-permission", "Coverage never grants replay permission and allows replay.", "replay-or-cross-base-authority"},
				{"two-negatives-then-authority", "Coverage never grants replay permission and is not replay-safe and is cross-base-safe.", "replay-or-cross-base-authority"},
				{"negative-then-historical-proof", "Coverage does not authenticate its producer and proves historical origin.", "coverage-as-authentication"},
				{"negative-then-broad-completeness", "Replace-in-file does not satisfy complete coverage and satisfies v1 completeness.", "broad-operation-completeness"},
				{"contrast-elided-subject", "Coverage never grants replay permission but is replay-safe.", "replay-or-cross-base-authority"},
				{"qualified-coordination", "Coverage never grants replay permission and still is cross-base-safe.", "replay-or-cross-base-authority"},
				{"explicit-subject-replaces-owner", "Coverage never grants replay permission and replace-in-file does not satisfy complete coverage and satisfies v1 completeness.", "broad-operation-completeness"},
			} {
				t.Run(fixture.name, func(t *testing.T) {
					err := validateRecipeAuthoritySurface(good+"\n\n"+fixture.claim+"\n", installed)
					if err == nil || !strings.Contains(err.Error(), fixture.code) {
						t.Fatalf("coordinated affirmative predicate escaped %s: %v", fixture.code, err)
					}
				})
			}
			for _, disclaimer := range []string{
				"Coverage never grants replay permission and is not replay-safe.",
				"Coverage does not grant replay permission and is not cross-base-safe.",
				"Coverage never grants replay permission and never allows replay.",
				"Coverage never grants replay permission and is not replay-safe and is not cross-base-safe.",
				"Coverage does not authenticate its producer and does not prove historical origin.",
				"Replace-in-file does not satisfy complete coverage and cannot satisfy v1 completeness.",
				"Coverage never grants replay permission but is not replay-safe.",
				"Coverage never grants replay permission and still is not cross-base-safe.",
				"Coverage never grants replay permission and replace-in-file does not satisfy complete coverage and cannot satisfy v1 completeness.",
				"Coverage never grants replay permission and the deployment is healthy and is eligible for replay.",
				"Coverage never grants replay permission. A deployment is healthy and is eligible for replay.",
			} {
				t.Run("negated/"+disclaimer, func(t *testing.T) {
					if err := validateRecipeAuthoritySurface(good+"\n\n"+disclaimer+"\n", installed); err != nil {
						t.Fatalf("coordinated negations rejected: %v", err)
					}
				})
			}
		})
	}
}

func TestRecipeAuthorityChangelogHeadingBoundarySensitivity(t *testing.T) {
	compact := "Recipe coverage is necessary, not sufficient, for future replay eligibility; it is not cross-base safety.\n" +
		"A warn/exit0 coverage row is not eligibility and never grants replay permission.\n"
	current := "# Changelog\n\n## Unreleased — v0.17 planned — recipe generation authority\n\n" + compact
	history := "\n## [0.16.0]\nHistorical release notes.\n"
	check := func(document string) error {
		section, err := rgaUnreleasedSection(document)
		if err != nil {
			return err
		}
		return validateRecipeAuthoritySurface(section, false)
	}
	if err := check(current + history); err != nil {
		t.Fatalf("positive current/historical baseline: %v", err)
	}
	for _, fixture := range []struct {
		name    string
		heading string
	}{
		{"parenthesized-duplicate-unreleased", "## Unreleased (v0.17 planned)"},
		{"bracketed-decorated-duplicate", "## [Unreleased] (v0.17 planned)"},
		{"non-release-current-notes", "## Current implementation notes"},
		{"planned-version-is-not-release", "## v0.17 planned notes"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			for _, content := range []string{
				"Coverage is replay-safe.",
				"Current implementation notes; no additional authority claimed.",
			} {
				t.Run(content, func(t *testing.T) {
					document := current + "\n" + fixture.heading + "\n" + content + "\n" + history
					if err := check(document); err == nil {
						t.Fatalf("unknown/duplicate current heading silently hid current text: %q", fixture.heading)
					}
				})
			}
		})
	}
	for _, heading := range []string{
		"## [0.16.0]",
		"## [0.16.0] - 2026-08-31",
		"## v0.16.0 — 2026-08-31",
	} {
		t.Run("historical/"+heading, func(t *testing.T) {
			// This deliberately contradictory historical sentence must not be
			// mistaken for current guidance after an identified release boundary.
			document := current + "\n" + heading + "\nCoverage is replay-safe.\n"
			if err := check(document); err != nil {
				t.Fatalf("genuine historical release was treated as current guidance: %v", err)
			}
		})
	}
	for _, fixture := range []struct {
		name     string
		document string
	}{
		{"preamble-claim", "# Changelog\nCoverage is replay-safe.\n\n## [Unreleased]\n" + compact + history},
		{"preamble-notes", "# Changelog\n### Current notes\nCoverage is replay-safe.\n\n## [Unreleased]\n" + compact + history},
		{"additional-current-notes", current + "\n### Current notes\nCoverage is replay-safe.\n" + history},
		{"fenced-release-is-current", current + "\n```markdown\n## [0.16.0]\nCoverage is replay-safe.\n```\n" + history},
		{"tilde-fenced-release-is-current", current + "\n~~~markdown\n## [0.16.0]\nCoverage is replay-safe.\n~~~\n" + history},
		{"indented-release-is-current", current + "\n    ## [0.16.0]\nCoverage is replay-safe.\n" + history},
		{"tab-indented-release-is-current", current + "\n\t## [0.16.0]\nCoverage is replay-safe.\n" + history},
		{"planned-full-version-is-not-release", current + "\n## v0.17.0 - planned\nCoverage is replay-safe.\n" + history},
		{"dated-planned-version-is-not-release", current + "\n## v0.17.0 - 2026-09-30 - planned\nCoverage is replay-safe.\n" + history},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			if err := check(fixture.document); err == nil {
				t.Fatal("current guidance disappeared at a false section boundary")
			}
		})
	}
}
