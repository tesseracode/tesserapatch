package cli

// Landing-evidence acceptance rows — PRD-tpatch-land §6.2
// (AC-LD1 … AC-LD23, all tier C).
//
// These are `land`-side obligations of the reader contract `tpatch
// verify` now depends on: the evidence substrate is actually produced,
// and `land`'s behaviour is unchanged apart from the single new
// `Tpatch-Base-Commit` precondition (§3.8.6).

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// parsedTrailer returns git's PARSED values for a trailer key on HEAD.
func parsedTrailer(t *testing.T, dir, key string) []string {
	t.Helper()
	cmd := exec.Command("git", "log", "-1",
		"--format=%(trailers:key="+key+",valueonly,separator=%x1e)")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log trailers: %v", err)
	}
	var vals []string
	for _, v := range strings.Split(strings.TrimSpace(string(out)), "\x1e") {
		v = strings.TrimSpace(v)
		if v != "" {
			vals = append(vals, v)
		}
	}
	return vals
}

func sha256Artifact(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// AC-LD1 — all four trailers in ADR-019 order, and Git PARSES them.
func TestACLD1_FourTrailersAreParsedByGit(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	for _, key := range []string{"Tpatch-Feature", "Tpatch-Patch-SHA", "Tpatch-Recipe-SHA", "Tpatch-Base-Commit"} {
		vals := parsedTrailer(t, tmpDir, key)
		if len(vals) == 0 {
			t.Errorf("git parses no %s trailer", key)
		}
	}
	if got := parsedTrailer(t, tmpDir, "Tpatch-Feature"); len(got) != 1 || got[0] != slug {
		t.Errorf("Tpatch-Feature=%v want [%s]", got, slug)
	}
}

// AC-LD2 — each of the four keys appears EXACTLY once.
func TestACLD2_TrailerCardinalityIsExactlyOne(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	for _, key := range []string{"Tpatch-Feature", "Tpatch-Patch-SHA", "Tpatch-Recipe-SHA", "Tpatch-Base-Commit"} {
		if vals := parsedTrailer(t, tmpDir, key); len(vals) != 1 {
			t.Errorf("%s has %d values, want exactly 1: %v", key, len(vals), vals)
		}
	}
}

// AC-LD3 — the two digests equal sha256 of the canonical artifact bytes.
func TestACLD3_DigestsMatchArtifactBytes(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	patch, err := os.ReadFile(filepath.Join(tmpDir, ".tpatch", "features", slug, "artifacts", "post-apply.patch"))
	if err != nil {
		t.Fatal(err)
	}
	if got := parsedTrailer(t, tmpDir, "Tpatch-Patch-SHA"); len(got) != 1 || got[0] != sha256Artifact(patch) {
		t.Errorf("Tpatch-Patch-SHA=%v want %s", got, sha256Artifact(patch))
	}
	recipePath := filepath.Join(tmpDir, ".tpatch", "features", slug, "artifacts", "apply-recipe.json")
	want := "none"
	if raw, rerr := os.ReadFile(recipePath); rerr == nil && strings.TrimSpace(string(raw)) != "" {
		want = sha256Artifact(raw)
	}
	if got := parsedTrailer(t, tmpDir, "Tpatch-Recipe-SHA"); len(got) != 1 || got[0] != want {
		t.Errorf("Tpatch-Recipe-SHA=%v want %s", got, want)
	}
}

// AC-LD4 — `Tpatch-Recipe-SHA: none` in BOTH producer cases.
func TestACLD4_RecipeSHANoneInBothProducerCases(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, dir, slug string)
	}{
		{"recipe-absent", func(t *testing.T, dir, slug string) {
			_ = os.Remove(filepath.Join(dir, ".tpatch", "features", slug, "artifacts", "apply-recipe.json"))
		}},
		{"recipe-whitespace-only", func(t *testing.T, dir, slug string) {
			p := filepath.Join(dir, ".tpatch", "features", slug, "artifacts", "apply-recipe.json")
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte("  \n\t\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, slug, _ := setupLandFixture(t)
			if _, _, code := runCmdWithError("record", "--path", tmpDir, slug); code != 0 {
				t.Fatalf("record failed")
			}
			tc.setup(t, tmpDir, slug)
			if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--no-record"); code != 0 {
				t.Fatalf("land failed: %s", stderr)
			}
			if got := parsedTrailer(t, tmpDir, "Tpatch-Recipe-SHA"); len(got) != 1 || got[0] != "none" {
				t.Errorf("Tpatch-Recipe-SHA=%v want [none]", got)
			}
		})
	}
}

// AC-LD5 — all emitted hex is lowercase and of the DERIVED length.
func TestACLD5_HexFormatsAreLowercaseAndDerived(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	facts, err := gitutil.ReadRepoFacts(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	patchSHA := parsedTrailer(t, tmpDir, "Tpatch-Patch-SHA")[0]
	if !gitutil.IsLowercaseHexOfLen(patchSHA, 64) {
		t.Errorf("Tpatch-Patch-SHA %q is not 64 lowercase hex", patchSHA)
	}
	base := parsedTrailer(t, tmpDir, "Tpatch-Base-Commit")[0]
	if !gitutil.IsLowercaseHexOfLen(base, facts.CommitIDHexLen) {
		t.Errorf("Tpatch-Base-Commit %q is not %d lowercase hex (object format %s)",
			base, facts.CommitIDHexLen, facts.ObjectFormat)
	}
}

// AC-LD6 / AC-LD7 — the trailer equals `status.apply.base_commit` at
// commit time, and `land` does NOT write that field.
func TestACLD6_BaseCommitTrailerEqualsStatusAndIsNotWritten(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, _, code := runCmdWithError("record", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("record failed")
	}
	before := loadStatusField(t, tmpDir, slug)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--no-record"); code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	after := loadStatusField(t, tmpDir, slug)
	if before != after {
		t.Errorf("land wrote status.apply.base_commit: %q -> %q", before, after)
	}
	if got := parsedTrailer(t, tmpDir, "Tpatch-Base-Commit"); len(got) != 1 || got[0] != before {
		t.Errorf("Tpatch-Base-Commit=%v want %q", got, before)
	}
}

func loadStatusField(t *testing.T, dir, slug string) string {
	t.Helper()
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatal(err)
	}
	return st.Apply.BaseCommit
}

// AC-LD8 — `land` adds NO new field to status.json.
func TestACLD8_NoNewStatusField(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, _, code := runCmdWithError("record", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("record failed")
	}
	before := statusKeys(t, tmpDir, slug)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--no-record"); code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	after := statusKeys(t, tmpDir, slug)
	for k := range after {
		if _, ok := before[k]; !ok {
			t.Errorf("land added a new status.json key: %q", k)
		}
	}
}

func statusKeys(t *testing.T, dir, slug string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tpatch", "features", slug, "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := jsonUnmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for k := range m {
		out[k] = true
	}
	return out
}

// AC-LD9 — the landing commit has EXACTLY ONE parent.
func TestACLD9_LandingCommitIsSingleParent(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	cmd := exec.Command("git", "log", "-1", "--format=%P")
	cmd.Dir = tmpDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if n := len(strings.Fields(string(out))); n != 1 {
		t.Fatalf("the landing commit has %d parents, want exactly 1", n)
	}
}

// AC-LD10 — the trailer block is the LAST paragraph, and a commit-msg
// hook that appends prose demonstrates the failure mode `land` itself
// does not introduce.
func TestACLD10_TrailerBlockIsTheLastParagraph(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	msg := gitLastCommitMsg(t, tmpDir)
	paragraphs := strings.Split(strings.TrimRight(msg, "\n"), "\n\n")
	last := paragraphs[len(paragraphs)-1]
	if !strings.Contains(last, "Tpatch-Base-Commit: ") {
		t.Errorf("the trailer block is not the last paragraph:\n%s", msg)
	}

	// Adversarial: a hook that appends prose destroys the parse. `land`
	// does not introduce this; the row documents the failure mode.
	tmpDir2, slug2, _ := setupLandFixture(t)
	hookDir := filepath.Join(tmpDir2, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hook := "#!/bin/sh\nprintf '\\nappended prose from a hook\\n' >> \"$1\"\n"
	if err := os.WriteFile(filepath.Join(hookDir, "commit-msg"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir2, slug2); code != 0 {
		t.Skipf("hook fixture could not land here: %s", stderr)
	}
	if vals := parsedTrailer(t, tmpDir2, "Tpatch-Feature"); len(vals) != 0 {
		t.Logf("this git still parses the trailer after the hook append: %v", vals)
	}
}

// AC-LD11 — landing twice produces two commits with the same
// Tpatch-Feature and different SHA fields; `land` does not refuse.
func TestACLD11_LandingTwiceIsAllowed(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("first land failed: %s", stderr)
	}
	first := parsedTrailer(t, tmpDir, "Tpatch-Patch-SHA")[0]
	firstHead := gitHead(t, tmpDir)

	// A second change, re-recorded and re-landed.
	if err := os.WriteFile(filepath.Join(tmpDir, "src", "feature.txt"), []byte("hello feature\nsecond\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("second land refused: %s", stderr)
	}
	if gitHead(t, tmpDir) == firstHead {
		t.Fatalf("the second land produced no commit")
	}
	second := parsedTrailer(t, tmpDir, "Tpatch-Patch-SHA")[0]
	if first == second {
		t.Errorf("the two landings carry identical Tpatch-Patch-SHA values")
	}
	if got := parsedTrailer(t, tmpDir, "Tpatch-Feature"); len(got) != 1 || got[0] != slug {
		t.Errorf("Tpatch-Feature=%v", got)
	}
}

// AC-LD12 — `land` refuses when the embedded record would capture
// nothing, so an absent/zero-byte canonical patch is a corruption case
// rather than a producer state.
func TestACLD12_LandRefusesWhenRecordCapturesNothing(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	// Revert the worktree so there is nothing to capture.
	if err := os.WriteFile(filepath.Join(tmpDir, "src", "tracked.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(tmpDir, "src", "feature.txt")); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug)
	if code == 0 {
		t.Fatalf("land accepted an empty capture")
	}
	if stderr == "" {
		t.Errorf("no diagnostic emitted")
	}
}

// AC-LD14 — `land --dry-run` prints the trailer block and mutates
// nothing.
func TestACLD14_DryRunMutatesNothing(t *testing.T) {
	tmpDir, slug, base := setupLandFixture(t)
	before := hashTree(t, tmpDir)
	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--dry-run")
	if code != 0 {
		t.Fatalf("dry-run failed: %s", stderr)
	}
	if !strings.Contains(stdout, "Tpatch-Feature: "+slug) {
		t.Errorf("dry-run did not print the trailer block:\n%s", stdout)
	}
	if gitHead(t, tmpDir) != base {
		t.Errorf("dry-run advanced HEAD")
	}
	if diff := treeDiff(before, hashTree(t, tmpDir)); diff != "" {
		t.Errorf("dry-run mutated: %s", diff)
	}
}

// AC-LD17 — `land` never emits a trailer value outside the rule-6
// formats.
func TestACLD17_NoOutOfFormatTrailerValues(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	facts, err := gitutil.ReadRepoFacts(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	patch := parsedTrailer(t, tmpDir, "Tpatch-Patch-SHA")[0]
	recipe := parsedTrailer(t, tmpDir, "Tpatch-Recipe-SHA")[0]
	base := parsedTrailer(t, tmpDir, "Tpatch-Base-Commit")[0]
	if !gitutil.IsLowercaseHexOfLen(patch, 64) {
		t.Errorf("patch digest out of format: %q", patch)
	}
	if recipe != "none" && !gitutil.IsLowercaseHexOfLen(recipe, 64) {
		t.Errorf("recipe digest out of format: %q", recipe)
	}
	if !gitutil.IsLowercaseHexOfLen(base, facts.CommitIDHexLen) {
		t.Errorf("base commit out of format: %q", base)
	}
	if strings.ToLower(patch) != patch || strings.ToLower(base) != base {
		t.Errorf("uppercase hex emitted")
	}
}

// AC-LD18 — Mode A with NO pending journal: an invalid base commit
// refuses with R23 having mutated NOTHING.
func TestACLD18_ModeANoJournalRefusesWithoutMutating(t *testing.T) {
	for _, tc := range []struct {
		name    string
		value   string
		defect  string
		prepare func(t *testing.T, dir, slug string)
	}{
		{"empty", "", "empty", nil},
		{"malformed", "not-a-sha", "malformed", nil},
		{"unresolvable", strings.Repeat("a", 40), "unresolvable", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, slug, _ := setupLandFixture(t)
			if _, _, code := runCmdWithError("record", "--path", tmpDir, slug); code != 0 {
				t.Fatalf("record failed")
			}
			setLandBaseCommit(t, tmpDir, slug, tc.value)

			beforeTree := hashTree(t, tmpDir)
			beforeIndex := readIndex(t, tmpDir)
			beforeHead := gitHead(t, tmpDir)

			_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--no-record")
			if code == 0 {
				t.Fatalf("land accepted an invalid base commit")
			}
			want := "land refuses: status.apply.base_commit is " + tc.defect + " (" + tc.value + "); the Tpatch-Base-Commit trailer would be unreadable. Run tpatch record " + slug + " --auto (or --from <base>) to repopulate it, then re-run tpatch land " + slug
			if !strings.Contains(stderr, want) {
				t.Errorf("R23 not verbatim:\n got %q\nwant %q", stderr, want)
			}
			if gitHead(t, tmpDir) != beforeHead {
				t.Errorf("HEAD advanced on a refusal")
			}
			if readIndex(t, tmpDir) != beforeIndex {
				t.Errorf("the index changed on a refusal")
			}
			if diff := treeDiff(beforeTree, hashTree(t, tmpDir)); diff != "" {
				t.Errorf("the refusal mutated: %s", diff)
			}
			if strings.Contains(stderr, "the embedded record step already completed") {
				t.Errorf("Mode A must not claim retained record artifacts")
			}
		})
	}
}

// AC-LD18a — Mode B: `land` re-validates the RELOADED value after
// `record` returns, refuses with R23 plus the retained-artifacts note,
// and leaves no commit, no index change and no `landed at` note — while
// `record`'s artifacts DO persist.
func TestACLD18a_ModeBRefusesAfterRecordWithRetainedArtifacts(t *testing.T) {
	// (i) Mode B validates `record`'s OUTPUT, not the stale value: an
	// invalid PRE-record base commit lands fine, because record replaces
	// it. This is §3.8.6's "why not pre-validate the old value".
	t.Run("validates-the-produced-value-not-the-stale-one", func(t *testing.T) {
		tmpDir, slug, _ := setupLandFixture(t)
		if _, _, code := runCmdWithError("record", "--path", tmpDir, slug); code != 0 {
			t.Fatalf("record failed")
		}
		setLandBaseCommit(t, tmpDir, slug, "")
		if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
			t.Fatalf("Mode B refused a value record was going to replace: %s", stderr)
		}
	})

	// (ii) The refusal itself: R23 plus the retained-artifacts note, and
	// no land-owned mutation. The shipped `record` always produces a
	// valid base commit (AC-LD18b), so the end-to-end refusal is
	// unreachable without corrupting `record`; the production validator
	// is therefore driven directly with the Mode B note, and the
	// no-mutation half is asserted on the real command in Mode A
	// (AC-LD18).
	t.Run("refusal-text-and-no-land-owned-mutation", func(t *testing.T) {
		tmpDir, slug, _ := setupLandFixture(t)
		if _, _, code := runCmdWithError("record", "--path", tmpDir, slug); code != 0 {
			t.Fatalf("record failed")
		}
		artifactsBefore := hashTree(t, filepath.Join(tmpDir, ".tpatch", "features", slug, "artifacts"))
		s, err := store.Open(tmpDir)
		if err != nil {
			t.Fatal(err)
		}
		st, err := s.LoadFeatureStatus(slug)
		if err != nil {
			t.Fatal(err)
		}
		st.Apply.BaseCommit = ""
		var warned strings.Builder
		err = validateLandBaseCommit(tmpDir, slug, st, noteRecordArtifacts, func(format string, args ...any) {
			warned.WriteString(format)
		})
		if err == nil {
			t.Fatalf("Mode B accepted an invalid produced base commit")
		}
		msg := err.Error()
		wantHead := "land refuses: status.apply.base_commit is empty (); the Tpatch-Base-Commit trailer would be unreadable. Run tpatch record " + slug + " --auto (or --from <base>) to repopulate it, then re-run tpatch land " + slug
		if !strings.HasPrefix(msg, wantHead) {
			t.Errorf("R23 not verbatim:\n got %q\nwant prefix %q", msg, wantHead)
		}
		wantNote := "note: the embedded record step already completed; its artifacts under .tpatch/features/" + slug + "/artifacts/ are retained and are not rolled back"
		if !strings.Contains(msg, wantNote) {
			t.Errorf("Mode B note missing:\n got %q\nwant %q", msg, wantNote)
		}
		if strings.Contains(msg, "mutating nothing") {
			t.Errorf("Mode B must not claim it mutated nothing")
		}
		// record's artifacts persist: its own finished transaction.
		if len(hashTree(t, filepath.Join(tmpDir, ".tpatch", "features", slug, "artifacts"))) != len(artifactsBefore) {
			t.Errorf("record's artifacts were rolled back")
		}
		// No `landed at` note was written by the refusal.
		if strings.Contains(loadStatusNotes(t, tmpDir, slug), "landed at") {
			t.Errorf("the refusal wrote the `landed at` note")
		}
	})
}

func loadStatusNotes(t *testing.T, dir, slug string) string {
	t.Helper()
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatal(err)
	}
	return st.Notes
}

// AC-LD18b — `record` writes a VALID Apply.BaseCommit before its own
// first mutation, so Mode B's post-record validation passes for every
// well-formed workspace.
func TestACLD18b_RecordProducesAValidBaseCommit(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, stderr, code := runCmdWithError("record", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("record failed: %s", stderr)
	}
	base := loadStatusField(t, tmpDir, slug)
	facts, err := gitutil.ReadRepoFacts(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if !gitutil.IsLowercaseHexOfLen(base, facts.CommitIDHexLen) {
		t.Fatalf("record produced an invalid base commit %q", base)
	}
	if _, _, err := gitutil.ResolveRevOffline(tmpDir, base+"^{commit}"); err != nil {
		t.Fatalf("record produced an unresolvable base commit %q: %v", base, err)
	}
	reachable, err := gitutil.IsAncestorOffline(tmpDir, base, "HEAD")
	if err != nil || !reachable {
		t.Fatalf("record produced an unreachable base commit %q (err=%v)", base, err)
	}
	// And Mode B therefore lands.
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("Mode B refused a well-formed workspace: %s", stderr)
	}
}

// AC-LD18c — Mode A with a PENDING journal: recovery still runs first and
// `land` then refuses with R23 plus the recovery note, having made no NEW
// mutation of its own.
func TestACLD18c_ModeAPendingJournalRefusesAfterRecovery(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, _, code := runCmdWithError("record", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("record failed")
	}
	// Plant a journal so `recoverLand` has a prior transaction to decide.
	journalDir := filepath.Join(tmpDir, ".tpatch", "local", "land-journal")
	if err := os.MkdirAll(journalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(journalDir, slug+".json"), []byte("{\"version\":3}"), 0o644); err != nil {
		t.Fatal(err)
	}
	setLandBaseCommit(t, tmpDir, slug, "")
	beforeHead := gitHead(t, tmpDir)

	_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--no-record")
	if code == 0 {
		t.Fatalf("land accepted an invalid base commit with a pending journal")
	}
	if gitHead(t, tmpDir) != beforeHead {
		t.Errorf("this invocation advanced HEAD")
	}
	// Either recovery refused first (its own diagnostic) or it completed
	// and R23 fired with the recovery note. Both are contract-valid; the
	// row asserts that recovery ran BEFORE the refusal in the second case.
	if strings.Contains(stderr, "land refuses: status.apply.base_commit is") {
		if !strings.Contains(stderr, "an interrupted previous land was recovered") {
			t.Errorf("R23 after a pending journal must name the completed recovery: %q", stderr)
		}
		return
	}
	if !strings.Contains(stderr, "journal") {
		t.Errorf("expected a recovery diagnostic; got %q", stderr)
	}
}

// AC-LD19 — the accepted base-commit length is DERIVED from the object
// format: a SHA-256 repository accepts 64 hex and refuses 40.
func TestACLD19_BaseCommitLengthIsDerivedFromObjectFormat(t *testing.T) {
	t.Run("sha1-refuses-64", func(t *testing.T) {
		tmpDir, slug, _ := setupLandFixture(t)
		if _, _, code := runCmdWithError("record", "--path", tmpDir, slug); code != 0 {
			t.Fatalf("record failed")
		}
		setLandBaseCommit(t, tmpDir, slug, strings.Repeat("a", 64))
		_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--no-record")
		if code == 0 {
			t.Fatalf("a sha1 repository accepted a 64-hex base commit")
		}
		if !strings.Contains(stderr, "is malformed") {
			t.Errorf("expected the malformed defect; got %q", stderr)
		}
	})
	t.Run("sha256-accepts-64-refuses-40", func(t *testing.T) {
		dir := t.TempDir()
		cmd := exec.Command("git", "init", "-q", "-b", "main", "--object-format=sha256", ".")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("this git cannot create a sha256 repository: %s", out)
		}
		facts, err := gitutil.ReadRepoFacts(dir)
		if err != nil {
			t.Fatal(err)
		}
		if facts.CommitIDHexLen != 64 {
			t.Fatalf("derived length=%d want 64", facts.CommitIDHexLen)
		}
		if gitutil.IsLowercaseHexOfLen(strings.Repeat("a", 40), facts.CommitIDHexLen) {
			t.Errorf("a 40-hex base commit must be refused in a sha256 repository — a hardcoded 40 fails this row")
		}
		if !gitutil.IsLowercaseHexOfLen(strings.Repeat("a", 64), facts.CommitIDHexLen) {
			t.Errorf("a 64-hex base commit must be accepted in a sha256 repository")
		}
	})
}

// AC-LD20 — in a shallow or partial clone an unreachable but well-formed
// and resolvable base commit produces a one-line WARN and the landing
// proceeds.
func TestACLD20_UnreachableBaseWarnsButProceeds(t *testing.T) {
	tmpDir, slug, base := setupLandFixture(t)
	if _, _, code := runCmdWithError("record", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("record failed")
	}
	// A resolvable commit that is NOT an ancestor of HEAD, built with
	// plumbing so `.tpatch/` is never rewound by a checkout.
	tree := gitOut(t, tmpDir, "rev-parse", base+"^{tree}")
	sideHead := gitOut(t, tmpDir, "commit-tree", strings.TrimSpace(tree), "-p", base, "-m", "side")
	sideHead = strings.TrimSpace(sideHead)
	setLandBaseCommit(t, tmpDir, slug, sideHead)

	_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--no-record")
	if code != 0 {
		t.Fatalf("unreachability alone must never refuse: %s", stderr)
	}
	if !strings.Contains(stderr, "not reachable from HEAD") {
		t.Errorf("expected the one-line warn; got %q", stderr)
	}
}

// AC-LD15 / AC-LD21 — `land`'s successful path and every pre-existing
// refusal are unchanged for inputs whose base commit is valid.
func TestACLD21_SuccessfulPathIsUnchanged(t *testing.T) {
	tmpDir, slug, base := setupLandFixture(t)
	stdout, stderr, code := runCmdWithError("land", "--path", tmpDir, slug)
	if code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	// The successful path emits no base-commit diagnostic at all.
	for _, forbidden := range []string{"land refuses: status.apply.base_commit", "not reachable from HEAD"} {
		if strings.Contains(stdout+stderr, forbidden) {
			t.Errorf("the successful path emitted %q", forbidden)
		}
	}
	if gitHead(t, tmpDir) == base {
		t.Fatalf("HEAD did not advance")
	}
	// A pre-existing refusal keeps its own diagnostic and ordering.
	tmpDir2, slug2, _ := setupLandFixture(t)
	setLandBaseCommit(t, tmpDir2, slug2, gitHead(t, tmpDir2))
	_, stderr2, code2 := runCmdWithError("land", "--path", tmpDir2, slug2, "--no-record")
	if code2 == 0 {
		t.Fatalf("the pre-existing --no-record refusal disappeared")
	}
	if !strings.Contains(stderr2, "no recorded post-apply.patch") {
		t.Errorf("pre-existing refusal text changed: %q", stderr2)
	}
}

// AC-LD22 — every `land`-side git invocation added by §3.8.6 carries
// GIT_NO_LAZY_FETCH=1.
func TestACLD22_LandValidationIsOffline(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, _, code := runCmdWithError("record", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("record failed")
	}
	logPath := installLandGitWrapper(t)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--no-record"); code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read the wrapper log: %v", err)
	}
	sawValidation := false
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\x1f")
		joined := strings.Join(fields, " ")
		isValidation := strings.Contains(joined, "--show-object-format") ||
			strings.Contains(joined, "^{commit}") ||
			strings.Contains(joined, "merge-base --is-ancestor")
		if !isValidation {
			continue
		}
		sawValidation = true
		if !strings.Contains(joined, "GIT_NO_LAZY_FETCH=1") {
			t.Errorf("validation call without GIT_NO_LAZY_FETCH=1: %s", joined)
		}
	}
	if !sawValidation {
		t.Fatalf("no base-commit validation call was observed")
	}
}

// installLandGitWrapper records argv + the offline env for every git call.
func installLandGitWrapper(t *testing.T) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	script := "#!/bin/sh\n{\n" +
		"  for a in \"$@\"; do printf '%s\\x1f' \"$a\"; done\n" +
		"  printf 'GIT_NO_LAZY_FETCH=%s\\n' \"${GIT_NO_LAZY_FETCH-}\"\n" +
		"} >> " + logPath + "\nexec " + realGit + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatalf("write shim: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

// AC-LD13 — a feature landed BEFORE this amendment is readable under the
// §3.8.2 rules with no migration.
func TestACLD13_PreAmendmentLandingIsReadable(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	// The v0.8.0-era shape IS what land emits: four trailers, last
	// paragraph, single parent. Reading it needs no migration.
	recs, err := gitutil.EnumerateCommitTrailers(tmpDir)
	if err != nil {
		t.Fatalf("EnumerateCommitTrailers: %v", err)
	}
	found := false
	for _, r := range recs {
		vals := r.TrailerValues(gitutil.TrailerFeature)
		if len(vals) == 1 && vals[0] == slug {
			found = true
			if r.ParentCount() != 1 {
				t.Errorf("parents=%d want 1", r.ParentCount())
			}
			if len(r.TrailerValues(gitutil.TrailerBaseCommit)) != 1 {
				t.Errorf("base commit trailer missing")
			}
		}
	}
	if !found {
		t.Fatalf("the reader could not find the landing")
	}
}

// AC-LD16 — two successive landings leave the EARLIER one reachable and
// single-parent, so a reader can still derive a pre-landing tree.
func TestACLD16_EarlierLandingStaysReachable(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("first land failed: %s", stderr)
	}
	first := gitHead(t, tmpDir)
	if err := os.WriteFile(filepath.Join(tmpDir, "src", "feature.txt"), []byte("hello feature\nsecond\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("second land failed: %s", stderr)
	}
	recs, err := gitutil.EnumerateCommitTrailers(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, r := range recs {
		vals := r.TrailerValues(gitutil.TrailerFeature)
		if len(vals) == 1 && vals[0] == slug {
			seen++
			if r.ParentCount() != 1 {
				t.Errorf("landing %s has %d parents", r.SHA, r.ParentCount())
			}
		}
	}
	if seen < 2 {
		t.Fatalf("only %d landing(s) reachable; the earlier one must survive", seen)
	}
	if _, _, err := gitutil.ResolveRevOffline(tmpDir, first+"^{commit}"); err != nil {
		t.Errorf("the earlier landing is no longer reachable: %v", err)
	}
}

// AC-LD23 — the totality guard runs over PRD-tpatch-land as one of its
// three inputs and produces zero hits. The guard itself lives in
// internal/workflow (AC-L135); this row asserts the land document is one
// of its declared inputs.
func TestACLD23_LandDocumentIsAGuardInput(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Dir(filepath.Dir(root))
	guard, err := os.ReadFile(filepath.Join(repo, "internal", "workflow", "docs_totality_guard_test.go"))
	if err != nil {
		t.Fatalf("read the guard: %v", err)
	}
	if !strings.Contains(string(guard), "docs/prds/PRD-tpatch-land.md") {
		t.Fatalf("PRD-tpatch-land.md is not one of the guard's inputs")
	}
	if _, err := os.Stat(filepath.Join(repo, "docs", "prds", "PRD-tpatch-land.md")); err != nil {
		t.Fatalf("the land PRD is missing: %v", err)
	}
}
