package intent

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The status mirror in status_schema.go must stay field-for-field identical to
// store.FeatureStatus, or `status-malformed` silently narrows back to "the
// state member is not a string" — the exact rev-0 defect. The parity guard
// below is mechanical: it walks both shapes by AST and compares JSON names,
// `omitempty` flags and normalized types transitively. A sensitivity fixture
// proves it fails on a field addition, a tag rename and a type change.

type schemaField struct {
	JSON      string
	Omitempty bool
	Type      string
}

// storeToCanonical maps every struct reachable from FeatureStatus to its
// canonical name; mirrorToCanonical does the same for the local mirror. The
// two maps are the only hand-maintained part of the guard, and a struct that
// appears in one and not the other fails the walk.
var storeToCanonical = map[string]string{
	"FeatureStatus":         "FeatureStatus",
	"ApplySummary":          "ApplySummary",
	"ReconcileSummary":      "ReconcileSummary",
	"PatchIDMatch":          "PatchIDMatch",
	"Dependency":            "Dependency",
	"VerifyRecord":          "VerifyRecord",
	"RejectionStatus":       "RejectionStatus",
	"RejectionHistoryEntry": "RejectionHistoryEntry",
	"EvidenceRef":           "EvidenceRef",
	"DivergenceDetail":      "DivergenceDetail",
}

var mirrorToCanonical = map[string]string{
	"statusDocument":              "FeatureStatus",
	"statusApplySummary":          "ApplySummary",
	"statusReconcileSummary":      "ReconcileSummary",
	"statusPatchIDMatch":          "PatchIDMatch",
	"statusDependency":            "Dependency",
	"statusVerifyRecord":          "VerifyRecord",
	"statusRejection":             "RejectionStatus",
	"statusRejectionHistoryEntry": "RejectionHistoryEntry",
	"statusEvidenceRef":           "EvidenceRef",
	"statusDivergenceDetail":      "DivergenceDetail",
}

// namedStringTypes are store enums that marshal as plain JSON strings. The
// mirror uses `string` for them deliberately: it validates shape, and the
// lifecycle value is validated separately by validFeatureState.
var namedStringTypes = map[string]bool{
	"FeatureState":        true,
	"CompatibilityStatus": true,
	"ReconcileOutcome":    true,
	"ReconcileLabel":      true,
}

func TestAVPStatusSchemaParity(t *testing.T) {
	storeShape := schemaShapeFromSources(t, map[string]string{
		"types.go":  repoFile(t, "internal/store/types.go"),
		"status.go": repoFile(t, "internal/store/status.go"),
	}, storeToCanonical)
	mirrorShape := schemaShapeFromSources(t, map[string]string{
		"status_schema.go": repoFile(t, "internal/intent/status_schema.go"),
	}, mirrorToCanonical)

	if err := compareSchemaShapes(storeShape, mirrorShape); err != nil {
		t.Fatalf("the local status mirror drifted from store.FeatureStatus: %v", err)
	}
	if len(storeShape) != len(storeToCanonical) {
		t.Fatalf("walked %d store structs, want %d", len(storeShape), len(storeToCanonical))
	}
}

func TestAVPStatusSchemaParityIsSensitive(t *testing.T) {
	mirrorShape := schemaShapeFromSources(t, map[string]string{
		"status_schema.go": repoFile(t, "internal/intent/status_schema.go"),
	}, mirrorToCanonical)

	storeSources := map[string]string{
		"types.go":  repoFile(t, "internal/store/types.go"),
		"status.go": repoFile(t, "internal/store/status.go"),
	}

	mutations := []struct {
		name    string
		file    string
		old     string
		replace string
	}{
		{
			name: "new-field-added-to-FeatureStatus",
			file: "types.go",
			old:  "\tApply         ApplySummary        `json:\"apply\"`",
			replace: "\tApply         ApplySummary        `json:\"apply\"`\n" +
				"\tArchive       ApplySummary        `json:\"archive\"`",
		},
		{
			name:    "field-type-changed",
			file:    "types.go",
			old:     "\tLastCommand   string              `json:\"last_command\"`",
			replace: "\tLastCommand   []string            `json:\"last_command\"`",
		},
		{
			name:    "json-tag-renamed",
			file:    "types.go",
			old:     "\tRequestedAt   string              `json:\"requested_at\"`",
			replace: "\tRequestedAt   string              `json:\"requested_at_utc\"`",
		},
		{
			name:    "omitempty-dropped",
			file:    "types.go",
			old:     "\tNotes         string              `json:\"notes,omitempty\"`",
			replace: "\tNotes         string              `json:\"notes\"`",
		},
		{
			name:    "nested-struct-field-added",
			file:    "types.go",
			old:     "\tHasRecipe   bool   `json:\"has_recipe,omitempty\"`",
			replace: "\tHasRecipe   bool   `json:\"has_recipe,omitempty\"`\n\tHasBundle   bool   `json:\"has_bundle,omitempty\"`",
		},
		{
			name:    "rejection-field-type-changed",
			file:    "status.go",
			old:     "\tReason     string        `json:\"reason\"`",
			replace: "\tReason     int           `json:\"reason\"`",
		},
	}

	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			mutated := map[string]string{}
			for name, source := range storeSources {
				mutated[name] = source
			}
			original, ok := mutated[mutation.file]
			if !ok {
				t.Fatalf("no fixture source %s", mutation.file)
			}
			if !strings.Contains(original, mutation.old) {
				t.Fatalf("the sensitivity fixture no longer matches %s: %q", mutation.file, mutation.old)
			}
			mutated[mutation.file] = strings.Replace(original, mutation.old, mutation.replace, 1)

			drifted := schemaShapeFromSources(t, mutated, storeToCanonical)
			if err := compareSchemaShapes(drifted, mirrorShape); err == nil {
				t.Fatal("the parity guard accepted a drifted store shape")
			}
		})
	}
}

// TestAVPStatusSchemaRejectsUnknownFieldStrictness pins the accepted contract
// that unknown JSON keys stay acceptable.
func TestAVPStatusSchemaRejectsUnknownFieldStrictness(t *testing.T) {
	if strings.Contains(repoFile(t, "internal/intent/status_schema.go"), "DisallowUnknownFields()") {
		t.Fatal("the mirror sets DisallowUnknownFields; forward-compatible documents must still decode")
	}
	if _, ok := decodeStatusDocument([]byte(`{"state":"defined","future_key":{"nested":[1,2,3]}}`)); !ok {
		t.Fatal("an unknown forward-compatible key was rejected")
	}
	if _, ok := decodeStatusDocument([]byte(`{"state":"defined","apply":{"future":true}}`)); !ok {
		t.Fatal("an unknown nested key was rejected")
	}
}

// TestAVPStatusSchemaMirrorsRuntimeShape is the belt-and-braces half: the
// mirror's own reflected JSON surface must match what the AST walk derived,
// so a build-tag trick or a type alias cannot hide a divergence.
func TestAVPStatusSchemaMirrorsRuntimeShape(t *testing.T) {
	mirrorShape := schemaShapeFromSources(t, map[string]string{
		"status_schema.go": repoFile(t, "internal/intent/status_schema.go"),
	}, mirrorToCanonical)
	declared := mirrorShape["FeatureStatus"]
	reflected := reflect.TypeOf(statusDocument{})
	if reflected.NumField() != len(declared) {
		t.Fatalf("the mirror declares %d fields at run time and %d in source", reflected.NumField(), len(declared))
	}
	for index := 0; index < reflected.NumField(); index++ {
		tag := reflected.Field(index).Tag.Get("json")
		name, options, _ := strings.Cut(tag, ",")
		if declared[index].JSON != name {
			t.Fatalf("field %d: runtime json name %q, source %q", index, name, declared[index].JSON)
		}
		if declared[index].Omitempty != strings.Contains(options, "omitempty") {
			t.Fatalf("field %d (%s): omitempty disagreement", index, name)
		}
	}
}

// ---------------------------------------------------------------------------
// Shape extraction
// ---------------------------------------------------------------------------

func schemaShapeFromSources(t *testing.T, sources map[string]string, canonical map[string]string) map[string][]schemaField {
	t.Helper()
	fset := token.NewFileSet()
	shape := map[string][]schemaField{}
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		file, err := parser.ParseFile(fset, name, sources[name], 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				key, tracked := canonical[typeSpec.Name.Name]
				if !tracked {
					continue
				}
				shape[key] = schemaFieldsOf(structType, canonical)
			}
		}
	}
	return shape
}

func schemaFieldsOf(structType *ast.StructType, canonical map[string]string) []schemaField {
	var fields []schemaField
	for _, field := range structType.Fields.List {
		if field.Tag == nil {
			continue
		}
		tag, err := strconv.Unquote(field.Tag.Value)
		if err != nil {
			continue
		}
		jsonTag := reflect.StructTag(tag).Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		name, options, _ := strings.Cut(jsonTag, ",")
		fields = append(fields, schemaField{
			JSON:      name,
			Omitempty: strings.Contains(options, "omitempty"),
			Type:      normalizeSchemaType(field.Type, canonical),
		})
	}
	return fields
}

func normalizeSchemaType(expr ast.Expr, canonical map[string]string) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		if namedStringTypes[typed.Name] {
			return "string"
		}
		if key, ok := canonical[typed.Name]; ok {
			return "struct:" + key
		}
		return typed.Name
	case *ast.StarExpr:
		return "*" + normalizeSchemaType(typed.X, canonical)
	case *ast.ArrayType:
		return "[]" + normalizeSchemaType(typed.Elt, canonical)
	case *ast.MapType:
		return "map[" + normalizeSchemaType(typed.Key, canonical) + "]" + normalizeSchemaType(typed.Value, canonical)
	case *ast.SelectorExpr:
		if pkg, ok := typed.X.(*ast.Ident); ok {
			return pkg.Name + "." + typed.Sel.Name
		}
		return typed.Sel.Name
	case *ast.InterfaceType:
		return "any"
	}
	return "unknown"
}

func compareSchemaShapes(left, right map[string][]schemaField) error {
	keys := map[string]bool{}
	for key := range left {
		keys[key] = true
	}
	for key := range right {
		keys[key] = true
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)

	for _, key := range ordered {
		leftFields, okLeft := left[key]
		rightFields, okRight := right[key]
		if !okLeft {
			return fmt.Errorf("%s exists only in the mirror", key)
		}
		if !okRight {
			return fmt.Errorf("%s exists only in the authoritative shape", key)
		}
		if len(leftFields) != len(rightFields) {
			return fmt.Errorf("%s has %d fields upstream and %d in the mirror",
				key, len(leftFields), len(rightFields))
		}
		for index := range leftFields {
			if leftFields[index] != rightFields[index] {
				return fmt.Errorf("%s field %d: upstream %+v, mirror %+v",
					key, index, leftFields[index], rightFields[index])
			}
		}
	}
	if len(ordered) == 0 {
		return errors.New("no structs were walked; the parity guard would be vacuous")
	}
	return nil
}
