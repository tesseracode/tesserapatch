package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBlockedOutcomeWithoutCategoryRoundTrips(t *testing.T) {
	s, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	st, err := s.AddFeature(AddFeatureInput{Title: "blocked", Slug: "blocked", Request: "blocked"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.featureDir(st.Slug), "status.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	rec := raw["reconcile"].(map[string]any)
	rec["outcome"] = string(ReconcileBlocked)
	delete(rec, "blocked_category")
	delete(rec, "recommended_action")
	data, _ = json.MarshalIndent(raw, "", "  ")
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.LoadFeatureStatus(st.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Reconcile.Outcome != ReconcileBlocked {
		t.Fatalf("got %q", loaded.Reconcile.Outcome)
	}
	if err := s.SaveFeatureStatus(loaded); err != nil {
		t.Fatal(err)
	}
}
