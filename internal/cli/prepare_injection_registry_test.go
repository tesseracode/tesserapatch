package cli

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

type injectionSeamSpec struct {
	owner     string
	signature string
}

var prepareInjectionSeamRegistry = map[string]injectionSeamSpec{
	"beforeLockAcquire":           {owner: "cli", signature: "func()"},
	"failLockAcquire":             {owner: "intentlock", signature: "func() error"},
	"beforeAbandonBranch":         {owner: "cli", signature: "func()"},
	"beforeAbandonMove":           {owner: "cli", signature: "func(string)"},
	"afterAbandonMove":            {owner: "cli", signature: "func(string)"},
	"beforeRedactionScan":         {owner: "cli", signature: "func()"},
	"beforeBlobWrite":             {owner: "store", signature: "func(string)"},
	"afterBlobWrite":              {owner: "store", signature: "func(string)"},
	"beforeJournalWrite":          {owner: "intentpub", signature: "func(string)"},
	"afterJournalWrite":           {owner: "intentpub", signature: "func(string)"},
	"beforeControlWriteRename":    {owner: "intentpub", signature: "func(string)"},
	"beforeEntryCAS":              {owner: "intentpub", signature: "func(int)"},
	"beforeRename":                {owner: "intentpub", signature: "func(int)"},
	"afterRename":                 {owner: "intentpub", signature: "func(int)"},
	"beforeIndexRewrite":          {owner: "cli", signature: "func(string)"},
	"beforeStatusRename":          {owner: "intentpub", signature: "func(string)"},
	"afterStatusRename":           {owner: "intentpub", signature: "func(string)"},
	"beforeFinalVerify":           {owner: "intentpub", signature: "func()"},
	"beforeJournalClear":          {owner: "intentpub", signature: "func(string)"},
	"beforeLockRelease":           {owner: "intentlock", signature: "func()"},
	"afterLockRelease":            {owner: "intentlock", signature: "func()"},
	"beforeBlobRemove":            {owner: "store", signature: "func(string)"},
	"afterPurgeBlobRemove":        {owner: "store", signature: "func(string)"},
	"beforePendingTombstoneCAS":   {owner: "store", signature: "func(string)"},
	"failPurgeAfterFirstMutation": {owner: "store", signature: "func() error"},
	"beforeManualStatusCAS":       {owner: "cli", signature: "func()"},
	"beforePurgeIndexCAS":         {owner: "store", signature: "func(string)"},
	"afterPurgeIndexRename":       {owner: "store", signature: "func(string)"},
	"beforeRehydrateIndexRename":  {owner: "cli", signature: "func(string)"},
	"beforeRootIdentityCheck":     {owner: "intentlock", signature: "func(string)"},
	"beforePurgeBlobRemove":       {owner: "store", signature: "func(string)"},
	"afterPurgeBlobRevalidate":    {owner: "store", signature: "func(string)"},
	"failPurgeBetweenHashes":      {owner: "store", signature: "func() error"},
	"failOrphanRemoveAfterFirst":  {owner: "store", signature: "func() error"},
	"afterRecoveryComplete":       {owner: "cli", signature: "func()"},
	"failFsync":                   {owner: "intentpub", signature: "func(string) error"},
	"failRename":                  {owner: "intentpub", signature: "func(string) error"},
}

type injectionSeamObservation struct {
	declarations int
	calls        int
	assignments  int
	owner        string
	signature    string
	initialized  bool
}

func TestPIB232NamedInjectionSeamRegistry(t *testing.T) {
	if len(prepareInjectionSeamRegistry) != 37 {
		t.Fatalf("injection registry has %d seams, want the 37 exact names listed in §18.1", len(prepareInjectionSeamRegistry))
	}
	observed, err := scanPrepareInjectionSeams(filepath.Join("..", "..", "internal"))
	if err != nil {
		t.Fatal(err)
	}
	for name, spec := range prepareInjectionSeamRegistry {
		got := observed[name]
		if got.declarations != 1 ||
			got.calls == 0 ||
			got.assignments != 0 ||
			got.initialized ||
			got.owner != spec.owner ||
			got.signature != spec.signature {
			t.Errorf("%s = %+v, want owner=%s signature=%s one nil declaration, calls, no assignments",
				name, got, spec.owner, spec.signature)
		}
	}
}

func TestPIB232AssignmentSensitivity(t *testing.T) {
	source := []byte(`package intentlock
var beforeLockAcquire func()
func bad() { beforeLockAcquire = func() {} }
`)
	fileset := token.NewFileSet()
	parsed, err := parser.ParseFile(fileset, "bad.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	observed := make(map[string]injectionSeamObservation)
	observePrepareInjectionFile(fileset, parsed, observed)
	if observed["beforeLockAcquire"].assignments != 1 {
		t.Fatal("a production assignment escaped the injection seam guard")
	}
}

func scanPrepareInjectionSeams(root string) (map[string]injectionSeamObservation, error) {
	observed := make(map[string]injectionSeamObservation, len(prepareInjectionSeamRegistry))
	fileset := token.NewFileSet()
	err := filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(filePath) != ".go" || strings.HasSuffix(filePath, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(fileset, filePath, nil, 0)
		if err != nil {
			return err
		}
		observePrepareInjectionFile(fileset, parsed, observed)
		return nil
	})
	return observed, err
}

func observePrepareInjectionFile(
	fileset *token.FileSet,
	parsed *ast.File,
	observed map[string]injectionSeamObservation,
) {
	owner := parsed.Name.Name
	ast.Inspect(parsed, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.ValueSpec:
			for index, identifier := range typed.Names {
				if _, tracked := prepareInjectionSeamRegistry[identifier.Name]; !tracked {
					continue
				}
				item := observed[identifier.Name]
				item.declarations++
				item.owner = owner
				item.signature = renderInjectionNode(fileset, typed.Type)
				item.initialized = len(typed.Values) > index || len(typed.Values) > 0
				observed[identifier.Name] = item
			}
		case *ast.AssignStmt:
			for _, expression := range typed.Lhs {
				identifier, ok := expression.(*ast.Ident)
				if !ok {
					continue
				}
				if _, tracked := prepareInjectionSeamRegistry[identifier.Name]; tracked {
					item := observed[identifier.Name]
					item.assignments++
					observed[identifier.Name] = item
				}
			}
		case *ast.CallExpr:
			identifier, ok := typed.Fun.(*ast.Ident)
			if !ok {
				break
			}
			if _, tracked := prepareInjectionSeamRegistry[identifier.Name]; tracked {
				item := observed[identifier.Name]
				item.calls++
				observed[identifier.Name] = item
			}
		}
		return true
	})
}

func renderInjectionNode(fileset *token.FileSet, node ast.Node) string {
	if node == nil {
		return ""
	}
	var output bytes.Buffer
	if err := format.Node(&output, fileset, node); err != nil {
		return fmt.Sprintf("<format error: %v>", err)
	}
	return output.String()
}
