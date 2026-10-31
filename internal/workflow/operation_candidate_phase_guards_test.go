package workflow

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func rocS6MentionsProof(node ast.Node) bool {
	if node == nil {
		return false
	}
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == "phase2AuthorityProof" {
			found = true
		}
		return !found
	})
	return found
}

func rocS6NodeText(node ast.Node) string {
	var body bytes.Buffer
	_ = format.Node(&body, token.NewFileSet(), node)
	return body.String()
}

func validateROCS6AuthoritySources(sources map[string]string) error {
	proofTypes, stagingFields, constructors := 0, 0, 0
	initializations, mutations, reads := 0, 0, 0
	proofParameters := map[string]bool{}
	allowedProofFunctions := map[string]bool{
		"evaluateReconcileOperationCandidate": true,
		"saveReconcileArtifacts":              true,
		"applyUpstreamedConfirmationGate":     true,
	}
	var failure error
	for path, source := range sources {
		file, err := parser.ParseFile(token.NewFileSet(), path, source, 0)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			group, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range group.Specs {
				switch spec := spec.(type) {
				case *ast.ValueSpec:
					if rocS6MentionsProof(spec) {
						return fmt.Errorf("package-level proof storage")
					}
				case *ast.TypeSpec:
					if spec.Name.Name != "phase2AuthorityProof" && rocS6MentionsProof(spec.Type) {
						return fmt.Errorf("proof alias or containing type")
					}
					structure, ok := spec.Type.(*ast.StructType)
					if !ok {
						continue
					}
					if spec.Name.Name == "phase2AuthorityProof" {
						proofTypes++
						for _, field := range structure.Fields.List {
							if field.Tag != nil || len(field.Names) != 1 || field.Names[0].IsExported() {
								return fmt.Errorf("proof exposes a wire field")
							}
						}
					}
					if spec.Name.Name == "ReconcileResult" {
						for _, field := range structure.Fields.List {
							for _, name := range field.Names {
								if name.Name == "verdictAttempts" {
									stagingFields++
									if field.Tag != nil || rocS6NodeText(field.Type) != "[]store.ReconcileEvidence" {
										return fmt.Errorf("staging wire/type drift")
									}
								}
							}
						}
					}
				}
			}
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := fn.Name.Name
			if fn.Recv != nil && rocS6MentionsProof(fn.Recv) {
				return fmt.Errorf("proof method or marshaller")
			}
			if fn.Type.Results != nil && rocS6MentionsProof(fn.Type.Results) {
				return fmt.Errorf("proof returned from %s", name)
			}
			for _, field := range fn.Type.Params.List {
				if !rocS6MentionsProof(field.Type) {
					continue
				}
				if (name != "saveReconcileArtifacts" && name != "applyUpstreamedConfirmationGate") ||
					rocS6NodeText(field.Type) != "*phase2AuthorityProof" || len(field.Names) != 1 || field.Names[0].Name != "proof" ||
					len(fn.Type.Params.List) != 4 || fn.Type.Params.List[3] != field {
					return fmt.Errorf("proof parameter escaped to %s", name)
				}
				proofParameters[name] = true
			}
			allowedSelectors := map[token.Pos]bool{}
			parents := map[ast.Node]ast.Node{}
			var stack []ast.Node
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if n == nil {
					stack = stack[:len(stack)-1]
					return false
				}
				if len(stack) > 0 {
					parents[n] = stack[len(stack)-1]
				}
				stack = append(stack, n)
				return true
			})
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if failure != nil {
					return false
				}
				switch node := n.(type) {
				case *ast.FuncLit:
					if rocS6MentionsProof(node.Type) {
						failure = fmt.Errorf("proof closure carrier")
					}
					if allowedProofFunctions[name] {
						ast.Inspect(node.Body, func(n ast.Node) bool {
							if id, ok := n.(*ast.Ident); ok && id.Name == "proof" {
								failure = fmt.Errorf("proof captured by closure")
							}
							return true
						})
					}
				case *ast.ValueSpec:
					if rocS6MentionsProof(node.Type) && (name != "evaluateReconcileOperationCandidate" ||
						len(node.Names) != 1 || node.Names[0].Name != "proof") {
						failure = fmt.Errorf("unregistered proof variable")
					}
				case *ast.CompositeLit:
					if rocS6NodeText(node.Type) == "phase2AuthorityProof" {
						constructors++
						if name != "evaluateReconcileOperationCandidate" {
							failure = fmt.Errorf("proof minted outside evaluator")
						}
					}
				case *ast.AssignStmt:
					for _, left := range node.Lhs {
						selected, ok := left.(*ast.SelectorExpr)
						if !ok || selected.Sel.Name != "verdictAttempts" {
							continue
						}
						if name != "evaluateReconcileOperationCandidate" {
							failure = fmt.Errorf("staging assigned outside evaluator")
							continue
						}
						if len(node.Lhs) == 1 && len(node.Rhs) == 1 {
							call, ok := node.Rhs[0].(*ast.CallExpr)
							if ok && rocS2Selector(call.Fun) == "append" && len(call.Args) == 3 && rocS6NodeText(call.Args[0]) == "result.verdictAttempts" {
								mutations++
								allowedSelectors[selected.Pos()] = true
								allowedSelectors[call.Args[0].Pos()] = true
								continue
							}
						}
						if len(node.Lhs) != 2 || len(node.Rhs) != 2 || rocS6NodeText(node.Lhs[1]) != "proof" {
							failure = fmt.Errorf("proof and qualifying attempt are not minted together")
							continue
						}
						address, ok := node.Rhs[1].(*ast.UnaryExpr)
						if !ok || address.Op != token.AND || !rocS6MentionsProof(address.X) {
							failure = fmt.Errorf("joint mint lost typed proof")
							continue
						}
						initializations++
						allowedSelectors[selected.Pos()] = true
					}
					if name == "persistReconcileEvidence" && len(node.Lhs) == 1 && len(node.Rhs) == 1 &&
						rocS6NodeText(node.Lhs[0]) == "attempts" && rocS6NodeText(node.Rhs[0]) == "result.verdictAttempts" {
						reads++
						allowedSelectors[node.Rhs[0].Pos()] = true
					}
				case *ast.CallExpr:
					called := rocS2Selector(node.Fun)
					if called == "applyUpstreamedConfirmationGate" &&
						(name != "saveReconcileArtifacts" || len(node.Args) != 4 || rocS6NodeText(node.Args[3]) != "proof") {
						failure = fmt.Errorf("confirmation gate has an unregistered caller")
					}
					if called == "saveReconcileArtifacts" {
						if len(node.Args) != 4 || (name != "reconcileFeature" && name != "evaluateReconcileOperationCandidate") {
							failure = fmt.Errorf("artifact persistence has an unregistered caller")
						} else if rocS6NodeText(node.Args[3]) != "nil" &&
							(name != "evaluateReconcileOperationCandidate" || rocS6NodeText(node.Args[3]) != "proof") {
							failure = fmt.Errorf("unregistered non-nil proof argument")
						}
					}
					if !allowedProofFunctions[name] {
						break
					}
					contains := false
					ast.Inspect(node, func(n ast.Node) bool {
						if nested, ok := n.(*ast.CallExpr); ok && nested != node {
							return false
						}
						if id, ok := n.(*ast.Ident); ok && id.Name == "proof" {
							contains = true
						}
						return true
					})
					if !contains {
						break
					}
					callee := rocS2Selector(node.Fun)
					allowed := (name == "evaluateReconcileOperationCandidate" && callee == "saveReconcileArtifacts") ||
						(name == "saveReconcileArtifacts" && callee == "applyUpstreamedConfirmationGate")
					if allowed && len(node.Args) == 4 && rocS6NodeText(node.Args[3]) == "proof" {
						break
					}
					if name == "saveReconcileArtifacts" && callee == "persistReconcileEvidence" &&
						len(node.Args) == 4 && rocS6NodeText(node.Args[3]) == "proof != nil" {
						break
					}
					if name != "applyUpstreamedConfirmationGate" || callee != "os.SameFile" {
						failure = fmt.Errorf("proof passed to unregistered call %s", callee)
					}
				case *ast.Ident:
					if node.Name != "proof" || !allowedProofFunctions[name] {
						break
					}
					switch parent := parents[node].(type) {
					case *ast.ValueSpec:
					case *ast.SelectorExpr:
						if parent.X != node || name != "applyUpstreamedConfirmationGate" {
							failure = fmt.Errorf("proof fields used outside gate")
						}
					case *ast.BinaryExpr:
						if (parent.Op != token.EQL && parent.Op != token.NEQ) ||
							(rocS6NodeText(parent.X) != "nil" && rocS6NodeText(parent.Y) != "nil") {
							failure = fmt.Errorf("unregistered proof comparison")
						}
					case *ast.AssignStmt:
						isLeft := false
						for _, left := range parent.Lhs {
							isLeft = isLeft || left == node
						}
						if !isLeft || (name != "evaluateReconcileOperationCandidate" &&
							(name != "saveReconcileArtifacts" || len(parent.Rhs) != 1 || rocS6NodeText(parent.Rhs[0]) != "nil")) {
							failure = fmt.Errorf("proof copied or stored")
						}
					case *ast.CallExpr:
					default:
						failure = fmt.Errorf("proof escaped its parameter route")
					}
				}
				return true
			})
			if failure != nil {
				return failure
			}
			if name == "saveReconcileArtifacts" {
				order := []string{"persistReconcileEvidence", "persistFileNoveltyEvidence", "persistHunkOverlapEvidence",
					"persistPathRestructureEvidence", "persistBlockedClassificationEvidence", "applyUpstreamedConfirmationGate",
					"persistRevisionPassLog"}
				var got []string
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					if call, ok := n.(*ast.CallExpr); ok {
						callee := rocS2Selector(call.Fun)
						for _, wanted := range order {
							if callee == wanted {
								got = append(got, callee)
							}
						}
					}
					return true
				})
				if strings.Join(got, ",") != strings.Join(order, ",") {
					return fmt.Errorf("supporting evidence call sequence changed")
				}
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if selected, ok := n.(*ast.SelectorExpr); ok && selected.Sel.Name == "verdictAttempts" && !allowedSelectors[selected.Pos()] {
					failure = fmt.Errorf("unregistered staging read/mutation in %s", name)
				}
				return true
			})
			if failure != nil {
				return failure
			}
		}
	}
	if proofTypes != 1 || stagingFields != 1 || constructors != 1 || initializations != 1 || mutations != 1 || reads != 1 || len(proofParameters) != 2 {
		return fmt.Errorf("authority inventory drift: types=%d fields=%d mints=%d init=%d mutation=%d reads=%d parameters=%d",
			proofTypes, stagingFields, constructors, initializations, mutations, reads, len(proofParameters))
	}
	return nil
}

func TestROCS6AuthoritySourceGuardAndSensitivity(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		body, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		sources[entry.Name()] = string(body)
	}
	if err := validateROCS6AuthoritySources(sources); err != nil {
		t.Fatal(err)
	}
	for _, addition := range []string{
		"func forgedProof() any { return &phase2AuthorityProof{} }",
		"type StoredProof struct { Value *phase2AuthorityProof }",
		"type ProofAlias = phase2AuthorityProof",
		"var globalProof *phase2AuthorityProof",
		"func proofCarrier(p *phase2AuthorityProof) {}",
		"func proofResult() *phase2AuthorityProof { return nil }",
		"func (p *phase2AuthorityProof) MarshalJSON() ([]byte,error) { return nil,nil }",
		"func stealStaging(result *ReconcileResult) any { return result.verdictAttempts }",
		"func alterStaging(result *ReconcileResult) { result.verdictAttempts = nil }",
		"func extraGate() { applyUpstreamedConfirmationGate(nil,\"x\",nil,nil) }",
	} {
		mutant := map[string]string{}
		for path, source := range sources {
			mutant[path] = source
		}
		mutant["forged.go"] = "package workflow\n" + addition
		if err := validateROCS6AuthoritySources(mutant); err == nil {
			t.Fatalf("source guard accepted %s", addition)
		}
	}
	for _, mutation := range [][3]string{
		{"operation_candidate_phase.go", "type phase2AuthorityProof struct {", "type phase2AuthorityProof struct {\n Public bool"},
		{"operation_candidate_phase.go", "saveReconcileArtifacts(s, slug, result, proof)", "leakProof(proof)"},
		{"operation_candidate_phase.go", "saveReconcileArtifacts(s, slug, result, proof)", "func() { saveReconcileArtifacts(s, slug, result, proof) }()"},
		{"operation_candidate_phase.go", "var proof *phase2AuthorityProof", "var proof *phase2AuthorityProof\n alias := proof\n _ = alias"},
		{"operation_candidate_phase.go", "result.verdictAttempts = append(result.verdictAttempts, first, second)", "result.verdictAttempts = nil"},
		{"reconcile.go", "attempts := result.verdictAttempts", "attempts := []store.ReconcileEvidence{}"},
		{"reconcile.go", "persistFileNoveltyEvidence(s, slug, result)", "skipFileNoveltyEvidence(s, slug, result)"},
	} {
		mutant := map[string]string{}
		for path, source := range sources {
			mutant[path] = source
		}
		if strings.Count(mutant[mutation[0]], mutation[1]) != 1 {
			t.Fatalf("mutation anchor not unique: %s", mutation[1])
		}
		mutant[mutation[0]] = strings.Replace(mutant[mutation[0]], mutation[1], mutation[2], 1)
		if err := validateROCS6AuthoritySources(mutant); err == nil {
			t.Fatalf("source guard accepted %s", mutation[2])
		}
	}
}
