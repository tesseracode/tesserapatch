package workflow

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/store"
)

type FileNoveltyClassification string

const (
	FileNoveltyAllNewFiles           FileNoveltyClassification = "all-new-files"
	FileNoveltyMixedAdditive         FileNoveltyClassification = "mixed-additive"
	FileNoveltyModifiesExistingFiles FileNoveltyClassification = "modifies-existing-files"
	FileNoveltyDeletesOrRenames      FileNoveltyClassification = "deletes-or-renames"
	FileNoveltyUnknown               FileNoveltyClassification = "unknown"
)

type FileNoveltyAction string

const (
	FileNoveltyActionCreate FileNoveltyAction = "create"
	FileNoveltyActionModify FileNoveltyAction = "modify"
	FileNoveltyActionDelete FileNoveltyAction = "delete"
	FileNoveltyActionRename FileNoveltyAction = "rename"
)

type FileNoveltyUpstreamState string

const (
	FileNoveltyUpstreamAbsent  FileNoveltyUpstreamState = "absent"
	FileNoveltyUpstreamPresent FileNoveltyUpstreamState = "present"
)

type FileNoveltyResult struct {
	Paths          []PathNovelty             `json:"paths"`
	Classification FileNoveltyClassification `json:"classification"`
}

type PathNovelty struct {
	Path          string                   `json:"path"`
	FeatureAction FileNoveltyAction        `json:"feature_action"`
	UpstreamState FileNoveltyUpstreamState `json:"upstream_state"`
}

func ClassifyFileNovelty(featurePatch string, upstreamCommit, baseCommit string, repoRoot string) (FileNoveltyResult, error) {
	_ = baseCommit
	paths := parsePatchNoveltyPaths(featurePatch)
	if len(paths) == 0 {
		return FileNoveltyResult{Paths: []PathNovelty{}, Classification: FileNoveltyUnknown}, nil
	}
	if upstreamCommit == "" {
		upstreamCommit = "HEAD"
	}
	for i := range paths {
		present, err := pathPresentAtCommit(repoRoot, upstreamCommit, paths[i].Path)
		if err != nil {
			return FileNoveltyResult{}, err
		}
		if present {
			paths[i].UpstreamState = FileNoveltyUpstreamPresent
		} else {
			paths[i].UpstreamState = FileNoveltyUpstreamAbsent
		}
	}
	sort.Slice(paths, func(i, j int) bool {
		if paths[i].Path == paths[j].Path {
			return paths[i].FeatureAction < paths[j].FeatureAction
		}
		return paths[i].Path < paths[j].Path
	})
	return FileNoveltyResult{Paths: paths, Classification: classifyNovelty(paths)}, nil
}

func FileNoveltyEvidence(slug, upstreamRef, upstreamCommit, baseCommit, rawVerdict string, result FileNoveltyResult) store.ReconcileEvidence {
	matchedPaths := make([]string, 0, len(result.Paths))
	for _, p := range result.Paths {
		matchedPaths = append(matchedPaths, p.Path)
	}
	presence := store.EvidencePresenceUnknown
	if len(result.Paths) > 0 {
		allAbsent := true
		allPresent := true
		for _, p := range result.Paths {
			if p.UpstreamState != FileNoveltyUpstreamAbsent {
				allAbsent = false
			}
			if p.UpstreamState != FileNoveltyUpstreamPresent {
				allPresent = false
			}
		}
		switch {
		case allAbsent:
			presence = store.EvidencePresenceAbsent
		case allPresent:
			presence = store.EvidencePresencePresent
		}
	}
	confidence := store.EvidenceConfidenceMedium
	if result.Classification == FileNoveltyAllNewFiles {
		confidence = store.EvidenceConfidenceHigh
	} else if result.Classification == FileNoveltyUnknown {
		confidence = store.EvidenceConfidenceUnknown
	}
	entry := store.ReconcileEvidence{
		SchemaVersion:        store.ReconcileEvidenceSchemaVersion,
		FeatureSlug:          slug,
		UpstreamRef:          upstreamRef,
		UpstreamCommit:       upstreamCommit,
		BaseCommit:           baseCommit,
		RawReconcileVerdict:  rawVerdict,
		Phase:                store.EvidencePhase35,
		EvidenceKind:         store.EvidenceKindFileNovelty,
		Confidence:           confidence,
		MatchedPaths:         matchedPaths,
		MatchedOperations:    []string{},
		MatchOrigin:          store.EvidenceMatchOriginUnknown,
		UpstreamCommitRefs:   []string{},
		PreReconcilePresence: presence,
		RequiresConfirmation: true,
		ReasonCode:           string(result.Classification),
	}
	entry.AttemptID = store.ComputeAttemptID(entry)
	return entry
}

func parsePatchNoveltyPaths(patch string) []PathNovelty {
	var out []PathNovelty
	var current *PathNovelty
	flush := func() {
		if current != nil && current.Path != "" {
			out = append(out, *current)
		}
		current = nil
	}
	for _, line := range strings.Split(patch, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flush()
			oldPath, newPath := parseDiffGitPaths(line)
			path := newPath
			if path == "" {
				path = oldPath
			}
			current = &PathNovelty{Path: path, FeatureAction: FileNoveltyActionModify}
		case current == nil:
			continue
		case strings.HasPrefix(line, "new file mode "):
			current.FeatureAction = FileNoveltyActionCreate
		case strings.HasPrefix(line, "deleted file mode "):
			current.FeatureAction = FileNoveltyActionDelete
		case strings.HasPrefix(line, "rename from "):
			current.FeatureAction = FileNoveltyActionRename
		case strings.HasPrefix(line, "rename to "):
			current.FeatureAction = FileNoveltyActionRename
			current.Path = strings.TrimSpace(strings.TrimPrefix(line, "rename to "))
		case strings.HasPrefix(line, "--- /dev/null"):
			current.FeatureAction = FileNoveltyActionCreate
		case strings.HasPrefix(line, "+++ /dev/null"):
			current.FeatureAction = FileNoveltyActionDelete
		case strings.HasPrefix(line, "--- ") && current.FeatureAction == FileNoveltyActionModify:
			oldPath := cleanPatchPath(strings.TrimSpace(strings.TrimPrefix(line, "--- ")))
			if oldPath != "" && current.Path == "" {
				current.Path = oldPath
			}
		case strings.HasPrefix(line, "+++ "):
			newPath := cleanPatchPath(strings.TrimSpace(strings.TrimPrefix(line, "+++ ")))
			if newPath != "" && current.FeatureAction != FileNoveltyActionDelete {
				current.Path = newPath
			}
		}
	}
	flush()
	return dedupePathNovelty(out)
}

func classifyNovelty(paths []PathNovelty) FileNoveltyClassification {
	if len(paths) == 0 {
		return FileNoveltyUnknown
	}
	hasCreate := false
	hasModify := false
	for _, p := range paths {
		switch p.FeatureAction {
		case FileNoveltyActionDelete, FileNoveltyActionRename:
			return FileNoveltyDeletesOrRenames
		case FileNoveltyActionCreate:
			hasCreate = true
		case FileNoveltyActionModify:
			hasModify = true
		}
	}
	if hasCreate && hasModify {
		return FileNoveltyMixedAdditive
	}
	if hasCreate {
		for _, p := range paths {
			if p.UpstreamState != FileNoveltyUpstreamAbsent {
				return FileNoveltyMixedAdditive
			}
		}
		return FileNoveltyAllNewFiles
	}
	if hasModify {
		return FileNoveltyModifiesExistingFiles
	}
	return FileNoveltyUnknown
}

func parseDiffGitPaths(line string) (string, string) {
	parts := strings.Fields(line)
	if len(parts) < 4 {
		return "", ""
	}
	return cleanPatchPath(parts[2]), cleanPatchPath(parts[3])
}

func cleanPatchPath(path string) string {
	path = strings.Trim(path, "\"")
	if path == "/dev/null" {
		return ""
	}
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	return path
}

func dedupePathNovelty(paths []PathNovelty) []PathNovelty {
	seen := map[string]PathNovelty{}
	for _, p := range paths {
		key := string(p.FeatureAction) + "\x00" + p.Path
		seen[key] = p
	}
	out := make([]PathNovelty, 0, len(seen))
	for _, p := range seen {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].FeatureAction < out[j].FeatureAction
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func pathPresentAtCommit(repoRoot, commit, path string) (bool, error) {
	cmd := exec.Command("git", "cat-file", "-e", fmt.Sprintf("%s:%s", commit, path))
	cmd.Dir = repoRoot
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 1 || exitErr.ExitCode() == 128 {
			return false, nil
		}
	}
	return false, err
}
