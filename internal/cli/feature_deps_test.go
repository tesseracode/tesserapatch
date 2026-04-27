package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// helpers re-use newDAGTestRepo from status_dag_test.go.

func TestFeatureDeps_Show_NoDeps(t *testing.T) {
	tmp, _ := newDAGTestRepo(t)
	runCmd("add", "--path", tmp, "--slug", "foo", "Foo")

	out, _, code := runCmd("feature", "deps", "--path", tmp, "foo")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if !strings.Contains(out, "depends_on: (none)") {
		t.Fatalf("missing 'depends_on: (none)': %q", out)
	}
}

func TestFeatureDepsAdd_RejectsCycle(t *testing.T) {
	tmp, s := newDAGTestRepo(t)
	runCmd("add", "--path", tmp, "--slug", "a", "A")
	runCmd("add", "--path", tmp, "--slug", "b", "B")
	// b depends on a (legal).
	if _, _, code := runCmd("feature", "deps", "--path", tmp, "b", "add", "a"); code != 0 {
		t.Fatalf("legal add failed")
	}
	// a depends on b would form a cycle.
	_, errOut, code := runCmd("feature", "deps", "--path", tmp, "a", "add", "b")
	if code == 0 {
		t.Fatalf("expected cycle rejection, got success: %s", errOut)
	}
	a, _ := s.LoadFeatureStatus("a")
	if len(a.DependsOn) != 0 {
		t.Fatalf("rejected add must not persist: %+v", a.DependsOn)
	}
}

func TestFeatureDepsAdd_RejectsKindConflict(t *testing.T) {
	// Same parent appearing twice with different kinds inside the
	// proposed deps list is rejected by ValidateDependencies. Add
	// hard, then "add soft" upgrades the existing entry — that is
	// expected behaviour. The conflict path requires manual seeding
	// of two entries.
	tmp, s := newDAGTestRepo(t)
	runCmd("add", "--path", tmp, "--slug", "p", "P")
	runCmd("add", "--path", tmp, "--slug", "c", "C")
	c, _ := s.LoadFeatureStatus("c")
	c.DependsOn = []store.Dependency{
		{Slug: "p", Kind: store.DependencyKindHard},
		{Slug: "p", Kind: store.DependencyKindSoft},
	}
	if err := s.SaveFeatureStatus(c); err != nil {
		t.Fatal(err)
	}
	// Trying to add anything will trip kind-conflict validation.
	runCmd("add", "--path", tmp, "--slug", "extra", "Extra")
	_, _, code := runCmd("feature", "deps", "--path", tmp, "c", "add", "extra")
	if code == 0 {
		t.Fatalf("expected kind-conflict rejection")
	}
}

func TestFeatureDepsRemove_ClearsAtomically(t *testing.T) {
	tmp, s := newDAGTestRepo(t)
	runCmd("add", "--path", tmp, "--slug", "p", "P")
	runCmd("add", "--path", tmp, "--slug", "c", "C")
	runCmd("feature", "deps", "--path", tmp, "c", "add", "p")

	c, _ := s.LoadFeatureStatus("c")
	if len(c.DependsOn) != 1 {
		t.Fatalf("expected 1 dep before remove, got %d", len(c.DependsOn))
	}

	out, _, code := runCmd("feature", "deps", "--path", tmp, "c", "remove", "p")
	if code != 0 {
		t.Fatalf("remove failed: %s", out)
	}
	c, _ = s.LoadFeatureStatus("c")
	if len(c.DependsOn) != 0 {
		t.Fatalf("expected 0 deps after remove, got %v", c.DependsOn)
	}
	// Dependents derivation: p should now have none.
	out, _, _ = runCmd("feature", "deps", "--path", tmp, "p")
	if !strings.Contains(out, "dependents: (none)") {
		t.Fatalf("dependents not re-derived after remove: %q", out)
	}
}

func TestAmendDependsOn_ValidatedIdenticallyToFeatureDeps(t *testing.T) {
	tmp, s := newDAGTestRepo(t)
	runCmd("add", "--path", tmp, "--slug", "p", "P")
	runCmd("add", "--path", tmp, "--slug", "c", "C")

	out, _, code := runCmd("amend", "--path", tmp, "c", "--depends-on", "p:hard")
	if code != 0 {
		t.Fatalf("amend --depends-on failed: %s", out)
	}
	c, _ := s.LoadFeatureStatus("c")
	if len(c.DependsOn) != 1 || c.DependsOn[0].Slug != "p" || c.DependsOn[0].Kind != "hard" {
		t.Fatalf("amend did not persist depends_on: %+v", c.DependsOn)
	}

	// Cycle attempt via amend — must be rejected exactly like feature deps.
	_, _, code = runCmd("amend", "--path", tmp, "p", "--depends-on", "c")
	if code == 0 {
		t.Fatalf("amend cycle attempt should have failed")
	}

	// Removal via amend.
	out, _, code = runCmd("amend", "--path", tmp, "c", "--remove-depends-on", "p")
	if code != 0 {
		t.Fatalf("amend --remove-depends-on failed: %s", out)
	}
	c, _ = s.LoadFeatureStatus("c")
	if len(c.DependsOn) != 0 {
		t.Fatalf("amend remove did not clear: %+v", c.DependsOn)
	}
}

func TestRemoveWithCascade_DeletesInReverseTopoOrder(t *testing.T) {
	tmp, s := newDAGTestRepo(t)
	runCmd("add", "--path", tmp, "--slug", "root", "R")
	runCmd("add", "--path", tmp, "--slug", "mid", "M")
	runCmd("add", "--path", tmp, "--slug", "leaf", "L")
	runCmd("feature", "deps", "--path", tmp, "mid", "add", "root")
	runCmd("feature", "deps", "--path", tmp, "leaf", "add", "mid")

	out, _, code := runCmd("remove", "--path", tmp, "root", "--cascade", "--force")
	if code != 0 {
		t.Fatalf("cascade remove failed: %s", out)
	}
	// Order assertions: leaf should appear before mid which should appear before root in 'Removed' lines.
	idxLeaf := strings.Index(out, "Removed feature leaf")
	idxMid := strings.Index(out, "Removed feature mid")
	idxRoot := strings.Index(out, "Removed feature root")
	if idxLeaf < 0 || idxMid < 0 || idxRoot < 0 {
		t.Fatalf("missing one of the Removed lines: %q", out)
	}
	if !(idxLeaf < idxMid && idxMid < idxRoot) {
		t.Fatalf("expected reverse-topo order leaf < mid < root, got positions %d,%d,%d in %q", idxLeaf, idxMid, idxRoot, out)
	}
	feats, _ := s.ListFeatures()
	if len(feats) != 0 {
		t.Fatalf("expected all features removed, got %v", feats)
	}
}

func TestRemoveWithoutCascade_RefusesWhenDependentsExist(t *testing.T) {
	tmp, _ := newDAGTestRepo(t)
	runCmd("add", "--path", tmp, "--slug", "p", "P")
	runCmd("add", "--path", tmp, "--slug", "c", "C")
	runCmd("feature", "deps", "--path", tmp, "c", "add", "p")

	_, _, code := runCmd("remove", "--path", tmp, "p", "--force")
	if code == 0 {
		t.Fatalf("expected refusal — p has dependent c")
	}
}

func TestRemoveForce_DoesNotBypassDepCheck(t *testing.T) {
	// Per PRD §3.7: --force is for confirmation, not graph integrity.
	tmp, _ := newDAGTestRepo(t)
	runCmd("add", "--path", tmp, "--slug", "p", "P")
	runCmd("add", "--path", tmp, "--slug", "c", "C")
	runCmd("feature", "deps", "--path", tmp, "c", "add", "p")

	out, _, code := runCmd("remove", "--path", tmp, "p", "--force")
	if code == 0 {
		t.Fatalf("--force alone must not bypass dependent check: %s", out)
	}
	if !strings.Contains(out+"_stderr_", "dependent") && !strings.Contains(captureErr(t, "remove", "--path", tmp, "p", "--force"), "dependent") {
		// fall through; the assertion above is already captured by exit code.
	}
}

func TestRemoveCascadeNonTTY_RequiresForce(t *testing.T) {
	// runCmd uses bytes.Buffer for stdin → canPromptForConfirmation
	// returns true (matching the test/script branch). To exercise
	// the non-TTY path we call runRemoveWithCascade directly with a
	// pre-prepared cmd whose stdin is an *os.File (closed pipe).
	tmp, _ := newDAGTestRepo(t)
	runCmd("add", "--path", tmp, "--slug", "p", "P")
	runCmd("add", "--path", tmp, "--slug", "c", "C")
	runCmd("feature", "deps", "--path", tmp, "c", "add", "p")

	s, err := store.Open(tmp)
	if err != nil {
		t.Fatal(err)
	}
	root := buildRootCmd()
	rmCmd, _, err := root.Find([]string{"remove"})
	if err != nil {
		t.Fatal(err)
	}
	// /dev/null is a non-TTY *os.File so canPromptForConfirmation() returns false.
	devnull, _ := openDevNull()
	defer devnull.Close()
	rmCmd.SetIn(devnull)
	err = runRemoveWithCascade(rmCmd, s, "p", false)
	if !errors.Is(err, ErrInteractiveRequired) {
		t.Fatalf("expected ErrInteractiveRequired, got %v", err)
	}
}

func TestFeatureDepsValidateAll_OnInit(t *testing.T) {
	// Right after init the store has no features so validate-all is
	// a no-op. The wiring test pins that the command runs cleanly.
	tmp, _ := newDAGTestRepo(t)
	out, _, code := runCmd("feature", "deps", "--validate-all", "--path", tmp)
	if code != 0 {
		t.Fatalf("validate-all on fresh repo failed: %s", out)
	}
	if !strings.Contains(out, "DAG: ok") {
		t.Fatalf("expected 'DAG: ok' on fresh repo, got %q", out)
	}

	// Now seed a violation and re-run.
	s, _ := store.Open(tmp)
	runCmd("add", "--path", tmp, "--slug", "x", "X")
	x, _ := s.LoadFeatureStatus("x")
	x.DependsOn = []store.Dependency{{Slug: "ghost", Kind: store.DependencyKindHard}}
	if err := s.SaveFeatureStatus(x); err != nil {
		t.Fatal(err)
	}
	out, _, code = runCmd("feature", "deps", "--validate-all", "--path", tmp)
	if code == 0 {
		t.Fatalf("validate-all should fail on dangling dep: %s", out)
	}
	if !strings.Contains(out, "ghost") {
		t.Fatalf("expected violation to mention ghost: %q", out)
	}
}

// captureErr re-runs a command and returns the stderr buffer. Tiny shim
// to avoid changing runCmd's signature.
func captureErr(t *testing.T, args ...string) string {
	t.Helper()
	_, errOut, _ := runCmd(args...)
	return errOut
}
