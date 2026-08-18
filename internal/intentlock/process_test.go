//go:build (linux && !android) || (darwin && !ios)

package intentlock

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestAuthorityHelperProcess(t *testing.T) {
	if os.Getenv("TPATCH_INTENTLOCK_HELPER") != "1" {
		return
	}
	workspace := os.Getenv("TPATCH_INTENTLOCK_WORKSPACE")
	authority, err := Acquire(workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "acquire: %v\n", err)
		os.Exit(41)
	}
	if os.Getenv("TPATCH_INTENTLOCK_GC") == "1" {
		for range 16 {
			runtime.GC()
		}
	}
	runtime.KeepAlive(authority)
	fmt.Println("READY")
	if os.Getenv("TPATCH_INTENTLOCK_COMMANDS") == "1" {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			switch scanner.Text() {
			case "VALIDATE":
				err := authority.ValidateOriginalPath(false)
				if err == nil {
					fmt.Println("VALIDATION OK")
					continue
				}
				var typed *Error
				if !errors.As(err, &typed) {
					fmt.Fprintf(os.Stderr, "validate: %v\n", err)
					os.Exit(43)
				}
				fmt.Printf("VALIDATION %s\n", typed.Code)
			case "RELEASE":
				if err := authority.Release(); err != nil {
					fmt.Fprintf(os.Stderr, "release: %v\n", err)
					os.Exit(42)
				}
				fmt.Println("RELEASED")
				return
			default:
				fmt.Fprintf(os.Stderr, "unknown helper command: %q\n", scanner.Text())
				os.Exit(44)
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "command pipe: %v\n", err)
			os.Exit(45)
		}
		os.Exit(46)
	}
	if os.Getenv("TPATCH_INTENTLOCK_RELEASE") == "1" {
		_, _ = io.Copy(io.Discard, os.Stdin)
		if err := authority.Release(); err != nil {
			fmt.Fprintf(os.Stderr, "release: %v\n", err)
			os.Exit(42)
		}
		return
	}
	for {
		time.Sleep(time.Hour)
		runtime.KeepAlive(authority)
	}
}

func TestProcessDeathReleasesAuthorityWithoutArtifact(t *testing.T) {
	workspace := t.TempDir()
	before := directoryNames(t, workspace)
	command := startAuthorityHelper(t, workspace, false)
	if err := command.Process.Kill(); err != nil {
		t.Fatalf("kill helper: %v", err)
	}
	_ = command.Wait()

	authority, err := Acquire(workspace)
	if err != nil {
		t.Fatalf("Acquire after process death: %v", err)
	}
	if err := authority.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if after := directoryNames(t, workspace); !equalStrings(after, before) {
		t.Fatalf("process death left residue: before=%v after=%v", before, after)
	}
}

func TestForcedGCInHolderProcessDoesNotRelease(t *testing.T) {
	workspace := t.TempDir()
	command := startAuthorityHelper(t, workspace, true)
	defer func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	}()

	authority, err := Acquire(workspace)
	if authority != nil {
		_ = authority.Release()
		t.Fatal("forced GC released live process authority")
	}
	assertCode(t, err, CodeTransactionInProgress)
}

func TestRealProcessContentionAndExplicitRelease(t *testing.T) {
	workspace := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestAuthorityHelperProcess$")
	command.Env = append(os.Environ(),
		"TPATCH_INTENTLOCK_HELPER=1",
		"TPATCH_INTENTLOCK_RELEASE=1",
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
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "READY" {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("helper did not become ready: %q (%v)", scanner.Text(), scanner.Err())
	}

	contender, err := Acquire(workspace)
	if contender != nil {
		_ = contender.Release()
		t.Fatal("different conceptual slug unexpectedly acquired workspace authority")
	}
	assertCode(t, err, CodeTransactionInProgress)

	if err := stdin.Close(); err != nil {
		t.Fatalf("signal explicit release: %v", err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("helper release: %v", err)
	}
	reacquired, err := Acquire(workspace)
	if err != nil {
		t.Fatalf("Acquire after helper release: %v", err)
	}
	if err := reacquired.Release(); err != nil {
		t.Fatalf("Release reacquired authority: %v", err)
	}
}

func TestPIB413RenameRetainsInodeContentionAndInvalidatesOriginalAlias(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "workspace")
	moved := filepath.Join(parent, "moved")
	alias := filepath.Join(parent, "alias")
	if err := os.Mkdir(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(workspace, alias); err != nil {
		t.Fatal(err)
	}
	before := directoryNames(t, workspace)

	command := exec.Command(os.Args[0], "-test.run=^TestAuthorityHelperProcess$")
	command.Env = append(os.Environ(),
		"TPATCH_INTENTLOCK_HELPER=1",
		"TPATCH_INTENTLOCK_COMMANDS=1",
		"TPATCH_INTENTLOCK_WORKSPACE="+alias,
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
		if finished {
			return
		}
		_ = command.Process.Kill()
		_ = command.Wait()
	})
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "READY" {
		t.Fatalf("helper did not become ready: %q (%v)", scanner.Text(), scanner.Err())
	}

	if err := os.Rename(workspace, moved); err != nil {
		t.Fatal(err)
	}
	contender, err := Acquire(moved)
	if contender != nil {
		_ = contender.Release()
		t.Fatal("moved-path contender acquired the helper-held inode")
	}
	assertCode(t, err, CodeTransactionInProgress)

	if _, err := io.WriteString(stdin, "VALIDATE\n"); err != nil {
		t.Fatalf("request validation: %v", err)
	}
	wantValidation := "VALIDATION " + string(CodeWorkspaceRootChanged)
	if !scanner.Scan() || scanner.Text() != wantValidation {
		t.Fatalf("validation response = %q, want %q (%v)", scanner.Text(), wantValidation, scanner.Err())
	}

	if _, err := io.WriteString(stdin, "RELEASE\n"); err != nil {
		t.Fatalf("request release: %v", err)
	}
	if !scanner.Scan() || scanner.Text() != "RELEASED" {
		t.Fatalf("release response = %q (%v)", scanner.Text(), scanner.Err())
	}
	if err := stdin.Close(); err != nil {
		t.Fatalf("close command pipe: %v", err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("helper exit: %v", err)
	}
	finished = true

	reacquired, err := Acquire(moved)
	if err != nil {
		t.Fatalf("Acquire moved path after helper release: %v", err)
	}
	if err := reacquired.Release(); err != nil {
		t.Fatalf("Release moved path: %v", err)
	}
	if after := directoryNames(t, moved); !equalStrings(after, before) {
		t.Fatalf("PIB-413 flow left workspace residue: before=%v after=%v", before, after)
	}
}

func startAuthorityHelper(t *testing.T, workspace string, forceGC bool) *exec.Cmd {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestAuthorityHelperProcess$")
	command.Env = append(os.Environ(),
		"TPATCH_INTENTLOCK_HELPER=1",
		"TPATCH_INTENTLOCK_WORKSPACE="+workspace,
	)
	if forceGC {
		command.Env = append(command.Env, "TPATCH_INTENTLOCK_GC=1")
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "READY" {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("helper did not become ready: %q (%v)", scanner.Text(), scanner.Err())
	}
	return command
}
