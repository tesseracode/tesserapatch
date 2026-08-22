//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	_ "unsafe"

	"github.com/tesseracode/tesserapatch/internal/store"
)

//go:linkname s7APFailPurgeAfterFirstMutation github.com/tesseracode/tesserapatch/internal/store.failPurgeAfterFirstMutation
var s7APFailPurgeAfterFirstMutation func() error

//go:linkname s7APAfterPurgeIndexRename github.com/tesseracode/tesserapatch/internal/store.afterPurgeIndexRename
var s7APAfterPurgeIndexRename func(string)

//go:linkname s7APBeforePurgeBlobRemove github.com/tesseracode/tesserapatch/internal/store.beforePurgeBlobRemove
var s7APBeforePurgeBlobRemove func(string)

//go:linkname s7APBeforePendingTombstoneCAS github.com/tesseracode/tesserapatch/internal/store.beforePendingTombstoneCAS
var s7APBeforePendingTombstoneCAS func(string)

func TestS7APPurgeCLIContracts(t *testing.T) {
	t.Run("PIB-466", func(t *testing.T) {
		observation := s7APRunPartialPurge(t)
		progress := observation.report.PurgeProgress
		if observation.code != 5 || progress == nil ||
			observation.report.Outcome != string(store.IntentArchivePurgePartial) ||
			progress.Resume != string(store.IntentArchiveResumePendingRecoveryThenCompletion) ||
			progress.PendingHash != observation.hashes[0] ||
			len(progress.CompletedHashes) != 0 ||
			fmt.Sprint(progress.RemainingHashes) != fmt.Sprint(observation.hashes[1:]) ||
			progress.RetryCWD != store.IntentArchiveRepairCWD ||
			progress.State != store.IntentArchivePurgeStateConsistent ||
			strings.Contains(progress.Retry, observation.root) {
			t.Fatalf("PIB-466 partial public report = exit:%d report:%+v",
				observation.code, observation.report)
		}
		wantArgv := []string{
			"feature", "intent-archive", "purge", observation.slug,
			"--all", "--yes", "--json", "--quiet",
		}
		argv, err := s7APParseRenderedCommand(progress.Retry)
		if err != nil || fmt.Sprint(argv) != fmt.Sprint(wantArgv) {
			t.Fatalf("PIB-466 retry argv = %v err=%v, want %v", argv, err, wantArgv)
		}
		if !intentArchiveCLIHashAllState(
			observation.index,
			observation.hashes[0],
			store.IntentArchiveWireRemovalPending,
		) {
			t.Fatalf("PIB-466 strict intermediate index lacks pending owner: %+v", observation.index)
		}
	})

	t.Run("PIB-467", func(t *testing.T) {
		observation := s7APRunPartialPurge(t)
		argv, err := s7APParseRenderedCommand(observation.report.PurgeProgress.Retry)
		if err != nil {
			t.Fatal(err)
		}
		recoveredCode, recoveredOut, recoveredErr := s7APRunFromWorkspace(t, observation.root, argv)
		recovered := decodeIntentArchivePurgeReport(t, recoveredOut)
		_, recoveredIndex := readIntentArchiveCLIIndex(t, observation.root, observation.slug)
		if recoveredCode != 0 || recoveredErr != "" ||
			recovered.Outcome != string(store.IntentArchivePurgeRecovered) ||
			recovered.Recovery == nil ||
			fmt.Sprint(recovered.Recovery.FinalizedHashes) !=
				fmt.Sprint([]string{observation.hashes[0]}) ||
			!intentArchiveCLIHashAllState(
				recoveredIndex,
				observation.hashes[0],
				store.IntentArchiveWireTombstoned,
			) {
			t.Fatalf("PIB-467 pending recovery = exit:%d stderr:%q report:%+v index:%+v",
				recoveredCode, recoveredErr, recovered, recoveredIndex)
		}
		nextArgv, err := s7APParseRenderedCommand(recovered.Recovery.Retry)
		if err != nil || fmt.Sprint(nextArgv) != fmt.Sprint(argv) {
			t.Fatalf("PIB-467 recovery retry = %v err=%v, want %v", nextArgv, err, argv)
		}
		completedCode, completedOut, completedErr := s7APRunFromWorkspace(
			t, observation.root, nextArgv,
		)
		completed := decodeIntentArchivePurgeReport(t, completedOut)
		_, finalIndex := readIntentArchiveCLIIndex(t, observation.root, observation.slug)
		if completedCode != 0 || completedErr != "" {
			t.Fatalf("PIB-467 completion transport = exit:%d stderr:%q",
				completedCode, completedErr)
		}
		if err := validateS7APCompletedPurgeReport(completed, observation, finalIndex); err != nil {
			t.Fatalf("PIB-467 completion report: %v\n%+v", err, completed)
		}
		wrongCompleted := completed
		wrongCompleted.Hashes = append([]string(nil), completed.Hashes[1:]...)
		if err := validateS7APCompletedPurgeReport(
			wrongCompleted, observation, finalIndex,
		); err == nil {
			t.Fatal("PIB-467 completion validator accepted a missing completed hash")
		}
		if !intentArchiveCLIHashAllState(
			finalIndex,
			observation.hashes[0],
			store.IntentArchiveWireTombstoned,
		) ||
			!intentArchiveCLIHashAllState(
				finalIndex,
				observation.hashes[1],
				store.IntentArchiveWireTombstoned,
			) {
			t.Fatalf("PIB-467 completion index = %+v", finalIndex)
		}
	})

	t.Run("PIB-468", func(t *testing.T) {
		observation := s7APRunDivergentPurge(t)
		divergence := observation.report.Divergence
		indexRel, _ := store.IntentArchiveIndexRel(observation.slug)
		pendingBlobRel, _ := store.IntentArchiveBlobRel(
			observation.slug, observation.hashes[0],
		)
		if observation.code != 6 || observation.stderr == "" ||
			observation.report.Outcome != "refused" ||
			observation.report.Refusal == nil ||
			observation.report.Refusal.Code != string(store.IntentArchiveCodePurgeEvidenceDivergent) ||
			divergence == nil ||
			divergence.Kind != "blob" ||
			divergence.PendingHash != observation.hashes[0] ||
			divergence.Blob != pendingBlobRel ||
			divergence.Index != indexRel ||
			len(divergence.CompletedHashes) != 0 ||
			fmt.Sprint(divergence.RemainingHashes) != fmt.Sprint(observation.hashes[1:]) ||
			fmt.Sprint(observation.report.Hashes) != fmt.Sprint(observation.hashes) ||
			!s7APAllSelectedBlobsNamed(observation.report, observation.hashes) ||
			!reflect.DeepEqual(observation.events, []string{
				"pending-cas", "post-cas-evidence-read", "production-exit-6",
			}) ||
			!bytes.Equal(observation.pendingIndex, observation.finalIndex) {
			t.Fatalf("PIB-468 public divergence = exit:%d stderr:%q report:%+v events:%v",
				observation.code, observation.stderr, observation.report, observation.events)
		}
		for hash, before := range observation.blobs {
			blobRel, _ := store.IntentArchiveBlobRel(observation.slug, hash)
			after, err := os.ReadFile(filepath.Join(observation.root, filepath.FromSlash(blobRel)))
			if err != nil || !bytes.Equal(after, before) {
				t.Fatalf("PIB-468 selected blob %s changed: err=%v before=%q after=%q",
					hash, err, before, after)
			}
		}
		_, index := readIntentArchiveCLIIndex(t, observation.root, observation.slug)
		if !intentArchiveCLIHashAllState(
			index, observation.hashes[0], store.IntentArchiveWireRemovalPending,
		) ||
			!intentArchiveCLIHashAllState(
				index, observation.hashes[1], store.IntentArchiveWireRetained,
			) {
			t.Fatalf("PIB-468 exact pending index = %+v", index)
		}
		source := s6RepoFile(t, "internal/store/intent_archive.go")
		if err := validateS7APPostClaimEvidenceValidation(source); err != nil {
			t.Fatal(err)
		}
		skippedValidation := strings.Replace(
			source,
			"revalidatedProbe, err := storage.ProbeBlob(blobRel)",
			"revalidatedProbe := probe\n\tvar err error",
			1,
		)
		if err := validateS7APPostClaimEvidenceValidation(skippedValidation); err == nil {
			t.Fatal("PIB-468 validator accepted removal without a post-claim evidence read")
		}
	})
}

func s7APAllSelectedBlobsNamed(report intentArchivePurgeReport, hashes []string) bool {
	if len(report.Blobs) != len(hashes) {
		return false
	}
	for index, blob := range report.Blobs {
		if blob.Hash != hashes[index] || blob.Path == "" ||
			!blob.Present || blob.Removed {
			return false
		}
	}
	return true
}

func validateS7APCompletedPurgeReport(
	report intentArchivePurgeReport,
	observation s7APPartialPurgeObservation,
	finalIndex store.IntentArchiveIndex,
) error {
	wantHashes := append([]string(nil), observation.hashes...)
	sort.Strings(wantHashes)
	wantGenerations := s7GenerationIDs(finalIndex)
	sort.Strings(wantGenerations)
	if report.SchemaVersion != 1 ||
		report.Command != "feature intent-archive purge" ||
		report.Slug != observation.slug ||
		report.Outcome != string(store.IntentArchivePurgePurged) ||
		report.Action != "none" ||
		report.Selector != "all" ||
		!report.Confirmed ||
		fmt.Sprint(report.Hashes) != fmt.Sprint(wantHashes) ||
		fmt.Sprint(report.GenerationIDs) != fmt.Sprint(wantGenerations) {
		return fmt.Errorf("identity/work list = schema:%d command:%q slug:%q outcome:%q action:%q selector:%q confirmed:%t hashes:%v generations:%v",
			report.SchemaVersion, report.Command, report.Slug, report.Outcome,
			report.Action, report.Selector, report.Confirmed, report.Hashes,
			report.GenerationIDs)
	}
	if len(report.References) != len(wantHashes) {
		return fmt.Errorf("references = %d, want %d", len(report.References), len(wantHashes))
	}
	for index, reference := range report.References {
		if reference.Hash != wantHashes[index] ||
			reference.GenerationID != wantGenerations[0] ||
			reference.Path == "" || filepath.IsAbs(reference.Path) {
			return fmt.Errorf("reference %d = %+v", index, reference)
		}
		wantState := string(store.IntentArchiveWireRetained)
		if index == 0 {
			wantState = string(store.IntentArchiveWireTombstoned)
		}
		if reference.WireState != wantState {
			return fmt.Errorf("reference %d state = %q, want %q", index, reference.WireState, wantState)
		}
	}
	if len(report.Blobs) != 1 ||
		report.Blobs[0].Hash != wantHashes[1] ||
		!report.Blobs[0].Removed || report.Blobs[0].Present ||
		report.Blobs[0].Path == "" || filepath.IsAbs(report.Blobs[0].Path) {
		return fmt.Errorf("completed blob work = %+v", report.Blobs)
	}
	const history = "A committed blob remains in Git history; removing it from history is not something tpatch performs, and tpatch does not rewrite Git history."
	const blast = "The --all selector tombstones every reference in every generation and removes every blob in this archive. The unconfirmed preview is the default; repeated --blob selectors are the narrower alternative."
	if len(report.OrphanBlobs) != 0 || len(report.Advisories) != 0 ||
		report.HistoryDisclosure != history || report.BlastRadius != blast ||
		report.Retry != "" || report.RetryCWD != "" ||
		report.Refusal != nil || report.Recovery != nil ||
		report.PendingPurge != nil || report.PurgeProgress != nil ||
		report.RemainingRepairs != nil || report.Divergence != nil {
		return fmt.Errorf("completion optional/truth fields = %+v", report)
	}
	return nil
}

func TestS7APPurgeControlFlowGuard(t *testing.T) {
	t.Run("PIB-469", func(t *testing.T) {
		catalog, err := s6RefusalCatalogFromPRD(t)
		if err != nil {
			t.Fatal(err)
		}
		exitThree := 0
		for _, code := range catalog {
			if s6ExpectedRefusalExit(code) != 3 {
				continue
			}
			exitThree++
			s7APExitThreeSnapshotMode = true
			s7APExitThreeSnapshots = map[string]string{}
			observation := s7APObserveRefusalOnce(t, code)
			s7APExitThreeSnapshotMode = false
			if observation.exit != 3 || observation.code != code {
				t.Fatalf("PIB-469 %s = exit:%d code:%q", code, observation.exit, observation.code)
			}
			if len(s7APExitThreeSnapshots) == 0 {
				t.Fatalf("PIB-469 %s produced no whole-tree snapshot", code)
			}
			for root, before := range s7APExitThreeSnapshots {
				if after := snapshotTreeMetadata(t, "exit-3", root); before != after {
					t.Fatalf("PIB-469 %s exit-3 path mutated %s", code, root)
				}
			}
			s7APExitThreeSnapshots = nil
		}
		if exitThree == 0 {
			t.Fatal("PIB-469 catalog has no exit-3 population")
		}

		storeSource := s6RepoFile(t, "internal/store/intent_archive.go")
		cliSource := s6RepoFile(t, "internal/cli/feature_intent_archive.go")
		if err := validateS7APPurgeControlFlow(storeSource, cliSource); err != nil {
			t.Fatal(err)
		}
		writeThenExitThree := strings.Replace(
			storeSource,
			"if committed {\n\t\treturn 5\n\t}\n\treturn 3",
			"if committed {\n\t\treturn 3\n\t}\n\treturn 3",
			1,
		)
		if err := validateS7APPurgeControlFlow(writeThenExitThree, cliSource); err == nil {
			t.Fatal("PIB-469 same validator accepted write-then-exit-3")
		}
		recoverThenExitThree := strings.Replace(
			cliSource,
			"return emitIntentArchivePurgeReport(cmd, report, 0)\n\t\t}",
			"return emitIntentArchivePurgeReport(cmd, report, 3)\n\t\t}",
			1,
		)
		if err := validateS7APPurgeControlFlow(storeSource, recoverThenExitThree); err == nil {
			t.Fatal("PIB-469 same validator accepted recovery fallthrough to exit 3")
		}
	})
}

type s7APPartialPurgeObservation struct {
	root   string
	slug   string
	hashes []string
	code   int
	stdout string
	stderr string
	report intentArchivePurgeReport
	index  store.IntentArchiveIndex
}

type s7APDivergentPurgeObservation struct {
	root         string
	slug         string
	hashes       []string
	blobs        map[string][]byte
	code         int
	stderr       string
	report       intentArchivePurgeReport
	events       []string
	pendingIndex []byte
	finalIndex   []byte
}

type s7APHashWrongProbeStorage struct {
	store.IntentArchiveStorage
	targetHash string
	armed      *bool
	events     *[]string
	injections int
}

func (storage *s7APHashWrongProbeStorage) ProbeBlob(
	blobRel string,
) (store.IntentArchiveBlobProbe, error) {
	probe, err := storage.IntentArchiveStorage.ProbeBlob(blobRel)
	if err != nil || !*storage.armed || storage.injections != 0 ||
		!strings.HasSuffix(blobRel, storage.targetHash+".blob") {
		return probe, err
	}
	*storage.events = append(*storage.events, "post-cas-evidence-read")
	storage.injections++
	probe.SHA256 = strings.Repeat("f", 64)
	if probe.SHA256 == storage.targetHash {
		probe.SHA256 = strings.Repeat("e", 64)
	}
	return probe, nil
}

func s7APRunDivergentPurge(t *testing.T) s7APDivergentPurgeObservation {
	t.Helper()
	root, slug := intentArchiveCLIWorkspace(t)
	firstBody := []byte("S7 AP first divergent hash\n")
	secondBody := []byte("S7 AP second divergent hash\n")
	first := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactAnalysis, firstBody, store.IntentArchiveWireRetained,
	)
	second := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactSpec, secondBody, store.IntentArchiveWireRetained,
	)
	hashes := []string{first.ContentSHA256, second.ContentSHA256}
	sort.Strings(hashes)
	blobs := map[string][]byte{
		first.ContentSHA256:  firstBody,
		second.ContentSHA256: secondBody,
	}
	writeIntentArchiveCLIFixture(
		t,
		root,
		slug,
		intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, first, second)),
		blobs,
	)

	previousAfterCAS := s7APAfterPurgeIndexRename
	previousBeforeRemove := s7APBeforePurgeBlobRemove
	previousBeforeTombstone := s7APBeforePendingTombstoneCAS
	previousExecute := intentArchiveExecutePurge
	t.Cleanup(func() {
		s7APAfterPurgeIndexRename = previousAfterCAS
		s7APBeforePurgeBlobRemove = previousBeforeRemove
		s7APBeforePendingTombstoneCAS = previousBeforeTombstone
		intentArchiveExecutePurge = previousExecute
	})
	events := []string{}
	armed := false
	var pendingIndex []byte
	s7APAfterPurgeIndexRename = func(string) {
		events = append(events, "pending-cas")
		armed = true
		raw, err := os.ReadFile(filepath.Join(
			root, ".tpatch", "features", slug, "artifacts", "intent-archive", "index.json",
		))
		if err != nil {
			t.Fatal(err)
		}
		pendingIndex = append([]byte(nil), raw...)
	}
	s7APBeforePurgeBlobRemove = func(string) {
		events = append(events, "blob-remove")
	}
	s7APBeforePendingTombstoneCAS = func(string) {
		events = append(events, "tombstone-cas")
	}
	intentArchiveExecutePurge = func(
		storage store.IntentArchiveStorage,
		plan store.IntentArchivePurgePlan,
	) (store.IntentArchivePurgeResult, error) {
		wrapped := &s7APHashWrongProbeStorage{
			IntentArchiveStorage: storage,
			targetHash:           hashes[0],
			armed:                &armed,
			events:               &events,
		}
		result, err := store.ExecuteIntentArchivePurge(wrapped, plan)
		var typed *store.IntentArchiveError
		if errors.As(err, &typed) &&
			typed.Code == store.IntentArchiveCodePurgeEvidenceDivergent {
			events = append(events, "production-exit-6")
		}
		return result, err
	}
	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--all", "--yes", "--json", "--quiet",
	)
	s7APAfterPurgeIndexRename = previousAfterCAS
	s7APBeforePurgeBlobRemove = previousBeforeRemove
	s7APBeforePendingTombstoneCAS = previousBeforeTombstone
	intentArchiveExecutePurge = previousExecute
	finalIndex, err := os.ReadFile(filepath.Join(
		root, ".tpatch", "features", slug, "artifacts", "intent-archive", "index.json",
	))
	if err != nil {
		t.Fatal(err)
	}
	return s7APDivergentPurgeObservation{
		root:         root,
		slug:         slug,
		hashes:       hashes,
		blobs:        blobs,
		code:         code,
		stderr:       stderr,
		report:       decodeIntentArchivePurgeReport(t, stdout),
		events:       events,
		pendingIndex: pendingIndex,
		finalIndex:   finalIndex,
	}
}

func validateS7APPostClaimEvidenceValidation(source string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "intent_archive.go", source, 0)
	if err != nil {
		return err
	}
	var function *ast.FuncDecl
	for _, declaration := range file.Decls {
		candidate, _ := declaration.(*ast.FuncDecl)
		if candidate != nil && candidate.Name.Name == "executeIntentArchivePurgeHash" {
			function = candidate
			break
		}
	}
	if function == nil {
		return errors.New("executeIntentArchivePurgeHash is missing")
	}
	claimCAS, evidenceRead, hashValidation, removal := token.NoPos, token.NoPos, token.NoPos, token.NoPos
	ast.Inspect(function.Body, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.CallExpr:
			switch called := value.Fun.(type) {
			case *ast.Ident:
				if called.Name == "publishIntentArchiveIndex" && claimCAS == token.NoPos {
					claimCAS = value.Pos()
				}
				if called.Name == "removeIntentArchiveBlob" &&
					evidenceRead != token.NoPos && removal == token.NoPos {
					removal = value.Pos()
				}
			case *ast.SelectorExpr:
				receiver, _ := called.X.(*ast.Ident)
				if receiver != nil && receiver.Name == "storage" &&
					called.Sel.Name == "ProbeBlob" && claimCAS != token.NoPos &&
					evidenceRead == token.NoPos {
					evidenceRead = value.Pos()
				}
			}
		case *ast.BinaryExpr:
			if value.Op == token.NEQ &&
				strings.Contains(s7APNodeText(source, value), "revalidatedProbe.SHA256") &&
				strings.Contains(s7APNodeText(source, value), "hash") {
				hashValidation = value.Pos()
			}
		}
		return true
	})
	if claimCAS == token.NoPos || evidenceRead == token.NoPos ||
		hashValidation == token.NoPos || removal == token.NoPos ||
		!(claimCAS < evidenceRead && evidenceRead < hashValidation && hashValidation < removal) {
		return fmt.Errorf("post-claim evidence order = claim:%d read:%d validate:%d remove:%d",
			claimCAS, evidenceRead, hashValidation, removal)
	}
	return nil
}

func s7APRunPartialPurge(t *testing.T) s7APPartialPurgeObservation {
	t.Helper()
	root, slug := intentArchiveCLIWorkspace(t)
	firstBody := []byte("S7 AP first partial hash\n")
	secondBody := []byte("S7 AP second partial hash\n")
	first := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactAnalysis, firstBody, store.IntentArchiveWireRetained,
	)
	second := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactSpec, secondBody, store.IntentArchiveWireRetained,
	)
	hashes := []string{first.ContentSHA256, second.ContentSHA256}
	sort.Strings(hashes)
	writeIntentArchiveCLIFixture(
		t,
		root,
		slug,
		intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, first, second)),
		map[string][]byte{
			first.ContentSHA256:  firstBody,
			second.ContentSHA256: secondBody,
		},
	)
	hookCalls := 0
	previous := s7APFailPurgeAfterFirstMutation
	s7APFailPurgeAfterFirstMutation = func() error {
		hookCalls++
		return errors.New("S7 AP fail after first committed purge mutation")
	}
	t.Cleanup(func() { s7APFailPurgeAfterFirstMutation = previous })
	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "feature", "intent-archive", "purge", slug,
		"--all", "--yes", "--json", "--quiet",
	)
	s7APFailPurgeAfterFirstMutation = previous
	report := decodeIntentArchivePurgeReport(t, stdout)
	_, index := readIntentArchiveCLIIndex(t, root, slug)
	if code != 5 || stderr == "" || hookCalls != 1 ||
		report.PurgeProgress == nil {
		t.Fatalf("partial purge fixture = exit:%d stderr:%q hooks:%d report:%+v",
			code, stderr, hookCalls, report)
	}
	return s7APPartialPurgeObservation{
		root: root, slug: slug, hashes: hashes, code: code,
		stdout: stdout, stderr: stderr, report: report, index: index,
	}
}

func s7APRunFromWorkspace(t *testing.T, root string, argv []string) (int, string, string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	}()
	code, stdout, stderr, _ := runPrepare(t, argv...)
	return code, stdout, stderr
}

func validateS7APPurgeControlFlow(storeSource, cliSource string) error {
	parseFunction := func(name, source, functionName string) (*ast.FuncDecl, error) {
		file, err := parser.ParseFile(token.NewFileSet(), name, source, 0)
		if err != nil {
			return nil, err
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Name.Name == functionName && function.Body != nil {
				return function, nil
			}
		}
		return nil, fmt.Errorf("%s is missing", functionName)
	}
	exitFunction, err := parseFunction(
		"intent_archive.go", storeSource, "intentArchiveExitAfterMutation",
	)
	if err != nil {
		return err
	}
	if len(exitFunction.Body.List) != 2 {
		return fmt.Errorf("exit-after-mutation statements = %d, want 2", len(exitFunction.Body.List))
	}
	condition, ok := exitFunction.Body.List[0].(*ast.IfStmt)
	if !ok {
		return errors.New("exit-after-mutation lost committed branch")
	}
	committed, _ := condition.Cond.(*ast.Ident)
	if committed == nil || committed.Name != "committed" ||
		s7APSingleReturnedInteger(condition.Body) != 5 {
		return errors.New("committed purge mutation no longer maps to exit 5")
	}
	finalReturn, _ := exitFunction.Body.List[1].(*ast.ReturnStmt)
	if finalReturn == nil || len(finalReturn.Results) != 1 {
		return errors.New("pre-mutation purge return is malformed")
	}
	finalValue, _ := finalReturn.Results[0].(*ast.BasicLit)
	if finalValue == nil || finalValue.Value != "3" {
		return errors.New("pre-mutation purge refusal no longer maps to exit 3")
	}

	confirmed, err := parseFunction(
		"feature_intent_archive.go", cliSource, "runFeatureIntentArchivePurgeConfirmed",
	)
	if err != nil {
		return err
	}
	var recoveryIf *ast.IfStmt
	planPosition := token.NoPos
	ast.Inspect(confirmed.Body, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.IfStmt:
			if strings.Contains(s7APNodeText(cliSource, value.Cond), "IntentArchivePurgeRecovered") {
				recoveryIf = value
			}
		case *ast.CallExpr:
			ident, _ := value.Fun.(*ast.Ident)
			if ident != nil && ident.Name == "intentArchivePlanPurge" {
				planPosition = value.Pos()
			}
		}
		return true
	})
	if recoveryIf == nil || planPosition == token.NoPos || recoveryIf.Pos() >= planPosition {
		return errors.New("pending recovery is not checked before selector planning")
	}
	if len(recoveryIf.Body.List) == 0 {
		return errors.New("pending recovery branch is empty")
	}
	terminal, _ := recoveryIf.Body.List[len(recoveryIf.Body.List)-1].(*ast.ReturnStmt)
	if terminal == nil || len(terminal.Results) != 1 {
		return errors.New("successful pending recovery is not terminal")
	}
	call, _ := terminal.Results[0].(*ast.CallExpr)
	if call == nil || len(call.Args) != 3 {
		return errors.New("successful pending recovery report return is malformed")
	}
	exit, _ := call.Args[2].(*ast.BasicLit)
	if exit == nil || exit.Value != "0" {
		return errors.New("successful pending recovery does not return exit 0")
	}
	return nil
}

func s7APSingleReturnedInteger(body *ast.BlockStmt) int {
	if body == nil || len(body.List) != 1 {
		return -1
	}
	statement, _ := body.List[0].(*ast.ReturnStmt)
	if statement == nil || len(statement.Results) != 1 {
		return -1
	}
	value, _ := statement.Results[0].(*ast.BasicLit)
	if value == nil {
		return -1
	}
	if value.Value == "5" {
		return 5
	}
	if value.Value == "3" {
		return 3
	}
	return -1
}

func s7APNodeText(source string, node ast.Node) string {
	if node == nil {
		return ""
	}
	start := int(node.Pos()) - 1
	end := int(node.End()) - 1
	if start < 0 || end < start || end > len(source) {
		return ""
	}
	return source[start:end]
}
