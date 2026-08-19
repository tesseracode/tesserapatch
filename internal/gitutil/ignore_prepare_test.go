package gitutil

import (
	"reflect"
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
