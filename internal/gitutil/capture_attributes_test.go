package gitutil

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestCaptureConversionSentinel(t *testing.T) {
	if os.Getenv("TPATCH_CAPTURE_CONVERSION_CHILD") != "1" {
		return
	}
	if err := os.WriteFile(os.Getenv("TPATCH_CAPTURE_CONVERSION_MARKER"), []byte("conversion executed\n"), 0600); err != nil {
		os.Exit(87)
	}
	os.Exit(86)
}

func captureGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}

func captureConversionFixture(t *testing.T) (root, marker, config string) {
	t.Helper()
	config = filepath.Join(t.TempDir(), "hosted.gitconfig")
	if err := os.WriteFile(config, nil, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", config)
	root = t.TempDir()
	gitInit(t, root)
	marker = filepath.Join(t.TempDir(), "conversion-ran")
	t.Setenv("TPATCH_CAPTURE_CONVERSION_CHILD", "1")
	t.Setenv("TPATCH_CAPTURE_CONVERSION_MARKER", marker)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := "'" + strings.ReplaceAll(filepath.ToSlash(binary), "'", "'\\''") + "' -test.run=^TestCaptureConversionSentinel$"
	for key, value := range map[string]string{
		"filter.lfs.clean": command, "filter.lfs.smudge": command,
		"filter.lfs.process": command, "filter.lfs.required": "true",
		"diff.document.textconv": command, "diff.document.command": command,
	} {
		captureGit(t, root, "config", "--file", config, key, value)
	}
	return root, marker, config
}

func captureWrite(t *testing.T, root, path, body string) {
	t.Helper()
	path = filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func captureTreeState(t *testing.T, root string) string {
	t.Helper()
	var rows []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		var body []byte
		if !entry.IsDir() {
			body, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}
		rows = append(rows, fmt.Sprintf("%q:%s:%d:%x", rel, info.Mode(), info.ModTime().UnixNano(), sha256.Sum256(body)))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(rows)
	return strings.Join(rows, "\n")
}

func captureAssertReadonly(t *testing.T, root, marker, before string) {
	t.Helper()
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("a conversion command executed or its sentinel is unreadable: %v", err)
	}
	if after := captureTreeState(t, root); after != before {
		t.Fatal("readonly applicability/capture changed files, index, objects or metadata")
	}
}

func TestRGAS5CaptureConversionSentinelIsSensitive(t *testing.T) {
	root, marker, _ := captureConversionFixture(t)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "-test.run=^TestCaptureConversionSentinel$")
	cmd.Dir = root
	err = cmd.Run()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 86 {
		t.Fatalf("conversion sentinel did not fail when executed: %v", err)
	}
	body, err := os.ReadFile(marker)
	if err != nil || string(body) != "conversion executed\n" {
		t.Fatalf("conversion sentinel did not record execution: %q %v", body, err)
	}
}

func TestRGAS5CaptureInstalledUnusedConversionsStayUsable(t *testing.T) {
	root, marker, config := captureConversionFixture(t)
	captureWrite(t, root, "hello.txt", "edited\n")
	captureWrite(t, root, "new file.txt", "new\r\n")
	captureWrite(t, root, "literal[1].txt", "literal\n")
	captureWrite(t, root, "literal1.txt", "glob alternative\n")
	if runtime.GOOS != "windows" {
		captureWrite(t, root, "new\nline.txt", "newline path\n")
	}
	defined := captureGit(t, root, "config", "--file", config, "--get-regexp", "^filter\\.lfs\\.")
	if strings.Count(defined, "filter.lfs.") != 4 {
		t.Fatal("fixture lost the installed hosted-style LFS configuration")
	}
	for _, paths := range [][]string{nil, {"hello.txt"}, {"new file.txt"}, {":(literal)literal[1].txt"}, {"literal*.txt"}} {
		before := captureTreeState(t, root)
		got, err := CapturePatchScopedReadOnly(root, paths)
		if err != nil {
			t.Fatalf("unused filter/diff definitions refused for %v: %v", paths, err)
		}
		captureAssertReadonly(t, root, marker, before)
		want, err := CapturePatchScoped(root, paths)
		if err != nil || got != want {
			t.Fatalf("capture no longer matches the actual producer for %v: %v", paths, err)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatal("the supposedly inapplicable converter actually ran")
		}
	}
}

func TestRGAS5CaptureApplicableConversionsRefuseBeforeExecution(t *testing.T) {
	for _, tracked := range []bool{false, true} {
		for _, source := range []string{"worktree", "info", "global"} {
			for _, attribute := range []string{"filter=lfs", "diff=document"} {
				t.Run(fmt.Sprintf("tracked=%v/%s/%s", tracked, source, attribute), func(t *testing.T) {
					root, marker, config := captureConversionFixture(t)
					path := "selected file.txt"
					if tracked {
						captureWrite(t, root, path, "before\n")
						captureGit(t, root, "add", "--", path)
						captureGit(t, root, "commit", "-qm", "tracked candidate")
					}
					captureWrite(t, root, path, "modified content\n")
					rule := "\"selected file.txt\" " + attribute + "\n"
					switch source {
					case "worktree":
						captureWrite(t, root, ".gitattributes", rule)
					case "info":
						captureWrite(t, root, ".git/info/attributes", rule)
					case "global":
						attributes := filepath.Join(t.TempDir(), "attributes")
						if err := os.WriteFile(attributes, []byte(rule), 0644); err != nil {
							t.Fatal(err)
						}
						captureGit(t, root, "config", "--file", config, "core.attributesfile", attributes)
					}
					before := captureTreeState(t, root)
					patch, err := CapturePatchScopedReadOnly(root, []string{path})
					if err == nil || patch != "" || !strings.Contains(err.Error(), "readonly capture") ||
						!strings.Contains(err.Error(), path) {
						t.Fatalf("applicable conversion was not refused at the path guard: patch=%q err=%v", patch, err)
					}
					captureAssertReadonly(t, root, marker, before)
				})
			}
		}
	}
}

func TestRGAS5CaptureConversionScopeAndUnsetAttributes(t *testing.T) {
	for _, tracked := range []bool{false, true} {
		t.Run(fmt.Sprintf("tracked=%v", tracked), func(t *testing.T) {
			root, marker, _ := captureConversionFixture(t)
			captureWrite(t, root, "outside.dat", "outside\n")
			if tracked {
				captureGit(t, root, "add", "--", "outside.dat")
				captureGit(t, root, "commit", "-qm", "outside candidate")
			}
			captureWrite(t, root, "outside.dat", "changed outside\n")
			captureWrite(t, root, "hello.txt", "changed hello\n")
			captureWrite(t, root, "new.txt", "new\n")
			captureWrite(t, root, ".git/info/attributes", "outside.dat filter=lfs diff=document\nhello.txt -filter -diff\nnew.txt -filter\n")
			for _, paths := range [][]string{{"hello.txt"}, {"new.txt"}, {".", ":(exclude)outside.dat"}} {
				before := captureTreeState(t, root)
				patch, err := CapturePatchScopedReadOnly(root, paths)
				if err != nil || patch == "" || strings.Contains(patch, "outside.dat") {
					t.Fatalf("inactive/out-of-scope conversion affected %v: %v", paths, err)
				}
				captureAssertReadonly(t, root, marker, before)
			}
			before := captureTreeState(t, root)
			if _, err := CapturePatchScopedReadOnly(root, []string{"outside.dat"}); err == nil {
				t.Fatal("the same configured filter was not refused when selected")
			}
			captureAssertReadonly(t, root, marker, before)
		})
	}
}

func TestRGAS5CaptureConversionExclusionsRemainEffective(t *testing.T) {
	root, marker, _ := captureConversionFixture(t)
	captureWrite(t, root, ".tpatch/tracked.dat", "before\n")
	captureGit(t, root, "add", "--", ".tpatch/tracked.dat")
	captureGit(t, root, "commit", "-qm", "excluded metadata")
	captureGit(t, root, "worktree", "add", "--detach", "nested", "HEAD")
	captureWrite(t, root, "hello.txt", "changed hello\n")
	captureWrite(t, root, ".tpatch/tracked.dat", "changed metadata\n")
	captureWrite(t, root, ".tpatch/new.dat", "new metadata\n")
	captureWrite(t, root, ".github/skills/new.dat", "generated skill\n")
	captureWrite(t, root, "nested/new.dat", "nested work\n")
	captureWrite(t, root, ".git/info/attributes",
		".tpatch/** filter=lfs\n.github/skills/** filter=lfs\nnested/** filter=lfs\nnested filter=lfs\n")
	before := captureTreeState(t, root)
	patch, err := CapturePatchScopedReadOnly(root, []string{"."})
	if err != nil || !strings.Contains(patch, "hello.txt") {
		t.Fatalf("excluded conversions prevented ordinary capture: %v", err)
	}
	for _, excluded := range []string{".tpatch/", ".github/skills/", "nested/"} {
		if strings.Contains(patch, excluded) {
			t.Fatalf("excluded path leaked into readonly capture: %s", excluded)
		}
	}
	captureAssertReadonly(t, root, marker, before)
}

func TestRGAS5CaptureAttributeMetadataAndSafetyControls(t *testing.T) {
	config, err := parseCaptureConversionConfig("filter.lfs.clean\nunused command\x00diff.document.textconv\nunused converter\x00")
	if err != nil {
		t.Fatal(err)
	}
	paths, err := captureAttributeCandidates("with\nnewline\x00literal[1].txt\x00", []string{"literal[1].txt", "new.txt"})
	if err != nil || len(paths) != 3 {
		t.Fatalf("NUL-safe candidate enumeration: %v %v", paths, err)
	}
	attributes, err := parseCaptureAttributes("literal[1].txt\x00filter\x00unset\x00", paths)
	if err != nil || validateCaptureAttributes(attributes, config, []string{"literal[1].txt"}) != nil {
		t.Fatalf("explicitly unset filter was not accepted: %v", err)
	}
	for _, attributes := range [][]captureAttribute{
		{{"new.txt", "filter", "lfs"}},
		{{"new.txt", "filter", "set"}},
		{{"new.txt", "filter", "unspecified"}},
		{{"new.txt", "diff", "document"}},
		{{"new.txt", "text", "set"}},
	} {
		if validateCaptureAttributes(attributes, config, []string{"new.txt"}) == nil {
			t.Fatalf("the same applicability validator accepted an unsafe input: %+v", attributes)
		}
	}
	config.filters["unset"] = true
	if validateCaptureAttributes([]captureAttribute{{"new.txt", "filter", "unset"}}, config, nil) == nil {
		t.Fatal("ambiguous literal unset driver gained conversion authority")
	}
	for _, raw := range []string{
		"new.txt\x00filter\x00lfs",
		"new.txt\x00filter\x00",
		"outside.txt\x00filter\x00lfs\x00",
		"new.txt\x00filter\x00lfs\x00new.txt\x00filter\x00unset\x00",
	} {
		if _, err := parseCaptureAttributes(raw, paths); err == nil {
			t.Fatalf("attribute parser accepted malformed or mismatched metadata: %q", raw)
		}
	}
	for _, raw := range []string{
		"diff.external\ncommand\x00", "core.autocrlf\ntrue\x00", "diff.renames\ncopies\x00",
		"filter.lfs.clean\nunterminated", "\nempty key\x00", "filter.\ncommand\x00",
	} {
		if _, err := parseCaptureConversionConfig(raw); err == nil {
			t.Fatalf("configuration validator accepted unsafe or malformed input: %q", raw)
		}
	}
	for _, raw := range []string{"core.autocrlf\nfalse\x00", "diff.renames\ntrue\x00", "filter.lfs.required\x00"} {
		if _, err := parseCaptureConversionConfig(raw); err != nil {
			t.Fatalf("safe existing global configuration refused: %v", err)
		}
	}
	if _, err := captureAttributeCandidates("unterminated", nil); err == nil {
		t.Fatal("candidate parser accepted a partial pathname")
	}
}
