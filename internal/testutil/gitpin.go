// Package testutil provides shared helpers for test setup across
// internal/ packages. PinGitAutoGCOff sets process-wide git env to
// prevent detached Git maintenance racing t.TempDir() teardown.
package testutil

import "os"

// PinGitAutoGCOff disables legacy auto-gc and modern auto-maintenance for all
// Git subprocesses spawned by tests in the calling test binary. Call from
// TestMain before m.Run(). Idempotent and safe to call multiple times.
// Fixes teardown race: an unpinned `git commit` can fork
// `git maintenance --auto --detach`, which keeps touching .git/{info,objects}
// after the parent exits while `t.TempDir()` removes the tree (see
// docs/supervisor/LOG.md 2026-08-04 Cluster E entries).
//
// Note: this unconditionally sets GIT_CONFIG_COUNT=4, so any
// pre-existing GIT_CONFIG_KEY_N/VALUE_N entries in the environment
// are discarded. Today nothing else sets these, so the clobber is harmless.
// If a future test needs another env-config key, extend this helper to append
// rather than overwrite.
func PinGitAutoGCOff() {
	_ = os.Setenv("GIT_CONFIG_COUNT", "4")
	_ = os.Setenv("GIT_CONFIG_KEY_0", "gc.auto")
	_ = os.Setenv("GIT_CONFIG_VALUE_0", "0")
	_ = os.Setenv("GIT_CONFIG_KEY_1", "gc.autoDetach")
	_ = os.Setenv("GIT_CONFIG_VALUE_1", "false")
	_ = os.Setenv("GIT_CONFIG_KEY_2", "maintenance.auto")
	_ = os.Setenv("GIT_CONFIG_VALUE_2", "false")
	_ = os.Setenv("GIT_CONFIG_KEY_3", "maintenance.autoDetach")
	_ = os.Setenv("GIT_CONFIG_VALUE_3", "false")
}
