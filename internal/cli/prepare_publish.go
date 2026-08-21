package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/intent"
	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/intentpub"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/redact"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

const (
	prepareDisclaimer  = "Structural presence only. This report does not certify semantic quality."
	preparePlanOnly    = "Plan only. Generation was not attempted and may still fail. Execution preflight was not evaluated: the actual mutation can still refuse on platform, filesystem, Git, lock or recovery grounds."
	prepareRetryHeader = "Run this again from the same workspace root:"
)

type prepareMode string

const (
	prepareModeCheck      prepareMode = "check"
	prepareModeGenerate   prepareMode = "generate"
	prepareModeManual     prepareMode = "manual"
	prepareModeRegenerate prepareMode = "regenerate"
	prepareModeAbandon    prepareMode = "abandon"
)

type prepareOptions struct {
	mode             prepareMode
	allowHeuristic   bool
	dryRun           bool
	yes              bool
	asJSON           bool
	quiet            bool
	timeout          time.Duration
	timeoutPhase     time.Duration
	noRetry          bool
	timeoutChanged   bool
	phaseChanged     bool
	noRetryChanged   bool
	allowHeurChanged bool
}

func readPrepareOptions(cmd *cobra.Command) prepareOptions {
	check, _ := cmd.Flags().GetBool("check")
	manual, _ := cmd.Flags().GetBool("manual")
	regenerate, _ := cmd.Flags().GetBool("regenerate")
	abandon, _ := cmd.Flags().GetBool("abandon-transaction")
	mode := prepareModeGenerate
	switch {
	case check:
		mode = prepareModeCheck
	case manual:
		mode = prepareModeManual
	case regenerate:
		mode = prepareModeRegenerate
	case abandon:
		mode = prepareModeAbandon
	}
	allowHeuristic, _ := cmd.Flags().GetBool("allow-heuristic")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	yes, _ := cmd.Flags().GetBool("yes")
	asJSON, _ := cmd.Flags().GetBool("json")
	quiet, _ := cmd.Flags().GetBool("quiet")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	timeoutPhase, _ := cmd.Flags().GetDuration("timeout-phase")
	noRetry, _ := cmd.Flags().GetBool("no-retry")
	return prepareOptions{
		mode:             mode,
		allowHeuristic:   allowHeuristic,
		dryRun:           dryRun,
		yes:              yes,
		asJSON:           asJSON,
		quiet:            quiet,
		timeout:          timeout,
		timeoutPhase:     timeoutPhase,
		noRetry:          noRetry,
		timeoutChanged:   cmd.Flags().Changed("timeout"),
		phaseChanged:     cmd.Flags().Changed("timeout-phase"),
		noRetryChanged:   cmd.Flags().Changed("no-retry"),
		allowHeurChanged: cmd.Flags().Changed("allow-heuristic"),
	}
}

type prepareArtifactReport struct {
	ID           string `json:"id"`
	Path         string `json:"path"`
	Role         string `json:"role"`
	Disposition  string `json:"disposition"`
	Generator    string `json:"generator"`
	ArchivedBlob string `json:"archived_blob"`
}

type prepareActionReport struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Action string `json:"action"`
}

type prepareArchiveReport struct {
	GenerationID string `json:"generation_id"`
	BlobsDir     string `json:"blobs_dir"`
}

type prepareAdvisoryReport struct {
	Code       string `json:"code"`
	ArtifactID string `json:"artifact_id"`
	Message    string `json:"message"`
}

type prepareRefusalReport struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Remediation string `json:"remediation"`
	Retry       string `json:"retry,omitempty"`
	RetryCWD    string `json:"retry_cwd,omitempty"`
}

type prepareRecoveryReport struct {
	Kind            string   `json:"kind"`
	RestoredEntries []string `json:"restored_entries"`
	FinalizedHashes []string `json:"finalized_hashes"`
	Retry           string   `json:"retry"`
	RetryCWD        string   `json:"retry_cwd"`
}

type prepareAbandonedReport struct {
	Directory     string   `json:"directory,omitempty"`
	Moved         []string `json:"moved,omitempty"`
	Existing      []string `json:"existing,omitempty"`
	RemoveCommand string   `json:"remove_command,omitempty"`
}

type preparePurgeProgressReport struct {
	CompletedHashes []string `json:"completed_hashes"`
	PendingHash     string   `json:"pending_hash,omitempty"`
	RemainingHashes []string `json:"remaining_hashes"`
	Resume          string   `json:"resume"`
	Retry           string   `json:"retry"`
	RetryCWD        string   `json:"retry_cwd"`
	State           string   `json:"state"`
}

// preparePublishReport is the fixed-order version-one mutating wire schema.
// Optional objects are emitted only on the outcomes that own them.
type preparePublishReport struct {
	SchemaVersion      int                         `json:"schema_version"`
	Command            string                      `json:"command"`
	Mode               prepareMode                 `json:"mode"`
	DryRun             bool                        `json:"dry_run"`
	Slug               string                      `json:"slug"`
	Outcome            string                      `json:"outcome"`
	Action             string                      `json:"action"`
	FeatureState       string                      `json:"feature_state"`
	Disclaimer         string                      `json:"disclaimer"`
	ExecutionPreflight string                      `json:"execution_preflight,omitempty"`
	PlanNote           string                      `json:"plan_note,omitempty"`
	Actions            *[]prepareActionReport      `json:"actions,omitempty"`
	Artifacts          []prepareArtifactReport     `json:"artifacts"`
	Archive            *prepareArchiveReport       `json:"archive,omitempty"`
	OrphanBlobs        []string                    `json:"orphan_blobs"`
	Advisories         []prepareAdvisoryReport     `json:"advisories"`
	Refusal            *prepareRefusalReport       `json:"refusal,omitempty"`
	Recovery           *prepareRecoveryReport      `json:"recovery,omitempty"`
	Abandoned          *prepareAbandonedReport     `json:"abandoned,omitempty"`
	PurgeProgress      *preparePurgeProgressReport `json:"purge_progress,omitempty"`
	allowHeuristic     bool
}

func newPreparePublishReport(mode prepareMode, slug, state string) preparePublishReport {
	if state == "" {
		state = intent.FeatureStateUnknown
	}
	return preparePublishReport{
		SchemaVersion: 1,
		Command:       "prepare",
		Mode:          mode,
		Slug:          slug,
		Outcome:       "refused",
		Action:        "none",
		FeatureState:  state,
		Disclaimer:    prepareDisclaimer,
		Artifacts:     []prepareArtifactReport{},
		OrphanBlobs:   []string{},
		Advisories:    []prepareAdvisoryReport{},
	}
}

func applyPrepareOptionFields(report preparePublishReport, options prepareOptions) preparePublishReport {
	report.allowHeuristic = options.allowHeuristic
	if options.dryRun {
		report.DryRun = true
		report.ExecutionPreflight = "not_evaluated"
		report.PlanNote = preparePlanOnly
	}
	return report
}

func emitPreparePublishReport(cmd *cobra.Command, report preparePublishReport, exitCode int) error {
	if err := writePreparePublishReport(cmd, report); err != nil {
		return err
	}
	if exitCode == 0 {
		return nil
	}
	message := fmt.Sprintf("prepare %s: %s %s", report.Slug, report.Mode, report.Outcome)
	if report.Refusal != nil {
		message = fmt.Sprintf("prepare %s: %s refused %s", report.Slug, report.Mode, report.Refusal.Code)
	}
	if report.Slug == "" {
		message = fmt.Sprintf("prepare: %s refused %s", report.Mode, report.Refusal.Code)
	}
	return &ExitCodeError{Code: exitCode, Message: message}
}

func writePreparePublishReport(cmd *cobra.Command, report preparePublishReport) error {
	if report.Artifacts == nil {
		report.Artifacts = []prepareArtifactReport{}
	}
	if report.OrphanBlobs == nil {
		report.OrphanBlobs = []string{}
	}
	if report.Advisories == nil {
		report.Advisories = []prepareAdvisoryReport{}
	}
	if report.Actions != nil && *report.Actions == nil {
		empty := []prepareActionReport{}
		report.Actions = &empty
	}
	if report.Recovery != nil {
		if report.Recovery.RestoredEntries == nil {
			report.Recovery.RestoredEntries = []string{}
		}
		if report.Recovery.FinalizedHashes == nil {
			report.Recovery.FinalizedHashes = []string{}
		}
	}

	asJSON, _ := cmd.Flags().GetBool("json")
	quiet, _ := cmd.Flags().GetBool("quiet")
	if asJSON {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return err
		}
		if !quiet {
			writePreparePublishHuman(cmd.ErrOrStderr(), report)
		}
		return nil
	}
	if quiet {
		writePreparePublishQuiet(cmd.OutOrStdout(), report)
		return nil
	}
	writePreparePublishHuman(cmd.OutOrStdout(), report)
	return nil
}

func writePreparePublishQuiet(w io.Writer, report preparePublishReport) {
	detail := ""
	switch {
	case report.Refusal != nil:
		detail = " " + report.Refusal.Code
	case report.Outcome == "published":
		changed, archived := 0, 0
		for _, artifact := range report.Artifacts {
			if artifact.Disposition == "generated" || artifact.Disposition == "regenerated" {
				changed++
			}
			if artifact.ArchivedBlob != "" {
				archived++
			}
		}
		detail = fmt.Sprintf(" (%d artifacts, %d archived)", changed, archived)
	case report.Outcome == "abandoned" || report.Outcome == "abandon-planned":
		if report.Abandoned != nil {
			detail = fmt.Sprintf(" (%d control files moved)", len(report.Abandoned.Moved))
		}
	case report.Outcome == "recovered":
		if report.Recovery != nil {
			detail = fmt.Sprintf(" (%d entries restored)", len(report.Recovery.RestoredEntries))
		}
	}
	fmt.Fprintf(w, "prepare %s: %s %s%s\n", report.Slug, report.Mode, report.Outcome, detail)
}

func writePreparePublishHuman(w io.Writer, report preparePublishReport) {
	if report.Slug == "" {
		fmt.Fprintf(w, "Feature: (slug withheld)\n")
	} else {
		fmt.Fprintf(w, "Feature: %s   (state: %s)\n", report.Slug, report.FeatureState)
	}
	fmt.Fprintf(w, "Mode:    %s", report.Mode)
	if report.allowHeuristic {
		fmt.Fprint(w, " --allow-heuristic")
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w)

	if report.Recovery != nil {
		fmt.Fprintln(w, "Recovered an interrupted prepare transaction. Nothing else was done.")
		for _, entry := range report.Recovery.RestoredEntries {
			fmt.Fprintf(w, "  restored  %s\n", prepareDisplayPath(entry))
		}
		for _, hash := range report.Recovery.FinalizedHashes {
			fmt.Fprintf(w, "  finalized %s\n", hash)
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w, prepareRetryHeader)
		fmt.Fprintf(w, "  %s\n\n", report.Recovery.Retry)
	} else if report.Abandoned != nil {
		switch report.Outcome {
		case "abandoned", "abandon-planned":
			fmt.Fprintf(w, "Transaction evidence: %s\n", report.Abandoned.Directory)
			for _, moved := range report.Abandoned.Moved {
				fmt.Fprintf(w, "  %s\n", moved)
			}
			fmt.Fprintln(w, "No canonical file changed.")
			if report.Abandoned.RemoveCommand != "" {
				fmt.Fprintf(w, "Remove it when no longer needed: %s\n", report.Abandoned.RemoveCommand)
			}
			fmt.Fprintln(w)
		default:
			for _, existing := range report.Abandoned.Existing {
				fmt.Fprintf(w, "Previously abandoned evidence preserved: %s\n", existing)
			}
			fmt.Fprintln(w)
		}
	} else {
		for _, artifact := range report.Artifacts {
			fmt.Fprintf(w, "  %-30s %-13s", prepareDisplayPath(artifact.Path), artifact.Disposition)
			if artifact.Generator != "" {
				fmt.Fprintf(w, " (%s)", artifact.Generator)
			}
			if artifact.ArchivedBlob != "" {
				fmt.Fprintf(w, " archived %s", artifact.ArchivedBlob)
			}
			fmt.Fprintln(w)
		}
		if len(report.Artifacts) > 0 {
			fmt.Fprintln(w)
		}
		if report.Actions != nil {
			fmt.Fprintln(w, "Planned actions:")
			for _, action := range *report.Actions {
				fmt.Fprintf(w, "  %-8s %s\n", action.Action, action.Path)
			}
			fmt.Fprintln(w)
		}
	}

	if len(report.OrphanBlobs) != 0 {
		fmt.Fprintln(w, "Orphan archive blobs:")
		for _, hash := range report.OrphanBlobs {
			fmt.Fprintf(w, "  %s\n", hash)
		}
		fmt.Fprintf(w, "Remove them with: tpatch feature intent-archive purge %s --orphans --yes\n\n", report.Slug)
	}
	if report.PurgeProgress != nil {
		fmt.Fprintf(w, "Purge state: %s\n", report.PurgeProgress.State)
		for _, hash := range report.PurgeProgress.CompletedHashes {
			fmt.Fprintf(w, "  completed %s\n", hash)
		}
		if report.PurgeProgress.PendingHash != "" {
			fmt.Fprintf(w, "  pending   %s\n", report.PurgeProgress.PendingHash)
		}
		for _, hash := range report.PurgeProgress.RemainingHashes {
			fmt.Fprintf(w, "  remaining %s\n", hash)
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w, prepareRetryHeader)
		fmt.Fprintf(w, "  %s\n\n", report.PurgeProgress.Retry)
	}
	if report.Archive != nil {
		fmt.Fprintf(w, "Archive: %s\n", report.Archive.BlobsDir)
		fmt.Fprintf(w, "Generation: %s\n\n", report.Archive.GenerationID)
		for _, artifact := range report.Artifacts {
			if artifact.ArchivedBlob == "" {
				continue
			}
			fmt.Fprintf(w, "Restore prior %s with: cp %s/%s.blob %s\n",
				prepareDisplayPath(artifact.Path),
				report.Archive.BlobsDir,
				artifact.ArchivedBlob,
				artifact.Path,
			)
		}
		fmt.Fprintf(w, "Remove archived bytes with: tpatch feature intent-archive purge %s --blob <hash> --yes\n", report.Slug)
		fmt.Fprintln(w, "Purging the working tree does not remove committed archive bytes from Git history.")
		fmt.Fprintln(w)
	}
	for _, advisory := range report.Advisories {
		fmt.Fprintf(w, "Advisory: %s\n", advisory.Message)
	}
	if len(report.Advisories) > 0 {
		fmt.Fprintln(w)
	}
	if report.Refusal != nil {
		fmt.Fprintf(w, "Refusal: %s\n", report.Refusal.Code)
		fmt.Fprintf(w, "  %s\n", report.Refusal.Message)
		fmt.Fprintf(w, "  %s\n", report.Refusal.Remediation)
		if report.Refusal.Retry != "" {
			fmt.Fprintln(w)
			fmt.Fprintln(w, prepareRetryHeader)
			fmt.Fprintf(w, "  %s\n", report.Refusal.Retry)
		}
		fmt.Fprintln(w)
	}
	if report.PlanNote != "" {
		fmt.Fprintf(w, "Execution preflight: %s\n\n", report.ExecutionPreflight)
		fmt.Fprintln(w, report.PlanNote)
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, report.Disclaimer)
}

func prepareDisplayPath(value string) string {
	if strings.HasPrefix(value, ".tpatch/features/") {
		parts := strings.Split(value, "/")
		if len(parts) >= 4 {
			return strings.Join(parts[3:], "/")
		}
	}
	return value
}

func prepareProgress(cmd *cobra.Command, options prepareOptions, format string, args ...any) {
	if options.asJSON || options.quiet {
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(), format+"\n", args...)
}

var (
	prepareAcquireAuthority           = intentlock.Acquire
	prepareMutationAuthoritySupported = func() bool { return intentlock.AuthoritySupported }
	prepareRandomHex12                = store.RandomHex12
	prepareNow                        = time.Now
	prepareLoadProvider               = loadProviderFromStore
	prepareLoadProviderConfig         = func(repoStore *store.Store) provider.Config {
		merged, err := repoStore.LoadMergedConfig()
		if err != nil {
			return provider.Config{}
		}
		return provider.Config{
			Type:      merged.Provider.Type,
			BaseURL:   merged.Provider.BaseURL,
			Model:     merged.Provider.Model,
			AuthEnv:   merged.Provider.AuthEnv,
			Initiator: merged.Provider.Initiator,
		}
	}

	beforeAbandonBranch          func()
	beforeAbandonMove            func(string)
	afterAbandonMove             func(string)
	beforeLockAcquire            func()
	beforeRedactionScan          func()
	beforeAbandonEvidenceRename  func(*os.Root, string, string, bool) error
	afterRecoveryComplete        func()
	beforeManualStatusCAS        func()
	beforeIndexRewrite           func(string)
	beforeRehydrateIndexRename   func(string)
	beforePrepareSetRevalidation func()
	prepareIntentpubHook         func(intentpub.CrashPoint, *os.Root, *intentpub.Entry) error
	prepareIntentpubRootOps      func(*os.Root) intentpub.RootOps
	prepareIntentpubRandomHex12  func() (string, error)
	prepareIntentpubScratchMaker func(int) []byte
)

func newPrepareIntentpubOptions() intentpub.Options {
	return intentpub.Options{
		RootOpsFactory: prepareIntentpubRootOps,
		RandomHex12:    prepareIntentpubRandomHex12,
		Hook:           prepareIntentpubHook,
		ScratchFactory: prepareIntentpubScratchMaker,
	}
}

type prepareCaptured struct {
	state    string
	bytes    []byte
	identity intentpub.Identity
}

type prepareReadState struct {
	inspection  intent.Report
	status      prepareCaptured
	analysis    prepareCaptured
	spec        prepareCaptured
	exploration prepareCaptured
	sidecar     prepareCaptured
	statusDoc   store.FeatureStatus
}

type prepareInspectionAbort struct {
	abort *intent.Abort
}

type prepareAbandonEvidenceError struct {
	rel string
}

type prepareAbandonMoveError struct {
	cause       error
	entryRel    string
	evidenceRel string
	unsafe      bool
}

func (err *prepareAbandonEvidenceError) Error() string {
	return "unsafe abandon evidence"
}

func (err *prepareAbandonMoveError) Error() string {
	if err == nil {
		return "abandon move failed"
	}
	if err.unsafe {
		return "abandon rollback could not preserve one coherent evidence location"
	}
	return "abandon move failed and was rolled back"
}

func (err *prepareAbandonMoveError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.cause
}

func (err *prepareInspectionAbort) Error() string {
	if err == nil || err.abort == nil {
		return "prepare inspection aborted"
	}
	return string(err.abort.Code)
}

func runPreparePublish(cmd *cobra.Command, rawSlug string, options prepareOptions) error {
	slug, err := intent.CanonicalSlug(rawSlug)
	if err != nil {
		report := applyPrepareOptionFields(
			newPreparePublishReport(options.mode, "", intent.FeatureStateUnknown), options,
		)
		return emitPreparePublishReport(cmd, refusePrepare(report, "slug-unsafe", ""), 3)
	}

	start, _ := cmd.Flags().GetString("path")
	if start == "" {
		start = "."
	}
	repoRoot, err := store.FindProjectRoot(start)
	if err != nil {
		report := applyPrepareOptionFields(
			newPreparePublishReport(options.mode, slug, intent.FeatureStateUnknown), options,
		)
		return emitPreparePublishReport(cmd, refusePrepare(report, "workspace-not-initialized", ""), 3)
	}
	if !intent.RootConfinementSupported() {
		report := applyPrepareOptionFields(
			newPreparePublishReport(options.mode, slug, intent.FeatureStateUnknown), options,
		)
		return emitPreparePublishReport(cmd, refusePrepare(report, "workspace-unsupported-platform", ""), 3)
	}

	if options.mode == prepareModeAbandon {
		return runPrepareAbandon(cmd, repoRoot, slug, options)
	}

	readRoot, err := os.OpenRoot(repoRoot)
	if err != nil {
		report := applyPrepareOptionFields(
			newPreparePublishReport(options.mode, slug, intent.FeatureStateUnknown), options,
		)
		return emitPreparePublishReport(cmd, refusePrepare(report, "workspace-not-initialized", ""), 3)
	}
	readState, readErr := inspectPrepareReadState(readRoot, slug)
	if readErr != nil {
		_ = readRoot.Close()
		report := applyPrepareOptionFields(
			newPreparePublishReport(options.mode, slug, intent.FeatureStateUnknown), options,
		)
		code, exit := prepareReadError(readErr)
		return emitPreparePublishReport(cmd, refusePrepare(report, code, ""), exit)
	}
	report := applyPrepareOptionFields(
		newPreparePublishReport(options.mode, slug, readState.inspection.FeatureState), options,
	)
	report.Artifacts = prepareArtifactRows(readState, options.mode)

	if options.dryRun {
		planned, exit := runPrepareDryPlan(repoRoot, readRoot, readState, report, options)
		_ = readRoot.Close()
		return emitPreparePublishReport(cmd, planned, exit)
	}
	_ = readRoot.Close()

	if !prepareMutationAuthoritySupported() {
		report = prepareAuthorityRefusal(repoRoot, slug, report, "prepare-unsupported-platform", "")
		return emitPreparePublishReport(cmd, report, 3)
	}

	if beforeLockAcquire != nil {
		beforeLockAcquire()
	}
	authority, err := prepareAcquireAuthority(repoRoot)
	if err != nil {
		code, class := prepareAuthorityError(err)
		report = prepareAuthorityRefusal(repoRoot, slug, report, code, class)
		return emitPreparePublishReport(cmd, report, 3)
	}
	released := false
	release := func() error {
		if released {
			return nil
		}
		released = true
		return authority.Release()
	}
	defer func() {
		_ = release()
	}()

	gitState, gitErr := gitutil.DiscoverGitState(repoRoot)
	if gitErr != nil || gitState == gitutil.GitUnverifiable {
		_ = release()
		return emitPreparePublishReport(cmd, refusePrepare(report, "local-lane-unverifiable", ""), 3)
	}
	if gitState == gitutil.GitNonWorktree {
		report.Advisories = append(report.Advisories, prepareAdvisory(
			"workspace-not-git", "", "Git established that this workspace is not a worktree; no Git recovery route is available.",
		))
	} else {
		laneRel := ".tpatch/local/intent-prepare/" + slug
		ignored, ignoreErr := gitutil.IsIgnoredWithState(repoRoot, gitState, laneRel)
		tracked, trackedErr := gitutil.AnythingTrackedUnderWithState(repoRoot, gitState)
		if ignoreErr != nil || trackedErr != nil {
			_ = release()
			return emitPreparePublishReport(cmd, refusePrepare(report, "local-lane-unverifiable", ""), 3)
		}
		if !ignored || tracked {
			_ = release()
			return emitPreparePublishReport(cmd, refusePrepare(report, "local-lane-not-ignored", ""), 3)
		}
		if options.mode == prepareModeRegenerate {
			trackedBundle, trackErr := gitutil.IsTpatchTrackedWithState(repoRoot, gitState)
			if trackErr != nil {
				_ = release()
				return emitPreparePublishReport(cmd, refusePrepare(report, "local-lane-unverifiable", ""), 3)
			}
			if !trackedBundle {
				report.Advisories = append(report.Advisories, prepareAdvisory(
					"bundle-untracked-in-git", "",
					"The intent archive will not survive a fresh clone or Git clean until .tpatch is committed.",
				))
			}
		}
	}

	transactionOptions := newPrepareIntentpubOptions()
	recovery, recoveryErr := intentpub.Recover(authority, slug, transactionOptions)
	if recoveryErr != nil {
		report = prepareIntentpubFailure(report, recovery, recoveryErr)
		_ = release()
		return emitPreparePublishReport(cmd, report, recovery.ExitClass)
	}
	if recovery.Outcome == intentpub.OutcomeRecovered {
		if recoveredState, inspectErr := inspectPrepareWithAuthority(authority, slug); inspectErr == nil {
			report.FeatureState = recoveredState.inspection.FeatureState
			report.Artifacts = prepareArtifactRows(recoveredState, options.mode)
		}
		report.Outcome = "recovered"
		report.Action = "none"
		report.Recovery = &prepareRecoveryReport{
			Kind:            "journal-undo",
			RestoredEntries: prepareArtifactIDPaths(slug, recovery.Restored),
			FinalizedHashes: []string{},
			Retry:           prepareRetryCommand(slug, options),
			RetryCWD:        "workspace-root",
		}
		report.Advisories = append(report.Advisories, prepareAdvisory(
			"recovered-prior-transaction", "",
			fmt.Sprintf(
				"Recovered %d interrupted prepare entrie(s); the requested operation was not performed. Re-run with %s.",
				len(report.Recovery.RestoredEntries), report.Recovery.Retry,
			),
		))
		if prepareContainsArtifactID(recovery.Restored, intentpub.ArtifactStatus) || recovery.Completed {
			refreshPrepareFeaturesIndex(authority, repoRoot, &report)
		}
		if afterRecoveryComplete != nil {
			afterRecoveryComplete()
		}
		_ = release()
		return emitPreparePublishReport(cmd, report, 0)
	}
	archiveStorage := newPrepareArchiveStorage(authority, nil)
	pending, pendingErr := preparePendingArchiveHashes(archiveStorage, slug)
	if pendingErr != nil {
		report = prepareStoreArchiveFailure(report, pendingErr, false)
		_ = release()
		return emitPreparePublishReport(cmd, report, prepareArchiveExit(pendingErr, 3))
	}
	if len(pending) != 0 {
		retry := preparePendingPurgeCommand(slug, pending)
		report = refusePrepare(report, "recovery-pending", retry)
		_ = release()
		return emitPreparePublishReport(cmd, report, 3)
	}

	authoritativeState, inspectErr := inspectPrepareWithAuthority(authority, slug)
	if inspectErr != nil {
		code, exit := prepareReadError(inspectErr)
		report = applyPrepareOptionFields(
			newPreparePublishReport(options.mode, slug, intent.FeatureStateUnknown), options,
		)
		_ = release()
		return emitPreparePublishReport(cmd, refusePrepare(report, code, ""), exit)
	}
	readState = authoritativeState
	report.FeatureState = readState.inspection.FeatureState
	report.Artifacts = prepareArtifactRows(readState, options.mode)

	request := prepareCaptured{state: intent.StateAbsent}
	if options.mode == prepareModeGenerate || options.mode == prepareModeRegenerate {
		request, err = capturePrepareWithAuthority(authority, prepareFeatureRel(slug)+"/request.md", intent.MaxArtifactBytes)
		if err != nil || request.state != intent.StatePresentNonempty {
			_ = release()
			return emitPreparePublishReport(cmd, refusePrepare(report, "request-unreadable", ""), 3)
		}
	}

	if code := prepareStateRefusal(readState.inspection.FeatureState); code != "" {
		_ = release()
		return emitPreparePublishReport(cmd, refusePrepare(report, code, ""), 3)
	}

	plan, planCode, planExit := buildPreparePlan(readState, options.mode)
	report.Artifacts = plan.artifacts
	report.Action = plan.action
	if planCode != "" {
		_ = release()
		return emitPreparePublishReport(cmd, refusePrepare(report, planCode, ""), planExit)
	}
	report = applyPreparePlanAdvisories(report, readState, plan)
	if options.allowHeuristic && options.mode == prepareModeGenerate {
		report.Advisories = append(report.Advisories, prepareAdvisory(
			"allow-heuristic-redundant", "",
			"--allow-heuristic changed nothing because heuristic fallback is already the default in generate mode.",
		))
	}

	repoStore := &store.Store{Root: repoRoot}
	var prov provider.Provider
	var provCfg provider.Config

	if plan.noop || plan.statusOnly {
		snapshot, _, _, archiveErr := prepareArchivePreflight(archiveStorage, slug, plan)
		report = applyPrepareArchiveObservation(report, snapshot)
		if archiveErr != nil {
			report = prepareStoreArchiveFailure(report, archiveErr, false)
			_ = release()
			return emitPreparePublishReport(cmd, report, prepareArchiveExit(archiveErr, 3))
		}
		if plan.noop {
			if revalidationErr := revalidatePrepareSet(authority, readState); revalidationErr != nil {
				report = refusePrepare(report, prepareIntentpubCode(revalidationErr, "entry-changed"), "")
				report.Outcome = "rolled-back"
				_ = release()
				return emitPreparePublishReport(cmd, report, 5)
			}
			report.Outcome = "no-op"
			report.Action = "none"
			_ = release()
			return emitPreparePublishReport(cmd, report, 0)
		}
		notes := "Intent bundle adopted (prepare --manual); artifacts authored by hand"
		if options.mode == prepareModeGenerate {
			notes = "Intent bundle prepared (prepare); generated: "
		}
		statusBytes, statusErr := prepareStatusBytes(readState.status.bytes, notes)
		if statusErr != nil {
			_ = release()
			return emitPreparePublishReport(cmd, refusePrepare(report, "status-malformed", ""), 3)
		}
		if cleanupErr := cleanupPrepareUnarmedLane(authority, slug, transactionOptions); cleanupErr != nil {
			result := intentpub.Result{ExitClass: 6, Orphans: []string{}}
			report = prepareIntentpubFailure(report, result, cleanupErr)
			_ = release()
			return emitPreparePublishReport(cmd, report, 6)
		}
		if revalidationErr := revalidatePrepareSet(authority, readState); revalidationErr != nil {
			report = refusePrepare(report, prepareIntentpubCode(revalidationErr, "entry-changed"), "")
			report.Outcome = "rolled-back"
			_ = release()
			return emitPreparePublishReport(cmd, report, 5)
		}
		resultReport, exit := publishPrepareStatusOnly(authority, repoRoot, readState, statusBytes, report)
		_ = release()
		return emitPreparePublishReport(cmd, resultReport, exit)
	}

	if options.mode == prepareModeGenerate || options.mode == prepareModeRegenerate {
		prov, provCfg = prepareLoadProvider(repoStore)
	}
	if options.mode == prepareModeRegenerate && !options.allowHeuristic && (prov == nil || !provCfg.Configured()) {
		_ = release()
		return emitPreparePublishReport(cmd, refusePrepare(report, "provider-required-for-regenerate", ""), 3)
	}

	generated, generationErr := generatePrepareBundle(
		cmd, authority, repoStore, slug, request.bytes, readState, plan, options, prov, provCfg,
	)
	if generationErr != nil {
		report = prepareGenerationFailure(report, generated)
		if cleanupErr := cleanupPrepareUnarmedLane(authority, slug, transactionOptions); cleanupErr != nil {
			result := intentpub.Result{ExitClass: 6, Orphans: []string{}}
			report = prepareIntentpubFailure(report, result, cleanupErr)
			_ = release()
			return emitPreparePublishReport(cmd, report, 6)
		}
		stageRel, retainErr := retainPrepareGeneratedStage(
			authority, slug, readState, generated, transactionOptions,
		)
		if retainErr != nil {
			report, exit := prepareStagingFailure(report, retainErr, stageRel)
			_ = release()
			return emitPreparePublishReport(cmd, report, exit)
		}
		if stageRel != "" {
			report = appendPrepareStagingAdvisory(report, stageRel)
		}
		_ = release()
		return emitPreparePublishReport(cmd, report, 5)
	}
	report = applyPrepareGenerationReport(report, generated, options)

	hasArchive := options.mode == prepareModeRegenerate && len(plan.replacements) != 0
	if hasArchive {
		if sensitiveCode, artifactID, classes := prepareRedactionRefusal(plan.replacements); sensitiveCode != "" {
			report = refusePrepare(report, sensitiveCode, "")
			report.Refusal.Message = "Prior bytes for artifact " + artifactID +
				" matched sensitive classes " + classes + "."
			_ = release()
			return emitPreparePublishReport(cmd, report, 3)
		}
	}
	snapshot, appendPlan, _, archiveErr := prepareArchivePreflight(archiveStorage, slug, plan)
	report = applyPrepareArchiveObservation(report, snapshot)
	if archiveErr != nil {
		report = prepareStoreArchiveFailure(report, archiveErr, false)
		_ = release()
		return emitPreparePublishReport(cmd, report, prepareArchiveExit(archiveErr, 3))
	}
	stageInputs, stageInputErr := preparePublicationStageInputs(readState, report, plan, generated)
	if stageInputErr != nil {
		_ = release()
		return emitPreparePublishReport(cmd, refusePrepare(report, "status-malformed", ""), 3)
	}
	if validationErr := intentpub.ValidateStageInputs(stageInputs); validationErr != nil {
		code := prepareIntentpubCode(validationErr, "staged-output-invalid")
		report = refusePrepare(report, code, "")
		exit := 5
		var typed *intentpub.Error
		if errors.As(validationErr, &typed) && typed.ExitClass != 0 {
			exit = typed.ExitClass
		}
		_ = release()
		return emitPreparePublishReport(cmd, report, exit)
	}
	if cleanupErr := cleanupPrepareUnarmedLane(authority, slug, transactionOptions); cleanupErr != nil {
		result := intentpub.Result{ExitClass: 6, Orphans: []string{}}
		report = prepareIntentpubFailure(report, result, cleanupErr)
		_ = release()
		return emitPreparePublishReport(cmd, report, 6)
	}

	stageResult, stageErr := stagePreparePublicationBase(
		authority, slug, stageInputs, transactionOptions,
	)
	if stageErr != nil {
		report, exit := prepareStagingFailure(report, stageErr, stageResult.StageRel)
		_ = release()
		return emitPreparePublishReport(cmd, report, exit)
	}
	if beforePrepareSetRevalidation != nil {
		beforePrepareSetRevalidation()
	}
	if revalidationErr := revalidatePrepareSet(authority, readState); revalidationErr != nil {
		report = refusePrepare(report, prepareIntentpubCode(revalidationErr, "entry-changed"), "")
		report.Outcome = "rolled-back"
		report = appendPrepareStagingAdvisory(report, stageResult.StageRel)
		_ = release()
		return emitPreparePublishReport(cmd, report, 5)
	}

	if hasArchive {
		stageErr = stagePrepareArchiveIndex(authority, &stageResult, appendPlan, transactionOptions)
		if stageErr != nil {
			report, exit := prepareStagingFailure(report, stageErr, stageResult.StageRel)
			_ = release()
			return emitPreparePublishReport(cmd, report, exit)
		}
	}

	resultReport, exit := publishPrepareTransaction(
		authority, repoRoot, slug, readState, report, plan, stageResult,
		appendPlan, hasArchive, transactionOptions,
	)
	_ = release()
	return emitPreparePublishReport(cmd, resultReport, exit)
}

func inspectPrepareReadState(root *os.Root, slug string) (prepareReadState, error) {
	scratch := make([]byte, intent.MaxArtifactBytes+1)
	inspection := intent.Inspect(intent.NewRootOps(root), slug, scratch)
	if inspection.Abort != nil {
		return prepareReadState{}, &prepareInspectionAbort{abort: inspection.Abort}
	}
	base := prepareFeatureRel(slug)
	status, err := capturePrepareFileWithScratch(root, base+"/status.json", intent.MaxStatusBytes, scratch)
	if err != nil || !status.identity.Exists {
		return prepareReadState{}, errors.New("status-unreadable")
	}
	var statusDoc store.FeatureStatus
	if err := json.Unmarshal(status.bytes, &statusDoc); err != nil {
		return prepareReadState{}, errors.New("status-malformed")
	}
	analysis, err := capturePrepareFileWithScratch(root, base+"/analysis.md", intent.MaxArtifactBytes, scratch)
	if err != nil {
		return prepareReadState{}, err
	}
	spec, err := capturePrepareFileWithScratch(root, base+"/spec.md", intent.MaxArtifactBytes, scratch)
	if err != nil {
		return prepareReadState{}, err
	}
	exploration, err := capturePrepareFileWithScratch(root, base+"/exploration.md", intent.MaxArtifactBytes, scratch)
	if err != nil {
		return prepareReadState{}, err
	}
	sidecar, err := capturePrepareFileWithScratch(root, base+"/artifacts/analysis.json", intent.MaxArtifactBytes, scratch)
	if err != nil {
		return prepareReadState{}, err
	}
	return prepareReadState{
		inspection:  inspection,
		status:      status,
		analysis:    analysis,
		spec:        spec,
		exploration: exploration,
		sidecar:     sidecar,
		statusDoc:   statusDoc,
	}, nil
}

func inspectPrepareWithAuthority(
	authority *intentlock.WorkspaceAuthority,
	slug string,
) (prepareReadState, error) {
	var state prepareReadState
	err := authority.WithRoot(func(root *os.Root) error {
		var inspectErr error
		state, inspectErr = inspectPrepareReadState(root, slug)
		return inspectErr
	})
	return state, err
}

func prepareReadError(err error) (string, int) {
	var aborted *prepareInspectionAbort
	if errors.As(err, &aborted) && aborted.abort != nil {
		switch aborted.abort.Code {
		case intent.AbortFeatureNotFound:
			return "feature-not-found", 3
		case intent.AbortStatusMalformed, intent.AbortStatusInvalidState:
			return "status-malformed", 3
		case intent.AbortStatusSymlink, intent.AbortStatusNotRegular,
			intent.AbortStatusOversize, intent.AbortStatusUnreadable, intent.AbortStatusUnstable:
			return "status-unreadable", 3
		default:
			return "artifact-unsafe", 3
		}
	}
	switch err.Error() {
	case "status-malformed":
		return "status-malformed", 3
	case "status-unreadable":
		return "status-unreadable", 3
	default:
		return "artifact-unstable", 3
	}
}

func capturePrepareFile(root *os.Root, rel string, limit int) (prepareCaptured, error) {
	return capturePrepareFileWithScratch(root, rel, limit, make([]byte, limit+1))
}

func capturePrepareFileWithScratch(
	root *os.Root,
	rel string,
	limit int,
	scratch []byte,
) (prepareCaptured, error) {
	if root == nil || !validPrepareRel(rel) || limit < 0 || limit > intent.MaxArtifactBytes {
		return prepareCaptured{}, errors.New("invalid rooted capture")
	}
	if len(scratch) < limit+1 {
		return prepareCaptured{}, errors.New("rooted capture scratch is undersized")
	}
	before, err := root.Lstat(rel)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return prepareCaptured{state: intent.StateAbsent, identity: intentpub.AbsentIdentity()}, nil
		}
		return prepareCaptured{state: intent.StateUnreadable}, nil
	}
	if prepareRefusedInfo(before) {
		return prepareCaptured{state: intent.StateSymlinkRefused}, nil
	}
	if !before.Mode().IsRegular() {
		return prepareCaptured{state: intent.StateNotRegular}, nil
	}
	if before.Size() < 0 || before.Size() > int64(limit) {
		return prepareCaptured{state: intent.StateOversize}, nil
	}
	file, err := root.OpenFile(rel, prepareReadOpenFlags(), 0)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return prepareCaptured{state: intent.StateUnstable}, nil
		}
		return prepareCaptured{state: intent.StateUnreadable}, nil
	}
	closed := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
	}()
	opened, err := file.Stat()
	if err != nil || prepareRefusedInfo(opened) || !opened.Mode().IsRegular() ||
		!os.SameFile(before, opened) || before.Size() != opened.Size() ||
		before.Mode().Perm() != opened.Mode().Perm() || !before.ModTime().Equal(opened.ModTime()) {
		return prepareCaptured{state: intent.StateUnstable}, nil
	}
	count, readErr := io.ReadFull(file, scratch[:limit+1])
	switch {
	case readErr == nil:
		return prepareCaptured{state: intent.StateOversize}, nil
	case !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF):
		return prepareCaptured{state: intent.StateUnreadable}, nil
	}
	if int64(count) != opened.Size() {
		return prepareCaptured{state: intent.StateUnstable}, nil
	}
	descriptorAfter, err := file.Stat()
	if err != nil || !os.SameFile(opened, descriptorAfter) ||
		opened.Size() != descriptorAfter.Size() ||
		opened.Mode().Perm() != descriptorAfter.Mode().Perm() ||
		!opened.ModTime().Equal(descriptorAfter.ModTime()) {
		return prepareCaptured{state: intent.StateUnstable}, nil
	}
	if err := file.Close(); err != nil {
		closed = true
		return prepareCaptured{state: intent.StateUnreadable}, nil
	}
	closed = true
	after, err := root.Lstat(rel)
	if err != nil || prepareRefusedInfo(after) || !after.Mode().IsRegular() ||
		!os.SameFile(before, after) || before.Size() != after.Size() ||
		before.Mode().Perm() != after.Mode().Perm() || !before.ModTime().Equal(after.ModTime()) {
		return prepareCaptured{state: intent.StateUnstable}, nil
	}
	data := append([]byte(nil), scratch[:count]...)
	sum := sha256.Sum256(data)
	state := intent.StatePresentNonempty
	if len(bytes.TrimSpace(data)) == 0 {
		state = intent.StatePresentEmpty
	}
	return prepareCaptured{
		state: state,
		bytes: data,
		identity: intentpub.Identity{
			Exists: true,
			SHA256: hex.EncodeToString(sum[:]),
			Size:   int64(len(data)),
			Mode:   uint32(after.Mode().Perm()),
		},
	}, nil
}

func capturePrepareWithAuthority(authority *intentlock.WorkspaceAuthority, rel string, limit int) (prepareCaptured, error) {
	var captured prepareCaptured
	err := authority.WithRoot(func(root *os.Root) error {
		var captureErr error
		captured, captureErr = capturePrepareFile(root, rel, limit)
		return captureErr
	})
	return captured, err
}

func prepareReadOpenFlags() int {
	switch runtime.GOOS {
	case "linux":
		return os.O_RDONLY | 0x800 | 0x20000
	case "darwin":
		return os.O_RDONLY | 0x4 | 0x100
	default:
		return os.O_RDONLY
	}
}

func prepareRefusedInfo(info fs.FileInfo) bool {
	return info == nil || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0
}

func validPrepareRel(rel string) bool {
	return rel != "" && rel != "." && fs.ValidPath(rel)
}

func prepareFeatureRel(slug string) string {
	return ".tpatch/features/" + slug
}

func prepareArtifactRows(state prepareReadState, mode prepareMode) []prepareArtifactReport {
	rows := []prepareArtifactReport{
		prepareArtifactRow(state.inspection, "analysis", state.analysis, mode),
		prepareArtifactRow(state.inspection, "spec", state.spec, mode),
		prepareArtifactRow(state.inspection, "exploration", state.exploration, mode),
		prepareArtifactRow(state.inspection, "analysis_sidecar", state.sidecar, mode),
	}
	return rows
}

func prepareArtifactRow(inspection intent.Report, id string, captured prepareCaptured, mode prepareMode) prepareArtifactReport {
	role := "required"
	if id == "analysis_sidecar" {
		role = "optional"
	}
	inspectionState := captured.state
	for _, artifact := range inspection.Artifacts {
		if artifact.ID == id {
			inspectionState = artifact.State
			break
		}
	}
	disposition := "untouched"
	switch mode {
	case prepareModeGenerate:
		if captured.state == intent.StatePresentNonempty &&
			(id != "analysis_sidecar" || inspectionState == intent.StatePresentNonempty) {
			disposition = "preserved"
		} else if captured.state == intent.StateAbsent && id == "analysis_sidecar" {
			disposition = "absent-optional"
		}
	case prepareModeManual:
		if captured.state == intent.StatePresentNonempty &&
			(id != "analysis_sidecar" || inspectionState == intent.StatePresentNonempty) {
			disposition = "preserved"
		} else if id == "analysis_sidecar" && captured.state == intent.StateAbsent {
			disposition = "absent-optional"
		}
	case prepareModeRegenerate:
		if captured.state == intent.StateAbsent {
			disposition = "generated"
		} else {
			disposition = "regenerated"
		}
	}
	artifactPath := prepareFeatureRel(inspection.Slug) + "/" + prepareArtifactFeaturePath(id)
	return prepareArtifactReport{
		ID:           id,
		Path:         artifactPath,
		Role:         role,
		Disposition:  disposition,
		Generator:    "",
		ArchivedBlob: "",
	}
}

func prepareArtifactFeaturePath(id string) string {
	switch id {
	case "analysis":
		return "analysis.md"
	case "spec":
		return "spec.md"
	case "exploration":
		return "exploration.md"
	case "analysis_sidecar":
		return "artifacts/analysis.json"
	default:
		return ""
	}
}

func prepareAdvisory(code, artifactID, message string) prepareAdvisoryReport {
	return prepareAdvisoryReport{Code: code, ArtifactID: artifactID, Message: message}
}

func refusePrepare(report preparePublishReport, code, retry string) preparePublishReport {
	message, remediation := prepareRefusalText(code, report.Mode, report.Slug, retry)
	report.Outcome = "refused"
	report.Refusal = &prepareRefusalReport{
		Code:        code,
		Message:     message,
		Remediation: remediation,
	}
	if retry != "" {
		report.Refusal.Retry = retry
		report.Refusal.RetryCWD = "workspace-root"
	}
	return report
}

func prepareRefusalText(code string, mode prepareMode, slug, retry string) (string, string) {
	switch code {
	case "slug-unsafe":
		return "The feature slug is not a canonical tpatch slug.", "Use a lowercase kebab-case feature slug."
	case "workspace-not-initialized":
		return "No tpatch workspace was found.", "Run from inside an initialized workspace or pass --path."
	case "workspace-unsupported-platform":
		return "This platform cannot perform rooted workspace inspection.", "Run the command on a supported tpatch platform."
	case "prepare-unsupported-platform":
		return "This platform does not support mutating prepare.", "Run mutating prepare on non-mobile Linux or Darwin."
	case "lock-filesystem-unsupported":
		return "The workspace root filesystem is not supported for prepare mutation.", "Move the workspace to a supported local filesystem and retry."
	case "directory-flock-unavailable":
		return "The workspace directory authority could not be established.", "Fix workspace access and retry."
	case "transaction-in-progress":
		return "The workspace mutation authority is held by another mutating prepare or archive operation. The holder's identity is unknowable.", "Wait for the live operation to finish, then retry."
	case "local-lane-not-ignored":
		return "The local prepare lane is not effectively ignored or contains tracked files.", "Run tpatch init, ensure .tpatch/local/ is ignored, untrack local-lane files, and retry."
	case "local-lane-unverifiable":
		return "Git could not verify the local prepare lane.", "Fix Git availability and repository selection, then retry."
	case "feature-not-found":
		return "The requested feature does not exist.", "Run tpatch add to create the feature, then retry."
	case "status-malformed":
		return "status.json is malformed.", "Repair status.json with a valid tpatch status document, then retry."
	case "status-unreadable":
		return "status.json could not be read safely.", "Restore status.json as a readable regular file, then retry."
	case "request-unreadable":
		return "request.md is absent, empty, unsafe, unstable, or unreadable.", "Restore a non-empty regular request.md and retry."
	case "artifact-unsafe":
		return "An intent artifact cannot be read or replaced safely.", "Restore the artifact as a bounded regular file and retry."
	case "artifact-unstable":
		return "An intent artifact changed while it was inspected.", "Stop concurrent writers and retry."
	case "artifact-empty-not-overwritten":
		return "A required artifact is present but empty and will not be overwritten by default.", "Use tpatch prepare " + slug + " --regenerate, or author the file and run tpatch prepare " + slug + " --manual."
	case "incoherent-bundle-gap":
		return "The present and absent artifacts are not a dependency-coherent suffix.", "Use tpatch prepare " + slug + " --regenerate, or author the missing file and run tpatch prepare " + slug + " --manual."
	case "not-ready":
		return "The hand-authored intent bundle is not structurally ready.", "Complete all three required Markdown artifacts and retry tpatch prepare " + slug + " --manual."
	case "state-refused":
		return "The feature lifecycle state does not permit prepare.", prepareStateRemediation(slug)
	case "provider-required-for-regenerate":
		return "Regeneration requires a configured successful provider unless heuristic replacement is explicitly allowed.", "Run tpatch provider set and tpatch provider check, or retry with tpatch prepare " + slug + " --regenerate --allow-heuristic."
	case "archive-content-refused-sensitive":
		return "Prior artifact bytes matched the archive redaction policy.", "Remove the sensitive material from the named artifact and retry."
	case "recovery-pending":
		if retry != "" {
			return "An archive purge transaction owns one or more content hashes.", "Resume the owning archive purge transaction."
		}
		return "An interrupted prepare transaction must be recovered or abandoned before planning.", "Run a mutating tpatch prepare " + slug + ", or tpatch prepare " + slug + " --abandon-transaction --yes."
	case "no-pending-transaction":
		return "No journal, preimage, or staging evidence exists to abandon.", "Run an admissible prepare operation, or remove only the reported abandoned residue when it is no longer needed."
	case "abandon-evidence-unsafe":
		return "Pending transaction evidence has an unsafe kind or location.", "Inspect the reported repo-relative lane entry and restore it to the expected regular-file or directory kind."
	case "regenerate-generation-failed":
		return "Regeneration could not produce a complete validated bundle.", "Fix the provider or deadline condition and retry the same prepare command."
	case "status-changed":
		return "status.json changed before the rooted publication rename.", "Retry from the newly observed feature state."
	case "entry-appeared", "entry-changed", "archive-index-changed", "workspace-root-changed":
		return "The frozen publication set changed before publication.", "Retry from the newly observed workspace tree."
	case "staged-output-invalid":
		return "A generated artifact failed structural staged-output validation.", "Fix the generation input or provider and retry."
	case "undo-cas-mismatch", "recovery-divergent", "journal-corrupt",
		"journal-version-mismatch", "journal-foreign", "journal-path-escape",
		"journal-forged", "post-publication-divergence",
		"workspace-root-replaced-after-publication":
		return "Prepare transaction evidence requires manual intervention.", "Run tpatch prepare " + slug + " --abandon-transaction to preview the evidence-preserving escape."
	case "archive-index-version-unsupported":
		return "The intent archive requires a newer tpatch version.", "Upgrade tpatch and retry."
	case "archive-index-corrupt", "archive-index-foreign", "archive-index-path-escape",
		"archive-index-generation-mismatch", "archive-generation-id-collision",
		"archive-blob-corrupt", "archive-blob-dangling",
		"archive-index-storage-inconsistent", "archive-blob-shared",
		"archive-purge-index-changed":
		return "The intent archive is not admissible for mutation.", prepareArchiveRepairText(code, slug)
	default:
		return "Prepare could not safely complete the requested operation.", "Correct the reported condition and retry."
	}
}

func prepareStateRemediation(slug string) string {
	return "For rejected features run tpatch reopen " + slug + "; for unapplied features run tpatch apply " + slug + "; otherwise finish the current lifecycle operation first."
}

func prepareArchiveRepairText(code, slug string) string {
	switch code {
	case "archive-blob-dangling":
		return "Run tpatch feature intent-archive purge " + slug + " --blob <hash> --yes from the workspace root."
	case "archive-index-storage-inconsistent":
		return "Run the reported tpatch feature intent-archive purge repair from the workspace root."
	case "archive-blob-corrupt":
		return "Restore the exact correct managed blob, or remove the reported managed object and run the reported tpatch feature intent-archive purge repair."
	default:
		return "Preserve the archive bytes, repair the reported index or blob condition, and retry."
	}
}

type preparePlan struct {
	action       string
	artifacts    []prepareArtifactReport
	generated    []intentpub.ArtifactID
	replacements []store.IntentArchiveReplacementInput
	noop         bool
	statusOnly   bool
}

func buildPreparePlan(state prepareReadState, mode prepareMode) (preparePlan, string, int) {
	plan := preparePlan{
		action:       "none",
		artifacts:    prepareArtifactRows(state, mode),
		generated:    []intentpub.ArtifactID{},
		replacements: []store.IntentArchiveReplacementInput{},
	}
	switch mode {
	case prepareModeManual:
		for _, captured := range []prepareCaptured{state.analysis, state.spec, state.exploration} {
			switch captured.state {
			case intent.StatePresentNonempty:
			case intent.StateUnstable:
				return plan, "artifact-unstable", 3
			case intent.StateSymlinkRefused, intent.StateNotRegular,
				intent.StateUnreadable, intent.StateOversize:
				return plan, "artifact-unsafe", 3
			default:
				return plan, "not-ready", 2
			}
		}
		if prepareManualStatusUnchanged(state.statusDoc) {
			plan.noop = true
			return plan, "", 0
		}
		plan.action = "adopt"
		plan.statusOnly = true
		return plan, "", 0

	case prepareModeRegenerate:
		captures := []struct {
			id       intentpub.ArtifactID
			archive  store.IntentArchiveArtifactID
			feature  string
			captured prepareCaptured
		}{
			{intentpub.ArtifactAnalysis, store.IntentArchiveArtifactAnalysis, "analysis.md", state.analysis},
			{intentpub.ArtifactSpec, store.IntentArchiveArtifactSpec, "spec.md", state.spec},
			{intentpub.ArtifactExploration, store.IntentArchiveArtifactExploration, "exploration.md", state.exploration},
			{intentpub.ArtifactAnalysisSidecar, store.IntentArchiveArtifactAnalysisSidecar, "artifacts/analysis.json", state.sidecar},
		}
		for _, item := range captures {
			switch item.captured.state {
			case intent.StateAbsent:
			case intent.StatePresentNonempty, intent.StatePresentEmpty:
				plan.replacements = append(plan.replacements, store.IntentArchiveReplacementInput{
					ArtifactID: item.archive,
					Path:       item.feature,
					PriorBytes: append([]byte(nil), item.captured.bytes...),
				})
			default:
				if item.id == intentpub.ArtifactAnalysisSidecar &&
					item.captured.state == intent.StatePresentNonempty {
					break
				}
				if item.captured.state == intent.StateUnstable {
					return plan, "artifact-unstable", 3
				}
				return plan, "artifact-unsafe", 3
			}
			plan.generated = append(plan.generated, item.id)
		}
		plan.action = "regenerate"
		return plan, "", 0
	}

	required := []prepareCaptured{state.analysis, state.spec, state.exploration}
	for _, captured := range required {
		switch captured.state {
		case intent.StatePresentNonempty, intent.StateAbsent:
		case intent.StatePresentEmpty:
			return plan, "artifact-empty-not-overwritten", 2
		case intent.StateUnstable:
			return plan, "artifact-unstable", 3
		default:
			return plan, "artifact-unsafe", 3
		}
	}

	present := [3]bool{
		state.analysis.state == intent.StatePresentNonempty,
		state.spec.state == intent.StatePresentNonempty,
		state.exploration.state == intent.StatePresentNonempty,
	}
	switch present {
	case [3]bool{false, false, false}:
		if state.sidecar.state != intent.StateAbsent {
			if prepareUnsafeArtifactState(state.sidecar.state) {
				if state.sidecar.state == intent.StateUnstable {
					return plan, "artifact-unstable", 3
				}
				return plan, "artifact-unsafe", 3
			}
			return plan, "incoherent-bundle-gap", 2
		}
		plan.generated = []intentpub.ArtifactID{
			intentpub.ArtifactAnalysis,
			intentpub.ArtifactSpec,
			intentpub.ArtifactExploration,
			intentpub.ArtifactAnalysisSidecar,
		}
		plan.action = "complete"
	case [3]bool{true, false, false}:
		plan.generated = []intentpub.ArtifactID{intentpub.ArtifactSpec, intentpub.ArtifactExploration}
		plan.action = "complete"
		if state.sidecar.state == intent.StateAbsent {
			plan.artifacts[3].Disposition = "absent-optional"
		}
	case [3]bool{true, true, false}:
		plan.generated = []intentpub.ArtifactID{intentpub.ArtifactExploration}
		plan.action = "complete"
	case [3]bool{true, true, true}:
		if state.inspection.FeatureState == intent.FeatureStateDefined {
			plan.noop = true
			plan.action = "none"
		} else {
			plan.statusOnly = true
			plan.action = "adopt"
		}
	default:
		return plan, "incoherent-bundle-gap", 2
	}
	for index := range plan.artifacts {
		id, _ := prepareIntentpubArtifactID(plan.artifacts[index].ID)
		if prepareContainsArtifactID(plan.generated, id) {
			plan.artifacts[index].Disposition = "generated"
		}
	}
	if state.analysis.state == intent.StatePresentNonempty && state.sidecar.state == intent.StateAbsent {
		plan.artifacts[3].Disposition = "absent-optional"
	}
	return plan, "", 0
}

func prepareManualStatusUnchanged(status store.FeatureStatus) bool {
	return status.State == store.StateDefined &&
		status.LastCommand == "prepare" &&
		status.Notes == "Intent bundle adopted (prepare --manual); artifacts authored by hand"
}

func prepareUnsafeArtifactState(state string) bool {
	switch state {
	case intent.StateSymlinkRefused, intent.StateNotRegular, intent.StateUnreadable,
		intent.StateOversize, intent.StateUnstable:
		return true
	default:
		return false
	}
}

func prepareStateRefusal(state string) string {
	switch state {
	case intent.FeatureStateRequested, intent.FeatureStateAnalyzed, intent.FeatureStateDefined:
		return ""
	default:
		return "state-refused"
	}
}

func runPrepareDryPlan(
	repoRoot string,
	root *os.Root,
	state prepareReadState,
	report preparePublishReport,
	options prepareOptions,
) (preparePublishReport, int) {
	report.DryRun = true
	report.ExecutionPreflight = "not_evaluated"
	report.PlanNote = preparePlanOnly
	pending, err := prepareJournalMarkerExists(root, report.Slug)
	if err != nil || pending {
		return refusePrepare(report, "recovery-pending", ""), 3
	}
	if code := prepareStateRefusal(state.inspection.FeatureState); code != "" {
		return refusePrepare(report, code, ""), 3
	}
	if options.mode == prepareModeGenerate || options.mode == prepareModeRegenerate {
		request, captureErr := capturePrepareFile(root, prepareFeatureRel(report.Slug)+"/request.md", intent.MaxArtifactBytes)
		if captureErr != nil || request.state != intent.StatePresentNonempty {
			return refusePrepare(report, "request-unreadable", ""), 3
		}
	}
	plan, code, exit := buildPreparePlan(state, options.mode)
	report.Artifacts = plan.artifacts
	report.Action = plan.action
	if code != "" {
		return refusePrepare(report, code, ""), exit
	}
	report = applyPreparePlanAdvisories(report, state, plan)

	storage := newPrepareArchiveStorage(nil, root)
	snapshot, archiveErr := store.CaptureIntentArchive(storage, report.Slug)
	if archiveErr == nil {
		archiveErr = prepareValidateArchiveSnapshot(snapshot, false)
	}
	if archiveErr != nil {
		return prepareStoreArchiveFailure(report, archiveErr, false), prepareArchiveExit(archiveErr, 3)
	}
	if options.mode == prepareModeRegenerate {
		if !options.allowHeuristic {
			repoStore := &store.Store{Root: repoRoot}
			cfg := prepareLoadProviderConfig(repoStore)
			if !cfg.Configured() {
				return refusePrepare(report, "provider-required-for-regenerate", ""), 3
			}
		}
	}
	if options.allowHeuristic && options.mode == prepareModeGenerate {
		report.Advisories = append(report.Advisories, prepareAdvisory(
			"allow-heuristic-redundant", "",
			"--allow-heuristic changed nothing because heuristic fallback is already the default in generate mode.",
		))
	}
	report.Outcome = "planned"
	actions := preparePlanActions(plan, report.Slug)
	report.Actions = &actions
	return report, 0
}

func preparePlanActions(plan preparePlan, slug string) []prepareActionReport {
	actions := make([]prepareActionReport, 0, len(plan.generated)+2)
	for _, id := range plan.generated {
		name := prepareIntentpubReportID(id)
		action := "create"
		if plan.action == "regenerate" {
			for _, artifact := range plan.artifacts {
				if artifact.ID == name && artifact.Disposition == "regenerated" {
					action = "replace"
					break
				}
			}
		}
		actions = append(actions, prepareActionReport{
			ID:     name,
			Path:   prepareFeatureRel(slug) + "/" + prepareArtifactFeaturePath(name),
			Action: action,
		})
	}
	if !plan.noop {
		actions = append(actions, prepareActionReport{
			ID:     "status",
			Path:   prepareFeatureRel(slug) + "/status.json",
			Action: "replace",
		})
	}
	return actions
}

func applyPreparePlanAdvisories(
	report preparePublishReport,
	state prepareReadState,
	plan preparePlan,
) preparePublishReport {
	if report.Mode == prepareModeGenerate &&
		state.analysis.state == intent.StatePresentNonempty &&
		state.sidecar.state == intent.StateAbsent {
		report.Advisories = append(report.Advisories, prepareAdvisory(
			"analysis-preserved-sidecar-untouched", "analysis_sidecar",
			"analysis.md was preserved, so the absent analysis sidecar was not synthesized.",
		))
	}
	if !plan.noop &&
		(state.inspection.FeatureState == intent.FeatureStateRequested ||
			state.inspection.FeatureState == intent.FeatureStateAnalyzed) {
		report.Advisories = append(report.Advisories, prepareAdvisory(
			"feature-state-below-defined", "",
			"The feature was below defined and this prepare transition advances it to defined.",
		))
	}
	return report
}

func prepareJournalMarkerExists(root *os.Root, slug string) (bool, error) {
	for _, rel := range []string{intentpub.JournalRel(slug), intentpub.JournalMarkerRel(slug)} {
		info, err := root.Lstat(rel)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, err
		}
		if info != nil {
			return true, nil
		}
	}
	return false, nil
}

func prepareAuthorityError(err error) (string, string) {
	var typed *intentlock.Error
	if errors.As(err, &typed) {
		return string(typed.Code), typed.Class
	}
	return "directory-flock-unavailable", ""
}

func prepareAuthorityRefusal(repoRoot, slug string, report preparePublishReport, code, class string) preparePublishReport {
	report = refusePrepare(report, code, "")
	if code == "transaction-in-progress" {
		return report
	}
	if code != "prepare-unsupported-platform" &&
		code != "lock-filesystem-unsupported" &&
		code != "directory-flock-unavailable" {
		return report
	}
	if !prepareLaneHasPendingEvidence(repoRoot, slug) {
		return report
	}
	lane := ".tpatch/local/intent-prepare/" + slug + "/"
	detail := ""
	if class != "" {
		detail = " (" + class + ")"
	}
	report.Refusal.Message += " Pending evidence remains in " + lane + detail + "."
	report.Refusal.Remediation =
		"Nothing under that lane is tracked. Last resort, from the workspace root run rm -rf " + lane +
			". This unblocks the slug without changing .tpatch/features/, but permanently discards the undo evidence; canonical artifacts remain exactly as the interrupted run left them."
	return report
}

func prepareLaneHasPendingEvidence(repoRoot, slug string) bool {
	root, err := os.OpenRoot(repoRoot)
	if err != nil {
		return false
	}
	defer root.Close()
	lane := ".tpatch/local/intent-prepare/" + slug
	info, err := root.Lstat(lane)
	if err != nil || prepareRefusedInfo(info) || !info.IsDir() {
		return false
	}
	directory, err := root.Open(lane)
	if err != nil {
		return false
	}
	names, readErr := directory.Readdirnames(-1)
	closeErr := directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) || closeErr != nil {
		return false
	}
	for _, name := range names {
		if preparePendingEvidenceName(name) {
			return true
		}
	}
	return false
}

func runPrepareAbandon(cmd *cobra.Command, repoRoot, slug string, options prepareOptions) error {
	report := newPreparePublishReport(prepareModeAbandon, slug, intent.FeatureStateUnknown)
	report.allowHeuristic = options.allowHeuristic
	if !prepareMutationAuthoritySupported() {
		report = prepareAuthorityRefusal(repoRoot, slug, report, "prepare-unsupported-platform", "")
		return emitPreparePublishReport(cmd, report, 3)
	}
	if beforeLockAcquire != nil {
		beforeLockAcquire()
	}
	authority, err := prepareAcquireAuthority(repoRoot)
	if err != nil {
		code, class := prepareAuthorityError(err)
		report = prepareAuthorityRefusal(repoRoot, slug, report, code, class)
		return emitPreparePublishReport(cmd, report, 3)
	}
	released := false
	release := func() error {
		if released {
			return nil
		}
		released = true
		return authority.Release()
	}
	defer func() { _ = release() }()

	if beforeAbandonBranch != nil {
		beforeAbandonBranch()
	}
	evidence, existing, err := inspectPrepareAbandonEvidence(authority, slug)
	if err != nil {
		report = refusePrepare(report, "abandon-evidence-unsafe", "")
		var unsafe *prepareAbandonEvidenceError
		if errors.As(err, &unsafe) && unsafe.rel != "" {
			report.Refusal.Message += " Affected lane entry: " + unsafe.rel + "."
		}
		_ = release()
		return emitPreparePublishReport(cmd, report, 3)
	}
	if len(evidence) == 0 {
		report = refusePrepare(report, "no-pending-transaction", "")
		if len(existing) != 0 {
			report.Abandoned = &prepareAbandonedReport{
				Existing: append([]string(nil), existing...),
			}
			report.Refusal.Remediation = prepareAbandonedResidueRemediation(existing)
		} else if pending, pendingErr := preparePendingArchiveHashes(newPrepareArchiveStorage(authority, nil), slug); pendingErr == nil && len(pending) != 0 {
			retry := preparePendingPurgeCommand(slug, pending)
			report.Refusal.Remediation = "Resume the owning archive purge transaction."
			report.Refusal.Retry = retry
			report.Refusal.RetryCWD = "workspace-root"
		}
		_ = release()
		return emitPreparePublishReport(cmd, report, 3)
	}

	suffix, err := prepareRandomHex12()
	if err != nil || !validPrepareHex(suffix, 12) {
		report = refusePrepare(report, "abandon-evidence-unsafe", "")
		_ = release()
		return emitPreparePublishReport(cmd, report, 3)
	}
	destination := ".tpatch/local/intent-prepare/" + slug + "/abandoned-" + suffix
	report.Action = "abandon"
	report.Outcome = "abandon-planned"
	report.Abandoned = &prepareAbandonedReport{
		Directory:     destination + "/",
		Moved:         append([]string(nil), evidence...),
		RemoveCommand: "rm -rf " + destination + "/",
	}
	if !options.yes {
		_ = release()
		return emitPreparePublishReport(cmd, report, 0)
	}

	if beforeAbandonMove != nil {
		beforeAbandonMove(destination)
	}
	if moveErr := movePrepareAbandonEvidence(authority, slug, destination, evidence); moveErr != nil {
		report = refusePrepare(report, "entry-changed", "")
		report.Outcome = "rolled-back"
		report.Abandoned = nil
		exit := 5
		var unsafe *prepareAbandonMoveError
		if errors.As(moveErr, &unsafe) && unsafe.unsafe {
			report = refusePrepare(report, "recovery-divergent", "")
			report.Outcome = "recovery-refused"
			report.Refusal.Message =
				"Abandon rollback could not restore lane entry " + unsafe.entryRel +
					" without overwriting concurrent bytes. Evidence remains preserved in " +
					unsafe.evidenceRel + "."
			report.Refusal.Remediation =
				"Inspect both repo-relative locations, then rerun tpatch prepare " + slug +
					" --abandon-transaction --yes to preserve any remaining canonical-lane evidence."
			report.Abandoned = &prepareAbandonedReport{
				Existing: []string{unsafe.evidenceRel},
			}
			exit = 6
		}
		_ = release()
		return emitPreparePublishReport(cmd, report, exit)
	}
	if afterAbandonMove != nil {
		afterAbandonMove(destination)
	}
	report.Outcome = "abandoned"
	_ = release()
	return emitPreparePublishReport(cmd, report, 0)
}

func inspectPrepareAbandonEvidence(authority *intentlock.WorkspaceAuthority, slug string) ([]string, []string, error) {
	lane := ".tpatch/local/intent-prepare/" + slug
	evidence := []string{}
	existing := []string{}
	err := authority.WithRoot(func(root *os.Root) error {
		info, err := root.Lstat(lane)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil || prepareRefusedInfo(info) || !info.IsDir() {
			return &prepareAbandonEvidenceError{rel: lane + "/"}
		}
		directory, err := root.Open(lane)
		if err != nil {
			return err
		}
		names, readErr := directory.Readdirnames(-1)
		closeErr := directory.Close()
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		sort.Strings(names)
		for _, name := range names {
			rel := lane + "/" + name
			switch {
			case prepareControlEvidenceName(name):
				entry, statErr := root.Lstat(rel)
				if statErr != nil || prepareRefusedInfo(entry) || !entry.Mode().IsRegular() {
					return &prepareAbandonEvidenceError{rel: rel}
				}
				evidence = append(evidence, rel)
			case prepareStageEvidenceName(name):
				entry, statErr := root.Lstat(rel)
				if statErr != nil || prepareRefusedInfo(entry) || !entry.IsDir() {
					return &prepareAbandonEvidenceError{rel: rel}
				}
				evidence = append(evidence, rel)
			case prepareAbandonedEvidenceName(name):
				entry, statErr := root.Lstat(rel)
				if statErr != nil || prepareRefusedInfo(entry) || !entry.IsDir() {
					return &prepareAbandonEvidenceError{rel: rel}
				}
				existing = append(existing, rel+"/")
			}
		}
		return nil
	})
	return evidence, existing, err
}

func movePrepareAbandonEvidence(
	authority *intentlock.WorkspaceAuthority,
	slug, destination string,
	evidence []string,
) error {
	lane := ".tpatch/local/intent-prepare/" + slug
	return authority.WithRoot(func(root *os.Root) error {
		if _, err := root.Lstat(destination); !errors.Is(err, fs.ErrNotExist) {
			return errors.New("abandon destination already exists")
		}
		for _, source := range evidence {
			if path.Dir(source) != lane {
				return errors.New("evidence escaped lane")
			}
			info, err := root.Lstat(source)
			if err != nil || prepareRefusedInfo(info) {
				return errors.New("abandon evidence changed")
			}
			name := path.Base(source)
			if prepareControlEvidenceName(name) && !info.Mode().IsRegular() {
				return errors.New("abandon control evidence changed")
			}
			if prepareStageEvidenceName(name) && !info.IsDir() {
				return errors.New("abandon staging evidence changed")
			}
		}
		if err := root.Mkdir(destination, 0o700); err != nil {
			return err
		}
		if err := syncPrepareRootDirectory(root, lane); err != nil {
			return rollbackPrepareAbandonMove(root, lane, destination, nil, err)
		}
		moved := []string{}
		for _, source := range evidence {
			target := destination + "/" + path.Base(source)
			if beforeAbandonEvidenceRename != nil {
				if err := beforeAbandonEvidenceRename(root, source, target, false); err != nil {
					return rollbackPrepareAbandonMove(root, lane, destination, moved, err)
				}
			}
			if _, err := root.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
				return rollbackPrepareAbandonMove(
					root, lane, destination, moved, errors.New("abandon evidence target appeared"),
				)
			}
			if err := root.Rename(source, target); err != nil {
				return rollbackPrepareAbandonMove(root, lane, destination, moved, err)
			}
			moved = append(moved, source)
			if err := syncPrepareRootDirectory(root, destination); err != nil {
				return rollbackPrepareAbandonMove(root, lane, destination, moved, err)
			}
			if err := syncPrepareRootDirectory(root, lane); err != nil {
				return rollbackPrepareAbandonMove(root, lane, destination, moved, err)
			}
		}
		return nil
	})
}

func rollbackPrepareAbandonMove(
	root *os.Root,
	lane, destination string,
	moved []string,
	cause error,
) error {
	var rollbackErr error
	affected := ""
	recordRollbackError := func(source string, err error) {
		if err == nil || rollbackErr != nil {
			return
		}
		rollbackErr = err
		affected = source
	}
	for index := len(moved) - 1; index >= 0; index-- {
		source := moved[index]
		target := destination + "/" + path.Base(source)
		if beforeAbandonEvidenceRename != nil {
			if err := beforeAbandonEvidenceRename(root, target, source, true); err != nil {
				recordRollbackError(source, err)
				continue
			}
		}
		if _, err := root.Lstat(source); !errors.Is(err, fs.ErrNotExist) {
			if err == nil {
				err = errors.New("abandon rollback destination is occupied")
			}
			recordRollbackError(source, err)
			continue
		}
		if err := root.Rename(target, source); err != nil {
			recordRollbackError(source, err)
		}
	}
	if rollbackErr == nil {
		if err := root.Remove(destination); err != nil && !errors.Is(err, fs.ErrNotExist) {
			recordRollbackError(destination+"/", err)
		}
	} else if _, err := root.Lstat(destination); err == nil {
		recordRollbackError(destination+"/", syncPrepareRootDirectory(root, destination))
	} else if !errors.Is(err, fs.ErrNotExist) {
		recordRollbackError(destination+"/", err)
	}
	if err := syncPrepareRootDirectory(root, lane); err != nil {
		recordRollbackError(lane+"/", err)
	}
	if affected == "" {
		affected = lane + "/"
	}
	return &prepareAbandonMoveError{
		cause:       errors.Join(cause, rollbackErr),
		entryRel:    affected,
		evidenceRel: destination + "/",
		unsafe:      rollbackErr != nil,
	}
}

func syncPrepareRootDirectory(root *os.Root, rel string) error {
	directory, err := root.Open(rel)
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	return errors.Join(syncErr, closeErr)
}

func preparePendingEvidenceName(name string) bool {
	return prepareControlEvidenceName(name) || prepareStageEvidenceName(name)
}

func prepareControlEvidenceName(name string) bool {
	switch name {
	case "journal.json", "journal.clearing.json", "index.preimage.json", "status.preimage.json":
		return true
	default:
		return false
	}
}

func prepareStageEvidenceName(name string) bool {
	return strings.HasPrefix(name, "stage-") && validPrepareHex(strings.TrimPrefix(name, "stage-"), 12)
}

func prepareAbandonedEvidenceName(name string) bool {
	return strings.HasPrefix(name, "abandoned-") && validPrepareHex(strings.TrimPrefix(name, "abandoned-"), 12)
}

func validPrepareHex(value string, size int) bool {
	if len(value) != size {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func prepareAbandonedResidueRemediation(existing []string) string {
	var b strings.Builder
	b.WriteString("Previously abandoned evidence was preserved and not touched.")
	for _, rel := range existing {
		b.WriteString(" Remove it only when no longer needed with rm -rf ")
		b.WriteString(rel)
		b.WriteString(".")
	}
	return b.String()
}

type prepareArchiveStorage struct {
	authority *intentlock.WorkspaceAuthority
	readRoot  *os.Root
}

func newPrepareArchiveStorage(authority *intentlock.WorkspaceAuthority, readRoot *os.Root) *prepareArchiveStorage {
	return &prepareArchiveStorage{authority: authority, readRoot: readRoot}
}

func (storage *prepareArchiveStorage) withRoot(fn func(*os.Root) error) error {
	if storage == nil || fn == nil {
		return errors.New("archive storage unavailable")
	}
	if storage.authority != nil {
		return storage.authority.WithRoot(fn)
	}
	if storage.readRoot == nil {
		return errors.New("archive root unavailable")
	}
	return fn(storage.readRoot)
}

func (storage *prepareArchiveStorage) CaptureIndex(indexRel string) (store.IntentArchiveIndexCapture, error) {
	var capture store.IntentArchiveIndexCapture
	err := storage.withRoot(func(root *os.Root) error {
		captured, err := capturePrepareFile(root, indexRel, intent.MaxArtifactBytes)
		if err != nil {
			return err
		}
		switch captured.state {
		case intent.StateAbsent:
			capture = store.IntentArchiveIndexCapture{
				Exists:   false,
				Raw:      []byte{},
				Identity: prepareArchiveIdentityToken(intentpub.AbsentIdentity()),
			}
			return nil
		case intent.StatePresentNonempty, intent.StatePresentEmpty:
			capture = store.IntentArchiveIndexCapture{
				Exists:   true,
				Raw:      append([]byte(nil), captured.bytes...),
				Identity: prepareArchiveIdentityToken(captured.identity),
			}
			return nil
		default:
			return &store.IntentArchiveError{
				Code:      store.IntentArchiveCodeIndexCorrupt,
				Class:     captured.state,
				ExitClass: 3,
			}
		}
	})
	return capture, err
}

func (storage *prepareArchiveStorage) EnumerateBlobs(blobsRel string) ([]string, error) {
	entries := []string{}
	err := storage.withRoot(func(root *os.Root) error {
		info, err := root.Lstat(blobsRel)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil || prepareRefusedInfo(info) || !info.IsDir() {
			return &store.IntentArchiveError{
				Code:      store.IntentArchiveCodeBlobCorrupt,
				Class:     "blobs-directory",
				ExitClass: 3,
			}
		}
		directory, err := root.Open(blobsRel)
		if err != nil {
			return err
		}
		opened, statErr := directory.Stat()
		if statErr != nil || !os.SameFile(info, opened) || !opened.IsDir() {
			_ = directory.Close()
			return &store.IntentArchiveError{
				Code:      store.IntentArchiveCodeBlobCorrupt,
				Class:     "blobs-directory-unstable",
				ExitClass: 3,
			}
		}
		names, readErr := directory.Readdirnames(-1)
		closeErr := directory.Close()
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		sort.Strings(names)
		for _, name := range names {
			if name == "." || name == ".." || strings.Contains(name, "/") {
				return &store.IntentArchiveError{
					Code:      store.IntentArchiveCodeIndexPathEscape,
					ExitClass: 3,
				}
			}
			entries = append(entries, blobsRel+"/"+name)
		}
		return nil
	})
	return entries, err
}

func (storage *prepareArchiveStorage) ProbeBlob(blobRel string) (store.IntentArchiveBlobProbe, error) {
	var probe store.IntentArchiveBlobProbe
	err := storage.withRoot(func(root *os.Root) error {
		info, err := root.Lstat(blobRel)
		if errors.Is(err, fs.ErrNotExist) {
			probe.Kind = store.IntentArchiveBlobKindAbsent
			probe.Identity = prepareArchiveIdentityToken(intentpub.AbsentIdentity())
			return nil
		}
		if err != nil {
			return err
		}
		probe.SizeBytes = info.Size()
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			probe.Kind = store.IntentArchiveBlobKindSymlink
		case info.IsDir():
			probe.Kind = store.IntentArchiveBlobKindDirectory
		case info.Mode()&os.ModeNamedPipe != 0:
			probe.Kind = store.IntentArchiveBlobKindFIFO
		case info.Mode()&os.ModeDevice != 0:
			probe.Kind = store.IntentArchiveBlobKindDevice
		case !info.Mode().IsRegular() || info.Mode()&os.ModeIrregular != 0:
			probe.Kind = store.IntentArchiveBlobKindOtherNonRegular
		default:
			captured, captureErr := capturePrepareFile(root, blobRel, intent.MaxArtifactBytes)
			if captureErr != nil || captured.state != intent.StatePresentNonempty && captured.state != intent.StatePresentEmpty {
				hash := strings.TrimSuffix(path.Base(blobRel), ".blob")
				return &store.IntentArchiveError{
					Code:      store.IntentArchiveCodeBlobCorrupt,
					Hash:      hash,
					Class:     "blob-unstable",
					ExitClass: 3,
				}
			}
			probe.Kind = store.IntentArchiveBlobKindRegular
			probe.SHA256 = captured.identity.SHA256
			probe.SizeBytes = captured.identity.Size
			probe.Identity = prepareArchiveIdentityToken(captured.identity)
			return nil
		}
		probe.Identity = store.IntentArchiveIdentityToken(
			"object:" + strconv.FormatUint(uint64(info.Mode()), 10) + ":" + strconv.FormatInt(info.Size(), 10),
		)
		return nil
	})
	return probe, err
}

func (storage *prepareArchiveStorage) PreflightIndexCAS(indexRel string, expected store.IntentArchiveIdentityToken) error {
	capture, err := storage.CaptureIndex(indexRel)
	if err != nil {
		return err
	}
	if capture.Identity != expected {
		return errors.New("archive index changed")
	}
	return nil
}

func (storage *prepareArchiveStorage) PreflightBlobRemove(blobRel string, expected store.IntentArchiveIdentityToken) error {
	probe, err := storage.ProbeBlob(blobRel)
	if err != nil {
		return err
	}
	if probe.Identity != expected {
		return errors.New("archive blob changed")
	}
	return nil
}

func (storage *prepareArchiveStorage) PublishBlob(blobRel, contentSHA256 string, data []byte) (store.IntentArchiveMutationResult, error) {
	result := store.IntentArchiveMutationResult{}
	if storage.authority == nil {
		return result, errors.New("read-only archive storage")
	}
	if !validPrepareHex(contentSHA256, 64) ||
		path.Base(blobRel) != contentSHA256+".blob" ||
		prepareSHA256(data) != contentSHA256 {
		return result, &store.IntentArchiveError{
			Code:      store.IntentArchiveCodeBlobCorrupt,
			Hash:      contentSHA256,
			Class:     "content-address-mismatch",
			ExitClass: 3,
		}
	}
	probe, err := storage.ProbeBlob(blobRel)
	if err != nil {
		return result, err
	}
	if probe.Kind == store.IntentArchiveBlobKindRegular {
		if probe.SHA256 == contentSHA256 && probe.SizeBytes == int64(len(data)) {
			result.Reused = true
			result.Phase = store.IntentArchiveStoragePhaseValidated
			return result, nil
		}
		return result, errors.New("archive blob content-address collision")
	}
	if probe.Kind != store.IntentArchiveBlobKindAbsent {
		return result, errors.New("archive blob path is not absent")
	}
	writeResult, err := intentpub.DurableWrite(storage.authority, intentpub.WriteRequest{
		Rel:          blobRel,
		Data:         data,
		Mode:         0o644,
		Expected:     prepareIdentityPointer(intentpub.AbsentIdentity()),
		MismatchCode: intentpub.CodeEntryAppeared,
		Role:         intentpub.WriteRoleOrdinaryCanonical,
	}, newPrepareIntentpubOptions())
	result.Committed = writeResult.Committed
	result.Phase = prepareArchiveStoragePhase(writeResult.Phase)
	return result, prepareArchiveStorageWriteError(err, contentSHA256, writeResult)
}

func (storage *prepareArchiveStorage) CASIndex(
	indexRel string,
	expected store.IntentArchiveIdentityToken,
	canonical []byte,
) (store.IntentArchiveMutationResult, error) {
	result := store.IntentArchiveMutationResult{}
	if storage.authority == nil {
		return result, errors.New("read-only archive storage")
	}
	identity, err := prepareArchiveTokenIdentity(expected)
	if err != nil {
		return result, err
	}
	mode := fs.FileMode(0o644)
	if identity.Exists {
		mode = fs.FileMode(identity.Mode)
	}
	writeOptions := newPrepareIntentpubOptions()
	writeOptions.BeforeRename = func(request intentpub.WriteRequest) {
		if beforeIndexRewrite != nil {
			beforeIndexRewrite(request.Rel)
		}
	}
	writeResult, err := intentpub.DurableWrite(storage.authority, intentpub.WriteRequest{
		Rel:          indexRel,
		Data:         canonical,
		Mode:         mode,
		Expected:     prepareIdentityPointer(identity),
		MismatchCode: intentpub.CodeEntryChanged,
		Role:         intentpub.WriteRoleOrdinaryCanonical,
	}, writeOptions)
	result.Committed = writeResult.Committed
	result.Phase = prepareArchiveStoragePhase(writeResult.Phase)
	return result, prepareArchiveStorageWriteError(err, "", writeResult)
}

func (storage *prepareArchiveStorage) RemoveBlob(
	blobRel string,
	expected store.IntentArchiveIdentityToken,
) (store.IntentArchiveMutationResult, error) {
	result := store.IntentArchiveMutationResult{}
	if storage.authority == nil {
		return result, errors.New("read-only archive storage")
	}
	if err := storage.PreflightBlobRemove(blobRel, expected); err != nil {
		return result, err
	}
	err := storage.authority.WithRoot(func(root *os.Root) error {
		if err := root.Remove(blobRel); err != nil {
			return err
		}
		result.Committed = true
		result.Phase = store.IntentArchiveStoragePhaseRemoved
		if err := syncPrepareRootDirectory(root, path.Dir(blobRel)); err != nil {
			return err
		}
		result.Phase = store.IntentArchiveStoragePhaseDirectorySynced
		return nil
	})
	return result, err
}

func (storage *prepareArchiveStorage) SyncDirectory(dirRel string) error {
	return storage.withRoot(func(root *os.Root) error {
		return syncPrepareRootDirectory(root, dirRel)
	})
}

func prepareArchiveIdentityToken(identity intentpub.Identity) store.IntentArchiveIdentityToken {
	if !identity.Exists {
		return store.IntentArchiveIdentityToken("absent")
	}
	return store.IntentArchiveIdentityToken(
		"file:" + identity.SHA256 + ":" +
			strconv.FormatInt(identity.Size, 10) + ":" +
			strconv.FormatUint(uint64(identity.Mode), 10),
	)
}

func prepareArchiveTokenIdentity(token store.IntentArchiveIdentityToken) (intentpub.Identity, error) {
	if token == "absent" {
		return intentpub.AbsentIdentity(), nil
	}
	parts := strings.Split(string(token), ":")
	if len(parts) != 4 || parts[0] != "file" || !validPrepareHex(parts[1], 64) {
		return intentpub.Identity{}, errors.New("archive identity token is not a regular file identity")
	}
	size, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || size < 0 {
		return intentpub.Identity{}, errors.New("archive identity size is invalid")
	}
	mode, err := strconv.ParseUint(parts[3], 10, 32)
	if err != nil || mode == 0 || mode > 0o777 {
		return intentpub.Identity{}, errors.New("archive identity mode is invalid")
	}
	return intentpub.Identity{
		Exists: true,
		SHA256: parts[1],
		Size:   size,
		Mode:   uint32(mode),
	}, nil
}

func prepareArchiveStoragePhase(phase intentpub.WritePhase) store.IntentArchiveStoragePhase {
	switch phase {
	case intentpub.WritePhaseTempWritten:
		return store.IntentArchiveStoragePhaseWritten
	case intentpub.WritePhaseTempSynced, intentpub.WritePhaseTempClosed:
		return store.IntentArchiveStoragePhaseSynced
	case intentpub.WritePhaseRenamed:
		return store.IntentArchiveStoragePhaseRenamed
	case intentpub.WritePhaseDirectorySynced, intentpub.WritePhaseVerified:
		return store.IntentArchiveStoragePhaseDirectorySynced
	default:
		return store.IntentArchiveStoragePhaseNone
	}
}

func prepareArchiveStorageWriteError(
	err error,
	hash string,
	result intentpub.WriteResult,
) error {
	if err == nil {
		return nil
	}
	var typed *intentpub.Error
	if !errors.As(err, &typed) {
		return err
	}
	class := string(typed.Code)
	if typed.Class != "" {
		class += ":" + typed.Class
	}
	exit := typed.ExitClass
	committed := typed.Committed || result.Committed
	if exit == 0 {
		if committed {
			exit = 5
		} else {
			exit = 3
		}
	}
	return &store.IntentArchiveError{
		Code:      store.IntentArchiveCodeStorageFailed,
		Hash:      hash,
		Class:     class,
		Detail:    "the rooted archive publication did not complete safely",
		ExitClass: exit,
		Committed: committed,
	}
}

func prepareIdentityPointer(identity intentpub.Identity) *intentpub.Identity {
	return &identity
}

func preparePendingArchiveHashes(storage store.IntentArchiveStorage, slug string) ([]string, error) {
	indexRel, err := store.IntentArchiveIndexRel(slug)
	if err != nil {
		return nil, err
	}
	capture, err := storage.CaptureIndex(indexRel)
	if err != nil {
		return nil, err
	}
	if !capture.Exists {
		return []string{}, nil
	}
	index, err := store.DecodeIntentArchiveIndex(capture.Raw, slug)
	if err != nil {
		return nil, err
	}
	return store.PendingIntentArchiveHashes(index), nil
}

func prepareArchivePreflight(
	storage store.IntentArchiveStorage,
	slug string,
	plan preparePlan,
) (store.IntentArchiveSnapshot, store.IntentArchiveAppendPlan, bool, error) {
	snapshot, err := store.CaptureIntentArchive(storage, slug)
	hasArchive := len(plan.replacements) != 0
	if err != nil {
		return snapshot, store.IntentArchiveAppendPlan{}, hasArchive, err
	}
	if hasArchive {
		appendPlan, buildErr := store.BuildIntentArchiveAppendPlan(snapshot, plan.replacements)
		return snapshot, appendPlan, true, buildErr
	}
	return snapshot, store.IntentArchiveAppendPlan{}, false, prepareValidateArchiveSnapshot(snapshot, true)
}

func cleanupPrepareUnarmedLane(
	authority *intentlock.WorkspaceAuthority,
	slug string,
	options intentpub.Options,
) error {
	_, err := intentpub.CleanupUnarmedLane(authority, slug, options)
	return err
}

func preparePendingPurgeCommand(slug string, hashes []string) string {
	sorted := append([]string(nil), hashes...)
	sort.Strings(sorted)
	var b strings.Builder
	b.WriteString("tpatch feature intent-archive purge ")
	b.WriteString(slug)
	for _, hash := range sorted {
		b.WriteString(" --blob ")
		b.WriteString(hash)
	}
	b.WriteString(" --yes")
	return b.String()
}

func prepareValidateArchiveSnapshot(snapshot store.IntentArchiveSnapshot, includePending bool) error {
	if includePending && len(snapshot.Inspection.PendingHashes) != 0 {
		return &store.IntentArchiveError{
			Code:      store.IntentArchiveCodeRecoveryPending,
			Hash:      snapshot.Inspection.PendingHashes[0],
			ExitClass: 3,
		}
	}
	if len(snapshot.Inspection.Classes) == 0 {
		return nil
	}
	class := snapshot.Inspection.Classes[0]
	code := store.IntentArchiveCodeIndexStorageInconsistent
	switch class.Class {
	case store.IntentArchiveRepairCorruptObject:
		code = store.IntentArchiveCodeBlobCorrupt
	case store.IntentArchiveRepairDanglingReference:
		code = store.IntentArchiveCodeBlobDangling
	}
	typed := &store.IntentArchiveError{
		Code:      code,
		Class:     string(class.Class),
		ExitClass: 3,
	}
	if len(class.Hashes) != 0 {
		typed.Hash = class.Hashes[0]
	}
	return typed
}

func prepareArchiveOrphanHashes(snapshot store.IntentArchiveSnapshot) []string {
	hashes := make([]string, 0, len(snapshot.Inspection.Orphans))
	for _, orphan := range snapshot.Inspection.Orphans {
		if validPrepareHex(orphan.Hash, 64) {
			hashes = append(hashes, orphan.Hash)
		}
	}
	sort.Strings(hashes)
	return hashes
}

func applyPrepareArchiveObservation(
	report preparePublishReport,
	snapshot store.IntentArchiveSnapshot,
) preparePublishReport {
	report.OrphanBlobs = prepareArchiveOrphanHashes(snapshot)
	if len(report.OrphanBlobs) != 0 {
		report = appendPrepareOrphanAdvisory(report)
	}
	return report
}

func appendPrepareOrphanAdvisory(report preparePublishReport) preparePublishReport {
	if len(report.OrphanBlobs) == 0 {
		return report
	}
	for _, advisory := range report.Advisories {
		if advisory.Code == "archive-orphan-blobs" {
			return report
		}
	}
	report.Advisories = append(report.Advisories, prepareAdvisory(
		"archive-orphan-blobs", "",
		fmt.Sprintf(
			"%d orphan archive blob(s) remain; remove them with tpatch feature intent-archive purge %s --orphans --yes.",
			len(report.OrphanBlobs), report.Slug,
		),
	))
	return report
}

func refreshPrepareOrphanTruth(
	authority *intentlock.WorkspaceAuthority,
	slug string,
	report preparePublishReport,
) preparePublishReport {
	snapshot, err := store.CaptureIntentArchive(newPrepareArchiveStorage(authority, nil), slug)
	if err == nil {
		report.OrphanBlobs = prepareArchiveOrphanHashes(snapshot)
	}
	return report
}

func appendPrepareStagingAdvisory(report preparePublishReport, stageRel string) preparePublishReport {
	if stageRel == "" {
		return report
	}
	for _, advisory := range report.Advisories {
		if advisory.Code == "staging-retained" {
			return report
		}
	}
	report.Advisories = append(report.Advisories, prepareAdvisory(
		"staging-retained", "",
		"Staged canonical output was retained at "+stageRel+"; the next successful run removes it.",
	))
	return report
}

func prepareStagingFailure(
	report preparePublishReport,
	err error,
	stageRel string,
) (preparePublishReport, int) {
	code := prepareIntentpubCode(err, "entry-changed")
	exit := 5
	var typed *intentpub.Error
	if errors.As(err, &typed) && typed.ExitClass > exit {
		exit = typed.ExitClass
	}
	report = refusePrepare(report, code, "")
	report = appendPrepareStagingAdvisory(report, stageRel)
	if exit >= 6 {
		report.Outcome = "recovery-refused"
	} else {
		report.Outcome = "rolled-back"
	}
	return report, exit
}

func prepareRawPreimageFailure(
	report preparePublishReport,
	err error,
	stageRel, rawRel string,
) (preparePublishReport, int) {
	report, exit := prepareStagingFailure(report, err, stageRel)
	report = appendPrepareOrphanAdvisory(report)
	var typed *intentpub.Error
	if errors.As(err, &typed) && typed.Committed && rawRel != "" && report.Refusal != nil {
		report.Refusal.Message += " Retained raw preimage evidence: " + rawRel + "."
	}
	return report, exit
}

func prepareStoreArchiveFailure(report preparePublishReport, err error, afterWrite bool) preparePublishReport {
	code := "archive-index-corrupt"
	var typed *store.IntentArchiveError
	if errors.As(err, &typed) {
		code = string(typed.Code)
		if typed.Code == store.IntentArchiveCodeStorageFailed {
			switch {
			case typed.ExitClass >= 6 && typed.Committed:
				code = "post-publication-divergence"
			case typed.ExitClass >= 5 && strings.Contains(typed.Class, "index"):
				code = "archive-index-changed"
			case typed.ExitClass >= 5:
				code = "entry-changed"
			case strings.Contains(typed.Class, "index"):
				code = "archive-index-corrupt"
			default:
				code = "archive-blob-corrupt"
			}
		}
	}
	report = refusePrepare(report, code, "")
	if typed != nil {
		subject := ""
		if typed.ArtifactID != "" {
			subject = " artifact " + string(typed.ArtifactID)
		}
		if typed.Hash != "" {
			subject += " hash " + typed.Hash
		}
		if typed.Class != "" {
			subject += " class " + typed.Class
		}
		if subject != "" {
			report.Refusal.Message += " Affected" + subject + "."
		}
		switch typed.Code {
		case store.IntentArchiveCodeRecoveryPending:
			if typed.Hash != "" {
				retry := preparePendingPurgeCommand(report.Slug, []string{typed.Hash})
				report.Refusal.Remediation = "Resume the owning archive purge transaction."
				report.Refusal.Retry = retry
				report.Refusal.RetryCWD = "workspace-root"
			}
		case store.IntentArchiveCodeBlobDangling:
			report.Refusal.Remediation =
				"Run tpatch feature intent-archive purge " + report.Slug +
					" --blob " + typed.Hash + " --yes from the workspace root."
			report.Refusal.Retry = "tpatch feature intent-archive purge " + report.Slug +
				" --blob " + typed.Hash + " --yes"
			report.Refusal.RetryCWD = "workspace-root"
		case store.IntentArchiveCodeIndexStorageInconsistent:
			if typed.Class == string(store.IntentArchiveRepairUnreferencedResidue) {
				report.Refusal.Remediation =
					"Run tpatch feature intent-archive purge " + report.Slug +
						" --orphans --yes from the workspace root."
			} else if typed.Hash != "" {
				report.Refusal.Remediation =
					"Run tpatch feature intent-archive purge " + report.Slug +
						" --blob " + typed.Hash + " --yes from the workspace root."
			}
		case store.IntentArchiveCodeBlobCorrupt:
			if typed.Hash != "" {
				blob := prepareFeatureRel(report.Slug) + "/artifacts/intent-archive/blobs/" + typed.Hash + ".blob"
				report.Refusal.Remediation =
					"Warning: removing the managed object is destructive and has no undo. " +
						"Run rm -rf -- " + blob +
						", then run tpatch feature intent-archive purge " + report.Slug +
						" --blob " + typed.Hash + " --yes, or restore the exact hash-correct blob and retry. " +
						"Committed bytes can remain in this repository's Git history."
			}
		case store.IntentArchiveCodeContentSensitive:
			report.Refusal.Message =
				"Prior bytes for artifact " + string(typed.ArtifactID) +
					" matched sensitive classes " + typed.Class + "."
			report.Refusal.Remediation =
				"Remove the sensitive material from that artifact and retry."
		}
	}
	if afterWrite {
		if typed != nil && typed.ExitClass >= 6 {
			report.Outcome = "recovery-refused"
		} else {
			report.Outcome = "rolled-back"
		}
	}
	return report
}

func prepareArchiveExit(err error, fallback int) int {
	var typed *store.IntentArchiveError
	if errors.As(err, &typed) && typed.ExitClass != 0 {
		return typed.ExitClass
	}
	return fallback
}

func prepareIntentpubFailure(
	report preparePublishReport,
	result intentpub.Result,
	err error,
) preparePublishReport {
	code := prepareIntentpubCode(err, "post-publication-divergence")
	report = refusePrepare(report, code, "")
	report.OrphanBlobs = append([]string(nil), result.Orphans...)
	var typed *intentpub.Error
	if errors.As(err, &typed) && typed.ArtifactID != "" {
		entryRel := prepareCanonicalRel(report.Slug, typed.ArtifactID)
		if entryRel != "" {
			report.Refusal.Message += " Affected entry: " + entryRel + "."
		}
		if preimageRel := prepareFailurePreimageRel(report, typed.ArtifactID); preimageRel != "" {
			report.Refusal.Message += " Prior-byte evidence: " + preimageRel + "."
		}
	}
	if result.ExitClass == 5 {
		report.Outcome = "rolled-back"
	} else {
		report.Outcome = "recovery-refused"
	}
	return report
}

func prepareFailurePreimageRel(report preparePublishReport, id intentpub.ArtifactID) string {
	lane := ".tpatch/local/intent-prepare/" + report.Slug + "/"
	switch id {
	case intentpub.ArtifactArchiveIndex:
		return lane + "index.preimage.json"
	case intentpub.ArtifactStatus:
		return lane + "status.preimage.json"
	}
	reportID := prepareIntentpubReportID(id)
	for _, artifact := range report.Artifacts {
		if artifact.ID == reportID && artifact.ArchivedBlob != "" && report.Archive != nil {
			return report.Archive.BlobsDir + "/" + artifact.ArchivedBlob + ".blob"
		}
	}
	return lane + "journal.json"
}

func prepareIntentpubCode(err error, fallback string) string {
	var typed *intentpub.Error
	if !errors.As(err, &typed) {
		return fallback
	}
	switch typed.Code {
	case intentpub.CodeEntryAppeared:
		return "entry-appeared"
	case intentpub.CodeEntryChanged:
		return "entry-changed"
	case intentpub.CodeUndoCASMismatch:
		return "undo-cas-mismatch"
	case intentpub.CodePostPublicationDivergence:
		return "post-publication-divergence"
	case intentpub.CodeWorkspaceRootChanged:
		return "workspace-root-changed"
	case intentpub.CodeWorkspaceRootReplacedAfterPublication:
		return "workspace-root-replaced-after-publication"
	case intentpub.CodeJournalCorrupt:
		return "journal-corrupt"
	case intentpub.CodeJournalVersionMismatch:
		return "journal-version-mismatch"
	case intentpub.CodeJournalForeign:
		return "journal-foreign"
	case intentpub.CodeJournalPathEscape:
		return "journal-path-escape"
	case intentpub.CodeJournalForged:
		return "journal-forged"
	case intentpub.CodeJournalPending:
		return "journal-corrupt"
	case intentpub.CodeRecoveryDivergent:
		return "recovery-divergent"
	case intentpub.CodeCleanupFailed:
		return "recovery-divergent"
	case intentpub.CodeNonRegular, intentpub.CodeIdentityUnstable,
		intentpub.CodeFileOversize, intentpub.CodeSourceInvalid:
		return "recovery-divergent"
	case intentpub.CodeStagedOutputInvalid:
		return "staged-output-invalid"
	default:
		return fallback
	}
}

func prepareIntentpubArtifactID(id string) (intentpub.ArtifactID, bool) {
	switch id {
	case "analysis":
		return intentpub.ArtifactAnalysis, true
	case "spec":
		return intentpub.ArtifactSpec, true
	case "exploration":
		return intentpub.ArtifactExploration, true
	case "analysis_sidecar":
		return intentpub.ArtifactAnalysisSidecar, true
	default:
		return "", false
	}
}

func prepareIntentpubReportID(id intentpub.ArtifactID) string {
	switch id {
	case intentpub.ArtifactAnalysis:
		return "analysis"
	case intentpub.ArtifactSpec:
		return "spec"
	case intentpub.ArtifactExploration:
		return "exploration"
	case intentpub.ArtifactAnalysisSidecar:
		return "analysis_sidecar"
	case intentpub.ArtifactArchiveIndex:
		return "archive_index"
	case intentpub.ArtifactStatus:
		return "status"
	default:
		return ""
	}
}

func prepareContainsArtifactID(values []intentpub.ArtifactID, wanted intentpub.ArtifactID) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func prepareArtifactIDPaths(slug string, ids []intentpub.ArtifactID) []string {
	paths := make([]string, 0, len(ids))
	for _, id := range ids {
		rel, err := intentpub.CanonicalPath(slug, id)
		if err == nil {
			paths = append(paths, rel)
		}
	}
	return paths
}

func prepareRetryCommand(slug string, options prepareOptions) string {
	var args []string
	args = append(args, "tpatch", "prepare", slug)
	switch options.mode {
	case prepareModeManual:
		args = append(args, "--manual")
	case prepareModeRegenerate:
		args = append(args, "--regenerate")
	}
	if options.allowHeuristic {
		args = append(args, "--allow-heuristic")
	}
	if options.timeoutChanged {
		args = append(args, "--timeout", options.timeout.String())
	}
	if options.phaseChanged {
		args = append(args, "--timeout-phase", options.timeoutPhase.String())
	}
	if options.noRetryChanged && options.noRetry {
		args = append(args, "--no-retry")
	}
	if options.asJSON {
		args = append(args, "--json")
	}
	if options.quiet {
		args = append(args, "--quiet")
	}
	return strings.Join(args, " ")
}

func prepareRedactionRefusal(inputs []store.IntentArchiveReplacementInput) (string, string, string) {
	if beforeRedactionScan != nil {
		beforeRedactionScan()
	}
	for _, input := range inputs {
		if classes := redact.Scan(input.PriorBytes); len(classes) != 0 {
			sorted := append([]string(nil), classes...)
			sort.Strings(sorted)
			return "archive-content-refused-sensitive", string(input.ArtifactID), strings.Join(sorted, ",")
		}
	}
	return "", "", ""
}

type prepareGeneratedArtifact struct {
	id            intentpub.ArtifactID
	data          []byte
	note          workflow.GenNote
	deadlineScope string
}

type prepareGeneratedBundle struct {
	artifacts []prepareGeneratedArtifact
	failure   *prepareGeneratedFailure
}

type prepareGeneratedFailure struct {
	artifactID    intentpub.ArtifactID
	note          workflow.GenNote
	deadlineScope string
}

func (bundle prepareGeneratedBundle) find(id intentpub.ArtifactID) (prepareGeneratedArtifact, bool) {
	for _, artifact := range bundle.artifacts {
		if artifact.id == id {
			return artifact, true
		}
	}
	return prepareGeneratedArtifact{}, false
}

func generatePrepareBundle(
	cmd *cobra.Command,
	authority *intentlock.WorkspaceAuthority,
	repoStore *store.Store,
	slug string,
	request []byte,
	readState prepareReadState,
	plan preparePlan,
	options prepareOptions,
	prov provider.Provider,
	provCfg provider.Config,
) (prepareGeneratedBundle, error) {
	result := prepareGeneratedBundle{artifacts: []prepareGeneratedArtifact{}}
	merged, _ := repoStore.LoadMergedConfig()
	maxRetries := merged.MaxRetries
	if maxRetries < 0 || options.noRetry {
		maxRetries = 0
	}
	policy := workflow.GeneratorAuthorityPolicy{}
	if options.mode == prepareModeRegenerate {
		policy.Authority = workflow.GeneratorAuthorityRegenerate
		policy.AllowHeuristic = options.allowHeuristic
	}

	fileTree3, fileTree4, guidance := "", "", ""
	if err := authority.WithRoot(func(root *os.Root) error {
		fileTree3 = prepareFileTree(root, 3)
		fileTree4 = prepareFileTree(root, 4)
		guidance = prepareGuidance(root)
		return nil
	}); err != nil {
		return result, err
	}

	total := options.timeout
	if total <= 0 {
		total = 180 * time.Second
	}
	phase := options.timeoutPhase
	if phase <= 0 {
		phase = 90 * time.Second
	}
	totalCtx, totalCancel := context.WithTimeout(context.Background(), total)
	defer totalCancel()
	totalDeadline, _ := totalCtx.Deadline()
	phaseContext := func() (context.Context, context.CancelFunc) {
		deadline := totalDeadline
		if candidate := time.Now().Add(phase); candidate.Before(deadline) {
			deadline = candidate
		}
		return context.WithDeadline(totalCtx, deadline)
	}
	deadlineScope := func(note workflow.GenNote) string {
		if note.DeadlineClass != workflow.GenDeadlineExceeded {
			return ""
		}
		if totalCtx.Err() != nil {
			return "total"
		}
		return "per-phase"
	}

	effectiveAnalysis := string(readState.analysis.bytes)
	effectiveSidecar := ""
	if readState.sidecar.state == intent.StatePresentNonempty {
		effectiveSidecar = string(readState.sidecar.bytes)
	}
	effectiveSpec := string(readState.spec.bytes)

	if prepareContainsArtifactID(plan.generated, intentpub.ArtifactAnalysis) {
		prepareProgress(cmd, options, "[1/3] Generating analysis…")
		ctx, cancel := phaseContext()
		analysis, note, err := workflow.GenerateAnalysis(ctx, workflow.AnalysisInput{
			Slug:       slug,
			Request:    string(request),
			FileTree:   fileTree3,
			Guidance:   guidance,
			Provider:   prov,
			Config:     provCfg,
			MaxRetries: maxRetries,
			Authority:  policy,
		})
		cancel()
		if err != nil {
			result.failure = &prepareGeneratedFailure{
				artifactID:    intentpub.ArtifactAnalysis,
				note:          note,
				deadlineScope: deadlineScope(note),
			}
			return result, err
		}
		analysisMD := []byte(workflow.RenderAnalysisMD(analysis, slug))
		sidecar, err := json.MarshalIndent(analysis, "", "  ")
		if err != nil {
			return result, err
		}
		sidecar = append(sidecar, '\n')
		result.artifacts = append(result.artifacts,
			prepareGeneratedArtifact{
				id: intentpub.ArtifactAnalysis, data: analysisMD, note: note,
				deadlineScope: deadlineScope(note),
			},
			prepareGeneratedArtifact{
				id: intentpub.ArtifactAnalysisSidecar, data: sidecar, note: note,
				deadlineScope: deadlineScope(note),
			},
		)
		effectiveAnalysis = string(analysisMD)
		effectiveSidecar = string(sidecar)
	}

	if prepareContainsArtifactID(plan.generated, intentpub.ArtifactSpec) {
		prepareProgress(cmd, options, "[2/3] Generating specification…")
		ctx, cancel := phaseContext()
		spec, note, err := workflow.GenerateSpec(ctx, workflow.DefineInput{
			Slug:         slug,
			Request:      string(request),
			AnalysisMD:   effectiveAnalysis,
			AnalysisJSON: effectiveSidecar,
			Provider:     prov,
			Config:       provCfg,
			MaxRetries:   maxRetries,
			Authority:    policy,
		})
		cancel()
		if err != nil {
			result.failure = &prepareGeneratedFailure{
				artifactID:    intentpub.ArtifactSpec,
				note:          note,
				deadlineScope: deadlineScope(note),
			}
			return result, err
		}
		result.artifacts = append(result.artifacts,
			prepareGeneratedArtifact{
				id: intentpub.ArtifactSpec, data: []byte(spec), note: note,
				deadlineScope: deadlineScope(note),
			},
		)
		effectiveSpec = spec
	}

	if prepareContainsArtifactID(plan.generated, intentpub.ArtifactExploration) {
		prepareProgress(cmd, options, "[3/3] Generating exploration…")
		ctx, cancel := phaseContext()
		exploration, note, err := workflow.GenerateExploration(ctx, workflow.ExploreInput{
			Slug:       slug,
			Request:    string(request),
			AnalysisMD: effectiveAnalysis,
			SpecMD:     effectiveSpec,
			FileTree:   fileTree4,
			Provider:   prov,
			Config:     provCfg,
			MaxRetries: maxRetries,
			Authority:  policy,
		})
		cancel()
		if err != nil {
			result.failure = &prepareGeneratedFailure{
				artifactID:    intentpub.ArtifactExploration,
				note:          note,
				deadlineScope: deadlineScope(note),
			}
			return result, err
		}
		result.artifacts = append(result.artifacts,
			prepareGeneratedArtifact{
				id: intentpub.ArtifactExploration, data: []byte(exploration), note: note,
				deadlineScope: deadlineScope(note),
			},
		)
	}
	return result, nil
}

func prepareFileTree(root *os.Root, maxDepth int) string {
	var builder strings.Builder
	prepareWalkTree(root, ".", "", 0, maxDepth, &builder)
	return builder.String()
}

func prepareWalkTree(root *os.Root, rel, prefix string, depth, maxDepth int, builder *strings.Builder) {
	if depth >= maxDepth {
		return
	}
	directory, err := root.Open(rel)
	if err != nil {
		return
	}
	entries, readErr := directory.ReadDir(-1)
	_ = directory.Close()
	if readErr != nil {
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		name := entry.Name()
		if name == "." || name == ".." || strings.Contains(name, "/") {
			continue
		}
		child := name
		if rel != "." {
			child = rel + "/" + name
		}
		info, statErr := root.Lstat(child)
		if statErr != nil || prepareRefusedInfo(info) {
			continue
		}
		if info.IsDir() && prepareIgnoredTreeDirectory(name) {
			continue
		}
		builder.WriteString(prefix)
		builder.WriteString(name)
		if info.IsDir() {
			builder.WriteString("/\n")
			prepareWalkTree(root, child, prefix+"  ", depth+1, maxDepth, builder)
		} else {
			builder.WriteByte('\n')
		}
	}
}

func prepareIgnoredTreeDirectory(name string) bool {
	switch name {
	case "node_modules", ".git", ".tpatch", "dist", "build", "__pycache__", ".next":
		return true
	default:
		return false
	}
}

func prepareGuidance(root *os.Root) string {
	parts := []string{}
	for _, name := range []string{"PATCHING.md", "CONTRIBUTING.md", "AGENTS.md", "CLAUDE.md"} {
		captured, err := capturePrepareFile(root, name, intent.MaxArtifactBytes)
		if err != nil || captured.state != intent.StatePresentNonempty {
			continue
		}
		parts = append(parts, "### "+name+"\n\n"+string(captured.bytes))
	}
	return strings.Join(parts, "\n\n---\n\n")
}

func prepareGenerationFailure(
	report preparePublishReport,
	generated prepareGeneratedBundle,
) preparePublishReport {
	report = refusePrepare(report, "regenerate-generation-failed", "")
	if generated.failure != nil &&
		generated.failure.note.DeadlineClass == workflow.GenDeadlineExceeded {
		scope := generated.failure.deadlineScope
		if scope == "" {
			scope = "generation"
		}
		artifactID := prepareIntentpubReportID(generated.failure.artifactID)
		report.Refusal.Message = "The " + scope + " deadline expired while generating artifact " +
			artifactID + "."
	}
	report.Outcome = "rolled-back"
	return report
}

func retainPrepareGeneratedStage(
	authority *intentlock.WorkspaceAuthority,
	slug string,
	state prepareReadState,
	generated prepareGeneratedBundle,
	options intentpub.Options,
) (string, error) {
	if len(generated.artifacts) == 0 {
		return "", nil
	}
	inputs := make([]intentpub.StageInput, 0, len(generated.artifacts))
	for _, artifact := range generated.artifacts {
		preimage := prepareCapturedForID(state, artifact.id)
		mode := fs.FileMode(0o644)
		if preimage.identity.Exists {
			mode = fs.FileMode(preimage.identity.Mode)
		}
		inputs = append(inputs, intentpub.StageInput{
			ArtifactID: artifact.id,
			Rel:        prepareStageBase(artifact.id),
			Data:       artifact.data,
			Mode:       mode,
		})
	}
	staged, err := intentpub.Stage(authority, slug, inputs, options)
	if err != nil {
		return staged.StageRel, err
	}
	return staged.StageRel, nil
}

func applyPrepareGenerationReport(
	report preparePublishReport,
	generated prepareGeneratedBundle,
	options prepareOptions,
) preparePublishReport {
	totalDeadlineFallbacks := 0
	regenerateFallback := false
	for _, generatedArtifact := range generated.artifacts {
		reportID := prepareIntentpubReportID(generatedArtifact.id)
		for index := range report.Artifacts {
			if report.Artifacts[index].ID != reportID {
				continue
			}
			report.Artifacts[index].Generator = string(generatedArtifact.note.Generator)
			if report.Artifacts[index].Disposition == "untouched" ||
				report.Artifacts[index].Disposition == "absent-optional" {
				report.Artifacts[index].Disposition = "generated"
			}
		}
		if generatedArtifact.id != intentpub.ArtifactAnalysisSidecar {
			for _, advisory := range generatedArtifact.note.Advisories {
				switch advisory {
				case workflow.GenAdvisoryProviderNotConfigured:
					report.Advisories = append(report.Advisories, prepareAdvisory(
						"provider-not-configured", reportID,
						"No provider was configured, so the heuristic generator was used. Configure one with tpatch provider set and verify it with tpatch provider check.",
					))
				case workflow.GenAdvisoryProviderFallbackHeuristic:
					report.Advisories = append(report.Advisories, prepareAdvisory(
						"provider-fallback-heuristic", reportID,
						"The provider call failed and the heuristic generator was used instead. Re-run after fixing the provider to replace it.",
					))
				case workflow.GenAdvisoryProviderDeadlineHeuristic:
					if generatedArtifact.deadlineScope == "total" {
						totalDeadlineFallbacks++
					}
					scope := generatedArtifact.deadlineScope
					if scope == "" {
						scope = "generation"
					}
					report.Advisories = append(report.Advisories, prepareAdvisory(
						"provider-deadline-heuristic", reportID,
						"The "+scope+" deadline expired and the heuristic generator was used instead.",
					))
				case workflow.GenAdvisoryRegenerateHeuristicAllowed:
					regenerateFallback = true
				}
			}
		}
		if generatedArtifact.id == intentpub.ArtifactAnalysisSidecar &&
			generatedArtifact.note.Generator == workflow.GeneratorHeuristic {
			report.Advisories = append(report.Advisories, prepareAdvisory(
				"heuristic-mode-recorded-in-sidecar", reportID,
				"The generated analysis sidecar records heuristic_mode: true.",
			))
		}
	}
	if totalDeadlineFallbacks >= 2 {
		report.Advisories = append(report.Advisories, prepareAdvisory(
			"provider-deadline-cascade", "",
			fmt.Sprintf("One total deadline expiry caused %d heuristic fallbacks.", totalDeadlineFallbacks),
		))
	}
	if options.mode == prepareModeRegenerate && options.allowHeuristic && regenerateFallback {
		report.Advisories = append(report.Advisories, prepareAdvisory(
			"regenerate-heuristic-allowed", "",
			"--allow-heuristic explicitly permitted hand-authored content to be replaced by heuristic output during regeneration.",
		))
	}
	return report
}

func publishPrepareStatusOnly(
	authority *intentlock.WorkspaceAuthority,
	repoRoot string,
	state prepareReadState,
	statusBytes []byte,
	report preparePublishReport,
) (preparePublishReport, int) {
	if beforeManualStatusCAS != nil {
		beforeManualStatusCAS()
	}
	if err := authority.ValidateOriginalPath(false); err != nil {
		report = refusePrepare(report, "workspace-root-changed", "")
		report.Outcome = "rolled-back"
		return report, 5
	}
	writeResult, err := intentpub.DurableWrite(authority, intentpub.WriteRequest{
		Rel:           prepareFeatureRel(report.Slug) + "/status.json",
		Data:          statusBytes,
		Mode:          fs.FileMode(state.status.identity.Mode),
		Expected:      prepareIdentityPointer(state.status.identity),
		MismatchCode:  intentpub.CodeEntryChanged,
		ArtifactID:    intentpub.ArtifactStatus,
		RequireParent: true,
		Role:          intentpub.WriteRoleCanonicalStatus,
	}, newPrepareIntentpubOptions())
	if err != nil {
		code := "status-changed"
		var typed *intentpub.Error
		exit := 5
		if errors.As(err, &typed) {
			if typed.Committed {
				code = "post-publication-divergence"
			} else if typed.ExitClass >= 6 {
				code = prepareIntentpubCode(err, "status-changed")
			}
			if typed.ExitClass > exit {
				exit = typed.ExitClass
			}
		}
		report = refusePrepare(report, code, "")
		if writeResult.Committed || exit >= 6 {
			report.Outcome = "recovery-refused"
			return report, exit
		}
		report.Outcome = "rolled-back"
		return report, exit
	}
	if err := authority.ValidateOriginalPath(true); err != nil {
		report = refusePrepare(report, "workspace-root-replaced-after-publication", "")
		report.Outcome = "recovery-refused"
		return report, 6
	}
	report.Outcome = "published"
	report.FeatureState = intent.FeatureStateDefined
	refreshPrepareFeaturesIndex(authority, repoRoot, &report)
	return report, 0
}

type prepareStatusField struct {
	name string
	raw  json.RawMessage
}

func prepareStatusBytes(raw []byte, notes string) ([]byte, error) {
	var status store.FeatureStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, err
	}
	fields, err := decodePrepareStatusFields(raw)
	if err != nil {
		return nil, err
	}
	updates := []prepareStatusField{
		{name: "state", raw: mustPrepareJSONRaw(string(store.StateDefined))},
		{name: "updated_at", raw: mustPrepareJSONRaw(prepareNow().UTC().Format(time.RFC3339))},
		{name: "last_command", raw: mustPrepareJSONRaw("prepare")},
		{name: "notes", raw: mustPrepareJSONRaw(notes)},
	}
	for _, update := range updates {
		found := false
		for index := range fields {
			if fields[index].name == update.name {
				fields[index].raw = update.raw
				found = true
				break
			}
		}
		if !found {
			fields = append(fields, update)
		}
	}
	order := []string{
		"id", "slug", "title", "state", "compatibility", "requested_at",
		"updated_at", "last_command", "notes", "apply", "reconcile",
		"depends_on", "verify", "rejection", "rejection_history",
	}
	sorted := make([]prepareStatusField, 0, len(fields))
	used := make([]bool, len(fields))
	for _, name := range order {
		for index, field := range fields {
			if !used[index] && field.name == name {
				sorted = append(sorted, field)
				used[index] = true
				break
			}
		}
	}
	for index, field := range fields {
		if !used[index] {
			sorted = append(sorted, field)
		}
	}
	return encodePrepareStatusFields(sorted)
}

func decodePrepareStatusFields(raw []byte) ([]prepareStatusField, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return nil, errors.New("status is not an object")
	}
	fields := []prepareStatusField{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		name, ok := token.(string)
		if !ok {
			return nil, errors.New("status key is not a string")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		found := false
		for index := range fields {
			if fields[index].name == name {
				fields[index].raw = append(json.RawMessage(nil), value...)
				found = true
				break
			}
		}
		if !found {
			fields = append(fields, prepareStatusField{name: name, raw: append(json.RawMessage(nil), value...)})
		}
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return nil, errors.New("status object is incomplete")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("status has trailing content")
	}
	return fields, nil
}

func encodePrepareStatusFields(fields []prepareStatusField) ([]byte, error) {
	var compact bytes.Buffer
	compact.WriteByte('{')
	for index, field := range fields {
		if index != 0 {
			compact.WriteByte(',')
		}
		name, _ := json.Marshal(field.name)
		compact.Write(name)
		compact.WriteByte(':')
		compact.Write(field.raw)
	}
	compact.WriteByte('}')
	var indented bytes.Buffer
	if err := json.Indent(&indented, compact.Bytes(), "", "  "); err != nil {
		return nil, err
	}
	indented.WriteByte('\n')
	return indented.Bytes(), nil
}

func mustPrepareJSONRaw(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

func refreshPrepareFeaturesIndex(
	authority *intentlock.WorkspaceAuthority,
	repoRoot string,
	report *preparePublishReport,
) {
	data, err := renderPrepareFeaturesIndex(&store.Store{Root: repoRoot})
	if err == nil {
		mode := fs.FileMode(0o644)
		identity, captureErr := intentpub.CaptureIdentity(
			authority, ".tpatch/FEATURES.md",
			newPrepareIntentpubOptions(),
		)
		if captureErr != nil {
			err = captureErr
		} else {
			if identity.Exists {
				mode = fs.FileMode(identity.Mode)
			}
			_, err = intentpub.DurableWrite(authority, intentpub.WriteRequest{
				Rel:      ".tpatch/FEATURES.md",
				Data:     data,
				Mode:     mode,
				Expected: prepareIdentityPointer(identity),
				Role:     intentpub.WriteRoleOrdinaryCanonical,
			}, newPrepareIntentpubOptions())
		}
	}
	if err != nil && report != nil {
		report.Advisories = append(report.Advisories, prepareAdvisory(
			"features-index-refresh-failed", "",
			"status.json is authoritative; the best-effort FEATURES.md refresh failed and a later transition will retry it.",
		))
	}
}

func renderPrepareFeaturesIndex(repoStore *store.Store) ([]byte, error) {
	features, err := repoStore.ListFeatures()
	if err != nil {
		return nil, err
	}
	active := make([]store.FeatureStatus, 0, len(features))
	unapplied := []store.FeatureStatus{}
	rejected := []store.FeatureStatus{}
	for _, feature := range features {
		switch feature.State {
		case store.StateUnapplied:
			unapplied = append(unapplied, feature)
		case store.StateRejected:
			rejected = append(rejected, feature)
		default:
			active = append(active, feature)
		}
	}
	var builder strings.Builder
	builder.WriteString("# Tracked Features\n\n")
	if len(active) == 0 && len(unapplied) == 0 && len(rejected) == 0 {
		builder.WriteString("*No features yet. Run `tpatch add <description>` to add one.*\n")
	} else if len(active) == 0 {
		builder.WriteString("*No active features.*\n")
	} else {
		builder.WriteString("| Slug | Title | State | Compatibility |\n")
		builder.WriteString("|------|-------|-------|---------------|\n")
		for _, feature := range active {
			fmt.Fprintf(&builder, "| `%s` | %s | %s | %s |\n",
				feature.Slug, feature.Title, feature.State, feature.Compatibility)
		}
	}
	if len(unapplied) > 0 {
		builder.WriteString("\n## Unapplied\n\n")
		builder.WriteString("| Slug | Title | State | Note |\n")
		builder.WriteString("|------|-------|-------|------|\n")
		for _, feature := range unapplied {
			fmt.Fprintf(&builder, "| `%s` | %s | %s | %s |\n",
				feature.Slug, feature.Title, feature.State, prepareSingleLineCell(feature.Notes))
		}
	}
	if len(rejected) > 0 {
		builder.WriteString("\n## Rejected\n\n")
		builder.WriteString("| Slug | Reason | Evidence | Note |\n")
		builder.WriteString("|------|--------|----------|------|\n")
		for _, feature := range rejected {
			reason, evidence, note := "", "", ""
			if feature.Rejection != nil {
				reason = feature.Rejection.Reason
				note = prepareSingleLineCell(feature.Rejection.Note)
				paths := make([]string, 0, len(feature.Rejection.Evidence))
				for _, item := range feature.Rejection.Evidence {
					paths = append(paths, "`"+item.Path+"`")
				}
				evidence = strings.Join(paths, ", ")
			}
			fmt.Fprintf(&builder, "| `%s` | %s | %s | %s |\n",
				feature.Slug, reason, evidence, note)
		}
	}
	return []byte(builder.String()), nil
}

func prepareSingleLineCell(value string) string {
	replacer := strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "|", "\\|")
	return strings.TrimSpace(replacer.Replace(value))
}

func revalidatePrepareSet(
	authority *intentlock.WorkspaceAuthority,
	state prepareReadState,
) error {
	if err := authority.ValidateOriginalPath(false); err != nil {
		return &intentpub.Error{
			Code:      intentpub.CodeWorkspaceRootChanged,
			Class:     "original-path-changed",
			Detail:    "the original workspace path no longer identifies the held root",
			ExitClass: 5,
		}
	}
	checks := []struct {
		rel      string
		expected prepareCaptured
	}{
		{prepareFeatureRel(state.inspection.Slug) + "/analysis.md", state.analysis},
		{prepareFeatureRel(state.inspection.Slug) + "/spec.md", state.spec},
		{prepareFeatureRel(state.inspection.Slug) + "/exploration.md", state.exploration},
		{prepareFeatureRel(state.inspection.Slug) + "/status.json", state.status},
	}
	if state.sidecar.state == intent.StateAbsent ||
		state.sidecar.state == intent.StatePresentNonempty ||
		state.sidecar.state == intent.StatePresentEmpty {
		checks = append(checks, struct {
			rel      string
			expected prepareCaptured
		}{prepareFeatureRel(state.inspection.Slug) + "/artifacts/analysis.json", state.sidecar})
	}
	for _, check := range checks {
		captured, err := capturePrepareWithAuthority(authority, check.rel, intent.MaxArtifactBytes)
		if strings.HasSuffix(check.rel, "/status.json") {
			captured, err = capturePrepareWithAuthority(authority, check.rel, intent.MaxStatusBytes)
		}
		if err != nil || !captured.identity.Equal(check.expected.identity) {
			return &intentpub.Error{
				Code:      intentpub.CodeEntryChanged,
				Class:     "set-revalidation",
				Detail:    "the frozen prepare set changed",
				ExitClass: 5,
			}
		}
	}
	return nil
}

type preparePreimageReferences struct {
	appendPlan store.IntentArchiveAppendPlan
	indexRaw   string
	statusRaw  string
}

func (references preparePreimageReferences) CanonicalPreimage(
	id intentpub.ArtifactID,
	identity intentpub.Identity,
) (intentpub.ArchivePreimage, error) {
	archiveID, ok := prepareArchiveArtifactID(id)
	if !ok {
		return intentpub.ArchivePreimage{}, errors.New("artifact is not archive-backed")
	}
	reference, found := references.appendPlan.PreimageFor(archiveID)
	if !found || reference.ContentSHA256 != identity.SHA256 {
		return intentpub.ArchivePreimage{}, errors.New("archive preimage is unavailable")
	}
	return intentpub.ArchivePreimage{
		SHA256: reference.ContentSHA256,
		Rel:    reference.BlobRel,
	}, nil
}

func (references preparePreimageReferences) MetadataPreimage(id intentpub.ArtifactID) (string, error) {
	switch id {
	case intentpub.ArtifactArchiveIndex:
		if references.indexRaw == "" {
			return "", errors.New("archive index raw preimage is unavailable")
		}
		return references.indexRaw, nil
	case intentpub.ArtifactStatus:
		if references.statusRaw == "" {
			return "", errors.New("status raw preimage is unavailable")
		}
		return references.statusRaw, nil
	default:
		return "", errors.New("artifact does not use a raw metadata preimage")
	}
}

func prepareArchiveArtifactID(id intentpub.ArtifactID) (store.IntentArchiveArtifactID, bool) {
	switch id {
	case intentpub.ArtifactAnalysis:
		return store.IntentArchiveArtifactAnalysis, true
	case intentpub.ArtifactSpec:
		return store.IntentArchiveArtifactSpec, true
	case intentpub.ArtifactExploration:
		return store.IntentArchiveArtifactExploration, true
	case intentpub.ArtifactAnalysisSidecar:
		return store.IntentArchiveArtifactAnalysisSidecar, true
	default:
		return "", false
	}
}

func prepareStatusNotes(mode prepareMode, plan preparePlan) string {
	if mode == prepareModeRegenerate {
		return "Intent bundle regenerated (prepare --regenerate); prior bytes archived"
	}
	names := []string{}
	for _, id := range []intentpub.ArtifactID{
		intentpub.ArtifactAnalysis,
		intentpub.ArtifactSpec,
		intentpub.ArtifactExploration,
	} {
		if prepareContainsArtifactID(plan.generated, id) {
			names = append(names, path.Base(prepareArtifactFeaturePath(prepareIntentpubReportID(id))))
		}
	}
	return "Intent bundle prepared (prepare); generated: " + strings.Join(names, ", ")
}

func preparePublicationStageInputs(
	readState prepareReadState,
	report preparePublishReport,
	plan preparePlan,
	generated prepareGeneratedBundle,
) ([]intentpub.StageInput, error) {
	statusBytes, err := prepareStatusBytes(readState.status.bytes, prepareStatusNotes(report.Mode, plan))
	if err != nil {
		return nil, err
	}
	stageInputs := make([]intentpub.StageInput, 0, len(plan.generated)+1)
	for _, id := range []intentpub.ArtifactID{
		intentpub.ArtifactAnalysis,
		intentpub.ArtifactSpec,
		intentpub.ArtifactExploration,
		intentpub.ArtifactAnalysisSidecar,
	} {
		generatedArtifact, ok := generated.find(id)
		if !ok {
			continue
		}
		preimage := prepareCapturedForID(readState, id)
		mode := fs.FileMode(0o644)
		if preimage.identity.Exists {
			mode = fs.FileMode(preimage.identity.Mode)
		}
		stageInputs = append(stageInputs, intentpub.StageInput{
			ArtifactID: id,
			Rel:        prepareStageBase(id),
			Data:       generatedArtifact.data,
			Mode:       mode,
		})
	}
	stageInputs = append(stageInputs, intentpub.StageInput{
		ArtifactID: intentpub.ArtifactStatus,
		Rel:        "status.json",
		Data:       statusBytes,
		Mode:       fs.FileMode(readState.status.identity.Mode),
	})
	return stageInputs, nil
}

func stagePreparePublicationBase(
	authority *intentlock.WorkspaceAuthority,
	slug string,
	stageInputs []intentpub.StageInput,
	transactionOptions intentpub.Options,
) (intentpub.StageResult, error) {
	return intentpub.Stage(authority, slug, stageInputs, transactionOptions)
}

func stagePrepareArchiveIndex(
	authority *intentlock.WorkspaceAuthority,
	stageResult *intentpub.StageResult,
	appendPlan store.IntentArchiveAppendPlan,
	transactionOptions intentpub.Options,
) error {
	if authority == nil || stageResult == nil || !validPrepareRel(stageResult.StageRel) {
		return errors.New("invalid archive index staging target")
	}
	indexIdentity, err := prepareArchiveTokenIdentity(appendPlan.IndexPreimage().Identity)
	if err != nil {
		return err
	}
	indexBytes := appendPlan.IndexBytes()
	if len(indexBytes) > intentpub.MaxArtifactBytes {
		return &intentpub.Error{
			Code:       intentpub.CodeStagedOutputInvalid,
			ArtifactID: intentpub.ArtifactArchiveIndex,
			Class:      "v2-size",
			ExitClass:  2,
		}
	}
	if _, err := store.DecodeIntentArchiveIndex(indexBytes, appendPlan.Feature()); err != nil {
		return err
	}
	indexMode := fs.FileMode(0o644)
	if indexIdentity.Exists {
		indexMode = fs.FileMode(indexIdentity.Mode)
	}
	newImage := intentpub.Identity{
		Exists: true,
		SHA256: prepareSHA256(indexBytes),
		Size:   int64(len(indexBytes)),
		Mode:   uint32(indexMode.Perm()),
	}
	rel := stageResult.StageRel + "/index.json"
	writeResult, err := intentpub.DurableWrite(authority, intentpub.WriteRequest{
		Rel:        rel,
		Data:       indexBytes,
		Mode:       0o600,
		ArtifactID: intentpub.ArtifactArchiveIndex,
		Role:       intentpub.WriteRoleControl,
	}, transactionOptions)
	if err != nil {
		return err
	}
	if !writeResult.Identity.Equal(intentpub.Identity{
		Exists: true,
		SHA256: newImage.SHA256,
		Size:   newImage.Size,
		Mode:   0o600,
	}) {
		return &intentpub.Error{
			Code:       intentpub.CodeEntryChanged,
			ArtifactID: intentpub.ArtifactArchiveIndex,
			Class:      "v6-staged-identity",
			ExitClass:  5,
		}
	}
	stagedIndex := intentpub.StagedFile{
		ArtifactID: intentpub.ArtifactArchiveIndex,
		Rel:        rel,
		Identity:   writeResult.Identity,
		NewImage:   newImage,
	}
	if len(stageResult.Files) == 0 ||
		stageResult.Files[len(stageResult.Files)-1].ArtifactID != intentpub.ArtifactStatus {
		return errors.New("status is not the final staged publication entry")
	}
	status := stageResult.Files[len(stageResult.Files)-1]
	stageResult.Files[len(stageResult.Files)-1] = stagedIndex
	stageResult.Files = append(stageResult.Files, status)
	return syncPrepareAuthorityDirectory(authority, stageResult.StageRel)
}

func syncPrepareAuthorityDirectory(authority *intentlock.WorkspaceAuthority, rel string) error {
	return authority.WithRoot(func(root *os.Root) error {
		return syncPrepareRootDirectory(root, rel)
	})
}

func prepareSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func publishPrepareTransaction(
	authority *intentlock.WorkspaceAuthority,
	repoRoot, slug string,
	readState prepareReadState,
	report preparePublishReport,
	plan preparePlan,
	stageResult intentpub.StageResult,
	appendPlan store.IntentArchiveAppendPlan,
	hasArchive bool,
	transactionOptions intentpub.Options,
) (preparePublishReport, int) {
	var err error
	archiveResult := store.IntentArchiveAppendResult{
		BlobResults:     []store.IntentArchiveBlobPublishResult{},
		NewOrphanHashes: []string{},
	}
	if hasArchive {
		archiveStorage := newPrepareArchiveStorage(authority, nil)
		archiveResult, err = store.PublishIntentArchiveBlobs(archiveStorage, appendPlan)
		report.OrphanBlobs = append([]string(nil), archiveResult.NewOrphanHashes...)
		if err != nil {
			report = prepareStoreArchiveFailure(report, err, archiveResult.Committed)
			report = appendPrepareStagingAdvisory(report, stageResult.StageRel)
			report = appendPrepareOrphanAdvisory(report)
			exit := prepareArchiveExit(err, 5)
			if exit < 5 {
				exit = 5
			}
			if !archiveResult.Committed && exit < 6 {
				code := "entry-changed"
				var typed *store.IntentArchiveError
				if errors.As(err, &typed) && typed.Code == store.IntentArchiveCodeIndexChanged {
					code = "archive-index-changed"
				}
				report = refusePrepare(report, code, "")
				report.Refusal.Message = "The frozen archive state changed after staging."
			}
			if exit >= 6 {
				report.Outcome = "recovery-refused"
			} else {
				report.Outcome = "rolled-back"
			}
			return report, exit
		}
		for _, blob := range archiveResult.BlobResults {
			if blob.Reused {
				report.Advisories = append(report.Advisories, prepareAdvisory(
					"archive-blob-reused", "", "An existing equal content-addressed archive blob was reused, so zero new bytes were written for it.",
				))
			}
		}
		if appendPlan.Outcome() == store.IntentArchiveAppendNoOp {
			report.Advisories = append(report.Advisories, prepareAdvisory(
				"archive-generation-duplicate", "",
				"The content-addressed archive generation already existed, so no duplicate entry was appended.",
			))
		}
		report.Archive = &prepareArchiveReport{
			GenerationID: appendPlan.GenerationID(),
			BlobsDir:     prepareFeatureRel(slug) + "/artifacts/intent-archive/blobs",
		}
		for _, reference := range appendPlan.Preimages() {
			reportID := prepareStoreArchiveReportID(reference.ArtifactID)
			for index := range report.Artifacts {
				if report.Artifacts[index].ID == reportID {
					report.Artifacts[index].ArchivedBlob = reference.ContentSHA256
				}
			}
		}
	}

	references := preparePreimageReferences{appendPlan: appendPlan}
	if hasArchive {
		indexIdentity, _ := prepareArchiveTokenIdentity(appendPlan.IndexPreimage().Identity)
		if indexIdentity.Exists {
			references.indexRaw, err = intentpub.WriteRawPreimage(
				authority, slug, intentpub.ArtifactArchiveIndex,
				appendPlan.IndexPreimage().Raw, transactionOptions,
			)
			if err != nil {
				return prepareRawPreimageFailure(
					report, err, stageResult.StageRel, references.indexRaw,
				)
			}
		}
	}
	references.statusRaw, err = intentpub.WriteRawPreimage(
		authority, slug, intentpub.ArtifactStatus, readState.status.bytes, transactionOptions,
	)
	if err != nil {
		return prepareRawPreimageFailure(
			report, err, stageResult.StageRel, references.statusRaw,
		)
	}

	entries := make([]intentpub.Entry, 0, len(stageResult.Files))
	for _, staged := range stageResult.Files {
		preimage, preimageErr := preparePreimageForEntry(readState, staged.ArtifactID, appendPlan)
		if preimageErr != nil {
			report = refusePrepare(report, "entry-changed", "")
			report.Outcome = "rolled-back"
			report = appendPrepareStagingAdvisory(report, stageResult.StageRel)
			report = appendPrepareOrphanAdvisory(report)
			return report, 5
		}
		action := intentpub.ActionCreate
		if preimage.Exists {
			action = intentpub.ActionReplace
		}
		entry := intentpub.Entry{
			ArtifactID: staged.ArtifactID,
			Rel:        prepareCanonicalRel(slug, staged.ArtifactID),
			Action:     action,
			Preimage:   preimage,
			NewImage:   staged.NewImage,
			StagedRel:  staged.Rel,
		}
		if action == intentpub.ActionReplace {
			entry, err = intentpub.BindPreimageReference(entry, references)
			if err != nil {
				report = refusePrepare(report, prepareIntentpubCode(err, "entry-changed"), "")
				report.Outcome = "rolled-back"
				report = appendPrepareStagingAdvisory(report, stageResult.StageRel)
				report = appendPrepareOrphanAdvisory(report)
				return report, 5
			}
		}
		entries = append(entries, entry)
	}
	intentMode := intentpub.ModeGenerate
	if report.Mode == prepareModeRegenerate {
		intentMode = intentpub.ModeRegenerate
	}
	publicationPlan, err := intentpub.NewPlan(slug, intentMode, stageResult.StageRel, entries)
	if err != nil {
		report = refusePrepare(report, prepareIntentpubCode(err, "entry-changed"), "")
		report.Outcome = "rolled-back"
		report = appendPrepareStagingAdvisory(report, stageResult.StageRel)
		report = appendPrepareOrphanAdvisory(report)
		return report, 5
	}

	runNonce, err := intentpub.NewRunNonce()
	if err != nil {
		report = refusePrepare(report, "entry-changed", "")
		report.Outcome = "rolled-back"
		report = appendPrepareStagingAdvisory(report, stageResult.StageRel)
		report = appendPrepareOrphanAdvisory(report)
		return report, 5
	}
	createdArtifactsDir, err := ensurePrepareArtifactsDirectory(authority, slug, stageResult.Files)
	if err != nil {
		report = refusePrepare(report, "entry-changed", "")
		report.Outcome = "rolled-back"
		report = appendPrepareStagingAdvisory(report, stageResult.StageRel)
		report = appendPrepareOrphanAdvisory(report)
		return report, 5
	}
	transactionResult, transactionErr := intentpub.Execute(
		authority, publicationPlan, runNonce, report.OrphanBlobs,
		prepareTransactionOptions(transactionOptions, appendPlan, hasArchive),
	)
	if transactionErr != nil {
		report = prepareIntentpubFailure(report, transactionResult, transactionErr)
		report = refreshPrepareOrphanTruth(authority, slug, report)
		report = appendPrepareOrphanAdvisory(report)
		if prepareStageExists(authority, stageResult.StageRel) {
			report = appendPrepareStagingAdvisory(report, stageResult.StageRel)
		}
		if prepareContainsArtifactID(transactionResult.Restored, intentpub.ArtifactStatus) {
			refreshPrepareFeaturesIndex(authority, repoRoot, &report)
		}
		if transactionResult.ExitClass == 0 {
			transactionResult.ExitClass = 5
		}
		if createdArtifactsDir && transactionResult.ExitClass != 6 {
			if cleanupErr := removePrepareEmptyArtifactsDirectory(authority, slug); cleanupErr != nil {
				report = refusePrepare(report, "post-publication-divergence", "")
				report.Outcome = "recovery-refused"
				return report, 6
			}
		}
		return report, transactionResult.ExitClass
	}
	report.Outcome = "published"
	report.FeatureState = intent.FeatureStateDefined
	report.OrphanBlobs = []string{}
	refreshPrepareFeaturesIndex(authority, repoRoot, &report)
	return report, 0
}

func prepareTransactionOptions(
	options intentpub.Options,
	appendPlan store.IntentArchiveAppendPlan,
	hasArchive bool,
) intentpub.Options {
	previous := options.BeforeRename
	options.BeforeRename = func(request intentpub.WriteRequest) {
		if previous != nil {
			previous(request)
		}
		if hasArchive &&
			request.ArtifactID == intentpub.ArtifactArchiveIndex &&
			beforeIndexRewrite != nil {
			beforeIndexRewrite(request.Rel)
		}
		if hasArchive &&
			appendPlan.Outcome() == store.IntentArchiveAppendRehydrate &&
			request.ArtifactID == intentpub.ArtifactArchiveIndex &&
			beforeRehydrateIndexRename != nil {
			beforeRehydrateIndexRename(request.Rel)
		}
	}
	return options
}

func ensurePrepareArtifactsDirectory(
	authority *intentlock.WorkspaceAuthority,
	slug string,
	files []intentpub.StagedFile,
) (bool, error) {
	needed := false
	for _, staged := range files {
		if staged.ArtifactID == intentpub.ArtifactAnalysisSidecar {
			needed = true
			break
		}
	}
	if !needed {
		return false, nil
	}
	artifactsRel := prepareFeatureRel(slug) + "/artifacts"
	created := false
	err := authority.WithRoot(func(root *os.Root) error {
		info, err := root.Lstat(artifactsRel)
		if err == nil {
			if prepareRefusedInfo(info) || !info.IsDir() {
				return errors.New("artifacts directory is unsafe")
			}
			return nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := root.Mkdir(artifactsRel, 0o755); err != nil {
			return err
		}
		created = true
		return syncPrepareRootDirectory(root, prepareFeatureRel(slug))
	})
	return created, err
}

func removePrepareEmptyArtifactsDirectory(authority *intentlock.WorkspaceAuthority, slug string) error {
	artifactsRel := prepareFeatureRel(slug) + "/artifacts"
	return authority.WithRoot(func(root *os.Root) error {
		if err := root.Remove(artifactsRel); err != nil {
			return err
		}
		return syncPrepareRootDirectory(root, prepareFeatureRel(slug))
	})
}

func prepareStageExists(authority *intentlock.WorkspaceAuthority, rel string) bool {
	if authority == nil || !validPrepareRel(rel) {
		return false
	}
	exists := false
	_ = authority.WithRoot(func(root *os.Root) error {
		info, err := root.Lstat(rel)
		exists = err == nil && info != nil && !prepareRefusedInfo(info) && info.IsDir()
		return nil
	})
	return exists
}

func prepareCapturedForID(state prepareReadState, id intentpub.ArtifactID) prepareCaptured {
	switch id {
	case intentpub.ArtifactAnalysis:
		return state.analysis
	case intentpub.ArtifactSpec:
		return state.spec
	case intentpub.ArtifactExploration:
		return state.exploration
	case intentpub.ArtifactAnalysisSidecar:
		return state.sidecar
	case intentpub.ArtifactStatus:
		return state.status
	default:
		return prepareCaptured{state: intent.StateAbsent, identity: intentpub.AbsentIdentity()}
	}
}

func preparePreimageForEntry(
	state prepareReadState,
	id intentpub.ArtifactID,
	appendPlan store.IntentArchiveAppendPlan,
) (intentpub.Identity, error) {
	if id == intentpub.ArtifactArchiveIndex {
		identity, err := prepareArchiveTokenIdentity(appendPlan.IndexPreimage().Identity)
		if err != nil {
			return intentpub.Identity{}, err
		}
		return identity, nil
	}
	return prepareCapturedForID(state, id).identity, nil
}

func prepareStageBase(id intentpub.ArtifactID) string {
	switch id {
	case intentpub.ArtifactAnalysis:
		return "analysis.md"
	case intentpub.ArtifactSpec:
		return "spec.md"
	case intentpub.ArtifactExploration:
		return "exploration.md"
	case intentpub.ArtifactAnalysisSidecar:
		return "analysis.json"
	case intentpub.ArtifactArchiveIndex:
		return "index.json"
	case intentpub.ArtifactStatus:
		return "status.json"
	default:
		return ""
	}
}

func prepareCanonicalRel(slug string, id intentpub.ArtifactID) string {
	rel, _ := intentpub.CanonicalPath(slug, id)
	return rel
}

func prepareStoreArchiveReportID(id store.IntentArchiveArtifactID) string {
	switch id {
	case store.IntentArchiveArtifactAnalysis:
		return "analysis"
	case store.IntentArchiveArtifactSpec:
		return "spec"
	case store.IntentArchiveArtifactExploration:
		return "exploration"
	case store.IntentArchiveArtifactAnalysisSidecar:
		return "analysis_sidecar"
	default:
		return ""
	}
}
