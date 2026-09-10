package workflow

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestRGAS5DoctorMissingCoverageRequiresReadablePair(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		patch, recipe, coverage bool
		unreadable              string
		want                    int
	}{
		{name: "neither"},
		{name: "patch-only", patch: true},
		{name: "recipe-only", recipe: true},
		{name: "both", patch: true, recipe: true, want: 1},
		{name: "patch-unreadable", patch: true, recipe: true, unreadable: "post-apply.patch"},
		{name: "recipe-unreadable", patch: true, recipe: true, unreadable: "apply-recipe.json"},
		{name: "malformed-coverage-without-pair", coverage: true, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := store.Init(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.AddFeature(store.AddFeatureInput{Title: "D10 cohort", Slug: "cohort"}); err != nil {
				t.Fatal(err)
			}
			for name, present := range map[string]bool{
				"post-apply.patch": tc.patch, "apply-recipe.json": tc.recipe, "recipe-coverage.json": tc.coverage,
			} {
				if present {
					if err := s.WriteArtifact("cohort", name, "fixture bytes\n"); err != nil {
						t.Fatal(err)
					}
				}
			}
			read := ReadRecipeArtifact
			ReadRecipeArtifact = func(path string) RecipeArtifactRead {
				if filepath.Base(path) == tc.unreadable {
					return RecipeArtifactRead{Exists: true, Err: fs.ErrPermission}
				}
				return read(path)
			}
			t.Cleanup(func() { ReadRecipeArtifact = read })
			report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D10"}})
			if err != nil || len(report.Findings) != tc.want {
				t.Fatalf("missing-coverage cohort: findings=%+v err=%v", report.Findings, err)
			}
			for _, finding := range report.Findings {
				code := "recipe-coverage-missing"
				if tc.coverage {
					code = "recipe-coverage-malformed"
				}
				if finding.Code != code || finding.Severity != "warning" || finding.Fixable {
					t.Fatalf("cohort filter hid or promoted explicit coverage evidence: %+v", finding)
				}
			}
		})
	}
}

func rgaS5ReadOnlyGitWrapper(t *testing.T) *gitWrapper {
	t.Helper()
	wrapper := installGitWrapper(t)
	path := filepath.Join(wrapper.dir, "git")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const field = `\037GIT_INDEX_FILE=%s\n`
	const argument = `"${GIT_INDEX_FILE-}"`
	if strings.Count(string(raw), field) != 1 || strings.Count(string(raw), argument) != 1 {
		t.Fatal("readonly envelope instrumentation no longer matches the real Git wrapper")
	}
	script := strings.Replace(string(raw), field, `\037GIT_INDEX_FILE=%s\037GIT_OPTIONAL_LOCKS=%s\n`, 1)
	script = strings.Replace(script, argument, argument+` "${GIT_OPTIONAL_LOCKS-}"`, 1)
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	return wrapper
}

func rgaS5GitCallPrefix(call gitCall, prefix ...string) bool {
	if len(call.Args) < len(prefix) {
		return false
	}
	for i, argument := range prefix {
		if call.Args[i] != argument {
			return false
		}
	}
	return true
}

func rgaS5ReadOnlyGitCalls(calls []gitCall) error {
	if len(calls) == 0 {
		return fmt.Errorf("readonly capture issued no observed Git calls")
	}
	for _, call := range calls {
		for key, value := range map[string]string{
			"GIT_NO_LAZY_FETCH": "1", "LC_ALL": "C", "GIT_OPTIONAL_LOCKS": "0",
		} {
			if call.Env[key] != value {
				return fmt.Errorf("git %s received %s=%q, want %q", call.Joined(), key, call.Env[key], value)
			}
		}
	}
	return nil
}

func TestRGAS5Rung3AndDoctorCaptureGitEnvelope(t *testing.T) {
	s, in := rgaS5ReadFixture(t)
	in.Events.StaleMarkerPresent = true
	snapshot := rgaS5ReadSnapshot(t, in)
	for name, artifact := range map[string]RecipeArtifactRead{
		"post-apply.patch": snapshot.Patch, "apply-recipe.json": snapshot.Recipe,
		"recipe-coverage.json": snapshot.Coverage, "recipe-capture-event.json": snapshot.Event,
	} {
		if err := s.WriteArtifact("s5", name, string(artifact.Bytes)); err != nil {
			t.Fatal(err)
		}
	}
	inventory, err := buildInventory(s)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("LC_ALL", "fr_FR.UTF-8")
	t.Setenv("GIT_NO_LAZY_FETCH", "0")
	t.Setenv("GIT_OPTIONAL_LOCKS", "1")
	wrapper := rgaS5ReadOnlyGitWrapper(t)
	for _, consumer := range []string{"verify-rung3", "doctor-d10-fix"} {
		t.Run(consumer, func(t *testing.T) {
			wrapper.Reset()
			if consumer == "verify-rung3" {
				ctx := &verifyRunContext{root: s.Root, floorOK: true, inv: inventory}
				row := checkRecipeGenerationCoverage(ctx, s, "s5")
				if row.Passed || row.Severity != SeverityWarn ||
					!strings.Contains(row.Remediation, "tpatch record s5 --regenerate-recipe") {
					t.Fatalf("genuine rung-3 planning was suppressed: %+v", row)
				}
			} else {
				report, err := RunDoctor(s, DoctorOptions{Checks: []string{"D10"}, Fix: true})
				if err != nil || len(report.Findings) != 1 || report.Findings[0].Fixable ||
					report.Findings[0].Remediation != "tpatch record s5 --regenerate-recipe" {
					t.Fatalf("genuine doctor planning was suppressed: %+v %v", report, err)
				}
			}
			calls := wrapper.Calls()
			if err := rgaS5ReadOnlyGitCalls(calls); err != nil {
				t.Fatal(err)
			}
			for _, prefix := range [][]string{
				{"config", "--get-regexp"}, {"worktree", "list", "--porcelain", "-z"},
				{"diff", "--no-ext-diff", "--no-textconv"}, {"rev-parse", "--verify", "HEAD"}, {"diff", "--stat"},
				{"rev-parse", "--verify", in.Observation.Reference.Commit + "^{commit}"},
				{"--literal-pathspecs", "ls-tree", "-z", "--full-name"}, {"cat-file", "--batch"},
				{"-c", "core.quotePath=false", "ls-files", "--others"}, {"apply", "--reverse", "--check", "-"},
			} {
				found := false
				for _, call := range calls {
					found = found || rgaS5GitCallPrefix(call, prefix...)
				}
				if !found {
					t.Fatalf("actual readonly capture missed %v", prefix)
				}
			}
		})
	}
	for key, bad := range map[string]string{
		"GIT_NO_LAZY_FETCH": "0", "LC_ALL": "fr_FR.UTF-8", "GIT_OPTIONAL_LOCKS": "1",
	} {
		t.Run("bad-"+key, func(t *testing.T) {
			wrapper.Reset()
			command := exec.Command("git", "rev-parse", "--verify", "HEAD")
			command.Dir = s.Root
			command.Env = append(gitutil.CaptureReadOnlyEnv(), key+"="+bad)
			if _, err := command.Output(); err != nil {
				t.Fatal(err)
			}
			if err := rgaS5ReadOnlyGitCalls(wrapper.Calls()); err == nil {
				t.Fatalf("same actual-call validator accepted bad %s", key)
			}
		})
	}
}

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
	if err := s.WriteArtifact("s5", "apply-recipe.json", string(in.Recipe.Bytes)); err != nil {
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

func TestRGAS5ReadonlyOpeningCapabilitiesAndAliases(t *testing.T) {
	path := "internal/workflow/doctor_d10.go"
	original := rgaS0ReadRepoFile(t, path)
	const anchor = "func runDoctorD10(ctx *doctorContext) {"
	validate := func(source string) error {
		if err := rgaS5ReadSource(path, source); err != nil {
			return err
		}
		return rgaS0CoveragePhaseSource(path, source)
	}
	withImports := strings.Replace(original, `"fmt"`, `"fmt"
	"os"
	fs "os"
	"syscall"
	"strings"`, 1)
	if withImports == original {
		t.Fatal("actual-source import anchor missing")
	}
	mutate := func(body, declaration string) string {
		return strings.Replace(withImports, anchor, anchor+"\n"+body, 1) + "\n" + declaration
	}
	if err := validate(original); err != nil {
		t.Fatal(err)
	}
	for _, safe := range []string{
		`f, _ := os.Open(ctx.root + "/.tpatch/config.yaml"); if f != nil { f.Close() }`,
		`open := os.Open; f, _ := open(ctx.root + "/.tpatch/config.yaml"); if f != nil { f.Close() }`,
		`f, _ := os.OpenFile(ctx.root + "/.tpatch/config.yaml", os.O_RDONLY, 0); if f != nil { f.Close() }`,
		`const flags = os.O_RDONLY | 0; f, _ := os.OpenFile(ctx.root + "/.tpatch/config.yaml", flags, 0); if f != nil { f.Close() }`,
		`f, _ := fs.OpenFile(ctx.root + "/.tpatch/config.yaml", fs.O_RDONLY, 0); if f != nil { f.Close() }`,
		`var b strings.Builder; b.WriteString("in-memory"); _ = b.String()`,
	} {
		if err := validate(mutate(safe, "")); err != nil {
			t.Fatalf("legitimate readonly/builder operation refused: %s: %v", safe, err)
		}
	}
	for _, wrong := range []struct{ name, body, declaration string }{
		{"direct-truncate", `_, _ = os.OpenFile(ctx.root+"/.tpatch/features/s5/artifacts/recipe-coverage.json", os.O_WRONLY|os.O_TRUNC, 0600)`, ""},
		{"read-mode-but-truncate", `_, _ = os.OpenFile(ctx.root+"/.tpatch/features/s5/artifacts/recipe-coverage.json", os.O_RDONLY|os.O_TRUNC, 0600)`, ""},
		{"local-function-alias", `open := os.OpenFile; _, _ = open(ctx.root+"/.tpatch/features/s5/artifacts/recipe-coverage.json", os.O_RDWR|os.O_TRUNC, 0600)`, ""},
		{"package-function-alias", `_, _ = s5WriteOpen(ctx.root+"/.tpatch/features/s5/artifacts/recipe-coverage.json", os.O_TRUNC, 0600)`, `var s5WriteOpen = os.OpenFile`},
		{"package-import-alias", `_, _ = fs.OpenFile(ctx.root+"/.tpatch/features/s5/artifacts/recipe-coverage.json", fs.O_CREATE|fs.O_RDWR, 0600)`, ""},
		{"unknown-flags", `flags := os.O_RDONLY; _, _ = os.OpenFile(ctx.root+"/.tpatch/config.yaml", flags, 0)`, ""},
		{"numeric-unproved-flags", `_, _ = os.OpenFile(ctx.root+"/.tpatch/config.yaml", 512, 0600)`, ""},
		{"shadowed-readonly-name", `os := struct{ O_RDONLY int }{fs.O_TRUNC}; _, _ = fs.OpenFile(ctx.root+"/.tpatch/config.yaml", os.O_RDONLY, 0600)`, ""},
		{"root-method-value", `root, _ := os.OpenRoot(ctx.root); opening := root.OpenFile; _, _ = opening(".tpatch/config.yaml", os.O_TRUNC, 0600)`, ""},
		{"root-method-expression", `opening := (*os.Root).OpenFile; _ = opening`, ""},
		{"syscall-open", `_, _ = syscall.Open(ctx.root+"/.tpatch/config.yaml", syscall.O_RDWR|syscall.O_TRUNC, 0600)`, ""},
		{"syscall-openat", `_, _ = syscall.Openat(0, ".tpatch/config.yaml", syscall.O_CREAT, 0600)`, ""},
		{"truncate-existing-handle", `f, _ := os.Open(ctx.root+"/.tpatch/config.yaml"); if f != nil { _ = f.Truncate(0) }`, ""},
		{"mutate-readonly-handle-metadata", `f, _ := os.Open(ctx.root+"/.tpatch/config.yaml"); if f != nil { _ = f.Chmod(0600) }`, ""},
		{"unknown-descriptor", `f := os.NewFile(7, ".tpatch/config.yaml"); _ = f`, ""},
		{"creat-constructor", `_, _ = syscall.Creat(ctx.root+"/.tpatch/config.yaml", 0600)`, ""},
	} {
		t.Run(wrong.name, func(t *testing.T) {
			source := mutate(wrong.body, wrong.declaration)
			if source == withImports {
				t.Fatal("actual-source capability mutation did not apply")
			}
			if rgaS5ReadSource(path, source) == nil {
				t.Fatal("readonly validator accepted write-opening capability")
			}
			if rgaS0CoveragePhaseSource(path, source) == nil {
				t.Fatal("phase validator accepted write-opening capability")
			}
		})
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
