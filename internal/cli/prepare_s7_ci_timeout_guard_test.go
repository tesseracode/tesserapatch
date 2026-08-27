package cli

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

type s7CITimeoutStep struct {
	job             string
	name            string
	condition       string
	continueOnError string
	shell           string
	environment     map[string]string
	keys            map[string]int
	run             string
	runStyle        string
}

const (
	s7CIARPartitionPattern             = `^(TestS7AR.*|TestS7ObservedARRegistrationAuthority)$`
	s7CIARLegacyPattern                = `^TestS7ARRev(11|12|13|14|15).*$`
	s7CIARLegacyMidPattern             = `^TestS7ARRev(16|17|18).*$`
	s7CIARLegacyLatePattern            = `^TestS7ARRev(19|20).*$`
	s7CIARCurrentPattern               = `^TestS7ARRev(21|23|24).*$`
	s7CIARCurrentLatePattern           = `^TestS7ARRev(25|26|27).*$`
	s7CIARCorePattern                  = `^TestS7AR(ExitSixRouteGuard|PrepareGrammarGuard|DivergenceContracts|AbandonContracts|ArchiveControlContracts|CoverageLedger|CoverageLedgerRejectsEmptyTarget)$`
	s7CIARAbandonPattern               = `^TestS7ARAbandonGateTableGuard$`
	s7CIARPurgePattern                 = `^TestS7ARPurgeProgressGuard$`
	s7CIARPermanentPattern             = `^TestS7ARPermanentBlockClaimsGuard$`
	s7CIARObserverPattern              = `^TestS7ObservedARRegistrationAuthority$`
	s7CINonWindowsFullCommand          = `go test ./... -count=1 -timeout 40m -skip '` + s7CIARPartitionPattern + `'`
	s7CINonWindowsARLegacyCommand      = `go test ./internal/cli -count=1 -timeout 40m -run '` + s7CIARLegacyPattern + `'`
	s7CINonWindowsARLegacyMidCommand   = `go test ./internal/cli -count=1 -timeout 40m -run '` + s7CIARLegacyMidPattern + `'`
	s7CINonWindowsARLegacyLateCommand  = `go test ./internal/cli -count=1 -timeout 40m -run '` + s7CIARLegacyLatePattern + `'`
	s7CINonWindowsARCurrentCommand     = `go test ./internal/cli -count=1 -timeout 40m -run '` + s7CIARCurrentPattern + `'`
	s7CINonWindowsARCurrentLateCommand = `go test ./internal/cli -count=1 -timeout 40m -run '` + s7CIARCurrentLatePattern + `'`
	s7CINonWindowsARCoreCommand        = `go test ./internal/cli -count=1 -timeout 40m -run '` + s7CIARCorePattern + `'`
	s7CINonWindowsARAbandonCommand     = `go test ./internal/cli -count=1 -timeout 40m -run '` + s7CIARAbandonPattern + `'`
	s7CINonWindowsARPurgeCommand       = `go test ./internal/cli -count=1 -timeout 40m -run '` + s7CIARPurgePattern + `'`
	s7CINonWindowsARPermanentCommand   = `go test ./internal/cli -count=1 -timeout 40m -run '` + s7CIARPermanentPattern + `'`
	s7CINonWindowsARObserverCommand    = `go test ./internal/cli -count=1 -timeout 40m -run '` + s7CIARObserverPattern + `'`
	s7CINonWindowsTestScript           = "set -euo pipefail\n" +
		s7CINonWindowsFullCommand + "\n" +
		s7CINonWindowsARLegacyCommand + "\n" +
		s7CINonWindowsARLegacyMidCommand + "\n" +
		s7CINonWindowsARLegacyLateCommand + "\n" +
		s7CINonWindowsARCurrentCommand + "\n" +
		s7CINonWindowsARCurrentLateCommand + "\n" +
		s7CINonWindowsARCoreCommand + "\n" +
		s7CINonWindowsARAbandonCommand + "\n" +
		s7CINonWindowsARPurgeCommand + "\n" +
		s7CINonWindowsARPermanentCommand + "\n" +
		s7CINonWindowsARObserverCommand
	s7CIWindowsFullSuiteCommand = `go test ./... -count=1 -timeout 20m`
)

func TestS7CIFullSuiteTimeoutGuard(t *testing.T) {
	workflowPath := filepath.Join(avpRepoRoot(t), ".github", "workflows", "ci.yml")
	workflowBytes, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(workflowBytes)
	if err := validateS7CIFullSuiteTimeouts(workflow); err != nil {
		t.Fatal(err)
	}

	nonWindows := "      - name: Test\n" +
		"        if: runner.os != 'Windows'\n" +
		"        shell: bash\n" +
		"        env:\n" +
		"          BASH_ENV: /dev/null\n" +
		"          GOFLAGS: \"\"\n" +
		"          GOENV: \"off\"\n" +
		"        run: |\n" +
		"          " + strings.ReplaceAll(
		s7CINonWindowsTestScript, "\n", "\n          ",
	) + "\n"
	windows := `      - name: "Test (Windows full suite — allowed to fail, owned by GH #17)"
        if: runner.os == 'Windows'
        continue-on-error: true
        shell: bash
        run: ` + s7CIWindowsFullSuiteCommand + `
`
	swapTimeouts := strings.Replace(workflow, nonWindows, "__S7_NON_WINDOWS_FULL_SUITE__", 1)
	swapTimeouts = strings.Replace(
		swapTimeouts, windows, strings.ReplaceAll(windows, "20m", "40m"), 1,
	)
	swapTimeouts = strings.Replace(
		swapTimeouts, "__S7_NON_WINDOWS_FULL_SUITE__",
		strings.ReplaceAll(nonWindows, "40m", "20m"), 1,
	)
	withAdditionalStep := func(step string) string {
		return strings.Replace(workflow, nonWindows, nonWindows+step, 1)
	}
	releaseSteps := strings.LastIndex(workflow, "    steps:\n")
	if releaseSteps < 0 {
		t.Fatal("release job steps anchor not found")
	}
	withReleaseStep := func(step string) string {
		insert := releaseSteps + len("    steps:\n")
		return workflow[:insert] + step + workflow[insert:]
	}
	scalarStepDecoy := `      - name: S7 inert scalar decoy
        if: false
        uses: actions/checkout@v4
        with:
          payload: |
            steps:
              - name: Test
                if: runner.os != 'Windows'
                shell: bash
                env:
                  BASH_ENV: /dev/null
                  GOFLAGS: ""
                  GOENV: "off"
                run: |
                  ` + strings.ReplaceAll(
		s7CINonWindowsTestScript, "\n", "\n                  ",
	) + "\n"
	wideBareNonWindows := "      -\n" +
		"          name: Test\n" +
		"          if: false\n" +
		"          shell: bash\n" +
		"          env:\n" +
		"            BASH_ENV: /dev/null\n" +
		"            GOFLAGS: \"\"\n" +
		"            GOENV: \"off\"\n" +
		"          run: |\n" +
		"            " + strings.ReplaceAll(
		s7CINonWindowsTestScript, "\n", "\n            ",
	) + "\n" +
		"      -\n" +
		"          if: runner.os != 'Windows'\n"

	for _, fixture := range []struct {
		name     string
		mutation string
	}{
		{
			name: "lowered-timeout",
			mutation: strings.Replace(
				workflow,
				s7CINonWindowsFullCommand,
				strings.Replace(s7CINonWindowsFullCommand, "40m", "20m", 1),
				1,
			),
		},
		{
			name: "non-windows-shell-loses-fail-fast",
			mutation: strings.Replace(
				workflow,
				"if: runner.os != 'Windows'\n        shell: bash\n        env:\n          BASH_ENV: /dev/null\n          GOFLAGS: \"\"\n          GOENV: \"off\"\n        run: |",
				"if: runner.os != 'Windows'\n        shell: bash {0}\n        env:\n          BASH_ENV: /dev/null\n          GOFLAGS: \"\"\n          GOENV: \"off\"\n        run: |",
				1,
			),
		},
		{
			name: "non-windows-bash-env-removed",
			mutation: strings.Replace(
				workflow,
				"          BASH_ENV: /dev/null\n",
				"",
				1,
			),
		},
		{
			name: "non-windows-bash-env-inherited",
			mutation: strings.Replace(
				workflow,
				"BASH_ENV: /dev/null",
				"BASH_ENV: /tmp/inherited",
				1,
			),
		},
		{
			name: "non-windows-fail-fast-reset-removed",
			mutation: strings.Replace(
				workflow,
				"          set -euo pipefail\n",
				"",
				1,
			),
		},
		{
			name: "non-windows-goflags-removed",
			mutation: strings.Replace(
				workflow,
				"          GOFLAGS: \"\"\n",
				"",
				1,
			),
		},
		{
			name: "non-windows-goflags-list-only",
			mutation: strings.Replace(
				workflow,
				"GOFLAGS: \"\"",
				"GOFLAGS: -list=.",
				1,
			),
		},
		{
			name: "non-windows-goenv-removed",
			mutation: strings.Replace(
				workflow,
				"          GOENV: \"off\"\n",
				"",
				1,
			),
		},
		{
			name: "non-windows-goenv-inherited",
			mutation: strings.Replace(
				workflow,
				"GOENV: \"off\"",
				"GOENV: /tmp/inherited-goenv",
				1,
			),
		},
		{
			name: "non-windows-run-folded",
			mutation: strings.Replace(
				workflow,
				"          GOENV: \"off\"\n        run: |\n",
				"          GOENV: \"off\"\n        run: >\n",
				1,
			),
		},
		{
			name: "test-matrix-windows-only",
			mutation: strings.Replace(
				workflow,
				"os: [ubuntu-latest, macos-latest, windows-latest]",
				"os: [windows-latest]",
				1,
			),
		},
		{
			name: "test-runner-pinned-to-windows",
			mutation: strings.Replace(
				workflow,
				"runs-on: ${{ matrix.os }}",
				"runs-on: windows-latest",
				1,
			),
		},
		{
			name: "test-matrix-excludes-non-windows",
			mutation: strings.Replace(
				workflow,
				"        os: [ubuntu-latest, macos-latest, windows-latest]\n",
				"        os: [ubuntu-latest, macos-latest, windows-latest]\n"+
					"        exclude:\n"+
					"          - os: ubuntu-latest\n"+
					"          - os: macos-latest\n",
				1,
			),
		},
		{
			name: "test-job-disabled",
			mutation: strings.Replace(
				workflow,
				"  test:\n    name:",
				"  test:\n    if: false\n    name:",
				1,
			),
		},
		{
			name: "test-job-demoted",
			mutation: strings.Replace(
				workflow,
				"  test:\n    name:",
				"  test:\n    continue-on-error: true\n    name:",
				1,
			),
		},
		{
			name: "test-job-disabled-after-steps",
			mutation: strings.Replace(
				workflow,
				"\n  release:\n",
				"\n    if: false\n  release:\n",
				1,
			),
		},
		{
			name: "test-job-demoted-after-steps",
			mutation: strings.Replace(
				workflow,
				"\n  release:\n",
				"\n    continue-on-error: true\n  release:\n",
				1,
			),
		},
		{
			name: "inert-block-scalar-step-decoy",
			mutation: strings.Replace(
				workflow,
				nonWindows,
				scalarStepDecoy+strings.Replace(
					nonWindows,
					"if: runner.os != 'Windows'",
					"if: runner.os != 'Windows' && false",
					1,
				),
				1,
			),
		},
		{
			name:     "full-suite-step-removed",
			mutation: strings.Replace(workflow, nonWindows, "", 1),
		},
		{
			name: "default-timeout",
			mutation: strings.Replace(
				workflow,
				s7CINonWindowsFullCommand,
				strings.Replace(s7CINonWindowsFullCommand, " -timeout 40m", "", 1),
				1,
			),
		},
		{
			name: "windows-suite-skipped-by-or",
			mutation: strings.Replace(
				workflow,
				s7CIWindowsFullSuiteCommand,
				"true || "+s7CIWindowsFullSuiteCommand,
				1,
			),
		},
		{
			name: "windows-suite-failure-masked",
			mutation: strings.Replace(
				workflow,
				s7CIWindowsFullSuiteCommand,
				s7CIWindowsFullSuiteCommand+" || true",
				1,
			),
		},
		{
			name: "unbounded-timeout",
			mutation: strings.Replace(
				workflow,
				s7CINonWindowsFullCommand,
				strings.Replace(s7CINonWindowsFullCommand, "40m", "0", 1),
				1,
			),
		},
		{
			name: "duplicate-timeout",
			mutation: strings.Replace(
				workflow,
				s7CINonWindowsFullCommand,
				strings.Replace(
					s7CINonWindowsFullCommand,
					"-timeout 40m",
					"-timeout 40m -timeout=40m",
					1,
				),
				1,
			),
		},
		{
			name: "equals-timeout-noncanonical",
			mutation: strings.Replace(
				workflow,
				s7CINonWindowsFullCommand,
				strings.Replace(s7CINonWindowsFullCommand, "-timeout 40m", "-timeout=40m", 1),
				1,
			),
		},
		{
			name: "ar-partition-removed",
			mutation: strings.Replace(
				workflow, "\n          "+s7CINonWindowsARLegacyCommand, "", 1,
			),
		},
		{
			name: "ar-partition-timeout-lowered",
			mutation: strings.Replace(
				workflow,
				s7CINonWindowsARLegacyCommand,
				strings.Replace(s7CINonWindowsARLegacyCommand, "40m", "20m", 1),
				1,
			),
		},
		{
			name: "ar-partition-run-narrowed",
			mutation: strings.Replace(
				workflow,
				s7CINonWindowsARLegacyCommand,
				strings.Replace(
					s7CINonWindowsARLegacyCommand,
					s7CIARLegacyPattern,
					`^TestS7ARRev11ReviewerRepros$`,
					1,
				),
				1,
			),
		},
		{
			name: "full-suite-skip-narrowed",
			mutation: strings.Replace(
				workflow,
				s7CINonWindowsFullCommand,
				strings.Replace(
					s7CINonWindowsFullCommand,
					s7CIARPartitionPattern,
					`^TestS7AR.*$`,
					1,
				),
				1,
			),
		},
		{
			name: "partition-order-swapped",
			mutation: strings.Replace(
				workflow,
				"          "+s7CINonWindowsFullCommand+"\n"+
					"          "+s7CINonWindowsARLegacyCommand,
				"          "+s7CINonWindowsARLegacyCommand+"\n"+
					"          "+s7CINonWindowsFullCommand,
				1,
			),
		},
		{
			name: "partition-shard-skipped-by-or",
			mutation: strings.Replace(
				workflow,
				s7CINonWindowsARLegacyCommand,
				"true || "+s7CINonWindowsARLegacyCommand,
				1,
			),
		},
		{
			name: "partition-shard-failure-masked",
			mutation: strings.Replace(
				workflow,
				s7CINonWindowsARLegacyCommand,
				s7CINonWindowsARLegacyCommand+" || true",
				1,
			),
		},
		{
			name:     "timeouts-swapped",
			mutation: swapTimeouts,
		},
		{
			name: "blocking-step-demoted",
			mutation: strings.Replace(
				workflow,
				"      - name: Test\n        if: runner.os != 'Windows'\n",
				"      - name: Test\n        if: runner.os != 'Windows'\n        continue-on-error: true\n",
				1,
			),
		},
		{
			name: "blocking-step-demoted-by-quoted-key",
			mutation: strings.Replace(
				workflow,
				"      - name: Test\n        if: runner.os != 'Windows'\n",
				"      - name: Test\n        if: runner.os != 'Windows'\n"+
					"        \"continue-on-error\": true\n",
				1,
			),
		},
		{
			name: "blocking-step-demoted-by-escaped-key",
			mutation: strings.Replace(
				workflow,
				"      - name: Test\n        if: runner.os != 'Windows'\n",
				"      - name: Test\n        if: runner.os != 'Windows'\n"+
					`        "\u0063ontinue-on-error": true`+"\n",
				1,
			),
		},
		{
			name: "blocking-step-demoted-by-merge-key",
			mutation: strings.Replace(
				workflow,
				"      - name: Test\n        if: runner.os != 'Windows'\n",
				"      - name: Test\n        if: runner.os != 'Windows'\n"+
					"        <<: {continue-on-error: true}\n",
				1,
			),
		},
		{
			name: "blocking-step-duplicate-condition",
			mutation: strings.Replace(
				workflow,
				"      - name: Test\n        if: runner.os != 'Windows'\n",
				"      - name: Test\n        if: runner.os != 'Windows'\n"+
					"        if: false\n",
				1,
			),
		},
		{
			name: "bare-sequence-split-step-impersonation",
			mutation: strings.Replace(
				workflow,
				nonWindows,
				wideBareNonWindows,
				1,
			),
		},
		{
			name: "commented-bare-sequence-split",
			mutation: strings.Replace(
				workflow,
				nonWindows,
				strings.ReplaceAll(
					wideBareNonWindows,
					"      -\n",
					"      - # split item\n",
				),
				1,
			),
		},
		{
			name: "explicit-quoted-key-demotion",
			mutation: strings.Replace(
				workflow,
				"      - name: Test\n        if: runner.os != 'Windows'\n",
				"      - name: Test\n        if: runner.os != 'Windows'\n"+
					"        ? \"continue-on-error\"\n"+
					"        : true\n",
				1,
			),
		},
		{
			name: "non-windows-condition-narrowed",
			mutation: strings.Replace(
				workflow,
				"if: runner.os != 'Windows'",
				"if: runner.os != 'Windows' && github.event_name == 'push'",
				1,
			),
		},
		{
			name: "windows-ownership-removed",
			mutation: strings.Replace(
				workflow,
				"continue-on-error: true\n        shell: bash\n        run: go test ./... -count=1 -timeout 20m",
				"continue-on-error: false\n        shell: bash\n        run: go test ./... -count=1 -timeout 20m",
				1,
			),
		},
		{
			name: "windows-condition-narrowed",
			mutation: strings.Replace(
				workflow,
				`if: runner.os == 'Windows'
        continue-on-error: true
        shell: bash
        run: go test ./... -count=1 -timeout 20m`,
				`if: runner.os == 'Windows' && false
        continue-on-error: true
        shell: bash
        run: go test ./... -count=1 -timeout 20m`,
				1,
			),
		},
		{
			name: "second-blocking-package-last-unbounded",
			mutation: withAdditionalStep(`      - name: S7 extra blocking full suite
        if: runner.os != 'Windows'
        run: go test -count=1 -timeout 0 ./...
`),
		},
		{
			name: "additional-advisory-flags-after-package",
			mutation: withAdditionalStep(`      - name: S7 extra advisory full suite
        if: runner.os == 'Windows'
        continue-on-error: true
        run: go test ./... -timeout 0 -count=1
`),
		},
		{
			name: "additional-unconditioned-env-prefixed-reordered",
			mutation: withReleaseStep(`      - name: S7 extra unconditioned full suite
        run: env GOTOOLCHAIN=local go test -timeout=0 -count=1 ./...
`),
		},
		{
			name: "nested-bash-package-last",
			mutation: withReleaseStep(`      - name: S7 nested bash full suite
        run: bash -c 'go test -count=1 -timeout 0 ./...'
`),
		},
		{
			name: "nested-sh-package-first",
			mutation: withReleaseStep(`      - name: S7 nested sh full suite
        run: sh -c "go test ./... -timeout=0"
`),
		},
		{
			name: "nested-env-prefixed-shell",
			mutation: withReleaseStep(`      - name: S7 env-prefixed nested full suite
        run: env S7_TIMEOUT_GUARD=1 bash --command 'go test -timeout 0 ./...'
`),
		},
		{
			name: "nested-nested-shell",
			mutation: withReleaseStep(`      - name: S7 recursively nested full suite
        run: bash -c 'sh -c "go test -count=1 -timeout=0 ./..."'
`),
		},
		{
			name: "dynamic-shell-payload",
			mutation: withReleaseStep(`      - name: S7 dynamic nested shell
        run: bash -c "$cmd"
`),
		},
		{
			name: "opaque-shell-script",
			mutation: withReleaseStep(`      - name: S7 opaque nested shell
        run: bash scripts/run-tests.sh
`),
		},
		{
			name: "multiple-shell-payload-argv",
			mutation: withReleaseStep(`      - name: S7 assembled nested shell
        run: sh -c 'go test -timeout 0' './...'
`),
		},
		{
			name: "dynamic-bash-variable-executable",
			mutation: withReleaseStep(`      - name: S7 dynamic bash executable
        run: "$BASH" -c 'go test -timeout 0 ./...'
`),
		},
		{
			name: "dynamic-shell-parameter-executable",
			mutation: withReleaseStep(`      - name: S7 dynamic shell executable
        run: ${SHELL} -c 'go test -timeout 0 ./...'
`),
		},
		{
			name: "dynamic-command-substitution-executable",
			mutation: withReleaseStep(`      - name: S7 command-substitution shell executable
        run: $(command -v bash) -c 'go test -timeout 0 ./...'
`),
		},
		{
			name: "dynamic-backtick-executable",
			mutation: withReleaseStep("      - name: S7 backtick shell executable\n" +
				"        run: `command -v bash` -c 'go test -timeout 0 ./...'\n"),
		},
		{
			name: "env-prefixed-dynamic-shell-executable",
			mutation: withReleaseStep(`      - name: S7 env-prefixed dynamic shell executable
        run: env S7_SHELL="$BASH" "$S7_SHELL" --command 'go test -timeout 0 ./...'
`),
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			if fixture.mutation == workflow {
				t.Fatal("workflow sensitivity mutation did not apply")
			}
			if err := validateS7CIFullSuiteTimeouts(fixture.mutation); err == nil {
				t.Fatal("S7 CI timeout guard accepted wrong input")
			}
		})
	}

	if err := validateS7ARPartitionTestOwners(avpRepoRoot(t)); err != nil {
		t.Fatal(err)
	}
	if err := validateS7ARPartitionTestOwner(
		"internal/store/injected_test.go", "TestS7ARInjected",
	); err == nil {
		t.Fatal("S7 AR partition owner guard accepted a matching external package test")
	}
	windowsOnly := filepath.Join(t.TempDir(), "injected_windows_test.go")
	if err := os.WriteFile(
		windowsOnly,
		[]byte("//go:build windows\n\npackage cli\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	eligible, err := s7ARPartitionNonWindowsEligibility(windowsOnly)
	if err != nil {
		t.Fatal(err)
	}
	if eligible["linux"] || eligible["darwin"] {
		t.Fatal("S7 AR partition eligibility accepted a Windows-only test file")
	}
}

func validateS7CIFullSuiteTimeouts(workflow string) error {
	if err := validateS7CITestJobTopology(workflow); err != nil {
		return err
	}
	steps, err := parseS7CITimeoutSteps(workflow)
	if err != nil {
		return err
	}
	const (
		nonWindowsCondition = "runner.os != 'Windows'"
		windowsCondition    = "runner.os == 'Windows'"
		windowsName         = "Test (Windows full suite — allowed to fail, owned by GH #17)"
	)
	type fullSuiteInvocation struct {
		step s7CITimeoutStep
		argv []string
		args []string
	}
	var fullSuites []fullSuiteInvocation
	for _, step := range steps {
		invocations, err := collectS7ShellInvocations(step.run, 0, map[string]bool{})
		if err != nil {
			return fmt.Errorf("parse shell commands for step %q: %w", step.name, err)
		}
		for _, invocation := range invocations {
			argv := invocation.argv()
			args, ok := s7GoTestArgs(argv)
			if !ok || !slices.Contains(args, "./...") {
				continue
			}
			fullSuites = append(fullSuites, fullSuiteInvocation{
				step: step,
				argv: argv,
				args: args,
			})
		}
	}
	if len(fullSuites) != 2 {
		return fmt.Errorf("full-suite `go test` invocation count = %d, want exactly 2", len(fullSuites))
	}

	nonWindowsCount := 0
	windowsCount := 0
	for _, invocation := range fullSuites {
		step := invocation.step
		if step.job != "test" {
			return fmt.Errorf("full-suite step %q belongs to job %q, want test", step.name, step.job)
		}
		switch normalizeS7CICondition(step.condition) {
		case nonWindowsCondition:
			nonWindowsCount++
			if mode := classifyS7CIContinueOnError(step.continueOnError); mode != "blocking" {
				return fmt.Errorf("non-Windows full suite is %s, want blocking", mode)
			}
			if err := validateS7FullSuiteInvocation(
				invocation.argv, invocation.args, s7CINonWindowsFullCommand, "40m",
			); err != nil {
				return fmt.Errorf("non-Windows full suite: %w", err)
			}
			if err := validateS7NonWindowsTestPartition(step); err != nil {
				return err
			}
		case windowsCondition:
			windowsCount++
			wantKeys := map[string]int{
				"name": 1, "if": 1, "continue-on-error": 1,
				"shell": 1, "run": 1,
			}
			if !maps.Equal(step.keys, wantKeys) {
				return fmt.Errorf(
					"Windows full-suite keys = %v, want %v",
					step.keys, wantKeys,
				)
			}
			if step.name != windowsName {
				return fmt.Errorf("Windows allowed-failure full-suite name = %q, want %q", step.name, windowsName)
			}
			if classifyS7CIContinueOnError(step.continueOnError) != "allowed-failure" {
				return fmt.Errorf("Windows full suite continue-on-error = %q, want exact literal true", step.continueOnError)
			}
			if step.shell != "bash" {
				return fmt.Errorf("Windows allowed-failure full-suite shell = %q, want bash", step.shell)
			}
			if strings.TrimSpace(step.run) != s7CIWindowsFullSuiteCommand {
				return errors.New(
					"Windows allowed-failure full-suite script is not the exact canonical command",
				)
			}
			if err := validateS7FullSuiteInvocation(
				invocation.argv, invocation.args, s7CIWindowsFullSuiteCommand, "20m",
			); err != nil {
				return fmt.Errorf("Windows allowed-failure full suite: %w", err)
			}
		default:
			return fmt.Errorf("full-suite step %q has noncanonical condition %q", step.name, step.condition)
		}
	}
	if nonWindowsCount != 1 {
		return fmt.Errorf("blocking non-Windows full-suite step count = %d, want exactly 1", nonWindowsCount)
	}
	if windowsCount != 1 {
		return fmt.Errorf("Windows allowed-failure full-suite step count = %d, want exactly 1", windowsCount)
	}
	return nil
}

func validateS7NonWindowsTestPartition(step s7CITimeoutStep) error {
	wantKeys := map[string]int{
		"name": 1, "if": 1, "shell": 1, "env": 1, "run": 1,
	}
	if !maps.Equal(step.keys, wantKeys) {
		return fmt.Errorf(
			"non-Windows test partition keys = %v, want %v",
			step.keys, wantKeys,
		)
	}
	if step.shell != "bash" {
		return fmt.Errorf(
			"non-Windows test partition shell = %q, want exact bash",
			step.shell,
		)
	}
	if step.runStyle != "|" {
		return fmt.Errorf(
			"non-Windows test partition run style = %q, want literal block |",
			step.runStyle,
		)
	}
	wantEnvironment := map[string]string{
		"BASH_ENV": "/dev/null",
		"GOFLAGS":  "",
		"GOENV":    "off",
	}
	if !maps.Equal(step.environment, wantEnvironment) {
		return fmt.Errorf(
			"non-Windows test partition environment = %v, want %v",
			step.environment, wantEnvironment,
		)
	}
	if strings.TrimSpace(step.run) != s7CINonWindowsTestScript {
		return errors.New(
			"non-Windows test partition script is not the exact canonical five-command sequence",
		)
	}
	invocations, err := collectS7ShellInvocations(step.run, 0, map[string]bool{})
	if err != nil {
		return fmt.Errorf("parse non-Windows test partition: %w", err)
	}
	var tests []s7ShellInvocation
	for _, invocation := range invocations {
		if _, ok := s7GoTestArgs(invocation.argv()); ok {
			tests = append(tests, invocation)
		}
	}
	if len(tests) != 11 {
		return fmt.Errorf(
			"non-Windows test partition command count = %d, want exactly 11",
			len(tests),
		)
	}
	for index, expected := range []struct {
		command string
		timeout string
	}{
		{command: s7CINonWindowsFullCommand, timeout: "40m"},
		{command: s7CINonWindowsARLegacyCommand, timeout: "40m"},
		{command: s7CINonWindowsARLegacyMidCommand, timeout: "40m"},
		{command: s7CINonWindowsARLegacyLateCommand, timeout: "40m"},
		{command: s7CINonWindowsARCurrentCommand, timeout: "40m"},
		{command: s7CINonWindowsARCurrentLateCommand, timeout: "40m"},
		{command: s7CINonWindowsARCoreCommand, timeout: "40m"},
		{command: s7CINonWindowsARAbandonCommand, timeout: "40m"},
		{command: s7CINonWindowsARPurgeCommand, timeout: "40m"},
		{command: s7CINonWindowsARPermanentCommand, timeout: "40m"},
		{command: s7CINonWindowsARObserverCommand, timeout: "40m"},
	} {
		argv := tests[index].argv()
		args, _ := s7GoTestArgs(argv)
		if err := validateS7FullSuiteInvocation(
			argv, args, expected.command, expected.timeout,
		); err != nil {
			return fmt.Errorf("non-Windows test partition %d: %w", index, err)
		}
	}
	return nil
}

func validateS7CITestJobTopology(workflow string) error {
	lines := strings.Split(workflow, "\n")
	inJobs := false
	var starts []int
	for index := 0; index < len(lines); index++ {
		raw := lines[index]
		trimmed := strings.TrimSpace(raw)
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		if indent == 0 {
			inJobs = trimmed == "jobs:"
			continue
		}
		if inJobs && indent == 2 && trimmed == "test:" {
			starts = append(starts, index)
		}
	}
	if len(starts) != 1 {
		return fmt.Errorf("jobs.test definition count = %d, want exactly 1", len(starts))
	}
	start := starts[0]
	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		raw := lines[index]
		trimmed := strings.TrimSpace(raw)
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && indent <= 2 {
			end = index
			break
		}
	}
	direct := map[string]string{}
	for index := start + 1; index < end; index++ {
		raw := lines[index]
		trimmed := strings.TrimSpace(raw)
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || indent != 4 {
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			return fmt.Errorf("jobs.test direct entry is malformed: %q", trimmed)
		}
		key = strings.TrimSpace(key)
		value = normalizeS7CIScalar(strings.TrimSpace(value))
		if _, duplicate := direct[key]; duplicate {
			return fmt.Errorf("jobs.test direct key %q is duplicated", key)
		}
		direct[key] = value
		switch key {
		case "name", "runs-on", "steps":
		case "strategy":
			if err := validateS7CITestStrategy(lines, index, end); err != nil {
				return err
			}
		default:
			return fmt.Errorf("jobs.test has unsupported direct key %q", key)
		}
	}
	want := map[string]string{
		"name":     "test (${{ matrix.os }})",
		"runs-on":  "${{ matrix.os }}",
		"strategy": "",
		"steps":    "",
	}
	if !maps.Equal(direct, want) {
		return fmt.Errorf("jobs.test direct topology = %v, want %v", direct, want)
	}
	return nil
}

func validateS7CITestStrategy(lines []string, start, jobEnd int) error {
	end := jobEnd
	for index := start + 1; index < jobEnd; index++ {
		raw := lines[index]
		trimmed := strings.TrimSpace(raw)
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && indent <= 4 {
			end = index
			break
		}
	}
	strategy := map[string]string{}
	matrix := map[string]string{}
	inMatrix := false
	for index := start + 1; index < end; index++ {
		raw := lines[index]
		trimmed := strings.TrimSpace(raw)
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		switch indent {
		case 6:
			key, value, ok := strings.Cut(trimmed, ":")
			if !ok {
				return fmt.Errorf("jobs.test strategy entry is malformed: %q", trimmed)
			}
			key = strings.TrimSpace(key)
			if _, duplicate := strategy[key]; duplicate {
				return fmt.Errorf("jobs.test strategy key %q is duplicated", key)
			}
			strategy[key] = normalizeS7CIScalar(strings.TrimSpace(value))
			inMatrix = key == "matrix"
		case 8:
			if !inMatrix {
				return fmt.Errorf("jobs.test strategy has unexpected nested entry %q", trimmed)
			}
			key, value, ok := strings.Cut(trimmed, ":")
			if !ok {
				return fmt.Errorf("jobs.test matrix entry is malformed: %q", trimmed)
			}
			key = strings.TrimSpace(key)
			if _, duplicate := matrix[key]; duplicate {
				return fmt.Errorf("jobs.test matrix key %q is duplicated", key)
			}
			matrix[key] = normalizeS7CIScalar(strings.TrimSpace(value))
		default:
			return fmt.Errorf(
				"jobs.test strategy has unsupported indentation %d: %q",
				indent, trimmed,
			)
		}
	}
	wantStrategy := map[string]string{"fail-fast": "false", "matrix": ""}
	if !maps.Equal(strategy, wantStrategy) {
		return fmt.Errorf(
			"jobs.test strategy = %v, want %v",
			strategy, wantStrategy,
		)
	}
	wantMatrix := map[string]string{
		"os": "[ubuntu-latest, macos-latest, windows-latest]",
	}
	if !maps.Equal(matrix, wantMatrix) {
		return fmt.Errorf("jobs.test matrix = %v, want %v", matrix, wantMatrix)
	}
	return nil
}

func validateS7ARPartitionTestOwners(root string) error {
	pattern := regexp.MustCompile(s7CIARPartitionPattern)
	shards := []*regexp.Regexp{
		regexp.MustCompile(s7CIARLegacyPattern),
		regexp.MustCompile(s7CIARLegacyMidPattern),
		regexp.MustCompile(s7CIARLegacyLatePattern),
		regexp.MustCompile(s7CIARCurrentPattern),
		regexp.MustCompile(s7CIARCurrentLatePattern),
		regexp.MustCompile(s7CIARCorePattern),
		regexp.MustCompile(s7CIARAbandonPattern),
		regexp.MustCompile(s7CIARPurgePattern),
		regexp.MustCompile(s7CIARPermanentPattern),
		regexp.MustCompile(s7CIARObserverPattern),
	}
	matched := map[string][]int{
		"linux":  make([]int, len(shards)),
		"darwin": make([]int, len(shards)),
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == ".tpatch" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(content), "TestS7AR") &&
			!strings.Contains(string(content), "TestS7ObservedARRegistrationAuthority") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, content, 0)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || !pattern.MatchString(function.Name.Name) {
				continue
			}
			if err := validateS7ARPartitionTestOwner(rel, function.Name.Name); err != nil {
				return err
			}
			eligible, err := s7ARPartitionNonWindowsEligibility(path)
			if err != nil {
				return err
			}
			if !eligible["linux"] && !eligible["darwin"] {
				return fmt.Errorf(
					"S7 AR partition test %s is ineligible on all non-Windows runners: %s",
					function.Name.Name, rel,
				)
			}
			owners := 0
			for index, shard := range shards {
				if shard.MatchString(function.Name.Name) {
					for goos, included := range eligible {
						if included {
							matched[goos][index]++
						}
					}
					owners++
				}
			}
			if owners != 1 {
				return fmt.Errorf(
					"S7 AR partition test %s belongs to %d shards, want exactly 1",
					function.Name.Name, owners,
				)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for goos, counts := range matched {
		for index, count := range counts {
			if count == 0 {
				return fmt.Errorf(
					"S7 AR partition shard %d owns no %s tests",
					index, goos,
				)
			}
		}
	}
	return nil
}

func s7ARPartitionNonWindowsEligibility(path string) (map[string]bool, error) {
	result := map[string]bool{}
	for goos, goarch := range map[string]string{
		"linux": "amd64", "darwin": "arm64",
	} {
		context := build.Default
		context.GOOS = goos
		context.GOARCH = goarch
		matched, err := context.MatchFile(
			filepath.Dir(path), filepath.Base(path),
		)
		if err != nil {
			return nil, fmt.Errorf(
				"match S7 AR partition file %s for %s/%s: %w",
				path, goos, goarch, err,
			)
		}
		result[goos] = matched
	}
	return result, nil
}

func validateS7ARPartitionTestOwner(rel, name string) error {
	if !regexp.MustCompile(s7CIARPartitionPattern).MatchString(name) {
		return nil
	}
	if filepath.ToSlash(filepath.Dir(rel)) != "internal/cli" {
		return fmt.Errorf("S7 AR partition test %s is outside internal/cli: %s", name, rel)
	}
	return nil
}

func validateS7FullSuiteInvocation(argv, args []string, command, timeout string) error {
	value, count, err := s7GoTestTimeout(args)
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("timeout flag count = %d, want exactly 1", count)
	}
	if value != timeout {
		return fmt.Errorf("effective timeout = %q, want %q", value, timeout)
	}
	wantArgv, err := parseS7ShellInvocations(command)
	if err != nil || len(wantArgv) != 1 {
		return fmt.Errorf("internal canonical command parse failed: %v", err)
	}
	if !slices.Equal(argv, wantArgv[0]) {
		return fmt.Errorf("command argv = %q, want canonical %q", argv, wantArgv[0])
	}
	return nil
}

func s7GoTestTimeout(args []string) (string, int, error) {
	value := ""
	count := 0
	for index := 0; index < len(args); index++ {
		switch {
		case args[index] == "-timeout":
			count++
			if index+1 >= len(args) {
				return "", count, errors.New("-timeout has no value")
			}
			index++
			value = args[index]
		case strings.HasPrefix(args[index], "-timeout="):
			count++
			value = strings.TrimPrefix(args[index], "-timeout=")
			if value == "" {
				return "", count, errors.New("-timeout= has no value")
			}
		}
	}
	return value, count, nil
}

func collectS7ShellInvocations(
	script string,
	depth int,
	active map[string]bool,
) ([]s7ShellInvocation, error) {
	const maximumNestedShellDepth = 8
	if depth > maximumNestedShellDepth {
		return nil, fmt.Errorf("nested shell depth exceeds %d", maximumNestedShellDepth)
	}
	invocations, err := parseS7ShellInvocationTokens(script)
	if err != nil {
		return nil, err
	}
	all := append([]s7ShellInvocation(nil), invocations...)
	for _, invocation := range invocations {
		if err := validateS7ExecutableResolution(invocation); err != nil {
			return nil, err
		}
		payload, nested, err := s7NestedShellPayload(invocation)
		if err != nil {
			return nil, err
		}
		if !nested {
			continue
		}
		if active[payload] {
			return nil, errors.New("nested shell payload cycle detected")
		}
		active[payload] = true
		children, err := collectS7ShellInvocations(payload, depth+1, active)
		delete(active, payload)
		if err != nil {
			return nil, err
		}
		all = append(all, children...)
	}
	return all, nil
}

func validateS7ExecutableResolution(invocation s7ShellInvocation) error {
	argv := invocation.argv()
	commandIndex := s7EffectiveCommandIndex(argv)
	if commandIndex < 0 || !invocation.tokens[commandIndex].dynamic {
		return nil
	}
	arguments := argv[commandIndex+1:]
	if len(arguments) > 0 && arguments[0] == "=" {
		return nil
	}
	executable := strings.ReplaceAll(argv[commandIndex], `\`, "/")
	if slices.Equal(arguments, []string{"--version"}) &&
		(strings.HasSuffix(executable, "/bin/tpatch") ||
			strings.HasSuffix(executable, "/bin/tpatch.exe") ||
			strings.HasSuffix(executable, "bintpatch.exe")) {
		return nil
	}
	return fmt.Errorf("effective executable %q is dynamic or unresolved", argv[commandIndex])
}

func s7NestedShellPayload(invocation s7ShellInvocation) (string, bool, error) {
	argv := invocation.argv()
	commandIndex := s7EffectiveCommandIndex(argv)
	if commandIndex < 0 {
		return "", false, nil
	}
	executable := filepath.Base(argv[commandIndex])
	if executable != "sh" && executable != "bash" {
		return "", false, nil
	}
	args := argv[commandIndex+1:]
	commandMode := -1
	for index, argument := range args {
		if argument == "-c" || argument == "--command" {
			if commandMode >= 0 {
				return "", false, fmt.Errorf("%s has duplicate command modes", executable)
			}
			commandMode = index
			continue
		}
		if commandMode < 0 {
			return "", false, fmt.Errorf("%s uses opaque invocation mode %q", executable, argument)
		}
	}
	if commandMode < 0 {
		return "", false, fmt.Errorf("%s invocation can execute opaque commands without -c/--command", executable)
	}
	payloadIndex := commandIndex + 1 + commandMode + 1
	if payloadIndex >= len(invocation.tokens) {
		return "", false, fmt.Errorf("%s command mode has no payload", executable)
	}
	if payloadIndex+1 != len(invocation.tokens) {
		return "", false, fmt.Errorf("%s command mode has %d payload argv, want exactly 1",
			executable, len(invocation.tokens)-payloadIndex)
	}
	payload := invocation.tokens[payloadIndex]
	if payload.dynamic {
		return "", false, fmt.Errorf("%s command payload is dynamic or unresolved", executable)
	}
	return payload.value, true, nil
}

func s7GoTestArgs(argv []string) ([]string, bool) {
	index := s7EffectiveCommandIndex(argv)
	if index < 0 || index+1 >= len(argv) ||
		argv[index] != "go" || argv[index+1] != "test" {
		return nil, false
	}
	return argv[index+2:], true
}

func s7EffectiveCommandIndex(argv []string) int {
	index := 0
	for index < len(argv) && isS7ShellAssignment(argv[index]) {
		index++
	}
	if index < len(argv) && argv[index] == "env" {
		index++
		for index < len(argv) {
			if argv[index] == "--" {
				index++
				break
			}
			if isS7ShellAssignment(argv[index]) {
				index++
				continue
			}
			if argv[index] == "-i" || argv[index] == "--ignore-environment" {
				index++
				continue
			}
			if (argv[index] == "-u" || argv[index] == "--unset") && index+1 < len(argv) {
				index += 2
				continue
			}
			if strings.HasPrefix(argv[index], "--unset=") {
				index++
				continue
			}
			break
		}
	}
	if index < len(argv) && (argv[index] == "command" || argv[index] == "exec") {
		index++
	}
	if index >= len(argv) {
		return -1
	}
	return index
}

func isS7ShellAssignment(value string) bool {
	name, _, ok := strings.Cut(value, "=")
	if !ok || name == "" {
		return false
	}
	for index, character := range name {
		if character == '_' || character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			index > 0 && character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return true
}

type s7ShellToken struct {
	value   string
	dynamic bool
}

type s7ShellInvocation struct {
	tokens []s7ShellToken
}

func (invocation s7ShellInvocation) argv() []string {
	argv := make([]string, len(invocation.tokens))
	for index, token := range invocation.tokens {
		argv[index] = token.value
	}
	return argv
}

func parseS7ShellInvocations(script string) ([][]string, error) {
	parsed, err := parseS7ShellInvocationTokens(script)
	if err != nil {
		return nil, err
	}
	invocations := make([][]string, len(parsed))
	for index, invocation := range parsed {
		invocations[index] = invocation.argv()
	}
	return invocations, nil
}

func parseS7ShellInvocationTokens(script string) ([]s7ShellInvocation, error) {
	var (
		invocations  []s7ShellInvocation
		tokens       []s7ShellToken
		token        strings.Builder
		tokenOpen    bool
		tokenDynamic bool
		quote        rune
	)
	flushToken := func() {
		if tokenOpen {
			tokens = append(tokens, s7ShellToken{
				value:   token.String(),
				dynamic: tokenDynamic,
			})
			token.Reset()
			tokenOpen = false
			tokenDynamic = false
		}
	}
	flushCommand := func() {
		flushToken()
		if len(tokens) > 0 {
			invocations = append(invocations, s7ShellInvocation{tokens: tokens})
			tokens = nil
		}
	}
	runes := []rune(script)
	for index := 0; index < len(runes); index++ {
		character := runes[index]
		if quote != 0 {
			if character == quote {
				quote = 0
				continue
			}
			if quote == '"' && character == '\\' && index+1 < len(runes) {
				index++
				if runes[index] != '\n' {
					token.WriteRune(runes[index])
					tokenOpen = true
				}
				continue
			}
			if quote == '"' && (character == '$' || character == '`') {
				tokenDynamic = true
			}
			token.WriteRune(character)
			tokenOpen = true
			continue
		}
		switch character {
		case '\'', '"':
			quote = character
			tokenOpen = true
		case '\\':
			if index+1 >= len(runes) {
				return nil, errors.New("trailing shell escape")
			}
			index++
			if runes[index] != '\n' {
				token.WriteRune(runes[index])
				tokenOpen = true
			}
		case '#':
			if tokenOpen {
				token.WriteRune(character)
				continue
			}
			for index+1 < len(runes) && runes[index+1] != '\n' {
				index++
			}
		case '$':
			if index+1 < len(runes) && runes[index+1] == '(' {
				substitution, end, err := consumeS7CommandSubstitution(runes, index)
				if err != nil {
					return nil, err
				}
				token.WriteString(substitution)
				tokenOpen = true
				tokenDynamic = true
				index = end
				continue
			}
			token.WriteRune(character)
			tokenOpen = true
			tokenDynamic = true
		case '`':
			substitution, end, err := consumeS7BacktickSubstitution(runes, index)
			if err != nil {
				return nil, err
			}
			token.WriteString(substitution)
			tokenOpen = true
			tokenDynamic = true
			index = end
		case '*', '?':
			token.WriteRune(character)
			tokenOpen = true
			tokenDynamic = true
		case '~':
			token.WriteRune(character)
			tokenDynamic = tokenDynamic || !tokenOpen
			tokenOpen = true
		case ' ', '\t', '\r':
			flushToken()
		case '\n', ';', '|', '&':
			flushCommand()
			if index+1 < len(runes) && runes[index+1] == character &&
				(character == '|' || character == '&') {
				index++
			}
		default:
			token.WriteRune(character)
			tokenOpen = true
		}
	}
	if quote != 0 {
		return nil, errors.New("unterminated shell quote")
	}
	flushCommand()
	return invocations, nil
}

func consumeS7CommandSubstitution(runes []rune, start int) (string, int, error) {
	depth := 0
	quote := rune(0)
	escaped := false
	for index := start; index < len(runes); index++ {
		character := runes[index]
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if character == quote {
				quote = 0
			}
			continue
		}
		if character == '\'' || character == '"' {
			quote = character
			continue
		}
		if character == '(' {
			depth++
			continue
		}
		if character == ')' {
			depth--
			if depth == 0 {
				return string(runes[start : index+1]), index, nil
			}
		}
	}
	return "", start, errors.New("unterminated shell command substitution")
}

func consumeS7BacktickSubstitution(runes []rune, start int) (string, int, error) {
	escaped := false
	for index := start + 1; index < len(runes); index++ {
		if escaped {
			escaped = false
			continue
		}
		if runes[index] == '\\' {
			escaped = true
			continue
		}
		if runes[index] == '`' {
			return string(runes[start : index+1]), index, nil
		}
	}
	return "", start, errors.New("unterminated backtick shell substitution")
}

func parseS7CITimeoutSteps(workflow string) ([]s7CITimeoutStep, error) {
	lines := strings.Split(workflow, "\n")
	var (
		steps         []s7CITimeoutStep
		current       *s7CITimeoutStep
		job           string
		inJobs        bool
		inSteps       bool
		stepsIndent   = -1
		itemIndent    = -1
		contentIndent = -1
		blockRun      bool
		blockLines    []string
		blockEnv      bool
	)
	closeBlock := func() {
		if blockRun && current != nil {
			current.run = strings.Join(blockLines, "\n")
		}
		blockRun = false
		blockLines = nil
	}
	flush := func() {
		closeBlock()
		blockEnv = false
		if current != nil {
			steps = append(steps, *current)
			current = nil
		}
	}
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		if blockRun {
			if trimmed == "" {
				blockLines = append(blockLines, "")
				continue
			}
			if indent > contentIndent {
				blockLines = append(blockLines, raw[min(contentIndent+2, indent):])
				continue
			}
			closeBlock()
		}
		if blockEnv {
			if trimmed == "" {
				continue
			}
			if current != nil && indent == contentIndent+2 {
				key, value, ok := strings.Cut(trimmed, ":")
				if !ok {
					return nil, errors.New("workflow step environment entry is malformed")
				}
				key = strings.TrimSpace(key)
				if strings.HasPrefix(key, `"`) || strings.HasPrefix(key, `'`) {
					return nil, errors.New("quoted workflow environment keys are unsupported")
				}
				if _, duplicate := current.environment[key]; duplicate {
					return nil, fmt.Errorf(
						"workflow environment key %q is duplicated",
						key,
					)
				}
				current.environment[key] =
					normalizeS7CIScalar(strings.TrimSpace(value))
				continue
			}
			if indent > contentIndent {
				return nil, errors.New("workflow step environment nesting is unsupported")
			}
			blockEnv = false
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if indent == 0 {
			flush()
			inSteps = false
			inJobs = trimmed == "jobs:"
			continue
		}
		if inJobs && indent == 2 && strings.HasSuffix(trimmed, ":") &&
			!strings.HasPrefix(trimmed, "-") {
			flush()
			inSteps = false
			job = strings.TrimSuffix(trimmed, ":")
			continue
		}
		if inJobs && indent == 4 && trimmed == "steps:" {
			flush()
			inSteps = true
			stepsIndent = indent
			itemIndent = -1
			continue
		}
		if !inSteps {
			continue
		}
		if indent <= stepsIndent {
			flush()
			inSteps = false
			continue
		}
		if (trimmed == "-" || strings.HasPrefix(trimmed, "- #")) &&
			(itemIndent == -1 || indent == itemIndent) {
			return nil, errors.New("standalone workflow step markers are unsupported")
		}
		if strings.HasPrefix(trimmed, "- ") &&
			(itemIndent == -1 || indent == itemIndent) {
			flush()
			itemIndent = indent
			contentIndent = indent + 2
			current = &s7CITimeoutStep{
				job:         job,
				environment: map[string]string{},
				keys:        map[string]int{},
			}
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			indent = contentIndent
		}
		if current == nil || indent != contentIndent {
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			return nil, fmt.Errorf("workflow step entry is malformed: %q", trimmed)
		}
		key = strings.TrimSpace(key)
		if key == "" || strings.HasPrefix(key, "?") {
			return nil, fmt.Errorf("explicit workflow step keys are unsupported: %q", trimmed)
		}
		if strings.HasPrefix(key, `"`) || strings.HasPrefix(key, `'`) {
			return nil, errors.New("quoted workflow step keys are unsupported")
		}
		current.keys[key]++
		value = strings.TrimSpace(value)
		rawValue := value
		if value == "|" || value == ">" || value == "|-" {
			if key == "run" {
				current.runStyle = value
				blockRun = true
				blockLines = nil
			}
			continue
		}
		if key == "env" && value == "" {
			blockEnv = true
			continue
		}
		value = normalizeS7CIScalar(value)
		switch key {
		case "name":
			current.name = value
		case "if":
			current.condition = value
		case "continue-on-error":
			current.continueOnError = rawValue
		case "shell":
			current.shell = value
		case "run":
			current.run = value
		}
	}
	flush()
	if len(steps) == 0 {
		return nil, errors.New("no workflow steps parsed")
	}
	return steps, nil
}

func normalizeS7CIScalar(value string) string {
	if len(value) >= 2 && value[0] == value[len(value)-1] &&
		(value[0] == '"' || value[0] == '\'') {
		return value[1 : len(value)-1]
	}
	if index := strings.Index(value, " #"); index >= 0 {
		return strings.TrimSpace(value[:index])
	}
	return value
}

func normalizeS7CICondition(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "${{") && strings.HasSuffix(value, "}}") {
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "${{"), "}}"))
	}
	return strings.Join(strings.Fields(value), " ")
}

func classifyS7CIContinueOnError(value string) string {
	switch strings.TrimSpace(value) {
	case "", "false":
		return "blocking"
	case "true":
		return "allowed-failure"
	default:
		return "nonliteral"
	}
}
