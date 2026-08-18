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
	Job             string
	Index           int
	Name            string
	If              string
	Shell           string
	Uses            string
	Run             string
	ContinueOnError bool
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
			current.ContinueOnError = value == "true"
		}
	}
	flush()

	if len(steps) == 0 {
		return nil, errors.New("no workflow steps parsed; the CI guard would be vacuous")
	}
	return steps, nil
}

// runsOnWindows reports whether a step's `if` expression leaves the step
// enabled on the windows-latest leg. An empty condition runs everywhere, so
// it runs on Windows too.
func runsOnWindows(condition string) bool {
	if strings.TrimSpace(condition) == "" {
		return true
	}
	if strings.Contains(condition, "runner.os != 'Windows'") {
		return false
	}
	if strings.Contains(condition, "matrix.os == 'ubuntu-latest'") ||
		strings.Contains(condition, "matrix.os == 'macos-latest'") {
		return false
	}
	return true
}
