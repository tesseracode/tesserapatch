package assets_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var operationCandidateRegionIDs = []string{"A", "B", "C1", "C2", "D1", "D2", "D3", "E", "F"}

var operationCandidateRegionMarker = regexp.MustCompile(`<!-- parity-block:(A|B|C1|C2|D1|D2|D3|E|F):(begin|end) -->`)

func operationCandidateRegions(body []byte) (map[string][]byte, error) {
	regions := make(map[string][]byte)
	active, start := "", 0
	for _, match := range operationCandidateRegionMarker.FindAllSubmatchIndex(body, -1) {
		id := string(body[match[2]:match[3]])
		kind := string(body[match[4]:match[5]])
		if kind == "begin" {
			if _, duplicate := regions[id]; duplicate || active != "" {
				return nil, fmt.Errorf("duplicate or overlapping begin marker %s", id)
			}
			active, start = id, match[1]
			continue
		}
		if active != id {
			return nil, fmt.Errorf("unpaired or reversed end marker %s", id)
		}
		regions[id] = body[start:match[0]]
		active = ""
	}
	if active != "" {
		return nil, fmt.Errorf("unpaired begin marker %s", active)
	}
	for _, id := range operationCandidateRegionIDs {
		if _, ok := regions[id]; !ok {
			return nil, fmt.Errorf("missing parity region %s", id)
		}
	}
	return regions, nil
}

func validateOperationCandidateRegionParity(prd, adr []byte) error {
	left, err := operationCandidateRegions(prd)
	if err != nil {
		return fmt.Errorf("PRD: %w", err)
	}
	right, err := operationCandidateRegions(adr)
	if err != nil {
		return fmt.Errorf("ADR-037: %w", err)
	}
	for _, id := range operationCandidateRegionIDs {
		if !bytes.Equal(left[id], right[id]) {
			return fmt.Errorf("parity region %s differs byte-for-byte", id)
		}
	}
	return nil
}

func rocS9ReadDocument(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func rocS9ParityDocuments(t *testing.T) ([]byte, []byte) {
	t.Helper()
	return rocS9ReadDocument(t, "docs/prds/PRD-reconcile-operation-replay-candidate.md"),
		rocS9ReadDocument(t, "docs/adrs/ADR-037-reconcile-operation-replay-candidate-authority.md")
}

func TestROCS9ContractRegionParity(t *testing.T) {
	prd, adr := rocS9ParityDocuments(t)
	if err := validateOperationCandidateRegionParity(prd, adr); err != nil {
		t.Fatal(err)
	}
	// Literal illustrations (including an unmatched placeholder) and ordinary
	// prose must not be mistaken for a tenth region or compared with the ADR.
	outside := []byte("Different outside prose.\r\n<!-- parity-block:<ID>:begin -->\n<!-- parity-block:OTHER:end -->\n")
	if err := validateOperationCandidateRegionParity(append(outside, prd...), adr); err != nil {
		t.Fatalf("outside-region prose or illustration affected parity: %v", err)
	}
}

func TestROCS9ContractRegionParitySensitivity(t *testing.T) {
	prd, adr := rocS9ParityDocuments(t)
	if err := validateOperationCandidateRegionParity(prd, adr); err != nil {
		t.Fatal(err)
	}
	for _, id := range operationCandidateRegionIDs {
		t.Run(id, func(t *testing.T) {
			begin := []byte("<!-- parity-block:" + id + ":begin -->")
			end := []byte("<!-- parity-block:" + id + ":end -->")
			start := bytes.Index(prd, begin)
			stop := bytes.Index(prd, end) + len(end)
			region := prd[start:stop]
			without := bytes.Replace(prd, region, nil, 1)
			for _, row := range []struct {
				name string
				left []byte
			}{
				{"missing-pair", without},
				{"missing-begin", bytes.Replace(prd, begin, nil, 1)},
				{"missing-end", bytes.Replace(prd, end, nil, 1)},
				{"duplicate-begin", bytes.Replace(prd, begin, bytes.Repeat(begin, 2), 1)},
				{"duplicate-end", bytes.Replace(prd, end, bytes.Repeat(end, 2), 1)},
				{"duplicate-region", append(bytes.Clone(prd), region...)},
				{"reversed", bytes.Replace(prd, region, bytes.Join([][]byte{end, region[len(begin) : len(region)-len(end)], begin}, nil), 1)},
				{"one-sided-id", bytes.Replace(prd, begin, []byte("<!-- parity-block:OTHER:begin -->"), 1)},
				{"inner-space", bytes.Replace(prd, begin, append(bytes.Clone(begin), ' '), 1)},
				{"inner-CRLF", bytes.Replace(prd, begin, append(bytes.Clone(begin), '\r'), 1)},
			} {
				t.Run(row.name, func(t *testing.T) {
					if bytes.Equal(row.left, prd) {
						t.Fatal("negative control did not change the document")
					}
					if err := validateOperationCandidateRegionParity(row.left, adr); err == nil {
						t.Fatal("real parity validator accepted the wrong input")
					}
					if err := validateOperationCandidateRegionParity(adr, row.left); err == nil {
						t.Fatal("parity validator checked only one document's marker structure")
					}
				})
			}
			adrStart := bytes.Index(adr, begin)
			adrStop := bytes.Index(adr, end) + len(end)
			adrWithout := bytes.Replace(adr, adr[adrStart:adrStop], nil, 1)
			if err := validateOperationCandidateRegionParity(without, adrWithout); err == nil {
				t.Fatal("matching but incomplete region inventories were accepted")
			}
		})
	}
}

var operationCandidatePhaseRows = []string{
	"→ All already-present → UPSTREAMED only with CG1-CG7; otherwise confirmation gate → BLOCKED",
	"→ Any conflict → nonterminal phase-2 BLOCKED classification; continue phases 3 and 4",
	"→ Mix (present/applicable, at least one applicable) → nonterminal; candidate only after a phase-4 conflict",
}

func validateOperationCandidatePhaseRows(body string) error {
	prefixes := []string{"→ All already-present", "→ Any conflict", "→ Mix"}
	found := make([]bool, len(prefixes))
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		for i, prefix := range prefixes {
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			if found[i] || line != operationCandidatePhaseRows[i] {
				return fmt.Errorf("phase-2 row %q is duplicated or changes its outcome/terminality: %q", prefix, line)
			}
			found[i] = true
		}
	}
	for i, present := range found {
		if !present {
			return fmt.Errorf("missing phase-2 row %q", prefixes[i])
		}
	}
	return nil
}

func TestROCS9SPECPhaseRowsAndSensitivity(t *testing.T) {
	spec := string(rocS9ReadDocument(t, "SPEC.md"))
	if err := validateOperationCandidatePhaseRows(spec); err != nil {
		t.Fatal(err)
	}
	legacy := []string{
		"→ All already-present → UPSTREAMED",
		"→ Any conflict → BLOCKED",
		"→ Mix → STILL-NEEDED",
	}
	for i, wrong := range legacy {
		t.Run(wrong, func(t *testing.T) {
			mutant := strings.Replace(spec, operationCandidatePhaseRows[i], wrong, 1)
			if err := validateOperationCandidatePhaseRows(mutant); err == nil {
				t.Fatal("real SPEC validator accepted an unqualified or prematurely terminal row")
			}
		})
	}
	conflictOnly := strings.Replace(spec, operationCandidatePhaseRows[0], legacy[0], 1)
	conflictOnly = strings.Replace(conflictOnly, operationCandidatePhaseRows[2], legacy[2], 1)
	if err := validateOperationCandidatePhaseRows(conflictOnly); err == nil {
		t.Fatal("updating only the conflict row incorrectly satisfied the SPEC amendment")
	}
}

func validateOperationCandidateSkill(body string) error {
	const heading = "## Operation replay candidates (v0.18.0)\n"
	if strings.Count(body, heading) != 1 {
		return fmt.Errorf("want one operation-candidate section")
	}
	section := strings.SplitN(body, heading, 2)[1]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	if err := validateOperationCandidatePhaseRows(section); err != nil {
		return err
	}
	actions := map[string]string{
		"Review diff":        "--candidate-diff",
		"Explicitly accept":  "--accept-candidate",
		"Reject and prune":   "--reject-candidate",
		"Clear rejected IDs": "--clear-candidate-rejections",
		"Explicit recovery":  "--resume-candidate-recovery",
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) != 2 {
			return fmt.Errorf("invalid candidate action row: %s", line)
		}
		action, command := strings.TrimSpace(cells[0]), strings.TrimSpace(cells[1])
		if (action == "Action" && command == "Exact command") || (action == "---" && command == "---") {
			continue
		}
		flag, known := actions[action]
		if !known || seen[action] || command != "`tpatch reconcile "+flag+" <slug>`" {
			return fmt.Errorf("candidate action %q does not use its exact supported terminal: %q", action, command)
		}
		seen[action] = true
	}
	if len(seen) != len(actions) {
		return fmt.Errorf("candidate action table is incomplete")
	}
	plain := strings.Join(strings.Fields(section), " ")
	for _, sentence := range []string{
		"`operation_candidate_enabled` is default off.",
		"`operation_candidate_enabled: true` in `.tpatch/config.yaml`; omission or `false` disables derivation.",
		"This key is not supported by `tpatch config set`.",
		"An unsupported effect withholds the whole candidate; upstream deletions stay deleted.",
		"Stored labels, confidence and hashes are not replay or confirmation authority.",
		"Provider fallback remains available; `--resolve` suppresses candidate publication.",
		"Review the diff and intent before explicit acceptance; there is no candidate auto-accept.",
		"Each flag takes the slug as its value, with no positional slug.",
		"Only `--path` and `--format human|json` may accompany it; `--help` remains help.",
		"Resolver `--accept`, `--reject`, `--shadow-diff` are not candidate aliases;",
		"It requires upstream ancestry, not HEAD equality,",
		"No live index, refs or reflogs are rewritten; no commit, rebase or history rewrite.",
		"no commit marker restores snapshots/absences and prior Apply/State to pending; a committed marker permits cleanup only.",
	} {
		if !strings.Contains(plain, sentence) {
			return fmt.Errorf("candidate guidance changes the contract: missing %q", sentence)
		}
	}
	if strings.Contains(body, "No GH #13 replay consumer") {
		return fmt.Errorf("obsolete no-consumer claim contradicts the shipped candidate section")
	}
	return nil
}

func TestROCS9SkillContractSensitivity(t *testing.T) {
	for _, sf := range skillFiles {
		t.Run(sf.name, func(t *testing.T) {
			body := rgaEmbeddedSurface(t, sf.path)
			if err := validateOperationCandidateSkill(body); err != nil {
				t.Fatal(err)
			}
			for _, row := range []struct{ name, from, to string }{
				{"default-on", "is default off.", "is default on."},
				{"invented-setter", "This key is not supported by `tpatch config set`.", "Run `tpatch config set operation_candidate_enabled true`."},
				{"phase2-returns-candidate", operationCandidatePhaseRows[2], "→ Mix → STILL-NEEDED immediately; skip phase 4"},
				{"stored-hash-authority", "Stored labels, confidence and hashes are not replay or confirmation authority.", "Stored labels, confidence and hashes are replay and confirmation authority."},
				{"automatic-acceptance", "there is no candidate\nauto-accept.", "candidate\nauto-accept is enabled."},
				{"resolver-alias", "| Explicitly accept | `tpatch reconcile --accept-candidate <slug>` |", "| Explicitly accept | `tpatch reconcile --accept <slug>` |"},
				{"positional-slug", "| Review diff | `tpatch reconcile --candidate-diff <slug>` |", "| Review diff | `tpatch reconcile <slug> --candidate-diff` |"},
				{"wrong-recovery-direction", "a committed marker permits cleanup only.", "a committed marker restores the pre-accept tree."},
				{"ref-rewrite", "No live index, refs or reflogs are rewritten;", "Live index, refs and reflogs are rewritten;"},
			} {
				t.Run(row.name, func(t *testing.T) {
					mutant := strings.Replace(body, row.from, row.to, 1)
					if mutant == body {
						t.Fatal("negative fixture did not change installed guidance")
					}
					if err := validateOperationCandidateSkill(mutant); err == nil {
						t.Fatal("same installed-skill validator accepted a wrong contract")
					}
				})
			}
		})
	}
}
