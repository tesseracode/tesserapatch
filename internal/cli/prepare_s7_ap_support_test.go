package cli

import (
	"os"
	"testing"
)

var (
	s7APDryRunObservationMode              bool
	s7APDryRunAllowExternalFixtureMutation bool
	s7APDryRunGitBin                       string
	s7APExitThreeSnapshotMode              bool
	s7APExitThreeSnapshots                 map[string]string
)

func s7APRecordExitThreeSnapshot(root, before string, exit int) {
	if !s7APExitThreeSnapshotMode || exit != 3 || root == "" {
		return
	}
	if s7APExitThreeSnapshots == nil {
		s7APExitThreeSnapshots = map[string]string{}
	}
	s7APExitThreeSnapshots[root] = before
}

func s7APObservationSnapshot(t *testing.T, args []string) (string, string) {
	if !s7APExitThreeSnapshotMode {
		return "", ""
	}
	for i, arg := range args {
		if arg == "--path" && i+1 < len(args) {
			return args[i+1], snapshotTreeMetadata(t, "exit-3", args[i+1])
		}
	}
	return "", ""
}

func s7APAsDryRunPrepareArgs(args []string) []string {
	var root, slug string
	for i, arg := range args {
		if arg == "--path" && i+1 < len(args) {
			root = args[i+1]
		}
		if (arg == "list" || arg == "purge") && i+1 < len(args) {
			slug = args[i+1]
		}
	}

	return []string{
		"--path", root, "prepare", slug, "--regenerate", "--dry-run", "--json", "--quiet",
	}
}

func s7APObserveRefusalOnce(t *testing.T, code string) s6RuntimeObservation {
	t.Helper()
	if code != "provider-required-for-regenerate" {
		return s6ObserveRefusalOnce(t, code)
	}
	previous, existed := os.LookupEnv("XDG_CONFIG_HOME")
	if err := os.Setenv("XDG_CONFIG_HOME", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if existed {
			_ = os.Setenv("XDG_CONFIG_HOME", previous)
		} else {
			_ = os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()
	return s6ObserveRefusalOnce(t, code)
}
