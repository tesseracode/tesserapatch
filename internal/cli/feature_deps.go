// Dependency-management CLI verbs (M14.4 / Chunk C).
//
// Surfaces the M14.1 store-level DAG primitives + M14.2 apply gate to
// users without making them edit status.json by hand. Wires:
//
//   tpatch feature deps <slug>                       — print the dep block
//   tpatch feature deps <slug> add <parent>[:kind]   — add an edge
//   tpatch feature deps <slug> remove <parent>       — remove an edge
//   tpatch feature deps --validate-all               — global validation
//
// Plus flag additions on existing commands:
//
//   tpatch amend --depends-on <p>[:kind]             — same as `feature deps add`
//   tpatch amend --remove-depends-on <p>             — same as `feature deps remove`
//   tpatch remove --cascade                          — also delete dependents
//
// All edits route through store.ValidateDependencies so cycles, kind
// conflicts, dangling refs and self-edges are rejected uniformly.
//
// `--force` does NOT bypass DAG integrity (PRD §3.7, ADR-011 D7); it
// only suppresses the TTY confirmation prompt for cascade.

package cli

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// featureCmd is the parent for all per-feature management subcommands.
// Today only `deps` lives here — the namespace is reserved for future
// per-feature management surfaces (PRD §10) so we don't keep flat-listing
// new top-level commands.
func featureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feature",
		Short: "Per-feature management commands",
	}
	cmd.AddCommand(featureDepsCmd())
	return cmd
}

// featureDepsCmd dispatches the four `feature deps ...` shapes off the
// positional argument count + `--validate-all` flag. We use a single
// command rather than nested subcommands because the natural CLI verb is
// `feature deps <slug> add <parent>` (PRD §10), not `feature deps add
// <slug> <parent>` — the slug comes before the action.
func featureDepsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deps [slug] [add|remove] [parent[:kind]]",
		Short: "Show or edit a feature's depends_on block",
		Long: `Manage feature dependency edges.

  tpatch feature deps <slug>                       Print the depends_on block + dependents.
  tpatch feature deps <slug> add <parent>[:kind]   Add a parent edge (kind defaults to hard).
  tpatch feature deps <slug> remove <parent>       Remove a parent edge.
  tpatch feature deps --validate-all               Validate the whole DAG.

Edits go through the same validation as on-disk loads: cycles, kind
conflicts, dangling refs and self-edges are rejected (ADR-011 D5).`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			validateAll, _ := cmd.Flags().GetBool("validate-all")
			s, err := openStoreFromCmd(cmd)
			if err != nil {
				return err
			}

			if validateAll {
				return runFeatureDepsValidateAll(cmd, s)
			}

			if len(args) == 0 {
				return fmt.Errorf("usage: tpatch feature deps <slug> [add|remove <parent>[:kind]]")
			}
			slug := args[0]
			rest := args[1:]

			switch {
			case len(rest) == 0:
				return runFeatureDepsShow(cmd, s, slug)
			case len(rest) >= 1 && rest[0] == "add":
				if len(rest) != 2 {
					return fmt.Errorf("usage: tpatch feature deps <slug> add <parent>[:kind]")
				}
				return runFeatureDepsAdd(cmd, s, slug, rest[1])
			case len(rest) >= 1 && rest[0] == "remove":
				if len(rest) != 2 {
					return fmt.Errorf("usage: tpatch feature deps <slug> remove <parent>")
				}
				return runFeatureDepsRemove(cmd, s, slug, rest[1])
			default:
				return fmt.Errorf("unknown deps action %q", rest[0])
			}
		},
	}
	cmd.Flags().Bool("validate-all", false, "Validate every feature's depends_on block and report all violations")
	return cmd
}

// parseDepSpec splits "parent[:kind]" into (slug, kind). Defaults the
// kind to hard. Rejects unknown kinds at parse time so the error
// message identifies the user input rather than an opaque store error.
func parseDepSpec(spec string) (slug string, kind string, err error) {
	if i := strings.Index(spec, ":"); i >= 0 {
		slug = strings.TrimSpace(spec[:i])
		kind = strings.TrimSpace(spec[i+1:])
	} else {
		slug = strings.TrimSpace(spec)
		kind = store.DependencyKindHard
	}
	if slug == "" {
		return "", "", fmt.Errorf("dependency spec %q has empty parent slug", spec)
	}
	if kind != store.DependencyKindHard && kind != store.DependencyKindSoft {
		return "", "", fmt.Errorf("dependency spec %q has invalid kind %q (want hard|soft)", spec, kind)
	}
	return slug, kind, nil
}

// runFeatureDepsShow prints depends_on + dependents for a single slug.
func runFeatureDepsShow(cmd *cobra.Command, s *store.Store, slug string) error {
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return fmt.Errorf("feature %q: %w", slug, err)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Feature: %s [%s]\n", st.Slug, st.State)
	if len(st.DependsOn) == 0 {
		fmt.Fprintln(out, "  depends_on: (none)")
	} else {
		fmt.Fprintln(out, "  depends_on:")
		for _, d := range st.DependsOn {
			line := fmt.Sprintf("    - %s (%s)", d.Slug, d.Kind)
			if d.SatisfiedBy != "" {
				line += fmt.Sprintf(" satisfied_by=%s", d.SatisfiedBy)
			}
			fmt.Fprintln(out, line)
		}
	}
	deps := dependentEdges(s, slug)
	if len(deps) == 0 {
		fmt.Fprintln(out, "  dependents: (none)")
		return nil
	}
	fmt.Fprintln(out, "  dependents:")
	for _, d := range deps {
		fmt.Fprintf(out, "    - %s (%s)\n", d.slug, d.kind)
	}
	return nil
}

type dependentEdge struct {
	slug string
	kind string
}

// dependentEdges scans every feature for entries whose depends_on
// contains parent and returns the edges sorted by child slug. This is
// the live derivation — `dependents` is not persisted (per PRD §3.7).
func dependentEdges(s *store.Store, parent string) []dependentEdge {
	feats, err := s.ListFeatures()
	if err != nil {
		return nil
	}
	var out []dependentEdge
	for _, f := range feats {
		for _, d := range f.DependsOn {
			if d.Slug == parent {
				out = append(out, dependentEdge{slug: f.Slug, kind: d.Kind})
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].slug < out[j].slug })
	return out
}

// runFeatureDepsAdd validates and persists a new edge atomically.
// Existing edges to the same parent are replaced (so kind upgrades are
// expressible), preserving SatisfiedBy if already set.
func runFeatureDepsAdd(cmd *cobra.Command, s *store.Store, slug, spec string) error {
	parent, kind, err := parseDepSpec(spec)
	if err != nil {
		return err
	}
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return fmt.Errorf("feature %q: %w", slug, err)
	}
	updated := false
	for i, d := range st.DependsOn {
		if d.Slug == parent {
			st.DependsOn[i].Kind = kind
			updated = true
			break
		}
	}
	if !updated {
		st.DependsOn = append(st.DependsOn, store.Dependency{Slug: parent, Kind: kind})
	}
	if err := store.ValidateDependencies(s, slug, st.DependsOn); err != nil {
		return err
	}
	if err := s.SaveFeatureStatus(st); err != nil {
		return err
	}
	verb := "added"
	if updated {
		verb = "updated"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s dependency: %s -> %s (%s)\n", verb, slug, parent, kind)
	return nil
}

// runFeatureDepsRemove drops the edge to parent atomically. No-op (with
// a friendly message) when the edge is absent.
func runFeatureDepsRemove(cmd *cobra.Command, s *store.Store, slug, parent string) error {
	st, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return fmt.Errorf("feature %q: %w", slug, err)
	}
	out := make([]store.Dependency, 0, len(st.DependsOn))
	removed := false
	for _, d := range st.DependsOn {
		if d.Slug == parent {
			removed = true
			continue
		}
		out = append(out, d)
	}
	if !removed {
		return fmt.Errorf("feature %q has no dependency on %q", slug, parent)
	}
	st.DependsOn = out
	if err := store.ValidateDependencies(s, slug, st.DependsOn); err != nil {
		return err
	}
	if err := s.SaveFeatureStatus(st); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "removed dependency: %s -> %s\n", slug, parent)
	return nil
}

// runFeatureDepsValidateAll runs ValidateAllFeatures and prints every
// violation. Exits non-zero when any are found so CI can gate on it.
func runFeatureDepsValidateAll(cmd *cobra.Command, s *store.Store) error {
	errs := store.ValidateAllFeatures(s)
	out := cmd.OutOrStdout()
	if len(errs) == 0 {
		fmt.Fprintln(out, "DAG: ok (0 violations)")
		return nil
	}
	fmt.Fprintf(out, "DAG: %d violation(s)\n", len(errs))
	for _, e := range errs {
		fmt.Fprintf(out, "  ✗ %s\n", e)
	}
	return fmt.Errorf("DAG validation failed: %d violation(s)", len(errs))
}

// ─── amend --depends-on / --remove-depends-on ────────────────────────────────

// applyAmendDependsOn applies any --depends-on / --remove-depends-on
// flags from the amend command. Returns nil and emits no output when
// neither flag is set so the existing amend body proceeds unchanged.
func applyAmendDependsOn(cmd *cobra.Command, s *store.Store, slug string) error {
	addSpecs, _ := cmd.Flags().GetStringArray("depends-on")
	rmSpecs, _ := cmd.Flags().GetStringArray("remove-depends-on")
	if len(addSpecs) == 0 && len(rmSpecs) == 0 {
		return nil
	}
	for _, spec := range addSpecs {
		if err := runFeatureDepsAdd(cmd, s, slug, spec); err != nil {
			return err
		}
	}
	for _, parent := range rmSpecs {
		if err := runFeatureDepsRemove(cmd, s, slug, parent); err != nil {
			return err
		}
	}
	return nil
}

// ─── remove --cascade ────────────────────────────────────────────────────────

// ErrHasDependents is returned by removeWithCascade when a feature has
// downstream dependents and --cascade was not set. Wraps with a
// caller-friendly message listing the dependents (PRD §3.7).
var ErrHasDependents = fmt.Errorf("feature has dependents")

// ErrInteractiveRequired is returned when --cascade is used in a non-TTY
// context without --force. PRD §3.7 + ADR-011 D7: cascade is destructive
// and must require explicit user intent.
var ErrInteractiveRequired = fmt.Errorf("cascade requires interactive confirmation; pass --force to proceed unattended")

// runRemoveWithCascade executes the cascade-aware delete: collects the
// downstream subtree, asks for confirmation when at a TTY, and removes
// in reverse-topological order so leaves disappear before their parents.
func runRemoveWithCascade(cmd *cobra.Command, s *store.Store, slug string, force bool) error {
	subtree, err := collectSubtree(s, slug)
	if err != nil {
		return err
	}
	if len(subtree) == 1 {
		// No dependents — short-circuit to the existing single-feature path.
		return removeSingle(cmd, s, slug, force)
	}

	// Reverse-topological order: existing TopologicalOrder yields parents
	// first; reverse it so leaves go first.
	feats, _ := s.ListFeatures()
	graph := make(map[string][]store.Dependency, len(feats))
	for _, f := range feats {
		graph[f.Slug] = f.DependsOn
	}
	order, err := store.TopologicalOrder(graph)
	if err != nil {
		return err
	}

	// Filter to subtree and reverse.
	inSubtree := make(map[string]struct{}, len(subtree))
	for _, sl := range subtree {
		inSubtree[sl] = struct{}{}
	}
	plan := make([]string, 0, len(subtree))
	for i := len(order) - 1; i >= 0; i-- {
		if _, ok := inSubtree[order[i]]; ok {
			plan = append(plan, order[i])
		}
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Cascade-remove %s and %d dependent(s):\n", slug, len(plan)-1)
	for _, sl := range plan {
		fmt.Fprintf(out, "  - %s\n", sl)
	}

	if !force {
		if !canPromptForConfirmation(cmd) {
			return ErrInteractiveRequired
		}
		fmt.Fprintf(out, "Proceed? [y/N] ")
		reader := bufio.NewReader(cmd.InOrStdin())
		line, _ := reader.ReadString('\n')
		if t := strings.TrimSpace(line); t != "y" && t != "Y" {
			fmt.Fprintln(out, "aborted")
			return nil
		}
	}

	for _, sl := range plan {
		if err := s.RemoveFeature(sl); err != nil {
			return fmt.Errorf("remove %s: %w", sl, err)
		}
		fmt.Fprintf(out, "Removed feature %s\n", sl)
	}
	return nil
}

// collectSubtree returns slug and every transitive dependent (BFS via
// reverse adjacency). Cycle-safe: a visited set guarantees each node
// is enumerated once even if the DAG has been corrupted.
func collectSubtree(s *store.Store, root string) ([]string, error) {
	feats, err := s.ListFeatures()
	if err != nil {
		return nil, err
	}
	rev := make(map[string][]string, len(feats))
	for _, f := range feats {
		for _, d := range f.DependsOn {
			rev[d.Slug] = append(rev[d.Slug], f.Slug)
		}
	}
	seen := map[string]struct{}{root: {}}
	queue := []string{root}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, child := range rev[cur] {
			if _, ok := seen[child]; ok {
				continue
			}
			seen[child] = struct{}{}
			queue = append(queue, child)
		}
	}
	out := make([]string, 0, len(seen))
	for sl := range seen {
		out = append(out, sl)
	}
	sort.Strings(out)
	return out, nil
}

// removeSingle handles the no-dependents path. Mirrors the existing
// removeCmd behaviour (TTY → prompt unless --force; non-TTY → auto-yes).
func removeSingle(cmd *cobra.Command, s *store.Store, slug string, force bool) error {
	if !force && canPromptForConfirmation(cmd) {
		fmt.Fprintf(cmd.OutOrStdout(), "Remove feature %s and all its artifacts? [y/N] ", slug)
		reader := bufio.NewReader(cmd.InOrStdin())
		line, _ := reader.ReadString('\n')
		if t := strings.TrimSpace(line); t != "y" && t != "Y" {
			fmt.Fprintln(cmd.OutOrStdout(), "aborted")
			return nil
		}
	}
	if err := s.RemoveFeature(slug); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Removed feature %s\n", slug)
	return nil
}

// checkRemoveDependents enforces the no-cascade gate (PRD §3.7). When a
// feature has downstream dependents and --cascade was NOT set, refuse
// regardless of --force. `force` is a TTY-confirmation override; it
// does NOT bypass DAG integrity.
func checkRemoveDependents(s *store.Store, slug string) error {
	cfg, err := s.LoadConfig()
	if err != nil {
		return nil // can't load config — skip the check, preserve v0.5.x behaviour
	}
	if !cfg.DAGEnabled() {
		return nil
	}
	deps := dependentEdges(s, slug)
	if len(deps) == 0 {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s has %d dependent(s):", ErrHasDependents, slug, len(deps))
	for _, d := range deps {
		fmt.Fprintf(&b, "\n  - %s (%s)", d.slug, d.kind)
	}
	b.WriteString("\nUse `tpatch remove --cascade " + slug + "` to remove the whole subtree.")
	return fmt.Errorf("%s", b.String())
}

// dependencyConfigEnabled is a tiny helper used by amend to gate the
// --depends-on flags behind the feature flag. Without the flag the
// flags are still parsed (so help text is consistent) but applying
// them is a no-op error.
func dependencyConfigEnabled(s *store.Store) bool {
	cfg, err := s.LoadConfig()
	if err != nil {
		return false
	}
	return cfg.DAGEnabled()
}

// _ unused import guard for os when the file shrinks during edits.
var _ = os.Stdin
