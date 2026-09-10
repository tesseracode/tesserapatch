package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/store"
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

func TestRGAS5EarlierMutationInvalidatesCachedNoop(t *testing.T) {
	for _, tc := range []struct {
		name  string
		first RecipeOperation
	}{
		{"gated-write", RecipeOperation{Type: "write-file", Content: "A", PreimageHash: ptr(hashOf([]byte("B")))}},
		{"legacy-write", RecipeOperation{Type: "write-file", Content: "A"}},
		{"append", RecipeOperation{Type: "append-file", Content: "!"}},
		{"replace", RecipeOperation{Type: "replace-in-file", Search: "B", Replace: "A"}},
	} {
		for _, alias := range []string{"target.txt", "./target.txt", "sub/../target.txt", "hardlink.txt", "symlink.txt"} {
			t.Run(tc.name+"/"+alias, func(t *testing.T) {
				s := slice2Store(t)
				writeRepoFile(t, s, "target.txt", []byte("B"))
				target := filepath.Join(s.Root, "target.txt")
				switch alias {
				case "hardlink.txt":
					if err := os.Link(target, filepath.Join(s.Root, alias)); err != nil {
						t.Skipf("hard links unavailable: %v", err)
					}
				case "symlink.txt":
					if err := os.Symlink("target.txt", filepath.Join(s.Root, alias)); err != nil {
						t.Skipf("symbolic links unavailable: %v", err)
					}
				}
				first := tc.first
				first.Path = alias
				recipe := ApplyRecipe{Feature: "demo", Operations: []RecipeOperation{
					first,
					{Type: "write-file", Path: "target.txt", Content: "B", PreimageHash: ptr(hashOf([]byte("B")))},
				}}
				preview := DryRunRecipe(s, recipe)
				if !preview.Success || preview.Applied != 2 || preview.Skipped != 0 {
					t.Errorf("preview retained an invalidated no-write witness: %+v", preview)
				}
				before, err := os.ReadFile(target)
				if err != nil || string(before) != "B" {
					t.Fatal("preview changed the target")
				}
				result := ExecuteRecipe(s, recipe)
				got, err := os.ReadFile(target)
				if err != nil || !result.Success || result.Applied != 2 || result.Skipped != 0 || string(got) != "B" {
					t.Fatalf("later write was skipped after an earlier mutation: %+v final=%q err=%v", result, got, err)
				}
				if result.Messages[1] != "[write-file] target.txt: OK" {
					t.Fatalf("later write falsely reported no write: %v", result.Messages)
				}
			})
		}
	}
}

func TestRGAS5NoopRecheckUsesSequentialBytes(t *testing.T) {
	s := slice2Store(t)
	writeRepoFile(t, s, "target.txt", []byte("B"))
	recipe := ApplyRecipe{Feature: "demo", Operations: []RecipeOperation{
		{Type: "write-file", Path: "target.txt", Content: "A", PreimageHash: ptr(hashOf([]byte("B")))},
		{Type: "write-file", Path: "target.txt", Content: "B", PreimageHash: ptr(hashOf([]byte("B")))},
		{Type: "write-file", Path: "target.txt", Content: "B", PreimageHash: ptr(hashOf([]byte("B")))},
	}}
	result := ExecuteRecipe(s, recipe)
	if !result.Success || result.Applied != 3 || result.Skipped != 1 ||
		result.Messages[2] != "[write-file] target.txt: already present (exact postimage), no write" {
		t.Fatalf("no-write accounting ignored the actual sequential bytes: %+v", result)
	}
	got, err := os.ReadFile(filepath.Join(s.Root, "target.txt"))
	if err != nil || string(got) != "B" {
		t.Fatalf("sequential writes did not end in the declared final value: %q %v", got, err)
	}
}

func TestRGAS5NoopWitnessPreservedByNonmutatingPrefix(t *testing.T) {
	for _, first := range []RecipeOperation{
		{Type: "write-file", Content: "B", PreimageHash: ptr("")},
		{Type: "write-file", Content: "B"},
		{Type: "append-file", Content: ""},
		{Type: "replace-in-file", Search: "B", Replace: "B"},
		{Type: "ensure-directory", Path: "unrelated"},
		{Type: "write-file", Path: "unrelated.txt", Content: "unrelated", PreimageHash: ptr("")},
	} {
		t.Run(first.Type+"/"+first.Path, func(t *testing.T) {
			s := slice2Store(t)
			writeRepoFile(t, s, "target.txt", []byte("B"))
			wantSkipped := 1
			if first.Type == "write-file" && first.Path == "" && first.PreimageHash != nil {
				wantSkipped++
			}
			if first.Path == "" {
				first.Path = "target.txt"
			}
			recipe := ApplyRecipe{Feature: "demo", Operations: []RecipeOperation{
				first,
				{Type: "write-file", Path: "target.txt", Content: "B", PreimageHash: ptr(hashOf([]byte("C")))},
			}}
			result := ExecuteRecipe(s, recipe)
			if !result.Success || result.Applied != 2 || result.Skipped != wantSkipped {
				t.Fatalf("an unchanged postimage was treated as invalidated: %+v", result)
			}
		})
	}
}

func TestRGAS5PostimageOnlyWitnessSurvivesRestoringPrefix(t *testing.T) {
	s := slice2Store(t)
	writeRepoFile(t, s, "target.txt", []byte("B"))
	recipe := ApplyRecipe{Feature: "demo", Operations: []RecipeOperation{
		{Type: "append-file", Path: "target.txt", Content: "A"},
		{Type: "replace-in-file", Path: "target.txt", Search: "BA", Replace: "B"},
		{Type: "write-file", Path: "target.txt", Content: "B", PreimageHash: ptr("")},
	}}
	result := ExecuteRecipe(s, recipe)
	if !result.Success || result.Applied != 3 || result.Skipped != 1 {
		t.Fatalf("an overlapping prefix that restores the postimage was refused: %+v", result)
	}
	got, err := os.ReadFile(filepath.Join(s.Root, "target.txt"))
	if err != nil || string(got) != "B" {
		t.Fatalf("restored postimage changed: %q %v", got, err)
	}
}

func TestRGAS5InvalidatedNoopKeepsSupersessionSeverity(t *testing.T) {
	s := slice2Store(t)
	slice4SeedSupersession(t, s)
	writeRepoFile(t, s, "target.txt", []byte("B"))
	recipe := ApplyRecipe{Feature: "historical", Operations: []RecipeOperation{
		{Type: "append-file", Path: "target.txt", Content: "!"},
		{Type: "write-file", Path: "target.txt", Content: "B", PreimageHash: ptr("")},
	}}
	result := ExecuteRecipe(s, recipe)
	warnings := strings.Join(result.Warnings, "\n")
	if !result.Success || result.Applied != 2 || result.Skipped != 0 ||
		!strings.Contains(warnings, `superseded by "newer"`) ||
		!strings.Contains(warnings, "not a certification that explicit apply, coverage or replay is safe") {
		t.Fatalf("invalidated witness lost the existing supersession severity: %+v", result)
	}
}

func TestRGAS5PostimageOnlyInvalidationRefusesBeforeAnyWrite(t *testing.T) {
	for _, gate := range []string{"", hashOf([]byte("C"))} {
		for _, alias := range []string{"target.txt", "./target.txt", "sub/../target.txt", "hardlink.txt", "symlink.txt"} {
			for _, first := range []RecipeOperation{
				{Type: "write-file", Content: "A", PreimageHash: ptr(hashOf([]byte("B")))},
				{Type: "write-file", Content: "A"},
				{Type: "append-file", Content: "!"},
				{Type: "replace-in-file", Search: "B", Replace: "A"},
			} {
				t.Run(gate+"/"+alias+"/"+first.Type, func(t *testing.T) {
					s := slice2Store(t)
					writeRepoFile(t, s, "target.txt", []byte("B"))
					target := filepath.Join(s.Root, "target.txt")
					if alias == "hardlink.txt" {
						if err := os.Link(target, filepath.Join(s.Root, alias)); err != nil {
							t.Skipf("hard links unavailable: %v", err)
						}
					}
					if alias == "symlink.txt" {
						if err := os.Symlink("target.txt", filepath.Join(s.Root, alias)); err != nil {
							t.Skipf("symbolic links unavailable: %v", err)
						}
					}
					first.Path = alias
					recipe := ApplyRecipe{Feature: "demo", Operations: []RecipeOperation{
						{Type: "write-file", Path: "unrelated.txt", Content: "must not be written", PreimageHash: ptr("")},
						first,
						{Type: "write-file", Path: "target.txt", Content: "B", PreimageHash: ptr(gate)},
					}}
					for _, run := range []func(*store.Store, ApplyRecipe) RecipeExecResult{DryRunRecipe, ExecuteRecipe} {
						result := run(s, recipe)
						if result.Success || result.Applied != 0 || result.Skipped != 0 ||
							!strings.Contains(strings.Join(result.Errors, "\n"), "preceding operations invalidate") {
							t.Fatalf("invalidated postimage-only authority escaped precheck: %+v", result)
						}
						if got, err := os.ReadFile(target); err != nil || string(got) != "B" {
							t.Fatalf("prefix wrote before refusal: %q %v", got, err)
						}
						if _, err := os.Stat(filepath.Join(s.Root, "unrelated.txt")); !os.IsNotExist(err) {
							t.Fatalf("unrelated prefix operation executed: %v", err)
						}
					}
					pre := runWriteFilePreimagePrecheck(s, recipe)
					if len(pre.WrappedErrors) != 1 || !errors.Is(pre.WrappedErrors[0], ErrWriteFilePreimageMismatch) {
						t.Fatalf("ordered drift lost the production sentinel: %+v", pre)
					}
				})
			}
		}
	}
}

func TestRGAS5PrefixCannotRescueInitialRefusals(t *testing.T) {
	for _, refusal := range []string{"missing", "unreadable", "malformed", "uppercase", "unsafe"} {
		t.Run(refusal, func(t *testing.T) {
			s := slice2Store(t)
			op := RecipeOperation{Type: "write-file", Path: "target.txt", Content: "B", PreimageHash: ptr(hashOf([]byte("B")))}
			if refusal != "missing" {
				writeRepoFile(t, s, "target.txt", []byte("B"))
			}
			read := os.ReadFile
			switch refusal {
			case "unreadable":
				read = func(path string) ([]byte, error) {
					if filepath.Base(path) == "target.txt" {
						return nil, errors.New("injected prefix-precheck read failure")
					}
					return os.ReadFile(path)
				}
			case "malformed":
				op.PreimageHash = ptr("not-a-hash")
			case "uppercase":
				op.PreimageHash = ptr("sha256:" + strings.Repeat("A", 64))
			case "unsafe":
				op.Path = "../outside.txt"
			}
			recipe := ApplyRecipe{Feature: "demo", Operations: []RecipeOperation{
				{Type: "write-file", Path: "target.txt", Content: "B"},
				op,
			}}
			pre := runWriteFilePreimagePrecheckWithReader(s, recipe, read)
			if len(pre.Errors) == 0 || len(pre.AlreadyPresent) != 0 {
				t.Fatalf("prefix output rescued an initial %s refusal: %+v", refusal, pre)
			}
			if refusal == "unreadable" {
				if !strings.Contains(strings.Join(pre.Errors, "\n"), "injected prefix-precheck read failure") {
					t.Fatalf("read failure cause lost: %+v", pre)
				}
				return
			}
			result := ExecuteRecipe(s, recipe)
			if result.Success || result.Applied != 0 || result.Skipped != 0 {
				t.Fatalf("initial refusal occurred after mutation: %+v", result)
			}
		})
	}
}

func TestRGAS5PrefixModelsKnownErrorsAndCreatedBy(t *testing.T) {
	for _, first := range []RecipeOperation{
		{Type: "replace-in-file", Path: "target.txt", Search: "missing", Replace: "A"},
		{Type: "ensure-directory", Path: "target.txt"},
		{Type: "append-file", Path: "target.txt", Content: "A", CreatedBy: "undeclared"},
	} {
		t.Run(first.Type+"/"+first.CreatedBy+"/"+first.Path, func(t *testing.T) {
			s := createdByTestEnv(t, true, "parent", "child", store.DependencyKindHard)
			writeRepoFile(t, s, "target.txt", []byte("B"))
			recipe := ApplyRecipe{Feature: "child", Operations: []RecipeOperation{
				first,
				{Type: "write-file", Path: "target.txt", Content: "B", PreimageHash: ptr("")},
			}}
			for _, run := range []func(*store.Store, ApplyRecipe) RecipeExecResult{DryRunRecipe, ExecuteRecipe} {
				result := run(s, recipe)
				if result.Success || result.Applied != 1 || result.Skipped != 1 || len(result.Errors) != 1 {
					t.Fatalf("failed operation was projected as a mutation: %+v", result)
				}
			}
		})
	}
	t.Run("non-directory-ancestor", func(t *testing.T) {
		s := slice2Store(t)
		writeRepoFile(t, s, "target.txt", []byte("B"))
		recipe := ApplyRecipe{Feature: "demo", Operations: []RecipeOperation{
			{Type: "write-file", Path: "target.txt/child", Content: "A"},
			{Type: "write-file", Path: "target.txt", Content: "B", PreimageHash: ptr("")},
		}}
		result := ExecuteRecipe(s, recipe)
		if result.Success || result.Applied != 1 || result.Skipped != 1 {
			t.Fatalf("known non-directory failure invalidated the unaffected image: %+v", result)
		}
	})
	s := createdByTestEnv(t, true, "parent", "child", store.DependencyKindHard)
	writeRepoFile(t, s, "target.txt", []byte("B"))
	recipe := ApplyRecipe{Feature: "child", Operations: []RecipeOperation{
		{Type: "append-file", Path: "target.txt", Content: "A"},
		{Type: "replace-in-file", Path: "target.txt", Search: "BA", Replace: "B", CreatedBy: "undeclared"},
		{Type: "write-file", Path: "target.txt", Content: "B", PreimageHash: ptr("")},
	}}
	result := ExecuteRecipe(s, recipe)
	if result.Success || result.Applied != 0 || result.Skipped != 0 {
		t.Fatalf("a metadata-rejected replacement was assumed to restore the witness: %+v", result)
	}
	if got, err := os.ReadFile(filepath.Join(s.Root, "target.txt")); err != nil || string(got) != "B" {
		t.Fatalf("metadata-invalid restoring prefix mutated: %q %v", got, err)
	}
}

func TestRGAS5PrefixFirstReplacementAndAbsentAliases(t *testing.T) {
	s := slice2Store(t)
	writeRepoFile(t, s, "target.txt", []byte("BB"))
	recipe := ApplyRecipe{Feature: "demo", Operations: []RecipeOperation{
		{Type: "append-file", Path: "target.txt", Content: "BB"},
		{Type: "replace-in-file", Path: "target.txt", Search: "BB", Replace: "B"},
		{Type: "write-file", Path: "target.txt", Content: "BB", PreimageHash: ptr("")},
	}}
	if result := ExecuteRecipe(s, recipe); result.Success || result.Applied != 0 {
		t.Fatalf("projection replaced all matches instead of only the first: %+v", result)
	}
	for _, alias := range []string{"./new.txt", "sub/../new.txt", "dir-alias/new.txt"} {
		t.Run(alias, func(t *testing.T) {
			s := slice2Store(t)
			if strings.HasPrefix(alias, "dir-alias/") {
				if err := os.Symlink(".", filepath.Join(s.Root, "dir-alias")); err != nil {
					t.Skipf("symbolic links unavailable: %v", err)
				}
			}
			recipe := ApplyRecipe{Feature: "demo", Operations: []RecipeOperation{
				{Type: "write-file", Path: alias, Content: "B", PreimageHash: ptr("")},
				{Type: "append-file", Path: "new.txt", Content: "A"},
				{Type: "replace-in-file", Path: "new.txt", Search: "BA", Replace: "B"},
				{Type: "write-file", Path: "new.txt", Content: "B", PreimageHash: ptr("")},
			}}
			preview := DryRunRecipe(s, recipe)
			if !preview.Success || preview.Applied != 4 || preview.Skipped != 1 {
				t.Fatalf("absent alias preview ignored sequential creation: %+v", preview)
			}
			if _, err := os.Stat(filepath.Join(s.Root, "new.txt")); !os.IsNotExist(err) {
				t.Fatal("preview created a file")
			}
			result := ExecuteRecipe(s, recipe)
			if !result.Success || result.Applied != 4 || result.Skipped != 1 {
				t.Fatalf("absent aliases did not share the ordered image: %+v", result)
			}
		})
	}
}

func TestRGAS5RuntimeWitnessDivergence(t *testing.T) {
	for _, authorized := range []bool{false, true} {
		t.Run(fmt.Sprintf("write-authorized=%v", authorized), func(t *testing.T) {
			s := slice2Store(t)
			writeRepoFile(t, s, "target.txt", []byte("B"))
			gate := ""
			if authorized {
				gate = hashOf([]byte("B"))
			}
			recipe := ApplyRecipe{Feature: "demo", Operations: []RecipeOperation{
				{Type: "append-file", Path: "target.txt", Content: "A"},
				{Type: "replace-in-file", Path: "target.txt", Search: "BA", Replace: "B"},
				{Type: "write-file", Path: "target.txt", Content: "B", PreimageHash: ptr(gate)},
				{Type: "write-file", Path: "later.txt", Content: "later", PreimageHash: ptr("")},
			}}
			result := executeRecipeWithOperation(s, recipe, func(s *store.Store, slug string, op RecipeOperation) error {
				if op.Type == "replace-in-file" {
					op.Replace = "diverged"
				}
				return executeOperation(s, slug, op)
			})
			wantBody, wantApplied := "diverged", 2
			if authorized {
				wantBody, wantApplied = "B", 4
			}
			if result.Success != authorized || result.Applied != wantApplied || result.Skipped != 0 {
				t.Fatalf("runtime divergence silently skipped or gained write permission: %+v", result)
			}
			if got, err := os.ReadFile(filepath.Join(s.Root, "target.txt")); err != nil || string(got) != wantBody {
				t.Fatalf("runtime divergence result: %q %v", got, err)
			}
			if !authorized {
				if !strings.Contains(strings.Join(result.Errors, "\n"), "execution no longer matches") {
					t.Fatalf("runtime witness refusal not explained: %+v", result)
				}
				if _, err := os.Stat(filepath.Join(s.Root, "later.txt")); !os.IsNotExist(err) {
					t.Fatal("execution continued beyond the runtime witness refusal")
				}
			}
		})
	}
}

func TestRGAS5PrefixResourceGuard(t *testing.T) {
	s := slice2Store(t)
	writeRepoFile(t, s, "target.txt", []byte("B"))
	growth := strings.Repeat("A", recipePrefixMaxBytes)
	for _, first := range []RecipeOperation{
		{Type: "append-file", Path: "target.txt", Content: growth},
		{Type: "replace-in-file", Path: "target.txt", Search: "", Replace: growth},
	} {
		recipe := ApplyRecipe{Feature: "demo", Operations: []RecipeOperation{
			first,
			{Type: "write-file", Path: "target.txt", Content: "B", PreimageHash: ptr("")},
		}}
		result := ExecuteRecipe(s, recipe)
		if result.Success || result.Applied != 0 || !strings.Contains(strings.Join(result.Errors, "\n"), "8388608-byte proof limit") {
			t.Fatalf("oversized prefix escaped the production resource guard: %+v", result)
		}
		recipe.Operations[1].PreimageHash = ptr(hashOf([]byte("B")))
		if result := ExecuteRecipe(s, recipe); !result.Success || result.Applied != 2 || result.Skipped != 0 {
			t.Fatalf("oversized prefix revoked original initial-tree write permission: %+v", result)
		}
	}
	large := growth + "B"
	writeRepoFile(t, s, "large.txt", []byte(large))
	if _, err := readRecipePrefixImage(filepath.Join(s.Root, "large.txt")); !errors.Is(err, errRecipePrefixLimit) {
		t.Fatalf("oversized input was accepted by the production bounded reader: %v", err)
	}
	recipe := ApplyRecipe{Feature: "demo", Operations: []RecipeOperation{
		{Type: "write-file", Path: "large.txt", Content: large, PreimageHash: ptr("")},
	}}
	if result := ExecuteRecipe(s, recipe); result.Success || result.Applied != 0 {
		t.Fatalf("unproved large postimage-only operation reported success: %+v", result)
	}
	recipe.Operations[0].PreimageHash = ptr(hashOf([]byte(large)))
	if result := ExecuteRecipe(s, recipe); !result.Success || result.Applied != 1 || result.Skipped != 0 {
		t.Fatalf("optional proof limit revoked original write authority: %+v", result)
	}
	if got, _, err := replaceRecipeText("BB", RecipeOperation{Search: "B", Replace: ""}, 1); err != nil || got != "B" {
		t.Fatalf("bounded first replacement rejected a fitting output: %q %v", got, err)
	}
	if _, _, err := replaceRecipeText("BB", RecipeOperation{Search: "B", Replace: "BB"}, 2); !errors.Is(err, errRecipePrefixLimit) {
		t.Fatalf("production replacement growth guard accepted deliberate overflow: %v", err)
	}
}

func TestRGAS5PrefixRejectsUnsafeAndUnprovenAliases(t *testing.T) {
	outer := slice2Store(t)
	s, err := store.Init(filepath.Join(outer.Root, "repo"))
	if err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, outer, "outside.txt", []byte("B"))
	writeRepoFile(t, s, "target.txt", []byte("B"))
	writeRepoFile(t, s, "distinct.txt", []byte("B"))
	if sameRecipePrefixTarget(resolveRecipePrefixTarget(s.Root, "target.txt"), resolveRecipePrefixTarget(s.Root, "distinct.txt")) {
		t.Fatal("equal bytes on distinct files were accepted as physical identity")
	}
	if err := os.Symlink("../outside.txt", filepath.Join(s.Root, "escape")); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	if err := os.Symlink("missing.txt", filepath.Join(s.Root, "dangling")); err != nil {
		t.Fatal(err)
	}
	if got := resolveRecipePrefixTarget(s.Root, "escape"); !errors.Is(got.err, errRecipePrefixPathSafety) {
		t.Fatalf("physical escape accepted by production alias validator: %+v", got)
	}
	if got := resolveRecipePrefixTarget(s.Root, "dangling"); got.err == nil {
		t.Fatalf("dangling link was treated as a proven absent ordinary path: %+v", got)
	}
	for _, superseded := range []bool{false, true} {
		feature := "demo"
		if superseded {
			slice4SeedSupersession(t, s)
			feature = "historical"
		}
		recipe := ApplyRecipe{Feature: feature, Operations: []RecipeOperation{
			{Type: "append-file", Path: "escape", Content: "!"},
			{Type: "write-file", Path: "target.txt", Content: "B", PreimageHash: ptr("")},
		}}
		result := ExecuteRecipe(s, recipe)
		if result.Success || result.Applied != 0 || !strings.Contains(strings.Join(result.Errors, "\n"), "path safety") {
			t.Fatalf("unsafe prefix alias was ignored or downgraded: %+v", result)
		}
	}
	recipe := ApplyRecipe{Feature: "demo", Operations: []RecipeOperation{
		{Type: "write-file", Path: "dangling", Content: "A"},
		{Type: "write-file", Path: "target.txt", Content: "B", PreimageHash: ptr("")},
	}}
	if result := ExecuteRecipe(s, recipe); result.Success || result.Applied != 0 ||
		!strings.Contains(strings.Join(result.Errors, "\n"), "unproven") {
		t.Fatalf("unproved prefix alias received success-shaped fallback: %+v", result)
	}
}

func TestRGAS5AbsentTopologyUnprovedPreservesOriginalWrites(t *testing.T) {
	s := slice2Store(t)
	recipe := ApplyRecipe{Feature: "demo", Operations: []RecipeOperation{
		{Type: "ensure-directory", Path: "dir"},
		{Type: "write-file", Path: "dir/new.txt", Content: "B", PreimageHash: ptr("")},
		{Type: "write-file", Path: "dir/new.txt", Content: "B", PreimageHash: ptr("")},
	}}
	pre := runWriteFilePreimagePrecheck(s, recipe)
	if len(pre.Errors) != 0 || len(pre.AlreadyPresent) != 0 || !pre.WriteAuthorized[1] || !pre.WriteAuthorized[2] {
		t.Fatalf("unproved absent-target topology gained a skip or lost write permission: %+v", pre)
	}
	result := ExecuteRecipe(s, recipe)
	if !result.Success || result.Applied != 3 || result.Skipped != 0 {
		t.Fatalf("optional topology proof restricted original writes: %+v", result)
	}
	if got, err := os.ReadFile(filepath.Join(s.Root, "dir/new.txt")); err != nil || string(got) != "B" {
		t.Fatalf("ordinary authorized result changed: %q %v", got, err)
	}
}
