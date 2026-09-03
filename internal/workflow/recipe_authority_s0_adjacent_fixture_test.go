package workflow

// GH #15 / ADR-036 slice S0 — frozen evidence, promoted empirical fixture.
//
// PRD-recipe-generation-authority §8 "S0 - Frozen evidence" requires:
// "Promote the adjacent-conflict scripts and downstream V10 case into
// fixtures."
//
// The shell scripts under
// `docs/state-of-the-art/case-studies/adjacent-cli-args-conflict-2026-08/`
// are accepted empirical research (2026-08-15, "no implementation
// authorized"). They are shell-only, build the CLI, and use `mktemp`, so
// nothing in the Go suite holds them still. This file promotes their
// inputs into stable `testdata/` bytes and pins the CURRENT behaviour the
// case study measured — the whole-file autogeneration shape, its
// cross-base hazard, and the phase-2 inspection gap — WITHOUT implementing
// any part of the fix GH #15 owns.
//
// Everything here is git-free and OS-independent: the fixtures are plain
// bytes, and the assertions run against `RecipeFromPatch`,
// `evaluateRecipeOperations` and `ExecuteRecipe` directly.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

const rgaS0AdjacentSlug = "add-feature-args"

// rgaS0AdjacentDir is the promoted copy of the accepted case study's
// inputs. See its README.md for the provenance mapping.
func rgaS0AdjacentDir() string {
	return filepath.Join("testdata", "rga-s0", "adjacent-cli-args-conflict-2026-08")
}

func rgaS0Fixture(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(rgaS0AdjacentDir(), name))
	if err != nil {
		t.Fatalf("read promoted fixture %s: %v", name, err)
	}
	if name == "feature.post-apply.patch" {
		return strings.ReplaceAll(string(body), "{{CONTEXT}}", " ")
	}
	return string(body)
}

// rgaS0Store builds an initialised store whose worktree holds `command.go`
// with the given body.
func rgaS0Store(t *testing.T, commandBody string) *store.Store {
	t.Helper()
	tmp := t.TempDir()
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "command.go"), []byte(commandBody), 0o644); err != nil {
		t.Fatalf("write command.go: %v", err)
	}
	return s
}

// rgaS0StructuralOperation is the hand-authored, intent-preserving
// operation the case study §3.2 authored by hand. Its anchor is the slice
// closing brace plus the following `return run(args)`.
func rgaS0StructuralOperation() RecipeOperation {
	return RecipeOperation{
		Type:    "replace-in-file",
		Path:    "command.go",
		Search:  "\t}\n\treturn run(args)",
		Replace: "\t\t\"--feature-x\",\n\t\t\"--feature-y\",\n\t}\n\treturn run(args)",
	}
}

// TestRGAS0AdjacentFixtureMatchesCaseStudyShape guards the promoted bytes
// themselves. If the fixture drifts away from the accepted case study the
// behavioural rows below would freeze the wrong thing silently.
func TestRGAS0AdjacentFixtureMatchesCaseStudyShape(t *testing.T) {
	base := rgaS0Fixture(t, "base.command.go.txt")
	feature := rgaS0Fixture(t, "feature.command.go.txt")
	upstream := rgaS0Fixture(t, "upstream.command.go.txt")
	resolved := rgaS0Fixture(t, "resolved.command.go.txt")
	patch := rgaS0Fixture(t, "feature.post-apply.patch")

	for _, row := range []struct {
		name string
		body string
		want []string
		deny []string
	}{
		{"base", base, []string{`"--old-a"`, `"--old-b"`}, []string{"--feature-x", "--feature-y"}},
		{"feature", feature, []string{`"--old-a"`, `"--old-b"`, `"--feature-x"`, `"--feature-y"`}, nil},
		{"upstream", upstream, nil, []string{"--old-a", "--old-b", "--feature-x"}},
		{"resolved", resolved, []string{`"--feature-x"`, `"--feature-y"`}, []string{"--old-a", "--old-b"}},
	} {
		for _, want := range row.want {
			if !strings.Contains(row.body, want) {
				t.Errorf("%s fixture must contain %s", row.name, want)
			}
		}
		for _, deny := range row.deny {
			if strings.Contains(row.body, deny) {
				t.Errorf("%s fixture must not contain %s", row.name, deny)
			}
		}
	}

	// The feature tree adds its two arguments AFTER the upstream ones —
	// the `after` / `delete-all` variant of `reproduce.sh`.
	if strings.Index(feature, `"--old-b"`) > strings.Index(feature, `"--feature-x"`) {
		t.Error("feature fixture must add its arguments after the upstream ones")
	}

	// The patch is exactly one single-file `diff --git` record over
	// `command.go`, so every parser row below reads one effect.
	if got := strings.Count(patch, "diff --git "); got != 1 {
		t.Fatalf("promoted patch must carry exactly one diff --git record, got %d", got)
	}
	if !strings.Contains(patch, "diff --git a/command.go b/command.go") {
		t.Fatalf("promoted patch header changed:\n%s", patch)
	}
	// It ADDS only the two feature arguments; it never mentions the
	// upstream ones as added lines. That asymmetry is what makes the
	// whole-file row below meaningful.
	if !strings.Contains(patch, "+\t\t\"--feature-x\",") || !strings.Contains(patch, "+\t\t\"--feature-y\",") {
		t.Errorf("promoted patch must add both feature arguments:\n%s", patch)
	}
	if strings.Contains(patch, "+\t\t\"--old-a\",") {
		t.Errorf("promoted patch must not ADD the upstream arguments:\n%s", patch)
	}
}

// TestRGAS0AdjacentAutogenEmitsWholeFileWriteWithoutPreimage freezes
// case study §3.1 + §3.3: `tpatch record`'s autogeneration emits ONE
// whole-file `write-file` operation carrying the entire postimage and NO
// `preimage_hash`.
func TestRGAS0AdjacentAutogenEmitsWholeFileWriteWithoutPreimage(t *testing.T) {
	feature := rgaS0Fixture(t, "feature.command.go.txt")
	patch := rgaS0Fixture(t, "feature.post-apply.patch")
	s := rgaS0Store(t, feature)

	recipe, skipped, err := RecipeFromPatch(s.Root, rgaS0AdjacentSlug, patch)
	if err != nil {
		t.Fatalf("RecipeFromPatch: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("no path should be skipped for this fixture, got %v", skipped)
	}
	if len(recipe.Operations) != 1 {
		t.Fatalf("want exactly 1 derived operation, got %d: %+v", len(recipe.Operations), recipe.Operations)
	}
	op := recipe.Operations[0]
	if op.Type != "write-file" {
		t.Errorf("derived op type = %q, want write-file (whole-file generation)", op.Type)
	}
	if op.Path != "command.go" {
		t.Errorf("derived op path = %q, want command.go", op.Path)
	}
	if op.PreimageHash != nil {
		t.Errorf("autogenerated op must carry NO preimage_hash today, got %q", *op.PreimageHash)
	}
	// Whole-file, not hunk-scoped: the operation content is the entire
	// postimage, including the upstream arguments the patch never added.
	if op.Content != feature {
		t.Errorf("derived content is not the whole postimage:\n got %q\nwant %q", op.Content, feature)
	}
	if !strings.Contains(op.Content, `"--old-a"`) {
		t.Error("whole-file evidence lost: derived content should carry the upstream arguments verbatim")
	}
	// Sensitivity: the derivation reads the LIVE worktree, not the patch.
	// Changing the worktree changes the operation.
	if err := os.WriteFile(filepath.Join(s.Root, "command.go"), []byte("package command\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mutated, _, err := RecipeFromPatch(s.Root, rgaS0AdjacentSlug, patch)
	if err != nil {
		t.Fatalf("RecipeFromPatch after worktree mutation: %v", err)
	}
	if len(mutated.Operations) != 1 || mutated.Operations[0].Content != "package command\n" {
		t.Fatalf("derivation must follow the live worktree; got %+v", mutated.Operations)
	}
}

// TestRGAS0AdjacentWholeFileWriteRestoresDeletedUpstreamArguments freezes
// PRD §2.6 / case study §3.3: replayed against the upstream tree, the
// generated whole-file write is accepted and RESTORES the arguments
// upstream intentionally deleted. Current behaviour reports success; no
// cross-base guard exists.
func TestRGAS0AdjacentWholeFileWriteRestoresDeletedUpstreamArguments(t *testing.T) {
	feature := rgaS0Fixture(t, "feature.command.go.txt")
	upstream := rgaS0Fixture(t, "upstream.command.go.txt")
	resolved := rgaS0Fixture(t, "resolved.command.go.txt")
	patch := rgaS0Fixture(t, "feature.post-apply.patch")

	// Derive the recipe against the feature tree, exactly as `record` does.
	derivation := rgaS0Store(t, feature)
	recipe, _, err := RecipeFromPatch(derivation.Root, rgaS0AdjacentSlug, patch)
	if err != nil {
		t.Fatalf("RecipeFromPatch: %v", err)
	}
	recipe.Feature = rgaS0AdjacentSlug

	// Replay it against the UPSTREAM tree.
	target := rgaS0Store(t, upstream)
	result := ExecuteRecipe(target, recipe)
	if !result.Success {
		t.Fatalf("current behaviour executes without refusal; got errors %v", result.Errors)
	}
	if result.Applied != 1 || result.Operations != 1 {
		t.Errorf("applied/operations = %d/%d, want 1/1", result.Applied, result.Operations)
	}
	got, err := os.ReadFile(filepath.Join(target.Root, "command.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"--old-a"`) || !strings.Contains(string(got), `"--old-b"`) {
		t.Fatalf("expected the CURRENT (unsafe) outcome: deleted upstream arguments restored; got:\n%s", got)
	}
	if string(got) == resolved {
		t.Fatal("whole-file replay must NOT already produce the semantically correct tree; that is the GH #15 gap")
	}
}

// TestRGAS0AdjacentPhase2InspectionCounts freezes case study §3.2: on the
// upstream tree the generated whole-file operation is CONFLICTING while
// the hand-authored structural operation is APPLICABLE — and neither
// shape satisfies `allPresent`, which is the only early-return phase 2
// recognises today.
func TestRGAS0AdjacentPhase2InspectionCounts(t *testing.T) {
	feature := rgaS0Fixture(t, "feature.command.go.txt")
	upstream := rgaS0Fixture(t, "upstream.command.go.txt")
	patch := rgaS0Fixture(t, "feature.post-apply.patch")

	derivation := rgaS0Store(t, feature)
	generated, _, err := RecipeFromPatch(derivation.Root, rgaS0AdjacentSlug, patch)
	if err != nil {
		t.Fatalf("RecipeFromPatch: %v", err)
	}

	target := rgaS0Store(t, upstream)

	t.Run("generated-whole-file-conflicts", func(t *testing.T) {
		got := evaluateRecipeOperations(target.Root, generated.Operations)
		if got.allPresent {
			t.Error("allPresent must be false for the generated whole-file operation")
		}
		if !got.hasConflicts || got.conflictCount != 1 {
			t.Errorf("want exactly one conflicting operation, got %+v", got)
		}
		if got.applicableCount != 0 || got.presentCount != 0 {
			t.Errorf("want 0 applicable / 0 present, got %+v", got)
		}
	})

	t.Run("structural-operation-is-applicable-only", func(t *testing.T) {
		got := evaluateRecipeOperations(target.Root, []RecipeOperation{rgaS0StructuralOperation()})
		if got.allPresent {
			t.Error("allPresent must be false: nothing is present yet")
		}
		if got.hasConflicts || got.conflictCount != 0 {
			t.Errorf("an applicable structural operation must not be a conflict, got %+v", got)
		}
		if got.applicableCount != 1 {
			t.Errorf("want exactly one applicable operation, got %+v", got)
		}
		// This is the gap: `allPresent == false` AND `hasConflicts ==
		// false` is the one combination phase 2 neither terminates on
		// nor annotates. The source guard in
		// recipe_authority_s0_source_guards_test.go pins that branch
		// structure; this row pins the counts that reach it.
	})

	t.Run("already-present-operation-is-present-not-applicable", func(t *testing.T) {
		// Sensitivity twin: the SAME evaluator does report `allPresent`
		// once the effect is materialised, so the row above is measuring
		// the applicable-only gap and not a dead evaluator.
		resolvedStore := rgaS0Store(t, rgaS0Fixture(t, "resolved.command.go.txt"))
		got := evaluateRecipeOperations(resolvedStore.Root, []RecipeOperation{rgaS0StructuralOperation()})
		if !got.allPresent || got.presentCount != 1 {
			t.Errorf("want allPresent with one present operation, got %+v", got)
		}
	})
}

// TestRGAS0AdjacentStructuralOperationHazards freezes the three safety
// limits the case study §3.3 reproduced or source-verified. They are
// CURRENT behaviour, deliberately not fixed here.
func TestRGAS0AdjacentStructuralOperationHazards(t *testing.T) {
	upstream := rgaS0Fixture(t, "upstream.command.go.txt")
	resolved := rgaS0Fixture(t, "resolved.command.go.txt")

	t.Run("replace-in-file-is-not-idempotent", func(t *testing.T) {
		s := rgaS0Store(t, upstream)
		recipe := ApplyRecipe{Feature: rgaS0AdjacentSlug, Operations: []RecipeOperation{rgaS0StructuralOperation()}}

		if r := ExecuteRecipe(s, recipe); !r.Success {
			t.Fatalf("first apply failed: %v", r.Errors)
		}
		first, err := os.ReadFile(filepath.Join(s.Root, "command.go"))
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != resolved {
			t.Fatalf("first apply must produce the semantically correct tree:\n got %q\nwant %q", first, resolved)
		}

		if r := ExecuteRecipe(s, recipe); !r.Success {
			t.Fatalf("second apply failed: %v", r.Errors)
		}
		second, err := os.ReadFile(filepath.Join(s.Root, "command.go"))
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(string(second), `"--feature-x"`); got != 2 {
			t.Fatalf("current replay is NOT idempotent: want 2 occurrences after a second apply, got %d:\n%s", got, second)
		}
	})

	t.Run("duplicate-anchor-silently-selects-the-first-match", func(t *testing.T) {
		body := "package command\n\n" +
			"func first() []string {\n\targs := []string{\n\t}\n\treturn run(args)\n}\n\n" +
			"func second() []string {\n\targs := []string{\n\t}\n\treturn run(args)\n}\n\n" +
			"func run(args []string) []string { return args }\n"
		s := rgaS0Store(t, body)
		op := rgaS0StructuralOperation()
		op.Replace = "\t\t\"--feature-x\",\n\t}\n\treturn run(args)"

		if r := ExecuteRecipe(s, ApplyRecipe{Feature: rgaS0AdjacentSlug, Operations: []RecipeOperation{op}}); !r.Success {
			t.Fatalf("apply failed: %v", r.Errors)
		}
		got, err := os.ReadFile(filepath.Join(s.Root, "command.go"))
		if err != nil {
			t.Fatal(err)
		}
		text := string(got)
		if strings.Count(text, `"--feature-x"`) != 1 {
			t.Fatalf("expected exactly one substitution, got:\n%s", text)
		}
		if strings.Index(text, `"--feature-x"`) > strings.Index(text, "func second()") {
			t.Fatalf("current behaviour selects the FIRST substring match with no anchor-uniqueness check; got:\n%s", text)
		}
	})

	t.Run("whole-file-write-resurrects-a-missing-target", func(t *testing.T) {
		s := rgaS0Store(t, upstream)
		op := RecipeOperation{Type: "write-file", Path: "removed.go", Content: "package command\n"}
		target := filepath.Join(s.Root, "removed.go")
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Fatalf("fixture precondition: removed.go must not exist, stat err = %v", err)
		}
		// The evaluator calls a missing target "applicable, can be created".
		if got := evaluateRecipeOperations(s.Root, []RecipeOperation{op}); got.applicableCount != 1 || got.hasConflicts {
			t.Errorf("missing target must currently read as applicable, got %+v", got)
		}
		if r := ExecuteRecipe(s, ApplyRecipe{Feature: rgaS0AdjacentSlug, Operations: []RecipeOperation{op}}); !r.Success {
			t.Fatalf("apply failed: %v", r.Errors)
		}
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("current behaviour recreates an absent target; stat err = %v", err)
		}
	})
}
