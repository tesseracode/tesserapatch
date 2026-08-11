// Platform-contract tests (PRD §7.2, ADR-033 D9).
//
// These verify the build-tag contract as *source shape* plus the
// architecture-agnostic normalization logic, which is what the design
// actually promises: the allow/deny comparison must be correct by
// construction on every width/signedness class, not merely on whichever
// architecture CI happens to run.

package rescap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuildTagContract pins the exact build tags §7.2 mandates. The
// broader `unix` tag is deliberately not used: it also expands to AIX,
// Solaris and other targets where syscall.Flock is neither verified nor
// covered by this project's CI matrix.
func TestBuildTagContract(t *testing.T) {
	cases := []struct {
		file string
		tag  string
	}{
		{"lock_unix.go", "//go:build linux || darwin"},
		{"lock_unsupported.go", "//go:build !linux && !darwin"},
		{"observer_unix.go", "//go:build linux || darwin"},
		{"observer_unsupported.go", "//go:build !linux && !darwin"},
		{"statfs_linux.go", "//go:build linux"},
		{"statfs_darwin.go", "//go:build darwin"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			body, err := os.ReadFile(tc.file)
			if err != nil {
				t.Fatalf("read %s: %v", tc.file, err)
			}
			text := string(body)
			if !strings.HasPrefix(text, tc.tag+"\n") {
				t.Fatalf("%s must open with %q", tc.file, tc.tag)
			}
			if strings.Contains(text, "//go:build unix") {
				t.Fatalf("%s must not use the broader unix build tag", tc.file)
			}
		})
	}
}

// TestNoExternalSyscallDependency proves the stdlib-only rule: no file
// in this package imports golang.org/x/sys.
func TestNoExternalSyscallDependency(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no source files found")
	}
	// The needle is assembled at runtime so this assertion does not
	// trip over its own source.
	needle := "golang.org/" + "x/sys"
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if strings.Contains(stripGoComments(string(body)), needle) {
			t.Fatalf("%s imports %s; this project is stdlib-only outside cobra/pflag", f, needle)
		}
	}
}

// TestObserverUsesRawWaitidWithWNOWAIT proves the observer is the
// non-reaping raw syscall the design requires, retried on EINTR, and
// that Setpgid is set so a -pgid signal can never reach tpatch's own
// process group.
func TestObserverUsesRawWaitidWithWNOWAIT(t *testing.T) {
	body, err := os.ReadFile("observer_unix.go")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	text := stripGoComments(string(body))
	for _, want := range []string{
		"syscall.SYS_WAITID",
		"syscall.WEXITED|syscall.WNOWAIT",
		"syscall.EINTR",
		"attr.Setpgid = true",
		"syscall.Kill(-pgid, sig)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("observer_unix.go must contain %q", want)
		}
	}
	if strings.Contains(text, "cmd.Wait") {
		t.Fatal("the observer must never call cmd.Wait()")
	}
}

// TestLockAndObserverSupportedOnThisTarget documents that this build
// target has both real primitives; the unsupported stubs are exercised
// by the cross-compile check in the Makefile/CI and by
// TestBuildTagContract above.
func TestLockAndObserverSupportedOnThisTarget(t *testing.T) {
	if !LockSupported {
		t.Fatal("linux/darwin builds must have a real flock")
	}
	if !ObserverSupported {
		t.Fatal("linux/darwin builds must have a real waitid observer")
	}
}

// TestRefusalTaxonomyExitCodes pins the three-way exit taxonomy so a
// refusal can never silently drift between classes.
func TestRefusalTaxonomyExitCodes(t *testing.T) {
	if Internal(ReasonTrackedBatchMissing, "x").ExitCode() != 1 {
		t.Fatal("internal refusals are exit 1")
	}
	if Invalid(ReasonDoltArgumentRefused, "x").ExitCode() != 2 {
		t.Fatal("validation refusals are exit 2")
	}
	if Refuse(ReasonNotIgnored, "x").ExitCode() != 3 {
		t.Fatal("state/policy refusals are exit 3")
	}
}

// TestEveryNamedRefusalIsUnique proves the "each named refusal appears
// in exactly one row" convention holds in the implementation: no reason
// string is reused across two exit-code classes.
func TestEveryNamedRefusalIsUnique(t *testing.T) {
	exit1 := []string{
		ReasonTrackedBatchMissing, ReasonAdapterCopyFailed, ReasonAdapterProcessObserverFail,
		ReasonAdapterGroupSignalFailed, ReasonAdapterReapTimeout, ReasonAdapterOutputReadFailed,
		ReasonNoResourcesDeclared, ReasonResourceDomainIncomplete,
	}
	exit2 := []string{
		ReasonDoltArgumentRefused, ReasonDoltTrustFlagRequired, ReasonAdapterMissingAtAdd,
		ReasonDoltContractUnsupported, ReasonResourceNotDoltAdapter, ReasonInvalidDeclaration,
		ReasonNoSuchResource, ReasonAmbiguousResourcePrefix,
	}
	exit3 := []string{
		ReasonNotIgnored, ReasonTrackedAndIgnored, ReasonGitIgnoreCheckError, ReasonGitLsFilesError,
		ReasonSymlinkComponentRefused, ReasonPathMissing, ReasonPathReplacedDuringOpen,
		ReasonPathOutsideRepo, ReasonResourceLimitExceeded, ReasonRedactionRefused,
		ReasonAdapterMissing, ReasonAdapterExecutableInRepo, ReasonAdapterBinaryUntrusted,
		ReasonDoltTrustRequired, ReasonAdapterCopyNoexec, ReasonDBPathIdentityChanged,
		ReasonDoltQueryError, ReasonDoltJSONParseError, ReasonLocalRootNotIgnored,
		ReasonLocalPathTracked, ReasonCaptureInProgress, ReasonResourceLockUnsupported,
		ReasonResourceLockFSUnsupported, ReasonBatchIDCollision, ReasonBatchFileCorrupt,
		ReasonResourcesFileCorrupt, ReasonResourceIDCollision, ReasonIndexEntryMissing,
		ReasonAdapterDrainTimeout,
	}
	seen := map[string]int{}
	for code, group := range map[int][]string{1: exit1, 2: exit2, 3: exit3} {
		for _, name := range group {
			if prev, dup := seen[name]; dup {
				t.Fatalf("%q appears under both exit %d and exit %d", name, prev, code)
			}
			seen[name] = code
		}
	}
	// The two deliberately-distinct pairs the design renamed apart.
	if ReasonDoltTrustFlagRequired == ReasonDoltTrustRequired {
		t.Fatal("the add-time and capture-time trust refusals must have distinct names")
	}
	if ReasonAdapterMissingAtAdd == ReasonAdapterMissing {
		t.Fatal("the add-time and capture-time missing-adapter refusals must have distinct names")
	}
}

// stripGoComments removes // and /* */ comments so a source-shape
// assertion inspects code rather than the prose that explains it.
func stripGoComments(src string) string {
	var out strings.Builder
	inBlock := false
	for _, line := range strings.Split(src, "\n") {
		for {
			if inBlock {
				idx := strings.Index(line, "*/")
				if idx < 0 {
					line = ""
					break
				}
				line = line[idx+2:]
				inBlock = false
				continue
			}
			lineIdx := strings.Index(line, "//")
			blockIdx := strings.Index(line, "/*")
			switch {
			case blockIdx >= 0 && (lineIdx < 0 || blockIdx < lineIdx):
				out.WriteString(line[:blockIdx])
				line = line[blockIdx+2:]
				inBlock = true
			case lineIdx >= 0:
				line = line[:lineIdx]
				out.WriteString(line)
				line = ""
			default:
				out.WriteString(line)
				line = ""
			}
			if line == "" {
				break
			}
		}
		out.WriteString("\n")
	}
	return out.String()
}
