package workflow

import (
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestClassifyBlockedVerdictCategories(t *testing.T) {
	cases := []struct {
		name string
		in   BlockedClassificationInput
		want BlockedCategory
	}{
		{"dependency", BlockedClassificationInput{Outcome: store.ReconcileBlocked, Labels: []store.ReconcileLabel{store.LabelBlockedByParent}}, BlockedCategoryDependencyBlocked},
		{"validation", BlockedClassificationInput{Outcome: store.ReconcileBlockedRequiresHuman, Phase: "phase-3.5", FailedFiles: []string{"a.go"}}, BlockedCategoryValidationBlocked},
		{"target-deleted", inputWithEvidence("target-deleted", store.EvidenceKindHunkOverlap, string(HunkOverlapTargetGone)), BlockedCategoryTargetDeleted},
		{"structural", inputWithEvidence("structural", store.EvidenceKindPathRestructure, "path-prefix"), BlockedCategoryStructuralConflict},
		{"edit-overlap", inputWithEvidence("edit", store.EvidenceKindHunkOverlap, string(HunkOverlapEditOverlap)), BlockedCategoryEditOverlap},
		{"shifted", inputWithEvidence("shifted", store.EvidenceKindHunkOverlap, string(HunkOverlapContextOnly)), BlockedCategoryShiftedContext},
		{"clean-additive", inputWithEvidence("clean", store.EvidenceKindFileNovelty, string(FileNoveltyAllNewFiles)), BlockedCategoryCleanAdditive},
		{"unknown", BlockedClassificationInput{Outcome: store.ReconcileBlocked}, BlockedCategoryUnknownBlocked},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyBlockedVerdict(tc.in)
			if got.Category != tc.want {
				t.Fatalf("got %q want %q", got.Category, tc.want)
			}
			if got.RecommendedAction == "" {
				t.Fatalf("missing action")
			}
		})
	}
}

func TestClassifyBlockedVerdictPrecedenceAndSecondaryEvidence(t *testing.T) {
	in := inputWithEvidence("multi", store.EvidenceKindHunkOverlap, string(HunkOverlapEditOverlap))
	in.Evidence = append(in.Evidence, store.ReconcileEvidence{EvidenceKind: store.EvidenceKindFileNovelty, ReasonCode: string(FileNoveltyAllNewFiles)})
	in.Labels = []store.ReconcileLabel{store.LabelBlockedByParent}
	got := ClassifyBlockedVerdict(in)
	if got.Category != BlockedCategoryDependencyBlocked {
		t.Fatalf("got %q", got.Category)
	}
	if len(got.SecondaryEvidence) < 2 {
		t.Fatalf("expected secondary evidence, got %#v", got.SecondaryEvidence)
	}
}

func inputWithEvidence(slug string, kind store.ReconcileEvidenceKind, reason string) BlockedClassificationInput {
	return BlockedClassificationInput{Outcome: store.ReconcileBlocked, Evidence: []store.ReconcileEvidence{{FeatureSlug: slug, EvidenceKind: kind, ReasonCode: reason, MatchedOperations: []string{"op"}}}}
}
