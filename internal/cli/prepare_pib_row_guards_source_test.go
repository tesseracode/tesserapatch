//go:build (linux && !android) || (darwin && !ios)

package cli

// Owning source-, string- and document-guard tests for the aggregate ledger.
//
// Every leaf here derives its input from the shipped repository (production Go
// source, shipped skills, SPEC.md, or the strings the product actually emits),
// asserts the guard accepts that input, and then mutates the *same* input and
// requires the *same* validator to reject it. None of them scans a row name and
// none of them is satisfied by "the scan ran".

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intent"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// pibGuardSources reads the named repo-relative files.
func pibGuardSources(t *testing.T, rels ...string) map[string]string {
	t.Helper()
	sources := make(map[string]string, len(rels))
	for _, rel := range rels {
		body, err := os.ReadFile(filepath.Join(avpRepoRoot(t), filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		sources[rel] = string(body)
	}
	return sources
}

func pibGuardClone(sources map[string]string) map[string]string {
	copied := make(map[string]string, len(sources))
	for name, body := range sources {
		copied[name] = body
	}
	return copied
}

// pibGuardFunctionBody returns the text of one top-level function declaration.
func pibGuardFunctionBody(source, signature string) (string, bool) {
	start := strings.Index(source, signature)
	if start < 0 {
		return "", false
	}
	rest := source[start:]
	end := strings.Index(rest[1:], "\nfunc ")
	if end < 0 {
		return rest, true
	}
	return rest[:end+1], true
}

// pibGuardEmittedStrings returns every refusal message and remediation the
// product can emit, derived by calling the shipped renderer over the shipped
// closed catalog rather than by reading a hand-kept list.
func pibGuardEmittedStrings() map[string]string {
	emitted := map[string]string{}
	for _, code := range s6RefusalCatalog {
		for _, mode := range []prepareMode{
			prepareModeGenerate, prepareModeManual, prepareModeRegenerate, prepareModeAbandon,
		} {
			message, remediation := prepareRefusalText(code, mode, "demo-feature", "")
			emitted[code+"/"+string(mode)+"/message"] = message
			emitted[code+"/"+string(mode)+"/remediation"] = remediation
		}
	}
	return emitted
}

// ---------------------------------------------------------------------------
// PIB-014 — the retired exit-4 surface.
// ---------------------------------------------------------------------------

func pibValidateExitFourRetired(sources map[string]string) error {
	if len(sources) == 0 {
		return fmt.Errorf("the exit-4 scan received no sources")
	}
	constructor := regexp.MustCompile(`ExitCodeError\{\s*Code:\s*4\b`)
	for name, body := range sources {
		if constructor.MatchString(body) {
			return fmt.Errorf("%s constructs the retired exit-4 envelope", name)
		}
		if strings.Contains(body, "reserved-surface") {
			return fmt.Errorf("%s reintroduces the retired reserved-surface refusal string", name)
		}
	}
	return nil
}

func TestPIBRowPIB014ExitFourRetiredSourceGuard(t *testing.T) {
	sources := pibGuardCLIProductionSources(t)
	if err := pibValidateExitFourRetired(sources); err != nil {
		t.Fatalf("PIB-014: the shipped prepare surface failed its own guard: %v", err)
	}
	mutated := pibGuardClone(sources)
	mutated["internal/cli/prepare.go"] = strings.Replace(
		mutated["internal/cli/prepare.go"],
		"return &ExitCodeError{\n\t\t\t\t\tCode:    1,",
		"return &ExitCodeError{\n\t\t\t\t\tCode:    4,",
		1,
	)
	if mutated["internal/cli/prepare.go"] == sources["internal/cli/prepare.go"] {
		t.Fatal("PIB-014: the exit-4 mutation anchor is missing")
	}
	if err := pibValidateExitFourRetired(mutated); err == nil {
		t.Fatal("PIB-014: the same guard accepted a reintroduced exit-4 construction")
	}
	revived := pibGuardClone(sources)
	revived["internal/cli/prepare.go"] += "\n// reserved-surface\n"
	if err := pibValidateExitFourRetired(revived); err == nil {
		t.Fatal("PIB-014: the same guard accepted the retired reserved-surface string")
	}
}

func pibGuardCLIProductionSources(t *testing.T) map[string]string {
	t.Helper()
	return pibGuardSources(t,
		"internal/cli/prepare.go",
		"internal/cli/prepare_publish.go",
	)
}

// ---------------------------------------------------------------------------
// PIB-044 — the §6.1 disposition table is total over the nine artifact states.
// ---------------------------------------------------------------------------

func pibValidateDispositionTotality(states []string, source string) error {
	if len(states) != 9 {
		return fmt.Errorf("the disposition domain has %d states, §6.1 fixes nine", len(states))
	}
	seen := map[string]bool{}
	for _, state := range states {
		if seen[state] {
			return fmt.Errorf("the disposition domain repeats %q", state)
		}
		seen[state] = true
	}
	body, found := pibGuardFunctionBody(source, "func prepareUnsafeArtifactState(state string) bool {")
	if !found {
		return fmt.Errorf("the disposition classifier was not found")
	}
	for _, state := range states {
		switch state {
		case intent.StatePresentNonempty, intent.StatePresentEmpty,
			intent.StateAbsent, intent.StateInvalidStructured:
			continue
		}
		if !strings.Contains(body, "intent."+pibGuardStateIdentifier(state)) {
			return fmt.Errorf("the classifier does not name the unsafe state %q", state)
		}
	}
	if strings.Contains(body, "default:\n\t\treturn true") {
		return fmt.Errorf("the classifier falls through to a default that admits unknown states")
	}
	return nil
}

func pibGuardStateIdentifier(state string) string {
	switch state {
	case intent.StateSymlinkRefused:
		return "StateSymlinkRefused"
	case intent.StateNotRegular:
		return "StateNotRegular"
	case intent.StateUnreadable:
		return "StateUnreadable"
	case intent.StateOversize:
		return "StateOversize"
	case intent.StateUnstable:
		return "StateUnstable"
	}
	return "State" + state
}

func TestPIBRowPIB044DispositionTableTotality(t *testing.T) {
	source := pibGuardSources(t, "internal/cli/prepare_publish.go")["internal/cli/prepare_publish.go"]
	states := intent.ArtifactStates()
	if err := pibValidateDispositionTotality(states, source); err != nil {
		t.Fatalf("PIB-044: the shipped disposition table failed its own guard: %v", err)
	}
	widened := append(append([]string(nil), states...), "present-thin")
	if err := pibValidateDispositionTotality(widened, source); err == nil {
		t.Fatal("PIB-044: the guard accepted a tenth artifact state")
	}
	narrowed := append([]string(nil), states[:len(states)-1]...)
	if err := pibValidateDispositionTotality(narrowed, source); err == nil {
		t.Fatal("PIB-044: the guard accepted a domain missing one state")
	}
	dropped := strings.Replace(source,
		"case intent.StateSymlinkRefused, intent.StateNotRegular, intent.StateUnreadable,\n\t\tintent.StateOversize, intent.StateUnstable:",
		"case intent.StateNotRegular, intent.StateUnreadable,\n\t\tintent.StateOversize, intent.StateUnstable:",
		1,
	)
	if dropped == source {
		t.Fatal("PIB-044: the classifier mutation anchor is missing")
	}
	if err := pibValidateDispositionTotality(states, dropped); err == nil {
		t.Fatal("PIB-044: the guard accepted a classifier that forgot an unsafe state")
	}
}

// ---------------------------------------------------------------------------
// PIB-059 — the `--manual` code path constructs no provider and writes no
// artifact.
//
// Kind `S`: the static half is an AST control-flow proof, not a token scan. It
// evaluates the shipped mode guards under `options.mode == manual` and requires
// every provider-construction call site inside the mutating entry point to be
// unreachable under that assignment — and reachable under `generate`, so the
// analysis cannot pass by declaring everything dead. The runtime half installs
// a real provider-construction spy on the shipped `prepareLoadProvider` seam
// and drives the real CLI, so the "no artifact write" half of the row is an
// observation of the tree rather than a reading of the source.
// ---------------------------------------------------------------------------

const (
	pibProviderConstruction = "prov, provCfg = prepareLoadProvider(repoStore)"
	pibProviderGuard        = "if options.mode == prepareModeGenerate || options.mode == prepareModeRegenerate {"
)

// pibPrepareModeConstants binds the shipped mode identifiers to the shipped
// mode values, so the evaluator decides reachability from the product's own
// vocabulary instead of a copy of it.
var pibPrepareModeConstants = map[string]prepareMode{
	"prepareModeCheck":      prepareModeCheck,
	"prepareModeGenerate":   prepareModeGenerate,
	"prepareModeManual":     prepareModeManual,
	"prepareModeRegenerate": prepareModeRegenerate,
	"prepareModeAbandon":    prepareModeAbandon,
}

// pibModeComparison recognises `options.mode <op> <shipped mode constant>` in
// either operand order and returns the mode the comparison names.
func pibModeComparison(binary *ast.BinaryExpr) (prepareMode, bool) {
	for _, pair := range [][2]ast.Expr{{binary.X, binary.Y}, {binary.Y, binary.X}} {
		selector, ok := pair[0].(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "mode" {
			continue
		}
		if receiver, ok := selector.X.(*ast.Ident); !ok || receiver.Name != "options" {
			continue
		}
		identifier, ok := pair[1].(*ast.Ident)
		if !ok {
			continue
		}
		if named, known := pibPrepareModeConstants[identifier.Name]; known {
			return named, true
		}
	}
	return "", false
}

// pibEvalModeCondition evaluates one shipped guard condition under an
// assignment of `options.mode`. The second result reports whether the mode
// decides the condition at all: a condition that never mentions `options.mode`
// excludes no mode and must not be read as if it did.
func pibEvalModeCondition(condition ast.Expr, mode prepareMode) (bool, bool) {
	switch typed := condition.(type) {
	case *ast.ParenExpr:
		return pibEvalModeCondition(typed.X, mode)
	case *ast.UnaryExpr:
		if typed.Op == token.NOT {
			value, decided := pibEvalModeCondition(typed.X, mode)
			if !decided {
				return false, false
			}
			return !value, true
		}
	case *ast.BinaryExpr:
		switch typed.Op {
		case token.LOR:
			left, leftDecided := pibEvalModeCondition(typed.X, mode)
			right, rightDecided := pibEvalModeCondition(typed.Y, mode)
			if leftDecided && left || rightDecided && right {
				return true, true
			}
			if leftDecided && rightDecided {
				return false, true
			}
			return false, false
		case token.LAND:
			left, leftDecided := pibEvalModeCondition(typed.X, mode)
			right, rightDecided := pibEvalModeCondition(typed.Y, mode)
			if leftDecided && !left || rightDecided && !right {
				return false, true
			}
			if leftDecided && rightDecided {
				return true, true
			}
			return false, false
		case token.EQL, token.NEQ:
			named, ok := pibModeComparison(typed)
			if !ok {
				return false, false
			}
			if typed.Op == token.NEQ {
				return named != mode, true
			}
			return named == mode, true
		}
	}
	return false, false
}

// pibASTParents records the parent of every node beneath root, which is what
// lets a call site be walked back up through its enclosing `if` branches.
func pibASTParents(root ast.Node) map[ast.Node]ast.Node {
	parents := map[ast.Node]ast.Node{}
	stack := []ast.Node{}
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return false
		}
		if len(stack) > 0 {
			parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	return parents
}

// pibCallSites returns every direct call of the named package-local function
// beneath root, in source order.
func pibCallSites(root ast.Node, callee string) []*ast.CallExpr {
	sites := []*ast.CallExpr{}
	ast.Inspect(root, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if identifier, ok := call.Fun.(*ast.Ident); ok && identifier.Name == callee {
			sites = append(sites, call)
		}
		return true
	})
	return sites
}

// pibModeReachable reports whether a run in `mode` can reach the site: it walks
// the enclosing `if` chain and answers false as soon as the site sits in a
// branch the mode decides against. Conditions the mode does not decide leave
// reachability untouched, so the analysis only ever over-approximates reach.
func pibModeReachable(site ast.Node, parents map[ast.Node]ast.Node, mode prepareMode) bool {
	node := site
	for {
		parent, ok := parents[node]
		if !ok {
			return true
		}
		if branch, isIf := parent.(*ast.IfStmt); isIf {
			if value, decided := pibEvalModeCondition(branch.Cond, mode); decided {
				if node == ast.Node(branch.Body) && !value {
					return false
				}
				if branch.Else != nil && node == branch.Else && value {
					return false
				}
			}
		}
		node = parent
	}
}

func pibValidateManualPathIsInert(source string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "prepare_publish.go", source, 0)
	if err != nil {
		return fmt.Errorf("the mutating publication source does not parse: %v", err)
	}
	var entry *ast.FuncDecl
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || function.Body == nil {
			continue
		}
		if function.Name.Name == "runPreparePublish" {
			entry = function
		}
	}
	if entry == nil {
		return fmt.Errorf("the mutating publication entry point was not found")
	}
	parents := pibASTParents(entry.Body)
	providers := pibCallSites(entry.Body, "prepareLoadProvider")
	if len(providers) == 0 {
		return fmt.Errorf("no provider construction was found in the mutating entry point")
	}
	for _, site := range providers {
		if pibModeReachable(site, parents, prepareModeManual) {
			return fmt.Errorf(
				"a provider construction at offset %d is reachable from --manual", int(site.Pos()))
		}
		if !pibModeReachable(site, parents, prepareModeGenerate) {
			return fmt.Errorf(
				"the provider construction at offset %d is unreachable from the mode that needs it",
				int(site.Pos()))
		}
	}
	statusOnly := pibCallSites(entry.Body, "publishPrepareStatusOnly")
	generation := pibCallSites(entry.Body, "generatePrepareBundle")
	if len(statusOnly) != 1 || len(generation) != 1 {
		return fmt.Errorf(
			"the manual and generation seams are %d and %d call sites, want one each",
			len(statusOnly), len(generation))
	}
	if statusOnly[0].Pos() > generation[0].Pos() || statusOnly[0].Pos() > providers[0].Pos() {
		return fmt.Errorf("the status-only publication does not precede provider construction")
	}
	return nil
}

// pibReplaceProviderGuard rewrites the one guard that dominates the shipped
// provider construction — the last occurrence before the construction — so a
// delta cannot silently land on an identically worded guard elsewhere.
func pibReplaceProviderGuard(source, replacement string) string {
	site := strings.Index(source, pibProviderConstruction)
	if site < 0 {
		return source
	}
	anchor := strings.LastIndex(source[:site], pibProviderGuard)
	if anchor < 0 {
		return source
	}
	return source[:anchor] + replacement + source[anchor+len(pibProviderGuard):]
}

func TestPIBRowPIB059ManualPathConstructsNoProvider(t *testing.T) {
	source := pibGuardSources(t, "internal/cli/prepare_publish.go")["internal/cli/prepare_publish.go"]
	if err := pibValidateManualPathIsInert(source); err != nil {
		t.Fatalf("PIB-059: the shipped manual path failed its own guard: %v", err)
	}
	widened := pibReplaceProviderGuard(source,
		"if options.mode == prepareModeGenerate || options.mode == prepareModeRegenerate ||\n"+
			"\t\toptions.mode == prepareModeManual {")
	if widened == source {
		t.Fatal("PIB-059: the provider-guard mutation anchor is missing")
	}
	if err := pibValidateManualPathIsInert(widened); err == nil {
		t.Fatal("PIB-059: the guard accepted a provider construction reachable from --manual")
	}
	broadened := pibReplaceProviderGuard(source, "if options.mode != prepareModeCheck {")
	if broadened == source {
		t.Fatal("PIB-059: the broadened-guard mutation anchor is missing")
	}
	if err := pibValidateManualPathIsInert(broadened); err == nil {
		t.Fatal("PIB-059: the guard accepted a mode test that no longer excludes --manual")
	}
	// Control: a second, correctly guarded provider construction is legal
	// neighbouring behaviour and must not be rejected.
	neighbour := strings.Replace(source, pibProviderConstruction,
		pibProviderConstruction+"\n\t\tif options.mode == prepareModeGenerate {\n\t\t\t"+
			pibProviderConstruction+"\n\t\t}", 1)
	if neighbour == source {
		t.Fatal("PIB-059: the neighbouring-construction control anchor is missing")
	}
	if err := pibValidateManualPathIsInert(neighbour); err != nil {
		t.Fatalf("PIB-059: the guard rejected a correctly guarded second construction: %v", err)
	}

	// The runtime half: a real provider-construction spy over a real CLI run.
	oldLoad := prepareLoadProvider
	constructions := 0
	t.Cleanup(func() { prepareLoadProvider = oldLoad })
	prepareLoadProvider = func(repoStore *store.Store) (provider.Provider, provider.Config) {
		constructions++
		return oldLoad(repoStore)
	}
	manualRoot, manualSlug := prepareS4Workspace(t, "PIB row 059 manual")
	prepareS4WriteReadyBundle(t, manualRoot, manualSlug, false)
	feature := filepath.Join(manualRoot, ".tpatch", "features", manualSlug)
	preimages := map[string][]byte{}
	for _, name := range []string{"request.md", "analysis.md", "spec.md", "exploration.md"} {
		body, readErr := os.ReadFile(filepath.Join(feature, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		preimages[name] = body
	}
	statusBefore, err := os.ReadFile(filepath.Join(feature, "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	manualCode, manualOut, manualErr, _ := runPrepare(
		t, "--path", manualRoot, "prepare", manualSlug, "--manual", "--json", "--quiet",
	)
	if manualCode != 0 || manualErr != "" {
		t.Fatalf("PIB-059: prepare --manual = exit %d stderr=%q\n%s", manualCode, manualErr, manualOut)
	}
	if constructions != 0 {
		t.Fatalf("PIB-059: the --manual run constructed a provider %d time(s)", constructions)
	}
	for name, want := range preimages {
		got, readErr := os.ReadFile(filepath.Join(feature, name))
		if readErr != nil || !bytes.Equal(got, want) {
			t.Fatalf("PIB-059: the --manual run rewrote artifact %s (%v)", name, readErr)
		}
	}
	if _, statErr := os.Stat(filepath.Join(feature, "artifacts", "analysis.json")); !os.IsNotExist(statErr) {
		t.Fatalf("PIB-059: the --manual run wrote the analysis sidecar: %v", statErr)
	}
	statusAfter, err := os.ReadFile(filepath.Join(feature, "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(statusAfter, statusBefore) {
		t.Fatal("PIB-059: the --manual run published nothing, so its zero-write claim is vacuous")
	}
	// Control: the same spy fires in generate mode, so the zero above is a
	// property of `--manual` and not of a dead seam.
	generateRoot, generateSlug := prepareS4Workspace(t, "PIB row 059 generate")
	constructions = 0
	generateCode, generateOut, generateErr, _ := runPrepare(
		t, "--path", generateRoot, "prepare", generateSlug, "--json", "--quiet",
	)
	if generateCode != 0 || generateErr != "" {
		t.Fatalf("PIB-059: prepare generate = exit %d stderr=%q\n%s",
			generateCode, generateErr, generateOut)
	}
	if constructions == 0 {
		t.Fatal("PIB-059: the provider-construction spy never fired in generate mode")
	}
}

// ---------------------------------------------------------------------------
// PIB-283 — the lane gate precedes the first local-lane write and is absent
// from every non-mutating mode.
// ---------------------------------------------------------------------------

func pibValidateLaneGatePlacement(sources map[string]string) error {
	publish := sources["internal/cli/prepare_publish.go"]
	command := sources["internal/cli/prepare.go"]
	if publish == "" || command == "" {
		return fmt.Errorf("the lane-gate scan received no sources")
	}
	body, found := pibGuardFunctionBody(
		publish, "func runPreparePublish(cmd *cobra.Command, rawSlug string, options prepareOptions) error {",
	)
	if !found {
		return fmt.Errorf("the mutating publication entry point was not found")
	}
	gate := strings.Index(body, "gitutil.IsIgnoredWithState(")
	if gate < 0 {
		return fmt.Errorf("the lane gate is not reached from the mutating entry point")
	}
	firstLaneWrite := strings.Index(body, "newPrepareTransactionOptions(")
	if firstLaneWrite < 0 {
		firstLaneWrite = strings.Index(body, "generatePrepareBundle(")
	}
	if firstLaneWrite < 0 {
		return fmt.Errorf("no local-lane write site was found")
	}
	if gate > firstLaneWrite {
		return fmt.Errorf("the lane gate runs after the first local-lane write")
	}
	for signature, label := range map[string]string{
		"func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {": "--abandon-transaction",
	} {
		abandon, ok := pibGuardFunctionBody(publish, signature)
		if !ok {
			return fmt.Errorf("the %s entry point was not found", label)
		}
		if strings.Contains(abandon, "gitutil.IsIgnoredWithState(") {
			return fmt.Errorf("the lane gate is reached from %s", label)
		}
	}
	check, ok := pibGuardFunctionBody(command, "func runPrepareCheck(cmd *cobra.Command, rawSlug string) error {")
	if !ok {
		return fmt.Errorf("the --check entry point was not found")
	}
	if strings.Contains(check, "gitutil.IsIgnoredWithState(") {
		return fmt.Errorf("the lane gate is reached from --check")
	}
	return nil
}

func TestPIBRowPIB283LaneGateCallSite(t *testing.T) {
	sources := pibGuardCLIProductionSources(t)
	if err := pibValidateLaneGatePlacement(sources); err != nil {
		t.Fatalf("PIB-283: the shipped lane gate failed its own guard: %v", err)
	}
	reached := pibGuardClone(sources)
	reached["internal/cli/prepare.go"] = strings.Replace(
		reached["internal/cli/prepare.go"],
		"	if !intent.RootConfinementSupported() {",
		"	_, _ = gitutil.IsIgnoredWithState(\"\", 0, \"\")\n	if !intent.RootConfinementSupported() {",
		1,
	)
	if reached["internal/cli/prepare.go"] == sources["internal/cli/prepare.go"] {
		t.Fatal("PIB-283: the --check mutation anchor is missing")
	}
	if err := pibValidateLaneGatePlacement(reached); err == nil {
		t.Fatal("PIB-283: the guard accepted a lane gate reachable from --check")
	}
	removed := pibGuardClone(sources)
	removed["internal/cli/prepare_publish.go"] = strings.Replace(
		removed["internal/cli/prepare_publish.go"],
		"gitutil.IsIgnoredWithState(repoRoot, gitState, laneRel)",
		"pibRemovedLaneGate(repoRoot, gitState, laneRel)",
		1,
	)
	if err := pibValidateLaneGatePlacement(removed); err == nil {
		t.Fatal("PIB-283: the guard accepted a removed lane gate")
	}
}

// ---------------------------------------------------------------------------
// PIB-103, PIB-191, PIB-249, PIB-269, PIB-342, PIB-373, PIB-374, PIB-377,
// PIB-378 — claims the shipped strings and documents must and must not make.
// ---------------------------------------------------------------------------

func pibValidateNoUnboundedRuntimeClaim(texts map[string]string) error {
	if len(texts) == 0 {
		return fmt.Errorf("the runtime-claim scan received no text")
	}
	forbidden := []string{
		"cannot " + "hang",
		"bounded " + "runtime",
		"guaranteed to " + "terminate",
		"never " + "hangs",
	}
	for name, body := range texts {
		lower := strings.ToLower(body)
		for _, claim := range forbidden {
			if strings.Contains(lower, claim) {
				return fmt.Errorf("%s claims %q", name, claim)
			}
		}
	}
	return nil
}

func pibGuardClaimCorpus(t *testing.T) map[string]string {
	t.Helper()
	corpus := map[string]string{"SPEC.md": s6RepoFile(t, "SPEC.md")}
	for name, body := range s6SkillBodies(t) {
		corpus["skill:"+name] = body
	}
	for key, value := range pibGuardEmittedStrings() {
		corpus["emitted:"+key] = value
	}
	return corpus
}

func TestPIBRowPIB191NoUnboundedRuntimeClaim(t *testing.T) {
	corpus := pibGuardClaimCorpus(t)
	if err := pibValidateNoUnboundedRuntimeClaim(corpus); err != nil {
		t.Fatalf("PIB-191: the shipped strings and docs failed their own guard: %v", err)
	}
	mutated := pibGuardClone(corpus)
	mutated["SPEC.md"] += "\nPrepare " + "cannot " + "hang.\n"
	if err := pibValidateNoUnboundedRuntimeClaim(mutated); err == nil {
		t.Fatal("PIB-191: the guard accepted a no-hang claim in SPEC.md")
	}
	skillMutated := pibGuardClone(corpus)
	for name := range skillMutated {
		if strings.HasPrefix(name, "skill:") {
			skillMutated[name] += "\nThe command has a " + "bounded " + "runtime.\n"
			break
		}
	}
	if err := pibValidateNoUnboundedRuntimeClaim(skillMutated); err == nil {
		t.Fatal("PIB-191: the guard accepted a bounded-runtime claim in a shipped skill")
	}
}

func pibValidateConcurrentWindowDisclosure(texts map[string]string) error {
	joined := strings.ToLower(strings.Join(pibGuardSortedValues(texts), "\n"))
	if !strings.Contains(joined, "concurrent") {
		return fmt.Errorf("no shipped text discloses the concurrent-write window")
	}
	for _, claim := range []string{
		"concurrent writes are preserved",
		"a concurrent write is preserved",
		"never loses a concurrent write",
	} {
		if strings.Contains(joined, claim) {
			return fmt.Errorf("a shipped text claims %q", claim)
		}
	}
	return nil
}

func pibGuardSortedValues(texts map[string]string) []string {
	keys := make([]string, 0, len(texts))
	for key := range texts {
		keys = append(keys, key)
	}
	values := make([]string, 0, len(texts))
	for _, key := range keys {
		values = append(values, texts[key])
	}
	return values
}

func TestPIBRowPIB103ConcurrentWindowDisclosure(t *testing.T) {
	corpus := pibGuardEmittedStrings()
	corpus["SPEC.md"] = s6RepoFile(t, "SPEC.md")
	if err := pibValidateConcurrentWindowDisclosure(corpus); err != nil {
		t.Fatalf("PIB-103: the shipped disclosure failed its own guard: %v", err)
	}
	silent := pibGuardClone(corpus)
	for name, body := range corpus {
		silent[name] = strings.ReplaceAll(
			strings.ReplaceAll(body, "concurrent", "parallel"), "Concurrent", "Parallel",
		)
	}
	if err := pibValidateConcurrentWindowDisclosure(silent); err == nil {
		t.Fatal("PIB-103: the guard accepted a corpus that discloses no window at all")
	}
	overclaiming := pibGuardClone(corpus)
	overclaiming["SPEC.md"] += "\nConcurrent writes are preserved.\n"
	if err := pibValidateConcurrentWindowDisclosure(overclaiming); err == nil {
		t.Fatal("PIB-103: the guard accepted a preservation claim")
	}
}

func pibValidateGapRefusalText(message, remediation string) error {
	joined := message + "\n" + remediation
	for _, route := range []string{"--regenerate", "--manual"} {
		if !strings.Contains(joined, route) {
			return fmt.Errorf("the incoherent-bundle-gap text does not name %s", route)
		}
	}
	if regexp.MustCompile(`(^|[^A-Za-z])rm([^A-Za-z]|$)`).MatchString(joined) {
		return fmt.Errorf("the incoherent-bundle-gap text tells the operator to remove files")
	}
	if strings.Contains(joined, "docs/") || strings.Contains(joined, ".md") {
		return fmt.Errorf("the incoherent-bundle-gap text points at a document")
	}
	return nil
}

func TestPIBRowPIB249IncoherentGapMessage(t *testing.T) {
	root, slug := prepareS4Workspace(t, "PIB row 249")
	if err := os.WriteFile(
		filepath.Join(pibRowFeature(root, slug), "spec.md"), []byte("orphan specification\n"), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
	report := prepareS4Report(t, stdout)
	if code != 2 || report.Refusal == nil || report.Refusal.Code != "incoherent-bundle-gap" {
		t.Fatalf("PIB-249: incoherent bundle = exit %d refusal=%#v", code, report.Refusal)
	}
	if err := pibValidateGapRefusalText(report.Refusal.Message, report.Refusal.Remediation); err != nil {
		t.Fatalf("PIB-249: the emitted refusal failed its own guard: %v", err)
	}
	if err := pibValidateGapRefusalText(
		report.Refusal.Message, report.Refusal.Remediation+" See docs/feature-layout.md.",
	); err == nil {
		t.Fatal("PIB-249: the guard accepted a document pointer in the remediation")
	}
	if err := pibValidateGapRefusalText(
		report.Refusal.Message, "Run rm on the stray artifact and retry.",
	); err == nil {
		t.Fatal("PIB-249: the guard accepted a remove instruction")
	}
}

func pibValidateRecoveryPendingText(message, remediation string) error {
	joined := message + "\n" + remediation
	if !strings.Contains(joined, "tpatch prepare demo-feature") {
		return fmt.Errorf("the recovery-pending text does not name the mutating re-run")
	}
	if !strings.Contains(joined, "--abandon-transaction") {
		return fmt.Errorf("the recovery-pending text does not name the abandon mode")
	}
	for _, claim := range []string{"unchanged", "would be the same", "identical plan"} {
		if strings.Contains(strings.ToLower(joined), claim) {
			return fmt.Errorf("the recovery-pending text claims the plan %q", claim)
		}
	}
	return nil
}

func TestPIBRowPIB269RecoveryPendingMessage(t *testing.T) {
	message, remediation := prepareRefusalText("recovery-pending", prepareModeGenerate, "demo-feature", "")
	if err := pibValidateRecoveryPendingText(message, remediation); err != nil {
		t.Fatalf("PIB-269: the shipped recovery-pending text failed its own guard: %v", err)
	}
	if err := pibValidateRecoveryPendingText(
		message, strings.Replace(remediation, "--abandon-transaction", "--abandon", 1),
	); err == nil {
		t.Fatal("PIB-269: the guard accepted a text that drops the abandon route")
	}
	if err := pibValidateRecoveryPendingText(
		message+" The plan would be unchanged.", remediation,
	); err == nil {
		t.Fatal("PIB-269: the guard accepted an unchanged-plan claim")
	}
}

func pibValidateUntrackedBundleAdvisory(text string) error {
	for _, required := range []string{"fresh clone", "Git clean"} {
		if !strings.Contains(text, required) {
			return fmt.Errorf("the bundle-untracked-in-git advisory does not state the %q risk", required)
		}
	}
	for _, claim := range []string{
		"Git-independent", "recoverable after a clone", "durable across a clean",
	} {
		if strings.Contains(text, claim) {
			return fmt.Errorf("the advisory claims %q", claim)
		}
	}
	return nil
}

func TestPIBRowPIB342UntrackedBundleAdvisory(t *testing.T) {
	source := pibGuardSources(t, "internal/cli/prepare_publish.go")["internal/cli/prepare_publish.go"]
	text, found := pibGuardAdvisoryText(source, "bundle-untracked-in-git")
	if !found {
		t.Fatal("PIB-342: the bundle-untracked-in-git advisory text was not found in the shipped source")
	}
	if err := pibValidateUntrackedBundleAdvisory(text); err != nil {
		t.Fatalf("PIB-342: the shipped advisory failed its own guard: %v", err)
	}
	if err := pibValidateUntrackedBundleAdvisory(
		strings.Replace(text, "fresh clone", "later run", 1),
	); err == nil {
		t.Fatal("PIB-342: the guard accepted an advisory that drops the clone risk")
	}
	if err := pibValidateUntrackedBundleAdvisory(text + " The archive is Git-independent."); err == nil {
		t.Fatal("PIB-342: the guard accepted a Git-independence claim")
	}
}

// pibGuardAdvisoryText returns the message literal that follows an advisory
// code in the shipped source.
func pibGuardAdvisoryText(source, code string) (string, bool) {
	marker := "\"" + code + "\""
	index := strings.Index(source, marker)
	if index < 0 {
		return "", false
	}
	rest := source[index+len(marker):]
	first := strings.Index(rest, "\"")
	if first < 0 {
		return "", false
	}
	rest = rest[first+1:]
	for {
		next := strings.Index(rest, "\"")
		if next < 0 {
			return "", false
		}
		candidate := rest[:next]
		if len(candidate) > 20 {
			return candidate, true
		}
		rest = rest[next+1:]
		second := strings.Index(rest, "\"")
		if second < 0 {
			return "", false
		}
		rest = rest[second+1:]
	}
}

func pibValidateDeadlineCascadeText(text string) error {
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "one ") {
		return fmt.Errorf("the deadline cascade advisory does not name a single expiry")
	}
	for _, claim := range []string{
		"independent provider failures", "separate provider failures", "three provider failures",
	} {
		if strings.Contains(lower, claim) {
			return fmt.Errorf("the advisory presents the fallbacks as %q", claim)
		}
	}
	return nil
}

func TestPIBRowPIB373DeadlineCascadeNamesOneExpiry(t *testing.T) {
	source := pibGuardSources(t, "internal/cli/prepare_publish.go")["internal/cli/prepare_publish.go"]
	text, found := pibGuardAdvisoryText(source, "provider-deadline-cascade")
	if !found {
		t.Fatal("PIB-373: the provider-deadline-cascade advisory text was not found")
	}
	if err := pibValidateDeadlineCascadeText(text); err != nil {
		t.Fatalf("PIB-373: the shipped cascade advisory failed its own guard: %v", err)
	}
	if err := pibValidateDeadlineCascadeText(
		strings.Replace(text, "One total deadline expiry caused", "There were", 1),
	); err == nil {
		t.Fatal("PIB-373: the guard accepted a report that names no single expiry")
	}
	if err := pibValidateDeadlineCascadeText(
		text + " These are independent provider failures.",
	); err == nil {
		t.Fatal("PIB-373: the guard accepted an independent-failures presentation")
	}
}

func pibValidateAllowHeuristicOptIn(help, flagUsage, prd string) error {
	if !strings.Contains(help, flagUsage) {
		return fmt.Errorf("prepare --help does not carry the verbatim opt-in sentence")
	}
	normalized := strings.ReplaceAll(prd, "`", "")
	normalized = strings.Join(strings.Fields(normalized), " ")
	wanted := strings.Join(strings.Fields(flagUsage), " ")
	if !strings.Contains(normalized, wanted) {
		return fmt.Errorf("§11.3.2 does not carry the same opt-in sentence the flag prints")
	}
	return nil
}

func TestPIBRowPIB374AllowHeuristicHelpSentence(t *testing.T) {
	flag := prepareCmd().Flags().Lookup("allow-heuristic")
	if flag == nil {
		t.Fatal("PIB-374: prepare declares no --allow-heuristic flag")
	}
	code, help, stderr, _ := runPrepare(t, "prepare", "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("PIB-374: prepare --help = exit %d stderr=%q", code, stderr)
	}
	prd := s6RepoFile(t, "docs/prds/PRD-prepare-intent-bundle.md")
	if err := pibValidateAllowHeuristicOptIn(
		strings.Join(strings.Fields(help), " "), flag.Usage, prd,
	); err != nil {
		t.Fatalf("PIB-374: the shipped opt-in sentence failed its own guard: %v", err)
	}
	if err := pibValidateAllowHeuristicOptIn(
		"Permit --regenerate to use heuristics.", flag.Usage, prd,
	); err == nil {
		t.Fatal("PIB-374: the guard accepted a paraphrased help sentence")
	}
	if err := pibValidateAllowHeuristicOptIn(
		strings.Join(strings.Fields(help), " "), flag.Usage,
		strings.Replace(prd, "Without this flag, regeneration\nrefuses", "Without this flag, regeneration may downgrade", 1),
	); err == nil {
		t.Fatal("PIB-374: the guard accepted a PRD that no longer carries the sentence")
	}
}

func pibValidateNoHeuristicConfigKey(sources map[string]string) error {
	if len(sources) == 0 {
		return fmt.Errorf("the config scan received no sources")
	}
	tokens := []string{"allow-heuristic", "allow_heuristic", "AllowHeuristic", "ALLOW_HEURISTIC"}
	for name, body := range sources {
		for _, token := range tokens {
			if strings.Contains(body, token) {
				return fmt.Errorf("%s exposes %q outside the flag", name, token)
			}
		}
	}
	return nil
}

func TestPIBRowPIB375NoHeuristicConfigRoute(t *testing.T) {
	sources := pibGuardSources(t,
		"internal/store/global.go",
		"internal/store/types.go",
	)
	if err := pibValidateNoHeuristicConfigKey(sources); err != nil {
		t.Fatalf("PIB-375: the shipped config surface failed its own guard: %v", err)
	}
	mutated := pibGuardClone(sources)
	mutated["internal/store/global.go"] += "\ntype pibHeuristicConfig struct{ AllowHeuristic bool }\n"
	if err := pibValidateNoHeuristicConfigKey(mutated); err == nil {
		t.Fatal("PIB-375: the guard accepted a config key that turns the opt-in on globally")
	}
}

func pibValidateNoTimingFields(sources map[string]string) error {
	if len(sources) == 0 {
		return fmt.Errorf("the timing-field scan received no sources")
	}
	tokens := []string{
		"json:\"duration", "json:\"deadline", "json:\"elapsed",
		"Duration:", "Elapsed:", "elapsed_time",
	}
	for name, body := range sources {
		for _, token := range tokens {
			if strings.Contains(body, token) {
				return fmt.Errorf("%s exposes the timing field %q on a report surface", name, token)
			}
		}
	}
	return nil
}

func TestPIBRowPIB377NoTimingFieldsInReports(t *testing.T) {
	sources := pibGuardSources(t, "internal/cli/prepare_publish.go")
	if err := pibValidateNoTimingFields(sources); err != nil {
		t.Fatalf("PIB-377: the shipped report surfaces failed their own guard: %v", err)
	}
	mutated := pibGuardClone(sources)
	mutated["internal/cli/prepare_publish.go"] += "\ntype pibTimingReport struct{ Elapsed int64 }\n// json:\"elapsed_time\"\n"
	if err := pibValidateNoTimingFields(mutated); err == nil {
		t.Fatal("PIB-377: the guard accepted an elapsed-time field on the JSON report")
	}
}

// ---------------------------------------------------------------------------
// PIB-312 — the durable writers are unchanged and prepare adds no caller.
// ---------------------------------------------------------------------------

func pibValidateDurableWriterParity(sources map[string]string) error {
	storeSource := sources["internal/store/store.go"]
	snapshot := sources["internal/gitutil/index_snapshot.go"]
	prepare := sources["internal/cli/prepare_publish.go"]
	if storeSource == "" || snapshot == "" || prepare == "" {
		return fmt.Errorf("the durable-writer scan received no sources")
	}
	if !strings.Contains(storeSource, "func writeFileAtomicWithRename(") {
		return fmt.Errorf("internal/store/store.go no longer declares writeFileAtomicWithRename")
	}
	if !strings.Contains(snapshot, "func DurableWriteFile(") &&
		!strings.Contains(storeSource, "func DurableWriteFile(") {
		return fmt.Errorf("DurableWriteFile is declared in neither shipped owner")
	}
	for _, callee := range []string{"writeFileAtomicWithRename(", "DurableWriteFile("} {
		if strings.Contains(prepare, callee) {
			return fmt.Errorf("prepare added a caller of %s", callee)
		}
	}
	return nil
}

func TestPIBRowPIB312DurableWriterParity(t *testing.T) {
	sources := pibGuardSources(t,
		"internal/store/store.go",
		"internal/gitutil/index_snapshot.go",
		"internal/cli/prepare_publish.go",
	)
	if err := pibValidateDurableWriterParity(sources); err != nil {
		t.Fatalf("PIB-312: the shipped durable writers failed their own guard: %v", err)
	}
	added := pibGuardClone(sources)
	added["internal/cli/prepare_publish.go"] += "\nfunc pibPrepareDurableCaller() { DurableWriteFile() }\n"
	if err := pibValidateDurableWriterParity(added); err == nil {
		t.Fatal("PIB-312: the guard accepted a new prepare caller of DurableWriteFile")
	}
	renamed := pibGuardClone(sources)
	renamed["internal/store/store.go"] = strings.Replace(
		renamed["internal/store/store.go"],
		"func writeFileAtomicWithRename(",
		"func writeFileAtomicWithRenameV2(",
		1,
	)
	if err := pibValidateDurableWriterParity(renamed); err == nil {
		t.Fatal("PIB-312: the guard accepted a renamed durable writer")
	}
}
