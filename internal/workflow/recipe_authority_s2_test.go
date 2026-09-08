package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func rgaS2Git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func rgaS2Creation(t *testing.T) (*store.Store, patchobs.Observation) {
	t.Helper()
	s := setupAutogenStore(t, "s2", map[string]string{"z.txt": "last\r\n", "a.txt": "first"})
	obs := captureRecipeForTest(s.Root, "s2", newFilePatch("z.txt", "a.txt"))
	return s, obs
}

func rgaS2Write(t *testing.T, s *store.Store, name, raw string) {
	t.Helper()
	if err := s.WriteArtifact("s2", name, raw); err != nil {
		t.Fatal(err)
	}
}

func rgaS2Read(t *testing.T, s *store.Store, name string) string {
	t.Helper()
	raw, err := s.ReadFeatureFile("s2", "artifacts/"+name)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestRGAS2PureExactDerivation(t *testing.T) {
	s, obs := rgaS2Creation(t)
	before, err := DeriveRecipe(obs)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.canonical) == 0 || len(before.recipe.Operations) != 2 {
		t.Fatalf("no complete derivation: %+v", before)
	}
	for i, want := range []struct{ path, body string }{{"a.txt", "first"}, {"z.txt", "last\r\n"}} {
		op := before.recipe.Operations[i]
		if op.Path != want.path || op.Content != want.body || op.PreimageHash == nil || *op.PreimageHash != "" {
			t.Fatalf("creation %d loses exact bytes or explicit absence: %+v", i, op)
		}
	}
	if err := os.WriteFile(filepath.Join(s.Root, "a.txt"), []byte("late edit"), 0o644); err != nil {
		t.Fatal(err)
	}
	obs.RepoRoot = filepath.Join(s.Root, "does-not-exist")
	after, err := DeriveRecipe(obs)
	if err != nil || !bytes.Equal(before.canonical, after.canonical) {
		t.Fatalf("derivation consulted live state: %v", err)
	}
	returned := before.CanonicalBytes()
	returned[0] = '!'
	if !before.ProvesOrigin(after.canonical) {
		t.Fatal("caller mutated the private proof bytes")
	}
	// The same validator must reject changed captured image bytes.
	bad := obs
	bad.Effects = slices.Clone(obs.Effects)
	bad.Effects[0].Bytes.Postimage = []byte("late edit")
	if _, err := DeriveRecipe(bad); err == nil {
		t.Fatal("changed observation image accepted")
	}
	bad = obs
	bad.PatchBytes = append(bytes.Clone(obs.PatchBytes), '\n')
	if _, err := DeriveRecipe(bad); err == nil {
		t.Fatal("changed captured patch accepted")
	}
}

func TestRGAS2ExistingPreimageAndRange(t *testing.T) {
	s := setupAutogenStore(t, "s2", map[string]string{"a.txt": "old\r\nno newline"})
	rgaS2Git(t, s.Root, "add", "a.txt")
	rgaS2Git(t, s.Root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "preimage")
	base := strings.TrimSpace(rgaS2Git(t, s.Root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(s.Root, "a.txt"), []byte("new\r\nno newline"), 0o644); err != nil {
		t.Fatal(err)
	}
	rgaS2Git(t, s.Root, "add", "a.txt")
	rgaS2Git(t, s.Root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "postimage")
	patch := rgaS2Git(t, s.Root, "diff", "--binary", base, "HEAD")
	if err := os.WriteFile(filepath.Join(s.Root, "a.txt"), []byte("unrelated worktree"), 0o644); err != nil {
		t.Fatal(err)
	}
	obs := patchobs.Observe(patchobs.Input{
		Producer: patchobs.ProducerRecord, RepoRoot: s.Root, Slug: "s2",
		Patch: patch, PatchPresent: true, PreimageRef: base, PostimageRef: "HEAD",
		Capture: patchobs.CaptureDescriptor{Mode: patchobs.CaptureModeCommittedRange},
	})
	d, err := DeriveRecipe(obs)
	if err != nil || len(d.recipe.Operations) != 1 {
		t.Fatalf("range derivation: %+v, %v", d, err)
	}
	op := d.recipe.Operations[0]
	want := "sha256:" + store.SHA256HexString("old\r\nno newline")
	if op.PreimageHash == nil || *op.PreimageHash != want || op.Content != "new\r\nno newline" {
		t.Fatalf("range must use lower/upper commit bytes, got %+v", op)
	}
	out, err := AutogenRecipeForRecord(s, obs, true, false)
	if err != nil || !out.OriginProved || !out.ProvenanceWritten {
		t.Fatalf("range origin: %+v, %v", out, err)
	}
	var prov RecipeProvenance
	if err := json.Unmarshal([]byte(rgaS2Read(t, s, "recipe-provenance.json")), &prov); err != nil || prov.BaseCommit != base {
		t.Fatalf("provenance must use lower bound, not HEAD: %+v, %v", prov, err)
	}
}

func TestRGAS2CanonicalEncoder(t *testing.T) {
	recipe := ApplyRecipe{Feature: "s2", Operations: []RecipeOperation{
		{Type: "write-file", Path: "a.txt", Content: "<&>\r\n\u2028", PreimageHash: new(string)},
	}}
	first, err := EncodeRecipe(recipe)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ApplyRecipe
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatal(err)
	}
	second, err := EncodeRecipe(decoded)
	if err != nil || !bytes.Equal(first, second) || !bytes.HasSuffix(first, []byte("\n")) ||
		!reflect.DeepEqual(decoded, recipe) {
		t.Fatalf("noncanonical or lossy encoding: %q / %q / %v", first, second, err)
	}
	for _, field := range []string{"feature", "path", "content", "preimage"} {
		t.Run(field+"-invalid-UTF8", func(t *testing.T) {
			bad := decoded
			bad.Operations = slices.Clone(decoded.Operations)
			switch field {
			case "feature":
				bad.Feature = "\xff"
			case "path":
				bad.Operations[0].Path = "\xff"
			case "content":
				bad.Operations[0].Content = "\xff"
			case "preimage":
				value := "\xff"
				bad.Operations[0].PreimageHash = &value
			}
			if _, err := EncodeRecipe(bad); err == nil {
				t.Fatal("encoder silently replaced invalid UTF-8")
			}
		})
	}
}

func TestRGAS2OriginDiscriminatorAndManualPreservation(t *testing.T) {
	s, obs := rgaS2Creation(t)
	d, err := DeriveRecipe(obs)
	if err != nil {
		t.Fatal(err)
	}
	canonical := string(d.canonical)
	var compact bytes.Buffer
	if err := json.Compact(&compact, d.canonical); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(d.canonical, &decoded); err != nil {
		t.Fatal(err)
	}
	reordered := fmt.Sprintf("{\"operations\":%s,\"feature\":%s}\n", decoded["operations"], decoded["feature"])
	cases := map[string]string{
		"indentation":                      compact.String(),
		"key-order":                        reordered,
		"one-operation-field":              strings.Replace(canonical, `"search": ""`, `"search": "manual"`, 1),
		"origin-by-file-set-equality":      strings.Replace(canonical, `"first"`, `"different content"`, 1),
		"provenance-trust-by-label-marker": strings.Replace(canonical, "{", "{\"generated-by\":\"tpatch\",", 1),
		"created-by-edge":                  strings.Replace(canonical, `"preimage_hash": ""`, `"created_by": "parent", "preimage_hash": ""`, 1),
		"richer-provider":                  `{"feature":"s2","operations":[{"type":"replace-in-file","path":"a.txt","search":"before","replace":"after"}]}`,
	}
	if !d.ProvesOrigin(d.canonical) || (RecipeDerivation{}).ProvesOrigin(nil) {
		t.Fatal("origin validator lacks an exact positive or incomplete negative")
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if raw == canonical || d.ProvesOrigin([]byte(raw)) {
				t.Fatal("same origin validator accepted a near-match or label")
			}
			rgaS2Write(t, s, "apply-recipe.json", raw)
			provenancePath := filepath.Join(s.Root, ".tpatch/features/s2/artifacts/recipe-provenance.json")
			if err := os.Remove(provenancePath); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			out, err := AutogenRecipeForRecord(s, obs, true, false)
			if err != nil || out.OriginProved || out.ProvenanceWritten {
				t.Fatalf("unproved recipe acquired missing provenance: %+v, %v", out, err)
			}
			if _, err := os.Stat(provenancePath); !os.IsNotExist(err) {
				t.Fatalf("manual recipe provenance fabricated: %v", err)
			}
			const historical = "historical provenance must not be adopted"
			rgaS2Write(t, s, "recipe-provenance.json", historical)
			out, err = AutogenRecipeForRecord(s, obs, true, false)
			if err != nil || out.OriginProved || out.ProvenanceWritten ||
				rgaS2Read(t, s, "apply-recipe.json") != raw ||
				rgaS2Read(t, s, "recipe-provenance.json") != historical {
				t.Fatalf("manual-recipe-provenance-fabricated: %+v, %v", out, err)
			}
		})
	}
}

func TestRGAS2ConvergentProvenance(t *testing.T) {
	s, obs := rgaS2Creation(t)
	d, err := DeriveRecipe(obs)
	if err != nil {
		t.Fatal(err)
	}
	// Crash window: recipe was written but provenance never was.
	rgaS2Write(t, s, "apply-recipe.json", string(d.canonical))
	out, err := AutogenRecipeForRecord(s, obs, true, false)
	if err != nil || out.Action != AutogenNoop || !out.OriginProved || !out.ProvenanceWritten {
		t.Fatalf("crash recovery did not converge: %+v, %v", out, err)
	}
	hash := store.SHA256HexString(string(d.canonical))
	matching := fmt.Sprintf("{ \"generated_at\": \"2020-01-02T03:04:05Z\", \"base_commit\": %q, \"recipe_sha256\": %q }\n", obs.Reference.Commit, hash)
	rgaS2Write(t, s, "recipe-provenance.json", matching)
	out, err = AutogenRecipeForRecord(s, obs, false, false)
	if err != nil || out.ProvenanceWritten || rgaS2Read(t, s, "recipe-provenance.json") != matching {
		t.Fatalf("truthful noop changed provenance bytes/time: %+v, %v", out, err)
	}
	for _, raw := range []string{
		"{broken", `{}`, strings.Replace(matching, hash, strings.Repeat("a", 64), 1),
		strings.Replace(matching, obs.Reference.Commit, strings.Repeat("b", 40), 1),
	} {
		rgaS2Write(t, s, "recipe-provenance.json", raw)
		out, err = AutogenRecipeForRecord(s, obs, true, false)
		if err != nil || out.Action != AutogenNoop || !out.ProvenanceWritten {
			t.Fatalf("stale/malformed provenance not repaired: %+v, %v", out, err)
		}
		var prov RecipeProvenance
		if err := json.Unmarshal([]byte(rgaS2Read(t, s, "recipe-provenance.json")), &prov); err != nil ||
			prov.BaseCommit != obs.Reference.Commit || prov.RecipeSHA256 == nil || *prov.RecipeSHA256 != hash {
			t.Fatalf("historical-adoption-backdated-base accepted: %+v / %v", prov, err)
		}
	}
	// Non-durable input can never prove origin, even beside identical bytes.
	for _, kind := range []patchobs.ReferenceKind{patchobs.ReferenceKindUnavailable, patchobs.ReferenceKindIndexSnapshot} {
		bad := obs
		bad.Reference.Kind, bad.Reference.Commit = kind, ""
		before := rgaS2Read(t, s, "recipe-provenance.json")
		out, err = AutogenRecipeForRecord(s, bad, true, true)
		if err != nil || out.OriginProved || out.ProvenanceWritten ||
			rgaS2Read(t, s, "recipe-provenance.json") != before ||
			rgaS2Read(t, s, "apply-recipe.json") != string(d.canonical) {
			t.Fatalf("non-durable recipe acquired provenance: %+v, %v", out, err)
		}
	}
}

func TestRGAS2ProvenanceFailureAndRecovery(t *testing.T) {
	s, obs := rgaS2Creation(t)
	provPath := filepath.Join(s.Root, ".tpatch", "features", "s2", "artifacts", "recipe-provenance.json")
	if err := os.Mkdir(provPath, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := AutogenRecipeForRecord(s, obs, true, false)
	if err == nil || out.ProvenanceWritten {
		t.Fatalf("provenance failure became success: %+v, %v", out, err)
	}
	recipe := rgaS2Read(t, s, "apply-recipe.json")
	if err := os.Remove(provPath); err != nil {
		t.Fatal(err)
	}
	out, err = AutogenRecipeForRecord(s, obs, true, false)
	if err != nil || out.Action != AutogenNoop || !out.ProvenanceWritten ||
		rgaS2Read(t, s, "apply-recipe.json") != recipe {
		t.Fatalf("recovery rewrote recipe or failed: %+v, %v", out, err)
	}
}

func rgaS2PureSource(src string) error {
	file, err := rgaS0Parse("recipe_derivation.go", src)
	if err != nil {
		return err
	}
	for _, imp := range file.Imports {
		switch imp.Path.Value {
		case `"bytes"`, `"encoding/json"`, `"fmt"`, `"slices"`, `"sort"`, `"strings"`, `"unicode/utf8"`,
			`"github.com/tesseracode/tesserapatch/internal/gitutil"`,
			`"github.com/tesseracode/tesserapatch/internal/patchobs"`,
			`"github.com/tesseracode/tesserapatch/internal/store"`:
		default:
			return fmt.Errorf("impure/unreviewed derivation import %s", imp.Path.Value)
		}
	}
	for _, forbidden := range []string{"patchobs.Observe(", "patchobs.ObserveAndEmit(", "time.Now(", "os.ReadFile("} {
		if strings.Contains(src, forbidden) {
			return fmt.Errorf("derivation acquired live state via %s", forbidden)
		}
	}
	return nil
}

func TestRGAS2PurityGuardAndSensitivity(t *testing.T) {
	src := rgaS0ReadRepoFile(t, "internal/workflow/recipe_derivation.go")
	if err := rgaS2PureSource(src); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{
		strings.Replace(src, `"encoding/json"`, `"encoding/json"`+"\n\t\"os\"", 1),
		strings.Replace(src, `"encoding/json"`, `"encoding/json"`+"\n\tclock \"time\"", 1),
		strings.Replace(src, "d := RecipeDerivation{", "obs = patchobs.Observe(obs)\n d := RecipeDerivation{", 1),
	} {
		if mutation == src || rgaS2PureSource(mutation) == nil {
			t.Fatal("origin-proved-without-immutable-observation mutation passed the same purity guard")
		}
	}
}

func TestRGAS2UnsupportedEffectsAndNoPartialRecipe(t *testing.T) {
	for _, tc := range []struct {
		name, patch, pre, post, oldMode, newMode string
		want                                     []string
	}{
		{"delete", deletePatch("x"), "old\n", "", "100644", "", []string{"effect-delete-unsupported"}},
		{"rename-binary-executable", "diff --git a/old b/x\nsimilarity index 100%\nrename from old\nrename to x\n", "\x00", "\x00", "100755", "100755",
			[]string{"effect-binary-unsupported", "effect-executable-unsupported", "effect-rename-unsupported"}},
		{"copy-symlink", "diff --git a/old b/x\nsimilarity index 100%\ncopy from old\ncopy to x\n", "target", "target", "120000", "120000",
			[]string{"effect-copy-unsupported", "effect-symlink-unsupported"}},
		{"mode-only", "diff --git a/x b/x\nold mode 100644\nnew mode 100755\n", "same", "same", "100644", "100755",
			[]string{"effect-executable-unsupported", "effect-mode-only-unsupported"}},
		{"executable-add", strings.Replace(newFilePatch("x"), "100644", "100755", 1), "", "x\n", "", "100755",
			[]string{"effect-executable-unsupported"}},
		{"gitlink-add", strings.Replace(newFilePatch("x"), "100644", "160000", 1), "", "", "", "160000",
			[]string{"effect-gitlink-unsupported"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := setupAutogenStore(t, "s2", nil)
			obs := captureRecipeForTest(s.Root, "s2", tc.patch)
			e := &obs.Effects[0]
			preExtant, postExtant := e.Effect.ExtantSides()
			e.Effect.PreimageObserved, e.Effect.PostimageObserved = true, true
			e.Effect.PreimagePresent, e.Effect.PostimagePresent = preExtant, postExtant
			e.Effect.OldMode, e.Effect.NewMode = tc.oldMode, tc.newMode
			e.Effect.PreimageSHA256, e.Effect.PostimageSHA256 = "", ""
			if preExtant {
				e.Bytes.Preimage = []byte(tc.pre)
				e.Effect.PreimageSHA256 = store.SHA256HexString(tc.pre)
			}
			if postExtant {
				e.Bytes.Postimage = []byte(tc.post)
				e.Effect.PostimageSHA256 = store.SHA256HexString(tc.post)
			}
			obs.Reference.PreimageSetSHA256 = patchobs.PreimageSetDigest(obs.Effects)
			d, err := DeriveRecipe(obs)
			if err != nil || len(d.canonical) != 0 || len(d.recipe.Operations) != 0 ||
				len(d.Exclusions) != 1 || !reflect.DeepEqual(d.Exclusions[0].ReasonCodes, tc.want) {
				t.Fatalf("exclusions not exact, or partial bytes leaked: %+v / %v", d, err)
			}
			if d.ProvesOrigin([]byte(`{"feature":"s2","operations":[]}`)) {
				t.Fatal("unsupported observation proved origin")
			}
			if tc.name == "executable-add" {
				bad := obs
				bad.Effects = slices.Clone(obs.Effects)
				bad.Effects[0].Effect.NewMode = "100644"
				bad.Effects[0].Effect.ObjectKind = "regular"
				bad.Reference.PreimageSetSHA256 = patchobs.PreimageSetDigest(bad.Effects)
				if forged, err := DeriveRecipe(bad); err == nil || len(forged.canonical) != 0 {
					t.Fatal("executable header with forged regular observation proved origin")
				}
			}
			for _, existing := range []bool{false, true} {
				if existing {
					rgaS2Write(t, s, "apply-recipe.json", "manual bytes")
					rgaS2Write(t, s, "recipe-provenance.json", "manual provenance")
				}
				out, err := AutogenRecipeForRecord(s, obs, true, true)
				if err != nil || out.OriginProved || out.ProvenanceWritten {
					t.Fatalf("unsupported regeneration: %+v / %v", out, err)
				}
				if existing && (rgaS2Read(t, s, "apply-recipe.json") != "manual bytes" ||
					rgaS2Read(t, s, "recipe-provenance.json") != "manual provenance") {
					t.Fatal("incomplete regeneration replaced manual artifacts")
				}
				if !existing {
					if _, err := os.Stat(filepath.Join(s.Root, ".tpatch/features/s2/artifacts/apply-recipe.json")); !os.IsNotExist(err) {
						t.Fatalf("partial recipe published: %v", err)
					}
				}
			}
		})
	}
}

func TestRGAS2MissingObservationAndParentCreated(t *testing.T) {
	_, obs := rgaS2Creation(t)
	for _, which := range []string{"preimage", "postimage", "parent"} {
		t.Run(which, func(t *testing.T) {
			bad := obs
			bad.Effects = slices.Clone(obs.Effects)
			e := &bad.Effects[0]
			want := "parent-created-target-unsupported"
			switch which {
			case "preimage":
				e.Effect.PreimageObserved = false
				want = "preimage-unavailable"
			case "postimage":
				e.Effect.PostimageObserved, e.Effect.PostimagePresent = false, false
				e.Effect.PostimageSHA256, e.Effect.NewMode = "", ""
				e.Bytes.Postimage = nil
				want = "postimage-unavailable"
			case "parent":
				bad.ParentCreatedPaths = []string{e.Effect.Path}
			}
			bad.Reference.PreimageSetSHA256 = patchobs.PreimageSetDigest(bad.Effects)
			d, err := DeriveRecipe(bad)
			if err != nil || len(d.Exclusions) != 1 || !reflect.DeepEqual(d.Exclusions[0].ReasonCodes, []string{want}) ||
				len(d.canonical) != 0 || d.ProvesOrigin(nil) {
				t.Fatalf("missing/parent exclusion not enforced: %+v / %v", d, err)
			}
		})
	}
	for _, patch := range []string{"", " \r\n\t"} {
		empty := captureRecipeForTest(obs.RepoRoot, obs.Slug, patch)
		d, err := DeriveRecipe(empty)
		if err != nil || !slices.Contains(d.Reasons, patchobs.ReasonPatchEmpty) || len(d.canonical) != 0 {
			t.Fatalf("semantic emptiness became a recipe: %+v / %v", d, err)
		}
	}
	missing := patchobs.Observe(patchobs.Input{
		Producer: patchobs.ProducerRecord, RepoRoot: obs.RepoRoot, Slug: obs.Slug,
		Capture: obs.Capture, PreimageRef: "HEAD",
	})
	d, err := DeriveRecipe(missing)
	if err != nil || !slices.Contains(d.Reasons, patchobs.ReasonPatchMissing) || len(d.canonical) != 0 {
		t.Fatalf("absent patch became a recipe: %+v / %v", d, err)
	}
}

func TestRGAS2UnencodablePostimageRefusedWithoutLoss(t *testing.T) {
	s := setupAutogenStore(t, "s2", map[string]string{"a.txt": "\xff"})
	obs := captureRecipeForTest(s.Root, "s2", newFilePatch("a.txt"))
	if d, err := DeriveRecipe(obs); err == nil || len(d.canonical) != 0 {
		t.Fatalf("invalid UTF-8 was rewritten by JSON or mislabeled binary: %+v / %v", d, err)
	}
}

func TestRGAS2ObservationBindingGuardSensitivity(t *testing.T) {
	_, obs := rgaS2Creation(t)
	if _, err := DeriveRecipe(obs); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct {
		name  string
		apply func(*patchobs.Observation)
	}{
		{"wrong-effect-path", func(o *patchobs.Observation) { o.Effects[0].Effect.Path = "different" }},
		{"wrong-fragment", func(o *patchobs.Observation) { o.Effects[0].Effect.FragmentSHA256 = strings.Repeat("f", 64) }},
		{"extant-side-proven-absent", func(o *patchobs.Observation) {
			e := &o.Effects[0]
			e.Effect.PostimagePresent = false
			e.Effect.NewMode, e.Effect.PostimageSHA256, e.Bytes.Postimage = "", "", nil
		}},
		{"unobserved-side-with-body", func(o *patchobs.Observation) { o.Effects[0].Effect.PostimageObserved = false }},
		{"fabricated-commit", func(o *patchobs.Observation) { o.Reference.Commit = "HEAD" }},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			bad := obs
			bad.Effects = slices.Clone(obs.Effects)
			mutation.apply(&bad)
			bad.Reference.PreimageSetSHA256 = patchobs.PreimageSetDigest(bad.Effects)
			if _, err := DeriveRecipe(bad); err == nil {
				t.Fatal("same derivation validator accepted fabricated evidence")
			}
		})
	}
}
