package candidate

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/candidatelock"
	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

func TestConstructorSignatures(t *testing.T) {
	var _ func(string, string) (*ReferenceRoot, error) = OpenReferenceRoot
	var _ func(string, string, string, string, *candidatelock.Handle, *candidatelock.Handle) (*CandidateRoot, error) = CreateCandidateRoot
	var _ func(*CandidateRoot, *candidatelock.Handle, *candidatelock.Handle) error = (*CandidateRoot).Prune
	var _ func(*ReferenceRoot, *ReferenceRoot, string, []gitutil.PatchEffect, string, string, *candidatelock.Handle) (ExecutionResult, error) = Execute
	var _ func(*ReferenceRoot, *ReferenceRoot, string, []gitutil.PatchEffect, Plan, string, string, *candidatelock.Handle) (ExecutionResult, error) = executeWithPlan
}

func TestCandidateProductionSourceInventory(t *testing.T) {
	files, err := candidateProductionFiles()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, file := range files {
		names = append(names, filepath.Base(file.path))
	}
	sort.Strings(names)
	if err := checkCandidateProductionInventory(names); err != nil {
		t.Fatal(err)
	}
	for _, mutated := range [][]string{
		{"readflags_other.go", "readflags_unix.go", "root.go"},
		append(append([]string(nil), names...), "third_root.go"),
		append(append([]string(nil), names...), "reference.go"),
	} {
		if err := checkCandidateProductionInventory(mutated); err == nil {
			t.Fatalf("inventory accepted missing or unregistered source: %v", mutated)
		}
	}
}

func checkCandidateProductionInventory(names []string) error {
	want := []string{
		"alignment.go",
		"derivation.go",
		"execute.go",
		"readflags_other.go",
		"readflags_unix.go",
		"reference.go",
		"root.go",
		"snapshot.go",
		"validation.go",
		"writer.go",
	}
	if strings.Join(names, "\n") != strings.Join(want, "\n") {
		return fmt.Errorf("candidate production file inventory changed:\n got %v\nwant %v", names, want)
	}
	return nil
}

func TestBareRootConstructorsBoundary(t *testing.T) {
	files, err := candidateProductionFiles()
	if err != nil {
		t.Fatal(err)
	}
	if err := checkBareRootConstructors(files); err != nil {
		t.Fatal(err)
	}
}

func TestBareRootConstructorsBoundarySensitivity(t *testing.T) {
	files, err := candidateProductionFiles()
	if err != nil {
		t.Fatal(err)
	}
	if err := checkBareRootConstructors(files); err != nil {
		t.Fatal(err)
	}
	source := mustReadCandidateSource(t, "root.go")
	for _, extra := range []string{
		"\nfunc helper(workspace string) (*heldRoot, error) { return openHeldRoot(absoluteRootPath(workspace)) }\n",
		"\nfunc renamed(path string) (*os.Root, error) { return os.OpenRoot(path) }\n",
		"\nfunc (r *ReferenceRoot) rawRead(path string) ([]byte, error) { return os.ReadFile(path) }\n",
		"\nvar alternateOpener = os.OpenRoot\n",
		"\nvar alternateRead = os.ReadFile\n",
		"\nvar alternateFactory = openHeldRoot\n",
	} {
		mutants := append([]candidateFile(nil), files...)
		for i := range mutants {
			if mutants[i].path == "root.go" {
				mutants[i].file = mustParseCandidateSource(t, "root.go", source+extra)
			}
		}
		if err := checkBareRootConstructors(mutants); err == nil {
			t.Fatalf("boundary guard accepted %s", extra)
		}
	}
}

func TestExecutionTypedBoundary(t *testing.T) {
	file := mustParseCandidateSource(t, "execute.go", mustReadCandidateSource(t, "execute.go"))
	if err := checkExecutionTypedBoundary(file); err != nil {
		t.Fatal(err)
	}
}

func TestExecutionTypedBoundarySensitivity(t *testing.T) {
	source := `package candidate

import (
	"github.com/tesseracode/tesserapatch/internal/candidatelock"
	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

func Execute(reference, upstream *ReferenceRoot, patch string, effects []gitutil.PatchEffect, repoRoot, slug, candidateID string, featureLock, worktreeMutationLock *candidatelock.Handle) (ExecutionResult, error) {
	return ExecutionResult{}, nil
}

func executeWithPlan(reference, upstream *ReferenceRoot, patch string, effects []gitutil.PatchEffect, plan Plan, repoRoot, slug, candidateID string, featureLock, worktreeMutationLock *candidatelock.Handle) (ExecutionResult, error) {
	return ExecutionResult{}, nil
}
`
	if err := checkExecutionTypedBoundary(mustParseCandidateSource(t, "execute.go", source)); err == nil {
		t.Fatal("execution boundary guard accepted old bare-root forwarding signatures")
	}
}

func TestGitEnvelopeGuard(t *testing.T) {
	files, err := candidateProductionFiles()
	if err != nil {
		t.Fatal(err)
	}
	if err := checkGitEnvelope(files); err != nil {
		t.Fatal(err)
	}
}

func TestGitEnvelopeGuardSensitivity(t *testing.T) {
	rootSource, err := os.ReadFile("root.go")
	if err != nil {
		t.Fatal(err)
	}
	baseline := mustParseCandidateSource(t, "root.go", string(rootSource))
	readFlagsUnix := mustParseCandidateSource(t, "readflags_unix.go", mustReadCandidateSource(t, "readflags_unix.go"))
	readFlagsOther := mustParseCandidateSource(t, "readflags_other.go", mustReadCandidateSource(t, "readflags_other.go"))
	files := []candidateFile{
		{path: "root.go", file: baseline},
		{path: "readflags_unix.go", file: readFlagsUnix},
		{path: "readflags_other.go", file: readFlagsOther},
	}
	if err := checkGitEnvelope(files); err != nil {
		t.Fatalf("baseline guard failed: %v", err)
	}

	for _, mutation := range []struct {
		name string
		src  string
	}{
		{
			name: "missing-hardened-envelope",
			src: strings.Replace(
				string(rootSource),
				"cmd.Env = gitutil.HardenedNoLazyGitEnv(extraEnv...)",
				"cmd.Env = os.Environ()",
				1,
			),
		},
		{
			name: "env-overwrite",
			src: strings.Replace(
				string(rootSource),
				"stdout, err := cmd.Output()",
				"cmd.Env = append(os.Environ(), \"GIT_NO_LAZY_FETCH=0\")\n\tstdout, err := cmd.Output()",
				1,
			),
		},
		{
			name: "runner-wrapper",
			src: strings.Replace(
				string(rootSource),
				"cmd := exec.Command(\"git\", args...)",
				"runner := exec.Command\n\tcmd := runner(\"git\", args...)",
				1,
			),
		},
		{
			name: "dynamic-git-name",
			src: strings.Replace(
				string(rootSource),
				"cmd := exec.Command(\"git\", args...)",
				"commandName := \"git\"\n\tcmd := exec.Command(commandName, args...)",
				1,
			),
		},
		{
			name: "unregistered-path-call",
			src: strings.Replace(
				string(rootSource),
				"return strings.Contains(trimmed, \"is not a working tree\") ||",
				"_ = runGitViaHelper(\"git\", \"status\")\n\treturn strings.Contains(trimmed, \"is not a working tree\") ||",
				1,
			),
		},
		{
			name: "package-level-executor-alias",
			src:  string(rootSource) + "\nvar hiddenRunner = exec.Command\n",
		},
		{
			name: "command-handle-alias",
			src: strings.Replace(string(rootSource), "stdout, err := cmd.Output()",
				"alias := cmd\n\talias.Env = nil\n\tstdout, err := cmd.Output()", 1),
		},
		{
			name: "conditional-envelope",
			src: strings.Replace(string(rootSource), "cmd.Env = gitutil.HardenedNoLazyGitEnv(extraEnv...)",
				"if len(extraEnv) > 0 { cmd.Env = gitutil.HardenedNoLazyGitEnv(extraEnv...) }", 1),
		},
		{
			name: "missing-root-directory",
			src:  strings.Replace(string(rootSource), "cmd.Dir = repo.path", "", 1),
		},
		{
			name: "wrong-root-directory",
			src:  strings.Replace(string(rootSource), "cmd.Dir = repo.path", `cmd.Dir = "."`, 1),
		},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			if mutation.src == string(rootSource) {
				t.Fatal("mutation did not change source")
			}
			mutant := []candidateFile{
				{path: "root.go", file: mustParseCandidateSource(t, "root.go", mutation.src)},
				{path: "readflags_unix.go", file: readFlagsUnix},
				{path: "readflags_other.go", file: readFlagsOther},
			}
			if err := checkGitEnvelope(mutant); err == nil {
				t.Fatal("git envelope guard accepted mutation")
			}
		})
	}
}

func TestCallerHeldLockBoundary(t *testing.T) {
	rootSource, err := os.ReadFile("root.go")
	if err != nil {
		t.Fatal(err)
	}
	file := mustParseCandidateSource(t, "root.go", string(rootSource))
	if err := checkNoInternalLockAcquireRelease(file); err != nil {
		t.Fatal(err)
	}
}

func TestCallerHeldLockBoundarySensitivity(t *testing.T) {
	rootSource, err := os.ReadFile("root.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct {
		name string
		src  string
	}{
		{
			name: "try-acquire-reintroduced",
			src: strings.Replace(
				string(rootSource),
				"return nil, err",
				"_ = candidatelock.TryAcquire(r.featureLockPath())\n\treturn nil, err",
				1,
			),
		},
		{
			name: "release-reintroduced",
			src: strings.Replace(
				string(rootSource),
				"return &borrowedMutationLocks{releaseGlobal: releaseGlobal, releaseFeature: releaseFeature}, nil",
				"_ = featureLock.Release()\n\treturn &borrowedMutationLocks{releaseGlobal: releaseGlobal, releaseFeature: releaseFeature}, nil",
				1,
			),
		},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			if mutation.src == string(rootSource) {
				t.Fatal("mutation did not change source")
			}
			if err := checkNoInternalLockAcquireRelease(mustParseCandidateSource(t, "root.go", mutation.src)); err == nil {
				t.Fatal("lock boundary guard accepted mutation")
			}
		})
	}
}

func TestNoNewDependencyContract(t *testing.T) {
	moduleRoot := findModuleRoot(t)
	goModBytes, err := os.ReadFile(filepath.Join(moduleRoot, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := candidateProductionFiles()
	if err != nil {
		t.Fatal(err)
	}
	candidatelockSources, err := packageSources(filepath.Join(moduleRoot, "internal", "candidatelock"))
	if err != nil {
		t.Fatal(err)
	}
	candidateSources := map[string]string{}
	for _, file := range files {
		body, readErr := os.ReadFile(file.path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		candidateSources[file.path] = string(body)
	}
	for name, src := range candidatelockSources {
		candidateSources[filepath.ToSlash(filepath.Join("internal/candidatelock", name))] = src
	}
	if err := validateDependencyContract(string(goModBytes), candidateSources); err != nil {
		t.Fatal(err)
	}
}

func TestNoNewDependencyContractSensitivity(t *testing.T) {
	cleanGoMod := `module github.com/tesseracode/tesserapatch

require (
	github.com/spf13/cobra v1.8.0
)
`
	cleanSources := map[string]string{
		"internal/candidate/root.go": `package candidate
import "github.com/tesseracode/tesserapatch/internal/gitutil"
var _ = gitutil.NoLazyFetchEnv
`,
		"internal/candidatelock/lock.go": `package candidatelock
import "syscall"
var _ = syscall.EINVAL
`,
	}
	if err := validateDependencyContract(cleanGoMod, cleanSources); err != nil {
		t.Fatalf("clean fixture rejected: %v", err)
	}
	mutants := []struct {
		name   string
		goMod  string
		source map[string]string
	}{
		{
			name:   "forbidden-module",
			goMod:  cleanGoMod + "require golang.org/x/sys v0.33.0\n",
			source: cleanSources,
		},
		{
			name:   "unused-new-module",
			goMod:  cleanGoMod + "require example.com/unreviewed/module v1.0.0\n",
			source: cleanSources,
		},
		{
			name:  "forbidden-import",
			goMod: cleanGoMod,
			source: map[string]string{
				"internal/candidate/root.go": `package candidate
import "golang.org/x/sys/unix"
var _ = unix.O_RDONLY
`,
				"internal/candidatelock/lock.go": `package candidatelock
import "syscall"
var _ = syscall.EINVAL
`,
			},
		},
		{
			name:  "new-external-import",
			goMod: cleanGoMod,
			source: map[string]string{
				"internal/candidate/root.go": `package candidate
import dependency "example.com/new/dependency"
var _ = dependency.Symbol
`,
				"internal/candidatelock/lock.go": `package candidatelock
`,
			},
		},
	}
	for _, mutant := range mutants {
		t.Run(mutant.name, func(t *testing.T) {
			if err := validateDependencyContract(mutant.goMod, mutant.source); err == nil {
				t.Fatal("dependency contract accepted mutation")
			}
		})
	}
}

type candidateFile struct {
	path string
	file *ast.File
}

func candidateProductionFiles() ([]candidateFile, error) {
	names, err := filepath.Glob("*.go")
	if err != nil {
		return nil, err
	}
	var files []candidateFile
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return nil, err
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), name, string(body), parser.ParseComments)
		if err != nil {
			return nil, err
		}
		files = append(files, candidateFile{path: name, file: parsed})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	return files, nil
}

func checkBareRootConstructors(files []candidateFile) error {
	required := map[string]bool{"OpenReferenceRoot": false, "CreateCandidateRoot": false}
	for _, file := range files {
		imports := importPathIndex(file.file)
		allowedOpeners := map[*ast.SelectorExpr]bool{}
		allowedFactories := map[*ast.Ident]bool{}
		for _, decl := range file.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if containsCall(fn.Body, "openHeldRoot") {
				if fn.Recv != nil || (fn.Name.Name != "OpenReferenceRoot" && fn.Name.Name != "CreateCandidateRoot") {
					return fmt.Errorf("non-constructor function calls openHeldRoot: %s", fn.Name.Name)
				}
				required[fn.Name.Name] = true
			}
			if fn.Name.Name == "openHeldRoot" {
				allowedFactories[fn.Name] = true
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok && isCallToIdent(call, "openHeldRoot") {
					allowedFactories[call.Fun.(*ast.Ident)] = true
				}
				return true
			})
			if err := enforceNoAlternateRootOpeners(file.file, fn); err != nil {
				return err
			}
			if fn.Name.Name == "openHeldRoot" && fn.Recv == nil {
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					if call, ok := node.(*ast.CallExpr); ok && isSelectorCallOnImport(call, imports, "os", "OpenRoot") {
						allowedOpeners[call.Fun.(*ast.SelectorExpr)] = true
					}
					return true
				})
			}
		}
		var forbidden bool
		ast.Inspect(file.file, func(node ast.Node) bool {
			if id, ok := node.(*ast.Ident); ok && id.Name == "openHeldRoot" && !allowedFactories[id] {
				forbidden = true
			}
			if imp, ok := node.(*ast.ImportSpec); ok && imp.Name != nil && imp.Name.Name == "." {
				forbidden = true
			}
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			qualifier, ok := selector.X.(*ast.Ident)
			if !ok || imports[qualifier.Name] != "os" {
				return true
			}
			switch selector.Sel.Name {
			case "Open", "OpenFile", "ReadFile", "StartProcess":
				forbidden = true
			case "OpenRoot":
				forbidden = forbidden || !allowedOpeners[selector]
			}
			return true
		})
		if forbidden {
			return fmt.Errorf("%s contains an unregistered filesystem opener or alias", file.path)
		}
	}
	if err := checkConstructorSignature(files, "OpenReferenceRoot", []string{"repoRoot", "commit"}, "ReferenceRoot"); err != nil {
		return err
	}
	if err := checkCreateCandidateSignature(files); err != nil {
		return err
	}
	if err := checkPruneSignature(files); err != nil {
		return err
	}
	for name, hit := range required {
		if !hit {
			return fmt.Errorf("constructor boundary lost openHeldRoot call: %s", name)
		}
	}
	return nil
}

func enforceNoAlternateRootOpeners(file *ast.File, fn *ast.FuncDecl) error {
	if fn.Body == nil {
		return nil
	}
	importPathByAlias := importPathIndex(file)
	violation := false
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if isCallToIdent(call, "openHeldRoot") &&
			fn.Name.Name != "OpenReferenceRoot" && fn.Name.Name != "CreateCandidateRoot" {
			violation = true
			return false
		}
		if isSelectorCallOnImport(call, importPathByAlias, "os", "OpenRoot") && fn.Name.Name != "openHeldRoot" {
			violation = true
			return false
		}
		for _, name := range []string{"Open", "OpenFile", "ReadFile", "StartProcess"} {
			if isSelectorCallOnImport(call, importPathByAlias, "os", name) {
				violation = true
				return false
			}
		}
		return true
	})
	if violation {
		return fmt.Errorf("function %s introduces an alternate bare-root entry", fn.Name.Name)
	}
	return nil
}

func checkCreateCandidateSignature(files []candidateFile) error {
	for _, file := range files {
		for _, decl := range file.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "CreateCandidateRoot" || fn.Recv != nil {
				continue
			}
			if fn.Type.Params == nil {
				return fmt.Errorf("CreateCandidateRoot parameter shape changed")
			}
			expectNames := []string{"repoRoot", "slug", "candidateID", "commit", "featureLock", "worktreeMutationLock"}
			gotNames := make([]string, 0, 6)
			var gotTypes []ast.Expr
			for _, field := range fn.Type.Params.List {
				for _, name := range field.Names {
					gotNames = append(gotNames, name.Name)
					gotTypes = append(gotTypes, field.Type)
				}
			}
			if len(gotNames) != len(expectNames) {
				return fmt.Errorf("CreateCandidateRoot parameter count changed: got %d want %d", len(gotNames), len(expectNames))
			}
			if strings.Join(gotNames, ",") != strings.Join(expectNames, ",") {
				return fmt.Errorf("CreateCandidateRoot parameters changed: got %v want %v", gotNames, expectNames)
			}
			for _, paramType := range gotTypes[:4] {
				id, ok := paramType.(*ast.Ident)
				if !ok || id.Name != "string" {
					return fmt.Errorf("CreateCandidateRoot first four params must remain string")
				}
			}
			for _, paramType := range gotTypes[4:] {
				if !isCandidatelockHandlePointer(paramType) {
					return fmt.Errorf("CreateCandidateRoot lock params must remain *candidatelock.Handle")
				}
			}
			if fn.Type.Results == nil || len(fn.Type.Results.List) != 2 {
				return fmt.Errorf("CreateCandidateRoot result shape changed")
			}
			ptr, ok := fn.Type.Results.List[0].Type.(*ast.StarExpr)
			if !ok {
				return fmt.Errorf("CreateCandidateRoot first result must remain pointer")
			}
			ret, ok := ptr.X.(*ast.Ident)
			if !ok || ret.Name != "CandidateRoot" {
				return fmt.Errorf("CreateCandidateRoot first result changed")
			}
			errID, ok := fn.Type.Results.List[1].Type.(*ast.Ident)
			if !ok || errID.Name != "error" {
				return fmt.Errorf("CreateCandidateRoot second result must remain error")
			}
			return nil
		}
	}
	return fmt.Errorf("CreateCandidateRoot constructor not found")
}

func checkPruneSignature(files []candidateFile) error {
	for _, file := range files {
		for _, decl := range file.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "Prune" || fn.Recv == nil {
				continue
			}
			if len(fn.Recv.List) != 1 {
				return fmt.Errorf("Prune receiver changed")
			}
			recvPtr, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				return fmt.Errorf("Prune receiver must remain pointer")
			}
			recv, ok := recvPtr.X.(*ast.Ident)
			if !ok || recv.Name != "CandidateRoot" {
				return fmt.Errorf("Prune receiver changed")
			}
			if fn.Type.Params == nil {
				return fmt.Errorf("Prune parameter shape changed")
			}
			expectNames := []string{"featureLock", "worktreeMutationLock"}
			var got []string
			var gotTypes []ast.Expr
			for _, field := range fn.Type.Params.List {
				for _, name := range field.Names {
					got = append(got, name.Name)
					gotTypes = append(gotTypes, field.Type)
				}
			}
			if len(got) != len(expectNames) {
				return fmt.Errorf("Prune parameter count changed: got %d want %d", len(got), len(expectNames))
			}
			for _, paramType := range gotTypes {
				if !isCandidatelockHandlePointer(paramType) {
					return fmt.Errorf("Prune parameters must remain *candidatelock.Handle")
				}
			}
			if strings.Join(got, ",") != strings.Join(expectNames, ",") {
				return fmt.Errorf("Prune parameter names changed: got %v want %v", got, expectNames)
			}
			if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
				return fmt.Errorf("Prune results changed")
			}
			errID, ok := fn.Type.Results.List[0].Type.(*ast.Ident)
			if !ok || errID.Name != "error" {
				return fmt.Errorf("Prune must return error")
			}
			return nil
		}
	}
	return fmt.Errorf("Prune method not found")
}

func checkExecutionTypedBoundary(file *ast.File) error {
	checkSignature := func(fnName string, expected []string) error {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != fnName {
				continue
			}
			if fn.Type.Params == nil {
				return fmt.Errorf("%s parameter shape changed", fnName)
			}
			var names []string
			for _, field := range fn.Type.Params.List {
				for _, name := range field.Names {
					names = append(names, name.Name)
				}
			}
			if strings.Join(names, ",") != strings.Join(expected, ",") {
				return fmt.Errorf("%s parameters changed: got %v want %v", fnName, names, expected)
			}
			for _, forbidden := range []string{"repoRoot", "worktreeMutationLock"} {
				for _, name := range names {
					if name == forbidden {
						return fmt.Errorf("%s reintroduced forbidden parameter %s", fnName, forbidden)
					}
				}
			}
			return nil
		}
		return fmt.Errorf("%s function not found", fnName)
	}
	if err := checkSignature("Execute", []string{"reference", "upstream", "patch", "effects", "slug", "candidateID", "featureLock"}); err != nil {
		return err
	}
	return checkSignature("executeWithPlan", []string{"reference", "upstream", "patch", "effects", "plan", "slug", "candidateID", "featureLock"})
}

func isCandidatelockHandlePointer(expr ast.Expr) bool {
	ptr, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := ptr.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "candidatelock" && sel.Sel.Name == "Handle"
}

func checkConstructorSignature(files []candidateFile, name string, params []string, result string) error {
	for _, file := range files {
		for _, decl := range file.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != name || fn.Recv != nil {
				continue
			}
			if fn.Type.Params == nil {
				return fmt.Errorf("%s parameter shape changed", name)
			}
			var names []string
			for _, field := range fn.Type.Params.List {
				id, ok := field.Type.(*ast.Ident)
				if !ok || id.Name != "string" {
					return fmt.Errorf("%s parameters must stay string-typed", name)
				}
				for _, param := range field.Names {
					names = append(names, param.Name)
				}
			}
			if len(names) != len(params) {
				return fmt.Errorf("%s parameter count changed", name)
			}
			for i, want := range params {
				if names[i] != want {
					return fmt.Errorf("%s parameter %d = %s, want %s", name, i, names[i], want)
				}
			}
			if fn.Type.Results == nil || len(fn.Type.Results.List) != 2 {
				return fmt.Errorf("%s result shape changed", name)
			}
			ptr, ok := fn.Type.Results.List[0].Type.(*ast.StarExpr)
			if !ok {
				return fmt.Errorf("%s first result is no longer a pointer", name)
			}
			ret, ok := ptr.X.(*ast.Ident)
			if !ok || ret.Name != result {
				return fmt.Errorf("%s first result = %v, want *%s", name, ret, result)
			}
			errID, ok := fn.Type.Results.List[1].Type.(*ast.Ident)
			if !ok || errID.Name != "error" {
				return fmt.Errorf("%s second result must remain error", name)
			}
			return nil
		}
	}
	return fmt.Errorf("constructor %s not found", name)
}

func checkGitEnvelope(files []candidateFile) error {
	var rootFile *ast.File
	for _, file := range files {
		if file.path == "root.go" {
			rootFile = file.file
			break
		}
	}
	if rootFile == nil {
		return fmt.Errorf("root.go not found in source inventory")
	}
	var gitCalls int
	var envAssigns int
	var invalid bool
	var hasHardenedCall bool
	for _, file := range files {
		importPathByAlias := importPathIndex(file.file)
		allowedExecutors := map[*ast.SelectorExpr]bool{}
		for _, decl := range file.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fn.Name.Name == "runGitWithIdentity" {
				if err := checkCommandHandleUse(fn); err != nil {
					return err
				}
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.CallExpr:
					if isSelectorCallOnImport(n, importPathByAlias, "os/exec", "Command") ||
						isSelectorCallOnImport(n, importPathByAlias, "os/exec", "CommandContext") {
						gitCalls++
						if fn.Name.Name != "runGitWithIdentity" || !firstArgIsLiteralGit(n) {
							invalid = true
						} else {
							allowedExecutors[n.Fun.(*ast.SelectorExpr)] = true
						}
					}
					if firstArgIsLiteralGit(n) &&
						!(isSelectorCallOnImport(n, importPathByAlias, "os/exec", "Command") ||
							isSelectorCallOnImport(n, importPathByAlias, "os/exec", "CommandContext")) {
						invalid = true
					}
					if fn.Name.Name == "runGitWithIdentity" && isHardenedEnvCall(n) {
						hasHardenedCall = true
					}
				case *ast.AssignStmt:
					for _, rhs := range n.Rhs {
						if selector, ok := rhs.(*ast.SelectorExpr); ok {
							if ident, ok := selector.X.(*ast.Ident); ok &&
								importPathByAlias[ident.Name] == "os/exec" &&
								(selector.Sel.Name == "Command" || selector.Sel.Name == "CommandContext") {
								invalid = true
							}
						}
					}
					if fn.Name.Name != "runGitWithIdentity" {
						return true
					}
					for index, lhs := range n.Lhs {
						sel, ok := lhs.(*ast.SelectorExpr)
						if !ok {
							continue
						}
						ident, ok := sel.X.(*ast.Ident)
						if !ok || ident.Name != "cmd" || sel.Sel.Name != "Env" {
							continue
						}
						envAssigns++
						if index >= len(n.Rhs) || !isHardenedEnvCallExpr(n.Rhs[index]) {
							invalid = true
						}
					}
				}
				return true
			})
		}
		ast.Inspect(file.file, func(node ast.Node) bool {
			if imp, ok := node.(*ast.ImportSpec); ok && imp.Name != nil && imp.Name.Name == "." {
				invalid = true
			}
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			if importPathByAlias[pkg.Name] == "os/exec" &&
				(selector.Sel.Name == "Command" || selector.Sel.Name == "CommandContext") &&
				!allowedExecutors[selector] {
				invalid = true
			}
			return true
		})
	}
	if gitCalls != 1 {
		return fmt.Errorf("git subprocess call sites = %d, want exactly 1 (runGitWithIdentity)", gitCalls)
	}
	if invalid {
		return fmt.Errorf("git subprocess contract changed (non-runGitWithIdentity call or env mutation)")
	}
	if envAssigns != 1 {
		return fmt.Errorf("cmd.Env assignment contract changed: got %d", envAssigns)
	}
	if !hasHardenedCall {
		return fmt.Errorf("runGitWithIdentity no longer uses gitutil.HardenedNoLazyGitEnv")
	}
	return nil
}

func checkCommandHandleUse(fn *ast.FuncDecl) error {
	if fn.Body == nil {
		return fmt.Errorf("Git runner has no inspectable body")
	}
	parents := map[ast.Node]ast.Node{}
	var stack []ast.Node
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if len(stack) > 0 {
			parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	invalid := false
	fields := map[string]int{}
	var envPosition, outputPosition token.Pos
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		id, ok := node.(*ast.Ident)
		if !ok || id.Name != "cmd" {
			return true
		}
		switch parent := parents[id].(type) {
		case *ast.AssignStmt:
			if parent.Tok != token.DEFINE || len(parent.Lhs) != 1 || parent.Lhs[0] != id ||
				parents[parent] != fn.Body {
				invalid = true
			}
		case *ast.SelectorExpr:
			if parent.X != id {
				invalid = true
				break
			}
			if parent.Sel.Name == "Output" {
				fields["Output"]++
				outputPosition = parent.Pos()
				call, ok := parents[parent].(*ast.CallExpr)
				if !ok || call.Fun != parent {
					invalid = true
				}
				break
			}
			assignment, ok := parents[parent].(*ast.AssignStmt)
			if !ok || len(assignment.Lhs) != 1 || assignment.Lhs[0] != parent ||
				parents[assignment] != fn.Body ||
				(parent.Sel.Name != "Dir" && parent.Sel.Name != "Env" && parent.Sel.Name != "Stderr") {
				invalid = true
				break
			}
			fields[parent.Sel.Name]++
			if parent.Sel.Name == "Env" {
				envPosition = parent.Pos()
			}
			if parent.Sel.Name == "Dir" {
				if len(assignment.Rhs) != 1 {
					invalid = true
					break
				}
				value, ok := assignment.Rhs[0].(*ast.SelectorExpr)
				if !ok {
					invalid = true
					break
				}
				receiver, ok := value.X.(*ast.Ident)
				if !ok || receiver.Name != "repo" || value.Sel.Name != "path" {
					invalid = true
				}
			}
		default:
			invalid = true
		}
		return true
	})
	if invalid || fields["Dir"] != 1 || fields["Env"] != 1 || fields["Stderr"] != 1 ||
		fields["Output"] != 1 || envPosition >= outputPosition {
		return fmt.Errorf("Git command handle escapes or is conditionally configured")
	}
	return nil
}

func firstArgIsLiteralGit(call *ast.CallExpr) bool {
	if len(call.Args) == 0 {
		return false
	}
	first, ok := call.Args[0].(*ast.BasicLit)
	if !ok || first.Kind != token.STRING {
		return false
	}
	name, err := strconv.Unquote(first.Value)
	return err == nil && name == "git"
}

func isHardenedEnvCall(call *ast.CallExpr) bool {
	return isHardenedEnvCallExpr(call)
}

func isHardenedEnvCallExpr(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Obj == nil && pkg.Name == "gitutil" && selector.Sel.Name == "HardenedNoLazyGitEnv"
}

func containsCall(body *ast.BlockStmt, callee string) bool {
	if body == nil {
		return false
	}
	hit := false
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := call.Fun.(*ast.Ident)
		if ok && ident.Name == callee {
			hit = true
			return false
		}
		return true
	})
	return hit
}

func importPathIndex(file *ast.File) map[string]string {
	aliases := map[string]string{}
	for _, imp := range file.Imports {
		pathValue, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		alias := filepath.Base(pathValue)
		if imp.Name != nil && imp.Name.Name != "_" && imp.Name.Name != "." {
			alias = imp.Name.Name
		}
		aliases[alias] = pathValue
	}
	return aliases
}

func isSelectorCallOnImport(call *ast.CallExpr, aliasToPath map[string]string, wantPath, wantSelector string) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	alias, ok := selector.X.(*ast.Ident)
	if !ok || selector.Sel.Name != wantSelector {
		return false
	}
	return aliasToPath[alias.Name] == wantPath
}

func isCallToIdent(call *ast.CallExpr, name string) bool {
	ident, ok := call.Fun.(*ast.Ident)
	return ok && ident.Name == name
}

func checkNoInternalLockAcquireRelease(file *ast.File) error {
	imports := importPathIndex(file)
	var violation bool
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if selector.Sel.Name == "Release" {
			violation = true
		}
		if pkg, ok := selector.X.(*ast.Ident); ok &&
			imports[pkg.Name] == "github.com/tesseracode/tesserapatch/internal/candidatelock" &&
			selector.Sel.Name == "TryAcquire" {
			violation = true
		}
		return true
	})
	if violation {
		return fmt.Errorf("candidate source acquires or releases caller-owned lock capabilities")
	}
	return nil
}

func validateDependencyContract(goMod string, sources map[string]string) error {
	requires := parseGoModRequires(goMod)
	for module := range requires {
		switch module {
		case "github.com/spf13/cobra", "github.com/spf13/pflag", "github.com/inconshreveable/mousetrap":
		default:
			return fmt.Errorf("go.mod added forbidden module dependency %s", module)
		}
	}
	for name, src := range sources {
		file, err := parser.ParseFile(token.NewFileSet(), name, src, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse %s: %w", name, err)
		}
		for _, imp := range file.Imports {
			pathValue, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return fmt.Errorf("unquote import %s: %w", name, err)
			}
			if pathValue == "golang.org/x/sys" || strings.HasPrefix(pathValue, "golang.org/x/sys/") {
				return fmt.Errorf("%s imports forbidden dependency %s", name, pathValue)
			}
			if isExternalImport(pathValue) && !strings.HasPrefix(pathValue, "github.com/tesseracode/tesserapatch/") {
				return fmt.Errorf("%s imports unreviewed external dependency %s", name, pathValue)
			}
		}
	}
	return nil
}

func parseGoModRequires(goMod string) map[string]bool {
	modules := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(goMod))
	inRequire := false
	for scanner.Scan() {
		line := scanner.Text()
		if comment := strings.Index(line, "//"); comment >= 0 {
			line = line[:comment]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "require ("):
			inRequire = true
			continue
		case inRequire && line == ")":
			inRequire = false
			continue
		case inRequire:
			fields := strings.Fields(line)
			if len(fields) > 0 {
				modules[fields[0]] = true
			}
		case strings.HasPrefix(line, "require "):
			fields := strings.Fields(strings.TrimPrefix(line, "require "))
			if len(fields) > 0 {
				modules[fields[0]] = true
			}
		}
	}
	return modules
}

func isExternalImport(path string) bool {
	first := path
	if idx := strings.Index(path, "/"); idx >= 0 {
		first = path[:idx]
	}
	return strings.Contains(first, ".")
}

func packageSources(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	sources := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		sources[name] = string(body)
	}
	return sources, nil
}

func mustParseCandidateSource(t *testing.T, name, src string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), name, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return file
}

func mustReadCandidateSource(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(body)
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above internal/candidate")
		}
		dir = parent
	}
}
