//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/tesseracode/tesserapatch/internal/store"
)

const s7AXDoctorSource = "internal/workflow/doctor_d9.go"

// s7AXParsedSources parses one AX source program and fails the caller when the
// bytes under analysis are not valid Go. Every AX semantic sensitivity fixture
// is asserted to parse before the validator is asked to reject it, so a fixture
// can never pass by being a syntax error.
func s7AXParsedSources(t *testing.T, sources map[string]string, order []string) *s7AVProgram {
	t.Helper()
	program, err := s7AVParseProgram(sources, order)
	if err != nil {
		t.Fatalf("AX sensitivity fixture is not valid Go: %v", err)
	}
	return program
}

// s7AXProveBuildable proves one AX sensitivity fixture's mutated program is not
// merely parseable but type-checks as the shipped package does. It reuses the AP
// guard's go/types checker, which substitutes the mutated bytes into the real
// package and reports every type error, so a fixture that names an identifier
// which is not in scope where its mutation landed — the defect the first
// `case IntentArchiveSelectorOrphans:` anchor produced — fails here rather than
// being silently accepted as a rejected fixture.
func s7AXProveBuildable(t *testing.T, sources map[string]string, order []string) *s7AVProgram {
	t.Helper()
	program := s7AXParsedSources(t, sources, order)
	if _, err := s7APTypeCheckReportPackages(sources); err != nil {
		t.Fatalf("AX sensitivity fixture does not type-check: %v", err)
	}
	return program
}

// s7AXAssertMutationAnchor proves a sensitivity fixture's anchor text occurs
// exactly once in the source and lies inside the named function's declaration,
// so the mutation can only land where that function's identifiers are in scope.
// This is the static half of the guarantee `s7AXProveBuildable` completes: the
// anchor identity alone makes an undefined identifier impossible.
func s7AXAssertMutationAnchor(
	t *testing.T, sources map[string]string, source, function, anchor string,
) {
	t.Helper()
	body, present := sources[source]
	if !present {
		t.Fatalf("AX anchor source %q is missing", source)
	}
	if occurrences := strings.Count(body, anchor); occurrences != 1 {
		t.Fatalf("AX anchor %q occurs %d time(s) in %s, want exactly 1",
			anchor, occurrences, source)
	}
	program := s7AXParsedSources(t, sources, []string{source})
	declaration := program.function(function)
	if declaration == nil {
		t.Fatalf("AX anchor host %s is missing from %s", function, source)
	}
	file := program.fset.File(declaration.Pos())
	if file == nil {
		t.Fatalf("AX anchor host %s has no position information", function)
	}
	start, end := file.Offset(declaration.Pos()), file.Offset(declaration.End())
	offset := strings.Index(body, anchor)
	if offset < start || offset+len(anchor) > end {
		t.Fatalf("AX anchor %q lands at offset %d, outside %s (%d..%d)",
			anchor, offset, function, start, end)
	}
}

func s7AXAssignedIdents(node ast.Node, field string) []string {
	names := []string{}
	ast.Inspect(node, func(current ast.Node) bool {
		switch typed := current.(type) {
		case *ast.AssignStmt:
			for index, left := range typed.Lhs {
				selector, ok := left.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != field || index >= len(typed.Rhs) {
					continue
				}
				if ident, ok := typed.Rhs[index].(*ast.Ident); ok {
					names = append(names, ident.Name)
				}
			}
		case *ast.KeyValueExpr:
			key, ok := typed.Key.(*ast.Ident)
			if !ok || key.Name != field {
				return true
			}
			if ident, ok := typed.Value.(*ast.Ident); ok {
				names = append(names, ident.Name)
			}
		}
		return true
	})
	return names
}

// ─── PIB-561 ──────────────────────────────────────────────────────────────────

// s7AXRepairClassRankOrder reads the shipped rank order out of the classifier's
// own declaration rather than out of prose.
func s7AXRepairClassRankOrder(program *s7AVProgram) ([]string, error) {
	for _, source := range program.order {
		for _, declaration := range program.files[source].Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.VAR {
				continue
			}
			for _, spec := range generic.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok || len(value.Names) != 1 ||
					value.Names[0].Name != "intentArchiveRepairClassOrder" ||
					len(value.Values) != 1 {
					continue
				}
				literal, ok := value.Values[0].(*ast.CompositeLit)
				if !ok {
					return nil, errors.New("the repair-class order is not a composite literal")
				}
				order := []string{}
				for _, element := range literal.Elts {
					ident, ok := element.(*ast.Ident)
					if !ok {
						return nil, errors.New("the repair-class order holds a non-constant element")
					}
					order = append(order, ident.Name)
				}
				return order, nil
			}
		}
	}
	return nil, errors.New("intentArchiveRepairClassOrder is missing")
}

// s7AXValidatePrecedenceSource derives the four ranks, the single blocking rank
// and the corrupt-first admission precondition from the implementation. It is
// the one validator PIB-561's real observations and its four semantic
// sensitivity fixtures are judged by.
func s7AXValidatePrecedenceSource(sources map[string]string) error {
	program, err := s7AVParseProgram(sources, []string{s7AVStoreArchiveSource})
	if err != nil {
		return err
	}

	// (1) The rank order is the declared closed order, corrupt first.
	order, err := s7AXRepairClassRankOrder(program)
	if err != nil {
		return err
	}
	wantOrder := []string{
		"IntentArchiveRepairCorruptObject",
		"IntentArchiveRepairDanglingReference",
		"IntentArchiveRepairMixedReference",
		"IntentArchiveRepairUnreferencedResidue",
	}
	if !reflect.DeepEqual(order, wantOrder) {
		return fmt.Errorf("repair-class rank order = %v, want %v", order, wantOrder)
	}

	inspect := program.function("InspectIntentArchive")
	if inspect == nil {
		return errors.New("InspectIntentArchive is missing")
	}

	// (2) Rank is the declared position and exactly one rank is blocking.
	blockingExpressions := 0
	rankExpressions := 0
	ast.Inspect(inspect.Body, func(node ast.Node) bool {
		literal, ok := node.(*ast.CompositeLit)
		if !ok {
			return true
		}
		ident, ok := literal.Type.(*ast.Ident)
		if !ok || ident.Name != "IntentArchiveRepairClassReport" {
			return true
		}
		for _, element := range literal.Elts {
			pair, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := pair.Key.(*ast.Ident)
			if !ok {
				continue
			}
			switch key.Name {
			case "Rank":
				binary, ok := pair.Value.(*ast.BinaryExpr)
				if !ok || binary.Op != token.ADD {
					continue
				}
				left, leftOK := binary.X.(*ast.Ident)
				right, rightOK := binary.Y.(*ast.BasicLit)
				if leftOK && rightOK && left.Name == "rank" && right.Value == "1" {
					rankExpressions++
				}
			case "Blocking":
				binary, ok := pair.Value.(*ast.BinaryExpr)
				if !ok || binary.Op != token.EQL {
					continue
				}
				left, leftOK := binary.X.(*ast.Ident)
				right, rightOK := binary.Y.(*ast.Ident)
				if leftOK && rightOK && left.Name == "class" &&
					right.Name == "IntentArchiveRepairCorruptObject" {
					blockingExpressions++
				}
			}
		}
		return true
	})
	if rankExpressions != 1 {
		return fmt.Errorf("the class report derives its rank from the declared order %d time(s), want 1", rankExpressions)
	}
	if blockingExpressions != 1 {
		return errors.New(
			"the blocking rank is not exactly the rank-1 corrupt class: no single `class == IntentArchiveRepairCorruptObject` blocking expression survives",
		)
	}

	// (3) One hash collapses into exactly one class arm, and the corrupt arm is
	// conditioned on the observation alone — never on liveness, which is what
	// would reclassify an unreferenced corrupt object as removable residue.
	collapse := (*ast.SwitchStmt)(nil)
	ast.Inspect(inspect.Body, func(node ast.Node) bool {
		if collapse != nil {
			return false
		}
		candidate, ok := node.(*ast.SwitchStmt)
		if !ok || candidate.Tag != nil {
			return true
		}
		for _, clause := range candidate.Body.List {
			caseClause, ok := clause.(*ast.CaseClause)
			if !ok {
				continue
			}
			if len(s7AXAssignedIdents(caseClause, "RepairClass")) != 0 {
				collapse = candidate
				return false
			}
		}
		return true
	})
	if collapse == nil {
		return errors.New("the per-hash repair-class collapse switch is missing")
	}
	assignedByClause := [][]string{}
	conditions := []ast.Expr{}
	for _, clause := range collapse.Body.List {
		caseClause, ok := clause.(*ast.CaseClause)
		if !ok {
			return errors.New("the collapse switch carries an unsupported clause")
		}
		if caseClause.List == nil {
			return errors.New("the collapse switch carries a default clause, so its domain is not closed")
		}
		if len(caseClause.List) != 1 {
			return errors.New("a collapse clause matches more than one condition")
		}
		conditions = append(conditions, caseClause.List[0])
		assignedByClause = append(assignedByClause, s7AXAssignedIdents(caseClause, "RepairClass"))
	}
	if len(assignedByClause) != 5 {
		return fmt.Errorf("the collapse switch has %d clauses, want the owned escape plus one arm per rank", len(assignedByClause))
	}
	if len(assignedByClause[0]) != 0 {
		return errors.New("the owned escape assigns a repair class instead of routing to the pending owner")
	}
	if ident, ok := conditions[0].(*ast.Ident); !ok || ident.Name != "owned" {
		return errors.New("the first collapse clause is not the ownership escape")
	}
	collapsed := []string{}
	for index := 1; index < len(assignedByClause); index++ {
		if len(assignedByClause[index]) != 1 {
			return fmt.Errorf("collapse clause %d assigns %d classes, want exactly 1", index, len(assignedByClause[index]))
		}
		collapsed = append(collapsed, assignedByClause[index][0])
	}
	if !reflect.DeepEqual(collapsed, wantOrder) {
		return fmt.Errorf("the collapse arms are ordered %v, want the declared rank order %v", collapsed, wantOrder)
	}
	corrupt, ok := conditions[1].(*ast.BinaryExpr)
	if !ok || corrupt.Op != token.EQL {
		return errors.New("the corrupt arm is not the bare unidentifiable-observation comparison")
	}
	corruptNames := s7AVIdentNames(corrupt)
	if corruptNames["IntentArchiveBlobUnidentifiable"] != 1 ||
		corruptNames["live"] != 0 || corruptNames["hasRetained"] != 0 ||
		corruptNames["hasTombstone"] != 0 {
		return errors.New(
			"the corrupt arm is conditioned on reference liveness, so an unreferenced corrupt object can fall through to another class",
		)
	}
	for index := 2; index < len(conditions); index++ {
		if s7AVIdentNames(conditions[index])["IntentArchiveBlobUnidentifiable"] != 0 {
			return fmt.Errorf("collapse clause %d also matches an unidentifiable object", index)
		}
	}

	// (4) The rank-1 block belongs to the admission predicate as a whole: no
	// selector arm carries its own corrupt-object refusal. A per-selector block
	// is what would let `--orphans` refuse while `--blob` repairs the storage
	// layer around bytes tpatch cannot identify.
	admit := program.function("admitIntentArchiveRepair")
	if admit == nil {
		return errors.New("admitIntentArchiveRepair is missing")
	}
	perSelectorBlocks := 0
	ast.Inspect(admit.Body, func(node ast.Node) bool {
		clause, ok := node.(*ast.CaseClause)
		if !ok {
			return true
		}
		if s7AVIdentNames(clause)["IntentArchiveRepairCorruptObject"] != 0 {
			perSelectorBlocks++
		}
		return true
	})
	if perSelectorBlocks != 0 {
		return fmt.Errorf(
			"%d selector arm(s) name the rank-1 corrupt class themselves, so the block is evaluated per selector rather than once over every confirmed selector",
			perSelectorBlocks,
		)
	}

	// (5) The corrupt-first precondition is unconditional, dominating and total
	// over every selector arm.
	return s7AVValidateAdmissionPredicate(sources)
}

// s7AXCorruptOrigin is one construction of §9.3's corrupt-object rows.
type s7AXCorruptOrigin struct {
	name      string
	root      string
	slug      string
	paths     []string
	hashes    []string
	resulting []string
	stages    int
}

func s7AXConfirmedSelectors(t *testing.T, root, slug string) [][]string {
	t.Helper()
	_, index := readIntentArchiveCLIIndex(t, root, slug)
	hashes := map[string]bool{}
	generations := []string{}
	for _, generation := range index.Generations {
		generations = append(generations, generation.GenerationID)
		for _, replacement := range generation.Replaced {
			hashes[replacement.ContentSHA256] = true
		}
	}
	sorted := []string{}
	for hash := range hashes {
		sorted = append(sorted, hash)
	}
	sort.Strings(sorted)
	sort.Strings(generations)
	selectors := [][]string{{"--orphans"}}
	for _, hash := range sorted {
		selectors = append(selectors, []string{"--blob", hash})
	}
	for _, generation := range generations {
		selectors = append(selectors, []string{"--generation", generation})
	}
	return append(selectors, []string{"--all"})
}

func s7AXWriteUnindexedCorruptObject(t *testing.T, root, slug string) string {
	t.Helper()
	stray := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactSpec,
		[]byte("PIB-561 hash the index never mentions\n"),
		store.IntentArchiveWireRetained,
	)
	rel, err := store.IntentArchiveBlobRel(slug, stray.ContentSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(root, filepath.FromSlash(rel)),
		[]byte("PIB-561 bytes that do not hash to this managed name\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	return rel
}

func s7AXCorruptOrigins(t *testing.T) []s7AXCorruptOrigin {
	t.Helper()
	origins := []s7AXCorruptOrigin{}

	// §9.3 row 5: an unidentifiable object under a retained reference.
	retained := s7AVWriteRepairArchive(
		t, "PIB-561-retained", s7AVRepairSpec{corrupt: true},
	)
	origins = append(origins, s7AXCorruptOrigin{
		name:      "retained-reference",
		root:      retained.root,
		slug:      retained.slug,
		paths:     []string{retained.blobRel[retained.corrupt]},
		hashes:    []string{retained.corrupt},
		resulting: []string{string(store.IntentArchiveRepairDanglingReference)},
		stages:    2,
	})

	// §9.3 row 18: an unidentifiable object under a tombstoned reference of a
	// hash that is still live through another retained reference.
	overlap := s7AWWriteOverlapFixture(t, false)
	origins = append(origins, s7AXCorruptOrigin{
		name:      "tombstoned-reference-of-live-hash",
		root:      overlap.root,
		slug:      overlap.slug,
		paths:     []string{overlap.blobRel},
		hashes:    []string{overlap.hash},
		resulting: []string{string(store.IntentArchiveRepairDanglingReference)},
		stages:    2,
	})

	// §9.3 row 17: an unidentifiable object whose hash every reference
	// tombstones, so the prerequisite leaves it clean rather than dangling.
	unreferenced := s7AXWriteUnreferencedCorruptArchive(
		t, "PIB-561-unreferenced", s7AXUnreferencedCorruptSpec{},
	)
	origins = append(origins, s7AXCorruptOrigin{
		name:      "tombstoned-reference-of-unreferenced-hash",
		root:      unreferenced.root,
		slug:      unreferenced.slug,
		paths:     []string{unreferenced.blobRel[unreferenced.corrupt]},
		hashes:    []string{unreferenced.corrupt},
		resulting: []string{},
		stages:    1,
	})

	// An object in blobs/ whose hash the index never mentions.
	stray := s7AVWriteRepairArchive(
		t, "PIB-561-unindexed", s7AVRepairSpec{residues: 1},
	)
	strayRel := s7AXWriteUnindexedCorruptObject(t, stray.root, stray.slug)
	origins = append(origins, s7AXCorruptOrigin{
		name:      "object-the-index-never-mentions",
		root:      stray.root,
		slug:      stray.slug,
		paths:     []string{strayRel},
		hashes:    []string{strings.TrimSuffix(filepath.Base(strayRel), ".blob")},
		resulting: []string{},
		stages:    2,
	})
	return origins
}

func s7AXAssertCorruptOriginBlocked(t *testing.T, origin s7AXCorruptOrigin) {
	t.Helper()
	archive := s7AVRepairArchive{root: origin.root, slug: origin.slug}
	var shared *intentArchiveRemainingRepairsReport
	for _, selector := range s7AXConfirmedSelectors(t, origin.root, origin.slug) {
		before := s7AXTreeSnapshot(t, filepath.Join(origin.root, ".tpatch"))
		code, stdout, _, report, removed, indexCASs :=
			s7AWRunPurgeWithMutationSpy(t, archive, selector)
		if code != 3 {
			t.Fatalf("PIB-561 %s selector %v exit=%d, want 3\n%s",
				origin.name, selector, code, stdout)
		}
		if len(removed) != 0 || indexCASs != 0 {
			t.Fatalf("PIB-561 %s selector %v wrote: removed=%v index-CAS=%d",
				origin.name, selector, removed, indexCASs)
		}
		if !reflect.DeepEqual(
			before, s7AXTreeSnapshot(t, filepath.Join(origin.root, ".tpatch")),
		) {
			t.Fatalf("PIB-561 %s selector %v changed the tree", origin.name, selector)
		}
		if report.Outcome != "refused" || report.Refusal == nil ||
			report.Refusal.Code != string(store.IntentArchiveCodeBlobCorrupt) {
			t.Fatalf("PIB-561 %s selector %v refusal = %#v\n%s",
				origin.name, selector, report.Refusal, stdout)
		}
		remaining := report.RemainingRepairs
		if remaining == nil || len(remaining.Stages) != origin.stages ||
			remaining.StagesRemaining != origin.stages {
			t.Fatalf("PIB-561 %s selector %v plan = %#v", origin.name, selector, remaining)
		}
		stage := remaining.Stages[0]
		if stage.Ordinal != 1 ||
			stage.Class != string(store.IntentArchiveRepairCorruptObject) ||
			stage.Kind != string(store.IntentArchiveRepairStageManual) ||
			stage.RepairCWD != store.IntentArchiveRepairCWD ||
			!reflect.DeepEqual(stage.Paths, origin.paths) ||
			!reflect.DeepEqual(stage.Hashes, origin.hashes) ||
			!reflect.DeepEqual(stage.ResultingClasses, origin.resulting) {
			t.Fatalf("PIB-561 %s selector %v corrupt stage = %#v", origin.name, selector, stage)
		}
		for index, next := range remaining.Stages {
			if next.Ordinal != index+1 {
				t.Fatalf("PIB-561 %s stage ordinals are not contiguous: %#v", origin.name, remaining.Stages)
			}
			if index > 0 && next.Class == string(store.IntentArchiveRepairCorruptObject) {
				t.Fatalf("PIB-561 %s ranked a second corrupt stage: %#v", origin.name, remaining.Stages)
			}
		}
		warning, removal := s7AVExtractRemovalLine(stage.Repair)
		if err := s7AVValidatePrintedRemoval(s7AVPrintedProcedure{
			label:          "PIB-561 " + origin.name,
			block:          stage.Repair + "\n" + s7AVGitHistoryCaveat,
			warning:        warning,
			removeCommand:  removal,
			blobRel:        origin.paths[0],
			historyCaveats: []string{s7AVGitHistoryCaveat},
		}); err != nil {
			t.Fatal(err)
		}
		if shared != nil && !reflect.DeepEqual(shared, remaining) {
			t.Fatalf("PIB-561 %s selector %v changed the shared plan", origin.name, selector)
		}
		shared = remaining
	}
}

// s7AXRankOrderPermutations enumerates every presentation order of the three
// non-blocking ranks.
func s7AXRankOrderPermutations() [][]string {
	return [][]string{
		{"dangling", "mixed", "residue"},
		{"dangling", "residue", "mixed"},
		{"mixed", "dangling", "residue"},
		{"mixed", "residue", "dangling"},
		{"residue", "dangling", "mixed"},
		{"residue", "mixed", "dangling"},
	}
}

func s7AXAssertRanksTwoToFourUnordered(t *testing.T) {
	t.Helper()
	for _, order := range s7AXRankOrderPermutations() {
		label := strings.Join(order, "-")
		archive := s7AVWriteRepairArchive(
			t, "PIB-561-"+label,
			s7AVRepairSpec{residues: 1, dangling: 1, mixed: 1},
		)
		for step, class := range order {
			var selector []string
			switch class {
			case "dangling":
				selector = s7AVBlobSelector(archive.dangling)
			case "mixed":
				selector = s7AVBlobSelector(archive.mixed)
			case "residue":
				selector = []string{"--orphans"}
			}
			code, stdout, stderr, report, _, _ :=
				s7AWRunPurgeWithMutationSpy(t, archive, selector)
			if code != 0 || stderr != "" ||
				report.Outcome != string(store.IntentArchivePurgePurged) {
				t.Fatalf("PIB-561 order %s step %d (%s) = exit:%d stderr:%q report:%#v\n%s",
					label, step, class, code, stderr, report, stdout)
			}
		}
		code, stdout, _, _ := runPrepare(t,
			"--path", archive.root, "feature", "intent-archive", "list", archive.slug,
			"--json", "--quiet",
		)
		if code != 0 {
			t.Fatalf("PIB-561 order %s left the archive inconsistent: exit=%d\n%s",
				label, code, stdout)
		}
	}
}

func TestS7AXRepairPrecedenceGuard(t *testing.T) {
	sources := s7AVRepoSources(t, s7AVStoreArchiveSource)
	if err := s7AXValidatePrecedenceSource(sources); err != nil {
		t.Fatalf("PIB-561 baseline source validation failed: %v", err)
	}

	for _, origin := range s7AXCorruptOrigins(t) {
		s7AXAssertCorruptOriginBlocked(t, origin)
	}
	s7AXAssertRanksTwoToFourUnordered(t)

	corruptGuard := "\tif inspection.Classes[0].Class == IntentArchiveRepairCorruptObject {\n" +
		"\t\treturn \"\", intentArchiveInspectionError(inspection, \"a corrupt managed object blocks every confirmed selector\")\n" +
		"\t}\n"
	corruptArm := "\t\tcase observation.State == IntentArchiveBlobUnidentifiable:\n" +
		"\t\t\thashReport.RepairClass = IntentArchiveRepairCorruptObject\n"
	// The orphans arm carries its first statement, which makes the anchor unique
	// to admitIntentArchiveRepair. The bare `case IntentArchiveSelectorOrphans:`
	// line also opens selectIntentArchivePurgeTargets' selector switch, where
	// `inspection` does not exist and which this validator never reads.
	orphansArm := "\tcase IntentArchiveSelectorOrphans:\n" +
		"\t\tif class := intentArchiveClassReport("

	// The negative control for that anchor choice: the bare arm line occurs in
	// both functions, and mutating on it lands in selectIntentArchivePurgeTargets
	// where `inspection` does not exist. The type check every fixture below is
	// put through rejects exactly that program, so this harness cannot regress to
	// a fixture that only looks like Go.
	ambiguousArm := "\tcase IntentArchiveSelectorOrphans:\n"
	if occurrences := strings.Count(
		sources[s7AVStoreArchiveSource], ambiguousArm,
	); occurrences != 2 {
		t.Fatalf("PIB-561 the bare orphans-arm anchor occurs %d time(s), want the 2 that made it ambiguous",
			occurrences)
	}
	ambiguous := s7AVMutate(t, sources, s7AVStoreArchiveSource, ambiguousArm,
		ambiguousArm+
			"\t\tif inspection.Classes[0].Class == IntentArchiveRepairCorruptObject {\n"+
			"\t\t\treturn \"\", intentArchiveInspectionError(inspection, \"a corrupt managed object blocks every confirmed selector\")\n"+
			"\t\t}\n",
		1)
	s7AXParsedSources(t, ambiguous, []string{s7AVStoreArchiveSource})
	_, ambiguousErr := s7APTypeCheckReportPackages(ambiguous)
	if ambiguousErr == nil || !strings.Contains(ambiguousErr.Error(), "inspection") {
		t.Fatalf("PIB-561 the type check accepted a mutation naming `inspection` outside admitIntentArchiveRepair: %v",
			ambiguousErr)
	}

	fixtures := []struct {
		name      string
		rejection string
		mutate    func(*testing.T, map[string]string) map[string]string
	}{
		{
			// rev-12's withdrawn rule: the corrupt class "does not block the
			// other two", so --orphans --yes clears residue beside an object
			// tpatch cannot identify.
			name:      "corrupt-does-not-block-the-other-two",
			rejection: "the corrupt-first precondition is missing",
			mutate: func(t *testing.T, sources map[string]string) map[string]string {
				return s7AVMutate(t, sources, s7AVStoreArchiveSource, corruptGuard, "", 1)
			},
		},
		{
			// An unreferenced corrupt object treated as removable residue.
			name:      "unreferenced-corrupt-object-is-residue",
			rejection: "the collapse switch has",
			mutate: func(t *testing.T, sources map[string]string) map[string]string {
				return s7AVMutate(t, sources, s7AVStoreArchiveSource, corruptArm,
					"\t\tcase observation.State == IntentArchiveBlobUnidentifiable && live:\n"+
						"\t\t\thashReport.RepairClass = IntentArchiveRepairCorruptObject\n"+
						"\t\tcase observation.State == IntentArchiveBlobUnidentifiable && !live:\n"+
						"\t\t\thashReport.RepairClass = IntentArchiveRepairUnreferencedResidue\n",
					1)
			},
		},
		{
			// Ranks 2-4 made blocking too, which imposes a total order the
			// design does not require and refuses PIB-552's residue-then-mixed
			// sequence.
			name:      "ranks-two-to-four-block-as-well",
			rejection: "conditioned on the observed class set",
			mutate: func(t *testing.T, sources map[string]string) map[string]string {
				return s7AVMutate(t, sources, s7AVStoreArchiveSource,
					"\tif inspection.Classes[0].Class == IntentArchiveRepairCorruptObject {\n",
					"\tif inspection.Classes[0].Class == IntentArchiveRepairCorruptObject || len(inspection.Classes) > 1 {\n",
					1)
			},
		},
		{
			// The block evaluated per selector: --orphans refuses while --blob
			// repairs the storage layer around unidentified bytes.
			name:      "block-evaluated-per-selector",
			rejection: "evaluated per selector",
			mutate: func(t *testing.T, sources map[string]string) map[string]string {
				mutated := s7AVMutate(t, sources, s7AVStoreArchiveSource, corruptGuard, "", 1)
				s7AXAssertMutationAnchor(t, mutated, s7AVStoreArchiveSource,
					"admitIntentArchiveRepair", orphansArm)
				return s7AVMutate(t, mutated, s7AVStoreArchiveSource, orphansArm,
					"\tcase IntentArchiveSelectorOrphans:\n"+
						"\t\tif inspection.Classes[0].Class == IntentArchiveRepairCorruptObject {\n"+
						"\t\t\treturn \"\", intentArchiveInspectionError(inspection, \"a corrupt managed object blocks every confirmed selector\")\n"+
						"\t\t}\n"+
						"\t\tif class := intentArchiveClassReport(",
					1)
			},
		},
	}
	// Every sensitivity fixture is judged inside this body: the mutation is
	// applied, proved to be a program that both parses and type-checks, and then
	// required to be rejected by the one validator the real observations above
	// were judged by — and rejected for its own reason, so no fixture can decay
	// into a duplicate of another.
	mutatedBodies := map[string]string{}
	rejections := map[string]string{}
	for _, fixture := range fixtures {
		mutated := fixture.mutate(t, sources)
		s7AXProveBuildable(t, mutated, []string{s7AVStoreArchiveSource})
		if previous, seen := mutatedBodies[mutated[s7AVStoreArchiveSource]]; seen {
			t.Fatalf("PIB-561 %s mutates %s identically to %s",
				fixture.name, s7AVStoreArchiveSource, previous)
		}
		mutatedBodies[mutated[s7AVStoreArchiveSource]] = fixture.name
		err := s7AXValidatePrecedenceSource(mutated)
		if err == nil {
			t.Fatalf("PIB-561 %s: precedence validator accepted a wrong precedence rule",
				fixture.name)
		}
		if !strings.Contains(err.Error(), fixture.rejection) {
			t.Fatalf("PIB-561 %s was rejected for the wrong reason: want %q, got %v",
				fixture.name, fixture.rejection, err)
		}
		if previous, seen := rejections[err.Error()]; seen {
			t.Fatalf("PIB-561 %s and %s are rejected identically: %v",
				fixture.name, previous, err)
		}
		rejections[err.Error()] = fixture.name
	}
	if len(fixtures) != 4 {
		t.Fatalf("PIB-561 carries %d semantic sensitivity fixtures, want 4", len(fixtures))
	}
}

// ─── PIB-564 ──────────────────────────────────────────────────────────────────

type s7AXExpectedStage struct {
	class             string
	kind              string
	hashes            []string
	afterPrerequisite bool
}

// s7AXArchiveModel is the guard's own account of one archive's repair classes,
// built from the fixture's construction rather than from the shipped report.
type s7AXArchiveModel struct {
	corrupt   []string
	predicted []string
	dangling  []string
	mixed     []string
	residues  []string
}

func s7AXSortedCopy(values []string) []string {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return sorted
}

// s7AXDeriveStages re-derives §9.3.1's arithmetic independently: one stage where
// the corrupt class is non-empty, plus one stage for each of the three
// repairable classes that is non-empty **after** the prerequisite's predicted
// reclassification.
func s7AXDeriveStages(model s7AXArchiveModel, repaired string) []s7AXExpectedStage {
	stages := []s7AXExpectedStage{}
	if len(model.corrupt) != 0 {
		stages = append(stages, s7AXExpectedStage{
			class:  string(store.IntentArchiveRepairCorruptObject),
			kind:   string(store.IntentArchiveRepairStageManual),
			hashes: s7AXSortedCopy(model.corrupt),
		})
	}
	dangling := s7AXSortedCopy(append(append([]string(nil), model.dangling...), model.predicted...))
	for _, candidate := range []struct {
		class  string
		hashes []string
		after  bool
	}{
		{string(store.IntentArchiveRepairDanglingReference), dangling, len(model.predicted) != 0},
		{string(store.IntentArchiveRepairMixedReference), s7AXSortedCopy(model.mixed), false},
		{string(store.IntentArchiveRepairUnreferencedResidue), s7AXSortedCopy(model.residues), false},
	} {
		if len(candidate.hashes) == 0 || candidate.class == repaired {
			continue
		}
		stages = append(stages, s7AXExpectedStage{
			class:             candidate.class,
			kind:              string(store.IntentArchiveRepairStagePurge),
			hashes:            candidate.hashes,
			afterPrerequisite: candidate.after,
		})
	}
	return stages
}

// s7AXDeriveStagesByObservedClass is the wrong arithmetic PIB-564 exists to
// reject: it emits one stage per observed class *and* the prerequisite, so a
// corrupt class whose hash is clean over-counts by one.
func s7AXDeriveStagesByObservedClass(model s7AXArchiveModel, repaired string) []s7AXExpectedStage {
	stages := s7AXDeriveStages(model, repaired)
	if len(model.corrupt) != 0 && len(model.predicted) == 0 {
		stages = append(stages, s7AXExpectedStage{
			class:             string(store.IntentArchiveRepairDanglingReference),
			kind:              string(store.IntentArchiveRepairStagePurge),
			hashes:            s7AXSortedCopy(model.corrupt),
			afterPrerequisite: true,
		})
	}
	return stages
}

// s7AXValidateStagePlan is the single validator every PIB-564 observation and
// every PIB-564 sensitivity fixture is judged by.
func s7AXValidateStagePlan(
	label string,
	rankOrder []string,
	plan *intentArchiveRemainingRepairsReport,
	expected []s7AXExpectedStage,
	repaired string,
	admitted bool,
) error {
	if plan == nil {
		return fmt.Errorf("%s carried no remaining_repairs object", label)
	}
	if !plan.RerunRequired {
		return fmt.Errorf("%s did not set rerun_required", label)
	}
	if plan.StagesRemaining != len(plan.Stages) {
		return fmt.Errorf("%s stages_remaining = %d, len(stages) = %d",
			label, plan.StagesRemaining, len(plan.Stages))
	}
	if len(plan.Stages) != len(expected) {
		return fmt.Errorf("%s stages = %d, independently derived %d",
			label, len(plan.Stages), len(expected))
	}
	if admitted != (plan.RepairedClass != "") {
		return fmt.Errorf("%s repaired_class = %q on an admitted=%t carrier",
			label, plan.RepairedClass, admitted)
	}
	if plan.RepairedClass != repaired {
		return fmt.Errorf("%s repaired_class = %q, want %q", label, plan.RepairedClass, repaired)
	}
	rank := map[string]int{}
	for index, class := range rankOrder {
		rank[class] = index
	}
	previousRank := -1
	for index, stage := range plan.Stages {
		want := expected[index]
		if stage.Ordinal != index+1 {
			return fmt.Errorf("%s stage %d ordinal = %d, want 1-based and contiguous",
				label, index, stage.Ordinal)
		}
		if stage.Class != want.class || stage.Kind != want.kind ||
			stage.AfterPrerequisite != want.afterPrerequisite ||
			!reflect.DeepEqual(stage.Hashes, want.hashes) {
			return fmt.Errorf("%s stage %d = %#v, independently derived %#v", label, index, stage, want)
		}
		if stage.RepairCWD != store.IntentArchiveRepairCWD {
			return fmt.Errorf("%s stage %d repair_cwd = %q, want %q",
				label, index, stage.RepairCWD, store.IntentArchiveRepairCWD)
		}
		isManual := stage.Kind == string(store.IntentArchiveRepairStageManual)
		isCorrupt := stage.Class == string(store.IntentArchiveRepairCorruptObject)
		if isManual != isCorrupt {
			return fmt.Errorf("%s stage %d kind %q does not belong to class %q",
				label, index, stage.Kind, stage.Class)
		}
		position, known := rank[stage.Class]
		if !known {
			return fmt.Errorf("%s stage %d names class %q outside the declared rank order",
				label, index, stage.Class)
		}
		if position <= previousRank {
			return fmt.Errorf("%s stages are not sorted in the declared rank order", label)
		}
		previousRank = position
		if err := s7AXScanRepairQuantification(label+" stage repair", stage.Repair); err != nil {
			return err
		}
	}
	if plan.NextStage == nil {
		return fmt.Errorf("%s carried no next_stage", label)
	}
	first := plan.Stages[0]
	if plan.NextStage.Ordinal != first.Ordinal ||
		plan.NextStage.Kind != first.Kind ||
		plan.NextStage.Class != first.Class {
		return fmt.Errorf("%s next_stage = %#v, which disagrees with stages[0] %#v",
			label, plan.NextStage, first)
	}
	return nil
}

// s7AXPerClassInvocationClaim matches the quantification rev-12 promised and
// rev-13 withdrew. `one class per invocation` is a different — and true —
// statement and is deliberately not matched.
var s7AXPerClassInvocationClaim = regexp.MustCompile(
	`(?i)one\s+(?:tpatch\s+)?invocations?\s+per\s+(?:repair\s+)?class` +
		`|invocation\s+count\s+is\s+exactly\s+one\s+per\s+class`,
)

// s7AXQuantificationExemption is one cue that marks a matched quantification as
// a denial, a prohibition or a report of the withdrawn claim rather than an
// assertion of it.
type s7AXQuantificationExemption struct {
	cue     string
	pattern *regexp.Regexp
}

// s7AXExactCue matches one whole word or phrase. `not` must be the word `not`:
// the raw-substring scan this replaces also fired on `another`, `cannot` and
// `note`.
func s7AXExactCue(cue string) s7AXQuantificationExemption {
	return s7AXQuantificationExemption{
		cue:     cue,
		pattern: regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(cue) + `\b`),
	}
}

// s7AXStemCue matches one word stem and its inflections — `fail`/`fails`,
// `promis`/`promises`/`promising` — but only where the stem starts a word.
func s7AXStemCue(stem string) s7AXQuantificationExemption {
	return s7AXQuantificationExemption{
		cue:     stem,
		pattern: regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(stem) + `[a-z]*\b`),
	}
}

// s7AXQuantificationExemptions is the closed cue set. Every cue is matched on
// word boundaries and only inside the offending sentence, ahead of the claim.
var s7AXQuantificationExemptions = []s7AXQuantificationExemption{
	s7AXExactCue("not"),
	s7AXExactCue("never"),
	s7AXExactCue("no longer"),
	s7AXExactCue("may say"),
	s7AXExactCue("rather than"),
	s7AXExactCue("instead of"),
	s7AXExactCue("reads"),
	s7AXStemCue("fail"),
	s7AXStemCue("promis"),
	s7AXStemCue("restor"),
	s7AXStemCue("withdraw"),
}

// s7AXSentenceTerminators end one scanned sentence. A table-cell pipe and a
// blank line end a sentence as surely as a full stop does, so a cue or a
// quotation mark in a neighbouring cell, clause or paragraph cannot reach across
// into the claim being judged.
const s7AXSentenceTerminators = ".;!?|"

// s7AXSentenceBounds returns the bounds of the sentence enclosing one match.
func s7AXSentenceBounds(body string, start, end int) (int, int) {
	sentenceStart := 0
	for index := start - 1; index >= 0; index-- {
		if strings.IndexByte(s7AXSentenceTerminators, body[index]) >= 0 {
			sentenceStart = index + 1
			break
		}
		if body[index] == '\n' && index > 0 && body[index-1] == '\n' {
			sentenceStart = index + 1
			break
		}
	}
	sentenceEnd := len(body)
	for index := end; index < len(body); index++ {
		if strings.IndexByte(s7AXSentenceTerminators, body[index]) >= 0 {
			sentenceEnd = index
			break
		}
		if body[index] == '\n' && index+1 < len(body) && body[index+1] == '\n' {
			sentenceEnd = index
			break
		}
	}
	return sentenceStart, sentenceEnd
}

// s7AXQuotedSpan reports whether the offending claim is itself enclosed in a
// quotation — an opener before it and its matching closer after it, with no
// intervening delimiter of the same pair — rather than merely sharing a sentence
// with one. Sharing was enough for the scan this replaces, so any quoted phrase
// anywhere in the preceding 120 bytes exempted an unrelated promise.
func s7AXQuotedSpan(sentence string, start, end int) bool {
	for _, pair := range [][2]string{
		{"\u201c", "\u201d"},
		{"\u2018", "\u2019"},
	} {
		opener := strings.LastIndex(sentence[:start], pair[0])
		if opener < 0 ||
			strings.Contains(sentence[opener+len(pair[0]):start], pair[1]) {
			continue
		}
		rest := sentence[end:]
		closer := strings.Index(rest, pair[1])
		if closer < 0 {
			continue
		}
		if next := strings.Index(rest, pair[0]); next >= 0 && next < closer {
			continue
		}
		return true
	}
	// A straight double quote is its own opener and closer, so the claim is
	// inside one exactly when an odd number of them precede it in the sentence
	// and at least one closes it.
	return strings.Count(sentence[:start], "\"")%2 == 1 &&
		strings.Contains(sentence[end:], "\"")
}

func s7AXScanRepairQuantification(label, body string) error {
	for _, match := range s7AXPerClassInvocationClaim.FindAllStringIndex(body, -1) {
		sentenceStart, sentenceEnd := s7AXSentenceBounds(body, match[0], match[1])
		sentence := body[sentenceStart:sentenceEnd]
		relativeStart, relativeEnd := match[0]-sentenceStart, match[1]-sentenceStart
		before := strings.Join(strings.Fields(sentence[:relativeStart]), " ")
		exempt := false
		for _, exemption := range s7AXQuantificationExemptions {
			if exemption.pattern.MatchString(before) {
				exempt = true
				break
			}
		}
		if !exempt {
			exempt = s7AXQuotedSpan(sentence, relativeStart, relativeEnd)
		}
		if !exempt {
			return fmt.Errorf(
				"%s promises one tpatch invocation per class: %q",
				label, strings.Join(strings.Fields(sentence), " "),
			)
		}
	}
	return nil
}

// s7AXShippedStringLiterals collects every string literal in the shipped
// (non-test) Go sources, which is where a report sentence would live.
func s7AXShippedStringLiterals(t *testing.T) map[string][]string {
	t.Helper()
	root := avpRepoRoot(t)
	literals := map[string][]string{}
	for _, directory := range []string{"internal", "cmd"} {
		base := filepath.Join(root, directory)
		err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
				strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			program, err := s7AVParseProgram(
				map[string]string{rel: string(body)}, []string{rel},
			)
			if err != nil {
				return err
			}
			ast.Inspect(program.files[rel], func(node ast.Node) bool {
				literal, ok := node.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					return true
				}
				if value, unquoteErr := strconv.Unquote(literal.Value); unquoteErr == nil {
					literals[rel] = append(literals[rel], value)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(literals) == 0 {
		t.Fatal("PIB-564 shipped-string scan found no sources")
	}
	return literals
}

func s7AXStageObservation(
	t *testing.T,
	label string,
	archive s7AVRepairArchive,
	selector []string,
	wantExit int,
) *intentArchiveRemainingRepairsReport {
	t.Helper()
	code, stdout, _, report, _, _ := s7AWRunPurgeWithMutationSpy(t, archive, selector)
	if code != wantExit {
		t.Fatalf("PIB-564 %s exit=%d, want %d\n%s", label, code, wantExit, stdout)
	}
	return report.RemainingRepairs
}

func TestS7AXStageModelGuard(t *testing.T) {
	program, err := s7AVParseProgram(
		s7AVRepoSources(t, s7AVStoreArchiveSource), []string{s7AVStoreArchiveSource},
	)
	if err != nil {
		t.Fatal(err)
	}
	rankOrderNames, err := s7AXRepairClassRankOrder(program)
	if err != nil {
		t.Fatal(err)
	}
	constants := program.stringConstants()
	rankOrder := []string{}
	for _, name := range rankOrderNames {
		value, known := constants[name]
		if !known {
			t.Fatalf("PIB-564 rank-order constant %s has no shipped token", name)
		}
		rankOrder = append(rankOrder, value)
	}

	// PIB-552's archive: both carriers on one fixture.
	twoClass := s7AVWriteRepairArchive(
		t, "PIB-564-552", s7AVRepairSpec{residues: 3, mixed: 2},
	)
	twoClassModel := s7AXArchiveModel{
		mixed: twoClass.mixed, residues: twoClass.residues,
	}
	refusal := s7AXStageObservation(t, "PIB-552 refusal", twoClass, []string{"--all"}, 3)
	if err := s7AXValidateStagePlan(
		"PIB-552 exit-3 refusal", rankOrder, refusal,
		s7AXDeriveStages(twoClassModel, ""), "", false,
	); err != nil {
		t.Fatal(err)
	}
	admitted := s7AXStageObservation(t, "PIB-552 admitted", twoClass, []string{"--orphans"}, 0)
	if err := s7AXValidateStagePlan(
		"PIB-552 exit-0 admitted", rankOrder, admitted,
		s7AXDeriveStages(twoClassModel, string(store.IntentArchiveRepairUnreferencedResidue)),
		string(store.IntentArchiveRepairUnreferencedResidue), true,
	); err != nil {
		t.Fatal(err)
	}

	// PIB-553's, PIB-562's and PIB-563's archives, each re-derived.
	threeClass := s7AVWriteRepairArchive(
		t, "PIB-564-553", s7AVRepairSpec{residues: 2, dangling: 2, corrupt: true},
	)
	threeClassModel := s7AXArchiveModel{
		corrupt:   []string{threeClass.corrupt},
		predicted: []string{threeClass.corrupt},
		dangling:  threeClass.dangling,
		residues:  threeClass.residues,
	}
	unreferenced := s7AXWriteUnreferencedCorruptArchive(
		t, "PIB-564-562", s7AXUnreferencedCorruptSpec{residues: 2},
	)
	unreferencedModel := s7AXArchiveModel{
		corrupt:  []string{unreferenced.corrupt},
		residues: unreferenced.residues,
	}
	corruptMixed := s7AVWriteRepairArchive(
		t, "PIB-564-563a", s7AVRepairSpec{mixed: 2, corrupt: true},
	)
	corruptMixedModel := s7AXArchiveModel{
		corrupt:   []string{corruptMixed.corrupt},
		predicted: []string{corruptMixed.corrupt},
		mixed:     corruptMixed.mixed,
	}
	corruptDangling := s7AVWriteRepairArchive(
		t, "PIB-564-563b", s7AVRepairSpec{dangling: 2, corrupt: true},
	)
	corruptDanglingModel := s7AXArchiveModel{
		corrupt:   []string{corruptDangling.corrupt},
		predicted: []string{corruptDangling.corrupt},
		dangling:  corruptDangling.dangling,
	}

	type derivedCase struct {
		label   string
		archive s7AVRepairArchive
		model   s7AXArchiveModel
		classes int
		stages  int
		calls   int
	}
	cases := []derivedCase{
		{"PIB-553", threeClass, threeClassModel, 3, 3, 2},
		{"PIB-562", unreferenced, unreferencedModel, 2, 2, 1},
		{"PIB-563 (a)", corruptMixed, corruptMixedModel, 2, 3, 2},
		{"PIB-563 (b)", corruptDangling, corruptDanglingModel, 2, 2, 1},
	}
	for _, fixture := range cases {
		expected := s7AXDeriveStages(fixture.model, "")
		plan := s7AXStageObservation(t, fixture.label, fixture.archive, []string{"--all"}, 3)
		if err := s7AXValidateStagePlan(
			fixture.label+" exit-3 refusal", rankOrder, plan, expected, "", false,
		); err != nil {
			t.Fatal(err)
		}
		if len(expected) != fixture.stages {
			t.Fatalf("%s derived %d stages, want %d", fixture.label, len(expected), fixture.stages)
		}
		invocations := 0
		for _, stage := range expected {
			if stage.kind == string(store.IntentArchiveRepairStagePurge) {
				invocations++
			}
		}
		if invocations != fixture.calls {
			t.Fatalf("%s derived %d tpatch invocation(s), want %d",
				fixture.label, invocations, fixture.calls)
		}
	}

	// The three counts are genuinely distinct: at least one archive needs fewer
	// invocations than it has stages, and at least one has more stages than
	// observed classes.
	distinctInvocations := false
	distinctStages := false
	for _, fixture := range cases {
		if fixture.stages != fixture.calls {
			distinctInvocations = true
		}
		if fixture.stages != fixture.classes {
			distinctStages = true
		}
	}
	if !distinctInvocations || !distinctStages {
		t.Fatalf("PIB-564 no longer separates classes, stages and invocations: %#v", cases)
	}

	// The counter-examples the row re-derives: PIB-553 is three classes, three
	// stages and two invocations; PIB-562 is two classes, two stages and one.
	if cases[0].classes != 3 || cases[0].stages != 3 || cases[0].calls != 2 ||
		cases[1].classes != 2 || cases[1].stages != 2 || cases[1].calls != 1 {
		t.Fatalf("PIB-564 counter-examples drifted: %#v", cases[:2])
	}

	// Every shipped string and every PRD/ADR sentence that quantifies repair
	// work.
	for source, literals := range s7AXShippedStringLiterals(t) {
		for _, literal := range literals {
			if err := s7AXScanRepairQuantification(source, literal); err != nil {
				t.Fatal(err)
			}
		}
	}
	for label, document := range map[string]string{
		s7AVPRDRelPath: s7AVRepoDocument(t, s7AVPRDRelPath),
		s7AVADRRelPath: s7AVRepoDocument(t, s7AVADRRelPath),
	} {
		if err := s7AXScanRepairQuantification(label, document); err != nil {
			t.Fatal(err)
		}
	}

	// The exemption is sentence-scoped and word-boundary aware. Each line below
	// carries the shape of a real exempt occurrence and must survive the scan.
	cues := map[string]bool{}
	for _, exemption := range s7AXQuantificationExemptions {
		if cues[exemption.cue] {
			t.Fatalf("PIB-564 exemption cue %q is declared twice", exemption.cue)
		}
		cues[exemption.cue] = true
	}
	if len(cues) != 11 {
		t.Fatalf("PIB-564 carries %d exemption cues, want the closed set of 11", len(cues))
	}
	for _, exempt := range []string{
		"Reports enumerate repair stages, not one invocation per class.",
		"PIB-564 fails any surface that promises one invocation per class.",
		"Reports describe stages rather than one invocation per class.",
		"The plan never promises one invocation per class.",
		"No sentence in this PRD may say that an archive needs " +
			"one tpatch invocation per class.",
		"Rev-12's \u201cthe total tpatch invocation count is exactly one per " +
			"class\u201d is withdrawn.",
		"An operator reads \"one invocation per class\" and budgets three commands.",
	} {
		if err := s7AXScanRepairQuantification("PIB-564 exempt fixture", exempt); err != nil {
			t.Fatalf("PIB-564 rejected a genuinely exempt sentence: %v", err)
		}
	}

	// The must-fail cases the withdrawn substring scan exempted: `another`,
	// `cannot` and a leading `note` each carry a bare `not`; a cue or a quotation
	// belonging to a neighbouring sentence or clause is not this claim's denial
	// or this claim's quotation. All but the last were accepted before this
	// scan became sentence-scoped and token-aware.
	for _, promise := range []string{
		"Another stage clears the residue, so an archive needs one invocation per class.",
		"tpatch cannot do better, so an archive needs one invocation per class.",
		"Note that an archive needs one invocation per class.",
		"The archive is \u201chealthy\u201d and it needs one invocation per class.",
		"This is not a promise. The archive needs one invocation per class.",
		"The plan never lies; an archive needs one invocation per class.",
		"A denial would read \"whatever\" here. " +
			"The report says an archive needs one invocation per class.",
		"The total tpatch invocation count is exactly one per class.",
	} {
		if err := s7AXScanRepairQuantification("PIB-564 promise fixture", promise); err == nil {
			t.Fatalf("PIB-564 exempted a promise of one invocation per class: %q", promise)
		}
	}

	// The document bite, preserved: one unexempted promise injected into the
	// shipped PRD is still rejected.
	injectedPRD := s7AVRepoDocument(t, s7AVPRDRelPath) +
		"\n\nEvery three-class archive needs one tpatch invocation per class.\n"
	if err := s7AXScanRepairQuantification(
		s7AVPRDRelPath+" fixture", injectedPRD,
	); err == nil {
		t.Fatal("PIB-564 accepted a PRD sentence promising one tpatch invocation per class")
	}

	// The four semantic sensitivity fixtures, each judged in this body by the
	// same validator the observations above were judged by. Each carrier is
	// built first so the rejection loop below holds the only binding assertion.
	observedClassPlan := s7AXStageObservation(
		t, "PIB-562 sensitivity", unreferenced, []string{"--all"}, 3,
	)

	laterNextStage := *s7AXStageObservation(
		t, "PIB-553 next-stage", threeClass, []string{"--all"}, 3,
	)
	laterNextStage.Stages = append(
		[]intentArchiveRepairStageReport(nil), laterNextStage.Stages...,
	)
	if len(laterNextStage.Stages) < 2 {
		t.Fatalf("PIB-564 next-stage carrier has %d stage(s), want at least 2",
			len(laterNextStage.Stages))
	}
	second := laterNextStage.Stages[1]
	laterNextStage.NextStage = &intentArchiveRepairNextReport{
		Ordinal: second.Ordinal, Kind: second.Kind, Class: second.Class,
	}

	restoredSentence := *s7AXStageObservation(
		t, "PIB-553 sentence", threeClass, []string{"--all"}, 3,
	)
	restoredSentence.Stages = append(
		[]intentArchiveRepairStageReport(nil), restoredSentence.Stages...,
	)
	if len(restoredSentence.Stages) == 0 {
		t.Fatal("PIB-564 sentence carrier has no stages")
	}
	restoredSentence.Stages[0].Repair = "The total tpatch invocation count is exactly one per class."

	sensitivities := []struct {
		name     string
		label    string
		plan     *intentArchiveRemainingRepairsReport
		expected []s7AXExpectedStage
		accepted string
	}{
		{
			name:     "stage-count-from-observed-classes",
			label:    "PIB-562 observed-class count",
			plan:     observedClassPlan,
			expected: s7AXDeriveStagesByObservedClass(unreferencedModel, ""),
			accepted: "a stage count derived from observed classes",
		},
		{
			name:     "plan-only-on-the-exit-0-form",
			label:    "PIB-553 refusal without a plan",
			plan:     nil,
			expected: s7AXDeriveStages(threeClassModel, ""),
			accepted: "a refusal that carried no plan",
		},
		{
			name:     "next-stage-names-a-later-stage",
			label:    "PIB-553 later next stage",
			plan:     &laterNextStage,
			expected: s7AXDeriveStages(threeClassModel, ""),
			accepted: "a next_stage naming a later stage",
		},
		{
			name:     "report-restores-the-per-class-invocation-sentence",
			label:    "PIB-553 restored sentence",
			plan:     &restoredSentence,
			expected: s7AXDeriveStages(threeClassModel, ""),
			accepted: "a report promising one invocation per class",
		},
	}
	for _, sensitivity := range sensitivities {
		if err := s7AXValidateStagePlan(
			sensitivity.label, rankOrder, sensitivity.plan,
			sensitivity.expected, "", false,
		); err == nil {
			t.Fatalf("PIB-564 %s: validator accepted %s",
				sensitivity.name, sensitivity.accepted)
		}
	}
	if len(sensitivities) != 4 {
		t.Fatalf("PIB-564 carries %d semantic sensitivity fixtures, want 4", len(sensitivities))
	}
}

// ─── PIB-565 ──────────────────────────────────────────────────────────────────

// s7AXOutcomeWrites records how one function writes a report `Outcome` field.
type s7AXOutcomeWrites struct {
	literals  []string
	constants []string
	dynamic   []string
}

// s7AXCollectOutcomeWrites enumerates a function's return paths by collecting
// every write to the named field.
func s7AXCollectOutcomeWrites(node ast.Node, field string) s7AXOutcomeWrites {
	writes := s7AXOutcomeWrites{}
	record := func(value ast.Expr) {
		switch typed := value.(type) {
		case *ast.BasicLit:
			if typed.Kind == token.STRING {
				if decoded, err := strconv.Unquote(typed.Value); err == nil {
					writes.literals = append(writes.literals, decoded)
				}
			}
		case *ast.Ident:
			writes.constants = append(writes.constants, typed.Name)
		case *ast.CallExpr:
			ident, ok := typed.Fun.(*ast.Ident)
			if !ok || ident.Name != "string" || len(typed.Args) != 1 {
				return
			}
			switch inner := typed.Args[0].(type) {
			case *ast.Ident:
				writes.constants = append(writes.constants, inner.Name)
			case *ast.SelectorExpr:
				if pkg, ok := inner.X.(*ast.Ident); ok && pkg.Name == "store" {
					writes.constants = append(writes.constants, inner.Sel.Name)
					return
				}
				writes.dynamic = append(writes.dynamic, inner.Sel.Name)
			}
		case *ast.SelectorExpr:
			if pkg, ok := typed.X.(*ast.Ident); ok && pkg.Name == "store" {
				writes.constants = append(writes.constants, typed.Sel.Name)
				return
			}
			writes.dynamic = append(writes.dynamic, typed.Sel.Name)
		}
	}
	ast.Inspect(node, func(current ast.Node) bool {
		switch typed := current.(type) {
		case *ast.AssignStmt:
			for index, left := range typed.Lhs {
				selector, ok := left.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != field || index >= len(typed.Rhs) {
					continue
				}
				record(typed.Rhs[index])
			}
		case *ast.KeyValueExpr:
			if key, ok := typed.Key.(*ast.Ident); ok && key.Name == field {
				record(typed.Value)
			}
		}
		return true
	})
	return writes
}

func s7AXResolveOutcomeTokens(
	constants map[string]string,
	names []string,
) ([]string, error) {
	tokens := map[string]bool{}
	for _, name := range names {
		value, known := constants[name]
		if !known {
			return nil, fmt.Errorf("outcome constant %s has no shipped token", name)
		}
		tokens[value] = true
	}
	out := []string{}
	for value := range tokens {
		out = append(out, value)
	}
	sort.Strings(out)
	return out, nil
}

func s7AXUnion(sets ...[]string) []string {
	seen := map[string]bool{}
	for _, set := range sets {
		for _, value := range set {
			seen[value] = true
		}
	}
	out := []string{}
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// s7AXDerivePurgeOutcomeSets enumerates the reachable `outcome` token set of
// each purge form from the command's own return paths.
func s7AXDerivePurgeOutcomeSets(sources map[string]string) (preview, confirmed []string, err error) {
	order := []string{s7AVStoreArchiveSource, s7AVCLIArchiveSource}
	program, parseErr := s7AVParseProgram(sources, order)
	if parseErr != nil {
		return nil, nil, parseErr
	}
	constants := program.stringConstants()
	functions := program.functions()
	required := []string{
		"runFeatureIntentArchivePurge",
		"runFeatureIntentArchivePurgePreview",
		"runFeatureIntentArchivePurgeConfirmed",
		"applyIntentArchivePurgePlan",
		"applyIntentArchivePurgeResult",
		"emitIntentArchivePurgeFailure",
		"newIntentArchivePurgeReport",
		"BuildIntentArchivePurgePlan",
	}
	for _, name := range required {
		if functions[name] == nil {
			return nil, nil, fmt.Errorf("purge return-path function %s is missing", name)
		}
	}

	// The plan applier must not write the field, or the per-form derivation
	// would not be total.
	if planWrites := s7AXCollectOutcomeWrites(functions["applyIntentArchivePurgePlan"], "Outcome"); len(planWrites.literals)+
		len(planWrites.constants)+len(planWrites.dynamic) != 0 {
		return nil, nil, errors.New("applyIntentArchivePurgePlan writes the report outcome, so the per-form derivation is not total")
	}

	constructed := s7AXCollectOutcomeWrites(functions["newIntentArchivePurgeReport"], "Outcome")
	if len(constructed.constants) != 0 || len(constructed.dynamic) != 0 ||
		len(constructed.literals) != 1 {
		return nil, nil, fmt.Errorf("the purge report constructor writes %#v, want one closed default", constructed)
	}

	// The dispatcher's own refusals precede the fork, so both forms inherit
	// them and the per-form derivation stays total over the command.
	dispatch := s7AXCollectOutcomeWrites(functions["runFeatureIntentArchivePurge"], "Outcome")
	if len(dispatch.dynamic) != 0 {
		return nil, nil, fmt.Errorf("the purge dispatcher writes a dynamic outcome %v", dispatch.dynamic)
	}
	dispatchConstants, err := s7AXResolveOutcomeTokens(constants, dispatch.constants)
	if err != nil {
		return nil, nil, err
	}

	previewWrites := s7AXCollectOutcomeWrites(functions["runFeatureIntentArchivePurgePreview"], "Outcome")
	if len(previewWrites.dynamic) != 0 {
		return nil, nil, fmt.Errorf("the preview form writes a dynamic outcome %v", previewWrites.dynamic)
	}
	previewConstants, err := s7AXResolveOutcomeTokens(constants, previewWrites.constants)
	if err != nil {
		return nil, nil, err
	}
	preview = s7AXUnion(
		constructed.literals, dispatch.literals, dispatchConstants,
		previewWrites.literals, previewConstants,
	)

	// The confirmed form's dynamic writes resolve through the purge result and
	// the purge plan.
	resultTokens := []string{}
	for name, function := range functions {
		writes := s7AXCollectOutcomeWrites(function, "Outcome")
		isResultProducer := false
		ast.Inspect(function, func(node ast.Node) bool {
			assign, ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, left := range assign.Lhs {
				selector, ok := left.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "Outcome" {
					continue
				}
				if base, ok := selector.X.(*ast.Ident); ok && base.Name == "result" {
					isResultProducer = true
				}
			}
			return true
		})
		if !isResultProducer && name != "RecoverPendingPurge" &&
			name != "classifyIntentArchivePendingRecoveryInterruption" {
			continue
		}
		for _, dynamic := range writes.dynamic {
			// The only dynamic outcome a result producer inherits is the plan's
			// own, whose confirmed-form domain is derived separately below.
			if !strings.EqualFold(dynamic, "outcome") {
				return nil, nil, fmt.Errorf(
					"%s writes an underived dynamic result outcome %q", name, dynamic,
				)
			}
		}
		tokens, resolveErr := s7AXResolveOutcomeTokens(constants, writes.constants)
		if resolveErr != nil {
			return nil, nil, resolveErr
		}
		resultTokens = s7AXUnion(resultTokens, tokens)
	}
	if len(resultTokens) == 0 {
		return nil, nil, errors.New("no purge result outcome producer was derived")
	}

	// The plan outcomes reachable under --yes: every preview-only token is
	// assigned inside a branch the confirmed path cannot fall out of.
	planTokens, err := s7AXConfirmedPlanOutcomes(functions["BuildIntentArchivePurgePlan"], constants)
	if err != nil {
		return nil, nil, err
	}

	confirmedWrites := s7AXCollectOutcomeWrites(functions["runFeatureIntentArchivePurgeConfirmed"], "Outcome")
	confirmedConstants, err := s7AXResolveOutcomeTokens(constants, confirmedWrites.constants)
	if err != nil {
		return nil, nil, err
	}
	applyWrites := s7AXCollectOutcomeWrites(functions["applyIntentArchivePurgeResult"], "Outcome")
	failureWrites := s7AXCollectOutcomeWrites(functions["emitIntentArchivePurgeFailure"], "Outcome")
	failureConstants, err := s7AXResolveOutcomeTokens(constants, failureWrites.constants)
	if err != nil {
		return nil, nil, err
	}
	for _, dynamic := range append(append([]string(nil), confirmedWrites.dynamic...), applyWrites.dynamic...) {
		if dynamic != "Outcome" {
			return nil, nil, fmt.Errorf("the confirmed form writes an underived dynamic outcome %q", dynamic)
		}
	}
	confirmed = s7AXUnion(
		constructed.literals, dispatch.literals, dispatchConstants,
		confirmedWrites.literals, confirmedConstants,
		failureWrites.literals, failureConstants, resultTokens, planTokens,
	)
	return preview, confirmed, nil
}

// s7AXConfirmedPlanOutcomes derives which plan outcomes can survive into a
// confirmed report: every preview-only assignment must sit inside a `!confirmed`
// branch that returns, and the recovery-required assignment inside a branch that
// returns.
func s7AXConfirmedPlanOutcomes(
	build *ast.FuncDecl,
	constants map[string]string,
) ([]string, error) {
	previewOnly := map[string]bool{
		"IntentArchivePurgePlanned":          true,
		"IntentArchivePurgeRecoveryRequired": true,
	}
	total := map[string]int{}
	for _, name := range s7AXCollectOutcomeWrites(build.Body, "Outcome").constants {
		total[name]++
	}
	guarded := map[string]int{}
	ast.Inspect(build.Body, func(node ast.Node) bool {
		branch, ok := node.(*ast.IfStmt)
		if !ok {
			return true
		}
		body := branch.Body.List
		terminates := len(body) != 0
		if terminates {
			_, terminates = body[len(body)-1].(*ast.ReturnStmt)
		}
		if !terminates {
			return true
		}
		notConfirmed := false
		if unary, ok := branch.Cond.(*ast.UnaryExpr); ok && unary.Op == token.NOT {
			ident, isIdent := unary.X.(*ast.Ident)
			notConfirmed = isIdent && ident.Name == "confirmed"
		}
		// Only statements written directly in this branch are attributed to it,
		// so a second, unguarded assignment of the same constant elsewhere
		// cannot hide behind a guarded one.
		for _, statement := range body {
			for _, name := range s7AXCollectOutcomeWrites(statement, "Outcome").constants {
				switch name {
				case "IntentArchivePurgePlanned":
					if notConfirmed {
						guarded[name]++
					}
				case "IntentArchivePurgeRecoveryRequired":
					guarded[name]++
				}
			}
		}
		return true
	})
	reachable := []string{}
	for name, count := range total {
		if !previewOnly[name] {
			reachable = append(reachable, name)
			continue
		}
		if guarded[name] != count || count == 0 {
			return nil, fmt.Errorf(
				"plan outcome %s escapes into the confirmed form: %d of %d assignments sit in a returning preview-only branch",
				name, guarded[name], count,
			)
		}
	}
	sort.Strings(reachable)
	return s7AXResolveOutcomeTokens(constants, reachable)
}

// s7AXRefusedOutcome is the purge report's exit-3/exit-6 outcome token. It has
// no shipped constant because the refusal envelope, not the purge machine,
// writes it, so PIB-565 derives the rest of the set and pins this one literal.
const s7AXRefusedOutcome = "refused"

// s7AXPurgeReportExamples returns every JSON example in the two documents that
// shows a purge report.
func s7AXPurgeReportExamples(documents map[string]string) ([]map[string]any, error) {
	purgeKeys := []string{"remaining_repairs", "pending_purge", "purge_progress", "divergence"}
	examples := []map[string]any{}
	names := []string{}
	for name := range documents {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, block := range s7AWMarkdownFences(documents[name]) {
			var decoded map[string]any
			if err := json.Unmarshal([]byte(block), &decoded); err != nil {
				continue
			}
			if _, present := decoded["outcome"]; !present {
				continue
			}
			if command, present := decoded["command"]; present {
				if text, ok := command.(string); !ok || text != intentArchiveCommandPurge {
					continue
				}
				examples = append(examples, decoded)
				continue
			}
			if _, present := decoded["action"]; !present {
				continue
			}
			for _, key := range purgeKeys {
				if _, present := decoded[key]; present {
					examples = append(examples, decoded)
					break
				}
			}
		}
	}
	if len(examples) == 0 {
		return nil, errors.New("no purge-report JSON example was found")
	}
	return examples, nil
}

// s7AXValidatePurgeExample is the single validator the real examples and the
// JSON sensitivity fixtures are judged by.
func s7AXValidatePurgeExample(
	label string,
	example map[string]any,
	preview, confirmed []string,
) error {
	outcome, ok := example["outcome"].(string)
	if !ok {
		return fmt.Errorf("%s carries no string outcome", label)
	}
	inPreview := false
	for _, candidate := range preview {
		if candidate == outcome {
			inPreview = true
		}
	}
	inConfirmed := false
	for _, candidate := range confirmed {
		if candidate == outcome {
			inConfirmed = true
		}
	}
	if !inPreview && !inConfirmed {
		return fmt.Errorf(
			"%s shows outcome %q, which is outside the preview set %v and the confirmed set %v",
			label, outcome, preview, confirmed,
		)
	}
	if outcome == string(store.IntentArchivePurgePurged) {
		if action, _ := example["action"].(string); action != "none" {
			return fmt.Errorf("%s pins a successful purge at action %q, want \"none\"", label, action)
		}
	}
	remaining, present := example["remaining_repairs"].(map[string]any)
	if !present {
		return nil
	}
	stages, ok := remaining["stages"].([]any)
	if !ok {
		return fmt.Errorf("%s carries a remaining_repairs object with no stages array", label)
	}
	_, repaired := remaining["repaired_class"]
	// §10.2: `repaired_class` is present **if and only if** this invocation
	// repaired a class, so it belongs to the exit-0 admitted form and never to
	// the exit-3/exit-6 refusal form the `refused` token marks.
	if repaired && outcome == s7AXRefusedOutcome {
		return fmt.Errorf(
			"%s is a refusal's plan yet carries repaired_class, which only the exit-0 admitted form emits",
			label,
		)
	}
	prerequisite := -1
	for index, raw := range stages {
		stage, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("%s stage %d is not an object", label, index)
		}
		cwd, _ := stage["repair_cwd"].(string)
		if cwd != store.IntentArchiveRepairCWD {
			return fmt.Errorf("%s stage %d repair_cwd = %q, want %q",
				label, index, cwd, store.IntentArchiveRepairCWD)
		}
		kind, _ := stage["kind"].(string)
		class, _ := stage["class"].(string)
		if prerequisite < 0 &&
			(kind == string(store.IntentArchiveRepairStageManual) ||
				class == string(store.IntentArchiveRepairCorruptObject)) {
			prerequisite = index
		}
	}
	// §9.3.1/PIB-561: while the archive holds any `corrupt-object` instance it
	// is the rank-1 blocking class and **every** confirmed selector refuses
	// zero-write, so a plan that still carries that manual prerequisite can
	// never belong to an invocation that repaired a class.
	if repaired && prerequisite >= 0 {
		return fmt.Errorf(
			"%s reports a repaired class beside stage %d, a corrupt-object manual prerequisite "+
				"that refuses every confirmed selector zero-write",
			label, prerequisite,
		)
	}
	return nil
}

func TestS7AXPurgeOutcomeLiteralGuard(t *testing.T) {
	sources := s7AVRepoSources(t, s7AVStoreArchiveSource, s7AVCLIArchiveSource)
	preview, confirmed, err := s7AXDerivePurgeOutcomeSets(sources)
	if err != nil {
		t.Fatalf("PIB-565 outcome derivation failed: %v", err)
	}
	wantPreview := []string{
		string(store.IntentArchivePurgePlanned),
		string(store.IntentArchivePurgeRecoveryRequired),
		s7AXRefusedOutcome,
	}
	sort.Strings(wantPreview)
	wantConfirmed := []string{
		string(store.IntentArchivePurgeNoOp),
		string(store.IntentArchivePurgePartial),
		string(store.IntentArchivePurgePurged),
		string(store.IntentArchivePurgeRecovered),
		s7AXRefusedOutcome,
	}
	sort.Strings(wantConfirmed)
	if !reflect.DeepEqual(preview, wantPreview) {
		t.Fatalf("PIB-565 derived preview outcomes = %v, want %v", preview, wantPreview)
	}
	if !reflect.DeepEqual(confirmed, wantConfirmed) {
		t.Fatalf("PIB-565 derived confirmed outcomes = %v, want %v", confirmed, wantConfirmed)
	}

	// A confirmed purge that performed its selection is pinned.
	archive := s7AVWriteRepairArchive(
		t, "PIB-565", s7AVRepairSpec{residues: 1, mixed: 1},
	)
	code, stdout, stderr, report, _, _ :=
		s7AWRunPurgeWithMutationSpy(t, archive, []string{"--orphans"})
	if code != 0 || stderr != "" ||
		report.Outcome != string(store.IntentArchivePurgePurged) ||
		report.Action != "none" {
		t.Fatalf("PIB-565 confirmed purge = exit:%d stderr:%q outcome:%q action:%q\n%s",
			code, stderr, report.Outcome, report.Action, stdout)
	}
	if report.RemainingRepairs == nil || len(report.RemainingRepairs.Stages) == 0 {
		t.Fatalf("PIB-565 confirmed purge carried no stage plan: %#v", report.RemainingRepairs)
	}
	for index, stage := range report.RemainingRepairs.Stages {
		if stage.RepairCWD != store.IntentArchiveRepairCWD {
			t.Fatalf("PIB-565 stage %d repair_cwd = %q, want %q",
				index, stage.RepairCWD, store.IntentArchiveRepairCWD)
		}
	}

	// The preview form's tokens, observed rather than read.
	previewCode, previewStdout, _, _ := runPrepare(t,
		s7ASPurgeArgs(archive.root, archive.slug, []string{"--all"}, false, true, true)...,
	)
	previewReport := decodeIntentArchivePurgeReport(t, previewStdout)
	if previewCode != 0 || previewReport.Outcome != string(store.IntentArchivePurgePlanned) {
		t.Fatalf("PIB-565 preview = exit:%d outcome:%q\n%s",
			previewCode, previewReport.Outcome, previewStdout)
	}
	pendingRoot, pendingSlug := intentArchiveCLIWorkspace(t)
	s7ASWritePendingArchiveFixture(t, pendingRoot, pendingSlug, 2)
	recoveryCode, recoveryStdout, _, _ := runPrepare(t,
		s7ASPurgeArgs(pendingRoot, pendingSlug, []string{"--all"}, false, true, true)...,
	)
	recoveryReport := decodeIntentArchivePurgeReport(t, recoveryStdout)
	if recoveryCode != 0 ||
		recoveryReport.Outcome != string(store.IntentArchivePurgeRecoveryRequired) ||
		recoveryReport.PendingPurge == nil ||
		recoveryReport.PendingPurge.RetryCWD != store.IntentArchiveRepairCWD {
		t.Fatalf("PIB-565 pending preview = exit:%d report:%#v\n%s",
			recoveryCode, recoveryReport, recoveryStdout)
	}

	// Every JSON example in this PRD and in ADR-035 that shows a purge report.
	documents := map[string]string{
		s7AVPRDRelPath: s7AVRepoDocument(t, s7AVPRDRelPath),
		s7AVADRRelPath: s7AVRepoDocument(t, s7AVADRRelPath),
	}
	examples, err := s7AXPurgeReportExamples(documents)
	if err != nil {
		t.Fatal(err)
	}
	if len(examples) != 2 {
		t.Fatalf("PIB-565 derived %d purge-report examples, want 2", len(examples))
	}
	for index, example := range examples {
		if err := s7AXValidatePurgeExample(
			fmt.Sprintf("PIB-565 example %d", index), example, preview, confirmed,
		); err != nil {
			t.Fatal(err)
		}
	}

	// The three JSON semantic sensitivity fixtures, judged in this body by the
	// same validator the two real examples above were judged by.
	jsonSensitivities := []struct {
		name     string
		label    string
		document string
		accepted string
	}{
		{
			name:     "successful-purge-reported-as-published",
			label:    "published fixture",
			document: `{"outcome": "published", "action": "none", "remaining_repairs": {"stages": []}}`,
			accepted: "a purge reported as a canonical publication",
		},
		{
			name:     "thirteenth-outcome-token-from-the-purge-path",
			label:    "extra-token fixture",
			document: `{"outcome": "purge-swept", "action": "none", "remaining_repairs": {"stages": []}}`,
			accepted: "a purge-only outcome token outside the closed sets",
		},
		{
			name:  "repair-cwd-spelled-with-an-underscore",
			label: "cwd fixture",
			document: `{"outcome": "purged", "action": "none", "remaining_repairs": {"stages": [` +
				`{"ordinal": 1, "kind": "purge-invocation", "class": "mixed-reference", "repair_cwd": "workspace_root"}]}}`,
			accepted: "repair_cwd spelled workspace_root",
		},
	}
	for _, sensitivity := range jsonSensitivities {
		var fixture map[string]any
		if err := json.Unmarshal([]byte(sensitivity.document), &fixture); err != nil {
			t.Fatalf("PIB-565 %s fixture is not valid JSON: %v", sensitivity.name, err)
		}
		if err := s7AXValidatePurgeExample(
			sensitivity.label, fixture, preview, confirmed,
		); err == nil {
			t.Fatalf("PIB-565 %s: validator accepted %s",
				sensitivity.name, sensitivity.accepted)
		}
	}
	if len(jsonSensitivities) != 3 {
		t.Fatalf("PIB-565 carries %d semantic sensitivity fixtures, want 3", len(jsonSensitivities))
	}

	// §10.2's `repaired_class`-presence rule, which PIB-564 states and this
	// validator now enforces on every example it judges. These fixtures are
	// **not** additions to PIB-565's pinned trio above — that stays at three —
	// they are the coherence bite the rev-19 erratum's worked-example
	// correction rests on.
	coherenceSensitivities := []struct {
		name     string
		label    string
		document string
		accepted string
	}{
		{
			// The exact shape §10.2 carried before rev-19: an exit-0 admitted
			// report whose plan still begins with the rank-1 corrupt-object
			// manual prerequisite that refuses every confirmed selector.
			name:  "purged-with-repaired-class-beside-a-corrupt-manual-stage",
			label: "impossible admitted-plus-prerequisite fixture",
			document: `{"outcome": "purged", "action": "none", "remaining_repairs": {` +
				`"rerun_required": true, "repaired_class": "unreferenced-residue", ` +
				`"stages_remaining": 2, "stages": [` +
				`{"ordinal": 1, "kind": "manual-prerequisite", "class": "corrupt-object", "repair_cwd": "workspace-root"}, ` +
				`{"ordinal": 2, "kind": "purge-invocation", "class": "dangling-reference", "repair_cwd": "workspace-root"}]}}`,
			accepted: "a repaired class beside a rank-1 corrupt-object manual prerequisite",
		},
		{
			name:  "refused-plan-carrying-a-repaired-class",
			label: "refusal-with-repaired-class fixture",
			document: `{"outcome": "refused", "action": "none", "remaining_repairs": {` +
				`"rerun_required": true, "repaired_class": "unreferenced-residue", ` +
				`"stages_remaining": 1, "stages": [` +
				`{"ordinal": 1, "kind": "purge-invocation", "class": "mixed-reference", "repair_cwd": "workspace-root"}]}}`,
			accepted: "repaired_class on a report the exit code already calls a refusal",
		},
	}
	for _, sensitivity := range coherenceSensitivities {
		var fixture map[string]any
		if err := json.Unmarshal([]byte(sensitivity.document), &fixture); err != nil {
			t.Fatalf("PIB-565 %s fixture is not valid JSON: %v", sensitivity.name, err)
		}
		if err := s7AXValidatePurgeExample(
			sensitivity.label, fixture, preview, confirmed,
		); err == nil {
			t.Fatalf("PIB-565 %s: validator accepted %s",
				sensitivity.name, sensitivity.accepted)
		}
	}
	// The rule bites the impossible combination, not the field: both coherent
	// forms of the same plan still pass.
	for _, control := range []struct {
		name     string
		document string
	}{
		{
			name: "admitted-form-with-no-prerequisite-left",
			document: `{"outcome": "purged", "action": "none", "remaining_repairs": {` +
				`"rerun_required": true, "repaired_class": "unreferenced-residue", ` +
				`"stages_remaining": 1, "stages": [` +
				`{"ordinal": 1, "kind": "purge-invocation", "class": "dangling-reference", "repair_cwd": "workspace-root"}]}}`,
		},
		{
			name: "refusal-form-carrying-the-prerequisite-and-no-repaired-class",
			document: `{"outcome": "refused", "action": "none", "remaining_repairs": {` +
				`"rerun_required": true, "stages_remaining": 2, "stages": [` +
				`{"ordinal": 1, "kind": "manual-prerequisite", "class": "corrupt-object", "repair_cwd": "workspace-root"}, ` +
				`{"ordinal": 2, "kind": "purge-invocation", "class": "dangling-reference", "repair_cwd": "workspace-root"}]}}`,
		},
	} {
		var fixture map[string]any
		if err := json.Unmarshal([]byte(control.document), &fixture); err != nil {
			t.Fatalf("PIB-565 %s control is not valid JSON: %v", control.name, err)
		}
		if err := s7AXValidatePurgeExample(
			"PIB-565 "+control.name, fixture, preview, confirmed,
		); err != nil {
			t.Fatalf("PIB-565 %s control was rejected: %v", control.name, err)
		}
	}
}

// ─── PIB-566 ──────────────────────────────────────────────────────────────────

// s7AXPendingEvidenceMarkers are the ways a function can come to hold a
// removal-pending reference set.
var s7AXPendingEvidenceMarkers = []string{
	"preparePendingArchiveHashes",
	"PendingIntentArchiveHashes",
	"doctorD9PendingHashesFromValidatedIndex",
	"PendingHashes",
	"IntentArchiveCodeRecoveryPending",
	"archive-purge-pending",
}

// s7AXOperatorRouteFields are the fields an emitter writes when it hands the
// operator a command line.
var s7AXOperatorRouteFields = []string{"Retry", "RetryCWD", "Remediation"}

// s7AXNarrowRouteBuilders name the exact observed pending set; the
// selector-preserving builder echoes the operator's own selector instead and is
// PIB-557's subject rather than PIB-566's.
var s7AXNarrowRouteBuilders = []string{
	"preparePendingPurgeCommand", "doctorD9BlobPurgeCommand",
}

var s7AXSelectorPreservingBuilders = []string{"intentArchivePurgeRetry"}

var s7AXAllSelectorToken = regexp.MustCompile(`--all([^0-9A-Za-z-]|$)`)

type s7AXPendingEmitters struct {
	narrow      []string
	preserving  []string
	nonBuilding []string
}

func s7AXDerivePendingEmitters(sources map[string]string, order []string) (s7AXPendingEmitters, error) {
	emitters := s7AXPendingEmitters{}
	program, err := s7AVParseProgram(sources, order)
	if err != nil {
		return emitters, err
	}
	functions := program.functions()
	names := []string{}
	for name := range functions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		function := functions[name]
		idents := s7AVIdentNames(function.Body)
		literals := map[string]bool{}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if ok && literal.Kind == token.STRING {
				if value, unquoteErr := strconv.Unquote(literal.Value); unquoteErr == nil {
					literals[value] = true
				}
			}
			return true
		})
		observesPending := false
		for _, marker := range s7AXPendingEvidenceMarkers {
			if idents[marker] != 0 || literals[marker] {
				observesPending = true
				break
			}
		}
		if !observesPending {
			continue
		}
		emitsRoute := false
		for _, field := range s7AXOperatorRouteFields {
			if idents[field] != 0 {
				emitsRoute = true
				break
			}
		}
		if !emitsRoute {
			continue
		}
		narrow := false
		for _, builder := range s7AXNarrowRouteBuilders {
			if len(s7AVCalls(function.Body, builder)) != 0 {
				narrow = true
			}
		}
		preserving := false
		for _, builder := range s7AXSelectorPreservingBuilders {
			if len(s7AVCalls(function.Body, builder)) != 0 {
				preserving = true
			}
		}
		switch {
		case narrow && preserving:
			return emitters, fmt.Errorf(
				"%s builds both a narrow and a selector-preserving route, so its pending route is undecidable", name,
			)
		case narrow:
			emitters.narrow = append(emitters.narrow, name)
		case preserving:
			emitters.preserving = append(emitters.preserving, name)
		default:
			// A function that observes a pending set and touches a route field
			// without calling any route builder can only delegate or render an
			// already-built route. It is only safe to classify that way when it
			// mints no purge command line of its own.
			for literal := range literals {
				if strings.Contains(literal, "intent-archive purge") {
					return emitters, fmt.Errorf(
						"%s mints a pending purge command line outside the derived builders", name,
					)
				}
			}
			emitters.nonBuilding = append(emitters.nonBuilding, name)
		}
	}
	return emitters, nil
}

// s7AXValidateNarrowBuilderSource proves the narrow builders emit exactly the
// repeated `--blob <h> --yes` form and can never widen.
func s7AXValidateNarrowBuilderSource(sources map[string]string, order []string) error {
	program, err := s7AVParseProgram(sources, order)
	if err != nil {
		return err
	}
	for _, name := range s7AXNarrowRouteBuilders {
		builder := program.function(name)
		if builder == nil {
			return fmt.Errorf("narrow pending-route builder %s is missing", name)
		}
		literals := []string{}
		ast.Inspect(builder.Body, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if ok && literal.Kind == token.STRING {
				if value, unquoteErr := strconv.Unquote(literal.Value); unquoteErr == nil {
					literals = append(literals, value)
				}
			}
			return true
		})
		want := []string{"tpatch feature intent-archive purge ", " --blob ", " --yes"}
		if !reflect.DeepEqual(literals, want) {
			return fmt.Errorf("%s emits literals %q, want %q", name, literals, want)
		}
		if len(s7AVCalls(builder.Body, "Strings"))+
			len(s7AVCalls(builder.Body, "doctorD9SortedUnique")) == 0 {
			return fmt.Errorf("%s does not sort the observed pending set", name)
		}
	}
	return nil
}

// s7AXValidatePendingRoute is the single validator PIB-566's real emitters and
// its three semantic sensitivity fixtures are judged by.
func s7AXValidatePendingRoute(
	label, slug, retry, retryCWD string,
	pending []string,
	requireCWD bool,
) error {
	if retry == "" {
		return fmt.Errorf("%s emitted no pending route", label)
	}
	if s7AXAllSelectorToken.MatchString(retry) {
		return fmt.Errorf("%s widened the pending route to --all: %q", label, retry)
	}
	if strings.Contains(retry, "--path") {
		return fmt.Errorf("%s inherited --path into the pending route: %q", label, retry)
	}
	argv, err := s7APParseRenderedCommand(retry)
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	sorted := append([]string(nil), pending...)
	sort.Strings(sorted)
	want := []string{"feature", "intent-archive", "purge", slug}
	for _, hash := range sorted {
		want = append(want, "--blob", hash)
	}
	want = append(want, "--yes")
	if !reflect.DeepEqual(argv, want) {
		return fmt.Errorf("%s pending route argv = %v, want %v", label, argv, want)
	}
	if requireCWD && retryCWD != store.IntentArchiveRepairCWD {
		return fmt.Errorf("%s pending route retry_cwd = %q, want %q",
			label, retryCWD, store.IntentArchiveRepairCWD)
	}
	return nil
}

func s7AXDoctorPendingRemediation(t *testing.T, root string) string {
	t.Helper()
	code, stdout, stderr, _ := runPrepare(t, "--path", root, "doctor", "--json", "--check", "D9")
	if code != 0 || stderr != "" {
		t.Fatalf("PIB-566 doctor exit=%d stderr=%q\n%s", code, stderr, stdout)
	}
	report := s7AXDecodeDoctorReport(t, stdout)
	for _, finding := range report.Findings {
		if finding.CheckID == "D9" && finding.Code == "archive-purge-pending" {
			return finding.Remediation
		}
	}
	t.Fatalf("PIB-566 doctor emitted no pending-purge finding\n%s", stdout)
	return ""
}

func TestS7AXPendingRouteNarrowingGuard(t *testing.T) {
	order := []string{s7AVCLIPrepareSource, s7AVCLIArchiveSource, s7AXDoctorSource}
	sources := s7AVRepoSources(t, order...)
	emitters, err := s7AXDerivePendingEmitters(sources, order)
	if err != nil {
		t.Fatalf("PIB-566 emitter derivation failed: %v", err)
	}
	wantNarrow := []string{
		"doctorD9ReportPendingHashes",
		"prepareStoreArchiveFailure",
		"runPrepareAbandon",
		"runPreparePublish",
	}
	wantPreserving := []string{
		"buildIntentArchivePendingPurge",
		"intentArchiveRefusalFromError",
		"runFeatureIntentArchivePurgeConfirmed",
	}
	wantNonBuilding := []string{"runDoctorD9FeatureArchive", "writeIntentArchivePurgeHuman"}
	if !reflect.DeepEqual(emitters.narrow, wantNarrow) ||
		!reflect.DeepEqual(emitters.preserving, wantPreserving) ||
		!reflect.DeepEqual(emitters.nonBuilding, wantNonBuilding) {
		t.Fatalf("PIB-566 derived emitters = %#v, want narrow %v / preserving %v / non-building %v",
			emitters, wantNarrow, wantPreserving, wantNonBuilding)
	}
	if err := s7AXValidateNarrowBuilderSource(sources, order); err != nil {
		t.Fatal(err)
	}

	for _, pendingCount := range []int{1, 2, 3} {
		root, slug := intentArchiveCLIWorkspace(t)
		fixture := s7ASWritePendingArchiveFixture(t, root, slug, pendingCount)
		label := fmt.Sprintf("PIB-566 pending-%d", pendingCount)

		for _, surface := range []struct {
			name string
			args []string
		}{
			{"prepare", []string{"--path", root, "prepare", slug, "--json", "--quiet"}},
			{"manual", []string{"--path", root, "prepare", slug, "--manual", "--json", "--quiet"}},
			{"regenerate", []string{"--path", root, "prepare", slug, "--regenerate", "--json", "--quiet"}},
			{"abandon", []string{
				"--path", root, "prepare", slug,
				"--abandon-transaction", "--yes", "--json", "--quiet",
			}},
		} {
			code, stdout, _, _ := runPrepare(t, surface.args...)
			if code != 3 {
				t.Fatalf("%s %s exit=%d, want 3\n%s", label, surface.name, code, stdout)
			}
			report := prepareS4Report(t, stdout)
			if report.Refusal == nil {
				t.Fatalf("%s %s emitted no refusal\n%s", label, surface.name, stdout)
			}
			wantCode := "recovery-pending"
			if surface.name == "abandon" {
				wantCode = "no-pending-transaction"
			}
			if report.Refusal.Code != wantCode {
				t.Fatalf("%s %s refusal code = %q, want %q",
					label, surface.name, report.Refusal.Code, wantCode)
			}
			if err := s7AXValidatePendingRoute(
				label+" "+surface.name, slug,
				report.Refusal.Retry, report.Refusal.RetryCWD,
				fixture.hashes, true,
			); err != nil {
				t.Fatal(err)
			}
		}

		// The doctor emitter renders the same narrow route as a remediation
		// line; per the accepted rev-17 correction it carries no retry_cwd pair.
		if err := s7AXValidatePendingRoute(
			label+" doctor", slug,
			s7AXDoctorPendingRemediation(t, root), "", fixture.hashes, false,
		); err != nil {
			t.Fatal(err)
		}
	}

	// The substitution is behaviour-preserving: the emitted narrow command and
	// --all --yes perform the identical terminal recovery, because the recovery
	// pass precedes selector processing and is selector-independent.
	narrowRoot, narrowSlug := intentArchiveCLIWorkspace(t)
	narrowFixture := s7ASWritePendingArchiveFixture(t, narrowRoot, narrowSlug, 2)
	wideRoot, wideSlug := intentArchiveCLIWorkspace(t)
	wideFixture := s7ASWritePendingArchiveFixture(t, wideRoot, wideSlug, 2)
	humanRoot, humanSlug := intentArchiveCLIWorkspace(t)
	humanFixture := s7ASWritePendingArchiveFixture(t, humanRoot, humanSlug, 2)
	if !reflect.DeepEqual(narrowFixture.hashes, wideFixture.hashes) ||
		!reflect.DeepEqual(narrowFixture.hashes, humanFixture.hashes) ||
		narrowSlug != wideSlug || narrowSlug != humanSlug {
		t.Fatalf("PIB-566 equivalence twins diverged: %v vs %v vs %v",
			narrowFixture.hashes, wideFixture.hashes, humanFixture.hashes)
	}
	_, narrowStdout, _, _ := runPrepare(
		t, "--path", narrowRoot, "prepare", narrowSlug, "--json", "--quiet",
	)
	narrowRefusal := prepareS4Report(t, narrowStdout)
	if narrowRefusal.Refusal == nil {
		t.Fatalf("PIB-566 equivalence twin emitted no refusal\n%s", narrowStdout)
	}
	emitted := narrowRefusal.Refusal.Retry
	if err := s7AXValidatePendingRoute(
		"PIB-566 equivalence emitted route", narrowSlug,
		emitted, narrowRefusal.Refusal.RetryCWD, narrowFixture.hashes, true,
	); err != nil {
		t.Fatal(err)
	}
	narrowArgv, err := s7APParseRenderedCommand(emitted)
	if err != nil {
		t.Fatal(err)
	}
	// The emitted route is the operator-facing command line and deliberately
	// carries no --json/--quiet pair, so the machine-readable observation runs
	// the same argv with that pair appended. Decoding the human writer's output
	// as JSON would fail on the writer's first line rather than on any
	// behavioural difference. The verbatim operator form is asserted separately
	// against its own twin below.
	for _, flag := range []string{"--json", "--quiet"} {
		for _, token := range narrowArgv {
			if token == flag {
				t.Fatalf("PIB-566 emitted operator route already carries %s: %v",
					flag, narrowArgv)
			}
		}
	}
	narrowJSONArgv := append(append([]string(nil), narrowArgv...), "--json", "--quiet")
	narrowCode, narrowOut, narrowErr := s7APRunFromWorkspace(t, narrowRoot, narrowJSONArgv)
	wideCode, wideOut, wideErr := s7APRunFromWorkspace(t, wideRoot, []string{
		"feature", "intent-archive", "purge", wideSlug, "--all", "--yes", "--json", "--quiet",
	})
	if narrowCode != 0 || narrowErr != "" || wideCode != 0 || wideErr != "" {
		t.Fatalf("PIB-566 equivalence exits = narrow:%d/%q wide:%d/%q\n%s\n%s",
			narrowCode, narrowErr, wideCode, wideErr, narrowOut, wideOut)
	}
	narrowReport := decodeIntentArchivePurgeReport(t, narrowOut)
	wideReport := decodeIntentArchivePurgeReport(t, wideOut)
	if narrowReport.Outcome != string(store.IntentArchivePurgeRecovered) ||
		wideReport.Outcome != string(store.IntentArchivePurgeRecovered) {
		t.Fatalf("PIB-566 equivalence outcomes = %q / %q",
			narrowReport.Outcome, wideReport.Outcome)
	}
	if narrowReport.Recovery == nil || wideReport.Recovery == nil {
		t.Fatalf("PIB-566 equivalence recoveries = %#v / %#v",
			narrowReport.Recovery, wideReport.Recovery)
	}
	narrowFinalized := s7AXSortedCopy(narrowReport.Recovery.FinalizedHashes)
	wideFinalized := s7AXSortedCopy(wideReport.Recovery.FinalizedHashes)
	if !reflect.DeepEqual(narrowFinalized, s7AXSortedCopy(narrowFixture.hashes)) ||
		!reflect.DeepEqual(wideFinalized, narrowFinalized) {
		t.Fatalf("PIB-566 equivalence finalized sets = %v / %v, want %v",
			narrowFinalized, wideFinalized, narrowFixture.hashes)
	}
	if len(narrowReport.Hashes) != 0 || len(wideReport.Hashes) != 0 ||
		len(narrowReport.Blobs) != 0 || len(wideReport.Blobs) != 0 {
		t.Fatalf("PIB-566 a recovery processed its selector: narrow=%#v wide=%#v",
			narrowReport.Hashes, wideReport.Hashes)
	}

	// Human parity, asserted on its own twin: the operator pastes the emitted
	// line verbatim — no --json, no --quiet — and the human writer reports the
	// identical terminal recovery over the identical finalized set.
	humanCode, humanOut, humanErr := s7APRunFromWorkspace(t, humanRoot, narrowArgv)
	if humanCode != 0 || humanErr != "" {
		t.Fatalf("PIB-566 verbatim emitted route = exit:%d stderr:%q\n%s",
			humanCode, humanErr, humanOut)
	}
	wantHumanHeadline := fmt.Sprintf("%s %s: %s",
		intentArchiveCommandPurge, humanSlug, string(store.IntentArchivePurgeRecovered))
	if !strings.Contains(humanOut, wantHumanHeadline) ||
		!strings.Contains(humanOut, "Recovered pending purge state.") {
		t.Fatalf("PIB-566 verbatim emitted route did not report %q:\n%s",
			wantHumanHeadline, humanOut)
	}
	for _, hash := range humanFixture.hashes {
		if !strings.Contains(humanOut, "finalized "+hash) {
			t.Fatalf("PIB-566 verbatim emitted route omitted finalized %s:\n%s", hash, humanOut)
		}
	}
	for _, root := range []string{narrowRoot, wideRoot, humanRoot} {
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", narrowSlug, "--json", "--quiet")
		if code != 0 || stderr != "" {
			t.Fatalf("PIB-566 recovered workspace did not proceed: exit=%d stderr=%q\n%s",
				code, stderr, stdout)
		}
	}

	sensitivitySlug := "pending-route"
	hashes := []string{
		strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64),
	}
	narrowRoute := "tpatch feature intent-archive purge " + sensitivitySlug +
		" --blob " + hashes[0] + " --blob " + hashes[1] + " --blob " + hashes[2] + " --yes"
	if err := s7AXValidatePendingRoute(
		"PIB-566 control", sensitivitySlug, narrowRoute,
		store.IntentArchiveRepairCWD, hashes, true,
	); err != nil {
		t.Fatalf("PIB-566 control route was rejected: %v", err)
	}
	for _, fixture := range []struct {
		name     string
		retry    string
		retryCWD string
	}{
		{
			name:     "widens-to-all-when-more-than-one-hash-is-pending",
			retry:    "tpatch feature intent-archive purge " + sensitivitySlug + " --all --yes",
			retryCWD: store.IntentArchiveRepairCWD,
		},
		{
			name: "names-only-the-first-pending-hash",
			retry: "tpatch feature intent-archive purge " + sensitivitySlug +
				" --blob " + hashes[0] + " --yes",
			retryCWD: store.IntentArchiveRepairCWD,
		},
		{
			name:     "omits-retry-cwd",
			retry:    narrowRoute,
			retryCWD: "",
		},
	} {
		if err := s7AXValidatePendingRoute(
			"PIB-566 "+fixture.name, sensitivitySlug,
			fixture.retry, fixture.retryCWD, hashes, true,
		); err == nil {
			t.Fatalf("PIB-566 %s: pending-route validator accepted wrong input", fixture.name)
		}
	}
}

// ─── PIB-567 ──────────────────────────────────────────────────────────────────

var (
	s7AXMatrixRowPattern   = regexp.MustCompile(`^\|\s*(PIB-[0-9]{3})\s*\|\s*([ICGUS])\s*\|`)
	s7AXFixtureRowPattern  = regexp.MustCompile(`^\|\s*(PIB-[0-9]{3})\s+[^|]*\|`)
	s7AXRevisionRowPattern = regexp.MustCompile(`^\| rev-([0-9]+) \|`)
	s7AXLedgerIDPattern    = regexp.MustCompile("`(PIB-[0-9]{3})`")
	s7AXBulletRunPattern   = regexp.MustCompile(
		"\\):\\s*((?:`PIB-[0-9]{3}`(?:,\\s+|\\s+and\\s+)?)+)")
	s7AXProseRunPattern = regexp.MustCompile(
		"amends?\\s+exactly\\s+([a-z]+)\\s+(?:stable\\s+)?rows?[^:]*:\\s*" +
			"((?:`PIB-[0-9]{3}`(?:,\\s+|\\s+and\\s+)?)+)")
	s7AXNewRowsPattern = regexp.MustCompile(
		"(?:\\*\\*)?([0-9]+)(?:\\*\\*)?\\s+(?:rows\\s+)?are new in rev-([0-9]+)\\s*" +
			"\\(`PIB-([0-9]{3})`…`PIB-([0-9]{3})`\\)")
	s7AXCountWords = map[string]int{
		"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6,
		"seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11,
		"twelve": 12, "thirteen": 13, "fourteen": 14, "fifteen": 15,
	}
)

// s7AXRevisionWindow is one revision's real commit range, derived from the
// repository rather than named in a test constant: the writer commit is the one
// that first carries the revision's own history row, the base is the reviewed
// tip immediately before it, and the tip is the last commit before the next
// revision's writer commit.
type s7AXRevisionWindow struct {
	revision int
	writer   string
	base     string
	tip      string
}

func s7AXGitCommits(t *testing.T, root, rel string) []string {
	t.Helper()
	command := exec.Command("git", "log", "--reverse", "--format=%H", "--", rel)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git log %s: %v", rel, err)
	}
	commits := strings.Fields(string(output))
	if len(commits) < 2 {
		t.Fatalf("git history for %s has %d commits; the guard needs the full history", rel, len(commits))
	}
	return commits
}

func s7AXRevisionsIn(document string) map[int]bool {
	present := map[int]bool{}
	for _, line := range strings.Split(document, "\n") {
		match := s7AXRevisionRowPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if value, err := strconv.Atoi(match[1]); err == nil {
			present[value] = true
		}
	}
	return present
}

func s7AXRevisionWindows(t *testing.T, root, rel string, from int) []s7AXRevisionWindow {
	t.Helper()
	commits := s7AXGitCommits(t, root, rel)
	introducer := map[int]int{}
	for index, commit := range commits {
		for revision := range s7AXRevisionsIn(string(s7GitFileAt(t, root, commit, rel))) {
			if _, seen := introducer[revision]; !seen {
				introducer[revision] = index
			}
		}
	}
	revisions := []int{}
	for revision := range introducer {
		revisions = append(revisions, revision)
	}
	sort.Ints(revisions)
	windows := []s7AXRevisionWindow{}
	for position, revision := range revisions {
		if revision < from {
			continue
		}
		writerIndex := introducer[revision]
		if writerIndex == 0 {
			t.Fatalf("rev-%d has no reviewed predecessor commit", revision)
		}
		tipIndex := len(commits) - 1
		if position+1 < len(revisions) {
			tipIndex = introducer[revisions[position+1]] - 1
		}
		if tipIndex < writerIndex {
			t.Fatalf("rev-%d window is empty: writer=%d tip=%d", revision, writerIndex, tipIndex)
		}
		windows = append(windows, s7AXRevisionWindow{
			revision: revision,
			writer:   commits[writerIndex],
			base:     commits[writerIndex-1],
			tip:      commits[tipIndex],
		})
	}
	if len(windows) == 0 {
		t.Fatalf("no revision window from rev-%d onward", from)
	}
	return windows
}

type s7AXDocumentRows struct {
	matrix   map[string]string
	fixtures map[string]string
}

func s7AXReadDocumentRows(document string) s7AXDocumentRows {
	rows := s7AXDocumentRows{matrix: map[string]string{}, fixtures: map[string]string{}}
	for _, line := range strings.Split(document, "\n") {
		trimmed := strings.TrimRight(line, " \t")
		if match := s7AXMatrixRowPattern.FindStringSubmatch(trimmed); match != nil {
			rows.matrix[match[1]] = trimmed
			continue
		}
		if match := s7AXFixtureRowPattern.FindStringSubmatch(trimmed); match != nil {
			rows.fixtures[match[1]] += trimmed + "\n"
		}
	}
	return rows
}

type s7AXDiffLedger struct {
	amended   []string
	added     []string
	fixtureOn []string
}

func s7AXDiffRows(before, after s7AXDocumentRows) s7AXDiffLedger {
	ledger := s7AXDiffLedger{amended: []string{}, added: []string{}, fixtureOn: []string{}}
	changed := map[string]bool{}
	for id, text := range after.matrix {
		previous, existed := before.matrix[id]
		switch {
		case !existed:
			ledger.added = append(ledger.added, id)
			changed[id] = true
		case previous != text:
			ledger.amended = append(ledger.amended, id)
			changed[id] = true
		}
	}
	for id := range before.matrix {
		if _, present := after.matrix[id]; !present {
			ledger.amended = append(ledger.amended, id)
			changed[id] = true
		}
	}
	for id, text := range after.fixtures {
		if before.fixtures[id] != text && !changed[id] {
			ledger.fixtureOn = append(ledger.fixtureOn, id)
		}
	}
	for id := range before.fixtures {
		if _, present := after.fixtures[id]; !present && !changed[id] {
			ledger.fixtureOn = append(ledger.fixtureOn, id)
		}
	}
	sort.Strings(ledger.amended)
	sort.Strings(ledger.added)
	sort.Strings(ledger.fixtureOn)
	return ledger
}

func s7AXStripParentheses(text string) string {
	builder := strings.Builder{}
	depth := 0
	for _, character := range text {
		switch character {
		case '(':
			depth++
			continue
		case ')':
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth == 0 {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func s7AXSortedUniqueIDs(ids []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// s7AXSection181Bounds returns §18.1's bounds in one document.
func s7AXSection181Bounds(document string) (int, int, bool) {
	start := strings.Index(document, "### 18.1 How to read this matrix")
	end := strings.Index(document, "### 18.2 A —")
	if start < 0 || end <= start {
		return 0, 0, false
	}
	return start, end, true
}

// s7AXLedgerClaim is one revision's §18.1 amended-row claim, located in the
// document's own bytes rather than pinned to one revision's prose: the declared
// count word's span, the ID run's span, and the IDs the shipped ledger reader
// takes from that run.
type s7AXLedgerClaim struct {
	revision  int
	countWord string
	countSpan [2]int
	runSpan   [2]int
	ids       []string
}

// s7AXLedgerRunToken is one claimed row, with the optional parenthetical gloss
// §18.1 sometimes attaches to it. The shipped reader strips parentheses before
// it reads IDs, so this locator does too.
const s7AXLedgerRunToken = "`PIB-[0-9]{3}`(?:\\s*\\([^()]*\\))?"

var (
	s7AXLedgerHeadPattern = regexp.MustCompile(
		"(amends?\\s+exactly\\s+)([a-z]+)(\\s+(?:stable\\s+)?rows?[^:]*:\\s*)")
	s7AXLedgerRunPattern = regexp.MustCompile(
		"^" + s7AXLedgerRunToken +
			"(?:(?:,\\s+|\\s+and\\s+)" + s7AXLedgerRunToken + ")*")
)

// s7AXCountWord reverses §18.1's spelled-out counts.
func s7AXCountWord(count int) (string, bool) {
	for word, value := range s7AXCountWords {
		if value == count {
			return word, true
		}
	}
	return "", false
}

// s7AXLocateSection181Claim locates one revision's §18.1 prose ledger inside a
// document, so a sensitivity fixture can mutate a row that revision really
// claims. It reads the paragraph the shipped ledger reader reads, and declines
// the bulleted shape, which carries no single count word to keep consistent.
func s7AXLocateSection181Claim(document string, revision int) (s7AXLedgerClaim, bool) {
	start, end, bounded := s7AXSection181Bounds(document)
	if !bounded {
		return s7AXLedgerClaim{}, false
	}
	section := document[start:end]
	offset := strings.Index(section, fmt.Sprintf("**Rev-%d ", revision))
	if offset < 0 {
		return s7AXLedgerClaim{}, false
	}
	paragraph := section[offset:]
	if next := strings.Index(paragraph, fmt.Sprintf("**Rev-%d ", revision+1)); next > 0 {
		paragraph = paragraph[:next]
	}
	head := s7AXLedgerHeadPattern.FindStringSubmatchIndex(paragraph)
	if head == nil {
		return s7AXLedgerClaim{}, false
	}
	runStart := head[1]
	run := s7AXLedgerRunPattern.FindString(paragraph[runStart:])
	if run == "" {
		return s7AXLedgerClaim{}, false
	}
	ids := []string{}
	for _, match := range s7AXLedgerIDPattern.FindAllStringSubmatch(
		s7AXStripParentheses(run), -1,
	) {
		ids = append(ids, match[1])
	}
	base := start + offset
	return s7AXLedgerClaim{
		revision:  revision,
		countWord: paragraph[head[4]:head[5]],
		countSpan: [2]int{base + head[4], base + head[5]},
		runSpan:   [2]int{base + runStart, base + runStart + len(run)},
		ids:       s7AXSortedUniqueIDs(ids),
	}, true
}

// s7AXRenderLedgerIDs renders an amended-row list in §18.1's own shape.
func s7AXRenderLedgerIDs(ids []string) string {
	quoted := []string{}
	for _, id := range s7AXSortedUniqueIDs(ids) {
		quoted = append(quoted, "`"+id+"`")
	}
	switch len(quoted) {
	case 0:
		return ""
	case 1:
		return quoted[0]
	}
	return strings.Join(quoted[:len(quoted)-1], ", ") + " and " + quoted[len(quoted)-1]
}

// s7AXRewriteSection181Claim rewrites one located claim to a different
// amended-row list, keeping the spelled-out count consistent with the list it
// renders so the mutated paragraph stays readable Markdown the shipped ledger
// reader accepts.
func s7AXRewriteSection181Claim(
	document string, claim s7AXLedgerClaim, ids []string,
) (string, error) {
	if claim.countSpan[1] > claim.runSpan[0] {
		return "", fmt.Errorf("rev-%d's located count word and ID run overlap", claim.revision)
	}
	unique := s7AXSortedUniqueIDs(ids)
	rendered := s7AXRenderLedgerIDs(unique)
	if rendered == "" {
		return "", fmt.Errorf(
			"a rev-%d ledger fixture must still claim at least one row", claim.revision)
	}
	word, known := s7AXCountWord(len(unique))
	if !known {
		return "", fmt.Errorf("§18.1 has no spelled-out count for %d rows", len(unique))
	}
	mutated := document[:claim.runSpan[0]] + rendered + document[claim.runSpan[1]:]
	return mutated[:claim.countSpan[0]] + word + mutated[claim.countSpan[1]:], nil
}

// s7AXRevisionRowLedger reads the amended-row list a revision's own history row
// claims, stopping at the bolded count so the row's later prose about other IDs
// is not read as part of the claim.
func s7AXRevisionRowLedger(document string, revision int) ([]string, int, bool) {
	row := s7RevisionRow(document, revision)
	if row == "" {
		return nil, 0, false
	}
	marker := strings.Index(row, "Amended rows, exactly")
	if marker < 0 {
		return nil, 0, false
	}
	rest := row[marker:]
	count := 0
	end := strings.Index(rest, " — **")
	if end >= 0 {
		if word := regexp.MustCompile(`^ — \*\*([a-z]+)\*\*`).FindStringSubmatch(rest[end:]); word != nil {
			count = s7AXCountWords[word[1]]
		}
		rest = rest[:end]
	}
	if colon := strings.Index(rest, ":"); colon >= 0 {
		rest = rest[colon+1:]
	}
	ids := []string{}
	for _, match := range s7AXLedgerIDPattern.FindAllStringSubmatch(rest, -1) {
		ids = append(ids, match[1])
	}
	return s7AXSortedUniqueIDs(ids), count, true
}

// s7AXSection181Ledger reads the amended-row list §18.1 claims for one revision,
// in either the bulleted or the prose shape.
func s7AXSection181Ledger(document string, revision int) ([]string, int, bool) {
	start, end, bounded := s7AXSection181Bounds(document)
	if !bounded {
		return nil, 0, false
	}
	section := document[start:end]
	offset := strings.Index(section, fmt.Sprintf("**Rev-%d ", revision))
	if offset < 0 {
		return nil, 0, false
	}
	paragraph := section[offset:]
	if next := strings.Index(paragraph, fmt.Sprintf("**Rev-%d ", revision+1)); next > 0 {
		paragraph = paragraph[:next]
	}
	bullets := strings.Split(paragraph, "\n- ")
	if len(bullets) > 1 {
		ids := []string{}
		for _, bullet := range bullets[1:] {
			flat := strings.Join(strings.Fields(bullet), " ")
			run := s7AXBulletRunPattern.FindStringSubmatch(flat)
			if run == nil {
				continue
			}
			for _, match := range s7AXLedgerIDPattern.FindAllStringSubmatch(run[1], -1) {
				ids = append(ids, match[1])
			}
		}
		return s7AXSortedUniqueIDs(ids), 0, true
	}
	flat := s7AXStripParentheses(strings.Join(strings.Fields(paragraph), " "))
	run := s7AXProseRunPattern.FindStringSubmatch(flat)
	if run == nil {
		return nil, 0, false
	}
	ids := []string{}
	for _, match := range s7AXLedgerIDPattern.FindAllStringSubmatch(run[2], -1) {
		ids = append(ids, match[1])
	}
	return s7AXSortedUniqueIDs(ids), s7AXCountWords[run[1]], true
}

// s7AXValidateRevisionLedger is the single validator PIB-567's real revisions,
// its two historical driving defects and its three sensitivity fixtures are all
// judged by. It fails in both directions.
func s7AXValidateRevisionLedger(
	revision int,
	document string,
	diff s7AXDiffLedger,
) error {
	section181, count181, has181 := s7AXSection181Ledger(document, revision)
	if !has181 {
		return fmt.Errorf("rev-%d states no §18.1 amended-row ledger", revision)
	}
	ledgers := map[string][]string{"§18.1": section181}
	counts := map[string]int{"§18.1": count181}
	if row, countRow, hasRow := s7AXRevisionRowLedger(document, revision); hasRow {
		ledgers["revision-history row"] = row
		counts["revision-history row"] = countRow
	}
	names := []string{}
	for name := range ledgers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		claimed := ledgers[name]
		for _, id := range claimed {
			found := false
			for _, amended := range diff.amended {
				if amended == id {
					found = true
				}
			}
			if found {
				continue
			}
			detail := ""
			for _, fixture := range diff.fixtureOn {
				if fixture == id {
					detail = " (the diff touches it only in its §18.53 semantic-fixture entry)"
				}
			}
			return fmt.Errorf(
				"rev-%d %s claims %s as an amended matrix row, but its diff did not change that row%s",
				revision, name, id, detail,
			)
		}
		for _, id := range diff.amended {
			found := false
			for _, claim := range claimed {
				if claim == id {
					found = true
				}
			}
			if !found {
				return fmt.Errorf(
					"rev-%d %s omits %s, which its diff did change", revision, name, id,
				)
			}
		}
		if counts[name] != 0 && counts[name] != len(claimed) {
			return fmt.Errorf(
				"rev-%d %s declares %d amended rows and lists %d",
				revision, name, counts[name], len(claimed),
			)
		}
	}
	return nil
}

// s7AXNewRowRanges reads §18.52's per-revision new-row ranges.
func s7AXNewRowRanges(document string) (map[int][3]int, error) {
	start := strings.Index(document, "### 18.52 Counts, kinds and slice partition")
	end := strings.Index(document, "### 18.53 Sensitivity requirement")
	if start < 0 || end <= start {
		return nil, errors.New("§18.52 is missing")
	}
	section := strings.Join(strings.Fields(document[start:end]), " ")
	ranges := map[int][3]int{}
	for _, match := range s7AXNewRowsPattern.FindAllStringSubmatch(section, -1) {
		declared, _ := strconv.Atoi(match[1])
		revision, _ := strconv.Atoi(match[2])
		first, _ := strconv.Atoi(match[3])
		last, _ := strconv.Atoi(match[4])
		ranges[revision] = [3]int{declared, first, last}
	}
	if len(ranges) == 0 {
		return nil, errors.New("§18.52 declares no new-row ranges")
	}
	return ranges, nil
}

// s7AXNormativeTokenScan implements the "corrected throughout outside
// quoted/meta references" claim: an occurrence of the corrected token is
// exempt only where it is directly enclosed in quotes, which is how every
// preserved reference in the revision history, §18.1 and PIB-567's own guard
// text carries it.
func s7AXNormativeTokenScan(label, document, corrected string) error {
	pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(corrected) + `\b`)
	openers := "\u201c\u2018\"'`"
	closers := "\u201d\u2019\"'`"
	for _, match := range pattern.FindAllStringIndex(document, -1) {
		before := ""
		if match[0] > 0 {
			previous, _ := utf8.DecodeLastRuneInString(document[:match[0]])
			before = string(previous)
		}
		after := ""
		if match[1] < len(document) {
			next, _ := utf8.DecodeRuneInString(document[match[1]:])
			after = string(next)
		}
		if before != "" && after != "" &&
			strings.Contains(openers, before) && strings.Contains(closers, after) {
			continue
		}
		start := match[0] - 90
		if start < 0 {
			start = 0
		}
		return fmt.Errorf(
			"%s still carries a normative %q outside a quoted or meta reference: %q",
			label, corrected, document[start:match[1]],
		)
	}
	return nil
}

func TestS7AXRevisionLedgerGuard(t *testing.T) {
	root := avpRepoRoot(t)
	current := s7AVRepoDocument(t, s7AVPRDRelPath)
	windows := s7AXRevisionWindows(t, root, s7AVPRDRelPath, 12)
	if len(windows) < 7 {
		t.Fatalf("PIB-567 derived %d revision windows from rev-12 onward, want at least 7", len(windows))
	}

	newRows, err := s7AXNewRowRanges(current)
	if err != nil {
		t.Fatal(err)
	}
	rows, categoryCounts, err := s7ParseFullMatrix(current)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 567 {
		t.Fatalf("PIB-567 shipped matrix rows = %d, want 567", len(rows))
	}
	if err := s7ValidateSection1852(current, rows, categoryCounts); err != nil {
		t.Fatal(err)
	}

	anchor := regexp.MustCompile("rev-([0-9]+) diff `([0-9a-f]{7,40})`→(?:that revision's writer tip )?`([0-9a-f]{7,40})`")
	anchors := map[int][2]string{}
	for _, match := range anchor.FindAllStringSubmatch(current, -1) {
		revision, _ := strconv.Atoi(match[1])
		anchors[revision] = [2]string{match[2], match[3]}
	}
	if len(anchors) < 2 {
		t.Fatalf("PIB-567 found %d documented diff anchors, want the rev-12 and rev-13 pairs", len(anchors))
	}

	windowDiffs := map[int]s7AXDiffLedger{}
	for _, window := range windows {
		before := s7AXReadDocumentRows(string(s7GitFileAt(t, root, window.base, s7AVPRDRelPath)))
		after := s7AXReadDocumentRows(string(s7GitFileAt(t, root, window.tip, s7AVPRDRelPath)))
		diff := s7AXDiffRows(before, after)
		windowDiffs[window.revision] = diff
		if err := s7AXValidateRevisionLedger(window.revision, current, diff); err != nil {
			t.Fatalf("PIB-567 %v: %v", window, err)
		}
		if pair, documented := anchors[window.revision]; documented {
			if !strings.HasPrefix(window.base, pair[0]) || !strings.HasPrefix(window.tip, pair[1]) {
				t.Fatalf("PIB-567 rev-%d derived window %s..%s does not match the documented diff %s→%s",
					window.revision, window.base[:7], window.tip[:7], pair[0], pair[1])
			}
		}
		declared, hasRange := newRows[window.revision]
		if !hasRange {
			if len(diff.added) != 0 {
				t.Fatalf("PIB-567 rev-%d added %v with no §18.52 new-row range",
					window.revision, diff.added)
			}
			continue
		}
		count, first, last := declared[0], declared[1], declared[2]
		if last-first+1 != count {
			t.Fatalf("PIB-567 rev-%d new-row range PIB-%03d…PIB-%03d is not contiguous over %d rows",
				window.revision, first, last, count)
		}
		wantAdded := []string{}
		for id := first; id <= last; id++ {
			wantAdded = append(wantAdded, fmt.Sprintf("PIB-%03d", id))
		}
		if !reflect.DeepEqual(diff.added, wantAdded) {
			t.Fatalf("PIB-567 rev-%d added %v, want the declared contiguous range %v",
				window.revision, diff.added, wantAdded)
		}
		for _, added := range wantAdded {
			for _, amended := range diff.amended {
				if added == amended {
					t.Fatalf("PIB-567 rev-%d lists %s as both new and amended", window.revision, added)
				}
			}
		}
	}

	// ── The uncommitted-revision boundary ────────────────────────────────────
	//
	// s7AXRevisionWindows derives its windows from commits, so a revision the
	// current document declares that no commit carries yet has **no writer
	// diff**, and inventing a SHA for it would be exactly the false claim this
	// row exists to catch. The boundary is therefore made explicit and guarded
	// rather than passed over: the uncommitted set must be a suffix of the
	// revision history and must hold at most one revision, so the one honest
	// surrogate — the newest committed tip of this document versus the working
	// tree — is unambiguously attributable to it. That surrogate is judged by
	// the same validator the committed windows are. The moment the pending
	// revision is committed it becomes an ordinary window above, validated
	// against its real writer diff with no further change here.
	windowed := map[int]bool{}
	newestWindowed := 0
	for _, window := range windows {
		windowed[window.revision] = true
		if window.revision > newestWindowed {
			newestWindowed = window.revision
		}
	}
	declaredRevisions := []int{}
	for revision := range s7AXRevisionsIn(current) {
		if revision >= 12 {
			declaredRevisions = append(declaredRevisions, revision)
		}
	}
	sort.Ints(declaredRevisions)
	pending := []int{}
	for _, revision := range declaredRevisions {
		if windowed[revision] {
			continue
		}
		if revision < newestWindowed {
			t.Fatalf("PIB-567 rev-%d is declared but uncommitted while the later rev-%d is committed: "+
				"the uncommitted set must be a suffix of the revision history",
				revision, newestWindowed)
		}
		pending = append(pending, revision)
	}
	if len(pending) > 1 {
		t.Fatalf("PIB-567 %d revisions %v are declared but uncommitted; the working-tree surrogate "+
			"cannot attribute its row changes to one of them, so at most one may be pending",
			len(pending), pending)
	}
	for _, revision := range pending {
		claim, located := s7AXLocateSection181Claim(current, revision)
		if !located || len(claim.ids) == 0 {
			t.Fatalf("PIB-567 uncommitted rev-%d states no readable §18.1 amended-row ledger", revision)
		}
		ids, count, readable := s7AXSection181Ledger(current, revision)
		if !readable || count != len(ids) || !reflect.DeepEqual(ids, claim.ids) {
			t.Fatalf("PIB-567 uncommitted rev-%d §18.1 ledger %v/%d disagrees with its located claim %v",
				revision, ids, count, claim.ids)
		}
		for _, id := range ids {
			if _, present := rows[id]; !present {
				t.Fatalf("PIB-567 uncommitted rev-%d claims %s, which the shipped matrix does not contain",
					revision, id)
			}
		}
		if declared, hasRange := newRows[revision]; hasRange {
			t.Fatalf("PIB-567 uncommitted rev-%d declares the §18.52 new-row range %v; an erratum adds none",
				revision, declared)
		}
		committedTip := windows[len(windows)-1].tip
		before := s7AXReadDocumentRows(string(s7GitFileAt(t, root, committedTip, s7AVPRDRelPath)))
		diff := s7AXDiffRows(before, s7AXReadDocumentRows(current))
		if len(diff.added) != 0 {
			t.Fatalf("PIB-567 uncommitted rev-%d adds matrix rows %v; an erratum adds none",
				revision, diff.added)
		}
		if err := s7AXValidateRevisionLedger(revision, current, diff); err != nil {
			t.Fatalf("PIB-567 uncommitted rev-%d, working tree versus committed tip %s: %v",
				revision, committedTip[:7], err)
		}
		windowDiffs[revision] = diff
		t.Logf("PIB-567 rev-%d has no writer commit yet: its ledger %v is validated against the "+
			"working tree versus committed tip %s, not against a writer diff; the writer-diff check "+
			"applies automatically once it is committed", revision, ids, committedTip[:7])
	}

	// The normative "triple"→"tuple" correction, scanned rather than trusted.
	for label, document := range map[string]string{
		s7AVPRDRelPath: current,
		s7AVADRRelPath: s7AVRepoDocument(t, s7AVADRRelPath),
	} {
		if err := s7AXNormativeTokenScan(label, document, "triple"); err != nil {
			t.Fatal(err)
		}
	}

	// Driving fixture 1: rev-12's own ledger claimed PIB-548 while the rev-12
	// diff did not touch that row.
	// Driving fixture 2: rev-13's own ledgers listed PIB-524 as an amended
	// matrix row while the rev-13 diff changed it only inside its §18.53
	// semantic-fixture entry.
	for _, defect := range []struct {
		revision int
		id       string
		fixture  bool
	}{
		{revision: 12, id: "PIB-548", fixture: false},
		{revision: 13, id: "PIB-524", fixture: true},
	} {
		window := s7AXRevisionWindow{}
		for _, candidate := range windows {
			if candidate.revision == defect.revision {
				window = candidate
			}
		}
		if window.writer == "" {
			t.Fatalf("PIB-567 has no window for rev-%d", defect.revision)
		}
		historical := string(s7GitFileAt(t, root, window.writer, s7AVPRDRelPath))
		before := s7AXReadDocumentRows(string(s7GitFileAt(t, root, window.base, s7AVPRDRelPath)))
		after := s7AXReadDocumentRows(historical)
		diff := s7AXDiffRows(before, after)
		err := s7AXValidateRevisionLedger(defect.revision, historical, diff)
		if err == nil {
			t.Fatalf("PIB-567 accepted rev-%d's historical ledger defect", defect.revision)
		}
		if !strings.Contains(err.Error(), defect.id) {
			t.Fatalf("PIB-567 rev-%d defect was not attributed to %s: %v",
				defect.revision, defect.id, err)
		}
		if defect.fixture && !strings.Contains(err.Error(), "§18.53") {
			t.Fatalf("PIB-567 did not distinguish a fixture-only touch from a matrix-row amendment: %v", err)
		}
	}

	// Sensitivity fixtures over the current document, derived from a real
	// revision instead of from one revision's prose: the newest settled window
	// whose §18.1 ledger is in the amendable prose shape and claims at least two
	// rows, so both directions stay constructible as later revisions — the
	// planned rev-19 erratum among them — are appended.
	reference := s7AXRevisionWindow{}
	referenceClaim := s7AXLedgerClaim{}
	referenceDiff := s7AXDiffLedger{}
	for index := len(windows) - 2; index >= 0; index-- {
		candidate := windows[index]
		claim, located := s7AXLocateSection181Claim(current, candidate.revision)
		if !located || len(claim.ids) < 2 {
			continue
		}
		diff := windowDiffs[candidate.revision]
		if len(diff.amended) < 2 {
			continue
		}
		reference, referenceClaim, referenceDiff = candidate, claim, diff
		break
	}
	if reference.writer == "" {
		t.Fatal("PIB-567 found no settled revision whose §18.1 ledger claims two or more amended rows")
	}
	referenceRevision := reference.revision

	// The located claim is the one the shipped reader reads, so a fixture built
	// on it mutates the ledger the validator will judge.
	shippedIDs, shippedCount, readable := s7AXSection181Ledger(current, referenceRevision)
	if !readable || !reflect.DeepEqual(shippedIDs, referenceClaim.ids) ||
		shippedCount != len(referenceClaim.ids) {
		t.Fatalf("PIB-567 rev-%d located ledger %v/%s disagrees with the shipped reader %v/%d (readable=%t)",
			referenceRevision, referenceClaim.ids, referenceClaim.countWord,
			shippedIDs, shippedCount, readable)
	}
	if !reflect.DeepEqual(referenceClaim.ids, referenceDiff.amended) {
		t.Fatalf("PIB-567 rev-%d claims %v but its diff amended %v",
			referenceRevision, referenceClaim.ids, referenceDiff.amended)
	}
	if err := s7AXValidateRevisionLedger(referenceRevision, current, referenceDiff); err != nil {
		t.Fatalf("PIB-567 reference revision failed before mutation: %v", err)
	}

	// The over-claimed row is a real matrix row the reference revision neither
	// claims nor touches, so the fixture reproduces rev-12's PIB-548 defect in
	// whatever shape the current document happens to carry.
	involved := map[string]bool{}
	for _, group := range [][]string{
		referenceClaim.ids, referenceDiff.amended,
		referenceDiff.added, referenceDiff.fixtureOn,
	} {
		for _, id := range group {
			involved[id] = true
		}
	}
	candidates := []string{}
	for id := range rows {
		if !involved[id] {
			candidates = append(candidates, id)
		}
	}
	sort.Strings(candidates)
	if len(candidates) == 0 {
		t.Fatalf("PIB-567 rev-%d touches every matrix row, so no over-claim is constructible",
			referenceRevision)
	}
	overClaimed := candidates[0]
	omitted := referenceClaim.ids[len(referenceClaim.ids)-1]

	overClaimedIDs := append(append([]string{}, referenceClaim.ids...), overClaimed)
	underClaimedIDs := referenceClaim.ids[:len(referenceClaim.ids)-1]
	overClaimedDocument, err := s7AXRewriteSection181Claim(
		current, referenceClaim, overClaimedIDs,
	)
	if err != nil {
		t.Fatal(err)
	}
	underClaimedDocument, err := s7AXRewriteSection181Claim(
		current, referenceClaim, underClaimedIDs,
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, fixture := range []struct {
		name     string
		document string
		wantIDs  []string
		blames   string
	}{
		{
			// A ledger listing an amended row the revision's diff did not
			// change: rev-12's own PIB-548 defect, reproduced in the current
			// document's shape.
			name:     "ledger-lists-a-row-the-diff-did-not-change",
			document: overClaimedDocument,
			wantIDs:  s7AXSortedUniqueIDs(overClaimedIDs),
			blames:   overClaimed,
		},
		{
			// A ledger omitting a row the diff did change.
			name:     "ledger-omits-a-row-the-diff-did-change",
			document: underClaimedDocument,
			wantIDs:  underClaimedIDs,
			blames:   omitted,
		},
	} {
		if fixture.document == current {
			t.Fatalf("PIB-567 %s: sensitivity fixture changed nothing", fixture.name)
		}
		ids, count, ok := s7AXSection181Ledger(fixture.document, referenceRevision)
		if !ok || count != len(fixture.wantIDs) || !reflect.DeepEqual(ids, fixture.wantIDs) {
			t.Fatalf("PIB-567 %s: sensitivity fixture is not readable Markdown claiming %v: ids=%v count=%d ok=%t",
				fixture.name, fixture.wantIDs, ids, count, ok)
		}
		err := s7AXValidateRevisionLedger(referenceRevision, fixture.document, referenceDiff)
		if err == nil {
			t.Fatalf("PIB-567 %s: ledger validator accepted wrong input", fixture.name)
		}
		if !strings.Contains(err.Error(), fixture.blames) {
			t.Fatalf("PIB-567 %s was not attributed to %s: %v", fixture.name, fixture.blames, err)
		}
	}

	// The next revision after the newest one this document declares, proved
	// rather than assumed: appending one more revision paragraph to §18.1 must
	// leave every declared revision's ledger exactly where it was, must read
	// the appended revision's own ledger, and must still leave a reference
	// revision both fixture directions can be built from.
	nextRevision := newestWindowed + 1
	for _, revision := range declaredRevisions {
		if revision >= nextRevision {
			nextRevision = revision + 1
		}
	}
	_, sectionEnd, bounded := s7AXSection181Bounds(current)
	if !bounded {
		t.Fatal("PIB-567 §18.1 is missing from the current document")
	}
	simulated := current[:sectionEnd] + fmt.Sprintf(
		"**Rev-%d is an implementation-discovered, no-decision erratum only**, and\n"+
			"amends exactly two stable rows: %s.\n\n",
		nextRevision, s7AXRenderLedgerIDs(referenceClaim.ids[:2]),
	) + current[sectionEnd:]
	for _, revision := range declaredRevisions {
		before, beforeCount, hadLedger := s7AXSection181Ledger(current, revision)
		after, afterCount, hasLedger := s7AXSection181Ledger(simulated, revision)
		if hadLedger != hasLedger || beforeCount != afterCount ||
			!reflect.DeepEqual(before, after) {
			t.Fatalf("PIB-567 appending rev-%d moved rev-%d's ledger from %v/%d to %v/%d",
				nextRevision, revision, before, beforeCount, after, afterCount)
		}
		diff, validated := windowDiffs[revision]
		if !validated {
			t.Fatalf("PIB-567 rev-%d has neither a writer diff nor a working-tree surrogate", revision)
		}
		if err := s7AXValidateRevisionLedger(revision, simulated, diff); err != nil {
			t.Fatalf("PIB-567 appending rev-%d stopped rev-%d validating: %v",
				nextRevision, revision, err)
		}
	}
	appendedClaim, appendedLocated := s7AXLocateSection181Claim(simulated, nextRevision)
	if !appendedLocated ||
		!reflect.DeepEqual(appendedClaim.ids, referenceClaim.ids[:2]) {
		t.Fatalf("PIB-567 could not read the appended rev-%d ledger: %#v (located=%t)",
			nextRevision, appendedClaim, appendedLocated)
	}
	stillReference, stillLocated := s7AXLocateSection181Claim(simulated, referenceRevision)
	if !stillLocated || !reflect.DeepEqual(stillReference.ids, referenceClaim.ids) {
		t.Fatalf("PIB-567 appending rev-%d changed rev-%d's locatable ledger to %#v (located=%t)",
			nextRevision, referenceRevision, stillReference, stillLocated)
	}

	// Sensitivity fixture 3: a "corrected throughout" claim that still carries a
	// surviving normative occurrence of the withdrawn token.
	injected := current + "\nThe storage tuple is a triple of wire state, blob state and ownership.\n"
	if err := s7AXNormativeTokenScan(
		"PIB-567 fixture", injected, "triple",
	); err == nil {
		t.Fatal("PIB-567 corrected-throughout-claim-with-surviving-normative-uses: " +
			"accepted a corrected-throughout claim with a surviving normative occurrence")
	}
}
