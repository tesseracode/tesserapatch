package gitutil

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestPrepareGitStateThreadsOneG1AndExactG2G3G4(t *testing.T) {
	previous := runGitProcess
	t.Cleanup(func() { runGitProcess = previous })

	var calls []gitProcessRequest
	runGitProcess = func(request gitProcessRequest) gitProcessResult {
		copyRequest := request
		copyRequest.args = append([]string(nil), request.args...)
		copyRequest.env = append([]string(nil), request.env...)
		calls = append(calls, copyRequest)
		switch strings.Join(request.args, "\x00") {
		case "rev-parse\x00--is-inside-work-tree":
			return gitProcessResult{stdout: "true\n"}
		case "check-ignore\x00-q\x00--no-index\x00--\x00.tpatch/local/intent-prepare/demo":
			return gitProcessResult{}
		case "--literal-pathspecs\x00ls-files\x00--\x00.tpatch/local/":
			return gitProcessResult{}
		case "ls-files\x00--\x00.tpatch":
			return gitProcessResult{}
		default:
			t.Fatalf("unexpected argv: %q", request.args)
			return gitProcessResult{}
		}
	}

	state, err := DiscoverGitState("/workspace")
	if err != nil || state != GitWorktree {
		t.Fatalf("G1 = (%q, %v)", state, err)
	}
	if ignored, err := IsIgnoredWithState(
		"/workspace", state, ".tpatch/local/intent-prepare/demo",
	); err != nil || !ignored {
		t.Fatalf("G2 = (%v, %v)", ignored, err)
	}
	if tracked, err := AnythingTrackedUnderWithState("/workspace", state); err != nil || tracked {
		t.Fatalf("G3 = (%v, %v)", tracked, err)
	}
	if tracked, err := IsTpatchTrackedWithState("/workspace", state); err != nil || tracked {
		t.Fatalf("G4 = (%v, %v)", tracked, err)
	}
	if len(calls) != 4 {
		t.Fatalf("calls = %d, want 4", len(calls))
	}
	for _, call := range calls {
		if call.repoRoot != "/workspace" {
			t.Fatalf("cwd = %q", call.repoRoot)
		}
		for _, arg := range call.args {
			if strings.HasPrefix(arg, "/workspace") {
				t.Fatalf("absolute argv leaked: %q", call.args)
			}
		}
	}
}

func TestS7PIB427GitStateExactArgvEnvironmentAndProcessCounts(t *testing.T) {
	previous := runGitProcess
	t.Cleanup(func() { runGitProcess = previous })
	authoritativeScrub := s7PrepareGitAuthoritativeScrubExamples(t)
	runtimeScrub := append(
		append([]string(nil), authoritativeScrub...),
		s7PrepareGitOverflowIndexedNames()...,
	)
	for _, name := range runtimeScrub {
		t.Setenv(name, "S7-FOREIGN-"+name)
	}
	var calls []gitProcessRequest
	runGitProcess = func(request gitProcessRequest) gitProcessResult {
		copyRequest := request
		copyRequest.args = append([]string(nil), request.args...)
		copyRequest.env = append([]string(nil), request.env...)
		calls = append(calls, copyRequest)
		switch strings.Join(request.args, "\x00") {
		case "rev-parse\x00--is-inside-work-tree":
			return gitProcessResult{stdout: "true\n"}
		case "check-ignore\x00-q\x00--no-index\x00--\x00.tpatch/local/intent-prepare/demo":
			return gitProcessResult{}
		case "--literal-pathspecs\x00ls-files\x00--\x00.tpatch/local/":
			return gitProcessResult{}
		case "ls-files\x00--\x00.tpatch":
			return gitProcessResult{}
		default:
			t.Fatalf("unexpected Git argv: %q", request.args)
			return gitProcessResult{}
		}
	}
	state, err := DiscoverGitState("/workspace")
	if err != nil || state != GitWorktree {
		t.Fatalf("G1 = state:%q err:%v", state, err)
	}
	if ignored, err := IsIgnoredWithState(
		"/workspace", state, ".tpatch/local/intent-prepare/demo",
	); err != nil || !ignored {
		t.Fatalf("G2 = ignored:%t err:%v", ignored, err)
	}
	if tracked, err := AnythingTrackedUnderWithState("/workspace", state); err != nil || tracked {
		t.Fatalf("G3 = tracked:%t err:%v", tracked, err)
	}
	if tracked, err := IsTpatchTrackedWithState("/workspace", state); err != nil || tracked {
		t.Fatalf("G4 = tracked:%t err:%v", tracked, err)
	}
	wantArgv := [][]string{
		{"rev-parse", "--is-inside-work-tree"},
		{"check-ignore", "-q", "--no-index", "--", ".tpatch/local/intent-prepare/demo"},
		{"--literal-pathspecs", "ls-files", "--", ".tpatch/local/"},
		{"ls-files", "--", ".tpatch"},
	}
	if err := validateS7PIB427GitRequests(calls, wantArgv); err != nil {
		t.Fatal(err)
	}
	scrubbed := prepareGitEnvironment([]string{
		"GIT_DIR=/foreign", "GIT_WORK_TREE=/foreign-tree",
		"GIT_INDEX_FILE=/foreign-index", "GIT_OBJECT_DIRECTORY=/foreign-objects",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES=/foreign-alt", "GIT_COMMON_DIR=/foreign-common",
		"GIT_CEILING_DIRECTORIES=/",
		"GIT_DISCOVERY_ACROSS_FILESYSTEM=1", "GIT_PREFIX=prefix",
		"GIT_IMPLICIT_WORK_TREE=1", "GIT_SUPER_PREFIX=super",
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=alias.x", "GIT_CONFIG_VALUE_0=!echo",
		"GIT_CONFIG_KEY_999=alias.y", "GIT_CONFIG_VALUE_999=!false",
		s7PrepareGitOverflowIndexedNames()[0] + "=alias.overflow",
		s7PrepareGitOverflowIndexedNames()[1] + "=!overflow",
		"LC_ALL=fr_FR",
		"LANG=fr_FR",
		"PATH=/bin",
	})
	if !reflect.DeepEqual(scrubbed, []string{"PATH=/bin", "LC_ALL=C", "LANG=C"}) {
		t.Fatalf("Git environment scrub = %#v", scrubbed)
	}
	for _, name := range authoritativeScrub {
		wrong := cloneS7PIB427GitRequests(calls)
		wrong[0].env = append(wrong[0].env, name+"=selectively-reintroduced")
		if err := validateS7PIB427GitRequests(wrong, wantArgv); err == nil {
			t.Fatalf("Git request validator accepted reintroduced %s", name)
		}
	}
	for _, name := range s7PrepareGitOverflowIndexedNames() {
		for requestIndex := range calls {
			wrong := cloneS7PIB427GitRequests(calls)
			wrong[requestIndex].env = append(
				wrong[requestIndex].env,
				name+"=overflow-index-reintroduced",
			)
			if err := validateS7PIB427GitRequests(wrong, wantArgv); err == nil {
				t.Fatalf(
					"Git request validator accepted overflow %s in G%d",
					name,
					requestIndex+1,
				)
			}
		}
	}
	malformed := []string{
		"GIT_CONFIG_KEY_",
		"GIT_CONFIG_VALUE_",
		"GIT_CONFIG_KEY_-1",
		"GIT_CONFIG_VALUE_+1",
		"GIT_CONFIG_KEY_1x",
		"GIT_CONFIG_VALUE_１２",
		"GIT_CONFIG_COUNT_0",
	}
	malformedEnvironment := make([]string, 0, len(malformed)+2)
	for _, name := range malformed {
		malformedEnvironment = append(malformedEnvironment, name+"=preserved")
	}
	malformedEnvironment = append(malformedEnvironment, "LC_ALL=fr_FR", "LANG=fr_FR")
	wantMalformed := append([]string(nil), malformedEnvironment[:len(malformed)]...)
	wantMalformed = append(wantMalformed, "LC_ALL=C", "LANG=C")
	if got := prepareGitEnvironment(malformedEnvironment); !reflect.DeepEqual(got, wantMalformed) {
		t.Fatalf("malformed indexed variables = %#v, want preserved %#v", got, wantMalformed)
	}
}

func s7PrepareGitOverflowIndexedNames() []string {
	suffix := strings.Repeat("9", 96)
	return []string{
		"GIT_CONFIG_KEY_" + suffix,
		"GIT_CONFIG_VALUE_" + suffix,
	}
}

func s7PrepareGitExactScrubExamples() []string {
	return []string{
		"GIT_DIR",
		"GIT_WORK_TREE",
		"GIT_INDEX_FILE",
		"GIT_OBJECT_DIRECTORY",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES",
		"GIT_COMMON_DIR",
		"GIT_CEILING_DIRECTORIES",
		"GIT_DISCOVERY_ACROSS_FILESYSTEM",
		"GIT_PREFIX",
		"GIT_IMPLICIT_WORK_TREE",
		"GIT_SUPER_PREFIX",
		"GIT_CONFIG_COUNT",
		"GIT_CONFIG_KEY_0",
		"GIT_CONFIG_VALUE_0",
		"GIT_CONFIG_KEY_999",
		"GIT_CONFIG_VALUE_999",
	}
}

func s7PrepareGitAuthoritativeScrubExamples(t *testing.T) []string {
	t.Helper()
	prd, err := os.ReadFile(filepath.Join("..", "..", "docs", "prds", "PRD-prepare-intent-bundle.md"))
	if err != nil {
		t.Fatal(err)
	}
	document := string(prd)
	start := strings.Index(document, "#### The pinned environment scrub")
	end := strings.Index(document, "**Global and system ignore configuration remains intentionally available.**")
	if start < 0 || end <= start {
		t.Fatal("PRD §7.13 pinned environment subsection is missing")
	}
	names := map[string]bool{}
	pattern := regexp.MustCompile("`(GIT_[A-Z_]+(?:<n>)?)`")
	for _, match := range pattern.FindAllStringSubmatch(document[start:end], -1) {
		switch match[1] {
		case "GIT_CONFIG_KEY_<n>":
			names["GIT_CONFIG_KEY_0"] = true
			names["GIT_CONFIG_KEY_999"] = true
		case "GIT_CONFIG_VALUE_<n>":
			names["GIT_CONFIG_VALUE_0"] = true
			names["GIT_CONFIG_VALUE_999"] = true
		default:
			names[match[1]] = true
		}
	}
	derived := make([]string, 0, len(names))
	for name := range names {
		derived = append(derived, name)
	}
	sort.Strings(derived)
	expected := s7PrepareGitExactScrubExamples()
	sort.Strings(expected)
	if !reflect.DeepEqual(derived, expected) {
		t.Fatalf("derived §7.13 Git scrub = %q, want %q", derived, expected)
	}
	return derived
}

func validateS7PIB427GitRequests(calls []gitProcessRequest, wantArgv [][]string) error {
	if len(calls) != len(wantArgv) {
		return fmt.Errorf("Git process count = %d, want %d", len(calls), len(wantArgv))
	}
	for index, call := range calls {
		if call.repoRoot != "/workspace" {
			return fmt.Errorf("Git[%d] cwd = %q", index, call.repoRoot)
		}
		if !reflect.DeepEqual(call.args, wantArgv[index]) {
			return fmt.Errorf("Git[%d] argv = %q, want %q", index, call.args, wantArgv[index])
		}
		for _, arg := range call.args {
			if strings.HasPrefix(arg, "/") {
				return fmt.Errorf("Git[%d] argv contains an absolute lane: %q", index, call.args)
			}
		}
		environment := map[string]string{}
		for _, entry := range call.env {
			name, value, ok := strings.Cut(entry, "=")
			if !ok {
				continue
			}
			environment[name] = value
			if s7PrepareGitEnvironmentForbidden(name) {
				return fmt.Errorf("Git[%d] environment retained %s", index, name)
			}
		}
		if environment["LC_ALL"] != "C" || environment["LANG"] != "C" {
			return fmt.Errorf("Git[%d] locale = LC_ALL:%q LANG:%q", index, environment["LC_ALL"], environment["LANG"])
		}
	}
	return nil
}

func s7PrepareGitEnvironmentForbidden(name string) bool {
	for _, exact := range s7PrepareGitExactScrubExamples()[:13] {
		if name == exact {
			return true
		}
	}
	for _, prefix := range []string{"GIT_CONFIG_KEY_", "GIT_CONFIG_VALUE_"} {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		suffix := strings.TrimPrefix(name, prefix)
		if suffix == "" {
			return false
		}
		for _, character := range suffix {
			if character < '0' || character > '9' {
				return false
			}
		}
		return true
	}
	return false
}

func cloneS7PIB427GitRequests(calls []gitProcessRequest) []gitProcessRequest {
	clone := make([]gitProcessRequest, len(calls))
	for index, call := range calls {
		clone[index] = call
		clone[index].args = append([]string(nil), call.args...)
		clone[index].env = append([]string(nil), call.env...)
	}
	return clone
}

func TestPrepareGitEnvironmentScrubAndCompatibilityPolicy(t *testing.T) {
	source := []string{
		"HOME=/home/operator",
		"XDG_CONFIG_HOME=/home/operator/.config",
		"GIT_CONFIG_GLOBAL=/home/operator/.gitconfig",
		"GIT_CONFIG_SYSTEM=/etc/gitconfig",
		"GIT_DIR=/foreign",
		"GIT_WORK_TREE=/foreign-tree",
		"GIT_INDEX_FILE=/foreign-index",
		"GIT_COMMON_DIR=/foreign-common",
		"GIT_CEILING_DIRECTORIES=/",
		"GIT_OBJECT_DIRECTORY=/foreign-objects",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES=/foreign-alt",
		"GIT_DISCOVERY_ACROSS_FILESYSTEM=1",
		"GIT_NAMESPACE=other",
		"GIT_PREFIX=prefix",
		"GIT_IMPLICIT_WORK_TREE=1",
		"GIT_SUPER_PREFIX=super",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=core.excludesFile",
		"GIT_CONFIG_VALUE_0=/foreign-ignore",
		"GIT_CONFIG_KEY_999=alias.x",
		"GIT_CONFIG_VALUE_999=!echo",
		"LC_ALL=fr_FR",
		"LANG=fr_FR",
		"PATH=/bin",
	}
	got := prepareGitEnvironment(source)
	want := []string{
		"HOME=/home/operator",
		"XDG_CONFIG_HOME=/home/operator/.config",
		"GIT_CONFIG_GLOBAL=/home/operator/.gitconfig",
		"GIT_CONFIG_SYSTEM=/etc/gitconfig",
		"GIT_NAMESPACE=other",
		"PATH=/bin",
		"LC_ALL=C",
		"LANG=C",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("environment\n got: %#v\nwant: %#v", got, want)
	}

	previous := runGitProcess
	t.Cleanup(func() { runGitProcess = previous })
	runGitProcess = func(request gitProcessRequest) gitProcessResult {
		if request.env != nil {
			t.Fatalf("compatibility wrapper changed the inherited environment: %#v", request.env)
		}
		return gitProcessResult{}
	}
	if _, _, _, err := RunGitCompatibility("/workspace", "status"); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareGitLeadingColonDisarm(t *testing.T) {
	previous := runGitProcess
	t.Cleanup(func() { runGitProcess = previous })
	runGitProcess = func(request gitProcessRequest) gitProcessResult {
		want := []string{"check-ignore", "-q", "--no-index", "--", "./:lane"}
		if !reflect.DeepEqual(request.args, want) {
			t.Fatalf("argv = %#v, want %#v", request.args, want)
		}
		return gitProcessResult{}
	}
	ignored, err := IsIgnoredWithState("/workspace", GitWorktree, ":lane")
	if err != nil || !ignored {
		t.Fatalf("ignored = (%v, %v)", ignored, err)
	}
}
