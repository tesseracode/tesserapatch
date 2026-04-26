package workflow

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// createdByTestEnv builds a planTestEnv-style store, registers the parent and
// child features with the given dependency edge, and returns the store
// plus the slugs. Tests that exercise the child's recipe pass
// recipe.Feature = childSlug.
func createdByTestEnv(t *testing.T, dagEnabled bool, parentSlug, childSlug, kind string) *store.Store {
	t.Helper()
	s := planTestEnv(t, dagEnabled)
	addPlanFeature(t, s, parentSlug, nil)
	addPlanFeature(t, s, childSlug, []store.Dependency{
		{Slug: parentSlug, Kind: kind},
	})
	return s
}

func writeWorkingFile(t *testing.T, s *store.Store, rel, content string) {
	t.Helper()
	full := filepath.Join(s.Root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// captureWarn redirects WarnWriter for the duration of the test and
// returns a function to read what was written.
func captureWarn(t *testing.T) (read func() string, restore func()) {
	t.Helper()
	prev := WarnWriter
	buf := &bytes.Buffer{}
	WarnWriter = buf
	return buf.String, func() { WarnWriter = prev }
}

// TestCreatedByGate_FlagOff_NoOp — when features_dependencies is false,
// CreatedBy must remain inert. A missing target with CreatedBy set
// surfaces the bare "file not found" error byte-identical to v0.5.3.
func TestCreatedByGate_FlagOff_NoOp(t *testing.T) {
	s := createdByTestEnv(t, false, "parent", "child", store.DependencyKindHard)

	recipe := ApplyRecipe{
		Feature: "child",
		Operations: []RecipeOperation{
			{Type: "replace-in-file", Path: "src/missing.go", Search: "x", Replace: "y", CreatedBy: "parent"},
		},
	}
	res := DryRunRecipe(s, recipe)
	if res.Success {
		t.Fatalf("expected failure (file missing), got success")
	}
	if len(res.Errors) != 1 {
		t.Fatalf("want 1 error, got %d: %v", len(res.Errors), res.Errors)
	}
	if !strings.Contains(res.Errors[0], "file not found") {
		t.Fatalf("expected bare not-found error (flag off), got %q", res.Errors[0])
	}
	if strings.Contains(res.Errors[0], "will be created by parent feature") {
		t.Fatalf("flag off must not surface created_by guidance; got %q", res.Errors[0])
	}
}

// TestCreatedByGate_HardParent_TargetMissing_ErrPathCreatedByParent — the
// canonical hard-parent case. Recipe op for a file that doesn't exist yet,
// declared as created_by a hard parent, surfaces the actionable sentinel
// error naming the parent slug.
func TestCreatedByGate_HardParent_TargetMissing_ErrPathCreatedByParent(t *testing.T) {
	s := createdByTestEnv(t, true, "parent", "child", store.DependencyKindHard)

	op := RecipeOperation{Type: "replace-in-file", Path: "src/auth.ts", Search: "x", Replace: "y", CreatedBy: "parent"}
	gateErr := checkCreatedByGate(s, "child", op, false)
	if gateErr == nil {
		t.Fatalf("expected ErrPathCreatedByParent, got nil")
	}
	if !errors.Is(gateErr, ErrPathCreatedByParent) {
		t.Fatalf("expected wraps ErrPathCreatedByParent, got %v", gateErr)
	}
	msg := gateErr.Error()
	if !strings.Contains(msg, "src/auth.ts") {
		t.Errorf("error message must include the target path; got %q", msg)
	}
	if !strings.Contains(msg, "parent") {
		t.Errorf("error message must include the parent slug; got %q", msg)
	}
	if !strings.Contains(msg, "apply parent first") {
		t.Errorf("error message must direct operator to apply parent; got %q", msg)
	}

	// Also exercise via DryRunRecipe end-to-end.
	recipe := ApplyRecipe{Feature: "child", Operations: []RecipeOperation{op}}
	res := DryRunRecipe(s, recipe)
	if res.Success {
		t.Fatalf("recipe must fail when hard parent owns missing target")
	}
	if len(res.Errors) != 1 || !strings.Contains(res.Errors[0], "will be created by parent feature parent") {
		t.Fatalf("wrong recipe error: %v", res.Errors)
	}
}

// TestCreatedByGate_HardParent_TargetExists_NoError — when the target is
// already present (e.g. parent has been applied), the gate must yield to
// the normal op flow.
func TestCreatedByGate_HardParent_TargetExists_NoError(t *testing.T) {
	s := createdByTestEnv(t, true, "parent", "child", store.DependencyKindHard)
	writeWorkingFile(t, s, "src/auth.ts", "OLD\n")

	op := RecipeOperation{Type: "replace-in-file", Path: "src/auth.ts", Search: "OLD", Replace: "NEW", CreatedBy: "parent"}
	if err := checkCreatedByGate(s, "child", op, true); err != nil {
		t.Fatalf("gate must pass when target exists; got %v", err)
	}

	recipe := ApplyRecipe{Feature: "child", Operations: []RecipeOperation{op}}
	res := ExecuteRecipe(s, recipe)
	if !res.Success {
		t.Fatalf("execute should succeed; got errors %v", res.Errors)
	}
	got, _ := os.ReadFile(filepath.Join(s.Root, "src/auth.ts"))
	if string(got) != "NEW\n" {
		t.Fatalf("expected replacement applied; got %q", got)
	}
}

// TestCreatedByGate_SoftParent_TargetMissing_FallsThroughWithWarning —
// soft deps are ordering hints; gate emits a warning and surfaces the
// existing not-found error rather than ErrPathCreatedByParent.
func TestCreatedByGate_SoftParent_TargetMissing_FallsThroughWithWarning(t *testing.T) {
	s := createdByTestEnv(t, true, "soft-parent", "child", store.DependencyKindSoft)

	read, restore := captureWarn(t)
	defer restore()

	recipe := ApplyRecipe{
		Feature: "child",
		Operations: []RecipeOperation{
			{Type: "append-file", Path: "src/missing.go", Content: "x", CreatedBy: "soft-parent"},
		},
	}
	res := DryRunRecipe(s, recipe)
	if res.Success {
		t.Fatalf("expected failure (file missing), got success")
	}
	if len(res.Errors) != 1 {
		t.Fatalf("want 1 error, got %d: %v", len(res.Errors), res.Errors)
	}
	if !strings.Contains(res.Errors[0], "file not found") {
		t.Fatalf("soft-dep gate must fall through to bare not-found; got %q", res.Errors[0])
	}
	if errors.Is(errors.New(res.Errors[0]), ErrPathCreatedByParent) {
		t.Errorf("soft-dep gate must not return ErrPathCreatedByParent")
	}
	warn := read()
	if !strings.Contains(warn, "soft-parent") || !strings.Contains(warn, "soft deps do not gate apply") {
		t.Fatalf("expected soft-dep warning emitted; got %q", warn)
	}
}

// TestCreatedByGate_ParentNotInDependsOn_RecipeRejected — created_by must
// reference a declared dependency. If it names a feature outside the
// child's depends_on, the gate fails at recipe-load time as a
// recipe-shape validation error.
func TestCreatedByGate_ParentNotInDependsOn_RecipeRejected(t *testing.T) {
	s := planTestEnv(t, true)
	addPlanFeature(t, s, "child", nil) // no deps declared
	addPlanFeature(t, s, "stranger", nil)
	writeWorkingFile(t, s, "src/x.go", "OLD\n")

	op := RecipeOperation{Type: "replace-in-file", Path: "src/x.go", Search: "OLD", Replace: "NEW", CreatedBy: "stranger"}
	gateErr := checkCreatedByGate(s, "child", op, true)
	if gateErr == nil {
		t.Fatalf("expected validation error, got nil")
	}
	if !strings.Contains(gateErr.Error(), "is not in depends_on") {
		t.Fatalf("expected depends_on validation error; got %v", gateErr)
	}
	if errors.Is(gateErr, ErrPathCreatedByParent) {
		t.Errorf("validation error must NOT wrap ErrPathCreatedByParent")
	}

	recipe := ApplyRecipe{Feature: "child", Operations: []RecipeOperation{op}}
	res := DryRunRecipe(s, recipe)
	if res.Success {
		t.Fatalf("recipe with dangling created_by must be rejected")
	}
	if len(res.Errors) != 1 || !strings.Contains(res.Errors[0], "is not in depends_on") {
		t.Fatalf("wrong recipe error: %v", res.Errors)
	}
}

// TestCreatedByGate_ParentUpstreamMerged_TargetExists_NoError — once the
// hard parent has been adopted upstream (state=upstream_merged) and the
// target file is present, the gate must yield (ADR-011 D5).
func TestCreatedByGate_ParentUpstreamMerged_TargetExists_NoError(t *testing.T) {
	s := createdByTestEnv(t, true, "parent", "child", store.DependencyKindHard)
	setParentState(t, s, "parent", store.StateUpstreamMerged, store.ReconcileUpstreamed, "")
	writeWorkingFile(t, s, "src/auth.ts", "OLD\n")

	op := RecipeOperation{Type: "replace-in-file", Path: "src/auth.ts", Search: "OLD", Replace: "NEW", CreatedBy: "parent"}
	if err := checkCreatedByGate(s, "child", op, true); err != nil {
		t.Fatalf("upstream_merged + target present must pass gate; got %v", err)
	}
	recipe := ApplyRecipe{Feature: "child", Operations: []RecipeOperation{op}}
	res := ExecuteRecipe(s, recipe)
	if !res.Success {
		t.Fatalf("execute should succeed; got errors %v", res.Errors)
	}
}

// TestCreatedByGate_AppliesToReplaceAndAppend — gate fires for
// replace-in-file and append-file (target-must-exist preconditions) but
// NOT for write-file or ensure-directory (which create their target).
// This pins ADR-011 D4's "only ops with hard target preconditions are
// gated" rule against accidental scope creep.
func TestCreatedByGate_AppliesToReplaceAndAppend(t *testing.T) {
	s := createdByTestEnv(t, true, "parent", "child", store.DependencyKindHard)

	gated := []RecipeOperation{
		{Type: "replace-in-file", Path: "src/missing-r.go", Search: "x", Replace: "y", CreatedBy: "parent"},
		{Type: "append-file", Path: "src/missing-a.go", Content: "x", CreatedBy: "parent"},
	}
	for _, op := range gated {
		recipe := ApplyRecipe{Feature: "child", Operations: []RecipeOperation{op}}
		res := DryRunRecipe(s, recipe)
		if res.Success {
			t.Fatalf("[%s] expected gate to fire, got success", op.Type)
		}
		if len(res.Errors) != 1 || !strings.Contains(res.Errors[0], "will be created by parent feature parent") {
			t.Fatalf("[%s] expected ErrPathCreatedByParent error; got %v", op.Type, res.Errors)
		}
	}

	// write-file and ensure-directory must NOT trigger the gate even with
	// CreatedBy set against a hard parent and a missing target — those
	// ops create their target, so the precondition does not apply.
	// (We seed the parent directory so write-file's own parent-dir
	// precondition doesn't mask the test of the gate.)
	if err := os.MkdirAll(filepath.Join(s.Root, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	notGated := []RecipeOperation{
		{Type: "write-file", Path: "src/created-by-child.go", Content: "package x\n", CreatedBy: "parent"},
		{Type: "ensure-directory", Path: "src/newdir", CreatedBy: "parent"},
	}
	for _, op := range notGated {
		recipe := ApplyRecipe{Feature: "child", Operations: []RecipeOperation{op}}
		res := DryRunRecipe(s, recipe)
		if !res.Success {
			t.Fatalf("[%s] must not be gated by created_by; got errors %v", op.Type, res.Errors)
		}
	}
}
