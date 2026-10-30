package candidate

import (
	"fmt"

	"github.com/tesseracode/tesserapatch/internal/patchobs"
)

type treeSnapshot struct {
	root    *ReferenceRoot
	entries map[string]patchobs.CommitPath
}

func (r *ReferenceRoot) snapshotPaths(paths []string) (treeSnapshot, error) {
	if err := r.ensureAvailable(); err != nil {
		return treeSnapshot{}, err
	}
	for _, path := range paths {
		if _, err := normalizeRelativePath(path); err != nil {
			return treeSnapshot{}, err
		}
	}
	entries, err := patchobs.SnapshotCommitPathsOffline(r.repo.path, r.commit, paths)
	if identityErr := r.ensureAvailable(); identityErr != nil {
		return treeSnapshot{}, identityErr
	}
	if err != nil {
		return treeSnapshot{}, err
	}
	result := treeSnapshot{root: r, entries: make(map[string]patchobs.CommitPath, len(entries))}
	for _, entry := range entries {
		result.entries[entry.Path] = entry
	}
	return result, nil
}

func (s treeSnapshot) read(path string, limit int64) ([]byte, error) {
	entry, known := s.entries[path]
	if !known || !entry.Present {
		return nil, fmt.Errorf("snapshot path is absent")
	}
	if entry.Size > limit {
		return nil, patchobs.ErrCommitObjectLimit
	}
	if err := s.root.ensureAvailable(); err != nil {
		return nil, err
	}
	body, err := patchobs.ReadCommitObjectOffline(s.root.repo.path, entry.ObjectID, limit)
	if identityErr := s.root.ensureAvailable(); identityErr != nil {
		return nil, identityErr
	}
	if err != nil {
		return nil, err
	}
	if int64(len(body)) != entry.Size {
		return nil, fmt.Errorf("snapshot object size differs")
	}
	return body, nil
}
