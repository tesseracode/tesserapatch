package workflow

import (
	"reflect"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestClassifyFileNoveltyAllNewFiles(t *testing.T) {
	repo := noveltyRepo(t)
	result, err := ClassifyFileNovelty(`diff --git a/a.txt b/a.txt
new file mode 100644
index 0000000..1111111
--- /dev/null
+++ b/a.txt
@@ -0,0 +1 @@
+hello
`, "HEAD", "HEAD", repo)
	if err != nil {
		t.Fatal(err)
	}
	if result.Classification != FileNoveltyAllNewFiles {
		t.Fatalf("expected all-new-files, got %+v", result)
	}
	if len(result.Paths) != 1 || result.Paths[0].FeatureAction != FileNoveltyActionCreate || result.Paths[0].UpstreamState != FileNoveltyUpstreamAbsent {
		t.Fatalf("unexpected path novelty: %+v", result.Paths)
	}
}

func TestClassifyFileNoveltyCreateModifyMix(t *testing.T) {
	repo := noveltyRepo(t)
	patch := `diff --git a/new.txt b/new.txt
new file mode 100644
index 0000000..1111111
--- /dev/null
+++ b/new.txt
@@ -0,0 +1 @@
+hello
diff --git a/README.md b/README.md
index 1111111..2222222 100644
--- a/README.md
+++ b/README.md
@@ -1 +1 @@
-old
+new
`
	result, err := ClassifyFileNovelty(patch, "HEAD", "HEAD", repo)
	if err != nil {
		t.Fatal(err)
	}
	if result.Classification != FileNoveltyMixedAdditive {
		t.Fatalf("expected mixed-additive, got %+v", result)
	}
}

func TestClassifyFileNoveltyModifyOnly(t *testing.T) {
	repo := noveltyRepo(t)
	patch := `diff --git a/README.md b/README.md
index 1111111..2222222 100644
--- a/README.md
+++ b/README.md
@@ -1 +1 @@
-old
+new
`
	result, err := ClassifyFileNovelty(patch, "HEAD", "HEAD", repo)
	if err != nil {
		t.Fatal(err)
	}
	if result.Classification != FileNoveltyModifiesExistingFiles {
		t.Fatalf("expected modifies-existing-files, got %+v", result)
	}
}

func TestClassifyFileNoveltyAnyRename(t *testing.T) {
	repo := noveltyRepo(t)
	patch := `diff --git a/README.md b/README-renamed.md
similarity index 100%
rename from README.md
rename to README-renamed.md
`
	result, err := ClassifyFileNovelty(patch, "HEAD", "HEAD", repo)
	if err != nil {
		t.Fatal(err)
	}
	if result.Classification != FileNoveltyDeletesOrRenames {
		t.Fatalf("expected deletes-or-renames, got %+v", result)
	}
	if len(result.Paths) != 1 || result.Paths[0].FeatureAction != FileNoveltyActionRename || result.Paths[0].Path != "README-renamed.md" {
		t.Fatalf("unexpected rename path: %+v", result.Paths)
	}
}

func TestClassifyFileNoveltyAnyDelete(t *testing.T) {
	repo := noveltyRepo(t)
	patch := `diff --git a/README.md b/README.md
deleted file mode 100644
index 1111111..0000000
--- a/README.md
+++ /dev/null
@@ -1 +0,0 @@
-old
`
	result, err := ClassifyFileNovelty(patch, "HEAD", "HEAD", repo)
	if err != nil {
		t.Fatal(err)
	}
	if result.Classification != FileNoveltyDeletesOrRenames {
		t.Fatalf("expected deletes-or-renames, got %+v", result)
	}
}

func TestClassifyFileNoveltyBinaryPatchUsesActionRules(t *testing.T) {
	repo := noveltyRepo(t)
	patch := `diff --git a/image.bin b/image.bin
new file mode 100644
index 0000000..d95f3ad
GIT binary patch
literal 3
KcmZQzzy8r+
`
	result, err := ClassifyFileNovelty(patch, "HEAD", "HEAD", repo)
	if err != nil {
		t.Fatal(err)
	}
	if result.Classification != FileNoveltyAllNewFiles {
		t.Fatalf("binary new-file patch should use action rules, got %+v", result)
	}
}

func TestClassifyFileNoveltyPathsSortedDeterministically(t *testing.T) {
	repo := noveltyRepo(t)
	patch := `diff --git a/z.txt b/z.txt
new file mode 100644
--- /dev/null
+++ b/z.txt
diff --git a/a.txt b/a.txt
new file mode 100644
--- /dev/null
+++ b/a.txt
`
	result, err := ClassifyFileNovelty(patch, "HEAD", "HEAD", repo)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{result.Paths[0].Path, result.Paths[1].Path}
	if !reflect.DeepEqual(got, []string{"a.txt", "z.txt"}) {
		t.Fatalf("paths not sorted: %+v", result.Paths)
	}
}

func TestFileNoveltyEvidenceHelper(t *testing.T) {
	result := FileNoveltyResult{
		Classification: FileNoveltyAllNewFiles,
		Paths:          []PathNovelty{{Path: "new.txt", FeatureAction: FileNoveltyActionCreate, UpstreamState: FileNoveltyUpstreamAbsent}},
	}
	entry := FileNoveltyEvidence("demo", "origin/main", "abc", "base", string(store.ReconcileBlocked), result)
	if entry.EvidenceKind != store.EvidenceKindFileNovelty || entry.ReasonCode != "all-new-files" || entry.Confidence != store.EvidenceConfidenceHigh || entry.PreReconcilePresence != store.EvidencePresenceAbsent {
		t.Fatalf("unexpected evidence entry: %+v", entry)
	}
	if entry.AttemptID == "" || len(entry.MatchedPaths) != 1 || entry.MatchedPaths[0] != "new.txt" {
		t.Fatalf("missing helper fields: %+v", entry)
	}
}

func noveltyRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	setupGitRepo(t, dir)
	return dir
}
