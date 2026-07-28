package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/safety"
	"github.com/tesseracode/tesserapatch/internal/store"
)

const DoctorReportSchemaVersion = 1

type DoctorOptions struct {
	DryRun bool
	Fix    bool
	Checks []string
}

type DoctorReport struct {
	SchemaVersion int                 `json:"schema_version"`
	Command       string              `json:"command"`
	DryRun        bool                `json:"dry_run"`
	Fix           bool                `json:"fix"`
	Summary       DoctorSummary       `json:"summary"`
	Findings      []DoctorFinding     `json:"findings"`
	Checks        []DoctorCheckStatus `json:"checks"`
}

type DoctorSummary struct {
	ChecksRun int `json:"checks_run"`
	Findings  int `json:"findings"`
	Warnings  int `json:"warnings"`
	Fixed     int `json:"fixed"`
	Errors    int `json:"errors"`
}

type DoctorFinding struct {
	CheckID     string `json:"check_id"`
	Code        string `json:"code"`
	Severity    string `json:"severity"`
	Feature     string `json:"feature,omitempty"`
	Path        string `json:"path,omitempty"`
	Line        int    `json:"line,omitempty"`
	Field       string `json:"field,omitempty"`
	Message     string `json:"message"`
	Fixable     bool   `json:"fixable"`
	Remediation string `json:"remediation,omitempty"`
	BackupPath  string `json:"backup_path,omitempty"`
}

type DoctorCheckStatus struct {
	CheckID string `json:"check_id"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

type DoctorHardInvariantError struct {
	Message string
}

func (e *DoctorHardInvariantError) Error() string { return e.Message }

type doctorCheck struct {
	id  string
	run func(*doctorContext)
}

type doctorContext struct {
	store    *store.Store
	root     string
	options  DoctorOptions
	report   *DoctorReport
	features []doctorFeature
}

type doctorFeature struct {
	Slug       string
	Status     *store.FeatureStatus
	StatusErr  error
	StatusPath string
	Dir        string
}

func DoctorCheckIDs() []string {
	ids := make([]string, 0, len(doctorRegistry()))
	for _, check := range doctorRegistry() {
		ids = append(ids, check.id)
	}
	return ids
}

func ValidateDoctorCheckIDs(ids []string) error {
	_, err := selectDoctorChecks(ids)
	return err
}

func RunDoctor(s *store.Store, options DoctorOptions) (DoctorReport, error) {
	if !options.Fix {
		options.DryRun = true
	}
	if options.DryRun && options.Fix {
		return DoctorReport{}, fmt.Errorf("doctor: --dry-run and --fix cannot be used together")
	}
	selected, err := selectDoctorChecks(options.Checks)
	if err != nil {
		return DoctorReport{}, err
	}
	features, err := validateDoctorWorkspace(s)
	if err != nil {
		return DoctorReport{}, &DoctorHardInvariantError{Message: err.Error()}
	}
	report := DoctorReport{
		SchemaVersion: DoctorReportSchemaVersion,
		Command:       "doctor",
		DryRun:        options.DryRun,
		Fix:           options.Fix,
		Findings:      []DoctorFinding{},
		Checks:        []DoctorCheckStatus{},
	}
	ctx := &doctorContext{
		store:    s,
		root:     s.Root,
		options:  options,
		report:   &report,
		features: features,
	}
	for _, check := range selected {
		beforeErrors := report.Summary.Errors
		func() {
			defer func() {
				if r := recover(); r != nil {
					ctx.addFinding(DoctorFinding{
						CheckID:  check.id,
						Code:     "check-error",
						Severity: "error",
						Message:  fmt.Sprintf("check panicked: %v", r),
						Fixable:  false,
					})
				}
			}()
			check.run(ctx)
		}()
		status := "clean"
		if report.Summary.Errors > beforeErrors {
			status = "error"
		}
		report.Checks = append(report.Checks, DoctorCheckStatus{CheckID: check.id, Status: status})
		report.Summary.ChecksRun++
	}
	sortDoctorReport(&report)
	return report, nil
}

func WriteDoctorJSON(w io.Writer, report DoctorReport) error {
	sortDoctorReport(&report)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteDoctorHuman(w io.Writer, report DoctorReport) {
	sortDoctorReport(&report)
	for _, f := range report.Findings {
		level := strings.ToUpper(f.Severity)
		fmt.Fprintf(w, "%s  %s %s", level, f.CheckID, f.Code)
		if f.Feature != "" {
			fmt.Fprintf(w, "  feature=%s", f.Feature)
		}
		if f.Path != "" {
			fmt.Fprintf(w, "  path=%s", f.Path)
		}
		if f.Line > 0 {
			fmt.Fprintf(w, ":%d", f.Line)
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, "       %s\n", f.Message)
		if f.Remediation != "" {
			fmt.Fprintf(w, "       remediation: %s\n", f.Remediation)
		}
		if f.BackupPath != "" {
			fmt.Fprintf(w, "       backup: %s\n", f.BackupPath)
		}
	}
	fmt.Fprintf(w, "summary: %d drift findings, %d warnings, %d fixed, %d errors\n",
		report.Summary.Findings, report.Summary.Warnings, report.Summary.Fixed, report.Summary.Errors)
}

func DoctorExitCode(report DoctorReport) int {
	if report.Summary.Errors == 0 && report.Summary.Findings == 0 {
		return 0
	}
	if report.Fix && report.Summary.Errors > 0 {
		return 2
	}
	return 1
}

func BackupPathForOverwrite(path string) string {
	return path + ".orig"
}

func EnsureDoctorBackup(root, path string) (string, error) {
	backup := BackupPathForOverwrite(path)
	if err := safety.EnsureSafeRepoPath(root, backup); err != nil {
		return "", err
	}
	if _, err := os.Stat(backup); err == nil {
		return "", fmt.Errorf("backup target already exists: %s", backup)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return backup, nil
}

func doctorRegistry() []doctorCheck {
	return []doctorCheck{
		{id: "D1", run: runDoctorD1},
		{id: "D2", run: runDoctorD2},
		{id: "D3", run: runDoctorD3},
		{id: "D4", run: runDoctorD4},
		{id: "D5", run: runDoctorD5},
		{id: "D7", run: runDoctorD7},
		{id: "D8", run: runDoctorD8},
	}
}

func selectDoctorChecks(ids []string) ([]doctorCheck, error) {
	registry := doctorRegistry()
	byID := map[string]doctorCheck{}
	for _, check := range registry {
		byID[check.id] = check
	}
	if len(ids) == 0 {
		return registry, nil
	}
	seen := map[string]bool{}
	var selected []doctorCheck
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		check, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("unknown doctor check %q (known: %s)", raw, strings.Join(DoctorCheckIDs(), ", "))
		}
		if !seen[id] {
			selected = append(selected, check)
			seen[id] = true
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].id < selected[j].id })
	return selected, nil
}

func validateDoctorWorkspace(s *store.Store) ([]doctorFeature, error) {
	if s == nil {
		return nil, fmt.Errorf("doctor: nil store")
	}
	tpatchDir := filepath.Join(s.Root, ".tpatch")
	if info, err := os.Stat(tpatchDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("doctor: missing workspace root %s", relOrAbs(s.Root, tpatchDir))
		}
		return nil, fmt.Errorf("doctor: cannot stat workspace root %s: %w", relOrAbs(s.Root, tpatchDir), err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("doctor: workspace root %s is not a directory", relOrAbs(s.Root, tpatchDir))
	}
	featuresDir := filepath.Join(tpatchDir, "features")
	if err := safety.EnsureSafeRepoPath(s.Root, featuresDir); err != nil {
		return nil, fmt.Errorf("doctor: unsafe features path: %w", err)
	}
	entries, err := os.ReadDir(featuresDir)
	if err != nil {
		return nil, fmt.Errorf("doctor: cannot list feature directories: %w", err)
	}
	var out []doctorFeature
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		slug := entry.Name()
		dir := filepath.Join(featuresDir, slug)
		statusPath := filepath.Join(dir, "status.json")
		if err := safety.EnsureSafeRepoPath(s.Root, dir); err != nil {
			return nil, fmt.Errorf("doctor: unsafe feature path for %q: %w", slug, err)
		}
		if _, err := os.Stat(statusPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			out = append(out, doctorFeature{Slug: slug, StatusErr: fmt.Errorf("failed to stat status.json: %w", err), StatusPath: statusPath, Dir: dir})
			continue
		}
		status, err := s.LoadFeatureStatus(slug)
		if err != nil {
			out = append(out, doctorFeature{Slug: slug, StatusErr: err, StatusPath: statusPath, Dir: dir})
			continue
		}
		st := status
		out = append(out, doctorFeature{Slug: slug, Status: &st, StatusPath: statusPath, Dir: dir})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

func (ctx *doctorContext) addFinding(f DoctorFinding) {
	if f.Severity == "" {
		f.Severity = "drift"
	}
	if f.CheckID == "" {
		f.CheckID = "D8"
	}
	ctx.report.Findings = append(ctx.report.Findings, f)
	switch f.Severity {
	case "warning", "warn":
		ctx.report.Summary.Warnings++
	case "fixed":
		ctx.report.Summary.Fixed++
	case "error":
		ctx.report.Summary.Errors++
	default:
		ctx.report.Summary.Findings++
	}
}

func sortDoctorReport(report *DoctorReport) {
	sort.Slice(report.Findings, func(i, j int) bool {
		a, b := report.Findings[i], report.Findings[j]
		return doctorFindingSortKey(a) < doctorFindingSortKey(b)
	})
	sort.Slice(report.Checks, func(i, j int) bool { return report.Checks[i].CheckID < report.Checks[j].CheckID })
}

func doctorFindingSortKey(f DoctorFinding) string {
	return strings.Join([]string{f.CheckID, f.Feature, f.Path, fmt.Sprintf("%09d", f.Line), f.Code, f.Message}, "\x00")
}

func relOrAbs(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." {
		return filepath.ToSlash(rel)
	}
	return path
}
