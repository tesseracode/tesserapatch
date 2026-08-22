//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const s7AQYesLiteral = "prepare: --yes is only valid with --abandon-transaction"

func TestS7AQFlagContracts(t *testing.T) {
	t.Run("PIB-500", func(t *testing.T) {
		type observation struct {
			report preparePublishReport
			bytes  map[string][]byte
		}
		run := func(withFlag bool) observation {
			root, slug := prepareS4Workspace(t, "AQ allow heuristic")
			args := []string{"--path", root, "prepare", slug}
			if withFlag {
				args = append(args, "--allow-heuristic")
			}
			args = append(args, "--json", "--quiet")
			code, stdout, stderr, _ := runPrepare(t, args...)
			report := prepareS4Report(t, stdout)
			if code != 0 || stderr != "" || report.Outcome != "published" {
				t.Fatalf("PIB-500 allow=%t = exit:%d stderr:%q report:%+v",
					withFlag, code, stderr, report)
			}
			produced := map[string][]byte{}
			for _, rel := range []string{
				"analysis.md",
				"spec.md",
				"exploration.md",
				"artifacts/analysis.json",
			} {
				body, err := os.ReadFile(filepath.Join(
					root, ".tpatch", "features", slug, filepath.FromSlash(rel),
				))
				if err != nil {
					t.Fatal(err)
				}
				produced[rel] = body
			}
			return observation{report: report, bytes: produced}
		}
		baseline := run(false)
		flagged := run(true)
		if !s7AQHasAdvisory(flagged.report, "allow-heuristic-redundant") ||
			s7AQHasAdvisory(baseline.report, "allow-heuristic-redundant") ||
			!reflect.DeepEqual(flagged.bytes, baseline.bytes) {
			t.Fatalf("PIB-500 default allow-heuristic parity = baseline:%+v flagged:%+v",
				baseline.report, flagged.report)
		}
	})

	t.Run("PIB-501", func(t *testing.T) {
		for _, mode := range []struct {
			name string
			args []string
		}{
			{name: "check", args: []string{"--check"}},
			{name: "manual", args: []string{"--manual"}},
			{name: "abandon", args: []string{"--abandon-transaction"}},
		} {
			root, slug := prepareS4Workspace(t, "AQ heuristic mutex "+mode.name)
			before := snapshotTreeMetadata(t, "PIB-501", root)
			args := []string{"--path", root, "prepare", slug}
			args = append(args, mode.args...)
			args = append(args, "--allow-heuristic")
			code, stdout, stderr, _ := runPrepare(t, args...)
			after := snapshotTreeMetadata(t, "PIB-501", root)
			if code != 1 || stdout != "" ||
				!strings.Contains(stderr, "flags in the group") ||
				!strings.Contains(stderr, "allow-heuristic") ||
				before != after {
				t.Fatalf("PIB-501 %s mutex = exit:%d stdout:%q stderr:%q changed:%t",
					mode.name, code, stdout, stderr, before != after)
			}
		}
	})

	t.Run("PIB-502", func(t *testing.T) {
		for _, mode := range []struct {
			name string
			args []string
		}{
			{name: "default"},
			{name: "check", args: []string{"--check"}},
			{name: "manual", args: []string{"--manual"}},
			{name: "regenerate", args: []string{"--regenerate"}},
		} {
			root, slug := prepareS4Workspace(t, "AQ illegal yes "+mode.name)
			before := snapshotTreeMetadata(t, "PIB-502", root)
			args := []string{"--path", root, "prepare", slug}
			args = append(args, mode.args...)
			args = append(args, "--yes")
			code, stdout, stderr, _ := runPrepare(t, args...)
			after := snapshotTreeMetadata(t, "PIB-502", root)
			if code != 1 || stdout != "" ||
				stderr != "error: "+s7AQYesLiteral+"\n" ||
				before != after {
				t.Fatalf("PIB-502 %s --yes = exit:%d stdout:%q stderr:%q changed:%t",
					mode.name, code, stdout, stderr, before != after)
			}
		}

		root, slug := prepareS4Workspace(t, "AQ abandon yes table")
		s6WriteJournalFixture(t, root, slug, "journal-corrupt")
		before := snapshotTreeMetadata(t, "PIB-502 preview", filepath.Join(root, ".tpatch"))
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--abandon-transaction", "--json", "--quiet",
		)
		preview := prepareS4Report(t, stdout)
		afterPreview := snapshotTreeMetadata(t, "PIB-502 preview", filepath.Join(root, ".tpatch"))
		if code != 0 || stderr != "" || preview.Outcome != "abandon-planned" ||
			preview.Abandoned == nil || before != afterPreview {
			t.Fatalf("PIB-502 abandon preview = exit:%d stderr:%q report:%+v changed:%t",
				code, stderr, preview, before != afterPreview)
		}
		code, stdout, stderr, _ = runPrepare(
			t, "--path", root, "prepare", slug,
			"--abandon-transaction", "--yes", "--json", "--quiet",
		)
		confirmed := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" || confirmed.Outcome != "abandoned" ||
			confirmed.Abandoned == nil || len(confirmed.Abandoned.Moved) == 0 {
			t.Fatalf("PIB-502 abandon confirmation = exit:%d stderr:%q report:%+v",
				code, stderr, confirmed)
		}
	})
}

func TestS7AQYesSourceGuard(t *testing.T) {
	t.Run("PIB-503", func(t *testing.T) {
		source := s6RepoFile(t, "internal/cli/prepare.go")
		if err := validateS7AQYesSource(source); err != nil {
			t.Fatal(err)
		}
		duplicate := source + "\nconst duplicateAQYes = " + strconv.Quote(s7AQYesLiteral) + "\n"
		if err := validateS7AQYesSource(duplicate); err == nil {
			t.Fatal("PIB-503 same validator accepted a second fixed literal")
		}
		mutex := strings.Replace(
			source,
			"\treturn cmd\n}",
			"\tcmd.MarkFlagsMutuallyExclusive(\"yes\", \"manual\")\n\treturn cmd\n}",
			1,
		)
		if err := validateS7AQYesSource(mutex); err == nil {
			t.Fatal("PIB-503 same validator accepted a cobra --yes mutex")
		}
		methodValue := strings.Replace(
			source,
			"\treturn cmd\n}",
			"\tmark := cmd.MarkFlagsMutuallyExclusive\n\tmark(\"yes\", \"manual\")\n\treturn cmd\n}",
			1,
		)
		if err := validateS7AQYesSource(methodValue); err == nil {
			t.Fatal("PIB-503 same validator accepted a --yes method-value mutex")
		}
		wrapper := strings.Replace(
			source,
			"\treturn cmd\n}",
			"\ts7AQMarkMutex(cmd, \"yes\", \"manual\")\n\treturn cmd\n}",
			1,
		) + `
func s7AQMarkMutex(cmd *cobra.Command, names ...string) {
	cmd.MarkFlagsMutuallyExclusive(names...)
}
`
		if err := validateS7AQYesSource(wrapper); err == nil {
			t.Fatal("PIB-503 same validator accepted a --yes wrapper mutex")
		}
		parameter := strings.Replace(
			source,
			"\treturn cmd\n}",
			"\ts7AQInvokeMutex(cmd.MarkFlagsMutuallyExclusive, \"yes\", \"manual\")\n\treturn cmd\n}",
			1,
		) + `
func s7AQInvokeMutex(mark func(...string), names ...string) {
	mark(names...)
}
`
		if err := validateS7AQYesSource(parameter); err == nil {
			t.Fatal("PIB-503 same validator accepted a --yes callable-parameter mutex")
		}
	})
}

func validateS7AQYesSource(source string) error {
	fileset := token.NewFileSet()
	file, err := parser.ParseFile(fileset, "prepare.go", source, 0)
	if err != nil {
		return err
	}
	literalCount := 0
	var prepare *ast.FuncDecl
	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.BasicLit:
			if value.Kind != token.STRING {
				return true
			}
			decoded, decodeErr := strconv.Unquote(value.Value)
			if decodeErr == nil && decoded == s7AQYesLiteral {
				literalCount++
			}
		case *ast.FuncDecl:
			if value.Recv == nil && value.Name.Name == "prepareCmd" {
				prepare = value
			}
		}
		return true
	})
	if literalCount != 1 {
		return fmt.Errorf("fixed --yes literal declarations = %d, want 1", literalCount)
	}
	if prepare == nil || prepare.Body == nil {
		return errors.New("prepareCmd is missing")
	}
	runEValidations := 0
	model, err := s6BuildSourceTypeModel(map[string]string{
		"internal/cli/prepare.go": source,
	})
	if err != nil {
		return fmt.Errorf("type-check prepare --yes source: %w", err)
	}
	pkg := model.typedPackages["internal/cli"]
	if pkg == nil || pkg.info == nil || pkg.relFiles["internal/cli/prepare.go"] == nil {
		return errors.New("typed prepare.go source is missing")
	}
	analyzer := newS7AQMutexAnalyzer(pkg)
	typedPrepare := analyzer.functionsByName["prepareCmd"]
	if typedPrepare == nil {
		return errors.New("typed prepareCmd is missing")
	}
	if err := analyzer.inspectFunction(
		typedPrepare, s7AQMutexBindings{}, map[*types.Func]bool{},
	); err != nil {
		return err
	}
	ast.Inspect(prepare.Body, func(node ast.Node) bool {
		branch, ok := node.(*ast.IfStmt)
		if !ok {
			return true
		}
		condition := s7AQNodeText(source, branch.Cond)
		body := s7AQNodeText(source, branch.Body)
		if strings.Contains(condition, "options.yes") {
			if strings.Contains(condition, "options.mode != prepareModeAbandon") &&
				strings.Contains(body, "Code:    1") &&
				strings.Contains(body, strconv.Quote(s7AQYesLiteral)) {
				runEValidations++
			} else {
				err = errors.New("unclassified --yes command validation")
			}
		}
		return true
	})
	if err != nil {
		return err
	}
	if runEValidations != 1 {
		return fmt.Errorf("classified --yes validations = %d, want 1", runEValidations)
	}
	if analyzer.mutexCalls == 0 {
		return errors.New("prepare has no cobra mutex declarations to classify")
	}
	return nil
}

type s7AQMutexCallable struct {
	function     *types.Func
	cobraMethod  bool
	skipReceiver bool
}

type s7AQMutexBindings struct {
	callables map[types.Object]s7AQMutexCallable
	strings   map[types.Object][]string
}

type s7AQMutexAnalyzer struct {
	pkg             *s6TypedPackage
	assignments     map[types.Object][]ast.Expr
	functions       map[*types.Func]*ast.FuncDecl
	functionsByName map[string]*ast.FuncDecl
	mutexCalls      int
}

func newS7AQMutexAnalyzer(pkg *s6TypedPackage) *s7AQMutexAnalyzer {
	analyzer := &s7AQMutexAnalyzer{
		pkg: pkg, assignments: map[types.Object][]ast.Expr{},
		functions:       map[*types.Func]*ast.FuncDecl{},
		functionsByName: map[string]*ast.FuncDecl{},
	}
	for _, file := range pkg.files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			object, _ := pkg.info.Defs[function.Name].(*types.Func)
			if object != nil {
				analyzer.functions[object] = function
				analyzer.functionsByName[function.Name.Name] = function
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.AssignStmt:
				for index, left := range value.Lhs {
					identifier, _ := left.(*ast.Ident)
					if identifier == nil || identifier.Name == "_" || index >= len(value.Rhs) {
						continue
					}
					object := pkg.info.Defs[identifier]
					if object == nil {
						object = pkg.info.Uses[identifier]
					}
					if object != nil {
						analyzer.assignments[object] = append(
							analyzer.assignments[object], value.Rhs[index],
						)
					}
				}
			case *ast.ValueSpec:
				for index, name := range value.Names {
					if index >= len(value.Values) {
						continue
					}
					if object := pkg.info.Defs[name]; object != nil {
						analyzer.assignments[object] = append(
							analyzer.assignments[object], value.Values[index],
						)
					}
				}
			case *ast.RangeStmt:
				identifier, _ := value.Value.(*ast.Ident)
				if identifier == nil || identifier.Name == "_" {
					break
				}
				object := pkg.info.Defs[identifier]
				if object == nil {
					object = pkg.info.Uses[identifier]
				}
				if object != nil {
					analyzer.assignments[object] = append(
						analyzer.assignments[object], value.X,
					)
				}
			}
			return true
		})
	}
	return analyzer
}

func (analyzer *s7AQMutexAnalyzer) inspectFunction(
	declaration *ast.FuncDecl,
	bindings s7AQMutexBindings,
	visiting map[*types.Func]bool,
) error {
	function, _ := analyzer.pkg.info.Defs[declaration.Name].(*types.Func)
	if function == nil || visiting[function] {
		return nil
	}
	visiting[function] = true
	defer delete(visiting, function)

	var validationErr error
	ast.Inspect(declaration.Body, func(node ast.Node) bool {
		if validationErr != nil {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		target, resolved := analyzer.resolveCallable(
			call.Fun, bindings, map[types.Object]bool{},
		)
		if !resolved {
			if analyzer.callableCouldBeMutex(call.Fun) {
				validationErr = errors.New("reachable mutex-shaped callable is unresolved")
				return false
			}
			return true
		}
		if target.cobraMethod {
			analyzer.mutexCalls++
			start := 0
			if target.skipReceiver {
				start = 1
			}
			values, ok := analyzer.resolveCallStrings(call, start, bindings)
			if !ok {
				validationErr = errors.New("cobra mutex effective flag names are unresolved")
				return false
			}
			for _, value := range values {
				if value == "yes" {
					validationErr = errors.New("--yes is incorrectly registered as a cobra mutex")
					return false
				}
			}
			return true
		}
		callee := analyzer.functions[target.function]
		if callee == nil {
			return true
		}
		next, err := analyzer.bindCall(callee, call, bindings)
		if err != nil {
			validationErr = err
			return false
		}
		if err := analyzer.inspectFunction(callee, next, visiting); err != nil {
			validationErr = err
			return false
		}
		return true
	})
	return validationErr
}

func (analyzer *s7AQMutexAnalyzer) resolveCallable(
	expression ast.Expr,
	bindings s7AQMutexBindings,
	visiting map[types.Object]bool,
) (s7AQMutexCallable, bool) {
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return analyzer.resolveCallable(value.X, bindings, visiting)
	case *ast.SelectorExpr:
		selection := analyzer.pkg.info.Selections[value]
		if selection == nil {
			return s7AQMutexCallable{}, false
		}
		function, _ := selection.Obj().(*types.Func)
		if function == nil || function.Pkg() == nil ||
			function.Pkg().Path() != "github.com/spf13/cobra" ||
			function.Name() != "MarkFlagsMutuallyExclusive" {
			return s7AQMutexCallable{}, false
		}
		return s7AQMutexCallable{
			function: function, cobraMethod: true,
			skipReceiver: selection.Kind() == types.MethodExpr,
		}, true
	case *ast.Ident:
		object := analyzer.pkg.info.ObjectOf(value)
		if object == nil || visiting[object] {
			return s7AQMutexCallable{}, false
		}
		if binding, ok := bindings.callables[object]; ok {
			return binding, true
		}
		if function, _ := object.(*types.Func); function != nil {
			return s7AQMutexCallable{function: function}, true
		}
		assigned := analyzer.assignments[object]
		if len(assigned) != 1 {
			return s7AQMutexCallable{}, false
		}
		visiting[object] = true
		defer delete(visiting, object)
		return analyzer.resolveCallable(assigned[0], bindings, visiting)
	default:
		return s7AQMutexCallable{}, false
	}
}

func (analyzer *s7AQMutexAnalyzer) bindCall(
	declaration *ast.FuncDecl,
	call *ast.CallExpr,
	caller s7AQMutexBindings,
) (s7AQMutexBindings, error) {
	result := s7AQMutexBindings{
		callables: map[types.Object]s7AQMutexCallable{},
		strings:   map[types.Object][]string{},
	}
	argumentIndex := 0
	fields := declaration.Type.Params.List
	for fieldIndex, field := range fields {
		_, variadic := field.Type.(*ast.Ellipsis)
		variadic = variadic && fieldIndex == len(fields)-1
		for _, name := range field.Names {
			object := analyzer.pkg.info.Defs[name]
			if object == nil {
				continue
			}
			if variadic {
				values, ok := analyzer.resolveCallStrings(call, argumentIndex, caller)
				if ok {
					result.strings[object] = values
				}
				argumentIndex = len(call.Args)
				continue
			}
			if argumentIndex >= len(call.Args) {
				return result, errors.New("local mutex wrapper argument binding is incomplete")
			}
			argument := call.Args[argumentIndex]
			if target, ok := analyzer.resolveCallable(
				argument, caller, map[types.Object]bool{},
			); ok {
				result.callables[object] = target
			}
			if values, ok := analyzer.resolveStringExpression(argument, caller); ok {
				result.strings[object] = values
			}
			argumentIndex++
		}
	}
	return result, nil
}

func (analyzer *s7AQMutexAnalyzer) resolveCallStrings(
	call *ast.CallExpr,
	start int,
	bindings s7AQMutexBindings,
) ([]string, bool) {
	var values []string
	for index := start; index < len(call.Args); index++ {
		resolved, ok := analyzer.resolveStringExpression(call.Args[index], bindings)
		if !ok {
			return nil, false
		}
		values = append(values, resolved...)
	}
	return values, true
}

func (analyzer *s7AQMutexAnalyzer) resolveStringExpression(
	expression ast.Expr,
	bindings s7AQMutexBindings,
) ([]string, bool) {
	if value := analyzer.pkg.info.Types[expression].Value; value != nil &&
		value.Kind() == constant.String {
		return []string{constant.StringVal(value)}, true
	}
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return analyzer.resolveStringExpression(value.X, bindings)
	case *ast.CompositeLit:
		var values []string
		for _, element := range value.Elts {
			expression, _ := element.(ast.Expr)
			resolved, ok := analyzer.resolveStringExpression(expression, bindings)
			if !ok {
				return nil, false
			}
			values = append(values, resolved...)
		}
		return values, true
	case *ast.Ident:
		object := analyzer.pkg.info.ObjectOf(value)
		if values, ok := bindings.strings[object]; ok {
			return values, true
		}
		assigned := analyzer.assignments[object]
		if len(assigned) != 1 {
			return nil, false
		}
		return analyzer.resolveStringExpression(assigned[0], bindings)
	default:
		return nil, false
	}
}

func (analyzer *s7AQMutexAnalyzer) callableCouldBeMutex(expression ast.Expr) bool {
	expressionType := analyzer.pkg.info.TypeOf(expression)
	if expressionType == nil {
		return false
	}
	signature, _ := expressionType.Underlying().(*types.Signature)
	if signature == nil || !signature.Variadic() || signature.Params().Len() != 1 ||
		signature.Results().Len() != 0 {
		return false
	}
	slice, _ := signature.Params().At(0).Type().(*types.Slice)
	if slice == nil {
		return false
	}
	basic, _ := slice.Elem().Underlying().(*types.Basic)
	return basic != nil && basic.Kind() == types.String
}
