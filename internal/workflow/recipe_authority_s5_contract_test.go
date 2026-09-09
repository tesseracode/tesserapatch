package workflow

import (
	"fmt"
	"strings"
	"testing"
)

// These accepted-text pins complement, rather than replace, S5's runtime
// classifier and same-validator mutation tests.
func rgaS5ContractTable(doc, header string) (string, error) {
	needle := header + "\n"
	if strings.Count(doc, needle) != 1 {
		return "", fmt.Errorf("contract table header missing or ambiguous: %s", header)
	}
	start := strings.Index(doc, needle)
	if start > 0 && doc[start-1] != '\n' {
		return "", fmt.Errorf("contract table header is not a line")
	}
	end := start
	for _, line := range strings.SplitAfter(doc[start:], "\n") {
		if !strings.HasPrefix(line, "|") {
			break
		}
		end += len(line)
	}
	return doc[start:end], nil
}

func rgaS5ContractTableMatches(doc, header, digest string) error {
	table, err := rgaS5ContractTable(doc, header)
	if err != nil {
		return err
	}
	if rgaS3DocHash(table) != digest {
		return fmt.Errorf("accepted S5 contract table changed: %s", header)
	}
	return nil
}

func TestRGAS5ContractOrderedTablesAndSensitivities(t *testing.T) {
	adr := rgaS0ReadRepoFile(t, rgaS3DocADR)
	prd := rgaS0ReadRepoFile(t, rgaS3DocPRD)
	for _, tc := range []struct {
		name, doc, header, digest, old, replacement string
	}{
		{"supersession", adr, "| Feature status | Third-state (`drift`) result | Effective replay |",
			"67f79ac86a5e9f76285ac04e41ce5f8604cfaafc74498517210e0ace1aced1d4",
			"warning-class", "refusal-class"},
		{"two-narrowed-refusals", adr, "| ADR-029 D3 refusal case | ADR-036 effect |",
			"ff9dd4db5b651b8b3fdad9d1a0112b412a9e0a481d2c1453a85ed0057b86ed8f",
			"**unchanged**: still a refusal", "**narrowed**: always succeeds"},
		{"adr-verify-ladder", adr, "| Rung | Coverage state | Severity | Check row | Verdict/exit implication |",
			"2c486ac92700147d281a7b6dcb054dc677845907ed71f0532e19442062838be8",
			"| `warn` |", "| `block` |"},
		{"prd-verify-ladder", prd, "| Rung | Coverage state | Severity | Row outcome | Verdict and exit |",
			"eb41a5c33fd4b7f99bf85a901085c5f596ee46a3b2148c087234202603b64e4a",
			"| `warn` |", "| `block` |"},
		{"adr-execute-classifier", adr, "| Order | State on disk | `apply --mode execute` behavior |",
			"5ba8fe39769432b3ca0569b10b608cf2a9b2e52ed03f33d76b53594abef9add7",
			"**executes.**", "**refuses.**"},
		{"prd-execute-classifier", prd, "| Order | State on disk | Behavior |",
			"90e0513166eea517ee48fb850d2ee842597417bfe9ebf1b157bc98c34e48c275",
			"**executes.**", "**refuses.**"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			validate := func(doc string) error {
				return rgaS5ContractTableMatches(doc, tc.header, tc.digest)
			}
			if err := validate(tc.doc); err != nil {
				t.Fatal(err)
			}
			table, err := rgaS5ContractTable(tc.doc, tc.header)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(table, tc.old) {
				t.Fatalf("mutation anchor missing: %q", tc.old)
			}
			wrong := strings.Replace(table, tc.old, tc.replacement, 1)
			for name, doc := range map[string]string{
				"wrong-behavior": strings.Replace(tc.doc, table, wrong, 1),
				"missing-table":  strings.Replace(tc.doc, table, "", 1),
				"duplicate":      tc.doc + "\n" + table,
			} {
				t.Run(name, func(t *testing.T) {
					if err := validate(doc); err == nil {
						t.Fatal("same contract validator accepted the mutation")
					}
				})
			}
		})
	}
}

func rgaS5ContractGuidance(doc string) error {
	var guidance strings.Builder
	count := 0
	for _, after := range strings.Split(doc, "```text\n")[1:] {
		block, _, ok := strings.Cut(after, "```\n")
		if !ok {
			return fmt.Errorf("unterminated text fence")
		}
		if strings.HasPrefix(block, "apply --mode execute refuses:") ||
			strings.HasPrefix(block, "  This feature is already applied:") ||
			strings.HasPrefix(block, "  The canonical patch is the authority") {
			guidance.WriteString(block)
			count++
		}
	}
	if count != 4 || rgaS3DocHash(guidance.String()) != "6ef79cb660722aa8e2fb7c158ac1b5d65cc3bd6492781838ebce88b85a276383" {
		return fmt.Errorf("accepted state-aware execute guidance changed")
	}
	return nil
}

func TestRGAS5ContractGuidanceAndSensitivities(t *testing.T) {
	for _, path := range []string{rgaS3DocADR, rgaS3DocPRD} {
		t.Run(path, func(t *testing.T) {
			doc := rgaS0ReadRepoFile(t, path)
			if err := rgaS5ContractGuidance(doc); err != nil {
				t.Fatal(err)
			}
			for _, mutation := range []struct{ old, replacement string }{
				{"recipe status: <absent | unreadable: <the actual read error>>", "recipe status: absent"},
				{"git apply --check .tpatch/features/<slug>/artifacts/post-apply.patch", "git apply .tpatch/features/<slug>/artifacts/post-apply.patch"},
				{"tpatch status <slug>", "tpatch feature unapply <slug>"},
				{"checkpoint moves this feature from applied to implementing.", "checkpoint leaves this feature applied."},
				{"readable executable recipe — no readable apply-recipe.json is present.", "readable executable recipe — generation withheld the recipe."},
			} {
				if !strings.Contains(doc, mutation.old) {
					t.Fatalf("guidance mutation anchor missing: %q", mutation.old)
				}
				wrong := strings.Replace(doc, mutation.old, mutation.replacement, 1)
				if err := rgaS5ContractGuidance(wrong); err == nil {
					t.Fatalf("same guidance validator accepted %q", mutation.old)
				}
			}
		})
	}
}

func rgaS5ContractHistoricalSection(doc, heading, digest string) error {
	section, err := rgaS3DocSection(doc, heading)
	if err != nil {
		return err
	}
	if rgaS3DocHash(section) != digest {
		return fmt.Errorf("historical ADR-029 decision changed: %s", heading)
	}
	return nil
}

func TestRGAS5ContractADR029PreservedAndSensitive(t *testing.T) {
	doc := rgaS0ReadRepoFile(t, "docs/adrs/ADR-029-write-file-recipe-safety.md")
	for _, tc := range []struct{ heading, digest, old, replacement string }{
		{"### D3 — Apply prechecks are all-or-nothing",
			"60ea209d0f76bcb598318b6b0dd99c995c930a9c475692a409e4446fcd841718",
			"no operation from that recipe is written", "earlier operations may be written"},
		{"### D7 — Supersession controls severity for historical features",
			"30a448d4bad158c8bad4beaae94a92c757308890087b6194e447395b546ae3f3",
			"warning-class audit signals", "proof that replay is safe"},
	} {
		t.Run(tc.heading, func(t *testing.T) {
			if err := rgaS5ContractHistoricalSection(doc, tc.heading, tc.digest); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(doc, tc.old) {
				t.Fatalf("historical mutation anchor missing: %q", tc.old)
			}
			wrong := strings.Replace(doc, tc.old, tc.replacement, 1)
			if err := rgaS5ContractHistoricalSection(wrong, tc.heading, tc.digest); err == nil {
				t.Fatal("same historical-section validator accepted the mutation")
			}
		})
	}
}
