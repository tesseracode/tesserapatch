package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

var operationCandidateTerminalFlagNames = []string{
	"candidate-diff",
	"accept-candidate",
	"reject-candidate",
	"clear-candidate-rejections",
	"resume-candidate-recovery",
}

type operationCandidateTerminalSelection struct {
	Name string
	Slug string
}

func selectedOperationCandidateTerminal(cmd *cobra.Command) (operationCandidateTerminalSelection, bool) {
	for _, name := range operationCandidateTerminalFlagNames {
		if !cmd.Flags().Changed(name) {
			continue
		}
		slug, _ := cmd.Flags().GetString(name)
		return operationCandidateTerminalSelection{Name: name, Slug: slug}, true
	}
	return operationCandidateTerminalSelection{}, false
}

func operationCandidateFlagParseError(_ *cobra.Command, err error) error {
	message := err.Error()
	for _, name := range operationCandidateTerminalFlagNames {
		if strings.Contains(message, "flag needs an argument: --"+name) {
			return validationError("reconcile: --%s takes exactly one slug", name)
		}
	}
	return err
}

func validateOperationCandidateTerminalFlags(cmd *cobra.Command, args []string) error {
	selected := make([]operationCandidateTerminalSelection, 0, len(operationCandidateTerminalFlagNames))
	newFlags := make(map[string]bool, len(operationCandidateTerminalFlagNames))
	for _, name := range operationCandidateTerminalFlagNames {
		newFlags[name] = true
		if cmd.Flags().Changed(name) {
			slug, _ := cmd.Flags().GetString(name)
			selected = append(selected, operationCandidateTerminalSelection{Name: name, Slug: slug})
		}
	}
	if len(selected) == 0 {
		return nil
	}
	if len(selected) > 1 {
		names := make([]string, 0, len(selected))
		for _, terminal := range selected {
			names = append(names, "--"+terminal.Name)
		}
		sort.Strings(names)
		return validationError("reconcile: %s are mutually exclusive", strings.Join(names, ", "))
	}

	changedBehavioral := operationCandidateBehavioralFlags(cmd, newFlags, true)
	if len(changedBehavioral) > 0 {
		return validationError("reconcile: --%s cannot be combined with --%s", selected[0].Name, changedBehavioral[0])
	}
	if strings.TrimSpace(selected[0].Slug) == "" || len(args) != 0 {
		return validationError("reconcile: --%s takes exactly one slug", selected[0].Name)
	}
	return nil
}

func operationCandidateBehavioralFlags(cmd *cobra.Command, newFlags map[string]bool, changedOnly bool) []string {
	var names []string
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Name == "path" || flag.Name == "format" || flag.Name == "help" || newFlags[flag.Name] {
			return
		}
		if !changedOnly || flag.Changed {
			names = append(names, flag.Name)
		}
	})
	sort.Strings(names)
	return names
}

func operationCandidateExitError(err error) error {
	if err == nil {
		return nil
	}
	code, ok := workflow.OperationCandidateActionCode(err)
	if !ok {
		return &ExitCodeError{Code: 1, Message: err.Error()}
	}
	exitCode := 1
	switch code {
	case "candidate-absent", "head-diverged", "candidate-cas-failed",
		"candidate-path-occupied", "candidate-binding-changed", "candidate-test-failed":
		exitCode = 2
	case "candidate-pending", "shadow-pending", "candidate-not-pending",
		"candidate-already-accepted", "candidate-feature-state-changed",
		"candidate-lock-held", "lock-unsupported", "lock-filesystem-unsupported",
		"recovery-required", "coverage-publication-failed",
		"acceptance-rolled-back":
		exitCode = 3
	case "acceptance-rollback-failed":
		exitCode = 1
	default:
		return &ExitCodeError{Code: 1, Message: err.Error()}
	}
	return &ExitCodeError{Code: exitCode, Message: err.Error()}
}

func runOperationCandidateTerminal(cmd *cobra.Command, s *store.Store, selection operationCandidateTerminalSelection, format string) error {
	slug := selection.Slug
	switch selection.Name {
	case "candidate-diff":
		patch, err := workflow.DiffOperationCandidate(s, slug)
		if err != nil {
			return operationCandidateExitError(err)
		}
		if format == "json" {
			return writeOperationCandidateJSON(cmd, map[string]any{"slug": slug, "diff": string(patch)})
		}
		_, err = cmd.OutOrStdout().Write(patch)
		return err
	case "reject-candidate":
		id, err := workflow.RejectOperationCandidate(s, slug)
		if err != nil {
			return operationCandidateExitError(err)
		}
		if format == "json" {
			return writeOperationCandidateJSON(cmd, map[string]any{"slug": slug, "candidate_id": id, "status": "rejected"})
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Rejected operation candidate %s for %s\n", id, slug)
		return operationCandidateExitError(err)
	case "clear-candidate-rejections":
		if err := workflow.ClearOperationCandidateRejections(s, slug); err != nil {
			return operationCandidateExitError(err)
		}
		if format == "json" {
			return writeOperationCandidateJSON(cmd, map[string]any{"slug": slug, "candidate_rejections": "cleared"})
		}
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "Cleared operation candidate rejections for %s\n", slug)
		return operationCandidateExitError(err)
	case "accept-candidate":
		if err := workflow.AcceptOperationCandidate(s, slug); err != nil {
			return operationCandidateExitError(err)
		}
		if format == "json" {
			return writeOperationCandidateJSON(cmd, map[string]any{"slug": slug, "status": "accepted"})
		}
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "Accepted operation candidate for %s\n", slug)
		return operationCandidateExitError(err)
	case "resume-candidate-recovery":
		if err := workflow.ResumeOperationCandidateRecovery(s, slug); err != nil {
			return operationCandidateExitError(err)
		}
		if format == "json" {
			return writeOperationCandidateJSON(cmd, map[string]any{"slug": slug, "candidate_recovery": "complete"})
		}
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "Completed operation candidate recovery for %s\n", slug)
		return operationCandidateExitError(err)
	default:
		return fmt.Errorf("unknown operation candidate terminal --%s", selection.Name)
	}
}

func writeOperationCandidateJSON(cmd *cobra.Command, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", data)
	return operationCandidateExitError(err)
}
