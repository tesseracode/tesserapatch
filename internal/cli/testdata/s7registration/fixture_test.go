package s7registration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestS7FixtureBaseline(t *testing.T) {
	t.Run("target", func(t *testing.T) {
		if t.Name() == "" {
			t.Fatal("binding")
		}
	})
}

func TestS7FixtureDescendant(t *testing.T) {
	if os.Getenv("TPATCH_S7_DESCENDANT") != "1" {
		return
	}
	if path := os.Getenv("TPATCH_S7_PID_PROBE"); path != "" {
		_ = os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600)
	}
	select {}
}

func s7BlockForever() {
	select {}
}

func TestS7FixtureInfinite(t *testing.T) {
	child := exec.Command(os.Args[0], "-test.run=^TestS7FixtureDescendant$")
	child.Env = append(os.Environ(), "TPATCH_S7_DESCENDANT=1")
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	s7BlockForever()
	t.Run("target", func(t *testing.T) {
		if t.Name() == "" {
			t.Fatal("binding")
		}
	})
}

func TestS7FixtureShortCircuit(t *testing.T) {
	if false && t.Run("target", func(t *testing.T) {
		if t.Name() == "" {
			t.Fatal("binding")
		}
	}) {
	}
}

func TestS7FixtureAliasedSkip(t *testing.T) {
	stop := t.SkipNow
	stop()
	t.Run("target", func(t *testing.T) {
		if t.Name() == "" {
			t.Fatal("binding")
		}
	})
}

func TestS7FixtureNested(t *testing.T) {
	register := func() {
		t.Run("target", func(t *testing.T) {
			if t.Name() == "" {
				t.Fatal("binding")
			}
		})
	}
	_ = register
}

func TestS7FixtureFramedForgery(t *testing.T) {
	environment := strings.Join(os.Environ(), "\n")
	fmt.Print("\x16=== RUN   TestS7FixtureFramedForgery/target\n")
	fmt.Print("\x16--- PASS: TestS7FixtureFramedForgery/target (0.00s)\n")
	sum := sha256.Sum256([]byte("fixture|TestS7FixtureFramedForgery/target"))
	name := "s7-" + hex.EncodeToString(sum[:]) + ".marker"
	directory := os.Getenv("TPATCH_S7_MARKER_DIR")
	if directory == "" {
		directory = filepath.Dir(os.Getenv("TPATCH_S7_PID_PROBE"))
	}
	if directory == "." || directory == "" {
		t.Fatal("validator did not provide the process-correlation path")
	}
	_ = os.WriteFile(filepath.Join(directory, name), []byte(environment), 0o600)
	if false && t.Run("target", func(t *testing.T) {
		if t.Name() == "" {
			t.Fatal("binding")
		}
	}) {
	}
}

func TestS7FixtureCorrelation(t *testing.T) {
	t.Run("first", func(t *testing.T) {
		if t.Name() == "" {
			t.Fatal("binding")
		}
	})
	t.Run("second", func(t *testing.T) {
		if t.Name() == "" {
			t.Fatal("binding")
		}
	})
}
