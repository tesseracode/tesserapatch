package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Pre-change routing goldens — AVP-071, AVP-072, AVP-136 and AVP-137.
//
// These fixtures are recorded from the WAVE_BASE binary (the commit that
// preceded this wave), not from a no-op run of the current binary. The
// distinction is the whole point of the rows: a before/after comparison
// across the current binary cannot detect a routing change that this wave
// introduced, because both sides would carry it.
//
// Recording procedure (see testdata/routing-goldens/README.md for the
// recorded provenance):
//
//	git worktree add --detach <dir> <WAVE_BASE>
//	(cd <dir> && go build -o <bin> ./cmd/tpatch)
//	TPATCH_ROUTING_GOLDEN_BIN=<bin> TPATCH_RECORD_ROUTING_GOLDENS=1 \
//	  go test ./internal/cli -run TestAVPRoutingGoldens
//
// A normal run builds the CURRENT binary and compares byte-for-byte.

const routingGoldenDir = "testdata/routing-goldens"

func TestAVPRoutingGoldens(t *testing.T) {
	binary := os.Getenv("TPATCH_ROUTING_GOLDEN_BIN")
	if binary == "" {
		binary = buildCurrentBinary(t)
	}
	captured := captureRoutingSurfaces(t, binary)

	if os.Getenv("TPATCH_RECORD_ROUTING_GOLDENS") == "1" {
		recordRoutingGoldens(t, captured)
		t.Log("recorded routing goldens; re-run without TPATCH_RECORD_ROUTING_GOLDENS to compare")
		return
	}

	names := make([]string, 0, len(captured))
	for name := range captured {
		names = append(names, name)
	}
	sort.Strings(names)

	// AVP-136 covers the four `next` routing populations in both formats;
	// AVP-137 the `cycle --skip-execute` transcript and final state; AVP-071
	// and AVP-072 are the compatibility rows those goldens satisfy.
	for _, id := range []string{"AVP-071", "AVP-072", "AVP-136", "AVP-137"} {
		t.Run(id, func(t *testing.T) {
			for _, name := range names {
				if !strings.HasPrefix(name, routingRowPrefix(id)) {
					continue
				}
				want, err := os.ReadFile(filepath.Join(routingGoldenDir, name))
				if err != nil {
					t.Fatalf("missing golden %s: %v", name, err)
				}
				if !bytes.Equal(want, []byte(captured[name])) {
					t.Fatalf("%s drifted from the pre-change golden\n--- golden ---\n%s\n--- current ---\n%s",
						name, want, captured[name])
				}
			}
		})
	}

	t.Run("AVP-010-bounded-delta", func(t *testing.T) {
		// The only pre-change surface this wave may alter is the `--mode`
		// flag description, and only by appending the pointer sentence.
		before, err := os.ReadFile(filepath.Join(routingGoldenDir, "changed-apply-help.txt"))
		if err != nil {
			t.Fatal(err)
		}
		const pointer = " For read-only intent inspection, use tpatch prepare <slug> --check."
		want := strings.Replace(string(before),
			"done (default \"auto\")", "done."+pointer+" (default \"auto\")", 1)
		if want == string(before) {
			t.Fatal("the pre-change golden no longer contains the --mode description")
		}
		if want != captured["changed-apply-help.txt"] {
			t.Fatalf("apply --help changed by more than the documented pointer\n--- want ---\n%s\n--- got ---\n%s",
				want, captured["changed-apply-help.txt"])
		}
	})

	t.Run("every-golden-is-covered", func(t *testing.T) {
		entries, err := os.ReadDir(routingGoldenDir)
		if err != nil {
			t.Fatal(err)
		}
		recorded := 0
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
				continue
			}
			recorded++
			if _, ok := captured[entry.Name()]; !ok {
				t.Errorf("golden %s is no longer produced by the capture", entry.Name())
			}
		}
		if recorded != len(captured) {
			t.Fatalf("%d goldens recorded, %d captured", recorded, len(captured))
		}
		if recorded < 10 {
			t.Fatalf("only %d goldens; the four routing populations in two formats need at least 10", recorded)
		}
	})
}

func routingRowPrefix(id string) string {
	switch id {
	case "AVP-071", "AVP-136":
		return "next-"
	case "AVP-072", "AVP-137":
		return "cycle-"
	default:
		return ""
	}
}

func buildCurrentBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "tpatch-current")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/tpatch")
	cmd.Dir = avpRepoRoot(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build current binary: %v\n%s", err, output)
	}
	return binary
}

// hermeticRoutingEnv builds a deliberately minimal environment.
//
// Inheriting os.Environ() is not safe here: the goldens must be a property of
// the binary, not of the machine or of whatever another test in this package
// happened to os.Setenv. A leaked provider credential turns the heuristic
// `cycle` transcript into a network transcript, which is exactly the drift
// this row is supposed to detect in the CLI rather than in the environment.
func hermeticRoutingEnv(t *testing.T) []string {
	t.Helper()
	home := t.TempDir()
	env := []string{
		"HOME=" + home,
		"USERPROFILE=" + home,
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		"GIT_CONFIG_GLOBAL=" + filepath.Join(home, "gitconfig-absent"),
		"GIT_CONFIG_SYSTEM=" + filepath.Join(home, "gitconfig-absent"),
		"GIT_TERMINAL_PROMPT=0",
	}
	for _, name := range []string{"PATH", "SystemRoot", "ComSpec", "TMPDIR", "TEMP", "TMP", "windir"} {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	return env
}

func runRoutingBinary(t *testing.T, binary, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	exit := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exit = exitErr.ExitCode()
		} else {
			t.Fatalf("run %v: %v", args, err)
		}
	}
	normalized := strings.ReplaceAll(string(output), dir, "<workspace>")
	return fmt.Sprintf("$ tpatch %s\nexit %d\n%s", strings.Join(args, " "), exit, normalized)
}

// captureRoutingSurfaces walks the four `next` routing populations in both
// formats, the `apply --mode prepare` surface that shares the `prepare` word,
// and a `cycle --skip-execute` transcript over a heuristic workspace.
func captureRoutingSurfaces(t *testing.T, binary string) map[string]string {
	t.Helper()
	captured := map[string]string{}

	env := hermeticRoutingEnv(t)
	dir := t.TempDir()
	runRoutingBinary(t, binary, dir, env, "init")
	runRoutingBinary(t, binary, dir, env, "add", "routing demo")
	slug := "routing-demo"
	feature := filepath.Join(dir, ".tpatch", "features", slug)

	populations := []struct {
		name    string
		prepare func()
	}{
		{"requested", func() {}},
		{"analyzed", func() {
			mustWrite(t, filepath.Join(feature, "analysis.md"), "analysis\n")
			runRoutingBinary(t, binary, dir, env, "analyze", slug, "--manual")
		}},
		{"defined-pre-explore", func() {
			mustWrite(t, filepath.Join(feature, "spec.md"), "spec\n")
			runRoutingBinary(t, binary, dir, env, "define", slug, "--manual")
		}},
		{"defined-post-explore", func() {
			mustWrite(t, filepath.Join(feature, "exploration.md"), "exploration\n")
			runRoutingBinary(t, binary, dir, env, "explore", slug, "--manual")
		}},
	}
	for _, population := range populations {
		population.prepare()
		captured["next-"+population.name+"-text.txt"] = runRoutingBinary(t, binary, dir, env, "next", slug)
		captured["next-"+population.name+"-harness-json.txt"] =
			runRoutingBinary(t, binary, dir, env, "next", slug, "--format", "harness-json")
	}
	captured["next-apply-mode-prepare.txt"] = runRoutingBinary(t, binary, dir, env, "apply", slug, "--mode", "prepare")
	// `apply --help` is the ONE surface this wave deliberately changes
	// (AVP-010's reciprocal collision pointer). It is recorded under a
	// `changed-` prefix so it is excluded from the byte-identity rows and
	// asserted separately as a bounded, documented delta.
	captured["changed-apply-help.txt"] = runRoutingBinary(t, binary, dir, env, "apply", "--help")

	cycleDir := t.TempDir()
	runRoutingBinary(t, binary, cycleDir, env, "init")
	runRoutingBinary(t, binary, cycleDir, env, "add", "cycle demo")
	captured["cycle-skip-execute-transcript.txt"] =
		runRoutingBinary(t, binary, cycleDir, env, "cycle", "cycle-demo", "--skip-execute")
	captured["cycle-final-state.txt"] = readStateGolden(t, cycleDir, "cycle-demo")

	return captured
}

func readStateGolden(t *testing.T, dir, slug string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tpatch", "features", slug, "status.json"))
	if err != nil {
		t.Fatalf("read status.json: %v", err)
	}
	var state string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, `"state"`) {
			state = strings.TrimSpace(line)
			break
		}
	}
	return "final " + state + "\n"
}

func recordRoutingGoldens(t *testing.T, captured map[string]string) {
	t.Helper()
	if err := os.MkdirAll(routingGoldenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range captured {
		if err := os.WriteFile(filepath.Join(routingGoldenDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
