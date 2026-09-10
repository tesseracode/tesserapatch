package workflow

import (
	"errors"
	"go/ast"
	"go/token"
	"strings"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func rgaS5SharedPlannerSource(cli, planner, doctor, verify string) error {
	file, err := rgaS0Parse("cobra.go", cli)
	if err != nil {
		return err
	}
	fn := rgaS0FuncBody(file, "recordCmd")
	if fn == nil {
		return errors.New("real record command missing")
	}
	planned, gated, outcome, recipeError := false, false, false, false
	firstWrite := fn.End()
	ast.Inspect(fn, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if rgaS0CallName(call) == "workflow.PlanRecord" {
				planned = true
			}
			if rgaS0CallName(call) == "s.WriteArtifactAtomic" && call.Pos() < firstWrite {
				firstWrite = call.Pos()
			}
		}
		if field, ok := n.(*ast.SelectorExpr); ok {
			if owner, ok := field.X.(*ast.Ident); ok && owner.Name == "recordPlan" {
				outcome = outcome || field.Sel.Name == "Autogen"
				recipeError = recipeError || field.Sel.Name == "RecipeError"
			}
		}
		return true
	})
	ast.Inspect(fn, func(n ast.Node) bool {
		branch, ok := n.(*ast.IfStmt)
		if !ok || branch.Pos() >= firstWrite {
			return true
		}
		assign, ok := branch.Init.(*ast.AssignStmt)
		if !ok || len(assign.Rhs) != 1 || len(assign.Lhs) != 1 {
			return true
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok || rgaS0CallName(call) != "recordPlan.RecordGateError" {
			return true
		}
		cause, ok := assign.Lhs[0].(*ast.Ident)
		if !ok {
			return true
		}
		condition, ok := branch.Cond.(*ast.BinaryExpr)
		if !ok || condition.Op != token.NEQ {
			return true
		}
		left, leftOK := condition.X.(*ast.Ident)
		right, rightOK := condition.Y.(*ast.Ident)
		if !leftOK || !rightOK || left.Name != cause.Name || right.Name != "nil" {
			return true
		}
		for _, statement := range branch.Body.List {
			if ret, ok := statement.(*ast.ReturnStmt); ok && len(ret.Results) == 1 {
				if id, ok := ret.Results[0].(*ast.Ident); ok && id.Name == cause.Name {
					gated = true
				}
			}
		}
		return true
	})
	if !planned || !gated || !outcome || !recipeError {
		return errors.New("real record bypasses shared planning, pre-write gates or planned recipe outcome")
	}
	for _, part := range []struct{ source, fn, call string }{
		{planner, "PlanDefaultRecord", "PlanRecord"},
		{doctor, "runDoctorD10", "PlanDefaultRecord"},
		{verify, "checkRecipeGenerationCoverage", "PlanDefaultRecord"},
	} {
		file, err := rgaS0Parse("read-planner.go", part.source)
		if err != nil {
			return err
		}
		fn := rgaS0FuncBody(file, part.fn)
		if fn == nil {
			return errors.New("shared planning caller missing")
		}
		found := false
		ast.Inspect(fn, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok && rgaS0CallName(call) == part.call {
				found = true
			}
			return true
		})
		if !found {
			return errors.New("diagnostic bypasses actual record planner")
		}
	}
	return nil
}

func TestRGAS5SharedPlannerSourceAndSensitivity(t *testing.T) {
	inputs := []string{
		rgaS0ReadRepoFile(t, "internal/cli/cobra.go"),
		rgaS0ReadRepoFile(t, "internal/workflow/record_plan.go"),
		rgaS0ReadRepoFile(t, "internal/workflow/doctor_d10.go"),
		rgaS0ReadRepoFile(t, "internal/workflow/recipe_coverage_diagnostics.go"),
	}
	validate := func(in []string) error { return rgaS5SharedPlannerSource(in[0], in[1], in[2], in[3]) }
	if err := validate(inputs); err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct {
		index            int
		old, replacement string
	}{
		{0, "recordPlan.RecordGateError(", "ignoredPlan.RecordGateError("},
		{0, "recordPlan.RecordGateError(forceAmend, allowCollisionReason, lenient, stagedFlag); err != nil", "recordPlan.RecordGateError(forceAmend, allowCollisionReason, lenient, stagedFlag); err == nil"},
		{0, "recordPlan.Autogen, recordPlan.RecipeError", "unrelated.Autogen, unrelated.RecipeError"},
		{1, "return PlanRecord(", "return guessedRecordPlan("},
		{2, "plan := PlanDefaultRecord(", "plan := guessedRecordPlan("},
		{3, "plan := PlanDefaultRecord(", "plan := guessedRecordPlan("},
	} {
		wrong := append([]string{}, inputs...)
		wrong[change.index] = strings.Replace(wrong[change.index], change.old, change.replacement, 1)
		if wrong[change.index] == inputs[change.index] || validate(wrong) == nil {
			t.Fatal("same shared-planner validator accepted " + change.old)
		}
	}
}

func TestRGAS5SharedRecordPlanAndDryRecommendation(t *testing.T) {
	s, _ := rgaS5ReadFixture(t)
	status, err := s.LoadFeatureStatus("s5")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := SnapshotRecipeCoverage(s.Root, "s5")
	now := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	dry := PlanDefaultRecord(s, status, snapshot, now)
	if got := dry.Remediation("s5"); got != "tpatch record s5 --regenerate-recipe" {
		t.Fatalf("honest default capture not recommended: %s (recipe error %v)", got, dry.RecipeError)
	}
	actual := PlanRecord(s, status, dry.Observation, dry.Publication, true, true, now)
	if actual.RecipeError != nil || actual.Remediation("s5") != dry.Remediation("s5") ||
		string(actual.Autogen.Recipe.Bytes) != string(dry.Autogen.Recipe.Bytes) {
		t.Fatal("actual and diagnostic record plans diverged")
	}
	validate := func(plan RecordPlan, wantCommand bool) error {
		gotCommand := strings.HasPrefix(plan.Remediation("s5"), "tpatch record ")
		if gotCommand != wantCommand {
			return errors.New("recommendation disagrees with observed producer feasibility")
		}
		return nil
	}
	if err := validate(dry, true); err != nil {
		t.Fatal(err)
	}
	if err := validate(RecordPlan{}, true); err == nil {
		t.Fatal("same validator accepted an unproved recommendation")
	}
	if err := validate(RecordPlan{}, false); err != nil {
		t.Fatal(err)
	}
	for _, blocker := range []string{"pending baseline", "amend orphans", "cross-feature collision", "empty capture", "roundtrip failure", "diffstat failure", "publication path"} {
		wrong := dry
		wrong.Blockers = []string{blocker}
		if err := validate(wrong, false); err != nil {
			t.Fatal(err)
		}
		if err := validate(wrong, true); err == nil {
			t.Fatalf("same validator ignored %s", blocker)
		}
	}
	// An actual second feature collision affects BOTH uses of the plan.
	if _, err := s.AddFeature(store.AddFeatureInput{Slug: "collision", Title: "collision", Request: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact("collision", "post-apply.patch", string(dry.Observation.PatchBytes)); err != nil {
		t.Fatal(err)
	}
	dry = PlanDefaultRecord(s, status, snapshot, now)
	actual = PlanRecord(s, status, dry.Observation, dry.Publication, true, true, now)
	for _, plan := range []RecordPlan{dry, actual} {
		if err := validate(plan, false); err != nil || !strings.Contains(plan.Remediation("s5"), "cross-feature") {
			t.Fatalf("collision escaped shared producer plan: %s", plan.Remediation("s5"))
		}
		if err := plan.RecordGateError(false, "", false, false); err == nil {
			t.Fatal("actual producer ignored the shared collision gate")
		}
		if err := plan.RecordGateError(false, "intentional duplicate", false, false); err != nil {
			t.Fatal("existing explicit collision override changed")
		}
	}
}

func TestRGAS5RecordGateFailuresCannotBecomeDryCommands(t *testing.T) {
	s, _ := rgaS5ReadFixture(t)
	status, err := s.LoadFeatureStatus("s5")
	if err != nil {
		t.Fatal(err)
	}
	base := PlanDefaultRecord(s, status, SnapshotRecipeCoverage(s.Root, "s5"), time.Now().UTC())
	if len(base.Blockers) != 0 || base.RecordGateError(false, "", false, false) != nil {
		t.Fatal(base.Blockers)
	}
	validate := func(plan RecordPlan) error {
		if strings.HasPrefix(plan.Remediation("s5"), "tpatch record ") ||
			plan.RecordGateError(false, "", false, false) == nil {
			return errors.New("known gate bypassed by recommendation or actual producer")
		}
		return nil
	}
	for _, kind := range []string{"identity", "amend", "empty", "capture", "generations", "collision-read", "collision", "roundtrip", "diffstat", "publication-path"} {
		blocked := base
		blocked.Blockers = []string{kind}
		blocked.gateFailures = []recordGateFailure{{kind, kind}}
		if err := validate(blocked); err != nil {
			t.Fatal(kind, err)
		}
		if kind == "publication-path" && blocked.RecordGateError(true, "explicit duplicate", true, true) == nil {
			t.Fatal("publication path gate was bypassed by producer override flags")
		}
		wrong := blocked
		wrong.gateFailures = nil
		if err := validate(wrong); err == nil {
			t.Fatal("same gate validator accepted producer bypass: " + kind)
		}
		wrong = blocked
		wrong.Blockers = nil
		wrong.gateFailures = nil
		if err := validate(wrong); err == nil {
			t.Fatal("same gate validator accepted dry recommendation bypass: " + kind)
		}
	}
}

func TestRGAS5RecipePlanningIsPureAndKeepsNoopTime(t *testing.T) {
	s, in := rgaS5ReadFixture(t)
	publication := ObserveCoveragePublication(s, in.Observation)
	before := rgaS5TreeState(t, s.Root)
	now := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	first, err := PlanRecipeForRecord(in.Observation, true, true, publication, now)
	if err != nil || first.Action != AutogenGenerated {
		t.Fatalf("%s %v", first.Action, err)
	}
	if after := rgaS5TreeState(t, s.Root); after != before {
		t.Fatal("pure recipe plan wrote files")
	}
	publication.Recipe = first.Recipe
	publication.Provenance = CoverageArtifact{Present: true, Bytes: first.plan.provenance}
	second, err := PlanRecipeForRecord(in.Observation, true, false, publication, now.Add(time.Hour))
	if err != nil || second.Action != AutogenNoop || second.plan.provenance != nil {
		t.Fatalf("noop churned provenance: %s %v", second.Action, err)
	}
}
