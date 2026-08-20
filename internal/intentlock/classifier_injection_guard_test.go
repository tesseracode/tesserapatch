package intentlock

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const filesystemClassifierEntryName = "AcquireWithFilesystemClassifier"

func TestFilesystemClassifierEntryHasNoProductionCaller(t *testing.T) {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
	declarations := 0
	var references []string
	command := exec.Command("git", "ls-files", "-z", "--", "*.go")
	command.Dir = repositoryRoot
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	for _, raw := range strings.Split(string(output), "\x00") {
		if raw == "" || strings.HasSuffix(raw, "_test.go") {
			continue
		}
		path := filepath.Join(repositoryRoot, filepath.FromSlash(raw))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read tracked production source %s: %v", raw, err)
		}
		inspection := inspectFilesystemClassifierEntry(path, data)
		declarations += inspection.declarations
		for _, position := range inspection.references {
			references = append(references, position)
		}
	}
	if declarations != 1 {
		t.Fatalf("%s declarations = %d, want exactly 1", filesystemClassifierEntryName, declarations)
	}
	if len(references) != 0 {
		t.Fatalf("production references to test-only classifier entry:\n%s", strings.Join(references, "\n"))
	}
}

func TestFilesystemClassifierEntryGuardSensitivity(t *testing.T) {
	fixture := []byte(`package fixture
import (
	"os"
	"github.com/tesseracode/tesserapatch/internal/intentlock"
)
func productionCall(path string) {
	_, _ = intentlock.AcquireWithFilesystemClassifier(
		path,
		func(*os.File) (string, bool, error) { return "nfs", true, nil },
	)
}`)
	inspection := inspectFilesystemClassifierEntry("production_fixture.go", fixture)
	if len(inspection.references) == 0 {
		t.Fatal("production-call sensitivity fixture did not trip classifier entry guard")
	}
}

type classifierEntryInspection struct {
	declarations int
	references   []string
}

func inspectFilesystemClassifierEntry(filename string, source []byte) classifierEntryInspection {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filename, source, parser.AllErrors)
	if err != nil {
		return classifierEntryInspection{
			references: []string{filename + ": parse error: " + err.Error()},
		}
	}
	declarationNames := map[*ast.Ident]struct{}{}
	var result classifierEntryInspection
	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.FuncDecl:
			if value.Name.Name == filesystemClassifierEntryName {
				result.declarations++
				declarationNames[value.Name] = struct{}{}
			}
		case *ast.Ident:
			if value.Name != filesystemClassifierEntryName {
				return true
			}
			if _, declaration := declarationNames[value]; !declaration {
				result.references = append(
					result.references,
					fileSet.Position(value.Pos()).String(),
				)
			}
		}
		return true
	})
	return result
}
