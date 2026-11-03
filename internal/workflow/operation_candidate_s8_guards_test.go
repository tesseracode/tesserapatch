package workflow

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"
	"testing"
)

func rocS8CoverageSource(rel, src string) error {
	file, err := rgaS0Parse(rel, src)
	if err != nil {
		return err
	}
	allowed := map[string]map[string]bool{
		"acceptOperationCandidate":                               {"PublishCoverage": true},
		"operationCandidatePublication":                          {"CoveragePublicationInput": true, "CoverageArtifact": true, "CoverageEvents": true},
		"(*operationCandidateTransaction).publishInputs":         {"CoveragePublicationInput": true, "RecipeCoverage": true, "CoverageComplete": true, "CoverageStatus": true},
		"validateOperationCandidateBaseConsistency":              {"RecipeCoverage": true},
		"(*operationCandidateTransaction).validatePublishedBase": {"DecodeRecipeCoverage": true},
		"(*operationCandidateTransaction).failed":                {"ErrCoveragePublication": true},
	}
	checkOwner := func(owner string, source ast.Node) error {
		var refusal error
		ast.Inspect(source, func(node ast.Node) bool {
			if id, ok := node.(*ast.Ident); ok && (strings.Contains(id.Name, "Coverage") || strings.Contains(id.Name, "CaptureEvent")) {
				if !allowed[owner][id.Name] {
					refusal = fmt.Errorf("unregistered S8 coverage use %s:%s:%s", rel, owner, id.Name)
				}
			}
			if expression, ok := node.(ast.Expr); ok {
				value, constant := rgaS0ConstantString(expression)
				if constant && (strings.Contains(value, "recipe-coverage.json") || strings.Contains(value, "recipe-capture-event.json")) {
					if owner != "operationCandidateFeatureFiles" && owner != "(*operationCandidateTransaction).validatePublishedBase" {
						refusal = fmt.Errorf("S8 evidence artifact outside the registry/base reader")
					}
				}
			}
			return true
		})
		return refusal
	}
	for _, decl := range file.Decls {
		switch node := decl.(type) {
		case *ast.FuncDecl:
			if err := checkOwner(rgaS0EnclosingName(node), node); err != nil {
				return err
			}
		case *ast.GenDecl:
			for _, spec := range node.Specs {
				owner := ""
				if value, ok := spec.(*ast.ValueSpec); ok && len(value.Names) == 1 && value.Names[0].Name == "operationCandidateFeatureFiles" {
					owner = "operationCandidateFeatureFiles"
				}
				if err := checkOwner(owner, spec); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func rocS8Calls(node ast.Node, names []string) []string {
	var calls []string
	ast.Inspect(node, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			name := rocS6NodeText(call.Fun)
			if slices.Contains(names, name) {
				calls = append(calls, name)
			}
		}
		return true
	})
	return calls
}

func rocS8RequireCallOrder(file *ast.File, function string, want []string) error {
	fn := rgaS0FuncBody(file, function)
	if fn == nil {
		return fmt.Errorf("missing S8 function %s", function)
	}
	if got := rocS8Calls(fn.Body, want); !slices.Equal(got, want) {
		return fmt.Errorf("S8 transaction call order in %s: %v; want %v", function, got, want)
	}
	return nil
}

func rocS8AcceptanceSources(sources map[string]string) error {
	files := map[string]*ast.File{}
	for name, source := range sources {
		file, err := rgaS0Parse(name, source)
		if err != nil {
			return err
		}
		files[name] = file
		if name == "reconcile.go" || name == "recipe_coverage_publish.go" {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			owner := rgaS0EnclosingName(fn)
			var refusal error
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				callee := rocS6NodeText(call.Fun)
				for _, forbidden := range []string{"AcceptShadow", "CopyShadowToReal", "RefreshAfterAccept", "ForwardApply",
					"ReverseApply", "SaveFeatureStatus", "WriteArtifact", "WriteFeatureFile", "WriteFile", "RemoveAll", "CreateTemp"} {
					if strings.Contains(callee, forbidden) {
						if forbidden == "RemoveAll" && owner == "(*operationCandidateTransaction).removePrunedWorktree" && callee == "parent.RemoveAll" {
							continue
						}
						refusal = fmt.Errorf("acceptance reached unjournaled/legacy mutation %s", callee)
					}
				}
				if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
					if name == "operation_candidate_recovery.go" && (selector.Sel.Name == "CaptureAcceptanceOwnership" ||
						selector.Sel.Name == "OpenExistingCandidateRoot" || selector.Sel.Name == "CreateCandidateRoot") {
						refusal = fmt.Errorf("recovery attempted to mint fresh cleanup ownership")
					}
					allowed := true
					switch selector.Sel.Name {
					case "Write", "Chmod":
						allowed = owner == "(*operationCandidateDisk).replace"
					case "Rename":
						allowed = owner == "(*operationCandidateDisk).replace" || owner == "(*operationCandidateTransaction).relocatePruneWorktree"
					case "Remove":
						allowed = owner == "(*operationCandidateDisk).replace" || owner == "(*operationCandidateDisk).removeFile" ||
							owner == "(*operationCandidateDisk).removeEmptyDirectory"
					case "Mkdir":
						allowed = owner == "(*operationCandidateDisk).parent"
					case "OpenFile":
						allowed = owner == "(*operationCandidateDisk).replace" || owner == "(*operationCandidateDisk).read"
					case "replace":
						if callee != "t.replace" {
							allowed = (callee == "t.disk.replace" && (owner == "(*operationCandidateTransaction).replace" ||
								owner == "(*operationCandidateTransaction).cleanupTransactionData"))
							if owner == "(*operationCandidateTransaction).replace" {
								var args []string
								for _, arg := range call.Args {
									args = append(args, rocS6NodeText(arg))
								}
								allowed = allowed && slices.Equal(args, []string{"relative", "data", "mode", "t.directory"})
							}
							if owner == "(*operationCandidateTransaction).cleanupTransactionData" {
								allowed = allowed && len(call.Args) == 4 && rocS6NodeText(call.Args[0]) == "journalPath" &&
									rocS6NodeText(call.Args[3]) == "t.directory"
							}
						} else {
							allowed = slices.Contains([]string{"writePublicationFile", "snapshot", "prepareJournal", "applyLive",
								"markCommitted", "acceptState", "rollback", "cleanupCommitted"}, fn.Name.Name)
						}
					case "removeFile":
						targets := map[string][]string{
							"resumeOperationCandidateRecovery": {`path.Join(directory, ".accept-write")`},
							"rollback":                         {"target"},
							"cleanupTransactionData":           {`path.Join(t.directory, ".accept-write")`, "journalPath"},
							"removeKnownDirectory":             {"path.Join(name, entry.Name())"},
						}
						allowed = len(call.Args) == 1 && slices.Contains(targets[fn.Name.Name], rocS6NodeText(call.Args[0]))
					case "removeEmptyDirectory":
						allowed = fn.Name.Name == "rollback" || fn.Name.Name == "removeKnownDirectory"
					}
					if !allowed {
						refusal = fmt.Errorf("unregistered acceptance write: %s %s", owner, callee)
					}
				}
				return true
			})
			if refusal != nil {
				return refusal
			}
		}
	}
	accept := files["operation_candidate_acceptance.go"]
	transaction := files["operation_candidate_transaction.go"]
	recovery := files["operation_candidate_recovery.go"]
	publisher := files["recipe_coverage_publish.go"]
	actions := files["operation_candidate_actions.go"]
	for name, file := range map[string]*ast.File{"acceptance": accept, "transaction": transaction, "recovery": recovery, "publisher": publisher, "actions": actions} {
		if file == nil {
			return fmt.Errorf("missing %s source", name)
		}
	}
	if err := rocS8RequireCallOrder(actions, "AcceptOperationCandidate", []string{"acceptOperationCandidate"}); err != nil {
		return err
	}
	if err := rocS8RequireCallOrder(actions, "ResumeOperationCandidateRecovery", []string{"resumeOperationCandidateRecovery"}); err != nil {
		return err
	}
	want := []string{"acquireOperationCandidateFeatureLock", "candidatelock.TryAcquire", "operationCandidateRecoveryIDs",
		"s.LoadFeatureStatus", "candidate.DecodeCandidateArtifact", "candidate.DecodeCandidateState",
		"assessOperationCandidateEligibility", "candidate.DescribeDerivation", "candidate.ComputeCandidateID",
		"candidate.ValidateExistingCandidate", "matchOperationCandidateResult", "candidate.RematerializeForAcceptance",
		"RunTestCommandInShadow", "candidate.ValidateExistingCandidate", "matchOperationCandidateResult",
		"assessOperationCandidateEligibility", "candidate.DecodeCandidateArtifact", "candidate.DecodeCandidateState", "candidate.AcceptanceChanges",
		"operationCandidateAncestor", "transaction.checkCAS", "operationCandidatePublication", "PublishCoverage",
		"transaction.snapshot", "transaction.prepareJournal", "transaction.applyLive", "PublishCoverage",
		"transaction.acceptState", "transaction.validatePublishedBase", "transaction.markCommitted", "transaction.cleanupCommitted"}
	if err := rocS8RequireCallOrder(accept, "acceptOperationCandidate", want); err != nil {
		return err
	}
	if err := rocS8RequireCallOrder(transaction, "(*operationCandidateTransaction).replace", []string{"t.authorizeWrite", "t.disk.replace"}); err != nil {
		return err
	}
	fn := rgaS0FuncBody(transaction, "(*operationCandidateTransaction).applyLive")
	if fn == nil || len(fn.Body.List) != 3 {
		return fmt.Errorf("CAS recheck must directly precede live loop")
	}
	check, ok := fn.Body.List[0].(*ast.IfStmt)
	if !ok || check.Init == nil || len(rocS8Calls(check.Init, []string{"t.checkCAS"})) != 1 {
		return fmt.Errorf("first live mutation lacks its CAS recheck")
	}
	recheck := false
	ast.Inspect(check.Init, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok && rocS6NodeText(call.Fun) == "t.checkCAS" &&
			len(call.Args) == 1 && rocS6NodeText(call.Args[0]) == "true" {
			recheck = true
		}
		return true
	})
	loop, ok := fn.Body.List[1].(*ast.RangeStmt)
	if !ok || rocS6NodeText(loop.X) != "t.changes" || !recheck ||
		!slices.Equal(rocS8Calls(loop.Body, []string{"t.replace"}), []string{"t.replace"}) {
		return fmt.Errorf("first live mutation is not the immediately checked change set")
	}
	ancestor := rgaS0FuncBody(accept, "operationCandidateAncestor")
	if ancestor == nil {
		return fmt.Errorf("missing ancestry check")
	}
	ancestorCommand, equality := false, false
	ast.Inspect(ancestor, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok && rocS6NodeText(call.Fun) == "exec.Command" {
			var args []string
			for _, arg := range call.Args {
				args = append(args, rocS6NodeText(arg))
			}
			ancestorCommand = slices.Equal(args, []string{`"git"`, `"merge-base"`, `"--is-ancestor"`, "upstream", "head"})
		}
		if binary, ok := node.(*ast.BinaryExpr); ok && (binary.Op == token.EQL || binary.Op == token.NEQ) {
			a, b := rocS6NodeText(binary.X), rocS6NodeText(binary.Y)
			equality = equality || (a == "head" && b == "upstream") || (a == "upstream" && b == "head")
		}
		return true
	})
	if !ancestorCommand || equality {
		return fmt.Errorf("acceptance must use ancestry, not HEAD equality")
	}
	var inventory, steps, constants []string
	firstStep := ""
	for _, decl := range transaction.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != 1 {
				continue
			}
			if gen.Tok == token.CONST {
				constants = append(constants, value.Names[0].Name)
				if len(value.Values) == 1 && value.Names[0].Name == "candidateStepLocks" {
					firstStep = rocS6NodeText(value.Values[0])
				}
			}
			if len(value.Values) != 1 {
				continue
			}
			literal, ok := value.Values[0].(*ast.CompositeLit)
			if !ok {
				continue
			}
			for _, element := range literal.Elts {
				switch value.Names[0].Name {
				case "operationCandidateFeatureFiles":
					name, _ := rgaS0StringLit(element)
					inventory = append(inventory, name)
				case "operationCandidatePreparedSteps":
					steps = append(steps, rocS6NodeText(element))
				}
			}
		}
	}
	if !slices.Equal(inventory, []string{"artifacts/post-apply.patch", "artifacts/apply-recipe.json", "artifacts/recipe-provenance.json",
		"artifacts/patch-generations.json", "artifacts/recipe-capture-event.json", "artifacts/recipe-coverage.json", "status.json"}) ||
		!slices.Equal(steps, []string{"candidateStepLocks", "candidateStepRecoveryCheck", "candidateStepPending", "candidateStepRederive",
			"candidateStepRematerialize", "candidateStepPreconditions", "candidateStepStage", "candidateStepSnapshot", "candidateStepJournal",
			"candidateStepApply", "candidateStepPublish", "candidateStepAccepted", "candidateStepCommit", "candidateStepCleanup"}) {
		return fmt.Errorf("closed seven-file/fourteen-step inventory changed")
	}
	if firstStep != "iota + 1" || !slices.Equal(constants, steps) {
		return fmt.Errorf("transaction step numbers no longer match the fourteen-step contract")
	}
	if err := rocS8RequireCallOrder(transaction, "(*operationCandidateTransaction).snapshot",
		[]string{"t.disk.read", "root.CaptureAcceptanceOwnership", "t.disk.read", "t.disk.read", "t.validateJournal", "t.replace"}); err != nil {
		return err
	}
	snapshotOwnership := false
	ast.Inspect(rgaS0FuncBody(transaction, "(*operationCandidateTransaction).snapshot"), func(node ast.Node) bool {
		if literal, ok := node.(*ast.CompositeLit); ok && rocS6NodeText(literal.Type) == "operationCandidateJournal" {
			for _, element := range literal.Elts {
				if field, ok := element.(*ast.KeyValueExpr); ok && rocS6NodeText(field.Key) == "PruneOwnership" {
					snapshotOwnership = rocS6NodeText(field.Value) == "&ownership"
				}
			}
		}
		return true
	})
	if !snapshotOwnership {
		return fmt.Errorf("initial acceptance journal lost its original cleanup identities")
	}
	if err := rocS8RequireCallOrder(recovery, "resumeOperationCandidateRecovery",
		[]string{"acquireOperationCandidateFeatureLock", "candidatelock.TryAcquire", "operationCandidateRecoveryIDs",
			"decodeOperationCandidateJournal", "transaction.validateJournal", "transaction.rollback", "transaction.cleanupCommitted"}); err != nil {
		return err
	}
	resume := rgaS0FuncBody(recovery, "resumeOperationCandidateRecovery")
	noMarkerOnly := false
	for _, stmt := range resume.Body.List {
		if branch, ok := stmt.(*ast.IfStmt); ok && rocS6NodeText(branch.Cond) == "!transaction.committed" &&
			slices.Equal(rocS8Calls(branch.Body, []string{"transaction.rollback", "transaction.cleanupCommitted"}), []string{"transaction.rollback"}) {
			noMarkerOnly = true
		}
	}
	if !noMarkerOnly {
		return fmt.Errorf("explicit recovery lost the no-marker-only rollback branch")
	}
	if err := rocS8RequireCallOrder(recovery, "(*operationCandidateTransaction).cleanupCommitted",
		[]string{"t.relocatePruneWorktree", "upstream.PruneAcceptanceWorktree", "t.removePrunedWorktree", "t.cleanupTransactionData"}); err != nil {
		return err
	}
	if err := rocS8RequireCallOrder(recovery, "(*operationCandidateTransaction).relocatePruneWorktree",
		[]string{"t.journal.PruneOwnership.Worktree.CheckRoot", "t.journal.PruneOwnership.Worktree.CheckRoot", "parent.Rename"}); err != nil {
		return err
	}
	if err := rocS8RequireCallOrder(recovery, "(*operationCandidateTransaction).removePrunedWorktree",
		[]string{"t.journal.PruneOwnership.Worktree.CheckRoot", "parent.RemoveAll"}); err != nil {
		return err
	}
	ownedRegistration := false
	ast.Inspect(rgaS0FuncBody(recovery, "(*operationCandidateTransaction).cleanupCommitted"), func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok && rocS6NodeText(call.Fun) == "upstream.PruneAcceptanceWorktree" {
			var args []string
			for _, arg := range call.Args {
				args = append(args, rocS6NodeText(arg))
			}
			ownedRegistration = slices.Equal(args, []string{"t.slug", "t.id", "t.featureLock", "t.globalLock",
				"*t.journal.PruneOwnership", "operationCandidateSyncDirectory"})
		}
		return true
	})
	if !ownedRegistration {
		return fmt.Errorf("registration prune lost original ownership or the real directory durability implementation")
	}
	prune := rgaS0FuncBody(recovery, "(*operationCandidateTransaction).removePrunedWorktree")
	if prune == nil {
		return fmt.Errorf("missing reserved worktree cleanup")
	}
	boundedParent, boundedRemove, fixedReservation := false, false, false
	for _, decl := range recovery.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if ok && len(value.Names) == 1 && value.Names[0].Name == "operationCandidatePruneDirectory" && len(value.Values) == 1 {
				name, _ := rgaS0StringLit(value.Values[0])
				fixedReservation = name == ".accept-prune"
			}
		}
	}
	ast.Inspect(prune.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch rocS6NodeText(call.Fun) {
		case "t.disk.parent":
			boundedParent = len(call.Args) == 2 && rocS6NodeText(call.Args[0]) == "path.Join(t.directory, operationCandidatePruneDirectory)" &&
				rocS6NodeText(call.Args[1]) == "false"
		case "parent.RemoveAll":
			boundedRemove = len(call.Args) == 1 && rocS6NodeText(call.Args[0]) == "leaf"
		}
		return true
	})
	if !boundedParent || !boundedRemove || !fixedReservation {
		return fmt.Errorf("recursive prune escaped its fixed reserved candidate leaf")
	}
	if source, ok := files["reconcile.go"]; ok {
		if got := rocS8Calls(source, []string{"ResumeOperationCandidateRecovery", "resumeOperationCandidateRecovery"}); len(got) != 0 {
			return fmt.Errorf("ordinary reconcile acquired automatic recovery")
		}
	}
	return rocS8PublisherVariant(publisher)
}

func rocS8PublisherVariant(file *ast.File) error {
	fn := rgaS0FuncBody(file, "PublishCoverage")
	if fn == nil {
		return fmt.Errorf("shared publisher is missing")
	}
	var variant *ast.IfStmt
	for _, stmt := range fn.Body.List {
		if branch, ok := stmt.(*ast.IfStmt); ok && branch.Init != nil &&
			rocS6NodeText(branch.Init) == "transaction := in.candidateAcceptance" && rocS6NodeText(branch.Cond) == "transaction != nil" {
			variant = branch
		}
	}
	if variant == nil {
		return fmt.Errorf("candidate variant did not use the single publisher")
	}
	var writes []string
	ast.Inspect(variant.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if rocS6NodeText(call.Fun) == "transaction.publishInputs" {
			writes = append(writes, "inputs")
		}
		if rocS6NodeText(call.Fun) == "transaction.writePublicationFile" && len(call.Args) > 0 {
			value, _ := rgaS0StringLit(call.Args[0])
			writes = append(writes, value)
		}
		return true
	})
	if !slices.Equal(writes, []string{"inputs", "artifacts/recipe-capture-event.json", "artifacts/recipe-coverage.json"}) {
		return fmt.Errorf("candidate publication lost E-before-C, C-last order: %v", writes)
	}
	return nil
}

func TestROCS8AcceptanceSourceInventoryAndSensitivity(t *testing.T) {
	sources := map[string]string{}
	for _, name := range []string{"operation_candidate_actions.go", "operation_candidate_acceptance.go", "operation_candidate_transaction.go",
		"operation_candidate_recovery.go", "operation_candidate_publication.go", "recipe_coverage_publish.go", "reconcile.go"} {
		sources[name] = rgaS0ReadRepoFile(t, "internal/workflow/"+name)
	}
	if err := rocS8AcceptanceSources(sources); err != nil {
		t.Fatal(err)
	}
	for _, mutant := range []struct{ file, from, to string }{
		{"operation_candidate_acceptance.go", "candidate.DescribeDerivation(", "trustStoredDerivation("},
		{"operation_candidate_acceptance.go", "candidate.RematerializeForAcceptance(", "AcceptShadow("},
		{"operation_candidate_acceptance.go", "candidate.RematerializeForAcceptance(", "gitutil.CopyShadowToReal("},
		{"operation_candidate_acceptance.go", "candidate.RematerializeForAcceptance(", "RefreshAfterAccept("},
		{"operation_candidate_acceptance.go", "transaction.checkCAS(false)", "skipCAS(false)"},
		{"operation_candidate_acceptance.go", `"--is-ancestor"`, `"--independent"`},
		{"operation_candidate_acceptance.go", "command := exec.Command", `if head == upstream { return nil }; command := exec.Command`},
		{"operation_candidate_transaction.go", "t.checkCAS(true)", "t.checkCAS(false)"},
		{"operation_candidate_transaction.go", `"artifacts/recipe-capture-event.json",`, ""},
		{"operation_candidate_transaction.go", "candidateStepCommit, candidateStepCleanup,", "candidateStepCleanup,"},
		{"operation_candidate_transaction.go", "iota + 1", "iota + 2"},
		{"operation_candidate_transaction.go", "return t.disk.replace(relative, data, mode, t.directory)", `return t.disk.replace("foreign.txt", data, mode, t.directory)`},
		{"operation_candidate_transaction.go", "root.CaptureAcceptanceOwnership(t.featureLock, t.globalLock)", "trustFreshDirectory()"},
		{"operation_candidate_transaction.go", "PruneOwnership: &ownership", "PruneOwnership: nil"},
		{"operation_candidate_recovery.go", "if !transaction.committed {", "if transaction.committed {"},
		{"operation_candidate_recovery.go", "t.disk.removeFile(target)", `t.disk.removeFile("foreign.txt")`},
		{"operation_candidate_recovery.go", "t.disk.replace(journalPath,", `t.disk.replace("foreign.txt",`},
		{"operation_candidate_recovery.go", "parent.RemoveAll(leaf)", `parent.RemoveAll(".")`},
		{"operation_candidate_recovery.go", "t.journal.PruneOwnership.Worktree.CheckRoot(held)", "trustFreshDirectory(held)"},
		{"operation_candidate_recovery.go", "t.journal.PruneOwnership.Worktree.CheckRoot(owned)", "trustFreshDirectory(owned)"},
		{"operation_candidate_recovery.go", "*t.journal.PruneOwnership, operationCandidateSyncDirectory", "freshOwnership, operationCandidateSyncDirectory"},
		{"operation_candidate_recovery.go", "*t.journal.PruneOwnership, operationCandidateSyncDirectory", "*t.journal.PruneOwnership, skipDirectorySync"},
		{"operation_candidate_recovery.go", `const operationCandidatePruneDirectory = ".accept-prune"`, `const operationCandidatePruneDirectory = "../../foreign"`},
		{"recipe_coverage_publish.go", `"artifacts/recipe-capture-event.json", eventData`, `"artifacts/recipe-coverage.json", eventData`},
		{"reconcile.go", "slugs, supersededWarnings, err := effectiveReconcileSlugs", "ResumeOperationCandidateRecovery(s, slugs[0]); slugs, supersededWarnings, err := effectiveReconcileSlugs"},
	} {
		t.Run(mutant.file+"/"+mutant.from, func(t *testing.T) {
			original := sources[mutant.file]
			sources[mutant.file] = strings.Replace(original, mutant.from, mutant.to, 1)
			defer func() { sources[mutant.file] = original }()
			if sources[mutant.file] == original {
				t.Fatal("negative fixture did not mutate its source")
			}
			if err := rocS8AcceptanceSources(sources); err == nil {
				t.Fatal("same acceptance validator accepted broken authority")
			}
		})
	}
	original := sources["operation_candidate_acceptance.go"]
	sources["operation_candidate_acceptance.go"] += "\nfunc unregistered(t *operationCandidateTransaction) { t.disk.replace(\"foreign.txt\", nil, 0644, t.directory) }\n"
	if err := rocS8AcceptanceSources(sources); err == nil {
		t.Fatal("actual unregistered rooted writer escaped the inventory")
	}
	sources["operation_candidate_acceptance.go"] = original
	original = sources["operation_candidate_recovery.go"]
	sources["operation_candidate_recovery.go"] += "\nfunc unregistered() { candidate.OpenExistingCandidateRoot(nil, \"feature\", \"rc_111111111111\") }\n"
	if err := rocS8AcceptanceSources(sources); err == nil {
		t.Fatal("recovery validator allowed fresh ownership to be inferred from a replacement worktree")
	}
	sources["operation_candidate_recovery.go"] = original
}

func TestROCS8CoverageRegistrationHasNoParallelPublisher(t *testing.T) {
	for _, name := range []string{"acceptance", "publication", "transaction", "recovery"} {
		rel := "internal/workflow/operation_candidate_" + name + ".go"
		source := rgaS0ReadRepoFile(t, rel)
		if err := rgaS0CoveragePhaseSource(rel, source); err != nil {
			t.Fatal(err)
		}
		for _, extra := range []string{
			"\nfunc unregistered() { PublishCoverage(nil, CoveragePublicationInput{}) }\n",
			"\nfunc unregistered() { EncodeRecipeCoverage(RecipeCoverage{}) }\n",
			"\nfunc unregistered() { write(\"artifacts/recipe-capture-event.json\") }\n",
			"\nfunc unregistered() { write(\"artifacts/recipe-\" + \"capture-event.json\") }\n",
			"\nfunc unregistered() { const prefix = \"artifacts/recipe-\"; const suffix = \"capture-event.json\"; const target = prefix + suffix; write(target) }\n",
			"\nfunc unregistered() { write(\"artifacts/recipe-\" + \"coverage.json\") }\n",
			"\nfunc unregistered() { const prefix = \"artifacts/recipe-\"; const suffix = \"coverage.json\"; const target = prefix + suffix; write(target) }\n",
		} {
			if err := rgaS0CoveragePhaseSource(rel, source+extra); err == nil {
				t.Fatal("S8 file registration exempted a parallel writer")
			}
		}
		if name == "transaction" {
			file, err := rgaS0Parse(rel, source)
			if err != nil {
				t.Fatal(err)
			}
			mutated := false
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.VAR {
					continue
				}
				for _, spec := range gen.Specs {
					value, ok := spec.(*ast.ValueSpec)
					if !ok || len(value.Names) != 1 || value.Names[0].Name != "operationCandidateFeatureFiles" {
						continue
					}
					start, end := int(gen.Pos())-1, int(gen.End())-1
					group := "var (\n" + strings.TrimPrefix(source[start:end], "var ") +
						"\nunregisteredEvidenceTarget = \"artifacts/recipe-coverage.json\"\n)"
					mutant := source[:start] + group + source[end:]
					if err := rgaS0CoveragePhaseSource(rel, mutant); err == nil {
						t.Fatal("an adjacent declaration inherited the evidence registry's ownership")
					}
					mutated = true
				}
			}
			if !mutated {
				t.Fatal("grouped declaration mutation anchor disappeared")
			}
		}
	}
}
