package intentpub

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
)

func TestPIB148PIB149PIB151RenameFinalGate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, string, *intentlockFixture, string)
		check  func(*testing.T, string, *intentlockFixture, string)
	}{
		{
			name: "PIB-148-final-symlink",
			mutate: func(t *testing.T, workspace string, fixture *intentlockFixture, rel string) {
				t.Helper()
				targetRel := filepath.ToSlash(filepath.Join(filepath.Dir(rel), "symlink-target"))
				rootWrite(t, fixture.authority, targetRel, []byte("target bytes\n"), 0o644)
				if err := fixture.withRoot(func(root *os.Root) error {
					return root.Symlink("symlink-target", rel)
				}); err != nil {
					t.Fatal(err)
				}
				fixture.targetBytes = []byte("target bytes\n")
			},
			check: func(t *testing.T, workspace string, fixture *intentlockFixture, rel string) {
				t.Helper()
				info, err := os.Lstat(filepath.Join(workspace, filepath.FromSlash(rel)))
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("final symlink changed: %v %v", info, err)
				}
				targetRel := filepath.ToSlash(filepath.Join(filepath.Dir(rel), "symlink-target"))
				if got := rootRead(t, fixture.authority, targetRel); !bytes.Equal(got, fixture.targetBytes) {
					t.Fatalf("symlink target changed: %q", got)
				}
			},
		},
		{
			name: "PIB-149-final-directory",
			mutate: func(t *testing.T, _ string, fixture *intentlockFixture, rel string) {
				t.Helper()
				if err := fixture.withRoot(func(root *os.Root) error {
					return root.Mkdir(rel, 0o755)
				}); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, workspace string, _ *intentlockFixture, rel string) {
				t.Helper()
				info, err := os.Lstat(filepath.Join(workspace, filepath.FromSlash(rel)))
				if err != nil || !info.IsDir() {
					t.Fatalf("final directory changed: %v %v", info, err)
				}
				entries, err := os.ReadDir(filepath.Join(workspace, filepath.FromSlash(rel)))
				if err != nil || len(entries) != 0 {
					t.Fatalf("final directory contents changed: %v %v", entries, err)
				}
			},
		},
		{
			name: "PIB-151-artifacts-symlink",
			mutate: func(t *testing.T, workspace string, fixture *intentlockFixture, rel string) {
				t.Helper()
				artifactsRel := filepath.ToSlash(filepath.Dir(rel))
				outside := filepath.Join(workspace, "outside-artifacts")
				if err := os.Mkdir(outside, 0o755); err != nil {
					t.Fatal(err)
				}
				marker := filepath.Join(outside, filepath.Base(rel))
				if err := os.WriteFile(marker, []byte("outside bytes\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				fixture.outsidePath = marker
				fixture.targetBytes = []byte("outside bytes\n")
				if err := fixture.withRoot(func(root *os.Root) error {
					if err := root.Rename(artifactsRel, artifactsRel+"-real"); err != nil {
						return err
					}
					return root.Symlink(outside, artifactsRel)
				}); err != nil {
					t.Fatal(err)
				}
				fixture.tempDir = artifactsRel + "-real"
			},
			check: func(t *testing.T, workspace string, fixture *intentlockFixture, rel string) {
				t.Helper()
				artifactsRel := filepath.ToSlash(filepath.Dir(rel))
				info, err := os.Lstat(filepath.Join(workspace, filepath.FromSlash(artifactsRel)))
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("artifacts symlink changed: %v %v", info, err)
				}
				got, err := os.ReadFile(fixture.outsidePath)
				if err != nil || !bytes.Equal(got, fixture.targetBytes) {
					t.Fatalf("outside target changed: %q %v", got, err)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace, authority := acquireWorkspace(t)
			rel := ".tpatch/features/rename-gate/artifacts/index.json"
			fixture := &intentlockFixture{
				authority: authority,
				tempDir:   filepath.ToSlash(filepath.Dir(rel)),
			}
			if err := authority.WithRoot(func(root *os.Root) error {
				return mkdirChain(NewRootOps(root), filepath.ToSlash(filepath.Dir(rel)))
			}); err != nil {
				t.Fatal(err)
			}
			restoreBeforeRename(t, func(index int) {
				if index != 2 {
					t.Fatalf("beforeRename index = %d, want 2", index)
				}
				test.mutate(t, workspace, fixture, rel)
			})
			oldAfterRename := afterRename
			afterRenameCalls := 0
			t.Cleanup(func() { afterRename = oldAfterRename })
			afterRename = func(int) { afterRenameCalls++ }

			result, err := DurableWrite(authority, WriteRequest{
				Rel:        rel,
				Data:       []byte("ours\n"),
				Mode:       0o644,
				Expected:   identityPointer(AbsentIdentity()),
				Indexed:    true,
				EntryIndex: 2,
				Role:       WriteRoleOrdinaryCanonical,
			}, Options{RandomHex12: fixedHex("abcabcabcabc")})
			var typed *Error
			if err == nil || !errors.As(err, &typed) || typed.ExitClass != 5 ||
				result.Committed || afterRenameCalls != 0 {
				t.Fatalf("rename-time refusal = result=%+v after=%d err=%v", result, afterRenameCalls, err)
			}
			test.check(t, workspace, fixture, rel)
			assertNoTemps(t, authority, fixture.tempDir)
		})
	}
}

func TestCanonicalStatusWriteUsesSameRenameFinalGate(t *testing.T) {
	_, authority := acquireWorkspace(t)
	rel := ".tpatch/features/control-gate/status.json"
	rootWrite(t, authority, rel, []byte("before\n"), 0o644)
	expected, err := CaptureIdentity(authority, rel, Options{})
	if err != nil {
		t.Fatal(err)
	}
	old := beforeStatusRename
	oldAfter := afterStatusRename
	afterCalls := 0
	t.Cleanup(func() {
		beforeStatusRename = old
		afterStatusRename = oldAfter
	})
	beforeStatusRename = func(got string) {
		if got != rel {
			t.Fatalf("control rel = %q", got)
		}
		if err := authority.WithRoot(func(root *os.Root) error {
			if err := root.Rename(rel, rel+".real"); err != nil {
				return err
			}
			return root.Symlink("status.json.real", rel)
		}); err != nil {
			t.Fatal(err)
		}
	}
	afterStatusRename = func(string) { afterCalls++ }

	result, err := DurableWrite(authority, WriteRequest{
		Rel:        rel,
		Data:       []byte("after\n"),
		Mode:       0o644,
		Expected:   identityPointer(expected),
		ArtifactID: ArtifactStatus,
		Role:       WriteRoleCanonicalStatus,
	}, Options{RandomHex12: fixedHex("defdefdefdef")})
	var typed *Error
	if err == nil || !errors.As(err, &typed) || typed.ExitClass != 5 ||
		result.Committed || afterCalls != 0 {
		t.Fatalf("status rename-time refusal = result=%+v after=%d err=%v", result, afterCalls, err)
	}
	if got := rootRead(t, authority, rel+".real"); string(got) != "before\n" {
		t.Fatalf("control preimage changed: %q", got)
	}
}

func TestRenameFinalGateRefusesSameBytesOnNewIdentity(t *testing.T) {
	_, authority := acquireWorkspace(t)
	rel := ".tpatch/features/control-gate/identity.json"
	rootWrite(t, authority, rel, []byte("same bytes\n"), 0o644)
	expected, err := CaptureIdentity(authority, rel, Options{})
	if err != nil {
		t.Fatal(err)
	}
	old := beforeControlWriteRename
	t.Cleanup(func() { beforeControlWriteRename = old })
	beforeControlWriteRename = func(string) {
		if err := authority.WithRoot(func(root *os.Root) error {
			if err := root.Rename(rel, rel+".old"); err != nil {
				return err
			}
			return writeRootFile(root, rel, []byte("same bytes\n"), 0o644)
		}); err != nil {
			t.Fatal(err)
		}
	}

	result, err := DurableWrite(authority, WriteRequest{
		Rel:      rel,
		Data:     []byte("ours\n"),
		Mode:     0o644,
		Expected: identityPointer(expected),
		Role:     WriteRoleControl,
	}, Options{RandomHex12: fixedHex("123123123123")})
	var typed *Error
	if err == nil || !errors.As(err, &typed) || typed.ExitClass != 5 || result.Committed {
		t.Fatalf("same-bytes identity refusal = result=%+v err=%v", result, err)
	}
	if got := rootRead(t, authority, rel); string(got) != "same bytes\n" {
		t.Fatalf("replacement identity was overwritten: %q", got)
	}
}

func TestRenameFinalGateUsesRolledBackTransactionOutcome(t *testing.T) {
	_, authority := acquireWorkspace(t)
	plan := stageCreatePlan(t, authority)
	entries := plan.Entries()
	if len(entries) < 2 {
		t.Fatalf("transaction fixture has %d entries", len(entries))
	}
	first := entries[0]
	targetRel := first.Rel + ".target"
	rootWrite(t, authority, targetRel, []byte("external\n"), 0o644)
	restoreBeforeRename(t, func(index int) {
		if index != 0 {
			return
		}
		if err := authority.WithRoot(func(root *os.Root) error {
			return root.Symlink(filepath.Base(targetRel), first.Rel)
		}); err != nil {
			t.Fatal(err)
		}
	})

	result, err := Execute(
		authority,
		plan,
		"0123456789abcdef",
		nil,
		Options{RandomHex12: sequenceHex()},
	)
	var typed *Error
	if err == nil || !errors.As(err, &typed) || typed.ExitClass != 5 ||
		result.Outcome != OutcomeRolledBack || result.ExitClass != 5 ||
		len(result.Published) != 0 {
		t.Fatalf("transaction final-gate outcome = result=%+v err=%v", result, err)
	}
	for _, entry := range entries[1:] {
		current := captureForTest(t, authority, entry.Rel)
		if !current.Equal(entry.Preimage) {
			t.Fatalf("later canonical entry %s changed: got=%+v want=%+v", entry.Rel, current, entry.Preimage)
		}
	}
	if rootExists(t, authority, JournalRel(testSlug)) {
		t.Fatal("rolled-back final-gate refusal retained a journal claim")
	}
	if got := rootRead(t, authority, targetRel); string(got) != "external\n" {
		t.Fatalf("symlink target changed: %q", got)
	}
}

func TestRenameFinalGateSensitivity(t *testing.T) {
	source, err := os.ReadFile("writer.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := requireRenameFinalGate(source); err != nil {
		t.Fatal(err)
	}
	mutated := bytes.Replace(source, []byte("if err := revalidateRenameTarget(ops, renamePreimage); err != nil {"),
		[]byte("if err := error(nil); err != nil {"), 1)
	if err := requireRenameFinalGate(mutated); err == nil {
		t.Fatal("removing the final rooted rename gate escaped the sensitivity guard")
	}
}

func requireRenameFinalGate(source []byte) error {
	text := string(source)
	hook := strings.Index(text, "if request.Indexed && beforeRename != nil")
	gate := strings.Index(text, "if err := revalidateRenameTarget(ops, renamePreimage); err != nil")
	rename := strings.Index(text, "if err := ops.Rename(tempRel, request.Rel); err != nil")
	if hook < 0 || gate <= hook || rename <= gate {
		return errors.New("rooted rename final gate is missing or out of order")
	}
	return nil
}

type intentlockFixture struct {
	authority   *intentlock.WorkspaceAuthority
	targetBytes []byte
	outsidePath string
	tempDir     string
}

func (fixture *intentlockFixture) withRoot(fn func(*os.Root) error) error {
	return fixture.authority.WithRoot(fn)
}

func restoreBeforeRename(t *testing.T, hook func(int)) {
	t.Helper()
	old := beforeRename
	t.Cleanup(func() { beforeRename = old })
	beforeRename = hook
}
