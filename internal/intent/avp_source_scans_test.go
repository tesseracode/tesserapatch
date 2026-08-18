package intent

import (
	"go/ast"
	"os"
	"path/filepath"
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
		// The reverse call graph: only the prepare command file imports the
		// inspector.
		importers := inspectorImporters(t)
		if len(importers) != 1 || !strings.HasSuffix(importers[0], filepath.Join("internal", "cli", "prepare.go")) {
			t.Fatalf("internal/intent is imported by %v, want only internal/cli/prepare.go", importers)
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

// inspectorImporters returns every production file that imports the inspector.
func inspectorImporters(t *testing.T) []string {
	t.Helper()
	const path = "github.com/tesseracode/tesserapatch/internal/intent"
	var out []string
	repo := repoRootDir(t)
	err := filepath.WalkDir(repo, func(candidate string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "docs", "tests", ".golden-baseline":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(candidate, ".go") || strings.HasSuffix(candidate, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(candidate)
		if readErr != nil {
			return nil
		}
		if strings.Contains(string(data), `"`+path+`"`) {
			out = append(out, candidate)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
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
