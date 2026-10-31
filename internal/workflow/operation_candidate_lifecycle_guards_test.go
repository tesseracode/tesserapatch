package workflow

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func validateROCS5LifecycleSource(source string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "lifecycle.go", source, 0)
	if err != nil {
		return err
	}
	functions := map[string]*ast.FuncDecl{}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			functions[fn.Name.Name] = fn
		}
	}
	for name, ordered := range map[string][]string{
		"runOperationCandidatePreflight":  {"acquireOperationCandidateFeatureLock", "runOperationCandidatePreflightHeld"},
		"runReconcileCandidatePreflight":  {"acquireOperationCandidateFeatureLock", "runOperationCandidatePreflightHeld"},
		"runOperationCandidatePreflightHeld": {"borrowOperationCandidateFeatureMutation", "findOperationCandidateRecoveryJournal", "snapshotOperationCandidateBindings", "stalePendingOperationCandidate"},
		"prepareOperationCandidateHeld":   {"borrowOperationCandidateFeatureMutation", "operationCandidateActionPreflight", "assessOperationCandidateEligibility", "candidate.DescribeDerivation", "candidate.ComputeCandidateID"},
		"finalizeOperationCandidate":      {"acquireOperationCandidateFeatureLock", "finalizeOperationCandidateHeld"},
		"finalizeOperationCandidateHeld":  {"borrowOperationCandidateFeatureMutation", "prepareOperationCandidateHeld", "candidate.Execute", "candidate.WriteCandidateMetadata", "s.SaveFeatureStatus"},
		"rejectPendingOperationCandidate": {"operationCandidateActionPreflight", "pruneOperationCandidateIfPresent", "appendRejectedOperationCandidate", "reference.WriteCandidateState", "s.SaveFeatureStatus"},
	} {
		fn := functions[name]
		if fn == nil {
			return fmt.Errorf("missing lifecycle function %s", name)
		}
		calls := map[string]token.Pos{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				callee := rocS2Selector(call.Fun)
				if calls[callee] == 0 {
					calls[callee] = call.Pos()
				}
			}
			return true
		})
		var before token.Pos
		for _, required := range ordered {
			if calls[required] <= before {
				return fmt.Errorf("%s loses ordered authority %s", name, required)
			}
			before = calls[required]
		}
	}
	mints := 0
	for name, fn := range functions {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			literal, ok := n.(*ast.CompositeLit)
			if !ok || rocS2Selector(literal.Type) != "operationCandidatePrepared" || len(literal.Elts) == 0 {
				return true
			}
			mints++
			if name != "prepareOperationCandidateHeld" {
				err = fmt.Errorf("preparation minted outside eligibility owner")
			}
			return true
		})
	}
	if err != nil {
		return err
	}
	if mints != 1 {
		return fmt.Errorf("preparation must have exactly one mint site")
	}
	return nil
}

func TestROCS5LifecycleSourceGuardAndSensitivity(t *testing.T) {
	body, err := os.ReadFile("operation_candidate_lifecycle.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	if err := validateROCS5LifecycleSource(source); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range [][2]string{
		{"acquireOperationCandidateFeatureLock(s.Root, slug)", "skipLockAcquire(s.Root, slug)"},
		{"findOperationCandidateRecoveryJournal(reference, slug)", "skipRecovery(reference, slug)"},
		{"stalePendingOperationCandidate(s, slug, status, current, lock)", "skipStale(s, slug, status, current, lock)"},
		{"assessOperationCandidateEligibility(s, slug, invocation)", "trustCallerEligibility(s, slug, invocation)"},
		{"appendRejectedOperationCandidate(s.Root, slug, artifact, lock)", "skipRejection(s.Root, slug, artifact, lock)"},
		{"return finalizeOperationCandidateHeld(s, slug, expected, lock)", "return skipFinalizationHold(s, slug, expected, lock)"},
		{"return runOperationCandidatePreflightHeld(s, slug, current, lock, false)", "return skipPreflightHold(s, slug, current, lock, false)"},
	} {
		mutant := strings.Replace(source, mutation[0], mutation[1], 1)
		if mutant == source {
			t.Fatalf("mutation anchor missing: %s", mutation[0])
		}
		if err := validateROCS5LifecycleSource(mutant); err == nil {
			t.Fatalf("accepted authority mutation %s", mutation[0])
		}
	}
	mutant := source + "\nfunc forgedPreparation() operationCandidatePrepared { return operationCandidatePrepared{candidateID:\"rc_000000000000\"} }\n"
	if err := validateROCS5LifecycleSource(mutant); err == nil {
		t.Fatal("accepted additional preparation mint")
	}
}
