//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

func TestS7AQAbsoluteRootNeverReachesReportsOrProse(t *testing.T) {
	// PIB-497: real recovered, partial, refusal, and success populations keep
	// an absolute --path out of stdout, stderr, JSON, and human prose.
	type observation struct {
		name    string
		root    string
		stdout  string
		stderr  string
		payload map[string]any
		human   string
	}
	var observations []observation

	recovered := s7AQObserveTerminalRecovery(
		t, prepareModeGenerate, "CP3", false,
	)
	var recoveredHuman bytes.Buffer
	writePreparePublishHuman(&recoveredHuman, recovered.report)
	observations = append(observations, observation{
		name: "recovered", root: recovered.root, stdout: recovered.stdout,
		stderr: recovered.stderr, payload: s7AQDecodeObject(t, recovered.stdout),
		human: recoveredHuman.String(),
	})

	partial := s7APRunPartialPurge(t)
	var partialHuman bytes.Buffer
	writeIntentArchivePurgeHuman(&partialHuman, partial.report)
	observations = append(observations, observation{
		name: "partial-purge", root: partial.root, stdout: partial.stdout,
		stderr: partial.stderr, payload: s7AQDecodeObject(t, partial.stdout),
		human: partialHuman.String(),
	})

	refusalRoot, refusalSlug := prepareS4Workspace(t, "AQ privacy refusal")
	refusalCode, refusalOut, refusalErr, _ := runPrepare(
		t, "--path", refusalRoot, "prepare", refusalSlug,
		"--manual", "--json", "--quiet",
	)
	refusalReport := prepareS4Report(t, refusalOut)
	if refusalCode != 2 || refusalReport.Refusal == nil {
		t.Fatalf("PIB-497 refusal fixture = exit:%d stderr:%q report:%+v",
			refusalCode, refusalErr, refusalReport)
	}
	var refusalHuman bytes.Buffer
	writePreparePublishHuman(&refusalHuman, refusalReport)
	observations = append(observations, observation{
		name: "refusal", root: refusalRoot, stdout: refusalOut, stderr: refusalErr,
		payload: s7AQDecodeObject(t, refusalOut), human: refusalHuman.String(),
	})

	successRoot, successSlug := prepareS4Workspace(t, "AQ privacy success")
	successCode, successOut, successErr, _ := runPrepare(
		t, "--path", successRoot, "prepare", successSlug, "--json", "--quiet",
	)
	successReport := prepareS4Report(t, successOut)
	if successCode != 0 || successErr != "" || successReport.Outcome != "published" {
		t.Fatalf("PIB-497 success fixture = exit:%d stderr:%q report:%+v",
			successCode, successErr, successReport)
	}
	var successHuman bytes.Buffer
	writePreparePublishHuman(&successHuman, successReport)
	observations = append(observations, observation{
		name: "success", root: successRoot, stdout: successOut, stderr: successErr,
		payload: s7AQDecodeObject(t, successOut), human: successHuman.String(),
	})

	payloads := map[string]map[string]any{}
	var forbidden []string
	for _, observation := range observations {
		if !filepath.IsAbs(observation.root) {
			t.Fatalf("PIB-497 %s root is not absolute: %q",
				observation.name, observation.root)
		}
		combined := observation.stdout + observation.stderr + observation.human
		if strings.Contains(combined, observation.root) {
			t.Fatalf("PIB-497 %s leaked absolute root %q:\n%s",
				observation.name, observation.root, combined)
		}
		payloads[observation.name] = observation.payload
		forbidden = append(forbidden, observation.root)
	}

	reportSources := map[string]string{
		"internal/cli/prepare_publish.go":        s6RepoFile(t, "internal/cli/prepare_publish.go"),
		"internal/cli/feature_intent_archive.go": s6RepoFile(t, "internal/cli/feature_intent_archive.go"),
		"internal/workflow/doctor.go":            s6RepoFile(t, "internal/workflow/doctor.go"),
	}
	gitSource := s6RepoFile(t, "internal/gitutil/ignore.go")
	if err := validateS7AQReportPrivacy(
		gitSource, reportSources, payloads, forbidden,
	); err != nil {
		t.Fatal(err)
	}

	mutated := cloneS7AQSources(reportSources)
	const rel = "internal/cli/prepare_publish.go"
	anchor := "\tfmt.Fprintln(w, report.Disclaimer)\n"
	replacement := "\tfmt.Fprintf(w, \"Workspace path: %s\\n\", \"/absolute/aq-prose\")\n" + anchor
	mutated[rel] = strings.Replace(mutated[rel], anchor, replacement, 1)
	if mutated[rel] == reportSources[rel] {
		t.Fatal("PIB-497 prose sensitivity anchor missing")
	}
	if err := validateS7AQReportPrivacy(
		gitSource, mutated, payloads, forbidden,
	); err == nil {
		t.Fatal("PIB-497 same validator accepted an absolute path rendered in prose")
	}
	dynamic := cloneS7AQSources(reportSources)
	dynamic[rel] = strings.Replace(
		dynamic[rel],
		anchor,
		"\tfmt.Fprintf(w, \"Workspace path: %s\\n\", s7AQUnsafeProsePath())\n"+anchor,
		1,
	) + `
func s7AQUnsafeProsePath() string { return os.Getenv("HOME") }
`
	if err := validateS7AQReportPrivacy(
		gitSource, dynamic, payloads, forbidden,
	); err == nil {
		t.Fatal("PIB-497 same validator accepted an unresolved helper in prose")
	}
	localDynamic := cloneS7AQSources(reportSources)
	localDynamic[rel] = strings.Replace(
		localDynamic[rel],
		anchor,
		"\tpath := os.Getenv(\"HOME\")\n\tfmt.Fprintln(w, path)\n"+anchor,
		1,
	)
	if err := validateS7AQReportPrivacy(
		gitSource, localDynamic, payloads, forbidden,
	); err == nil {
		t.Fatal("PIB-497 same validator accepted a typed local HOME value in a real emitter")
	}
	for _, sensitivity := range []struct {
		name   string
		insert string
		suffix string
	}{
		{
			name:   "io-writestring",
			insert: "\tio.WriteString(w, os.Getenv(\"HOME\"))\n",
		},
		{
			name:   "writer-write",
			insert: "\t_, _ = w.Write([]byte(os.Getenv(\"HOME\")))\n",
		},
		{
			name: "writer-method-value",
			insert: "\twrite := w.Write\n" +
				"\t_, _ = write([]byte(os.Getenv(\"HOME\")))\n",
		},
		{
			name: "fmt-function-alias",
			insert: "\tprintLine := fmt.Fprintln\n" +
				"\t_, _ = printLine(w, os.Getenv(\"HOME\"))\n",
		},
		{
			name:   "global-print-helper",
			insert: "\tfmt.Print(os.Getenv(\"HOME\"))\n",
		},
		{
			name:   "builtin-print",
			insert: "\tprint(os.Getenv(\"HOME\"))\n",
		},
		{
			name:   "builtin-println",
			insert: "\tprintln(os.Getenv(\"HOME\"))\n",
		},
		{
			name:   "builtin-output-wrapper",
			insert: "\ts7AQBuiltinOutputWrapper(os.Getenv(\"HOME\"))\n",
			suffix: "\nfunc s7AQBuiltinOutputWrapper(value string) {\n" +
				"\tprintln(value)\n}\n",
		},
		{
			name:   "local-output-wrapper",
			insert: "\ts7AQOutputWrapper(w, os.Getenv(\"HOME\"))\n",
			suffix: "\nfunc s7AQOutputWrapper(w io.Writer, value string) {\n" +
				"\tfmt.Fprintln(w, value)\n}\n",
		},
		{
			name:   "unresolved-external-output",
			insert: "\t_, _ = io.Copy(w, strings.NewReader(os.Getenv(\"HOME\")))\n",
		},
	} {
		mutated := cloneS7AQSources(reportSources)
		mutated[rel] = strings.Replace(
			mutated[rel], anchor, sensitivity.insert+anchor, 1,
		) + sensitivity.suffix
		if err := validateS7AQReportPrivacy(
			gitSource, mutated, payloads, forbidden,
		); err == nil {
			t.Fatalf("PIB-497 same validator accepted %s sink", sensitivity.name)
		}
	}
	safeInterface := cloneS7AQSources(reportSources)
	safeInterface[rel] = strings.Replace(
		safeInterface[rel], anchor,
		"\tvar renderer s7AQSafeRenderer = s7AQSafeValueRenderer{}\n"+
			"\trenderer.Render(w)\n"+anchor,
		1,
	) + `
type s7AQSafeRenderer interface {
	Render(io.Writer)
}

type s7AQSafeValueRenderer struct{}

func (s7AQSafeValueRenderer) Render(w io.Writer) {
	_, _ = io.WriteString(w, "safe renderer")
}

type s7AQSafePointerRenderer struct{}

func (*s7AQSafePointerRenderer) Render(w io.Writer) {
	_, _ = w.Write([]byte("safe pointer renderer"))
}

type s7AQSafeEmbeddedRenderer struct {
	s7AQSafeValueRenderer
}

type s7AQSafeGenericRenderer[T any] struct{}

func (s7AQSafeGenericRenderer[T]) Render(w io.Writer) {
	_, _ = io.WriteString(w, "safe generic renderer")
}
`
	if err := validateS7AQReportPrivacy(
		gitSource, safeInterface, payloads, forbidden,
	); err != nil {
		t.Fatalf("PIB-497 same validator rejected safe local interface implementations: %v", err)
	}
	for _, sensitivity := range []struct {
		name     string
		receiver string
		value    string
		body     string
		call     string
	}{
		{
			name:     "local-interface-io-writestring",
			receiver: "s7AQUnsafeRendererImpl",
			value:    "s7AQUnsafeRendererImpl{}",
			body:     "\t_, _ = io.WriteString(w, os.Getenv(\"HOME\"))\n",
		},
		{
			name:     "local-interface-writer-write",
			receiver: "*s7AQUnsafeRendererImpl",
			value:    "&s7AQUnsafeRendererImpl{}",
			body:     "\t_, _ = w.Write([]byte(os.Getenv(\"HOME\")))\n",
			call:     "\trender := renderer.Render\n\trender(w)\n",
		},
		{
			name:     "local-interface-builtin-println",
			receiver: "s7AQUnsafeRendererImpl",
			value:    "s7AQUnsafeRendererImpl{}",
			body:     "\tprintln(os.Getenv(\"HOME\"))\n",
		},
	} {
		mutated := cloneS7AQSources(reportSources)
		call := sensitivity.call
		if call == "" {
			call = "\trenderer.Render(w)\n"
		}
		mutated[rel] = strings.Replace(
			mutated[rel], anchor,
			"\tvar renderer s7AQLocalRenderer = "+sensitivity.value+"\n"+
				call+anchor,
			1,
		) + `
type s7AQLocalRenderer interface {
	Render(io.Writer)
}

type s7AQSelectedSafeRenderer struct{}

func (s7AQSelectedSafeRenderer) Render(w io.Writer) {
	_, _ = io.WriteString(w, "selected safe renderer")
}

type s7AQUnsafeRendererImpl struct{}

func (` + sensitivity.receiver + `) Render(w io.Writer) {
` + sensitivity.body + `}
`
		err := validateS7AQReportPrivacy(
			gitSource, mutated, payloads, forbidden,
		)
		if err == nil {
			t.Fatalf("PIB-497 same validator accepted %s", sensitivity.name)
		}
	}
	safeModule := cloneS7AQSources(reportSources)
	safeModule[rel] = strings.Replace(
		safeModule[rel], anchor,
		"\tvar renderer fmt.Stringer = workflow.OpPresenceUnsupported\n"+
			"\t_ = renderer.String()\n"+anchor,
		1,
	)
	if err := validateS7AQReportPrivacy(
		gitSource, safeModule, payloads, forbidden,
	); err != nil {
		t.Fatalf("PIB-497 same validator rejected selected safe module implementation: %v", err)
	}
	const workflowRel = "internal/workflow/verify_diagnostics.go"
	unsafeWorkflow := s6RepoFile(t, workflowRel)
	unsafeWorkflow = strings.Replace(
		unsafeWorkflow,
		"import (\n\t\"strings\"\n)",
		"import (\n\t\"os\"\n\t\"strings\"\n)",
		1,
	)
	unsafeWorkflow = strings.Replace(
		unsafeWorkflow,
		"func (v OpPresenceVerdict) String() string {\n",
		"func (v OpPresenceVerdict) String() string {\n"+
			"\tprintln(os.Getenv(\"HOME\"))\n",
		1,
	)
	for _, sensitivity := range []struct {
		name      string
		call      string
		cliSuffix string
	}{
		{
			name: "imported-module-implementation",
			call: "\tvar renderer fmt.Stringer = workflow.OpPresenceUnsupported\n" +
				"\t_ = renderer.String()\n",
		},
		{
			name: "constructor-return-imported-implementation",
			call: "\trenderer := s7AQImportedRenderer()\n" +
				"\t_ = renderer.String()\n",
			cliSuffix: "\nfunc s7AQImportedRenderer() fmt.Stringer {\n" +
				"\treturn workflow.OpPresenceUnsupported\n}\n",
		},
		{
			name: "imported-interface-parameter-callsite",
			call: "\ts7AQRenderImported(workflow.OpPresenceUnsupported)\n",
			cliSuffix: "\nfunc s7AQRenderImported(" +
				"renderer fmt.Stringer) {\n" +
				"\t_ = renderer.String()\n}\n",
		},
	} {
		mutated := cloneS7AQSources(reportSources)
		mutated[rel] = strings.Replace(
			mutated[rel], anchor, sensitivity.call+anchor, 1,
		) + sensitivity.cliSuffix
		err := validateS7AQReportPrivacyWithRendererSources(
			gitSource, mutated,
			map[string]string{workflowRel: unsafeWorkflow},
			payloads, forbidden,
		)
		if err == nil {
			t.Fatalf("PIB-497 same validator accepted %s", sensitivity.name)
		}
		if !strings.Contains(err.Error(), "builtin println argument") {
			t.Fatalf("PIB-497 %s failed before the imported method body: %v",
				sensitivity.name, err)
		}
	}
	externalUnavailable := cloneS7AQSources(reportSources)
	externalUnavailable[rel] = strings.Replace(
		externalUnavailable[rel], anchor,
		"\tvar renderer error = fmt.Errorf(\"safe external renderer\")\n"+
			"\t_ = renderer.Error()\n"+anchor,
		1,
	)
	err := validateS7AQReportPrivacy(
		gitSource, externalUnavailable, payloads, forbidden,
	)
	if err == nil {
		t.Fatal("PIB-497 same validator accepted an unavailable external interface implementation")
	}
	if !strings.Contains(err.Error(), "unresolved concrete implementations") {
		t.Fatalf("PIB-497 unavailable external implementation failed open for another reason: %v", err)
	}
	jsonDynamic := cloneS7AQSources(reportSources)
	const jsonAnchor = "\tif asJSON {\n\t\tencoder := json.NewEncoder(cmd.OutOrStdout())"
	jsonDynamic[rel] = strings.Replace(
		jsonDynamic[rel],
		jsonAnchor,
		"\tif asJSON {\n\t\t_ = json.NewEncoder(cmd.OutOrStdout()).Encode("+
			"map[string]string{\"path\": os.Getenv(\"HOME\")})\n"+
			"\t\tencoder := json.NewEncoder(cmd.OutOrStdout())",
		1,
	)
	if err := validateS7AQReportPrivacy(
		gitSource, jsonDynamic, payloads, forbidden,
	); err == nil {
		t.Fatal("PIB-497 same validator accepted an uncontrolled JSON writer sink")
	}
}

func s7AQDecodeObject(t *testing.T, output string) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("decode JSON report: %v\n%s", err, output)
	}
	return payload
}

func validateS7AQReportPrivacy(
	gitSource string,
	reportSources map[string]string,
	payloads map[string]map[string]any,
	forbidden []string,
) error {
	return validateS7AQReportPrivacyWithRendererSources(
		gitSource, reportSources, nil, payloads, forbidden,
	)
}

func validateS7AQReportPrivacyWithRendererSources(
	gitSource string,
	reportSources map[string]string,
	rendererSources map[string]string,
	payloads map[string]map[string]any,
	forbidden []string,
) error {
	if err := validateS7APReportFieldInventory(reportSources); err != nil {
		return err
	}
	if err := validateS7APGitSourcePrivacy(
		gitSource, reportSources["internal/cli/prepare_publish.go"],
	); err != nil {
		return err
	}
	for name, payload := range payloads {
		if err := s7APWalkGitReport(name, payload, forbidden); err != nil {
			return err
		}
	}
	analysisSources := cloneS7AQSources(reportSources)
	for rel, source := range rendererSources {
		analysisSources[rel] = source
	}
	typedPackages, err := s7AQTypeCheckRendererPackages(analysisSources)
	if err != nil {
		return fmt.Errorf("type-check AQ privacy renderers: %w", err)
	}
	graph := s7AQBuildReportFlows(typedPackages)
	wantRoots := map[string]map[string]bool{
		"internal/cli/prepare_publish.go": {
			"emitPreparePublishReport": true,
		},
		"internal/cli/feature_intent_archive.go": {
			"emitIntentArchiveListReport":  true,
			"emitIntentArchivePurgeReport": true,
		},
	}
	observed := map[string]map[string]bool{}
	for name, roots := range wantRoots {
		packageName := filepath.ToSlash(filepath.Dir(name))
		pkg := typedPackages[packageName]
		flow := graph.flows[packageName]
		if pkg == nil || flow == nil || pkg.relFiles[name] == nil {
			return fmt.Errorf("typed AQ privacy source %s is missing", name)
		}
		observed[name] = map[string]bool{}
		for _, declaration := range pkg.relFiles[name].Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil || !roots[function.Name.Name] {
				continue
			}
			object, _ := pkg.info.Defs[function.Name].(*types.Func)
			if object == nil {
				return fmt.Errorf("%s:%s has no typed object", name, function.Name.Name)
			}
			observed[name][function.Name.Name] = true
			if err := s7AQValidateRendererFlow(
				object, graph, map[string]bool{}, s7AQRendererBindings{},
			); err != nil {
				return fmt.Errorf("%s:%s: %w", name, function.Name.Name, err)
			}
		}
	}
	if !reflect.DeepEqual(observed, wantRoots) {
		return fmt.Errorf("AQ privacy renderer inventory = %v, want %v", observed, wantRoots)
	}
	return nil
}

type s7AQRendererGraph struct {
	flows    map[string]*s7APReportStringFlow
	universe map[string]s7APFunctionDefinition
}

type s7AQRendererValue struct {
	expression ast.Expr
	flow       *s7APReportStringFlow
}

type s7AQRendererBindings map[types.Object][]s7AQRendererValue

var s7AQRendererDependencyCache sync.Map

func s7AQTypeCheckRendererPackages(
	sources map[string]string,
) (map[string]*s6TypedPackage, error) {
	moduleRoot, err := s6FindModuleRoot()
	if err != nil {
		return nil, err
	}
	roots := map[string]bool{}
	for rel := range sources {
		roots[s6FullModulePackagePath(filepath.ToSlash(filepath.Dir(rel)))] = true
	}
	complete := cloneS7AQSources(sources)
	for root := range roots {
		normalized, local := s6NormalizeModulePackage(root)
		if !local {
			continue
		}
		directory := filepath.Join(moduleRoot, filepath.FromSlash(normalized))
		entries, readErr := os.ReadDir(directory)
		if readErr != nil {
			return nil, readErr
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
				strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			matched, matchErr := build.Default.MatchFile(directory, entry.Name())
			if matchErr != nil {
				return nil, matchErr
			}
			if !matched {
				continue
			}
			rel := filepath.ToSlash(filepath.Join(normalized, entry.Name()))
			if _, overridden := complete[rel]; overridden {
				continue
			}
			body, readErr := os.ReadFile(filepath.Join(directory, entry.Name()))
			if readErr != nil {
				return nil, readErr
			}
			complete[rel] = string(body)
		}
	}
	graph, err := s6BuildTypeGraph(complete)
	if err != nil {
		return nil, fmt.Errorf("build module-aware renderer source graph: %w", err)
	}
	rootList := make([]string, 0, len(roots))
	for root := range roots {
		rootList = append(rootList, root)
	}
	sort.Strings(rootList)
	cacheKey := strings.Join(rootList, "\x00")
	var listed map[string]bool
	if cached, ok := s7AQRendererDependencyCache.Load(cacheKey); ok {
		listed = cached.(map[string]bool)
	} else {
		args := []string{"list", "-deps", "-mod=readonly", "-f={{.ImportPath}}"}
		args = append(args, rootList...)
		command := exec.Command("go", args...)
		command.Dir = moduleRoot
		for _, entry := range os.Environ() {
			if strings.HasPrefix(entry, "GOPROXY=") ||
				strings.HasPrefix(entry, "GOSUMDB=") {
				continue
			}
			command.Env = append(command.Env, entry)
		}
		command.Env = append(command.Env, "GOPROXY=off", "GOSUMDB=off")
		output, listErr := command.Output()
		if listErr != nil {
			return nil, fmt.Errorf("offline go list -deps renderer graph: %w", listErr)
		}
		listed = map[string]bool{}
		for _, path := range strings.Fields(string(output)) {
			if path != s6ModulePath &&
				!strings.HasPrefix(path, s6ModulePath+"/") {
				continue
			}
			normalized, _ := s6NormalizeModulePackage(path)
			listed[normalized] = true
		}
		s7AQRendererDependencyCache.Store(cacheKey, listed)
	}
	for packageName := range listed {
		if graph.packages[packageName] == nil {
			return nil, fmt.Errorf(
				"module-local renderer dependency %s is absent from the source graph",
				packageName,
			)
		}
	}
	for packageName := range graph.packages {
		if !listed[packageName] {
			return nil, fmt.Errorf(
				"renderer source package %s is absent from offline go list -deps",
				packageName,
			)
		}
	}
	return graph.packages, nil
}

func s7AQBuildReportFlows(
	typedPackages map[string]*s6TypedPackage,
) *s7AQRendererGraph {
	flows := map[string]*s7APReportStringFlow{}
	universe := map[string]s7APFunctionDefinition{}
	for packageName, pkg := range typedPackages {
		flow := newS7APReportStringFlow(pkg)
		flows[packageName] = flow
		for function, declaration := range flow.functions {
			universe[s7APFunctionKey(function)] = s7APFunctionDefinition{
				flow: flow, function: function, declaration: declaration,
			}
		}
	}
	enumUniverse := map[string][]s7APEnumFieldAssignment{}
	enumDefaults := map[string]bool{}
	for _, flow := range flows {
		flow.universe = universe
		for key, expressions := range flow.enumAssignments {
			for _, expression := range expressions {
				enumUniverse[key] = append(enumUniverse[key], s7APEnumFieldAssignment{
					flow: flow, expression: expression,
				})
			}
		}
		for key := range flow.enumDefaults {
			enumDefaults[key] = true
		}
	}
	for _, flow := range flows {
		flow.enumUniverse = enumUniverse
		flow.enumDefaultUniverse = enumDefaults
	}
	return &s7AQRendererGraph{
		flows: flows, universe: universe,
	}
}

func s7AQValidateRendererFlow(
	function *types.Func,
	graph *s7AQRendererGraph,
	visiting map[string]bool,
	bindings s7AQRendererBindings,
) error {
	key := s7APFunctionKey(function)
	if visiting[key] {
		return nil
	}
	definition, ok := graph.universe[key]
	if !ok || definition.declaration == nil || definition.declaration.Body == nil {
		return fmt.Errorf("renderer call target %s is unresolved", key)
	}
	visiting[key] = true
	defer delete(visiting, key)

	var validationErr error
	ast.Inspect(definition.declaration.Body, func(node ast.Node) bool {
		if validationErr != nil {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if builtin := s7AQBuiltinObject(definition.flow.info, call); builtin != nil {
			switch builtin.Name() {
			case "print", "println":
				for index, argument := range call.Args {
					if !definition.flow.provesNoAbsolute(argument, nil, nil, nil) {
						validationErr = fmt.Errorf(
							"builtin %s argument %d is not recursively proven non-absolute via %s",
							builtin.Name(), index, s7APFormatExpression(argument),
						)
						return false
					}
				}
				return true
			default:
				if s7AQKnownRendererBuiltin(builtin) {
					return true
				}
				validationErr = fmt.Errorf(
					"reachable renderer builtin %s is not allowlisted",
					builtin.Name(),
				)
				return false
			}
		}
		targets, err := s7AQRendererCallTargets(
			call, definition.flow, graph, bindings,
		)
		if err != nil {
			validationErr = err
			return false
		}
		for _, called := range targets {
			if err := s7AQValidateRendererCallTarget(
				call, called, definition.flow, graph, visiting, bindings,
			); err != nil {
				validationErr = err
				return false
			}
		}
		return true
	})
	return validationErr
}

func s7AQValidateRendererCallTarget(
	call *ast.CallExpr,
	called *types.Func,
	flow *s7APReportStringFlow,
	graph *s7AQRendererGraph,
	visiting map[string]bool,
	bindings s7AQRendererBindings,
) error {
	if start, sink, reject := s7AQRendererSink(call, called, flow); sink {
		if reject {
			return fmt.Errorf(
				"reachable renderer sink %s is not statically traceable",
				s7APFunctionKey(called),
			)
		}
		for index := start; index < len(call.Args); index++ {
			if !flow.provesNoAbsolute(call.Args[index], nil, nil, nil) {
				return fmt.Errorf(
					"%s argument %d is not recursively proven non-absolute via %s",
					called.Name(), index, s7APFormatExpression(call.Args[index]),
				)
			}
		}
		return nil
	}
	if s7AQJSONEncodeSink(called) {
		if len(call.Args) != 1 ||
			!s7AQControlledJSONReportType(flow.info.TypeOf(call.Args[0])) {
			return fmt.Errorf(
				"JSON Encode has unclassified payload %s",
				s7APFormatExpression(call.Args[0]),
			)
		}
		return nil
	}
	if s7AQLocalModuleFunction(called) {
		if _, reachable := graph.universe[s7APFunctionKey(called)]; !reachable {
			return fmt.Errorf(
				"internal renderer target %s is absent from the typed universe",
				s7APFunctionKey(called),
			)
		}
		next, err := s7AQBindRendererCall(called, call, flow, graph, bindings)
		if err != nil {
			return err
		}
		return s7AQValidateRendererFlow(called, graph, visiting, next)
	}
	if s7AQKnownRendererSupportCall(called) {
		return nil
	}
	class := "support"
	if s7AQCallHasWriterArgument(flow.info, call) ||
		s7AQOutputLikeFunction(called) {
		class = "output"
	}
	return fmt.Errorf(
		"unclassified external %s call %s",
		class, s7APFunctionKey(called),
	)
}

func s7AQRendererCallTargets(
	call *ast.CallExpr,
	flow *s7APReportStringFlow,
	graph *s7AQRendererGraph,
	bindings s7AQRendererBindings,
) ([]*types.Func, error) {
	dispatches := s7AQInterfaceDispatches(
		call.Fun, flow, bindings, map[types.Object]bool{},
	)
	if len(dispatches) > 1 {
		return nil, fmt.Errorf(
			"renderer interface dispatch %s has %d ambiguous selections",
			s7APFormatExpression(call.Fun), len(dispatches),
		)
	}
	if len(dispatches) == 1 {
		dispatch := dispatches[0]
		method, _ := dispatch.selection.Obj().(*types.Func)
		if method == nil {
			return nil, fmt.Errorf(
				"renderer interface dispatch %s has no method object",
				s7APFormatExpression(call.Fun),
			)
		}
		if _, sink, reject := s7AQRendererSink(call, method, flow); sink && !reject {
			return []*types.Func{method}, nil
		}
		receiverTypes, unresolved, err := s7AQResolveRendererTypes(
			dispatch.receiver, dispatch.flow, graph, bindings,
			map[types.Object]bool{}, map[string]bool{},
		)
		if err != nil {
			return nil, err
		}
		if unresolved || len(receiverTypes) == 0 {
			return nil, fmt.Errorf(
				"renderer interface receiver %s has unresolved concrete implementations",
				s7APFormatExpression(dispatch.receiver),
			)
		}
		return s7AQConcreteInterfaceMethods(
			dispatch.selection, receiverTypes, graph,
		)
	}
	called := flow.calledFunction(call)
	if called == nil {
		if flow.info.Types[call.Fun].IsType() {
			return nil, nil
		}
		return nil, fmt.Errorf(
			"reachable renderer call %s is unresolved",
			s7APFormatExpression(call.Fun),
		)
	}
	if signature, _ := called.Type().(*types.Signature); signature != nil &&
		signature.Recv() != nil &&
		s7AQInterfaceType(signature.Recv().Type()) != nil {
		return nil, fmt.Errorf(
			"renderer interface call %s lost its typed selection",
			s7APFunctionKey(called),
		)
	}
	return []*types.Func{called}, nil
}

type s7AQInterfaceDispatch struct {
	selection *types.Selection
	receiver  ast.Expr
	flow      *s7APReportStringFlow
}

func s7AQInterfaceDispatches(
	expression ast.Expr,
	flow *s7APReportStringFlow,
	bindings s7AQRendererBindings,
	visiting map[types.Object]bool,
) []s7AQInterfaceDispatch {
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return s7AQInterfaceDispatches(value.X, flow, bindings, visiting)
	case *ast.SelectorExpr:
		selection := flow.info.Selections[value]
		if selection != nil && s7AQInterfaceType(selection.Recv()) != nil {
			return []s7AQInterfaceDispatch{{
				selection: selection, receiver: value.X, flow: flow,
			}}
		}
	case *ast.Ident:
		object := flow.info.ObjectOf(value)
		if object == nil || visiting[object] {
			return nil
		}
		visiting[object] = true
		defer delete(visiting, object)
		seen := map[string]s7AQInterfaceDispatch{}
		for _, bound := range bindings[object] {
			for _, dispatch := range s7AQInterfaceDispatches(
				bound.expression, bound.flow, bindings, visiting,
			) {
				method, _ := dispatch.selection.Obj().(*types.Func)
				key := s7APFunctionKey(method) + "#" +
					types.TypeString(dispatch.selection.Recv(), s7AQTypeQualifier) +
					"#" + s7APFormatExpression(dispatch.receiver)
				seen[key] = dispatch
			}
		}
		for _, assigned := range flow.assignments[object] {
			for _, dispatch := range s7AQInterfaceDispatches(
				assigned, flow, bindings, visiting,
			) {
				method, _ := dispatch.selection.Obj().(*types.Func)
				key := s7APFunctionKey(method) + "#" +
					types.TypeString(dispatch.selection.Recv(), s7AQTypeQualifier) +
					"#" + s7APFormatExpression(dispatch.receiver)
				seen[key] = dispatch
			}
		}
		var result []s7AQInterfaceDispatch
		for _, dispatch := range seen {
			result = append(result, dispatch)
		}
		sort.Slice(result, func(i, j int) bool {
			left, _ := result[i].selection.Obj().(*types.Func)
			right, _ := result[j].selection.Obj().(*types.Func)
			return s7APFunctionKey(left) < s7APFunctionKey(right)
		})
		return result
	}
	return nil
}

func s7AQConcreteInterfaceMethods(
	selection *types.Selection,
	receiverTypes map[string]types.Type,
	graph *s7AQRendererGraph,
) ([]*types.Func, error) {
	iface := s7AQInterfaceType(selection.Recv())
	method, _ := selection.Obj().(*types.Func)
	if iface == nil || method == nil {
		return nil, errors.New("local renderer interface selection is incomplete")
	}
	iface.Complete()
	targets := map[string]*types.Func{}
	for _, candidate := range receiverTypes {
		if s7AQInterfaceType(candidate) != nil {
			return nil, fmt.Errorf(
				"renderer receiver remained interface-typed at %s",
				types.TypeString(candidate, s7AQTypeQualifier),
			)
		}
		if !types.Implements(candidate, iface) {
			return nil, fmt.Errorf(
				"renderer receiver %s does not implement %s",
				types.TypeString(candidate, s7AQTypeQualifier),
				types.TypeString(iface, s7AQTypeQualifier),
			)
		}
		concrete := types.NewMethodSet(candidate).Lookup(
			method.Pkg(), method.Name(),
		)
		if concrete == nil {
			return nil, fmt.Errorf(
				"%s implements %s but method %s is unresolved",
				types.TypeString(candidate, s7AQTypeQualifier),
				types.TypeString(iface, s7AQTypeQualifier),
				method.Name(),
			)
		}
		function, _ := concrete.Obj().(*types.Func)
		if function == nil {
			return nil, fmt.Errorf(
				"concrete renderer method %s has no function object",
				concrete.Obj().Name(),
			)
		}
		targets[s7APFunctionKey(function)] = function
	}
	keys := make([]string, 0, len(targets))
	for key := range targets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]*types.Func, 0, len(keys))
	for _, key := range keys {
		function := targets[key]
		if s7AQLocalModuleFunction(function) {
			if _, ok := graph.universe[key]; !ok {
				return nil, fmt.Errorf(
					"local renderer implementation %s is absent from the typed universe",
					key,
				)
			}
		}
		result = append(result, function)
	}
	return result, nil
}

func s7AQResolveRendererTypes(
	expression ast.Expr,
	flow *s7APReportStringFlow,
	graph *s7AQRendererGraph,
	bindings s7AQRendererBindings,
	visitingObjects map[types.Object]bool,
	visitingFunctions map[string]bool,
) (map[string]types.Type, bool, error) {
	result := map[string]types.Type{}
	addConcrete := func(value types.Type) bool {
		if value == nil || s7AQInterfaceType(value) != nil {
			return false
		}
		result[types.TypeString(value, s7AQTypeQualifier)] = value
		return true
	}
	merge := func(values map[string]types.Type) {
		for key, value := range values {
			result[key] = value
		}
	}
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return s7AQResolveRendererTypes(
			value.X, flow, graph, bindings, visitingObjects, visitingFunctions,
		)
	case *ast.CompositeLit, *ast.FuncLit:
		if addConcrete(flow.info.TypeOf(expression)) {
			return result, false, nil
		}
	case *ast.UnaryExpr:
		if value.Op == token.AND && addConcrete(flow.info.TypeOf(expression)) {
			return result, false, nil
		}
	case *ast.TypeAssertExpr:
		if value.Type != nil && addConcrete(flow.info.TypeOf(expression)) {
			return result, false, nil
		}
	case *ast.Ident:
		object := flow.info.ObjectOf(value)
		if object == nil || visitingObjects[object] {
			return nil, true, nil
		}
		if addConcrete(object.Type()) {
			return result, false, nil
		}
		visitingObjects[object] = true
		defer delete(visitingObjects, object)
		found := false
		unresolved := false
		for _, bound := range bindings[object] {
			values, unknown, err := s7AQResolveRendererTypes(
				bound.expression, bound.flow, graph, bindings,
				visitingObjects, visitingFunctions,
			)
			if err != nil {
				return nil, false, err
			}
			merge(values)
			found = found || len(values) != 0
			unresolved = unresolved || unknown
		}
		for _, assigned := range flow.assignments[object] {
			values, unknown, err := s7AQResolveRendererTypes(
				assigned, flow, graph, bindings,
				visitingObjects, visitingFunctions,
			)
			if err != nil {
				return nil, false, err
			}
			merge(values)
			found = found || len(values) != 0
			unresolved = unresolved || unknown
		}
		return result, unresolved || !found, nil
	case *ast.SelectorExpr:
		object := flow.info.ObjectOf(value.Sel)
		if object != nil {
			if addConcrete(object.Type()) {
				return result, false, nil
			}
			if visitingObjects[object] {
				return nil, true, nil
			}
			visitingObjects[object] = true
			defer delete(visitingObjects, object)
			found := false
			unresolved := false
			for _, assigned := range flow.assignments[object] {
				values, unknown, err := s7AQResolveRendererTypes(
					assigned, flow, graph, bindings,
					visitingObjects, visitingFunctions,
				)
				if err != nil {
					return nil, false, err
				}
				merge(values)
				found = found || len(values) != 0
				unresolved = unresolved || unknown
			}
			return result, unresolved || !found, nil
		}
	case *ast.CallExpr:
		if flow.info.Types[value.Fun].IsType() && len(value.Args) == 1 {
			return s7AQResolveRendererTypes(
				value.Args[0], flow, graph, bindings,
				visitingObjects, visitingFunctions,
			)
		}
		called := flow.calledFunction(value)
		if called == nil {
			return nil, true, nil
		}
		key := s7APFunctionKey(called)
		definition, local := graph.universe[key]
		if !local {
			if addConcrete(flow.info.TypeOf(value)) {
				return result, false, nil
			}
			return nil, true, nil
		}
		if visitingFunctions[key] {
			return nil, true, nil
		}
		next, err := s7AQBindRendererCall(
			called, value, flow, graph, bindings,
		)
		if err != nil {
			return nil, false, err
		}
		visitingFunctions[key] = true
		defer delete(visitingFunctions, key)
		found := false
		unresolved := false
		ast.Inspect(definition.declaration.Body, func(node ast.Node) bool {
			if err != nil {
				return false
			}
			if _, nested := node.(*ast.FuncLit); nested {
				return false
			}
			statement, ok := node.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			if len(statement.Results) == 0 {
				unresolved = true
				return true
			}
			values, unknown, resolveErr := s7AQResolveRendererTypes(
				statement.Results[0], definition.flow, graph, next,
				visitingObjects, visitingFunctions,
			)
			if resolveErr != nil {
				err = resolveErr
				return false
			}
			merge(values)
			found = found || len(values) != 0
			unresolved = unresolved || unknown
			return true
		})
		if err != nil {
			return nil, false, err
		}
		return result, unresolved || !found, nil
	default:
		if addConcrete(flow.info.TypeOf(expression)) {
			return result, false, nil
		}
	}
	return nil, true, nil
}

func s7AQBindRendererCall(
	target *types.Func,
	call *ast.CallExpr,
	callerFlow *s7APReportStringFlow,
	graph *s7AQRendererGraph,
	parent s7AQRendererBindings,
) (s7AQRendererBindings, error) {
	definition, ok := graph.universe[s7APFunctionKey(target)]
	if !ok || definition.declaration == nil {
		return nil, fmt.Errorf(
			"internal renderer target %s has no body for argument binding",
			s7APFunctionKey(target),
		)
	}
	result := s7AQRendererBindings{}
	for object, values := range parent {
		result[object] = append(result[object], values...)
	}
	argument := 0
	if parameters := definition.declaration.Type.Params; parameters != nil {
		for _, field := range parameters.List {
			for _, name := range field.Names {
				if argument >= len(call.Args) {
					return nil, fmt.Errorf(
						"renderer call %s has too few arguments",
						s7APFunctionKey(target),
					)
				}
				if object := definition.flow.info.Defs[name]; object != nil {
					result[object] = append(result[object], s7AQRendererValue{
						expression: call.Args[argument], flow: callerFlow,
					})
				}
				argument++
			}
		}
	}
	if definition.declaration.Recv != nil &&
		len(definition.declaration.Recv.List) == 1 &&
		len(definition.declaration.Recv.List[0].Names) == 1 {
		dispatches := s7AQInterfaceDispatches(
			call.Fun, callerFlow, parent, map[types.Object]bool{},
		)
		if len(dispatches) == 1 {
			name := definition.declaration.Recv.List[0].Names[0]
			if object := definition.flow.info.Defs[name]; object != nil {
				result[object] = append(result[object], s7AQRendererValue{
					expression: dispatches[0].receiver,
					flow:       dispatches[0].flow,
				})
			}
		}
	}
	return result, nil
}

func s7AQInterfaceType(value types.Type) *types.Interface {
	if value == nil {
		return nil
	}
	iface, _ := types.Unalias(value).Underlying().(*types.Interface)
	return iface
}

func s7AQLocalModuleFunction(function *types.Func) bool {
	return function != nil && function.Pkg() != nil &&
		strings.HasPrefix(
			function.Pkg().Path(),
			"github.com/tesseracode/tesserapatch/internal/",
		)
}

func s7AQTypeQualifier(pkg *types.Package) string {
	if pkg == nil {
		return ""
	}
	return pkg.Path()
}

func s7AQBuiltinObject(info *types.Info, call *ast.CallExpr) *types.Builtin {
	identifier, _ := call.Fun.(*ast.Ident)
	if identifier == nil {
		return nil
	}
	builtin, _ := info.ObjectOf(identifier).(*types.Builtin)
	return builtin
}

func s7AQKnownRendererBuiltin(builtin *types.Builtin) bool {
	if builtin == nil {
		return false
	}
	switch builtin.Name() {
	case "append", "len", "make":
		return true
	default:
		return false
	}
}

func s7AQRendererSink(
	call *ast.CallExpr,
	function *types.Func,
	flow *s7APReportStringFlow,
) (start int, sink bool, reject bool) {
	if function == nil {
		return 0, false, false
	}
	pkg := ""
	if function.Pkg() != nil {
		pkg = function.Pkg().Path()
	}
	switch {
	case pkg == "fmt" &&
		(function.Name() == "Fprint" || function.Name() == "Fprintf" ||
			function.Name() == "Fprintln"):
		return 1, true, false
	case pkg == "fmt" &&
		(function.Name() == "Print" || function.Name() == "Printf" ||
			function.Name() == "Println"):
		return 0, true, false
	case pkg == "log" && strings.HasPrefix(function.Name(), "Print"):
		return 0, true, false
	case pkg == "io" && function.Name() == "WriteString":
		return 1, true, false
	case pkg == "io" && function.Name() == "Copy":
		return 0, true, true
	case s7AQWriterWriteMethod(function):
		return 0, true, false
	case pkg == "github.com/spf13/cobra" &&
		(strings.HasPrefix(function.Name(), "Print") ||
			strings.HasPrefix(function.Name(), "PrintErr")):
		return 0, true, false
	default:
		_ = call
		_ = flow
		return 0, false, false
	}
}

func s7AQWriterWriteMethod(function *types.Func) bool {
	if function == nil || function.Name() != "Write" {
		return false
	}
	signature, _ := function.Type().(*types.Signature)
	if signature == nil || signature.Params().Len() != 1 ||
		signature.Results().Len() != 2 {
		return false
	}
	slice, _ := signature.Params().At(0).Type().Underlying().(*types.Slice)
	if slice == nil {
		return false
	}
	basic, _ := slice.Elem().Underlying().(*types.Basic)
	return basic != nil && basic.Kind() == types.Byte
}

func s7AQJSONEncodeSink(function *types.Func) bool {
	if function == nil || function.Pkg() == nil ||
		function.Pkg().Path() != "encoding/json" ||
		function.Name() != "Encode" {
		return false
	}
	signature, _ := function.Type().(*types.Signature)
	return signature != nil && signature.Recv() != nil
}

func s7AQControlledJSONReportType(value types.Type) bool {
	for {
		pointer, ok := value.(*types.Pointer)
		if !ok {
			break
		}
		value = pointer.Elem()
	}
	named, _ := value.(*types.Named)
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil ||
		named.Obj().Pkg().Path() != "github.com/tesseracode/tesserapatch/internal/cli" {
		return false
	}
	switch named.Obj().Name() {
	case "preparePublishReport", "intentArchiveListReport", "intentArchivePurgeReport":
		return true
	default:
		return false
	}
}

func s7AQKnownRendererSupportCall(function *types.Func) bool {
	if function == nil {
		return false
	}
	pkg := ""
	if function.Pkg() != nil {
		pkg = function.Pkg().Path()
	}
	key := pkg + "." + function.Name()
	switch key {
	case "encoding/json.NewEncoder",
		"encoding/json.SetIndent",
		"fmt.Errorf",
		"fmt.Sprint",
		"fmt.Sprintf",
		"sort.SliceStable",
		"strings.HasPrefix",
		"strings.Join",
		"strings.ReplaceAll",
		"strings.Split",
		"strings.TrimPrefix",
		"strings.TrimSpace",
		"strings.TrimSuffix",
		"strings.NewReader":
		return true
	}
	if pkg == "github.com/spf13/cobra" {
		switch function.Name() {
		case "Flags", "OutOrStdout", "ErrOrStderr":
			return true
		}
	}
	if pkg == "github.com/spf13/pflag" {
		return function.Name() == "GetBool"
	}
	return false
}

func s7AQCallHasWriterArgument(info *types.Info, call *ast.CallExpr) bool {
	for _, argument := range call.Args {
		if s7AQWriterType(info.TypeOf(argument)) {
			return true
		}
	}
	return false
}

func s7AQWriterType(value types.Type) bool {
	if value == nil {
		return false
	}
	methods := types.NewMethodSet(value)
	for index := 0; index < methods.Len(); index++ {
		function, _ := methods.At(index).Obj().(*types.Func)
		if s7AQWriterWriteMethod(function) {
			return true
		}
	}
	return false
}

func s7AQOutputLikeFunction(function *types.Func) bool {
	if function == nil {
		return false
	}
	name := strings.ToLower(function.Name())
	return strings.Contains(name, "print") ||
		strings.Contains(name, "write") ||
		strings.Contains(name, "copy") ||
		strings.Contains(name, "log")
}
