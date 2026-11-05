package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Gate-time reduction (2026-11-05): `make wave-close-check
// WAVE_CLOSE_TEST_LANES=2` runs the serial shard commands in two concurrent
// lanes. These guards keep that mode opt-in, derived from the serial script,
// and inside the measured and audited partition.

const (
	s7WaveCloseLaneScriptPath = "scripts/wave-close-test-lanes.sh"
	s7WaveCloseLaneCall       = "sh " + s7WaveCloseLaneScriptPath + " $(WAVE_CLOSE_TEST_LANES)"
	// s7WaveCloseCheckShardBlock is check 8 of wave-close-check: serial by
	// default, the lane runner only when WAVE_CLOSE_TEST_LANES is non-empty,
	// and the runner's exit status decides the check either way.
	s7WaveCloseCheckShardBlock = "\tshards=\"sh scripts/wave-close-test-shards.sh\"; \\\n" +
		"\tif [ -n \"$(WAVE_CLOSE_TEST_LANES)\" ]; then \\\n" +
		"\t\tshards=\"" + s7WaveCloseLaneCall + "\"; \\\n" +
		"\t\techo \"  (opt-in: WAVE_CLOSE_TEST_LANES=$(WAVE_CLOSE_TEST_LANES))\"; \\\n" +
		"\tfi; \\\n" +
		"\tif out=$$($$shards 2>&1); then \\\n" +
		"\t\techo \"  OK\"; \\\n"
	// s7WaveCloseLaneRefusal is the runner's refusal loop. The concurrency
	// audit assumed every one of these is unset.
	s7WaveCloseLaneRefusal = "for name in "
	// s7WaveCloseLaneLaunch starts both lanes in the background, each holding
	// the cancellation marker on descriptor 9, and collects each lane's exit
	// status.
	s7WaveCloseLaneLaunch = "run_lane A \"$lane_a\" >\"$tmp/a.out\" 2>&1 9<\"$marker\" &\n" +
		"pid_a=$!\n" +
		"run_lane B \"$lane_b\" >\"$tmp/b.out\" 2>&1 9<\"$marker\" &\n" +
		"pid_b=$!\n" +
		"status_a=0\n" +
		"wait \"$pid_a\" || status_a=$?\n" +
		"status_b=0\n" +
		"wait \"$pid_b\" || status_b=$?\n"
	// s7WaveCloseSoloLaunch runs the solo tail only after both lanes passed.
	s7WaveCloseSoloLaunch = "status_s=\n" +
		"if [ \"$status_a\" -eq 0 ] && [ \"$status_b\" -eq 0 ]; then\n" +
		"\tstatus_s=0\n" +
		"\trun_lane S \"$solo\" >\"$tmp/s.out\" 2>&1 9<\"$marker\" &\n" +
		"\tpid_s=$!\n" +
		"\twait \"$pid_s\" || status_s=$?\n" +
		"fi\n"
	// s7WaveCloseLaneExit is the runner's last line: it passes only if both
	// lanes and the solo tail passed.
	s7WaveCloseLaneExit = "\n[ \"$status_a\" -eq 0 ] && [ \"$status_b\" -eq 0 ] && [ \"$status_s\" -eq 0 ]\n"
	// s7WaveCloseLaneTraps runs cleanup once on any exit or cancellation,
	// ignoring further signals while it runs.
	s7WaveCloseLaneTraps = "trap 'trap \"\" INT TERM HUP; cleanup' EXIT\n" +
		"trap 'on_signal 130' INT\n" +
		"trap 'on_signal 143' TERM\n" +
		"trap 'on_signal 129' HUP\n"
)

// s7WaveCloseSoloTests are the observer tests that run in the solo tail. In
// the reference serial run (old Mac, b8a5f37) each one's child `go test`
// used more than half of its fixed wall-clock budget, and in two-lane
// overlap the first of them timed out (GT-D18).
var s7WaveCloseSoloTests = []string{
	"TestS7ObservedAMThroughAORegistrationAuthority",
	"TestS7ObservedAPRegistrationAuthority",
	"TestS7ObservedAQRegistrationAuthority",
}

// s7WaveCloseSoloRecord is the fake go record fragment of a solo test's
// shard.
func s7WaveCloseSoloRecord(test string) string { return "[^" + test + "$]" }

// s7WaveCloseHeavyShards are the shards (1-based, serial-script order) whose
// peak RSS reached 2.5 GiB or more on the reference host (old-Mac serial gate
// at b8a5f37). Keeping them in one lane means no two of them run at once.
var s7WaveCloseHeavyShards = []int{9, 23, 25, 28, 33}

// s7WaveCloseAuditedUnsetVariables are the non-recording variables the
// concurrency audit assumed unset: live provider traffic and Git directory
// redirection.
var s7WaveCloseAuditedUnsetVariables = []string{
	"TPATCH_LIVE_PROVIDER", "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE",
}

func validateS7WaveCloseLaneOptIn(makefile string) error {
	if strings.Count(makefile, "\nWAVE_CLOSE_TEST_LANES ?=\n") != 1 {
		return errors.New("wave-close lanes are not opt-in: WAVE_CLOSE_TEST_LANES must default to empty")
	}
	uses := 0
	for _, line := range strings.Split(makefile, "\n") {
		if !strings.HasPrefix(line, "#") {
			uses += strings.Count(line, "WAVE_CLOSE_TEST_LANES")
		}
	}
	// One declaration plus the four references in check 8.
	if uses != 5 {
		return fmt.Errorf("WAVE_CLOSE_TEST_LANES is referenced %d times outside comments, want 5", uses)
	}
	if strings.Count(makefile, "sh "+s7WaveCloseLaneScriptPath) != 1 {
		return errors.New("wave-close lane runner must be invoked exactly once, from check 8")
	}
	return nil
}

func TestS7WaveCloseLaneParity(t *testing.T) {
	root := avpRepoRoot(t)
	read := func(path string) string {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	lanes := read(s7WaveCloseLaneScriptPath)
	shards := read("scripts/wave-close-test-shards.sh")
	switches, err := s7WaveCloseRecordingSwitches(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(switches) < 3 {
		t.Fatalf("recording-switch scan found %q; the scan is broken", switches)
	}
	if err := validateS7WaveCloseLaneScript(lanes, shards, switches); err != nil {
		t.Fatal(err)
	}
	move := func(body, from, to string) string {
		return strings.Replace(body, from, to, 1)
	}
	for _, fixture := range []struct {
		name     string
		mutate   func(string) string
		shards   func(string) string
		switches []string
	}{
		{name: "serial-source-replaced", mutate: func(body string) string {
			return move(body, "shards=scripts/wave-close-test-shards.sh", "shards=scripts/other-shards.sh")
		}},
		{name: "command-hardcoded", mutate: func(body string) string {
			return body + "go test -p=1 ./... -count=1 -timeout 40m\n"
		}},
		{name: "shard-dropped", mutate: func(body string) string {
			return move(body, " 15 16'\n", " 15'\n")
		}},
		{name: "shard-duplicated", mutate: func(body string) string {
			return move(body, "lane_a='1 ", "lane_a='1 16 ")
		}},
		{name: "heavy-shards-split", mutate: func(body string) string {
			return move(move(body, " 28 30 ", " 30 "), " 22 26'\n", " 22 26 28'\n")
		}},
		{name: "lane-b-idles-at-barrier", mutate: func(body string) string {
			return move(move(body, "lane_b='18 ", "lane_b='5 18 "), " 29 5 7 ", " 29 7 ")
		}},
		{name: "shard-1-not-first", mutate: func(body string) string {
			return move(body, "lane_a='1 2 ", "lane_a='2 1 ")
		}},
		{name: "shard-1-in-lane-b", mutate: func(body string) string {
			return move(move(body, "lane_a='1 ", "lane_a='"), "lane_b='18 ", "lane_b='1 18 ")
		}},
		{name: "recording-switch-not-refused", mutate: func(body string) string {
			return move(body, "TPATCH_UPDATE_ROC_S0_GOLDENS ", "")
		}},
		{name: "git-redirection-not-refused", mutate: func(body string) string {
			return move(body, " GIT_INDEX_FILE;", ";")
		}},
		{name: "new-recording-switch", switches: append(slices.Clone(switches),
			strings.Join([]string{"TPATCH", "RECORD", "NEW_GOLDENS"}, "_"))},
		{name: "lanes-serialized", mutate: func(body string) string {
			return move(body, "2>&1 9<\"$marker\" &\npid_a=$!\n", "2>&1 9<\"$marker\"\npid_a=$!\n")
		}},
		{name: "lane-status-masked", mutate: func(body string) string {
			return move(body, s7WaveCloseLaneExit, "\ntrue\n")
		}},
		{name: "solo-status-masked", mutate: func(body string) string {
			return move(body, s7WaveCloseLaneExit, "\n[ \"$status_a\" -eq 0 ] && [ \"$status_b\" -eq 0 ]\n")
		}},
		{name: "marker-not-inherited", mutate: func(body string) string {
			return move(body, "\"$tmp/b.out\" 2>&1 9<\"$marker\" &", "\"$tmp/b.out\" 2>&1 &")
		}},
		{name: "solo-shard-in-lane", mutate: func(body string) string {
			return move(move(body, "solo='31 32 33'", "solo='32 33'"), " 30 34 ", " 30 31 34 ")
		}},
		{name: "solo-holds-lane-shard", mutate: func(body string) string {
			return move(move(body, " 30 34 ", " 30 "), "solo='31 32 33'", "solo='31 32 33 34'")
		}},
		{name: "solo-without-lanes-passing", mutate: func(body string) string {
			return move(body, "if [ \"$status_a\" -eq 0 ] && [ \"$status_b\" -eq 0 ]; then\n", "if true; then\n")
		}},
		{name: "solo-before-lanes", mutate: func(body string) string {
			launch := "\n" + s7WaveCloseLaneLaunch
			solo := "\n" + s7WaveCloseSoloLaunch
			return move(move(body, solo, "\n"), launch, solo+launch)
		}},
		{name: "solo-test-renamed", shards: func(body string) string {
			return move(body, "'^TestS7ObservedAPRegistrationAuthority$'", "'^TestS7ObservedAP.*$'")
		}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			mutated, shardScript := lanes, shards
			if fixture.mutate != nil {
				mutated = fixture.mutate(lanes)
			}
			if fixture.shards != nil {
				shardScript = fixture.shards(shards)
			}
			wantSwitches := switches
			if fixture.switches != nil {
				wantSwitches = fixture.switches
			}
			if mutated == lanes && shardScript == shards && fixture.switches == nil {
				t.Fatal("mutation changed nothing")
			}
			if err := validateS7WaveCloseLaneScript(mutated, shardScript, wantSwitches); err == nil {
				t.Fatal("wave-close lane parity accepted wrong input")
			}
		})
	}
}

// s7WaveCloseSwitchName matches a TPATCH_RECORD_* / TPATCH_UPDATE_* name and
// captures the part after the prefix.
var s7WaveCloseSwitchName = regexp.MustCompile(`TPATCH_(?:RECORD|UPDATE)_([A-Z0-9_]*)`)

// s7WaveCloseRecordingSwitches lists every TPATCH_RECORD_* / TPATCH_UPDATE_*
// name spelled by a constant string in a Go source under cmd/, internal/ or
// tests/. Such switches make tests write real checkout files, which is unsafe
// while another shard reads them. Constant strings are folded first, so raw
// strings, concatenations and named constants are all seen. A name that is
// only partly constant cannot be listed, so it is an error.
func s7WaveCloseRecordingSwitches(root string) ([]string, error) {
	found := map[string]bool{}
	for _, top := range []string{"cmd", "internal", "tests"} {
		var paths []string
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if err := scanS7WaveCloseSwitches(paths, found); err != nil {
			return nil, err
		}
	}
	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

// s7WaveCloseFoldLimit bounds the values one constant expression may fold to.
const s7WaveCloseFoldLimit = 64

// s7WaveCloseSwitchScan holds one package's constants. An identifier is
// resolved lexically, as the compiler would: the parser binds a reference to
// a declaration in the same file, including a function-local constant, a
// parameter or a variable that shadows a constant. A reference it leaves
// unresolved names a declaration in another file of the package, so it folds
// to the package-level constants of that name. A package-level constant
// declared more than once, for example in files with different build tags,
// folds to each declared value. A test package (`a_test`) is a package of
// its own, even in the same directory.
type s7WaveCloseSwitchScan struct {
	fset *token.FileSet
	// consts holds every declaration of each package-level constant.
	consts   map[string][]ast.Expr
	topLevel map[*ast.ValueSpec]bool
	// values holds each constant spec's expressions. Within a parenthesized
	// const declaration an omitted list repeats the previous non-empty one.
	values map[*ast.ValueSpec][]ast.Expr
	// scopes holds, for each constant spec, the constants declared by the
	// earlier specs of its declaration. A repeated list is a textual copy,
	// so inside a function its identifiers resolve where it is repeated: a
	// local constant's scope begins after its own spec, and may shadow the
	// name the original list referred to. At package level every name is
	// already in scope, so package-level constants do not use it.
	scopes map[*ast.ValueSpec]s7WaveCloseScope
	found  map[string]bool
	err    error
}

// s7WaveCloseScope maps a name to the function-local constant spec that
// declares it.
type s7WaveCloseScope map[string]*ast.ValueSpec

// s7WaveCloseDefinition is a constant's expression and the scope in which
// its identifiers resolve.
type s7WaveCloseDefinition struct {
	expr  ast.Expr
	scope s7WaveCloseScope
}

func scanS7WaveCloseSwitches(paths []string, found map[string]bool) error {
	type packageKey struct{ dir, name string }
	fset := token.NewFileSet()
	packages := map[packageKey][]*ast.File{}
	var keys []packageKey
	for _, path := range paths {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		key := packageKey{filepath.Dir(path), file.Name.Name}
		if _, seen := packages[key]; !seen {
			keys = append(keys, key)
		}
		packages[key] = append(packages[key], file)
	}
	for _, key := range keys {
		scan := &s7WaveCloseSwitchScan{fset: fset, consts: map[string][]ast.Expr{}, topLevel: map[*ast.ValueSpec]bool{},
			values: map[*ast.ValueSpec][]ast.Expr{}, scopes: map[*ast.ValueSpec]s7WaveCloseScope{}, found: found}
		for _, file := range packages[key] {
			ast.Inspect(file, func(node ast.Node) bool {
				if gen, ok := node.(*ast.GenDecl); ok && gen.Tok == token.CONST {
					var last []ast.Expr
					scope := s7WaveCloseScope{}
					for _, spec := range gen.Specs {
						value := spec.(*ast.ValueSpec)
						if len(value.Values) > 0 {
							last = value.Values
						}
						scan.values[value] = last
						scan.scopes[value] = scope
						scope = maps.Clone(scope)
						for _, name := range value.Names {
							scope[name.Name] = value
						}
					}
				}
				return true
			})
			for _, decl := range file.Decls {
				if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.CONST {
					for _, spec := range gen.Specs {
						value := spec.(*ast.ValueSpec)
						scan.topLevel[value] = true
						for i, name := range value.Names {
							if i < len(scan.values[value]) {
								scan.consts[name.Name] = append(scan.consts[name.Name], scan.values[value][i])
							}
						}
					}
				}
			}
		}
		for _, file := range packages[key] {
			ast.Inspect(file, scan.visit)
		}
		if scan.err != nil {
			return scan.err
		}
	}
	return nil
}

// fold evaluates a constant string expression to each value it can take.
// scope holds the local constants that take precedence over the parser's
// bindings, or is nil where those bindings are the compiler's.
func (s *s7WaveCloseSwitchScan) fold(expr ast.Expr, scope s7WaveCloseScope, depth int) ([]string, bool) {
	if depth > 64 || s.err != nil {
		return nil, false
	}
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			value, err := strconv.Unquote(e.Value)
			return []string{value}, err == nil
		}
	case *ast.ParenExpr:
		return s.fold(e.X, scope, depth+1)
	case *ast.Ident:
		definitions, ok := s.definitions(e, scope)
		if !ok || len(definitions) == 0 {
			return nil, false
		}
		var values []string
		for _, definition := range definitions {
			folded, ok := s.fold(definition.expr, definition.scope, depth+1)
			if !ok {
				return nil, false
			}
			values = append(values, folded...)
		}
		slices.Sort(values)
		return s.bound(slices.Compact(values), e.Pos())
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			x, ok := s.fold(e.X, scope, depth+1)
			if !ok {
				return nil, false
			}
			y, ok := s.fold(e.Y, scope, depth+1)
			if !ok {
				return nil, false
			}
			return s.concat(x, y, e.Pos())
		}
	}
	return nil, false
}

// definitions returns the constant expressions an identifier can stand for,
// or false if it does not name a constant.
func (s *s7WaveCloseSwitchScan) definitions(ident *ast.Ident, scope s7WaveCloseScope) ([]s7WaveCloseDefinition, bool) {
	if spec, ok := scope[ident.Name]; ok {
		return s.local(spec, ident.Name)
	}
	// ast.Object is deprecated, but the parser's resolver is the only scope
	// information available without type-checking the module.
	object := ident.Obj
	if object == nil {
		return s.packageLevel(ident.Name), true
	}
	spec, ok := object.Decl.(*ast.ValueSpec)
	if object.Kind != ast.Con || !ok {
		return nil, false
	}
	if s.topLevel[spec] {
		return s.packageLevel(ident.Name), true
	}
	return s.local(spec, ident.Name)
}

// packageLevel returns every declaration of a package-level constant.
func (s *s7WaveCloseSwitchScan) packageLevel(name string) []s7WaveCloseDefinition {
	var definitions []s7WaveCloseDefinition
	for _, expr := range s.consts[name] {
		definitions = append(definitions, s7WaveCloseDefinition{expr: expr})
	}
	return definitions
}

// local returns the expression of a function-local constant, resolved in
// the scope of the spec that declares it.
func (s *s7WaveCloseSwitchScan) local(spec *ast.ValueSpec, name string) ([]s7WaveCloseDefinition, bool) {
	index := slices.IndexFunc(spec.Names, func(ident *ast.Ident) bool { return ident.Name == name })
	values := s.values[spec]
	if index < 0 || index >= len(values) {
		return nil, false
	}
	return []s7WaveCloseDefinition{{expr: values[index], scope: s.scopes[spec]}}, true
}

// concat returns every value of x followed by every value of y.
func (s *s7WaveCloseSwitchScan) concat(x, y []string, pos token.Pos) ([]string, bool) {
	if len(x)*len(y) > s7WaveCloseFoldLimit {
		return s.bound(make([]string, len(x)*len(y)), pos)
	}
	var values []string
	for _, a := range x {
		for _, b := range y {
			values = append(values, a+b)
		}
	}
	return values, true
}

// bound fails the scan when an expression can take too many values to check.
func (s *s7WaveCloseSwitchScan) bound(values []string, pos token.Pos) ([]string, bool) {
	if len(values) > s7WaveCloseFoldLimit {
		s.err = fmt.Errorf("%s: constant string has %d possible values, more than %d can be checked",
			s.fset.Position(pos), len(values), s7WaveCloseFoldLimit)
		return nil, false
	}
	return values, true
}

func (s *s7WaveCloseSwitchScan) visit(node ast.Node) bool {
	if s.err != nil {
		return false
	}
	expr, ok := node.(ast.Expr)
	if !ok {
		return true
	}
	binary, isBinary := expr.(*ast.BinaryExpr)
	switch {
	case isBinary && binary.Op == token.ADD:
	case isBinary:
		return true
	default:
		if _, ok := expr.(*ast.BasicLit); !ok {
			if _, ok := expr.(*ast.ParenExpr); !ok {
				return true
			}
		}
	}
	if values, ok := s.fold(expr, nil, 0); ok {
		s.record(values, expr.Pos(), false)
		return false
	}
	if !isBinary || s.err != nil {
		return s.err == nil
	}
	// A concatenation with a non-constant operand: its constant runs are
	// checked on their own, and a run that ends where a non-constant operand
	// begins must not end inside a switch name.
	var operands []ast.Expr
	var flatten func(ast.Expr)
	flatten = func(e ast.Expr) {
		if b, ok := e.(*ast.BinaryExpr); ok && b.Op == token.ADD {
			flatten(b.X)
			flatten(b.Y)
			return
		}
		operands = append(operands, e)
	}
	flatten(binary)
	var run []string
	start := token.NoPos
	for _, operand := range operands {
		if values, ok := s.fold(operand, nil, 0); ok {
			if start == token.NoPos {
				run, start = values, operand.Pos()
			} else if run, ok = s.concat(run, values, operand.Pos()); !ok {
				return false
			}
			continue
		}
		if start != token.NoPos {
			s.record(run, start, true)
			run, start = nil, token.NoPos
		}
		ast.Inspect(operand, s.visit)
	}
	if start != token.NoPos {
		s.record(run, start, false)
	}
	return false
}

// record lists the switch names in each value of a constant string. openEnd
// means a non-constant operand follows it.
func (s *s7WaveCloseSwitchScan) record(values []string, pos token.Pos, openEnd bool) {
	for _, value := range values {
		for _, match := range s7WaveCloseSwitchName.FindAllStringSubmatchIndex(value, -1) {
			if s.err != nil {
				return
			}
			name, rest := value[match[0]:match[1]], value[match[2]:match[3]]
			switch {
			case rest == "" || strings.HasSuffix(rest, "_"):
				s.err = fmt.Errorf("%s: recording-switch prefix in %q is not a complete constant name", s.fset.Position(pos), value)
			case openEnd && match[1] == len(value):
				s.err = fmt.Errorf("%s: recording-switch name %q continues in a non-constant operand", s.fset.Position(pos), name)
			default:
				s.found[name] = true
			}
		}
	}
}

func TestS7WaveCloseRecordingSwitchDiscovery(t *testing.T) {
	root := avpRepoRoot(t)
	read := func(path string) string {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	// The prefixes are built at run time so this file's own constants do
	// not name a switch.
	recordPrefix := strings.Join([]string{"TPATCH", "RECORD", ""}, "_")
	updatePrefix := strings.Join([]string{"TPATCH", "UPDATE", ""}, "_")
	tree := func(t *testing.T, files map[string]string) string {
		t.Helper()
		dir := t.TempDir()
		for _, top := range []string{"cmd", "internal", "tests"} {
			if err := os.Mkdir(filepath.Join(dir, top), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		for path, body := range files {
			body = strings.NewReplacer("@R@", recordPrefix, "@U@", updatePrefix).Replace(body)
			full := filepath.Join(dir, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}

	got, err := s7WaveCloseRecordingSwitches(tree(t, map[string]string{
		"cmd/tool/main.go": "package main\n\nimport \"os\"\n\nfunc main() { _ = os.Getenv(`@R@RAW`) }\n",
		"internal/a/a.go": "package a\n\nimport \"os\"\n\nconst base = \"TPATCH_\"\n\n" +
			"// @R@COMMENT is only mentioned in a comment.\n" +
			"func f(err error) {\n" +
			"\t_ = os.Getenv(\"@R@\" + \"CONCAT\")\n" +
			"\t_ = os.Getenv(base + (\"UPDATE_\" + \"IDENT\"))\n" +
			"\t_ = os.Getenv(split + \"DATE_CROSS_FILE\")\n" +
			"\tprintln(\"re-run without @U@MESSAGE: \" + err.Error())\n" +
			"}\n",
		"internal/a/b.go":   "package a\n\nconst split = \"TPATCH_UP\"\n",
		"tests/x/notes.txt": "@R@NOT_GO\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		recordPrefix + "CONCAT", recordPrefix + "RAW",
		updatePrefix + "CROSS_FILE", updatePrefix + "IDENT", updatePrefix + "MESSAGE",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("recording switches = %q, want %q", got, want)
	}
	// Discovered names reach the refusal check.
	if validateS7WaveCloseLaneScript(read(s7WaveCloseLaneScriptPath), read("scripts/wave-close-test-shards.sh"), got) == nil {
		t.Fatal("lane runner accepted switches it does not refuse")
	}

	// Constants resolve as the compiler resolves them: within their own
	// package, with function-local constants and shadowing parameters bound
	// to their scope. A package-level name declared in two build-tagged files
	// folds to both values, whichever file comes first.
	got, err = s7WaveCloseRecordingSwitches(tree(t, map[string]string{
		"internal/a/a.go":      "package a\n\nimport \"os\"\n\nconst part = \"TPATCH_\"\n\nfunc f() { _ = os.Getenv(part + \"RECORD_COLLIDE\") }\n",
		"internal/a/a_test.go": "package a_test\n\nimport \"os\"\n\nconst part = \"OTHER_\"\n\nfunc g() { _ = os.Getenv(part + \"RECORD_EXTERNAL\") }\n",
		"internal/b/b.go":      "package b\n\nimport \"os\"\n\nfunc f() { _ = os.Getenv(part + \"UPDATE_TAGGED\") }\n",
		"internal/b/b_a.go":    "//go:build linux\n\npackage b\n\nconst part = \"TPATCH_\"\n",
		"internal/b/b_b.go":    "//go:build !linux\n\npackage b\n\nconst part = \"OTHER_\"\n",
		"internal/c/c.go":      "package c\n\nimport \"os\"\n\nfunc f() { _ = os.Getenv(part + \"UPDATE_REVERSED\") }\n",
		"internal/c/c_a.go":    "//go:build linux\n\npackage c\n\nconst part = \"OTHER_\"\n",
		"internal/c/c_b.go":    "//go:build !linux\n\npackage c\n\nconst part = \"TPATCH_\"\n",
		"internal/d/d.go": "package d\n\nimport \"os\"\n\n" +
			"func f() {\n\tconst part = \"TPATCH_\"\n\t_ = os.Getenv(part + \"RECORD_LOCAL\")\n}\n\n" +
			"func g() {\n\tconst part = \"OTHER_\"\n\t_ = os.Getenv(part + \"UPDATE_OTHER_SCOPE\")\n}\n\n" +
			"func h(part string) { _ = os.Getenv(part + \"RECORD_PARAMETER\") }\n",
		"internal/d/d_part.go": "package d\n\nconst part = \"TPATCH_\"\n",
		"internal/e/e.go": "package e\n\nimport \"os\"\n\nconst (\n\tp = \"TPATCH_\"\n\tq\n)\n\n" +
			"func f() { _ = os.Getenv(q + \"RECORD_IMPLICIT\") }\n\n" +
			"func g() {\n\tconst (\n\t\tx, y = \"OTHER_\", \"TPATCH_\"\n\t\tu, v\n\t)\n" +
			"\t_ = os.Getenv(v + \"UPDATE_IMPLICIT_LOCAL\")\n\t_ = os.Getenv(u + \"UPDATE_NOT_A_SWITCH\")\n}\n",
		// A repeated list resolves where it is repeated. In f, tail repeats
		// head's list but cannot see itself, so it is RECORD_REBOUND_OUTER;
		// rebound sees the local tail, so it is
		// RECORD_REBOUND_RECORD_REBOUND_OUTER. In g, again sees the tail its
		// source list declares, so it is RECORD_SOURCE_RECORD_SOURCE_OUTER.
		"internal/f/f.go": "package f\n\nimport \"os\"\n\nconst tail = \"OUTER\"\n\n" +
			"func f() {\n\tconst (\n\t\thead = \"RECORD_REBOUND_\" + tail\n\t\ttail\n\t\trebound\n\t)\n" +
			"\t_ = os.Getenv(\"TPATCH_\" + tail)\n\t_ = os.Getenv(\"TPATCH_\" + rebound)\n}\n\n" +
			"func g() {\n\tconst (\n\t\ttail = \"RECORD_SOURCE_\" + tail\n\t\tagain\n\t)\n" +
			"\t_ = os.Getenv(\"TPATCH_\" + again)\n}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want = []string{recordPrefix + "COLLIDE", recordPrefix + "IMPLICIT", recordPrefix + "LOCAL",
		recordPrefix + "REBOUND_OUTER", recordPrefix + "REBOUND_RECORD_REBOUND_OUTER",
		recordPrefix + "SOURCE_RECORD_SOURCE_OUTER",
		updatePrefix + "IMPLICIT_LOCAL", updatePrefix + "REVERSED", updatePrefix + "TAGGED"}
	if !slices.Equal(got, want) {
		t.Fatalf("recording switches across packages = %q, want %q", got, want)
	}

	for name, files := range map[string]map[string]string{
		"prefix-plus-variable": {"internal/a/a.go": "package a\n\nfunc f(s string) string { return \"@R@\" + s }\n"},
		"name-continues":       {"internal/a/a.go": "package a\n\nfunc f(s string) string { return \"@U@ROC\" + s + \".\" }\n"},
		"formatted":            {"internal/a/a.go": "package a\n\nimport \"fmt\"\n\nfunc f(s string) string { return fmt.Sprintf(\"@R@%s\", s) }\n"},
		"prefix-constant":      {"internal/a/a.go": "package a\n\nconst prefix = \"@U@\"\n"},
		"too-many-values": {
			"internal/a/a.go":       "package a\n\nconst p = \"A\"\n\nconst all = p + p + p + p + p + p + p\n",
			"internal/a/a_other.go": "//go:build ignore\n\npackage a\n\nconst p = \"B\"\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if names, err := s7WaveCloseRecordingSwitches(tree(t, files)); err == nil {
				t.Fatalf("partly constant switch name accepted, found %q", names)
			}
		})
	}
}

func validateS7WaveCloseLaneScript(lanes, shards string, switches []string) error {
	if !strings.HasPrefix(lanes, "#!/bin/sh\n") || strings.Count(lanes, "\nset -eu\n") != 1 {
		return errors.New("wave-close lane runner is not a set -eu POSIX shell script")
	}
	if strings.Count(lanes, "\nshards=scripts/wave-close-test-shards.sh\n") != 1 {
		return errors.New("wave-close lane runner does not derive its commands from the serial script")
	}
	var assignments = map[string]string{}
	for _, line := range strings.Split(lanes, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		for _, token := range []string{"-count", "-timeout", "-run ", "-skip ", "./...", "./internal/"} {
			if strings.Contains(line, token) {
				return fmt.Errorf("wave-close lane runner hardcodes go test arguments: %q", line)
			}
		}
		for _, name := range []string{"lane_a", "lane_b", "solo"} {
			if strings.HasPrefix(line, name+"=") {
				if _, seen := assignments[name]; seen {
					return fmt.Errorf("wave-close lane runner assigns %s twice", name)
				}
				value := strings.TrimPrefix(line, name+"=")
				if len(value) < 2 || value[0] != '\'' || value[len(value)-1] != '\'' {
					return fmt.Errorf("wave-close lane runner %s is not a single-quoted list", name)
				}
				assignments[name] = value[1 : len(value)-1]
			}
		}
	}
	var targets, commands []string
	for _, line := range strings.Split(shards, "\n") {
		if rest, ok := strings.CutPrefix(line, "go test -p=1 "); ok {
			target, _, _ := strings.Cut(rest, " ")
			targets = append(targets, target)
			commands = append(commands, line)
		}
	}
	// laneOf maps each shard to lane A (0), lane B (1) or the solo tail (2).
	laneOf := map[int]int{}
	var order [3][]int
	for index, name := range []string{"lane_a", "lane_b", "solo"} {
		value, ok := assignments[name]
		if !ok {
			return fmt.Errorf("wave-close lane runner does not assign %s", name)
		}
		for _, field := range strings.Fields(value) {
			shard, err := strconv.Atoi(field)
			if err != nil || shard < 1 || shard > len(targets) {
				return fmt.Errorf("wave-close lane runner names unknown shard %q", field)
			}
			if _, seen := laneOf[shard]; seen {
				return fmt.Errorf("wave-close lane runner schedules shard %d twice", shard)
			}
			laneOf[shard] = index
			order[index] = append(order[index], shard)
		}
	}
	if len(laneOf) != len(targets) {
		return fmt.Errorf("wave-close lanes cover %d of %d serial shards", len(laneOf), len(targets))
	}
	// The solo tail runs alone, so a heavy shard may sit there; every other
	// heavy shard shares one lane.
	heavyLane := -1
	for _, shard := range s7WaveCloseHeavyShards {
		switch lane := laneOf[shard]; {
		case lane == 2:
		case heavyLane < 0:
			heavyLane = lane
		case lane != heavyLane:
			return fmt.Errorf("heavy shard %d is not in the same lane as the other heavy lane shards", shard)
		}
	}
	// The solo tail holds exactly the shards of the solo tests, one each.
	for _, test := range s7WaveCloseSoloTests {
		count := 0
		for _, command := range commands {
			if strings.Contains(command, " -run '^"+test+"$'") {
				count++
			}
		}
		if count != 1 {
			return fmt.Errorf("solo test %s is the whole -run pattern of %d shards, want 1", test, count)
		}
	}
	for shard := 1; shard <= len(commands); shard++ {
		solo := slices.ContainsFunc(s7WaveCloseSoloTests, func(test string) bool {
			return strings.Contains(commands[shard-1], " -run '^"+test+"$'")
		})
		if solo != (laneOf[shard] == 2) {
			return fmt.Errorf("shard %d: in the solo tail %v, runs a solo test %v", shard, laneOf[shard] == 2, solo)
		}
	}
	// Shard 1's store tests create and remove a scratch directory inside the
	// checkout. Lane A runs it first, and the runner holds lane B's ./...
	// shards until it passes, so lane B must run its ./internal/cli shards
	// first or it would sit idle at that barrier.
	if laneOf[1] != 0 || order[0][0] != 1 {
		return errors.New("shard 1 must start lane A")
	}
	broad := false
	for _, shard := range order[1] {
		switch targets[shard-1] {
		case "./...":
			broad = true
		case "./internal/cli":
			if broad {
				return fmt.Errorf("shard %d (./internal/cli) runs after a ./... shard in lane B", shard)
			}
		default:
			return fmt.Errorf("shard %d has unclassified target %q", shard, targets[shard-1])
		}
	}
	start := strings.Index(lanes, "\n"+s7WaveCloseLaneRefusal)
	end := strings.Index(lanes, "; do\n\tif printenv \"$name\" >/dev/null; then\n")
	if start < 0 || end < start ||
		!strings.Contains(lanes[end:], "\t\texit 2\n\tfi\ndone\n") {
		return errors.New("wave-close lane runner lost its refusal loop")
	}
	refused := strings.Fields(strings.ReplaceAll(lanes[start+1+len(s7WaveCloseLaneRefusal):end], "\\\n", " "))
	for _, name := range append(slices.Clone(switches), s7WaveCloseAuditedUnsetVariables...) {
		if !slices.Contains(refused, name) {
			return fmt.Errorf("wave-close lane runner does not refuse %s", name)
		}
	}
	launch := strings.Index(lanes, "\n"+s7WaveCloseLaneLaunch)
	if launch < 0 || strings.Count(lanes, "\n"+s7WaveCloseLaneLaunch) != 1 {
		return errors.New("wave-close lane runner does not run both lanes concurrently and collect both statuses")
	}
	if strings.Count(lanes, "\n"+s7WaveCloseSoloLaunch) != 1 || strings.Index(lanes, "\n"+s7WaveCloseSoloLaunch) < launch {
		return errors.New("wave-close lane runner does not run the solo tail after both lanes passed")
	}
	if !strings.HasSuffix(lanes, s7WaveCloseLaneExit) {
		return errors.New("wave-close lane runner does not exit with both lanes' and the solo tail's status")
	}
	return nil
}

// s7WaveCloseShard1Marker is in shard 1's fake go record and in no other.
const s7WaveCloseShard1Marker = "[-skip][^Test(S6[A-Z].*|S7[A-Z].*|AVP.*)$]"

// s7WaveCloseFakeGo records each invocation's load-bearing environment and
// argv on one line. The commands of shard 1 and of lane B's first shard wait
// for each other, so a runner that does not run the lanes concurrently
// fails. A command matching FAKE_GO_SLOW records "done:" two seconds later,
// one matching FAKE_GO_FAIL fails, and with FAKE_GO_HANG set every command
// starts a child, a child that ignores SIGTERM and a grandchild whose parent
// has already exited, appends its own pid and theirs to that file and waits.
// It also starts a decoy that, on SIGTERM, closes the cancellation marker
// and outlives its parent: it keeps its pid but is no longer the runner's,
// as a process that took over a reused pid would be. Its pid goes to
// FAKE_GO_HANG.decoy.
const s7WaveCloseFakeGo = `#!/bin/sh
line="$GOMAXPROCS|$GOENV|$GOFLAGS|$BASH_ENV|"
for arg in "$@"; do line="$line[$arg]"; done
printf '%s\n' "$line" >>"$FAKE_GO_RECORD"
rendezvous() {
	: >"$FAKE_GO_SYNC/$1"
	i=0
	while [ ! -e "$FAKE_GO_SYNC/$2" ]; do
		i=$((i + 1))
		if [ "$i" -gt 200 ]; then
			echo "fake go: lanes did not overlap"
			exit 9
		fi
		sleep 0.05
	done
}
case "$line" in
*"$FAKE_GO_SYNC_A"*) rendezvous a b ;;
*"$FAKE_GO_SYNC_B"*) rendezvous b a ;;
esac
if [ -n "$FAKE_GO_HANG" ]; then
	sleep 60 &
	child=$!
	sh -c "trap '' TERM; exec sleep 60" &
	stubborn=$!
	orphan=$(sh -c 'sleep 60 >/dev/null 2>&1 & echo $!')
	sh -c 'trap "exec 9<&-" TERM; i=0; while [ "$i" -lt 60 ]; do sleep 1; i=$((i + 1)); done' &
	printf '%s\n' "$!" >>"$FAKE_GO_HANG.decoy"
	printf '%s %s %s %s\n' "$$" "$child" "$stubborn" "$orphan" >>"$FAKE_GO_HANG"
	wait
fi
case "$line" in
*"$FAKE_GO_SLOW"*)
	if [ -n "$FAKE_GO_SLOW" ]; then
		sleep 2
		printf 'done:%s\n' "$line" >>"$FAKE_GO_RECORD"
	fi
	;;
esac
case "$line" in
*"$FAKE_GO_FAIL"*)
	if [ -n "$FAKE_GO_FAIL" ]; then
		echo "fake go failure: $*"
		exit 1
	fi
	;;
esac
`

type s7WaveCloseLaneRun struct {
	records   []string
	output    string
	code      int
	timedOut  bool
	leftovers int
	// hung holds the pids of hung fake go commands and their children;
	// survivors holds those still alive after the runner was cancelled.
	hung, survivors []int
	// decoys holds the decoys' pids; spared holds those the runner did not
	// kill. A runner that signals only processes it still owns spares all.
	decoys, spared []int
}

// s7WaveCloseLaneOptions configures one fixture run; nil args means "2".
type s7WaveCloseLaneOptions struct {
	args []string
	env  []string
	// cancel hangs every fake go command, sends SIGTERM to the runner once
	// both lanes are inside one, then SIGHUP and SIGTERM again while it is
	// cleaning up, and records which hung processes survive, including
	// those that ignore SIGTERM or have left the process tree.
	cancel bool
}

func runS7WaveCloseLaneFixture(t *testing.T, lanes, shards string, options s7WaveCloseLaneOptions) s7WaveCloseLaneRun {
	t.Helper()
	root := t.TempDir()
	for path, body := range map[string]string{
		"scripts/wave-close-test-lanes.sh":  lanes,
		"scripts/wave-close-test-shards.sh": shards,
		"bin/go":                            s7WaveCloseFakeGo,
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{"sync", "tmp"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	record := filepath.Join(root, "record")
	hung := ""
	if options.cancel {
		hung = filepath.Join(root, "hung")
	}
	var env []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "TPATCH_") || strings.HasPrefix(name, "GIT_") || strings.HasPrefix(name, "FAKE_GO_") ||
			slices.Contains([]string{"PATH", "GOMAXPROCS", "GOENV", "GOFLAGS", "BASH_ENV", "TMPDIR"}, name) {
			continue
		}
		env = append(env, entry)
	}
	env = append(env,
		"PATH="+filepath.Join(root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GOMAXPROCS=7", "GOENV=on", "GOFLAGS=-v", "BASH_ENV=/nonexistent",
		"TMPDIR="+filepath.Join(root, "tmp"),
		"FAKE_GO_RECORD="+record,
		"FAKE_GO_SYNC="+filepath.Join(root, "sync"),
		"FAKE_GO_SYNC_A="+s7WaveCloseShard1Marker,
		"FAKE_GO_SYNC_B=[-run][^TestS7ARRev16.*$]",
		"FAKE_GO_HANG="+hung,
	)
	env = append(env, options.env...)
	args := options.args
	if args == nil {
		args = []string{"2"}
	}
	// A runner that does not finish is stopped as an operator would stop it.
	ctx, stop := context.WithTimeout(context.Background(), time.Minute)
	defer stop()
	cmd := exec.CommandContext(ctx, "sh", append([]string{s7WaveCloseLaneScriptPath}, args...)...)
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 10 * time.Second
	cmd.Dir = root
	cmd.Env = env
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var run s7WaveCloseLaneRun
	if options.cancel {
		run.hung = awaitS7WaveCloseHung(hung, 8, 20*time.Second)
		run.decoys = awaitS7WaveCloseHung(hung+".decoy", 2, 0)
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		// A second cancellation, while cleanup waits on the processes that
		// ignore SIGTERM, must not cut it short. The runner may have exited.
		time.Sleep(1500 * time.Millisecond)
		_ = cmd.Process.Signal(syscall.SIGHUP)
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}
	err := cmd.Wait()
	run.output = output.String()
	run.timedOut = ctx.Err() != nil
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		run.code = exitErr.ExitCode()
	} else if err != nil && !run.timedOut {
		t.Fatal(err)
	}
	if options.cancel {
		run.survivors = s7WaveCloseSurvivors(run.hung, 5*time.Second)
		run.spared = s7WaveCloseSurvivors(run.decoys, 0)
		for _, pid := range append(slices.Clone(run.survivors), run.spared...) {
			if process, err := os.FindProcess(pid); err == nil {
				_ = process.Kill()
			}
		}
	}
	if body, err := os.ReadFile(record); err == nil {
		run.records = strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	leftovers, err := os.ReadDir(filepath.Join(root, "tmp"))
	if err != nil {
		t.Fatal(err)
	}
	run.leftovers = len(leftovers)
	return run
}

// awaitS7WaveCloseHung waits until the hung fake go commands have recorded
// want pids, then returns the pids recorded so far.
func awaitS7WaveCloseHung(path string, want int, limit time.Duration) []int {
	deadline := time.Now().Add(limit)
	for {
		var pids []int
		body, _ := os.ReadFile(path)
		lines := strings.Split(string(body), "\n")
		for _, line := range lines[:len(lines)-1] {
			for _, field := range strings.Fields(line) {
				if pid, err := strconv.Atoi(field); err == nil {
					pids = append(pids, pid)
				}
			}
		}
		if len(pids) >= want || time.Now().After(deadline) {
			return pids
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// s7WaveCloseSurvivors returns the pids still running once limit has passed,
// or none as soon as all have exited. A zombie counts as exited.
func s7WaveCloseSurvivors(pids []int, limit time.Duration) []int {
	deadline := time.Now().Add(limit)
	for {
		var alive []int
		for _, pid := range pids {
			out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
			if state := strings.TrimSpace(string(out)); err == nil && state != "" && !strings.HasPrefix(state, "Z") {
				alive = append(alive, pid)
			}
		}
		if len(alive) == 0 || time.Now().After(deadline) {
			return alive
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// s7WaveCloseSerialRecords is what the fake go records when the serial
// script runs: its pinned environment and each command's argv, in order.
func s7WaveCloseSerialRecords(shards string) ([]string, error) {
	commands, err := parseS7ShellInvocations(strings.Join(slices.DeleteFunc(
		strings.Split(shards, "\n"),
		func(line string) bool { return !strings.HasPrefix(line, "go test -p=1 ") },
	), "\n"))
	if err != nil {
		return nil, err
	}
	var records []string
	for _, argv := range commands {
		line := "1|off||/dev/null|"
		for _, arg := range argv[1:] {
			line += "[" + arg + "]"
		}
		records = append(records, line)
	}
	return records, nil
}

func checkS7WaveCloseLaneRecords(serial, lanes []string) error {
	if len(lanes) != len(serial) {
		return fmt.Errorf("lane runner ran %d commands, serial script has %d", len(lanes), len(serial))
	}
	got, want := slices.Sorted(slices.Values(lanes)), slices.Sorted(slices.Values(serial))
	if !slices.Equal(got, want) {
		return fmt.Errorf("lane runner commands or environment differ from the serial script:\n got %q\nwant %q", got, want)
	}
	return nil
}

// checkS7WaveCloseLaneBehavior runs the lane runner against the fake go and
// returns the first way it breaks the serial script's guarantees.
func checkS7WaveCloseLaneBehavior(t *testing.T, lanes, shards string, serial []string) error {
	t.Helper()
	run := func(options s7WaveCloseLaneOptions) (s7WaveCloseLaneRun, error) {
		result := runS7WaveCloseLaneFixture(t, lanes, shards, options)
		switch {
		case result.timedOut:
			return result, fmt.Errorf("lane runner did not finish within a minute:\n%s", result.output)
		case result.leftovers != 0:
			return result, fmt.Errorf("lane runner left %d temporary entries", result.leftovers)
		}
		return result, nil
	}
	walks := func(record string) bool { return strings.Contains(record, "[./...]") }

	// Every serial command runs once, with the serial environment.
	pass, err := run(s7WaveCloseLaneOptions{})
	if err != nil {
		return err
	}
	if pass.code != 0 {
		return fmt.Errorf("lane runner exit %d:\n%s", pass.code, pass.output)
	}
	if err := checkS7WaveCloseLaneRecords(serial, pass.records); err != nil {
		return err
	}
	// The solo shards run last, one at a time, in serial order.
	var solo []string
	for _, test := range s7WaveCloseSoloTests {
		index := slices.IndexFunc(serial, func(record string) bool { return strings.Contains(record, s7WaveCloseSoloRecord(test)) })
		if index < 0 {
			return fmt.Errorf("no serial command runs %s", test)
		}
		solo = append(solo, serial[index])
	}
	isSolo := func(record string) bool { return slices.Contains(solo, record) }
	if tail := pass.records[len(pass.records)-len(solo):]; !slices.Equal(tail, solo) {
		return fmt.Errorf("lane runner ended with %q, want the solo shards %q", tail, solo)
	}

	// A failure in lane A fails the runner, the solo tail does not run, and
	// the output ends with lane A.
	failing, err := run(s7WaveCloseLaneOptions{env: []string{"FAKE_GO_FAIL=[^TestS6[A-F].*$]"}})
	if err != nil {
		return err
	}
	lastHeader := strings.LastIndex(failing.output, "=== lane ")
	if failing.code == 0 || lastHeader < 0 ||
		!strings.HasPrefix(failing.output[lastHeader:], "=== lane A ") ||
		!strings.Contains(failing.output[lastHeader:], "fake go failure:") {
		return fmt.Errorf("lane failure not reported last (exit %d):\n%s", failing.code, failing.output)
	}
	if slices.ContainsFunc(failing.records, isSolo) || !strings.Contains(failing.output, "): not run, a lane failed ===") {
		return fmt.Errorf("solo tail ran or was not reported after a lane failed:\n%s", failing.output)
	}

	// A solo failure fails the runner, stops the tail and is reported last.
	soloFailing, err := run(s7WaveCloseLaneOptions{env: []string{"FAKE_GO_FAIL=" + s7WaveCloseSoloRecord(s7WaveCloseSoloTests[1])}})
	if err != nil {
		return err
	}
	soloHeader := strings.LastIndex(soloFailing.output, "=== solo (shards ")
	if soloFailing.code == 0 || soloHeader < strings.LastIndex(soloFailing.output, "=== lane ") ||
		!strings.Contains(soloFailing.output[soloHeader:], "fake go failure:") ||
		!slices.Contains(soloFailing.records, solo[0]) || !slices.Contains(soloFailing.records, solo[1]) ||
		slices.Contains(soloFailing.records, solo[2]) || len(soloFailing.records) != len(serial)-1 {
		return fmt.Errorf("solo failure (exit %d, %d commands) not stopped and reported last:\n%s",
			soloFailing.code, len(soloFailing.records), soloFailing.output)
	}

	// Lane B starts no ./... shard until shard 1 has finished.
	slow, err := run(s7WaveCloseLaneOptions{env: []string{"FAKE_GO_SLOW=" + s7WaveCloseShard1Marker}})
	if err != nil {
		return err
	}
	done := slices.Index(slow.records, "done:"+serial[0])
	if slow.code != 0 || done < 0 {
		return fmt.Errorf("slow shard 1: exit %d, finished at record %d:\n%s", slow.code, done, slow.output)
	}
	for _, record := range slow.records[:done] {
		if walks(record) && record != serial[0] {
			return fmt.Errorf("lane runner started %q while shard 1 was running", record)
		}
	}

	// When shard 1 fails, no ./... shard starts and lane B says why.
	stopped, err := run(s7WaveCloseLaneOptions{env: []string{"FAKE_GO_FAIL=" + s7WaveCloseShard1Marker}})
	if err != nil {
		return err
	}
	broad := slices.DeleteFunc(slices.Clone(stopped.records), func(record string) bool { return !walks(record) })
	if stopped.code == 0 || len(broad) != 1 || !strings.Contains(stopped.output, "shard 1 failed") {
		return fmt.Errorf("shard 1 failure (exit %d) left ./... commands %q running:\n%s", stopped.code, broad, stopped.output)
	}

	// Cancelling the runner, even twice, stops every process it started and
	// signals none that it no longer owns.
	cancelled, err := run(s7WaveCloseLaneOptions{cancel: true})
	if err != nil {
		return err
	}
	if cancelled.code != 143 || len(cancelled.hung) != 8 || len(cancelled.survivors) != 0 ||
		len(cancelled.decoys) != 2 || len(cancelled.spared) != 2 {
		return fmt.Errorf("cancelled lane runner: exit %d, hung pids %v, survivors %v, decoys %v, spared %v:\n%s",
			cancelled.code, cancelled.hung, cancelled.survivors, cancelled.decoys, cancelled.spared, cancelled.output)
	}
	return nil
}

func TestS7WaveCloseLaneRunner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("wave-close-check is a POSIX make/sh gate that is not run on Windows (Windows portability: GH #17)")
	}
	root := avpRepoRoot(t)
	read := func(path string) string {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	lanes := read(s7WaveCloseLaneScriptPath)
	shards := read("scripts/wave-close-test-shards.sh")
	serial, err := s7WaveCloseSerialRecords(shards)
	if err != nil || len(serial) == 0 {
		t.Fatalf("serial commands: %d, %v", len(serial), err)
	}
	if !strings.Contains(serial[0], s7WaveCloseShard1Marker) ||
		slices.ContainsFunc(serial[1:], func(record string) bool { return strings.Contains(record, s7WaveCloseShard1Marker) }) {
		t.Fatal("the shard 1 marker does not single out shard 1")
	}
	if err := checkS7WaveCloseLaneBehavior(t, lanes, shards, serial); err != nil {
		t.Fatal(err)
	}

	pass := runS7WaveCloseLaneFixture(t, lanes, shards, s7WaveCloseLaneOptions{})
	for _, mutation := range [][]string{
		pass.records[1:],
		append(slices.Clone(pass.records[1:]), strings.Replace(pass.records[0], "1|off|", "7|off|", 1)),
	} {
		if checkS7WaveCloseLaneRecords(serial, mutation) == nil {
			t.Fatal("lane record check accepted wrong records")
		}
	}

	for _, refused := range []struct {
		name  string
		args  []string
		extra []string
		shard string
	}{
		{name: "lane-count", args: []string{"3"}},
		{name: "recording-switch", args: []string{"2"}, extra: []string{"TPATCH_UPDATE_ROC_S0_GOLDENS="}},
		{name: "git-redirection", args: []string{"2"}, extra: []string{"GIT_DIR=/elsewhere"}},
		{name: "partition-drift", args: []string{"2"}, shard: "go test -p=1 ./internal/cli -count=1 -timeout 40m -run '^TestNew$'\n"},
	} {
		t.Run(refused.name, func(t *testing.T) {
			run := runS7WaveCloseLaneFixture(t, lanes, shards+refused.shard, s7WaveCloseLaneOptions{args: refused.args, env: refused.extra})
			if run.code != 2 || len(run.records) != 0 || run.leftovers != 0 {
				t.Fatalf("lane runner did not refuse (exit %d, %d commands):\n%s", run.code, len(run.records), run.output)
			}
		})
	}
	// Without lsof or /proc, cancellation could not find escaped processes.
	// On macOS lsof lives in /usr/sbin, so a PATH without it removes it.
	t.Run("no-lsof-or-proc", func(t *testing.T) {
		if runtime.GOOS != "darwin" {
			t.Skip("needs a host where lsof is outside /bin:/usr/bin and /proc is absent")
		}
		for _, path := range []string{"/proc/self/fd", "/bin/lsof", "/usr/bin/lsof"} {
			if _, err := os.Stat(path); err == nil {
				t.Skipf("%s exists", path)
			}
		}
		run := runS7WaveCloseLaneFixture(t, lanes, shards, s7WaveCloseLaneOptions{env: []string{"PATH=/bin:/usr/bin"}})
		if run.code != 2 || len(run.records) != 0 || run.leftovers != 0 ||
			!strings.Contains(run.output, "need lsof or /proc") {
			t.Fatalf("lane runner did not refuse (exit %d, %d commands):\n%s", run.code, len(run.records), run.output)
		}
	})

	// The behavioral checks must fail against broken runners.
	for _, broken := range []struct {
		name, from, to string
	}{
		{name: "preamble-dropped", from: "\tn == 0 { preamble = preamble $0 \"\\n\" }\n", to: ""},
		{name: "lanes-serialized", from: s7WaveCloseLaneLaunch, to: "status_a=0\n" +
			"run_lane A \"$lane_a\" >\"$tmp/a.out\" 2>&1 9<\"$marker\" || status_a=$?\n" +
			"status_b=0\n" +
			"run_lane B \"$lane_b\" >\"$tmp/b.out\" 2>&1 9<\"$marker\" || status_b=$?\n"},
		{name: "status-masked", from: s7WaveCloseLaneExit, to: "\ntrue\n"},
		{name: "solo-status-masked", from: " && [ \"$status_s\" -eq 0 ]\n", to: "\n"},
		{name: "barrier-removed", from: "\t\t\"B\"*\" $shard \"*) await_shard_1 || return 1 ;;\n", to: ""},
		{name: "barrier-ignores-failure", from: "\tif [ \"$(cat \"$tmp/shard-1.status\")\" != 0 ]; then\n", to: "\tif false; then\n"},
		{name: "lane-shells-only-stopped", from: "owned=$(members | tr '\\n' ' ')", to: "owned=\"$pid_a $pid_b\""},
		{name: "kill-escalation-removed", from: "kill -KILL $left 2>/dev/null || true", to: ":"},
		{name: "escapees-not-tracked", from: "{ descendants; marked; }", to: "{ descendants; }"},
		{name: "remembered-pids-killed", from: "\t\tfrozen=\n\t\tfreeze\n\t\tleft=$(alive $(verified) | tr '\\n' ' ')\n",
			to: "\t\tfreeze\n\t\tleft=$(alive $frozen | tr '\\n' ' ')\n"},
		{name: "second-signal-exits", from: s7WaveCloseLaneTraps, to: "trap cleanup EXIT\n" +
			"trap 'exit 130' INT\ntrap 'exit 143' TERM\ntrap 'exit 129' HUP\n"},
		{name: "solo-runs-after-lane-failure", from: "if [ \"$status_a\" -eq 0 ] && [ \"$status_b\" -eq 0 ]; then\n", to: "if true; then\n"},
		{name: "solo-in-lane-b", from: "lane_b='18 ", to: "lane_b='31 32 33 18 "},
	} {
		t.Run(broken.name, func(t *testing.T) {
			mutated := strings.Replace(lanes, broken.from, broken.to, 1)
			if broken.name == "solo-in-lane-b" {
				mutated = strings.Replace(mutated, "\nsolo='31 32 33'\n", "\nsolo=''\n", 1)
			}
			if mutated == lanes {
				t.Fatal("mutation changed nothing")
			}
			if checkS7WaveCloseLaneBehavior(t, mutated, shards, serial) == nil {
				t.Fatal("broken lane runner passed the behavioral checks")
			}
		})
	}
}
