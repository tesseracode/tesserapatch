package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// v0.12.0 Wave α rev-1 F-SEXT-1 — PRD-feature-supersession §4.1
// (binding label-value contract) + §4.3:178 + ADR-028 D4:58.
//
// The `superseded-by` render token must carry the healthy superseder's
// slug as a composite `superseded-by <slug>` on BOTH the ASCII DAG and
// the `--dag --json` payload. A bare `superseded-by` without the slug
// violates the locked value contract.
func TestStatusDag_SupersededByCarriesSlugText(t *testing.T) {
	tmp, s := newDAGTestRepo(t)
	runCmd("add", "--path", tmp, "--slug", "old-target", "Old target")
	runCmd("add", "--path", tmp, "--slug", "new-replacer", "New replacer")

	// Mark both applied and wire the supersedes edge on the replacer.
	for _, slug := range []string{"old-target", "new-replacer"} {
		f, err := s.LoadFeatureStatus(slug)
		if err != nil {
			t.Fatalf("load %s: %v", slug, err)
		}
		f.State = store.StateApplied
		if err := s.SaveFeatureStatus(f); err != nil {
			t.Fatalf("save %s: %v", slug, err)
		}
	}
	replacer, _ := s.LoadFeatureStatus("new-replacer")
	replacer.DependsOn = []store.Dependency{
		{Slug: "old-target", Kind: store.DependencyKindSupersedes},
	}
	if err := s.SaveFeatureStatus(replacer); err != nil {
		t.Fatalf("save replacer: %v", err)
	}

	out, _, code := runCmd("status", "--dag", "--path", tmp)
	if code != 0 {
		t.Fatalf("exit %d, out=%q", code, out)
	}
	if !strings.Contains(out, "superseded-by new-replacer") {
		t.Fatalf("text output missing composite `superseded-by new-replacer` (F-SEXT-1); got %q", out)
	}
}

// TestStatusDag_SupersededByCarriesSlugJSON — same contract on the
// `--dag --json` payload. A bare `superseded-by` token in the JSON
// `labels` array is a contract violation.
func TestStatusDag_SupersededByCarriesSlugJSON(t *testing.T) {
	tmp, s := newDAGTestRepo(t)
	runCmd("add", "--path", tmp, "--slug", "old-target", "Old target")
	runCmd("add", "--path", tmp, "--slug", "new-replacer", "New replacer")

	for _, slug := range []string{"old-target", "new-replacer"} {
		f, _ := s.LoadFeatureStatus(slug)
		f.State = store.StateApplied
		if err := s.SaveFeatureStatus(f); err != nil {
			t.Fatalf("save %s: %v", slug, err)
		}
	}
	replacer, _ := s.LoadFeatureStatus("new-replacer")
	replacer.DependsOn = []store.Dependency{
		{Slug: "old-target", Kind: store.DependencyKindSupersedes},
	}
	if err := s.SaveFeatureStatus(replacer); err != nil {
		t.Fatal(err)
	}

	out, _, code := runCmd("status", "--dag", "--json", "--path", tmp)
	if code != 0 {
		t.Fatalf("exit %d, out=%q", code, out)
	}
	var payload struct {
		Features []struct {
			Slug   string   `json:"slug"`
			Labels []string `json:"labels"`
		} `json:"features"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	var targetLabels []string
	for _, f := range payload.Features {
		if f.Slug == "old-target" {
			targetLabels = f.Labels
			break
		}
	}
	if targetLabels == nil {
		t.Fatalf("old-target not present in features payload: %s", out)
	}
	foundComposite := false
	for _, l := range targetLabels {
		if l == "superseded-by new-replacer" {
			foundComposite = true
		}
		if l == "superseded-by" {
			t.Fatalf("JSON payload emits bare `superseded-by` without slug — violates F-SEXT-1 / PRD §4.1; labels=%v", targetLabels)
		}
	}
	if !foundComposite {
		t.Fatalf("JSON payload missing composite `superseded-by new-replacer`; labels=%v", targetLabels)
	}
}

// TestStatusDag_SupersessionLabelsSeverityOrder — v0.12.0 rev-1
// F-SEXT-2 + PRD §4.3:184-188 + ADR-028 D4:63-67: within the
// supersession label group, render order MUST be severity-first
// `[stale-superseder] [orphan-superseder] [superseded-by <slug>] [active-superseder]`.
//
// Construct a `combo` node that carries all four labels simultaneously
// and verify the text output preserves the locked positional order.
// A single label group in this order is a sufficient regression guard
// because both text and JSON render paths share the
// DeriveSupersessionLabels ordering + appendLabelPreserveOrder path.
func TestStatusDag_SupersessionLabelsSeverityOrder(t *testing.T) {
	tmp, s := newDAGTestRepo(t)
	runCmd("add", "--path", tmp, "--slug", "target-healthy", "target-healthy")
	runCmd("add", "--path", tmp, "--slug", "combo", "combo")
	runCmd("add", "--path", tmp, "--slug", "peer", "peer")

	// target-healthy applied → active-superseder fires for combo.
	th, _ := s.LoadFeatureStatus("target-healthy")
	th.State = store.StateApplied
	if err := s.SaveFeatureStatus(th); err != nil {
		t.Fatal(err)
	}
	// combo: leave draft → its supersedes edges make it a superseder
	// but it is not healthy → stale-superseder on itself. Wire two
	// supersedes edges: one to target-healthy (active), one to a
	// ghost slug (orphan).
	combo, _ := s.LoadFeatureStatus("combo")
	combo.DependsOn = []store.Dependency{
		{Slug: "target-healthy", Kind: store.DependencyKindSupersedes},
		{Slug: "ghost-target", Kind: store.DependencyKindSupersedes},
	}
	if err := s.SaveFeatureStatus(combo); err != nil {
		t.Fatal(err)
	}
	// peer: healthy superseder of combo → superseded-by peer on combo.
	peer, _ := s.LoadFeatureStatus("peer")
	peer.State = store.StateApplied
	peer.DependsOn = []store.Dependency{
		{Slug: "combo", Kind: store.DependencyKindSupersedes},
	}
	if err := s.SaveFeatureStatus(peer); err != nil {
		t.Fatal(err)
	}

	out, _, code := runCmd("status", "--dag", "--path", tmp)
	if code != 0 {
		t.Fatalf("exit %d, out=%q", code, out)
	}
	// Isolate the combo node's label parenthetical.
	var comboLabels string
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "combo") {
			continue
		}
		lp := strings.Index(line, "(")
		rp := strings.LastIndex(line, ")")
		if lp == -1 || rp == -1 || rp < lp {
			continue
		}
		comboLabels = line[lp+1 : rp]
		break
	}
	if comboLabels == "" {
		t.Fatalf("could not locate combo label parenthetical in output:\n%s", out)
	}
	// Extract just the supersession labels while preserving output
	// order (drop non-supersession labels like never-verified for the
	// order comparison).
	var supersession []string
	for _, tok := range strings.Split(comboLabels, ", ") {
		switch {
		case tok == "stale-superseder",
			tok == "orphan-superseder",
			strings.HasPrefix(tok, "superseded-by "),
			tok == "active-superseder":
			supersession = append(supersession, tok)
		}
	}
	want := []string{
		"stale-superseder",
		"orphan-superseder",
		"superseded-by peer",
		"active-superseder",
	}
	if len(supersession) != len(want) {
		t.Fatalf("expected 4 supersession labels in output, got %v (combo=%q)", supersession, comboLabels)
	}
	for i, tok := range want {
		if supersession[i] != tok {
			t.Fatalf("severity-order violation at position %d (F-SEXT-2 / PRD §4.3):\n got: %v\nwant: %v", i, supersession, want)
		}
	}
}
