package store

import (
	"encoding/json"
	"fmt"
	"testing"
)

func rocS0SummaryBytes(summary ReconcileSummary, want string) error {
	body, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	if string(body) != want {
		return fmt.Errorf("no-candidate summary changed: got %s, want %s", body, want)
	}
	return nil
}

func TestROCS0NoCandidateSummarySerialization(t *testing.T) {
	cases := []struct {
		name    string
		summary ReconcileSummary
		want    string
	}{
		{"empty", ReconcileSummary{}, `{}`},
		{"blocked", ReconcileSummary{Outcome: ReconcileBlocked}, `{"outcome":"blocked"}`},
		{"confirmed", ReconcileSummary{
			AttemptedAt: "2000-01-01T00:00:00Z", UpstreamRef: "origin/main", UpstreamCommit: "upstream",
			Outcome: ReconcileUpstreamed, ReviewVerdict: "confirmed-upstreamed",
		}, `{"attempted_at":"2000-01-01T00:00:00Z","upstream_ref":"origin/main","upstream_commit":"upstream","outcome":"upstreamed","review_verdict":"confirmed-upstreamed"}`},
		{"shadow", ReconcileSummary{
			Outcome: ReconcileShadowAwaiting, ShadowPath: "shadow", ResolveSession: "session",
			ResolvedFiles: 1, FailedFiles: 2, SkippedFiles: 3,
		}, `{"outcome":"shadow-awaiting","shadow_path":"shadow","resolve_session_id":"session","resolved_files":1,"failed_files":2,"skipped_files":3}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := rocS0SummaryBytes(tc.summary, tc.want); err != nil {
				t.Fatal(err)
			}
			var decoded ReconcileSummary
			if err := json.Unmarshal([]byte(tc.want), &decoded); err != nil {
				t.Fatal(err)
			}
			if err := rocS0SummaryBytes(decoded, tc.want); err != nil {
				t.Fatal(err)
			}
			for _, mutant := range []string{"", tc.want + "\n", `{"operation_candidate":null}`} {
				if err := rocS0SummaryBytes(tc.summary, mutant); err == nil {
					t.Fatalf("summary byte guard accepted %q", mutant)
				}
			}
		})
	}
}
