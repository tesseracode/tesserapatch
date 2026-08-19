package cli

import (
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/intent"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func prepareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prepare <slug>",
		Short: "Inspect, complete, adopt, regenerate, or abandon an intent bundle",
		Long: `tpatch prepare has five modes:
  --check                 inspect the bundle without changing it
  (no mode flag)          generate the missing dependency-coherent suffix
  --manual                adopt a complete hand-authored bundle
  --regenerate            replace the complete bundle and archive prior bytes
  --abandon-transaction   preserve interrupted local transaction evidence

Only --regenerate overwrites an existing intent artifact. --allow-heuristic is
the only opt-in that permits regeneration to downgrade to heuristic output.
This command is unrelated to ` + "`tpatch apply --mode prepare`" + `.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options := readPrepareOptions(cmd)
			if options.yes && options.mode != prepareModeAbandon {
				return &ExitCodeError{
					Code:    1,
					Message: "prepare: --yes is only valid with --abandon-transaction",
				}
			}
			if options.mode == prepareModeCheck {
				return runPrepareCheck(cmd, args[0])
			}
			return runPreparePublish(cmd, args[0], options)
		},
	}
	cmd.Flags().Bool("check", false, "Inspect intent artifacts without changing state")
	cmd.Flags().Bool("manual", false, "Adopt a complete hand-authored intent bundle")
	cmd.Flags().Bool("regenerate", false, "Replace the complete intent bundle and archive prior bytes")
	cmd.Flags().Bool("abandon-transaction", false, "Preview or preserve interrupted local transaction evidence")
	cmd.Flags().Bool("allow-heuristic", false, "Permit --regenerate to replace the bundle with heuristic output when the provider is missing or fails. Without this flag, regeneration refuses rather than downgrading hand-authored content.")
	cmd.Flags().Bool("dry-run", false, "Report the mutation plan without executing it")
	cmd.Flags().Bool("yes", false, "Confirm --abandon-transaction")
	cmd.Flags().Bool("json", false, "Emit the structured report on stdout")
	cmd.Flags().Bool("quiet", false, "Suppress the human report")
	cmd.Flags().Duration("timeout", 180*time.Second, "Total generation deadline")
	cmd.Flags().Duration("timeout-phase", 90*time.Second, "Per-generator deadline")
	cmd.Flags().Bool("no-retry", false, "Disable validator-driven provider retry")

	cmd.MarkFlagsMutuallyExclusive("check", "manual", "regenerate", "abandon-transaction")
	for _, generationFlag := range []string{"timeout", "timeout-phase", "no-retry"} {
		for _, incompatibleMode := range []string{"check", "manual", "abandon-transaction"} {
			cmd.MarkFlagsMutuallyExclusive(generationFlag, incompatibleMode)
		}
	}
	for _, incompatibleMode := range []string{"check", "manual", "abandon-transaction"} {
		cmd.MarkFlagsMutuallyExclusive("allow-heuristic", incompatibleMode)
	}
	cmd.MarkFlagsMutuallyExclusive("dry-run", "check")
	cmd.MarkFlagsMutuallyExclusive("dry-run", "abandon-transaction")
	return cmd
}

func runPrepareCheck(cmd *cobra.Command, rawSlug string) error {
	// This is intentionally allocated before every report-bearing
	// check path, including aborts, and shared by all captures.
	scratch := make([]byte, intent.MaxArtifactBytes+1)
	slug, err := intent.CanonicalSlug(rawSlug)
	if err != nil {
		return emitPrepareReport(cmd, intent.NewAbortReport("", intent.AbortSlugUnsafe))
	}
	if !intent.RootConfinementSupported() {
		return emitPrepareReport(cmd, intent.NewAbortReport(slug, intent.AbortUnsupportedPlatform))
	}

	start, _ := cmd.Flags().GetString("path")
	if start == "" {
		start = "."
	}
	repoRoot, err := store.FindProjectRoot(start)
	if err != nil {
		return emitPrepareReport(cmd, intent.NewAbortReport(slug, intent.AbortWorkspaceMissing))
	}
	root, err := os.OpenRoot(repoRoot)
	if err != nil {
		return emitPrepareReport(cmd, intent.NewAbortReport(slug, intent.AbortRootUnopenable))
	}

	report := intent.Inspect(intent.NewRootOps(root), slug, scratch)
	renderErr := writePrepareReport(cmd, report)
	_ = root.Close()
	if renderErr != nil {
		return renderErr
	}
	return prepareExit(report)
}

func emitPrepareReport(cmd *cobra.Command, report intent.Report) error {
	if err := writePrepareReport(cmd, report); err != nil {
		return err
	}
	return prepareExit(report)
}

func writePrepareReport(cmd *cobra.Command, report intent.Report) error {
	asJSON, _ := cmd.Flags().GetBool("json")
	quiet, _ := cmd.Flags().GetBool("quiet")
	if asJSON {
		if err := report.WriteJSON(cmd.OutOrStdout()); err != nil {
			return err
		}
		if !quiet {
			report.WriteHuman(cmd.ErrOrStderr())
		}
		return nil
	}
	if quiet {
		report.WriteQuiet(cmd.OutOrStdout())
		return nil
	}
	report.WriteHuman(cmd.OutOrStdout())
	return nil
}

func prepareExit(report intent.Report) error {
	if report.Abort != nil {
		return &ExitCodeError{Code: 3, Message: report.ExitMessage()}
	}
	switch report.Readiness() {
	case intent.ReadinessReady:
		return nil
	case intent.ReadinessNotReady:
		return &ExitCodeError{Code: 2, Message: report.ExitMessage()}
	case intent.ReadinessIndeterminate:
		return &ExitCodeError{Code: 3, Message: report.ExitMessage()}
	default:
		panic("cli.prepareExit: readiness outside the closed domain: " + string(report.Readiness()))
	}
}
