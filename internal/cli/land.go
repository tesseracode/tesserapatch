// Package cli — `tpatch land <slug>` (M17 Wave C).
//
// `land` is the user-visible bridge between tpatch state and Git
// history: it composes (record → safe staging → one Git commit) into
// one verb that produces an ordinary Git commit carrying the locked
// four-trailer block (Tpatch-Feature, Tpatch-Patch-SHA,
// Tpatch-Recipe-SHA, Tpatch-Base-Commit).
//
// PRD: docs/prds/PRD-tpatch-land.md
// ADR: docs/adrs/ADR-019-tpatch-land-trailer-block-schema.md
package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

// coAuthorTrailer is the repo-policy trailer required by CLAUDE.md
// working rule 8 / PRD §3.4 "Repo-level trailers". `land` appends it
// after the four locked Tpatch-* trailers.
const coAuthorTrailer = "Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"

// landCmd defines the `tpatch land <slug>` cobra command.
//
// Scope is bounded per PRD §1: land does NOT run analyze, define,
// explore, implement, apply, test, push, rebase, or merge. It runs
// (optionally) `record`, then safe-staging, then one `git commit`.
func landCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "land <slug>",
		Short: "Project a feature into Git history with a Tpatch-Feature trailer block (one commit)",
		Long: `Land a feature: (re-)record its post-apply.patch, safely stage the
feature's files plus its .tpatch/features/<slug>/ metadata, and
produce one ordinary Git commit carrying the four-trailer block:

  Tpatch-Feature: <slug>
  Tpatch-Patch-SHA: <sha256 of post-apply.patch>
  Tpatch-Recipe-SHA: <sha256 of apply-recipe.json | none>
  Tpatch-Base-Commit: <status.apply.base_commit>

land does NOT push, rebase, merge, or amend. It does not run any
phase of the lifecycle other than the embedded record step.

See docs/prds/PRD-tpatch-land.md for the full contract.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLand(cmd, args[0])
		},
	}
	cmd.Flags().String("message", "", "Commit subject (overrides spec.md/request.md derivation)")
	cmd.Flags().Bool("allow-extra-paths", false, "Permit staging paths outside the feature's recorded patch scope (one-line warning per file)")
	cmd.Flags().Bool("no-record", false, "Skip the embedded record step; trust the existing post-apply.patch (refuses if no patch is recorded)")
	cmd.Flags().String("from", "", "Forwarded to the embedded record step (committed-range capture base)")
	cmd.Flags().Bool("auto", false, "Forwarded to the embedded record step (--auto baseline inference); mutually exclusive with --from")
	cmd.Flags().String("files", "", "Forwarded to the embedded record step (comma-separated pathspecs)")
	cmd.Flags().String("allow-collision", "", "Forwarded to the embedded record step (override cross-feature canonical patch collision)")
	cmd.Flags().Bool("dry-run", false, "Print the would-be commit subject, trailer block, and staging plan; perform no mutations")
	return cmd
}

// runLand is the main land orchestration. PRD §3 contract.
func runLand(cmd *cobra.Command, slug string) error {
	s, err := openStoreFromCmd(cmd)
	if err != nil {
		return err
	}

	// Refusal #1 (§3.2): the feature must exist. We surface the
	// status-load failure verbatim so the diagnostic matches what
	// `record` / `apply` produce on the same precondition.
	if _, err := s.LoadFeatureStatus(slug); err != nil {
		return fmt.Errorf("feature %q not found: %w", slug, err)
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		return runLandDryRun(cmd, s, slug)
	}

	noRecord, _ := cmd.Flags().GetBool("no-record")
	allowExtra, _ := cmd.Flags().GetBool("allow-extra-paths")
	message, _ := cmd.Flags().GetString("message")
	fromRef, _ := cmd.Flags().GetString("from")
	autoBase, _ := cmd.Flags().GetBool("auto")
	filesFlag, _ := cmd.Flags().GetString("files")
	allowCollision, _ := cmd.Flags().GetString("allow-collision")

	// Refusals 2/3/4 (§3.2): conflict markers, merge leftovers,
	// mid-merge state. These are deliberately narrower than
	// PreflightReconcile — dirty/untracked working-tree files are
	// expected and welcome (PRD §3.2 "Not a refusal").
	if err := landPreflight(s.Root); err != nil {
		return err
	}

	// Refusal #7 (§3.2): hard-parent dependency gate. Run before
	// the embedded record so a refusal leaves the tree untouched.
	if err := workflow.CheckDependencyGate(s, slug); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "%v\n", err)
		return err
	}

	// PRD §3.3 step 3: snapshot the global metadata files BEFORE
	// the embedded record step so we can decide later whether
	// `record` (e.g. --auto refreshing upstream.lock) actually
	// touched them. Operator-driven dirty drift on these globals
	// must NOT be silently absorbed into the feature commit.
	metaBefore := snapshotMetadataFiles(s.Root)

	// Refusals 5+6 (§3.2): "record would refuse" + cross-feature
	// collision. Both are produced verbatim by the embedded
	// `record` step; we surface them as-is per PRD §3.2 #5
	// ("No re-wrapping").
	if !noRecord {
		if err := embedRecord(cmd, s.Root, slug, fromRef, autoBase, filesFlag, allowCollision); err != nil {
			return err
		}
	} else {
		// --no-record: must already have a recorded canonical patch.
		status, _ := s.LoadFeatureStatus(slug)
		if !status.Apply.HasPatch {
			return fmt.Errorf("land --no-record refuses: feature %q has no recorded post-apply.patch (run `tpatch land %s` without --no-record, or `tpatch record %s` first)", slug, slug, slug)
		}
	}

	// Compute the global-metadata-changed set: only globals whose
	// content (or existence) changed across the embedded record
	// step are eligible for inclusion (PRD §3.3 step 3). If a
	// global is dirty for unrelated reasons, it is left alone.
	metaAfter := snapshotMetadataFiles(s.Root)
	metaChanged := metadataChangedSet(metaBefore, metaAfter)

	// Read the canonical patch produced (or already on disk for
	// --no-record) so the path-set builder can enumerate
	// user-code paths the feature owns.
	patch, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
	if err != nil || strings.TrimSpace(patch) == "" {
		return fmt.Errorf("land: cannot read post-apply.patch for %q: %v", slug, err)
	}

	// Update status.json:notes BEFORE staging so the freshly-dirty
	// status.json is picked up by the slug-prefix branch in
	// computePathSet (PRD §3.6: working tree must be clean
	// post-land, including on the --no-record path). The new HEAD
	// SHA is intentionally NOT written here (PRD F2 / §6 ac.5).
	now := time.Now().UTC().Format(time.RFC3339)
	status, _ := s.LoadFeatureStatus(slug)
	status.Notes = strings.TrimSpace(fmt.Sprintf("landed at %s", now))
	if err := s.SaveFeatureStatus(status); err != nil {
		return fmt.Errorf("land: cannot update status.json notes: %w", err)
	}

	// Compute path set (§3.3) — ordered AFTER the status.notes
	// save so the slug-prefix branch picks up the dirty
	// status.json on every path (with-record and --no-record).
	pathSet := computePathSet(s, slug, patch, metaChanged)

	// Identify dirty paths in the working tree NOT in the path set.
	// This is the WP-001 §5.2 row 5 boundary check moved one step
	// downstream: if the operator has unrelated edits in the tree,
	// we refuse rather than absorb them into the feature commit.
	dirty, err := dirtyPaths(s.Root)
	if err != nil {
		return fmt.Errorf("land: cannot read git status: %w", err)
	}
	// PRD §3.3 step 3: operator-driven dirty drift on the global
	// metadata files is neither swept into the commit (already
	// excluded above) nor counted as an "extras" refusal. Surface
	// a one-line note and leave them dirty in the working tree.
	filteredDirty := make([]string, 0, len(dirty))
	for _, p := range dirty {
		if (p == ".tpatch/upstream.lock" || p == ".tpatch/FEATURES.md") && !metaChanged[p] {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"note: leaving %s dirty (operator drift outside feature scope; not staged)\n", p)
			continue
		}
		filteredDirty = append(filteredDirty, p)
	}
	extras := classifyExtras(filteredDirty, pathSet)
	if len(extras) > 0 {
		if !allowExtra {
			return formatExtrasRefusal(slug, extras)
		}
		// --allow-extra-paths: stage the extras and warn one line
		// per file (PRD §3.3 step 4a). The extras are folded into
		// the path set so `git add` below picks them up.
		for _, p := range extras {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"note: staging extra path %s (not in feature patch); the feature commit will include this\n", p)
			pathSet = append(pathSet, p)
		}
	}

	// Stage the path set. `git add` accepts directories; for
	// untracked files we use --intent-to-add first then a normal
	// add, mirroring CapturePatchScoped's behaviour
	// (internal/gitutil/gitutil.go:228-251).
	if err := stagePathSet(s.Root, pathSet); err != nil {
		return fmt.Errorf("land: cannot stage path set: %w", err)
	}

	// Build commit subject + trailer block (§3.4).
	subject := deriveSubject(s, slug, message)
	patchSHA := sha256Hex([]byte(patch))
	recipeSHA := readRecipeSHA(s, slug)
	baseCommit := status.Apply.BaseCommit

	// PRD §3.4 trailer order (locked).
	trailerBlock := fmt.Sprintf(
		"Tpatch-Feature: %s\nTpatch-Patch-SHA: %s\nTpatch-Recipe-SHA: %s\nTpatch-Base-Commit: %s\n%s\n",
		slug, patchSHA, recipeSHA, baseCommit, coAuthorTrailer,
	)

	// `git commit -m subject -m trailerBlock` keeps the trailer
	// block separated from the subject by one blank line per
	// `--message` semantics. We disable gpgsign to avoid surprising
	// the operator in CI / sandboxed environments.
	commitArgs := []string{
		"-c", "commit.gpgsign=false",
		"commit",
		"-m", subject,
		"-m", trailerBlock,
	}
	commitOut, commitErr := runGitCapture(s.Root, commitArgs...)
	if commitErr != nil {
		// PRD §3.7 + §7.6: a pre-commit hook (or any commit
		// failure) leaves the staged index intact for recovery.
		// Surface the hook output verbatim and tell the user that
		// `--no-record` retries against the existing index.
		fmt.Fprintf(cmd.ErrOrStderr(), "%s", commitOut)
		return fmt.Errorf(
			"land: git commit failed (%v). The index is staged but uncommitted; "+
				"after fixing the reported error, retry with `tpatch land %s --no-record` "+
				"to commit the existing index without re-recording", commitErr, slug)
	}

	// Post-conditions (§3.6).
	newHead, _ := gitutil.HeadCommit(s.Root)

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Landed %s as %s\n", slug, abbrevSHA(newHead))
	fmt.Fprintf(out, "  trailer: Tpatch-Feature: %s\n", slug)
	fmt.Fprintf(out, "  feature ↔ commit binding: git log --grep '^Tpatch-Feature: %s$'\n", slug)
	return nil
}

// landPreflight implements PRD §3.2 refusals 2/3/4 — the narrow
// preflight that does NOT refuse on dirty/untracked files. Unstaged
// modifications and untracked files are precisely the substrate
// `record` is built to capture (PRD §3.2 "Not a refusal").
func landPreflight(repoRoot string) error {
	// Conflict markers in tracked files.
	if mark, _ := runGit(repoRoot, "grep", "-lE", "^<<<<<<< |^=======$|^>>>>>>> "); strings.TrimSpace(mark) != "" {
		files := strings.Split(strings.TrimSpace(mark), "\n")
		return fmt.Errorf("land refuses: conflict markers in tracked file(s): %s", strings.Join(files, ", "))
	}
	// *.orig / *.rej leftovers anywhere outside .git/.
	var leftovers []string
	_ = filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if strings.HasSuffix(name, ".orig") || strings.HasSuffix(name, ".rej") {
			rel, _ := filepath.Rel(repoRoot, path)
			leftovers = append(leftovers, rel)
		}
		return nil
	})
	if len(leftovers) > 0 {
		sort.Strings(leftovers)
		return fmt.Errorf("land refuses: merge leftover(s) present (resolve and remove them before landing): %s",
			strings.Join(leftovers, ", "))
	}
	// Mid-merge / mid-rebase / mid-cherry-pick. PRD §3.2 #4: detect
	// by file presence; no new gitutil helper.
	gitDir := filepath.Join(repoRoot, ".git")
	for _, marker := range []string{"MERGE_HEAD", "REBASE_HEAD", "CHERRY_PICK_HEAD"} {
		if _, err := os.Stat(filepath.Join(gitDir, marker)); err == nil {
			return fmt.Errorf("land refuses: repository is mid-%s (.git/%s exists). Resolve or abort the in-flight Git operation before landing.",
				strings.ToLower(strings.TrimSuffix(marker, "_HEAD")), marker)
		}
	}
	return nil
}

// embedRecord runs the existing `record` command via a fresh root.
// Surfacing record's diagnostics verbatim is a PRD §3.2 #5 contract
// ("No re-wrapping"); collision refusals (PRD §3.2 #6) are also
// produced by record and pass through here.
func embedRecord(cmd *cobra.Command, repoRoot, slug, fromRef string, autoBase bool, filesFlag, allowCollision string) error {
	args := []string{"record", "--path", repoRoot, slug}
	if autoBase {
		args = append(args, "--auto")
	}
	if fromRef != "" {
		args = append(args, "--from", fromRef)
	}
	if filesFlag != "" {
		args = append(args, "--files", filesFlag)
	}
	if allowCollision != "" {
		args = append(args, "--allow-collision", allowCollision)
	}
	root := buildRootCmd()
	root.SetArgs(args)
	root.SetOut(cmd.OutOrStdout())
	root.SetErr(cmd.ErrOrStderr())
	return root.Execute()
}

// computePathSet implements PRD §3.3 steps 1–3.
//
// Path set =
//   - FilesInPatch(post-apply.patch)
//   - everything dirty under .tpatch/features/<slug>/ (record's output)
//   - .tpatch/upstream.lock and .tpatch/FEATURES.md ONLY when the
//     embedded record step actually modified them (PRD §3.3 step 3).
//     `metaChanged` is the changed-set computed by snapshot/diff
//     across the embedded record step. A nil map means "no record
//     step ran" (e.g. dry-run): in that case the global metadata
//     files are NEVER swept in, since there is no record-driven
//     justification for staging them.
//
// We deliberately compute the metadata portion from `git status` so
// metadata-files-not-touched stay out of the index. Returned paths are
// repo-relative and unique.
func computePathSet(s *store.Store, slug, patch string, metaChanged map[string]bool) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(p string) {
		p = filepath.ToSlash(p)
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	for _, f := range gitutil.FilesInPatch(patch) {
		add(f)
	}
	dirty, _ := dirtyPaths(s.Root)
	featurePrefix := filepath.ToSlash(filepath.Join(".tpatch", "features", slug)) + "/"
	for _, p := range dirty {
		switch {
		case strings.HasPrefix(p, featurePrefix):
			add(p)
		case p == ".tpatch/upstream.lock":
			if metaChanged[p] {
				add(p)
			}
		case p == ".tpatch/FEATURES.md":
			if metaChanged[p] {
				add(p)
			}
		}
	}
	return out
}

// snapshotMetadataFiles returns a map from the global metadata file
// paths (relative to repoRoot) to a content fingerprint. A missing
// file is recorded as the sentinel "" so the diff against a later
// snapshot correctly classifies "missing → present" (or vice versa)
// as a change.
//
// Used by `land` to decide whether the embedded record step touched
// the global metadata (PRD §3.3 step 3): operator-driven dirty drift
// on these files MUST NOT be silently absorbed into the feature
// commit.
func snapshotMetadataFiles(repoRoot string) map[string]string {
	paths := []string{
		".tpatch/upstream.lock",
		".tpatch/FEATURES.md",
	}
	out := make(map[string]string, len(paths))
	for _, rel := range paths {
		abs := filepath.Join(repoRoot, filepath.FromSlash(rel))
		data, err := os.ReadFile(abs)
		if err != nil {
			out[rel] = ""
			continue
		}
		out[rel] = sha256Hex(data)
	}
	return out
}

// metadataChangedSet returns the set of metadata paths whose content
// (or existence) differs between the before and after snapshots.
// Missing-before / missing-after with no transition counts as no
// change; an existence transition in either direction counts as a
// change.
func metadataChangedSet(before, after map[string]string) map[string]bool {
	changed := make(map[string]bool, len(after))
	for k, v := range after {
		if before[k] != v {
			changed[k] = true
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			changed[k] = true
		}
	}
	return changed
}

// dirtyPaths returns repo-relative paths that `git status --porcelain`
// reports as modified, added, deleted, renamed, or untracked. Output
// is in stable (sorted) order.
func dirtyPaths(repoRoot string) ([]string, error) {
	out, err := runGit(repoRoot, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		// Porcelain v1: columns 1–2 are XY status; column 3 is a
		// space; column 4+ is the path. Renames use ` -> ` to
		// separate old → new — we take the new path because
		// that's what we'll be staging.
		path := strings.TrimSpace(line[3:])
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = strings.TrimSpace(path[idx+4:])
		}
		path = strings.Trim(path, "\"")
		path = filepath.ToSlash(path)
		if path != "" {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// classifyExtras returns the set of dirty paths NOT covered by the
// path set. Coverage is by literal path equality OR by directory
// prefix containment (the path set may include `.tpatch/features/<slug>/`
// as a directory entry implicitly through individual files; we rely
// on add-via-status to enumerate them).
func classifyExtras(dirty, pathSet []string) []string {
	covered := make(map[string]struct{}, len(pathSet))
	for _, p := range pathSet {
		covered[filepath.ToSlash(p)] = struct{}{}
	}
	var extras []string
	for _, p := range dirty {
		if _, ok := covered[p]; ok {
			continue
		}
		extras = append(extras, p)
	}
	sort.Strings(extras)
	return extras
}

// formatExtrasRefusal produces the PRD §3.3 step 4b refusal naming
// each extra path and pointing the operator at the three escape
// hatches.
func formatExtrasRefusal(slug string, extras []string) error {
	var b strings.Builder
	b.WriteString("land refuses: working tree contains edits outside the feature's path set:\n")
	for _, p := range extras {
		fmt.Fprintf(&b, "  - %s\n", p)
	}
	b.WriteString("\nResolve with one of:\n")
	b.WriteString("  - revert or remove the extra paths\n")
	b.WriteString("  - `git stash` the unrelated edits, run land, then `git stash pop`\n")
	fmt.Fprintf(&b, "  - rerun: tpatch land %s --allow-extra-paths\n", slug)
	return fmt.Errorf("%s", b.String())
}

// stagePathSet runs `git add --intent-to-add` followed by `git add`
// for each path in the set. --intent-to-add is a no-op for tracked
// files but turns untracked files into tracked-with-empty-blob, which
// is what `git add` then needs to capture their content. This mirrors
// CapturePatchScoped's behaviour at internal/gitutil/gitutil.go:228-251.
func stagePathSet(repoRoot string, pathSet []string) error {
	if len(pathSet) == 0 {
		return nil
	}
	// First pass: --intent-to-add makes untracked files visible to
	// `git add` without staging content. Errors here are tolerated:
	// the path may already be tracked or deleted.
	intentArgs := append([]string{"add", "--intent-to-add", "--"}, pathSet...)
	_, _ = runGit(repoRoot, intentArgs...)
	// Second pass: actually stage the content.
	addArgs := append([]string{"add", "--"}, pathSet...)
	if _, err := runGit(repoRoot, addArgs...); err != nil {
		return err
	}
	return nil
}

// deriveSubject implements PRD §3.4 subject-line precedence.
func deriveSubject(s *store.Store, slug, override string) string {
	if v := strings.TrimSpace(override); v != "" {
		return v
	}
	if spec, err := s.ReadFeatureFile(slug, "spec.md"); err == nil {
		for _, line := range strings.Split(spec, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# ") {
				return truncate72(strings.TrimSpace(strings.TrimPrefix(line, "# ")))
			}
		}
	}
	if req, err := s.ReadFeatureFile(slug, "request.md"); err == nil {
		for _, line := range strings.Split(req, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				return truncate72(line)
			}
		}
	}
	return fmt.Sprintf("feat(%s): apply tpatch feature", slug)
}

func truncate72(s string) string {
	if len(s) <= 72 {
		return s
	}
	return s[:72]
}

// readRecipeSHA returns the sha256 of the canonical apply-recipe.json
// bytes, or "none" when the artifact is absent. PRD §3.4.
func readRecipeSHA(s *store.Store, slug string) string {
	recipe, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "apply-recipe.json"))
	if err != nil {
		return "none"
	}
	if strings.TrimSpace(recipe) == "" {
		return "none"
	}
	return sha256Hex([]byte(recipe))
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// (abbrevSHA already provided by status_dag.go — reused here.)

// runGit shadows gitutil.runGit (which is unexported). It runs `git`
// in repoRoot with combined output and returns stdout on success.
func runGit(repoRoot string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return string(ee.Stderr), err
		}
		return "", err
	}
	return string(out), nil
}

// runGitCapture runs git with combined stdout+stderr captured, used
// by the commit step so a hook's stdout is surfaced verbatim.
func runGitCapture(repoRoot string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

// runLandDryRun implements PRD §3.5. It MUST NOT mutate the index,
// working tree, or .tpatch/. Values are derived from the current
// post-apply.patch (if any) and the current FeatureStatus.
func runLandDryRun(cmd *cobra.Command, s *store.Store, slug string) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "DRY RUN: tpatch land %s\n\n", slug)

	// Pre-flight section.
	status, _ := s.LoadFeatureStatus(slug)
	preflightErr := landPreflight(s.Root)
	depErr := workflow.CheckDependencyGate(s, slug)

	fmt.Fprintln(out, "Pre-flight:")
	fmt.Fprintf(out, "  feature state         : %s\n", status.State)
	if depErr != nil {
		fmt.Fprintf(out, "  hard-parent gate      : %v\n", depErr)
	} else {
		fmt.Fprintln(out, "  hard-parent gate      : ok")
	}
	if preflightErr != nil {
		fmt.Fprintf(out, "  working-tree hygiene  : %v\n", preflightErr)
	} else {
		fmt.Fprintln(out, "  working-tree hygiene  : clean")
	}
	fmt.Fprintln(out, "  collision check       : (deferred to embedded record)")
	fmt.Fprintln(out)

	// Embedded record section.
	fromRef, _ := cmd.Flags().GetString("from")
	autoBase, _ := cmd.Flags().GetBool("auto")
	filesFlag, _ := cmd.Flags().GetString("files")
	noRecord, _ := cmd.Flags().GetBool("no-record")
	mode := "working-tree"
	if autoBase {
		mode = "from-ref (auto)"
	} else if fromRef != "" {
		mode = "from-ref"
	}
	fmt.Fprintln(out, "Embedded record:")
	if noRecord {
		fmt.Fprintln(out, "  mode                  : skipped (--no-record)")
	} else {
		fmt.Fprintf(out, "  mode                  : %s\n", mode)
	}
	if fromRef != "" {
		fmt.Fprintf(out, "  --from                : %s\n", fromRef)
	}
	if filesFlag != "" {
		fmt.Fprintf(out, "  --files               : %s\n", filesFlag)
	}

	patch, _ := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
	if patch == "" {
		fmt.Fprintln(out, "  expected patch bytes  : 0 (no current capture; the embedded record will produce one)")
		fmt.Fprintln(out, "  expected files in patch: 0")
	} else {
		fmt.Fprintf(out, "  expected patch bytes  : %d (current capture)\n", len(patch))
		fmt.Fprintf(out, "  expected files in patch: %d\n", len(gitutil.FilesInPatch(patch)))
	}
	fmt.Fprintln(out)

	// Staging section. Dry-run does NOT run the embedded record
	// step, so there is no record-driven justification for sweeping
	// global metadata into the would-be commit. Pass nil so the
	// path-set builder leaves `.tpatch/upstream.lock` and
	// `.tpatch/FEATURES.md` alone (PRD §3.3 step 3).
	pathSet := computePathSet(s, slug, patch, nil)
	dirty, _ := dirtyPaths(s.Root)
	extras := classifyExtras(dirty, pathSet)
	fmt.Fprintln(out, "Staging (path set):")
	if len(pathSet) == 0 {
		fmt.Fprintln(out, "  (no current path set; rerun without --dry-run for a fresh capture)")
	}
	for _, p := range pathSet {
		fmt.Fprintf(out, "   M %s\n", p)
	}
	fmt.Fprintln(out)
	if len(extras) > 0 {
		fmt.Fprintln(out, "Outside path set (would refuse without --allow-extra-paths):")
		for _, p := range extras {
			fmt.Fprintf(out, "   M %s\n", p)
		}
		fmt.Fprintln(out)
	}

	// Commit section.
	subject := deriveSubject(s, slug, mustString(cmd, "message"))
	patchSHA := "<computed at land-time>"
	if patch != "" {
		patchSHA = sha256Hex([]byte(patch))
	}
	recipeSHA := readRecipeSHA(s, slug)
	fmt.Fprintln(out, "Commit:")
	fmt.Fprintf(out, "  subject               : %s\n", subject)
	fmt.Fprintln(out, "  trailers              :")
	fmt.Fprintf(out, "    Tpatch-Feature: %s\n", slug)
	fmt.Fprintf(out, "    Tpatch-Patch-SHA: %s\n", patchSHA)
	fmt.Fprintf(out, "    Tpatch-Recipe-SHA: %s\n", recipeSHA)
	fmt.Fprintf(out, "    Tpatch-Base-Commit: %s\n", status.Apply.BaseCommit)
	fmt.Fprintln(out)

	// Post-conditions section.
	headSHA, _ := gitutil.HeadCommit(s.Root)
	fmt.Fprintln(out, "Post-conditions if you re-run without --dry-run:")
	fmt.Fprintf(out, "  HEAD will move from %s to a new commit.\n", abbrevSHA(headSHA))
	fmt.Fprintln(out, "  Working tree will be clean.")
	fmt.Fprintf(out, "  Feature → commit binding: git log --grep '^Tpatch-Feature: %s$'\n", slug)
	fmt.Fprintln(out, "  status.json:apply.base_commit unchanged (owned by record/auto-base).")
	return nil
}

func mustString(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}
