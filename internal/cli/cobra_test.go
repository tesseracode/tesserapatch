package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func init() {
	// Prevent auto-detection from interfering with tests
	os.Setenv("TPATCH_NO_AUTO_DETECT", "1")
}

func runCmd(args ...string) (stdout, stderr string, code int) {
	var outBuf, errBuf bytes.Buffer
	root := buildRootCmd()
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err := root.Execute()
	if err != nil {
		return outBuf.String(), errBuf.String(), 1
	}
	return outBuf.String(), errBuf.String(), 0
}

func TestHelpReturns0(t *testing.T) {
	_, _, code := runCmd("--help")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestVersionReturns0(t *testing.T) {
	out, _, code := runCmd("--version")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "tpatch") {
		t.Fatalf("expected version in stdout, got %q", out)
	}
}

func TestUnknownCommandReturns1(t *testing.T) {
	_, _, code := runCmd("bogus")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

func TestCommandsNeedArgs(t *testing.T) {
	commands := []string{"analyze", "define", "explore", "implement", "apply", "record"}
	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			_, _, code := runCmd(cmd)
			if code != 1 {
				t.Fatalf("expected exit 1 for %q without args, got %d", cmd, code)
			}
		})
	}
}

func TestAnalyzeHeuristic(t *testing.T) {
	tmpDir := t.TempDir()

	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "Fix model translation")

	out, _, code := runCmd("analyze", "--path", tmpDir, "fix-model-translation")
	if code != 0 {
		t.Fatalf("analyze failed (code %d)", code)
	}
	if !strings.Contains(out, "heuristic mode") {
		t.Fatalf("expected heuristic mode output, got %q", out)
	}
}

func TestDefineHeuristic(t *testing.T) {
	tmpDir := t.TempDir()

	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "Fix model translation")
	runCmd("analyze", "--path", tmpDir, "fix-model-translation")

	out, _, code := runCmd("define", "--path", tmpDir, "fix-model-translation")
	if code != 0 {
		t.Fatalf("define failed (code %d)", code)
	}
	if !strings.Contains(out, "Spec generated") {
		t.Fatalf("expected spec generated output, got %q", out)
	}
}

func TestInitAndAddIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	out, _, code := runCmd("init", "--path", tmpDir)
	if code != 0 {
		t.Fatalf("init failed (code %d)", code)
	}
	if !strings.Contains(out, "Initialized") {
		t.Fatalf("expected Initialized in output, got %q", out)
	}

	out, _, code = runCmd("add", "--path", tmpDir, "Fix model translation bug")
	if code != 0 {
		t.Fatalf("add failed (code %d)", code)
	}
	if !strings.Contains(out, "fix-model-translation-bug") {
		t.Fatalf("expected slug in output, got %q", out)
	}

	out, _, code = runCmd("status", "--path", tmpDir)
	if code != 0 {
		t.Fatalf("status failed (code %d)", code)
	}
	if !strings.Contains(out, "fix-model-translation-bug") {
		t.Fatalf("expected feature in status output, got %q", out)
	}

	out, _, code = runCmd("config", "show", "--path", tmpDir)
	if code != 0 {
		t.Fatalf("config show failed (code %d)", code)
	}
	if !strings.Contains(out, "provider") {
		t.Fatalf("expected provider in config output, got %q", out)
	}

	out, _, code = runCmd("config", "set", "--path", tmpDir, "provider.base_url", "http://localhost:4141")
	if code != 0 {
		t.Fatalf("config set failed (code %d)", code)
	}

	out, _, code = runCmd("status", "--path", tmpDir, "--json")
	if code != 0 {
		t.Fatalf("status --json failed (code %d)", code)
	}
	if !strings.Contains(out, "fix-model-translation-bug") {
		t.Fatalf("expected feature in JSON output, got %q", out)
	}
}

func TestApplyModeFlagsAfterSlug(t *testing.T) {
	// This test verifies BUG-1 is fixed: --mode after slug should work
	tmpDir := t.TempDir()

	runCmd("init", "--path", tmpDir)
	runCmd("add", "--path", tmpDir, "Test feature")

	// With cobra, flags can appear after positional args
	out, _, code := runCmd("apply", "--path", tmpDir, "test-feature", "--mode", "started")
	if code != 0 {
		t.Fatalf("apply with --mode after slug failed (code %d)", code)
	}
	if !strings.Contains(out, "marked as implementing") {
		t.Fatalf("expected 'marked as implementing', got %q", out)
	}
}
