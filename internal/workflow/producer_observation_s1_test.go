package workflow

// GH #15 / ADR-036 slice S1 — the producer observations owned by this
// package (PRD-recipe-generation-authority §6.2, ADR-036 D2/D15).
//
// Two producers live here:
//
//   - P6 `implement`, whose two recipe-write arms bind DIFFERENT bytes and
//     must each observe the bytes their own arm writes;
//   - P3 `reconcile --accept` → RefreshAfterAccept, which binds the
//     REGENERATED patch it is about to write, against the accepted
//     upstream commit.
//
// The third obligation is ordering (PI-3): a patch the strict grammar
// refuses must be refused in the producer's discovery window, BEFORE its
// first bound write, rather than surfacing from the downstream
// `AppendPatchGenerationForFeature` reparse once the artifacts have
// already landed. Ordering is proven structurally, because the runtime
// inputs of these producers come from `git diff` and cannot be made
// malformed from a test.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// s1Recorder collects everything handed to the publication seam.
type s1Recorder struct{ got []patchobs.Observation }

func (r *s1Recorder) Record(o patchobs.Observation) { r.got = append(r.got, o) }

func s1Collect(t *testing.T) *s1Recorder {
	t.Helper()
	rec := &s1Recorder{}
	restore := patchobs.SetRecorder(rec)
	t.Cleanup(restore)
	return rec
}

func s1SHA256(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// s1ImplementFixture builds a repo with one feature ready for implement.
func s1ImplementFixture(t *testing.T) (*store.Store, string, string) {
	t.Helper()
	dir := t.TempDir()
	setupGitRepo(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "Demo", Slug: "demo", Request: "demo request"}); err != nil {
		t.Fatal(err)
	}
	return s, dir, "demo"
}

// ── P6: each arm observes the bytes IT writes ────────────────────────────

// TestS1ImplementObservesTheExactBytesItWrites is P6's byte-exactness
// row. The valid arm reserializes the recipe and appends a newline, so an
// observation taken before the parse would bind the raw provider response
// — bytes that arm never writes and that no later consumer could match
// against the artifact on disk.
func TestS1ImplementObservesTheExactBytesItWrites(t *testing.T) {
	cases := []struct {
		name     string
		provider provider.Provider
		cfg      provider.Config
		raw      string
	}{
		{
			name: "provider-supplied-recipe",
			// Deliberately compact, unindented and newline-free, so the
			// reserialized bytes cannot accidentally equal the response.
			provider: &rgaS0ScriptedProvider{response: `{"feature":"demo","operations":[{"type":"ensure-directory","path":"src/"}]}`},
			cfg:      provider.Config{Type: "openai-compatible", BaseURL: "http://x", Model: "m", AuthEnv: "TPATCH_TEST_KEY"},
			raw:      `{"feature":"demo","operations":[{"type":"ensure-directory","path":"src/"}]}`,
		},
		{
			name:     "heuristic-fallback",
			provider: nil,
			cfg:      provider.Config{},
			raw:      heuristicRecipe("demo"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TPATCH_TEST_KEY", "stub")
			s, root, slug := s1ImplementFixture(t)
			rec := s1Collect(t)

			var warn strings.Builder
			prev := WarnWriter
			WarnWriter = &warn
			defer func() { WarnWriter = prev }()

			if err := RunImplement(context.Background(), s, slug, tc.provider, tc.cfg); err != nil {
				t.Fatalf("RunImplement: %v (warnings=%q)", err, warn.String())
			}

			if len(rec.got) != 1 {
				t.Fatalf("the seam received %d observation(s), want exactly 1", len(rec.got))
			}
			obs := rec.got[0]
			if obs.Producer != patchobs.ProducerImplement {
				t.Fatalf("producer = %q, want %q", obs.Producer, patchobs.ProducerImplement)
			}
			if obs.ArtifactAfter == nil {
				t.Fatal("P6 must checkpoint the recipe bytes it binds")
			}

			onDisk, err := os.ReadFile(filepath.Join(root, ".tpatch", "features", slug, "artifacts", "apply-recipe.json"))
			if err != nil {
				t.Fatalf("read apply-recipe.json: %v", err)
			}
			if string(obs.ArtifactAfter.Bytes) != string(onDisk) {
				t.Fatalf("the observation bound bytes the arm did not write:\n obs  %q\n disk %q",
					obs.ArtifactAfter.Bytes, onDisk)
			}
			if obs.ArtifactAfter.SHA256 != s1SHA256(string(onDisk)) {
				t.Fatalf("checkpoint digest %q does not describe the bytes on disk", obs.ArtifactAfter.SHA256)
			}
			// The valid arm's payload really is a REserialization, so the
			// row is not vacuous: raw and written bytes differ.
			if string(onDisk) == tc.raw {
				t.Fatalf("this row needs the valid arm to rewrite the bytes; raw and written are identical (%q)", tc.raw)
			}
			if string(obs.ArtifactAfter.Bytes) == tc.raw {
				t.Fatal("the observation bound the RAW response instead of the reserialized recipe")
			}
			if !strings.HasSuffix(string(obs.ArtifactAfter.Bytes), "\n") {
				t.Fatalf("the trailing newline is part of the bound payload, got %q", obs.ArtifactAfter.Bytes)
			}
			if obs.ArtifactAfter.Path != filepath.Join(root, ".tpatch", "features", slug, "artifacts", "apply-recipe.json") {
				t.Fatalf("checkpoint path = %q, want the resolved recipe path", obs.ArtifactAfter.Path)
			}
			// P6 runs no capture, so it establishes no reference.
			if obs.Capture.Mode != patchobs.CaptureModeNoCapture {
				t.Fatalf("capture mode = %q, want no-capture", obs.Capture.Mode)
			}
			if obs.Reference.Kind != patchobs.ReferenceKindUnavailable {
				t.Fatalf("reference kind = %q, want unavailable", obs.Reference.Kind)
			}
		})
	}
}

// TestS1ImplementRawArmObservesTheRawBytes covers the arm `RunImplement`
// cannot currently reach (frozen by
// TestRGAS0ImplementRawArmIsCurrentlyUnreachable): when the response is
// not decodable, the RAW bytes are what lands, so the raw bytes are what
// is observed. The helper is exercised directly, and the source guard
// below proves the arm passes it the same expression it writes.
func TestS1ImplementRawArmObservesTheRawBytes(t *testing.T) {
	s, root, slug := s1ImplementFixture(t)
	rec := s1Collect(t)

	raw := "not json at all\n"
	obs := ObserveImplementCheckpoint(s, slug, raw)

	if len(rec.got) != 1 {
		t.Fatalf("the seam received %d observation(s), want 1", len(rec.got))
	}
	if string(obs.ArtifactAfter.Bytes) != raw {
		t.Fatalf("checkpoint bytes = %q, want the raw payload %q", obs.ArtifactAfter.Bytes, raw)
	}
	if obs.ArtifactAfter.SHA256 != s1SHA256(raw) {
		t.Fatalf("checkpoint digest does not describe the raw payload")
	}
	if obs.ArtifactAfter.Path != filepath.Join(root, ".tpatch", "features", slug, "artifacts", "apply-recipe.json") {
		t.Fatalf("checkpoint path = %q, want the resolved recipe path", obs.ArtifactAfter.Path)
	}
}

// TestS1ImplementCheckpointBindsTheCanonicalPatchTruthfully covers the
// other half of P6's input: the canonical patch is bound when it is
// readable and recorded as MISSING when there is none. A producer that
// defaulted a real patch to absent would publish incompleteness it does
// not have.
func TestS1ImplementCheckpointBindsTheCanonicalPatchTruthfully(t *testing.T) {
	t.Run("no-canonical-patch-is-recorded-missing", func(t *testing.T) {
		s, _, slug := s1ImplementFixture(t)
		s1Collect(t)
		obs := ObserveImplementCheckpoint(s, slug, "{}\n")
		if obs.PatchPresent {
			t.Fatal("there is no canonical patch; presence must be false")
		}
		if !s1HasReason(obs.Reasons, "canonical-patch-missing") {
			t.Fatalf("reasons = %v, want canonical-patch-missing", obs.Reasons)
		}
		if len(obs.Effects) != 0 {
			t.Fatalf("no patch means no effects, got %+v", obs.Effects)
		}
	})

	t.Run("an-existing-canonical-patch-is-bound", func(t *testing.T) {
		s, root, slug := s1ImplementFixture(t)
		if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Test\nchanged\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		patch := "diff --git a/README.md b/README.md\nindex 1111111..2222222 100644\n" +
			"--- a/README.md\n+++ b/README.md\n@@ -1 +1,2 @@\n # Test\n+changed\n"
		if err := s.WriteArtifact(slug, "post-apply.patch", patch); err != nil {
			t.Fatal(err)
		}
		s1Collect(t)
		obs := ObserveImplementCheckpoint(s, slug, "{}\n")
		if !obs.PatchPresent {
			t.Fatal("a readable canonical patch must be bound, not defaulted to missing")
		}
		if obs.PatchSHA256 != s1SHA256(patch) {
			t.Fatalf("patch digest = %q, want the digest of the canonical bytes", obs.PatchSHA256)
		}
		if len(obs.Effects) != 1 || obs.Effects[0].Effect.Path != "README.md" {
			t.Fatalf("effects = %+v, want one effect on README.md", obs.Effects)
		}
	})
}

// TestS1ImplementArmsObserveWhatTheyWrite is the source-level half of the
// byte-exactness rule, and it covers the arm no runtime row can reach.
//
// For BOTH arms of the recipe-parse if/else it requires:
//
//   - exactly one bound `apply-recipe.json` write;
//   - exactly one `ObserveImplementCheckpoint` call;
//   - the checkpoint's payload argument to be rendered IDENTICALLY to the
//     write's third argument, so the two cannot drift apart;
//   - the checkpoint to come first.
func TestS1ImplementArmsObserveWhatTheyWrite(t *testing.T) {
	src := rgaS0ReadRepoFile(t, "internal/workflow/implement.go")
	if err := s1CheckImplementArmObservation(src); err != nil {
		t.Fatalf("P6's arms no longer observe the bytes they write: %v", err)
	}

	t.Run("sensitivity", func(t *testing.T) {
		for _, tc := range []struct{ name, old, new string }{
			{
				// The rev-0 defect: one observation of the RAW response,
				// taken before the parse, for both arms.
				name: "valid-arm-observes-the-raw-response",
				old:  "\t\tObserveImplementCheckpoint(s, slug, reserialized)",
				new:  "\t\tObserveImplementCheckpoint(s, slug, recipeContent)",
			},
			{
				name: "an-arm-stops-observing-at-all",
				old:  "\t\tObserveImplementCheckpoint(s, slug, recipeContent)\n",
				new:  "",
			},
			{
				name: "the-observation-moves-below-the-write",
				old: "\t\tObserveImplementCheckpoint(s, slug, reserialized)\n" +
					"\t\tif err := s.WriteArtifact(slug, \"apply-recipe.json\", reserialized); err != nil {\n" +
					"\t\t\treturn err\n\t\t}",
				new: "\t\tif err := s.WriteArtifact(slug, \"apply-recipe.json\", reserialized); err != nil {\n" +
					"\t\t\treturn err\n\t\t}\n" +
					"\t\tObserveImplementCheckpoint(s, slug, reserialized)",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if !strings.Contains(src, tc.old) {
					t.Fatalf("mutation anchor no longer present:\n%q", tc.old)
				}
				if err := s1CheckImplementArmObservation(strings.Replace(src, tc.old, tc.new, 1)); err == nil {
					t.Fatalf("guard did not catch mutation %q", tc.name)
				}
			})
		}
	})
}

// s1CheckImplementArmObservation is the pure matcher the row above runs
// over both real and deliberately mutated source.
func s1CheckImplementArmObservation(src string) error {
	file, err := rgaS0Parse("implement.go", src)
	if err != nil {
		return err
	}
	fn := rgaS0FuncBody(file, "RunImplement")
	if fn == nil {
		return fmt.Errorf("RunImplement not found")
	}

	var parseStmt *ast.IfStmt
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if parseStmt != nil {
			return false
		}
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok || ifStmt.Init == nil || ifStmt.Else == nil {
			return true
		}
		assign, ok := ifStmt.Init.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, rhs := range assign.Rhs {
			if call, ok := rhs.(*ast.CallExpr); ok && rgaS0CallName(call) == "json.Unmarshal" {
				parseStmt = ifStmt
				return false
			}
		}
		return true
	})
	if parseStmt == nil {
		return fmt.Errorf("the json.Unmarshal recipe-parse if/else is gone from RunImplement")
	}
	elseBlock, ok := parseStmt.Else.(*ast.BlockStmt)
	if !ok {
		return fmt.Errorf("the recipe-parse else branch is no longer a block")
	}
	for name, block := range map[string]*ast.BlockStmt{
		"raw-invalid arm": parseStmt.Body,
		"valid arm":       elseBlock,
	} {
		if err := s1CheckArmPayloadIdentity(name, block); err != nil {
			return err
		}
	}
	return nil
}

// s1CheckArmPayloadIdentity asserts one arm observes exactly what it
// writes, and observes it first.
func s1CheckArmPayloadIdentity(name string, block *ast.BlockStmt) error {
	var writeArg, observeArg ast.Expr
	writes, observes := 0, 0
	ast.Inspect(block, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch {
		case s1CalleeName(call) == "WriteArtifact" && len(call.Args) == 3:
			artifact, isLit := rgaS0StringLit(call.Args[1])
			if !isLit || artifact != "apply-recipe.json" {
				return true
			}
			writes++
			writeArg = call.Args[2]
		case s1CalleeName(call) == "ObserveImplementCheckpoint" && len(call.Args) == 3:
			observes++
			observeArg = call.Args[2]
		}
		return true
	})
	if writes != 1 {
		return fmt.Errorf("%s: want exactly one bound apply-recipe.json write, got %d", name, writes)
	}
	if observes != 1 {
		return fmt.Errorf("%s: want exactly one ObserveImplementCheckpoint call, got %d", name, observes)
	}
	if s1RenderExpr(writeArg) != s1RenderExpr(observeArg) {
		return fmt.Errorf("%s: observes %s but writes %s; the observation must bind the bytes the write receives",
			name, s1RenderExpr(observeArg), s1RenderExpr(writeArg))
	}
	if observeArg.Pos() > writeArg.Pos() {
		return fmt.Errorf("%s: the observation is taken AFTER the write it binds", name)
	}
	return nil
}

// s1CalleeName renders a call's callee as the bare METHOD or function
// name, so `s.WriteArtifact` and `store.WriteArtifact` both match the
// obligation rather than the receiver that happens to spell it.
func s1CalleeName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}

// s1RenderExpr renders the small expression shapes a payload argument can
// take. Anything else renders as a distinct opaque form, so two different
// complex expressions never compare equal by accident.
func s1RenderExpr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return "ident:" + e.Name
	case *ast.BasicLit:
		return "lit:" + e.Value
	case *ast.BinaryExpr:
		return "(" + s1RenderExpr(e.X) + e.Op.String() + s1RenderExpr(e.Y) + ")"
	case *ast.CallExpr:
		parts := make([]string, 0, len(e.Args))
		for _, arg := range e.Args {
			parts = append(parts, s1RenderExpr(arg))
		}
		return "call:" + rgaS0CallName(e) + "(" + strings.Join(parts, ",") + ")"
	case *ast.SelectorExpr:
		return s1RenderExpr(e.X) + "." + e.Sel.Name
	default:
		return fmt.Sprintf("opaque:%T@%d", expr, expr.Pos())
	}
}

// ── P3: the observation binds the patch it is about to write ─────────────

// TestS1RefreshObservationBindsTheNewPatch is P3's binding row. The event
// P3 owns is the canonical patch WRITE, so the observation must describe
// `newPatch` — the bytes about to land — and not the pre-refresh patch
// those bytes replace.
func TestS1RefreshObservationBindsTheNewPatch(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "Demo", Slug: "demo", Request: "demo"}); err != nil {
		t.Fatal(err)
	}
	slug := "demo"
	upstream, err := gitutil.HeadCommit(dir)
	if err != nil {
		t.Fatalf("HeadCommit: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\nupdated line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	originalPatch := "diff --git a/README.md b/README.md\n" +
		"--- a/README.md\n+++ b/README.md\n@@ -1 +1,2 @@\n # Test\n+different line\n"
	if err := s.WriteArtifact(slug, "post-apply.patch", originalPatch); err != nil {
		t.Fatal(err)
	}

	rec := s1Collect(t)
	if err := RefreshAfterAccept(s, slug, upstream, originalPatch); err != nil {
		t.Fatalf("RefreshAfterAccept: %v", err)
	}
	if len(rec.got) != 1 {
		t.Fatalf("the seam received %d observation(s), want 1", len(rec.got))
	}
	obs := rec.got[0]
	if obs.Producer != patchobs.ProducerReconcileAccept {
		t.Fatalf("producer = %q, want %q", obs.Producer, patchobs.ProducerReconcileAccept)
	}

	written, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
	if err != nil {
		t.Fatal(err)
	}
	if written == originalPatch {
		t.Fatal("this row needs the refresh to produce DIFFERENT bytes")
	}
	if !obs.PatchPresent {
		t.Fatal("P3 binds a patch it is about to write; presence must be true")
	}
	if obs.PatchSHA256 != s1SHA256(written) {
		t.Fatalf("the observation bound %q, want the digest of the newly written patch %q",
			obs.PatchSHA256, s1SHA256(written))
	}
	if obs.PatchSHA256 == s1SHA256(originalPatch) {
		t.Fatal("the observation bound the PRE-refresh patch; P3's event is the write, not the state it ends")
	}
	if obs.Reference.Kind != patchobs.ReferenceKindCommit || obs.Reference.Commit != upstream {
		t.Fatalf("reference = %+v, want the accepted upstream commit %q", obs.Reference, upstream)
	}
	if obs.Capture.Mode != patchobs.CaptureModeReconcile {
		t.Fatalf("capture mode = %q, want reconcile", obs.Capture.Mode)
	}
	// Postimages come from the working tree the accept produced, which is
	// the state the new patch was diffed out of.
	if len(obs.Effects) != 1 {
		t.Fatalf("effects = %+v, want one", obs.Effects)
	}
	if got := string(obs.Effects[0].Bytes.Postimage); got != "# Test\nupdated line\n" {
		t.Fatalf("postimage bytes = %q, want the accepted working tree", got)
	}
}

// TestS1RefreshObservationOfAnEmptyPatchIsStillPresent covers the
// distinction the rev-0 implementation collapsed: a canonical patch that
// is ABOUT TO EXIST and happens to be empty is present-and-empty, which
// is not the same statement as "no canonical patch was readable".
func TestS1RefreshObservationOfAnEmptyPatchIsStillPresent(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	s, err := store.Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFeature(store.AddFeatureInput{Title: "Demo", Slug: "demo", Request: "demo"}); err != nil {
		t.Fatal(err)
	}
	slug := "demo"
	upstream, err := gitutil.HeadCommit(dir)
	if err != nil {
		t.Fatal(err)
	}

	// The original patch touches a path that is identical to upstream, so
	// the regenerated diff is empty.
	originalPatch := "diff --git a/README.md b/README.md\n" +
		"--- a/README.md\n+++ b/README.md\n@@ -1 +1 @@\n-# Test\n+# Test changed\n"
	if err := s.WriteArtifact(slug, "post-apply.patch", originalPatch); err != nil {
		t.Fatal(err)
	}

	rec := s1Collect(t)
	if err := RefreshAfterAccept(s, slug, upstream, originalPatch); err != nil {
		t.Fatalf("RefreshAfterAccept: %v", err)
	}
	written, err := s.ReadFeatureFile(slug, filepath.Join("artifacts", "post-apply.patch"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(written) != "" {
		t.Fatalf("this row needs an empty regenerated patch, got %q", written)
	}
	if len(rec.got) != 1 {
		t.Fatalf("the seam received %d observation(s), want 1", len(rec.got))
	}
	obs := rec.got[0]
	if !obs.PatchPresent {
		t.Fatal("a patch that is about to exist is PRESENT, even when it is empty")
	}
	if len(obs.PatchSHA256) != 64 {
		t.Fatalf("the empty-input digest is a real value: %q", obs.PatchSHA256)
	}
	if !s1HasReason(obs.Reasons, "canonical-patch-empty") {
		t.Fatalf("reasons = %v, want canonical-patch-empty", obs.Reasons)
	}
	if s1HasReason(obs.Reasons, "canonical-patch-missing") {
		t.Fatalf("reasons = %v: an empty patch is not a missing one", obs.Reasons)
	}
}

// ── PI-3: the strict refusal precedes every bound write ──────────────────

// TestS1RefreshRefusesBeforeItWrites is the ordering proof for P3. The
// runtime input is `git diff` output, which a test cannot make malformed,
// so the guarantee is measured on source: the preflight refusal must be
// positioned before the function's first bound write.
func TestS1RefreshRefusesBeforeItWrites(t *testing.T) {
	src := rgaS0ReadRepoFile(t, "internal/workflow/refresh.go")
	if err := s1CheckPreflightPrecedesWrites("refresh.go", src, "RefreshAfterAccept"); err != nil {
		t.Fatalf("P3's preflight ordering broke: %v", err)
	}

	t.Run("sensitivity", func(t *testing.T) {
		anchor := "\tif perr := obs.PreflightError(); perr != nil {\n" +
			"\t\treturn fmt.Errorf(\"refresh: regenerated post-apply.patch is unreadable, refusing to bind it: %w\", perr)\n\t}\n"
		if !strings.Contains(src, anchor) {
			t.Fatalf("preflight anchor no longer present:\n%q", anchor)
		}
		t.Run("preflight-removed", func(t *testing.T) {
			if err := s1CheckPreflightPrecedesWrites("refresh.go", strings.Replace(src, anchor, "", 1), "RefreshAfterAccept"); err == nil {
				t.Fatal("guard did not catch a removed preflight")
			}
		})
		t.Run("preflight-moved-below-the-write", func(t *testing.T) {
			moved := strings.Replace(src, anchor, "", 1)
			writeAnchor := "\tif err := s.WriteArtifact(slug, \"post-apply.patch\", newPatch); err != nil {\n" +
				"\t\treturn fmt.Errorf(\"refresh: write post-apply.patch: %w\", err)\n\t}\n"
			if !strings.Contains(moved, writeAnchor) {
				t.Fatalf("write anchor no longer present:\n%q", writeAnchor)
			}
			moved = strings.Replace(moved, writeAnchor, writeAnchor+anchor, 1)
			if err := s1CheckPreflightPrecedesWrites("refresh.go", moved, "RefreshAfterAccept"); err == nil {
				t.Fatal("guard did not catch a preflight that runs after the write")
			}
		})
		t.Run("preflight-error-discarded", func(t *testing.T) {
			discarded := strings.Replace(src, anchor, "\t_ = obs.PreflightError()\n", 1)
			if err := s1CheckPreflightPrecedesWrites("refresh.go", discarded, "RefreshAfterAccept"); err == nil {
				t.Fatal("guard did not catch a discarded preflight error")
			}
		})
	})
}

// s1CheckPreflightPrecedesWrites asserts that a producer refuses an
// unreadable patch before it writes anything bound.
//
// "Refuses" is specific: the preflight error must be BOUND and RETURNED,
// not evaluated and dropped. "Before" is positional over the function's
// own body, which is what a reader of the source can check too.
func s1CheckPreflightPrecedesWrites(fileName, src, fnName string) error {
	file, err := rgaS0Parse(fileName, src)
	if err != nil {
		return err
	}
	fn := rgaS0FuncBody(file, fnName)
	if fn == nil {
		return fmt.Errorf("%s not found in %s", fnName, fileName)
	}

	preflight := token.NoPos
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok || ifStmt.Init == nil {
			return true
		}
		assign, ok := ifStmt.Init.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, rhs := range assign.Rhs {
			call, ok := rhs.(*ast.CallExpr)
			if !ok || s1CalleeName(call) != "PreflightError" {
				continue
			}
			// The bound error must be returned, not logged and ignored.
			returns := false
			ast.Inspect(ifStmt.Body, func(inner ast.Node) bool {
				if _, ok := inner.(*ast.ReturnStmt); ok {
					returns = true
				}
				return true
			})
			if returns && (preflight == token.NoPos || call.Pos() < preflight) {
				preflight = call.Pos()
			}
		}
		return true
	})
	if preflight == token.NoPos {
		return fmt.Errorf("%s does not refuse on its observation's preflight error", fnName)
	}

	firstWrite := token.NoPos
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch s1CalleeName(call) {
		case "WriteArtifact":
			// Only a BOUND artifact counts: an unrelated artifact write
			// is not the event the preflight protects.
			if len(call.Args) < 2 {
				return true
			}
			artifact, isLit := rgaS0StringLit(call.Args[1])
			if !isLit || !s1BoundArtifacts[artifact] {
				return true
			}
		case "WritePatch":
		default:
			return true
		}
		if firstWrite == token.NoPos || call.Pos() < firstWrite {
			firstWrite = call.Pos()
		}
		return true
	})
	if firstWrite == token.NoPos {
		return fmt.Errorf("%s no longer performs a bound write; the ordering guard is stale", fnName)
	}
	if preflight > firstWrite {
		return fmt.Errorf("%s writes before it refuses an unreadable patch", fnName)
	}
	return nil
}

// s1BoundArtifacts is the closed set of artifacts coverage binds
// (ADR-036 §6.15). It mirrors the S0 bound-write inventory.
var s1BoundArtifacts = map[string]bool{
	"post-apply.patch":  true,
	"apply-recipe.json": true,
}

// ── the authority refuses duplicate destinations everywhere ─────────────

// TestS1AdaptersRefuseDuplicateDestinations is the workflow-side half of
// the duplicate-destination rule: every authoritative adapter refuses,
// and the one frozen PI-12 projection still de-duplicates.
func TestS1AdaptersRefuseDuplicateDestinations(t *testing.T) {
	duplicate := "diff --git a/dup.txt b/dup.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n" +
		"diff --git a/dup.txt b/dup.txt\nindex 2..3 100644\n@@ -1 +1 @@\n-b\n+c\n"
	distinct := "diff --git a/dup.txt b/dup.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n" +
		"diff --git a/other.txt b/other.txt\nindex 2..3 100644\n@@ -1 +1 @@\n-b\n+c\n"

	root := t.TempDir()
	for _, name := range []string{"dup.txt", "other.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("c\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("recipe-derivation-refuses", func(t *testing.T) {
		recipe, skipped, err := RecipeFromPatch(root, "demo", duplicate)
		if err == nil {
			t.Fatalf("the derivation must refuse a repeated destination, got %+v", recipe.Operations)
		}
		if len(recipe.Operations) != 0 || skipped != nil {
			t.Fatalf("a refusal must not return a partial derivation: %+v / %v", recipe.Operations, skipped)
		}
	})

	t.Run("novelty-classification-refuses", func(t *testing.T) {
		if paths, err := parsePatchNoveltyPaths(duplicate); err == nil {
			t.Fatalf("novelty classification must refuse a repeated destination, got %+v", paths)
		}
	})

	t.Run("hunk-attribution-refuses", func(t *testing.T) {
		if hunks, err := parsePatchHunks(duplicate); err == nil {
			t.Fatalf("hunk attribution must refuse a repeated destination, got %+v", hunks)
		}
	})

	t.Run("PI-12-touched-paths-keeps-its-frozen-shape", func(t *testing.T) {
		touched, err := strictTouchedPaths(duplicate)
		if err != nil {
			t.Fatalf("the frozen b-side projection must still accept this input: %v", err)
		}
		if len(touched) != 1 || touched[0] != "dup.txt" {
			t.Fatalf("touched paths = %q, want the frozen de-duplicated list [dup.txt]", touched)
		}
	})

	// Wrong-input sensitivity: the refusal is about duplicates, not about
	// multi-record patches.
	t.Run("two-distinct-destinations-are-accepted", func(t *testing.T) {
		recipe, _, err := RecipeFromPatch(root, "demo", distinct)
		if err != nil {
			t.Fatalf("two distinct destinations must be accepted: %v", err)
		}
		if len(recipe.Operations) != 2 {
			t.Fatalf("operations = %+v, want one per destination", recipe.Operations)
		}
		if paths, err := parsePatchNoveltyPaths(distinct); err != nil || len(paths) != 2 {
			t.Fatalf("novelty paths = %+v (err=%v), want two", paths, err)
		}
	})
}

func s1HasReason(reasons []string, want string) bool {
	for _, r := range reasons {
		if r == want {
			return true
		}
	}
	return false
}
