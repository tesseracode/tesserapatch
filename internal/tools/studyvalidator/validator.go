package studyvalidator

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Issue struct {
	File    string `json:"file"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
}
type Counts struct {
	Features int `json:"features"`
	Hunks    int `json:"hunks"`
	Patches  int `json:"patches"`
}
type Report struct {
	Path     string  `json:"path"`
	Errors   []Issue `json:"errors"`
	Warnings []Issue `json:"warnings"`
	Counts   Counts  `json:"counts"`
}

func Validate(path string) (*Report, error) {
	r := &Report{Path: path, Errors: []Issue{}, Warnings: []Issue{}}
	study := map[string]any{}
	metrics := map[string]any{}
	readJSON(r, path, "study.json", &study)
	readJSON(r, path, "metrics.json", &metrics)
	_ = readRequired(r, path, "summary.md")
	studyID, _ := study["study_id"].(string)
	if studyID == "" {
		r.err("study.json", 0, "missing study_id")
	}
	features := readJSONL(r, path, "features.jsonl")
	hunks := readJSONL(r, path, "hunks.jsonl")
	patches := readJSONL(r, path, "patches.jsonl")
	r.Counts = Counts{Features: len(features), Hunks: len(hunks), Patches: len(patches)}
	checkStudyIDs(r, studyID, "features.jsonl", features)
	checkStudyIDs(r, studyID, "hunks.jsonl", hunks)
	checkStudyIDs(r, studyID, "patches.jsonl", patches)
	checkStudyIDMap(r, studyID, "metrics.json", metrics)
	if want, ok := number(study["feature_count"]); ok && int(want) != len(features) {
		r.err("study.json", 0, fmt.Sprintf("feature_count=%d does not match features.jsonl rows=%d", int(want), len(features)))
	}
	checkMetrics(r, metrics, features, hunks, patches)
	checkCorrections(r, path, study, metrics, features)
	return r, nil
}

func readRequired(r *Report, dir, name string) []byte {
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		r.err(name, 0, err.Error())
		return nil
	}
	return b
}
func readJSON(r *Report, dir, name string, out *map[string]any) {
	b := readRequired(r, dir, name)
	if b == nil {
		return
	}
	if err := json.Unmarshal(b, out); err != nil {
		r.err(name, 1, err.Error())
	}
}
func readJSONL(r *Report, dir, name string) []map[string]any {
	f, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		r.err(name, 0, err.Error())
		return nil
	}
	defer f.Close()
	var rows []map[string]any
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		txt := strings.TrimSpace(sc.Text())
		if txt == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(txt), &m); err != nil {
			r.err(name, line, err.Error())
			continue
		}
		rows = append(rows, m)
	}
	if err := sc.Err(); err != nil {
		r.err(name, line, err.Error())
	}
	return rows
}
func (r *Report) err(file string, line int, msg string) {
	r.Errors = append(r.Errors, Issue{File: file, Line: line, Message: msg})
}
func (r *Report) warn(file string, line int, msg string) {
	r.Warnings = append(r.Warnings, Issue{File: file, Line: line, Message: msg})
}
func number(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}
func checkStudyIDs(r *Report, want, file string, rows []map[string]any) {
	for i, row := range rows {
		if got, ok := row["study_id"].(string); ok && want != "" && got != want {
			r.err(file, i+1, fmt.Sprintf("study_id %q does not match %q", got, want))
		}
	}
}
func checkStudyIDMap(r *Report, want, file string, m map[string]any) {
	if got, ok := m["study_id"].(string); ok && want != "" && got != want {
		r.err(file, 0, fmt.Sprintf("study_id %q does not match %q", got, want))
	}
}

func checkMetrics(r *Report, metrics map[string]any, features, hunks, patches []map[string]any) {
	totals, _ := metrics["totals"].(map[string]any)
	if want, ok := number(totals["features_total"]); ok && int(want) != len(features) {
		r.err("metrics.json", 0, fmt.Sprintf("totals.features_total=%d does not match features.jsonl rows=%d", int(want), len(features)))
	}
	if want, ok := number(totals["hunks_total"]); ok && int(want) != len(hunks) {
		r.err("metrics.json", 0, fmt.Sprintf("totals.hunks_total=%d does not match hunks.jsonl rows=%d", int(want), len(hunks)))
	}
	if want, ok := number(totals["patches_total"]); ok && int(want) != len(patches) {
		r.err("metrics.json", 0, fmt.Sprintf("totals.patches_total=%d does not match patches.jsonl rows=%d", int(want), len(patches)))
	}
	gt := map[string]int{}
	for _, f := range features {
		if g, ok := f["ground_truth"].(string); ok && g != "" {
			gt[g]++
		}
	}
	if dist, ok := metrics["ground_truth_distribution"].(map[string]any); ok {
		for k, v := range dist {
			if n, ok := number(v); ok && gt[k] != int(n) {
				r.err("metrics.json", 0, fmt.Sprintf("ground_truth_distribution.%s=%d does not match features.jsonl count=%d", k, int(n), gt[k]))
			}
		}
	}
	if _, hasRaw := metrics["verdicts"]; hasRaw {
		if _, hasPost := metrics["verdict_post_review"]; hasPost { /* explicit distinction present */
		}
	}
	for _, key := range []string{"features_upstreamed", "features_applied", "features_skipped_blocked"} {
		if _, ok := totals[key]; ok {
			r.warn("metrics.json", 0, "aggregate "+key+" should declare phase when compared to raw verdicts or final states")
		}
	}
	if _, ok := metrics["verdicts_phase"]; !ok {
		if _, has := metrics["verdicts"]; has {
			r.warn("metrics.json", 0, "raw verdict counts should declare phase (verdicts_phase)")
		}
	}
}

func checkCorrections(r *Report, dir string, study, metrics map[string]any, features []map[string]any) {
	hasNotes := false
	if _, err := os.Stat(filepath.Join(dir, "local-notes.md")); err == nil {
		hasNotes = true
	}
	corrected := 0
	for _, f := range features {
		rowText := fmt.Sprint(f)
		if gt, _ := f["ground_truth"].(string); strings.Contains(gt, "false_positive") || strings.Contains(gt, "false_negative") || strings.Contains(rowText, "false_positive") || strings.Contains(rowText, "false_negative") {
			corrected++
		}
	}
	if v, ok := metrics["verdict_post_review"].(map[string]any); ok {
		for k, val := range v {
			if strings.Contains(k, "false_positive") || strings.Contains(k, "false_negative") {
				if n, ok := number(val); ok {
					corrected += int(n)
				}
			}
		}
	}
	if corrected == 0 && hasNotes {
		return
	}
	if hasRevisionReference(dir) || hasNotes {
		if !hasNotes {
			r.warn("local-notes.md", 0, "missing local-notes.md; relying on revision-pass references")
		}
		return
	}
	old := false
	if tv, ok := study["tpatch_version"].(string); ok {
		old = legacyStudyVersion(tv)
	}
	if old {
		r.warn("local-notes.md", 0, "missing local-notes.md for old study with corrected verdicts")
	} else {
		r.err("local-notes.md", 0, "missing local-notes.md for study with corrected verdicts")
	}
}
func hasRevisionReference(dir string) bool {
	names := []string{"reconcile-revisions.jsonl", "revision-pass.jsonl"}
	for _, n := range names {
		if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
			return true
		}
	}
	return false
}

func (r *Report) OK() bool { return len(r.Errors) == 0 }
func (r *Report) Sort() {
	sort.Slice(r.Errors, func(i, j int) bool {
		return r.Errors[i].File < r.Errors[j].File || (r.Errors[i].File == r.Errors[j].File && r.Errors[i].Line < r.Errors[j].Line)
	})
	sort.Slice(r.Warnings, func(i, j int) bool {
		return r.Warnings[i].File < r.Warnings[j].File || (r.Warnings[i].File == r.Warnings[j].File && r.Warnings[i].Line < r.Warnings[j].Line)
	})
}

func legacyStudyVersion(v string) bool {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) < 2 || parts[0] != "0" {
		return false
	}
	switch parts[1] {
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		return true
	}
	return false
}
