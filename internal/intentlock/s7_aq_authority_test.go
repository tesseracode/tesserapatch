//go:build (linux && !android) || (darwin && !ios)

package intentlock

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/build/constraint"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestS7AQAuthorityUsesRetainedDescriptorControl(t *testing.T) {
	// PIB-504: fstatfs, flock, and unlock stay on retained-directory
	// SyscallConn.Control callbacks, and no os.File.Fd escape exists.
	sources := s7AQIntentlockProductionSources(t)
	if err := validateS7AQAuthorityControl(sources); err != nil {
		t.Fatal(err)
	}

	mutated := cloneS7AQIntentlockSources(sources)
	const rel = "statfs_linux.go"
	mutated[rel] += `
func s7AQBadDescriptorEscape(f *os.File) error {
	var stat syscall.Statfs_t
	return syscall.Fstatfs(int(f.Fd()), &stat)
}
`
	if err := validateS7AQAuthorityControl(mutated); err == nil {
		t.Fatal("PIB-504 same validator accepted syscall.Fstatfs(int(f.Fd()), ...)")
	}
	for _, sensitivity := range []struct {
		name string
		file string
		body string
	}{
		{
			name: "standalone-flock",
			file: "control_unix.go",
			body: "\nfunc s7AQStandaloneFlock() error {\n" +
				"\treturn syscall.Flock(3, syscall.LOCK_EX)\n}\n",
		},
		{
			name: "standalone-fstatfs",
			file: "statfs_linux.go",
			body: "\nfunc s7AQStandaloneFstatfs() error {\n" +
				"\tvar stat syscall.Statfs_t\n\treturn syscall.Fstatfs(3, &stat)\n}\n",
		},
		{
			name: "flock-function-alias",
			file: "control_unix.go",
			body: "\nfunc s7AQAliasedFlock() error {\n" +
				"\tflock := syscall.Flock\n\treturn flock(3, syscall.LOCK_EX)\n}\n",
		},
		{
			name: "fstatfs-function-alias",
			file: "statfs_linux.go",
			body: "\nfunc s7AQAliasedFstatfs() error {\n" +
				"\tvar stat syscall.Statfs_t\n\tfstatfs := syscall.Fstatfs\n" +
				"\treturn fstatfs(3, &stat)\n}\n",
		},
		{
			name: "flock-assignment-chain",
			file: "control_unix.go",
			body: "\nfunc s7AQChainedFlock() error {\n" +
				"\tfirst := syscall.Flock\n\tsecond := first\n" +
				"\treturn second(3, syscall.LOCK_EX)\n}\n",
		},
		{
			name: "flock-callable-parameter",
			file: "control_unix.go",
			body: "\nfunc s7AQInvokeFlock(call func(int, int) error) error {\n" +
				"\treturn call(3, syscall.LOCK_EX)\n}\n" +
				"func s7AQPassFlock() error {\n" +
				"\treturn s7AQInvokeFlock(syscall.Flock)\n}\n",
		},
		{
			name: "flock-callable-return",
			file: "control_unix.go",
			body: "\nfunc s7AQReturnFlock() func(int, int) error {\n" +
				"\treturn syscall.Flock\n}\n" +
				"func s7AQCallReturnedFlock() error {\n" +
				"\treturn s7AQReturnFlock()(3, syscall.LOCK_EX)\n}\n",
		},
		{
			name: "file-fd-method-value",
			file: "statfs_linux.go",
			body: "\nfunc s7AQFileFDMethodValue(file *os.File) uintptr {\n" +
				"\tfd := file.Fd\n\t_ = fd\n\treturn 0\n}\n",
		},
	} {
		mutated = cloneS7AQIntentlockSources(sources)
		mutated[sensitivity.file] += sensitivity.body
		if err := validateS7AQAuthorityControl(mutated); err == nil {
			t.Fatalf("PIB-504 same validator accepted %s", sensitivity.name)
		}
	}
	mutated = cloneS7AQIntentlockSources(sources)
	mutated["extra_linux.go"] = `package intentlock

import "syscall"

func s7AQNewFileFlock() error {
	return syscall.Flock(3, syscall.LOCK_EX)
}
`
	if err := validateS7AQAuthorityControl(mutated); err == nil {
		t.Fatal("PIB-504 same validator accepted a syscall in a new production file")
	}
	mutated = cloneS7AQIntentlockSources(sources)
	mutated["extra_freebsd.go"] = `package intentlock

import "syscall"

func s7AQFreeBSDFlock() error {
	flock := syscall.Flock
	return flock(3, syscall.LOCK_EX)
}
`
	if err := validateS7AQAuthorityControl(mutated); err == nil {
		t.Fatal("PIB-504 same validator accepted a FreeBSD-only syscall")
	}
}

func s7AQIntentlockProductionSources(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		sources[name] = string(data)
	}
	return sources
}

func cloneS7AQIntentlockSources(source map[string]string) map[string]string {
	clone := make(map[string]string, len(source))
	for name, content := range source {
		clone[name] = content
	}
	return clone
}

func validateS7AQAuthorityControl(sources map[string]string) error {
	for _, required := range []string{
		"control_unix.go", "statfs_linux.go", "statfs_darwin.go",
	} {
		if sources[required] == "" {
			return fmt.Errorf("authority source %s is missing", required)
		}
	}
	for _, name := range []string{"statfs_linux.go", "statfs_darwin.go"} {
		if err := validateS7APHeldFstatfsSource(name, sources[name]); err != nil {
			return fmt.Errorf("%s retained-descriptor path: %w", name, err)
		}
	}
	if err := validateS7AQFlockControlSource(sources["control_unix.go"]); err != nil {
		return err
	}
	targets, err := s7AQAuthorityTargets()
	if err != nil {
		return err
	}
	covered := map[string]bool{}
	union := map[string]bool{}
	for _, goos := range targets {
		files, calls, err := validateS7AQAuthoritySyscallInventory(sources, goos)
		if err != nil {
			return fmt.Errorf("GOOS=%s authority syscall inventory: %w", goos, err)
		}
		for _, name := range files {
			covered[name] = true
		}
		for key := range calls {
			union[key] = true
		}
	}
	var missing []string
	for name := range sources {
		if !covered[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) != 0 {
		return fmt.Errorf("production sources skipped by every build target: %v", missing)
	}
	wantUnion := map[string]bool{
		"control_unix.go:flockThroughControl:Flock":       true,
		"statfs_darwin.go:classifyHeldFilesystem:Fstatfs": true,
		"statfs_linux.go:classifyHeldFilesystem:Fstatfs":  true,
	}
	if !reflect.DeepEqual(union, wantUnion) {
		return fmt.Errorf("package-wide syscall union = %v, want %v", union, wantUnion)
	}
	return nil
}

func validateS7AQAuthoritySyscallInventory(
	sources map[string]string,
	goos string,
) ([]string, map[string]int, error) {
	fileset := token.NewFileSet()
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	var files []*ast.File
	fileNames := map[*ast.File]string{}
	var selected []string
	needsTargetExports := false
	for _, name := range names {
		matches, err := s7AQAuthorityFileMatches(name, sources[name], goos)
		if err != nil {
			return nil, nil, err
		}
		if !matches {
			continue
		}
		file, err := parser.ParseFile(fileset, name, sources[name], 0)
		if err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", name, err)
		}
		files = append(files, file)
		fileNames[file] = name
		selected = append(selected, name)
		ast.Inspect(file, func(node ast.Node) bool {
			selector, _ := node.(*ast.SelectorExpr)
			if selector == nil {
				return true
			}
			packageName, _ := selector.X.(*ast.Ident)
			if selector.Sel.Name == "Fd" ||
				packageName != nil && packageName.Name == "syscall" &&
					(selector.Sel.Name == "Flock" || selector.Sel.Name == "Fstatfs") {
				needsTargetExports = true
			}
			return true
		})
	}
	info := &types.Info{
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Types:      map[ast.Expr]types.TypeAndValue{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	var targetImporter types.Importer
	var err error
	if needsTargetExports {
		targetImporter, err = s7APTargetImporter(goos, fileset)
		if err != nil {
			return nil, nil, err
		}
	} else {
		targetImporter = importer.Default()
	}
	config := types.Config{Importer: targetImporter}
	if _, err := config.Check(
		"github.com/tesseracode/tesserapatch/internal/intentlock",
		fileset, files, info,
	); err != nil {
		return nil, nil, fmt.Errorf("type-check target package: %w", err)
	}
	want := map[string]int{}
	if goos == "linux" || goos == "darwin" {
		want = map[string]int{
			"control_unix.go:flockThroughControl:Flock":             1,
			"statfs_" + goos + ".go:classifyHeldFilesystem:Fstatfs": 1,
		}
	}
	analyzer := newS7AQAuthorityCallableAnalyzer(fileset, files, fileNames, info)
	got, err := analyzer.validate()
	if err != nil {
		return nil, nil, err
	}
	if !reflect.DeepEqual(got, want) {
		return nil, nil, fmt.Errorf("typed syscall inventory = %v, want %v", got, want)
	}
	return selected, got, nil
}

func s7AQAuthorityTargets() ([]string, error) {
	command := exec.Command("go", "tool", "dist", "list")
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("go tool dist list: %w", err)
	}
	set := map[string]bool{}
	for _, target := range strings.Fields(string(output)) {
		goos, _, ok := strings.Cut(target, "/")
		if ok {
			set[goos] = true
		}
	}
	var targets []string
	for goos := range set {
		targets = append(targets, goos)
	}
	sort.Strings(targets)
	for _, required := range []string{
		"android", "darwin", "dragonfly", "freebsd", "ios", "linux",
		"netbsd", "openbsd", "windows",
	} {
		if !set[required] {
			return nil, fmt.Errorf("GOOS target matrix omits %s", required)
		}
	}
	return targets, nil
}

func s7AQAuthorityFileMatches(name, source, goos string) (bool, error) {
	if _, err := os.Stat(name); err == nil {
		context := build.Default
		context.GOOS = goos
		context.GOARCH = runtime.GOARCH
		context.CgoEnabled = false
		return context.MatchFile(".", name)
	} else if !os.IsNotExist(err) {
		return false, err
	}
	base := strings.TrimSuffix(filepath.Base(name), ".go")
	for _, target := range []string{
		"aix", "android", "darwin", "dragonfly", "freebsd", "illumos", "ios",
		"js", "linux", "netbsd", "openbsd", "plan9", "solaris", "wasip1", "windows",
	} {
		if strings.HasSuffix(base, "_"+target) {
			if target != goos {
				return false, nil
			}
			break
		}
	}
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "//go:build ") {
			expression, err := constraint.Parse(line)
			if err != nil {
				return false, err
			}
			return expression.Eval(func(tag string) bool {
				switch tag {
				case goos, runtime.GOARCH, "gc":
					return true
				case "unix":
					return s7AQUnixGOOS(goos)
				case "cgo":
					return false
				default:
					return false
				}
			}), nil
		}
		if line != "" && !strings.HasPrefix(line, "//") {
			break
		}
	}
	return true, nil
}

func s7AQUnixGOOS(goos string) bool {
	switch goos {
	case "aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos",
		"ios", "linux", "netbsd", "openbsd", "solaris":
		return true
	default:
		return false
	}
}

func s7AQAuthorityParents(root ast.Node) map[ast.Node]ast.Node {
	parents := map[ast.Node]ast.Node{}
	var stack []ast.Node
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if len(stack) != 0 {
			parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	return parents
}

type s7AQAuthorityCallable struct {
	function *types.Func
	literal  *ast.FuncLit
	origins  map[token.Pos]bool
}

type s7AQAuthorityCallableAnalyzer struct {
	fileset       *token.FileSet
	files         []*ast.File
	fileNames     map[*ast.File]string
	info          *types.Info
	functions     map[*types.Func]*ast.FuncDecl
	assignments   map[types.Object][]ast.Expr
	parents       map[*ast.File]map[ast.Node]ast.Node
	selectorKinds map[token.Pos]string
	usedSelectors map[token.Pos]bool
	observedCalls map[token.Pos]string
}

func newS7AQAuthorityCallableAnalyzer(
	fileset *token.FileSet,
	files []*ast.File,
	fileNames map[*ast.File]string,
	info *types.Info,
) *s7AQAuthorityCallableAnalyzer {
	analyzer := &s7AQAuthorityCallableAnalyzer{
		fileset: fileset, files: files, fileNames: fileNames, info: info,
		functions:     map[*types.Func]*ast.FuncDecl{},
		assignments:   map[types.Object][]ast.Expr{},
		parents:       map[*ast.File]map[ast.Node]ast.Node{},
		selectorKinds: map[token.Pos]string{},
		usedSelectors: map[token.Pos]bool{},
		observedCalls: map[token.Pos]string{},
	}
	for _, file := range files {
		analyzer.parents[file] = s7AQAuthorityParents(file)
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			if object, _ := info.Defs[function.Name].(*types.Func); object != nil {
				analyzer.functions[object] = function
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.AssignStmt:
				if len(value.Lhs) != len(value.Rhs) {
					return true
				}
				for index, left := range value.Lhs {
					identifier, _ := left.(*ast.Ident)
					if identifier == nil || identifier.Name == "_" {
						continue
					}
					object := info.Defs[identifier]
					if object == nil {
						object = info.Uses[identifier]
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
					if object := info.Defs[name]; object != nil {
						analyzer.assignments[object] = append(
							analyzer.assignments[object], value.Values[index],
						)
					}
				}
			case *ast.SelectorExpr:
				if s7AQTypedFileFdSelector(value, info) {
					analyzer.selectorKinds[value.Pos()] = "Fd"
				} else if name := s7AQTypedAuthorityFunction(value, info); name != "" {
					analyzer.selectorKinds[value.Pos()] = name
				}
			}
			return true
		})
	}
	return analyzer
}

func (analyzer *s7AQAuthorityCallableAnalyzer) validate() (map[string]int, error) {
	for position, kind := range analyzer.selectorKinds {
		if kind == "Fd" {
			return nil, fmt.Errorf("package contains typed (*os.File).Fd selection at %s",
				analyzer.fileset.Position(position))
		}
	}
	got := map[string]int{}
	for _, file := range analyzer.files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			if err := analyzer.inspectBlock(
				file, function.Body, map[types.Object]s7AQAuthorityCallable{},
				map[string]bool{}, got,
			); err != nil {
				return nil, err
			}
		}
	}
	var unused []string
	for position, kind := range analyzer.selectorKinds {
		if kind != "Fd" && !analyzer.usedSelectors[position] {
			unused = append(unused,
				kind+"@"+analyzer.fileset.Position(position).String())
		}
	}
	sort.Strings(unused)
	if len(unused) != 0 {
		return nil, fmt.Errorf("unconsumed authority syscall selections: %v", unused)
	}
	return got, nil
}

func (analyzer *s7AQAuthorityCallableAnalyzer) inspectBlock(
	file *ast.File,
	body *ast.BlockStmt,
	bindings map[types.Object]s7AQAuthorityCallable,
	visiting map[string]bool,
	got map[string]int,
) error {
	var validationErr error
	ast.Inspect(body, func(node ast.Node) bool {
		if validationErr != nil {
			return false
		}
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		target, resolved := analyzer.resolveCallable(
			call.Fun, bindings, map[types.Object]bool{},
		)
		if resolved {
			if syscallName := s7AQAuthorityCallableName(target); syscallName != "" {
				for origin := range target.origins {
					analyzer.usedSelectors[origin] = true
				}
				if previous := analyzer.observedCalls[call.Pos()]; previous != "" &&
					previous != syscallName {
					validationErr = fmt.Errorf(
						"authority call at %s resolves to both %s and %s",
						analyzer.fileset.Position(call.Pos()), previous, syscallName,
					)
					return false
				}
				if analyzer.observedCalls[call.Pos()] == "" {
					analyzer.observedCalls[call.Pos()] = syscallName
					function := s7AQEnclosingAuthorityFunction(
						call, analyzer.parents[file],
					)
					if function == nil {
						validationErr = fmt.Errorf(
							"%s %s is outside a named authority function",
							analyzer.fileNames[file], syscallName,
						)
						return false
					}
					key := analyzer.fileNames[file] + ":" +
						function.Name.Name + ":" + syscallName
					got[key]++
					if !s7AQSyscallUsesControlDescriptor(
						call, analyzer.parents[file], analyzer.info,
					) {
						validationErr = fmt.Errorf(
							"%s is not directly inside Control with its exact descriptor",
							key,
						)
						return false
					}
				}
				return true
			}
			next := analyzer.bindCall(target, call, bindings)
			switch {
			case target.function != nil && analyzer.functions[target.function] != nil:
				key := target.function.FullName()
				if !visiting[key] {
					visiting[key] = true
					targetFile := analyzer.functionFile(target.function)
					validationErr = analyzer.inspectBlock(
						targetFile, analyzer.functions[target.function].Body,
						next, visiting, got,
					)
					delete(visiting, key)
				}
			case target.literal != nil:
				validationErr = analyzer.inspectBlock(
					file, target.literal.Body, next, visiting, got,
				)
			default:
				validationErr = analyzer.rejectExternalCallableArguments(
					file, call, bindings, visiting, got,
				)
			}
		} else {
			validationErr = analyzer.rejectExternalCallableArguments(
				file, call, bindings, visiting, got,
			)
		}
		return validationErr == nil
	})
	return validationErr
}

func (analyzer *s7AQAuthorityCallableAnalyzer) rejectExternalCallableArguments(
	file *ast.File,
	call *ast.CallExpr,
	bindings map[types.Object]s7AQAuthorityCallable,
	visiting map[string]bool,
	got map[string]int,
) error {
	for _, argument := range call.Args {
		target, resolved := analyzer.resolveCallable(
			argument, bindings, map[types.Object]bool{},
		)
		if resolved && s7AQAuthorityCallableName(target) != "" {
			return fmt.Errorf(
				"authority syscall callable escapes through unresolved/external call %s",
				analyzer.fileset.Position(call.Pos()),
			)
		}
		if !resolved {
			continue
		}
		switch {
		case target.function != nil && analyzer.functions[target.function] != nil:
			key := target.function.FullName()
			if visiting[key] {
				continue
			}
			visiting[key] = true
			targetFile := analyzer.functionFile(target.function)
			err := analyzer.inspectBlock(
				targetFile, analyzer.functions[target.function].Body,
				map[types.Object]s7AQAuthorityCallable{}, visiting, got,
			)
			delete(visiting, key)
			if err != nil {
				return err
			}
		case target.literal != nil:
			if err := analyzer.inspectBlock(
				file, target.literal.Body,
				map[types.Object]s7AQAuthorityCallable{}, visiting, got,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func (analyzer *s7AQAuthorityCallableAnalyzer) resolveCallable(
	expression ast.Expr,
	bindings map[types.Object]s7AQAuthorityCallable,
	visiting map[types.Object]bool,
) (s7AQAuthorityCallable, bool) {
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return analyzer.resolveCallable(value.X, bindings, visiting)
	case *ast.FuncLit:
		return s7AQAuthorityCallable{literal: value}, true
	case *ast.SelectorExpr:
		function := s7AQSelectorFunction(value, analyzer.info)
		if function == nil {
			return s7AQAuthorityCallable{}, false
		}
		target := s7AQAuthorityCallable{function: function}
		if s7AQAuthorityFunctionName(function) != "" {
			target.origins = map[token.Pos]bool{value.Pos(): true}
		}
		return target, true
	case *ast.Ident:
		object := analyzer.info.ObjectOf(value)
		if function, _ := object.(*types.Func); function != nil {
			return s7AQAuthorityCallable{function: function}, true
		}
		if object == nil || visiting[object] {
			return s7AQAuthorityCallable{}, false
		}
		if bound, ok := bindings[object]; ok {
			return bound, true
		}
		expressions := analyzer.assignments[object]
		if len(expressions) == 0 {
			return s7AQAuthorityCallable{}, false
		}
		visiting[object] = true
		defer delete(visiting, object)
		var result s7AQAuthorityCallable
		for _, assigned := range expressions {
			target, resolved := analyzer.resolveCallable(assigned, bindings, visiting)
			if !resolved || !s7AQMergeAuthorityCallable(&result, target) {
				return s7AQAuthorityCallable{}, false
			}
		}
		return result, true
	case *ast.CallExpr:
		factory, resolved := analyzer.resolveCallable(
			value.Fun, bindings, map[types.Object]bool{},
		)
		if !resolved || factory.function == nil {
			return s7AQAuthorityCallable{}, false
		}
		declaration := analyzer.functions[factory.function]
		if declaration == nil {
			return s7AQAuthorityCallable{}, false
		}
		next := analyzer.bindCall(factory, value, bindings)
		var result s7AQAuthorityCallable
		found := false
		valid := true
		ast.Inspect(declaration.Body, func(node ast.Node) bool {
			if !valid {
				return false
			}
			if _, nested := node.(*ast.FuncLit); nested {
				return false
			}
			statement, ok := node.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			if len(statement.Results) != 1 {
				valid = false
				return false
			}
			target, ok := analyzer.resolveCallable(
				statement.Results[0], next, map[types.Object]bool{},
			)
			if !ok || !s7AQMergeAuthorityCallable(&result, target) {
				valid = false
				return false
			}
			found = true
			return true
		})
		return result, found && valid
	default:
		return s7AQAuthorityCallable{}, false
	}
}

func (analyzer *s7AQAuthorityCallableAnalyzer) bindCall(
	target s7AQAuthorityCallable,
	call *ast.CallExpr,
	caller map[types.Object]s7AQAuthorityCallable,
) map[types.Object]s7AQAuthorityCallable {
	result := map[types.Object]s7AQAuthorityCallable{}
	var parameters *ast.FieldList
	if target.function != nil {
		if declaration := analyzer.functions[target.function]; declaration != nil {
			parameters = declaration.Type.Params
		}
	} else if target.literal != nil {
		parameters = target.literal.Type.Params
	}
	if parameters == nil {
		return result
	}
	argument := 0
	for _, field := range parameters.List {
		for _, name := range field.Names {
			if argument >= len(call.Args) {
				return result
			}
			object := analyzer.info.Defs[name]
			if callable, ok := analyzer.resolveCallable(
				call.Args[argument], caller, map[types.Object]bool{},
			); ok && object != nil {
				result[object] = callable
			}
			argument++
		}
	}
	return result
}

func (analyzer *s7AQAuthorityCallableAnalyzer) functionFile(
	target *types.Func,
) *ast.File {
	declaration := analyzer.functions[target]
	for _, file := range analyzer.files {
		for _, candidate := range file.Decls {
			if candidate == declaration {
				return file
			}
		}
	}
	return nil
}

func s7AQMergeAuthorityCallable(
	current *s7AQAuthorityCallable,
	next s7AQAuthorityCallable,
) bool {
	if current.function == nil && current.literal == nil {
		*current = next
		return true
	}
	if current.function != next.function || current.literal != next.literal {
		return false
	}
	if current.origins == nil {
		current.origins = map[token.Pos]bool{}
	}
	for origin := range next.origins {
		current.origins[origin] = true
	}
	return true
}

func s7AQAuthorityCallableName(target s7AQAuthorityCallable) string {
	return s7AQAuthorityFunctionName(target.function)
}

func s7AQAuthorityFunctionName(function *types.Func) string {
	if function == nil || function.Pkg() == nil ||
		function.Pkg().Path() != "syscall" {
		return ""
	}
	switch function.Name() {
	case "Flock", "Fstatfs":
		return function.Name()
	default:
		return ""
	}
}

func s7AQSelectorFunction(
	selector *ast.SelectorExpr,
	info *types.Info,
) *types.Func {
	if selector == nil {
		return nil
	}
	if selection := info.Selections[selector]; selection != nil {
		function, _ := selection.Obj().(*types.Func)
		return function
	}
	function, _ := info.Uses[selector.Sel].(*types.Func)
	return function
}

func s7AQTypedAuthorityFunction(
	selector *ast.SelectorExpr,
	info *types.Info,
) string {
	return s7AQAuthorityFunctionName(s7AQSelectorFunction(selector, info))
}

func s7AQTypedFileFdSelector(
	selector *ast.SelectorExpr,
	info *types.Info,
) bool {
	function := s7AQSelectorFunction(selector, info)
	return function != nil && function.Pkg() != nil &&
		function.Pkg().Path() == "os" && function.Name() == "Fd"
}

func s7AQEnclosingAuthorityFunction(
	node ast.Node,
	parents map[ast.Node]ast.Node,
) *ast.FuncDecl {
	for current := parents[node]; current != nil; current = parents[current] {
		if function, ok := current.(*ast.FuncDecl); ok {
			return function
		}
	}
	return nil
}

func s7AQSyscallUsesControlDescriptor(
	call *ast.CallExpr,
	parents map[ast.Node]ast.Node,
	info *types.Info,
) bool {
	var callback *ast.FuncLit
	for current := parents[call]; current != nil; current = parents[current] {
		if _, nested := current.(*ast.FuncDecl); nested {
			break
		}
		if function, ok := current.(*ast.FuncLit); ok {
			callback = function
			break
		}
	}
	if callback == nil || callback.Type.Params == nil ||
		len(callback.Type.Params.List) != 1 ||
		len(callback.Type.Params.List[0].Names) != 1 {
		return false
	}
	controlCall, _ := parents[callback].(*ast.CallExpr)
	if controlCall == nil || len(controlCall.Args) != 1 || controlCall.Args[0] != callback {
		return false
	}
	selector, _ := controlCall.Fun.(*ast.SelectorExpr)
	selection := info.Selections[selector]
	if selection == nil || selection.Obj().Pkg() == nil ||
		selection.Obj().Pkg().Path() != "syscall" ||
		selection.Obj().Name() != "Control" {
		return false
	}
	descriptor := info.Defs[callback.Type.Params.List[0].Names[0]]
	return len(call.Args) != 0 &&
		s7AQExactBuiltinInt(call.Args[0], descriptor, info)
}

func validateS7AQFlockControlSource(source string) error {
	fileset := token.NewFileSet()
	file, err := parser.ParseFile(fileset, "control_unix.go", source, 0)
	if err != nil {
		return err
	}
	info := &types.Info{
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Types:      map[ast.Expr]types.TypeAndValue{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	config := types.Config{Importer: mustS7AQImporter()}
	if _, err := config.Check("s7/intentlock", fileset, []*ast.File{file}, info); err != nil {
		return fmt.Errorf("type-check control_unix.go: %w", err)
	}
	functions := map[string]*ast.FuncDecl{}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok {
			functions[function.Name.Name] = function
		}
	}
	if err := validateS7AQLockWrapper(
		functions["lockHeldDirectory"], "syscall.LOCK_EX|syscall.LOCK_NB", source,
	); err != nil {
		return fmt.Errorf("lock wrapper: %w", err)
	}
	if err := validateS7AQLockWrapper(
		functions["unlockHeldDirectory"], "syscall.LOCK_UN", source,
	); err != nil {
		return fmt.Errorf("unlock wrapper: %w", err)
	}
	function := functions["flockThroughControl"]
	if function == nil || function.Body == nil || len(function.Body.List) != 5 {
		return errors.New("flockThroughControl must retain the canonical five-statement body")
	}
	fileObject := s7AQParameterObject(function, "file", info)
	operationObject := s7AQParameterObject(function, "operation", info)
	if fileObject == nil || operationObject == nil {
		return errors.New("flockThroughControl parameters are unresolved")
	}

	assignRaw, ok := function.Body.List[0].(*ast.AssignStmt)
	if !ok || assignRaw.Tok != token.DEFINE || len(assignRaw.Lhs) != 2 ||
		len(assignRaw.Rhs) != 1 {
		return errors.New("SyscallConn assignment shape changed")
	}
	rawObject := s7AQDefinedObject(assignRaw.Lhs[0], info)
	errObject := s7AQDefinedObject(assignRaw.Lhs[1], info)
	syscallConn, ok := assignRaw.Rhs[0].(*ast.CallExpr)
	if rawObject == nil || errObject == nil || !ok || len(syscallConn.Args) != 0 ||
		!s7AQExactMethodCall(syscallConn.Fun, fileObject, "os", "SyscallConn", info) {
		return errors.New("retained file must define raw, err through exact SyscallConn()")
	}
	if !s7AQExactErrorGuard(function.Body.List[1], errObject, info) {
		return errors.New("SyscallConn error lacks its immediate canonical guard")
	}

	flockDeclaration, ok := function.Body.List[2].(*ast.DeclStmt)
	if !ok {
		return errors.New("flockErr declaration moved or changed")
	}
	flockObject := s7AQDeclaredObject(flockDeclaration, "flockErr", info)
	if flockObject == nil {
		return errors.New("flockErr local is unresolved")
	}
	controlGuard, ok := function.Body.List[3].(*ast.IfStmt)
	if !ok || controlGuard.Init == nil {
		return errors.New("Control call lacks the canonical same-error guard")
	}
	controlAssign, ok := controlGuard.Init.(*ast.AssignStmt)
	if !ok || controlAssign.Tok != token.DEFINE || len(controlAssign.Lhs) != 1 ||
		len(controlAssign.Rhs) != 1 {
		return errors.New("Control result must define its canonical error object")
	}
	controlErrObject := s7AQDefinedObject(controlAssign.Lhs[0], info)
	if controlErrObject == nil ||
		!s7AQExactErrorGuard(controlGuard, controlErrObject, info) {
		return errors.New("Control call lacks its matching canonical error guard")
	}
	controlCall, ok := controlAssign.Rhs[0].(*ast.CallExpr)
	if !ok || len(controlCall.Args) != 1 ||
		!s7AQExactMethodCall(controlCall.Fun, rawObject, "syscall", "Control", info) {
		return errors.New("flock must execute through exact RawConn.Control")
	}
	callback, ok := controlCall.Args[0].(*ast.FuncLit)
	if !ok || callback.Body == nil || len(callback.Body.List) != 1 ||
		callback.Type.Params == nil || len(callback.Type.Params.List) != 1 {
		return errors.New("Control callback shape changed")
	}
	descriptor := callback.Type.Params.List[0].Names[0]
	descriptorObject := info.Defs[descriptor]
	flockAssign, ok := callback.Body.List[0].(*ast.AssignStmt)
	if !ok || flockAssign.Tok != token.ASSIGN || len(flockAssign.Lhs) != 1 ||
		len(flockAssign.Rhs) != 1 || info.Uses[s7AQIdent(flockAssign.Lhs[0])] != flockObject {
		return errors.New("Control callback must assign flockErr exactly once")
	}
	flockCall, ok := flockAssign.Rhs[0].(*ast.CallExpr)
	if !ok || len(flockCall.Args) != 2 {
		return errors.New("Control callback Flock call shape changed")
	}
	exactFunction := s7AQExactPackageFunction(flockCall.Fun, "syscall", "Flock", info)
	exactDescriptor := s7AQExactBuiltinInt(flockCall.Args[0], descriptorObject, info)
	exactOperation := info.Uses[s7AQIdent(flockCall.Args[1])] == operationObject
	if !exactFunction || !exactDescriptor || !exactOperation {
		return fmt.Errorf(
			"Control callback must call syscall.Flock(int(descriptor), operation): function=%t descriptor=%t operation=%t",
			exactFunction, exactDescriptor, exactOperation,
		)
	}
	returnStatement, ok := function.Body.List[4].(*ast.ReturnStmt)
	if !ok || len(returnStatement.Results) != 1 ||
		info.Uses[s7AQIdent(returnStatement.Results[0])] != flockObject {
		return errors.New("flockThroughControl must return the callback flockErr unchanged")
	}
	return nil
}

func mustS7AQImporter() types.Importer {
	importer, err := s7APTargetImporter("darwin", token.NewFileSet())
	if err != nil {
		panic(err)
	}
	return importer
}

func validateS7AQLockWrapper(function *ast.FuncDecl, operation, source string) error {
	if function == nil || function.Body == nil || len(function.Body.List) != 1 {
		return errors.New("wrapper must have one return statement")
	}
	returnStatement, ok := function.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(returnStatement.Results) != 1 {
		return errors.New("wrapper must return the control helper")
	}
	call, ok := returnStatement.Results[0].(*ast.CallExpr)
	if !ok || len(call.Args) != 2 {
		return errors.New("wrapper control call shape changed")
	}
	identifier, ok := call.Fun.(*ast.Ident)
	if !ok || identifier.Name != "flockThroughControl" ||
		strings.ReplaceAll(s7AQNodeText(source, call.Args[0]), " ", "") != "file" ||
		strings.ReplaceAll(s7AQNodeText(source, call.Args[1]), " ", "") != operation {
		return fmt.Errorf("wrapper does not preserve file and %s", operation)
	}
	return nil
}

func s7AQExactErrorGuard(statement ast.Stmt, object types.Object, info *types.Info) bool {
	guard, ok := statement.(*ast.IfStmt)
	if !ok || guard.Cond == nil || guard.Body == nil || len(guard.Body.List) != 1 {
		return false
	}
	condition, ok := guard.Cond.(*ast.BinaryExpr)
	if !ok || condition.Op != token.NEQ ||
		info.Uses[s7AQIdent(condition.X)] != object ||
		!s7AQIsNil(condition.Y) {
		return false
	}
	returnStatement, ok := guard.Body.List[0].(*ast.ReturnStmt)
	return ok && len(returnStatement.Results) == 1 &&
		info.Uses[s7AQIdent(returnStatement.Results[0])] == object
}

func s7AQExactMethodCall(
	expression ast.Expr,
	receiver types.Object,
	pkgName, methodName string,
	info *types.Info,
) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != methodName ||
		info.Uses[s7AQIdent(selector.X)] != receiver {
		return false
	}
	selection := info.Selections[selector]
	return selection != nil && selection.Obj().Pkg() != nil &&
		selection.Obj().Pkg().Name() == pkgName && selection.Obj().Name() == methodName
}

func s7AQExactPackageFunction(
	expression ast.Expr, pkgName, functionName string, info *types.Info,
) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != functionName {
		return false
	}
	object := info.Uses[selector.Sel]
	return object != nil && object.Pkg() != nil &&
		object.Pkg().Name() == pkgName && object.Name() == functionName
}

func s7AQExactBuiltinInt(expression ast.Expr, argument types.Object, info *types.Info) bool {
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || info.Uses[s7AQIdent(call.Args[0])] != argument {
		return false
	}
	identifier, ok := call.Fun.(*ast.Ident)
	if !ok || identifier.Name != "int" {
		return false
	}
	return info.Uses[identifier] == types.Universe.Lookup("int")
}

func s7AQParameterObject(function *ast.FuncDecl, name string, info *types.Info) types.Object {
	for _, field := range function.Type.Params.List {
		for _, identifier := range field.Names {
			if identifier.Name == name {
				return info.Defs[identifier]
			}
		}
	}
	return nil
}

func s7AQDefinedObject(expression ast.Expr, info *types.Info) types.Object {
	identifier := s7AQIdent(expression)
	if identifier == nil {
		return nil
	}
	if object := info.Defs[identifier]; object != nil {
		return object
	}
	return info.Uses[identifier]
}

func s7AQDeclaredObject(
	statement *ast.DeclStmt, name string, info *types.Info,
) types.Object {
	declaration, ok := statement.Decl.(*ast.GenDecl)
	if !ok {
		return nil
	}
	for _, specification := range declaration.Specs {
		value, ok := specification.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, identifier := range value.Names {
			if identifier.Name == name {
				return info.Defs[identifier]
			}
		}
	}
	return nil
}

func s7AQIdent(expression ast.Expr) *ast.Ident {
	for {
		parenthesized, ok := expression.(*ast.ParenExpr)
		if !ok {
			break
		}
		expression = parenthesized.X
	}
	identifier, _ := expression.(*ast.Ident)
	return identifier
}

func s7AQIsNil(expression ast.Expr) bool {
	identifier, ok := expression.(*ast.Ident)
	return ok && identifier.Name == "nil"
}

func s7AQNodeText(source string, node ast.Node) string {
	var output bytes.Buffer
	if err := formatNode(&output, node); err != nil {
		return ""
	}
	return output.String()
}

func formatNode(output *bytes.Buffer, node ast.Node) error {
	return format.Node(output, token.NewFileSet(), node)
}
