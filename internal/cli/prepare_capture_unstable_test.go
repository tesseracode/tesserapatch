package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/intent"
	"github.com/tesseracode/tesserapatch/internal/intentlock"
)

func appendPrepareCaptureFixtureByte(root *os.Root, rel string) error {
	file, err := root.OpenFile(rel, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	_, writeErr := file.Write([]byte{'!'})
	return errors.Join(writeErr, file.Close())
}

func TestPrepareCaptureArtifactUnstable(t *testing.T) {
	for _, name := range []string{"stable", "size-change-after-read", "hook-error"} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			body := []byte("unchanged capture\n")
			if err := os.WriteFile(filepath.Join(directory, "spec.md"), body, 0o644); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			oldHook := afterPrepareCaptureRead
			t.Cleanup(func() { afterPrepareCaptureRead = oldHook })
			afterPrepareCaptureRead = nil
			scratch := make([]byte, 128)
			calls := 0
			injected := errors.New("fixture mutation failed")
			if name != "stable" {
				afterPrepareCaptureRead = func(captureRoot *os.Root, rel string) error {
					calls++
					if captureRoot != root || rel != "spec.md" || !bytes.Equal(scratch[:len(body)], body) {
						t.Fatal("post-read hook did not observe the real completed capture")
					}
					if name == "hook-error" {
						return injected
					}
					return appendPrepareCaptureFixtureByte(captureRoot, rel)
				}
			}
			captured, err := capturePrepareFileWithScratch(root, "spec.md", 127, scratch)
			switch name {
			case "stable":
				if err != nil || captured.state != intent.StatePresentNonempty ||
					!bytes.Equal(captured.bytes, body) || !captured.identity.Exists {
					t.Fatalf("stable capture = %+v, %v", captured, err)
				}
			case "size-change-after-read":
				if err != nil || calls != 1 || captured.state != intent.StateUnstable ||
					len(captured.bytes) != 0 || captured.identity.Exists {
					t.Fatalf("post-read mutation was not refused: capture=%+v calls=%d err=%v", captured, calls, err)
				}
			case "hook-error":
				if calls != 1 || !errors.Is(err, injected) || captured.identity.Exists {
					t.Fatalf("hook failure = %+v calls=%d err=%v", captured, calls, err)
				}
			}
		})
	}
}

func TestPrepareArtifactUnstableFixture(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Skip("prepare publication authority is unsupported on this platform")
	}
	oldHuman, oldDry, oldExternal, oldSnapshot, oldSnapshots := s6HumanObservationMode,
		s7APDryRunObservationMode, s7APDryRunAllowExternalFixtureMutation,
		s7APExitThreeSnapshotMode, s7APExitThreeSnapshots
	t.Cleanup(func() {
		s6HumanObservationMode, s7APDryRunObservationMode, s7APDryRunAllowExternalFixtureMutation =
			oldHuman, oldDry, oldExternal
		s7APExitThreeSnapshotMode, s7APExitThreeSnapshots = oldSnapshot, oldSnapshots
	})
	for _, name := range []string{"json", "human", "dry-run", "exit-three-snapshot"} {
		t.Run(name, func(t *testing.T) {
			s6HumanObservationMode = name == "human"
			s7APDryRunObservationMode = name == "dry-run"
			s7APDryRunAllowExternalFixtureMutation = name == "dry-run"
			s7APExitThreeSnapshotMode = name == "exit-three-snapshot"
			s7APExitThreeSnapshots = map[string]string{}
			observation := s6ObserveArtifactUnstable(t)
			if observation.exit != 3 {
				t.Fatalf("fixture exit = %d, want 3", observation.exit)
			}
			if s7APExitThreeSnapshotMode {
				if len(s7APExitThreeSnapshots) != 1 {
					t.Fatalf("snapshot count = %d, want 1", len(s7APExitThreeSnapshots))
				}
				for root, before := range s7APExitThreeSnapshots {
					if after := snapshotTreeMetadata(t, "exit-3", root); before != after {
						t.Fatal("exit-three workspace snapshot changed")
					}
				}
			}
		})
	}
}

func TestPrepareArtifactUnstableMutationControls(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Skip("prepare publication authority is unsupported on this platform")
	}
	root := avpRepoRoot(t)
	original := s6RepoFile(t, "internal/cli/prepare_publish.go")
	for _, fixture := range []struct {
		name, selector, diagnostic string
		replacements               [][2]string
	}{
		{
			name:         "missing-capture-hook",
			selector:     "^TestPrepareArtifactUnstableFixture$/^json$",
			diagnostic:   "capture mutation count = 0",
			replacements: [][2]string{{"if afterPrepareCaptureRead != nil {", "if false {"}},
		},
		{
			name:       "bypassed-post-read-checks",
			selector:   "^TestPrepareCaptureArtifactUnstable$/^size-change-after-read$",
			diagnostic: "post-read mutation was not refused",
			replacements: [][2]string{
				{"opened.Size() != descriptorAfter.Size()", "false"},
				{"!opened.ModTime().Equal(descriptorAfter.ModTime())", "false"},
				{"before.Size() != after.Size()", "false"},
				{"!before.ModTime().Equal(after.ModTime())", "false"},
			},
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			mutated := original
			for _, replacement := range fixture.replacements {
				if strings.Count(mutated, replacement[0]) != 1 {
					t.Fatalf("mutation anchor %q is not unique", replacement[0])
				}
				mutated = strings.Replace(mutated, replacement[0], replacement[1], 1)
			}
			directory := t.TempDir()
			backing := filepath.Join(directory, "prepare_publish.go")
			if err := os.WriteFile(backing, []byte(mutated), 0o600); err != nil {
				t.Fatal(err)
			}
			overlay, err := json.Marshal(map[string]map[string]string{
				"Replace": {filepath.Join(root, "internal", "cli", "prepare_publish.go"): backing},
			})
			if err != nil {
				t.Fatal(err)
			}
			overlayPath := filepath.Join(directory, "overlay.json")
			if err := os.WriteFile(overlayPath, overlay, 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			command := exec.CommandContext(ctx, "go", "test", "-p=1", "-count=1", "-timeout=2m",
				"-overlay="+overlayPath, "./internal/cli", "-run", fixture.selector)
			command.Dir = root
			s7ConfigureObservedProcess(command)
			output, runErr := command.CombinedOutput()
			if ctx.Err() != nil || runErr == nil ||
				!bytes.Contains(output, []byte(fixture.diagnostic)) ||
				!bytes.Contains(output, []byte("--- FAIL:")) {
				t.Fatalf("broken capture did not fail its real assertion: err=%v context=%v\n%s",
					runErr, ctx.Err(), output)
			}
		})
	}
}
