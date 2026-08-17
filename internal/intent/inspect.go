package intent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
)

const (
	abortSlugUnsafe          = "slug-unsafe"
	abortUnsupportedPlatform = "workspace-unsupported-platform"
	abortWorkspaceMissing    = "workspace-not-initialized"
	abortRootUnopenable      = "workspace-root-unopenable"
	abortFeatureUnsafe       = "feature-dir-unsafe"
	abortFeatureNotFound     = "feature-not-found"
	abortStatusSymlink       = "status-symlink-refused"
	abortStatusNotRegular    = "status-not-regular"
	abortStatusOversize      = "status-oversize"
	abortStatusUnreadable    = "status-unreadable"
	abortStatusUnstable      = "status-unstable"
	abortStatusMalformed     = "status-malformed"
	abortStatusInvalidState  = "status-invalid-state"
)

const (
	readinessReady         = "ready"
	readinessNotReady      = "not_ready"
	readinessIndeterminate = "indeterminate"
)

// Artifact is one stable, structural artifact result.
type Artifact struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Role        string `json:"role"`
	State       string `json:"state"`
	ReasonCode  string `json:"reason_code"`
	Provenance  string `json:"provenance"`
	Remediation string `json:"remediation"`
}

// Overall contains the stable readiness counters.
type Overall struct {
	StructuralReadiness string `json:"structural_readiness"`
	RequiredTotal       int    `json:"required_total"`
	RequiredSatisfied   int    `json:"required_satisfied"`
	OptionalTotal       int    `json:"optional_total"`
	OptionalSatisfied   int    `json:"optional_satisfied"`
}

// Advisory is a neutral report note selected from a closed catalog.
type Advisory struct {
	Code       string `json:"code"`
	ArtifactID string `json:"artifact_id"`
	Message    string `json:"message"`
}

// Abort is the single pre-artifact inspection refusal.
type Abort struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Report is the version-one prepare --check wire schema. Struct declaration
// order is its JSON emission order.
type Report struct {
	SchemaVersion int        `json:"schema_version"`
	Command       string     `json:"command"`
	Slug          string     `json:"slug"`
	FeatureState  string     `json:"feature_state"`
	Disclaimer    string     `json:"disclaimer"`
	Artifacts     []Artifact `json:"artifacts"`
	Overall       Overall    `json:"overall"`
	Advisories    []Advisory `json:"advisories"`
	Abort         *Abort     `json:"abort,omitempty"`
}

type artifactSpec struct {
	id       string
	relative string
	display  string
	role     string
	sidecar  bool
}

var artifactSpecs = [...]artifactSpec{
	{id: "analysis", relative: "analysis.md", display: "analysis.md", role: "required"},
	{id: "spec", relative: "spec.md", display: "spec.md", role: "required"},
	{id: "exploration", relative: "exploration.md", display: "exploration.md", role: "required"},
	{id: "analysis_sidecar", relative: "artifacts/analysis.json", display: "artifacts/analysis.json", role: "optional", sidecar: true},
}

// CanonicalSlug accepts the closed tpatch feature-slug grammar without
// rewriting it.
func CanonicalSlug(raw string) (string, error) {
	if len(raw) == 0 || len(raw) > 60 || windowsReserved(raw) {
		return "", errors.New("not a canonical tpatch slug")
	}
	segmentStart := true
	for _, b := range []byte(raw) {
		if b == '-' {
			if segmentStart {
				return "", errors.New("not a canonical tpatch slug")
			}
			segmentStart = true
			continue
		}
		if !(b >= 'a' && b <= 'z' || b >= '0' && b <= '9') {
			return "", errors.New("not a canonical tpatch slug")
		}
		segmentStart = false
	}
	if segmentStart {
		return "", errors.New("not a canonical tpatch slug")
	}
	return raw, nil
}

func windowsReserved(slug string) bool {
	upper := strings.ToUpper(slug)
	switch upper {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return true
	default:
		return false
	}
}

// NewAbortReport constructs the complete, no-artifact abort shape.
func NewAbortReport(slug, code string) Report {
	if code == abortSlugUnsafe {
		slug = ""
	}
	return Report{
		SchemaVersion: 1,
		Command:       "prepare --check",
		Slug:          slug,
		FeatureState:  "unknown",
		Disclaimer:    disclaimer,
		Artifacts:     []Artifact{},
		Overall: Overall{
			StructuralReadiness: readinessIndeterminate,
			RequiredTotal:       3,
			RequiredSatisfied:   0,
			OptionalTotal:       1,
			OptionalSatisfied:   0,
		},
		Advisories: []Advisory{},
		Abort:      &Abort{Code: code, Message: abortMessage(code, slug)},
	}
}

// Inspect classifies the complete intent bundle without mutating the rooted
// tree. The caller owns ops and the one shared scratch allocation.
func Inspect(ops RootOps, canonicalSlug string, scratch []byte) Report {
	if _, err := CanonicalSlug(canonicalSlug); err != nil {
		return NewAbortReport("", abortSlugUnsafe)
	}
	if len(scratch) != MaxArtifactBytes+1 {
		return NewAbortReport(canonicalSlug, abortRootUnopenable)
	}

	base := featureBase(canonicalSlug)
	if code := inspectFeatureDirectory(ops, canonicalSlug); code != "" {
		return NewAbortReport(canonicalSlug, code)
	}

	statusName := base + "/status.json"
	status := capture(ops, statusName, nil, scratch[:MaxStatusBytes+1], MaxStatusBytes)
	featureState := "unknown"
	statusAbsent := false
	switch status.state {
	case StatePresentNonempty:
		var document struct {
			State string `json:"state"`
		}
		if !jsonObject(status.bytes) || json.Unmarshal(status.bytes, &document) != nil {
			return NewAbortReport(canonicalSlug, abortStatusMalformed)
		}
		if !validFeatureState(document.State) {
			return NewAbortReport(canonicalSlug, abortStatusInvalidState)
		}
		featureState = document.State
	case StateAbsent:
		statusAbsent = true
	case StatePresentEmpty:
		return NewAbortReport(canonicalSlug, abortStatusMalformed)
	case StateSymlinkRefused:
		return NewAbortReport(canonicalSlug, abortStatusSymlink)
	case StateNotRegular:
		return NewAbortReport(canonicalSlug, abortStatusNotRegular)
	case StateOversize:
		return NewAbortReport(canonicalSlug, abortStatusOversize)
	case StateUnstable:
		return NewAbortReport(canonicalSlug, abortStatusUnstable)
	default:
		return NewAbortReport(canonicalSlug, abortStatusUnreadable)
	}

	report := Report{
		SchemaVersion: 1,
		Command:       "prepare --check",
		Slug:          canonicalSlug,
		FeatureState:  featureState,
		Disclaimer:    disclaimer,
		Artifacts:     make([]Artifact, 0, len(artifactSpecs)),
		Advisories:    make([]Advisory, 0, 3),
		Overall: Overall{
			RequiredTotal: 3,
			OptionalTotal: 1,
		},
	}
	if statusAbsent {
		report.Advisories = append(report.Advisories, Advisory{
			Code:       "feature-state-absent",
			ArtifactID: "",
			Message:    "This feature directory has no status.json, so the lifecycle state is reported as unknown. Artifact inspection is unaffected: no artifact classification reads status.json.",
		})
	}

	for _, spec := range artifactSpecs {
		name := base + "/" + spec.relative
		components := []string(nil)
		if spec.sidecar {
			components = []string{base + "/artifacts"}
		}
		result := capture(ops, name, components, scratch, MaxArtifactBytes)
		state := result.state
		if spec.sidecar && state == StatePresentNonempty {
			if !json.Valid(result.bytes) {
				state = StateInvalidStructured
				result.structuredReason = "sidecar-not-json"
			} else if !jsonObject(result.bytes) {
				state = StateInvalidStructured
				result.structuredReason = "sidecar-not-json-object"
			}
		}
		reason := reasonCode(state, spec.sidecar, result.structuredReason)
		artifact := Artifact{
			ID:          spec.id,
			Path:        name,
			Role:        spec.role,
			State:       state,
			ReasonCode:  reason,
			Provenance:  "unknown",
			Remediation: remediation(spec, name, canonicalSlug, state),
		}
		report.Artifacts = append(report.Artifacts, artifact)
		if spec.role == "required" && state == StatePresentNonempty {
			report.Overall.RequiredSatisfied++
		}
		if spec.role == "optional" && state == StatePresentNonempty {
			report.Overall.OptionalSatisfied++
		}
		if spec.sidecar {
			if advisory := sidecarAdvisory(state); advisory != nil {
				report.Advisories = append(report.Advisories, *advisory)
			}
		}
	}

	report.Advisories = append(report.Advisories, Advisory{
		Code:       "provenance-unknown-by-design",
		ArtifactID: "",
		Message:    "Per-artifact provenance is reported as unknown for every artifact. tpatch does not yet persist durable per-artifact source metadata.",
	})
	report.Overall.StructuralReadiness = readinessFor(report.Artifacts)
	return report
}

func featureBase(slug string) string {
	return ".tpatch/features/" + slug
}

func inspectFeatureDirectory(ops RootOps, slug string) string {
	for _, name := range []string{".tpatch", ".tpatch/features", featureBase(slug)} {
		mustValidRootName(name)
		info, err := ops.Lstat(name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return abortFeatureNotFound
			}
			return abortFeatureUnsafe
		}
		if refused(info) || !info.IsDir() {
			return abortFeatureUnsafe
		}
	}
	return ""
}

type captureResult struct {
	state            string
	bytes            []byte
	structuredReason string
}

func capture(ops RootOps, name string, components []string, buffer []byte, limit int) captureResult {
	mustValidRootName(name)
	for _, component := range components {
		mustValidRootName(component)
		info, err := ops.Lstat(component)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return captureResult{state: StateAbsent}
			}
			return captureResult{state: StateUnreadable}
		}
		if refused(info) {
			return captureResult{state: StateSymlinkRefused}
		}
		if !info.IsDir() {
			return captureResult{state: StateUnreadable}
		}
	}

	pre, err := ops.Lstat(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return captureResult{state: StateAbsent}
		}
		return captureResult{state: StateUnreadable}
	}
	if refused(pre) {
		return captureResult{state: StateSymlinkRefused}
	}
	if !pre.Mode().IsRegular() {
		return captureResult{state: StateNotRegular}
	}
	if pre.Size() > int64(limit) {
		return captureResult{state: StateOversize}
	}

	file, err := ops.OpenFile(name, os.O_RDONLY|openFlags(), 0)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return captureResult{state: StateUnstable}
		}
		return captureResult{state: StateUnreadable}
	}

	result := captureOpen(ops, file, pre, components, buffer, limit)
	return result
}

func captureOpen(ops RootOps, file FileOps, pre fs.FileInfo, components []string, buffer []byte, limit int) (result captureResult) {
	state := ""
	var data []byte
	post, err := file.Stat()
	if err != nil {
		state = StateUnreadable
	} else if !ops.SameFile(pre, post) {
		state = StateUnstable
	} else if !post.Mode().IsRegular() {
		state = StateUnstable
	} else if post.Size() != pre.Size() {
		state = StateUnstable
	}

	if state == "" {
		var n int
		n, err = io.ReadFull(file, buffer[:limit+1])
		switch {
		case err == nil:
			state = StateUnstable
		case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
			data = buffer[:n]
			if int64(n) != post.Size() {
				state = StateUnstable
			}
		default:
			state = StateUnreadable
		}
	}

	if state == "" {
		postRead, statErr := file.Stat()
		if statErr != nil {
			state = StateUnreadable
		} else if postRead.Size() != post.Size() {
			state = StateUnstable
		}
	}

	if state == "" {
		for _, component := range components {
			mustValidRootName(component)
			info, walkErr := ops.Lstat(component)
			if walkErr != nil {
				if errors.Is(walkErr, fs.ErrNotExist) {
					state = StateUnstable
				} else {
					state = StateUnreadable
				}
				break
			}
			if refused(info) || !info.IsDir() {
				state = StateUnstable
				break
			}
		}
	}

	if closeErr := file.Close(); closeErr != nil && state == "" {
		state = StateUnreadable
	}
	if state != "" {
		return captureResult{state: state}
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return captureResult{state: StatePresentEmpty, bytes: data}
	}
	return captureResult{state: StatePresentNonempty, bytes: data}
}

func refused(info fs.FileInfo) bool {
	return info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0
}

func mustValidRootName(name string) {
	if !fs.ValidPath(name) {
		panic(fmt.Sprintf("invalid rooted inspection name %q", name))
	}
}

func jsonObject(data []byte) bool {
	if !json.Valid(data) {
		return false
	}
	trimmed := bytes.TrimSpace(data)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

func readinessFor(artifacts []Artifact) string {
	for _, artifact := range artifacts {
		if artifact.Role == "required" && artifact.State == StateUnstable {
			return readinessIndeterminate
		}
	}
	for _, artifact := range artifacts {
		if artifact.Role == "required" && artifact.State != StatePresentNonempty {
			return readinessNotReady
		}
	}
	return readinessReady
}

func reasonCode(state string, sidecar bool, structuredReason string) string {
	switch state {
	case StatePresentNonempty:
		return ""
	case StatePresentEmpty:
		return "artifact-empty"
	case StateAbsent:
		return "artifact-absent"
	case StateSymlinkRefused:
		return "artifact-symlink-refused"
	case StateNotRegular:
		return "artifact-not-regular"
	case StateUnreadable:
		return "artifact-unreadable"
	case StateOversize:
		return "artifact-oversize"
	case StateInvalidStructured:
		return structuredReason
	case StateUnstable:
		return "artifact-snapshot-unstable"
	default:
		return "artifact-unreadable"
	}
}

func remediation(spec artifactSpec, path, slug, state string) string {
	if spec.role != "required" || state == StatePresentNonempty {
		return ""
	}
	switch state {
	case StatePresentEmpty:
		return fmt.Sprintf("Author %s with non-whitespace content, then re-run tpatch prepare %s --check.", path, slug)
	case StateAbsent:
		return fmt.Sprintf("Author %s, then re-run tpatch prepare %s --check.", path, slug)
	case StateSymlinkRefused, StateNotRegular:
		return fmt.Sprintf("Replace %s with a regular file, then re-run tpatch prepare %s --check.", path, slug)
	case StateUnreadable:
		return fmt.Sprintf("Make %s readable as a regular file, then re-run tpatch prepare %s --check.", path, slug)
	case StateOversize:
		return fmt.Sprintf("Reduce %s below the 4 MiB inspection limit, then re-run tpatch prepare %s --check.", path, slug)
	case StateUnstable:
		return "Re-run when no other tpatch process is writing this feature."
	default:
		return ""
	}
}

func sidecarAdvisory(state string) *Advisory {
	var code, message string
	switch state {
	case StatePresentNonempty:
		return nil
	case StateAbsent:
		code, message = "analysis-sidecar-absent-path-b-normal", "artifacts/analysis.json is written by the CLI-driven analyze phase and is not produced by analyze --manual. Its absence is not a defect."
	case StatePresentEmpty:
		code, message = "analysis-sidecar-empty", "artifacts/analysis.json exists but contains no non-whitespace bytes. This is not a readiness input; the file can be regenerated by re-running the analyze phase or removed."
	case StateInvalidStructured:
		code, message = "analysis-sidecar-invalid-structured", "artifacts/analysis.json exists but is not a JSON object. This is not a readiness input; the file can be regenerated by re-running the analyze phase or removed."
	case StateUnstable:
		code, message = "analysis-sidecar-unstable", "artifacts/analysis.json changed while it was being inspected, so its state was not determined. This is not a readiness input; re-run when no other tpatch process is writing this feature."
	case StateSymlinkRefused:
		code, message = "analysis-sidecar-symlink-refused", "artifacts/analysis.json is a symbolic link and was not followed or read. This is not a readiness input; replace it with a regular file or remove it."
	case StateNotRegular:
		code, message = "analysis-sidecar-not-regular", "artifacts/analysis.json exists but is not a regular file, so it was not read. This is not a readiness input."
	case StateUnreadable:
		code, message = "analysis-sidecar-unreadable", "artifacts/analysis.json could not be inspected. This is not a readiness input; check the file's permissions or remove it."
	case StateOversize:
		code, message = "analysis-sidecar-oversize", "artifacts/analysis.json exceeds the 4 MiB inspection limit and was not read. This is not a readiness input; inspect it manually."
	}
	if code == "" {
		return nil
	}
	return &Advisory{Code: code, ArtifactID: "analysis_sidecar", Message: message}
}

func abortMessage(code, slug string) string {
	switch code {
	case abortSlugUnsafe:
		return "the requested feature name is not a canonical tpatch slug. Canonical slugs are lowercase letters, digits and single dashes, 1-60 bytes. Create features with tpatch add, or rename a hand-made feature directory under .tpatch/features/ to a canonical name."
	case abortUnsupportedPlatform:
		return "this build of tpatch cannot guarantee that artifact inspection stays inside the repository on this platform, so prepare --check refuses to run here. Inspect the files under .tpatch/features/ directly."
	case abortWorkspaceMissing:
		return "no tpatch workspace was found here or in any parent directory. Run tpatch init in the repository root, or pass --path with the repository directory."
	case abortRootUnopenable:
		return "the repository root could not be opened for inspection. Check that the directory still exists and is readable, then re-run."
	case abortFeatureUnsafe:
		return fmt.Sprintf(".tpatch/features/%s could not be inspected safely: a directory on the way to it is a symbolic link, a reparse point, or not a directory. Replace it with a real directory, or inspect the feature by hand.", slug)
	case abortFeatureNotFound:
		return fmt.Sprintf("no feature directory exists at .tpatch/features/%s. Run tpatch status to list the features in this workspace.", slug)
	case abortStatusSymlink:
		return fmt.Sprintf(".tpatch/features/%s/status.json is a symbolic link or reparse point and was not followed. Replace it with a regular file, then run tpatch doctor.", slug)
	case abortStatusNotRegular:
		return fmt.Sprintf(".tpatch/features/%s/status.json is not a regular file and was not read. Replace it with a regular file, then run tpatch doctor.", slug)
	case abortStatusOversize:
		return fmt.Sprintf(".tpatch/features/%s/status.json exceeds the 1 MiB inspection limit and was not read. Inspect it by hand, then run tpatch doctor.", slug)
	case abortStatusUnreadable:
		return fmt.Sprintf(".tpatch/features/%s/status.json could not be read and closed cleanly, so the lifecycle state was not determined. Check the file's permissions and the filesystem it lives on, then run tpatch doctor.", slug)
	case abortStatusUnstable:
		return fmt.Sprintf(".tpatch/features/%s/status.json changed while it was being read, so the lifecycle state could not be determined. Re-run when no other tpatch process is writing this feature.", slug)
	case abortStatusMalformed:
		return fmt.Sprintf(".tpatch/features/%s/status.json was read but is not a valid tpatch status document. Run tpatch doctor to inspect and repair the workspace metadata.", slug)
	case abortStatusInvalidState:
		return fmt.Sprintf(".tpatch/features/%s/status.json was read but records a lifecycle state this version of tpatch does not recognise. Upgrade tpatch, or run tpatch doctor to inspect the workspace metadata.", slug)
	default:
		return "artifact inspection could not be completed."
	}
}

// AbortCode returns the report's closed abort code, if it has one.
func (r Report) AbortCode() string {
	if r.Abort == nil {
		return ""
	}
	return r.Abort.Code
}

// Readiness returns the report's closed structural-readiness value.
func (r Report) Readiness() string {
	return r.Overall.StructuralReadiness
}
