package intentlock

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestAuthoritySourceGuard(t *testing.T) {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(current)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var issues []string
	openRootCalls := 0
	rootDotCalls := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		result := inspectAuthoritySource(entry.Name(), string(data))
		for _, issue := range result.issues {
			issues = append(issues, entry.Name()+": "+issue)
		}
		openRootCalls += result.openRootCalls
		rootDotCalls += result.rootDotCalls
	}
	if len(issues) != 0 {
		t.Fatalf("authority source guard failed:\n%s", strings.Join(issues, "\n"))
	}
	if openRootCalls != 1 {
		t.Fatalf("os.OpenRoot calls = %d, want exactly 1", openRootCalls)
	}
	if rootDotCalls != 1 {
		t.Fatalf("root.Open(\".\") calls = %d, want exactly 1", rootDotCalls)
	}
}

func TestAuthoritySourceGuardSensitivity(t *testing.T) {
	fixtures := map[string]string{
		"naked fd": `package fixture
import ("os"; "syscall")
func bad(file *os.File) { var stat syscall.Statfs_t; _ = syscall.Fstatfs(int(file.Fd()), &stat) }`,
		"path statfs": `package fixture
import "syscall"
func bad(path string) { var stat syscall.Statfs_t; _ = syscall.Statfs(path, &stat) }`,
		"fuzzy prefix": `package fixture
import "strings"
func bad(name string) bool { return strings.HasPrefix(name, "fuse") }`,
		"fuzzy suffix": `package fixture
import "strings"
func bad(name string) bool { return strings.HasSuffix(name, "fs") }`,
		"user cache": `package fixture
import "os"
func bad() { _, _ = os.UserCacheDir() }`,
		"home": `package fixture
import "os"
func bad() string { return os.Getenv("HOME") }`,
		"xdg": `package fixture
import "os"
func bad() string { return os.LookupEnv("XDG_CACHE_HOME") }`,
		"local app data": `package fixture
import "os"
func bad() string { return os.Getenv("LocalAppData") }`,
		"rescap": `package fixture
import "example.test/internal/rescap"
var _ = rescap.Open`,
		"finalizer": `package fixture
import "runtime"
func bad(value any) { runtime.SetFinalizer(value, func(any){}) }`,
		"named semaphore": `package fixture
func sem_open(name string) {}
func bad() { sem_open("intentlock") }`,
		"channel semaphore": `package fixture
func bad() chan struct{} { return make(chan struct{}, 1) }`,
		"openfile intent lock": `package fixture
import "os"
func bad(dir string) { file, _ := os.OpenFile(dir+"/intent.lock", os.O_CREATE, 0600); _ = file }`,
		"create": `package fixture
import "os"
func bad(path string) { file, _ := os.Create(path); _ = file }`,
		"create temp": `package fixture
import "os"
func bad(path string) { file, _ := os.CreateTemp(path, "intent.lock"); _ = file }`,
		"mkdir": `package fixture
import "os"
func bad(path string) { _ = os.Mkdir(path, 0700) }`,
		"mkdir all": `package fixture
import "os"
func bad(path string) { _ = os.MkdirAll(path, 0700) }`,
		"write file": `package fixture
import "os"
func bad(path string) { _ = os.WriteFile(path, nil, 0600) }`,
		"key cache path": `package fixture
import "path/filepath"
func bad(root, key string) string { return filepath.Join(root, ".cache", key) }`,
		"flock outside callback": `package fixture
import "syscall"
func bad(fd int) { _ = syscall.Flock(fd, syscall.LOCK_EX) }`,
		"fstatfs outside callback": `package fixture
import "syscall"
func bad(fd int) { var stat syscall.Statfs_t; _ = syscall.Fstatfs(fd, &stat) }`,
		"captured fd used later": `package fixture
import "syscall"
type rawConn interface { Control(func(uintptr)) error }
func bad(raw rawConn) {
	var fd uintptr
	_ = raw.Control(func(descriptor uintptr) { fd = descriptor })
	_ = syscall.Flock(int(fd), syscall.LOCK_EX)
}`,
		"named callback": `package fixture
import "syscall"
type rawConn interface { Control(func(uintptr)) error }
func bad(raw rawConn) {
	callback := func(fd uintptr) { _ = syscall.Flock(int(fd), syscall.LOCK_EX) }
	_ = raw.Control(callback)
}`,
	}
	names := make([]string, 0, len(fixtures))
	for name := range fixtures {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			if issues := inspectAuthoritySource(name+".go", fixtures[name]).issues; len(issues) == 0 {
				t.Fatalf("sensitivity fixture did not trip guard:\n%s", fixtures[name])
			}
		})
	}
}

func TestAuthoritySourceGuardAllowsSyscallsOnlyInDirectControlCallback(t *testing.T) {
	fixture := `package fixture
import "syscall"
type rawConn interface { Control(func(uintptr)) error }
func good(raw rawConn) error {
	var stat syscall.Statfs_t
	var inner error
	if err := raw.Control(func(fd uintptr) {
		inner = syscall.Fstatfs(int(fd), &stat)
		if inner == nil {
			inner = syscall.Flock(int(fd), syscall.LOCK_EX)
		}
	}); err != nil {
		return err
	}
	return inner
}`
	if issues := inspectAuthoritySource("allowed.go", fixture).issues; len(issues) != 0 {
		t.Fatalf("direct Control callback rejected: %v", issues)
	}
}

type sourceInspection struct {
	issues        []string
	openRootCalls int
	rootDotCalls  int
}

func inspectAuthoritySource(filename, source string) sourceInspection {
	file, err := parser.ParseFile(token.NewFileSet(), filename, source, parser.AllErrors)
	if err != nil {
		return sourceInspection{issues: []string{"parse error: " + err.Error()}}
	}
	imports := importedPackageNames(file)
	parents := parentNodes(file)
	var result sourceInspection

	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.ImportSpec:
			importPath, err := strconv.Unquote(value.Path.Value)
			if err == nil && (strings.Contains(importPath, "/rescap") || path.Base(importPath) == "rescap") {
				result.issues = append(result.issues, "rescap reuse")
			}
			if err == nil && strings.Contains(strings.ToLower(importPath), "semaphore") {
				result.issues = append(result.issues, "semaphore dependency")
			}
		case *ast.Ident:
			name := strings.ToLower(value.Name)
			if name == "sem_open" || name == "semopen" {
				result.issues = append(result.issues, "named semaphore")
			}
		case *ast.ChanType:
			result.issues = append(result.issues, "channel semaphore")
		case *ast.SelectorExpr:
			packageName, ok := value.X.(*ast.Ident)
			if value.Sel.Name == "Fd" {
				result.issues = append(result.issues, "naked file descriptor")
			}
			if !ok {
				return true
			}
			switch importPathForName(imports, packageName.Name) {
			case "os":
				switch value.Sel.Name {
				case "OpenRoot":
					result.openRootCalls++
				case "OpenFile", "Create", "CreateTemp", "Mkdir", "MkdirAll", "WriteFile":
					result.issues = append(result.issues, "filesystem artifact API os."+value.Sel.Name)
				case "UserCacheDir":
					result.issues = append(result.issues, "user cache")
				case "UserHomeDir":
					result.issues = append(result.issues, "user home")
				case "Getenv", "LookupEnv":
					result.issues = append(result.issues, "user environment path")
				}
			case "path/filepath":
				result.issues = append(result.issues, "filepath-based lock/cache/key construction")
			case "runtime":
				if value.Sel.Name == "SetFinalizer" {
					result.issues = append(result.issues, "finalizer")
				}
			case "strings":
				if value.Sel.Name == "HasPrefix" {
					result.issues = append(result.issues, "fuzzy prefix classification")
				}
				if value.Sel.Name == "HasSuffix" {
					result.issues = append(result.issues, "fuzzy suffix classification")
				}
			case "syscall":
				switch value.Sel.Name {
				case "Statfs":
					result.issues = append(result.issues, "path-based statfs")
				case "Fstatfs", "Flock":
					call, directCall := parents[value].(*ast.CallExpr)
					if !directCall || call.Fun != value || !insideDirectControlCallback(call, parents) {
						result.issues = append(result.issues, "syscall."+value.Sel.Name+" outside direct Control callback")
					}
				}
			}
		case *ast.CallExpr:
			selector, ok := value.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if receiver, ok := selector.X.(*ast.Ident); ok &&
				receiver.Name == "root" && selector.Sel.Name == "Open" &&
				hasSingleStringArgument(value, ".") {
				result.rootDotCalls++
			}
		}
		return true
	})
	return result
}

func importedPackageNames(file *ast.File) map[string]string {
	imports := make(map[string]string)
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(importPath)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = importPath
	}
	return imports
}

func importPathForName(imports map[string]string, name string) string {
	return imports[name]
}

func parentNodes(root ast.Node) map[ast.Node]ast.Node {
	parents := make(map[ast.Node]ast.Node)
	var stack []ast.Node
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if len(stack) != 0 {
			parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	return parents
}

func insideDirectControlCallback(call *ast.CallExpr, parents map[ast.Node]ast.Node) bool {
	for node := ast.Node(call); node != nil; node = parents[node] {
		literal, ok := node.(*ast.FuncLit)
		if !ok {
			continue
		}
		parentCall, ok := parents[literal].(*ast.CallExpr)
		if !ok {
			return false
		}
		selector, ok := parentCall.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Control" || len(parentCall.Args) != 1 {
			return false
		}
		for _, argument := range parentCall.Args {
			if argument == literal {
				return true
			}
		}
		return false
	}
	return false
}

func hasSingleStringArgument(call *ast.CallExpr, want string) bool {
	if len(call.Args) != 1 {
		return false
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return false
	}
	value, err := strconv.Unquote(literal.Value)
	return err == nil && value == want
}
