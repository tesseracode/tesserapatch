//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestS7PIB437AuthorityDurationDocsGuardAndWrongInput(t *testing.T) {
	prd := s6RepoFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
	adr := s6RepoFile(t, "docs/adrs/ADR-035-intent-bundle-publication-and-history.md")
	command := s6RepoFile(t, "internal/cli/prepare.go")
	t.Run("baseline", func(t *testing.T) {
		if err := validateS7PIB437DurationDocs(command, prd, adr); err != nil {
			t.Fatal(err)
		}
	})
	fixtures := []struct {
		name    string
		command string
		prd     string
		adr     string
	}{
		{
			name: "command-total-bound",
			command: strings.Replace(command,
				`"Total generation deadline"`, `"Total command deadline"`, 1),
			prd: prd, adr: adr,
		},
		{
			name:    "authority-total-bound",
			command: command,
			prd: strings.Replace(prd,
				"this PRD makes no total-command or total-lock-hold\npromise",
				"the command and total-lock-hold are bounded by the total timeout", 1),
			adr: adr,
		},
		{
			name:    "adr-filesystem-bound",
			command: command, prd: prd,
			adr: strings.Replace(adr,
				"No deadline is\nconsulted after staging validation",
				"The deadline remains consulted through publication and release", 1),
		},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			if err := validateS7PIB437DurationDocs(
				fixture.command, fixture.prd, fixture.adr,
			); err == nil {
				t.Fatal("same PIB-437 validator accepted the one-delta real input mutation")
			}
		})
	}
}

func TestS7PIB445DoctorD9SourceGuardAndWrongInput(t *testing.T) {
	input := s7PIB445BaselineInput(t)
	t.Run("baseline", func(t *testing.T) {
		if err := validateS7PIB445Doctor(input); err != nil {
			t.Fatal(err)
		}
	})
	fixtures := []struct {
		name   string
		mutate func(s7PIB445Input) s7PIB445Input
	}{
		{
			name: "reachable-root-open",
			mutate: func(wrong s7PIB445Input) s7PIB445Input {
				wrong.sources = s7CloneStrings(wrong.sources)
				wrong.sources["internal/workflow/doctor_d9.go"] = strings.Replace(
					wrong.sources["internal/workflow/doctor_d9.go"],
					"boundary := newDoctorD9Boundary(ctx.root)",
					"boundary := newDoctorD9Boundary(ctx.root)\n\t_ = boundary.OpenRoot(\".\")", 1,
				)
				return wrong
			},
		},
		{
			name: "live-authority-output-claim",
			mutate: func(wrong s7PIB445Input) s7PIB445Input {
				wrong.output += "\nauthority is free\n"
				return wrong
			},
		},
		{
			name: "prechange-golden-byte",
			mutate: func(wrong s7PIB445Input) s7PIB445Input {
				wrong.currentGoldens = s7CloneBytes(wrong.currentGoldens)
				wrong.currentGoldens["doctor-D1.txt"] = append(
					append([]byte(nil), wrong.currentGoldens["doctor-D1.txt"]...), '\n',
				)
				return wrong
			},
		},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			if err := validateS7PIB445Doctor(fixture.mutate(input)); err == nil {
				t.Fatal("same PIB-445 validator accepted the one-delta real input mutation")
			}
		})
	}
}

func s7PIB445BaselineInput(t *testing.T) s7PIB445Input {
	t.Helper()
	sources := s7PIB445ProductionSources(t)
	root, _ := prepareS4Workspace(t, "S7 PIB 445 output")
	code, output, stderr, _ := runPrepare(
		t, "--path", root, "doctor", "--check", "D9", "--json",
	)
	if code != 0 || stderr != "" {
		t.Fatalf("clean D9 output = exit:%d stderr:%q output:%q", code, stderr, output)
	}
	helpCode, help, helpErr, _ := runPrepare(t, "doctor", "--help")
	if helpCode != 0 || helpErr != "" {
		t.Fatalf("doctor help = exit:%d stderr:%q", helpCode, helpErr)
	}
	currentGoldens, baseGoldens := s7PIB445GoldenSnapshots(t)
	return s7PIB445Input{
		sources: sources, output: output, help: help,
		currentGoldens: currentGoldens, baseGoldens: baseGoldens,
	}
}

func TestS7PIB448RefusalPrecedenceHelpAndReadOnlyListGuard(t *testing.T) {
	input := s7PIB448BaselineInput(t)
	t.Run("baseline", func(t *testing.T) {
		if err := validateS7PIB448(input); err != nil {
			t.Fatal(err)
		}
	})
	fixtures := []struct {
		name   string
		mutate func(s7PIB448Input) s7PIB448Input
	}{
		{
			name: "partial-exit-collapse",
			mutate: func(wrong s7PIB448Input) s7PIB448Input {
				wrong.baseline = cloneS6ContractBaseline(wrong.baseline)
				fixture := wrong.baseline.refusalEvidence["archive-purge-partial"]
				fixture.expectedExit = 3
				wrong.baseline.refusalEvidence["archive-purge-partial"] = fixture
				return wrong
			},
		},
		{
			name: "distinct-index-code-collapse",
			mutate: func(wrong s7PIB448Input) s7PIB448Input {
				wrong.baseline = cloneS6ContractBaseline(wrong.baseline)
				fixture := wrong.baseline.refusalEvidence["archive-index-foreign"]
				fixture.emittedCode = "archive-index-corrupt"
				wrong.baseline.refusalEvidence["archive-index-foreign"] = fixture
				return wrong
			},
		},
		{
			name: "list-acquires-authority",
			mutate: func(wrong s7PIB448Input) s7PIB448Input {
				wrong.listSource = strings.Replace(
					wrong.listSource,
					"func runFeatureIntentArchiveList(cmd *cobra.Command, rawSlug string) error {\n",
					"func runFeatureIntentArchiveList(cmd *cobra.Command, rawSlug string) error {\n\t_, _ = intentArchiveAcquireAuthority(\"\")\n",
					1,
				)
				return wrong
			},
		},
		{
			name: "stale-section-anchor",
			mutate: func(wrong s7PIB448Input) s7PIB448Input {
				wrong.adr = strings.Replace(wrong.adr, "§9.7.2", "§99.99", 1)
				return wrong
			},
		},
		{
			name: "row-catalog-drift",
			mutate: func(wrong s7PIB448Input) s7PIB448Input {
				wrong.prd = strings.Replace(wrong.prd,
					"| PIB-448 | G | refusal/precedence/help/JSON/row catalog guard |",
					"| PIB-448 | C | refusal/precedence/help/JSON/row catalog guard |", 1)
				return wrong
			},
		},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			if err := validateS7PIB448(fixture.mutate(input)); err == nil {
				t.Fatal("same PIB-448 validator accepted the one-delta real input mutation")
			}
		})
	}
}

func s7PIB448BaselineInput(t *testing.T) s7PIB448Input {
	t.Helper()
	baseline, err := s6ContractBaseline(t)
	if err != nil {
		t.Fatal(err)
	}
	source := s6RepoFile(t, "internal/cli/feature_intent_archive.go")
	prd := s6RepoFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
	adr := s6RepoFile(t, "docs/adrs/ADR-035-intent-bundle-publication-and-history.md")
	helpCode, help, stderr, _ := runPrepare(
		t, "feature", "intent-archive", "list", "--help",
	)
	if helpCode != 0 || stderr != "" {
		t.Fatalf("archive list help = exit:%d stderr:%q", helpCode, stderr)
	}
	return s7PIB448Input{
		baseline: baseline, listSource: source, help: help, prd: prd, adr: adr,
	}
}

type s7PIB448Input struct {
	baseline   s6ContractBaselineEvidence
	listSource string
	help       string
	prd        string
	adr        string
}

func validateS7PIB448(input s7PIB448Input) error {
	if err := validateS6Refusals(
		input.baseline.refusalCatalog,
		input.baseline.refusalCatalog,
		input.baseline.refusalEvidence,
	); err != nil {
		return fmt.Errorf("PIB-448 full refusal/JSON/human catalog: %w", err)
	}
	for code, wantExit := range map[string]int{
		"workspace-root-changed":                    5,
		"workspace-root-replaced-after-publication": 6,
		"archive-blob-dangling":                     3,
		"archive-purge-partial":                     5,
		"archive-index-corrupt":                     3,
		"archive-index-version-unsupported":         3,
		"archive-index-foreign":                     3,
		"archive-index-path-escape":                 3,
		"archive-index-generation-mismatch":         3,
		"archive-index-storage-inconsistent":        3,
	} {
		fixture, ok := input.baseline.refusalEvidence[code]
		if !ok || fixture.emittedCode != code || fixture.exit != wantExit ||
			fixture.expectedExit != wantExit || strings.TrimSpace(fixture.remediation) == "" {
			return fmt.Errorf("PIB-448 refusal %q parity = %+v present=%t", code, fixture, ok)
		}
		observation := input.baseline.refusalObservations[code]
		if observation.code != code || observation.exit != wantExit ||
			strings.TrimSpace(observation.remediation) == "" {
			return fmt.Errorf("PIB-448 refusal %q actual JSON observation = %+v", code, observation)
		}
	}
	file, err := parser.ParseFile(token.NewFileSet(), "feature_intent_archive.go", input.listSource, 0)
	if err != nil {
		return err
	}
	functions := map[string]*ast.FuncDecl{}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Recv == nil && function.Body != nil {
			functions[function.Name.Name] = function
		}
	}
	root := functions["runFeatureIntentArchiveList"]
	if root == nil {
		return fmt.Errorf("PIB-448 archive list entry missing")
	}
	reachable := map[string]bool{}
	queue := []string{"runFeatureIntentArchiveList"}
	for len(queue) != 0 {
		name := queue[0]
		queue = queue[1:]
		if reachable[name] || functions[name] == nil {
			continue
		}
		reachable[name] = true
		ast.Inspect(functions[name].Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			ident, _ := call.Fun.(*ast.Ident)
			if ident != nil && functions[ident.Name] != nil {
				queue = append(queue, ident.Name)
			}
			return true
		})
	}
	for name := range reachable {
		var forbidden string
		ast.Inspect(functions[name].Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch called := call.Fun.(type) {
			case *ast.Ident:
				switch called.Name {
				case "intentArchiveAcquireAuthority", "intentArchiveRecoverPurge":
					forbidden = called.Name
				}
			case *ast.SelectorExpr:
				switch called.Sel.Name {
				case "Acquire", "Flock", "CASIndex", "PublishBlob", "RemoveBlob", "Write", "WriteFile", "Rename", "Remove":
					forbidden = called.Sel.Name
				}
			}
			return forbidden == ""
		})
		if forbidden != "" {
			return fmt.Errorf("PIB-448 archive list reachable %s calls %s", name, forbidden)
		}
	}
	for _, token := range []string{
		"list <slug>", "--json", "--quiet",
	} {
		if !strings.Contains(input.help, token) {
			return fmt.Errorf("PIB-448 archive list help missing %q", token)
		}
	}
	if strings.Contains(input.help, "--yes") {
		return fmt.Errorf("PIB-448 read-only list help exposes --yes")
	}
	row := s7MarkdownTableRow(input.prd, "PIB-448")
	for _, token := range []string{
		"| PIB-448 | G |",
		"refusal/precedence/help/JSON/row catalog guard",
		"archive list stays read-only/no-lock",
		"no stale section reference remains",
	} {
		if !strings.Contains(row, token) {
			return fmt.Errorf("PIB-448 normative row lost %q: %s", token, row)
		}
	}
	headings := map[string]bool{}
	headingPattern := regexp.MustCompile(`(?m)^#{2,6}\s+([0-9]+(?:\.[0-9]+)*)\b`)
	for _, match := range headingPattern.FindAllStringSubmatch(input.prd, -1) {
		headings[match[1]] = true
	}
	referencePattern := regexp.MustCompile(`§([0-9]+(?:\.[0-9]+)*)`)
	for _, document := range []string{input.prd, input.adr} {
		for _, match := range referencePattern.FindAllStringSubmatch(document, -1) {
			if !headings[match[1]] {
				return fmt.Errorf("PIB-448 stale normative section reference §%s", match[1])
			}
		}
	}
	return nil
}

func validateS7PIB437DurationDocs(command, prd, adr string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "prepare.go", command, 0)
	if err != nil {
		return err
	}
	flagHelp := map[string]string{}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) != 3 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Duration" {
			return true
		}
		name, nameOK := s7StringLiteral(call.Args[0])
		help, helpOK := s7StringLiteral(call.Args[2])
		if nameOK && helpOK {
			flagHelp[name] = help
		}
		return true
	})
	if fmt.Sprint(flagHelp) != fmt.Sprint(map[string]string{
		"timeout": "Total generation deadline", "timeout-phase": "Per-generator deadline",
	}) {
		return fmt.Errorf("PIB-437 timeout command help = %v", flagHelp)
	}
	authority, err := s7MarkdownSection(prd, "### 7.4 ", "### 7.5 ")
	if err != nil {
		return err
	}
	budget, err := s7MarkdownSection(prd, "### 11.5 ", "### 11.6 ")
	if err != nil {
		return err
	}
	d18, err := s7MarkdownSection(adr, "### D18 ", "### D19 ")
	if err != nil {
		return err
	}
	for label, scoped := range map[string]struct {
		section string
		scope   string
	}{
		"PRD authority": {authority, "generation"},
		"PRD budget":    {budget, "generation"},
		"ADR D18":       {d18, "provider"},
	} {
		lower := strings.ToLower(scoped.section)
		if !strings.Contains(lower, scoped.scope) {
			return fmt.Errorf("PIB-437 %s lacks provider-generation scope %q", label, scoped.scope)
		}
		for _, forbidden := range []string{
			"total command deadline",
			"total-command bound",
			"authority hold is bounded",
			"total-lock-hold are bounded",
			"also bound inspection",
		} {
			if strings.Contains(lower, forbidden) {
				return fmt.Errorf("PIB-437 %s asserts command/authority total bound %q", label, forbidden)
			}
		}
	}
	for _, required := range []struct {
		section string
		token   string
	}{
		{authority, "Only provider generation obeys `--timeout` / `--timeout-phase`"},
		{authority, "no total-command or total-lock-hold\npromise"},
		{budget, "they do not bound the command or authority lifetime"},
		{budget, "Filesystem, Git,\n  recovery, publication, fsync and release have no hard wall-clock bound"},
		{d18, "No deadline is\nconsulted after staging validation"},
	} {
		if !strings.Contains(required.section, required.token) {
			return fmt.Errorf("PIB-437 provider-only duration claim missing %q", required.token)
		}
	}
	return nil
}

func s7StringLiteral(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}

type s7PIB445Input struct {
	sources        map[string]string
	output         string
	help           string
	currentGoldens map[string][]byte
	baseGoldens    map[string][]byte
}

func validateS7PIB445Doctor(input s7PIB445Input) error {
	functions := map[string][]*ast.FuncDecl{}
	importsByFunction := map[*ast.FuncDecl]map[string]string{}
	fileset := token.NewFileSet()
	for name, source := range input.sources {
		file, err := parser.ParseFile(fileset, name, source, 0)
		if err != nil {
			return err
		}
		imports := map[string]string{}
		for _, spec := range file.Imports {
			pathValue, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			alias := filepath.Base(pathValue)
			if spec.Name != nil {
				alias = spec.Name.Name
			}
			imports[alias] = pathValue
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			functions[function.Name.Name] = append(functions[function.Name.Name], function)
			importsByFunction[function] = imports
		}
	}
	if len(functions["runDoctorD9"]) != 1 || len(functions["doctorCmd"]) != 1 {
		return fmt.Errorf("PIB-445 D9/command roots are not unique")
	}
	reachable := map[*ast.FuncDecl]bool{}
	queue := []*ast.FuncDecl{functions["runDoctorD9"][0], functions["doctorCmd"][0]}
	for len(queue) != 0 {
		function := queue[0]
		queue = queue[1:]
		if reachable[function] {
			continue
		}
		reachable[function] = true
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch called := call.Fun.(type) {
			case *ast.Ident:
				queue = append(queue, functions[called.Name]...)
			case *ast.SelectorExpr:
				queue = append(queue, functions[called.Sel.Name]...)
			}
			return true
		})
	}
	for function := range reachable {
		ast.Inspect(function.Body, func(node ast.Node) bool {
			return true
		})
		var issue string
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch called := call.Fun.(type) {
			case *ast.Ident:
				if called.Name == "open" || called.Name == "flock" {
					issue = called.Name
				}
			case *ast.SelectorExpr:
				switch called.Sel.Name {
				case "OpenRoot", "OpenDot", "Control", "Flock", "Fstatfs", "Unlock",
					"RunProcess", "Write", "WriteFile", "Create", "Rename", "Remove", "RemoveAll":
					issue = called.Sel.Name
				}
				base, _ := called.X.(*ast.Ident)
				if base != nil {
					pathValue := importsByFunction[function][base.Name]
					if pathValue == "os" && called.Sel.Name == "OpenRoot" {
						issue = "os.OpenRoot"
					}
					if pathValue == "os/exec" || pathValue == "syscall" ||
						strings.Contains(pathValue, "/intentlock") {
						issue = pathValue + "." + called.Sel.Name
					}
				}
			}
			return issue == ""
		})
		if issue != "" {
			return fmt.Errorf("PIB-445 reachable D9 call graph contains forbidden %s in %s", issue, function.Name.Name)
		}
	}
	combinedOutput := strings.ToLower(input.output + "\n" + input.help)
	for _, claim := range []string{
		"authority held", "authority is free", "holder identity", "process id",
	} {
		if strings.Contains(combinedOutput, claim) {
			return fmt.Errorf("PIB-445 D9 output speculates about live authority: %q", claim)
		}
	}
	for _, required := range []string{
		"D9 is evidence-only and never repairs findings",
		"opens or probes workspace mutation authority",
		"A removed prepare journal is unrecoverable and ordinarily undetectable",
	} {
		if !strings.Contains(input.help, required) {
			return fmt.Errorf("PIB-445 actual doctor help lost claim %q", required)
		}
	}
	if !strings.Contains(input.output, `"check_id": "D9"`) ||
		!strings.Contains(input.output, `"findings": []`) {
		return fmt.Errorf("PIB-445 actual clean D9 JSON shape drifted: %s", input.output)
	}
	if len(input.currentGoldens) != 51 || len(input.baseGoldens) != 51 {
		return fmt.Errorf("PIB-445 prechange golden counts = current:%d base:%d, want 51",
			len(input.currentGoldens), len(input.baseGoldens))
	}
	for index := 1; index <= 8; index++ {
		name := fmt.Sprintf("doctor-D%d.txt", index)
		if input.currentGoldens[name] == nil {
			return fmt.Errorf("PIB-445 missing prechange %s", name)
		}
	}
	for name, current := range input.currentGoldens {
		base, ok := input.baseGoldens[name]
		if !ok || !bytes.Equal(current, base) {
			return fmt.Errorf("PIB-445 prechange golden %s changed from 2d9492c", name)
		}
	}
	return nil
}

func s7PIB445ProductionSources(t *testing.T) map[string]string {
	t.Helper()
	sources := map[string]string{
		"internal/cli/doctor.go": s6RepoFile(t, "internal/cli/doctor.go"),
	}
	root := avpRepoRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "internal", "workflow"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		rel := filepath.ToSlash(filepath.Join("internal", "workflow", entry.Name()))
		sources[rel] = s6RepoFile(t, rel)
	}
	return sources
}

func s7PIB445GoldenSnapshots(t *testing.T) (map[string][]byte, map[string][]byte) {
	t.Helper()
	const base = "2d9492cbf6fd9c69c5aa75d64d05983c05e1563f"
	root := avpRepoRoot(t)
	dir := filepath.Join(root, "internal", "cli", "testdata", "prepare-pib-goldens")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	current := map[string][]byte{}
	baseline := map[string][]byte{}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".txt") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		current[name] = data
		rel := filepath.ToSlash(filepath.Join("internal", "cli", "testdata", "prepare-pib-goldens", name))
		command := exec.Command("git", "show", base+":"+rel)
		command.Dir = root
		data, err = command.Output()
		if err != nil {
			t.Fatalf("git show %s:%s: %v", base, rel, err)
		}
		baseline[name] = data
	}
	return current, baseline
}

func s7CloneStrings(source map[string]string) map[string]string {
	clone := make(map[string]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func s7CloneBytes(source map[string][]byte) map[string][]byte {
	clone := make(map[string][]byte, len(source))
	for key, value := range source {
		clone[key] = append([]byte(nil), value...)
	}
	return clone
}

func s7FunctionBody(source, function string) (string, error) {
	start := strings.Index(source, "func "+function+"(")
	if start < 0 {
		return "", fmt.Errorf("function %s missing", function)
	}
	rest := source[start:]
	next := strings.Index(rest[len("func "+function+"("):], "\nfunc ")
	if next >= 0 {
		rest = rest[:len("func "+function+"(")+next]
	}
	return rest, nil
}
