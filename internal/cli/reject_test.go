package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/testutil"
)

// ─── PRD-rejected-feature-state §9 — CLI half of the 27-item matrix ─────────
//
// The store-layer half (data model, reason enum, actor precedence,
// Rule 7) lives in internal/store/rejection_test.go.
//
// Exit-code envelope under test (ADR-031 D4 addendum):
//   0 success · 1 unexpected · 2 pre-mutation validation · 3 state refusal

// runRJ is an exit-code-aware runner. The shared runCmd helper collapses
// every error to exit 1, which would make every assertion in this file
// vacuous, so this variant unwraps *ExitCodeError.
func runRJ(args ...string) (stdout, stderr string, code int) {
	var outBuf, errBuf bytes.Buffer
	root := buildRootCmd()
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err := root.Execute()
	if err == nil {
		return outBuf.String(), errBuf.String(), 0
	}
	errBuf.WriteString(err.Error())
	if e := asExitCodeError(err); e != nil {
		return outBuf.String(), errBuf.String(), e.ExitCode()
	}
	return outBuf.String(), errBuf.String(), 1
}

// newRejectRepo builds an isolated worktree with one feature per
// requested slug/state pair, plus a deterministic evidence file inside
// each feature directory.
func newRejectRepo(t *testing.T, features map[string]store.FeatureState) (string, *store.Store) {
	t.Helper()
	testutil.PinGitAutoGCOff()
	dir := t.TempDir()
	gitInitTestRepo(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	for slug, state := range features {
		f, err := s.AddFeature(store.AddFeatureInput{Title: slug, Slug: slug, Request: slug})
		if err != nil {
			t.Fatalf("AddFeature %s: %v", slug, err)
		}
		if state != "" {
			f.State = state
			if err := s.SaveFeatureStatus(f); err != nil {
				t.Fatalf("SaveFeatureStatus %s: %v", slug, err)
			}
		}
		writeEvidence(t, dir, slug, "analysis.md", "evidence for "+slug+"\n")
	}
	return dir, s
}

// writeEvidence writes a file into .tpatch/features/<slug>/<rel>.
func writeEvidence(t *testing.T, dir, slug, rel, content string) string {
	t.Helper()
	p := filepath.Join(dir, ".tpatch", "features", slug, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func sha256Of(t *testing.T, content string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func mustLoad(t *testing.T, s *store.Store, slug string) store.FeatureStatus {
	t.Helper()
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatalf("LoadFeatureStatus %s: %v", slug, err)
	}
	return st
}

// ─── items 1-7: happy-path transitions from every eligible source ───────────

func TestReject_HappyPathFromEveryEligibleState(t *testing.T) {
	for _, src := range store.RejectableStateList() {
		t.Run(string(src), func(t *testing.T) {
			dir, s := newRejectRepo(t, map[string]store.FeatureState{"alpha": src})

			out, errOut, code := runRJ("reject", "alpha", "--path", dir,
				"--reason", "out-of-scope",
				"--note", "not in scope for this fork",
				"--evidence", "analysis.md",
				"--actor", "dev@example.com")
			if code != 0 {
				t.Fatalf("exit %d\nstdout=%s\nstderr=%s", code, out, errOut)
			}

			st := mustLoad(t, s, "alpha")
			if st.State != store.StateRejected {
				t.Fatalf("state = %q, want rejected", st.State)
			}
			r := st.Rejection
			if r == nil {
				t.Fatal("rejection sub-object not written")
			}
			if r.Reason != "out-of-scope" {
				t.Errorf("reason = %q", r.Reason)
			}
			if r.Note != "not in scope for this fork" {
				t.Errorf("note = %q", r.Note)
			}
			if r.Actor != "dev@example.com" {
				t.Errorf("actor = %q", r.Actor)
			}
			if r.PriorState != src {
				t.Errorf("prior_state = %q, want %q", r.PriorState, src)
			}
			if r.RejectedAt.IsZero() {
				t.Error("rejected_at not set")
			}
			if len(r.Evidence) != 1 || r.Evidence[0].Path != "analysis.md" {
				t.Fatalf("evidence = %+v", r.Evidence)
			}
			if want := sha256Of(t, "evidence for alpha\n"); r.Evidence[0].SHA256 != want {
				t.Errorf("sha256 = %q, want %q", r.Evidence[0].SHA256, want)
			}
			if len(r.History) != 1 {
				t.Fatalf("history len = %d, want 1", len(r.History))
			}
			h := r.History[0]
			if h.Action != store.RejectionActionReject || h.Actor != "dev@example.com" || h.Reason != "out-of-scope" {
				t.Errorf("history[0] = %+v", h)
			}
			// Operator-facing output must hand over the reopen command.
			if !strings.Contains(out, "tpatch reopen alpha") {
				t.Errorf("stdout missing reopen hint: %s", out)
			}
		})
	}
}

// The `--related` pointer is free-form and round-trips verbatim.
func TestReject_RelatedPointerRoundTrips(t *testing.T) {
	dir, s := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateAnalyzed})
	if _, _, code := runRJ("reject", "alpha", "--path", dir,
		"--reason", "duplicate", "--note", "dupe of beta",
		"--evidence", "analysis.md", "--related", "GH#41"); code != 0 {
		t.Fatalf("exit %d", code)
	}
	st := mustLoad(t, s, "alpha")
	if st.Rejection.Related != "GH#41" {
		t.Fatalf("related = %q", st.Rejection.Related)
	}
	if st.Rejection.History[0].Related != "GH#41" {
		t.Fatalf("history related = %q", st.Rejection.History[0].Related)
	}
}

// The JSON envelope carries every PRD §8 field, including exit_code.
func TestReject_JSONEnvelope(t *testing.T) {
	dir, _ := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateDefined})
	out, _, code := runRJ("reject", "alpha", "--path", dir,
		"--reason", "unsafe", "--note", "introduces a path traversal",
		"--evidence", "analysis.md", "--actor", "sec@example.com", "--json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("envelope is not JSON: %v\n%s", err, out)
	}
	for k, want := range map[string]any{
		"slug":        "alpha",
		"state":       "rejected",
		"prior_state": "defined",
		"reason":      "unsafe",
		"rejected_by": "sec@example.com",
		"exit_code":   float64(0),
	} {
		if env[k] != want {
			t.Errorf("envelope[%q] = %v, want %v", k, env[k], want)
		}
	}
	if env["related"] != nil {
		t.Errorf("unset --related must serialize as null, got %v", env["related"])
	}
	ev, ok := env["evidence"].([]any)
	if !ok || len(ev) != 1 {
		t.Fatalf("evidence = %v", env["evidence"])
	}
}

// ─── item 8: refusal from every post-implementation / ineligible state ──────

func TestReject_RefusedFromIneligibleStates(t *testing.T) {
	ineligible := []store.FeatureState{
		store.StateImplementing,
		store.StateApplied,
		store.StateActive,
		store.StateReconciling,
		store.StateReconcilingShadow,
		store.StateBlocked,
		store.StateUpstreamMerged,
	}
	for _, src := range ineligible {
		t.Run(string(src), func(t *testing.T) {
			dir, s := newRejectRepo(t, map[string]store.FeatureState{"alpha": src})
			before := mustLoad(t, s, "alpha")

			_, errOut, code := runRJ("reject", "alpha", "--path", dir,
				"--reason", "obsolete", "--note", "n", "--evidence", "analysis.md")
			if code != 3 {
				t.Fatalf("exit %d, want 3 (state-machine refusal); stderr=%s", code, errOut)
			}
			if !strings.Contains(errOut, string(src)) {
				t.Errorf("refusal should name the offending state %q: %s", src, errOut)
			}
			after := mustLoad(t, s, "alpha")
			if after.State != before.State {
				t.Fatalf("state mutated on refusal: %q → %q", before.State, after.State)
			}
			if after.Rejection != nil {
				t.Fatal("rejection record written despite refusal")
			}
		})
	}
}

// Rejecting an already-rejected feature is refused, and the existing
// record is left untouched (no silent re-write).
func TestReject_RefusedWhenAlreadyRejected(t *testing.T) {
	dir, s := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateRequested})
	if _, _, code := runRJ("reject", "alpha", "--path", dir,
		"--reason", "duplicate", "--note", "first", "--evidence", "analysis.md"); code != 0 {
		t.Fatalf("first reject failed: %d", code)
	}
	before := mustLoad(t, s, "alpha")

	_, errOut, code := runRJ("reject", "alpha", "--path", dir,
		"--reason", "obsolete", "--note", "second", "--evidence", "analysis.md")
	if code != 3 {
		t.Fatalf("exit %d, want 3; stderr=%s", code, errOut)
	}
	after := mustLoad(t, s, "alpha")
	if after.Rejection.Reason != before.Rejection.Reason || after.Rejection.Note != before.Rejection.Note {
		t.Fatalf("existing rejection record was overwritten: %+v", after.Rejection)
	}
	if len(after.Rejection.History) != 1 {
		t.Fatalf("history grew on a refused reject: %d", len(after.Rejection.History))
	}
	if !strings.Contains(errOut, "tpatch reopen alpha") {
		t.Errorf("refusal should point at reopen: %s", errOut)
	}
}

// ─── item 9 + 22: live dependents block rejection, for all three kinds ──────

func TestReject_RefusedWhenDependentsExist(t *testing.T) {
	for _, kind := range []string{
		store.DependencyKindHard, store.DependencyKindSoft, store.DependencyKindSupersedes,
	} {
		t.Run(kind, func(t *testing.T) {
			dir, s := newRejectRepo(t, map[string]store.FeatureState{
				"parent": store.StateRequested,
				"child":  store.StateRequested,
			})
			c := mustLoad(t, s, "child")
			c.DependsOn = []store.Dependency{{Slug: "parent", Kind: kind}}
			if err := s.SaveFeatureStatus(c); err != nil {
				t.Fatal(err)
			}

			_, errOut, code := runRJ("reject", "parent", "--path", dir,
				"--reason", "out-of-scope", "--note", "n", "--evidence", "analysis.md")
			if code != 3 {
				t.Fatalf("exit %d, want 3; stderr=%s", code, errOut)
			}
			for _, want := range []string{"child", kind, "tpatch feature deps"} {
				if !strings.Contains(errOut, want) {
					t.Errorf("refusal missing %q: %s", want, errOut)
				}
			}
			if mustLoad(t, s, "parent").State != store.StateRequested {
				t.Fatal("parent state mutated despite refusal")
			}
		})
	}
}

// ─── item 22: dependency-order symmetry, the OTHER order ────────────────────
//
// Reject the parent first, then attempt to add the edge: the CLI edge
// command must refuse too, so neither order can produce a rejected
// feature with live dependents.
func TestRejectDependencySymmetry_EdgeOntoRejectedParentRefused(t *testing.T) {
	for _, kind := range []string{
		store.DependencyKindHard, store.DependencyKindSoft, store.DependencyKindSupersedes,
	} {
		t.Run(kind, func(t *testing.T) {
			dir, s := newRejectRepo(t, map[string]store.FeatureState{
				"parent": store.StateRequested,
				"child":  store.StateRequested,
			})
			cfg, _ := s.LoadConfig()
			cfg.FeaturesDependencies = true
			if err := s.SaveConfig(cfg); err != nil {
				t.Fatal(err)
			}
			if _, _, code := runRJ("reject", "parent", "--path", dir,
				"--reason", "out-of-scope", "--note", "n", "--evidence", "analysis.md"); code != 0 {
				t.Fatalf("reject parent failed: %d", code)
			}

			_, errOut, code := runRJ("feature", "deps", "child", "add", "parent:"+kind, "--path", dir)
			if code == 0 {
				t.Fatalf("edge onto a rejected parent must be refused (kind=%s)", kind)
			}
			if !strings.Contains(errOut, "rejected") {
				t.Errorf("refusal should say the parent is rejected: %s", errOut)
			}
			if got := mustLoad(t, s, "child"); len(got.DependsOn) != 0 {
				t.Fatalf("edge persisted despite refusal: %+v", got.DependsOn)
			}
		})
	}
}

// ─── items 10-15: evidence validation, hashing, path safety ────────────────

func TestReject_ValidationErrorsExitTwo(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"missing-reason", []string{"--note", "n", "--evidence", "analysis.md"}, "reason required"},
		{"invalid-reason", []string{"--reason", "wont-fix", "--note", "n", "--evidence", "analysis.md"}, "invalid reason"},
		{"empty-note", []string{"--reason", "duplicate", "--note", "   ", "--evidence", "analysis.md"}, "note required"},
		{"missing-note", []string{"--reason", "duplicate", "--evidence", "analysis.md"}, "note required"},
		{"no-evidence", []string{"--reason", "duplicate", "--note", "n"}, "evidence required"},
		{"absolute-evidence", []string{"--reason", "duplicate", "--note", "n", "--evidence", "/etc/passwd"}, "absolute"},
		{"traversal-evidence", []string{"--reason", "duplicate", "--note", "n", "--evidence", "../../etc/passwd"}, "escapes"},
		{"missing-evidence-file", []string{"--reason", "duplicate", "--note", "n", "--evidence", "nope.md"}, "no such file"},
		{"empty-evidence-path", []string{"--reason", "duplicate", "--note", "n", "--evidence", "  "}, "must not be empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, s := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateRequested})
			args := append([]string{"reject", "alpha", "--path", dir}, tc.args...)
			_, errOut, code := runRJ(args...)
			if code != 2 {
				t.Fatalf("exit %d, want 2; stderr=%s", code, errOut)
			}
			if !strings.Contains(errOut, tc.want) {
				t.Errorf("message missing %q: %s", tc.want, errOut)
			}
			st := mustLoad(t, s, "alpha")
			if st.State != store.StateRequested || st.Rejection != nil {
				t.Fatal("validation failure must be pre-mutation")
			}
		})
	}
}

// A directory is not a regular file.
func TestReject_EvidenceNonRegularFileExitsTwo(t *testing.T) {
	dir, _ := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateRequested})
	if err := os.MkdirAll(filepath.Join(dir, ".tpatch", "features", "alpha", "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runRJ("reject", "alpha", "--path", dir,
		"--reason", "duplicate", "--note", "n", "--evidence", "artifacts")
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr=%s", code, errOut)
	}
	if !strings.Contains(errOut, "not a regular file") {
		t.Errorf("message = %s", errOut)
	}
}

// A symlink escaping the repository root is refused BEFORE its target's
// bytes are read (PRD §6, F-INT-3).
func TestReject_EvidenceSymlinkEscapeRefusedWithoutHashing(t *testing.T) {
	dir, _ := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateRequested})
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("top secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, ".tpatch", "features", "alpha", "escape.md")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	var hashed []string
	restore := evidenceHashFn
	evidenceHashFn = func(abs string) (string, error) {
		hashed = append(hashed, abs)
		return restore(abs)
	}
	t.Cleanup(func() { evidenceHashFn = restore })

	_, errOut, code := runRJ("reject", "alpha", "--path", dir,
		"--reason", "unsafe", "--note", "n", "--evidence", "escape.md")
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr=%s", code, errOut)
	}
	if !strings.Contains(errOut, "outside the repository root") {
		t.Errorf("message = %s", errOut)
	}
	for _, h := range hashed {
		if strings.Contains(h, "secret.txt") || strings.HasPrefix(h, outside) {
			t.Fatalf("escaping symlink target was hashed: %s", h)
		}
	}
}

// Evidence hashes are the SHA-256 of the file's raw bytes, and the list
// is deduplicated and sorted for deterministic serialization.
func TestReject_EvidenceHashDedupAndSort(t *testing.T) {
	dir, s := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateRequested})
	writeEvidence(t, dir, "alpha", "zeta.md", "zeta\n")
	writeEvidence(t, dir, "alpha", "artifacts/beta.md", "beta\n")

	if _, _, code := runRJ("reject", "alpha", "--path", dir,
		"--reason", "obsolete", "--note", "n",
		"--evidence", "zeta.md",
		"--evidence", "artifacts/beta.md",
		"--evidence", "./zeta.md",
		"--evidence", "analysis.md"); code != 0 {
		t.Fatalf("reject failed")
	}
	ev := mustLoad(t, s, "alpha").Rejection.Evidence
	got := make([]string, len(ev))
	for i, e := range ev {
		got[i] = e.Path
	}
	want := []string{"analysis.md", "artifacts/beta.md", "zeta.md"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("evidence paths = %v, want %v (deduped + sorted)", got, want)
	}
	byPath := map[string]string{}
	for _, e := range ev {
		byPath[e.Path] = e.SHA256
	}
	if byPath["zeta.md"] != sha256Of(t, "zeta\n") {
		t.Errorf("zeta hash = %q", byPath["zeta.md"])
	}
	if byPath["artifacts/beta.md"] != sha256Of(t, "beta\n") {
		t.Errorf("beta hash = %q", byPath["artifacts/beta.md"])
	}
}

// Evidence resolves against the feature directory FIRST, then the
// repository root (ADR-031 D3).
func TestReject_EvidenceResolutionOrder(t *testing.T) {
	dir, s := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateRequested})
	// Same relative name in both locations; the feature-dir copy wins.
	writeEvidence(t, dir, "alpha", "shared.md", "feature-dir copy\n")
	if err := os.WriteFile(filepath.Join(dir, "shared.md"), []byte("repo-root copy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, code := runRJ("reject", "alpha", "--path", dir,
		"--reason", "obsolete", "--note", "n", "--evidence", "shared.md"); code != 0 {
		t.Fatal("reject failed")
	}
	ev := mustLoad(t, s, "alpha").Rejection.Evidence
	if len(ev) != 1 || ev[0].SHA256 != sha256Of(t, "feature-dir copy\n") {
		t.Fatalf("feature dir must win resolution: %+v", ev)
	}

	// A repo-root-only path still resolves via the second candidate.
	dir2, s2 := newRejectRepo(t, map[string]store.FeatureState{"beta": store.StateRequested})
	if err := os.WriteFile(filepath.Join(dir2, "ROOTONLY.md"), []byte("root only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, code := runRJ("reject", "beta", "--path", dir2,
		"--reason", "obsolete", "--note", "n", "--evidence", "ROOTONLY.md"); code != 0 {
		t.Fatal("repo-root evidence must resolve")
	}
	if ev := mustLoad(t, s2, "beta").Rejection.Evidence; ev[0].SHA256 != sha256Of(t, "root only\n") {
		t.Fatalf("repo-root evidence hash = %+v", ev)
	}
}

// ─── items 16-19: reopen ───────────────────────────────────────────────────

func rejectAlpha(t *testing.T, dir string, extra ...string) {
	t.Helper()
	args := append([]string{"reject", "alpha", "--path", dir,
		"--reason", "premise-disproved", "--note", "premise was wrong",
		"--evidence", "analysis.md", "--actor", "rejector@example.com"}, extra...)
	if _, errOut, code := runRJ(args...); code != 0 {
		t.Fatalf("setup reject failed: exit %d %s", code, errOut)
	}
}

func TestReopen_TransitionsToRequestedAndAppends(t *testing.T) {
	dir, s := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateDefined})
	rejectAlpha(t, dir)

	out, errOut, code := runRJ("reopen", "alpha", "--path", dir,
		"--note", "new information landed", "--actor", "reopener@example.com")
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	st := mustLoad(t, s, "alpha")
	// PRD §3.8 / §5: reopen always lands on `requested`, never on the
	// recorded prior_state (which is retained for audit only).
	if st.State != store.StateRequested {
		t.Fatalf("state = %q, want requested", st.State)
	}
	if st.Rejection == nil {
		t.Fatal("append-only violated: the rejection record was cleared")
	}
	if st.Rejection.PriorState != store.StateDefined {
		t.Errorf("prior_state must survive reopen for audit: %q", st.Rejection.PriorState)
	}
	if st.Rejection.Reason != "premise-disproved" || st.Rejection.Note != "premise was wrong" {
		t.Errorf("prior rejection fields lost: %+v", st.Rejection)
	}
	if len(st.Rejection.History) != 2 {
		t.Fatalf("history len = %d, want 2 (reject + reopen)", len(st.Rejection.History))
	}
	h := st.Rejection.History[1]
	if h.Action != store.RejectionActionReopen {
		t.Errorf("history[1].action = %q", h.Action)
	}
	if h.Actor != "reopener@example.com" || h.Note != "new information landed" {
		t.Errorf("history[1] = %+v", h)
	}
	if h.Timestamp.IsZero() {
		t.Error("reopen timestamp not set")
	}
}

func TestReopen_RefusedFromNonRejectedStates(t *testing.T) {
	for _, src := range []store.FeatureState{
		store.StateRequested, store.StateAnalyzed, store.StateApplied, store.StateUpstreamMerged,
	} {
		t.Run(string(src), func(t *testing.T) {
			dir, s := newRejectRepo(t, map[string]store.FeatureState{"alpha": src})
			_, errOut, code := runRJ("reopen", "alpha", "--path", dir, "--note", "n")
			if code != 3 {
				t.Fatalf("exit %d, want 3; stderr=%s", code, errOut)
			}
			if mustLoad(t, s, "alpha").State != src {
				t.Fatal("state mutated on refusal")
			}
		})
	}
}

func TestReopen_ValidationErrors(t *testing.T) {
	dir, s := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateRequested})
	rejectAlpha(t, dir)

	// Empty note → exit 2, pre-mutation.
	_, errOut, code := runRJ("reopen", "alpha", "--path", dir, "--note", "  ")
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr=%s", code, errOut)
	}
	// Bad NEW evidence → exit 2, pre-mutation.
	_, errOut, code = runRJ("reopen", "alpha", "--path", dir, "--note", "n", "--evidence", "../escape.md")
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr=%s", code, errOut)
	}
	st := mustLoad(t, s, "alpha")
	if st.State != store.StateRejected || len(st.Rejection.History) != 1 {
		t.Fatalf("reopen validation failures must be pre-mutation: state=%q history=%d",
			st.State, len(st.Rejection.History))
	}
}

// Item 26: note-only reopen (zero --evidence) still runs historical
// verification, and a clean verdict OMITS evidence_integrity.
func TestReopen_NoteOnly_CleanVerificationRuns(t *testing.T) {
	dir, s := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateRequested})
	rejectAlpha(t, dir)

	var hashed []string
	restore := evidenceHashFn
	evidenceHashFn = func(abs string) (string, error) {
		hashed = append(hashed, abs)
		return restore(abs)
	}
	t.Cleanup(func() { evidenceHashFn = restore })

	out, _, code := runRJ("reopen", "alpha", "--path", dir, "--note", "reconsidering", "--json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	// Verification is unconditional: the historical file was re-hashed
	// even though no new --evidence was supplied.
	if len(hashed) != 1 || !strings.HasSuffix(hashed[0], filepath.FromSlash("alpha/analysis.md")) {
		t.Fatalf("historical evidence not re-hashed on a note-only reopen: %v", hashed)
	}

	var env map[string]any
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("bad envelope: %v\n%s", err, out)
	}
	if _, present := env["evidence_integrity"]; present {
		t.Errorf("clean verification must OMIT evidence_integrity, got %v", env["evidence_integrity"])
	}
	if _, present := env["divergence_detail"]; present {
		t.Errorf("clean verification must OMIT divergence_detail")
	}
	ev, ok := env["reopen_evidence"].([]any)
	if !ok || len(ev) != 0 {
		t.Errorf("reopen_evidence must serialize as [] on a note-only reopen, got %v", env["reopen_evidence"])
	}

	h := mustLoad(t, s, "alpha").Rejection.History[1]
	if h.EvidenceIntegrity != "" || len(h.DivergenceDetail) != 0 {
		t.Errorf("clean verdict must not be persisted: %+v", h)
	}
}

// Item 26b: note-only reopen with MUTATED historical evidence still
// exits 0 but records evidence_integrity=divergent.
func TestReopen_NoteOnly_DivergentHistoricalEvidence(t *testing.T) {
	dir, s := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateRequested})
	rejectAlpha(t, dir)
	// Mutate the cited file in place — exactly what a re-run of
	// `tpatch analyze` would do.
	writeEvidence(t, dir, "alpha", "analysis.md", "REWRITTEN\n")

	out, _, code := runRJ("reopen", "alpha", "--path", dir, "--note", "reconsidering", "--json")
	if code != 0 {
		t.Fatalf("divergence must NOT block the reopen; exit %d: %s", code, out)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("bad envelope: %v\n%s", err, out)
	}
	if env["evidence_integrity"] != "divergent" {
		t.Fatalf("evidence_integrity = %v, want divergent", env["evidence_integrity"])
	}
	det, ok := env["divergence_detail"].([]any)
	if !ok || len(det) != 1 {
		t.Fatalf("divergence_detail = %v", env["divergence_detail"])
	}
	first := det[0].(map[string]any)
	if first["path"] != "analysis.md" || first["divergent_reason"] != "hash-mismatch" {
		t.Fatalf("divergence_detail[0] = %v", first)
	}

	st := mustLoad(t, s, "alpha")
	if st.State != store.StateRequested {
		t.Fatalf("state = %q, want requested", st.State)
	}
	h := st.Rejection.History[1]
	if h.EvidenceIntegrity != store.EvidenceIntegrityDivergent {
		t.Fatalf("persisted integrity = %q", h.EvidenceIntegrity)
	}
	if len(h.DivergenceDetail) != 1 || h.DivergenceDetail[0].DivergentReason != store.DivergentReasonHashMismatch {
		t.Fatalf("persisted divergence = %+v", h.DivergenceDetail)
	}
	// The ORIGINAL recorded hash is preserved — divergence detection
	// must not overwrite the audit record.
	if st.Rejection.Evidence[0].SHA256 != sha256Of(t, "evidence for alpha\n") {
		t.Fatalf("recorded evidence hash was overwritten: %q", st.Rejection.Evidence[0].SHA256)
	}
}

// Every member of the closed divergence taxonomy that can be provoked
// through the filesystem.
func TestReopen_DivergenceTaxonomy(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(t *testing.T, dir string)
		wantRsn string
	}{
		{
			name:    "hash-mismatch",
			mutate:  func(t *testing.T, dir string) { writeEvidence(t, dir, "alpha", "analysis.md", "changed\n") },
			wantRsn: store.DivergentReasonHashMismatch,
		},
		{
			name: "missing",
			mutate: func(t *testing.T, dir string) {
				if err := os.Remove(filepath.Join(dir, ".tpatch", "features", "alpha", "analysis.md")); err != nil {
					t.Fatal(err)
				}
			},
			wantRsn: store.DivergentReasonMissing,
		},
		{
			name: "non-regular",
			mutate: func(t *testing.T, dir string) {
				p := filepath.Join(dir, ".tpatch", "features", "alpha", "analysis.md")
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(p, 0o755); err != nil {
					t.Fatal(err)
				}
			},
			wantRsn: store.DivergentReasonNonRegular,
		},
		{
			name: "path-safety-failed-at-reopen",
			mutate: func(t *testing.T, dir string) {
				p := filepath.Join(dir, ".tpatch", "features", "alpha", "analysis.md")
				outside := t.TempDir()
				target := filepath.Join(outside, "elsewhere.md")
				if err := os.WriteFile(target, []byte("elsewhere\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, p); err != nil {
					t.Skipf("symlinks unsupported: %v", err)
				}
			},
			wantRsn: store.DivergentReasonPathSafetyFailed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, s := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateRequested})
			rejectAlpha(t, dir)
			tc.mutate(t, dir)

			_, errOut, code := runRJ("reopen", "alpha", "--path", dir, "--note", "reconsidering")
			if code != 0 {
				t.Fatalf("divergence must not block reopen; exit %d %s", code, errOut)
			}
			h := mustLoad(t, s, "alpha").Rejection.History[1]
			if h.EvidenceIntegrity != store.EvidenceIntegrityDivergent {
				t.Fatalf("integrity = %q, want divergent", h.EvidenceIntegrity)
			}
			if len(h.DivergenceDetail) != 1 || h.DivergenceDetail[0].DivergentReason != tc.wantRsn {
				t.Fatalf("divergence = %+v, want reason %q", h.DivergenceDetail, tc.wantRsn)
			}
		})
	}
}

// New evidence attached at reopen is hashed and recorded on the reopen
// history entry (not merged into the rejection's evidence list).
func TestReopen_NewEvidenceRecordedOnHistoryEntry(t *testing.T) {
	dir, s := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateRequested})
	rejectAlpha(t, dir)
	writeEvidence(t, dir, "alpha", "upstream-revert.md", "upstream reverted it\n")

	if _, errOut, code := runRJ("reopen", "alpha", "--path", dir,
		"--note", "upstream reverted", "--evidence", "upstream-revert.md"); code != 0 {
		t.Fatalf("exit %d %s", code, errOut)
	}
	st := mustLoad(t, s, "alpha")
	h := st.Rejection.History[1]
	if len(h.Evidence) != 1 || h.Evidence[0].Path != "upstream-revert.md" {
		t.Fatalf("reopen evidence = %+v", h.Evidence)
	}
	if h.Evidence[0].SHA256 != sha256Of(t, "upstream reverted it\n") {
		t.Fatalf("reopen evidence hash = %q", h.Evidence[0].SHA256)
	}
	if len(st.Rejection.Evidence) != 1 || st.Rejection.Evidence[0].Path != "analysis.md" {
		t.Fatalf("reopen must not rewrite the rejection's own evidence list: %+v", st.Rejection.Evidence)
	}
}

// ─── items 20-21: unbounded reject/reopen cycles preserve full history ─────

func TestRejectReopenCycles_HistoryIsAppendOnlyAndUnbounded(t *testing.T) {
	dir, s := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateRequested})
	const cycles = 3
	for i := 0; i < cycles; i++ {
		if _, errOut, code := runRJ("reject", "alpha", "--path", dir,
			"--reason", "obsolete", "--note", "cycle", "--evidence", "analysis.md"); code != 0 {
			t.Fatalf("cycle %d reject: exit %d %s", i, code, errOut)
		}
		if _, errOut, code := runRJ("reopen", "alpha", "--path", dir, "--note", "cycle"); code != 0 {
			t.Fatalf("cycle %d reopen: exit %d %s", i, code, errOut)
		}
	}
	st := mustLoad(t, s, "alpha")
	if st.State != store.StateRequested {
		t.Fatalf("final state = %q", st.State)
	}
	if len(st.Rejection.History) != cycles*2 {
		t.Fatalf("history len = %d, want %d (nothing may be truncated)", len(st.Rejection.History), cycles*2)
	}
	for i, h := range st.Rejection.History {
		want := store.RejectionActionReject
		if i%2 == 1 {
			want = store.RejectionActionReopen
		}
		if h.Action != want {
			t.Fatalf("history[%d].action = %q, want %q", i, h.Action, want)
		}
	}
	// Timestamps are non-decreasing — the log reads oldest-first.
	for i := 1; i < len(st.Rejection.History); i++ {
		if st.Rejection.History[i].Timestamp.Before(st.Rejection.History[i-1].Timestamp) {
			t.Fatalf("history is not chronological at index %d", i)
		}
	}
}

// ─── item 23: actor precedence, observed through the CLI ───────────────────

func TestReject_ActorPrecedenceThroughCLI(t *testing.T) {
	t.Run("flag-wins", func(t *testing.T) {
		dir, s := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateRequested})
		t.Setenv("TPATCH_ACTOR", "env@example.com")
		rejectAlpha(t, dir)
		if got := mustLoad(t, s, "alpha").Rejection.Actor; got != "rejector@example.com" {
			t.Fatalf("actor = %q, want the --actor flag value", got)
		}
	})
	t.Run("env-beats-git-config", func(t *testing.T) {
		dir, s := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateRequested})
		t.Setenv("TPATCH_ACTOR", "env@example.com")
		if _, _, code := runRJ("reject", "alpha", "--path", dir,
			"--reason", "obsolete", "--note", "n", "--evidence", "analysis.md"); code != 0 {
			t.Fatal("reject failed")
		}
		if got := mustLoad(t, s, "alpha").Rejection.Actor; got != "env@example.com" {
			t.Fatalf("actor = %q, want env@example.com", got)
		}
	})
	t.Run("git-config-fallback", func(t *testing.T) {
		dir, s := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateRequested})
		os.Unsetenv("TPATCH_ACTOR")
		if _, _, code := runRJ("reject", "alpha", "--path", dir,
			"--reason", "obsolete", "--note", "n", "--evidence", "analysis.md"); code != 0 {
			t.Fatal("reject failed")
		}
		// gitInitTestRepo pins user.email=test@test.com in the worktree.
		if got := mustLoad(t, s, "alpha").Rejection.Actor; got != "test@test.com" {
			t.Fatalf("actor = %q, want the repo's git config user.email", got)
		}
	})
}

// ─── item 24: confirm-upstreamed on a rejected slug ────────────────────────
//
// The guard is defense-in-depth: it must fire BEFORE the reconcile
// revision append, so neither status.json nor the revisions log moves.
func TestConfirmUpstreamed_RefusedOnRejectedFeature(t *testing.T) {
	dir, slug, upstream, _ := setupConfirmUpstreamedReviewFixture(t)
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	f := mustLoad(t, s, slug)
	f.State = store.StateRejected
	f.Rejection = &store.RejectionStatus{
		Reason: "premise-disproved", Note: "n", Actor: "a", PriorState: store.StateRequested,
	}
	if err := s.SaveFeatureStatus(f); err != nil {
		t.Fatal(err)
	}
	beforeRevs, err := store.LoadReconcileRevisions(s, slug)
	if err != nil {
		t.Fatal(err)
	}

	_, errOut, code := runRJ("reconcile", "confirm-upstreamed", "--path", dir, slug,
		"--upstream-commit", upstream)
	if code != 3 {
		t.Fatalf("exit %d, want 3; stderr=%s", code, errOut)
	}
	if !strings.Contains(errOut, "rejected") || !strings.Contains(errOut, "tpatch reopen") {
		t.Errorf("refusal message = %s", errOut)
	}

	after := mustLoad(t, s, slug)
	if after.State != store.StateRejected {
		t.Fatalf("state mutated: %q", after.State)
	}
	if after.Reconcile.UpstreamCommit != "" {
		t.Fatalf("reconcile metadata mutated: %+v", after.Reconcile)
	}
	afterRevs, err := store.LoadReconcileRevisions(s, slug)
	if err != nil {
		t.Fatal(err)
	}
	if len(afterRevs) != len(beforeRevs) {
		t.Fatalf("a false audit revision was appended: %d → %d", len(beforeRevs), len(afterRevs))
	}
}

// `apply` and `reconcile <slug>` also refuse on a rejected feature.
func TestApplyAndReconcile_RefuseRejectedFeature(t *testing.T) {
	dir, _ := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateRequested})
	rejectAlpha(t, dir)

	_, errOut, code := runRJ("apply", "alpha", "--path", dir)
	if code != 3 {
		t.Fatalf("apply: exit %d, want 3; stderr=%s", code, errOut)
	}
	_, errOut, code = runRJ("reconcile", "alpha", "--path", dir)
	if code != 3 {
		t.Fatalf("reconcile: exit %d, want 3; stderr=%s", code, errOut)
	}
	if !strings.Contains(errOut, "rejected") {
		t.Errorf("reconcile refusal message = %s", errOut)
	}
}

// ─── status filtering + next guidance ──────────────────────────────────────

func TestStatus_RejectedHiddenUnlessIncludeRejected(t *testing.T) {
	dir, _ := newRejectRepo(t, map[string]store.FeatureState{
		"alpha": store.StateRequested,
		"beta":  store.StateRequested,
	})
	rejectAlpha(t, dir)

	out, _, code := runRJ("status", "--path", dir)
	if code != 0 {
		t.Fatalf("status exit %d", code)
	}
	if strings.Contains(out, "alpha") {
		t.Errorf("rejected feature must be hidden by default:\n%s", out)
	}
	if !strings.Contains(out, "beta") {
		t.Errorf("active feature missing:\n%s", out)
	}
	if !strings.Contains(out, "Rejected (hidden): 1") {
		t.Errorf("the omission must be made visible:\n%s", out)
	}

	out, _, code = runRJ("status", "--path", dir, "--include-rejected")
	if code != 0 {
		t.Fatalf("status --include-rejected exit %d", code)
	}
	if !strings.Contains(out, "alpha") {
		t.Errorf("--include-rejected must show the rejected feature:\n%s", out)
	}
}

func TestStatusJSON_RejectedHiddenCount(t *testing.T) {
	dir, _ := newRejectRepo(t, map[string]store.FeatureState{
		"alpha": store.StateRequested,
		"beta":  store.StateRequested,
	})
	rejectAlpha(t, dir)

	out, _, code := runRJ("status", "--path", dir, "--json")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, out)
	}
	if env["rejected_hidden"] != float64(1) {
		t.Errorf("rejected_hidden = %v, want 1", env["rejected_hidden"])
	}
	feats, _ := env["features"].([]any)
	for _, f := range feats {
		if m, ok := f.(map[string]any); ok && m["slug"] == "alpha" {
			t.Fatalf("rejected feature leaked into the default JSON listing:\n%s", out)
		}
	}

	out, _, _ = runRJ("status", "--path", dir, "--json", "--include-rejected")
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	found := false
	feats, _ = env["features"].([]any)
	for _, f := range feats {
		if m, ok := f.(map[string]any); ok && m["slug"] == "alpha" {
			found = true
		}
	}
	if !found {
		t.Fatalf("--include-rejected must include the feature:\n%s", out)
	}
}

// The per-feature detail view is never filtered (PRD §3.7).
func TestStatus_FeatureDetailAlwaysShowsRejection(t *testing.T) {
	dir, _ := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateAnalyzed})
	rejectAlpha(t, dir)

	out, _, code := runRJ("status", "--path", dir, "--feature", "alpha")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	for _, want := range []string{
		"Rejection:", "premise-disproved", "premise was wrong",
		"rejector@example.com", "Prior state: analyzed", "analysis.md",
		"tpatch reopen alpha",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("detail view missing %q:\n%s", want, out)
		}
	}
}

func TestNext_RejectedFeatureEmitsReopenGuidance(t *testing.T) {
	dir, _ := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateRequested})
	rejectAlpha(t, dir)

	out, _, code := runRJ("next", "alpha", "--path", dir, "--format", "harness-json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	var task map[string]any
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		t.Fatalf("bad harness JSON: %v\n%s", err, out)
	}
	if task["phase"] != "rejected" {
		t.Fatalf("phase = %v, want rejected", task["phase"])
	}
	oc, _ := task["on_complete"].(string)
	if !strings.HasPrefix(oc, "tpatch reopen alpha") {
		t.Fatalf("on_complete = %q, want the reopen command", oc)
	}
	instr, _ := task["instructions"].(string)
	if !strings.Contains(instr, "premise-disproved") {
		t.Errorf("instructions must carry the rejection reason: %s", instr)
	}
	for _, forbidden := range []string{"tpatch analyze", "tpatch define", "tpatch implement", "tpatch apply"} {
		if strings.Contains(oc, forbidden) {
			t.Errorf("next must not propose a forward phase for a rejected feature: %q", oc)
		}
	}
}

// ─── item 25: exhaustive exit-code assertions ──────────────────────────────

func TestRejectReopen_ExitCodeEnvelope(t *testing.T) {
	dir, _ := newRejectRepo(t, map[string]store.FeatureState{
		"alpha": store.StateRequested,
		"gamma": store.StateApplied,
	})

	// 2 — unresolvable input (unknown slug).
	if _, _, code := runRJ("reject", "ghost", "--path", dir,
		"--reason", "duplicate", "--note", "n", "--evidence", "analysis.md"); code != 2 {
		t.Errorf("unknown slug: exit %d, want 2", code)
	}
	// 2 — invalid reason.
	if _, _, code := runRJ("reject", "alpha", "--path", dir,
		"--reason", "nope", "--note", "n", "--evidence", "analysis.md"); code != 2 {
		t.Errorf("invalid reason: exit %d, want 2", code)
	}
	// 3 — ineligible source state.
	if _, _, code := runRJ("reject", "gamma", "--path", dir,
		"--reason", "duplicate", "--note", "n", "--evidence", "analysis.md"); code != 3 {
		t.Errorf("ineligible state: exit %d, want 3", code)
	}
	// 0 — success.
	if _, _, code := runRJ("reject", "alpha", "--path", dir,
		"--reason", "duplicate", "--note", "n", "--evidence", "analysis.md"); code != 0 {
		t.Errorf("happy path: exit %d, want 0", code)
	}
	// 3 — already rejected.
	if _, _, code := runRJ("reject", "alpha", "--path", dir,
		"--reason", "duplicate", "--note", "n", "--evidence", "analysis.md"); code != 3 {
		t.Errorf("already rejected: exit %d, want 3", code)
	}
	// 2 — reopen with an empty note.
	if _, _, code := runRJ("reopen", "alpha", "--path", dir, "--note", ""); code != 2 {
		t.Errorf("reopen empty note: exit %d, want 2", code)
	}
	// 0 — reopen success.
	if _, _, code := runRJ("reopen", "alpha", "--path", dir, "--note", "back"); code != 0 {
		t.Errorf("reopen: exit %d, want 0", code)
	}
	// 3 — reopen a non-rejected feature.
	if _, _, code := runRJ("reopen", "alpha", "--path", dir, "--note", "again"); code != 3 {
		t.Errorf("reopen non-rejected: exit %d, want 3", code)
	}
}

// The failure envelope carries the exit code in JSON mode too, so a
// harness that only reads stdout can still branch correctly.
func TestReject_JSONEnvelopeCarriesFailureExitCode(t *testing.T) {
	dir, _ := newRejectRepo(t, map[string]store.FeatureState{"alpha": store.StateApplied})
	out, _, code := runRJ("reject", "alpha", "--path", dir,
		"--reason", "duplicate", "--note", "n", "--evidence", "analysis.md", "--json")
	if code != 3 {
		t.Fatalf("exit %d, want 3", code)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("bad envelope: %v\n%s", err, out)
	}
	if env["exit_code"] != float64(3) {
		t.Errorf("exit_code = %v, want 3", env["exit_code"])
	}
	if env["state"] != "applied" {
		t.Errorf("state = %v, want applied", env["state"])
	}
	if s, _ := env["error"].(string); s == "" {
		t.Error("failure envelope must carry an error string")
	}
}

// ─── item 27: --help cross-references (golden strings) ─────────────────────

func TestHelp_RejectAndReconcileCrossReference(t *testing.T) {
	rejectHelp, _, code := runRJ("reject", "--help")
	if code != 0 {
		t.Fatalf("reject --help exit %d", code)
	}
	if !strings.Contains(rejectHelp, rejectReconcileDisambiguation) {
		t.Errorf("`tpatch reject --help` must render the cross-reference:\nwant: %s\ngot:\n%s",
			rejectReconcileDisambiguation, rejectHelp)
	}
	if !strings.Contains(rejectHelp, "tpatch reconcile --reject") {
		t.Errorf("reject --help must name the other surface:\n%s", rejectHelp)
	}

	reconcileHelp, _, code := runRJ("reconcile", "--help")
	if code != 0 {
		t.Fatalf("reconcile --help exit %d", code)
	}
	if !strings.Contains(reconcileHelp, reconcileRejectDisambiguation) {
		t.Errorf("`tpatch reconcile --help` must render the symmetric cross-reference:\nwant: %s\ngot:\n%s",
			reconcileRejectDisambiguation, reconcileHelp)
	}
	if !strings.Contains(reconcileHelp, "tpatch reject <slug>") {
		t.Errorf("reconcile --help must name the other surface:\n%s", reconcileHelp)
	}

	// The two constants must not be the same string — each surface
	// points AT the other one.
	if rejectReconcileDisambiguation == reconcileRejectDisambiguation {
		t.Fatal("the two cross-references must be distinct")
	}

	// reopen --help documents the append-only + unconditional
	// verification contract.
	reopenHelp, _, code := runRJ("reopen", "--help")
	if code != 0 {
		t.Fatalf("reopen --help exit %d", code)
	}
	for _, want := range []string{"append-only", "rejected", "requested", "divergent"} {
		if !strings.Contains(reopenHelp, want) {
			t.Errorf("reopen --help missing %q:\n%s", want, reopenHelp)
		}
	}
}

// The reason enum is rendered in `reject --help` so an operator can
// self-correct after an exit-2 without reading the PRD.
func TestHelp_RejectListsReasonEnum(t *testing.T) {
	out, _, code := runRJ("reject", "--help")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for r := range store.ValidRejectionReasons {
		if !strings.Contains(out, r) {
			t.Errorf("reject --help missing reason %q", r)
		}
	}
	for _, st := range store.RejectableStateList() {
		if !strings.Contains(out, string(st)) {
			t.Errorf("reject --help missing eligible state %q", st)
		}
	}
}
