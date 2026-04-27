package workflow

import "runtime"

// UserShell returns the (name, flag) pair used to invoke a user-supplied
// shell command via `exec.Command(name, flag, command)`. On Unix-like
// systems this is `sh -c`; on Windows it is `cmd /C`.
//
// The helper exists so call sites that previously hard-coded `sh -c`
// (test_command runner, syntax-check gate, `tpatch test`) keep working
// on Windows shells where /bin/sh is absent. Default Unix behaviour is
// byte-identical to the historical hard-coded path.
func UserShell() (name, flag string) {
	return userShellFor(runtime.GOOS)
}

// userShellFor is the testable seam: passing a fixed GOOS lets unit
// tests cover both branches without needing a Windows runner.
func userShellFor(goos string) (name, flag string) {
	if goos == "windows" {
		return "cmd", "/C"
	}
	return "sh", "-c"
}
