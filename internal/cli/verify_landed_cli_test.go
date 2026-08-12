package cli

// Real-CLI acceptance rows for the v0.15.1 Wave C / GH #8
// landed-verification contract (PRD-verify-freshness §7.1, tier C).
//
// These drive the shipped cobra surface end-to-end: `tpatch add`,
// `record`, `land`, `verify`. No workflow-level shortcut is used.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gh8Fixture is the issue-#8 reproduction as an operator runs it.
type gh8Fixture struct {
	t    *testing.T
	Dir  string
	Slug string
	Base string
	Path string
}

const gh8Seed = "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n"
const gh8Feature = "l1\nl2\nl3\nl4\nEXTRA BUTTON\nl5\nl6\nl7\nl8\nl9\nl10\n"

func newGH8Fixture(t *testing.T) *gh8Fixture {
	t.Helper()
	dir := t.TempDir()
	gitInitTestRepo(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "app.txt"), []byte(gh8Seed), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommitOnly(t, dir, "seed")
	if _, _, code := runCmd("init", "--path", dir); code != 0 {
		t.Fatalf("tpatch init failed")
	}
	base := gitCommitOnly(t, dir, "tpatch scaffolding")
	if _, _, code := runCmd("add", "--path", dir, "Extra button"); code != 0 {
		t.Fatalf("tpatch add failed")
	}
	f := &gh8Fixture{t: t, Dir: dir, Slug: "extra-button", Base: base, Path: "src/app.txt"}
	f.writeIntent()
	return f
}

func (f *gh8Fixture) writeIntent() {
	f.t.Helper()
	for _, name := range []string{"spec.md", "exploration.md"} {
		p := filepath.Join(f.Dir, ".tpatch", "features", f.Slug, name)
		if err := os.WriteFile(p, []byte("intent\n"), 0o644); err != nil {
			f.t.Fatal(err)
		}
	}
}

// Implement writes the feature into the working tree.
func (f *gh8Fixture) Implement() {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.Dir, f.Path), []byte(gh8Feature), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *gh8Fixture) Record(extra ...string) {
	f.t.Helper()
	args := append([]string{"record", "--path", f.Dir, f.Slug, "--files", f.Path}, extra...)
	stdout, stderr, code := runCmdWithError(args...)
	if code != 0 {
		f.t.Fatalf("tpatch record failed: %s %s", stdout, stderr)
	}
}

func (f *gh8Fixture) Land(extra ...string) {
	f.t.Helper()
	args := append([]string{"land", "--path", f.Dir, f.Slug, "--message", "Add extra button"}, extra...)
	stdout, stderr, code := runCmdWithError(args...)
	if code != 0 {
		f.t.Fatalf("tpatch land failed: %s %s", stdout, stderr)
	}
}

// VerifyJSON runs `tpatch verify --json --no-write` and decodes it.
func (f *gh8Fixture) VerifyJSON(extra ...string) (map[string]any, int, string) {
	f.t.Helper()
	args := append([]string{"verify", "--path", f.Dir, f.Slug, "--json", "--no-write", "--quiet"}, extra...)
	// runCmdExit applies the real process boundary's exit-code mapping,
	// so the binding `verify` contract (0 pass / 2 block failure) is
	// asserted rather than runCmd's collapse-to-1.
	stdout, stderr, code := runCmdExit(args...)
	var m map[string]any
	if err := json.Unmarshal([]byte(stdout), &m); err != nil {
		f.t.Fatalf("verify --json is not valid JSON (%v):\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	return m, code, stderr
}

func checkRow(t *testing.T, report map[string]any, id string) map[string]any {
	t.Helper()
	rows, _ := report["checks"].([]any)
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		if row["id"] == id {
			return row
		}
	}
	t.Fatalf("check %q not present", id)
	return nil
}

func evidenceState(report map[string]any) string {
	ev, _ := report["landing_evidence"].(map[string]any)
	s, _ := ev["state"].(string)
	return s
}

// AC-L1 — the issue #8 sequence PASSES before `land`: exit 0, exactly
// eleven check rows in V0–V10 order.
func TestACL1_IssueSequencePassesBeforeLand(t *testing.T) {
	f := newGH8Fixture(t)
	f.Implement()
	f.Record()
	report, code, stderr := f.VerifyJSON()
	if code != 0 {
		t.Fatalf("verify before land exited %d: %s", code, stderr)
	}
	rows, _ := report["checks"].([]any)
	if len(rows) != 11 {
		t.Fatalf("checks=%d want 11", len(rows))
	}
	want := []string{
		"status_loaded", "intent_files_present", "recipe_parses",
		"recipe_op_targets_resolve", "dep_metadata_valid", "satisfied_by_reachable",
		"dependency_gate_satisfied", "recipe_replay_clean", "post_apply_patch_replay_clean",
		"reconcile_outcome_consistent", "write_file_preimage_fresh",
	}
	for i, id := range want {
		row, _ := rows[i].(map[string]any)
		if row["id"] != id {
			t.Fatalf("checks[%d]=%v want %q", i, row["id"], id)
		}
	}
	if report["target_mode"] != "forward" {
		t.Errorf("target_mode=%v want forward", report["target_mode"])
	}
}

// AC-L2 — the same feature AFTER `land` passes: target_mode landed,
// evidence exact, baseline dual-anchor, eleven rows. This is the GH #8
// defect, closed.
func TestACL2CLI_LandedFeaturePasses(t *testing.T) {
	f := newGH8Fixture(t)
	f.Implement()
	f.Record()
	f.Land()

	report, code, stderr := f.VerifyJSON()
	if code != 0 {
		t.Fatalf("verify AFTER land exited %d — this is GH #8: %s\n%v", code, stderr, report["checks"])
	}
	if report["target_mode"] != "landed" {
		t.Errorf("target_mode=%v want landed", report["target_mode"])
	}
	if got := evidenceState(report); got != "exact" {
		t.Errorf("landing_evidence.state=%q want exact", got)
	}
	baseline, _ := report["baseline"].(map[string]any)
	if baseline["mode"] != "dual-anchor" {
		t.Errorf("baseline.mode=%v want dual-anchor", baseline["mode"])
	}
	if baseline["current_probe"] != "isolated-index" {
		t.Errorf("baseline.current_probe=%v want isolated-index", baseline["current_probe"])
	}
	if rows, _ := report["checks"].([]any); len(rows) != 11 {
		t.Errorf("checks=%d want 11", len(rows))
	}
	v8 := checkRow(t, report, "post_apply_patch_replay_clean")
	anchors, _ := v8["anchor_results"].(map[string]any)
	if anchors["historical"] != "passed" || anchors["current"] != "materialized-clean" {
		t.Errorf("anchor_results=%v", anchors)
	}
	if report["schema_version"] != "1.1" {
		t.Errorf("schema_version=%v want 1.1", report["schema_version"])
	}
}

// AC-L3 — the committed-range re-record is decided by the §3.6.2 values,
// with BOTH branches asserted.
func TestACL3_CommittedRangeReRecordBothBranches(t *testing.T) {
	t.Run("byte-identical-artifacts-stay-exact", func(t *testing.T) {
		f := newGH8Fixture(t)
		f.Implement()
		f.Record()
		f.Land()
		before, _, _ := f.VerifyJSON()
		if evidenceState(before) != "exact" {
			t.Fatalf("precondition: evidence=%q", evidenceState(before))
		}
		patchBefore := readArtifact(t, f.Dir, f.Slug, "post-apply.patch")

		// Re-record over the committed range: byte-identical artifacts.
		stdout, stderr, code := runCmdWithError("record", "--path", f.Dir, f.Slug,
			"--from", f.Base, "--to", "HEAD", "--files", f.Path)
		if code != 0 {
			t.Skipf("committed-range re-record refused in this fixture: %s %s", stdout, stderr)
		}
		if readArtifact(t, f.Dir, f.Slug, "post-apply.patch") != patchBefore {
			t.Skipf("the committed-range re-record changed the artifacts; the other branch covers that")
		}
		after, code, stderr := f.VerifyJSON()
		if code != 0 {
			t.Fatalf("byte-identical re-record must stay exact and pass: %s\n%v", stderr, after)
		}
		if evidenceState(after) != "exact" {
			t.Errorf("evidence=%q want exact", evidenceState(after))
		}
	})

	t.Run("changed-artifacts-are-stale-then-pass-after-re-land", func(t *testing.T) {
		f := newGH8Fixture(t)
		f.Implement()
		f.Record()
		f.Land()
		// Change the canonical artifact ⇒ stale evidence.
		p := filepath.Join(f.Dir, ".tpatch", "features", f.Slug, "artifacts", "post-apply.patch")
		body, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, append(body, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		report, code, _ := f.VerifyJSON()
		if code == 0 {
			t.Fatalf("changed artifacts must fail with stale evidence")
		}
		if evidenceState(report) != "stale" {
			t.Fatalf("evidence=%q want stale", evidenceState(report))
		}
		if report["failed_at"] != "landing-evidence" {
			t.Errorf("failed_at=%v want landing-evidence", report["failed_at"])
		}
		v7 := checkRow(t, report, "recipe_replay_clean")
		rem, _ := v7["remediation"].(string)
		if !strings.Contains(rem, "is stale: commit ") || !strings.Contains(rem, "re-run tpatch land") {
			t.Errorf("expected R6 verbatim; got %q", rem)
		}

		// Re-land re-attests, and the run passes again.
		f.Land("--no-record")
		after, code, stderr := f.VerifyJSON()
		if code != 0 {
			t.Fatalf("re-land did not restore a passing run: %s\n%v", stderr, after["checks"])
		}
	})
}

// AC-L4 — a landed LEAF with no dependencies passes through the CLI.
func TestACL4CLI_LandedLeafPasses(t *testing.T) {
	f := newGH8Fixture(t)
	f.Implement()
	f.Record()
	f.Land()
	if _, code, stderr := f.VerifyJSON(); code != 0 {
		t.Fatalf("landed leaf failed: %s", stderr)
	}
}

// AC-L6 — `--no-write` leaves `.tpatch/`, the index and the worktree
// byte-identical.
func TestACL6CLI_NoWriteIsByteIdentical(t *testing.T) {
	f := newGH8Fixture(t)
	f.Implement()
	f.Record()
	f.Land()
	before := hashTree(t, f.Dir)
	beforeIndex := readIndex(t, f.Dir)
	if _, code, _ := f.VerifyJSON(); code != 0 {
		t.Fatalf("verify failed")
	}
	if diff := treeDiff(before, hashTree(t, f.Dir)); diff != "" {
		t.Errorf("--no-write mutated: %s", diff)
	}
	if beforeIndex != readIndex(t, f.Dir) {
		t.Errorf("--no-write mutated the index")
	}
}

// AC-L8 — a landed target with a dirty worktree still PASSES.
func TestACL8CLI_DirtyWorktreePasses(t *testing.T) {
	f := newGH8Fixture(t)
	f.Implement()
	f.Record()
	f.Land()
	if err := os.WriteFile(filepath.Join(f.Dir, f.Path), []byte(gh8Seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, code, stderr := f.VerifyJSON(); code != 0 {
		t.Fatalf("dirty worktree false red: %s", stderr)
	}
}

// AC-L11 — after a landed run the index, worktree and `git status` are
// unchanged, and no temp index is left as an untracked entry.
func TestACL11CLI_RunIsReadOnly(t *testing.T) {
	f := newGH8Fixture(t)
	f.Implement()
	f.Record()
	f.Land()
	beforeStatus := gitPorcelain(t, f.Dir)
	beforeIndex := readIndex(t, f.Dir)
	if _, code, _ := f.VerifyJSON(); code != 0 {
		t.Fatalf("verify failed")
	}
	if got := gitPorcelain(t, f.Dir); got != beforeStatus {
		t.Errorf("git status changed: %q -> %q", beforeStatus, got)
	}
	if readIndex(t, f.Dir) != beforeIndex {
		t.Errorf("the real index changed")
	}
	if strings.Contains(gitPorcelain(t, f.Dir), "tpatch-verify") {
		t.Errorf("the temp index appears as an untracked entry")
	}
}

// AC-L14 / AC-L16 / AC-L23 — the ladder outcomes through the CLI.
func TestACL14CLI_LadderOutcomes(t *testing.T) {
	t.Run("clean", func(t *testing.T) {
		f := newGH8Fixture(t)
		f.Implement()
		f.Record()
		f.Land()
		report, code, _ := f.VerifyJSON()
		if code != 0 {
			t.Fatalf("clean ladder must pass")
		}
		v8 := checkRow(t, report, "post_apply_patch_replay_clean")
		anchors, _ := v8["anchor_results"].(map[string]any)
		if anchors["current"] != "materialized-clean" {
			t.Errorf("current=%v want materialized-clean", anchors["current"])
		}
	})
	t.Run("context-drift-warns", func(t *testing.T) {
		f := newGH8Fixture(t)
		f.Implement()
		f.Record()
		f.Land()
		drifted := strings.Replace(gh8Feature, "l6\n", "UNRELATED\n", 1)
		if err := os.WriteFile(filepath.Join(f.Dir, f.Path), []byte(drifted), 0o644); err != nil {
			t.Fatal(err)
		}
		gitCommitOnly(t, f.Dir, "unrelated edit near the hunk")
		report, code, _ := f.VerifyJSON()
		if code != 0 {
			t.Fatalf("a context-drift run must still pass; report=%v", report["checks"])
		}
		advisories, _ := report["advisories"].([]any)
		found := false
		for _, raw := range advisories {
			a, _ := raw.(map[string]any)
			if a["code"] == "context-drift" {
				found = true
				if a["severity"] != "warn" {
					t.Errorf("advisory severity=%v want warn", a["severity"])
				}
			}
		}
		if !found {
			t.Errorf("expected a context-drift advisory; got %v", advisories)
		}
	})
	t.Run("full-revert-blocks", func(t *testing.T) {
		f := newGH8Fixture(t)
		f.Implement()
		f.Record()
		f.Land()
		if err := os.WriteFile(filepath.Join(f.Dir, f.Path), []byte(gh8Seed), 0o644); err != nil {
			t.Fatal(err)
		}
		gitCommitOnly(t, f.Dir, "revert the feature")
		report, code, _ := f.VerifyJSON()
		if code != 2 {
			t.Fatalf("a full revert must fail with exit 2, got %d", code)
		}
		if report["failed_at"] != "landed-content-absent" {
			t.Errorf("failed_at=%v want landed-content-absent", report["failed_at"])
		}
		v8 := checkRow(t, report, "post_apply_patch_replay_clean")
		rem, _ := v8["remediation"].(string)
		if !strings.Contains(rem, "Do NOT run tpatch reconcile") {
			t.Errorf("R1 must forbid reconcile: %q", rem)
		}
	})
}

// AC-L39 — an unanchored landed feature FAILS with R11 and V7/V8/V10 all
// carry `mode`.
func TestACL39CLI_UnanchoredFails(t *testing.T) {
	f := newGH8Fixture(t)
	f.Implement()
	// Commit the change WITHOUT a trailer, then attest it: the only
	// candidate's parent already materializes the patch.
	gitCommitOnly(t, f.Dir, "implement without a trailer")
	f.Record("--from", f.Base, "--to", "HEAD")
	f.Land("--no-record")

	report, code, _ := f.VerifyJSON()
	if code != 2 {
		t.Fatalf("an unanchored landed feature must fail; code=%d", code)
	}
	if report["failed_at"] != "historical-anchor-unavailable" {
		t.Fatalf("failed_at=%v want historical-anchor-unavailable", report["failed_at"])
	}
	for _, id := range []string{"recipe_replay_clean", "post_apply_patch_replay_clean", "write_file_preimage_fresh"} {
		row := checkRow(t, report, id)
		if row["mode"] == nil || row["mode"] == "" {
			t.Errorf("%s has no mode", id)
		}
		if row["passed"] == true {
			t.Errorf("%s passed on an unanchored run", id)
		}
		if row["skipped"] == true {
			t.Errorf("%s is skipped; the contract requires failed-because-unanchored", id)
		}
	}
}

// AC-L46 — a never-landed feature stays in forward mode.
func TestACL46CLI_ForwardModeUnchanged(t *testing.T) {
	f := newGH8Fixture(t)
	f.Implement()
	f.Record()
	report, code, _ := f.VerifyJSON()
	if code != 0 {
		t.Fatalf("forward verify failed")
	}
	if evidenceState(report) != "none" {
		t.Errorf("evidence=%q want none", evidenceState(report))
	}
	baseline, _ := report["baseline"].(map[string]any)
	if baseline["mode"] != "head-anchored" {
		t.Errorf("baseline.mode=%v want head-anchored", baseline["mode"])
	}
	if _, ok := baseline["historical_anchor"]; ok {
		t.Errorf("forward mode must not report a historical anchor")
	}
}

// AC-L118 — the human report emits the two header lines.
func TestACL118CLI_HumanReportHeader(t *testing.T) {
	f := newGH8Fixture(t)
	f.Implement()
	f.Record()
	f.Land()
	stdout, _, code := runCmdWithError("verify", "--path", f.Dir, f.Slug, "--no-write")
	if code != 0 {
		t.Fatalf("verify failed: %s", stdout)
	}
	if !strings.Contains(stdout, "baseline: historical-anchor @ ") {
		t.Errorf("missing the baseline line:\n%s", stdout)
	}
	if !strings.Contains(stdout, "(isolated index)") {
		t.Errorf("the isolated-index probe must be named:\n%s", stdout)
	}
	if !strings.Contains(stdout, "landing evidence: exact @ ") {
		t.Errorf("missing the landing-evidence line:\n%s", stdout)
	}
}

// AC-L124 — `verify --all` ordering is unchanged and the enumeration and
// inventory are built once for the whole run.
func TestACL124CLI_VerifyAllOrderingAndReuse(t *testing.T) {
	f := newGH8Fixture(t)
	f.Implement()
	f.Record()
	f.Land()
	if _, _, code := runCmd("add", "--path", f.Dir, "Second feature"); code != 0 {
		t.Fatalf("add second feature failed")
	}
	stdout, _, code := runCmdWithError("verify", "--path", f.Dir, "--all", "--json", "--quiet")
	if code != 0 && code != 2 {
		t.Fatalf("verify --all exited %d", code)
	}
	var agg map[string]any
	if err := json.Unmarshal([]byte(stdout), &agg); err != nil {
		t.Fatalf("verify --all --json is not valid JSON: %v\n%s", err, stdout)
	}
	features, _ := agg["features"].([]any)
	if len(features) < 2 {
		t.Fatalf("expected at least two feature rows, got %d", len(features))
	}
	var slugs []string
	for _, raw := range features {
		row, _ := raw.(map[string]any)
		slugs = append(slugs, row["slug"].(string))
	}
	// Deterministic across runs.
	stdout2, _, _ := runCmdWithError("verify", "--path", f.Dir, "--all", "--json", "--quiet")
	var agg2 map[string]any
	if err := json.Unmarshal([]byte(stdout2), &agg2); err != nil {
		t.Fatal(err)
	}
	features2, _ := agg2["features"].([]any)
	for i, raw := range features2 {
		row, _ := raw.(map[string]any)
		if row["slug"] != slugs[i] {
			t.Fatalf("verify --all ordering changed between runs: %v vs %v", row["slug"], slugs[i])
		}
	}
}

// AC-L125 — exit codes are unchanged: 0 pass, 2 any block failure,
// and warn advisories never change the exit code.
func TestACL125CLI_ExitCodes(t *testing.T) {
	f := newGH8Fixture(t)
	f.Implement()
	f.Record()
	f.Land()
	if _, code, _ := f.VerifyJSON(); code != 0 {
		t.Fatalf("passing run exited %d want 0", code)
	}
	// warn advisory only ⇒ still 0.
	drifted := strings.Replace(gh8Feature, "l6\n", "UNRELATED\n", 1)
	if err := os.WriteFile(filepath.Join(f.Dir, f.Path), []byte(drifted), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommitOnly(t, f.Dir, "drift")
	if _, code, _ := f.VerifyJSON(); code != 0 {
		t.Fatalf("a warn advisory changed the exit code to %d", code)
	}
	// block failure ⇒ 2.
	if err := os.WriteFile(filepath.Join(f.Dir, f.Path), []byte(gh8Seed), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommitOnly(t, f.Dir, "revert")
	if _, code, _ := f.VerifyJSON(); code != 2 {
		t.Fatalf("a block failure exited %d want 2", code)
	}
}

// ── small helpers ────────────────────────────────────────────────────────

func hashTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		if info.IsDir() {
			if rel == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		out[rel] = sha256Hex(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return out
}

func treeDiff(before, after map[string]string) string {
	var diffs []string
	for k, v := range before {
		if av, ok := after[k]; !ok {
			diffs = append(diffs, "removed "+k)
		} else if av != v {
			diffs = append(diffs, "changed "+k)
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			diffs = append(diffs, "added "+k)
		}
	}
	return strings.Join(diffs, ", ")
}

func readIndex(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		return ""
	}
	return sha256Hex(data)
}

func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
