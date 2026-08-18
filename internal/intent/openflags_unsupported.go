//go:build !(unix || windows)

package intent

// openFlags exists on unsupported targets for buildability only, and returns
// no flags.
//
// PRD rev-6 / ADR-034 rev-3 erratum: the accepted design wrote this partition
// as two halves, `!windows` and `windows`. That does not compile, because
// syscall.O_NONBLOCK is undeclared in `syscall` on js/wasm and plan9 — exactly
// the targets the platform allowlist refuses — so a `!windows` file naming the
// constant breaks the build for the unsupported set before any refusal can be
// reported. Three halves are therefore required: `unix`, `windows` and this
// one.
//
// This function is unreachable in production: RootConfinementSupported() is
// false here, and the CLI returns the workspace-unsupported-platform abort
// before os.OpenRoot is called and before any root-relative name is composed,
// so no capture and therefore no open ever happens on these targets
// (AVP-118, AVP-177, AVP-179, AVP-208).
func openFlags() int {
	return 0
}
