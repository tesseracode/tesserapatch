package workflow

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"reflect"
	"sort"
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
		body *ast.FuncDecl
		pkg  string
		file string
	}
	functions := map[string]function{}
	sites := map[string][]string{}
	for path, src := range sources {
		file, err := rgaS0Parse(path, src)
		if err != nil {
			return err
		}
		bound, _, err := rgaS0ScanWriteArtifact(path, src)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				key := file.Name.Name + "." + fn.Name.Name
				functions[key] = function{fn, file.Name.Name, path}
				for _, site := range bound {
					if site.Func == rgaS0EnclosingName(fn) {
						sites[key] = append(sites[key], site.key())
					}
				}
			}
		}
	}
	owners := map[string]map[string]bool{}
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
		var visit func(string)
		visit = func(key string) {
			if seen[key] || (key == "workflow.RunImplement" && producer != "implement") {
				return
			}
			// Only P1/P2 supply a recipe plan to the typed publisher;
			// the other producers preserve their recipe. This distinguishes
			// actual input flow from a context-free common-helper graph.
			if key == "workflow.publishRecordRecipePlan" && !suppliesRecipePlan {
				return
			}
			seen[key] = true
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
			ast.Inspect(fn.body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				name := rgaS0CallName(call)
				if strings.HasPrefix(name, "s.") {
					name = "store." + strings.TrimPrefix(name, "s.")
				} else if !strings.Contains(name, ".") {
					name = fn.pkg + "." + name
				}
				visit(name)
				return true
			})
		}
		for _, root := range roots {
			if _, ok := functions[root]; !ok {
				return fmt.Errorf("producer root missing: %s", root)
			}
			visit(root)
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
					if owner != "PublishCoverage" && (name == "BuildRecipeCoverage" || name == "EncodeRecipeCoverage") {
						refusal = fmt.Errorf("coverage derivation/encoding outside the primary publisher: %s", owner)
					}
					if selector, ok := call.Fun.(*ast.SelectorExpr); ok && owner != "PublishCoverage" &&
						(selector.Sel.Name == "WriteArtifact" || selector.Sel.Name == "WriteArtifactAtomic") {
						refusal = fmt.Errorf("alternate artifact writer in the publisher file: %s", owner)
					}
				}
			}
			if id, ok := n.(*ast.Ident); ok {
				coverageSymbol := strings.Contains(id.Name, "Coverage") || id.Name == "coverageFinalizer"
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
						"ErrCoveragePublication":
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
				if value, valid := rgaS0StringLit(literal); valid && value == "recipe-coverage.json" && !publisher {
					refusal = fmt.Errorf("producer owns a private coverage artifact")
				}
			}
			return true
		})
		if refusal != nil {
			return refusal
		}
	}
	if !publisher {
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
				if name, ok := rgaS0StringLit(call.Args[1]); ok && name == "recipe-coverage.json" {
					order = append(order, "coverage-write")
				}
			}
		case "s.WriteArtifact", "os.WriteFile":
			order = append(order, "non-atomic-write")
		}
		return true
	})
	want := "BuildRecipeCoverage>EncodeRecipeCoverage>publishRecordRecipePlan>AppendPatchGenerationForFeature>coverage-write"
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
		{"unregistered-producer-chain", "internal/cli/cobra.go", "package cli\nfunc other(){workflow.PublishCoverage(nil,workflow.CoveragePublicationInput{})}"},
		{"private-producer-codec", "internal/cli/cobra.go", "package cli\nfunc recordCmd(){workflow.EncodeRecipeCoverage(c)}"},
		{"alternate-publisher-entry", rel, src + "\nfunc alternatePublisher(){ s.WriteArtifactAtomic(slug, \"recipe-coverage.json\", \"{}\") }\n"},
		{"alternate-publisher-encoder", rel, src + "\nfunc alternateEncoder(){ EncodeRecipeCoverage(c) }\n"},
		{"non-atomic-publication", rel, strings.Replace(src, `s.WriteArtifactAtomic(`, `s.WriteArtifact(`, 1)},
		{"publication-before-recipe", rel, strings.Replace(src, "data, err := EncodeRecipeCoverage(c)", "s.WriteArtifactAtomic(in.Observation.Slug, \"recipe-coverage.json\", \"{}\")\n data, err := EncodeRecipeCoverage(c)", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if rgaS0CoveragePhaseSource(tc.path, tc.source) == nil {
				t.Fatalf("phase validator accepted %s", tc.name)
			}
		})
	}
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
		line := "\t_, coverageErr := PublishCoverage(s, publication)\n"
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
			`_, coverageErr := PublishCoverage(s, publication)
	stateErr := s.MarkFeatureState`,
		},
		{
			"publication-only-on-state-success",
			`_, coverageErr := PublishCoverage(s, publication)`,
			`var coverageErr error
	if stateErr == nil { _, coverageErr = PublishCoverage(s, publication) }`,
		},
		{
			"literal-nil-success-finalizer",
			`_, coverageErr := PublishCoverage(s, publication)`,
			`if stateErr != nil { return stateErr }
	_, coverageErr := PublishCoverage(s, publication)`,
		},
		{
			"publication-failure-discarded",
			`return errors.Join(stateErr, provenanceErr, coverageErr)`,
			`return stateErr`,
		},
		{
			"reversed-error-precedence",
			`return errors.Join(stateErr, provenanceErr, coverageErr)`,
			`return errors.Join(coverageErr, provenanceErr, stateErr)`,
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
			for _, method := range []string{"WriteArtifactAtomic", "ReadFeatureFile"} {
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
