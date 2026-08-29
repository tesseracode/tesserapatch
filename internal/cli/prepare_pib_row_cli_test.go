//go:build (linux && !android) || (darwin && !ios)

package cli

// Owning acceptance tests for the aggregate-ledger rows that had no
// body-sensitive executable target anywhere in the repository.
//
// Every leaf below is a literal subtest that performs that row's own setup,
// runs the real CLI through the real root error printer, and asserts the exact
// observable §18's matrix names for it. Nothing here is a label-only proxy: a
// row's subtest can only pass if the product actually behaves the way its
// matrix line says.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intent"
	"github.com/tesseracode/tesserapatch/internal/intentpub"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

const pibRowMutexText = "none of the others can be"

// pibRowFeature returns the on-disk feature directory for a workspace/slug.
func pibRowFeature(root, slug string) string {
	return filepath.Join(root, ".tpatch", "features", slug)
}

// pibRowFileIdentity returns a stable identity for a regular file: its bytes,
// its inode and its modification time. Two identities compare equal only when
// the file was neither rewritten in place nor replaced by a rename.
func pibRowFileIdentity(t *testing.T, path string) string {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	inode := uint64(0)
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		inode = uint64(stat.Ino)
	}
	return strings.Join([]string{
		hex.EncodeToString(sum[:]),
		info.ModTime().UTC().Format("2006-01-02T15:04:05.000000000Z"),
		info.Mode().String(),
	}, "|") + "|" + hex.EncodeToString([]byte{
		byte(inode), byte(inode >> 8), byte(inode >> 16), byte(inode >> 24),
		byte(inode >> 32), byte(inode >> 40), byte(inode >> 48), byte(inode >> 56),
	})
}

// pibRowWriteSpy records every write-intent operation the rooted publication
// path performs, so a row that asserts "zero writes to X" can be proven by
// observation rather than by absence of a visible change.
type pibRowWriteSpy struct {
	opens   []string
	renames []string
}

type pibRowSpyOps struct {
	intentpub.RootOps
	spy *pibRowWriteSpy
}

func (ops *pibRowSpyOps) OpenFile(name string, flag int, mode os.FileMode) (intentpub.RootFile, error) {
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_APPEND|os.O_TRUNC) != 0 {
		ops.spy.opens = append(ops.spy.opens, filepath.ToSlash(name))
	}
	return ops.RootOps.OpenFile(name, flag, mode)
}

func (ops *pibRowSpyOps) Rename(oldName, newName string) error {
	ops.spy.renames = append(ops.spy.renames, filepath.ToSlash(newName))
	return ops.RootOps.Rename(oldName, newName)
}

func pibRowInstallWriteSpy(t *testing.T) *pibRowWriteSpy {
	t.Helper()
	spy := &pibRowWriteSpy{}
	old := prepareIntentpubRootOps
	t.Cleanup(func() { prepareIntentpubRootOps = old })
	prepareIntentpubRootOps = func(rooted *os.Root) intentpub.RootOps {
		return &pibRowSpyOps{RootOps: intentpub.NewRootOps(rooted), spy: spy}
	}
	return spy
}

// pibRowRecordingProvider is a successful provider that keeps every prompt it
// was handed, so a context-flow row can inspect the real bytes.
type pibRowRecordingProvider struct {
	prompts []string
}

func (*pibRowRecordingProvider) Check(context.Context, provider.Config) (*provider.Health, error) {
	return &provider.Health{}, nil
}

func (p *pibRowRecordingProvider) Generate(
	_ context.Context,
	_ provider.Config,
	request provider.GenerateRequest,
) (string, error) {
	p.prompts = append(p.prompts, request.SystemPrompt+"\n"+request.UserPrompt)
	return "generated body for the aggregate ledger row", nil
}

func pibRowInstallProvider(t *testing.T, prov provider.Provider) *int {
	t.Helper()
	loads := 0
	old := prepareLoadProvider
	t.Cleanup(func() { prepareLoadProvider = old })
	prepareLoadProvider = func(*store.Store) (provider.Provider, provider.Config) {
		loads++
		if prov == nil {
			return nil, provider.Config{}
		}
		return prov, provider.Config{
			Type:    "openai-compatible",
			BaseURL: "https://provider.invalid",
			Model:   "aggregate-ledger-model",
		}
	}
	return &loads
}

// TestPIBRowPrepareFlagMutexCombinations owns the flag-grammar rows whose only
// observable is the process-boundary envelope of a rejected combination.
func TestPIBRowPrepareFlagMutexCombinations(t *testing.T) {
	t.Run("PIB-004", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 004")
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--regenerate",
		)
		if code != 1 || stdout != "" || !strings.Contains(stderr, pibRowMutexText) {
			t.Fatalf("--manual --regenerate = %d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("PIB-004: the rejected combination wrote to the workspace")
		}
	})

	t.Run("PIB-007", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 007")
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--check", "--timeout", "5s",
		)
		if code != 1 || stdout != "" || !strings.Contains(stderr, pibRowMutexText) {
			t.Fatalf("--check --timeout = %d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("PIB-007: the rejected combination wrote to the workspace")
		}
	})

	t.Run("PIB-008", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 008")
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--timeout", "5s",
		)
		if code != 1 || stdout != "" || !strings.Contains(stderr, pibRowMutexText) {
			t.Fatalf("--manual --timeout = %d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("PIB-008: the rejected combination wrote to the workspace")
		}
	})

	t.Run("PIB-009", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 009")
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--no-retry",
		)
		if code != 1 || stdout != "" || !strings.Contains(stderr, pibRowMutexText) {
			t.Fatalf("--manual --no-retry = %d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("PIB-009: the rejected combination wrote to the workspace")
		}
	})

	t.Run("PIB-010", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 010")
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--check", "--dry-run",
		)
		if code != 1 || stdout != "" || !strings.Contains(stderr, pibRowMutexText) {
			t.Fatalf("--check --dry-run = %d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})

	t.Run("PIB-235", func(t *testing.T) {
		// The U-category restatement additionally freezes the whole `.tpatch/`
		// tree, not just the mutating feature directory.
		root, slug := prepareS4Workspace(t, "PIB row 235")
		prepareS4WriteReadyBundle(t, root, slug, true)
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--check", "--dry-run",
		)
		if code != 1 || stdout != "" || !strings.Contains(stderr, pibRowMutexText) {
			t.Fatalf("--check --dry-run = %d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("PIB-235: the rejected combination changed the .tpatch tree")
		}
	})
}

// TestPIBRowPrepareArtifactAdmissibility owns the artifact-classification rows.
// Each fixture makes exactly one artifact inadmissible in exactly one way and
// asserts the refusal code and exit class §6 assigns to that kind.
func TestPIBRowPrepareArtifactAdmissibility(t *testing.T) {
	t.Run("PIB-026", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 026")
		specPath := filepath.Join(pibRowFeature(root, slug), "spec.md")
		if err := os.WriteFile(specPath, []byte("  \t\n \n"), 0o644); err != nil {
			t.Fatal(err)
		}
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		report := prepareS4Report(t, stdout)
		if code != 2 || report.Refusal == nil ||
			report.Refusal.Code != "artifact-empty-not-overwritten" {
			t.Fatalf("whitespace-only spec.md = exit %d refusal=%#v", code, report.Refusal)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("PIB-026: the empty-artifact refusal changed the workspace")
		}
	})

	t.Run("PIB-028", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 028")
		explorationPath := filepath.Join(pibRowFeature(root, slug), "exploration.md")
		if err := os.MkdirAll(explorationPath, 0o755); err != nil {
			t.Fatal(err)
		}
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		report := prepareS4Report(t, stdout)
		if code != 3 || report.Refusal == nil || report.Refusal.Code != "artifact-unsafe" {
			t.Fatalf("exploration.md directory = exit %d refusal=%#v", code, report.Refusal)
		}
	})

	t.Run("PIB-029", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 029")
		analysisPath := filepath.Join(pibRowFeature(root, slug), "analysis.md")
		if err := syscall.Mkfifo(analysisPath, 0o600); err != nil {
			t.Fatalf("mkfifo analysis.md: %v", err)
		}
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		report := prepareS4Report(t, stdout)
		if code != 3 || report.Refusal == nil || report.Refusal.Code != "artifact-unsafe" {
			t.Fatalf("analysis.md FIFO = exit %d refusal=%#v", code, report.Refusal)
		}
		// The FIFO is still a FIFO: nothing opened it, which is what would
		// otherwise have blocked forever on a reader-less pipe.
		info, err := os.Lstat(analysisPath)
		if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
			t.Fatalf("PIB-029: analysis.md is no longer the FIFO: %v %v", err, info)
		}
	})

	t.Run("PIB-031", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 031")
		specPath := filepath.Join(pibRowFeature(root, slug), "spec.md")
		if err := os.WriteFile(specPath, []byte("hand specification\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(specPath, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(specPath, 0o644) })
		if _, err := os.ReadFile(specPath); err == nil {
			// The running identity bypasses mode bits (root). Make the artifact
			// inadmissible in a way no identity bypasses; the asserted
			// observable — artifact-unsafe, exit 3 — is the same one §6 assigns
			// to every unreadable/non-regular required artifact.
			if err := os.Remove(specPath); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(specPath, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		report := prepareS4Report(t, stdout)
		if code != 3 || report.Refusal == nil || report.Refusal.Code != "artifact-unsafe" {
			t.Fatalf("mode-0000 spec.md = exit %d refusal=%#v", code, report.Refusal)
		}
	})

	t.Run("PIB-037", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 037")
		analysisPath := filepath.Join(pibRowFeature(root, slug), "analysis.md")
		oversize := bytes.Repeat([]byte("a"), intent.MaxArtifactBytes+1)
		if err := os.WriteFile(analysisPath, oversize, 0o644); err != nil {
			t.Fatal(err)
		}
		code, stdout, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		report := prepareS4Report(t, stdout)
		if code != 3 || report.Refusal == nil || report.Refusal.Code != "artifact-unsafe" {
			t.Fatalf("oversize analysis.md = exit %d refusal=%#v", code, report.Refusal)
		}
		info, err := os.Stat(analysisPath)
		if err != nil || info.Size() != int64(intent.MaxArtifactBytes+1) {
			t.Fatalf("PIB-037: the oversize artifact was touched: %v %v", err, info)
		}
	})

	t.Run("PIB-040", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 040")
		prepareS4WriteReadyBundle(t, root, slug, false)
		sidecar := filepath.Join(pibRowFeature(root, slug), "artifacts", "analysis.json")
		if err := os.MkdirAll(filepath.Dir(sidecar), 0o755); err != nil {
			t.Fatal(err)
		}
		_ = os.Remove(sidecar)
		if err := os.Symlink("../analysis.md", sidecar); err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		report := prepareS4Report(t, stdout)
		if code != 0 || report.Refusal != nil {
			t.Fatalf("symlinked sidecar = exit %d stderr=%q refusal=%#v", code, stderr, report.Refusal)
		}
		info, err := os.Lstat(sidecar)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("PIB-040: the optional sidecar symlink was replaced: %v %v", err, info)
		}
		target, err := os.Readlink(sidecar)
		if err != nil || target != "../analysis.md" {
			t.Fatalf("PIB-040: the sidecar symlink target changed to %q (%v)", target, err)
		}
	})

	t.Run("PIB-054", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 054")
		prepareS4WriteReadyBundle(t, root, slug, false)
		analysisPath := filepath.Join(pibRowFeature(root, slug), "analysis.md")
		if err := os.Remove(analysisPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("spec.md", analysisPath); err != nil {
			t.Fatal(err)
		}
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 3 || report.Refusal == nil || report.Refusal.Code != "artifact-unsafe" {
			t.Fatalf("--manual with symlinked analysis.md = exit %d refusal=%#v", code, report.Refusal)
		}
		if report.Mode != prepareModeManual {
			t.Fatalf("PIB-054: report mode = %q, want manual", report.Mode)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("PIB-054: the manual refusal changed the workspace")
		}
	})
}

// TestPIBRowPrepareDefaultModePreservation owns the default-mode rows whose
// observable is what prepare did *not* touch.
func TestPIBRowPrepareDefaultModePreservation(t *testing.T) {
	t.Run("PIB-023", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 023")
		feature := pibRowFeature(root, slug)
		if err := os.WriteFile(
			filepath.Join(feature, "analysis.md"), []byte("preserved analysis\n"), 0o644,
		); err != nil {
			t.Fatal(err)
		}
		spy := pibRowInstallWriteSpy(t)
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		if code != 0 || stderr != "" {
			t.Fatalf("suffix completion = exit %d stderr=%q\n%s", code, stderr, stdout)
		}
		for _, name := range spy.opens {
			if strings.HasSuffix(name, "analysis.md") {
				t.Fatalf("PIB-023: a preserved artifact was opened for write: %q (all %v)", name, spy.opens)
			}
		}
		for _, name := range spy.renames {
			if strings.HasSuffix(name, "analysis.md") {
				t.Fatalf("PIB-023: a rename targeted the preserved artifact: %q (all %v)", name, spy.renames)
			}
		}
		if len(spy.opens)+len(spy.renames) == 0 {
			t.Fatal("PIB-023: the write spy observed nothing, so it proves nothing")
		}
	})

	t.Run("PIB-030", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 030")
		feature := pibRowFeature(root, slug)
		if err := os.WriteFile(
			filepath.Join(feature, "analysis.md"), []byte("preserved analysis\n"), 0o644,
		); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(feature, "spec.md"), []byte("preserved specification\n"), 0o644,
		); err != nil {
			t.Fatal(err)
		}
		sidecar := filepath.Join(feature, "artifacts", "analysis.json")
		if err := os.MkdirAll(filepath.Dir(sidecar), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sidecar, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		analysisBefore := pibRowFileIdentity(t, filepath.Join(feature, "analysis.md"))
		specBefore := pibRowFileIdentity(t, filepath.Join(feature, "spec.md"))
		sidecarBefore := pibRowFileIdentity(t, sidecar)

		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		if code != 0 || stderr != "" {
			t.Fatalf("missing-only completion = exit %d stderr=%q\n%s", code, stderr, stdout)
		}
		exploration, err := os.ReadFile(filepath.Join(feature, "exploration.md"))
		if err != nil || len(bytes.TrimSpace(exploration)) == 0 {
			t.Fatalf("PIB-030: exploration.md was not created: %v", err)
		}
		if got := pibRowFileIdentity(t, filepath.Join(feature, "analysis.md")); got != analysisBefore {
			t.Fatalf("PIB-030: analysis.md changed\n got %s\nwant %s", got, analysisBefore)
		}
		if got := pibRowFileIdentity(t, filepath.Join(feature, "spec.md")); got != specBefore {
			t.Fatalf("PIB-030: spec.md changed\n got %s\nwant %s", got, specBefore)
		}
		if got := pibRowFileIdentity(t, sidecar); got != sidecarBefore {
			t.Fatalf("PIB-030: the sidecar changed\n got %s\nwant %s", got, sidecarBefore)
		}
	})

	t.Run("PIB-032", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 032")
		feature := pibRowFeature(root, slug)
		if err := os.WriteFile(
			filepath.Join(feature, "analysis.md"), []byte("path A analysis\n"), 0o644,
		); err != nil {
			t.Fatal(err)
		}
		sidecar := filepath.Join(feature, "artifacts", "analysis.json")
		if err := os.MkdirAll(filepath.Dir(sidecar), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sidecar, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		sidecarBefore := pibRowFileIdentity(t, sidecar)
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		if code != 0 || stderr != "" {
			t.Fatalf("path A completion = exit %d stderr=%q\n%s", code, stderr, stdout)
		}
		if got := pibRowFileIdentity(t, sidecar); got != sidecarBefore {
			t.Fatalf("PIB-032: the preserved sidecar changed\n got %s\nwant %s", got, sidecarBefore)
		}
	})

	t.Run("PIB-036", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 036")
		feature := pibRowFeature(root, slug)
		prepareS4WriteReadyBundle(t, root, slug, false)
		sidecar := filepath.Join(feature, "artifacts", "analysis.json")
		if err := os.MkdirAll(filepath.Dir(sidecar), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sidecar, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		statusPath := filepath.Join(feature, "status.json")
		raw, err := os.ReadFile(statusPath)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]any
		if err := json.Unmarshal(raw, &document); err != nil {
			t.Fatal(err)
		}
		document["state"] = "analyzed"
		staged, err := json.MarshalIndent(document, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(statusPath, append(staged, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		identities := map[string]string{}
		for _, name := range []string{
			"analysis.md", "spec.md", "exploration.md", "artifacts/analysis.json",
		} {
			path := filepath.Join(feature, filepath.FromSlash(name))
			identities[name] = pibRowFileIdentity(t, path)
		}
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		report := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" || report.Action != "adopt" {
			t.Fatalf("complete bundle adoption = exit %d stderr=%q action=%q", code, stderr, report.Action)
		}
		for name, want := range identities {
			path := filepath.Join(feature, filepath.FromSlash(name))
			if got := pibRowFileIdentity(t, path); got != want {
				t.Fatalf("PIB-036: %s changed on adoption\n got %s\nwant %s", name, got, want)
			}
		}
		var status map[string]any
		after, err := os.ReadFile(statusPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(after, &status); err != nil {
			t.Fatal(err)
		}
		if status["state"] != "defined" {
			t.Fatalf("PIB-036: adoption did not publish state=defined: %v", status["state"])
		}
	})

	t.Run("PIB-042", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 042")
		feature := pibRowFeature(root, slug)
		if err := os.WriteFile(
			filepath.Join(feature, "analysis.md"), []byte("preserved analysis\n"), 0o644,
		); err != nil {
			t.Fatal(err)
		}
		sidecar := filepath.Join(feature, "artifacts", "analysis.json")
		if err := os.MkdirAll(filepath.Dir(sidecar), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sidecar, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		spy := pibRowInstallWriteSpy(t)
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		if code != 0 || stderr != "" {
			t.Fatalf("path A completion = exit %d stderr=%q\n%s", code, stderr, stdout)
		}
		for _, name := range spy.renames {
			if strings.HasSuffix(name, "analysis.json") {
				t.Fatalf("PIB-042: a rename targeted the sidecar: %q (all %v)", name, spy.renames)
			}
		}
		if len(spy.renames) == 0 {
			t.Fatal("PIB-042: the rename spy observed nothing, so the sidecar claim is vacuous")
		}
		if _, err := os.Stat(filepath.Join(feature, "spec.md")); err != nil {
			t.Fatalf("PIB-042: spec.md was not generated, so no publication happened: %v", err)
		}
	})

	t.Run("PIB-043", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 043")
		feature := pibRowFeature(root, slug)
		const marker = "preserved analysis context marker 7f3a"
		if err := os.WriteFile(
			filepath.Join(feature, "analysis.md"), []byte("# Analysis\n\n"+marker+"\n"), 0o644,
		); err != nil {
			t.Fatal(err)
		}
		recorder := &pibRowRecordingProvider{}
		pibRowInstallProvider(t, recorder)
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		if code != 0 || stderr != "" {
			t.Fatalf("provider-backed completion = exit %d stderr=%q\n%s", code, stderr, stdout)
		}
		if len(recorder.prompts) == 0 {
			t.Fatal("PIB-043: the provider was never asked to generate anything")
		}
		carried := false
		for _, prompt := range recorder.prompts {
			carried = carried || strings.Contains(prompt, marker)
		}
		if !carried {
			t.Fatalf("PIB-043: no provider prompt carried the preserved analysis bytes: %v", recorder.prompts)
		}
	})

	t.Run("PIB-129", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 129")
		statusPath := filepath.Join(pibRowFeature(root, slug), "status.json")
		raw, err := os.ReadFile(statusPath)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]any
		if err := json.Unmarshal(raw, &document); err != nil {
			t.Fatal(err)
		}
		verify := map[string]any{
			"verified_at":  "2024-01-02T03:04:05Z",
			"recipe_sha":   strings.Repeat("a", 64),
			"outcome":      "pass",
			"tool_version": "pib-row-129",
		}
		document["verify"] = verify
		updated, err := json.MarshalIndent(document, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(statusPath, append(updated, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		wantVerify, err := json.Marshal(verify)
		if err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		if code != 0 || stderr != "" {
			t.Fatalf("prepare over a verify record = exit %d stderr=%q\n%s", code, stderr, stdout)
		}
		after, err := os.ReadFile(statusPath)
		if err != nil {
			t.Fatal(err)
		}
		var afterDocument map[string]any
		if err := json.Unmarshal(after, &afterDocument); err != nil {
			t.Fatal(err)
		}
		gotVerify, err := json.Marshal(afterDocument["verify"])
		if err != nil {
			t.Fatal(err)
		}
		if string(gotVerify) != string(wantVerify) {
			t.Fatalf("PIB-129: status.verify changed\n got %s\nwant %s", gotVerify, wantVerify)
		}
	})

	t.Run("PIB-137", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 137")
		statusPath := filepath.Join(pibRowFeature(root, slug), "status.json")
		raw, err := os.ReadFile(statusPath)
		if err != nil {
			t.Fatal(err)
		}
		var before map[string]any
		if err := json.Unmarshal(raw, &before); err != nil {
			t.Fatal(err)
		}
		// A legacy document carries no optional sub-records at all.
		for _, optional := range []string{"verify", "rejection", "depends_on", "notes"} {
			delete(before, optional)
		}
		legacy, err := json.MarshalIndent(before, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(statusPath, append(legacy, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		if code != 0 || stderr != "" {
			t.Fatalf("prepare over a legacy status = exit %d stderr=%q\n%s", code, stderr, stdout)
		}
		afterRaw, err := os.ReadFile(statusPath)
		if err != nil {
			t.Fatal(err)
		}
		var after map[string]any
		if err := json.Unmarshal(afterRaw, &after); err != nil {
			t.Fatal(err)
		}
		allowed := map[string]bool{
			"state": true, "last_command": true, "updated_at": true, "notes": true,
		}
		for key, value := range after {
			previous, existed := before[key]
			if !existed {
				if !allowed[key] {
					t.Fatalf("PIB-137: prepare added the field %q", key)
				}
				continue
			}
			if !pibRowJSONEqual(t, previous, value) && !allowed[key] {
				t.Fatalf("PIB-137: prepare changed the field %q", key)
			}
		}
		for key := range before {
			if _, kept := after[key]; !kept {
				t.Fatalf("PIB-137: prepare dropped the field %q", key)
			}
		}
	})

	t.Run("PIB-255", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 255")
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		report := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" || report.Outcome != "published" {
			t.Fatalf("default-mode publication = exit %d stderr=%q outcome=%q", code, stderr, report.Outcome)
		}
		archive := filepath.Join(pibRowFeature(root, slug), "artifacts", "intent-archive")
		if _, err := os.Stat(archive); !os.IsNotExist(err) {
			t.Fatalf("PIB-255: a default-mode run created %s: %v", archive, err)
		}
	})

	t.Run("PIB-256", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 256")
		spy := pibRowInstallWriteSpy(t)
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		report := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" {
			t.Fatalf("default-mode publication = exit %d stderr=%q\n%s", code, stderr, stdout)
		}
		for _, name := range append(append([]string{}, spy.opens...), spy.renames...) {
			if strings.Contains(name, "intent-archive") {
				t.Fatalf("PIB-256: a default-mode run wrote under the archive: %q", name)
			}
		}
		if report.Archive != nil {
			t.Fatalf("PIB-256: a default-mode run reported an archive generation: %#v", report.Archive)
		}
		if len(report.OrphanBlobs) != 0 {
			t.Fatalf("PIB-256: a default-mode run created blobs: %v", report.OrphanBlobs)
		}
	})

	t.Run("PIB-068", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 068")
		for _, args := range [][]string{
			{"--path", root, "prepare", slug, "--json", "--quiet"},
			{"--path", root, "prepare", slug, "--json", "--quiet"},
			{"--path", root, "prepare", slug, "--check", "--json", "--quiet"},
		} {
			if code, _, stderr, _ := runPrepare(t, args...); code != 0 {
				t.Fatalf("never-regenerating workspace run %v = exit %d stderr=%q", args, code, stderr)
			}
		}
		walked := 0
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			walked++
			if strings.Contains(filepath.ToSlash(path), "intent-archive") {
				t.Fatalf("PIB-068: a never-regenerating workspace holds %s", path)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if walked == 0 {
			t.Fatal("PIB-068: the walk visited nothing")
		}
	})

	t.Run("PIB-106", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "PIB row 106")
		dirty := filepath.Join(root, "dirty.txt")
		if err := os.WriteFile(dirty, []byte("dirty index entry\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		pibRowGit(t, root, "add", "dirty.txt")
		// Settle the stat cache so the recorded checksum is the steady state.
		beforePorcelain := pibRowGit(t, root, "status", "--porcelain")
		indexPath := filepath.Join(root, ".git", "index")
		before := pibRowFileDigest(t, indexPath)

		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--json", "--quiet")
		if code != 0 || stderr != "" {
			t.Fatalf("prepare over a dirty index = exit %d stderr=%q\n%s", code, stderr, stdout)
		}
		if after := pibRowFileDigest(t, indexPath); after != before {
			t.Fatalf("PIB-106: prepare rewrote the Git index (%s -> %s)", before, after)
		}
		afterPorcelain := pibRowGit(t, root, "status", "--porcelain")
		previous := map[string]bool{}
		for _, line := range strings.Split(strings.TrimRight(beforePorcelain, "\n"), "\n") {
			previous[line] = true
		}
		for _, line := range strings.Split(strings.TrimRight(afterPorcelain, "\n"), "\n") {
			if line == "" || previous[line] {
				continue
			}
			path := strings.TrimSpace(line[2:])
			if !strings.HasPrefix(path, ".tpatch") {
				t.Fatalf("PIB-106: prepare changed a non-.tpatch path: %q\nbefore:\n%s\nafter:\n%s",
					path, beforePorcelain, afterPorcelain)
			}
		}
		if !strings.Contains(afterPorcelain, ".tpatch") {
			t.Fatalf("PIB-106: the run reported no .tpatch path at all:\n%s", afterPorcelain)
		}
		if !previous["A  dirty.txt"] || !strings.Contains(afterPorcelain, "A  dirty.txt") {
			t.Fatalf("PIB-106: the staged index entry did not survive:\nbefore:\n%s\nafter:\n%s",
				beforePorcelain, afterPorcelain)
		}
	})
}

func pibRowJSONEqual(t *testing.T, left, right any) bool {
	t.Helper()
	leftBytes, err := json.Marshal(left)
	if err != nil {
		t.Fatal(err)
	}
	rightBytes, err := json.Marshal(right)
	if err != nil {
		t.Fatal(err)
	}
	return string(leftBytes) == string(rightBytes)
}

func pibRowFileDigest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func pibRowGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	command.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+filepath.Join(root, "missing-global"),
		"GIT_CONFIG_SYSTEM="+filepath.Join(root, "missing-system"),
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}
