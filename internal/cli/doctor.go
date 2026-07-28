package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/workflow"
)

func doctorCmd() *cobra.Command {
	var dryRun bool
	var fix bool
	var jsonOut bool
	var checks []string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose tpatch workspace metadata drift",
		Long: "Diagnose tpatch workspace metadata drift.\n\n" +
			"Checks feature metadata (D1), patch-generations manifests (D2), installed skill assets at the six init-managed paths (D3), recipe schemas (D7), and workspace invariants (D8).\n" +
			"Hand-copied skill assets outside the init-managed paths are intentionally out of scope for this doctor wave.\n" +
			"Supported local flags are --dry-run, --fix, --json, and repeated --check; root persistent flags such as --path are inherited.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun && fix {
				return fmt.Errorf("doctor: --dry-run and --fix cannot be used together")
			}
			if err := workflow.ValidateDoctorCheckIDs(checks); err != nil {
				return err
			}
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			report, err := workflow.RunDoctor(s, workflow.DoctorOptions{
				DryRun: dryRun,
				Fix:    fix,
				Checks: checks,
			})
			if err != nil {
				return err
			}
			if jsonOut {
				if err := workflow.WriteDoctorJSON(cmd.OutOrStdout(), report); err != nil {
					return err
				}
			} else {
				workflow.WriteDoctorHuman(cmd.OutOrStdout(), report)
			}
			if code := workflow.DoctorExitCode(report); code != 0 {
				return &ExitCodeError{Code: code, Message: fmt.Sprintf("doctor found %d drift findings, %d warnings, %d errors", report.Summary.Findings, report.Summary.Warnings, report.Summary.Errors)}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview findings without writing changes (default)")
	cmd.Flags().BoolVar(&fix, "fix", false, "Apply safe doctor fixes with backups before overwrites")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit deterministic schema-versioned JSON report")
	cmd.Flags().StringArrayVar(&checks, "check", nil, "Limit execution to a doctor check ID (repeatable: D1, D2, D3, D7, D8)")
	return cmd
}
