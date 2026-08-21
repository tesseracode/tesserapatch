package store

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestS7PIB430ExternalReplacementAfterRevalidationIsDisclosedResidual(t *testing.T) {
	const feature = "demo"
	replacement := archiveReplacement(
		t, IntentArchiveArtifactAnalysis, "residual", IntentArchiveWireRetained,
	)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, replacement),
	))
	blobRel := storage.putRegular(feature, replacement.ContentSHA256, []byte("residual"))
	plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Blobs: []string{replacement.ContentSHA256},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	storage.hooks["after-remove-revalidate"] = func(memory *archiveMemoryStorage) {
		delete(memory.hooks, "after-remove-revalidate")
		memory.blobs[blobRel] = archiveMemoryBlob{
			kind:    IntentArchiveBlobKindNonRegular,
			version: 99,
		}
	}
	result, err := ExecuteIntentArchivePurge(storage, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !result.RemovalRaceResidualDisclosed {
		t.Fatal("revalidate-to-unlink residual was not represented")
	}
	if _, exists := storage.blobs[blobRel]; exists {
		t.Fatal("replacement in disclosed unlink window survived")
	}
}

type s7PIB447Evidence struct {
	claimBeforeRemove       bool
	tombstoneAfterRemove    bool
	everyReferenceClaimed   bool
	everyReferenceTombstone bool
	danglingCode            IntentArchiveErrorCode
	danglingRepairNoRemove  bool
	mixedClass              IntentArchiveRepairClass
	unreferencedClass       IntentArchiveRepairClass
	corruptClass            IntentArchiveRepairClass
	ownedCorruptCode        IntentArchiveErrorCode
	ownedCorruptExit        int
	selectorIndependentCode IntentArchiveErrorCode
	partialCode             IntentArchiveErrorCode
	partialExit             int
}

func TestS7PIB447ArchiveStateMachineSemanticParityAndWrongInput(t *testing.T) {
	evidence := deriveS7PIB447Evidence(t)
	sources := map[string]string{
		"internal/cli/feature_intent_archive.go": s7StoreRepositoryFile(t, "internal/cli/feature_intent_archive.go"),
		"internal/cli/prepare_publish.go":        s7StoreRepositoryFile(t, "internal/cli/prepare_publish.go"),
		"internal/store/intent_archive.go":       s7StoreRepositoryFile(t, "internal/store/intent_archive.go"),
	}
	prd := s7StoreRepositoryFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
	adr := s7StoreRepositoryFile(t, "docs/adrs/ADR-035-intent-bundle-publication-and-history.md")
	t.Run("baseline", func(t *testing.T) {
		if err := validateS7PIB447Parity(evidence, sources, prd, adr); err != nil {
			t.Fatal(err)
		}
	})
	fixtures := []struct {
		name   string
		source string
		prd    string
		adr    string
	}{
		{
			name: "prepare-recovers-pending",
			source: `
func pib447Wrong(storage store.IntentArchiveStorage, slug string) {
	_, _ = intentArchiveRecoverPurge(storage, slug)
}`,
			prd: prd, adr: adr,
		},
		{name: "pending-must-still-exist", prd: prd, adr: s7PIB447Inject(adr, "Rehydration is not another recovery entry point.", "Pending must still exist after recovery.")},
		{name: "rehydration-dangling-repair", prd: s7PIB447Inject(prd, "### 9.3 `index.json`", "Rehydration repairs a dangling reference."), adr: adr},
		{name: "prepare-finalizes-pending", prd: s7PIB447Inject(prd, "#### 9.7.2 Honest purge procedure and residual race", "A mutating `prepare` finalizes a pending hash."), adr: adr},
		{name: "present-tombstone-divergence", prd: s7PIB447Inject(prd, "#### 9.3.1 Strict decoding — the index is never guessed at", "A tombstone beside a present blob is purge divergence."), adr: adr},
		{name: "present-tombstone-always-orphan", prd: s7PIB447Inject(prd, "#### 9.3.1 Strict decoding — the index is never guessed at", "A tombstone beside a present blob is always an orphan."), adr: adr},
		{name: "recovery-leaves-dangling", prd: s7PIB447Inject(prd, "#### 9.7.2 Honest purge procedure and residual race", "The recovery removes the blob and leaves the other reference dangling."), adr: adr},
		{name: "selector-scoped-X11", prd: s7PIB447Inject(prd, "#### 9.3.1 Strict decoding — the index is never guessed at", "X11 validates only the selected references."), adr: adr},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			wrongSources := map[string]string{}
			for name, source := range sources {
				wrongSources[name] = source
			}
			if fixture.source != "" {
				wrongSources["internal/cli/prepare_publish.go"] += fixture.source
			}
			if err := validateS7PIB447Parity(
				evidence, wrongSources, fixture.prd, fixture.adr,
			); err == nil {
				t.Fatal("same PIB-447 validator accepted the contradictory copied real input")
			}
		})
	}
}

func deriveS7PIB447Evidence(t *testing.T) s7PIB447Evidence {
	t.Helper()
	const feature = "demo"
	var evidence s7PIB447Evidence

	first := archiveReplacement(t, IntentArchiveArtifactAnalysis, "shared", IntentArchiveWireRetained)
	second := archiveReplacement(t, IntentArchiveArtifactSpec, "shared", IntentArchiveWireRetained)
	storage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, first),
		archiveGeneration(t, feature, second),
	))
	storage.putRegular(feature, first.ContentSHA256, []byte("shared"))
	plan, err := PlanIntentArchivePurge(storage, feature, IntentArchivePurgeSelector{
		Blobs: []string{first.ContentSHA256},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	storage.calls = nil
	if _, err := ExecuteIntentArchivePurge(storage, plan); err != nil {
		t.Fatal(err)
	}
	firstCAS := callIndex(storage.calls, "cas-index")
	remove := callIndex(storage.calls, "remove:")
	lastCAS := -1
	for index, call := range storage.calls {
		if strings.HasPrefix(call, "cas-index") {
			lastCAS = index
		}
	}
	evidence.claimBeforeRemove = firstCAS >= 0 && remove > firstCAS
	evidence.tombstoneAfterRemove = lastCAS > remove
	evidence.everyReferenceClaimed = len(plan.References) == 2
	evidence.everyReferenceTombstone = intentArchiveHashAllTombstoned(
		storage.decodedIndex(t, feature), first.ContentSHA256,
	)

	danglingStorage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature,
			archiveReplacement(t, IntentArchiveArtifactAnalysis, "dangling", IntentArchiveWireRetained),
		),
	))
	_, err = PlanIntentArchiveAppend(danglingStorage, feature, []IntentArchiveReplacementInput{{
		ArtifactID: IntentArchiveArtifactAnalysis,
		Path:       "analysis.md",
		PriorBytes: []byte("dangling"),
	}})
	evidence.danglingCode = s7ArchiveErrorCode(err)
	danglingHash := danglingStorage.decodedIndex(t, feature).Generations[0].Replaced[0].ContentSHA256
	danglingPlan, err := PlanIntentArchivePurge(danglingStorage, feature, IntentArchivePurgeSelector{
		Blobs: []string{danglingHash},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	danglingStorage.calls = nil
	if _, err := ExecuteIntentArchivePurge(danglingStorage, danglingPlan); err != nil {
		t.Fatal(err)
	}
	evidence.danglingRepairNoRemove =
		callIndex(danglingStorage.calls, "remove:") < 0 &&
			intentArchiveHashAllTombstoned(danglingStorage.decodedIndex(t, feature), danglingHash)

	mixed := archiveIndex(t, feature,
		archiveGeneration(t, feature,
			archiveReplacement(t, IntentArchiveArtifactAnalysis, "mixed", IntentArchiveWireRetained),
		),
		archiveGeneration(t, feature,
			archiveReplacement(t, IntentArchiveArtifactSpec, "mixed", IntentArchiveWireTombstoned),
		),
	)
	mixedHash := mixed.Generations[0].Replaced[0].ContentSHA256
	mixedReport, err := InspectIntentArchive(mixed, []IntentArchiveBlobObservation{
		archiveObservation(feature, mixedHash, IntentArchiveBlobPresentCorrect, int64(len("mixed"))),
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence.mixedClass = mixedReport.Classes[0].Class

	unreferenced := archiveIndex(t, feature, archiveGeneration(t, feature,
		archiveReplacement(t, IntentArchiveArtifactAnalysis, "residue", IntentArchiveWireTombstoned),
	))
	unreferencedHash := unreferenced.Generations[0].Replaced[0].ContentSHA256
	unreferencedReport, err := InspectIntentArchive(unreferenced, []IntentArchiveBlobObservation{
		archiveObservation(feature, unreferencedHash, IntentArchiveBlobPresentCorrect, int64(len("residue"))),
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence.unreferencedClass = unreferencedReport.Classes[0].Class

	corrupt := archiveIndex(t, feature, archiveGeneration(t, feature,
		archiveReplacement(t, IntentArchiveArtifactAnalysis, "corrupt", IntentArchiveWireRetained),
	))
	corruptHash := corrupt.Generations[0].Replaced[0].ContentSHA256
	corruptReport, err := InspectIntentArchive(corrupt, []IntentArchiveBlobObservation{
		archiveObservation(feature, corruptHash, IntentArchiveBlobUnidentifiable, int64(len("wrong"))),
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence.corruptClass = corruptReport.Classes[0].Class

	owned := archiveReplacement(t, IntentArchiveArtifactAnalysis, "owned", IntentArchiveWireRemovalPending)
	ownedStorage := newArchiveMemoryStorage(t, archiveIndex(t, feature, archiveGeneration(t, feature, owned)))
	ownedRel := ownedStorage.putRegular(feature, owned.ContentSHA256, []byte("wrong"))
	blob := ownedStorage.blobs[ownedRel]
	blob.kind = IntentArchiveBlobKindNonRegular
	ownedStorage.blobs[ownedRel] = blob
	_, err = RecoverPendingPurge(ownedStorage, feature)
	var ownedErr *IntentArchiveError
	if errors.As(err, &ownedErr) {
		evidence.ownedCorruptCode = ownedErr.Code
		evidence.ownedCorruptExit = ownedErr.ExitClass
	}

	selected := archiveReplacement(t, IntentArchiveArtifactSpec, "selected", IntentArchiveWireRetained)
	selectorStorage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature,
			archiveReplacement(t, IntentArchiveArtifactAnalysis, "unselected dangling", IntentArchiveWireRetained),
			selected,
		),
	))
	selectorStorage.putRegular(feature, selected.ContentSHA256, []byte("selected"))
	_, err = PlanIntentArchivePurge(selectorStorage, feature, IntentArchivePurgeSelector{
		Blobs: []string{selected.ContentSHA256},
	}, true)
	evidence.selectorIndependentCode = s7ArchiveErrorCode(err)

	partialFirst := archiveReplacement(t, IntentArchiveArtifactAnalysis, "partial-a", IntentArchiveWireRetained)
	partialSecond := archiveReplacement(t, IntentArchiveArtifactSpec, "partial-b", IntentArchiveWireRetained)
	partialStorage := newArchiveMemoryStorage(t, archiveIndex(t, feature,
		archiveGeneration(t, feature, partialFirst, partialSecond),
	))
	partialStorage.putRegular(feature, partialFirst.ContentSHA256, []byte("partial-a"))
	secondRel := partialStorage.putRegular(feature, partialSecond.ContentSHA256, []byte("partial-b"))
	partialPlan, err := PlanIntentArchivePurge(partialStorage, feature, IntentArchivePurgeSelector{
		Blobs: []string{partialFirst.ContentSHA256, partialSecond.ContentSHA256},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	partialStorage.fail["remove:"+filepath.Base(secondRel)] = 1
	_, err = ExecuteIntentArchivePurge(partialStorage, partialPlan)
	var partialErr *IntentArchiveError
	if errors.As(err, &partialErr) {
		evidence.partialCode = partialErr.Code
		evidence.partialExit = partialErr.ExitClass
	}
	return evidence
}

func validateS7PIB447Parity(
	evidence s7PIB447Evidence,
	sources map[string]string,
	prd, adr string,
) error {
	if !evidence.claimBeforeRemove || !evidence.tombstoneAfterRemove ||
		!evidence.everyReferenceClaimed || !evidence.everyReferenceTombstone {
		return fmt.Errorf("PIB-447 production claim/remove/tombstone order drift: %+v", evidence)
	}
	if evidence.danglingCode != IntentArchiveCodeBlobDangling ||
		!evidence.danglingRepairNoRemove ||
		evidence.mixedClass != IntentArchiveRepairMixedReference ||
		evidence.unreferencedClass != IntentArchiveRepairUnreferencedResidue ||
		evidence.corruptClass != IntentArchiveRepairCorruptObject ||
		evidence.ownedCorruptCode != IntentArchiveCodePurgeEvidenceDivergent ||
		evidence.ownedCorruptExit != 6 ||
		evidence.selectorIndependentCode != IntentArchiveCodeBlobDangling ||
		evidence.partialCode != IntentArchiveCodePurgePartial ||
		evidence.partialExit != 5 {
		return fmt.Errorf("PIB-447 production classifier/state model drift: %+v", evidence)
	}
	callSites := []string{}
	for name, source := range sources {
		file, err := parser.ParseFile(token.NewFileSet(), name, source, 0)
		if err != nil {
			return err
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				ident, _ := call.Fun.(*ast.Ident)
				if ident != nil && ident.Name == "intentArchiveRecoverPurge" {
					callSites = append(callSites, name+":"+function.Name.Name)
				}
				return true
			})
		}
	}
	if len(callSites) != 1 ||
		!strings.HasSuffix(callSites[0], ":runFeatureIntentArchivePurgeConfirmed") {
		return fmt.Errorf("PIB-447 purge-owner call sites = %v, want one purge --yes owner", callSites)
	}
	if err := s7PIB447ProductionOrder(sources["internal/store/intent_archive.go"]); err != nil {
		return err
	}
	indexSemantics, err := s7StoreSection(prd, "### 9.3 ", "### 9.4 ")
	if err != nil {
		return err
	}
	retentionSemantics, err := s7StoreSection(prd, "### 9.7 ", "### 9.8 ")
	if err != nil {
		return err
	}
	crashSemantics, err := s7StoreSection(prd, "| CP11 |", "| CP14 |")
	if err != nil {
		return err
	}
	d10, err := s7StoreSection(adr, "### D10 ", "### D11 ")
	if err != nil {
		return err
	}
	d16, err := s7StoreSection(adr, "### D16 ", "### D17 ")
	if err != nil {
		return err
	}
	for _, semantic := range []struct {
		label string
		body  string
		terms []string
	}{
		{
			label: "PRD §9.3/X11", body: indexSemantics,
			terms: []string{
				"every tombstoned reference",
				"removal-pending, `h` is purge-owned",
				"X11's scope is the whole index, and it does not depend on the selector",
				"confirmed purge is its only shipped repair",
			},
		},
		{
			label: "PRD §9.7.1/§9.7.2", body: retentionSemantics,
			terms: []string{
				"**Claim `h` globally.**",
				"**Remove.** Rooted removal of `h.blob`",
				"**Tombstone.** CAS-publish **every** reference to `h`",
				"archive-purge-partial",
			},
		},
		{
			label: "PRD CP12/CP12a/CP13", body: crashSemantics,
			terms: []string{
				"claims `h` globally",
				"pending `h`; `h.blob` absent",
				"ownership outranks both subcases",
			},
		},
		{
			label: "ADR D10", body: d10,
			terms: []string{
				"makes every **tombstoned**\nreference",
				"Rehydration never consumes a removal-pending reference",
				"claim CAS sets retained references **and already-tombstoned references**",
			},
		},
		{
			label: "ADR D16", body: d16,
			terms: []string{
				"one CAS-published\nindex rewrite setting every reference to `h`",
				"X11 storage observation over the **whole\nindex**",
				"Rehydration is not another recovery entry point",
			},
		},
	} {
		for _, term := range semantic.terms {
			if !strings.Contains(semantic.body, term) {
				return fmt.Errorf("PIB-447 %s lost semantic claim %q", semantic.label, term)
			}
		}
	}
	combined := indexSemantics + "\n" + retentionSemantics + "\n" +
		crashSemantics + "\n" + d10 + "\n" + d16
	for _, contradiction := range []string{
		"pending must still exist",
		"rehydration repairs a dangling reference",
		"a mutating `prepare` finalizes a pending hash",
		"a tombstone beside a present blob is purge divergence",
		"a tombstone beside a present blob is always an orphan",
		"the recovery removes the blob and leaves the other reference dangling",
		"x11 validates only the selected references",
	} {
		if strings.Contains(strings.ToLower(combined), contradiction) {
			return fmt.Errorf("PIB-447 contradictory normative claim %q", contradiction)
		}
	}
	return nil
}

func s7PIB447Inject(document, anchor, sentence string) string {
	return strings.Replace(document, anchor, anchor+"\n\n"+sentence, 1)
}

func s7PIB447ProductionOrder(source string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "intent_archive.go", source, 0)
	if err != nil {
		return err
	}
	var body *ast.BlockStmt
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Name.Name == "executeIntentArchivePurgeHash" {
			body = function.Body
		}
	}
	if body == nil {
		return fmt.Errorf("PIB-447 production hash transition missing")
	}
	position := 0
	claimPublish, remove, tombstonePublish := -1, -1, -1
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		position++
		ident, _ := call.Fun.(*ast.Ident)
		if ident == nil {
			return true
		}
		switch ident.Name {
		case "publishIntentArchiveIndex":
			if len(call.Args) != 4 {
				return true
			}
			last, _ := call.Args[3].(*ast.Ident)
			if last != nil && last.Name == "claimed" {
				claimPublish = position
			}
			if last != nil && last.Name == "tombstoned" {
				tombstonePublish = position
			}
		case "removeIntentArchiveBlob":
			if claimPublish >= 0 && remove < 0 {
				remove = position
			}
		}
		return true
	})
	if claimPublish < 0 || remove <= claimPublish || tombstonePublish <= remove {
		return fmt.Errorf(
			"PIB-447 production claim/remove/tombstone control flow = claim:%d remove:%d tombstone:%d",
			claimPublish, remove, tombstonePublish,
		)
	}
	return nil
}

func s7StoreSection(document, start, end string) (string, error) {
	startAt := strings.Index(document, start)
	if startAt < 0 {
		return "", fmt.Errorf("PIB-447 section %q missing", start)
	}
	rest := document[startAt+len(start):]
	endAt := strings.Index(rest, end)
	if endAt < 0 {
		return "", fmt.Errorf("PIB-447 section %q lacks terminator %q", start, end)
	}
	return rest[:endAt], nil
}

func s7ArchiveErrorCode(err error) IntentArchiveErrorCode {
	var typed *IntentArchiveError
	if errors.As(err, &typed) {
		return typed.Code
	}
	return ""
}

func s7StoreRepositoryFile(t *testing.T, rel string) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
