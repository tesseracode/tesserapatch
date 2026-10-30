package workflow

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func rocS2Selector(expr ast.Expr) string {
	switch node := expr.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.SelectorExpr:
		return rocS2Selector(node.X) + "." + node.Sel.Name
	case *ast.IndexExpr:
		return rocS2Selector(node.X) + "[]"
	}
	return ""
}

func rocS2EligibilitySourceGuard(src string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "eligibility.go", src, 0)
	if err != nil {
		return err
	}
	methods := map[string]*ast.FuncDecl{}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			methods[fn.Name.Name] = fn
		}
	}
	runner := methods["assessOperationCandidateEligibility"]
	if runner == nil {
		return fmt.Errorf("eligibility runner missing")
	}
	want := []string{"c.coverageUsable", "c.coverageOwner", "c.coverageComplete", "c.captureEvent",
		"c.bindingPresence", "c.bindingHashes", "c.recipeDecodable", "c.recipeOwner",
		"c.references", "c.effectBindings", "c.recipeWitnesses", "c.supersession",
		"c.parents", "c.invocationMode", "c.shadow"}
	var got []string
	var sequence *ast.RangeStmt
	eligibleWrites, proofArrays := 0, 0
	reasons := map[string]bool{}
	for _, statement := range runner.Body.List {
		if assign, ok := statement.(*ast.AssignStmt); ok {
			for i, lhs := range assign.Lhs {
				if rocS2Selector(lhs) == "gates" {
					literal, ok := assign.Rhs[i].(*ast.CompositeLit)
					if !ok {
						return fmt.Errorf("gate inventory is not literal")
					}
					kind, ok := literal.Type.(*ast.ArrayType)
					if !ok || rocS2Selector(kind.Len) != "" {
						return fmt.Errorf("gate inventory is not fixed")
					}
					size, ok := kind.Len.(*ast.BasicLit)
					if !ok || size.Value != "15" {
						return fmt.Errorf("gate count changed")
					}
					for _, item := range literal.Elts {
						got = append(got, rocS2Selector(item))
					}
				}
			}
		}
		if loop, ok := statement.(*ast.RangeStmt); ok && rocS2Selector(loop.X) == "gates" {
			sequence = loop
		}
	}
	if !reflect.DeepEqual(got, want) || sequence == nil {
		return fmt.Errorf("gate inventory/order changed")
	}
	if len(sequence.Body.List) != 3 {
		return fmt.Errorf("gate runner no longer first-match")
	}
	refusal, ok := sequence.Body.List[1].(*ast.IfStmt)
	if !ok || refusal.Else != nil || len(refusal.Body.List) != 1 {
		return fmt.Errorf("gate refusal must return immediately")
	}
	negation, ok := refusal.Cond.(*ast.UnaryExpr)
	if !ok || negation.Op != token.NOT {
		return fmt.Errorf("gate failure is not tested")
	}
	call, ok := negation.X.(*ast.CallExpr)
	if !ok || rocS2Selector(call.Fun) != "gate" || len(call.Args) != 0 {
		return fmt.Errorf("gate not evaluated")
	}
	if _, ok := refusal.Body.List[0].(*ast.ReturnStmt); !ok {
		return fmt.Errorf("gate failure falls through")
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		ast.Inspect(fn, func(node ast.Node) bool {
			if err != nil {
				return false
			}
			switch item := node.(type) {
			case *ast.AssignStmt:
				for _, lhs := range item.Lhs {
					name := rocS2Selector(lhs)
					if name == "c.result.eligible" {
						eligibleWrites++
						if fn != runner || sequence.End() >= item.Pos() {
							err = fmt.Errorf("eligibility granted before all gates")
						}
					}
					for field, owner := range map[string]string{
						"c.coverage": "coverageUsable", "c.event": "captureEvent",
						"c.recipe": "recipeDecodable", "c.observation": "references",
					} {
						if name == field || strings.HasPrefix(name, field+".") || strings.HasPrefix(name, field+"[") {
							if name != field || fn.Name.Name != owner {
								err = fmt.Errorf("validated input mutated outside its owner")
							}
						}
					}
				}
			case *ast.CallExpr:
				name := rocS2Selector(item.Fun)
				if name == "c.refuse" {
					if len(item.Args) != 2 {
						err = fmt.Errorf("invalid refusal")
						return false
					}
					literal, ok := item.Args[0].(*ast.BasicLit)
					if !ok || literal.Kind != token.STRING {
						err = fmt.Errorf("dynamic refusal vocabulary")
						return false
					}
					reason, unquoteErr := strconv.Unquote(literal.Value)
					if unquoteErr != nil {
						err = unquoteErr
						return false
					}
					reasons[reason] = true
				}
				for _, forbidden := range []string{
					"json.Unmarshal", "json.NewDecoder", "AssessRecipeCoverage", "ValidateRecipeCaptureEventPair",
					"ExecuteRecipe", "evaluateRecipeOperations", "dryRunOperation", "executeOperation",
					"candidate.CreateCandidateRoot", "SaveFeatureStatus", "WriteArtifact", "RunVerify",
				} {
					if name == forbidden || strings.HasSuffix(name, "."+forbidden) {
						err = fmt.Errorf("forbidden eligibility authority: %s", name)
					}
				}
			}
			return true
		})
	}
	ast.Inspect(file, func(node ast.Node) bool {
		field, ok := node.(*ast.Field)
		if !ok {
			return true
		}
		for _, name := range field.Names {
			if name.Name == "gatesPassed" {
				kind, ok := field.Type.(*ast.ArrayType)
				if !ok {
					err = fmt.Errorf("proof is not fixed")
					continue
				}
				size, ok := kind.Len.(*ast.BasicLit)
				if !ok || size.Value != "15" || rocS2Selector(kind.Elt) != "bool" {
					err = fmt.Errorf("proof cardinality changed")
				}
				proofArrays++
			}
		}
		return true
	})
	if err != nil {
		return err
	}
	if eligibleWrites != 1 || proofArrays != 1 {
		return fmt.Errorf("authority/proof is not unique")
	}
	for _, reason := range []string{
		"candidate-feature-state-unsupported", "coverage-unusable", "coverage-owner-mismatch", "coverage-incomplete",
		"capture-event-unusable", "capture-event-owner-mismatch", "capture-event-pair-mismatch",
		"binding-presence-drift", "binding-hash-drift", "recipe-undecodable", "recipe-owner-mismatch",
		"reference-not-reconstructable", "effect-binding-drift", "recipe-unsupported", "recipe-stale-marker-present",
		"supersession-not-authority", "parent-blocked", "derivation-mode-unsupported", "shadow-pending",
	} {
		if !reasons[reason] {
			return fmt.Errorf("missing refusal %s", reason)
		}
		delete(reasons, reason)
	}
	if len(reasons) != 0 {
		return fmt.Errorf("unregistered refusal vocabulary")
	}
	for method, required := range map[string][]string{
		"coverageUsable":     {"DecodeRecipeCoverage"},
		"captureEvent":       {"DecodeRecipeCaptureEvent", "RecipeCaptureEventIdentity"},
		"recipeDecodable":    {"decodeCoverageRecipe"},
		"references":         {"candidate.OpenReferenceRoot", "base.ReconstructPatch", "upstream.CheckTree", "patchobs.PreimageSetDigest", "reconstructedObservation"},
		"witnessesSupported": {"coverageParents", "coverageAssignments", "simulateCoverage"},
	} {
		fn := methods[method]
		if fn == nil {
			return fmt.Errorf("missing gate method %s", method)
		}
		calls := map[string]bool{}
		ast.Inspect(fn, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				calls[rocS2Selector(call.Fun)] = true
				if rocS2Selector(call.Fun) == "reconstructedObservation" &&
					(len(call.Args) != 4 || rocS2Selector(call.Args[2]) != "c.event") {
					err = fmt.Errorf("reconstruction substitutes for captured event")
				}
			}
			return true
		})
		for _, name := range required {
			if !calls[name] {
				return fmt.Errorf("%s lost authority %s", method, name)
			}
		}
	}
	if err != nil {
		return err
	}
	for _, imported := range file.Imports {
		path, _ := strconv.Unquote(imported.Path.Value)
		if path == "encoding/json" {
			return fmt.Errorf("private decoder imported")
		}
	}
	return nil
}

func TestROCS2EligibilitySourceGuardSensitivity(t *testing.T) {
	source := rgaS0ReadRepoFile(t, "internal/workflow/operation_candidate_eligibility.go")
	if err := rocS2EligibilitySourceGuard(source); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range [][2]string{
		{"[15]bool", "[16]bool"},
		{`"coverage-owner-mismatch"`, `"invented-eligibility-reason"`},
		{"c.bindingPresence, c.bindingHashes", "c.bindingHashes, c.bindingPresence"},
		{"c.recipeDecodable, c.recipeOwner", "c.recipeOwner, c.recipeDecodable"},
		{"c.coverageOwner,", ""},
		{"DecodeRecipeCoverage(read.Bytes)", "privateLenientDecoder(read.Bytes)"},
		{"DecodeRecipeCaptureEvent(read.Bytes)", "eventCopiedFromCoverage(c.coverage)"},
		{"RecipeCaptureEventIdentity(event)", "rawEventHash(read.Bytes)"},
		{"decodeCoverageRecipe(c.snapshot.Recipe.Bytes)", "trustRecipeDecodableFlag(c.coverage)"},
		{"c.coverage, c.event,", "c.coverage, RecipeCaptureEvent{},"},
		{"if !gate() {", "if false {"},
		{"\tc.snapshot = SnapshotRecipeCoverage", "\tc.result.eligible = true\n\tc.snapshot = SnapshotRecipeCoverage"},
		{"return simulation.ExactPostimage && simulation.AllAlreadyPresent", "c.event.ParentCreatedPaths = currentParentScan()\n\treturn simulation.ExactPostimage && simulation.AllAlreadyPresent"},
	} {
		if strings.Count(source, mutation[0]) != 1 {
			t.Fatalf("mutation anchor not unique: %s", mutation[0])
		}
		bad := strings.Replace(source, mutation[0], mutation[1], 1)
		if err := rocS2EligibilitySourceGuard(bad); err == nil {
			t.Fatalf("guard accepted %s", mutation[1])
		}
	}
}
