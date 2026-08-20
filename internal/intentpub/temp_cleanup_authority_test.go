//go:build (linux && !android && (amd64 || arm64)) || (darwin && !ios)

package intentpub

import (
	"bytes"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
)

func TestTempCleanupAuthorityNeverUnlinksUnprovenBasename(t *testing.T) {
	if !descriptorTempCleanupSupported() {
		t.Skip("descriptor-relative temporary cleanup is unavailable")
	}

	t.Run("stale-parent-same-basename", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		directory := ".tpatch/features/temp-authority/current"
		staleDirectory := ".tpatch/features/temp-authority/stale"
		rootMkdirAll(t, authority, directory, 0o755)
		rootMkdirAll(t, authority, staleDirectory, 0o755)
		rel := directory + "/value.json"
		tempBase := ".value.json.tmp-111111111111"
		staleTemp := staleDirectory + "/" + tempBase
		rootWrite(t, authority, staleTemp, []byte("unrelated\n"), 0o600)

		result, err := DurableWrite(authority, WriteRequest{
			Rel:  rel,
			Data: []byte("owned\n"),
			Mode: 0o644,
			Role: WriteRoleOrdinaryCanonical,
		}, Options{
			RandomHex12: fixedHex("111111111111"),
			RootOpsFactory: func(root *os.Root) RootOps {
				return &staleParentRootOps{
					RootOps:        NewRootOps(root),
					directory:      directory,
					staleDirectory: staleDirectory,
				}
			},
		})
		assertCleanupAuthorityFailure(t, result, err)
		if got := rootRead(t, authority, staleTemp); string(got) != "unrelated\n" {
			t.Fatalf("stale-parent unrelated basename changed: %q", got)
		}
		currentTemp := directory + "/" + tempBase
		if !rootExists(t, authority, currentTemp) {
			t.Fatal("unproven current-parent temporary evidence was removed")
		}
		if rootExists(t, authority, rel) {
			t.Fatal("destination was published after parent-authority mismatch")
		}
	})

	t.Run("ancestor-swap-removes-only-proven-temp", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		directory := ".tpatch/features/temp-authority/swapped"
		oldDirectory := directory + "-old"
		rootMkdirAll(t, authority, directory, 0o755)
		rel := directory + "/value.json"
		tempBase := ".value.json.tmp-222222222222"
		restoreBeforeRename(t, func(int) {
			if err := authority.WithRoot(func(root *os.Root) error {
				if err := root.Rename(directory, oldDirectory); err != nil {
					return err
				}
				if err := root.Mkdir(directory, 0o755); err != nil {
					return err
				}
				return writeRootFile(root, directory+"/"+tempBase, []byte("unrelated\n"), 0o600)
			}); err != nil {
				t.Fatal(err)
			}
		})

		result, err := DurableWrite(authority, WriteRequest{
			Rel:        rel,
			Data:       []byte("owned\n"),
			Mode:       0o644,
			Indexed:    true,
			EntryIndex: 0,
			Role:       WriteRoleOrdinaryCanonical,
		}, Options{RandomHex12: fixedHex("222222222222")})
		var typed *Error
		if !errors.As(err, &typed) || typed.ExitClass != 5 || result.Committed {
			t.Fatalf("ancestor swap = result=%+v err=%v", result, err)
		}
		if rootExists(t, authority, oldDirectory+"/"+tempBase) {
			t.Fatal("proven temporary remained in the retained old parent")
		}
		if got := rootRead(t, authority, directory+"/"+tempBase); string(got) != "unrelated\n" {
			t.Fatalf("new-parent unrelated basename changed: %q", got)
		}
		if rootExists(t, authority, rel) {
			t.Fatal("destination was published through the replacement parent")
		}
	})

	t.Run("replaced-temp-basename-preserves-both", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		directory := ".tpatch/features/temp-authority/replaced"
		rootMkdirAll(t, authority, directory, 0o755)
		rel := directory + "/value.json"
		tempRel := directory + "/.value.json.tmp-333333333333"
		ownedEvidence := tempRel + ".owned"
		restoreBeforeRename(t, func(int) {
			if err := authority.WithRoot(func(root *os.Root) error {
				if err := root.Rename(tempRel, ownedEvidence); err != nil {
					return err
				}
				return writeRootFile(root, tempRel, []byte("replacement\n"), 0o600)
			}); err != nil {
				t.Fatal(err)
			}
		})

		result, err := DurableWrite(authority, WriteRequest{
			Rel:        rel,
			Data:       []byte("owned\n"),
			Mode:       0o644,
			Indexed:    true,
			EntryIndex: 0,
			Role:       WriteRoleOrdinaryCanonical,
		}, Options{RandomHex12: fixedHex("333333333333")})
		assertCleanupAuthorityFailure(t, result, err)
		if got := rootRead(t, authority, ownedEvidence); string(got) != "owned\n" {
			t.Fatalf("owned temporary evidence changed: %q", got)
		}
		if got := rootRead(t, authority, tempRel); string(got) != "replacement\n" {
			t.Fatalf("replacement basename changed: %q", got)
		}
		if rootExists(t, authority, rel) {
			t.Fatal("destination was published from a replacement temporary")
		}
	})
}

func TestTempContentGateRejectsSameInodeTampering(t *testing.T) {
	if !descriptorTempCleanupSupported() {
		t.Skip("descriptor-relative temporary cleanup is unavailable")
	}
	tests := []struct {
		name   string
		mutate func(*testing.T, *os.File)
	}{
		{
			name: "same-length",
			mutate: func(t *testing.T, file *os.File) {
				t.Helper()
				rewriteTempForTest(t, file, []byte("pwned\n"))
			},
		},
		{
			name: "longer",
			mutate: func(t *testing.T, file *os.File) {
				t.Helper()
				rewriteTempForTest(t, file, []byte("attacker-longer\n"))
			},
		},
		{
			name: "shorter",
			mutate: func(t *testing.T, file *os.File) {
				t.Helper()
				rewriteTempForTest(t, file, []byte("x\n"))
			},
		},
		{
			name: "chmod",
			mutate: func(t *testing.T, file *os.File) {
				t.Helper()
				if err := file.Chmod(0o600); err != nil {
					t.Fatal(err)
				}
				if err := file.Sync(); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, authority := acquireWorkspace(t)
			directory := ".tpatch/features/temp-content/" + test.name
			rootMkdirAll(t, authority, directory, 0o755)
			rel := directory + "/value.json"
			suffix := strings.Repeat(string(rune('a'+index)), 12)
			tempRel := directory + "/.value.json.tmp-" + suffix
			restoreBeforeRename(t, func(int) {
				if err := authority.WithRoot(func(root *os.Root) error {
					file, err := root.OpenFile(tempRel, os.O_RDWR, 0)
					if err != nil {
						return err
					}
					test.mutate(t, file)
					return file.Close()
				}); err != nil {
					t.Fatal(err)
				}
			})

			result, err := DurableWrite(authority, WriteRequest{
				Rel:        rel,
				Data:       []byte("owned\n"),
				Mode:       0o644,
				Indexed:    true,
				EntryIndex: index,
				Role:       WriteRoleOrdinaryCanonical,
			}, Options{RandomHex12: fixedHex(suffix)})
			var typed *Error
			if !errors.As(err, &typed) || typed.Code != CodeRootedWrite ||
				typed.Class != "temp-content-gate" || typed.ExitClass != 5 ||
				result.Committed {
				t.Fatalf("same-inode tamper = result=%+v err=%v", result, err)
			}
			if rootExists(t, authority, rel) {
				t.Fatal("same-inode attacker bytes reached the canonical destination")
			}
			if rootExists(t, authority, tempRel) {
				t.Fatal("proven same-inode temporary was not cleaned up")
			}
		})
	}
}

func TestTempContentGateDetectsPostReadMutationMetadata(t *testing.T) {
	if !descriptorTempCleanupSupported() {
		t.Skip("descriptor-relative temporary cleanup is unavailable")
	}
	tests := []struct {
		name   string
		mutate func(*testing.T, string, *intentlock.WorkspaceAuthority, string)
	}{
		{
			name: "same-size-overwrite",
			mutate: func(t *testing.T, _ string, authority *intentlock.WorkspaceAuthority, rel string) {
				t.Helper()
				if err := authority.WithRoot(func(root *os.Root) error {
					file, err := root.OpenFile(rel, os.O_RDWR, 0)
					if err != nil {
						return err
					}
					rewriteTempForTest(t, file, []byte("pwned\n"))
					return file.Close()
				}); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "mtime-only",
			mutate: func(t *testing.T, workspace string, _ *intentlock.WorkspaceAuthority, rel string) {
				t.Helper()
				absolute := filepath.Join(workspace, filepath.FromSlash(rel))
				if err := os.Chtimes(
					absolute,
					time.Unix(1_700_000_000, 111),
					time.Unix(1_700_000_001, 222),
				); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "chmod",
			mutate: func(t *testing.T, _ string, authority *intentlock.WorkspaceAuthority, rel string) {
				t.Helper()
				if err := authority.WithRoot(func(root *os.Root) error {
					file, err := root.OpenFile(rel, os.O_RDWR, 0)
					if err != nil {
						return err
					}
					if err := file.Chmod(0o600); err != nil {
						_ = file.Close()
						return err
					}
					if err := file.Sync(); err != nil {
						_ = file.Close()
						return err
					}
					return file.Close()
				}); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace, authority := acquireWorkspace(t)
			directory := ".tpatch/features/temp-post-read/" + test.name
			rootMkdirAll(t, authority, directory, 0o755)
			rel := directory + "/value.json"
			suffix := strings.Repeat(string(rune('a'+index)), 12)
			tempRel := directory + "/.value.json.tmp-" + suffix
			oldHook := afterTempContentRead
			t.Cleanup(func() { afterTempContentRead = oldHook })
			calls := 0
			afterTempContentRead = func(name string) {
				calls++
				if name != path.Base(tempRel) {
					t.Fatalf("post-read hook path = %q, want %q", name, path.Base(tempRel))
				}
				test.mutate(t, workspace, authority, tempRel)
			}

			result, err := DurableWrite(authority, WriteRequest{
				Rel:        rel,
				Data:       []byte("owned\n"),
				Mode:       0o644,
				Indexed:    true,
				EntryIndex: index,
				Role:       WriteRoleOrdinaryCanonical,
			}, Options{RandomHex12: fixedHex(suffix)})
			var typed *Error
			if !errors.As(err, &typed) || typed.Class != "temp-content-gate" ||
				typed.ExitClass != 5 || result.Committed || calls != 1 {
				t.Fatalf("post-read mutation = result=%+v calls=%d err=%v", result, calls, err)
			}
			if rootExists(t, authority, rel) || rootExists(t, authority, tempRel) {
				t.Fatal("post-read mutation was published or its proven temp was retained")
			}
		})
	}

	t.Run("normal-read", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		rel := ".tpatch/features/temp-post-read/normal/value.json"
		oldHook := afterTempContentRead
		t.Cleanup(func() { afterTempContentRead = oldHook })
		calls := 0
		afterTempContentRead = func(string) { calls++ }
		result, err := DurableWrite(authority, WriteRequest{
			Rel:  rel,
			Data: []byte("owned\n"),
			Mode: 0o644,
			Role: WriteRoleOrdinaryCanonical,
		}, Options{RandomHex12: fixedHex("dddddddddddd")})
		if err != nil || !result.Committed || calls != 1 {
			t.Fatalf("normal held read = result=%+v calls=%d err=%v", result, calls, err)
		}
	})
}

func TestTempContentGateSensitivity(t *testing.T) {
	writer, err := os.ReadFile("writer.go")
	if err != nil {
		t.Fatal(err)
	}
	remove, err := os.ReadFile("removeat_unix.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := requireTempContentGate(writer, remove); err != nil {
		t.Fatal(err)
	}
	for file, fields := range map[string][]string{
		"temp_stat_linux.go":  {"Mtim.Sec", "Mtim.Nsec", "Ctim.Sec", "Ctim.Nsec"},
		"temp_stat_darwin.go": {"Mtimespec.Sec", "Mtimespec.Nsec", "Ctimespec.Sec", "Ctimespec.Nsec"},
	} {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, field := range fields {
			if !bytes.Contains(source, []byte(field)) {
				t.Fatalf("%s lost explicit mutation metadata field %s", file, field)
			}
		}
	}
	if bytes.Contains(remove, []byte("reflect.")) {
		t.Fatal("temporary identity checks reverted to reflection-based Stat_t comparison")
	}
	mutated := bytes.Replace(
		remove,
		[]byte("if hex.EncodeToString(sum[:]) != intended.SHA256 {"),
		[]byte("if false {"),
		1,
	)
	if err := requireTempContentGate(writer, mutated); err == nil {
		t.Fatal("removing the held-descriptor hash comparison escaped the sensitivity guard")
	}
}

func requireTempContentGate(writer, remove []byte) error {
	writerText := string(writer)
	destinationGate := strings.Index(writerText, "retainedParentMatchesDestination")
	contentGate := strings.Index(writerText, "verifyTempContentAtHeldDirectory")
	rename := strings.Index(writerText, "if err := ops.Rename(tempRel, request.Rel); err != nil")
	if destinationGate < 0 || contentGate <= destinationGate || rename <= contentGate {
		return errors.New("held-descriptor content gate is absent or out of order")
	}
	removeText := string(remove)
	for _, required := range []string{
		"syscall.Pread",
		"before.Size != intended.Size",
		"uint32(before.Mode&0o777) != intended.Mode",
		"if hex.EncodeToString(sum[:]) != intended.SHA256 {",
		"sameTempMutationMetadata(before, after)",
		"sameTempMutationMetadata(after, pathStat)",
	} {
		if !strings.Contains(removeText, required) {
			return errors.New("held-descriptor content gate lost " + required)
		}
	}
	return nil
}

func rewriteTempForTest(t *testing.T, file *os.File, data []byte) {
	t.Helper()
	if err := file.Truncate(0); err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt(data, 0); err != nil {
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
}

func assertCleanupAuthorityFailure(t *testing.T, result WriteResult, err error) {
	t.Helper()
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != CodeCleanupFailed ||
		typed.ExitClass != 6 || result.Committed {
		t.Fatalf("cleanup authority = result=%+v err=%v", result, err)
	}
}

type staleParentRootOps struct {
	RootOps
	directory      string
	staleDirectory string
}

func (*staleParentRootOps) descriptorTempCleanup() {}

func (ops *staleParentRootOps) Open(name string) (RootFile, error) {
	if path.Clean(name) == path.Clean(ops.directory) {
		return ops.RootOps.Open(ops.staleDirectory)
	}
	return ops.RootOps.Open(name)
}
