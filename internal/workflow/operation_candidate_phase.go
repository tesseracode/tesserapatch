package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/candidate"
	"github.com/tesseracode/tesserapatch/internal/store"
)

var ErrOperationCandidateRecoveryRequired = errors.New("recovery-required")
var ErrOperationCandidatePersistence = errors.New("operation candidate persistence failed")

var operationCandidateBeforeSummaryWrite = func() {}

type phase2AuthorityProof struct {
	owner              *store.Store
	result             *ReconcileResult
	repoPath           string
	repoIdentity       os.FileInfo
	upstreamCommit     string
	referenceCommit    string
	captureEventSHA256 string
	coverageSHA256     string
	patchSHA256        string
	recipeSHA256       string
	sourceFeatureState string
	gatesPassed        [15]bool
	effectCount        int
	effectDigest       string
	presentCount       int
	applicableCount    int
	conflictCount      int
	operationCount     int
}

type operationPhaseSnapshot struct {
	owner           *store.Store
	repoPath        string
	repoIdentity    os.FileInfo
	invocation      operationCandidateInvocation
	eligibility     operationCandidateEligibility
	derivation      candidate.CandidateDerivation
	presentCount    int
	applicableCount int
	conflictCount   int
	operationCount  int
	effectCount     int
	effectDigest    string
	conflictCode    string
}

func captureOperationPhaseSnapshot(s *store.Store, slug string, invocation operationCandidateInvocation) (*operationPhaseSnapshot, error) {
	eligibility, err := assessOperationCandidateEligibility(s, slug, invocation)
	if err != nil {
		return nil, err
	}
	snapshot := &operationPhaseSnapshot{owner: s, repoPath: s.Root, eligibility: eligibility, invocation: invocation}
	snapshot.invocation.slugs = slices.Clone(invocation.slugs)
	if !eligibility.enabled || !eligibility.eligible || eligibility.inputs == nil {
		return snapshot, nil
	}
	snapshot.repoIdentity, err = os.Stat(s.Root)
	if err != nil {
		return nil, err
	}
	reference, err := openOperationCandidateReference(s.Root, eligibility.referenceCommit)
	if err != nil {
		return nil, err
	}
	defer reference.Close()
	upstream, err := openOperationCandidateReference(s.Root, eligibility.upstreamCommit)
	if err != nil {
		return nil, err
	}
	defer upstream.Close()
	in := eligibility.inputs
	snapshot.presentCount, snapshot.applicableCount, snapshot.conflictCount, snapshot.operationCount, snapshot.conflictCode, err =
		observeOperationPhaseCounts(reference, upstream, in)
	if err != nil {
		return nil, err
	}
	snapshot.effectCount = len(in.effects)
	if snapshot.presentCount+snapshot.applicableCount+snapshot.conflictCount != snapshot.effectCount {
		return nil, fmt.Errorf("effect-unexplained")
	}
	if snapshot.conflictCount == 0 && snapshot.applicableCount > 0 {
		snapshot.derivation, err = candidate.DescribeDerivation(reference, upstream, in.patch, in.effects, in.effectHashes)
		if err != nil {
			return nil, err
		}
	}
	if snapshot.presentCount == snapshot.effectCount && snapshot.operationCount == 0 {
		hashes := make([]string, 0, len(in.effects))
		for _, effect := range in.effects {
			hash, err := candidate.CandidateEffectMapProofHash(candidate.CandidateEffectMapEntry{
				EffectOrdinal: effect.Ordinal, EffectSHA256: in.effectHashes[effect.Ordinal],
				Path: effect.Path, Resolution: "absorbed-present",
				OperationOrdinals: []int{}, ReasonCodes: []string{},
			})
			if err != nil {
				return nil, err
			}
			hashes = append(hashes, hash)
		}
		sort.Strings(hashes)
		raw, err := json.Marshal(hashes)
		if err != nil {
			return nil, err
		}
		snapshot.effectDigest = CoverageSHA256(raw)
	}
	after, err := os.Stat(s.Root)
	if err != nil || !os.SameFile(snapshot.repoIdentity, after) {
		return nil, fmt.Errorf("reference-not-reconstructable")
	}
	// Nothing retained through provider execution needs patch bodies.
	snapshot.eligibility.inputs = nil
	return snapshot, nil
}

func observeOperationPhaseCounts(reference, upstream *candidate.ReferenceRoot, in *operationCandidateValidatedInputs) (int, int, int, int, string, error) {
	plan, err := candidate.Derive(reference, upstream, in.patch, in.effects)
	if err != nil {
		return 0, 0, 0, 0, "", err
	}
	present, applicable, conflicting := plan.Counts()
	return present, applicable, conflicting, plan.OperationCount(), plan.ConflictCode(), nil
}

func operationPhaseRefusalCode(err error) string {
	var coded interface{ Code() string }
	if errors.As(err, &coded) && coded.Code() != "" {
		return coded.Code()
	}
	if err != nil {
		for _, code := range []string{
			"recovery-required", "candidate-lock-held", "lock-unsupported", "lock-filesystem-unsupported",
			"candidate-feature-state-unsupported", "coverage-unusable", "coverage-owner-mismatch", "coverage-incomplete",
			"capture-event-owner-mismatch", "capture-event-pair-mismatch", "binding-presence-drift", "binding-hash-drift",
			"recipe-undecodable", "recipe-owner-mismatch", "reference-not-reconstructable", "effect-binding-drift",
			"recipe-stale-marker-present", "recipe-unsupported", "supersession-not-authority",
			"parent-blocked", "derivation-mode-unsupported",
			"candidate-already-accepted", "candidate-rejected", "candidate-id-collision",
			"candidate-integrity-drift", "candidate-pending", "shadow-pending",
			"capture-event-unusable", "effect-unexplained",
		} {
			if err.Error() == code || strings.HasPrefix(err.Error(), code+":") {
				return code
			}
		}
	}
	return "reference-not-reconstructable"
}

func operationRefusalConfidence(code string) store.ReconcileEvidenceConfidence {
	for _, eligibility := range []string{
		"candidate-lock-held", "lock-unsupported", "lock-filesystem-unsupported",
		"candidate-feature-state-unsupported", "coverage-unusable", "coverage-owner-mismatch", "coverage-incomplete",
		"capture-event-unusable", "capture-event-owner-mismatch", "capture-event-pair-mismatch",
		"binding-presence-drift", "binding-hash-drift", "recipe-undecodable", "recipe-owner-mismatch",
		"reference-not-reconstructable", "effect-binding-drift", "recipe-stale-marker-present", "recipe-unsupported",
		"supersession-not-authority", "parent-blocked", "derivation-mode-unsupported", "shadow-pending",
	} {
		if code == eligibility {
			return store.EvidenceConfidenceUnknown
		}
	}
	return store.EvidenceConfidenceLow
}

func operationPhaseEvidence(s *store.Store, slug string, result *ReconcileResult, phase store.ReconcileEvidencePhase, kind store.ReconcileEvidenceKind, verdict, reason string, confidence store.ReconcileEvidenceConfidence) store.ReconcileEvidence {
	status, _ := s.LoadFeatureStatus(slug)
	base := status.Apply.BaseCommit
	if base == "" {
		base = "unknown"
	}
	entry := store.ReconcileEvidence{
		SchemaVersion: store.ReconcileEvidenceSchemaVersion, FeatureSlug: slug,
		UpstreamRef: result.UpstreamRef, UpstreamCommit: result.UpstreamCommit,
		BaseCommit: base, Phase: phase, EvidenceKind: kind, RawReconcileVerdict: verdict,
		Confidence: confidence, ReasonCode: reason, MatchedPaths: []string{},
		MatchedOperations: []string{}, UpstreamCommitRefs: []string{},
		MatchOrigin:          store.EvidenceMatchOriginUnknown,
		PreReconcilePresence: store.EvidencePresenceNotChecked, RequiresConfirmation: true,
	}
	entry.AttemptID = store.ComputeAttemptID(entry)
	return entry
}

func qualifyingOperationPhaseEvidence(s *store.Store, slug string, result *ReconcileResult) store.ReconcileEvidence {
	entry := operationPhaseEvidence(s, slug, result, store.EvidencePhase2, store.EvidenceKindRecipeOperationMatch,
		string(store.ReconcileUpstreamed), "phase-2-operation-level", store.EvidenceConfidenceHigh)
	entry.MatchOrigin = store.EvidenceMatchOriginUpstream
	entry.PreReconcilePresence = store.EvidencePresencePresent
	entry.UpstreamCommitRefs = []string{result.UpstreamCommit}
	entry.RequiresConfirmation = false
	entry.MatchedPaths = append([]string{}, result.Conflicts...)
	entry.AttemptID = store.ComputeAttemptID(entry)
	return entry
}

func appendOperationPhaseRefusal(s *store.Store, slug string, result *ReconcileResult, reason string, confidence store.ReconcileEvidenceConfidence) {
	if reason == "" {
		reason = "effect-unexplained"
	}
	result.Notes = append(result.Notes, "Operation candidate refused: "+reason)
	entry := operationPhaseEvidence(s, slug, result, store.EvidencePhase2, store.EvidenceKindRecipeOperationMatch,
		string(store.ReconcileBlocked), reason, confidence)
	if err := store.AppendReconcileEvidence(s, slug, entry); err != nil {
		warnReconcileEvidenceAppendError(slug, err)
		return
	}
	result.Evidence = append(result.Evidence, entry)
}

func operationCandidateObservation(s *store.Store, slug string, result *ReconcileResult, derivation candidate.CandidateDerivation, confidence store.ReconcileEvidenceConfidence) (store.ReconcileEvidence, error) {
	entry := operationPhaseEvidence(s, slug, result, store.EvidencePhase2, store.EvidenceKindRecipeOperationMatch,
		string(store.ReconcileStillNeeded), "phase-2-operation-candidate", confidence)
	entry.MatchOrigin = store.EvidenceMatchOriginUpstream
	entry.PreReconcilePresence = store.EvidencePresencePresent
	entry.UpstreamCommitRefs = []string{result.UpstreamCommit}
	seen := map[string]bool{}
	for _, effect := range derivation.EffectMap {
		if !seen[effect.Path] {
			entry.MatchedPaths = append(entry.MatchedPaths, effect.Path)
			seen[effect.Path] = true
		}
	}
	sort.Strings(entry.MatchedPaths)
	for _, operation := range derivation.Operations {
		hash, err := candidate.CandidateOperationProofHash(operation)
		if err != nil {
			return store.ReconcileEvidence{}, err
		}
		entry.MatchedOperations = append(entry.MatchedOperations, hash)
	}
	entry.AttemptID = store.ComputeAttemptID(entry)
	return entry, nil
}

func evaluateReconcileOperationCandidate(s *store.Store, slug string, result *ReconcileResult, invocation operationCandidateInvocation, preflight operationCandidatePreflightResult, pending *operationPhaseSnapshot, finalReason string) (*operationPhaseSnapshot, bool, error) {
	if finalReason == "" {
		if !preflight.DerivationEnabled {
			return nil, false, nil
		}
		snapshot, err := captureOperationPhaseSnapshot(s, slug, invocation)
		if err != nil {
			appendOperationPhaseRefusal(s, slug, result, operationPhaseRefusalCode(err), store.EvidenceConfidenceUnknown)
			return nil, false, nil
		}
		if !snapshot.eligibility.enabled {
			return nil, false, nil
		}
		if !snapshot.eligibility.eligible {
			appendOperationPhaseRefusal(s, slug, result, snapshot.eligibility.reason, store.EvidenceConfidenceUnknown)
			return nil, false, nil
		}
		if snapshot.conflictCount != 0 {
			appendOperationPhaseRefusal(s, slug, result, snapshot.conflictCode, store.EvidenceConfidenceLow)
			return nil, false, nil
		}
		if snapshot.effectCount > 0 && snapshot.presentCount == snapshot.effectCount && snapshot.operationCount == 0 {
			result.Outcome = store.ReconcileUpstreamed
			result.Phase = "phase-2-operation-level"
			result.Notes = append(result.Notes, "All recipe operations already present in upstream")
			var proof *phase2AuthorityProof
			result.verdictAttempts, proof = []store.ReconcileEvidence{qualifyingOperationPhaseEvidence(s, slug, result)}, &phase2AuthorityProof{
				owner: s, result: result, repoPath: snapshot.repoPath, repoIdentity: snapshot.repoIdentity,
				upstreamCommit: snapshot.eligibility.upstreamCommit, referenceCommit: snapshot.eligibility.referenceCommit,
				captureEventSHA256: snapshot.eligibility.captureEventSHA256, coverageSHA256: snapshot.eligibility.coverageSHA256,
				patchSHA256: snapshot.eligibility.patchSHA256, recipeSHA256: snapshot.eligibility.recipeSHA256,
				sourceFeatureState: string(snapshot.eligibility.sourceFeatureState), gatesPassed: snapshot.eligibility.gatesPassed,
				effectCount: snapshot.effectCount, effectDigest: snapshot.effectDigest, presentCount: snapshot.presentCount,
				applicableCount: snapshot.applicableCount, conflictCount: snapshot.conflictCount, operationCount: snapshot.operationCount,
			}
			if err := saveReconcileArtifacts(s, slug, result, proof); err != nil {
				return nil, true, fmt.Errorf("%w: %v", ErrOperationCandidatePersistence, err)
			}
			updateFeatureState(s, slug, result)
			return nil, true, nil
		}
		if snapshot.applicableCount > 0 {
			return snapshot, false, nil
		}
		appendOperationPhaseRefusal(s, slug, result, "effect-unexplained", store.EvidenceConfidenceLow)
		return nil, false, nil
	}
	if pending == nil {
		return nil, false, nil
	}
	if finalReason != "phase-4-forward-apply-conflicts" && finalReason != "phase-4-blocked" {
		entry, err := operationCandidateObservation(s, slug, result, pending.derivation, store.EvidenceConfidenceMedium)
		if err == nil {
			err = store.AppendReconcileEvidence(s, slug, entry)
			if err == nil {
				result.Evidence = append(result.Evidence, entry)
			}
		}
		if err != nil {
			warnReconcileEvidenceAppendError(slug, err)
		}
		return nil, false, nil
	}
	lock, refusal, err := acquireOperationCandidateFeatureLock(s.Root, slug)
	if err != nil || refusal.Reason != "" {
		code := refusal.Reason
		if code == "" {
			code = operationPhaseRefusalCode(err)
		}
		appendOperationPhaseRefusal(s, slug, result, code, store.EvidenceConfidenceUnknown)
		return nil, false, nil
	}
	defer lock.Release()
	prepared, err := prepareOperationCandidateHeld(s, slug, invocation, lock)
	if err == nil {
		info, statErr := os.Stat(s.Root)
		if statErr != nil || pending.owner != s || pending.repoPath != s.Root || pending.repoIdentity == nil ||
			!os.SameFile(pending.repoIdentity, info) ||
			operationCandidateBindings(pending.eligibility) != operationCandidateBindings(prepared.eligibility) ||
			pending.eligibility.referenceCommit != prepared.eligibility.referenceCommit ||
			pending.eligibility.upstreamCommit != prepared.eligibility.upstreamCommit ||
			pending.eligibility.sourceFeatureState != prepared.eligibility.sourceFeatureState ||
			!reflect.DeepEqual(pending.derivation, prepared.derivation) {
			err = fmt.Errorf("candidate-integrity-drift")
		}
	}
	var artifact candidate.CandidateArtifact
	if err == nil {
		artifact, err = finalizeOperationCandidateHeld(s, slug, prepared, lock)
	}
	if err != nil {
		code := operationPhaseRefusalCode(err)
		if code == "recovery-required" {
			return nil, false, fmt.Errorf("%w: %v", ErrOperationCandidateRecoveryRequired, err)
		}
		appendOperationPhaseRefusal(s, slug, result, code, operationRefusalConfidence(code))
		return nil, false, nil
	}
	first, err := operationCandidateObservation(s, slug, result, artifact.Derivation, store.EvidenceConfidenceHigh)
	if err != nil {
		return nil, true, fmt.Errorf("%w: %v", ErrOperationCandidatePersistence, err)
	}
	second := operationPhaseEvidence(s, slug, result, store.EvidencePhase4, store.EvidenceKindForwardApply,
		string(store.ReconcileBlocked), finalReason, store.EvidenceConfidenceLow)
	second.MatchedPaths = append([]string{}, result.Conflicts...)
	second.AttemptID = store.ComputeAttemptID(second)
	result.verdictAttempts = append(result.verdictAttempts, first, second)
	result.Outcome = store.ReconcileStillNeeded
	result.Phase = "phase-2-operation-candidate"
	result.Notes = append(result.Notes, "Operation candidate ready: "+artifact.CandidateID)
	if err := saveReconcileArtifacts(s, slug, result, nil); err != nil {
		return nil, true, fmt.Errorf("%w: %v", ErrOperationCandidatePersistence, err)
	}
	operationCandidateBeforeSummaryWrite()
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return nil, true, fmt.Errorf("%w: load candidate summary: %v", ErrOperationCandidatePersistence, err)
	}
	status.Reconcile = mergedReconcileSummary(status.Reconcile, result)
	status.LastCommand = "reconcile"
	status.UpdatedAt = result.attemptedAt
	if err := s.SaveFeatureStatus(status); err != nil {
		return nil, true, fmt.Errorf("%w: save candidate summary: %v", ErrOperationCandidatePersistence, err)
	}
	return nil, true, nil
}

func qualifyingOperationAttempt(entry store.ReconcileEvidence, result *ReconcileResult) bool {
	return entry.Phase == store.EvidencePhase2 && entry.EvidenceKind == store.EvidenceKindRecipeOperationMatch &&
		entry.Confidence == store.EvidenceConfidenceHigh && entry.MatchOrigin == store.EvidenceMatchOriginUpstream &&
		entry.PreReconcilePresence == store.EvidencePresencePresent && !entry.RequiresConfirmation &&
		entry.RawReconcileVerdict == string(store.ReconcileUpstreamed) && entry.ReasonCode == "phase-2-operation-level" &&
		entry.UpstreamCommit == result.UpstreamCommit && slices.Equal(entry.UpstreamCommitRefs, []string{result.UpstreamCommit})
}

func validOperationStaging(entries []store.ReconcileEvidence, s *store.Store, slug string, result *ReconcileResult, hasProof bool) bool {
	if len(entries) != 1 && len(entries) != 2 {
		return false
	}
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return false
	}
	base := status.Apply.BaseCommit
	if base == "" {
		base = "unknown"
	}
	for _, entry := range entries {
		if entry.SchemaVersion != store.ReconcileEvidenceSchemaVersion || entry.FeatureSlug != slug ||
			entry.UpstreamRef == "" || entry.UpstreamRef != result.UpstreamRef || entry.UpstreamCommit == "" ||
			entry.UpstreamCommit != result.UpstreamCommit || entry.BaseCommit != base ||
			entry.MatchedPaths == nil || entry.MatchedOperations == nil || entry.UpstreamCommitRefs == nil ||
			entry.AttemptID == "" || entry.AttemptID != store.ComputeAttemptID(entry) {
			return false
		}
	}
	if len(entries) == 1 {
		return hasProof && result.Outcome == store.ReconcileUpstreamed && result.Phase == "phase-2-operation-level" &&
			qualifyingOperationAttempt(entries[0], result)
	}
	first, second := entries[0], entries[1]
	return !hasProof && result.Outcome == store.ReconcileStillNeeded && result.Phase == "phase-2-operation-candidate" &&
		first.AttemptID != second.AttemptID &&
		first.Phase == store.EvidencePhase2 && first.EvidenceKind == store.EvidenceKindRecipeOperationMatch &&
		first.RawReconcileVerdict == string(store.ReconcileStillNeeded) && first.Confidence == store.EvidenceConfidenceHigh &&
		first.MatchOrigin == store.EvidenceMatchOriginUpstream && first.PreReconcilePresence == store.EvidencePresencePresent &&
		slices.Equal(first.UpstreamCommitRefs, []string{result.UpstreamCommit}) && first.RequiresConfirmation &&
		first.ReasonCode == "phase-2-operation-candidate" && len(first.MatchedPaths) > 0 && len(first.MatchedOperations) > 0 &&
		second.Phase == store.EvidencePhase4 && second.EvidenceKind == store.EvidenceKindForwardApply &&
		second.RawReconcileVerdict == string(store.ReconcileBlocked) && second.Confidence == store.EvidenceConfidenceLow &&
		second.MatchOrigin == store.EvidenceMatchOriginUnknown && second.PreReconcilePresence == store.EvidencePresenceNotChecked &&
		second.RequiresConfirmation && len(second.UpstreamCommitRefs) == 0 && len(second.MatchedOperations) == 0 &&
		(second.ReasonCode == "phase-4-forward-apply-conflicts" || second.ReasonCode == "phase-4-blocked")
}
