package workflow

import (
	"fmt"
	"strings"
	"testing"
)

func rgaS4ReviewBoundaryName(src string) error {
	file, err := rgaS0Parse("recipe_authority_s0_source_guards_test.go", src)
	if err != nil {
		return err
	}
	if rgaS0FuncBody(file, "TestRGAS0CoveragePhaseBoundaryHolds") == nil {
		return fmt.Errorf("N1: phase-boundary test declaration is missing")
	}
	if rgaS0FuncBody(file, "TestRGAS0NoProductionCoverageSurfaceYet") != nil {
		return fmt.Errorf("N1: obsolete absence-test declaration remains")
	}
	if rgaS0FuncBody(file, "TestRGAS0CoveragePhaseBoundaryIsSensitive") == nil {
		return fmt.Errorf("N2: original sensitivity companion is missing")
	}
	return nil
}

func TestRGAS4ReviewN1BoundaryNameAndSensitivity(t *testing.T) {
	src := rgaS0ReadRepoFile(t, "internal/workflow/recipe_authority_s0_source_guards_test.go")
	if err := rgaS4ReviewBoundaryName(src); err != nil {
		t.Fatal(err)
	}
	const current = "func TestRGAS0CoveragePhaseBoundaryHolds("
	if !strings.Contains(src, current) {
		t.Fatal("name mutation anchor is missing")
	}
	for _, name := range []string{
		"TestRGAS0NoProductionCoverageSurfaceYet",
		"TestAnUnrelatedBoundary",
	} {
		wrong := strings.Replace(src, current, "func "+name+"(", 1)
		if err := rgaS4ReviewBoundaryName(wrong); err == nil {
			t.Fatalf("same name validator accepted %s", name)
		}
	}
	const companion = "func TestRGAS0CoveragePhaseBoundaryIsSensitive("
	if !strings.Contains(src, companion) {
		t.Fatal("companion mutation anchor is missing")
	}
	if err := rgaS4ReviewBoundaryName(strings.Replace(src, companion, "func TestUnrelatedSensitivity(", 1)); err == nil {
		t.Fatal("same name validator accepted removal of the sensitivity companion")
	}
}

// These are the eleven S3 mutations accepted by the external review, kept
// independently of the implementation agent's retargeted S0 test table.
func TestRGAS4ReviewN2AllElevenPureCoreMutationsRemainBiting(t *testing.T) {
	const rel = "internal/workflow/recipe_coverage.go"
	src := rgaS0ReadRepoFile(t, rel)
	if err := rgaS0CoveragePhaseSource(rel, src); err != nil {
		t.Fatalf("the unchanged pure core is rejected: %v", err)
	}
	if !strings.Contains(src, `"bytes"`) {
		t.Fatal("os-import mutation anchor is missing")
	}
	mutations := []struct{ name, path, source string }{
		{"different-file", "internal/workflow/record.go", src},
		{"os-import", rel, strings.Replace(src, `"bytes"`, `"bytes"`+"\n\"os\"", 1)},
		{"store-New", rel, src + "\nfunc plantedPublisher() { store.New(\"root\") }\n"},
		{"observation", rel, src + "\nfunc plantedRead() { patchobs.Observe(patchobs.Input{}) }\n"},
		{"artifact-snapshot", rel, src + "\nfunc plantedSnapshot() { patchobs.SnapshotArtifact(\"bound\") }\n"},
		{"Git-call", rel, src + "\nfunc plantedGit() { gitutil.Run(\"root\", \"status\") }\n"},
		{"execution", rel, src + "\nfunc plantedExecution() { ExecuteRecipe(nil, ApplyRecipe{}) }\n"},
		{"spaced-existing-writer", rel, src + "\nfunc plantedExistingWriter() { writeRecipe (nil, \"s3\", ApplyRecipe{}) }\n"},
		{"artifact-path-constant", rel, src + "\nconst plantedArtifact = \"recipe-coverage.json\"\n"},
		{"CWD", rel, src + "\nfunc plantedCWD() { filepath.Abs(\".\") }\n"},
		{"spaced-AST-CWD", rel, src + "\nfunc plantedSpacedCWD() { filepath . Abs (\".\") }\n"},
	}
	if len(mutations) != 11 {
		t.Fatal("external review mutation inventory changed without disposition")
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			if mutation.path == rel && mutation.source == src {
				t.Fatal("mutation did not alter its guarded input")
			}
			if err := rgaS0CoveragePhaseSource(mutation.path, mutation.source); err == nil {
				t.Fatal("retargeted validator no longer rejects an original S3 mutation")
			}
		})
	}
}

func TestRGAS4ReviewN2PublisherPermissionIsFileScoped(t *testing.T) {
	const publisher = "internal/workflow/recipe_coverage_publish.go"
	src := rgaS0ReadRepoFile(t, publisher)
	if err := rgaS0CoveragePhaseSource(publisher, src); err != nil {
		t.Fatalf("designated publisher rejected: %v", err)
	}
	for _, rel := range []string{
		"internal/workflow/recipe_coverage_shadow.go",
		"internal/workflow/recipe_coverage.go",
		"internal/cli/cobra.go",
	} {
		t.Run(rel, func(t *testing.T) {
			if err := rgaS0CoveragePhaseSource(rel, src); err == nil {
				t.Fatal("the exact publisher source acquired permission in another file")
			}
		})
	}
}

func TestRGAS4ReviewN3ConservativeDomainDoesNotWiden(t *testing.T) {
	empty := ""
	existing := "sha256:" + strings.Repeat("a", 64)
	for _, gate := range []*string{&empty, &existing} {
		if !coverageAdmissibleReclassification(RecipeOperation{Type: "write-file", PreimageHash: gate}) {
			t.Fatal("supported gated fixed write lost its admissible operation class")
		}
	}
	for _, op := range []RecipeOperation{
		{Type: "write-file"},
		{Type: "replace-in-file", PreimageHash: &existing, Search: "before", Replace: "after"},
		{Type: "append-file", PreimageHash: &existing, Content: "after"},
		{Type: "ensure-directory", PreimageHash: &empty},
	} {
		if coverageAdmissibleReclassification(op) {
			t.Fatalf("S4 widened the operation domain deferred to GH #24: %s", op.Type)
		}
	}
}
