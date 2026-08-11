// Auditable coverage map for the Cluster H′ acceptance contract.
//
// PRD-feature-resource-claims-and-capture-adapters §14 defines 120
// AC-tagged clauses (AC-1..AC-120) and ADR-033's Test Matrix expands
// them into 189 rows. Tests are allowed to group rows, but the grouping
// must stay auditable: this file is the single place a reviewer can
// read to see, for every clause, which test discharges it and which
// matrix rows that clause covers.
//
// The map is verified mechanically below:
//
//   - every AC from 1 to 120 has an entry, with no gaps and no extras;
//   - every entry names a package and at least one concrete test;
//   - the union of every entry's MatrixRows is exactly 1..189.
//
// Test names are given as they appear in `go test -run`. Subtests are
// written as `Parent/subtest`; a comma-separated list means several
// tests jointly discharge the clause.

package rescap

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ACCount and MatrixRowCount are the contract's own totals.
const (
	ACCount        = 120
	MatrixRowCount = 189
)

// acCoverage records which test discharges each acceptance clause.
type acCoverage struct {
	// Package is the Go package the test lives in: "rescap", "store",
	// "cli" or "redact".
	Package string
	// Tests names the concrete test function(s)/subtest(s).
	Tests string
	// MatrixRows lists the ADR-033 Test Matrix rows this clause covers.
	MatrixRows []int
}

// acCoverageMap is the auditable clause-to-test mapping.
var acCoverageMap = map[int]acCoverage{
	1:   {Package: "rescap", Tests: "TestBuildDiffSummarySQLShape", MatrixRows: []int{1, 2}},
	2:   {Package: "rescap", Tests: "TestValidateDoltArgs/control-byte, /nul-byte, /backslash, /dot-range", MatrixRows: []int{3, 4, 5}},
	3:   {Package: "rescap", Tests: "TestSingleQuoteEscaping", MatrixRows: []int{6}},
	4:   {Package: "rescap", Tests: "TestValidateDoltArgs/missing-*, /unknown-key, /duplicate-key", MatrixRows: []int{7, 8, 9}},
	5:   {Package: "rescap", Tests: "TestDiffTypeTrackedVerbatim", MatrixRows: []int{10}},
	6:   {Package: "rescap", Tests: "TestParseDiffSummaryJSON/row-*", MatrixRows: []int{11, 12, 13, 14}},
	7:   {Package: "rescap", Tests: "TestParseDiffSummaryJSON/zero-rows-*, /schema-key-refused, /extra-top-level-key", MatrixRows: []int{15, 16}},
	8:   {Package: "rescap", Tests: "TestEngineDoltQueryErrorIsExitThree", MatrixRows: []int{17}},
	9:   {Package: "rescap", Tests: "TestZeroRowResultShape", MatrixRows: []int{18}},
	10:  {Package: "rescap", Tests: "TestValidateDoltArgs/working-*, /staged-lower", MatrixRows: []int{19, 20}},
	11:  {Package: "rescap", Tests: "TestValidateDoltArgs/dot-range", MatrixRows: []int{21}},
	12:  {Package: "store", Tests: "TestGoldenBatchIDVector + rescap TestZeroRowResultShape", MatrixRows: []int{22}},
	13:  {Package: "rescap", Tests: "TestRealOutputShapesParseIdentically", MatrixRows: []int{23, 24}},
	14:  {Package: "rescap", Tests: "TestNoVersionProbeAnywhere", MatrixRows: []int{25}},
	15:  {Package: "rescap", Tests: "TestEngineDoltCaptureEndToEnd (tool_identity has no path)", MatrixRows: []int{26}},
	16:  {Package: "rescap", Tests: "TestEngineDoltCaptureEndToEnd (minimal env assertion)", MatrixRows: []int{27}},
	17:  {Package: "rescap", Tests: "TestResolveExternalExecutablePolicy/inside-repo-refused, /under-git-directory-refused", MatrixRows: []int{28, 29}},
	18:  {Package: "rescap", Tests: "TestMakeVerifiedPrivateCopy/verified-copy-is-0500", MatrixRows: []int{30}},
	19:  {Package: "rescap", Tests: "TestEngineDoltCaptureEndToEnd (argv[0] is the private copy)", MatrixRows: []int{31}},
	20:  {Package: "cli", Tests: "TestResourceExitCodeTaxonomy/dolt-trust-flag-required", MatrixRows: []int{32, 33}},
	21:  {Package: "rescap", Tests: "TestMakeVerifiedPrivateCopy/untrusted-digest-refuses-and-leaves-nothing", MatrixRows: []int{34, 35}},
	22:  {Package: "rescap", Tests: "TestResolveExternalExecutablePolicy/outside-repo-accepted-through-symlink", MatrixRows: []int{36, 37}},
	23:  {Package: "rescap", Tests: "TestMakeVerifiedPrivateCopy/missing-pin-refuses-before-opening", MatrixRows: []int{38, 39, 40}},
	24:  {Package: "cli", Tests: "TestTrustDoltRePinsWithoutTouchingIdentityOrHistory", MatrixRows: []int{41}},
	25:  {Package: "cli", Tests: "TestTrustDoltRefusesNonDoltResource", MatrixRows: []int{42, 43}},
	26:  {Package: "rescap", Tests: "TestIsValidBinarySHA256 + cli TestResourceExitCodeTaxonomy/trust-dolt-bad-hex", MatrixRows: []int{44, 45}},
	27:  {Package: "store", Tests: "TestTrustExcludedFromResourceIdentity", MatrixRows: []int{46}},
	28:  {Package: "cli", Tests: "TestResourceAddListRemoveClearRoundTrip (duplicate add is a strict no-op)", MatrixRows: []int{47, 48}},
	29:  {Package: "rescap", Tests: "TestValidateDoltArgs/unsupported-contract", MatrixRows: []int{49}},
	30:  {Package: "store", Tests: "TestGoldenResourceIDVectors/vector2-*, /vector3-*", MatrixRows: []int{50}},
	31:  {Package: "rescap", Tests: "TestHashExecutableDescriptorIsCopyFree", MatrixRows: []int{51}},
	32:  {Package: "rescap", Tests: "TestCaptureIgnoredFileSingle", MatrixRows: []int{52, 53}},
	33:  {Package: "rescap", Tests: "TestGoldenDirectoryCombinedHash", MatrixRows: []int{54}},
	34:  {Package: "rescap", Tests: "TestBoundedReadRefusesOversizeContent", MatrixRows: []int{55, 56, 57}},
	35:  {Package: "rescap", Tests: "TestDirectoryFileCountLimit", MatrixRows: []int{58}},
	36:  {Package: "rescap", Tests: "TestNoRawBytesEverReachDisk", MatrixRows: []int{59, 60}},
	37:  {Package: "rescap", Tests: "TestCombinedHashCoversMode", MatrixRows: []int{61, 62}},
	38:  {Package: "cli", Tests: "TestDirectorySelectorCapture (chmod-only diff)", MatrixRows: []int{63}},
	39:  {Package: "rescap", Tests: "TestTrackedAndIgnoredRefusal", MatrixRows: []int{64, 65, 66}},
	40:  {Package: "rescap", Tests: "TestIgnoreGateExitCodeHandling", MatrixRows: []int{67, 68}},
	41:  {Package: "rescap", Tests: "TestIgnoreCheckArgumentColonRule, TestColonMagicSelectorIsNotFatal", MatrixRows: []int{69}},
	42:  {Package: "rescap", Tests: "TestLiteralPathspecsOnLsFiles", MatrixRows: []int{70}},
	43:  {Package: "rescap", Tests: "TestPathGateRefusesSymlinkComponents/ancestor-symlink-refused-even-when-target-is-safe", MatrixRows: []int{71, 72}},
	44:  {Package: "rescap", Tests: "TestPathGateRefusesSymlinkComponents/final-component-symlink-refused", MatrixRows: []int{73, 74}},
	45:  {Package: "rescap", Tests: "TestPathGateRefusesSymlinkComponents (descriptor identity via os.SameFile)", MatrixRows: []int{75}},
	46:  {Package: "rescap", Tests: "TestPathGateRefusesSymlinkComponents/missing-prefix-refused", MatrixRows: []int{76}},
	47:  {Package: "rescap", Tests: "TestLockContentionRefusesImmediately, cli TestCaptureLockContentionAcrossProcesses", MatrixRows: []int{77, 78}},
	48:  {Package: "rescap", Tests: "TestLockContentionRefusesImmediately (0600, empty body, never removed)", MatrixRows: []int{79}},
	49:  {Package: "cli", Tests: "TestCaptureLockContentionAcrossProcesses", MatrixRows: []int{80}},
	50:  {Package: "rescap", Tests: "TestLockContentionRefusesImmediately (release is implicit descriptor close)", MatrixRows: []int{81, 82, 83}},
	51:  {Package: "cli", Tests: "TestCaptureAndDiffLifecycle (list/diff never acquire the lock)", MatrixRows: []int{84, 85}},
	52:  {Package: "cli", Tests: "TestLocalPathTrackedRefusal (add/remove/clear/trust-dolt/capture)", MatrixRows: []int{86}},
	53:  {Package: "rescap", Tests: "TestScratchLifecycle", MatrixRows: []int{87}},
	54:  {Package: "cli", Tests: "TestCaptureAndDiffLifecycle (--dry-run writes nothing tracked)", MatrixRows: []int{88}},
	55:  {Package: "rescap", Tests: "TestScratchLifecycle (orphan sweep under the acquired lock)", MatrixRows: []int{89, 90}},
	56:  {Package: "store", Tests: "TestPublishBatchFirstWriteAndIdempotency", MatrixRows: []int{91}},
	57:  {Package: "store", Tests: "TestPublishBatchPresentationDriftIsIdempotent", MatrixRows: []int{92}},
	58:  {Package: "store", Tests: "TestPublishBatchCollisionAndCorruption/semantic-collision", MatrixRows: []int{93, 94}},
	59:  {Package: "store", Tests: "TestPublishBatchCollisionAndCorruption/unparseable-file, /self-inconsistent-batch-id", MatrixRows: []int{95}},
	60:  {Package: "store", Tests: "TestPublishBatchFirstWriteAndIdempotency (pointer is the sole commit point)", MatrixRows: []int{96, 97}},
	61:  {Package: "store", Tests: "TestPublishBatchFirstWriteAndIdempotency (carry-forward)", MatrixRows: []int{98, 99}},
	62:  {Package: "cli", Tests: "TestResourceAddListRemoveClearRoundTrip (remove leaves current.json alone)", MatrixRows: []int{100}},
	63:  {Package: "store", Tests: "TestPublishBatchFirstWriteAndIdempotency (current_batch_id provenance)", MatrixRows: []int{101}},
	64:  {Package: "store", Tests: "TestTrackedBatchMissing, cli TestTrackedBatchMissingIsExitOne", MatrixRows: []int{102, 103}},
	65:  {Package: "cli", Tests: "TestCaptureAndDiffLifecycle (revert repoints, no third batch)", MatrixRows: []int{104, 105}},
	66:  {Package: "store", Tests: "TestSweepTrackedTempArtifacts", MatrixRows: []int{106}},
	67:  {Package: "store", Tests: "TestTrackedArtifactPermissions, rescap TestScratchLifecycle", MatrixRows: []int{107}},
	68:  {Package: "store", Tests: "TestBatchFileWireShape, rescap TestGitMetadataViews", MatrixRows: []int{108}},
	69:  {Package: "store", Tests: "TestBatchFileWireShape (arrays never null, explicit nulls)", MatrixRows: []int{109, 110}},
	70:  {Package: "store", Tests: "TestBatchFileWireShape, cli TestNoTimestampsInTrackedResourceArtifacts", MatrixRows: []int{111}},
	71:  {Package: "store", Tests: "TestCanonNodePreservesFieldOrder, TestCanonNodeRejectsDuplicateKeys", MatrixRows: []int{112}},
	72:  {Package: "store", Tests: "TestCanonicalArgsJSONEncoding", MatrixRows: []int{113}},
	73:  {Package: "store", Tests: "TestGoldenBatchIDVector (batch_id absent from its own hash input)", MatrixRows: []int{114}},
	74:  {Package: "store", Tests: "TestControlByteRejection", MatrixRows: []int{115, 116}},
	75:  {Package: "store", Tests: "TestGoldenResourceIDVectors", MatrixRows: []int{117, 118, 119, 120}},
	76:  {Package: "store", Tests: "TestGoldenBatchIDVector", MatrixRows: []int{121}},
	77:  {Package: "cli", Tests: "TestLocalPathTrackedRefusal (trust-dolt included in the lock/gate set)", MatrixRows: []int{122, 123}},
	78:  {Package: "rescap", Tests: "TestBuildTagContract, TestLockAndObserverSupportedOnThisTarget", MatrixRows: []int{124, 125, 126}},
	79:  {Package: "rescap", Tests: "TestNoExternalSyscallDependency, statfs_{linux,darwin}_test.go", MatrixRows: []int{127}},
	80:  {Package: "rescap", Tests: "TestStatfsTypeNormalizationAcrossWidths (linux), TestDarwinFilesystemAllowDenyLists (darwin)", MatrixRows: []int{128, 129, 130, 131, 132, 133, 134, 135, 136, 137, 138, 139, 140, 141, 142, 143, 144, 145}},
	81:  {Package: "store", Tests: "TestMkdirAllAndSyncChainIsRetrySafe, TestNearestExistingAncestor", MatrixRows: []int{146, 147}},
	82:  {Package: "cli", Tests: "TestLocalPathTrackedRefusal", MatrixRows: []int{148}},
	83:  {Package: "cli", Tests: "TestCaptureRedactionRefusal, rescap TestRedactionRefusesTheWholeInvocation", MatrixRows: []int{149}},
	84:  {Package: "redact", Tests: "TestResourceClassInventoryIsClosedAtSix, TestScanCoversEveryResourceClass", MatrixRows: []int{150}},
	85:  {Package: "rescap", Tests: "TestBenignLeaderEventCompletes, TestSingleCleanupOwner", MatrixRows: []int{151, 152}},
	86:  {Package: "cli", Tests: "TestLocalPathTrackedRefusal", MatrixRows: []int{153}},
	87:  {Package: "rescap", Tests: "TestOutputCapIsARefusalNotATruncation", MatrixRows: []int{154}},
	88:  {Package: "rescap", Tests: "TestGitMetadataViews (every view scans its resolved values)", MatrixRows: []int{155, 156}},
	89:  {Package: "rescap", Tests: "TestGoldenDirectoryCombinedHash", MatrixRows: []int{157}},
	90:  {Package: "cli", Tests: "TestRecordResourcesTwoDomainOrdering/zero-resource-preflight", MatrixRows: []int{158}},
	91:  {Package: "cli", Tests: "TestRecordResourcesTwoDomainOrdering/git-failure-discards-the-candidate", MatrixRows: []int{159, 160}},
	92:  {Package: "cli", Tests: "TestRecordResourcesTwoDomainOrdering/git-success-publishes", MatrixRows: []int{161}},
	93:  {Package: "cli", Tests: "TestRecordResourcesTwoDomainOrdering/partial-domain-when-staging-fails-after-git-success", MatrixRows: []int{162}},
	94:  {Package: "rescap", Tests: "TestEngineDoltCaptureEndToEnd (cmd.Dir is the gated db_path)", MatrixRows: []int{163}},
	95:  {Package: "cli", Tests: "TestResourcesFileCorruptIsExitThree, store TestResourcesManifestCorruptionAndCollision", MatrixRows: []int{164}},
	96:  {Package: "rescap", Tests: "TestNonECHILDTerminalObserverError, TestTriggerPriorityOrder", MatrixRows: []int{165}},
	97:  {Package: "rescap", Tests: "TestReapTimeoutDisclosesTwoResiduals, TestBenignEntryPrimaryErrorIsFirstPhaseFailure", MatrixRows: []int{166}},
	98:  {Package: "rescap", Tests: "TestEngineDBPathIdentityChangedAfterExit", MatrixRows: []int{167}},
	99:  {Package: "cli", Tests: "TestCaptureAllOrNothingStaging", MatrixRows: []int{168}},
	100: {Package: "cli", Tests: "TestCaptureSubsetTargeting", MatrixRows: []int{169}},
	101: {Package: "rescap", Tests: "TestAnythingTrackedUnder, cli TestLocalPathTrackedRefusal", MatrixRows: []int{170}},
	102: {Package: "rescap", Tests: "TestHashExecutableDescriptorIsCopyFree", MatrixRows: []int{171}},
	103: {Package: "rescap", Tests: "TestMakeVerifiedPrivateCopy/verified-copy-is-0500", MatrixRows: []int{172}},
	104: {Package: "rescap", Tests: "TestObserverUsesRawWaitidWithWNOWAIT", MatrixRows: []int{173}},
	105: {Package: "rescap", Tests: "TestSingleCleanupOwner, TestTriggerPriorityOrder", MatrixRows: []int{174}},
	106: {Package: "rescap", Tests: "TestBuildTagContract, TestNoExternalSyscallDependency (+ CI cross-compile target)", MatrixRows: []int{175}},
	107: {Package: "rescap", Tests: "TestObserverUsesRawWaitidWithWNOWAIT (Darwin stop is a fail-closed trigger)", MatrixRows: []int{176}},
	108: {Package: "rescap", Tests: "TestToleratedSignalErrnos", MatrixRows: []int{177}},
	109: {Package: "rescap", Tests: "TestECHILDFinalizerSendsNoSignals, TestOwnerInducedReaderErrorsAreSuppressed", MatrixRows: []int{178}},
	110: {Package: "rescap", Tests: "TestGenuineReaderErrorClassification", MatrixRows: []int{179}},
	111: {Package: "rescap", Tests: "TestBenignEntryPrimaryErrorIsFirstPhaseFailure (bounded drain)", MatrixRows: []int{180}},
	112: {Package: "rescap", Tests: "TestSingleCleanupOwner", MatrixRows: []int{181}},
	113: {Package: "rescap", Tests: "TestTriggerPriorityOrder", MatrixRows: []int{182}},
	114: {Package: "rescap", Tests: "TestOutputCapIsARefusalNotATruncation", MatrixRows: []int{183}},
	115: {Package: "rescap", Tests: "TestOwnerInducedReaderErrorsAreSuppressed", MatrixRows: []int{184}},
	116: {Package: "rescap", Tests: "TestStartFailureCarveOut", MatrixRows: []int{185}},
	117: {Package: "rescap", Tests: "TestECHILDFinalizerSendsNoSignals, TestOutputCapIsARefusalNotATruncation", MatrixRows: []int{186}},
	118: {Package: "rescap", Tests: "TestReapTimeoutDisclosesTwoResiduals", MatrixRows: []int{187}},
	119: {Package: "rescap", Tests: "TestLateECHILDCutoffDrain", MatrixRows: []int{188}},
	120: {Package: "rescap", Tests: "TestBenignLeaderEventCompletes (WaitLaunchedBeforeSignals is false)", MatrixRows: []int{189}}}

// TestEveryAcceptanceClauseIsClaimed proves the map covers AC-1..AC-120
// exactly, with no gaps, no extras, and no empty claims.
func TestEveryAcceptanceClauseIsClaimed(t *testing.T) {
	if len(acCoverageMap) != ACCount {
		t.Fatalf("coverage map has %d entries, want %d", len(acCoverageMap), ACCount)
	}
	validPackages := map[string]struct{}{"rescap": {}, "store": {}, "cli": {}, "redact": {}}
	for i := 1; i <= ACCount; i++ {
		entry, ok := acCoverageMap[i]
		if !ok {
			t.Errorf("AC-%d has no coverage entry", i)
			continue
		}
		if _, ok := validPackages[entry.Package]; !ok {
			t.Errorf("AC-%d names an unknown package %q", i, entry.Package)
		}
		if strings.TrimSpace(entry.Tests) == "" {
			t.Errorf("AC-%d claims no concrete test", i)
		}
		if !strings.Contains(entry.Tests, "Test") {
			t.Errorf("AC-%d does not name a Go test function: %q", i, entry.Tests)
		}
		if len(entry.MatrixRows) == 0 {
			t.Errorf("AC-%d claims no matrix rows", i)
		}
	}
	for i := range acCoverageMap {
		if i < 1 || i > ACCount {
			t.Errorf("coverage map contains an out-of-range clause AC-%d", i)
		}
	}
}

// TestEveryMatrixRowIsCovered proves the union of every clause's rows
// is exactly the 189-row matrix, with no duplicates and no gaps.
func TestEveryMatrixRowIsCovered(t *testing.T) {
	seen := map[int]int{}
	for ac, entry := range acCoverageMap {
		for _, row := range entry.MatrixRows {
			if row < 1 || row > MatrixRowCount {
				t.Errorf("AC-%d claims out-of-range matrix row %d", ac, row)
				continue
			}
			if prev, dup := seen[row]; dup {
				t.Errorf("matrix row %d is claimed by both AC-%d and AC-%d", row, prev, ac)
				continue
			}
			seen[row] = ac
		}
	}
	var missing []int
	for row := 1; row <= MatrixRowCount; row++ {
		if _, ok := seen[row]; !ok {
			missing = append(missing, row)
		}
	}
	sort.Ints(missing)
	if len(missing) != 0 {
		t.Fatalf("matrix rows with no coverage entry: %v", missing)
	}
	if len(seen) != MatrixRowCount {
		t.Fatalf("covered %d matrix rows, want %d", len(seen), MatrixRowCount)
	}
}

// TestCoverageIsSpreadAcrossEveryPackage is a sanity check that the
// contract is not silently concentrated in one place: each of the four
// packages must discharge at least one clause.
func TestCoverageIsSpreadAcrossEveryPackage(t *testing.T) {
	byPackage := map[string]int{}
	for _, entry := range acCoverageMap {
		byPackage[entry.Package]++
	}
	for _, pkg := range []string{"rescap", "store", "cli", "redact"} {
		if byPackage[pkg] == 0 {
			t.Errorf("package %q discharges no acceptance clause", pkg)
		}
	}
}

// TestClaimedTestsActuallyExist makes the coverage map falsifiable: it
// parses every claimed test name out of the map and confirms a matching
// `func Test<name>(` declaration exists in the named package. Without
// this the map would be prose that can silently rot.
func TestClaimedTestsActuallyExist(t *testing.T) {
	declared := map[string]map[string]struct{}{}
	for pkg, dir := range map[string]string{
		"rescap": ".",
		"store":  "../store",
		"cli":    "../cli",
		"redact": "../redact",
	} {
		names, err := declaredTestNames(dir)
		if err != nil {
			t.Fatalf("scanning %s: %v", dir, err)
		}
		declared[pkg] = names
	}
	for ac, entry := range acCoverageMap {
		for _, name := range claimedTestNames(entry.Tests) {
			if _, ok := declared[entry.Package][name]; !ok {
				// A claim may name a test in a different package by
				// prefixing it, e.g. "cli TestFoo"; accept it if it
				// exists anywhere.
				found := false
				for _, set := range declared {
					if _, ok := set[name]; ok {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("AC-%d claims %s, which does not exist in any package", ac, name)
				}
			}
		}
	}
}

// claimedTestNames extracts every top-level Go test function name from
// a coverage entry's free-form Tests field, dropping subtest suffixes
// and prose.
func claimedTestNames(field string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, token := range strings.FieldsFunc(field, func(r rune) bool {
		return r == ',' || r == ' ' || r == '(' || r == ')' || r == '+'
	}) {
		if !strings.HasPrefix(token, "Test") {
			continue
		}
		name := token
		if idx := strings.Index(name, "/"); idx >= 0 {
			name = name[:idx]
		}
		name = strings.TrimRight(name, ".,")
		if name == "Test" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

// declaredTestNames returns every `func TestX(` name declared in a
// directory's _test.go files.
func declaredTestNames(dir string) (map[string]struct{}, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := map[string]struct{}{}
	re := regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]*)\(`)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		for _, m := range re.FindAllStringSubmatch(string(body), -1) {
			out[m[1]] = struct{}{}
		}
	}
	return out, nil
}
