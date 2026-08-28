//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// ─── shared AV source program ─────────────────────────────────────────────────

// The AV guards read real production source text and analyse it exactly as the
// shipped bytes are analysed, so a semantic sensitivity fixture is a mutation of
// that same text rather than of a hand-written model.
const (
	s7AVStoreArchiveSource = "internal/store/intent_archive.go"
	s7AVCLIArchiveSource   = "internal/cli/feature_intent_archive.go"
	s7AVCLIPrepareSource   = "internal/cli/prepare_publish.go"
	s7AVPRDRelPath         = "docs/prds/PRD-prepare-intent-bundle.md"
	s7AVADRRelPath         = "docs/adrs/ADR-035-intent-bundle-publication-and-history.md"
)

type s7AVProgram struct {
	fset  *token.FileSet
	order []string
	files map[string]*ast.File
	text  map[string]string
}

func s7AVParseProgram(sources map[string]string, order []string) (*s7AVProgram, error) {
	program := &s7AVProgram{
		fset:  token.NewFileSet(),
		order: order,
		files: map[string]*ast.File{},
		text:  map[string]string{},
	}
	for _, name := range order {
		body, ok := sources[name]
		if !ok {
			return nil, fmt.Errorf("archive source %q is missing", name)
		}
		file, err := parser.ParseFile(program.fset, name, body, 0)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		program.files[name] = file
		program.text[name] = body
	}
	return program, nil
}

// function returns the single top-level, receiverless declaration of name.
func (program *s7AVProgram) function(name string) *ast.FuncDecl {
	for _, source := range program.order {
		for _, declaration := range program.files[source].Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && function.Name.Name == name && function.Body != nil {
				return function
			}
		}
	}
	return nil
}

// functions indexes every top-level receiverless function in the program.
func (program *s7AVProgram) functions() map[string]*ast.FuncDecl {
	index := map[string]*ast.FuncDecl{}
	for _, source := range program.order {
		for _, declaration := range program.files[source].Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && function.Body != nil {
				index[function.Name.Name] = function
			}
		}
	}
	return index
}

// stringConstants collects every `Name Type = "value"` constant, which is how
// the wire-state, blob-observation, disposition, action, code and repair-class
// vocabularies are declared.
func (program *s7AVProgram) stringConstants() map[string]string {
	constants := map[string]string{}
	for _, source := range program.order {
		for _, declaration := range program.files[source].Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.CONST {
				continue
			}
			for _, spec := range generic.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok || len(value.Names) != len(value.Values) {
					continue
				}
				for index, name := range value.Names {
					literal, ok := value.Values[index].(*ast.BasicLit)
					if !ok || literal.Kind != token.STRING {
						continue
					}
					decoded, err := strconv.Unquote(literal.Value)
					if err != nil {
						continue
					}
					constants[name.Name] = decoded
				}
			}
		}
	}
	return constants
}

// typedConstantNames returns the declared constant names of one named type, in
// declaration order. The domain of §9.3's table is derived from these rather
// than from the document.
func (program *s7AVProgram) typedConstantNames(typeName string) []string {
	names := []string{}
	for _, source := range program.order {
		for _, declaration := range program.files[source].Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.CONST {
				continue
			}
			for _, spec := range generic.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok || value.Type == nil {
					continue
				}
				ident, ok := value.Type.(*ast.Ident)
				if !ok || ident.Name != typeName {
					continue
				}
				for _, name := range value.Names {
					names = append(names, name.Name)
				}
			}
		}
	}
	return names
}

func s7AVRepoSources(t *testing.T, names ...string) map[string]string {
	t.Helper()
	sources := map[string]string{}
	for _, name := range names {
		sources[name] = s6RepoFile(t, name)
	}
	return sources
}

func s7AVRepoDocument(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(avpRepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func s7AVMutate(t *testing.T, sources map[string]string, source, old, new string, count int) map[string]string {
	t.Helper()
	mutated := map[string]string{}
	for name, body := range sources {
		mutated[name] = body
	}
	replaced := strings.Replace(mutated[source], old, new, count)
	if replaced == mutated[source] {
		t.Fatalf("sensitivity fixture changed nothing in %s", source)
	}
	mutated[source] = replaced
	return mutated
}

// ─── PIB-551 ──────────────────────────────────────────────────────────────────

// s7AVTuple mirrors the 4-tuple domain of §9.3 with plain strings, so the
// interpreter below can produce it from mutated source without depending on the
// shipped types.
type s7AVTuple struct {
	Wire  string
	Blob  string
	Owned bool
	Live  bool
}

func (tuple s7AVTuple) String() string {
	return fmt.Sprintf("%s|%s|owned=%t|live=%t", tuple.Wire, tuple.Blob, tuple.Owned, tuple.Live)
}

// s7AVClassification is the interpreted result of ClassifyIntentArchiveTuple.
type s7AVClassification struct {
	Reachable   bool
	Disposition string
	Action      string
	Code        string
	RepairClass string
	ExitClass   int
}

// s7AVVocabulary carries the four dimension vocabularies as the production
// constants declare them, so neither the dependencies below nor the table parse
// hard-codes a wire value.
type s7AVVocabulary struct {
	retained       string
	removalPending string
	tombstoned     string
	absent         string
	presentCorrect string
	unidentifiable string
}

func s7AVDeriveVocabulary(constants map[string]string) (s7AVVocabulary, error) {
	vocab := s7AVVocabulary{
		retained:       constants["IntentArchiveWireRetained"],
		removalPending: constants["IntentArchiveWireRemovalPending"],
		tombstoned:     constants["IntentArchiveWireTombstoned"],
		absent:         constants["IntentArchiveBlobAbsent"],
		presentCorrect: constants["IntentArchiveBlobPresentCorrect"],
		unidentifiable: constants["IntentArchiveBlobUnidentifiable"],
	}
	for label, value := range map[string]string{
		"IntentArchiveWireRetained":       vocab.retained,
		"IntentArchiveWireRemovalPending": vocab.removalPending,
		"IntentArchiveWireTombstoned":     vocab.tombstoned,
		"IntentArchiveBlobAbsent":         vocab.absent,
		"IntentArchiveBlobPresentCorrect": vocab.presentCorrect,
		"IntentArchiveBlobUnidentifiable": vocab.unidentifiable,
	} {
		if value == "" {
			return vocab, fmt.Errorf("the dimension constant %s is missing", label)
		}
	}
	return vocab, nil
}

// s7AVDependencyReachable implements §9.3's three stated dependencies directly,
// independently of the shipped predicate: a retained reference makes its hash
// live; a removal-pending reference makes its hash owned and live; owned implies
// live. The guard derives reachability from these, then requires the shipped
// predicate to agree.
func s7AVDependencyReachable(tuple s7AVTuple, vocab s7AVVocabulary) bool {
	if tuple.Owned && !tuple.Live {
		return false
	}
	switch tuple.Wire {
	case vocab.retained:
		return tuple.Live
	case vocab.removalPending:
		return tuple.Owned && tuple.Live
	case vocab.tombstoned:
		return true
	default:
		return false
	}
}

// ─── bounded interpreter over the shipped classifier ──────────────────────────

type s7AVInterpreter struct {
	program   *s7AVProgram
	constants map[string]string
	wire      map[string]string
	blob      map[string]string
}

type s7AVEvalError struct{ detail string }

func (err *s7AVEvalError) Error() string { return err.detail }

func s7AVNewInterpreter(program *s7AVProgram) (*s7AVInterpreter, error) {
	interpreter := &s7AVInterpreter{
		program:   program,
		constants: program.stringConstants(),
		wire:      map[string]string{},
		blob:      map[string]string{},
	}
	for _, name := range program.typedConstantNames("IntentArchiveWireState") {
		interpreter.wire[name] = interpreter.constants[name]
	}
	for _, name := range program.typedConstantNames("IntentArchiveBlobState") {
		interpreter.blob[name] = interpreter.constants[name]
	}
	if len(interpreter.wire) == 0 || len(interpreter.blob) == 0 {
		return nil, &s7AVEvalError{detail: "the tuple domain could not be derived from the declared constants"}
	}
	return interpreter, nil
}

type s7AVFlow int

const (
	s7AVFlowNormal s7AVFlow = iota
	s7AVFlowReturned
)

// evalBool evaluates the restricted boolean expression grammar the two shipped
// predicates use. Anything outside it is an interpreter error, never a silent
// pass.
func (in *s7AVInterpreter) evalBool(expression ast.Expr, tuple s7AVTuple) (bool, error) {
	switch typed := expression.(type) {
	case *ast.ParenExpr:
		return in.evalBool(typed.X, tuple)
	case *ast.UnaryExpr:
		if typed.Op != token.NOT {
			return false, &s7AVEvalError{detail: "unsupported unary operator " + typed.Op.String()}
		}
		value, err := in.evalBool(typed.X, tuple)
		return !value, err
	case *ast.BinaryExpr:
		switch typed.Op {
		case token.LAND:
			left, err := in.evalBool(typed.X, tuple)
			if err != nil || !left {
				return false, err
			}
			return in.evalBool(typed.Y, tuple)
		case token.LOR:
			left, err := in.evalBool(typed.X, tuple)
			if err != nil || left {
				return left, err
			}
			return in.evalBool(typed.Y, tuple)
		case token.EQL, token.NEQ:
			left, err := in.evalString(typed.X, tuple)
			if err != nil {
				return false, err
			}
			right, err := in.evalString(typed.Y, tuple)
			if err != nil {
				return false, err
			}
			if typed.Op == token.EQL {
				return left == right, nil
			}
			return left != right, nil
		}
		return false, &s7AVEvalError{detail: "unsupported binary operator " + typed.Op.String()}
	case *ast.SelectorExpr:
		if base, ok := typed.X.(*ast.Ident); ok && base.Name == "tuple" {
			switch typed.Sel.Name {
			case "Owned":
				return tuple.Owned, nil
			case "Live":
				return tuple.Live, nil
			}
		}
		return false, &s7AVEvalError{detail: "unsupported selector in boolean position"}
	case *ast.CallExpr:
		name, ok := typed.Fun.(*ast.Ident)
		if !ok || name.Name != "IntentArchiveTupleReachable" || len(typed.Args) != 1 {
			return false, &s7AVEvalError{detail: "unsupported call in boolean position"}
		}
		return in.reachable(tuple)
	case *ast.Ident:
		switch typed.Name {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
		return false, &s7AVEvalError{detail: "unsupported identifier " + typed.Name + " in boolean position"}
	}
	return false, &s7AVEvalError{detail: "unsupported boolean expression"}
}

// evalString resolves a tuple field or a declared string constant.
func (in *s7AVInterpreter) evalString(expression ast.Expr, tuple s7AVTuple) (string, error) {
	switch typed := expression.(type) {
	case *ast.ParenExpr:
		return in.evalString(typed.X, tuple)
	case *ast.SelectorExpr:
		if base, ok := typed.X.(*ast.Ident); ok && base.Name == "tuple" {
			switch typed.Sel.Name {
			case "WireState":
				return tuple.Wire, nil
			case "BlobState":
				return tuple.Blob, nil
			}
		}
		return "", &s7AVEvalError{detail: "unsupported selector in string position"}
	case *ast.Ident:
		if value, ok := in.constants[typed.Name]; ok {
			return value, nil
		}
		return "", &s7AVEvalError{detail: "unknown constant " + typed.Name}
	case *ast.BasicLit:
		if typed.Kind == token.STRING {
			return strconv.Unquote(typed.Value)
		}
	}
	return "", &s7AVEvalError{detail: "unsupported string expression"}
}

// reachable interprets the shipped IntentArchiveTupleReachable body.
func (in *s7AVInterpreter) reachable(tuple s7AVTuple) (bool, error) {
	function := in.program.function("IntentArchiveTupleReachable")
	if function == nil {
		return false, &s7AVEvalError{detail: "IntentArchiveTupleReachable is missing"}
	}
	value, flow, err := in.runBool(function.Body.List, tuple)
	if err != nil {
		return false, err
	}
	if flow != s7AVFlowReturned {
		return false, &s7AVEvalError{detail: "IntentArchiveTupleReachable fell through without returning"}
	}
	return value, nil
}

// runBool executes a statement list whose only returns are boolean.
func (in *s7AVInterpreter) runBool(statements []ast.Stmt, tuple s7AVTuple) (bool, s7AVFlow, error) {
	for _, statement := range statements {
		switch typed := statement.(type) {
		case *ast.ReturnStmt:
			if len(typed.Results) != 1 {
				return false, s7AVFlowNormal, &s7AVEvalError{detail: "unsupported boolean return arity"}
			}
			value, err := in.evalBool(typed.Results[0], tuple)
			return value, s7AVFlowReturned, err
		case *ast.IfStmt:
			condition, err := in.evalBool(typed.Cond, tuple)
			if err != nil {
				return false, s7AVFlowNormal, err
			}
			if condition {
				value, flow, err := in.runBool(typed.Body.List, tuple)
				if err != nil || flow == s7AVFlowReturned {
					return value, flow, err
				}
				continue
			}
			if typed.Else == nil {
				continue
			}
			value, flow, err := in.runBool(s7AVElseStatements(typed.Else), tuple)
			if err != nil || flow == s7AVFlowReturned {
				return value, flow, err
			}
		case *ast.SwitchStmt:
			body, err := in.selectCase(typed, tuple)
			if err != nil {
				return false, s7AVFlowNormal, err
			}
			value, flow, err := in.runBool(body, tuple)
			if err != nil || flow == s7AVFlowReturned {
				return value, flow, err
			}
		default:
			return false, s7AVFlowNormal, &s7AVEvalError{detail: "unsupported statement in a boolean predicate"}
		}
	}
	return false, s7AVFlowNormal, nil
}

func s7AVElseStatements(node ast.Stmt) []ast.Stmt {
	switch typed := node.(type) {
	case *ast.BlockStmt:
		return typed.List
	default:
		return []ast.Stmt{typed}
	}
}

// selectCase resolves the matching clause body of a tag switch over a tuple
// field, returning the default clause when no case matches.
func (in *s7AVInterpreter) selectCase(node *ast.SwitchStmt, tuple s7AVTuple) ([]ast.Stmt, error) {
	if node.Init != nil {
		return nil, &s7AVEvalError{detail: "unsupported switch initializer"}
	}
	var fallback []ast.Stmt
	if node.Tag == nil {
		for _, clause := range node.Body.List {
			caseClause, ok := clause.(*ast.CaseClause)
			if !ok {
				return nil, &s7AVEvalError{detail: "unsupported switch clause"}
			}
			if caseClause.List == nil {
				fallback = caseClause.Body
				continue
			}
			for _, candidate := range caseClause.List {
				matched, err := in.evalBool(candidate, tuple)
				if err != nil {
					return nil, err
				}
				if matched {
					return caseClause.Body, nil
				}
			}
		}
		return fallback, nil
	}
	tag, err := in.evalString(node.Tag, tuple)
	if err != nil {
		return nil, err
	}
	for _, clause := range node.Body.List {
		caseClause, ok := clause.(*ast.CaseClause)
		if !ok {
			return nil, &s7AVEvalError{detail: "unsupported switch clause"}
		}
		if caseClause.List == nil {
			fallback = caseClause.Body
			continue
		}
		for _, candidate := range caseClause.List {
			value, err := in.evalString(candidate, tuple)
			if err != nil {
				return nil, err
			}
			if value == tag {
				return caseClause.Body, nil
			}
		}
	}
	return fallback, nil
}

// classify interprets the shipped ClassifyIntentArchiveTuple body.
func (in *s7AVInterpreter) classify(tuple s7AVTuple) (s7AVClassification, error) {
	function := in.program.function("ClassifyIntentArchiveTuple")
	if function == nil {
		return s7AVClassification{}, &s7AVEvalError{detail: "ClassifyIntentArchiveTuple is missing"}
	}
	result := s7AVClassification{}
	returned, flow, err := in.runClassify(function.Body.List, tuple, &result)
	if err != nil {
		return s7AVClassification{}, err
	}
	if flow != s7AVFlowReturned {
		return s7AVClassification{}, &s7AVEvalError{detail: "ClassifyIntentArchiveTuple fell through without returning"}
	}
	return returned, nil
}

func (in *s7AVInterpreter) runClassify(
	statements []ast.Stmt,
	tuple s7AVTuple,
	result *s7AVClassification,
) (s7AVClassification, s7AVFlow, error) {
	for _, statement := range statements {
		switch typed := statement.(type) {
		case *ast.ReturnStmt:
			if len(typed.Results) != 1 {
				return s7AVClassification{}, s7AVFlowNormal, &s7AVEvalError{detail: "unsupported classifier return arity"}
			}
			switch value := typed.Results[0].(type) {
			case *ast.Ident:
				if value.Name != "result" {
					return s7AVClassification{}, s7AVFlowNormal, &s7AVEvalError{detail: "unsupported classifier return value"}
				}
				return *result, s7AVFlowReturned, nil
			case *ast.CompositeLit:
				literal, err := in.compositeClassification(value)
				if err != nil {
					return s7AVClassification{}, s7AVFlowNormal, err
				}
				return literal, s7AVFlowReturned, nil
			}
			return s7AVClassification{}, s7AVFlowNormal, &s7AVEvalError{detail: "unsupported classifier return expression"}
		case *ast.AssignStmt:
			if err := in.applyAssignment(typed, tuple, result); err != nil {
				return s7AVClassification{}, s7AVFlowNormal, err
			}
		case *ast.IfStmt:
			condition, err := in.evalBool(typed.Cond, tuple)
			if err != nil {
				return s7AVClassification{}, s7AVFlowNormal, err
			}
			var branch []ast.Stmt
			if condition {
				branch = typed.Body.List
			} else if typed.Else != nil {
				branch = s7AVElseStatements(typed.Else)
			}
			returned, flow, err := in.runClassify(branch, tuple, result)
			if err != nil || flow == s7AVFlowReturned {
				return returned, flow, err
			}
		case *ast.SwitchStmt:
			body, err := in.selectCase(typed, tuple)
			if err != nil {
				return s7AVClassification{}, s7AVFlowNormal, err
			}
			returned, flow, err := in.runClassify(body, tuple, result)
			if err != nil || flow == s7AVFlowReturned {
				return returned, flow, err
			}
		default:
			return s7AVClassification{}, s7AVFlowNormal, &s7AVEvalError{detail: "unsupported statement in the classifier"}
		}
	}
	return s7AVClassification{}, s7AVFlowNormal, nil
}

func (in *s7AVInterpreter) compositeClassification(literal *ast.CompositeLit) (s7AVClassification, error) {
	result := s7AVClassification{}
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			return result, &s7AVEvalError{detail: "unsupported unkeyed classifier literal"}
		}
		key, ok := pair.Key.(*ast.Ident)
		if !ok {
			return result, &s7AVEvalError{detail: "unsupported classifier literal key"}
		}
		if err := in.setField(&result, key.Name, pair.Value); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (in *s7AVInterpreter) applyAssignment(
	statement *ast.AssignStmt,
	tuple s7AVTuple,
	result *s7AVClassification,
) error {
	if len(statement.Lhs) != 1 || len(statement.Rhs) != 1 {
		return &s7AVEvalError{detail: "unsupported classifier assignment arity"}
	}
	switch target := statement.Lhs[0].(type) {
	case *ast.Ident:
		if target.Name != "result" || statement.Tok != token.DEFINE {
			return &s7AVEvalError{detail: "unsupported classifier assignment target"}
		}
		literal, ok := statement.Rhs[0].(*ast.CompositeLit)
		if !ok {
			return &s7AVEvalError{detail: "unsupported classifier initializer"}
		}
		value, err := in.compositeClassification(literal)
		if err != nil {
			return err
		}
		*result = value
		return nil
	case *ast.SelectorExpr:
		base, ok := target.X.(*ast.Ident)
		if !ok || base.Name != "result" || statement.Tok != token.ASSIGN {
			return &s7AVEvalError{detail: "unsupported classifier assignment target"}
		}
		_ = tuple
		return in.setField(result, target.Sel.Name, statement.Rhs[0])
	}
	return &s7AVEvalError{detail: "unsupported classifier assignment"}
}

func (in *s7AVInterpreter) setField(result *s7AVClassification, field string, value ast.Expr) error {
	switch field {
	case "Reachable":
		ident, ok := value.(*ast.Ident)
		if !ok || (ident.Name != "true" && ident.Name != "false") {
			return &s7AVEvalError{detail: "unsupported Reachable value"}
		}
		result.Reachable = ident.Name == "true"
		return nil
	case "ExitClass":
		literal, ok := value.(*ast.BasicLit)
		if !ok || literal.Kind != token.INT {
			return &s7AVEvalError{detail: "unsupported ExitClass value"}
		}
		parsed, err := strconv.Atoi(literal.Value)
		if err != nil {
			return &s7AVEvalError{detail: "unsupported ExitClass literal"}
		}
		result.ExitClass = parsed
		return nil
	case "Disposition", "Action", "Code", "RepairClass":
		ident, ok := value.(*ast.Ident)
		if !ok {
			return &s7AVEvalError{detail: "unsupported " + field + " value"}
		}
		decoded, ok := in.constants[ident.Name]
		if !ok {
			return &s7AVEvalError{detail: "unknown constant " + ident.Name}
		}
		switch field {
		case "Disposition":
			result.Disposition = decoded
		case "Action":
			result.Action = decoded
		case "Code":
			result.Code = decoded
		case "RepairClass":
			result.RepairClass = decoded
		}
		return nil
	}
	return &s7AVEvalError{detail: "unsupported classifier field " + field}
}

// ─── §9.3 table parsing ───────────────────────────────────────────────────────

type s7AVTableRow struct {
	ordinal int
	tuple   s7AVTuple
	action  string
}

const s7AVDispositionTableHeader = "| # | Wire state | Blob observation, ownership and liveness | Required next action |"

func s7AVParseDispositionTable(document string, vocab s7AVVocabulary) ([]s7AVTableRow, error) {
	start := strings.Index(document, s7AVDispositionTableHeader)
	if start < 0 {
		return nil, fmt.Errorf("the §9.3 disposition table header was not found")
	}
	lines := strings.Split(document[start:], "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[1], "|") {
		return nil, fmt.Errorf("the §9.3 disposition table has no alignment row")
	}
	rows := []s7AVTableRow{}
	for _, line := range lines[2:] {
		if !strings.HasPrefix(line, "|") {
			break
		}
		cells := s7AVSplitTableRow(line)
		if len(cells) != 4 {
			return nil, fmt.Errorf("row %q has %d cells, want 4", line, len(cells))
		}
		ordinal, err := strconv.Atoi(strings.TrimSpace(cells[0]))
		if err != nil {
			return nil, fmt.Errorf("row %q has a non-numeric ordinal", line)
		}
		tuple, err := s7AVParseTupleCells(cells[1], cells[2], vocab)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", ordinal, err)
		}
		rows = append(rows, s7AVTableRow{ordinal: ordinal, tuple: tuple, action: cells[3]})
	}
	return rows, nil
}

func s7AVSplitTableRow(line string) []string {
	trimmed := strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	parts := strings.Split(trimmed, "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

// s7AVParseTupleCells derives the row's 4-tuple from the two stated columns.
// Every phrase it accepts is the document's own vocabulary; an unrecognised
// phrase is an error rather than a default, so a reworded row fails loudly.
func s7AVParseTupleCells(wireCell, observationCell string, vocab s7AVVocabulary) (s7AVTuple, error) {
	tuple := s7AVTuple{}
	switch strings.TrimSpace(wireCell) {
	case vocab.retained:
		tuple.Wire = vocab.retained
	case vocab.removalPending:
		tuple.Wire = vocab.removalPending
	case vocab.tombstoned:
		tuple.Wire = vocab.tombstoned
	default:
		return tuple, fmt.Errorf("unknown wire state %q", wireCell)
	}
	plain := s7AVStripMarkdownEmphasis(observationCell)
	switch {
	case strings.Contains(plain, "present, regular and hash-correct"):
		tuple.Blob = vocab.presentCorrect
	case strings.Contains(plain, "present, non-regular or hash-wrong"):
		tuple.Blob = vocab.unidentifiable
	case strings.Contains(plain, "absent"):
		tuple.Blob = vocab.absent
	default:
		return tuple, fmt.Errorf("unknown blob observation %q", observationCell)
	}
	switch {
	case strings.Contains(plain, "not owned"):
		tuple.Owned = false
	case strings.Contains(plain, "owned"):
		tuple.Owned = true
	default:
		return tuple, fmt.Errorf("row states no ownership: %q", observationCell)
	}
	switch {
	case strings.Contains(plain, "unreferenced"):
		tuple.Live = false
	case strings.Contains(plain, "live"):
		tuple.Live = true
	default:
		return tuple, fmt.Errorf("row states no liveness: %q", observationCell)
	}
	if tuple.Owned && !tuple.Live {
		return tuple, fmt.Errorf("row states owned and unreferenced, which dependency (3) rules out")
	}
	return tuple, nil
}

func s7AVStripMarkdownEmphasis(value string) string {
	replacer := strings.NewReplacer("**", "", "`", "", "*", "")
	return replacer.Replace(value)
}

// s7AVRouteFamily names the single repair family a table row may offer. Two
// families in one row is two routes for one tuple, which §9.3 forbids.
type s7AVRouteFamily string

const (
	s7AVRouteNone     s7AVRouteFamily = "none"
	s7AVRouteOwner    s7AVRouteFamily = "owner"
	s7AVRouteDivergen s7AVRouteFamily = "owner-divergent"
	s7AVRouteOrphans  s7AVRouteFamily = "orphans"
	s7AVRouteBlob     s7AVRouteFamily = "blob"
	s7AVRouteCorrupt  s7AVRouteFamily = "corrupt"
)

// A route is *offered* only in its full shipped command form for the residue
// class; §9.3 row 17 names the bare `--orphans --yes` selector precisely to say
// it does **not** step around a corrupt object, and a negative quantifier is not
// an offer.
func s7AVRowFamilies(action string) []s7AVRouteFamily {
	plain := s7AVStripMarkdownEmphasis(action)
	families := []s7AVRouteFamily{}
	if strings.Contains(plain, "type-total removal") {
		families = append(families, s7AVRouteCorrupt)
	}
	if strings.Contains(plain, "tpatch feature intent-archive purge <slug> --orphans --yes") {
		families = append(families, s7AVRouteOrphans)
	}
	if strings.Contains(plain, "--blob <hash> --yes") {
		families = append(families, s7AVRouteBlob)
	}
	return families
}

// s7AVExpectedFamily maps an interpreted classification to the single repair
// family §9.3's row for that tuple must name.
func s7AVExpectedFamily(classification s7AVClassification) s7AVRouteFamily {
	switch classification.RepairClass {
	case "corrupt-object":
		return s7AVRouteCorrupt
	case "unreferenced-residue":
		return s7AVRouteOrphans
	case "dangling-reference", "mixed-reference":
		return s7AVRouteBlob
	}
	if classification.Action == "route-pending-owner" {
		return s7AVRouteOwner
	}
	return s7AVRouteNone
}

// ─── the per-hash purge machine, derived beside the classifier ────────────────

// The classifier answers "which repair class owns this tuple"; it deliberately
// assigns an owned tuple no code and no exit class, because the owner resolves
// it. §9.3 rows 6, 9 and 14 nonetheless promise a specific *route* for an owned
// unidentifiable blob — exit 6 `archive-purge-evidence-divergent` — and rows 2,
// 4, 7, 8, 12 and 13 promise the owner simply completes. Neither promise is
// derivable from the classifier, so the purge machine itself is the second
// authority these rows are checked against.
const (
	s7AVPurgeMachineFunction      = "executeIntentArchivePurgeHash"
	s7AVPurgeRecoveryFunction     = "RecoverPendingPurge"
	s7AVUnidentifiableOwnedError  = "intentArchiveUnidentifiablePurgeError"
	s7AVUnidentifiablePredicate   = "intentArchiveBlobKindUnidentifiable"
	s7AVPurgeErrorConstructor     = "intentArchiveError"
	s7AVPurgeRemovalCall          = "removeIntentArchiveBlob"
	s7AVPurgeTombstonePublishCall = "publishIntentArchiveIndex"
	s7AVDivergentCodeConstant     = "IntentArchiveCodePurgeEvidenceDivergent"
	s7AVCorruptCodeConstant       = "IntentArchiveCodeBlobCorrupt"
	s7AVRegularKindConstant       = "IntentArchiveBlobKindRegular"
)

// s7AVPurgeRoute is one (code, exit class) pair the machine produces. A
// completing path carries the empty code and exit class 0.
type s7AVPurgeRoute struct {
	code      string
	exitClass int
}

type s7AVErrorConstruction struct {
	constant  string
	exitClass int
}

// s7AVErrorConstructions collects every `intentArchiveError(<code>, …, <exit>)`
// built inside one node, with its code constant and numeric exit class.
func s7AVErrorConstructions(node ast.Node) []s7AVErrorConstruction {
	constructions := []s7AVErrorConstruction{}
	for _, call := range s7AVCalls(node, s7AVPurgeErrorConstructor) {
		if len(call.Args) < 3 {
			continue
		}
		constant, ok := call.Args[0].(*ast.Ident)
		if !ok {
			continue
		}
		literal, ok := call.Args[2].(*ast.BasicLit)
		if !ok || literal.Kind != token.INT {
			continue
		}
		exit, err := strconv.Atoi(literal.Value)
		if err != nil {
			continue
		}
		constructions = append(constructions, s7AVErrorConstruction{constant: constant.Name, exitClass: exit})
	}
	return constructions
}

// s7AVAssignsCommitted reports whether the node marks a constructed error as
// having been produced after the transaction's first mutation, which is what
// forbids an exit-3 zero-write classification for it.
func s7AVAssignsCommitted(node ast.Node) bool {
	committed := false
	ast.Inspect(node, func(current ast.Node) bool {
		assignment, ok := current.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
			return true
		}
		selector, ok := assignment.Lhs[0].(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Committed" {
			return true
		}
		if ident, ok := assignment.Rhs[0].(*ast.Ident); ok && ident.Name == "true" {
			committed = true
		}
		return true
	})
	return committed
}

// s7AVUnidentifiableBranches returns the `if` statements of one function whose
// condition tests the shipped unidentifiable-kind predicate.
func s7AVUnidentifiableBranches(function *ast.FuncDecl) []*ast.IfStmt {
	branches := []*ast.IfStmt{}
	ast.Inspect(function.Body, func(node ast.Node) bool {
		branch, ok := node.(*ast.IfStmt)
		if !ok {
			return true
		}
		if len(s7AVCalls(branch.Cond, s7AVUnidentifiablePredicate)) != 0 {
			branches = append(branches, branch)
		}
		return true
	})
	return branches
}

// s7AVDeriveOwnedUnidentifiableRoute derives, from the shipped machine rather
// than from §9.3's prose, the single route an owned blob that is present but
// unidentifiable takes: the per-hash machine's evidence check, the owned
// recovery's preflight and the shared owned/not-owned error constructor must
// all agree on one code and one exit class, and the not-owned arm of that
// constructor must stay a distinct exit-3 classification.
func s7AVDeriveOwnedUnidentifiableRoute(
	program *s7AVProgram,
	constants map[string]string,
) (s7AVPurgeRoute, error) {
	route := s7AVPurgeRoute{}
	for _, name := range []string{s7AVPurgeMachineFunction, s7AVPurgeRecoveryFunction} {
		function := program.function(name)
		if function == nil {
			return route, fmt.Errorf("the purge machine function %s is missing", name)
		}
		branches := s7AVUnidentifiableBranches(function)
		if len(branches) != 1 {
			return route, fmt.Errorf(
				"%s tests the unidentifiable predicate in %d branches, want exactly 1", name, len(branches),
			)
		}
		branch := branches[0]
		if !s7AVTerminates(branch.Body.List) {
			return route, fmt.Errorf("%s continues past its owned unidentifiable evidence check", name)
		}
		constructions := s7AVErrorConstructions(branch.Body)
		if len(constructions) != 1 {
			return route, fmt.Errorf(
				"%s builds %d errors for an owned unidentifiable blob, want exactly 1", name, len(constructions),
			)
		}
		if constructions[0].constant != s7AVDivergentCodeConstant {
			return route, fmt.Errorf(
				"%s routes an owned unidentifiable blob to %s, want %s",
				name, constructions[0].constant, s7AVDivergentCodeConstant,
			)
		}
		if !s7AVAssignsCommitted(branch.Body) {
			return route, fmt.Errorf("%s does not mark the owned unidentifiable refusal as post-mutation", name)
		}
		derived := s7AVPurgeRoute{
			code:      constants[constructions[0].constant],
			exitClass: constructions[0].exitClass,
		}
		if (route != s7AVPurgeRoute{}) && route != derived {
			return route, fmt.Errorf(
				"the purge machine disagrees with itself on the owned unidentifiable route: %+v then %+v",
				route, derived,
			)
		}
		route = derived
	}

	shared := program.function(s7AVUnidentifiableOwnedError)
	if shared == nil {
		return route, fmt.Errorf("the owned/not-owned unidentifiable constructor is missing")
	}
	sharedConstructions := s7AVErrorConstructions(shared.Body)
	if len(sharedConstructions) != 2 {
		return route, fmt.Errorf(
			"%s builds %d errors, want the owned and the not-owned arm",
			s7AVUnidentifiableOwnedError, len(sharedConstructions),
		)
	}
	owned, notOwned := sharedConstructions[0], sharedConstructions[1]
	if owned.constant != s7AVDivergentCodeConstant || owned.exitClass != route.exitClass {
		return route, fmt.Errorf(
			"%s routes the owned arm to %s exit %d, want %s exit %d",
			s7AVUnidentifiableOwnedError, owned.constant, owned.exitClass,
			s7AVDivergentCodeConstant, route.exitClass,
		)
	}
	if notOwned.constant != s7AVCorruptCodeConstant || notOwned.exitClass == route.exitClass {
		return route, fmt.Errorf(
			"%s routes the not-owned arm to %s exit %d, want the distinct %s classification",
			s7AVUnidentifiableOwnedError, notOwned.constant, notOwned.exitClass, s7AVCorruptCodeConstant,
		)
	}
	if route.code == "" || route.exitClass == 0 {
		return route, fmt.Errorf("the owned unidentifiable route derived to %+v", route)
	}
	return route, nil
}

// s7AVDeriveOwnedCompletionRoute derives the other half of the rev-12 split:
// with a present, regular, hash-correct blob the same owned machine removes the
// blob, publishes the tombstone and returns no error at all, which is the exit-0
// completion §9.3 row 13 promises.
func s7AVDeriveOwnedCompletionRoute(program *s7AVProgram) (s7AVPurgeRoute, error) {
	route := s7AVPurgeRoute{}
	function := program.function(s7AVPurgeMachineFunction)
	if function == nil {
		return route, fmt.Errorf("the purge machine function %s is missing", s7AVPurgeMachineFunction)
	}
	branches := []*ast.IfStmt{}
	ast.Inspect(function.Body, func(node ast.Node) bool {
		branch, ok := node.(*ast.IfStmt)
		if !ok {
			return true
		}
		comparison, ok := branch.Cond.(*ast.BinaryExpr)
		if !ok || comparison.Op != token.EQL {
			return true
		}
		selector, ok := comparison.X.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Kind" {
			return true
		}
		kind, ok := comparison.Y.(*ast.Ident)
		if !ok || kind.Name != s7AVRegularKindConstant {
			return true
		}
		if len(s7AVCalls(branch.Body, s7AVPurgeRemovalCall)) == 0 {
			return true
		}
		branches = append(branches, branch)
		return true
	})
	if len(branches) != 1 {
		return route, fmt.Errorf(
			"%s has %d hash-correct branches that reach %s, want exactly 1",
			s7AVPurgeMachineFunction, len(branches), s7AVPurgeRemovalCall,
		)
	}
	regular := branches[0]
	for _, construction := range s7AVErrorConstructions(regular.Body) {
		if construction.constant == s7AVDivergentCodeConstant {
			return route, fmt.Errorf(
				"%s routes an owned hash-correct blob to %s, which is the unidentifiable route",
				s7AVPurgeMachineFunction, s7AVDivergentCodeConstant,
			)
		}
	}
	tombstoned := false
	for _, publish := range s7AVCalls(function.Body, s7AVPurgeTombstonePublishCall) {
		if publish.Pos() > regular.End() {
			tombstoned = true
		}
	}
	if !tombstoned {
		return route, fmt.Errorf(
			"%s never publishes the tombstone after removing the owned hash-correct blob",
			s7AVPurgeMachineFunction,
		)
	}
	body := function.Body.List
	final, ok := body[len(body)-1].(*ast.ReturnStmt)
	if !ok || len(final.Results) != 2 {
		return route, fmt.Errorf("%s does not end in a two-result return", s7AVPurgeMachineFunction)
	}
	nilResult, ok := final.Results[1].(*ast.Ident)
	if !ok || nilResult.Name != "nil" {
		return route, fmt.Errorf(
			"%s does not complete the owned hash-correct path without an error", s7AVPurgeMachineFunction,
		)
	}
	return route, nil
}

// s7AVStatedExitPattern matches the shipped "exit N" form only. The negative
// "No exit-3 code is reachable" form is a denial rather than a stated route and
// is deliberately not matched.
var s7AVStatedExitPattern = regexp.MustCompile(`exit ([0-9])`)

// s7AVStatedExitClasses returns the numeric exit classes a table row states.
func s7AVStatedExitClasses(action string) []int {
	classes := []int{}
	for _, match := range s7AVStatedExitPattern.FindAllStringSubmatch(action, -1) {
		value, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		classes = append(classes, value)
	}
	return classes
}

// s7AVValidateDispositionTotality is the single combined validator: the derived
// domain and its arithmetic, the interpreted classifier, the shipped table's
// totality and one-route-per-tuple property, and the two rev-12 splits.
func s7AVValidateDispositionTotality(sources map[string]string, document string) error {
	program, err := s7AVParseProgram(sources, []string{s7AVStoreArchiveSource})
	if err != nil {
		return err
	}
	interpreter, err := s7AVNewInterpreter(program)
	if err != nil {
		return err
	}
	vocab, err := s7AVDeriveVocabulary(interpreter.constants)
	if err != nil {
		return err
	}

	wireValues := []string{}
	for _, name := range program.typedConstantNames("IntentArchiveWireState") {
		wireValues = append(wireValues, interpreter.wire[name])
	}
	blobValues := []string{}
	for _, name := range program.typedConstantNames("IntentArchiveBlobState") {
		blobValues = append(blobValues, interpreter.blob[name])
	}
	if len(wireValues) != 3 || len(blobValues) != 3 {
		return fmt.Errorf("domain cardinality = %d wire × %d blob, want 3 × 3", len(wireValues), len(blobValues))
	}

	domain := []s7AVTuple{}
	for _, wire := range wireValues {
		for _, blob := range blobValues {
			for _, owned := range []bool{false, true} {
				for _, live := range []bool{false, true} {
					domain = append(domain, s7AVTuple{Wire: wire, Blob: blob, Owned: owned, Live: live})
				}
			}
		}
	}
	if len(domain) != 36 {
		return fmt.Errorf("the Cartesian product has %d tuples, want 36", len(domain))
	}

	retainedUnreferenced, pendingOther, tombstonedOwnedUnreferenced := 0, 0, 0
	reachable := map[string]s7AVTuple{}
	for _, tuple := range domain {
		derived := s7AVDependencyReachable(tuple, vocab)
		shipped, err := interpreter.reachable(tuple)
		if err != nil {
			return fmt.Errorf("reachability interpretation for %s: %w", tuple, err)
		}
		if derived != shipped {
			return fmt.Errorf(
				"reachability disagreement for %s: dependencies say %t, the shipped predicate says %t",
				tuple, derived, shipped,
			)
		}
		if derived {
			reachable[tuple.String()] = tuple
			continue
		}
		switch {
		case tuple.Wire == vocab.retained && !tuple.Live:
			retainedUnreferenced++
		case tuple.Wire == vocab.removalPending:
			pendingOther++
		case tuple.Wire == vocab.tombstoned && tuple.Owned && !tuple.Live:
			tombstonedOwnedUnreferenced++
		default:
			return fmt.Errorf("tuple %s is unreachable for no stated dependency", tuple)
		}
	}
	if retainedUnreferenced != 6 || pendingOther != 9 || tombstonedOwnedUnreferenced != 3 {
		return fmt.Errorf(
			"ruled-out partition = %d retained×unreferenced / %d removal-pending / %d tombstoned×owned×unreferenced, want 6/9/3",
			retainedUnreferenced, pendingOther, tombstonedOwnedUnreferenced,
		)
	}
	if len(reachable) != 18 {
		return fmt.Errorf("reachable tuples = %d, want 18", len(reachable))
	}

	// The purge machine is derived beside the classifier, because the
	// classifier deliberately assigns an owned tuple no code and no exit class
	// while §9.3's owned rows promise two different owner outcomes.
	divergentRoute, err := s7AVDeriveOwnedUnidentifiableRoute(program, interpreter.constants)
	if err != nil {
		return err
	}
	completionRoute, err := s7AVDeriveOwnedCompletionRoute(program)
	if err != nil {
		return err
	}
	if divergentRoute.exitClass == completionRoute.exitClass {
		return fmt.Errorf(
			"the purge machine gives an owned hash-correct and an owned unidentifiable blob the same exit class %d",
			divergentRoute.exitClass,
		)
	}

	// Ownership outranks every other observation: no owned tuple carries a
	// repair class, a code or an exit class of its own.
	for _, tuple := range reachable {
		classification, err := interpreter.classify(tuple)
		if err != nil {
			return fmt.Errorf("classification of %s: %w", tuple, err)
		}
		if !classification.Reachable {
			return fmt.Errorf("the classifier calls reachable tuple %s unreachable", tuple)
		}
		if tuple.Owned &&
			(classification.RepairClass != "" || classification.Code != "" || classification.ExitClass != 0 ||
				classification.Action != "route-pending-owner") {
			return fmt.Errorf(
				"owned tuple %s is not routed to its transaction: class=%q code=%q exit=%d action=%q",
				tuple, classification.RepairClass, classification.Code,
				classification.ExitClass, classification.Action,
			)
		}
	}

	rows, err := s7AVParseDispositionTable(document, vocab)
	if err != nil {
		return err
	}
	if len(rows) != 18 {
		return fmt.Errorf("the shipped §9.3 table has %d rows, want exactly 18", len(rows))
	}
	seen := map[string]int{}
	byTuple := map[string]s7AVTableRow{}
	for index, row := range rows {
		if row.ordinal != index+1 {
			return fmt.Errorf("row %d carries ordinal %d; ordinals must be contiguous from 1", index+1, row.ordinal)
		}
		key := row.tuple.String()
		if previous, duplicate := seen[key]; duplicate {
			return fmt.Errorf("rows %d and %d state the same tuple %s", previous, row.ordinal, key)
		}
		if _, ok := reachable[key]; !ok {
			return fmt.Errorf("row %d states unreachable tuple %s", row.ordinal, key)
		}
		seen[key] = row.ordinal
		byTuple[key] = row
	}
	for key, tuple := range reachable {
		row, ok := byTuple[key]
		if !ok {
			return fmt.Errorf("reachable tuple %s has no row", tuple)
		}
		classification, err := interpreter.classify(tuple)
		if err != nil {
			return fmt.Errorf("classification of %s: %w", tuple, err)
		}
		if tuple.Owned {
			if err := s7AVValidateOwnedRowRoute(
				row, tuple, vocab, divergentRoute, completionRoute,
			); err != nil {
				return err
			}
		}
		want := s7AVExpectedFamily(classification)
		families := s7AVRowFamilies(row.action)
		switch want {
		case s7AVRouteNone, s7AVRouteOwner, s7AVRouteDivergen:
			if len(families) != 0 {
				return fmt.Errorf(
					"row %d (%s) names repair route(s) %v while the classifier routes it to %q",
					row.ordinal, tuple, families, want,
				)
			}
		case s7AVRouteCorrupt:
			// The corrupt class's row names its manual prerequisite and may
			// name the dangling follow-up the prerequisite produces; it must
			// never also offer `--orphans --yes`, which would be a second
			// route for one tuple.
			if len(families) == 0 || families[0] != s7AVRouteCorrupt {
				return fmt.Errorf("row %d (%s) does not name the type-total removal route", row.ordinal, tuple)
			}
			for _, family := range families[1:] {
				if family != s7AVRouteBlob {
					return fmt.Errorf("row %d (%s) offers a second route %q beside the corrupt procedure", row.ordinal, tuple, family)
				}
			}
		default:
			if len(families) != 1 || families[0] != want {
				return fmt.Errorf(
					"row %d (%s) names routes %v, want exactly [%s]",
					row.ordinal, tuple, families, want,
				)
			}
		}
	}

	// The two rev-12 splits are asserted directly rather than inferred from the
	// counts above.
	splits := []struct {
		label   string
		tuple   s7AVTuple
		want    s7AVRouteFamily
		machine *s7AVPurgeRoute
	}{
		{
			label: "row 10 tombstoned × not-owned × absent × unreferenced",
			tuple: s7AVTuple{Wire: vocab.tombstoned, Blob: vocab.absent, Owned: false, Live: false},
			want:  s7AVRouteNone,
		},
		{
			label: "row 11 tombstoned × not-owned × absent × live",
			tuple: s7AVTuple{Wire: vocab.tombstoned, Blob: vocab.absent, Owned: false, Live: true},
			want:  s7AVRouteBlob,
		},
		{
			label:   "row 13 tombstoned × owned × present-regular-hash-correct",
			tuple:   s7AVTuple{Wire: vocab.tombstoned, Blob: vocab.presentCorrect, Owned: true, Live: true},
			want:    s7AVRouteOwner,
			machine: &completionRoute,
		},
		{
			label:   "row 14 tombstoned × owned × present-unidentifiable",
			tuple:   s7AVTuple{Wire: vocab.tombstoned, Blob: vocab.unidentifiable, Owned: true, Live: true},
			want:    s7AVRouteOwner,
			machine: &divergentRoute,
		},
	}
	splitOrdinals := map[string]int{}
	for _, split := range splits {
		row, ok := byTuple[split.tuple.String()]
		if !ok {
			return fmt.Errorf("the split tuple %s has no row (%s)", split.tuple, split.label)
		}
		classification, err := interpreter.classify(split.tuple)
		if err != nil {
			return err
		}
		if got := s7AVExpectedFamily(classification); got != split.want {
			return fmt.Errorf("%s routes to %q, want %q", split.label, got, split.want)
		}
		if split.machine != nil {
			if err := s7AVValidateOwnedRowRoute(
				row, split.tuple, vocab, divergentRoute, completionRoute,
			); err != nil {
				return fmt.Errorf("%s: %w", split.label, err)
			}
			stated := s7AVStatedExitClasses(s7AVStripMarkdownEmphasis(row.action))
			switch {
			case split.machine.exitClass == 0 && len(stated) != 0:
				return fmt.Errorf(
					"%s states exit class(es) %v while the purge machine completes it under the owner",
					split.label, stated,
				)
			case split.machine.exitClass != 0 &&
				(len(stated) != 1 || stated[0] != split.machine.exitClass):
				return fmt.Errorf(
					"%s states exit class(es) %v, want exactly the machine's exit %d",
					split.label, stated, split.machine.exitClass,
				)
			}
		}
		if previous, duplicate := splitOrdinals[split.tuple.String()]; duplicate {
			return fmt.Errorf("%s collapsed into row %d", split.label, previous)
		}
		splitOrdinals[split.tuple.String()] = row.ordinal
	}
	if len(splitOrdinals) != 4 {
		return fmt.Errorf("the two rev-12 splits resolve to %d rows, want 4", len(splitOrdinals))
	}
	return nil
}

// s7AVValidateOwnedRowRoute holds every owned row to the purge machine's own
// outcome for its blob observation: an unidentifiable blob must state the
// derived divergent code and exactly the derived exit class, and every other
// owned observation must state the owner's completion with no exit class at all.
func s7AVValidateOwnedRowRoute(
	row s7AVTableRow,
	tuple s7AVTuple,
	vocab s7AVVocabulary,
	divergent, completion s7AVPurgeRoute,
) error {
	plain := s7AVStripMarkdownEmphasis(row.action)
	stated := s7AVStatedExitClasses(plain)
	if tuple.Blob == vocab.unidentifiable {
		if !strings.Contains(plain, divergent.code) {
			return fmt.Errorf(
				"row %d (%s) does not name the purge machine's owned-unidentifiable code %q",
				row.ordinal, tuple, divergent.code,
			)
		}
		if len(stated) != 1 || stated[0] != divergent.exitClass {
			return fmt.Errorf(
				"row %d (%s) states exit class(es) %v, want exactly the purge machine's exit %d",
				row.ordinal, tuple, stated, divergent.exitClass,
			)
		}
		return nil
	}
	if strings.Contains(plain, divergent.code) {
		return fmt.Errorf(
			"row %d (%s) names the owned-unidentifiable code %q for an identifiable observation",
			row.ordinal, tuple, divergent.code,
		)
	}
	if len(stated) != 0 || completion.exitClass != 0 {
		return fmt.Errorf(
			"row %d (%s) states exit class(es) %v while the purge machine completes it under the owner at exit %d",
			row.ordinal, tuple, stated, completion.exitClass,
		)
	}
	return nil
}

func TestS7AVDispositionTableTotalityGuard(t *testing.T) {
	sources := s7AVRepoSources(t, s7AVStoreArchiveSource)
	document := s7AVRepoDocument(t, s7AVPRDRelPath)

	if err := s7AVValidateDispositionTotality(sources, document); err != nil {
		t.Fatalf("PIB-551 baseline validation failed: %v", err)
	}

	// The interpreter is only load-bearing if it agrees with the shipped
	// functions on the whole domain, so its faithfulness is proved against the
	// live build before any fixture is judged by it.
	program, err := s7AVParseProgram(sources, []string{s7AVStoreArchiveSource})
	if err != nil {
		t.Fatal(err)
	}
	interpreter, err := s7AVNewInterpreter(program)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, wire := range []store.IntentArchiveWireState{
		store.IntentArchiveWireRetained,
		store.IntentArchiveWireRemovalPending,
		store.IntentArchiveWireTombstoned,
	} {
		for _, blob := range []store.IntentArchiveBlobState{
			store.IntentArchiveBlobAbsent,
			store.IntentArchiveBlobPresentCorrect,
			store.IntentArchiveBlobUnidentifiable,
		} {
			for _, owned := range []bool{false, true} {
				for _, live := range []bool{false, true} {
					checked++
					shipped := store.ClassifyIntentArchiveTuple(store.IntentArchiveTuple{
						WireState: wire, BlobState: blob, Owned: owned, Live: live,
					})
					interpreted, err := interpreter.classify(s7AVTuple{
						Wire: string(wire), Blob: string(blob), Owned: owned, Live: live,
					})
					if err != nil {
						t.Fatalf("PIB-551 interpreter failed on %s/%s/%t/%t: %v", wire, blob, owned, live, err)
					}
					if interpreted.Reachable != shipped.Reachable ||
						interpreted.Disposition != string(shipped.Disposition) ||
						interpreted.Action != string(shipped.Action) ||
						interpreted.Code != string(shipped.Code) ||
						interpreted.RepairClass != string(shipped.RepairClass) ||
						interpreted.ExitClass != shipped.ExitClass {
						t.Fatalf(
							"PIB-551 interpreter diverged from the live classifier on %s/%s/%t/%t:\n interpreted=%+v\n shipped=%+v",
							wire, blob, owned, live, interpreted, shipped,
						)
					}
				}
			}
		}
	}
	if checked != 36 {
		t.Fatalf("PIB-551 checked %d tuples, want 36", checked)
	}

	fixtures := []struct {
		name      string
		source    string
		mutateDoc func(string) string
		old       string
		new       string
		count     int
		wantClass string
	}{
		{
			name:   "owned-dangling-tuple-dropped",
			source: s7AVStoreArchiveSource,
			old: "		case IntentArchiveBlobAbsent:\n" +
				"			result.Disposition = IntentArchiveDispositionPendingFinalize\n" +
				"			result.Action = IntentArchiveActionRoutePendingOwner\n" +
				"		}\n" +
				"		return result\n" +
				"	}\n",
			new: "		case IntentArchiveBlobAbsent:\n" +
				"			result.Disposition = IntentArchiveDispositionDanglingReference\n" +
				"			result.Action = IntentArchiveActionPurgeHash\n" +
				"			result.Code = IntentArchiveCodeBlobDangling\n" +
				"			result.RepairClass = IntentArchiveRepairDanglingReference\n" +
				"			result.ExitClass = 3\n" +
				"		}\n" +
				"		return result\n" +
				"	}\n",
			count:     1,
			wantClass: "is not routed to its transaction",
		},
		{
			name: "second-route-on-tombstoned-unidentifiable",
			mutateDoc: func(document string) string {
				return strings.Replace(
					document,
					"| 17 | tombstoned | present, **non-regular or hash-wrong**; `h` **not owned** and **unreferenced** | not an orphan and not residue:",
					"| 17 | tombstoned | present, **non-regular or hash-wrong**; `h` **not owned** and **unreferenced** | "+
						"either `tpatch feature intent-archive purge <slug> --orphans --yes` or the corrupt procedure clears it; not an orphan and not residue:",
					1,
				)
			},
			wantClass: "offers a second route",
		},
		{
			name: "rows-13-and-14-recollapsed",
			mutateDoc: func(document string) string {
				collapsed := "| 13 | tombstoned | present, regular and hash-correct; `h` **owned** (live) | rev-11's single collapsed row: the transaction in flight owns it whether or not the object is identifiable, and the observation is merely swept into the recovery's global claim and tombstoned again at the end (§9.7.2) |\n"
				start := strings.Index(document, "| 13 | tombstoned | present, regular and hash-correct; `h` **owned** (live) |")
				if start < 0 {
					return document
				}
				end := strings.Index(document[start:], "| 15 | tombstoned |")
				if end < 0 {
					return document
				}
				return document[:start] + collapsed + document[start+end:]
			},
			wantClass: "has 17 rows",
		},
		{
			// The collapse fixture above is caught by the row count. This one
			// keeps all eighteen rows and changes only row 14's action to row
			// 13's owner-sweep wording, so the only thing that can reject it is
			// the exit-6 route derived from the purge machine.
			name: "row-14-states-row-13-owner-completion",
			mutateDoc: func(document string) string {
				anchor := "| 14 | tombstoned | present, **non-regular or hash-wrong**; `h` **owned** (live) | "
				start := strings.Index(document, anchor)
				if start < 0 {
					return document
				}
				end := strings.Index(document[start:], "\n")
				if end < 0 {
					return document
				}
				return document[:start] + anchor +
					"the transaction in flight owns it; the recovery's global claim sweeps this reference into " +
					"removal-pending and tombstones it again at the end. Every other command routes to the owner " +
					"with `recovery-pending` (§9.7.2) |" + document[start+end:]
			},
			wantClass: "does not name the purge machine's owned-unidentifiable code",
		},
		{
			// The converse: row 13's identifiable observation may not borrow
			// the divergent route the machine reserves for row 14.
			name: "row-13-claims-the-divergent-exit",
			mutateDoc: func(document string) string {
				return strings.Replace(
					document,
					"| 13 | tombstoned | present, regular and hash-correct; `h` **owned** (live) | the transaction in flight owns it;",
					"| 13 | tombstoned | present, regular and hash-correct; `h` **owned** (live) | `purge --yes` refuses "+
						"**exit 6** `archive-purge-evidence-divergent`;",
					1,
				)
			},
			wantClass: "names the owned-unidentifiable code",
		},
		{
			// The route is derived, not read: demoting the shared owned arm to
			// the not-owned exit-3 classification must fail even though every
			// document row is untouched.
			name:   "owned-unidentifiable-demoted-to-exit-3",
			source: s7AVStoreArchiveSource,
			old: "\tif owned {\n" +
				"\t\terr := intentArchiveError(IntentArchiveCodePurgeEvidenceDivergent, \"the owned blob is present but unidentifiable\", 6)\n",
			new: "\tif owned {\n" +
				"\t\terr := intentArchiveError(IntentArchiveCodeBlobCorrupt, \"the owned blob is present but unidentifiable\", 3)\n",
			count:     1,
			wantClass: "routes the owned arm to",
		},
		{
			// The same derivation must reject a machine that disagrees with
			// its own owned recovery preflight about the exit class.
			name:   "purge-machine-evidence-check-demoted",
			source: s7AVStoreArchiveSource,
			old: "\t\tdivergent := intentArchiveError(IntentArchiveCodePurgeEvidenceDivergent, " +
				"\"the owned blob is no longer identifiable\", 6)\n",
			new: "\t\tdivergent := intentArchiveError(IntentArchiveCodePurgeEvidenceDivergent, " +
				"\"the owned blob is no longer identifiable\", 3)\n",
			count:     1,
			wantClass: "disagrees with itself on the owned unidentifiable route",
		},
		{
			name: "row-count-disagrees-with-domain",
			mutateDoc: func(document string) string {
				anchor := "| 16 | tombstoned | present, regular and hash-correct; `h` **not owned** but **live**"
				start := strings.Index(document, anchor)
				if start < 0 {
					return document
				}
				end := strings.Index(document[start:], "\n")
				if end < 0 {
					return document
				}
				row := document[start : start+end+1]
				return document[:start+end+1] + row + document[start+end+1:]
			},
			wantClass: "has 19 rows",
		},
	}

	for _, fixture := range fixtures {
		mutatedSources := sources
		mutatedDocument := document
		if fixture.source != "" {
			mutatedSources = s7AVMutate(t, sources, fixture.source, fixture.old, fixture.new, fixture.count)
		}
		if fixture.mutateDoc != nil {
			mutatedDocument = fixture.mutateDoc(document)
			if mutatedDocument == document {
				t.Fatalf("PIB-551 sensitivity fixture %q changed nothing", fixture.name)
			}
		}
		err := s7AVValidateDispositionTotality(mutatedSources, mutatedDocument)
		if err == nil {
			t.Fatalf("PIB-551 sensitivity fixture %q was accepted by the totality validator", fixture.name)
		}
		if !strings.Contains(err.Error(), fixture.wantClass) {
			t.Fatalf("PIB-551 sensitivity fixture %q: want error class %q, got: %v",
				fixture.name, fixture.wantClass, err)
		}
	}

	if err := s7AVValidateDispositionTotality(sources, document); err != nil {
		t.Fatalf("PIB-551 unmutated totality was rejected after sensitivity: %v", err)
	}
}

// ─── PIB-549 ──────────────────────────────────────────────────────────────────

// s7AVSelectorArms names the confirmed selector kinds §9.3.1's admission table
// decides over. The guard derives each arm's admitted class from the shipped
// switch rather than from the prose.
var s7AVSelectorArms = map[string][]string{
	"IntentArchiveSelectorOrphans":    {"IntentArchiveRepairUnreferencedResidue"},
	"IntentArchiveSelectorBlob":       {"IntentArchiveRepairDanglingReference", "IntentArchiveRepairMixedReference"},
	"IntentArchiveSelectorGeneration": {},
	"IntentArchiveSelectorAll":        nil,
}

func s7AVCallName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		return fun.Sel.Name
	case *ast.Ident:
		return fun.Name
	}
	return ""
}

func s7AVCalls(node ast.Node, name string) []*ast.CallExpr {
	found := []*ast.CallExpr{}
	ast.Inspect(node, func(current ast.Node) bool {
		if call, ok := current.(*ast.CallExpr); ok && s7AVCallName(call) == name {
			found = append(found, call)
		}
		return true
	})
	return found
}

func s7AVIdentNames(node ast.Node) map[string]int {
	names := map[string]int{}
	ast.Inspect(node, func(current ast.Node) bool {
		if ident, ok := current.(*ast.Ident); ok {
			names[ident.Name]++
		}
		return true
	})
	return names
}

// s7AVValidateAdmissionPredicate derives §9.3.1's admission predicate from the
// implementation: the corrupt-first precondition, the four conjunctive
// conditions over one chosen class, and the per-selector admitted class map.
func s7AVValidateAdmissionPredicate(sources map[string]string) error {
	program, err := s7AVParseProgram(sources, []string{s7AVStoreArchiveSource})
	if err != nil {
		return err
	}
	admit := program.function("admitIntentArchiveRepair")
	if admit == nil {
		return fmt.Errorf("admitIntentArchiveRepair is missing")
	}

	// (0) The corrupt-first precondition: a rank-1 corrupt instance blocks every
	// confirmed selector, and the check is unconditional on the class set's size
	// and dominates every admitting statement.
	corruptGuard := (*ast.IfStmt)(nil)
	for _, statement := range admit.Body.List {
		branch, ok := statement.(*ast.IfStmt)
		if !ok {
			continue
		}
		if len(s7AVCalls(branch.Cond, "")) != 0 {
			continue
		}
		names := s7AVIdentNames(branch.Cond)
		if names["IntentArchiveRepairCorruptObject"] == 0 {
			continue
		}
		corruptGuard = branch
		break
	}
	if corruptGuard == nil {
		return fmt.Errorf("the corrupt-first precondition is missing from admitIntentArchiveRepair")
	}
	guardNames := s7AVIdentNames(corruptGuard.Cond)
	if guardNames["len"] != 0 || guardNames["Classes"] != 1 {
		return fmt.Errorf(
			"the corrupt-first precondition is conditioned on the observed class set rather than on the presence of a corrupt instance",
		)
	}
	if !s7AVTerminates(corruptGuard.Body.List) {
		return fmt.Errorf("the corrupt-first precondition does not refuse: its body does not return")
	}
	if corruptGuard.Else != nil {
		return fmt.Errorf("the corrupt-first precondition carries an else branch that can fall past it")
	}

	// (1) No sole-class gate survives: rev-11's rule refused an archive holding
	// one residue and one mixed hash, which the sequential admission withdrew.
	// Comparing the observed class set against **zero** is the "nothing is
	// wrong" early exit and is not a gate; comparing it against any other
	// cardinality is.
	for _, statement := range admit.Body.List {
		branch, ok := statement.(*ast.IfStmt)
		if !ok || branch == corruptGuard {
			continue
		}
		names := s7AVIdentNames(branch.Cond)
		if names["Classes"] == 0 || names["len"] == 0 {
			continue
		}
		binary, ok := branch.Cond.(*ast.BinaryExpr)
		if !ok {
			return fmt.Errorf("a sole-class gate on the observed class set precedes the per-selector arms")
		}
		literal, ok := binary.Y.(*ast.BasicLit)
		if !ok || literal.Value != "0" {
			return fmt.Errorf("a sole-class gate on the observed class set precedes the per-selector arms")
		}
	}

	// (2) Every selector arm admits exactly the classes §9.3.1 assigns it, and
	// each admission is total over the chosen class's instances.
	switches := []*ast.SwitchStmt{}
	ast.Inspect(admit.Body, func(node ast.Node) bool {
		if node, ok := node.(*ast.SwitchStmt); ok && node.Tag != nil {
			switches = append(switches, node)
		}
		return true
	})
	if len(switches) != 1 {
		return fmt.Errorf("admitIntentArchiveRepair has %d selector switches, want exactly 1", len(switches))
	}
	seenArms := map[string]bool{}
	for _, clause := range switches[0].Body.List {
		caseClause, ok := clause.(*ast.CaseClause)
		if !ok {
			return fmt.Errorf("the selector switch carries an unsupported clause")
		}
		if caseClause.List == nil {
			return fmt.Errorf("the selector switch carries a default clause, so its domain is not closed")
		}
		for _, candidate := range caseClause.List {
			ident, ok := candidate.(*ast.Ident)
			if !ok {
				return fmt.Errorf("the selector switch matches a non-constant case")
			}
			want, known := s7AVSelectorArms[ident.Name]
			if !known {
				return fmt.Errorf("unknown selector arm %q", ident.Name)
			}
			seenArms[ident.Name] = true
			names := s7AVIdentNames(caseClause)
			admitted := []string{}
			for _, class := range []string{
				"IntentArchiveRepairCorruptObject",
				"IntentArchiveRepairDanglingReference",
				"IntentArchiveRepairMixedReference",
				"IntentArchiveRepairUnreferencedResidue",
			} {
				if names[class] != 0 {
					admitted = append(admitted, class)
				}
			}
			if want != nil {
				if strings.Join(admitted, ",") != strings.Join(want, ",") {
					return fmt.Errorf(
						"selector arm %s admits %v, want %v",
						ident.Name, admitted, want,
					)
				}
			}
			switch ident.Name {
			case "IntentArchiveSelectorOrphans", "IntentArchiveSelectorBlob":
				if names["equalStringSets"] == 0 {
					return fmt.Errorf(
						"selector arm %s does not require total class coverage: no set equality over the class hashes",
						ident.Name,
					)
				}
				if names["class"]+names["report"] == 0 || names["Hashes"] == 0 {
					return fmt.Errorf("selector arm %s does not compare the selection against a class report", ident.Name)
				}
				if names["len"] != 0 {
					return fmt.Errorf(
						"selector arm %s conditions admission on an instance count rather than on total class coverage",
						ident.Name,
					)
				}
			case "IntentArchiveSelectorAll":
				if names["len"] == 0 || names["Classes"] == 0 {
					return fmt.Errorf("the --all arm is not restricted to a sole observed class")
				}
				binary := (*ast.BinaryExpr)(nil)
				ast.Inspect(caseClause, func(node ast.Node) bool {
					if node, ok := node.(*ast.BinaryExpr); ok && binary == nil {
						binary = node
					}
					return true
				})
				if binary == nil || binary.Op != token.EQL {
					return fmt.Errorf("the --all arm admits a class set that is not exactly one class")
				}
				literal, ok := binary.Y.(*ast.BasicLit)
				if !ok || literal.Value != "1" {
					return fmt.Errorf("the --all arm admits a class set that is not exactly one class")
				}
			case "IntentArchiveSelectorGeneration":
				if len(admitted) != 0 {
					return fmt.Errorf("the --generation arm admits a repair class")
				}
			}
		}
	}
	if len(seenArms) != len(s7AVSelectorArms) {
		return fmt.Errorf("the selector switch covers %d of %d arms", len(seenArms), len(s7AVSelectorArms))
	}

	// (3) The class-collapse: §9.3's precedence assigns each hash exactly one
	// class, ownership first and unidentifiable bytes second, before any class
	// set is built.
	inspect := program.function("InspectIntentArchive")
	if inspect == nil {
		return fmt.Errorf("InspectIntentArchive is missing")
	}
	collapse := (*ast.SwitchStmt)(nil)
	ast.Inspect(inspect.Body, func(node ast.Node) bool {
		branch, ok := node.(*ast.SwitchStmt)
		if !ok || branch.Tag != nil {
			return true
		}
		if s7AVIdentNames(branch)["RepairClass"] == 0 {
			return true
		}
		collapse = branch
		return true
	})
	if collapse == nil {
		return fmt.Errorf("the repair-class collapse switch is missing from InspectIntentArchive")
	}
	clauses := []*ast.CaseClause{}
	for _, clause := range collapse.Body.List {
		caseClause, ok := clause.(*ast.CaseClause)
		if !ok {
			return fmt.Errorf("the collapse switch carries an unsupported clause")
		}
		clauses = append(clauses, caseClause)
	}
	if len(clauses) < 2 {
		return fmt.Errorf("the collapse switch has %d clauses, want the full precedence", len(clauses))
	}
	first := s7AVIdentNames(clauses[0])
	if first["owned"] == 0 || first["RepairClass"] != 0 {
		return fmt.Errorf("the collapse switch does not decide ownership first, before any repair class")
	}
	second := s7AVIdentNames(clauses[1])
	if second["IntentArchiveBlobUnidentifiable"] == 0 ||
		second["IntentArchiveRepairCorruptObject"] == 0 {
		return fmt.Errorf(
			"the collapse switch does not put an unidentifiable observation in the corrupt-object class second, before liveness",
		)
	}
	if second["hasRetained"] != 0 || second["hasTombstone"] != 0 || second["live"] != 0 {
		return fmt.Errorf(
			"the unidentifiable precedence arm is conditioned on liveness, so a hash can land in a class other than corrupt-object",
		)
	}
	for _, clause := range clauses {
		if s7AVIdentNames(clause)["RepairClass"] > 1 {
			return fmt.Errorf("a collapse clause assigns more than one repair class to a hash")
		}
	}
	return nil
}

func s7AVTerminates(statements []ast.Stmt) bool {
	if len(statements) == 0 {
		return false
	}
	_, ok := statements[len(statements)-1].(*ast.ReturnStmt)
	return ok
}

func TestS7AVRepairAdmissionPredicateGuard(t *testing.T) {
	sources := s7AVRepoSources(t, s7AVStoreArchiveSource)

	if err := s7AVValidateAdmissionPredicate(sources); err != nil {
		t.Fatalf("PIB-549 baseline validation failed: %v", err)
	}

	fixtures := []struct {
		name      string
		old       string
		new       string
		count     int
		wantClass string
	}{
		{
			name:      "rev-10-sole-inconsistency-rule",
			old:       "equalStringSets(selected, class.Hashes) {",
			new:       "equalStringSets(selected, class.Hashes) && len(class.Hashes) == 1 {",
			count:     1,
			wantClass: "conditions admission on an instance count",
		},
		{
			name:      "rev-11-sole-class-rule",
			old:       "\tselected := sortedUniqueStrings(selectedHashes)\n",
			new:       "\tselected := sortedUniqueStrings(selectedHashes)\n\tif len(inspection.Classes) != 1 {\n\t\treturn \"\", intentArchiveInspectionError(inspection, \"the archive holds more than one repair class\")\n\t}\n",
			count:     1,
			wantClass: "sole-class gate",
		},
		{
			name:      "partial-class-coverage-admitted",
			old:       "if report != nil && equalStringSets(selected, report.Hashes) {",
			new:       "if report != nil && len(selected) != 0 {",
			count:     1,
			wantClass: "does not require total class coverage",
		},
		{
			name:      "all-admitted-beside-a-second-class",
			old:       "\t\tif len(inspection.Classes) == 1 {",
			new:       "\t\tif len(inspection.Classes) >= 1 {",
			count:     1,
			wantClass: "not exactly one class",
		},
		{
			name: "precedence-routes-a-hash-to-another-class",
			old: "		case observation.State == IntentArchiveBlobUnidentifiable:\n" +
				"			hashReport.RepairClass = IntentArchiveRepairCorruptObject\n",
			new: "		case observation.State == IntentArchiveBlobUnidentifiable && hasRetained && hasTombstone:\n" +
				"			hashReport.RepairClass = IntentArchiveRepairMixedReference\n" +
				"		case observation.State == IntentArchiveBlobUnidentifiable:\n" +
				"			hashReport.RepairClass = IntentArchiveRepairCorruptObject\n",
			count:     1,
			wantClass: "in the corrupt-object class second",
		},
		{
			name:      "rev-12-corrupt-does-not-block",
			old:       "\tif inspection.Classes[0].Class == IntentArchiveRepairCorruptObject {",
			new:       "\tif inspection.Classes[0].Class == IntentArchiveRepairCorruptObject && len(inspection.Classes) == 1 {",
			count:     1,
			wantClass: "conditioned on the observed class set",
		},
	}

	for _, fixture := range fixtures {
		mutated := s7AVMutate(t, sources, s7AVStoreArchiveSource, fixture.old, fixture.new, fixture.count)
		err := s7AVValidateAdmissionPredicate(mutated)
		if err == nil {
			t.Fatalf("PIB-549 sensitivity fixture %q was accepted by the admission validator", fixture.name)
		}
		if !strings.Contains(err.Error(), fixture.wantClass) {
			t.Fatalf("PIB-549 sensitivity fixture %q: want error class %q, got: %v",
				fixture.name, fixture.wantClass, err)
		}
	}

	if err := s7AVValidateAdmissionPredicate(sources); err != nil {
		t.Fatalf("PIB-549 unmutated admission predicate was rejected after sensitivity: %v", err)
	}
}

// ─── PIB-546 ──────────────────────────────────────────────────────────────────

// The three storage methods that write archive bytes. Every archive mutation in
// the product reaches one of them; the guard derives the mutating store API and
// the mutating CLI entry points from that fact rather than from a list.
var s7AVStorageMutators = map[string]bool{
	"PublishBlob": true,
	"CASIndex":    true,
	"RemoveBlob":  true,
}

// The whole-index X11 scan. Every global storage validation reaches it.
const s7AVGlobalScanPrimitive = "InspectIntentArchive"

type s7AVFinding struct {
	entry string
	route string
}

// s7AVMemoEntry is one memoised sub-walk. Caching only the resulting "already
// scanned" fact would silently drop the mutations the sub-walk found, because
// findings are recorded as a side effect: a helper first walked under the
// recovery would never report the same mutation again when the ordinary purge
// reached it. The entry therefore carries the findings too, and the cache key
// carries the route context they were attributed under, so a cache hit replays
// them for the context that produced them and a different context re-walks.
type s7AVMemoEntry struct {
	after    bool
	findings []s7AVFinding
}

type s7AVFlowWalker struct {
	functions map[string]*ast.FuncDecl
	aliases   map[string]string
	memo      map[string]s7AVMemoEntry
	stack     []string
	findings  []s7AVFinding
	entry     string
	truncated int
}

// resolve maps a call expression to a walkable top-level function, following the
// package-level indirection vars the CLI installs over the store API.
func (walker *s7AVFlowWalker) resolve(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		if target, ok := walker.aliases[fun.Name]; ok {
			return target
		}
		if _, ok := walker.functions[fun.Name]; ok {
			return fun.Name
		}
	case *ast.SelectorExpr:
		if base, ok := fun.X.(*ast.Ident); ok && base.Name == "store" {
			if _, ok := walker.functions[fun.Sel.Name]; ok {
				return fun.Sel.Name
			}
		}
	}
	return ""
}

// walkFunction returns whether the function scans on every path that returns
// normally, recording every mutation reachable without a preceding scan.
func (walker *s7AVFlowWalker) walkFunction(name string, scanned bool) bool {
	for _, active := range walker.stack {
		if active == name {
			// The recursion guard truncates this path. Any sub-walk in
			// progress therefore depends on the caller chain, which the memo
			// key does not carry, so nothing above it may be cached.
			walker.truncated++
			return scanned
		}
	}
	key := walker.entry + "|" + walker.routeContext() + "|" + name + "|" + strconv.FormatBool(scanned)
	if cached, ok := walker.memo[key]; ok {
		walker.findings = append(walker.findings, cached.findings...)
		return cached.after
	}
	function := walker.functions[name]
	if function == nil {
		return scanned
	}
	before := len(walker.findings)
	truncatedBefore := walker.truncated
	walker.stack = append(walker.stack, name)
	after, _ := walker.walkStatements(function.Body.List, scanned)
	walker.stack = walker.stack[:len(walker.stack)-1]
	if walker.truncated == truncatedBefore {
		walker.memo[key] = s7AVMemoEntry{
			after:    after,
			findings: append([]s7AVFinding(nil), walker.findings[before:]...),
		}
	}
	return after
}

// routeContext is the outermost store API already on the stack, which is what
// route() attributes a finding to. Two calls of the same helper under two
// different store APIs are two different contexts and must both be walked.
func (walker *s7AVFlowWalker) routeContext() string {
	for _, name := range walker.stack {
		if s7AVRouteAttribution[name] {
			return name
		}
	}
	return ""
}

// walkStatements threads the "has the global scan already run on this path"
// fact through the structured control flow, reporting every mutation the fact
// does not dominate.
func (walker *s7AVFlowWalker) walkStatements(statements []ast.Stmt, scanned bool) (bool, bool) {
	terminated := false
	for _, statement := range statements {
		switch typed := statement.(type) {
		case *ast.ReturnStmt:
			scanned = walker.walkExpressions(typed, scanned)
			return scanned, true
		case *ast.BranchStmt:
			return scanned, true
		case *ast.IfStmt:
			scanned = walker.walkExpressions(typed.Cond, scanned)
			if typed.Init != nil {
				scanned = walker.walkExpressions(typed.Init, scanned)
			}
			bodyAfter, bodyTerminated := walker.walkStatements(typed.Body.List, scanned)
			elseAfter, elseTerminated := scanned, false
			if typed.Else != nil {
				elseAfter, elseTerminated = walker.walkStatements(s7AVElseStatements(typed.Else), scanned)
			}
			switch {
			case bodyTerminated && elseTerminated:
				return scanned, true
			case bodyTerminated:
				scanned = elseAfter
			case elseTerminated:
				scanned = bodyAfter
			default:
				scanned = bodyAfter && elseAfter
			}
		case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			after := scanned
			hasDefault := false
			for _, clause := range s7AVClauseBodies(typed, &hasDefault) {
				clauseAfter, clauseTerminated := walker.walkStatements(clause, scanned)
				if !clauseTerminated {
					after = after && clauseAfter
				}
			}
			if !hasDefault {
				after = after && scanned
			}
			scanned = after
		case *ast.ForStmt:
			walker.walkStatements(typed.Body.List, scanned)
		case *ast.RangeStmt:
			scanned = walker.walkExpressions(typed.X, scanned)
			walker.walkStatements(typed.Body.List, scanned)
		case *ast.BlockStmt:
			var blockTerminated bool
			scanned, blockTerminated = walker.walkStatements(typed.List, scanned)
			if blockTerminated {
				return scanned, true
			}
		case *ast.LabeledStmt:
			var labelTerminated bool
			scanned, labelTerminated = walker.walkStatements([]ast.Stmt{typed.Stmt}, scanned)
			if labelTerminated {
				return scanned, true
			}
		default:
			scanned = walker.walkExpressions(statement, scanned)
		}
	}
	return scanned, terminated
}

func s7AVClauseBodies(statement ast.Stmt, hasDefault *bool) [][]ast.Stmt {
	bodies := [][]ast.Stmt{}
	var list []ast.Stmt
	switch typed := statement.(type) {
	case *ast.SwitchStmt:
		list = typed.Body.List
	case *ast.TypeSwitchStmt:
		list = typed.Body.List
	case *ast.SelectStmt:
		list = typed.Body.List
	}
	for _, clause := range list {
		switch typed := clause.(type) {
		case *ast.CaseClause:
			if typed.List == nil {
				*hasDefault = true
			}
			bodies = append(bodies, typed.Body)
		case *ast.CommClause:
			if typed.Comm == nil {
				*hasDefault = true
			}
			bodies = append(bodies, typed.Body)
		}
	}
	return bodies
}

// walkExpressions visits the calls inside one statement or expression in source
// order, descending into resolvable callees.
func (walker *s7AVFlowWalker) walkExpressions(node ast.Node, scanned bool) bool {
	calls := []*ast.CallExpr{}
	ast.Inspect(node, func(current ast.Node) bool {
		if call, ok := current.(*ast.CallExpr); ok {
			calls = append(calls, call)
		}
		return true
	})
	sort.SliceStable(calls, func(i, j int) bool { return calls[i].Pos() < calls[j].Pos() })
	for _, call := range calls {
		name := s7AVCallName(call)
		if name == s7AVGlobalScanPrimitive {
			scanned = true
			continue
		}
		if s7AVStorageMutators[name] {
			if _, ok := call.Fun.(*ast.SelectorExpr); ok && !scanned {
				walker.findings = append(walker.findings, s7AVFinding{
					entry: walker.entry,
					route: walker.route(),
				})
			}
			continue
		}
		if target := walker.resolve(call); target != "" {
			scanned = walker.walkFunction(target, scanned) || scanned
		}
	}
	return scanned
}

// s7AVRouteAttribution names the operator-visible archive mutation routes a
// finding is attributed to.
var s7AVRouteAttribution = map[string]bool{
	"PublishIntentArchiveBlobs": true,
	"ExecuteIntentArchivePurge": true,
	"RecoverPendingPurge":       true,
}

// route attributes a finding to the outermost store API function on the stack,
// which is the operator-visible archive mutation.
func (walker *s7AVFlowWalker) route() string {
	if context := walker.routeContext(); context != "" {
		return context
	}
	if len(walker.stack) == 0 {
		return walker.entry
	}
	return walker.stack[len(walker.stack)-1]
}

// s7AVEntryPoints derives the archive command entry points: the CLI's `run*`
// command runners that no other `run*` runner delegates to. Cobra wiring
// closures are not runners, so a runner referenced only from a command
// constructor stays an entry point.
func s7AVEntryPoints(program *s7AVProgram, functions map[string]*ast.FuncDecl) []string {
	runners := []*ast.FuncDecl{}
	for _, source := range []string{s7AVCLIArchiveSource, s7AVCLIPrepareSource} {
		file := program.files[source]
		if file == nil {
			continue
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || function.Body == nil {
				continue
			}
			if !strings.HasPrefix(function.Name.Name, "run") {
				continue
			}
			if _, ok := functions[function.Name.Name]; ok {
				runners = append(runners, function)
			}
		}
	}
	delegated := map[string]bool{}
	for _, runner := range runners {
		ast.Inspect(runner.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			ident, ok := call.Fun.(*ast.Ident)
			if !ok || ident.Name == runner.Name.Name {
				return true
			}
			if strings.HasPrefix(ident.Name, "run") {
				delegated[ident.Name] = true
			}
			return true
		})
	}
	entries := []string{}
	for _, runner := range runners {
		if delegated[runner.Name.Name] {
			continue
		}
		entries = append(entries, runner.Name.Name)
	}
	sort.Strings(entries)
	return entries
}

// s7AVArchiveAliases resolves the CLI's package-level store-API indirection.
func s7AVArchiveAliases(program *s7AVProgram) map[string]string {
	aliases := map[string]string{}
	for _, source := range program.order {
		file := program.files[source]
		if file == nil {
			continue
		}
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.VAR {
				continue
			}
			for _, spec := range generic.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok || len(value.Names) != len(value.Values) {
					continue
				}
				for index, name := range value.Names {
					selector, ok := value.Values[index].(*ast.SelectorExpr)
					if !ok {
						continue
					}
					base, ok := selector.X.(*ast.Ident)
					if !ok || base.Name != "store" {
						continue
					}
					aliases[name.Name] = selector.Sel.Name
				}
			}
		}
	}
	return aliases
}

// s7AVValidateRecoveryOrdering derives the ordering of pending-purge recovery
// against the whole-index X11 scan from the implementation's control flow.
func s7AVValidateRecoveryOrdering(sources map[string]string) error {
	order := []string{s7AVStoreArchiveSource, s7AVCLIArchiveSource, s7AVCLIPrepareSource}
	program, err := s7AVParseProgram(sources, order)
	if err != nil {
		return err
	}
	functions := program.functions()
	aliases := s7AVArchiveAliases(program)
	entries := s7AVEntryPoints(program, functions)
	if len(entries) == 0 {
		return fmt.Errorf("no archive command entry point was derived")
	}

	findings := []s7AVFinding{}
	for _, entry := range entries {
		walker := &s7AVFlowWalker{
			functions: functions,
			aliases:   aliases,
			memo:      map[string]s7AVMemoEntry{},
			entry:     entry,
		}
		walker.walkFunction(entry, false)
		findings = append(findings, walker.findings...)
	}

	routes := map[string]map[string]bool{}
	for _, finding := range findings {
		if routes[finding.route] == nil {
			routes[finding.route] = map[string]bool{}
		}
		routes[finding.route][finding.entry] = true
	}
	if len(routes) == 0 {
		return fmt.Errorf(
			"no path mutates before the global scan, so the pending-purge recovery exception no longer exists",
		)
	}
	if len(routes) != 1 {
		names := []string{}
		for route := range routes {
			names = append(names, route)
		}
		sort.Strings(names)
		return fmt.Errorf("%d archive mutations precede the global scan: %v, want only RecoverPendingPurge", len(routes), names)
	}
	for route, entrySet := range routes {
		if route != "RecoverPendingPurge" {
			return fmt.Errorf("the archive mutation preceding the global scan is %s, want RecoverPendingPurge", route)
		}
		if len(entrySet) != 1 {
			names := []string{}
			for entry := range entrySet {
				names = append(names, entry)
			}
			sort.Strings(names)
			return fmt.Errorf("the recovery exception is granted to %d entry points: %v", len(entrySet), names)
		}
		for entry := range entrySet {
			if !strings.Contains(entry, "IntentArchivePurge") {
				return fmt.Errorf("the recovery exception is granted to %s rather than to the confirmed purge", entry)
			}
		}
	}

	// The recovery is restricted to hashes the index already marks
	// removal-pending, and it never continues into the requested selector.
	recover := program.function("RecoverPendingPurge")
	if recover == nil {
		return fmt.Errorf("RecoverPendingPurge is missing")
	}
	pendingBound := false
	ast.Inspect(recover.Body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Rhs) != 1 {
			return true
		}
		call, ok := assignment.Rhs[0].(*ast.CallExpr)
		if ok && s7AVCallName(call) == "PendingIntentArchiveHashes" {
			pendingBound = true
		}
		return true
	})
	if !pendingBound {
		return fmt.Errorf("RecoverPendingPurge does not restrict itself to the index's removal-pending hashes")
	}
	ranged := false
	ast.Inspect(recover.Body, func(node ast.Node) bool {
		loop, ok := node.(*ast.RangeStmt)
		if !ok {
			return true
		}
		if ident, ok := loop.X.(*ast.Ident); ok && ident.Name == "pending" {
			ranged = true
		}
		return true
	})
	if !ranged {
		return fmt.Errorf("RecoverPendingPurge does not iterate the removal-pending set it computed")
	}

	confirmed := program.function("runFeatureIntentArchivePurgeConfirmed")
	if confirmed == nil {
		return fmt.Errorf("runFeatureIntentArchivePurgeConfirmed is missing")
	}
	recoveredBranch := (*ast.IfStmt)(nil)
	ast.Inspect(confirmed.Body, func(node ast.Node) bool {
		branch, ok := node.(*ast.IfStmt)
		if !ok {
			return true
		}
		if s7AVIdentNames(branch.Cond)["IntentArchivePurgeRecovered"] == 0 {
			return true
		}
		recoveredBranch = branch
		return true
	})
	if recoveredBranch == nil {
		return fmt.Errorf("the confirmed purge does not branch on a completed recovery")
	}
	if !s7AVTerminates(recoveredBranch.Body.List) {
		return fmt.Errorf(
			"the recovery branch continues into the selector after finalizing rather than returning terminal recovered",
		)
	}
	planCalls := s7AVCalls(confirmed.Body, "intentArchivePlanPurge")
	if len(planCalls) != 1 {
		return fmt.Errorf("the confirmed purge plans the selector %d times, want exactly 1", len(planCalls))
	}
	if planCalls[0].Pos() < recoveredBranch.Pos() {
		return fmt.Errorf("the confirmed purge plans the selector before completing the pending recovery")
	}
	return nil
}

// s7AVRecoveryFixture is the PIB-546 runtime composition: one removal-pending
// hash the purge transaction owns, beside one unrelated mixed tombstone/live
// hash the whole-index scan would refuse.
type s7AVRecoveryFixture struct {
	root       string
	slug       string
	pendingH1  string
	mixedH2    string
	h1BlobRel  string
	h2BlobRel  string
	h2Bytes    []byte
	generation string
}

func s7AVWriteRecoveryFixture(t *testing.T, label string) s7AVRecoveryFixture {
	t.Helper()
	root, slug := prepareS4Workspace(t, "S7 AV PIB 546 "+label)
	prepareS4WriteReadyBundle(t, root, slug, true)

	h1Data := []byte("PIB-546 owned pending h1 " + label + "\n")
	h2Data := []byte("PIB-546 unrelated mixed h2 " + label + "\n")

	pending := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactAnalysis, h1Data, store.IntentArchiveWireRemovalPending,
	)
	mixedRetained := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactExploration, h2Data, store.IntentArchiveWireRetained,
	)
	mixedTombstoned := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactSpec, h2Data, store.IntentArchiveWireTombstoned,
	)
	pendingGen := intentArchiveCLIGeneration(t, slug, pending)
	mixedGen := intentArchiveCLIGeneration(t, slug, mixedRetained, mixedTombstoned)
	index := intentArchiveCLIIndex(t, slug, pendingGen, mixedGen)
	writeIntentArchiveCLIFixture(t, root, slug, index, map[string][]byte{
		pending.ContentSHA256:       h1Data,
		mixedRetained.ContentSHA256: h2Data,
	})
	h1Rel, err := store.IntentArchiveBlobRel(slug, pending.ContentSHA256)
	if err != nil {
		t.Fatal(err)
	}
	h2Rel, err := store.IntentArchiveBlobRel(slug, mixedRetained.ContentSHA256)
	if err != nil {
		t.Fatal(err)
	}
	return s7AVRecoveryFixture{
		root:       root,
		slug:       slug,
		pendingH1:  pending.ContentSHA256,
		mixedH2:    mixedRetained.ContentSHA256,
		h1BlobRel:  h1Rel,
		h2BlobRel:  h2Rel,
		h2Bytes:    append([]byte(nil), h2Data...),
		generation: mixedGen.GenerationID,
	}
}

func TestS7AVRecoveryBeforeGlobalScanGuard(t *testing.T) {
	sources := s7AVRepoSources(t,
		s7AVStoreArchiveSource, s7AVCLIArchiveSource, s7AVCLIPrepareSource,
	)

	if err := s7AVValidateRecoveryOrdering(sources); err != nil {
		t.Fatalf("PIB-546 baseline control-flow validation failed: %v", err)
	}

	// The derived control flow is only load-bearing if the product behaves the
	// way the walk says it does, so the same property is observed end to end.
	for _, selector := range []struct {
		name string
		args func(fixture s7AVRecoveryFixture) []string
	}{
		{
			name: "orphans",
			args: func(s7AVRecoveryFixture) []string { return []string{"--orphans"} },
		},
		{
			name: "blob-h2",
			args: func(fixture s7AVRecoveryFixture) []string {
				return []string{"--blob", fixture.mixedH2}
			},
		},
	} {
		fixture := s7AVWriteRecoveryFixture(t, selector.name)
		h1Path := filepath.Join(fixture.root, filepath.FromSlash(fixture.h1BlobRel))
		h2Path := filepath.Join(fixture.root, filepath.FromSlash(fixture.h2BlobRel))

		// A mutating prepare over the same archive still refuses exit 3
		// recovery-pending: the exception belongs to the confirmed purge alone.
		preCode, preStdout, _, _ := runPrepare(t,
			"--path", fixture.root, "prepare", fixture.slug, "--json", "--quiet",
		)
		if preCode != 3 {
			t.Fatalf("PIB-546 %s pre-recovery prepare exit=%d, want 3\n%s", selector.name, preCode, preStdout)
		}
		preReport := prepareS4Report(t, preStdout)
		if preReport.Refusal == nil ||
			preReport.Refusal.Code != string(store.IntentArchiveCodeRecoveryPending) {
			t.Fatalf("PIB-546 %s pre-recovery prepare refusal = %#v", selector.name, preReport.Refusal)
		}

		code, stdout, stderr, _ := runPrepare(t,
			s7ASPurgeArgs(fixture.root, fixture.slug, selector.args(fixture), true, true, true)...,
		)
		if code != 0 || stderr != "" {
			t.Fatalf("PIB-546 %s exit=%d stderr=%q\n%s", selector.name, code, stderr, stdout)
		}
		report := decodeIntentArchivePurgeReport(t, stdout)
		if report.Outcome != string(store.IntentArchivePurgeRecovered) {
			t.Fatalf("PIB-546 %s outcome = %q, want recovered\n%s", selector.name, report.Outcome, stdout)
		}
		if report.Recovery == nil ||
			len(report.Recovery.FinalizedHashes) != 1 ||
			report.Recovery.FinalizedHashes[0] != fixture.pendingH1 {
			t.Fatalf("PIB-546 %s recovery = %#v\n%s", selector.name, report.Recovery, stdout)
		}
		// The selector was not processed: the report says so, and the mixed
		// hash is untouched.
		advisory := false
		for _, item := range report.Advisories {
			if item.Code == "recovered-prior-transaction" {
				advisory = true
			}
		}
		if !advisory {
			t.Fatalf("PIB-546 %s did not disclose that the selector was skipped\n%s", selector.name, stdout)
		}
		if report.Refusal != nil || report.RemainingRepairs != nil {
			t.Fatalf("PIB-546 %s carried a refusal or repair plan on the recovery form\n%s", selector.name, stdout)
		}
		if _, err := os.Stat(h1Path); !os.IsNotExist(err) {
			t.Fatalf("PIB-546 %s left the finalized owned blob in place: %v", selector.name, err)
		}
		h2After, err := os.ReadFile(h2Path)
		if err != nil || string(h2After) != string(fixture.h2Bytes) {
			t.Fatalf("PIB-546 %s changed the unrelated mixed blob: err=%v", selector.name, err)
		}
		_, index := readIntentArchiveCLIIndex(t, fixture.root, fixture.slug)
		for _, state := range s7ATWireStates(index, fixture.pendingH1) {
			if state != store.IntentArchiveWireTombstoned {
				t.Fatalf("PIB-546 %s left h1 in state %q", selector.name, state)
			}
		}
		mixedStates := s7ATWireStates(index, fixture.mixedH2)
		if len(mixedStates) != 2 {
			t.Fatalf("PIB-546 %s changed the mixed hash's reference count: %v", selector.name, mixedStates)
		}
		retained, tombstoned := 0, 0
		for _, state := range mixedStates {
			switch state {
			case store.IntentArchiveWireRetained:
				retained++
			case store.IntentArchiveWireTombstoned:
				tombstoned++
			}
		}
		if retained != 1 || tombstoned != 1 {
			t.Fatalf("PIB-546 %s changed the mixed hash's wire states: %v", selector.name, mixedStates)
		}

		// The rerun performs the full global scan and decides per §9.3.1.
		rerunCode, rerunStdout, _, _ := runPrepare(t,
			s7ASPurgeArgs(fixture.root, fixture.slug, selector.args(fixture), true, true, true)...,
		)
		rerunReport := decodeIntentArchivePurgeReport(t, rerunStdout)
		switch selector.name {
		case "orphans":
			if rerunCode != 3 || rerunReport.Refusal == nil ||
				rerunReport.Refusal.Code != string(store.IntentArchiveCodeIndexStorageInconsistent) {
				t.Fatalf("PIB-546 orphans rerun exit=%d refusal=%#v\n%s",
					rerunCode, rerunReport.Refusal, rerunStdout)
			}
			if _, err := os.Stat(h2Path); err != nil {
				t.Fatalf("PIB-546 orphans rerun removed the mixed blob: %v", err)
			}
		case "blob-h2":
			if rerunCode != 0 || rerunReport.Refusal != nil {
				t.Fatalf("PIB-546 blob rerun exit=%d refusal=%#v\n%s",
					rerunCode, rerunReport.Refusal, rerunStdout)
			}
			if _, err := os.Stat(h2Path); !os.IsNotExist(err) {
				t.Fatalf("PIB-546 blob rerun left the repaired mixed blob: %v", err)
			}
		}
	}

	// A fixture is one or more source deltas applied in order. Every fixture
	// below is a single delta except the shared-helper one, which needs the two
	// deltas its comment names because the ordinary route is dominated by two
	// independent global scans.
	type s7AVSourceDelta struct {
		source string
		old    string
		new    string
		count  int
	}
	fixtures := []struct {
		name      string
		deltas    []s7AVSourceDelta
		wantClass string
	}{
		{
			name: "global-scan-runs-before-the-recovery",
			deltas: []s7AVSourceDelta{{
				source: s7AVCLIArchiveSource,
				old:    "\tif indexExists {\n\t\trecovered, recoverErr := intentArchiveRecoverPurge(storage, slug)\n",
				new: "\tif indexExists {\n" +
					"\t\tif _, scanErr := store.CaptureIntentArchive(storage, slug); scanErr != nil {\n" +
					"\t\t\treport.Outcome = \"refused\"\n" +
					"\t\t\treport.Refusal = intentArchiveRefusalFromError(slug, scanErr, nil, options)\n" +
					"\t\t\treturn emitIntentArchivePurgeReport(cmd, report, 3)\n" +
					"\t\t}\n" +
					"\t\trecovered, recoverErr := intentArchiveRecoverPurge(storage, slug)\n",
				count: 1,
			}},
			wantClass: "no path mutates before the global scan",
		},
		{
			name: "recovery-continues-into-the-selector",
			deltas: []s7AVSourceDelta{{
				source: s7AVCLIArchiveSource,
				old:    "\t\t\treturn emitIntentArchivePurgeReport(cmd, report, 0)\n\t\t}\n\t}\n\tplan, planErr := intentArchivePlanPurge(",
				new:    "\t\t\t_ = emitIntentArchivePurgeReport(cmd, report, 0)\n\t\t}\n\t}\n\tplan, planErr := intentArchivePlanPurge(",
				count:  1,
			}},
			wantClass: "continues into the selector",
		},
		{
			// The ordinary selector execution and the recovery reach the same
			// per-hash helper, `executeIntentArchivePurgeHash`. A walk that
			// memoised that helper by name and scan-state alone would record
			// its removal once, under the recovery that reached it first, and
			// then silently accept this second, unscanned route on a cache hit.
			// Two deltas are required because the ordinary route is dominated
			// by two independent scans: the CLI plans before executing, and
			// `ExecuteIntentArchivePurge` re-captures and re-derives the plan.
			// Removing both is exactly the regression the row exists to catch —
			// an ordinary purge that removes blobs without the whole-index scan.
			name: "ordinary-purge-execution-shares-the-helper-before-the-scan",
			deltas: []s7AVSourceDelta{
				{
					source: s7AVCLIArchiveSource,
					old:    "\tplan, planErr := intentArchivePlanPurge(storage, slug, options.selector(), true)\n",
					new: "\tif early, earlyErr := intentArchiveExecutePurge(storage, store.IntentArchivePurgePlan{\n" +
						"\t\tFeature:  slug,\n" +
						"\t\tSelector: options.selector(),\n" +
						"\t}); earlyErr != nil {\n" +
						"\t\treport = applyIntentArchivePurgeResult(report, early, options)\n" +
						"\t\treturn emitIntentArchivePurgeFailure(cmd, report, early, earlyErr, options)\n" +
						"\t}\n" +
						"\tplan, planErr := intentArchivePlanPurge(storage, slug, options.selector(), true)\n",
					count: 1,
				},
				{
					source: s7AVStoreArchiveSource,
					old: "\tsnapshot, err := CaptureIntentArchive(storage, plan.Feature)\n" +
						"\tif err != nil {\n" +
						"\t\treturn result, err\n" +
						"\t}\n" +
						"\tif !snapshot.IndexCapture.Equal(plan.IndexPreimage) {\n" +
						"\t\treturn result, intentArchiveError(IntentArchiveCodePurgeIndexChanged, \"index.json changed after purge planning\", 3)\n" +
						"\t}\n" +
						"\tfresh, err := BuildIntentArchivePurgePlan(snapshot, plan.Selector, true)\n" +
						"\tif err != nil {\n" +
						"\t\treturn result, err\n" +
						"\t}\n",
					new: "\tindexCapture, index, err := captureIntentArchiveIndexOnly(storage, plan.Feature)\n" +
						"\tif err != nil {\n" +
						"\t\treturn result, err\n" +
						"\t}\n" +
						"\tsnapshot := IntentArchiveSnapshot{Feature: plan.Feature, IndexCapture: indexCapture, Index: index}\n" +
						"\tif !snapshot.IndexCapture.Equal(plan.IndexPreimage) {\n" +
						"\t\treturn result, intentArchiveError(IntentArchiveCodePurgeIndexChanged, \"index.json changed after purge planning\", 3)\n" +
						"\t}\n" +
						"\tfresh := plan\n",
					count: 1,
				},
			},
			wantClass: "2 archive mutations precede the global scan",
		},
		{
			name: "mutating-prepare-granted-the-same-exception",
			deltas: []s7AVSourceDelta{{
				source: s7AVCLIPrepareSource,
				old: "\tif len(pending) != 0 {\n" +
					"\t\tretry := preparePendingPurgeCommand(slug, pending)\n",
				new: "\tif len(pending) != 0 {\n" +
					"\t\tif _, recoverErr := store.RecoverPendingPurge(archiveStorage, slug); recoverErr != nil {\n" +
					"\t\t\treport = prepareStoreArchiveFailure(report, recoverErr, true)\n" +
					"\t\t\t_ = release()\n" +
					"\t\t\treturn emitPreparePublishReport(cmd, report, prepareArchiveExit(recoverErr, 3))\n" +
					"\t\t}\n" +
					"\t\tretry := preparePendingPurgeCommand(slug, pending)\n",
				count: 1,
			}},
			wantClass: "entry points",
		},
	}

	for _, fixture := range fixtures {
		mutated := sources
		for _, delta := range fixture.deltas {
			mutated = s7AVMutate(t, mutated, delta.source, delta.old, delta.new, delta.count)
		}
		err := s7AVValidateRecoveryOrdering(mutated)
		if err == nil {
			t.Fatalf("PIB-546 sensitivity fixture %q was accepted by the ordering validator", fixture.name)
		}
		if !strings.Contains(err.Error(), fixture.wantClass) {
			t.Fatalf("PIB-546 sensitivity fixture %q: want error class %q, got: %v",
				fixture.name, fixture.wantClass, err)
		}
	}

	if err := s7AVValidateRecoveryOrdering(sources); err != nil {
		t.Fatalf("PIB-546 unmutated ordering was rejected after sensitivity: %v", err)
	}
}
