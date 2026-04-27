package workflow

import "testing"

// TestShellQuoteFor pins the OS-aware quoting contract introduced for
// M15-W2 review F4: SyntaxCheckCmd `{file}` substitution must produce
// quoting that the OS-selected shell (UserShell) actually strips, or
// quote characters leak into the invoked tool's argv.
func TestShellQuoteFor(t *testing.T) {
	tests := []struct {
		name string
		goos string
		in   string
		want string
	}{
		{"unix simple", "linux", "/tmp/foo.go", `'/tmp/foo.go'`},
		{"unix darwin", "darwin", "/Users/x/file.txt", `'/Users/x/file.txt'`},
		{"unix with quote", "linux", "it's.go", `'it'\''s.go'`},
		{"windows simple", "windows", `C:\Users\x\file.go`, `"C:\Users\x\file.go"`},
		{"windows with space", "windows", `C:\Program Files\x.go`, `"C:\Program Files\x.go"`},
		{"windows with quote", "windows", `a"b.go`, `"a""b.go"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellQuoteFor(tt.goos, tt.in)
			if got != tt.want {
				t.Errorf("shellQuoteFor(%q, %q) = %q, want %q", tt.goos, tt.in, got, tt.want)
			}
		})
	}
}

// TestShellQuoteFor_PairsWithUserShell guards the invariant that the
// quoting form for a given GOOS pairs with the UserShell selection
// for that GOOS — i.e. POSIX shells get POSIX quoting, cmd gets cmd
// quoting. A regression that changes one without the other would
// silently break the syntax-check gate on the affected platform.
func TestShellQuoteFor_PairsWithUserShell(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		shell, _ := userShellFor(goos)
		quoted := shellQuoteFor(goos, "/tmp/x")
		if shell == "cmd" && quoted[0] != '"' {
			t.Errorf("goos=%s shell=cmd but quoting is %q (expected double-quote form)", goos, quoted)
		}
		if shell == "sh" && quoted[0] != '\'' {
			t.Errorf("goos=%s shell=sh but quoting is %q (expected single-quote form)", goos, quoted)
		}
	}
}
