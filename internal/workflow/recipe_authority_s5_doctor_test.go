package workflow

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func rgaS5TreeState(t *testing.T, root string) string {
	t.Helper()
	var rows []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			rows = append(rows, fmt.Sprintf("%s:%s", rel, info.Mode()))
			return nil
		}
		var data []byte
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			data = []byte(target)
		} else {
			data, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}
		rows = append(rows, fmt.Sprintf("%s:%s:%d:%x", rel, info.Mode(), info.ModTime().UnixNano(), sha256.Sum256(data)))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(rows)
	return strings.Join(rows, "\n")
}

func TestRGAS5DoctorD10FixWritesNothingAndTruthfulCommand(t *testing.T) {
	s, in := rgaS5ReadFixture(t)
	if err := s.WriteArtifact("s5", "post-apply.patch", string(in.Observation.PatchBytes)); err != nil {
		t.Fatal(err)
	}
	for _, fix := range []bool{false, true} {
		before := rgaS5TreeState(t, s.Root)
		report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D10"}, Fix: fix})
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Findings) != 1 || report.Findings[0].Code != "recipe-coverage-missing" ||
			report.Findings[0].Fixable || report.Findings[0].Severity != "warning" || DoctorExitCode(report) != 0 {
			t.Fatalf("D10 outcome: %+v", report)
		}
		if report.Findings[0].Remediation != "tpatch record s5 --regenerate-recipe" {
			t.Fatalf("honest regeneration suppressed: %s", report.Findings[0].Remediation)
		}
		if after := rgaS5TreeState(t, s.Root); before != after {
			t.Fatal("D10 changed source/store/index/object bytes or file mtimes")
		}
	}
	// Default record is NOT a committed-range capture.
	mustGit(t, s.Root, "add", "a.txt")
	mustGit(t, s.Root, "commit", "-qm", "materialized")
	before := rgaS5TreeState(t, s.Root)
	report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D10"}, Fix: true})
	if err != nil || len(report.Findings) != 1 {
		t.Fatalf("%+v %v", report, err)
	}
	if !strings.Contains(report.Findings[0].Remediation, "no automatic truthful regeneration") ||
		strings.Contains(report.Findings[0].Remediation, "tpatch record ") {
		t.Fatal(report.Findings[0].Remediation)
	}
	if after := rgaS5TreeState(t, s.Root); before != after {
		t.Fatal("blocked D10 plan wrote files")
	}
}

func TestRGAS5PurePlannerExtractionBoundarySensitivity(t *testing.T) {
	src := rgaS0ReadRepoFile(t, "internal/workflow/recipe_autogen.go")
	if err := rgaS0CheckAutogenDerivation(src); err != nil {
		t.Fatal(err)
	}
	for _, wrong := range []string{
		`_, _ = os.ReadFile(obs.RepoRoot)`,
		`_ = ObserveCoveragePublication(s, obs)`,
		`_ = time.Now()`,
		`_ = writeRecipe(s, obs.Slug, nil)`,
		`_ = clearStaleMarker(s, obs.Slug)`,
		`_, _ = AutogenRecipeForRecord(s, obs, true, true)`,
	} {
		const anchor = "if publication.observationErr != nil {"
		mutated := strings.Replace(src, anchor, wrong+"\n"+anchor, 1)
		if mutated == src || rgaS0CheckAutogenDerivation(mutated) == nil {
			t.Fatal("same pure-plan validator accepted " + wrong)
		}
	}
	mutated := strings.Replace(src, "outcome, retErr = PlanRecipeForRecord(", "outcome, retErr = guessedRecipePlan(", 1)
	if mutated == src || rgaS0CheckAutogenDerivation(mutated) == nil {
		t.Fatal("compatibility wrapper bypassed pure planner undetected")
	}
}

func TestRGAS5ReadonlySourceBoundarySensitivity(t *testing.T) {
	for path := range rgaS5ReadFunctions {
		src := rgaS0ReadRepoFile(t, path)
		if err := rgaS5ReadSource(path, src); err != nil {
			t.Fatalf("positive %s: %v", path, err)
		}

	}
	path := "internal/workflow/doctor_d10.go"
	src := rgaS0ReadRepoFile(t, path)
	anchor := "func runDoctorD10(ctx *doctorContext) {"
	for _, wrong := range []string{
		`ctx.store.WriteArtifactAtomic("s5", "recipe-coverage.json", "{}")`,
		`writer := ctx.store.WriteFeatureFile; _ = writer`,
		`writer := (*store.Store).WriteFeatureFile; _ = writer`,
		`var writer = os.WriteFile; _ = writer`,
		`const target = "artifacts/" + "recipe-coverage.json"; ctx.store.WriteFeatureFile("s5", target, "{}")`,
		`os.WriteFile("artifacts/recipe-\x63overage.json", nil, 0600)`,
		`gitutil.NewTempIndex(ctx.root, ctx.root)`,
		`gitutil.ValidateStagedPatch(ctx.root, "")`,
		`gitutil.ReverseApplyCheckAtHEAD(ctx.root, "")`,
		`AutogenRecipeForRecord(ctx.store, obs, true, true)`,
		`PublishCoverage(ctx.store, in)`,
	} {
		mutated := strings.Replace(src, anchor, anchor+"\n"+wrong, 1)
		if mutated == src || rgaS5ReadSource(path, mutated) == nil {
			t.Fatalf("same readonly validator accepted %s", wrong)
		}
		if rgaS0CoveragePhaseSource(path, mutated) == nil {
			t.Fatalf("outer phase validator accepted %s", wrong)
		}
	}
}
