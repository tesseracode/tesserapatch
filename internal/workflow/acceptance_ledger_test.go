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
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func formatGoSource(src []byte) ([]byte, error) { return format.Source(src) }

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
		"AC-L129": {"TestACL129_EveryGitCallCarriesNoLazyFetch"},
		"AC-L134": {"TestACL134_GitFloorPreflight"},
		"AC-L135": {"TestACL135_DocsTotalityGuard", "TestACL135_GuardCoversTheNamedSections", "TestACL135_GuardIsSensitive"},

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
		"AC-L26": {"TestACL26_C0StepRunsUnderLCAllC"},
		"AC-L27": {"TestACL27_LadderIsMemoised"},
		"AC-L28": {"TestACL28_LandedRemediationsNeverRouteToReconcile"},

		// ── Group D ──────────────────────────────────────────────────
		"AC-L29":  {"TestACL29_AllCandidatesCollectedBeforeSelection"},
		"AC-L30":  {"TestACL30_QualificationIsForwardAtC1"},
		"AC-L31":  {"TestACL31_ForwardAndReverseAreInverted"},
		"AC-L32":  {"TestACL32_DriftInsideContextWindowDoesNotQualify"},
		"AC-L33":  {"TestACL33_ReRecordReLandSplitsAttestationFromAnchor"},
		"AC-L34":  {"TestACL34_EqualIdentityQualifiersSelectTopoOldest"},
		"AC-L35":  {"TestACL35_DifferingIdentityQualifiersFail"},
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
		"AC-L53": {"TestACL53_PresenceCrossProductIsTotalAndExclusive"},
		"AC-L54": {"TestACL54_CardinalityViolationsAreMalformed"},
		"AC-L55": {"TestACL55_RawOnlyMatchIsMalformed"},
		"AC-L56": {"TestACL56_SlugMatchingAndKeyCase", "TestACL56b_LowercaseTrailerKeyIsACandidate"},
		"AC-L57": {"TestACL57_BaseCommitLengthIsDerived"},
		"AC-L58": {"TestACL58_ReaderFailureIsUnavailable"},
		"AC-L59": {"TestACL59_UnreachableBaseCommitIsAdvisoryOnly"},
		"AC-L60": {"TestACL60_SingleEnumerationPerRun"},
		"AC-L61": {"TestACL61_RevListIsNeverInvoked"},
		"AC-L62": {"TestACL62_RecordsAreOldestFirst"},
		"AC-L63": {"TestACL63_InvocationBudget"},
		"AC-L64": {"TestACL64_RootLandingIsUnsupportedTopology"},
		"AC-L65": {"TestACL65_MergeLandingIsUnsupportedTopology"},
		"AC-L66": {"TestACL66_ShallowBoundaryIsShallowHistory"},
		"AC-L67": {"TestACL67_ShallowDiscriminatorIsThePreflight"},
		"AC-L68": {"TestACL68_RealFilteredCloneVerifiesNormally"},
		"AC-L69": {"TestACL69_MissingPromisorObjectIsHistoryIncomplete"},
		"AC-L70": {"TestACL70_NonFirstParentAndCherryPickAreFound"},
		"AC-L71": {"TestACL71_BranchSwitchDetachedHeadAndRewrite"},

		// ── Group F ──────────────────────────────────────────────────
		"AC-L72": {"TestACL72_PresenceTestIsThePatchLadder"},
		"AC-L73": {"TestACL73_LandedMemberWithCleanLadderIsSkipped"},
		"AC-L74": {"TestACL74_LandedAppendFileParentIsNotDoubleApplied"},
		"AC-L75": {"TestACL75_LandedReplaceInFileParentIsSkipped"},
		"AC-L76": {"TestACL76_LandedParentDriftFailsFast"},
		"AC-L77": {"TestACL77_UnattributedMaterializedParentIsSkipped"},
		"AC-L78": {"TestACL78_UnmaterializedParentIsReplayed"},
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
		"AC-L107": {"TestACL107_InventoryCoversEveryFeatureViaListFeatureEntries"},
		"AC-L108": {"TestACL108_ClassificationIsPureOverTheInventory"},
		"AC-L109": {"TestACL109_StagesConsumeCapturedBytes"},
		"AC-L110": {"TestACL110_SnapshotInstabilityIsDetected"},
		"AC-L111": {"TestACL111_UnreadableFeaturePolicy"},
		"AC-L112": {"TestACL112_InventoryOrderIsDeterministic"},
		"AC-L113": {"TestACL113_SchemaIsAdditiveSuperset"},
		"AC-L114": {"TestACL114_ClosedVocabularies"},
		"AC-L115": {"TestACL115_NoFreshnessLabelInVerifyReports"},
		"AC-L116": {"TestACL116_ModePresenceRule"},
		"AC-L117": {"TestACL117_RemediationGoldenStrings"},
		"AC-L118": {"TestACL118_HumanReportHeaderLines", "TestACL118CLI_HumanReportHeader"},
		"AC-L119": {"TestACL119_PersistedRecordFieldSetIsUnchanged"},
		"AC-L120": {"TestACL120_StickyClearingIsModeAgnostic"},
		"AC-L121": {"TestACL121_GH2RegressionTestIsUnmodified", "TestRunVerify_EquivalentRecipeAndPatchBothPass"},
		"AC-L122": {"TestACL122_GH2ResetHoldsAtAnchorH"},
		"AC-L123": {"TestACL123_ShadowPrunedOnEveryExitPath"},
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
		"AC-LD5":   {"TestACLD5_HexFormatsAreLowercaseAndDerived"},
		"AC-LD6":   {"TestACLD6_BaseCommitTrailerEqualsStatusAndIsNotWritten"},
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
		"AC-LD17":  {"TestACLD17_NoOutOfFormatTrailerValues"},
		"AC-LD18":  {"TestACLD18_ModeANoJournalRefusesWithoutMutating"},
		"AC-LD18a": {"TestACLD18a_ModeBRefusesAfterRecordWithRetainedArtifacts"},
		"AC-LD18b": {"TestACLD18b_RecordProducesAValidBaseCommit"},
		"AC-LD18c": {"TestACLD18c_ModeAPendingJournalRefusesAfterRecovery"},
		"AC-LD19":  {"TestACLD19_BaseCommitLengthIsDerivedFromObjectFormat"},
		"AC-LD20":  {"TestACLD20_UnreachableBaseWarnsButProceeds"},
		"AC-LD21":  {"TestACLD21_SuccessfulPathIsUnchanged"},
		"AC-LD22":  {"TestACLD22_LandValidationIsOffline"},
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

// TestAcceptanceLedger_TestsExist asserts every named test function is
// present in the repository's test sources — a ledger that names a
// non-existent test proves nothing.
func TestAcceptanceLedger_TestsExist(t *testing.T) {
	sources := testSourceBodies(t)
	for id, names := range acceptanceLedger() {
		for _, name := range names {
			decl := "func " + name + "("
			found := false
			for _, body := range sources {
				if strings.Contains(body, decl) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s names %s, which does not exist", id, name)
			}
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

// testSourceBodies returns every *_test.go body under internal/.
func testSourceBodies(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join(docsRootForTest(t), "internal")
	out := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		out[path] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return out
}
