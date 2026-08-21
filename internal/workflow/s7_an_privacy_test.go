package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/provider"
)

type s7PIB419Provider struct {
	responses []string
	errs      []error
	calls     int
}

func (*s7PIB419Provider) Check(context.Context, provider.Config) (*provider.Health, error) {
	return &provider.Health{}, nil
}

func (p *s7PIB419Provider) Generate(
	context.Context,
	provider.Config,
	provider.GenerateRequest,
) (string, error) {
	index := p.calls
	p.calls++
	if index < len(p.errs) && p.errs[index] != nil {
		return "", p.errs[index]
	}
	if index >= len(p.responses) {
		return "", errors.New("unexpected provider call")
	}
	return p.responses[index], nil
}

func TestS7PIB419ProviderSuccessFailureRetryHaveNoRawSink(t *testing.T) {
	t.Run("retry-then-success", func(t *testing.T) {
		for _, root := range []string{"analysis", "spec", "exploration"} {
			observation := s7RunPIB419Subprocess(t, root, "retry")
			if err := s7ValidatePIB419Observation(observation, true); err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Run("provider-failure", func(t *testing.T) {
		for _, root := range []string{"analysis", "spec", "exploration"} {
			observation := s7RunPIB419Subprocess(t, root, "failure")
			if err := s7ValidatePIB419Observation(observation, false); err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Run("success-allows-final-provider-content", func(t *testing.T) {
		for _, root := range []string{"analysis", "spec", "exploration"} {
			observation := s7RunPIB419Subprocess(t, root, "success")
			if err := s7ValidatePIB419Observation(observation, true); err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Run("temp-root-write-bites-observation", func(t *testing.T) {
		observation := s7RunPIB419Subprocess(t, "analysis", "temp-leak")
		if err := s7ValidatePIB419Observation(observation, true); err == nil ||
			!strings.Contains(strings.Join(observation.paths, "\n"), "temp/provider-response-") {
			t.Fatalf("same PIB-419 observation accepted an out-of-cwd temp leak: paths=%v err=%v", observation.paths, err)
		}
	})
}

func TestS7PIB419PrivacyHelperProcess(t *testing.T) {
	scenario := os.Getenv("TPATCH_S7_PIB419_SCENARIO")
	root := os.Getenv("TPATCH_S7_PIB419_ROOT")
	if scenario == "" {
		return
	}
	const (
		retryMarker   = "S7-RAW-RETRY-MARKER"
		failureMarker = "S7-RAW-FAILURE-MARKER"
		successMarker = "S7-CANONICAL-SUCCESS-MARKER"
	)
	valid := successMarker
	if root == "analysis" {
		valid = `{"summary":"` + successMarker + `","compatibility":{"status":"unclear","reasoning":"final"},"affected_areas":[],"acceptance_criteria":[],"implementation_notes":[],"unresolved_questions":[]}`
	}
	fixture := &s7PIB419Provider{}
	maxRetries := 0
	switch scenario {
	case "retry":
		fixture.responses = []string{valid}
		if root == "analysis" {
			fixture.responses = []string{retryMarker, valid}
		} else {
			fixture.responses = []string{"", valid}
		}
		maxRetries = 1
	case "failure":
		fixture.responses = []string{""}
		fixture.errs = []error{errors.New(failureMarker)}
	case "success", "temp-leak":
		fixture.responses = []string{valid}
	default:
		t.Fatalf("unknown PIB-419 helper scenario %q", scenario)
	}
	config := provider.Config{Type: "fixture", BaseURL: "https://provider.invalid", Model: "s7"}
	authority := GeneratorAuthorityPolicy{Authority: GeneratorAuthorityRegenerate}
	var (
		final []byte
		note  GenNote
		err   error
	)
	switch root {
	case "analysis":
		var result AnalysisResult
		result, note, err = GenerateAnalysis(context.Background(), AnalysisInput{
			Slug: "s7", Request: "privacy", FileTree: "internal/", Guidance: "none",
			Provider: fixture, Config: config, MaxRetries: maxRetries, Authority: authority,
		})
		if err == nil {
			final, err = json.Marshal(result)
		}
	case "spec":
		var result string
		result, note, err = GenerateSpec(context.Background(), DefineInput{
			Slug: "s7", Request: "privacy", AnalysisMD: "analysis", AnalysisJSON: "{}",
			Provider: fixture, Config: config, MaxRetries: maxRetries, Authority: authority,
		})
		final = []byte(result)
	case "exploration":
		var result string
		result, note, err = GenerateExploration(context.Background(), ExploreInput{
			Slug: "s7", Request: "privacy", AnalysisMD: "analysis", SpecMD: "spec", FileTree: "internal/",
			Provider: fixture, Config: config, MaxRetries: maxRetries, Authority: authority,
		})
		final = []byte(result)
	default:
		t.Fatalf("unknown PIB-419 helper root %q", root)
	}
	if scenario == "failure" {
		if err == nil || fixture.calls != 1 || note.Generator != GeneratorNone ||
			strings.Contains(fmt.Sprint(err, string(final), note), failureMarker) {
			t.Fatalf("%s failure result=%q note=%+v calls=%d err=%v", root, final, note, fixture.calls, err)
		}
		return
	}
	wantCalls := 1
	if scenario == "retry" {
		wantCalls = 2
	}
	if err != nil || fixture.calls != wantCalls || note.Generator != GeneratorProvider ||
		!bytes.Contains(final, []byte(successMarker)) {
		t.Fatalf("%s/%s result=%q note=%+v calls=%d err=%v", root, scenario, final, note, fixture.calls, err)
	}
	if err := os.WriteFile("allowed-canonical-stage.out", final, 0o600); err != nil {
		t.Fatal(err)
	}
	if scenario == "temp-leak" {
		file, err := os.CreateTemp("", "provider-response-*")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.WriteString(successMarker); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

type s7PIB419Observation struct {
	root, scenario string
	stdout, stderr string
	paths          []string
	files          map[string][]byte
}

func s7RunPIB419Subprocess(t *testing.T, root, scenario string) s7PIB419Observation {
	t.Helper()
	sandbox := t.TempDir()
	cwd := filepath.Join(sandbox, "work")
	tempRoot := filepath.Join(sandbox, "temp")
	homeRoot := filepath.Join(sandbox, "home")
	for _, directory := range []string{cwd, tempRoot, homeRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	before := s7PIB419SandboxSnapshot(t, sandbox)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(
		executable,
		"-test.run=^TestS7PIB419PrivacyHelperProcess$",
		"-test.count=1",
	)
	command.Dir = cwd
	command.Env = append(
		os.Environ(),
		"TPATCH_S7_PIB419_SCENARIO="+scenario,
		"TPATCH_S7_PIB419_ROOT="+root,
		"TMPDIR="+tempRoot,
		"TMP="+tempRoot,
		"TEMP="+tempRoot,
		"HOME="+homeRoot,
		"USERPROFILE="+homeRoot,
		"XDG_CACHE_HOME="+filepath.Join(homeRoot, "cache"),
		"XDG_CONFIG_HOME="+filepath.Join(homeRoot, "config"),
		"XDG_DATA_HOME="+filepath.Join(homeRoot, "data"),
	)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err = command.Run()
	if err != nil {
		t.Fatalf("PIB-419 %s/%s helper: %v\nstdout:%s\nstderr:%s", root, scenario, err, stdout.String(), stderr.String())
	}
	after := s7PIB419SandboxSnapshot(t, sandbox)
	files := map[string][]byte{}
	var paths []string
	for name, data := range after {
		old, existed := before[name]
		if existed && bytes.Equal(old, data) {
			continue
		}
		paths = append(paths, name)
		if !strings.HasSuffix(name, "/") {
			files[name] = data
		}
	}
	for name := range before {
		if _, exists := after[name]; !exists {
			paths = append(paths, "!"+name)
		}
	}
	sort.Strings(paths)
	return s7PIB419Observation{
		root: root, scenario: scenario,
		stdout: stdout.String(), stderr: stderr.String(),
		paths: paths, files: files,
	}
}

func s7PIB419SandboxSnapshot(t *testing.T, sandbox string) map[string][]byte {
	t.Helper()
	snapshot := map[string][]byte{}
	err := filepath.WalkDir(sandbox, func(name string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if name != sandbox {
				rel, err := filepath.Rel(sandbox, name)
				if err != nil {
					return err
				}
				snapshot[filepath.ToSlash(rel)+"/"] = nil
			}
			return nil
		}
		rel, err := filepath.Rel(sandbox, name)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(rel)] = data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func s7ValidatePIB419Observation(observation s7PIB419Observation, wantCanonical bool) error {
	const (
		retryMarker   = "S7-RAW-RETRY-MARKER"
		failureMarker = "S7-RAW-FAILURE-MARKER"
		successMarker = "S7-CANONICAL-SUCCESS-MARKER"
	)
	output := observation.stdout + observation.stderr
	if strings.Contains(output, retryMarker) ||
		strings.Contains(output, failureMarker) ||
		strings.Contains(output, successMarker) {
		return fmt.Errorf("%s/%s helper output leaked provider content: stdout=%q stderr=%q",
			observation.root, observation.scenario, observation.stdout, observation.stderr)
	}
	if !wantCanonical {
		if len(observation.paths) != 0 {
			return fmt.Errorf("%s/%s created paths: %v", observation.root, observation.scenario, observation.paths)
		}
		return nil
	}
	if len(observation.paths) != 1 || observation.paths[0] != "work/allowed-canonical-stage.out" ||
		observation.files["work/allowed-canonical-stage.out"] == nil {
		return fmt.Errorf("%s/%s created non-canonical paths: %v", observation.root, observation.scenario, observation.paths)
	}
	for name, data := range observation.files {
		if strings.Contains(name, retryMarker) || strings.Contains(name, failureMarker) ||
			strings.Contains(string(data), retryMarker) || strings.Contains(string(data), failureMarker) {
			return fmt.Errorf("%s raw provider marker persisted at %s", observation.scenario, name)
		}
		if name != "work/allowed-canonical-stage.out" && strings.Contains(string(data), successMarker) {
			return fmt.Errorf("%s/%s final provider marker escaped canonical stage to %s", observation.root, observation.scenario, name)
		}
	}
	if !strings.Contains(string(observation.files["work/allowed-canonical-stage.out"]), successMarker) {
		return fmt.Errorf("%s/%s canonical stage lost validated provider content", observation.root, observation.scenario)
	}
	return nil
}

func sortedS7PrivacyPaths(files map[string][]byte) []string {
	paths := make([]string, 0, len(files))
	for name := range files {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	return paths
}

func TestS7PIB420PureGeneratorRetryConstructionGuardAndWrongInput(t *testing.T) {
	sources := s7WorkflowProductionSources(t)
	if err := validateS7PureGeneratorPackage(sources); err != nil {
		t.Fatal(err)
	}
	wrong := s7CloneWorkflowSources(sources)
	wrong["privacy_wrong.go"] = `package workflow
import (
	"fmt"
	"os"
)
func persistRaw(value string) { fmt.Fprintln(os.Stderr, value) }
`
	wrong["generate_analysis.go"] = strings.Replace(
		wrong["generate_analysis.go"],
		"return result, GenNote{",
		"persistRaw(result.Summary)\n\treturn result, GenNote{", 1,
	)
	if err := validateS7PureGeneratorPackage(wrong); err == nil ||
		!strings.Contains(err.Error(), "persistRaw") {
		t.Fatalf("PIB-420 same validator accepted cross-file fmt.Fprintln(os.Stderr, response): %v", err)
	}
}

func TestS7PIB446RawResponseStructuralGuardAndWrongInput(t *testing.T) {
	sources := s7WorkflowProductionSources(t)
	t.Run("baseline", func(t *testing.T) {
		if err := validateS7PureGeneratorPackage(sources); err != nil {
			t.Fatal(err)
		}
	})
	fixtures := []struct {
		name   string
		mutate func(map[string]string)
		want   []string
	}{
		{
			name: "legacy-retry-store",
			mutate: func(wrong map[string]string) {
				wrong["generate_spec.go"] = strings.Replace(
					wrong["generate_spec.go"],
					"generateWithRetryForAuthority(",
					"GenerateWithRetry(", 1,
				)
				wrong["generate_spec.go"] = strings.Replace(
					wrong["generate_spec.go"],
					"}, in.Authority)",
					"})", 1,
				)
			},
			want: []string{"authority retry calls = 0"},
		},
		{
			name: "forbidden-legacy-retry-identity",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
func consumeLegacyRetry(identity any, value string) {
	_, _ = identity, value
}
func passLegacyRetry(value string) {
	consumeLegacyRetry(GenerateWithRetry, value)
}
`
				wrong["generate_analysis.go"] = strings.Replace(
					wrong["generate_analysis.go"],
					"return result, GenNote{",
					"passLegacyRetry(result.Summary)\n\treturn result, GenNote{", 1,
				)
			},
			want: []string{"GenerateWithRetry"},
		},
		{
			name: "forbidden-retry-store-field",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
func inspectRetryStore(opts RetryOptions, value string) {
	_, _ = opts.Store, value
}
`
				wrong["generate_spec.go"] = strings.Replace(
					wrong["generate_spec.go"],
					`return fmt.Sprintf("# Specification: %s\n\n%s\n", in.Slug, response), GenNote{`,
					`inspectRetryStore(RetryOptions{}, response)
	return fmt.Sprintf("# Specification: %s\n\n%s\n", in.Slug, response), GenNote{`, 1,
				)
			},
			want: []string{"RetryOptions.Store"},
		},
		{
			name: "forbidden-central-legacy-identities",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
import (
	"fmt"
	"io"
	"log"
	"os"
)
func consumeLegacySinkIdentities(value string) {
	_, _, _ = os.Create, os.OpenFile, os.Rename
	_, _, _ = fmt.Print, fmt.Fprintln, log.Print
	_, _, _ = io.Copy, io.WriteString, value
}
`
				wrong["generate_exploration.go"] = strings.Replace(
					wrong["generate_exploration.go"],
					`return fmt.Sprintf("# Exploration: %s\n\n%s\n", in.Slug, response), GenNote{`,
					`consumeLegacySinkIdentities(response)
	return fmt.Sprintf("# Exploration: %s\n\n%s\n", in.Slug, response), GenNote{`, 1,
				)
			},
			want: []string{
				"os.Create", "os.OpenFile", "os.Rename",
				"fmt.Print", "fmt.Fprintln", "log.Print",
				"io.Copy", "io.WriteString",
			},
		},
		{
			name: "forbidden-write-artifact-method",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
type rawArtifactStore struct{}
func (rawArtifactStore) WriteArtifact(string, string, string) error { return nil }
func persistThroughArtifact(value string) {
	_ = (rawArtifactStore{}).WriteArtifact("s7", "provider-history.log", value)
}
`
				wrong["generate_analysis.go"] = strings.Replace(
					wrong["generate_analysis.go"],
					"return result, GenNote{",
					"persistThroughArtifact(result.Summary)\n\treturn result, GenNote{", 1,
				)
			},
			want: []string{"WriteArtifact"},
		},
		{
			name: "cross-file-report-history-sink",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
import "os"
func persistReportHistory(value string) { _ = os.WriteFile("provider-history.log", []byte(value), 0600) }
`
				wrong["generate_exploration.go"] = strings.Replace(
					wrong["generate_exploration.go"],
					`return fmt.Sprintf("# Exploration: %s\n\n%s\n", in.Slug, response), GenNote{`,
					`persistReportHistory(response)
	return fmt.Sprintf("# Exploration: %s\n\n%s\n", in.Slug, response), GenNote{`, 1,
				)
			},
			want: []string{"os.WriteFile"},
		},
		{
			name: "cross-file-stdout-writer",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
import (
	"fmt"
	"os"
)
func printRawResponse(value string) { fmt.Fprintln(os.Stdout, value) }
`
				wrong["generate_spec.go"] = strings.Replace(
					wrong["generate_spec.go"],
					`return fmt.Sprintf("# Specification: %s\n\n%s\n", in.Slug, response), GenNote{`,
					`printRawResponse(response)
	return fmt.Sprintf("# Specification: %s\n\n%s\n", in.Slug, response), GenNote{`, 1,
				)
			},
			want: []string{"fmt.Fprintln"},
		},
		{
			name: "cross-file-fmt-file-writer",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
import (
	"fmt"
	"os"
)
func writeRawResponse(value string) {
	file, _ := os.Create("provider-transcript.log")
	if file != nil {
		fmt.Fprintln(file, value)
		_ = file.Close()
	}
}
`
				wrong["generate_exploration.go"] = strings.Replace(
					wrong["generate_exploration.go"],
					`return fmt.Sprintf("# Exploration: %s\n\n%s\n", in.Slug, response), GenNote{`,
					`writeRawResponse(response)
	return fmt.Sprintf("# Exploration: %s\n\n%s\n", in.Slug, response), GenNote{`, 1,
				)
			},
			want: []string{"os.Create", "fmt.Fprintln"},
		},
		{
			name: "cross-file-create-temp-file-method",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
import "os"
func openRawTemp() *os.File {
	file, _ := os.CreateTemp("", "provider-response-*")
	return file
}
func writeTempRaw(value string) {
	file := openRawTemp()
	if file != nil {
		_, _ = file.WriteString(value)
		_ = file.Close()
	}
}
`
				wrong["generate_analysis.go"] = strings.Replace(
					wrong["generate_analysis.go"],
					"return result, GenNote{",
					"writeTempRaw(result.Summary)\n\treturn result, GenNote{", 1,
				)
			},
			want: []string{"os.CreateTemp", "*os.File.WriteString"},
		},
		{
			name: "forbidden-identity-helper-parameter",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
import "os"
func acceptRawIdentity(identity any, value string) {
	_, _ = identity, value
}
func passRawIdentity(value string) {
	acceptRawIdentity(os.CreateTemp, value)
}
`
				wrong["generate_analysis.go"] = strings.Replace(
					wrong["generate_analysis.go"],
					"return result, GenNote{",
					"passRawIdentity(result.Summary)\n\treturn result, GenNote{", 1,
				)
			},
			want: []string{"os.CreateTemp"},
		},
		{
			name: "forbidden-identity-helper-return",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
import "os"
func returnedRawIdentity() any { return os.CreateTemp }
func consumeReturnedIdentity(value string) {
	_, _ = returnedRawIdentity(), value
}
`
				wrong["generate_exploration.go"] = strings.Replace(
					wrong["generate_exploration.go"],
					`return fmt.Sprintf("# Exploration: %s\n\n%s\n", in.Slug, response), GenNote{`,
					`consumeReturnedIdentity(response)
	return fmt.Sprintf("# Exploration: %s\n\n%s\n", in.Slug, response), GenNote{`, 1,
				)
			},
			want: []string{"os.CreateTemp"},
		},
		{
			name: "forbidden-interface-package-variable",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
import "os"
var rawInterfaceIdentity any = os.CreateTemp
func consumeInterfaceIdentity(value string) {
	_, _ = rawInterfaceIdentity, value
}
`
				wrong["generate_spec.go"] = strings.Replace(
					wrong["generate_spec.go"],
					`return fmt.Sprintf("# Specification: %s\n\n%s\n", in.Slug, response), GenNote{`,
					`consumeInterfaceIdentity(response)
	return fmt.Sprintf("# Specification: %s\n\n%s\n", in.Slug, response), GenNote{`, 1,
				)
			},
			want: []string{"os.CreateTemp"},
		},
		{
			name: "forbidden-package-variable-alias-chain",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
import "os"
var rawCreateIdentity = os.CreateTemp
var rawCreateAlias = rawCreateIdentity
func consumePackageAlias(value string) {
	_, _ = rawCreateAlias, value
}
`
				wrong["generate_analysis.go"] = strings.Replace(
					wrong["generate_analysis.go"],
					"return result, GenNote{",
					"consumePackageAlias(result.Summary)\n\treturn result, GenNote{", 1,
				)
			},
			want: []string{"os.CreateTemp"},
		},
		{
			name: "forbidden-selector-method-dispatch",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
import "os"
type rawIdentityHolder struct{}
func (rawIdentityHolder) Identity() any { return (*os.File).WriteString }
func consumeSelectedIdentity(value string) {
	_, _ = (rawIdentityHolder{}).Identity(), value
}
`
				wrong["generate_exploration.go"] = strings.Replace(
					wrong["generate_exploration.go"],
					`return fmt.Sprintf("# Exploration: %s\n\n%s\n", in.Slug, response), GenNote{`,
					`consumeSelectedIdentity(response)
	return fmt.Sprintf("# Exploration: %s\n\n%s\n", in.Slug, response), GenNote{`, 1,
				)
			},
			want: []string{"*os.File.WriteString"},
		},
		{
			name: "forbidden-init-assigned-package-aliases",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
import "os"
var initCreateIdentity any
var initWriteIdentity any
func init() {
	initCreateIdentity = os.CreateTemp
	initWriteIdentity = (*os.File).WriteString
}
func consumeInitIdentity(value string) {
	_, _, _ = initCreateIdentity, initWriteIdentity, value
}
`
				wrong["generate_spec.go"] = strings.Replace(
					wrong["generate_spec.go"],
					`return fmt.Sprintf("# Specification: %s\n\n%s\n", in.Slug, response), GenNote{`,
					`consumeInitIdentity(response)
	return fmt.Sprintf("# Specification: %s\n\n%s\n", in.Slug, response), GenNote{`, 1,
				)
			},
			want: []string{"os.CreateTemp", "*os.File.WriteString"},
		},
		{
			name: "forbidden-init-assigned-legacy-sink",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
import "os"
var initLegacyWrite func(string, []byte, os.FileMode) error
func init() {
	initLegacyWrite = os.WriteFile
}
func consumeInitLegacy(value string) {
	path := "provider-history.log"
	_ = initLegacyWrite(path, []byte(value), 0600)
	_ = os.Remove(path)
}
`
				wrong["generate_exploration.go"] = strings.Replace(
					wrong["generate_exploration.go"],
					`return fmt.Sprintf("# Exploration: %s\n\n%s\n", in.Slug, response), GenNote{`,
					`consumeInitLegacy(response)
	return fmt.Sprintf("# Exploration: %s\n\n%s\n", in.Slug, response), GenNote{`, 1,
				)
			},
			want: []string{"os.WriteFile"},
		},
		{
			name: "forbidden-imported-interface-local-method",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
import (
	"io"
	"os"
)
type rawInterfaceWriter struct{}
func (rawInterfaceWriter) Write(value []byte) (int, error) {
	file, err := os.CreateTemp("", "provider-response-*")
	if err != nil {
		return 0, err
	}
	_, _ = file.Write(value)
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return len(value), nil
}
func consumeThroughImportedInterface(value string) {
	var writer io.Writer = rawInterfaceWriter{}
	_, _ = writer.Write([]byte(value))
}
`
				wrong["generate_analysis.go"] = strings.Replace(
					wrong["generate_analysis.go"],
					"return result, GenNote{",
					"consumeThroughImportedInterface(result.Summary)\n\treturn result, GenNote{", 1,
				)
			},
			want: []string{"os.CreateTemp", "*os.File.Write"},
		},
		{
			name: "forbidden-imported-interface-writefile-method",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
import (
	"io"
	"os"
)
type rawWriteFileWriter struct{}
func (rawWriteFileWriter) Write(value []byte) (int, error) {
	path := "provider-response.log"
	if err := os.WriteFile(path, value, 0600); err != nil {
		return 0, err
	}
	_ = os.Remove(path)
	return len(value), nil
}
func consumeThroughWriteFileInterface(value string) {
	var writer io.Writer = rawWriteFileWriter{}
	_, _ = writer.Write([]byte(value))
}
`
				wrong["generate_analysis.go"] = strings.Replace(
					wrong["generate_analysis.go"],
					"return result, GenNote{",
					"consumeThroughWriteFileInterface(result.Summary)\n\treturn result, GenNote{", 1,
				)
			},
			want: []string{"os.WriteFile"},
		},
		{
			name: "forbidden-module-provider-implementation",
			mutate: func(wrong map[string]string) {
				wrong["generate_types.go"] = strings.Replace(
					wrong["generate_types.go"],
					`"errors"`,
					`"errors"
	"os"`,
					1,
				)
				wrong["generate_types.go"] = strings.Replace(
					wrong["generate_types.go"],
					`response, err := p.Provider.Generate(ctx, cfg, req)`,
					`response, err := p.Provider.Generate(ctx, cfg, req)
	path := "provider-module-response.log"
	_ = os.WriteFile(path, []byte(response), 0600)
	_ = os.Remove(path)`,
					1,
				)
			},
			want: []string{"Generate: os.WriteFile"},
		},
		{
			name: "forbidden-generic-writer-dispatch",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
import (
	"io"
	"os"
)
type rawGenericWriter struct{}
func (rawGenericWriter) Write(value []byte) (int, error) {
	path := "provider-generic.log"
	if err := os.WriteFile(path, value, 0600); err != nil {
		return 0, err
	}
	_ = os.Remove(path)
	return len(value), nil
}
func consumeGenericWriter[T io.Writer](writer T, value string) {
	_, _ = writer.Write([]byte(value))
}
func dispatchGenericWriter(value string) {
	consumeGenericWriter(rawGenericWriter{}, value)
}
`
				wrong["generate_spec.go"] = strings.Replace(
					wrong["generate_spec.go"],
					`return fmt.Sprintf("# Specification: %s\n\n%s\n", in.Slug, response), GenNote{`,
					`dispatchGenericWriter(response)
	return fmt.Sprintf("# Specification: %s\n\n%s\n", in.Slug, response), GenNote{`, 1,
				)
			},
			want: []string{"os.WriteFile"},
		},
		{
			name: "fail-closed-unresolved-module-import",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
import "github.com/tesseracode/tesserapatch/internal/missing-s7-package"
var _ = missings7package.Value
`
			},
			want: []string{
				"module-aware go list",
				"github.com/tesseracode/tesserapatch/internal/missing-s7-package",
			},
		},
		{
			name: "fail-closed-type-error",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
var s7BrokenType int = "not-an-int"
`
			},
			want: []string{"type check failed closed", "cannot use"},
		},
		{
			name: "cross-file-create-temp-alias-write-unlink",
			mutate: func(wrong map[string]string) {
				wrong["privacy_wrong.go"] = `package workflow
import "os"
func persistAliasedTemp(value string) {
	create := os.CreateTemp
	file, err := create("", "provider-response-*")
	if err != nil {
		return
	}
	writeExpression := (*os.File).WriteString
	_, _ = writeExpression(file, value)
	writeValue := file.WriteString
	_, _ = writeValue(value)
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
}
`
				wrong["generate_spec.go"] = strings.Replace(
					wrong["generate_spec.go"],
					`return fmt.Sprintf("# Specification: %s\n\n%s\n", in.Slug, response), GenNote{`,
					`persistAliasedTemp(response)
	return fmt.Sprintf("# Specification: %s\n\n%s\n", in.Slug, response), GenNote{`, 1,
				)
			},
			want: []string{"os.CreateTemp", "*os.File.WriteString"},
		},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			wrong := s7CloneWorkflowSources(sources)
			fixture.mutate(wrong)
			err := validateS7PureGeneratorPackage(wrong)
			if err == nil {
				t.Fatal("same PIB-446 validator accepted a retry/report/history sink")
			}
			if !strings.HasPrefix(fixture.name, "fail-closed-") &&
				(strings.Contains(err.Error(), "type check failed closed") ||
					strings.Contains(err.Error(), "module-aware go list")) {
				t.Fatalf("PIB-446 sink fixture did not type-check: %v", err)
			}
			for _, fragment := range fixture.want {
				if !strings.Contains(err.Error(), fragment) {
					t.Fatalf("same PIB-446 validator did not report %s: %v", fragment, err)
				}
			}
		})
	}
}

func s7WorkflowProductionSources(t *testing.T) map[string]string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(current)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		sources[entry.Name()] = string(data)
	}
	return sources
}

func s7CloneWorkflowSources(sources map[string]string) map[string]string {
	clone := make(map[string]string, len(sources))
	for name, source := range sources {
		clone[name] = source
	}
	return clone
}

func validateS7PureGeneratorPackage(sources map[string]string) error {
	functions := map[string][]*ast.FuncDecl{}
	fileSet := token.NewFileSet()
	var parsedFiles []*ast.File
	for name, source := range sources {
		file, err := parser.ParseFile(fileSet, name, source, 0)
		if err != nil {
			return err
		}
		parsedFiles = append(parsedFiles, file)
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			functions[function.Name.Name] = append(functions[function.Name.Name], function)
		}
	}
	typeInfo, err := s7WorkflowTypeInformation(fileSet, parsedFiles)
	if err != nil {
		return err
	}
	functionObjects := s7WorkflowFunctionObjects(functions, typeInfo)
	packageInitializers := s7WorkflowPackageInitializers(parsedFiles, typeInfo)
	retryStoreFields := s7WorkflowRetryStoreFields(parsedFiles, typeInfo)
	roots := []string{"GenerateAnalysis", "GenerateSpec", "GenerateExploration"}
	for _, root := range roots {
		if len(functions[root]) != 1 {
			return fmt.Errorf("S7 pure generator root %s count = %d", root, len(functions[root]))
		}
		direct := 0
		ast.Inspect(functions[root][0].Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			ident, _ := call.Fun.(*ast.Ident)
			if ident != nil && ident.Name == "generateWithRetryForAuthority" {
				direct++
			}
			return true
		})
		if direct != 1 {
			return fmt.Errorf("S7 pure generator %s authority retry calls = %d, want 1", root, direct)
		}
	}
	rootFunctions := make([]*ast.FuncDecl, 0, len(roots))
	for _, root := range roots {
		rootFunctions = append(rootFunctions, functions[root][0])
	}
	allRoots := append([]*ast.FuncDecl(nil), rootFunctions...)
	allRoots = append(allRoots, functions["init"]...)
	reachable, reachableInitializers := s7WorkflowReachableConstruction(
		allRoots,
		functionObjects,
		packageInitializers,
		typeInfo,
	)
	var issues []string
	for function := range reachable {
		values := s7WorkflowTypedValues(function, typeInfo)
		for _, identity := range s7WorkflowForbiddenIdentities(
			function.Body, function, typeInfo, values, retryStoreFields,
		) {
			issues = append(issues, function.Name.Name+": "+identity)
		}
	}
	for initializer := range reachableInitializers {
		for _, expression := range initializer.expressions {
			for _, identity := range s7WorkflowForbiddenIdentities(
				expression, nil, typeInfo, nil, retryStoreFields,
			) {
				issues = append(issues, "package var "+initializer.name+": "+identity)
			}
		}
	}
	if len(issues) != 0 {
		sort.Strings(issues)
		return fmt.Errorf("S7 pure generator call graph reaches sinks: %s", strings.Join(issues, "; "))
	}
	if err := s7ValidateInMemoryRetryPolicy(functions); err != nil {
		return err
	}
	return nil
}

type s7WorkflowExportPackage struct {
	ImportPath string
	Export     string
	Incomplete bool
	Error      *struct {
		Err string
	}
}

type s7WorkflowExportCatalog struct {
	mu         sync.Mutex
	baseLoaded bool
	exports    map[string]string
}

var s7WorkflowExports = s7WorkflowExportCatalog{
	exports: map[string]string{},
}

func s7WorkflowTypeInformation(
	fileSet *token.FileSet,
	files []*ast.File,
) (*types.Info, error) {
	info := &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	importPaths := map[string]bool{}
	for _, file := range files {
		for _, specification := range file.Imports {
			pathValue, err := strconv.Unquote(specification.Path.Value)
			if err != nil {
				return nil, fmt.Errorf("decode workflow import: %w", err)
			}
			if pathValue != "unsafe" {
				importPaths[pathValue] = true
			}
		}
	}
	exports, err := s7WorkflowModuleExports(importPaths)
	if err != nil {
		return nil, err
	}
	moduleImporter := importer.ForCompiler(
		fileSet,
		"gc",
		func(pathValue string) (io.ReadCloser, error) {
			exportPath := exports[pathValue]
			if exportPath == "" {
				return nil, fmt.Errorf(
					"module-aware importer has no export data for %q",
					pathValue,
				)
			}
			file, err := os.Open(exportPath)
			if err != nil {
				return nil, fmt.Errorf(
					"open export data for %q: %w",
					pathValue,
					err,
				)
			}
			return file, nil
		},
	)
	var typeErrors []string
	config := types.Config{
		Importer: moduleImporter,
		Error: func(typeErr error) {
			typeErrors = append(typeErrors, typeErr.Error())
		},
	}
	_, checkErr := config.Check(
		"github.com/tesseracode/tesserapatch/internal/workflow",
		fileSet,
		files,
		info,
	)
	if len(typeErrors) == 0 && checkErr != nil {
		typeErrors = append(typeErrors, checkErr.Error())
	}
	if len(typeErrors) != 0 {
		return nil, fmt.Errorf(
			"S7 workflow type check failed closed: %s",
			strings.Join(typeErrors, "; "),
		)
	}
	return info, nil
}

func s7WorkflowModuleExports(importPaths map[string]bool) (map[string]string, error) {
	s7WorkflowExports.mu.Lock()
	defer s7WorkflowExports.mu.Unlock()

	repositoryRoot, err := s7WorkflowRepositoryRoot()
	if err != nil {
		return nil, err
	}
	if !s7WorkflowExports.baseLoaded {
		if err := s7WorkflowLoadExports(
			repositoryRoot,
			[]string{"./internal/workflow"},
			s7WorkflowExports.exports,
		); err != nil {
			return nil, err
		}
		s7WorkflowExports.baseLoaded = true
	}
	var missing []string
	for pathValue := range importPaths {
		if s7WorkflowExports.exports[pathValue] == "" {
			missing = append(missing, pathValue)
		}
	}
	if len(missing) != 0 {
		sort.Strings(missing)
		if err := s7WorkflowLoadExports(
			repositoryRoot,
			missing,
			s7WorkflowExports.exports,
		); err != nil {
			return nil, err
		}
	}
	result := make(map[string]string, len(s7WorkflowExports.exports))
	for pathValue, exportPath := range s7WorkflowExports.exports {
		result[pathValue] = exportPath
	}
	return result, nil
}

func s7WorkflowRepositoryRoot() (string, error) {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("resolve workflow validator source path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return "", fmt.Errorf("resolve workflow module root: %w", err)
	}
	return root, nil
}

func s7WorkflowLoadExports(
	repositoryRoot string,
	patterns []string,
	exports map[string]string,
) error {
	args := append([]string{"list", "-deps", "-export", "-json"}, patterns...)
	command := exec.Command("go", args...)
	command.Dir = repositoryRoot
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf(
			"module-aware go list %q failed: %w: %s",
			patterns,
			err,
			strings.TrimSpace(stderr.String()),
		)
	}
	decoder := json.NewDecoder(&stdout)
	for {
		var listed s7WorkflowExportPackage
		err := decoder.Decode(&listed)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("decode module-aware go list: %w", err)
		}
		if listed.Error != nil {
			return fmt.Errorf(
				"module-aware go list %s: %s",
				listed.ImportPath,
				listed.Error.Err,
			)
		}
		if listed.Incomplete {
			return fmt.Errorf(
				"module-aware go list returned incomplete package %s",
				listed.ImportPath,
			)
		}
		if listed.ImportPath != "" && listed.Export != "" {
			exports[listed.ImportPath] = listed.Export
		}
	}
	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, ".") {
			continue
		}
		if pattern != "unsafe" && exports[pattern] == "" {
			return fmt.Errorf(
				"module-aware go list returned no export data for %s",
				pattern,
			)
		}
	}
	return nil
}

type s7WorkflowPackageInitializer struct {
	name        string
	expressions []ast.Expr
}

func s7WorkflowFunctionObjects(
	functions map[string][]*ast.FuncDecl,
	info *types.Info,
) map[types.Object]*ast.FuncDecl {
	result := map[types.Object]*ast.FuncDecl{}
	for _, declarations := range functions {
		for _, function := range declarations {
			if object := info.Defs[function.Name]; object != nil {
				result[object] = function
			}
		}
	}
	return result
}

func s7WorkflowPackageInitializers(
	files []*ast.File,
	info *types.Info,
) map[types.Object]*s7WorkflowPackageInitializer {
	result := map[types.Object]*s7WorkflowPackageInitializer{}
	for _, file := range files {
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.VAR {
				continue
			}
			for _, specification := range general.Specs {
				value, ok := specification.(*ast.ValueSpec)
				if !ok || len(value.Values) == 0 {
					continue
				}
				names := make([]string, 0, len(value.Names))
				for _, name := range value.Names {
					names = append(names, name.Name)
				}
				initializer := &s7WorkflowPackageInitializer{
					name:        strings.Join(names, ","),
					expressions: append([]ast.Expr(nil), value.Values...),
				}
				for _, name := range value.Names {
					if object := info.Defs[name]; object != nil {
						result[object] = initializer
					}
				}
			}
		}
	}
	return result
}

func s7WorkflowRetryStoreFields(
	files []*ast.File,
	info *types.Info,
) map[types.Object]bool {
	fields := map[types.Object]bool{}
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			specification, ok := node.(*ast.TypeSpec)
			if !ok || specification.Name.Name != "RetryOptions" {
				return true
			}
			structure, ok := specification.Type.(*ast.StructType)
			if !ok {
				return false
			}
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					if name.Name == "Store" && info.Defs[name] != nil {
						fields[info.Defs[name]] = true
					}
				}
			}
			return false
		})
	}
	return fields
}

func s7WorkflowReachableConstruction(
	roots []*ast.FuncDecl,
	functionObjects map[types.Object]*ast.FuncDecl,
	packageInitializers map[types.Object]*s7WorkflowPackageInitializer,
	info *types.Info,
) (
	map[*ast.FuncDecl]bool,
	map[*s7WorkflowPackageInitializer]bool,
) {
	reachableFunctions := map[*ast.FuncDecl]bool{}
	reachableInitializers := map[*s7WorkflowPackageInitializer]bool{}
	var functionQueue []*ast.FuncDecl
	var initializerQueue []*s7WorkflowPackageInitializer
	methodsByName := map[string][]*ast.FuncDecl{}
	for object, function := range functionObjects {
		typed, ok := object.(*types.Func)
		if !ok {
			continue
		}
		signature, _ := typed.Type().(*types.Signature)
		if signature != nil && signature.Recv() != nil {
			methodsByName[typed.Name()] = append(methodsByName[typed.Name()], function)
		}
	}
	enqueueFunction := func(function *ast.FuncDecl) {
		if function != nil && !reachableFunctions[function] {
			reachableFunctions[function] = true
			functionQueue = append(functionQueue, function)
		}
	}
	enqueueInitializer := func(initializer *s7WorkflowPackageInitializer) {
		if initializer != nil && !reachableInitializers[initializer] {
			reachableInitializers[initializer] = true
			initializerQueue = append(initializerQueue, initializer)
		}
	}
	for _, root := range roots {
		enqueueFunction(root)
	}
	scan := func(node ast.Node) {
		ast.Inspect(node, func(current ast.Node) bool {
			var object types.Object
			switch typed := current.(type) {
			case *ast.Ident:
				object = info.Uses[typed]
			case *ast.SelectorExpr:
				if selection := info.Selections[typed]; selection != nil {
					object = selection.Obj()
					if interfaceType := s7WorkflowInterfaceType(selection.Recv()); interfaceType != nil {
						for candidate, method := range functionObjects {
							function, ok := candidate.(*types.Func)
							if !ok || function.Name() != selection.Obj().Name() {
								continue
							}
							signature, _ := function.Type().(*types.Signature)
							if signature != nil && signature.Recv() != nil &&
								types.Implements(signature.Recv().Type(), interfaceType) {
								enqueueFunction(method)
							}
						}
					}
				} else {
					object = info.ObjectOf(typed.Sel)
				}
			}
			if object == nil {
				return true
			}
			enqueueFunction(functionObjects[object])
			enqueueInitializer(packageInitializers[object])
			if function, ok := object.(*types.Func); ok &&
				functionObjects[object] == nil &&
				function.Pkg() != nil &&
				function.Pkg().Path() == "github.com/tesseracode/tesserapatch/internal/workflow" {
				for _, method := range methodsByName[function.Name()] {
					enqueueFunction(method)
				}
			}
			return true
		})
	}
	for len(functionQueue) != 0 || len(initializerQueue) != 0 {
		for len(functionQueue) != 0 {
			function := functionQueue[0]
			functionQueue = functionQueue[1:]
			scan(function.Body)
		}
		for len(initializerQueue) != 0 {
			initializer := initializerQueue[0]
			initializerQueue = initializerQueue[1:]
			for _, expression := range initializer.expressions {
				scan(expression)
			}
		}
	}
	return reachableFunctions, reachableInitializers
}

func s7WorkflowInterfaceType(typeValue types.Type) *types.Interface {
	if typeValue == nil {
		return nil
	}
	typeValue = types.Unalias(typeValue)
	if parameter, ok := typeValue.(*types.TypeParam); ok {
		typeValue = types.Unalias(parameter.Constraint())
	}
	if named, ok := typeValue.(*types.Named); ok {
		typeValue = named.Underlying()
	}
	interfaceType, ok := typeValue.(*types.Interface)
	if !ok {
		return nil
	}
	interfaceType.Complete()
	return interfaceType
}

type s7WorkflowTypedKind uint8

const (
	s7WorkflowCreateTemp s7WorkflowTypedKind = 1 << iota
	s7WorkflowFileWrite
	s7WorkflowFileWriteString
	s7WorkflowFileValue
)

func s7WorkflowForbiddenIdentities(
	node ast.Node,
	function *ast.FuncDecl,
	info *types.Info,
	values map[types.Object]s7WorkflowTypedKind,
	retryStoreFields map[types.Object]bool,
) []string {
	found := map[string]bool{}
	parents := map[ast.Node]ast.Node{}
	var stack []ast.Node
	ast.Inspect(node, func(current ast.Node) bool {
		if current == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if len(stack) != 0 {
			parents[current] = stack[len(stack)-1]
		}
		stack = append(stack, current)
		return true
	})
	ast.Inspect(node, func(current ast.Node) bool {
		expression, ok := current.(ast.Expr)
		if !ok {
			return true
		}
		if identifier, isIdentifier := expression.(*ast.Ident); isIdentifier {
			if selector, isSelector := parents[identifier].(*ast.SelectorExpr); isSelector && selector.Sel == identifier {
				return true
			}
		}
		kind := s7WorkflowTypedExpressionKind(expression, info, values)
		if kind&s7WorkflowCreateTemp != 0 {
			found["os.CreateTemp"] = true
		}
		if kind&s7WorkflowFileWrite != 0 {
			found["*os.File.Write"] = true
		}
		if kind&s7WorkflowFileWriteString != 0 {
			found["*os.File.WriteString"] = true
		}
		identity := s7WorkflowForbiddenObjectIdentity(
			s7WorkflowExpressionObject(expression, info),
			retryStoreFields,
		)
		if identity == "" {
			if selector, isSelector := expression.(*ast.SelectorExpr); isSelector && selector.Sel.Name == "WriteArtifact" {
				identity = "WriteArtifact"
			}
		}
		if identity == "RetryOptions.Store" &&
			s7WorkflowAllowedRetryStoreClear(
				expression, parents[expression], function,
			) {
			identity = ""
		}
		if strings.HasPrefix(identity, "fmt.Fprint") &&
			s7WorkflowDirectCallHasSafeWriter(expression, parents[expression], info) {
			identity = ""
		}
		if identity != "" && s7WorkflowAllowedLegacySpinnerIdentity(
			identity,
			expression,
			parents[expression],
			function,
			info,
		) {
			identity = ""
		}
		if identity != "" {
			found[identity] = true
		}
		return true
	})
	result := make([]string, 0, len(found))
	for identity := range found {
		result = append(result, identity)
	}
	sort.Strings(result)
	return result
}

func s7WorkflowExpressionObject(
	expression ast.Expr,
	info *types.Info,
) types.Object {
	switch typed := expression.(type) {
	case *ast.Ident:
		return s7WorkflowIdentifierObject(typed, info)
	case *ast.SelectorExpr:
		if selection := info.Selections[typed]; selection != nil {
			return selection.Obj()
		}
		return info.ObjectOf(typed.Sel)
	}
	return nil
}

func s7WorkflowForbiddenObjectIdentity(
	object types.Object,
	retryStoreFields map[types.Object]bool,
) string {
	if retryStoreFields[object] {
		return "RetryOptions.Store"
	}
	if variable, ok := object.(*types.Var); ok && variable.Pkg() != nil &&
		variable.Pkg().Path() == "os" &&
		(variable.Name() == "Stdout" || variable.Name() == "Stderr") {
		return "os." + variable.Name()
	}
	function, ok := object.(*types.Func)
	if !ok {
		return ""
	}
	signature, _ := function.Type().(*types.Signature)
	if signature != nil && signature.Recv() != nil &&
		s7WorkflowIsOSFileType(signature.Recv().Type()) {
		switch function.Name() {
		case "Write":
			return "*os.File.Write"
		case "WriteString":
			return "*os.File.WriteString"
		}
	}
	pathValue := ""
	if function.Pkg() != nil {
		pathValue = function.Pkg().Path()
	}
	switch pathValue {
	case "os":
		switch function.Name() {
		case "WriteFile", "Create", "OpenFile", "Rename", "CreateTemp":
			return "os." + function.Name()
		}
	case "fmt":
		switch function.Name() {
		case "Fprint", "Fprintf", "Fprintln", "Print", "Printf", "Println":
			return "fmt." + function.Name()
		}
	case "log":
		switch function.Name() {
		case "Print", "Printf", "Println":
			return "log." + function.Name()
		}
	case "io":
		switch function.Name() {
		case "Copy", "WriteString":
			return "io." + function.Name()
		}
	case "github.com/tesseracode/tesserapatch/internal/workflow":
		if function.Name() == "GenerateWithRetry" {
			return function.Name()
		}
	}
	if function.Name() == "WriteArtifact" {
		return "WriteArtifact"
	}
	return ""
}

func s7WorkflowAllowedRetryStoreClear(
	expression ast.Expr,
	parent ast.Node,
	function *ast.FuncDecl,
) bool {
	if function == nil || function.Name.Name != "generateWithRetryInMemory" {
		return false
	}
	assignment, ok := parent.(*ast.AssignStmt)
	if !ok || len(assignment.Lhs) != len(assignment.Rhs) {
		return false
	}
	for index, left := range assignment.Lhs {
		if left != expression {
			continue
		}
		nilValue, isIdentifier := assignment.Rhs[index].(*ast.Ident)
		return isIdentifier && nilValue.Name == "nil" && nilValue.Obj == nil
	}
	return false
}

func s7WorkflowAllowedLegacySpinnerIdentity(
	identity string,
	expression ast.Expr,
	parent ast.Node,
	function *ast.FuncDecl,
	info *types.Info,
) bool {
	if function == nil {
		return false
	}
	receiverType := s7WorkflowReceiverTypeName(function, info)
	switch {
	case identity == "os.Stderr" &&
		function.Name.Name == "Generate" &&
		receiverType == "legacyPhaseRecordingProvider":
		call, ok := parent.(*ast.CallExpr)
		if !ok {
			return false
		}
		identifier, _ := call.Fun.(*ast.Ident)
		return identifier != nil &&
			identifier.Name == "NewSpinnerIfTTY" &&
			len(call.Args) != 0 && call.Args[0] == expression
	case identity == "os.Stderr" && function.Name.Name == "isTerminal":
		assignment, ok := parent.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 ||
			assignment.Rhs[0] != expression {
			return false
		}
		left, _ := assignment.Lhs[0].(*ast.Ident)
		return left != nil && left.Name == "f"
	case identity == "fmt.Fprintf" &&
		function.Name.Name == "run" && receiverType == "Spinner":
		return s7WorkflowSpinnerWriterCall(expression, parent, function)
	case identity == "fmt.Fprint" &&
		function.Name.Name == "Stop" && receiverType == "Spinner":
		return s7WorkflowSpinnerWriterCall(expression, parent, function)
	default:
		return false
	}
}

func s7WorkflowReceiverTypeName(
	function *ast.FuncDecl,
	info *types.Info,
) string {
	object, _ := info.Defs[function.Name].(*types.Func)
	if object == nil {
		return ""
	}
	signature, _ := object.Type().(*types.Signature)
	if signature == nil || signature.Recv() == nil {
		return ""
	}
	typeValue := types.Unalias(signature.Recv().Type())
	if pointer, ok := typeValue.(*types.Pointer); ok {
		typeValue = types.Unalias(pointer.Elem())
	}
	named, _ := typeValue.(*types.Named)
	if named == nil || named.Obj() == nil {
		return ""
	}
	return named.Obj().Name()
}

func s7WorkflowSpinnerWriterCall(
	expression ast.Expr,
	parent ast.Node,
	function *ast.FuncDecl,
) bool {
	call, ok := parent.(*ast.CallExpr)
	if !ok || call.Fun != expression || len(call.Args) == 0 ||
		function.Recv == nil || len(function.Recv.List) != 1 ||
		len(function.Recv.List[0].Names) != 1 {
		return false
	}
	receiver := function.Recv.List[0].Names[0].Name
	destination, ok := call.Args[0].(*ast.SelectorExpr)
	if !ok {
		return false
	}
	base, _ := destination.X.(*ast.Ident)
	if base == nil || base.Name != receiver || destination.Sel.Name != "w" {
		return false
	}
	switch function.Name.Name {
	case "run":
		if len(call.Args) != 4 {
			return false
		}
		formatValue, ok := s7WorkflowStringLiteral(call.Args[1])
		return ok && formatValue == "\r%s %s"
	case "Stop":
		if len(call.Args) != 2 {
			return false
		}
		clear, _ := call.Args[1].(*ast.Ident)
		return clear != nil && clear.Name == "clear"
	default:
		return false
	}
}

func s7WorkflowStringLiteral(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}

func s7WorkflowDirectCallHasSafeWriter(
	expression ast.Expr,
	parent ast.Node,
	info *types.Info,
) bool {
	call, ok := parent.(*ast.CallExpr)
	if !ok || call.Fun != expression || len(call.Args) == 0 {
		return false
	}
	if selector, isSelector := call.Args[0].(*ast.SelectorExpr); isSelector {
		if object := info.ObjectOf(selector.Sel); object != nil &&
			object.Pkg() != nil && object.Pkg().Path() == "io" &&
			object.Name() == "Discard" {
			return true
		}
	}
	typeValue := types.Unalias(info.TypeOf(call.Args[0]))
	if pointer, isPointer := typeValue.(*types.Pointer); isPointer {
		typeValue = types.Unalias(pointer.Elem())
	}
	named, ok := typeValue.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	pathValue := named.Obj().Pkg().Path()
	name := named.Obj().Name()
	return (pathValue == "bytes" && name == "Buffer") ||
		(pathValue == "strings" && name == "Builder")
}

func s7WorkflowTypedValues(
	function *ast.FuncDecl,
	info *types.Info,
) map[types.Object]s7WorkflowTypedKind {
	values := map[types.Object]s7WorkflowTypedKind{}
	if function.Type.Params != nil {
		for _, field := range function.Type.Params.List {
			for _, name := range field.Names {
				object := info.Defs[name]
				if object != nil && s7WorkflowIsOSFileType(object.Type()) {
					values[object] |= s7WorkflowFileValue
				}
			}
		}
	}
	for changed := true; changed; {
		changed = false
		ast.Inspect(function.Body, func(node ast.Node) bool {
			if literal, ok := node.(*ast.FuncLit); ok && literal != nil {
				return false
			}
			switch statement := node.(type) {
			case *ast.AssignStmt:
				for index, left := range statement.Lhs {
					identifier, ok := left.(*ast.Ident)
					if !ok {
						continue
					}
					object := s7WorkflowIdentifierObject(identifier, info)
					kind := s7WorkflowAssignmentKind(
						statement.Rhs, index, len(statement.Lhs), info, values,
					)
					if object != nil && kind != 0 && values[object]|kind != values[object] {
						values[object] |= kind
						changed = true
					}
				}
			case *ast.ValueSpec:
				for index, name := range statement.Names {
					object := info.Defs[name]
					kind := s7WorkflowAssignmentKind(
						statement.Values, index, len(statement.Names), info, values,
					)
					if object != nil && kind != 0 && values[object]|kind != values[object] {
						values[object] |= kind
						changed = true
					}
				}
			}
			return true
		})
	}
	return values
}

func s7WorkflowAssignmentKind(
	right []ast.Expr,
	index int,
	leftCount int,
	info *types.Info,
	values map[types.Object]s7WorkflowTypedKind,
) s7WorkflowTypedKind {
	if len(right) == 0 {
		return 0
	}
	if len(right) == leftCount && index < len(right) {
		return s7WorkflowTypedExpressionKind(right[index], info, values)
	}
	if len(right) != 1 {
		return 0
	}
	expression := right[0]
	if index == 0 {
		if call, ok := expression.(*ast.CallExpr); ok &&
			s7WorkflowTypedExpressionKind(call.Fun, info, values)&s7WorkflowCreateTemp != 0 {
			return s7WorkflowFileValue
		}
	}
	if tuple, ok := info.TypeOf(expression).(*types.Tuple); ok {
		if index < tuple.Len() && s7WorkflowIsOSFileType(tuple.At(index).Type()) {
			return s7WorkflowFileValue
		}
		return 0
	}
	if index == 0 {
		return s7WorkflowTypedExpressionKind(expression, info, values)
	}
	return 0
}

func s7WorkflowTypedExpressionKind(
	expression ast.Expr,
	info *types.Info,
	values map[types.Object]s7WorkflowTypedKind,
) s7WorkflowTypedKind {
	var kind s7WorkflowTypedKind
	switch typed := expression.(type) {
	case *ast.ParenExpr:
		return s7WorkflowTypedExpressionKind(typed.X, info, values)
	case *ast.Ident:
		object := s7WorkflowIdentifierObject(typed, info)
		kind |= values[object]
		kind |= s7WorkflowTypedObjectKind(object)
	case *ast.SelectorExpr:
		if selection := info.Selections[typed]; selection != nil {
			kind |= s7WorkflowTypedObjectKind(selection.Obj())
		} else {
			kind |= s7WorkflowTypedObjectKind(info.ObjectOf(typed.Sel))
		}
	case *ast.CallExpr:
		if s7WorkflowIsOSFileType(info.TypeOf(typed)) {
			kind |= s7WorkflowFileValue
		}
	}
	if s7WorkflowIsOSFileType(info.TypeOf(expression)) {
		kind |= s7WorkflowFileValue
	}
	return kind
}

func s7WorkflowIdentifierObject(
	identifier *ast.Ident,
	info *types.Info,
) types.Object {
	if object := info.Uses[identifier]; object != nil {
		return object
	}
	return info.Defs[identifier]
}

func s7WorkflowTypedObjectKind(object types.Object) s7WorkflowTypedKind {
	function, ok := object.(*types.Func)
	if !ok || function.Pkg() == nil || function.Pkg().Path() != "os" {
		return 0
	}
	signature, _ := function.Type().(*types.Signature)
	switch function.Name() {
	case "CreateTemp":
		if signature != nil && signature.Recv() == nil {
			return s7WorkflowCreateTemp
		}
	case "Write":
		if signature != nil && signature.Recv() != nil &&
			s7WorkflowIsOSFileType(signature.Recv().Type()) {
			return s7WorkflowFileWrite
		}
	case "WriteString":
		if signature != nil && signature.Recv() != nil &&
			s7WorkflowIsOSFileType(signature.Recv().Type()) {
			return s7WorkflowFileWriteString
		}
	}
	return 0
}

func s7WorkflowIsOSFileType(typeValue types.Type) bool {
	if typeValue == nil {
		return false
	}
	typeValue = types.Unalias(typeValue)
	pointer, ok := typeValue.(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(pointer.Elem()).(*types.Named)
	if !ok {
		return false
	}
	object := named.Obj()
	return object != nil && object.Pkg() != nil &&
		object.Pkg().Path() == "os" && object.Name() == "File"
}

func s7ValidateInMemoryRetryPolicy(functions map[string][]*ast.FuncDecl) error {
	candidates := functions["generateWithRetryInMemory"]
	if len(candidates) != 1 {
		return fmt.Errorf("S7 in-memory retry constructor count = %d", len(candidates))
	}
	function := candidates[0]
	if function.Type.Params == nil || len(function.Type.Params.List) < 2 {
		return fmt.Errorf("S7 in-memory retry constructor parameters are incomplete")
	}
	var opts, enforceContext *ast.Ident
	for _, parameter := range function.Type.Params.List {
		for _, name := range parameter.Names {
			switch name.Name {
			case "opts":
				opts = name
			case "enforceContext":
				enforceContext = name
			}
		}
	}
	if opts == nil || opts.Obj == nil || enforceContext == nil || enforceContext.Obj == nil {
		return fmt.Errorf("S7 in-memory retry constructor lost exact policy parameters")
	}
	storeCleared, slugCleared, delegated := false, false, false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if ok && len(assignment.Lhs) == 1 && len(assignment.Rhs) == 1 {
			selector, selectorOK := assignment.Lhs[0].(*ast.SelectorExpr)
			var base *ast.Ident
			if selectorOK {
				base, _ = selector.X.(*ast.Ident)
			}
			if selectorOK && base != nil && base.Obj == opts.Obj {
				switch selector.Sel.Name {
				case "Store":
					identifier, _ := assignment.Rhs[0].(*ast.Ident)
					storeCleared = identifier != nil && identifier.Obj == nil && identifier.Name == "nil"
				case "Slug":
					literal, _ := assignment.Rhs[0].(*ast.BasicLit)
					if literal != nil && literal.Kind == token.STRING {
						value, err := strconv.Unquote(literal.Value)
						slugCleared = err == nil && value == ""
					}
				}
			}
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		identifier, _ := call.Fun.(*ast.Ident)
		if identifier == nil || identifier.Name != "generateWithRetry" || len(call.Args) != 6 {
			return true
		}
		optsArgument, _ := call.Args[4].(*ast.Ident)
		policy, _ := call.Args[5].(*ast.CompositeLit)
		if optsArgument == nil || optsArgument.Obj != opts.Obj || policy == nil {
			return true
		}
		policyType, _ := policy.Type.(*ast.Ident)
		if policyType == nil || policyType.Name != "retryIOPolicy" || len(policy.Elts) != 1 {
			return true
		}
		field, _ := policy.Elts[0].(*ast.KeyValueExpr)
		if field == nil {
			return true
		}
		key, _ := field.Key.(*ast.Ident)
		value, _ := field.Value.(*ast.Ident)
		delegated = key != nil && key.Name == "enforceContext" &&
			value != nil && value.Obj == enforceContext.Obj
		return true
	})
	if !storeCleared || !slugCleared || !delegated {
		return fmt.Errorf(
			"S7 in-memory retry policy store-cleared=%t slug-cleared=%t delegated=%t",
			storeCleared, slugCleared, delegated,
		)
	}
	return nil
}
