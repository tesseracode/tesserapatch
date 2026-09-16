//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Preserve AP's historical oracle, projecting only the reviewed S6 prose
// bodies. Expected hashes are pinned independently, never read from live docs.
func rgaS6ExpectedDanglingSurfaces(previous map[string]string) (map[string]string, error) {
	delta := []struct {
		path, before, after string
	}{
		{
			"assets/prompts/copilot/tessera-patch-apply.prompt.md",
			"a0d00e4490b16e1bd62751651f36ae1e1d47145f5b35a08a4549dff0587de296",
			"1fabc947282019dd1676997a62318ac1466d53239ced82851eab8c7658e116a8",
		},
		{
			"assets/skills/claude/tessera-patch/SKILL.md",
			"1348460eb0243d318577249ae380c2db8da9b94283e3093a8f3d7e06bc36eb4a",
			"35299f5b3caa450df48b819c812d994a7d1d651bb048a2e8f9ed6331c2a2d6d4",
		},
		{
			"assets/skills/copilot/tessera-patch/SKILL.md",
			"ad0ef9ddd93ca3b6b17623eb36f0d3297434bdef3e1e635c4873067d3e7d13c5",
			"c51e23147a46ac749bf75149b84c33f6b7921fe14dee171f9d0c75ca83a559eb",
		},
		{
			"assets/skills/cursor/tessera-patch.mdc",
			"88cb89a4aec4f3400eb654ffb545b5f377446ae69b3dc6d1bb9f66a1a05c8eea",
			"963ab3be368e34a4fc05f1fcc11e7f7f7c7ffe38bb38725d328e63191400bbd8",
		},
		{
			"assets/skills/windsurf/windsurfrules",
			"60df5c3a9758c4d58e899621d34fdcb70eec97be4fbcd3424f1b267543f8eaae",
			"c82f2abc4db356ba849f7f56765f8d3146b9e26e84d97175593521c5153c697e",
		},
		{
			"assets/workflows/tessera-patch-generic.md",
			"7325c4507b67058fbe9092b4f6c49bc5d5a911712a645912f8b561c09788d735",
			"70becebf81c99fbe59e4bc175c5f13e9028639e6040bf39d137dd8f2623dfed1",
		},
		{
			"docs/feature-layout.md",
			"7065454f25457249e63d409494c7f9da25078e2d2e4c4d4c6570cdebca3b707e",
			"01d2bdd5b4fbf863cb2759c999a7255b95d9190b5250f94a42e11bc891a3bd8d",
		},
		{
			"docs/adrs/ADR-035-intent-bundle-publication-and-history.md",
			"edbf901f42e7b7df24ecac8c2b6befae0a5a16136322ccee19634ce4f1af17c8",
			"edbf901f42e7b7df24ecac8c2b6befae0a5a16136322ccee19634ce4f1af17c8",
		},
		{
			"docs/prds/PRD-prepare-intent-bundle.md",
			"3dd28b88f34f5f5f2e523a8738681d473a6255c8bf03d9b80d3a165ab49e3cc9",
			"3dd28b88f34f5f5f2e523a8738681d473a6255c8bf03d9b80d3a165ab49e3cc9",
		},
	}
	if len(previous) != len(delta) {
		return nil, fmt.Errorf("S6 dangling-doc projection requires the unchanged AP inventory")
	}
	expected := make(map[string]string, len(delta))
	for _, item := range delta {
		if previous[item.path] != item.before {
			return nil, fmt.Errorf("S6 dangling-doc projection refuses historical drift at %s", item.path)
		}
		expected[item.path] = item.after
	}
	return expected, nil
}

func TestRGAS6DanglingDocDeltaAndSensitivity(t *testing.T) {
	before := s7APCloneStringMap(s7APAcceptedDanglingSurfaces)
	after, err := rgaS6ExpectedDanglingSurfaces(s7APAcceptedDanglingSurfaces)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, s7APAcceptedDanglingSurfaces) {
		t.Fatal("S6 projection mutated the historical AP oracle")
	}
	changed := 0
	for path, digest := range before {
		if after[path] != digest {
			changed++
		}
	}
	if changed != 7 {
		t.Fatalf("S6 projection changed %d bodies, want exactly six skills plus feature layout", changed)
	}

	unknown := s7APCloneStringMap(before)
	unknown["docs/unowned.md"] = strings.Repeat("a", 64)
	missing := s7APCloneStringMap(before)
	delete(missing, "docs/feature-layout.md")
	drifted := s7APCloneStringMap(before)
	drifted["docs/prds/PRD-prepare-intent-bundle.md"] = strings.Repeat("a", 64)
	for name, wrong := range map[string]map[string]string{
		"unknown-path": unknown, "missing-path": missing, "historical-drift": drifted,
		"already-projected": after,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := rgaS6ExpectedDanglingSurfaces(wrong); err == nil {
				t.Fatal("same S6 projection accepted an invalid historical oracle")
			}
		})
	}

	surfaces := s7APDanglingOwnedSurfaces(t)
	if err := validateS7APDanglingOwnedSurfaces(surfaces); err != nil {
		t.Fatalf("S6 current bodies failed the existing AP validator: %v", err)
	}
	for path := range before {
		t.Run("unapproved-body/"+path, func(t *testing.T) {
			wrong := s7APCloneStringMap(surfaces)
			wrong[path] += "\nUnapproved S6 prose body change.\n"
			if err := validateS7APDanglingOwnedSurfaces(wrong); err == nil ||
				!strings.Contains(err.Error(), "prose surface content drift") {
				t.Fatalf("same AP validator lost body sensitivity: %v", err)
			}
		})
	}
}
