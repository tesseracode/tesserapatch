//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestS7AQRerunEmittersAreDerivedAndCanonical(t *testing.T) {
	// PIB-498: retry-bearing report models and every human-renderer handoff are
	// derived from source; real branch reports preserve command/CWD semantics.
	sources := map[string]string{
		"internal/cli/prepare_publish.go":        s6RepoFile(t, "internal/cli/prepare_publish.go"),
		"internal/cli/feature_intent_archive.go": s6RepoFile(t, "internal/cli/feature_intent_archive.go"),
	}
	if err := validateS7AQRerunEmitters(sources); err != nil {
		t.Fatal(err)
	}

	for _, sensitivity := range []struct {
		name   string
		rel    string
		mutate func(string) string
	}{
		{
			name: "rev-8-heading",
			rel:  "internal/cli/prepare_publish.go",
			mutate: func(source string) string {
				return strings.Replace(
					source,
					"fmt.Fprintln(w, prepareRetryHeader)",
					`fmt.Fprintln(w, "Run this again from the same workspace root to perform it:")`,
					1,
				)
			},
		},
		{
			name: "numbered-heading",
			rel:  "internal/cli/prepare_publish.go",
			mutate: func(source string) string {
				return strings.Replace(
					source,
					"fmt.Fprintln(w, prepareRetryHeader)",
					`fmt.Fprintln(w, "3. run this again from the same workspace root:")`,
					1,
				)
			},
		},
		{
			name: "blank-between-heading-and-command",
			rel:  "internal/cli/feature_intent_archive.go",
			mutate: func(source string) string {
				return strings.Replace(
					source,
					"fmt.Fprintln(w, prepareRetryHeader)\n\tfmt.Fprintf(w, \"  %s\\n\", retry)",
					"fmt.Fprintln(w, prepareRetryHeader)\n\tfmt.Fprintln(w)\n\tfmt.Fprintf(w, \"  %s\\n\", retry)",
					1,
				)
			},
		},
		{
			name: "indented-heading",
			rel:  "internal/cli/prepare_publish.go",
			mutate: func(source string) string {
				return strings.Replace(
					source,
					"fmt.Fprintln(w, prepareRetryHeader)",
					`fmt.Fprintln(w, "  Run this again from the same workspace root:")`,
					1,
				)
			},
		},
		{
			name: "trailing-heading-space",
			rel:  "internal/cli/prepare_publish.go",
			mutate: func(source string) string {
				return strings.Replace(
					source,
					"fmt.Fprintln(w, prepareRetryHeader)",
					`fmt.Fprintln(w, "Run this again from the same workspace root: ")`,
					1,
				)
			},
		},
	} {
		mutated := cloneS7AQSources(sources)
		before := mutated[sensitivity.rel]
		mutated[sensitivity.rel] = sensitivity.mutate(before)
		if mutated[sensitivity.rel] == before {
			t.Fatalf("PIB-498 %s mutation anchor missing", sensitivity.name)
		}
		if err := validateS7AQRerunEmitters(mutated); err == nil {
			t.Fatalf("PIB-498 same validator accepted %s", sensitivity.name)
		}
	}

	recovered := s7AQObserveTerminalRecovery(t, prepareModeGenerate, "CP3", false)
	s7AQRequireEquivalentRetry(
		t, "terminal-journal-recovery", recovered.report.Recovery.Retry,
		"tpatch prepare "+recovered.slug+" --json --quiet",
	)
	var recoveredHuman bytes.Buffer
	writePreparePublishHuman(&recoveredHuman, recovered.report)
	s7AQAssertRetryEmission(
		t, "terminal-journal-recovery", recovered.report.Recovery.Retry,
		recovered.report.Recovery.RetryCWD, recoveredHuman.String(),
	)

	partial := s7APRunPartialPurge(t)
	s7AQRequireEquivalentRetry(
		t, "partial-purge", partial.report.PurgeProgress.Retry,
		"tpatch feature intent-archive purge "+partial.slug+" --all --yes --json --quiet",
	)
	var partialHuman bytes.Buffer
	writeIntentArchivePurgeHuman(&partialHuman, partial.report)
	s7AQAssertRetryEmission(
		t, "partial-purge", partial.report.PurgeProgress.Retry,
		partial.report.PurgeProgress.RetryCWD, partialHuman.String(),
	)

	code, pendingOut, stderr, _ := runPrepare(
		t, "--path", partial.root, "feature", "intent-archive", "purge", partial.slug,
		"--all", "--json", "--quiet",
	)
	pending := decodeIntentArchivePurgeReport(t, pendingOut)
	if code != 0 || stderr != "" || pending.PendingPurge == nil {
		t.Fatalf("PIB-498 pending preview = exit:%d stderr:%q report:%+v",
			code, stderr, pending)
	}
	s7AQRequireEquivalentRetry(
		t, "pending-purge-preview", pending.PendingPurge.Retry,
		"tpatch feature intent-archive purge "+partial.slug+" --all --yes --json --quiet",
	)
	var pendingHuman bytes.Buffer
	writeIntentArchivePurgeHuman(&pendingHuman, pending)
	s7AQAssertRetryEmission(
		t, "pending-purge-preview", pending.PendingPurge.Retry,
		pending.PendingPurge.RetryCWD, pendingHuman.String(),
	)

	retryArgv, err := s7APParseRenderedCommand(pending.PendingPurge.Retry)
	if err != nil {
		t.Fatal(err)
	}
	code, recoveryOut, stderr := s7APRunFromWorkspace(t, partial.root, retryArgv)
	recovery := decodeIntentArchivePurgeReport(t, recoveryOut)
	if code != 0 || stderr != "" || recovery.Recovery == nil {
		t.Fatalf("PIB-498 pending purge recovery = exit:%d stderr:%q report:%+v",
			code, stderr, recovery)
	}
	s7AQRequireEquivalentRetry(
		t, "terminal-pending-purge-recovery", recovery.Recovery.Retry,
		"tpatch feature intent-archive purge "+partial.slug+" --all --yes --json --quiet",
	)
	var recoveryHuman bytes.Buffer
	writeIntentArchivePurgeHuman(&recoveryHuman, recovery)
	s7AQAssertRetryEmission(
		t, "terminal-pending-purge-recovery", recovery.Recovery.Retry,
		recovery.Recovery.RetryCWD, recoveryHuman.String(),
	)

	divergent := s7APRunDivergentPurge(t)
	if divergent.code != 6 || divergent.report.Divergence == nil {
		t.Fatalf("PIB-498 divergence fixture = exit:%d report:%+v",
			divergent.code, divergent.report)
	}
	s7AQRequireEquivalentRetry(
		t, "archive-divergence", divergent.report.Divergence.Retry,
		"tpatch feature intent-archive purge "+divergent.slug+" --all --yes --json --quiet",
	)
	var divergentHuman bytes.Buffer
	writeIntentArchivePurgeHuman(&divergentHuman, divergent.report)
	s7AQAssertRetryEmission(
		t, "archive-divergence", divergent.report.Divergence.Retry,
		divergent.report.Divergence.RetryCWD, divergentHuman.String(),
	)

	s7AQRunListRetryPopulations(t)
}

func s7AQRunListRetryPopulations(t *testing.T) {
	t.Helper()
	root, slug := intentArchiveCLIWorkspace(t)
	body := []byte("AQ dangling retry\n")
	replacement := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactAnalysis, body, store.IntentArchiveWireRetained,
	)
	writeIntentArchiveCLIFixture(
		t, root, slug,
		intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
		map[string][]byte{},
	)
	s7AQAssertRealListRetry(
		t, "dangling-reference", root, slug,
		intentArchiveBlobRetry(slug, []string{replacement.ContentSHA256}),
	)

	root, slug = intentArchiveCLIWorkspace(t)
	body = []byte("AQ corrupt expected\n")
	replacement = intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactAnalysis, body, store.IntentArchiveWireRetained,
	)
	writeIntentArchiveCLIFixture(
		t, root, slug,
		intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
		map[string][]byte{replacement.ContentSHA256: []byte("AQ corrupt actual\n")},
	)
	s7AQAssertRealListRetry(
		t, "corrupt-blob", root, slug,
		intentArchiveBlobRetry(slug, []string{replacement.ContentSHA256}),
	)

	root, slug = intentArchiveCLIWorkspace(t)
	body = []byte("AQ tombstone unreferenced\n")
	replacement = intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactAnalysis, body, store.IntentArchiveWireTombstoned,
	)
	writeIntentArchiveCLIFixture(
		t, root, slug,
		intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, replacement)),
		map[string][]byte{replacement.ContentSHA256: body},
	)
	s7AQAssertRealListRetry(
		t, "tombstone-unreferenced-blob", root, slug,
		"tpatch feature intent-archive purge "+slug+" --orphans --yes",
	)

	root, slug = intentArchiveCLIWorkspace(t)
	body = []byte("AQ tombstone live mixed\n")
	retained := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactAnalysis, body, store.IntentArchiveWireRetained,
	)
	tombstoned := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactSpec, body, store.IntentArchiveWireTombstoned,
	)
	writeIntentArchiveCLIFixture(
		t, root, slug,
		intentArchiveCLIIndex(
			t, slug,
			intentArchiveCLIGeneration(t, slug, retained),
			intentArchiveCLIGeneration(t, slug, tombstoned),
		),
		map[string][]byte{retained.ContentSHA256: body},
	)
	s7AQAssertRealListRetry(
		t, "tombstone-live-mixed-blob", root, slug,
		intentArchiveBlobRetry(slug, []string{retained.ContentSHA256}),
	)
}

func validateS7AQRerunEmitters(sources map[string]string) error {
	const heading = "Run this again from the same workspace root:"
	const phrase = "run this again from the same workspace root"
	if len(sources) != 2 ||
		sources["internal/cli/prepare_publish.go"] == "" ||
		sources["internal/cli/feature_intent_archive.go"] == "" {
		return errors.New("retry renderer source inventory drift")
	}
	type parsedSource struct {
		name   string
		source string
		file   *ast.File
	}
	var parsed []parsedSource
	exactHeadingLiterals := 0
	retryModels := 0
	headingSites := 0
	archiveHelperCalls := 0
	for name, source := range sources {
		fileset := token.NewFileSet()
		file, err := parser.ParseFile(fileset, name, source, 0)
		if err != nil {
			return err
		}
		parsed = append(parsed, parsedSource{name: name, source: source, file: file})
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				return true
			}
			if value == heading {
				exactHeadingLiterals++
			}
			if strings.Contains(strings.ToLower(value), phrase) && value != heading {
				exactHeadingLiterals = -1000
			}
			return true
		})
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, specification := range general.Specs {
				typeSpec, _ := specification.(*ast.TypeSpec)
				structType, _ := typeSpec.Type.(*ast.StructType)
				if typeSpec == nil || structType == nil {
					continue
				}
				fields := map[string]bool{}
				hasJSON := false
				for _, field := range structType.Fields.List {
					if field.Tag != nil && strings.Contains(field.Tag.Value, `json:`) {
						hasJSON = true
					}
					for _, name := range field.Names {
						fields[name.Name] = true
					}
				}
				if !hasJSON || !fields["Retry"] && !fields["Repair"] {
					continue
				}
				retryModels++
				if fields["Retry"] && !fields["RetryCWD"] {
					return fmt.Errorf("%s retry model %s lacks RetryCWD", name, typeSpec.Name)
				}
				if fields["Repair"] && !fields["Retry"] && !fields["RepairCWD"] {
					return fmt.Errorf("%s repair model %s lacks a workspace CWD field", name, typeSpec.Name)
				}
			}
		}
	}
	if exactHeadingLiterals != 1 {
		return fmt.Errorf("canonical retry heading literal count = %d, want 1", exactHeadingLiterals)
	}
	if retryModels != 13 {
		return fmt.Errorf("derived retry-bearing report models = %d, want 13", retryModels)
	}

	for _, item := range parsed {
		for _, declaration := range item.file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			if function.Name.Name == "writeIntentArchiveRetry" {
				if err := s7AQValidateRetryHelper(function); err != nil {
					return err
				}
			}
			var err error
			s7AQWalkStatementLists(function.Body, func(statements []ast.Stmt) {
				if err != nil {
					return
				}
				for index, statement := range statements {
					if s7AQIsHeadingStatement(statement) {
						headingSites++
						if index+1 >= len(statements) ||
							!s7AQIsRetryCommandStatement(statements[index+1]) {
							err = fmt.Errorf("%s:%s has content between heading and retry",
								item.name, function.Name.Name)
							return
						}
					}
					expression, ok := statement.(*ast.ExprStmt)
					if !ok {
						continue
					}
					call, ok := expression.X.(*ast.CallExpr)
					if ok && s7AQCalledIdentifier(call.Fun, "writeIntentArchiveRetry") {
						archiveHelperCalls++
					}
				}
			})
			if err != nil {
				return err
			}
		}
	}
	if headingSites != 4 {
		return fmt.Errorf("derived heading sites = %d, want 4", headingSites)
	}
	// PIB-556 renders remaining purge stages as repair lines, not immediate
	// rerun handoffs, so they deliberately do not use the canonical heading.
	if archiveHelperCalls != 7 {
		return fmt.Errorf("derived archive retry handoffs = %d, want 7", archiveHelperCalls)
	}
	return nil
}

func s7AQValidateRetryHelper(function *ast.FuncDecl) error {
	if len(function.Body.List) != 3 ||
		!s7AQIsHeadingStatement(function.Body.List[1]) ||
		!s7AQIsRetryCommandStatement(function.Body.List[2]) {
		return errors.New("writeIntentArchiveRetry lost its canonical guard/heading/command body")
	}
	return nil
}

func s7AQWalkStatementLists(block *ast.BlockStmt, visit func([]ast.Stmt)) {
	if block == nil {
		return
	}
	visit(block.List)
	for _, statement := range block.List {
		switch value := statement.(type) {
		case *ast.BlockStmt:
			s7AQWalkStatementLists(value, visit)
		case *ast.IfStmt:
			s7AQWalkStatementLists(value.Body, visit)
			if alternate, ok := value.Else.(*ast.BlockStmt); ok {
				s7AQWalkStatementLists(alternate, visit)
			}
		case *ast.ForStmt:
			s7AQWalkStatementLists(value.Body, visit)
		case *ast.RangeStmt:
			s7AQWalkStatementLists(value.Body, visit)
		case *ast.SwitchStmt:
			s7AQWalkStatementLists(value.Body, visit)
		case *ast.TypeSwitchStmt:
			s7AQWalkStatementLists(value.Body, visit)
		case *ast.SelectStmt:
			s7AQWalkStatementLists(value.Body, visit)
		case *ast.CaseClause:
			visit(value.Body)
		}
	}
}

func s7AQIsHeadingStatement(statement ast.Stmt) bool {
	expression, ok := statement.(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := expression.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 2 || !s7AQSelectorNamed(call.Fun, "fmt", "Fprintln") {
		return false
	}
	identifier, ok := call.Args[1].(*ast.Ident)
	return ok && identifier.Name == "prepareRetryHeader"
}

func s7AQIsRetryCommandStatement(statement ast.Stmt) bool {
	expression, ok := statement.(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := expression.X.(*ast.CallExpr)
	if !ok || len(call.Args) < 3 || !s7AQSelectorNamed(call.Fun, "fmt", "Fprintf") {
		return false
	}
	formatLiteral, ok := call.Args[1].(*ast.BasicLit)
	if !ok || formatLiteral.Kind != token.STRING {
		return false
	}
	formatValue, err := strconv.Unquote(formatLiteral.Value)
	return err == nil && strings.HasPrefix(formatValue, "  %s\n")
}

func s7AQSelectorNamed(expression ast.Expr, packageName, name string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != name {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	return ok && identifier.Name == packageName
}

func s7AQCalledIdentifier(expression ast.Expr, name string) bool {
	identifier, ok := expression.(*ast.Ident)
	return ok && identifier.Name == name
}

func s7AQAssertRetryEmission(
	t *testing.T, name, retry, retryCWD, human string,
) {
	t.Helper()
	if retry == "" || retryCWD != store.IntentArchiveRepairCWD ||
		strings.Contains(retry, "--path") {
		t.Fatalf("PIB-498 %s retry semantics = retry:%q cwd:%q",
			name, retry, retryCWD)
	}

	argv, err := s7APParseRenderedCommand(retry)
	if err != nil || len(argv) < 2 {
		t.Fatalf("PIB-498 %s retry is not executable: argv:%v err:%v", name, argv, err)
	}
	lines := strings.Split(strings.TrimSuffix(human, "\n"), "\n")
	matches := 0
	for index, line := range lines {
		if strings.TrimSpace(line) != retry {
			continue
		}
		matches++
		if index == 0 || lines[index-1] != prepareRetryHeader {
			t.Fatalf("PIB-498 %s retry lacks adjacent canonical heading:\n%s", name, human)
		}
	}
	if matches != 1 || strings.Count(human, prepareRetryHeader) != 1 {
		t.Fatalf("PIB-498 %s heading/retry count = heading:%d retry:%d\n%s",
			name, strings.Count(human, prepareRetryHeader), matches, human)
	}
}

func s7AQRequireEquivalentRetry(t *testing.T, name, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("PIB-498 %s retry = %q, want equivalent %q", name, got, want)
	}
}

func s7AQAssertRealListRetry(t *testing.T, name, root, slug, want string) {
	t.Helper()
	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "list", slug,
		"--json", "--quiet",
	)
	if code != 0 && code != 3 || code == 3 && stderr == "" {
		t.Fatalf("PIB-498 %s list = exit:%d stderr:%q\n%s", name, code, stderr, stdout)
	}
	report := decodeIntentArchiveListReport(t, stdout)
	var retries []struct {
		command string
		cwd     string
	}
	for _, generation := range report.Generations {
		for _, entry := range generation.Entries {
			if entry.Retry != "" {
				retries = append(retries, struct {
					command string
					cwd     string
				}{entry.Retry, entry.RetryCWD})
			}
		}
	}
	for _, orphan := range report.Orphans {
		if orphan.Retry != "" {
			retries = append(retries, struct {
				command string
				cwd     string
			}{orphan.Retry, orphan.RetryCWD})
		}
	}
	for _, object := range report.CorruptObjects {
		if object.Retry != "" {
			retries = append(retries, struct {
				command string
				cwd     string
			}{object.Retry, object.RetryCWD})
		}
	}
	if len(retries) == 0 {
		t.Fatalf("PIB-498 %s emitted no tpatch retry: %+v", name, report)
	}
	humanCode, human, _, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "list", slug,
	)
	if humanCode != code {
		t.Fatalf("PIB-498 %s human exit = %d, want %d", name, humanCode, code)
	}
	seen := map[string]bool{}
	for _, retry := range retries {
		s7AQRequireEquivalentRetry(t, name, retry.command, want)
		if seen[retry.command] {
			continue
		}
		seen[retry.command] = true
		s7AQAssertRetryEmission(t, name, retry.command, retry.cwd, human)
	}
}
