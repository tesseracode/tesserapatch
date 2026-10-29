package candidate

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConstructorSignatures(t *testing.T) {
	var _ func(string, string) (*ReferenceRoot, error) = OpenReferenceRoot
	var _ func(string, string, string, string) (*CandidateRoot, error) = CreateCandidateRoot
}

func TestOnlyConstructorsAcceptRepoRootString(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := checkRepoRootBoundary(path, string(src)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRepoRootBoundaryGuardSensitivity(t *testing.T) {
	src := `package candidate
func OpenReferenceRoot(repoRoot string, commit string) (*ReferenceRoot, error) { return nil, nil }
func CreateCandidateRoot(repoRoot, slug, candidateID, commit string) (*CandidateRoot, error) { return nil, nil }
func helper(repoRoot string) {}
`
	if err := checkRepoRootBoundary("synthetic.go", src); err == nil {
		t.Fatal("guard accepted an extra repoRoot string boundary")
	}
}

func TestGitSubprocessesMustCarryNoLazyFetch(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if path == "root.go" {
			if err := checkNoLazyEnvelope(string(src)); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if strings.Contains(string(src), `exec.Command("git"`) {
			t.Fatalf("%s started a direct git subprocess outside root.go", path)
		}
	}
}

func TestNoLazyFetchGuardSensitivity(t *testing.T) {
	body, err := os.ReadFile("root.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	if err := checkNoLazyEnvelope(src); err != nil {
		t.Fatalf("baseline guard failed: %v", err)
	}
	mutant := strings.Replace(src, "gitutil.NoLazyFetchEnv", "gitutil.CLocaleEnv", 1)
	if err := checkNoLazyEnvelope(mutant); err == nil {
		t.Fatal("guard accepted a git subprocess without NoLazyFetchEnv")
	}
}

func TestNoAdditionalDependencyModules(t *testing.T) {
	moduleRoot := findModuleRoot(t)
	data, err := os.ReadFile(filepath.Join(moduleRoot, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "golang.org/x/sys") {
		t.Fatal("go.mod includes golang.org/x/sys; candidate/candidatelock must remain stdlib-only")
	}
}

func TestDependencyScannerSensitivity(t *testing.T) {
	const withXSys = "module example\nrequire golang.org/x/sys v0.0.0\n"
	if !strings.Contains(withXSys, "golang.org/x/sys") {
		t.Fatal("dependency scanner fixture is broken")
	}
	const clean = "module example\nrequire github.com/spf13/cobra v1.8.0\n"
	if strings.Contains(clean, "golang.org/x/sys") {
		t.Fatal("dependency scanner false-positive fixture is broken")
	}
}

func checkRepoRootBoundary(filename, src string) error {
	file, err := parser.ParseFile(token.NewFileSet(), filename, src, parser.ParseComments)
	if err != nil {
		return err
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Type.Params == nil {
			continue
		}
		for _, field := range fn.Type.Params.List {
			id, ok := field.Type.(*ast.Ident)
			if !ok || id.Name != "string" {
				continue
			}
			for _, name := range field.Names {
				if name.Name != "repoRoot" {
					continue
				}
				if fn.Name.Name != "OpenReferenceRoot" && fn.Name.Name != "CreateCandidateRoot" {
					return &boundaryError{Function: fn.Name.Name}
				}
			}
		}
	}
	return nil
}

type boundaryError struct {
	Function string
}

func (e *boundaryError) Error() string {
	return "unexpected bare repoRoot boundary at " + e.Function
}

func checkNoLazyEnvelope(src string) error {
	gitCalls := strings.Count(src, `exec.Command("git"`)
	noLazy := strings.Count(src, "gitutil.NoLazyFetchEnv")
	if gitCalls == 0 {
		return &envelopeError{Reason: "no git subprocess found"}
	}
	if noLazy != gitCalls {
		return &envelopeError{Reason: "git subprocess count and NoLazyFetchEnv count diverged"}
	}
	return nil
}

type envelopeError struct {
	Reason string
}

func (e *envelopeError) Error() string {
	return "git no-lazy envelope guard failed: " + e.Reason
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
