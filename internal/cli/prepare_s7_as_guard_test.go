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

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func s7ASReachableTuples() []store.IntentArchiveTuple {
	wireStates := []store.IntentArchiveWireState{
		store.IntentArchiveWireRetained,
		store.IntentArchiveWireRemovalPending,
		store.IntentArchiveWireTombstoned,
	}
	blobStates := []store.IntentArchiveBlobState{
		store.IntentArchiveBlobAbsent,
		store.IntentArchiveBlobPresentCorrect,
		store.IntentArchiveBlobUnidentifiable,
	}
	tuples := []store.IntentArchiveTuple{}
	for _, wireState := range wireStates {
		for _, blobState := range blobStates {
			for _, owned := range []bool{false, true} {
				for _, live := range []bool{false, true} {
					tuple := store.IntentArchiveTuple{
						WireState: wireState,
						BlobState: blobState,
						Owned:     owned,
						Live:      live,
					}
					if store.IntentArchiveTupleReachable(tuple) {
						tuples = append(tuples, tuple)
					}
				}
			}
		}
	}
	return tuples
}

func s7ASExpectedTupleResult(tuple store.IntentArchiveTuple) (store.IntentArchiveTupleResult, error) {
	if !store.IntentArchiveTupleReachable(tuple) {
		return store.IntentArchiveTupleResult{}, fmt.Errorf("unreachable tuple: %+v", tuple)
	}
	if tuple.Owned {
		result := store.IntentArchiveTupleResult{
			Reachable: true,
			Action:    store.IntentArchiveActionRoutePendingOwner,
		}
		switch tuple.BlobState {
		case store.IntentArchiveBlobAbsent:
			result.Disposition = store.IntentArchiveDispositionPendingFinalize
		case store.IntentArchiveBlobPresentCorrect, store.IntentArchiveBlobUnidentifiable:
			result.Disposition = store.IntentArchiveDispositionPendingRemove
		default:
			return store.IntentArchiveTupleResult{}, fmt.Errorf("owned tuple has unexpected blob state: %+v", tuple)
		}
		return result, nil
	}
	if tuple.BlobState == store.IntentArchiveBlobUnidentifiable {
		return store.IntentArchiveTupleResult{
			Reachable:   true,
			Disposition: store.IntentArchiveDispositionCorruptObject,
			Action:      store.IntentArchiveActionRemoveCorruptObject,
			Code:        store.IntentArchiveCodeBlobCorrupt,
			RepairClass: store.IntentArchiveRepairCorruptObject,
			ExitClass:   3,
		}, nil
	}
	switch tuple.WireState {
	case store.IntentArchiveWireRetained:
		if tuple.BlobState == store.IntentArchiveBlobAbsent {
			return store.IntentArchiveTupleResult{
				Reachable:   true,
				Disposition: store.IntentArchiveDispositionDanglingReference,
				Action:      store.IntentArchiveActionPurgeHash,
				Code:        store.IntentArchiveCodeBlobDangling,
				RepairClass: store.IntentArchiveRepairDanglingReference,
				ExitClass:   3,
			}, nil
		}
		return store.IntentArchiveTupleResult{
			Reachable:   true,
			Disposition: store.IntentArchiveDispositionHealthyRetained,
			Action:      store.IntentArchiveActionNone,
		}, nil
	case store.IntentArchiveWireRemovalPending:
		return store.IntentArchiveTupleResult{
			Reachable:   true,
			Disposition: store.IntentArchiveDispositionPendingFinalize,
			Action:      store.IntentArchiveActionRoutePendingOwner,
		}, nil
	case store.IntentArchiveWireTombstoned:
		switch tuple.BlobState {
		case store.IntentArchiveBlobAbsent:
			if tuple.Live {
				return store.IntentArchiveTupleResult{
					Reachable:   true,
					Disposition: store.IntentArchiveDispositionDanglingReference,
					Action:      store.IntentArchiveActionPurgeHash,
					Code:        store.IntentArchiveCodeBlobDangling,
					RepairClass: store.IntentArchiveRepairDanglingReference,
					ExitClass:   3,
				}, nil
			}
			return store.IntentArchiveTupleResult{
				Reachable:   true,
				Disposition: store.IntentArchiveDispositionHealthyPurged,
				Action:      store.IntentArchiveActionNone,
			}, nil
		case store.IntentArchiveBlobPresentCorrect:
			if tuple.Live {
				return store.IntentArchiveTupleResult{
					Reachable:   true,
					Disposition: store.IntentArchiveDispositionMixedReference,
					Action:      store.IntentArchiveActionPurgeHash,
					Code:        store.IntentArchiveCodeIndexStorageInconsistent,
					RepairClass: store.IntentArchiveRepairMixedReference,
					ExitClass:   3,
				}, nil
			}
			return store.IntentArchiveTupleResult{
				Reachable:   true,
				Disposition: store.IntentArchiveDispositionResidue,
				Action:      store.IntentArchiveActionPurgeOrphans,
				Code:        store.IntentArchiveCodeIndexStorageInconsistent,
				RepairClass: store.IntentArchiveRepairUnreferencedResidue,
				ExitClass:   3,
			}, nil
		}
	}
	return store.IntentArchiveTupleResult{}, fmt.Errorf("no expected classification for %+v", tuple)
}

func validateS7ASX11Classification(
	classify func(store.IntentArchiveTuple) store.IntentArchiveTupleResult,
) error {
	tuples := s7ASReachableTuples()
	if len(tuples) != 18 {
		return fmt.Errorf("reachable tuple count = %d, want 18", len(tuples))
	}
	for _, tuple := range tuples {
		want, err := s7ASExpectedTupleResult(tuple)
		if err != nil {
			return err
		}
		got := classify(tuple)
		if !reflect.DeepEqual(got, want) {
			return fmt.Errorf("tuple %+v classified as %+v, want %+v", tuple, got, want)
		}
	}
	return nil
}

func s7ASMutatedClassifier(
	mutate func(store.IntentArchiveTuple, store.IntentArchiveTupleResult) (store.IntentArchiveTupleResult, bool),
) (func(store.IntentArchiveTuple) store.IntentArchiveTupleResult, error) {
	changed := 0
	for _, tuple := range s7ASReachableTuples() {
		baseline := store.ClassifyIntentArchiveTuple(tuple)
		candidate, ok := mutate(tuple, baseline)
		if ok && !reflect.DeepEqual(candidate, baseline) {
			changed++
		}
	}
	if changed == 0 {
		return nil, fmt.Errorf("mutation left the reachable classifier unchanged")
	}
	return func(tuple store.IntentArchiveTuple) store.IntentArchiveTupleResult {
		baseline := store.ClassifyIntentArchiveTuple(tuple)
		candidate, ok := mutate(tuple, baseline)
		if ok {
			return candidate
		}
		return baseline
	}, nil
}

func TestS7ASX11ClassificationGuard(t *testing.T) {
	if err := validateS7ASX11Classification(store.ClassifyIntentArchiveTuple); err != nil {
		t.Fatal(err)
	}
}

func TestS7ASX11ClassificationGuardSensitivityTombstonedDivergent(t *testing.T) {
	classifier, err := s7ASMutatedClassifier(func(
		tuple store.IntentArchiveTuple,
		baseline store.IntentArchiveTupleResult,
	) (store.IntentArchiveTupleResult, bool) {
		if tuple.WireState == store.IntentArchiveWireTombstoned &&
			tuple.BlobState == store.IntentArchiveBlobPresentCorrect &&
			!tuple.Owned && !tuple.Live {
			baseline.Code = store.IntentArchiveCodePurgeEvidenceDivergent
			return baseline, true
		}
		return baseline, false
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateS7ASX11Classification(classifier); err == nil {
		t.Fatal("validator accepted tombstoned residue as archive-purge-evidence-divergent")
	}
}

func TestS7ASX11ClassificationGuardSensitivityCollapseTombstonedPresent(t *testing.T) {
	classifier, err := s7ASMutatedClassifier(func(
		tuple store.IntentArchiveTuple,
		baseline store.IntentArchiveTupleResult,
	) (store.IntentArchiveTupleResult, bool) {
		if tuple.WireState == store.IntentArchiveWireTombstoned &&
			tuple.BlobState == store.IntentArchiveBlobPresentCorrect &&
			!tuple.Owned {
			baseline.Disposition = store.IntentArchiveDispositionResidue
			baseline.Action = store.IntentArchiveActionPurgeOrphans
			baseline.Code = store.IntentArchiveCodeIndexStorageInconsistent
			baseline.RepairClass = store.IntentArchiveRepairUnreferencedResidue
			baseline.ExitClass = 3
			return baseline, true
		}
		return baseline, false
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateS7ASX11Classification(classifier); err == nil {
		t.Fatal("validator accepted live and unreferenced tombstoned-present tuples as the same route")
	}
}

func TestS7ASX11ClassificationGuardSensitivityDropOwnership(t *testing.T) {
	classifier, err := s7ASMutatedClassifier(func(
		tuple store.IntentArchiveTuple,
		_ store.IntentArchiveTupleResult,
	) (store.IntentArchiveTupleResult, bool) {
		if tuple.Owned {
			tuple.Owned = false
			return store.ClassifyIntentArchiveTuple(tuple), true
		}
		return store.IntentArchiveTupleResult{}, false
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateS7ASX11Classification(classifier); err == nil {
		t.Fatal("validator accepted owned tuples after the ownership dimension was dropped")
	}
}

func TestS7ASX11ClassificationGuardSensitivityCorruptAsResidue(t *testing.T) {
	classifier, err := s7ASMutatedClassifier(func(
		tuple store.IntentArchiveTuple,
		baseline store.IntentArchiveTupleResult,
	) (store.IntentArchiveTupleResult, bool) {
		if tuple.WireState == store.IntentArchiveWireTombstoned &&
			tuple.BlobState == store.IntentArchiveBlobUnidentifiable &&
			!tuple.Owned {
			baseline.Disposition = store.IntentArchiveDispositionResidue
			baseline.Action = store.IntentArchiveActionPurgeOrphans
			baseline.Code = store.IntentArchiveCodeIndexStorageInconsistent
			baseline.RepairClass = store.IntentArchiveRepairUnreferencedResidue
			baseline.ExitClass = 3
			return baseline, true
		}
		return baseline, false
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateS7ASX11Classification(classifier); err == nil {
		t.Fatal("validator accepted corrupt non-owned objects as residue")
	}
}

func TestS7ASX11ClassificationGuardSensitivityOwnedUnsafeExitThree(t *testing.T) {
	classifier, err := s7ASMutatedClassifier(func(
		tuple store.IntentArchiveTuple,
		baseline store.IntentArchiveTupleResult,
	) (store.IntentArchiveTupleResult, bool) {
		if tuple.Owned && tuple.BlobState == store.IntentArchiveBlobUnidentifiable {
			baseline.ExitClass = 3
			return baseline, true
		}
		return baseline, false
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateS7ASX11Classification(classifier); err == nil {
		t.Fatal("validator accepted rev-11 exit-3 semantics for owned unsafe blobs")
	}
}

func TestS7ASX11ClassificationGuardSensitivityUnchangedMutationFatal(t *testing.T) {
	if _, err := s7ASMutatedClassifier(func(
		_ store.IntentArchiveTuple,
		baseline store.IntentArchiveTupleResult,
	) (store.IntentArchiveTupleResult, bool) {
		return baseline, true
	}); err == nil {
		t.Fatal("unchanged mutation was not rejected")
	}
}

type s7ASCallGraph struct {
	functions        map[string]*ast.FuncDecl
	calls            map[string]map[string]bool
	recoverCall      map[string]int
	recoverAliasRefs int
	storeRecoverRefs int
}

func s7ASRecoverCallGraphSources(t *testing.T) map[string]string {
	t.Helper()
	return s7ARCLIPackageSources(s7ARProductionSourceSet(t))
}

func s7ASBuildCallGraph(sources map[string]string) (s7ASCallGraph, error) {
	graph := s7ASCallGraph{
		functions:   map[string]*ast.FuncDecl{},
		calls:       map[string]map[string]bool{},
		recoverCall: map[string]int{},
	}
	for rel, source := range sources {
		file, err := parser.ParseFile(token.NewFileSet(), rel, source, 0)
		if err != nil {
			return graph, err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.Ident:
				if typed.Name == "intentArchiveRecoverPurge" {
					graph.recoverAliasRefs++
				}
			case *ast.SelectorExpr:
				if typed.Sel.Name == "RecoverPendingPurge" {
					graph.storeRecoverRefs++
				}
			}
			return true
		})
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || function.Body == nil {
				continue
			}
			graph.functions[function.Name.Name] = function
			if graph.calls[function.Name.Name] == nil {
				graph.calls[function.Name.Name] = map[string]bool{}
			}
		}
	}
	for name, function := range graph.functions {
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			identifier, ok := call.Fun.(*ast.Ident)
			if ok {
				if identifier.Name == "intentArchiveRecoverPurge" {
					graph.recoverCall[name]++
					return true
				}
				if graph.functions[identifier.Name] != nil {
					graph.calls[name][identifier.Name] = true
				}
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if ok && selector.Sel.Name == "RecoverPendingPurge" {
				graph.recoverCall[name]++
			}
			return true
		})
	}
	return graph, nil
}

func s7ASDirectCallers(calls map[string]map[string]bool, callee string) []string {
	callers := []string{}
	for caller, targets := range calls {
		if targets[callee] {
			callers = append(callers, caller)
		}
	}
	sort.Strings(callers)
	return callers
}

func validateS7ASRecoverPendingPurgeCallGraph(sources map[string]string) error {
	graph, err := s7ASBuildCallGraph(sources)
	if err != nil {
		return err
	}
	callers := []string{}
	for caller, count := range graph.recoverCall {
		if count != 0 {
			callers = append(callers, caller)
		}
	}
	sort.Strings(callers)
	if !reflect.DeepEqual(callers, []string{"runFeatureIntentArchivePurgeConfirmed"}) {
		return fmt.Errorf("recover callers = %v, want [runFeatureIntentArchivePurgeConfirmed]", callers)
	}
	if graph.recoverCall["runFeatureIntentArchivePurgeConfirmed"] != 1 {
		return fmt.Errorf("recover call count = %d, want 1", graph.recoverCall["runFeatureIntentArchivePurgeConfirmed"])
	}
	if graph.recoverAliasRefs != 2 || graph.storeRecoverRefs != 1 {
		return fmt.Errorf(
			"recovery authority references = alias:%d store:%d, want alias:2 store:1",
			graph.recoverAliasRefs,
			graph.storeRecoverRefs,
		)
	}
	directCallers := s7ASDirectCallers(
		graph.calls, "runFeatureIntentArchivePurgeConfirmed",
	)
	if !reflect.DeepEqual(directCallers, []string{"runFeatureIntentArchivePurge"}) {
		return fmt.Errorf(
			"confirmed purge direct callers = %v, want [runFeatureIntentArchivePurge]",
			directCallers,
		)
	}
	if !graph.calls["runFeatureIntentArchivePurge"]["runFeatureIntentArchivePurgePreview"] ||
		!graph.calls["runFeatureIntentArchivePurge"]["runFeatureIntentArchivePurgeConfirmed"] {
		return fmt.Errorf("runFeatureIntentArchivePurge lost preview/confirmed branch reachability: %v", graph.calls["runFeatureIntentArchivePurge"])
	}
	if graph.recoverCall["runPreparePublish"] != 0 || graph.recoverCall["runFeatureIntentArchiveList"] != 0 || graph.recoverCall["runFeatureIntentArchivePurgePreview"] != 0 {
		return fmt.Errorf("recover leaked into prepare/list/preview callers: prepare=%d list=%d preview=%d",
			graph.recoverCall["runPreparePublish"],
			graph.recoverCall["runFeatureIntentArchiveList"],
			graph.recoverCall["runFeatureIntentArchivePurgePreview"],
		)
	}
	if err := validateS7ASConfirmedPurgeBranch(graph.functions["runFeatureIntentArchivePurge"]); err != nil {
		return err
	}
	return nil
}

func validateS7ASConfirmedPurgeBranch(function *ast.FuncDecl) error {
	if function == nil || function.Body == nil || len(function.Body.List) < 2 {
		return errors.New("runFeatureIntentArchivePurge body is missing")
	}
	branch, ok := function.Body.List[len(function.Body.List)-2].(*ast.IfStmt)
	if !ok || branch.Else != nil {
		return errors.New("purge preview/confirmed split is not the penultimate if without else")
	}
	negation, ok := branch.Cond.(*ast.UnaryExpr)
	if !ok || negation.Op != token.NOT {
		return errors.New("purge preview branch is not gated by negation")
	}
	selector, ok := negation.X.(*ast.SelectorExpr)
	if !ok {
		return errors.New("purge preview branch condition is not a selector")
	}
	receiver, receiverOK := selector.X.(*ast.Ident)
	if !receiverOK || receiver.Name != "options" || selector.Sel.Name != "yes" {
		return errors.New("purge preview branch is not gated by !options.yes")
	}
	if len(branch.Body.List) != 1 ||
		s7ASReturnedCallName(branch.Body.List[0]) != "runFeatureIntentArchivePurgePreview" ||
		s7ASReturnedCallName(function.Body.List[len(function.Body.List)-1]) !=
			"runFeatureIntentArchivePurgeConfirmed" {
		return errors.New("purge preview/confirmed calls are not the exact branch returns")
	}
	return nil
}

func s7ASReturnedCallName(statement ast.Stmt) string {
	returned, ok := statement.(*ast.ReturnStmt)
	if !ok || len(returned.Results) != 1 {
		return ""
	}
	call, ok := returned.Results[0].(*ast.CallExpr)
	if !ok {
		return ""
	}
	identifier, _ := call.Fun.(*ast.Ident)
	if identifier == nil {
		return ""
	}
	return identifier.Name
}

func TestS7ASRecoverPendingPurgeCallGraphGuard(t *testing.T) {
	if err := validateS7ASRecoverPendingPurgeCallGraph(s7ASRecoverCallGraphSources(t)); err != nil {
		t.Fatal(err)
	}
}

func TestS7ASRecoverPendingPurgeCallGraphGuardSensitivityPreparePublish(t *testing.T) {
	sources := s7ASRecoverCallGraphSources(t)
	old := "archiveStorage := newPrepareArchiveStorage(authority, nil)"
	new := old + "\n\t_, _ = intentArchiveRecoverPurge(archiveStorage, slug)"
	mutated := strings.Replace(sources["internal/cli/prepare_publish.go"], old, new, 1)
	if mutated == sources["internal/cli/prepare_publish.go"] {
		t.Fatal("prepare_publish mutation did not change the source")
	}
	sources["internal/cli/prepare_publish.go"] = mutated
	if err := validateS7ASRecoverPendingPurgeCallGraph(sources); err == nil {
		t.Fatal("call-graph validator accepted a prepare_publish recovery call")
	}
}

type s7ASSelectorTotalityObservation struct {
	selector           s7ASSelectorCase
	slug               string
	pendingHash        string
	selectorHash       string
	generationID       string
	blobRel            string
	indexRel           string
	previewCode        int
	previewStdout      string
	previewStderr      string
	previewReport      intentArchivePurgeReport
	previewHuman       string
	previewHumanErr    string
	previewAuthority   int
	previewWrites      int
	confirmedCode      int
	confirmedStdout    string
	confirmedStderr    string
	confirmedReport    intentArchivePurgeReport
	confirmedAuthority int
	confirmedRemoved   []string
	selectorPreserved  bool
}

func s7ASObserveSelectorTotality(t *testing.T, selector s7ASSelectorCase) s7ASSelectorTotalityObservation {
	t.Helper()
	root, slug := intentArchiveCLIWorkspace(t)
	pendingData := []byte("S7 AS selector pending\n")
	targetData := []byte("S7 AS selector retained target\n")
	orphanData := []byte("S7 AS selector orphan target\n")
	pending := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactAnalysis, pendingData,
		store.IntentArchiveWireRemovalPending,
	)
	target := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactSpec, targetData,
		store.IntentArchiveWireRetained,
	)
	orphan := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactExploration, orphanData,
		store.IntentArchiveWireTombstoned,
	)
	pendingGeneration := intentArchiveCLIGeneration(t, slug, pending)
	targetGeneration := intentArchiveCLIGeneration(t, slug, target)
	orphanGeneration := intentArchiveCLIGeneration(t, slug, orphan)
	writeIntentArchiveCLIFixture(
		t,
		root,
		slug,
		intentArchiveCLIIndex(
			t, slug, pendingGeneration, targetGeneration, orphanGeneration,
		),
		map[string][]byte{
			pending.ContentSHA256: pendingData,
			target.ContentSHA256:  targetData,
			orphan.ContentSHA256:  orphanData,
		},
	)
	pendingBlobRel, err := store.IntentArchiveBlobRel(slug, pending.ContentSHA256)
	if err != nil {
		t.Fatal(err)
	}
	targetBlobRel, err := store.IntentArchiveBlobRel(slug, target.ContentSHA256)
	if err != nil {
		t.Fatal(err)
	}
	orphanBlobRel, err := store.IntentArchiveBlobRel(slug, orphan.ContentSHA256)
	if err != nil {
		t.Fatal(err)
	}
	indexRel, err := store.IntentArchiveIndexRel(slug)
	if err != nil {
		t.Fatal(err)
	}
	selectorArgs := s7ASSelectorArgs(
		selector, target.ContentSHA256, targetGeneration.GenerationID,
	)
	sentinelRel := targetBlobRel
	sentinelBytes := targetData
	if selector.kind == store.IntentArchiveSelectorOrphans {
		sentinelRel = orphanBlobRel
		sentinelBytes = orphanData
	}
	previewAuthority := 0
	previewWrites := 0
	previousAcquire := intentArchiveAcquireAuthority
	previousFactory := intentArchiveNewStorage
	t.Cleanup(func() {
		intentArchiveAcquireAuthority = previousAcquire
		intentArchiveNewStorage = previousFactory
	})
	intentArchiveAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
		previewAuthority++
		return previousAcquire(path)
	}
	intentArchiveNewStorage = func(
		authority *intentlock.WorkspaceAuthority,
		rootFS *os.Root,
	) store.IntentArchiveStorage {
		return &intentArchiveWriteSpyStorage{
			IntentArchiveStorage: previousFactory(authority, rootFS),
			writes:               &previewWrites,
		}
	}
	previewCode, previewStdout, previewStderr, _ := runPrepare(
		t,
		s7ASPurgeArgs(root, slug, selectorArgs, false, true, true)...,
	)
	previewReport := decodeIntentArchivePurgeReport(t, previewStdout)
	humanCode, previewHuman, previewHumanErr, _ := runPrepare(
		t,
		s7ASPurgeArgs(root, slug, selectorArgs, false, false, false)...,
	)
	if humanCode != 0 {
		t.Fatalf("%s human preview = %d stderr=%q\n%s", selector.name, humanCode, previewHumanErr, previewHuman)
	}
	intentArchiveAcquireAuthority = previousAcquire
	intentArchiveNewStorage = previousFactory
	confirmedAuthority := 0
	confirmedRemoved := []string{}
	intentArchiveAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
		confirmedAuthority++
		return previousAcquire(path)
	}
	intentArchiveNewStorage = func(
		authority *intentlock.WorkspaceAuthority,
		rootFS *os.Root,
	) store.IntentArchiveStorage {
		return &s7ASRemoveSpyStorage{
			IntentArchiveStorage: previousFactory(authority, rootFS),
			removed:              &confirmedRemoved,
		}
	}
	confirmedCode, confirmedStdout, confirmedStderr, _ := runPrepare(
		t,
		s7ASPurgeArgs(root, slug, selectorArgs, true, true, true)...,
	)
	intentArchiveAcquireAuthority = previousAcquire
	intentArchiveNewStorage = previousFactory
	confirmedReport := decodeIntentArchivePurgeReport(t, confirmedStdout)
	sentinel, sentinelErr := os.ReadFile(
		filepath.Join(root, filepath.FromSlash(sentinelRel)),
	)
	selectorPreserved := sentinelErr == nil && bytes.Equal(sentinel, sentinelBytes)
	_, finalIndex := readIntentArchiveCLIIndex(t, root, slug)
	for _, generation := range finalIndex.Generations {
		for _, replacement := range generation.Replaced {
			if replacement.ContentSHA256 == target.ContentSHA256 &&
				replacement.WireState() != store.IntentArchiveWireRetained {
				selectorPreserved = false
			}
			if replacement.ContentSHA256 == orphan.ContentSHA256 &&
				replacement.WireState() != store.IntentArchiveWireTombstoned {
				selectorPreserved = false
			}
		}
	}
	return s7ASSelectorTotalityObservation{
		selector:           selector,
		slug:               slug,
		pendingHash:        pending.ContentSHA256,
		selectorHash:       target.ContentSHA256,
		generationID:       targetGeneration.GenerationID,
		blobRel:            pendingBlobRel,
		indexRel:           indexRel,
		previewCode:        previewCode,
		previewStdout:      previewStdout,
		previewStderr:      previewStderr,
		previewReport:      previewReport,
		previewHuman:       previewHuman,
		previewHumanErr:    previewHumanErr,
		previewAuthority:   previewAuthority,
		previewWrites:      previewWrites,
		confirmedCode:      confirmedCode,
		confirmedStdout:    confirmedStdout,
		confirmedStderr:    confirmedStderr,
		confirmedReport:    confirmedReport,
		confirmedAuthority: confirmedAuthority,
		confirmedRemoved:   confirmedRemoved,
		selectorPreserved:  selectorPreserved,
	}
}

func validateS7ASSelectorTotality(observations []s7ASSelectorTotalityObservation) error {
	if len(observations) != 4 {
		return fmt.Errorf("selector observations = %d, want 4", len(observations))
	}
	wantBlastRadius := "The --all selector tombstones every reference in every generation and removes every blob in this archive. The unconfirmed preview is the default; repeated --blob selectors are the narrower alternative."
	seen := map[store.IntentArchiveSelectorKind]bool{}
	for _, observation := range observations {
		if seen[observation.selector.kind] {
			return fmt.Errorf("duplicate selector observation for %s", observation.selector.kind)
		}
		seen[observation.selector.kind] = true
		selectorArgs := s7ASSelectorArgs(
			observation.selector, observation.selectorHash, observation.generationID,
		)
		wantJSONRetry := s7ASRenderedRetry(observation.slug, selectorArgs, true, true, true)
		wantHumanRetry := s7ASRenderedRetry(observation.slug, selectorArgs, true, false, false)
		wantJSONArgv := []string{"feature", "intent-archive", "purge", observation.slug}
		wantJSONArgv = append(wantJSONArgv, selectorArgs...)
		wantJSONArgv = append(wantJSONArgv, "--yes", "--json", "--quiet")
		if observation.previewCode != 0 || observation.previewStderr != "" ||
			observation.previewAuthority != 0 || observation.previewWrites != 0 {
			return fmt.Errorf("%s preview effects = exit:%d stderr:%q authority:%d writes:%d",
				observation.selector.name,
				observation.previewCode,
				observation.previewStderr,
				observation.previewAuthority,
				observation.previewWrites,
			)
		}
		if observation.previewReport.Outcome != string(store.IntentArchivePurgeRecoveryRequired) ||
			observation.previewReport.Action != "none" ||
			observation.previewReport.Selector != string(observation.selector.kind) ||
			observation.previewReport.PendingPurge == nil ||
			observation.previewReport.PendingPurge.Selector != string(observation.selector.kind) ||
			observation.previewReport.PendingPurge.Retry != wantJSONRetry ||
			observation.previewReport.PendingPurge.RetryCWD != store.IntentArchiveRepairCWD ||
			len(observation.previewReport.PendingPurge.PendingHashes) != 1 ||
			observation.previewReport.PendingPurge.PendingHashes[0].Hash != observation.pendingHash ||
			observation.previewReport.PendingPurge.PendingHashes[0].Blob != observation.blobRel ||
			observation.previewReport.PendingPurge.PendingHashes[0].Index != observation.indexRel ||
			observation.previewReport.PendingPurge.PendingHashes[0].Plan != intentArchivePendingPlan ||
			observation.previewReport.Refusal != nil || observation.previewReport.Recovery != nil ||
			observation.previewReport.PurgeProgress != nil || observation.previewReport.Divergence != nil ||
			len(observation.previewReport.Hashes) != 0 || len(observation.previewReport.GenerationIDs) != 0 ||
			len(observation.previewReport.References) != 0 || len(observation.previewReport.Blobs) != 0 ||
			len(observation.previewReport.OrphanBlobs) != 0 {
			return fmt.Errorf("%s preview report = %#v", observation.selector.name, observation.previewReport)
		}
		argv, err := s7APParseRenderedCommand(observation.previewReport.PendingPurge.Retry)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(argv, wantJSONArgv) {
			return fmt.Errorf("%s preview retry argv = %v, want %v", observation.selector.name, argv, wantJSONArgv)
		}
		wantHumanBlock := "    plan:  " + intentArchivePendingPlan + "\n" + prepareRetryHeader + "\n  " + wantHumanRetry + "\n"
		if observation.previewHumanErr != "" ||
			!strings.Contains(observation.previewHuman, "A previous purge stopped with pending references. Nothing was changed.\n") ||
			!strings.Contains(observation.previewHuman, "  pending hash: "+observation.pendingHash+"\n") ||
			!strings.Contains(observation.previewHuman, "    blob:  "+observation.blobRel+"\n") ||
			!strings.Contains(observation.previewHuman, "    index: "+observation.indexRel+"\n") ||
			!strings.Contains(observation.previewHuman, wantHumanBlock) ||
			strings.Contains(observation.previewHuman, "--path") {
			return fmt.Errorf("%s preview human = %q", observation.selector.name, observation.previewHuman)
		}
		if observation.confirmedCode != 0 || observation.confirmedStderr != "" || observation.confirmedAuthority != 1 {
			return fmt.Errorf("%s confirmed effects = exit:%d stderr:%q authority:%d",
				observation.selector.name,
				observation.confirmedCode,
				observation.confirmedStderr,
				observation.confirmedAuthority,
			)
		}
		if !reflect.DeepEqual(observation.confirmedRemoved, []string{observation.blobRel}) ||
			!observation.selectorPreserved {
			return fmt.Errorf(
				"%s confirmed processed selector: removed=%v pending=%s preserved=%t",
				observation.selector.name,
				observation.confirmedRemoved,
				observation.blobRel,
				observation.selectorPreserved,
			)
		}
		if observation.confirmedReport.Outcome != string(store.IntentArchivePurgeRecovered) ||
			observation.confirmedReport.Action != "none" ||
			observation.confirmedReport.Selector != string(observation.selector.kind) ||
			observation.confirmedReport.PendingPurge != nil ||
			observation.confirmedReport.Refusal != nil ||
			observation.confirmedReport.Recovery == nil ||
			observation.confirmedReport.Recovery.Kind != "archive-purge-finalize" ||
			!reflect.DeepEqual(observation.confirmedReport.Recovery.RestoredEntries, []string{}) ||
			!reflect.DeepEqual(observation.confirmedReport.Recovery.FinalizedHashes, []string{observation.pendingHash}) ||
			observation.confirmedReport.Recovery.Retry != wantJSONRetry ||
			observation.confirmedReport.Recovery.RetryCWD != store.IntentArchiveRepairCWD ||
			len(observation.confirmedReport.Hashes) != 0 || len(observation.confirmedReport.GenerationIDs) != 0 ||
			len(observation.confirmedReport.References) != 0 || len(observation.confirmedReport.Blobs) != 0 ||
			len(observation.confirmedReport.OrphanBlobs) != 0 {
			return fmt.Errorf("%s confirmed report = %#v", observation.selector.name, observation.confirmedReport)
		}
		confirmedArgv, err := s7APParseRenderedCommand(observation.confirmedReport.Recovery.Retry)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(confirmedArgv, wantJSONArgv) {
			return fmt.Errorf("%s confirmed retry argv = %v, want %v", observation.selector.name, confirmedArgv, wantJSONArgv)
		}
		if observation.selector.kind == store.IntentArchiveSelectorAll {
			if observation.previewReport.BlastRadius != wantBlastRadius {
				return fmt.Errorf("all preview blast radius = %q, want %q", observation.previewReport.BlastRadius, wantBlastRadius)
			}
			blastAt := strings.Index(observation.previewHuman, wantBlastRadius)
			headingAt := strings.Index(observation.previewHuman, "A previous purge stopped with pending references. Nothing was changed.")
			if blastAt < 0 || headingAt < 0 || blastAt > headingAt {
				return fmt.Errorf("all preview human lost blast-radius disclosure ordering")
			}
		}
	}
	if len(seen) != 4 {
		return fmt.Errorf("unique selector count = %d, want 4", len(seen))
	}
	return nil
}

func TestS7ASSelectorTotalityGuard(t *testing.T) {
	observations := make([]s7ASSelectorTotalityObservation, 0, len(s7ASSelectorCases()))
	for _, selector := range s7ASSelectorCases() {
		observations = append(observations, s7ASObserveSelectorTotality(t, selector))
	}
	if err := validateS7ASSelectorTotality(observations); err != nil {
		t.Fatal(err)
	}
}

func TestS7ASSelectorTotalityGuardSensitivityOrphansRetryWidened(t *testing.T) {
	observations := make([]s7ASSelectorTotalityObservation, 0, len(s7ASSelectorCases()))
	for _, selector := range s7ASSelectorCases() {
		observations = append(observations, s7ASObserveSelectorTotality(t, selector))
	}
	for index := range observations {
		if observations[index].selector.kind == store.IntentArchiveSelectorOrphans {
			observations[index].previewReport.PendingPurge.Retry = "tpatch feature intent-archive purge " + observations[index].slug + " --all --yes --json --quiet"
			break
		}
	}
	if err := validateS7ASSelectorTotality(observations); err == nil {
		t.Fatal("selector validator accepted an orphans preview widened to --all")
	}
}

func TestS7ASSelectorTotalityGuardSensitivityAllDisclosureMissing(t *testing.T) {
	observations := make([]s7ASSelectorTotalityObservation, 0, len(s7ASSelectorCases()))
	for _, selector := range s7ASSelectorCases() {
		observations = append(observations, s7ASObserveSelectorTotality(t, selector))
	}
	for index := range observations {
		if observations[index].selector.kind == store.IntentArchiveSelectorAll {
			observations[index].previewReport.BlastRadius = ""
			observations[index].previewHuman = strings.ReplaceAll(
				observations[index].previewHuman,
				"The --all selector tombstones every reference in every generation and removes every blob in this archive. The unconfirmed preview is the default; repeated --blob selectors are the narrower alternative.\n",
				"",
			)
			break
		}
	}
	if err := validateS7ASSelectorTotality(observations); err == nil {
		t.Fatal("selector validator accepted missing --all blast-radius disclosure")
	}
}
