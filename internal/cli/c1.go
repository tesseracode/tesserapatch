package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// featureDirPath constructs the absolute path to a feature's directory
// under .tpatch/features/<slug>/. Kept local to the c1 commands (edit/
// amend/remove) rather than exported from store.
func featureDirPath(s *store.Store, slug string) string {
	return filepath.Join(s.TpatchDir(), "features", slug)
}

// resolveArtifactPath probes both the feature root and its artifacts/
// subdir and returns the first path that exists. Some files (request.md,
// spec.md, apply-recipe.json, analysis.md, exploration.md, record.md) live
// directly under .tpatch/features/<slug>/; others (post-apply.patch,
// apply-session.json, etc.) live under artifacts/. When neither exists,
// the top-level path is returned with exists=false so callers can surface
// a "does not exist" error that still points at the canonical location.
func resolveArtifactPath(s *store.Store, slug, artifact string) (path string, exists bool) {
	topLevel := filepath.Join(featureDirPath(s, slug), artifact)
	inArtifacts := filepath.Join(featureDirPath(s, slug), "artifacts", artifact)
	if _, err := os.Stat(topLevel); err == nil {
		return topLevel, true
	}
	if _, err := os.Stat(inArtifacts); err == nil {
		return inArtifacts, true
	}
	return topLevel, false
}

// defaultEditArtifact picks the most relevant artifact for a given
// feature state. Falls back to request.md for unknown / edge states.
func defaultEditArtifact(state store.FeatureState) string {
	switch state {
	case store.StateRequested:
		return "request.md"
	case store.StateAnalyzed, store.StateDefined:
		return "spec.md"
	case store.StateImplementing:
		return "apply-recipe.json"
	case store.StateApplied:
		return "post-apply.patch"
	default:
		return "request.md"
	}
}

// ─── edit ────────────────────────────────────────────────────────────────────

func editCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <slug> [artifact]",
		Short: "Open a feature artifact in $EDITOR (defaulted by state)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			if _, err := os.Stat(featureDirPath(s, slug)); err != nil {
				return fmt.Errorf("feature %s does not exist", slug)
			}

			artifact := ""
			if len(args) == 2 {
				artifact = args[1]
			} else {
				status, _ := s.LoadFeatureStatus(slug)
				artifact = defaultEditArtifact(status.State)
			}

			path, exists := resolveArtifactPath(s, slug, artifact)
			if !exists {
				return fmt.Errorf("artifact %q does not exist for feature %s", artifact, slug)
			}
			openInEditor(cmd.OutOrStdout(), path)
			return nil
		},
	}
	return cmd
}

// ─── amend ───────────────────────────────────────────────────────────────────

func amendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "amend <slug> [description...]",
		Short: "Replace a feature's request.md (use --append to add instead of replace)",
		Long: `Replace (default) or append to a feature's request.md.

Default behavior REPLACES the existing request.md with the new description.
Use --append to concatenate the new description onto the existing request
(separated by a blank line).

--append and --reset are mutually exclusive: a state reset alongside an
append makes no sense (you are reopening the intent while pretending to
preserve it).

Reads the description from positional args, or from stdin when none are
provided.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			if _, err := os.Stat(featureDirPath(s, slug)); err != nil {
				return fmt.Errorf("feature %s does not exist", slug)
			}

			appendMode, _ := cmd.Flags().GetBool("append")
			reset, _ := cmd.Flags().GetBool("reset")
			if appendMode && reset {
				return fmt.Errorf("amend: --append and --reset are mutually exclusive")
			}

			depAdds, _ := cmd.Flags().GetStringArray("depends-on")
			depRms, _ := cmd.Flags().GetStringArray("remove-depends-on")
			depsOnly := (len(depAdds) > 0 || len(depRms) > 0) && len(args) == 1 && !stdinIsPiped(cmd)

			var description string
			if !depsOnly {
				switch {
				case len(args) > 1:
					description = strings.Join(args[1:], " ")
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
					return fmt.Errorf("provide a new description as arguments or pipe via stdin")
				}
			}

			if !depsOnly {
				var newBody string
				if appendMode {
					// Blind concat with a blank-line separator. A future enhancement
					// could parse request.md section headers and append to a canonical
					// "Additional requirements" block; documented in help text so
					// users are not surprised by the current straightforward model.
					existing, _ := s.ReadFeatureFile(slug, "request.md")
					existing = strings.TrimRight(existing, "\n")
					if existing == "" {
						newBody = description + "\n"
					} else {
						newBody = existing + "\n\n" + description + "\n"
					}
				} else {
					newBody = description + "\n"
				}

				if err := s.WriteFeatureFile(slug, "request.md", newBody); err != nil {
					return err
				}

				if reset {
					if err := s.MarkFeatureState(slug, store.StateRequested, "amend --reset", "Request replaced; state reset"); err != nil {
						return err
					}
				}
			}

			// Apply any --depends-on / --remove-depends-on edits.
			if len(depAdds) > 0 || len(depRms) > 0 {
				if !dependencyConfigEnabled(s) {
					return fmt.Errorf("amend --depends-on requires features_dependencies: true in .tpatch/config.yaml")
				}
				if err := applyAmendDependsOn(cmd, s, slug); err != nil {
					return err
				}
			}

			status, _ := s.LoadFeatureStatus(slug)
			verb := "Amended"
			if appendMode {
				verb = "Appended to"
			}
			if depsOnly {
				verb = "Updated dependencies on"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s feature %s (state: %s)\n", verb, slug, status.State)
			return nil
		},
	}
	cmd.Flags().Bool("reset", false, "Reset feature state to \"requested\"")
	cmd.Flags().Bool("append", false, "Append to request.md instead of replacing it")
	cmd.Flags().StringArray("depends-on", nil, "Add or upgrade a depends_on edge (parent[:hard|:soft], repeatable). Requires features_dependencies.")
	cmd.Flags().StringArray("remove-depends-on", nil, "Remove a depends_on edge (parent slug, repeatable). Requires features_dependencies.")
	return cmd
}

// ─── remove ──────────────────────────────────────────────────────────────────

func removeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <slug>",
		Short: "Delete a feature directory and all its artifacts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}
			if _, err := os.Stat(featureDirPath(s, slug)); err != nil {
				return fmt.Errorf("feature %s does not exist", slug)
			}

			force, _ := cmd.Flags().GetBool("force")
			cascade, _ := cmd.Flags().GetBool("cascade")

			// PRD §3.7 / ADR-011 D7: --force never bypasses the dep
			// integrity gate. Only --cascade may opt into removing a
			// feature with downstream dependents.
			if !cascade {
				if err := checkRemoveDependents(s, slug); err != nil {
					return err
				}
			}

			if cascade {
				return runRemoveWithCascade(cmd, s, slug, force)
			}

			// Contract (v0.5.1 shipped):
			//   --force       → always skip confirmation
			//   TTY stdin     → prompt [y/N]
			//   piped stdin / redirected / no TTY → skip confirmation (auto-yes)
			// The last branch is what lets scripts like
			//   printf 'y\n' | tpatch remove <slug>
			// and unattended CI steps succeed without --force.
			if !force && canPromptForConfirmation(cmd) {
				fmt.Fprintf(cmd.OutOrStdout(), "Remove feature %s and all its artifacts? [y/N] ", slug)
				reader := bufio.NewReader(cmd.InOrStdin())
				line, _ := reader.ReadString('\n')
				line = strings.TrimSpace(line)
				if line != "y" && line != "Y" {
					fmt.Fprintln(cmd.OutOrStdout(), "aborted")
					return nil
				}
			}

			if err := s.RemoveFeature(slug); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed feature %s\n", slug)
			return nil
		},
	}
	cmd.Flags().Bool("force", false, "Skip confirmation prompt (does NOT bypass dependency check)")
	cmd.Flags().Bool("cascade", false, "Also remove every dependent of this feature (reverse-topo order)")
	return cmd
}

// canPromptForConfirmation reports whether it is safe to ask the user
// a y/N question on the command's input stream. Returns true when:
//   - stdin is a terminal (regular interactive run), OR
//   - stdin has been replaced by cobra's SetIn (tests / scripted input).
//
// Returns false when stdin is a redirected file or pipe from an
// unattended context — the classical case where we must refuse to
// destructively act without an explicit --force.
func canPromptForConfirmation(cmd *cobra.Command) bool {
	in := cmd.InOrStdin()
	f, ok := in.(*os.File)
	if !ok {
		return true // tests / scripts using SetIn
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
