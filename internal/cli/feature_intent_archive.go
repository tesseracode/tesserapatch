package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/intent"
	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/intentpub"
	"github.com/tesseracode/tesserapatch/internal/store"
)

const (
	intentArchiveCommandList       = "feature intent-archive list"
	intentArchiveCommandPurge      = "feature intent-archive purge"
	intentArchiveHistoryDisclosure = "A committed blob remains in Git history; removing it from history is not something tpatch performs, and tpatch does not rewrite Git history."
	intentArchivePendingPlan       = "claim every reference to it, remove the blob if it is present, then tombstone every reference to it"
	intentArchiveAllBlastRadius    = "The --all selector claims and tombstones every reference in every generation and removes every blob in this archive, leaving no recoverable bytes for any artifact until identical content is archived again. The unconfirmed preview is the default and shows the full selection first; repeated --blob selectors over the hashes listed in this report cover the same work while touching nothing else."
	intentArchiveSHA256HexLength   = 64
)

var (
	intentArchiveOpenRoot                                              = os.OpenRoot
	intentArchiveAcquireAuthority                                      = intentlock.Acquire
	intentArchiveReadBoundarySupported                                 = intent.RootConfinementSupported
	intentArchiveMutationAuthoritySupported                            = func() bool { return intentlock.AuthoritySupported }
	intentArchiveJournals                   intentArchiveJournalAccess = intentArchiveRootJournalAccess{}
	intentArchiveNewStorage                                            = func(authority *intentlock.WorkspaceAuthority, root *os.Root) store.IntentArchiveStorage {
		return newPrepareArchiveStorage(authority, root)
	}
	intentArchiveCapture      = store.CaptureIntentArchive
	intentArchivePreviewPurge = store.PreviewIntentArchivePurge
	intentArchivePlanPurge    = store.PlanIntentArchivePurge
	intentArchiveRecoverPurge = store.RecoverPendingPurge
	intentArchiveExecutePurge = store.ExecuteIntentArchivePurge
)

type intentArchiveJournalAccess interface {
	Execute(*os.Root, intentArchiveJournalRequest) (intentArchiveJournalResult, error)
}

type intentArchiveRootJournalAccess struct{}

type intentArchiveJournalOperation string

const (
	intentArchiveJournalObserveMarker intentArchiveJournalOperation = "observe-marker"
	intentArchiveJournalDecode        intentArchiveJournalOperation = "decode"
	intentArchiveJournalRename        intentArchiveJournalOperation = "rename"
)

type intentArchiveJournalRequest struct {
	Operation intentArchiveJournalOperation
	Slug      string
	OldRel    string
	NewRel    string
}

type intentArchiveJournalResult struct {
	MarkerExists bool
	Journal      intentpub.Journal
}

func (intentArchiveRootJournalAccess) Execute(
	root *os.Root,
	request intentArchiveJournalRequest,
) (intentArchiveJournalResult, error) {
	switch request.Operation {
	case intentArchiveJournalObserveMarker:
		exists, err := prepareJournalMarkerExists(root, request.Slug)
		return intentArchiveJournalResult{MarkerExists: exists}, err
	case intentArchiveJournalDecode:
		data, err := root.ReadFile(intentpub.JournalRel(request.Slug))
		if err != nil {
			return intentArchiveJournalResult{}, err
		}
		journal, err := intentpub.DecodeJournal(data, request.Slug)
		return intentArchiveJournalResult{Journal: journal}, err
	case intentArchiveJournalRename:
		return intentArchiveJournalResult{}, root.Rename(request.OldRel, request.NewRel)
	default:
		return intentArchiveJournalResult{}, errors.New("unsupported intent-archive journal operation")
	}
}

func observeIntentArchiveJournalMarker(root *os.Root, slug string) (bool, error) {
	result, err := intentArchiveJournals.Execute(root, intentArchiveJournalRequest{
		Operation: intentArchiveJournalObserveMarker,
		Slug:      slug,
	})
	return result.MarkerExists, err
}

func decodeIntentArchiveJournal(root *os.Root, slug string) (intentpub.Journal, error) {
	result, err := intentArchiveJournals.Execute(root, intentArchiveJournalRequest{
		Operation: intentArchiveJournalDecode,
		Slug:      slug,
	})
	return result.Journal, err
}

func renameIntentArchiveJournal(root *os.Root, oldRel, newRel string) error {
	_, err := intentArchiveJournals.Execute(root, intentArchiveJournalRequest{
		Operation: intentArchiveJournalRename,
		OldRel:    oldRel,
		NewRel:    newRel,
	})
	return err
}

type intentArchiveRefusalReport struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Remediation string `json:"remediation"`
	Retry       string `json:"retry,omitempty"`
	RetryCWD    string `json:"retry_cwd,omitempty"`
}

type intentArchiveListEntryReport struct {
	ArtifactID             string   `json:"artifact_id"`
	Path                   string   `json:"path"`
	ContentSHA256          string   `json:"content_sha256"`
	Blob                   string   `json:"blob"`
	SizeBytes              int64    `json:"size_bytes"`
	Purged                 bool     `json:"purged"`
	PurgePending           bool     `json:"purge_pending"`
	Storage                string   `json:"storage"`
	Present                bool     `json:"present"`
	BlobPath               string   `json:"blob_path"`
	BlobSizeBytes          int64    `json:"blob_size_bytes"`
	Availability           string   `json:"availability"`
	Repair                 string   `json:"repair,omitempty"`
	Retry                  string   `json:"retry,omitempty"`
	RetryCWD               string   `json:"retry_cwd,omitempty"`
	LiveGenerationIDs      []string `json:"live_generation_ids,omitempty"`
	TombstoneGenerationIDs []string `json:"tombstone_generation_ids,omitempty"`
}

type intentArchiveListGenerationReport struct {
	GenerationID string                         `json:"generation_id"`
	Mode         string                         `json:"mode"`
	Entries      []intentArchiveListEntryReport `json:"entries"`
}

type intentArchiveListOrphanReport struct {
	Hash      string `json:"hash"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	Present   bool   `json:"present"`
	Storage   string `json:"storage"`
	Repair    string `json:"repair"`
	Retry     string `json:"retry,omitempty"`
	RetryCWD  string `json:"retry_cwd,omitempty"`
}

type intentArchiveListCorruptObjectReport struct {
	Hash     string `json:"hash,omitempty"`
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Repair   string `json:"repair"`
	Retry    string `json:"retry,omitempty"`
	RetryCWD string `json:"retry_cwd,omitempty"`
}

type intentArchiveListReport struct {
	SchemaVersion     int                                    `json:"schema_version"`
	Command           string                                 `json:"command"`
	Slug              string                                 `json:"slug"`
	Outcome           string                                 `json:"outcome"`
	Index             string                                 `json:"index"`
	Generations       []intentArchiveListGenerationReport    `json:"generations"`
	CorruptObjects    []intentArchiveListCorruptObjectReport `json:"corrupt_objects"`
	Orphans           []intentArchiveListOrphanReport        `json:"orphans"`
	HistoryDisclosure string                                 `json:"history_disclosure"`
	Refusal           *intentArchiveRefusalReport            `json:"refusal,omitempty"`
}

type intentArchivePurgeOptions struct {
	blobs       []string
	generations []string
	all         bool
	orphans     bool
	yes         bool
	asJSON      bool
	quiet       bool
}

type intentArchivePurgeReferenceReport struct {
	GenerationID string `json:"generation_id"`
	ArtifactID   string `json:"artifact_id"`
	Path         string `json:"path"`
	Hash         string `json:"hash"`
	WireState    string `json:"wire_state"`
}

type intentArchivePurgeBlobReport struct {
	Hash      string `json:"hash"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	Present   bool   `json:"present"`
	Removed   bool   `json:"removed"`
}

type intentArchivePendingHashReport struct {
	Hash  string `json:"hash"`
	Blob  string `json:"blob"`
	Index string `json:"index"`
	Plan  string `json:"plan"`
}

type intentArchivePendingPurgeReport struct {
	RecoveryRequired bool                             `json:"recovery_required"`
	PendingHashes    []intentArchivePendingHashReport `json:"pending_hashes"`
	Selector         string                           `json:"selector"`
	Retry            string                           `json:"retry"`
	RetryCWD         string                           `json:"retry_cwd"`
}

type intentArchiveRecoveryReport struct {
	Kind            string   `json:"kind"`
	RestoredEntries []string `json:"restored_entries"`
	FinalizedHashes []string `json:"finalized_hashes"`
	Retry           string   `json:"retry"`
	RetryCWD        string   `json:"retry_cwd"`
}

type intentArchivePurgeProgressReport struct {
	CompletedHashes []string `json:"completed_hashes"`
	PendingHash     string   `json:"pending_hash,omitempty"`
	RemainingHashes []string `json:"remaining_hashes"`
	Resume          string   `json:"resume"`
	Retry           string   `json:"retry"`
	RetryCWD        string   `json:"retry_cwd"`
	State           string   `json:"state"`
}

type intentArchiveRepairNextReport struct {
	Ordinal int    `json:"ordinal"`
	Kind    string `json:"kind"`
	Class   string `json:"class"`
}

type intentArchiveRepairStageReport struct {
	Ordinal           int      `json:"ordinal"`
	Kind              string   `json:"kind"`
	Class             string   `json:"class"`
	Hashes            []string `json:"hashes"`
	Paths             []string `json:"paths"`
	Repair            string   `json:"repair"`
	RepairCWD         string   `json:"repair_cwd"`
	ResultingClasses  []string `json:"resulting_classes"`
	AfterPrerequisite bool     `json:"after_prerequisite"`
}

type intentArchiveRemainingRepairsReport struct {
	RerunRequired   bool                             `json:"rerun_required"`
	RepairedClass   string                           `json:"repaired_class,omitempty"`
	StagesRemaining int                              `json:"stages_remaining"`
	NextStage       *intentArchiveRepairNextReport   `json:"next_stage"`
	Stages          []intentArchiveRepairStageReport `json:"stages"`
}

type intentArchiveDivergenceReport struct {
	Kind               string   `json:"kind"`
	PendingHash        string   `json:"pending_hash"`
	Blob               string   `json:"blob,omitempty"`
	Index              string   `json:"index"`
	Warning            string   `json:"warning"`
	RemoveCommand      string   `json:"remove_command,omitempty"`
	RestoreInstruction string   `json:"restore_instruction,omitempty"`
	CompletedHashes    []string `json:"completed_hashes"`
	RemainingHashes    []string `json:"remaining_hashes"`
	Retry              string   `json:"retry"`
	RetryCWD           string   `json:"retry_cwd"`
	Cost               string   `json:"cost"`
}

type intentArchivePurgeReport struct {
	SchemaVersion     int                                  `json:"schema_version"`
	Command           string                               `json:"command"`
	Slug              string                               `json:"slug"`
	Outcome           string                               `json:"outcome"`
	Action            string                               `json:"action"`
	Selector          string                               `json:"selector"`
	Confirmed         bool                                 `json:"confirmed"`
	Hashes            []string                             `json:"hashes"`
	GenerationIDs     []string                             `json:"generation_ids"`
	References        []intentArchivePurgeReferenceReport  `json:"references"`
	Blobs             []intentArchivePurgeBlobReport       `json:"blobs"`
	OrphanBlobs       []string                             `json:"orphan_blobs"`
	Advisories        []prepareAdvisoryReport              `json:"advisories"`
	HistoryDisclosure string                               `json:"history_disclosure"`
	BlastRadius       string                               `json:"blast_radius,omitempty"`
	Retry             string                               `json:"retry,omitempty"`
	RetryCWD          string                               `json:"retry_cwd,omitempty"`
	Refusal           *intentArchiveRefusalReport          `json:"refusal,omitempty"`
	Recovery          *intentArchiveRecoveryReport         `json:"recovery,omitempty"`
	PendingPurge      *intentArchivePendingPurgeReport     `json:"pending_purge,omitempty"`
	PurgeProgress     *intentArchivePurgeProgressReport    `json:"purge_progress,omitempty"`
	RemainingRepairs  *intentArchiveRemainingRepairsReport `json:"remaining_repairs,omitempty"`
	Divergence        *intentArchiveDivergenceReport       `json:"divergence,omitempty"`
}

func featureIntentArchiveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "intent-archive",
		Short: "Inspect or purge archived intent-bundle bytes",
	}
	cmd.AddCommand(featureIntentArchiveListCmd())
	cmd.AddCommand(featureIntentArchivePurgeCmd())
	return cmd
}

func featureIntentArchiveListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <slug>",
		Short: "List intent-archive generations and storage truth",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFeatureIntentArchiveList(cmd, args[0])
		},
	}
	cmd.Flags().Bool("json", false, "Emit the fixed version-one report as JSON")
	cmd.Flags().Bool("quiet", false, "Emit exactly one summary line")
	return cmd
}

func featureIntentArchivePurgeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "purge <slug>",
		Short: "Preview or purge archived intent-bundle bytes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options := readIntentArchivePurgeOptions(cmd)
			if err := validateIntentArchivePurgeScope(options); err != nil {
				return err
			}
			return runFeatureIntentArchivePurge(cmd, args[0], options)
		},
	}
	cmd.Flags().StringArray("blob", nil, "Select every reference to this content hash (repeatable)")
	cmd.Flags().StringArray("generation", nil, "Select an archive generation (repeatable)")
	cmd.Flags().Bool("orphans", false, "Select every validated globally unreferenced blob")
	cmd.Flags().Bool("all", false, "Select the whole archive")
	cmd.Flags().Bool("yes", false, "Perform the purge instead of previewing it")
	cmd.Flags().Bool("json", false, "Emit the fixed version-one report as JSON")
	cmd.Flags().Bool("quiet", false, "Emit exactly one summary line")
	return cmd
}

func readIntentArchivePurgeOptions(cmd *cobra.Command) intentArchivePurgeOptions {
	blobs, _ := cmd.Flags().GetStringArray("blob")
	generations, _ := cmd.Flags().GetStringArray("generation")
	all, _ := cmd.Flags().GetBool("all")
	orphans, _ := cmd.Flags().GetBool("orphans")
	yes, _ := cmd.Flags().GetBool("yes")
	asJSON, _ := cmd.Flags().GetBool("json")
	quiet, _ := cmd.Flags().GetBool("quiet")
	return intentArchivePurgeOptions{
		blobs:       append([]string(nil), blobs...),
		generations: append([]string(nil), generations...),
		all:         all,
		orphans:     orphans,
		yes:         yes,
		asJSON:      asJSON,
		quiet:       quiet,
	}
}

func validateIntentArchivePurgeScope(options intentArchivePurgeOptions) error {
	families := 0
	if len(options.blobs) != 0 {
		families++
	}
	if len(options.generations) != 0 {
		families++
	}
	if options.orphans {
		families++
	}
	if options.all {
		families++
	}
	if families != 1 {
		return errors.New("feature intent-archive purge requires exactly one scope family: --blob, --generation, --orphans, or --all")
	}
	return nil
}

func runFeatureIntentArchiveList(cmd *cobra.Command, rawSlug string) error {
	slug, err := intent.CanonicalSlug(rawSlug)
	if err != nil {
		report := newIntentArchiveListReport("")
		report.Outcome = "refused"
		report.Refusal = intentArchiveSimpleRefusal("slug-unsafe",
			"The feature slug is unsafe.",
			"Use a lowercase kebab-case feature slug.")
		return emitIntentArchiveListReport(cmd, report, 3)
	}
	report := newIntentArchiveListReport(slug)
	repoRoot, code, message := intentArchiveWorkspace(cmd)
	if code != "" {
		report.Outcome = "refused"
		report.Refusal = intentArchiveSimpleRefusal(code, message, intentArchiveWorkspaceRemediation(code))
		return emitIntentArchiveListReport(cmd, report, 3)
	}
	if !intentArchiveReadBoundarySupported() {
		report.Outcome = "refused"
		report.Refusal = intentArchiveSimpleRefusal(
			"workspace-unsupported-platform",
			"This platform cannot establish the rooted read boundary.",
			"Run this command on a supported Linux, macOS, or Windows filesystem.",
		)
		return emitIntentArchiveListReport(cmd, report, 3)
	}
	root, err := intentArchiveOpenRoot(repoRoot)
	if err != nil {
		report.Outcome = "refused"
		report.Refusal = intentArchiveSimpleRefusal(
			"workspace-not-initialized",
			"The workspace root could not be opened.",
			"Run the command from an initialized tpatch workspace.",
		)
		return emitIntentArchiveListReport(cmd, report, 3)
	}
	defer root.Close()
	storage := intentArchiveNewStorage(nil, root)
	snapshot, captureErr := intentArchiveCapture(storage, slug)
	if captureErr != nil {
		exit := prepareArchiveExit(captureErr, 3)
		report.Outcome = "refused"
		report.Refusal = intentArchiveRefusalFromError(slug, captureErr, nil, intentArchivePurgeOptions{})
		return emitIntentArchiveListReport(cmd, report, exit)
	}
	if pathErr := validateIntentArchiveSnapshotReportPaths(snapshot); pathErr != nil {
		report.Outcome = "refused"
		report.Refusal = intentArchiveRefusalFromError(slug, pathErr, nil, intentArchivePurgeOptions{})
		return emitIntentArchiveListReport(cmd, report, 3)
	}
	report = buildIntentArchiveListReport(snapshot)
	report.CorruptObjects = buildIntentArchiveUnindexedCorruptObjects(storage, snapshot)
	exit := intentArchiveListExit(snapshot.Inspection)
	if exit != 0 {
		report.Outcome = "refused"
		report.Refusal = intentArchiveListInspectionRefusal(slug, snapshot.Inspection)
	}
	return emitIntentArchiveListReport(cmd, report, exit)
}

func runFeatureIntentArchivePurge(cmd *cobra.Command, rawSlug string, options intentArchivePurgeOptions) error {
	slug, err := intent.CanonicalSlug(rawSlug)
	if err != nil {
		report := newIntentArchivePurgeReport("", options)
		report.Outcome = "refused"
		report.Refusal = intentArchiveSimpleRefusal("slug-unsafe",
			"The feature slug is unsafe.",
			"Use a lowercase kebab-case feature slug.")
		return emitIntentArchivePurgeReport(cmd, report, 3)
	}
	report := newIntentArchivePurgeReport(slug, options)
	repoRoot, code, message := intentArchiveWorkspace(cmd)
	if code != "" {
		report.Outcome = "refused"
		report.Refusal = intentArchiveSimpleRefusal(code, message, intentArchiveWorkspaceRemediation(code))
		return emitIntentArchivePurgeReport(cmd, report, 3)
	}
	if !intentArchiveReadBoundarySupported() {
		report.Outcome = "refused"
		report.Refusal = intentArchiveSimpleRefusal(
			"workspace-unsupported-platform",
			"This platform cannot establish the rooted archive boundary.",
			"Run this command on a supported platform.",
		)
		return emitIntentArchivePurgeReport(cmd, report, 3)
	}
	if !options.yes {
		return runFeatureIntentArchivePurgePreview(cmd, repoRoot, slug, options, report)
	}
	return runFeatureIntentArchivePurgeConfirmed(cmd, repoRoot, slug, options, report)
}

func normalizeIntentArchivePurgeOptions(options intentArchivePurgeOptions) (intentArchivePurgeOptions, error) {
	normalized := options
	normalized.blobs = sortedUniqueIntentArchiveStrings(options.blobs)
	normalized.generations = sortedUniqueIntentArchiveStrings(options.generations)
	for _, hash := range normalized.blobs {
		if !intentArchiveValidSelectorID(hash) {
			return intentArchivePurgeOptions{}, errors.New("each --blob value must be 64 lowercase hexadecimal characters")
		}
	}
	for _, generationID := range normalized.generations {
		if !intentArchiveValidSelectorID(generationID) {
			return intentArchivePurgeOptions{}, errors.New("each --generation value must be 64 lowercase hexadecimal characters")
		}
	}
	return normalized, nil
}

func intentArchiveValidSelectorID(value string) bool {
	if len(value) != intentArchiveSHA256HexLength {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func runFeatureIntentArchivePurgePreview(
	cmd *cobra.Command,
	repoRoot, slug string,
	options intentArchivePurgeOptions,
	report intentArchivePurgeReport,
) error {
	root, err := intentArchiveOpenRoot(repoRoot)
	if err != nil {
		report.Outcome = "refused"
		report.Refusal = intentArchiveSimpleRefusal(
			"workspace-not-initialized",
			"The workspace root could not be opened.",
			"Run the command from an initialized tpatch workspace.",
		)
		return emitIntentArchivePurgeReport(cmd, report, 3)
	}
	defer root.Close()
	pendingJournal, markerErr := observeIntentArchiveJournalMarker(root, slug)
	if markerErr != nil || pendingJournal {
		report.Outcome = "refused"
		report.Refusal = intentArchivePendingJournalRefusal(slug)
		return emitIntentArchivePurgeReport(cmd, report, 3)
	}
	normalizedOptions, normalizeErr := normalizeIntentArchivePurgeOptions(options)
	if normalizeErr != nil {
		return normalizeErr
	}
	options = normalizedOptions
	report = newIntentArchivePurgeReport(slug, options)
	storage := intentArchiveNewStorage(nil, root)
	plan, planErr := intentArchivePreviewPurge(storage, slug, options.selector())
	if pathErr := validateIntentArchivePurgePlanReportPaths(plan); pathErr != nil {
		report.Outcome = "refused"
		report.Refusal = intentArchiveRefusalFromError(slug, pathErr, nil, options)
		return emitIntentArchivePurgeReport(cmd, report, 3)
	}
	report = applyIntentArchivePurgePlan(report, plan)
	if planErr != nil {
		exit := prepareArchiveExit(planErr, 3)
		report.Outcome = "refused"
		report.Refusal = intentArchiveRefusalFromError(slug, planErr, &plan, options)
		return emitIntentArchivePurgeReport(cmd, report, exit)
	}
	if plan.RecoveryRequired {
		clearIntentArchiveLowerPrecedenceReport(&report)
		report.Outcome = string(store.IntentArchivePurgeRecoveryRequired)
		report.PendingPurge = buildIntentArchivePendingPurge(slug, plan, options)
		return emitIntentArchivePurgeReport(cmd, report, 0)
	}
	report.Outcome = string(store.IntentArchivePurgePlanned)
	report.Retry = intentArchivePurgeRetry(slug, options, true)
	report.RetryCWD = store.IntentArchiveRepairCWD
	return emitIntentArchivePurgeReport(cmd, report, 0)
}

func runFeatureIntentArchivePurgeConfirmed(
	cmd *cobra.Command,
	repoRoot, slug string,
	options intentArchivePurgeOptions,
	report intentArchivePurgeReport,
) error {
	if !intentArchiveMutationAuthoritySupported() {
		report.Outcome = "refused"
		report.Refusal = intentArchiveSimpleRefusal(
			"prepare-unsupported-platform",
			"This platform cannot establish the workspace mutation authority.",
			"Move the workspace to a supported local filesystem and retry.",
		)
		return emitIntentArchivePurgeReport(cmd, report, 3)
	}
	if beforeLockAcquire != nil {
		beforeLockAcquire()
	}
	authority, err := intentArchiveAcquireAuthority(repoRoot)
	if err != nil {
		code, class := prepareAuthorityError(err)
		message := "The workspace mutation authority could not be established."
		remediation := "Move the workspace to a supported local filesystem and retry."
		if code == "transaction-in-progress" {
			message = "The workspace mutation authority is held by another mutating prepare or archive operation. The holder's identity is unknowable."
			remediation = "Wait for the current holder to finish, then retry."
		}
		if class != "" {
			message += " Class: " + class + "."
		}
		report.Outcome = "refused"
		report.Refusal = intentArchiveSimpleRefusal(code, message, remediation)
		return emitIntentArchivePurgeReport(cmd, report, 3)
	}
	defer authority.Release()
	pendingJournal := false
	markerErr := authority.WithRoot(func(root *os.Root) error {
		var err error
		pendingJournal, err = observeIntentArchiveJournalMarker(root, slug)
		return err
	})
	if markerErr != nil || pendingJournal {
		report.Outcome = "refused"
		report.Refusal = intentArchivePendingJournalRefusal(slug)
		return emitIntentArchivePurgeReport(cmd, report, 3)
	}
	normalizedOptions, normalizeErr := normalizeIntentArchivePurgeOptions(options)
	if normalizeErr != nil {
		return normalizeErr
	}
	options = normalizedOptions
	report = newIntentArchivePurgeReport(slug, options)
	storage := intentArchiveNewStorage(authority, nil)
	indexExists, indexErr := intentArchiveIndexExists(storage, slug)
	if indexErr != nil {
		report.Outcome = "refused"
		report.Refusal = intentArchiveRefusalFromError(slug, indexErr, nil, options)
		return emitIntentArchivePurgeReport(cmd, report, 3)
	}
	if indexExists {
		recovered, recoverErr := intentArchiveRecoverPurge(storage, slug)
		report = applyIntentArchivePurgeResult(report, recovered, options)
		if recoverErr != nil {
			return emitIntentArchivePurgeFailure(cmd, report, recovered, recoverErr, options)
		}
		if recovered.Outcome == store.IntentArchivePurgeRecovered {
			report.Outcome = string(store.IntentArchivePurgeRecovered)
			report.Recovery = &intentArchiveRecoveryReport{
				Kind:            "archive-purge-finalize",
				RestoredEntries: []string{},
				FinalizedHashes: append([]string(nil), recovered.FinalizedHashes...),
				Retry:           intentArchivePurgeRetry(slug, options, true),
				RetryCWD:        store.IntentArchiveRepairCWD,
			}
			report.Advisories = append(report.Advisories, prepareAdvisory(
				"recovered-prior-transaction", "",
				"Recovered pending archive purge hashes; the requested selector was not processed.",
			))
			return emitIntentArchivePurgeReport(cmd, report, 0)
		}
	}
	plan, planErr := intentArchivePlanPurge(storage, slug, options.selector(), true)
	if pathErr := validateIntentArchivePurgePlanReportPaths(plan); pathErr != nil {
		report.Outcome = "refused"
		report.Refusal = intentArchiveRefusalFromError(slug, pathErr, nil, options)
		return emitIntentArchivePurgeReport(cmd, report, 3)
	}
	report = applyIntentArchivePurgePlan(report, plan)
	if planErr != nil {
		exit := prepareArchiveExit(planErr, 3)
		report.Outcome = "refused"
		report.Refusal = intentArchiveRefusalFromError(slug, planErr, &plan, options)
		return emitIntentArchivePurgeReport(cmd, report, exit)
	}
	if plan.RecoveryRequired {
		report.Outcome = "refused"
		report.Refusal = intentArchiveSimpleRefusal(
			string(store.IntentArchiveCodeRecoveryPending),
			"A pending purge appeared before the new selector could run.",
			"Retry the same command to let the owning purge transaction finish first.",
		)
		report.Refusal.Retry = intentArchivePurgeRetry(slug, options, true)
		report.Refusal.RetryCWD = store.IntentArchiveRepairCWD
		return emitIntentArchivePurgeReport(cmd, report, 3)
	}
	if plan.Outcome == store.IntentArchivePurgeNoOp &&
		len(plan.Hashes) == 0 && len(plan.BlobRemovals) == 0 {
		report.Outcome = string(store.IntentArchivePurgeNoOp)
		return emitIntentArchivePurgeReport(cmd, report, 0)
	}
	result, executeErr := intentArchiveExecutePurge(storage, plan)
	report = applyIntentArchivePurgeResult(report, result, options)
	if executeErr != nil {
		return emitIntentArchivePurgeFailure(cmd, report, result, executeErr, options)
	}
	report.Outcome = string(result.Outcome)
	if report.Outcome == "" {
		report.Outcome = string(plan.Outcome)
	}
	return emitIntentArchivePurgeReport(cmd, report, 0)
}

func clearIntentArchiveLowerPrecedenceReport(report *intentArchivePurgeReport) {
	report.Hashes = []string{}
	report.GenerationIDs = []string{}
	report.References = []intentArchivePurgeReferenceReport{}
	report.Blobs = []intentArchivePurgeBlobReport{}
	report.OrphanBlobs = []string{}
	report.Advisories = []prepareAdvisoryReport{}
	report.RemainingRepairs = nil
	report.Retry = ""
	report.RetryCWD = ""
}

func intentArchiveIndexExists(storage store.IntentArchiveStorage, slug string) (bool, error) {
	indexRel, err := store.IntentArchiveIndexRel(slug)
	if err != nil {
		return false, err
	}
	capture, err := storage.CaptureIndex(indexRel)
	if err != nil {
		return false, err
	}
	return capture.Exists, nil
}

func intentArchiveManagedBlobReportPathSafe(slug, value string) bool {
	if !utf8.ValidString(value) ||
		value == "" ||
		value == "." ||
		!fs.ValidPath(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	blobsRel, err := store.IntentArchiveBlobsRel(slug)
	return err == nil && path.Dir(value) == blobsRel
}

func validateIntentArchiveSnapshotReportPaths(snapshot store.IntentArchiveSnapshot) error {
	for _, observation := range snapshot.BlobObservations {
		if !intentArchiveManagedBlobReportPathSafe(snapshot.Feature, observation.Path) {
			return intentArchiveUnsafeReportPathError()
		}
	}
	for _, hash := range snapshot.Inspection.Hashes {
		if !intentArchiveManagedBlobReportPathSafe(snapshot.Feature, hash.Blob.Path) {
			return intentArchiveUnsafeReportPathError()
		}
	}
	for _, orphan := range snapshot.Inspection.Orphans {
		if !intentArchiveManagedBlobReportPathSafe(snapshot.Feature, orphan.Path) {
			return intentArchiveUnsafeReportPathError()
		}
	}
	if err := validateIntentArchiveRepairClassReportPaths(snapshot.Feature, snapshot.Inspection.Classes); err != nil {
		return err
	}
	return nil
}

func validateIntentArchivePurgePlanReportPaths(plan store.IntentArchivePurgePlan) error {
	for _, observation := range plan.BlobRemovals {
		if !intentArchiveManagedBlobReportPathSafe(plan.Feature, observation.Path) {
			return intentArchiveUnsafeReportPathError()
		}
	}
	for _, classes := range [][]store.IntentArchiveRepairClassReport{
		plan.ObservedClasses,
		plan.ResultingClasses,
	} {
		if err := validateIntentArchiveRepairClassReportPaths(plan.Feature, classes); err != nil {
			return err
		}
	}
	if plan.RemainingRepairs != nil {
		for _, stage := range plan.RemainingRepairs.Stages {
			for _, item := range stage.Paths {
				if !intentArchiveManagedBlobReportPathSafe(plan.Feature, item) {
					return intentArchiveUnsafeReportPathError()
				}
			}
		}
	}
	return nil
}

func validateIntentArchiveRepairClassReportPaths(
	slug string,
	classes []store.IntentArchiveRepairClassReport,
) error {
	for _, class := range classes {
		for _, item := range class.Paths {
			if !intentArchiveManagedBlobReportPathSafe(slug, item) {
				return intentArchiveUnsafeReportPathError()
			}
		}
		for _, instance := range class.Instances {
			if !intentArchiveManagedBlobReportPathSafe(slug, instance.Path) {
				return intentArchiveUnsafeReportPathError()
			}
		}
	}
	return nil
}

func intentArchiveUnsafeReportPathError() error {
	return &store.IntentArchiveError{
		Code:      store.IntentArchiveCodeIndexPathEscape,
		Detail:    "the archive contains a managed path that is unsafe to report",
		ExitClass: 3,
	}
}

func (options intentArchivePurgeOptions) selector() store.IntentArchivePurgeSelector {
	return store.IntentArchivePurgeSelector{
		Blobs:       append([]string(nil), options.blobs...),
		Generations: append([]string(nil), options.generations...),
		All:         options.all,
		Orphans:     options.orphans,
	}
}

func (options intentArchivePurgeOptions) selectorKind() string {
	switch {
	case len(options.blobs) != 0:
		return string(store.IntentArchiveSelectorBlob)
	case len(options.generations) != 0:
		return string(store.IntentArchiveSelectorGeneration)
	case options.all:
		return string(store.IntentArchiveSelectorAll)
	case options.orphans:
		return string(store.IntentArchiveSelectorOrphans)
	default:
		return ""
	}
}

func intentArchiveWorkspace(cmd *cobra.Command) (string, string, string) {
	start, _ := cmd.Flags().GetString("path")
	if start == "" {
		start = "."
	}
	repoRoot, err := store.FindProjectRoot(start)
	if err != nil {
		return "", "workspace-not-initialized", "No initialized tpatch workspace was found."
	}
	return repoRoot, "", ""
}

func intentArchiveWorkspaceRemediation(code string) string {
	if code == "workspace-not-initialized" {
		return "Run the command from an initialized tpatch workspace."
	}
	return "Correct the workspace condition and retry."
}

func newIntentArchiveListReport(slug string) intentArchiveListReport {
	index, _ := store.IntentArchiveIndexRel(slug)
	return intentArchiveListReport{
		SchemaVersion:     1,
		Command:           intentArchiveCommandList,
		Slug:              slug,
		Outcome:           "listed",
		Index:             index,
		Generations:       []intentArchiveListGenerationReport{},
		CorruptObjects:    []intentArchiveListCorruptObjectReport{},
		Orphans:           []intentArchiveListOrphanReport{},
		HistoryDisclosure: intentArchiveHistoryDisclosure,
	}
}

func buildIntentArchiveListReport(snapshot store.IntentArchiveSnapshot) intentArchiveListReport {
	report := newIntentArchiveListReport(snapshot.Feature)
	for _, generation := range snapshot.Index.Generations {
		generationReport := intentArchiveListGenerationReport{
			GenerationID: generation.GenerationID,
			Mode:         generation.Mode,
			Entries:      []intentArchiveListEntryReport{},
		}
		for _, replacement := range generation.Replaced {
			hashReport := intentArchiveHashReport(snapshot.Inspection, replacement.ContentSHA256)
			reference := intentArchiveReferenceReport(hashReport, generation.GenerationID, replacement.ArtifactID)
			storageToken := intentArchiveStorageToken(reference.Disposition)
			availability := intentArchiveAvailability(storageToken)
			repair := intentArchiveListRepair(snapshot.Feature, snapshot.Inspection, hashReport, reference)
			liveIDs, tombstoneIDs := intentArchiveGenerationPartitions(hashReport)
			generationReport.Entries = append(generationReport.Entries, intentArchiveListEntryReport{
				ArtifactID:             string(replacement.ArtifactID),
				Path:                   prepareFeatureRel(snapshot.Feature) + "/" + replacement.Path,
				ContentSHA256:          replacement.ContentSHA256,
				Blob:                   replacement.Blob,
				SizeBytes:              replacement.SizeBytes,
				Purged:                 replacement.Purged,
				PurgePending:           replacement.PurgePending,
				Storage:                storageToken,
				Present:                hashReport.Blob.State != store.IntentArchiveBlobAbsent,
				BlobPath:               hashReport.Blob.Path,
				BlobSizeBytes:          hashReport.Blob.SizeBytes,
				Availability:           availability,
				Repair:                 repair.Repair,
				Retry:                  repair.Retry,
				RetryCWD:               repair.RetryCWD,
				LiveGenerationIDs:      liveIDs,
				TombstoneGenerationIDs: tombstoneIDs,
			})
		}
		report.Generations = append(report.Generations, generationReport)
	}
	for _, orphan := range snapshot.Inspection.Orphans {
		retry := "tpatch feature intent-archive purge " + snapshot.Feature + " --orphans --yes"
		report.Orphans = append(report.Orphans, intentArchiveListOrphanReport{
			Hash:      orphan.Hash,
			Path:      orphan.Path,
			SizeBytes: orphan.SizeBytes,
			Present:   true,
			Storage:   "orphan",
			Repair:    retry,
			Retry:     retry,
			RetryCWD:  store.IntentArchiveRepairCWD,
		})
	}
	return report
}

func buildIntentArchiveUnindexedCorruptObjects(
	storage store.IntentArchiveStorage,
	snapshot store.IntentArchiveSnapshot,
) []intentArchiveListCorruptObjectReport {
	indexedHashes := map[string]bool{}
	for _, generation := range snapshot.Index.Generations {
		for _, replacement := range generation.Replaced {
			indexedHashes[replacement.ContentSHA256] = true
		}
	}
	repair := intentArchiveCorruptClassPrerequisite(snapshot.Inspection.Classes)
	retry := ""
	retryCWD := ""
	if intentArchiveCorruptClassPredictsDangling(snapshot.Inspection.Classes) {
		retry = intentArchiveDanglingClassRetry(snapshot.Feature, snapshot.Inspection.Classes)
		retryCWD = store.IntentArchiveRepairCWD
	}
	objects := []intentArchiveListCorruptObjectReport{}
	for _, class := range snapshot.Inspection.Classes {
		if class.Class != store.IntentArchiveRepairCorruptObject {
			continue
		}
		for _, instance := range class.Instances {
			hash := instance.Hash
			if hash == "" {
				hash = intentArchiveHashFromBlobPath(instance.Path)
			}
			if hash != "" && indexedHashes[hash] {
				continue
			}
			kind := "unidentifiable"
			if probe, err := storage.ProbeBlob(instance.Path); err == nil {
				kind = string(probe.Kind)
				if probe.Kind == store.IntentArchiveBlobKindRegular &&
					hash != "" && probe.SHA256 != hash {
					kind = "regular-hash-wrong"
				}
			}
			objects = append(objects, intentArchiveListCorruptObjectReport{
				Hash:     hash,
				Path:     instance.Path,
				Kind:     kind,
				Repair:   repair,
				Retry:    retry,
				RetryCWD: retryCWD,
			})
		}
	}
	sort.Slice(objects, func(left, right int) bool {
		if objects[left].Path != objects[right].Path {
			return objects[left].Path < objects[right].Path
		}
		return objects[left].Hash < objects[right].Hash
	})
	return objects
}

func intentArchiveHashFromBlobPath(blobPath string) string {
	base := path.Base(blobPath)
	if !strings.HasSuffix(base, ".blob") {
		return ""
	}
	hash := strings.TrimSuffix(base, ".blob")
	if !intentArchiveValidSelectorID(hash) {
		return ""
	}
	return hash
}

func intentArchiveHashReport(inspection store.IntentArchiveInspection, hash string) store.IntentArchiveHashReport {
	for _, report := range inspection.Hashes {
		if report.Hash == hash {
			return report
		}
	}
	blobRel := ""
	if hash != "" {
		blobRel, _ = store.IntentArchiveBlobRel("", hash)
	}
	return store.IntentArchiveHashReport{
		Hash: hash,
		Blob: store.IntentArchiveBlobObservation{
			Hash:  hash,
			Path:  blobRel,
			State: store.IntentArchiveBlobAbsent,
		},
		References: []store.IntentArchiveReferenceReport{},
	}
}

func intentArchiveReferenceReport(
	hashReport store.IntentArchiveHashReport,
	generationID string,
	artifactID store.IntentArchiveArtifactID,
) store.IntentArchiveReferenceReport {
	for _, reference := range hashReport.References {
		if reference.GenerationID == generationID && reference.ArtifactID == artifactID {
			return reference
		}
	}
	return store.IntentArchiveReferenceReport{}
}

func intentArchiveStorageToken(disposition store.IntentArchiveDisposition) string {
	switch disposition {
	case store.IntentArchiveDispositionHealthyRetained:
		return "present"
	case store.IntentArchiveDispositionHealthyPurged:
		return "purged"
	case store.IntentArchiveDispositionPendingRemove:
		return "pending-remove"
	case store.IntentArchiveDispositionPendingFinalize:
		return "pending-finalize"
	case store.IntentArchiveDispositionResidue:
		return "orphan"
	case store.IntentArchiveDispositionMixedReference:
		return "mixed-reference"
	case store.IntentArchiveDispositionCorruptObject:
		return "corrupt"
	case store.IntentArchiveDispositionDanglingReference:
		return "dangling"
	default:
		return "corrupt"
	}
}

func intentArchiveAvailability(storageToken string) string {
	switch storageToken {
	case "present":
		return "recoverable from the present archive blob"
	case "pending-remove", "pending-finalize":
		return "owned by a pending purge transaction"
	case "mixed-reference":
		return "recoverable bytes are present, but references disagree"
	case "corrupt":
		return "present but not safely recoverable until the managed object is repaired"
	case "orphan":
		return "present but globally unreferenced"
	default:
		return "not recoverable until identical content is archived again"
	}
}

type intentArchiveRepairPresentation struct {
	Repair   string
	Retry    string
	RetryCWD string
}

func intentArchiveListRepair(
	slug string,
	inspection store.IntentArchiveInspection,
	hashReport store.IntentArchiveHashReport,
	reference store.IntentArchiveReferenceReport,
) intentArchiveRepairPresentation {
	hash := hashReport.Hash
	switch intentArchiveStorageToken(reference.Disposition) {
	case "pending-remove", "pending-finalize":
		return intentArchiveRetryPresentation(
			"tpatch feature intent-archive purge " + slug + " --blob " + hash + " --yes",
		)
	case "mixed-reference":
		return intentArchiveRetryPresentation(intentArchiveClassBlobRetry(
			slug, inspection.Classes, store.IntentArchiveRepairMixedReference,
		))
	case "dangling":
		return intentArchiveRetryPresentation(intentArchiveDanglingClassRetry(slug, inspection.Classes))
	case "orphan":
		return intentArchiveRetryPresentation(
			"tpatch feature intent-archive purge " + slug + " --orphans --yes",
		)
	case "corrupt":
		presentation := intentArchiveRepairPresentation{
			Repair: intentArchiveCorruptClassPrerequisite(inspection.Classes),
		}
		if intentArchiveCorruptClassPredictsDangling(inspection.Classes) {
			presentation.Retry = intentArchiveDanglingClassRetry(slug, inspection.Classes)
			presentation.RetryCWD = store.IntentArchiveRepairCWD
		}
		return presentation
	default:
		return intentArchiveRepairPresentation{}
	}
}

func intentArchiveRetryPresentation(retry string) intentArchiveRepairPresentation {
	if retry == "" {
		return intentArchiveRepairPresentation{}
	}
	return intentArchiveRepairPresentation{
		Repair:   retry,
		Retry:    retry,
		RetryCWD: store.IntentArchiveRepairCWD,
	}
}

func intentArchiveClassBlobRetry(
	slug string,
	classes []store.IntentArchiveRepairClassReport,
	className store.IntentArchiveRepairClass,
) string {
	for _, class := range classes {
		if class.Class == className {
			return intentArchiveBlobRetry(slug, class.Hashes)
		}
	}
	return ""
}

func intentArchiveDanglingClassRetry(slug string, classes []store.IntentArchiveRepairClassReport) string {
	hashes := []string{}
	for _, class := range classes {
		switch class.Class {
		case store.IntentArchiveRepairDanglingReference:
			hashes = append(hashes, class.Hashes...)
		case store.IntentArchiveRepairCorruptObject:
			for _, instance := range class.Instances {
				if instance.Hash != "" &&
					intentArchiveContainsRepairClass(instance.ResultingClasses, store.IntentArchiveRepairDanglingReference) {
					hashes = append(hashes, instance.Hash)
				}
			}
		}
	}
	return intentArchiveBlobRetry(slug, sortedUniqueIntentArchiveStrings(hashes))
}

func intentArchiveBlobRetry(slug string, hashes []string) string {
	if len(hashes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("tpatch feature intent-archive purge ")
	b.WriteString(slug)
	for _, hash := range sortedUniqueIntentArchiveStrings(hashes) {
		b.WriteString(" --blob ")
		b.WriteString(hash)
	}
	b.WriteString(" --yes")
	return b.String()
}

func intentArchiveCorruptClassPredictsDangling(classes []store.IntentArchiveRepairClassReport) bool {
	for _, class := range classes {
		if class.Class != store.IntentArchiveRepairCorruptObject {
			continue
		}
		for _, instance := range class.Instances {
			if intentArchiveContainsRepairClass(instance.ResultingClasses, store.IntentArchiveRepairDanglingReference) {
				return true
			}
		}
	}
	return false
}

func intentArchiveContainsRepairClass(
	classes []store.IntentArchiveRepairClass,
	want store.IntentArchiveRepairClass,
) bool {
	for _, class := range classes {
		if class == want {
			return true
		}
	}
	return false
}

func intentArchiveCorruptClassPrerequisite(classes []store.IntentArchiveRepairClassReport) string {
	for _, class := range classes {
		if class.Class != store.IntentArchiveRepairCorruptObject {
			continue
		}
		parts := []string{
			"Remove every corrupt-object instance below before running any tpatch repair selector in this archive.",
			"Restoring each exact hash-correct managed object instead also resolves its observation.",
		}
		for _, instance := range class.Instances {
			parts = append(parts, intentArchiveCorruptRemovalText(instance.Path))
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func intentArchiveGenerationPartitions(hashReport store.IntentArchiveHashReport) ([]string, []string) {
	live := []string{}
	tombstoned := []string{}
	for _, reference := range hashReport.References {
		switch reference.WireState {
		case store.IntentArchiveWireRetained, store.IntentArchiveWireRemovalPending:
			live = append(live, reference.GenerationID)
		case store.IntentArchiveWireTombstoned:
			tombstoned = append(tombstoned, reference.GenerationID)
		}
	}
	return sortedUniqueIntentArchiveStrings(live), sortedUniqueIntentArchiveStrings(tombstoned)
}

func intentArchiveListExit(inspection store.IntentArchiveInspection) int {
	for _, class := range inspection.Classes {
		if class.Class != store.IntentArchiveRepairUnreferencedResidue {
			return 3
		}
	}
	return 0
}

func intentArchiveListInspectionRefusal(slug string, inspection store.IntentArchiveInspection) *intentArchiveRefusalReport {
	for _, class := range inspection.Classes {
		if class.Class == store.IntentArchiveRepairUnreferencedResidue {
			continue
		}
		code := string(store.IntentArchiveCodeIndexStorageInconsistent)
		switch class.Class {
		case store.IntentArchiveRepairCorruptObject:
			code = string(store.IntentArchiveCodeBlobCorrupt)
		case store.IntentArchiveRepairDanglingReference:
			code = string(store.IntentArchiveCodeBlobDangling)
		}
		return intentArchiveSimpleRefusal(
			code,
			"The archive inventory contains one or more untrustworthy storage observations.",
			intentArchiveClassRepairText(slug, class),
		)
	}
	return nil
}

func newIntentArchivePurgeReport(slug string, options intentArchivePurgeOptions) intentArchivePurgeReport {
	report := intentArchivePurgeReport{
		SchemaVersion:     1,
		Command:           intentArchiveCommandPurge,
		Slug:              slug,
		Outcome:           "refused",
		Action:            "none",
		Selector:          options.selectorKind(),
		Confirmed:         options.yes,
		Hashes:            []string{},
		GenerationIDs:     []string{},
		References:        []intentArchivePurgeReferenceReport{},
		Blobs:             []intentArchivePurgeBlobReport{},
		OrphanBlobs:       []string{},
		Advisories:        []prepareAdvisoryReport{},
		HistoryDisclosure: intentArchiveHistoryDisclosure,
	}
	if options.all {
		report.BlastRadius = intentArchiveAllBlastRadius
	}
	return report
}

func applyIntentArchivePurgePlan(report intentArchivePurgeReport, plan store.IntentArchivePurgePlan) intentArchivePurgeReport {
	if plan.SelectorKind != "" {
		report.Selector = string(plan.SelectorKind)
	}
	report.Hashes = append([]string(nil), plan.Hashes...)
	report.GenerationIDs = append([]string(nil), plan.GenerationIDs...)
	report.References = []intentArchivePurgeReferenceReport{}
	for _, reference := range plan.References {
		report.References = append(report.References, intentArchivePurgeReferenceReport{
			GenerationID: reference.GenerationID,
			ArtifactID:   string(reference.ArtifactID),
			Path:         prepareFeatureRel(plan.Feature) + "/" + reference.Path,
			Hash:         reference.Hash,
			WireState:    string(reference.WireState),
		})
	}
	report.Blobs = []intentArchivePurgeBlobReport{}
	for _, blob := range plan.BlobRemovals {
		report.Blobs = append(report.Blobs, intentArchivePurgeBlobReport{
			Hash:      blob.Hash,
			Path:      blob.Path,
			SizeBytes: blob.SizeBytes,
			Present:   blob.State != store.IntentArchiveBlobAbsent,
			Removed:   false,
		})
	}
	report.RemainingRepairs = buildIntentArchiveRemainingRepairsReport(plan.RemainingRepairs)
	classes := plan.ResultingClasses
	if plan.Preview {
		classes = plan.ObservedClasses
	}
	report.OrphanBlobs = intentArchiveClassOrphanHashes(classes)
	return report
}

func applyIntentArchivePurgeResult(
	report intentArchivePurgeReport,
	result store.IntentArchivePurgeResult,
	options intentArchivePurgeOptions,
) intentArchivePurgeReport {
	report.Outcome = string(result.Outcome)
	report.RemainingRepairs = nil
	report.Advisories = intentArchiveWithoutRepairAdvisory(report.Advisories)
	for index := range report.Blobs {
		if intentArchiveContainsString(result.RemovedBlobs, report.Blobs[index].Path) {
			report.Blobs[index].Present = false
			report.Blobs[index].Removed = true
		}
	}
	if result.Resume == store.IntentArchiveResumeOrphanScan {
		report.OrphanBlobs = append([]string(nil), result.RemainingHashes...)
	}
	if result.Outcome == store.IntentArchivePurgePurged && result.RemainingRepairs != nil {
		report.RemainingRepairs = buildIntentArchiveRemainingRepairsReport(result.RemainingRepairs)
		report.Advisories = append(report.Advisories, prepareAdvisory(
			"archive-repairs-remaining", "",
			fmt.Sprintf(
				"Repaired %s; %d repair stage(s) remain and the archive is not consistent until they are completed.",
				result.RemainingRepairs.RepairedClass,
				result.RemainingRepairs.StagesRemaining,
			),
		))
	}
	if result.Outcome == store.IntentArchivePurgePartial {
		report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)
	}
	return report
}

func intentArchiveWithoutRepairAdvisory(advisories []prepareAdvisoryReport) []prepareAdvisoryReport {
	filtered := make([]prepareAdvisoryReport, 0, len(advisories))
	for _, advisory := range advisories {
		if advisory.Code != "archive-repairs-remaining" {
			filtered = append(filtered, advisory)
		}
	}
	return filtered
}

func buildIntentArchivePendingPurge(
	slug string,
	plan store.IntentArchivePurgePlan,
	options intentArchivePurgeOptions,
) *intentArchivePendingPurgeReport {
	indexRel, _ := store.IntentArchiveIndexRel(slug)
	pending := &intentArchivePendingPurgeReport{
		RecoveryRequired: true,
		PendingHashes:    []intentArchivePendingHashReport{},
		Selector:         options.selectorKind(),
		Retry:            intentArchivePurgeRetry(slug, options, true),
		RetryCWD:         store.IntentArchiveRepairCWD,
	}
	for _, hash := range plan.PendingHashes {
		blobRel, _ := store.IntentArchiveBlobRel(slug, hash)
		pending.PendingHashes = append(pending.PendingHashes, intentArchivePendingHashReport{
			Hash:  hash,
			Blob:  blobRel,
			Index: indexRel,
			Plan:  intentArchivePendingPlan,
		})
	}
	return pending
}

func buildIntentArchivePurgeProgress(
	result store.IntentArchivePurgeResult,
	slug string,
	options intentArchivePurgeOptions,
) *intentArchivePurgeProgressReport {
	return &intentArchivePurgeProgressReport{
		CompletedHashes: append([]string(nil), result.CompletedHashes...),
		PendingHash:     result.PendingHash,
		RemainingHashes: append([]string(nil), result.RemainingHashes...),
		Resume:          string(result.Resume),
		Retry:           intentArchivePurgeRetry(slug, options, true),
		RetryCWD:        store.IntentArchiveRepairCWD,
		State:           result.State,
	}
}

func buildIntentArchiveRemainingRepairsReport(
	remaining *store.IntentArchiveRemainingRepairs,
) *intentArchiveRemainingRepairsReport {
	if remaining == nil {
		return nil
	}
	report := &intentArchiveRemainingRepairsReport{
		RerunRequired:   remaining.RerunRequired,
		RepairedClass:   string(remaining.RepairedClass),
		StagesRemaining: remaining.StagesRemaining,
		Stages:          []intentArchiveRepairStageReport{},
	}
	if remaining.NextStage != nil {
		report.NextStage = &intentArchiveRepairNextReport{
			Ordinal: remaining.NextStage.Ordinal,
			Kind:    string(remaining.NextStage.Kind),
			Class:   string(remaining.NextStage.Class),
		}
	}
	for _, stage := range remaining.Stages {
		resulting := []string{}
		for _, class := range stage.ResultingClasses {
			resulting = append(resulting, string(class))
		}
		repair := stage.Repair
		if stage.Class == store.IntentArchiveRepairCorruptObject &&
			!strings.Contains(repair, "No tpatch repair selector runs anywhere in this archive until that removal has happened.") {
			repair = "No tpatch repair selector runs anywhere in this archive until that removal has happened.\n" + repair
		}
		report.Stages = append(report.Stages, intentArchiveRepairStageReport{
			Ordinal:           stage.Ordinal,
			Kind:              string(stage.Kind),
			Class:             string(stage.Class),
			Hashes:            append([]string(nil), stage.Hashes...),
			Paths:             append([]string(nil), stage.Paths...),
			Repair:            repair,
			RepairCWD:         stage.RepairCWD,
			ResultingClasses:  resulting,
			AfterPrerequisite: stage.AfterPrerequisite,
		})
	}
	return report
}

func emitIntentArchivePurgeFailure(
	cmd *cobra.Command,
	report intentArchivePurgeReport,
	result store.IntentArchivePurgeResult,
	err error,
	options intentArchivePurgeOptions,
) error {
	var typed *store.IntentArchiveError
	exit := prepareArchiveExit(err, 1)
	if errors.As(err, &typed) && typed.Code == store.IntentArchiveCodePurgePartial {
		report.Outcome = string(store.IntentArchivePurgePartial)
		report.RemainingRepairs = nil
		report.Advisories = intentArchiveWithoutRepairAdvisory(report.Advisories)
		report.PurgeProgress = buildIntentArchivePurgeProgress(result, report.Slug, options)
		return emitIntentArchivePurgeReport(cmd, report, 5)
	}
	report.Outcome = "refused"
	report.Refusal = intentArchiveRefusalFromError(report.Slug, err, nil, options)
	if errors.As(err, &typed) && typed.Code == store.IntentArchiveCodePurgeEvidenceDivergent {
		report.PurgeProgress = nil
		report.RemainingRepairs = nil
		report.Advisories = intentArchiveWithoutRepairAdvisory(report.Advisories)
		report.Divergence = buildIntentArchiveDivergence(report.Slug, result, typed, options)
		if report.Divergence.Kind == "index" {
			report.Blobs = []intentArchivePurgeBlobReport{}
			report.OrphanBlobs = []string{}
		}
		return emitIntentArchivePurgeReport(cmd, report, 6)
	}
	return emitIntentArchivePurgeReport(cmd, report, exit)
}

func buildIntentArchiveDivergence(
	slug string,
	result store.IntentArchivePurgeResult,
	typed *store.IntentArchiveError,
	options intentArchivePurgeOptions,
) *intentArchiveDivergenceReport {
	hash := typed.Hash
	if hash == "" {
		hash = result.PendingHash
	}
	indexRel, _ := store.IntentArchiveIndexRel(slug)
	kind := "blob"
	if strings.Contains(typed.Detail, "index.json") || strings.Contains(typed.Detail, "strict-decod") {
		kind = "index"
	}
	report := &intentArchiveDivergenceReport{
		Kind:            kind,
		PendingHash:     hash,
		Index:           indexRel,
		CompletedHashes: append([]string(nil), result.CompletedHashes...),
		RemainingHashes: append([]string(nil), result.RemainingHashes...),
		Retry:           intentArchivePurgeRetry(slug, options, true),
		RetryCWD:        store.IntentArchiveRepairCWD,
		Cost:            "What this costs: what you remove is gone, and this hash has no archived recovery material afterwards. If that blob was ever committed, it is still in this repository's Git history; removing it from history is not something tpatch does.",
	}
	if kind == "index" {
		report.Warning = "The archive index stopped strict-decoding after purge ownership was established."
		report.RestoreInstruction = "Restore index.json to bytes that strict-decode using your own version control or backup, then rerun the purge. Do not remove index.json."
		report.Cost = "Restoring the strict index preimage preserves the archive generations; removing index.json would discard them. Archived blob bytes that were ever committed remain in this repository's Git history; removing them from history is not something tpatch does."
		return report
	}
	blobRel, _ := store.IntentArchiveBlobRel(slug, hash)
	report.Blob = blobRel
	report.Warning = "tpatch will not delete or overwrite bytes it cannot identify. WARNING: the next command permanently deletes whatever object is at the managed blob path, including directory contents, with no undo. If you want to keep that object, stop here and preserve it with tooling appropriate to its type; tpatch does not name a preservation command."
	report.RemoveCommand = "rm -rf -- " + blobRel
	return report
}

func intentArchiveRefusalFromError(
	slug string,
	err error,
	plan *store.IntentArchivePurgePlan,
	options intentArchivePurgeOptions,
) *intentArchiveRefusalReport {
	var typed *store.IntentArchiveError
	if !errors.As(err, &typed) {
		return intentArchiveSimpleRefusal(
			"archive-index-corrupt",
			"The intent archive could not be inspected safely.",
			"Preserve the archive bytes, correct the reported condition, and retry.",
		)
	}
	code := string(typed.Code)
	if typed.Code == store.IntentArchiveCodeSelectorInvalid {
		code = string(store.IntentArchiveCodeIndexCorrupt)
	}
	message := "The intent archive refused the requested operation."
	if typed.Hash != "" {
		message += " Hash: " + typed.Hash + "."
	}
	if typed.GenerationID != "" {
		message += " Generation: " + typed.GenerationID + "."
	}
	remediation := "Preserve the archive bytes, correct the reported condition, and retry."
	retry := ""
	switch typed.Code {
	case store.IntentArchiveCodeRecoveryPending:
		remediation = "Retry the same confirmed purge so the owning pending-hash transaction can finish first."
		retry = intentArchivePurgeRetry(slug, options, true)
	case store.IntentArchiveCodeBlobDangling:
		if plan != nil && plan.RemainingRepairs != nil {
			remediation = intentArchiveRemainingRepairsText(plan.RemainingRepairs)
		} else {
			retry = "tpatch feature intent-archive purge " + slug + " --blob " + typed.Hash + " --yes"
			remediation = "The hash is not recoverable until identical content is archived again. Run the retry below from the workspace root to tombstone every reference to it."
		}
	case store.IntentArchiveCodeIndexStorageInconsistent:
		if plan != nil && plan.RemainingRepairs != nil {
			remediation = intentArchiveRemainingRepairsText(plan.RemainingRepairs)
		} else if typed.Class == string(store.IntentArchiveRepairUnreferencedResidue) {
			retry = "tpatch feature intent-archive purge " + slug + " --orphans --yes"
			remediation = "Run the retry below from the workspace root to remove every validated globally unreferenced blob."
		} else if typed.Hash != "" {
			retry = "tpatch feature intent-archive purge " + slug + " --blob " + typed.Hash + " --yes"
			remediation = "Run the retry below from the workspace root to claim and tombstone every reference to the live mixed hash."
		}
	case store.IntentArchiveCodeBlobCorrupt:
		if plan != nil && plan.RemainingRepairs != nil {
			remediation = intentArchiveRemainingRepairsText(plan.RemainingRepairs)
		} else if typed.Hash != "" {
			blobRel, _ := store.IntentArchiveBlobRel(slug, typed.Hash)
			remediation = intentArchiveCorruptRepairText(slug, typed.Hash, blobRel)
		}
	case store.IntentArchiveCodeBlobShared:
		retry = "tpatch feature intent-archive purge " + slug + " --blob " + typed.Hash + " --yes"
		allPreview := "tpatch feature intent-archive purge " + slug + " --all"
		allConfirmed := allPreview + " --yes"
		remediation = "The narrow repair is " + retry + ". Preview the broader selector first with " +
			allPreview + "; only after reviewing that full selection confirm with " + allConfirmed +
			". " + intentArchiveAllBlastRadius
	case store.IntentArchiveCodePurgeIndexChanged:
		remediation = "Retry from the newly observed archive tree."
	case store.IntentArchiveCodePurgeEvidenceDivergent:
		remediation = "Follow the archive-specific divergence procedure in this report; prepare recovery modes cannot repair an archive purge."
	}
	report := intentArchiveSimpleRefusal(code, message, remediation)
	if retry != "" {
		report.Retry = retry
		report.RetryCWD = store.IntentArchiveRepairCWD
	}
	return report
}

func intentArchivePendingJournalRefusal(slug string) *intentArchiveRefusalReport {
	report := intentArchiveSimpleRefusal(
		"recovery-pending",
		"An interrupted prepare journal exists. This retention command did not decode, move, consume, or recover it.",
		"Run the retry below to recover the journal. Alternatively, run prepare for this slug with --abandon-transaction --yes.",
	)
	report.Retry = "tpatch prepare " + slug
	report.RetryCWD = store.IntentArchiveRepairCWD
	return report
}

func intentArchiveSimpleRefusal(code, message, remediation string) *intentArchiveRefusalReport {
	return &intentArchiveRefusalReport{
		Code:        code,
		Message:     message,
		Remediation: remediation,
	}
}

func intentArchiveClassRepairText(slug string, class store.IntentArchiveRepairClassReport) string {
	switch class.Class {
	case store.IntentArchiveRepairCorruptObject:
		return "Remove every corrupt-object instance listed in the observations before running any tpatch repair selector in this archive."
	case store.IntentArchiveRepairDanglingReference, store.IntentArchiveRepairMixedReference:
		return "Use the retry printed with the affected observations from the workspace root."
	case store.IntentArchiveRepairUnreferencedResidue:
		return "Use the --orphans retry printed with the orphan observations from the workspace root."
	default:
		return "Preserve the archive and repair the reported storage condition."
	}
}

func intentArchiveCorruptRepairText(slug, hash, blobRel string) string {
	if blobRel == "" && hash != "" {
		blobRel, _ = store.IntentArchiveBlobRel(slug, hash)
	}
	return intentArchiveCorruptRemovalText(blobRel) +
		"\nAfter the complete corrupt-object class has been removed, follow the report's structured remaining-repair stages. Restoring the exact hash-correct object instead also resolves this observation. " +
		intentArchiveHistoryDisclosure
}

func intentArchiveCorruptRemovalText(blobRel string) string {
	quotedBlobRel := quoteIntentArchivePOSIXShell(blobRel)
	return "WARNING: destructive archive repair. The next command permanently deletes whatever object is at the single managed path " +
		quotedBlobRel + ", including directory contents, with no undo. Stop before running it if you need to preserve that object with tooling appropriate to its type. " +
		"No tpatch repair selector runs anywhere in this archive until every corrupt-object removal has happened. Run this prerequisite from the workspace root:\nrm -rf -- " + quotedBlobRel
}

func quoteIntentArchivePOSIXShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func intentArchiveRemainingRepairsText(remaining *store.IntentArchiveRemainingRepairs) string {
	if remaining == nil {
		return ""
	}
	return fmt.Sprintf(
		"Follow the %d structured remaining-repair stage(s) in rank order. Complete every manual prerequisite before running any tpatch retry printed below it.",
		remaining.StagesRemaining,
	)
}

func intentArchivePurgeRetry(slug string, options intentArchivePurgeOptions, confirmed bool) string {
	argv := []string{"tpatch", "feature", "intent-archive", "purge", slug}
	switch {
	case len(options.blobs) != 0:
		for _, hash := range options.blobs {
			argv = append(argv, "--blob", hash)
		}
	case len(options.generations) != 0:
		for _, generationID := range options.generations {
			argv = append(argv, "--generation", generationID)
		}
	case options.orphans:
		argv = append(argv, "--orphans")
	case options.all:
		argv = append(argv, "--all")
	}
	if confirmed {
		argv = append(argv, "--yes")
	}
	if options.asJSON {
		argv = append(argv, "--json")
	}
	if options.quiet {
		argv = append(argv, "--quiet")
	}
	return strings.Join(argv, " ")
}

func emitIntentArchiveListReport(cmd *cobra.Command, report intentArchiveListReport, exit int) error {
	if report.Generations == nil {
		report.Generations = []intentArchiveListGenerationReport{}
	}
	if report.CorruptObjects == nil {
		report.CorruptObjects = []intentArchiveListCorruptObjectReport{}
	}
	if report.Orphans == nil {
		report.Orphans = []intentArchiveListOrphanReport{}
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
			writeIntentArchiveListHuman(cmd.ErrOrStderr(), report)
		}
	} else if quiet {
		detail := ""
		if report.Refusal != nil {
			detail = " " + report.Refusal.Code
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s%s\n", intentArchiveCommandList, report.Slug, report.Outcome, detail)
	} else {
		writeIntentArchiveListHuman(cmd.OutOrStdout(), report)
	}
	if exit == 0 {
		return nil
	}
	code := ""
	if report.Refusal != nil {
		code = " " + report.Refusal.Code
	}
	return &ExitCodeError{
		Code:    exit,
		Message: fmt.Sprintf("%s %s: %s%s", intentArchiveCommandList, report.Slug, report.Outcome, code),
	}
}

func emitIntentArchivePurgeReport(cmd *cobra.Command, report intentArchivePurgeReport, exit int) error {
	normalizeIntentArchivePurgeReport(&report)
	asJSON, _ := cmd.Flags().GetBool("json")
	quiet, _ := cmd.Flags().GetBool("quiet")
	if asJSON {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return err
		}
		if !quiet {
			writeIntentArchivePurgeHuman(cmd.ErrOrStderr(), report)
		}
	} else if quiet {
		writeIntentArchivePurgeQuiet(cmd.OutOrStdout(), report)
	} else {
		writeIntentArchivePurgeHuman(cmd.OutOrStdout(), report)
	}
	if exit == 0 {
		return nil
	}
	detail := ""
	if report.Refusal != nil {
		detail = " " + report.Refusal.Code
	}
	return &ExitCodeError{
		Code:    exit,
		Message: fmt.Sprintf("%s %s: %s%s", intentArchiveCommandPurge, report.Slug, report.Outcome, detail),
	}
}

func normalizeIntentArchivePurgeReport(report *intentArchivePurgeReport) {
	if report.Hashes == nil {
		report.Hashes = []string{}
	}
	if report.GenerationIDs == nil {
		report.GenerationIDs = []string{}
	}
	if report.References == nil {
		report.References = []intentArchivePurgeReferenceReport{}
	}
	if report.Blobs == nil {
		report.Blobs = []intentArchivePurgeBlobReport{}
	}
	if report.OrphanBlobs == nil {
		report.OrphanBlobs = []string{}
	}
	if report.Advisories == nil {
		report.Advisories = []prepareAdvisoryReport{}
	}
	if report.Recovery != nil {
		if report.Recovery.RestoredEntries == nil {
			report.Recovery.RestoredEntries = []string{}
		}
		if report.Recovery.FinalizedHashes == nil {
			report.Recovery.FinalizedHashes = []string{}
		}
	}
	if report.PurgeProgress != nil {
		if report.PurgeProgress.CompletedHashes == nil {
			report.PurgeProgress.CompletedHashes = []string{}
		}
		if report.PurgeProgress.RemainingHashes == nil {
			report.PurgeProgress.RemainingHashes = []string{}
		}
	}
	if report.PendingPurge != nil && report.PendingPurge.PendingHashes == nil {
		report.PendingPurge.PendingHashes = []intentArchivePendingHashReport{}
	}
	if report.Divergence != nil {
		if report.Divergence.CompletedHashes == nil {
			report.Divergence.CompletedHashes = []string{}
		}
		if report.Divergence.RemainingHashes == nil {
			report.Divergence.RemainingHashes = []string{}
		}
	}
}

func writeIntentArchiveListHuman(w io.Writer, report intentArchiveListReport) {
	fmt.Fprintf(w, "feature intent-archive list %s: %s\n", report.Slug, report.Outcome)
	fmt.Fprintf(w, "index: %s\n", report.Index)
	for _, generation := range report.Generations {
		fmt.Fprintf(w, "generation %s (%s):\n", generation.GenerationID, generation.Mode)
		for _, entry := range generation.Entries {
			fmt.Fprintf(w, "  %s %s\n", entry.ArtifactID, entry.Path)
			fmt.Fprintf(w, "    blob: %s\n", entry.ContentSHA256)
			fmt.Fprintf(w, "    size: %d\n", entry.SizeBytes)
			fmt.Fprintf(w, "    present: %t\n", entry.Present)
			fmt.Fprintf(w, "    storage: %s\n", entry.Storage)
			fmt.Fprintf(w, "    blob path: %s\n", entry.BlobPath)
			fmt.Fprintf(w, "    blob size: %d\n", entry.BlobSizeBytes)
			fmt.Fprintf(w, "    availability: %s\n", entry.Availability)
			if len(entry.LiveGenerationIDs) != 0 {
				fmt.Fprintf(w, "    live generations: %s\n", strings.Join(entry.LiveGenerationIDs, ", "))
			}
			if len(entry.TombstoneGenerationIDs) != 0 {
				fmt.Fprintf(w, "    tombstone generations: %s\n", strings.Join(entry.TombstoneGenerationIDs, ", "))
			}
		}
	}
	fmt.Fprintln(w, "corrupt objects:")
	for _, object := range report.CorruptObjects {
		if object.Hash != "" {
			fmt.Fprintf(w, "  %s\n", object.Hash)
		} else {
			fmt.Fprintln(w, "  unindexed")
		}
		fmt.Fprintf(w, "    path: %s\n", object.Path)
		fmt.Fprintf(w, "    kind: %s\n", object.Kind)
	}
	fmt.Fprintln(w, "orphans:")
	for _, orphan := range report.Orphans {
		fmt.Fprintf(w, "  %s\n", orphan.Hash)
		fmt.Fprintf(w, "    path: %s\n", orphan.Path)
		fmt.Fprintf(w, "    size: %d\n", orphan.SizeBytes)
		fmt.Fprintf(w, "    present: %t\n", orphan.Present)
		fmt.Fprintf(w, "    storage: %s\n", orphan.Storage)
	}
	writeIntentArchiveListRepairs(w, report)
	if report.Refusal != nil {
		writeIntentArchiveRefusal(w, report.Refusal)
	}
	fmt.Fprintln(w, report.HistoryDisclosure)
}

type intentArchiveHumanRepair struct {
	priority int
	repair   string
	retry    string
}

func writeIntentArchiveListRepairs(w io.Writer, report intentArchiveListReport) {
	repairs := []intentArchiveHumanRepair{}
	for _, generation := range report.Generations {
		for _, entry := range generation.Entries {
			repairs = append(repairs, intentArchiveHumanRepair{
				priority: intentArchiveRepairPriority(entry.Storage),
				repair:   entry.Repair,
				retry:    entry.Retry,
			})
		}
	}
	for _, orphan := range report.Orphans {
		repairs = append(repairs, intentArchiveHumanRepair{
			priority: intentArchiveRepairPriority(orphan.Storage),
			repair:   orphan.Repair,
			retry:    orphan.Retry,
		})
	}
	for _, object := range report.CorruptObjects {
		repairs = append(repairs, intentArchiveHumanRepair{
			priority: intentArchiveRepairPriority("corrupt"),
			repair:   object.Repair,
			retry:    object.Retry,
		})
	}
	sort.SliceStable(repairs, func(left, right int) bool {
		if repairs[left].priority != repairs[right].priority {
			return repairs[left].priority < repairs[right].priority
		}
		if repairs[left].repair != repairs[right].repair {
			return repairs[left].repair < repairs[right].repair
		}
		return repairs[left].retry < repairs[right].retry
	})
	seenRepair := map[string]bool{}
	seenRetry := map[string]bool{}
	printed := false
	for _, item := range repairs {
		if item.repair != "" && item.repair != item.retry && !seenRepair[item.repair] {
			if !printed {
				fmt.Fprintln(w, "repairs:")
				printed = true
			}
			fmt.Fprintf(w, "  %s\n", strings.ReplaceAll(item.repair, "\n", "\n  "))
			seenRepair[item.repair] = true
		}
		if item.retry != "" && !seenRetry[item.retry] {
			if !printed {
				fmt.Fprintln(w, "repairs:")
				printed = true
			}
			writeIntentArchiveRetry(w, item.retry)
			seenRetry[item.retry] = true
		}
	}
}

func intentArchiveRepairPriority(storage string) int {
	switch storage {
	case "pending-remove", "pending-finalize":
		return 0
	case "corrupt":
		return 1
	case "dangling":
		return 2
	case "mixed-reference":
		return 3
	case "orphan":
		return 4
	default:
		return 5
	}
}

func writeIntentArchivePurgeQuiet(w io.Writer, report intentArchivePurgeReport) {
	detail := ""
	switch {
	case report.Refusal != nil:
		detail = " " + report.Refusal.Code
	case report.PendingPurge != nil:
		detail = fmt.Sprintf(" (%d pending hash)", len(report.PendingPurge.PendingHashes))
	case report.Recovery != nil:
		detail = fmt.Sprintf(" (%d hashes finalized)", len(report.Recovery.FinalizedHashes))
	case report.PurgeProgress != nil:
		detail = " " + report.PurgeProgress.Resume
	case report.Outcome == string(store.IntentArchivePurgePurged):
		remaining := 0
		removed := 0
		for _, blob := range report.Blobs {
			if blob.Removed {
				removed++
			}
		}
		if report.RemainingRepairs != nil {
			remaining = report.RemainingRepairs.StagesRemaining
		}
		detail = fmt.Sprintf(" (%d blobs removed, %d repair stages remaining)", removed, remaining)
	}
	fmt.Fprintf(w, "%s %s: %s%s\n", intentArchiveCommandPurge, report.Slug, report.Outcome, detail)
}

func writeIntentArchivePurgeHuman(w io.Writer, report intentArchivePurgeReport) {
	detail := ""
	if report.Refusal != nil {
		detail = " " + report.Refusal.Code
	}
	fmt.Fprintf(w, "%s %s: %s%s\n", intentArchiveCommandPurge, report.Slug, report.Outcome, detail)
	fmt.Fprintf(w, "selector: %s\n", report.Selector)
	fmt.Fprintf(w, "confirmed: %t\n", report.Confirmed)
	if len(report.GenerationIDs) != 0 {
		fmt.Fprintln(w, "generations:")
		for _, generationID := range report.GenerationIDs {
			fmt.Fprintf(w, "  %s\n", generationID)
		}
	}
	if len(report.Hashes) != 0 {
		fmt.Fprintln(w, "hashes:")
		for _, hash := range report.Hashes {
			fmt.Fprintf(w, "  %s\n", hash)
		}
	}
	if len(report.References) != 0 {
		fmt.Fprintln(w, "references:")
		for _, reference := range report.References {
			fmt.Fprintf(w, "  %s %s %s %s %s\n",
				reference.GenerationID, reference.ArtifactID, reference.Path, reference.Hash, reference.WireState)
		}
	}
	if len(report.Blobs) != 0 {
		fmt.Fprintln(w, "blobs:")
		for _, blob := range report.Blobs {
			fmt.Fprintf(w, "  %s %s size=%d present=%t removed=%t\n", blob.Hash, blob.Path, blob.SizeBytes, blob.Present, blob.Removed)
		}
	}
	if len(report.OrphanBlobs) != 0 {
		fmt.Fprintln(w, "orphan blobs:")
		for _, hash := range report.OrphanBlobs {
			fmt.Fprintf(w, "  %s\n", hash)
		}
	}
	if report.BlastRadius != "" {
		fmt.Fprintln(w, report.BlastRadius)
	}
	if report.PendingPurge != nil {
		fmt.Fprintln(w, "A previous purge stopped with pending references. Nothing was changed.")
		for _, pending := range report.PendingPurge.PendingHashes {
			fmt.Fprintf(w, "  pending hash: %s\n", pending.Hash)
			fmt.Fprintf(w, "    blob:  %s\n", pending.Blob)
			fmt.Fprintf(w, "    index: %s\n", pending.Index)
			fmt.Fprintf(w, "    plan:  %s\n", pending.Plan)
		}
		writeIntentArchiveRetry(w, report.PendingPurge.Retry)
	}
	if report.Recovery != nil {
		fmt.Fprintln(w, "Recovered pending purge state. The requested selector was not processed.")
		for _, hash := range report.Recovery.FinalizedHashes {
			fmt.Fprintf(w, "  finalized %s\n", hash)
		}
		writeIntentArchiveRetry(w, report.Recovery.Retry)
	}
	if report.PurgeProgress != nil {
		fmt.Fprintf(w, "Purge state: %s\n", report.PurgeProgress.State)
		fmt.Fprintf(w, "Resume: %s\n", report.PurgeProgress.Resume)
		for _, hash := range report.PurgeProgress.CompletedHashes {
			fmt.Fprintf(w, "  completed %s\n", hash)
		}
		if report.PurgeProgress.PendingHash != "" {
			fmt.Fprintf(w, "  pending %s\n", report.PurgeProgress.PendingHash)
		}
		for _, hash := range report.PurgeProgress.RemainingHashes {
			fmt.Fprintf(w, "  remaining %s\n", hash)
		}
		fmt.Fprintln(w, intentArchivePurgeResumeInstruction(report.PurgeProgress.Resume))
		writeIntentArchiveRetry(w, report.PurgeProgress.Retry)
	}
	if report.RemainingRepairs != nil {
		fmt.Fprintf(w, "Rerun required: %t\n", report.RemainingRepairs.RerunRequired)
		if report.RemainingRepairs.RepairedClass != "" {
			fmt.Fprintf(w, "Repaired class: %s\n", report.RemainingRepairs.RepairedClass)
		}
		fmt.Fprintf(w, "Remaining repair stages: %d\n", report.RemainingRepairs.StagesRemaining)
		if report.RemainingRepairs.NextStage != nil {
			fmt.Fprintf(w, "Next repair stage: %d %s (%s)\n",
				report.RemainingRepairs.NextStage.Ordinal,
				report.RemainingRepairs.NextStage.Class,
				report.RemainingRepairs.NextStage.Kind,
			)
		}
		for _, stage := range report.RemainingRepairs.Stages {
			fmt.Fprintf(w, "  stage %d: %s (%s)\n", stage.Ordinal, stage.Class, stage.Kind)
			for _, hash := range stage.Hashes {
				fmt.Fprintf(w, "    hash: %s\n", hash)
			}
			for _, item := range stage.Paths {
				fmt.Fprintf(w, "    path: %s\n", item)
			}
			fmt.Fprintf(w, "    repair: %s\n", strings.ReplaceAll(stage.Repair, "\n", "\n    "))
			fmt.Fprintf(w, "    repair_cwd: %s\n", stage.RepairCWD)
			if len(stage.ResultingClasses) != 0 {
				fmt.Fprintf(w, "    resulting classes: %s\n", strings.Join(stage.ResultingClasses, ", "))
			}
			fmt.Fprintf(w, "    after prerequisite: %t\n", stage.AfterPrerequisite)
		}
	}
	if report.Divergence != nil {
		fmt.Fprintf(w, "Archive divergence: %s\n", report.Divergence.Warning)
		fmt.Fprintf(w, "  pending hash: %s\n", report.Divergence.PendingHash)
		if report.Divergence.Blob != "" {
			fmt.Fprintf(w, "  blob:  %s\n", report.Divergence.Blob)
		}
		fmt.Fprintf(w, "  index: %s\n", report.Divergence.Index)
		if report.Divergence.RemoveCommand != "" {
			fmt.Fprintf(w, "  %s\n", report.Divergence.RemoveCommand)
		}
		if report.Divergence.RestoreInstruction != "" {
			fmt.Fprintf(w, "  %s\n", report.Divergence.RestoreInstruction)
		}
		for _, hash := range report.Divergence.CompletedHashes {
			fmt.Fprintf(w, "  completed %s\n", hash)
		}
		for _, hash := range report.Divergence.RemainingHashes {
			fmt.Fprintf(w, "  remaining %s\n", hash)
		}
		fmt.Fprintln(w, report.Divergence.Cost)
		writeIntentArchiveRetry(w, report.Divergence.Retry)
	}
	for _, advisory := range report.Advisories {
		fmt.Fprintf(w, "Advisory: %s\n", advisory.Message)
	}
	if report.Refusal != nil {
		writeIntentArchiveRefusal(w, report.Refusal)
	}
	if report.Outcome == string(store.IntentArchivePurgePlanned) {
		writeIntentArchiveRetry(w, report.Retry)
	}
	fmt.Fprintln(w, report.HistoryDisclosure)
}

func intentArchivePurgeResumeInstruction(resume string) string {
	switch store.IntentArchivePurgeResume(resume) {
	case store.IntentArchiveResumePendingRecoveryThenCompletion:
		return "The first retry finalizes the pending hash and exits 0 recovered without processing the selector. Run the same command a second time to complete the remaining work."
	case store.IntentArchiveResumeCompletionOnly:
		return "Exactly one retry completes the remaining hashes. It does not produce or promise a recovered outcome."
	case store.IntentArchiveResumeOrphanScan:
		return "Exactly one retry rescans the archive and removes the remaining orphan blobs. It does not produce or promise a recovered outcome."
	default:
		return "The purge cannot state a retry procedure because its resume class is unknown."
	}
}

func writeIntentArchiveRefusal(w io.Writer, refusal *intentArchiveRefusalReport) {
	fmt.Fprintf(w, "Refusal: %s\n", refusal.Code)
	fmt.Fprintf(w, "  %s\n", refusal.Message)
	fmt.Fprintf(w, "  %s\n", refusal.Remediation)
	if refusal.Retry != "" {
		writeIntentArchiveRetry(w, refusal.Retry)
	}
}

func writeIntentArchiveRetry(w io.Writer, retry string) {
	if retry == "" {
		return
	}
	fmt.Fprintln(w, prepareRetryHeader)
	fmt.Fprintf(w, "  %s\n", retry)
}

func sortedUniqueIntentArchiveStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	out := sorted[:0]
	for _, value := range sorted {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}

func intentArchiveClassOrphanHashes(classes []store.IntentArchiveRepairClassReport) []string {
	for _, class := range classes {
		if class.Class == store.IntentArchiveRepairUnreferencedResidue {
			return append([]string(nil), class.Hashes...)
		}
	}
	return []string{}
}

func intentArchiveContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
