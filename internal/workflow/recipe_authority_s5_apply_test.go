package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRGAS5ExactPostimageAccounting(t *testing.T) {
	for _, gate := range []string{"", hashOf([]byte("before\n"))} {
		for _, body := range []string{"", "exact postimage\r\n"} {
			t.Run(fmt.Sprintf("gate=%q/body=%q", gate, body), func(t *testing.T) {
				s := slice2Store(t)
				writeRepoFile(t, s, "target.txt", []byte(body))
				path := filepath.Join(s.Root, "target.txt")
				unchangedTime := time.Unix(1000000000, 0)
				if err := os.Chtimes(path, unchangedTime, unchangedTime); err != nil {
					t.Fatal(err)
				}
				before, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				recipe := ApplyRecipe{Feature: "demo", Operations: []RecipeOperation{
					{Type: "write-file", Path: "target.txt", Content: body, PreimageHash: ptr(gate)},
				}}
				for _, run := range []func() RecipeExecResult{
					func() RecipeExecResult { return DryRunRecipe(s, recipe) },
					func() RecipeExecResult { return ExecuteRecipe(s, recipe) },
					func() RecipeExecResult { return ExecuteRecipe(s, recipe) },
				} {
					got := run()
					if !got.Success || got.Applied != 1 || got.Skipped != 1 || got.Operations != 1 ||
						len(got.Messages) != 1 || got.Messages[0] != "[write-file] target.txt: already present (exact postimage), no write" {
						t.Fatalf("already-present accounting: %+v", got)
					}
				}
				after, err := os.Stat(path)
				if err != nil || !before.ModTime().Equal(after.ModTime()) {
					t.Fatalf("already-present performed a write: %v", err)
				}
				got, err := os.ReadFile(path)
				if err != nil || string(got) != body {
					t.Fatal("already-present changed bytes")
				}
			})
		}
	}
}

func TestRGAS5ApplyClassificationsAndMutationControls(t *testing.T) {
	s := slice2Store(t)
	writeRepoFile(t, s, "target.txt", []byte("post\n"))
	type classifier func(string, string, int, RecipeOperation) (preimageCheckOutcome, string)
	validate := func(classify classifier) error {
		for _, tc := range []struct {
			gate *string
			post string
			want preimageCheckOutcome
		}{
			{ptr(""), "post\n", preimageAlreadyPresent},
			{ptr(hashOf([]byte("pre\n"))), "post\n", preimageAlreadyPresent},
			{ptr(""), "other\n", preimageRejected},
			{ptr(hashOf([]byte("pre\n"))), "other\n", preimageRejected},
			{ptr(hashOf([]byte("post\n"))), "other\n", preimageOK},
			{ptr("malformed"), "post\n", preimageRejected},
			{ptr("sha256:" + strings.Repeat("A", 64)), "post\n", preimageRejected},
			{nil, "post\n", preimageLegacyWarn},
		} {
			op := RecipeOperation{Type: "write-file", Path: "target.txt", Content: tc.post, PreimageHash: tc.gate, CreatedBy: "parent"}
			if got, _ := classify(s.Root, "demo", 0, op); got != tc.want {
				return fmt.Errorf("gate=%v post=%q: got %v want %v", tc.gate, tc.post, got, tc.want)
			}
		}
		return nil
	}
	if err := validate(checkWriteFilePreimage); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"classifier-postimage-branch-removed", "classifier-collision-branch-removed", "created-by-empty-preimage-exemption"} {
		t.Run(mutation, func(t *testing.T) {
			wrong := func(root, slug string, i int, op RecipeOperation) (preimageCheckOutcome, string) {
				out, msg := checkWriteFilePreimage(root, slug, i, op)
				switch mutation {
				case "classifier-postimage-branch-removed":
					if out == preimageAlreadyPresent {
						out = preimageRejected
					}
				case "classifier-collision-branch-removed":
					if op.PreimageHash != nil && *op.PreimageHash == "" && out == preimageRejected {
						out = preimageOK
					}
				case "created-by-empty-preimage-exemption":
					if op.CreatedBy != "" && op.PreimageHash != nil && *op.PreimageHash == "" {
						out = preimageOK
					}
				}
				return out, msg
			}
			if err := validate(wrong); err == nil {
				t.Fatal("the classification validator accepted its wrong-input mutation")
			}
		})
	}
}

func TestRGAS5UnreadableAndMissingTargetsStillRefuse(t *testing.T) {
	s := slice2Store(t)
	writeRepoFile(t, s, "target.txt", []byte("post\n"))
	for _, gate := range []string{"", hashOf([]byte("pre\n"))} {
		op := RecipeOperation{Type: "write-file", Path: "target.txt", Content: "post\n", PreimageHash: ptr(gate)}
		got, msg := checkWriteFilePreimageWithReader(s.Root, "demo", 0, op, func(string) ([]byte, error) {
			return nil, errors.New("injected read failure")
		})
		if got != preimageRejected || !strings.Contains(msg, "injected read failure") || strings.Contains(msg, "post\n") {
			t.Fatalf("unreadable target lost its cause or disclosed bytes: %v %s", got, msg)
		}
	}
	op := RecipeOperation{Type: "write-file", Path: "absent.txt", Content: "", PreimageHash: ptr(hashOf(nil))}
	if got, msg := checkWriteFilePreimage(s.Root, "demo", 0, op); got != preimageRejected || !strings.Contains(msg, "missing") {
		t.Fatalf("missing target became an empty postimage: %v %s", got, msg)
	}
}

func TestRGAS5NoopDoesNotBypassAtomicityOrPathSafety(t *testing.T) {
	for _, unsafe := range []bool{false, true} {
		s := slice2Store(t)
		writeRepoFile(t, s, "already.txt", []byte("post\n"))
		writeRepoFile(t, s, "applicable.txt", []byte("pre\n"))
		bad := RecipeOperation{Type: "write-file", Path: "missing.txt", Content: "post\n", PreimageHash: ptr(hashOf([]byte("pre\n")))}
		feature := "demo"
		if unsafe {
			slice4SeedSupersession(t, s)
			feature = "historical"
			bad.Path = "../outside.txt"
		}
		recipe := ApplyRecipe{Feature: feature, Operations: []RecipeOperation{
			{Type: "write-file", Path: "already.txt", Content: "post\n", PreimageHash: ptr("")},
			{Type: "write-file", Path: "applicable.txt", Content: "post\n", PreimageHash: ptr(hashOf([]byte("pre\n")))},
			bad,
		}}
		got := ExecuteRecipe(s, recipe)
		if got.Success || got.Applied != 0 || got.Skipped != 0 {
			t.Fatalf("precheck did not refuse before every operation: %+v", got)
		}
		data, err := os.ReadFile(filepath.Join(s.Root, "applicable.txt"))
		if err != nil || string(data) != "pre\n" {
			t.Fatal("earlier applicable operation was written")
		}
		if unsafe && !strings.Contains(strings.Join(got.Errors, "\n"), "path safety") {
			t.Fatal("supersession downgraded a path-safety refusal")
		}
	}
}

func TestRGAS5SupersededDriftIsAuditNotSafety(t *testing.T) {
	s := slice2Store(t)
	slice4SeedSupersession(t, s)
	writeRepoFile(t, s, "target.txt", []byte("later\n"))
	recipe := ApplyRecipe{Feature: "historical", Operations: []RecipeOperation{
		{Type: "write-file", Path: "target.txt", Content: "historical\n", PreimageHash: ptr(hashOf([]byte("pre\n")))},
	}}
	got := ExecuteRecipe(s, recipe)
	warnings := strings.Join(got.Warnings, "\n")
	if !got.Success || got.Applied != 1 || got.Skipped != 0 ||
		!strings.Contains(warnings, `superseded by "newer"`) ||
		!strings.Contains(warnings, "audit signal, not a certification that explicit apply, coverage or replay is safe") {
		t.Fatalf("supersession severity or safety boundary changed: %+v", got)
	}
}
