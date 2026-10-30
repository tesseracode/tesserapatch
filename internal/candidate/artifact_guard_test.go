package candidate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func validateS5ArtifactSchemaSource(source string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "artifact.go", source, 0)
	if err != nil {
		return err
	}
	structFields := map[string][]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, field := range st.Fields.List {
				for _, name := range field.Names {
					structFields[ts.Name.Name] = append(structFields[ts.Name.Name], name.Name)
				}
			}
		}
	}
	if got := strings.Join(structFields["CandidateArtifact"], ","); got != "SchemaVersion,CandidateID,Feature,SourceFeatureState,UpstreamCommit,ReferenceCommit,DerivationMode,Bindings,Derivation,Result" {
		return fmt.Errorf("candidate artifact immutable field set changed: %s", got)
	}
	if got := strings.Join(structFields["CandidateState"], ","); got != "SchemaVersion,CandidateID,Status,AcceptValidation" {
		return fmt.Errorf("candidate state field set changed: %s", got)
	}
	for _, forbidden := range []string{"Payload", "Boundary", "Left", "Right", "ByteOffset", "BaseRecordCount", "UpstreamRecordCount", "AlignmentLength", "InsertionRecordCount"} {
		for _, field := range structFields["CandidateOperation"] {
			if field == forbidden {
				return fmt.Errorf("candidate operation leaked ephemeral execution metadata %s", forbidden)
			}
		}
	}
	for _, forbidden := range []string{"Status", "Timestamp"} {
		for _, field := range structFields["CandidateArtifact"] {
			if field == forbidden {
				return fmt.Errorf("candidate artifact leaked mutable field %s", forbidden)
			}
		}
	}
	return nil
}

func TestS5ArtifactSchemaGuardAndSensitivity(t *testing.T) {
	body, err := os.ReadFile("artifact.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	if err := validateS5ArtifactSchemaSource(source); err != nil {
		t.Fatal(err)
	}
	mutant := strings.Replace(source, "type CandidateArtifact struct {\n", "type CandidateArtifact struct {\n\tTimestamp string `json:\"timestamp\"`\n", 1)
	if mutant == source {
		t.Fatal("artifact guard mutation anchor missing")
	}
	if err := validateS5ArtifactSchemaSource(mutant); err == nil {
		t.Fatal("artifact guard accepted mutable-field leak")
	}
}
