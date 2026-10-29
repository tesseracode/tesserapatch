package cli

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"sync"
	"testing"
)

// Only independent, nonparallel fixtures may share this scope. Release all
// coupled memos after four completed fixtures, never during an active analysis.
// Immutable baseline/import caches outlive the scope.
func s6SerialAnalyzerMemoScope(t *testing.T) func(*testing.T) {
	t.Helper()
	completed := s6SerialAnalyzerMemoBatch(t)
	return func(fixture *testing.T) {
		fixture.Helper()
		fixture.Cleanup(completed)
	}
}

// Flat fixture loops call completed only after all analysis/assertions finish.
func s6SerialAnalyzerMemoBatch(t *testing.T) func() {
	t.Helper()
	s6ClearAnalyzerMemoization()
	t.Cleanup(s6ClearAnalyzerMemoization)
	completed := 0
	return func() {
		completed++
		if completed == 4 {
			s6ClearAnalyzerMemoization()
			completed = 0
		}
	}
}

func s6ClearAnalyzerMemoization() {
	s6ParsedSourceCache.Clear()
	s6TypeModelCache.Clear()
	s6TypeGraphCache.Clear()
	s6BindingCache.Clear()
	s6MutationSummaryCache.Clear()
	s6GlobalEffectCache.Clear()
	s6GlobalInitCache.Clear()
	s6RangeIndexCache.Clear()
	s6RangeSelectorCache.Clear()
	s6RangeValueCache.Clear()
	s6IndexedEntryCache.Clear()
	s6FrozenAppendCache.Clear()
	s6FrozenSliceCache.Clear()
	s6ZeroMapElementCache.Clear()
	s6StableExprKeyCache.Clear()
}

func validateS6AnalyzerMemoInventory(declarations, cleanup string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "declarations.go", declarations, 0)
	if err != nil {
		return err
	}
	expected := map[string]bool{}
	for _, declaration := range file.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok || group.Tok != token.VAR {
			continue
		}
		for _, item := range group.Specs {
			value, ok := item.(*ast.ValueSpec)
			if !ok {
				continue
			}
			selector, ok := value.Type.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Map" {
				continue
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || pkg.Name != "sync" {
				continue
			}
			for _, name := range value.Names {
				if name.Name != "s6ExportPathCache" {
					expected[name.Name] = true
				}
			}
		}
	}
	if len(expected) == 0 {
		return fmt.Errorf("no analyzer memo declarations found")
	}
	file, err = parser.ParseFile(token.NewFileSet(), "cleanup.go", cleanup, 0)
	if err != nil {
		return err
	}
	var body *ast.BlockStmt
	for _, declaration := range file.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok &&
			function.Name.Name == "s6ClearAnalyzerMemoization" {
			body = function.Body
		}
	}
	if body == nil || len(body.List) != len(expected) {
		return fmt.Errorf("cleanup must clear every transient memo exactly once")
	}
	for _, statement := range body.List {
		expression, ok := statement.(*ast.ExprStmt)
		if !ok {
			return fmt.Errorf("cleanup contains a non-clear operation")
		}
		call, ok := expression.X.(*ast.CallExpr)
		if !ok || len(call.Args) != 0 {
			return fmt.Errorf("cleanup contains a non-clear call")
		}
		method, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || method.Sel.Name != "Clear" {
			return fmt.Errorf("cleanup contains a non-clear method")
		}
		cache, ok := method.X.(*ast.Ident)
		if !ok || !expected[cache.Name] {
			return fmt.Errorf("cleanup clears an unknown, immutable or duplicate cache")
		}
		delete(expected, cache.Name)
	}
	return nil
}

func TestS6AnalyzerMemoScopeInventory(t *testing.T) {
	declarations := s6RepoFile(t, "internal/cli/prepare_s6_contract_test.go")
	cleanup := s6RepoFile(t, "internal/cli/prepare_s6_cache_scope_test.go")
	if err := validateS6AnalyzerMemoInventory(declarations, cleanup); err != nil {
		t.Fatal(err)
	}
	const anchor = "\ts6BindingCache.Clear()\n"
	if strings.Count(cleanup, anchor) != 1 {
		t.Fatal("cleanup mutation anchor is not unique")
	}
	for _, replacement := range []string{
		"",
		"\ts6TypeModelCache.Clear()\n",
		"\ts6ExportPathCache.Clear()\n",
		"\ts6ContractBaselineOnce = sync.Once{}\n",
	} {
		mutated := strings.Replace(cleanup, anchor, replacement, 1)
		if err := validateS6AnalyzerMemoInventory(declarations, mutated); err == nil {
			t.Fatalf("cleanup guard accepted mutation %q", replacement)
		}
	}
	if err := validateS6AnalyzerMemoInventory(
		declarations+"\nvar s6UnregisteredMemo sync.Map\n", cleanup,
	); err == nil {
		t.Fatal("cleanup guard accepted a newly unowned analyzer cache")
	}
}

func TestS6AnalyzerMemoScopeLifecycle(t *testing.T) {
	key := &struct{ value string }{"memo-scope-sentinel"}
	beforeBuilds, beforeReady := s6ContractBaselineBuilds.Load(), s6ContractBaselineReady.Load()
	s6ExportPathCache.Store(key, "retained-import")
	defer s6ExportPathCache.Delete(key)
	t.Run("scope", func(t *testing.T) {
		fixtureScope := s6SerialAnalyzerMemoScope(t)
		for index := 0; index < 5; index++ {
			t.Run(fmt.Sprint(index), func(t *testing.T) {
				fixtureScope(t)
				s6TypeModelCache.Store(key, "retained-model")
				s6BindingCache.Store(key, "retained-binding")
				s6StableExprKeyCache.Store(key, "retained-expression")
			})
			for name, cache := range map[string]*sync.Map{
				"model": &s6TypeModelCache, "binding": &s6BindingCache, "expression": &s6StableExprKeyCache,
			} {
				_, retained := cache.Load(key)
				if retained != (index != 3) {
					t.Errorf("%s memo retention after fixture %d = %v, want %v",
						name, index+1, retained, index != 3)
				}
			}
		}
	})
	for name, cache := range map[string]*sync.Map{
		"model": &s6TypeModelCache, "binding": &s6BindingCache, "expression": &s6StableExprKeyCache,
	} {
		if _, retained := cache.Load(key); retained {
			t.Errorf("%s memo survived final scope cleanup", name)
		}
	}
	if value, ok := s6ExportPathCache.Load(key); !ok || value != "retained-import" {
		t.Fatal("scope cleanup cleared shared import data")
	}
	if s6ContractBaselineBuilds.Load() != beforeBuilds || s6ContractBaselineReady.Load() != beforeReady {
		t.Fatal("scope cleanup changed immutable baseline state")
	}
}

func TestS6AnalyzerMemoBatchLifecycle(t *testing.T) {
	key := &struct{ value string }{"flat-fixture-sentinel"}
	t.Run("flat-loop", func(t *testing.T) {
		completed := s6SerialAnalyzerMemoBatch(t)
		for index := 0; index < 5; index++ {
			s6TypeModelCache.Store(key, "current-fixture")
			if _, found := s6TypeModelCache.Load(key); !found {
				t.Fatal("memo missing during active fixture")
			}
			completed()
			if _, found := s6TypeModelCache.Load(key); found != (index != 3) {
				t.Fatalf("memo retention after flat fixture %d = %v", index+1, found)
			}
		}
	})
	if _, found := s6TypeModelCache.Load(key); found {
		t.Fatal("flat fixture memo survived scope cleanup")
	}
}

func TestS6AnalyzerMemoScopeIdentity(t *testing.T) {
	const rel = "internal/memo_scope_fixture/fixture.go"
	source := map[string]string{rel: "package memo_scope_fixture\nfunc Value() int { return 1 }\n"}
	var previous *s6SourceTypeModel
	for _, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			fixtureScope := s6SerialAnalyzerMemoScope(t)
			t.Run("analysis", func(t *testing.T) {
				fixtureScope(t)
				first, err := s6BuildSourceTypeModel(source)
				if err != nil {
					t.Fatal(err)
				}

				reused, err := s6BuildSourceTypeModel(source)
				if err != nil || reused != first {
					t.Fatalf("fixture-local model identity changed: same=%v err=%v", reused == first, err)
				}
				mutated, err := s6BuildSourceTypeModel(map[string]string{
					rel: strings.Replace(source[rel], "return 1", "return 2", 1),
				})
				if err != nil || mutated == first || mutated.sourceKey == first.sourceKey {
					t.Fatalf("source mutation reused fixture-local model: same=%v err=%v", mutated == first, err)
				}
				if previous != nil && (first == previous || first.sourceKey != previous.sourceKey) {
					t.Fatal("fresh scope did not rebuild an equivalent independent model")
				}
				previous = first
			})
		})
	}
}
