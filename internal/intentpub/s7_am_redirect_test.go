//go:build (linux && !android) || (darwin && !ios)

package intentpub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestS7PIB406HeldRootRedirectsNeverEscapeAuthority(t *testing.T) {
	t.Run("stable-in-root-symlink-is-refused-without-following", func(t *testing.T) {
		workspace, authority := acquireWorkspace(t)
		rootMkdirAll(t, authority, ".tpatch/features/redirect/real", 0o755)
		rootWrite(t, authority, ".tpatch/features/redirect/real/value.json", []byte("in-root-secret"), 0o644)
		if err := authority.WithRoot(func(root *os.Root) error {
			return root.Symlink("real", ".tpatch/features/redirect/artifacts")
		}); err != nil {
			t.Fatal(err)
		}
		_, err := CaptureIdentity(
			authority, ".tpatch/features/redirect/artifacts/value.json", Options{},
		)
		assertCode(t, err, CodeNonRegular)
		if strings.Contains(err.Error(), workspace) || strings.Contains(err.Error(), "in-root-secret") {
			t.Fatalf("stable redirect refusal leaked absolute path/content: %v", err)
		}
	})

	t.Run("outside-root-symlink-is-refused-without-following", func(t *testing.T) {
		workspace, authority := acquireWorkspace(t)
		rootMkdirAll(t, authority, ".tpatch/features/redirect", 0o755)
		outside := filepath.Join(t.TempDir(), "outside.json")
		if err := os.WriteFile(outside, []byte("outside-secret"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := authority.WithRoot(func(root *os.Root) error {
			return root.Symlink(outside, ".tpatch/features/redirect/value.json")
		}); err != nil {
			t.Fatal(err)
		}
		_, err := CaptureIdentity(
			authority, ".tpatch/features/redirect/value.json", Options{},
		)
		assertCode(t, err, CodeNonRegular)
		if strings.Contains(err.Error(), workspace) ||
			strings.Contains(err.Error(), outside) ||
			strings.Contains(err.Error(), "outside-secret") {
			t.Fatalf("outside redirect refusal leaked absolute path/content: %v", err)
		}
		got, readErr := os.ReadFile(outside)
		if readErr != nil || string(got) != "outside-secret" {
			t.Fatalf("outside target changed: %q err=%v", got, readErr)
		}
	})

	t.Run("ancestor-substitution-is-detected-after-rooted-open", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		rel := ".tpatch/features/capture/artifacts/value.json"
		rootWrite(t, authority, rel, []byte(`{"ok":true}`), 0o644)
		state := &ancestorRaceState{}
		_, err := CaptureIdentity(authority, rel, Options{
			RootOpsFactory: func(root *os.Root) RootOps {
				return &ancestorRaceOps{RootOps: NewRootOps(root), root: root, state: state}
			},
		})
		assertCode(t, err, CodeIdentityUnstable)
		if !state.raced {
			t.Fatal("ancestor substitution seam did not run")
		}
	})
}
