package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/tesseracode/tesserapatch/internal/intent"
	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/intentpub"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func prepareS4Workspace(t *testing.T, title string) (string, string) {
	t.Helper()
	root := t.TempDir()
	git := exec.Command("git", "init", "-q")
	git.Dir = root
	git.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+filepath.Join(root, "missing-global"),
		"GIT_CONFIG_SYSTEM="+filepath.Join(root, "missing-system"),
	)
	if output, err := git.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	if code, _, stderr, _ := runPrepare(t, "--path", root, "init"); code != 0 {
		t.Fatalf("tpatch init: %s", stderr)
	}
	if code, _, stderr, _ := runPrepare(t, "--path", root, "add", title); code != 0 {
		t.Fatalf("tpatch add: %s", stderr)
	}
	return root, storeSlug(title)
}

func storeSlug(title string) string {
	return strings.Trim(strings.ReplaceAll(strings.ToLower(title), " ", "-"), "-")
}

func TestPrepareS4GenerateManualDryRunAndYesPreflight(t *testing.T) {
	t.Run("generate", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S4 generate")
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--json", "--quiet",
		)
		if code != 0 || stderr != "" {
			t.Fatalf("prepare generate = %d stderr=%q\n%s", code, stderr, stdout)
		}
		var report preparePublishReport
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatal(err)
		}
		if report.Command != "prepare" || report.Mode != prepareModeGenerate ||
			report.Outcome != "published" || report.Action != "complete" {
			t.Fatalf("report = %#v", report)
		}
		for _, rel := range []string{
			"analysis.md", "spec.md", "exploration.md", "artifacts/analysis.json",
		} {
			if _, err := os.Stat(filepath.Join(root, ".tpatch", "features", slug, filepath.FromSlash(rel))); err != nil {
				t.Fatalf("%s: %v", rel, err)
			}
		}
	})

	t.Run("manual", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S4 manual")
		feature := filepath.Join(root, ".tpatch", "features", slug)
		for name, content := range map[string]string{
			"analysis.md":    "analysis\n",
			"spec.md":        "spec\n",
			"exploration.md": "exploration\n",
		} {
			if err := os.WriteFile(filepath.Join(feature, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		if code != 0 || stderr != "" {
			t.Fatalf("prepare manual = %d stderr=%q\n%s", code, stderr, stdout)
		}
		var report preparePublishReport
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatal(err)
		}
		if report.Mode != prepareModeManual || report.Outcome != "published" || report.Action != "adopt" {
			t.Fatalf("report = %#v", report)
		}
	})

	t.Run("dry-run", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S4 dry run")
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--dry-run", "--json", "--quiet",
		)
		if code != 0 || stderr != "" {
			t.Fatalf("prepare dry-run = %d stderr=%q\n%s", code, stderr, stdout)
		}
		var report preparePublishReport
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatal(err)
		}
		if report.Outcome != "planned" || !report.DryRun ||
			report.ExecutionPreflight != "not_evaluated" || report.PlanNote != preparePlanOnly {
			t.Fatalf("report = %#v", report)
		}
		after := readTree(t, filepath.Join(root, ".tpatch"))
		if !bytes.Equal(before, after) {
			t.Fatal("dry-run changed the workspace")
		}
	})

	t.Run("yes-preflight", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S4 yes")
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, stderr, _ := runPrepare(t, "--path", root, "prepare", slug, "--yes")
		if code != 1 || stdout != "" ||
			stderr != "error: prepare: --yes is only valid with --abandon-transaction\n" {
			t.Fatalf("yes preflight = %d stdout=%q stderr=%q", code, stdout, stderr)
		}
		after := readTree(t, filepath.Join(root, ".tpatch"))
		if !bytes.Equal(before, after) {
			t.Fatal("--yes preflight changed the workspace")
		}
	})
}

func TestPrepareS4AbandonPreviewAndMove(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S4 abandon")
	lane := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug)
	if err := os.MkdirAll(filepath.Join(lane, "stage-0123456789ab"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lane, "journal.json"), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := readTree(t, filepath.Join(root, ".tpatch"))
	code, _, _, _ := runPrepare(t, "--path", root, "prepare", slug, "--abandon-transaction")
	if code != 0 {
		t.Fatalf("preview exit = %d", code)
	}
	afterPreview := readTree(t, filepath.Join(root, ".tpatch"))
	if !bytes.Equal(before, afterPreview) {
		t.Fatal("abandon preview changed the workspace")
	}

	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--abandon-transaction", "--yes", "--json", "--quiet",
	)
	if code != 0 || stderr != "" {
		t.Fatalf("abandon = %d stderr=%q\n%s", code, stderr, stdout)
	}
	var report preparePublishReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if report.Outcome != "abandoned" || report.Abandoned == nil ||
		!strings.Contains(report.Abandoned.Directory, "abandoned-") {
		t.Fatalf("report = %#v", report)
	}
	if _, err := os.Stat(filepath.Join(lane, "journal.json")); !os.IsNotExist(err) {
		t.Fatalf("journal remains at canonical lane: %v", err)
	}
}

func TestPrepareS4RegenerateArchivesPriorBytes(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S4 regenerate")
	if code, _, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--json", "--quiet",
	); code != 0 {
		t.Fatalf("initial prepare: %s", stderr)
	}
	specPath := filepath.Join(root, ".tpatch", "features", slug, "spec.md")
	prior := []byte("hand-authored specification\n")
	if err := os.WriteFile(specPath, prior, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(specPath, 0o640); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--regenerate", "--allow-heuristic", "--json", "--quiet",
	)
	if code != 0 || stderr != "" {
		t.Fatalf("regenerate = %d stderr=%q\n%s", code, stderr, stdout)
	}
	var report preparePublishReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if report.Outcome != "published" || report.Archive == nil {
		t.Fatalf("report = %#v", report)
	}
	if len(report.OrphanBlobs) != 0 {
		t.Fatalf("published archive reported referenced blobs as orphans: %#v", report.OrphanBlobs)
	}
	sum := sha256.Sum256(prior)
	hash := hex.EncodeToString(sum[:])
	blob := filepath.Join(
		root, ".tpatch", "features", slug, "artifacts", "intent-archive", "blobs", hash+".blob",
	)
	got, err := os.ReadFile(blob)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, prior) {
		t.Fatalf("blob = %q, want %q", got, prior)
	}
	info, err := os.Stat(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("replacement mode = %o, want 0640", info.Mode().Perm())
	}
}

func prepareS4WriteReadyBundle(t *testing.T, root, slug string, defined bool) {
	t.Helper()
	feature := filepath.Join(root, ".tpatch", "features", slug)
	for name, content := range map[string]string{
		"analysis.md":    "hand analysis\n",
		"spec.md":        "hand specification\n",
		"exploration.md": "hand exploration\n",
	} {
		if err := os.WriteFile(filepath.Join(feature, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !defined {
		return
	}
	statusPath := filepath.Join(feature, "status.json")
	raw, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	var status store.FeatureStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	status.State = store.StateDefined
	status.LastCommand = "prepare"
	status.Notes = "Intent bundle prepared (prepare); generated: "
	raw, err = json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(statusPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func prepareS4Report(t *testing.T, stdout string) preparePublishReport {
	t.Helper()
	var report preparePublishReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout)
	}
	return report
}

func TestPrepareS4ExactFlagGrammarAndFalseValues(t *testing.T) {
	cmd := prepareCmd()
	var names []string
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		names = append(names, flag.Name)
	})
	sort.Strings(names)
	want := []string{
		"abandon-transaction", "allow-heuristic", "check", "dry-run", "json",
		"manual", "no-retry", "quiet", "regenerate", "timeout", "timeout-phase", "yes",
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("flags = %#v, want %#v", names, want)
	}
	if cmd.Flags().Lookup("mode") != nil {
		t.Fatal("prepare registered forbidden --mode")
	}

	root, slug := prepareS4Workspace(t, "S4 false values")
	prepareS4WriteReadyBundle(t, root, slug, true)
	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--abandon-transaction=false", "--json", "--quiet",
	)
	if code != 0 || stderr != "" {
		t.Fatalf("false abandon = %d stderr=%q\n%s", code, stderr, stdout)
	}
	report := prepareS4Report(t, stdout)
	if report.Mode != prepareModeGenerate || report.Outcome != "no-op" {
		t.Fatalf("false abandon selected wrong mode: %#v", report)
	}

	code, stdout, stderr, _ = runPrepare(
		t, "--path", root, "prepare", slug, "--check", "--abandon-transaction=false",
	)
	if code != 1 || stdout != "" ||
		!strings.Contains(stderr, "flags in the group") ||
		!strings.Contains(stderr, "[abandon-transaction check]") {
		t.Fatalf("presence mutex = %d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestPrepareS4DryRunSkipsProviderGitAuthorityAndWrites(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S4 dry zero effects")
	before := readTree(t, filepath.Join(root, ".tpatch"))

	oldAcquire := prepareAcquireAuthority
	oldLoad := prepareLoadProvider
	oldConfig := prepareLoadProviderConfig
	t.Cleanup(func() {
		prepareAcquireAuthority = oldAcquire
		prepareLoadProvider = oldLoad
		prepareLoadProviderConfig = oldConfig
	})
	acquires, providerLoads, configLoads := 0, 0, 0
	prepareAcquireAuthority = func(repoRoot string) (*intentlock.WorkspaceAuthority, error) {
		acquires++
		return oldAcquire(repoRoot)
	}
	prepareLoadProvider = func(*store.Store) (provider.Provider, provider.Config) {
		providerLoads++
		return nil, provider.Config{}
	}
	prepareLoadProviderConfig = func(*store.Store) provider.Config {
		configLoads++
		return provider.Config{}
	}

	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--regenerate", "--allow-heuristic",
		"--dry-run", "--json", "--quiet",
	)
	if code != 0 || stderr != "" {
		t.Fatalf("dry regenerate = %d stderr=%q\n%s", code, stderr, stdout)
	}
	report := prepareS4Report(t, stdout)
	if !report.DryRun || report.Outcome != "planned" ||
		report.ExecutionPreflight != "not_evaluated" || report.Actions == nil {
		t.Fatalf("dry report = %#v", report)
	}
	if acquires != 0 || providerLoads != 0 || configLoads != 0 {
		t.Fatalf("dry-run effects: authority=%d provider=%d config=%d", acquires, providerLoads, configLoads)
	}
	after := readTree(t, filepath.Join(root, ".tpatch"))
	if !bytes.Equal(before, after) {
		t.Fatal("dry-run changed the workspace")
	}
}

func TestPrepareS4ArchiveDecodeAndGlobalDanglingRefuseNoOp(t *testing.T) {
	t.Run("decode-error", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S4 archive decode")
		prepareS4WriteReadyBundle(t, root, slug, true)
		stale := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug, "stage-aaaaaaaaaaaa")
		if err := os.MkdirAll(stale, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stale, "old"), []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
		archive := filepath.Join(root, ".tpatch", "features", slug, "artifacts", "intent-archive")
		if err := os.MkdirAll(archive, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(archive, "index.json"), []byte("{broken\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 3 || report.Refusal == nil || report.Refusal.Code != "archive-index-corrupt" {
			t.Fatalf("decode refusal = %d %#v", code, report)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("archive decode refusal changed the workspace")
		}
	})

	t.Run("dangling-global-x11", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S4 dangling")
		prepareS4WriteReadyBundle(t, root, slug, true)
		prior := []byte("missing archived bytes\n")
		sum := sha256.Sum256(prior)
		hash := hex.EncodeToString(sum[:])
		replacements := []store.IntentArchiveReplacement{{
			ArtifactID:    store.IntentArchiveArtifactSpec,
			Path:          "spec.md",
			ContentSHA256: hash,
			Blob:          hash,
			SizeBytes:     int64(len(prior)),
		}}
		generationID, _, err := store.ComputeIntentArchiveGenerationID(slug, replacements)
		if err != nil {
			t.Fatal(err)
		}
		index := store.IntentArchiveIndex{
			SchemaVersion: store.IntentArchiveSchemaVersion,
			Feature:       slug,
			Generations: []store.IntentArchiveGeneration{{
				GenerationID: generationID,
				Mode:         store.IntentArchiveModeRegenerate,
				Replaced:     replacements,
			}},
		}
		encoded, err := store.EncodeIntentArchiveIndex(index)
		if err != nil {
			t.Fatal(err)
		}
		archive := filepath.Join(root, ".tpatch", "features", slug, "artifacts", "intent-archive")
		if err := os.MkdirAll(archive, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(archive, "index.json"), encoded, 0o644); err != nil {
			t.Fatal(err)
		}
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 3 || report.Refusal == nil ||
			report.Refusal.Code != "archive-blob-dangling" ||
			report.Refusal.Retry != "tpatch feature intent-archive purge "+slug+" --blob "+hash+" --yes" ||
			report.Refusal.RetryCWD != "workspace-root" {
			t.Fatalf("dangling refusal = %d %#v", code, report)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("global X11 refusal changed the workspace")
		}
	})
}

func TestPrepareS4ExitThreeAndNoOpPreserveWholeTree(t *testing.T) {
	t.Run("request-refusal-does-not-clean-stale-stage", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S4 request zero write")
		if err := os.Remove(filepath.Join(root, ".tpatch", "features", slug, "request.md")); err != nil {
			t.Fatal(err)
		}
		stage := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug, "stage-aaaaaaaaaaaa")
		if err := os.MkdirAll(stage, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stage, "old"), []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 3 || report.Refusal == nil || report.Refusal.Code != "request-unreadable" {
			t.Fatalf("request refusal = %d %#v", code, report)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("request refusal cleaned or staged workspace bytes")
		}
	})

	t.Run("redaction-refusal-does-not-clean-or-stage", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S4 redaction zero write")
		prepareS4WriteReadyBundle(t, root, slug, false)
		spec := filepath.Join(root, ".tpatch", "features", slug, "spec.md")
		if err := os.WriteFile(
			spec,
			[]byte("Authorization: Bearer "+strings.Repeat("x", 24)+"\n"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}
		stage := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug, "stage-bbbbbbbbbbbb")
		if err := os.MkdirAll(stage, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stage, "old"), []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--regenerate", "--allow-heuristic", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 3 || report.Refusal == nil ||
			report.Refusal.Code != "archive-content-refused-sensitive" {
			t.Fatalf("redaction refusal = %d %#v", code, report)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("redaction refusal cleaned or staged workspace bytes")
		}
	})

	t.Run("no-op-does-not-clean-stale-stage", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S4 noop zero write")
		prepareS4WriteReadyBundle(t, root, slug, true)
		stage := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug, "stage-cccccccccccc")
		if err := os.MkdirAll(stage, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stage, "old"), []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
		before := readTree(t, filepath.Join(root, ".tpatch"))
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 0 || report.Outcome != "no-op" {
			t.Fatalf("no-op = %d %#v", code, report)
		}
		if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
			t.Fatal("no-op cleaned stale staging evidence")
		}
	})
}

func TestPrepareS4ReinspectsArtifactsUnderAuthority(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S4 authoritative inspection")
	prepareS4WriteReadyBundle(t, root, slug, false)
	specPath := filepath.Join(root, ".tpatch", "features", slug, "spec.md")
	statusPath := filepath.Join(root, ".tpatch", "features", slug, "status.json")

	oldAcquire := prepareAcquireAuthority
	t.Cleanup(func() { prepareAcquireAuthority = oldAcquire })
	prepareAcquireAuthority = func(repoRoot string) (*intentlock.WorkspaceAuthority, error) {
		authority, err := oldAcquire(repoRoot)
		if err == nil {
			if removeErr := os.Remove(specPath); removeErr != nil {
				t.Fatal(removeErr)
			}
		}
		return authority, err
	}

	code, stdout, _, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
	)
	report := prepareS4Report(t, stdout)
	if code != 2 || report.Refusal == nil || report.Refusal.Code != "not-ready" {
		t.Fatalf("authoritative inspection = %d %#v", code, report)
	}
	raw, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	var status store.FeatureStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	if status.State == store.StateDefined {
		t.Fatalf("status advanced over a missing artifact: %#v", status)
	}
}

func TestPrepareS4StagesBeforeRevalidationAndPublishesStatusLast(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S4 transaction order")
	if code, _, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--json", "--quiet",
	); code != 0 {
		t.Fatalf("initial prepare: %s", stderr)
	}

	oldHook := prepareIntentpubHook
	t.Cleanup(func() { prepareIntentpubHook = oldHook })
	var order []intentpub.ArtifactID
	prepareIntentpubHook = func(point intentpub.CrashPoint, _ *os.Root, entry *intentpub.Entry) error {
		if point == intentpub.PointBeforeEntryCAS && entry != nil {
			order = append(order, entry.ArtifactID)
		}
		return nil
	}
	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--regenerate", "--allow-heuristic",
		"--json", "--quiet",
	)
	if code != 0 || stderr != "" {
		t.Fatalf("regenerate = %d stderr=%q\n%s", code, stderr, stdout)
	}
	want := []intentpub.ArtifactID{
		intentpub.ArtifactAnalysis,
		intentpub.ArtifactSpec,
		intentpub.ArtifactExploration,
		intentpub.ArtifactAnalysisSidecar,
		intentpub.ArtifactArchiveIndex,
		intentpub.ArtifactStatus,
	}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("publication order = %#v, want %#v", order, want)
	}
	if matches, err := filepath.Glob(filepath.Join(
		root, ".tpatch", "local", "intent-prepare", slug, "stage-*",
	)); err != nil || len(matches) != 0 {
		t.Fatalf("successful staging residue = %#v, err=%v", matches, err)
	}
}

func TestPrepareS4RevalidationFailureRetainsStageAndConcurrentBytes(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S4 revalidation")
	feature := filepath.Join(root, ".tpatch", "features", slug)
	specPath := filepath.Join(feature, "spec.md")
	concurrent := []byte("concurrent specification\n")

	oldHook := beforePrepareSetRevalidation
	t.Cleanup(func() { beforePrepareSetRevalidation = oldHook })
	beforePrepareSetRevalidation = func() {
		if err := os.WriteFile(specPath, concurrent, 0o640); err != nil {
			t.Fatal(err)
		}
	}
	code, stdout, _, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--json", "--quiet",
	)
	report := prepareS4Report(t, stdout)
	if code != 5 || report.Refusal == nil || report.Refusal.Code != "entry-changed" ||
		report.Outcome != "rolled-back" {
		t.Fatalf("revalidation = %d %#v", code, report)
	}
	got, err := os.ReadFile(specPath)
	if err != nil || !bytes.Equal(got, concurrent) {
		t.Fatalf("concurrent bytes lost: %v %q", err, got)
	}
	if _, err := os.Stat(filepath.Join(feature, "analysis.md")); !os.IsNotExist(err) {
		t.Fatalf("canonical analysis was published: %v", err)
	}
	var retained string
	for _, advisory := range report.Advisories {
		if advisory.Code == "staging-retained" {
			retained = strings.TrimSuffix(
				strings.TrimPrefix(advisory.Message, "Staged canonical output was retained at "),
				"; the next successful run removes it.",
			)
		}
	}
	if retained == "" || strings.HasPrefix(retained, root) {
		t.Fatalf("missing repo-relative retained stage: %#v", report.Advisories)
	}
	if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(retained))); err != nil || !info.IsDir() {
		t.Fatalf("retained stage missing: %v", err)
	}
}

func TestPrepareS4PostStageArchiveChangeIsExitFive(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S4 archive race")
	prepareS4WriteReadyBundle(t, root, slug, false)
	prior := []byte("hand specification\n")
	sum := sha256.Sum256(prior)
	hash := hex.EncodeToString(sum[:])
	blob := filepath.Join(
		root, ".tpatch", "features", slug, "artifacts", "intent-archive", "blobs", hash+".blob",
	)

	oldHook := beforePrepareSetRevalidation
	t.Cleanup(func() { beforePrepareSetRevalidation = oldHook })
	beforePrepareSetRevalidation = func() {
		if err := os.MkdirAll(filepath.Dir(blob), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(blob, []byte("concurrent wrong bytes\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	code, stdout, _, _ := runPrepare(
		t, "--path", root, "prepare", slug,
		"--regenerate", "--allow-heuristic", "--json", "--quiet",
	)
	report := prepareS4Report(t, stdout)
	if code != 5 || report.Outcome != "rolled-back" ||
		report.Refusal == nil || report.Refusal.Code != "entry-changed" {
		t.Fatalf("post-stage archive race = %d %#v", code, report)
	}
	got, err := os.ReadFile(blob)
	if err != nil || string(got) != "concurrent wrong bytes\n" {
		t.Fatalf("concurrent archive bytes changed: %v %q", err, got)
	}
}

func TestPrepareS4TerminalJournalRecoveryAndStaleStageCleanup(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S4 terminal recovery")
	lane := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug)
	stale := filepath.Join(lane, "stage-aaaaaaaaaaaa")
	if err := os.MkdirAll(stale, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "old"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	oldHook := prepareIntentpubHook
	oldLoad := prepareLoadProvider
	t.Cleanup(func() {
		prepareIntentpubHook = oldHook
		prepareLoadProvider = oldLoad
	})
	prepareIntentpubHook = func(point intentpub.CrashPoint, _ *os.Root, _ *intentpub.Entry) error {
		if point == intentpub.PointAfterJournalDurable {
			return errors.New("stop after journal")
		}
		return nil
	}
	code, stdout, _, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--json", "--quiet",
	)
	report := prepareS4Report(t, stdout)
	if code != 6 || report.Outcome != "recovery-refused" {
		t.Fatalf("injected interruption = %d %#v", code, report)
	}
	if _, err := os.Stat(filepath.Join(lane, "journal.json")); err != nil {
		t.Fatalf("journal not retained: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale stage was not cleaned before new staging: %v", err)
	}

	prepareIntentpubHook = nil
	providerLoads := 0
	prepareLoadProvider = func(*store.Store) (provider.Provider, provider.Config) {
		providerLoads++
		return nil, provider.Config{}
	}
	code, stdout, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--json", "--quiet",
	)
	report = prepareS4Report(t, stdout)
	recoveredAdvisory := false
	for _, advisory := range report.Advisories {
		recoveredAdvisory = recoveredAdvisory || advisory.Code == "recovered-prior-transaction"
	}
	if code != 0 || stderr != "" || report.Outcome != "recovered" ||
		report.Action != "none" || report.Recovery == nil ||
		report.Recovery.Kind != "journal-undo" ||
		report.Recovery.Retry != "tpatch prepare "+slug+" --json --quiet" ||
		!recoveredAdvisory {
		t.Fatalf("terminal recovery = %d stderr=%q %#v", code, stderr, report)
	}
	if providerLoads != 0 {
		t.Fatalf("recovery continued into provider loading: %d", providerLoads)
	}
}

func TestPrepareS4DefaultSuffixSidecarAndManualStatusCAS(t *testing.T) {
	t.Run("legacy-missing-artifacts-directory", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S4 legacy artifacts")
		artifacts := filepath.Join(root, ".tpatch", "features", slug, "artifacts")
		if err := os.RemoveAll(artifacts); err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--json", "--quiet",
		)
		if code != 0 || stderr != "" {
			t.Fatalf("legacy artifacts = %d stderr=%q\n%s", code, stderr, stdout)
		}
		info, err := os.Stat(artifacts)
		if err != nil || !info.IsDir() || info.Mode().Perm() != 0o755 {
			t.Fatalf("artifacts directory = %v %#v", err, info)
		}
	})

	t.Run("suffix-preserves-sidecar-absence", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S4 suffix")
		feature := filepath.Join(root, ".tpatch", "features", slug)
		analysis := []byte("preserved analysis\n")
		if err := os.WriteFile(filepath.Join(feature, "analysis.md"), analysis, 0o640); err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--json", "--quiet",
		)
		if code != 0 || stderr != "" {
			t.Fatalf("suffix = %d stderr=%q\n%s", code, stderr, stdout)
		}
		report := prepareS4Report(t, stdout)
		if report.Action != "complete" {
			t.Fatalf("suffix report = %#v", report)
		}
		got, err := os.ReadFile(filepath.Join(feature, "analysis.md"))
		if err != nil || !bytes.Equal(got, analysis) {
			t.Fatalf("analysis changed: %v %q", err, got)
		}
		if _, err := os.Stat(filepath.Join(feature, "artifacts", "analysis.json")); !os.IsNotExist(err) {
			t.Fatalf("sidecar was synthesized: %v", err)
		}
		found := false
		for _, advisory := range report.Advisories {
			found = found || advisory.Code == "analysis-preserved-sidecar-untouched"
		}
		if !found {
			t.Fatalf("missing sidecar advisory: %#v", report.Advisories)
		}
		statusRaw, err := os.ReadFile(filepath.Join(feature, "status.json"))
		if err != nil {
			t.Fatal(err)
		}
		var status store.FeatureStatus
		if err := json.Unmarshal(statusRaw, &status); err != nil {
			t.Fatal(err)
		}
		if status.State != store.StateDefined || status.LastCommand != "prepare" ||
			status.Notes != "Intent bundle prepared (prepare); generated: spec.md, exploration.md" {
			t.Fatalf("status metadata = %#v", status)
		}
	})

	t.Run("manual-status-cas", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "S4 manual cas")
		prepareS4WriteReadyBundle(t, root, slug, false)
		statusPath := filepath.Join(root, ".tpatch", "features", slug, "status.json")
		concurrent := []byte("{\"concurrent\":true}\n")
		oldHook := beforeManualStatusCAS
		t.Cleanup(func() { beforeManualStatusCAS = oldHook })
		beforeManualStatusCAS = func() {
			if err := os.WriteFile(statusPath, concurrent, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		code, stdout, _, _ := runPrepare(
			t, "--path", root, "prepare", slug, "--manual", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 5 || report.Refusal == nil || report.Refusal.Code != "status-changed" {
			t.Fatalf("status CAS = %d %#v", code, report)
		}
		got, err := os.ReadFile(statusPath)
		if err != nil || !bytes.Equal(got, concurrent) {
			t.Fatalf("concurrent status lost: %v %q", err, got)
		}
	})
}

func TestPrepareS4ProviderGateReleasesAuthority(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S4 provider gate")
	prepareS4WriteReadyBundle(t, root, slug, false)
	before := readTree(t, filepath.Join(root, ".tpatch"))

	oldLoad := prepareLoadProvider
	t.Cleanup(func() { prepareLoadProvider = oldLoad })
	loads := 0
	prepareLoadProvider = func(*store.Store) (provider.Provider, provider.Config) {
		loads++
		return nil, provider.Config{}
	}
	code, stdout, _, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--regenerate", "--json", "--quiet",
	)
	report := prepareS4Report(t, stdout)
	if code != 3 || report.Refusal == nil ||
		report.Refusal.Code != "provider-required-for-regenerate" || loads != 1 {
		t.Fatalf("provider gate = %d loads=%d %#v", code, loads, report)
	}
	if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
		t.Fatal("provider gate changed the workspace")
	}
	authority, err := intentlock.Acquire(root)
	if err != nil {
		t.Fatalf("prepare leaked authority: %v", err)
	}
	if err := authority.Release(); err != nil {
		t.Fatalf("release verification authority: %v", err)
	}
}

type prepareS4DeadlineProvider struct {
	calls int
}

func (*prepareS4DeadlineProvider) Check(context.Context, provider.Config) (*provider.Health, error) {
	return &provider.Health{}, nil
}

func (p *prepareS4DeadlineProvider) Generate(
	ctx context.Context,
	_ provider.Config,
	_ provider.GenerateRequest,
) (string, error) {
	p.calls++
	if p.calls == 1 {
		return `{"summary":"provider analysis"}`, nil
	}
	<-ctx.Done()
	return "", ctx.Err()
}

type prepareS4InvalidOutputProvider struct {
	calls int
}

func (*prepareS4InvalidOutputProvider) Check(context.Context, provider.Config) (*provider.Health, error) {
	return &provider.Health{}, nil
}

func (p *prepareS4InvalidOutputProvider) Generate(
	context.Context,
	provider.Config,
	provider.GenerateRequest,
) (string, error) {
	p.calls++
	if p.calls == 1 {
		return `{"summary":"provider analysis"}`, nil
	}
	return "invalid\x00specification", nil
}

func TestPrepareS4StagedValidationRefusalIsZeroWrite(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S4 staged validation")
	prepareS4WriteReadyBundle(t, root, slug, false)
	stage := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug, "stage-dddddddddddd")
	if err := os.MkdirAll(stage, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "old"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	oldLoad := prepareLoadProvider
	t.Cleanup(func() { prepareLoadProvider = oldLoad })
	invalid := &prepareS4InvalidOutputProvider{}
	prepareLoadProvider = func(*store.Store) (provider.Provider, provider.Config) {
		return invalid, provider.Config{
			Type:    "openai-compatible",
			BaseURL: "https://provider.invalid",
			Model:   "test-model",
		}
	}

	before := readTree(t, filepath.Join(root, ".tpatch"))
	code, stdout, _, _ := runPrepare(
		t, "--path", root, "prepare", slug,
		"--regenerate", "--no-retry", "--json", "--quiet",
	)
	report := prepareS4Report(t, stdout)
	if code != 2 || report.Refusal == nil || report.Refusal.Code != "staged-output-invalid" {
		t.Fatalf("staged validation = %d calls=%d %#v", code, invalid.calls, report)
	}
	if !bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch"))) {
		t.Fatal("staged-output refusal cleaned or staged workspace bytes")
	}
}

type prepareS4StageSyncFaultState struct {
	targetSuffix string
	renamed      bool
	failed       bool
}

type prepareS4StageSyncFaultOps struct {
	intentpub.RootOps
	state *prepareS4StageSyncFaultState
}

func (ops *prepareS4StageSyncFaultOps) Rename(oldName, newName string) error {
	err := ops.RootOps.Rename(oldName, newName)
	if err == nil && strings.Contains(newName, "/stage-") &&
		strings.HasSuffix(newName, "/"+ops.state.targetSuffix) {
		ops.state.renamed = true
	}
	return err
}

func (ops *prepareS4StageSyncFaultOps) Open(name string) (intentpub.RootFile, error) {
	file, err := ops.RootOps.Open(name)
	if err != nil {
		return nil, err
	}
	return &prepareS4StageSyncFaultFile{
		RootFile: file,
		state:    ops.state,
	}, nil
}

type prepareS4StageSyncFaultFile struct {
	intentpub.RootFile
	state *prepareS4StageSyncFaultState
}

func (file *prepareS4StageSyncFaultFile) Sync() error {
	if file.state.renamed && !file.state.failed {
		file.state.failed = true
		return errors.New("injected post-rename staging sync failure")
	}
	return file.RootFile.Sync()
}

func TestPrepareS4StagingPreservesExitSix(t *testing.T) {
	tests := []struct {
		name         string
		targetSuffix string
		ready        bool
		args         []string
	}{
		{
			name:         "base-artifact",
			targetSuffix: "analysis.md",
		},
		{
			name:         "archive-index",
			targetSuffix: "index.json",
			ready:        true,
			args:         []string{"--regenerate", "--allow-heuristic"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, slug := prepareS4Workspace(t, "S4 stage exit six "+test.name)
			if test.ready {
				prepareS4WriteReadyBundle(t, root, slug, false)
			}
			state := &prepareS4StageSyncFaultState{targetSuffix: test.targetSuffix}
			oldRootOps := prepareIntentpubRootOps
			t.Cleanup(func() { prepareIntentpubRootOps = oldRootOps })
			prepareIntentpubRootOps = func(rooted *os.Root) intentpub.RootOps {
				return &prepareS4StageSyncFaultOps{
					RootOps: intentpub.NewRootOps(rooted),
					state:   state,
				}
			}

			args := []string{"--path", root, "prepare", slug}
			args = append(args, test.args...)
			args = append(args, "--json", "--quiet")
			code, stdout, _, _ := runPrepare(t, args...)
			report := prepareS4Report(t, stdout)
			if code != 6 || report.Outcome != "recovery-refused" ||
				report.Refusal == nil ||
				report.Refusal.Code != "post-publication-divergence" ||
				!state.failed {
				t.Fatalf("staging failure = %d %#v state=%#v", code, report, state)
			}
			retained := false
			for _, advisory := range report.Advisories {
				retained = retained || advisory.Code == "staging-retained"
			}
			if !retained {
				t.Fatalf("staging failure omitted retained evidence: %#v", report.Advisories)
			}
		})
	}
}

func TestPrepareS4RegenerateDeadlineNamesScopeAndArtifact(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S4 deadline classification")
	prepareS4WriteReadyBundle(t, root, slug, false)

	oldLoad := prepareLoadProvider
	t.Cleanup(func() { prepareLoadProvider = oldLoad })
	blocking := &prepareS4DeadlineProvider{}
	prepareLoadProvider = func(*store.Store) (provider.Provider, provider.Config) {
		return blocking, provider.Config{
			Type:    "openai-compatible",
			BaseURL: "https://provider.invalid",
			Model:   "test-model",
		}
	}

	code, stdout, _, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--regenerate", "--no-retry",
		"--timeout=1s", "--timeout-phase=10ms", "--json", "--quiet",
	)
	report := prepareS4Report(t, stdout)
	if code != 5 || report.Refusal == nil ||
		report.Refusal.Code != "regenerate-generation-failed" ||
		!strings.Contains(report.Refusal.Message, "per-phase deadline") ||
		!strings.Contains(report.Refusal.Message, "artifact spec") {
		t.Fatalf("deadline refusal = %d calls=%d %#v", code, blocking.calls, report)
	}
}

func TestPrepareS4AbandonRollbackPreservesConcurrentEvidence(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S4 abandon rollback")
	lane := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug)
	stage := filepath.Join(lane, "stage-0123456789ab")
	if err := os.MkdirAll(stage, 0o700); err != nil {
		t.Fatal(err)
	}
	original := []byte("{broken")
	if err := os.WriteFile(filepath.Join(lane, "journal.json"), original, 0o600); err != nil {
		t.Fatal(err)
	}

	oldHook := beforeAbandonEvidenceRename
	t.Cleanup(func() { beforeAbandonEvidenceRename = oldHook })
	forward := 0
	concurrent := []byte("concurrent journal\n")
	beforeAbandonEvidenceRename = func(
		rooted *os.Root,
		_, target string,
		rollback bool,
	) error {
		if !rollback {
			forward++
			if forward == 2 {
				return errors.New("injected second move failure")
			}
			return nil
		}
		file, err := rooted.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		if _, err := file.Write(concurrent); err != nil {
			_ = file.Close()
			return err
		}
		return file.Close()
	}

	code, stdout, _, _ := runPrepare(
		t, "--path", root, "prepare", slug,
		"--abandon-transaction", "--yes", "--json", "--quiet",
	)
	report := prepareS4Report(t, stdout)
	if code != 6 || report.Outcome != "recovery-refused" ||
		report.Refusal == nil || report.Refusal.Code != "recovery-divergent" ||
		report.Abandoned == nil || len(report.Abandoned.Existing) != 1 {
		t.Fatalf("unsafe abandon rollback = %d %#v", code, report)
	}
	got, err := os.ReadFile(filepath.Join(lane, "journal.json"))
	if err != nil || !bytes.Equal(got, concurrent) {
		t.Fatalf("concurrent evidence was overwritten: %v %q", err, got)
	}
	preserved := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(report.Abandoned.Existing[0], "/")))
	got, err = os.ReadFile(filepath.Join(preserved, "journal.json"))
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("moved evidence was not preserved: %v %q", err, got)
	}
	if info, err := os.Stat(stage); err != nil || !info.IsDir() {
		t.Fatalf("unmoved stage evidence changed: %v", err)
	}
}

func TestPrepareS4HumanArchiveSectionsRenderOnce(t *testing.T) {
	report := newPreparePublishReport(prepareModeRegenerate, "feature", intent.FeatureStateDefined)
	report.Outcome = "rolled-back"
	report.Archive = &prepareArchiveReport{
		GenerationID: strings.Repeat("a", 64),
		BlobsDir:     ".tpatch/features/feature/artifacts/intent-archive/blobs",
	}
	report.Artifacts = []prepareArtifactReport{
		{ID: "analysis", Path: ".tpatch/features/feature/analysis.md", ArchivedBlob: strings.Repeat("b", 64)},
		{ID: "spec", Path: ".tpatch/features/feature/spec.md", ArchivedBlob: strings.Repeat("c", 64)},
	}
	report.OrphanBlobs = []string{strings.Repeat("d", 64), strings.Repeat("e", 64)}
	report.PurgeProgress = &preparePurgeProgressReport{
		State:           "partial",
		CompletedHashes: []string{},
		RemainingHashes: []string{strings.Repeat("f", 64)},
		Retry:           "tpatch feature intent-archive purge feature --orphans --yes",
	}
	var output bytes.Buffer
	writePreparePublishHuman(&output, report)
	if got := strings.Count(output.String(), "Orphan archive blobs:"); got != 1 {
		t.Fatalf("orphan section count = %d\n%s", got, output.String())
	}
	if got := strings.Count(output.String(), "Purge state:"); got != 1 {
		t.Fatalf("purge section count = %d\n%s", got, output.String())
	}
}

func TestPrepareS4RootedArchiveStorageTruthCASAndPhases(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S4 archive adapter")
	authority, err := intentlock.Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !authority.Released() {
			_ = authority.Release()
		}
	})
	storage := newPrepareArchiveStorage(authority, nil)
	data := []byte("archived bytes\n")
	hash := prepareSHA256(data)
	blobRel, err := store.IntentArchiveBlobRel(slug, hash)
	if err != nil {
		t.Fatal(err)
	}
	badHash := strings.Repeat("0", 64)
	if _, err := storage.PublishBlob(blobRel, badHash, data); err == nil {
		t.Fatal("content-address mismatch was accepted")
	}
	published, err := storage.PublishBlob(blobRel, hash, data)
	if err != nil || !published.Committed ||
		published.Phase != store.IntentArchiveStoragePhaseDirectorySynced {
		t.Fatalf("publish = %#v, %v", published, err)
	}
	reused, err := storage.PublishBlob(blobRel, hash, data)
	if err != nil || !reused.Reused || reused.Committed ||
		reused.Phase != store.IntentArchiveStoragePhaseValidated {
		t.Fatalf("reuse = %#v, %v", reused, err)
	}
	probe, err := storage.ProbeBlob(blobRel)
	if err != nil || probe.Kind != store.IntentArchiveBlobKindRegular ||
		probe.SHA256 != hash || probe.SizeBytes != int64(len(data)) {
		t.Fatalf("probe = %#v, %v", probe, err)
	}

	indexRel, err := store.IntentArchiveIndexRel(slug)
	if err != nil {
		t.Fatal(err)
	}
	indexBytes, err := store.EncodeIntentArchiveIndex(store.IntentArchiveIndex{
		SchemaVersion: store.IntentArchiveSchemaVersion,
		Feature:       slug,
		Generations:   []store.IntentArchiveGeneration{},
	})
	if err != nil {
		t.Fatal(err)
	}
	indexResult, err := storage.CASIndex(
		indexRel, prepareArchiveIdentityToken(intentpub.AbsentIdentity()), indexBytes,
	)
	if err != nil || !indexResult.Committed ||
		indexResult.Phase != store.IntentArchiveStoragePhaseDirectorySynced {
		t.Fatalf("index CAS = %#v, %v", indexResult, err)
	}
	if err := storage.PreflightIndexCAS(
		indexRel, prepareArchiveIdentityToken(intentpub.AbsentIdentity()),
	); err == nil {
		t.Fatal("stale index CAS token was accepted")
	}
	removed, err := storage.RemoveBlob(blobRel, probe.Identity)
	if err != nil || !removed.Committed ||
		removed.Phase != store.IntentArchiveStoragePhaseDirectorySynced {
		t.Fatalf("remove = %#v, %v", removed, err)
	}
	if err := authority.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareS4ReportRenderingAndAbandonSecondRun(t *testing.T) {
	root, slug := prepareS4Workspace(t, "S4 rendering")
	code, quiet, stderr, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--quiet",
	)
	wantQuiet := "prepare " + slug + ": generate published (4 artifacts, 0 archived)\n"
	if code != 0 || stderr != "" || quiet != wantQuiet {
		t.Fatalf("quiet = %d stdout=%q stderr=%q", code, quiet, stderr)
	}

	code, human, _, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--dry-run",
	)
	if code != 0 || !strings.Contains(human, "Planned actions:") ||
		strings.Contains(human, root) || !strings.Contains(human, preparePlanOnly) {
		t.Fatalf("human dry report leaked or omitted fields:\n%s", human)
	}

	feature := filepath.Join(root, ".tpatch", "features", slug)
	if err := os.RemoveAll(feature); err != nil {
		t.Fatal(err)
	}
	lane := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug)
	if err := os.MkdirAll(lane, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lane, "journal.json"), []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, _, _ := runPrepare(
		t, "--path", root, "prepare", slug, "--abandon-transaction", "--yes", "--json", "--quiet",
	)
	report := prepareS4Report(t, stdout)
	if code != 0 || report.Outcome != "abandoned" || report.Abandoned == nil ||
		strings.Contains(stdout, root) {
		t.Fatalf("abandon missing feature = %d %#v", code, report)
	}
	code, stdout, _, _ = runPrepare(
		t, "--path", root, "prepare", slug, "--abandon-transaction", "--json", "--quiet",
	)
	report = prepareS4Report(t, stdout)
	if code != 3 || report.Refusal == nil ||
		report.Refusal.Code != "no-pending-transaction" ||
		report.Abandoned == nil || len(report.Abandoned.Existing) != 1 {
		t.Fatalf("second abandon = %d %#v", code, report)
	}
}

func TestPrepareS4GitProcessCountsByMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-backed Git argv spy")
	}
	root, slug := prepareS4Workspace(t, "S4 git counts")
	prepareS4WriteReadyBundle(t, root, slug, false)
	bin := filepath.Join(root, "git-spy-bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "git-spy.log")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$PREPARE_GIT_LOG\"\n" +
		"if [ \"$1\" = \"rev-parse\" ]; then printf 'true\\n'; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("PREPARE_GIT_LOG", logPath)

	runAndRead := func(args ...string) []string {
		t.Helper()
		if err := os.WriteFile(logPath, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		_, _, _, _ = runPrepare(t, args...)
		raw, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.TrimSpace(string(raw))
		if text == "" {
			return []string{}
		}
		return strings.Split(text, "\n")
	}

	calls := runAndRead("--path", root, "prepare", slug, "--manual", "--json", "--quiet")
	wantManual := []string{
		"rev-parse --is-inside-work-tree",
		"check-ignore -q --no-index -- .tpatch/local/intent-prepare/" + slug,
		"--literal-pathspecs ls-files -- .tpatch/local/",
	}
	if !reflect.DeepEqual(calls, wantManual) {
		t.Fatalf("manual Git calls = %#v, want %#v", calls, wantManual)
	}
	for _, call := range calls {
		if strings.Contains(call, root) {
			t.Fatalf("absolute path in Git argv: %q", call)
		}
	}
	calls = runAndRead("--path", root, "prepare", slug, "--manual", "--dry-run", "--json", "--quiet")
	if len(calls) != 0 {
		t.Fatalf("dry-run Git calls = %#v", calls)
	}
	oldLoad := prepareLoadProvider
	t.Cleanup(func() { prepareLoadProvider = oldLoad })
	prepareLoadProvider = func(*store.Store) (provider.Provider, provider.Config) {
		return nil, provider.Config{}
	}
	calls = runAndRead("--path", root, "prepare", slug, "--regenerate", "--json", "--quiet")
	wantRegenerate := append(append([]string(nil), wantManual...), "ls-files -- .tpatch")
	if !reflect.DeepEqual(calls, wantRegenerate) {
		t.Fatalf("regenerate Git calls = %#v, want %#v", calls, wantRegenerate)
	}
	lane := filepath.Join(root, ".tpatch", "local", "intent-prepare", slug)
	if err := os.MkdirAll(filepath.Join(lane, "stage-bbbbbbbbbbbb"), 0o700); err != nil {
		t.Fatal(err)
	}
	calls = runAndRead("--path", root, "prepare", slug, "--abandon-transaction", "--json", "--quiet")
	if len(calls) != 0 {
		t.Fatalf("abandon Git calls = %#v", calls)
	}
}
