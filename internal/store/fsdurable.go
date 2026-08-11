// Crash-safe filesystem primitives shared by the resource-capture
// tracked tree and the local scratch tree.
//
// PRD-feature-resource-claims-and-capture-adapters §7.1 step 4 requires
// an *unconditional* whole-chain fsync after MkdirAll — including
// directories that already existed. A directory can be Stat-visible
// immediately after creation well before the kernel has made that
// visibility crash-durable, so a retried invocation that only fsynced
// newly-created entries could still lose an earlier, not-yet-durable
// creation across a second crash.

package store

import (
	"crypto/rand"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SyncDir opens a directory and fsyncs it.
func SyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	if err := d.Sync(); err != nil {
		_ = d.Close()
		return err
	}
	return d.Close()
}

// MkdirAllAndSyncChain creates every missing directory from stopAt
// down to leaf with the given mode, then fsyncs every directory in
// that chain from the leaf back up to and including stopAt —
// unconditionally, whether or not this call is the one that created
// it.
//
// stopAt must be an ancestor of leaf; if it is not, the chain walk
// stops at the filesystem root.
func MkdirAllAndSyncChain(leaf, stopAt string, mode fs.FileMode) error {
	if err := os.MkdirAll(leaf, mode); err != nil {
		return err
	}
	for _, dir := range DirChain(leaf, stopAt) {
		if err := SyncDir(dir); err != nil {
			return err
		}
	}
	return nil
}

// DirChain returns leaf and each ancestor up to and including stopAt,
// deepest first. When stopAt is not an ancestor the walk terminates at
// the filesystem root.
func DirChain(leaf, stopAt string) []string {
	leaf = filepath.Clean(leaf)
	stopAt = filepath.Clean(stopAt)
	var chain []string
	cur := leaf
	for {
		chain = append(chain, cur)
		if cur == stopAt {
			break
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return chain
}

// NearestExistingAncestor walks upward from path until it finds an
// existing directory. It is used for the statfs preflight, which is a
// kernel call on an existing inode and genuinely cannot target a
// not-yet-created leaf (§7.1 step 2).
func NearestExistingAncestor(path string) (string, error) {
	cur := filepath.Clean(path)
	for {
		info, err := os.Stat(cur)
		if err == nil && info.IsDir() {
			return cur, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", os.ErrNotExist
		}
		cur = parent
	}
}

// RandomHex12 returns 12 lowercase hex characters, the per-invocation
// scratch/temp suffix convention (§7.1, §7.3 step 3).
func RandomHex12() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// OctalMode renders a file mode in the same 6-digit octal-string
// convention git uses for index entries (e.g. "100644"/"100755"),
// sourced from a plain os.Stat rather than the Git index (§5.1).
func OctalMode(mode fs.FileMode) string {
	perm := mode.Perm()
	if perm&0o111 != 0 {
		return "100755"
	}
	return "100644"
}

// HasPathPrefix reports whether child is path-equal to, or lexically
// inside, parent. Both must already be cleaned absolute paths.
func HasPathPrefix(child, parent string) bool {
	if child == parent {
		return true
	}
	return strings.HasPrefix(child, parent+string(filepath.Separator))
}
