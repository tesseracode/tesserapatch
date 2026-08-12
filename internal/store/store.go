// Package store provides the .tpatch/ data model, file I/O, and state management.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tesseracode/tesserapatch/internal/safety"
)

// Store provides read/write access to the .tpatch/ workspace.
type Store struct {
	Root string // absolute path to the project root
}

// FindProjectRoot walks up from start looking for a .tpatch/ directory.
func FindProjectRoot(start string) (string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if fileExists(filepath.Join(current, ".tpatch")) {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", errors.New("could not find .tpatch in this directory or any parent")
}

// Init creates a new .tpatch/ workspace at root.
func Init(root string) (*Store, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	store := &Store{Root: absRoot}
	if fileExists(store.tpatchDir()) {
		return nil, fmt.Errorf("%s already exists — already initialized", store.tpatchDir())
	}

	// Create directory structure
	dirs := []string{
		store.featuresDir(),
		store.steeringDir(),
		store.workflowsDir(),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	// Write config.yaml
	configContent := `# Tessera Patch configuration
provider:
  type: openai-compatible  # openai-compatible | anthropic
  base_url: ""
  model: ""
  auth_env: ""  # env var name containing auth token (NOT the secret itself)

# Merge strategy for applying patches: "3way" (default) or "rebase"
merge_strategy: 3way

# Max LLM validation retries when output fails to parse (0 disables retry)
max_retries: 2

# Max output tokens for the implement phase (default 16384). Bump higher
# for features that emit many large file bodies inline.
max_tokens_implement: 16384

# Shell command run by ` + "`tpatch test <slug>`" + ` (e.g. "go test ./...", "bun test")
test_command: ""

# Feature dependency DAG (ADR-011). Default true from v0.6.0.
# Set to false to opt back into v0.5.x byte-identity behaviour.
features_dependencies: true

# Path restructure detector thresholds (WP-003 PRD 9).
# Defaults: prefix-split >=3 files across >=2 prefixes; prefix-move >=5 files.
prefix_split_min_files: 3
prefix_split_min_prefixes: 2
prefix_move_min_files: 5
`
	if err := writeFile(store.configPath(), configContent); err != nil {
		return nil, err
	}

	// Write FEATURES.md
	featuresContent := "# Tracked Features\n\n*No features yet. Run `tpatch add <description>` to add one.*\n"
	if err := writeFile(store.featuresIndexPath(), featuresContent); err != nil {
		return nil, err
	}

	// Write upstream.lock
	lockContent := `# Upstream Lock
# Updated automatically by tpatch reconcile.
remote: ""
branch: ""
commit: ""
url: ""
`
	if err := writeFile(store.upstreamLockPath(), lockContent); err != nil {
		return nil, err
	}

	// Write steering files
	localSteering := "# Local Steering\n\n<!-- Add custom instructions for patching this project here. -->\n"
	if err := writeFile(filepath.Join(store.steeringDir(), "local.md"), localSteering); err != nil {
		return nil, err
	}

	upstreamSteering := "# Upstream Steering\n\n<!-- Cached PATCHING.md from upstream, if available. -->\n"
	if err := writeFile(filepath.Join(store.steeringDir(), "upstream.md"), upstreamSteering); err != nil {
		return nil, err
	}

	return store, nil
}

// Open loads an existing Store from root.
func Open(root string) (*Store, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	store := &Store{Root: absRoot}
	if !fileExists(store.tpatchDir()) {
		return nil, fmt.Errorf("%s is not initialized with tpatch — run 'tpatch init' first", absRoot)
	}
	return store, nil
}

// AddFeature creates a new feature directory with request.md and status.json.
func (s *Store) AddFeature(input AddFeatureInput) (FeatureStatus, error) {
	title := strings.TrimSpace(input.Title)
	request := strings.TrimSpace(input.Request)
	if title == "" {
		return FeatureStatus{}, errors.New("feature title is required")
	}
	if request == "" {
		request = title
	}

	slug := Slugify(input.Slug)
	if slug == "" {
		slug = Slugify(title)
	}
	if slug == "" {
		return FeatureStatus{}, errors.New("could not derive a valid feature slug")
	}

	featureDir := s.featureDir(slug)
	if fileExists(featureDir) {
		return FeatureStatus{}, fmt.Errorf("feature %q already exists", slug)
	}

	// Create feature directories
	if err := os.MkdirAll(s.featureArtifactsDir(slug), 0o755); err != nil {
		return FeatureStatus{}, err
	}
	if err := os.MkdirAll(s.featureReconciliationDir(slug), 0o755); err != nil {
		return FeatureStatus{}, err
	}

	now := nowStamp()
	status := FeatureStatus{
		ID:            slug,
		Slug:          slug,
		Title:         title,
		State:         StateRequested,
		Compatibility: CompatibilityUnknown,
		RequestedAt:   now,
		UpdatedAt:     now,
		LastCommand:   "add",
	}

	// Write request.md
	requestContent := fmt.Sprintf("# Feature Request: %s\n\n**Slug**: `%s`\n**Created**: %s\n\n## Description\n\n%s\n", title, slug, now, request)
	if err := writeFile(s.featureRequestPath(slug), requestContent); err != nil {
		return FeatureStatus{}, err
	}

	// Write status.json
	if err := s.SaveFeatureStatus(status); err != nil {
		return FeatureStatus{}, err
	}

	// Update FEATURES.md
	if err := s.RefreshFeaturesIndex(); err != nil {
		return FeatureStatus{}, err
	}

	return status, nil
}

// ListFeatures returns all tracked features sorted by slug.
func (s *Store) ListFeatures() ([]FeatureStatus, error) {
	entries, err := os.ReadDir(s.featuresDir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	features := make([]FeatureStatus, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		status, err := s.LoadFeatureStatus(entry.Name())
		if err != nil {
			continue // skip features without valid status.json
		}
		features = append(features, status)
	}

	sort.Slice(features, func(i, j int) bool {
		return features[i].Slug < features[j].Slug
	})

	return features, nil
}

// FeatureEntry pairs a slug with either a loaded status or a load error.
// Used by ListFeatureEntries so aggregate operations can surface broken
// features rather than silently skip them.
type FeatureEntry struct {
	Slug   string
	Status *FeatureStatus // nil iff Err != nil
	Err    error
}

// ListFeatureEntries returns every directory under features/ that
// contains a status.json entry (file OR unreadable — e.g. directory in
// place of file, permission denied, malformed JSON). Use this for
// aggregate operations that must surface broken features rather than
// silently skip them.
//
// Semantics (pinned by tests):
//   - Directory under features/ with NO status.json entry at all is
//     silently dropped — same treatment as a non-feature directory in
//     today's ListFeatures(). The contract is "directories that look
//     like features"; absence of status.json means it doesn't.
//   - Directory under features/ whose status.json cannot even be
//     stat-ed for reasons OTHER than ENOENT (permission denied, IO
//     error, non-traversable parent, …) is surfaced as an error
//     entry — silent omission is the same false-green class as the
//     original Slice D bug (revision-2 finding).
//   - Directory under features/ with a status.json that is itself a
//     directory (or otherwise unreadable at the JSON layer) is
//     surfaced as an error entry — its presence signals an attempt
//     at a feature.
//   - Successful loads sort lexicographically by slug; failed loads
//     are interleaved by slug so the overall slice is fully lex-sorted
//     for deterministic ordering.
//
// Existing ListFeatures() behavior is intentionally unchanged; other
// call sites (FEATURES.md rendering, dependency walkers) rely on
// silent skip-on-broken.
func (s *Store) ListFeatureEntries() ([]FeatureEntry, error) {
	entries, err := os.ReadDir(s.featuresDir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Distinguish "workspace not initialized" (.tpatch/ also
			// missing → callers that pre-check init handle it) from
			// "workspace corrupted" (.tpatch/ present but features/
			// gone). Silent (nil, nil) on the corruption case is the
			// same false-green class as the rev-1/rev-2 bugs, one
			// layer higher (workspace-discovery layer): aggregate
			// callers like `verify --all` would emit an empty report
			// and exit 0 (revision-3 finding).
			//
			// Revision-4: explicit 3-way branch on the .tpatch/ stat.
			// The previous 2-branch form silently treated *any*
			// non-nil stat error (EACCES, EIO, exotic FS errors) as
			// "workspace not initialized" → false-green empty
			// aggregate, exit 0. Same false-green class as rev-1/2/3,
			// one layer higher (workspace-discovery layer):
			// non-ENOENT errors must surface as errors.
			_, statErr := os.Stat(s.tpatchDir())
			switch {
			case statErr == nil:
				return nil, fmt.Errorf("workspace corruption: .tpatch/features directory is missing")
			case errors.Is(statErr, fs.ErrNotExist):
				return nil, nil
			default:
				return nil, fmt.Errorf("checking workspace state at %s: %w", s.tpatchDir(), statErr)
			}
		}
		return nil, err
	}

	out := make([]FeatureEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		slug := entry.Name()
		statusPath := s.featureStatusPath(slug)
		if _, statErr := os.Stat(statusPath); statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				// No status.json entry at all → not a tracked
				// feature from this helper's perspective. Drop
				// silently (matches today's behavior for empty
				// dirs and non-feature noise under features/).
				continue
			}
			// Any other stat failure (permission denied, IO
			// error, non-traversable parent, …) signals an
			// attempted feature whose entry we cannot inspect.
			// Surface it as an error row rather than silently
			// dropping it — silent omission is the same
			// false-green class as the original Slice D bug
			// (revision-2 finding).
			out = append(out, FeatureEntry{
				Slug: slug,
				Err:  fmt.Errorf("failed to stat status.json: %w", statErr),
			})
			continue
		}
		status, loadErr := s.LoadFeatureStatus(slug)
		if loadErr != nil {
			out = append(out, FeatureEntry{Slug: slug, Err: loadErr})
			continue
		}
		st := status
		out = append(out, FeatureEntry{Slug: slug, Status: &st})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Slug < out[j].Slug
	})
	return out, nil
}

// LoadFeatureStatus reads the status.json for a feature.
func (s *Store) LoadFeatureStatus(slug string) (FeatureStatus, error) {
	data, err := os.ReadFile(s.featureStatusPath(slug))
	if err != nil {
		return FeatureStatus{}, err
	}
	var status FeatureStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return FeatureStatus{}, err
	}
	return status, nil
}

// SaveFeatureStatus writes status.json for a feature and refreshes FEATURES.md
// so the human-readable index stays in sync with every state transition.
// Errors refreshing the index are swallowed: status.json is the source of
// truth and must land even if the derived index can't be rewritten (e.g.
// read-only FS, concurrent writer). The next SaveFeatureStatus call retries.
func (s *Store) SaveFeatureStatus(status FeatureStatus) error {
	if status.UpdatedAt == "" {
		status.UpdatedAt = nowStamp()
	}
	if err := writeJSONAtomic(s.featureStatusPath(status.Slug), status); err != nil {
		return err
	}
	_ = s.RefreshFeaturesIndex()
	return nil
}

// MarkFeatureState updates a feature's state and metadata.
func (s *Store) MarkFeatureState(slug string, state FeatureState, command, notes string) error {
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return err
	}
	if status.State == StateUnapplied && state != StateUnapplied && state != StateApplied {
		return fmt.Errorf("cannot transition feature %q from %q to %q: run `tpatch apply %s` to leave the unapplied state", slug, status.State, state, slug)
	}
	status.State = state
	status.LastCommand = command
	status.UpdatedAt = nowStamp()
	status.Notes = strings.TrimSpace(notes)
	return s.SaveFeatureStatus(status)
}

// WriteVerifyRecord persists the freshness overlay produced by the
// explicit `tpatch verify` verb (ADR-013 D1 / D5). This is the ONLY
// store-level entry point for setting `FeatureStatus.Verify`; read paths
// (`LoadFeatureStatus`, `ComposeLabels`, status rendering) must NOT call
// it. Slice A: `tpatch verify` is the sole caller; Slice B will add
// `tpatch amend` (recipe-touching) as the second producer.
//
// `LastCommand = "verify"` and `UpdatedAt` are bumped via SaveFeatureStatus.
// `FeatureState` is left untouched — verify is a freshness overlay, not a
// lifecycle transition (ADR-013 D1).
func (s *Store) WriteVerifyRecord(slug string, record VerifyRecord) error {
	status, err := s.LoadFeatureStatus(slug)
	if err != nil {
		return err
	}
	_, err = s.WriteVerifyRecordFrom(status, record)
	return err
}

// WriteVerifyRecordFrom persists the freshness record onto an ALREADY
// CAPTURED status and returns the exact value written.
//
// v0.15.1 Wave C rev-1 (adjudication finding 2): `tpatch verify` holds
// one immutable capture of every feature; re-loading `status.json` here
// just to write it back is the persistence reload that contract
// forbids, and it also made the in-memory capture diverge from disk
// (`last_command` / `updated_at` are set by this writer).
func (s *Store) WriteVerifyRecordFrom(status FeatureStatus, record VerifyRecord) (FeatureStatus, error) {
	rec := record
	status.Verify = &rec
	status.LastCommand = "verify"
	status.UpdatedAt = nowStamp()
	if err := s.SaveFeatureStatus(status); err != nil {
		return FeatureStatus{}, err
	}
	return status, nil
}

// ReadFeatureFile reads a named file from the feature directory.
func (s *Store) ReadFeatureFile(slug, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(s.featureDir(slug), name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFeatureFile writes a named file to the feature directory.
func (s *Store) WriteFeatureFile(slug, name, content string) error {
	target := filepath.Join(s.featureDir(slug), name)
	if err := safety.EnsureSafeRepoPath(s.Root, target); err != nil {
		return fmt.Errorf("unsafe path in WriteFeatureFile: %w", err)
	}
	return writeFile(target, content)
}

// WriteArtifact writes a file to the feature's artifacts directory.
//
// PRD-multi-slug-reconcile-canonical-safety §4.5 D5 / ADR-030 D4
// (v0.12.1): every `.patch`-suffixed artifact is inspected for
// `.git/**` references before it lands on disk. Any header line
// (`diff --git`, `--- `, `+++ `, `Only in`, `Binary files`, `diff -`)
// pointing at `.git/`, `.git\`, or the exact path `.git` refuses
// the write with a descriptive error. Defense-in-depth against a
// future upstream patch producer regressing the D4 diff-boundary
// exclusion. INV-3/INV-6 of the PRD.
func (s *Store) WriteArtifact(slug, name, content string) error {
	target := s.featureArtifactPath(slug, name)
	if err := safety.EnsureSafeRepoPath(s.Root, target); err != nil {
		return fmt.Errorf("unsafe path in WriteArtifact: %w", err)
	}
	if strings.HasSuffix(name, ".patch") {
		if offending, ok := patchReferencesGitInternal(content); ok {
			return fmt.Errorf("WriteArtifact refused: %s/%s references repository-internal path %q — canonical patches must not carry .git/** entries (PRD-multi-slug-reconcile-canonical-safety D5, ADR-030 D4)", slug, name, offending)
		}
	}
	return writeFile(target, content)
}

// patchReferencesGitInternal scans `patch` for any header line that
// points at `.git/**` or the exact path `.git`. Returns the offending
// path and true on the first match; empty string and false when the
// patch is clean. Header shapes recognised:
//
//	diff --git a/<path> b/<path>
//	diff -<flags> <src> <dst>
//	--- a/<path>   +++ b/<path>
//	Only in <dir>: .git
//	Binary files a/<path> and b/<path> differ
func patchReferencesGitInternal(patch string) (string, bool) {
	if patch == "" {
		return "", false
	}
	if !strings.Contains(patch, ".git") {
		return "", false
	}
	for _, line := range strings.Split(patch, "\n") {
		if p, ok := headerReferencedGitPath(line); ok {
			return p, true
		}
	}
	return "", false
}

// headerReferencedGitPath extracts the file path from a diff/patch
// header line and returns (path, true) when it references `.git`,
// `.git/**`, `\.git\**`, `/.git`, or `/.git/**`. Kept in the store
// package (not gitutil) so the boundary guard is enforced at the
// write layer even when the calling code path is not gitutil-owned.
func headerReferencedGitPath(line string) (string, bool) {
	stripAB := func(p string) string {
		if strings.HasPrefix(p, "a/") {
			return strings.TrimPrefix(p, "a/")
		}
		if strings.HasPrefix(p, "b/") {
			return strings.TrimPrefix(p, "b/")
		}
		return p
	}
	isGit := func(p string) bool {
		if p == "" || p == "/dev/null" {
			return false
		}
		if p == ".git" || p == ".git/" || p == ".git\\" {
			return true
		}
		if strings.HasPrefix(p, ".git/") || strings.HasPrefix(p, ".git\\") {
			return true
		}
		if strings.Contains(p, "/.git/") || strings.Contains(p, "\\.git\\") {
			return true
		}
		if strings.HasSuffix(p, "/.git") || strings.HasSuffix(p, "\\.git") {
			return true
		}
		return false
	}
	switch {
	case strings.HasPrefix(line, "diff --git "):
		rest := strings.TrimPrefix(line, "diff --git ")
		for _, f := range strings.Fields(rest) {
			p := stripAB(f)
			if isGit(p) {
				return p, true
			}
		}
	case strings.HasPrefix(line, "diff -"):
		for _, f := range strings.Fields(line) {
			p := stripAB(f)
			if isGit(p) {
				return p, true
			}
		}
	case strings.HasPrefix(line, "--- "):
		p := strings.TrimSpace(strings.TrimPrefix(line, "--- "))
		p = strings.SplitN(p, "\t", 2)[0]
		p = stripAB(p)
		if isGit(p) {
			return p, true
		}
	case strings.HasPrefix(line, "+++ "):
		p := strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
		p = strings.SplitN(p, "\t", 2)[0]
		p = stripAB(p)
		if isGit(p) {
			return p, true
		}
	case strings.HasPrefix(line, "Only in "):
		rest := strings.TrimPrefix(line, "Only in ")
		if idx := strings.LastIndex(rest, ": "); idx >= 0 {
			dir := rest[:idx]
			leaf := strings.TrimSpace(rest[idx+2:])
			if leaf == ".git" {
				return dir + "/.git", true
			}
			if isGit(dir) || isGit(dir+"/"+leaf) {
				return dir + "/" + leaf, true
			}
		}
	case strings.HasPrefix(line, "Binary files "):
		for _, f := range strings.Fields(line) {
			p := stripAB(f)
			if isGit(p) {
				return p, true
			}
		}
	}
	return "", false
}

// LoadConfig reads the YAML config (parsed as simple key extraction for zero-dep).
func (s *Store) LoadConfig() (Config, error) {
	data, err := os.ReadFile(s.configPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}
	return parseYAMLConfig(string(data)), nil
}

// SaveConfig writes the YAML config.
func (s *Store) SaveConfig(cfg Config) error {
	mergeStrat := cfg.MergeStrategy
	if mergeStrat == "" {
		mergeStrat = "3way"
	}
	maxRetries := cfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	maxTokensImplement := cfg.MaxTokensImplement
	if maxTokensImplement <= 0 {
		maxTokensImplement = DefaultMaxTokensImplement
	}
	featuresDeps := "false"
	if cfg.FeaturesDependencies {
		featuresDeps = "true"
	}
	prefixSplitMinFiles := cfg.PathRestructurePrefixSplitMinFiles
	if prefixSplitMinFiles <= 0 {
		prefixSplitMinFiles = DefaultPathRestructurePrefixSplitMinFiles
	}
	prefixSplitMinPrefixes := cfg.PathRestructurePrefixSplitMinPrefixes
	if prefixSplitMinPrefixes <= 0 {
		prefixSplitMinPrefixes = DefaultPathRestructurePrefixSplitMinPrefixes
	}
	prefixMoveMinFiles := cfg.PathRestructurePrefixMoveMinFiles
	if prefixMoveMinFiles <= 0 {
		prefixMoveMinFiles = DefaultPathRestructurePrefixMoveMinFiles
	}
	initiatorLine := ""
	if cfg.Provider.Initiator != "" {
		initiatorLine = fmt.Sprintf("  initiator: %s\n", yamlQuote(cfg.Provider.Initiator))
	}
	content := fmt.Sprintf(`# Tessera Patch configuration
provider:
  type: %s
  base_url: %s
  model: %s
  auth_env: %s
%s
# Merge strategy for applying patches: "3way" (default) or "rebase"
merge_strategy: %s

# Max LLM validation retries when output fails to parse (0 disables retry)
max_retries: %d

# Max output tokens for the implement phase (default 16384). Bump higher
# for features that emit many large file bodies inline.
max_tokens_implement: %d

# Shell command run by `+"`tpatch test <slug>`"+` (e.g. "go test ./...", "bun test")
test_command: %s

# Feature dependency DAG (ADR-011). Default false until v0.6.0.
features_dependencies: %s

# Path restructure detector thresholds (WP-003 PRD 9).
# Defaults: prefix-split >=3 files across >=2 prefixes; prefix-move >=5 files.
prefix_split_min_files: %d
prefix_split_min_prefixes: %d
prefix_move_min_files: %d
`, yamlQuote(cfg.Provider.Type), yamlQuote(cfg.Provider.BaseURL),
		yamlQuote(cfg.Provider.Model), yamlQuote(cfg.Provider.AuthEnv),
		initiatorLine, mergeStrat,
		maxRetries, maxTokensImplement, yamlQuote(cfg.TestCommand),
		featuresDeps, prefixSplitMinFiles, prefixSplitMinPrefixes, prefixMoveMinFiles)
	// M17 Wave D: only emit the patch-id detector keys when non-default,
	// so v0.6.x → v0.8.0 fixtures round-trip byte-identical until the
	// operator explicitly opts in.
	if cfg.PatchIDDetectorEnabled {
		content += "\n# Phase-1.5 patch-id detector (PRD-patch-already-upstream-detector).\n"
		content += "patch_id_detector_enabled: true\n"
	}
	if cfg.PatchIDScanLimit > 0 {
		content += fmt.Sprintf("patch_id_scan_limit: %d\n", cfg.PatchIDScanLimit)
	}
	return writeFile(s.configPath(), content)
}

// RemoveFeature deletes the feature directory (including artifacts,
// patches, reconciliation, status.json) and refreshes FEATURES.md.
// Returns an error when the slug does not exist.
func (s *Store) RemoveFeature(slug string) error {
	dir := s.featureDir(slug)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("feature %s does not exist", slug)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove %s: %w", dir, err)
	}
	return s.RefreshFeaturesIndex()
}

// HasPatchingInstructions checks for a PATCHING.md in the project root.
func (s *Store) HasPatchingInstructions() bool {
	return fileExists(filepath.Join(s.Root, "PATCHING.md"))
}

// RefreshFeaturesIndex rebuilds FEATURES.md from current feature state.
//
// Unapplied features (v0.14.0, ADR-032 D8) are rendered in a distinct
// section after the active table and before rejected features. Rejected
// features (v0.13.0 GH #6) remain in the terminal trailing section,
// mirroring `tpatch status`'s default exclusion. All views are sourced
// from the same feature slice.
func (s *Store) RefreshFeaturesIndex() error {
	features, err := s.ListFeatures()
	if err != nil {
		return err
	}

	active := make([]FeatureStatus, 0, len(features))
	unapplied := make([]FeatureStatus, 0)
	rejected := make([]FeatureStatus, 0)
	for _, f := range features {
		switch f.State {
		case StateUnapplied:
			unapplied = append(unapplied, f)
			continue
		case StateRejected:
			rejected = append(rejected, f)
			continue
		}
		active = append(active, f)
	}

	var b strings.Builder
	b.WriteString("# Tracked Features\n\n")
	if len(active) == 0 && len(unapplied) == 0 && len(rejected) == 0 {
		b.WriteString("*No features yet. Run `tpatch add <description>` to add one.*\n")
	} else if len(active) == 0 {
		b.WriteString("*No active features.*\n")
	} else {
		b.WriteString("| Slug | Title | State | Compatibility |\n")
		b.WriteString("|------|-------|-------|---------------|\n")
		for _, f := range active {
			b.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n", f.Slug, f.Title, f.State, f.Compatibility))
		}
	}

	if len(unapplied) > 0 {
		b.WriteString("\n## Unapplied\n\n")
		b.WriteString("| Slug | Title | State | Note |\n")
		b.WriteString("|------|-------|-------|------|\n")
		for _, f := range unapplied {
			b.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n",
				f.Slug, f.Title, f.State, singleLineCell(f.Notes)))
		}
	}

	if len(rejected) > 0 {
		b.WriteString("\n## Rejected\n\n")
		b.WriteString("| Slug | Reason | Evidence | Note |\n")
		b.WriteString("|------|--------|----------|------|\n")
		for _, f := range rejected {
			reason, evidence, note := "", "", ""
			if f.Rejection != nil {
				reason = f.Rejection.Reason
				note = singleLineCell(f.Rejection.Note)
				paths := make([]string, 0, len(f.Rejection.Evidence))
				for _, e := range f.Rejection.Evidence {
					paths = append(paths, "`"+e.Path+"`")
				}
				evidence = strings.Join(paths, ", ")
			}
			b.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n", f.Slug, reason, evidence, note))
		}
	}

	return writeFile(s.featuresIndexPath(), b.String())
}

// singleLineCell collapses newlines and escapes pipe characters so a
// free-form operator note cannot break the generated markdown table.
func singleLineCell(v string) string {
	r := strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "|", "\\|")
	return strings.TrimSpace(r.Replace(v))
}

// Path accessors

func (s *Store) tpatchDir() string             { return filepath.Join(s.Root, ".tpatch") }
func (s *Store) configPath() string            { return filepath.Join(s.tpatchDir(), "config.yaml") }
func (s *Store) featuresIndexPath() string     { return filepath.Join(s.tpatchDir(), "FEATURES.md") }
func (s *Store) upstreamLockPath() string      { return filepath.Join(s.tpatchDir(), "upstream.lock") }
func (s *Store) steeringDir() string           { return filepath.Join(s.tpatchDir(), "steering") }
func (s *Store) workflowsDir() string          { return filepath.Join(s.tpatchDir(), "workflows") }
func (s *Store) featuresDir() string           { return filepath.Join(s.tpatchDir(), "features") }
func (s *Store) featureDir(slug string) string { return filepath.Join(s.featuresDir(), slug) }
func (s *Store) featureArtifactsDir(slug string) string {
	return filepath.Join(s.featureDir(slug), "artifacts")
}
func (s *Store) featureReconciliationDir(slug string) string {
	return filepath.Join(s.featureDir(slug), "reconciliation")
}
func (s *Store) featureRequestPath(slug string) string {
	return filepath.Join(s.featureDir(slug), "request.md")
}
func (s *Store) featureStatusPath(slug string) string {
	return filepath.Join(s.featureDir(slug), "status.json")
}
func (s *Store) featureArtifactPath(slug, name string) string {
	return filepath.Join(s.featureArtifactsDir(slug), name)
}

// TpatchDir returns the path to .tpatch/.
func (s *Store) TpatchDir() string { return s.tpatchDir() }

// ConfigPath returns the path to config.yaml.
func (s *Store) ConfigPath() string { return s.configPath() }

// NextPatchNumber returns the next sequential patch number for a feature.
func (s *Store) NextPatchNumber(slug string) int {
	patchDir := filepath.Join(s.featureDir(slug), "patches")
	entries, err := os.ReadDir(patchDir)
	if err != nil {
		return 1
	}
	max := 0
	for _, e := range entries {
		name := e.Name()
		if len(name) >= 3 {
			var n int
			if _, err := fmt.Sscanf(name[:3], "%03d", &n); err == nil && n > max {
				max = n
			}
		}
	}
	return max + 1
}

// WritePatch writes a patch to the sequential patches/ directory.
func (s *Store) WritePatch(slug, label, content string) (string, error) {
	num := s.NextPatchNumber(slug)
	filename := fmt.Sprintf("%03d-%s.patch", num, label)
	patchDir := filepath.Join(s.featureDir(slug), "patches")
	if err := os.MkdirAll(patchDir, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(patchDir, filename)
	if err := safety.EnsureSafeRepoPath(s.Root, target); err != nil {
		return "", err
	}
	return filename, writeFile(target, content)
}

// SaveApplySession writes the apply-session.json artifact.
func (s *Store) SaveApplySession(slug string, session ApplySession) error {
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return s.WriteArtifact(slug, "apply-session.json", string(data)+"\n")
}

// Helpers

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, string(data)+"\n")
}

func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'), 0o644)
}

func writeFileAtomic(path string, content []byte, mode fs.FileMode) error {
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		mode = info.Mode().Perm()
	}
	return writeFileAtomicWithRename(path, content, mode, os.Rename)
}

func writeFileAtomicWithRename(path string, content []byte, mode fs.FileMode, rename func(string, string) error) (retErr error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		if retErr != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := rename(tmpPath, path); err != nil {
		return err
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func nowStamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// parseYAMLConfig does minimal YAML parsing for our known config structure.
// This avoids adding a yaml dependency — our config format is flat and simple.
func parseYAMLConfig(content string) Config {
	cfg := Config{}
	cfg.Provider.Type = extractYAMLValue(content, "type")
	cfg.Provider.BaseURL = extractYAMLValue(content, "base_url")
	cfg.Provider.Model = extractYAMLValue(content, "model")
	cfg.Provider.AuthEnv = extractYAMLValue(content, "auth_env")
	cfg.Provider.Initiator = extractYAMLValue(content, "initiator")
	cfg.MergeStrategy = extractYAMLValue(content, "merge_strategy")
	if cfg.MergeStrategy == "" {
		cfg.MergeStrategy = "3way"
	}
	if v := extractYAMLValue(content, "max_retries"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n >= 0 {
			cfg.MaxRetries = n
		}
	} else {
		cfg.MaxRetries = 2
	}
	if v := extractYAMLValue(content, "max_tokens_implement"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.MaxTokensImplement = n
		}
	}
	cfg.TestCommand = extractYAMLValue(content, "test_command")
	if v := extractYAMLValue(content, "copilot_native_optin"); v == "true" {
		cfg.CopilotNativeOptIn = true
	}
	cfg.CopilotNativeOptInAt = extractYAMLValue(content, "copilot_native_optin_at")
	if v := extractYAMLValue(content, "features_dependencies"); v != "" {
		// Explicit value wins. Default (when key is absent) is true
		// from v0.6.0 onward — see ADR-011 D9 + PRD-feature-dependencies.
		cfg.FeaturesDependencies = v != "false"
	} else {
		cfg.FeaturesDependencies = true
	}
	if v := extractYAMLValue(content, "patch_id_detector_enabled"); v != "" {
		// Default false until v0.7.x (PRD-patch-already-upstream-detector
		// §6). The key is absent in pre-M17-Wave-D config.yaml, which
		// must continue to behave byte-identically.
		cfg.PatchIDDetectorEnabled = v == "true"
	}
	if v := extractYAMLValue(content, "patch_id_scan_limit"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.PatchIDScanLimit = n
		}
	}
	if v := extractYAMLValue(content, "prefix_split_min_files"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.PathRestructurePrefixSplitMinFiles = n
		}
	}
	if v := extractYAMLValue(content, "prefix_split_min_prefixes"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.PathRestructurePrefixSplitMinPrefixes = n
		}
	}
	if v := extractYAMLValue(content, "prefix_move_min_files"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.PathRestructurePrefixMoveMinFiles = n
		}
	}
	return cfg
}

func extractYAMLValue(content, key string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+":") {
			val := strings.TrimPrefix(trimmed, key+":")
			val = strings.TrimSpace(val)
			// Remove surrounding quotes
			val = strings.Trim(val, "\"'")
			// Remove inline comments
			if idx := strings.Index(val, " #"); idx >= 0 {
				val = strings.TrimSpace(val[:idx])
				val = strings.Trim(val, "\"'")
			}
			return val
		}
	}
	return ""
}

func yamlQuote(s string) string {
	if s == "" {
		return `""`
	}
	// Quote if it contains special chars
	if strings.ContainsAny(s, ": #{}[]|>&*!%@`") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}
