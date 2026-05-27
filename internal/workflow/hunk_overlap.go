package workflow

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/store"
)

const hunkOverlapNearbyWindow = 3

type HunkOverlapClassification string

const (
	HunkOverlapNone        HunkOverlapClassification = "none"
	HunkOverlapContextOnly HunkOverlapClassification = "context-only"
	HunkOverlapEditOverlap HunkOverlapClassification = "edit-overlap"
	HunkOverlapTargetGone  HunkOverlapClassification = "target-deleted"
	HunkOverlapPathMoved   HunkOverlapClassification = "path-moved"
	HunkOverlapUnknown     HunkOverlapClassification = "unknown"
)

type HunkOverlapResult struct {
	Classification HunkOverlapClassification `json:"classification"`
	NearbyWindow   int                       `json:"nearby_window"`
	Hunks          []HunkOverlap             `json:"hunks"`
}

type HunkOverlap struct {
	Path                 string                    `json:"path"`
	FeatureHunkID        string                    `json:"feature_hunk_id"`
	Overlap              HunkOverlapClassification `json:"overlap"`
	UpstreamHunksNearby  int                       `json:"upstream_hunks_nearby"`
	FeatureOldStart      int                       `json:"feature_old_start"`
	FeatureOldLineCount  int                       `json:"feature_old_line_count"`
	UpstreamOldStart     int                       `json:"upstream_old_start,omitempty"`
	UpstreamOldLineCount int                       `json:"upstream_old_line_count,omitempty"`
}

type patchHunkRange struct {
	Path     string
	OldStart int
	OldLen   int
	NewStart int
	NewLen   int
	Header   string
}

func DetectHunkOverlap(repoRoot, featurePatch, baseCommit, upstreamCommit string, novelty FileNoveltyResult) (HunkOverlapResult, error) {
	if baseCommit == "" || upstreamCommit == "" || strings.TrimSpace(featurePatch) == "" {
		return HunkOverlapResult{Classification: HunkOverlapUnknown, NearbyWindow: hunkOverlapNearbyWindow, Hunks: []HunkOverlap{}}, nil
	}
	eligible := map[string]bool{}
	for _, p := range novelty.Paths {
		if p.FeatureAction == FileNoveltyActionModify && p.UpstreamState == FileNoveltyUpstreamPresent {
			eligible[p.Path] = true
		}
	}
	if len(eligible) == 0 {
		return HunkOverlapResult{Classification: HunkOverlapNone, NearbyWindow: hunkOverlapNearbyWindow, Hunks: []HunkOverlap{}}, nil
	}
	featureHunks := filterHunksByPath(parsePatchHunks(featurePatch), eligible)
	if len(featureHunks) == 0 {
		return HunkOverlapResult{Classification: HunkOverlapUnknown, NearbyWindow: hunkOverlapNearbyWindow, Hunks: []HunkOverlap{}}, nil
	}
	upstreamPatch, err := gitDiffPatch(repoRoot, baseCommit, upstreamCommit)
	if err != nil {
		return HunkOverlapResult{}, err
	}
	upstreamHunks := groupHunksByPath(parsePatchHunks(upstreamPatch))
	out := make([]HunkOverlap, 0, len(featureHunks))
	for _, fh := range featureHunks {
		classification := HunkOverlapNone
		nearby := 0
		best := patchHunkRange{}
		for _, uh := range upstreamHunks[fh.Path] {
			if rangesOverlap(fh.OldStart, normalizedLen(fh.OldLen), uh.OldStart, normalizedLen(uh.OldLen)) {
				classification = HunkOverlapEditOverlap
				nearby++
				best = uh
				break
			}
			if rangesNear(fh.OldStart, normalizedLen(fh.OldLen), uh.OldStart, normalizedLen(uh.OldLen), hunkOverlapNearbyWindow) {
				nearby++
				if classification != HunkOverlapContextOnly {
					classification = HunkOverlapContextOnly
					best = uh
				}
			}
		}
		out = append(out, HunkOverlap{
			Path:                 fh.Path,
			FeatureHunkID:        hunkID(fh),
			Overlap:              classification,
			UpstreamHunksNearby:  nearby,
			FeatureOldStart:      fh.OldStart,
			FeatureOldLineCount:  fh.OldLen,
			UpstreamOldStart:     best.OldStart,
			UpstreamOldLineCount: best.OldLen,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].FeatureHunkID < out[j].FeatureHunkID
	})
	return HunkOverlapResult{Classification: overallHunkOverlap(out), NearbyWindow: hunkOverlapNearbyWindow, Hunks: out}, nil
}

func HunkOverlapEvidence(slug, upstreamRef, upstreamCommit, baseCommit, rawVerdict string, result HunkOverlapResult) store.ReconcileEvidence {
	paths := make([]string, 0, len(result.Hunks))
	ops := make([]string, 0, len(result.Hunks)+1)
	ops = append(ops, fmt.Sprintf("nearby-window=%d", result.NearbyWindow))
	for _, h := range result.Hunks {
		paths = append(paths, h.Path)
		ops = append(ops, fmt.Sprintf("%s:%s:%s:nearby=%d", h.Path, h.FeatureHunkID, h.Overlap, h.UpstreamHunksNearby))
	}
	confidence := store.EvidenceConfidenceMedium
	if result.Classification == HunkOverlapUnknown {
		confidence = store.EvidenceConfidenceUnknown
	} else if result.Classification == HunkOverlapEditOverlap {
		confidence = store.EvidenceConfidenceHigh
	}
	entry := store.ReconcileEvidence{
		SchemaVersion:        store.ReconcileEvidenceSchemaVersion,
		FeatureSlug:          slug,
		UpstreamRef:          upstreamRef,
		UpstreamCommit:       upstreamCommit,
		BaseCommit:           baseCommit,
		RawReconcileVerdict:  rawVerdict,
		Phase:                store.EvidencePhase35,
		EvidenceKind:         store.EvidenceKindHunkOverlap,
		Confidence:           confidence,
		MatchedPaths:         paths,
		MatchedOperations:    ops,
		MatchOrigin:          store.EvidenceMatchOriginUnknown,
		UpstreamCommitRefs:   []string{},
		PreReconcilePresence: store.EvidencePresencePresent,
		RequiresConfirmation: false,
		ReasonCode:           string(result.Classification),
	}
	entry.AttemptID = store.ComputeAttemptID(entry)
	return entry
}

func parsePatchHunks(patch string) []patchHunkRange {
	var hunks []patchHunkRange
	path := ""
	for _, line := range strings.Split(patch, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			_, newPath := parseDiffGitPaths(line)
			path = newPath
		case strings.HasPrefix(line, "+++ "):
			newPath := cleanPatchPath(strings.TrimSpace(strings.TrimPrefix(line, "+++ ")))
			if newPath != "" {
				path = newPath
			}
		case strings.HasPrefix(line, "@@ ") && path != "":
			oldStart, oldLen, newStart, newLen, ok := parseUnifiedRange(line)
			if ok {
				hunks = append(hunks, patchHunkRange{Path: path, OldStart: oldStart, OldLen: oldLen, NewStart: newStart, NewLen: newLen, Header: line})
			}
		}
	}
	return hunks
}

func parseUnifiedRange(header string) (int, int, int, int, bool) {
	fields := strings.Fields(header)
	if len(fields) < 3 || !strings.HasPrefix(fields[1], "-") || !strings.HasPrefix(fields[2], "+") {
		return 0, 0, 0, 0, false
	}
	oldStart, oldLen, ok1 := parseRangeToken(strings.TrimPrefix(fields[1], "-"))
	newStart, newLen, ok2 := parseRangeToken(strings.TrimPrefix(fields[2], "+"))
	return oldStart, oldLen, newStart, newLen, ok1 && ok2
}

func parseRangeToken(token string) (int, int, bool) {
	parts := strings.SplitN(token, ",", 2)
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	length := 1
	if len(parts) == 2 {
		length, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, false
		}
	}
	return start, length, true
}

func filterHunksByPath(hunks []patchHunkRange, eligible map[string]bool) []patchHunkRange {
	out := make([]patchHunkRange, 0, len(hunks))
	for _, h := range hunks {
		if eligible[h.Path] {
			out = append(out, h)
		}
	}
	return out
}

func groupHunksByPath(hunks []patchHunkRange) map[string][]patchHunkRange {
	out := map[string][]patchHunkRange{}
	for _, h := range hunks {
		out[h.Path] = append(out[h.Path], h)
	}
	return out
}

func gitDiffPatch(repoRoot, baseCommit, upstreamCommit string) (string, error) {
	cmd := exec.Command("git", "diff", "--no-color", "--unified=0", baseCommit, upstreamCommit)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func normalizedLen(n int) int {
	if n <= 0 {
		return 1
	}
	return n
}

func rangesOverlap(aStart, aLen, bStart, bLen int) bool {
	aEnd := aStart + aLen - 1
	bEnd := bStart + bLen - 1
	return aStart <= bEnd && bStart <= aEnd
}

func rangesNear(aStart, aLen, bStart, bLen, window int) bool {
	return rangesOverlap(aStart-window, aLen+(2*window), bStart, bLen)
}

func hunkID(h patchHunkRange) string {
	return fmt.Sprintf("old:%d,%d-new:%d,%d", h.OldStart, h.OldLen, h.NewStart, h.NewLen)
}

func overallHunkOverlap(hunks []HunkOverlap) HunkOverlapClassification {
	if len(hunks) == 0 {
		return HunkOverlapNone
	}
	hasContext := false
	for _, h := range hunks {
		switch h.Overlap {
		case HunkOverlapEditOverlap:
			return HunkOverlapEditOverlap
		case HunkOverlapContextOnly:
			hasContext = true
		case HunkOverlapTargetGone:
			return HunkOverlapTargetGone
		case HunkOverlapPathMoved:
			return HunkOverlapPathMoved
		case HunkOverlapUnknown:
			return HunkOverlapUnknown
		}
	}
	if hasContext {
		return HunkOverlapContextOnly
	}
	return HunkOverlapNone
}
