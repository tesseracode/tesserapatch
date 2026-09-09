package workflow

import (
	"fmt"
	"strings"
	"testing"
)

func rgaS4ContractPointer(doc string) (string, error) {
	const marker = "**S4 P2 publication-reason addendum**:\n"
	start := strings.Index(doc, marker)
	if start < 0 || strings.Count(doc, marker) != 1 {
		return "", fmt.Errorf("P2 reason addendum pointer missing or ambiguous")
	}
	end := strings.Index(doc[start:], "\n\n")
	if end < 0 {
		return "", fmt.Errorf("P2 reason addendum pointer is unterminated")
	}
	return doc[start : start+end+1], nil
}

func rgaS4ContractSemanticReasons(adr, prd, addendum string) error {
	for _, input := range []struct{ text, want string }{
		{adr, "250ac9d16eaaf0d7acf3ce824949e75d01a7348c1d580b33a058f42cd34acabb"},
		{prd, "b2be7ab40200a0d2264c32c247652469091c1e076922443eca71d2f96da0eba0"},
	} {
		pointer, err := rgaS4ContractPointer(input.text)
		if err != nil {
			return err
		}
		if rgaS3DocHash(pointer) != input.want {
			return fmt.Errorf("primary P2 reason qualification changed")
		}
	}
	decision, err := rgaS3DocSection(addendum, "## Decision")
	if err != nil {
		return err
	}
	if rgaS3DocHash(decision) != "538a297e9debbd0095e1864a097291c99dec0378c1326919e24c5e30bf7d90f1" {
		return fmt.Errorf("operator-selected semantic-reasons decision changed")
	}
	const startMarker = "**`producer-patch-rewrite` is conditional, not automatic.**"
	start := strings.Index(adr, startMarker)
	if start < 0 || strings.Count(adr, startMarker) != 1 {
		return fmt.Errorf("D3 semantic reason condition missing or ambiguous")
	}
	end := strings.Index(adr[start:], "\nBecause the axes")
	if end < 0 || rgaS3DocHash(adr[start:start+end+1]) != "fd8af4f1a74c6ba9cbadf35da52684b32a0902e0b576573d2237b7e865265619" {
		return fmt.Errorf("D3 semantic reason condition was changed")
	}
	return nil
}

func TestRGAS4ContractSemanticReasonsAndSensitivities(t *testing.T) {
	adr, prd := rgaS0ReadRepoFile(t, rgaS3DocADR), rgaS0ReadRepoFile(t, rgaS3DocPRD)
	addendum := rgaS0ReadRepoFile(t, "docs/adrs/ADR-040-p2-publication-reason-semantics.md")
	if err := rgaS4ContractSemanticReasons(adr, prd, addendum); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct{ old, replacement string }{
		{"neither is added merely", "both are always added"},
		{"Do not redefine semantic explanation", "Redefine semantic explanation"},
		{"No wire field, enum, reason code", "A new wire field, enum, reason code"},
	} {
		if !strings.Contains(addendum, mutation.old) {
			t.Fatalf("reason mutation anchor missing: %q", mutation.old)
		}
		wrong := strings.Replace(addendum, mutation.old, mutation.replacement, 1)
		if err := rgaS4ContractSemanticReasons(adr, prd, wrong); err == nil {
			t.Fatalf("same reason validator accepted %q", mutation.old)
		}
	}
	wrongADR := strings.Replace(adr, "raises **neither** code", "raises both codes", 1)
	if wrongADR == adr || rgaS4ContractSemanticReasons(wrongADR, prd, addendum) == nil {
		t.Fatal("same validator accepted a changed D3 raising condition")
	}
	pointer, err := rgaS4ContractPointer(prd)
	if err != nil {
		t.Fatal(err)
	}
	if err := rgaS4ContractSemanticReasons(adr, strings.Replace(prd, pointer, "", 1), addendum); err == nil {
		t.Fatal("same validator accepted removal of the PRD qualification")
	}
}
