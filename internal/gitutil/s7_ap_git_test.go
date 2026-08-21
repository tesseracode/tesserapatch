//go:build (linux && !android) || (darwin && !ios)

package gitutil

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestS7APGitContracts(t *testing.T) {
	t.Run("PIB-472", func(t *testing.T) {
		gitSource := s7GitutilSource(t, "ignore.go")
		rescapSource := s7RepositorySource(t, "internal/rescap/gitgate.go")
		prepareSource := s7RepositorySource(t, "internal/cli/prepare_publish.go")
		if err := validateS7APCentralGitExecutor(gitSource, rescapSource, prepareSource); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("PIB-473", func(t *testing.T) {
		gitSource := s7GitutilSource(t, "ignore.go")
		rescapSource := s7RepositorySource(t, "internal/rescap/gitgate.go")
		prepareSource := s7RepositorySource(t, "internal/cli/prepare_publish.go")
		if err := validateS7APCentralGitExecutor(gitSource, rescapSource, prepareSource); err != nil {
			t.Fatal(err)
		}
		if err := validateS7APCompatibilityCallGraph(gitSource, rescapSource); err != nil {
			t.Fatal(err)
		}
		previous := runGitProcess
		defer func() { runGitProcess = previous }()
		var requests []gitProcessRequest
		runGitProcess = func(request gitProcessRequest) gitProcessResult {
			copyRequest := request
			copyRequest.args = append([]string(nil), request.args...)
			copyRequest.env = append([]string(nil), request.env...)
			requests = append(requests, copyRequest)
			switch strings.Join(request.args, "\x00") {
			case "rev-parse\x00--is-inside-work-tree":
				return gitProcessResult{stdout: "true\n"}
			case "check-ignore\x00-q\x00--no-index\x00--\x00/legacy/absolute/path":
				return gitProcessResult{}
			case "status\x00--short":
				return gitProcessResult{
					stdout: "compat-out\n", stderr: "compat-err\n",
					exitCode: 7, err: errors.New("compat failure"),
				}
			default:
				return gitProcessResult{exitCode: 88, err: errors.New("unexpected compatibility argv")}
			}
		}
		if !CompatibilityIsGitAvailable("/legacy/repo") ||
			!IsGitAvailable("/legacy/repo") {
			t.Fatal("PIB-473 Git-availability compatibility wrappers changed")
		}
		ignored, err := CompatibilityIsPathIgnored("/legacy/repo", "/legacy/absolute/path")
		if err != nil || !ignored {
			t.Fatalf("PIB-473 direct ignored wrapper = %t %v", ignored, err)
		}
		ignored, err = IsPathIgnored("/legacy/repo", "/legacy/absolute/path")
		if err != nil || !ignored {
			t.Fatalf("PIB-473 legacy ignored caller = %t %v", ignored, err)
		}
		stdout, stderr, exit, runErr := RunGitCompatibility(
			"/legacy/repo", "status", "--short",
		)
		wantArgs := [][]string{
			{"rev-parse", "--is-inside-work-tree"},
			{"rev-parse", "--is-inside-work-tree"},
			{"rev-parse", "--is-inside-work-tree"},
			{"check-ignore", "-q", "--no-index", "--", "/legacy/absolute/path"},
			{"rev-parse", "--is-inside-work-tree"},
			{"check-ignore", "-q", "--no-index", "--", "/legacy/absolute/path"},
			{"status", "--short"},
		}
		if len(requests) != len(wantArgs) ||
			stdout != "compat-out\n" || stderr != "compat-err\n" ||
			exit != 7 || runErr == nil {
			t.Fatalf("PIB-473 compatibility behavior = ignored:%t err:%v requests:%+v",
				ignored, err, requests)
		}
		for index, request := range requests {
			if request.env != nil || fmt.Sprint(request.args) != fmt.Sprint(wantArgs[index]) {
				t.Fatalf("PIB-473 compatibility request %d = env:%v argv:%v, want inherited/%v",
					index, request.env, request.args, wantArgs[index])
			}
		}
		wrongEnv := strings.Replace(
			gitSource,
			"repoRoot:      repoRoot,\n\t\targs:          append([]string(nil), args...),",
			"repoRoot:      repoRoot,\n\t\targs:          append([]string(nil), args...),\n\t\tenv:           []string{\"LC_ALL=C\"},",
			1,
		)
		if err := validateS7APCompatibilityCallGraph(wrongEnv, rescapSource); err == nil {
			t.Fatal("PIB-473 same validator accepted a compatibility environment rewrite")
		}
		wrongOutput := strings.Replace(
			gitSource,
			"return result.stdout, result.stderr, result.exitCode, result.err",
			"return \"\", result.stderr, result.exitCode, result.err",
			1,
		)
		if err := validateS7APCompatibilityCallGraph(wrongOutput, rescapSource); err == nil {
			t.Fatal("PIB-473 same validator accepted compatibility output rewriting")
		}
	})

	t.Run("PIB-474", func(t *testing.T) {
		previous := runGitProcess
		defer func() { runGitProcess = previous }()
		for _, name := range s7PrepareGitAuthoritativeScrubExamples(t) {
			t.Setenv(name, "AP-FOREIGN-"+name)
		}
		t.Setenv("GIT_CONFIG_GLOBAL", "/preserved/global")
		t.Setenv("GIT_CONFIG_SYSTEM", "/preserved/system")
		var requests []gitProcessRequest
		runGitProcess = func(request gitProcessRequest) gitProcessResult {
			copyRequest := request
			copyRequest.args = append([]string(nil), request.args...)
			copyRequest.env = append([]string(nil), request.env...)
			requests = append(requests, copyRequest)
			switch strings.Join(request.args, "\x00") {
			case "rev-parse\x00--is-inside-work-tree":
				return gitProcessResult{stdout: "true\n"}
			case "check-ignore\x00-q\x00--no-index\x00--\x00.tpatch/local/intent-prepare/ap":
				return gitProcessResult{}
			case "--literal-pathspecs\x00ls-files\x00--\x00.tpatch/local/":
				return gitProcessResult{}
			case "ls-files\x00--\x00.tpatch":
				return gitProcessResult{}
			default:
				return gitProcessResult{exitCode: 88, err: errors.New("unexpected argv")}
			}
		}
		state, err := DiscoverGitState("/workspace")
		if err != nil || state != GitWorktree {
			t.Fatalf("PIB-474 G1 = %q %v", state, err)
		}
		if ignored, err := IsIgnoredWithState(
			"/workspace", state, ".tpatch/local/intent-prepare/ap",
		); err != nil || !ignored {
			t.Fatalf("PIB-474 G2 = %t %v", ignored, err)
		}
		if tracked, err := AnythingTrackedUnderWithState("/workspace", state); err != nil || tracked {
			t.Fatalf("PIB-474 G3 = %t %v", tracked, err)
		}
		if tracked, err := IsTpatchTrackedWithState("/workspace", state); err != nil || tracked {
			t.Fatalf("PIB-474 G4 = %t %v", tracked, err)
		}
		wantArgv := [][]string{
			{"rev-parse", "--is-inside-work-tree"},
			{"check-ignore", "-q", "--no-index", "--", ".tpatch/local/intent-prepare/ap"},
			{"--literal-pathspecs", "ls-files", "--", ".tpatch/local/"},
			{"ls-files", "--", ".tpatch"},
		}
		if err := validateS7PIB427GitRequests(requests, wantArgv); err != nil {
			t.Fatal(err)
		}
		for index, request := range requests {
			joined := "\n" + strings.Join(request.env, "\n") + "\n"
			if !strings.Contains(joined, "\nGIT_CONFIG_GLOBAL=/preserved/global\n") ||
				!strings.Contains(joined, "\nGIT_CONFIG_SYSTEM=/preserved/system\n") {
				t.Fatalf("PIB-474 G%d lost global/system configuration: %v", index+1, request.env)
			}
		}
	})

	t.Run("PIB-475", func(t *testing.T) {
		main := s7APInitGitFixture(t, "main")
		linked := filepath.Join(t.TempDir(), "linked")
		s7APGit(t, main, "worktree", "add", "-q", "-b", "ap-linked", linked)
		defer func() {
			command := exec.Command("git", "worktree", "remove", "--force", linked)
			command.Dir = main
			_ = command.Run()
		}()
		s7APAssertPrepareGitContext(t, linked)

		childSource := s7APInitGitFixture(t, "child")
		s7APGit(t, main, "-c", "protocol.file.allow=always", "submodule", "add", "-q", childSource, "modules/child")
		submodule := filepath.Join(main, "modules", "child")
		contexts := []string{main, linked, submodule}
		for _, context := range contexts {
			s7APAssertPrepareGitContext(t, context)
		}
		if len(contexts) != 3 {
			t.Fatalf("PIB-475 Git contexts = %d, want main/worktree/submodule", len(contexts))
		}
	})

	t.Run("PIB-476", func(t *testing.T) {
		gitSource := s7GitutilSource(t, "ignore.go")
		cliSource := s7RepositorySource(t, "internal/cli/prepare_publish.go")
		if err := validateS7PIB438GitSources(gitSource, cliSource); err != nil {
			t.Fatal(err)
		}
		wrong := strings.Replace(
			gitSource,
			`"--", ".tpatch/local/"`,
			`"--", "/absolute/.tpatch/local/"`,
			1,
		)
		if err := validateS7PIB438GitSources(wrong, cliSource); err == nil {
			t.Fatal("PIB-476 same validator accepted an absolute Git lane argument")
		}
	})
}

func validateS7APCentralGitExecutor(gitSource, rescapSource, prepareSource string) error {
	gitFile, err := parser.ParseFile(token.NewFileSet(), "ignore.go", gitSource, 0)
	if err != nil {
		return err
	}

	execCalls := 0
	ast.Inspect(gitFile, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, _ := call.Fun.(*ast.SelectorExpr)
		if selector == nil {
			return true
		}
		pkg, _ := selector.X.(*ast.Ident)
		if pkg != nil && pkg.Name == "exec" && selector.Sel.Name == "Command" {
			execCalls++
		}
		return true
	})
	if execCalls != 1 ||
		strings.Count(gitSource, "func defaultRunGitProcess(") != 1 ||
		strings.Count(gitSource, "func RunGitCompatibility(") != 1 {
		return fmt.Errorf("central Git executor shape = exec:%d", execCalls)
	}
	rescapFile, err := parser.ParseFile(token.NewFileSet(), "gitgate.go", rescapSource, 0)
	if err != nil {
		return err
	}
	rescapCompatibilityCalls := 0
	ast.Inspect(rescapFile, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, _ := call.Fun.(*ast.SelectorExpr)
		if selector == nil {
			return true
		}
		pkg, _ := selector.X.(*ast.Ident)
		if pkg == nil {
			return true
		}
		if pkg.Name == "exec" {
			err = errors.New("rescap bypasses the central Git executor")
			return false
		}
		if pkg.Name == "gitutil" && selector.Sel.Name == "RunGitCompatibility" {
			rescapCompatibilityCalls++
		}
		return true
	})
	if err != nil {
		return err
	}
	if rescapCompatibilityCalls != 4 {
		return fmt.Errorf("rescap compatibility routes = %d, want 4", rescapCompatibilityCalls)
	}
	prepareFile, err := parser.ParseFile(token.NewFileSet(), "prepare_publish.go", prepareSource, 0)
	if err != nil {
		return err
	}
	prepareCalls := map[string]int{}
	ast.Inspect(prepareFile, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, _ := call.Fun.(*ast.SelectorExpr)
		if selector == nil {
			return true
		}
		pkg, _ := selector.X.(*ast.Ident)
		if pkg == nil || pkg.Name != "gitutil" {
			return true
		}
		prepareCalls[selector.Sel.Name]++
		return true
	})
	want := map[string]int{
		"DiscoverGitState":              1,
		"IsIgnoredWithState":            1,
		"AnythingTrackedUnderWithState": 1,
		"IsTpatchTrackedWithState":      1,
	}
	if !reflect.DeepEqual(prepareCalls, want) {
		return fmt.Errorf("prepare Git call graph = %v, want %v", prepareCalls, want)
	}
	return nil
}

func validateS7APCompatibilityCallGraph(gitSource, rescapSource string) error {
	parse := func(name, source string) (map[string]*ast.FuncDecl, error) {
		file, err := parser.ParseFile(token.NewFileSet(), name, source, 0)
		if err != nil {
			return nil, err
		}
		functions := map[string]*ast.FuncDecl{}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && function.Body != nil {
				functions[function.Name.Name] = function
			}
		}
		return functions, nil
	}
	gitFunctions, err := parse("ignore.go", gitSource)
	if err != nil {
		return err
	}
	rescapFunctions, err := parse("gitgate.go", rescapSource)
	if err != nil {
		return err
	}
	calls := func(function *ast.FuncDecl, packageName, callee string) int {
		count := 0
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fun := call.Fun.(type) {
			case *ast.Ident:
				if packageName == "" && fun.Name == callee {
					count++
				}
			case *ast.SelectorExpr:
				pkg, _ := fun.X.(*ast.Ident)
				if pkg != nil && pkg.Name == packageName && fun.Sel.Name == callee {
					count++
				}
			}
			return true
		})
		return count
	}
	for caller, callee := range map[string]string{
		"IsGitAvailable":             "CompatibilityIsGitAvailable",
		"IsPathIgnored":              "CompatibilityIsPathIgnored",
		"CompatibilityIsPathIgnored": "CompatibilityIsGitAvailable",
	} {
		if gitFunctions[caller] == nil || calls(gitFunctions[caller], "", callee) != 1 {
			return fmt.Errorf("compatibility caller %s no longer delegates exactly once to %s",
				caller, callee)
		}
	}
	for _, caller := range []string{
		"IsIgnored", "IsTracked", "anythingTrackedUnderCompatibility", "RunGit",
	} {
		if rescapFunctions[caller] == nil ||
			calls(rescapFunctions[caller], "gitutil", "RunGitCompatibility") != 1 {
			return fmt.Errorf("rescap caller %s no longer uses one compatibility wrapper", caller)
		}
	}
	wrapper := gitFunctions["RunGitCompatibility"]
	if wrapper == nil {
		return errors.New("RunGitCompatibility is missing")
	}
	wrapperText := s7APGitNodeText(gitSource, wrapper.Body)
	if strings.Contains(wrapperText, "env:") ||
		!strings.Contains(wrapperText, "args:          append([]string(nil), args...)") ||
		!strings.Contains(wrapperText, "captureStdout: true") ||
		!strings.Contains(wrapperText, "captureStderr: true") ||
		!strings.Contains(wrapperText,
			"return result.stdout, result.stderr, result.exitCode, result.err") {
		return errors.New("RunGitCompatibility no longer preserves inherited env/argv/output")
	}
	return nil
}

func s7APGitNodeText(source string, node ast.Node) string {
	if node == nil {
		return ""
	}
	start := int(node.Pos()) - 1
	end := int(node.End()) - 1
	if start < 0 || end < start || end > len(source) {
		return ""
	}
	return source[start:end]
}

func s7APInitGitFixture(t *testing.T, name string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Join(root, ".tpatch"), 0o755); err != nil {
		t.Fatal(err)
	}
	s7APGit(t, root, "init", "-q")
	s7APGit(t, root, "config", "user.email", "ap@example.invalid")
	s7APGit(t, root, "config", "user.name", "AP Test")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".tpatch/local/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".tpatch", "config.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s7APGit(t, root, "add", ".gitignore", ".tpatch/config.json")
	s7APGit(t, root, "commit", "-qm", "fixture")
	return root
}

func s7APAssertPrepareGitContext(t *testing.T, root string) {
	t.Helper()
	state, err := DiscoverGitState(root)
	if err != nil || state != GitWorktree {
		t.Fatalf("Git context %s G1 = %q %v", root, state, err)
	}
	if ignored, err := IsIgnoredWithState(
		root, state, ".tpatch/local/intent-prepare/ap",
	); err != nil || !ignored {
		t.Fatalf("Git context %s G2 = %t %v", root, ignored, err)
	}
	if tracked, err := AnythingTrackedUnderWithState(root, state); err != nil || tracked {
		t.Fatalf("Git context %s G3 = %t %v", root, tracked, err)
	}
	if tracked, err := IsTpatchTrackedWithState(root, state); err != nil || !tracked {
		t.Fatalf("Git context %s G4 = %t %v", root, tracked, err)
	}
}

func s7APGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, root, err, output)
	}
}
