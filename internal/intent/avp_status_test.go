package intent

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"
)

// TestAVPStatusLadder covers category V's unit rows — the total `status.json`
// inspection ladder (AVP-155…AVP-163) plus the descriptor-close row AVP-204
// and the status half of AVP-206.
func TestAVPStatusLadder(t *testing.T) {
	t.Run("AVP-155", func(t *testing.T) {
		for _, variant := range []struct {
			name string
			info fs.FileInfo
		}{
			{"in-repo-target", fakeInfo{name: testStatus, mode: fs.ModeSymlink, size: 19}},
			{"out-of-repo-target", fakeInfo{name: testStatus, mode: fs.ModeSymlink, size: 32}},
			{"dangling", fakeInfo{name: testStatus, mode: fs.ModeSymlink}},
		} {
			t.Run(variant.name, func(t *testing.T) {
				report, root := runInspect(t, func(r *fakeRoot) { r.set(testStatus, variant.info) })
				if report.AbortCode() != AbortStatusSymlink {
					t.Fatalf("abort = %q, want %q", report.AbortCode(), AbortStatusSymlink)
				}
				if root.opensOf(testStatus) != 0 {
					t.Fatal("a refused status symlink was opened")
				}
				if strings.Contains(report.Abort.Message, "/etc/") {
					t.Fatal("the link target was named")
				}
			})
		}
	})

	t.Run("AVP-156", func(t *testing.T) {
		for _, variant := range []struct {
			name string
			info fs.FileInfo
		}{
			{"directory", dir(testStatus)},
			{"fifo-no-writer", fakeInfo{name: testStatus, mode: fs.ModeNamedPipe}},
		} {
			t.Run(variant.name, func(t *testing.T) {
				deadline := time.Now().Add(20 * time.Second)
				report, root := runInspect(t, func(r *fakeRoot) { r.set(testStatus, variant.info) })
				if time.Now().After(deadline) {
					t.Fatal("status capture did not return under its hard deadline")
				}
				if report.AbortCode() != AbortStatusNotRegular {
					t.Fatalf("abort = %q, want %q", report.AbortCode(), AbortStatusNotRegular)
				}
				if root.opensOf(testStatus) != 0 {
					t.Fatal("a non-regular status file was opened")
				}
			})
		}
	})

	t.Run("AVP-157", func(t *testing.T) {
		var file *fakeFile
		report, root := runInspect(t, func(r *fakeRoot) {
			file = r.nodes[testStatus].file
			r.nodes[testStatus].info = sized(testStatus, MaxStatusBytes+1)
		})
		if report.AbortCode() != AbortStatusOversize {
			t.Fatalf("abort = %q, want %q", report.AbortCode(), AbortStatusOversize)
		}
		if root.opensOf(testStatus) != 0 {
			t.Fatal("an oversize status file was opened")
		}
		if file.bytesRead() != 0 {
			t.Fatal("an oversize status file was read")
		}
	})

	t.Run("AVP-158", func(t *testing.T) {
		data := statusPaddedTo(MaxStatusBytes)
		if len(data) != MaxStatusBytes {
			t.Fatalf("fixture is %d bytes, want exactly %d", len(data), MaxStatusBytes)
		}
		var file *fakeFile
		report, _ := runInspect(t, func(r *fakeRoot) { file = r.setFile(testStatus, data) })
		if report.Abort != nil {
			t.Fatalf("unexpected abort %q", report.Abort.Code)
		}
		if report.FeatureState != FeatureStateDefined {
			t.Fatalf("feature_state = %q, want %q", report.FeatureState, FeatureStateDefined)
		}
		if file.reads[0].requested != MaxStatusBytes+1 {
			t.Fatalf("status read requested %d bytes, want %d", file.reads[0].requested, MaxStatusBytes+1)
		}
	})

	t.Run("AVP-159", func(t *testing.T) {
		report, _ := runInspect(t, func(r *fakeRoot) {
			info := sized(testStatus, MaxStatusBytes)
			r.nodes[testStatus] = &fakeNode{info: info, file: &fakeFile{
				name: testStatus, data: filledBytes(MaxStatusBytes + 4096),
				statInfos: []fs.FileInfo{info},
			}}
		})
		if report.AbortCode() != AbortStatusUnstable {
			t.Fatalf("abort = %q, want %q (never status-oversize)", report.AbortCode(), AbortStatusUnstable)
		}
	})

	t.Run("AVP-160", func(t *testing.T) {
		var file *fakeFile
		report, root := runInspect(t, func(r *fakeRoot) {
			file = r.nodes[testStatus].file
			r.sameFile = func(a, b fs.FileInfo) bool { return a.Name() != testStatus }
		})
		if report.AbortCode() != AbortStatusUnstable {
			t.Fatalf("abort = %q, want %q", report.AbortCode(), AbortStatusUnstable)
		}
		if file.bytesRead() != 0 {
			t.Fatal("bytes were classified from a replaced status inode")
		}
		if root.sameFileCalls == 0 {
			t.Fatal("status identity did not go through the SameFile seam")
		}
	})

	t.Run("AVP-161", func(t *testing.T) {
		report, _ := runInspect(t, func(r *fakeRoot) {
			info := regular(testStatus, []byte(`{"state":"defined"}`))
			r.nodes[testStatus] = &fakeNode{info: info, file: &fakeFile{
				name: testStatus, data: []byte(`{"state":"defined"}`),
				statInfos: []fs.FileInfo{info},
				statErrs:  []error{errors.New("fstat failed")},
			}}
		})
		if report.AbortCode() != AbortStatusUnreadable {
			t.Fatalf("abort = %q, want %q", report.AbortCode(), AbortStatusUnreadable)
		}
	})

	t.Run("AVP-162", func(t *testing.T) {
		report, _ := runInspect(t, func(r *fakeRoot) {
			data := []byte(`{"state":"defined"}`)
			info := regular(testStatus, data)
			r.nodes[testStatus] = &fakeNode{info: info, file: &fakeFile{
				name: testStatus, data: data, statInfos: []fs.FileInfo{info},
				readErrOn: 1, readErr: errors.New("read failed"),
			}}
		})
		if report.AbortCode() != AbortStatusUnreadable {
			t.Fatalf("abort = %q, want %q", report.AbortCode(), AbortStatusUnreadable)
		}
	})

	t.Run("AVP-163", func(t *testing.T) {
		// A torn read must never be reported as corruption.
		report, _ := runInspect(t, func(r *fakeRoot) {
			info := sized(testStatus, 19)
			r.nodes[testStatus] = &fakeNode{info: info, file: &fakeFile{
				name: testStatus, data: nil, statInfos: []fs.FileInfo{info},
			}}
		})
		if report.AbortCode() != AbortStatusUnstable {
			t.Fatalf("abort = %q, want %q (never status-malformed)", report.AbortCode(), AbortStatusUnstable)
		}
	})

	t.Run("AVP-204", func(t *testing.T) {
		report, _ := runInspect(t, func(r *fakeRoot) {
			r.nodes[testStatus].file.closeErr = errors.New("close failed")
		})
		if report.AbortCode() != AbortStatusUnreadable {
			t.Fatalf("abort = %q, want %q", report.AbortCode(), AbortStatusUnreadable)
		}
		if len(report.Artifacts) != 0 {
			t.Fatalf("abort emitted %d artifact rows", len(report.Artifacts))
		}
		if report.FeatureState != FeatureStateUnknown {
			t.Fatalf("feature_state = %q, want unknown", report.FeatureState)
		}
		want := abortMessage(AbortStatusUnreadable, testSlug)
		if report.Abort.Message != want {
			t.Fatalf("message = %q, want the frozen template", report.Abort.Message)
		}
		if !strings.Contains(want, "could not be read and closed cleanly") {
			t.Fatal("the close-failure template no longer states the close half")
		}
		if len(AbortCodes()) != 13 {
			t.Fatalf("abort catalog has %d codes; the close contract adds no fourteenth", len(AbortCodes()))
		}
	})

	t.Run("AVP-166-status-echo-domain", func(t *testing.T) {
		for _, state := range FeatureStates() {
			t.Run(state, func(t *testing.T) {
				report, _ := runInspect(t, func(r *fakeRoot) {
					r.setFile(testStatus, []byte(`{"state":"`+state+`"}`))
				})
				if report.Abort != nil {
					t.Fatalf("unexpected abort %q", report.Abort.Code)
				}
				if report.FeatureState != state {
					t.Fatalf("feature_state = %q, want %q", report.FeatureState, state)
				}
			})
		}
	})

	t.Run("AVP-169-status-precedes-artifacts", func(t *testing.T) {
		report, root := runInspect(t, func(r *fakeRoot) {
			r.setFile(testStatus, []byte("not json"))
			r.remove(testAnalysis)
			r.remove(testSpec)
			r.remove(testExploration)
		})
		if report.AbortCode() != AbortStatusMalformed {
			t.Fatalf("abort = %q, want %q", report.AbortCode(), AbortStatusMalformed)
		}
		if len(report.Artifacts) != 0 {
			t.Fatalf("abort emitted %d artifact rows", len(report.Artifacts))
		}
		for _, name := range []string{testAnalysis, testSpec, testExploration, testSidecar} {
			if root.lstatsOf(name) != 0 {
				t.Fatalf("artifact %q was stat'd after the status abort", name)
			}
		}
	})
}

// TestAVPStatusSchemaFidelity is the rev-1 correction: `status-malformed`
// means "would not decode into the tpatch status document", not "the state
// member is not a string".
func TestAVPStatusSchemaFidelity(t *testing.T) {
	malformed := []struct {
		name     string
		document string
	}{
		{"not-json", `not json at all`},
		{"json-array", `[1,2,3]`},
		{"json-string", `"defined"`},
		{"json-number", `12`},
		{"truncated-object", `{"state":"defined"`},
		{"state-wrong-type", `{"state":7}`},
		{"id-wrong-type", `{"state":"defined","id":7}`},
		{"slug-wrong-type", `{"state":"defined","slug":[]}`},
		{"title-wrong-type", `{"state":"defined","title":{}}`},
		{"compatibility-wrong-type", `{"state":"defined","compatibility":3}`},
		{"requested-at-wrong-type", `{"state":"defined","requested_at":1}`},
		{"updated-at-wrong-type", `{"state":"defined","updated_at":false}`},
		{"last-command-wrong-type", `{"state":"defined","last_command":[]}`},
		{"notes-wrong-type", `{"state":"defined","notes":9}`},
		{"apply-not-object", `{"state":"defined","apply":7}`},
		{"apply-field-wrong-type", `{"state":"defined","apply":{"has_patch":"yes"}}`},
		{"apply-base-commit-wrong-type", `{"state":"defined","apply":{"base_commit":3}}`},
		{"reconcile-not-object", `{"state":"defined","reconcile":"blocked"}`},
		{"reconcile-counter-wrong-type", `{"state":"defined","reconcile":{"resolved_files":"two"}}`},
		{"reconcile-labels-wrong-type", `{"state":"defined","reconcile":{"labels":"waiting-on-parent"}}`},
		{"reconcile-outcome-wrong-type", `{"state":"defined","reconcile":{"outcome":5}}`},
		{"patch-id-match-not-object", `{"state":"defined","reconcile":{"patch_id_match":[]}}`},
		{"patch-id-scanned-count-wrong-type", `{"state":"defined","reconcile":{"patch_id_match":{"scanned_count":"many"}}}`},
		{"patch-id-additional-matches-wrong-type", `{"state":"defined","reconcile":{"patch_id_match":{"additional_matches":"a"}}}`},
		{"depends-on-not-array", `{"state":"defined","depends_on":{}}`},
		{"depends-on-element-wrong-type", `{"state":"defined","depends_on":["parent"]}`},
		{"depends-on-kind-wrong-type", `{"state":"defined","depends_on":[{"slug":"p","kind":2}]}`},
		{"verify-not-object", `{"state":"defined","verify":true}`},
		{"verify-passed-wrong-type", `{"state":"defined","verify":{"passed":"yes"}}`},
		{"verify-parent-snapshot-wrong-type", `{"state":"defined","verify":{"parent_snapshot":["p"]}}`},
		{"verify-parent-snapshot-value-wrong-type", `{"state":"defined","verify":{"parent_snapshot":{"p":3}}}`},
		{"rejection-not-object", `{"state":"rejected","rejection":"because"}`},
		{"rejection-evidence-wrong-type", `{"state":"rejected","rejection":{"evidence":"e"}}`},
		{"rejection-evidence-element-wrong-type", `{"state":"rejected","rejection":{"evidence":[5]}}`},
		{"rejection-rejected-at-not-a-time", `{"state":"rejected","rejection":{"rejected_at":"yesterday"}}`},
		{"rejection-prior-state-wrong-type", `{"state":"rejected","rejection":{"prior_state":1}}`},
		{"rejection-history-not-array", `{"state":"defined","rejection_history":{}}`},
		{"rejection-history-entry-wrong-type", `{"state":"defined","rejection_history":[7]}`},
		{"rejection-history-reopened-at-not-a-time", `{"state":"defined","rejection_history":[{"reopened_at":"soon"}]}`},
		{"rejection-history-divergence-wrong-type", `{"state":"defined","rejection_history":[{"divergence_detail":"x"}]}`},
		{"rejection-history-divergence-element-wrong-type", `{"state":"defined","rejection_history":[{"divergence_detail":[3]}]}`},
	}
	for _, tc := range malformed {
		t.Run("malformed/"+tc.name, func(t *testing.T) {
			if state, ok := decodeStatusDocument([]byte(tc.document)); ok {
				t.Fatalf("decodeStatusDocument accepted %s (state %q)", tc.document, state)
			}
			report, _ := runInspect(t, func(r *fakeRoot) { r.setFile(testStatus, []byte(tc.document)) })
			if report.AbortCode() != AbortStatusMalformed {
				t.Fatalf("abort = %q, want %q", report.AbortCode(), AbortStatusMalformed)
			}
			if strings.Contains(report.Abort.Message, tc.document) {
				t.Fatal("the offending document bytes were echoed")
			}
		})
	}

	accepted := []struct {
		name     string
		document string
	}{
		{"minimal", `{"state":"defined"}`},
		{"unknown-future-key", `{"state":"defined","unknown_future_field":{"deep":[1,2]}}`},
		{"full-shape", `{"id":"f1","slug":"s","title":"t","state":"applied",` +
			`"compatibility":"compatible","requested_at":"2026-01-01T00:00:00Z",` +
			`"updated_at":"2026-01-02T00:00:00Z","last_command":"apply","notes":"n",` +
			`"apply":{"prepared_at":"x","has_patch":true,"has_recipe":false},` +
			`"reconcile":{"outcome":"still_needed","resolved_files":2,"labels":["waiting-on-parent"],` +
			`"patch_id_match":{"our_patch_id":"a","matched_upstream_sha":"b","scanned_range":"c","scanned_count":4}},` +
			`"depends_on":[{"slug":"p","kind":"hard","satisfied_by":"abc"}],` +
			`"verify":{"verified_at":"t","passed":true,"parent_snapshot":{"p":"applied"}},` +
			`"rejection_history":[{"rejected_at":"2026-01-01T00:00:00Z","rejected_by":"me",` +
			`"reason":"r","reject_note":"n","reopened_at":"2026-01-02T00:00:00Z","reopened_by":"me",` +
			`"reopen_note":"n","divergence_detail":[{"path":"p","divergent_reason":"missing"}]}]}`},
		{"live-rejection", `{"state":"rejected","rejection":{"reason":"r","note":"n","actor":"a",` +
			`"evidence":[{"path":"p","sha256":"h"}],"rejected_at":"2026-01-01T00:00:00Z","prior_state":"defined"}}`},
	}
	for _, tc := range accepted {
		t.Run("accepted/"+tc.name, func(t *testing.T) {
			state, ok := decodeStatusDocument([]byte(tc.document))
			if !ok {
				t.Fatalf("decodeStatusDocument rejected a well-formed document: %s", tc.document)
			}
			report, _ := runInspect(t, func(r *fakeRoot) { r.setFile(testStatus, []byte(tc.document)) })
			if report.Abort != nil {
				t.Fatalf("unexpected abort %q", report.Abort.Code)
			}
			if report.FeatureState != state {
				t.Fatalf("feature_state = %q, want %q", report.FeatureState, state)
			}
		})
	}

	t.Run("well-formed-but-unknown-state-is-invalid-state-not-malformed", func(t *testing.T) {
		for _, value := range []string{"prepared", "", strings.Repeat("j", 4096)} {
			report, _ := runInspect(t, func(r *fakeRoot) {
				r.setFile(testStatus, []byte(`{"state":"`+value+`"}`))
			})
			if report.AbortCode() != AbortStatusInvalidState {
				t.Fatalf("abort = %q, want %q", report.AbortCode(), AbortStatusInvalidState)
			}
			if value != "" && strings.Contains(report.Abort.Message, value) {
				t.Fatal("the rejected state value was echoed")
			}
		}
	})
}
