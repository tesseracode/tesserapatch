package gitutil

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestS7PIB438CentralGitExecutorGuardAndWrongInput(t *testing.T) {
	gitSource := s7GitutilSource(t, "ignore.go")
	cliSource := s7RepositorySource(t, "internal/cli/prepare_publish.go")
	t.Run("baseline", func(t *testing.T) {
		if err := validateS7PIB438GitSources(gitSource, cliSource); err != nil {
			t.Fatal(err)
		}
	})
	fixtures := []struct {
		name string
		git  string
		cli  string
	}{
		{
			name: "rev-parse-show-toplevel",
			git: strings.Replace(gitSource,
				`[]string{"rev-parse", "--is-inside-work-tree"}`,
				`[]string{"rev-parse", "--show-toplevel"}`, 1),
			cli: cliSource,
		},
		{
			name: "second-G1-probe",
			git:  gitSource,
			cli: strings.Replace(cliSource,
				`gitState, gitErr := gitutil.DiscoverGitState(repoRoot)`,
				`_, _ = gitutil.DiscoverGitState(repoRoot)
	gitState, gitErr := gitutil.DiscoverGitState(repoRoot)`, 1),
		},
		{
			name: "absolute-lane",
			git: strings.Replace(gitSource,
				`"--", ".tpatch/local/"`,
				`"--", "/.tpatch/local/"`, 1),
			cli: cliSource,
		},
		{
			name: "duplicate-privacy-gate",
			git: strings.Replace(gitSource,
				`env:           prepareGitEnvironment(os.Environ()),`,
				`env:           prepareGitEnvironment(prepareGitEnvironment(os.Environ())),`, 1),
			cli: cliSource,
		},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			if err := validateS7PIB438GitSources(fixture.git, fixture.cli); err == nil {
				t.Fatal("same PIB-438 validator accepted the one-delta production mutation")
			}
		})
	}
}

func validateS7PIB438GitSources(gitSource, cliSource string) error {
	fileset := token.NewFileSet()
	gitFile, err := parser.ParseFile(fileset, "ignore.go", gitSource, 0)
	if err != nil {
		return err
	}
	expected := map[string][]string{
		"DiscoverGitState":              {`"rev-parse"`, `"--is-inside-work-tree"`},
		"IsIgnoredWithState":            {`"check-ignore"`, `"-q"`, `"--no-index"`, `"--"`, "disarmLeadingColon(repoRelative)"},
		"AnythingTrackedUnderWithState": {`"--literal-pathspecs"`, `"ls-files"`, `"--"`, `".tpatch/local/"`},
		"IsTpatchTrackedWithState":      {`"ls-files"`, `"--"`, `".tpatch"`},
	}
	for functionName, wantArgs := range expected {
		function, err := s7PIB438Function(gitFile, functionName)
		if err != nil {
			return err
		}
		requests := s7PIB438Calls(function.Body, "runGitProcess")
		if len(requests) != 1 {
			return fmt.Errorf("PIB-438 %s Git process requests = %d, want 1", functionName, len(requests))
		}
		args, gateCount, err := s7PIB438Request(fileset, requests[0])
		if err != nil {
			return fmt.Errorf("PIB-438 %s: %w", functionName, err)
		}
		if fmt.Sprint(args) != fmt.Sprint(wantArgs) {
			return fmt.Errorf("PIB-438 %s argv = %v, want %v", functionName, args, wantArgs)
		}
		if gateCount != 1 {
			return fmt.Errorf("PIB-438 %s privacy gates = %d, want exactly one", functionName, gateCount)
		}
		for _, arg := range args {
			if strings.HasPrefix(strings.Trim(arg, `"`), "/") {
				return fmt.Errorf("PIB-438 %s has absolute Git lane %q", functionName, arg)
			}
		}
	}
	if strings.Count(gitSource, "func prepareGitEnvironment(") != 1 ||
		strings.Count(gitSource, `exec.Command("git", request.args...)`) != 1 {
		return fmt.Errorf("PIB-438 central executor/privacy gate definitions drifted")
	}
	if !strings.Contains(gitSource, "if !validRepoRelative(repoRelative)") ||
		!strings.Contains(gitSource, "fs.ValidPath(value)") {
		return fmt.Errorf("PIB-438 G2 lost repo-relative validation")
	}

	cliFile, err := parser.ParseFile(fileset, "prepare_publish.go", cliSource, 0)
	if err != nil {
		return err
	}
	runPrepare, err := s7PIB438Function(cliFile, "runPreparePublish")
	if err != nil {
		return err
	}
	for functionName, want := range map[string]int{
		"DiscoverGitState":              1,
		"IsIgnoredWithState":            1,
		"AnythingTrackedUnderWithState": 1,
		"IsTpatchTrackedWithState":      1,
	} {
		got := 0
		ast.Inspect(runPrepare.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, _ := selector.X.(*ast.Ident)
			if pkg != nil && pkg.Name == "gitutil" && selector.Sel.Name == functionName {
				got++
			}
			return true
		})
		if got != want {
			return fmt.Errorf("PIB-438 runPreparePublish %s calls = %d, want %d", functionName, got, want)
		}
	}
	return nil
}

func s7PIB438Function(file *ast.File, name string) (*ast.FuncDecl, error) {
	var found []*ast.FuncDecl
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Name.Name == name && function.Recv == nil && function.Body != nil {
			found = append(found, function)
		}
	}
	if len(found) != 1 {
		return nil, fmt.Errorf("PIB-438 function %s count = %d", name, len(found))
	}
	return found[0], nil
}

func s7PIB438Calls(body *ast.BlockStmt, name string) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, _ := call.Fun.(*ast.Ident)
		if ident != nil && ident.Name == name {
			calls = append(calls, call)
		}
		return true
	})
	return calls
}

func s7PIB438Request(fileset *token.FileSet, call *ast.CallExpr) ([]string, int, error) {
	if len(call.Args) != 1 {
		return nil, 0, fmt.Errorf("runGitProcess argument count = %d", len(call.Args))
	}
	request, ok := call.Args[0].(*ast.CompositeLit)
	if !ok {
		return nil, 0, fmt.Errorf("runGitProcess does not receive a literal request")
	}
	var args []string
	gates := 0
	for _, element := range request.Elts {
		kv, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, _ := kv.Key.(*ast.Ident)
		if key == nil {
			continue
		}
		switch key.Name {
		case "args":
			values, ok := kv.Value.(*ast.CompositeLit)
			if !ok {
				return nil, 0, fmt.Errorf("args is not a literal array")
			}
			for _, value := range values.Elts {
				var rendered bytes.Buffer
				if err := format.Node(&rendered, fileset, value); err != nil {
					return nil, 0, err
				}
				args = append(args, rendered.String())
			}
		case "env":
			ast.Inspect(kv.Value, func(node ast.Node) bool {
				typed, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				ident, _ := typed.Fun.(*ast.Ident)
				if ident != nil && ident.Name == "prepareGitEnvironment" {
					gates++
				}
				return true
			})
			var rendered bytes.Buffer
			if err := format.Node(&rendered, fileset, kv.Value); err != nil {
				return nil, 0, err
			}
			if rendered.String() != "prepareGitEnvironment(os.Environ())" {
				return nil, gates, fmt.Errorf("environment route = %s", rendered.String())
			}
		}
	}
	return args, gates, nil
}

func s7GitutilSource(t *testing.T, name string) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(current), name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func s7RepositorySource(t *testing.T, rel string) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func s7PIB438StringLiteral(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}
