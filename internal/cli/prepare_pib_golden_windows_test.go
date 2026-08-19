//go:build windows

package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestPreparePIBUnsupportedPlatformRuntimeGolden(t *testing.T) {
	root := t.TempDir()
	gitGolden(t, root, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".tpatch/local/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{
		Title: "Golden feature", Request: "Golden feature", Slug: preparePIBSlug,
	}); err != nil {
		t.Fatalf("add feature: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "golden.env"), []byte("A=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resource := store.Resource{
		ResourceID:         ignoredFileResourceID,
		Kind:               store.ResourceKindIgnoredFile,
		Selector:           "config/golden.env",
		Args:               []store.ResourceArg{},
		AddedByToolVersion: "tpatch/dev",
	}
	manifest := store.ResourcesManifest{
		Version: store.ResourcesManifestVersion, Feature: preparePIBSlug,
		Resources: []store.Resource{resource},
	}
	if err := store.SaveResources(s, manifest); err != nil {
		t.Fatalf("save resources: %v", err)
	}

	args := []string{"--path", root, "feature", "resource", "capture", preparePIBSlug}
	var stdout, stderr bytes.Buffer
	cmd := buildRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	runErr := cmd.Execute()
	if runErr != nil {
		fmt.Fprintf(&stderr, "error: %v\n", runErr)
	}
	exit := exitCodeFor(runErr)
	displayArgs := strings.ReplaceAll(strings.Join(args, " "), root, "<workspace>")
	got := fmt.Sprintf("$ tpatch %s\nexit %d\nstdout:\n%sstderr:\n%s",
		displayArgs, exit, normalizePreparePIBBytes(stdout.String(), root), normalizePreparePIBBytes(stderr.String(), root))

	want, err := os.ReadFile(filepath.Join(preparePIBGoldenDir, "resource-unsupported-platform.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal([]byte(got), want) {
		t.Fatalf("native Windows unsupported-platform transcript drifted\n--- want ---\n%s--- got ---\n%s", want, got)
	}

	t.Run("PIB-222", func(t *testing.T) {
		before := snapshotTreeMetadata(t, "workspace", root)
		previousAcquire := prepareAcquireAuthority
		acquires := 0
		prepareAcquireAuthority = func(repoRoot string) (*intentlock.WorkspaceAuthority, error) {
			acquires++
			return previousAcquire(repoRoot)
		}
		t.Cleanup(func() { prepareAcquireAuthority = previousAcquire })

		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", preparePIBSlug, "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 3 || report.Refusal == nil || report.Refusal.Code != "prepare-unsupported-platform" {
			t.Fatalf("native Windows mutating prepare = %d stderr=%q report=%#v", code, stderr, report)
		}
		if acquires != 0 {
			t.Fatalf("native Windows refusal acquired workspace authority %d time(s)", acquires)
		}
		after := snapshotTreeMetadata(t, "workspace", root)
		if after != before {
			t.Fatalf("native Windows refusal mutated the workspace\n--- before ---\n%s--- after ---\n%s", before, after)
		}
	})
}
