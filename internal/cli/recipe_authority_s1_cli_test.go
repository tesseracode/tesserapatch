package cli

// GH #15 / ADR-036 slice S1 — the migrated CLI call sites.
//
// Behavioural rows for the three PI-7 migrations that fail closed
// (RGA-082..RGA-084), the derived operation count (RGA-069), and P7's
// editor-error contract (§6.2).
//
// Each fail-closed row proves the refusal happens BEFORE the side effect
// it is protecting: no snapshot, no diff, no artifact write.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/tesseracode/tesserapatch/internal/patchobs"
	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

// s1UnreadableCanonicalPatch is a present, non-empty canonical patch the
// strict grammar refuses: the header names no b-side operand, so nothing
// can be said about which paths it touches.
const s1UnreadableCanonicalPatch = "diff --git a/truncated.txt\nindex 1111111..2222222 100644\n" +
	"--- a/truncated.txt\n+++ b/truncated.txt\n@@ -1 +1 @@\n-a\n+b\n"

// TestS1ValidateReapplyMaterializationFailsClosed is RGA-083: the strict
// parse error returns from validateReapplyMaterialization before
// DiffFromCommitForPaths is reached.
//
// The repo root is a directory with no git repository at all, so any git
// call would fail with a git-shaped error. Getting the strict refusal
// instead is the proof that the parse happens first.
func TestS1ValidateReapplyMaterializationFailsClosed(t *testing.T) {
	root := t.TempDir()
	err := validateReapplyMaterialization(root, s1UnreadableCanonicalPatch, false)
	if err == nil {
		t.Fatal("an unreadable canonical patch must refuse before the source diff is computed")
	}
	if !strings.Contains(err.Error(), "refusing to inspect a partial path set") {
		t.Fatalf("the refusal must name its cause, got: %v", err)
	}
	if strings.Contains(err.Error(), "not fully materialized") {
		t.Fatalf("the reverse-apply check ran before the strict parse: %v", err)
	}
}

// TestS1ApplyExecuteReapplyFailsClosed is RGA-082: with an unreadable
// canonical patch, the reapply path returns before SnapshotWorktreePaths
// runs and before any mutation.
//
// The function is driven directly rather than through the CLI so the row
// measures the migrated call site itself, and an unrelated preflight
// cannot mask — or fake — the refusal.
func TestS1ApplyExecuteReapplyFailsClosed(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateUnapplied)
	patchPath := filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "post-apply.patch")
	if err := os.WriteFile(patchPath, []byte(s1UnreadableCanonicalPatch), 0o644); err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "status.json")
	statusBefore := s1ReadFile(t, statusPath)
	readmeBefore := s1ReadFile(t, filepath.Join(fx.dir, "README.md"))

	cmd := &cobra.Command{}
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	result, err := runApplyExecuteChecked(cmd, fx.store, fx.slug, false)
	if err == nil {
		t.Fatalf("the reapply path must refuse an unreadable canonical patch, got %+v", result)
	}
	if !strings.Contains(err.Error(), "refusing to snapshot a partial path set") {
		t.Fatalf("the refusal must name its cause, got: %v", err)
	}
	if result.Applied != 0 || result.Success {
		t.Fatalf("a refused reapply must report nothing applied: %+v", result)
	}
	if got := s1ReadFile(t, statusPath); got != statusBefore {
		t.Fatalf("state was mutated by a refused reapply:\n got %s\nwant %s", got, statusBefore)
	}
	if got := s1ReadFile(t, filepath.Join(fx.dir, "README.md")); got != readmeBefore {
		t.Fatal("the worktree was mutated by a refused reapply")
	}
}

// TestS1FeatureUnapplyFailsClosed is RGA-084: the strict parse error
// returns before any snapshot, reverse patch or unapply artifact write.
func TestS1FeatureUnapplyFailsClosed(t *testing.T) {
	fx := newUnapplyFixture(t, store.StateApplied)
	patchPath := filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "post-apply.patch")
	if err := os.WriteFile(patchPath, []byte(s1UnreadableCanonicalPatch), 0o644); err != nil {
		t.Fatal(err)
	}
	statusBefore := s1ReadFile(t, filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "status.json"))

	stdout, stderr, code := runRJ("feature", "unapply", fx.slug, "--path", fx.dir, "--actor", "agent@example.com")
	if code == 0 {
		t.Fatalf("feature unapply must refuse an unreadable canonical patch:\nstdout=%s\nstderr=%s", stdout, stderr)
	}
	if !strings.Contains(stdout+stderr, "refusing to unapply a partial path set") {
		t.Fatalf("the refusal must name its cause:\n%s", stdout+stderr)
	}
	unapplyDir := filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "artifacts", "unapply")
	if _, err := os.Stat(unapplyDir); err == nil {
		t.Fatal("a refused unapply must not write an attempt directory")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", unapplyDir, err)
	}
	if got := s1ReadFile(t, filepath.Join(fx.dir, ".tpatch", "features", fx.slug, "status.json")); got != statusBefore {
		t.Fatal("state was mutated by a refused unapply")
	}
}

// TestS1RecordPrintsTheDerivedOperationCount is RGA-069: the
// `Recipe generated` line prints the operation count of the recipe the
// producer just derived.
//
// Two halves, because one alone would not prove the migration:
//
//   - the runtime half shows the printed number tracking the DERIVED
//     recipe on a real capture;
//   - the divergence half shows the two answers disagreeing on a patch
//     whose records do not map one-to-one onto operations, which is the
//     defect the migration removes. `countPatchFiles` matches any line
//     starting with `diff --git`, including one inside a `GIT binary
//     patch` stanza that the grammar correctly refuses to read as a
//     record boundary — so it says 2 where the derivation says 1, and
//     the pre-migration expression, file count minus skip list, said 2
//     as well.
func TestS1RecordPrintsTheDerivedOperationCount(t *testing.T) {
	t.Run("printed-number-tracks-the-derived-recipe", func(t *testing.T) {
		tmp := modesFixture(t, "s1-op-count")
		rgaS0CommitAll(t, tmp)
		modesWriteFile(t, tmp, "src/kept.txt", "kept\n")
		// README.md is committed by the fixture; deleting it adds a file
		// record the recipe schema cannot express.
		if err := os.Remove(filepath.Join(tmp, "README.md")); err != nil {
			t.Fatal(err)
		}

		stdout, stderr, code := runRecord(t, "record", "--path", tmp, "s1-op-count", "--lenient")
		if code != 0 {
			t.Fatalf("record failed: %s", stderr)
		}
		patch := readRecordedPatch(t, tmp, "s1-op-count")
		if got := countPatchFiles(patch); got != 2 {
			t.Fatalf("captured patch file count = %d, want 2 (one create + one delete)", got)
		}
		recipe := rgaS0ReadRecipe(t, tmp, "s1-op-count")
		if len(recipe.Operations) != 1 {
			t.Fatalf("derived recipe operations = %d, want 1: %+v", len(recipe.Operations), recipe.Operations)
		}
		wantLine := "  Recipe generated: artifacts/apply-recipe.json (1 ops)\n"
		if !strings.Contains(stdout, wantLine) {
			t.Fatalf("stdout missing %q — the printed number must be the DERIVED operation count:\n%s\npatch:\n%s",
				wantLine, stdout, patch)
		}
	})

	t.Run("derived-count-differs-from-the-file-counter", func(t *testing.T) {
		tmp := t.TempDir()
		s, err := store.Init(tmp)
		if err != nil {
			t.Fatal(err)
		}
		feature, err := s.AddFeature(store.AddFeatureInput{Title: "Dup", Slug: "dup", Request: "dup"})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmp, "bin.dat"), []byte("c\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		// ONE record. The `diff --git` line inside the binary stanza is
		// stanza payload, not a record boundary — the grammar tracks the
		// stanza and the display counter does not.
		patch := "diff --git a/bin.dat b/bin.dat\nindex 1..2 100644\nGIT binary patch\n" +
			"diff --git a/not-a-record.txt b/not-a-record.txt\n\n"

		obs := patchobs.Observe(patchobs.Input{
			Producer: patchobs.ProducerRecord, RepoRoot: tmp, Slug: feature.Slug,
			Patch: patch, PatchPresent: true, PreimageRef: "HEAD",
			Capture: patchobs.CaptureDescriptor{Mode: patchobs.CaptureModeWorkingTreeAll},
		})
		outcome, err := workflow.AutogenRecipeForRecord(s, obs, true, false)
		if err != nil {
			t.Fatalf("AutogenRecipeForRecord: %v", err)
		}
		if outcome.Action != workflow.AutogenSkipped {
			t.Fatalf("action = %q, want skipped for an unsupported binary effect", outcome.Action)
		}
		if outcome.Operations != 0 {
			t.Fatalf("derived operation count = %d, want 0 (no partial recipe)", outcome.Operations)
		}
		if got := countPatchFiles(patch); got != 2 {
			t.Fatalf("countPatchFiles = %d, want 2 — the row needs the counter to over-count", got)
		}
		if countPatchFiles(patch) == outcome.Operations {
			t.Fatal("binary file counter must not agree with the withheld operation count")
		}
	})
}

// TestS1EditReturnsTheEditorError is the P7 contract (§6.2, ADR-036 D2):
// an editor failure is REPORTED rather than discarded, and the after
// snapshot is taken regardless — a failed editor that still saved must
// not leave a mutated bound artifact behind an exit code of 0.
func TestS1EditReturnsTheEditorError(t *testing.T) {
	tmp := t.TempDir()
	gitInitTestRepo(t, tmp)
	runCmd("init", "--path", tmp)
	slug := "s1-edit-error"
	runCmd("add", "--path", tmp, "--slug", slug, "S1 edit error")

	artifactsDir := filepath.Join(tmp, ".tpatch", "features", slug, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(artifactsDir, "apply-recipe.json")
	body := "{\"feature\":\"" + slug + "\",\"operations\":[]}\n"
	if err := os.WriteFile(canonical, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	// An `$EDITOR` that does not exist cannot start, so c.Run fails.
	t.Setenv("EDITOR", filepath.Join(tmp, "no-such-editor-binary"))

	_, stderr, code := runCmdExit("edit", "--path", tmp, slug, "artifacts/apply-recipe.json")
	if code == 0 {
		t.Fatalf("a failed editor must not exit 0; stderr=%s", stderr)
	}
	after, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != body {
		t.Fatalf("no byte should have changed:\n got %q\nwant %q", after, body)
	}
	rgaS0AssertNoCLICoverageArtifact(t, tmp, slug)
}

// TestS1EditWithoutEditorStillExitsZero pins the other half of the same
// rule: an unset `$EDITOR` is not an error and not an event. The refactor
// that made openInEditor return its error must not turn "no editor
// configured" into a failure.
func TestS1EditWithoutEditorStillExitsZero(t *testing.T) {
	tmp := t.TempDir()
	gitInitTestRepo(t, tmp)
	runCmd("init", "--path", tmp)
	slug := "s1-edit-noeditor"
	runCmd("add", "--path", tmp, "--slug", slug, "S1 edit without editor")

	artifactsDir := filepath.Join(tmp, ".tpatch", "features", slug, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(artifactsDir, "apply-recipe.json")
	body := "{\"feature\":\"" + slug + "\",\"operations\":[]}\n"
	if err := os.WriteFile(canonical, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("EDITOR", "")

	stdout, stderr, code := runCmdExit("edit", "--path", tmp, slug, "artifacts/apply-recipe.json")
	if code != 0 {
		t.Fatalf("an unset $EDITOR is not a failure: code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "(set $EDITOR to review ") {
		t.Fatalf("the pointer line is missing:\n%s", stdout)
	}
	after, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != body {
		t.Fatalf("no byte may change when $EDITOR is unset:\n got %q\nwant %q", after, body)
	}
}

func s1ReadFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

// ── the publication seam, from the CLI side ─────────────────────────────

// s1CLIRecorder collects everything the governed producers hand over.
type s1CLIRecorder struct{ got []patchobs.Observation }

func (r *s1CLIRecorder) Record(o patchobs.Observation) { r.got = append(r.got, o) }

func s1CLICollect(t *testing.T) *s1CLIRecorder {
	t.Helper()
	rec := &s1CLIRecorder{}
	restore := patchobs.SetRecorder(rec)
	t.Cleanup(restore)
	return rec
}

func s1CLISHA256(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// s1WriteEditorScript writes an executable `$EDITOR` stand-in that
// rewrites the file it is given, optionally failing afterwards.
//
// `printf '%b'` is used deliberately: it expands the backslash escapes in
// the BODY argument, so a multi-line artifact can be written from a
// single-line script without a heredoc.
func s1WriteEditorScript(t *testing.T, dir, name, body string, exitCode int) string {
	t.Helper()
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\nprintf '%b' \"" + body + "\" > \"$1\"\nexit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// ── P2: both categories publish, the empty capture does not ─────────────

// TestS1FeaturePatchObservesBothOfItsCategories is P2's timing row
// (ADR-036 §6.15 P2). `feature patch refresh|fixup` owns TWO events:
//
//   - the writing branch, whose first bound write is the canonical patch;
//   - the category-(c) checkpoint, where the capture matches the current
//     generation byte-for-byte and the command returns without writing.
//
// The observation is therefore taken after the classification is known
// and before the branch between them. Taking it inside the writing branch
// — the rev-0 shape — published only half the producer.
//
// An EMPTY capture is deliberately not an event: the producer captured
// nothing, so there is nothing to describe.
func TestS1FeaturePatchObservesBothOfItsCategories(t *testing.T) {
	tmp := modesFixture(t, "s1-p2")
	rgaS0CommitAll(t, tmp)
	modesWriteFile(t, tmp, "src/one.txt", "one\n")
	if _, stderr, code := runRecord(t, "record", "--path", tmp, "s1-p2", "--lenient"); code != 0 {
		t.Fatalf("record failed: %s", stderr)
	}
	recorded := readRecordedPatch(t, tmp, "s1-p2")

	t.Run("the-same-patch-checkpoint-publishes-without-writing", func(t *testing.T) {
		rec := s1CLICollect(t)
		before := s1ReadFile(t, filepath.Join(tmp, ".tpatch", "features", "s1-p2", "artifacts", "post-apply.patch"))

		_, stderr, code := runCmdExit("feature", "patch", "refresh", "--path", tmp, "s1-p2")
		if code != 0 {
			t.Fatalf("a byte-identical refresh must succeed: %s", stderr)
		}
		if !strings.Contains(stderr, "refresh skipped") {
			t.Fatalf("this row needs the non-writing branch; stderr = %q", stderr)
		}
		if got := s1ReadFile(t, filepath.Join(tmp, ".tpatch", "features", "s1-p2", "artifacts", "post-apply.patch")); got != before {
			t.Fatal("the checkpoint branch must not rewrite the canonical patch")
		}
		if len(rec.got) != 1 {
			t.Fatalf("the seam received %d observation(s), want 1 for the checkpoint branch", len(rec.got))
		}
		obs := rec.got[0]
		if obs.Producer != patchobs.ProducerFeaturePatch {
			t.Fatalf("producer = %q, want %q", obs.Producer, patchobs.ProducerFeaturePatch)
		}
		if !obs.PatchPresent || obs.PatchSHA256 != s1CLISHA256(recorded) {
			t.Fatalf("the checkpoint must bind the captured bytes (%q), got %q", s1CLISHA256(recorded), obs.PatchSHA256)
		}
	})

	t.Run("the-writing-branch-publishes-too", func(t *testing.T) {
		modesWriteFile(t, tmp, "src/one.txt", "one changed\n")
		rec := s1CLICollect(t)

		_, stderr, code := runCmdExit("feature", "patch", "refresh", "--path", tmp, "s1-p2")
		if code != 0 {
			t.Fatalf("feature patch refresh failed: %s", stderr)
		}
		if strings.Contains(stderr, "refresh skipped") {
			t.Fatalf("this row needs the WRITING branch; stderr = %q", stderr)
		}
		if len(rec.got) != 1 {
			t.Fatalf("the seam received %d observation(s), want 1 for the writing branch", len(rec.got))
		}
		written := readRecordedPatch(t, tmp, "s1-p2")
		if rec.got[0].PatchSHA256 != s1CLISHA256(written) {
			t.Fatal("the observation must bind the bytes the producer wrote")
		}
	})

	t.Run("an-empty-capture-is-not-an-event", func(t *testing.T) {
		rgaS0CommitAll(t, tmp)
		rec := s1CLICollect(t)

		_, stderr, code := runCmdExit("feature", "patch", "refresh", "--path", tmp, "s1-p2")
		if code != 0 {
			t.Fatalf("an empty capture is a no-op, not a failure: %s", stderr)
		}
		if len(rec.got) != 0 {
			t.Fatalf("an empty capture published %d observation(s); it describes nothing", len(rec.got))
		}
	})
}

// ── P7: only the two resolved bound artifacts are events ────────────────

// s1EditFixture builds a feature carrying both bound artifacts plus an
// unrelated one, and returns the repo root.
func s1EditFixture(t *testing.T, slug string) string {
	t.Helper()
	tmp := t.TempDir()
	gitInitTestRepo(t, tmp)
	runCmd("init", "--path", tmp)
	runCmd("add", "--path", tmp, "--slug", slug, "S1 edit fixture")

	artifacts := filepath.Join(tmp, ".tpatch", "features", slug, "artifacts")
	if err := os.MkdirAll(artifacts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifacts, "apply-recipe.json"),
		[]byte("{\"feature\":\""+slug+"\",\"operations\":[]}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifacts, "post-apply.patch"),
		[]byte("diff --git a/README.md b/README.md\nindex 1111111..2222222 100644\n"+
			"--- a/README.md\n+++ b/README.md\n@@ -1 +1 @@\n-# Test\n+# Edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".tpatch", "features", slug, "spec.md"), []byte("spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return tmp
}

// TestS1EditPublishesOnlyForResolvedBoundArtifacts is P7's trigger row
// (ADR-036 §6.15 P7). The governed trigger is the RESOLVED path:
// `<feature>/artifacts/apply-recipe.json` and
// `<feature>/artifacts/post-apply.patch`, never the token the operator
// typed. A feature-root decoy resolves first and shadows the canonical
// file, so editing it publishes nothing — the rev-0 shape published for
// the decoy and for `spec.md` alike.
func TestS1EditPublishesOnlyForResolvedBoundArtifacts(t *testing.T) {
	t.Run("editing-the-canonical-recipe-is-an-event", func(t *testing.T) {
		slug := "s1-p7-recipe"
		tmp := s1EditFixture(t, slug)
		canonicalPatch := s1ReadFile(t, filepath.Join(tmp, ".tpatch", "features", slug, "artifacts", "post-apply.patch"))
		t.Setenv("EDITOR", s1WriteEditorScript(t, tmp, "edit-ok.sh", "{}", 0))
		rec := s1CLICollect(t)

		_, stderr, code := runCmdExit("edit", "--path", tmp, slug, "artifacts/apply-recipe.json")
		if code != 0 {
			t.Fatalf("edit failed: %s", stderr)
		}
		if len(rec.got) != 1 {
			t.Fatalf("the seam received %d observation(s), want 1", len(rec.got))
		}
		obs := rec.got[0]
		if obs.Producer != patchobs.ProducerEdit {
			t.Fatalf("producer = %q, want %q", obs.Producer, patchobs.ProducerEdit)
		}
		if !obs.ArtifactMutated() {
			t.Fatalf("the before/after pair must prove the mutation: %+v / %+v", obs.ArtifactBefore, obs.ArtifactAfter)
		}
		if string(obs.ArtifactAfter.Bytes) != "{}" {
			t.Fatalf("after-snapshot bytes = %q, want the edited bytes", obs.ArtifactAfter.Bytes)
		}
		// Editing the recipe leaves the canonical patch alone, so the
		// observation binds whatever is currently readable there.
		if !obs.PatchPresent || obs.PatchSHA256 != s1CLISHA256(canonicalPatch) {
			t.Fatalf("a readable canonical patch must be bound, got present=%v sha=%q", obs.PatchPresent, obs.PatchSHA256)
		}
		if len(obs.Effects) != 1 || obs.Effects[0].Effect.Path != "README.md" {
			t.Fatalf("effects = %+v, want the canonical patch's own effect", obs.Effects)
		}
	})

	t.Run("editing-the-canonical-patch-binds-the-AFTER-bytes", func(t *testing.T) {
		slug := "s1-p7-patch"
		tmp := s1EditFixture(t, slug)
		edited := "diff --git a/EDITED.md b/EDITED.md\nnew file mode 100644\n--- /dev/null\n+++ b/EDITED.md\n@@ -0,0 +1 @@\n+edited\n"
		t.Setenv("EDITOR", s1WriteEditorScript(t, tmp, "edit-patch.sh", strings.ReplaceAll(edited, "\n", "\\n"), 0))
		rec := s1CLICollect(t)

		_, stderr, code := runCmdExit("edit", "--path", tmp, slug, "artifacts/post-apply.patch")
		if code != 0 {
			t.Fatalf("edit failed: %s", stderr)
		}
		if len(rec.got) != 1 {
			t.Fatalf("the seam received %d observation(s), want 1", len(rec.got))
		}
		obs := rec.got[0]
		onDisk := s1ReadFile(t, filepath.Join(tmp, ".tpatch", "features", slug, "artifacts", "post-apply.patch"))
		if onDisk != edited {
			t.Fatalf("the editor stand-in did not produce the expected bytes:\n got %q\nwant %q", onDisk, edited)
		}
		// The edited file IS the canonical patch, so the observation
		// binds the bytes that are now there — never the pre-edit ones,
		// and never "missing" for a patch that is really present.
		if !obs.PatchPresent {
			t.Fatal("the canonical patch is present; it must not be defaulted to missing")
		}
		if obs.PatchSHA256 != s1CLISHA256(edited) {
			t.Fatalf("patch digest = %q, want the AFTER bytes %q", obs.PatchSHA256, s1CLISHA256(edited))
		}
		if len(obs.Effects) != 1 || obs.Effects[0].Effect.Path != "EDITED.md" {
			t.Fatalf("effects = %+v, want the edited patch's effect", obs.Effects)
		}
	})

	t.Run("a-feature-root-decoy-is-not-an-event", func(t *testing.T) {
		slug := "s1-p7-decoy"
		tmp := s1EditFixture(t, slug)
		decoy := filepath.Join(tmp, ".tpatch", "features", slug, "apply-recipe.json")
		if err := os.WriteFile(decoy, []byte("decoy\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("EDITOR", s1WriteEditorScript(t, tmp, "edit-decoy.sh", "mutated", 0))
		rec := s1CLICollect(t)

		if _, stderr, code := runCmdExit("edit", "--path", tmp, slug, "apply-recipe.json"); code != 0 {
			t.Fatalf("edit failed: %s", stderr)
		}
		// The decoy really was the file that got edited — that is the
		// resolution precedence this row depends on.
		if got := s1ReadFile(t, decoy); got != "mutated" {
			t.Fatalf("the decoy was not the resolved path; it holds %q", got)
		}
		if len(rec.got) != 0 {
			t.Fatalf("editing a shadowing decoy published %d observation(s); it is not a bound artifact", len(rec.got))
		}
	})

	t.Run("an-unrelated-artifact-is-not-an-event", func(t *testing.T) {
		slug := "s1-p7-spec"
		tmp := s1EditFixture(t, slug)
		t.Setenv("EDITOR", s1WriteEditorScript(t, tmp, "edit-spec.sh", "mutated spec", 0))
		rec := s1CLICollect(t)

		if _, stderr, code := runCmdExit("edit", "--path", tmp, slug, "spec.md"); code != 0 {
			t.Fatalf("edit failed: %s", stderr)
		}
		if len(rec.got) != 0 {
			t.Fatalf("editing spec.md published %d observation(s)", len(rec.got))
		}
	})

	t.Run("an-editor-that-changed-nothing-is-not-an-event", func(t *testing.T) {
		slug := "s1-p7-noop"
		tmp := s1EditFixture(t, slug)
		// An editor that exits without touching the file.
		noop := filepath.Join(tmp, "edit-noop.sh")
		if err := os.WriteFile(noop, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("EDITOR", noop)
		rec := s1CLICollect(t)

		if _, stderr, code := runCmdExit("edit", "--path", tmp, slug, "artifacts/apply-recipe.json"); code != 0 {
			t.Fatalf("edit failed: %s", stderr)
		}
		if len(rec.got) != 0 {
			t.Fatalf("an inspect-and-quit edit published %d observation(s)", len(rec.got))
		}
	})

	t.Run("a-failed-editor-that-still-saved-publishes-and-then-fails", func(t *testing.T) {
		slug := "s1-p7-failed"
		tmp := s1EditFixture(t, slug)
		t.Setenv("EDITOR", s1WriteEditorScript(t, tmp, "edit-fail.sh", "{}", 3))
		rec := s1CLICollect(t)

		_, stderr, code := runCmdExit("edit", "--path", tmp, slug, "artifacts/apply-recipe.json")
		if code == 0 {
			t.Fatalf("a failing editor must not exit 0; stderr = %s", stderr)
		}
		if len(rec.got) != 1 {
			t.Fatalf("the seam received %d observation(s); a failed editor that saved is still a mutation", len(rec.got))
		}
		if string(rec.got[0].ArtifactAfter.Bytes) != "{}" {
			t.Fatalf("after-snapshot = %q, want the saved bytes", rec.got[0].ArtifactAfter.Bytes)
		}
	})
}

// TestS1EditBoundArtifactClassification is the unit-level companion to the
// row above: the classifier answers on the RESOLVED path, and neither a
// decoy nor an alternative spelling of the same file can fool it.
func TestS1EditBoundArtifactClassification(t *testing.T) {
	slug := "s1-classify"
	tmp := s1EditFixture(t, slug)
	s, err := store.Open(tmp)
	if err != nil {
		t.Fatal(err)
	}
	featureDir := filepath.Join(tmp, ".tpatch", "features", slug)
	artifacts := filepath.Join(featureDir, "artifacts")

	cases := []struct {
		name string
		path string
		want boundArtifactKind
	}{
		{"canonical-recipe", filepath.Join(artifacts, "apply-recipe.json"), boundArtifactRecipe},
		{"canonical-patch", filepath.Join(artifacts, "post-apply.patch"), boundArtifactPatch},
		{"uncleaned-spelling-of-the-canonical-recipe",
			filepath.Join(artifacts, "..", "artifacts", "apply-recipe.json"), boundArtifactRecipe},
		{"feature-root-decoy", filepath.Join(featureDir, "apply-recipe.json"), boundArtifactNone},
		{"unrelated-artifact", filepath.Join(featureDir, "spec.md"), boundArtifactNone},
		{"same-name-in-another-feature",
			filepath.Join(tmp, ".tpatch", "features", "other", "artifacts", "apply-recipe.json"), boundArtifactNone},
		{"outside-the-feature", filepath.Join(tmp, "apply-recipe.json"), boundArtifactNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyBoundArtifact(s, slug, tc.path); got != tc.want {
				t.Fatalf("classifyBoundArtifact(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// ── P6: the manual checkpoint ───────────────────────────────────────────

// TestS1ImplementManualIsACheckpointEvent covers ADR-036 D15 P6's second
// half: `implement --manual` does not AUTHOR a recipe, it accepts bytes
// somebody else wrote. That is still a governed producer event, and it
// binds the EXACT bytes the store validated.
//
// A refused artifact publishes nothing, and the other `--manual` phases
// are not producers at all.
func TestS1ImplementManualIsACheckpointEvent(t *testing.T) {
	fixture := func(t *testing.T, slug, recipe string) string {
		t.Helper()
		tmp := t.TempDir()
		gitInitTestRepo(t, tmp)
		runCmd("init", "--path", tmp)
		runCmd("add", "--path", tmp, "--slug", slug, "S1 manual implement")
		artifacts := filepath.Join(tmp, ".tpatch", "features", slug, "artifacts")
		if err := os.MkdirAll(artifacts, 0o755); err != nil {
			t.Fatal(err)
		}
		if recipe != "" {
			if err := os.WriteFile(filepath.Join(artifacts, "apply-recipe.json"), []byte(recipe), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return tmp
	}

	t.Run("a-valid-checkpoint-publishes-the-validated-bytes", func(t *testing.T) {
		slug := "s1-manual-ok"
		// Deliberately NOT the shape RunImplement would write: the
		// checkpoint binds what the operator authored, byte for byte.
		recipe := "{ \"feature\":\"" + slug + "\", \"operations\":[] }\n"
		tmp := fixture(t, slug, recipe)
		rec := s1CLICollect(t)

		stdout, stderr, code := runCmdExit("implement", "--path", tmp, slug, "--manual")
		if code != 0 {
			t.Fatalf("implement --manual failed: %s", stderr)
		}
		if !strings.Contains(stdout, "advanced manually") {
			t.Fatalf("stdout missing the manual-advance line:\n%s", stdout)
		}
		if len(rec.got) != 1 {
			t.Fatalf("the seam received %d observation(s), want 1", len(rec.got))
		}
		obs := rec.got[0]
		if obs.Producer != patchobs.ProducerImplement {
			t.Fatalf("producer = %q, want %q", obs.Producer, patchobs.ProducerImplement)
		}
		if obs.Capture.Mode != patchobs.CaptureModeNoCapture {
			t.Fatalf("capture mode = %q, want no-capture", obs.Capture.Mode)
		}
		if obs.ArtifactAfter == nil || string(obs.ArtifactAfter.Bytes) != recipe {
			t.Fatalf("the checkpoint must bind the exact validated bytes %q, got %+v", recipe, obs.ArtifactAfter)
		}
		// A checkpoint does not rewrite what it checkpoints.
		if got := s1ReadFile(t, filepath.Join(tmp, ".tpatch", "features", slug, "artifacts", "apply-recipe.json")); got != recipe {
			t.Fatalf("the artifact changed:\n got %q\nwant %q", got, recipe)
		}
	})

	t.Run("a-refused-artifact-publishes-nothing", func(t *testing.T) {
		for _, tc := range []struct{ name, recipe string }{
			{"missing-artifact", ""},
			{"empty-artifact", "   \n"},
			{"invalid-json", "{not json"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				slug := "s1-manual-bad"
				tmp := fixture(t, slug, tc.recipe)
				rec := s1CLICollect(t)

				if _, _, code := runCmdExit("implement", "--path", tmp, slug, "--manual"); code == 0 {
					t.Fatal("a refused artifact must not advance state")
				}
				if len(rec.got) != 0 {
					t.Fatalf("a refused checkpoint published %d observation(s)", len(rec.got))
				}
			})
		}
	})

	t.Run("other-manual-phases-are-not-producers", func(t *testing.T) {
		slug := "s1-manual-analyze"
		tmp := fixture(t, slug, "")
		if err := os.WriteFile(filepath.Join(tmp, ".tpatch", "features", slug, "analysis.md"), []byte("analysis\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		rec := s1CLICollect(t)

		if _, stderr, code := runCmdExit("analyze", "--path", tmp, slug, "--manual"); code != 0 {
			t.Fatalf("analyze --manual failed: %s", stderr)
		}
		if len(rec.got) != 0 {
			t.Fatalf("analyze --manual published %d observation(s); it binds no governed artifact", len(rec.got))
		}
	})
}

// ── PI-3: every patch producer refuses before its first bound write ─────

// TestS1ObservePatchProducerReturnsThePreflightRefusal is the unit-level
// half of PI-3: the helper every CLI producer calls hands the observation
// to the seam AND returns the strict refusal, so the caller can return
// before it writes.
func TestS1ObservePatchProducerReturnsThePreflightRefusal(t *testing.T) {
	tmp := t.TempDir()
	gitInitTestRepo(t, tmp)
	runCmd("init", "--path", tmp)
	s, err := store.Open(tmp)
	if err != nil {
		t.Fatal(err)
	}

	rec := s1CLICollect(t)
	obs, err := observePatchProducer(patchobs.ProducerRecord, s, "demo", s1UnreadableCanonicalPatch,
		string(captureModeWorkingTreeAll), "", "", nil, nil)
	if err == nil {
		t.Fatal("an unreadable patch must return a preflight refusal")
	}
	if !strings.Contains(err.Error(), string(patchobs.ProducerRecord)) {
		t.Fatalf("the refusal must name the producer, got %v", err)
	}
	// The observation is still taken and still published: "the producer
	// looked and the grammar refused" is the record S1 owes.
	if len(rec.got) != 1 {
		t.Fatalf("the seam received %d observation(s), want 1 even on refusal", len(rec.got))
	}
	if rec.got[0].ParseRefusal == "" {
		t.Fatal("the refusal message must be retained on the observation")
	}
	if obs.ParseRefusal == "" || len(obs.Effects) != 0 {
		t.Fatalf("a refused parse invents no effects: %+v", obs)
	}

	// Wrong-input sensitivity: a readable capture returns no error.
	readable := "diff --git a/x.txt b/x.txt\nindex 1..2 100644\n@@ -1 +1 @@\n-a\n+b\n"
	if _, err := observePatchProducer(patchobs.ProducerRecord, s, "demo", readable,
		string(captureModeWorkingTreeAll), "", "", nil, nil); err != nil {
		t.Fatalf("a readable capture must not be refused: %v", err)
	}
}

// s1CLIProducerSites are the four CLI producers that write a canonical
// patch, with the file and function that owns each write.
var s1CLIProducerSites = []struct {
	Label string
	File  string
	Func  string
}{
	{Label: "P1 record", File: "internal/cli/cobra.go", Func: "recordCmd"},
	{Label: "P2 feature-patch-amend", File: "internal/cli/feature_patch.go", Func: "runFeaturePatchAmend"},
	{Label: "P4 cycle", File: "internal/cli/phase2.go", Func: "cycleCmd"},
	{Label: "P5 apply-done", File: "internal/cli/cobra.go", Func: "runApplyDone"},
}

// TestS1CLIProducersRefuseBeforeTheirFirstBoundWrite is PI-3's ordering
// proof for the CLI producers. Their runtime inputs come from `git diff`
// and cannot be made malformed from a test, so the guarantee is measured
// where it is decided: each producer binds the observation error and
// returns on it BEFORE its first bound write.
func TestS1CLIProducersRefuseBeforeTheirFirstBoundWrite(t *testing.T) {
	for _, site := range s1CLIProducerSites {
		t.Run(site.Label, func(t *testing.T) {
			src := rgaS0CLISource(t, site.File)
			if err := s1CheckProducerRefusesFirst(site.File, src, site.Func); err != nil {
				t.Fatalf("%s: %v", site.Label, err)
			}
		})
	}

	t.Run("sensitivity", func(t *testing.T) {
		src := rgaS0CLISource(t, "internal/cli/feature_patch.go")
		anchor := "\tif recipeObservation, obsErr = observePatchProducer(patchobs.ProducerFeaturePatch, s, slug, patch,\n" +
			"\t\tstring(captureModeWorkingTreeAll), \"\", \"\", nil, nil); obsErr != nil {\n" +
			"\t\treturn obsErr\n\t}\n"
		if !strings.Contains(src, anchor) {
			t.Fatalf("P2 observation anchor no longer present:\n%q", anchor)
		}
		for _, tc := range []struct{ name, replacement string }{
			{
				name:        "the-observation-is-removed",
				replacement: "",
			},
			{
				// The rev-0 shape for the error: taken, then dropped.
				name:        "the-preflight-error-is-discarded",
				replacement: "\tobservePatchProducer(patchobs.ProducerFeaturePatch, s, slug, patch,\n\t\tstring(captureModeWorkingTreeAll), \"\", \"\", nil, nil)\n",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				mutated := strings.Replace(src, anchor, tc.replacement, 1)
				if err := s1CheckProducerRefusesFirst("internal/cli/feature_patch.go", mutated, "runFeaturePatchAmend"); err == nil {
					t.Fatalf("guard did not catch mutation %q", tc.name)
				}
			})
		}

		t.Run("the-observation-moves-below-the-write", func(t *testing.T) {
			write := "\tif err := s.WriteArtifact(slug, \"post-apply.patch\", patch); err != nil {\n" +
				"\t\treturn fmt.Errorf(\"write post-apply.patch: %w\", err)\n\t}\n"
			if !strings.Contains(src, write) {
				t.Fatalf("P2 write anchor no longer present:\n%q", write)
			}
			moved := strings.Replace(src, anchor, "", 1)
			moved = strings.Replace(moved, write, write+anchor, 1)
			if err := s1CheckProducerRefusesFirst("internal/cli/feature_patch.go", moved, "runFeaturePatchAmend"); err == nil {
				t.Fatal("guard did not catch an observation taken after the write")
			}
		})
	})
}

// s1CheckProducerRefusesFirst asserts one producer takes its observation,
// RETURNS on the preflight error, and does both before its first bound
// write.
func s1CheckProducerRefusesFirst(fileName, src, fnName string) error {
	fn, err := rgaS0CLIFunc(fileName, src, fnName)
	if err != nil {
		return err
	}

	refusal := token.NoPos
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
			if !ok || rgaS0CLICallName(call) != "observePatchProducer" {
				continue
			}
			returns := false
			ast.Inspect(ifStmt.Body, func(inner ast.Node) bool {
				if _, ok := inner.(*ast.ReturnStmt); ok {
					returns = true
				}
				return true
			})
			if returns && (refusal == token.NoPos || call.Pos() < refusal) {
				refusal = call.Pos()
			}
		}
		return true
	})
	if refusal == token.NoPos {
		return fmt.Errorf("%s does not take an observation and return on its preflight error", fnName)
	}

	firstWrite := token.NoPos
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "WriteArtifact":
			// Only a BOUND artifact counts. A producer may legitimately
			// write an unrelated artifact (a spec, a note) before it
			// captures anything.
			if len(call.Args) < 2 {
				return true
			}
			artifact, isLit := rgaS0CLIStringLit(call.Args[1])
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
	if refusal > firstWrite {
		return fmt.Errorf("%s writes before it refuses an unreadable capture", fnName)
	}
	return nil
}

// s1BoundArtifacts is the closed set of artifacts coverage binds
// (ADR-036 §6.15). It mirrors the S0 bound-write inventory.
var s1BoundArtifacts = map[string]bool{
	"post-apply.patch":  true,
	"apply-recipe.json": true,
}
