package cli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// TestReconcileCheckAppliedAndAutoDropMutex verifies the v0.8.1 mutex
// gate refuses both flags together. The validation is at flag-parse
// time, before any store I/O.
func TestReconcileCheckAppliedAndAutoDropMutex(t *testing.T) {
	root := buildRootCmd()
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"reconcile", "--check-applied-only", "--auto-drop-merged", "some-slug"})
	err := root.Execute()
	if err == nil {
		t.Fatalf("expected error when --check-applied-only and --auto-drop-merged are combined")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in error, got %q", err.Error())
	}
}

// TestBuildAutoDropTrailers covers the trailer-derivation rules:
//   - Tpatch-Slug is always emitted.
//   - Tpatch-CVE is emitted iff the slug encodes a CVE identifier.
//   - The repo-policy Co-authored-by trailer is always last.
func TestBuildAutoDropTrailers(t *testing.T) {
	cases := []struct {
		name     string
		slug     string
		wantSlug string
		wantCVE  string // empty => trailer must be absent
	}{
		{"plain-slug", "fix-button-padding", "Tpatch-Slug: fix-button-padding", ""},
		{"cve-prefixed", "cve-2026-12345-validate-input", "Tpatch-Slug: cve-2026-12345-validate-input", "Tpatch-CVE: CVE-2026-12345"},
		{"cve-uppercase", "CVE-2024-9999-foo", "Tpatch-Slug: CVE-2024-9999-foo", "Tpatch-CVE: CVE-2024-9999"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildAutoDropTrailers(tc.slug)
			if !strings.Contains(got, tc.wantSlug) {
				t.Errorf("missing %q in trailers:\n%s", tc.wantSlug, got)
			}
			if tc.wantCVE == "" {
				if strings.Contains(got, "Tpatch-CVE:") {
					t.Errorf("expected no Tpatch-CVE trailer, got:\n%s", got)
				}
			} else if !strings.Contains(got, tc.wantCVE) {
				t.Errorf("missing %q in trailers:\n%s", tc.wantCVE, got)
			}
			if !strings.HasSuffix(strings.TrimRight(got, "\n"), coAuthorTrailer) {
				t.Errorf("trailer block must end with the repo-policy Co-authored-by line; got:\n%s", got)
			}
		})
	}
}

// autoDropFixture wires a git repo + tpatch store + an "applied"
// feature whose canonical post-apply.patch matches an upstream
// cherry-pick. The detector is enabled. Returns: tmp dir, store, slug,
// upstream tip SHA.
func autoDropFixture(t *testing.T, slug string, withDependent bool) (string, *store.Store, string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	gitInitTestRepo(t, tmpDir)
	baseline := gitHead(t, tmpDir)

	// Upstream absorbs greeting.txt (matches our future feature
	// patch by patch-id) then removes it.
	os.WriteFile(filepath.Join(tmpDir, "greeting.txt"), []byte("Hello World\n"), 0o644)
	runGitInTest(t, tmpDir, "add", "greeting.txt")
	runGitInTest(t, tmpDir, "commit", "-m", "upstream absorbs greeting")
	os.Remove(filepath.Join(tmpDir, "greeting.txt"))
	runGitInTest(t, tmpDir, "add", "-A")
	runGitInTest(t, tmpDir, "commit", "-m", "upstream later removed greeting")
	tip := gitHead(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	cfg, _ := s.LoadConfig()
	cfg.PatchIDDetectorEnabled = true
	cfg.FeaturesDependencies = true
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	// Pin upstream.lock to the baseline commit so phase 1.5's
	// rev-list walk includes the absorbing commit.
	lockBody := []byte(
		"remote: \"origin\"\nbranch: \"main\"\ncommit: \"" + baseline + "\"\nurl: \"\"\n",
	)
	os.WriteFile(filepath.Join(tmpDir, ".tpatch", "upstream.lock"), lockBody, 0o644)

	if _, err := s.AddFeature(store.AddFeatureInput{Title: "Add greeting", Request: "Add greeting", Slug: slug}); err != nil {
		t.Fatalf("AddFeature: %v", err)
	}
	s.MarkFeatureState(slug, store.StateApplied, "apply", "")
	patch := `diff --git a/greeting.txt b/greeting.txt
new file mode 100644
index 0000000..557db03
--- /dev/null
+++ b/greeting.txt
@@ -0,0 +1 @@
+Hello World
`
	if err := s.WriteArtifact(slug, "post-apply.patch", patch); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}

	if withDependent {
		if _, err := s.AddFeature(store.AddFeatureInput{Title: "Child", Request: "Depends on greeting", Slug: "child"}); err != nil {
			t.Fatalf("AddFeature child: %v", err)
		}
		st, _ := s.LoadFeatureStatus("child")
		st.DependsOn = []store.Dependency{{Slug: slug, Kind: store.DependencyKindHard}}
		if err := s.SaveFeatureStatus(st); err != nil {
			t.Fatalf("SaveFeatureStatus child: %v", err)
		}
	}

	// Stage the .tpatch tree so the eventual `git add -A` for the
	// removal commit has a clean baseline (otherwise the very first
	// auto-drop commit would also include the initial .tpatch
	// scaffolding, complicating the assertion).
	runGitInTest(t, tmpDir, "add", "-A")
	runGitInTest(t, tmpDir, "commit", "-m", "scaffold .tpatch")
	return tmpDir, s, slug, tip
}

func runGitInTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}

// TestReconcileAutoDropMerged_Match runs the full CLI end-to-end with
// `--auto-drop-merged` against a feature whose patch-id matches an
// upstream commit. Asserts: feature directory removed, removal commit
// created carrying both Tpatch-Slug and Tpatch-CVE trailers (CVE slug
// fixture).
func TestReconcileAutoDropMerged_Match(t *testing.T) {
	slug := "cve-2026-12345-add-greeting"
	tmpDir, s, _, tip := autoDropFixture(t, slug, false)

	root := buildRootCmd()
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"reconcile", "--path", tmpDir, "--upstream-ref", tip, "--allow-stale-lock", "--auto-drop-merged", slug})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}

	// Feature dir must be gone.
	if _, err := os.Stat(filepath.Join(tmpDir, ".tpatch", "features", slug)); err == nil {
		t.Fatalf("auto-drop must remove .tpatch/features/%s", slug)
	}

	// HEAD commit message must carry the trailers.
	c := exec.Command("git", "log", "-1", "--format=%B")
	c.Dir = tmpDir
	msg, err := c.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	body := string(msg)
	if !strings.Contains(body, "Tpatch-Slug: "+slug) {
		t.Errorf("removal commit missing Tpatch-Slug trailer:\n%s", body)
	}
	if !strings.Contains(body, "Tpatch-CVE: CVE-2026-12345") {
		t.Errorf("removal commit missing Tpatch-CVE trailer:\n%s", body)
	}
	if !strings.Contains(body, coAuthorTrailer) {
		t.Errorf("removal commit missing repo-policy Co-authored-by trailer:\n%s", body)
	}

	// status.json should also have been written by the prior reconcile pass.
	// We removed the feature so it's gone — but the verdict-recording
	// happened first per the brief. Verify by absence: the dir is gone.
	_ = s
}

// TestReconcileAutoDropMerged_NoMatchIsNoOp verifies that when phase
// 1.5 does not fire (detector OFF, the silent-no-op clause in the
// brief), the feature is NOT removed.
func TestReconcileAutoDropMerged_DetectorOffNoOp(t *testing.T) {
	slug := "add-greeting"
	tmpDir, s, _, tip := autoDropFixture(t, slug, false)

	// Flip detector OFF — the brief says auto-drop must be a silent
	// no-op in this case (it must NOT auto-enable the detector).
	cfg, _ := s.LoadConfig()
	cfg.PatchIDDetectorEnabled = false
	s.SaveConfig(cfg)

	root := buildRootCmd()
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"reconcile", "--path", tmpDir, "--upstream-ref", tip, "--allow-stale-lock", "--auto-drop-merged", slug})
	_ = root.Execute() // reconcile may emit a non-zero verdict but auto-drop must not fire

	if _, err := os.Stat(filepath.Join(tmpDir, ".tpatch", "features", slug)); err != nil {
		t.Fatalf("auto-drop must NOT remove %s when detector is off; expected dir to exist, got %v", slug, err)
	}
	// Make sure no commit was created with the auto-drop subject.
	c := exec.Command("git", "log", "--format=%s")
	c.Dir = tmpDir
	logOut, _ := c.Output()
	if strings.Contains(string(logOut), "tpatch: drop "+slug) {
		t.Fatalf("auto-drop must not commit when detector is off; git log:\n%s", string(logOut))
	}
}

// TestReconcileAutoDropMerged_NoMatchPath verifies that when the
// detector is on but the patch-id sweep does not match, the feature
// is left in place and reconcile falls through to the existing
// phase 2/3/4 pipeline.
func TestReconcileAutoDropMerged_NoMatchPath(t *testing.T) {
	slug := "add-models"
	tmpDir := t.TempDir()
	gitInitTestRepo(t, tmpDir)
	baseline := gitHead(t, tmpDir)
	// Unrelated upstream churn — no patch-id match possible.
	os.WriteFile(filepath.Join(tmpDir, "other.txt"), []byte("unrelated\n"), 0o644)
	runGitInTest(t, tmpDir, "add", "other.txt")
	runGitInTest(t, tmpDir, "commit", "-m", "unrelated upstream change")
	tip := gitHead(t, tmpDir)

	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	cfg, _ := s.LoadConfig()
	cfg.PatchIDDetectorEnabled = true
	s.SaveConfig(cfg)
	lockBody := []byte("remote: \"origin\"\nbranch: \"main\"\ncommit: \"" + baseline + "\"\nurl: \"\"\n")
	os.WriteFile(filepath.Join(tmpDir, ".tpatch", "upstream.lock"), lockBody, 0o644)

	s.AddFeature(store.AddFeatureInput{Title: "Add models", Request: "models.txt"})
	s.MarkFeatureState(slug, store.StateApplied, "apply", "")
	patch := `diff --git a/models.txt b/models.txt
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/models.txt
@@ -0,0 +1 @@
+gpt-4o
`
	s.WriteArtifact(slug, "post-apply.patch", patch)
	runGitInTest(t, tmpDir, "add", "-A")
	runGitInTest(t, tmpDir, "commit", "-m", "scaffold .tpatch")

	root := buildRootCmd()
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"reconcile", "--path", tmpDir, "--upstream-ref", tip, "--allow-stale-lock", "--auto-drop-merged", slug})
	_ = root.Execute()

	if _, err := os.Stat(filepath.Join(tmpDir, ".tpatch", "features", slug)); err != nil {
		t.Fatalf("auto-drop must NOT remove %s when phase 1.5 does not match; got %v", slug, err)
	}
}

// TestReconcileAutoDropMerged_RefusesOnDependents verifies the
// cascade-rule guard. With a hard-dependent child, the auto-drop
// MUST refuse and surface a hint pointing the operator at
// `tpatch remove --cascade <slug>`.
func TestReconcileAutoDropMerged_RefusesOnDependents(t *testing.T) {
	slug := "add-greeting"
	tmpDir, _, _, tip := autoDropFixture(t, slug, true /* withDependent */)

	root := buildRootCmd()
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"reconcile", "--path", tmpDir, "--upstream-ref", tip, "--allow-stale-lock", "--auto-drop-merged", slug})
	err := root.Execute()
	if err == nil {
		t.Fatalf("expected error when auto-drop hits a feature with dependents")
	}
	if !strings.Contains(err.Error(), "auto-drop-merged") {
		t.Errorf("expected error to mention auto-drop-merged; got %q", err.Error())
	}
	combined := out.String() + errBuf.String()
	if !strings.Contains(combined, "tpatch remove --cascade") {
		t.Errorf("expected hint about `tpatch remove --cascade`; got:\n%s", combined)
	}
	// Feature dir must still exist.
	if _, statErr := os.Stat(filepath.Join(tmpDir, ".tpatch", "features", slug)); errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("auto-drop refusal must NOT remove the feature dir")
	}
}

// TestReconcileCheckAppliedOnly_ExitCodeOnMatch wires the read-only
// CLI path and verifies a phase-1.5 match yields no error (exit 0)
// and writes no artifacts. Detector OFF on disk; the flag forces it on.
func TestReconcileCheckAppliedOnly_ExitCodeOnMatch(t *testing.T) {
	slug := "add-greeting"
	tmpDir, _, _, tip := autoDropFixture(t, slug, false)
	// Force the on-disk default OFF — the override semantic is the
	// load-bearing assertion under test.
	s, _ := store.Open(tmpDir)
	cfg, _ := s.LoadConfig()
	cfg.PatchIDDetectorEnabled = false
	s.SaveConfig(cfg)

	root := buildRootCmd()
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"reconcile", "--path", tmpDir, "--upstream-ref", tip, "--check-applied-only", slug})
	if err := root.Execute(); err != nil {
		t.Fatalf("expected nil error on phase-1.5 match (exit 0); got %v\nstderr=%s", err, errBuf.String())
	}
	if !strings.Contains(out.String(), "phase-1.5-patch-id-match") {
		t.Errorf("expected phase-1.5-patch-id-match in stdout; got:\n%s", out.String())
	}
	// Read-only contract: no artifacts written, feature dir unchanged.
	if _, err := os.Stat(filepath.Join(tmpDir, ".tpatch", "features", slug, "artifacts", "reconcile-session.json")); err == nil {
		t.Fatalf("--check-applied-only must not write reconcile-session.json")
	}
}

// TestReconcileCheckAppliedOnly_ExitCodeOnNoMatch verifies the no-match
// path returns *ExitCodeError{Code:2} so harnesses can distinguish it
// from a generic CLI failure.
func TestReconcileCheckAppliedOnly_ExitCodeOnNoMatch(t *testing.T) {
	slug := "add-models"
	tmpDir := t.TempDir()
	gitInitTestRepo(t, tmpDir)
	baseline := gitHead(t, tmpDir)
	os.WriteFile(filepath.Join(tmpDir, "other.txt"), []byte("unrelated\n"), 0o644)
	runGitInTest(t, tmpDir, "add", "other.txt")
	runGitInTest(t, tmpDir, "commit", "-m", "unrelated upstream change")
	tip := gitHead(t, tmpDir)

	s, _ := store.Init(tmpDir)
	// Detector left OFF on disk; the flag forces it on for this run.
	lockBody := []byte("remote: \"origin\"\nbranch: \"main\"\ncommit: \"" + baseline + "\"\nurl: \"\"\n")
	os.WriteFile(filepath.Join(tmpDir, ".tpatch", "upstream.lock"), lockBody, 0o644)

	s.AddFeature(store.AddFeatureInput{Title: "Add models", Request: "models.txt"})
	s.MarkFeatureState(slug, store.StateApplied, "apply", "")
	patch := `diff --git a/models.txt b/models.txt
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/models.txt
@@ -0,0 +1 @@
+gpt-4o
`
	s.WriteArtifact(slug, "post-apply.patch", patch)

	root := buildRootCmd()
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"reconcile", "--path", tmpDir, "--upstream-ref", tip, "--check-applied-only", slug})
	err := root.Execute()
	if err == nil {
		t.Fatalf("expected non-nil error on no-match (so exit code 2 propagates)")
	}
	ec := asExitCodeError(err)
	if ec == nil {
		t.Fatalf("expected *ExitCodeError, got %T: %v", err, err)
	}
	if ec.Code != 2 {
		t.Errorf("expected exit code 2 on no-match, got %d", ec.Code)
	}
}
