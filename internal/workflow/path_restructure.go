package workflow

import (
	"fmt"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func DetectPathRestructure(input PathRestructureInput) (*PathRestructureEvidence, error) {
	thresholds := normalizePathRestructureThresholds(input.Thresholds)
	featurePaths := normalizeFeaturePaths(input.FeaturePaths)
	if len(featurePaths) == 0 {
		return basePathRestructureEvidence(PathRestructureUnknown, thresholds), nil
	}
	nameStatus := input.RenameCopyNameStatus
	if strings.TrimSpace(input.NameStatus) != "" && input.NameStatus != input.RenameCopyNameStatus {
		nameStatus += "\n" + input.NameStatus
	}
	if strings.TrimSpace(nameStatus) == "" && input.RepoRoot != "" && input.BaseCommit != "" && input.TargetCommit != "" {
		out, err := gitDiffNameStatus(input.RepoRoot, input.BaseCommit, input.TargetCommit)
		if err != nil {
			return nil, err
		}
		nameStatus = out
	}
	if strings.TrimSpace(nameStatus) == "" {
		return basePathRestructureEvidence(PathRestructureNone, thresholds), nil
	}
	entries, malformed := parseNameStatusEntries(nameStatus)
	if malformed {
		return basePathRestructureEvidence(PathRestructureUnknown, thresholds), nil
	}
	if len(entries) == 0 {
		return basePathRestructureEvidence(PathRestructureNone, thresholds), nil
	}

	aggregates := map[string]*pathPrefixAggregate{}
	ensure := func(prefix string) *pathPrefixAggregate {
		agg := aggregates[prefix]
		if agg == nil {
			agg = &pathPrefixAggregate{oldPrefix: prefix, moveSupport: map[string]int{}, affected: map[string]bool{}}
			aggregates[prefix] = agg
		}
		for _, p := range featurePaths {
			if strings.HasPrefix(p, prefix) {
				agg.affected[p] = true
			}
		}
		return agg
	}

	for _, entry := range entries {
		switch entry.status {
		case 'R', 'C':
			for _, prefix := range pathDirectoryPrefixes(entry.oldPath) {
				if !anyPathHasPrefix(featurePaths, prefix) {
					continue
				}
				candidate := candidatePrefixForMove(prefix, entry.oldPath, entry.newPath)
				if candidate == "" || candidate == prefix {
					continue
				}
				agg := ensure(prefix)
				agg.moveSupport[candidate]++
				agg.movedFiles++
			}
		case 'D':
			for _, featurePath := range featurePaths {
				if featurePath != entry.oldPath {
					continue
				}
				prefix := pathDirPrefix(featurePath)
				if prefix == "" {
					prefix = featurePath
				}
				agg := ensure(prefix)
				agg.deletedFiles++
			}
		}
	}

	best := basePathRestructureEvidence(PathRestructureNone, thresholds)
	for _, agg := range aggregates {
		ev := evidenceForPathPrefixAggregate(agg, thresholds)
		if betterPathRestructureEvidence(ev, best) {
			copy := ev
			best = &copy
		}
	}
	return best, nil
}

func PathRestructureReconcileEvidence(slug, upstreamRef, upstreamCommit, baseCommit, rawVerdict string, result PathRestructureEvidence) store.ReconcileEvidence {
	ops := []string{
		fmt.Sprintf("old_prefix=%s", result.OldPrefix),
		fmt.Sprintf("prefix_split_min_files=%d", result.Thresholds.PrefixSplitMinFiles),
		fmt.Sprintf("prefix_split_min_prefixes=%d", result.Thresholds.PrefixSplitMinPrefixes),
		fmt.Sprintf("prefix_move_min_files=%d", result.Thresholds.PrefixMoveMinFiles),
		fmt.Sprintf("moved_files=%d", result.MovedFileCount),
		fmt.Sprintf("deleted_files=%d", result.DeletedFileCount),
	}
	if len(result.CandidatePrefixes) > 0 {
		ops = append(ops, "candidate_prefixes="+strings.Join(result.CandidatePrefixes, "|"))
		support := make([]string, 0, len(result.CandidateSupport))
		for _, c := range result.CandidateSupport {
			support = append(support, fmt.Sprintf("%s:%d", c.Prefix, c.Support))
		}
		ops = append(ops, "candidate_prefix_support="+strings.Join(support, "|"))
	}
	entry := store.ReconcileEvidence{
		SchemaVersion:        store.ReconcileEvidenceSchemaVersion,
		FeatureSlug:          slug,
		UpstreamRef:          upstreamRef,
		UpstreamCommit:       upstreamCommit,
		BaseCommit:           baseCommit,
		RawReconcileVerdict:  rawVerdict,
		Phase:                store.EvidencePhase35,
		EvidenceKind:         store.EvidenceKindPathRestructure,
		Confidence:           result.Confidence,
		MatchedPaths:         append([]string(nil), result.AffectedFeaturePaths...),
		MatchedOperations:    ops,
		MatchOrigin:          store.EvidenceMatchOriginUpstream,
		UpstreamCommitRefs:   []string{},
		PreReconcilePresence: pathRestructurePresence(result.Classification),
		RequiresConfirmation: false,
		ReasonCode:           string(result.Classification),
	}
	entry.AttemptID = store.ComputeAttemptID(entry)
	return entry
}

func normalizePathRestructureThresholds(in PathRestructureThresholds) PathRestructureThresholds {
	defaults := DefaultPathRestructureThresholds()
	if in.PrefixSplitMinFiles <= 0 {
		in.PrefixSplitMinFiles = defaults.PrefixSplitMinFiles
	}
	if in.PrefixSplitMinPrefixes <= 0 {
		in.PrefixSplitMinPrefixes = defaults.PrefixSplitMinPrefixes
	}
	if in.PrefixMoveMinFiles <= 0 {
		in.PrefixMoveMinFiles = defaults.PrefixMoveMinFiles
	}
	return in
}

func basePathRestructureEvidence(classification PathRestructureClassification, thresholds PathRestructureThresholds) *PathRestructureEvidence {
	return &PathRestructureEvidence{
		EvidenceKind:         string(store.EvidenceKindPathRestructure),
		Classification:       classification,
		CandidatePrefixes:    []string{},
		CandidateSupport:     []PathRestructureCandidate{},
		AffectedFeaturePaths: []string{},
		Confidence:           pathRestructureConfidence(classification),
		Thresholds:           thresholds,
	}
}

func evidenceForPathPrefixAggregate(agg *pathPrefixAggregate, thresholds PathRestructureThresholds) PathRestructureEvidence {
	classification := classifyPathPrefixAggregate(agg, thresholds)
	support := sortedPathRestructureCandidates(agg.moveSupport)
	if len(support) > pathRestructureCandidateLimit {
		support = support[:pathRestructureCandidateLimit]
	}
	prefixes := make([]string, 0, len(support))
	for _, c := range support {
		prefixes = append(prefixes, c.Prefix)
	}
	return PathRestructureEvidence{
		EvidenceKind:         string(store.EvidenceKindPathRestructure),
		Classification:       classification,
		OldPrefix:            agg.oldPrefix,
		CandidatePrefixes:    prefixes,
		CandidateSupport:     support,
		AffectedFeaturePaths: sortedBoolKeys(agg.affected),
		Confidence:           pathRestructureConfidence(classification),
		Thresholds:           thresholds,
		MovedFileCount:       agg.movedFiles,
		DeletedFileCount:     agg.deletedFiles,
	}
}

func classifyPathPrefixAggregate(agg *pathPrefixAggregate, thresholds PathRestructureThresholds) PathRestructureClassification {
	split := agg.movedFiles >= thresholds.PrefixSplitMinFiles && len(agg.moveSupport) >= thresholds.PrefixSplitMinPrefixes
	move := agg.movedFiles >= thresholds.PrefixMoveMinFiles && len(agg.moveSupport) == 1
	deleted := agg.deletedFiles > 0
	switch {
	case deleted && (split || move):
		return PathRestructureMixed
	case deleted:
		return PathRestructureTargetDeleted
	case split:
		return PathRestructurePrefixSplit
	case move:
		return PathRestructurePrefixMove
	default:
		return PathRestructureNone
	}
}

func betterPathRestructureEvidence(candidate PathRestructureEvidence, current *PathRestructureEvidence) bool {
	if current == nil {
		return true
	}
	candScore := pathRestructureScore(candidate.Classification)
	curScore := pathRestructureScore(current.Classification)
	if candScore != curScore {
		return candScore > curScore
	}
	if len(candidate.OldPrefix) != len(current.OldPrefix) {
		return len(candidate.OldPrefix) > len(current.OldPrefix)
	}
	if candidate.MovedFileCount != current.MovedFileCount {
		return candidate.MovedFileCount > current.MovedFileCount
	}
	return candidate.OldPrefix < current.OldPrefix
}

func pathRestructureScore(classification PathRestructureClassification) int {
	switch classification {
	case PathRestructureMixed:
		return 5
	case PathRestructureTargetDeleted:
		return 4
	case PathRestructurePrefixSplit:
		return 3
	case PathRestructurePrefixMove:
		return 2
	case PathRestructureNone:
		return 1
	default:
		return 0
	}
}

func pathRestructureConfidence(classification PathRestructureClassification) store.ReconcileEvidenceConfidence {
	switch classification {
	case PathRestructurePrefixMove, PathRestructurePrefixSplit, PathRestructureMixed:
		return store.EvidenceConfidenceHigh
	case PathRestructureTargetDeleted:
		return store.EvidenceConfidenceMedium
	case PathRestructureNone:
		return store.EvidenceConfidenceLow
	default:
		return store.EvidenceConfidenceUnknown
	}
}

func pathRestructurePresence(classification PathRestructureClassification) store.ReconcileEvidencePresence {
	switch classification {
	case PathRestructureTargetDeleted:
		return store.EvidencePresenceAbsent
	case PathRestructurePrefixMove, PathRestructurePrefixSplit, PathRestructureMixed:
		return store.EvidencePresencePresent
	default:
		return store.EvidencePresenceUnknown
	}
}
