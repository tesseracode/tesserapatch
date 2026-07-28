package workflow

import (
	"fmt"
	"os"

	"github.com/tesseracode/tesserapatch/internal/store"
)

const doctorD5RemediationPrefix = "run tpatch reconcile "

func runDoctorD5(ctx *doctorContext) {
	for _, feature := range ctx.features {
		evidencePath := ctx.store.ReconcileEvidencePath(feature.Slug)
		evidenceExists := fileExistsDoctor(evidencePath)
		if feature.Status != nil && doctorD5RelevantState(feature.Status.State) && !evidenceExists {
			severity := "warning"
			code := "reconcile-evidence-missing-pre-adr025"
			message := "reconcile-evidence.jsonl is absent for a feature in a reconcile-capable state; treating as pre-ADR-025 grace because status.json has no modern reconcile signal"
			if doctorD5ModernReconcileAttempt(*feature.Status) {
				severity = "drift"
				code = "reconcile-evidence-missing"
				message = "reconcile-evidence.jsonl missing for feature whose status.json indicates a modern reconcile attempt"
			}
			ctx.addFinding(DoctorFinding{
				CheckID:     "D5",
				Code:        code,
				Severity:    severity,
				Feature:     feature.Slug,
				Path:        relOrAbs(ctx.root, evidencePath),
				Message:     message,
				Fixable:     false,
				Remediation: doctorD5RemediationPrefix + feature.Slug,
			})
		}
		if evidenceExists {
			valid, corrupt, err := store.LoadReconcileEvidenceLenient(evidencePath)
			_ = valid
			if err != nil {
				ctx.addFinding(DoctorFinding{
					CheckID:     "D5",
					Code:        "reconcile-evidence-unreadable",
					Severity:    "error",
					Feature:     feature.Slug,
					Path:        relOrAbs(ctx.root, evidencePath),
					Message:     fmt.Sprintf("cannot read reconcile-evidence.jsonl: %v", err),
					Fixable:     false,
					Remediation: doctorD5RemediationPrefix + feature.Slug,
				})
			}
			for _, entry := range corrupt {
				ctx.addFinding(DoctorFinding{
					CheckID:     "D5",
					Code:        "reconcile-evidence-malformed",
					Severity:    "drift",
					Feature:     feature.Slug,
					Path:        relOrAbs(ctx.root, evidencePath),
					Line:        entry.Line,
					Message:     fmt.Sprintf("reconcile-evidence.jsonl line %d is malformed: %s", entry.Line, entry.Error),
					Fixable:     false,
					Remediation: doctorD5RemediationPrefix + feature.Slug,
				})
			}
		}
		revisionsPath := ctx.store.ReconcileRevisionsPath(feature.Slug)
		if _, err := os.Stat(revisionsPath); err != nil {
			if !os.IsNotExist(err) {
				ctx.addFinding(DoctorFinding{
					CheckID:  "D5",
					Code:     "reconcile-revisions-unreadable",
					Severity: "error",
					Feature:  feature.Slug,
					Path:     relOrAbs(ctx.root, revisionsPath),
					Message:  fmt.Sprintf("cannot stat reconcile-revisions.jsonl: %v", err),
					Fixable:  false,
				})
			}
			continue
		}
		valid, corrupt, err := store.LoadReconcileRevisionsLenient(revisionsPath)
		_ = valid
		if err != nil {
			ctx.addFinding(DoctorFinding{
				CheckID:     "D5",
				Code:        "reconcile-revisions-unreadable",
				Severity:    "error",
				Feature:     feature.Slug,
				Path:        relOrAbs(ctx.root, revisionsPath),
				Message:     fmt.Sprintf("cannot read reconcile-revisions.jsonl: %v", err),
				Fixable:     false,
				Remediation: doctorD5RemediationPrefix + feature.Slug,
			})
			continue
		}
		for _, entry := range corrupt {
			ctx.addFinding(DoctorFinding{
				CheckID:     "D5",
				Code:        "reconcile-revisions-malformed",
				Severity:    "drift",
				Feature:     feature.Slug,
				Path:        relOrAbs(ctx.root, revisionsPath),
				Line:        entry.Line,
				Message:     fmt.Sprintf("reconcile-revisions.jsonl line %d is malformed: %s", entry.Line, entry.Error),
				Fixable:     false,
				Remediation: doctorD5RemediationPrefix + feature.Slug,
			})
		}
	}
}

func doctorD5RelevantState(state store.FeatureState) bool {
	switch state {
	case store.StateApplied, store.StateActive, store.StateReconciling, store.StateReconcilingShadow, store.StateBlocked, store.StateUpstreamMerged:
		return true
	default:
		return false
	}
}

func doctorD5ModernReconcileAttempt(status store.FeatureStatus) bool {
	r := status.Reconcile
	return r.AttemptedAt != "" ||
		r.Outcome != "" ||
		r.ReviewVerdict != "" ||
		r.PatchIDMatch != nil ||
		r.ShadowPath != "" ||
		r.ResolveSession != "" ||
		r.ResolvedFiles != 0 ||
		r.FailedFiles != 0 ||
		r.SkippedFiles != 0 ||
		r.UpstreamRef != "" ||
		r.UpstreamCommit != ""
}
