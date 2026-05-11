package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// runCmdWithError mirrors runCmd but folds the returned error
// message into stderr so callers can assert on it. The shared
// runCmd helper discards the error string (cobra's SilenceErrors
// mode swallows the message), which would mask the
// `feat-amend-dependent-warning` user-facing text.
func runCmdWithError(args ...string) (stdout, stderr string, code int) {
	var outBuf, errBuf bytes.Buffer
	root := buildRootCmd()
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err := root.Execute()
	if err != nil {
		errBuf.WriteString(err.Error())
		return outBuf.String(), errBuf.String(), 1
	}
	return outBuf.String(), errBuf.String(), 0
}

// gitRun runs an arbitrary `git ...` command in `dir` and fatals the
// test on non-zero exit, surfacing combined stdout+stderr. Test-only
// convenience for the v0.7.0 dependent-broken fixtures which build
// non-trivial commit topologies.
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
	return strings.TrimSpace(string(out))
}

// seedDependentChain creates two features, A and B, where B carries a
// hard `satisfied_by` edge to A's commit SHA. Returns the commit SHA
// that becomes A's recorded base (== B.dependencies[0].satisfied_by).
//
// Used by the dependent-broken status / record-amend tests so each
// case starts from the same legal pre-amend topology.
func seedDependentChain(t *testing.T, tmp string, s *store.Store) string {
	t.Helper()
	// Feature A: write a file, commit, record. The commit becomes
	// A.apply.base_commit and A's "upstream merge" sha for B.
	if err := os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("A v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tmp, "add", "a.txt")
	gitRun(t, tmp, "commit", "-m", "A v1")
	aSHA := gitOut(t, tmp, "rev-parse", "HEAD")

	if _, _, code := runCmd("add", "--path", tmp, "--slug", "a", "Feature A"); code != 0 {
		t.Fatalf("add a failed")
	}
	aStatus, _ := s.LoadFeatureStatus("a")
	aStatus.State = store.StateUpstreamMerged
	aStatus.Apply.BaseCommit = aSHA
	if err := s.SaveFeatureStatus(aStatus); err != nil {
		t.Fatal(err)
	}

	if _, _, code := runCmd("add", "--path", tmp, "--slug", "b", "Feature B"); code != 0 {
		t.Fatalf("add b failed")
	}
	bStatus, _ := s.LoadFeatureStatus("b")
	bStatus.DependsOn = []store.Dependency{{
		Slug: "a", Kind: store.DependencyKindHard, SatisfiedBy: aSHA,
	}}
	bStatus.Apply.BaseCommit = aSHA
	if err := s.SaveFeatureStatus(bStatus); err != nil {
		t.Fatal(err)
	}
	return aSHA
}

// TestRecord_RefusesAmendThatOrphansDependent — chain A→B (B's
// satisfied_by points at A's commit), `git commit --amend` rewrites
// A's commit, then `tpatch record b` is invoked. The amend gate must
// abort with a non-zero exit code and a stderr message that names the
// orphaned downstream feature.
func TestRecord_RefusesAmendThatOrphansDependent(t *testing.T) {
	tmp, s := newDAGTestRepo(t)
	_ = seedDependentChain(t, tmp, s)

	// Amend the just-recorded commit. This rewrites the SHA that B's
	// satisfied_by points at — classic orphaning shape.
	if err := os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("A v1 (amended)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tmp, "add", "a.txt")
	gitRun(t, tmp, "commit", "--amend", "-m", "A v1 amended")

	// Stage a small change for `tpatch record b` to actually capture.
	if err := os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("B v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runCmdWithError("record", "--path", tmp, "b")
	if code == 0 {
		t.Fatalf("expected non-zero exit, got 0\nstdout=%s\nstderr=%s", stdout, stderr)
	}
	combined := stdout + stderr
	if !strings.Contains(combined, "record refuses") {
		t.Fatalf("expected 'record refuses' in output, got: %s", combined)
	}
	if !strings.Contains(combined, "b") || !strings.Contains(combined, "orphan") {
		t.Fatalf("expected orphan diagnostic naming feature 'b', got: %s", combined)
	}
}

// TestRecord_ForceAmendBypassesGate — same setup with --force-amend.
// The record must succeed (exit 0) while still warning on stderr.
func TestRecord_ForceAmendBypassesGate(t *testing.T) {
	tmp, s := newDAGTestRepo(t)
	_ = seedDependentChain(t, tmp, s)

	if err := os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("A v1 (amended)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tmp, "add", "a.txt")
	gitRun(t, tmp, "commit", "--amend", "-m", "A v1 amended")

	if err := os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("B v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runCmd("record", "--path", tmp, "--force-amend", "b")
	if code != 0 {
		t.Fatalf("expected exit 0 with --force-amend, got %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "force-amend") {
		t.Fatalf("expected --force-amend warning on stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "b") {
		t.Fatalf("expected orphan slug 'b' in --force-amend warning, got: %s", stderr)
	}
}

// TestStatus_DependentBrokenLabel — chain A→B, then forcibly rewrite
// history so A's recorded SHA is no longer reachable from HEAD. The
// derived `dependent-broken` label must appear on B's status line and
// the recovery hint must be emitted exactly once.
func TestStatus_DependentBrokenLabel(t *testing.T) {
	tmp, s := newDAGTestRepo(t)
	parentSHA := gitOut(t, tmp, "rev-parse", "HEAD") // pre-A commit
	_ = seedDependentChain(t, tmp, s)

	// Reset HEAD to before A, then commit a different A. The original
	// A SHA is now unreachable.
	gitRun(t, tmp, "reset", "--hard", parentSHA)
	if err := os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("different A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tmp, "add", "a.txt")
	gitRun(t, tmp, "commit", "-m", "different A")

	stdout, _, code := runCmd("status", "--path", tmp)
	if code != 0 {
		t.Fatalf("status exit %d, stdout=%s", code, stdout)
	}
	if !strings.Contains(stdout, "dependent-broken") {
		t.Fatalf("expected dependent-broken label/line in status, got: %s", stdout)
	}
	if !strings.Contains(stdout, "hint: re-record") {
		t.Fatalf("expected recovery hint line, got: %s", stdout)
	}
	// The hint must appear exactly once per invocation, not per
	// affected feature.
	if c := strings.Count(stdout, "hint: re-record"); c != 1 {
		t.Fatalf("expected hint exactly once, got %d in: %s", c, stdout)
	}
}

// TestStatus_DependentBrokenJSON — same setup as above, asserting the
// JSON shape for `dependent_broken` + `broken_refs`.
func TestStatus_DependentBrokenJSON(t *testing.T) {
	tmp, s := newDAGTestRepo(t)
	parentSHA := gitOut(t, tmp, "rev-parse", "HEAD")
	originalASHA := seedDependentChain(t, tmp, s)

	gitRun(t, tmp, "reset", "--hard", parentSHA)
	if err := os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("different A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tmp, "add", "a.txt")
	gitRun(t, tmp, "commit", "-m", "different A")

	stdout, _, code := runCmd("status", "--json", "--path", tmp)
	if code != 0 {
		t.Fatalf("status --json exit %d, stdout=%s", code, stdout)
	}

	var payload struct {
		Features []struct {
			Slug            string `json:"slug"`
			DependentBroken bool   `json:"dependent_broken"`
			BrokenRefs      []struct {
				Kind    string `json:"kind"`
				SHA     string `json:"sha"`
				Feature string `json:"feature"`
			} `json:"broken_refs"`
			RenderedLabels []string `json:"labels_rendered"`
		} `json:"features"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}

	var b *struct {
		Slug            string `json:"slug"`
		DependentBroken bool   `json:"dependent_broken"`
		BrokenRefs      []struct {
			Kind    string `json:"kind"`
			SHA     string `json:"sha"`
			Feature string `json:"feature"`
		} `json:"broken_refs"`
		RenderedLabels []string `json:"labels_rendered"`
	}
	for i, f := range payload.Features {
		if f.Slug == "b" {
			b = &payload.Features[i]
			break
		}
	}
	if b == nil {
		t.Fatalf("feature 'b' missing from payload: %s", stdout)
	}
	if !b.DependentBroken {
		t.Fatalf("expected dependent_broken=true on b, got %+v", b)
	}
	foundSatisfiedBy := false
	for _, r := range b.BrokenRefs {
		if r.Kind == "satisfied_by" && r.SHA == originalASHA && r.Feature == "b" {
			foundSatisfiedBy = true
		}
	}
	if !foundSatisfiedBy {
		t.Fatalf("expected satisfied_by broken_ref with SHA=%s feature=b, got %+v", originalASHA, b.BrokenRefs)
	}
	hasLabel := false
	for _, l := range b.RenderedLabels {
		if l == "dependent-broken" {
			hasLabel = true
		}
	}
	if !hasLabel {
		t.Fatalf("expected 'dependent-broken' in labels_rendered, got %v", b.RenderedLabels)
	}
}

// TestDependentDetection_BaseCommitBroken — a single feature whose
// own `apply.base_commit` becomes unreachable. The dependent-broken
// label must fire on the feature itself with kind=base_commit.
func TestDependentDetection_BaseCommitBroken(t *testing.T) {
	tmp, s := newDAGTestRepo(t)
	parentSHA := gitOut(t, tmp, "rev-parse", "HEAD")

	if err := os.WriteFile(filepath.Join(tmp, "x.txt"), []byte("X v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tmp, "add", "x.txt")
	gitRun(t, tmp, "commit", "-m", "X v1")
	xSHA := gitOut(t, tmp, "rev-parse", "HEAD")

	if _, _, code := runCmd("add", "--path", tmp, "--slug", "x", "Feature X"); code != 0 {
		t.Fatalf("add x failed")
	}
	st, _ := s.LoadFeatureStatus("x")
	st.Apply.BaseCommit = xSHA
	if err := s.SaveFeatureStatus(st); err != nil {
		t.Fatal(err)
	}

	// Rewrite history so xSHA is no longer reachable.
	gitRun(t, tmp, "reset", "--hard", parentSHA)
	if err := os.WriteFile(filepath.Join(tmp, "x.txt"), []byte("different X\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tmp, "add", "x.txt")
	gitRun(t, tmp, "commit", "-m", "different X")

	broken, err := store.CollectBrokenRefs(s)
	if err != nil {
		t.Fatalf("CollectBrokenRefs: %v", err)
	}
	refs, ok := broken["x"]
	if !ok {
		t.Fatalf("expected x in broken map, got %+v", broken)
	}
	foundBaseCommit := false
	for _, r := range refs {
		if r.Kind == store.FeatureRefKindBaseCommit && r.SHA == xSHA && r.Feature == "x" {
			foundBaseCommit = true
		}
	}
	if !foundBaseCommit {
		t.Fatalf("expected base_commit broken ref, got %+v", refs)
	}

	// Round-trip through the status text path too — confirms the
	// label composes onto the rendered line.
	stdout, _, code := runCmd("status", "--path", tmp)
	if code != 0 {
		t.Fatalf("status exit %d", code)
	}
	if !strings.Contains(stdout, "dependent-broken") {
		t.Fatalf("expected dependent-broken in status, got: %s", stdout)
	}
}
