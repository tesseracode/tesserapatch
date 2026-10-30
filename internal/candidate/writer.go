package candidate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/safety"
)

type candidateWriter func(root *CandidateRoot, op operation) error

var candidateWriterRegistry = map[string]candidateWriter{
	"create-file":        writeCreateFile,
	"insert-by-boundary": writeInsertByBoundary,
}

func applyCandidateOperations(root *CandidateRoot, operations []operation) error {
	for _, op := range operations {
		writer, err := candidateWriterForKind(op.kind)
		if err != nil {
			return err
		}
		if err := writer(root, op); err != nil {
			return err
		}
	}
	return nil
}

func candidateWriterForKind(kind string) (candidateWriter, error) {
	writer, ok := candidateWriterRegistry[kind]
	if !ok {
		return nil, &refusal{code: "candidate-operation-kind-unsupported", detail: kind}
	}
	return writer, nil
}

func writeCreateFile(root *CandidateRoot, op operation) error {
	normalized, err := prepareCandidateWritePath(root, op.path, true)
	if err != nil {
		return err
	}
	if _, err := root.worktree.root.Lstat(normalized); err == nil {
		return &refusal{code: "candidate-postcondition-failed", path: op.path, detail: "create-file target already exists"}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return &refusal{code: "candidate-postcondition-failed", path: op.path, detail: fmt.Sprintf("inspect create-file target: %v", err)}
	}
	if err := writeRootedFile(root.worktree.root, normalized, op.payload); err != nil {
		return &refusal{code: "candidate-postcondition-failed", path: op.path, detail: fmt.Sprintf("write create-file target: %v", err)}
	}
	return nil
}

func writeInsertByBoundary(root *CandidateRoot, op operation) error {
	normalized, err := prepareCandidateWritePath(root, op.path, false)
	if err != nil {
		return err
	}
	body, err := root.ReadFile(normalized)
	if err != nil {
		return &refusal{code: "candidate-postcondition-failed", path: op.path, detail: fmt.Sprintf("read insertion target: %v", err)}
	}
	if op.byteOffset < 0 || op.byteOffset > len(body) {
		return &refusal{code: "candidate-postcondition-failed", path: op.path, detail: "insertion offset is outside the file"}
	}
	updated := make([]byte, 0, len(body)+len(op.payload))
	updated = append(updated, body[:op.byteOffset]...)
	updated = append(updated, op.payload...)
	updated = append(updated, body[op.byteOffset:]...)
	if err := writeRootedFile(root.worktree.root, normalized, updated); err != nil {
		return &refusal{code: "candidate-postcondition-failed", path: op.path, detail: fmt.Sprintf("write insertion target: %v", err)}
	}
	return nil
}

func prepareCandidateWritePath(root *CandidateRoot, relPath string, createParents bool) (string, error) {
	if err := root.ensureAvailable(); err != nil {
		return "", err
	}
	normalized, err := normalizeRelativePath(relPath)
	if err != nil {
		return "", &refusal{code: "candidate-postcondition-failed", path: relPath, detail: err.Error()}
	}
	absolute := filepath.Join(root.worktree.path, filepath.FromSlash(normalized))
	if err := safety.EnsureSafeRepoPath(root.worktree.path, absolute); err != nil {
		return "", &refusal{code: "candidate-postcondition-failed", path: relPath, detail: fmt.Sprintf("unsafe candidate path: %v", err)}
	}
	if err := ensureCandidateParentDirectories(root, normalized, createParents); err != nil {
		return "", err
	}
	return normalized, nil
}

func ensureCandidateParentDirectories(root *CandidateRoot, relPath string, createParents bool) error {
	parent := path.Dir(relPath)
	if parent == "." {
		return nil
	}
	segments := strings.Split(parent, "/")
	current := ""
	for _, segment := range segments {
		if current == "" {
			current = segment
		} else {
			current = path.Join(current, segment)
		}
		info, err := root.worktree.root.Lstat(current)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return &refusal{code: "candidate-postcondition-failed", path: relPath, detail: fmt.Sprintf("inspect parent path %q: %v", current, err)}
			}
			if !createParents {
				return &refusal{code: "candidate-postcondition-failed", path: relPath, detail: "parent directory is absent"}
			}
			if mkErr := root.worktree.root.Mkdir(current, 0o755); mkErr != nil && !errors.Is(mkErr, fs.ErrExist) {
				return &refusal{code: "candidate-postcondition-failed", path: relPath, detail: fmt.Sprintf("create parent path %q: %v", current, mkErr)}
			}
			info, err = root.worktree.root.Lstat(current)
			if err != nil {
				return &refusal{code: "candidate-postcondition-failed", path: relPath, detail: fmt.Sprintf("inspect created parent path %q: %v", current, err)}
			}
		}
		mode := info.Mode()
		if mode&os.ModeSymlink != 0 || mode&os.ModeIrregular != 0 || !mode.IsDir() {
			return &refusal{code: "candidate-postcondition-failed", path: relPath, detail: fmt.Sprintf("parent path %q is not a safe directory", current)}
		}
	}
	return nil
}

func writeRootedFile(root *os.Root, name string, body []byte) error {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(body); err != nil {
		return err
	}
	return nil
}
