package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	run             string
}

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

	nonWindows := `      - name: Test
        if: runner.os != 'Windows'
        run: go test ./... -count=1 -timeout 40m
`
	windows := `      - name: "Test (Windows full suite — allowed to fail, owned by GH #17)"
        if: runner.os == 'Windows'
        continue-on-error: true
        shell: bash
        run: go test ./... -count=1 -timeout 20m
`
	swapTimeouts := strings.Replace(workflow, nonWindows, "__S7_NON_WINDOWS_FULL_SUITE__", 1)
	swapTimeouts = strings.Replace(swapTimeouts, windows, strings.Replace(windows, "20m", "40m", 1), 1)
	swapTimeouts = strings.Replace(swapTimeouts, "__S7_NON_WINDOWS_FULL_SUITE__", strings.Replace(nonWindows, "40m", "20m", 1), 1)
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

	for _, fixture := range []struct {
		name     string
		mutation string
	}{
		{
			name: "lowered-timeout",
			mutation: strings.Replace(
				workflow,
				"go test ./... -count=1 -timeout 40m",
				"go test ./... -count=1 -timeout 20m",
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
				"go test ./... -count=1 -timeout 40m",
				"go test ./... -count=1",
				1,
			),
		},
		{
			name: "unbounded-timeout",
			mutation: strings.Replace(
				workflow,
				"go test ./... -count=1 -timeout 40m",
				"go test ./... -count=1 -timeout 0",
				1,
			),
		},
		{
			name: "duplicate-timeout",
			mutation: strings.Replace(
				workflow,
				"go test ./... -count=1 -timeout 40m",
				"go test ./... -count=1 -timeout 40m -timeout=40m",
				1,
			),
		},
		{
			name: "equals-timeout-noncanonical",
			mutation: strings.Replace(
				workflow,
				"go test ./... -count=1 -timeout 40m",
				"go test ./... -count=1 -timeout=40m",
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
}

func validateS7CIFullSuiteTimeouts(workflow string) error {
	steps, err := parseS7CITimeoutSteps(workflow)
	if err != nil {
		return err
	}
	const (
		nonWindowsCondition = "runner.os != 'Windows'"
		windowsCondition    = "runner.os == 'Windows'"
		nonWindowsCommand   = "go test ./... -count=1 -timeout 40m"
		windowsCommand      = "go test ./... -count=1 -timeout 20m"
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
				invocation.argv, invocation.args, nonWindowsCommand, "40m",
			); err != nil {
				return fmt.Errorf("non-Windows full suite: %w", err)
			}
		case windowsCondition:
			windowsCount++
			if step.name != windowsName {
				return fmt.Errorf("Windows allowed-failure full-suite name = %q, want %q", step.name, windowsName)
			}
			if classifyS7CIContinueOnError(step.continueOnError) != "allowed-failure" {
				return fmt.Errorf("Windows full suite continue-on-error = %q, want exact literal true", step.continueOnError)
			}
			if step.shell != "bash" {
				return fmt.Errorf("Windows allowed-failure full-suite shell = %q, want bash", step.shell)
			}
			if err := validateS7FullSuiteInvocation(
				invocation.argv, invocation.args, windowsCommand, "20m",
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
		if trimmed == "steps:" {
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
		if strings.HasPrefix(trimmed, "- ") && (itemIndent == -1 || indent == itemIndent) {
			flush()
			itemIndent = indent
			contentIndent = indent + 2
			current = &s7CITimeoutStep{job: job}
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			indent = contentIndent
		}
		if current == nil || indent != contentIndent {
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		rawValue := value
		if value == "|" || value == ">" || value == "|-" {
			if key == "run" {
				blockRun = true
				blockLines = nil
			}
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
