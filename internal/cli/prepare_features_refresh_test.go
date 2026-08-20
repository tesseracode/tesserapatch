package cli

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/intentpub"
	"github.com/tesseracode/tesserapatch/internal/store"
)

func TestPrepareFeaturesRefreshUsesCapturedIdentity(t *testing.T) {
	if !intentlock.AuthoritySupported {
		t.Skip("workspace mutation authority is unsupported on this target")
	}

	t.Run("existing", func(t *testing.T) {
		root, _ := prepareS4Workspace(t, "features refresh existing")
		features := filepath.Join(root, ".tpatch", "FEATURES.md")
		if err := os.WriteFile(features, []byte("stale\n"), 0o640); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(features, 0o640); err != nil {
			t.Fatal(err)
		}
		unrelated := filepath.Join(root, ".tpatch", "unrelated")
		if err := os.WriteFile(unrelated, []byte("unchanged\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		authority := acquirePrepareRefreshAuthority(t, root)
		report := preparePublishReport{}
		refreshPrepareFeaturesIndex(authority, root, &report)
		if len(report.Advisories) != 0 {
			t.Fatalf("existing refresh advisories = %+v", report.Advisories)
		}
		want, err := renderPrepareFeaturesIndex(&store.Store{Root: root})
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(features)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("existing FEATURES.md = %q err=%v want=%q", got, err, want)
		}
		info, err := os.Stat(features)
		if err != nil || info.Mode().Perm() != 0o640 {
			t.Fatalf("existing FEATURES.md mode = %v err=%v", info, err)
		}
		assertPrepareRefreshUnrelated(t, unrelated)
	})

	t.Run("absent", func(t *testing.T) {
		root, _ := prepareS4Workspace(t, "features refresh absent")
		features := filepath.Join(root, ".tpatch", "FEATURES.md")
		if err := os.Remove(features); err != nil {
			t.Fatal(err)
		}
		unrelated := filepath.Join(root, ".tpatch", "unrelated")
		if err := os.WriteFile(unrelated, []byte("unchanged\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		authority := acquirePrepareRefreshAuthority(t, root)
		report := preparePublishReport{}
		refreshPrepareFeaturesIndex(authority, root, &report)
		if len(report.Advisories) != 0 {
			t.Fatalf("absent refresh advisories = %+v", report.Advisories)
		}
		if info, err := os.Stat(features); err != nil || info.Mode().Perm() != 0o644 {
			t.Fatalf("created FEATURES.md = %v err=%v", info, err)
		}
		assertPrepareRefreshUnrelated(t, unrelated)
	})

	t.Run("concurrent-edit", func(t *testing.T) {
		root, _ := prepareS4Workspace(t, "features refresh concurrent")
		featuresRel := ".tpatch/FEATURES.md"
		features := filepath.Join(root, filepath.FromSlash(featuresRel))
		external := []byte("operator edit\n")
		unrelated := filepath.Join(root, ".tpatch", "unrelated")
		if err := os.WriteFile(unrelated, []byte("unchanged\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		state := &featuresRefreshRaceState{target: featuresRel, external: external}
		oldFactory := prepareIntentpubRootOps
		t.Cleanup(func() { prepareIntentpubRootOps = oldFactory })
		prepareIntentpubRootOps = func(root *os.Root) intentpub.RootOps {
			return &featuresRefreshRaceOps{
				RootOps: intentpub.NewRootOps(root),
				state:   state,
			}
		}
		authority := acquirePrepareRefreshAuthority(t, root)
		report := preparePublishReport{}
		refreshPrepareFeaturesIndex(authority, root, &report)
		if !state.fired {
			t.Fatal("concurrent FEATURES.md edit was not injected")
		}
		if len(report.Advisories) != 1 || report.Advisories[0].Code != "features-index-refresh-failed" {
			t.Fatalf("concurrent refresh advisories = %+v", report.Advisories)
		}
		got, err := os.ReadFile(features)
		if err != nil || !bytes.Equal(got, external) {
			t.Fatalf("concurrent FEATURES.md = %q err=%v", got, err)
		}
		assertPrepareRefreshUnrelated(t, unrelated)
	})
}

func acquirePrepareRefreshAuthority(t *testing.T, root string) *intentlock.WorkspaceAuthority {
	t.Helper()
	authority, err := intentlock.Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !authority.Released() {
			_ = authority.Release()
		}
	})
	return authority
}

func assertPrepareRefreshUnrelated(t *testing.T, path string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "unchanged\n" {
		t.Fatalf("unrelated file = %q err=%v", got, err)
	}
}

type featuresRefreshRaceState struct {
	target          string
	external        []byte
	targetReadCount int
	postCASLstat    bool
	fired           bool
}

type featuresRefreshRaceOps struct {
	intentpub.RootOps
	state *featuresRefreshRaceState
}

func (ops *featuresRefreshRaceOps) OpenFile(name string, flag int, mode fs.FileMode) (intentpub.RootFile, error) {
	file, err := ops.RootOps.OpenFile(name, flag, mode)
	if err == nil && name == ops.state.target && flag&os.O_WRONLY == 0 && flag&os.O_RDWR == 0 {
		ops.state.targetReadCount++
	}
	return file, err
}

func (ops *featuresRefreshRaceOps) Lstat(name string) (fs.FileInfo, error) {
	if name == ops.state.target && ops.state.targetReadCount >= 2 && !ops.state.fired {
		if ops.state.postCASLstat {
			file, err := ops.RootOps.OpenFile(name, os.O_WRONLY|os.O_TRUNC, 0)
			if err != nil {
				return nil, err
			}
			if _, err := file.Write(ops.state.external); err != nil {
				_ = file.Close()
				return nil, err
			}
			if err := file.Sync(); err != nil {
				_ = file.Close()
				return nil, err
			}
			if err := file.Close(); err != nil {
				return nil, err
			}
			ops.state.fired = true
		} else {
			ops.state.postCASLstat = true
		}
	}
	return ops.RootOps.Lstat(name)
}
