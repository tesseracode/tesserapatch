package store

import (
	"strings"
	"testing"
)

// TestWriteArtifact_RefusesGitInternalPatch is the D5 store-boundary
// regression for PRD-multi-slug-reconcile-canonical-safety / ADR-030.
//
// Any patch text whose diff/patch headers reference `.git/**` or the
// exact path `.git` must be refused at the WriteArtifact boundary
// with a descriptive error and no bytes on disk. INV-3/INV-6 of the
// PRD.
//
// The test asserts both refusal and the invariant that the artifact
// file is not created on rejection — the store must fail closed.
func TestWriteArtifact_RefusesGitInternalPatch(t *testing.T) {
	tmp := t.TempDir()
	s, err := Init(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(AddFeatureInput{Title: "T", Request: "R"}); err != nil {
		t.Fatal(err)
	}
	slug := "t"

	cases := []struct {
		name    string
		content string
	}{
		{
			name: "unified-git-log",
			content: `diff --git a/.git/logs/HEAD b/.git/logs/HEAD
--- a/.git/logs/HEAD
+++ b/.git/logs/HEAD
@@ -1 +1 @@
-old
+new
`,
		},
		{
			name: "plain-diff-git-index",
			content: `diff -ruN prev/.git/index curr/.git/index
Binary files prev/.git/index and curr/.git/index differ
`,
		},
		{
			name: "only-in-git",
			content: `Only in prev/.git: HEAD
`,
		},
		{
			name: "nested-git-in-subdir",
			content: `diff --git a/vendor/dep/.git/index b/vendor/dep/.git/index
--- a/vendor/dep/.git/index
+++ b/vendor/dep/.git/index
`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			name := "test-" + c.name + ".patch"
			err := s.WriteArtifact(slug, name, c.content)
			if err == nil {
				t.Fatalf("expected WriteArtifact to refuse %q, got nil error", name)
			}
			if !strings.Contains(err.Error(), "WriteArtifact refused") {
				t.Errorf("error message missing 'WriteArtifact refused' anchor: %v", err)
			}
			if !strings.Contains(err.Error(), ".git") {
				t.Errorf("error message missing '.git' path in diagnostic: %v", err)
			}
			// The refusal must be text-only — no artifact file on
			// disk. AC-7 second half: refused and no partial write.
			if _, statErr := s.ReadFeatureFile(slug, "artifacts/"+name); statErr == nil {
				t.Errorf("expected no artifact on disk after refusal, but read succeeded")
			}
		})
	}
}

// TestWriteArtifact_AcceptsCleanPatch is the negative — a real
// feature-scoped patch (no .git/** entries) must land on disk normally.
// Guards against overreach in the D5 guard: the store must accept
// legitimate canonical patches without regression.
func TestWriteArtifact_AcceptsCleanPatch(t *testing.T) {
	tmp := t.TempDir()
	s, err := Init(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(AddFeatureInput{Title: "T", Request: "R"}); err != nil {
		t.Fatal(err)
	}
	clean := `diff --git a/feature.txt b/feature.txt
new file mode 100644
--- /dev/null
+++ b/feature.txt
@@ -0,0 +1 @@
+hello
`
	if err := s.WriteArtifact("t", "post-apply.patch", clean); err != nil {
		t.Fatalf("clean patch was rejected: %v", err)
	}
	got, err := s.ReadFeatureFile("t", "artifacts/post-apply.patch")
	if err != nil {
		t.Fatal(err)
	}
	if got != clean {
		t.Errorf("bytes-out != bytes-in on clean write")
	}
}
