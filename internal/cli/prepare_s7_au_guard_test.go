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
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	_ "unsafe"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// ─── PIB-540 crash injection seams ────────────────────────────────────────────

//go:linkname s7AUAfterPurgeBlobRemove github.com/tesseracode/tesserapatch/internal/store.afterPurgeBlobRemove
var s7AUAfterPurgeBlobRemove func(string)

//go:linkname s7AUBeforePurgeIndexCAS github.com/tesseracode/tesserapatch/internal/store.beforePurgeIndexCAS
var s7AUBeforePurgeIndexCAS func(string)

const s7AUCrashExit = 97

func TestS7AUCrashFixtureHelper(t *testing.T) {
	if os.Getenv("TPATCH_S7_AU_CRASH_HELPER") != "1" {
		return
	}
	root := os.Getenv("TPATCH_S7_AU_CRASH_ROOT")
	slug := os.Getenv("TPATCH_S7_AU_CRASH_SLUG")
	hash := os.Getenv("TPATCH_S7_AU_CRASH_HASH")
	point := os.Getenv("TPATCH_S7_AU_CRASH_POINT")

	switch point {
	case "beforeClaimCAS":
		s7AUBeforePurgeIndexCAS = func(_ string) {
			os.Exit(s7AUCrashExit)
		}
	case "afterClaimBeforeRemove":
		s7APBeforePurgeBlobRemove = func(_ string) {
			os.Exit(s7AUCrashExit)
		}
	case "afterRemove":
		s7AUAfterPurgeBlobRemove = func(_ string) {
			os.Exit(s7AUCrashExit)
		}
	case "beforeTombstoneCAS":
		s7APBeforePendingTombstoneCAS = func(_ string) {
			os.Exit(s7AUCrashExit)
		}
	}

	command := buildRootCmd()
	command.SetOut(os.Stdout)
	command.SetErr(os.Stderr)
	args := []string{
		"--path", root,
		"feature", "intent-archive", "purge", slug,
		"--blob", hash, "--yes", "--json", "--quiet",
	}
	command.SetArgs(args)
	os.Exit(execute(command, os.Stderr))
}

func s7AURunCrashChild(t *testing.T, root, slug, hash, point string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestS7AUCrashFixtureHelper$")
	cmd.Env = append(os.Environ(),
		"TPATCH_S7_AU_CRASH_HELPER=1",
		"TPATCH_S7_AU_CRASH_ROOT="+root,
		"TPATCH_S7_AU_CRASH_SLUG="+slug,
		"TPATCH_S7_AU_CRASH_HASH="+hash,
		"TPATCH_S7_AU_CRASH_POINT="+point,
	)
	output, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != s7AUCrashExit {
		t.Fatalf("%s crash helper: err=%v output=%s", point, err, output)
	}
}

// ─── PIB-540 ──────────────────────────────────────────────────────────────────

func TestS7AUCrashInjectionContracts(t *testing.T) {
	s7AUInstallDeterministicProvider(t)

	type crashPoint struct {
		name       string
		nextAction string
	}
	points := []crashPoint{
		{name: "beforeClaimCAS", nextAction: "claim,remove,tombstone"},
		{name: "afterClaimBeforeRemove", nextAction: "remove,tombstone"},
		{name: "afterRemove", nextAction: "tombstone"},
		{name: "beforeTombstoneCAS", nextAction: "tombstone"},
	}

	for _, crash := range points {
		root, slug := intentArchiveCLIWorkspace(t)
		data := []byte("PIB-540 crash " + crash.name + "\n")
		retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained)
		pending := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, data, store.IntentArchiveWireRemovalPending)
		g1 := intentArchiveCLIGeneration(t, slug, retained)
		g2 := intentArchiveCLIGeneration(t, slug, pending)
		writeIntentArchiveCLIFixture(t, root, slug, intentArchiveCLIIndex(t, slug, g1, g2), map[string][]byte{retained.ContentSHA256: data})
		hash := retained.ContentSHA256
		blobRel, _ := store.IntentArchiveBlobRel(slug, hash)

		s7AURunCrashChild(t, root, slug, hash, crash.name)

		blobPresent := true
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(blobRel))); os.IsNotExist(err) {
			blobPresent = false
		}
		_, parsed := readIntentArchiveCLIIndex(t, root, slug)
		hasRetained := false
		for _, gen := range parsed.Generations {
			for _, repl := range gen.Replaced {
				if repl.ContentSHA256 == hash && repl.WireState() == store.IntentArchiveWireRetained {
					hasRetained = true
				}
			}
		}
		if !blobPresent && hasRetained {
			t.Fatalf("PIB-540 %s invariant violated: blob absent but retained reference exists", crash.name)
		}

		events := []string{}
		previousAfterCAS := s7APAfterPurgeIndexRename
		previousBeforeRemove := s7APBeforePurgeBlobRemove
		s7APAfterPurgeIndexRename = func(string) {
			if len(events) == 0 || events[len(events)-1] != "claim" {
				if crash.nextAction == "claim,remove,tombstone" && len(events) == 0 {
					events = append(events, "claim")
					return
				}
			}
			events = append(events, "tombstone")
		}
		s7APBeforePurgeBlobRemove = func(string) {
			events = append(events, "remove")
		}
		code, stdout, stderr := s7APRunFromWorkspace(t, root,
			[]string{"feature", "intent-archive", "purge", slug, "--blob", hash, "--yes", "--json", "--quiet"},
		)
		s7APAfterPurgeIndexRename = previousAfterCAS
		s7APBeforePurgeBlobRemove = previousBeforeRemove
		if code != 0 {
			t.Fatalf("PIB-540 %s rerun exit=%d stderr=%q\n%s", crash.name, code, stderr, stdout)
		}
		report := decodeIntentArchivePurgeReport(t, stdout)
		if report.Outcome != string(store.IntentArchivePurgeRecovered) {
			t.Fatalf("PIB-540 %s rerun outcome=%q, want recovered", crash.name, report.Outcome)
		}
		if strings.Join(events, ",") != crash.nextAction {
			t.Fatalf(
				"PIB-540 %s rerun actions=%v, want %s",
				crash.name, events, crash.nextAction,
			)
		}

		_, parsedAfter := readIntentArchiveCLIIndex(t, root, slug)
		for _, gen := range parsedAfter.Generations {
			for _, repl := range gen.Replaced {
				if repl.ContentSHA256 == hash && repl.WireState() != store.IntentArchiveWireTombstoned {
					t.Fatalf("PIB-540 %s rerun left ref in state %s", crash.name, repl.WireState())
				}
			}
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(blobRel))); !os.IsNotExist(err) {
			t.Fatalf("PIB-540 %s blob still present after rerun", crash.name)
		}
	}
}

// ─── PIB-544 ──────────────────────────────────────────────────────────────────

// s7AUInsertSameHashReference publishes an external index containing one extra
// retained reference to the same hash, exactly as the store's insertion-window
// fixtures do.
func s7AUInsertSameHashReference(t *testing.T, root, slug string, data []byte) {
	t.Helper()
	_, index := readIntentArchiveCLIIndex(t, root, slug)
	inserted := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactExploration, data, store.IntentArchiveWireRetained,
	)
	index.Generations = append(index.Generations, intentArchiveCLIGeneration(t, slug, inserted))
	encoded, err := store.EncodeIntentArchiveIndex(index)
	if err != nil {
		t.Fatal(err)
	}
	indexRel, err := store.IntentArchiveIndexRel(slug)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(indexRel)), encoded, 0o644); err != nil {
		t.Fatal(err)
	}
}

func s7AUSameHashReferenceStates(t *testing.T, root, slug, hash string) []store.IntentArchiveWireState {
	t.Helper()
	_, index := readIntentArchiveCLIIndex(t, root, slug)
	return s7ATWireStates(index, hash)
}

// s7AUClaimRereadSpyStorage inserts an external reference immediately before
// the claim's re-read — the CLI analogue of the store fixture's
// `before:capture-index` hook, which fires after the recovery preflight and
// before the per-hash capture.
type s7AUClaimRereadSpyStorage struct {
	store.IntentArchiveStorage
	preflighted bool
	inserted    bool
	insert      func()
}

func (spy *s7AUClaimRereadSpyStorage) PreflightIndexCAS(
	indexRel string,
	expected store.IntentArchiveIdentityToken,
) error {
	err := spy.IntentArchiveStorage.PreflightIndexCAS(indexRel, expected)
	if err == nil {
		spy.preflighted = true
	}
	return err
}

func (spy *s7AUClaimRereadSpyStorage) CaptureIndex(
	indexRel string,
) (store.IntentArchiveIndexCapture, error) {
	if spy.preflighted && !spy.inserted {
		spy.inserted = true
		spy.insert()
	}
	return spy.IntentArchiveStorage.CaptureIndex(indexRel)
}

func TestS7AUSameHashInsertionWindowContracts(t *testing.T) {
	s7AUInstallDeterministicProvider(t)

	// The four windows, their seams and their outcomes are the CLI composition
	// of the store's TestRecoverPendingPurgeInsertionWindowsPIB544 subtests, in
	// the same order. The fifth window — external replacement of the managed
	// object between the pre-removal revalidation and the unlink, reachable
	// through afterPurgeBlobRevalidate — is the disclosed final-syscall
	// residual that the PRD assigns to PIB-550, so it is not modelled here.
	windows := []struct {
		name        string
		arm         func(t *testing.T, insert func()) func()
		wantExit    int
		wantRemoved bool
	}{
		{
			name: "before-claim-reread",
			arm: func(t *testing.T, insert func()) func() {
				t.Helper()
				previous := intentArchiveNewStorage
				intentArchiveNewStorage = func(
					authority *intentlock.WorkspaceAuthority,
					rootFS *os.Root,
				) store.IntentArchiveStorage {
					return &s7AUClaimRereadSpyStorage{
						IntentArchiveStorage: previous(authority, rootFS),
						insert:               insert,
					}
				}
				return func() { intentArchiveNewStorage = previous }
			},
			wantExit:    0,
			wantRemoved: true,
		},
		{
			name: "between-reread-and-claim-CAS",
			arm: func(t *testing.T, insert func()) func() {
				t.Helper()
				previous := s7AUBeforePurgeIndexCAS
				fired := false
				s7AUBeforePurgeIndexCAS = func(string) {
					if fired {
						return
					}
					fired = true
					insert()
				}
				return func() { s7AUBeforePurgeIndexCAS = previous }
			},
			wantExit:    5,
			wantRemoved: false,
		},
		{
			name: "between-claim-CAS-and-revalidate",
			arm: func(t *testing.T, insert func()) func() {
				t.Helper()
				previous := s7APAfterPurgeIndexRename
				fired := false
				s7APAfterPurgeIndexRename = func(string) {
					if fired {
						return
					}
					fired = true
					insert()
				}
				return func() { s7APAfterPurgeIndexRename = previous }
			},
			wantExit:    5,
			wantRemoved: false,
		},
		{
			name: "after-removal-before-tombstone-CAS",
			arm: func(t *testing.T, insert func()) func() {
				t.Helper()
				previous := s7APBeforePendingTombstoneCAS
				fired := false
				s7APBeforePendingTombstoneCAS = func(string) {
					if fired {
						return
					}
					fired = true
					insert()
				}
				return func() { s7APBeforePendingTombstoneCAS = previous }
			},
			wantExit:    5,
			wantRemoved: true,
		},
	}

	for _, window := range windows {
		root, slug := intentArchiveCLIWorkspace(t)
		data := []byte("PIB-544 original " + window.name + "\n")
		retained := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireRetained)
		pending := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactSpec, data, store.IntentArchiveWireRemovalPending)
		g1 := intentArchiveCLIGeneration(t, slug, retained)
		g2 := intentArchiveCLIGeneration(t, slug, pending)
		writeIntentArchiveCLIFixture(t, root, slug, intentArchiveCLIIndex(t, slug, g1, g2), map[string][]byte{retained.ContentSHA256: data})
		hash := retained.ContentSHA256
		blobRel, _ := store.IntentArchiveBlobRel(slug, hash)
		blobPath := filepath.Join(root, filepath.FromSlash(blobRel))

		restore := window.arm(t, func() { s7AUInsertSameHashReference(t, root, slug, data) })

		argv := []string{"feature", "intent-archive", "purge", slug, "--blob", hash, "--yes", "--json", "--quiet"}
		code, stdout, stderr := s7APRunFromWorkspace(t, root, argv)
		restore()

		if code != window.wantExit {
			t.Fatalf("PIB-544 %s exit=%d, want %d stderr=%q\n%s", window.name, code, window.wantExit, stderr, stdout)
		}
		report := decodeIntentArchivePurgeReport(t, stdout)
		_, blobErr := os.Stat(blobPath)
		blobPresent := blobErr == nil
		if blobPresent == window.wantRemoved {
			t.Fatalf("PIB-544 %s blob present=%t, want removed=%t", window.name, blobPresent, window.wantRemoved)
		}

		states := s7AUSameHashReferenceStates(t, root, slug, hash)
		if !blobPresent && window.name != "after-removal-before-tombstone-CAS" {
			for _, state := range states {
				if state == store.IntentArchiveWireRetained {
					t.Fatalf("PIB-544 %s removed the blob with a retained reference visible: %v", window.name, states)
				}
			}
		}

		switch window.wantExit {
		case 0:
			if report.Outcome != string(store.IntentArchivePurgeRecovered) {
				t.Fatalf("PIB-544 %s outcome = %q, want recovered\n%s", window.name, report.Outcome, stdout)
			}
			if len(states) != 3 {
				t.Fatalf("PIB-544 %s same-hash references = %d, want the inserted one claimed too", window.name, len(states))
			}
			for _, state := range states {
				if state != store.IntentArchiveWireTombstoned {
					t.Fatalf("PIB-544 %s left reference in state %s", window.name, state)
				}
			}
		default:
			if report.Outcome != string(store.IntentArchivePurgePartial) ||
				report.PurgeProgress == nil ||
				report.PurgeProgress.PendingHash != hash ||
				report.PurgeProgress.Resume != string(store.IntentArchiveResumePendingRecoveryThenCompletion) ||
				report.PurgeProgress.RetryCWD != store.IntentArchiveRepairCWD {
				t.Fatalf("PIB-544 %s partial report = %#v\n%s", window.name, report, stdout)
			}
			if len(report.PurgeProgress.CompletedHashes) != 0 {
				t.Fatalf("PIB-544 %s completed hashes = %v, want none", window.name, report.PurgeProgress.CompletedHashes)
			}
			if strings.Contains(stdout, string(store.IntentArchiveCodePurgeEvidenceDivergent)) {
				t.Fatalf("PIB-544 %s reported divergence for an insertion window\n%s", window.name, stdout)
			}

			retryRemovals := []string{}
			previousFactory := intentArchiveNewStorage
			intentArchiveNewStorage = func(
				authority *intentlock.WorkspaceAuthority,
				rootFS *os.Root,
			) store.IntentArchiveStorage {
				return &s7ASRemoveSpyStorage{
					IntentArchiveStorage: previousFactory(authority, rootFS),
					removed:              &retryRemovals,
				}
			}
			retryCode, retryStdout, retryStderr := s7APRunFromWorkspace(t, root, argv)
			intentArchiveNewStorage = previousFactory
			if retryCode != 0 || retryStderr != "" {
				t.Fatalf("PIB-544 %s retry exit=%d stderr=%q\n%s", window.name, retryCode, retryStderr, retryStdout)
			}
			retryReport := decodeIntentArchivePurgeReport(t, retryStdout)
			if retryReport.Outcome != string(store.IntentArchivePurgeRecovered) {
				t.Fatalf("PIB-544 %s retry outcome = %q, want recovered\n%s", window.name, retryReport.Outcome, retryStdout)
			}
			if window.name == "after-removal-before-tombstone-CAS" && len(retryRemovals) != 0 {
				t.Fatalf("PIB-544 %s retry performed removals %v through the absent path",
					window.name, retryRemovals)
			}
			finalStates := s7AUSameHashReferenceStates(t, root, slug, hash)
			if len(finalStates) != 3 {
				t.Fatalf("PIB-544 %s retry same-hash references = %d, want the inserted one too", window.name, len(finalStates))
			}
			for _, state := range finalStates {
				if state != store.IntentArchiveWireTombstoned {
					t.Fatalf("PIB-544 %s retry left reference in state %s", window.name, state)
				}
			}
			if _, err := os.Stat(blobPath); !os.IsNotExist(err) {
				t.Fatalf("PIB-544 %s retry left the blob in place", window.name)
			}
		}
	}

	// Fifth fixture: the same insertion against a new selection, before its
	// first write, is exit 3 archive-purge-index-changed with a byte-identical
	// tree relative to the inserted index.
	root5, slug5 := intentArchiveCLIWorkspace(t)
	data5 := []byte("PIB-544 new selection\n")
	retained5 := intentArchiveCLIReplacement(t, store.IntentArchiveArtifactAnalysis, data5, store.IntentArchiveWireRetained)
	writeIntentArchiveCLIFixture(t, root5, slug5,
		intentArchiveCLIIndex(t, slug5, intentArchiveCLIGeneration(t, slug5, retained5)),
		map[string][]byte{retained5.ContentSHA256: data5},
	)
	hash5 := retained5.ContentSHA256

	var afterInsertion []byte
	previousCAS := s7AUBeforePurgeIndexCAS
	insertedFive := false
	s7AUBeforePurgeIndexCAS = func(string) {
		if insertedFive {
			return
		}
		insertedFive = true
		s7AUInsertSameHashReference(t, root5, slug5, data5)
		afterInsertion = readTree(t, filepath.Join(root5, ".tpatch"))
	}
	code5, stdout5, stderr5 := s7APRunFromWorkspace(t, root5,
		[]string{"feature", "intent-archive", "purge", slug5, "--blob", hash5, "--yes", "--json", "--quiet"},
	)
	s7AUBeforePurgeIndexCAS = previousCAS

	if !insertedFive {
		t.Fatal("PIB-544 new-selection fixture never reached its first index CAS")
	}
	if code5 != 3 {
		t.Fatalf("PIB-544 new-selection exit=%d, want 3 stderr=%q\n%s", code5, stderr5, stdout5)
	}
	report5 := decodeIntentArchivePurgeReport(t, stdout5)
	if report5.Refusal == nil ||
		report5.Refusal.Code != string(store.IntentArchiveCodePurgeIndexChanged) {
		t.Fatalf("PIB-544 new-selection refusal = %#v\n%s", report5.Refusal, stdout5)
	}
	if !bytes.Equal(afterInsertion, readTree(t, filepath.Join(root5, ".tpatch"))) {
		t.Fatal("PIB-544 new-selection wrote to the workspace after the external insertion")
	}
}

// ─── PIB-545 ──────────────────────────────────────────────────────────────────

// s7AUStoreAuthoritySource and s7AUCLIAuthoritySource are the two files that
// carry the archive's classification, claim and removal authority.
const (
	s7AUStoreAuthoritySource = "internal/store/intent_archive.go"
	s7AUCLIAuthoritySource   = "internal/cli/prepare_publish.go"
)

// s7AUAuthorityPredicates are the only total predicates that may authorize a
// managed blob removal, mapped to the property each one establishes.
var s7AUAuthorityPredicates = map[string]string{
	"allIntentArchiveReferencesPending": "all-pending",
	"intentArchiveHashUnreferenced":     "unreferenced",
}

// s7AUProgram is the parsed authority program: real source text plus its AST,
// so a mutated source is analysed exactly like the shipped one.
type s7AUProgram struct {
	fset  *token.FileSet
	order []string
	files map[string]*ast.File
	text  map[string]string
}

func s7AUParseProgram(sources map[string]string) (*s7AUProgram, error) {
	program := &s7AUProgram{
		fset:  token.NewFileSet(),
		order: []string{s7AUStoreAuthoritySource, s7AUCLIAuthoritySource},
		files: map[string]*ast.File{},
		text:  map[string]string{},
	}
	for _, name := range program.order {
		body, ok := sources[name]
		if !ok {
			return nil, fmt.Errorf("authority source %q is missing", name)
		}
		file, err := parser.ParseFile(program.fset, name, body, 0)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		program.files[name] = file
		program.text[name] = body
	}
	return program, nil
}

// nodeText returns the exact source text of a node.
func (program *s7AUProgram) nodeText(node ast.Node) string {
	position := program.fset.Position(node.Pos())
	end := program.fset.Position(node.End())
	body, ok := program.text[position.Filename]
	if !ok || position.Offset < 0 || end.Offset > len(body) || position.Offset > end.Offset {
		return ""
	}
	return body[position.Offset:end.Offset]
}

func (program *s7AUProgram) function(name string) *ast.FuncDecl {
	for _, source := range program.order {
		for _, declaration := range program.files[source].Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && function.Name.Name == name && function.Body != nil {
				return function
			}
		}
	}
	return nil
}

// eachFunction walks every function body in declaration order.
func (program *s7AUProgram) eachFunction(visit func(source string, function *ast.FuncDecl)) {
	for _, source := range program.order {
		for _, declaration := range program.files[source].Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Body != nil {
				visit(source, function)
			}
		}
	}
}

func s7AUCallName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		return fun.Sel.Name
	case *ast.Ident:
		return fun.Name
	default:
		return ""
	}
}

func s7AUContains(outer, inner ast.Node) bool {
	return inner.Pos() >= outer.Pos() && inner.End() <= outer.End()
}

// s7AUStatementBlocks returns every nested statement list of one statement.
func s7AUStatementBlocks(statement ast.Stmt) [][]ast.Stmt {
	var blocks [][]ast.Stmt
	switch typed := statement.(type) {
	case *ast.BlockStmt:
		blocks = append(blocks, typed.List)
	case *ast.IfStmt:
		if typed.Body != nil {
			blocks = append(blocks, typed.Body.List)
		}
		if typed.Else != nil {
			blocks = append(blocks, s7AUStatementBlocks(typed.Else)...)
		}
	case *ast.ForStmt:
		if typed.Body != nil {
			blocks = append(blocks, typed.Body.List)
		}
	case *ast.RangeStmt:
		if typed.Body != nil {
			blocks = append(blocks, typed.Body.List)
		}
	case *ast.SwitchStmt:
		for _, clause := range typed.Body.List {
			if caseClause, ok := clause.(*ast.CaseClause); ok {
				blocks = append(blocks, caseClause.Body)
			}
		}
	case *ast.TypeSwitchStmt:
		for _, clause := range typed.Body.List {
			if caseClause, ok := clause.(*ast.CaseClause); ok {
				blocks = append(blocks, caseClause.Body)
			}
		}
	case *ast.SelectStmt:
		for _, clause := range typed.Body.List {
			if commClause, ok := clause.(*ast.CommClause); ok {
				blocks = append(blocks, commClause.Body)
			}
		}
	case *ast.LabeledStmt:
		blocks = append(blocks, s7AUStatementBlocks(typed.Stmt)...)
	}
	return blocks
}

func s7AUTerminates(body *ast.BlockStmt) bool {
	if body == nil || len(body.List) == 0 {
		return false
	}
	switch body.List[len(body.List)-1].(type) {
	case *ast.ReturnStmt, *ast.BranchStmt:
		return true
	default:
		return false
	}
}

// s7AUAuthorityGuard reports the property a terminating `if !<authority>(…)`
// guard establishes, or "" when the statement is not such a guard.
func s7AUAuthorityGuard(statement ast.Stmt) string {
	ifStatement, ok := statement.(*ast.IfStmt)
	if !ok {
		return ""
	}
	unary, ok := ifStatement.Cond.(*ast.UnaryExpr)
	if !ok || unary.Op != token.NOT {
		return ""
	}
	call, ok := unary.X.(*ast.CallExpr)
	if !ok {
		return ""
	}
	property, ok := s7AUAuthorityPredicates[s7AUCallName(call)]
	if !ok || !s7AUTerminates(ifStatement.Body) {
		return ""
	}
	return property
}

// s7AUGuardDominates walks the function's statement lists and reports whether a
// terminating authority guard precedes target on every path that reaches it.
func s7AUGuardDominates(function *ast.FuncDecl, target ast.Node) bool {
	var walk func(list []ast.Stmt, guarded bool) bool
	walk = func(list []ast.Stmt, guarded bool) bool {
		for _, statement := range list {
			if s7AUContains(statement, target) {
				if guarded {
					return true
				}
				for _, nested := range s7AUStatementBlocks(statement) {
					if walk(nested, guarded) {
						return true
					}
				}
				return false
			}
			if s7AUAuthorityGuard(statement) != "" {
				guarded = true
			}
		}
		return false
	}
	return walk(function.Body.List, false)
}

// s7AUValidateClassificationMap derives the storage-classification map from
// ClassifyIntentArchiveTuple and asserts ownership outranks every other
// classification of the same hash.
func s7AUValidateClassificationMap(program *s7AUProgram) error {
	function := program.function("ClassifyIntentArchiveTuple")
	if function == nil {
		return errors.New("missing classification authority: ClassifyIntentArchiveTuple not found")
	}
	dispositions := map[string]token.Pos{}
	ast.Inspect(function.Body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
			return true
		}
		selector, ok := assignment.Lhs[0].(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Disposition" {
			return true
		}
		dispositions[program.nodeText(assignment.Rhs[0])] = assignment.Pos()
		return true
	})
	if len(dispositions) == 0 {
		return errors.New("missing classification: no disposition is assigned")
	}
	positionOf := func(suffix string) (token.Pos, bool) {
		for name, pos := range dispositions {
			if strings.HasSuffix(name, "IntentArchiveDisposition"+suffix) {
				return pos, true
			}
		}
		return token.NoPos, false
	}
	for _, required := range []string{
		"PendingRemove",
		"PendingFinalize",
		"CorruptObject",
		"DanglingReference",
		"MixedReference",
		"Residue",
		"HealthyRetained",
		"HealthyPurged",
	} {
		if _, ok := positionOf(required); !ok {
			return fmt.Errorf("missing classification %q; derived dispositions: %v", required, dispositions)
		}
	}

	var owned *ast.IfStmt
	for _, statement := range function.Body.List {
		ifStatement, ok := statement.(*ast.IfStmt)
		if ok && program.nodeText(ifStatement.Cond) == "tuple.Owned" {
			owned = ifStatement
			break
		}
	}
	if owned == nil {
		return errors.New("owned precedence: no ownership branch precedes the storage classification")
	}
	if !s7AUTerminates(owned.Body) {
		return errors.New("owned precedence: the ownership branch does not return before the non-owned map")
	}
	ownedText := program.nodeText(owned)
	for _, required := range []string{
		"IntentArchiveBlobUnidentifiable",
		"IntentArchiveBlobPresentCorrect",
		"IntentArchiveDispositionPendingRemove",
		"IntentArchiveActionRoutePendingOwner",
	} {
		if !strings.Contains(ownedText, required) {
			return fmt.Errorf("owned precedence: the ownership branch does not route %s", required)
		}
	}
	corrupt, ok := positionOf("CorruptObject")
	if !ok || corrupt < owned.End() {
		return errors.New("owned precedence: a corrupt classification precedes the ownership branch")
	}
	return nil
}

// s7AUValidateOwnedDivergentRoute asserts an owned unsafe-or-wrong blob maps
// only to exit-6 archive-purge-evidence-divergent.
func s7AUValidateOwnedDivergentRoute(program *s7AUProgram) error {
	function := program.function("intentArchiveUnidentifiablePurgeError")
	if function == nil {
		return errors.New("exit-6 route: intentArchiveUnidentifiablePurgeError not found")
	}
	var ownedBranch *ast.IfStmt
	for _, statement := range function.Body.List {
		ifStatement, ok := statement.(*ast.IfStmt)
		if ok && program.nodeText(ifStatement.Cond) == "owned" {
			ownedBranch = ifStatement
			break
		}
	}
	if ownedBranch == nil {
		return errors.New("exit-6 route: the owned arm of the unidentifiable route is missing")
	}
	branchText := program.nodeText(ownedBranch)
	if !strings.Contains(branchText, "IntentArchiveCodePurgeEvidenceDivergent") {
		return errors.New("exit-6 route: the owned unidentifiable arm does not emit archive-purge-evidence-divergent")
	}
	for _, forbidden := range []string{
		"IntentArchiveCodeBlobCorrupt",
		"IntentArchiveCodeIndexStorageInconsistent",
		"IntentArchiveCodeBlobDangling",
	} {
		if strings.Contains(branchText, forbidden) {
			return fmt.Errorf("exit-6 route: the owned unidentifiable arm reaches the exit-3 code %s", forbidden)
		}
	}
	exitSix := false
	ast.Inspect(ownedBranch, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || s7AUCallName(call) != "intentArchiveError" || len(call.Args) == 0 {
			return true
		}
		if program.nodeText(call.Args[len(call.Args)-1]) == "6" {
			exitSix = true
		}
		return true
	})
	if !exitSix {
		return errors.New("exit-6 route: the owned unidentifiable arm does not carry exit class 6")
	}
	return nil
}

// s7AUHashStateSkip returns the inner skip condition of
// setIntentArchiveHashState, which decides the claim's reference set.
func s7AUHashStateSkip(program *s7AUProgram) (*ast.IfStmt, error) {
	function := program.function("setIntentArchiveHashState")
	if function == nil {
		return nil, errors.New("setIntentArchiveHashState not found")
	}
	var outer *ast.RangeStmt
	for _, statement := range function.Body.List {
		rangeStatement, ok := statement.(*ast.RangeStmt)
		if ok && strings.Contains(program.nodeText(rangeStatement.X), "Generations") {
			outer = rangeStatement
			break
		}
	}
	if outer == nil {
		return nil, errors.New("setIntentArchiveHashState does not iterate every generation")
	}
	var inner *ast.RangeStmt
	for _, statement := range outer.Body.List {
		rangeStatement, ok := statement.(*ast.RangeStmt)
		if ok && strings.Contains(program.nodeText(rangeStatement.X), "Replaced") {
			inner = rangeStatement
			break
		}
	}
	if inner == nil {
		return nil, errors.New("setIntentArchiveHashState does not iterate every replacement")
	}
	for _, statement := range inner.Body.List {
		ifStatement, ok := statement.(*ast.IfStmt)
		if !ok || !s7AUTerminates(ifStatement.Body) {
			continue
		}
		return ifStatement, nil
	}
	return nil, errors.New("setIntentArchiveHashState has no reference skip condition")
}

// s7AUValidateClaimTotality asserts the claim CAS covers every reference with
// the target hash, tombstoned references included.
func s7AUValidateClaimTotality(program *s7AUProgram) error {
	skip, err := s7AUHashStateSkip(program)
	if err != nil {
		return fmt.Errorf("claim CAS: %w", err)
	}
	condition := program.nodeText(skip.Cond)
	if strings.Contains(condition, "IntentArchiveWireTombstoned") {
		return fmt.Errorf("claim CAS: the claim skips already-tombstoned references: %s", condition)
	}
	execute := program.function("executeIntentArchivePurgeHash")
	if execute == nil {
		return errors.New("claim CAS: executeIntentArchivePurgeHash not found")
	}
	claimed := false
	ast.Inspect(execute.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || s7AUCallName(call) != "setIntentArchiveHashState" || len(call.Args) != 3 {
			return true
		}
		if program.nodeText(call.Args[2]) == "IntentArchiveWireRemovalPending" &&
			program.nodeText(call.Args[1]) == "hash" {
			claimed = true
		}
		return true
	})
	if !claimed {
		return errors.New("claim CAS: no whole-index removal-pending claim is published for the hash")
	}
	return nil
}

// s7AUValidateAbsentPathTotality asserts the absent-blob path tombstones every
// reference to the hash rather than only the pending ones.
func s7AUValidateAbsentPathTotality(program *s7AUProgram) error {
	skip, err := s7AUHashStateSkip(program)
	if err != nil {
		return fmt.Errorf("absent-blob: %w", err)
	}
	condition := program.nodeText(skip.Cond)
	for _, forbidden := range []string{"IntentArchiveWireRetained", "IntentArchiveWireRemovalPending"} {
		if strings.Contains(condition, forbidden) {
			return fmt.Errorf("absent-blob: the tombstone rewrite skips %s references: %s", forbidden, condition)
		}
	}
	absent := program.function("executeIntentArchiveAbsentTombstone")
	if absent == nil {
		return errors.New("absent-blob: executeIntentArchiveAbsentTombstone not found")
	}
	tombstonesAll := false
	ast.Inspect(absent.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || s7AUCallName(call) != "setIntentArchiveHashState" || len(call.Args) != 3 {
			return true
		}
		if program.nodeText(call.Args[2]) == "IntentArchiveWireTombstoned" &&
			program.nodeText(call.Args[1]) == "hash" {
			tombstonesAll = true
		}
		return true
	})
	if !tombstonesAll {
		return errors.New("absent-blob: the absent path does not tombstone every same-hash reference")
	}
	return nil
}

// s7AUAuthorizationParameter finds the boolean parameter whose negation
// terminates the removal wrapper before it can reach the syscall.
func s7AUAuthorizationParameter(function *ast.FuncDecl, removals []ast.Node) (int, error) {
	index := 0
	for _, field := range function.Type.Params.List {
		typeIdentifier, isIdentifier := field.Type.(*ast.Ident)
		names := field.Names
		if len(names) == 0 {
			names = []*ast.Ident{ast.NewIdent("_")}
		}
		for _, name := range names {
			if isIdentifier && typeIdentifier.Name == "bool" {
				gated := false
				for _, statement := range function.Body.List {
					ifStatement, ok := statement.(*ast.IfStmt)
					if !ok {
						continue
					}
					unary, ok := ifStatement.Cond.(*ast.UnaryExpr)
					if !ok || unary.Op != token.NOT {
						continue
					}
					operand, ok := unary.X.(*ast.Ident)
					if !ok || operand.Name != name.Name || !s7AUTerminates(ifStatement.Body) {
						continue
					}
					gated = true
					for _, removal := range removals {
						if removal.Pos() < ifStatement.End() {
							gated = false
						}
					}
				}
				if gated {
					return index, nil
				}
			}
			index++
		}
	}
	return -1, fmt.Errorf(
		"%s reaches the removal syscall without an all-pending authorization parameter",
		function.Name.Name,
	)
}

// s7AUValidateRemovalAuthority walks the removal call graph and asserts every
// managed blob removal is dominated by a total all-pending or unreferenced
// authority check, wherever that check lives.
func s7AUValidateRemovalAuthority(program *s7AUProgram) error {
	predicates := make([]string, 0, len(s7AUAuthorityPredicates))
	for predicate := range s7AUAuthorityPredicates {
		predicates = append(predicates, predicate)
	}
	sort.Strings(predicates)
	for _, predicate := range predicates {
		function := program.function(predicate)
		if function == nil {
			return fmt.Errorf("all-pending authority predicate %s is missing", predicate)
		}
		body := program.nodeText(function.Body)
		if !strings.Contains(body, "Generations") || !strings.Contains(body, "Replaced") ||
			!strings.Contains(body, "return false") {
			return fmt.Errorf("all-pending authority predicate %s is not total over the index", predicate)
		}
	}

	wrappers := map[string][]ast.Node{}
	var syscallError error
	program.eachFunction(func(source string, function *ast.FuncDecl) {
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || s7AUCallName(call) != "RemoveBlob" {
				return true
			}
			if source != s7AUStoreAuthoritySource {
				syscallError = fmt.Errorf(
					"blob removal call site %s is outside the store authority (%s)",
					function.Name.Name, source,
				)
				return false
			}
			wrappers[function.Name.Name] = append(wrappers[function.Name.Name], call)
			return true
		})
	})
	if syscallError != nil {
		return syscallError
	}
	if len(wrappers) == 0 {
		return errors.New("no blob removal call site found — RemoveBlob is never called")
	}

	names := make([]string, 0, len(wrappers))
	for name := range wrappers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		wrapper := program.function(name)
		if wrapper == nil {
			return fmt.Errorf("removal wrapper %s could not be resolved", name)
		}
		parameter, err := s7AUAuthorizationParameter(wrapper, wrappers[name])
		if err != nil {
			return err
		}
		sites := 0
		var siteError error
		program.eachFunction(func(_ string, caller *ast.FuncDecl) {
			ast.Inspect(caller.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || s7AUCallName(call) != name || caller.Name.Name == name {
					return true
				}
				sites++
				if parameter >= len(call.Args) {
					siteError = fmt.Errorf(
						"blob removal in %s passes no all-pending authorization argument",
						caller.Name.Name,
					)
					return false
				}
				if err := s7AUAuthorizedArgument(program, caller, call, call.Args[parameter]); err != nil {
					siteError = err
					return false
				}
				return true
			})
		})
		if siteError != nil {
			return siteError
		}
		if sites == 0 {
			return fmt.Errorf("removal wrapper %s has no call site to authorize", name)
		}
	}
	return nil
}

// s7AUAuthorizedArgument resolves one authorization argument to a total
// authority predicate, either directly, through its binding, or through a
// dominating guard when the literal true is passed.
func s7AUAuthorizedArgument(
	program *s7AUProgram,
	caller *ast.FuncDecl,
	call *ast.CallExpr,
	argument ast.Expr,
) error {
	failure := fmt.Errorf(
		"blob removal in %s is not dominated by an all-pending or unreferenced authority: %s",
		caller.Name.Name, program.nodeText(argument),
	)
	switch typed := argument.(type) {
	case *ast.CallExpr:
		if _, ok := s7AUAuthorityPredicates[s7AUCallName(typed)]; ok {
			return nil
		}
		return failure
	case *ast.Ident:
		if typed.Name == "true" {
			if s7AUGuardDominates(caller, call) {
				return nil
			}
			return failure
		}
		bindings := 0
		bound := true
		ast.Inspect(caller.Body, func(node ast.Node) bool {
			assignment, ok := node.(*ast.AssignStmt)
			if !ok || assignment.Pos() > call.Pos() || len(assignment.Lhs) != len(assignment.Rhs) {
				return true
			}
			for index, left := range assignment.Lhs {
				identifier, ok := left.(*ast.Ident)
				if !ok || identifier.Name != typed.Name {
					continue
				}
				bindings++
				value, ok := assignment.Rhs[index].(*ast.CallExpr)
				if !ok {
					bound = false
					continue
				}
				if _, ok := s7AUAuthorityPredicates[s7AUCallName(value)]; !ok {
					bound = false
				}
			}
			return true
		})
		if bindings == 0 || !bound {
			if s7AUGuardDominates(caller, call) {
				return nil
			}
			return failure
		}
		return nil
	default:
		return failure
	}
}

// s7AUValidateArchiveAuthority is the single combined validator: the derived
// classification map, the owned exit-6 route, the claim's reference set, the
// absent-blob path and every blob removal's authority.
func s7AUValidateArchiveAuthority(sources map[string]string) error {
	program, err := s7AUParseProgram(sources)
	if err != nil {
		return err
	}
	if err := s7AUValidateClassificationMap(program); err != nil {
		return err
	}
	if err := s7AUValidateOwnedDivergentRoute(program); err != nil {
		return err
	}
	if err := s7AUValidateClaimTotality(program); err != nil {
		return err
	}
	if err := s7AUValidateAbsentPathTotality(program); err != nil {
		return err
	}
	return s7AUValidateRemovalAuthority(program)
}

func TestS7AUClassificationAuthorityGuard(t *testing.T) {
	sources := map[string]string{
		s7AUStoreAuthoritySource: s6RepoFile(t, s7AUStoreAuthoritySource),
		s7AUCLIAuthoritySource:   s6RepoFile(t, s7AUCLIAuthoritySource),
	}

	if err := s7AUValidateArchiveAuthority(sources); err != nil {
		t.Fatalf("PIB-545 baseline validation failed: %v", err)
	}

	fixtures := []struct {
		name      string
		source    string
		old       string
		new       string
		count     int
		wantClass string
	}{
		{
			name:      "owned-hash-classified-as-mixed",
			source:    s7AUStoreAuthoritySource,
			old:       "result.Disposition = IntentArchiveDispositionPendingRemove",
			new:       "result.Disposition = IntentArchiveDispositionMixedReference",
			count:     1,
			wantClass: "missing classification",
		},
		{
			name:      "removal-with-surviving-retained-reference",
			source:    s7AUStoreAuthoritySource,
			old:       "if !allIntentArchiveReferencesPending(revalidatedIndex, hash) {",
			new:       "if !intentArchiveHashHasState(revalidatedIndex, hash, IntentArchiveWireRemovalPending) {",
			count:     -1,
			wantClass: "blob removal in executeIntentArchivePurgeHash",
		},
		{
			name:      "hash-wrong-non-owned-as-dangling",
			source:    s7AUStoreAuthoritySource,
			old:       "result.Disposition = IntentArchiveDispositionCorruptObject",
			new:       "result.Disposition = IntentArchiveDispositionDanglingReference",
			count:     1,
			wantClass: "missing classification",
		},
		{
			name:      "orphans-removes-a-mixed-hash-blob",
			source:    s7AUStoreAuthoritySource,
			old:       "authorized := intentArchiveHashUnreferenced(snapshot.Index, observation.Hash)",
			new:       "authorized := true",
			count:     1,
			wantClass: "blob removal in executeIntentArchiveOrphanPurge",
		},
		{
			name:      "owned-unsafe-blob-exit-3-not-exit-6",
			source:    s7AUStoreAuthoritySource,
			old:       `intentArchiveError(IntentArchiveCodePurgeEvidenceDivergent, "the owned blob is present but unidentifiable", 6)`,
			new:       `intentArchiveError(IntentArchiveCodeBlobCorrupt, "the owned blob is present but unidentifiable", 3)`,
			count:     1,
			wantClass: "exit-6 route",
		},
		{
			name:   "claim-skips-tombstoned-references",
			source: s7AUStoreAuthoritySource,
			old:    "if replacement.ContentSHA256 != hash || replacement.WireState() == state {",
			new: "if replacement.ContentSHA256 != hash || replacement.WireState() == state ||" +
				" replacement.WireState() == IntentArchiveWireTombstoned {",
			count:     1,
			wantClass: "claim CAS",
		},
		{
			name:   "absent-blob-path-leaves-a-retained-reference",
			source: s7AUStoreAuthoritySource,
			old:    "if replacement.ContentSHA256 != hash || replacement.WireState() == state {",
			new: "if replacement.ContentSHA256 != hash || replacement.WireState() == state ||" +
				" replacement.WireState() == IntentArchiveWireRetained {",
			count:     1,
			wantClass: "absent-blob",
		},
	}

	for _, fixture := range fixtures {
		mutated := map[string]string{}
		for name, body := range sources {
			mutated[name] = body
		}
		replaced := strings.Replace(mutated[fixture.source], fixture.old, fixture.new, fixture.count)
		if replaced == mutated[fixture.source] {
			t.Fatalf("PIB-545 sensitivity fixture %q changed nothing", fixture.name)
		}
		mutated[fixture.source] = replaced

		err := s7AUValidateArchiveAuthority(mutated)
		if err == nil {
			t.Fatalf("PIB-545 sensitivity fixture %q was accepted by the authority validator", fixture.name)
		}
		if !strings.Contains(err.Error(), fixture.wantClass) {
			t.Fatalf("PIB-545 sensitivity fixture %q: want error class %q, got: %v",
				fixture.name, fixture.wantClass, err)
		}
	}

	if err := s7AUValidateArchiveAuthority(sources); err != nil {
		t.Fatalf("PIB-545 unmutated authority was rejected after sensitivity: %v", err)
	}
}
