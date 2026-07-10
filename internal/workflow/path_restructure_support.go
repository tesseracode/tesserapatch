package workflow

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/store"
)

const pathRestructureCandidateLimit = 5

type PathRestructureClassification string

const (
	PathRestructureNone          PathRestructureClassification = "none"
	PathRestructurePrefixMove    PathRestructureClassification = "prefix-move"
	PathRestructurePrefixSplit   PathRestructureClassification = "prefix-split"
	PathRestructureTargetDeleted PathRestructureClassification = "target-deleted"
	PathRestructureMixed         PathRestructureClassification = "mixed"
	PathRestructureUnknown       PathRestructureClassification = "unknown"
)

type PathRestructureThresholds struct {
	PrefixSplitMinFiles    int `json:"prefix_split_min_files"`
	PrefixSplitMinPrefixes int `json:"prefix_split_min_prefixes"`
	PrefixMoveMinFiles     int `json:"prefix_move_min_files"`
}

type PathRestructureInput struct {
	RepoRoot             string
	BaseCommit           string
	TargetCommit         string
	FeaturePaths         []string
	NameStatus           string
	RenameCopyNameStatus string
	Thresholds           PathRestructureThresholds
}

type PathRestructureEvidence struct {
	EvidenceKind         string                            `json:"evidence_kind"`
	Classification       PathRestructureClassification     `json:"classification"`
	OldPrefix            string                            `json:"old_prefix,omitempty"`
	CandidatePrefixes    []string                          `json:"candidate_prefixes,omitempty"`
	CandidateSupport     []PathRestructureCandidate        `json:"candidate_support,omitempty"`
	AffectedFeaturePaths []string                          `json:"affected_feature_paths,omitempty"`
	Confidence           store.ReconcileEvidenceConfidence `json:"confidence"`
	Thresholds           PathRestructureThresholds         `json:"thresholds"`
	MovedFileCount       int                               `json:"moved_file_count,omitempty"`
	DeletedFileCount     int                               `json:"deleted_file_count,omitempty"`
}

type PathRestructureCandidate struct {
	Prefix  string `json:"prefix"`
	Support int    `json:"support"`
}

type nameStatusEntry struct {
	status  byte
	oldPath string
	newPath string
}

type pathPrefixAggregate struct {
	oldPrefix    string
	moveSupport  map[string]int
	movedFiles   int
	deletedFiles int
	affected     map[string]bool
}

func DefaultPathRestructureThresholds() PathRestructureThresholds {
	return PathRestructureThresholds{
		PrefixSplitMinFiles:    store.DefaultPathRestructurePrefixSplitMinFiles,
		PrefixSplitMinPrefixes: store.DefaultPathRestructurePrefixSplitMinPrefixes,
		PrefixMoveMinFiles:     store.DefaultPathRestructurePrefixMoveMinFiles,
	}
}

func PathRestructureThresholdsFromConfig(cfg store.Config) PathRestructureThresholds {
	return normalizePathRestructureThresholds(PathRestructureThresholds{
		PrefixSplitMinFiles:    cfg.PathRestructurePrefixSplitMinFiles,
		PrefixSplitMinPrefixes: cfg.PathRestructurePrefixSplitMinPrefixes,
		PrefixMoveMinFiles:     cfg.PathRestructurePrefixMoveMinFiles,
	})
}

func gitDiffNameStatus(repoRoot, baseCommit, targetCommit string) (string, error) {
	cmd := exec.Command("git", "diff", "--name-status", "--find-renames", "--find-copies", baseCommit, targetCommit)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func parseNameStatusEntries(content string) ([]nameStatusEntry, bool) {
	var entries []nameStatusEntry
	malformed := false
	seen := map[string]bool{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 || fields[0] == "" {
			malformed = true
			continue
		}
		status := fields[0][0]
		entry := nameStatusEntry{status: status}
		switch status {
		case 'R', 'C':
			if len(fields) < 3 {
				malformed = true
				continue
			}
			entry.oldPath = normalizeRestructurePath(fields[1])
			entry.newPath = normalizeRestructurePath(fields[2])
		case 'D':
			entry.oldPath = normalizeRestructurePath(fields[1])
		default:
			continue
		}
		if entry.oldPath == "" || (status != 'D' && entry.newPath == "") {
			malformed = true
			continue
		}
		key := fmt.Sprintf("%c\x00%s\x00%s", entry.status, entry.oldPath, entry.newPath)
		if seen[key] {
			continue
		}
		seen[key] = true
		entries = append(entries, entry)
	}
	return entries, malformed
}

func normalizeFeaturePaths(paths []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		p = normalizeRestructurePath(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func normalizeRestructurePath(path string) string {
	path = strings.TrimSpace(strings.Trim(path, `"`))
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, "/")
	return path
}

func pathDirectoryPrefixes(path string) []string {
	path = normalizeRestructurePath(path)
	parts := strings.Split(path, "/")
	if len(parts) <= 1 {
		return nil
	}
	prefixes := make([]string, 0, len(parts)-1)
	for i := 1; i < len(parts); i++ {
		prefixes = append(prefixes, strings.Join(parts[:i], "/")+"/")
	}
	return prefixes
}

func pathDirPrefix(path string) string {
	path = normalizeRestructurePath(path)
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return ""
	}
	return path[:idx+1]
}

func anyPathHasPrefix(paths []string, prefix string) bool {
	for _, p := range paths {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

func candidatePrefixForMove(oldPrefix, oldPath, newPath string) string {
	suffix := strings.TrimPrefix(oldPath, oldPrefix)
	if suffix != "" && strings.HasSuffix(newPath, suffix) {
		return newPath[:len(newPath)-len(suffix)]
	}
	return pathDirPrefix(newPath)
}

func sortedPathRestructureCandidates(support map[string]int) []PathRestructureCandidate {
	out := make([]PathRestructureCandidate, 0, len(support))
	for prefix, count := range support {
		out = append(out, PathRestructureCandidate{Prefix: prefix, Support: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Support != out[j].Support {
			return out[i].Support > out[j].Support
		}
		return out[i].Prefix < out[j].Prefix
	})
	return out
}

func sortedBoolKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for v := range values {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
