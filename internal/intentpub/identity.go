package intentpub

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
)

type capturedFile struct {
	Identity Identity
	Bytes    []byte
}

type walkedComponent struct {
	rel  string
	info fs.FileInfo
}

func CaptureIdentity(authority *intentlock.WorkspaceAuthority, rel string, options Options) (Identity, error) {
	if authority == nil || !validRootRel(rel) {
		return Identity{}, transactionError(CodeInvalidPlan, "", "identity-path", "the identity path is not root-relative", 5)
	}
	options, err := options.withScratch()
	if err != nil {
		return Identity{}, err
	}
	var identity Identity
	err = authority.WithRoot(func(root *os.Root) error {
		captured, captureErr := options.captureBytes(options.rootOps(root), rel)
		identity = captured.Identity
		return captureErr
	})
	return identity, err
}

func captureRegular(ops RootOps, rel string, scratch []byte) (capturedFile, error) {
	if !validRootRel(rel) || len(scratch) != MaxArtifactBytes+1 {
		return capturedFile{}, transactionError(CodeInvalidPlan, "", "capture-input", "the rooted capture input is invalid", 5)
	}

	components, err := walkComponents(ops, rel)
	if err != nil {
		return capturedFile{}, err
	}
	before, err := ops.Lstat(rel)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if err := revalidateComponents(ops, components); err != nil {
				return capturedFile{}, err
			}
			return capturedFile{Identity: AbsentIdentity()}, nil
		}
		return capturedFile{}, identityError("lstat", "the file identity could not be captured")
	}
	if refusedFileInfo(before) || !before.Mode().IsRegular() {
		return capturedFile{}, transactionError(CodeNonRegular, "", "final-leaf", "the final leaf is not a regular file", 5)
	}
	if before.Size() < 0 || before.Size() > MaxArtifactBytes {
		return capturedFile{}, transactionError(CodeFileOversize, "", "bounded-read", "the file exceeds the accepted identity bound", 5)
	}

	file, err := ops.OpenFile(rel, os.O_RDONLY|openFlags(), 0)
	if err != nil {
		return capturedFile{}, identityError("open", "the file identity changed during capture")
	}
	closed := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
	}()

	opened, err := file.Stat()
	if err != nil || !sameSnapshot(ops, before, opened) {
		return capturedFile{}, identityError("open-identity", "the file identity changed during capture")
	}

	count, readErr := io.ReadFull(file, scratch)
	switch {
	case readErr == nil:
		return capturedFile{}, transactionError(CodeFileOversize, "", "bounded-read", "the file exceeds the accepted identity bound", 5)
	case !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF):
		return capturedFile{}, identityError("read", "the file identity could not be read")
	}
	if int64(count) != opened.Size() {
		return capturedFile{}, identityError("read-size", "the file identity changed during capture")
	}

	descriptorPost, err := file.Stat()
	if err != nil || !sameSnapshot(ops, opened, descriptorPost) {
		return capturedFile{}, identityError("descriptor-post-read", "the file identity changed during capture")
	}
	if err := revalidateComponents(ops, components); err != nil {
		return capturedFile{}, err
	}
	after, err := ops.Lstat(rel)
	if err != nil || refusedFileInfo(after) || !after.Mode().IsRegular() || !sameSnapshot(ops, before, after) {
		return capturedFile{}, identityError("post-read", "the file identity changed during capture")
	}
	if err := file.Close(); err != nil {
		closed = true
		return capturedFile{}, identityError("close", "the file identity could not be closed")
	}
	closed = true

	sum := sha256.Sum256(scratch[:count])
	return capturedFile{
		Identity: Identity{
			Exists: true,
			SHA256: hex.EncodeToString(sum[:]),
			Size:   int64(count),
			Mode:   uint32(after.Mode().Perm()),
		},
		Bytes: scratch[:count],
	}, nil
}

func walkComponents(ops RootOps, rel string) ([]walkedComponent, error) {
	directory := path.Dir(rel)
	if directory == "." {
		return nil, nil
	}
	parts := strings.Split(directory, "/")
	components := make([]walkedComponent, 0, len(parts))
	current := ""
	for _, part := range parts {
		if current == "" {
			current = part
		} else {
			current += "/" + part
		}
		info, err := ops.Lstat(current)
		if err != nil {
			return nil, identityError("component-walk", "a rooted ancestor could not be inspected")
		}
		if refusedFileInfo(info) || !info.IsDir() {
			return nil, transactionError(CodeNonRegular, "", "component-walk", "a rooted ancestor is not a real directory", 5)
		}
		components = append(components, walkedComponent{rel: current, info: info})
	}
	return components, nil
}

func revalidateComponents(ops RootOps, components []walkedComponent) error {
	for _, component := range components {
		current, err := ops.Lstat(component.rel)
		if err != nil || refusedFileInfo(current) || !current.IsDir() ||
			!ops.SameFile(component.info, current) ||
			component.info.Mode().Perm() != current.Mode().Perm() ||
			!component.info.ModTime().Equal(current.ModTime()) {
			return identityError("component-post-walk", "a rooted ancestor changed during capture")
		}
	}
	return nil
}

func sameSnapshot(ops RootOps, first, second fs.FileInfo) bool {
	return first != nil && second != nil &&
		!refusedFileInfo(second) &&
		second.Mode().IsRegular() &&
		ops.SameFile(first, second) &&
		first.Size() == second.Size() &&
		first.Mode().Perm() == second.Mode().Perm() &&
		first.ModTime().Equal(second.ModTime())
}

func refusedFileInfo(info fs.FileInfo) bool {
	return info == nil || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0
}

func identityError(class, detail string) *Error {
	return transactionError(CodeIdentityUnstable, "", class, detail, 5)
}

func identityForBytes(data []byte, mode fs.FileMode) (Identity, error) {
	if len(data) > MaxArtifactBytes {
		return Identity{}, transactionError(CodeFileOversize, "", "bounded-write", "the intended file exceeds the accepted identity bound", 5)
	}
	if mode.Perm() != mode || mode.Perm() == 0 {
		return Identity{}, transactionError(CodeInvalidPlan, "", "identity-mode", "the intended file mode is invalid", 5)
	}
	sum := sha256.Sum256(data)
	return Identity{
		Exists: true,
		SHA256: hex.EncodeToString(sum[:]),
		Size:   int64(len(data)),
		Mode:   uint32(mode.Perm()),
	}, nil
}

func stagedIdentity(newImage Identity) Identity {
	if !newImage.Exists {
		return AbsentIdentity()
	}
	return Identity{
		Exists: true,
		SHA256: newImage.SHA256,
		Size:   newImage.Size,
		Mode:   0o600,
	}
}
