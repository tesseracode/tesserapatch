package workflow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestDoctorD7CleanRecipe(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	addFeatureWithRecipe(t, s, "clean", `{
  "feature": "clean",
  "operations": [
    {"type": "ensure-directory", "path": "src"}
  ]
}
`)
	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D7"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Findings != 0 || report.Summary.Errors != 0 {
		t.Fatalf("clean recipe reported drift: %#v %#v", report.Summary, report.Findings)
	}
}

func TestDoctorD7ReportsSchemaDriftClasses(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	addFeatureWithRecipe(t, s, "missing-feature", `{
  "operations": [
    {"type": "ensure-directory", "path": "src"}
  ]
}
`)
	addFeatureWithRecipe(t, s, "unknown-field", `{
  "feature": "unknown-field",
  "version": 1,
  "operations": [
    {"type": "ensure-directory", "path": "src"}
  ]
}
`)
	addFeatureWithRecipe(t, s, "bad-type", `{
  "feature": "bad-type",
  "operations": "not-an-array"
}
`)
	addFeatureWithRecipe(t, s, "malformed", "{\n  \"feature\": \"malformed\",\n  \"operations\": [\n")

	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D7"}})
	if err != nil {
		t.Fatal(err)
	}
	missing := assertFinding(t, report, "D7", "recipe-missing-feature", "missing-feature")
	if missing.Line != 1 || missing.Fixable {
		t.Fatalf("missing feature finding = %#v", missing)
	}
	unknown := assertFinding(t, report, "D7", "recipe-unknown-field", "unknown-field")
	if unknown.Line != 3 {
		t.Fatalf("unknown field line = %d, want 3", unknown.Line)
	}
	badType := assertFinding(t, report, "D7", "recipe-field-type-invalid", "bad-type")
	if badType.Line != 3 {
		t.Fatalf("bad type line = %d, want 3", badType.Line)
	}
	malformed := assertFinding(t, report, "D7", "recipe-parse-error", "malformed")
	if malformed.Line != 3 {
		t.Fatalf("malformed line = %d, want 3", malformed.Line)
	}
	if malformed.Remediation != "hand-fix apply-recipe.json to the current workflow.ApplyRecipe schema, verify with 'tpatch verify malformed', or regenerate with 'tpatch implement malformed'" {
		t.Fatalf("unexpected remediation: %q", malformed.Remediation)
	}
}

func TestDoctorD7ReportsInstalledSkillRecipeExampleDrift(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	asset := doctorSkillAssets(root)[4]
	writeFile(t, asset.Dst, []byte(`# tpatch example

`+"```json"+`
{
  "feature": "skill",
  "version": 1,
  "operations": [
    {"type": "ensure-directory", "path": "src"}
  ]
}
`+"```"+`
`))
	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D7"}})
	if err != nil {
		t.Fatal(err)
	}
	f := assertFinding(t, report, "D7", "skill-recipe-schema-drift", "")
	if f.Path != relOrAbs(root, asset.Dst) || f.Line != 6 {
		t.Fatalf("skill recipe finding = %#v", f)
	}
}

func addFeatureWithRecipe(t *testing.T, s *store.Store, slug, recipe string) {
	t.Helper()
	if _, err := s.AddFeature(store.AddFeatureInput{Title: slug, Slug: slug, Request: slug}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Root, ".tpatch", "features", slug, "artifacts", "apply-recipe.json")
	if err := os.WriteFile(path, []byte(recipe), 0o644); err != nil {
		t.Fatal(err)
	}
}
