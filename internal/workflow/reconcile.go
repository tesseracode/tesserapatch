package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// ReconcileResult is the outcome for a single feature.
type ReconcileResult struct {
	Slug           string                 `json:"slug"`
	Title          string                 `json:"title"`
	Outcome        store.ReconcileOutcome `json:"outcome"`
	Phase          string                 `json:"phase"`
	UpstreamRef    string                 `json:"upstream_ref"`
	UpstreamCommit string                 `json:"upstream_commit"`
	Notes          []string               `json:"notes"`
	Conflicts      []string               `json:"conflicts,omitempty"`

	// Phase-3.5 (M12 / ADR-010) extensions. Populated only when
	// RunReconcile is called with ReconcileOptions.Resolve and the
	// feature reaches phase 3.5.
	ShadowPath     string   `json:"shadow_path,omitempty"`
	ResolvedFiles  []string `json:"resolved_files,omitempty"`
	FailedFiles    []string `json:"failed_files,omitempty"`
	SkippedFiles   []string `json:"skipped_files,omitempty"`
	ResolveSession string   `json:"resolve_session_id,omitempty"`

	// Labels is the M14.3 composable-label overlay (ADR-011 D3 + D6).
	// Populated only when Config.DAGEnabled() is true and at least one
	// label applies to this child. `omitempty` is load-bearing for
	// flag-off byte-identity of reconcile-session.json against pre-M14.3
	// fixtures.
	Labels []store.ReconcileLabel `json:"labels,omitempty"`

	// PatchIDMatch is the M17 Wave D / PRD-patch-already-upstream-detector
	// audit payload for the phase-1.5 deterministic sweep. Populated only
	// when Config.PatchIDDetectorEnabled is true AND the sweep produced a
	// match. `omitempty` is load-bearing for default-OFF byte-identity
	// vs pre-M17-Wave-D fixtures.
	PatchIDMatch *store.PatchIDMatch `json:"patch_id_match,omitempty"`

	// Evidence exposes the reconcile evidence entries successfully appended
	// during this RunReconcile invocation. `omitempty` is load-bearing for
	// byte-identity when no evidence artifact was written.
	Evidence []store.ReconcileEvidence `json:"evidence,omitempty"`

	// ReviewVerdict records the confirmation gate decision for upstreamed
	// candidates. Empty means no confirmation gate ran.
	ReviewVerdict string `json:"review_verdict,omitempty"`

	// Revisions exposes revision-pass entries appended during this invocation.
	Revisions []store.ReconcileRevision `json:"revisions,omitempty"`

	// RetirementAudit exposes the cleanup audit triggered after upstreamed
	// confirmation. Runtime/display only; status.json remains lifecycle truth.
	RetirementAudit *RetirementAuditReport `json:"retirement_audit,omitempty"`

	// BlockedCategory and RecommendedAction enrich blocked verdict presentation.
	// They are runtime/display fields only; lifecycle truth remains Outcome.
	BlockedCategory   string   `json:"blocked_category,omitempty"`
	RecommendedAction string   `json:"recommended_action,omitempty"`
	BlockedEvidence   []string `json:"blocked_evidence,omitempty"`

	// attemptedAt is the timestamp shared between saveReconcileArtifacts
	// (which feeds it to composeLabelsAt as the staleness baseline) and
	// updateFeatureState (which writes it as ReconcileSummary.AttemptedAt
	// + FeatureStatus.UpdatedAt). Populated lazily by whichever runs
	// first. M14 fix-pass F2: prior to this field, labels were composed
	// against the OLD on-disk AttemptedAt, then the new AttemptedAt
	// overwrote it — leaving a child flagged stale against itself.
	//
	// Unexported and so ignored by encoding/json (no schema impact, no
	// fixture drift).
	attemptedAt string
}

// ReconcileOptions configures RunReconcile. Zero value keeps v0.4.x
// behavior (phases 1-4, no provider-assisted conflict resolution).
type ReconcileOptions struct {
	// Resolve enables phase 3.5 (ADR-010 provider-assisted per-file
	// conflict resolver). When false, 3-way conflicts short-circuit to
	// ReconcileBlocked as before.
	Resolve bool

	// Apply, when combined with Resolve, copies the resolved shadow
	// worktree onto the real tree iff every file passed validation
	// (including the optional test_command gate). When false, phase 3.5
	// leaves the shadow staged and returns ReconcileShadowAwaiting for
	// human review.
	Apply bool

	// Model, if non-empty, overrides the provider model for phase 3.5
	// calls. Useful for budget-sensitive users who reconcile with a
	// cheaper model than their implement phase.
	Model string

	// MaxConflicts caps the number of conflicted files per feature.
	// Zero uses workflow.DefaultMaxConflicts.
	MaxConflicts int

	// CumulativeLegacy opts into the pre-v0.12.1 cumulative-derivation
	// path (ADR-030 D2 / PRD-multi-slug-reconcile-canonical-safety §4.2).
	// When true, `deriveIncrementalPatches` runs on multi-slug
	// invocations, `reconcileFeature` prefers `incremental.patch` over
	// `post-apply.patch`, the ADR-011 D9 DAG-topological reorder is
	// skipped (D8 / ADR-030 D6), and phase 1.5 patch-id detection is
	// skipped with a note attached to each ReconcileResult (D9 /
	// ADR-030 D7). When false (default), each feature's canonical
	// `post-apply.patch` is authoritative and no `incremental.patch`
	// artifact is written — matching INV-1/INV-2 of the PRD.
	CumulativeLegacy bool
}

// RunReconcile reconciles features against the upstream ref.
//
// Compatibility: the zero-value ReconcileOptions reproduces the
// pre-M12 behavior, so existing callers that pass ReconcileOptions{}
// see no change. Phase 3.5 is opt-in via Options.Resolve.
func RunReconcile(ctx context.Context, s *store.Store, slugs []string, upstreamRef string, prov provider.Provider, cfg provider.Config, opts ReconcileOptions) ([]ReconcileResult, error) {
	// v0.12.0 Wave α (ADR-028 D6, PRD-feature-supersession §3.3):
	// track which of the input slugs were explicitly named by the
	// caller (so they receive a historical-feature warning if
	// superseded) versus implicitly picked up by the default
	// applied/active sweep (so they are silently filtered out).
	callerNamedSlugs := len(slugs) > 0

	// If no slugs specified, reconcile all applied/active features
	if len(slugs) == 0 {
		features, err := s.ListFeatures()
		if err != nil {
			return nil, err
		}
		for _, f := range features {
			if f.State == store.StateApplied || f.State == store.StateActive {
				slugs = append(slugs, f.Slug)
			}
		}
	}
	if len(slugs) == 0 {
		return nil, fmt.Errorf("no features to reconcile (no applied or active features found)")
	}

	// v0.12.0 Wave α (ADR-028 D6, PRD-feature-supersession §3.3,
	// AC-5): filter the default effective set so features that are
	// superseded by an active, healthy superseder are silently
	// excluded from default replay. When the caller explicitly
	// named the slugs, we preserve them but attach a historical-
	// feature warning note (§3.3: "Direct reconcile of a superseded
	// feature is allowed for audit/repair but emits a historical-
	// feature warning").
	all, allErr := s.ListFeatures()
	var supersededWarnings map[string]string
	if allErr == nil {
		filtered := make([]string, 0, len(slugs))
		for _, sl := range slugs {
			if superseder, superseded := isFeatureSupersededIn(all, sl); superseded {
				if callerNamedSlugs {
					// Explicit direct reconcile — warn but keep.
					if supersededWarnings == nil {
						supersededWarnings = map[string]string{}
					}
					supersededWarnings[sl] = superseder
					filtered = append(filtered, sl)
				}
				// Implicit default set — silently drop.
				continue
			}
			filtered = append(filtered, sl)
		}
		slugs = filtered
	}
	if len(slugs) == 0 {
		return nil, fmt.Errorf("no features to reconcile (default effective set is empty after supersession filtering)")
	}

	// M14.3 / ADR-011 D9: when the dependency-DAG flag is enabled,
	// reorder the input slug set into hard-parent topological order
	// (parents reconcile before children). When disabled, preserve the
	// pre-M14.3 input order byte-for-byte. PlanReconcile rejects cycles
	// and unknown slugs with descriptive errors.
	//
	// PRD-multi-slug-reconcile-canonical-safety §4.8 D8 / ADR-030 D6:
	// under --cumulative-legacy, the DAG-topological reorder is SKIPPED
	// so `deriveIncrementalPatches` sees the caller's exact ordering
	// (cumulative subtraction depends on `prevCumulative` being the
	// previous slug in caller order, not an inferred hard-parent).
	if !opts.CumulativeLegacy {
		if cfg, cerr := s.LoadConfig(); cerr == nil && cfg.DAGEnabled() {
			ordered, perr := PlanReconcile(s, slugs)
			if perr != nil {
				return nil, fmt.Errorf("reconcile planning failed: %w", perr)
			}
			slugs = ordered
		}
	}

	// Resolve upstream commit
	upstreamCommit, err := gitutil.ResolveRef(s.Root, upstreamRef)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve upstream ref %q: %w", upstreamRef, err)
	}

	results := make([]ReconcileResult, 0, len(slugs))

	// PRD-multi-slug-reconcile-canonical-safety §4.2 D2 / ADR-030 D1:
	// cumulative delta derivation is default-OFF. Each feature's
	// canonical `post-apply.patch` is authoritative in multi-slug
	// reconcile (INV-1/INV-2). When --cumulative-legacy is set, the
	// pre-v0.12.1 behavior is restored: derive per-slug incremental
	// patches so reconcileFeature can consume `incremental.patch`.
	if opts.CumulativeLegacy && len(slugs) > 1 {
		deriveIncrementalPatches(s, slugs, upstreamCommit)
	}

	// v0.12.0 Wave β rev-1 Slice R3 (PRD-write-file-recipe-safety
	// AC-8 + §4.2 "During reconcile", ADR-029 D6): scan the
	// effective replay set for later-touch overlaps and attach the
	// resulting warnings to each owner's ReconcileResult.Notes.
	// Warning-class per D6; reconcile does not refuse based on this
	// signal (PRD §7.2 "v1 blocks only on preimage mismatch").
	laterTouchByOwner := DetectReconcileLaterTouchWarningsByOwner(s, slugs)

	for i, slug := range slugs {
		result, err := reconcileFeature(ctx, s, slug, upstreamRef, upstreamCommit, prov, cfg, opts)
		if err != nil {
			results = append(results, ReconcileResult{
				Slug:           slug,
				Outcome:        store.ReconcileBlocked,
				Phase:          "error",
				UpstreamRef:    upstreamRef,
				UpstreamCommit: upstreamCommit,
				Notes:          []string{fmt.Sprintf("Error: %v", err)},
			})
			continue
		}
		// v0.12.0 Wave α: prepend a historical-feature warning note
		// when the caller directly reconciled a superseded target.
		if superseder, ok := supersededWarnings[slug]; ok && result != nil {
			result.Notes = append([]string{
				fmt.Sprintf("historical-feature warning: %s is superseded by %s (active superseder). Default replay excludes it; this run was requested explicitly for audit/repair (ADR-028 D6, PRD §3.3).", slug, superseder),
			}, result.Notes...)
		}
		// v0.12.0 Wave β rev-1 Slice R3: attach later-touch warnings
		// owned by this slug. Notes are prepended so they appear
		// alongside the historical-feature warning at the top of the
		// result rather than after phase-specific noise.
		if lts, ok := laterTouchByOwner[slug]; ok && result != nil && len(lts) > 0 {
			result.Notes = append(append([]string(nil), lts...), result.Notes...)
		}
		// v0.12.1 D10 migration diagnostic
		// (PRD-multi-slug-reconcile-canonical-safety §4.10 / AC-15).
		// When the default (non-legacy) path failed phase 1 on this
		// slug AND some earlier slug in this run touched a subset of
		// this slug's touched_paths, emit the hint pointing the
		// operator at --cumulative-legacy. Fail-soft: missing
		// patch-generations.json never fires the hint.
		if !opts.CumulativeLegacy && result != nil && phaseIndicatesReverseApplyFailure(result.Phase) && i > 0 {
			maybeEmitMigrationHint(s, slugs[:i], slug)
		}
		results = append(results, *result)
	}

	// Update upstream.lock
	updateUpstreamLock(s, upstreamRef, upstreamCommit)

	return results, nil
}

func reconcileFeature(ctx context.Context, s *store.Store, slug, upstreamRef, upstreamCommit string, prov provider.Provider, cfg provider.Config, opts ReconcileOptions) (*ReconcileResult, error) {
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return nil, err
	}

	// PRD-multi-slug-reconcile-canonical-safety §4.1 D1 / ADR-030 D1
	// (INV-1/INV-2): canonical `post-apply.patch` is authoritative in
	// the default multi-slug path — the incremental.patch sidecar is
	// consulted ONLY when the operator opted into --cumulative-legacy.
	// This makes single-slug and default-multi-slug reads
	// byte-identical for the same feature.
	var patch string
	if opts.CumulativeLegacy {
		// Legacy path: prefer incremental.patch (GAP 4 semantic).
		patch, err = s.ReadFeatureFile(slug, filepath.Join("artifacts", "incremental.patch"))
		if err != nil {
			patch, err = s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
			if err != nil {
				return nil, fmt.Errorf("no recorded patch for feature %q — run 'tpatch record' first", slug)
			}
		}
	} else {
		// Default path: canonical post-apply.patch is authoritative.
		patch, err = s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
		if err != nil {
			return nil, fmt.Errorf("no recorded patch for feature %q — run 'tpatch record' first", slug)
		}
	}

	result := &ReconcileResult{
		Slug:           slug,
		Title:          status.Title,
		UpstreamRef:    upstreamRef,
		UpstreamCommit: upstreamCommit,
	}

	// Phase 1: Reverse-apply check (fast, free)
	reverseOK, _ := gitutil.ReverseApplyCheck(s.Root, patch)
	if reverseOK {
		result.Outcome = store.ReconcileUpstreamed
		result.Phase = "phase-1-reverse-apply"
		result.Notes = append(result.Notes, "Patch is already present in upstream (reverse-apply succeeded)")
		saveReconcileArtifacts(s, slug, result)
		updateFeatureState(s, slug, result)
		return result, nil
	}

	// Phase 1.5 (M17 Wave D / PRD-patch-already-upstream-detector):
	// deterministic `git patch-id --stable` sweep against the upstream-
	// since-lock range. Default-OFF — flag-gated on
	// Config.PatchIDDetectorEnabled so pre-M17 reconcile behaviour is
	// byte-identical when the operator has not opted in.
	//
	// PRD-multi-slug-reconcile-canonical-safety §4.9 D9 / ADR-030 D7:
	// under --cumulative-legacy, phase 1.5 is skipped for the run and
	// a note is attached to each ReconcileResult. The detector's
	// canonical-load carve-out (rev-1 M17 Wave D) exists precisely
	// because the derived incremental form produces false-positive
	// retirements; under the legacy flag the whole pipeline is
	// intentionally running on the derived form and mixing semantics
	// is worse than skipping.
	switch {
	case opts.CumulativeLegacy:
		result.Notes = append(result.Notes, "phase 1.5 skipped: --cumulative-legacy")
	default:
		if storeCfg, cerr := s.LoadConfig(); cerr == nil && storeCfg.PatchIDDetectorEnabled {
			// The detector reads canonical `post-apply.patch` directly
			// (PRD-patch-already-upstream-detector §5.1). In the
			// default v0.12.1+ path, `patch` above is already canonical
			// (INV-1), so this second read is a defense-in-depth
			// invariant that survives future refactors.
			canonical, canonErr := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
			switch {
			case canonErr != nil || strings.TrimSpace(canonical) == "":
				result.Notes = append(result.Notes, "phase 1.5 skipped: no canonical post-apply.patch artifact")
			default:
				det := runPatchIDDetector(s, canonical, upstreamCommit, storeCfg.PatchIDScanLimit)
				switch {
				case det.Match != nil:
					result.Outcome = store.ReconcileUpstreamed
					result.Phase = "phase-1.5-patch-id-match"
					// PRD §3.1 step 6: UpstreamCommit becomes the matching SHA
					// (the commit that absorbed the patch), not the upstream tip.
					result.UpstreamCommit = det.Match.MatchedUpstreamSHA
					result.PatchIDMatch = det.Match
					note := fmt.Sprintf("Patch-id sweep matched upstream commit %s (scanned %d in %s)",
						truncateCommit(det.Match.MatchedUpstreamSHA), det.Match.ScannedCount, det.Match.ScannedRange)
					if len(det.Match.AdditionalMatches) > 0 {
						note += fmt.Sprintf("; %d additional match(es)", len(det.Match.AdditionalMatches))
					}
					result.Notes = append(result.Notes, note)
					saveReconcileArtifacts(s, slug, result)
					updateFeatureState(s, slug, result)
					return result, nil
				case det.Skipped:
					// Fail-soft (PRD §5.1): never treat tooling failure as a
					// no-match verdict. Surface a single audit note and fall
					// through to phase 2.
					result.Notes = append(result.Notes, "phase 1.5 skipped: "+det.SkipReason)
				}
			}
		}
	}

	// Phase 2: Operation-level evaluation (if apply-recipe.json exists)
	recipeData, recipeErr := s.ReadFeatureFile(slug, filepath.Join("artifacts", "apply-recipe.json"))
	if recipeErr == nil && recipeData != "" {
		var recipe ApplyRecipe
		if err := json.Unmarshal([]byte(recipeData), &recipe); err == nil && len(recipe.Operations) > 0 {
			opResult := evaluateRecipeOperations(s.Root, recipe.Operations)
			if opResult.allPresent {
				result.Outcome = store.ReconcileUpstreamed
				result.Phase = "phase-2-operation-level"
				result.Notes = append(result.Notes, "All recipe operations already present in upstream")
				saveReconcileArtifacts(s, slug, result)
				updateFeatureState(s, slug, result)
				return result, nil
			}
			if opResult.hasConflicts {
				result.Notes = append(result.Notes, fmt.Sprintf("Operation-level: %d present, %d applicable, %d conflicts",
					opResult.presentCount, opResult.applicableCount, opResult.conflictCount))
			}
		}
	}

	// Phase 3: Provider-assisted semantic check (if provider available)
	if prov != nil && cfg.Configured() {
		request, _ := s.ReadFeatureFile(slug, "request.md")
		spec, _ := s.ReadFeatureFile(slug, "spec.md")

		// Extract affected files from patch and read their current upstream content
		upstreamContext := extractUpstreamContext(s.Root, patch)

		semanticResult, err := providerSemanticCheck(ctx, prov, cfg, request, spec, patch, upstreamRef, upstreamContext)
		if err == nil {
			if semanticResult == "upstreamed" {
				result.Outcome = store.ReconcileUpstreamed
				result.Phase = "phase-3-provider-semantic"
				result.Notes = append(result.Notes, "Provider determined upstream satisfies acceptance criteria")
				saveReconcileArtifacts(s, slug, result)
				updateFeatureState(s, slug, result)
				return result, nil
			}
			result.Notes = append(result.Notes, fmt.Sprintf("Provider semantic check: %s", semanticResult))
		} else {
			result.Notes = append(result.Notes, fmt.Sprintf("Provider semantic check error: %v", err))
		}
	}

	// Phase 4: Forward-apply preview (safety net).
	// Uses PreviewForwardApply which runs the 3-way merge in an
	// isolated worktree when a strict apply fails. This replaces the
	// older ForwardApplyCheck which wrongly reported "reapplied" when
	// `git apply --3way --check` merely accepted the merge *attempt*
	// even though the apply would leave conflict markers.
	preview, _ := gitutil.PreviewForwardApply(s.Root, patch)

	// Belt-and-braces: even though PreviewForwardApply runs in an
	// isolated worktree, a prior reconcile run (or an outside workflow)
	// may have left conflict markers in the live tree. A `reapplied`
	// verdict in the presence of live markers is the worst-case user
	// experience — they commit bad code trusting the verdict.
	// See bug-reconcile-reapplied-with-conflict-markers (t3code case
	// study, v0.4.4). If markers exist, promote to Blocked.
	promoteIfMarkers := func(res *ReconcileResult) bool {
		markers := gitutil.ScanConflictMarkers(s.Root)
		if len(markers) == 0 {
			return false
		}
		res.Outcome = store.ReconcileBlocked
		res.Phase = "phase-4-live-conflict-markers"
		res.Notes = append(res.Notes,
			fmt.Sprintf("Refused to report reapplied: %d file(s) in the working tree contain unresolved conflict markers", len(markers)))
		res.Conflicts = append(res.Conflicts, markers...)
		return true
	}

	switch preview.Verdict {
	case gitutil.ForwardApplyStrict:
		result.Outcome = store.ReconcileReapplied
		result.Phase = "phase-4-forward-apply-strict"
		result.Notes = append(result.Notes, "Patch applies cleanly (strict) — safe to auto-apply")
		promoteIfMarkers(result)
		saveReconcileArtifacts(s, slug, result)
		updateFeatureState(s, slug, result)
		return result, nil
	case gitutil.ForwardApply3WayClean:
		result.Outcome = store.ReconcileReapplied
		result.Phase = "phase-4-forward-apply-3way"
		note := "Patch applies via 3-way merge (no conflict markers in preview)"
		if preview.Stderr != "" {
			note = fmt.Sprintf("%s [git: %s]", note, preview.Stderr)
		}
		result.Notes = append(result.Notes, note)
		promoteIfMarkers(result)
		saveReconcileArtifacts(s, slug, result)
		updateFeatureState(s, slug, result)
		return result, nil
	case gitutil.ForwardApply3WayConflicts:
		// Phase 3.5 (M12 / ADR-010): try provider-assisted per-file
		// conflict resolution if the operator asked for it via
		// --resolve. Otherwise, preserve the v0.4.4 behavior:
		// report as BLOCKED so humans are warned.
		if opts.Resolve {
			// M14.3 / ADR-011 D6: when a hard parent is blocked, the
			// resolver cannot meaningfully fix this child — running it
			// would burn provider budget against a broken baseline.
			// Short-circuit BEFORE invoking the resolver. The compound
			// presentation "blocked-by-parent-and-needs-resolution" is
			// computed at read time by ReconcileSummary.EffectiveOutcome.
			if cfg, cerr := s.LoadConfig(); cerr == nil && cfg.DAGEnabled() {
				labels, _ := ComposeLabels(s, slug)
				// Slice B (ADR-013 D4): freshness labels are read-time
				// only; never persisted to status.json. The
				// blocked-by-parent check below operates on the unfiltered
				// set; we strip before persisting via result.Labels.
				if hasLabel(labels, store.LabelBlockedByParent) {
					result.Outcome = store.ReconcileBlockedRequiresHuman
					result.Phase = "phase-3.5-skipped-blocked-by-parent"
					result.Labels = stripDerivedLabels(labels)
					result.Conflicts = append(result.Conflicts, preview.ConflictFiles...)
					result.Notes = append(result.Notes,
						"phase 3.5 skipped: a hard parent is blocked — resolve the parent first, then retry `tpatch reconcile "+slug+"` (compound verdict: blocked-by-parent-and-needs-resolution)")
					saveReconcileArtifacts(s, slug, result)
					updateFeatureState(s, slug, result)
					return result, nil
				}
			}
			phase35 := tryPhase35(ctx, s, slug, upstreamCommit, prov, cfg, opts, preview.ConflictFiles, result)
			saveReconcileArtifacts(s, slug, phase35)
			updateFeatureState(s, slug, phase35)
			return phase35, nil
		}
		result.Outcome = store.ReconcileBlocked
		result.Phase = "phase-4-forward-apply-conflicts"
		result.Notes = append(result.Notes,
			fmt.Sprintf("3-way merge would leave conflict markers in %d file(s) — manual resolution required (re-run with --resolve to attempt provider-assisted resolution)",
				len(preview.ConflictFiles)))
		result.Conflicts = append(result.Conflicts, preview.ConflictFiles...)
		if preview.Stderr != "" {
			result.Notes = append(result.Notes, fmt.Sprintf("git: %s", preview.Stderr))
		}
		saveReconcileArtifacts(s, slug, result)
		updateFeatureState(s, slug, result)
		return result, nil
	}

	// All phases exhausted — blocked
	result.Outcome = store.ReconcileBlocked
	result.Phase = "phase-4-blocked"
	result.Notes = append(result.Notes, "Patch cannot be applied cleanly — manual intervention needed")
	if preview.Stderr != "" {
		result.Notes = append(result.Notes, fmt.Sprintf("git: %s", preview.Stderr))
	}
	result.Conflicts = append(result.Conflicts, "Forward-apply failed — check for merge conflicts")
	saveReconcileArtifacts(s, slug, result)
	updateFeatureState(s, slug, result)
	return result, nil
}

// Operation-level evaluation

type operationEvalResult struct {
	allPresent      bool
	hasConflicts    bool
	presentCount    int
	applicableCount int
	conflictCount   int
}

func evaluateRecipeOperations(repoRoot string, ops []RecipeOperation) operationEvalResult {
	result := operationEvalResult{}

	for _, op := range ops {
		switch op.Type {
		case "replace-in-file":
			filePath := filepath.Join(repoRoot, op.Path)
			content, err := os.ReadFile(filePath)
			if err != nil {
				result.conflictCount++
				result.hasConflicts = true
				continue
			}
			contentStr := string(content)
			if strings.Contains(contentStr, op.Replace) {
				result.presentCount++
			} else if strings.Contains(contentStr, op.Search) {
				result.applicableCount++
			} else {
				result.conflictCount++
				result.hasConflicts = true
			}

		case "write-file":
			filePath := filepath.Join(repoRoot, op.Path)
			content, err := os.ReadFile(filePath)
			if err == nil && strings.TrimSpace(string(content)) == strings.TrimSpace(op.Content) {
				result.presentCount++
			} else if err != nil {
				result.applicableCount++ // file doesn't exist, can be created
			} else {
				result.conflictCount++
				result.hasConflicts = true
			}

		case "ensure-directory":
			result.presentCount++ // directories are always fine

		default:
			result.applicableCount++
		}
	}

	result.allPresent = result.presentCount > 0 && result.conflictCount == 0 && result.applicableCount == 0
	return result
}

// Provider-assisted semantic check
func providerSemanticCheck(ctx context.Context, prov provider.Provider, cfg provider.Config, request, spec, patch, upstreamRef, upstreamContext string) (string, error) {
	systemPrompt := `You are evaluating whether an upstream update has incorporated a local feature's changes.

Compare the feature's acceptance criteria against the current upstream code.
The "Current Upstream Code" section shows what the relevant files look like NOW in upstream.
If the upstream now satisfies the feature's requirements (even if implemented differently), respond with: {"decision": "upstreamed", "reasoning": "..."}
If the feature is still needed (upstream does NOT have equivalent functionality), respond with: {"decision": "still_needed", "reasoning": "..."}
If you cannot determine, respond with: {"decision": "unclear", "reasoning": "..."}

Output ONLY valid JSON.`

	userPrompt := fmt.Sprintf(`# Feature Request
%s

# Feature Specification
%s

# Recorded Patch (our local changes)
%s

# Upstream Ref: %s

# Current Upstream Code (relevant files as they exist now)
%s

Does the upstream now satisfy this feature's requirements? Compare the acceptance criteria against the current upstream code shown above.`, request, spec, patch, upstreamRef, upstreamContext)

	response, err := prov.Generate(ctx, cfg, provider.GenerateRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    1024,
		Temperature:  0.1,
	})
	if err != nil {
		return "", err
	}

	cleaned, _ := ExtractJSONObject(response)

	var decision struct {
		Decision  string `json:"decision"`
		Reasoning string `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(cleaned), &decision); err != nil {
		return "unclear", nil
	}

	return decision.Decision, nil
}

// saveReconcileArtifacts persists the high-level ReconcileResult for one
// RunReconcile invocation to artifacts/reconcile-session.json + reconcile.md.
//
// Contract (locked in Tranche C3 / v0.5.3):
//   - reconcile-session.json is reconcile-owned and describes a RunReconcile
//     invocation (verdict, phase, upstream ref, notes, cost). It is an audit
//     record of the invocation, not a live mirror of post-accept state.
//   - Manual accept (workflow.AcceptShadow) intentionally does NOT rewrite
//     this artifact. status.json is the source of current truth post-accept
//     (e.g., status.Reconcile.Outcome flips to ReconcileReapplied while the
//     session artifact may still describe a shadow-awaiting outcome from the
//     prior RunReconcile call). Re-running reconcile overwrites it.
//   - Per-file resolver outcomes live in artifacts/resolution-session.json
//     (resolver-owned) — see resolver.persistSession. Splitting the two
//     artifacts is what fixes the v0.5.2 dual-writer collision.
func saveReconcileArtifacts(s *store.Store, slug string, result *ReconcileResult) {
	// M14 fix-pass F2: generate the AttemptedAt timestamp once and
	// share it with updateFeatureState. ComposeLabels uses this same
	// value as the staleness baseline so that, when persisted, the
	// Labels field reflects the AttemptedAt about to be written —
	// not the previous run's value (which would leave the child
	// flagged stale against itself).
	if result != nil && result.attemptedAt == "" {
		result.attemptedAt = time.Now().UTC().Format(time.RFC3339)
	}
	// M14.3: enrich the result with composable labels before serializing
	// so reconcile-session.json captures the DAG context. When the flag
	// is off, composeLabelsAt returns nil and `omitempty` keeps the field
	// out of JSON (byte-identity vs pre-M14.3 fixtures).
	//
	// Skip if the caller already set labels (e.g. the phase-3.5 short-
	// circuit path explicitly attaches its own label set).
	//
	// C5 F1: if the in-memory outcome marks the child as retired
	// (currently only ReconcileUpstreamed), suppress label composition
	// entirely. composeLabelsAt re-loads the child status FROM DISK,
	// where the OLD outcome still lives — so without this guard, parent
	// labels would be derived from the pre-reconcile baseline and
	// persisted alongside a freshly-upstreamed verdict. ADR-011: parent
	// state is irrelevant once a child is absorbed upstream. Force
	// Labels to nil so updateFeatureState propagates the same.
	if result != nil {
		if _, retired := childRetiredOutcomes[result.Outcome]; retired {
			result.Labels = nil
		} else if len(result.Labels) == 0 {
			if labels, lerr := composeLabelsAt(s, slug, result.attemptedAt); lerr == nil {
				// Slice B (ADR-013 D4) + Wave α (ADR-028 D4): strip
				// freshness AND supersession labels before persisting.
				// composeLabelsAt returns the union of M14.3 + freshness
				// + supersession; only M14.3 is persisted.
				if persisted := stripDerivedLabels(labels); len(persisted) > 0 {
					result.Labels = persisted
				}
			}
		}
	}

	result.Evidence = append(result.Evidence, persistReconcileEvidence(s, slug, result)...)
	result.Evidence = append(result.Evidence, persistFileNoveltyEvidence(s, slug, result)...)
	result.Evidence = append(result.Evidence, persistHunkOverlapEvidence(s, slug, result)...)
	result.Evidence = append(result.Evidence, persistPathRestructureEvidence(s, slug, result)...)
	result.Evidence = append(result.Evidence, persistBlockedClassificationEvidence(s, slug, result)...)
	result.Evidence = append(result.Evidence, applyUpstreamedConfirmationGate(s, slug, result)...)
	result.Revisions = append(result.Revisions, persistRevisionPassLog(s, slug, result)...)

	// Save reconcile-session.json
	data, _ := json.MarshalIndent(result, "", "  ")
	s.WriteArtifact(slug, "reconcile-session.json", string(data)+"\n")

	// Save reconcile.md
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Reconciliation: %s\n\n", slug))
	b.WriteString(fmt.Sprintf("**Outcome**: %s\n", result.Outcome))
	b.WriteString(fmt.Sprintf("**Phase**: %s\n", result.Phase))
	b.WriteString(fmt.Sprintf("**Upstream Ref**: %s\n", result.UpstreamRef))
	b.WriteString(fmt.Sprintf("**Upstream Commit**: %s\n", result.UpstreamCommit))
	b.WriteString(fmt.Sprintf("**Timestamp**: %s\n\n", time.Now().UTC().Format(time.RFC3339)))

	if len(result.Notes) > 0 {
		b.WriteString("## Notes\n\n")
		for _, note := range result.Notes {
			b.WriteString("- " + note + "\n")
		}
		b.WriteString("\n")
	}
	if len(result.Conflicts) > 0 {
		b.WriteString("## Conflicts\n\n")
		for _, c := range result.Conflicts {
			b.WriteString("- " + c + "\n")
		}
		b.WriteString("\n")
	}
	s.WriteArtifact(slug, "reconcile.md", b.String())

	// Save per-version log
	commitRange := fmt.Sprintf("%s-to-%s", truncateCommit(result.UpstreamCommit), "HEAD")
	s.WriteFeatureFile(slug, filepath.Join("reconciliation", commitRange+".md"), b.String())
}

var warnReconcileEvidence = func(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
}

func persistReconcileEvidence(s *store.Store, slug string, result *ReconcileResult) []store.ReconcileEvidence {
	if result == nil || result.Outcome == "" {
		return nil
	}
	phase, kind := evidencePhaseAndKind(result)
	if phase == "" {
		return nil
	}
	status, _ := s.LoadFeatureStatus(slug)
	baseCommit := status.Apply.BaseCommit
	if baseCommit == "" {
		baseCommit = "unknown"
	}
	confidence := store.EvidenceConfidenceMedium
	matchOrigin := store.EvidenceMatchOriginUnknown
	presence := store.EvidencePresenceNotChecked
	requiresConfirmation := true
	upstreamCommitRefs := []string{}
	switch kind {
	case store.EvidenceKindPatchIDMatch:
		confidence = store.EvidenceConfidenceHigh
		matchOrigin = store.EvidenceMatchOriginUpstream
		presence = store.EvidencePresencePresent
		requiresConfirmation = false
		if result.PatchIDMatch != nil && result.PatchIDMatch.MatchedUpstreamSHA != "" {
			upstreamCommitRefs = []string{result.PatchIDMatch.MatchedUpstreamSHA}
		}
	case store.EvidenceKindReverseApply:
		confidence = store.EvidenceConfidenceHigh
		matchOrigin = store.EvidenceMatchOriginUpstream
		presence = store.EvidencePresencePresent
	case store.EvidenceKindRecipeOperationMatch:
		confidence = store.EvidenceConfidenceLow
		presence = store.EvidencePresencePresent
	case store.EvidenceKindForwardApply:
		if result.Outcome == store.ReconcileBlocked {
			confidence = store.EvidenceConfidenceLow
		}
	}
	entry := store.ReconcileEvidence{
		SchemaVersion:        store.ReconcileEvidenceSchemaVersion,
		FeatureSlug:          slug,
		UpstreamRef:          result.UpstreamRef,
		UpstreamCommit:       result.UpstreamCommit,
		BaseCommit:           baseCommit,
		RawReconcileVerdict:  string(result.Outcome),
		Phase:                phase,
		EvidenceKind:         kind,
		Confidence:           confidence,
		MatchedPaths:         append([]string(nil), result.Conflicts...),
		MatchedOperations:    []string{},
		MatchOrigin:          matchOrigin,
		UpstreamCommitRefs:   upstreamCommitRefs,
		PreReconcilePresence: presence,
		RequiresConfirmation: requiresConfirmation,
		ReasonCode:           result.Phase,
	}
	if entry.MatchedPaths == nil {
		entry.MatchedPaths = []string{}
	}
	if result.PatchIDMatch != nil && kind == store.EvidenceKindPatchIDMatch {
		entry = store.PatchIDMatchEvidenceFields(entry, *result.PatchIDMatch)
	}
	entry.AttemptID = store.ComputeAttemptID(entry)
	if err := store.AppendReconcileEvidence(s, slug, entry); err != nil {
		warnReconcileEvidenceAppendError(slug, err)
		return nil
	}
	return []store.ReconcileEvidence{entry}
}

func persistFileNoveltyEvidence(s *store.Store, slug string, result *ReconcileResult) []store.ReconcileEvidence {
	if result == nil || result.Outcome == "" || result.UpstreamCommit == "" {
		return nil
	}
	status, err := s.LoadFeatureStatus(slug)
	if err != nil || status.Apply.BaseCommit == "" {
		// File novelty is diagnostic evidence only; skip silently until the
		// canonical post-apply patch and commit anchors are all available.
		return nil
	}
	patch, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
	if err != nil || strings.TrimSpace(patch) == "" {
		// File novelty is diagnostic evidence only; skip silently when the
		// canonical post-apply patch is unavailable.
		return nil
	}
	novelty, err := ClassifyFileNovelty(patch, result.UpstreamCommit, status.Apply.BaseCommit, s.Root)
	if err != nil {
		return nil
	}
	entry := FileNoveltyEvidence(slug, result.UpstreamRef, result.UpstreamCommit, status.Apply.BaseCommit, string(result.Outcome), novelty)
	entry.AttemptID = store.ComputeAttemptID(entry)
	if err := store.AppendReconcileEvidence(s, slug, entry); err != nil {
		warnReconcileEvidenceAppendError(slug, err)
		return nil
	}
	return []store.ReconcileEvidence{entry}
}

func persistHunkOverlapEvidence(s *store.Store, slug string, result *ReconcileResult) []store.ReconcileEvidence {
	if result == nil || result.Outcome == "" || result.UpstreamCommit == "" {
		return nil
	}
	status, err := s.LoadFeatureStatus(slug)
	if err != nil || status.Apply.BaseCommit == "" {
		return nil
	}
	patch, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
	if err != nil || strings.TrimSpace(patch) == "" {
		return nil
	}
	novelty, err := ClassifyFileNovelty(patch, result.UpstreamCommit, status.Apply.BaseCommit, s.Root)
	if err != nil {
		return nil
	}
	if novelty.Classification != FileNoveltyModifiesExistingFiles && novelty.Classification != FileNoveltyMixedAdditive {
		return nil
	}
	overlap, err := DetectHunkOverlap(s.Root, patch, status.Apply.BaseCommit, result.UpstreamCommit, novelty)
	if err != nil || len(overlap.Hunks) == 0 || overlap.Classification == HunkOverlapNone {
		return nil
	}
	entry := HunkOverlapEvidence(slug, result.UpstreamRef, result.UpstreamCommit, status.Apply.BaseCommit, string(result.Outcome), overlap)
	if err := store.AppendReconcileEvidence(s, slug, entry); err != nil {
		warnReconcileEvidenceAppendError(slug, err)
		return nil
	}
	return []store.ReconcileEvidence{entry}
}

func persistPathRestructureEvidence(s *store.Store, slug string, result *ReconcileResult) []store.ReconcileEvidence {
	if result == nil || result.UpstreamCommit == "" {
		return nil
	}
	if result.Outcome != store.ReconcileBlocked && result.Outcome != store.ReconcileBlockedRequiresHuman && result.Outcome != store.ReconcileBlockedTooManyConflicts {
		return nil
	}
	status, err := s.LoadFeatureStatus(slug)
	if err != nil || status.Apply.BaseCommit == "" {
		return nil
	}
	patch, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
	if err != nil || strings.TrimSpace(patch) == "" {
		return nil
	}
	featurePaths := featurePathsFromPatch(patch)
	if len(featurePaths) == 0 {
		return nil
	}
	storeCfg, _ := s.LoadConfig()
	detected, err := DetectPathRestructure(PathRestructureInput{
		RepoRoot:     s.Root,
		BaseCommit:   status.Apply.BaseCommit,
		TargetCommit: result.UpstreamCommit,
		FeaturePaths: featurePaths,
		Thresholds:   PathRestructureThresholdsFromConfig(storeCfg),
	})
	if err != nil || detected == nil {
		return nil
	}
	switch detected.Classification {
	case PathRestructurePrefixMove, PathRestructurePrefixSplit, PathRestructureTargetDeleted, PathRestructureMixed:
	default:
		return nil
	}
	entry := PathRestructureReconcileEvidence(slug, result.UpstreamRef, result.UpstreamCommit, status.Apply.BaseCommit, string(result.Outcome), *detected)
	if err := store.AppendReconcileEvidence(s, slug, entry); err != nil {
		warnReconcileEvidenceAppendError(slug, err)
		return nil
	}
	return []store.ReconcileEvidence{entry}
}

func featurePathsFromPatch(patch string) []string {
	paths := parsePatchNoveltyPaths(patch)
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p.Path != "" {
			out = append(out, p.Path)
		}
	}
	return out
}

func persistBlockedClassificationEvidence(s *store.Store, slug string, result *ReconcileResult) []store.ReconcileEvidence {
	if result == nil || (result.Outcome != store.ReconcileBlocked && result.Outcome != store.ReconcileBlockedRequiresHuman && result.Outcome != store.ReconcileBlockedTooManyConflicts) {
		return nil
	}
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return nil
	}
	baseCommit := status.Apply.BaseCommit
	if baseCommit == "" {
		baseCommit = "unknown"
	}
	cls := ClassifyBlockedVerdict(BlockedClassificationInput{
		Outcome:      result.Outcome,
		Phase:        result.Phase,
		Labels:       result.Labels,
		Evidence:     result.Evidence,
		FailedFiles:  result.FailedFiles,
		SkippedFiles: result.SkippedFiles,
		Notes:        result.Notes,
	})
	if cls.Category == "" {
		return nil
	}
	result.BlockedCategory = string(cls.Category)
	result.RecommendedAction = cls.RecommendedAction
	result.BlockedEvidence = append([]string(nil), cls.Evidence...)
	entry := blockedClassificationEvidence(slug, result.UpstreamRef, result.UpstreamCommit, baseCommit, cls)
	if err := store.AppendReconcileEvidence(s, slug, entry); err != nil {
		warnReconcileEvidenceAppendError(slug, err)
		return nil
	}
	return []store.ReconcileEvidence{entry}
}

func applyUpstreamedConfirmationGate(s *store.Store, slug string, result *ReconcileResult) []store.ReconcileEvidence {
	if result == nil || result.Outcome != store.ReconcileUpstreamed {
		return nil
	}
	status, _ := s.LoadFeatureStatus(slug)
	baseCommit := status.Apply.BaseCommit
	if baseCommit == "" {
		baseCommit = "unknown"
	}
	confirmed := false
	reason := "missing-upstream-commit-ref"
	confidence := store.EvidenceConfidenceLow
	matchOrigin := store.EvidenceMatchOriginUnknown
	refs := []string{}
	for _, entry := range result.Evidence {
		if entry.EvidenceKind == store.EvidenceKindPatchIDMatch && entry.MatchedUpstreamSHA != "" {
			confirmed = true
			reason = "confirmed-upstreamed"
			confidence = store.EvidenceConfidenceHigh
			matchOrigin = store.EvidenceMatchOriginUpstream
			refs = []string{entry.MatchedUpstreamSHA}
			break
		}
		if entry.EvidenceKind == store.EvidenceKindReverseApply && entry.Confidence == store.EvidenceConfidenceHigh {
			confirmed = true
			reason = "confirmed-upstreamed"
			confidence = store.EvidenceConfidenceHigh
			matchOrigin = store.EvidenceMatchOriginUpstream
		}
	}
	entry := store.ReconcileEvidence{
		SchemaVersion:        store.ReconcileEvidenceSchemaVersion,
		FeatureSlug:          slug,
		UpstreamRef:          result.UpstreamRef,
		UpstreamCommit:       result.UpstreamCommit,
		BaseCommit:           baseCommit,
		RawReconcileVerdict:  string(store.ReconcileUpstreamed),
		Phase:                store.EvidencePhase35,
		EvidenceKind:         store.EvidenceKindManualReview,
		Confidence:           confidence,
		MatchedPaths:         []string{},
		MatchedOperations:    []string{"confirmation-gate"},
		MatchOrigin:          matchOrigin,
		UpstreamCommitRefs:   refs,
		PreReconcilePresence: store.EvidencePresenceNotChecked,
		RequiresConfirmation: !confirmed,
		ReasonCode:           reason,
	}
	entry.AttemptID = store.ComputeAttemptID(entry)
	if err := store.AppendReconcileEvidence(s, slug, entry); err != nil {
		warnReconcileEvidenceAppendError(slug, err)
		return nil
	}
	if confirmed {
		result.ReviewVerdict = "confirmed-upstreamed"
		result.Notes = append(result.Notes, "confirmation gate: upstreamed verdict confirmed")
	} else {
		result.ReviewVerdict = "rejected-upstreamed"
		result.Outcome = store.ReconcileBlocked
		result.Notes = append(result.Notes, "confirmation gate: upstreamed candidate blocked pending confirmation (missing upstream commit evidence)")
	}
	return []store.ReconcileEvidence{entry}
}

func persistRevisionPassLog(s *store.Store, slug string, result *ReconcileResult) []store.ReconcileRevision {
	if result == nil || result.Outcome == "" || result.ReviewVerdict == "" {
		return nil
	}
	status, _ := s.LoadFeatureStatus(slug)
	finalState := store.StateBlocked
	review := store.ReviewVerdictFalsePositive
	action := store.ReconcileActionNone
	reason := "missing-upstream-commit-ref"
	if result.ReviewVerdict == "confirmed-upstreamed" {
		finalState = store.StateUpstreamMerged
		review = store.ReviewVerdictConfirmed
		action = store.ReconcileActionConfirmedRetired
		reason = "confirmed-upstreamed"
	} else if status.State != "" {
		finalState = store.StateBlocked
	}
	evidenceAttempt := ""
	for i := len(result.Evidence) - 1; i >= 0; i-- {
		if result.Evidence[i].EvidenceKind == store.EvidenceKindManualReview && containsString(result.Evidence[i].MatchedOperations, "confirmation-gate") {
			evidenceAttempt = result.Evidence[i].AttemptID
			reason = result.Evidence[i].ReasonCode
			break
		}
	}
	validationRefs := []store.ValidationRef{}
	if result.UpstreamCommit != "" && result.UpstreamCommit != "unknown" {
		validationRefs = append(validationRefs, store.ValidationRef{Kind: "upstream-commit", Value: result.UpstreamCommit, Result: "referenced"})
	}
	entry := store.ReconcileRevision{
		SchemaVersion:       store.ReconcileRevisionSchemaVersion,
		FeatureSlug:         slug,
		EvidenceAttemptID:   evidenceAttempt,
		RawReconcileVerdict: string(store.ReconcileUpstreamed),
		ReviewVerdict:       review,
		FinalFeatureState:   finalState,
		ActionTaken:         action,
		ReasonCode:          reason,
		ValidationRefs:      validationRefs,
	}
	entry.EntryID = store.ComputeRevisionID(entry)
	if err := store.AppendReconcileRevision(s, slug, entry); err != nil {
		warnReconcileEvidenceAppendError(slug, err)
		return nil
	}
	return []store.ReconcileRevision{entry}
}

func warnReconcileEvidenceAppendError(slug string, err error) {
	if errors.Is(err, store.ErrMalformedEvidence) || errors.Is(err, store.ErrMalformedRevision) {
		warnReconcileEvidence("warning: tpatch reconcile evidence artifact malformed for feature %q: %v\n", slug, err)
		return
	}
	warnReconcileEvidence("warning: tpatch reconcile could not write evidence for feature %q: %v\n", slug, err)
}

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func evidencePhaseAndKind(result *ReconcileResult) (store.ReconcileEvidencePhase, store.ReconcileEvidenceKind) {
	switch {
	case strings.HasPrefix(result.Phase, "phase-1.5"):
		return store.EvidencePhase15, store.EvidenceKindPatchIDMatch
	case strings.HasPrefix(result.Phase, "phase-1"):
		return store.EvidencePhase1, store.EvidenceKindReverseApply
	case strings.HasPrefix(result.Phase, "phase-2"):
		return store.EvidencePhase2, store.EvidenceKindRecipeOperationMatch
	case strings.HasPrefix(result.Phase, "phase-3.5"):
		return store.EvidencePhase35, store.EvidenceKindProviderSemantic
	case strings.HasPrefix(result.Phase, "phase-3"):
		return store.EvidencePhase3, store.EvidenceKindProviderSemantic
	case strings.HasPrefix(result.Phase, "phase-4"):
		return store.EvidencePhase4, store.EvidenceKindForwardApply
	default:
		return "", store.EvidenceKindUnknown
	}
}

// Update feature state based on reconciliation outcome
func updateFeatureState(s *store.Store, slug string, result *ReconcileResult) {
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return
	}

	if result != nil && result.attemptedAt == "" {
		result.attemptedAt = time.Now().UTC().Format(time.RFC3339)
	}

	status.Reconcile = store.ReconcileSummary{
		AttemptedAt:    result.attemptedAt,
		UpstreamRef:    result.UpstreamRef,
		UpstreamCommit: result.UpstreamCommit,
		Outcome:        result.Outcome,
		ReviewVerdict:  result.ReviewVerdict,
		ShadowPath:     result.ShadowPath,
		ResolveSession: result.ResolveSession,
		ResolvedFiles:  len(result.ResolvedFiles),
		FailedFiles:    len(result.FailedFiles),
		SkippedFiles:   len(result.SkippedFiles),
		// M14.3: persist the DAG-derived labels alongside the intrinsic
		// outcome. `omitempty` guarantees flag-off byte-identity.
		Labels: result.Labels,
		// M17 Wave D: persist the patch-id sweep audit payload alongside
		// the verdict. `omitempty` on the field is load-bearing for
		// default-OFF byte-identity.
		PatchIDMatch: result.PatchIDMatch,
	}
	status.LastCommand = "reconcile"
	status.UpdatedAt = result.attemptedAt

	switch result.Outcome {
	case store.ReconcileUpstreamed:
		status.State = store.StateUpstreamMerged
		status.Notes = "Feature adopted by upstream — local patch retired"
	case store.ReconcileReapplied:
		status.State = store.StateApplied
		status.Notes = "Patch re-applied cleanly to new upstream"
	case store.ReconcileStillNeeded:
		status.State = store.StateApplied
		status.Notes = "Feature still needed — partial upstream adoption"
	case store.ReconcileShadowAwaiting:
		status.State = store.StateReconcilingShadow
		status.Notes = fmt.Sprintf("Phase 3.5 staged %d resolved file(s) in shadow worktree — review with `tpatch reconcile --shadow-diff %s`, then `--accept` or `--reject`", len(result.ResolvedFiles), slug)
	case store.ReconcileBlockedTooManyConflicts:
		status.State = store.StateBlocked
		status.Notes = "Reconciliation blocked — conflict count exceeds --max-conflicts cap"
	case store.ReconcileBlockedRequiresHuman:
		status.State = store.StateBlocked
		status.Notes = "Phase 3.5 could not resolve all files (provider/validation failure) — manual intervention required"
	case store.ReconcileBlocked:
		status.State = store.StateBlocked
		status.Notes = "Reconciliation blocked — manual intervention needed"
	}

	s.SaveFeatureStatus(status)
}

func updateUpstreamLock(s *store.Store, ref, commit string) {
	remote, branch, ok := gitutil.SplitUpstreamRef(ref)
	if !ok {
		fmt.Fprintf(os.Stderr,
			"warning: tpatch reconcile could not update upstream.lock: malformed upstream ref %q (expected <remote>/<branch>)\n",
			ref)
		return
	}
	url, _ := gitutil.GitRemoteURL(s.Root, remote)
	content := fmt.Sprintf(`# Upstream Lock
# Updated by tpatch reconcile at %s
remote: %q
branch: %q
commit: %q
url: %q
`, time.Now().UTC().Format(time.RFC3339), remote, branch, commit, url)
	lockPath := filepath.Join(s.TpatchDir(), "upstream.lock")
	os.WriteFile(lockPath, []byte(content), 0o644)
}

func truncateCommit(commit string) string {
	if len(commit) > 8 {
		return commit[:8]
	}
	return commit
}

// deriveIncrementalPatches computes per-feature incremental patches for multi-feature scenarios.
// When features A and B are applied sequentially, B's cumulative patch includes A's changes.
// This function derives the delta (incremental) patch for each feature and saves it alongside
// the cumulative patch so reconciliation uses only the feature's own changes.
func deriveIncrementalPatches(s *store.Store, slugs []string, baseCommit string) {
	var prevCumulative string

	for _, slug := range slugs {
		currentPatch, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
		if err != nil {
			prevCumulative = currentPatch
			continue
		}

		if prevCumulative == "" {
			// First feature: incremental = cumulative
			s.WriteArtifact(slug, "incremental.patch", currentPatch)
		} else {
			// Derive delta between previous cumulative and current cumulative
			incremental, err := gitutil.DeriveIncrementalPatch(s.Root, baseCommit, prevCumulative, currentPatch)
			if err != nil || incremental == "" {
				// Fallback: use the full patch if derivation fails
				s.WriteArtifact(slug, "incremental.patch", currentPatch)
			} else {
				s.WriteArtifact(slug, "incremental.patch", incremental)
			}
		}

		prevCumulative = currentPatch
	}
}

// extractUpstreamContext reads the current contents of files affected by the patch.
// This gives the LLM the actual upstream code to compare against acceptance criteria.
func extractUpstreamContext(repoRoot, patch string) string {
	var files []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			file := strings.TrimPrefix(line, "+++ b/")
			if !seen[file] && file != "/dev/null" {
				seen[file] = true
				files = append(files, file)
			}
		} else if strings.HasPrefix(line, "--- a/") {
			file := strings.TrimPrefix(line, "--- a/")
			if !seen[file] && file != "/dev/null" {
				seen[file] = true
				files = append(files, file)
			}
		}
	}

	var b strings.Builder
	for _, file := range files {
		content, err := os.ReadFile(filepath.Join(repoRoot, file))
		if err != nil {
			b.WriteString(fmt.Sprintf("## %s\n(file not present in upstream)\n\n", file))
			continue
		}
		// Truncate large files to keep prompt manageable
		text := string(content)
		if len(text) > 4000 {
			text = text[:4000] + "\n... (truncated)"
		}
		b.WriteString(fmt.Sprintf("## %s\n```\n%s\n```\n\n", file, text))
	}
	return b.String()
}

// tryPhase35 runs the ADR-010 provider-assisted resolver for a feature
// whose forward-apply preview reported 3WayConflicts. It owns the git
// plumbing (deriving base/ours/theirs for each conflicted file) and
// then delegates to RunConflictResolve for the actual per-file loop.
//
// Assumption about "ours": reconcile runs after `tpatch apply`, so the
// feature's patched version lives in the real working tree. We read
// it from disk. If the user reconciles on a branch that has the
// feature committed but no working tree change, git show HEAD:path
// would give the same content — we prefer the on-disk read because it
// also captures any uncommitted hand edits the user intends to carry
// through reconciliation.
//
// The "base" side is derived as merge-base(HEAD, upstreamCommit). The
// ".tpatch/upstream.lock" commit from the prior reconcile is a
// tempting shortcut but may not exist on first reconcile and can be
// stale; merge-base is always authoritative.
func tryPhase35(
	ctx context.Context,
	s *store.Store,
	slug string,
	upstreamCommit string,
	prov provider.Provider,
	cfg provider.Config,
	opts ReconcileOptions,
	conflictFiles []string,
	result *ReconcileResult,
) *ReconcileResult {
	result.Phase = "phase-3.5-provider-resolve"

	// Refuse without a provider up-front — ADR-010 D9: no heuristic fallback.
	if prov == nil || !cfg.Configured() {
		result.Outcome = store.ReconcileBlockedRequiresHuman
		result.Notes = append(result.Notes,
			"phase 3.5 requested (--resolve) but no provider is configured; configure a provider (`tpatch provider set ...`) or resolve manually")
		result.Conflicts = append(result.Conflicts, conflictFiles...)
		return result
	}

	headCommit, headErr := gitutil.HeadCommit(s.Root)
	if headErr != nil {
		result.Outcome = store.ReconcileBlockedRequiresHuman
		result.Notes = append(result.Notes, fmt.Sprintf("phase 3.5: cannot resolve HEAD: %v", headErr))
		result.Conflicts = append(result.Conflicts, conflictFiles...)
		return result
	}
	baseCommit, mbErr := gitutil.MergeBase(s.Root, headCommit, upstreamCommit)
	if mbErr != nil || baseCommit == "" {
		result.Outcome = store.ReconcileBlockedRequiresHuman
		result.Notes = append(result.Notes,
			fmt.Sprintf("phase 3.5: cannot derive merge-base(HEAD, %s): %v", upstreamCommit, mbErr))
		result.Conflicts = append(result.Conflicts, conflictFiles...)
		return result
	}

	// Build inputs. A git-reported conflict file may be missing on
	// any of the three sides (rename, delete, add) — FileAtCommit
	// returns (nil, nil) for missing, which the resolver treats as
	// empty content. The on-disk read for "ours" may also fail if
	// git reported a path no longer present; same treatment.
	inputs := make([]ConflictInput, 0, len(conflictFiles))
	for _, path := range conflictFiles {
		base, berr := gitutil.FileAtCommit(s.Root, baseCommit, path)
		if berr != nil {
			result.Notes = append(result.Notes, fmt.Sprintf("phase 3.5: read base %s: %v", path, berr))
		}
		theirs, terr := gitutil.FileAtCommit(s.Root, upstreamCommit, path)
		if terr != nil {
			result.Notes = append(result.Notes, fmt.Sprintf("phase 3.5: read theirs %s: %v", path, terr))
		}
		ours, _ := os.ReadFile(filepath.Join(s.Root, path))
		inputs = append(inputs, ConflictInput{
			Path:   path,
			Base:   base,
			Ours:   ours,
			Theirs: theirs,
		})
	}

	cfgForCall := cfg
	testCmd := ""
	if conf, cErr := s.LoadConfig(); cErr == nil {
		testCmd = conf.TestCommand
	}
	resolveOpts := ResolveOptions{
		MaxConflicts:  opts.MaxConflicts,
		ModelOverride: opts.Model,
		AutoApply:     opts.Apply,
		Validation: ValidationConfig{
			TestCommand:     testCmd,
			IdentifierCheck: true,
		},
	}

	rr, err := RunConflictResolve(ctx, s, slug, prov, cfgForCall, inputs, upstreamCommit, resolveOpts)
	if err != nil {
		result.Outcome = store.ReconcileBlockedRequiresHuman
		result.Notes = append(result.Notes, fmt.Sprintf("phase 3.5 failed: %v", err))
		result.Conflicts = append(result.Conflicts, conflictFiles...)
		return result
	}

	// Thread resolver state onto the reconcile result.
	result.ShadowPath = rr.ShadowPath
	result.ResolveSession = rr.SessionID
	for _, o := range rr.Outcomes {
		switch o.Status {
		case FileStatusResolved:
			result.ResolvedFiles = append(result.ResolvedFiles, o.Path)
		case FileStatusValidationFailed, FileStatusProviderError:
			result.FailedFiles = append(result.FailedFiles, o.Path)
		case FileStatusSkippedTooLarge:
			result.SkippedFiles = append(result.SkippedFiles, o.Path)
		}
	}
	result.Conflicts = append(result.Conflicts, conflictFiles...)

	switch rr.Verdict {
	case ResolveVerdictAutoAccepted:
		// v0.5.2 fix (C2 finding #1): the resolver's "AutoAccepted"
		// verdict only means every file passed validation and the
		// caller has consented to auto-apply — the files are still
		// sitting in the shadow worktree. Previously this path set
		// ReconcileReapplied without any file copy. Now we call the
		// shared accept helper BEFORE claiming success; if any step
		// fails the shadow is preserved and the outcome flips to
		// BlockedRequiresHuman with diagnostics.
		acceptOpts := AcceptOptions{
			Phase:            "reconcile --resolve --apply",
			ResolveSessionID: rr.SessionID,
		}
		acceptRes, aerr := AcceptShadow(s, slug, result.ResolvedFiles, upstreamCommit, acceptOpts)
		if aerr != nil {
			// Preserve shadow for manual follow-up. Do NOT prune.
			result.Outcome = store.ReconcileBlockedRequiresHuman
			result.Notes = append(result.Notes,
				fmt.Sprintf("phase 3.5 resolved %d file(s) but auto-apply failed mid-flight: %v; shadow preserved for manual review (`tpatch reconcile --accept %s` or `--reject %s`)",
					len(result.ResolvedFiles), aerr, slug, slug))
			return result
		}
		result.Outcome = store.ReconcileReapplied
		result.Notes = append(result.Notes,
			fmt.Sprintf("phase 3.5 auto-accepted %d file(s) (validation + test_command passed); copied onto real tree: %s",
				len(result.ResolvedFiles), strings.Join(acceptRes.AcceptedFiles, ", ")))
		if acceptRes.RefreshWarning != "" {
			result.Notes = append(result.Notes, "auto-apply warning: "+acceptRes.RefreshWarning)
		}
		// AcceptShadow already called MarkFeatureState(applied);
		// clear the phase-3.5 bookkeeping from result so the outer
		// updateFeatureState does not rewrite the state to a stale
		// shadow pointer. Callers still see ShadowPath/ResolveSession
		// via the ReconcileResult for logging; the on-disk status
		// has been updated correctly by the helper.
		result.ShadowPath = ""
		return result
	case ResolveVerdictShadowAwaiting:
		result.Outcome = store.ReconcileShadowAwaiting
		result.Notes = append(result.Notes,
			fmt.Sprintf("phase 3.5 staged %d resolved file(s) in shadow worktree; review with `tpatch reconcile --accept %s`",
				len(result.ResolvedFiles), slug))
		return result
	case ResolveVerdictBlockedTooManyConflicts:
		result.Outcome = store.ReconcileBlockedTooManyConflicts
		result.Notes = append(result.Notes,
			fmt.Sprintf("phase 3.5 refused: %d conflict(s) exceeds cap (--max-conflicts)", len(conflictFiles)))
		return result
	case ResolveVerdictBlockedRequiresHuman:
		result.Outcome = store.ReconcileBlockedRequiresHuman
		result.Notes = append(result.Notes,
			fmt.Sprintf("phase 3.5 blocked: %d file(s) failed validation or provider; see resolution-session.json",
				len(result.FailedFiles)))
		return result
	default:
		result.Outcome = store.ReconcileBlockedRequiresHuman
		result.Notes = append(result.Notes,
			fmt.Sprintf("phase 3.5 produced unknown verdict %q; blocking", rr.Verdict))
		return result
	}
}
