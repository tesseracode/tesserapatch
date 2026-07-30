package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// slice2Store builds a fresh store rooted at a t.TempDir and returns it.
// The store is initialized empty; callers seed feature slugs and files
// as the individual scenarios require. Kept local to Slice 2 tests so
// no other test package is coupled to this helper's shape.
func slice2Store(t *testing.T) *store.Store {
	t.Helper()
	tmp := t.TempDir()
	s, err := store.Init(tmp)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	return s
}

// ptr returns *string(&v). Test-only convenience for setting the
// v0.12.0 Wave β `PreimageHash *string` field to an explicit value
// (including "" for the new-file gate). Kept local to Slice 2 tests.
func ptr(v string) *string { return &v }

// hashOf returns the ADR-029 D1 display form (`sha256:<64 lowercase hex>`)
// of the given byte slice. Used to construct matching preimages in tests.
func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// writeRepoFile writes bytes to repoRoot/rel, creating parent dirs.
func writeRepoFile(t *testing.T, s *store.Store, rel string, body []byte) {
	t.Helper()
	full := filepath.Join(s.Root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, body, 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// TestSlice2_PreimageMatch — PRD AC-1: "a `write-file` with `preimage_hash`
// matching the current file applies successfully."
func TestSlice2_PreimageMatch(t *testing.T) {
	s := slice2Store(t)
	pre := []byte("old content\n")
	writeRepoFile(t, s, "src/a.txt", pre)

	recipe := ApplyRecipe{
		Feature: "demo",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(hashOf(pre)), Content: "new content\n"},
		},
	}
	result := ExecuteRecipe(s, recipe)
	if !result.Success {
		t.Fatalf("expected success, got errors: %v", result.Errors)
	}
	got, _ := os.ReadFile(filepath.Join(s.Root, "src/a.txt"))
	if string(got) != "new content\n" {
		t.Errorf("write not applied: got %q", got)
	}
}

// TestSlice2_PreimageMismatch — PRD AC-2: "a `write-file` with a differing
// current file hash refuses before writing and reports recipe drift."
func TestSlice2_PreimageMismatch(t *testing.T) {
	s := slice2Store(t)
	pre := []byte("old content\n")
	writeRepoFile(t, s, "src/a.txt", pre)

	recipe := ApplyRecipe{
		Feature: "demo",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(hashOf([]byte("different"))), Content: "new content\n"},
		},
	}
	result := ExecuteRecipe(s, recipe)
	if result.Success {
		t.Fatalf("expected failure; got success with %d applied", result.Applied)
	}
	if len(result.Errors) == 0 {
		t.Fatalf("expected at least one error")
	}
	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "recipe drift") || !strings.Contains(joined, "src/a.txt") {
		t.Errorf("error should name recipe drift + path; got: %s", joined)
	}
	// D3 all-or-nothing: the file must be unchanged.
	got, _ := os.ReadFile(filepath.Join(s.Root, "src/a.txt"))
	if string(got) != "old content\n" {
		t.Errorf("D3 all-or-nothing violated: file was modified after mismatch; got %q", got)
	}
}

// TestSlice2_ExpectedHashMissingFile — PRD AC-3: "a `write-file` with
// non-empty `preimage_hash` refuses when the target file is absent."
func TestSlice2_ExpectedHashMissingFile(t *testing.T) {
	s := slice2Store(t)
	// Ensure parent dir exists so we don't fail on that basis instead.
	if err := os.MkdirAll(filepath.Join(s.Root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	recipe := ApplyRecipe{
		Feature: "demo",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(hashOf([]byte("x"))), Content: "new"},
		},
	}
	result := ExecuteRecipe(s, recipe)
	if result.Success {
		t.Fatalf("expected failure for missing file with non-empty preimage_hash")
	}
	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "missing") {
		t.Errorf("error message should mention the missing file; got: %s", joined)
	}
}

// TestSlice2_EmptyPreimageNewFile — PRD AC-4: "a `write-file` with
// `preimage_hash: \"\"` succeeds when the target path does not exist."
func TestSlice2_EmptyPreimageNewFile(t *testing.T) {
	s := slice2Store(t)
	// Ensure parent dir exists so write-file itself can proceed.
	if err := os.MkdirAll(filepath.Join(s.Root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	recipe := ApplyRecipe{
		Feature: "demo",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/new.txt", PreimageHash: ptr(""), Content: "hi\n"},
		},
	}
	result := ExecuteRecipe(s, recipe)
	if !result.Success {
		t.Fatalf("expected new-file empty-preimage to succeed; errors: %v", result.Errors)
	}
	got, _ := os.ReadFile(filepath.Join(s.Root, "src/new.txt"))
	if string(got) != "hi\n" {
		t.Errorf("new file not written: got %q", got)
	}
}

// TestSlice2_EmptyPreimageCollision — PRD AC-5: "a `write-file` with
// `preimage_hash: \"\"` refuses when the target path already exists."
func TestSlice2_EmptyPreimageCollision(t *testing.T) {
	s := slice2Store(t)
	writeRepoFile(t, s, "src/a.txt", []byte("existing\n"))

	recipe := ApplyRecipe{
		Feature: "demo",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(""), Content: "clobber\n"},
		},
	}
	result := ExecuteRecipe(s, recipe)
	if result.Success {
		t.Fatalf("expected new-file empty-preimage against existing file to refuse")
	}
	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "new-file collision") {
		t.Errorf("error should name new-file collision; got: %s", joined)
	}
	// D3 all-or-nothing: original contents preserved.
	got, _ := os.ReadFile(filepath.Join(s.Root, "src/a.txt"))
	if string(got) != "existing\n" {
		t.Errorf("collision-guard violated: file was modified; got %q", got)
	}
}

// TestSlice2_AtomicPrecheck — PRD AC-6 + ADR-029 D3: "if any operation
// precondition fails, no earlier operation in that recipe is written."
// Puts a good write-file BEFORE a bad one and verifies the good op did
// NOT run.
func TestSlice2_AtomicPrecheck(t *testing.T) {
	s := slice2Store(t)
	pre := []byte("preimage\n")
	writeRepoFile(t, s, "src/good.txt", pre)
	// src/bad.txt does not exist but recipe claims a preimage → will fail.
	if err := os.MkdirAll(filepath.Join(s.Root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}

	recipe := ApplyRecipe{
		Feature: "demo",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/good.txt", PreimageHash: ptr(hashOf(pre)), Content: "new-good\n"},
			{Type: "write-file", Path: "src/bad.txt", PreimageHash: ptr(hashOf([]byte("wrong"))), Content: "new-bad\n"},
		},
	}
	result := ExecuteRecipe(s, recipe)
	if result.Success {
		t.Fatalf("expected failure because op[1] precondition fails")
	}
	// Good op MUST NOT have executed — D3 all-or-nothing.
	got, _ := os.ReadFile(filepath.Join(s.Root, "src/good.txt"))
	if string(got) != "preimage\n" {
		t.Errorf("D3 all-or-nothing violated: op[0] was written before op[1] precheck failed; got %q", got)
	}
	// Bad op MUST NOT have executed either.
	if _, err := os.Stat(filepath.Join(s.Root, "src/bad.txt")); err == nil {
		t.Errorf("D3 all-or-nothing violated: op[1] target was created despite precheck failure")
	}
}

// TestSlice2_LegacyRecipeCompat — PRD AC-11 + ADR-029 D4: "pre-preimage-
// hash `write-file` recipes still apply in v1 with a warning."
// Legacy = nil PreimageHash pointer (JSON field omitted).
func TestSlice2_LegacyRecipeCompat(t *testing.T) {
	s := slice2Store(t)
	writeRepoFile(t, s, "src/a.txt", []byte("legacy-existing\n"))

	recipe := ApplyRecipe{
		Feature: "demo",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: nil, Content: "legacy-new\n"},
		},
	}
	result := ExecuteRecipe(s, recipe)
	if !result.Success {
		t.Fatalf("legacy nil-preimage recipe must still apply; errors: %v", result.Errors)
	}
	if len(result.Warnings) == 0 {
		t.Errorf("legacy nil-preimage recipe must emit a warning per ADR-029 D4")
	} else {
		joined := strings.Join(result.Warnings, "\n")
		if !strings.Contains(joined, "legacy") || !strings.Contains(joined, "preimage_hash") {
			t.Errorf("legacy warning should mention 'legacy' and 'preimage_hash'; got: %s", joined)
		}
	}
	got, _ := os.ReadFile(filepath.Join(s.Root, "src/a.txt"))
	if string(got) != "legacy-new\n" {
		t.Errorf("legacy write did not apply: got %q", got)
	}
}

// TestSlice2_MalformedPreimageRejected exercises ADR-029 D1 canonical
// form: uppercase hex, missing prefix, wrong length all refuse before
// mutation. Prevents "sHA256:..." or "sha256:ABCD..." from silently
// passing the precheck.
func TestSlice2_MalformedPreimageRejected(t *testing.T) {
	s := slice2Store(t)
	writeRepoFile(t, s, "src/a.txt", []byte("x"))

	cases := []struct {
		name    string
		hash    string
		wantSub string
	}{
		{"no-prefix", strings.Repeat("a", 64), "malformed preimage_hash"},
		{"wrong-len", "sha256:abc", "malformed preimage_hash"},
		{"uppercase-hex", "sha256:" + strings.Repeat("A", 64), "lowercase"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recipe := ApplyRecipe{
				Feature: "demo",
				Operations: []RecipeOperation{
					{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(tc.hash), Content: "n"},
				},
			}
			result := ExecuteRecipe(s, recipe)
			if result.Success {
				t.Fatalf("expected failure for %s", tc.name)
			}
			joined := strings.Join(result.Errors, "\n")
			if !strings.Contains(joined, tc.wantSub) {
				t.Errorf("expected error to mention %q; got: %s", tc.wantSub, joined)
			}
		})
	}
}

// TestSlice2_NonWriteFileOpsIgnorePreimage confirms that ADR-029 D1
// applicability ("`write-file` only") holds at runtime: replace-in-file,
// append-file, ensure-directory ops carrying a rogue preimage_hash
// value are unaffected by the Slice 2 precheck.
func TestSlice2_NonWriteFileOpsIgnorePreimage(t *testing.T) {
	s := slice2Store(t)
	writeRepoFile(t, s, "src/a.txt", []byte("hello world\n"))

	// A rogue PreimageHash pointer on a non-write-file op must NOT
	// gate execution (it should be silently ignored by the precheck).
	recipe := ApplyRecipe{
		Feature: "demo",
		Operations: []RecipeOperation{
			{Type: "replace-in-file", Path: "src/a.txt", PreimageHash: ptr(hashOf([]byte("stale"))), Search: "hello", Replace: "hi"},
		},
	}
	result := ExecuteRecipe(s, recipe)
	if !result.Success {
		t.Fatalf("non-write-file op must ignore preimage_hash; errors: %v", result.Errors)
	}
}

// TestSlice2_SentinelErrorsExist locks in the sentinel export contract.
// Downstream Wave γ + external supervisor rev tooling matches on these
// two names via errors.Is; renaming or removing them is a breaking
// change.
func TestSlice2_SentinelErrorsExist(t *testing.T) {
	if ErrWriteFilePreimageMismatch == nil {
		t.Error("ErrWriteFilePreimageMismatch sentinel must be exported")
	}
	if ErrWriteFileLaterTouch == nil {
		t.Error("ErrWriteFileLaterTouch sentinel must be exported")
	}
	// errors.Is on itself must be true (sanity — package-level sentinels).
	if !errors.Is(ErrWriteFilePreimageMismatch, ErrWriteFilePreimageMismatch) {
		t.Error("sentinel identity broken")
	}
}

// TestSlice2_DryRunSurfacesPreimageErrors ensures DryRunRecipe also
// short-circuits under the precheck so `tpatch apply --dry-run` shows
// the same drift verdict without mutating disk (parity with execute).
func TestSlice2_DryRunSurfacesPreimageErrors(t *testing.T) {
	s := slice2Store(t)
	writeRepoFile(t, s, "src/a.txt", []byte("current\n"))

	recipe := ApplyRecipe{
		Feature: "demo",
		Operations: []RecipeOperation{
			{Type: "write-file", Path: "src/a.txt", PreimageHash: ptr(hashOf([]byte("stale"))), Content: "n"},
		},
	}
	result := DryRunRecipe(s, recipe)
	if result.Success {
		t.Fatalf("dry-run must also fail on preimage mismatch")
	}
	got, _ := os.ReadFile(filepath.Join(s.Root, "src/a.txt"))
	if string(got) != "current\n" {
		t.Errorf("dry-run must not mutate disk; got %q", got)
	}
}
