//go:build windows

package cli

import (
	"strings"
	"testing"
)

func TestS7APDryRunWindowsNotEvaluatedPlatform(t *testing.T) {
	t.Run("PIB-463", func(t *testing.T) {
		observation := s7APObserveNotEvaluatedDryRun(
			t, "S7 AP Windows dry-run", false,
		)
		if observation.code != 0 || observation.stderr != "" ||
			observation.report.Outcome != "planned" ||
			!observation.report.DryRun ||
			observation.report.ExecutionPreflight != "not_evaluated" ||
			observation.report.PlanNote != s7APNotEvaluatedPlanNote ||
			observation.report.Refusal != nil ||
			observation.acquires != 0 || observation.providers != 0 ||
			observation.lockCalls != 0 || observation.after != observation.before {
			t.Fatalf("PIB-463 native Windows not-evaluated dry-run = %+v", observation)
		}
		for _, refusal := range s7APNotEvaluatedRefusals {
			if strings.Contains(observation.stdout, refusal) {
				t.Fatalf("PIB-463 native Windows dry-run emitted refusal %q: %s",
					refusal, observation.stdout)
			}
		}
	})
}
