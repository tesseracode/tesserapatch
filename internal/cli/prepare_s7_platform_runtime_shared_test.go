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

func s7TestPlatformMutationRefusal(t *testing.T) {
	t.Helper()
	root, slug := s7PlatformWorkspace(t)
	before := readTree(t, filepath.Join(root, ".tpatch"))
	previousAcquire := prepareAcquireAuthority
	acquires := 0
	prepareAcquireAuthority = func(repoRoot string) (*intentlock.WorkspaceAuthority, error) {
		acquires++
		return previousAcquire(repoRoot)
	}
	t.Cleanup(func() { prepareAcquireAuthority = previousAcquire })

	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--json", "--quiet",
	)
	report := prepareS4Report(t, stdout)
	if code != 3 || report.Refusal == nil ||
		report.Refusal.Code != "prepare-unsupported-platform" {
		t.Fatalf("mutation refusal = %d stderr=%q report=%#v", code, stderr, report)
	}
	if acquires != 0 {
		t.Fatalf("unsupported mutation acquired authority %d time(s)", acquires)
	}
	if strings.Contains(stdout+stderr, root) {
		t.Fatal("unsupported-platform refusal leaked the absolute workspace path")
	}
	after := readTree(t, filepath.Join(root, ".tpatch"))
	if !bytes.Equal(before, after) {
		t.Fatal("unsupported-platform refusal mutated .tpatch")
	}
}

func s7TestPlatformCheckCompatibility(t *testing.T) {
	t.Helper()
	root, slug := s7PlatformWorkspace(t)
	setFeatureState(t, root, slug, "defined")
	writeFeaturePrereqs(t, root, map[string]string{
		"analysis.md":    "# Analysis\n",
		"spec.md":        "# Spec\n",
		"exploration.md": "# Exploration\n",
	})
	for _, fixture := range []struct {
		name string
		args []string
	}{
		{name: "check-ready-human.txt", args: []string{"--path", root, "prepare", slug, "--check"}},
		{name: "check-ready-json.txt", args: []string{"--path", root, "prepare", slug, "--check", "--json"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			before := readTree(t, filepath.Join(root, ".tpatch"))
			code, stdout, stderr, _ := runPrepare(t, fixture.args...)
			displayArgs := strings.ReplaceAll(strings.Join(fixture.args, " "), root, "<workspace>")
			got := fmt.Sprintf("$ tpatch %s\nexit %d\nstdout:\n%sstderr:\n%s",
				displayArgs, code,
				normalizePreparePIBBytes(stdout, root),
				normalizePreparePIBBytes(stderr, root),
			)
			want, err := os.ReadFile(filepath.Join(preparePIBGoldenDir, fixture.name))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal([]byte(got), want) {
				t.Fatalf("%s drifted\n--- want ---\n%s--- got ---\n%s", fixture.name, want, got)
			}
			after := readTree(t, filepath.Join(root, ".tpatch"))
			if !bytes.Equal(before, after) {
				t.Fatalf("%s mutated .tpatch", fixture.name)
			}
		})
	}
}

func s7PlatformWorkspace(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	repoStore, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	status, err := repoStore.AddFeature(store.AddFeatureInput{
		Title: "PIB golden", Request: "PIB golden", Slug: preparePIBSlug,
	})
	if err != nil {
		t.Fatal(err)
	}
	return root, status.Slug
}
