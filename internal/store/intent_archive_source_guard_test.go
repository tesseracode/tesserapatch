package store

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestIntentArchiveSourceGuards(t *testing.T) {
	const sourcePath = "intent_archive.go"
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), sourcePath, source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	forbiddenImports := []string{
		"github.com/tesseracode/tesserapatch/internal/intentpub",
		"github.com/tesseracode/tesserapatch/internal/intentlock",
		"github.com/tesseracode/tesserapatch/internal/cli",
		"regexp",
		"time",
	}
	for _, spec := range file.Imports {
		importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil {
			t.Fatal(unquoteErr)
		}
		for _, forbidden := range forbiddenImports {
			if importPath == forbidden {
				t.Fatalf("intent archive imports forbidden package %q", importPath)
			}
		}
	}

	wireStructs := map[string]bool{
		"IntentArchiveIndex":                    true,
		"IntentArchiveGeneration":               true,
		"IntentArchiveReplacement":              true,
		"intentArchiveGenerationBody":           true,
		"intentArchiveImmutableReplacementBody": true,
	}
	seenWire := map[string]bool{}
	appendPlanOpaque := false
	ast.Inspect(file, func(node ast.Node) bool {
		typeSpec, ok := node.(*ast.TypeSpec)
		if !ok {
			return true
		}
		if typeSpec.Name.Name == "IntentArchiveAppendPlan" {
			appendPlanOpaque = true
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				t.Fatal("IntentArchiveAppendPlan is no longer a struct")
			}
			for _, field := range structType.Fields.List {
				for _, name := range field.Names {
					if ast.IsExported(name.Name) {
						t.Fatalf("append plan field %s is externally mutable", name.Name)
					}
				}
			}
			return false
		}
		if !wireStructs[typeSpec.Name.Name] {
			return true
		}
		seenWire[typeSpec.Name.Name] = true
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			t.Fatalf("%s is no longer a fixed-field struct", typeSpec.Name.Name)
		}
		ast.Inspect(structType, func(child ast.Node) bool {
			if _, isMap := child.(*ast.MapType); isMap {
				t.Fatalf("%s contains a Go map", typeSpec.Name.Name)
			}
			return true
		})
		return false
	})
	for name := range wireStructs {
		if !seenWire[name] {
			t.Fatalf("wire struct %s not found", name)
		}
	}
	if !appendPlanOpaque {
		t.Fatal("IntentArchiveAppendPlan not found")
	}

	text := string(source)
	for _, forbidden := range []string{
		"`json:\"timestamp",
		"`json:\"created_at",
		"`json:\"provider",
		"`json:\"provenance",
		"`json:\"generator",
		"regexp.MustCompile",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("forbidden archive source token %q", forbidden)
		}
	}
	if !strings.Contains(text, "redact.Scan(") {
		t.Fatal("archive write path does not use the shared redact.Scan")
	}

	removeCalls := 0
	recoveryCalls := 0
	appendPublisherFound := false
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		if function.Name.Name == "PublishIntentArchiveBlobs" {
			appendPublisherFound = true
		}
		if function.Name.Name == "ExecuteIntentArchiveAppend" {
			t.Fatal("retired append index executor is still present")
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "RemoveBlob" {
				removeCalls++
				if function.Name.Name != "removeIntentArchiveBlob" {
					t.Fatalf("RemoveBlob call escaped the single authorization helper into %s", function.Name.Name)
				}
			}
			if identifier, ok := call.Fun.(*ast.Ident); ok &&
				identifier.Name == "RecoverPendingPurge" {
				recoveryCalls++
			}
			return true
		})
	}
	if removeCalls != 1 {
		t.Fatalf("RemoveBlob call sites = %d, want one authorization choke point", removeCalls)
	}
	if !strings.Contains(text, "if !authorized {") {
		t.Fatal("blob removal is not dominated by the authorization check")
	}
	if recoveryCalls != 0 {
		t.Fatalf("RecoverPendingPurge has %d production call sites in store; S4b must own the only call site", recoveryCalls)
	}
	if !appendPublisherFound {
		t.Fatal("PublishIntentArchiveBlobs not found")
	}
	if calls := intentArchiveFunctionSelectorCount(file, "PublishIntentArchiveBlobs", "CASIndex"); calls != 0 {
		t.Fatalf("PublishIntentArchiveBlobs contains %d CASIndex references", calls)
	}
}

func TestIntentArchiveBlobPublisherCASIndexGuardSensitivity(t *testing.T) {
	fixtures := []struct {
		name   string
		source string
	}{
		{
			name: "direct-call",
			source: `package store
func PublishIntentArchiveBlobs(storage interface{ CASIndex() }) {
	storage.CASIndex()
}`,
		},
		{
			name: "method-value-alias",
			source: `package store
func PublishIntentArchiveBlobs(storage interface{ CASIndex() }) {
	publishIndex := storage.CASIndex
	publishIndex()
}`,
		},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "sensitivity.go", fixture.source, 0)
			if err != nil {
				t.Fatal(err)
			}
			if calls := intentArchiveFunctionSelectorCount(file, "PublishIntentArchiveBlobs", "CASIndex"); calls != 1 {
				t.Fatalf("CASIndex selector count = %d, want 1", calls)
			}
			if intentArchiveBlobOnlyPublisherValid(file) {
				t.Fatal("blob-only source guard accepted a CASIndex insertion")
			}
		})
	}
}

func intentArchiveBlobOnlyPublisherValid(file *ast.File) bool {
	return intentArchiveFunctionSelectorCount(file, "PublishIntentArchiveBlobs", "CASIndex") == 0
}

func intentArchiveFunctionSelectorCount(file *ast.File, functionName, selectorName string) int {
	count := 0
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil || function.Name.Name != functionName {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			if selector, ok := node.(*ast.SelectorExpr); ok && selector.Sel.Name == selectorName {
				count++
			}
			return true
		})
	}
	return count
}

func TestIntentArchiveEmptyReportsUseNonNullSlices(t *testing.T) {
	index, err := NewIntentArchiveIndex("demo")
	if err != nil {
		t.Fatal(err)
	}
	report, err := InspectIntentArchive(index, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Hashes == nil ||
		report.References == nil ||
		report.Orphans == nil ||
		report.PendingHashes == nil ||
		report.Classes == nil {
		t.Fatalf("empty inspection contains a nil array: %+v", report)
	}
}
