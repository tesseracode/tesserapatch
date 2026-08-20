package intentpub

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLinuxDescriptorCleanupArchitectureCoverage(t *testing.T) {
	groups := []struct {
		architectures []string
		source        string
		statSymbol    string
	}{
		{
			architectures: []string{"386", "arm", "mips", "mipsle"},
			source:        "sysnum_at_linux_fstatat64.go",
			statSymbol:    "SYS_FSTATAT64",
		},
		{
			architectures: []string{"amd64", "ppc64", "ppc64le", "s390x"},
			source:        "sysnum_unlinkat_linux.go",
			statSymbol:    "SYS_NEWFSTATAT",
		},
		{
			architectures: []string{"arm64", "riscv64"},
			source:        "sysnum_at_linux_fstatat.go",
			statSymbol:    "SYS_FSTATAT",
		},
	}
	covered := map[string]bool{}
	for _, group := range groups {
		source := readArchitectureSource(t, group.source)
		if !strings.Contains(source, group.statSymbol) ||
			!strings.Contains(source, "SYS_UNLINKAT") {
			t.Fatalf("%s lost %s or SYS_UNLINKAT", group.source, group.statSymbol)
		}
		for _, architecture := range group.architectures {
			if covered[architecture] {
				t.Fatalf("linux/%s appears in multiple descriptor-cleanup groups", architecture)
			}
			covered[architecture] = true
			stdlib := readArchitectureSource(
				t,
				filepath.Join(runtime.GOROOT(), "src", "syscall", "zsysnum_linux_"+architecture+".go"),
			)
			if !strings.Contains(stdlib, group.statSymbol) ||
				!strings.Contains(stdlib, "SYS_UNLINKAT") {
				t.Fatalf("stdlib linux/%s does not provide %s and SYS_UNLINKAT", architecture, group.statSymbol)
			}
		}
	}

	mipsSource := readArchitectureSource(t, "sysnum_at_linux_mips64x.go")
	mipsWrapper := readArchitectureSource(
		t,
		filepath.Join(runtime.GOROOT(), "src", "syscall", "syscall_linux_mips64x.go"),
	)
	if !strings.Contains(mipsSource, "syscall.Fstatat") ||
		strings.Contains(mipsSource, "rawStatAt") ||
		!strings.Contains(mipsWrapper, "func Fstatat") {
		t.Fatal("linux/mips64 and linux/mips64le must use the stdlib converting Fstatat wrapper")
	}
	for _, architecture := range []string{"mips64", "mips64le"} {
		numbers := readArchitectureSource(
			t,
			filepath.Join(runtime.GOROOT(), "src", "syscall", "zsysnum_linux_"+architecture+".go"),
		)
		if !strings.Contains(numbers, "SYS_UNLINKAT") {
			t.Fatalf("stdlib linux/%s lost SYS_UNLINKAT", architecture)
		}
		covered[architecture] = true
	}

	loongSource := readArchitectureSource(t, "sysnum_at_linux_loong64.go")
	loongSyscall := readArchitectureSource(
		t,
		filepath.Join(runtime.GOROOT(), "src", "syscall", "syscall_linux_loong64.go"),
	)
	loongNumbers := readArchitectureSource(
		t,
		filepath.Join(runtime.GOROOT(), "src", "syscall", "zsysnum_linux_loong64.go"),
	)
	if !strings.Contains(loongSource, "syscall.Fstatat") ||
		!strings.Contains(loongSyscall, "func Fstatat") ||
		!strings.Contains(loongNumbers, "SYS_UNLINKAT") {
		t.Fatal("linux/loong64 lost its stdlib statx-backed Fstatat or unlinkat mapping")
	}
	covered["loong64"] = true

	want := []string{
		"386", "amd64", "arm", "arm64", "loong64", "mips", "mipsle",
		"mips64", "mips64le", "ppc64", "ppc64le", "riscv64", "s390x",
	}
	for _, architecture := range want {
		if !covered[architecture] {
			t.Fatalf("linux/%s lacks descriptor-cleanup coverage", architecture)
		}
	}
	if len(covered) != len(want) {
		t.Fatalf("descriptor-cleanup architecture set = %v, want %v", covered, want)
	}
}

func readArchitectureSource(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
