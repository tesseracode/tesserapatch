package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

func featurePatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patch",
		Short: "Refresh or fix up a feature's canonical patch",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("unknown subverb; v1 supports refresh|fixup")
		},
	}
	cmd.AddCommand(featurePatchRefreshCmd())
	cmd.AddCommand(featurePatchFixupCmd())
	return cmd
}

func featurePatchRefreshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh <slug>",
		Short: "Refresh a feature's current patch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reason, _ := cmd.Flags().GetString("reason")
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			return runFeaturePatchAmend(cmd, s, args[0], store.PatchGenerationIntentRefresh, strings.TrimSpace(reason), "")
		},
	}
	cmd.Flags().String("reason", "", "Optional amendment reason stored on patch-generations.json")
	return cmd
}

func featurePatchFixupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fixup <slug>",
		Short: "Append an explicit fixup generation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reason, _ := cmd.Flags().GetString("reason")
			target, _ := cmd.Flags().GetString("target")
			reason = strings.TrimSpace(reason)
			target = strings.TrimSpace(target)
			if reason == "" {
				return fmt.Errorf("feature patch fixup requires --reason")
			}
			if target == "" {
				return fmt.Errorf("feature patch fixup requires --target <generation_id>")
			}
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			return runFeaturePatchAmend(cmd, s, args[0], store.PatchGenerationIntentFixup, reason, target)
		},
	}
	cmd.Flags().String("reason", "", "Required fixup reason stored on patch-generations.json")
	cmd.Flags().String("target", "", "Generation ID this fixup amends")
	return cmd
}

func runFeaturePatchAmend(cmd *cobra.Command, s *store.Store, slug, intent, reason, target string) error {
	if _, err := s.LoadFeatureStatus(slug); err != nil {
		return err
	}
	manifest, err := store.LoadPatchGenerations(s, slug)
	if err != nil {
		return err
	}
	if intent == store.PatchGenerationIntentFixup && !manifestHasGenerationID(manifest, target) {
		return fmt.Errorf("feature patch fixup target %q does not exist in patch-generations.json", target)
	}

	patch, err := gitutil.CapturePatchScoped(s.Root, nil)
	if err != nil {
		return fmt.Errorf("cannot capture patch: %w", err)
	}
	if patch == "" {
		if intent == store.PatchGenerationIntentRefresh {
			fmt.Fprintln(cmd.ErrOrStderr(), "no patch byte change; refresh skipped")
			return nil
		}
		return fmt.Errorf("feature patch fixup captured 0 bytes — nothing to record")
	}

	patchSHA := store.SHA256HexString(patch)
	classification, err := store.ClassifyPatchGenerationKind(manifest.Generations, patchSHA, intent)
	if err != nil {
		return err
	}
	if !classification.Append {
		if intent == store.PatchGenerationIntentRefresh {
			fmt.Fprintln(cmd.ErrOrStderr(), "no patch byte change; refresh skipped")
		} else {
			fmt.Fprintln(cmd.ErrOrStderr(), "no patch byte change; fixup skipped")
		}
		return nil
	}

	if err := s.WriteArtifact(slug, "post-apply.patch", patch); err != nil {
		return fmt.Errorf("write post-apply.patch: %w", err)
	}
	patchLabel := strings.TrimPrefix(classification.Kind, "amend-")
	patchName, err := s.WritePatch(slug, patchLabel, patch)
	if err != nil {
		return fmt.Errorf("write numbered patch: %w", err)
	}
	status, _ := s.LoadFeatureStatus(slug)
	commit, _ := gitutil.HeadCommit(s.Root)
	if commit != "" && status.Apply.BaseCommit == "" {
		status.Apply.BaseCommit = commit
	}
	status.Apply.HasPatch = true
	if err := s.SaveFeatureStatus(status); err != nil {
		return err
	}
	if err := s.MarkFeatureState(slug, store.StateApplied, "feature patch "+strings.TrimPrefix(intent, "patch-"), "Patch amended"); err != nil {
		return err
	}

	if action, skippedPaths, driftReason, agErr := workflow.AutogenRecipeForRecord(s, slug, patch, true, false); agErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: recipe autogen failed: %v\n", agErr)
	} else {
		for _, sp := range skippedPaths {
			fmt.Fprintf(cmd.ErrOrStderr(), "  recipe autogen skipped: %s\n", sp)
		}
		if action == workflow.AutogenStale && driftReason != "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: apply-recipe.json drift: %s\n", driftReason)
		}
	}

	auditPatch := ""
	if patchName != "" {
		auditPatch = "patches/" + patchName
	}
	if _, err := workflow.AppendPatchGenerationForFeature(s, slug, workflow.PatchGenerationInput{
		Intent:            intent,
		Reason:            reason,
		FixupOfGeneration: target,
		Patch:             patch,
		AuditPatch:        auditPatch,
		BaseCommit:        status.Apply.BaseCommit,
		Upper:             store.GenerationUpper{Kind: "working-tree", Ref: "working-tree", Commit: ""},
		Capture:           store.GenerationCapture{Mode: "working-tree-all", Pathspecs: []string{}, ClaimIDs: []string{}},
	}); err != nil {
		return fmt.Errorf("record patch generation: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Amended patch for %s (%s, %d bytes, %d files)\n", slug, classification.Kind, len(patch), countPatchFiles(patch))
	return nil
}

func manifestHasGenerationID(m store.PatchGenerationsManifest, id string) bool {
	for _, g := range m.Generations {
		if g.GenerationID == id {
			return true
		}
	}
	return false
}
