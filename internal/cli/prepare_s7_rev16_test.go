//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"crypto/sha256"
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
			name: "unrelated-prd-normative-edit",
			mutate: func(wrong s7Rev16Evidence) s7Rev16Evidence {
				wrong.prd = bytes.Replace(wrong.prd,
					[]byte("### 5.1 Authorized grammar (v1, complete)"),
					[]byte("### 5.1 Authorized grammar (v1, complete; unrelated rev-16 edit)"), 1)
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
	const base = "2d9492cbf6fd9c69c5aa75d64d05983c05e1563f"
	basePRD := s7GitFileAt(t, root, base, "docs/prds/PRD-prepare-intent-bundle.md")
	baseADR := s7GitFileAt(t, root, base, "docs/adrs/ADR-035-intent-bundle-publication-and-history.md")
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
		base: base, prd: prd, adr: adr, basePRD: basePRD, baseADR: baseADR, diff: diff,
	}
}

type s7Rev16Evidence struct {
	base             string
	prd, adr         []byte
	basePRD, baseADR []byte
	diff             []byte
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
		"PIB-542", "PIB-543", "PIB-544",
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
		if len(revisions) < 2 || revisions[len(revisions)-2] != 16 || revisions[len(revisions)-1] != 17 {
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
				{label: "status-and-acceptance-header", heading: "<header>", baseHash: "a3aa799f8ab92acf0d16bcafe716977ad9b2d8279c84670a1736595e3d523f2a", currentHash: "7450fbeef810da38de9223490da93bc9abf48bcefc677d47949c6d0516212cb6"},
				{label: "revision-history", heading: "## Revision history", baseHash: "d9b6aff94362d6bdc8cbe89eec06f514d5d3eccae4175673a84c5771a669067c", currentHash: "17909fb698b8d56d555558b03c3a62a9f7cb225f696702c63c5871f54b42b103"},
				{label: "section-9.3-rehydration", heading: "### 9.3 `index.json`", baseHash: "fd9f4f35b499a2b17a60c6d256b9c1ff2448b57ef750e9f1f367505406e5ecc9", currentHash: "e8298d7394da53eee093bfc0b3b1e7a61bea0c8579fea731922b63f6a5e98254"},
				{label: "section-9.7.1-consistency", heading: "#### 9.7.1 Selection and shared references", baseHash: "3210b5cf279b85e0f53e3d9edaa3caae1677849392155d06690455ae59f3287c", currentHash: "03b78d83c11bc56599cbb8e71cdb4f11f9506ed3c13922215e82a252ff0b1d93"},
				{label: "mechanical-slice-summary", heading: "### 17.2 Slices", baseHash: "32831b33b9200267975945357a3f035ad8ac84d8c91720496007986fcdea8f2a", currentHash: "d18b0764ee3c2985016ba5efe2c6aca5d1c32765ee9844e32a1dacdfd82aca05"},
				{label: "section-18.1-amendment-ledger", heading: "### 18.1 How to read this matrix", baseHash: "2d07413506629fc420e5e2768bb446633a2579195a60189820cc6f16187b91cd", currentHash: "11aa208cf76d4800615c11e154416ee3f2f3a93bbf234a5c78c7a7a4399616d3"},
				{label: "matrix-pib-402-403", heading: "### 18.40 AM — Rev-2 adjudication rows, amended by rev-3", baseHash: "ea342198fd9ecfe95385245fb29af8f89dab05332ea7d63947a488907637a2b6", currentHash: "307e6c60f1b23cd64ca254aaef2e49135eb0c0cf33475e8fc887c9b5b7def16a"},
				{label: "matrix-pib-425", heading: "### 18.41 AN — Rev-3 adjudication: directory authority, privacy and archive truth", baseHash: "c3812ced4e0254b749cc1b9e24ecf8b284223678619de5966aa16f25e50ed3a0", currentHash: "5edc1c5c1ccac9f836099faee0c2c329da858b7cab357b96df9bd50fae3efc5d"},
				{label: "matrix-pib-542-544", heading: "### 18.48 AU — Rev-10 adjudication: global pending ownership, selector-independent validation and the corrupt-blob route", baseHash: "7b8dc7b2b98655beef1f5f03706cc832d21a48699418d24ffc13396d8d237d01", currentHash: "0c5a59211d322812d6c14dd37cd99b2ffc78fc60d2df43602bc36435fab37098"},
			},
		},
		{
			label: "ADR",
			base:  input.baseADR, current: input.adr,
			allowedRegions: []s7Rev16AllowedRegion{
				{label: "status-revision-amendment-ledgers", heading: "<header>", baseHash: "09f50199314281aa28d17c63bbb25519738c4ffcefb3c2a2c2d960380a7e1ab6", currentHash: "79af540d36b91ba1c3921b92c301c34974fc04b82219c9c37293ceeb4683176a"},
				{label: "D10-pending-owner", heading: "### D10 — Content-addressed, deterministic identifiers; no wall-clock in tracked bytes", baseHash: "04b8affc6125b69e4191b2b30d6cb8c9760e0ab872abf9aa520aa53040a4b4e5", currentHash: "c9b97c41e7e1127dad58fe227aa06e3f62da053a468661d3333fa255db819380"},
				{label: "D13-terminal-owner-precedence", heading: "### D13 — Recovery has three entry points, it is terminal, the operator's runs *instead of* the automatic ones, and the diagnostic touches nothing", baseHash: "77633643c1cc2c1b5607ba673cfe8af6b17d25d47510e1c6904b0a0f50c6cbb1", currentHash: "b5b45839dc04492a0673aa44b232d7138fff88cc7b8657072feb26c0c01a454f"},
				{label: "D16-purge-owner", heading: "### D16 — Retention is bounded: listing, purging, tombstones and orphans", baseHash: "ee66479c3eae9c1ed0af5de192d63e088d00ddc3843aaf9c8666151909e2ec41", currentHash: "e2131eeb1727beca9783d7d81f428e765bf6b8f2ccb15814c10a4e4510111e21"},
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
