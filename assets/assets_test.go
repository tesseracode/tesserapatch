package assets

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/tesserabox/tesserapatch/internal/workflow"
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
	{"provider-fallback/you-are-the-provider", "You are the provider"},
	{"recipe-schema/ops-table", "apply-recipe.json schema"},
	{"recipe-schema/literal-search", "literal string match, not a regex"},
	{"conflict-playbook/checkout-stash", "git checkout stash@{0}^3 -- .tpatch/"},
	{"conflict-playbook/never-pop", "Never pop the stash"},
	{"patch-vs-recipe/intent-vs-snapshot", "patch captures intent"},
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

// TestSkillRecipeSchemaMatchesCLI extracts each ```json ... ``` block
// from every skill file, looks for a top-level `"operations"` array,
// and unmarshals it into the authoritative workflow.RecipeOperation
// struct. Any field the skill documents that the CLI does not accept
// (e.g. `op` instead of `type`, `contents` instead of `content`,
// `occurrences` — bug-skill-recipe-schema-mismatch, v0.4.3) fails here.
// Prevents the skill docs from drifting out of sync with the code
// agents actually run.
func TestSkillRecipeSchemaMatchesCLI(t *testing.T) {
	codeBlock := regexp.MustCompile("(?s)```json\\s*\\n(.*?)\\n```")
	for _, sf := range skillFiles {
		t.Run(sf.name, func(t *testing.T) {
			data, err := Skills.ReadFile(sf.path)
			if err != nil {
				t.Fatalf("cannot read %s: %v", sf.path, err)
			}
			content := string(data)
			matches := codeBlock.FindAllStringSubmatch(content, -1)
			checked := 0
			for _, m := range matches {
				block := m[1]
				if !strings.Contains(block, "\"operations\"") {
					continue
				}
				var recipe struct {
					Version    int                        `json:"version"`
					Operations []workflow.RecipeOperation `json:"operations"`
					Extra      map[string]json.RawMessage `json:"-"`
				}
				dec := json.NewDecoder(strings.NewReader(block))
				dec.DisallowUnknownFields()
				if err := dec.Decode(&recipe); err != nil {
					t.Errorf("%s: recipe JSON block does not match workflow.RecipeOperation schema: %v\nBlock:\n%s",
						sf.path, err, block)
					continue
				}
				if len(recipe.Operations) == 0 {
					t.Errorf("%s: recipe JSON block has zero operations", sf.path)
					continue
				}
				for i, op := range recipe.Operations {
					if op.Type == "" {
						t.Errorf("%s: operation %d missing `type` field — schema drift (likely using `op` instead)", sf.path, i)
					}
					switch op.Type {
					case "write-file", "replace-in-file", "append-file", "ensure-directory":
						// ok — known op types supported by the CLI
					default:
						t.Errorf("%s: operation %d has unknown type %q — CLI supports write-file, replace-in-file, append-file, ensure-directory only",
							sf.path, i, op.Type)
					}
				}
				checked++
			}
			if checked == 0 {
				t.Errorf("%s: no recipe JSON block found — every skill must include at least one worked apply-recipe.json example", sf.path)
			}
		})
	}
}
