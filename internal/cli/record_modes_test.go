package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── Helpers ────────────────────────────────────────────────────────────────

// modesFixture creates a fresh git repo + tpatch init + one feature
// and returns (repo path, slug). Use this as the baseline for all
// capture-mode tests so each test starts from a known clean HEAD.
func modesFixture(t *testing.T, slug string) string {
	t.Helper()
	tmp := t.TempDir()
	gitInitTestRepo(t, tmp)
	if _, _, code := runCmd("init", "--path", tmp); code != 0 {
		t.Fatalf("tpatch init failed")
	}
	if _, _, code := runCmd("add", "--path", tmp, "--slug", slug, "Capture-mode fixture"); code != 0 {
		t.Fatalf("tpatch add failed")
	}
	return tmp
}

// modesWriteFile writes a file under the repo and returns its absolute
// path. Helper to keep individual test bodies compact.
func modesWriteFile(t *testing.T, tmp, rel, content string) string {
	t.Helper()
	p := filepath.Join(tmp, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// readRecordedPatch returns the canonical artifacts/post-apply.patch
// for the named feature, fatal-ing the test on read errors.
func readRecordedPatch(t *testing.T, tmp, slug string) string {
	t.Helper()
	p := filepath.Join(tmp, ".tpatch", "features", slug, "artifacts", "post-apply.patch")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read post-apply.patch for %s: %v", slug, err)
	}
	return string(b)
}

// readRecordMD returns the per-feature record.md.
func readRecordMD(t *testing.T, tmp, slug string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(tmp, ".tpatch", "features", slug, "record.md"))
	if err != nil {
		t.Fatalf("read record.md for %s: %v", slug, err)
	}
	return string(b)
}

// runRecord is a thin wrapper around the root command that returns
// stdout, stderr and an error. It mirrors runClaim / runCmdWithError
// so failure messages include the full error message in stderr.
func runRecord(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var out, errBuf bytes.Buffer
	root := buildRootCmd()
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		errBuf.WriteString(err.Error())
		return out.String(), errBuf.String(), 1
	}
	return out.String(), errBuf.String(), 0
}

// ─── PRD §7 bullet 1: --all == default (byte-identical) ────────────────────

// TestRecordModes_AllEqualsDefault — PRD §7 bullet 1.
//
// `record --all` must produce a byte-identical post-apply.patch
// compared to the default `record` invocation given the same repo
// state. The provenance string in record.md changes between the two,
// but the canonical patch bytes do not.
func TestRecordModes_AllEqualsDefault(t *testing.T) {
	tmpA := modesFixture(t, "alpha")
	modesWriteFile(t, tmpA, "src/server.go", "package main\nfunc f() {}\n")
	if _, stderr, code := runRecord(t, "record", "--path", tmpA, "alpha", "--lenient"); code != 0 {
		t.Fatalf("default record failed: %s", stderr)
	}
	defaultBytes := readRecordedPatch(t, tmpA, "alpha")

	tmpB := modesFixture(t, "alpha")
	modesWriteFile(t, tmpB, "src/server.go", "package main\nfunc f() {}\n")
	if _, stderr, code := runRecord(t, "record", "--path", tmpB, "alpha", "--all", "--lenient"); code != 0 {
		t.Fatalf("record --all failed: %s", stderr)
	}
	allBytes := readRecordedPatch(t, tmpB, "alpha")

	if defaultBytes != allBytes {
		t.Fatalf("--all and default produce different post-apply.patch bytes\n--- default ---\n%s\n--- --all ---\n%s", defaultBytes, allBytes)
	}

	// And the --all path must still write `working-tree-all`
	// provenance to record.md (so future patch-identity-metadata work
	// has the explicit signal).
	md := readRecordMD(t, tmpB, "alpha")
	if !strings.Contains(md, "capture_mode") || !strings.Contains(md, "working-tree-all") {
		t.Errorf("--all record.md must declare working-tree-all provenance:\n%s", md)
	}
}

// ─── PRD §7 bullet 12: mutex matrix returns errors before capture ───────────

// TestRecordModes_MutexMatrix covers PRD §3.7 / §7 bullet 12. Each
// disallowed pair must refuse before patch capture runs.
func TestRecordModes_MutexMatrix(t *testing.T) {
	tmp := modesFixture(t, "mux")
	modesWriteFile(t, tmp, "src/server.go", "package main\n")

	pairs := []struct {
		name string
		args []string
	}{
		{"all+staged", []string{"--all", "--staged"}},
		{"all+unstaged", []string{"--all", "--unstaged"}},
		{"staged+unstaged", []string{"--staged", "--unstaged"}},
		{"all+auto", []string{"--all", "--auto"}},
		{"all+from", []string{"--all", "--from", "HEAD~"}},
		{"all+range", []string{"--all", "--commit-range", "HEAD~..HEAD"}},
		{"staged+auto", []string{"--staged", "--auto"}},
		{"staged+from", []string{"--staged", "--from", "HEAD~"}},
		{"staged+range", []string{"--staged", "--commit-range", "HEAD~..HEAD"}},
		{"unstaged+auto", []string{"--unstaged", "--auto"}},
		{"unstaged+from", []string{"--unstaged", "--from", "HEAD~"}},
		{"unstaged+range", []string{"--unstaged", "--commit-range", "HEAD~..HEAD"}},
	}
	for _, tc := range pairs {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"record", "--path", tmp, "mux"}, tc.args...)
			_, stderr, code := runRecord(t, args...)
			if code == 0 {
				t.Fatalf("expected refusal for %s, stderr=%q", tc.name, stderr)
			}
			if !strings.Contains(stderr, "mutually exclusive") {
				t.Errorf("expected 'mutually exclusive' diagnostic for %s, got: %s", tc.name, stderr)
			}
			// Mutex must trip BEFORE capture — no post-apply.patch.
			ppPath := filepath.Join(tmp, ".tpatch", "features", "mux", "artifacts", "post-apply.patch")
			if _, err := os.Stat(ppPath); err == nil {
				t.Errorf("%s: mutex failed AFTER capture (post-apply.patch exists)", tc.name)
			}
		})
	}
}

// ─── PRD §7 bullets 2, 4: --staged happy path & new-file gating ────────────

// TestRecordModes_StagedHappyPath — PRD §7 bullet 2.
// stage one hunk → record --staged → patch contains exactly that hunk.
func TestRecordModes_StagedHappyPath(t *testing.T) {
	tmp := modesFixture(t, "feat")
	modesWriteFile(t, tmp, "src/server.go", "line1\nline2\n")
	gitRun(t, tmp, "add", "src/server.go")
	gitRun(t, tmp, "commit", "-m", "seed server.go")

	// Stage a change. Leave the worktree exactly matching the index
	// (no further edits) so this is the simplest happy path: the
	// staged patch must capture the staged change, and the worktree
	// has nothing extra to refuse against.
	modesWriteFile(t, tmp, "src/server.go", "line1 STAGED\nline2\n")
	gitRun(t, tmp, "add", "src/server.go")

	_, stderr, code := runRecord(t, "record", "--path", tmp, "feat", "--staged", "--lenient")
	if code != 0 {
		t.Fatalf("record --staged failed: %s", stderr)
	}
	patch := readRecordedPatch(t, tmp, "feat")
	if !strings.Contains(patch, "STAGED") {
		t.Errorf("staged patch must contain staged change: %s", patch)
	}
	md := readRecordMD(t, tmp, "feat")
	if !strings.Contains(md, "staged-index") {
		t.Errorf("record.md must declare staged-index provenance:\n%s", md)
	}
}

// TestRecordModes_StagedTwoFilesIsolation — PRD §7 bullet 2 (the
// "happy" shape with two files: file A is staged in full, file B is
// unstaged in full; --staged captures A only and emits a note about
// B). This is the realistic operator workflow: split the feature into
// curated index hunks first, leave the rest dirty.
func TestRecordModes_StagedTwoFilesIsolation(t *testing.T) {
	tmp := modesFixture(t, "split")
	// Seed both files first so subsequent staging operations don't
	// accidentally commit each other.
	modesWriteFile(t, tmp, "src/a.go", "a-original\n")
	modesWriteFile(t, tmp, "src/b.go", "b-original\n")
	gitRun(t, tmp, "add", ".")
	gitRun(t, tmp, "commit", "-m", "seed")

	// A: staged. B: unstaged tracked.
	modesWriteFile(t, tmp, "src/a.go", "a-staged\n")
	gitRun(t, tmp, "add", "src/a.go")
	modesWriteFile(t, tmp, "src/b.go", "b-unstaged-wip\n")

	_, stderr, code := runRecord(t, "record", "--path", tmp, "split", "--staged", "--lenient")
	if code != 0 {
		t.Fatalf("record --staged failed: %s", stderr)
	}
	patch := readRecordedPatch(t, tmp, "split")
	if !strings.Contains(patch, "src/a.go") {
		t.Errorf("--staged patch must contain src/a.go: %s", patch)
	}
	if strings.Contains(patch, "src/b.go") {
		t.Errorf("--staged patch must NOT contain src/b.go: %s", patch)
	}
	if !strings.Contains(stderr, "unrelated unstaged") {
		t.Errorf("expected note about unrelated unstaged path, got stderr: %s", stderr)
	}
	md := readRecordMD(t, tmp, "split")
	if !strings.Contains(md, "staged-index") {
		t.Errorf("record.md must declare staged-index provenance:\n%s", md)
	}
}

// TestRecordModes_StagedRefusesOnOverlap — PRD §7 bullet 3.
func TestRecordModes_StagedRefusesOnOverlap(t *testing.T) {
	tmp := modesFixture(t, "ov")
	modesWriteFile(t, tmp, "src/server.go", "line1\nline2\n")
	gitRun(t, tmp, "add", "src/server.go")
	gitRun(t, tmp, "commit", "-m", "seed")

	modesWriteFile(t, tmp, "src/server.go", "line1 STAGED\nline2\n")
	gitRun(t, tmp, "add", "src/server.go")
	modesWriteFile(t, tmp, "src/server.go", "line1 STAGED\nline2 WIP\n")

	_, stderr, code := runRecord(t, "record", "--path", tmp, "ov", "--staged", "--lenient")
	if code == 0 {
		t.Fatalf("expected --staged refusal on overlap, stderr=%s", stderr)
	}
	if !strings.Contains(stderr, "refuses") || !strings.Contains(stderr, "src/server.go") {
		t.Errorf("refusal must name overlapping path: %s", stderr)
	}
}

// TestRecordModes_StagedNewFileGating — PRD §7 bullet 4. A truly
// untracked file (no `git add`) must NOT appear in a --staged patch;
// a `git add`-ed new file MUST appear.
func TestRecordModes_StagedNewFileGating(t *testing.T) {
	tmp := modesFixture(t, "newf")

	// Tracked addition: git-added before record.
	modesWriteFile(t, tmp, "src/added.go", "added\n")
	gitRun(t, tmp, "add", "src/added.go")
	// Plain untracked file: not git-added.
	modesWriteFile(t, tmp, "src/orphan.go", "orphan\n")

	_, stderr, code := runRecord(t, "record", "--path", tmp, "newf", "--staged", "--lenient")
	if code != 0 {
		t.Fatalf("record --staged failed: %s", stderr)
	}
	patch := readRecordedPatch(t, tmp, "newf")
	if !strings.Contains(patch, "src/added.go") {
		t.Errorf("--staged patch must include git-added new file: %s", patch)
	}
	if strings.Contains(patch, "src/orphan.go") {
		t.Errorf("--staged patch must NOT include plain untracked file: %s", patch)
	}
}

// TestRecordModes_StagedRefusesEmpty — PRD §3.3 empty refusal.
func TestRecordModes_StagedRefusesEmpty(t *testing.T) {
	tmp := modesFixture(t, "empty")
	modesWriteFile(t, tmp, "src/wip.go", "wip\n") // unstaged only

	_, stderr, code := runRecord(t, "record", "--path", tmp, "empty", "--staged", "--lenient")
	if code == 0 {
		t.Fatalf("expected empty-staged refusal")
	}
	if !strings.Contains(stderr, "nothing staged") {
		t.Errorf("empty-staged refusal must explain nothing staged: %s", stderr)
	}
}

// ─── PRD §7 bullets 5, 6, 7: --unstaged ─────────────────────────────────────

// TestRecordModes_UnstagedHappyPath — PRD §7 bullet 5.
func TestRecordModes_UnstagedHappyPath(t *testing.T) {
	tmp := modesFixture(t, "un")
	modesWriteFile(t, tmp, "src/a.go", "a-orig\n")
	modesWriteFile(t, tmp, "src/b.go", "b-orig\n")
	gitRun(t, tmp, "add", ".")
	gitRun(t, tmp, "commit", "-m", "seed")

	// A is staged in full; B is unstaged in full.
	modesWriteFile(t, tmp, "src/a.go", "a-staged-only\n")
	gitRun(t, tmp, "add", "src/a.go")
	modesWriteFile(t, tmp, "src/b.go", "b-unstaged-only\n")

	_, stderr, code := runRecord(t, "record", "--path", tmp, "un", "--unstaged", "--lenient")
	if code != 0 {
		t.Fatalf("record --unstaged failed: %s", stderr)
	}
	patch := readRecordedPatch(t, tmp, "un")
	if !strings.Contains(patch, "src/b.go") {
		t.Errorf("--unstaged patch must contain src/b.go: %s", patch)
	}
	if strings.Contains(patch, "src/a.go") {
		t.Errorf("--unstaged patch must NOT contain src/a.go: %s", patch)
	}
	if !strings.Contains(stderr, "unrelated staged") {
		t.Errorf("expected note about unrelated staged path, got: %s", stderr)
	}
	md := readRecordMD(t, tmp, "un")
	if !strings.Contains(md, "unstaged-worktree") {
		t.Errorf("record.md must declare unstaged-worktree provenance:\n%s", md)
	}
}

// TestRecordModes_UnstagedRefusesOnOverlap — PRD §7 bullet 6.
func TestRecordModes_UnstagedRefusesOnOverlap(t *testing.T) {
	tmp := modesFixture(t, "uov")
	modesWriteFile(t, tmp, "src/server.go", "v1\n")
	gitRun(t, tmp, "add", "src/server.go")
	gitRun(t, tmp, "commit", "-m", "seed")

	modesWriteFile(t, tmp, "src/server.go", "v2-staged\n")
	gitRun(t, tmp, "add", "src/server.go")
	modesWriteFile(t, tmp, "src/server.go", "v2-staged-then-edited\n")

	_, stderr, code := runRecord(t, "record", "--path", tmp, "uov", "--unstaged", "--lenient")
	if code == 0 {
		t.Fatalf("expected --unstaged overlap refusal")
	}
	if !strings.Contains(stderr, "refuses") || !strings.Contains(stderr, "src/server.go") {
		t.Errorf("refusal must name overlapping path: %s", stderr)
	}
}

// TestRecordModes_UnstagedIncludesUntracked — PRD §7 bullet 7.
func TestRecordModes_UnstagedIncludesUntracked(t *testing.T) {
	tmp := modesFixture(t, "uun")
	// Plain untracked file, not in the index.
	modesWriteFile(t, tmp, "src/new.go", "brand new\n")

	_, stderr, code := runRecord(t, "record", "--path", tmp, "uun", "--unstaged", "--lenient")
	if code != 0 {
		t.Fatalf("record --unstaged failed: %s", stderr)
	}
	patch := readRecordedPatch(t, tmp, "uun")
	if !strings.Contains(patch, "src/new.go") {
		t.Errorf("--unstaged patch must include untracked file: %s", patch)
	}
}

// ─── PRD §7 bullets 8, 9, 10, 11: --claimed-only ────────────────────────────

// TestRecordModes_ClaimedOnlyRefusesNoClaims — PRD §7 bullet 8.
func TestRecordModes_ClaimedOnlyRefusesNoClaims(t *testing.T) {
	tmp := modesFixture(t, "noclaim")
	modesWriteFile(t, tmp, "src/x.go", "x\n")
	_, stderr, code := runRecord(t, "record", "--path", tmp, "noclaim", "--claimed-only", "--lenient")
	if code == 0 {
		t.Fatalf("expected --claimed-only refusal when no claims")
	}
	if !strings.Contains(stderr, "no active claims") {
		t.Errorf("refusal must mention no claims: %s", stderr)
	}
}

// TestRecordModes_ClaimedOnlyFiltersToClaimedPaths — PRD §7 bullet 9.
// `tpatch feature claim add` is exercised via the real CLI so we hit
// the alpha-1 surface (no direct claims.json writes).
func TestRecordModes_ClaimedOnlyFiltersToClaimedPaths(t *testing.T) {
	tmp := modesFixture(t, "cl")
	modesWriteFile(t, tmp, "src/keep.go", "keep\n")
	modesWriteFile(t, tmp, "src/drop.go", "drop\n")

	if _, _, code := runRecord(t, "feature", "claim", "add", "--path", tmp, "cl", "src/keep.go"); code != 0 {
		t.Fatalf("feature claim add failed")
	}

	_, stderr, code := runRecord(t, "record", "--path", tmp, "cl", "--claimed-only", "--lenient")
	if code != 0 {
		t.Fatalf("record --claimed-only failed: %s", stderr)
	}
	patch := readRecordedPatch(t, tmp, "cl")
	if !strings.Contains(patch, "src/keep.go") {
		t.Errorf("claimed patch must include src/keep.go: %s", patch)
	}
	if strings.Contains(patch, "src/drop.go") {
		t.Errorf("claimed patch must NOT include src/drop.go: %s", patch)
	}
	md := readRecordMD(t, tmp, "cl")
	if !strings.Contains(md, "claim_ids") {
		t.Errorf("record.md must list claim_ids when --claimed-only is used:\n%s", md)
	}
}

// TestRecordModes_ClaimedOnlyDirectoryClaim — directory claim
// (trailing slash) matches paths inside the directory.
func TestRecordModes_ClaimedOnlyDirectoryClaim(t *testing.T) {
	tmp := modesFixture(t, "cldir")
	if err := os.MkdirAll(filepath.Join(tmp, "src", "models"), 0o755); err != nil {
		t.Fatal(err)
	}
	modesWriteFile(t, tmp, "src/models/cat.go", "cat\n")
	modesWriteFile(t, tmp, "src/models/dog.go", "dog\n")
	modesWriteFile(t, tmp, "src/util.go", "util\n")

	if _, _, code := runRecord(t, "feature", "claim", "add", "--path", tmp, "cldir", "src/models"); code != 0 {
		t.Fatalf("feature claim add failed")
	}

	_, stderr, code := runRecord(t, "record", "--path", tmp, "cldir", "--claimed-only", "--lenient")
	if code != 0 {
		t.Fatalf("record --claimed-only failed: %s", stderr)
	}
	patch := readRecordedPatch(t, tmp, "cldir")
	if !strings.Contains(patch, "src/models/cat.go") || !strings.Contains(patch, "src/models/dog.go") {
		t.Errorf("directory claim must match both files under src/models/: %s", patch)
	}
	if strings.Contains(patch, "src/util.go") {
		t.Errorf("directory claim must NOT match src/util.go: %s", patch)
	}
}

// TestRecordModes_AutoClaimedOnlyIntersection — PRD §7 bullet 10.
// `--auto --claimed-only` runs the auto committed-range capture but
// narrows by claim paths.
func TestRecordModes_AutoClaimedOnlyIntersection(t *testing.T) {
	tmp := modesFixture(t, "auc")
	modesWriteFile(t, tmp, "keep.txt", "keep-v1\n")
	modesWriteFile(t, tmp, "drop.txt", "drop-v1\n")
	gitRun(t, tmp, "add", ".")
	gitRun(t, tmp, "commit", "-m", "seed")
	baseSha := gitOut(t, tmp, "rev-parse", "HEAD")
	writeUpstreamLock(t, tmp, "origin", "main", baseSha)
	modesWriteFile(t, tmp, "keep.txt", "keep-v2\n")
	modesWriteFile(t, tmp, "drop.txt", "drop-v2\n")
	gitRun(t, tmp, "add", ".")
	gitRun(t, tmp, "commit", "-m", "feature edits")

	if _, _, code := runRecord(t, "feature", "claim", "add", "--path", tmp, "auc", "keep.txt"); code != 0 {
		t.Fatalf("feature claim add failed")
	}

	_, stderr, code := runRecord(t, "record", "--path", tmp, "auc", "--auto", "--claimed-only", "--lenient")
	if code != 0 {
		t.Fatalf("record --auto --claimed-only failed: %s", stderr)
	}
	patch := readRecordedPatch(t, tmp, "auc")
	if !strings.Contains(patch, "keep.txt") {
		t.Errorf("claimed-auto patch must contain keep.txt: %s", patch)
	}
	if strings.Contains(patch, "drop.txt") {
		t.Errorf("claimed-auto patch must NOT contain drop.txt: %s", patch)
	}
	md := readRecordMD(t, tmp, "auc")
	if !strings.Contains(md, "auto-committed-range") {
		t.Errorf("record.md must declare auto-committed-range provenance:\n%s", md)
	}
}

// TestRecordModes_FilesAndClaimedOnlyEmptyIntersection — PRD §7 bullet 11.
func TestRecordModes_FilesAndClaimedOnlyEmptyIntersection(t *testing.T) {
	tmp := modesFixture(t, "ix")
	modesWriteFile(t, tmp, "src/keep.go", "keep\n")
	modesWriteFile(t, tmp, "src/other.go", "other\n")
	if _, _, code := runRecord(t, "feature", "claim", "add", "--path", tmp, "ix", "src/keep.go"); code != 0 {
		t.Fatalf("feature claim add failed")
	}

	_, stderr, code := runRecord(t, "record", "--path", tmp, "ix", "--claimed-only", "--files", "src/other.go", "--lenient")
	if code == 0 {
		t.Fatalf("expected empty-intersection refusal")
	}
	if !strings.Contains(stderr, "empty intersection") {
		t.Errorf("refusal must explain empty intersection: %s", stderr)
	}
}

// ─── PRD §7 bullet 13: provenance is written for every mode ────────────────

// TestRecordModes_ProvenanceWrittenForEachMode covers the per-mode
// provenance row of PRD §7 bullet 13. Each successful capture writes
// its expected normalized capture_mode label into record.md.
func TestRecordModes_ProvenanceWrittenForEachMode(t *testing.T) {
	type tc struct {
		name string
		mode string
		args []string
		seed func(t *testing.T, tmp string)
	}
	cases := []tc{
		{
			name: "default",
			mode: "working-tree-all",
			args: []string{},
			seed: func(t *testing.T, tmp string) { modesWriteFile(t, tmp, "x.go", "x\n") },
		},
		{
			name: "all",
			mode: "working-tree-all",
			args: []string{"--all"},
			seed: func(t *testing.T, tmp string) { modesWriteFile(t, tmp, "x.go", "x\n") },
		},
		{
			name: "staged",
			mode: "staged-index",
			args: []string{"--staged"},
			seed: func(t *testing.T, tmp string) {
				modesWriteFile(t, tmp, "x.go", "x\n")
				gitRun(t, tmp, "add", "x.go")
			},
		},
		{
			name: "unstaged",
			mode: "unstaged-worktree",
			args: []string{"--unstaged"},
			seed: func(t *testing.T, tmp string) { modesWriteFile(t, tmp, "x.go", "x\n") },
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			slug := "prov-" + c.name
			tmp := modesFixture(t, slug)
			c.seed(t, tmp)
			args := append([]string{"record", "--path", tmp, slug, "--lenient"}, c.args...)
			if _, stderr, code := runRecord(t, args...); code != 0 {
				t.Fatalf("record failed: %s", stderr)
			}
			md := readRecordMD(t, tmp, slug)
			if !strings.Contains(md, "**capture_mode**: `"+c.mode+"`") {
				t.Errorf("expected capture_mode=%s in record.md:\n%s", c.mode, md)
			}
			if !strings.Contains(md, "Capture Provenance") {
				t.Errorf("expected 'Capture Provenance' section in record.md:\n%s", md)
			}
		})
	}
}
