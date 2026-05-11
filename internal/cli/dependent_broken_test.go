package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

// TestStatusDAG_DependentBrokenLabel — chain A→B with A's recorded
// SHA rewritten away, run `status --dag` (text) and assert the rendered
// tree shows `dependent-broken` in the labels parens for B.
// Regression for rev-1 Finding 1.
func TestStatusDAG_DependentBrokenLabel(t *testing.T) {
	tmp, s := newDAGTestRepo(t)
	parentSHA := gitOut(t, tmp, "rev-parse", "HEAD")
	_ = seedDependentChain(t, tmp, s)

	gitRun(t, tmp, "reset", "--hard", parentSHA)
	if err := os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("different A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tmp, "add", "a.txt")
	gitRun(t, tmp, "commit", "-m", "different A")

	stdout, _, code := runCmd("status", "--dag", "--path", tmp)
	if code != 0 {
		t.Fatalf("status --dag exit %d, stdout=%s", code, stdout)
	}
	// Expect a line for `b` containing the dependent-broken label
	// inside parens, e.g. `b [...] (dependent-broken, ...)`.
	hit := false
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, "b [") && strings.Contains(line, "dependent-broken") {
			hit = true
			break
		}
	}
	if !hit {
		t.Fatalf("expected `b ... dependent-broken` line in --dag output, got:\n%s", stdout)
	}
}

// TestStatusDAG_DependentBrokenJSON — same setup, asserts that the
// `--dag --json` payload carries `dependent_broken: true` and a
// `broken_refs[]` array shaped identically to the non-DAG --json
// payload (kind/sha/feature). Regression for rev-1 Finding 1.
func TestStatusDAG_DependentBrokenJSON(t *testing.T) {
	tmp, s := newDAGTestRepo(t)
	parentSHA := gitOut(t, tmp, "rev-parse", "HEAD")
	originalASHA := seedDependentChain(t, tmp, s)

	gitRun(t, tmp, "reset", "--hard", parentSHA)
	if err := os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("different A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tmp, "add", "a.txt")
	gitRun(t, tmp, "commit", "-m", "different A")

	stdout, _, code := runCmd("status", "--dag", "--json", "--path", tmp)
	if code != 0 {
		t.Fatalf("status --dag --json exit %d, stdout=%s", code, stdout)
	}

	var payload struct {
		Features []struct {
			Slug            string   `json:"slug"`
			Labels          []string `json:"labels"`
			DependentBroken bool     `json:"dependent_broken"`
			BrokenRefs      []struct {
				Kind    string `json:"kind"`
				SHA     string `json:"sha"`
				Feature string `json:"feature"`
			} `json:"broken_refs"`
		} `json:"features"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	var bIdx = -1
	for i, f := range payload.Features {
		if f.Slug == "b" {
			bIdx = i
			break
		}
	}
	if bIdx < 0 {
		t.Fatalf("feature 'b' missing from --dag --json payload: %s", stdout)
	}
	b := payload.Features[bIdx]
	if !b.DependentBroken {
		t.Fatalf("expected dependent_broken=true on b, got %+v", b)
	}
	foundSat := false
	for _, r := range b.BrokenRefs {
		if r.Kind == "satisfied_by" && r.SHA == originalASHA && r.Feature == "b" {
			foundSat = true
		}
	}
	if !foundSat {
		t.Fatalf("expected satisfied_by broken_ref kind=%q sha=%s feature=b, got %+v",
			"satisfied_by", originalASHA, b.BrokenRefs)
	}
	hasLabel := false
	for _, l := range b.Labels {
		if l == "dependent-broken" {
			hasLabel = true
		}
	}
	if !hasLabel {
		t.Fatalf("expected 'dependent-broken' in labels, got %v", b.Labels)
	}
}

// TestStatus_DependentBrokenSingleLinePerFeature — feature B references
// the same rewritten SHA via BOTH apply.base_commit AND satisfied_by;
// the plain-text status path must emit exactly ONE `dependent-broken:
// feature "b"` line (deduped), not one per ref. Regression for rev-1
// Finding 2.
func TestStatus_DependentBrokenSingleLinePerFeature(t *testing.T) {
	tmp, s := newDAGTestRepo(t)
	parentSHA := gitOut(t, tmp, "rev-parse", "HEAD")
	originalASHA := seedDependentChain(t, tmp, s)
	// seedDependentChain already sets B.apply.base_commit = aSHA AND
	// B.depends_on[0].satisfied_by = aSHA — exactly the dual-ref shape.
	_ = originalASHA

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
	// Count lines that begin with `dependent-broken: feature "b"`.
	count := 0
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, `dependent-broken: feature "b"`) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 dependent-broken line for feature 'b' (deduped across base_commit + satisfied_by), got %d in:\n%s", count, stdout)
	}
	// Sanity: the line must include the abbrev SHA (first 7 chars).
	abbrev := originalASHA[:7]
	if !strings.Contains(stdout, abbrev) {
		t.Fatalf("expected abbrev SHA %q in dependent-broken line, got:\n%s", abbrev, stdout)
	}
}

// TestStatus_DependentBrokenMultipleSHAsPerFeature — feature C
// references two different broken SHAs (apply.base_commit = SHA1,
// satisfied_by = SHA2, both unreachable). The status line must list
// BOTH abbrevs in a single line, sorted ascending. Regression for
// rev-1 Finding 2 (multi-SHA aggregation).
func TestStatus_DependentBrokenMultipleSHAsPerFeature(t *testing.T) {
	tmp, s := newDAGTestRepo(t)
	parentSHA := gitOut(t, tmp, "rev-parse", "HEAD")

	// Feature A → SHA-A (becomes A's recorded apply.base_commit and
	// the SHA that C's satisfied_by points at).
	if err := os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("A v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tmp, "add", "a.txt")
	gitRun(t, tmp, "commit", "-m", "A v1")
	aSHA := gitOut(t, tmp, "rev-parse", "HEAD")

	// SHA-X is a SECOND distinct commit that C will record as its own
	// apply.base_commit. After the reset both aSHA and xSHA become
	// unreachable.
	if err := os.WriteFile(filepath.Join(tmp, "x.txt"), []byte("X v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tmp, "add", "x.txt")
	gitRun(t, tmp, "commit", "-m", "X v1")
	xSHA := gitOut(t, tmp, "rev-parse", "HEAD")

	if _, _, code := runCmd("add", "--path", tmp, "--slug", "a", "Feature A"); code != 0 {
		t.Fatalf("add a failed")
	}
	aStatus, _ := s.LoadFeatureStatus("a")
	aStatus.State = store.StateUpstreamMerged
	aStatus.Apply.BaseCommit = aSHA
	if err := s.SaveFeatureStatus(aStatus); err != nil {
		t.Fatal(err)
	}

	if _, _, code := runCmd("add", "--path", tmp, "--slug", "c", "Feature C"); code != 0 {
		t.Fatalf("add c failed")
	}
	cStatus, _ := s.LoadFeatureStatus("c")
	cStatus.DependsOn = []store.Dependency{{
		Slug: "a", Kind: store.DependencyKindHard, SatisfiedBy: aSHA,
	}}
	cStatus.Apply.BaseCommit = xSHA
	if err := s.SaveFeatureStatus(cStatus); err != nil {
		t.Fatal(err)
	}

	// Rewrite history so both aSHA and xSHA become unreachable.
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
	// Find the `dependent-broken: feature "c"` line (must exist
	// exactly once). It must list both abbrevs, joined by ", ", in
	// ascending order.
	var cLine string
	count := 0
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, `dependent-broken: feature "c"`) {
			count++
			cLine = line
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 dependent-broken line for feature 'c', got %d in:\n%s", count, stdout)
	}
	a7, x7 := aSHA[:7], xSHA[:7]
	expected := []string{a7, x7}
	sort.Strings(expected)
	joined := strings.Join(expected, ", ")
	if !strings.Contains(cLine, joined) {
		t.Fatalf("expected sorted-joined abbrevs %q in line, got: %s", joined, cLine)
	}
}

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
