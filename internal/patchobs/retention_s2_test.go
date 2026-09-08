package patchobs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

type s2ReadProbe struct {
	reader  io.Reader
	calls   int
	largest int
}

func (r *s2ReadProbe) Read(p []byte) (int, error) {
	r.calls++
	if len(p) > r.largest {
		r.largest = len(p)
	}
	return r.reader.Read(p)
}

func TestRGAS2ImageReaderBudgetBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name        string
		size, limit int64
		used        int64
		wantError   bool
	}{
		{name: "below", size: 2, limit: 3},
		{name: "exact", size: 3, limit: 3},
		{name: "above", size: 4, limit: 3, wantError: true},
		{name: "cumulative-exact", size: 3, limit: 5, used: 2},
		{name: "cumulative-over", size: 4, limit: 5, used: 2, wantError: true},
		{name: "empty-at-zero", size: 0, limit: 0},
		{name: "nonempty-at-zero", size: 1, limit: 0, wantError: true},
		{name: "negative-size", size: -1, limit: 3, wantError: true},
		{name: "huge-advertised-size", size: 1 << 40, limit: 3, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			budget := &imageBudget{limit: tc.limit, used: tc.used}
			reader := &s2ReadProbe{reader: strings.NewReader("abcdef")}
			body, err := readImage(reader, tc.size, budget)
			if tc.wantError {
				if err == nil || !strings.Contains(err.Error(), "image retention budget exceeded") {
					t.Fatalf("over-budget size %d: body=%q error=%v", tc.size, body, err)
				}
				if body != nil || reader.calls != 0 || budget.used != tc.used {
					t.Fatalf("refusal read or retained bytes: body=%v calls=%d used=%d",
						body, reader.calls, budget.used)
				}
				return
			}
			if err != nil || string(body) != "abcdef"[:int(tc.size)] || cap(body) != int(tc.size) {
				t.Fatalf("accepted boundary: body=%q capacity=%d error=%v", body, cap(body), err)
			}
			if budget.used != tc.used+tc.size {
				t.Fatalf("used=%d, want %d", budget.used, tc.used+tc.size)
			}
		})
	}
}

func TestRGAS2WorktreeSizeChangeFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		size       int64
		wantError  bool
	}{
		{name: "exact", body: "abc", size: 3},
		{name: "empty", size: 0},
		{name: "grew", body: "abcd", size: 3, wantError: true},
		{name: "grew-from-empty", body: "a", size: 0, wantError: true},
		{name: "shrank", body: "ab", size: 3, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			budget := &imageBudget{limit: 3}
			body, err := readWorktreeImage(strings.NewReader(tc.body), tc.size, budget)
			if tc.wantError {
				if err == nil || body != nil || budget.used != 0 {
					t.Fatalf("partial body established: body=%q used=%d error=%v", body, budget.used, err)
				}
				return
			}
			if err != nil || string(body) != tc.body || budget.used != tc.size {
				t.Fatalf("exact body refused: body=%q used=%d error=%v", body, budget.used, err)
			}
		})
	}
}

// s2ZeroStream supplies a huge body without constructing a huge fixture.
// Refusing large read requests makes an unchecked whole-body read fail
// behaviorally, independently of the allocation counter below.
type s2ZeroStream struct {
	left    int64
	largest int
}

func (r *s2ZeroStream) Read(p []byte) (int, error) {
	if len(p) > r.largest {
		r.largest = len(p)
	}
	if len(p) > 32<<10 {
		return 0, fmt.Errorf("unbounded read request: %d", len(p))
	}
	if r.left == 0 {
		return 0, io.EOF
	}
	n := len(p)
	if int64(n) > r.left {
		n = int(r.left)
	}
	clear(p[:n])
	r.left -= int64(n)
	return n, nil
}

func TestRGAS2CatFileStreamsHugeBodyWithoutRetainingIt(t *testing.T) {
	const hugeSize = 64 << 20
	huge := &s2ZeroStream{left: hugeSize}
	stream := io.MultiReader(
		strings.NewReader(fmt.Sprintf("huge blob %d\n", hugeSize)),
		huge,
		strings.NewReader("\nsmall blob 3\nok\n\nempty blob 0\n\n"),
	)
	budget := &imageBudget{limit: 3}
	blobs := map[string]blobObservation{}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	err := parseCatFileBatch(stream, blobs, budget)
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	if huge.left != 0 || huge.largest > 32<<10 {
		t.Fatalf("huge body was not streamed: remaining=%d largest-read=%d", huge.left, huge.largest)
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 1<<20 {
		t.Fatalf("streaming allocated %d bytes for a three-byte budget", allocated)
	}
	if blobs["huge"].bytes != nil || !strings.Contains(blobs["huge"].diagnostic, "image retention budget exceeded") {
		t.Fatalf("huge body was not explicitly refused: %+v", blobs["huge"])
	}
	if string(blobs["small"].bytes) != "ok\n" || blobs["small"].diagnostic != "" || budget.used != 3 {
		t.Fatalf("a refused huge body blocked the small body: %+v used=%d", blobs, budget.used)
	}
	if empty, ok := blobs["empty"]; !ok || empty.bytes == nil || empty.diagnostic != "" {
		t.Fatalf("an empty image did not fit at the exhausted boundary: %+v", blobs)
	}
}

func TestRGAS2CatFileRejectsMalformedBodies(t *testing.T) {
	for _, tc := range []struct {
		name, stream string
		wantError    bool
		wantCount    int
		wantUsed     int64
	}{
		{name: "valid", stream: "one blob 3\nabc\n", wantCount: 1, wantUsed: 3},
		{name: "missing", stream: "one missing\n"},
		{name: "non-blob", stream: "one tree 3\nabc\n"},
		{name: "duplicate-does-not-allocate-again", stream: "one blob 3\nabc\none blob 3\nabc\n", wantCount: 1, wantUsed: 3},
		{name: "short-body", stream: "one blob 3\nab", wantError: true},
		{name: "missing-delimiter", stream: "one blob 3\nabc", wantError: true},
		{name: "wrong-delimiter", stream: "one blob 3\nabc!", wantError: true},
		{name: "negative-size", stream: "one blob -1\n", wantError: true},
		{name: "overflow-size", stream: "one blob 9223372036854775808\n", wantError: true},
		{name: "bad-size", stream: "one blob enormous\n", wantError: true},
		{name: "bad-header", stream: "one strange\n", wantError: true},
		{name: "unterminated-header", stream: "one blob 3", wantError: true},
		{name: "unbounded-header", stream: strings.Repeat("x", 4096) + "\n", wantError: true},
		{name: "prior-valid-frame-survives", stream: "one blob 3\nabc\ntwo blob 3\nab", wantError: true, wantCount: 1, wantUsed: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			budget := &imageBudget{limit: 6}
			blobs := map[string]blobObservation{}
			err := parseCatFileBatch(strings.NewReader(tc.stream), blobs, budget)
			if (err != nil) != tc.wantError || len(blobs) != tc.wantCount || budget.used != tc.wantUsed {
				t.Fatalf("error=%v blobs=%+v used=%d; want error=%v count=%d used=%d",
					err, blobs, budget.used, tc.wantError, tc.wantCount, tc.wantUsed)
			}
		})
	}
	t.Run("oversized-truncated-body-keeps-refusal-detail", func(t *testing.T) {
		budget := &imageBudget{limit: 3}
		blobs := map[string]blobObservation{}
		err := parseCatFileBatch(strings.NewReader("one blob 4096\nshort"), blobs, budget)
		if err == nil || budget.used != 0 || blobs["one"].bytes != nil ||
			!strings.Contains(blobs["one"].diagnostic, "image retention budget exceeded") {
			t.Fatalf("truncated oversized body lost its explicit refusal: error=%v blobs=%+v used=%d", err, blobs, budget.used)
		}
	})
}

func s2Input(t *testing.T, dir string) Input {
	t.Helper()
	return Input{
		Producer: ProducerRecord, RepoRoot: dir, Slug: "retention",
		Patch: obsGit(t, dir, "diff", "HEAD"), PatchPresent: true,
		Capture: CaptureDescriptor{Mode: CaptureModeWorkingTreeAll}, PreimageRef: "HEAD",
	}
}

func s2Effect(t *testing.T, obs Observation, path string) EffectObservation {
	t.Helper()
	if obs.ParseRefusal != "" {
		t.Fatalf("fixture patch refused: %s", obs.ParseRefusal)
	}
	for _, effect := range obs.Effects {
		if effect.Effect.Path == path {
			return effect
		}
	}
	t.Fatalf("missing effect %q: %+v", path, obs.Effects)
	return EffectObservation{}
}

func s2RequireUnavailable(t *testing.T, e EffectObservation, preimage bool) {
	t.Helper()
	observed, present := e.Effect.PostimageObserved, e.Effect.PostimagePresent
	hash, mode, body := e.Effect.PostimageSHA256, e.Effect.NewMode, e.Bytes.Postimage
	reason, side := ReasonPostimageUnavailable, "postimage"
	if preimage {
		observed, present = e.Effect.PreimageObserved, e.Effect.PreimagePresent
		hash, mode, body = e.Effect.PreimageSHA256, e.Effect.OldMode, e.Bytes.Preimage
		reason, side = ReasonPreimageUnavailable, "preimage"
	}
	if observed || present || hash != "" || mode != "" || body != nil || !obsHasReason(e.ReasonCodes, reason) {
		t.Fatalf("%s over-budget side is not honestly unavailable: %+v", side, e)
	}
	for _, diagnostic := range e.Contradictions {
		if strings.Contains(diagnostic, side+" of ") && strings.Contains(diagnostic, "image retention budget exceeded") {
			return
		}
	}
	t.Fatalf("%s refusal lacks human diagnostic: %+v", side, e)
}

func TestRGAS2ObservationSharedWorktreeAndGitBudget(t *testing.T) {
	dir := obsRepo(t)
	obsWrite(t, dir, "kept.txt", "v2\n", 0o644)
	in := s2Input(t, dir)
	full := observeWithImageBudget(in, 6)
	e := s2Effect(t, full, "kept.txt")
	if !e.Effect.PreimageObserved || !e.Effect.PostimageObserved ||
		string(e.Bytes.Preimage) != "v1\n" || string(e.Bytes.Postimage) != "v2\n" ||
		e.Effect.PreimageSHA256 != sha256Hex([]byte("v1\n")) ||
		e.Effect.PostimageSHA256 != sha256Hex([]byte("v2\n")) ||
		e.Effect.OldMode != gitutil.ModeRegular || e.Effect.NewMode != gitutil.ModeRegular ||
		e.Effect.ObjectKind != gitutil.ObjectKindRegular || e.Effect.ContentKind != gitutil.ContentKindText ||
		len(e.ReasonCodes) != 0 || len(e.Contradictions) != 0 {
		t.Fatalf("exact-boundary observation changed S1 semantics: %+v", e)
	}
	before := gitProcessCount()
	partial := observeWithImageBudget(in, 5)
	if cost := gitProcessCount() - before; cost != 3 {
		t.Fatalf("bounded observation uses %d Git processes, want 3", cost)
	}
	e = s2Effect(t, partial, "kept.txt")
	s2RequireUnavailable(t, e, true)
	if !e.Effect.PostimageObserved || string(e.Bytes.Postimage) != "v2\n" ||
		e.Effect.ContentKind != gitutil.ContentKindUnknown {
		t.Fatalf("retained postimage or unavailable content classification is wrong: %+v", e)
	}
	if partial.PatchSHA256 != full.PatchSHA256 || string(partial.PatchBytes) != in.Patch ||
		partial.Reference.Commit != full.Reference.Commit ||
		partial.Reference.PreimageSetSHA256 == full.Reference.PreimageSetSHA256 {
		t.Fatal("bounded capture lost patch/reference identity or retained the full observation's set digest")
	}
	for _, limit := range []int64{0, 2} {
		e = s2Effect(t, observeWithImageBudget(in, limit), "kept.txt")
		s2RequireUnavailable(t, e, true)
		s2RequireUnavailable(t, e, false)
		if e.Effect.ObjectKind != gitutil.ObjectKindUnknown || e.Effect.ContentKind != gitutil.ContentKindUnknown {
			t.Fatalf("unobserved bodies were classified from headers: %+v", e)
		}
	}
}

func TestRGAS2WorktreeCumulativeRetentionAndLaterSmallFiles(t *testing.T) {
	for _, tc := range []struct {
		name   string
		bodies []string
		limit  int64
		want   []bool
	}{
		{name: "cumulative", bodies: []string{"aa\n", "bb\n", "cc\n"}, limit: 6, want: []bool{true, true, false}},
		{name: "oversized-does-not-starve-small", bodies: []string{"too large\n", "ok\n", ""}, limit: 3, want: []bool{false, true, true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := obsRepo(t)
			for i, body := range tc.bodies {
				obsWrite(t, dir, fmt.Sprintf("added-%d.txt", i), body, 0o644)
			}
			obsGit(t, dir, "add", "-A")
			in := s2Input(t, dir)
			obs := observeWithImageBudget(in, tc.limit)
			var retained int64
			for i, want := range tc.want {
				e := s2Effect(t, obs, fmt.Sprintf("added-%d.txt", i))
				if !e.Effect.PreimageObserved || e.Effect.PreimagePresent ||
					e.Effect.PreimageSHA256 != "" || e.Effect.OldMode != "" {
					t.Fatalf("budget converted proven preimage absence to unavailable: %+v", e)
				}
				if !want {
					s2RequireUnavailable(t, e, false)
					continue
				}
				if !e.Effect.PostimageObserved || !e.Effect.PostimagePresent ||
					string(e.Bytes.Postimage) != tc.bodies[i] ||
					e.Effect.PostimageSHA256 != sha256Hex([]byte(tc.bodies[i])) ||
					len(e.ReasonCodes) != 0 {
					t.Fatalf("small retained image is wrong: %+v", e)
				}
				retained += int64(cap(e.Bytes.Postimage))
			}
			if retained != tc.limit {
				t.Fatalf("retained capacity=%d, want the exact %d-byte budget", retained, tc.limit)
			}
		})
	}
}

func TestRGAS2DefaultLimitRejectsHugeWorktreeBeforeReading(t *testing.T) {
	dir := obsRepo(t)
	obsWrite(t, dir, "kept.txt", "v2\n", 0o644)
	in := s2Input(t, dir)
	file, err := os.OpenFile(filepath.Join(dir, "kept.txt"), os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	err = file.Truncate(imageRetentionLimit + 1)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("sparse fixture: truncate=%v close=%v", err, closeErr)
	}
	obs := Observe(in)
	e := s2Effect(t, obs, "kept.txt")
	s2RequireUnavailable(t, e, false)
	if !e.Effect.PreimageObserved || string(e.Bytes.Preimage) != "v1\n" {
		t.Fatalf("oversized postimage consumed the preimage budget: %+v", e)
	}
}

func TestRGAS2GitOversizedPreimageDoesNotBlockOtherEffects(t *testing.T) {
	dir := obsRepo(t)
	large := strings.Repeat("x", 4096) + "\n"
	obsWrite(t, dir, "huge.txt", large, 0o644)
	obsGit(t, dir, "add", "-A")
	obsGit(t, dir, "commit", "-qm", "large preimage")
	obsWrite(t, dir, "huge.txt", "v2\n", 0o644)
	obsWrite(t, dir, "kept.txt", "v2\n", 0o644)
	in := s2Input(t, dir)
	before := gitProcessCount()
	obs := observeWithImageBudget(in, 9)
	if cost := gitProcessCount() - before; cost != 3 {
		t.Fatalf("oversized Git image spawned %d processes, want 3", cost)
	}
	huge := s2Effect(t, obs, "huge.txt")
	s2RequireUnavailable(t, huge, true)
	if !huge.Effect.PostimageObserved || string(huge.Bytes.Postimage) != "v2\n" {
		t.Fatalf("oversized preimage lost its small postimage: %+v", huge)
	}
	small := s2Effect(t, obs, "kept.txt")
	if !small.Effect.PreimageObserved || !small.Effect.PostimageObserved ||
		string(small.Bytes.Preimage) != "v1\n" || string(small.Bytes.Postimage) != "v2\n" {
		t.Fatalf("oversized preimage blocked another effect: %+v", small)
	}
	full := s2Effect(t, Observe(in), "huge.txt")
	if !full.Effect.PreimageObserved || string(full.Bytes.Preimage) != large ||
		full.Effect.PreimageSHA256 != sha256Hex([]byte(large)) || len(full.ReasonCodes) != 0 {
		t.Fatalf("the same image should be observed within budget: %+v", full)
	}
}

func TestRGAS2SymlinkImagesConsumeTheSameBudget(t *testing.T) {
	dir := obsRepo(t)
	if err := os.Symlink("kept.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int64{7, 8} {
		budget := &imageBudget{limit: limit}
		side, directory := observeWorktreePath(dir, "link", budget)
		if directory {
			t.Fatal("symlink was treated as a gitlink candidate")
		}
		if limit == 7 {
			if side.observed || side.present || side.bytes != nil || side.sha256 != "" || side.mode != "" ||
				!strings.Contains(side.diagnostic, "image retention budget exceeded") || budget.used != 0 {
				t.Fatalf("symlink escaped image budget: %+v used=%d", side, budget.used)
			}
			continue
		}
		if !side.observed || !side.present || string(side.bytes) != "kept.txt" ||
			side.mode != gitutil.ModeSymlink || side.sha256 != sha256Hex([]byte("kept.txt")) || budget.used != 8 {
			t.Fatalf("exact-budget symlink lost its truthful identity: %+v used=%d", side, budget.used)
		}
	}
}

func TestRGAS2CommittedImagesRemainBatchedAndDeduplicated(t *testing.T) {
	dir := obsRepo(t)
	for i := range 4 {
		obsWrite(t, dir, fmt.Sprintf("same-%d.txt", i), "old\n", 0o644)
	}
	obsGit(t, dir, "add", "-A")
	obsGit(t, dir, "commit", "-qm", "duplicate preimages")
	base := obsHead(t, dir)
	for i := range 4 {
		obsWrite(t, dir, fmt.Sprintf("same-%d.txt", i), "new\n", 0o644)
	}
	obsGit(t, dir, "add", "-A")
	obsGit(t, dir, "commit", "-qm", "duplicate postimages")
	in := Input{
		Producer: ProducerRecord, RepoRoot: dir, Slug: "range",
		Patch: obsGit(t, dir, "diff", base, "HEAD"), PatchPresent: true,
		Capture: CaptureDescriptor{Mode: CaptureModeCommittedRange}, PreimageRef: base, PostimageRef: "HEAD",
	}
	before := gitProcessCount()
	obs := observeWithImageBudget(in, 8)
	if cost := gitProcessCount() - before; cost != 5 {
		t.Fatalf("committed observation used %d Git processes, want 5", cost)
	}
	if len(obs.Effects) != 4 {
		t.Fatalf("effects=%d, want 4", len(obs.Effects))
	}
	rec := &obsCollector{}
	restore := SetRecorder(rec)
	defer restore()
	Emit(obs)
	if len(rec.got) != 1 {
		t.Fatalf("recorder received %d observations", len(rec.got))
	}
	cloned := rec.got[0]
	for i, e := range obs.Effects {
		if !e.Effect.PreimageObserved || !e.Effect.PostimageObserved ||
			string(e.Bytes.Preimage) != "old\n" || string(e.Bytes.Postimage) != "new\n" {
			t.Fatalf("shared images exhausted budget at effect %d: %+v", i, e)
		}
		for _, sides := range [][]SideBytes{
			{obs.Effects[0].Bytes, e.Bytes},
			{cloned.Effects[0].Bytes, cloned.Effects[i].Bytes},
		} {
			if &sides[0].Preimage[0] != &sides[1].Preimage[0] ||
				&sides[0].Postimage[0] != &sides[1].Postimage[0] {
				t.Fatal("repeated objects multiplied retained backing arrays")
			}
		}
		if &e.Bytes.Preimage[0] == &cloned.Effects[i].Bytes.Preimage[0] ||
			&e.Bytes.Postimage[0] == &cloned.Effects[i].Bytes.Postimage[0] {
			t.Fatal("recorder images alias the producer")
		}
	}
	partial := observeWithImageBudget(in, 7)
	for _, e := range partial.Effects {
		if e.Effect.PreimageObserved == e.Effect.PostimageObserved {
			t.Fatalf("distinct cumulative blob budget was not enforced: %+v", e)
		}
		s2RequireUnavailable(t, e, !e.Effect.PreimageObserved)
	}
}

func TestRGAS2LateWorktreeMutationAndRecorderCannotChangeObservation(t *testing.T) {
	dir := obsRepo(t)
	obsWrite(t, dir, "kept.txt", "v2\n", 0o644)
	in := s2Input(t, dir)
	in.Capture.Pathspecs = []string{"kept.txt"}
	in.Capture.ClaimIDs = []string{"claim-1"}
	obs := observeWithImageBudget(in, 6)
	obs.Reasons = []string{"diagnostic"}
	obs.Effects[0].ReasonCodes = []string{"effect-diagnostic"}
	obs.Effects[0].Contradictions = []string{"human detail"}
	obs.ArtifactBefore = &ArtifactSnapshot{Path: "before", Observed: true, Present: true, Bytes: []byte("before")}
	obs.ArtifactAfter = &ArtifactSnapshot{Path: "after", Observed: true, Present: true, Bytes: []byte("after")}
	in.Capture.Pathspecs[0] = "caller-mutated"
	in.Capture.ClaimIDs[0] = "caller-mutated"
	obsWrite(t, dir, "kept.txt", "late worktree edit\n", 0o755)
	e := s2Effect(t, obs, "kept.txt")
	if string(e.Bytes.Preimage) != "v1\n" || string(e.Bytes.Postimage) != "v2\n" ||
		e.Effect.PostimageSHA256 != sha256Hex([]byte("v2\n")) || e.Effect.NewMode != gitutil.ModeRegular {
		t.Fatalf("observation followed late live changes: %+v", e)
	}
	rec := &obsCollector{}
	restore := SetRecorder(rec)
	defer restore()
	Emit(obs)
	if len(rec.got) != 1 {
		t.Fatalf("recorder received %d observations", len(rec.got))
	}
	copy := &rec.got[0]
	copy.PatchBytes[0] = '!'
	copy.Capture.Pathspecs[0] = "recorder-mutated"
	copy.Capture.ClaimIDs[0] = "recorder-mutated"
	copy.Reasons[0] = "recorder-mutated"
	copy.Effects[0].Effect.Path = "recorder-mutated"
	copy.Effects[0].Bytes.Preimage[0] = '!'
	copy.Effects[0].Bytes.Postimage[0] = '!'
	copy.Effects[0].ReasonCodes[0] = "recorder-mutated"
	copy.Effects[0].Contradictions[0] = "recorder-mutated"
	copy.ArtifactBefore.Path = "recorder-mutated"
	copy.ArtifactBefore.Bytes[0] = '!'
	copy.ArtifactAfter.Path = "recorder-mutated"
	copy.ArtifactAfter.Bytes[0] = '!'
	if string(obs.PatchBytes) != in.Patch ||
		obs.Capture.Pathspecs[0] != "kept.txt" || obs.Capture.ClaimIDs[0] != "claim-1" ||
		obs.Reasons[0] != "diagnostic" || obs.Effects[0].Effect.Path != "kept.txt" ||
		string(obs.Effects[0].Bytes.Preimage) != "v1\n" || string(obs.Effects[0].Bytes.Postimage) != "v2\n" ||
		obs.Effects[0].ReasonCodes[0] != "effect-diagnostic" ||
		obs.Effects[0].Contradictions[0] != "human detail" ||
		obs.ArtifactBefore.Path != "before" || string(obs.ArtifactBefore.Bytes) != "before" ||
		obs.ArtifactAfter.Path != "after" || string(obs.ArtifactAfter.Bytes) != "after" {
		t.Fatalf("producer observation shares mutable recorder or input storage: %+v", obs)
	}
}

func TestRGAS2ParentCreatedPathsAreCapturedAndIsolated(t *testing.T) {
	in := Input{
		Capture:            CaptureDescriptor{Mode: CaptureModeNoCapture},
		ParentCreatedPaths: []string{"z-parent.txt", "a-parent.txt"},
	}
	obs := Observe(in)
	if strings.Join(obs.ParentCreatedPaths, ",") != "a-parent.txt,z-parent.txt" ||
		strings.Join(in.ParentCreatedPaths, ",") != "z-parent.txt,a-parent.txt" {
		t.Fatalf("exclusions were not sorted in an independent copy: input=%v observation=%v",
			in.ParentCreatedPaths, obs.ParentCreatedPaths)
	}
	in.ParentCreatedPaths[0] = "late-caller-edit"
	if obs.ParentCreatedPaths[1] != "z-parent.txt" {
		t.Fatal("captured exclusions alias producer input")
	}
	if obs.Reference.Kind != ReferenceKindUnavailable || obs.PreflightError() != nil ||
		!obsHasReason(obs.Reasons, ReasonPatchMissing) || !obsHasReason(obs.Reasons, ReasonReferenceNotDurable) {
		t.Fatalf("exclusions changed no-capture or preflight behavior: %+v", obs)
	}
	rec := &obsCollector{}
	restore := SetRecorder(rec)
	defer restore()
	Emit(obs)
	if len(rec.got) != 1 || strings.Join(rec.got[0].ParentCreatedPaths, ",") != "a-parent.txt,z-parent.txt" {
		t.Fatalf("recorder lost captured exclusions: %+v", rec.got)
	}
	rec.got[0].ParentCreatedPaths[0] = "recorder-edit"
	obs.ParentCreatedPaths[1] = "producer-edit"
	if obs.ParentCreatedPaths[0] != "a-parent.txt" || rec.got[0].ParentCreatedPaths[1] != "z-parent.txt" {
		t.Fatal("recorder and producer share mutable exclusions")
	}
	empty := Observe(Input{Capture: CaptureDescriptor{Mode: CaptureModeNoCapture}})
	if empty.ParentCreatedPaths == nil || len(empty.ParentCreatedPaths) != 0 {
		t.Fatalf("missing exclusions should normalize to an empty set: %v", empty.ParentCreatedPaths)
	}
}
