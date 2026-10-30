package candidate

import (
	"bytes"
	"fmt"
	"sort"
)

const (
	alignmentMaxBytes   = 4194304
	alignmentMaxRecords = 131072
	alignmentMaxCells   = 1048576
)

type refusal struct {
	code string
	path string
}

func (e *refusal) Error() string { return e.code }
func (e *refusal) Code() string  { return e.code }
func (e *refusal) Path() string  { return e.path }

type classification string

const (
	applicable classification = "applicable"
	present    classification = "present"
	conflict   classification = "conflict"
)

type insertionRun struct {
	effectOrdinal int
	runOrdinal    int
	boundary      int
	records       [][]byte
}

type alignedBoundary struct {
	boundary    int
	runOrdinals []int
	left, right int
	byteOffset  int
	records     [][]byte
	state       classification
}

type alignmentTables struct {
	width int
	f, g  []int32
	l     int32
}

func recordCount(body []byte) int {
	count := bytes.Count(body, []byte{'\n'})
	if len(body) != 0 && body[len(body)-1] != '\n' {
		count++
	}
	return count
}

func lineRecords(body []byte) [][]byte {
	out := make([][]byte, 0, recordCount(body))
	for len(body) != 0 {
		end := bytes.IndexByte(body, '\n') + 1
		if end == 0 {
			end = len(body)
		}
		out = append(out, body[:end])
		body = body[end:]
	}
	return out
}

func alignmentLimits(base, upstream []byte) error {
	if len(base) > alignmentMaxBytes || len(upstream) > alignmentMaxBytes {
		return &refusal{code: "alignment-limit"}
	}
	n, m := recordCount(base), recordCount(upstream)
	if n > alignmentMaxRecords || m > alignmentMaxRecords ||
		n+1 > alignmentMaxCells/(m+1) {
		return &refusal{code: "alignment-limit"}
	}
	return nil
}

func newAlignmentTables(base, upstream [][]byte) (alignmentTables, error) {
	n, m := len(base), len(upstream)
	if n > alignmentMaxRecords || m > alignmentMaxRecords || n+1 > alignmentMaxCells/(m+1) {
		return alignmentTables{}, &refusal{code: "alignment-limit"}
	}
	width := m + 1
	cells := (n + 1) * width
	f := make([]int32, cells)
	g := make([]int32, cells)
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if bytes.Equal(base[i-1], upstream[j-1]) {
				f[i*width+j] = f[(i-1)*width+j-1] + 1
			} else {
				f[i*width+j] = max(f[(i-1)*width+j], f[i*width+j-1])
			}
		}
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if bytes.Equal(base[i], upstream[j]) {
				g[i*width+j] = g[(i+1)*width+j+1] + 1
			} else {
				g[i*width+j] = max(g[(i+1)*width+j], g[i*width+j+1])
			}
		}
	}
	return alignmentTables{width: width, f: f, g: g, l: f[n*width+m]}, nil
}

func (t alignmentTables) unanimousBounds(base, upstream [][]byte, boundary int) (int, int, error) {
	left, right := -1, -1
	for cut := 0; cut <= len(upstream); cut++ {
		if t.f[boundary*t.width+cut]+t.g[boundary*t.width+cut] == t.l {
			if left < 0 {
				left = cut
			}
			right = cut
		}
	}
	if left < 0 {
		return 0, 0, fmt.Errorf("alignment has no optimal cut")
	}
	for i := range base {
		for j := range upstream {
			if bytes.Equal(base[i], upstream[j]) &&
				t.f[i*t.width+j]+1+t.g[(i+1)*t.width+j+1] == t.l &&
				((i < boundary && j >= left) || (i >= boundary && j < right)) {
				return 0, 0, &refusal{code: "alignment-ambiguous"}
			}
		}
	}
	return left, right, nil
}

func combineInsertionRuns(runs []insertionRun, baseRecords int) ([]alignedBoundary, error) {
	ordered := append([]insertionRun(nil), runs...)
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if a.boundary != b.boundary {
			return a.boundary < b.boundary
		}
		if a.effectOrdinal != b.effectOrdinal {
			return a.effectOrdinal < b.effectOrdinal
		}
		return a.runOrdinal < b.runOrdinal
	})
	var result []alignedBoundary
	seen := map[[2]int]bool{}
	for _, run := range ordered {
		if run.boundary < 0 || run.boundary > baseRecords || run.effectOrdinal < 1 || run.runOrdinal < 1 {
			return nil, &refusal{code: "fragment-boundary-underivable"}
		}
		key := [2]int{run.effectOrdinal, run.runOrdinal}
		if seen[key] {
			return nil, &refusal{code: "fragment-boundary-underivable"}
		}
		seen[key] = true
		if len(result) == 0 || result[len(result)-1].boundary != run.boundary {
			result = append(result, alignedBoundary{boundary: run.boundary})
		}
		target := &result[len(result)-1]
		target.runOrdinals = append(target.runOrdinals, run.runOrdinal)
		target.records = append(target.records, run.records...)
	}
	return result, nil
}

func classifyGap(gap, insertion [][]byte) classification {
	if len(gap) == 0 {
		return applicable
	}
	if len(gap) != len(insertion) {
		return conflict
	}
	for i := range gap {
		if !bytes.Equal(gap[i], insertion[i]) {
			return conflict
		}
	}
	return present
}

func alignInsertionRuns(base, upstream []byte, runs []insertionRun) ([]alignedBoundary, error) {
	if err := alignmentLimits(base, upstream); err != nil {
		return nil, err
	}
	b, u := lineRecords(base), lineRecords(upstream)
	boundaries, err := combineInsertionRuns(runs, len(b))
	if err != nil {
		return nil, err
	}
	tables, err := newAlignmentTables(b, u)
	if err != nil {
		return nil, err
	}
	for i := range boundaries {
		bound := &boundaries[i]
		left, right, err := tables.unanimousBounds(b, u, bound.boundary)
		if err != nil {
			return nil, err
		}
		if left == 0 && bound.boundary != 0 {
			return nil, &refusal{code: "alignment-left-unanchored"}
		}
		if right == len(u) && bound.boundary != len(b) {
			return nil, &refusal{code: "alignment-right-unanchored"}
		}
		for _, record := range bound.records {
			if !bytes.HasSuffix(record, []byte{'\n'}) &&
				(bound.boundary != len(b) || (len(base) != 0 && base[len(base)-1] != '\n')) {
				return nil, &refusal{code: "record-terminator-unsupported"}
			}
		}
		bound.left, bound.right = left, right
		for _, record := range u[:left] {
			bound.byteOffset += len(record)
		}
		bound.state = classifyGap(u[left:right], bound.records)
	}
	for i := 1; i < len(boundaries); i++ {
		if boundaries[i-1].right >= boundaries[i].left {
			return nil, &refusal{code: "alignment-range-overlap"}
		}
	}
	return boundaries, nil
}
