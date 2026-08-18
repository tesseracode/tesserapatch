package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/intent"
	"github.com/tesseracode/tesserapatch/internal/store"
)

const prepareReservedMessage = "tpatch prepare requires --check in this release; the mutating intent-bundle form is not implemented. Run `tpatch prepare <slug> --check`, or `tpatch prepare --help` for the full grammar."

func prepareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prepare <slug> --check",
		Short: "Inspect an intent bundle without changing it",
		Long: `tpatch prepare <slug> --check reports the structural state of the
three required intent Markdown artifacts and the optional analysis sidecar.
It is read-only and never advances state. This command is unrelated to
` + "`tpatch apply --mode prepare`" + `.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			check, _ := cmd.Flags().GetBool("check")
			if !check {
				return &ExitCodeError{Code: 4, Message: prepareReservedMessage}
			}

			// This is intentionally allocated before every report-bearing
			// check path, including aborts, and shared by all captures.
			scratch := make([]byte, intent.MaxArtifactBytes+1)
			slug, err := intent.CanonicalSlug(args[0])
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
		},
	}
	cmd.Flags().Bool("check", false, "Inspect intent artifacts without changing state")
	cmd.Flags().Bool("json", false, "Emit the structured report on stdout")
	cmd.Flags().Bool("quiet", false, "Suppress the human report")
	return cmd
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
