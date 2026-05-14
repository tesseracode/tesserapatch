package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initRepo bootstraps a minimal git repository under t.TempDir() with
// one committed file so HEAD exists. The helper is duplicated locally
// rather than imported from the CLI test package to keep gitutil tests
// dependency-free.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "."},
		{"commit", "-m", "seed"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	return dir
}

func gitMust(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}

// TestCaptureStagedPatch_OnlyStaged verifies that
// CaptureStagedPatch returns the HEAD→index diff and NOT
// index→worktree changes on the same path.
func TestCaptureStagedPatch_OnlyStaged(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("staged-line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitMust(t, dir, "add", "a.txt")
	// Layer unstaged extra content on top.
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("staged-line\nlocal-wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch, summary, err := CaptureStagedPatch(dir, nil)
	if err != nil {
		t.Fatalf("CaptureStagedPatch: %v", err)
	}
	if !strings.Contains(patch, "+staged-line") {
		t.Errorf("staged patch must contain staged content: %s", patch)
	}
	if strings.Contains(patch, "local-wip") {
		t.Errorf("staged patch must NOT contain unstaged content: %s", patch)
	}
	if summary.StagedPaths != 1 {
		t.Errorf("expected 1 staged path, got %d", summary.StagedPaths)
	}
	if summary.UnrelatedUnstagedPaths != 0 {
		// a.txt is in both sets; not unrelated.
		t.Errorf("expected 0 unrelated unstaged, got %d", summary.UnrelatedUnstagedPaths)
	}
}

// TestStagedUnstagedOverlap_DetectsOverlap exercises the set
// arithmetic that backs the refuse-on-overlap rule.
func TestStagedUnstagedOverlap_DetectsOverlap(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitMust(t, dir, "add", "a.txt")
	gitMust(t, dir, "commit", "-m", "add a")

	// Stage v2; then layer v3 unstaged.
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("v2\n"), 0o644)
	gitMust(t, dir, "add", "a.txt")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("v3\n"), 0o644)

	// Also stage a separate file (unrelated staged).
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b-staged\n"), 0o644)
	gitMust(t, dir, "add", "b.txt")

	overlap, unrelatedStaged, unrelatedUnstaged, err := StagedUnstagedOverlap(dir, nil)
	if err != nil {
		t.Fatalf("StagedUnstagedOverlap: %v", err)
	}
	if len(overlap) != 1 || overlap[0] != "a.txt" {
		t.Errorf("expected overlap=[a.txt], got %v", overlap)
	}
	if len(unrelatedStaged) != 1 || unrelatedStaged[0] != "b.txt" {
		t.Errorf("expected unrelatedStaged=[b.txt], got %v", unrelatedStaged)
	}
	if len(unrelatedUnstaged) != 0 {
		t.Errorf("expected no unrelatedUnstaged, got %v", unrelatedUnstaged)
	}
}

// TestValidateStagedPatch_HappyPath confirms the temp-index handshake
// approves a well-formed HEAD→index patch.
func TestValidateStagedPatch_HappyPath(t *testing.T) {
	dir := initRepo(t)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("staged\n"), 0o644)
	gitMust(t, dir, "add", "a.txt")
	patch, _, err := CaptureStagedPatch(dir, nil)
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if err := ValidateStagedPatch(dir, patch); err != nil {
		t.Fatalf("ValidateStagedPatch: %v", err)
	}
}
