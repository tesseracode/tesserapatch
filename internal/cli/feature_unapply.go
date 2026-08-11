package cli

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
)

const featureUnapplyApplyHint = "Use 'tpatch apply <slug>' to reapply a feature that has been unapplied."

type unapplyOptions struct {
	DryRun              bool
	AllowSoftDependents bool
	Actor               string
	Mode                string
}

type unapplyPreflight struct {
	CleanTree       bool     `json:"clean_tree"`
	ConflictMarkers []string `json:"conflict_markers"`
	Leftovers       []string `json:"leftovers"`
}

// unapplySession is the binding ADR-032 D3 wire schema. Field order is
// load-bearing because encoding/json emits struct fields in declaration order.
type unapplySession struct {
	Version              int                `json:"version"`
	Feature              string             `json:"feature"`
	AttemptID            string             `json:"attempt_id"`
	AttemptedAt          string             `json:"attempted_at"`
	Mode                 string             `json:"mode"`
	Actor                string             `json:"actor"`
	PreviousState        store.FeatureState `json:"previous_state"`
	Result               string             `json:"result"`
	CanonicalPatchSHA256 string             `json:"canonical_patch_sha256"`
	ReversePatch         string             `json:"reverse_patch"`
	TouchedPaths         []string           `json:"touched_paths"`
	DependencyBlockers   []string           `json:"dependency_blockers"`
	Preflight            unapplyPreflight   `json:"preflight"`
}

type unapplyRuntime struct {
	now             func() time.Time
	newAttemptID    func() (string, error)
	validateReverse func(string, string) error
	previewReverse  func(string, string) error
	snapshot        func(string, []string) (gitutil.WorktreeSnapshot, error)
	reverseApply    func(string, string) error
	captureReverse  func(string, []string) (string, error)
	writeArtifact   func(*store.Store, string, string, string) error
	saveStatus      func(*store.Store, store.FeatureStatus) error
	removeAttempt   func(string) error
}

func defaultUnapplyRuntime() unapplyRuntime {
	return unapplyRuntime{
		now:             func() time.Time { return time.Now().UTC().Truncate(time.Second) },
		newAttemptID:    newUnapplyAttemptID,
		validateReverse: gitutil.ValidatePatchReverse,
		previewReverse:  gitutil.PreviewReverseApply,
		snapshot:        gitutil.SnapshotWorktreePaths,
		reverseApply:    gitutil.ReverseApply,
		captureReverse: func(root string, paths []string) (string, error) {
			return gitutil.CapturePatchScoped(root, literalGitPathspecs(paths))
		},
		writeArtifact: func(s *store.Store, slug, name, content string) error {
			return s.WriteArtifact(slug, name, content)
		},
		saveStatus: func(s *store.Store, status store.FeatureStatus) error {
			return s.SaveFeatureStatus(status)
		},
		removeAttempt: os.RemoveAll,
	}
}

func featureUnapplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unapply <slug>",
		Short: "Remove a feature's canonical patch from the working tree",
		Long: `Remove a feature's canonical patch from the working tree while preserving
its metadata, canonical patch, patch generations, and audit history.

V1 uses strict patch mode only and refuses when dependency or worktree safety
cannot be proven. --dry-run reports every blocker without mutating the
worktree, index, or .tpatch metadata.

Use 'tpatch apply <slug>' to reapply a feature that has been unapplied.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := unapplyOptions{}
			opts.DryRun, _ = cmd.Flags().GetBool("dry-run")
			opts.AllowSoftDependents, _ = cmd.Flags().GetBool("allow-soft-dependents")
			opts.Actor, _ = cmd.Flags().GetString("actor")
			opts.Mode, _ = cmd.Flags().GetString("mode")
			return runFeatureUnapply(cmd, s, args[0], opts)
		},
	}
	cmd.Flags().Bool("dry-run", false, "Report blockers and planned artifacts without mutation")
	cmd.Flags().Bool("allow-soft-dependents", false, "Allow unapply when only soft dependents exist")
	cmd.Flags().String("actor", "", "Actor recorded in unapply-session.json")
	cmd.Flags().String("mode", "patch", "Unapply mode (v1 supports patch only)")
	return cmd
}

func runFeatureUnapply(cmd *cobra.Command, s *store.Store, slug string, opts unapplyOptions) error {
	return runFeatureUnapplyWithRuntime(cmd, s, slug, opts, defaultUnapplyRuntime())
}

func runFeatureUnapplyWithRuntime(cmd *cobra.Command, s *store.Store, slug string, opts unapplyOptions, runtime unapplyRuntime) error {
	if slug == "" || store.Slugify(slug) != slug {
		return validationError("invalid feature slug %q", slug)
	}
	if opts.Mode != "patch" {
		if opts.Mode == "landed-commit" {
			return validationError("unapply mode %q is reserved for a future release; v1 supports --mode patch only", opts.Mode)
		}
		return validationError("unknown unapply mode %q (valid: patch)", opts.Mode)
	}
	if err := validateUnapplyActor(opts.Actor); err != nil {
		return validationError("%v", err)
	}

	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return validationError("feature %q not found or status.json is unreadable: %v", slug, err)
	}
	if !opts.DryRun && !unapplySourceStateAllowed(status.State) {
		return stateRefusalError("cannot unapply feature %q from state %q: unapply is only valid from applied, active, reconciling, or reconciling-shadow", slug, status.State)
	}
	patch, err := s.ReadFeatureFile(slug, "artifacts/post-apply.patch")
	if err != nil {
		return validationError("feature %q canonical patch is missing or unreadable: %v", slug, err)
	}
	if strings.TrimSpace(patch) == "" {
		return validationError("feature %q canonical patch is empty", slug)
	}

	touched := gitutil.PathsAffectedByPatch(patch)
	sort.Strings(touched)
	if len(touched) == 0 {
		return validationError("feature %q canonical patch names no touched files", slug)
	}
	if err := gitutil.ValidateWorktreePaths(s.Root, touched); err != nil {
		return validationError("feature %q canonical patch contains an unsafe path: %v", slug, err)
	}

	attemptID, err := runtime.newAttemptID()
	if err != nil {
		return fmt.Errorf("generate unapply attempt ID: %w", err)
	}
	attemptedAt := runtime.now()
	actor := store.ResolveActorIn(opts.Actor, s.Root)
	patchHash := sha256.Sum256([]byte(patch))
	reverseRel := filepath.ToSlash(filepath.Join("artifacts", "unapply", attemptID, "reverse.patch"))
	sessionRel := filepath.ToSlash(filepath.Join("artifacts", "unapply", attemptID, "unapply-session.json"))
	attemptDir := filepath.Join(s.TpatchDir(), "features", slug, "artifacts", "unapply", attemptID)

	preflight, err := gitutil.PreflightReconcile(s.Root)
	if err != nil {
		return fmt.Errorf("unapply preflight: %w", err)
	}
	operation, err := gitutil.GitOperationInProgress(s.Root)
	if err != nil {
		return fmt.Errorf("unapply preflight: %w", err)
	}

	dependents := dependentEdges(s, slug)
	dependencyBlockers := make([]string, 0)
	blockers := make([]string, 0)
	if !unapplySourceStateAllowed(status.State) {
		blockers = append(blockers, fmt.Sprintf("state %q is not unapply-eligible", status.State))
	}
	for _, dep := range dependents {
		if dependent, loadErr := s.LoadFeatureStatus(dep.slug); loadErr == nil {
			if dependent.State == store.StateRejected || dependent.State == store.StateUpstreamMerged {
				continue
			}
		}
		switch dep.kind {
		case store.DependencyKindSoft:
			if !opts.AllowSoftDependents {
				dependencyBlockers = append(dependencyBlockers, dep.slug)
				blockers = append(blockers, fmt.Sprintf("soft dependent %s (use --allow-soft-dependents to acknowledge)", dep.slug))
			}
		case store.DependencyKindHard:
			dependencyBlockers = append(dependencyBlockers, dep.slug)
			blockers = append(blockers, fmt.Sprintf("hard dependent %s", dep.slug))
		case store.DependencyKindSupersedes:
			dependencyBlockers = append(dependencyBlockers, dep.slug)
			blockers = append(blockers, fmt.Sprintf("supersedes dependent %s", dep.slug))
		default:
			dependencyBlockers = append(dependencyBlockers, dep.slug)
			blockers = append(blockers, fmt.Sprintf("dependent %s has unsupported kind %q", dep.slug, dep.kind))
		}
	}
	if len(preflight.UnstagedFiles) > 0 || len(preflight.UntrackedFiles) > 0 {
		blockers = append(blockers, "working tree is dirty; commit or clean it before unapply")
	}
	if len(preflight.MergeMarkerFiles) > 0 {
		blockers = append(blockers, "tracked files contain merge conflict markers")
	}
	if len(preflight.LeftoverFiles) > 0 {
		blockers = append(blockers, "working tree contains .orig/.rej leftovers")
	}
	if operation != "" {
		blockers = append(blockers, "repository is mid-"+operation)
	}

	reverseErr := runtime.validateReverse(s.Root, patch)
	if reverseErr != nil {
		blockers = append(blockers, "canonical patch does not reverse-apply cleanly: "+reverseErr.Error())
	}
	previewErr := runtime.previewReverse(s.Root, patch)
	if previewErr != nil {
		blockers = append(blockers, "temporary-worktree reverse preview failed: "+previewErr.Error())
	}

	reportPreflight := unapplyPreflight{
		CleanTree:       len(preflight.UnstagedFiles) == 0 && len(preflight.UntrackedFiles) == 0,
		ConflictMarkers: nonNilStrings(preflight.MergeMarkerFiles),
		Leftovers:       nonNilStrings(preflight.LeftoverFiles),
	}
	if opts.DryRun {
		renderUnapplyDryRun(cmd, status, patchHash, touched, dependents, dependencyBlockers, reportPreflight, operation, reverseErr, previewErr, sessionRel, reverseRel, blockers)
		return nil
	}
	if len(blockers) > 0 {
		return stateRefusalError("cannot unapply feature %q: %s", slug, strings.Join(blockers, "; "))
	}

	snapshot, err := runtime.snapshot(s.Root, touched)
	if err != nil {
		return fmt.Errorf("cannot snapshot feature %q touched paths: %w", slug, err)
	}
	if err := runtime.reverseApply(s.Root, patch); err != nil {
		return rollbackUnapply(s.Root, snapshot, attemptDir, runtime.removeAttempt,
			fmt.Errorf("reverse apply failed after successful checks: %w", err))
	}

	reversePatch, err := runtime.captureReverse(s.Root, touched)
	if err != nil {
		return rollbackUnapply(s.Root, snapshot, attemptDir, runtime.removeAttempt,
			fmt.Errorf("capture reverse.patch: %w", err))
	}
	if strings.TrimSpace(reversePatch) == "" {
		return rollbackUnapply(s.Root, snapshot, attemptDir, runtime.removeAttempt,
			fmt.Errorf("capture reverse.patch: reverse apply produced an empty working-tree diff"))
	}

	session := unapplySession{
		Version:              1,
		Feature:              slug,
		AttemptID:            attemptID,
		AttemptedAt:          attemptedAt.Format(time.RFC3339),
		Mode:                 "patch",
		Actor:                actor,
		PreviousState:        status.State,
		Result:               string(store.StateUnapplied),
		CanonicalPatchSHA256: hex.EncodeToString(patchHash[:]),
		ReversePatch:         reverseRel,
		TouchedPaths:         nonNilStrings(touched),
		DependencyBlockers:   []string{},
		Preflight:            reportPreflight,
	}
	sessionJSON, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return rollbackUnapply(s.Root, snapshot, attemptDir, runtime.removeAttempt,
			fmt.Errorf("marshal unapply-session.json: %w", err))
	}
	sessionJSON = append(sessionJSON, '\n')

	reverseName := filepath.ToSlash(filepath.Join("unapply", attemptID, "reverse.patch"))
	sessionName := filepath.ToSlash(filepath.Join("unapply", attemptID, "unapply-session.json"))
	if err := runtime.writeArtifact(s, slug, reverseName, reversePatch); err != nil {
		return rollbackUnapply(s.Root, snapshot, attemptDir, runtime.removeAttempt,
			fmt.Errorf("write reverse.patch: %w", err))
	}
	if err := runtime.writeArtifact(s, slug, sessionName, string(sessionJSON)); err != nil {
		return rollbackUnapply(s.Root, snapshot, attemptDir, runtime.removeAttempt,
			fmt.Errorf("write unapply-session.json: %w", err))
	}

	status.State = store.StateUnapplied
	status.LastCommand = "feature unapply"
	status.UpdatedAt = attemptedAt.Format(time.RFC3339)
	status.Notes = "Patch removed from working tree; audit: " + sessionRel
	status.Verify = nil
	if err := runtime.saveStatus(s, status); err != nil {
		return rollbackUnapply(s.Root, snapshot, attemptDir, runtime.removeAttempt,
			fmt.Errorf("save unapplied status: %w", err))
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Unapplied %s; state %s → %s\n", slug, session.PreviousState, store.StateUnapplied)
	fmt.Fprintf(cmd.OutOrStdout(), "  audit: %s\n", sessionRel)
	fmt.Fprintf(cmd.OutOrStdout(), "  reverse patch: %s\n", reverseRel)
	fmt.Fprintf(cmd.OutOrStdout(), "Reapply with: tpatch apply %s\n", slug)
	return nil
}

func renderUnapplyDryRun(
	cmd *cobra.Command,
	status store.FeatureStatus,
	patchHash [sha256.Size]byte,
	touched []string,
	dependents []dependentEdge,
	dependencyBlockers []string,
	preflight unapplyPreflight,
	operation string,
	reverseErr, previewErr error,
	sessionRel, reverseRel string,
	blockers []string,
) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Dry-run: tpatch feature unapply %s\n", status.Slug)
	fmt.Fprintf(out, "  state: %s\n", status.State)
	fmt.Fprintf(out, "  canonical_patch_sha256: %s\n", hex.EncodeToString(patchHash[:]))
	fmt.Fprintln(out, "  touched_paths:")
	for _, path := range touched {
		fmt.Fprintf(out, "    - %s\n", path)
	}
	if len(dependents) == 0 {
		fmt.Fprintln(out, "  dependents: []")
	} else {
		fmt.Fprintln(out, "  dependents:")
		for _, dep := range dependents {
			fmt.Fprintf(out, "    - %s (%s)\n", dep.slug, dep.kind)
		}
	}
	fmt.Fprintf(out, "  dependency_blockers: %v\n", dependencyBlockers)
	fmt.Fprintf(out, "  clean_tree: %t\n", preflight.CleanTree)
	fmt.Fprintf(out, "  conflict_markers: %v\n", preflight.ConflictMarkers)
	fmt.Fprintf(out, "  leftovers: %v\n", preflight.Leftovers)
	if operation == "" {
		fmt.Fprintln(out, "  git_operation: none")
	} else {
		fmt.Fprintf(out, "  git_operation: %s\n", operation)
	}
	fmt.Fprintf(out, "  reverse_apply_check: %s\n", viability(reverseErr))
	fmt.Fprintf(out, "  worktree_preview: %s\n", viability(previewErr))
	fmt.Fprintln(out, "  planned_artifacts:")
	fmt.Fprintf(out, "    - %s\n", sessionRel)
	fmt.Fprintf(out, "    - %s\n", reverseRel)
	if len(blockers) == 0 {
		fmt.Fprintln(out, "  blockers: []")
	} else {
		fmt.Fprintln(out, "  blockers:")
		for _, blocker := range blockers {
			fmt.Fprintf(out, "    - %s\n", blocker)
		}
	}
}

func rollbackUnapply(repoRoot string, snapshot gitutil.WorktreeSnapshot, attemptDir string, removeAttempt func(string) error, primary error) error {
	restoreErr := snapshot.Restore(repoRoot)
	removeErr := removeAttempt(attemptDir)
	if rollbackErr := errors.Join(restoreErr, removeErr); rollbackErr != nil {
		return fmt.Errorf("%w; rollback also failed: %v", primary, rollbackErr)
	}
	return primary
}

func unapplySourceStateAllowed(state store.FeatureState) bool {
	switch state {
	case store.StateApplied, store.StateActive, store.StateReconciling, store.StateReconcilingShadow:
		return true
	default:
		return false
	}
}

func validateUnapplyActor(actor string) error {
	for _, r := range actor {
		if unicode.IsControl(r) {
			return fmt.Errorf("actor must not contain control characters")
		}
	}
	return nil
}

func newUnapplyAttemptID() (string, error) {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "ua_" + hex.EncodeToString(raw), nil
}

func nonNilStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

func viability(err error) string {
	if err == nil {
		return "viable"
	}
	return "blocked: " + err.Error()
}

func literalGitPathspecs(paths []string) []string {
	literal := make([]string, len(paths))
	for i, path := range paths {
		literal[i] = ":(literal)" + path
	}
	return literal
}

func refuseIfUnappliedBaselinePending(s *store.Store, status store.FeatureStatus, verb string) error {
	pending, err := unappliedBaselinePending(s, status)
	if err != nil {
		return fmt.Errorf("%s: cannot inspect feature %q unapply baseline: %w", verb, status.Slug, err)
	}
	if !pending {
		return nil
	}
	return stateRefusalError(
		"cannot %s feature %q while HEAD still contains its canonical patch and the worktree carries the unapply reversal; commit the unapplied baseline first or run `tpatch apply %s`",
		verb, status.Slug, status.Slug)
}

func unappliedBaselinePending(s *store.Store, status store.FeatureStatus) (bool, error) {
	if status.State != store.StateUnapplied &&
		status.LastCommand != "feature unapply" &&
		!strings.Contains(status.Notes, "artifacts/unapply/") {
		return false, nil
	}
	canonical, err := s.ReadFeatureFile(status.Slug, filepath.Join("artifacts", "post-apply.patch"))
	if err != nil {
		if status.State == store.StateUnapplied {
			return false, fmt.Errorf("read canonical patch: %w", err)
		}
		return false, nil
	}
	presentAtHEAD, err := gitutil.ReverseApplyCheckAtHEAD(s.Root, canonical)
	if err != nil {
		return false, err
	}
	if !presentAtHEAD {
		return false, nil
	}
	return gitutil.ValidatePatchReverse(s.Root, canonical) != nil, nil
}

func refuseIfUnappliedState(s *store.Store, slug, verb string) error {
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return err
	}
	if status.State != store.StateUnapplied {
		return nil
	}
	return stateRefusalError("cannot %s feature %q while it is unapplied; run `tpatch apply %s` first", verb, slug, slug)
}
