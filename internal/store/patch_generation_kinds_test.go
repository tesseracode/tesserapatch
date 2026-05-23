package store

import "testing"

func TestClassifyPatchGenerationKindTransitions(t *testing.T) {
	prior := []PatchGeneration{
		{Generation: 1, GenerationID: "pg_one", Kind: PatchGenerationKindRecord, PatchSHA256: "sha-one"},
	}
	tests := []struct {
		name    string
		prior   []PatchGeneration
		sha     string
		intent  string
		want    PatchGenerationClassification
		wantErr bool
	}{
		{
			name:   "no prior generations plain record",
			sha:    "sha-one",
			intent: PatchGenerationIntentPlainRecord,
			want:   PatchGenerationClassification{Kind: PatchGenerationKindRecord, Append: true},
		},
		{
			name:   "prior generations identical patch bytes skip append",
			prior:  prior,
			sha:    "sha-one",
			intent: PatchGenerationIntentPlainRecord,
			want:   PatchGenerationClassification{Append: false},
		},
		{
			name:   "prior generations changed plain record becomes amend refresh",
			prior:  prior,
			sha:    "sha-two",
			intent: PatchGenerationIntentPlainRecord,
			want:   PatchGenerationClassification{Kind: PatchGenerationKindAmendRefresh, Append: true},
		},
		{
			name:   "prior generations explicit refresh becomes amend refresh",
			prior:  prior,
			sha:    "sha-two",
			intent: PatchGenerationIntentRefresh,
			want:   PatchGenerationClassification{Kind: PatchGenerationKindAmendRefresh, Append: true},
		},
		{
			name:   "prior generations explicit fixup becomes amend fixup",
			prior:  prior,
			sha:    "sha-two",
			intent: PatchGenerationIntentFixup,
			want:   PatchGenerationClassification{Kind: PatchGenerationKindAmendFixup, Append: true},
		},
		{
			name:    "unknown intent fails",
			prior:   prior,
			sha:     "sha-two",
			intent:  "fork",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ClassifyPatchGenerationKind(tt.prior, tt.sha, tt.intent)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ClassifyPatchGenerationKind: %v", err)
			}
			if got != tt.want {
				t.Fatalf("classification mismatch: got %+v want %+v", got, tt.want)
			}
		})
	}
}

func TestClassifyPlainRecordKindWrapper(t *testing.T) {
	got := ClassifyPlainRecordKind(nil, "sha-one")
	want := PatchGenerationClassification{Kind: PatchGenerationKindRecord, Append: true}
	if got != want {
		t.Fatalf("ClassifyPlainRecordKind()=%+v want %+v", got, want)
	}
}

func TestPatchGenerationWritableKinds(t *testing.T) {
	for _, kind := range []string{PatchGenerationKindRecord, PatchGenerationKindReconcile, PatchGenerationKindAmendRefresh, PatchGenerationKindAmendFixup} {
		if !IsWritablePatchGenerationKind(kind) {
			t.Fatalf("kind %q should be writable", kind)
		}
	}
	for _, kind := range []string{PatchGenerationKindImport, PatchGenerationKindManualMeta, "fork"} {
		if IsWritablePatchGenerationKind(kind) {
			t.Fatalf("kind %q should remain non-writable", kind)
		}
	}
}
