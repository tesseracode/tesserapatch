package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/testutil"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

type unapplyFixture struct {
	dir        string
	store      *store.Store
	slug       string
	patch      []byte
	generation []byte
}

func newUnapplyFixture(t *testing.T, state store.FeatureState) unapplyFixture {
	t.Helper()
	testutil.PinGitAutoGCOff()
	dir := t.TempDir()
	gitInitTestRepo(t, dir)
	baseCommit := gitHead(t, dir)

	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	feature, err := s.AddFeature(store.AddFeatureInput{
		Title:   "Feature",
		Slug:    "feature",
		Request: "feature request",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\nfeature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch, err := gitutil.CapturePatch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(patch) == "" {
		t.Fatal("fixture patch is empty")
	}
	if err := s.WriteArtifact(feature.Slug, "post-apply.patch", patch); err != nil {
		t.Fatal(err)
	}

	feature.State = state
	feature.Apply.HasPatch = true
	feature.Apply.BaseCommit = baseCommit
	feature.Verify = &store.VerifyRecord{
		VerifiedAt: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Passed:     true,
	}
	if err := s.SaveFeatureStatus(feature); err != nil {
		t.Fatal(err)
	}

	generation := []byte("{\"sentinel\":\"unchanged\"}\n")
	generationPath := filepath.Join(dir, ".tpatch", "features", feature.Slug, "patch-generations.json")
	if err := os.WriteFile(generationPath, generation, 0o644); err != nil {
		t.Fatal(err)
	}
	runUnapplyGit(t, dir, "add", "-A")
	runUnapplyGit(t, dir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "feature applied")

	return unapplyFixture{
		dir:        dir,
		store:      s,
		slug:       feature.Slug,
		patch:      []byte(patch),
		generation: generation,
	}
}

func TestDependencyEdgeCLIAllowsUnappliedParent(t *testing.T) {
	dir, s := newRejectRepo(t, map[string]store.FeatureState{
		"parent": store.StateUnapplied,
		"child":  store.StateRequested,
	})
	if _, stderr, code := runRJ("feature", "deps", "child", "add", "parent:hard", "--path", dir); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	child, err := s.LoadFeatureStatus("child")
	if err != nil {
		t.Fatal(err)
	}
	if len(child.DependsOn) != 1 || child.DependsOn[0].Slug != "parent" {
		t.Fatalf("depends_on = %+v", child.DependsOn)
	}
}

func TestFeatureUnapplyCleanWritesAuditAndPreservesCanonicalMetadata(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateApplied)
	statusBefore, err := os.ReadFile(filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	featureDir := filepath.Join(fx.dir, ".tpatch", "features", fx.slug)
	patchesDir := filepath.Join(featureDir, "patches")
	patchesBefore := directoryEntryNames(t, patchesDir)

	out, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir, "--actor", "agent@example.com")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s stdout=%s", code, stderr, out)
	}
	if !strings.Contains(out, "state applied → unapplied") {
		t.Fatalf("success output missing state transition:\n%s", out)
	}

	if got, err := os.ReadFile(filepath.Join(fx.dir, "README.md")); err != nil || string(got) != "# Test\n" {
		t.Fatalf("README after unapply = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(fx.dir, "feature.txt")); !os.IsNotExist(err) {
		t.Fatalf("feature.txt must be absent, err=%v", err)
	}

	status, err := fx.store.LoadFeatureStatus(fx.slug)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != store.StateUnapplied || status.LastCommand != "feature unapply" {
		t.Fatalf("status = state:%q command:%q", status.State, status.LastCommand)
	}
	if status.Verify != nil {
		t.Fatal("successful unapply must clear Verify")
	}
	if !status.Apply.HasPatch {
		t.Fatal("unapply must preserve apply.has_patch")
	}
	if !strings.Contains(status.Notes, "artifacts/unapply/") {
		t.Fatalf("status note does not reference audit session: %q", status.Notes)
	}
	if bytes.Equal(statusBefore, mustRead(t, filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "status.json"))) {
		t.Fatal("status.json did not change")
	}
	if info, err := os.Stat(featureDir); err != nil || !info.IsDir() {
		t.Fatalf("feature directory missing after unapply: %v", err)
	}
	if patchesAfter := directoryEntryNames(t, patchesDir); strings.Join(patchesAfter, "\n") != strings.Join(patchesBefore, "\n") {
		t.Fatalf("patches directory changed:\nbefore=%v\nafter=%v", patchesBefore, patchesAfter)
	}

	if got := mustRead(t, filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "post-apply.patch")); !bytes.Equal(got, fx.patch) {
		t.Fatal("canonical patch changed")
	}
	if got := mustRead(t, filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "patch-generations.json")); !bytes.Equal(got, fx.generation) {
		t.Fatal("patch-generations.json changed")
	}

	attempts, err := filepath.Glob(filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "unapply", "ua_*"))
	if err != nil || len(attempts) != 1 {
		t.Fatalf("attempt directories = %v, %v", attempts, err)
	}
	sessionBytes := mustRead(t, filepath.Join(attempts[0], "unapply-session.json"))
	reverseBytes := mustRead(t, filepath.Join(attempts[0], "reverse.patch"))
	if len(reverseBytes) == 0 {
		t.Fatal("reverse.patch is empty")
	}
	if err := gitutil.ValidatePatchReverse(fx.dir, string(reverseBytes)); err != nil {
		t.Fatalf("reverse.patch does not round-trip against the unapplied tree: %v", err)
	}

	var session unapplySession
	if err := json.Unmarshal(sessionBytes, &session); err != nil {
		t.Fatal(err)
	}
	if session.Version != 1 || session.Feature != fx.slug || session.Mode != "patch" ||
		session.Actor != "agent@example.com" || session.PreviousState != store.StateApplied ||
		session.Result != string(store.StateUnapplied) {
		t.Fatalf("unexpected session: %+v", session)
	}
	if !regexp.MustCompile(`^ua_[0-9a-f]{12}$`).MatchString(session.AttemptID) {
		t.Fatalf("attempt_id = %q", session.AttemptID)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(session.CanonicalPatchSHA256) {
		t.Fatalf("canonical_patch_sha256 = %q", session.CanonicalPatchSHA256)
	}
	if session.DependencyBlockers == nil || session.Preflight.ConflictMarkers == nil || session.Preflight.Leftovers == nil {
		t.Fatalf("fixed-envelope arrays must encode as []: %+v", session)
	}
	if !session.Preflight.CleanTree || len(session.TouchedPaths) != 2 {
		t.Fatalf("preflight/touched paths = %+v / %v", session.Preflight, session.TouchedPaths)
	}
	if strings.Join(session.TouchedPaths, ",") != "README.md,feature.txt" {
		t.Fatalf("touched_paths = %v", session.TouchedPaths)
	}
	assertJSONFieldOrder(t, string(sessionBytes), []string{
		"version", "feature", "attempt_id", "attempted_at", "mode", "actor",
		"previous_state", "result", "canonical_patch_sha256", "reverse_patch",
		"touched_paths", "dependency_blockers", "preflight",
	})
}

func TestFeatureUnapplyUnicodeDeletedPathAndRollback(t *testing.T) {
	t.Run("success-captures-recreated-untracked-path", func(t *testing.T) {
		fx := newUnicodeDeleteUnapplyFixture(t)
		if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 0 {
			t.Fatalf("code=%d stderr=%s", code, stderr)
		}
		if got := string(mustRead(t, filepath.Join(fx.dir, "café.txt"))); got != "unicode\n" {
			t.Fatalf("unicode file after unapply = %q", got)
		}
		attempts, _ := filepath.Glob(filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "unapply", "ua_*"))
		var session unapplySession
		if err := json.Unmarshal(mustRead(t, filepath.Join(attempts[0], "unapply-session.json")), &session); err != nil {
			t.Fatal(err)
		}
		if strings.Join(session.TouchedPaths, ",") != "café.txt" {
			t.Fatalf("touched_paths = %v", session.TouchedPaths)
		}
		reverse := string(mustRead(t, filepath.Join(attempts[0], "reverse.patch")))
		if got := gitutil.PathsAffectedByPatch(reverse); len(got) != 1 || got[0] != "café.txt" {
			t.Fatalf("reverse.patch paths = %v\n%s", got, reverse)
		}
	})

	t.Run("status-failure-restores-absence", func(t *testing.T) {
		fx := newUnicodeDeleteUnapplyFixture(t)
		rt := defaultUnapplyRuntime()
		rt.newAttemptID = func() (string, error) { return "ua_dddddddddddd", nil }
		rt.saveStatus = func(*store.Store, store.FeatureStatus) error {
			return errors.New("injected status failure")
		}
		cmd := &cobra.Command{}
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		if err := runFeatureUnapplyWithRuntime(cmd, fx.store, fx.slug, unapplyOptions{Mode: "patch"}, rt); err == nil {
			t.Fatal("expected status failure")
		}
		if _, err := os.Stat(filepath.Join(fx.dir, "café.txt")); !os.IsNotExist(err) {
			t.Fatalf("unicode path residue remains after rollback, err=%v", err)
		}
	})
}

func TestFeatureUnapplyPathspecMagicDeletedPaths(t *testing.T) {
	for _, name := range []string{":(literal)gone.txt", "*.txt", "[x].txt"} {
		t.Run(name, func(t *testing.T) {
			fx := newDeletedPathUnapplyFixture(t, name, "magic", "magic\n")
			if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 0 {
				t.Fatalf("code=%d stderr=%s", code, stderr)
			}
			if got := string(mustRead(t, filepath.Join(fx.dir, name))); got != "magic\n" {
				t.Fatalf("%s after unapply = %q", name, got)
			}
			attempts, _ := filepath.Glob(filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "unapply", "ua_*"))
			var session unapplySession
			if err := json.Unmarshal(mustRead(t, filepath.Join(attempts[0], "unapply-session.json")), &session); err != nil {
				t.Fatal(err)
			}
			if len(session.TouchedPaths) != 1 || session.TouchedPaths[0] != name {
				t.Fatalf("touched_paths = %v", session.TouchedPaths)
			}
		})
	}
}

func TestUnappliedCaptureCommandsRefusePendingBaseline(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateApplied)
	if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 0 {
		t.Fatalf("unapply code=%d stderr=%s", code, stderr)
	}
	patchPath := filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "post-apply.patch")
	before := mustRead(t, patchPath)
	commands := [][]string{
		{"cycle", fx.slug, "--path", fx.dir},
		{"feature", "patch", "refresh", fx.slug, "--path", fx.dir},
		{"feature", "patch", "fixup", fx.slug, "--path", fx.dir, "--reason", "fix"},
	}
	for _, args := range commands {
		if _, stderr, code := runRJ(args...); code != 3 || !strings.Contains(stderr, "tpatch apply feature") {
			t.Fatalf("%v code=%d stderr=%s", args, code, stderr)
		}
		if got := mustRead(t, patchPath); !bytes.Equal(got, before) {
			t.Fatalf("%v rewrote canonical patch", args)
		}
	}
}

func TestUnappliedPhaseCommandsRefuseBeforeArtifactsOrStateDrift(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateApplied)
	if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 0 {
		t.Fatalf("unapply code=%d stderr=%s", code, stderr)
	}
	statusPath := filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "status.json")
	before := mustRead(t, statusPath)
	commands := [][]string{
		{"analyze", fx.slug, "--path", fx.dir, "--manual"},
		{"define", fx.slug, "--path", fx.dir, "--manual"},
		{"explore", fx.slug, "--path", fx.dir, "--manual"},
		{"implement", fx.slug, "--path", fx.dir, "--manual"},
	}
	for _, args := range commands {
		if _, stderr, code := runRJ(args...); code != 3 || !strings.Contains(stderr, "tpatch apply feature") {
			t.Fatalf("%v code=%d stderr=%s", args, code, stderr)
		}
		if got := mustRead(t, statusPath); !bytes.Equal(got, before) {
			t.Fatalf("%v changed status.json", args)
		}
	}
}

func TestPendingUnapplyBaselineGuardSurvivesStateDrift(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateApplied)
	if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 0 {
		t.Fatalf("unapply code=%d stderr=%s", code, stderr)
	}
	status, err := fx.store.LoadFeatureStatus(fx.slug)
	if err != nil {
		t.Fatal(err)
	}
	status.State = store.StateImplementing // Simulate a hand-edited/legacy phase drift.
	if err := fx.store.SaveFeatureStatus(status); err != nil {
		t.Fatal(err)
	}
	patchPath := filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "post-apply.patch")
	before := mustRead(t, patchPath)
	if _, stderr, code := runRJ("record", fx.slug, "--path", fx.dir); code != 3 || !strings.Contains(stderr, "unapply reversal") {
		t.Fatalf("record code=%d stderr=%s", code, stderr)
	}
	if got := mustRead(t, patchPath); !bytes.Equal(got, before) {
		t.Fatal("state drift bypassed canonical-patch guard")
	}
	if _, stderr, code := runRJ("apply", fx.slug, "--path", fx.dir, "--mode", "done"); code != 1 || !strings.Contains(stderr, "reapply") {
		t.Fatalf("apply done code=%d stderr=%s", code, stderr)
	}
	if got := mustRead(t, patchPath); !bytes.Equal(got, before) {
		t.Fatal("state drift let apply --mode done rewrite canonical patch")
	}
}

func TestFeatureUnapplyDryRunReportsBlockersWithoutMutation(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateApplied)
	if err := os.WriteFile(filepath.Join(fx.dir, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "status.json")
	statusBefore := mustRead(t, statusPath)
	gitBefore := runUnapplyGit(t, fx.dir, "status", "--porcelain")

	out, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir, "--dry-run")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	for _, want := range []string{"touched_paths:", "dependency_blockers:", "clean_tree: false", "reverse_apply_check:", "worktree_preview:", "planned_artifacts:", "working tree is dirty"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run output missing %q:\n%s", want, out)
		}
	}
	if got := mustRead(t, statusPath); !bytes.Equal(got, statusBefore) {
		t.Fatal("dry-run changed status.json")
	}
	if got := runUnapplyGit(t, fx.dir, "status", "--porcelain"); got != gitBefore {
		t.Fatalf("dry-run changed git status:\nbefore=%q\nafter=%q", gitBefore, got)
	}
	if matches, _ := filepath.Glob(filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "unapply", "*")); len(matches) != 0 {
		t.Fatalf("dry-run wrote artifacts: %v", matches)
	}
}

func TestFeatureUnapplyDryRunStateAndDependentBlockersExitZero(t *testing.T) {
	t.Run("wrong-state", func(t *testing.T) {
		fx := newUnapplyFixture(t, store.StateRequested)
		statusPath := filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "status.json")
		before := mustRead(t, statusPath)
		out, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir, "--dry-run")
		if code != 0 || !strings.Contains(out, `state "requested" is not unapply-eligible`) {
			t.Fatalf("code=%d stderr=%s stdout=%s", code, stderr, out)
		}
		if got := mustRead(t, statusPath); !bytes.Equal(got, before) {
			t.Fatal("wrong-state dry-run changed status")
		}
	})
	t.Run("hard-dependent", func(t *testing.T) {
		fx := newUnapplyFixture(t, store.StateApplied)
		child, err := fx.store.AddFeature(store.AddFeatureInput{Title: "Child", Slug: "child", Request: "child"})
		if err != nil {
			t.Fatal(err)
		}
		child.DependsOn = []store.Dependency{{Slug: fx.slug, Kind: store.DependencyKindHard}}
		if err := fx.store.SaveFeatureStatus(child); err != nil {
			t.Fatal(err)
		}
		runUnapplyGit(t, fx.dir, "add", "-A")
		runUnapplyGit(t, fx.dir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "add hard dependent")
		out, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir, "--dry-run")
		if code != 0 || !strings.Contains(out, "hard dependent child") {
			t.Fatalf("code=%d stderr=%s stdout=%s", code, stderr, out)
		}
	})
}

func TestFeatureUnapplySourceStateMatrix(t *testing.T) {
	permitted := []store.FeatureState{
		store.StateApplied, store.StateActive, store.StateReconciling, store.StateReconcilingShadow,
	}
	for _, state := range permitted {
		t.Run("permitted-"+string(state), func(t *testing.T) {
			fx := newUnapplyFixture(t, state)
			_, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir)
			if code != 0 {
				t.Fatalf("state %s code=%d stderr=%s", state, code, stderr)
			}
		})
	}

	refused := []store.FeatureState{
		store.StateRequested, store.StateAnalyzed, store.StateDefined, store.StateImplementing,
		store.StateUnapplied, store.StateRejected, store.StateBlocked, store.StateUpstreamMerged,
	}
	for _, state := range refused {
		t.Run("refused-"+string(state), func(t *testing.T) {
			fx := newUnapplyFixture(t, state)
			_, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir)
			if code != 3 {
				t.Fatalf("state %s code=%d want=3 stderr=%s", state, code, stderr)
			}
			status, err := fx.store.LoadFeatureStatus(fx.slug)
			if err != nil {
				t.Fatal(err)
			}
			if status.State != state {
				t.Fatalf("state changed from %s to %s", state, status.State)
			}
		})
	}
}

func TestFeatureUnapplyRealPreImplementationFeatureRefusesBeforePatchLoad(t *testing.T) {
	dir, _ := newRejectRepo(t, map[string]store.FeatureState{"fresh": store.StateRequested})
	if _, stderr, code := runRJ("feature", "unapply", "fresh", "--path", dir); code != 3 || !strings.Contains(stderr, "state \"requested\"") {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if _, _, code := runRJ("feature", "unapply", "fresh", "--path", dir, "--dry-run"); code != 2 {
		t.Fatalf("dry-run missing-patch code=%d, want 2", code)
	}
}

func TestFeatureUnapplyDependentPolicy(t *testing.T) {
	tests := []struct {
		name      string
		kind      string
		allowSoft bool
		wantCode  int
	}{
		{name: "hard", kind: store.DependencyKindHard, wantCode: 3},
		{name: "soft-refused", kind: store.DependencyKindSoft, wantCode: 3},
		{name: "soft-allowed", kind: store.DependencyKindSoft, allowSoft: true, wantCode: 0},
		{name: "supersedes", kind: store.DependencyKindSupersedes, wantCode: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fx := newUnapplyFixture(t, store.StateApplied)
			child, err := fx.store.AddFeature(store.AddFeatureInput{Title: "Child", Slug: "child", Request: "child"})
			if err != nil {
				t.Fatal(err)
			}
			child.DependsOn = []store.Dependency{{Slug: fx.slug, Kind: tt.kind}}
			if err := fx.store.SaveFeatureStatus(child); err != nil {
				t.Fatal(err)
			}
			runUnapplyGit(t, fx.dir, "add", "-A")
			runUnapplyGit(t, fx.dir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "add dependent")

			args := []string{"feature", "unapply", fx.slug, "--path", fx.dir}
			if tt.allowSoft {
				args = append(args, "--allow-soft-dependents")
			}
			_, stderr, code := runRJ(args...)
			if code != tt.wantCode {
				t.Fatalf("code=%d want=%d stderr=%s", code, tt.wantCode, stderr)
			}
		})
	}
}

func TestFeatureUnapplyTerminalDependentsDoNotBlock(t *testing.T) {
	for _, state := range []store.FeatureState{store.StateRejected, store.StateUpstreamMerged} {
		t.Run(string(state), func(t *testing.T) {
			fx := newUnapplyFixture(t, store.StateApplied)
			child, err := fx.store.AddFeature(store.AddFeatureInput{Title: "Child", Slug: "child", Request: "child"})
			if err != nil {
				t.Fatal(err)
			}
			child.State = state
			child.DependsOn = []store.Dependency{{Slug: fx.slug, Kind: store.DependencyKindHard}}
			if state == store.StateRejected {
				child.Rejection = &store.RejectionStatus{Reason: "obsolete", Note: "done", Actor: "test"}
			}
			if err := fx.store.SaveFeatureStatus(child); err != nil {
				t.Fatal(err)
			}
			runUnapplyGit(t, fx.dir, "add", "-A")
			runUnapplyGit(t, fx.dir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "add terminal dependent")
			if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 0 {
				t.Fatalf("code=%d stderr=%s", code, stderr)
			}
		})
	}
}

func TestFeatureUnapplyValidationAndHelp(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateApplied)

	if _, _, code := runRJ("feature", "unapply", "../bad", "--path", fx.dir); code != 2 {
		t.Fatalf("invalid slug code=%d, want 2", code)
	}
	if _, _, code := runRJ("feature", "unapply", "missing", "--path", fx.dir, "--dry-run"); code != 2 {
		t.Fatalf("missing feature code=%d, want 2", code)
	}
	if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir, "--mode", "landed-commit"); code != 2 || !strings.Contains(stderr, "future release") {
		t.Fatalf("landed mode code=%d stderr=%s", code, stderr)
	}
	if _, _, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir, "--actor", "bad\nactor"); code != 2 {
		t.Fatalf("bad actor code=%d, want 2", code)
	}

	unapplyHelp, _, code := runRJ("feature", "unapply", "--help")
	if code != 0 || !strings.Contains(unapplyHelp, "Use 'tpatch apply <slug>' to reapply a feature that has been unapplied.") {
		t.Fatalf("unapply help code=%d:\n%s", code, unapplyHelp)
	}
	applyHelp, _, code := runRJ("apply", "--help")
	if code != 0 || !strings.Contains(applyHelp, "Use 'tpatch feature unapply <slug>' to remove a feature's patch from the working tree.") {
		t.Fatalf("apply help code=%d:\n%s", code, applyHelp)
	}
}

func TestFeatureUnapplyInvalidFeatureArtifactsExitTwo(t *testing.T) {
	t.Run("missing-patch", func(t *testing.T) {
		fx := newUnapplyFixture(t, store.StateApplied)
		if err := os.Remove(filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "post-apply.patch")); err != nil {
			t.Fatal(err)
		}
		if _, _, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir, "--dry-run"); code != 2 {
			t.Fatalf("code=%d, want 2", code)
		}
	})
	t.Run("empty-patch", func(t *testing.T) {
		fx := newUnapplyFixture(t, store.StateApplied)
		if err := os.WriteFile(filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "post-apply.patch"), []byte("\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir, "--dry-run"); code != 2 {
			t.Fatalf("code=%d, want 2", code)
		}
	})
	t.Run("malformed-status", func(t *testing.T) {
		fx := newUnapplyFixture(t, store.StateApplied)
		if err := os.WriteFile(filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "status.json"), []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir, "--dry-run"); code != 2 {
			t.Fatalf("code=%d, want 2", code)
		}
	})
}

func TestFeatureUnapplyPreflightRefusals(t *testing.T) {
	t.Run("dirty-tree", func(t *testing.T) {
		fx := newUnapplyFixture(t, store.StateApplied)
		if err := os.WriteFile(filepath.Join(fx.dir, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 3 || !strings.Contains(stderr, "working tree is dirty") {
			t.Fatalf("code=%d stderr=%s", code, stderr)
		}
	})
	t.Run("conflict-markers", func(t *testing.T) {
		fx := newUnapplyFixture(t, store.StateApplied)
		body := "<<<<<<< ours\nx\n=======\ny\n>>>>>>> theirs\n"
		if err := os.WriteFile(filepath.Join(fx.dir, "conflict.txt"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		runUnapplyGit(t, fx.dir, "add", "conflict.txt")
		runUnapplyGit(t, fx.dir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "conflict marker fixture")
		if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 3 || !strings.Contains(stderr, "merge conflict markers") {
			t.Fatalf("code=%d stderr=%s", code, stderr)
		}
	})
	t.Run("leftovers", func(t *testing.T) {
		fx := newUnapplyFixture(t, store.StateApplied)
		if err := os.WriteFile(filepath.Join(fx.dir, "stale.orig"), []byte("stale\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 3 || !strings.Contains(stderr, ".orig/.rej leftovers") {
			t.Fatalf("code=%d stderr=%s", code, stderr)
		}
	})
	t.Run("mid-rebase", func(t *testing.T) {
		fx := newUnapplyFixture(t, store.StateApplied)
		gitDir := strings.TrimSpace(runUnapplyGit(t, fx.dir, "rev-parse", "--git-dir"))
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(fx.dir, gitDir)
		}
		head := strings.TrimSpace(runUnapplyGit(t, fx.dir, "rev-parse", "HEAD"))
		if err := os.WriteFile(filepath.Join(gitDir, "REBASE_HEAD"), []byte(head+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 3 || !strings.Contains(stderr, "mid-rebase") {
			t.Fatalf("code=%d stderr=%s", code, stderr)
		}
	})
	t.Run("mid-merge", func(t *testing.T) {
		fx := newUnapplyFixture(t, store.StateApplied)
		gitDir := strings.TrimSpace(runUnapplyGit(t, fx.dir, "rev-parse", "--git-dir"))
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(fx.dir, gitDir)
		}
		head := strings.TrimSpace(runUnapplyGit(t, fx.dir, "rev-parse", "HEAD"))
		if err := os.WriteFile(filepath.Join(gitDir, "MERGE_HEAD"), []byte(head+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 3 || !strings.Contains(stderr, "mid-merge") {
			t.Fatalf("code=%d stderr=%s", code, stderr)
		}
	})
	t.Run("mid-cherry-pick", func(t *testing.T) {
		fx := newUnapplyFixture(t, store.StateApplied)
		gitDir := strings.TrimSpace(runUnapplyGit(t, fx.dir, "rev-parse", "--git-dir"))
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(fx.dir, gitDir)
		}
		head := strings.TrimSpace(runUnapplyGit(t, fx.dir, "rev-parse", "HEAD"))
		if err := os.WriteFile(filepath.Join(gitDir, "CHERRY_PICK_HEAD"), []byte(head+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 3 || !strings.Contains(stderr, "mid-cherry-pick") {
			t.Fatalf("code=%d stderr=%s", code, stderr)
		}
	})
	t.Run("reverse-check-failure", func(t *testing.T) {
		fx := newUnapplyFixture(t, store.StateApplied)
		if err := os.WriteFile(filepath.Join(fx.dir, "README.md"), []byte("# Test\nfeature drifted\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runUnapplyGit(t, fx.dir, "add", "README.md")
		runUnapplyGit(t, fx.dir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "drift feature")
		statusPath := filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "status.json")
		statusBefore := mustRead(t, statusPath)
		gitBefore := runUnapplyGit(t, fx.dir, "status", "--porcelain=v1")
		if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 3 || !strings.Contains(stderr, "does not reverse-apply cleanly") {
			t.Fatalf("code=%d stderr=%s", code, stderr)
		}
		if got := mustRead(t, statusPath); !bytes.Equal(got, statusBefore) {
			t.Fatal("reverse-check refusal changed status.json")
		}
		if got := runUnapplyGit(t, fx.dir, "status", "--porcelain=v1"); got != gitBefore {
			t.Fatalf("reverse-check refusal changed worktree/index:\nbefore=%q\nafter=%q", gitBefore, got)
		}
		if matches, _ := filepath.Glob(filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "unapply", "*")); len(matches) != 0 {
			t.Fatalf("reverse-check refusal wrote artifacts: %v", matches)
		}
	})
}

func TestFeatureUnapplyRollbackFailures(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *unapplyRuntime, unapplyRuntime, unapplyFixture)
	}{
		{
			name: "real-reverse-failure",
			mutate: func(t *testing.T, rt *unapplyRuntime, _ unapplyRuntime, fx unapplyFixture) {
				rt.reverseApply = func(_, _ string) error {
					if err := os.WriteFile(filepath.Join(fx.dir, "README.md"), []byte("partial\n"), 0o644); err != nil {
						t.Fatal(err)
					}
					if err := os.Remove(filepath.Join(fx.dir, "feature.txt")); err != nil {
						t.Fatal(err)
					}
					return errors.New("injected reverse failure")
				}
			},
		},
		{
			name: "artifact-write-failure",
			mutate: func(_ *testing.T, rt *unapplyRuntime, original unapplyRuntime, _ unapplyFixture) {
				writes := 0
				rt.writeArtifact = func(s *store.Store, slug, name, content string) error {
					writes++
					if writes == 2 {
						return errors.New("injected artifact failure")
					}
					return original.writeArtifact(s, slug, name, content)
				}
			},
		},
		{
			name: "status-write-failure",
			mutate: func(_ *testing.T, rt *unapplyRuntime, _ unapplyRuntime, _ unapplyFixture) {
				rt.saveStatus = func(*store.Store, store.FeatureStatus) error {
					return errors.New("injected status failure")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fx := newUnapplyFixture(t, store.StateApplied)
			statusPath := filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "status.json")
			statusBefore := mustRead(t, statusPath)
			readmeBefore := mustRead(t, filepath.Join(fx.dir, "README.md"))
			featureBefore := mustRead(t, filepath.Join(fx.dir, "feature.txt"))

			original := defaultUnapplyRuntime()
			rt := original
			rt.newAttemptID = func() (string, error) { return "ua_aaaaaaaaaaaa", nil }
			rt.now = func() time.Time { return time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC) }
			tt.mutate(t, &rt, original, fx)

			cmd := &cobra.Command{}
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			err := runFeatureUnapplyWithRuntime(cmd, fx.store, fx.slug, unapplyOptions{Mode: "patch"}, rt)
			if err == nil {
				t.Fatal("expected injected failure")
			}

			if got := mustRead(t, filepath.Join(fx.dir, "README.md")); !bytes.Equal(got, readmeBefore) {
				t.Fatalf("README not restored: %q", got)
			}
			if got := mustRead(t, filepath.Join(fx.dir, "feature.txt")); !bytes.Equal(got, featureBefore) {
				t.Fatalf("feature.txt not restored: %q", got)
			}
			if got := mustRead(t, statusPath); !bytes.Equal(got, statusBefore) {
				t.Fatalf("status bytes changed after failure:\n%s", got)
			}
			status, loadErr := fx.store.LoadFeatureStatus(fx.slug)
			if loadErr != nil || status.State != store.StateApplied {
				t.Fatalf("status after failure = %q, %v", status.State, loadErr)
			}
			attemptDir := filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "unapply", "ua_aaaaaaaaaaaa")
			if _, statErr := os.Stat(attemptDir); !os.IsNotExist(statErr) {
				t.Fatalf("partial attempt directory remains, err=%v", statErr)
			}
		})
	}
}

func TestFeatureUnapplySnapshotFailureIsInternalError(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateApplied)
	rt := defaultUnapplyRuntime()
	rt.snapshot = func(string, []string) (gitutil.WorktreeSnapshot, error) {
		return gitutil.WorktreeSnapshot{}, errors.New("injected snapshot IO failure")
	}
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := runFeatureUnapplyWithRuntime(cmd, fx.store, fx.slug, unapplyOptions{Mode: "patch"}, rt)
	if err == nil || asExitCodeError(err) != nil {
		t.Fatalf("snapshot error must use exit 1, got %v", err)
	}
}

func TestFeatureUnapplyRecordRefusesAndPreservesCanonicalPatch(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateApplied)
	if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 0 {
		t.Fatalf("unapply code=%d stderr=%s", code, stderr)
	}
	patchPath := filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "post-apply.patch")
	before := mustRead(t, patchPath)
	if _, stderr, code := runRJ("record", fx.slug, "--path", fx.dir); code != 3 || !strings.Contains(stderr, "tpatch apply feature") {
		t.Fatalf("record code=%d stderr=%s", code, stderr)
	}
	if got := mustRead(t, patchPath); !bytes.Equal(got, before) {
		t.Fatal("record from unapplied rewrote canonical patch")
	}
	status, err := fx.store.LoadFeatureStatus(fx.slug)
	if err != nil || status.State != store.StateUnapplied {
		t.Fatalf("status = %q, %v", status.State, err)
	}
}

func TestRecordAfterCommittedUnapplyBaselineCapturesNewWork(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateApplied)
	if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 0 {
		t.Fatalf("unapply code=%d stderr=%s", code, stderr)
	}
	runUnapplyGit(t, fx.dir, "add", "-A")
	runUnapplyGit(t, fx.dir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "commit unapplied baseline")
	if err := os.WriteFile(filepath.Join(fx.dir, "replacement.txt"), []byte("replacement\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runRJ("record", fx.slug, "--path", fx.dir); code != 0 {
		t.Fatalf("record code=%d stderr=%s", code, stderr)
	}
	patch := string(mustRead(t, filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "post-apply.patch")))
	if !strings.Contains(patch, "replacement.txt") {
		t.Fatalf("recorded patch misses new work:\n%s", patch)
	}
	if strings.Contains(patch, "feature.txt") {
		t.Fatalf("recorded patch recaptured the old unapply reversal:\n%s", patch)
	}
}

func TestFeatureUnapplyRenamePathsAndRollback(t *testing.T) {
	t.Run("success-audits-both-sides", func(t *testing.T) {
		fx := newRenameUnapplyFixture(t)
		if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 0 {
			t.Fatalf("code=%d stderr=%s", code, stderr)
		}
		if _, err := os.Stat(filepath.Join(fx.dir, "old.txt")); err != nil {
			t.Fatalf("old.txt was not restored by reverse rename: %v", err)
		}
		if _, err := os.Stat(filepath.Join(fx.dir, "new.txt")); !os.IsNotExist(err) {
			t.Fatalf("new.txt remains after reverse rename, err=%v", err)
		}
		attempts, _ := filepath.Glob(filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "unapply", "ua_*"))
		var session unapplySession
		if err := json.Unmarshal(mustRead(t, filepath.Join(attempts[0], "unapply-session.json")), &session); err != nil {
			t.Fatal(err)
		}
		if strings.Join(session.TouchedPaths, ",") != "new.txt,old.txt" {
			t.Fatalf("touched_paths = %v", session.TouchedPaths)
		}
		reverse := string(mustRead(t, filepath.Join(attempts[0], "reverse.patch")))
		if !strings.Contains(reverse, "old.txt") || !strings.Contains(reverse, "new.txt") {
			t.Fatalf("reverse.patch omits rename side:\n%s", reverse)
		}
	})

	t.Run("status-failure-restores-both-sides", func(t *testing.T) {
		fx := newRenameUnapplyFixture(t)
		rt := defaultUnapplyRuntime()
		rt.newAttemptID = func() (string, error) { return "ua_bbbbbbbbbbbb", nil }
		rt.saveStatus = func(*store.Store, store.FeatureStatus) error {
			return errors.New("injected status failure")
		}
		cmd := &cobra.Command{}
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		if err := runFeatureUnapplyWithRuntime(cmd, fx.store, fx.slug, unapplyOptions{Mode: "patch"}, rt); err == nil {
			t.Fatal("expected status failure")
		}
		if _, err := os.Stat(filepath.Join(fx.dir, "old.txt")); !os.IsNotExist(err) {
			t.Fatalf("old.txt residue remains after rollback, err=%v", err)
		}
		if got := string(mustRead(t, filepath.Join(fx.dir, "new.txt"))); got != "rename me\n" {
			t.Fatalf("new.txt after rollback = %q", got)
		}
	})
}

func TestFeatureUnapplySpacePathAndRollback(t *testing.T) {
	t.Run("success-audits-exact-path", func(t *testing.T) {
		fx := newSpaceUnapplyFixture(t)
		if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 0 {
			t.Fatalf("code=%d stderr=%s", code, stderr)
		}
		if got := string(mustRead(t, filepath.Join(fx.dir, "my file.txt"))); got != "base\n" {
			t.Fatalf("file after unapply = %q", got)
		}
		attempts, _ := filepath.Glob(filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "unapply", "ua_*"))
		var session unapplySession
		if err := json.Unmarshal(mustRead(t, filepath.Join(attempts[0], "unapply-session.json")), &session); err != nil {
			t.Fatal(err)
		}
		if strings.Join(session.TouchedPaths, ",") != "my file.txt" {
			t.Fatalf("touched_paths = %v", session.TouchedPaths)
		}
		reverse := string(mustRead(t, filepath.Join(attempts[0], "reverse.patch")))
		if !strings.Contains(reverse, "my file.txt") {
			t.Fatalf("reverse.patch omits spaced path:\n%s", reverse)
		}
	})

	t.Run("status-failure-restores-exact-path", func(t *testing.T) {
		fx := newSpaceUnapplyFixture(t)
		rt := defaultUnapplyRuntime()
		rt.newAttemptID = func() (string, error) { return "ua_cccccccccccc", nil }
		rt.saveStatus = func(*store.Store, store.FeatureStatus) error {
			return errors.New("injected status failure")
		}
		cmd := &cobra.Command{}
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		if err := runFeatureUnapplyWithRuntime(cmd, fx.store, fx.slug, unapplyOptions{Mode: "patch"}, rt); err == nil {
			t.Fatal("expected status failure")
		}
		if got := string(mustRead(t, filepath.Join(fx.dir, "my file.txt"))); got != "base\nfeature\n" {
			t.Fatalf("spaced path not restored: %q", got)
		}
	})
}

func TestDependencyEdgeOntoUnappliedParentRemainsAllowed(t *testing.T) {
	dir, s := newRejectRepo(t, map[string]store.FeatureState{
		"parent": store.StateUnapplied,
		"child":  store.StateRequested,
	})
	_ = dir
	err := store.ValidateDependencies(s, "child", []store.Dependency{{
		Slug: "parent",
		Kind: store.DependencyKindHard,
	}})
	if err != nil {
		t.Fatalf("edge creation onto unapplied parent must remain allowed: %v", err)
	}
}

func TestFeatureUnapplyLifecycleIntegrations(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateApplied)
	if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 0 {
		t.Fatalf("unapply code=%d stderr=%s", code, stderr)
	}

	statusOut, stderr, code := runRJ("status", "--path", fx.dir)
	if code != 0 || !strings.Contains(statusOut, "feature [unapplied]") {
		t.Fatalf("status code=%d stderr=%s:\n%s", code, stderr, statusOut)
	}
	includedOut, _, code := runRJ("status", "--path", fx.dir, "--include-rejected")
	if code != 0 || !strings.Contains(includedOut, "feature [unapplied]") {
		t.Fatalf("--include-rejected hid unapplied feature:\n%s", includedOut)
	}
	jsonOut, stderr, code := runRJ("status", "--path", fx.dir, "--json")
	if code != 0 || !strings.Contains(jsonOut, `"state": "unapplied"`) {
		t.Fatalf("status JSON code=%d stderr=%s:\n%s", code, stderr, jsonOut)
	}

	nextOut, stderr, code := runRJ("next", fx.slug, "--path", fx.dir, "--format", "harness-json")
	if code != 0 || !strings.Contains(nextOut, `"phase": "apply"`) || !strings.Contains(nextOut, "tpatch apply feature") {
		t.Fatalf("next code=%d stderr=%s:\n%s", code, stderr, nextOut)
	}
	if _, stderr, code := runRJ("land", fx.slug, "--path", fx.dir); code != 3 || !strings.Contains(stderr, "tpatch apply feature") {
		t.Fatalf("land code=%d stderr=%s", code, stderr)
	}

	if _, stderr, code := runRJ("reject", fx.slug, "--path", fx.dir,
		"--reason", "obsolete", "--note", "discard", "--evidence", "request.md"); code != 3 || !strings.Contains(stderr, "tpatch remove feature") {
		t.Fatalf("reject code=%d stderr=%s", code, stderr)
	}
	statusBeforeReopen := mustRead(t, filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "status.json"))
	if _, stderr, code := runRJ("reopen", fx.slug, "--path", fx.dir, "--note", "not rejected"); code != 3 {
		t.Fatalf("reopen code=%d stderr=%s", code, stderr)
	}
	if got := mustRead(t, filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "status.json")); !bytes.Equal(got, statusBeforeReopen) {
		t.Fatal("refused reopen changed status.json")
	}

	status, err := fx.store.LoadFeatureStatus(fx.slug)
	if err != nil {
		t.Fatal(err)
	}
	revisionsPath := fx.store.ReconcileRevisionsPath(fx.slug)
	err = applyConfirmUpstreamedTransition(fx.store, &status, &confirmUpstreamedTransition{})
	if exit := asExitCodeError(err); exit == nil || exit.ExitCode() != 3 || !strings.Contains(err.Error(), "tpatch apply feature") {
		t.Fatalf("confirm-upstreamed error = %v", err)
	}
	if _, statErr := os.Stat(revisionsPath); !os.IsNotExist(statErr) {
		t.Fatalf("confirm-upstreamed guard wrote revision log, err=%v", statErr)
	}

	rejected, err := fx.store.AddFeature(store.AddFeatureInput{Title: "Rejected", Slug: "rejected", Request: "rejected"})
	if err != nil {
		t.Fatal(err)
	}
	rejected.State = store.StateRejected
	rejected.Rejection = &store.RejectionStatus{Reason: "obsolete", Note: "done", Actor: "test"}
	if err := fx.store.SaveFeatureStatus(rejected); err != nil {
		t.Fatal(err)
	}
	index := string(mustRead(t, filepath.Join(fx.dir, ".tpatch", "FEATURES.md")))
	unappliedAt, rejectedAt := strings.Index(index, "## Unapplied"), strings.Index(index, "## Rejected")
	if unappliedAt < 0 || rejectedAt < 0 || unappliedAt >= rejectedAt {
		t.Fatalf("FEATURES.md section order is wrong:\n%s", index)
	}
}

func TestFeatureUnapplyReopenRejectsInconsistentLiveRejection(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateApplied)
	if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 0 {
		t.Fatalf("unapply code=%d stderr=%s", code, stderr)
	}
	status, err := fx.store.LoadFeatureStatus(fx.slug)
	if err != nil {
		t.Fatal(err)
	}
	status.Rejection = &store.RejectionStatus{Reason: "obsolete", Note: "bad", Actor: "test"}
	if err := fx.store.SaveFeatureStatus(status); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runRJ("reopen", fx.slug, "--path", fx.dir, "--note", "retry"); code != 1 || !strings.Contains(stderr, "inconsistent status") {
		t.Fatalf("reopen code=%d stderr=%s", code, stderr)
	}
}

func TestFeatureApplyReappliesUnappliedFeature(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateApplied)
	beforeStatus, err := fx.store.LoadFeatureStatus(fx.slug)
	if err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 0 {
		t.Fatalf("unapply code=%d stderr=%s", code, stderr)
	}
	recipe := `{
  "feature": "feature",
  "operations": [
    {
      "type": "replace-in-file",
      "path": "README.md",
      "search": "# Test\n",
      "replace": "# Test\nfeature\n"
    },
    {
      "type": "write-file",
      "path": "feature.txt",
      "content": "feature file\n",
      "preimage_hash": ""
    }
  ]
}
`
	if err := fx.store.WriteArtifact(fx.slug, "apply-recipe.json", recipe); err != nil {
		t.Fatal(err)
	}
	out, stderr, code := runRJ("apply", fx.slug, "--path", fx.dir)
	if code != 0 {
		t.Fatalf("apply code=%d stderr=%s stdout=%s", code, stderr, out)
	}
	status, err := fx.store.LoadFeatureStatus(fx.slug)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != store.StateApplied {
		t.Fatalf("state after reapply = %q, want applied", status.State)
	}
	if !status.Apply.HasPatch {
		t.Fatal("reapply cleared apply.has_patch despite preserving the canonical patch")
	}
	if status.Apply.BaseCommit != beforeStatus.Apply.BaseCommit {
		t.Fatalf("reapply changed base_commit from %q to %q", beforeStatus.Apply.BaseCommit, status.Apply.BaseCommit)
	}
	if got := mustRead(t, filepath.Join(fx.dir, "README.md")); string(got) != "# Test\nfeature\n" {
		t.Fatalf("README after reapply = %q", got)
	}
	if got := mustRead(t, filepath.Join(fx.dir, "feature.txt")); string(got) != "feature file\n" {
		t.Fatalf("feature.txt after reapply = %q", got)
	}
}

func TestFeatureApplyIncompleteReapplyPreservesCanonicalAndState(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateApplied)
	if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 0 {
		t.Fatalf("unapply code=%d stderr=%s", code, stderr)
	}
	patchPath := filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "post-apply.patch")
	canonicalBefore := mustRead(t, patchPath)
	incompleteRecipe := `{
  "feature": "feature",
  "operations": [
    {
      "type": "replace-in-file",
      "path": "README.md",
      "search": "# Test\n",
      "replace": "# Test\nfeature\n"
    }
  ]
}
`
	if err := fx.store.WriteArtifact(fx.slug, "apply-recipe.json", incompleteRecipe); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runRJ("apply", fx.slug, "--path", fx.dir); code != 1 || !strings.Contains(stderr, "reapply") {
		t.Fatalf("apply code=%d stderr=%s", code, stderr)
	}
	if got := mustRead(t, patchPath); !bytes.Equal(got, canonicalBefore) {
		t.Fatal("incomplete reapply overwrote canonical patch")
	}
	status, err := fx.store.LoadFeatureStatus(fx.slug)
	if err != nil || status.State != store.StateUnapplied {
		t.Fatalf("status = %q, %v", status.State, err)
	}
	if got := string(mustRead(t, filepath.Join(fx.dir, "README.md"))); got != "# Test\n" {
		t.Fatalf("failed reapply did not restore README: %q", got)
	}
	if _, err := os.Stat(filepath.Join(fx.dir, "feature.txt")); !os.IsNotExist(err) {
		t.Fatalf("failed reapply left feature.txt, err=%v", err)
	}
}

func TestFeatureRemoveUnappliedFeatureIsUnchanged(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateApplied)
	if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 0 {
		t.Fatalf("unapply code=%d stderr=%s", code, stderr)
	}
	if _, stderr, code := runRJ("remove", fx.slug, "--path", fx.dir, "--force"); code != 0 {
		t.Fatalf("remove code=%d stderr=%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(fx.dir, ".tpatch", "features", fx.slug)); !os.IsNotExist(err) {
		t.Fatalf("feature directory remains after remove, err=%v", err)
	}
}

func TestFeatureUnapplyActorPrecedenceInSession(t *testing.T) {
	tests := []struct {
		name      string
		flag      string
		env       string
		unsetGit  bool
		wantActor string
	}{
		{name: "flag", flag: "flag@example.com", env: "env@example.com", wantActor: "flag@example.com"},
		{name: "environment", env: "env@example.com", wantActor: "env@example.com"},
		{name: "git-config", wantActor: "test@test.com"},
		{name: "unknown", unsetGit: true, wantActor: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(store.ActorEnvVar, tt.env)
			fx := newUnapplyFixture(t, store.StateApplied)
			if tt.unsetGit {
				runUnapplyGit(t, fx.dir, "config", "user.email", "")
			}
			args := []string{"feature", "unapply", fx.slug, "--path", fx.dir}
			if tt.flag != "" {
				args = append(args, "--actor", tt.flag)
			}
			if _, stderr, code := runRJ(args...); code != 0 {
				t.Fatalf("code=%d stderr=%s", code, stderr)
			}
			attempts, _ := filepath.Glob(filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "unapply", "ua_*"))
			var session unapplySession
			if err := json.Unmarshal(mustRead(t, filepath.Join(attempts[0], "unapply-session.json")), &session); err != nil {
				t.Fatal(err)
			}
			if session.Actor != tt.wantActor {
				t.Fatalf("actor = %q, want %q", session.Actor, tt.wantActor)
			}
		})
	}
}

func TestUnappliedParentBlocksHardDependencyAndLabelsWaiting(t *testing.T) {
	_, s := newRejectRepo(t, map[string]store.FeatureState{
		"parent": store.StateUnapplied,
		"child":  store.StateApplied,
	})
	child, err := s.LoadFeatureStatus("child")
	if err != nil {
		t.Fatal(err)
	}
	child.DependsOn = []store.Dependency{{Slug: "parent", Kind: store.DependencyKindHard}}
	if err := s.SaveFeatureStatus(child); err != nil {
		t.Fatal(err)
	}
	if err := workflow.CheckDependencyGate(s, "child"); !errors.Is(err, workflow.ErrParentNotApplied) || !strings.Contains(err.Error(), "state=unapplied") {
		t.Fatalf("dependency gate error = %v", err)
	}
	labels, err := workflow.ComposeLabels(s, "child")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, label := range labels {
		if label == store.LabelWaitingOnParent {
			found = true
		}
	}
	if !found {
		t.Fatalf("labels = %v, want waiting-on-parent", labels)
	}
	if _, stderr, code := runRJ("apply", "child", "--path", s.Root); code == 0 || !strings.Contains(stderr, "state=unapplied") {
		t.Fatalf("apply child code=%d stderr=%s", code, stderr)
	}
	statusOut, stderr, code := runRJ("status", "--path", s.Root)
	if code != 0 || !strings.Contains(statusOut, `hard parent "parent" is unapplied (waiting-on-parent)`) {
		t.Fatalf("status code=%d stderr=%s:\n%s", code, stderr, statusOut)
	}
	dagOut, stderr, code := runRJ("status", "--dag", "--path", s.Root)
	if code != 0 || !strings.Contains(stderr, `hard parent "parent" is unapplied (waiting-on-parent)`) {
		t.Fatalf("status --dag code=%d stderr=%s stdout=%s", code, stderr, dagOut)
	}
}

func TestActiveParentStillSatisfiesHardDependency(t *testing.T) {
	_, s := newRejectRepo(t, map[string]store.FeatureState{
		"parent": store.StateActive,
		"child":  store.StateApplied,
	})
	child, err := s.LoadFeatureStatus("child")
	if err != nil {
		t.Fatal(err)
	}
	child.DependsOn = []store.Dependency{{Slug: "parent", Kind: store.DependencyKindHard}}
	if err := s.SaveFeatureStatus(child); err != nil {
		t.Fatal(err)
	}
	if err := workflow.CheckDependencyGate(s, "child"); err != nil {
		t.Fatalf("active parent must satisfy hard dependency: %v", err)
	}
}

func TestExplicitReconcileOnUnappliedReportsViabilityWithoutStateChange(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateApplied)
	if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 0 {
		t.Fatalf("unapply code=%d stderr=%s", code, stderr)
	}
	base := strings.TrimSpace(runUnapplyGit(t, fx.dir, "rev-list", "--max-parents=0", "HEAD"))
	results, err := workflow.RunReconcile(
		context.Background(),
		fx.store,
		[]string{fx.slug},
		base,
		nil,
		provider.Config{},
		workflow.ReconcileOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Phase != "unapplied-forward-apply-viability" ||
		results[0].Outcome != store.ReconcileStillNeeded {
		t.Fatalf("reconcile results = %+v", results)
	}
	status, err := fx.store.LoadFeatureStatus(fx.slug)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != store.StateUnapplied {
		t.Fatalf("explicit reconcile changed state to %q", status.State)
	}
	if _, err := os.Stat(filepath.Join(fx.dir, "feature.txt")); !os.IsNotExist(err) {
		t.Fatalf("explicit reconcile materialized feature.txt, err=%v", err)
	}
}

func TestExplicitReconcileCLIReachesUnappliedViabilityPath(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateApplied)
	if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 0 {
		t.Fatalf("unapply code=%d stderr=%s", code, stderr)
	}
	base := strings.TrimSpace(runUnapplyGit(t, fx.dir, "rev-list", "--max-parents=0", "HEAD"))
	out, stderr, code := runRJ(
		"reconcile", fx.slug,
		"--path", fx.dir,
		"--upstream-ref", base,
		"--allow-dirty",
		"--allow-stale-lock",
	)
	if code != 0 || !strings.Contains(out, "unapplied-forward-apply-viability") {
		t.Fatalf("code=%d stderr=%s stdout=%s", code, stderr, out)
	}
	status, err := fx.store.LoadFeatureStatus(fx.slug)
	if err != nil || status.State != store.StateUnapplied {
		t.Fatalf("status = %q, %v", status.State, err)
	}
}

func TestAggregateReconcileSkipsUnappliedFeature(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateApplied)
	if _, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir); code != 0 {
		t.Fatalf("unapply code=%d stderr=%s", code, stderr)
	}
	other, err := fx.store.AddFeature(store.AddFeatureInput{Title: "Other", Slug: "other", Request: "other"})
	if err != nil {
		t.Fatal(err)
	}
	other.State = store.StateApplied
	if err := fx.store.SaveFeatureStatus(other); err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(runUnapplyGit(t, fx.dir, "rev-list", "--max-parents=0", "HEAD"))
	results, err := workflow.RunReconcile(
		context.Background(),
		fx.store,
		nil,
		base,
		nil,
		provider.Config{},
		workflow.ReconcileOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Slug != "other" {
		t.Fatalf("aggregate results = %+v; unapplied feature must be skipped", results)
	}
	status, loadErr := fx.store.LoadFeatureStatus(fx.slug)
	if loadErr != nil || status.State != store.StateUnapplied {
		t.Fatalf("status after aggregate reconcile = %q, %v", status.State, loadErr)
	}
}

func assertJSONFieldOrder(t *testing.T, body string, fields []string) {
	t.Helper()
	last := -1
	for _, field := range fields {
		at := strings.Index(body, `"`+field+`"`)
		if at < 0 {
			t.Fatalf("missing JSON field %q:\n%s", field, body)
		}
		if at <= last {
			t.Fatalf("JSON field %q is out of order:\n%s", field, body)
		}
		last = at
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func directoryEntryNames(t *testing.T, path string) []string {
	t.Helper()
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	return names
}

func runUnapplyGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func newRenameUnapplyFixture(t *testing.T) unapplyFixture {
	t.Helper()
	testutil.PinGitAutoGCOff()
	dir := t.TempDir()
	gitInitTestRepo(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	feature, err := s.AddFeature(store.AddFeatureInput{Title: "Rename", Slug: "rename", Request: "rename"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "old.txt"), []byte("rename me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runUnapplyGit(t, dir, "add", "-A")
	runUnapplyGit(t, dir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "rename baseline")
	if err := os.Rename(filepath.Join(dir, "old.txt"), filepath.Join(dir, "new.txt")); err != nil {
		t.Fatal(err)
	}
	runUnapplyGit(t, dir, "add", "-A")
	runUnapplyGit(t, dir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "rename feature")
	patch := runUnapplyGit(t, dir, "show", "--find-renames=50%", "--format=", "--binary", "HEAD")
	if !strings.Contains(patch, "rename from old.txt") {
		t.Fatalf("fixture did not produce rename patch:\n%s", patch)
	}
	if err := s.WriteArtifact(feature.Slug, "post-apply.patch", patch); err != nil {
		t.Fatal(err)
	}
	feature.State = store.StateApplied
	feature.Apply.HasPatch = true
	if err := s.SaveFeatureStatus(feature); err != nil {
		t.Fatal(err)
	}
	runUnapplyGit(t, dir, "add", "-A")
	runUnapplyGit(t, dir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "record rename metadata")
	return unapplyFixture{dir: dir, store: s, slug: feature.Slug, patch: []byte(patch)}
}

func newUnicodeDeleteUnapplyFixture(t *testing.T) unapplyFixture {
	return newDeletedPathUnapplyFixture(t, "café.txt", "unicode", "unicode\n")
}

func newDeletedPathUnapplyFixture(t *testing.T, name, slug, content string) unapplyFixture {
	t.Helper()
	testutil.PinGitAutoGCOff()
	dir := t.TempDir()
	gitInitTestRepo(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	feature, err := s.AddFeature(store.AddFeatureInput{Title: slug, Slug: slug, Request: slug})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runUnapplyGit(t, dir, "add", "-A")
	runUnapplyGit(t, dir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "deleted-path baseline")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	runUnapplyGit(t, dir, "add", "-A")
	runUnapplyGit(t, dir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "deleted-path feature")
	patch := runUnapplyGit(t, dir, "show", "--format=", "--binary", "HEAD")
	if got := gitutil.PathsAffectedByPatch(patch); len(got) != 1 || got[0] != name {
		t.Fatalf("fixture patch paths = %v\n%s", got, patch)
	}
	if err := s.WriteArtifact(feature.Slug, "post-apply.patch", patch); err != nil {
		t.Fatal(err)
	}
	feature.State = store.StateApplied
	feature.Apply.HasPatch = true
	if err := s.SaveFeatureStatus(feature); err != nil {
		t.Fatal(err)
	}
	runUnapplyGit(t, dir, "add", "-A")
	runUnapplyGit(t, dir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "record deleted-path metadata")
	return unapplyFixture{dir: dir, store: s, slug: feature.Slug, patch: []byte(patch)}
}

func newSpaceUnapplyFixture(t *testing.T) unapplyFixture {
	t.Helper()
	testutil.PinGitAutoGCOff()
	dir := t.TempDir()
	gitInitTestRepo(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	feature, err := s.AddFeature(store.AddFeatureInput{Title: "Space", Slug: "space", Request: "space"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "my file.txt")
	if err := os.WriteFile(path, []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runUnapplyGit(t, dir, "add", "-A")
	runUnapplyGit(t, dir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "space baseline")
	if err := os.WriteFile(path, []byte("base\nfeature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch, err := gitutil.CapturePatch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(patch, "diff --git a/my file.txt b/my file.txt") {
		t.Fatalf("fixture did not produce unquoted spaced header:\n%s", patch)
	}
	if err := s.WriteArtifact(feature.Slug, "post-apply.patch", patch); err != nil {
		t.Fatal(err)
	}
	feature.State = store.StateApplied
	feature.Apply.HasPatch = true
	if err := s.SaveFeatureStatus(feature); err != nil {
		t.Fatal(err)
	}
	runUnapplyGit(t, dir, "add", "-A")
	runUnapplyGit(t, dir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "record spaced feature")
	return unapplyFixture{dir: dir, store: s, slug: feature.Slug, patch: []byte(patch)}
}
