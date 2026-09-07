package workflow

import (
	"fmt"
	"os/exec"
	"sort"

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
	paths, err := parsePatchNoveltyPaths(featurePatch)
	if err != nil {
		return FileNoveltyResult{}, fmt.Errorf("file novelty: cannot determine which paths the feature patch touches: %w", err)
	}
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

// parsePatchNoveltyPaths projects the strict normalized effect set onto
// the novelty view: one entry per record, carrying the canonical path and
// the change axis read from the record header (PI-5).
//
// It used to split `diff --git` on whitespace and dequote with
// strings.Trim(path, "\""), which dropped every Git C-quoted path and
// mis-attributed any path containing a space. Both classes are now either
// classified correctly or refused — the strict error is returned, never
// absorbed into a short list that would silently classify a patch from a
// subset of its effects.
func parsePatchNoveltyPaths(patch string) ([]PathNovelty, error) {
	views, err := patchEffectViews(patch)
	if err != nil {
		return nil, err
	}
	out := make([]PathNovelty, 0, len(views))
	for _, view := range views {
		action, ok := noveltyActionForChangeKind(view.ChangeKind)
		if !ok {
			return nil, fmt.Errorf("novelty classification: unmapped change kind %q for %q", view.ChangeKind, view.Path)
		}
		if view.Path == "" {
			continue
		}
		out = append(out, PathNovelty{Path: view.Path, FeatureAction: action})
	}
	return dedupePathNovelty(out), nil
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
