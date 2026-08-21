//go:build (linux && !android) || (darwin && !ios)

package intentlock

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestS7PIB433RealHolderSurvivesGCUntilExplicitRelease(t *testing.T) {
	workspace := t.TempDir()
	before := directoryNames(t, workspace)
	command := exec.Command(os.Args[0], "-test.run=^TestAuthorityHelperProcess$")
	command.Env = append(os.Environ(),
		"TPATCH_INTENTLOCK_HELPER=1",
		"TPATCH_INTENTLOCK_GC=1",
		"TPATCH_INTENTLOCK_COMMANDS=1",
		"TPATCH_INTENTLOCK_WORKSPACE="+workspace,
	)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	finished := false
	t.Cleanup(func() {
		if !finished {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	})
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "READY" {
		t.Fatalf("GC holder readiness = %q err=%v", scanner.Text(), scanner.Err())
	}
	for range 8 {
		runtime.GC()
	}
	contender, err := Acquire(workspace)
	if contender != nil {
		_ = contender.Release()
		t.Fatal("forced GC released the live process authority")
	}
	assertCode(t, err, CodeTransactionInProgress)
	if _, err := fmt.Fprintln(stdin, "RELEASE"); err != nil {
		t.Fatal(err)
	}
	if !scanner.Scan() || scanner.Text() != "RELEASED" {
		t.Fatalf("explicit release response = %q err=%v", scanner.Text(), scanner.Err())
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
	finished = true
	reacquired, err := Acquire(workspace)
	if err != nil {
		t.Fatalf("explicit release did not permit reacquire: %v", err)
	}
	if err := reacquired.Release(); err != nil {
		t.Fatal(err)
	}
	if after := directoryNames(t, workspace); !equalStrings(after, before) {
		t.Fatalf("GC/release left authority residue: before=%v after=%v", before, after)
	}
}

func TestS7PIB434AuthorityLifetimeGuardAndWrongInput(t *testing.T) {
	sources := s7AuthorityProductionSources(t)
	if err := validateS7PIB434AuthorityLifetime(sources); err != nil {
		t.Fatal(err)
	}
	wrong := make(map[string]string, len(sources)+1)
	for name, source := range sources {
		wrong[name] = source
	}
	wrong["pib434-wrong.go"] = `package intentlock
import "runtime"
func wrong(value any) { runtime.SetFinalizer(value, func(any) {}) }
`
	if err := validateS7PIB434AuthorityLifetime(wrong); err == nil ||
		!strings.Contains(err.Error(), "finalizer") {
		t.Fatalf("PIB-434 same validator accepted finalizer release: %v", err)
	}
}

func validateS7PIB434AuthorityLifetime(sources map[string]string) error {
	if err := validateS7PIB398AuthoritySources(sources); err != nil {
		return err
	}
	authority := sources["authority.go"]
	if !strings.Contains(authority, "directory    *os.File") ||
		!strings.Contains(authority, "runtime.KeepAlive(a)") {
		return fmt.Errorf("PIB-434 authority lost its strong directory lifetime")
	}
	for name, source := range sources {
		if strings.Contains(source, "runtime.SetFinalizer") {
			return fmt.Errorf("PIB-434 finalizer release found in %s", name)
		}
	}
	return nil
}
