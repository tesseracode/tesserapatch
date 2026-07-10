package workflow

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/store"
)

type BlockedCategory string

const (
	BlockedCategoryDependencyBlocked  BlockedCategory = "dependency-blocked"
	BlockedCategoryValidationBlocked  BlockedCategory = "validation-blocked"
	BlockedCategoryTargetDeleted      BlockedCategory = "target-deleted"
	BlockedCategoryStructuralConflict BlockedCategory = "structural-conflict"
	BlockedCategoryEditOverlap        BlockedCategory = "edit-overlap"
	BlockedCategoryShiftedContext     BlockedCategory = "shifted-context"
	BlockedCategoryCleanAdditive      BlockedCategory = "clean-additive"
	BlockedCategoryUnknownBlocked     BlockedCategory = "unknown-blocked"
)

type BlockedClassification struct {
	Category          BlockedCategory `json:"category"`
	RecommendedAction string          `json:"recommended_action"`
	Evidence          []string        `json:"evidence"`
	SecondaryEvidence []string        `json:"secondary_evidence,omitempty"`
}

type BlockedClassificationInput struct {
	Outcome      store.ReconcileOutcome
	Phase        string
	Labels       []store.ReconcileLabel
	Evidence     []store.ReconcileEvidence
	FailedFiles  []string
	SkippedFiles []string
	Notes        []string
}

var blockedPrecedence = []BlockedCategory{
	BlockedCategoryDependencyBlocked,
	BlockedCategoryValidationBlocked,
	BlockedCategoryTargetDeleted,
	BlockedCategoryStructuralConflict,
	BlockedCategoryEditOverlap,
	BlockedCategoryShiftedContext,
	BlockedCategoryCleanAdditive,
	BlockedCategoryUnknownBlocked,
}

var blockedRecommendedActions = map[BlockedCategory]string{
	BlockedCategoryDependencyBlocked:  "reconcile-or-repair-parent-first",
	BlockedCategoryValidationBlocked:  "inspect-tests-or-typecheck",
	BlockedCategoryTargetDeleted:      "check-path-restructure-or-rewrite",
	BlockedCategoryStructuralConflict: "manual-rewrite-or-path-migration",
	BlockedCategoryEditOverlap:        "human-or-provider-resolution",
	BlockedCategoryShiftedContext:     "try-relocation",
	BlockedCategoryCleanAdditive:      "reapply-or-accept-deterministic-apply",
	BlockedCategoryUnknownBlocked:     "manual-review",
}

func ClassifyBlockedVerdict(input BlockedClassificationInput) BlockedClassification {
	candidates := map[BlockedCategory][]string{}
	add := func(cat BlockedCategory, evidence string) {
		if evidence == "" {
			evidence = string(cat)
		}
		candidates[cat] = append(candidates[cat], evidence)
	}
	if input.Outcome != store.ReconcileBlocked && input.Outcome != store.ReconcileBlockedRequiresHuman && input.Outcome != store.ReconcileBlockedTooManyConflicts {
		return BlockedClassification{}
	}
	for _, l := range input.Labels {
		switch l {
		case store.LabelBlockedByParent, store.LabelWaitingOnParent:
			add(BlockedCategoryDependencyBlocked, "dependency-label="+string(l))
		}
	}
	if input.Outcome == store.ReconcileBlockedRequiresHuman || strings.Contains(input.Phase, "phase-3.5") {
		if len(input.FailedFiles) > 0 || len(input.SkippedFiles) > 0 || noteContains(input.Notes, "validation") || noteContains(input.Notes, "provider") {
			add(BlockedCategoryValidationBlocked, "phase="+input.Phase)
		}
	}
	for _, ev := range input.Evidence {
		switch ev.EvidenceKind {
		case store.EvidenceKindPathRestructure:
			switch ev.ReasonCode {
			case string(PathRestructurePrefixMove), string(PathRestructurePrefixSplit), string(PathRestructureMixed):
				add(BlockedCategoryStructuralConflict, evidenceSummary(ev))
			case string(PathRestructureTargetDeleted):
				add(BlockedCategoryTargetDeleted, evidenceSummary(ev))
			}
		case store.EvidenceKindFileNovelty:
			switch ev.ReasonCode {
			case string(FileNoveltyAllNewFiles), string(FileNoveltyMixedAdditive):
				add(BlockedCategoryCleanAdditive, evidenceSummary(ev))
			case string(FileNoveltyDeletesOrRenames):
				add(BlockedCategoryStructuralConflict, evidenceSummary(ev))
			}
		case store.EvidenceKindHunkOverlap:
			switch ev.ReasonCode {
			case string(HunkOverlapTargetGone):
				add(BlockedCategoryTargetDeleted, evidenceSummary(ev))
			case string(HunkOverlapPathMoved):
				add(BlockedCategoryStructuralConflict, evidenceSummary(ev))
			case string(HunkOverlapEditOverlap):
				add(BlockedCategoryEditOverlap, evidenceSummary(ev))
			case string(HunkOverlapContextOnly):
				add(BlockedCategoryShiftedContext, evidenceSummary(ev))
			}
		}
	}
	primary := BlockedCategoryUnknownBlocked
	for _, cat := range blockedPrecedence {
		if _, ok := candidates[cat]; ok {
			primary = cat
			break
		}
	}
	if primary == BlockedCategoryUnknownBlocked {
		add(primary, "insufficient-evidence")
	}
	secondary := make([]string, 0)
	for _, cat := range blockedPrecedence {
		if cat == primary {
			continue
		}
		for _, ev := range sortedUnique(candidates[cat]) {
			secondary = append(secondary, fmt.Sprintf("%s: %s", cat, ev))
		}
	}
	return BlockedClassification{Category: primary, RecommendedAction: blockedRecommendedActions[primary], Evidence: sortedUnique(candidates[primary]), SecondaryEvidence: secondary}
}

func noteContains(notes []string, needle string) bool {
	needle = strings.ToLower(needle)
	for _, n := range notes {
		if strings.Contains(strings.ToLower(n), needle) {
			return true
		}
	}
	return false
}

func evidenceSummary(ev store.ReconcileEvidence) string {
	parts := []string{string(ev.EvidenceKind), ev.ReasonCode}
	if len(ev.MatchedOperations) > 0 {
		parts = append(parts, ev.MatchedOperations...)
	}
	return strings.Join(parts, " ")
}

func sortedUnique(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func blockedClassificationEvidence(slug, upstreamRef, upstreamCommit, baseCommit string, cls BlockedClassification) store.ReconcileEvidence {
	ops := []string{"primary=" + string(cls.Category), "recommended_action=" + cls.RecommendedAction}
	for _, ev := range cls.SecondaryEvidence {
		ops = append(ops, "secondary="+ev)
	}
	entry := store.ReconcileEvidence{SchemaVersion: store.ReconcileEvidenceSchemaVersion, FeatureSlug: slug, UpstreamRef: upstreamRef, UpstreamCommit: upstreamCommit, BaseCommit: baseCommit, RawReconcileVerdict: string(store.ReconcileBlocked), Phase: store.EvidencePhase4, EvidenceKind: store.EvidenceKindBlockedClassification, Confidence: store.EvidenceConfidenceMedium, MatchedPaths: []string{}, MatchedOperations: ops, MatchOrigin: store.EvidenceMatchOriginUnknown, UpstreamCommitRefs: []string{}, PreReconcilePresence: store.EvidencePresenceNotChecked, RequiresConfirmation: false, ReasonCode: string(cls.Category)}
	if cls.Category == BlockedCategoryUnknownBlocked {
		entry.Confidence = store.EvidenceConfidenceUnknown
	}
	entry.AttemptID = store.ComputeAttemptID(entry)
	return entry
}
