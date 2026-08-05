package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/assets"
	"github.com/tesseracode/tesserapatch/internal/buildinfo"
	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

// Execute runs the tpatch CLI root command.
//
// Most command errors collapse to exit 1. Commands that need a binding
// non-1 exit code (currently only `tpatch verify`, per
// PRD-verify-freshness §6 Q7) return an *ExitCodeError; we surface that
// code here so harnesses can treat verify-failed (exit 2) as distinct
// from a generic CLI error (exit 1).
func Execute() int {
	rootCmd := buildRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		if e := asExitCodeError(err); e != nil {
			return e.ExitCode()
		}
		return 1
	}
	return 0
}

func buildRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "tpatch",
		Short:         "Tessera Patch — customize open-source projects with natural-language patches",
		Version:       buildinfo.String(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("tpatch {{.Version}}\n")

	// Persistent flags
	root.PersistentFlags().String("path", "", "Target repository path (default: current directory)")

	// Commands
	root.AddCommand(
		initCmd(),
		addCmd(),
		statusCmd(),
		analyzeCmd(),
		defineCmd(),
		exploreCmd(),
		implementCmd(),
		applyCmd(),
		recordCmd(),
		landCmd(),
		reconcileCmd(),
		providerCmd(),
		configCmd(),
		cycleCmd(),
		testCmd(),
		nextCmd(),
		editCmd(),
		amendCmd(),
		removeCmd(),
		featureCmd(),
		verifyCmd(),
		doctorCmd(),
		sessionCmd(),
		rejectCmd(),
		reopenCmd(),
	)

	return root
}

// ─── init ────────────────────────────────────────────────────────────────────

func initCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize .tpatch/ workspace and install skill formats",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveRoot(cmd, args)
			if err != nil {
				return err
			}
			s, err := store.Init(root)
			if err != nil {
				return err
			}

			// Wave γ (PRD-active-feature-session §4 D6 mandate 1 + ADR-027 D1):
			// install/preserve the effective .gitignore rule for the local
			// buffer lane at `.tpatch/local/`. If the rule cannot be written
			// (mandate 2), refuse — the error message enumerates all six
			// mandates verbatim and prints the exact rule so the user can
			// add it manually.
			//
			// v0.12.0 Wave γ rev-1 Slice R6 (F-INT-γ-4 LOW): the status
			// distinguishes appended / already-present / created so the
			// post-init summary can be honest instead of always printing
			// `appended` even when nothing was written.
			gitignoreStatus, err := workflow.EnsureLocalGitignoreRuleStatus(root)
			if err != nil {
				return err
			}

			installSkills(cmd, root)

			// GAP 6: Auto-detect provider
			autoDetectProvider(cmd, s)

			// Post-init: run a reachability probe for local endpoints (warn-continue).
			// Per ADR-004 D4 — init must never fail because the proxy is down;
			// the user may start it later. Emit a friendly pointer instead.
			postProbeCtx, cancel := context.WithTimeout(context.Background(), provider.ProbeTimeout)
			defer cancel()
			provCfg := providerConfigFromStore(s)
			maybeShowAUPWarning(cmd.OutOrStdout(), provCfg)
			warnIfUnreachable(postProbeCtx, cmd.OutOrStdout(), provCfg)

			fmt.Fprintf(cmd.OutOrStdout(), "Initialized .tpatch/ in %s\n", s.Root)
			fmt.Fprintf(cmd.OutOrStdout(), "  config:    %s\n", filepath.Join(s.TpatchDir(), "config.yaml"))
			fmt.Fprintf(cmd.OutOrStdout(), "  features:  %s\n", filepath.Join(s.TpatchDir(), "FEATURES.md"))
			fmt.Fprintf(cmd.OutOrStdout(), "  steering:  %s\n", filepath.Join(s.TpatchDir(), "steering/"))
			var gitignoreVerb string
			switch gitignoreStatus {
			case workflow.LocalIgnoreAlreadyPresent:
				gitignoreVerb = "already present"
			case workflow.LocalIgnoreAppended:
				gitignoreVerb = "appended"
			case workflow.LocalIgnoreCreated:
				gitignoreVerb = "created"
			default:
				gitignoreVerb = "installed"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  gitignore: %s %q in %s (PRD-active-feature-session §4 D6)\n",
				gitignoreVerb, workflow.LocalIgnoreRule, filepath.Join(s.Root, ".gitignore"))
			return nil
		},
	}
	return cmd
}

// ─── add ─────────────────────────────────────────────────────────────────────

func addCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <description...>",
		Short: "Create a tracked feature request (reads stdin when no args)",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			slug, _ := cmd.Flags().GetString("slug")

			var description string
			switch {
			case len(args) > 0:
				description = strings.Join(args, " ")
			case stdinIsPiped(cmd):
				raw, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("read stdin: %w", err)
				}
				description = strings.TrimSpace(string(raw))
				if description == "" {
					return fmt.Errorf("empty description on stdin")
				}
			default:
				return fmt.Errorf("provide a description as arguments or pipe via stdin")
			}

			status, err := s.AddFeature(store.AddFeatureInput{
				Title: description, Request: description, Slug: slug,
			})
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created feature: %s\n", status.Slug)
			fmt.Fprintf(cmd.OutOrStdout(), "  directory: .tpatch/features/%s/\n", status.Slug)
			fmt.Fprintf(cmd.OutOrStdout(), "  state:     %s\n", status.State)
			return nil
		},
	}
	cmd.Flags().String("slug", "", "Override feature slug")
	return cmd
}

// stdinIsPiped reports whether the command's stdin appears to be a pipe
// or redirected file rather than an interactive terminal. Uses the
// cobra-injected reader when it's an *os.File (tests using SetIn with
// a strings.Reader return true via the type-check fallback so piped
// input is always honoured).
func stdinIsPiped(cmd *cobra.Command) bool {
	in := cmd.InOrStdin()
	f, ok := in.(*os.File)
	if !ok {
		// Tests inject strings.Reader / bytes.Buffer etc.; treat as piped.
		return true
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice == 0
}

// ─── status ──────────────────────────────────────────────────────────────────

func statusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [slug]",
		Short: "Show feature status dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}

			features, err := s.ListFeatures()
			if err != nil {
				return err
			}
			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}

			asJSON, _ := cmd.Flags().GetBool("json")
			verbose, _ := cmd.Flags().GetBool("verbose")
			dagMode, _ := cmd.Flags().GetBool("dag")
			includeRejected, _ := cmd.Flags().GetBool("include-rejected")
			featureSlug, _ := cmd.Flags().GetString("feature")
			if featureSlug == "" && len(args) > 0 {
				featureSlug = args[0]
			}

			out := cmd.OutOrStdout()

			// Chunk D — status-time DAG validation: re-validate the whole
			// graph and surface cycle/dangling/kind-conflict warnings inline
			// regardless of --dag mode (PRD §6 + §10). No-op when the flag
			// is off so v0.5.x byte-identity is preserved.
			var dagWarnings []string
			if cfg.DAGEnabled() {
				for _, verr := range store.ValidateAllFeatures(s) {
					dagWarnings = append(dagWarnings, verr.Error())
				}
			}

			// feat-amend-dependent-warning (v0.7.0) — derive
			// `dependent-broken` overlay alongside freshness. We
			// compute once and reuse for both text and JSON paths
			// below. A nil/empty map means no dependent reference is
			// currently broken.
			brokenByFeature, _ := store.CollectBrokenRefs(s)
			warnMalformedPatchGenerations(cmd, s, features)

			// v0.13.0 GH #6 (PRD-rejected-feature-state §7): rejected
			// features are excluded from the default listing — both the
			// text dashboard and the JSON envelope's `features[]` array
			// — and opted back in with --include-rejected. DAG
			// validation, the malformed-manifest sweep and the
			// per-feature detail view all keep operating on the full
			// set: rejection hides a feature from the actionable
			// backlog, it does not make it unqueryable.
			allFeatures := features
			hiddenRejected := 0
			if !includeRejected {
				kept := make([]store.FeatureStatus, 0, len(features))
				for _, f := range features {
					if f.State == store.StateRejected {
						hiddenRejected++
						continue
					}
					kept = append(kept, f)
				}
				features = kept
			}
			_ = allFeatures

			// --dag short-circuits the dashboard render and emits the tree
			// (or JSON) view directly.
			if dagMode {
				if asJSON {
					return runStatusDAG(out, s, featureSlug, true, brokenByFeature)
				}
				if len(dagWarnings) > 0 {
					for _, w := range dagWarnings {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", w)
					}
				}
				return runStatusDAG(out, s, featureSlug, false, brokenByFeature)
			}

			if asJSON {
				// Slice B (ADR-013 D5): per-feature, derive the
				// freshness label at render time and emit it alongside
				// the labels array + the persisted Verify sub-record.
				type brokenRefJSON struct {
					Kind    string `json:"kind"`
					SHA     string `json:"sha"`
					Feature string `json:"feature"`
				}
				type featureWithFreshness struct {
					store.FeatureStatus
					FreshnessLabel        store.ReconcileLabel   `json:"freshness_label,omitempty"`
					RenderedLabels        []store.ReconcileLabel `json:"labels_rendered,omitempty"`
					DependentBroken       bool                   `json:"dependent_broken,omitempty"`
					ParentGenerationStale bool                   `json:"parent_generation_stale,omitempty"`
					BrokenRefs            []brokenRefJSON        `json:"broken_refs,omitempty"`
					EvidenceArtifact      string                 `json:"evidence_artifact,omitempty"`
				}
				rendered := make([]featureWithFreshness, len(features))
				for i, f := range features {
					fl := workflow.DeriveFreshnessLabel(s, f)
					labels := mergedLabels(f, fl)
					parentStale := workflow.ParentGenerationStale(s, f.Slug)
					if parentStale {
						labels = appendLabel(labels, store.LabelParentGenerationStale)
					}
					var brokenJSON []brokenRefJSON
					if refs, ok := brokenByFeature[f.Slug]; ok && len(refs) > 0 {
						labels = appendLabel(labels, store.LabelDependentBroken)
						brokenJSON = make([]brokenRefJSON, len(refs))
						for j, r := range refs {
							brokenJSON[j] = brokenRefJSON{Kind: r.Kind, SHA: r.SHA, Feature: r.Feature}
						}
					}
					rendered[i] = featureWithFreshness{
						FeatureStatus:         f,
						FreshnessLabel:        fl,
						RenderedLabels:        labels,
						DependentBroken:       len(brokenJSON) > 0,
						ParentGenerationStale: parentStale,
						BrokenRefs:            brokenJSON,
						EvidenceArtifact:      evidenceArtifactRef(s, f.Slug),
					}
				}
				payload := map[string]any{
					"root": s.Root, "provider": cfg.Provider,
					"provider_configured": cfg.Provider.Configured(),
					"features":            rendered,
				}
				if len(dagWarnings) > 0 {
					payload["dag_warnings"] = dagWarnings
				}
				if hiddenRejected > 0 {
					payload["rejected_hidden"] = hiddenRejected
				}
				data, _ := json.MarshalIndent(payload, "", "  ")
				fmt.Fprintf(out, "%s\n", data)
				return nil
			}

			fmt.Fprintf(out, "Project: %s\n", s.Root)
			if cfg.Provider.Configured() {
				fmt.Fprintf(out, "Provider: %s (%s, model=%s)\n", cfg.Provider.Type, cfg.Provider.BaseURL, cfg.Provider.Model)
			} else {
				fmt.Fprintf(out, "Provider: not configured\n")
			}
			if len(dagWarnings) > 0 {
				fmt.Fprintln(out, "DAG warnings:")
				for _, w := range dagWarnings {
					fmt.Fprintf(out, "  ⚠ %s\n", w)
				}
			}
			if len(features) == 0 {
				fmt.Fprintln(out, "Features: none")
				if hiddenRejected > 0 {
					fmt.Fprintf(out, "Rejected (hidden): %d — pass --include-rejected to show\n", hiddenRejected)
				}
				if featureSlug == "" {
					return nil
				}
			} else {
				fmt.Fprintf(out, "Features: %d\n", len(features))
			}
			for _, f := range features {
				freshness := workflow.DeriveFreshnessLabel(s, f)
				labels := mergedLabels(f, freshness)
				if workflow.ParentGenerationStale(s, f.Slug) {
					labels = appendLabel(labels, store.LabelParentGenerationStale)
				}
				if refs, ok := brokenByFeature[f.Slug]; ok && len(refs) > 0 {
					labels = appendLabel(labels, store.LabelDependentBroken)
				}
				line := fmt.Sprintf("  - %s [%s] %s", f.Slug, f.State, f.Title)
				if len(labels) > 0 {
					strs := make([]string, len(labels))
					for i, l := range labels {
						strs[i] = string(l)
					}
					line += " (" + strings.Join(strs, ", ") + ")"
				}
				fmt.Fprintln(out, line)
			}
			if len(features) > 0 && hiddenRejected > 0 {
				fmt.Fprintf(out, "Rejected (hidden): %d — pass --include-rejected to show\n", hiddenRejected)
			}
			// feat-amend-dependent-warning (v0.7.0) — emit one
			// diagnostic line per affected feature listing the abbrev
			// SHA(s) that broke, then a single recovery hint. A single
			// feature can reference the same rewritten SHA via both
			// `apply.base_commit` and `satisfied_by` — dedupe abbrevs
			// so the line collapses to one entry per unique SHA.
			if len(brokenByFeature) > 0 {
				slugs := make([]string, 0, len(brokenByFeature))
				for slug := range brokenByFeature {
					slugs = append(slugs, slug)
				}
				sortStringsAsc(slugs)
				for _, slug := range slugs {
					refs := brokenByFeature[slug]
					abbrevs := make([]string, 0, len(refs))
					seen := make(map[string]struct{}, len(refs))
					for _, r := range refs {
						a := abbrevSHA(r.SHA)
						if _, ok := seen[a]; ok {
							continue
						}
						seen[a] = struct{}{}
						abbrevs = append(abbrevs, a)
					}
					sortStringsAsc(abbrevs)
					fmt.Fprintf(out, "dependent-broken: feature %q references SHA(s) %s which are no longer reachable (likely amend / rebase upstream)\n",
						slug, strings.Join(abbrevs, ", "))
				}
				fmt.Fprintln(out, "hint: re-record affected feature(s) on the new base, or run 'tpatch reconcile' to attempt automatic re-anchor.")
			}
			if featureSlug != "" || verbose {
				slugs := []string{}
				if featureSlug != "" {
					slugs = append(slugs, featureSlug)
				} else {
					for _, f := range features {
						slugs = append(slugs, f.Slug)
					}
				}
				for _, sl := range slugs {
					st, err := s.LoadFeatureStatus(sl)
					if err != nil {
						continue
					}
					fmt.Fprintf(out, "\nDetail: %s\n", st.Slug)
					fmt.Fprintf(out, "  Title:         %s\n", st.Title)
					fmt.Fprintf(out, "  State:         %s\n", st.State)
					fmt.Fprintf(out, "  Compatibility: %s\n", st.Compatibility)
					fmt.Fprintf(out, "  Requested:     %s\n", st.RequestedAt)
					fmt.Fprintf(out, "  Updated:       %s\n", st.UpdatedAt)
					if st.Notes != "" {
						fmt.Fprintf(out, "  Notes:         %s\n", st.Notes)
					}
					// v0.13.0 GH #6: the per-feature detail view always
					// renders the full rejection record, regardless of
					// --include-rejected. Rejection hides a feature from
					// the backlog; it never hides it from a direct query
					// (PRD §3.7).
					if st.Rejection != nil {
						r := st.Rejection
						fmt.Fprintf(out, "  Rejection:\n")
						fmt.Fprintf(out, "    Reason:      %s\n", r.Reason)
						fmt.Fprintf(out, "    Note:        %s\n", r.Note)
						fmt.Fprintf(out, "    Actor:       %s\n", r.Actor)
						fmt.Fprintf(out, "    Rejected at: %s\n", r.RejectedAt.Format(time.RFC3339))
						fmt.Fprintf(out, "    Prior state: %s\n", r.PriorState)
						if r.Related != "" {
							fmt.Fprintf(out, "    Related:     %s\n", r.Related)
						}
						for _, e := range r.Evidence {
							fmt.Fprintf(out, "    Evidence:    %s (sha256=%s)\n", e.Path, e.SHA256)
						}
						fmt.Fprintf(out, "    History:     %d entr%s\n", len(r.History), pluralEntries(len(r.History)))
						if st.State == store.StateRejected {
							fmt.Fprintf(out, "    Next:        tpatch reopen %s --note \"<why this is being reconsidered>\"\n", st.Slug)
						}
					}
					if st.State == store.StateReconcilingShadow || st.Reconcile.ShadowPath != "" {
						fmt.Fprintf(out, "  Shadow:        %s\n", st.Reconcile.ShadowPath)
						if st.Reconcile.ResolveSession != "" {
							fmt.Fprintf(out, "  Session:       %s\n", st.Reconcile.ResolveSession)
						}
						fmt.Fprintf(out, "  Files:         %d resolved, %d failed, %d skipped\n",
							st.Reconcile.ResolvedFiles, st.Reconcile.FailedFiles, st.Reconcile.SkippedFiles)
						if st.State == store.StateReconcilingShadow {
							fmt.Fprintf(out, "  Next:          tpatch reconcile --shadow-diff %s  |  --accept %s  |  --reject %s\n",
								st.Slug, st.Slug, st.Slug)
						}
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "Output as JSON")
	cmd.Flags().Bool("verbose", false, "Show all feature details")
	cmd.Flags().String("feature", "", "Show detail for one feature")
	cmd.Flags().Bool("include-rejected", false, "Include rejected features in the listing (excluded by default)")
	wireStatusDagFlag(cmd)
	return cmd
}

// pluralEntries renders the "entr(y|ies)" suffix for history counts.
func pluralEntries(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

func warnMalformedPatchGenerations(cmd *cobra.Command, s *store.Store, features []store.FeatureStatus) {
	for _, f := range features {
		if _, err := store.LoadPatchGenerations(s, f.Slug); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %v\n", s.PatchGenerationsPath(f.Slug), err)
		}
	}
}

// ─── analyze ─────────────────────────────────────────────────────────────────

func analyzeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze <slug>",
		Short: "Run analysis phase on a feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			if isManualFlag(cmd) {
				return runManualPhase(cmd, s, args[0], "analyze")
			}
			timeout, _ := cmd.Flags().GetDuration("timeout")
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			prov, cfg, perr := loadAndProbeProvider(ctx, s)
			if perr != nil {
				return perr
			}
			if noRetry, _ := cmd.Flags().GetBool("no-retry"); noRetry {
				ctx = workflow.WithDisableRetry(ctx, true)
			}

			result, err := workflow.RunAnalysis(ctx, s, args[0], prov, cfg)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Analysis saved for %s\n", args[0])
			fmt.Fprintf(cmd.OutOrStdout(), "  Summary: %s\n", result.Summary)
			if result.HeuristicMode {
				fmt.Fprintln(cmd.OutOrStdout(), "  (heuristic mode — no provider connected)")
			}
			return nil
		},
	}
	cmd.Flags().Duration("timeout", 60*time.Second, "Analysis timeout")
	cmd.Flags().Bool("no-retry", false, "Disable retry-with-feedback on invalid LLM output")
	addManualFlag(cmd)
	return cmd
}

// ─── define ──────────────────────────────────────────────────────────────────

func defineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "define <slug>",
		Aliases: []string{"spec"},
		Short:   "Generate acceptance criteria and plan",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			if isManualFlag(cmd) {
				return runManualPhase(cmd, s, args[0], "define")
			}
			timeout, _ := cmd.Flags().GetDuration("timeout")
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			prov, cfg, perr := loadAndProbeProvider(ctx, s)
			if perr != nil {
				return perr
			}
			if noRetry, _ := cmd.Flags().GetBool("no-retry"); noRetry {
				ctx = workflow.WithDisableRetry(ctx, true)
			}

			if err := workflow.RunDefine(ctx, s, args[0], prov, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Spec generated for %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().Duration("timeout", 60*time.Second, "Timeout")
	cmd.Flags().Bool("no-retry", false, "Disable retry-with-feedback on invalid LLM output")
	addManualFlag(cmd)
	return cmd
}

// ─── explore ─────────────────────────────────────────────────────────────────

func exploreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "explore <slug>",
		Short: "Read codebase, find minimal changeset",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			if isManualFlag(cmd) {
				return runManualPhase(cmd, s, args[0], "explore")
			}
			timeout, _ := cmd.Flags().GetDuration("timeout")
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			prov, cfg, perr := loadAndProbeProvider(ctx, s)
			if perr != nil {
				return perr
			}
			if noRetry, _ := cmd.Flags().GetBool("no-retry"); noRetry {
				ctx = workflow.WithDisableRetry(ctx, true)
			}

			if err := workflow.RunExplore(ctx, s, args[0], prov, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Exploration saved for %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().Duration("timeout", 60*time.Second, "Timeout")
	cmd.Flags().Bool("no-retry", false, "Disable retry-with-feedback on invalid LLM output")
	addManualFlag(cmd)
	return cmd
}

// ─── implement ───────────────────────────────────────────────────────────────

func implementCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "implement <slug>",
		Short: "Generate deterministic apply recipe",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			if isManualFlag(cmd) {
				return runManualPhase(cmd, s, args[0], "implement")
			}
			timeout, _ := cmd.Flags().GetDuration("timeout")
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			prov, cfg, perr := loadAndProbeProvider(ctx, s)
			if perr != nil {
				return perr
			}
			if noRetry, _ := cmd.Flags().GetBool("no-retry"); noRetry {
				ctx = workflow.WithDisableRetry(ctx, true)
			}
			if noInfer, _ := cmd.Flags().GetBool("no-created-by-infer"); noInfer {
				ctx = workflow.WithDisableCreatedByInference(ctx, true)
			}

			if err := workflow.RunImplement(ctx, s, args[0], prov, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Implementation recipe generated for %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().Duration("timeout", 90*time.Second, "Timeout")
	cmd.Flags().Bool("no-retry", false, "Disable retry-with-feedback on invalid LLM output")
	cmd.Flags().Bool("no-created-by-infer", false, "Disable advisory created_by inference for replace-in-file ops (PRD §4.3.1)")
	addManualFlag(cmd)
	return cmd
}

// ─── apply ───────────────────────────────────────────────────────────────────

func applyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply <slug>",
		Short: "Execute apply recipe or record session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			mode, _ := cmd.Flags().GetString("mode")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			out := cmd.OutOrStdout()

			// v0.13.0 GH #6 (PRD §3.6 / §7): a rejected feature does not
			// participate in apply. Refuse before any recipe is loaded or
			// executed.
			if err := refuseIfRejected(s, slug, "apply"); err != nil {
				return err
			}

			// Handle --dry-run: preview recipe operations without modifying anything
			if dryRun {
				recipe, err := workflow.LoadRecipe(s, slug)
				if err != nil {
					return err
				}
				result := workflow.DryRunRecipe(s, recipe)
				fmt.Fprintf(out, "Dry-run for %s (%d operations):\n", slug, result.Operations)
				for _, msg := range result.Messages {
					fmt.Fprintf(out, "  ✓ %s\n", msg)
				}
				for _, w := range result.Warnings {
					fmt.Fprintf(out, "  ⚠ %s\n", w)
				}
				for _, e := range result.Errors {
					fmt.Fprintf(out, "  ✗ %s\n", e)
				}
				if result.Success {
					if len(result.Warnings) > 0 {
						fmt.Fprintf(out, "All operations would succeed (%d warning(s)).\n", len(result.Warnings))
					} else {
						fmt.Fprintln(out, "All operations would succeed.")
					}
				} else {
					fmt.Fprintf(out, "%d error(s) — recipe would fail.\n", len(result.Errors))
				}
				return nil
			}

			switch mode {
			case "prepare":
				return runApplyPrepare(cmd, s, slug)
			case "started":
				return runApplyStarted(cmd, s, slug)
			case "execute":
				_, err := runApplyExecute(cmd, s, slug)
				return err
			case "done":
				_, _, err := runApplyDone(cmd, s, slug)
				return err
			case "auto":
				return runApplyAuto(cmd, s, slug)
			default:
				return fmt.Errorf("unknown apply mode %q (valid: auto, prepare, started, execute, done)", mode)
			}
		},
	}
	cmd.Flags().String("mode", "auto", "Apply mode: auto (default, runs prepare→execute→done), prepare, started, execute, done")
	cmd.Flags().Bool("dry-run", false, "Preview recipe execution without modifying files")
	cmd.Flags().String("note", "", "Operator notes about the apply session")
	cmd.Flags().String("validation-status", "", "Validation outcome: passed, failed, needs_review")
	cmd.Flags().String("validation-note", "", "Details about validation")
	return cmd
}

func runApplyPrepare(cmd *cobra.Command, s *store.Store, slug string) error {
	request, _ := s.ReadFeatureFile(slug, "request.md")
	spec, _ := s.ReadFeatureFile(slug, "spec.md")
	exploration, _ := s.ReadFeatureFile(slug, "exploration.md")
	packet := fmt.Sprintf("# Apply Packet: %s\n\n## Request\n%s\n\n## Spec\n%s\n\n## Exploration\n%s\n",
		slug, request, spec, exploration)
	if err := s.WriteArtifact(slug, "apply-packet.md", packet); err != nil {
		return err
	}
	if err := s.MarkFeatureState(slug, store.StateImplementing, "apply --mode prepare", "Agent packet ready"); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Apply packet prepared for %s\n", slug)
	return nil
}

func runApplyStarted(cmd *cobra.Command, s *store.Store, slug string) error {
	if err := s.MarkFeatureState(slug, store.StateImplementing, "apply --mode started", "Implementation in progress"); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Feature %s marked as implementing\n", slug)
	return nil
}

// runApplyExecute loads the recipe, warns on stale provenance, runs the
// operations, and returns the execution result. The result is returned
// so auto-mode can roll it into a final summary without re-running.
func runApplyExecute(cmd *cobra.Command, s *store.Store, slug string) (workflow.RecipeExecResult, error) {
	return runApplyExecuteChecked(cmd, s, slug, true)
}

func runApplyExecuteChecked(cmd *cobra.Command, s *store.Store, slug string, checkParentGenerationStale bool) (workflow.RecipeExecResult, error) {
	out := cmd.OutOrStdout()
	// ADR-011 D4: when features_dependencies is on, hard-dependency parents
	// must be applied or upstream_merged before the child can execute. The
	// gate is a no-op when the flag is off, preserving v0.5.3 behaviour.
	if err := workflow.CheckDependencyGate(s, slug); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "%v\n", err)
		return workflow.RecipeExecResult{}, err
	}
	if checkParentGenerationStale {
		if err := checkParentGenerationStaleGate(cmd.ErrOrStderr(), s, slug, "apply"); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "%v\n", err)
			return workflow.RecipeExecResult{}, err
		}
	}
	recipe, err := workflow.LoadRecipe(s, slug)
	if err != nil {
		return workflow.RecipeExecResult{}, err
	}
	warnRecipeStale(cmd.ErrOrStderr(), s, slug)
	if err := s.MarkFeatureState(slug, store.StateImplementing, "apply --mode execute", "Executing recipe"); err != nil {
		return workflow.RecipeExecResult{}, err
	}
	result := workflow.ExecuteRecipe(s, recipe)
	for _, msg := range result.Messages {
		fmt.Fprintf(out, "  %s\n", msg)
	}
	// v0.12.0 Wave β (Slice 2, PRD-write-file-recipe-safety AC-11 /
	// ADR-029 D4): surface Warnings on stderr so the legacy-recipe
	// advisory ("recipe lacks preimage_hash precondition") is visible
	// to operators. Slice 4 will route supersession-downgrade notes
	// through the same channel.
	for _, w := range result.Warnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "  ⚠ %s\n", w)
	}
	for _, e := range result.Errors {
		fmt.Fprintf(cmd.ErrOrStderr(), "  ERROR: %s\n", e)
	}
	if result.Success {
		fmt.Fprintf(out, "Recipe executed: %d/%d operations succeeded\n", result.Applied, result.Operations)
		return result, nil
	}
	return result, fmt.Errorf("recipe execution failed: %d error(s)", len(result.Errors))
}

// runApplyDone captures the post-apply patch, writes apply-session.json,
// and advances the feature to state=applied. Returns the patch string
// and its byte-length so auto-mode can summarise without re-reading.
func runApplyDone(cmd *cobra.Command, s *store.Store, slug string) (patch string, patchBytes int, err error) {
	out := cmd.OutOrStdout()
	note, _ := cmd.Flags().GetString("note")
	valStatus, _ := cmd.Flags().GetString("validation-status")
	valNote, _ := cmd.Flags().GetString("validation-note")

	patch, patchErr := gitutil.CapturePatch(s.Root)
	if patchErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not capture patch: %v\n", patchErr)
	}
	if patch != "" {
		s.WriteArtifact(slug, "post-apply.patch", patch)
		patchName, _ := s.WritePatch(slug, "apply", patch)
		if patchName != "" {
			fmt.Fprintf(out, "  Saved patch: patches/%s\n", patchName)
		}
	}
	diffStat, _ := gitutil.CaptureDiffStat(s.Root)
	if diffStat != "" {
		s.WriteArtifact(slug, "post-apply-diff.txt", diffStat)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	commit, _ := gitutil.HeadCommit(s.Root)
	status, _ := s.LoadFeatureStatus(slug)
	status.Apply.BaseCommit = commit
	status.Apply.CompletedAt = now
	status.Apply.HasPatch = patch != ""
	s.SaveFeatureStatus(status)

	session := store.ApplySession{
		Slug:             slug,
		PreparedAt:       status.Apply.PreparedAt,
		StartedAt:        status.Apply.StartedAt,
		CompletedAt:      now,
		BaseCommit:       commit,
		HasPatch:         patch != "",
		OperatorNotes:    note,
		ValidationStatus: valStatus,
		ValidationNotes:  valNote,
	}
	s.SaveApplySession(slug, session)

	if valNote != "" || valStatus != "" {
		vs := valStatus
		if vs == "" {
			vs = "pending"
		}
		validationMD := fmt.Sprintf("# Manual Validation\n\n**Status**: %s\n**Timestamp**: %s\n\n## Notes\n\n%s\n", vs, now, valNote)
		s.WriteArtifact(slug, "manual-validation.md", validationMD)
	}

	if err := s.MarkFeatureState(slug, store.StateApplied, "apply --mode done", "Changes applied and recorded"); err != nil {
		return patch, len(patch), err
	}
	fmt.Fprintf(out, "Feature %s marked as applied\n", slug)
	return patch, len(patch), nil
}

// runApplyAuto chains prepare → execute → done in one shot. Stops on the
// first error; surfaces it as-is. On success, prints a consolidated
// summary naming each phase.
func runApplyAuto(cmd *cobra.Command, s *store.Store, slug string) error {
	// ADR-011 D4: gate at the top so we don't even write the apply
	// packet when hard parents are unsatisfied. Re-checked inside
	// runApplyExecute as a defence-in-depth — same call, same result.
	if err := workflow.CheckDependencyGate(s, slug); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "%v\n", err)
		return err
	}
	if err := checkParentGenerationStaleGate(cmd.ErrOrStderr(), s, slug, "apply"); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "%v\n", err)
		return err
	}
	if err := runApplyPrepare(cmd, s, slug); err != nil {
		return err
	}
	execResult, err := runApplyExecuteChecked(cmd, s, slug, false)
	if err != nil {
		return err
	}
	_, patchBytes, err := runApplyDone(cmd, s, slug)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(),
		"Feature %s: prepared → executed → recorded (%d ops, %d bytes patch)\n",
		slug, execResult.Operations, patchBytes)
	return nil
}

func checkParentGenerationStaleForReconcile(cmd *cobra.Command, s *store.Store, args []string) error {
	slugs := append([]string(nil), args...)
	if len(slugs) == 0 {
		features, err := s.ListFeatures()
		if err != nil {
			return err
		}
		for _, f := range features {
			if f.State == store.StateApplied || f.State == store.StateActive {
				slugs = append(slugs, f.Slug)
			}
		}
	}
	for _, slug := range slugs {
		if err := checkParentGenerationStaleGate(cmd.ErrOrStderr(), s, slug, "reconcile"); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "%v\n", err)
			return err
		}
	}
	return nil
}

func checkParentGenerationStaleGate(w io.Writer, s *store.Store, slug, action string) error {
	cfg, err := s.LoadConfig()
	if err != nil {
		return err
	}
	if !cfg.FeaturesDependencies {
		return nil
	}
	stale := workflow.ParentGenerationStaleDependencies(s, slug)
	if len(stale) == 0 {
		return nil
	}
	var blockers []workflow.ParentGenerationStaleDependency
	for _, dep := range stale {
		if dep.Kind == store.DependencyKindSoft {
			fmt.Fprintf(w, "warning: parent-generation-stale: feature %q has stale soft parent %q (snapshot parent_generation=%d, current generation_id=%s); run `tpatch feature patch refresh %s` or `tpatch reconcile %s` to refresh the dependency snapshot.\n", slug, dep.Slug, dep.SnapshotParentGeneration, dep.CurrentGenerationID, dep.Slug, slug)
			continue
		}
		blockers = append(blockers, dep)
	}
	if len(blockers) == 0 {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "parent-generation-stale: %s refused for feature %q because %d hard parent generation snapshot(s) are stale:", action, slug, len(blockers))
	for _, dep := range blockers {
		fmt.Fprintf(&b, "\n  - %s (snapshot parent_generation=%d, current generation_id=%s)", dep.Slug, dep.SnapshotParentGeneration, dep.CurrentGenerationID)
	}
	fmt.Fprintf(&b, "\nRun `tpatch feature patch refresh <parent>` for the stale parent or `tpatch reconcile %s` to refresh the child dependency snapshot.", slug)
	return fmt.Errorf("%s", b.String())
}

// warnRecipeStale prints a stderr warning when the recipe-provenance
// sidecar indicates EITHER the recipe was generated against a commit
// different from current HEAD OR the apply-recipe.json bytes have
// been edited since generation (content drift).
//
// No-op when the sidecar is missing (pre-provenance recipes) — this
// keeps the guard backward-compatible. The recipe_sha256 field is a
// pointer so older sidecars that predate the content-drift guard
// decode as nil and the hash check is silently skipped (with a note).
func warnRecipeStale(w io.Writer, s *store.Store, slug string) {
	raw, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "recipe-provenance.json"))
	if err != nil || strings.TrimSpace(raw) == "" {
		return
	}
	var prov workflow.RecipeProvenance
	if err := json.Unmarshal([]byte(raw), &prov); err != nil || prov.BaseCommit == "" {
		return
	}
	short := func(sha string) string {
		if len(sha) > 7 {
			return sha[:7]
		}
		return sha
	}

	head, herr := gitutil.HeadCommit(s.Root)
	if herr == nil && head != "" && head != prov.BaseCommit {
		fmt.Fprintf(w, "warning: recipe was generated at commit %s but HEAD is now %s — results may differ\n",
			short(prov.BaseCommit), short(head))
	}

	// Content-drift check. Older sidecars without the field land
	// here with RecipeSHA256 == nil; emit a one-line note so the
	// user understands why the hash guard was silent, then return.
	if prov.RecipeSHA256 == nil {
		fmt.Fprintln(w, "note: recipe provenance predates recipe-hash guard (content-drift check skipped)")
		return
	}
	recipeBytes, rerr := s.ReadFeatureFile(slug, filepath.Join("artifacts", "apply-recipe.json"))
	if rerr != nil {
		return
	}
	sum := sha256.Sum256([]byte(recipeBytes))
	current := fmt.Sprintf("%x", sum[:])
	if current != *prov.RecipeSHA256 {
		fmt.Fprintf(w, "warning: apply-recipe.json has been edited since it was generated (hash %s → %s) — results may differ\n",
			short(*prov.RecipeSHA256), short(current))
	}
}

// ─── record ──────────────────────────────────────────────────────────────────

func recordCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "record <slug>",
		Short: "Capture patches (tracked + untracked files)",
		Long: `Capture changes as a patch.

Modes:
  auto-inferred range (--auto):  tpatch record <slug> --auto [--to <ref>] [--files <paths>]
  committed range (--from):      tpatch record <slug> --from <base> [--to <ref>] [--files <paths>]
  committed range (explicit):    tpatch record <slug> --commit-range <a>..<b> [--files <paths>]
  working tree (default):        tpatch record <slug> [--files <paths>]

Use --auto when the feature is already committed and the branch tracks an
upstream — record infers the baseline from .tpatch/upstream.lock and Git
merge-base information. Use the explicit --from / --commit-range forms when
features are interleaved on the same branch (the headline scoping case for
--files). --files scopes the capture to specific paths in any mode.
Committed-range captures never include untracked working-tree files — only
the committed snapshots at the endpoints contribute to the diff.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}

			// v0.12.0 Wave γ rev-1 Slice R4 (F-EXT-γ-4 HIGH). Hoist
			// the --from-session requires --with-session mutex check
			// to fire IMMEDIATELY after flag parsing, BEFORE any
			// capture / recipe generation / artifact write. Rev-0
			// applied the check after `s.WritePatch` had already
			// written `.tpatch/features/<slug>/patches/001-record.patch`
			// and `artifacts/post-apply.patch`, so a bad invocation
			// left partial artifacts on disk. Validate-before-mutate.
			withSessionEarly, _ := cmd.Flags().GetBool("with-session")
			fromSessionEarly, _ := cmd.Flags().GetString("from-session")
			if fromSessionEarly != "" && !withSessionEarly {
				return fmt.Errorf("record: --from-session requires --with-session (PRD §8.8)")
			}

			// feat-amend-dependent-warning (v0.7.0) — if the user is
			// recording on top of an amended commit AND the rewritten
			// SHA was referenced by a downstream feature
			// (satisfied_by or base_commit), abort by default. The
			// detection signal is "HEAD@{1}'s parent equals HEAD's
			// parent" — classic `git commit --amend` shape. A missing
			// reflog (fresh clone, etc.) is a non-error: we silently
			// skip the gate rather than block on missing signal.
			forceAmend, _ := cmd.Flags().GetBool("force-amend")
			if orphans, prevHead, ok := detectAmendOrphans(s); ok && len(orphans) > 0 {
				slugs := uniqueOrphanSlugs(orphans)
				if !forceAmend {
					return fmt.Errorf("record refuses: this amend would orphan downstream feature(s) [%s]. Their satisfied_by/base_commit references would dangle (rewritten SHA: %s). Use --force-amend to proceed (you take responsibility for re-recording downstream features)",
						strings.Join(slugs, ", "), abbrevSHA(prevHead))
				}
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: --force-amend: amend orphans downstream feature(s) [%s] (rewritten SHA: %s). Re-record those features on the new base or run 'tpatch reconcile'.\n",
					strings.Join(slugs, ", "), abbrevSHA(prevHead))
			}

			fromRef, _ := cmd.Flags().GetString("from")
			toRef, _ := cmd.Flags().GetString("to")
			commitRange, _ := cmd.Flags().GetString("commit-range")
			autoBase, _ := cmd.Flags().GetBool("auto")
			allFlag, _ := cmd.Flags().GetBool("all")
			stagedFlag, _ := cmd.Flags().GetBool("staged")
			unstagedFlag, _ := cmd.Flags().GetBool("unstaged")
			claimedOnly, _ := cmd.Flags().GetBool("claimed-only")
			filesFlag, _ := cmd.Flags().GetString("files")
			allowCollisionReason, _ := cmd.Flags().GetString("allow-collision")
			allowCollisionReason = strings.TrimSpace(allowCollisionReason)
			var pathspecs []string
			if strings.TrimSpace(filesFlag) != "" {
				for _, p := range strings.Split(filesFlag, ",") {
					if p = strings.TrimSpace(p); p != "" {
						pathspecs = append(pathspecs, p)
					}
				}
			}

			// PRD-record-capture-modes §3.7: mutex validation must run
			// BEFORE any patch capture so refusals leave the working
			// tree, index, and store untouched.
			if err := validateRecordCaptureMode(recordCaptureFlags{
				All:         allFlag,
				Staged:      stagedFlag,
				Unstaged:    unstagedFlag,
				Auto:        autoBase,
				From:        fromRef,
				To:          toRef,
				CommitRange: commitRange,
				ClaimedOnly: claimedOnly,
			}); err != nil {
				return err
			}

			// PRD §3.5: --claimed-only resolves its pathspec set up
			// front. It refuses when no claims exist OR when the
			// intersection with --files is empty; in both cases we
			// must refuse BEFORE patch capture.
			var activeClaimIDs []string
			if claimedOnly {
				resolved, ids, err := resolveClaimedOnly(s, slug, pathspecs)
				if err != nil {
					return err
				}
				pathspecs = resolved
				activeClaimIDs = ids
			}

			var autoResolved *autoBaseResolution
			if autoBase {
				if fromRef != "" {
					return fmt.Errorf("--auto is mutually exclusive with --from")
				}
				if commitRange != "" {
					return fmt.Errorf("--auto is mutually exclusive with --commit-range")
				}
				if isTrackedTreeDirty(s.Root) {
					return fmt.Errorf("--auto refuses a dirty working tree.\n" +
						"  Commit or stash your changes, then rerun `tpatch record <slug> --auto`,\n" +
						"  or record the working tree without --auto: `tpatch record <slug>`.")
				}
				resolved, err := resolveAutoBase(s, toRef, cmd.ErrOrStderr())
				if err != nil {
					return err
				}
				autoResolved = resolved
				// Print the decision line (PRD §3.1).
				autoLabel := resolved.ToLabel
				if autoLabel == "" {
					autoLabel = "HEAD"
				}
				equiv := fmt.Sprintf("tpatch record %s --from %s", slug, resolved.BaseShort)
				if filesFlag != "" {
					equiv += " --files " + filesFlag
				}
				if autoLabel != "HEAD" {
					equiv += " --to " + autoLabel
				}
				fmt.Fprintf(cmd.OutOrStdout(),
					"record --auto selected base %s from %s (%s)\n",
					resolved.BaseShort, resolved.SourceRef, string(resolved.Source))
				fmt.Fprintf(cmd.OutOrStdout(), "  equivalent: %s\n", equiv)
				commitWord := "commits"
				if resolved.AheadCount == 1 {
					commitWord = "commit"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  range: %s..%s (%d %s ahead)\n",
					resolved.BaseShort, autoLabel, resolved.AheadCount, commitWord)
				if !resolved.FromFallback && resolved.AheadCount > 1 {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"  warning: %d commits in the inferred range — if unrelated feature commits are present, narrow with --files or --to.\n",
						resolved.AheadCount)
				}
				fromRef = resolved.Base
				toRef = autoLabel
			}

			if commitRange != "" {
				if fromRef != "" {
					return fmt.Errorf("--commit-range is mutually exclusive with --from")
				}
				if toRef != "" {
					return fmt.Errorf("--commit-range is mutually exclusive with --to")
				}
				parts := strings.SplitN(commitRange, "..", 2)
				if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
					return fmt.Errorf("--commit-range must be of the form <a>..<b> (got %q)", commitRange)
				}
				fromRef = strings.TrimSpace(parts[0])
				toRef = strings.TrimSpace(parts[1])
			}

			rangeMode := fromRef != "" || toRef != ""
			if rangeMode && fromRef == "" {
				return fmt.Errorf("--to requires --from (or use --commit-range <a>..<b>)")
			}
			if rangeMode && toRef == "" {
				toRef = "HEAD"
			}

			var patch string
			var stagedSummary gitutil.StagedDirtySummary
			var unstagedSummary gitutil.UnstagedDirtySummary
			switch {
			case stagedFlag:
				// PRD §3.3: refuse on overlap before capture; emit a
				// note line for unrelated unstaged paths.
				overlap, _, unrelatedUnstaged, oerr := gitutil.StagedUnstagedOverlap(s.Root, pathspecs)
				if oerr != nil {
					return fmt.Errorf("cannot inspect staged/unstaged paths: %w", oerr)
				}
				if len(overlap) > 0 {
					return fmt.Errorf("record --staged refuses: staged and unstaged edits both touch %s. Commit, stash, or split the unstaged edits, then rerun", strings.Join(overlap, ", "))
				}
				patch, stagedSummary, err = gitutil.CaptureStagedPatch(s.Root, pathspecs)
				if err != nil {
					return fmt.Errorf("cannot capture staged patch: %w", err)
				}
				if len(unrelatedUnstaged) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"note: record --staged ignored %d unrelated unstaged path(s): %s\n",
						len(unrelatedUnstaged), strings.Join(unrelatedUnstaged, ", "))
				}
			case unstagedFlag:
				overlap, unrelatedStaged, _, oerr := gitutil.StagedUnstagedOverlap(s.Root, pathspecs)
				if oerr != nil {
					return fmt.Errorf("cannot inspect staged/unstaged paths: %w", oerr)
				}
				if len(overlap) > 0 {
					return fmt.Errorf("record --unstaged refuses: staged and unstaged edits both touch %s. Commit, stash, or split the staged edits, then rerun", strings.Join(overlap, ", "))
				}
				patch, unstagedSummary, err = gitutil.CaptureUnstagedPatch(s.Root, pathspecs)
				if err != nil {
					return fmt.Errorf("cannot capture unstaged patch: %w", err)
				}
				if len(unrelatedStaged) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"note: record --unstaged ignored %d unrelated staged path(s): %s\n",
						len(unrelatedStaged), strings.Join(unrelatedStaged, ", "))
				}
			case rangeMode:
				patch, err = gitutil.CapturePatchFromCommitsScoped(s.Root, fromRef, toRef, pathspecs)
			default:
				// `--all` is byte-equivalent to the historical default
				// working-tree capture (PRD §3.1). Both paths land
				// here; the only difference is provenance below.
				patch, err = gitutil.CapturePatchScoped(s.Root, pathspecs)
			}
			if err != nil {
				return fmt.Errorf("cannot capture patch: %w", err)
			}
			if patch == "" {
				// PRD §3.3 / §3.4: --staged and --unstaged refuse on
				// empty capture with a targeted diagnostic (not the
				// auto-base / --from candidate listing that the
				// historical working-tree path produces).
				if stagedFlag {
					return fmt.Errorf("record --staged refuses: nothing staged for capture (run `git add -p` or `git add <path>` first)")
				}
				if unstagedFlag {
					return fmt.Errorf("record --unstaged refuses: no unstaged worktree edits to capture")
				}
				// bug / footgun A8 doc-record-timing: previously we
				// wrote a no-op patch and reported success, letting
				// the feature advance to state=applied with zero
				// recorded bytes. The common cause is "user committed
				// their edits before running record"; working tree is
				// then clean and CapturePatch (diff HEAD) returns "".
				// Refuse the empty capture and surface --from candidates.
				if rangeMode {
					if autoResolved != nil {
						// PRD-record-auto-base.md: refuse when an
						// --auto-inferred committed range yields zero
						// textual diff after pathspec filtering. Unlike
						// an explicit --from/--to range (where "no
						// changes" is a legitimate outcome), --auto
						// promised the operator a feature capture; an
						// empty patch silently advancing to applied is
						// a footgun.
						w := cmd.ErrOrStderr()
						commitWord := "commits"
						if autoResolved.AheadCount == 1 {
							commitWord = "commit"
						}
						fmt.Fprintf(w,
							"record --auto: inferred range %s..%s (%d %s ahead) yields zero textual diff",
							autoResolved.BaseShort, autoResolved.ToLabel,
							autoResolved.AheadCount, commitWord)
						if filesFlag != "" {
							fmt.Fprintf(w, " after filtering by --files %q", filesFlag)
						}
						fmt.Fprintln(w, ".")
						fmt.Fprintln(w, "  Recover with one of:")
						if filesFlag != "" {
							fmt.Fprintln(w, "    - drop --files to capture the full inferred range")
							fmt.Fprintln(w, "    - widen the pathspec to a path actually touched in the range")
						}
						fmt.Fprintf(w, "    - use an explicit base: tpatch record %s --from <base> --to %s\n",
							slug, autoResolved.ToLabel)
						return fmt.Errorf("record --auto: empty capture from inferred range %s..%s",
							autoResolved.BaseShort, autoResolved.ToLabel)
					}
					// Explicit --from/--to with no diff in the range —
					// preserve the legacy success semantic so harness
					// scripts and human users can probe ranges safely.
					fmt.Fprintln(cmd.OutOrStdout(), "No changes to record in the specified range")
					return nil
				}
				w := cmd.ErrOrStderr()
				fmt.Fprintln(w, "tpatch record captured 0 bytes — nothing unstaged or untracked in the working tree.")
				if gitutil.IsWorkingTreeDirty(s.Root) {
					fmt.Fprintln(w, "  (working tree is dirty, but no textual diff was produced — possibly mode-only or binary changes)")
				} else {
					fmt.Fprintln(w, "  If you already committed your feature edits and this branch tracks upstream, try:")
					fmt.Fprintln(w, "    tpatch record "+slug+" --auto")
					fmt.Fprintln(w, "  Otherwise rerun with --from <base>:")
					fmt.Fprintln(w, "    tpatch record "+slug+" --from <base-commit-or-ref>")
					commits := gitutil.RecentCommits(s.Root, 10)
					if len(commits) > 0 {
						fmt.Fprintln(w, "  Recent commits on this branch (candidates for --from base):")
						for _, c := range commits {
							fmt.Fprintf(w, "    %s  %s  %s\n", c.SHA, c.When, c.Subject)
						}
					}
				}
				return fmt.Errorf("empty capture — see diagnostic above")
			}

			// Determine capture mode once — used both by the collision
			// diagnostic (PRD-record-collision-detection §5) and by
			// generateRecordMD below. Strings are the normalized PRD
			// §4 forms; future patch-identity-metadata work consumes
			// the same labels.
			captureMode := string(captureModeWorkingTreeAll)
			if autoBase {
				captureMode = string(captureModeAutoCommittedRange)
			} else if commitRange != "" {
				captureMode = string(captureModeExplicitCommittedRange)
			} else if rangeMode {
				captureMode = string(captureModeCommittedRange)
			} else if stagedFlag {
				captureMode = string(captureModeStagedIndex)
			} else if unstagedFlag {
				captureMode = string(captureModeUnstagedWorktree)
			}

			// PRD §3.3: validate the staged patch against a temp
			// index seeded from HEAD before any writes. Run BEFORE
			// the collision scan so a malformed staged patch refuses
			// without consuming collision-scan side effects.
			if stagedFlag {
				if verr := gitutil.ValidateStagedPatch(s.Root, patch); verr != nil {
					return fmt.Errorf("record --staged refuses: %w", verr)
				}
			}

			// PRD-record-collision-detection §4: scan canonical
			// post-apply.patch files across feature directories for
			// byte-identical collisions BEFORE writing this feature's
			// canonical patch. The scan runs after empty-patch handling
			// (PRD §4 step 0: empty patches skip scanning) and before
			// any artifact write so a cross-feature refusal leaves the
			// store untouched (PRD §8 acceptance: "refuses before
			// writing any artifact for the current feature").
			if _, err := store.LoadPatchGenerations(s, slug); err != nil {
				return fmt.Errorf("record refuses: %w", err)
			}

			collision, cerr := scanCanonicalPatchCollisions(s, slug, patch)
			if cerr != nil {
				return fmt.Errorf("collision scan failed: %w", cerr)
			}
			sameFeatureDup := collision.SameFeature && len(collision.CrossFeature) == 0
			if len(collision.CrossFeature) > 0 {
				if allowCollisionReason == "" {
					printCollisionRefusal(cmd.ErrOrStderr(), slug, collision.CrossFeature, captureMode, filesFlag)
					return fmt.Errorf("record refuses: cross-feature canonical patch collision (use --allow-collision \"<reason>\" to override)")
				}
				// Override path (PRD §3.1): warn loudly on stderr and
				// proceed. The reason is also threaded into record.md
				// below so audit traces survive the session.
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: --allow-collision: writing byte-identical canonical patch despite %d existing collision(s); reason: %s\n",
					len(collision.CrossFeature), allowCollisionReason)
				for _, m := range collision.CrossFeature {
					short := m.SHA256
					if len(short) > 12 {
						short = short[:12]
					}
					fmt.Fprintf(cmd.ErrOrStderr(),
						"  collides with: %s sha256=%s... bytes=%d files=%d\n",
						m.Slug, short, m.Bytes, m.Files)
				}
			}

			// GH #5 (v0.12.1): round-trip validation must run BEFORE
			// any artifact write so a failure without --lenient leaves
			// the feature directory byte-identical to its pre-command
			// state. Historically this check ran AFTER post-apply.patch
			// and patches/NNN-record.patch had already been written, so
			// a failed round-trip printed a warning but still mutated
			// record.md / status.json / patch-generations / recipe and
			// exited zero. See PRD-record-roundtrip-transactional.
			//
			// Semantics:
			//   --lenient  → warn on stderr, continue with writes
			//               (unchanged from pre-fix behavior).
			//   --staged   → ValidateStagedPatch already ran up top;
			//               the worktree reverse-apply check is the
			//               wrong validation for that capture mode.
			//   default    → reverse-apply against the working tree;
			//               refuse (exit non-zero, no writes) on
			//               failure.
			lenient, _ := cmd.Flags().GetBool("lenient")
			switch {
			case lenient:
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: --lenient: skipping patch round-trip validation")
			case stagedFlag:
				fmt.Fprintf(cmd.OutOrStdout(), "  Patch validated: applies cleanly against temp index seeded from HEAD\n")
			default:
				if valErr := gitutil.ValidatePatchReverse(s.Root, patch); valErr != nil {
					w := cmd.ErrOrStderr()
					fmt.Fprintf(w, "error: %v\n", valErr)
					fmt.Fprintf(w, "  The recorded patch may not represent the on-disk changes accurately.\n")
					fmt.Fprintf(w, "  Common causes: line-ending differences, binary files without --binary, or post-apply edits.\n")
					if rangeMode {
						fmt.Fprintf(w, "  hint: committed-range capture (--from/--to or --auto) covers committed history only; commit the follow-up edits (or discard them) before re-running.\n")
					}
					fmt.Fprintf(w, "  To bypass this check (not recommended for source patches), rerun with --lenient.\n")
					return fmt.Errorf("record refuses: patch does not round-trip against working tree without --lenient")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  Patch validated: round-trips cleanly against working tree\n")
			}

			// Write post-apply.patch (backwards compat) + sequential patch (GAP 7)
			if err := s.WriteArtifact(slug, "post-apply.patch", patch); err != nil {
				return err
			}
			patchName := ""
			if sameFeatureDup {
				// PRD §3.2: re-recording the same feature with
				// unchanged canonical patch bytes skips the numbered
				// audit snapshot. The canonical artifact above is
				// rewritten with identical bytes (no semantic change),
				// matching PRD §3.2's "no-op when bytes are identical"
				// guidance.
				fmt.Fprintln(cmd.OutOrStdout(),
					"  record: no content change since current artifacts/post-apply.patch; skipping numbered audit snapshot")
			} else {
				patchName, _ = s.WritePatch(slug, "record", patch)
				if patchName != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  Saved patch: patches/%s\n", patchName)
					// A9 doc-patches-vs-artifacts: patches/ is append-only
					// audit trail; surface a one-liner once the directory
					// gets crowded so users know cleanup is an option (not
					// a silent footgun). NextPatchNumber is cheap (ReadDir).
					if nextN := s.NextPatchNumber(slug); nextN > 6 {
						fmt.Fprintf(cmd.OutOrStdout(),
							"  note: %d patches accumulated under .tpatch/features/%s/patches/ — patches/NNN-*.patch is historical audit only; for replay use artifacts/post-apply.patch.\n",
							nextN-1, slug)
					}
				}
			}

			// GH #5 (v0.12.1): the round-trip validation has been
			// hoisted above the artifact writes (search for
			// "PRD-record-roundtrip-transactional" upstream). By the
			// time execution reaches this point the patch has already
			// round-tripped (or --lenient was passed), so we proceed
			// directly to diff-stat capture.

			diffStat, _ := gitutil.CaptureDiffStatScoped(s.Root, pathspecs)
			if diffStat != "" {
				s.WriteArtifact(slug, "post-apply-diff.txt", diffStat)
			}

			// GAP 3: Generate record.md
			// PRD-record-capture-modes §4: provenance fields are
			// human-readable in this slice (machine-readable metadata
			// is the next PRD's domain).
			prov := captureProvenance{
				CaptureMode: captureMode,
				Pathspecs:   pathspecs,
				ClaimIDs:    activeClaimIDs,
				BaseCommit:  fromRef,
				UpperCommit: toRef,
				DirtyState:  "",
			}
			if !rangeMode {
				prov.UpperCommit = "working-tree"
				if head, herr := gitutil.HeadCommit(s.Root); herr == nil && head != "" {
					prov.BaseCommit = head
				} else {
					prov.BaseCommit = "HEAD"
				}
			}
			if stagedFlag {
				prov.DirtyState = fmt.Sprintf("%d staged paths, %d unrelated unstaged paths",
					stagedSummary.StagedPaths, stagedSummary.UnrelatedUnstagedPaths)
			} else if unstagedFlag {
				prov.DirtyState = fmt.Sprintf("%d unstaged paths, %d unrelated staged paths",
					unstagedSummary.UnstagedPaths, unstagedSummary.UnrelatedStagedPaths)
			}
			filesChanged := countPatchFiles(patch)
			recordMD := generateRecordMD(slug, filesChanged, len(patch), diffStat, fromRef, toRef, captureMode, filesFlag, allowCollisionReason, collision.CrossFeature, prov)
			s.WriteFeatureFile(slug, "record.md", recordMD)

			status, _ := s.LoadFeatureStatus(slug)
			status.Apply.HasPatch = true
			if rangeMode {
				// PRD §3.3 — for committed-range modes (including
				// --auto, which sets rangeMode via fromRef above),
				// the persisted base_commit is the resolved lower
				// bound, NOT HEAD. This aligns with
				// docs/feature-layout.md: the canonical
				// post-apply.patch is the full diff against
				// status.apply.base_commit.
				if resolved, err := gitutil.ResolveRef(s.Root, fromRef); err == nil && resolved != "" {
					status.Apply.BaseCommit = resolved
				} else {
					status.Apply.BaseCommit = fromRef
				}
			} else {
				commit, _ := gitutil.HeadCommit(s.Root)
				if commit != "" {
					status.Apply.BaseCommit = commit
				}
			}
			s.SaveFeatureStatus(status)

			if err := s.MarkFeatureState(slug, store.StateApplied, "record", "Patch recorded"); err != nil {
				return err
			}

			// Item M15-W2.2/3 — recipe autogen + drift detection.
			// `artifacts/post-apply.patch` remains the reconcile source
			// of truth; the recipe is derived for replay/inspection only.
			noAutogen, _ := cmd.Flags().GetBool("no-recipe-autogen")
			regen, _ := cmd.Flags().GetBool("regenerate-recipe")
			autogen := !noAutogen
			action, skippedPaths, reason, agErr := workflow.AutogenRecipeForRecord(s, slug, patch, autogen, regen)
			if agErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: recipe autogen failed: %v\n", agErr)
			} else {
				out := cmd.OutOrStdout()
				w := cmd.ErrOrStderr()
				switch action {
				case workflow.AutogenGenerated:
					fmt.Fprintf(out, "  Recipe generated: artifacts/apply-recipe.json (%d ops)\n", countPatchFiles(patch)-len(skippedPaths))
				case workflow.AutogenRegenerated:
					fmt.Fprintf(out, "  Recipe regenerated from captured patch (--regenerate-recipe)\n")
					if reason != "" {
						fmt.Fprintf(w, "  drift reason: %s\n", reason)
					}
				case workflow.AutogenStale:
					fmt.Fprintln(w, "warning: apply-recipe.json no longer matches the captured patch (likely after manual edits).")
					if reason != "" {
						fmt.Fprintf(w, "  drift: %s\n", reason)
					}
					fmt.Fprintln(w, "  recipe-stale.json sidecar written. To replace the recipe with one derived from the patch, rerun:")
					fmt.Fprintf(w, "    tpatch record %s --regenerate-recipe\n", slug)
				case workflow.AutogenNoop, workflow.AutogenSkipped:
					// no user-visible message
				}
				for _, sp := range skippedPaths {
					fmt.Fprintf(w, "  recipe autogen skipped: %s\n", sp)
				}
			}

			auditPatch := ""
			if patchName != "" {
				auditPatch = "patches/" + patchName
			}
			if _, err := workflow.AppendPatchGenerationForFeature(s, slug, workflow.PatchGenerationInput{
				Kind:       "record",
				Patch:      patch,
				AuditPatch: auditPatch,
				BaseCommit: prov.BaseCommit,
				Upper:      recordGenerationUpper(s.Root, captureMode, toRef),
				Capture: store.GenerationCapture{
					Mode:      prov.CaptureMode,
					Pathspecs: prov.Pathspecs,
					ClaimIDs:  prov.ClaimIDs,
				},
			}); err != nil {
				return fmt.Errorf("record patch generation: %w", err)
			}

			// v0.12.0 Wave β rev-1 Slice R2 (PRD-write-file-recipe-
			// safety AC-7 + §4.2 "During record", ADR-029 D6):
			// scan older active/effective features for write-file
			// operations that target any path the just-recorded
			// feature touched. Emit warning-class advisories on
			// stderr. Warning-class per D6 ("Record-time later-
			// touch detection is warning-class in v1."); execution
			// of `record` continues regardless.
			if ltWarnings := workflow.DetectRecordLaterTouchWarnings(s, slug); len(ltWarnings) > 0 {
				for _, w := range ltWarnings {
					fmt.Fprintf(cmd.ErrOrStderr(), "⚠ %s\n", w)
				}
			}

			// v0.12.0 Wave γ Slice 4 (PRD-active-feature-session
			// §6 D15 + §8.7 + §8.8): opt-in session promotion at
			// record time. `--with-session` reads the same-feature
			// local buffer and writes a redacted committed summary;
			// `--from-session <cs_id>` disambiguates when multiple
			// eligible sessions exist for the same feature.
			//
			// v0.12.0 Wave γ rev-1 Slice R4 (F-EXT-γ-4 HIGH): the
			// `--from-session requires --with-session` mutex is now
			// enforced up-front at the top of RunE, before any
			// capture / recipe / artifact write. This block only
			// runs the summarize half.
			withSession, _ := cmd.Flags().GetBool("with-session")
			fromSession, _ := cmd.Flags().GetString("from-session")
			if withSession {
				target, err := pickSessionForOp(s, slug, fromSession, sessionEligibleForSummarize)
				if err != nil {
					// v0.12.0 Wave γ rev-1 Slice R6 (F-INT-γ-2 LOW):
					// pickSessionForOp's ambiguity refusal mentions
					// `--session` (correct for `session stop` /
					// `session summarize`) but at the record surface
					// the disambiguator flag is `--from-session`.
					// Rewrite the message so the operator's
					// remediation hint matches the actual flag they
					// need to pass.
					msg := err.Error()
					msg = strings.ReplaceAll(msg, "pass --session <cs_id>", "pass --from-session <cs_id>")
					return fmt.Errorf("record --with-session: %s", msg)
				}
				if err := runSessionSummarize(cmd.OutOrStdout(), s, target, sessionSummarizeOpts{
					Write: true,
				}); err != nil {
					return fmt.Errorf("record --with-session: %w", err)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Recorded patch for %s (%d bytes, %d files)\n", slug, len(patch), filesChanged)
			return nil
		},
	}
	cmd.Flags().String("from", "", "Base commit to diff from (captures committed diff instead of working tree)")
	cmd.Flags().Bool("auto", false, "Infer the committed-range base from .tpatch/upstream.lock and merge-base; mutually exclusive with --from/--commit-range")
	cmd.Flags().String("to", "", "Upper bound ref for committed-range capture (defaults to HEAD; requires --from)")
	cmd.Flags().String("commit-range", "", "Explicit committed range as <a>..<b>; mutually exclusive with --from/--to")
	cmd.Flags().Bool("lenient", false, "Skip reverse-apply round-trip validation (use for whitespace-sensitive files)")
	cmd.Flags().Bool("no-recipe-autogen", false, "Disable deriving apply-recipe.json from the captured patch when none exists")
	cmd.Flags().Bool("regenerate-recipe", false, "Overwrite an existing apply-recipe.json with one derived from the captured patch")
	cmd.Flags().String("files", "", "Comma-separated git pathspecs to scope the capture to (e.g. 'src/auth/,docs/auth.md'); default captures the full working tree")
	cmd.Flags().String("allow-collision", "", "Permit a byte-identical cross-feature canonical patch collision; the reason is recorded in record.md and printed to stderr")
	cmd.Flags().Bool("force-amend", false, "Bypass the dependent-orphan gate when recording on top of an amended commit (you take responsibility for re-recording downstream features)")
	cmd.Flags().Bool("all", false, "Explicit alias for the default working-tree capture (records `working-tree-all` provenance)")
	cmd.Flags().Bool("staged", false, "Capture only staged changes (HEAD → index); refuses when staged paths also have unstaged edits")
	cmd.Flags().Bool("unstaged", false, "Capture only unstaged changes (index → worktree); refuses when staged and unstaged edits overlap")
	cmd.Flags().Bool("claimed-only", false, "Intersect the capture with the feature's active claims; refuses when no claims exist")
	cmd.Flags().Bool("with-session", false, "Opt-in: promote the same-feature active/closed session as a redacted committed summary (PRD-active-feature-session §6 D15)")
	cmd.Flags().String("from-session", "", "Select a specific cs_<12hex> when multiple sessions are eligible; requires --with-session (PRD §6 D15 + §8.8)")
	return cmd
}

// detectAmendOrphans inspects the reflog to decide whether the current
// HEAD looks like the result of `git commit --amend` (or a one-commit
// rebase that replaced a single commit on top of the same parent), and
// if so, returns the FeatureRefs that would be orphaned by the rewrite.
//
// Detection signal: HEAD@{1}'s parent equals HEAD's parent. That is
// the byte-for-byte shape of a classic `git commit --amend` —
// new HEAD replaces old HEAD on top of the same parent commit.
//
// The third return value is `false` when the gate cannot run (no
// reflog, single root commit, git failure, etc.). Callers should
// treat that as "skip the gate" rather than block — missing signal is
// not the same as "no orphans".
func detectAmendOrphans(s *store.Store) (orphans []store.FeatureRef, prevHead string, ok bool) {
	prev, err := gitutil.RevParse(s.Root, "HEAD@{1}")
	if err != nil || prev == "" {
		return nil, "", false
	}
	prevParent, err := gitutil.RevParse(s.Root, "HEAD@{1}^")
	if err != nil || prevParent == "" {
		return nil, "", false
	}
	headParent, err := gitutil.RevParse(s.Root, "HEAD^")
	if err != nil || headParent == "" {
		return nil, "", false
	}
	if prevParent != headParent {
		// Not an amend shape — the user moved more than one commit
		// (rebase across multiple commits, merge, branch switch, …).
		return nil, "", true
	}
	curHead, err := gitutil.RevParse(s.Root, "HEAD")
	if err != nil || curHead == "" {
		return nil, "", false
	}
	if curHead == prev {
		// HEAD did not actually move — nothing was amended away.
		return nil, "", true
	}
	deps, derr := store.CollectDependentSHAs(s)
	if derr != nil {
		return nil, "", false
	}
	return store.IsAmendBreaking(prev, deps), prev, true
}

// uniqueOrphanSlugs returns the de-duplicated, sorted feature slugs
// from the orphan list. Used to format the user-facing error message
// emitted by the v0.7.0 record amend gate.
func uniqueOrphanSlugs(refs []store.FeatureRef) []string {
	seen := make(map[string]struct{}, len(refs))
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		if _, ok := seen[r.Feature]; ok {
			continue
		}
		seen[r.Feature] = struct{}{}
		out = append(out, r.Feature)
	}
	sortStringsAsc(out)
	return out
}

func countPatchFiles(patch string) int {
	count := 0
	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "diff --git") {
			count++
		}
	}
	return count
}

func recordGenerationUpper(repoRoot, captureMode, toRef string) store.GenerationUpper {
	switch captureMode {
	case string(captureModeStagedIndex):
		return store.GenerationUpper{Kind: "index", Ref: "index", Commit: ""}
	case string(captureModeCommittedRange), string(captureModeAutoCommittedRange), string(captureModeExplicitCommittedRange):
		ref := toRef
		if ref == "" {
			ref = "HEAD"
		}
		commit := ""
		if resolved, err := gitutil.ResolveRef(repoRoot, ref); err == nil {
			commit = resolved
		}
		kind := "commit"
		if captureMode == string(captureModeExplicitCommittedRange) {
			kind = "range"
		}
		return store.GenerationUpper{Kind: kind, Ref: ref, Commit: commit}
	default:
		return store.GenerationUpper{Kind: "working-tree", Ref: "working-tree", Commit: ""}
	}
}

// captureProvenance is the human-readable provenance block written to
// record.md by `tpatch record` (PRD-record-capture-modes §4). Machine
// metadata is explicitly out of scope; this struct only carries the
// fields rendered into the per-feature record.md.
type captureProvenance struct {
	CaptureMode string
	Pathspecs   []string
	ClaimIDs    []string
	BaseCommit  string
	UpperCommit string
	DirtyState  string
}

func generateRecordMD(slug string, filesChanged, patchBytes int, diffStat, fromRef, toRef, captureMode, filesFlag, allowCollisionReason string, collisionMatches []collisionMatch, prov captureProvenance) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Implementation Record: %s\n\n", slug))
	b.WriteString(fmt.Sprintf("**Recorded**: %s\n", time.Now().UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("**Files changed**: %d\n", filesChanged))
	b.WriteString(fmt.Sprintf("**Patch size**: %d bytes\n", patchBytes))
	if captureMode != "" {
		b.WriteString(fmt.Sprintf("**Capture mode**: %s\n", captureMode))
	}
	if fromRef != "" {
		b.WriteString(fmt.Sprintf("**Base commit**: %s\n", fromRef))
	}
	if toRef != "" && toRef != "HEAD" {
		b.WriteString(fmt.Sprintf("**Upper bound**: %s\n", toRef))
	} else if fromRef != "" {
		b.WriteString("**Upper bound**: HEAD\n")
	}
	if filesFlag != "" {
		b.WriteString(fmt.Sprintf("**Pathspecs**: %s\n", filesFlag))
	}
	b.WriteString("\n")

	// PRD-record-collision-detection §3.1: when --allow-collision is
	// used, the operator-supplied reason is persisted to record.md so
	// the override survives the session as an audit trace.
	if allowCollisionReason != "" && len(collisionMatches) > 0 {
		b.WriteString("## Collision Override\n\n")
		b.WriteString(fmt.Sprintf("**Reason**: %s\n\n", allowCollisionReason))
		b.WriteString("This patch is byte-identical to the canonical post-apply.patch of:\n\n")
		for _, m := range collisionMatches {
			short := m.SHA256
			if len(short) > 12 {
				short = short[:12]
			}
			b.WriteString(fmt.Sprintf("- `%s` — sha256=%s... bytes=%d files=%d\n", m.Slug, short, m.Bytes, m.Files))
		}
		b.WriteString("\n")
	}

	if diffStat != "" {
		b.WriteString("## Change Summary\n\n```\n")
		b.WriteString(diffStat)
		b.WriteString("```\n\n")
	}

	// PRD-record-capture-modes §4: emit a structured (Markdown
	// key/value list) capture-provenance section so humans (and the
	// upcoming patch-identity-metadata PRD work) can see the
	// normalized capture mode, pathspec filter, active claim ids,
	// base/upper bounds, and a one-line dirty_state summary. The
	// `**Capture mode**:` field above is the legacy single-line
	// pointer; this section is the canonical block.
	if prov.CaptureMode != "" {
		b.WriteString("## Capture Provenance\n\n")
		b.WriteString(fmt.Sprintf("- **capture_mode**: `%s`\n", prov.CaptureMode))
		if len(prov.Pathspecs) > 0 {
			b.WriteString(fmt.Sprintf("- **pathspecs**: %s\n", strings.Join(prov.Pathspecs, ", ")))
		} else {
			b.WriteString("- **pathspecs**: (none)\n")
		}
		if len(prov.ClaimIDs) > 0 {
			b.WriteString(fmt.Sprintf("- **claim_ids**: %s\n", strings.Join(prov.ClaimIDs, ", ")))
		} else {
			b.WriteString("- **claim_ids**: (none)\n")
		}
		if prov.BaseCommit != "" {
			b.WriteString(fmt.Sprintf("- **base_commit**: `%s`\n", prov.BaseCommit))
		}
		if prov.UpperCommit != "" {
			b.WriteString(fmt.Sprintf("- **upper_commit**: `%s`\n", prov.UpperCommit))
		}
		if prov.DirtyState != "" {
			b.WriteString(fmt.Sprintf("- **dirty_state**: %s\n", prov.DirtyState))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Replay Instructions\n\n")
	b.WriteString("To re-apply this feature to a clean checkout:\n\n")
	b.WriteString("```bash\n")
	b.WriteString("# From the feature's artifacts directory:\n")
	b.WriteString(fmt.Sprintf("git apply .tpatch/features/%s/artifacts/post-apply.patch\n", slug))
	b.WriteString("```\n\n")
	if fromRef != "" {
		b.WriteString(fmt.Sprintf("*Patch was captured as a committed diff from `%s` to `HEAD`.*\n", fromRef))
	}

	return b.String()
}

func evidenceArtifactRef(s *store.Store, slug string) string {
	entries, err := store.LoadReconcileEvidence(s, slug)
	if err != nil || len(entries) == 0 {
		return ""
	}
	return filepath.ToSlash(filepath.Join(".tpatch", "features", slug, "artifacts", "reconcile-evidence.jsonl"))
}

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func reconcileEvidenceHints(entries []store.ReconcileEvidence) []string {
	if len(entries) == 0 {
		return nil
	}
	hints := make([]string, 0, len(entries))
	seen := map[string]bool{}
	for _, entry := range entries {
		var hint string
		if entry.EvidenceKind == store.EvidenceKindFileNovelty || entry.EvidenceKind == store.EvidenceKindHunkOverlap {
			hint = fmt.Sprintf("%s %s", entry.EvidenceKind, entry.ReasonCode)
		} else if entry.EvidenceKind == store.EvidenceKindPathRestructure {
			hint = fmt.Sprintf("%s %s", entry.EvidenceKind, entry.ReasonCode)
			if oldPrefix := evidenceOperationValue(entry.MatchedOperations, "old_prefix"); oldPrefix != "" {
				hint += " old_prefix=" + oldPrefix
			}
			if candidates := evidenceOperationValue(entry.MatchedOperations, "candidate_prefixes"); candidates != "" {
				hint += " candidate_prefixes=" + candidates
			}
			if entry.Confidence != "" {
				hint += " confidence=" + string(entry.Confidence)
			}
		} else if entry.EvidenceKind == store.EvidenceKindManualReview && containsString(entry.MatchedOperations, "confirmation-gate") {
			hint = fmt.Sprintf("confirmation-gate %s", entry.ReasonCode)
		} else {
			hint = fmt.Sprintf("%s %s", entry.Phase, entry.EvidenceKind)
		}
		if hint == " " || seen[hint] {
			continue
		}
		seen[hint] = true
		hints = append(hints, hint)
	}
	return hints
}

func evidenceOperationValue(ops []string, key string) string {
	prefix := key + "="
	for _, op := range ops {
		if strings.HasPrefix(op, prefix) {
			return strings.TrimPrefix(op, prefix)
		}
	}
	return ""
}

// ─── reconcile ───────────────────────────────────────────────────────────────

func reconcileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reconcile [slug...]",
		Short: "Reconcile features against upstream",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Phase-3.5 (M12) terminal operations. These act on the
			// shadow state left behind by a prior `reconcile --resolve`
			// and do NOT re-run the reconcile pipeline. They are
			// mutually exclusive with each other and with --resolve.
			acceptSlug, _ := cmd.Flags().GetString("accept")
			rejectSlug, _ := cmd.Flags().GetString("reject")
			shadowDiffSlug, _ := cmd.Flags().GetString("shadow-diff")
			resolve, _ := cmd.Flags().GetBool("resolve")
			apply, _ := cmd.Flags().GetBool("apply")
			maxConflicts, _ := cmd.Flags().GetInt("max-conflicts")
			modelOverride, _ := cmd.Flags().GetString("model")
			cumulativeLegacy, _ := cmd.Flags().GetBool("cumulative-legacy")
			checkApplied, _ := cmd.Flags().GetBool("check-applied-only")
			autoDrop, _ := cmd.Flags().GetBool("auto-drop-merged")
			format, _ := cmd.Flags().GetString("format")

			if err := validateReconcileFlags(acceptSlug, rejectSlug, shadowDiffSlug, resolve, apply); err != nil {
				return err
			}
			if checkApplied && autoDrop {
				return fmt.Errorf("reconcile: --check-applied-only and --auto-drop-merged are mutually exclusive")
			}
			if format != "human" && format != "json" {
				return fmt.Errorf("reconcile: unsupported --format %q (expected human or json)", format)
			}

			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}

			if acceptSlug != "" {
				return runReconcileAccept(cmd, s, acceptSlug)
			}
			if rejectSlug != "" {
				return runReconcileReject(cmd, s, rejectSlug)
			}
			if shadowDiffSlug != "" {
				return runReconcileShadowDiff(cmd, s, shadowDiffSlug)
			}

			if err := checkParentGenerationStaleForReconcile(cmd, s, args); err != nil {
				return err
			}

			// v0.13.0 GH #6 (PRD §3.6 / §7 / §9 item 15): rejected
			// features do not participate in reconcile. Explicitly named
			// rejected slugs are dropped with a per-slug warning so a
			// multi-slug sweep is not aborted; if every named slug is
			// rejected there is nothing left to do and we refuse with
			// exit 3. The default (no-args) sweep never picks up a
			// rejected feature — it only walks applied/active.
			if len(args) > 0 {
				kept := make([]string, 0, len(args))
				var refusals []error
				for _, sl := range args {
					if rerr := refuseIfRejected(s, sl, "reconcile"); rerr != nil {
						refusals = append(refusals, rerr)
						continue
					}
					kept = append(kept, sl)
				}
				if len(kept) == 0 && len(refusals) > 0 {
					return refusals[0]
				}
				for _, rerr := range refusals {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping — %v\n", rerr)
				}
				args = kept
			}

			if checkApplied {
				upstreamRef, _ := cmd.Flags().GetString("upstream-ref")
				return runReconcileCheckAppliedOnly(cmd, s, args, upstreamRef)
			}

			// A10 doc-reconcile-workflow: hard-refuse dirty trees /
			// lingering conflict markers. See docs/reconcile.md for
			// the rationale — silent corruption beats loud failure.
			allowDirty, _ := cmd.Flags().GetBool("allow-dirty")
			allowStaleLock, _ := cmd.Flags().GetBool("allow-stale-lock")
			upstreamRef, _ := cmd.Flags().GetString("upstream-ref")
			preflight, pfErr := gitutil.PreflightReconcileWithOverride(s.Root, upstreamRef)
			if pfErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: reconcile preflight failed: %v\n", pfErr)
			} else if !preflight.Clean() {
				printReconcilePreflight(cmd.ErrOrStderr(), preflight, allowDirty)
				if !allowDirty {
					return fmt.Errorf("reconcile refused — see preflight diagnostic above")
				}
			}
			// Lock-guard (PRD-reconcile-lock-guard §3, §7.3).
			// Independent of the working-tree gate — separate
			// override flag.
			if pfErr == nil {
				switch preflight.LockState {
				case gitutil.LockStateValid, gitutil.LockStateUnknown:
				case gitutil.LockStateEmpty:
					printEmptyLockWarning(cmd.ErrOrStderr())
				case gitutil.LockStateMissing:
					printMissingLockWarning(cmd.ErrOrStderr())
				case gitutil.LockStateSkipped:
					printSkippedLockNote(cmd.ErrOrStderr(), preflight.LockDiagnostic)
				case gitutil.LockStateStale:
					if !allowStaleLock {
						fmt.Fprintln(cmd.ErrOrStderr(), formatStaleLockRefusal(preflight.LockDiagnostic))
						return fmt.Errorf("reconcile refused — upstream.lock is stale (pass --allow-stale-lock to override)")
					}
					printStaleLockOverrideWarning(cmd.ErrOrStderr(), preflight.LockDiagnostic)
				}
			}
			preflightOnly, _ := cmd.Flags().GetBool("preflight")
			if preflightOnly {
				if preflight.Clean() {
					fmt.Fprintln(cmd.OutOrStdout(), "Preflight: clean. Reconcile is safe to run.")
				}
				return nil
			}

			timeout, _ := cmd.Flags().GetDuration("timeout")
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			prov, cfg, perr := loadAndProbeProvider(ctx, s)
			if perr != nil {
				return perr
			}

			opts := workflow.ReconcileOptions{
				Resolve:          resolve,
				Apply:            apply,
				Model:            modelOverride,
				MaxConflicts:     maxConflicts,
				CumulativeLegacy: cumulativeLegacy,
			}
			results, err := workflow.RunReconcile(ctx, s, args, upstreamRef, prov, cfg, opts)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			for i := range results {
				if err := runConfirmedUpstreamedRetirementAudit(s, &results[i]); err != nil {
					return err
				}
			}
			if format == "json" {
				data, _ := json.MarshalIndent(results, "", "  ")
				fmt.Fprintf(out, "%s\n", data)
				return nil
			}
			fmt.Fprintf(out, "Reconciled %d feature(s) against %s\n", len(results), upstreamRef)
			for _, result := range results {
				displayOutcome := reconcileDisplayOutcome(result)
				if result.Outcome == store.ReconcileBlocked && result.BlockedCategory != "" {
					fmt.Fprintf(out, "  - %s [%s] (%s) %s\n", result.Slug, displayOutcome, result.Phase, result.Title)
					fmt.Fprintf(out, "    %s: blocked (%s)\n", result.Slug, result.BlockedCategory)
					for _, ev := range result.BlockedEvidence {
						fmt.Fprintf(out, "    evidence: %s\n", ev)
					}
					fmt.Fprintf(out, "    next: %s\n", result.RecommendedAction)
				} else {
					fmt.Fprintf(out, "  - %s [%s] (%s) %s\n", result.Slug, displayOutcome, result.Phase, result.Title)
				}
				for _, hint := range reconcileEvidenceHints(result.Evidence) {
					fmt.Fprintf(out, "    evidence: %s\n", hint)
				}
				for _, note := range result.Notes {
					fmt.Fprintf(out, "    %s\n", note)
				}
				if result.RetirementAudit != nil && len(result.RetirementAudit.Findings) > 0 {
					for _, line := range workflow.RetirementAuditLines(*result.RetirementAudit) {
						fmt.Fprintf(out, "    %s\n", line)
					}
				}
				if result.ShadowPath != "" {
					fmt.Fprintf(out, "    shadow:   %s\n", result.ShadowPath)
					fmt.Fprintf(out, "    files:    %d resolved, %d failed, %d skipped\n",
						len(result.ResolvedFiles), len(result.FailedFiles), len(result.SkippedFiles))
					if result.Outcome == store.ReconcileShadowAwaiting {
						fmt.Fprintf(out, "    next:     tpatch reconcile --shadow-diff %s  |  --accept %s  |  --reject %s\n",
							result.Slug, result.Slug, result.Slug)
					}
				}
			}

			// PRD-patch-already-upstream-detector §3.3: opt-in
			// post-pass that converts a phase-1.5 verdict into an
			// actual feature drop + audit-preserving removal commit.
			// No-op when --auto-drop-merged was not passed, or when
			// no result reached the phase-1.5 short-circuit (e.g.
			// the detector is off via Config.PatchIDDetectorEnabled,
			// in which case phase 1.5 cannot fire and this is a
			// silent no-op per the brief).
			if autoDrop {
				targets := make([]resultDropTarget, 0, len(results))
				for _, r := range results {
					targets = append(targets, resultDropTarget{
						Slug:           r.Slug,
						Phase:          r.Phase,
						Outcome:        r.Outcome,
						UpstreamCommit: r.UpstreamCommit,
					})
				}
				if dropErr := reconcileAutoDropMerged(cmd, s, targets); dropErr != nil {
					return dropErr
				}
			}

			// Tip: if .tpatch/ is untracked the user's feature state will
			// not travel with their branch. Cheap to check post-run.
			if isTpatchUntracked(s.Root) {
				fmt.Fprintln(out, "tip: .tpatch/ is not tracked; consider `git add .tpatch/` so feature state travels with your branch.")
			}
			return nil
		},
	}
	cmd.Flags().String("upstream-ref", "upstream/main", "Upstream ref to reconcile against")
	cmd.Flags().Duration("timeout", 120*time.Second, "Reconciliation timeout")
	cmd.Flags().Bool("preflight", false, "Only run the preflight checks and exit (does not reconcile)")
	cmd.Flags().Bool("allow-dirty", false, "Bypass the clean-tree requirement (verdicts may be wrong — not recommended)")
	cmd.Flags().Bool("allow-stale-lock", false, "Bypass the upstream.lock validation guard (verdicts may be computed against a baseline that no longer exists in upstream)")
	// M12 / ADR-010 phase-3.5 flags.
	cmd.Flags().Bool("resolve", false, "Enable provider-assisted conflict resolution (phase 3.5) on 3-way conflicts")
	cmd.Flags().Bool("apply", false, "With --resolve, auto-copy the shadow worktree onto the real tree on full success (skips human review)")
	cmd.Flags().Int("max-conflicts", 0, "With --resolve, cap the number of conflicted files per feature (0 = workflow default)")
	cmd.Flags().String("model", "", "With --resolve, override the provider model for phase-3.5 calls only")
	cmd.Flags().String("accept", "", "Accept a shadow worktree: copy resolved files onto the real tree and transition state → applied")
	cmd.Flags().String("reject", "", "Reject a shadow worktree: prune it and roll feature state back to applied. "+reconcileRejectDisambiguation)
	cmd.Flags().String("shadow-diff", "", "Emit a unified diff between shadow and real tree for a feature (review without accepting)")
	// PRD-patch-already-upstream-detector §3.2 / §3.3 (v0.8.1).
	cmd.Flags().Bool("check-applied-only", false, "Read-only: run only phase 1 (reverse-apply) + phase 1.5 (patch-id sweep) for the given slug. Forces phase 1.5 even when patch_id_detector_enabled=false (per-invocation opt-in). Writes no artifacts. Exit 0 on phase-1.5 match, 2 on no match. Mutually exclusive with --auto-drop-merged.")
	cmd.Flags().Bool("auto-drop-merged", false, "On a phase-1.5 patch-id match, remove the feature from the DAG (ADR-011 cascade rules) and create a removal commit that preserves Tpatch-CVE / Tpatch-Slug trailers. Off by default. No-op when phase 1.5 does not fire (including when patch_id_detector_enabled=false). Mutually exclusive with --check-applied-only.")
	// PRD-multi-slug-reconcile-canonical-safety §4.2 / ADR-030 D2 (v0.12.1).
	// Opt-in flag re-enabling the pre-v0.12.1 cumulative-derivation branch
	// (deriveIncrementalPatches). Default OFF: multi-slug reconcile uses each
	// feature's canonical post-apply.patch as authoritative. When set, the
	// ADR-011 D9 DAG-topological reorder is skipped (ADR-030 D6) and
	// phase 1.5 patch-id detection is skipped (ADR-030 D7).
	cmd.Flags().Bool("cumulative-legacy", false, "Opt in to pre-v0.12.1 cumulative delta derivation for multi-slug reconcile. Historical behavior for stacks recorded cumulatively (each canonical patch is a superset of the previous slug's). Skips the ADR-011 D9 DAG-topological reorder and phase 1.5 patch-id detection. See ADR-030.")
	cmd.Flags().String("format", "human", "Output format: human or json")
	cmd.AddCommand(reconcileReviewCmd(), reconcileAuditRetirementCmd(), reconcileConfirmUpstreamedCmd())
	return cmd
}

func reconcileDisplayOutcome(result workflow.ReconcileResult) string {
	if result.ReviewVerdict == "rejected-upstreamed" {
		return "upstreamed-candidate"
	}
	return string(result.Outcome)
}

func reconcileAuditRetirementCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit-retirement <slug>",
		Short: "Read-only audit of retired feature dependency metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			report, err := workflow.AuditRetirement(s, args[0])
			if err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				data, _ := json.MarshalIndent(report, "", "  ")
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", data)
				return nil
			}
			for _, line := range workflow.RetirementAuditLines(report) {
				fmt.Fprintln(cmd.OutOrStdout(), line)
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "Emit JSON")
	return cmd
}

func reconcileConfirmUpstreamedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "confirm-upstreamed <slug>",
		Short: "Confirm an upstreamed feature and run retirement cleanup audit",
		Long: `Confirm an upstreamed feature and run retirement cleanup audit.

Two entry paths are supported (PRD-confirm-upstreamed-human-review-path
§4-§5): the v0.11 "fast path" (status.Reconcile.Outcome already
upstreamed or review_verdict already confirmed-upstreamed) and the
human-review path (consumes a recorded "reconcile review add" revision
plus --upstream-commit).

Consumer note (GH #5 follow-up, PRD §AC-2): on the fast path, the
--json/--format json payload emits the base ReconcileResult shape with
upstream_ref/upstream_commit present but forced to EMPTY STRINGS -- the
populated values already recorded in status.json are NOT propagated
through the fast-path JSON. This is an accepted byte-identity tradeoff
(AC-2), not a bug -- it keeps fast-path output stable across releases.
The review path's JSON payload DOES carry the real upstream_commit/
upstream_ref values, plus source_revision_entry_id and
transition_revision_entry_id. Consumers that need the full status
record (populated upstream_ref/upstream_commit) should read
"tpatch status --json <slug>" instead of relying on this command's
fast-path JSON alone. Retirement audit detail (retirement_audit) is
emitted by the reconcile and reconcile confirm-upstreamed commands'
JSON payloads whenever the confirmation audit runs -- this includes
both the fast path and the review path of confirm-upstreamed. It is
absent otherwise (the field carries omitempty, so nil suppresses it)
and is not part of tpatch status --json.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			slug := args[0]
			status, err := s.LoadFeatureStatus(slug)
			if err != nil {
				return err
			}
			upstreamCommit, _ := cmd.Flags().GetString("upstream-commit")
			fromRevision, _ := cmd.Flags().GetString("from-revision")

			fastPath := status.Reconcile.Outcome == store.ReconcileUpstreamed || status.Reconcile.ReviewVerdict == "confirmed-upstreamed"

			var transitionInfo *confirmUpstreamedTransition
			if fastPath {
				// PRD §5 fast-path entry invariant: refuse when the
				// shipped-path atomicity contract has been broken.
				if status.State != store.StateUpstreamMerged {
					return fmt.Errorf("confirm-upstreamed: fast path requires state %q for %s but found %q (Reconcile.Outcome=%q ReviewVerdict=%q); refusing to run audit against inconsistent metadata", store.StateUpstreamMerged, slug, status.State, status.Reconcile.Outcome, status.Reconcile.ReviewVerdict)
				}
			} else {
				// Review path — v0.11 gate would refuse today. Look for
				// an authorising human review revision.
				if upstreamCommit == "" {
					// Fall back to the v0.11 gate error verbatim when
					// there is no signal at all that the reviewer path
					// is being taken. This preserves byte-identical
					// wording for the pre-existing failure mode.
					if _, ok, ferr := findAuthorisingReviewRevision(s, slug, fromRevision); ferr == nil && !ok {
						return fmt.Errorf("confirm-upstreamed requires reconcile outcome %q or review_verdict %q for %s", store.ReconcileUpstreamed, "confirmed-upstreamed", slug)
					}
					return fmt.Errorf("confirm-upstreamed: review path requires --upstream-commit <sha> for %s", slug)
				}
				consumed, ok, ferr := findAuthorisingReviewRevision(s, slug, fromRevision)
				if ferr != nil {
					return ferr
				}
				if !ok {
					return fmt.Errorf("confirm-upstreamed: feature %q has review_verdict %q and no non-superseded revision authorises retirement. Record a review with:\n  tpatch reconcile review add %s --verdict confirmed --action confirmed-retired --final-state upstream_merged --reason-code manual-review [--evidence <id>]\nthen re-run confirm-upstreamed with --upstream-commit <sha>.", slug, status.Reconcile.ReviewVerdict, slug)
				}
				if err := checkConfirmUpstreamedSupersessionSafety(s, slug); err != nil {
					return err
				}
				resolvedRef, err := verifyUpstreamCommitReachability(cmd, s, status, upstreamCommit)
				if err != nil {
					return err
				}
				transitionInfo = &confirmUpstreamedTransition{
					ConsumedEntry:  consumed,
					UpstreamCommit: upstreamCommit,
					ResolvedRef:    resolvedRef,
					PriorOutcome:   status.Reconcile.Outcome,
				}
				if err := applyConfirmUpstreamedTransition(s, &status, transitionInfo); err != nil {
					return err
				}
			}

			// PRD-#4 rev-1 F-1 (AC-2 byte-identity): fast path emits the
			// pre-PRD-#4 shape — Outcome + ReviewVerdict + Slug only.
			// The review path carries UpstreamCommit + UpstreamRef in
			// the JSON payload because they are new signals from the
			// transition (status.json still records them via
			// applyConfirmUpstreamedTransition regardless of path).
			result := workflow.ReconcileResult{Slug: slug, Outcome: status.Reconcile.Outcome, ReviewVerdict: status.Reconcile.ReviewVerdict}
			if transitionInfo != nil {
				result.UpstreamCommit = status.Reconcile.UpstreamCommit
				result.UpstreamRef = status.Reconcile.UpstreamRef
			}
			if result.ReviewVerdict == "" && result.Outcome == store.ReconcileUpstreamed {
				result.ReviewVerdict = "confirmed-upstreamed"
			}
			if transitionInfo != nil {
				result.Revisions = append(result.Revisions, transitionInfo.TransitionEntry)
			}
			if err := runConfirmedUpstreamedRetirementAudit(s, &result); err != nil {
				return err
			}
			format, _ := cmd.Flags().GetString("format")
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				format = "json"
			}
			if format != "human" && format != "json" {
				return fmt.Errorf("confirm-upstreamed: unsupported --format %q (expected human or json)", format)
			}
			if format == "json" {
				payload := any(result)
				if transitionInfo != nil {
					m, _ := marshalReconcileResultMap(result)
					m["source_revision_entry_id"] = transitionInfo.ConsumedEntry.EntryID
					m["transition_revision_entry_id"] = transitionInfo.TransitionEntry.EntryID
					m["upstream_commit"] = transitionInfo.UpstreamCommit
					payload = m
				}
				data, _ := json.MarshalIndent(payload, "", "  ")
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", data)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "confirmed upstreamed: %s\n", slug)
			if transitionInfo != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "consumed review revision: %s\n", transitionInfo.ConsumedEntry.EntryID)
				fmt.Fprintf(cmd.OutOrStdout(), "appended transition revision: %s\n", transitionInfo.TransitionEntry.EntryID)
			}
			if result.RetirementAudit != nil {
				for _, line := range workflow.RetirementAuditLines(*result.RetirementAudit) {
					fmt.Fprintln(cmd.OutOrStdout(), line)
				}
			}
			if len(result.Revisions) > 0 {
				cleanup := 0
				for _, r := range result.Revisions {
					if r.ActionTaken == store.ReconcileActionCleanupNeeded {
						cleanup++
					}
				}
				if cleanup > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "cleanup-needed revisions: %d\n", cleanup)
				}
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "Emit JSON")
	cmd.Flags().String("format", "human", "Output format: human or json")
	cmd.Flags().String("upstream-commit", "", "Upstream commit SHA that absorbed the feature (required on the review path)")
	cmd.Flags().String("from-revision", "", "Consume a specific review revision entry id (defaults to latest non-superseded match)")
	return cmd
}

// confirmUpstreamedTransition carries the review-path artifacts between
// the reviewer selection step and the state-mutation step.
type confirmUpstreamedTransition struct {
	ConsumedEntry   store.ReconcileRevision
	TransitionEntry store.ReconcileRevision
	UpstreamCommit  string
	// ResolvedRef is the upstream ref (or "HEAD") against which
	// reachability was checked. Used for logging.
	ResolvedRef  string
	PriorOutcome store.ReconcileOutcome
}

// findAuthorisingReviewRevision returns the latest non-superseded
// revision on the feature that matches the authorising tuple
// (verdict=confirmed, action=confirmed-retired,
// final_state=upstream_merged). When `fromRevision` is non-empty, that
// specific entry is selected and validated against the tuple.
//
// PRD §4 D4: reuse the same latestRevisionEntries filter that
// `reconcile review list --all=false` uses (dedupe + drop entries
// pointed at by SupersedesEntryID), then tie-break surviving matches by
// last-in-file-order (RevisionLog.Entries[-1]).
func findAuthorisingReviewRevision(s *store.Store, slug, fromRevision string) (store.ReconcileRevision, bool, error) {
	entries, err := store.LoadReconcileRevisions(s, slug)
	if err != nil {
		return store.ReconcileRevision{}, false, err
	}
	if fromRevision != "" {
		for _, e := range entries {
			if e.EntryID != fromRevision {
				continue
			}
			if !isAuthorisingTuple(e) {
				return store.ReconcileRevision{}, false, fmt.Errorf("confirm-upstreamed: --from-revision %q does not match required tuple (review_verdict=confirmed, action_taken=confirmed-retired, final_feature_state=upstream_merged); got (%s, %s, %s)", fromRevision, e.ReviewVerdict, e.ActionTaken, e.FinalFeatureState)
			}
			// Verify not superseded.
			superseded := map[string]bool{}
			for _, other := range entries {
				if other.SupersedesEntryID != "" {
					superseded[other.SupersedesEntryID] = true
				}
			}
			if superseded[e.EntryID] {
				return store.ReconcileRevision{}, false, fmt.Errorf("confirm-upstreamed: --from-revision %q is superseded by a later entry", fromRevision)
			}
			return e, true, nil
		}
		return store.ReconcileRevision{}, false, fmt.Errorf("confirm-upstreamed: --from-revision %q not found in reconcile-revisions.jsonl for %s", fromRevision, slug)
	}
	filtered := latestRevisionEntries(entries)
	// PRD-#4 rev-1 F2 tie-break fix: latestRevisionEntries dedups
	// in-place, preserving the earliest positional slot for a
	// repeated key. That order does NOT match "last-in-file-order"
	// when a later entry re-hits an earlier evidence key (A x1 / B
	// x2 / C x1 → out=[C,B], reverse walk would return B). PRD §4
	// D4 mandates the last-in-file-order match. Reselect by walking
	// `entries` in reverse and returning the first authorising
	// entry that also survived dedup+supersession filtering.
	survivors := make(map[string]bool, len(filtered))
	for _, e := range filtered {
		survivors[e.EntryID] = true
	}
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if !survivors[e.EntryID] {
			continue
		}
		if isAuthorisingTuple(e) {
			return e, true, nil
		}
	}
	return store.ReconcileRevision{}, false, nil
}

func isAuthorisingTuple(e store.ReconcileRevision) bool {
	return e.ReviewVerdict == store.ReviewVerdictConfirmed &&
		e.ActionTaken == store.ReconcileActionConfirmedRetired &&
		e.FinalFeatureState == store.StateUpstreamMerged
}

// checkConfirmUpstreamedSupersessionSafety enforces the PRD §7.4 five-row
// matrix. When the feature being confirmed carries a `supersedes` edge
// pointing at a target, that target's state gates whether retiring the
// superseder is safe.
func checkConfirmUpstreamedSupersessionSafety(s *store.Store, slug string) error {
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return err
	}
	for _, dep := range status.DependsOn {
		if dep.Kind != store.DependencyKindSupersedes {
			continue
		}
		target, err := s.LoadFeatureStatus(dep.Slug)
		if err != nil {
			return fmt.Errorf("confirm-upstreamed: cannot load supersedes-target %q for %s: %w", dep.Slug, slug, err)
		}
		switch target.State {
		case store.StateUpstreamMerged:
			// Row 5: proceed.
			continue
		case store.StateApplied:
			// Rows 1-2 (healthy or stale satisfied_by — same class).
			return fmt.Errorf("confirm-upstreamed: refusing to retire %s while supersedes-target %q is state %q; resolve the target first (§7.4 case 1)", slug, dep.Slug, target.State)
		case store.FeatureState("promoted"):
			return fmt.Errorf("confirm-upstreamed: refusing to retire %s while supersedes-target %q is state %q (not yet closed); the promotion contract expects the superseder to remain (§7.4 case 1)", slug, dep.Slug, target.State)
		case store.StateBlocked:
			return fmt.Errorf("confirm-upstreamed: refusing to retire %s while supersedes-target %q is state %q; resolve the target's blocker first (§7.4 case 1)", slug, dep.Slug, target.State)
		default:
			// Safe default: any state not enumerated in the matrix is
			// out of scope for v1 and refuses.
			return fmt.Errorf("confirm-upstreamed: refusing to retire %s while supersedes-target %q is state %q (not enumerated in §7.4 v1 matrix)", slug, dep.Slug, target.State)
		}
	}
	return nil
}

// verifyUpstreamCommitReachability implements the PRD §7.1 two-tier
// reachability contract. Returns the resolved ref against which
// reachability was checked.
func verifyUpstreamCommitReachability(cmd *cobra.Command, s *store.Store, status store.FeatureStatus, sha string) (string, error) {
	// Tier 1 — preferred: status.Reconcile.UpstreamRef.
	if ref := strings.TrimSpace(status.Reconcile.UpstreamRef); ref != "" {
		ok, err := gitutil.IsAncestor(s.Root, sha, ref)
		if err != nil {
			return "", fmt.Errorf("confirm-upstreamed: reachability check against upstream ref %q failed: %w", ref, err)
		}
		if !ok {
			return "", fmt.Errorf("confirm-upstreamed: --upstream-commit %s is not an ancestor of upstream ref %q for %s", sha, ref, status.Slug)
		}
		return ref, nil
	}
	// Tier 1 — preferred: fall through to git's @{upstream} tracking ref.
	if resolved, err := gitutil.SymbolicFullRefName(s.Root, "@{upstream}"); err == nil {
		resolved = strings.TrimSpace(resolved)
		if resolved != "" {
			ok, aerr := gitutil.IsAncestor(s.Root, sha, resolved)
			if aerr != nil {
				return "", fmt.Errorf("confirm-upstreamed: reachability check against upstream ref %q failed: %w", resolved, aerr)
			}
			if !ok {
				return "", fmt.Errorf("confirm-upstreamed: --upstream-commit %s is not an ancestor of upstream ref %q for %s", sha, resolved, status.Slug)
			}
			return resolved, nil
		}
	}
	// Tier 2 — fall-back: HEAD-only with residual-risk warning.
	ok, err := gitutil.IsAncestor(s.Root, sha, "HEAD")
	if err != nil {
		return "", fmt.Errorf("confirm-upstreamed: reachability check against HEAD failed: %w", err)
	}
	if !ok {
		return "", fmt.Errorf("confirm-upstreamed: --upstream-commit %s is not an ancestor of HEAD for %s (no upstream ref resolvable)", sha, status.Slug)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "warning: no upstream ref resolvable for %s; verified %s is reachable from HEAD only. Local operators can insert commits into HEAD's ancestry without upstream ever seeing them — audit before relying on this transition.\n", status.Slug, sha)
	return "HEAD", nil
}

// confirmedViaReviewReasonCode is the ReasonCode applyConfirmUpstreamedTransition
// stamps onto every transition it appends. It doubles as the crash-
// recovery idempotency marker (see isConfirmedViaReviewTransition):
// no other writer in the codebase uses this literal reason code, so
// an entry bearing it plus a non-empty SupersedesEntryID is,
// unambiguously, a transition record this function itself produced.
const confirmedViaReviewReasonCode = "confirmed-via-review"

// isConfirmedViaReviewTransition reports whether `e` is itself a
// transition record previously appended by applyConfirmUpstreamedTransition
// (as opposed to a human-authored review revision from
// `tpatch reconcile review add`).
//
// PRD-#4 F-4 crash-recovery idempotency guard. applyConfirmUpstreamedTransition
// appends the transition record THEN saves status.json — two separate
// writes that are not atomic across a process crash. If the process
// crashes after the append succeeds but before the save completes,
// status.json still looks like retirement never happened, so a retry
// re-enters the review-path branch. findAuthorisingReviewRevision
// walks to the LATEST authorising, non-superseded entry — and since
// the transition record this function just appended is ITSELF an
// authorising tuple (ReviewVerdict=confirmed, ActionTaken=confirmed-
// retired, FinalFeatureState=upstream_merged) that nothing has yet
// superseded, the retry selects that transition record as
// info.ConsumedEntry. Without this guard, the retry would append a
// SECOND transition record chained on top of the first (superseding
// the first transition instead of the original human review), and
// every subsequent crash-recovery retry would extend the chain by
// one more entry.
func isConfirmedViaReviewTransition(e store.ReconcileRevision) bool {
	return e.ReasonCode == confirmedViaReviewReasonCode && e.SupersedesEntryID != ""
}

// applyConfirmUpstreamedTransition mutates status.json and appends the
// superseding transition revision. The consumed revision is left
// byte-identical in the file.
func applyConfirmUpstreamedTransition(s *store.Store, status *store.FeatureStatus, info *confirmUpstreamedTransition) error {
	// v0.13.0 GH #6 defense-in-depth guard (ADR-031 D6, PRD §7).
	// MUST be the first statement in this function: the
	// ReconcileRevision append below happens BEFORE
	// saveConfirmUpstreamedStatus runs, and the crash-recovery
	// idempotency branch reaches saveConfirmUpstreamedStatus directly.
	// A guard placed in the callee would let a false audit revision be
	// appended to a rejected feature before ever firing.
	//
	// `confirm-upstreamed` asserts "an implementation already exists
	// upstream" — the opposite verdict from rejection's "this should
	// never be implemented". Refuse with a state-machine error (exit 3)
	// before ANY mutation.
	if status != nil && status.State == store.StateRejected {
		reason := ""
		if status.Rejection != nil {
			reason = status.Rejection.Reason
		}
		return stateRefusalError(
			"cannot confirm-upstreamed feature %q: feature is rejected (reason=%s); run `tpatch reopen %s` first if this is no longer accurate",
			status.Slug, reason, status.Slug)
	}

	// Crash-recovery idempotency (PRD-#4 F-4): info.ConsumedEntry is
	// already our own previously-appended transition record — skip
	// the append (it already happened; appending again would chain a
	// second transition) and reuse it verbatim as info.TransitionEntry,
	// then fall straight through to the status.json repair that did
	// not complete before the crash.
	if isConfirmedViaReviewTransition(info.ConsumedEntry) {
		info.TransitionEntry = info.ConsumedEntry
		return saveConfirmUpstreamedStatus(s, status, info)
	}

	priorOutcome := info.PriorOutcome
	transition := store.ReconcileRevision{
		SchemaVersion:       store.ReconcileRevisionSchemaVersion,
		FeatureSlug:         status.Slug,
		EvidenceAttemptID:   info.ConsumedEntry.EvidenceAttemptID,
		RawReconcileVerdict: string(priorOutcome),
		ReviewVerdict:       store.ReviewVerdictConfirmed,
		FinalFeatureState:   store.StateUpstreamMerged,
		ActionTaken:         store.ReconcileActionConfirmedRetired,
		ReasonCode:          confirmedViaReviewReasonCode,
		ValidationRefs: []store.ValidationRef{
			{Kind: "upstream-commit", Value: info.UpstreamCommit, Result: "verified"},
			{Kind: "source-revision", Value: info.ConsumedEntry.EntryID, Result: "consumed"},
		},
		SupersedesEntryID: info.ConsumedEntry.EntryID,
	}
	if transition.RawReconcileVerdict == "" {
		transition.RawReconcileVerdict = string(store.ReconcileBlocked)
	}
	transition.EntryID = store.ComputeRevisionID(transition)
	if err := store.AppendReconcileRevision(s, status.Slug, transition); err != nil {
		return err
	}
	info.TransitionEntry = transition
	return saveConfirmUpstreamedStatus(s, status, info)
}

// saveConfirmUpstreamedStatus repairs status.json to reflect a
// confirmed-upstreamed transition. Factored out of
// applyConfirmUpstreamedTransition so the crash-recovery idempotency
// guard above can reach it without re-appending. info.TransitionEntry
// must be populated (either freshly appended or reused from a prior
// crashed run) before calling this. The Notes message references
// info.TransitionEntry.SupersedesEntryID — the ORIGINAL human review
// revision — rather than info.ConsumedEntry.EntryID directly, because
// on the crash-recovery path info.ConsumedEntry IS the transition
// record itself, not the original human review entry; on the fresh-
// append path the two values are identical by construction
// (transition.SupersedesEntryID == info.ConsumedEntry.EntryID).
func saveConfirmUpstreamedStatus(s *store.Store, status *store.FeatureStatus, info *confirmUpstreamedTransition) error {
	status.Reconcile.Outcome = store.ReconcileUpstreamed
	status.Reconcile.ReviewVerdict = "confirmed-upstreamed"
	status.Reconcile.UpstreamCommit = info.UpstreamCommit
	status.State = store.StateUpstreamMerged
	status.LastCommand = "reconcile"
	status.Notes = fmt.Sprintf("Feature adopted by upstream — confirmed via human review revision %s", info.TransitionEntry.SupersedesEntryID)
	status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return s.SaveFeatureStatus(*status)
}

// marshalReconcileResultMap round-trips a ReconcileResult through JSON
// into a map so we can add review-path envelope fields without
// widening the ReconcileResult struct.
func marshalReconcileResultMap(result workflow.ReconcileResult) (map[string]any, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func runConfirmedUpstreamedRetirementAudit(s *store.Store, result *workflow.ReconcileResult) error {
	if result == nil || result.ReviewVerdict != "confirmed-upstreamed" {
		return nil
	}
	report, err := workflow.AuditRetirement(s, result.Slug)
	if err != nil {
		return err
	}
	result.RetirementAudit = &report
	if len(report.Findings) == 0 {
		return nil
	}
	revisions, err := workflow.AppendRetirementCleanupRevisions(s, report)
	if err != nil {
		return err
	}
	result.Revisions = append(result.Revisions, revisions...)
	return nil
}

func reconcileReviewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "review", Short: "Record or list reconcile revision-pass entries"}
	add := &cobra.Command{
		Use:   "add <slug>",
		Short: "Append a reconcile revision-pass entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			slug := args[0]
			raw, _ := cmd.Flags().GetString("raw-verdict")
			review, _ := cmd.Flags().GetString("verdict")
			action, _ := cmd.Flags().GetString("action")
			reason, _ := cmd.Flags().GetString("reason-code")
			finalState, _ := cmd.Flags().GetString("final-state")
			evidence, _ := cmd.Flags().GetString("evidence")
			if review == "" || action == "" || reason == "" {
				return fmt.Errorf("reconcile review add: --verdict, --action, and --reason-code are required")
			}
			status, err := s.LoadFeatureStatus(slug)
			if err != nil {
				return err
			}
			if raw == "" {
				raw = string(status.Reconcile.Outcome)
			}
			if finalState == "" {
				finalState = string(status.State)
			}
			entry := store.ReconcileRevision{
				SchemaVersion:       store.ReconcileRevisionSchemaVersion,
				FeatureSlug:         slug,
				EvidenceAttemptID:   evidence,
				RawReconcileVerdict: raw,
				ReviewVerdict:       store.ReconcileReviewVerdict(review),
				FinalFeatureState:   store.FeatureState(finalState),
				ActionTaken:         store.ReconcileActionTaken(action),
				ReasonCode:          reason,
				ValidationRefs:      []store.ValidationRef{},
			}
			entry.EntryID = store.ComputeRevisionID(entry)
			if err := store.AppendReconcileRevision(s, slug, entry); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "recorded revision %s for %s\n", entry.EntryID, slug)
			return nil
		},
	}
	add.Flags().String("raw-verdict", "", "Raw reconcile verdict being reviewed (defaults to status.json outcome)")
	add.Flags().String("verdict", "", "Review verdict: confirmed, false-positive, false-negative, inconclusive, deferred")
	add.Flags().String("action", "", "Action taken: none, confirmed-retired, reapplied, reapplied-and-recorded, implemented, deferred, skipped, cleanup-needed")
	add.Flags().String("reason-code", "", "Enumerated reason code")
	add.Flags().String("final-state", "", "Final feature state (defaults to current status.json state)")
	add.Flags().String("evidence", "", "Evidence attempt ID this revision reviews")
	list := &cobra.Command{
		Use:   "list <slug>",
		Short: "List reconcile revision-pass entries",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			entries, corrupt, err := store.LoadReconcileRevisionsLenient(s.ReconcileRevisionsPath(args[0]))
			if err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			all, _ := cmd.Flags().GetBool("all")
			if !all {
				entries = latestRevisionEntries(entries)
			}
			if asJSON {
				if corrupt == nil {
					corrupt = []store.CorruptEntry{}
				}
				data, _ := json.MarshalIndent(map[string]any{"revisions": entries, "corrupt_entries": corrupt}, "", "  ")
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", data)
				if len(corrupt) > 0 {
					return fmt.Errorf("reconcile review list: corrupt revision entries")
				}
				return nil
			}
			for _, e := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: raw=%s review=%s action=%s final=%s reason=%s evidence=%s\n", e.EntryID, e.RawReconcileVerdict, e.ReviewVerdict, e.ActionTaken, e.FinalFeatureState, e.ReasonCode, e.EvidenceAttemptID)
			}
			for _, c := range corrupt {
				fmt.Fprintf(cmd.OutOrStdout(), "corrupted entries: line %d: %s\n", c.Line, c.Error)
			}
			if len(corrupt) > 0 {
				return fmt.Errorf("reconcile review list: corrupt revision entries")
			}
			return nil
		},
	}
	list.Flags().Bool("json", false, "Emit JSON")
	list.Flags().Bool("all", false, "Include superseded entries")
	cmd.AddCommand(add, list)
	return cmd
}

func latestRevisionEntries(entries []store.ReconcileRevision) []store.ReconcileRevision {
	superseded := map[string]bool{}
	for _, e := range entries {
		if e.SupersedesEntryID != "" {
			superseded[e.SupersedesEntryID] = true
		}
	}
	out := make([]store.ReconcileRevision, 0, len(entries))
	seen := map[string]int{}
	for _, e := range entries {
		key := e.FeatureSlug + "\x00" + e.EvidenceAttemptID + "\x00" + string(e.ReviewVerdict) + "\x00" + string(e.ActionTaken)
		if idx, ok := seen[key]; ok {
			out[idx] = e
			continue
		}
		seen[key] = len(out)
		out = append(out, e)
	}
	filtered := out[:0]
	for _, e := range out {
		if !superseded[e.EntryID] {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// validateReconcileFlags refuses nonsensical combinations of the phase-3.5
// flags. Terminal operations (--accept/--reject/--shadow-diff) are
// mutually exclusive with each other and with the pipeline flags
// (--resolve/--apply).
func validateReconcileFlags(accept, reject, shadowDiff string, resolve, apply bool) error {
	terminals := 0
	if accept != "" {
		terminals++
	}
	if reject != "" {
		terminals++
	}
	if shadowDiff != "" {
		terminals++
	}
	if terminals > 1 {
		return fmt.Errorf("reconcile: --accept, --reject, --shadow-diff are mutually exclusive")
	}
	if terminals == 1 && (resolve || apply) {
		return fmt.Errorf("reconcile: --accept/--reject/--shadow-diff cannot be combined with --resolve/--apply")
	}
	if apply && !resolve {
		return fmt.Errorf("reconcile: --apply requires --resolve")
	}
	return nil
}

// runReconcileAccept delegates to workflow.AcceptShadow for the actual
// shadow → real-tree transition. See that function's package doc for
// the sequence (steps 1-5). This wrapper just validates CLI state,
// loads the resolved-files list, and forwards warnings to stderr.
func runReconcileAccept(cmd *cobra.Command, s *store.Store, slug string) error {
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return fmt.Errorf("reconcile --accept: load status: %w", err)
	}
	if st.State != store.StateReconcilingShadow {
		return fmt.Errorf("reconcile --accept: feature %q is in state %q; --accept only valid in %q",
			slug, st.State, store.StateReconcilingShadow)
	}

	files, err := loadResolvedFiles(s, slug)
	if err != nil {
		return fmt.Errorf("reconcile --accept: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("reconcile --accept: no resolved files recorded for %q (resolution-session.json missing or empty)", slug)
	}

	res, err := workflow.AcceptShadow(s, slug, files, st.Reconcile.UpstreamCommit, workflow.AcceptOptions{
		Phase:            "reconcile --accept",
		ResolveSessionID: st.Reconcile.ResolveSession,
	})
	if err != nil {
		return fmt.Errorf("reconcile --accept: %w", err)
	}
	if res.RefreshWarning != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: reconcile --accept: %s\n", res.RefreshWarning)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Accepted %d file(s) for %s; state → applied\n", len(res.AcceptedFiles), slug)
	return nil
}

// runReconcileReject discards the shadow worktree for a feature and
// rolls state back to `applied` (the pre-reconcile state). Safe to run
// from any state that has a shadow registered — the shadow is always
// off-to-the-side, never on the real tree.
func runReconcileReject(cmd *cobra.Command, s *store.Store, slug string) error {
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return fmt.Errorf("reconcile --reject: load status: %w", err)
	}
	if st.Reconcile.ShadowPath == "" {
		return fmt.Errorf("reconcile --reject: no shadow recorded for %q", slug)
	}

	if err := gitutil.PruneShadow(s.Root, slug); err != nil {
		return fmt.Errorf("reconcile --reject: prune shadow: %w", err)
	}
	if err := clearShadowPointer(s, slug); err != nil {
		return fmt.Errorf("reconcile --reject: clear shadow pointer: %w", err)
	}

	// Only roll the state back if we were parked in reconciling-shadow.
	// Other states (applied, active) mean the user ran --reject as a
	// cleanup after having already manually accepted or abandoned; do
	// not clobber those.
	if st.State == store.StateReconcilingShadow {
		if err := s.MarkFeatureState(slug, store.StateApplied, "reconcile --reject", "shadow rejected; rolled back to applied"); err != nil {
			return fmt.Errorf("reconcile --reject: mark state: %w", err)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Rejected shadow for %s; state → applied\n", slug)
	return nil
}

// runReconcileShadowDiff streams a unified diff of the shadow's
// resolved files vs the real tree to stdout. Non-destructive.
func runReconcileShadowDiff(cmd *cobra.Command, s *store.Store, slug string) error {
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return fmt.Errorf("reconcile --shadow-diff: load status: %w", err)
	}
	if st.Reconcile.ShadowPath == "" {
		return fmt.Errorf("reconcile --shadow-diff: no shadow recorded for %q", slug)
	}

	files, err := loadResolvedFiles(s, slug)
	if err != nil {
		return fmt.Errorf("reconcile --shadow-diff: %w", err)
	}
	if len(files) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no resolved files recorded in resolution-session.json)")
		return nil
	}

	diff, err := gitutil.ShadowDiff(s.Root, slug, files)
	if err != nil {
		return fmt.Errorf("reconcile --shadow-diff: %w", err)
	}
	if diff == "" {
		fmt.Fprintln(cmd.OutOrStdout(), "(shadow matches real tree — nothing to review)")
		return nil
	}
	fmt.Fprint(cmd.OutOrStdout(), diff)
	return nil
}

// loadResolvedFiles reads resolution-session.json (written by the
// phase-3.5 resolver; see workflow/resolver.go:persistSession) and
// returns the list of files whose resolver status is `resolved`
// (i.e., worth copying). Skipped and failed files are intentionally
// excluded. Split from reconcile-session.json in v0.5.3 to fix a
// dual-writer collision: saveReconcileArtifacts now owns
// reconcile-session.json exclusively.
func loadResolvedFiles(s *store.Store, slug string) ([]string, error) {
	raw, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "resolution-session.json"))
	if err != nil {
		return nil, fmt.Errorf("read resolution-session.json: %w", err)
	}
	var session struct {
		Outcomes []struct {
			Path   string `json:"path"`
			Status string `json:"status"`
		} `json:"outcomes"`
	}
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return nil, fmt.Errorf("parse resolution-session.json: %w", err)
	}
	var files []string
	for _, o := range session.Outcomes {
		if o.Status == workflow.FileStatusResolved {
			files = append(files, o.Path)
		}
	}
	return files, nil
}

// clearShadowPointer resets the phase-3.5 bookkeeping fields on the
// feature's Reconcile summary. The ResolveSession id is preserved as
// an audit record.
func clearShadowPointer(s *store.Store, slug string) error {
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return err
	}
	st.Reconcile.ShadowPath = ""
	st.Reconcile.ResolvedFiles = 0
	st.Reconcile.FailedFiles = 0
	st.Reconcile.SkippedFiles = 0
	return s.SaveFeatureStatus(st)
}

// printReconcilePreflight renders a user-facing diagnostic from the
// preflight report. Mirrors the error-message template from the A10
// todo so the remediation is reachable without leaving the terminal.
func printReconcilePreflight(w io.Writer, p gitutil.ReconcilePreflight, allowDirty bool) {
	if allowDirty {
		fmt.Fprintln(w, "warning: --allow-dirty set; reconcile will proceed against an unclean tree.")
	} else {
		fmt.Fprintln(w, "error: reconcile requires a clean working tree. Detected:")
	}
	for _, line := range p.UnstagedFiles {
		fmt.Fprintf(w, "  modified:         %s\n", line)
	}
	for _, f := range p.UntrackedFiles {
		fmt.Fprintf(w, "  untracked:        %s\n", f)
	}
	for _, f := range p.MergeMarkerFiles {
		fmt.Fprintf(w, "  merge markers:    %s\n", f)
	}
	for _, f := range p.LeftoverFiles {
		fmt.Fprintf(w, "  merge leftover:   %s\n", f)
	}
	if allowDirty {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "To recover:")
	fmt.Fprintln(w, "  - If these changes belong to an active feature, commit them first.")
	fmt.Fprintln(w, "  - If they are a half-applied merge or stash, resolve or abort first:")
	fmt.Fprintln(w, "      git merge --abort         (if mid-merge)")
	fmt.Fprintln(w, "      git reset --hard HEAD     (to discard — destructive!)")
	fmt.Fprintln(w, "      git stash                 (to set aside)")
	fmt.Fprintln(w, "  - If you understand the risks and want to proceed anyway, pass")
	fmt.Fprintln(w, "    `--allow-dirty` (not recommended; verdicts may be wrong).")
	fmt.Fprintln(w, "  - See docs/reconcile.md for the full workflow patterns.")
}

// isTpatchUntracked reports whether the `.tpatch/` directory is not
// tracked in the repo. Used as a non-fatal tip at the end of reconcile.
func isTpatchUntracked(repoRoot string) bool {
	return !gitutil.IsPathTracked(repoRoot, ".tpatch")
}

// ─── --manual / --skip-llm shared helpers ────────────────────────────────────

// addManualFlag installs the --manual flag and the --skip-llm alias on a
// phase command. Both flags are boolean and hidden behind the same state in
// cobra's flag set; tests and callers read --manual.
func addManualFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("manual", false, "Advance feature state from a hand-authored artifact; do not call the provider")
	cmd.Flags().Bool("skip-llm", false, "Alias for --manual")
}

// isManualFlag returns true if either --manual or --skip-llm is set.
func isManualFlag(cmd *cobra.Command) bool {
	m, _ := cmd.Flags().GetBool("manual")
	if m {
		return true
	}
	alias, _ := cmd.Flags().GetBool("skip-llm")
	return alias
}

// runManualPhase validates the expected phase artifact exists and advances
// feature state without invoking the provider. It is the single entry point
// shared by analyze/define/explore/implement when --manual is set.
func runManualPhase(cmd *cobra.Command, s *store.Store, slug, phase string) error {
	if err := s.AdvanceStateManually(slug, phase); err != nil {
		return err
	}
	m, _ := store.ManualPhase(phase)
	fmt.Fprintf(cmd.OutOrStdout(), "Phase %s advanced manually for %s (artifact: %s; state: %s)\n", phase, slug, m.Path, m.State)
	fmt.Fprintln(cmd.OutOrStdout(), "  (manual mode — provider not called)")
	return nil
}

// ─── provider ────────────────────────────────────────────────────────────────

func providerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage LLM provider settings",
	}
	cmd.AddCommand(
		providerCheckCmd(),
		providerSetCmd(),
		providerCopilotLoginCmd(),
		providerCopilotLogoutCmd(),
	)
	return cmd
}

func providerCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Validate provider endpoint",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			_, cfg := loadProviderFromStore(s)
			if !cfg.Configured() {
				return fmt.Errorf("provider is not configured — run 'tpatch config set provider.base_url <url>' and 'tpatch config set provider.model <model>'")
			}
			timeout, _ := cmd.Flags().GetDuration("timeout")
			prov := provider.NewFromConfig(cfg)
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			health, err := prov.Check(ctx, cfg)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Provider OK: %s\n", health.Endpoint)
			fmt.Fprintf(cmd.OutOrStdout(), "Models: %s\n", strings.Join(health.Models, ", "))
			return nil
		},
	}
	cmd.Flags().Duration("timeout", 15*time.Second, "Request timeout")
	return cmd
}

func providerSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Configure provider endpoint (global by default; --repo to override per-repo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoScope, _ := cmd.Flags().GetBool("repo")

			// Load whichever config we are targeting so preset/flag merges
			// layer onto the existing values (same UX as before the global
			// default). Repo mode: require .tpatch. Global mode: load from
			// disk or start empty.
			var (
				cfg    store.Config
				s      *store.Store
				target string
			)
			if repoScope {
				var err error
				s, err = openStoreFromCmd(cmd)
				if err != nil {
					return err
				}
				cfg, err = s.LoadConfig()
				if err != nil {
					return err
				}
				target = "repo (" + s.ConfigPath() + ")"
			} else {
				var err error
				cfg, err = store.LoadGlobalConfig()
				if err != nil {
					return err
				}
				path, err := store.GlobalConfigPath()
				if err != nil {
					return err
				}
				target = "global (" + path + ")"
			}

			if preset, _ := cmd.Flags().GetString("preset"); preset != "" {
				p, ok := providerPresets[strings.ToLower(preset)]
				if !ok {
					return fmt.Errorf("unknown preset %q (valid: copilot, copilot-native, openai, openrouter, anthropic, ollama)", preset)
				}
				cfg.Provider.Type = p.Type
				cfg.Provider.BaseURL = p.BaseURL
				cfg.Provider.Model = p.Model
				cfg.Provider.AuthEnv = p.AuthEnv
			}
			if v, _ := cmd.Flags().GetString("type"); v != "" {
				if v != "openai-compatible" && v != "anthropic" && v != provider.CopilotNativeType {
					return fmt.Errorf("invalid provider type %q (valid: openai-compatible, anthropic, copilot-native)", v)
				}
				cfg.Provider.Type = v
			}
			if v, _ := cmd.Flags().GetString("base-url"); v != "" {
				cfg.Provider.BaseURL = v
			}
			if v, _ := cmd.Flags().GetString("model"); v != "" {
				cfg.Provider.Model = v
			}
			if v, _ := cmd.Flags().GetString("auth-env"); v != "" {
				cfg.Provider.AuthEnv = v
			}
			// Enforce the copilot-native opt-in gate before persisting.
			// Per rubber-duck #2: this is the first of three activation
			// paths (set, auto-detect, config-set); all three must gate.
			if cfg.Provider.Type == provider.CopilotNativeType && !store.CopilotNativeOptedIn() {
				fmt.Fprint(cmd.OutOrStdout(), copilotNativeOptInPrompt())
				return fmt.Errorf("copilot-native requires opt-in; run `tpatch config set provider.copilot_native_optin true`")
			}

			if repoScope {
				if err := s.SaveConfig(cfg); err != nil {
					return err
				}
			} else {
				if err := store.SaveGlobalConfig(cfg); err != nil {
					return err
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Provider configured [%s]: type=%s url=%s model=%s\n",
				target, cfg.Provider.Type, cfg.Provider.BaseURL, cfg.Provider.Model)

			// Show the AUP warning the first time a Copilot-flavoured config
			// is selected (once per user, recorded in global config).
			provCfg := provider.Config{
				Type:      cfg.Provider.Type,
				BaseURL:   cfg.Provider.BaseURL,
				Model:     cfg.Provider.Model,
				AuthEnv:   cfg.Provider.AuthEnv,
				Initiator: cfg.Provider.Initiator,
			}
			maybeShowAUPWarning(cmd.OutOrStdout(), provCfg)
			return nil
		},
	}
	cmd.Flags().String("preset", "", "Preset: copilot | copilot-native | openai | openrouter | anthropic | ollama")
	cmd.Flags().String("type", "", "Provider type: openai-compatible | anthropic | copilot-native")
	cmd.Flags().String("base-url", "", "Provider base URL")
	cmd.Flags().String("model", "", "Default model")
	cmd.Flags().String("auth-env", "", "Environment variable name for auth token")
	cmd.Flags().Bool("repo", false, "Write to the repo-level .tpatch/config.yaml instead of the global config")
	return cmd
}

// providerPresets are vetted one-line configurations for common providers.
// Each preset matches a widely used endpoint that speaks either the
// OpenAI chat completions API or the Anthropic Messages API — the two
// protocols implemented in internal/provider.
//
// Note: the `copilot` preset intentionally leaves AuthEnv empty. The
// local copilot-api proxy (localhost:4141) strips and replaces inbound
// auth headers with its own session token (see ADR-NNN / proxy
// `lib/api-config.ts:copilotHeaders`), so the user's GITHUB_TOKEN is
// never forwarded upstream and tpatch sending it just clutters the
// proxy's request log. Users can still opt in via
// `--auth-env GITHUB_TOKEN`.
var providerPresets = map[string]struct {
	Type, BaseURL, Model, AuthEnv string
}{
	"copilot":        {"openai-compatible", "http://localhost:4141", "claude-sonnet-4", ""},
	"copilot-native": {provider.CopilotNativeType, "", "claude-sonnet-4", ""},
	"openai":         {"openai-compatible", "https://api.openai.com", "gpt-4o", "OPENAI_API_KEY"},
	"openrouter":     {"openai-compatible", "https://openrouter.ai/api", "anthropic/claude-sonnet-4", "OPENROUTER_API_KEY"},
	"anthropic":      {"anthropic", "https://api.anthropic.com", "claude-sonnet-4-5", "ANTHROPIC_API_KEY"},
	"ollama":         {"openai-compatible", "http://localhost:11434", "llama3.2", ""},
}

// ─── config ──────────────────────────────────────────────────────────────────

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}
	cmd.AddCommand(configShowCmd(), configSetCmd())
	return cmd
}

func configShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(s.ConfigPath())
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
}

func configSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]

			// Global-only keys bypass the repo store. Per rubber-duck
			// review #3: opt-in state must persist across repos.
			switch key {
			case "provider.copilot_native_optin":
				if strings.ToLower(value) != "true" {
					return fmt.Errorf("opt-in must be set to `true`; see ADR-005")
				}
				if err := store.AcknowledgeCopilotNativeOptIn(); err != nil {
					return err
				}
				fmt.Fprint(cmd.OutOrStdout(), copilotNativeAUPNotice())
				fmt.Fprintf(cmd.OutOrStdout(), "\nOpt-in recorded in global config.\n")
				return nil
			}

			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}
			switch key {
			case "provider.type":
				if value == provider.CopilotNativeType && !store.CopilotNativeOptedIn() {
					fmt.Fprint(cmd.OutOrStdout(), copilotNativeOptInPrompt())
					return fmt.Errorf("copilot-native requires opt-in; run `tpatch config set provider.copilot_native_optin true`")
				}
				cfg.Provider.Type = value
			case "provider.base_url":
				cfg.Provider.BaseURL = value
			case "provider.model":
				cfg.Provider.Model = value
			case "provider.auth_env":
				cfg.Provider.AuthEnv = value
			case "provider.initiator":
				if value != "" && value != "user" && value != "agent" {
					return fmt.Errorf("provider.initiator must be empty, \"user\", or \"agent\"")
				}
				cfg.Provider.Initiator = value
			case "merge_strategy":
				if value != "3way" && value != "rebase" {
					return fmt.Errorf("invalid merge_strategy %q (valid: 3way, rebase)", value)
				}
				cfg.MergeStrategy = value
			case "max_retries":
				var n int
				if _, err := fmt.Sscanf(value, "%d", &n); err != nil || n < 0 {
					return fmt.Errorf("invalid max_retries %q (must be non-negative integer)", value)
				}
				cfg.MaxRetries = n
			case "prefix_split_min_files":
				n, err := parsePositiveConfigInt(key, value)
				if err != nil {
					return err
				}
				cfg.PathRestructurePrefixSplitMinFiles = n
			case "prefix_split_min_prefixes":
				n, err := parsePositiveConfigInt(key, value)
				if err != nil {
					return err
				}
				cfg.PathRestructurePrefixSplitMinPrefixes = n
			case "prefix_move_min_files":
				n, err := parsePositiveConfigInt(key, value)
				if err != nil {
					return err
				}
				cfg.PathRestructurePrefixMoveMinFiles = n
			case "test_command":
				cfg.TestCommand = value
			default:
				return fmt.Errorf("unknown config key %q (valid: provider.type, provider.base_url, provider.model, provider.auth_env, provider.initiator, provider.copilot_native_optin, merge_strategy, max_retries, prefix_split_min_files, prefix_split_min_prefixes, prefix_move_min_files, test_command)", key)
			}
			if err := s.SaveConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s\n", key, value)
			return nil
		},
	}
}

func parsePositiveConfigInt(key, value string) (int, error) {
	var n int
	if _, err := fmt.Sscanf(value, "%d", &n); err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid %s %q (must be positive integer)", key, value)
	}
	return n, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func installSkills(cmd *cobra.Command, root string) {
	skillInstalls := []struct {
		src, dst, name string
	}{
		{"skills/claude/tessera-patch/SKILL.md", filepath.Join(root, ".claude", "skills", "tessera-patch", "SKILL.md"), "Claude skill"},
		{"skills/copilot/tessera-patch/SKILL.md", filepath.Join(root, ".github", "skills", "tessera-patch", "SKILL.md"), "Copilot skill"},
		{"prompts/copilot/tessera-patch-apply.prompt.md", filepath.Join(root, ".github", "prompts", "tessera-patch-apply.prompt.md"), "Copilot prompt"},
		{"skills/cursor/tessera-patch.mdc", filepath.Join(root, ".cursor", "rules", "tessera-patch.mdc"), "Cursor rules"},
		{"skills/windsurf/windsurfrules", filepath.Join(root, ".windsurfrules"), "Windsurf rules"},
		{"workflows/tessera-patch-generic.md", filepath.Join(root, ".tpatch", "workflows", "tessera-patch-generic.md"), "Generic workflow"},
	}
	for _, si := range skillInstalls {
		data, err := assets.Skills.ReadFile(si.src)
		if err != nil {
			continue
		}
		os.MkdirAll(filepath.Dir(si.dst), 0o755)
		if err := os.WriteFile(si.dst, data, 0o644); err != nil {
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  Installed %s\n", si.name)
	}
}

func resolveRoot(cmd *cobra.Command, args []string) (string, error) {
	pathFlag, _ := cmd.Flags().GetString("path")
	if pathFlag != "" {
		return filepath.Abs(pathFlag)
	}
	if len(args) > 0 {
		return filepath.Abs(args[0])
	}
	return filepath.Abs(".")
}

func openStoreFromCmd(cmd *cobra.Command) (*store.Store, error) {
	pathFlag, _ := cmd.Flags().GetString("path")
	start := pathFlag
	if start == "" {
		start = "."
	}
	root, err := store.FindProjectRoot(start)
	if err != nil {
		return nil, err
	}
	return store.Open(root)
}

// probedEndpoints tracks base URLs already probed this process so the
// reachability check only runs once per run, not per workflow phase.
// Stores both the probe error and the resolved health so PickProvider
// can use the cached metadata across phases.
type probedResult struct {
	health *provider.Health
	err    error
}

var (
	probedEndpoints   = map[string]probedResult{}
	probedEndpointsMu sync.Mutex
)

func loadProviderFromStore(s *store.Store) (provider.Provider, provider.Config) {
	cfg, err := s.LoadMergedConfig()
	if err != nil {
		return nil, provider.Config{}
	}
	provCfg := provider.Config{
		Type:      cfg.Provider.Type,
		BaseURL:   cfg.Provider.BaseURL,
		Model:     cfg.Provider.Model,
		AuthEnv:   cfg.Provider.AuthEnv,
		Initiator: cfg.Provider.Initiator,
	}
	if !provCfg.Configured() {
		return nil, provCfg
	}
	return provider.NewFromConfig(provCfg), provCfg
}

// loadAndProbeProvider is loadProviderFromStore + a one-time reachability
// probe for local endpoints (cached per-process). Workflow commands use
// this to hard-fail with an install hint when a local proxy is expected
// but not running. Returns (nil, cfg, nil) if the provider is not
// configured (heuristic fallback path is preserved).
//
// When probing succeeds and the cached *provider.Health carries
// per-model `supported_endpoints` (currently the copilot-api proxy at
// localhost:4141), the returned Provider is selected via
// provider.PickProvider — so e.g. Claude models on the proxy
// transparently use the Anthropic /v1/messages provider, dodging the
// proxy's missing chat-completions->messages routing branch.
func loadAndProbeProvider(ctx context.Context, s *store.Store) (provider.Provider, provider.Config, error) {
	prov, cfg := loadProviderFromStore(s)
	if prov == nil || !provider.IsLocalEndpoint(cfg) || os.Getenv("TPATCH_NO_PROBE") != "" {
		return prov, cfg, nil
	}
	probedEndpointsMu.Lock()
	cached, seen := probedEndpoints[cfg.BaseURL]
	probedEndpointsMu.Unlock()
	if !seen {
		health, err := ensureProviderReachable(ctx, cfg)
		cached = probedResult{health: health, err: err}
		probedEndpointsMu.Lock()
		probedEndpoints[cfg.BaseURL] = cached
		probedEndpointsMu.Unlock()
	}
	if cached.err != nil {
		return nil, cfg, cached.err
	}
	prov = provider.PickProvider(cfg, cached.health)
	return prov, cfg, nil
}

// autoDetectProvider probes known provider endpoints and auto-configures if found.
func autoDetectProvider(cmd *cobra.Command, s *store.Store) {
	// Skip auto-detection in test environments
	if os.Getenv("TPATCH_NO_AUTO_DETECT") != "" {
		return
	}

	cfg, _ := s.LoadConfig()
	if cfg.Provider.Configured() {
		return // already configured
	}

	type candidate struct {
		name   string
		preset struct{ Type, BaseURL, Model, AuthEnv string }
	}

	candidates := []candidate{
		{"copilot-api (localhost:4141)", providerPresets["copilot"]},
		{"Ollama (localhost:11434)", providerPresets["ollama"]},
	}

	// Also check env vars for direct API keys
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		candidates = append(candidates, candidate{"Anthropic (from ANTHROPIC_API_KEY)", providerPresets["anthropic"]})
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		candidates = append(candidates, candidate{"OpenAI (from OPENAI_API_KEY)", providerPresets["openai"]})
	}
	if os.Getenv("OPENROUTER_API_KEY") != "" {
		candidates = append(candidates, candidate{"OpenRouter (from OPENROUTER_API_KEY)", providerPresets["openrouter"]})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for _, c := range candidates {
		provCfg := provider.Config{Type: c.preset.Type, BaseURL: c.preset.BaseURL, Model: c.preset.Model, AuthEnv: c.preset.AuthEnv}
		prov := provider.NewFromConfig(provCfg)
		if _, err := prov.Check(ctx, provCfg); err == nil {
			cfg.Provider.Type = c.preset.Type
			cfg.Provider.BaseURL = c.preset.BaseURL
			cfg.Provider.Model = c.preset.Model
			cfg.Provider.AuthEnv = c.preset.AuthEnv
			s.SaveConfig(cfg)
			fmt.Fprintf(cmd.OutOrStdout(), "  Auto-detected provider: %s\n", c.name)
			maybeShowAUPWarning(cmd.OutOrStdout(), provCfg)
			return
		}
	}
}
