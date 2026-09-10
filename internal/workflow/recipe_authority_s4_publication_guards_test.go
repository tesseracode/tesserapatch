package workflow

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var rgaS4ProducerFunctions = map[string]map[string]bool{
	"internal/workflow/recipe_autogen.go":  {"AutogenRecipeForRecord": true, "convergeRecipeProvenance": true, "publishRecordRecipePlan": true},
	"internal/workflow/implement.go":       {"RunImplement": true, "ObserveImplementCheckpoint": true},
	"internal/workflow/refresh.go":         {"RefreshAfterAccept": true},
	"internal/workflow/accept.go":          {"AcceptShadow": true},
	"internal/workflow/reconcile.go":       {"RunReconcile": true, "reconcileFeature": true, "tryPhase35": true},
	"internal/cli/cobra.go":                {"recordCmd": true, "runApplyDone": true, "runManualPhase": true, "manualCheckpointHook": true},
	"internal/cli/feature_patch.go":        {"runFeaturePatchAmend": true},
	"internal/cli/phase2.go":               {"cycleCmd": true},
	"internal/cli/c1.go":                   {"runEditWithObservation": true, "observeArtifactEdit": true},
	"internal/cli/producer_observation.go": {"coverageFinalizer": true},
}

var rgaS4Registry = map[string][]string{
	"record":              {"cli.recordCmd"},
	"feature-patch-amend": {"cli.runFeaturePatchAmend"},
	"reconcile-accept":    {"workflow.RefreshAfterAccept", "workflow.AcceptShadow", "cli.runReconcileAccept", "workflow.RunReconcile"},
	"cycle":               {"cli.cycleCmd"},
	"apply-done":          {"cli.runApplyDone"},
	"implement":           {"workflow.RunImplement", "cli.runManualPhase"},
	"artifact-edit":       {"cli.runEditWithObservation"},
}

func rgaS4ValidateRegistry(registry map[string][]string, producerSource string) error {
	file, err := rgaS0Parse("patchobs.go", producerSource)
	if err != nil {
		return err
	}
	enum := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		typ, ok := spec.Type.(*ast.Ident)
		if !ok || typ.Name != "ProducerID" {
			return true
		}
		for _, value := range spec.Values {
			if text, ok := rgaS0StringLit(value); ok {
				enum[text] = true
			}
		}
		return true
	})
	if len(registry) != 7 || len(enum) != 7 {
		return fmt.Errorf("registry and producer enum must both have seven entries")
	}
	for producer, roots := range registry {
		if !enum[producer] || len(roots) == 0 {
			return fmt.Errorf("unregistered producer %s", producer)
		}
	}
	return nil
}

// The site inventory is independent of the seven-entry registry. Shared
// publication/recipe helpers inherit the caller's event, while cycle's
// RunImplement call starts a separate P6 event instead of becoming a P4 write.
func rgaS4ValidateMapping(sources map[string]string, registry map[string][]string) error {
	type function struct {
		body    *ast.FuncDecl
		pkg     string
		file    string
		imports map[string]string
	}
	type edge struct {
		callee string
		call   *ast.CallExpr
	}
	functions := map[string]function{}
	sites := map[string][]string{}
	files := map[string]*ast.File{}
	fileImports := map[string]map[string]string{}
	for path, src := range sources {
		file, err := rgaS0Parse(path, src)
		if err != nil {
			return err
		}
		imports := map[string]string{}
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if strings.HasPrefix(path, "github.com/tesseracode/tesserapatch/internal/") {
				name := filepath.Base(path)
				if spec.Name != nil {
					name = spec.Name.Name
				}
				imports[name] = filepath.Base(path)
			}
		}
		files[path], fileImports[path] = file, imports
		bound, _, err := rgaS0ScanWriteArtifact(path, src)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				key := file.Name.Name + "." + fn.Name.Name
				functions[key] = function{fn, file.Name.Name, path, imports}
				for _, site := range bound {
					if site.Func == rgaS0EnclosingName(fn) {
						sites[key] = append(sites[key], site.key())
					}
				}
			}
		}
	}
	unparen := func(expr ast.Expr) ast.Expr {
		for {
			p, ok := expr.(*ast.ParenExpr)
			if !ok {
				return expr
			}
			expr = p.X
		}
	}
	ambiguousImports := map[ast.Expr]string{}
	resolve := func(pkg string, imports map[string]string, expr ast.Expr) string {
		expr = unparen(expr)
		name := rgaS0CallName(&ast.CallExpr{Fun: expr})
		if name == "" {
			return ""
		}
		if qualifier, member, found := strings.Cut(name, "."); found && imports[qualifier] != "" {
			target := imports[qualifier] + "." + member
			if selector, ok := expr.(*ast.SelectorExpr); ok {
				if receiver, ok := selector.X.(*ast.Ident); ok &&
					receiver.Obj != nil && receiver.Obj.Kind != ast.Pkg {
					ambiguousImports[expr] = target
					return ""
				}
			}
			return target
		}
		if strings.HasPrefix(name, "s.") {
			return "store." + strings.TrimPrefix(name, "s.")
		}
		if !strings.Contains(name, ".") {
			local := pkg + "." + name
			if _, found := functions[local]; !found && imports["."] != "" {
				return imports["."] + "." + name
			}
			return local
		}
		return name
	}
	calls := map[string][]edge{}
	incoming := map[string][]string{}
	directCallees := map[ast.Expr]bool{}
	for key, fn := range functions {
		ast.Inspect(fn.body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := resolve(fn.pkg, fn.imports, call.Fun)
			directCallees[unparen(call.Fun)] = true
			calls[key] = append(calls[key], edge{name, call})
			incoming[name] = append(incoming[name], key)
			return true
		})
	}
	registeredRoots := map[string]bool{}
	for _, roots := range registry {
		for _, root := range roots {
			registeredRoots[root] = true
		}
	}
	owners := map[string]map[string]bool{}
	coveredFunctions := map[string]bool{}
	for producer, roots := range registry {
		suppliesRecipePlan := false
		for _, root := range roots {
			fn, ok := functions[root]
			if !ok {
				continue
			}
			ast.Inspect(fn.body, func(n ast.Node) bool {
				if assign, ok := n.(*ast.AssignStmt); ok {
					for _, lhs := range assign.Lhs {
						if field, ok := lhs.(*ast.SelectorExpr); ok && field.Sel.Name == "Autogen" {
							suppliesRecipePlan = true
						}
					}
				}
				return true
			})
		}
		seen := map[string]bool{}
		type visitState struct {
			key              string
			immediateAutogen bool
		}
		visited := map[visitState]bool{}
		var visit func(string, bool)
		visit = func(key string, immediateAutogen bool) {
			state := visitState{key, immediateAutogen}
			if visited[state] || (key == "workflow.RunImplement" && producer != "implement") {
				return
			}
			visited[state] = true
			seen[key] = true
			coveredFunctions[key] = true
			fn, ok := functions[key]
			if !ok {
				return
			}
			for _, site := range sites[key] {
				if owners[site] == nil {
					owners[site] = map[string]bool{}
				}
				owners[site][producer] = true
			}
			for _, call := range calls[key] {
				if call.callee == "workflow.publishRecordRecipePlan" {
					switch key {
					case "workflow.PublishCoverage":
						if !suppliesRecipePlan {
							continue
						}
					case "workflow.AutogenRecipeForRecord":
						if !immediateAutogen {
							continue
						}
					}
				}
				immediate := false
				if call.callee == "workflow.AutogenRecipeForRecord" {
					immediate = !rgaS4AutogenCallDefersWrites(fn.body, call.call)
				}
				visit(call.callee, immediate)
			}
		}
		for _, root := range roots {
			if _, ok := functions[root]; !ok {
				return fmt.Errorf("producer root missing: %s", root)
			}
			visit(root, root == "workflow.AutogenRecipeForRecord")
		}
		if !seen["workflow.PublishCoverage"] {
			return fmt.Errorf("producer %s cannot reach the shared publisher", producer)
		}
		if producer == "implement" && (!seen["store.AdvanceStateManuallyWithCheckpoint"] || !seen["cli.manualCheckpointHook"]) {
			return fmt.Errorf("manual checkpoint delegation is unmapped")
		}
		if producer == "artifact-edit" && !seen["cli.openInEditor"] {
			return fmt.Errorf("editor delegation is unmapped")
		}
	}
	// Trace every incoming route back from a bound writer, stopping only at
	// registered event boundaries. A shared helper being covered from P1/P2
	// does not excuse an additional unregistered caller of that same helper.
	writeReachable := map[string]bool{}
	var queue []string
	for key := range sites {
		writeReachable[key] = true
		queue = append(queue, key)
	}
	for len(queue) != 0 {
		key := queue[0]
		queue = queue[1:]
		if registeredRoots[key] {
			continue
		}
		for _, caller := range incoming[key] {
			if !writeReachable[caller] {
				writeReachable[caller] = true
				queue = append(queue, caller)
			}
		}
	}
	for key := range writeReachable {
		if !coveredFunctions[key] {
			return fmt.Errorf("unmapped incoming caller reaches a bound writer: %s", key)
		}
	}
	// The graph models direct calls, not escaping function values. Reject
	// unmodeled writer references instead of pretending their eventual call
	// inherits an already registered owner. Scan whole files so package-level
	// aliases and initializers cannot hide outside the function inventory.
	for path, file := range files {
		nonValues := map[*ast.Ident]bool{}
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				nonValues[node.Name] = true
			case *ast.Field:
				for _, name := range node.Names {
					nonValues[name] = true
				}
			case *ast.ValueSpec:
				for _, name := range node.Names {
					nonValues[name] = true
				}
			case *ast.TypeSpec:
				nonValues[node.Name] = true
			case *ast.ImportSpec:
				nonValues[node.Name] = true
			case *ast.SelectorExpr:
				nonValues[node.Sel] = true
				if name, ok := node.X.(*ast.Ident); ok {
					nonValues[name] = true
				}
			case *ast.KeyValueExpr:
				if name, ok := node.Key.(*ast.Ident); ok {
					nonValues[name] = true
				}
			case *ast.LabeledStmt:
				nonValues[node.Label] = true
			case *ast.BranchStmt:
				nonValues[node.Label] = true
			}
			return true
		})
		var referenceErr error
		ast.Inspect(file, func(n ast.Node) bool {
			var expr ast.Expr
			artifactMethod := false
			switch node := n.(type) {
			case *ast.Ident:
				if nonValues[node] || (node.Obj != nil && node.Obj.Kind != ast.Fun) {
					return true
				}
				expr = node
			case *ast.SelectorExpr:
				expr = node
				artifactMethod = node.Sel.Name == "WriteArtifact" || node.Sel.Name == "WriteArtifactAtomic"
			default:
				return true
			}
			target := resolve(file.Name.Name, fileImports[path], expr)
			if shadowed := ambiguousImports[expr]; shadowed != "" &&
				(writeReachable[shadowed] || artifactMethod) {
				referenceErr = fmt.Errorf("ambiguous shadowed import names a bound writer %s in %s", shadowed, path)
				return true
			}
			if directCallees[expr] {
				return true
			}
			if writeReachable[target] || artifactMethod {
				referenceErr = fmt.Errorf("unmapped function-value reference to bound writer %s in %s", target, path)
			}
			return true
		})
		if referenceErr != nil {
			return referenceErr
		}
	}
	for _, bound := range sites {
		for _, site := range bound {
			if len(owners[site]) == 0 {
				return fmt.Errorf("unmapped bound-input site: %s", site)
			}
			if strings.Contains(site, "|writeRecipe|") {
				if !reflect.DeepEqual(owners[site], map[string]bool{"record": true, "feature-patch-amend": true}) {
					return fmt.Errorf("shared recipe helper must inherit exactly P1 and P2: %v", owners[site])
				}
			} else if len(owners[site]) != 1 {
				return fmt.Errorf("ambiguous producer chain: %s %v", site, owners[site])
			}
		}
	}
	return nil
}

func rgaS4AutogenCallDefersWrites(fn *ast.FuncDecl, target *ast.CallExpr) bool {
	if len(target.Args) != 5 {
		// The compatibility call with four arguments publishes immediately.
		return false
	}
	input, ok := target.Args[4].(*ast.Ident)
	if !ok {
		return false
	}
	deferred := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		block, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}
		for index, stmt := range block.List {
			assign, ok := stmt.(*ast.AssignStmt)
			if !ok {
				continue
			}
			isTarget := false
			for _, rhs := range assign.Rhs {
				isTarget = isTarget || rhs == target
			}
			if !isTarget {
				continue
			}
			for _, prior := range block.List[:index] {
				set, ok := prior.(*ast.AssignStmt)
				if !ok || len(set.Lhs) != 1 || len(set.Rhs) != 1 {
					continue
				}
				if name, ok := set.Lhs[0].(*ast.Ident); ok && name.Name == input.Name {
					deferred = false
				}
				field, ok := set.Lhs[0].(*ast.SelectorExpr)
				if !ok || field.Sel.Name != "DeferRecipeWrites" {
					continue
				}
				receiver, receiverOK := field.X.(*ast.Ident)
				value, valueOK := set.Rhs[0].(*ast.Ident)
				if receiverOK && receiver.Name == input.Name {
					deferred = valueOK && value.Name == "true"
				}
			}
		}
		return true
	})
	return deferred
}

func TestRGAS4ProducerRegistryAndReachableSiteMapping(t *testing.T) {
	producerSource := rgaS0ReadRepoFile(t, "internal/patchobs/patchobs.go")
	if err := rgaS4ValidateRegistry(rgaS4Registry, producerSource); err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, rel := range rgaS0ProductionGoFiles(t) {
		if strings.HasPrefix(rel, "internal/cli/") || strings.HasPrefix(rel, "internal/workflow/") || strings.HasPrefix(rel, "internal/store/") {
			sources[rel] = rgaS0ReadRepoFile(t, rel)
		}
	}
	if err := rgaS4ValidateMapping(sources, rgaS4Registry); err != nil {
		t.Fatal(err)
	}
	t.Run("existing-p1-p2-deferred-calls", func(t *testing.T) {
		for path, name := range map[string]string{
			"internal/cli/cobra.go":         "recordCmd",
			"internal/cli/feature_patch.go": "runFeaturePatchAmend",
		} {
			file, err := rgaS0Parse(path, sources[path])
			if err != nil {
				t.Fatal(err)
			}
			fn := rgaS0FuncBody(file, name)
			if fn == nil {
				t.Fatalf("%s missing", name)
			}
			calls := 0
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok && rgaS0CallName(call) == "workflow.AutogenRecipeForRecord" {
					calls++
					if !rgaS4AutogenCallDefersWrites(fn, call) {
						t.Fatalf("%s no longer supplies the proven deferred plan", name)
					}
				}
				return true
			})
			if calls != 1 {
				t.Fatalf("%s autogen call count=%d", name, calls)
			}
		}
		if err := rgaS4ValidateMapping(sources, rgaS4Registry); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("p3-direct-compatibility-autogen-writer", func(t *testing.T) {
		path := "internal/workflow/refresh.go"
		before := sources[path]
		anchor := "publication := ObserveCoveragePublication(s, obs)"
		sources[path] = strings.Replace(before, anchor, "_, _ = AutogenRecipeForRecord(s, obs, true, false)\n\t"+anchor, 1)
		defer func() { sources[path] = before }()
		if sources[path] == before || rgaS4ValidateMapping(sources, rgaS4Registry) == nil {
			t.Fatal("P3's direct compatibility writer escaped site-to-producer mapping")
		}
	})
	t.Run("unregistered-command-calls-compatibility-autogen", func(t *testing.T) {
		path := "internal/cli/s4_unregistered_command.go"
		sources[path] = "package cli\nfunc unregisteredCommand(s *store.Store, obs patchobs.Observation) { workflow.AutogenRecipeForRecord(s, obs, true, false) }\n"
		defer delete(sources, path)
		if rgaS4ValidateMapping(sources, rgaS4Registry) == nil {
			t.Fatal("unregistered incoming compatibility caller escaped mapping")
		}
	})
	t.Run("unregistered-command-uses-import-alias", func(t *testing.T) {
		path := "internal/cli/s4_unregistered_alias.go"
		sources[path] = "package cli\nimport wf \"github.com/tesseracode/tesserapatch/internal/workflow\"\nfunc unregisteredAlias(s *store.Store, obs patchobs.Observation) { wf.AutogenRecipeForRecord(s, obs, true, false) }\n"
		defer delete(sources, path)
		if rgaS4ValidateMapping(sources, rgaS4Registry) == nil {
			t.Fatal("an import alias hid an unregistered incoming compatibility caller")
		}
	})
	t.Run("p3-function-value-compatibility-autogen", func(t *testing.T) {
		path := "internal/workflow/refresh.go"
		before := sources[path]
		const anchor = "publication := ObserveCoveragePublication(s, obs)"
		sources[path] = strings.Replace(before, anchor,
			"autogen := AutogenRecipeForRecord\n\t_, _ = autogen(s, obs, true, false)\n\t"+anchor, 1)
		defer func() { sources[path] = before }()
		if sources[path] == before || rgaS4ValidateMapping(sources, rgaS4Registry) == nil {
			t.Fatal("P3 hid an immediate bound writer behind a function-value alias")
		}
	})
	t.Run("unregistered-function-value-compatibility-autogen", func(t *testing.T) {
		path := "internal/cli/s4_unregistered_function_value.go"
		sources[path] = "package cli\nimport (\nwf \"github.com/tesseracode/tesserapatch/internal/workflow\"\n\"github.com/tesseracode/tesserapatch/internal/store\"\n\"github.com/tesseracode/tesserapatch/internal/patchobs\"\n)\nfunc unregisteredFunctionValue(s *store.Store, obs patchobs.Observation) { autogen := wf.AutogenRecipeForRecord; _, _ = autogen(s, obs, true, false) }\n"
		defer delete(sources, path)
		if rgaS4ValidateMapping(sources, rgaS4Registry) == nil {
			t.Fatal("an unregistered function-value alias hid the shared writer")
		}
	})
	for _, test := range []struct{ name, body string }{
		{"colliding-import-direct-call", "_, _ = s.AutogenRecipeForRecord(st, obs, true, false)"},
		{"colliding-import-function-value", "autogen := s.AutogenRecipeForRecord; _, _ = autogen(st, obs, true, false)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := "internal/cli/s4_colliding_import.go"
			sources[path] = "package cli\nimport (\ns \"github.com/tesseracode/tesserapatch/internal/workflow\"\n\"github.com/tesseracode/tesserapatch/internal/store\"\n\"github.com/tesseracode/tesserapatch/internal/patchobs\"\n)\nfunc unregisteredCollidingImport(st *store.Store, obs patchobs.Observation) { " + test.body + " }\n"
			defer delete(sources, path)
			if rgaS4ValidateMapping(sources, rgaS4Registry) == nil {
				t.Fatal("workflow import s was incorrectly treated as a store receiver")
			}
		})
	}
	t.Run("shadowed-import-writer-binding-is-not-guessed", func(t *testing.T) {
		path := "internal/cli/s4_shadowed_import.go"
		sources[path] = "package cli\nimport s \"github.com/tesseracode/tesserapatch/internal/workflow\"\nvar _ s.ApplyRecipe\nfunc shadowedImport(s struct { AutogenRecipeForRecord func() }) { s.AutogenRecipeForRecord() }\n"
		defer delete(sources, path)
		if rgaS4ValidateMapping(sources, rgaS4Registry) == nil {
			t.Fatal("a shadowed imported writer binding received guessed ownership")
		}
	})
	t.Run("colliding-import-type-reference-is-harmless", func(t *testing.T) {
		path := "internal/cli/s4_colliding_import_type.go"
		sources[path] = "package cli\nimport s \"github.com/tesseracode/tesserapatch/internal/workflow\"\nfunc collidingImportType() s.ApplyRecipe { return s.ApplyRecipe{} }\n"
		defer delete(sources, path)
		if err := rgaS4ValidateMapping(sources, rgaS4Registry); err != nil {
			t.Fatalf("harmless type reference through import s was rejected: %v", err)
		}
	})
	t.Run("package-level-function-value-alias", func(t *testing.T) {
		path := "internal/cli/s4_unregistered_package_alias.go"
		sources[path] = "package cli\nimport (\nwf \"github.com/tesseracode/tesserapatch/internal/workflow\"\n\"github.com/tesseracode/tesserapatch/internal/store\"\n\"github.com/tesseracode/tesserapatch/internal/patchobs\"\n)\nvar hiddenAutogen = wf.AutogenRecipeForRecord\nfunc unregisteredPackageAlias(s *store.Store, obs patchobs.Observation) { _, _ = hiddenAutogen(s, obs, true, false) }\n"
		defer delete(sources, path)
		if rgaS4ValidateMapping(sources, rgaS4Registry) == nil {
			t.Fatal("a package-level alias escaped the incoming writer inventory")
		}
	})
	t.Run("artifact-method-value-alias", func(t *testing.T) {
		path := "internal/cli/s4_unregistered_method_value.go"
		sources[path] = "package cli\nimport \"github.com/tesseracode/tesserapatch/internal/store\"\nfunc unregisteredMethodValue(s *store.Store) { write := s.WriteArtifactAtomic; _ = write(\"slug\", \"apply-recipe.json\", \"{}\") }\n"
		defer delete(sources, path)
		if rgaS4ValidateMapping(sources, rgaS4Registry) == nil {
			t.Fatal("an artifact method-value hid a bound write")
		}
	})
	t.Run("parenthesized-direct-call-keeps-owner", func(t *testing.T) {
		path := "internal/cli/cobra.go"
		before := sources[path]
		sources[path] = strings.Replace(before, "workflow.AutogenRecipeForRecord(s,",
			"(workflow.AutogenRecipeForRecord)(s,", 1)
		defer func() { sources[path] = before }()
		if sources[path] == before {
			t.Fatal("parenthesized direct-call control did not change its input")
		}
		if err := rgaS4ValidateMapping(sources, rgaS4Registry); err != nil {
			t.Fatalf("parenthesized direct call lost its registered owner: %v", err)
		}
	})
	t.Run("function-name-text-is-not-a-reference", func(t *testing.T) {
		path := "internal/cli/s4_writer_name_text.go"
		sources[path] = "package cli\nfunc writerNameText() string { return \"AutogenRecipeForRecord\" }\n"
		defer delete(sources, path)
		if err := rgaS4ValidateMapping(sources, rgaS4Registry); err != nil {
			t.Fatalf("ordinary text was treated as a writer reference: %v", err)
		}
	})
	t.Run("five-only-registry", func(t *testing.T) {
		five := map[string][]string{}
		for name, roots := range rgaS4Registry {
			if name != "implement" && name != "artifact-edit" {
				five[name] = roots
			}
		}
		if rgaS4ValidateRegistry(five, producerSource) == nil {
			t.Fatal("five-only registry accepted")
		}
	})
	t.Run("enum-count-drift", func(t *testing.T) {
		changed := strings.Replace(producerSource, `ProducerEdit            ProducerID`, `ProducerEdit            string`, 1)
		if changed == producerSource || rgaS4ValidateRegistry(rgaS4Registry, changed) == nil {
			t.Fatal("enum-count drift accepted or mutation did not apply")
		}
	})
	t.Run("shared-helper-invents-producer", func(t *testing.T) {
		mutated := map[string][]string{}
		for name, roots := range rgaS4Registry {
			mutated[name] = roots
		}
		mutated["recipe-helper"] = []string{"workflow.writeRecipe"}
		if rgaS4ValidateRegistry(mutated, producerSource) == nil {
			t.Fatal("helper cannot become an eighth producer")
		}
	})
	t.Run("shared-helper-loses-one-caller", func(t *testing.T) {
		path := "internal/cli/cobra.go"
		before := sources[path]
		sources[path] = strings.Replace(before, "publication.Autogen = &autogenOutcome", "", 1)
		defer func() { sources[path] = before }()
		if before == sources[path] || rgaS4ValidateMapping(sources, rgaS4Registry) == nil {
			t.Fatal("shared helper lost P1 without a mapping refusal")
		}
	})
	for _, tc := range []struct{ name, path, addition string }{
		{"unmapped-bound-writer", "internal/cli/s4_planted.go", "package cli\nfunc unowned(s *Store){s.WriteArtifactAtomic(slug,\"post-apply.patch\",body)}"},
		{"unmapped-recipe-writer", "internal/workflow/s4_planted.go", "package workflow\nfunc unowned(s *Store){s.WriteArtifactAtomic(slug,\"apply-recipe.json\",body)}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sources[tc.path] = tc.addition
			defer delete(sources, tc.path)
			if rgaS4ValidateMapping(sources, rgaS4Registry) == nil {
				t.Fatal("new unmapped site accepted")
			}
		})
	}
	t.Run("new-site-in-existing-producer-is-not-an-eighth-producer", func(t *testing.T) {
		path := "internal/cli/feature_patch.go"
		before := sources[path]
		sources[path] = strings.Replace(before, "patchLabel :=", "s.WriteArtifactAtomic(slug, \"post-apply.patch\", patch)\npatchLabel :=", 1)
		defer func() { sources[path] = before }()
		if sources[path] == before {
			t.Fatal("mutation did not apply")
		}
		if err := rgaS4ValidateMapping(sources, rgaS4Registry); err != nil {
			t.Fatal(err)
		}
	})
}

func TestRGAS4PublisherReachableCallChainsStayRegistered(t *testing.T) {
	var paths []string
	for path := range rgaS4ProducerFunctions {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			src := rgaS0ReadRepoFile(t, path)
			if err := rgaS0CoveragePhaseSource(path, src); err != nil {
				t.Fatal(err)
			}
			if err := rgaS0CoveragePhaseSource(path, src+"\nfunc unregistered(){ PublishCoverage(nil, CoveragePublicationInput{}) }\n"); err == nil {
				t.Fatal("registered file must not exempt an unregistered function")
			}
		})
	}
}

// Registered producer files are not exempt files: only their named event
// chains may reference publication, and none may encode or write coverage.
func rgaS4PublicationSource(rel, src string) error {
	file, err := rgaS0Parse(rel, src)
	if err != nil {
		return err
	}
	publisher := rel == "internal/workflow/recipe_coverage_publish.go"
	if !publisher {
		if err := rgaS0UnregisteredEvidenceLiteral(rel, src); err != nil {
			return err
		}
	}
	for _, decl := range file.Decls {
		fn, isFunc := decl.(*ast.FuncDecl)
		owner := ""
		if isFunc {
			owner = rgaS0EnclosingName(fn)
		}
		var refusal error
		ast.Inspect(decl, func(n ast.Node) bool {
			if publisher {
				if call, ok := n.(*ast.CallExpr); ok {
					name := rgaS0CallName(call)
					eventCodec := strings.Contains(name, "CaptureEvent") && (strings.HasPrefix(name, "Build") || strings.HasPrefix(name, "Encode"))
					if owner != "PublishCoverage" && (name == "BuildRecipeCoverage" || name == "EncodeRecipeCoverage" || eventCodec) {
						refusal = fmt.Errorf("coverage derivation/encoding outside the primary publisher: %s", owner)
					}
					if selector, ok := call.Fun.(*ast.SelectorExpr); ok && owner != "PublishCoverage" &&
						(selector.Sel.Name == "WriteArtifact" || selector.Sel.Name == "WriteArtifactAtomic" || selector.Sel.Name == "WriteFeatureFile") {
						refusal = fmt.Errorf("alternate artifact writer in the publisher file: %s", owner)
					}
				}
			}
			if id, ok := n.(*ast.Ident); ok {
				coverageSymbol := strings.Contains(id.Name, "Coverage") || strings.Contains(id.Name, "CaptureEvent") || id.Name == "coverageFinalizer"
				if coverageSymbol && !publisher {
					// The shared outcome carries exact bytes; it defines no
					// wire schema and performs no coverage policy itself.
					outcomeField := !isFunc && rel == "internal/workflow/recipe_autogen.go" && id.Name == "CoverageArtifact"
					if !outcomeField && !rgaS4ProducerFunctions[rel][owner] {
						refusal = fmt.Errorf("unregistered coverage chain: %s:%s:%s", rel, owner, id.Name)
					}
					switch id.Name {
					case "CoverageArtifact", "CoveragePublicationInput", "ObserveCoveragePublication",
						"PublishCoverage", "ReconstructEditedCoverage", "coverageFinalizer", "finishCoverage", "CoverageStatus",
						"ErrCoveragePublication", "ReportCoverageStatus":
					default:
						refusal = fmt.Errorf("producer acquired private coverage policy: %s", id.Name)
					}
					if id.Name == "BuildRecipeCoverage" || id.Name == "EncodeRecipeCoverage" ||
						id.Name == "DecodeRecipeCoverage" || id.Name == "RecipeCoverage" ||
						(isFunc && fn.Name == id && id.Name == "PublishCoverage") {
						refusal = fmt.Errorf("producer contains coverage policy/codec: %s", id.Name)
					}
				}
			}
			if literal, ok := n.(*ast.BasicLit); ok {
				if value, valid := rgaS0StringLit(literal); valid && !publisher {
					if value == "recipe-coverage.json" || value == "recipe-capture-event.json" {
						refusal = fmt.Errorf("producer owns a private coverage or capture-event artifact")
					}
					if strings.HasPrefix(strings.TrimSpace(strings.ToLower(value)), "recipe coverage:") {
						refusal = fmt.Errorf("producer owns a private coverage status formatter")
					}
				}
			}
			return true
		})
		if refusal != nil {
			return refusal
		}
	}
	if !publisher {
		if err := rgaS4CoverageCompletionCalls(src); err != nil {
			return err
		}
		if rel == "internal/workflow/implement.go" {
			return rgaS4ImplementPublicationOrder(src)
		}
		if rel == "internal/workflow/accept.go" {
			return rgaS4AcceptPublicationErrorBoundary(src)
		}
		return nil
	}
	fn := rgaS0FuncBody(file, "PublishCoverage")
	if fn == nil {
		return fmt.Errorf("designated publisher lacks PublishCoverage")
	}
	var order []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch rgaS0CallName(call) {
		case "BuildRecipeCoverage", "EncodeRecipeCoverage", "publishRecordRecipePlan", "AppendPatchGenerationForFeature":
			order = append(order, rgaS0CallName(call))
		case "s.WriteArtifactAtomic":
			if len(call.Args) > 1 {
				if name, ok := rgaS0StringLit(call.Args[1]); ok {
					switch name {
					case "recipe-capture-event.json":
						order = append(order, "capture-event-write")
					case "recipe-coverage.json":
						order = append(order, "coverage-write")
					}
				}
			}
		case "s.WriteArtifact", "s.WriteFeatureFile", "os.WriteFile":
			order = append(order, "non-atomic-write")
		}
		return true
	})
	want := "BuildRecipeCoverage>EncodeRecipeCoverage>publishRecordRecipePlan>AppendPatchGenerationForFeature>capture-event-write>coverage-write"
	if strings.Join(order, ">") != want {
		return fmt.Errorf("publication ordering changed: %v", order)
	}
	return nil
}

func TestRGAS4PublicationBoundaryAndSensitivity(t *testing.T) {
	rel := "internal/workflow/recipe_coverage_publish.go"
	src := rgaS0ReadRepoFile(t, rel)
	if err := rgaS0CoveragePhaseSource(rel, src); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, path, source string }{
		{"copied-publisher", "internal/workflow/record.go", src},
		{"copied-publisher-in-producer", "internal/workflow/refresh.go", src},
		{"inline-producer-writer", "internal/cli/cobra.go", "package cli\nfunc recordCmd(){ s.WriteArtifactAtomic(slug,\"recipe-coverage.json\",\"{}\") }"},
		{"inline-producer-event-writer", "internal/cli/cobra.go", "package cli\nfunc recordCmd(){ s.WriteArtifactAtomic(slug,\"recipe-capture-event.json\",\"{}\") }"},
		{"unregistered-event-only-writer", "internal/cli/unregistered_event.go", "package cli\nfunc unregistered(){ s.WriteArtifactAtomic(slug,\"recipe-capture-event.json\",\"{}\") }"},
		{"unregistered-producer-chain", "internal/cli/cobra.go", "package cli\nfunc other(){workflow.PublishCoverage(nil,workflow.CoveragePublicationInput{})}"},
		{"private-producer-codec", "internal/cli/cobra.go", "package cli\nfunc recordCmd(){workflow.EncodeRecipeCoverage(c)}"},
		{"private-producer-event-codec", "internal/cli/cobra.go", "package cli\nfunc recordCmd(){workflow.EncodeCaptureEvent(e)}"},
		{"private-producer-status", "internal/cli/cobra.go", "package cli\nfunc recordCmd(){fmt.Fprintln(w,\"recipe coverage: complete\")}"},
		{"alternate-publisher-entry", rel, src + "\nfunc alternatePublisher(){ s.WriteArtifactAtomic(slug, \"recipe-coverage.json\", \"{}\") }\n"},
		{"alternate-publisher-event-entry", rel, src + "\nfunc alternateEventPublisher(){ s.WriteArtifactAtomic(slug, \"recipe-capture-event.json\", \"{}\") }\n"},
		{"alternate-publisher-feature-entry", rel, src + "\nfunc alternateFeaturePublisher(){ s.WriteFeatureFile(slug, \"artifacts/recipe-capture-event.json\", \"{}\") }\n"},
		{"alternate-publisher-encoder", rel, src + "\nfunc alternateEncoder(){ EncodeRecipeCoverage(c) }\n"},
		{"alternate-publisher-event-encoder", rel, src + "\nfunc alternateEventEncoder(){ EncodeCaptureEvent(e) }\n"},
		{"non-atomic-publication", rel, strings.Replace(src, `s.WriteArtifactAtomic(`, `s.WriteArtifact(`, 1)},
		{"publication-before-recipe", rel, strings.Replace(src, "data, err := EncodeRecipeCoverage(c)", "s.WriteArtifactAtomic(in.Observation.Slug, \"recipe-coverage.json\", \"{}\")\n data, err := EncodeRecipeCoverage(c)", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if rgaS0CoveragePhaseSource(tc.path, tc.source) == nil {
				t.Fatalf("phase validator accepted %s", tc.name)
			}
		})
	}
	for _, mutation := range []struct{ name, artifact, replacement string }{
		{"capture-evidence-omitted", `"recipe-capture-event.json"`, `"unrelated.json"`},
		{"final-coverage-omitted", `"recipe-coverage.json"`, `"recipe-capture-event.json"`},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			if !strings.Contains(src, mutation.artifact) {
				t.Fatalf("publication mutation anchor missing: %s", mutation.artifact)
			}
			wrong := strings.ReplaceAll(src, mutation.artifact, mutation.replacement)
			if err := rgaS0CoveragePhaseSource(rel, wrong); err == nil {
				t.Fatal("same phase validator accepted an invalid E/C publication order")
			}
		})
	}
	t.Run("coverage-before-evidence", func(t *testing.T) {
		if !strings.Contains(src, `"recipe-capture-event.json"`) || !strings.Contains(src, `"recipe-coverage.json"`) {
			t.Fatal("E/C publication mutation anchors missing")
		}
		wrong := strings.NewReplacer(
			`"recipe-capture-event.json"`, `"recipe-coverage.json"`,
			`"recipe-coverage.json"`, `"recipe-capture-event.json"`,
		).Replace(src)
		if err := rgaS0CoveragePhaseSource(rel, wrong); err == nil {
			t.Fatal("same phase validator accepted C-before-E publication")
		}
	})
	t.Run("registered-file-evidence-only-writers", func(t *testing.T) {
		const registered = "internal/cli/c1.go"
		original := rgaS0ReadRepoFile(t, registered)
		for name, addition := range map[string]string{
			"full-path":      "\nfunc unregistered(s *store.Store, slug string) error { return s.WriteFeatureFile(slug, \"artifacts/recipe-capture-event.json\", \"{}\") }\n",
			"concatenated":   "\nfunc unregistered(s *store.Store, slug string) error { return s.WriteFeatureFile(slug, \"artifacts/\" + \"recipe-\" + \"capture-event.json\", \"{}\") }\n",
			"constant-alias": "\nconst eventName = \"recipe-\" + \"capture-\" + \"event.json\"\nconst eventPath = \"artifacts/\" + eventName\nfunc unregistered(s *store.Store, slug string) error { return s.WriteFeatureFile(slug, eventPath, \"{}\") }\n",
			"escaped-path":   "\nfunc unregistered(s *store.Store, slug string) error { return s.WriteFeatureFile(slug, \"artifacts/recipe-\\x63apture-event.json\", \"{}\") }\n",
		} {
			t.Run(name, func(t *testing.T) {
				if err := rgaS0CoveragePhaseSource(registered, original+addition); err == nil {
					t.Fatal("registered-file dispatch bypassed evidence-only writer detection")
				}
			})
		}
		harmless := original + "\nfunc ordinaryFeatureFile(s *store.Store, slug string) error { return s.WriteFeatureFile(slug, \"artifacts/custom.json\", \"{}\") }\n"
		if err := rgaS0CoveragePhaseSource(registered, harmless); err != nil {
			t.Fatalf("unrelated feature file acquired an evidence restriction: %v", err)
		}
	})
}

func TestRGAS4AtomicBoundWriterScanner(t *testing.T) {
	for _, writer := range []string{"WriteArtifact", "WriteArtifactAtomic"} {
		src := "package cli\nfunc recordCmd(){s." + writer + "(slug,\"post-apply.patch\",patch)}"
		bound, _, err := rgaS0ScanWriteArtifact("internal/cli/cobra.go", src)
		if err != nil || len(bound) != 1 {
			t.Fatalf("%s scanner: %v %v", writer, bound, err)
		}
		mutated := strings.Replace(src, `"post-apply.patch"`, "dynamicName", 1)
		bound, dynamic, err := rgaS0ScanWriteArtifact("internal/cli/cobra.go", mutated)
		if err != nil || len(bound) != 0 || len(dynamic) != 1 {
			t.Fatalf("dynamic %s escaped scanner: %v %v %v", writer, bound, dynamic, err)
		}
	}
}

func rgaS4ImplementPublicationOrder(src string) error {
	file, err := rgaS0Parse("implement.go", src)
	if err != nil {
		return err
	}
	fn := rgaS0FuncBody(file, "RunImplement")
	if fn == nil {
		return fmt.Errorf("P6 RunImplement missing")
	}
	if err := rgaS0CheckImplementParseArms(src); err != nil {
		return err
	}
	var recipeWrites, provenanceWrites, stateAttempts, publications []token.Pos
	var parseEnd token.Pos
	validArmFallsThrough := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if branch, ok := n.(*ast.IfStmt); ok && branch.Init != nil && branch.Else != nil {
			if assign, ok := branch.Init.(*ast.AssignStmt); ok {
				for _, expr := range assign.Rhs {
					if call, ok := expr.(*ast.CallExpr); ok && rgaS0CallName(call) == "json.Unmarshal" {
						parseEnd = branch.End()
						if valid, ok := branch.Else.(*ast.BlockStmt); ok {
							validArmFallsThrough = rgaS0BlockReturnsOnlyWriteError(valid)
						}
					}
				}
			}
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch rgaS0CallName(call) {
		case "s.WriteArtifactAtomic":
			if len(call.Args) > 1 {
				name, _ := rgaS0StringLit(call.Args[1])
				switch name {
				case "apply-recipe.json":
					recipeWrites = append(recipeWrites, call.Pos())
				case "recipe-provenance.json":
					provenanceWrites = append(provenanceWrites, call.Pos())
				}
			}
		case "s.MarkFeatureState":
			stateAttempts = append(stateAttempts, call.Pos())
		case "PublishCoverage":
			publications = append(publications, call.Pos())
		}
		return true
	})
	if len(recipeWrites) != 2 || len(provenanceWrites) != 1 || len(stateAttempts) != 1 ||
		len(publications) != 1 || !validArmFallsThrough {
		return fmt.Errorf("P6 lost an arm or its common publication path")
	}
	if recipeWrites[1] >= provenanceWrites[0] || provenanceWrites[0] >= stateAttempts[0] ||
		stateAttempts[0] >= publications[0] {
		return fmt.Errorf("P6 must write recipe, attempt provenance, attempt state, then publish")
	}
	stateVariable, publicationVariable := "", ""
	for _, stmt := range fn.Body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok {
			continue
		}
		for _, expr := range assign.Rhs {
			call, ok := expr.(*ast.CallExpr)
			if !ok {
				continue
			}
			switch rgaS0CallName(call) {
			case "s.MarkFeatureState":
				if len(assign.Lhs) == 1 {
					if id, ok := assign.Lhs[0].(*ast.Ident); ok {
						stateVariable = id.Name
					}
				}
			case "PublishCoverage":
				if len(assign.Lhs) == 2 {
					if id, ok := assign.Lhs[1].(*ast.Ident); ok {
						publicationVariable = id.Name
					}
				}
			}
		}
	}
	if stateVariable == "" || publicationVariable == "" {
		return fmt.Errorf("P6 state/publication attempts must be unconditional at the common return point")
	}
	earlyReturn := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok && ret.Pos() >= parseEnd && ret.Pos() < publications[0] {
			earlyReturn = true
		}
		return true
	})
	if earlyReturn {
		return fmt.Errorf("P6 cancels owed publication after a successful recipe write")
	}
	last, ok := fn.Body.List[len(fn.Body.List)-1].(*ast.ReturnStmt)
	if !ok || len(last.Results) != 1 {
		return fmt.Errorf("P6 final return missing")
	}
	joined, ok := last.Results[0].(*ast.CallExpr)
	if ok && rgaS0CallName(joined) == "ReportCoverageStatus" && len(joined.Args) == 3 {
		joined, ok = joined.Args[2].(*ast.CallExpr)
	}
	if !ok || rgaS0CallName(joined) != "errors.Join" || len(joined.Args) != 3 {
		return fmt.Errorf("P6 must preserve both primary and publication errors")
	}
	first, firstOK := joined.Args[0].(*ast.Ident)
	final, finalOK := joined.Args[2].(*ast.Ident)
	if !firstOK || !finalOK || first.Name != stateVariable || final.Name != publicationVariable {
		return fmt.Errorf("P6 error precedence must be state first, publication last")
	}
	return nil
}

func TestRGAS4ImplementFinalizerOrderAndSensitivity(t *testing.T) {
	src := rgaS0ReadRepoFile(t, "internal/workflow/implement.go")
	if err := rgaS4ImplementPublicationOrder(src); err != nil {
		t.Fatal(err)
	}
	t.Run("same-call-moved-before-provenance", func(t *testing.T) {
		line := "\tcoverage, coverageErr := PublishCoverage(s, publication)\n"
		mutated := strings.Replace(src, line, "", 1)
		mutated = strings.Replace(mutated, "\tvar provenanceErr error\n", line+"\tvar provenanceErr error\n", 1)
		if strings.Count(mutated, "PublishCoverage(s, publication)") != 1 ||
			mutated == src || rgaS4ImplementPublicationOrder(mutated) == nil {
			t.Fatal("P6 validator accepted publication moved before provenance")
		}
	})
	for _, tc := range []struct{ name, old, replacement string }{
		{
			"publication-before-state",
			`stateErr := s.MarkFeatureState`,
			`coverage, coverageErr := PublishCoverage(s, publication)
	stateErr := s.MarkFeatureState`,
		},
		{
			"publication-only-on-state-success",
			`coverage, coverageErr := PublishCoverage(s, publication)`,
			`var coverage RecipeCoverage
	var coverageErr error
	if stateErr == nil { coverage, coverageErr = PublishCoverage(s, publication) }`,
		},
		{
			"literal-nil-success-finalizer",
			`coverage, coverageErr := PublishCoverage(s, publication)`,
			`if stateErr != nil { return stateErr }
	coverage, coverageErr := PublishCoverage(s, publication)`,
		},
		{
			"publication-failure-discarded",
			`return ReportCoverageStatus(nil, coverage, errors.Join(stateErr, provenanceErr, coverageErr))`,
			`return stateErr`,
		},
		{
			"reversed-error-precedence",
			`errors.Join(stateErr, provenanceErr, coverageErr)`,
			`errors.Join(coverageErr, provenanceErr, stateErr)`,
		},
		{
			"valid-arm-returns-before-common-finalizer",
			"if err := s.WriteArtifactAtomic(slug, \"apply-recipe.json\", reserialized); err != nil {\n\t\t\treturn err\n\t\t}",
			"if err := s.WriteArtifactAtomic(slug, \"apply-recipe.json\", reserialized); err != nil {\n\t\t\treturn err\n\t\t}\n\t\treturn nil",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutated := strings.Replace(src, tc.old, tc.replacement, 1)
			if mutated == src || rgaS4ImplementPublicationOrder(mutated) == nil {
				t.Fatalf("P6 validator accepted %s or mutation did not apply", tc.name)
			}
		})
	}
}

func TestRGAS4PureCoreRejectsSpacedBoundIOMethods(t *testing.T) {
	for _, rel := range []string{
		"internal/workflow/recipe_coverage_types.go",
		"internal/workflow/recipe_coverage_codec.go",
		"internal/workflow/recipe_coverage.go",
		"internal/workflow/recipe_coverage_simulation.go",
	} {
		t.Run(filepath.Base(rel), func(t *testing.T) {
			src := rgaS0ReadRepoFile(t, rel)
			if err := rgaS0CoveragePhaseSource(rel, src); err != nil {
				t.Fatal(err)
			}
			for _, method := range []string{"WriteArtifactAtomic", "ReadFeatureFile", "ReportCoverageStatus"} {
				mutated := src + "\nfunc plantedReceiver(storeLike interface{}) { storeLike . " + method + " (\"slug\", \"artifact\", \"bytes\") }\n"
				if rgaS0CoveragePhaseSource(rel, mutated) == nil {
					t.Fatalf("pure phase validator accepted spaced %s in %s", method, rel)
				}
			}
			if rgaS0CoveragePhaseSource(rel, src+"\nvar plantedPublicationError = ErrCoveragePublication\n") == nil {
				t.Fatal("pure phase validator accepted the publication error boundary")
			}
		})
	}
}

func rgaS4AcceptPublicationErrorBoundary(src string) error {
	file, err := rgaS0Parse("accept.go", src)
	if err != nil {
		return err
	}
	fn := rgaS0FuncBody(file, "AcceptShadow")
	if fn == nil {
		return fmt.Errorf("AcceptShadow missing")
	}
	var gate, state, prune token.Pos
	causePreserved := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			switch rgaS0CallName(call) {
			case "s.MarkFeatureState":
				state = call.Pos()
			case "gitutil.PruneShadow":
				prune = call.Pos()
			}
		}
		outer, ok := n.(*ast.IfStmt)
		if !ok || outer.Init == nil {
			return true
		}
		assign, ok := outer.Init.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		refresh, ok := assign.Rhs[0].(*ast.CallExpr)
		errName, named := assign.Lhs[0].(*ast.Ident)
		if !ok || !named || rgaS0CallName(refresh) != "RefreshAfterAccept" {
			return true
		}
		for _, stmt := range outer.Body.List {
			branch, ok := stmt.(*ast.IfStmt)
			if !ok {
				continue
			}
			check, ok := branch.Cond.(*ast.CallExpr)
			if !ok || rgaS0CallName(check) != "errors.Is" || len(check.Args) != 2 {
				continue
			}
			cause, causeOK := check.Args[0].(*ast.Ident)
			sentinel, sentinelOK := check.Args[1].(*ast.Ident)
			if !causeOK || !sentinelOK || cause.Name != errName.Name || sentinel.Name != "ErrCoveragePublication" {
				continue
			}
			gate = branch.Pos()
			if len(branch.Body.List) != 1 {
				continue
			}
			ret, ok := branch.Body.List[0].(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 2 {
				continue
			}
			if direct, ok := ret.Results[1].(*ast.Ident); ok && direct.Name == errName.Name {
				causePreserved = true
				continue
			}
			wrapped, ok := ret.Results[1].(*ast.CallExpr)
			if !ok || rgaS0CallName(wrapped) != "fmt.Errorf" || len(wrapped.Args) != 2 {
				continue
			}
			format, formatOK := rgaS0StringLit(wrapped.Args[0])
			wrappedCause, wrappedOK := wrapped.Args[1].(*ast.Ident)
			causePreserved = formatOK && strings.Contains(format, "%w") && wrappedOK && wrappedCause.Name == errName.Name
		}
		return true
	})
	if gate == token.NoPos || state == token.NoPos || prune == token.NoPos || !causePreserved || gate >= state || gate >= prune {
		return fmt.Errorf("AcceptShadow must return the typed publication cause before applied state or shadow pruning")
	}
	return nil
}

func TestRGAS4AcceptPublicationErrorBoundaryAndSensitivity(t *testing.T) {
	src := rgaS0ReadRepoFile(t, "internal/workflow/accept.go")
	if err := rgaS4AcceptPublicationErrorBoundary(src); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, old, replacement string }{
		{"string-instead-of-typed-cause", "errors.Is(rerr, ErrCoveragePublication)", `strings.Contains(rerr.Error(), "coverage publication failed")`},
		{"typed-cause-flattened", "shadow is preserved for recovery: %w", "shadow is preserved for recovery: %v"},
		{"cause-swallowed", `return res, fmt.Errorf("accept: bound coverage publication failed; accepted files are on disk and the shadow is preserved for recovery: %w", rerr)`, `return res, nil`},
		{"shadow-pruned-before-refusal", "if errors.Is(rerr, ErrCoveragePublication) {", "if errors.Is(rerr, ErrCoveragePublication) {\n gitutil.PruneShadow(s.Root, slug)"},
		{"false-applied-before-refusal", "if errors.Is(rerr, ErrCoveragePublication) {", "if errors.Is(rerr, ErrCoveragePublication) {\n s.MarkFeatureState(slug, store.StateApplied, \"accept\", \"complete\")"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutated := strings.Replace(src, tc.old, tc.replacement, 1)
			if mutated == src || rgaS4AcceptPublicationErrorBoundary(mutated) == nil {
				t.Fatalf("accept boundary accepted %s or mutation did not apply", tc.name)
			}
		})
	}
}

func rgaS4CoverageCompletionCalls(src string) error {
	file, err := rgaS0Parse("producer.go", src)
	if err != nil {
		return err
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		publications, completions := 0, 0
		var refusal error
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				name := rgaS0CallName(call)
				if name == "PublishCoverage" || strings.HasSuffix(name, ".PublishCoverage") {
					publications++
				}
			}
			block, ok := n.(*ast.BlockStmt)
			if !ok {
				return true
			}
			for index, stmt := range block.List {
				bind, ok := stmt.(*ast.AssignStmt)
				if !ok || len(bind.Rhs) != 1 {
					continue
				}
				publish, ok := bind.Rhs[0].(*ast.CallExpr)
				if !ok {
					continue
				}
				name := rgaS0CallName(publish)
				if name != "PublishCoverage" && !strings.HasSuffix(name, ".PublishCoverage") {
					continue
				}
				if len(bind.Lhs) != 2 || index+1 >= len(block.List) {
					refusal = fmt.Errorf("%s loses publication completion", fn.Name.Name)
					continue
				}
				record, recordOK := bind.Lhs[0].(*ast.Ident)
				cause, causeOK := bind.Lhs[1].(*ast.Ident)
				if !recordOK || !causeOK || record.Name == "_" || cause.Name == "_" {
					refusal = fmt.Errorf("%s discards the published record or failure", fn.Name.Name)
					continue
				}
				var result ast.Expr
				propagated := false
				switch next := block.List[index+1].(type) {
				case *ast.AssignStmt:
					if len(next.Lhs) == 1 && len(next.Rhs) == 1 {
						result = next.Rhs[0]
						if returned, ok := next.Lhs[0].(*ast.Ident); ok {
							propagated = returned.Name == cause.Name
							if fn.Type.Results != nil {
								for _, field := range fn.Type.Results.List {
									for _, name := range field.Names {
										propagated = propagated || returned.Name == name.Name
									}
								}
							}
						}
					}
				case *ast.IfStmt:
					if completion, ok := next.Init.(*ast.AssignStmt); ok && len(completion.Lhs) == 1 && len(completion.Rhs) == 1 {
						result = completion.Rhs[0]
						returned, ok := completion.Lhs[0].(*ast.Ident)
						propagated = ok && returned.Name == cause.Name
					}
				case *ast.ReturnStmt:
					if len(next.Results) == 1 {
						result, propagated = next.Results[0], true
					}
				}
				if result == nil || !propagated {
					refusal = fmt.Errorf("%s has conditional or missing common reporting", fn.Name.Name)
					continue
				}
				report, ok := result.(*ast.CallExpr)
				if !ok || (rgaS0CallName(report) != "ReportCoverageStatus" && !strings.HasSuffix(rgaS0CallName(report), ".ReportCoverageStatus")) ||
					len(report.Args) != 3 {
					refusal = fmt.Errorf("%s bypasses the common coverage reporter", fn.Name.Name)
					continue
				}
				value, valueOK := report.Args[1].(*ast.Ident)
				failureExpr := report.Args[2]
				joined := false
				if call, ok := failureExpr.(*ast.CallExpr); ok && rgaS0CallName(call) == "errors.Join" && len(call.Args) >= 2 {
					failureExpr = call.Args[len(call.Args)-1]
					joined = true
				}
				failure, failureOK := failureExpr.(*ast.Ident)
				needsPrimary := fn.Name.Name == "RunImplement" || fn.Name.Name == "RefreshAfterAccept" ||
					fn.Name.Name == "runEditWithObservation" || fn.Name.Name == "coverageFinalizer"
				if !valueOK || !failureOK || value.Name != record.Name || failure.Name != cause.Name ||
					(needsPrimary && !joined) {
					refusal = fmt.Errorf("%s reports a different record or drops its failure", fn.Name.Name)
					continue
				}
				completions++
			}
			return true
		})
		if refusal != nil {
			return refusal
		}
		if publications != completions {
			return fmt.Errorf("%s does not report every publication through the common completion", fn.Name.Name)
		}
		primaryCauses := map[string][]string{
			"RunImplement":           {"stateErr", "provenanceErr", "coverageErr"},
			"RefreshAfterAccept":     {"retErr", "coverageErr"},
			"runEditWithObservation": {"editErr", "observationErr", "coverageErr"},
			"coverageFinalizer":      {"primary", "err"},
		}[fn.Name.Name]
		if publications > 0 && len(primaryCauses) > 0 {
			if err := rgaS5CallerClosureSource(src, fn.Name.Name, primaryCauses); err != nil {
				return err
			}
		}
	}
	return nil
}

func TestRGAS4CoverageCompletionGuardAndSensitivity(t *testing.T) {
	for path := range rgaS4ProducerFunctions {
		if err := rgaS4CoverageCompletionCalls(rgaS0ReadRepoFile(t, path)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	src := rgaS0ReadRepoFile(t, "internal/cli/producer_observation.go")
	line := "return workflow.ReportCoverageStatus(statusWriter, coverage, errors.Join(primary, err))"
	for _, tc := range []struct{ name, old, replacement string }{
		{"discarded-result", "coverage, err := workflow.PublishCoverage", "_, err := workflow.PublishCoverage"},
		{"missing-report", line, "_ = coverage"},
		{"generated-only-report", line, "if in.Autogen != nil && in.Autogen.Action == workflow.AutogenGenerated { " + line + " }"},
		{"wrong-record", line, "return workflow.ReportCoverageStatus(statusWriter, previousCoverage, errors.Join(primary, err))"},
		{"dropped-publication-failure", line, "return workflow.ReportCoverageStatus(statusWriter, coverage, errors.Join(primary, nil))"},
		{"dropped-primary-failure", line, "return workflow.ReportCoverageStatus(statusWriter, coverage, err)"},
		{"nil-primary-failure", line, "return workflow.ReportCoverageStatus(statusWriter, coverage, errors.Join(nil, err))"},
		{"reversed-primary-failure", line, "return workflow.ReportCoverageStatus(statusWriter, coverage, errors.Join(err, primary))"},
		{"discarded-reporting-result", line, "workflow.ReportCoverageStatus(statusWriter, coverage, errors.Join(primary, err))"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutated := strings.Replace(src, tc.old, tc.replacement, 1)
			if mutated == src || rgaS4CoverageCompletionCalls(mutated) == nil {
				t.Fatalf("common completion validator accepted %s", tc.name)
			}
		})
	}
	t.Run("aliased-publisher-without-report", func(t *testing.T) {
		mutated := strings.Replace(src, "workflow.PublishCoverage", "wf.PublishCoverage", 1)
		mutated = strings.Replace(mutated, line, "", 1)
		if rgaS4CoverageCompletionCalls(mutated) == nil {
			t.Fatal("an import alias hid a missing common completion")
		}
	})
}
