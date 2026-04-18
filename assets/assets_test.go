package assets

import (
	"strings"
	"testing"
)

// Required CLI commands that must appear in all skill formats.
var requiredCommands = []string{
	"tpatch init",
	"tpatch add",
	"tpatch status",
	"tpatch analyze",
	"tpatch define",
	"tpatch explore",
	"tpatch implement",
	"tpatch apply",
	"tpatch record",
	"tpatch reconcile",
	"tpatch provider",
	"tpatch config",
	"tpatch cycle",
	"tpatch test",
	"tpatch next",
}

// Required anchor strings that must appear VERBATIM in every skill
// format. These are the Invocation / Phase Ordering / Preflight
// contract from bug-skill-invocation-clarity — agents were inventing
// `npx tpatch` and speculative cwds because the skills never said
// otherwise. Changing the wording here is a breaking change to the
// skill-CLI contract; expect to update all 6 asset files together.
var requiredAnchors = []struct {
	label  string
	anchor string
}{
	{"invocation/go-binary", "compiled Go binary on PATH"},
	{"invocation/no-npx", "✗ `npx tpatch"},
	{"invocation/no-cd", "Do not `cd` to speculative paths"},
	{"phase-ordering/table", "requested    → tpatch analyze    → analyzed"},
	{"phase-ordering/never-skip", "Never skip a phase"},
	{"preflight/status", "`tpatch status <slug>`"},
	{"preflight/next", "`tpatch next <slug>`"},
	{"preflight/no-guess", "Do not guess the next phase"},
	{"preflight/record-timing", "tpatch record <slug> BEFORE git commit"},
	{"preflight/reconcile-clean-tree", "tpatch reconcile only on a CLEAN working tree"},
}

// Skill format files that must mention all CLI commands.
var skillFiles = []struct {
	name string
	path string
}{
	{"Claude", "skills/claude/tessera-patch/SKILL.md"},
	{"Copilot", "skills/copilot/tessera-patch/SKILL.md"},
	{"Copilot Prompt", "prompts/copilot/tessera-patch-apply.prompt.md"},
	{"Cursor", "skills/cursor/tessera-patch.mdc"},
	{"Windsurf", "skills/windsurf/windsurfrules"},
	{"Generic", "workflows/tessera-patch-generic.md"},
}

func TestSkillParityGuard(t *testing.T) {
	for _, sf := range skillFiles {
		t.Run(sf.name, func(t *testing.T) {
			data, err := Skills.ReadFile(sf.path)
			if err != nil {
				t.Fatalf("cannot read %s: %v", sf.path, err)
			}
			content := string(data)

			for _, cmd := range requiredCommands {
				if !strings.Contains(content, cmd) {
					t.Errorf("%s (%s) missing CLI command: %q", sf.name, sf.path, cmd)
				}
			}
			for _, a := range requiredAnchors {
				if !strings.Contains(content, a.anchor) {
					t.Errorf("%s (%s) missing required anchor [%s]: %q",
						sf.name, sf.path, a.label, a.anchor)
				}
			}
		})
	}
}

func TestAllSkillFilesExist(t *testing.T) {
	for _, sf := range skillFiles {
		t.Run(sf.name, func(t *testing.T) {
			_, err := Skills.ReadFile(sf.path)
			if err != nil {
				t.Fatalf("skill file %s (%s) not found: %v", sf.name, sf.path, err)
			}
		})
	}
}
