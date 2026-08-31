//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestS7PIB402RegenerationRehydratesTombstonesAndRefusesPendingOwner(t *testing.T) {
	t.Run("tombstone-multireference-rehydrates", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S7 PIB 402 tombstones")
		s7PrepareInitialBundle(t, root, slug)
		bodies := s7WriteControlledIntentBundle(t, root, slug)
		replacements := s7IntentArchiveReplacements(t, bodies, store.IntentArchiveWireTombstoned)
		candidate := intentArchiveCLIGeneration(t, slug, replacements...)

		shared := bodies[store.IntentArchiveArtifactAnalysis]
		sharedElsewhere := intentArchiveCLIReplacement(
			t, store.IntentArchiveArtifactAnalysisSidecar, shared, store.IntentArchiveWireTombstoned,
		)
		elsewhere := intentArchiveCLIGeneration(t, slug, sharedElsewhere)
		initial := intentArchiveCLIIndex(t, slug, candidate, elsewhere)
		writeIntentArchiveCLIFixture(t, root, slug, initial, nil)
		initialIDs := s7GenerationIDs(initial)

		oldIndexRewrite := beforeIndexRewrite
		indexRewrites := 0
		beforeIndexRewrite = func(string) { indexRewrites++ }
		t.Cleanup(func() { beforeIndexRewrite = oldIndexRewrite })

		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--regenerate", "--allow-heuristic", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" || report.Outcome != "published" ||
			report.Archive == nil || report.Archive.GenerationID != candidate.GenerationID {
			t.Fatalf("rehydration = exit:%d stderr:%q report:%+v", code, stderr, report)
		}
		if indexRewrites != 1 {
			t.Fatalf("archive index rewrites = %d, want one CAS publication", indexRewrites)
		}
		_, final := readIntentArchiveCLIIndex(t, root, slug)
		if got := s7GenerationIDs(final); fmt.Sprint(got) != fmt.Sprint(initialIDs) {
			t.Fatalf("rehydration appended/reordered generations: got=%v want=%v", got, initialIDs)
		}
		targetHashes := map[string]bool{}
		for _, replacement := range replacements {
			targetHashes[replacement.ContentSHA256] = true
		}
		revived := 0
		for _, generation := range final.Generations {
			for _, replacement := range generation.Replaced {
				if targetHashes[replacement.ContentSHA256] {
					revived++
					if replacement.WireState() != store.IntentArchiveWireRetained {
						t.Fatalf("target hash was not globally rehydrated: %+v", replacement)
					}
				}
			}
		}
		if revived != len(replacements)+1 || len(report.OrphanBlobs) != 0 {
			t.Fatalf("revived=%d want=%d orphans=%v", revived, len(replacements)+1, report.OrphanBlobs)
		}
		for id, body := range bodies {
			replacement := intentArchiveCLIReplacement(t, id, body, store.IntentArchiveWireRetained)
			blobRel, err := store.IntentArchiveBlobRel(slug, replacement.ContentSHA256)
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(blobRel)))
			if err != nil || !bytes.Equal(got, body) {
				t.Fatalf("%s blob = %q err=%v", id, got, err)
			}
		}
	})

	t.Run("pending-owner-refuses-zero-write", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S7 PIB 402 pending")
		s7PrepareInitialBundle(t, root, slug)
		bodies := s7WriteControlledIntentBundle(t, root, slug)
		shared := bodies[store.IntentArchiveArtifactAnalysis]
		pending := intentArchiveCLIReplacement(
			t, store.IntentArchiveArtifactAnalysis, shared, store.IntentArchiveWireRemovalPending,
		)
		tombstone := intentArchiveCLIReplacement(
			t, store.IntentArchiveArtifactSpec, shared, store.IntentArchiveWireTombstoned,
		)
		index := intentArchiveCLIIndex(t, slug,
			intentArchiveCLIGeneration(t, slug, pending),
			intentArchiveCLIGeneration(t, slug, tombstone),
		)
		writeIntentArchiveCLIFixture(t, root, slug, index, map[string][]byte{
			pending.ContentSHA256: shared,
		})
		before := readTree(t, filepath.Join(root, ".tpatch"))

		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--regenerate", "--allow-heuristic", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		wantRetry := "tpatch feature intent-archive purge " + slug +
			" --blob " + pending.ContentSHA256 + " --yes"
		wantStderr := "error: prepare " + slug + ": regenerate refused recovery-pending\n"
		if code != 3 || stderr != wantStderr || report.Refusal == nil ||
			report.Refusal.Code != "recovery-pending" ||
			report.Refusal.Retry != wantRetry ||
			report.Refusal.RetryCWD != store.IntentArchiveRepairCWD {
			t.Fatalf("pending refusal = exit:%d stderr:%q report:%+v", code, stderr, report)
		}
		if after := readTree(t, filepath.Join(root, ".tpatch")); !bytes.Equal(after, before) {
			t.Fatal("pending-owner refusal changed the workspace")
		}
	})
}

func TestS7PIB403PendingOwnerRecoversBeforeLaterRegeneration(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S7 PIB 403")
	s7PrepareInitialBundle(t, root, slug)
	bodies := s7WriteControlledIntentBundle(t, root, slug)
	pending := s7IntentArchiveReplacements(t, bodies, store.IntentArchiveWireRemovalPending)
	initial := intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, pending...))
	blobs := map[string][]byte{}
	for id, body := range bodies {
		replacement := intentArchiveCLIReplacement(t, id, body, store.IntentArchiveWireRetained)
		blobs[replacement.ContentSHA256] = body
	}
	writeIntentArchiveCLIFixture(t, root, slug, initial, blobs)
	initialIDs := s7GenerationIDs(initial)

	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--blob", pending[0].ContentSHA256, "--yes", "--json", "--quiet",
	)
	purge := decodeIntentArchivePurgeReport(t, stdout)
	if code != 0 || stderr != "" || purge.Outcome != "recovered" ||
		purge.Recovery == nil || len(purge.Recovery.FinalizedHashes) != len(blobs) {
		t.Fatalf("terminal pending recovery = exit:%d stderr:%q report:%+v", code, stderr, purge)
	}
	_, recovered := readIntentArchiveCLIIndex(t, root, slug)
	for _, generation := range recovered.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.WireState() != store.IntentArchiveWireTombstoned {
				t.Fatalf("purge owner did not terminally tombstone pending ref: %+v", replacement)
			}
		}
	}

	code, stdout, stderr, _ = runPrepare(
		t, "--path", root, "prepare", slug,
		"--regenerate", "--allow-heuristic", "--json", "--quiet",
	)
	report := prepareS4Report(t, stdout)
	if code != 0 || stderr != "" || report.Archive == nil ||
		report.Archive.GenerationID != initial.Generations[0].GenerationID {
		t.Fatalf("later regeneration = exit:%d stderr:%q report:%+v", code, stderr, report)
	}
	_, rehydrated := readIntentArchiveCLIIndex(t, root, slug)
	if got := s7GenerationIDs(rehydrated); fmt.Sprint(got) != fmt.Sprint(initialIDs) {
		t.Fatalf("later regeneration changed generation identity/order: got=%v want=%v", got, initialIDs)
	}

	shared := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactAnalysis, bodies[store.IntentArchiveArtifactAnalysis],
		store.IntentArchiveWireRetained,
	)
	code, stdout, stderr, _ = runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--blob", shared.ContentSHA256, "--json", "--quiet",
	)
	preview := decodeIntentArchivePurgeReport(t, stdout)
	if code != 0 || stderr != "" || len(preview.Hashes) != 1 || len(preview.References) != 2 {
		t.Fatalf("later purge live-reference count = exit:%d stderr:%q report:%+v", code, stderr, preview)
	}
}

func TestS7Rev16PendingOwnerErratumGuardAndSensitivities(t *testing.T) {
	input := s7Rev16BaselineEvidence(t)
	t.Run("baseline", func(t *testing.T) {
		if err := validateS7Rev16Evidence(input); err != nil {
			t.Fatal(err)
		}
	})
	fixtures := []struct {
		name   string
		mutate func(s7Rev16Evidence) s7Rev16Evidence
	}{
		{
			name: "old-pending-rehydration-token",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("changes every **tombstoned**\nreference to `h`"),
					[]byte("changes every **tombstoned or removal-pending**\nreference to `h`"), 1)
				return wrong
			},
		},
		{
			name: "fourth-matrix-row-change",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("| PIB-424 | I |"),
					[]byte("| PIB-424 | I | rev-16 changed |"), 1)
				return wrong
			},
		},
		{
			name: "omitted-amendment-ledger-row",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("`PIB-402`, `PIB-403` and `PIB-425`"),
					[]byte("`PIB-402` and `PIB-403`"), 1)
				return wrong
			},
		},
		{
			name: "section-1852-count-drift",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("AM 15, AN 23"),
					[]byte("AM 16, AN 23"), 1)
				return wrong
			},
		},
		{
			name: "adr-revision-mismatch",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.adr = bytes.Replace(wrong.adr,
					[]byte("| rev-16 | **Accepted no-decision erratum — 2026-08-20** |"),
					[]byte("| rev-16 | **Accepted product decision — 2026-08-20** |"), 1)
				return wrong
			},
		},
		{
			name: "rev-18-retired-undetected-removal-reading-restored",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("**The probe→unlink gap is not detected, and is not claimed to be.**"),
					[]byte("**Not detected, and not claimed to be.** Step 3 removes whatever object is at that path."), 1)
				return wrong
			},
		},
		{
			name: "rev-18-revision-row-dropped",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("| rev-18 | **Accepted no-decision erratum — raised 2026-08-28"),
					[]byte("| rev-18b | **Accepted no-decision erratum — raised 2026-08-28"), 1)
				return wrong
			},
		},
		{
			name: "unrelated-prd-normative-edit",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("### 5.1 Authorized grammar (v1, complete)"),
					[]byte("### 5.1 Authorized grammar (v1, complete; unrelated rev-16 edit)"), 1)
				return wrong
			},
		},
		{
			// rev-19: PIB-562's joint `list`/`doctor` exit, restored.
			name: "rev-19-joint-list-and-doctor-exit-restored",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("`list` renders the corrupt object and both residues in one pass and exits **3**; `doctor`'s D9 renders that identical observation set as **warning** findings and exits **0**, which is ADR-035 D16's warning-only rule rather than a second refusal surface. Neither ever names"),
					[]byte("`list` and `doctor` render the corrupt object and both residues in one pass, exit 3, and never name"), 1)
				return wrong
			},
		},
		{
			// rev-19: the impossible exit-0 worked example, restored field by
			// field — outcome first.
			name: "rev-19-worked-example-outcome-purged-restored",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("{\n  \"outcome\": \"refused\",\n  \"action\": \"none\",\n  \"remaining_repairs\": {\n    \"rerun_required\": true,\n    \"stages_remaining\": 2,"),
					[]byte("{\n  \"outcome\": \"purged\",\n  \"action\": \"none\",\n  \"remaining_repairs\": {\n    \"rerun_required\": true,\n    \"stages_remaining\": 2,"), 1)
				return wrong
			},
		},
		{
			// rev-19: `repaired_class` restored beside the manual prerequisite.
			name: "rev-19-worked-example-repaired-class-restored",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("    \"rerun_required\": true,\n    \"stages_remaining\": 2,"),
					[]byte("    \"rerun_required\": true,\n    \"repaired_class\": \"unreferenced-residue\",\n    \"stages_remaining\": 2,"), 1)
				return wrong
			},
		},
		{
			// rev-19: PIB-566's derived population loses the doctor emitter.
			name: "rev-19-pib-566-doctor-emitter-dropped",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte(" — §12.5's `doctor` D9 pending-purge remediation included:"),
					[]byte(":"), 1)
				return wrong
			},
		},
		{
			// rev-19: PIB-566's `retry_cwd` re-widened to every carrier.
			name: "rev-19-pib-566-retry-cwd-rewidened",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("covering exactly the pending set, with no inherited `--path`"),
					[]byte("covering exactly the pending set, with `retry_cwd: \"workspace-root\"` and no inherited `--path`"), 1)
				return wrong
			},
		},
		{
			// rev-19: R25's joint exit, restored.
			name: "rev-19-r25-joint-exit-restored",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("The observation is `archive-blob-corrupt`, zero-write, and pinned across `list`, `doctor` and every ordinary mutation: `list` and every ordinary mutation refuse it at exit **3**, and `doctor`'s D9 renders the identical observation as a **warning** finding and exits **0**, which is ADR-035 D16's warning-only rule. All of them carry one repo-relative procedure:"),
					[]byte("The observation is `archive-blob-corrupt` at exit 3, zero-write, pinned across `list`, `doctor` and every ordinary mutation, with one repo-relative procedure:"), 1)
				return wrong
			},
		},
		{
			// rev-19: the rev-19 revision-history row dropped from the PRD.
			name: "rev-19-revision-row-dropped",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("| rev-19 | **Accepted no-decision erratum — raised 2026-08-28"),
					[]byte("| rev-19b | **Accepted no-decision erratum — raised 2026-08-28"), 1)
				return wrong
			},
		},
		{
			// Joint acceptance: the PRD rev-17 row reverted to proposed.
			name: "rev-17-revision-row-reverted-to-proposed",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("| rev-17 | **Accepted no-decision erratum — raised 2026-08-27, accepted 2026-08-29** |"),
					[]byte("| rev-17 | **Proposed no-decision erratum — 2026-08-27; acceptance pending joint review** |"), 1)
				return wrong
			},
		},
		{
			// Joint acceptance: the ADR rev-19 acceptance block reverted to
			// pending.
			name: "rev-19-adr-acceptance-reverted-to-pending",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.adr = bytes.Replace(wrong.adr,
					[]byte("**Rev-19 acceptance**: 2026-08-29 — **Accepted no-decision erratum**, jointly\nwith rev-17 and rev-18 (raised 2026-08-28)"),
					[]byte("**Rev-19 acceptance**: **pending joint review** (raised 2026-08-28) —\n**proposed no-decision erratum**"), 1)
				return wrong
			},
		},
		{
			// Readiness: §19's stale paused-production authorization wording,
			// restored.
			name: "readiness-authorization-gate-paused-production-restored",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("**The gate is satisfied and closed.** Conditions (1)–(4) are complete: rev-15's\nbounded evidence erratum was jointly approved on 2026-08-18, which lifted the\npre-change-commit-only restriction, and the GH #23 implementation of the §17.2\nproduction order has been accepted through aggregate review. Release\nauthorization is a separate decision and is not recorded by this gate."),
					[]byte("**The sequencing prerequisite is satisfied.** Conditions (1)–(3) are complete\nand GH #23 has dispatched condition (4). Until rev-15's bounded evidence\nerratum is jointly approved, only its pre-change test/golden commit is\nauthorized; mutating production slices remain paused."), 1)
				return wrong
			},
		},
		{
			// Readiness: ADR-035's `Blocks` metadata restored to the rev-15
			// wait over slices S1–S6.
			name: "readiness-adr-blocks-rev15-wait-restored",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.adr = bytes.Replace(wrong.adr,
					[]byte("slices S1–S7). The sequencing prerequisite is satisfied by GH #16, and the\nimplementation authorization gate (PRD §19) is satisfied: GH #23 landed\nS1–S7 and that implementation has been accepted through aggregate review.\nRelease authorization remains a separate decision."),
					[]byte("slices S1–S6). The sequencing prerequisite is satisfied by GH #16; GH #23 may\nland pre-change evidence, while mutating production slices wait for rev-15's\nbounded D14/PIB-391 evidence erratum to be jointly approved."), 1)
				return wrong
			},
		},
		{
			// Readiness: the ADR index row's blocking-prerequisite language,
			// restored.
			name: "readiness-adr-index-blocking-prerequisite-restored",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.index = bytes.Replace(wrong.index,
					[]byte("rev-20. Decisions D1–D21. The mutating implementation (PRD §17.2 slices S1–S7, GH #23) and its acceptance are complete; release authorization is pending."),
					[]byte("rev-14. Decisions D1–D21. Mutating implementation is blocked by the `prepare --check` implementation prerequisite, not by planning review."), 1)
				return wrong
			},
		},
		{
			// Readiness: §12.6's D6 delta promising an interpolated panic
			// detail again.
			name: "readiness-doctor-panic-detail-promise-restored",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("The recovered-panic finding message is a constant for **every** doctor check, so D1–D8 no longer interpolate the panic value either"),
					[]byte("The recovered-panic finding message still includes the panic value for every doctor check, so D1–D8 are untouched"), 1)
				return wrong
			},
		},
		{
			// rev-19: ADR D10's joint refusal sentence, restored.
			name: "rev-19-adr-d10-joint-refusal-restored",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.adr = bytes.Replace(wrong.adr,
					[]byte("`list` and\nevery ordinary mutation refuse it zero-write at exit **3** — and, since rev-13,\nso does **every confirmed purge selector in the archive**, because it is the\nrank-1 blocking repair class (PIB-561); `doctor`'s D9 renders the identical\nobservation as a **warning** finding and exits **0**, which is the warning-only\ndisposition D16 already states for that surface, not a fourth refusal."),
					[]byte("It refuses\nzero-write for `list`, `doctor` and every ordinary mutation — and, since rev-13,\nfor **every confirmed purge selector in the archive**, because it is the rank-1\nblocking repair class (PIB-561)."), 1)
				return wrong
			},
		},
		{
			// rev-19: an ADR `retry_cwd` claim D16's pending-route rule never
			// needed, injected into D10.
			name: "rev-19-adr-d10-gains-a-retry-cwd-claim",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.adr = bytes.Replace(wrong.adr,
					[]byte("restore the exact correct blob and retry. The destructive cost and the\nGit-history caveat are stated with it."),
					[]byte("restore the exact correct blob and retry. The destructive cost and the\nGit-history caveat are stated with it, and every surface carries `retry_cwd`."), 1)
				return wrong
			},
		},
		{
			// rev-20: the retired unknown-id reading, restored in §9.7.2.
			name: "rev-20-unknown-id-classified-as-index-corrupt",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("exit 3 (`archive-selector-invalid` for a well-formed `--blob` hash the index does not carry"),
					[]byte("exit 3 (`archive-index-corrupt` for an unknown id"), 1)
				return wrong
			},
		},
		{
			// rev-20's own correction: a malformed value is an exit-1 usage
			// error the command rejects before any archive read, so §9.7.2 may
			// not promise it the public exit-3 code.
			name: "rev-20-malformed-value-promised-a-public-refusal-code",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("exit 3 (`archive-selector-invalid` for a well-formed `--blob` hash the index does not carry"),
					[]byte("exit 3 (`archive-selector-invalid` for a malformed id, for a well-formed `--blob` hash the index does not carry"), 1)
				return wrong
			},
		},
		{
			// rev-20: the closed catalog loses the new code, so the emitters
			// have nothing to surface and the count falls back to 53.
			name: "rev-20-catalog-drops-the-selector-code",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				start := bytes.Index(wrong.prd, []byte("| `archive-selector-invalid` | 3 |"))
				if start < 0 {
					return wrong
				}
				end := bytes.IndexByte(wrong.prd[start:], '\n')
				wrong.prd = append(append([]byte(nil), wrong.prd[:start]...), wrong.prd[start+end+1:]...)
				return wrong
			},
		},
		{
			// rev-20: PIB-431 loses the renderer-scoped no-relabel obligation,
			// which is the row-level statement the CLI defect violated. The
			// blanket "no emitter anywhere" reading is also wrong — the
			// `archive-storage-failed` mapping is a legitimate classification —
			// so restoring it must fail too.
			name: "rev-20-pib-431-drops-the-scoped-no-relabel-obligation",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("**the typed archive refusal renderer surfaces an already-public catalog code unchanged**"),
					[]byte("**no emitter rewrites a classified code into a different catalog code**"), 1)
				return wrong
			},
		},
		{
			// rev-20: PIB-431 keeps the obligation but withdraws the explicit
			// permission for a boundary to classify an internal, non-catalog
			// transport failure — which would make the row false about
			// `prepareStoreArchiveFailure`.
			name: "rev-20-pib-431-forbids-internal-transport-classification",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte(" The obligation is scoped to that renderer and claims nothing about boundaries the validator does not read: an internal, non-catalog transport failure such as `archive-storage-failed` is legitimately classified into a catalog code by its owning boundary, which is what `prepareStoreArchiveFailure` does, and that mapping stays permitted."),
					[]byte(""), 1)
				return wrong
			},
		},
		{
			// rev-20: PIB-465 keeps the selector row but drops the
			// absent-archive population, which is the reported defect.
			name: "rev-20-pib-465-drops-the-absent-archive-population",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("over an absent/empty archive as well as a populated healthy one"),
					[]byte("over a populated healthy archive"), 1)
				return wrong
			},
		},
		{
			// rev-20: PIB-465 keeps the exit-3 half but drops the exit-1 half,
			// so nothing pins the malformed population as a non-report one.
			name: "rev-20-pib-465-drops-the-exit-1-non-report-half",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("Its exit-1 half is asserted separately and as a **non**-report population"),
					[]byte("Its exit-1 half is out of scope"), 1)
				return wrong
			},
		},
		{
			// rev-20: the disposition downgraded to the erratum shape its
			// predecessors use, although the emitted closed code changes.
			name: "rev-20-recorded-as-a-no-decision-erratum",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("| rev-20 | **Accepted bounded public diagnostic-vocabulary amendment — 2026-08-30** |"),
					[]byte("| rev-20 | **Accepted no-decision erratum — raised 2026-08-30, accepted 2026-08-30** |"), 1)
				return wrong
			},
		},
		{
			// Acceptance: the PRD revision-history row reverted to the
			// pre-acceptance proposed/pending disposition.
			name: "rev-20-revision-row-reverted-to-proposed",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("| rev-20 | **Accepted bounded public diagnostic-vocabulary amendment — 2026-08-30** |"),
					[]byte("| rev-20 | **Proposed bounded public diagnostic-vocabulary amendment — raised 2026-08-30; acceptance pending review** |"), 1)
				return wrong
			},
		},
		{
			// Acceptance: the PRD status header reverted to the proposed
			// clause with acceptance still pending.
			name: "rev-20-prd-status-header-reverted-to-pending",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("diagnostic-vocabulary amendment (accepted 2026-08-30) — not a no-decision\nerratum, because the closed code a selector that names nothing emits changes**"),
					[]byte("diagnostic-vocabulary amendment (proposed 2026-08-30) — not a no-decision\nerratum, because the closed code a selector that names nothing emits changes —\nacceptance pending review**"), 1)
				return wrong
			},
		},
		{
			// Acceptance: the PRD acceptance block reverted to the disposition
			// block it replaced, losing the external finding and the three
			// internal correction reviews.
			name: "rev-20-prd-acceptance-block-reverted-to-disposition",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("**Rev-20 acceptance**: **Accepted bounded public diagnostic-vocabulary\namendment — 2026-08-30**, accepted after the post-close external finding that\nraised it and **three** internal correction reviews of the drafted amendment,\nwith the semantics that were reviewed unchanged: the same two amended rows\n`PIB-431` and `PIB-465`, the same **567**-row matrix with unchanged §18.52\narithmetic and **thirty-six** §18.53 semantic guards, and no D1–D21 change\noutside D16's diagnostic vocabulary."),
					[]byte("**Rev-20 disposition**: **Proposed 2026-08-30 — bounded public\ndiagnostic-vocabulary amendment, acceptance pending review.**"), 1)
				return wrong
			},
		},
		{
			// Acceptance: §18.1's rev-20 paragraph reverted to proposed with
			// acceptance pending review.
			name: "rev-20-section-18.1-reverted-to-pending",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("erratum** — it was **accepted** on 2026-08-30, after the post-close external\nfinding that raised it and **three** internal correction reviews of the drafted\namendment, with the reviewed semantics unchanged by acceptance — and it\namends exactly two stable rows"),
					[]byte("erratum** — it is **proposed** on 2026-08-30 with acceptance **pending\nreview** — and it\namends exactly two stable rows"), 1)
				return wrong
			},
		},
		{
			// Acceptance: the ADR status header reverted to proposed with
			// acceptance pending review.
			name: "rev-20-adr-status-header-reverted-to-pending",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.adr = bytes.Replace(wrong.adr,
					[]byte("public diagnostic-vocabulary amendment (accepted 2026-08-30) — not a\nno-decision erratum**"),
					[]byte("public diagnostic-vocabulary amendment (proposed 2026-08-30) — not a\nno-decision erratum, and acceptance pending review**"), 1)
				return wrong
			},
		},
		{
			// Acceptance: the ADR acceptance block reverted to the disposition
			// block it replaced.
			name: "rev-20-adr-acceptance-block-reverted-to-disposition",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.adr = bytes.Replace(wrong.adr,
					[]byte("**Rev-20 acceptance**: **Accepted bounded public diagnostic-vocabulary\namendment — 2026-08-30**, accepted with the companion PRD's rev-20 after the\npost-close external finding that raised it and **three** internal correction\nreviews of the drafted amendment, with the semantics that were reviewed\nunchanged: the same companion row pair `PIB-431`/`PIB-465`, the same **567**-row\nmatrix with **thirty-six** semantic guards, and no D1–D21 change outside D16's\ndiagnostic vocabulary."),
					[]byte("**Rev-20 disposition**: **Proposed 2026-08-30 — bounded public\ndiagnostic-vocabulary amendment, acceptance pending review.**"), 1)
				return wrong
			},
		},
		{
			// Acceptance: the ADR revision-history row reverted to the
			// pre-acceptance proposed/pending disposition.
			name: "rev-20-adr-revision-row-reverted-to-proposed",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.adr = bytes.Replace(wrong.adr,
					[]byte("| rev-20 | **Accepted bounded public diagnostic-vocabulary amendment — 2026-08-30** |"),
					[]byte("| rev-20 | **Proposed bounded public diagnostic-vocabulary amendment — raised 2026-08-30; acceptance pending review** |"), 1)
				return wrong
			},
		},
		{
			// Acceptance: the PRD's companion-revision metadata left behind at
			// rev-19, so the paired acceptance is unrecorded on this side.
			name: "rev-20-prd-architecture-metadata-stops-at-rev-19",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("(**Accepted 2026-08-14**, rev-20;"),
					[]byte("(**Accepted 2026-08-14**, rev-19;"), 1)
				return wrong
			},
		},
		{
			// Acceptance: the ADR's companion metadata left behind at rev-19.
			name: "rev-20-adr-companion-metadata-stops-at-rev-19",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.adr = bytes.Replace(wrong.adr,
					[]byte("(rev-20 Accepted;"),
					[]byte("(rev-19 Accepted;"), 1)
				return wrong
			},
		},
		{
			// rev-20: the §18.1 ledger claims a row the diff does not change.
			name: "rev-20-ledger-claims-a-third-row",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("amends exactly two stable rows: `PIB-431` and `PIB-465`"),
					[]byte("amends exactly three stable rows: `PIB-347`, `PIB-431` and `PIB-465`"), 1)
				return wrong
			},
		},
		{
			// rev-20: §6.4's partition claims a dry-run evaluates the purge
			// selector it never reaches.
			name: "rev-20-selector-code-moved-into-the-dry-run-evaluated-column",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("`archive-purge-index-changed`, `archive-selector-invalid`, `archive-blob-shared`"),
					[]byte("`archive-purge-index-changed`, `archive-blob-shared`"), 1)
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("| `archive-index-corrupt`, `archive-index-version-unsupported`, `archive-index-foreign`, `archive-index-path-escape`, `archive-index-generation-mismatch`, `archive-blob-dangling`"),
					[]byte("| `archive-selector-invalid`, `archive-index-corrupt`, `archive-index-version-unsupported`, `archive-index-foreign`, `archive-index-path-escape`, `archive-index-generation-mismatch`, `archive-blob-dangling`"), 1)
				return wrong
			},
		},
		{
			// rev-20: ADR-035's D16 vocabulary paragraph gains the structured
			// retry carrier §10.4.1 deliberately withholds.
			name: "rev-20-adr-d16-selector-paragraph-gains-a-retry-carrier",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.adr = bytes.Replace(wrong.adr,
					[]byte("workspace root — a value to rerun with, or the fact that this archive holds\nnone."),
					[]byte("workspace root — a value to rerun with, or the fact that this archive holds\nnone, carried as `retry` with `retry_cwd: \"workspace-root\"`."), 1)
				return wrong
			},
		},
		{
			name: "unrelated-adr-normative-edit",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.adr = bytes.Replace(wrong.adr,
					[]byte("### D1 — Three separate guarantees; only two are claimed, and one is scoped"),
					[]byte("### D1 — Three separate guarantees; rev-16 changed another decision"), 1)
				return wrong
			},
		},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			if err := validateS7Rev16Evidence(fixture.mutate(input)); err == nil {
				t.Fatal("same rev-16 validator accepted the one-delta wrong input")
			}
		})
	}
}

func s7Rev16BaselineEvidence(t *testing.T) s7Rev16Evidence {
	t.Helper()
	root := avpRepoRoot(t)
	prd, err := os.ReadFile(filepath.Join(root, "docs", "prds", "PRD-prepare-intent-bundle.md"))
	if err != nil {
		t.Fatal(err)
	}
	adr, err := os.ReadFile(filepath.Join(root, "docs", "adrs", "ADR-035-intent-bundle-publication-and-history.md"))
	if err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(filepath.Join(root, "docs", "adrs", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	const base = "2d9492cbf6fd9c69c5aa75d64d05983c05e1563f"
	basePRD := s7GitFileAt(t, root, base, "docs/prds/PRD-prepare-intent-bundle.md")
	baseADR := s7GitFileAt(t, root, base, "docs/adrs/ADR-035-intent-bundle-publication-and-history.md")
	baseIndex := s7GitFileAt(t, root, base, "docs/adrs/README.md")
	diffCommand := exec.Command(
		"git", "diff", base, "--",
		"docs/prds/PRD-prepare-intent-bundle.md",
		"docs/adrs/ADR-035-intent-bundle-publication-and-history.md",
	)
	diffCommand.Dir = root
	diff, err := diffCommand.Output()
	if err != nil {
		t.Fatal(err)
	}
	return s7Rev16Evidence{
		base: base, prd: prd, adr: adr, index: index,
		basePRD: basePRD, baseADR: baseADR, baseIndex: baseIndex, diff: diff,
	}
}

type s7Rev16Evidence struct {
	base                        string
	prd, adr, index             []byte
	basePRD, baseADR, baseIndex []byte
	diff                        []byte
}

type s7Rev16MatrixRow struct {
	id, kind, category, text string
}

func validateS7Rev16Evidence(input s7Rev16Evidence) error {
	if input.base != "2d9492cbf6fd9c69c5aa75d64d05983c05e1563f" {
		return fmt.Errorf("rev-16 base = %q", input.base)
	}
	if err := validateS7Rev16PendingOwnerErratum(input.prd, input.adr); err != nil {
		return err
	}
	if err := validateS7Rev19RecordErratum(input.prd, input.adr, input.index); err != nil {
		return err
	}
	if err := validateS7Rev20SelectorAmendment(input.prd, input.adr); err != nil {
		return err
	}
	if err := validateS7Rev16DocumentDiffs(input); err != nil {
		return err
	}
	currentRows, currentCategories, err := s7ParseFullMatrix(string(input.prd))
	if err != nil {
		return err
	}
	baseRows, _, err := s7ParseFullMatrix(string(input.basePRD))
	if err != nil {
		return fmt.Errorf("base matrix: %w", err)
	}
	if len(currentRows) != 567 || len(baseRows) != 567 {
		return fmt.Errorf("rev-16 matrix rows current=%d base=%d", len(currentRows), len(baseRows))
	}
	changed := []string{}
	for index := 1; index <= 567; index++ {
		id := fmt.Sprintf("PIB-%03d", index)
		current, currentOK := currentRows[id]
		base, baseOK := baseRows[id]
		if !currentOK || !baseOK || current.kind != base.kind || current.category != base.category {
			return fmt.Errorf("rev-16 changed row identity/kind/category %s: current=%+v base=%+v", id, current, base)
		}
		if current.text != base.text {
			changed = append(changed, id)
		}
	}
	wantChanged := []string{
		"PIB-402", "PIB-403", "PIB-425",
		// rev-20's two amended rows: the refusal-renderer guard and the
		// §9.7.2 preflight-row fixture set.
		"PIB-431", "PIB-465",
		"PIB-542", "PIB-543", "PIB-544",
		"PIB-550",
		"PIB-562", "PIB-566",
	}
	if fmt.Sprint(changed) != fmt.Sprint(wantChanged) {
		return fmt.Errorf("rev-16 actual matrix row changes = %v, want %v", changed, wantChanged)
	}
	diffRows := map[string]bool{}
	diffPattern := regexp.MustCompile(`^[+-]\| (PIB-[0-9]{3}) \|`)
	for _, line := range strings.Split(string(input.diff), "\n") {
		if match := diffPattern.FindStringSubmatch(line); match != nil {
			diffRows[match[1]] = true
		}
	}
	if fmt.Sprint(sortedS7BoolKeys(diffRows)) != fmt.Sprint(wantChanged) {
		return fmt.Errorf("git diff %s changed matrix rows %v", input.base, sortedS7BoolKeys(diffRows))
	}
	if err := s7ValidateSection1852(string(input.prd), currentRows, currentCategories); err != nil {
		return err
	}
	for label, document := range map[string][]byte{
		"PRD current": input.prd, "ADR current": input.adr,
	} {
		revisions, err := s7RevisionHistory(document)
		if err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		if len(revisions) < 3 ||
			revisions[len(revisions)-3] != 18 ||
			revisions[len(revisions)-2] != 19 ||
			revisions[len(revisions)-1] != 20 {
			return fmt.Errorf("%s revision predecessor = %v", label, revisions)
		}
	}
	for label, document := range map[string][]byte{
		"PRD base": input.basePRD, "ADR base": input.baseADR,
	} {
		revisions, err := s7RevisionHistory(document)
		if err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		if len(revisions) == 0 || revisions[len(revisions)-1] != 15 {
			return fmt.Errorf("%s does not end at rev-15: %v", label, revisions)
		}
	}
	prdRev16 := s7RevisionRow(string(input.prd), 16)
	adrRev16 := s7RevisionRow(string(input.adr), 16)
	for label, row := range map[string]string{"PRD": prdRev16, "ADR": adrRev16} {
		for _, token := range []string{
			"Accepted no-decision erratum",
			"No decision",
			"row",
			"kind",
			"count",
		} {
			if !strings.Contains(strings.ToLower(row), strings.ToLower(token)) {
				return fmt.Errorf("%s rev-16 history lacks no-decision/count claim %q: %s", label, token, row)
			}
		}
	}
	// rev-17, rev-18 and rev-19 were raised as proposed no-decision errata and
	// accepted jointly with ADR-035's paired halves on 2026-08-29. Their rows
	// therefore carry the accepted disposition with both dates, and the same
	// no-decision claim rev-16's row makes; the PRD halves of rev-18 and rev-19
	// additionally state the amended rows and the unchanged counts.
	prdRev17 := s7RevisionRow(string(input.prd), 17)
	adrRev17 := s7RevisionRow(string(input.adr), 17)
	for label, row := range map[string]string{"PRD": prdRev17, "ADR": adrRev17} {
		if !strings.Contains(row, "**Accepted no-decision erratum — raised 2026-08-27, accepted 2026-08-29**") {
			return fmt.Errorf("%s rev-17 history is not a jointly accepted erratum: %s", label, row)
		}
	}
	prdRev18 := s7RevisionRow(string(input.prd), 18)
	adrRev18 := s7RevisionRow(string(input.adr), 18)
	for label, row := range map[string]string{"PRD": prdRev18, "ADR": adrRev18} {
		for _, token := range []string{
			"Accepted no-decision erratum — raised 2026-08-28, accepted 2026-08-29",
			"No decision",
		} {
			if !strings.Contains(strings.ToLower(row), strings.ToLower(token)) {
				return fmt.Errorf("%s rev-18 history lacks no-decision claim %q: %s", label, token, row)
			}
		}
	}
	for _, token := range []string{"PIB-550", "row", "kind", "count", "567", "thirty-six"} {
		if !strings.Contains(strings.ToLower(prdRev18), strings.ToLower(token)) {
			return fmt.Errorf("PRD rev-18 history lacks %q: %s", token, prdRev18)
		}
	}
	// rev-19 is the AX record erratum: two amended matrix rows plus three
	// non-matrix companion corrections, accepted with rev-17 and rev-18.
	prdRev19 := s7RevisionRow(string(input.prd), 19)
	adrRev19 := s7RevisionRow(string(input.adr), 19)
	for label, row := range map[string]string{"PRD": prdRev19, "ADR": adrRev19} {
		for _, token := range []string{
			"Accepted no-decision erratum — raised 2026-08-28, accepted 2026-08-29",
			"No decision",
		} {
			if !strings.Contains(strings.ToLower(row), strings.ToLower(token)) {
				return fmt.Errorf("%s rev-19 history lacks no-decision claim %q: %s", label, token, row)
			}
		}
	}
	for _, token := range []string{
		"PIB-562", "PIB-566", "row", "kind", "count", "567", "thirty-six",
		"amended rows, exactly", "**two**",
	} {
		if !strings.Contains(strings.ToLower(prdRev19), strings.ToLower(token)) {
			return fmt.Errorf("PRD rev-19 history lacks %q: %s", token, prdRev19)
		}
	}
	for _, token := range []string{"D10", "D16", "no `retry_cwd` claim is added"} {
		if !strings.Contains(strings.ToLower(adrRev19), strings.ToLower(token)) {
			return fmt.Errorf("ADR rev-19 history lacks %q: %s", token, adrRev19)
		}
	}
	// The retired rev-11 readings the erratum replaced must be gone from both
	// documents' current normative text, exactly as rev-16's superseded
	// rehydration wording is.
	for label, retired := range map[string][]string{
		"PRD": {
			"step 3 removes whatever object is at that path",
			"not detected, and not claimed to be",
			// rev-19: the joint `list`/`doctor` exit readings.
			"and `doctor` render the corrupt object and both residues in one pass, exit 3",
			"at exit 3, zero-write, pinned across `list`, `doctor` and every ordinary mutation",
			// rev-19: the impossible exit-0 worked example.
			"\"repaired_class\": \"unreferenced-residue\"",
		},
		"ADR": {
			"which the unlink cannot be conditioned on",
			"so the replacement is what gets removed",
			// rev-19: D10's joint refusal sentence.
			"zero-write for `list`, `doctor` and every ordinary mutation",
		},
	} {
		document := s7CurrentNormativeText(string(input.prd))
		if label == "ADR" {
			document = s7CurrentNormativeText(string(input.adr))
		}
		for _, phrase := range retired {
			if strings.Contains(document, strings.ToLower(phrase)) {
				return fmt.Errorf("%s rev-18 current normative text retains the retired reading %q", label, phrase)
			}
		}
	}
	if strings.Contains(s7CurrentNormativeText(string(input.prd)), "tombstoned/pending rehydration") ||
		strings.Contains(s7CurrentNormativeText(string(input.prd)), "tombstoned or removal-pending") ||
		strings.Contains(s7CurrentNormativeText(string(input.adr)), "tombstoned or removal-pending") {
		return fmt.Errorf("rev-16 current normative text retains stale pending rehydration")
	}
	return nil
}

type s7Rev16AllowedRegion struct {
	label       string
	heading     string
	baseHash    string
	currentHash string
}

type s7Rev16DocumentSpan struct {
	label      string
	start, end int
}

func validateS7Rev16DocumentDiffs(input s7Rev16Evidence) error {
	documents := []struct {
		label          string
		base, current  []byte
		allowedRegions []s7Rev16AllowedRegion
	}{
		{
			label: "PRD",
			base:  input.basePRD, current: input.prd,
			allowedRegions: []s7Rev16AllowedRegion{
				{label: "status-and-acceptance-header", heading: "<header>", baseHash: "a3aa799f8ab92acf0d16bcafe716977ad9b2d8279c84670a1736595e3d523f2a", currentHash: "31202b6e21780e5a911726edb715387636aad2069850242d72de31597d94e17c"},
				{label: "revision-history", heading: "## Revision history", baseHash: "d9b6aff94362d6bdc8cbe89eec06f514d5d3eccae4175673a84c5771a669067c", currentHash: "a959242dfc73707cbad30ca48f73e0147a2085ee32c4a3158854c6530bc76314"},
				// rev-20: §6.4's dry-run partition gains the new selector code in
				// its not-evaluated column.
				{label: "section-6.4-dry-run-partition", heading: "### 6.4 `--dry-run` — plan, not outcome", baseHash: "2b13832a1f351b834a7f9f8c9329c696cee1f6c0a50cffb4ac77b26bbd536422", currentHash: "da4c7e30b7d41ca1d675b0b52f1cfd60a6e33cc2b944463d31a207c53b6b3164"},
				{label: "section-9.3-rehydration", heading: "### 9.3 `index.json`", baseHash: "fd9f4f35b499a2b17a60c6d256b9c1ff2448b57ef750e9f1f367505406e5ecc9", currentHash: "e8298d7394da53eee093bfc0b3b1e7a61bea0c8579fea731922b63f6a5e98254"},
				{label: "section-9.7.1-consistency", heading: "#### 9.7.1 Selection and shared references", baseHash: "3210b5cf279b85e0f53e3d9edaa3caae1677849392155d06690455ae59f3287c", currentHash: "a2a59fd0907b0948bf59e8453aefc45b0731364ac8dff6911cc1cedef5b0b4a7"},
				{label: "section-9.7.2-residual-window", heading: "#### 9.7.2 Honest purge procedure and residual race", baseHash: "efe42995f891dce9e1bcef16fba4c9b8888898a67f0e5734d465f2ad8da14734", currentHash: "49660792f52cc19dad5bf0d65e5e75b48f882334ed1c9d112e360a78a50b9b3b"},
				// rev-19: §10.2's worked sequential-repair example becomes the
				// exit-3 archive-integrity refusal form it always had to be.
				{label: "section-10.2-purge-report-example", heading: "### 10.2 JSON report (v1)", baseHash: "b07b4080df837ee3ffdad9ce4dd6cc8d7fd892103d9309623daefbc947945102", currentHash: "108addefcf728a9d9b48ef8e6a07c6a4ab1a435d3ff542e0beb36b1d427a8bdf"},
				// rev-20: §10.4.1 gains the archive-selector-invalid row, so the
				// closed catalog grows from 53 to 54 codes.
				{label: "section-10.4.1-refusal-catalog", heading: "### 10.4.1 Closed refusal catalog — code, exit, human and JSON shape", baseHash: "42bb98b4b61a2aa9e3d8f55f414b486357897c9f4e113fe8b6afcb53d062113f", currentHash: "2282fa5e58d00c6d3fd92a8310801c1a0710bb8499719e696ffe814f07e0eaae"},
				// Readiness: §12.6's D6 delta records that the recovered-panic
				// finding message is constant for every doctor check.
				{label: "section-12.6-behavior-deltas", heading: "### 12.6 Enumerated behavior deltas — the complete list", baseHash: "87c382a287ec444a8eb6d410640d7f244bf5ccb1aa0c707599bb8e79d135dbad", currentHash: "1b8b2cc209014dec1a96e98850b6ca8de8493bd040a41f769df94d06f5846993"},
				// rev-19: R25's joint `list`/`doctor`/mutation exit is split.
				{label: "section-16-risks", heading: "## 16. Risks", baseHash: "8b245e8716ede244c3025597bb2a94f808c9839403acdc892447d536bd96448d", currentHash: "e9762690d2631ddb1f33c916404c4a771c061d9eb8a7fcd8e87cec3bb05925f9"},
				{label: "mechanical-slice-summary", heading: "### 17.2 Slices", baseHash: "32831b33b9200267975945357a3f035ad8ac84d8c91720496007986fcdea8f2a", currentHash: "d18b0764ee3c2985016ba5efe2c6aca5d1c32765ee9844e32a1dacdfd82aca05"},
				{label: "section-18.1-amendment-ledger", heading: "### 18.1 How to read this matrix", baseHash: "2d07413506629fc420e5e2768bb446633a2579195a60189820cc6f16187b91cd", currentHash: "dc3c69e7beaf4c1a1deb078b5a9f6cfe2354de0370d6f3e1ff40e07b355fdb12"},
				{label: "matrix-pib-402-403", heading: "### 18.40 AM — Rev-2 adjudication rows, amended by rev-3", baseHash: "ea342198fd9ecfe95385245fb29af8f89dab05332ea7d63947a488907637a2b6", currentHash: "307e6c60f1b23cd64ca254aaef2e49135eb0c0cf33475e8fc887c9b5b7def16a"},
				{label: "matrix-pib-425-431", heading: "### 18.41 AN — Rev-3 adjudication: directory authority, privacy and archive truth", baseHash: "c3812ced4e0254b749cc1b9e24ecf8b284223678619de5966aa16f25e50ed3a0", currentHash: "079807566d8f011be65bdec81e4e3383e96e38a12a5573a69d10588eae0d0c5d"},
				// rev-20: PIB-465 owns the §9.7.2 preflight-row fixture set.
				{label: "matrix-pib-465", heading: "### 18.43 AP — Rev-5 adjudication: reachability, rooted control writes and bounded totality", baseHash: "e05f2dc51cfbb947ecbf2ba6165ca7acb10496310268cea81c30558b1fab7b14", currentHash: "0b1dbbee9c424b8924c43bfab551d3d4650165fa358ec4348f962e70d93104b1"},
				{label: "matrix-pib-542-544", heading: "### 18.48 AU — Rev-10 adjudication: global pending ownership, selector-independent validation and the corrupt-blob route", baseHash: "7b8dc7b2b98655beef1f5f03706cc832d21a48699418d24ffc13396d8d237d01", currentHash: "0c5a59211d322812d6c14dd37cd99b2ffc78fc60d2df43602bc36435fab37098"},
				{label: "matrix-pib-550", heading: "### 18.49 AV — Rev-11 adjudication: total same-hash claim, the recovery exception, type-total removal and repair-class multiplicity", baseHash: "c7936d0efed11b970c6e5f9c802ee31209effc6fa12294d1674b2c704d748486", currentHash: "7a1a865a60b2fcd8d189bf1b1f6796a6ee2a365a6f451bf2f9e5a2036d0f5d08"},
				// rev-19: the two amended matrix rows, PIB-562 and PIB-566.
				{label: "matrix-pib-562-566", heading: "### 18.51 AX — Rev-13 adjudication: corrupt-first ordering, repair stages, pending-route narrowing and ledger parity", baseHash: "b93accf062371958db4f645bc9c4146008f9239306c87f0a9473a848b5fd42c2", currentHash: "5c560e913cb3c8591529ed0bc660b2b6542f0fe650183bf61707b5e46a8b624c"},
				{label: "section-18.53-sensitivities", heading: "### 18.53 Sensitivity requirement", baseHash: "231a50903312049358535b16f39da9148cf5733c50f5a24ebfa53d9ce535f15a", currentHash: "4cabfe5672743748dae062b97a4bfe2a0c235d39bb05ed956a801d8dfb484fd7"},
				// Readiness: §19's gate records conditions (1)–(4) complete and
				// the accepted GH #23 implementation, without a release claim.
				{label: "section-19-authorization-gate", heading: "## 19. Implementation authorization gate", baseHash: "360f2df976a308bae2f5bc541c5b6532f3b9f360a30c1ff1a9ce2aa2fade1375", currentHash: "a561571e0f058cb97454c4744c0e8266e89c8a8768cd98cfb3c006aa89a483a4"},
				{label: "section-21-alternatives", heading: "## 21. Alternatives considered (summary)", baseHash: "ec652ea2de6e47354d9c0229ebd1035ea6a5e17d3b171c94d1be32f040be0c7f", currentHash: "182e9da7f69590a01438dea6e266114a984c6664ca688479878048e3ec7373a2"},
			},
		},
		{
			label: "ADR",
			base:  input.baseADR, current: input.adr,
			allowedRegions: []s7Rev16AllowedRegion{
				{label: "status-revision-amendment-ledgers", heading: "<header>", baseHash: "09f50199314281aa28d17c63bbb25519738c4ffcefb3c2a2c2d960380a7e1ab6", currentHash: "935e5a8a5de61deb87143ff5a5dcc511efb732d45accf274448d7e5878c43d86"},
				{label: "D10-pending-owner", heading: "### D10 — Content-addressed, deterministic identifiers; no wall-clock in tracked bytes", baseHash: "04b8affc6125b69e4191b2b30d6cb8c9760e0ab872abf9aa520aa53040a4b4e5", currentHash: "76a7b43b42f63214620a4712ff96dfcef906d2e9944ec4b70dcf62c6566e9d33"},
				{label: "D13-terminal-owner-precedence", heading: "### D13 — Recovery has three entry points, it is terminal, the operator's runs *instead of* the automatic ones, and the diagnostic touches nothing", baseHash: "77633643c1cc2c1b5607ba673cfe8af6b17d25d47510e1c6904b0a0f50c6cbb1", currentHash: "b5b45839dc04492a0673aa44b232d7138fff88cc7b8657072feb26c0c01a454f"},
				{label: "D16-purge-owner", heading: "### D16 — Retention is bounded: listing, purging, tombstones and orphans", baseHash: "ee66479c3eae9c1ed0af5de192d63e088d00ddc3843aaf9c8666151909e2ec41", currentHash: "f556bc199ff3afaf15fbd2f13dfe1529b6522f06770647c4bad47f6b40918d6d"},
				{label: "adr-alternatives-considered", heading: "## Alternatives considered", baseHash: "fa303b87ef6fd1054570916d0dbcead3f770f8760b6177a66eb714d2319a5649", currentHash: "3f5062fda963eaf676a24821dd09e829ae36555c2cb9f2925641e17b9e586ac8"},
			},
		},
		{
			// Readiness: the ADR index row is the third frozen document. Only
			// its `## Index` block may move, and only for ADR-035's own row.
			label: "ADR index",
			base:  input.baseIndex, current: input.index,
			allowedRegions: []s7Rev16AllowedRegion{
				{label: "adr-index-entries", heading: "## Index", baseHash: "2c12ff44a6aa1d8efb52a1786982ee9bd9fbc0d45af76bf375c0c70d4a2f4bca", currentHash: "a0bb77a779b346063d0960bbd9ca3b0684fef0f9c4e68f25aae860e3c96fe976"},
			},
		},
	}
	for _, document := range documents {
		baseMasked, err := s7MaskRev16AllowedRegions(document.base, document.allowedRegions, false)
		if err != nil {
			return fmt.Errorf("%s base allowlist: %w", document.label, err)
		}
		currentMasked, err := s7MaskRev16AllowedRegions(document.current, document.allowedRegions, true)
		if err != nil {
			return fmt.Errorf("%s current allowlist: %w", document.label, err)
		}
		if !bytes.Equal(baseMasked, currentMasked) {
			return fmt.Errorf("%s rev-16 changed bytes outside the explicit allowlist", document.label)
		}
	}
	return nil
}

func s7MaskRev16AllowedRegions(
	document []byte,
	regions []s7Rev16AllowedRegion,
	current bool,
) ([]byte, error) {
	spans := make([]s7Rev16DocumentSpan, 0, len(regions))
	for _, region := range regions {
		start, end, err := s7Rev16RegionSpan(document, region.heading)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", region.label, err)
		}
		wantHash := region.baseHash
		if current {
			wantHash = region.currentHash
		}
		sum := fmt.Sprintf("%x", sha256.Sum256(document[start:end]))
		if sum != wantHash {
			return nil, fmt.Errorf("%s hash = %s, want %s", region.label, sum, wantHash)
		}
		spans = append(spans, s7Rev16DocumentSpan{label: region.label, start: start, end: end})
	}
	sort.Slice(spans, func(left, right int) bool { return spans[left].start > spans[right].start })
	masked := append([]byte(nil), document...)
	for _, span := range spans {
		marker := []byte("\n<<rev16-allowlist:" + span.label + ">>\n")
		masked = append(masked[:span.start], append(marker, masked[span.end:]...)...)
	}
	return masked, nil
}

func s7Rev16RegionSpan(document []byte, heading string) (int, int, error) {
	if heading == "<header>" {
		end := bytes.Index(document, []byte("\n## "))
		if end < 0 {
			return 0, 0, fmt.Errorf("first level-two heading missing")
		}
		return 0, end + 1, nil
	}
	needle := []byte(heading + "\n")
	start := bytes.Index(document, needle)
	if start < 0 || bytes.Index(document[start+len(needle):], needle) >= 0 {
		return 0, 0, fmt.Errorf("heading %q is missing or ambiguous", heading)
	}
	level := 0
	for level < len(heading) && heading[level] == '#' {
		level++
	}
	end := len(document)
	offset := start + len(needle)
	for offset < len(document) {
		nextEnd := bytes.IndexByte(document[offset:], '\n')
		if nextEnd < 0 {
			nextEnd = len(document) - offset
		}
		line := document[offset : offset+nextEnd]
		hashes := 0
		for hashes < len(line) && line[hashes] == '#' {
			hashes++
		}
		if hashes > 0 && hashes <= level && hashes < len(line) && line[hashes] == ' ' {
			end = offset
			break
		}
		offset += nextEnd + 1
	}
	return start, end, nil
}

func s7GitFileAt(t *testing.T, root, revision, rel string) []byte {
	t.Helper()
	command := exec.Command("git", "show", revision+":"+rel)
	command.Dir = root
	data, err := command.Output()
	if err != nil {
		t.Fatalf("git show %s:%s: %v", revision, rel, err)
	}
	return data
}

func s7ParseFullMatrix(document string) (
	map[string]s7Rev16MatrixRow,
	map[string]int,
	error,
) {
	headingPattern := regexp.MustCompile(`^### 18\.([0-9]+) ([A-Z]{1,2}) —`)
	rowPattern := regexp.MustCompile(`^\| (PIB-[0-9]{3}) \| ([ICGUS]) \|`)
	rows := map[string]s7Rev16MatrixRow{}
	categoryCounts := map[string]int{}
	category := ""
	for _, line := range strings.Split(document, "\n") {
		if match := headingPattern.FindStringSubmatch(line); match != nil {
			section, _ := strconv.Atoi(match[1])
			if section >= 2 && section <= 51 {
				category = match[2]
			} else {
				category = ""
			}
			continue
		}
		if strings.HasPrefix(line, "### 18.52 ") {
			category = ""
		}
		if category == "" {
			continue
		}
		match := rowPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if _, exists := rows[match[1]]; exists {
			return nil, nil, fmt.Errorf("duplicate matrix row %s", match[1])
		}
		rows[match[1]] = s7Rev16MatrixRow{
			id: match[1], kind: match[2], category: category, text: line,
		}
		categoryCounts[category]++
	}
	if len(categoryCounts) != 50 {
		return nil, nil, fmt.Errorf("matrix categories = %d, want 50", len(categoryCounts))
	}
	for index := 1; index <= 567; index++ {
		id := fmt.Sprintf("PIB-%03d", index)
		if rows[id].id != id {
			return nil, nil, fmt.Errorf("matrix is not contiguous at %s", id)
		}
	}
	return rows, categoryCounts, nil
}

func s7ValidateSection1852(
	document string,
	rows map[string]s7Rev16MatrixRow,
	categoryCounts map[string]int,
) error {
	start := strings.Index(document, "### 18.52 Counts, kinds and slice partition")
	end := strings.Index(document, "### 18.53 Sensitivity requirement")
	if start < 0 || end <= start {
		return fmt.Errorf("rev-16 §18.52 section missing")
	}
	section := document[start:end]
	categoryStart := strings.Index(section, "**50 categories**:")
	categoryEnd := strings.Index(section, "**50 categories; sum = 567.**")
	if categoryStart < 0 || categoryEnd <= categoryStart {
		return fmt.Errorf("rev-16 §18.52 category declaration missing")
	}
	declaredCategories := map[string]int{}
	categoryPattern := regexp.MustCompile(`\b([A-Z]{1,2}) ([0-9]+)\b`)
	for _, match := range categoryPattern.FindAllStringSubmatch(
		section[categoryStart:categoryEnd], -1,
	) {
		value, _ := strconv.Atoi(match[2])
		declaredCategories[match[1]] = value
	}
	if fmt.Sprint(declaredCategories) != fmt.Sprint(categoryCounts) {
		return fmt.Errorf("rev-16 §18.52 category counts=%v matrix=%v", declaredCategories, categoryCounts)
	}
	actualKinds := map[string]int{}
	for _, row := range rows {
		actualKinds[row.kind]++
	}
	declaredKinds := map[string]int{}
	kindPattern := regexp.MustCompile("`([ICGUS])` ([0-9]+)")
	for _, match := range kindPattern.FindAllStringSubmatch(section, -1) {
		if _, exists := declaredKinds[match[1]]; exists {
			continue
		}
		value, _ := strconv.Atoi(match[2])
		declaredKinds[match[1]] = value
	}
	if fmt.Sprint(declaredKinds) != fmt.Sprint(actualKinds) {
		return fmt.Errorf("rev-16 §18.52 kind counts=%v matrix=%v", declaredKinds, actualKinds)
	}
	slicePattern := regexp.MustCompile(`(?m)^\| (S[^|]+) \| ([A-Z, ]+) \| ([0-9]+) \|$`)
	assigned := map[string]string{}
	sliceTotal := 0
	slices := 0
	for _, match := range slicePattern.FindAllStringSubmatch(section, -1) {
		slices++
		declaredRows, _ := strconv.Atoi(match[3])
		computedRows := 0
		for _, category := range strings.Split(match[2], ",") {
			category = strings.TrimSpace(category)
			if category == "" {
				continue
			}
			if prior := assigned[category]; prior != "" {
				return fmt.Errorf("rev-16 §18.52 category %s assigned to %s and %s", category, prior, match[1])
			}
			assigned[category] = match[1]
			computedRows += categoryCounts[category]
		}
		if computedRows != declaredRows {
			return fmt.Errorf("rev-16 §18.52 slice %s rows=%d matrix=%d", match[1], declaredRows, computedRows)
		}
		sliceTotal += declaredRows
	}
	if slices != 9 || len(assigned) != 50 || sliceTotal != 567 {
		return fmt.Errorf("rev-16 §18.52 slice partition slices=%d categories=%d sum=%d", slices, len(assigned), sliceTotal)
	}
	return nil
}

func s7RevisionHistory(document []byte) ([]int, error) {
	pattern := regexp.MustCompile(`(?m)^\| rev-([0-9]+) \|`)
	var revisions []int
	seen := map[int]bool{}
	for _, match := range pattern.FindAllSubmatch(document, -1) {
		revision, err := strconv.Atoi(string(match[1]))
		if err != nil {
			return nil, err
		}
		if seen[revision] {
			return nil, fmt.Errorf("duplicate rev-%d history row", revision)
		}
		seen[revision] = true
		revisions = append(revisions, revision)
	}
	for index, revision := range revisions {
		if revision != index {
			return nil, fmt.Errorf("revision history order = %v", revisions)
		}
	}
	return revisions, nil
}

func s7RevisionRow(document string, revision int) string {
	prefix := fmt.Sprintf("| rev-%d |", revision)
	for _, line := range strings.Split(document, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

func s7CurrentNormativeText(document string) string {
	if start := strings.Index(document, "## 1. Problem statement"); start >= 0 {
		body := document[start:]
		ledgerStart := strings.Index(body, "### 18.1 How to read this matrix")
		matrixStart := strings.Index(body, "### 18.2 A —")
		if ledgerStart >= 0 && matrixStart > ledgerStart {
			body = body[:ledgerStart] + body[matrixStart:]
		}
		return strings.ToLower(body)
	}
	if start := strings.Index(document, "## Context"); start >= 0 {
		return strings.ToLower(document[start:])
	}
	return strings.ToLower(document)
}

func sortedS7BoolKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func validateS7Rev16PendingOwnerErratum(prd, adr []byte) error {
	prdText := string(prd)
	adrText := string(adr)
	prdRequired := []string{
		"rev-16 pending-owner\nno-decision erratum (2026-08-20)",
		"**Rev-16 acceptance**: 2026-08-20 — **Accepted errata**",
		"| rev-16 | **Accepted no-decision erratum — 2026-08-20** |",
		"amends exactly three stable rows: `PIB-402`, `PIB-403` and `PIB-425`",
		"matrix remains **567**",
		"§18.53 remains at **thirty-six** semantic guards",
		"global-hash rehydration of tombstoned references only, pending same-hash references routed exclusively to the purge owner",
	}
	for _, token := range prdRequired {
		if !strings.Contains(prdText, token) {
			return fmt.Errorf("PRD rev-16 token missing: %q", token)
		}
	}
	section93, err := s7MarkdownSection(prdText, "### 9.3 ", "### 9.4 ")
	if err != nil {
		return err
	}
	for _, token := range []string{
		"sets **every tombstoned reference",
		"with that content hash**",
		"changes every **tombstoned**\nreference to `h`",
		"If any same-hash reference\nis removal-pending, `h` is purge-owned",
		"`recovery-pending`",
		"rehydration never consumes or rewrites the pending\nreference",
	} {
		if !strings.Contains(section93, token) {
			return fmt.Errorf("PRD §9.3 rev-16 token missing: %q", token)
		}
	}
	if strings.Contains(section93, "tombstoned or removal-pending") {
		return fmt.Errorf("PRD §9.3 retains superseded pending-rehydration wording")
	}
	for _, row := range []struct {
		id     string
		tokens []string
	}{
		{"PIB-402", []string{"every tombstoned", "paired pending fixture", "recovery-pending", "writes zero bytes"}},
		{"PIB-403", []string{"every tombstoned", "terminally recovered by its purge owner", "never consumes it"}},
		{"PIB-425", []string{"every tombstoned h reference", "pending h reference is excluded", "entire hash to the purge owner"}},
	} {
		line := s7MarkdownTableRow(prdText, row.id)
		if line == "" {
			return fmt.Errorf("%s row missing", row.id)
		}
		for _, token := range row.tokens {
			if !strings.Contains(line, token) {
				return fmt.Errorf("%s rev-16 token missing: %q", row.id, token)
			}
		}
	}

	adrRequired := []string{
		"rev-16 pending-owner no-decision erratum (2026-08-20)",
		"**Rev-16 acceptance**: 2026-08-20 — **Accepted errata**",
		"| rev-16 | **Accepted no-decision erratum — 2026-08-20** |",
	}
	for _, token := range adrRequired {
		if !strings.Contains(adrText, token) {
			return fmt.Errorf("ADR rev-16 token missing: %q", token)
		}
	}
	for _, section := range []struct {
		start  string
		end    string
		tokens []string
	}{
		{
			start: "### D10 ", end: "### D11 ",
			tokens: []string{
				"makes every **tombstoned**\nreference",
				"Rehydration never consumes a removal-pending reference",
				"blocks\nmutating `prepare` with zero-write `recovery-pending`",
			},
		},
		{
			start: "### D13 ", end: "### D14 ",
			tokens: []string{
				"owner precedence also dominates rehydration",
				"pending same-hash\nreference is never un-tombstoned",
			},
		},
		{
			start: "### D16 ", end: "## ",
			tokens: []string{
				"Rehydration is not another recovery entry point",
				"changes tombstoned\nreferences only",
			},
		},
	} {
		body, sectionErr := s7MarkdownSection(adrText, section.start, section.end)
		if sectionErr != nil {
			return sectionErr
		}
		for _, token := range section.tokens {
			if !strings.Contains(body, token) {
				return fmt.Errorf("%s rev-16 token missing: %q", section.start, token)
			}
		}
		if strings.Contains(body, "tombstoned or removal-pending") {
			return fmt.Errorf("%s retains superseded pending-rehydration wording", section.start)
		}
	}
	return nil
}

// s7Rev19PurgePlanExample returns the §10.2 worked sequential-repair example —
// the fenced JSON block whose plan carries the rank-1 `corrupt-object`
// `manual-prerequisite` stage — so its coherence is judged over the example's
// own bytes rather than over prose about it.
func s7Rev19PurgePlanExample(prd string) (string, error) {
	section, err := s7MarkdownSection(prd, "### 10.2 JSON report (v1)", "### 10.3 ")
	if err != nil {
		return "", err
	}
	const fence = "```json\n"
	rest := section
	for {
		open := strings.Index(rest, fence)
		if open < 0 {
			return "", fmt.Errorf("PRD §10.2 shows no sequential-repair plan example")
		}
		rest = rest[open+len(fence):]
		closed := strings.Index(rest, "```")
		if closed < 0 {
			return "", fmt.Errorf("PRD §10.2 has an unterminated JSON fence")
		}
		block := rest[:closed]
		rest = rest[closed+3:]
		if strings.Contains(block, `"kind": "manual-prerequisite"`) {
			return block, nil
		}
	}
}

// validateS7Rev19RecordErratum is the rev-19 half of the frozen-document
// authority: the AX record erratum amends exactly `PIB-562` and `PIB-566` and
// corrects three non-matrix companions — §10.2's worked example, §16's `R25`
// and ADR-035's D10 prose — without moving a decision, an exit the product
// emits, or the 567/thirty-six arithmetic. It also owns the readiness record:
// rev-17, rev-18 and rev-19 are jointly accepted, PRD §19's authorization gate
// and ADR-035's `Blocks` metadata are satisfied without a release claim, the
// ADR index row says the same, and §12.6's D6 delta records the constant
// recovered-panic message.
func validateS7Rev19RecordErratum(prd, adr, index []byte) error {
	prdText := string(prd)
	adrText := string(adr)
	indexText := string(index)
	for _, token := range []string{
		"rev-19 observed-product surface-split\nno-decision erratum (2026-08-28)",
		"**Rev-19 acceptance**: 2026-08-29 — **Accepted no-decision erratum**, jointly\nwith rev-17 and rev-18 (raised 2026-08-28)",
		"| rev-19 | **Accepted no-decision erratum — raised 2026-08-28, accepted 2026-08-29** |",
		"amends exactly two stable rows: `PIB-562` and `PIB-566`",
		"**companions, not matrix rows**",
	} {
		if !strings.Contains(prdText, token) {
			return fmt.Errorf("PRD rev-19 token missing: %q", token)
		}
	}
	for _, token := range []string{
		"rev-19 D10\nobserved-product no-decision erratum (2026-08-28)",
		"**Rev-19 acceptance**: 2026-08-29 — **Accepted no-decision erratum**, jointly\nwith rev-17 and rev-18 (raised 2026-08-28)",
		"| rev-19 | **Accepted no-decision erratum — raised 2026-08-28, accepted 2026-08-29** |",
		"rev-17, rev-18 and rev-19 no-decision errata were\naccepted in the same joint review as this document's, on 2026-08-29",
	} {
		if !strings.Contains(adrText, token) {
			return fmt.Errorf("ADR rev-19 token missing: %q", token)
		}
	}
	if err := validateS7ReadinessRecord(prdText, adrText, indexText); err != nil {
		return err
	}

	// The two amended matrix rows, each judged on its own line.
	for _, row := range []struct {
		id      string
		tokens  []string
		retired []string
	}{
		{
			id: "PIB-562",
			tokens: []string{
				"`list` renders the corrupt object and both residues in one pass and exits **3**",
				"`doctor`'s D9 renders that identical observation set as **warning** findings and exits **0**",
				"Neither ever names `--orphans --yes` as the corrupt object's repair",
			},
			retired: []string{"in one pass, exit 3"},
		},
		{
			id: "PIB-566",
			tokens: []string{
				"§12.5's `doctor` D9 pending-purge remediation included",
				"asserted on the **structured retry carriers**",
				"has no `retry`/`retry_cwd` pair of its own",
				"no inherited `--path`",
				"**none** of them names `--all --yes` in any form",
				"identical terminal recovery as `--all --yes` would",
				"Three semantic sensitivity fixtures",
			},
			retired: []string{
				"covering exactly the pending set, with `retry_cwd: \"workspace-root\"` and no inherited `--path`",
			},
		},
	} {
		line := s7MarkdownTableRow(prdText, row.id)
		if line == "" {
			return fmt.Errorf("%s row missing", row.id)
		}
		for _, token := range row.tokens {
			if !strings.Contains(line, token) {
				return fmt.Errorf("%s rev-19 token missing: %q", row.id, token)
			}
		}
		for _, token := range row.retired {
			if strings.Contains(line, token) {
				return fmt.Errorf("%s retains the retired rev-13 reading %q", row.id, token)
			}
		}
	}

	// §18.53 keeps three fixtures for PIB-566, with the omitted-`retry_cwd` one
	// scoped to a carrier that has such a field at all.
	fixtureRow := s7MarkdownTableRow(prdText, "PIB-566 pending-route narrowing")
	if fixtureRow == "" {
		return fmt.Errorf("§18.53 PIB-566 fixture entry missing")
	}
	if !strings.Contains(fixtureRow,
		"names the right hashes on a structured retry carrier but omits that carrier's `retry_cwd`") {
		return fmt.Errorf("§18.53 PIB-566 fixture entry is not carrier-scoped: %s", fixtureRow)
	}
	if strings.Count(fixtureRow, ";") != 2 {
		return fmt.Errorf("§18.53 PIB-566 fixture entry no longer lists three fixtures: %s", fixtureRow)
	}

	// §10.2's worked example, judged over its own bytes: an archive-integrity
	// refusal plan that still carries the rank-1 manual prerequisite, and
	// therefore carries no `repaired_class`.
	example, err := s7Rev19PurgePlanExample(prdText)
	if err != nil {
		return err
	}
	for _, token := range []string{
		`"outcome": "refused"`,
		`"action": "none"`,
		`"class": "corrupt-object"`,
		`"kind": "manual-prerequisite"`,
	} {
		if !strings.Contains(example, token) {
			return fmt.Errorf("PRD §10.2 worked example lacks %q:\n%s", token, example)
		}
	}
	if strings.Contains(example, "repaired_class") {
		return fmt.Errorf(
			"PRD §10.2 worked example carries repaired_class beside a manual-prerequisite stage:\n%s",
			example,
		)
	}
	if !strings.Contains(prdText,
		"**The worked example below is that exit-3 archive-integrity refusal**") {
		return fmt.Errorf("PRD §10.2 does not label its worked example as the exit-3 refusal form")
	}
	if !strings.Contains(prdText,
		"can never appear beside a `manual-prerequisite` stage") {
		return fmt.Errorf("PRD §10.2 does not state the repaired_class/manual-prerequisite exclusion")
	}

	// §16's R25, split by surface the same way PIB-562 is.
	risk := s7MarkdownTableRow(prdText, "R25")
	if risk == "" {
		return fmt.Errorf("R25 row missing")
	}
	for _, token := range []string{
		"`list` and every ordinary mutation refuse it at exit **3**",
		"`doctor`'s D9 renders the identical observation as a **warning** finding and exits **0**",
	} {
		if !strings.Contains(risk, token) {
			return fmt.Errorf("R25 rev-19 token missing: %q", token)
		}
	}

	// ADR-035's paired D10 prose, and D16's already-correct warning-only rule
	// left exactly where it was.
	d10, err := s7MarkdownSection(adrText, "### D10 ", "### D11 ")
	if err != nil {
		return err
	}
	for _, token := range []string{
		"`list` and\nevery ordinary mutation refuse it zero-write at exit **3**",
		"`doctor`'s D9 renders the identical\nobservation as a **warning** finding and exits **0**",
		"rank-1 blocking repair class (PIB-561)",
	} {
		if !strings.Contains(d10, token) {
			return fmt.Errorf("ADR D10 rev-19 token missing: %q", token)
		}
	}
	if strings.Contains(d10, "retry_cwd") {
		return fmt.Errorf("ADR D10 gained a retry_cwd claim rev-19 does not make")
	}
	d16, err := s7MarkdownSection(adrText, "### D16 ", "### D17 ")
	if err != nil {
		return err
	}
	if !strings.Contains(d16, "`doctor`'s D9 reports the identical set, warning-only (PIB-541)") {
		return fmt.Errorf("ADR D16's warning-only rule moved; rev-19 changes no D16 text")
	}
	return nil
}

// validateS7ReadinessRecord pins the pre-release record the three frozen
// documents carry once the joint rev-17/18/19 acceptance landed: PRD §19's
// authorization gate is closed on all four conditions with the GH #23
// implementation accepted through aggregate review, ADR-035's `Blocks`
// metadata says the same over slices S1–S7, the ADR index row agrees, and
// none of the three claims a release or a tag. §12.6's D6 delta additionally
// records that the recovered-panic finding message is a constant for every
// doctor check, so D1–D8 carry no interpolated panic value either.
func validateS7ReadinessRecord(prd, adr, index string) error {
	gate, err := s7MarkdownSection(prd, "## 19. Implementation authorization gate", "\n## 20. ")
	if err != nil {
		return err
	}
	for _, token := range []string{
		"**The gate is satisfied and closed.**",
		"Conditions (1)–(4) are complete",
		"has been accepted through aggregate review",
		"Release\nauthorization is a separate decision",
		"production slices S1–S7 landed and were accepted through aggregate\n   review",
	} {
		if !strings.Contains(gate, token) {
			return fmt.Errorf("PRD §19 readiness token missing: %q", token)
		}
	}
	for _, retired := range []string{
		"Conditions (1)–(3) are complete",
		"mutating production slices remain paused",
		"production awaits rev-15 joint approval",
		"only its pre-change test/golden commit is",
		"tagged",
		"released",
	} {
		if strings.Contains(gate, retired) {
			return fmt.Errorf("PRD §19 carries the wrong authorization reading %q", retired)
		}
	}

	deltas, err := s7MarkdownSection(prd, "### 12.6 Enumerated behavior deltas", "\n## 13. ")
	if err != nil {
		return err
	}
	for _, token := range []string{
		"The recovered-panic finding message is a constant for **every** doctor check",
		"so D1–D8 no longer interpolate the panic value either",
		"no D1–D8 golden fixture drives a panicking check",
	} {
		if !strings.Contains(deltas, token) {
			return fmt.Errorf("PRD §12.6 D6 panic-record token missing: %q", token)
		}
	}
	for _, retired := range []string{
		"includes the panic value",
		"interpolates the panic value",
		"carries the panic value",
	} {
		if strings.Contains(deltas, retired) {
			return fmt.Errorf("PRD §12.6 D6 promises an interpolated panic detail: %q", retired)
		}
	}

	blocks, err := s7MarkdownSection(adr, "**Blocks**:", "\n\n**Revision history**")
	if err != nil {
		return err
	}
	for _, token := range []string{
		"slices S1–S7",
		"implementation authorization gate (PRD §19) is satisfied",
		"accepted through aggregate review",
		"Release authorization remains a separate decision.",
	} {
		if !strings.Contains(blocks, token) {
			return fmt.Errorf("ADR Blocks readiness token missing: %q", token)
		}
	}
	for _, retired := range []string{
		"slices S1–S6",
		"mutating production slices wait for rev-15",
		"may\nland pre-change evidence",
	} {
		if strings.Contains(blocks, retired) {
			return fmt.Errorf("ADR Blocks carries the wrong authorization reading %q", retired)
		}
	}

	row := ""
	for _, line := range strings.Split(index, "\n") {
		if strings.HasPrefix(line, "- [ADR-035:") {
			row = line
			break
		}
	}
	if row == "" {
		return fmt.Errorf("ADR index has no ADR-035 row")
	}
	for _, token := range []string{
		"no-decision errata through rev-19 accepted jointly on 2026-08-29",
		"rev-20's bounded selector diagnostic amendment accepted on 2026-08-30",
		"rev-20. Decisions D1–D21.",
		"and its acceptance are complete; release authorization is pending.",
	} {
		if !strings.Contains(row, token) {
			return fmt.Errorf("ADR index row readiness token missing: %q: %s", token, row)
		}
	}
	for _, retired := range []string{
		"Mutating implementation is blocked by",
		"not by planning review",
		"rev-14. Decisions",
	} {
		if strings.Contains(row, retired) {
			return fmt.Errorf("ADR index row carries the wrong readiness reading %q: %s", retired, row)
		}
	}
	return nil
}

func s7MarkdownSection(document, start, end string) (string, error) {
	startAt := strings.Index(document, start)
	if startAt < 0 {
		return "", fmt.Errorf("markdown section %q missing", start)
	}
	rest := document[startAt+len(start):]
	endAt := strings.Index(rest, end)
	if endAt < 0 {
		return "", fmt.Errorf("markdown section terminator %q missing after %q", end, start)
	}
	return rest[:endAt], nil
}

func s7MarkdownTableRow(document, id string) string {
	prefix := "| " + id + " |"
	for _, line := range strings.Split(document, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

func s7PrepareInitialBundle(t *testing.T, root, slug string) {
	t.Helper()
	code, _, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--allow-heuristic", "--json", "--quiet",
	)
	if code != 0 {
		t.Fatalf("initial prepare: exit=%d stderr=%q", code, stderr)
	}
}

func s7WriteControlledIntentBundle(
	t *testing.T,
	root, slug string,
) map[store.IntentArchiveArtifactID][]byte {
	t.Helper()
	feature := filepath.Join(root, ".tpatch", "features", slug)
	controlled := map[store.IntentArchiveArtifactID][]byte{
		store.IntentArchiveArtifactAnalysis:    []byte("shared prior intent\n"),
		store.IntentArchiveArtifactSpec:        []byte("shared prior intent\n"),
		store.IntentArchiveArtifactExploration: []byte("distinct prior exploration\n"),
	}
	for id, body := range controlled {
		rel, err := store.IntentArchiveArtifactPath(id)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(feature, filepath.FromSlash(rel)), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sidecarRel, err := store.IntentArchiveArtifactPath(store.IntentArchiveArtifactAnalysisSidecar)
	if err != nil {
		t.Fatal(err)
	}
	sidecar, err := os.ReadFile(filepath.Join(feature, filepath.FromSlash(sidecarRel)))
	if err != nil {
		t.Fatal(err)
	}
	controlled[store.IntentArchiveArtifactAnalysisSidecar] = sidecar
	return controlled
}

func s7IntentArchiveReplacements(
	t *testing.T,
	bodies map[store.IntentArchiveArtifactID][]byte,
	state store.IntentArchiveWireState,
) []store.IntentArchiveReplacement {
	t.Helper()
	order := []store.IntentArchiveArtifactID{
		store.IntentArchiveArtifactAnalysis,
		store.IntentArchiveArtifactAnalysisSidecar,
		store.IntentArchiveArtifactExploration,
		store.IntentArchiveArtifactSpec,
	}
	replacements := make([]store.IntentArchiveReplacement, 0, len(order))
	for _, id := range order {
		replacements = append(replacements, intentArchiveCLIReplacement(t, id, bodies[id], state))
	}
	return replacements
}

func s7GenerationIDs(index store.IntentArchiveIndex) []string {
	ids := make([]string, 0, len(index.Generations))
	for _, generation := range index.Generations {
		ids = append(ids, generation.GenerationID)
	}
	return ids
}

// validateS7Rev20SelectorAmendment pins rev-20, which is deliberately not
// shaped like rev-16…rev-19. Those were no-decision errata; rev-20 changes the
// closed public code an invalid purge selector emits, so its disposition says
// "amendment", the §10.4.1 catalog grows from 53 to 54 codes, and the two rows
// it amends carry the new obligations. Everything the amendment must *not* do —
// reclassify strict-index failures, echo a rejected raw value, claim
// corruption, offer preservation or an `rm` form — is asserted as an absence.
// Since 2026-08-30 the disposition is also terminal: both documents record the
// **accepted** amendment, name the external finding and the three internal
// correction reviews behind it, and carry no pre-acceptance proposed/pending
// metadata anywhere. The accepted semantics are the reviewed ones — the same
// `PIB-431`/`PIB-465` pair, 567 rows, thirty-six semantic guards.
func validateS7Rev20SelectorAmendment(prd, adr []byte) error {
	prdText := string(prd)
	adrText := string(adr)
	for _, token := range []string{
		"rev-20 selector-classification bounded public\ndiagnostic-vocabulary amendment (accepted 2026-08-30) — not a no-decision\nerratum",
		"**Rev-20 acceptance**: **Accepted bounded public diagnostic-vocabulary\namendment — 2026-08-30**",
		"| rev-20 | **Accepted bounded public diagnostic-vocabulary amendment — 2026-08-30** |",
		"amends exactly two stable rows: `PIB-431` and `PIB-465`",
		"**A selector that names nothing is a selector fault, not an archive fault.**",
		// The acceptance record: one external finding, three internal
		// correction reviews, and semantics unchanged by acceptance.
		"accepted after the post-close external finding that\nraised it and **three** internal correction reviews of the drafted amendment,\nwith the semantics that were reviewed unchanged",
		"it was **accepted** on 2026-08-30, after the post-close external\nfinding that raised it and **three** internal correction reviews of the drafted\namendment, with the reviewed semantics unchanged by acceptance",
		"(**Accepted 2026-08-14**, rev-20;",
	} {
		if !strings.Contains(prdText, token) {
			return fmt.Errorf("PRD rev-20 token missing: %q", token)
		}
	}
	for _, token := range []string{
		"rev-20 D16 selector-classification bounded\npublic diagnostic-vocabulary amendment (accepted 2026-08-30) — not a\nno-decision erratum",
		"**Rev-20 acceptance**: **Accepted bounded public diagnostic-vocabulary\namendment — 2026-08-30**",
		"| rev-20 | **Accepted bounded public diagnostic-vocabulary amendment — 2026-08-30** |",
		"accepted with the companion PRD's rev-20 after the\npost-close external finding that raised it and **three** internal correction\nreviews of the drafted amendment, with the semantics that were reviewed\nunchanged",
		"no D1–D21 change outside D16's\ndiagnostic vocabulary",
		"(rev-20 Accepted;",
	} {
		if !strings.Contains(adrText, token) {
			return fmt.Errorf("ADR rev-20 token missing: %q", token)
		}
	}

	// The accepted disposition is terminal in both documents: no current
	// metadata may still record rev-20 as proposed or as awaiting review.
	for label, document := range map[string]string{"PRD": prdText, "ADR": adrText} {
		for _, stale := range []string{
			"**Rev-20 disposition**",
			"acceptance pending review",
			"amendment (proposed 2026-08-30)",
			"| rev-20 | **Proposed",
		} {
			if strings.Contains(document, stale) {
				return fmt.Errorf("%s rev-20 still records the pre-acceptance disposition %q", label, stale)
			}
		}
	}

	// rev-20 is an amendment, not an erratum: neither document may record it
	// with the accepted-no-decision disposition its predecessors carry, and the
	// PRD's own ledger claim must be the two rows the diff really changes.
	prdRow := s7RevisionRow(prdText, 20)
	if !strings.Contains(prdRow, "**Amended rows, exactly**: `PIB-431` and `PIB-465` — **two**") {
		return fmt.Errorf("PRD rev-20 does not claim exactly two amended rows: %s", prdRow)
	}
	for label, row := range map[string]string{
		"PRD": prdRow,
		"ADR": s7RevisionRow(adrText, 20),
	} {
		if row == "" {
			return fmt.Errorf("%s rev-20 revision row missing", label)
		}
		if strings.Contains(row, "Accepted no-decision erratum") {
			return fmt.Errorf("%s rev-20 is recorded as an accepted no-decision erratum: %s", label, row)
		}
		if !strings.Contains(row,
			"**Accepted bounded public diagnostic-vocabulary amendment — 2026-08-30**") {
			return fmt.Errorf("%s rev-20 is not recorded as the accepted amendment: %s", label, row)
		}
		if !strings.Contains(row, "**three** internal correction reviews") {
			return fmt.Errorf("%s rev-20 does not record the three internal correction reviews: %s", label, row)
		}
		if !strings.Contains(row, "`PIB-431`") || !strings.Contains(row, "`PIB-465`") {
			return fmt.Errorf("%s rev-20 does not name both amended rows: %s", label, row)
		}
	}

	// §10.4.1 carries the code as its own exit-3 row, and the catalog is 54.
	catalog, err := s7Rev20RefusalCatalog(prdText)
	if err != nil {
		return err
	}
	if len(catalog) != 54 {
		return fmt.Errorf("§10.4.1 lists %d refusal codes, want 54", len(catalog))
	}
	selectorRows := 0
	for _, code := range catalog {
		if code == "archive-selector-invalid" {
			selectorRows++
		}
	}
	if selectorRows != 1 {
		return fmt.Errorf("§10.4.1 lists archive-selector-invalid %d times", selectorRows)
	}
	catalogRow := s7MarkdownTableRow(prdText, "`archive-selector-invalid`")
	if catalogRow == "" {
		return errors.New("§10.4.1 has no archive-selector-invalid row of its own")
	}
	for _, token := range []string{
		"| `archive-selector-invalid` | 3 |",
		"The code has exactly two emitted populations",
		"reject at exit **1** before any archive read",
		"tpatch feature intent-archive list <slug>",
		"a rejected malformed value is never echoed",
		"`archive-index-*` codes remain reserved for X1–X10 strict decoding and X11 storage",
		"none of them is a report population",
	} {
		if !strings.Contains(catalogRow, token) {
			return fmt.Errorf("§10.4.1 archive-selector-invalid row lacks %q", token)
		}
	}
	for _, forbidden := range []string{"preserve the archive", "rm -rf", "corrupt archive"} {
		if strings.Contains(strings.ToLower(catalogRow), forbidden) {
			return fmt.Errorf("§10.4.1 archive-selector-invalid row carries %q", forbidden)
		}
	}

	// §6.4's dry-run partition places it in the not-evaluated column only.
	partition, err := s7MarkdownSection(prdText,
		"**What dry-run reproduces, and what it deliberately does not evaluate.**",
		"The redaction scan is deliberately")
	if err != nil {
		return err
	}
	evaluated, notEvaluated := 0, 0
	for _, line := range strings.Split(partition, "\n") {
		if !strings.HasPrefix(line, "|") || strings.Contains(line, "---") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 4 {
			continue
		}
		if strings.Contains(cells[1], "`archive-selector-invalid`") {
			evaluated++
		}
		if strings.Contains(cells[2], "`archive-selector-invalid`") {
			notEvaluated++
		}
	}
	if evaluated != 0 || notEvaluated != 1 {
		return fmt.Errorf(
			"§6.4 places archive-selector-invalid evaluated=%d not-evaluated=%d, want 0/1",
			evaluated, notEvaluated,
		)
	}

	// §9.7.2's preflight selector row, and the retired readings it replaces.
	preflight := s7MarkdownTableRow(prdText,
		"selector present, exactly one, and well-formed (`^[0-9a-f]{64}$` hashes, known generation ids)")
	if preflight == "" {
		return errors.New("§9.7.2 selector preflight row missing")
	}
	for _, token := range []string{
		"exit 1 (no selector, a repeated scope family, or a value that is not a full lowercase SHA-256",
		"with an empty report, no refusal code and no echo of the rejected value",
		"exit 3 (`archive-selector-invalid` for a well-formed `--blob` hash the index does not carry",
		"the index does not record",
		"No `archive-index-*` code is reachable from it",
		"is never a report population",
	} {
		if !strings.Contains(preflight, token) {
			return fmt.Errorf("§9.7.2 selector preflight row lacks %q", token)
		}
	}
	if strings.Contains(prdText, "`archive-index-corrupt` for an unknown id") {
		return errors.New("PRD retains the retired unknown-id `archive-index-corrupt` reading")
	}
	// The correction rev-20 itself needed: a malformed value is an exit-1 usage
	// error, never a structured exit-3 refusal, so the PRD may not promise one.
	for _, overreach := range []string{
		"`archive-selector-invalid` for a malformed id",
		"malformed or unknown ids keep **exit 3**",
		"malformed or unknown ids\nexit 3",
	} {
		if strings.Contains(prdText, overreach) {
			return fmt.Errorf("PRD promises a public refusal code for a malformed selector: %q", overreach)
		}
	}

	// The two amended matrix rows.
	for _, row := range []struct {
		id      string
		tokens  []string
		retired []string
	}{
		{
			id: "PIB-431",
			tokens: []string{
				"**the typed archive refusal renderer surfaces an already-public catalog code unchanged**",
				"`intentArchiveRefusalFromError` may not relabel one catalog code as another",
				"`archive-selector-invalid` reaches the report as itself and never as an `archive-index-*` code",
				"an internal, non-catalog transport failure such as `archive-storage-failed` is legitimately classified into a catalog code by its owning boundary",
				"which is what `prepareStoreArchiveFailure` does",
				"identical in human and JSON",
			},
			retired: []string{
				"purge/publication index codes stay distinct |",
				"no emitter rewrites a classified code",
			},
		},
		{
			id: "PIB-465",
			tokens: []string{
				"each refuse `archive-selector-invalid` — never an `archive-index-*` code",
				"over an absent/empty archive as well as a populated healthy one",
				"Its exit-1 half is asserted separately and as a **non**-report population",
				"preview takes no authority, while confirmed takes and releases exactly one workspace authority before normalization",
				"Workspace, platform, pending-journal and confirmed contention refusals retain their higher precedence",
				"The strict-decode row keeps its `archive-index-*` classification unchanged",
			},
		},
	} {
		line := s7MarkdownTableRow(prdText, row.id)
		if line == "" {
			return fmt.Errorf("%s row missing", row.id)
		}
		flat := strings.Join(strings.Fields(line), " ")
		for _, token := range row.tokens {
			if !strings.Contains(flat, strings.Join(strings.Fields(token), " ")) {
				return fmt.Errorf("%s rev-20 token missing: %q", row.id, token)
			}
		}
		for _, token := range row.retired {
			if strings.Contains(line, token) {
				return fmt.Errorf("%s retains the retired pre-rev-20 reading %q", row.id, token)
			}
		}
	}

	// PIB-347 keeps its exit-1 grammar row: rev-20 amends exactly two rows, and
	// the missing/multiple-selector population is not one of them.
	grammar := s7MarkdownTableRow(prdText, "PIB-347")
	if !strings.Contains(grammar,
		"exit 1; the message names `--blob`, `--generation`, `--orphans`, `--all`; zero writes") {
		return fmt.Errorf("PIB-347's exit-1 selector grammar row changed: %s", grammar)
	}

	// §16's R38 states the misclassification risk this amendment closes.
	risk := s7MarkdownTableRow(prdText, "R38")
	if risk == "" {
		return errors.New("R38 row missing")
	}
	for _, token := range []string{
		"`archive-selector-invalid`",
		"feature intent-archive list <slug>",
		"none prints an `rm` form",
	} {
		if !strings.Contains(risk, token) {
			return fmt.Errorf("R38 rev-20 token missing: %q", token)
		}
	}

	// ADR-035 D16 states the same vocabulary, and adds no retry carrier.
	d16, err := s7MarkdownSection(adrText, "### D16 ", "### D17 ")
	if err != nil {
		return err
	}
	for _, token := range []string{
		"**The selector check has its own public code, and it is not an archive\naccusation.**",
		"**the typed archive refusal\nrenderer surfaces it unchanged**",
		"an internal, non-catalog transport failure is still\nclassified into a catalog code by the boundary that owns it",
		"`archive-index-*` stays reserved for X1–X10 strict decoding\nand X11 storage observation and is unreachable from a selector fault",
		"missing/multiple-selector grammar keeps exit\n**1**",
		"is not a full\nlowercase SHA-256",
	} {
		if !strings.Contains(d16, token) {
			return fmt.Errorf("ADR D16 rev-20 token missing: %q", token)
		}
	}
	if strings.Contains(d16, "**no\nemitter rewrites it into another catalog code**") {
		return errors.New("ADR D16 retains the unscoped no-rewrite claim rev-20 had to narrow")
	}
	// The selector paragraph itself names no structured retry carrier: D16's
	// other paragraphs legitimately do, so the scan is scoped to this one.
	selectorParagraph, err := s7MarkdownSection(d16,
		"**The selector check has its own public code",
		"`archive-index-*` stays reserved")
	if err != nil {
		return err
	}
	for _, forbidden := range []string{"retry_cwd", "rm -rf"} {
		if strings.Contains(selectorParagraph, forbidden) {
			return fmt.Errorf("ADR D16's selector paragraph carries %q", forbidden)
		}
	}
	for _, token := range []string{
		"names no preservation, correction or `rm` form",
		"carries no\nstructured retry",
	} {
		if !strings.Contains(selectorParagraph, token) {
			return fmt.Errorf("ADR D16's selector paragraph lacks %q", token)
		}
	}
	return nil
}

// s7Rev20RefusalCatalog reads §10.4.1's first-cell codes in document order,
// exactly as the shipped S6 catalog reader does.
func s7Rev20RefusalCatalog(document string) ([]string, error) {
	section, err := s7MarkdownSection(document,
		"### 10.4.1 Closed refusal catalog", "### 10.5 Precedence")
	if err != nil {
		return nil, err
	}
	pattern := regexp.MustCompile("`([a-z][a-z0-9-]+)`")
	seen := map[string]bool{}
	var codes []string
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		firstCell := strings.SplitN(strings.TrimPrefix(line, "|"), "|", 2)[0]
		for _, match := range pattern.FindAllStringSubmatch(firstCell, -1) {
			if !seen[match[1]] {
				seen[match[1]] = true
				codes = append(codes, match[1])
			}
		}
	}
	if len(codes) == 0 {
		return nil, errors.New("parsed no refusal codes from §10.4.1")
	}
	return codes, nil
}
