// CLI surface for feature claim manifests (PRD-feature-file-claims v1).
//
//   tpatch feature claim add <slug> <path...>
//   tpatch feature claim list <slug> [--json]
//   tpatch feature claim remove <slug> <claim-id-or-path...>
//   tpatch feature claim clear <slug>
//
// All v1 writes use kind="path", mode="advisory", source="manual". The
// reserved schema values (glob/symbol/anchor/strict/agent/...) are not
// exposed as flags; they are reserved for future PRDs.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// featureClaimCmd is the `tpatch feature claim ...` subtree. Wired
// onto featureCmd() so the namespace stays grouped with `feature deps`.
func featureClaimCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim",
		Short: "Declare which paths a feature expects to touch (advisory)",
		Long: `Manage the per-feature claims manifest at .tpatch/features/<slug>/claims.json.

Claims are advisory scope metadata: they document which paths a feature
expects to touch. v1 does NOT gate record/land/reconcile/apply on claims;
they exist as input for future capture and warning verbs.`,
	}
	cmd.AddCommand(
		featureClaimAddCmd(),
		featureClaimListCmd(),
		featureClaimRemoveCmd(),
		featureClaimClearCmd(),
	)
	return cmd
}

func featureClaimAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <slug> <path...>",
		Short: "Add one or more path claims to a feature (advisory)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			paths := args[1:]
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			if !s.FeatureExists(slug) {
				return fmt.Errorf("no such feature: %s", slug)
			}
			manifest, err := store.LoadClaims(s, slug)
			if err != nil {
				return err
			}
			manifest.Feature = slug
			if manifest.Version == 0 {
				manifest.Version = store.ClaimsManifestVersion
			}

			out := cmd.OutOrStdout()
			changed := false
			for _, p := range paths {
				normalized, err := store.NormalizeClaimPath(s.Root, p)
				if err != nil {
					return err
				}
				claim, added := store.AddClaim(&manifest, normalized)
				if !added {
					fmt.Fprintf(out, "already claimed: %s  %s  %s\n", claim.ClaimID, claim.Mode, claim.Value)
					continue
				}
				changed = true
				fmt.Fprintf(out, "added claim %s  %s  %s  %s\n", claim.ClaimID, claim.Mode, claim.Kind, claim.Value)
			}
			if !changed {
				return nil
			}
			return store.SaveClaims(s, manifest)
		},
	}
}

func featureClaimListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <slug>",
		Short: "List a feature's path claims",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			if !s.FeatureExists(slug) {
				return fmt.Errorf("no such feature: %s", slug)
			}
			manifest, err := store.LoadClaims(s, slug)
			if err != nil {
				return err
			}
			manifest.Feature = slug

			out := cmd.OutOrStdout()
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				data, err := json.MarshalIndent(manifest, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(data))
				return nil
			}
			if len(manifest.Claims) == 0 {
				fmt.Fprintf(out, "Claims for %s: (none)\n", slug)
				return nil
			}
			fmt.Fprintf(out, "Claims for %s:\n", slug)
			for _, c := range manifest.Claims {
				fmt.Fprintf(out, "  %s  %s  %s  %s\n", c.ClaimID, c.Mode, c.Kind, c.Value)
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "Emit the full manifest as pretty-printed JSON")
	return cmd
}

func featureClaimRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <slug> <claim-id-or-path...>",
		Short: "Remove one or more claims from a feature",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			targets := args[1:]
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			if !s.FeatureExists(slug) {
				return fmt.Errorf("no such feature: %s", slug)
			}
			manifest, err := store.LoadClaims(s, slug)
			if err != nil {
				return err
			}
			manifest.Feature = slug

			out := cmd.OutOrStdout()
			changed := false
			for _, t := range targets {
				match, ok, err := store.MatchClaim(s.Root, &manifest, t)
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintf(out, "no such claim: %s\n", t)
					continue
				}
				if store.RemoveClaim(&manifest, match.ClaimID) {
					changed = true
					fmt.Fprintf(out, "removed claim %s  %s  %s  %s\n", match.ClaimID, match.Mode, match.Kind, match.Value)
				}
			}
			if !changed {
				return nil
			}
			return store.SaveClaims(s, manifest)
		},
	}
}

func featureClaimClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear <slug>",
		Short: "Remove all claims from a feature (file is kept with claims: [])",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			if !s.FeatureExists(slug) {
				return fmt.Errorf("no such feature: %s", slug)
			}
			manifest := store.ClaimsManifest{
				Version: store.ClaimsManifestVersion,
				Feature: slug,
				Claims:  []store.Claim{},
			}
			if err := store.SaveClaims(s, manifest); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "cleared all claims for %s\n", slug)
			return nil
		},
	}
}
