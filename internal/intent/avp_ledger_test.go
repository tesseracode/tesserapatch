package intent

import (
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
	"unicode"
	"unicode/utf8"
)

// Acceptance ledger — GH #16 / PRD-artifact-validation-and-provenance rev-5
// (errata rev-6), 208 rows `AVP-001`…`AVP-208`.
//
// Every accepted row is mapped to the test that proves it, and the mapping is
// MECHANICALLY audited rather than reviewed by eye:
//
//  1. every row declared in the accepted matrix has a ledger entry;
//  2. every ledger entry names a row the matrix declares;
//  3. every reference resolves to a real, runnable Go test or subtest —
//     resolved by parsing the test sources, so a comment, a string fixture
//     or a declaration in the wrong package cannot satisfy it;
//  4. no two rows are satisfied by the identical reference;
//  5. the matrix's own arithmetic (208 rows, 25 categories, the kind table
//     and the 43-row guard predicate) reproduces from the rows themselves.
//
// §18.1's rule applies: a row that cannot be placed as written amends the
// PRD; it is never silently re-tiered.

type ledgerRef struct {
	Package string
	Test    string
	Subtest string
}

func (r ledgerRef) String() string {
	if r.Subtest == "" {
		return r.Package + ":" + r.Test
	}
	return r.Package + ":" + r.Test + "/" + r.Subtest
}

func parseLedgerRef(raw string) ledgerRef {
	pkg, rest, _ := strings.Cut(raw, ":")
	test, subtest, _ := strings.Cut(rest, "/")
	return ledgerRef{Package: pkg, Test: test, Subtest: subtest}
}

var ledgerPackages = map[string]struct {
	Dir       string
	GoPackage string
}{
	"intent": {Dir: "internal/intent", GoPackage: "intent"},
	"cli":    {Dir: "internal/cli", GoPackage: "cli"},
	"assets": {Dir: "assets", GoPackage: "assets_test"},
}

// acceptanceLedger maps every accepted row to the reference(s) that prove it.
func acceptanceLedger() map[string][]string {
	ledger := map[string][]string{
		// A — CLI grammar and surface boundary
		"AVP-001": {"cli:TestAVPGrammarAndSurface/AVP-001"},
		"AVP-002": {"cli:TestAVPGrammarAndSurface/AVP-002"},
		"AVP-003": {"cli:TestAVPGrammarAndSurface/AVP-003"},
		"AVP-004": {"cli:TestAVPGrammarAndSurface/AVP-004"},
		"AVP-005": {"cli:TestAVPGrammarAndSurface/AVP-005"},
		"AVP-006": {"cli:TestAVPGrammarAndSurface/AVP-006"},
		"AVP-007": {"cli:TestAVPGrammarAndSurface/AVP-007"},
		"AVP-008": {"cli:TestAVPGrammarAndSurface/AVP-008"},
		"AVP-009": {"cli:TestAVPGrammarAndSurface/AVP-009"},
		"AVP-010": {"cli:TestAVPGrammarAndSurface/AVP-010", "cli:TestAVPRoutingGoldens/AVP-010-bounded-delta"},

		// B — structural classification
		"AVP-011": {"intent:TestAVPStructuralClassification/AVP-011"},
		"AVP-012": {"intent:TestAVPStructuralClassification/AVP-012"},
		"AVP-013": {"intent:TestAVPStructuralClassification/AVP-013"},
		"AVP-014": {"intent:TestAVPStructuralClassification/AVP-014"},
		"AVP-015": {"intent:TestAVPStructuralClassification/AVP-015"},
		"AVP-016": {"intent:TestAVPStructuralClassification/AVP-016"},
		"AVP-017": {"intent:TestAVPStructuralClassification/AVP-017"},
		"AVP-018": {"intent:TestAVPStructuralClassification/AVP-018"},
		"AVP-019": {"intent:TestAVPStructuralClassification/AVP-019"},
		"AVP-020": {"intent:TestAVPStructuralClassification/AVP-020"},
		"AVP-021": {"intent:TestAVPStructuralClassification/AVP-021"},
		"AVP-022": {"intent:TestAVPStructuralClassification/AVP-022"},
		"AVP-023": {"intent:TestAVPStructuralClassification/AVP-023", "intent:TestAVPStructuralClassification/AVP-023-read-is-cap-plus-one-bounded"},
		"AVP-024": {"intent:TestAVPStructuralClassification/AVP-024"},
		"AVP-025": {"intent:TestAVPStructuralClassification/AVP-025"},
		"AVP-026": {"intent:TestAVPStructuralClassification/AVP-026"},
		"AVP-027": {"intent:TestAVPStructuralClassification/AVP-027"},
		"AVP-028": {"intent:TestAVPStructuralClassification/AVP-028"},
		"AVP-029": {"intent:TestAVPStructuralClassification/AVP-029"},
		"AVP-030": {"intent:TestAVPStructuralClassification/AVP-030"},

		// C — readiness and exit codes
		"AVP-031": {"cli:TestAVPReadinessAndOutput/AVP-031"},
		"AVP-032": {"cli:TestAVPReadinessAndOutput/AVP-032"},
		"AVP-033": {"cli:TestAVPReadinessAndOutput/AVP-033"},
		"AVP-034": {"cli:TestAVPReadinessAndOutput/AVP-034"},
		"AVP-035": {"cli:TestAVPReadinessAndOutput/AVP-035"},
		"AVP-036": {"cli:TestAVPReadinessAndOutput/AVP-036"},
		"AVP-037": {"cli:TestAVPReadinessAndOutput/AVP-037"},
		"AVP-038": {"cli:TestAVPReadinessAndOutput/AVP-038"},

		// D — output shape, order and determinism
		"AVP-039": {"cli:TestAVPReadinessAndOutput/AVP-039"},
		"AVP-040": {"cli:TestAVPReadinessAndOutput/AVP-040"},
		"AVP-041": {"cli:TestAVPReadinessAndOutput/AVP-041"},
		"AVP-042": {"cli:TestAVPReadinessAndOutput/AVP-042"},
		"AVP-043": {"cli:TestAVPReadinessAndOutput/AVP-043"},
		"AVP-044": {"cli:TestAVPReadinessAndOutput/AVP-044"},
		"AVP-045": {"cli:TestAVPReadinessAndOutput/AVP-045"},
		"AVP-046": {"cli:TestAVPReadinessAndOutput/AVP-046"},
		"AVP-047": {"cli:TestAVPReadinessAndOutput/AVP-047"},
		"AVP-048": {"cli:TestAVPReadinessAndOutput/AVP-048"},
		"AVP-049": {"cli:TestAVPReadinessAndOutput/AVP-049"},
		"AVP-050": {"cli:TestAVPReadinessAndOutput/AVP-050"},
		"AVP-051": {"intent:TestAVPGuards/AVP-051"},
		"AVP-052": {"cli:TestAVPReadinessAndOutput/AVP-052"},

		// E — zero mutation
		"AVP-053": {"cli:TestAVPZeroMutationAndPrivacy/AVP-053"},
		"AVP-054": {"cli:TestAVPZeroMutationAndPrivacy/AVP-054"},
		"AVP-055": {"cli:TestAVPZeroMutationAndPrivacy/AVP-055"},
		"AVP-056": {"cli:TestAVPZeroMutationAndPrivacy/AVP-056"},
		"AVP-057": {"cli:TestAVPZeroMutationAndPrivacy/AVP-057"},
		"AVP-058": {"cli:TestAVPZeroMutationAndPrivacy/AVP-058"},

		// F — provenance
		"AVP-059": {"cli:TestAVPZeroMutationAndPrivacy/AVP-059"},
		"AVP-060": {"intent:TestAVPSourceScans/AVP-060"},
		"AVP-061": {"cli:TestAVPPathParity/AVP-061"},
		"AVP-062": {"cli:TestAVPPathParity/AVP-062"},
		"AVP-063": {"cli:TestAVPPathParity/AVP-063"},

		// G — compatibility
		"AVP-064": {"cli:TestAVPCompatibility/AVP-064"},
		"AVP-065": {"cli:TestAVPCompatibility/AVP-065"},
		"AVP-066": {"cli:TestAVPCompatibility/AVP-066"},
		"AVP-067": {"cli:TestAVPCompatibility/AVP-067"},
		"AVP-068": {"cli:TestAVPCompatibility/AVP-068"},
		"AVP-069": {"cli:TestAVPCompatibility/AVP-069"},
		"AVP-070": {"cli:TestAVPCompatibility/AVP-070"},
		"AVP-071": {"cli:TestAVPRoutingGoldens/AVP-071"},
		"AVP-072": {"cli:TestAVPRoutingGoldens/AVP-072"},

		// H — Path A / Path B parity
		"AVP-073": {"cli:TestAVPPathParity/AVP-073"},
		"AVP-074": {"intent:TestAVPGuards/AVP-074"},
		"AVP-075": {"cli:TestAVPPathParity/AVP-075"},
		"AVP-076": {"cli:TestAVPPathParity/AVP-076"},

		// I — security and privacy
		"AVP-077": {"cli:TestAVPZeroMutationAndPrivacy/AVP-077"},
		"AVP-078": {"cli:TestAVPZeroMutationAndPrivacy/AVP-078"},
		"AVP-079": {"intent:TestAVPGuards/AVP-079"},
		"AVP-080": {"cli:TestAVPZeroMutationAndPrivacy/AVP-080"},
		"AVP-081": {"intent:TestAVPSourceScans/AVP-081"},
		"AVP-082": {"cli:TestAVPZeroMutationAndPrivacy/AVP-082"},

		// J — concurrency and snapshot
		"AVP-083": {"intent:TestAVPSnapshotAndCaptureRaces/AVP-083"},
		"AVP-084": {"intent:TestAVPSnapshotAndCaptureRaces/AVP-084"},
		"AVP-085": {"intent:TestAVPSnapshotAndCaptureRaces/AVP-085"},
		"AVP-086": {"cli:TestAVPZeroMutationAndPrivacy/AVP-086"},

		// K — source scans and parity guards
		"AVP-087": {"intent:TestAVPSourceScans/AVP-087"},
		"AVP-088": {"cli:TestAVPPathsAndSlugSafety/AVP-088"},
		"AVP-089": {"intent:TestAVPGuards/AVP-089"},
		"AVP-090": {"assets:TestAVPAssetParity/AVP-090"},
		"AVP-091": {"assets:TestAVPAssetParity/AVP-091"},
		"AVP-092": {"assets:TestAVPAssetParity/AVP-092"},

		// L — totality and completeness guards
		"AVP-093": {"intent:TestAVPGuards/AVP-093"},
		"AVP-094": {"intent:TestAVPGuards/AVP-094"},
		"AVP-095": {"intent:TestAVPGuards/AVP-095"},

		// M — CLI output envelope
		"AVP-096": {"cli:TestAVPExitEnvelope/AVP-096"},
		"AVP-097": {"cli:TestAVPExitEnvelope/AVP-097"},
		"AVP-098": {"cli:TestAVPExitEnvelope/AVP-098"},
		"AVP-099": {"cli:TestAVPExitEnvelope/AVP-099"},
		"AVP-100": {"cli:TestAVPExitEnvelope/AVP-100"},
		"AVP-101": {"intent:TestAVPGuards/AVP-101"},

		// N — slug safety
		"AVP-102": {"cli:TestAVPPathsAndSlugSafety/AVP-102"},
		"AVP-103": {"cli:TestAVPPathsAndSlugSafety/AVP-103"},
		"AVP-104": {"cli:TestAVPPathsAndSlugSafety/AVP-104"},
		"AVP-105": {"cli:TestAVPPathsAndSlugSafety/AVP-105"},
		"AVP-106": {"cli:TestAVPPathsAndSlugSafety/AVP-106"},

		// O — race-safe capture and bounded reads
		"AVP-107": {"intent:TestAVPSnapshotAndCaptureRaces/AVP-107"},
		"AVP-108": {"intent:TestAVPSnapshotAndCaptureRaces/AVP-108"},
		"AVP-109": {"intent:TestAVPSnapshotAndCaptureRaces/AVP-109"},
		"AVP-110": {"intent:TestAVPSnapshotAndCaptureRaces/AVP-110"},
		"AVP-111": {"intent:TestAVPSnapshotAndCaptureRaces/AVP-111"},
		"AVP-112": {"intent:TestAVPSnapshotAndCaptureRaces/AVP-112"},
		"AVP-113": {"intent:TestAVPSnapshotAndCaptureRaces/AVP-113"},
		"AVP-114": {"intent:TestAVPSnapshotAndCaptureRaces/AVP-114"},
		"AVP-115": {"intent:TestAVPSnapshotAndCaptureRaces/AVP-115"},
		"AVP-116": {"intent:TestAVPGuards/AVP-116"},
		"AVP-117": {"intent:TestAVPSnapshotAndCaptureRaces/AVP-117"},
		"AVP-118": {"intent:TestAVPGuards/AVP-118"},

		// P — diagnostic totality
		"AVP-119": {"intent:TestAVPGuards/AVP-119"},
		"AVP-120": {"cli:TestAVPAdvisoriesAndStatusSurface/AVP-120"},
		"AVP-121": {"cli:TestAVPAdvisoriesAndStatusSurface/AVP-121"},
		"AVP-122": {"intent:TestAVPGuards/AVP-122"},
		"AVP-123": {"cli:TestAVPAdvisoriesAndStatusSurface/AVP-123"},
		"AVP-124": {"cli:TestAVPAdvisoriesAndStatusSurface/AVP-124"},
		"AVP-125": {"cli:TestAVPAdvisoriesAndStatusSurface/AVP-125"},
		"AVP-126": {"cli:TestAVPAdvisoriesAndStatusSurface/AVP-126"},
		"AVP-127": {"intent:TestAVPGuards/AVP-127"},
		"AVP-128": {"cli:TestAVPRootLifetimeAndDifferential/AVP-128"},

		// Q — provenance forward compatibility
		"AVP-129": {"intent:TestAVPGuards/AVP-129"},

		// R — composite differential and routing non-invalidation
		"AVP-130": {"cli:TestAVPRootLifetimeAndDifferential/AVP-130"},
		"AVP-131": {"cli:TestAVPRootLifetimeAndDifferential/AVP-131"},
		"AVP-132": {"cli:TestAVPRootLifetimeAndDifferential/AVP-132"},
		"AVP-133": {"cli:TestAVPRootLifetimeAndDifferential/AVP-133"},
		"AVP-134": {"intent:TestAVPSourceScans/AVP-134"},
		"AVP-135": {"intent:TestAVPSourceScans/AVP-135"},
		"AVP-136": {"cli:TestAVPRoutingGoldens/AVP-136"},
		"AVP-137": {"cli:TestAVPRoutingGoldens/AVP-137"},
		"AVP-138": {"cli:TestAVPRootLifetimeAndDifferential/AVP-138"},

		// S — guard sensitivity and scoping
		"AVP-139": {"intent:TestAVPGuards/AVP-139"},
		"AVP-140": {"intent:TestAVPGuards/AVP-140", "cli:TestAVPRootLifetimeAndDifferential/AVP-140"},

		// T — rooted namespace, ancestor races and root lifetime
		"AVP-141": {"cli:TestAVPRootLifetimeAndDifferential/AVP-141"},
		"AVP-142": {"cli:TestAVPRootLifetimeAndDifferential/AVP-142"},
		"AVP-143": {"cli:TestAVPRootLifetimeAndDifferential/AVP-143"},
		"AVP-144": {"intent:TestAVPGuards/AVP-144", "intent:TestAVPAncestorPolicy/AVP-144-names-are-fs-ValidPath"},
		"AVP-145": {"intent:TestAVPAncestorPolicy/AVP-145"},
		"AVP-146": {"intent:TestAVPGuards/AVP-146", "intent:TestAVPAncestorPolicy/AVP-146"},
		"AVP-147": {"intent:TestAVPAncestorPolicy/AVP-147"},
		"AVP-148": {"intent:TestAVPAncestorPolicy/AVP-148"},
		"AVP-149": {"intent:TestAVPAncestorPolicy/AVP-149"},
		"AVP-150": {"intent:TestAVPGuards/AVP-150"},
		"AVP-151": {"intent:TestAVPAncestorPolicy/AVP-151"},
		"AVP-152": {"intent:TestAVPGuards/AVP-152"},

		// V — status.json inspection totality
		"AVP-153": {"intent:TestAVPGuards/AVP-153"},
		"AVP-154": {"cli:TestAVPAdvisoriesAndStatusSurface/AVP-154"},
		"AVP-155": {"intent:TestAVPStatusLadder/AVP-155"},
		"AVP-156": {"intent:TestAVPStatusLadder/AVP-156"},
		"AVP-157": {"intent:TestAVPStatusLadder/AVP-157"},
		"AVP-158": {"intent:TestAVPStatusLadder/AVP-158"},
		"AVP-159": {"intent:TestAVPStatusLadder/AVP-159"},
		"AVP-160": {"intent:TestAVPStatusLadder/AVP-160"},
		"AVP-161": {"intent:TestAVPStatusLadder/AVP-161"},
		"AVP-162": {"intent:TestAVPStatusLadder/AVP-162"},
		"AVP-163": {"intent:TestAVPStatusLadder/AVP-163"},
		"AVP-164": {"cli:TestAVPAdvisoriesAndStatusSurface/AVP-164"},
		"AVP-165": {"intent:TestAVPGuards/AVP-165"},
		"AVP-166": {"cli:TestAVPAdvisoriesAndStatusSurface/AVP-166", "intent:TestAVPStatusLadder/AVP-166-status-echo-domain"},
		"AVP-167": {"cli:TestAVPAdvisoriesAndStatusSurface/AVP-167"},
		"AVP-168": {"intent:TestAVPGuards/AVP-168"},
		"AVP-169": {"cli:TestAVPAdvisoriesAndStatusSurface/AVP-169", "intent:TestAVPStatusLadder/AVP-169-status-precedes-artifacts"},

		// W — fixed-buffer bounded reads
		"AVP-170": {"intent:TestAVPGuards/AVP-170"},
		"AVP-171": {"intent:TestAVPBoundedReads/AVP-171"},
		"AVP-172": {"intent:TestAVPGuards/AVP-172"},
		"AVP-173": {"intent:TestAVPBoundedReads/AVP-173"},
		"AVP-174": {"intent:TestAVPBoundedReads/AVP-174"},

		// X — platform matrix and native Windows CI
		"AVP-175": {"intent:TestAVPGuards/AVP-175", "intent:TestAVPWindowsSourceGuards/AVP-175-ci-half"},
		"AVP-176": {"intent:TestAVPNativeWindows/AVP-176"},
		"AVP-177": {"intent:TestAVPGuards/AVP-177"},
		"AVP-178": {"intent:TestAVPGuards/AVP-178", "intent:TestAVPWindowsSourceGuards/AVP-178-cross-build-half"},
		"AVP-179": {"cli:TestAVPRootLifetimeAndDifferential/AVP-179"},
		"AVP-180": {"intent:TestAVPSourceScans/AVP-180"},

		// Y — CLI ownership, output bytes and remediation coherence
		"AVP-181": {"intent:TestAVPGuards/AVP-181"},
		"AVP-182": {"cli:TestAVPRootLifetimeAndDifferential/AVP-182", "intent:TestAVPAncestorPolicy/AVP-182-walk-order"},
		"AVP-183": {"cli:TestAVPExitEnvelope/AVP-183"},
		"AVP-184": {"cli:TestAVPExitEnvelope/AVP-184"},
		"AVP-185": {"cli:TestAVPExitEnvelope/AVP-185"},
		"AVP-186": {"cli:TestAVPExitEnvelope/AVP-186"},
		"AVP-187": {"intent:TestAVPGuards/AVP-187"},
		"AVP-188": {"intent:TestAVPSkillSurfaces/AVP-188", "intent:TestAVPSkillSurfaces/AVP-188/sensitivity"},

		// Z — rooted-boundary honesty, seams, lifecycle, platform, citations
		"AVP-189": {"intent:TestAVPGuards/AVP-189"},
		"AVP-190": {"intent:TestAVPRootedBoundaryHonesty/AVP-190"},
		"AVP-191": {"intent:TestAVPGuards/AVP-191"},
		"AVP-192": {"cli:TestAVPPathsAndSlugSafety/AVP-192"},
		"AVP-193": {"intent:TestAVPGuards/AVP-193"},
		"AVP-194": {"intent:TestAVPGuards/AVP-194"},
		"AVP-195": {"intent:TestAVPRootedBoundaryHonesty/AVP-195"},
		"AVP-196": {"intent:TestAVPRootedBoundaryHonesty/AVP-196"},
		"AVP-197": {"intent:TestAVPGuards/AVP-197", "intent:TestAVPRootedBoundaryHonesty/AVP-197"},
		"AVP-198": {"intent:TestAVPGuards/AVP-198", "intent:TestAVPWindowsSourceGuards/AVP-198-source-half"},
		"AVP-199": {"intent:TestAVPNativeWindows/AVP-199", "intent:TestAVPWindowsSourceGuards/AVP-199-source-half"},
		"AVP-200": {"intent:TestAVPGuards/AVP-200"},
		"AVP-201": {"intent:TestAVPGuards/AVP-201"},
		"AVP-202": {"intent:TestAVPGuards/AVP-202"},
		"AVP-203": {"intent:TestAVPRootedBoundaryHonesty/AVP-203"},
		"AVP-204": {"intent:TestAVPStatusLadder/AVP-204"},
		"AVP-205": {"intent:TestAVPGuards/AVP-205", "intent:TestAVPRootedBoundaryHonesty/AVP-205"},
		"AVP-206": {"intent:TestAVPGuards/AVP-206", "intent:TestAVPRootedBoundaryHonesty/AVP-206"},
		"AVP-207": {"intent:TestAVPGuards/AVP-207"},
		"AVP-208": {"intent:TestAVPGuards/AVP-208"},
	}
	return ledger
}

// TestAcceptanceLedgerCoversEveryDeclaredRow asserts the ledger is total and
// exact against the accepted matrix, with no duplicate satisfaction.
func TestAcceptanceLedgerCoversEveryDeclaredRow(t *testing.T) {
	ledger := acceptanceLedger()
	rows := parseAcceptanceMatrix(t)

	declared := map[string]matrixRow{}
	for _, row := range rows {
		if _, exists := declared[row.ID]; exists {
			t.Fatalf("the matrix declares %s twice", row.ID)
		}
		declared[row.ID] = row
	}

	var missing []string
	for id := range declared {
		if len(ledger[id]) == 0 {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%d accepted row(s) have no ledger entry: %v", len(missing), missing)
	}

	var extra []string
	for id := range ledger {
		if _, ok := declared[id]; !ok {
			extra = append(extra, id)
		}
	}
	sort.Strings(extra)
	if len(extra) > 0 {
		t.Errorf("%d ledger entr(ies) name a row the PRD does not declare: %v", len(extra), extra)
	}

	if len(ledger) != 208 {
		t.Errorf("ledger has %d rows, want 208", len(ledger))
	}

	owner := map[string]string{}
	for id, refs := range ledger {
		for _, raw := range refs {
			if previous, taken := owner[raw]; taken {
				t.Errorf("%s and %s are both satisfied by the identical reference %s", previous, id, raw)
			}
			owner[raw] = id
		}
	}
}

// TestAcceptanceLedgerTestsExist resolves every reference against the parsed
// test sources. A comment, a string fixture, a declaration in another package
// or a non-runnable signature cannot satisfy a row.
func TestAcceptanceLedgerTestsExist(t *testing.T) {
	index := ledgerTestIndex(t)
	for id, refs := range acceptanceLedger() {
		for _, raw := range refs {
			ref := parseLedgerRef(raw)
			if !ledgerRefResolvesLive(index, ref) {
				t.Errorf("%s names %s, which does not resolve to a runnable test or subtest", id, ref)
			}
		}
	}
}

// TestAcceptanceLedgerGuardRowsAreRegistered ties the ledger's guard rows to
// the live guard registry: a `TestAVPGuards/<id>` reference is only honest if
// the registry actually contains that guard AND its sensitivity fixture.
func TestAcceptanceLedgerGuardRowsAreRegistered(t *testing.T) {
	derived := guardRowsFromMatrix(t)
	if len(derived) != 43 {
		t.Fatalf("the matrix yields %d guard rows, want 43", len(derived))
	}
	ledger := acceptanceLedger()
	for _, id := range derived {
		spec, ok := avpGuards[id]
		if !ok {
			t.Errorf("guard row %s is not registered in avpGuards", id)
			continue
		}
		if spec.Run == nil || spec.Sensitivity == nil {
			t.Errorf("guard row %s has an incomplete registration", id)
		}
		found := false
		for _, raw := range ledger[id] {
			if raw == "intent:TestAVPGuards/"+id {
				found = true
			}
		}
		if !found {
			t.Errorf("guard row %s is not mapped to intent:TestAVPGuards/%s", id, id)
		}
	}
	for id := range avpGuards {
		if _, ok := ledger[id]; !ok {
			t.Errorf("registered guard %s has no ledger row", id)
		}
	}
}

// TestAcceptanceLedgerMatrixArithmetic re-derives §18.27 from the rows.
func TestAcceptanceLedgerMatrixArithmetic(t *testing.T) {
	rows := parseAcceptanceMatrix(t)
	prd := repoFile(t, "docs/prds/PRD-artifact-validation-and-provenance.md")
	if err := checkMatrixArithmetic(t, rows, prd); err != nil {
		t.Fatal(err)
	}

	// Every kind's rows must be proved by a test in a package that can
	// actually assert that kind's observable.
	ledger := acceptanceLedger()
	for _, row := range rows {
		refs := ledger[row.ID]
		if len(refs) == 0 {
			continue
		}
		if strings.Contains(row.Kind, "G") {
			guarded := false
			for _, raw := range refs {
				if strings.HasPrefix(raw, "intent:TestAVPGuards/") {
					guarded = true
				}
			}
			if !guarded {
				t.Errorf("%s carries a G kind but no reference resolves to a registered guard", row.ID)
			}
		}
	}
}

// TestAcceptanceLedgerResolutionRejectsFalsePositives is the ledger's own
// sensitivity fixture: it proves the resolver cannot be satisfied by a
// comment, a string, a wrong package, a missing subtest or an unrunnable
// signature.
func TestAcceptanceLedgerResolutionRejectsFalsePositives(t *testing.T) {
	dir := t.TempDir()
	source := `package fixture

import "testing"

// func TestGhostRowNeverImplemented(t *testing.T) {}

func TestReal(t *testing.T) {
	t.Run("literal-subtest", func(t *testing.T) {})
	for _, tc := range []struct {
		id string
	}{{id: "AVP-777"}} {
		t.Run(tc.id, func(t *testing.T) { _ = tc })
	}
	for _, id := range []string{"AVP-778"} {
		t.Run(id, func(t *testing.T) {})
	}
	_ = "TestStringFixtureNotADeclaration"
}

func TestNotRunnable() {}
`
	if err := os.WriteFile(filepath.Join(dir, "fixture_test.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	external := `package fixture_test

import "testing"

func TestExternalOnly(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(dir, "external_test.go"), []byte(external), 0o644); err != nil {
		t.Fatal(err)
	}
	tests, err := indexLedgerPackageTests(dir, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	index := map[string]map[string]map[string]struct{}{"fixture": tests}

	for _, ref := range []ledgerRef{
		{Package: "fixture", Test: "TestGhostRowNeverImplemented"},
		{Package: "fixture", Test: "TestStringFixtureNotADeclaration"},
		{Package: "fixture", Test: "TestNotRunnable"},
		{Package: "fixture", Test: "TestExternalOnly"},
		{Package: "wrong-package", Test: "TestReal"},
		{Package: "fixture", Test: "TestReal", Subtest: "missing-subtest"},
		{Package: "fixture", Test: "TestReal", Subtest: "AVP-779"},
	} {
		if ledgerRefResolves(index, ref) {
			t.Errorf("false-positive reference resolved: %s", ref)
		}
	}
	for _, ref := range []ledgerRef{
		{Package: "fixture", Test: "TestReal"},
		{Package: "fixture", Test: "TestReal", Subtest: "literal-subtest"},
		{Package: "fixture", Test: "TestReal", Subtest: "AVP-777"},
		{Package: "fixture", Test: "TestReal", Subtest: "AVP-778"},
	} {
		if !ledgerRefResolves(index, ref) {
			t.Errorf("a real declaration did not resolve: %s", ref)
		}
	}

	// A duplicate mapping must be caught by the arithmetic, not by review.
	duplicate := map[string][]string{
		"AVP-001": {"cli:TestAVPGrammarAndSurface/AVP-001"},
		"AVP-002": {"cli:TestAVPGrammarAndSurface/AVP-001"},
	}
	owner := map[string]string{}
	collisions := 0
	for id, refs := range duplicate {
		for _, raw := range refs {
			if _, taken := owner[raw]; taken {
				collisions++
			}
			owner[raw] = id
		}
	}
	if collisions != 1 {
		t.Fatalf("the duplicate detector found %d collisions, want 1", collisions)
	}
}

// ---------------------------------------------------------------------------
// AST resolution
// ---------------------------------------------------------------------------

func ledgerTestIndex(t *testing.T) map[string]map[string]map[string]struct{} {
	t.Helper()
	index := map[string]map[string]map[string]struct{}{}
	root := repoRootDir(t)
	names := make([]string, 0, len(ledgerPackages))
	for name := range ledgerPackages {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		spec := ledgerPackages[name]
		tests, err := indexLedgerPackageTests(filepath.Join(root, filepath.FromSlash(spec.Dir)), spec.GoPackage)
		if err != nil {
			t.Fatalf("index %s: %v", name, err)
		}
		index[name] = tests
	}
	return index
}

func ledgerRefResolves(index map[string]map[string]map[string]struct{}, ref ledgerRef) bool {
	tests, ok := index[ref.Package]
	if !ok {
		return false
	}
	subtests, ok := tests[ref.Test]
	if !ok {
		return false
	}
	if ref.Subtest == "" {
		return true
	}
	if _, ok := subtests[ref.Subtest]; ok {
		return true
	}
	// A nested reference such as `AVP-188/sensitivity` also resolves when
	// each component was declared separately.
	components := strings.Split(ref.Subtest, "/")
	if len(components) < 2 {
		return false
	}
	for _, component := range components {
		if _, ok := subtests[component]; !ok {
			return false
		}
	}
	return true
}

// ledgerRefResolvesLive extends AST resolution with the one subtest family
// whose names are derived at run time rather than written as literals:
// TestAVPGuards iterates the live guard registry, so `TestAVPGuards/<id>`
// resolves exactly when the registry holds a complete guard for that id.
// This is strictly stronger than a literal, because it also proves the guard
// and its sensitivity fixture exist.
func ledgerRefResolvesLive(index map[string]map[string]map[string]struct{}, ref ledgerRef) bool {
	if ref.Package == "intent" && ref.Test == "TestAVPGuards" && ref.Subtest != "" {
		if !ledgerRefResolves(index, ledgerRef{Package: "intent", Test: "TestAVPGuards"}) {
			return false
		}
		id := strings.TrimSuffix(ref.Subtest, "/sensitivity")
		spec, ok := avpGuards[id]
		return ok && spec.Run != nil && spec.Sensitivity != nil
	}
	return ledgerRefResolves(index, ref)
}

func indexLedgerPackageTests(dir, goPackage string) (map[string]map[string]struct{}, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	out := map[string]map[string]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, 0)
		if err != nil {
			return nil, err
		}
		if file.Name == nil || file.Name.Name != goPackage {
			continue
		}
		aliases, dotTesting := ledgerTestingImports(file)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !isRunnableLedgerTest(fn, aliases, dotTesting) {
				continue
			}
			subtests := map[string]struct{}{}
			collectLedgerSubtests(fn, subtests)
			out[fn.Name.Name] = subtests
		}
	}
	return out, nil
}

func ledgerTestingImports(file *ast.File) (map[string]struct{}, bool) {
	aliases := map[string]struct{}{}
	dot := false
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != "testing" {
			continue
		}
		if imp.Name == nil {
			aliases["testing"] = struct{}{}
			continue
		}
		switch imp.Name.Name {
		case ".":
			dot = true
		case "_":
		default:
			aliases[imp.Name.Name] = struct{}{}
		}
	}
	return aliases, dot
}

func isRunnableLedgerTest(fn *ast.FuncDecl, aliases map[string]struct{}, dotTesting bool) bool {
	if fn.Recv != nil || fn.Body == nil || fn.Name == nil || !validLedgerTestName(fn.Name.Name) {
		return false
	}
	if fn.Type.TypeParams != nil && len(fn.Type.TypeParams.List) != 0 {
		return false
	}
	if fn.Type.Results != nil && len(fn.Type.Results.List) != 0 {
		return false
	}
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 || len(fn.Type.Params.List[0].Names) > 1 {
		return false
	}
	star, ok := fn.Type.Params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	if ident, ok := star.X.(*ast.Ident); ok {
		return dotTesting && ident.Name == "T"
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "T" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	_, ok = aliases[pkg.Name]
	return ok
}

func validLedgerTestName(name string) bool {
	if !strings.HasPrefix(name, "Test") || len(name) == len("Test") {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name[len("Test"):])
	return !unicode.IsLower(r)
}

// collectLedgerSubtests records every subtest name a test declares. Three
// forms are recognised, and nothing else:
//
//  1. `t.Run("literal", …)`;
//  2. `t.Run(tc.id, …)` over a table whose entries carry a keyed `id:` (or
//     `name:`) string literal;
//  3. `t.Run(id, …)` where `id` ranges over a `[]string{…}` literal.
//
// Anything computed at runtime is deliberately NOT resolvable, so a ledger
// entry can never be satisfied by a name the sources do not state.
func collectLedgerSubtests(fn *ast.FuncDecl, into map[string]struct{}) {
	fields := map[string]bool{}
	rangedIdents := map[string]bool{}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Run" {
			return true
		}
		switch argument := call.Args[0].(type) {
		case *ast.BasicLit:
			if argument.Kind == token.STRING {
				if value, err := strconv.Unquote(argument.Value); err == nil && value != "" {
					into[value] = struct{}{}
				}
			}
		case *ast.SelectorExpr:
			fields[argument.Sel.Name] = true
		case *ast.Ident:
			rangedIdents[argument.Name] = true
		}
		return true
	})

	if len(fields) > 0 {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			composite, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			for _, element := range composite.Elts {
				keyed, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := keyed.Key.(*ast.Ident)
				if !ok || !fields[key.Name] {
					continue
				}
				lit, ok := keyed.Value.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				if value, err := strconv.Unquote(lit.Value); err == nil && value != "" {
					into[value] = struct{}{}
				}
			}
			return true
		})
	}

	if len(rangedIdents) > 0 {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			rangeStmt, ok := n.(*ast.RangeStmt)
			if !ok || rangeStmt.Value == nil {
				return true
			}
			ident, ok := rangeStmt.Value.(*ast.Ident)
			if !ok || !rangedIdents[ident.Name] {
				return true
			}
			composite, ok := rangeStmt.X.(*ast.CompositeLit)
			if !ok {
				return true
			}
			arrayType, ok := composite.Type.(*ast.ArrayType)
			if !ok {
				return true
			}
			elem, ok := arrayType.Elt.(*ast.Ident)
			if !ok || elem.Name != "string" {
				return true
			}
			for _, element := range composite.Elts {
				lit, ok := element.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				if value, err := strconv.Unquote(lit.Value); err == nil && value != "" {
					into[value] = struct{}{}
				}
			}
			return true
		})
	}
}

// TestAcceptanceLedgerReport prints the coverage summary the handoff quotes,
// so the numbers in tracking documents are derived rather than asserted.
func TestAcceptanceLedgerReport(t *testing.T) {
	rows := parseAcceptanceMatrix(t)
	ledger := acceptanceLedger()
	byKind := map[string]int{}
	byCategory := map[string]int{}
	references := 0
	for _, row := range rows {
		byKind[row.Kind]++
		byCategory[row.Category]++
		references += len(ledger[row.ID])
	}
	kinds := make([]string, 0, len(byKind))
	for kind := range byKind {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	summary := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		summary = append(summary, fmt.Sprintf("%s=%d", kind, byKind[kind]))
	}
	t.Logf("rows=%d categories=%d references=%d guards=%d kinds[%s]",
		len(rows), len(byCategory), references, len(avpGuards), strings.Join(summary, " "))
	if references < len(rows) {
		t.Fatalf("%d references for %d rows", references, len(rows))
	}
}
