//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestS7PIB409WindowsBlockingSelectorGuard(t *testing.T) {
	workflowPath := filepath.Join(avpRepoRoot(t), ".github", "workflows", "ci.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateS7WindowsBlockingSelector(string(workflow)); err != nil {
		t.Fatal(err)
	}
	wrong := strings.Replace(
		string(workflow),
		"TestS7WindowsPlatformRows/PIB-409",
		"TestS7WindowsPlatformRows/PIB-410",
		1,
	)
	if err := validateS7WindowsBlockingSelector(wrong); err == nil ||
		!strings.Contains(err.Error(), "PIB-409") {
		t.Fatalf("Windows workflow guard accepted a missing PIB-409 native selector: %v", err)
	}
}

func TestS7PIB417WindowsBlockingLeafGuard(t *testing.T) {
	workflowPath := filepath.Join(avpRepoRoot(t), ".github", "workflows", "ci.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	wrong := strings.Replace(
		string(workflow),
		`TestS7WindowsPlatformRows/PIB-417/check-ready-json.txt`,
		`TestS7WindowsPlatformRows/PIB-417/check-ready-json-wrong.txt`,
		1,
	)
	if err := validateS7WindowsBlockingSelector(wrong); err == nil ||
		!strings.Contains(err.Error(), "PIB-417") {
		t.Fatalf("Windows workflow guard accepted a missing exact PIB-417 leaf assertion: %v", err)
	}
}

func TestS7APWindowsDryRunBlockingGuard(t *testing.T) {
	t.Run("PIB-461", func(t *testing.T) {
		workflowPath := filepath.Join(avpRepoRoot(t), ".github", "workflows", "ci.yml")
		workflowBytes, err := os.ReadFile(workflowPath)
		if err != nil {
			t.Fatal(err)
		}
		workflow := string(workflowBytes)
		if err := validateS7APWindowsDryRunBlocking(workflow); err != nil {
			t.Fatal(err)
		}
	})
}

func TestS7APWindowsDryRunBlockingGuardSensitivity(t *testing.T) {
	workflowPath := filepath.Join(avpRepoRoot(t), ".github", "workflows", "ci.yml")
	workflowBytes, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(workflowBytes)
	for _, fixture := range []struct {
		name     string
		mutation string
	}{
		{
			name: "target-omitted",
			mutation: strings.Replace(
				workflow,
				"-run '^TestS7APDryRunWindowsEvaluatedPlatform$'",
				"-run '^TestNothingAtAll$'",
				1,
			),
		},
		{
			name: "condition-narrowed",
			mutation: strings.Replace(
				workflow,
				`      - name: "Test (Windows GH #23 resource golden — blocking)"
        if: runner.os == 'Windows'
`,
				`      - name: "Test (Windows GH #23 resource golden — blocking)"
        if: runner.os == 'Windows' && false
`,
				1,
			),
		},
		{
			name: "step-demoted",
			mutation: strings.Replace(
				workflow,
				`      - name: "Test (Windows GH #23 resource golden — blocking)"
        if: runner.os == 'Windows'
`,
				`      - name: "Test (Windows GH #23 resource golden — blocking)"
        if: runner.os == 'Windows'
        continue-on-error: true
`,
				1,
			),
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			if fixture.mutation == workflow {
				t.Fatal("workflow sensitivity mutation did not apply")
			}
			if err := validateS7APWindowsDryRunBlocking(fixture.mutation); err == nil {
				t.Fatal("AP Windows blocking guard accepted wrong input")
			}
		})
	}
}

func validateS7APWindowsDryRunBlocking(workflow string) error {
	const header = `      - name: "Test (Windows GH #23 resource golden — blocking)"`
	start := strings.Index(workflow, header)
	if start < 0 {
		return &s7WorkflowGuardError{value: header}
	}
	step := workflow[start:]
	if next := strings.Index(step[len(header):], "\n      - name:"); next >= 0 {
		step = step[:len(header)+next]
	}
	required := []string{
		"\n        if: runner.os == 'Windows'\n",
		"-run '^TestS7APDryRunWindowsEvaluatedPlatform$'",
		"./internal/cli 2>&1 | tee \"$ap_log\"",
		"TestS7APDryRunWindowsEvaluatedPlatform/PIB-461",
		`--- PASS: ${selector}`,
	}
	for _, value := range required {
		if !strings.Contains(step, value) {
			return &s7WorkflowGuardError{value: value}
		}
	}
	if strings.Contains(step, "\n        continue-on-error:") {
		return errors.New("blocking AP Windows step is advisory")
	}
	if strings.Contains(step, "if: runner.os == 'Windows' &&") {
		return errors.New("blocking AP Windows condition is narrowed")
	}
	return nil
}

func validateS7WindowsBlockingSelector(workflow string) error {
	required := []string{
		"-run '^TestPreparePIBUnsupportedPlatformRuntimeGolden$'",
		"-run '^TestS7WindowsPlatformRows$'",
		"--- PASS: TestS7WindowsPlatformRows",
		"TestS7WindowsPlatformRows/PIB-409",
		"TestS7WindowsPlatformRows/PIB-417",
		"TestS7WindowsPlatformRows/PIB-417/check-ready-human.txt",
		"TestS7WindowsPlatformRows/PIB-417/check-ready-json.txt",
		`--- PASS: ${selector}`,
	}
	for _, value := range required {
		if !strings.Contains(workflow, value) {
			return &s7WorkflowGuardError{value: value}
		}
	}
	return nil
}

type s7WorkflowGuardError struct{ value string }

func (err *s7WorkflowGuardError) Error() string {
	return "blocking Windows selector lost exact fragment " + err.value
}
