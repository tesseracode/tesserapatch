package intentpub

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestProductionSourceHasNoPathWritersOrForbiddenJournalFields(t *testing.T) {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate package source")
	}
	directory := filepath.Dir(current)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		forbidden, parseErr := sourceUsesForbiddenPathOperation(entry.Name(), data)
		if parseErr != nil {
			t.Fatalf("%s: %v", entry.Name(), parseErr)
		}
		if forbidden != "" {
			t.Fatalf("%s contains forbidden path operation %s", entry.Name(), forbidden)
		}
		forbiddenField, parseErr := sourceForbiddenJournalField(entry.Name(), data)
		if parseErr != nil {
			t.Fatalf("%s: %v", entry.Name(), parseErr)
		}
		if forbiddenField != "" {
			t.Fatalf("%s declares forbidden persisted JSON field %q", entry.Name(), forbiddenField)
		}
	}

	sensitivities := map[string]string{
		"PIB-309-os-rename":               `package fixture; import "os"; func bad(){ os.Rename("a", "b") }`,
		"PIB-456-gitutil-writer":          `package fixture; import "x/internal/gitutil"; func bad(){ gitutil.DurableWriteFile("x", nil, 0600) }`,
		"store-artifact-writer":           `package fixture; func bad(s interface{ WriteArtifact(string,string,string) error }) { _ = s.WriteArtifact("s","p","v") }`,
		"store-status-writer":             `package fixture; func bad(){ SaveFeatureStatus("x") }`,
		"os-create-temp":                  `package fixture; import "os"; func bad(){ os.CreateTemp("x", "p") }`,
		"arbitrary-path-writer-insertion": `package fixture; import "os"; func arbitraryPathWriter(name string){ os.WriteFile(name, nil, 0600) }`,
		"aliased-os-writer":               `package fixture; import filesystem "os"; func bad(){ filesystem.Remove("x") }`,
		"os-function-value":               `package fixture; import filesystem "os"; var bad = filesystem.Rename`,
		"aliased-gitutil-function-value":  `package fixture; import durable "x/internal/gitutil"; var bad = durable.DurableWriteFile`,
		"aliased-filepath-construction":   `package fixture; import paths "path/filepath"; var bad = paths.Join`,
		"store-function-value":            `package fixture; var bad = SaveFeatureStatus`,
		"aliased-bounded-read-bypass":     `package fixture; import filesystem "os"; var bad = filesystem.ReadFile`,
	}
	for name, source := range sensitivities {
		if !containsForbiddenWriter(source) {
			t.Fatalf("%s did not trip the rooted-operation guard", name)
		}
	}

	for name, source := range map[string]string{
		"direct-root": `package fixture; import "os"; func good(root *os.Root){ _ = root.Rename("a", "b"); _ = root.Remove("a"); _ = root.Mkdir("a", 0700) }`,
		"root-ops":    `package fixture; type ops interface{ Rename(string,string) error; Remove(string) error }; func good(root ops){ _ = root.Rename("a", "b") }`,
	} {
		if containsForbiddenWriter(source) {
			t.Fatalf("%s incorrectly rejected an allowed rooted operation", name)
		}
	}

	for name, source := range map[string]string{
		"phase-options":       "package fixture; type wire struct { Ignored string `json:\"phase,omitempty\"` }",
		"phase-string-option": "package fixture; type wire struct { Alias int `json:\"phase,string\"` }",
		"phase-case":          "package fixture; type wire struct { Alias string `json:\"PhAsE,omitempty\"` }",
		"phase-default-name":  "package fixture; type wire struct { Version int `json:\"version\"`; Phase string }",
		"time-case-alias":     "package fixture; type wire struct { Alias string `json:\"CREATED_AT,omitempty\"` }",
		"time-camel-alias":    "package fixture; type wire struct { Alias string `json:\"createdAt,omitempty\"` }",
		"provider-alias":      "package fixture; type wire struct { Alias string `json:\"Provider_Name,omitempty\"` }",
		"provenance-case":     "package fixture; type wire struct { Alias string `json:\"PROVENANCE,omitempty\"` }",
		"raw-content":         "package fixture; type wire struct { Alias string `json:\"raw_content,omitempty\"` }",
	} {
		field, err := sourceForbiddenJournalField("fixture.go", []byte(source))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if field == "" {
			t.Fatalf("%s did not trip the AST journal-field guard", name)
		}
	}

	type zeroValueWire struct {
		Alias string `json:"phase,omitempty"`
	}
	encoded, err := json.Marshal(zeroValueWire{})
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != "{}" {
		t.Fatalf("zero-value sensitivity unexpectedly emitted forbidden key: %s", encoded)
	}
	field, err := sourceForbiddenJournalField(
		"fixture.go",
		[]byte("package fixture; type wire struct { Alias string `json:\"phase,omitempty\"` }"),
	)
	if err != nil || field != "phase" {
		t.Fatalf("zero-value omitted field escaped AST guard: field=%q err=%v", field, err)
	}
}

func containsForbiddenWriter(source string) bool {
	forbidden, err := sourceUsesForbiddenPathOperation("fixture.go", []byte(source))
	return err != nil || forbidden != ""
}

func sourceUsesForbiddenPathOperation(filename string, source []byte) (string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), filename, source, parser.SkipObjectResolution)
	if err != nil {
		return "", err
	}
	imports := make(map[string]string)
	dotImports := make(map[string]bool)
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return "", err
		}
		name := path.Base(importPath)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "_" {
			continue
		}
		if name == "." {
			dotImports[importPath] = true
			continue
		}
		imports[name] = importPath
	}

	var forbidden string
	ast.Inspect(file, func(node ast.Node) bool {
		if forbidden != "" {
			return false
		}
		switch expression := node.(type) {
		case *ast.SelectorExpr:
			if identifier, ok := expression.X.(*ast.Ident); ok {
				if importPath, imported := imports[identifier.Name]; imported &&
					forbiddenImportedSelector(importPath, expression.Sel.Name) {
					forbidden = identifier.Name + "." + expression.Sel.Name
					return false
				}
			}
			if forbiddenStoreWriter(expression.Sel.Name) {
				forbidden = expression.Sel.Name
				return false
			}
		case *ast.Ident:
			if forbiddenStoreWriter(expression.Name) {
				forbidden = expression.Name
				return false
			}
			for importPath := range dotImports {
				if forbiddenImportedSelector(importPath, expression.Name) {
					forbidden = expression.Name
					return false
				}
			}
		}
		return true
	})
	return forbidden, nil
}

func sourceForbiddenJournalField(filename string, source []byte) (string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), filename, source, parser.SkipObjectResolution)
	if err != nil {
		return "", err
	}
	var forbidden string
	var inspectErr error
	ast.Inspect(file, func(node ast.Node) bool {
		if forbidden != "" || inspectErr != nil {
			return false
		}
		structure, ok := node.(*ast.StructType)
		if !ok {
			return true
		}
		hasJSONTag := false
		for _, field := range structure.Fields.List {
			if field.Tag == nil {
				continue
			}
			rawTag, err := strconv.Unquote(field.Tag.Value)
			if err != nil {
				inspectErr = err
				return false
			}
			if _, ok := reflect.StructTag(rawTag).Lookup("json"); ok {
				hasJSONTag = true
				break
			}
		}
		if !hasJSONTag {
			return true
		}
		for _, field := range structure.Fields.List {
			jsonName := ""
			tagged := false
			if field.Tag != nil {
				rawTag, err := strconv.Unquote(field.Tag.Value)
				if err != nil {
					inspectErr = err
					return false
				}
				if jsonTag, ok := reflect.StructTag(rawTag).Lookup("json"); ok {
					tagged = true
					jsonName = strings.Split(jsonTag, ",")[0]
					if jsonName == "-" {
						continue
					}
				}
			}
			if tagged && jsonName != "" {
				if forbiddenJournalJSONName(jsonName) {
					forbidden = jsonName
					return false
				}
				continue
			}
			for _, fieldName := range field.Names {
				if ast.IsExported(fieldName.Name) && forbiddenJournalJSONName(fieldName.Name) {
					forbidden = fieldName.Name
					return false
				}
			}
			if forbidden != "" {
				return false
			}
		}
		return forbidden == ""
	})
	return forbidden, inspectErr
}

func forbiddenJournalJSONName(name string) bool {
	if name == "" || name == "-" {
		return false
	}
	folded := strings.ToLower(name)
	if folded == "phase" || folded == "timestamp" ||
		folded == "absolute_path" || folded == "duration" ||
		folded == "hostname" || folded == "pid" ||
		folded == "secret" || folded == "token" || folded == "env_value" {
		return true
	}
	if folded == "at" || strings.HasSuffix(folded, "_at") || strings.HasSuffix(folded, "-at") ||
		(len(name) > 2 && name[len(name)-2] == 'A' &&
			(name[len(name)-1] == 'T' || name[len(name)-1] == 't')) {
		return true
	}
	if strings.Contains(folded, "provider") || strings.Contains(folded, "provenance") {
		return true
	}
	return strings.Contains(folded, "content") ||
		strings.Contains(folded, "prompt") || strings.Contains(folded, "transcript")
}

func forbiddenImportedSelector(importPath, name string) bool {
	switch importPath {
	case "os":
		return stringSet(name,
			"Rename", "CreateTemp", "Create", "WriteFile", "OpenFile",
			"Remove", "Mkdir", "MkdirAll", "ReadFile", "Open", "Stat", "Lstat")
	case "io":
		return stringSet(name, "ReadAll", "LimitReader")
	case "bufio":
		return name == "NewScanner"
	case "path/filepath":
		return stringSet(name, "Join", "Abs", "Clean", "FromSlash", "Rel")
	}
	if strings.HasSuffix(importPath, "/gitutil") {
		return name == "DurableWriteFile"
	}
	if strings.HasSuffix(importPath, "/store") {
		return forbiddenStoreWriter(name)
	}
	return false
}

func forbiddenStoreWriter(name string) bool {
	return stringSet(name, "WriteArtifact", "SaveFeatureStatus", "writeFileAtomic", "DurableWriteFile")
}

func stringSet(value string, members ...string) bool {
	for _, member := range members {
		if value == member {
			return true
		}
	}
	return false
}
