//go:build (linux && !android) || (darwin && !ios)

package intentlock

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestS7APAuthorityContracts(t *testing.T) {
	t.Run("PIB-478", func(t *testing.T) {
		for _, rel := range []string{"statfs_linux.go", "statfs_darwin.go"} {
			source, err := os.ReadFile(rel)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateS7APHeldFstatfsSource(rel, string(source)); err != nil {
				t.Fatal(err)
			}
		}
		root := t.TempDir()
		events := []string{}
		ops := defaultAuthorityOps
		var opened *os.File
		var openedIdentity nativeIdentity
		var classifiedDescriptor uintptr
		var flockedDescriptor uintptr
		ops.openRoot = func(path string) (*os.Root, error) {
			events = append(events, "open-root")
			return os.OpenRoot(path)
		}
		ops.openDir = func(rooted *os.Root) (*os.File, error) {
			file, err := rooted.Open(".")
			opened = file
			events = append(events, "open-dot")
			return file, err
		}
		ops.fileIdentity = func(file *os.File) (nativeIdentity, error) {
			if file != opened {
				return nativeIdentity{}, errors.New("identity did not receive retained descriptor")
			}
			events = append(events, "identity")
			identity, err := identityFromFile(file)
			openedIdentity = identity
			return identity, err
		}
		ops.classify = func(file *os.File) (string, bool, error) {
			if file != opened {
				return "", false, errors.New("classifier did not receive retained descriptor")
			}
			identity, err := identityFromFile(file)
			if err != nil || identity != openedIdentity {
				return "", false, errors.New("classifier inode differs from retained identity")
			}
			events = append(events, "classify-fstatfs")
			classifiedDescriptor = s7APHeldDescriptor(t, file)
			return classifyHeldFilesystem(file)
		}
		ops.lock = func(file *os.File) error {
			if file != opened {
				return errors.New("flock did not receive classified descriptor")
			}
			identity, err := identityFromFile(file)
			if err != nil || identity != openedIdentity {
				return errors.New("flock inode differs from classified inode")
			}
			events = append(events, "flock-control")
			flockedDescriptor = s7APHeldDescriptor(t, file)
			return lockHeldDirectory(file)
		}
		authority, err := acquireWithOps(root, ops)
		if err != nil || authority == nil ||
			authority.directory != opened || authority.identity != openedIdentity ||
			classifiedDescriptor == 0 || classifiedDescriptor != flockedDescriptor ||
			!reflect.DeepEqual(events, []string{
				"open-root", "open-dot", "identity", "classify-fstatfs", "flock-control",
			}) {
			t.Fatalf("PIB-478 held descriptor workflow = authority:%v err:%v events:%v",
				authority, err, events)
		}
		for _, rel := range []string{"statfs_linux.go", "statfs_darwin.go"} {
			source, err := os.ReadFile(rel)
			if err != nil {
				t.Fatal(err)
			}
			wrongReceiver := strings.Replace(
				string(source),
				"raw, err := file.SyscallConn()",
				"other := file\n\traw, err := other.SyscallConn()",
				1,
			)
			if err := validateS7APHeldFstatfsSource(rel, wrongReceiver); err == nil {
				t.Fatalf("PIB-478 %s validator accepted another SyscallConn receiver", rel)
			}
			wrongDescriptor := strings.Replace(
				string(source),
				"raw, err := file.SyscallConn()",
				"other, _ := os.Open(\".\")\n\totherDescriptor := other.Fd()\n\traw, err := file.SyscallConn()",
				1,
			)
			wrongDescriptor = strings.Replace(
				wrongDescriptor,
				"syscall.Fstatfs(int(descriptor), &stat)",
				"syscall.Fstatfs(int(otherDescriptor), &stat)",
				1,
			)
			if err := validateS7APHeldFstatfsSource(rel, wrongDescriptor); err == nil {
				t.Fatalf("PIB-478 %s validator accepted a captured other descriptor", rel)
			}
			wrongConversion := strings.Replace(
				string(source),
				"syscall.Fstatfs(int(descriptor), &stat)",
				"syscall.Fstatfs(replaceFD(descriptor), &stat)",
				1,
			) + "\nfunc replaceFD(value uintptr) int { return int(value) }\n"
			if err := validateS7APHeldFstatfsSource(rel, wrongConversion); err == nil {
				t.Fatalf("PIB-478 %s validator accepted a non-builtin descriptor conversion", rel)
			}
			newBuffer := strings.Replace(
				string(source),
				"syscall.Fstatfs(int(descriptor), &stat)",
				"syscall.Fstatfs(int(descriptor), new(syscall.Statfs_t))",
				1,
			)
			if err := validateS7APHeldFstatfsSource(rel, newBuffer); err == nil {
				t.Fatalf("PIB-478 %s validator accepted a fresh Fstatfs buffer", rel)
			}
			otherBuffer := strings.Replace(
				string(source),
				"var stat syscall.Statfs_t",
				"var stat syscall.Statfs_t\n\tvar other syscall.Statfs_t",
				1,
			)
			otherBuffer = strings.Replace(
				otherBuffer,
				"syscall.Fstatfs(int(descriptor), &stat)",
				"syscall.Fstatfs(int(descriptor), &other)",
				1,
			)
			if err := validateS7APHeldFstatfsSource(rel, otherBuffer); err == nil {
				t.Fatalf("PIB-478 %s validator accepted another Fstatfs buffer", rel)
			}
			otherClassifierBuffer := strings.Replace(
				string(source),
				"var stat syscall.Statfs_t",
				"var stat syscall.Statfs_t\n\tvar other syscall.Statfs_t",
				1,
			)
			if rel == "statfs_linux.go" {
				otherClassifierBuffer = strings.Replace(
					otherClassifierBuffer,
					"normalizeLinuxStatfsType(stat.Type)",
					"normalizeLinuxStatfsType(other.Type)",
					1,
				)
			} else {
				otherClassifierBuffer = strings.Replace(
					otherClassifierBuffer,
					"darwinFilesystemName(stat.Fstypename[:])",
					"darwinFilesystemName(other.Fstypename[:])",
					1,
				)
			}
			if err := validateS7APHeldFstatfsSource(rel, otherClassifierBuffer); err == nil {
				t.Fatalf("PIB-478 %s validator accepted another classifier buffer", rel)
			}
			aliasedBuffer := strings.Replace(
				string(source),
				"var statErr error",
				"var statErr error\n\tstatAlias := &stat",
				1,
			)
			aliasedBuffer = strings.Replace(
				aliasedBuffer,
				"syscall.Fstatfs(int(descriptor), &stat)",
				"syscall.Fstatfs(int(descriptor), statAlias)",
				1,
			)
			if err := validateS7APHeldFstatfsSource(rel, aliasedBuffer); err == nil {
				t.Fatalf("PIB-478 %s validator accepted an aliased Fstatfs buffer", rel)
			}
			for name, replacement := range map[string]string{
				"blank-assignment": `_ = syscall.Fstatfs(int(descriptor), &stat)`,
				"ignored-result":   `syscall.Fstatfs(int(descriptor), &stat)`,
				"swapped-error":    `err = syscall.Fstatfs(int(descriptor), &stat)`,
			} {
				mutated := strings.Replace(
					string(source),
					`statErr = syscall.Fstatfs(int(descriptor), &stat)`,
					replacement,
					1,
				)
				if err := validateS7APHeldFstatfsSource(rel, mutated); err == nil {
					t.Fatalf("PIB-478 %s validator accepted %s Fstatfs result", rel, name)
				}
			}
			otherError := strings.Replace(
				string(source),
				"var statErr error",
				"var statErr error\n\tvar otherStatErr error",
				1,
			)
			otherError = strings.Replace(
				otherError,
				`statErr = syscall.Fstatfs(int(descriptor), &stat)`,
				`otherStatErr = syscall.Fstatfs(int(descriptor), &stat)`,
				1,
			)
			if err := validateS7APHeldFstatfsSource(rel, otherError); err == nil {
				t.Fatalf("PIB-478 %s validator accepted another Fstatfs error object", rel)
			}
			if rel == "statfs_linux.go" {
				for name, replacement := range map[string]string{
					"zero-multiplied": `classifyLinuxFilesystem(0 * normalizeLinuxStatfsType(stat.Type))`,
					"add-zero":        `classifyLinuxFilesystem(normalizeLinuxStatfsType(stat.Type) + 0)`,
					"other-field":     `classifyLinuxFilesystem(normalizeLinuxStatfsType(stat.Bsize))`,
				} {
					mutated := strings.Replace(
						string(source),
						`classifyLinuxFilesystem(normalizeLinuxStatfsType(stat.Type))`,
						replacement,
						1,
					)
					if err := validateS7APHeldFstatfsSource(rel, mutated); err == nil {
						t.Fatalf("PIB-478 Linux validator accepted %s classifier input", name)
					}
				}
				mutatedHelper := strings.Replace(
					string(source),
					"return uint32(uint64(value))",
					"return uint32(uint64(value) + 0)",
					1,
				)
				if err := validateS7APHeldFstatfsSource(rel, mutatedHelper); err == nil {
					t.Fatal("PIB-478 Linux validator accepted mutated transformation helper")
				}
			} else {
				wrapped := strings.Replace(
					string(source), "\t\"os\"\n", "\t\"os\"\n\t\"strings\"\n", 1,
				)
				wrapped = strings.Replace(
					wrapped,
					`classifyDarwinFilesystem(darwinFilesystemName(stat.Fstypename[:]))`,
					`classifyDarwinFilesystem(strings.TrimSpace(darwinFilesystemName(stat.Fstypename[:])))`,
					1,
				)
				if err := validateS7APHeldFstatfsSource(rel, wrapped); err == nil {
					t.Fatal("PIB-478 Darwin validator accepted wrapped classifier input")
				}
				otherField := strings.Replace(
					string(source),
					`darwinFilesystemName(stat.Fstypename[:])`,
					`darwinFilesystemName(stat.Mntonname[:])`,
					1,
				)
				if err := validateS7APHeldFstatfsSource(rel, otherField); err == nil {
					t.Fatal("PIB-478 Darwin validator accepted another sampled field")
				}
				mutatedHelper := strings.Replace(
					string(source),
					"return string(name)",
					"return string(append([]byte(nil), name...))",
					1,
				)
				if err := validateS7APHeldFstatfsSource(rel, mutatedHelper); err == nil {
					t.Fatal("PIB-478 Darwin validator accepted mutated conversion helper")
				}
			}
			wrongMember := strings.Replace(
				string(source),
				"syscall.Fstatfs(int(descriptor), &stat)",
				"s7APLocalFstatfsValue.Fstatfs(int(descriptor), &stat)",
				1,
			) + `
type s7APLocalFstatfs struct{}
func (s7APLocalFstatfs) Fstatfs(int, *syscall.Statfs_t) error { return nil }
var s7APLocalFstatfsValue s7APLocalFstatfs
`
			if err := validateS7APHeldFstatfsSource(rel, wrongMember); err == nil {
				t.Fatalf("PIB-478 %s validator accepted a local Fstatfs member", rel)
			}
			shadowed := strings.Replace(string(source), "\n\t\"syscall\"", "", 1)
			shadowed = strings.Replace(
				shadowed,
				"var stat syscall.Statfs_t",
				"var stat s7APShadowStatfs",
				1,
			)
			shadowed += `
type s7APShadowStatfs struct {
	Type       int64
	Fstypename [16]int8
}
type s7APShadowSyscall struct{}
func (s7APShadowSyscall) Fstatfs(int, *s7APShadowStatfs) error { return nil }
var syscall s7APShadowSyscall
`
			if err := validateS7APHeldFstatfsSource(rel, shadowed); err == nil {
				t.Fatalf("PIB-478 %s validator accepted a shadowed syscall binding", rel)
			}
		}
		if err := authority.Release(); err != nil {
			t.Fatal(err)
		}
		for _, rel := range []string{
			"acquire_supported.go", "statfs_linux.go", "statfs_darwin.go",
		} {
			source, err := os.ReadFile(rel)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(source), "syscall.Statfs(") ||
				strings.Contains(string(source), "unix.Statfs(") {
				t.Fatalf("PIB-478 %s contains path-based statfs", rel)
			}
		}
	})

	t.Run("PIB-479", func(t *testing.T) {
		linux, err := os.ReadFile("statfs_linux.go")
		if err != nil {
			t.Fatal(err)
		}
		darwin, err := os.ReadFile("statfs_darwin.go")
		if err != nil {
			t.Fatal(err)
		}
		if err := validateS7APFilesystemTables(string(linux), string(darwin)); err != nil {
			t.Fatal(err)
		}
		acquire, err := os.ReadFile("acquire_supported.go")
		if err != nil {
			t.Fatal(err)
		}
		if err := validateS7APAcquireClassifierGate(string(acquire)); err != nil {
			t.Fatal(err)
		}
		wrongAcquire := strings.Replace(
			string(acquire),
			"\tif denied {",
			"\tdenied = class == \"sshfs\"\n\tif denied {",
			1,
		)
		if err := validateS7APAcquireClassifierGate(wrongAcquire); err == nil {
			t.Fatal("PIB-479 same validator accepted a post-classifier acquisition mutation")
		}
		wrongLinux := strings.Replace(
			string(linux),
			"default:",
			"case 0xDEADBEEF:\n\t\treturn \"remote\", true\n\tdefault:",
			1,
		)
		if err := validateS7APFilesystemTables(wrongLinux, string(darwin)); err == nil {
			t.Fatal("PIB-479 same validator accepted an extra literal denied magic")
		}
		wrongDarwin := strings.Replace(
			string(darwin),
			"func classifyFilesystemType(name string) (string, bool) {",
			"const s7APRemoteDarwinFilesystem = \"afpfs\"\n\nfunc classifyFilesystemType(name string) (string, bool) {",
			1,
		)
		wrongDarwin = strings.Replace(
			wrongDarwin,
			"default:",
			"case s7APRemoteDarwinFilesystem:\n\t\treturn \"remote\", true\n\tdefault:",
			1,
		)
		if err := validateS7APFilesystemTables(string(linux), wrongDarwin); err == nil {
			t.Fatal("PIB-479 same validator accepted an aliased Darwin deny case")
		}
		wrongLinuxBranch := strings.Replace(
			string(linux),
			"func classifyLinuxFilesystem(fsType uint32) (string, bool) {\n\tswitch fsType {",
			"func classifyLinuxFilesystem(fsType uint32) (string, bool) {\n\tif fsType == 0x12345678 { return \"remote\", true }\n\tswitch fsType {",
			1,
		)
		if err := validateS7APFilesystemTables(wrongLinuxBranch, string(darwin)); err == nil {
			t.Fatal("PIB-479 same validator accepted a Linux denial branch outside the closed switch")
		}
		wrongDarwinBranch := strings.Replace(
			string(darwin),
			"func classifyDarwinFilesystem(name string) (string, bool) {\n\tswitch name {",
			"func classifyDarwinFilesystem(name string) (string, bool) {\n\tif name == \"afpfs\" { return \"remote\", true }\n\tswitch name {",
			1,
		)
		if err := validateS7APFilesystemTables(string(linux), wrongDarwinBranch); err == nil {
			t.Fatal("PIB-479 same validator accepted a Darwin denial branch outside the closed switch")
		}
		wrongLinuxHelper := strings.Replace(
			string(linux),
			"func classifyLinuxFilesystem(fsType uint32) (string, bool) {\n\tswitch fsType {",
			"func classifyLinuxFilesystem(fsType uint32) (string, bool) {\n\tif s7APDeniedFilesystem(fsType) { return \"remote\", true }\n\tswitch fsType {",
			1,
		) + "\nfunc s7APDeniedFilesystem(uint32) bool { return true }\n"
		if err := validateS7APFilesystemTables(wrongLinuxHelper, string(darwin)); err == nil {
			t.Fatal("PIB-479 same validator accepted a helper denial path outside the closed switch")
		}
		for rel, source := range map[string]string{
			"linux": string(linux), "darwin": string(darwin),
		} {
			nilChecked := strings.Replace(
				source, "if err != nil {", "if err == nil {", 1,
			)
			var nilCheckedErr error
			if rel == "linux" {
				nilCheckedErr = validateS7APFilesystemTables(nilChecked, string(darwin))
			} else {
				nilCheckedErr = validateS7APFilesystemTables(string(linux), nilChecked)
			}
			if nilCheckedErr == nil {
				t.Fatalf("PIB-479 same validator accepted %s nil-checked error return", rel)
			}
			const errorGuard = `if err != nil {
		return "", false, err
	}`
			unrelated := strings.Replace(
				source,
				errorGuard,
				errorGuard+`
	if s7APDeniedCondition() {
		return "", false, err
	}`,
				1,
			) + "\nfunc s7APDeniedCondition() bool { return true }\n"
			var unrelatedErr error
			if rel == "linux" {
				unrelatedErr = validateS7APFilesystemTables(unrelated, string(darwin))
			} else {
				unrelatedErr = validateS7APFilesystemTables(string(linux), unrelated)
			}
			if unrelatedErr == nil {
				t.Fatalf("PIB-479 same validator accepted %s error return under unrelated condition", rel)
			}
			swapped := strings.Replace(
				source,
				`if statErr != nil {
		return "", false, statErr
	}`,
				`if statErr != nil {
		return "", false, err
	}`,
				1,
			)
			var swappedErr error
			if rel == "linux" {
				swappedErr = validateS7APFilesystemTables(swapped, string(darwin))
			} else {
				swappedErr = validateS7APFilesystemTables(string(linux), swapped)
			}
			if swappedErr == nil {
				t.Fatalf("PIB-479 same validator accepted %s swapped returned error", rel)
			}
			synthetic := strings.Replace(
				source,
				errorGuard,
				errorGuard+`
	err = syscall.EINVAL
	if err != nil {
		return "", false, err
	}`,
				1,
			)
			var syntheticErr error
			if rel == "linux" {
				syntheticErr = validateS7APFilesystemTables(synthetic, string(darwin))
			} else {
				syntheticErr = validateS7APFilesystemTables(string(linux), synthetic)
			}
			if syntheticErr == nil {
				t.Fatalf("PIB-479 same validator accepted %s synthetic canonical error guard", rel)
			}
			reassigned := strings.Replace(
				source,
				`if statErr != nil {
		return "", false, statErr
	}`,
				`if statErr != nil {
		return "", false, statErr
	}
	statErr = syscall.EINVAL`,
				1,
			)
			var reassignedErr error
			if rel == "linux" {
				reassignedErr = validateS7APFilesystemTables(reassigned, string(darwin))
			} else {
				reassignedErr = validateS7APFilesystemTables(string(linux), reassigned)
			}
			if reassignedErr == nil {
				t.Fatalf("PIB-479 same validator accepted %s post-guard error reassignment", rel)
			}
			manual := source
			if rel == "darwin" {
				manual = strings.Replace(manual, "\t\"os\"\n", "\t\"fmt\"\n\t\"os\"\n", 1)
			}
			manual = strings.Replace(
				manual,
				`if statErr != nil {
		return "", false, statErr
	}`,
				`if statErr != nil {
		return "", false, statErr
	}
	statErr = fmt.Errorf("synthetic filesystem error")`,
				1,
			)
			var manualErr error
			if rel == "linux" {
				manualErr = validateS7APFilesystemTables(manual, string(darwin))
			} else {
				manualErr = validateS7APFilesystemTables(string(linux), manual)
			}
			if manualErr == nil {
				t.Fatalf("PIB-479 same validator accepted %s manual error provenance", rel)
			}
			phiCondition := "stat.Type == 0"
			if rel == "darwin" {
				phiCondition = "len(stat.Fstypename) == 0"
			}
			phi := strings.Replace(
				source,
				`if statErr != nil {
		return "", false, statErr
	}`,
				`if statErr != nil {
		return "", false, statErr
	}
	if `+phiCondition+` {
		statErr = syscall.EINVAL
	}`,
				1,
			)
			var phiErr error
			if rel == "linux" {
				phiErr = validateS7APFilesystemTables(phi, string(darwin))
			} else {
				phiErr = validateS7APFilesystemTables(string(linux), phi)
			}
			if phiErr == nil {
				t.Fatalf("PIB-479 same validator accepted %s multiple-definition error phi", rel)
			}
			unresolved := strings.Replace(
				source,
				"\tvar stat syscall.Statfs_t",
				`	var mystery error
	if mystery != nil {
		return "", false, mystery
	}
	var stat syscall.Statfs_t`,
				1,
			)
			var unresolvedErr error
			if rel == "linux" {
				unresolvedErr = validateS7APFilesystemTables(unresolved, string(darwin))
			} else {
				unresolvedErr = validateS7APFilesystemTables(string(linux), unresolved)
			}
			if unresolvedErr == nil {
				t.Fatalf("PIB-479 same validator accepted %s unresolved error provenance", rel)
			}
			const statGuard = `if statErr != nil {
		return "", false, statErr
	}`
			const controlGuard = `if err := raw.Control(func(descriptor uintptr) {
		statErr = syscall.Fstatfs(int(descriptor), &stat)
	}); err != nil {
		return "", false, err
	}`
			for name, mutate := range map[string]func(string) string{
				"missing-syscallconn-guard": func(value string) string {
					return strings.Replace(value, errorGuard, "", 1)
				},
				"missing-staterr-guard": func(value string) string {
					return strings.Replace(value, statGuard, "", 1)
				},
				"missing-control-guard": func(value string) string {
					return strings.Replace(
						value,
						controlGuard,
						`controlErr := raw.Control(func(descriptor uintptr) {
		statErr = syscall.Fstatfs(int(descriptor), &stat)
	})
	_ = controlErr`,
						1,
					)
				},
				"duplicate-staterr-guard": func(value string) string {
					return strings.Replace(value, statGuard, statGuard+"\n\t"+statGuard, 1)
				},
				"staterr-guard-wrong-object": func(value string) string {
					return strings.Replace(
						value,
						statGuard,
						`if err != nil {
		return "", false, err
	}`,
						1,
					)
				},
				"delayed-staterr-guard": func(value string) string {
					return strings.Replace(
						value,
						statGuard,
						"_ = stat\n\t"+statGuard,
						1,
					)
				},
			} {
				mutated := mutate(source)
				if mutated == source {
					t.Fatalf("PIB-479 %s %s mutation anchor missing", rel, name)
				}
				var guardErr error
				if rel == "linux" {
					guardErr = validateS7APFilesystemTables(mutated, string(darwin))
				} else {
					guardErr = validateS7APFilesystemTables(string(linux), mutated)
				}
				if guardErr == nil {
					t.Fatalf("PIB-479 same validator accepted %s %s", rel, name)
				}
			}
			for name, mutate := range map[string]func(string) string{
				"stat-initialized-declaration": func(value string) string {
					return strings.Replace(
						value,
						"var stat syscall.Statfs_t",
						"var stat = syscall.Statfs_t{}",
						1,
					)
				},
				"stat-reassignment": func(value string) string {
					return strings.Replace(
						value,
						statGuard,
						statGuard+"\n\tstat = syscall.Statfs_t{}",
						1,
					)
				},
				"stat-guard-initializer-assignment": func(value string) string {
					return strings.Replace(
						value,
						statGuard,
						statGuard+`
	if stat = (syscall.Statfs_t{}); statErr == nil {
	}`,
						1,
					)
				},
				"stat-pointer-alias-write": func(value string) string {
					return strings.Replace(
						value,
						statGuard,
						statGuard+`
	statPointer := &stat
	*statPointer = syscall.Statfs_t{}`,
						1,
					)
				},
				"stat-helper-address-escape": func(value string) string {
					value = strings.Replace(
						value,
						statGuard,
						statGuard+"\n\ts7APMutateSampledStat(&stat)",
						1,
					)
					return value + `
func s7APMutateSampledStat(stat *syscall.Statfs_t) {
	*stat = syscall.Statfs_t{}
}
`
				},
				"stat-closure-write": func(value string) string {
					return strings.Replace(
						value,
						statGuard,
						statGuard+`
	func() {
		stat = syscall.Statfs_t{}
	}()`,
						1,
					)
				},
				"stat-field-write": func(value string) string {
					fieldWrite := "stat.Type = 0"
					if rel == "darwin" {
						fieldWrite = "stat.Fstypename[0] = 0"
					}
					return strings.Replace(
						value,
						statGuard,
						statGuard+"\n\t"+fieldWrite,
						1,
					)
				},
				"stat-compound-field-write": func(value string) string {
					fieldWrite := "stat.Type += 1"
					if rel == "darwin" {
						fieldWrite = "stat.Fstypename[0] += 1"
					}
					return strings.Replace(
						value,
						statGuard,
						statGuard+"\n\t"+fieldWrite,
						1,
					)
				},
				"stat-range-key-write": func(value string) string {
					return strings.Replace(
						value,
						statGuard,
						statGuard+`
	statValues := map[syscall.Statfs_t]struct{}{}
	for stat = range statValues {
	}`,
						1,
					)
				},
				"stat-range-value-write": func(value string) string {
					return strings.Replace(
						value,
						statGuard,
						statGuard+`
	statValues := []syscall.Statfs_t{}
	for _, stat = range statValues {
	}`,
						1,
					)
				},
				"stat-select-receive-write": func(value string) string {
					return strings.Replace(
						value,
						statGuard,
						statGuard+`
	statValues := make(chan syscall.Statfs_t)
	select {
	case stat = <-statValues:
	default:
	}`,
						1,
					)
				},
			} {
				mutated := mutate(source)
				if mutated == source {
					t.Fatalf("PIB-479 %s %s mutation anchor missing", rel, name)
				}
				var provenanceErr error
				if rel == "linux" {
					provenanceErr = validateS7APFilesystemTables(mutated, string(darwin))
				} else {
					provenanceErr = validateS7APFilesystemTables(string(linux), mutated)
				}
				if provenanceErr == nil {
					t.Fatalf("PIB-479 same validator accepted %s %s", rel, name)
				}
			}
			for name, mutation := range map[string]string{
				"address-dereference-write": `if statErr != nil {
		return "", false, statErr
	}
	*(&statErr) = syscall.EINVAL`,
				"pointer-alias-write": `if statErr != nil {
		return "", false, statErr
	}
	statErrPointer := &statErr
	*statErrPointer = syscall.EINVAL`,
				"closure-write": `if statErr != nil {
		return "", false, statErr
	}
	func() {
		statErr = syscall.EINVAL
	}()`,
			} {
				aliasedWrite := strings.Replace(
					source,
					`if statErr != nil {
		return "", false, statErr
	}`,
					mutation,
					1,
				)
				var aliasedWriteErr error
				if rel == "linux" {
					aliasedWriteErr = validateS7APFilesystemTables(
						aliasedWrite, string(darwin),
					)
				} else {
					aliasedWriteErr = validateS7APFilesystemTables(
						string(linux), aliasedWrite,
					)
				}
				if aliasedWriteErr == nil {
					t.Fatalf("PIB-479 same validator accepted %s %s", rel, name)
				}
			}
			early := strings.Replace(
				source,
				"\traw, err := file.SyscallConn()",
				"\tif true { return \"remote\", true, nil }\n\traw, err := file.SyscallConn()",
				1,
			)
			var earlyErr error
			if rel == "linux" {
				earlyErr = validateS7APFilesystemTables(early, string(darwin))
			} else {
				earlyErr = validateS7APFilesystemTables(string(linux), early)
			}
			if earlyErr == nil {
				t.Fatalf("PIB-479 same validator accepted %s early outer denial", rel)
			}
			helper := strings.Replace(
				source,
				"\traw, err := file.SyscallConn()",
				"\tif s7APOuterDenied() { return \"remote\", true, nil }\n\traw, err := file.SyscallConn()",
				1,
			) + "\nfunc s7APOuterDenied() bool { return true }\n"
			var helperErr error
			if rel == "linux" {
				helperErr = validateS7APFilesystemTables(helper, string(darwin))
			} else {
				helperErr = validateS7APFilesystemTables(string(linux), helper)
			}
			if helperErr == nil {
				t.Fatalf("PIB-479 same validator accepted %s helper outer denial", rel)
			}
			mutated := strings.Replace(source, "import (", "import (\n\t\"strings\"", 1)
			mutated = strings.Replace(
				mutated,
				"\treturn name, denied, nil",
				"\tif strings.Contains(name, \"sshfs\") { denied = true }\n\treturn name, denied, nil",
				1,
			)
			var validateErr error
			if rel == "linux" {
				validateErr = validateS7APFilesystemTables(mutated, string(darwin))
			} else {
				validateErr = validateS7APFilesystemTables(string(linux), mutated)
			}
			if validateErr == nil {
				t.Fatalf("PIB-479 same validator accepted %s post-classifier denial mutation", rel)
			}
		}
	})

	t.Run("PIB-481", func(t *testing.T) {
		root := t.TempDir()
		events := []string{}
		ops := defaultAuthorityOps
		ops.unlock = func(file *os.File) error {
			events = append(events, "unlock-control")
			return unlockHeldDirectory(file)
		}
		ops.closeFile = func(file *os.File) error {
			events = append(events, "close-directory")
			return file.Close()
		}
		ops.closeRoot = func(rooted *os.Root) error {
			events = append(events, "close-root")
			return rooted.Close()
		}
		authority, err := acquireWithOps(root, ops)
		if err != nil {
			t.Fatal(err)
		}
		releasedRaw, err := authority.directory.SyscallConn()
		if err != nil {
			t.Fatal(err)
		}
		if err := authority.Release(); err != nil {
			t.Fatal(err)
		}
		controlCallbacks := 0
		if err := releasedRaw.Control(func(uintptr) { controlCallbacks++ }); err == nil ||
			controlCallbacks != 0 {
			t.Fatalf("PIB-481 post-release Control = callbacks:%d err:%v",
				controlCallbacks, err)
		}
		if err := authority.Release(); err == nil {
			t.Fatal("PIB-481 second release unexpectedly succeeded")
		}
		if !reflect.DeepEqual(events, []string{"unlock-control", "close-directory", "close-root"}) {
			t.Fatalf("PIB-481 release sequence = %v", events)
		}
		reacquired, err := Acquire(root)
		if err != nil {
			t.Fatalf("PIB-481 explicit release left live-lock residue: %v", err)
		}
		if err := reacquired.Release(); err != nil {
			t.Fatal(err)
		}

		concurrent, err := Acquire(root)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := concurrent.directory.SyscallConn()
		if err != nil {
			t.Fatal(err)
		}
		started := make(chan struct{})
		releaseControl := make(chan struct{})
		controlDone := make(chan error, 1)
		go func() {
			controlDone <- raw.Control(func(uintptr) {
				close(started)
				<-releaseControl
			})
		}()
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("PIB-481 Control callback did not start")
		}
		closeDone := make(chan error, 1)
		go func() { closeDone <- concurrent.directory.Close() }()
		time.Sleep(25 * time.Millisecond)
		contender, contenderErr := Acquire(root)
		if contender != nil {
			_ = contender.Release()
			t.Fatal("PIB-481 concurrent close allowed a second live authority")
		}
		var typed *Error
		if !errors.As(contenderErr, &typed) || typed.Code != CodeTransactionInProgress {
			t.Fatalf("PIB-481 concurrent-close contender = %v", contenderErr)
		}
		close(releaseControl)
		if err := <-controlDone; err != nil {
			t.Fatal(err)
		}
		if err := <-closeDone; err != nil {
			t.Fatal(err)
		}
		err = concurrent.Release()
		if !errors.As(err, &typed) || typed.Code != CodeDirectoryFlockUnavailable {
			t.Fatalf("PIB-481 concurrent-close misuse = %v", err)
		}
		source, err := os.ReadFile("authority.go")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(source),
			"It is single-goroutine owned: acquisition, rooted use, validation, and the") {
			t.Fatal("PIB-481 single-goroutine ownership contract is missing")
		}
	})
}

func s7APHeldDescriptor(t *testing.T, file *os.File) uintptr {
	t.Helper()
	raw, err := file.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var descriptor uintptr
	if err := raw.Control(func(value uintptr) { descriptor = value }); err != nil {
		t.Fatal(err)
	}
	return descriptor
}

func validateS7APHeldFstatfsSource(name, source string) error {
	fileset := token.NewFileSet()
	file, err := parser.ParseFile(fileset, name, source, 0)
	if err != nil {
		return err
	}
	goos := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(name), "statfs_"), ".go")
	info := &types.Info{
		Defs:  map[*ast.Ident]types.Object{},
		Uses:  map[*ast.Ident]types.Object{},
		Types: map[ast.Expr]types.TypeAndValue{},
	}
	targetImporter, err := s7APTargetImporter(goos, fileset)
	if err != nil {
		return fmt.Errorf("build %s export importer: %w", goos, err)
	}
	config := types.Config{Importer: targetImporter}
	if _, err := config.Check("s7/intentlock", fileset, []*ast.File{file}, info); err != nil {
		return fmt.Errorf("type-check %s for GOOS=%s: %w", name, goos, err)
	}
	var function *ast.FuncDecl
	for _, declaration := range file.Decls {
		candidate, ok := declaration.(*ast.FuncDecl)
		if ok && candidate.Name.Name == "classifyHeldFilesystem" {
			function = candidate
			break
		}
	}
	if function == nil {
		return errors.New("classifyHeldFilesystem is missing")
	}
	if len(function.Type.Params.List) != 1 ||
		len(function.Type.Params.List[0].Names) != 1 {
		return errors.New("classifyHeldFilesystem does not have one named file parameter")
	}
	fileParameter := function.Type.Params.List[0].Names[0]
	fileObject := info.Defs[fileParameter]
	var statObject types.Object
	var statErrObject types.Object
	var statSpecification *ast.ValueSpec
	var statErrSpecification *ast.ValueSpec
	statDeclarations := 0
	statErrDeclarations := 0
	ast.Inspect(function.Body, func(node ast.Node) bool {
		specification, _ := node.(*ast.ValueSpec)
		if specification == nil {
			return true
		}
		for _, name := range specification.Names {
			if name.Name == "statErr" {
				object := info.Defs[name]
				builtinError := types.Universe.Lookup("error")
				if object == nil || builtinError == nil ||
					!types.Identical(object.Type(), builtinError.Type()) ||
					len(specification.Values) != 0 {
					err = errors.New("held stat error is not the exact zero-valued error local")
					return false
				}
				statErrObject = object
				statErrSpecification = specification
				statErrDeclarations++
				continue
			}
			if name.Name != "stat" {
				continue
			}
			object := info.Defs[name]
			if object == nil {
				err = errors.New("held stat buffer has no typed local object")
				return false
			}
			if len(specification.Values) != 0 {
				err = errors.New("held stat buffer is not the exact zero-valued local")
				return false
			}
			named, _ := object.Type().(*types.Named)
			if named == nil || named.Obj() == nil ||
				named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != "syscall" ||
				named.Obj().Name() != "Statfs_t" {
				err = errors.New("held stat buffer is not the exact syscall.Statfs_t local")
				return false
			}
			statObject = object
			statSpecification = specification
			statDeclarations++
		}
		return err == nil
	})
	if err != nil {
		return err
	}
	if statDeclarations != 1 || statObject == nil {
		return errors.New("held classifier does not declare exactly one local stat buffer")
	}
	if statErrDeclarations != 1 || statErrObject == nil {
		return errors.New("held classifier does not declare exactly one local stat error")
	}
	if s7APTopLevelStatementIndex(function.Body, statSpecification) != 2 ||
		s7APTopLevelStatementIndex(function.Body, statErrSpecification) != 3 {
		return errors.New("held stat and statErr declarations are not in canonical statement positions")
	}
	var rawObject types.Object
	errorDefinitions := map[types.Object]*ast.AssignStmt{}
	errorRoles := map[string]types.Object{}
	var canonicalStatAddress *ast.UnaryExpr
	syscallConn, control, fstatfs := 0, 0, 0
	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, _ := call.Fun.(*ast.SelectorExpr)
		if selector == nil {
			return true
		}
		switch selector.Sel.Name {
		case "SyscallConn":
			syscallConn++
			receiver, _ := selector.X.(*ast.Ident)
			if receiver == nil ||
				info.Uses[receiver] != fileObject {
				err = errors.New("SyscallConn receiver is not the exact file parameter")
				return false
			}
			assignment, _ := s7APParentStatement(function.Body, call).(*ast.AssignStmt)
			if assignment == nil || len(assignment.Lhs) < 1 {
				err = errors.New("SyscallConn result is not bound by assignment")
				return false
			}
			if assignment == nil || assignment.Tok != token.DEFINE ||
				len(assignment.Lhs) != 2 ||
				len(assignment.Rhs) != 1 || assignment.Rhs[0] != call {
				err = errors.New("SyscallConn raw result is not a named object")
				return false
			}
			rawIdentifier, _ := assignment.Lhs[0].(*ast.Ident)
			errorIdentifier, _ := assignment.Lhs[1].(*ast.Ident)
			if rawIdentifier == nil || errorIdentifier == nil {
				err = errors.New("SyscallConn results are not exact named objects")
				return false
			}
			rawObject = info.Defs[rawIdentifier]
			errorObject := info.Defs[errorIdentifier]
			if errorObject == nil {
				err = errors.New("SyscallConn error result is not a typed local")
				return false
			}
			errorDefinitions[errorObject] = assignment
			errorRoles["syscall-conn"] = errorObject
		case "Control":
			control++
			receiver, _ := selector.X.(*ast.Ident)
			if receiver == nil ||
				rawObject == nil || info.Uses[receiver] != rawObject {
				err = errors.New("Control receiver is not the SyscallConn result")
				return false
			}
			if len(call.Args) != 1 {
				err = errors.New("Control does not receive one direct callback")
				return false
			}
			assignment, _ := s7APParentStatement(function.Body, call).(*ast.AssignStmt)
			errorIdentifier, _ := func() (*ast.Ident, bool) {
				if assignment == nil || assignment.Tok != token.DEFINE ||
					len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 ||
					assignment.Rhs[0] != call {
					return nil, false
				}
				result, ok := assignment.Lhs[0].(*ast.Ident)
				return result, ok
			}()
			if errorIdentifier == nil {
				err = errors.New("Control error result is not the exact guarded assignment")
				return false
			}
			errorObject := info.Defs[errorIdentifier]
			if errorObject == nil {
				err = errors.New("Control error result is not a typed local")
				return false
			}
			errorDefinitions[errorObject] = assignment
			errorRoles["control"] = errorObject
			callback, _ := call.Args[0].(*ast.FuncLit)
			if callback == nil || len(callback.Type.Params.List) != 1 ||
				len(callback.Type.Params.List[0].Names) != 1 {
				err = errors.New("Control callback is not a direct descriptor literal")
				return false
			}
			descriptorObject := info.Defs[callback.Type.Params.List[0].Names[0]]
			ast.Inspect(callback.Body, func(callbackNode ast.Node) bool {
				fstatCall, ok := callbackNode.(*ast.CallExpr)
				if !ok {
					return true
				}
				fstatSelector, _ := fstatCall.Fun.(*ast.SelectorExpr)
				if fstatSelector == nil || fstatSelector.Sel.Name != "Fstatfs" {
					return true
				}
				fstatfs++
				packageIdentifier, _ := fstatSelector.X.(*ast.Ident)
				if packageIdentifier == nil || packageIdentifier.Name != "syscall" {
					err = errors.New("Fstatfs is not selected from the syscall package")
					return false
				}
				object, _ := info.Uses[fstatSelector.Sel].(*types.Func)
				if object == nil || object.Pkg() == nil ||
					object.Pkg().Path() != "syscall" || object.Name() != "Fstatfs" {
					err = errors.New("Fstatfs does not resolve to the exact syscall function")
					return false
				}
				if len(fstatCall.Args) != 2 {
					err = errors.New("Fstatfs does not receive descriptor and stat")
					return false
				}
				conversion, _ := fstatCall.Args[0].(*ast.CallExpr)
				if conversion == nil || len(conversion.Args) != 1 {
					err = errors.New("Fstatfs descriptor is not the callback parameter conversion")
					return false
				}
				conversionName, _ := conversion.Fun.(*ast.Ident)
				if conversionName == nil ||
					info.Uses[conversionName] != types.Universe.Lookup("int") {
					err = errors.New("Fstatfs descriptor conversion is not the exact builtin int type")
					return false
				}
				descriptor, _ := conversion.Args[0].(*ast.Ident)
				if descriptor == nil ||
					info.Uses[descriptor] != descriptorObject {
					err = errors.New("Fstatfs does not use the Control callback descriptor")
					return false
				}
				address, _ := fstatCall.Args[1].(*ast.UnaryExpr)
				var statName *ast.Ident
				if address != nil && address.Op == token.AND {
					statName, _ = address.X.(*ast.Ident)
				}
				if statName == nil || info.Uses[statName] != statObject {
					err = errors.New("Fstatfs output is not & of the exact local stat object")
					return false
				}
				canonicalStatAddress = address
				assignment, _ := s7APParentStatement(callback.Body, fstatCall).(*ast.AssignStmt)
				statErrName, _ := func() (*ast.Ident, bool) {
					if assignment == nil || assignment.Tok != token.ASSIGN ||
						len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 ||
						assignment.Rhs[0] != fstatCall {
						return nil, false
					}
					result, ok := assignment.Lhs[0].(*ast.Ident)
					return result, ok
				}()
				if statErrName == nil || info.Uses[statErrName] != statErrObject {
					err = errors.New("Fstatfs result is not assigned to the exact statErr local")
					return false
				}
				errorDefinitions[statErrObject] = assignment
				errorRoles["fstatfs"] = statErrObject
				return false
			})
			if fstatfs != 1 {
				err = errors.New("Control callback does not contain exactly one Fstatfs")
				return false
			}
		}
		return err == nil
	})
	if err != nil {
		return err
	}
	if syscallConn != 1 || control != 1 || fstatfs != 1 {
		return fmt.Errorf("held fstatfs shape = SyscallConn:%d Control:%d Fstatfs:%d",
			syscallConn, control, fstatfs)
	}
	if err := s7APValidateSampledStatProvenance(
		function.Body, info, goos, statObject, canonicalStatAddress,
	); err != nil {
		return err
	}
	return s7APValidateClassifierPassthrough(
		file, function, info, goos, statObject, errorDefinitions, errorRoles,
	)
}

func s7APValidateSampledStatProvenance(
	body *ast.BlockStmt,
	info *types.Info,
	goos string,
	statObject types.Object,
	canonicalAddress *ast.UnaryExpr,
) error {
	if body == nil || statObject == nil || canonicalAddress == nil {
		return errors.New("sampled stat provenance is incomplete")
	}
	var validationErr error
	statSlices := 0
	ast.Inspect(body, func(node ast.Node) bool {
		if validationErr != nil {
			return false
		}
		switch value := node.(type) {
		case *ast.UnaryExpr:
			if value.Op == token.AND &&
				s7APExpressionRootObject(info, value.X) == statObject &&
				value != canonicalAddress {
				validationErr = errors.New("sampled stat address escapes outside canonical Fstatfs")
				return false
			}
		case *ast.AssignStmt:
			for _, left := range value.Lhs {
				if s7APExpressionRootObject(info, left) == statObject {
					validationErr = errors.New("sampled stat is assigned outside canonical Fstatfs")
					return false
				}
			}
		case *ast.IncDecStmt:
			if s7APExpressionRootObject(info, value.X) == statObject {
				validationErr = errors.New("sampled stat field is mutated by increment/decrement")
				return false
			}
		case *ast.RangeStmt:
			if value.Tok != token.ASSIGN {
				return true
			}
			for _, target := range []ast.Expr{value.Key, value.Value} {
				if target != nil &&
					s7APExpressionRootObject(info, target) == statObject {
					validationErr = errors.New("sampled stat is assigned by range iteration")
					return false
				}
			}
		case *ast.TypeSwitchStmt:
			assignment, _ := value.Assign.(*ast.AssignStmt)
			if assignment == nil {
				return true
			}
			for _, target := range assignment.Lhs {
				if s7APExpressionRootObject(info, target) == statObject {
					validationErr = errors.New("sampled stat is assigned by type-switch binding")
					return false
				}
			}
		case *ast.SliceExpr:
			if s7APExpressionRootObject(info, value.X) == statObject {
				statSlices++
			}
		}
		return true
	})
	if validationErr != nil {
		return validationErr
	}
	wantSlices := 0
	if goos == "darwin" {
		wantSlices = 1
	}
	if statSlices != wantSlices {
		return fmt.Errorf("sampled stat slice escapes = %d, want %d for %s",
			statSlices, wantSlices, goos)
	}
	return validationErr
}

func s7APExpressionRootObject(
	info *types.Info,
	expression ast.Expr,
) types.Object {
	switch value := expression.(type) {
	case *ast.Ident:
		return info.ObjectOf(value)
	case *ast.ParenExpr:
		return s7APExpressionRootObject(info, value.X)
	case *ast.SelectorExpr:
		return s7APExpressionRootObject(info, value.X)
	case *ast.IndexExpr:
		return s7APExpressionRootObject(info, value.X)
	case *ast.IndexListExpr:
		return s7APExpressionRootObject(info, value.X)
	case *ast.SliceExpr:
		return s7APExpressionRootObject(info, value.X)
	case *ast.StarExpr:
		return s7APExpressionRootObject(info, value.X)
	case *ast.UnaryExpr:
		if value.Op == token.AND {
			return s7APExpressionRootObject(info, value.X)
		}
	}
	return nil
}

func s7APValidateClassifierPassthrough(
	file *ast.File,
	function *ast.FuncDecl,
	info *types.Info,
	goos string,
	statObject types.Object,
	errorDefinitions map[types.Object]*ast.AssignStmt,
	errorRoles map[string]types.Object,
) error {
	if function == nil || function.Body == nil || len(function.Body.List) < 2 {
		return errors.New("held filesystem classifier has no terminal passthrough")
	}
	assignment, _ := function.Body.List[len(function.Body.List)-2].(*ast.AssignStmt)
	result, _ := function.Body.List[len(function.Body.List)-1].(*ast.ReturnStmt)
	if assignment == nil || assignment.Tok != token.DEFINE ||
		len(assignment.Lhs) != 2 || len(assignment.Rhs) != 1 ||
		result == nil || len(result.Results) != 3 {
		return errors.New("held filesystem classifier does not end in direct class passthrough")
	}
	name, _ := assignment.Lhs[0].(*ast.Ident)
	denied, _ := assignment.Lhs[1].(*ast.Ident)
	call, _ := assignment.Rhs[0].(*ast.CallExpr)
	if call == nil {
		return errors.New("held filesystem classifier result is not a direct call")
	}
	callee, _ := call.Fun.(*ast.Ident)
	want := "classifyLinuxFilesystem"
	if goos == "darwin" {
		want = "classifyDarwinFilesystem"
	}
	functionObject, _ := info.Uses[callee].(*types.Func)
	if name == nil || denied == nil || callee == nil ||
		functionObject == nil || functionObject.Pkg() == nil ||
		functionObject.Pkg().Path() != "s7/intentlock" ||
		functionObject.Name() != want {
		return fmt.Errorf("held filesystem classifier does not call exact %s", want)
	}
	if err := s7APValidateClassifierInput(
		file, call, info, goos, statObject,
	); err != nil {
		return err
	}

	nameObject := info.Defs[name]
	deniedObject := info.Defs[denied]
	returnName, _ := result.Results[0].(*ast.Ident)
	returnDenied, _ := result.Results[1].(*ast.Ident)
	returnNil, _ := result.Results[2].(*ast.Ident)
	if returnName == nil || returnDenied == nil || returnNil == nil ||
		returnNil.Name != "nil" ||
		info.Uses[returnName] != nameObject ||
		info.Uses[returnDenied] != deniedObject {
		return errors.New("held filesystem classifier mutates or replaces class results")
	}
	if len(errorDefinitions) != 3 {
		return fmt.Errorf("canonical error definition count = %d, want 3",
			len(errorDefinitions))
	}
	errorAliases := s7APCanonicalErrorAliases(info, function.Body, errorDefinitions)
	var errorMutationErr error
	ast.Inspect(function.Body, func(node ast.Node) bool {
		if errorMutationErr != nil {
			return false
		}
		address, _ := node.(*ast.UnaryExpr)
		if address == nil || address.Op != token.AND {
			return true
		}
		if object := s7APCanonicalErrorReference(
			info, address.X, errorDefinitions, errorAliases,
		); object != nil {
			errorMutationErr = fmt.Errorf(
				"canonical error object %s has its address taken",
				object.Name(),
			)
			return false
		}
		return true
	})
	if errorMutationErr != nil {
		return errorMutationErr
	}
	errorWrites := map[types.Object][]*ast.AssignStmt{}
	ast.Inspect(function.Body, func(node ast.Node) bool {
		assignment, _ := node.(*ast.AssignStmt)
		if assignment == nil {
			return true
		}
		for _, left := range assignment.Lhs {
			object := s7APCanonicalErrorReference(
				info, left, errorDefinitions, errorAliases,
			)
			if object != nil {
				errorWrites[object] = append(errorWrites[object], assignment)
			}
		}
		return true
	})
	for object, definition := range errorDefinitions {
		writes := errorWrites[object]
		if len(writes) != 1 || writes[0] != definition {
			return fmt.Errorf("error object %s definitions = %d, want exact canonical one",
				object.Name(), len(writes))
		}
	}
	if err := s7APValidateCanonicalErrorGuards(
		function.Body, assignment, info, errorDefinitions, errorRoles,
	); err != nil {
		return err
	}
	innerCalls := 0
	var validationErr error
	ast.Inspect(function.Body, func(node ast.Node) bool {
		if validationErr != nil {
			return false
		}
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		switch value := node.(type) {
		case *ast.CallExpr:
			called := s7APTypesCalledFunction(info, value)
			if called != nil && called.Pkg() != nil &&
				called.Pkg().Path() == "s7/intentlock" && called.Name() == want {
				innerCalls++
				if value != call {
					validationErr = errors.New("inner classifier is called outside the sole passthrough assignment")
					return false
				}
			}
		case *ast.AssignStmt:
			if value == assignment {
				return true
			}
			for _, left := range value.Lhs {
				if s7APAssignedObject(info, left) == nameObject ||
					s7APAssignedObject(info, left) == deniedObject {
					validationErr = errors.New("classifier result is reassigned outside its defining call")
					return false
				}
				if s7APBooleanExpression(info, left) {
					validationErr = errors.New("outer classifier has an additional boolean assignment path")
					return false
				}
			}
		case *ast.ValueSpec:
			for _, local := range value.Names {
				if object := info.Defs[local]; object != nil &&
					s7APBooleanType(object.Type()) {
					validationErr = errors.New("outer classifier declares an additional boolean path")
					return false
				}
			}
		case *ast.ReturnStmt:
			if value == result {
				return true
			}
			if !s7APClassifierErrorReturn(
				function.Body, value, info, errorDefinitions,
			) {
				validationErr = errors.New("outer classifier has a non-error return before its sole passthrough")
				return false
			}
		}
		return true
	})
	if validationErr != nil {
		return validationErr
	}
	if innerCalls != 1 {
		return fmt.Errorf("inner classifier call count = %d, want 1", innerCalls)
	}
	return nil
}

func s7APValidateCanonicalErrorGuards(
	body *ast.BlockStmt,
	classifier *ast.AssignStmt,
	info *types.Info,
	definitions map[types.Object]*ast.AssignStmt,
	roles map[string]types.Object,
) error {
	if body == nil || classifier == nil || len(roles) != 3 {
		return errors.New("canonical error role mapping is incomplete")
	}
	if len(body.List) != 8 {
		return fmt.Errorf("classifyHeldFilesystem statement count = %d, want exact 8",
			len(body.List))
	}
	guards := map[types.Object][]*ast.IfStmt{}
	ast.Inspect(body, func(node ast.Node) bool {
		guard, _ := node.(*ast.IfStmt)
		if guard == nil {
			return true
		}
		if object := s7APCanonicalGuardedError(info, guard, definitions); object != nil {
			guards[object] = append(guards[object], guard)
		}
		return true
	})
	classifierIndex := s7APTopLevelStatementIndex(body, classifier)
	if classifierIndex < 0 {
		return errors.New("classifier assignment has no top-level position")
	}
	order := []string{"syscall-conn", "control", "fstatfs"}
	guardIndexes := map[string]int{}
	for _, role := range order {
		object := roles[role]
		if object == nil || definitions[object] == nil {
			return fmt.Errorf("%s canonical error definition is missing", role)
		}
		matches := guards[object]
		if len(matches) != 1 {
			return fmt.Errorf("%s canonical error guards = %d, want exactly one",
				role, len(matches))
		}
		guard := matches[0]
		if !s7APDefinitionDominatesGuard(body, definitions[object], guard) {
			return fmt.Errorf("%s canonical error guard is missing, delayed, or non-dominating",
				role)
		}
		index := s7APTopLevelStatementIndex(body, guard)
		if index < 0 || index >= classifierIndex {
			return fmt.Errorf("%s canonical error guard does not precede classifier use",
				role)
		}
		guardIndexes[role] = index
	}
	if !(guardIndexes["syscall-conn"] < guardIndexes["control"] &&
		guardIndexes["control"] < guardIndexes["fstatfs"]) {
		return fmt.Errorf("canonical error guard order = syscall-conn:%d control:%d fstatfs:%d",
			guardIndexes["syscall-conn"],
			guardIndexes["control"],
			guardIndexes["fstatfs"],
		)
	}
	if guardIndexes["syscall-conn"] != 1 ||
		guardIndexes["control"] != 4 ||
		guardIndexes["fstatfs"] != 5 ||
		classifierIndex != 6 {
		return fmt.Errorf(
			"canonical statement positions = syscall-conn:%d control:%d fstatfs:%d classifier:%d",
			guardIndexes["syscall-conn"],
			guardIndexes["control"],
			guardIndexes["fstatfs"],
			classifierIndex,
		)
	}
	controlGuard := guards[roles["control"]][0]
	if controlGuard.Init != definitions[roles["control"]] {
		return errors.New("Control error definition is not owned by its exact guard initializer")
	}
	if s7APTopLevelStatementIndex(body, definitions[roles["syscall-conn"]])+1 !=
		guardIndexes["syscall-conn"] {
		return errors.New("SyscallConn error guard is not immediately after its definition")
	}
	fstatDefinition := definitions[roles["fstatfs"]]
	if !s7APNodeContains(controlGuard, fstatDefinition) ||
		guardIndexes["fstatfs"] != guardIndexes["control"]+1 {
		return errors.New("Fstatfs error guard does not immediately follow its owning Control scope")
	}
	if classifierIndex != guardIndexes["fstatfs"]+1 {
		return errors.New("classifier does not immediately consume the guarded sampled stat")
	}
	return nil
}

func s7APCanonicalGuardedError(
	info *types.Info,
	guard *ast.IfStmt,
	definitions map[types.Object]*ast.AssignStmt,
) types.Object {
	if guard == nil || guard.Else != nil || len(guard.Body.List) != 1 {
		return nil
	}
	statement, _ := guard.Body.List[0].(*ast.ReturnStmt)
	if statement == nil || len(statement.Results) != 3 {
		return nil
	}
	first := info.Types[statement.Results[0]].Value
	second := info.Types[statement.Results[1]].Value
	if first == nil || first.Kind() != constant.String ||
		constant.StringVal(first) != "" ||
		second == nil || second.Kind() != constant.Bool ||
		constant.BoolVal(second) {
		return nil
	}
	condition, _ := guard.Cond.(*ast.BinaryExpr)
	if condition == nil || condition.Op != token.NEQ {
		return nil
	}
	guarded, _ := condition.X.(*ast.Ident)
	nilIdentifier, _ := condition.Y.(*ast.Ident)
	returned, _ := statement.Results[2].(*ast.Ident)
	if guarded == nil || returned == nil || nilIdentifier == nil ||
		nilIdentifier.Name != "nil" ||
		info.ObjectOf(nilIdentifier) != types.Universe.Lookup("nil") {
		return nil
	}
	object := info.ObjectOf(guarded)
	if object == nil || info.ObjectOf(returned) != object ||
		definitions[object] == nil {
		return nil
	}
	return object
}

func s7APTopLevelStatementIndex(body *ast.BlockStmt, target ast.Node) int {
	if body == nil || target == nil {
		return -1
	}
	for index, statement := range body.List {
		if s7APNodeContains(statement, target) {
			return index
		}
	}
	return -1
}

func s7APCanonicalErrorReference(
	info *types.Info,
	expression ast.Expr,
	definitions map[types.Object]*ast.AssignStmt,
	aliases map[types.Object]types.Object,
) types.Object {
	switch value := expression.(type) {
	case *ast.Ident:
		object := info.ObjectOf(value)
		if definitions[object] != nil {
			return object
		}
	case *ast.ParenExpr:
		return s7APCanonicalErrorReference(info, value.X, definitions, aliases)
	case *ast.StarExpr:
		return s7APCanonicalErrorPointerTarget(
			info, value.X, definitions, aliases,
		)
	case *ast.UnaryExpr:
		if value.Op == token.AND {
			return s7APCanonicalErrorReference(
				info, value.X, definitions, aliases,
			)
		}
	}
	return nil
}

func s7APCanonicalErrorAliases(
	info *types.Info,
	body *ast.BlockStmt,
	definitions map[types.Object]*ast.AssignStmt,
) map[types.Object]types.Object {
	aliases := map[types.Object]types.Object{}
	changed := true
	for changed {
		changed = false
		ast.Inspect(body, func(node ast.Node) bool {
			var names []*ast.Ident
			var values []ast.Expr
			switch value := node.(type) {
			case *ast.AssignStmt:
				if len(value.Lhs) != len(value.Rhs) {
					return true
				}
				for _, left := range value.Lhs {
					name, _ := left.(*ast.Ident)
					names = append(names, name)
				}
				values = value.Rhs
			case *ast.ValueSpec:
				if len(value.Names) != len(value.Values) {
					return true
				}
				names = value.Names
				values = value.Values
			default:
				return true
			}
			for index, name := range names {
				if name == nil || aliases[info.ObjectOf(name)] != nil {
					continue
				}
				target := s7APCanonicalErrorPointerTarget(
					info, values[index], definitions, aliases,
				)
				if target == nil {
					continue
				}
				object := info.ObjectOf(name)
				if object != nil {
					aliases[object] = target
					changed = true
				}
			}
			return true
		})
	}
	return aliases
}

func s7APCanonicalErrorPointerTarget(
	info *types.Info,
	expression ast.Expr,
	definitions map[types.Object]*ast.AssignStmt,
	aliases map[types.Object]types.Object,
) types.Object {
	switch value := expression.(type) {
	case *ast.Ident:
		return aliases[info.ObjectOf(value)]
	case *ast.ParenExpr:
		return s7APCanonicalErrorPointerTarget(
			info, value.X, definitions, aliases,
		)
	case *ast.UnaryExpr:
		if value.Op == token.AND {
			return s7APCanonicalErrorReference(
				info, value.X, definitions, aliases,
			)
		}
	}
	return nil
}

func s7APValidateClassifierInput(
	file *ast.File,
	call *ast.CallExpr,
	info *types.Info,
	goos string,
	statObject types.Object,
) error {
	if call == nil || len(call.Args) != 1 {
		return errors.New("inner classifier does not receive one exact sampled input")
	}
	transformation, _ := call.Args[0].(*ast.CallExpr)
	if transformation == nil || len(transformation.Args) != 1 {
		return errors.New("classifier input is not the direct approved GOOS transformation")
	}
	called := s7APTypesCalledFunction(info, transformation)
	wantHelper := "normalizeLinuxStatfsType"
	wantHash := ""
	if goos == "darwin" {
		wantHelper = "darwinFilesystemName"
	}
	if called == nil || called.Pkg() == nil ||
		called.Pkg().Path() != "s7/intentlock" || called.Name() != wantHelper {
		return fmt.Errorf("classifier input does not call exact %s", wantHelper)
	}
	helper := s7APSiblingFunction(file, wantHelper)
	if helper == nil {
		return fmt.Errorf("classifier transformation helper %s is missing", wantHelper)
	}
	switch goos {
	case "linux":
		selector, _ := transformation.Args[0].(*ast.SelectorExpr)
		if !s7APExactStatField(selector, info, statObject, "Type") {
			return errors.New("Linux classifier input is not exact stat.Type")
		}
		wantHash = "f4c7788e7bdea3c33512ea5d4274375c3be7512525e33a01807ca944554452d0"
	case "darwin":
		slice, _ := transformation.Args[0].(*ast.SliceExpr)
		var selector *ast.SelectorExpr
		if slice != nil && slice.Low == nil && slice.High == nil && slice.Max == nil {
			selector, _ = slice.X.(*ast.SelectorExpr)
		}
		if !s7APExactStatField(selector, info, statObject, "Fstypename") {
			return errors.New("Darwin classifier input is not exact stat.Fstypename[:]")
		}
		wantHash = "fb052d2f9ecc6bad4733cab4b4ab134ee989b74223d5f5e900e2623e43970fcd"
	default:
		return fmt.Errorf("unsupported classifier validation GOOS %q", goos)
	}
	gotHash := s7APAuthorityFunctionBodyHash(helper)
	if gotHash != wantHash {
		return fmt.Errorf("%s helper body hash = %s, want %s",
			wantHelper, gotHash, wantHash)
	}
	return nil
}

func s7APExactStatField(
	selector *ast.SelectorExpr,
	info *types.Info,
	statObject types.Object,
	fieldName string,
) bool {
	if selector == nil || selector.Sel.Name != fieldName {
		return false
	}
	receiver, _ := selector.X.(*ast.Ident)
	field, _ := info.Uses[selector.Sel].(*types.Var)
	return receiver != nil && info.Uses[receiver] == statObject &&
		field != nil && field.IsField() && field.Name() == fieldName
}

func s7APSiblingFunction(file *ast.File, name string) *ast.FuncDecl {
	if file == nil {
		return nil
	}
	for _, declaration := range file.Decls {
		function, _ := declaration.(*ast.FuncDecl)
		if function != nil && function.Name.Name == name {
			return function
		}
	}
	return nil
}

func s7APAuthorityFunctionBodyHash(function *ast.FuncDecl) string {
	if function == nil || function.Body == nil {
		return ""
	}
	var body bytes.Buffer
	if err := format.Node(&body, token.NewFileSet(), function.Body); err != nil {
		return ""
	}
	sum := sha256.Sum256(body.Bytes())
	return fmt.Sprintf("%x", sum[:])
}

func s7APTypesCalledFunction(info *types.Info, call *ast.CallExpr) *types.Func {
	switch function := call.Fun.(type) {
	case *ast.Ident:
		result, _ := info.Uses[function].(*types.Func)
		return result
	case *ast.SelectorExpr:
		result, _ := info.Uses[function.Sel].(*types.Func)
		return result
	default:
		return nil
	}
}

func s7APAssignedObject(info *types.Info, expression ast.Expr) types.Object {
	switch value := expression.(type) {
	case *ast.Ident:
		return info.ObjectOf(value)
	case *ast.SelectorExpr:
		return info.ObjectOf(value.Sel)
	default:
		return nil
	}
}

func s7APBooleanExpression(info *types.Info, expression ast.Expr) bool {
	return s7APBooleanType(info.TypeOf(expression))
}

func s7APBooleanType(value types.Type) bool {
	basic, _ := value.Underlying().(*types.Basic)
	return basic != nil && basic.Kind() == types.Bool
}

func s7APClassifierErrorReturn(
	body *ast.BlockStmt,
	statement *ast.ReturnStmt,
	info *types.Info,
	errorDefinitions map[types.Object]*ast.AssignStmt,
) bool {
	if statement == nil || len(statement.Results) != 3 {
		return false
	}
	first := info.Types[statement.Results[0]].Value
	second := info.Types[statement.Results[1]].Value
	if first == nil || first.Kind() != constant.String ||
		constant.StringVal(first) != "" ||
		second == nil || second.Kind() != constant.Bool ||
		constant.BoolVal(second) {
		return false
	}
	errorIdentifier, _ := statement.Results[2].(*ast.Ident)
	if errorIdentifier == nil {
		return false
	}
	object := info.ObjectOf(errorIdentifier)
	builtinError := types.Universe.Lookup("error")
	if object == nil || builtinError == nil ||
		!types.Identical(object.Type(), builtinError.Type()) {
		return false
	}
	definition := errorDefinitions[object]
	if definition == nil {
		return false
	}
	guard := s7APDirectReturnGuard(body, statement)
	if guard == nil || guard.Else != nil || len(guard.Body.List) != 1 ||
		guard.Body.List[0] != statement {
		return false
	}
	condition, _ := guard.Cond.(*ast.BinaryExpr)
	if condition == nil || condition.Op != token.NEQ {
		return false
	}
	guardedError, _ := condition.X.(*ast.Ident)
	nilIdentifier, _ := condition.Y.(*ast.Ident)
	return guardedError != nil && info.ObjectOf(guardedError) == object &&
		nilIdentifier != nil && nilIdentifier.Name == "nil" &&
		info.ObjectOf(nilIdentifier) == types.Universe.Lookup("nil") &&
		s7APDefinitionDominatesGuard(body, definition, guard)
}

func s7APDefinitionDominatesGuard(
	body *ast.BlockStmt,
	definition *ast.AssignStmt,
	guard *ast.IfStmt,
) bool {
	if body == nil || definition == nil || guard == nil {
		return false
	}
	if guard.Init == definition {
		return true
	}
	definitionIndex := -1
	guardIndex := -1
	for index, statement := range body.List {
		if s7APNodeContains(statement, definition) {
			definitionIndex = index
		}
		if s7APNodeContains(statement, guard) {
			guardIndex = index
		}
	}
	return definitionIndex >= 0 && guardIndex == definitionIndex+1
}

func s7APNodeContains(root, target ast.Node) bool {
	found := false
	ast.Inspect(root, func(node ast.Node) bool {
		if node == target {
			found = true
			return false
		}
		return !found
	})
	return found
}

func s7APDirectReturnGuard(
	root ast.Node,
	target *ast.ReturnStmt,
) *ast.IfStmt {
	var result *ast.IfStmt
	var stack []ast.Node
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if node == target {
			for index := len(stack) - 1; index >= 0; index-- {
				guard, _ := stack[index].(*ast.IfStmt)
				if guard != nil {
					result = guard
					return false
				}
			}
		}
		stack = append(stack, node)
		return result == nil
	})
	return result
}

func validateS7APAcquireClassifierGate(source string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "acquire_supported.go", source, 0)
	if err != nil {
		return err
	}
	var function *ast.FuncDecl
	for _, declaration := range file.Decls {
		candidate, _ := declaration.(*ast.FuncDecl)
		if candidate != nil && candidate.Name.Name == "acquireWithOps" {
			function = candidate
			break
		}
	}
	if function == nil || function.Body == nil {
		return errors.New("acquireWithOps is missing")
	}
	classifierIndex := -1
	var className, deniedName string
	for index, statement := range function.Body.List {
		assignment, _ := statement.(*ast.AssignStmt)
		if assignment == nil || len(assignment.Lhs) != 3 || len(assignment.Rhs) != 1 {
			continue
		}
		call, _ := assignment.Rhs[0].(*ast.CallExpr)
		if call == nil {
			continue
		}
		selector, _ := call.Fun.(*ast.SelectorExpr)
		if selector == nil {
			continue
		}
		receiver, _ := selector.X.(*ast.Ident)
		if receiver == nil ||
			receiver.Name != "ops" || selector.Sel.Name != "classify" ||
			len(call.Args) != 1 {
			continue
		}
		directory, _ := call.Args[0].(*ast.Ident)
		classIdentifier, _ := assignment.Lhs[0].(*ast.Ident)
		deniedIdentifier, _ := assignment.Lhs[1].(*ast.Ident)
		errorIdentifier, _ := assignment.Lhs[2].(*ast.Ident)
		if directory == nil || directory.Name != "directory" ||
			classIdentifier == nil || deniedIdentifier == nil ||
			errorIdentifier == nil || errorIdentifier.Name != "err" {
			return errors.New("classifier result binding is not canonical")
		}
		classifierIndex = index
		className = classIdentifier.Name
		deniedName = deniedIdentifier.Name
		break
	}
	if classifierIndex < 0 || classifierIndex+2 >= len(function.Body.List) {
		return errors.New("classifier gate is missing")
	}
	errorGate, _ := function.Body.List[classifierIndex+1].(*ast.IfStmt)
	deniedGate, _ := function.Body.List[classifierIndex+2].(*ast.IfStmt)
	if !s7APBinaryIdentifierCheck(errorGate, "err") {
		return errors.New("classifier error result is not gated immediately")
	}
	if deniedGate == nil {
		return errors.New("classifier denied result is not gated immediately")
	}
	deniedIdentifier, _ := deniedGate.Cond.(*ast.Ident)
	if deniedIdentifier == nil || deniedIdentifier.Name != deniedName {
		return errors.New("classifier denied result is not gated immediately")
	}
	classUsed := false
	ast.Inspect(deniedGate.Body, func(node ast.Node) bool {
		call, _ := node.(*ast.CallExpr)
		if call == nil {
			return true
		}
		identifier, _ := call.Fun.(*ast.Ident)
		if identifier == nil || identifier.Name != "sanitizeFilesystemClass" ||
			len(call.Args) != 1 {
			return true
		}
		argument, _ := call.Args[0].(*ast.Ident)
		classUsed = argument != nil && argument.Name == className
		return !classUsed
	})
	if !classUsed {
		return errors.New("classifier class result is not used unchanged in refusal")
	}
	lockIndex := -1
	for index := classifierIndex + 3; index < len(function.Body.List); index++ {
		ast.Inspect(function.Body.List[index], func(node ast.Node) bool {
			call, _ := node.(*ast.CallExpr)
			if call == nil {
				return true
			}
			selector, _ := call.Fun.(*ast.SelectorExpr)
			if selector == nil {
				return true
			}
			receiver, _ := selector.X.(*ast.Ident)
			if receiver != nil && receiver.Name == "ops" &&
				selector.Sel.Name == "lock" && len(call.Args) == 1 {
				directory, _ := call.Args[0].(*ast.Ident)
				if directory != nil && directory.Name == "directory" {
					lockIndex = index
					return false
				}
			}
			return true
		})
		if lockIndex >= 0 {
			break
		}
	}
	if lockIndex < 0 {
		return errors.New("flock does not consume the classified held directory")
	}
	return nil
}

func s7APBinaryIdentifierCheck(statement *ast.IfStmt, name string) bool {
	if statement == nil {
		return false
	}
	condition, _ := statement.Cond.(*ast.BinaryExpr)
	if condition == nil {
		return false
	}
	identifier, _ := condition.X.(*ast.Ident)
	nilIdentifier, _ := condition.Y.(*ast.Ident)
	return condition.Op == token.NEQ &&
		identifier != nil && identifier.Name == name &&
		nilIdentifier != nil && nilIdentifier.Name == "nil"
}

var s7APExportMaps sync.Map

func s7APTargetImporter(goos string, fileset *token.FileSet) (types.Importer, error) {
	target := goos + "/" + runtime.GOARCH
	cached, ok := s7APExportMaps.Load(target)
	if !ok {
		root, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		for {
			if _, statErr := os.Stat(filepath.Join(root, "go.mod")); statErr == nil {
				break
			}
			parent := filepath.Dir(root)
			if parent == root {
				return nil, errors.New("module root not found")
			}
			root = parent
		}
		command := exec.Command("go", "list", "-deps", "-export", "-json", "fmt", "os", "strings", "syscall")
		command.Dir = root
		command.Env = append(os.Environ(),
			"GOOS="+goos,
			"GOARCH="+runtime.GOARCH,
			"CGO_ENABLED=0",
			"GOPROXY=off",
			"GOSUMDB=off",
		)
		output, err := command.Output()
		if err != nil {
			return nil, fmt.Errorf("go list GOOS=%s: %w", goos, err)
		}
		exports := map[string]string{}
		decoder := json.NewDecoder(strings.NewReader(string(output)))
		for {
			var item struct {
				ImportPath string
				Export     string
			}
			if err := decoder.Decode(&item); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return nil, err
			}
			if item.ImportPath != "" && item.Export != "" {
				exports[item.ImportPath] = item.Export
			}
		}
		cached = exports
		s7APExportMaps.Store(target, cached)
	}
	exports := cached.(map[string]string)
	lookup := func(path string) (io.ReadCloser, error) {
		exportPath := exports[path]
		if exportPath == "" {
			return nil, fmt.Errorf("no GOOS=%s export data for %s", goos, path)
		}
		return os.Open(exportPath)
	}
	return importer.ForCompiler(fileset, "gc", lookup), nil
}

func s7APParentStatement(root ast.Node, target ast.Node) ast.Node {
	var result ast.Node
	var stack []ast.Node
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if node == target {
			for index := len(stack) - 1; index >= 0; index-- {
				switch stack[index].(type) {
				case *ast.AssignStmt:
					result = stack[index]
					return false
				}
			}
		}
		stack = append(stack, node)
		return result == nil
	})
	return result
}

func validateS7APFilesystemTables(linux, darwin string) error {
	if err := validateS7APHeldFstatfsSource("statfs_linux.go", linux); err != nil {
		return err
	}
	if err := validateS7APHeldFstatfsSource("statfs_darwin.go", darwin); err != nil {
		return err
	}
	if err := s7APValidateClosedClassifier(
		"statfs_linux.go", linux, "classifyLinuxFilesystem", "fsType",
	); err != nil {
		return err
	}
	if err := s7APValidateClosedClassifier(
		"statfs_darwin.go", darwin, "classifyDarwinFilesystem", "name",
	); err != nil {
		return err
	}
	linuxDenied, linuxValues, err := s7APLinuxDeniedTable(linux)
	if err != nil {
		return err
	}
	darwinDenied, err := s7APDarwinDeniedTable(darwin)
	if err != nil {
		return err
	}
	wantLinuxNames := []string{
		"linuxMagicCIFS", "linuxMagicFUSE", "linuxMagicNFS",
		"linuxMagicSMB", "linuxMagicSMB2",
	}
	wantLinuxValues := map[uint64]bool{
		0x6969: true, 0x517B: true, 0xFF534D42: true,
		0xFE534D42: true, 0x65735546: true,
	}
	wantDarwin := []string{"macfuse", "nfs", "osxfuse", "smbfs", "webdav"}
	sort.Strings(linuxDenied)
	sort.Strings(darwinDenied)
	if !reflect.DeepEqual(linuxDenied, wantLinuxNames) ||
		!reflect.DeepEqual(linuxValues, wantLinuxValues) ||
		!reflect.DeepEqual(darwinDenied, wantDarwin) ||
		containsS7APString(darwinDenied, "overlay") ||
		containsS7APString(darwinDenied, "apfs") ||
		containsS7APString(darwinDenied, "") {
		return fmt.Errorf("filesystem tables = linux:%v darwin:%v values:%v",
			linuxDenied, darwinDenied, linuxValues)
	}
	return nil
}

func s7APValidateClosedClassifier(fileName, source, functionName, parameterName string) error {
	file, err := parser.ParseFile(token.NewFileSet(), fileName, source, 0)
	if err != nil {
		return err
	}
	var function *ast.FuncDecl
	for _, declaration := range file.Decls {
		candidate, _ := declaration.(*ast.FuncDecl)
		if candidate != nil && candidate.Name.Name == functionName {
			if function != nil {
				return fmt.Errorf("%s declares %s more than once", fileName, functionName)
			}
			function = candidate
		}
	}
	if function == nil || function.Body == nil || len(function.Body.List) != 1 {
		return fmt.Errorf("%s must contain only its closed switch", functionName)
	}
	statement, _ := function.Body.List[0].(*ast.SwitchStmt)
	if statement == nil {
		return fmt.Errorf("%s does not contain a switch", functionName)
	}
	tag, _ := statement.Tag.(*ast.Ident)
	if tag == nil || tag.Name != parameterName {
		return fmt.Errorf("%s does not switch directly on %s", functionName, parameterName)
	}
	defaults := 0
	denied := 0
	for _, rawClause := range statement.Body.List {
		clause, _ := rawClause.(*ast.CaseClause)
		if clause == nil || len(clause.Body) != 1 {
			return fmt.Errorf("%s has a noncanonical switch clause", functionName)
		}
		returnStatement, _ := clause.Body[0].(*ast.ReturnStmt)
		if returnStatement == nil || len(returnStatement.Results) != 2 {
			return fmt.Errorf("%s has a noncanonical switch return", functionName)
		}
		boolean, _ := returnStatement.Results[1].(*ast.Ident)
		if len(clause.List) == 0 {
			defaults++
			if boolean == nil || boolean.Name != "false" {
				return fmt.Errorf("%s default is not the sole allow route", functionName)
			}
			continue
		}
		if boolean == nil || boolean.Name != "true" {
			return fmt.Errorf("%s denial case does not return true", functionName)
		}
		denied += len(clause.List)
	}
	if defaults != 1 || denied != 5 {
		return fmt.Errorf("%s closed switch = denied:%d defaults:%d", functionName, denied, defaults)
	}
	return nil
}

func s7APLinuxDeniedTable(source string) ([]string, map[uint64]bool, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "statfs_linux.go", source, 0)
	if err != nil {
		return nil, nil, err
	}
	constants := map[string]ast.Expr{}
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.CONST {
			continue
		}
		for _, spec := range generic.Specs {
			value, _ := spec.(*ast.ValueSpec)
			if value == nil || len(value.Names) != 1 || len(value.Values) != 1 {
				continue
			}
			constants[value.Names[0].Name] = value.Values[0]
		}
	}
	var denied []string
	numeric := map[uint64]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		clause, ok := node.(*ast.CaseClause)
		if !ok || len(clause.Body) != 1 {
			return true
		}
		returnStatement, _ := clause.Body[0].(*ast.ReturnStmt)
		if returnStatement == nil || len(returnStatement.Results) != 2 {
			return true
		}
		boolean, _ := returnStatement.Results[1].(*ast.Ident)
		if boolean == nil || boolean.Name != "true" {
			return true
		}
		for _, expression := range clause.List {
			rendered := s7APConstantExpressionName(expression)
			denied = append(denied, rendered)
			number, ok := s7APUnsignedConstant(expression, constants, map[string]bool{})
			if !ok {
				err = fmt.Errorf("denied case %s is not a resolved unsigned constant", rendered)
				return false
			}
			numeric[number] = true
		}
		return true
	})
	if err != nil {
		return nil, nil, err
	}
	return denied, numeric, nil
}

func s7APConstantExpressionName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.BasicLit:
		return value.Value
	case *ast.UnaryExpr:
		return value.Op.String() + s7APConstantExpressionName(value.X)
	case *ast.ParenExpr:
		return "(" + s7APConstantExpressionName(value.X) + ")"
	default:
		return fmt.Sprintf("%T", expression)
	}
}

func s7APUnsignedConstant(
	expression ast.Expr,
	constants map[string]ast.Expr,
	visiting map[string]bool,
) (uint64, bool) {
	switch value := expression.(type) {
	case *ast.BasicLit:
		number := constant.MakeFromLiteral(value.Value, value.Kind, 0)
		result, ok := constant.Uint64Val(number)
		return result, ok
	case *ast.Ident:
		if visiting[value.Name] {
			return 0, false
		}
		target := constants[value.Name]
		if target == nil {
			return 0, false
		}
		visiting[value.Name] = true
		defer delete(visiting, value.Name)
		return s7APUnsignedConstant(target, constants, visiting)
	case *ast.UnaryExpr:
		operand, ok := s7APUnsignedConstantValue(value.X, constants, visiting)
		if !ok {
			return 0, false
		}
		result := constant.UnaryOp(value.Op, operand, 0)
		number, ok := constant.Uint64Val(result)
		return number, ok
	case *ast.ParenExpr:
		return s7APUnsignedConstant(value.X, constants, visiting)
	default:
		return 0, false
	}
}

func s7APUnsignedConstantValue(
	expression ast.Expr,
	constants map[string]ast.Expr,
	visiting map[string]bool,
) (constant.Value, bool) {
	switch value := expression.(type) {
	case *ast.BasicLit:
		result := constant.MakeFromLiteral(value.Value, value.Kind, 0)
		return result, result.Kind() != constant.Unknown
	case *ast.Ident:
		if visiting[value.Name] || constants[value.Name] == nil {
			return nil, false
		}
		visiting[value.Name] = true
		defer delete(visiting, value.Name)
		return s7APUnsignedConstantValue(constants[value.Name], constants, visiting)
	case *ast.UnaryExpr:
		operand, ok := s7APUnsignedConstantValue(value.X, constants, visiting)
		if !ok {
			return nil, false
		}
		result := constant.UnaryOp(value.Op, operand, 0)
		return result, result.Kind() != constant.Unknown
	case *ast.ParenExpr:
		return s7APUnsignedConstantValue(value.X, constants, visiting)
	default:
		return nil, false
	}
}

func s7APDarwinDeniedTable(source string) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "statfs_darwin.go", source, 0)
	if err != nil {
		return nil, err
	}
	constants := map[string]ast.Expr{}
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.CONST {
			continue
		}
		for _, spec := range generic.Specs {
			value, _ := spec.(*ast.ValueSpec)
			if value != nil && len(value.Names) == 1 && len(value.Values) == 1 {
				constants[value.Names[0].Name] = value.Values[0]
			}
		}
	}
	var denied []string
	ast.Inspect(file, func(node ast.Node) bool {
		clause, ok := node.(*ast.CaseClause)
		if !ok || len(clause.Body) != 1 {
			return true
		}
		returnStatement, _ := clause.Body[0].(*ast.ReturnStmt)
		if returnStatement == nil || len(returnStatement.Results) != 2 {
			return true
		}
		boolean, _ := returnStatement.Results[1].(*ast.Ident)
		if boolean == nil || boolean.Name != "true" {
			return true
		}
		for _, expression := range clause.List {
			value, ok := s7APStringConstant(expression, constants, map[string]bool{})
			if !ok {
				err = fmt.Errorf("Darwin denied case %s is not a resolved string constant",
					s7APConstantExpressionName(expression))
				return false
			}
			denied = append(denied, value)
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return denied, nil
}

func s7APStringConstant(
	expression ast.Expr,
	constants map[string]ast.Expr,
	visiting map[string]bool,
) (string, bool) {
	switch value := expression.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return "", false
		}
		text, err := strconv.Unquote(value.Value)
		return text, err == nil
	case *ast.Ident:
		if visiting[value.Name] || constants[value.Name] == nil {
			return "", false
		}
		visiting[value.Name] = true
		defer delete(visiting, value.Name)
		return s7APStringConstant(constants[value.Name], constants, visiting)
	case *ast.ParenExpr:
		return s7APStringConstant(value.X, constants, visiting)
	case *ast.UnaryExpr:
		operand, ok := s7APStringConstantValue(value.X, constants, visiting)
		if !ok {
			return "", false
		}
		result := constant.UnaryOp(value.Op, operand, 0)
		if result.Kind() != constant.String {
			return "", false
		}
		return constant.StringVal(result), true
	default:
		return "", false
	}
}

func s7APStringConstantValue(
	expression ast.Expr,
	constants map[string]ast.Expr,
	visiting map[string]bool,
) (constant.Value, bool) {
	text, ok := s7APStringConstant(expression, constants, visiting)
	if !ok {
		return nil, false
	}
	return constant.MakeString(text), true
}

func containsS7APString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
