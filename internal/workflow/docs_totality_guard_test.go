package workflow

// AC-L135 / PRD-verify-freshness §7.1.2 — the totality guard.
//
// A forbidden-phrase regex sweep (G1–G10) over the three authoritative
// documents. Every hit must be cleared by one of the four whitelist
// rules, or by a per-pattern voice exemption. This is a UNIT test over
// document bytes: it reads exactly three files and applies exactly the
// patterns of the §7.1.2 table.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type guardPattern struct {
	id string
	// re is the forbidden phrase.
	re *regexp.Regexp
	// scope, when non-nil, restricts the pattern to lines that also
	// match it (G5's base-commit scope, G8's refusal-context scope).
	scope *regexp.Regexp
	// exempt, when non-nil, clears a line that also matches it (the
	// per-pattern affirmative-voice exemptions of G5, G6 and G7).
	exempt *regexp.Regexp
	// pair, when non-empty, requires BOTH sub-patterns on the line
	// within 40 characters of each other (G7).
	pairA, pairB *regexp.Regexp
	// fenced restricts the pattern to fenced blocks that also contain
	// `"checks"` (G9).
	fencedWithChecks bool
}

func guardPatterns() []guardPattern {
	ci := func(expr string) *regexp.Regexp { return regexp.MustCompile(`(?i)` + expr) }
	return []guardPattern{
		{id: "G1", re: ci(`V9\s+is\s+last`)},
		{id: "G2", re: ci(`\bten[- ]check\b|\b10[- ]check\b|\b10[- ]row\b|exactly ten\b`)},
		{id: "G3", re: ci(`Amendment 1 rev-[0-6]\b|proposed rev-[0-6]\b`)},
		{id: "G4", re: ci(`land[’']?s? behaviou?r is unchanged|behaviou?r-frozen|behaviou?r-neutral`)},
		{
			id:     "G5",
			re:     ci(`40[- ](lowercase[- ])?hex|hardcode[sd]? 40|fixed 40`),
			scope:  ci(`Tpatch-Base-Commit|base_commit|BaseCommit`),
			exempt: ci(`derived|--show-object-format|would reject|rejects|fails this row`),
		},
		{
			id:     "G6",
			re:     ci(`absent.{0,60}mismatch|mismatch.{0,60}absent|any attested value mismatch`),
			exempt: ci(`never|not a mismatch|rather than a mismatch|no mismatch`),
		},
		{
			id:     "G7",
			pairA:  ci(`exact`),
			pairB:  ci(`absent|present-empty`),
			exempt: ci(`neither|cannot|never|not reachable|no row`),
		},
		{
			id:    "G8",
			re:    ci(`mutat(ing|es) nothing`),
			scope: ci(`base_commit|BaseCommit|R23|recoverLand|Mode A|Mode B|journal`),
		},
		{id: "G9", re: ci(`freshness_label`), fencedWithChecks: true},
		{id: "G10", re: ci(`E1–E4[0-6]\b`)},
	}
}

var (
	whitelistPrefix = regexp.MustCompile(`(?i)^(historical|superseded|rejected|withdrawn|pre-rev-5)\b`)
	whitelistBlock  = regexp.MustCompile(`(?i)(Revision history|Alternatives considered|Retraction)`)
	whitelistNegate = regexp.MustCompile(`(?i)\bnot\b|\bnever\b|\bno\b|cannot|neither|rather than|only when|only from`)
	guardSelfRef    = regexp.MustCompile(`AC-L135|^\| G\d`)
	punctStrip      = regexp.MustCompile(`^[\s>*\-+#|` + "`" + `]*(\*\*)?`)
)

// guardDocuments are the three authoritative inputs.
func guardDocuments(t *testing.T) map[string]string {
	t.Helper()
	root := docsRootForTest(t)
	out := map[string]string{}
	for _, rel := range []string{
		"docs/prds/PRD-verify-freshness.md",
		"docs/prds/PRD-tpatch-land.md",
		"docs/adrs/ADR-013-verify-freshness-overlay.md",
	} {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		out[rel] = string(data)
	}
	return out
}

func docsRootForTest(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Dir(filepath.Dir(wd))
}

// TestACL135_DocsTotalityGuard runs the G1–G10 sweep.
func TestACL135_DocsTotalityGuard(t *testing.T) {
	docs := guardDocuments(t)
	patterns := guardPatterns()

	for name, body := range docs {
		lines := strings.Split(body, "\n")
		inMarkedBlock := false
		inFence := false
		fenceLines := []string{}
		fenceStart := 0

		for i, raw := range lines {
			line := raw
			trimmed := strings.TrimSpace(line)

			// Track fenced blocks for G9.
			if strings.HasPrefix(trimmed, "```") {
				if inFence {
					checkFencedBlock(t, name, fenceStart, fenceLines)
					fenceLines = nil
					inFence = false
				} else {
					inFence = true
					fenceStart = i + 1
				}
				continue
			}
			if inFence {
				fenceLines = append(fenceLines, line)
				continue
			}

			// Marked-block tracking (whitelist rule 2).
			if strings.HasPrefix(trimmed, "#") {
				inMarkedBlock = whitelistBlock.MatchString(trimmed)
			} else if strings.HasPrefix(trimmed, "**") && whitelistBlock.MatchString(trimmed) {
				inMarkedBlock = true
			} else if trimmed == "" && inMarkedBlock {
				// A blank line does not end a marked list block; a new
				// heading does. Keep the block open.
				_ = trimmed
			}

			for _, p := range patterns {
				if p.fencedWithChecks {
					continue // handled per fenced block
				}
				if !patternHits(p, line) {
					continue
				}
				if cleared(p, line, inMarkedBlock) {
					continue
				}
				t.Errorf("%s:%d %s hit: %q", name, i+1, p.id, strings.TrimSpace(line))
			}
		}
		if inFence {
			checkFencedBlock(t, name, fenceStart, fenceLines)
		}
	}
}

func patternHits(p guardPattern, line string) bool {
	if p.pairA != nil && p.pairB != nil {
		locA := p.pairA.FindStringIndex(line)
		locB := p.pairB.FindStringIndex(line)
		if locA == nil || locB == nil {
			return false
		}
		d := locA[0] - locB[0]
		if d < 0 {
			d = -d
		}
		return d <= 40
	}
	if p.re == nil {
		return false
	}
	if !p.re.MatchString(line) {
		return false
	}
	if p.scope != nil && !p.scope.MatchString(line) {
		return false
	}
	return true
}

func cleared(p guardPattern, line string, inMarkedBlock bool) bool {
	stripped := punctStrip.ReplaceAllString(line, "")
	switch {
	case whitelistPrefix.MatchString(strings.TrimSpace(stripped)):
		return true
	case inMarkedBlock:
		return true
	case whitelistNegate.MatchString(line):
		return true
	case guardSelfRef.MatchString(line):
		return true
	case p.exempt != nil && p.exempt.MatchString(line):
		return true
	}
	return false
}

// checkFencedBlock implements G9: `freshness_label` inside a fenced block
// that also contains `"checks"`.
func checkFencedBlock(t *testing.T, doc string, startLine int, block []string) {
	t.Helper()
	joined := strings.Join(block, "\n")
	if !strings.Contains(joined, `"checks"`) {
		return
	}
	for i, line := range block {
		if strings.Contains(line, "freshness_label") {
			t.Errorf("%s:%d G9 hit: a verify report block must not carry freshness_label: %q",
				doc, startLine+i+1, strings.TrimSpace(line))
		}
	}
}

// TestACL135_GuardCoversTheNamedSections asserts the guard reads the
// right files: every named section must be present and non-empty, so the
// test cannot pass by reading the wrong document.
func TestACL135_GuardCoversTheNamedSections(t *testing.T) {
	docs := guardDocuments(t)
	required := map[string][]string{
		"docs/prds/PRD-verify-freshness.md": {
			"## 2. Goals / non-goals", "### 3.1 The check list", "#### 3.1.1",
			"### 3.4.3", "### 3.6 Landed-feature verification contract",
			"### 4.3 `tpatch verify --json` output schema", "## 5. Edge cases",
			"## 6. Open questions", "## 7. Acceptance criteria",
		},
		"docs/prds/PRD-tpatch-land.md": {
			"# PRD", "#### 3.8.2 Reader rules", "#### 3.8.6 Producer validation",
			"### 6.2 Landing-evidence acceptance rows",
		},
		"docs/adrs/ADR-013-verify-freshness-overlay.md": {
			"# ADR-013", "### D8.", "### D19.", "## Amendment 1 rev-7 — references",
		},
	}
	for doc, sections := range required {
		body := docs[doc]
		for _, section := range sections {
			idx := strings.Index(body, section)
			if idx < 0 {
				t.Errorf("%s: required section %q not found", doc, section)
				continue
			}
			rest := strings.TrimSpace(body[idx+len(section):])
			if len(rest) < 40 {
				t.Errorf("%s: section %q is empty", doc, section)
			}
		}
	}
}

// TestACL135_GuardIsSensitive proves the guard would catch a
// reintroduced stale claim — a guard that never fires proves nothing.
func TestACL135_GuardIsSensitive(t *testing.T) {
	cases := []struct {
		id   string
		line string
	}{
		{"G1", "The sequence ends because V9 is last."},
		{"G2", "Every report emits a ten-check array."},
		{"G3", "Binding: ADR-013 Amendment 1 rev-3 governs Wave C."},
		{"G4", "This amendment is behaviour-neutral for land."},
		{"G5", "`Tpatch-Base-Commit` is 40 lowercase hex."},
		{"G6", "An absent patch is reported as a digest mismatch."},
		{"G7", "An exact attestation with an absent patch is fine."},
		{"G8", "R23 refuses while mutating nothing."},
		{"G10", "The probe index is E1–E42."},
	}
	patterns := map[string]guardPattern{}
	for _, p := range guardPatterns() {
		patterns[p.id] = p
	}
	for _, tc := range cases {
		p, ok := patterns[tc.id]
		if !ok {
			t.Fatalf("unknown pattern %s", tc.id)
		}
		if !patternHits(p, tc.line) {
			t.Errorf("%s did not fire on %q", tc.id, tc.line)
			continue
		}
		if cleared(p, tc.line, false) {
			t.Errorf("%s was wrongly cleared for %q", tc.id, tc.line)
		}
	}
	// G9 fires only inside a fenced block that also carries "checks".
	fake := &testing.T{}
	checkFencedBlock(fake, "fake.md", 1, []string{`  "checks": [],`, `  "freshness_label": "verified-fresh"`})
	if !fake.Failed() {
		t.Errorf("G9 did not fire on a report block carrying freshness_label")
	}
}
