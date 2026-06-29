package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestReconcileBlockedCategoryHumanAndJSON(t *testing.T) {
	dir, slug, _ := cliEvidenceFixture(t, "blocked category", map[string]string{"new.txt": "brand new\n"})
	out, errOut, err := runCLIForEvidence("reconcile", "--path", dir, "--allow-dirty", "--upstream-ref", "HEAD", slug)
	if err != nil {
		t.Fatalf("reconcile failed: %v\nstderr=%s\nstdout=%s", err, errOut, out)
	}
	if !strings.Contains(out, slug+": blocked (clean-additive)") || !strings.Contains(out, "next: reapply-or-accept-deterministic-apply") {
		t.Fatalf("missing blocked category output:\n%s", out)
	}
	jsonOut, errOut, err := runCLIForEvidence("reconcile", "--path", dir, "--allow-dirty", "--upstream-ref", "HEAD", "--format", "json", slug)
	if err != nil {
		t.Fatalf("json reconcile failed: %v\nstderr=%s\nstdout=%s", err, errOut, jsonOut)
	}
	var results []map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &results); err != nil {
		t.Fatalf("bad json: %v\n%s", err, jsonOut)
	}
	if results[0]["outcome"] != "blocked" || results[0]["blocked_category"] != "clean-additive" || results[0]["recommended_action"] == "" {
		t.Fatalf("bad json %#v", results[0])
	}
}

func TestBlockedClassificationArtifactsDoNotLeakTitleOrRootPathSecret(t *testing.T) {
	secretTitle := "SECRET_TITLE_DO_NOT_LEAK"
	secretPath := "SECRET_ROOT_PATH_DO_NOT_LEAK"
	dir := filepath.Join(t.TempDir(), secretPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	gitInitTestRepo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("base\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "existing.txt")
	gitRun(t, dir, "commit", "-m", "base")
	baseCommit := gitHead(t, dir)
	s, _ := store.Init(dir)
	feature, _ := s.AddFeature(store.AddFeatureInput{Title: secretTitle, Slug: "privacy-slug", Request: secretTitle})
	_ = s.MarkFeatureState(feature.Slug, store.StateApplied, "apply", "")
	st, _ := s.LoadFeatureStatus(feature.Slug)
	st.Apply.BaseCommit = baseCommit
	st.Apply.HasPatch = true
	_ = s.SaveFeatureStatus(st)
	_ = s.WriteArtifact(feature.Slug, "post-apply.patch", "diff --git a/new.txt b/new.txt\nnew file mode 100644\n--- /dev/null\n+++ b/new.txt\n@@ -0,0 +1 @@\n+brand new\n")
	_, _, err := runCLIForEvidence("reconcile", "--path", dir, "--allow-dirty", "--upstream-ref", "HEAD", feature.Slug)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".tpatch", "features", feature.Slug, "artifacts", "reconcile-evidence.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), secretTitle) || strings.Contains(string(data), secretPath) {
		t.Fatalf("artifact leaked secret metadata:\n%s", data)
	}
}
