package workflow

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestDetectPathRestructureClassifications(t *testing.T) {
	cases := []struct {
		name       string
		paths      []string
		nameStatus string
		want       PathRestructureClassification
		oldPrefix  string
	}{
		{
			name:  "prefix-split",
			paths: []string{"apps/desktop/src/ManagedEnvironment.ts"},
			nameStatus: strings.Join([]string{
				"R100\tapps/desktop/src/alpha.ts\tapps/desktop/src/app/alpha.ts",
				"R100\tapps/desktop/src/beta.ts\tapps/desktop/src/backend/beta.ts",
				"R100\tapps/desktop/src/gamma.ts\tapps/desktop/src/electron/gamma.ts",
			}, "\n"),
			want:      PathRestructurePrefixSplit,
			oldPrefix: "apps/desktop/src/",
		},
		{
			name:  "prefix-move",
			paths: []string{"src/feature.go"},
			nameStatus: strings.Join([]string{
				"R100\tsrc/a.go\tlib/a.go",
				"R100\tsrc/b.go\tlib/b.go",
				"R100\tsrc/c.go\tlib/c.go",
				"R100\tsrc/d.go\tlib/d.go",
				"R100\tsrc/e.go\tlib/e.go",
			}, "\n"),
			want:      PathRestructurePrefixMove,
			oldPrefix: "src/",
		},
		{
			name:       "target-deleted",
			paths:      []string{"src/deleted.go"},
			nameStatus: "D\tsrc/deleted.go",
			want:       PathRestructureTargetDeleted,
			oldPrefix:  "src/",
		},
		{
			name:  "mixed",
			paths: []string{"src/deleted.go", "src/feature.go"},
			nameStatus: strings.Join([]string{
				"D\tsrc/deleted.go",
				"R100\tsrc/a.go\tpkg/a.go",
				"R100\tsrc/b.go\tother/b.go",
				"R100\tsrc/c.go\tpkg/c.go",
			}, "\n"),
			want:      PathRestructureMixed,
			oldPrefix: "src/",
		},
		{
			name:  "none",
			paths: []string{"src/feature.go"},
			nameStatus: strings.Join([]string{
				"R100\tsrc/a.go\tpkg/a.go",
				"R100\tsrc/b.go\tother/b.go",
			}, "\n"),
			want: PathRestructureNone,
		},
		{
			name:       "unknown",
			paths:      []string{"src/feature.go"},
			nameStatus: "R100\tsrc/a.go",
			want:       PathRestructureUnknown,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetectPathRestructure(PathRestructureInput{FeaturePaths: tc.paths, RenameCopyNameStatus: tc.nameStatus})
			if err != nil {
				t.Fatal(err)
			}
			if got.Classification != tc.want {
				t.Fatalf("classification got %q want %q: %+v", got.Classification, tc.want, got)
			}
			if tc.oldPrefix != "" && got.OldPrefix != tc.oldPrefix {
				t.Fatalf("old prefix got %q want %q", got.OldPrefix, tc.oldPrefix)
			}
			if got.EvidenceKind != string(store.EvidenceKindPathRestructure) {
				t.Fatalf("evidence kind got %q", got.EvidenceKind)
			}
		})
	}
}

func TestDetectPathRestructureThresholdBoundaries(t *testing.T) {
	twoMoves := strings.Join([]string{
		"R100\tsrc/a.go\tapp/a.go",
		"R100\tsrc/b.go\tbackend/b.go",
	}, "\n")
	got, err := DetectPathRestructure(PathRestructureInput{FeaturePaths: []string{"src/feature.go"}, RenameCopyNameStatus: twoMoves})
	if err != nil {
		t.Fatal(err)
	}
	if got.Classification != PathRestructureNone {
		t.Fatalf("two moved files must not meet default split threshold: %+v", got)
	}

	threeMoves := twoMoves + "\nR100\tsrc/c.go\tbackend/c.go"
	got, err = DetectPathRestructure(PathRestructureInput{FeaturePaths: []string{"src/feature.go"}, RenameCopyNameStatus: threeMoves})
	if err != nil {
		t.Fatal(err)
	}
	if got.Classification != PathRestructurePrefixSplit {
		t.Fatalf("three files across two prefixes must split by default: %+v", got)
	}
	if got.Thresholds != DefaultPathRestructureThresholds() {
		t.Fatalf("default thresholds not exposed: %+v", got.Thresholds)
	}
}

func TestDetectPathRestructureCandidateSortAndCap(t *testing.T) {
	status := strings.Join([]string{
		"R100\tsrc/a1.go\tz/a1.go",
		"R100\tsrc/a2.go\tz/a2.go",
		"R100\tsrc/a3.go\tz/a3.go",
		"R100\tsrc/b1.go\ta/b1.go",
		"R100\tsrc/b2.go\ta/b2.go",
		"R100\tsrc/b3.go\ta/b3.go",
		"R100\tsrc/c1.go\tm/c1.go",
		"R100\tsrc/c2.go\tm/c2.go",
		"R100\tsrc/d.go\tb/d.go",
		"R100\tsrc/e.go\tc/e.go",
		"R100\tsrc/f.go\td/f.go",
		"R100\tsrc/g.go\te/g.go",
	}, "\n")
	got, err := DetectPathRestructure(PathRestructureInput{FeaturePaths: []string{"src/feature.go"}, RenameCopyNameStatus: status})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a/", "z/", "m/", "b/", "c/"}
	if !reflect.DeepEqual(got.CandidatePrefixes, want) {
		t.Fatalf("candidate prefixes got %#v want %#v", got.CandidatePrefixes, want)
	}
	if len(got.CandidateSupport) != 5 {
		t.Fatalf("candidate support must be capped at 5: %+v", got.CandidateSupport)
	}
	if got.CandidateSupport[0].Support != 3 || got.CandidateSupport[0].Prefix != "a/" {
		t.Fatalf("support desc then path asc sort not applied: %+v", got.CandidateSupport)
	}
}

func TestPathRestructureReconcileEvidenceEncodesContractWithoutSchemaFields(t *testing.T) {
	result, err := DetectPathRestructure(PathRestructureInput{
		FeaturePaths: []string{"src/feature.go"},
		RenameCopyNameStatus: strings.Join([]string{
			"R100\tsrc/a.go\tapp/a.go",
			"R100\tsrc/b.go\tbackend/b.go",
			"R100\tsrc/c.go\tbackend/c.go",
		}, "\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	entry := PathRestructureReconcileEvidence("slug", "HEAD", "abc", "base", string(store.ReconcileBlocked), *result)
	if entry.EvidenceKind != store.EvidenceKindPathRestructure || entry.ReasonCode != string(PathRestructurePrefixSplit) {
		t.Fatalf("unexpected evidence: %+v", entry)
	}
	if entry.Confidence != store.EvidenceConfidenceHigh {
		t.Fatalf("expected high confidence, got %q", entry.Confidence)
	}
	ops := strings.Join(entry.MatchedOperations, "\n")
	for _, want := range []string{
		"old_prefix=src/",
		"candidate_prefixes=backend/|app/",
		"prefix_split_min_files=3",
		"prefix_split_min_prefixes=2",
		"prefix_move_min_files=5",
	} {
		if !strings.Contains(ops, want) {
			t.Fatalf("matched operations missing %q: %s", want, ops)
		}
	}
	if got := strings.Join(entry.MatchedPaths, ","); got != "src/feature.go" {
		t.Fatalf("affected paths got %q", got)
	}
}
