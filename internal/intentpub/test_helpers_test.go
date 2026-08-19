package intentpub

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
)

const (
	testSlug     = "journal-test"
	testStageRel = ".tpatch/local/intent-prepare/journal-test/stage-0123456789ab"
)

func testCreatePlan(t *testing.T) Plan {
	t.Helper()
	ids := []ArtifactID{
		ArtifactAnalysis,
		ArtifactSpec,
		ArtifactExploration,
		ArtifactAnalysisSidecar,
		ArtifactStatus,
	}
	entries := make([]Entry, 0, len(ids))
	for _, id := range ids {
		identity, err := identityForBytes([]byte("new-"+string(id)), 0o644)
		if err != nil {
			t.Fatal(err)
		}
		entry := Entry{
			ArtifactID: id,
			Rel:        canonicalRel(testSlug, id),
			Action:     ActionCreate,
			Preimage:   AbsentIdentity(),
			NewImage:   identity,
			StagedRel:  testStageRel + "/" + stagedBase(id),
		}
		if id == ArtifactStatus {
			preimage, preErr := identityForBytes([]byte("old-status"), 0o644)
			if preErr != nil {
				t.Fatal(preErr)
			}
			entry.Action = ActionReplace
			entry.Preimage = preimage
			entry.PreimageRawRel = laneRel(testSlug) + "/status.preimage.json"
		}
		entries = append(entries, entry)
	}

	plan, err := NewPlan(testSlug, ModeGenerate, testStageRel, entries)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func rootMkdirAll(t *testing.T, authority *intentlock.WorkspaceAuthority, rel string, mode fs.FileMode) {
	t.Helper()
	err := authority.WithRoot(func(root *os.Root) error {
		current := ""
		for _, component := range strings.Split(rel, "/") {
			if current == "" {
				current = component
			} else {
				current += "/" + component
			}
			if err := root.Mkdir(current, mode); err != nil && !errors.Is(err, fs.ErrExist) {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func acquireWorkspace(t *testing.T) (string, *intentlock.WorkspaceAuthority) {
	t.Helper()
	if !intentlock.AuthoritySupported {
		t.Skip("real workspace authority is unavailable on this platform")
	}
	root := t.TempDir()
	authority, err := intentlock.Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !authority.Released() {
			_ = authority.Release()
		}
	})
	return root, authority
}

func rootWrite(t *testing.T, authority *intentlock.WorkspaceAuthority, rel string, data []byte, mode fs.FileMode) {
	t.Helper()
	err := authority.WithRoot(func(root *os.Root) error {
		current := ""
		for _, component := range strings.Split(pathDir(rel), "/") {
			if component == "." {
				continue
			}
			if current == "" {
				current = component
			} else {
				current += "/" + component
			}
			if err := root.Mkdir(current, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
				return err
			}
		}
		file, err := root.OpenFile(rel, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			return err
		}
		if _, err := file.Write(data); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Sync(); err != nil {
			_ = file.Close()
			return err
		}
		return file.Close()
	})
	if err != nil {
		t.Fatal(err)
	}
}

func rootRead(t *testing.T, authority *intentlock.WorkspaceAuthority, rel string) []byte {
	t.Helper()
	var data []byte
	err := authority.WithRoot(func(root *os.Root) error {
		file, err := root.Open(rel)
		if err != nil {
			return err
		}
		defer file.Close()
		data, err = io.ReadAll(file)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func rootExists(t *testing.T, authority *intentlock.WorkspaceAuthority, rel string) bool {
	t.Helper()
	exists := false
	err := authority.WithRoot(func(root *os.Root) error {
		_, err := root.Lstat(rel)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		exists = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return exists
}

func pathDir(rel string) string {
	for index := len(rel) - 1; index >= 0; index-- {
		if rel[index] == '/' {
			return rel[:index]
		}
	}
	return "."
}

type pathSnapshot struct {
	Mode fs.FileMode
	Data string
}

func snapshotPath(t *testing.T, workspace, rel string) map[string]pathSnapshot {
	t.Helper()
	base := filepath.Join(workspace, filepath.FromSlash(rel))
	result := make(map[string]pathSnapshot)
	err := filepath.WalkDir(base, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		key, err := filepath.Rel(workspace, name)
		if err != nil {
			return err
		}
		item := pathSnapshot{Mode: info.Mode()}
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			item.Data = string(data)
		}
		result[filepath.ToSlash(key)] = item
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
