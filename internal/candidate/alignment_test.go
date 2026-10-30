package candidate

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"unsafe"
)

func refusalCode(err error) string {
	var r *refusal
	if errors.As(err, &r) {
		return r.code
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

func testRun(boundary int, insertion string) insertionRun {
	return insertionRun{effectOrdinal: 1, runOrdinal: 1, boundary: boundary, records: lineRecords([]byte(insertion))}
}

func TestS3ExactRecordsAndClassification(t *testing.T) {
	records := lineRecords([]byte("foo\nfoo\r\nfoo"))
	if len(records) != 3 || bytes.Equal(records[0], records[1]) || bytes.Equal(records[0], records[2]) || bytes.Equal(records[1], records[2]) {
		t.Fatal("record terminators were normalized")
	}
	for _, row := range []struct {
		upstream string
		want     classification
	}{
		{"a\nb\n", applicable},
		{"a\nx\nb\n", present},
		{"a\n x\nb\n", conflict},
		{"a\nxx\nb\n", conflict},
		{"x\na\nb\n", applicable},
	} {
		got, err := alignInsertionRuns([]byte("a\nb\n"), []byte(row.upstream), []insertionRun{testRun(1, "x\n")})
		if err != nil || len(got) != 1 || got[0].state != row.want {
			t.Fatalf("%q: %+v %v, want %s", row.upstream, got, err, row.want)
		}
	}
	got, err := alignInsertionRuns([]byte("a\r\nb\n"), []byte("a\r\nb\n"), []insertionRun{testRun(1, "x\r\n")})
	if err != nil || got[0].state != applicable || got[0].byteOffset != 3 {
		t.Fatalf("mixed EOL refused/normalized: %+v %v", got, err)
	}
}

// This oracle enumerates alignments only in tiny tests, independently of F/G.
func oracleBounds(base, upstream [][]byte, boundary int) (int, int, bool) {
	type pair struct{ i, j int }
	best := -1
	bounds := map[[2]int]bool{}
	var visit func(int, int, []pair)
	visit = func(i, j int, matched []pair) {
		if len(matched) >= best {
			left, right := 0, len(upstream)
			for _, p := range matched {
				if p.i < boundary {
					left = p.j + 1
				} else {
					right = p.j
					break
				}
			}
			if len(matched) > best {
				best, bounds = len(matched), map[[2]int]bool{}
			}
			bounds[[2]int{left, right}] = true
		}
		for x := i; x < len(base); x++ {
			for y := j; y < len(upstream); y++ {
				if bytes.Equal(base[x], upstream[y]) {
					next := append(append([]pair(nil), matched...), pair{x, y})
					visit(x+1, y+1, next)
				}
			}
		}
	}
	visit(0, 0, nil)
	if len(bounds) != 1 {
		return 0, 0, false
	}
	for bound := range bounds {
		return bound[0], bound[1], true
	}
	panic("empty oracle")
}

func smallRecordLists() [][][]byte {
	out := [][][]byte{nil}
	for n := 1; n <= 4; n++ {
		for mask := 0; mask < 1<<n; mask++ {
			list := make([][]byte, n)
			for i := range list {
				list[i] = []byte{byte('a' + (mask>>i)&1), '\n'}
			}
			out = append(out, list)
		}
	}
	return out
}

func TestS3UnanimityMatchesExhaustiveOracle(t *testing.T) {
	lists := smallRecordLists()
	for _, base := range lists {
		for _, upstream := range lists {
			tables, err := newAlignmentTables(base, upstream)
			if err != nil {
				t.Fatal(err)
			}
			boundaries := make([]alignedBoundary, len(base)+1)
			for i := range boundaries {
				boundaries[i].boundary = i
			}
			if err := tables.mapBoundaryBounds(base, upstream, boundaries); err != nil {
				t.Fatal(err)
			}
			for boundary := 0; boundary <= len(base); boundary++ {
				left, right, unanimous := oracleBounds(base, upstream, boundary)
				got := boundaries[boundary]
				if !unanimous {
					if got.unanimous {
						t.Fatalf("non-unanimous %q/%q b=%d accepted: %d,%d", base, upstream, boundary, got.left, got.right)
					}
				} else if !got.unanimous || got.left != left || got.right != right {
					t.Fatalf("%q/%q b=%d got %d,%d/%v want %d,%d", base, upstream, boundary, got.left, got.right, got.unanimous, left, right)
				}
			}
		}
	}
}

func TestS3RuleOrderSentinelsAndTerminators(t *testing.T) {
	for _, row := range []struct {
		name, base, upstream, insertion, code string
		boundary                              int
	}{
		{"ambiguous-before-unanchored", "a\na\n", "a\n", "x\n", "alignment-ambiguous", 1},
		{"left", "a\nb\n", "b\n", "x\n", "alignment-left-unanchored", 1},
		{"right", "a\nb\n", "a\n", "x\n", "alignment-right-unanchored", 1},
		{"no-newline-middle", "a\nb\n", "a\nb\n", "x", "record-terminator-unsupported", 1},
		{"no-newline-base-tail", "a", "a", "x", "record-terminator-unsupported", 1},
		{"terminal", "a\n", "a\n", "x", "", 1},
		{"start", "a\n", "a\n", "x\n", "", 0},
		{"empty-base", "", "", "x", "", 0},
	} {
		t.Run(row.name, func(t *testing.T) {
			_, err := alignInsertionRuns([]byte(row.base), []byte(row.upstream), []insertionRun{testRun(row.boundary, row.insertion)})
			if refusalCode(err) != row.code {
				t.Fatalf("got %v want %s", err, row.code)
			}
		})
	}
	_, err := alignInsertionRuns(bytes.Repeat([]byte("a"), alignmentMaxBytes+1), []byte("x\n"), []insertionRun{testRun(1, "x")})
	if refusalCode(err) != "alignment-limit" {
		t.Fatalf("limits did not win: %v", err)
	}
}

func TestS3StrictSeparationAndCanonicalCombination(t *testing.T) {
	for _, upstream := range []string{"a\nz\n", "a\nother\nz\n"} {
		_, err := alignInsertionRuns([]byte("a\nb\nc\nz\n"), []byte(upstream), []insertionRun{
			{effectOrdinal: 1, runOrdinal: 1, boundary: 1, records: lineRecords([]byte("x\n"))},
			{effectOrdinal: 1, runOrdinal: 2, boundary: 3, records: lineRecords([]byte("y\n"))},
		})
		if refusalCode(err) != "alignment-range-overlap" {
			t.Fatalf("collapsed gaps %q: %v", upstream, err)
		}
	}
	runs := []insertionRun{
		{effectOrdinal: 2, runOrdinal: 1, boundary: 1, records: lineRecords([]byte("c\n"))},
		{effectOrdinal: 1, runOrdinal: 2, boundary: 1, records: lineRecords([]byte("b\n"))},
		{effectOrdinal: 1, runOrdinal: 1, boundary: 1, records: lineRecords([]byte("a\n"))},
	}
	got, err := alignInsertionRuns([]byte("left\nright\n"), []byte("left\na\nb\nc\nright\n"), runs)
	if err != nil || len(got) != 1 || got[0].state != present || string(bytes.Join(got[0].records, nil)) != "a\nb\nc\n" {
		t.Fatalf("combination: %+v %v", got, err)
	}
	again, err := alignInsertionRuns([]byte("left\nright\n"), []byte("left\na\nb\nc\nright\n"), runs)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatal("alignment is not deterministic")
	}
}

func TestS3ManyBoundariesAtCellCeiling(t *testing.T) {
	var body []byte
	for i := 0; i < 1023; i++ {
		body = append(body, []byte(fmt.Sprintf("%04d\n", i))...)
	}
	runs := make([]insertionRun, 1024)
	for i := range runs {
		runs[i] = insertionRun{effectOrdinal: 1, runOrdinal: i + 1, boundary: i, records: [][]byte{[]byte("insert\n")}}
	}
	got, err := alignInsertionRuns(body, body, runs)
	if err != nil || len(got) != len(runs) {
		t.Fatalf("ceiling-scale boundaries: %d %v", len(got), err)
	}
	for i, bound := range got {
		if !bound.unanimous || bound.left != i || bound.right != i || bound.state != applicable {
			t.Fatalf("boundary %d: %+v", i, bound)
		}
	}
}

func TestS3ClosedResourceBounds(t *testing.T) {
	for _, row := range []struct {
		name           string
		base, upstream []byte
	}{
		{"bytes", bytes.Repeat([]byte{'a'}, alignmentMaxBytes+1), nil},
		{"records", bytes.Repeat([]byte{'\n'}, alignmentMaxRecords+1), nil},
		{"cells", bytes.Repeat([]byte("a\n"), 1024), bytes.Repeat([]byte("a\n"), 1024)},
	} {
		t.Run(row.name, func(t *testing.T) {
			if _, err := alignInsertionRuns(row.base, row.upstream, nil); refusalCode(err) != "alignment-limit" {
				t.Fatalf("limit not refused: %v", err)
			}
		})
	}
	body := lineRecords(bytes.Repeat([]byte("a\n"), 1023))
	tables, err := newAlignmentTables(body, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables.f) != alignmentMaxCells || len(tables.g) != alignmentMaxCells ||
		uintptr(len(tables.f)+len(tables.g))*unsafe.Sizeof(int32(0)) != 8<<20 {
		t.Fatal("two-matrix budget differs from 8 MiB")
	}
}

func TestS3ReplayProofIsIndependentOfExtraction(t *testing.T) {
	run := testRun(1, "x\n")
	got, err := replayInsertionDigest([]byte("a\nb\n"), []insertionRun{run})
	if err != nil || got != digestBytes([]byte("a\nx\nb\n")) {
		t.Fatalf("replay: %s %v", got, err)
	}
	run.boundary = 0
	wrong, err := replayInsertionDigest([]byte("a\nb\n"), []insertionRun{run})
	if err != nil || wrong == got {
		t.Fatal("wrong extracted boundary escaped postimage proof")
	}
}

func TestS3ClassificationGuardSensitivity(t *testing.T) {
	validate := func(classify func([][]byte, [][]byte) classification) error {
		for _, row := range []struct {
			gap, insertion string
			want           classification
		}{
			{"", "x\n", applicable}, {"x\n", "x\n", present},
			{" x\n", "x\n", conflict}, {"xx\n", "x\n", conflict}, {"x", "x\n", conflict},
		} {
			if classify(lineRecords([]byte(row.gap)), lineRecords([]byte(row.insertion))) != row.want {
				return fmt.Errorf("incorrect exact-gap classification")
			}
		}
		return nil
	}
	if err := validate(classifyGap); err != nil {
		t.Fatal(err)
	}
	for _, wrong := range []func([][]byte, [][]byte) classification{
		func(gap, insertion [][]byte) classification {
			if bytes.Equal(bytes.TrimSpace(bytes.Join(gap, nil)), bytes.TrimSpace(bytes.Join(insertion, nil))) {
				return present
			}
			return classifyGap(gap, insertion)
		},
		func(gap, insertion [][]byte) classification {
			if bytes.Contains(bytes.Join(gap, nil), bytes.TrimSpace(bytes.Join(insertion, nil))) {
				return present
			}
			return classifyGap(gap, insertion)
		},
	} {
		if err := validate(wrong); err == nil {
			t.Fatal("classification guard accepted a weakened matcher")
		}
	}
}
