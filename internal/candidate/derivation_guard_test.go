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
	var build, align *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if fn.Name.Name == "newAlignmentTables" {
				build = fn
			}
			if fn.Name.Name == "alignInsertionRuns" {
				align = fn
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
		{"1048576", "16777216"},
		{"make([]int32, cells)", "make([]int64, cells)"},
		{"f := make([]int32, cells)", "_ = make([]int32, cells)\n\tf := make([]int32, cells)"},
		{"if err := alignmentLimits(base, upstream); err != nil {", "if err := configurableLimits(base, upstream); err != nil {"},
		{"n > alignmentMaxRecords || m > alignmentMaxRecords || n+1 > alignmentMaxCells/(m+1)", "false"},
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
