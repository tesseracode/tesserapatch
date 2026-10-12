package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

const rocS0Baseline = "bbb7a4310b2ebea095890a284997fbbb42535bc7"

var rocS0RuntimeCases = []string{
	"phase1-confirmed",
	"phase15-confirmed",
	"phase2-legacy-rejected",
	"phase2-conflict-fallthrough",
	"phase2-applicable-only",
	"phase2-mixed",
	"phase3-rejected",
	"phase35-shadow-awaiting",
	"phase4-strict",
	"phase4-threeway-clean",
	"phase4-conflicts",
	"multi-slug",
	"cumulative-legacy",
}

type rocS0RuntimeFixture struct {
	store    *store.Store
	slugs    []string
	provider provider.Provider
	config   provider.Config
	options  ReconcileOptions
	phase    string
	outcome  store.ReconcileOutcome
}

func rocS0PinGit(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME":     "TesseraPatch S0",
		"GIT_AUTHOR_EMAIL":    "s0@example.invalid",
		"GIT_COMMITTER_NAME":  "TesseraPatch S0",
		"GIT_COMMITTER_EMAIL": "s0@example.invalid",
		"GIT_AUTHOR_DATE":     "2000-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE":  "2000-01-01T00:00:00Z",
		"GIT_DEFAULT_HASH":    "sha1",
	} {
		t.Setenv(key, value)
	}
}

func rocS0FreshStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	setupGitRepo(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func rocS0WriteFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func rocS0Fixture(t *testing.T, name string) rocS0RuntimeFixture {
	t.Helper()
	var f rocS0RuntimeFixture
	switch name {
	case "phase1-confirmed":
		s, slug := buildReverseApplyConfirmationFixture(t)
		f.store, f.slugs = s, []string{slug}
		f.phase, f.outcome = "phase-1-reverse-apply", store.ReconcileUpstreamed
	case "phase15-confirmed":
		s := rocS0FreshStore(t)
		base := git(t, s.Root, "rev-parse", "HEAD")
		rocS0WriteFile(t, filepath.Join(s.Root, "greeting.txt"), "Hello World\n")
		gitAdd(t, s.Root, "greeting.txt")
		gitCommit(t, s.Root, "upstream absorbed greeting")
		if err := os.Remove(filepath.Join(s.Root, "greeting.txt")); err != nil {
			t.Fatal(err)
		}
		gitAdd(t, s.Root, "greeting.txt")
		gitCommit(t, s.Root, "upstream later removed greeting")
		cfg, err := s.LoadConfig()
		if err != nil {
			t.Fatal(err)
		}
		cfg.PatchIDDetectorEnabled = true
		if err := s.SaveConfig(cfg); err != nil {
			t.Fatal(err)
		}
		writeLock(t, s.Root, "origin", "main", base)
		installIndependentFeature(t, s, "greeting")
		if err := s.WriteArtifact("greeting", "post-apply.patch", canonicalIndependentPatch("greeting.txt", "Hello World")); err != nil {
			t.Fatal(err)
		}
		f.store, f.slugs = s, []string{"greeting"}
		f.phase, f.outcome = "phase-1.5-patch-id-match", store.ReconcileUpstreamed
	case "phase2-legacy-rejected", "phase3-rejected":
		s, slug := buildOperationUpstreamedCandidateFixture(t)
		f.store, f.slugs = s, []string{slug}
		f.phase, f.outcome = "phase-2-operation-level", store.ReconcileBlocked
		if name == "phase3-rejected" {
			if err := os.Remove(filepath.Join(s.Root, ".tpatch", "features", slug, "artifacts", "apply-recipe.json")); err != nil {
				t.Fatal(err)
			}
			f.provider = &scriptedProvider{responses: []string{`{"decision":"upstreamed","reasoning":"equivalent behavior"}`}}
			f.config = provider.Config{Type: "openai-compatible", BaseURL: "http://example.invalid", Model: "fixture"}
			f.phase = "phase-3-provider-semantic"
		}
	case "phase35-shadow-awaiting", "phase4-conflicts":
		s, slug := buildConflictFixture(t)
		f.store, f.slugs = s, []string{slug}
		f.phase, f.outcome = "phase-4-forward-apply-conflicts", store.ReconcileBlocked
		if name == "phase35-shadow-awaiting" {
			f.provider = &scriptedProvider{
				responses: []string{`{"decision":"unclear"}`},
				keyed:     map[string]string{"shared.txt": "a\nB-merged\nc\n"},
			}
			f.config = provider.Config{Type: "openai-compatible", BaseURL: "http://example.invalid", Model: "fixture"}
			f.options.Resolve = true
			f.phase, f.outcome = "phase-3.5-provider-resolve", store.ReconcileShadowAwaiting
		}
	case "phase4-threeway-clean":
		s := rocS0FreshStore(t)
		path := filepath.Join(s.Root, "shared.txt")
		rocS0WriteFile(t, path, "a\nb\nc\nd\ne\nf\ng\n")
		gitAdd(t, s.Root, "shared.txt")
		gitCommit(t, s.Root, "threeway base")
		rocS0WriteFile(t, path, "a\nB-local\nc\nd\ne\nf\ng\n")
		gitAdd(t, s.Root, "shared.txt")
		patch := gitOutput(t, s.Root, "diff", "--cached", "HEAD")
		gitCommit(t, s.Root, "feature changes first region")
		rocS0WriteFile(t, path, "a\nb\nc\nd\nE-upstream\nf\ng\n")
		gitAdd(t, s.Root, "shared.txt")
		gitCommit(t, s.Root, "upstream changes independent region")
		feature, err := s.AddFeature(store.AddFeatureInput{Title: "roc-threeway", Request: "freeze clean threeway"})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.MarkFeatureState(feature.Slug, store.StateApplied, "apply", ""); err != nil {
			t.Fatal(err)
		}
		if err := s.WriteArtifact(feature.Slug, "post-apply.patch", patch); err != nil {
			t.Fatal(err)
		}
		f.store, f.slugs = s, []string{feature.Slug}
		f.phase, f.outcome = "phase-4-forward-apply-3way", store.ReconcileReapplied
	case "phase4-strict", "phase2-conflict-fallthrough", "phase2-applicable-only", "phase2-mixed":
		s, slug := buildFileNoveltyFixture(t, "roc-source", map[string]string{"new.txt": "new\n"})
		f.store, f.slugs = s, []string{slug}
		f.phase, f.outcome = "phase-4-forward-apply-strict", store.ReconcileReapplied
		if name != "phase4-strict" {
			ops := []RecipeOperation{{Type: "write-file", Path: "operation.txt", Content: "applicable\n"}}
			if name != "phase2-applicable-only" {
				ops = append(ops, RecipeOperation{Type: "write-file", Path: "README.md", Content: "# Test\n"})
			}
			if name == "phase2-conflict-fallthrough" {
				ops = append(ops, RecipeOperation{Type: "replace-in-file", Path: "README.md", Search: "missing", Replace: "also missing"})
			}
			body, err := json.Marshal(ApplyRecipe{Feature: slug, Operations: ops})
			if err != nil {
				t.Fatal(err)
			}
			if err := s.WriteArtifact(slug, "apply-recipe.json", string(body)); err != nil {
				t.Fatal(err)
			}
		}
	case "multi-slug", "cumulative-legacy":
		f.store = rocS0FreshStore(t)
		f.slugs = []string{"aaa", "bbb"}
		for _, slug := range f.slugs {
			installIndependentFeature(t, f.store, slug)
		}
		f.options.CumulativeLegacy = name == "cumulative-legacy"
		if f.options.CumulativeLegacy {
			cfg, err := f.store.LoadConfig()
			if err != nil {
				t.Fatal(err)
			}
			cfg.PatchIDDetectorEnabled = true
			if err := f.store.SaveConfig(cfg); err != nil {
				t.Fatal(err)
			}
		}
	default:
		t.Fatalf("unregistered S0 runtime fixture %q", name)
	}
	return f
}

var rocS0AttemptedAt = regexp.MustCompile(`("attempted_at"\s*:\s*")\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z(")`)
var rocS0MarkdownTime = regexp.MustCompile(`(\*\*Timestamp\*\*: )\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z`)
var rocS0ResolveID = regexp.MustCompile(`^rec-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}Z-[0-9a-f]{12}$`)

func rocS0Normalize(body, root, shadow, resolveID string) string {
	for _, replacement := range [][2]string{{shadow, "<shadow>"}, {root, "<repo>"}, {resolveID, "<resolve-session>"}} {
		if replacement[0] == "" {
			continue
		}
		encoded, _ := json.Marshal(replacement[0])
		body = strings.ReplaceAll(body, string(encoded[1:len(encoded)-1]), replacement[1])
		body = strings.ReplaceAll(body, replacement[0], replacement[1])
	}
	body = rocS0AttemptedAt.ReplaceAllString(body, `${1}<wall-clock>${2}`)
	return rocS0MarkdownTime.ReplaceAllString(body, `${1}<wall-clock>`)
}

func rocS0ValidateShadowPath(root, slug, path string) error {
	if filepath.Dir(path) != filepath.Join(root, ".tpatch", "shadow") {
		return fmt.Errorf("shadow is outside the existing shadow namespace: %q", path)
	}
	name := filepath.Base(path)
	prefix := slug + "-"
	if !strings.HasPrefix(name, prefix) {
		return fmt.Errorf("shadow path does not name its feature: %q", path)
	}
	const layout = "2006-01-02T15-04-05.000000Z"
	stamp := strings.TrimPrefix(name, prefix)
	parsed, err := time.Parse(layout, stamp)
	if err != nil || parsed.Format(layout) != stamp {
		return fmt.Errorf("shadow timestamp shape changed: %q", path)
	}
	return nil
}

func rocS0Frame(out *strings.Builder, name, body string) {
	fmt.Fprintf(out, "--- %s (%d bytes) ---\n%s", name, len(body), body)
	if !strings.HasSuffix(body, "\n") {
		out.WriteByte('\n')
	}
}

func rocS0GoldenDifference(want, got []byte) error {
	if !bytes.Equal(want, got) {
		return fmt.Errorf("frozen S0 bytes differ:\nwant:\n%s\ngot:\n%s", want, got)
	}
	return nil
}

func rocS0CompareGolden(t *testing.T, name, body string) {
	t.Helper()
	path := filepath.Join("testdata", "roc-s0", name+".golden")
	switch os.Getenv("TPATCH_UPDATE_ROC_S0_GOLDENS") {
	case "1":
		rocS0AssertTestOnlyScope(t)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		rocS0WriteFile(t, path, body)
		t.Logf("captured %s from production baseline %s; normal comparison still required", path, rocS0Baseline)
		return
	case "":
	default:
		t.Fatal("TPATCH_UPDATE_ROC_S0_GOLDENS must be empty or 1")
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := rocS0GoldenDifference(want, []byte(body)); err != nil {
		t.Fatal(err)
	}
}

func TestROCS0RuntimeGoldens(t *testing.T) {
	for _, name := range rocS0RuntimeCases {
		t.Run(name, func(t *testing.T) {
			rocS0PinGit(t)
			f := rocS0Fixture(t, name)
			canonical := make(map[string]string)
			for _, slug := range f.slugs {
				body, err := f.store.ReadFeatureFile(slug, "artifacts/post-apply.patch")
				if err != nil {
					t.Fatal(err)
				}
				canonical[slug] = body
			}
			results, err := RunReconcile(context.Background(), f.store, f.slugs, "HEAD", f.provider, f.config, f.options)
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != len(f.slugs) {
				t.Fatalf("got %d results for %d features", len(results), len(f.slugs))
			}
			var transcript strings.Builder
			for i, result := range results {
				slug := f.slugs[i]
				if result.Slug != slug {
					t.Fatalf("result order changed: got %s, want %s", result.Slug, slug)
				}
				if result.ShadowPath != "" {
					t.Cleanup(func() {
						if err := gitutil.PruneShadow(f.store.Root, slug); err != nil {
							t.Errorf("prune fixture shadow: %v", err)
						}
					})
					if err := rocS0ValidateShadowPath(f.store.Root, slug, result.ShadowPath); err != nil {
						t.Fatal(err)
					}
					if !rocS0ResolveID.MatchString(result.ResolveSession) {
						t.Fatalf("unexpected resolver session identifier %q", result.ResolveSession)
					}
				}
				if f.phase != "" && (result.Phase != f.phase || result.Outcome != f.outcome) {
					t.Fatalf("fixture reached %s/%s, want %s/%s", result.Phase, result.Outcome, f.phase, f.outcome)
				}
				if name == "phase2-conflict-fallthrough" && !containsString(result.Notes, "Operation-level: 1 present, 1 applicable, 1 conflicts") {
					t.Fatalf("missing exact mixed/conflict note: %v", result.Notes)
				}
				if name == "phase2-legacy-rejected" && !containsString(result.Notes, "All recipe operations already present in upstream") {
					t.Fatalf("missing exact all-present note: %v", result.Notes)
				}
				if name == "phase2-applicable-only" || name == "phase2-mixed" {
					for _, note := range result.Notes {
						if strings.HasPrefix(note, "Operation-level:") {
							t.Fatalf("gate-off baseline unexpectedly added an operation note: %q", note)
						}
					}
				}
				normalize := func(body string) string {
					return rocS0Normalize(body, f.store.Root, result.ShadowPath, result.ResolveSession)
				}
				for _, artifact := range []string{"reconcile-session.json", "reconcile-evidence.jsonl", "reconcile.md"} {
					body, err := f.store.ReadFeatureFile(slug, filepath.Join("artifacts", artifact))
					if err != nil {
						t.Fatal(err)
					}
					rocS0Frame(&transcript, slug+"/"+artifact, normalize(body))
				}
				st, err := f.store.LoadFeatureStatus(slug)
				if err != nil {
					t.Fatal(err)
				}
				if st.Reconcile.ResolveSession != result.ResolveSession {
					t.Fatalf("resolver session differs between result and status")
				}
				if st.Reconcile.ShadowPath != result.ShadowPath {
					t.Fatal("shadow path differs between result and status")
				}
				if result.ResolveSession != "" {
					raw, err := f.store.ReadFeatureFile(slug, "artifacts/resolution-session.json")
					if err != nil {
						t.Fatal(err)
					}
					var session ResolveResult
					if err := json.Unmarshal([]byte(raw), &session); err != nil {
						t.Fatal(err)
					}
					if session.SessionID != result.ResolveSession {
						t.Fatal("resolver-owned session identifier differs from reconcile")
					}
				}
				summary, err := json.MarshalIndent(st.Reconcile, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				rocS0Frame(&transcript, slug+"/reconcile-summary.json", normalize(string(summary)+"\n"))
				rocS0Frame(&transcript, slug+"/state", string(st.State)+"\n")
				body, err := f.store.ReadFeatureFile(slug, "artifacts/post-apply.patch")
				if err != nil || body != canonical[slug] {
					t.Fatalf("canonical input changed: %v", err)
				}
				_, err = os.Stat(filepath.Join(f.store.Root, ".tpatch", "features", slug, "artifacts", "incremental.patch"))
				if f.options.CumulativeLegacy && err != nil {
					t.Fatalf("legacy incremental artifact absent: %v", err)
				}
				if !f.options.CumulativeLegacy && !os.IsNotExist(err) {
					t.Fatalf("default mode wrote an incremental artifact: %v", err)
				}
			}
			rocS0CompareGolden(t, name, transcript.String())
		})
	}
}
