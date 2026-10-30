package candidate

import (
	"fmt"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
)

type snapshotEntry struct {
	Present bool
	Mode    string
	Size    int64
	Object  string
}

type treeSnapshot struct {
	entries map[string]snapshotEntry
	reader  func(path string, limit int64) ([]byte, error)
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
	result := treeSnapshot{entries: make(map[string]snapshotEntry, len(entries))}
	for _, entry := range entries {
		result.entries[entry.Path] = snapshotEntry{
			Present: entry.Present, Mode: entry.Mode, Size: entry.Size, Object: entry.ObjectID,
		}
	}
	result.reader = func(path string, limit int64) ([]byte, error) {
		entry, known := result.entry(path)
		if !known || !entry.Present {
			return nil, fmt.Errorf("snapshot path is absent")
		}
		if entry.Size > limit {
			return nil, patchobs.ErrCommitObjectLimit
		}
		if err := r.ensureAvailable(); err != nil {
			return nil, err
		}
		body, err := patchobs.ReadCommitObjectOffline(r.repo.path, entry.Object, limit)
		if identityErr := r.ensureAvailable(); identityErr != nil {
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
	return result, nil
}

func (s treeSnapshot) entry(path string) (snapshotEntry, bool) {
	entry, known := s.entries[path]
	return entry, known
}

func (s treeSnapshot) read(path string, limit int64) ([]byte, error) {
	entry, known := s.entry(path)
	if !known || !entry.Present {
		return nil, fmt.Errorf("snapshot path is absent")
	}
	if entry.Size > limit {
		return nil, patchobs.ErrCommitObjectLimit
	}
	if s.reader == nil {
		return nil, fmt.Errorf("snapshot reader is unavailable")
	}
	return s.reader(path, limit)
}
