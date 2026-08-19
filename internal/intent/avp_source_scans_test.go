package intent

import (
	"fmt"
	"go/ast"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestAVPSourceScans covers the matrix's pure `S` rows — the AST scans that
// pin the inspector's package boundary. Their guard-bearing siblings
// (AVP-089, AVP-150, AVP-193, AVP-194) live in TestAVPGuards with paired
// sensitivity fixtures; these rows assert the scan itself.
func TestAVPSourceScans(t *testing.T) {
	_, intentFiles := productionFiles(t, "internal/intent")
	command := prepareCommandFile(t)
	both := append(append([]*ast.File{}, intentFiles...), command...)

	t.Run("AVP-060", func(t *testing.T) {
		// No provenance inference source is referenced anywhere in the
		// inspector or renderer.
		forbidden := []string{
			"status.Notes", "status.LastCommand", "status.UpdatedAt", "status.RequestedAt",
			"info.ModTime", "pre.ModTime", "post.ModTime",
		}
		if err := forbiddenSelectors(both, forbidden); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"Notes", "LastCommand", "UpdatedAt", "RequestedAt", "ModTime", "recipe-provenance.json"} {
			for _, rel := range []string{"internal/intent/inspect.go", "internal/intent/render.go", "internal/cli/prepare.go"} {
				if strings.Contains(repoFile(t, rel), name) {
					t.Fatalf("%s references the forbidden provenance source %q", rel, name)
				}
			}
		}
	})

	t.Run("AVP-081", func(t *testing.T) {
		imports := importPaths(both)
		for _, banned := range []string{
			"github.com/tesseracode/tesserapatch/internal/provider",
			"net/http", "os/exec", "net", "net/url",
		} {
			if imports[banned] {
				t.Fatalf("the inspector or command imports %s", banned)
			}
		}
		if err := forbiddenSelectors(both, []string{"exec.Command", "exec.CommandContext", "http.Get", "http.NewRequest"}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("AVP-087", func(t *testing.T) {
		imports := importPaths(intentFiles)
		for _, banned := range []string{
			"github.com/tesseracode/tesserapatch/internal/store",
			"github.com/tesseracode/tesserapatch/internal/gitutil",
		} {
			if imports[banned] {
				t.Fatalf("internal/intent imports %s", banned)
			}
		}
		writers := []string{
			"store.MarkFeatureState", "store.SaveFeatureStatus", "store.WriteVerifyRecord",
			"store.WriteFeatureFile", "store.WriteArtifact", "store.RefreshFeaturesIndex",
		}
		if err := forbiddenSelectors(both, writers); err != nil {
			t.Fatal(err)
		}
		mutators := []string{
			"root.Create", "root.Mkdir", "root.MkdirAll", "root.Remove", "root.RemoveAll",
			"root.Rename", "root.Chmod", "root.Chown", "root.Chtimes", "root.Link",
			"root.Symlink", "root.WriteFile",
			"os.Create", "os.Mkdir", "os.MkdirAll", "os.Remove", "os.RemoveAll",
			"os.Rename", "os.WriteFile", "os.Chmod", "os.Chown", "os.Chtimes",
			"os.Link", "os.Symlink",
		}
		if err := forbiddenSelectors(both, mutators); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(repoFile(t, "internal/cli/prepare.go"), "O_WRONLY") ||
			strings.Contains(repoFile(t, "internal/intent/inspect.go"), "O_WRONLY") {
			t.Fatal("a write flag appears on an inspection open")
		}
	})

	t.Run("AVP-134", func(t *testing.T) {
		// The reverse call graph: only the two prepare command files import
		// the inspector. The population is the set of **tracked** non-test Go
		// files, taken from `git ls-files`. Walking the working tree instead
		// made the row fail for reasons that have nothing to do with the call
		// graph: a detached `git worktree` checked out inside the repository,
		// an editor scratch copy or any untracked experiment would be scanned,
		// and the previous fix for that — skipping one hard-coded scratch
		// directory name — only exempted the one name that had already bitten.
		sources := trackedGoSources(t)
		if err := checkInspectorImporters(sources); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("AVP-134/sensitivity", func(t *testing.T) {
		const inspectorImport = `"github.com/tesseracode/tesserapatch/internal/intent"`

		// A tracked non-prepare importer must fail the row.
		extra := map[string]string{}
		for path, source := range trackedGoSources(t) {
			extra[path] = source
		}
		extra[filepath.Join("internal", "workflow", "prepare_probe.go")] =
			"package workflow\n\nimport " + inspectorImport + "\n"
		if err := checkInspectorImporters(extra); err == nil {
			t.Fatal("a tracked forbidden importer passed the row")
		}

		// Losing the authorized prepare importer set must fail it too.
		if err := checkInspectorImporters(map[string]string{
			filepath.Join("internal", "cli", "phase2.go"): "package cli\n\nimport " + inspectorImport + "\n",
		}); err == nil {
			t.Fatal("moving the import to another file passed the row")
		}

		// An untracked file cannot participate: the population is exactly
		// what `git ls-files` reports, and every scanned path is tracked.
		tracked := map[string]bool{}
		for _, path := range gitLsFiles(t) {
			tracked[path] = true
		}
		for path := range trackedGoSources(t) {
			if !tracked[filepath.ToSlash(path)] {
				t.Fatalf("scanned %s, which git does not track", path)
			}
		}
	})

	t.Run("AVP-135", func(t *testing.T) {
		phase2 := repoFile(t, "internal/cli/phase2.go")
		for _, token := range []string{"intent.", "Inspect(", "CanonicalSlug", "StatePresentNonempty", "prepare --check"} {
			if strings.Contains(phase2, token) {
				t.Fatalf("internal/cli/phase2.go references %q", token)
			}
		}
	})

	t.Run("AVP-180", func(t *testing.T) {
		forbidden := []string{
			"syscall.CreateFile", "windows.Openat", "windows.GetFileType",
			"syscall.GetFileType", "rescap.openNoFollow",
		}
		if err := forbiddenSelectors(both, forbidden); err != nil {
			t.Fatal(err)
		}
		for _, rel := range []string{
			"internal/intent/inspect.go", "internal/intent/intent.go",
			"internal/intent/render.go", "internal/cli/prepare.go",
			"internal/intent/openflags_unix.go", "internal/intent/openflags_windows.go",
			"internal/intent/openflags_unsupported.go",
		} {
			source := repoFile(t, rel)
			for _, name := range []string{"FILE_FLAG_OPEN_REPARSE_POINT", "CreateFile", "GetFileType"} {
				if strings.Contains(source, name) {
					t.Fatalf("%s references the raw platform symbol %q", rel, name)
				}
			}
		}
	})
}

// gitLsFiles returns every path Git tracks, in repository-relative slash form.
// Untracked files, ignored files and nested worktree checkouts are absent by
// construction, which is the whole point: the scan population must be the
// repository's source, not whatever happens to sit in the working tree.
func gitLsFiles(t *testing.T) []string {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = repoRootDir(t)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	var paths []string
	for _, entry := range strings.Split(string(output), "\x00") {
		if entry != "" {
			paths = append(paths, entry)
		}
	}
	if len(paths) == 0 {
		t.Fatal("git ls-files reported no tracked files")
	}
	return paths
}

// trackedGoSources reads every tracked non-test Go file, keyed by its
// repository-relative path.
func trackedGoSources(t *testing.T) map[string]string {
	t.Helper()
	sources := map[string]string{}
	for _, path := range gitLsFiles(t) {
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			continue
		}
		local := filepath.FromSlash(path)
		data, err := os.ReadFile(filepath.Join(repoRootDir(t), local))
		if err != nil {
			t.Fatalf("read tracked file %s: %v", path, err)
		}
		sources[local] = string(data)
	}
	if len(sources) == 0 {
		t.Fatal("no tracked non-test Go sources; the AVP-134 scan would be vacuous")
	}
	return sources
}

// checkInspectorImporters is the AVP-134 body: only the tracked prepare
// command implementation files may import the inspector.
func checkInspectorImporters(sources map[string]string) error {
	const path = `"github.com/tesseracode/tesserapatch/internal/intent"`
	var importers []string
	for file, source := range sources {
		if strings.Contains(source, path) {
			importers = append(importers, file)
		}
	}
	sort.Strings(importers)
	want := []string{
		filepath.Join("internal", "cli", "prepare.go"),
		filepath.Join("internal", "cli", "prepare_publish.go"),
	}
	if len(importers) != len(want) || importers[0] != want[0] || importers[1] != want[1] {
		return fmt.Errorf("internal/intent is imported by %v, want only %v", importers, want)
	}
	return nil
}

// TestAVPSkillSurfaces is AVP-188: the six shipped skill surfaces must carry
// the §16.2 item 6 paragraph verbatim and must not describe exit 2 as a
// failure. It is a U row with a sensitivity arm of its own.
func TestAVPSkillSurfaces(t *testing.T) {
	const paragraph = "`tpatch prepare <slug> --check` exits 2 when the intent bundle is incomplete. " +
		"That is a report result, not a workflow or system failure: the command wrote nothing, " +
		"changed nothing, and the per-artifact rows say exactly what is missing. " +
		"Author the missing files and re-run, or continue without it — this check is optional."

	t.Run("AVP-188", func(t *testing.T) {
		for _, rel := range skillSurfaces {
			source := repoFile(t, rel)
			if !strings.Contains(source, paragraph) {
				t.Fatalf("%s does not contain the exit-2 paragraph verbatim", rel)
			}
			if err := checkExitTwoWording(exitTwoSentences(source)); err != nil {
				t.Fatalf("%s: %v", rel, err)
			}
		}
	})

	t.Run("AVP-188/sensitivity", func(t *testing.T) {
		reworded := "`tpatch prepare <slug> --check` fails with exit 2 when the intent bundle is incomplete."
		if err := checkExitTwoWording([]string{reworded}); err == nil {
			t.Fatal("a rewording to \"fails with exit 2\" passed the guard")
		}
		for _, wording := range []string{
			"prepare --check errors with exit 2 and you must abort the workflow.",
			"exit 2 is a blocker for the phase sequence.",
		} {
			if err := checkExitTwoWording([]string{wording}); err == nil {
				t.Fatalf("a failure wording passed the guard: %q", wording)
			}
		}
	})
}

func exitTwoSentences(source string) []string {
	var out []string
	for _, line := range strings.Split(source, "\n") {
		if strings.Contains(line, "exit 2") || strings.Contains(line, "exits 2") {
			out = append(out, line)
		}
	}
	return out
}

func checkExitTwoWording(sentences []string) error {
	if len(sentences) == 0 {
		return errNoExitTwoSentence
	}
	for _, sentence := range sentences {
		lower := strings.ToLower(sentence)
		for _, forbidden := range []string{"fails with exit 2", "errors with exit 2", "exit 2 is an error", "exit 2 is a blocker", "blocker"} {
			if strings.Contains(lower, forbidden) {
				return errExitTwoDescribedAsFailure
			}
		}
		if strings.Contains(lower, "abort the workflow") {
			return errExitTwoDescribedAsFailure
		}
	}
	return nil
}

var (
	errNoExitTwoSentence         = &wordingError{"no sentence describes exit 2"}
	errExitTwoDescribedAsFailure = &wordingError{"exit 2 is described as an error, failure, blocker or reason to abort"}
)

type wordingError struct{ message string }

func (e *wordingError) Error() string { return e.message }
