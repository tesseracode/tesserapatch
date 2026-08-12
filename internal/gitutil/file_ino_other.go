//go:build windows

package gitutil

import "io/fs"

// fileInoFromInfo reports no inode on platforms that do not expose one.
// Stale-lock identification then rests on the nonce alone, which is
// still the decisive check.
func fileInoFromInfo(fs.FileInfo) (uint64, bool) { return 0, false }
