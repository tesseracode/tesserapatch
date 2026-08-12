package cli

// Rev-1 fold regression suite — v0.15.1 Wave C, adjudication finding 6:
// `land` validated a TRIMMED `status.apply.base_commit` and then emitted
// the ORIGINAL value, so a base commit carrying a leading space, a
// trailing tab or a newline passed validation and still produced a
// `Tpatch-Base-Commit` trailer the §3.8.2 reader must reject.
//
// The field is now validated verbatim: any whitespace and any
// non-canonical spelling is malformed, nothing is normalised, and the
// trailer carries the exact validated string.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// TestRev1Land_NonCanonicalBaseCommitIsRefused pins every non-canonical
// spelling as a refusal rather than a silently-normalised acceptance.
func TestRev1Land_NonCanonicalBaseCommitIsRefused(t *testing.T) {
	cases := []struct {
		name   string
		mangle func(valid string) string
	}{
		{"leading-space", func(v string) string { return " " + v }},
		{"trailing-space", func(v string) string { return v + " " }},
		{"trailing-newline", func(v string) string { return v + "\n" }},
		{"leading-tab", func(v string) string { return "\t" + v }},
		{"trailing-tab", func(v string) string { return v + "\t" }},
		{"internal-space", func(v string) string { return v[:10] + " " + v[11:] }},
		{"uppercase", func(v string) string { return strings.ToUpper(v) }},
		{"mixed-case", func(v string) string { return strings.ToUpper(v[:8]) + v[8:] }},
		{"too-short", func(v string) string { return v[:39] }},
		{"too-long", func(v string) string { return v + "a" }},
		{"whitespace-only", func(v string) string { return "   " }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, slug, _ := setupLandFixture(t)
			if _, _, code := runCmdWithError("record", "--path", tmpDir, slug); code != 0 {
				t.Fatalf("record failed")
			}
			valid := loadStatusField(t, tmpDir, slug)
			if len(valid) != 40 {
				t.Fatalf("fixture base commit is not 40 hex: %q", valid)
			}
			mangled := tc.mangle(valid)
			setLandBaseCommit(t, tmpDir, slug, mangled)

			beforeHead := gitHead(t, tmpDir)
			_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--no-record")
			if code == 0 {
				t.Fatalf("land accepted a non-canonical base commit %q", mangled)
			}
			if !strings.Contains(stderr, "land refuses: status.apply.base_commit is") {
				t.Errorf("R23 not emitted: %q", stderr)
			}
			// The refusal quotes the STORED bytes, not a trimmed form.
			if !strings.Contains(stderr, "("+mangled+")") {
				t.Errorf("the refusal must quote the stored value %q verbatim; got %q", mangled, stderr)
			}
			if gitHead(t, tmpDir) != beforeHead {
				t.Fatalf("the refusal advanced HEAD")
			}
			// And nothing was ever emitted as a trailer.
			if msg := gitLastCommitMsg(t, tmpDir); strings.Contains(msg, "Tpatch-Base-Commit: "+mangled) {
				t.Fatalf("a non-canonical base commit reached the trailer block")
			}
		})
	}
}

// TestRev1Land_TrailerCarriesTheValidatedValue asserts the emitted
// trailer is byte-identical to the validated field — the half rev-0 got
// wrong by validating one string and emitting another.
func TestRev1Land_TrailerCarriesTheValidatedValue(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, _, code := runCmdWithError("record", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("record failed")
	}
	stored := loadStatusField(t, tmpDir, slug)

	s, err := store.Open(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		t.Fatal(err)
	}
	validated, vErr := validateLandBaseCommit(tmpDir, slug, st, noteNone, nil)
	if vErr != nil {
		t.Fatalf("a well-formed base commit must validate: %v", vErr)
	}
	if validated != stored {
		t.Fatalf("validated=%q stored=%q — validation must not normalise", validated, stored)
	}

	if _, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--no-record"); code != 0 {
		t.Fatalf("land failed: %s", stderr)
	}
	got := parsedTrailer(t, tmpDir, "Tpatch-Base-Commit")
	if len(got) != 1 || got[0] != validated {
		t.Fatalf("Tpatch-Base-Commit=%v want exactly [%s]", got, validated)
	}
	// Byte-level: the raw message must carry the value with no padding.
	if !strings.Contains(gitLastCommitMsg(t, tmpDir), "Tpatch-Base-Commit: "+validated+"\n") {
		t.Errorf("the raw trailer line is not the exact validated string")
	}
}

// TestRev1Land_ObjectFormatLengthIsDerived re-pins that the accepted
// length comes from the repository object format, now against the
// exactness rule (a 64-hex value in a sha1 repo is malformed even though
// it is lowercase hex).
func TestRev1Land_ObjectFormatLengthIsDerived(t *testing.T) {
	tmpDir, slug, _ := setupLandFixture(t)
	if _, _, code := runCmdWithError("record", "--path", tmpDir, slug); code != 0 {
		t.Fatalf("record failed")
	}
	facts, err := gitutil.ReadRepoFacts(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if facts.CommitIDHexLen != 40 {
		t.Skipf("fixture repo is not sha1 (len=%d)", facts.CommitIDHexLen)
	}
	setLandBaseCommit(t, tmpDir, slug, strings.Repeat("a", 64))
	_, stderr, code := runCmdWithError("land", "--path", tmpDir, slug, "--no-record")
	if code == 0 {
		t.Fatalf("a 64-hex base commit was accepted in a sha1 repository")
	}
	if !strings.Contains(stderr, "is malformed") {
		t.Errorf("expected the malformed defect; got %q", stderr)
	}
}

// TestRev1Land_ValidationIsOfflineAndFloorAware re-asserts AC-LD22 over
// the rev-1 code path and additionally proves the validator never falls
// back to a non-offline git invocation.
func TestRev1Land_ValidationIsOfflineAndFloorAware(t *testing.T) {
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
	seen := 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		joined := strings.Join(strings.Split(line, "\x1f"), " ")
		if !strings.Contains(joined, "--show-object-format") &&
			!strings.Contains(joined, "^{commit}") &&
			!strings.Contains(joined, "merge-base --is-ancestor") {
			continue
		}
		seen++
		if !strings.Contains(joined, "GIT_NO_LAZY_FETCH=1") {
			t.Errorf("validation call without GIT_NO_LAZY_FETCH=1: %s", joined)
		}
	}
	if seen == 0 {
		t.Fatalf("no base-commit validation call was observed")
	}
	_ = filepath.Base(logPath)
}
