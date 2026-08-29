//go:build (linux && !android) || (darwin && !ios)

package intentlock

// Direct per-row §18.53 sensitivity fixtures for PIB-418 and PIB-432.
//
// The shipped guards `TestS7PIB418AuthoritySourceAndDocsRejectLegacyLockPrimitives`
// and `TestS7PIB432PrepareOwnedAuthorityAndFrozenRescapParity` are already bound
// as PIB-393's and PIB-394's wrong-input fixtures, so PIB-418 and PIB-432 need
// their own identities. Each body below drives the *same* shipped validator over
// the shipped input and then over a different deliberately wrong version of it.

import (
	"strings"
	"testing"
)

// TestS7PIB418AuthorityDocsSensitivityDirect mutates the authority
// documentation rather than the authority source: dropping the no-extraction
// statement must fail the same validator that accepts the shipped documents.
func TestS7PIB418AuthorityDocsSensitivityDirect(t *testing.T) {
	sources := s7AuthorityProductionSources(t)
	prd := s7ReadRepositoryFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
	adr := s7ReadRepositoryFile(t, "docs/adrs/ADR-035-intent-bundle-publication-and-history.md")
	if err := validateS7PIB418AuthorityInputs(sources, prd, adr); err != nil {
		t.Fatalf("PIB-418: the shipped authority inputs failed their own guard: %v", err)
	}
	mutatedPRD := strings.ReplaceAll(
		prd, "not extracted or reused as prepare's authority", "reused as prepare's authority",
	)
	mutatedADR := strings.ReplaceAll(
		adr, "not extracted or reused as prepare's authority", "reused as prepare's authority",
	)
	if mutatedPRD == prd && mutatedADR == adr {
		t.Fatal("PIB-418: the documentation mutation anchor is missing")
	}
	if err := validateS7PIB418AuthorityInputs(sources, mutatedPRD, mutatedADR); err == nil {
		t.Fatal("PIB-418: the same validator accepted documents that drop the no-extraction statement")
	}
	named := make(map[string]string, len(sources)+1)
	for name, source := range sources {
		named[name] = source
	}
	named["pib418-cache-authority.go"] = `package intentlock
import "os"
func wrongCacheAuthorityForDocsRow() { _, _ = os.UserCacheDir() }
`
	if err := validateS7PIB418AuthorityInputs(named, prd, adr); err == nil {
		t.Fatal("PIB-418: the same validator accepted a user-cache authority read")
	}
}

// TestS7PIB432RescapParitySensitivityDirect mutates the *authority* half of the
// parity claim rather than the rescap half: an authority that advertises a
// file-lock extraction must fail the same validator.
func TestS7PIB432RescapParitySensitivityDirect(t *testing.T) {
	authority := s7AuthorityProductionSources(t)
	rescap := s7RescapProductionSources(t)
	prd := s7ReadRepositoryFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
	if err := validateS7PIB432Parity(authority, rescap, prd); err != nil {
		t.Fatalf("PIB-432: the shipped parity inputs failed their own guard: %v", err)
	}
	mutatedPRD := strings.ReplaceAll(
		prd, "**not** extraction of `rescap`'s file lock", "extraction of `rescap`'s file lock",
	)
	if mutatedPRD == prd {
		t.Fatal("PIB-432: the slice-claim mutation anchor is missing")
	}
	if err := validateS7PIB432Parity(authority, rescap, mutatedPRD); err == nil {
		t.Fatal("PIB-432: the same validator accepted a documented file-lock extraction")
	}
	mutatedAuthority := make(map[string]string, len(authority)+1)
	for name, source := range authority {
		mutatedAuthority[name] = source
	}
	mutatedAuthority["pib432-cache-authority.go"] = `package intentlock
import "os"
func wrongCacheAuthorityForParityRow() { _, _ = os.UserCacheDir() }
`
	if err := validateS7PIB432Parity(mutatedAuthority, rescap, prd); err == nil {
		t.Fatal("PIB-432: the same validator accepted a cache-directory authority")
	}
}
