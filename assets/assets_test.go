package assets

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/workflow"
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
	"tpatch land",
	"tpatch reconcile",
	"tpatch reconcile audit-retirement",
	"tpatch reconcile confirm-upstreamed",
	"tpatch provider",
	"tpatch config",
	"tpatch cycle",
	"tpatch test",
	"tpatch feature patch refresh",
	"tpatch feature patch fixup",
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
	// Slice D (M15-W3, PRD-verify-freshness §4.4): every shipped skill
	// surface must carry the freshness-overlay bullet so harness agents
	// know `tpatch verify <slug>` exists, what it writes, and that it
	// is NOT the project test runner. Anchor on the verbatim opening
	// phrase so the parity guard fires the moment the bullet is removed
	// or paraphrased.
	{"verify-freshness/bullet", "Verify before composing."},
	{"verify-freshness/all-mode", "tpatch verify --all"},
	// feat-amend-dependent-warning (v0.7.0): every shipped skill
	// surface must mention the new `dependent-broken` derived label
	// so harness agents know what to do when `tpatch status` flags a
	// downstream feature whose base SHA was rewritten away by an
	// upstream amend or rebase. Substring match on the label name is
	// sufficient — the string is unique to this troubleshooting
	// bullet across all six surfaces.
	{"dependent-broken/troubleshoot-line", "dependent-broken"},
	{"patch-amend/refresh", "tpatch feature patch refresh <slug>"},
	{"patch-amend/fixup", "tpatch feature patch fixup <slug>"},
	{"patch-amend/stale-label", "parent-generation-stale"},
}

// requiredRegexAnchors holds parity anchors that need richer matching
// than a literal substring check. Each anchor must match at least once
// in every shipped skill surface.
var requiredRegexAnchors = []struct {
	label string
	re    *regexp.Regexp
}{
	// Slice 3 (M16, v0.6.4) — revision-1 (external supervisor finding
	// on eab2c3c, 2026-05-10). Every shipped skill surface must
	// recommend the simplified one-verb invocation `tpatch apply
	// <slug>` (no explicit --mode flag). The CLI default has been
	// `--mode auto` since v0.6.0; this anchor locks the docs/skills
	// alignment so future drift back to `apply --mode execute` in
	// invocation-recommendation prose is caught at test time.
	//
	// The regex requires `tpatch apply <slug>` to be followed by one
	// of:
	//   - end-of-line (with optional trailing whitespace)
	//   - whitespace + a non-`-` non-whitespace character (e.g. `→`,
	//     `#`, `.`, a word — anything that is NOT the start of a
	//     CLI-flag continuation like ` --mode done`)
	//   - a backtick (closing inline-code wrapper such as
	//     `` `tpatch apply <slug>` ``)
	// This explicitly REJECTS substring false-passes such as
	// `tpatch apply <slug> --mode done` (the advanced-fallback
	// example), which previously satisfied a strings.Contains check
	// on the copilot prompt and generic workflow surfaces even though
	// neither carried a true standalone recommendation.
	//
	// The four-mode ladder (prepare/started/execute/done) remains
	// documented as an advanced fallback — this anchor only asserts
	// that the simple form is present, not that the ladder is gone.
	{
		label: "apply-default-auto/simple-invocation",
		re:    regexp.MustCompile("(?m)tpatch apply <slug>(?:\\s*$|\\s+[^-\\s]|`)"),
	},
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
			for _, a := range requiredRegexAnchors {
				if !a.re.MatchString(content) {
					t.Errorf("%s (%s) missing required regex anchor [%s]: %q",
						sf.name, sf.path, a.label, a.re.String())
				}
			}
		})
	}
}

// docsRefCandidateRe matches, in a single pass, EITHER a fully
// qualified URL token (consumed harmlessly so it cannot be
// reinterpreted as a docs reference) OR a candidate repo-relative
// `docs/...md` reference, including the variants `./docs/...md`,
// `../docs/...md`, and `/docs/...md`. Go's regexp has no
// lookbehind, so a single-pattern "preceded by `://`" exclusion
// isn't expressible; instead we list the URL alternative first
// and only treat the second branch (capture group 1) as a
// failure. The PRD §10 question 4 leaves URL policy open; for v1
// the guard forbids only bare repo-relative paths.
var docsRefCandidateRe = regexp.MustCompile(`[a-z][a-z0-9+.-]*://\S+|(?:^|[^A-Za-z0-9_])((?:\.{0,2}/)?docs/[A-Za-z0-9_./-]+\.md)\b`)

// findRepoRelativeDocsRefs returns every bare repo-relative
// `docs/...md` reference (including `./`, `../`, `/` prefixed
// variants) in `content`. URL-embedded paths are skipped because
// the URL alternative of `docsRefCandidateRe` consumes them
// without producing a capture in group 1.
func findRepoRelativeDocsRefs(content string) []string {
	var out []string
	for _, m := range docsRefCandidateRe.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 && m[1] != "" {
			out = append(out, m[1])
		}
	}
	return out
}

// TestSkillDocReferencesAreSelfContained enforces ADR-020 / PRD
// `feat-skill-doc-references-user-visible`: shipped skill surfaces
// must not point end users at development-repo docs (e.g.
// `docs/land.md`, `docs/reconcile.md`) that are not installed by
// `tpatch init`. Any command-critical guidance must be inlined as
// a concise snippet in each surface so installed skills work
// offline. URL-prefixed references (`https://.../docs/foo.md`) are
// allowed for now (PRD §10 q4 left open); only bare repo-relative
// paths (and their `./`, `../`, `/` prefixed siblings) fail.
func TestSkillDocReferencesAreSelfContained(t *testing.T) {
	// Probe table — synthetic strings exercising every variant we
	// expect the guard to (dis)allow. Kept inside the test so a
	// future change to the regex must continue to satisfy them.
	probes := []struct {
		name      string
		content   string
		wantFails []string // expected captured docs paths
	}{
		{name: "bare", content: "See docs/land.md", wantFails: []string{"docs/land.md"}},
		{name: "dot-slash", content: "See ./docs/land.md", wantFails: []string{"./docs/land.md"}},
		{name: "dot-dot-slash", content: "See ../docs/land.md", wantFails: []string{"../docs/land.md"}},
		{name: "leading-slash", content: "See /docs/land.md", wantFails: []string{"/docs/land.md"}},
		{name: "parens", content: "(docs/land.md)", wantFails: []string{"docs/land.md"}},
		{name: "https-url", content: "See https://example.com/docs/land.md", wantFails: nil},
		{name: "http-url", content: "See http://example.com/docs/land.md", wantFails: nil},
		{name: "file-url", content: "See file:///home/user/docs/land.md", wantFails: nil},
	}
	for _, p := range probes {
		t.Run("probe/"+p.name, func(t *testing.T) {
			got := findRepoRelativeDocsRefs(p.content)
			if !reflect.DeepEqual(got, p.wantFails) {
				t.Fatalf("probe %q: got %#v, want %#v", p.name, got, p.wantFails)
			}
		})
	}

	for _, sf := range skillFiles {
		t.Run(sf.name, func(t *testing.T) {
			data, err := Skills.ReadFile(sf.path)
			if err != nil {
				t.Fatalf("cannot read %s: %v", sf.path, err)
			}
			for _, ref := range findRepoRelativeDocsRefs(string(data)) {
				t.Errorf("%s (%s) contains forbidden repo-relative docs reference: %q — inline the guidance instead (ADR-020 / PRD-skill-doc-strategy)",
					sf.name, sf.path, ref)
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
