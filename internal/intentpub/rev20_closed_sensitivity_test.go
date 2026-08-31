package intentpub

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRev20ClosedSensitivityCarriersExistAndExecute answers the two carried
// notes of the post-close external review, which claimed the semantic-no-op CAS
// drift fixture and the noncanonical-artifact-path fixture were missing. They
// are not: both are declared in this package and both are executed here, by
// name, through the ordinary runner. The row is written as a guard rather than
// as prose so a later deletion or rename of either fixture fails a test instead
// of silently reopening the note.
//
// Nothing about the two fixtures is modified to satisfy this guard: it reads
// their declarations and then runs them.
func TestRev20ClosedSensitivityCarriersExistAndExecute(t *testing.T) {
	carriers := []struct {
		name   string
		file   string
		body   func(*testing.T)
		tokens []string
	}{
		{
			name: "TestExecuteSemanticNoOpCASRejectsDriftBeforePublication",
			file: "transaction_test.go",
			body: TestExecuteSemanticNoOpCASRejectsDriftBeforePublication,
			tokens: []string{
				"CodeEntryChanged",
				`"no-op-cas"`,
				"PointBeforeEntryCAS",
				"OutcomeRolledBack",
			},
		},
		{
			name: "TestPlanRejectsArtifactBoundToNoncanonicalPath",
			file: "plan_stage_hardening_test.go",
			body: TestPlanRejectsArtifactBoundToNoncanonicalPath,
			tokens: []string{
				"CodeInvalidPlan",
				`"canonical-path"`,
				"canonicalRel(testSlug, ArtifactSpec)",
			},
		},
	}

	fileset := token.NewFileSet()
	for _, carrier := range carriers {
		carrier := carrier
		t.Run(carrier.name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(".", carrier.file))
			if err != nil {
				t.Fatalf("%s: %v", carrier.file, err)
			}
			header := string(source)
			if cut := strings.Index(header, "\npackage "); cut > 0 {
				header = header[:cut]
			}
			if strings.Contains(header, "//go:build") {
				t.Fatalf("%s is build-constrained, so the fixture is not run by default", carrier.file)
			}
			parsed, err := parser.ParseFile(fileset, carrier.file, source, 0)
			if err != nil {
				t.Fatal(err)
			}
			var declaration *ast.FuncDecl
			for _, node := range parsed.Decls {
				function, ok := node.(*ast.FuncDecl)
				if ok && function.Recv == nil && function.Name.Name == carrier.name {
					declaration = function
				}
			}
			if declaration == nil {
				t.Fatalf("%s declares no %s", carrier.file, carrier.name)
			}
			if declaration.Body == nil || len(declaration.Body.List) == 0 {
				t.Fatalf("%s has an empty body", carrier.name)
			}
			if len(declaration.Type.Params.List) != 1 {
				t.Fatalf("%s is not an ordinary Test(t *testing.T) function", carrier.name)
			}
			rendered := string(source)
			start := strings.Index(rendered, "func "+carrier.name+"(")
			if start < 0 {
				t.Fatalf("%s source is missing %s", carrier.file, carrier.name)
			}
			end := strings.Index(rendered[start:], "\n}\n")
			if end < 0 {
				t.Fatalf("%s body is unterminated", carrier.name)
			}
			body := rendered[start : start+end]
			for _, assertion := range carrier.tokens {
				if !strings.Contains(body, assertion) {
					t.Fatalf("%s no longer asserts %q", carrier.name, assertion)
				}
			}
			if strings.Contains(body, "t.Skip") {
				t.Fatalf("%s skips itself, so it proves nothing", carrier.name)
			}
			// Executed, not merely inventoried.
			carrier.body(t)
		})
	}
}
