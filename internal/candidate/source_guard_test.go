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
)

func TestConstructorSignatures(t *testing.T) {
	var _ func(string, string) (*ReferenceRoot, error) = OpenReferenceRoot
	var _ func(string, string, string, string) (*CandidateRoot, error) = CreateCandidateRoot
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
	want := []string{"readflags_other.go", "readflags_unix.go", "root.go"}
	if strings.Join(names, "\n") != strings.Join(want, "\n") {
		t.Fatalf("candidate production file inventory changed:\n got %v\nwant %v", names, want)
	}
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
	fixture := mustParseCandidateSource(t, "synthetic.go", `package candidate
func OpenReferenceRoot(repoRoot string, commit string) (*ReferenceRoot, error) { return nil, nil }
func CreateCandidateRoot(repoRoot, slug, candidateID, commit string) (*CandidateRoot, error) { return nil, nil }
func helper(workspace string) (*ReferenceRoot, error) {
	return openRepositoryRoot(workspace)
}
`)
	if err := checkBareRootConstructors([]candidateFile{{path: "synthetic.go", file: fixture}}); err == nil {
		t.Fatal("boundary guard accepted non-constructor bare-root entry")
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
			name: "missing-no-lazy",
			src: strings.Replace(
				string(rootSource),
				"gitutil.NoLazyFetchEnv",
				"gitutil.CLocaleEnv",
				1,
			),
		},
		{
			name: "missing-env-assignment",
			src: strings.Replace(
				string(rootSource),
				"cmd.Env = candidateGitEnv(extraEnv)",
				"",
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
			name: "extra-git-call",
			src: strings.Replace(
				string(rootSource),
				"return strings.Contains(trimmed, \"is not a working tree\") ||",
				"cmd := exec.Command(\"git\", \"status\")\n\t_ = cmd\n\treturn strings.Contains(trimmed, \"is not a working tree\") ||",
				1,
			),
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
	required := map[string]bool{
		"OpenReferenceRoot":   false,
		"CreateCandidateRoot": false,
	}
	for _, file := range files {
		for _, decl := range file.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			if containsCall(fn.Body, "openRepositoryRoot") {
				if fn.Name.Name != "OpenReferenceRoot" && fn.Name.Name != "CreateCandidateRoot" {
					return fmt.Errorf("non-constructor function calls openRepositoryRoot: %s", fn.Name.Name)
				}
				required[fn.Name.Name] = true
			}
		}
	}
	if err := checkConstructorSignature(files, "OpenReferenceRoot", []string{"repoRoot", "commit"}, "ReferenceRoot"); err != nil {
		return err
	}
	if err := checkConstructorSignature(files, "CreateCandidateRoot", []string{"repoRoot", "slug", "candidateID", "commit"}, "CandidateRoot"); err != nil {
		return err
	}
	for name, hit := range required {
		if !hit {
			return fmt.Errorf("constructor boundary lost openRepositoryRoot call: %s", name)
		}
	}
	return nil
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
	var gitCalls int
	var envAssigns int
	var invalid bool
	var hasNoLazy bool
	var hasCLocale bool
	var hasScrubber bool
	var scrubbedVars = map[string]bool{}

	for _, file := range files {
		for _, decl := range file.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.CallExpr:
					if !isGitExecCall(n) {
						return true
					}
					gitCalls++
					if fn.Name.Name != "runGitWithIdentity" {
						invalid = true
					}
				case *ast.AssignStmt:
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
						if index >= len(n.Rhs) || !isCandidateGitEnvCall(n.Rhs[index]) {
							invalid = true
						}
					}
				case *ast.SelectorExpr:
					if fn.Name.Name == "candidateGitEnv" {
						if pkg, ok := n.X.(*ast.Ident); ok && pkg.Name == "gitutil" {
							switch n.Sel.Name {
							case "NoLazyFetchEnv":
								hasNoLazy = true
							case "CLocaleEnv":
								hasCLocale = true
							}
						}
					}
				case *ast.Ident:
					if fn.Name.Name == "candidateGitEnv" && n.Name == "scrubGitVariable" {
						hasScrubber = true
					}
				case *ast.BasicLit:
					if fn.Name.Name == "scrubGitVariable" && n.Kind == token.STRING {
						text, unquoteErr := strconv.Unquote(n.Value)
						if unquoteErr == nil {
							scrubbedVars[text] = true
						}
					}
				}
				return true
			})
		}
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
	if !hasNoLazy || !hasCLocale || !hasScrubber {
		return fmt.Errorf("candidate git environment lost NoLazy/CLocale/scrubber wiring")
	}
	for _, required := range []string{
		"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE",
		"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
		"GIT_CONFIG", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM", "GIT_CONFIG_COUNT",
		"GIT_REPLACE_REF_BASE",
	} {
		if !scrubbedVars[required] {
			return fmt.Errorf("scrubGitVariable no longer scrubs %s", required)
		}
	}
	return nil
}

func isGitExecCall(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "exec" {
		return false
	}
	if selector.Sel.Name != "Command" && selector.Sel.Name != "CommandContext" {
		return false
	}
	if len(call.Args) == 0 {
		return false
	}
	first, ok := call.Args[0].(*ast.BasicLit)
	if !ok || first.Kind != token.STRING {
		return false
	}
	name, err := strconv.Unquote(first.Value)
	if err != nil {
		return false
	}
	return name == "git"
}

func isCandidateGitEnvCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	ident, ok := call.Fun.(*ast.Ident)
	return ok && ident.Name == "candidateGitEnv"
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

func validateDependencyContract(goMod string, sources map[string]string) error {
	requires := parseGoModRequires(goMod)
	for module := range requires {
		if module == "golang.org/x/sys" || strings.HasPrefix(module, "golang.org/x/sys/") {
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
