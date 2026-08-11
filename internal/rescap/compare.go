// Structural result comparison for `feature resource diff`
// (PRD-feature-resource-claims-and-capture-adapters §5.1's diff
// contract).
//
// The comparison names exactly which structural field changed —
// size_bytes, hash, file_count, total_bytes, combined_hash, file-set
// membership, or a per-file mode — and never produces a textual
// line-level diff of file content. Because mode participates in
// combined_hash, a chmod-only change is reported as a mode difference
// on the specific files[] entry whose mode changed, with hash and
// byte_count unchanged for that entry, rather than being folded into an
// undifferentiated "combined_hash differs" report.

package rescap

import (
	"fmt"
	"sort"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// CompareResults returns a sorted list of human-readable difference
// descriptions between a recorded result and a freshly-recomputed one.
// An empty list means unchanged.
func CompareResults(recorded, fresh store.CanonNode) []string {
	if fresh.IsNull() {
		return nil
	}
	var diffs []string
	recordedFiles, hasRecordedFiles := filesByPath(recorded)
	freshFiles, hasFreshFiles := filesByPath(fresh)
	if hasRecordedFiles && hasFreshFiles {
		diffs = append(diffs, compareFileSets(recordedFiles, freshFiles)...)
	}
	for _, field := range []string{"file_kind", "size_bytes", "hash", "file_count", "total_bytes", "combined_hash"} {
		a, aok := recorded.Field(field)
		b, bok := fresh.Field(field)
		if !aok && !bok {
			continue
		}
		if aok != bok {
			diffs = append(diffs, fmt.Sprintf("%s presence differs", field))
			continue
		}
		if !nodesEqual(a, b) {
			diffs = append(diffs, fmt.Sprintf("%s differs", field))
		}
	}
	sort.Strings(diffs)
	return diffs
}

// fileEntry is one files[] row reduced to the fields diff compares.
type fileEntry struct {
	Hash      string
	ByteCount uint64
	Mode      string
}

func filesByPath(node store.CanonNode) (map[string]fileEntry, bool) {
	files, ok := node.Field("files")
	if !ok || files.Kind != store.CanonKindArray {
		return nil, false
	}
	out := map[string]fileEntry{}
	for _, item := range files.Array {
		pathNode, hasPath := item.Field("path")
		if !hasPath {
			continue
		}
		var entry fileEntry
		if v, ok := item.Field("raw_sha256"); ok {
			entry.Hash = v.Str
		}
		if v, ok := item.Field("byte_count"); ok {
			entry.ByteCount = v.Uint
		}
		if v, ok := item.Field("mode"); ok {
			entry.Mode = v.Str
		}
		out[pathNode.Str] = entry
	}
	return out, true
}

func compareFileSets(recorded, fresh map[string]fileEntry) []string {
	var diffs []string
	for path, a := range recorded {
		b, ok := fresh[path]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("file removed: %s", path))
			continue
		}
		if a.Hash != b.Hash || a.ByteCount != b.ByteCount {
			diffs = append(diffs, fmt.Sprintf("file content differs: %s", path))
		}
		if a.Mode != b.Mode {
			diffs = append(diffs, fmt.Sprintf("file mode differs: %s (%s -> %s)", path, a.Mode, b.Mode))
		}
	}
	for path := range fresh {
		if _, ok := recorded[path]; !ok {
			diffs = append(diffs, fmt.Sprintf("file added: %s", path))
		}
	}
	return diffs
}

// nodesEqual is a structural equality check over the canonical node
// model.
func nodesEqual(a, b store.CanonNode) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case store.CanonKindNull:
		return true
	case store.CanonKindString:
		return a.Str == b.Str
	case store.CanonKindBool:
		return a.Bool == b.Bool
	case store.CanonKindUint:
		return a.Uint == b.Uint
	case store.CanonKindArray:
		if len(a.Array) != len(b.Array) {
			return false
		}
		for i := range a.Array {
			if !nodesEqual(a.Array[i], b.Array[i]) {
				return false
			}
		}
		return true
	case store.CanonKindObject:
		if len(a.Object) != len(b.Object) {
			return false
		}
		for i := range a.Object {
			if a.Object[i].Key != b.Object[i].Key {
				return false
			}
			if !nodesEqual(a.Object[i].Value, b.Object[i].Value) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
