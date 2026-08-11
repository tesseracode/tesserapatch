// Bounded, in-memory content reads for
// PRD-feature-resource-claims-and-capture-adapters §5.1 / §8.1.
//
// Raw bytes are never written to any file, ephemeral or tracked. An
// ignored-file selector's content is read directly into a bounded
// in-process buffer and scanned/hashed entirely in memory; the buffer
// is then discarded (Go's GC reclaims it — there is no file to delete).
//
// The size cap is enforced by an actual cap-plus-one read rather than a
// pre-read Stat().Size() check, so a file that grows between the stat
// and the read cannot silently bypass it.

package rescap

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/redact"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// Directory limits, re-checked at every capture rather than being a
// one-time add-time check (§5.1).
const (
	MaxFileBytes      int64 = 5 << 20
	MaxDirectoryBytes int64 = 20 << 20
	MaxDirectoryFiles int   = 200
	// binarySniffBytes is the prefix examined for a NUL byte when
	// classifying content as binary or text.
	binarySniffBytes = 8 << 10
)

// FileCapture is one file's in-memory capture result. Content is
// deliberately not retained on the struct beyond the caller's own
// staging pass.
type FileCapture struct {
	RelPath   string
	Mode      string
	SizeBytes uint64
	SHA256Hex string
	FileKind  string
}

// readBounded reads at most limit+1 bytes and refuses if that many
// were actually read.
func readBounded(r io.Reader, limit int64) ([]byte, error) {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 32*1024)
	var total int64
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			total += int64(n)
			if total > limit {
				return nil, Refuse(ReasonResourceLimitExceeded,
					"content exceeds the %d-byte cap", limit)
			}
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if err == io.EOF {
				return buf, nil
			}
			return nil, Internal(ReasonAdapterOutputReadFailed, "reading content: %v", err)
		}
	}
}

// classifyContent returns "binary" when a NUL byte appears in the
// first 8 KiB, "text" otherwise.
func classifyContent(content []byte) string {
	limit := len(content)
	if limit > binarySniffBytes {
		limit = binarySniffBytes
	}
	for i := 0; i < limit; i++ {
		if content[i] == 0 {
			return "binary"
		}
	}
	return "text"
}

// captureFileContent gates the path, reads it under the cap, scans it
// for the six closed redaction classes, and hashes it verbatim — no
// text normalization of any kind: CRLF/LF, trailing newline and
// encoding are all left exactly as found.
func captureFileContent(repoRoot, relPath string, limit int64) (FileCapture, []byte, error) {
	gated, err := GatePath(repoRoot, relPath)
	if err != nil {
		return FileCapture{}, nil, err
	}
	defer func() { _ = gated.Close() }()
	if gated.IsDir {
		return FileCapture{}, nil, Refuse(ReasonPathMissing, "%s is a directory, not a file", relPath)
	}
	content, err := readBounded(gated.File, limit)
	if err != nil {
		return FileCapture{}, nil, err
	}
	if findings := redact.Scan(content); len(findings) > 0 {
		return FileCapture{}, nil, Refuse(ReasonRedactionRefused,
			"%s matched forbidden content classes %s", relPath, strings.Join(findings, ", "))
	}
	digest := sha256.Sum256(content)
	return FileCapture{
		RelPath:   filepath.ToSlash(gated.RelPath),
		Mode:      store.OctalMode(gated.Info.Mode()),
		SizeBytes: uint64(len(content)),
		SHA256Hex: hex.EncodeToString(digest[:]),
		FileKind:  classifyContent(content),
	}, content, nil
}

// CaptureIgnoredFile stages an ignored-file resource. A directory
// selector's descendants are each gated independently — a selector that
// was a plain directory of plain files at add time but has since had
// one entry replaced by a symlink is caught at the next capture, never
// grandfathered in because the top-level directory still passes.
//
// This is a sequential read, not an atomic multi-file snapshot: each
// file is opened, read and hashed one at a time, so an external process
// modifying a later file while an earlier one has already been hashed
// can in principle produce a combined_hash that never corresponded to
// any single point-in-time directory state. That residual is disclosed
// rather than claimed away.
func CaptureIgnoredFile(repoRoot, selector string) (store.CanonNode, *store.RawInfo, error) {
	gated, err := GatePath(repoRoot, selector)
	if err != nil {
		return store.CanonNull(), nil, err
	}
	isDir := gated.IsDir
	_ = gated.Close()

	if !isDir {
		fc, content, err := captureFileContent(repoRoot, selector, MaxFileBytes)
		if err != nil {
			return store.CanonNull(), nil, err
		}
		result := store.CanonObject(
			store.CanonFieldOf("file_kind", store.CanonString(fc.FileKind)),
			store.CanonFieldOf("size_bytes", store.CanonUint(fc.SizeBytes)),
			store.CanonFieldOf("hash", store.CanonString("sha256:"+fc.SHA256Hex)),
		)
		_ = content
		return result, &store.RawInfo{Hash: "sha256:" + fc.SHA256Hex, ByteCount: fc.SizeBytes}, nil
	}

	descendants, err := listDirectoryFiles(repoRoot, selector)
	if err != nil {
		return store.CanonNull(), nil, err
	}
	if len(descendants) > MaxDirectoryFiles {
		return store.CanonNull(), nil, Refuse(ReasonResourceLimitExceeded,
			"%s matches %d files, over the %d-file limit", selector, len(descendants), MaxDirectoryFiles)
	}
	var total uint64
	captures := make([]FileCapture, 0, len(descendants))
	for _, rel := range descendants {
		fc, _, err := captureFileContent(repoRoot, rel, MaxFileBytes)
		if err != nil {
			return store.CanonNull(), nil, err
		}
		total += fc.SizeBytes
		if int64(total) > MaxDirectoryBytes {
			return store.CanonNull(), nil, Refuse(ReasonResourceLimitExceeded,
				"%s exceeds the %d-byte directory total", selector, MaxDirectoryBytes)
		}
		captures = append(captures, fc)
	}
	sort.SliceStable(captures, func(i, j int) bool { return captures[i].RelPath < captures[j].RelPath })

	combined := CombinedDirectoryHash(captures)
	files := make([]store.CanonNode, 0, len(captures))
	for _, fc := range captures {
		files = append(files, store.CanonObject(
			store.CanonFieldOf("path", store.CanonString(fc.RelPath)),
			store.CanonFieldOf("raw_sha256", store.CanonString("sha256:"+fc.SHA256Hex)),
			store.CanonFieldOf("byte_count", store.CanonUint(fc.SizeBytes)),
			store.CanonFieldOf("mode", store.CanonString(fc.Mode)),
		))
	}
	result := store.CanonObject(
		store.CanonFieldOf("file_count", store.CanonUint(uint64(len(captures)))),
		store.CanonFieldOf("total_bytes", store.CanonUint(total)),
		store.CanonFieldOf("combined_hash", store.CanonString("sha256:"+combined)),
		store.CanonFieldOf("files", store.CanonArrayOf(files)),
	)
	return result, &store.RawInfo{Hash: "sha256:" + combined, ByteCount: total}, nil
}

// CombinedDirectoryHash implements §5.1's exact tuple-encoding rule.
//
// Each of the tuple's three fields — path (repo-relative), mode (the
// 6-digit octal string), and hash (the file's own *raw, unprefixed*
// 64-lowercase-hex digest, explicitly not the "sha256:"-prefixed wire
// form) — is individually terminated by a single 0x00 byte, so a file's
// contribution is exactly path+0x00+mode+0x00+hash+0x00: three fields,
// three trailing NUL bytes, not two separators. Contributions
// concatenate directly with no further separator, since neither a
// repo-relative path nor a fixed-width mode/hash can contain a NUL.
//
// mode participates, so a chmod-only change (identical bytes, different
// permission bits) changes combined_hash.
func CombinedDirectoryHash(captures []FileCapture) string {
	sorted := make([]FileCapture, len(captures))
	copy(sorted, captures)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].RelPath < sorted[j].RelPath })
	h := sha256.New()
	for _, fc := range sorted {
		h.Write([]byte(fc.RelPath))
		h.Write([]byte{0})
		h.Write([]byte(fc.Mode))
		h.Write([]byte{0})
		h.Write([]byte(fc.SHA256Hex))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// listDirectoryFiles walks a directory selector and returns every
// descendant regular file's repo-relative path, sorted. Symlinks are
// left for the per-file gate to refuse so the refusal name is the same
// whether the symlink is the selector, an ancestor, or a descendant.
func listDirectoryFiles(repoRoot, selector string) ([]string, error) {
	abs, err := LexicalContainment(repoRoot, selector)
	if err != nil {
		return nil, err
	}
	var out []string
	walkErr := filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return relErr
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if walkErr != nil {
		return nil, Refuse(ReasonPathMissing, "walking %s: %v", selector, walkErr)
	}
	sort.Strings(out)
	return out, nil
}
