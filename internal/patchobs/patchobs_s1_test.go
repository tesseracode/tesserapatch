package patchobs

// GH #15 / ADR-036 slice S1 — the immutable observation.
//
// Rows here cover PRD-recipe-generation-authority §6.2 and the RGA-100,
// RGA-104..RGA-112, RGA-117..RGA-119 matrix entries that S1 can establish
// without the coverage wire format a later slice owns:
//
//   - the capture-mode → reference-kind map, including the two shapes
//     that must NOT be `commit`;
//   - per-side observation flags: "did the producer look" is recorded
//     separately from "what did it find";
//   - the contradictory-observation rule — an extant side is never
//     recorded as observed-and-absent;
//   - object/content kind resolution from observed data, with headers
//     corroborating and refusing contradictions;
//   - the deterministic preimage-set digest.
//
// Every row that needs a repository builds one with real `git`, because
// the observation reads real trees. No network is used.

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

func obsGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %s: %v", args, dir, out, err)
	}
	return string(out)
}

// obsRepo builds a committed repository with one regular file.
func obsRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	obsGit(t, dir, "init", "-q", "-b", "main", ".")
	obsGit(t, dir, "config", "user.email", "t@example.com")
	obsGit(t, dir, "config", "user.name", "T")
	obsGit(t, dir, "config", "commit.gpgsign", "false")
	obsWrite(t, dir, "kept.txt", "v1\n", 0o644)
	obsGit(t, dir, "add", "-A")
	obsGit(t, dir, "commit", "-qm", "seed")
	return dir
}

func obsWrite(t *testing.T, dir, rel, body string, mode os.FileMode) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(full, mode); err != nil {
		t.Fatal(err)
	}
}

func obsHead(t *testing.T, dir string) string {
	t.Helper()
	commit, err := gitutil.HeadCommit(dir)
	if err != nil {
		t.Fatalf("HeadCommit: %v", err)
	}
	return strings.TrimSpace(commit)
}

// TestS1CaptureModeReferenceMap covers RGA-112 and the ADR-036 D2 table:
// every shipped record capture mode maps to `commit`, the two no-capture
// producers map to `unavailable`, and no shipped mode maps to
// `index-snapshot`.
func TestS1CaptureModeReferenceMap(t *testing.T) {
	commitModes := []CaptureMode{
		CaptureModeWorkingTreeAll,
		CaptureModeStagedIndex,
		CaptureModeUnstagedWorktree,
		CaptureModeCommittedRange,
		CaptureModeAutoCommittedRange,
		CaptureModeExplicitCommittedRange,
		CaptureModeReconcile,
	}
	for _, mode := range commitModes {
		if got := ReferenceKindForMode(mode); got != ReferenceKindCommit {
			t.Errorf("%s maps to %q, want commit", mode, got)
		}
		if !KnownCaptureMode(mode) {
			t.Errorf("%s is not a known capture mode", mode)
		}
	}
	if got := ReferenceKindForMode(CaptureModeNoCapture); got != ReferenceKindUnavailable {
		t.Errorf("no-capture maps to %q, want unavailable", got)
	}
	if !KnownCaptureMode(CaptureModeNoCapture) {
		t.Error("no-capture is not a known capture mode")
	}
	// `unstaged-worktree` is commit-kind HEAD, not an index snapshot.
	if got := ReferenceKindForMode(CaptureModeUnstagedWorktree); got == ReferenceKindIndexSnapshot {
		t.Error("unstaged-worktree is mislabelled index-snapshot")
	}
	// An unmapped mode is a bug, not a default.
	if KnownCaptureMode(CaptureMode("invented-mode")) {
		t.Error("an unmapped capture mode must not be accepted as known")
	}
	if got := ReferenceKindForMode(CaptureMode("invented-mode")); got != ReferenceKindUnavailable {
		t.Errorf("an unmapped capture mode resolved to %q, want unavailable", got)
	}
}

// TestS1NoCaptureCarriesEmptyPathspecs covers RGA-109's shape rule: a
// `no-capture` record carries empty pathspecs and claim IDs, and it
// licenses nothing.
func TestS1NoCaptureCarriesEmptyPathspecs(t *testing.T) {
	obs := Observe(Input{
		Producer: ProducerImplement,
		RepoRoot: t.TempDir(),
		Slug:     "demo",
		Capture: CaptureDescriptor{
			Mode:      CaptureModeNoCapture,
			Pathspecs: []string{"src/"},
			ClaimIDs:  []string{"claim-1"},
		},
		PreimageRef: "HEAD",
	})
	if len(obs.Capture.Pathspecs) != 0 || len(obs.Capture.ClaimIDs) != 0 {
		t.Fatalf("no-capture kept pathspecs=%v claim_ids=%v; both must be empty",
			obs.Capture.Pathspecs, obs.Capture.ClaimIDs)
	}
	if obs.Capture.Pathspecs == nil || obs.Capture.ClaimIDs == nil {
		t.Fatal("no-capture must carry present-but-empty arrays, never nil")
	}
	if obs.Reference.Kind != ReferenceKindUnavailable {
		t.Fatalf("reference kind = %q, want unavailable even with a PreimageRef supplied", obs.Reference.Kind)
	}
	if obs.Reference.Commit != "" {
		t.Fatalf("a non-commit reference must carry an empty commit, got %q", obs.Reference.Commit)
	}
	if !obsHasReason(obs.Reasons, ReasonReferenceNotDurable) {
		t.Fatalf("reasons = %v, want %s", obs.Reasons, ReasonReferenceNotDurable)
	}
}

// TestS1WorkingTreeAllObservation covers RGA-100 and RGA-117/RGA-118's
// flag discipline: with a resolvable HEAD, each side is observed and the
// modes and hashes come from the tree and the filesystem, not the header.
func TestS1WorkingTreeAllObservation(t *testing.T) {
	dir := obsRepo(t)
	obsWrite(t, dir, "kept.txt", "v2\n", 0o644)
	obsWrite(t, dir, "added.txt", "new\n", 0o644)
	// `git diff HEAD` does not describe an untracked file, so the fixture
	// stages first and captures the staged-vs-HEAD diff.
	obsGit(t, dir, "add", "-A")
	patch := obsGit(t, dir, "diff", "--cached", "HEAD")

	obs := Observe(Input{
		Producer:     ProducerRecord,
		RepoRoot:     dir,
		Slug:         "demo",
		Patch:        patch,
		PatchPresent: true,
		Capture:      CaptureDescriptor{Mode: CaptureModeWorkingTreeAll},
		PreimageRef:  "HEAD",
	})

	if obs.Reference.Kind != ReferenceKindCommit {
		t.Fatalf("reference kind = %q, want commit", obs.Reference.Kind)
	}
	if obs.Reference.Commit != obsHead(t, dir) {
		t.Fatalf("reference commit = %q, want the resolved HEAD %q", obs.Reference.Commit, obsHead(t, dir))
	}
	if obs.Reference.PreimageSetSHA256 == "" || len(obs.Reference.PreimageSetSHA256) != 64 {
		t.Fatalf("preimage-set digest = %q, want 64 hex characters", obs.Reference.PreimageSetSHA256)
	}
	if len(obs.Effects) != 2 {
		t.Fatalf("got %d effects, want 2: %+v", len(obs.Effects), obs.Effects)
	}

	byPath := map[string]EffectObservation{}
	for _, e := range obs.Effects {
		byPath[e.Effect.Path] = e
	}

	modified, ok := byPath["kept.txt"]
	if !ok {
		t.Fatalf("kept.txt is missing from %v", byPath)
	}
	if modified.Effect.ChangeKind != gitutil.ChangeKindModify {
		t.Errorf("kept.txt change_kind = %q, want modify", modified.Effect.ChangeKind)
	}
	if !modified.Effect.PreimageObserved || !modified.Effect.PreimagePresent {
		t.Errorf("kept.txt preimage must be observed and present: %+v", modified.Effect)
	}
	if !modified.Effect.PostimageObserved || !modified.Effect.PostimagePresent {
		t.Errorf("kept.txt postimage must be observed and present: %+v", modified.Effect)
	}
	if modified.Effect.OldMode != gitutil.ModeRegular || modified.Effect.NewMode != gitutil.ModeRegular {
		t.Errorf("kept.txt modes = (%q,%q), want both 100644", modified.Effect.OldMode, modified.Effect.NewMode)
	}
	if modified.Effect.ObjectKind != gitutil.ObjectKindRegular {
		t.Errorf("kept.txt object_kind = %q, want regular", modified.Effect.ObjectKind)
	}
	if modified.Effect.ContentKind != gitutil.ContentKindText {
		t.Errorf("kept.txt content_kind = %q, want text", modified.Effect.ContentKind)
	}
	if modified.Effect.PreimageSHA256 == modified.Effect.PostimageSHA256 {
		t.Errorf("kept.txt pre/post digests are equal; the observation is not reading two sides")
	}
	if len(modified.ReasonCodes) != 0 {
		t.Errorf("a fully observed effect must carry no availability reason: %v", modified.ReasonCodes)
	}

	added, ok := byPath["added.txt"]
	if !ok {
		t.Fatalf("added.txt is missing from %v", byPath)
	}
	if added.Effect.ChangeKind != gitutil.ChangeKindAdd {
		t.Errorf("added.txt change_kind = %q, want add", added.Effect.ChangeKind)
	}
	// The preimage of an add is a NON-extant side: the producer looked in
	// the reference tree and proved it absent, which is a valid shape.
	if !added.Effect.PreimageObserved || added.Effect.PreimagePresent {
		t.Errorf("added.txt preimage must be observed-and-absent: %+v", added.Effect)
	}
	if added.Effect.PreimageSHA256 != "" || added.Effect.OldMode != "" {
		t.Errorf("a proven-absent side carries no hash and no mode: %+v", added.Effect)
	}
	if added.Effect.ObjectKind != gitutil.ObjectKindRegular || added.Effect.ContentKind != gitutil.ContentKindText {
		t.Errorf("added.txt kinds = (%q,%q), want (regular,text) from its observed postimage",
			added.Effect.ObjectKind, added.Effect.ContentKind)
	}
	if len(added.ReasonCodes) != 0 {
		t.Errorf("added.txt should carry no availability reason: %v", added.ReasonCodes)
	}
}

// TestS1ExecutableAndBinaryObservation proves the kind axes are decided
// independently and from observed bytes: an executable postimage is
// `executable`, and NUL-bearing bytes are `binary` even with no stanza
// marker in the patch.
//
// The executable half adapts to the filesystem rather than skipping: on a
// filesystem that does not carry an exec bit, Git records `100644` and
// the observation must agree with it, which is the same assertion in its
// other truthful form.
func TestS1ExecutableAndBinaryObservation(t *testing.T) {
	dir := obsRepo(t)
	obsWrite(t, dir, "run.sh", "#!/bin/sh\n", 0o755)
	obsWrite(t, dir, "blob.bin", "a\x00b\n", 0o644)

	info, err := os.Stat(filepath.Join(dir, "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	wantMode, wantObject := gitutil.ModeRegular, gitutil.ObjectKindRegular
	if info.Mode().Perm()&0o100 != 0 {
		wantMode, wantObject = gitutil.ModeExecutable, gitutil.ObjectKindExecutable
	}

	obsGit(t, dir, "add", "-A")
	patch := obsGit(t, dir, "diff", "--cached", "HEAD")

	obs := Observe(Input{
		Producer:     ProducerRecord,
		RepoRoot:     dir,
		Slug:         "demo",
		Patch:        patch,
		PatchPresent: true,
		Capture:      CaptureDescriptor{Mode: CaptureModeWorkingTreeAll},
		PreimageRef:  "HEAD",
	})

	byPath := map[string]EffectObservation{}
	for _, e := range obs.Effects {
		byPath[e.Effect.Path] = e
	}
	script, ok := byPath["run.sh"]
	if !ok {
		t.Fatalf("run.sh missing from %v", byPath)
	}
	if script.Effect.NewMode != wantMode {
		t.Errorf("run.sh new_mode = %q, want %q", script.Effect.NewMode, wantMode)
	}
	if script.Effect.ObjectKind != wantObject {
		t.Errorf("run.sh object_kind = %q, want %q", script.Effect.ObjectKind, wantObject)
	}
	if script.Effect.ContentKind != gitutil.ContentKindText {
		t.Errorf("run.sh content_kind = %q, want text (the axes are orthogonal)", script.Effect.ContentKind)
	}

	blob, ok := byPath["blob.bin"]
	if !ok {
		t.Fatalf("blob.bin missing from %v", byPath)
	}
	if blob.Effect.ObjectKind != gitutil.ObjectKindRegular {
		t.Errorf("blob.bin object_kind = %q, want regular", blob.Effect.ObjectKind)
	}
	if blob.Effect.ContentKind != gitutil.ContentKindBinary {
		t.Errorf("blob.bin content_kind = %q, want binary (a NUL byte in observed content is positive proof)", blob.Effect.ContentKind)
	}
}

func TestS1WorktreeModeUsesOwnerExecuteBit(t *testing.T) {
	cases := []struct {
		name string
		mode fs.FileMode
		want string
	}{
		{name: "owner-execute", mode: 0o744, want: gitutil.ModeExecutable},
		{name: "group-execute-only", mode: 0o674, want: gitutil.ModeRegular},
		{name: "other-execute-only", mode: 0o645, want: gitutil.ModeRegular},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := worktreeGitMode(tc.mode); got != tc.want {
				t.Fatalf("worktreeGitMode(%#o) = %s, want %s", tc.mode, got, tc.want)
			}
		})
	}
}

// TestS1UnobservedSideIsNotProvenAbsence covers RGA-117/RGA-118 and the
// contradictory-observation rule: with no resolvable reference, every
// preimage is UNOBSERVED with its availability reason — never the same
// `false` a producer uses to prove a path absent.
func TestS1UnobservedSideIsNotProvenAbsence(t *testing.T) {
	dir := obsRepo(t)
	obsWrite(t, dir, "kept.txt", "v2\n", 0o644)
	obsGit(t, dir, "add", "-A")
	patch := obsGit(t, dir, "diff", "--cached", "HEAD")

	obs := Observe(Input{
		Producer:     ProducerRecord,
		RepoRoot:     dir,
		Slug:         "demo",
		Patch:        patch,
		PatchPresent: true,
		Capture:      CaptureDescriptor{Mode: CaptureModeWorkingTreeAll},
		// No reference ref: the producer has nothing to reconstruct from.
		PreimageRef: "",
	})
	if obs.Reference.Kind != ReferenceKindUnavailable {
		t.Fatalf("reference kind = %q, want unavailable", obs.Reference.Kind)
	}
	if !obsHasReason(obs.Reasons, ReasonReferenceNotDurable) {
		t.Fatalf("reasons = %v, want %s", obs.Reasons, ReasonReferenceNotDurable)
	}
	if len(obs.Effects) != 1 {
		t.Fatalf("got %d effects, want 1", len(obs.Effects))
	}
	e := obs.Effects[0]
	if e.Effect.PreimageObserved {
		t.Fatalf("the preimage cannot be observed with no reference: %+v", e.Effect)
	}
	if e.Effect.PreimagePresent || e.Effect.PreimageSHA256 != "" || e.Effect.OldMode != "" {
		t.Fatalf("an unobserved side carries no presence, hash or mode: %+v", e.Effect)
	}
	if !obsHasReason(e.ReasonCodes, ReasonPreimageUnavailable) {
		t.Fatalf("effect reason_codes = %v, want %s", e.ReasonCodes, ReasonPreimageUnavailable)
	}
	// The postimage WAS observed, so its kinds survive; the content axis
	// still degrades because a modify needs both extant sides.
	if !e.Effect.PostimageObserved || !e.Effect.PostimagePresent {
		t.Fatalf("the postimage is readable from the worktree: %+v", e.Effect)
	}
	if e.Effect.ObjectKind != gitutil.ObjectKindRegular {
		t.Errorf("object_kind = %q, want regular from the observed postimage", e.Effect.ObjectKind)
	}
	if e.Effect.ContentKind != gitutil.ContentKindUnknown {
		t.Errorf("content_kind = %q, want unknown: an extant side was never read", e.Effect.ContentKind)
	}
}

// TestS1PostimageOfADeleteIsProvenAbsent proves the non-extant side of a
// delete is observed-and-absent rather than unobserved, and that the kind
// axes are taken from the preimage the producer actually read.
func TestS1PostimageOfADeleteIsProvenAbsent(t *testing.T) {
	dir := obsRepo(t)
	if err := os.Remove(filepath.Join(dir, "kept.txt")); err != nil {
		t.Fatal(err)
	}
	obsGit(t, dir, "add", "-A")
	patch := obsGit(t, dir, "diff", "--cached", "HEAD")

	obs := Observe(Input{
		Producer:     ProducerRecord,
		RepoRoot:     dir,
		Slug:         "demo",
		Patch:        patch,
		PatchPresent: true,
		Capture:      CaptureDescriptor{Mode: CaptureModeWorkingTreeAll},
		PreimageRef:  "HEAD",
	})
	if len(obs.Effects) != 1 {
		t.Fatalf("got %d effects, want 1: %+v", len(obs.Effects), obs.Effects)
	}
	e := obs.Effects[0]
	if e.Effect.ChangeKind != gitutil.ChangeKindDelete {
		t.Fatalf("change_kind = %q, want delete", e.Effect.ChangeKind)
	}
	if !e.Effect.PreimageObserved || !e.Effect.PreimagePresent {
		t.Fatalf("a delete's preimage is extant and must be observed: %+v", e.Effect)
	}
	if !e.Effect.PostimageObserved || e.Effect.PostimagePresent {
		t.Fatalf("a delete's postimage must be observed-and-absent: %+v", e.Effect)
	}
	if e.Effect.NewMode != "" || e.Effect.PostimageSHA256 != "" {
		t.Fatalf("a proven-absent side carries no mode or hash: %+v", e.Effect)
	}
	if e.Effect.ObjectKind != gitutil.ObjectKindRegular {
		t.Errorf("object_kind = %q, want regular from the observed preimage", e.Effect.ObjectKind)
	}
	if e.Effect.ContentKind != gitutil.ContentKindText {
		t.Errorf("content_kind = %q, want text from the observed preimage", e.Effect.ContentKind)
	}
	if len(e.ReasonCodes) != 0 {
		t.Errorf("a fully observed delete carries no availability reason: %v", e.ReasonCodes)
	}
}

// TestS1ExtantSideThatCannotBeReadIsDemotedToUnobserved is the
// contradictory-observation rule's bite-proof: an extant side that is not
// there is never recorded as observed-and-absent, because that would
// contradict the change kind the same record carries.
func TestS1ExtantSideThatCannotBeReadIsDemotedToUnobserved(t *testing.T) {
	dir := obsRepo(t)
	obsWrite(t, dir, "kept.txt", "v2\n", 0o644)
	obsGit(t, dir, "add", "-A")
	patch := obsGit(t, dir, "diff", "--cached", "HEAD")
	// Remove the postimage the patch's `modify` record requires to exist.
	if err := os.Remove(filepath.Join(dir, "kept.txt")); err != nil {
		t.Fatal(err)
	}

	obs := Observe(Input{
		Producer:     ProducerRecord,
		RepoRoot:     dir,
		Slug:         "demo",
		Patch:        patch,
		PatchPresent: true,
		Capture:      CaptureDescriptor{Mode: CaptureModeWorkingTreeAll},
		PreimageRef:  "HEAD",
	})
	if len(obs.Effects) != 1 {
		t.Fatalf("got %d effects, want 1", len(obs.Effects))
	}
	e := obs.Effects[0]
	if e.Effect.PostimageObserved {
		t.Fatalf("an extant side that is missing must be recorded UNOBSERVED, not observed-and-absent: %+v", e.Effect)
	}
	if !obsHasReason(e.ReasonCodes, ReasonPostimageUnavailable) {
		t.Fatalf("reason_codes = %v, want %s", e.ReasonCodes, ReasonPostimageUnavailable)
	}
	if e.Effect.ContentKind != gitutil.ContentKindUnknown {
		t.Errorf("content_kind = %q, want unknown when an extant side was not established", e.Effect.ContentKind)
	}
	if e.Effect.ObjectKind != gitutil.ObjectKindUnknown {
		t.Errorf("object_kind = %q, want unknown: the postimage the selection needs was not observed", e.Effect.ObjectKind)
	}
}

// TestS1CommittedRangePostimageComesFromTheRange covers the committed
// range modes: the postimage is read from the upper commit, not from
// whatever the worktree holds now.
func TestS1CommittedRangePostimageComesFromTheRange(t *testing.T) {
	dir := obsRepo(t)
	base := obsHead(t, dir)
	obsWrite(t, dir, "kept.txt", "v2\n", 0o644)
	obsGit(t, dir, "add", "-A")
	obsGit(t, dir, "commit", "-qm", "second")
	upper := obsHead(t, dir)
	patch := obsGit(t, dir, "diff", base, upper)

	// The live worktree diverges from the range's postimage.
	obsWrite(t, dir, "kept.txt", "LIVE-DIRT\n", 0o644)

	obs := Observe(Input{
		Producer:     ProducerRecord,
		RepoRoot:     dir,
		Slug:         "demo",
		Patch:        patch,
		PatchPresent: true,
		Capture:      CaptureDescriptor{Mode: CaptureModeCommittedRange},
		PreimageRef:  base,
		PostimageRef: upper,
	})
	if obs.Reference.Commit != base {
		t.Fatalf("reference commit = %q, want the resolved lower bound %q", obs.Reference.Commit, base)
	}
	if len(obs.Effects) != 1 {
		t.Fatalf("got %d effects, want 1", len(obs.Effects))
	}
	body := string(obs.Effects[0].Bytes.Postimage)
	if body != "v2\n" {
		t.Fatalf("postimage bytes = %q, want the committed %q — the live worktree must not be read", body, "v2\n")
	}
}

// TestS1PreimageSetDigestIsDeterministic proves the digest binds the
// ordered observation set and changes when any observed axis changes.
func TestS1PreimageSetDigestIsDeterministic(t *testing.T) {
	base := []EffectObservation{
		{Effect: gitutil.PatchEffect{
			Ordinal:          1,
			ChangeKind:       gitutil.ChangeKindModify,
			ContentKind:      gitutil.ContentKindText,
			ObjectKind:       gitutil.ObjectKindRegular,
			Path:             "a.txt",
			OldMode:          gitutil.ModeRegular,
			NewMode:          gitutil.ModeRegular,
			PreimageObserved: true,
			PreimagePresent:  true,
			PreimageSHA256:   strings.Repeat("a", 64),
		}},
		{Effect: gitutil.PatchEffect{
			Ordinal:    2,
			ChangeKind: gitutil.ChangeKindAdd,
			Path:       "b.txt",
		}},
	}
	first := PreimageSetDigest(base)
	if len(first) != 64 {
		t.Fatalf("digest = %q, want 64 hex characters", first)
	}
	if PreimageSetDigest(base) != first {
		t.Fatal("the digest is not deterministic across two calls on the same input")
	}

	// Ordinal order, not slice order, decides the encoding.
	reordered := []EffectObservation{base[1], base[0]}
	if PreimageSetDigest(reordered) != first {
		t.Fatal("the digest depends on slice order; it must depend on the ordinal order")
	}

	// Every observed axis is bound.
	for _, mutate := range []func(*gitutil.PatchEffect){
		func(e *gitutil.PatchEffect) { e.Path = "changed.txt" },
		func(e *gitutil.PatchEffect) { e.OldMode = gitutil.ModeExecutable },
		func(e *gitutil.PatchEffect) { e.PreimagePresent = false },
		func(e *gitutil.PatchEffect) { e.PreimageObserved = false },
		func(e *gitutil.PatchEffect) { e.PreimageSHA256 = strings.Repeat("b", 64) },
		func(e *gitutil.PatchEffect) { e.ContentKind = gitutil.ContentKindBinary },
		func(e *gitutil.PatchEffect) { e.ObjectKind = gitutil.ObjectKindSymlink },
	} {
		mutated := []EffectObservation{
			{Effect: base[0].Effect},
			{Effect: base[1].Effect},
		}
		effect := mutated[0].Effect
		mutate(&effect)
		mutated[0].Effect = effect
		if PreimageSetDigest(mutated) == first {
			t.Fatalf("the digest did not change for a mutated observation: %+v", effect)
		}
	}
}

// TestS1PatchPresenceAndParseRefusal covers the record-level shapes S1 can
// establish: an absent patch, a semantically empty one, and one the strict
// grammar refuses. None of them invents an effect list.
func TestS1PatchPresenceAndParseRefusal(t *testing.T) {
	dir := obsRepo(t)

	absent := Observe(Input{
		Producer: ProducerRecord,
		RepoRoot: dir,
		Slug:     "demo",
		Capture:  CaptureDescriptor{Mode: CaptureModeWorkingTreeAll},
	})
	if absent.PatchPresent || absent.PatchSHA256 != "" || len(absent.Effects) != 0 {
		t.Fatalf("an absent patch must bind nothing: %+v", absent)
	}
	if !obsHasReason(absent.Reasons, ReasonPatchMissing) {
		t.Fatalf("reasons = %v, want %s", absent.Reasons, ReasonPatchMissing)
	}

	empty := Observe(Input{
		Producer:     ProducerRecord,
		RepoRoot:     dir,
		Slug:         "demo",
		Patch:        "",
		PatchPresent: true,
		Capture:      CaptureDescriptor{Mode: CaptureModeWorkingTreeAll},
		PreimageRef:  "HEAD",
	})
	if !empty.PatchPresent {
		t.Fatal("a present-but-empty patch is still present")
	}
	if len(empty.PatchSHA256) != 64 {
		t.Fatalf("patch digest = %q; the empty-input digest is a real value and is published as one", empty.PatchSHA256)
	}
	if len(empty.Effects) != 0 {
		t.Fatalf("a semantically empty patch has no effects: %+v", empty.Effects)
	}
	if !obsHasReason(empty.Reasons, ReasonPatchEmpty) {
		t.Fatalf("reasons = %v, want %s", empty.Reasons, ReasonPatchEmpty)
	}

	refused := Observe(Input{
		Producer:     ProducerRecord,
		RepoRoot:     dir,
		Slug:         "demo",
		Patch:        "this is not a patch at all\n",
		PatchPresent: true,
		Capture:      CaptureDescriptor{Mode: CaptureModeWorkingTreeAll},
		PreimageRef:  "HEAD",
	})
	if !refused.PatchPresent || len(refused.PatchSHA256) != 64 {
		t.Fatalf("an unparseable patch is still present and still bound by its raw-byte digest: %+v", refused)
	}
	if len(refused.Effects) != 0 {
		t.Fatalf("a refused parse must not invent an effect list: %+v", refused.Effects)
	}
	if refused.ParseRefusal == "" {
		t.Fatal("the strict refusal message must be retained, not swallowed")
	}
	if !obsHasReason(refused.Reasons, ReasonPatchUnparseable) {
		t.Fatalf("reasons = %v, want %s", refused.Reasons, ReasonPatchUnparseable)
	}
}

// TestS1ArtifactSnapshotAndMutation covers the P6/P7 bound-artifact
// snapshots: an unreadable path is unobserved, an absent path is
// observed-and-absent, and mutation is decided from two observed
// snapshots rather than assumed.
func TestS1ArtifactSnapshotAndMutation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apply-recipe.json")

	missing := SnapshotArtifact(path)
	if !missing.Observed || missing.Present {
		t.Fatalf("an absent artifact is observed-and-absent: %+v", missing)
	}
	if missing.SHA256 != "" || missing.Bytes != nil {
		t.Fatalf("an absent artifact carries no hash and no bytes: %+v", missing)
	}

	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := SnapshotArtifact(path)
	if !before.Observed || !before.Present || len(before.SHA256) != 64 {
		t.Fatalf("a readable artifact must be fully observed: %+v", before)
	}

	obs := Observation{ArtifactBefore: &before, ArtifactAfter: &before}
	if obs.ArtifactMutated() {
		t.Fatal("identical snapshots must not be reported as a mutation")
	}

	if err := os.WriteFile(path, []byte("{\"feature\":\"x\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := SnapshotArtifact(path)
	obs = Observation{ArtifactBefore: &before, ArtifactAfter: &after}
	if !obs.ArtifactMutated() {
		t.Fatalf("changed bytes must be reported as a mutation:\nbefore=%+v\nafter=%+v", before, after)
	}

	// A half-observed pair proves nothing.
	unread := ArtifactSnapshot{Path: path}
	obs = Observation{ArtifactBefore: &unread, ArtifactAfter: &after}
	if obs.ArtifactMutated() {
		t.Fatal("an unobserved snapshot must not be read as evidence of a mutation")
	}
	obs = Observation{ArtifactBefore: &before}
	if obs.ArtifactMutated() {
		t.Fatal("a missing after-snapshot must not be read as evidence of a mutation")
	}
}

// TestS1RecorderSeamReceivesObservations proves the hand-off point a later
// slice replaces is real: ObserveAndEmit delivers exactly one observation
// to the installed recorder, and the default recorder is restored.
func TestS1RecorderSeamReceivesObservations(t *testing.T) {
	rec := &obsCollector{}
	restore := SetRecorder(rec)
	Emit(Observation{Producer: ProducerRecord, Slug: "one"})
	ObserveAndEmit(Input{
		Producer: ProducerImplement,
		RepoRoot: t.TempDir(),
		Slug:     "two",
		Capture:  CaptureDescriptor{Mode: CaptureModeNoCapture},
	})
	restore()
	Emit(Observation{Producer: ProducerRecord, Slug: "after-restore"})

	if len(rec.got) != 2 {
		t.Fatalf("recorder saw %d observation(s), want 2: %+v", len(rec.got), rec.got)
	}
	if rec.got[0].Slug != "one" || rec.got[1].Slug != "two" {
		t.Fatalf("recorder saw %q/%q, want one/two", rec.got[0].Slug, rec.got[1].Slug)
	}
	if rec.got[1].Producer != ProducerImplement {
		t.Fatalf("producer = %q, want implement", rec.got[1].Producer)
	}
}

type obsCollector struct{ got []Observation }

func (c *obsCollector) Record(o Observation) { c.got = append(c.got, o) }

func obsHasReason(reasons []string, want string) bool {
	for _, r := range reasons {
		if r == want {
			return true
		}
	}
	return false
}

// ── the closed producer vocabulary ───────────────────────────────────────

// TestS1ProducerVocabularyIsTheClosedRegistry pins all seven producer
// identifiers to the ADR-036 D15 registry spelling. They are published
// verbatim by a later slice, so a mismatch here is a wire-format defect
// that no downstream consumer could interpret.
func TestS1ProducerVocabularyIsTheClosedRegistry(t *testing.T) {
	want := []struct {
		id       ProducerID
		spelling string
	}{
		{ProducerRecord, "record"},
		{ProducerFeaturePatch, "feature-patch-amend"},
		{ProducerReconcileAccept, "reconcile-accept"},
		{ProducerCycle, "cycle"},
		{ProducerApplyDone, "apply-done"},
		{ProducerImplement, "implement"},
		{ProducerEdit, "artifact-edit"},
	}
	if len(want) != 7 {
		t.Fatalf("the registry has seven producers, this table has %d", len(want))
	}
	seen := map[string]bool{}
	for _, tc := range want {
		if string(tc.id) != tc.spelling {
			t.Errorf("producer %q must be spelled %q (ADR-036 §6.15 registry)", tc.id, tc.spelling)
		}
		if seen[tc.spelling] {
			t.Errorf("producer identifier %q is used twice", tc.spelling)
		}
		seen[tc.spelling] = true
		if !KnownProducer(tc.id) {
			t.Errorf("registered producer %q is not recognized", tc.id)
		}
	}
	// Wrong-input sensitivity: the two spellings the rev-0 implementation
	// used are NOT in the vocabulary, and neither is an invented one.
	for _, wrong := range []ProducerID{"feature-patch", "edit", "reconcile", ""} {
		if KnownProducer(wrong) {
			t.Errorf("%q must not be accepted as a registered producer", wrong)
		}
	}
}

// ── byte-exact path identity in the preimage-set digest ──────────────────

// TestS1PreimageSetDigestIsByteExactForPaths is the regression for a
// digest that hashed paths as TEXT. `encoding/json` replaces every byte
// that is not valid UTF-8 with U+FFFD, so two observation sets whose only
// difference was one such byte produced one digest — and the digest is
// what proves observation identity.
func TestS1PreimageSetDigestIsByteExactForPaths(t *testing.T) {
	set := func(path, oldPath string) []EffectObservation {
		return []EffectObservation{{Effect: gitutil.PatchEffect{
			Ordinal:    1,
			ChangeKind: gitutil.ChangeKindRename,
			Path:       path,
			OldPath:    oldPath,
		}}}
	}

	// Two DIFFERENT invalid-UTF-8 path bytes.
	first := PreimageSetDigest(set("\xff.txt", "src.txt"))
	second := PreimageSetDigest(set("\xfe.txt", "src.txt"))
	if first == second {
		t.Fatal("paths \\xff and \\xfe collide in the digest; the path field is not byte-exact")
	}
	// The same distinction on the old-path side.
	if PreimageSetDigest(set("dst.txt", "\xff.txt")) == PreimageSetDigest(set("dst.txt", "\xfe.txt")) {
		t.Fatal("old_path \\xff and \\xfe collide in the digest")
	}
	// A replacement character is not the same path as the raw byte it
	// would have been mangled into.
	if PreimageSetDigest(set("\xff.txt", "src.txt")) == PreimageSetDigest(set("\ufffd.txt", "src.txt")) {
		t.Fatal("a raw \\xff path and a U+FFFD path share a digest")
	}
	// And the digest is still deterministic for non-UTF-8 input.
	if PreimageSetDigest(set("\xff.txt", "src.txt")) != first {
		t.Fatal("the digest is not deterministic for a non-UTF-8 path")
	}
	// Ordinary paths still differ from each other, so the encoding did
	// not collapse everything into one value.
	if PreimageSetDigest(set("a.txt", "src.txt")) == PreimageSetDigest(set("b.txt", "src.txt")) {
		t.Fatal("two ordinary paths share a digest")
	}
}

// ── header/tree contradictions are their own reason ──────────────────────

// TestS1HeaderModeContradictionIsRecordedAsItsOwnReason covers the
// contradiction rule: when the observed mode and an explicit header mode
// disagree, the side is unestablished AND the record says why. Filing it
// under plain unavailability would report a wrong-object observation as
// an IO problem.
func TestS1HeaderModeContradictionIsRecordedAsItsOwnReason(t *testing.T) {
	dir := obsRepo(t)
	// The header declares a mode change to 100755; the worktree file was
	// never chmod-ed, so the observed postimage mode is 100644.
	patch := "diff --git a/kept.txt b/kept.txt\nold mode 100644\nnew mode 100755\n"

	obs := Observe(Input{
		Producer:     ProducerRecord,
		RepoRoot:     dir,
		Slug:         "demo",
		Patch:        patch,
		PatchPresent: true,
		Capture:      CaptureDescriptor{Mode: CaptureModeWorkingTreeAll},
		PreimageRef:  "HEAD",
	})
	if len(obs.Effects) != 1 {
		t.Fatalf("got %d effects, want 1", len(obs.Effects))
	}
	e := obs.Effects[0]
	if e.Effect.PostimageObserved {
		t.Fatalf("a contradicted side must not stay observed: %+v", e.Effect)
	}
	if !obsHasReason(e.ReasonCodes, ReasonPostimageModeContradiction) {
		t.Fatalf("reason_codes = %v, want %s", e.ReasonCodes, ReasonPostimageModeContradiction)
	}
	if !obsHasReason(e.ReasonCodes, ReasonPostimageUnavailable) {
		t.Fatalf("reason_codes = %v: a contradicted side is also unestablished", e.ReasonCodes)
	}
	if len(e.Contradictions) != 1 || !strings.Contains(e.Contradictions[0], "100755") {
		t.Fatalf("contradiction detail = %v, want one entry naming the header mode", e.Contradictions)
	}
	// The preimage agreed with its header mode, so it is observed and
	// carries no contradiction.
	if !e.Effect.PreimageObserved || obsHasReason(e.ReasonCodes, ReasonPreimageModeContradiction) {
		t.Fatalf("the agreeing side must stay observed and uncontradicted: %+v (%v)", e.Effect, e.ReasonCodes)
	}

	// Wrong-input sensitivity: an ordinary modify record, whose `index`
	// line declares the mode both sides actually have, raises no
	// contradiction at all — so the row above measures the DISAGREEMENT
	// rather than the mere presence of a mode header.
	agreeing := Observe(Input{
		Producer:     ProducerRecord,
		RepoRoot:     dir,
		Slug:         "demo",
		Patch:        "diff --git a/kept.txt b/kept.txt\nindex 1111111..2222222 100644\n@@ -1 +1 @@\n-v1\n+v2\n",
		PatchPresent: true,
		Capture:      CaptureDescriptor{Mode: CaptureModeWorkingTreeAll},
		PreimageRef:  "HEAD",
	})
	if len(agreeing.Effects) != 1 {
		t.Fatalf("got %d effects, want 1", len(agreeing.Effects))
	}
	if !agreeing.Effects[0].Effect.PostimageObserved || !agreeing.Effects[0].Effect.PreimageObserved {
		t.Fatalf("agreeing sides must stay observed: %+v", agreeing.Effects[0].Effect)
	}
	if len(agreeing.Effects[0].Contradictions) != 0 {
		t.Fatalf("an agreeing side must record no contradiction: %v", agreeing.Effects[0].Contradictions)
	}
}

// ── a worktree submodule is observed from the index ──────────────────────

// TestS1WorktreeGitlinkIsObservedFromTheIndex covers the one working-tree
// shape a filesystem read cannot answer. A submodule is a DIRECTORY in
// the worktree, and the pointer the patch describes — mode 160000 and a
// commit id — lives in the index. Recording it as unobserved merely
// because `Lstat` said "directory" would drop a real, readable
// observation.
func TestS1WorktreeGitlinkIsObservedFromTheIndex(t *testing.T) {
	dir := obsRepo(t)
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	obsGit(t, sub, "init", "-q", "-b", "main", ".")
	obsGit(t, sub, "config", "user.email", "t@example.com")
	obsGit(t, sub, "config", "user.name", "T")
	obsGit(t, sub, "config", "commit.gpgsign", "false")
	obsWrite(t, sub, "inner.txt", "inner\n", 0o644)
	obsGit(t, sub, "add", "-A")
	obsGit(t, sub, "commit", "-qm", "sub seed")
	subCommit := obsHead(t, sub)

	// `git add <dir>` on an embedded repository records a gitlink.
	obsGit(t, dir, "add", "sub")

	patch := "diff --git a/sub b/sub\nnew file mode 160000\nindex 0000000.." + subCommit[:7] + "\n" +
		"--- /dev/null\n+++ b/sub\n@@ -0,0 +1 @@\n+Subproject commit " + subCommit + "\n"

	obs := Observe(Input{
		Producer:     ProducerRecord,
		RepoRoot:     dir,
		Slug:         "demo",
		Patch:        patch,
		PatchPresent: true,
		Capture:      CaptureDescriptor{Mode: CaptureModeWorkingTreeAll},
		PreimageRef:  "HEAD",
	})
	if len(obs.Effects) != 1 {
		t.Fatalf("got %d effects, want 1", len(obs.Effects))
	}
	e := obs.Effects[0]
	if !e.Effect.PostimageObserved || !e.Effect.PostimagePresent {
		t.Fatalf("a worktree gitlink must be observed from the index, got %+v (%v)", e.Effect, e.ReasonCodes)
	}
	if e.Effect.NewMode != gitutil.ModeGitlink {
		t.Fatalf("new_mode = %q, want %q", e.Effect.NewMode, gitutil.ModeGitlink)
	}
	if e.Effect.ObjectKind != gitutil.ObjectKindGitlink {
		t.Fatalf("object_kind = %q, want gitlink", e.Effect.ObjectKind)
	}
	if e.Effect.ContentKind != gitutil.ContentKindNone {
		t.Fatalf("content_kind = %q, want none: a gitlink has no file content", e.Effect.ContentKind)
	}
	// The identity binds the referenced commit, not a blob nobody can
	// read, so no bytes are carried.
	if len(e.Bytes.Postimage) != 0 {
		t.Fatalf("a gitlink side carries no bytes, got %q", e.Bytes.Postimage)
	}
	if e.Effect.PostimageSHA256 == "" {
		t.Fatal("a gitlink side must still bind an identity")
	}

	// Wrong-input sensitivity: an ordinary directory the index does NOT
	// record as a gitlink establishes nothing.
	plain := filepath.Join(dir, "plaindir")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	plainObs := Observe(Input{
		Producer:     ProducerRecord,
		RepoRoot:     dir,
		Slug:         "demo",
		Patch:        "diff --git a/plaindir b/plaindir\nnew file mode 160000\n--- /dev/null\n+++ b/plaindir\n@@ -0,0 +1 @@\n+Subproject commit " + subCommit + "\n",
		PatchPresent: true,
		Capture:      CaptureDescriptor{Mode: CaptureModeWorkingTreeAll},
		PreimageRef:  "HEAD",
	})
	if len(plainObs.Effects) != 1 {
		t.Fatalf("got %d effects, want 1", len(plainObs.Effects))
	}
	if plainObs.Effects[0].Effect.PostimageObserved {
		t.Fatalf("a directory the index does not record as a gitlink must stay unobserved: %+v", plainObs.Effects[0].Effect)
	}
}

// ── the read cost is bounded per reference ───────────────────────────────

// TestS1GitReadCostIsBoundedPerReference is the process-count seam
// (§6.2 performance obligation). Reading each side with its own
// subprocess made observing a patch cost O(effects) forks BEFORE the
// producer's first write; a 200-file patch would have started 400+ of
// them. The budget is per REFERENCE:
//
//	≤1 rev-parse per reference, ≤1 ls-tree per commit, 1 cat-file --batch
//
// so the measured count must not change when the effect count does.
func TestS1GitReadCostIsBoundedPerReference(t *testing.T) {
	build := func(t *testing.T, files int) (dir string, patch string) {
		t.Helper()
		dir = obsRepo(t)
		for i := 0; i < files; i++ {
			obsWrite(t, dir, fmt.Sprintf("f%02d.txt", i), "v1\n", 0o644)
		}
		obsGit(t, dir, "add", "-A")
		obsGit(t, dir, "commit", "-qm", "seed files")
		for i := 0; i < files; i++ {
			obsWrite(t, dir, fmt.Sprintf("f%02d.txt", i), "v2\n", 0o644)
		}
		return dir, obsGit(t, dir, "diff", "HEAD")
	}

	measure := func(t *testing.T, in Input) (Observation, int) {
		t.Helper()
		before := gitProcessCount()
		obs := Observe(in)
		return obs, gitProcessCount() - before
	}

	smallDir, smallPatch := build(t, 2)
	largeDir, largePatch := build(t, 12)

	small, smallCost := measure(t, Input{
		Producer:     ProducerRecord,
		RepoRoot:     smallDir,
		Slug:         "demo",
		Patch:        smallPatch,
		PatchPresent: true,
		Capture:      CaptureDescriptor{Mode: CaptureModeWorkingTreeAll},
		PreimageRef:  "HEAD",
	})
	large, largeCost := measure(t, Input{
		Producer:     ProducerRecord,
		RepoRoot:     largeDir,
		Slug:         "demo",
		Patch:        largePatch,
		PatchPresent: true,
		Capture:      CaptureDescriptor{Mode: CaptureModeWorkingTreeAll},
		PreimageRef:  "HEAD",
	})

	if len(small.Effects) != 2 || len(large.Effects) != 12 {
		t.Fatalf("fixture produced %d and %d effects, want 2 and 12", len(small.Effects), len(large.Effects))
	}
	if smallCost != largeCost {
		t.Fatalf("git process count scaled with the effect count: %d for 2 effects, %d for 12", smallCost, largeCost)
	}
	// One rev-parse for the reference, one ls-tree for its paths, one
	// cat-file batch for every blob body. Working-tree postimages are
	// direct filesystem reads and cost nothing.
	if smallCost != 3 {
		t.Fatalf("working-tree observation cost %d git processes, want 3 (rev-parse + ls-tree + cat-file)", smallCost)
	}
	// The batching must not have cost correctness: every side is still
	// established, with real bytes on both.
	for _, e := range large.Effects {
		if !e.Effect.PreimageObserved || !e.Effect.PreimagePresent || string(e.Bytes.Preimage) != "v1\n" {
			t.Fatalf("batched preimage is wrong for %s: %+v (%q)", e.Effect.Path, e.Effect, e.Bytes.Preimage)
		}
		if !e.Effect.PostimageObserved || !e.Effect.PostimagePresent || string(e.Bytes.Postimage) != "v2\n" {
			t.Fatalf("batched postimage is wrong for %s: %+v (%q)", e.Effect.Path, e.Effect, e.Bytes.Postimage)
		}
	}

	t.Run("a-committed-range-resolves-each-reference-once", func(t *testing.T) {
		dir := obsRepo(t)
		base := obsHead(t, dir)
		for i := 0; i < 8; i++ {
			obsWrite(t, dir, fmt.Sprintf("r%02d.txt", i), "v2\n", 0o644)
		}
		obsGit(t, dir, "add", "-A")
		obsGit(t, dir, "commit", "-qm", "range upper")
		upper := obsHead(t, dir)
		patch := obsGit(t, dir, "diff", base, upper)

		obs, cost := measure(t, Input{
			Producer:     ProducerRecord,
			RepoRoot:     dir,
			Slug:         "demo",
			Patch:        patch,
			PatchPresent: true,
			Capture:      CaptureDescriptor{Mode: CaptureModeCommittedRange},
			PreimageRef:  base,
			PostimageRef: upper,
		})
		if len(obs.Effects) != 8 {
			t.Fatalf("got %d effects, want 8", len(obs.Effects))
		}
		// Two rev-parse calls (one per reference), two ls-tree calls (one
		// per commit) and ONE cat-file batch covering both commits'
		// blobs. Nothing here is per-effect.
		if cost != 5 {
			t.Fatalf("committed-range observation cost %d git processes, want 5", cost)
		}
		for _, e := range obs.Effects {
			if string(e.Bytes.Postimage) != "v2\n" {
				t.Fatalf("range postimage for %s = %q, want the committed bytes", e.Effect.Path, e.Bytes.Postimage)
			}
		}
	})

	t.Run("an-observation-with-no-reference-starts-no-process", func(t *testing.T) {
		dir := obsRepo(t)
		obsWrite(t, dir, "kept.txt", "v2\n", 0o644)
		patch := "diff --git a/kept.txt b/kept.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-v1\n+v2\n"
		_, cost := measure(t, Input{
			Producer:     ProducerImplement,
			RepoRoot:     dir,
			Slug:         "demo",
			Patch:        patch,
			PatchPresent: true,
			Capture:      CaptureDescriptor{Mode: CaptureModeNoCapture},
		})
		if cost != 0 {
			t.Fatalf("a no-capture observation started %d git process(es), want 0", cost)
		}
	})
}
