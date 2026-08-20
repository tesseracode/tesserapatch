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

func TestProductionWriteRequestsDeclareExplicitRoles(t *testing.T) {
	for _, directory := range []string{".", "../cli"} {
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}
			path := filepath.Join(directory, entry.Name())
			fileSet := token.NewFileSet()
			file, err := parser.ParseFile(fileSet, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(file, func(node ast.Node) bool {
				literal, ok := node.(*ast.CompositeLit)
				if !ok || !isWriteRequestType(literal.Type) {
					return true
				}
				for _, element := range literal.Elts {
					field, ok := element.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if name, ok := field.Key.(*ast.Ident); ok && name.Name == "Role" {
						return true
					}
				}
				position := fileSet.Position(literal.Pos())
				t.Errorf("%s:%d WriteRequest omits its explicit Role", path, position.Line)
				return true
			})
		}
	}
}

func isWriteRequestType(expression ast.Expr) bool {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name == "WriteRequest"
	case *ast.SelectorExpr:
		return typed.Sel.Name == "WriteRequest"
	default:
		return false
	}
}
