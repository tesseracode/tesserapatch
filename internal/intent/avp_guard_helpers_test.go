package intent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Repository access helpers
// ---------------------------------------------------------------------------

func repoRootDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the working directory")
		}
		dir = parent
	}
}

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRootDir(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// productionFiles parses every non-test Go file of a package directory.
func productionFiles(t *testing.T, rel string) (*token.FileSet, []*ast.File) {
	t.Helper()
	dir := filepath.Join(repoRootDir(t), filepath.FromSlash(rel))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", rel, err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		t.Fatalf("no production files parsed under %s", rel)
	}
	return fset, files
}

func parseFixtureSource(t *testing.T, src string) []*ast.File {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse sensitivity fixture: %v", err)
	}
	return []*ast.File{file}
}

// selectorSet collects every `pkg.Symbol` selector reference in the files.
func selectorSet(files []*ast.File) map[string]int {
	out := map[string]int{}
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			out[ident.Name+"."+sel.Sel.Name]++
			return true
		})
	}
	return out
}

func importPaths(files []*ast.File) map[string]bool {
	out := map[string]bool{}
	for _, file := range files {
		for _, imp := range file.Imports {
			if path, err := strconv.Unquote(imp.Path.Value); err == nil {
				out[path] = true
			}
		}
	}
	return out
}

func forbiddenSelectors(files []*ast.File, forbidden []string) error {
	seen := selectorSet(files)
	var found []string
	for _, name := range forbidden {
		if seen[name] > 0 {
			found = append(found, name)
		}
	}
	if len(found) > 0 {
		sort.Strings(found)
		return fmt.Errorf("forbidden reference(s) present: %s", strings.Join(found, ", "))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Report matrix used by the output-shape guards
// ---------------------------------------------------------------------------

// guardReportMatrix builds one report for every abort code and one for every
// reachable artifact state, so the output guards operate over the full domain
// rather than a happy-path sample.
func guardReportMatrix(t *testing.T) []Report {
	t.Helper()
	var reports []Report
	for _, code := range AbortCodes() {
		slug := testSlug
		if code == AbortSlugUnsafe {
			slug = ""
		}
		reports = append(reports, NewAbortReport(slug, code))
	}
	mutations := []func(*fakeRoot){
		nil,
		func(r *fakeRoot) { r.setFile(testSpec, nil) },
		func(r *fakeRoot) { r.remove(testSpec) },
		func(r *fakeRoot) { r.set(testSpec, fakeInfo{name: testSpec, mode: fs.ModeSymlink}) },
		func(r *fakeRoot) { r.set(testSpec, dir(testSpec)) },
		func(r *fakeRoot) { r.nodes[testSpec].openErr = fs.ErrPermission },
		func(r *fakeRoot) { r.set(testSpec, sized(testSpec, MaxArtifactBytes+1)) },
		func(r *fakeRoot) { r.setFile(testSidecar, []byte("[")) },
		func(r *fakeRoot) { r.setFile(testSidecar, nil) },
		func(r *fakeRoot) { r.remove(testStatus) },
		func(r *fakeRoot) { r.sameFile = func(a, b fs.FileInfo) bool { return a.Name() != testSpec } },
		func(r *fakeRoot) {
			r.set(testSpec, sized(testSpec, MaxArtifactBytes+1))
			r.set(testSidecar, sized(testSidecar, MaxArtifactBytes+1))
		},
	}
	for _, mutate := range mutations {
		root := fixtureRoot(t)
		if mutate != nil {
			mutate(root)
		}
		reports = append(reports, Inspect(root, testSlug, scratchBuffer()))
	}
	return reports
}

func renderAllSurfaces(report Report) string {
	var human, quiet, structured bytes.Buffer
	report.WriteHuman(&human)
	report.WriteQuiet(&quiet)
	if err := report.WriteJSON(&structured); err != nil {
		return "json render failed: " + err.Error()
	}
	return human.String() + "\n" + quiet.String() + "\n" + structured.String() + "\n" + report.ExitMessage()
}

// walkJSONKeys visits every object key at every nesting level.
func walkJSONKeys(value any, visit func(key string)) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			visit(key)
			walkJSONKeys(nested, visit)
		}
	case []any:
		for _, nested := range typed {
			walkJSONKeys(nested, visit)
		}
	}
}

var forbiddenJSONKeys = []string{
	"captured_at", "generated_at", "timestamp", "mtime", "size", "size_bytes",
	"bytes", "sha256", "hash", "content", "excerpt", "first_line", "title",
	"path_absolute", "symlink_target", "path_kind", "snapshot_id",
}

// checkForbiddenKeys is the key-name-scoped guard body. It is deliberately a
// standalone function so the sensitivity fixture can feed it a document the
// production renderer cannot produce.
func checkForbiddenKeys(document []byte) error {
	var decoded any
	if err := json.Unmarshal(document, &decoded); err != nil {
		return fmt.Errorf("report is not JSON: %w", err)
	}
	forbidden := map[string]bool{}
	for _, key := range forbiddenJSONKeys {
		forbidden[key] = true
	}
	var found []string
	walkJSONKeys(decoded, func(key string) {
		if forbidden[key] {
			found = append(found, key)
		}
	})
	if len(found) > 0 {
		sort.Strings(found)
		return fmt.Errorf("forbidden key(s) at some nesting level: %s", strings.Join(found, ", "))
	}
	return nil
}

var closedHumanLabels = []string{
	"prepare --check", "lifecycle state", "required", "optional", "provenance",
	"advisories", "abort", "readiness", "no artifacts were inspected",
	"analysis.md", "spec.md", "exploration.md", "artifacts/analysis.json",
	disclaimer,
}

// checkHumanLabels asserts every flush-left label the human renderer emits is
// a member of the closed §10.5 set. Indented lines are artifact rows and
// wrapped message bodies, which the other guards own.
func checkHumanLabels(rendered string) error {
	for _, line := range strings.Split(rendered, "\n") {
		if line == "" || strings.HasPrefix(line, " ") {
			continue
		}
		label := line
		if index := strings.Index(label, ":"); index > 0 {
			label = label[:index]
		}
		if fields := strings.Fields(label); len(fields) > 0 {
			if fields[0] == "prepare" {
				label = "prepare --check"
			} else if len(fields) > 1 && fields[0] == "lifecycle" {
				label = "lifecycle state"
			} else if len(fields) > 1 && fields[0] == "no" {
				label = "no artifacts were inspected"
			}
		}
		matched := false
		for _, known := range closedHumanLabels {
			if label == known {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("human surface emits a label outside the closed set: %q", line)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Workflow parsing (AVP-175)
// ---------------------------------------------------------------------------

// workflowStep is one parsed step of a GitHub Actions workflow. AVP-175 has
// to reason about *step structure* — which step is blocking, which one is
// allowed to fail, and in what order they run — so a substring scan over the
// whole file is not enough: `continue-on-error: true` anywhere in the file
// would otherwise look like it belonged to every step.
type workflowStep struct {
	Job   string
	Index int
	Name  string
	If    string
	Shell string
	Uses  string
	Run   string
	// ContinueOnError is the *raw* scalar as written, not a decoded bool.
	// Decoding to a bool at parse time collapsed every non-literal form —
	// `${{ true }}`, `!false`, `${{ vars.SOFT_FAIL }}` — onto `false`, i.e.
	// onto "blocking", so an expression that GitHub evaluates to true at run
	// time read as a blocking step to this guard. The raw text is retained so
	// classifyContinueOnError can reject what it cannot evaluate statically.
	ContinueOnError string
}

// parseWorkflowSteps parses the small YAML subset this repository's workflow
// uses: mapping keys, plain scalars and `|` block scalars, nested one level
// under `jobs:` → `<job>:` → `steps:`. Anything deeper (`with:`, `env:`) is
// skipped deliberately — no step field this guard needs lives there.
func parseWorkflowSteps(workflow string) ([]workflowStep, error) {
	lines := strings.Split(workflow, "\n")

	var steps []workflowStep
	var current *workflowStep
	var blockKey string
	var blockLines []string

	job := ""
	inJobs := false
	inSteps := false
	stepsIndent := -1
	itemIndent := -1
	contentIndent := -1

	closeBlock := func() {
		if blockKey == "" {
			return
		}
		if current != nil && blockKey == "run" {
			current.Run = strings.Join(blockLines, "\n")
		}
		blockKey = ""
		blockLines = nil
	}
	flush := func() {
		closeBlock()
		if current != nil {
			steps = append(steps, *current)
			current = nil
		}
	}

	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		indent := len(raw) - len(strings.TrimLeft(raw, " "))

		if blockKey != "" {
			if trimmed == "" {
				blockLines = append(blockLines, "")
				continue
			}
			if indent > contentIndent {
				blockLines = append(blockLines, raw[min(contentIndent+2, indent):])
				continue
			}
			closeBlock()
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if indent == 0 {
			flush()
			inSteps = false
			inJobs = trimmed == "jobs:"
			continue
		}
		if inJobs && indent == 2 && strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, "-") {
			flush()
			inSteps = false
			job = strings.TrimSuffix(trimmed, ":")
			continue
		}
		if trimmed == "steps:" {
			flush()
			inSteps = true
			stepsIndent = indent
			itemIndent = -1
			continue
		}
		if !inSteps {
			continue
		}
		if indent <= stepsIndent {
			flush()
			inSteps = false
			continue
		}
		if strings.HasPrefix(trimmed, "- ") && (itemIndent == -1 || indent == itemIndent) {
			flush()
			itemIndent = indent
			contentIndent = indent + 2
			current = &workflowStep{Job: job, Index: len(steps)}
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			indent = contentIndent
		}
		if current == nil || indent != contentIndent {
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// rawValue is the scalar exactly as written, before the quote and
		// comment normalisation below. `continue-on-error` is compared
		// against it so that `"true"` (a quoted string, not the YAML boolean)
		// and every expression form stay distinguishable from the literal.
		rawValue := value
		if value == "|" || value == ">" || value == "|-" {
			blockKey = key
			blockLines = nil
			continue
		}
		// Only strip a *paired* surrounding quote: `if: runner.os ==
		// 'Windows'` must keep its inner quotes, or every condition check
		// downstream would silently compare truncated text.
		if len(value) >= 2 && value[0] == value[len(value)-1] && (value[0] == '"' || value[0] == '\'') {
			value = value[1 : len(value)-1]
		} else if index := strings.Index(value, " #"); index >= 0 {
			// An unquoted plain scalar ends at " #": YAML reads the rest as
			// a comment. A step named `Test (... GH #17)` without quotes is
			// really named `Test (... GH` on the runner, so the parser must
			// see the same thing the workflow engine does.
			value = strings.TrimSpace(value[:index])
		}
		switch key {
		case "name":
			current.Name = value
		case "if":
			current.If = value
		case "shell":
			current.Shell = value
		case "uses":
			current.Uses = value
		case "run":
			current.Run = value
		case "continue-on-error":
			current.ContinueOnError = rawValue
		}
	}
	flush()

	if len(steps) == 0 {
		return nil, errors.New("no workflow steps parsed; the CI guard would be vacuous")
	}
	return steps, nil
}

// The only two platform conditions AVP-175 accepts on a step it gates. Rev-2
// matched them with strings.Contains, which accepted `runner.os == 'Windows'
// && false` (never runs), `runner.os == 'Windows' && github.event_name ==
// 'schedule'` (runs only on a cron the repository does not have) and every
// other conjunct that silently disables a blocking gate while leaving the
// text a reviewer greps for in place.
const (
	conditionWindows    = "runner.os == 'Windows'"
	conditionNonWindows = "runner.os != 'Windows'"
	conditionTagVersion = "startsWith(github.ref, 'refs/tags/v') && runner.os != 'Windows'"
)

// normalizeWorkflowCondition strips an optional `${{ }}` wrapper and collapses
// runs of whitespace, so formatting differences do not change the meaning of
// an exact comparison.
func normalizeWorkflowCondition(condition string) string {
	value := strings.TrimSpace(condition)
	if strings.HasPrefix(value, "${{") && strings.HasSuffix(value, "}}") {
		value = strings.TrimSpace(value[3 : len(value)-2])
	}
	return strings.Join(strings.Fields(value), " ")
}

// platformCondition requires a step's `if` to be exactly one of the two
// accepted platform expressions and reports which leg it selects.
func platformCondition(step workflowStep) (windows bool, err error) {
	switch normalizeWorkflowCondition(step.If) {
	case conditionWindows:
		return true, nil
	case conditionNonWindows:
		return false, nil
	default:
		return false, fmt.Errorf("step %q has the condition %q; only %q and %q are accepted, so a hidden conjunct cannot disable the gate",
			step.Name, step.If, conditionWindows, conditionNonWindows)
	}
}

// ---------------------------------------------------------------------------
// continue-on-error ownership (AVP-175)
// ---------------------------------------------------------------------------

// continueOnErrorMode is the static classification of a raw `continue-on-error`
// scalar. GitHub Actions accepts an arbitrary expression there, and an
// expression's value is decided at run time from inputs this guard cannot see
// (`vars.*`, `env.*`, `github.*`), so the guard admits only the two literals
// and rejects everything else instead of guessing.
type continueOnErrorMode int

const (
	continueOnErrorAbsent continueOnErrorMode = iota
	continueOnErrorFalse
	continueOnErrorTrue
	continueOnErrorNonLiteral
)

// classifyContinueOnError maps a raw scalar onto the closed set above. Only
// the bare YAML booleans `false` and `true` are literals: `"true"` is a
// quoted string, `${{ true }}` and `!false` are expressions, and a bare
// identifier is a variable reference.
func classifyContinueOnError(raw string) continueOnErrorMode {
	switch strings.TrimSpace(raw) {
	case "":
		return continueOnErrorAbsent
	case "false":
		return continueOnErrorFalse
	case "true":
		return continueOnErrorTrue
	default:
		return continueOnErrorNonLiteral
	}
}

// requireBlockingContinueOnError is the ownership rule for anything that must
// gate: the field is absent, or it is the exact literal `false`. Applied to
// the `test` job and to every step in it except the single GH #17
// allowed-failure step.
func requireBlockingContinueOnError(scope, name, raw string) error {
	switch classifyContinueOnError(raw) {
	case continueOnErrorAbsent, continueOnErrorFalse:
		return nil
	case continueOnErrorTrue:
		return fmt.Errorf("the %s %q is continue-on-error: true; it is advisory and gates nothing", scope, name)
	default:
		return fmt.Errorf("the %s %q declares continue-on-error: %s; only an absent field or the exact literal `false` is accepted here, because an expression is evaluated at run time from inputs this guard cannot see", scope, name, raw)
	}
}

// requireAllowedFailureContinueOnError is the mirror rule for the one step
// that is deliberately advisory: its ownership must be the exact literal
// `true`, so the file states the demotion outright rather than deriving it
// from an expression a reader would have to evaluate.
func requireAllowedFailureContinueOnError(name, raw string) error {
	if classifyContinueOnError(raw) != continueOnErrorTrue {
		return fmt.Errorf("the allowed-failure step %q declares continue-on-error: %q; it must be the exact literal `true`", name, raw)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Job-level workflow parsing (AVP-175)
// ---------------------------------------------------------------------------

// workflowJob carries the job-level fields AVP-175 reasons about. A step-level
// scan is not sufficient: `continue-on-error: true` declared on the *job*
// makes every step in it advisory, including a step this guard has just
// proven to be blocking, and `needs:` is what makes the release job depend on
// the test job at all.
type workflowJob struct {
	Name string
	// ContinueOnError is the raw scalar, for the same reason as the step-level
	// field: a job-level `continue-on-error: ${{ true }}` demotes every step
	// in the job and must not decode to "blocking".
	ContinueOnError string
	If              string
	Needs           []string
}

// parseWorkflowJobs reads the job-level mapping keys of `jobs:` → `<job>:`.
// Job-level keys sit at indent 4 in this workflow; step keys sit at indent 8
// under a `- ` item at indent 6, and every other nested mapping (`strategy:`,
// `with:`, `permissions:`) is deeper than 4, so an indent test separates them
// without a full YAML implementation.
func parseWorkflowJobs(workflow string) (map[string]workflowJob, error) {
	jobs := map[string]workflowJob{}
	name := ""
	inJobs := false
	needsKey := false

	for _, raw := range strings.Split(workflow, "\n") {
		trimmed := strings.TrimSpace(raw)
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if indent == 0 {
			inJobs = trimmed == "jobs:"
			name = ""
			needsKey = false
			continue
		}
		if !inJobs {
			continue
		}
		if indent == 2 && strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, "-") {
			name = strings.TrimSuffix(trimmed, ":")
			needsKey = false
			jobs[name] = workflowJob{}
			continue
		}
		if name == "" {
			continue
		}
		if needsKey {
			// A `needs:` block sequence: `- test` items indented under it.
			if indent > 4 && strings.HasPrefix(trimmed, "- ") {
				job := jobs[name]
				job.Needs = append(job.Needs, strings.TrimSpace(strings.TrimPrefix(trimmed, "-")))
				jobs[name] = job
				continue
			}
			needsKey = false
		}
		if indent != 4 {
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		rawValue := value
		if len(value) >= 2 && value[0] == value[len(value)-1] && (value[0] == '"' || value[0] == '\'') {
			value = value[1 : len(value)-1]
		}
		job := jobs[name]
		switch key {
		case "name":
			job.Name = value
		case "if":
			job.If = value
		case "continue-on-error":
			job.ContinueOnError = rawValue
		case "needs":
			switch {
			case value == "":
				needsKey = true
			case strings.HasPrefix(value, "["):
				for _, entry := range strings.Split(strings.Trim(value, "[]"), ",") {
					if entry = strings.TrimSpace(entry); entry != "" {
						job.Needs = append(job.Needs, entry)
					}
				}
			default:
				job.Needs = append(job.Needs, value)
			}
		}
		jobs[name] = job
	}

	if len(jobs) == 0 {
		return nil, errors.New("no workflow jobs parsed; the CI guard would be vacuous")
	}
	return jobs, nil
}
