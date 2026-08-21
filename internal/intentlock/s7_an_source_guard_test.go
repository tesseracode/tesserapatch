package intentlock

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestS7PIB418AuthoritySourceAndDocsRejectLegacyLockPrimitives(t *testing.T) {
	sources := s7AuthorityProductionSources(t)
	prd := s7ReadRepositoryFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
	adr := s7ReadRepositoryFile(t, "docs/adrs/ADR-035-intent-bundle-publication-and-history.md")
	if err := validateS7PIB418AuthorityInputs(sources, prd, adr); err != nil {
		t.Fatal(err)
	}
	wrong := make(map[string]string, len(sources)+1)
	for name, source := range sources {
		wrong[name] = source
	}
	wrong["pib418-wrong.go"] = `package intentlock
import "os"
func wrong() { _, _ = os.UserCacheDir(); _ = os.Getenv("XDG_CACHE_HOME") }
`
	if err := validateS7PIB418AuthorityInputs(wrong, prd, adr); err == nil ||
		!strings.Contains(err.Error(), "authority source guard") {
		t.Fatalf("PIB-418 same validator accepted cache/XDG wrong input: %v", err)
	}
}

func TestS7PIB432PrepareOwnedAuthorityAndFrozenRescapParity(t *testing.T) {
	authority := s7AuthorityProductionSources(t)
	rescap := s7RescapProductionSources(t)
	prd := s7ReadRepositoryFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
	if err := validateS7PIB432Parity(authority, rescap, prd); err != nil {
		t.Fatal(err)
	}
	wrong := make(map[string][]byte, len(rescap))
	for name, source := range rescap {
		wrong[name] = append([]byte(nil), source...)
	}
	wrong["lock_unix.go"] = append(wrong["lock_unix.go"], []byte("\n// extracted by intentlock\n")...)
	if err := validateS7PIB432Parity(authority, wrong, prd); err == nil ||
		!strings.Contains(err.Error(), "rescap parity") {
		t.Fatalf("PIB-432 same validator accepted altered rescap source: %v", err)
	}
}

func validateS7PIB418AuthorityInputs(sources map[string]string, prd, adr string) error {
	if err := validateS7PIB398AuthoritySources(sources); err != nil {
		return err
	}
	joined := strings.Join([]string{prd, adr}, "\n")
	for _, required := range []string{
		"not extracted or reused as prepare's authority",
		"file-lock precedent, not an extraction or reuse",
		"**not** extraction of `rescap`'s file lock",
	} {
		if !strings.Contains(joined, required) {
			return fmt.Errorf("PIB-418 authority docs lost no-extraction statement %q", required)
		}
	}
	return nil
}

func validateS7PIB432Parity(
	authority map[string]string,
	rescap map[string][]byte,
	prd string,
) error {
	if err := validateS7PIB398AuthoritySources(authority); err != nil {
		return err
	}
	if !strings.Contains(prd, "This is **not** extraction of `rescap`'s file lock; rescap lock behavior is byte-identical.") {
		return fmt.Errorf("PIB-432 implementation-slice ownership statement missing")
	}
	expected := map[string]string{
		"compare.go":              "323f1ac83cca591fa165addb5171dc62dc97fe246e8b39d388b290d0776dbe15",
		"content.go":              "96f213c2c720b0a26c9eb995f93079ffe71d1acba37f3a3204cd02fadade703c",
		"dolt.go":                 "f3f9a8ff010c75a70daa3ce8094abd04721160e0c0d7b8ea85dd3aed1ff7dd18",
		"engine.go":               "4692ce00ab51f838f23170c78b8ff70d7b52915298d0b9f6c3908e51a58ffb1d",
		"gitgate.go":              "8ada84e0cb85297907b763e315262f66f4b94cde8f3eb6c98c141ee6eddca999",
		"gitmeta.go":              "424ccc1a8a1be46c13efde8598538319a1880c9f713be76f6a13bee55d85a1df",
		"lock_unix.go":            "77136d6ed5d82024813ec8c06a301ddb29306846eba12fb19461e74a8c55a402",
		"lock_unsupported.go":     "994b18c8f0edce3369f10fbbac46a021d910ae35bacbaf9bf268974304b63155",
		"observer_unix.go":        "a27811663f342a80e8c655889a6cca87f155abd96943a9f28571308537ae45f6",
		"observer_unsupported.go": "5fa0e4430e30d03ac17ceccc9912806480560e1d511753ddca2bd7e2fd57fa82",
		"pathgate.go":             "b8870202f677f8c2509e702278d9683d4756a5bb7b5946c016d0968dc4de87d1",
		"pathopen_unix.go":        "1a9c5538e111434facdc0473b49834d30e1091fa51ae29729e68c532ae9c8072",
		"pathopen_windows.go":     "2da533ce5b2eb686ad6b9d28d57e8b0224948e37c97cb5a3d94221ab977905a3",
		"process.go":              "e9c4d5c3a4aa3d6bd76b9b68d275605a2a659ab90446f9847e65c23964fde91f",
		"refusal.go":              "3b71b66dc26a577c8c09597561bfa20e49bfe6b0ece5dc96295e27275d6b2985",
		"scratch.go":              "6da09799056f0c480d1b604add4d7ce0c7d9da2855f0317fb87ef1356e68e734",
		"statfs_darwin.go":        "2e5a2011268e69d90ce735abc9c6a7e5bfaf24b5294dc1e037cce064b66eee4f",
		"statfs_linux.go":         "49d9709c6a558b8d486effd990a64f273a0ce26cd79ee974e47822c497be7ade",
	}
	if len(rescap) != len(expected) {
		return fmt.Errorf("PIB-432 rescap parity file count = %d, want %d", len(rescap), len(expected))
	}
	for name, want := range expected {
		source, ok := rescap[name]
		if !ok {
			return fmt.Errorf("PIB-432 rescap parity missing %s", name)
		}
		sum := sha256.Sum256(source)
		if got := hex.EncodeToString(sum[:]); got != want {
			return fmt.Errorf("PIB-432 rescap parity %s = %s, want %s", name, got, want)
		}
	}
	return nil
}

func s7ReadRepositoryFile(t *testing.T, rel string) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func s7RescapProductionSources(t *testing.T) map[string][]byte {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(current), "..", "rescap")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string][]byte{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		sources[entry.Name()] = data
	}
	return sources
}
