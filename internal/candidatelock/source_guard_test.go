package candidatelock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildTagContract(t *testing.T) {
	cases := []struct {
		file string
		tag  string
	}{
		{"lock_unix.go", "//go:build linux || darwin"},
		{"lock_windows.go", "//go:build windows"},
		{"lock_unsupported.go", "//go:build !linux && !darwin && !windows"},
		{"filesystem_linux.go", "//go:build linux"},
		{"filesystem_darwin.go", "//go:build darwin"},
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
		})
	}
}

func TestNoExternalSyscallDependency(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	needle := "golang.org/" + "x/sys"
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if strings.Contains(stripGoComments(string(body)), needle) {
			t.Fatalf("%s imports %s; candidatelock must remain stdlib-only", file, needle)
		}
	}
}

func TestNoExternalSyscallDependencyScannerSensitivity(t *testing.T) {
	const forbidden = "package p\nimport (\n\t\"fmt\"\n\t\"golang.org/x/sys/unix\"\n)\n"
	if !strings.Contains(stripGoComments(forbidden), "golang.org/x/sys") {
		t.Fatal("scanner missed an explicit forbidden import")
	}
	const clean = "package p\nimport \"syscall\"\n"
	if strings.Contains(stripGoComments(clean), "golang.org/x/sys") {
		t.Fatal("scanner flagged a stdlib syscall import")
	}
}

func stripGoComments(src string) string {
	var out strings.Builder
	inBlock := false
	for _, line := range strings.Split(src, "\n") {
		for {
			if inBlock {
				if idx := strings.Index(line, "*/"); idx >= 0 {
					line = line[idx+2:]
					inBlock = false
					continue
				}
				line = ""
				break
			}
			lineIdx := strings.Index(line, "//")
			blockIdx := strings.Index(line, "/*")
			switch {
			case blockIdx >= 0 && (lineIdx < 0 || blockIdx < lineIdx):
				out.WriteString(line[:blockIdx])
				line = line[blockIdx+2:]
				inBlock = true
			case lineIdx >= 0:
				out.WriteString(line[:lineIdx])
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
