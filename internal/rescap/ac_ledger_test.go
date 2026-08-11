// Acceptance-clause ledger for Cluster H′.
//
// PRD-feature-resource-claims-and-capture-adapters §14 defines 120
// AC-tagged clauses (AC-1..AC-120) and ADR-033's Test Matrix expands
// them into 189 rows. This file is a **ledger**, not a proof: it
// records, for every clause, exactly which test discharges it and which
// matrix rows the clause covers, and it verifies that those references
// resolve. It cannot and does not verify that a referenced test
// *semantically* exercises the mechanism — that judgement stays with the
// reviewer, and this file exists to make it cheap.
//
// Rev-1 hardening (rev-0 review finding 6). References are now
// structured rather than free-form prose:
//
//   - a reference is an exact (package, top-level test, optional
//     subtest) triple, never a sentence;
//   - a name is resolved ONLY in its declared package — rev-0 fell back
//     to "exists in any package", so a mis-attributed reference passed;
//   - a declared subtest is resolved too — rev-0 dropped everything
//     after the first `/`, so `TestFoo/typo` verified only `TestFoo`;
//   - resolution is done by parsing Go source with go/ast, not by
//     regex over text.
//
// Subtest resolution recognizes the two forms this suite actually uses:
// a literal `t.Run("name", ...)`, and a table-driven case whose `name`
// field is a string literal inside a composite literal in the same test
// function. Both are deterministic; a renamed case breaks the ledger.

package rescap

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// ACCount and MatrixRowCount are the contract's own totals.
const (
	ACCount        = 120
	MatrixRowCount = 189
)

// ledgerPackages maps a ledger package tag to its source directory.
var ledgerPackages = map[string]string{
	"rescap": ".",
	"store":  "../store",
	"cli":    "../cli",
	"redact": "../redact",
}

// testRef is an exact reference to one test. Subtest is optional; when
// present it names a literal subtest declared inside Test.
type testRef struct {
	Package string
	Test    string
	Subtest string
}

// ref is a terse testRef constructor used by the ledger below.
func ref(pkg, test, subtest string) testRef {
	return testRef{Package: pkg, Test: test, Subtest: subtest}
}

// String renders a reference the way `go test -run` would address it.
func (r testRef) String() string {
	if r.Subtest == "" {
		return r.Package + ":" + r.Test
	}
	return r.Package + ":" + r.Test + "/" + r.Subtest
}

// acLedgerEntry records which tests discharge one acceptance clause and
// which matrix rows that clause accounts for.
type acLedgerEntry struct {
	Refs       []testRef
	MatrixRows []int
}

// acLedger is the clause-to-test ledger.
var acLedger = map[int]acLedgerEntry{
	1:   {Refs: []testRef{ref("rescap", "TestBuildDiffSummarySQLShape", "")}, MatrixRows: []int{1, 2}},
	2:   {Refs: []testRef{ref("rescap", "TestValidateDoltArgs", "control-byte"), ref("rescap", "TestValidateDoltArgs", "nul-byte"), ref("rescap", "TestValidateDoltArgs", "backslash"), ref("rescap", "TestValidateDoltArgs", "dot-range")}, MatrixRows: []int{3, 4, 5}},
	3:   {Refs: []testRef{ref("rescap", "TestSingleQuoteEscaping", "")}, MatrixRows: []int{6}},
	4:   {Refs: []testRef{ref("rescap", "TestValidateDoltArgs", "missing-db-path"), ref("rescap", "TestValidateDoltArgs", "missing-table"), ref("rescap", "TestValidateDoltArgs", "missing-from"), ref("rescap", "TestValidateDoltArgs", "missing-to"), ref("rescap", "TestValidateDoltArgs", "unknown-key"), ref("rescap", "TestValidateDoltArgs", "duplicate-key")}, MatrixRows: []int{7, 8, 9}},
	5:   {Refs: []testRef{ref("rescap", "TestDiffTypeTrackedVerbatim", "")}, MatrixRows: []int{10}},
	6:   {Refs: []testRef{ref("rescap", "TestParseDiffSummaryJSON", "row-missing-field"), ref("rescap", "TestParseDiffSummaryJSON", "row-extra-field"), ref("rescap", "TestParseDiffSummaryJSON", "row-duplicate-key"), ref("rescap", "TestParseDiffSummaryJSON", "boolean-as-int"), ref("rescap", "TestParseDiffSummaryJSON", "boolean-as-string")}, MatrixRows: []int{11, 12, 13, 14}},
	7:   {Refs: []testRef{ref("rescap", "TestParseDiffSummaryJSON", "zero-rows-bare"), ref("rescap", "TestParseDiffSummaryJSON", "zero-rows-real-shape"), ref("rescap", "TestParseDiffSummaryJSON", "schema-key-refused"), ref("rescap", "TestParseDiffSummaryJSON", "extra-top-level-key")}, MatrixRows: []int{15, 16}},
	8:   {Refs: []testRef{ref("rescap", "TestEngineDoltQueryErrorIsExitThree", "")}, MatrixRows: []int{17}},
	9:   {Refs: []testRef{ref("rescap", "TestZeroRowResultShape", "")}, MatrixRows: []int{18}},
	10:  {Refs: []testRef{ref("rescap", "TestValidateDoltArgs", "working-upper"), ref("rescap", "TestValidateDoltArgs", "working-mixed"), ref("rescap", "TestValidateDoltArgs", "staged-lower")}, MatrixRows: []int{19, 20}},
	11:  {Refs: []testRef{ref("rescap", "TestValidateDoltArgs", "dot-range")}, MatrixRows: []int{21}},
	12:  {Refs: []testRef{ref("store", "TestGoldenBatchIDVector", ""), ref("rescap", "TestZeroRowResultShape", "")}, MatrixRows: []int{22}},
	13:  {Refs: []testRef{ref("rescap", "TestRealOutputShapesParseIdentically", "")}, MatrixRows: []int{23, 24}},
	14:  {Refs: []testRef{ref("rescap", "TestNoVersionProbeAnywhere", "")}, MatrixRows: []int{25}},
	15:  {Refs: []testRef{ref("rescap", "TestEngineDoltCaptureEndToEnd", "")}, MatrixRows: []int{26}},
	16:  {Refs: []testRef{ref("rescap", "TestEngineDoltCaptureEndToEnd", "")}, MatrixRows: []int{27}},
	17:  {Refs: []testRef{ref("rescap", "TestResolveExternalExecutablePolicy", "inside-repo-refused"), ref("rescap", "TestResolveExternalExecutablePolicy", "under-git-directory-refused")}, MatrixRows: []int{28, 29}},
	18:  {Refs: []testRef{ref("rescap", "TestMakeVerifiedPrivateCopy", "verified-copy-is-0500")}, MatrixRows: []int{30}},
	19:  {Refs: []testRef{ref("rescap", "TestEngineDoltCaptureEndToEnd", "")}, MatrixRows: []int{31}},
	20:  {Refs: []testRef{ref("cli", "TestResourceExitCodeTaxonomy", "dolt-trust-flag-required")}, MatrixRows: []int{32, 33}},
	21:  {Refs: []testRef{ref("rescap", "TestMakeVerifiedPrivateCopy", "untrusted-digest-refuses-and-leaves-nothing"), ref("rescap", "TestEngineUntrustedBinaryRefusesBeforeStarting", "")}, MatrixRows: []int{34, 35}},
	22:  {Refs: []testRef{ref("rescap", "TestResolveExternalExecutablePolicy", "outside-repo-accepted-through-symlink")}, MatrixRows: []int{36, 37}},
	23:  {Refs: []testRef{ref("rescap", "TestMakeVerifiedPrivateCopy", "missing-pin-refuses-before-opening")}, MatrixRows: []int{38, 39, 40}},
	24:  {Refs: []testRef{ref("cli", "TestTrustDoltRePinsWithoutTouchingIdentityOrHistory", "")}, MatrixRows: []int{41}},
	25:  {Refs: []testRef{ref("cli", "TestTrustDoltRefusesNonDoltResource", "")}, MatrixRows: []int{42, 43}},
	26:  {Refs: []testRef{ref("rescap", "TestIsValidBinarySHA256", ""), ref("cli", "TestResourceExitCodeTaxonomy", "trust-dolt-bad-hex")}, MatrixRows: []int{44, 45}},
	27:  {Refs: []testRef{ref("store", "TestTrustExcludedFromResourceIdentity", "")}, MatrixRows: []int{46}},
	28:  {Refs: []testRef{ref("cli", "TestGoldenResourceIDsThroughTheRealAddCLI", "vector3-dolt-explicit-capability-is-idempotent")}, MatrixRows: []int{47, 48}},
	29:  {Refs: []testRef{ref("rescap", "TestValidateDoltArgs", "unsupported-contract")}, MatrixRows: []int{49}},
	30:  {Refs: []testRef{ref("cli", "TestGoldenResourceIDsThroughTheRealAddCLI", "vector2-dolt-documented-form-without-capability"), ref("cli", "TestGoldenResourceIDsThroughTheRealAddCLI", "vector3-dolt-explicit-capability-is-idempotent")}, MatrixRows: []int{50}},
	31:  {Refs: []testRef{ref("rescap", "TestHashExecutableDescriptorIsCopyFree", "")}, MatrixRows: []int{51}},
	32:  {Refs: []testRef{ref("rescap", "TestCaptureIgnoredFileSingle", "")}, MatrixRows: []int{52, 53}},
	33:  {Refs: []testRef{ref("rescap", "TestGoldenDirectoryCombinedHash", "")}, MatrixRows: []int{54}},
	34:  {Refs: []testRef{ref("rescap", "TestBoundedReadRefusesOversizeContent", "")}, MatrixRows: []int{55, 56, 57}},
	35:  {Refs: []testRef{ref("rescap", "TestDirectoryFileCountLimit", "")}, MatrixRows: []int{58}},
	36:  {Refs: []testRef{ref("rescap", "TestNoRawBytesEverReachDisk", "")}, MatrixRows: []int{59, 60}},
	37:  {Refs: []testRef{ref("rescap", "TestCombinedHashCoversMode", "")}, MatrixRows: []int{61, 62}},
	38:  {Refs: []testRef{ref("cli", "TestDirectorySelectorCapture", "")}, MatrixRows: []int{63}},
	39:  {Refs: []testRef{ref("rescap", "TestTrackedAndIgnoredRefusal", "")}, MatrixRows: []int{64, 65, 66}},
	40:  {Refs: []testRef{ref("rescap", "TestIgnoreGateExitCodeHandling", "")}, MatrixRows: []int{67, 68}},
	41:  {Refs: []testRef{ref("rescap", "TestIgnoreCheckArgumentColonRule", ""), ref("rescap", "TestColonMagicSelectorIsNotFatal", "")}, MatrixRows: []int{69}},
	42:  {Refs: []testRef{ref("rescap", "TestLiteralPathspecsOnLsFiles", "")}, MatrixRows: []int{70}},
	43:  {Refs: []testRef{ref("rescap", "TestPathGateRefusesSymlinkComponents", "ancestor-symlink-refused-even-when-target-is-safe")}, MatrixRows: []int{71, 72}},
	44:  {Refs: []testRef{ref("rescap", "TestPathGateRefusesSymlinkComponents", "final-component-symlink-refused")}, MatrixRows: []int{73, 74}},
	45:  {Refs: []testRef{ref("rescap", "TestSameFileDescriptorGateIsTheLoadBearingCheck", ""), ref("rescap", "TestGatedOpenAcceptsAnUnreplacedEntry", "")}, MatrixRows: []int{75}},
	46:  {Refs: []testRef{ref("rescap", "TestPathGateRefusesSymlinkComponents", "missing-prefix-refused")}, MatrixRows: []int{76}},
	47:  {Refs: []testRef{ref("rescap", "TestLockContentionRefusesImmediately", ""), ref("cli", "TestCaptureLockContentionAcrossProcesses", "")}, MatrixRows: []int{77, 78}},
	48:  {Refs: []testRef{ref("rescap", "TestLockContentionRefusesImmediately", "")}, MatrixRows: []int{79}},
	49:  {Refs: []testRef{ref("cli", "TestCaptureLockContentionAcrossProcesses", "")}, MatrixRows: []int{80}},
	50:  {Refs: []testRef{ref("rescap", "TestLockContentionRefusesImmediately", "")}, MatrixRows: []int{81, 82, 83}},
	51:  {Refs: []testRef{ref("cli", "TestCaptureAndDiffLifecycle", "")}, MatrixRows: []int{84, 85}},
	52:  {Refs: []testRef{ref("cli", "TestLocalPathTrackedRefusal", "add"), ref("cli", "TestLocalPathTrackedRefusal", "remove"), ref("cli", "TestLocalPathTrackedRefusal", "clear"), ref("cli", "TestLocalPathTrackedRefusal", "trust-dolt")}, MatrixRows: []int{86}},
	53:  {Refs: []testRef{ref("rescap", "TestScratchLifecycle", "")}, MatrixRows: []int{87}},
	54:  {Refs: []testRef{ref("cli", "TestCaptureAndDiffLifecycle", "")}, MatrixRows: []int{88}},
	55:  {Refs: []testRef{ref("rescap", "TestScratchLifecycle", "")}, MatrixRows: []int{89, 90}},
	56:  {Refs: []testRef{ref("store", "TestPublishBatchFirstWriteAndIdempotency", "")}, MatrixRows: []int{91}},
	57:  {Refs: []testRef{ref("store", "TestPublishBatchPresentationDriftIsIdempotent", ""), ref("store", "TestPublishStillTreatsPresentationDriftAsIdempotent", "")}, MatrixRows: []int{92}},
	58:  {Refs: []testRef{ref("store", "TestPublishBatchCollisionAndCorruption", "semantic-collision-between-two-authentic-bodies"), ref("store", "TestPublishBatchCollisionAndCorruption", "tampered-existing-body-is-corruption-not-collision")}, MatrixRows: []int{93, 94}},
	59:  {Refs: []testRef{ref("store", "TestPublishBatchCollisionAndCorruption", "unparseable-file"), ref("store", "TestPublishBatchCollisionAndCorruption", "self-inconsistent-batch-id"), ref("store", "TestPublishRejectsTrailingJSONAfterAValidObject", ""), ref("cli", "TestListPreservesBatchLoadTaxonomy", ""), ref("cli", "TestDiffPreservesBatchLoadTaxonomy", ""), ref("cli", "TestCaptureOverTamperedBatchIsCorruptionNotCollision", "")}, MatrixRows: []int{95}},
	60:  {Refs: []testRef{ref("rescap", "TestCurrentPointerIsCommittedByRenameNotDirectWrite", "")}, MatrixRows: []int{96, 97}},
	61:  {Refs: []testRef{ref("store", "TestPublishBatchFirstWriteAndIdempotency", "")}, MatrixRows: []int{98, 99}},
	62:  {Refs: []testRef{ref("cli", "TestResourceAddListRemoveClearRoundTrip", "")}, MatrixRows: []int{100}},
	63:  {Refs: []testRef{ref("store", "TestPublishBatchFirstWriteAndIdempotency", "")}, MatrixRows: []int{101}},
	64:  {Refs: []testRef{ref("store", "TestLoadBatchAbsentFileStaysTrackedBatchMissing", ""), ref("cli", "TestTrackedBatchMissingIsExitOne", "")}, MatrixRows: []int{102, 103}},
	65:  {Refs: []testRef{ref("cli", "TestCaptureAndDiffLifecycle", "")}, MatrixRows: []int{104, 105}},
	66:  {Refs: []testRef{ref("store", "TestSweepTrackedTempArtifacts", "")}, MatrixRows: []int{106}},
	67:  {Refs: []testRef{ref("store", "TestTrackedArtifactPermissions", ""), ref("rescap", "TestScratchLifecycle", "")}, MatrixRows: []int{107}},
	68:  {Refs: []testRef{ref("store", "TestBatchFileWireShape", ""), ref("rescap", "TestGitMetadataViews", "")}, MatrixRows: []int{108}},
	69:  {Refs: []testRef{ref("store", "TestBatchFileWireShape", "")}, MatrixRows: []int{109, 110}},
	70:  {Refs: []testRef{ref("store", "TestBatchFileWireShape", ""), ref("cli", "TestNoTimestampsInTrackedResourceArtifacts", "")}, MatrixRows: []int{111}},
	71:  {Refs: []testRef{ref("store", "TestCanonNodePreservesFieldOrder", ""), ref("store", "TestCanonNodeRejectsDuplicateKeys", "")}, MatrixRows: []int{112}},
	72:  {Refs: []testRef{ref("store", "TestCanonicalArgsJSONEncoding", "")}, MatrixRows: []int{113}},
	73:  {Refs: []testRef{ref("store", "TestGoldenBatchIDVector", "")}, MatrixRows: []int{114}},
	74:  {Refs: []testRef{ref("store", "TestControlByteRejection", "")}, MatrixRows: []int{115, 116}},
	75:  {Refs: []testRef{ref("cli", "TestGoldenResourceIDsThroughTheRealAddCLI", "vector1-git-metadata-head-capability-omitted"), ref("cli", "TestGoldenResourceIDsThroughTheRealAddCLI", "vector4-ignored-file"), ref("store", "TestGoldenResourceIDVectors", "")}, MatrixRows: []int{117, 118, 119, 120}},
	76:  {Refs: []testRef{ref("store", "TestGoldenBatchIDVector", "")}, MatrixRows: []int{121}},
	77:  {Refs: []testRef{ref("cli", "TestLocalPathTrackedRefusal", "trust-dolt")}, MatrixRows: []int{122, 123}},
	78:  {Refs: []testRef{ref("rescap", "TestBuildTagContract", ""), ref("rescap", "TestLockAndObserverSupportedOnThisTarget", "")}, MatrixRows: []int{124, 125, 126}},
	79:  {Refs: []testRef{ref("rescap", "TestNoExternalSyscallDependency", "")}, MatrixRows: []int{127}},
	80:  {Refs: []testRef{ref("rescap", "TestDarwinFilesystemAllowDenyLists", ""), ref("rescap", "TestFstypenameSignedConversion", "")}, MatrixRows: []int{128, 129, 130, 131, 132, 133, 134, 135, 136, 137, 138, 139, 140, 141, 142, 143, 144, 145}},
	81:  {Refs: []testRef{ref("store", "TestMkdirAllAndSyncChainIsRetrySafe", ""), ref("store", "TestNearestExistingAncestor", "")}, MatrixRows: []int{146, 147}},
	82:  {Refs: []testRef{ref("cli", "TestLocalPathTrackedRefusal", "capture")}, MatrixRows: []int{148}},
	83:  {Refs: []testRef{ref("cli", "TestCaptureRedactionRefusal", ""), ref("rescap", "TestRedactionRefusesTheWholeInvocation", "")}, MatrixRows: []int{149}},
	84:  {Refs: []testRef{ref("redact", "TestResourceClassInventoryIsClosedAtSix", ""), ref("redact", "TestScanCoversEveryResourceClass", "")}, MatrixRows: []int{150}},
	85:  {Refs: []testRef{ref("rescap", "TestBenignLeaderEventCompletes", ""), ref("rescap", "TestSingleCleanupOwner", "")}, MatrixRows: []int{151, 152}},
	86:  {Refs: []testRef{ref("cli", "TestLocalPathTrackedRefusal", "add")}, MatrixRows: []int{153}},
	87:  {Refs: []testRef{ref("rescap", "TestRunawayChildIsBoundedToCapPlusOne", ""), ref("rescap", "TestClaimRetentionNeverOvershootsUnderConcurrency", "")}, MatrixRows: []int{154}},
	88:  {Refs: []testRef{ref("rescap", "TestGitMetadataViews", "")}, MatrixRows: []int{155, 156}},
	89:  {Refs: []testRef{ref("rescap", "TestGoldenDirectoryCombinedHash", "")}, MatrixRows: []int{157}},
	90:  {Refs: []testRef{ref("cli", "TestRecordResourcesTwoDomainOrdering", "zero-resource-preflight")}, MatrixRows: []int{158}},
	91:  {Refs: []testRef{ref("cli", "TestRecordResourcesTwoDomainOrdering", "git-failure-discards-the-candidate")}, MatrixRows: []int{159, 160}},
	92:  {Refs: []testRef{ref("cli", "TestRecordResourcesTwoDomainOrdering", "git-success-publishes")}, MatrixRows: []int{161}},
	93:  {Refs: []testRef{ref("cli", "TestRecordResourcesTwoDomainOrdering", "partial-domain-when-staging-fails-after-git-success")}, MatrixRows: []int{162}},
	94:  {Refs: []testRef{ref("rescap", "TestEngineDoltCaptureEndToEnd", "")}, MatrixRows: []int{163}},
	95:  {Refs: []testRef{ref("cli", "TestResourcesFileCorruptIsExitThree", ""), ref("store", "TestResourcesManifestCorruptionAndCollision", "self-inconsistent-entry"), ref("store", "TestResourcesManifestCorruptionAndCollision", "two-entries-one-id")}, MatrixRows: []int{164}},
	96:  {Refs: []testRef{ref("rescap", "TestNonECHILDTerminalObserverError", ""), ref("rescap", "TestTriggerPriorityOrder", "")}, MatrixRows: []int{165}},
	97:  {Refs: []testRef{ref("rescap", "TestReapTimeoutDisclosesTwoResiduals", ""), ref("rescap", "TestBenignEntryPrimaryErrorIsFirstPhaseFailure", "")}, MatrixRows: []int{166}},
	98:  {Refs: []testRef{ref("rescap", "TestEngineDBPathIdentityChangedAfterExit", ""), ref("rescap", "TestSamePathIdentityDetectsReplacement", "")}, MatrixRows: []int{167}},
	99:  {Refs: []testRef{ref("cli", "TestCaptureAllOrNothingStaging", "")}, MatrixRows: []int{168}},
	100: {Refs: []testRef{ref("cli", "TestCaptureSubsetTargeting", "")}, MatrixRows: []int{169}},
	101: {Refs: []testRef{ref("rescap", "TestAnythingTrackedUnder", ""), ref("cli", "TestLocalPathTrackedRefusal", "")}, MatrixRows: []int{170}},
	102: {Refs: []testRef{ref("rescap", "TestHashExecutableDescriptorIsCopyFree", "")}, MatrixRows: []int{171}},
	103: {Refs: []testRef{ref("rescap", "TestMakeVerifiedPrivateCopy", "verified-copy-is-0500")}, MatrixRows: []int{172}},
	104: {Refs: []testRef{ref("rescap", "TestNoexecPreflightRunsBeforeTheCopyIsCreated", "")}, MatrixRows: []int{173}},
	105: {Refs: []testRef{ref("rescap", "TestPrivateCopyExactHostErrnosCleanUpAndStartNothing", "enospc-mid-write"), ref("rescap", "TestPrivateCopyExactHostErrnosCleanUpAndStartNothing", "eio-mid-write"), ref("rescap", "TestPrivateCopyExactHostErrnosCleanUpAndStartNothing", "eio-on-sync"), ref("rescap", "TestPrivateCopyExactHostErrnosCleanUpAndStartNothing", "enospc-on-sync"), ref("rescap", "TestPrivateCopyHostFailureStartsNoProcess", "")}, MatrixRows: []int{174}},
	106: {Refs: []testRef{ref("rescap", "TestNativeCrossBuildContract", ""), ref("rescap", "TestBuildTagContract", "")}, MatrixRows: []int{175}},
	107: {Refs: []testRef{ref("rescap", "TestDarwinObserverReturnsForAStoppedChild", ""), ref("rescap", "TestSIGKILLTerminatesAStoppedChild", ""), ref("rescap", "TestFinalizerCompletesAgainstAStoppedChild", "")}, MatrixRows: []int{176}},
	108: {Refs: []testRef{ref("rescap", "TestToleratedSignalErrnos", "")}, MatrixRows: []int{177}},
	109: {Refs: []testRef{ref("rescap", "TestECHILDFinalizerSendsNoSignals", ""), ref("rescap", "TestOwnerInducedReaderErrorsAreSuppressed", "")}, MatrixRows: []int{178}},
	110: {Refs: []testRef{ref("rescap", "TestGenuineReaderErrorClassification", ""), ref("rescap", "TestSetReadDeadlineFailureIsAdapterOutputReadFailed", "")}, MatrixRows: []int{179}},
	111: {Refs: []testRef{ref("cli", "TestCaptureCLIDrainTimeoutFromEscapedWriter", ""), ref("rescap", "TestEngineDrainTimeoutFromEscapedWriterPublishesNothing", "")}, MatrixRows: []int{180}},
	112: {Refs: []testRef{ref("rescap", "TestSingleCleanupOwner", ""), ref("rescap", "TestTriggerPriorityOrder", "")}, MatrixRows: []int{181}},
	113: {Refs: []testRef{ref("rescap", "TestTriggerPriorityOrder", "")}, MatrixRows: []int{182}},
	114: {Refs: []testRef{ref("rescap", "TestMultiErrorPrecedenceCapOutranksSignalAndDrain", "")}, MatrixRows: []int{183}},
	115: {Refs: []testRef{ref("rescap", "TestOwnerInducedReaderErrorsAreSuppressed", "")}, MatrixRows: []int{184}},
	116: {Refs: []testRef{ref("rescap", "TestStartFailureCarveOut", "")}, MatrixRows: []int{185}},
	117: {Refs: []testRef{ref("rescap", "TestEveryForcedCloseBranchJoinsBothDrains", "echild-finalizer-force-close"), ref("rescap", "TestEveryForcedCloseBranchJoinsBothDrains", "drain-deadline-expiry-force-close"), ref("rescap", "TestEveryForcedCloseBranchJoinsBothDrains", "setreaddeadline-failure-force-close")}, MatrixRows: []int{186}},
	118: {Refs: []testRef{ref("rescap", "TestReapTimeoutDisclosesTwoResiduals", "")}, MatrixRows: []int{187}},
	119: {Refs: []testRef{ref("rescap", "TestLateECHILDCutoffDrain", "")}, MatrixRows: []int{188}},
	120: {Refs: []testRef{ref("rescap", "TestWaitIsLaunchedStrictlyAfterTheSignalPhase", "benign-leader-event"), ref("rescap", "TestWaitIsLaunchedStrictlyAfterTheSignalPhase", "invocation-timeout"), ref("rescap", "TestWaitIsLaunchedStrictlyAfterTheSignalPhase", "output-cap-exceeded"), ref("rescap", "TestWaitIsLaunchedStrictlyAfterTheSignalPhase", "genuine-reader-error"), ref("rescap", "TestWaitIsLaunchedStrictlyAfterTheSignalPhase", "non-echild-terminal-observer-error")}, MatrixRows: []int{189}}}

// TestLedgerCoversEveryAcceptanceClause proves the ledger accounts for
// AC-1..AC-120 exactly, with no gaps, no extras and no empty claims.
func TestLedgerCoversEveryAcceptanceClause(t *testing.T) {
	if len(acLedger) != ACCount {
		t.Fatalf("ledger has %d entries, want %d", len(acLedger), ACCount)
	}
	for i := 1; i <= ACCount; i++ {
		entry, ok := acLedger[i]
		if !ok {
			t.Errorf("AC-%d has no ledger entry", i)
			continue
		}
		if len(entry.Refs) == 0 {
			t.Errorf("AC-%d references no test", i)
		}
		if len(entry.MatrixRows) == 0 {
			t.Errorf("AC-%d accounts for no matrix row", i)
		}
		for _, r := range entry.Refs {
			if _, ok := ledgerPackages[r.Package]; !ok {
				t.Errorf("AC-%d references unknown package %q", i, r.Package)
			}
			if !strings.HasPrefix(r.Test, "Test") {
				t.Errorf("AC-%d reference %s does not name a Go test function", i, r)
			}
		}
	}
	for i := range acLedger {
		if i < 1 || i > ACCount {
			t.Errorf("ledger contains out-of-range clause AC-%d", i)
		}
	}
}

// TestLedgerAccountsForEveryMatrixRow proves the union of every
// clause's rows is exactly the 189-row matrix, with no duplicates and
// no gaps.
func TestLedgerAccountsForEveryMatrixRow(t *testing.T) {
	seen := map[int]int{}
	for ac, entry := range acLedger {
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
		t.Fatalf("matrix rows with no ledger entry: %v", missing)
	}
}

// TestLedgerReferencesResolveExactly is the anti-rot guard: every
// reference must resolve to a real test in its OWN declared package,
// and every declared subtest must exist inside that test.
func TestLedgerReferencesResolveExactly(t *testing.T) {
	index := map[string]map[string]map[string]struct{}{}
	for pkg, dir := range ledgerPackages {
		tests, err := indexPackageTests(dir)
		if err != nil {
			t.Fatalf("indexing %s: %v", dir, err)
		}
		index[pkg] = tests
	}

	for ac := 1; ac <= ACCount; ac++ {
		entry, ok := acLedger[ac]
		if !ok {
			continue
		}
		for _, r := range entry.Refs {
			subtests, ok := index[r.Package][r.Test]
			if !ok {
				t.Errorf("AC-%d references %s, which is not declared in package %q",
					ac, r, r.Package)
				continue
			}
			if r.Subtest == "" {
				continue
			}
			if _, ok := subtests[r.Subtest]; !ok {
				names := make([]string, 0, len(subtests))
				for n := range subtests {
					names = append(names, n)
				}
				sort.Strings(names)
				t.Errorf("AC-%d references subtest %s, which %s does not declare (declared: %v)",
					ac, r, r.Test, names)
			}
		}
	}
}

// TestLedgerHasNoCrossPackageFallback documents, as an executable
// assertion, that resolution is package-scoped: a reference naming a
// real test under the wrong package tag must not resolve.
func TestLedgerHasNoCrossPackageFallback(t *testing.T) {
	index, err := indexPackageTests(ledgerPackages["store"])
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	// TestGoldenBatchIDVector genuinely exists — but in `store`.
	if _, ok := index["TestGoldenBatchIDVector"]; !ok {
		t.Fatal("fixture assumption broken: TestGoldenBatchIDVector is not in package store")
	}
	rescapIndex, err := indexPackageTests(ledgerPackages["rescap"])
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	if _, ok := rescapIndex["TestGoldenBatchIDVector"]; ok {
		t.Fatal("fixture assumption broken: TestGoldenBatchIDVector must not exist in package rescap")
	}
	// A ledger entry tagging it "rescap" therefore cannot resolve,
	// which is exactly the rev-0 defect this closes.
}

// indexPackageTests parses every _test.go file in dir and returns, per
// top-level test function, the set of subtest names it declares.
//
// Two declaration forms are recognized, both deterministic:
//
//   - a literal `t.Run("name", ...)` call;
//   - a string literal appearing in a composite literal inside the test
//     function, which is how this suite's table-driven cases name
//     themselves (`{name: "case", ...}` consumed by `t.Run(tc.name, …)`).
func indexPackageTests(dir string) (map[string]map[string]struct{}, error) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []*ast.File
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if parseErr != nil {
			return nil, parseErr
		}
		files = append(files, file)
	}
	out := map[string]map[string]struct{}{}
	{
		for _, file := range files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil || fn.Body == nil {
					continue
				}
				if !strings.HasPrefix(fn.Name.Name, "Test") {
					continue
				}
				names, ok := out[fn.Name.Name]
				if !ok {
					names = map[string]struct{}{}
					out[fn.Name.Name] = names
				}
				collectSubtestNames(fn.Body, names)
			}
		}
	}
	return out, nil
}

// collectSubtestNames walks a test body gathering DECLARED subtest
// names under exactly two recognized forms:
//
//  1. a literal `t.Run("name", ...)` call;
//  2. a table entry with an explicitly keyed `name: "..."` field
//     (any case spelling of the key), which is how this suite's
//     table-driven cases name themselves before `t.Run(tc.name, …)`.
//
// Rev-1 additionally accepted ANY positional string literal inside a
// composite literal, which let unrelated fixture strings — SQL
// fragments, expected stderr text, file bodies — masquerade as subtest
// names and made a mistyped reference resolve anyway. Positional
// literals are no longer accepted at all; a legitimately referenced
// table case must use a keyed `name` field, or the ledger must
// reference the top-level test honestly instead.
func collectSubtestNames(body *ast.BlockStmt, into map[string]struct{}) {
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			sel, ok := node.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Run" || len(node.Args) == 0 {
				return true
			}
			addStringLiteral(node.Args[0], into)
		case *ast.CompositeLit:
			for _, elt := range node.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					// Positional entries declare no name; ignore them.
					continue
				}
				key, ok := kv.Key.(*ast.Ident)
				if !ok || !strings.EqualFold(key.Name, "name") {
					continue
				}
				addStringLiteral(kv.Value, into)
			}
		}
		return true
	})
}

// addStringLiteral records a node when, and only when, it is a string
// literal.
func addStringLiteral(expr ast.Expr, into map[string]struct{}) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil || s == "" {
		return
	}
	into[s] = struct{}{}
}

// TestLedgerSubtestDiscoveryRejectsUnrelatedLiterals proves the rev-2
// parser restriction: only a literal `t.Run("name", ...)` or an
// explicitly keyed `name: "..."` table field declares a subtest.
//
// Rev-1 accepted ANY positional string literal inside a composite
// literal, so unrelated fixture strings — SQL fragments, expected
// stderr text, file bodies — resolved as if they were subtest names,
// and a mistyped reference passed. Each fixture below is parsed
// directly, so the assertions are about the discovery rule itself
// rather than about any particular test in this repository.
func TestLedgerSubtestDiscoveryRejectsUnrelatedLiterals(t *testing.T) {
	cases := []struct {
		name     string
		source   string
		accepted []string
		rejected []string
	}{
		{
			name: "literal-t-run-is-accepted",
			source: `package p
func TestFoo(t *testing.T) {
	t.Run("real-subtest", func(t *testing.T) {})
}`,
			accepted: []string{"real-subtest"},
			rejected: []string{"nope"},
		},
		{
			name: "keyed-name-field-is-accepted",
			source: `package p
func TestFoo(t *testing.T) {
	cases := []struct{ name, raw string }{
		{name: "keyed-case", raw: "SELECT * FROM users"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {})
	}
}`,
			accepted: []string{"keyed-case"},
			// The raw SQL fixture must NOT masquerade as a subtest.
			rejected: []string{"SELECT * FROM users"},
		},
		{
			name: "positional-table-entries-are-rejected",
			source: `package p
func TestFoo(t *testing.T) {
	cases := []struct{ name, raw string }{
		{"positional-case", "some fixture body"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {})
	}
}`,
			rejected: []string{"positional-case", "some fixture body"},
		},
		{
			name: "unrelated-literals-anywhere-are-rejected",
			source: `package p
func TestFoo(t *testing.T) {
	want := "adapter-copy-failed"
	body := []string{"#!/bin/sh", "exit 0"}
	_ = want
	_ = body
	t.Run("only-this-one", func(t *testing.T) {})
}`,
			accepted: []string{"only-this-one"},
			rejected: []string{"adapter-copy-failed", "#!/bin/sh", "exit 0"},
		},
		{
			name: "non-name-keyed-fields-are-rejected",
			source: `package p
func TestFoo(t *testing.T) {
	cases := []struct{ label, name string }{
		{label: "not-a-subtest", name: "actual-subtest"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {})
	}
}`,
			accepted: []string{"actual-subtest"},
			rejected: []string{"not-a-subtest"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "sample_test.go")
			if err := os.WriteFile(path, []byte(tc.source), 0o644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			index, err := indexPackageTests(dir)
			if err != nil {
				t.Fatalf("index: %v", err)
			}
			subtests, ok := index["TestFoo"]
			if !ok {
				t.Fatal("the fixture's top-level test was not indexed")
			}
			for _, want := range tc.accepted {
				if _, ok := subtests[want]; !ok {
					t.Errorf("%q should be a declared subtest, declared: %v", want, sortedKeys(subtests))
				}
			}
			for _, unwanted := range tc.rejected {
				if _, ok := subtests[unwanted]; ok {
					t.Errorf("%q must NOT resolve as a subtest, declared: %v", unwanted, sortedKeys(subtests))
				}
			}
		})
	}
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
