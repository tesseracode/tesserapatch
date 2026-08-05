// Package testutil provides shared helpers for test setup across
// internal/ packages. PinGitAutoGCOff sets process-wide git env to
// prevent gc.autoDetach forks racing t.TempDir() teardown.
package testutil

import "os"

// PinGitAutoGCOff disables git's auto-gc for all git subprocesses
// spawned by tests in the calling test binary. Call from TestMain
// before m.Run(). Idempotent and safe to call multiple times.
// Fixes teardown race: unpinned `git commit` forks
// `git maintenance --auto --detach` which can touch .git/{info,objects}
// while `t.TempDir()` removes the tree (see docs/supervisor/LOG.md
// 2026-08-04 Cluster E entries).
func PinGitAutoGCOff() {
	_ = os.Setenv("GIT_CONFIG_COUNT", "1")
	_ = os.Setenv("GIT_CONFIG_KEY_0", "gc.auto")
	_ = os.Setenv("GIT_CONFIG_VALUE_0", "0")
}
