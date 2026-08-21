//go:build (linux && !android) || (darwin && !ios)

package workflow

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestS7APDoctorHolderHelper(t *testing.T) {
	if os.Getenv("TPATCH_S7_AP_DOCTOR_HOLDER") != "1" {
		return
	}
	authority, err := intentlock.Acquire(os.Getenv("TPATCH_S7_AP_DOCTOR_ROOT"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "acquire holder: %v\n", err)
		os.Exit(91)
	}
	fmt.Fprintln(os.Stdout, "holder-ready")
	if !bufio.NewScanner(os.Stdin).Scan() {
		_ = authority.Release()
		fmt.Fprintln(os.Stderr, "holder stdin closed before release")
		os.Exit(92)
	}
	if err := authority.Release(); err != nil {
		fmt.Fprintf(os.Stderr, "release holder: %v\n", err)
		os.Exit(93)
	}
	fmt.Fprintln(os.Stdout, "holder-released")
	os.Exit(0)
}

func TestS7APDoctorD9Contracts(t *testing.T) {
	t.Run("PIB-470", func(t *testing.T) {
		root := t.TempDir()
		repository, err := store.Init(root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repository.AddFeature(store.AddFeatureInput{
			Title: "AP D9 Live Holder", Slug: "ap-d9-live-holder",
		}); err != nil {
			t.Fatal(err)
		}
		doctorD9Write(t, root, ".tpatch/local/intent-prepare/ap-d9-live-holder/journal.json", "{}\n")
		holder := s7APStartDoctorHolder(t, root)
		before := doctorD9SnapshotTree(t, root)
		var spy *doctorD9BoundarySpy
		previous := newDoctorD9Boundary
		newDoctorD9Boundary = func(path string) doctorD9Boundary {
			spy = &doctorD9BoundarySpy{delegate: &doctorD9OSBoundary{root: path}}
			return spy
		}
		report, err := RunDoctor(repository, DoctorOptions{Checks: []string{"D9"}})
		newDoctorD9Boundary = previous
		if err != nil {
			t.Fatal(err)
		}
		if spy == nil || spy.readCalls == 0 {
			t.Fatal("PIB-470 D9 did not inspect persistent evidence")
		}
		doctorD9AssertNoForbiddenCapabilities(t, spy)
		doctorD9AssertTreeEqual(t, before, doctorD9SnapshotTree(t, root))
		var output bytes.Buffer
		if err := WriteDoctorJSON(&output, report); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(output.String(), root) {
			t.Fatalf("PIB-470 D9 leaked the workspace path: %s", output.String())
		}
		holder.release(t)
	})

	t.Run("PIB-471", func(t *testing.T) {
		root := t.TempDir()
		repository, err := store.Init(root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repository.AddFeature(store.AddFeatureInput{
			Title: "AP Holder Unknown", Slug: "ap-holder-unknown",
		}); err != nil {
			t.Fatal(err)
		}
		doctorD9Write(t, root, ".tpatch/local/intent-prepare/ap-holder-unknown/journal.json", "{}\n")
		holder := s7APStartDoctorHolder(t, root)
		report, err := RunDoctor(repository, DoctorOptions{Checks: []string{"D9"}})
		if err != nil {
			t.Fatal(err)
		}
		var human, structured bytes.Buffer
		WriteDoctorHuman(&human, report)
		if err := WriteDoctorJSON(&structured, report); err != nil {
			t.Fatal(err)
		}
		surfaces := s7APD9OwnedTruthSurfaces(t)
		surfaces["runtime/doctor-human"] = human.String()
		surfaces["runtime/doctor-json"] = structured.String()
		if err := validateS7APD9HolderClaims(surfaces); err != nil {
			t.Fatal(err)
		}
		wrong := make(map[string]string, len(surfaces))
		for name, body := range surfaces {
			wrong[name] = body
		}
		wrong["runtime/doctor-human"] += "\nThere is no active workspace mutator.\n"
		if err := validateS7APD9HolderClaims(wrong); err == nil {
			t.Fatal("PIB-471 same validator accepted an active-mutator absence claim")
		}
		concatenated := make(map[string]string, len(surfaces))
		for name, body := range surfaces {
			concatenated[name] = body
		}
		concatenated["internal/workflow/doctor_d9.go"] +=
			"\nconst s7APFalseHolderClaim = \"There is no active \" + \"workspace mutator.\"\n"
		if err := validateS7APD9HolderClaims(concatenated); err == nil {
			t.Fatal("PIB-471 same validator accepted a concatenated active-mutator absence claim")
		}
		holder.release(t)
	})

	t.Run("PIB-476", func(t *testing.T) {
		root := t.TempDir()
		repository, err := store.Init(root)
		if err != nil {
			t.Fatal(err)
		}
		leaked := filepath.Join(root, "private-panic-detail")
		previous := newDoctorD9Boundary
		newDoctorD9Boundary = func(path string) doctorD9Boundary {
			return &s7APPanicD9Boundary{
				doctorD9Boundary: &doctorD9OSBoundary{root: path},
				message:          leaked,
			}
		}
		report, err := RunDoctor(repository, DoctorOptions{Checks: []string{"D9"}})
		newDoctorD9Boundary = previous
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Findings) != 1 ||
			report.Findings[0].Code != "check-error" ||
			report.Findings[0].Message != "check panicked" {
			t.Fatalf("PIB-476 panic report = %+v", report)
		}
		raw, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), root) || strings.Contains(string(raw), leaked) {
			t.Fatalf("PIB-476 panic report leaked absolute detail: %s", raw)
		}
	})
}

type s7APPanicD9Boundary struct {
	doctorD9Boundary
	message string
}

func (boundary *s7APPanicD9Boundary) Lstat(string) (fs.FileInfo, error) {
	panic(boundary.message)
}

type s7APDoctorHolder struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	output  []string
	stderr  *bytes.Buffer
	cancel  context.CancelFunc
	done    bool
}

func s7APStartDoctorHolder(t *testing.T, root string) *s7APDoctorHolder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestS7APDoctorHolderHelper$")
	command.Env = append(os.Environ(),
		"TPATCH_S7_AP_DOCTOR_HOLDER=1",
		"TPATCH_S7_AP_DOCTOR_ROOT="+root,
	)
	stdin, err := command.StdinPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	stderr := &bytes.Buffer{}
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	holder := &s7APDoctorHolder{
		command: command, stdin: stdin, scanner: bufio.NewScanner(stdout),
		stderr: stderr, cancel: cancel,
	}
	t.Cleanup(func() {
		if holder.done {
			return
		}
		_, _ = fmt.Fprintln(holder.stdin, "release")
		_ = holder.stdin.Close()
		_ = holder.command.Process.Kill()
		_ = holder.command.Wait()
		holder.cancel()
	})
	if !holder.scanner.Scan() || holder.scanner.Text() != "holder-ready" {
		t.Fatalf("child holder did not become ready: line=%q err=%v stderr=%q",
			holder.scanner.Text(), holder.scanner.Err(), stderr.String())
	}
	holder.output = append(holder.output, "holder-ready")
	return holder
}

func (holder *s7APDoctorHolder) release(t *testing.T) {
	t.Helper()
	if holder.done {
		t.Fatal("child holder released twice")
	}
	if _, err := fmt.Fprintln(holder.stdin, "release"); err != nil {
		t.Fatal(err)
	}
	if err := holder.stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if !holder.scanner.Scan() {
		t.Fatalf("child holder emitted no release line: err=%v stderr=%q",
			holder.scanner.Err(), holder.stderr.String())
	}
	holder.output = append(holder.output, holder.scanner.Text())
	if holder.scanner.Scan() {
		holder.output = append(holder.output, holder.scanner.Text())
	}
	if err := holder.command.Wait(); err != nil {
		t.Fatalf("child holder exit changed: %v stderr=%q output=%v",
			err, holder.stderr.String(), holder.output)
	}
	holder.cancel()
	holder.done = true
	if fmt.Sprint(holder.output) != fmt.Sprint([]string{"holder-ready", "holder-released"}) ||
		holder.stderr.Len() != 0 {
		t.Fatalf("child holder output changed: stdout=%v stderr=%q",
			holder.output, holder.stderr.String())
	}
}

func validateS7APD9HolderClaims(surfaces map[string]string) error {
	actual, err := s7APHolderClauseInventory(surfaces)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, s7APAcceptedHolderClauses) {
		return fmt.Errorf("owned holder clause inventory drift:\ngot  %#v\nwant %#v",
			actual, s7APAcceptedHolderClauses)
	}
	return nil
}

var s7APAcceptedHolderClauses = map[string]map[string]string{
	"docs/adrs/ADR-035-intent-bundle-publication-and-history.md": {
		"and retry, and nothing else, because the evidence may be a live holder's undo":   "authority-held+wait-retry",
		"authority — as held, with the holder unknowable, and with wait-and-retry as the": "authority-held+wait-retry",
		"be established, no lock was taken and no holder is implied — the exact meaning":  "no-absence-claim",
		"because no holder is implied, d13's pre-abandon gate carries it in the same row": "no-absence-claim",
		"journal and deleting it under that holder would destroy a running transaction's": "authority-held",
		"probe: no diagnostic command can identify a lock holder or prove that none":      "no-absence-claim",
		"that single owner is also the only holder of d16's validation-ordering":          "authority-held",
	},
	"docs/prds/PRD-prepare-intent-bundle.md": {
		"because a live holder may be mid-transaction; its route is wait and retry":   "authority-held+wait-retry",
		"because no holder is implied, this refusal is safe to accompany with §6.6's": "no-absence-claim",
		"for transaction-in-progress: the workspace mutation authority is held, the holder's identity is unknowable, and the safe action is to wait and retry — no stronger claim is made anywhere (§12.5), and no manual removal is offered even in abandon mode with evidence present, because the evidence may be a live holder's undo journal (§6.6's gate table row 7, pib-512": "authority-held+identity-unknowable+wait-retry",
		"holder's identity is unknowable, and the safe action is to wait and retry":      "identity-unknowable+wait-retry",
		"not be established, no lock was taken, and no holder is implied — which is":     "no-absence-claim",
		"probe: no diagnostic command can identify a lock holder or prove that no":       "no-absence-claim",
		"text says exactly three things — the workspace mutation authority is held, the": "authority-held",
		"workspace mutation authority held, with no holder identity available to":        "authority-held+identity-unknowable",
	},
	"internal/cli/doctor.go": {
		"d9 is evidence-only and never repairs findings, opens or probes workspace mutation authority, identifies an authority holder, or proves that no holder exists": "no-absence-claim",
	},
	"internal/cli/feature_intent_archive.go": {
		"the holder's identity is unknowable":                                                       "identity-unknowable",
		"the workspace mutation authority could not be established":                                 "no-absence-claim",
		"the workspace mutation authority is held by another mutating prepare or archive operation": "authority-held",
		"this platform cannot establish the workspace mutation authority":                           "no-absence-claim",
		"wait for the current holder to finish, then retry":                                         "wait-retry",
	},
	"internal/cli/prepare_publish.go": {
		"the holder's identity is unknowable":                                                       "identity-unknowable",
		"the workspace mutation authority is held by another mutating prepare or archive operation": "authority-held",
	},
	"internal/workflow/doctor_d9.go": {
		"durable prepare transaction control evidence is present; d9 does not inspect or make any claim about a live authority holder": "no-absence-claim",
	},
	"runtime/doctor-human": {
		"durable prepare transaction control evidence is present; d9 does not inspect or make any claim about a live authority holder": "no-absence-claim",
	},
	"runtime/doctor-json": {
		"durable prepare transaction control evidence is present; d9 does not inspect or make any claim about a live authority holder": "no-absence-claim",
	},
}

func s7APHolderClauseInventory(surfaces map[string]string) (map[string]map[string]string, error) {
	result := map[string]map[string]string{}
	for name, body := range surfaces {
		texts, err := s7APHolderSurfaceTexts(name, body)
		if err != nil {
			return nil, err
		}
		for _, text := range texts {
			text = strings.NewReplacer("`", "", "*", "", "_", " ").Replace(text)
			for _, rawClause := range regexp.MustCompile(
				`[!?](?:\s+|$)|\.(?:\s+|$)|\n+|\s+\|\s+`,
			).Split(text, -1) {
				clause := strings.Join(strings.Fields(strings.ToLower(rawClause)), " ")
				clause = strings.Trim(clause, " \t\r\n\"',:;()[]{}-")
				if !s7APHolderRelatedClause(clause) {
					continue
				}
				if result[name] == nil {
					result[name] = map[string]string{}
				}
				result[name][clause] = s7APHolderClauseMeaning(clause)
			}
		}
	}
	return result, nil
}

func s7APHolderSurfaceTexts(name, body string) ([]string, error) {
	if name == "runtime/doctor-json" {
		var decoded any
		if err := json.Unmarshal([]byte(body), &decoded); err != nil {
			return nil, err
		}
		var values []string
		var walk func(any)
		walk = func(value any) {
			switch typed := value.(type) {
			case map[string]any:
				for _, child := range typed {
					walk(child)
				}
			case []any:
				for _, child := range typed {
					walk(child)
				}
			case string:
				values = append(values, typed)
			}
		}
		walk(decoded)
		return values, nil
	}
	if strings.HasSuffix(name, ".go") {
		file, err := parser.ParseFile(token.NewFileSet(), name, body, 0)
		if err != nil {
			return nil, err
		}
		return s7APMaximalCompileTimeStrings(file), nil
	}
	return []string{body}, nil
}

type s7APGoStaticStrings struct {
	bindings map[string]string
	helpers  map[string]*ast.FuncDecl
}

func s7APMaximalCompileTimeStrings(file *ast.File) []string {
	resolver := &s7APGoStaticStrings{
		bindings: map[string]string{},
		helpers:  map[string]*ast.FuncDecl{},
	}
	for _, declaration := range file.Decls {
		function, _ := declaration.(*ast.FuncDecl)
		if function != nil {
			resolver.helpers[function.Name.Name] = function
		}
	}
	for pass := 0; pass < 12; pass++ {
		changed := false
		ast.Inspect(file, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.ValueSpec:
				for index, name := range value.Names {
					if index >= len(value.Values) {
						continue
					}
					if resolved, ok := resolver.resolve(value.Values[index], nil, nil); ok &&
						resolver.bindings[name.Name] != resolved {
						resolver.bindings[name.Name] = resolved
						changed = true
					}
				}
			case *ast.AssignStmt:
				for index, left := range value.Lhs {
					if index >= len(value.Rhs) {
						continue
					}
					identifier, _ := left.(*ast.Ident)
					if identifier == nil {
						continue
					}
					resolved, ok := resolver.resolve(value.Rhs[index], nil, nil)
					if !ok {
						continue
					}
					if value.Tok == token.ADD_ASSIGN {
						resolved = resolver.bindings[identifier.Name] + resolved
					}
					if resolver.bindings[identifier.Name] != resolved {
						resolver.bindings[identifier.Name] = resolved
						changed = true
					}
				}
			}
			return true
		})
		if !changed {
			break
		}
	}
	var values []string
	var stack []ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		expression, isExpression := node.(ast.Expr)
		if isExpression {
			resolved, ok := resolver.resolve(expression, nil, nil)
			parentResolved := false
			if len(stack) != 0 {
				if parent, parentOK := stack[len(stack)-1].(ast.Expr); parentOK {
					_, parentResolved = resolver.resolve(parent, nil, nil)
				}
			}
			if ok && !parentResolved {
				values = append(values, resolved)
			}
		}
		stack = append(stack, node)
		return true
	})
	return values
}

func (resolver *s7APGoStaticStrings) resolve(
	expression ast.Expr,
	locals map[string]string,
	visiting map[string]bool,
) (string, bool) {
	if visiting == nil {
		visiting = map[string]bool{}
	}
	switch value := expression.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return "", false
		}
		text, err := strconv.Unquote(value.Value)
		return text, err == nil
	case *ast.Ident:
		if locals != nil {
			if text, ok := locals[value.Name]; ok {
				return text, true
			}
		}
		text, ok := resolver.bindings[value.Name]
		return text, ok
	case *ast.ParenExpr:
		return resolver.resolve(value.X, locals, visiting)
	case *ast.BinaryExpr:
		if value.Op != token.ADD {
			return "", false
		}
		left, leftOK := resolver.resolve(value.X, locals, visiting)
		right, rightOK := resolver.resolve(value.Y, locals, visiting)
		return left + right, leftOK && rightOK
	case *ast.CallExpr:
		identifier, _ := value.Fun.(*ast.Ident)
		if identifier == nil || visiting[identifier.Name] {
			return "", false
		}
		function := resolver.helpers[identifier.Name]
		if function == nil || function.Body == nil {
			return "", false
		}
		callLocals := map[string]string{}
		argument := 0
		if function.Type.Params != nil {
			for _, field := range function.Type.Params.List {
				for _, name := range field.Names {
					if argument >= len(value.Args) {
						return "", false
					}
					resolved, ok := resolver.resolve(value.Args[argument], locals, visiting)
					if !ok {
						return "", false
					}
					callLocals[name.Name] = resolved
					argument++
				}
			}
		}
		if argument != len(value.Args) {
			return "", false
		}
		visiting[identifier.Name] = true
		defer delete(visiting, identifier.Name)
		var result string
		found := false
		consistent := true
		ast.Inspect(function.Body, func(node ast.Node) bool {
			statement, _ := node.(*ast.ReturnStmt)
			if statement == nil {
				return consistent
			}
			if len(statement.Results) != 1 {
				consistent = false
				return false
			}
			resolved, ok := resolver.resolve(statement.Results[0], callLocals, visiting)
			if !ok || (found && resolved != result) {
				consistent = false
				return false
			}
			result = resolved
			found = true
			return true
		})
		return result, found && consistent
	default:
		return "", false
	}
}

func s7APHolderRelatedClause(clause string) bool {
	holderStatement := regexp.MustCompile(
		`\b(?:holder's|current holder|live holder|lock holder|authority holder|` +
			`holder identity|holder's identity|holder is|holder exists|holder unknowable|` +
			`under that holder|only holder|no holder)\b`,
	).MatchString(clause)
	authorityStatement := strings.Contains(clause, "workspace mutation authority") &&
		(strings.Contains(clause, "held") ||
			strings.Contains(clause, "establish") ||
			strings.Contains(clause, "platform"))
	return len(clause) >= 20 &&
		(holderStatement ||
			strings.Contains(clause, "active workspace mutator") ||
			authorityStatement ||
			(strings.Contains(clause, "prepare mutation") &&
				(strings.Contains(clause, "held") || strings.Contains(clause, "running"))))
}

func s7APHolderClauseMeaning(clause string) string {
	var meanings []string
	add := func(meaning string) {
		for _, existing := range meanings {
			if existing == meaning {
				return
			}
		}
		meanings = append(meanings, meaning)
	}
	if strings.Contains(clause, "authority") &&
		(strings.Contains(clause, "held") || strings.Contains(clause, "holds")) {
		add("authority-held")
	}
	if strings.Contains(clause, "live holder") ||
		strings.Contains(clause, "under that holder") ||
		strings.Contains(clause, "only holder") {
		add("authority-held")
	}
	if strings.Contains(clause, "identity") &&
		(strings.Contains(clause, "unknowable") ||
			strings.Contains(clause, "cannot identify") ||
			strings.Contains(clause, "no holder identity available")) {
		add("identity-unknowable")
	}
	if strings.Contains(clause, "wait") || strings.Contains(clause, "retry") {
		add("wait-retry")
	}
	if strings.Contains(clause, "no holder is implied") ||
		strings.Contains(clause, "prove that no") ||
		strings.Contains(clause, "proves that no") ||
		(strings.Contains(clause, "authority") &&
			(strings.Contains(clause, "could not be established") ||
				strings.Contains(clause, "cannot establish"))) ||
		strings.Contains(clause, "does not inspect or make any claim") {
		add("no-absence-claim")
	}
	if len(meanings) == 0 {
		add("holder-context")
	}
	return strings.Join(meanings, "+")
}

func s7APD9OwnedTruthSurfaces(t *testing.T) map[string]string {
	t.Helper()
	root := doctorD9S5RepoRoot(t)
	surfaces := map[string]string{}
	for _, directory := range []string{"internal/cli", "internal/workflow", "assets"} {
		err := filepath.WalkDir(filepath.Join(root, directory), func(
			path string,
			entry os.DirEntry,
			err error,
		) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			extension := filepath.Ext(path)
			if extension != ".go" && extension != ".md" && extension != ".mdc" &&
				entry.Name() != "windsurfrules" {
				return nil
			}
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			surfaces[filepath.ToSlash(rel)] = string(raw)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range []string{"SPEC.md", "docs/feature-layout.md"} {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		surfaces[rel] = string(raw)
	}
	prdRaw, err := os.ReadFile(filepath.Join(root, "docs/prds/PRD-prepare-intent-bundle.md"))
	if err != nil {
		t.Fatal(err)
	}
	prd := string(prdRaw)
	surfaces["docs/prds/PRD-prepare-intent-bundle.md"] =
		s7APD9Section(t, prd, "### 7.4 ", "### 7.5 ") + "\n" +
			s7APD9Section(t, prd, "### 10.4 ", "### 10.5 ") + "\n" +
			s7APD9Section(t, prd, "### 12.5 ", "### 12.6 ")
	adrRaw, err := os.ReadFile(filepath.Join(root, "docs/adrs/ADR-035-intent-bundle-publication-and-history.md"))
	if err != nil {
		t.Fatal(err)
	}
	adr := string(adrRaw)
	surfaces["docs/adrs/ADR-035-intent-bundle-publication-and-history.md"] =
		s7APD9Section(t, adr, "### D4 ", "### D5 ") + "\n" +
			s7APD9Section(t, adr, "### D13 ", "### D14 ")
	return surfaces
}

func s7APD9Section(t *testing.T, document, start, end string) string {
	t.Helper()
	begin := strings.Index(document, start)
	if begin < 0 {
		t.Fatalf("missing holder-truth section %q", start)
	}
	tail := begin + len(start)
	finish := strings.Index(document[tail:], end)
	if finish < 0 {
		t.Fatalf("missing holder-truth section terminator %q", end)
	}
	return document[begin : tail+finish]
}
