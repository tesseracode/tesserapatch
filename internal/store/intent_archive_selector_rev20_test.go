package store

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// TestIntentArchiveSelectorClassificationRev20 is the store half of the rev-20
// selector-classification amendment (PRD §9.7.2's preflight selector row,
// PIB-465). Every unusable selection the store is asked to normalize — malformed
// value, well-formed value the index does not carry, well-formed generation id
// the index does not record, wrong scope-family count — is classified
// `archive-selector-invalid`. The adjacent strict-index decode keeps its
// `archive-index-*` classification, and none of these routes writes anything.
//
// Scope note: only the two *well-formed but unknown* populations are public CLI
// report populations. The malformed and arity branches asserted here are the
// store's own defence in depth — the shipped `feature intent-archive purge`
// rejects both at exit 1, before any store call, so they never reach a report
// (see internal/cli's rev-20 malformed-selector table). They are asserted here
// for their classification and, crucially, for binding no rejected raw value to
// the typed error.
func TestIntentArchiveSelectorClassificationRev20(t *testing.T) {
	const feature = "demo"
	retained := archiveReplacement(t, IntentArchiveArtifactAnalysis, "selector rev20", IntentArchiveWireRetained)
	generation := archiveGeneration(t, feature, retained)

	unknownHash := archiveHash("no reference carries this content")
	unknownGeneration := archiveHash("no generation records this id")
	if unknownHash == retained.ContentSHA256 || unknownGeneration == generation.GenerationID {
		t.Fatal("the unknown fixture values collide with the indexed ones")
	}

	for _, populated := range []bool{true, false} {
		name := "populated-archive"
		if !populated {
			name = "empty-archive"
		}
		t.Run(name, func(t *testing.T) {
			for _, test := range []struct {
				name         string
				selector     IntentArchivePurgeSelector
				wantHash     string
				wantGenID    string
				wantExit     int
				wantDetail   string
				forbidValues []string
			}{
				{
					name:       "unknown-blob",
					selector:   IntentArchivePurgeSelector{Blobs: []string{unknownHash}},
					wantHash:   unknownHash,
					wantExit:   3,
					wantDetail: "a --blob selector does not name an indexed content hash",
				},
				{
					name:       "unknown-generation",
					selector:   IntentArchivePurgeSelector{Generations: []string{unknownGeneration}},
					wantGenID:  unknownGeneration,
					wantExit:   3,
					wantDetail: "a --generation selector does not name an archive generation",
				},
				{
					name:         "malformed-blob",
					selector:     IntentArchivePurgeSelector{Blobs: []string{"NOT-a-hash\n"}},
					wantExit:     3,
					wantDetail:   "a --blob selector is not a lowercase SHA-256",
					forbidValues: []string{"NOT-a-hash"},
				},
				{
					name:         "malformed-generation",
					selector:     IntentArchivePurgeSelector{Generations: []string{"../escape"}},
					wantExit:     3,
					wantDetail:   "a --generation selector is not a lowercase SHA-256",
					forbidValues: []string{"../escape"},
				},
				{
					name:       "no-selector",
					selector:   IntentArchivePurgeSelector{},
					wantExit:   1,
					wantDetail: "exactly one purge selector is required",
				},
				{
					name: "two-selector-families",
					selector: IntentArchivePurgeSelector{
						Blobs: []string{retained.ContentSHA256},
						All:   true,
					},
					wantExit:   1,
					wantDetail: "exactly one purge selector is required",
				},
			} {
				t.Run(test.name, func(t *testing.T) {
					for _, confirmed := range []bool{false, true} {
						form := "preview"
						if confirmed {
							form = "confirmed"
						}
						t.Run(form, func(t *testing.T) {
							index := archiveIndex(t, feature)
							if populated {
								index = archiveIndex(t, feature, generation)
							}
							storage := newArchiveMemoryStorage(t, index)
							if populated {
								storage.putRegular(feature, retained.ContentSHA256, []byte("selector rev20"))
							}
							before := append([]byte(nil), storage.index...)
							beforeBlobs := fmt.Sprint(storage.blobs)
							storage.calls = nil

							_, err := PlanIntentArchivePurge(storage, feature, test.selector, confirmed)
							typed := assertArchiveCode(t, err, IntentArchiveCodeSelectorInvalid)
							if string(typed.Code) != "archive-selector-invalid" {
								t.Fatalf("public code = %q, want the exact rev-20 spelling", typed.Code)
							}
							if typed.ExitClass != test.wantExit {
								t.Fatalf("exit class = %d, want %d", typed.ExitClass, test.wantExit)
							}
							if typed.Detail != test.wantDetail {
								t.Fatalf("detail = %q, want %q", typed.Detail, test.wantDetail)
							}
							if typed.Hash != test.wantHash {
								t.Fatalf("typed hash = %q, want %q", typed.Hash, test.wantHash)
							}
							if typed.GenerationID != test.wantGenID {
								t.Fatalf("typed generation = %q, want %q", typed.GenerationID, test.wantGenID)
							}
							if typed.Committed {
								t.Fatal("a selector refusal reported committed work")
							}
							rendered := typed.Error()
							for _, unsafe := range test.forbidValues {
								if strings.Contains(rendered, unsafe) {
									t.Fatalf("the rendered error echoed the rejected value: %q", rendered)
								}
							}
							if len(storage.mutationCalls()) != 0 ||
								len(storage.indexHistory) != 0 ||
								!bytes.Equal(before, storage.index) ||
								fmt.Sprint(storage.blobs) != beforeBlobs {
								t.Fatalf("selector refusal mutated storage: %v", storage.calls)
							}
						})
					}
				})
			}
		})
	}

	// The adjacent population: a real strict-decode failure over the same
	// storage keeps its `archive-index-*` classification, so rev-20 narrows
	// nothing outside the selector row.
	t.Run("adjacent-strict-index-failure-stays-index-corrupt", func(t *testing.T) {
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature, generation))
		storage.putRegular(feature, retained.ContentSHA256, []byte("selector rev20"))
		storage.index = []byte("{")
		storage.indexVersion++
		_, err := PlanIntentArchivePurge(storage, feature,
			IntentArchivePurgeSelector{Blobs: []string{retained.ContentSHA256}}, true)
		typed := assertArchiveCode(t, err, IntentArchiveCodeIndexCorrupt)
		if typed.ExitClass != 3 {
			t.Fatalf("strict decode exit class = %d, want 3", typed.ExitClass)
		}
		if len(storage.mutationCalls()) != 0 {
			t.Fatalf("strict decode refusal mutated storage: %v", storage.calls)
		}
	})

	// A preview plan is never executable, and that misuse is the same code.
	t.Run("preview-plan-execution-is-selector-invalid", func(t *testing.T) {
		storage := newArchiveMemoryStorage(t, archiveIndex(t, feature, generation))
		storage.putRegular(feature, retained.ContentSHA256, []byte("selector rev20"))
		plan, err := PreviewIntentArchivePurge(storage, feature,
			IntentArchivePurgeSelector{Blobs: []string{retained.ContentSHA256}})
		if err != nil {
			t.Fatal(err)
		}
		storage.calls = nil
		_, err = ExecuteIntentArchivePurge(storage, plan)
		typed := assertArchiveCode(t, err, IntentArchiveCodeSelectorInvalid)
		if typed.ExitClass != 3 || typed.Committed {
			t.Fatalf("preview execution refusal = %+v", typed)
		}
		if len(storage.mutationCalls()) != 0 {
			t.Fatalf("preview execution mutated storage: %v", storage.calls)
		}
	})
}
