package assets_test

import (
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/assets"
)

// TestAVPAssetParity covers the matrix's asset-parity rows — AVP-090,
// AVP-091 and AVP-092 — by asserting the named observables directly rather
// than relying on the general parity guard passing.
func TestAVPAssetParity(t *testing.T) {
	t.Run("AVP-090", func(t *testing.T) {
		found := false
		for _, command := range requiredCommands {
			if command == "tpatch prepare" {
				found = true
			}
		}
		if !found {
			t.Fatalf("requiredCommands does not contain %q: %v", "tpatch prepare", requiredCommands)
		}
	})

	t.Run("AVP-091", func(t *testing.T) {
		const anchor = "`tpatch prepare <slug> --check` is read-only"
		for _, sf := range skillFiles {
			data, err := assets.Skills.ReadFile(sf.path)
			if err != nil {
				t.Fatalf("read %s: %v", sf.path, err)
			}
			if !strings.Contains(string(data), anchor) {
				t.Fatalf("%s does not carry the read-only anchor verbatim", sf.path)
			}
		}
		if len(skillFiles) != 6 {
			t.Fatalf("%d skill surfaces, want the six shipped formats", len(skillFiles))
		}
	})

	t.Run("AVP-092", func(t *testing.T) {
		// The mandated phase sequence must be byte-unchanged: prepare --check
		// is optional and was NOT added to it (§16.2 item 5).
		unchanged := []string{
			"requested    → tpatch analyze    → analyzed",
			"Never skip a phase",
			"`tpatch status <slug>`",
			"`tpatch next <slug>`",
			"Do not guess the next phase",
			"tpatch record <slug> BEFORE git commit",
			"tpatch reconcile only on a CLEAN working tree",
		}
		for _, sf := range skillFiles {
			data, err := assets.Skills.ReadFile(sf.path)
			if err != nil {
				t.Fatalf("read %s: %v", sf.path, err)
			}
			text := string(data)
			for _, anchor := range unchanged {
				if !strings.Contains(text, anchor) {
					t.Fatalf("%s lost the byte-unchanged anchor %q", sf.path, anchor)
				}
			}
			for _, line := range strings.Split(text, "\n") {
				if strings.Contains(line, "requested    → tpatch analyze") && strings.Contains(line, "prepare") {
					t.Fatalf("%s added prepare --check to the mandated phase table: %q", sf.path, line)
				}
			}
		}
	})
}
