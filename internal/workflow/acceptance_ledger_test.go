package workflow

// Acceptance ledger — v0.15.1 Wave C / GH #8.
//
// The 161 accepted rows (135 verify + 26 land) are mapped here to the
// Go test functions that prove them. The ledger is MECHANICALLY audited:
//
//  1. every AC id declared in the two PRDs appears in the ledger,
//  2. every ledger entry names a test function that actually exists in
//     the repository's *_test.go sources,
//  3. the counts match the accepted matrix sizes exactly.
//
// A row that cannot be placed at its stated tier must amend the PRD
// rather than be silently re-tiered (PRD-verify-freshness §7.1).

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func formatGoSource(src []byte) ([]byte, error) { return format.Source(src) }

type acceptanceLedgerTestRef struct {
	Package string
	Test    string
	Subtest string
}

func (r acceptanceLedgerTestRef) String() string {
	if r.Subtest == "" {
		return r.Package + ":" + r.Test
	}
	return r.Package + ":" + r.Test + "/" + r.Subtest
}

type acceptanceLedgerPackageSpec struct {
	Dir       string
	GoPackage string
}

var acceptanceLedgerPackageSpecs = map[string]acceptanceLedgerPackageSpec{
	"workflow": {Dir: ".", GoPackage: "workflow"},
	"cli":      {Dir: "../cli", GoPackage: "cli"},
	"gitutil":  {Dir: "../gitutil", GoPackage: "gitutil"},
}

// Most acceptance tests live in workflow. Every exception is explicit so a
// test moved to another package cannot continue satisfying the ledger merely
// because its name still exists somewhere under internal/.
var acceptanceLedgerPackageOverrides = map[string]string{
	"TestACL1_IssueSequencePassesBeforeLand":                   "cli",
	"TestACL118CLI_HumanReportHeader":                          "cli",
	"TestACL11CLI_RunIsReadOnly":                               "cli",
	"TestACL124CLI_VerifyAllOrderingAndReuse":                  "cli",
	"TestACL125CLI_ExitCodes":                                  "cli",
	"TestACL14CLI_LadderOutcomes":                              "cli",
	"TestACL2CLI_LandedFeaturePasses":                          "cli",
	"TestACL3_CommittedRangeReRecordBothBranches":              "cli",
	"TestACL39CLI_UnanchoredFails":                             "cli",
	"TestACL46CLI_ForwardModeUnchanged":                        "cli",
	"TestACL4CLI_LandedLeafPasses":                             "cli",
	"TestACL6CLI_NoWriteIsByteIdentical":                       "cli",
	"TestACL8CLI_DirtyWorktreePasses":                          "cli",
	"TestACLD1_FourTrailersAreParsedByGit":                     "cli",
	"TestACLD10_TrailerBlockIsTheLastParagraph":                "cli",
	"TestACLD11_LandingTwiceIsAllowed":                         "cli",
	"TestACLD12_LandRefusesWhenRecordCapturesNothing":          "cli",
	"TestACLD13_PreAmendmentLandingIsReadable":                 "cli",
	"TestACLD14_DryRunMutatesNothing":                          "cli",
	"TestACLD16_EarlierLandingStaysReachable":                  "cli",
	"TestACLD17_NoOutOfFormatTrailerValues":                    "cli",
	"TestACLD18_ModeANoJournalRefusesWithoutMutating":          "cli",
	"TestACLD18a_ModeBRefusesAfterRecordWithRetainedArtifacts": "cli",
	"TestACLD18b_RecordProducesAValidBaseCommit":               "cli",
	"TestACLD18c_ModeAPendingJournalRefusesAfterRecovery":      "cli",
	"TestACLD19_BaseCommitLengthIsDerivedFromObjectFormat":     "cli",
	"TestACLD2_TrailerCardinalityIsExactlyOne":                 "cli",
	"TestACLD20_UnreachableBaseWarnsButProceeds":               "cli",
	"TestACLD21_SuccessfulPathIsUnchanged":                     "cli",
	"TestACLD22_LandValidationIsOffline":                       "cli",
	"TestACLD23_LandDocumentIsAGuardInput":                     "cli",
	"TestACLD3_DigestsMatchArtifactBytes":                      "cli",
	"TestACLD4_RecipeSHANoneInBothProducerCases":               "cli",
	"TestACLD5_HexFormatsAreLowercaseAndDerived":               "cli",
	"TestACLD6_BaseCommitTrailerEqualsStatusAndIsNotWritten":   "cli",
	"TestACLD8_NoNewStatusField":                               "cli",
	"TestACLD9_LandingCommitIsSingleParent":                    "cli",
	"TestEvidenceCommandsForceCLocale":                         "gitutil",
	"TestRev1Land_NonCanonicalBaseCommitIsRefused":             "cli",
	"TestRev1Land_ObjectFormatLengthIsDerived":                 "cli",
	"TestRev1Land_TrailerCarriesTheValidatedValue":             "cli",
	"TestRev1Land_ValidationIsOfflineAndFloorAware":            "cli",
}

func acceptanceLedgerRef(name string) acceptanceLedgerTestRef {
	test, subtest, _ := strings.Cut(name, "/")
	pkg := "workflow"
	if override, ok := acceptanceLedgerPackageOverrides[test]; ok {
		pkg = override
	}
	return acceptanceLedgerTestRef{Package: pkg, Test: test, Subtest: subtest}
}

// acceptanceLedger maps AC id → the test(s) that prove it.
func acceptanceLedger() map[string][]string {
	return map[string][]string{
		// ── Group A ──────────────────────────────────────────────────
		"AC-L1": {"TestACL1_IssueSequencePassesBeforeLand"},
		"AC-L2": {"TestACL2_LandedFeaturePassesWithExactEvidence", "TestACL2CLI_LandedFeaturePasses"},
		"AC-L3": {"TestACL3_CommittedRangeReRecordBothBranches"},
		"AC-L4": {"TestACL4_LandedLeafPasses", "TestACL4CLI_LandedLeafPasses"},
		"AC-L5": {"TestACL5_ElevenCheckRowsInOrder", "TestACL135_DocsTotalityGuard"},
		"AC-L6": {"TestACL6_NoWriteLeavesEverythingByteIdentical", "TestACL6CLI_NoWriteIsByteIdentical"},

		// ── Group B ──────────────────────────────────────────────────
		"AC-L7":   {"TestACL7_EveryApplyCallIsCached"},
		"AC-L8":   {"TestACL8_DirtyWorktreeStillPasses", "TestACL8CLI_DirtyWorktreePasses"},
		"AC-L9":   {"TestACL9_DirtyIndexStillPasses"},
		"AC-L10":  {"TestACL10_WorktreeOnlyFeatureBlocksAtAnchorC"},
		"AC-L11":  {"TestACL11_RunIsReadOnlyOnIndexAndWorktree", "TestACL11CLI_RunIsReadOnly"},
		"AC-L12":  {"TestACL12_TempIndexRemovedOnEveryExitPath"},
		"AC-L13":  {"TestACL13_TempIndexOutsideTrackedTree"},
		"AC-L129": {"TestACL129_EveryGitCallCarriesNoLazyFetch", "TestRev1_EveryVerifyGitCallIsOffline", "TestRev1_NoLegacyGitHelperInVerifyPath", "TestRev3_VerifyRunsUnderCLocale"},
		"AC-L134": {"TestACL134_GitFloorPreflight", "TestRev1_BelowFloorIssuesOnlyVersion", "TestRev1_GitGateRefusesWithoutSpawning", "TestRev3_VerifyRunsUnderCLocale"},
		"AC-L135": {"TestACL135_DocsTotalityGuard", "TestACL135_GuardCoversTheNamedSections", "TestACL135_GuardIsSensitive", "TestAcceptanceLedger_VerifyContractAnchorsResolve", "TestAcceptanceLedger_ContractAnchorGuardIsSensitive"},

		// ── Group C ──────────────────────────────────────────────────
		"AC-L14": {"TestACL14_LadderStep1CleanNoAdvisory", "TestACL14CLI_LadderOutcomes"},
		"AC-L15": {"TestACL15_OffsetAndFarEditsStayClean"},
		"AC-L16": {"TestACL16_TwoLinesAwayIsContextDriftWarn", "TestACL14CLI_LadderOutcomes"},
		"AC-L17": {"TestACL17_OneLineAwayBlocksWithR2"},
		"AC-L18": {"TestACL18_RevertInPlacePlusPastedTextBlocks"},
		"AC-L19": {"TestACL19_PartialRevertHunk1Blocks"},
		"AC-L20": {"TestACL20_PartialRevertHunk2Blocks"},
		"AC-L21": {"TestACL21_PartialRevertHunk3Blocks"},
		"AC-L22": {"TestACL22_PartialRevertHunks1And3Blocks"},
		"AC-L23": {"TestACL23_FullRevertBlocks", "TestACL14CLI_LadderOutcomes"},
		"AC-L24": {"TestACL24_DeletedFileBlocks"},
		"AC-L25": {"TestACL25_DegenerateWholeFileHunkIsContextDrift"},
		"AC-L26": {"TestACL26_C0StepRunsUnderLCAllC", "TestRev3_VerifyRunsUnderCLocale", "TestEvidenceCommandsForceCLocale"},
		"AC-L27": {"TestACL27_LadderIsMemoised"},
		"AC-L28": {"TestACL28_LandedRemediationsNeverRouteToReconcile"},

		// ── Group D ──────────────────────────────────────────────────
		"AC-L29":  {"TestACL29_AllCandidatesCollectedBeforeSelection"},
		"AC-L30":  {"TestACL30_QualificationIsForwardAtC1"},
		"AC-L31":  {"TestACL31_ForwardAndReverseAreInverted"},
		"AC-L32":  {"TestACL32_DriftInsideContextWindowDoesNotQualify"},
		"AC-L33":  {"TestACL33_ReRecordReLandSplitsAttestationFromAnchor"},
		"AC-L34":  {"TestACL34_EqualIdentityQualifiersSelectTopoOldest"},
		"AC-L35":  {"TestACL35_DifferingIdentityQualifiersFail", "TestRev2_DuplicateAttestationTrueAmbiguityStillUsesR7"},
		"AC-L36":  {"TestACL36_SelectionIsDeterministic"},
		"AC-L37":  {"TestACL37_AnchorSearchNeverBroadens"},
		"AC-L38":  {"TestACL38_StaleCandidateSuppliesOnlyATree"},
		"AC-L39":  {"TestACL39_NoQualifierFailsWithModePresent", "TestACL39CLI_UnanchoredFails"},
		"AC-L40":  {"TestACL40_UnavailableAnchorFailsEvenWhenCurrentIsClean"},
		"AC-L41":  {"TestACL41_ReLandRegainsAnchorOrFailsWithR11", "TestACL3_CommittedRangeReRecordBothBranches"},
		"AC-L42":  {"TestACL42_NormalizedIdentityArgv"},
		"AC-L43":  {"TestACL43_NormalizationMeasuredBehaviour"},
		"AC-L44":  {"TestACL44_NormalizationPreservesModeBinaryRename"},
		"AC-L45":  {"TestACL45_EmptyPathSetIsAmbiguous"},
		"AC-L130": {"TestACL130_ParentRevisionSyntax"},
		"AC-L131": {"TestACL131_QualificationLadderTable"},
		"AC-L132": {"TestACL132_RemediationLoopQualifiesAtC1"},
		"AC-L133": {"TestACL133_OffsetNormalizationTrade"},

		// ── Group E ──────────────────────────────────────────────────
		"AC-L46": {"TestACL46_NoEvidenceStaysForward", "TestACL46CLI_ForwardModeUnchanged"},
		"AC-L47": {"TestACL47_AllThreeValuesMatchIsExact"},
		"AC-L48": {"TestACL48_PatchSHAMismatchIsStale"},
		"AC-L49": {"TestACL49_RecipeAndBaseMismatchAreStale"},
		"AC-L50": {"TestACL50_RecipeSHANoneMatchesAbsentAndWhitespace"},
		"AC-L51": {"TestACL51_PresencePrecedesDigest"},
		"AC-L52": {"TestACL52_PresentEmptyPatchShortCircuits"},
		"AC-L53": {"TestACL53_PresenceCrossProductIsTotalAndExclusive", "TestRev2_V2ParsesCapturedRecipeBytes"},
		"AC-L54": {"TestACL54_CardinalityViolationsAreMalformed"},
		"AC-L55": {"TestACL55_RawOnlyMatchIsMalformed"},
		"AC-L56": {"TestACL56_SlugMatchingAndKeyCase", "TestACL56b_LowercaseTrailerKeyIsACandidate"},
		"AC-L57": {"TestACL57_BaseCommitLengthIsDerived"},
		"AC-L58": {"TestACL58_ReaderFailureIsUnavailable", "TestRev1_GenericProbeFailureIsUnavailable", "TestRev2_DuplicateAttestationGenericDiffFailureIsUnavailable", "TestRev2_HistoricalV8ExecutionFailureIsUnavailable", "TestRev3_HistoricalV8BroadPhraseFailureIsUnavailable", "TestRev3_LadderBroadPhraseFailureIsNotAnAnswer"},
		"AC-L59": {"TestACL59_UnreachableBaseCommitIsAdvisoryOnly", "TestRev1_BaseCommitUnreachableAdvisoryIsEmitted"},
		"AC-L60": {"TestACL60_SingleEnumerationPerRun"},
		"AC-L61": {"TestACL61_RevListIsNeverInvoked"},
		"AC-L62": {"TestACL62_RecordsAreOldestFirst"},
		"AC-L63": {"TestACL63_InvocationBudget"},
		"AC-L64": {"TestACL64_RootLandingIsUnsupportedTopology"},
		"AC-L65": {"TestACL65_MergeLandingIsUnsupportedTopology"},
		"AC-L66": {"TestACL66_ShallowBoundaryIsShallowHistory"},
		"AC-L67": {"TestACL67_ShallowDiscriminatorIsThePreflight"},
		"AC-L68": {"TestACL68_RealFilteredCloneVerifiesNormally"},
		"AC-L69": {"TestACL69_MissingPromisorObjectIsHistoryIncomplete", "TestRev2_DuplicateAttestationMissingObjectIsHistoryIncomplete", "TestRev2_HistoricalV8MissingObjectIsHistoryIncomplete", "TestRev3_HistoricalV8MissingObjectStillReachesR22", "TestRev3_LadderMissingObjectStillReachesR22"},
		"AC-L70": {"TestACL70_NonFirstParentAndCherryPickAreFound"},
		"AC-L71": {"TestACL71_BranchSwitchDetachedHeadAndRewrite"},

		// ── Group F ──────────────────────────────────────────────────
		"AC-L72": {"TestACL72_PresenceTestIsThePatchLadder"},
		"AC-L73": {"TestACL73_LandedMemberWithCleanLadderIsSkipped"},
		"AC-L74": {"TestACL74_LandedAppendFileParentIsNotDoubleApplied"},
		"AC-L75": {"TestACL75_LandedReplaceInFileParentIsSkipped"},
		"AC-L76": {"TestACL76_LandedParentDriftFailsFast"},
		"AC-L77": {"TestACL77_UnattributedMaterializedParentIsSkipped", "TestRev2_UnlandedParentAnsweredAbsenceStillReplays"},
		"AC-L78": {"TestACL78_UnmaterializedParentIsReplayed", "TestRev2_UnlandedParentProbeFailureNeverReplays", "TestRev2_UnlandedParentAnsweredAbsenceStillReplays"},
		"AC-L79": {"TestACL79_LandedMemberWithoutPatchIsTerminal"},
		"AC-L80": {"TestACL80_PatchIsSoleAuthorityWhenRecipeIsUnusable"},
		"AC-L81": {"TestACL81_LandedMemberWithNoUsableArtifact"},
		"AC-L82": {"TestACL82_ParentEvidenceIntegrityFailsFast"},
		"AC-L83": {"TestACL83_UnappliedParentFailsWithR16"},
		"AC-L84": {"TestACL84_RejectedParentFailsWithR17"},
		"AC-L85": {"TestACL85_UpstreamMergedAndSupersededParentsAreSkipped"},
		"AC-L86": {"TestACL86_ActiveParentIsTreatedAsApplied"},
		"AC-L87": {"TestACL87_AllFourActiveSitesAgree"},
		"AC-L88": {"TestACL88_RevertTimingIsReportedAtBothAnchors"},
		"AC-L89": {"TestACL89_ParentLandingOrderDoesNotChangeTheVerdict"},
		"AC-L90": {"TestACL90_MixedChainUnlandedTarget"},
		"AC-L91": {"TestACL91_MixedChainLandedTarget"},

		// ── Group G ──────────────────────────────────────────────────
		"AC-L92":  {"TestACL92_PerMemberV10Baselines"},
		"AC-L93":  {"TestACL93_LandedParentPassesAtItsOwnBaseline"},
		"AC-L94":  {"TestACL94_LandedTargetPreimageAtClosureBaseline"},
		"AC-L95":  {"TestACL95_LandedNewFilePreimagePasses"},
		"AC-L96":  {"TestACL96_ForwardModeUsesProvenanceAnchor"},
		"AC-L97":  {"TestACL97_UnboundProvenanceIsAcceptedAndReported"},
		"AC-L98":  {"TestACL98_MismatchedProvenanceHashIsRejected"},
		"AC-L99":  {"TestACL99_UnusableProvenanceFailsWithR24"},
		"AC-L100": {"TestACL100_NeverReadsTheLiveWorktreeForPreimages"},
		"AC-L101": {"TestACL101_LegacyOpsNeedNoProvenance"},
		"AC-L102": {"TestACL102_MalformedPreimageHashBlocks"},
		"AC-L103": {"TestACL103_LaterTouchIsMetadataNotBytes"},
		"AC-L104": {"TestACL104_GenuineLaterTouchIsWarnOnly"},
		"AC-L105": {"TestACL105_SupersessionDowngradeAndV2Skip"},
		"AC-L106": {"TestACL106_ParentV10Aggregation"},

		// ── Group H ──────────────────────────────────────────────────
		"AC-L107": {"TestACL107_InventoryCoversEveryFeatureViaListFeatureEntries", "TestRev1_InventoryIsReadExactlyOnce"},
		"AC-L108": {"TestACL108_ClassificationIsPureOverTheInventory", "TestRev1_ReportIsBuiltFromTheCapture", "TestRev2_VerifyPathHasNoLiveArtifactReads"},
		"AC-L109": {"TestACL109_StagesConsumeCapturedBytes", "TestRev1_InventoryIsReadExactlyOnce", "TestRev2_V2ParsesCapturedRecipeBytes"},
		"AC-L110": {"TestACL110_SnapshotInstabilityIsDetected", "TestRev2_ReadabilityTransitionsAreUnstable", "TestRev2_WorkspaceMutationDuringRunIsUnstable", "TestRev2_StableUnreadableFeatureIsNotUnstable"},
		"AC-L111": {"TestACL111_UnreadableFeaturePolicy", "TestRev1_ArtifactReadErrorIsNotAbsence", "TestRev1_GenerationsParseErrorIsRetained", "TestRev1_UnrelatedArtifactErrorIsAdvisoryOnly"},
		"AC-L112": {"TestACL112_InventoryOrderIsDeterministic"},
		"AC-L113": {"TestACL113_SchemaIsAdditiveSuperset"},
		"AC-L114": {"TestACL114_ClosedVocabularies", "TestRev1_BaseCommitUnreachableAdvisoryIsEmitted"},
		"AC-L115": {"TestACL115_NoFreshnessLabelInVerifyReports"},
		"AC-L116": {"TestACL116_ModePresenceRule"},
		"AC-L117": {"TestACL117_RemediationGoldenStrings", "TestRev2_HistoricalV8GenuineNonApplyIsR5", "TestRev2_HistoricalV8MalformedPatchStaysAnAnswer", "TestRev3_MalformedPatchStillGetsAPatchAnswer"},
		"AC-L118": {"TestACL118_HumanReportHeaderLines", "TestACL118CLI_HumanReportHeader"},
		"AC-L119": {"TestACL119_PersistedRecordFieldSetIsUnchanged", "TestRev1_PersistenceDoesNotReload"},
		"AC-L120": {"TestACL120_StickyClearingIsModeAgnostic"},
		"AC-L121": {"TestACL121_GH2RegressionTestIsUnmodified", "TestRunVerify_EquivalentRecipeAndPatchBothPass"},
		"AC-L122": {"TestACL122_GH2ResetHoldsAtAnchorH"},
		"AC-L123": {"TestACL123_ShadowPrunedOnEveryExitPath", "TestRev2_HistoricalV8ExecutionFailureIsUnavailable", "TestRev3_HistoricalV8BroadPhraseFailureIsUnavailable"},
		"AC-L124": {"TestACL124CLI_VerifyAllOrderingAndReuse"},
		"AC-L125": {"TestACL125CLI_ExitCodes"},
		"AC-L126": {"TestACL126_ReplaceInFilePredicateIsSound"},
		"AC-L127": {"TestACL127_DiagnosticsNeverCertify"},
		"AC-L128": {"TestACL128_ToolchainGates"},

		// ── PRD-tpatch-land §6.2 ─────────────────────────────────────
		"AC-LD1":   {"TestACLD1_FourTrailersAreParsedByGit"},
		"AC-LD2":   {"TestACLD2_TrailerCardinalityIsExactlyOne"},
		"AC-LD3":   {"TestACLD3_DigestsMatchArtifactBytes"},
		"AC-LD4":   {"TestACLD4_RecipeSHANoneInBothProducerCases"},
		"AC-LD5":   {"TestACLD5_HexFormatsAreLowercaseAndDerived", "TestRev1Land_TrailerCarriesTheValidatedValue"},
		"AC-LD6":   {"TestACLD6_BaseCommitTrailerEqualsStatusAndIsNotWritten", "TestRev1Land_TrailerCarriesTheValidatedValue"},
		"AC-LD7":   {"TestACLD6_BaseCommitTrailerEqualsStatusAndIsNotWritten"},
		"AC-LD8":   {"TestACLD8_NoNewStatusField"},
		"AC-LD9":   {"TestACLD9_LandingCommitIsSingleParent"},
		"AC-LD10":  {"TestACLD10_TrailerBlockIsTheLastParagraph"},
		"AC-LD11":  {"TestACLD11_LandingTwiceIsAllowed"},
		"AC-LD12":  {"TestACLD12_LandRefusesWhenRecordCapturesNothing"},
		"AC-LD13":  {"TestACLD13_PreAmendmentLandingIsReadable"},
		"AC-LD14":  {"TestACLD14_DryRunMutatesNothing"},
		"AC-LD15":  {"TestACLD21_SuccessfulPathIsUnchanged"},
		"AC-LD16":  {"TestACLD16_EarlierLandingStaysReachable"},
		"AC-LD17":  {"TestACLD17_NoOutOfFormatTrailerValues", "TestRev1Land_NonCanonicalBaseCommitIsRefused"},
		"AC-LD18":  {"TestACLD18_ModeANoJournalRefusesWithoutMutating", "TestRev1Land_NonCanonicalBaseCommitIsRefused"},
		"AC-LD18a": {"TestACLD18a_ModeBRefusesAfterRecordWithRetainedArtifacts"},
		"AC-LD18b": {"TestACLD18b_RecordProducesAValidBaseCommit"},
		"AC-LD18c": {"TestACLD18c_ModeAPendingJournalRefusesAfterRecovery"},
		"AC-LD19":  {"TestACLD19_BaseCommitLengthIsDerivedFromObjectFormat", "TestRev1Land_ObjectFormatLengthIsDerived"},
		"AC-LD20":  {"TestACLD20_UnreachableBaseWarnsButProceeds"},
		"AC-LD21":  {"TestACLD21_SuccessfulPathIsUnchanged"},
		"AC-LD22":  {"TestACLD22_LandValidationIsOffline", "TestRev1Land_ValidationIsOfflineAndFloorAware"},
		"AC-LD23":  {"TestACLD23_LandDocumentIsAGuardInput"},
	}
}

// TestAcceptanceLedger_CoversEveryDeclaredRow asserts the ledger is
// TOTAL against the two accepted matrices.
func TestAcceptanceLedger_CoversEveryDeclaredRow(t *testing.T) {
	ledger := acceptanceLedger()
	declared := declaredAcceptanceIDs(t)

	var missing []string
	for id := range declared {
		if _, ok := ledger[id]; !ok {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%d accepted row(s) have no ledger entry: %v", len(missing), missing)
	}

	var extra []string
	for id := range ledger {
		if !declared[id] {
			extra = append(extra, id)
		}
	}
	sort.Strings(extra)
	if len(extra) > 0 {
		t.Errorf("%d ledger entr(ies) name a row the PRDs do not declare: %v", len(extra), extra)
	}

	verify, land := 0, 0
	for id := range ledger {
		if strings.HasPrefix(id, "AC-LD") {
			land++
			continue
		}
		verify++
	}
	if verify != 135 {
		t.Errorf("verify matrix size = %d, want 135", verify)
	}
	if land != 26 {
		t.Errorf("land matrix size = %d, want 26", land)
	}
	if verify+land != 161 {
		t.Errorf("total = %d, want 161", verify+land)
	}
}

// TestAcceptanceLedger_TestsExist resolves every reference as an exact
// package/top-level-test/optional-subtest triple using Go's parser. Comments,
// string fixtures and declarations in the wrong package cannot satisfy it.
func TestAcceptanceLedger_TestsExist(t *testing.T) {
	index := acceptanceLedgerTestIndex(t)
	for id, names := range acceptanceLedger() {
		for _, name := range names {
			ref := acceptanceLedgerRef(name)
			if !acceptanceLedgerRefResolves(index, ref) {
				t.Errorf("%s names %s, which does not resolve exactly", id, ref)
			}
		}
	}
}

func acceptanceLedgerTestIndex(t *testing.T) map[string]map[string]map[string]struct{} {
	t.Helper()
	index := map[string]map[string]map[string]struct{}{}
	root := filepath.Join(docsRootForTest(t), "internal", "workflow")
	for pkg, spec := range acceptanceLedgerPackageSpecs {
		tests, err := indexAcceptanceLedgerPackageTests(
			filepath.Clean(filepath.Join(root, spec.Dir)),
			spec.GoPackage,
		)
		if err != nil {
			t.Fatalf("index %s: %v", pkg, err)
		}
		index[pkg] = tests
	}
	return index
}

func acceptanceLedgerRefResolves(index map[string]map[string]map[string]struct{}, ref acceptanceLedgerTestRef) bool {
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
	_, ok = subtests[ref.Subtest]
	return ok
}

func indexAcceptanceLedgerPackageTests(dir, goPackage string) (map[string]map[string]struct{}, error) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
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
		testingAliases, dotTesting := acceptanceLedgerTestingImports(file)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !isRunnableAcceptanceLedgerTest(fn, testingAliases, dotTesting) {
				continue
			}
			subtests := map[string]struct{}{}
			collectAcceptanceLedgerSubtests(fn.Body, acceptanceLedgerTestVariables(fn), subtests)
			out[fn.Name.Name] = subtests
		}
	}
	return out, nil
}

func acceptanceLedgerTestingImports(file *ast.File) (map[string]struct{}, bool) {
	aliases := map[string]struct{}{}
	dot := false
	for _, imp := range file.Imports {
		importPath, err := strconv.Unquote(imp.Path.Value)
		if err != nil || importPath != "testing" {
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
			// A blank import cannot name testing.T.
		default:
			aliases[imp.Name.Name] = struct{}{}
		}
	}
	return aliases, dot
}

func isRunnableAcceptanceLedgerTest(fn *ast.FuncDecl, testingAliases map[string]struct{}, dotTesting bool) bool {
	if fn.Recv != nil || fn.Body == nil || fn.Name == nil || !validGoTestName(fn.Name.Name) {
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
	_, ok = testingAliases[pkg.Name]
	return ok
}

func validGoTestName(name string) bool {
	if !strings.HasPrefix(name, "Test") || len(name) == len("Test") {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name[len("Test"):])
	return !unicode.IsLower(r)
}

func acceptanceLedgerTestVariables(fn *ast.FuncDecl) map[string]struct{} {
	out := map[string]struct{}{}
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return out
	}
	for _, name := range fn.Type.Params.List[0].Names {
		out[name.Name] = struct{}{}
	}
	return out
}

func collectAcceptanceLedgerSubtests(body *ast.BlockStmt, testVariables map[string]struct{}, into map[string]struct{}) {
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Run" {
			return true
		}
		receiver, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if _, ok := testVariables[receiver.Name]; ok {
			addAcceptanceLedgerString(call.Args[0], into)
		}
		return true
	})
}

func addAcceptanceLedgerString(expr ast.Expr, into map[string]struct{}) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return
	}
	value, err := strconv.Unquote(lit.Value)
	if err == nil && value != "" {
		into[value] = struct{}{}
	}
}

func TestAcceptanceLedger_ASTResolutionRejectsFalsePositives(t *testing.T) {
	dir := t.TempDir()
	source := `package fixture
import "testing"
// func TestGhostRowNeverImplemented(t *testing.T) {}
func TestReal(t *testing.T) {
	t.Run("real-subtest", func(t *testing.T) {})
	_ = []struct{ name string }{{name: "table-subtest"}}
	_ = []string{"not-a-subtest"}
}
func TestNotRunnable() {}`
	if !strings.Contains(source, "func TestGhostRowNeverImplemented(") {
		t.Fatal("fixture no longer reproduces the retired raw-text false positive")
	}
	if err := os.WriteFile(filepath.Join(dir, "fixture_test.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	externalSource := `package fixture_test
import "testing"
func TestExternalOnly(t *testing.T) {}`
	if err := os.WriteFile(filepath.Join(dir, "external_test.go"), []byte(externalSource), 0o644); err != nil {
		t.Fatal(err)
	}
	tests, err := indexAcceptanceLedgerPackageTests(dir, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	index := map[string]map[string]map[string]struct{}{"fixture": tests}
	for _, ref := range []acceptanceLedgerTestRef{
		{Package: "fixture", Test: "TestGhostRowNeverImplemented"},
		{Package: "wrong-package", Test: "TestReal"},
		{Package: "fixture", Test: "TestNotRunnable"},
		{Package: "fixture", Test: "TestExternalOnly"},
		{Package: "fixture", Test: "TestReal", Subtest: "missing-subtest"},
		{Package: "fixture", Test: "TestReal", Subtest: "not-a-subtest"},
		{Package: "fixture", Test: "TestReal", Subtest: "table-subtest"},
	} {
		if acceptanceLedgerRefResolves(index, ref) {
			t.Errorf("false-positive reference resolved: %s", ref)
		}
	}
	for _, ref := range []acceptanceLedgerTestRef{
		{Package: "fixture", Test: "TestReal"},
		{Package: "fixture", Test: "TestReal", Subtest: "real-subtest"},
	} {
		if !acceptanceLedgerRefResolves(index, ref) {
			t.Errorf("real declaration did not resolve: %s", ref)
		}
	}
}

func TestAcceptanceLedger_VerifyContractAnchorsResolve(t *testing.T) {
	root := docsRootForTest(t)
	anchorRE := regexp.MustCompile(`internal/workflow/(verify(?:_[a-z_]+)?\.go):([0-9]+)(?:-([0-9]+))?`)
	lineCounts := map[string]int{}
	checked := 0

	for _, rel := range []string{
		"docs/adrs/ADR-013-verify-freshness-overlay.md",
		"docs/prds/PRD-verify-freshness.md",
		"docs/prds/PRD-tpatch-land.md",
	} {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for _, match := range anchorRE.FindAllStringSubmatch(string(data), -1) {
			sourceRel := filepath.Join("internal", "workflow", match[1])
			lines, ok := lineCounts[sourceRel]
			if !ok {
				source, err := os.ReadFile(filepath.Join(root, sourceRel))
				if err != nil {
					t.Fatalf("%s cites missing source %s: %v", rel, sourceRel, err)
				}
				lines = acceptanceLedgerSourceLineCount(source)
				lineCounts[sourceRel] = lines
			}
			start, _ := strconv.Atoi(match[2])
			end := start
			if match[3] != "" {
				end, _ = strconv.Atoi(match[3])
			}
			if !contractAnchorWithinFile(lines, start, end) {
				t.Errorf("%s cites %s:%d-%d, but the file has %d lines", rel, sourceRel, start, end, lines)
			}
			checked++
		}
	}
	if checked < 60 {
		t.Fatalf("verified only %d source anchors; guard may be vacuous", checked)
	}
}

func contractAnchorWithinFile(lineCount, start, end int) bool {
	return start > 0 && end >= start && end <= lineCount
}

func acceptanceLedgerSourceLineCount(source []byte) int {
	if len(source) == 0 {
		return 0
	}
	lines := strings.Count(string(source), "\n")
	if source[len(source)-1] != '\n' {
		lines++
	}
	return lines
}

func TestAcceptanceLedger_ContractAnchorGuardIsSensitive(t *testing.T) {
	for _, tc := range []struct {
		name             string
		lineCount, start int
		end              int
		want             bool
	}{
		{name: "single-valid", lineCount: 10, start: 10, end: 10, want: true},
		{name: "range-valid", lineCount: 10, start: 2, end: 9, want: true},
		{name: "past-eof", lineCount: 10, start: 9, end: 11, want: false},
		{name: "reversed", lineCount: 10, start: 8, end: 7, want: false},
		{name: "zero", lineCount: 10, start: 0, end: 1, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := contractAnchorWithinFile(tc.lineCount, tc.start, tc.end); got != tc.want {
				t.Errorf("contractAnchorWithinFile(%d, %d, %d) = %v, want %v",
					tc.lineCount, tc.start, tc.end, got, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		source string
		want   int
	}{
		{source: "", want: 0},
		{source: "one", want: 1},
		{source: "one\n", want: 1},
		{source: "one\ntwo", want: 2},
		{source: "one\ntwo\n", want: 2},
	} {
		if got := acceptanceLedgerSourceLineCount([]byte(tc.source)); got != tc.want {
			t.Errorf("acceptanceLedgerSourceLineCount(%q) = %d, want %d", tc.source, got, tc.want)
		}
	}
}

// AC-L128 — the toolchain gates. `gofmt -l .`, `go build ./cmd/tpatch`,
// `go test ./...` and `make wave-close-check` are run by the wave-close
// gate; this row asserts the two that are safe to assert in-process:
// the package tree is gofmt-clean and every Wave C source file compiles
// (which it must, since this test is running).
func TestACL128_ToolchainGates(t *testing.T) {
	repo := docsRootForTest(t)
	var unformatted []string
	err := filepath.Walk(filepath.Join(repo, "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		src, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		formatted, ferr := formatGoSource(src)
		if ferr != nil {
			unformatted = append(unformatted, path+": "+ferr.Error())
			return nil
		}
		if string(formatted) != string(src) {
			rel, _ := filepath.Rel(repo, path)
			unformatted = append(unformatted, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal/: %v", err)
	}
	if len(unformatted) > 0 {
		t.Errorf("gofmt -l reports %d file(s): %v", len(unformatted), unformatted)
	}
}

// declaredAcceptanceIDs scrapes the two accepted matrices.
func declaredAcceptanceIDs(t *testing.T) map[string]bool {
	t.Helper()
	root := docsRootForTest(t)
	idRe := regexp.MustCompile(`^\| (AC-LD?\d+[a-c]?) \|`)
	out := map[string]bool{}
	for _, rel := range []string{
		"docs/prds/PRD-verify-freshness.md",
		"docs/prds/PRD-tpatch-land.md",
	} {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if m := idRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
				out[m[1]] = true
			}
		}
	}
	if len(out) == 0 {
		t.Fatalf("no acceptance ids scraped — the ledger audit would be vacuous")
	}
	return out
}
