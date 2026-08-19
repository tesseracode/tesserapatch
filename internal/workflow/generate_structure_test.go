package workflow

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestGeneratorAPIStructuralPurity(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(testFile)
	files, err := filepath.Glob(filepath.Join(dir, "generate_*.go"))
	if err != nil {
		t.Fatal(err)
	}

	var inMemoryRetryCalls int
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		count, scanErr := scanGeneratorSource(name, source)
		if scanErr != nil {
			t.Fatal(scanErr)
		}
		inMemoryRetryCalls += count
	}
	if inMemoryRetryCalls != 3 {
		t.Fatalf("expected one in-memory retry call per generator, got %d", inMemoryRetryCalls)
	}

	retrySource, err := os.ReadFile(filepath.Join(dir, "retry.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := scanPureRetryRoute("retry.go", retrySource, true); err != nil {
		t.Fatal(err)
	}

	for _, typ := range []reflect.Type{
		reflect.TypeOf(AnalysisInput{}),
		reflect.TypeOf(DefineInput{}),
		reflect.TypeOf(ExploreInput{}),
	} {
		assertPlainGeneratorInput(t, typ)
	}
	assertClosedGenNote(t)
	assertNoMapFields(t, reflect.TypeOf(AnalysisResult{}))
	assertNoMapFields(t, reflect.TypeOf(GenNote{}))
}

func TestGeneratorPurityGuardSensitivity(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "retry store field",
			source: `package workflow
func GenerateBad() { _ = RetryOptions{Store: nil} }`,
		},
		{
			name: "legacy retry route",
			source: `package workflow
func GenerateBad() { GenerateWithRetry() }`,
		},
		{
			name: "writer input",
			source: `package workflow
import "io"
type BadInput struct { Writer io.Writer }`,
		},
		{
			name: "sink callback",
			source: `package workflow
type BadInput struct { Sink func([]byte) error }`,
		},
		{
			name: "filesystem call",
			source: `package workflow
import "os"
func GenerateBad() { _ = os.WriteFile("x", nil, 0600) }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := scanGeneratorSource(tt.name+".go", []byte(tt.source)); err == nil {
				t.Fatal("purity guard accepted sensitive fixture")
			}
		})
	}
}

func TestPureRetryRouteGuardSensitivity(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "spinner",
			source: `package workflow
func GenerateWithRetryInMemory() { opts.Store = nil; opts.Slug = ""; generateWithRetry() }
func generateWithRetry() { NewSpinnerIfTTY() }`,
		},
		{
			name: "stderr",
			source: `package workflow
import "os"
func GenerateWithRetryInMemory() { opts.Store = nil; opts.Slug = ""; generateWithRetry() }
func generateWithRetry() { _ = os.Stderr }`,
		},
		{
			name: "helper call graph",
			source: `package workflow
import "os"
func GenerateWithRetryInMemory() { opts.Store = nil; opts.Slug = ""; generateWithRetry() }
func generateWithRetry() { helper.sink() }
func (T) sink() { _ = os.Stderr }`,
		},
		{
			name: "store",
			source: `package workflow
func GenerateWithRetryInMemory() { opts.Store = nil; opts.Slug = ""; generateWithRetry() }
func generateWithRetry() { _ = opts.Store }`,
		},
		{
			name: "raw artifact",
			source: `package workflow
func GenerateWithRetryInMemory() { opts.Store = nil; opts.Slug = ""; generateWithRetry() }
func generateWithRetry() { _ = "raw-response.txt" }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := scanPureRetryRoute(tt.name+".go", []byte(tt.source), false); err == nil {
				t.Fatal("pure retry route guard accepted sensitive fixture")
			}
		})
	}
}

func scanGeneratorSource(name string, source []byte) (int, error) {
	file, err := parser.ParseFile(token.NewFileSet(), name, source, 0)
	if err != nil {
		return 0, err
	}

	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return 0, err
		}
		switch path {
		case "os", "path/filepath", "io", "io/fs":
			return 0, fmt.Errorf("%s imports forbidden filesystem/writer package %q", name, path)
		}
	}

	var inMemoryRetryCalls int
	var scanErr error
	ast.Inspect(file, func(node ast.Node) bool {
		if scanErr != nil {
			return false
		}
		switch n := node.(type) {
		case *ast.TypeSpec:
			structType, ok := n.Type.(*ast.StructType)
			if !ok || !strings.HasSuffix(n.Name.Name, "Input") {
				return true
			}
			for _, field := range structType.Fields.List {
				for _, fieldName := range field.Names {
					lower := strings.ToLower(fieldName.Name)
					if strings.Contains(lower, "writer") || strings.Contains(lower, "sink") ||
						strings.Contains(lower, "store") || strings.Contains(lower, "outputpath") {
						scanErr = fmt.Errorf("%s declares sensitive input field %s", name, fieldName.Name)
						return false
					}
					if _, ok := field.Type.(*ast.FuncType); ok {
						scanErr = fmt.Errorf("%s declares callback input field %s", name, fieldName.Name)
						return false
					}
					if _, ok := field.Type.(*ast.InterfaceType); ok {
						scanErr = fmt.Errorf("%s declares interface input field %s", name, fieldName.Name)
						return false
					}
					if selector, ok := field.Type.(*ast.SelectorExpr); ok {
						pkg, _ := selector.X.(*ast.Ident)
						if pkg != nil && pkg.Name == "io" {
							scanErr = fmt.Errorf("%s declares writer input field %s", name, fieldName.Name)
							return false
						}
					}
				}
			}
		case *ast.CallExpr:
			if ident, ok := n.Fun.(*ast.Ident); ok {
				switch ident.Name {
				case "GenerateWithRetry":
					scanErr = fmt.Errorf("%s calls legacy retry route", name)
					return false
				case "generateWithRetryForAuthority":
					inMemoryRetryCalls++
				}
			}
			selector, ok := n.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, _ := selector.X.(*ast.Ident)
			if pkg != nil && (pkg.Name == "os" || pkg.Name == "filepath" || pkg.Name == "store") {
				scanErr = fmt.Errorf("%s calls forbidden %s.%s", name, pkg.Name, selector.Sel.Name)
				return false
			}
			switch selector.Sel.Name {
			case "Write", "WriteFile", "WriteArtifact", "ReadFile", "OpenFile", "Create":
				scanErr = fmt.Errorf("%s calls forbidden filesystem/writer method %s", name, selector.Sel.Name)
				return false
			}
		case *ast.CompositeLit:
			ident, ok := n.Type.(*ast.Ident)
			if !ok || ident.Name != "RetryOptions" {
				return true
			}
			for _, elt := range n.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, _ := kv.Key.(*ast.Ident)
				if key != nil && (key.Name == "Store" || key.Name == "Slug") {
					scanErr = fmt.Errorf("%s constructs RetryOptions with pure-route sink field %s", name, key.Name)
					return false
				}
			}
		}
		return true
	})
	return inMemoryRetryCalls, scanErr
}

func scanPureRetryRoute(name string, source []byte, requireLegacy bool) error {
	file, err := parser.ParseFile(token.NewFileSet(), name, source, 0)
	if err != nil {
		return err
	}

	functions := make(map[string]*ast.FuncDecl)
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			functions[fn.Name.Name] = fn
		}
	}
	entries := []string{"GenerateWithRetryInMemory"}
	if functions[entries[0]] == nil {
		return fmt.Errorf("%s lacks %s", name, entries[0])
	}
	const legacyEntry = "generateWithRetryInMemoryLegacyContext"
	if functions[legacyEntry] != nil {
		entries = append(entries, legacyEntry)
	} else if requireLegacy {
		return fmt.Errorf("%s lacks %s", name, legacyEntry)
	}

	pending := append([]string(nil), entries...)
	seen := make(map[string]bool)
	storeSelectors := 0
	slugSelectors := 0
	storeClears := 0
	slugClears := 0
	for len(pending) > 0 {
		functionName := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[functionName] {
			continue
		}
		seen[functionName] = true
		fn := functions[functionName]
		if fn == nil || fn.Body == nil {
			continue
		}

		var scanErr error
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			if scanErr != nil {
				return false
			}
			switch n := node.(type) {
			case *ast.AssignStmt:
				if functionName != "generateWithRetryInMemory" || len(n.Lhs) != 1 || len(n.Rhs) != 1 {
					return true
				}
				selector, ok := n.Lhs[0].(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch selector.Sel.Name {
				case "Store":
					value, ok := n.Rhs[0].(*ast.Ident)
					if ok && value.Name == "nil" {
						storeClears++
					}
				case "Slug":
					value, ok := n.Rhs[0].(*ast.BasicLit)
					if ok && value.Kind == token.STRING {
						unquoted, _ := strconv.Unquote(value.Value)
						if unquoted == "" {
							slugClears++
						}
					}
				}
			case *ast.CallExpr:
				if ident, ok := n.Fun.(*ast.Ident); ok {
					if ident.Name == "NewSpinnerIfTTY" {
						scanErr = fmt.Errorf("%s pure route reaches spinner in %s", name, functionName)
						return false
					}
					if functions[ident.Name] != nil {
						pending = append(pending, ident.Name)
					}
				}
				if selector, ok := n.Fun.(*ast.SelectorExpr); ok {
					if selector.Sel.Name == "WriteArtifact" {
						scanErr = fmt.Errorf("%s pure route reaches WriteArtifact in %s", name, functionName)
						return false
					}
					if functions[selector.Sel.Name] != nil {
						pending = append(pending, selector.Sel.Name)
					}
				}
			case *ast.SelectorExpr:
				pkg, _ := n.X.(*ast.Ident)
				if pkg != nil && pkg.Name == "os" && n.Sel.Name == "Stderr" {
					scanErr = fmt.Errorf("%s pure route reaches os.Stderr in %s", name, functionName)
					return false
				}
				switch n.Sel.Name {
				case "Store":
					if functionName == "generateWithRetryInMemory" {
						storeSelectors++
					} else {
						scanErr = fmt.Errorf("%s pure route reaches Store in %s", name, functionName)
						return false
					}
				case "Slug":
					if functionName == "generateWithRetryInMemory" {
						slugSelectors++
					}
				}
			case *ast.BasicLit:
				if n.Kind == token.STRING {
					value, _ := strconv.Unquote(n.Value)
					if strings.Contains(value, "raw-") {
						scanErr = fmt.Errorf("%s pure route reaches raw artifact string in %s", name, functionName)
						return false
					}
				}
			}
			return true
		})
		if scanErr != nil {
			return scanErr
		}
	}
	if storeSelectors != 1 || storeClears != 1 {
		return fmt.Errorf("%s pure routes must only clear Store once (selectors=%d clears=%d)", name, storeSelectors, storeClears)
	}
	if slugSelectors != 1 || slugClears != 1 {
		return fmt.Errorf("%s pure routes must only clear Slug once (selectors=%d clears=%d)", name, slugSelectors, slugClears)
	}
	return nil
}

func assertPlainGeneratorInput(t *testing.T, typ reflect.Type) {
	t.Helper()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		lower := strings.ToLower(field.Name)
		if strings.Contains(lower, "writer") || strings.Contains(lower, "sink") ||
			strings.Contains(lower, "store") || strings.Contains(lower, "outputpath") {
			t.Fatalf("%s has sensitive field %s", typ, field.Name)
		}
		if field.Type.Kind() == reflect.Func {
			t.Fatalf("%s has callback field %s", typ, field.Name)
		}
		if field.Type.Kind() == reflect.Interface && field.Name != "Provider" {
			t.Fatalf("%s has unexpected interface field %s", typ, field.Name)
		}
	}
}

func assertClosedGenNote(t *testing.T) {
	t.Helper()
	typ := reflect.TypeOf(GenNote{})
	want := []struct {
		name string
		kind reflect.Kind
	}{
		{"Generator", reflect.String},
		{"Advisories", reflect.Slice},
		{"ErrorClass", reflect.String},
		{"DeadlineClass", reflect.String},
		{"Attempts", reflect.Int},
		{"MaxAttempts", reflect.Int},
	}
	if typ.NumField() != len(want) {
		t.Fatalf("GenNote field count changed: want %d, got %d", len(want), typ.NumField())
	}
	for i, field := range want {
		got := typ.Field(i)
		if got.Name != field.name || got.Type.Kind() != field.kind {
			t.Fatalf("GenNote field %d: want %s/%s, got %s/%s", i, field.name, field.kind, got.Name, got.Type.Kind())
		}
	}
}

func assertNoMapFields(t *testing.T, typ reflect.Type) {
	t.Helper()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldType := field.Type
		if fieldType.Kind() == reflect.Map {
			t.Fatalf("%s has nondeterministic map field %s", typ, field.Name)
		}
		if fieldType.Kind() == reflect.Struct {
			assertNoMapFields(t, fieldType)
		}
	}
}
