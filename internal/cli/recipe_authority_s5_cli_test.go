package cli

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

const rgaS5CLIPatch = "diff --git a/a.txt b/a.txt\nindex 3367afd..3e75765 100644\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-old\n+new\n"

func rgaS5CLISetup(t *testing.T, recipeKind string, legacy bool) *store.Store {
	t.Helper()
	root := t.TempDir()
	gitInitTestRepo(t, root)
	s, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Slug: "s5", Title: "s5", Request: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkFeatureState("s5", store.StateApplied, "record", ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "a.txt")
	gitRun(t, root, "commit", "-qm", "reference")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("new\n"), 0644); err != nil {
		t.Fatal(err)
	}
	obs := patchobs.Observe(patchobs.Input{RepoRoot: root, Slug: "s5", Producer: patchobs.ProducerRecord,
		Patch: rgaS5CLIPatch, PatchPresent: true, PreimageRef: "HEAD",
		Capture: patchobs.CaptureDescriptor{Mode: patchobs.CaptureModeWorkingTreeAll}})
	recipe := workflow.CoverageArtifact{}
	if recipeKind != "missing" {
		raw := []byte("{broken")
		if recipeKind != "bad" {
			gate := "sha256:" + workflow.CoverageSHA256([]byte("old\n"))
			raw, err = workflow.EncodeRecipe(workflow.ApplyRecipe{Feature: "s5", Operations: []workflow.RecipeOperation{
				{Type: "write-file", Path: "a.txt", Content: "new\n", PreimageHash: &gate},
			}})
			if err != nil {
				t.Fatal(err)
			}
		}
		recipe = workflow.CoverageArtifact{Present: true, Bytes: raw}
		if err := s.WriteArtifact("s5", "apply-recipe.json", string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.WriteArtifact("s5", "post-apply.patch", rgaS5CLIPatch); err != nil {
		t.Fatal(err)
	}
	if !legacy {
		_, err := workflow.PublishCoverage(s, workflow.CoveragePublicationInput{Observation: obs, Recipe: recipe,
			Events: workflow.CoverageEvents{StaleMarkerPresent: recipeKind == "incomplete"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func rgaS5CLIRun(args ...string) (string, string, error) {
	cmd := buildRootCmd()
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), stderr.String(), err
}

func rgaS5CLIState(t *testing.T, root string) string {
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
		if entry.IsDir() {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
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
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
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

func TestRGAS5CLISevenExecuteCasesAndAutoRefusal(t *testing.T) {
	for _, tc := range []struct {
		name, recipe      string
		legacy, malformed bool
		code              int
		warning           bool
	}{
		{"coverage-invalid", "complete", false, true, 2, false},
		{"missing", "missing", false, false, 2, false},
		{"undecodable", "bad", false, false, 2, false},
		{"incomplete-executes", "incomplete", false, false, 0, true},
		{"legacy-missing", "missing", true, false, 1, false},
		{"legacy-executes", "complete", true, false, 0, false},
		{"complete-executes", "complete", false, false, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := rgaS5CLISetup(t, tc.recipe, tc.legacy)
			if tc.malformed {
				if err := s.WriteArtifact("s5", "recipe-coverage.json", "{}"); err != nil {
					t.Fatal(err)
				}
			}
			before := rgaS5CLIState(t, s.Root)
			_, warning, err := rgaS5CLIRun("apply", "s5", "--path", s.Root, "--mode", "execute")
			code := 0
			if err != nil {
				code = 1
				var exit *ExitCodeError
				if errors.As(err, &exit) {
					code = exit.Code
				}
			}
			if code != tc.code || strings.Contains(warning, "not replay authority") != tc.warning {
				t.Fatalf("exit=%d warning=%q error=%v", code, warning, err)
			}
			if code != 0 && rgaS5CLIState(t, s.Root) != before {
				t.Fatal("refused execute mutated files")
			}
			if tc.code == 2 {
				_, _, err := rgaS5CLIRun("apply", "s5", "--path", s.Root, "--mode", "auto")
				var exit *ExitCodeError
				if !errors.As(err, &exit) || exit.Code != 2 {
					t.Fatalf("auto did not preflight before prepare: %v", err)
				}
				if rgaS5CLIState(t, s.Root) != before {
					t.Fatal("auto refusal wrote prepare/progress files")
				}
			}
		})
	}
}

func TestRGAS5CLIUsesCapturedRecipeAndBypassesCanonicalReapply(t *testing.T) {
	s := rgaS5CLISetup(t, "complete", false)
	reader := workflow.ReadRecipeArtifact
	t.Cleanup(func() { workflow.ReadRecipeArtifact = reader })
	reads := 0
	workflow.ReadRecipeArtifact = func(path string) workflow.RecipeArtifactRead {
		if filepath.Base(path) == "apply-recipe.json" {
			reads++
			if reads > 1 {
				return workflow.RecipeArtifactRead{Exists: true, Bytes: []byte("{swapped")}
			}
		}
		return reader(path)
	}
	if _, _, err := rgaS5CLIRun("apply", "s5", "--path", s.Root, "--mode", "execute"); err != nil {
		t.Fatal(err)
	}
	if reads != 1 {
		t.Fatalf("validated recipe read %d times", reads)
	}
	workflow.ReadRecipeArtifact = reader
	if err := s.MarkFeatureState("s5", store.StateUnapplied, "feature unapply", ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Root, "a.txt"), []byte("old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteArtifact("s5", "recipe-coverage.json", "{}"); err != nil {
		t.Fatal(err)
	}
	workflow.ReadRecipeArtifact = func(path string) workflow.RecipeArtifactRead {
		if filepath.Base(path) == "recipe-coverage.json" {
			t.Fatal("canonical reapply reached recipe-only classifier")
		}
		return reader(path)
	}
	if _, _, err := rgaS5CLIRun("apply", "s5", "--path", s.Root, "--mode", "execute"); err != nil {
		t.Fatal(err)
	}
}

func TestRGAS5RecordPublicationPathsRefuseBeforeBoundWrites(t *testing.T) {
	for _, broken := range []string{"patches-symlink", "patches-file", "coverage-directory", "coverage-directory-with-recipe-error"} {
		t.Run(broken, func(t *testing.T) {
			s := rgaS5CLISetup(t, "missing", true)
			featureDir := filepath.Join(s.Root, ".tpatch", "features", "s5")
			if err := os.Remove(filepath.Join(featureDir, "artifacts", "post-apply.patch")); err != nil {
				t.Fatal(err)
			}
			patches := filepath.Join(featureDir, "patches")
			switch broken {
			case "patches-symlink", "patches-file":
				if err := os.Remove(patches); err != nil && !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
				if broken == "patches-symlink" {
					destination := filepath.Join(s.Root, ".tpatch", "audit-target")
					if err := os.MkdirAll(destination, 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(destination, "sentinel"), []byte("untouched"), 0644); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(destination, patches); err != nil {
						t.Fatal(err)
					}
				} else if err := os.WriteFile(patches, []byte("not a directory"), 0644); err != nil {
					t.Fatal(err)
				}
			default:
				if err := os.Mkdir(filepath.Join(featureDir, "artifacts", "recipe-coverage.json"), 0755); err != nil {
					t.Fatal(err)
				}
				if broken == "coverage-directory-with-recipe-error" {
					if err := os.Mkdir(filepath.Join(featureDir, "artifacts", "apply-recipe.json"), 0755); err != nil {
						t.Fatal(err)
					}
				}
			}
			before := rgaS5CLIState(t, filepath.Join(s.Root, ".tpatch"))
			source, err := os.ReadFile(filepath.Join(s.Root, "a.txt"))
			if err != nil {
				t.Fatal(err)
			}
			for _, overrides := range [][]string{nil, {"--force-amend", "--lenient", "--allow-collision", "intentional duplicate"}} {
				args := append([]string{"record", "s5", "--path", s.Root, "--regenerate-recipe"}, overrides...)
				_, _, err := rgaS5CLIRun(args...)
				if err == nil || !strings.Contains(err.Error(), "publication") {
					t.Fatalf("known publication failure did not gate actual producer: %v", err)
				}
				if after := rgaS5CLIState(t, filepath.Join(s.Root, ".tpatch")); after != before {
					t.Fatal("refused producer changed bound artifacts, audit files, or feature state")
				}
				after, err := os.ReadFile(filepath.Join(s.Root, "a.txt"))
				if err != nil || !bytes.Equal(after, source) {
					t.Fatal("refused producer changed source bytes")
				}
			}
		})
	}
}

func TestRGAS5CLIActualRecordPublishesDryRecommendedPair(t *testing.T) {
	s := rgaS5CLISetup(t, "missing", true)
	for _, scope := range [][]string{nil, {"--files", "a.txt"}} {
		args := append([]string{"record", "s5", "--path", s.Root, "--regenerate-recipe"}, scope...)
		if _, _, err := rgaS5CLIRun(args...); err != nil {
			t.Fatal(err)
		}
		snap := workflow.SnapshotRecipeCoverage(s.Root, "s5")
		if result := workflow.AssessRecipeCoverage(s.Root, "s5", snap, nil); result.Rung != 6 {
			t.Fatalf("real default/scoped producer did not publish complete independent evidence: %s", result.Diagnostic())
		}
	}
}
