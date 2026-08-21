//go:build !windows && !freebsd && !openbsd && !netbsd && !dragonfly

package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type s7BSDGuardInputs struct {
	bsd      []byte
	shared   []byte
	publish  []byte
	workflow []byte
}

func TestS7BSDPlatformPredicateSeamRuntime(t *testing.T) {
	t.Log("native BSD execution is unavailable in CI; this runs the identical production predicate branch while the BSD test binary is cross-compiled")

	t.Run("PIB-409", func(t *testing.T) {
		previous := prepareMutationAuthoritySupported
		prepareMutationAuthoritySupported = func() bool { return false }
		t.Cleanup(func() { prepareMutationAuthoritySupported = previous })
		s7TestPlatformMutationRefusal(t)
	})
	t.Run("PIB-417", s7TestPlatformCheckCompatibility)
}

func TestS7BSDPlatformSourceGuardSensitivity(t *testing.T) {
	inputs := loadS7BSDGuardInputs(t)
	t.Run("baseline", func(t *testing.T) {
		if err := validateS7BSDPlatformSources(inputs); err != nil {
			t.Fatalf("baseline: %v", err)
		}
	})
	for _, fixture := range []struct {
		name   string
		mutate func(s7BSDGuardInputs) s7BSDGuardInputs
	}{
		{
			name: "missing-openbsd-build-lane",
			mutate: func(in s7BSDGuardInputs) s7BSDGuardInputs {
				in.bsd = bytes.Replace(in.bsd, []byte(" || openbsd"), nil, 1)
				return in
			},
		},
		{
			name: "computed-ledger-leaf",
			mutate: func(in s7BSDGuardInputs) s7BSDGuardInputs {
				in.bsd = bytes.Replace(in.bsd, []byte(`t.Run("PIB-417"`), []byte(`t.Run(rowID`), 1)
				return in
			},
		},
		{
			name: "predicate-bypassed",
			mutate: func(in s7BSDGuardInputs) s7BSDGuardInputs {
				in.publish = bytes.Replace(in.publish, []byte("!prepareMutationAuthoritySupported()"), []byte("!intentlock.AuthoritySupported"), 1)
				return in
			},
		},
		{
			name: "json-golden-omitted",
			mutate: func(in s7BSDGuardInputs) s7BSDGuardInputs {
				in.shared = bytes.Replace(in.shared, []byte(`"check-ready-json.txt"`), []byte(`"check-ready-human.txt"`), 1)
				return in
			},
		},
		{
			name: "workflow-freebsd-cross-compile-omitted",
			mutate: func(in s7BSDGuardInputs) s7BSDGuardInputs {
				in.workflow = bytes.Replace(in.workflow, []byte("freebsd openbsd netbsd dragonfly"), []byte("openbsd netbsd dragonfly"), 1)
				return in
			},
		},
		{
			name: "workflow-openbsd-cross-compile-omitted",
			mutate: func(in s7BSDGuardInputs) s7BSDGuardInputs {
				in.workflow = bytes.Replace(in.workflow, []byte("freebsd openbsd netbsd dragonfly"), []byte("freebsd netbsd dragonfly"), 1)
				return in
			},
		},
		{
			name: "workflow-netbsd-cross-compile-omitted",
			mutate: func(in s7BSDGuardInputs) s7BSDGuardInputs {
				in.workflow = bytes.Replace(in.workflow, []byte("freebsd openbsd netbsd dragonfly"), []byte("freebsd openbsd dragonfly"), 1)
				return in
			},
		},
		{
			name: "workflow-dragonfly-cross-compile-omitted",
			mutate: func(in s7BSDGuardInputs) s7BSDGuardInputs {
				in.workflow = bytes.Replace(in.workflow, []byte("freebsd openbsd netbsd dragonfly"), []byte("freebsd openbsd netbsd"), 1)
				return in
			},
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			if err := validateS7BSDPlatformSources(fixture.mutate(inputs)); err == nil {
				t.Fatal("wrong-input fixture passed the BSD platform guard")
			}
		})
	}
}

func loadS7BSDGuardInputs(t *testing.T) s7BSDGuardInputs {
	t.Helper()
	root := avpRepoRoot(t)
	read := func(path string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	return s7BSDGuardInputs{
		bsd:      read("internal/cli/prepare_s7_bsd_test.go"),
		shared:   read("internal/cli/prepare_s7_platform_runtime_shared_test.go"),
		publish:  read("internal/cli/prepare_publish.go"),
		workflow: read(".github/workflows/ci.yml"),
	}
}

func validateS7BSDPlatformSources(in s7BSDGuardInputs) error {
	if !bytes.Contains(in.bsd, []byte("//go:build freebsd || openbsd || netbsd || dragonfly")) {
		return errors.New("BSD build constraint does not cover the four supported BSD test lanes")
	}
	for _, literal := range []string{
		`func TestS7BSDPlatformRows(`,
		`t.Run("PIB-409", s7TestPlatformMutationRefusal)`,
		`t.Run("PIB-417", s7TestPlatformCheckCompatibility)`,
		"s7BSDNativeLimitation",
	} {
		if !bytes.Contains(in.bsd, []byte(literal)) {
			return errors.New("BSD native test is missing literal " + literal)
		}
	}
	for _, literal := range []string{
		`"check-ready-human.txt"`,
		`"check-ready-json.txt"`,
		"runPrepare(",
		"readTree(",
		"prepare-unsupported-platform",
	} {
		if bytes.Count(in.shared, []byte(literal)) != 1 && strings.HasPrefix(literal, `"check-ready-`) {
			return errors.New("platform runtime helper must select exactly one " + literal)
		}
		if !strings.HasPrefix(literal, `"check-ready-`) && !bytes.Contains(in.shared, []byte(literal)) {
			return errors.New("platform runtime helper is missing " + literal)
		}
	}
	if bytes.Count(in.publish, []byte("if !prepareMutationAuthoritySupported()")) != 2 {
		return errors.New("both mutation routes must use the platform predicate seam")
	}
	for _, function := range []string{"func runPreparePublish(", "func runPrepareAbandon("} {
		start := bytes.Index(in.publish, []byte(function))
		if start < 0 {
			return errors.New("missing production function " + function)
		}
		body := in.publish[start:]
		if next := bytes.Index(body[len(function):], []byte("\nfunc ")); next >= 0 {
			body = body[:len(function)+next]
		}
		predicate := bytes.Index(body, []byte("if !prepareMutationAuthoritySupported()"))
		acquire := bytes.Index(body, []byte("prepareAcquireAuthority(repoRoot)"))
		if predicate < 0 || acquire < 0 || predicate > acquire {
			return errors.New(function + " does not refuse before authority acquisition")
		}
	}
	for _, literal := range []string{
		"for goos in freebsd openbsd netbsd dragonfly; do",
		`binary="${RUNNER_TEMP}/cli-${goos}.test"`,
		`GOOS="${goos}" GOARCH=amd64 CGO_ENABLED=0 go test -c -o "$binary" ./internal/cli`,
		`test -s "$binary"`,
		`rm -f "$binary"`,
		"TestS7WindowsPlatformRows",
		"TestS7WindowsPlatformRows/PIB-409",
		"TestS7WindowsPlatformRows/PIB-417/check-ready-human.txt",
		"TestS7WindowsPlatformRows/PIB-417/check-ready-json.txt",
	} {
		if !bytes.Contains(in.workflow, []byte(literal)) {
			return errors.New("blocking workflow is missing " + literal)
		}
	}
	for _, line := range strings.Split(string(in.workflow), "\n") {
		if strings.TrimSpace(line) == `"$binary"` ||
			strings.TrimSpace(line) == `"${binary}"` {
			return errors.New("blocking workflow attempts to execute a foreign BSD test binary")
		}
	}
	return nil
}
