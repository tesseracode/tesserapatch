package candidate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func validateS3AlignmentResources(source string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "alignment.go", source, 0)
	if err != nil {
		return err
	}
	want := map[string]string{"alignmentMaxBytes": "4194304", "alignmentMaxRecords": "131072", "alignmentMaxCells": "1048576"}
	for _, decl := range file.Decls {
		group, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if group.Tok == token.IMPORT {
			for _, spec := range group.Specs {
				path, _ := strconv.Unquote(spec.(*ast.ImportSpec).Path.Value)
				if path != "bytes" && path != "fmt" && path != "sort" {
					return fmt.Errorf("alignment acquired external/configuration input")
				}
			}
		}
		for _, spec := range group.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range value.Names {
				expected, controlled := want[name.Name]
				if !controlled {
					continue
				}
				if group.Tok != token.CONST || i >= len(value.Values) {
					return fmt.Errorf("alignment limit is not constant")
				}
				literal, ok := value.Values[i].(*ast.BasicLit)
				if !ok || literal.Kind != token.INT || literal.Value != expected {
					return fmt.Errorf("alignment limit changed")
				}
				delete(want, name.Name)
			}
		}
	}
	if len(want) != 0 {
		return fmt.Errorf("alignment limit missing")
	}
	var build, align, indexer *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if fn.Name.Name == "newAlignmentTables" {
				build = fn
			}
			if fn.Name.Name == "alignInsertionRuns" {
				align = fn
			}
			if fn.Name.Name == "mapBoundaryBounds" {
				indexer = fn
			}
			if fn.Name.Name == "unanimousBounds" {
				return fmt.Errorf("per-boundary grid scanner reintroduced")
			}
		}
	}
	if build == nil || align == nil || len(build.Body.List) < 2 || len(align.Body.List) == 0 {
		return fmt.Errorf("alignment entrypoints missing")
	}
	check, ok := build.Body.List[1].(*ast.IfStmt)
	if !ok {
		return fmt.Errorf("matrix allocation precedes limits")
	}
	hasCells := false
	ast.Inspect(check.Cond, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == "alignmentMaxCells" {
			hasCells = true
		}
		return true
	})
	if !hasCells {
		return fmt.Errorf("cell limit not checked before allocation")
	}
	first, ok := align.Body.List[0].(*ast.IfStmt)
	if !ok {
		return fmt.Errorf("alignment limits no longer first")
	}
	hasLimits := false
	if first.Init == nil {
		return fmt.Errorf("alignment entry bypasses limits")
	}
	ast.Inspect(first.Init, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "alignmentLimits" {
				hasLimits = true
			}
		}
		return true
	})
	if !hasLimits {
		return fmt.Errorf("alignment entry bypasses limits")
	}
	indexCalls, topLevelIndexCalls := 0, 0
	for _, statement := range align.Body.List {
		if check, ok := statement.(*ast.IfStmt); ok && check.Init != nil {
			ast.Inspect(check.Init, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if method, ok := call.Fun.(*ast.SelectorExpr); ok && method.Sel.Name == "mapBoundaryBounds" {
						topLevelIndexCalls++
					}
				}
				return true
			})
		}
	}
	ast.Inspect(align, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if method, ok := call.Fun.(*ast.SelectorExpr); ok && method.Sel.Name == "mapBoundaryBounds" {
				indexCalls++
			}
		}
		return true
	})
	if indexer == nil || indexCalls != 1 || topLevelIndexCalls != 1 {
		return fmt.Errorf("optimal pairs must be indexed once outside boundary loops")
	}
	topLevelPairLoops, pairLoops := 0, 0
	for _, statement := range indexer.Body.List {
		if loop, ok := statement.(*ast.RangeStmt); ok {
			if name, ok := loop.X.(*ast.Ident); ok && name.Name == "base" {
				topLevelPairLoops++
			}
		}
	}
	ast.Inspect(indexer, func(n ast.Node) bool {
		if loop, ok := n.(*ast.RangeStmt); ok {
			if name, ok := loop.X.(*ast.Ident); ok && (name.Name == "base" || name.Name == "upstream") {
				pairLoops++
			}
		}
		return true
	})
	if topLevelPairLoops != 1 || pairLoops != 2 {
		return fmt.Errorf("optimal-pair grid must have one top-level pass")
	}
	allocations := 0
	var refused error
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		fn, ok := call.Fun.(*ast.Ident)
		if !ok || fn.Name != "make" {
			return true
		}
		array, ok := call.Args[0].(*ast.ArrayType)
		if !ok {
			return true
		}
		element, ok := array.Elt.(*ast.Ident)
		if !ok || (!strings.HasPrefix(element.Name, "int") && !strings.HasPrefix(element.Name, "uint")) {
			return true
		}
		allocations++
		if element.Name != "int32" || call.Pos() <= check.End() || call.End() >= build.End() {
			refused = fmt.Errorf("unbounded or misplaced matrix allocation")
		}
		return true
	})
	if refused != nil {
		return refused
	}
	if allocations != 2 {
		return fmt.Errorf("need exactly two int32 matrices")
	}
	return nil
}

func validateS3DerivationBoundary(name, source string) error {
	file, err := parser.ParseFile(token.NewFileSet(), name, source, 0)
	if err != nil {
		return err
	}
	calls := map[string]bool{}
	for _, imp := range file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		switch path {
		case "bytes", "crypto/sha256", "errors", "fmt", "sort",
			"github.com/tesseracode/tesserapatch/internal/gitutil",
			"github.com/tesseracode/tesserapatch/internal/patchobs":
		default:
			return fmt.Errorf("derivation acquired IO/serialization/provider input")
		}
	}
	var invalid error
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		callee := ""
		switch f := call.Fun.(type) {
		case *ast.Ident:
			callee = f.Name
		case *ast.SelectorExpr:
			callee = f.Sel.Name
			if receiver, ok := f.X.(*ast.Ident); ok {
				callee = receiver.Name + "." + callee
			}
		}
		calls[callee] = true
		for _, bad := range []string{"ExecuteRecipe", "evaluateRecipeOperations", "ReadFile", "WriteFile", "CreateCandidateRoot", "Prune", "Print", "Printf", "Println", "TrimSpace", "Contains"} {
			if callee == bad || strings.HasSuffix(callee, "."+bad) {
				invalid = fmt.Errorf("derivation acquired unsafe shortcut %s", callee)
			}
		}
		return true
	})
	if invalid != nil {
		return invalid
	}
	required := map[string][]string{
		"derivation.go": {"gitutil.NormalizePatchEffects", "reference.snapshotPaths", "upstream.snapshotPaths", "gitutil.ExtractPatchInsertionRuns", "replayInsertionDigest", "alignInsertionRuns"},
		"snapshot.go":   {"patchobs.SnapshotCommitPathsOffline", "patchobs.ReadCommitObjectOffline"},
	}
	for _, call := range required[name] {
		if !calls[call] {
			return fmt.Errorf("derivation boundary lost %s", call)
		}
	}
	return nil
}

func TestS3DerivationBoundaryGuardSensitivity(t *testing.T) {
	for _, row := range []struct{ name, old, replacement string }{
		{"derivation.go", "gitutil.NormalizePatchEffects(patch)", "privateLenientParse(patch)"},
		{"derivation.go", "upstream.snapshotPaths(paths)", "readLiveTree(paths)"},
		{"derivation.go", "replayInsertionDigest(pre, projected)", "trustStoredPostimage(pre, projected)"},
		{"derivation.go", "alignInsertionRuns(pre, body, projected)", "firstTraceback(pre, body, projected)"},
		{"snapshot.go", "patchobs.ReadCommitObjectOffline", "readMutablePath"},
	} {
		raw, err := os.ReadFile(row.name)
		if err != nil {
			t.Fatal(err)
		}
		source := string(raw)
		if err := validateS3DerivationBoundary(row.name, source); err != nil {
			t.Fatal(err)
		}
		if strings.Count(source, row.old) != 1 {
			t.Fatalf("missing mutation anchor %q", row.old)
		}
		if err := validateS3DerivationBoundary(row.name, strings.Replace(source, row.old, row.replacement, 1)); err == nil {
			t.Fatalf("boundary guard accepted %q", row.replacement)
		}
	}
}

func TestS3AlignmentResourceGuardSensitivity(t *testing.T) {
	raw, err := os.ReadFile("alignment.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	if err := validateS3AlignmentResources(source); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range [][2]string{
		{"const (\n\talignmentMaxBytes", "var (\n\talignmentMaxBytes"},
		{"1048576", "16777216"},
		{"make([]int32, cells)", "make([]int64, cells)"},
		{"f := make([]int32, cells)", "_ = make([]int32, cells)\n\tf := make([]int32, cells)"},
		{"if err := alignmentLimits(base, upstream); err != nil {", "if err := configurableLimits(base, upstream); err != nil {"},
		{"n > alignmentMaxRecords || m > alignmentMaxRecords || n+1 > alignmentMaxCells/(m+1)", "false"},
		{"if err := tables.mapBoundaryBounds(b, u, boundaries); err != nil {", "if err := rescanEveryBoundary(b, u, boundaries); err != nil {"},
		{"rowMax, rowMin := int32(-1), int32(m)", "for range base {}\n\t\trowMax, rowMin := int32(-1), int32(m)"},
	} {
		if !strings.Contains(source, mutation[0]) {
			t.Fatalf("missing mutation anchor %q", mutation[0])
		}
		mutant := strings.Replace(source, mutation[0], mutation[1], 1)
		if err := validateS3AlignmentResources(mutant); err == nil {
			t.Fatalf("resource guard accepted %q", mutation[1])
		}
	}
}
