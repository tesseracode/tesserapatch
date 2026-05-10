// Package buildinfo exposes the resolved tpatch version string.
//
// Resolution order:
//
//  1. ldflags-injected value (preferred — set by the Makefile to
//     `git describe --tags --always --dirty`):
//
//     -ldflags '-X github.com/tesseracode/tesserapatch/internal/buildinfo.Version=v0.6.2'
//
//  2. runtime/debug.ReadBuildInfo() — populated automatically by
//     `go install github.com/tesseracode/tesserapatch/cmd/tpatch@vX.Y.Z`
//     even when ldflags are not used.
//
//  3. The literal "dev" — for `go run`, `go build` without ldflags, and
//     pseudo-version builds where BuildInfo reports "(devel)".
//
// Keeping resolution in one package means the cobra command and any
// future consumers (telemetry, crash reports, log lines) all read the
// same string.
package buildinfo

import "runtime/debug"

// Version is the tpatch version. Override at build time with:
//
//	go build -ldflags "-X github.com/tesseracode/tesserapatch/internal/buildinfo.Version=v0.6.2" ./cmd/tpatch
//
// It must remain a `var` (not `const`) for the linker `-X` flag to
// rewrite it. The default "dev" sentinel signals "no ldflags injected"
// and triggers the BuildInfo fallback in String().
var Version = "dev"

// String returns the resolved version, applying the precedence above.
func String() string {
	return resolve(Version, debug.ReadBuildInfo)
}

// resolve is the testable core of String. It takes the current value of
// the package-level Version (so tests don't have to mutate global state)
// and a BuildInfo reader (so tests can inject fixtures).
func resolve(injected string, readInfo func() (*debug.BuildInfo, bool)) string {
	if injected != "" && injected != "dev" {
		return injected
	}
	if info, ok := readInfo(); ok {
		v := info.Main.Version
		if v != "" && v != "(devel)" {
			return v
		}
	}
	if injected == "" {
		return "dev"
	}
	return injected
}
