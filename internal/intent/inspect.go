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

// AbortCode is the closed catalog of pre-artifact inspection refusals
// (§9.4.4). The zero value is deliberately not a member: a report either
// carries one of the thirteen codes below or carries no abort at all.
type AbortCode string

const (
	AbortSlugUnsafe          AbortCode = "slug-unsafe"
	AbortUnsupportedPlatform AbortCode = "workspace-unsupported-platform"
	AbortWorkspaceMissing    AbortCode = "workspace-not-initialized"
	AbortRootUnopenable      AbortCode = "workspace-root-unopenable"
	AbortFeatureUnsafe       AbortCode = "feature-dir-unsafe"
	AbortFeatureNotFound     AbortCode = "feature-not-found"
	AbortStatusSymlink       AbortCode = "status-symlink-refused"
	AbortStatusNotRegular    AbortCode = "status-not-regular"
	AbortStatusOversize      AbortCode = "status-oversize"
	AbortStatusUnreadable    AbortCode = "status-unreadable"
	AbortStatusUnstable      AbortCode = "status-unstable"
	AbortStatusMalformed     AbortCode = "status-malformed"
	AbortStatusInvalidState  AbortCode = "status-invalid-state"
)

// AbortCodes returns the thirteen §9.4.4 codes in declaration order. The
// catalog is closed: adding a fourteenth code without a §9.4.5 message and a
// §10.5.1 lifecycle line fails AVP-101, AVP-153 and AVP-181.
func AbortCodes() []AbortCode {
	return []AbortCode{
		AbortSlugUnsafe,
		AbortUnsupportedPlatform,
		AbortWorkspaceMissing,
		AbortRootUnopenable,
		AbortFeatureUnsafe,
		AbortFeatureNotFound,
		AbortStatusSymlink,
		AbortStatusNotRegular,
		AbortStatusOversize,
		AbortStatusUnreadable,
		AbortStatusUnstable,
		AbortStatusMalformed,
		AbortStatusInvalidState,
	}
}

// Valid reports membership in the closed catalog.
func (c AbortCode) Valid() bool {
	for _, code := range AbortCodes() {
		if code == c {
			return true
		}
	}
	return false
}

// Readiness is the closed structural-readiness verdict domain (§9.1).
type Readiness string

const (
	ReadinessReady         Readiness = "ready"
	ReadinessNotReady      Readiness = "not_ready"
	ReadinessIndeterminate Readiness = "indeterminate"
)

// Reason codes (§10.3), closed and total over the nine artifact states.
const (
	ReasonArtifactEmpty            = "artifact-empty"
	ReasonArtifactAbsent           = "artifact-absent"
	ReasonArtifactSymlinkRefused   = "artifact-symlink-refused"
	ReasonArtifactNotRegular       = "artifact-not-regular"
	ReasonArtifactUnreadable       = "artifact-unreadable"
	ReasonArtifactOversize         = "artifact-oversize"
	ReasonArtifactUnstable         = "artifact-snapshot-unstable"
	ReasonSidecarNotJSON           = "sidecar-not-json"
	ReasonSidecarNotJSONObject     = "sidecar-not-json-object"
	AdvisoryFeatureStateAbsent     = "feature-state-absent"
	AdvisoryProvenanceUnknown      = "provenance-unknown-by-design"
	AdvisorySidecarAbsent          = "analysis-sidecar-absent-path-b-normal"
	AdvisorySidecarEmpty           = "analysis-sidecar-empty"
	AdvisorySidecarInvalid         = "analysis-sidecar-invalid-structured"
	AdvisorySidecarUnstable        = "analysis-sidecar-unstable"
	AdvisorySidecarSymlinkRefused  = "analysis-sidecar-symlink-refused"
	AdvisorySidecarNotRegular      = "analysis-sidecar-not-regular"
	AdvisorySidecarUnreadable      = "analysis-sidecar-unreadable"
	AdvisorySidecarOversize        = "analysis-sidecar-oversize"
	ProvenanceUnknown              = "unknown"
	FeatureStateUnknown            = "unknown"
	RoleRequired                   = "required"
	RoleOptional                   = "optional"
	CommandName                    = "prepare --check"
	sidecarAdvisoryPrefix          = "analysis-sidecar-"
	scratchLengthInvariantMessage  = "intent.Inspect: scratch buffer length must be exactly MaxArtifactBytes+1"
	rootNameInvariantMessagePrefix = "invalid rooted inspection name "
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
	StructuralReadiness Readiness `json:"structural_readiness"`
	RequiredTotal       int       `json:"required_total"`
	RequiredSatisfied   int       `json:"required_satisfied"`
	OptionalTotal       int       `json:"optional_total"`
	OptionalSatisfied   int       `json:"optional_satisfied"`
}

// Advisory is a neutral report note selected from a closed catalog.
type Advisory struct {
	Code       string `json:"code"`
	ArtifactID string `json:"artifact_id"`
	Message    string `json:"message"`
}

// Abort is the single pre-artifact inspection refusal.
type Abort struct {
	Code    AbortCode `json:"code"`
	Message string    `json:"message"`
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

// NewAbortReport constructs the complete, no-artifact abort shape. The code
// must be a member of the closed §9.4.4 catalog; anything else is a
// programming error and panics rather than emitting an untyped diagnostic.
func NewAbortReport(slug string, code AbortCode) Report {
	if !code.Valid() {
		panic("intent.NewAbortReport: abort code outside the closed catalog: " + string(code))
	}
	if code == AbortSlugUnsafe {
		slug = ""
	}
	return Report{
		SchemaVersion: 1,
		Command:       CommandName,
		Slug:          slug,
		FeatureState:  FeatureStateUnknown,
		Disclaimer:    disclaimer,
		Artifacts:     []Artifact{},
		Overall: Overall{
			StructuralReadiness: ReadinessIndeterminate,
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
		return NewAbortReport("", AbortSlugUnsafe)
	}
	// The scratch buffer is the caller's contract, not workspace state: a
	// wrong length is a programming error in the calling package and must
	// never be laundered into a filesystem-shaped abort the operator can
	// neither reproduce nor remediate (§7.4.5).
	if len(scratch) != MaxArtifactBytes+1 {
		panic(scratchLengthInvariantMessage)
	}

	base := featureBase(canonicalSlug)
	if code, aborted := inspectFeatureDirectory(ops, canonicalSlug); aborted {
		return NewAbortReport(canonicalSlug, code)
	}

	statusName := base + "/status.json"
	status := capture(ops, statusName, nil, scratch[:MaxStatusBytes+1], MaxStatusBytes)
	featureState := FeatureStateUnknown
	statusAbsent := false
	switch status.state {
	case StatePresentNonempty:
		state, ok := decodeStatusDocument(status.bytes)
		if !ok {
			return NewAbortReport(canonicalSlug, AbortStatusMalformed)
		}
		if !validFeatureState(state) {
			return NewAbortReport(canonicalSlug, AbortStatusInvalidState)
		}
		featureState = state
	case StateAbsent:
		statusAbsent = true
	case StatePresentEmpty:
		return NewAbortReport(canonicalSlug, AbortStatusMalformed)
	case StateSymlinkRefused:
		return NewAbortReport(canonicalSlug, AbortStatusSymlink)
	case StateNotRegular:
		return NewAbortReport(canonicalSlug, AbortStatusNotRegular)
	case StateOversize:
		return NewAbortReport(canonicalSlug, AbortStatusOversize)
	case StateUnstable:
		return NewAbortReport(canonicalSlug, AbortStatusUnstable)
	case StateUnreadable:
		return NewAbortReport(canonicalSlug, AbortStatusUnreadable)
	default:
		panic("intent.Inspect: status capture produced a state outside the closed enum: " + status.state)
	}

	report := Report{
		SchemaVersion: 1,
		Command:       CommandName,
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
			Code:       AdvisoryFeatureStateAbsent,
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
				result.structuredReason = ReasonSidecarNotJSON
			} else if !jsonObject(result.bytes) {
				state = StateInvalidStructured
				result.structuredReason = ReasonSidecarNotJSONObject
			}
		}
		reason := reasonCode(state, spec.sidecar, result.structuredReason)
		artifact := Artifact{
			ID:          spec.id,
			Path:        name,
			Role:        spec.role,
			State:       state,
			ReasonCode:  reason,
			Provenance:  ProvenanceUnknown,
			Remediation: remediation(spec, name, canonicalSlug, state),
		}
		report.Artifacts = append(report.Artifacts, artifact)
		if spec.role == RoleRequired && state == StatePresentNonempty {
			report.Overall.RequiredSatisfied++
		}
		if spec.role == RoleOptional && state == StatePresentNonempty {
			report.Overall.OptionalSatisfied++
		}
		if spec.sidecar {
			if advisory := sidecarAdvisory(state); advisory != nil {
				report.Advisories = append(report.Advisories, *advisory)
			}
		}
	}

	report.Advisories = append(report.Advisories, Advisory{
		Code:       AdvisoryProvenanceUnknown,
		ArtifactID: "",
		Message:    "Per-artifact provenance is reported as unknown for every artifact. tpatch does not yet persist durable per-artifact source metadata.",
	})
	report.Overall.StructuralReadiness = readinessFor(report.Artifacts)
	return report
}

func featureBase(slug string) string {
	return ".tpatch/features/" + slug
}

func inspectFeatureDirectory(ops RootOps, slug string) (AbortCode, bool) {
	for _, name := range []string{".tpatch", ".tpatch/features", featureBase(slug)} {
		mustValidRootName(name)
		info, err := ops.Lstat(name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return AbortFeatureNotFound, true
			}
			return AbortFeatureUnsafe, true
		}
		if refused(info) || !info.IsDir() {
			return AbortFeatureUnsafe, true
		}
	}
	return "", false
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
		panic(fmt.Sprintf("%s%q", rootNameInvariantMessagePrefix, name))
	}
}

func jsonObject(data []byte) bool {
	if !json.Valid(data) {
		return false
	}
	trimmed := bytes.TrimSpace(data)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

func readinessFor(artifacts []Artifact) Readiness {
	for _, artifact := range artifacts {
		if artifact.Role == RoleRequired && artifact.State == StateUnstable {
			return ReadinessIndeterminate
		}
	}
	for _, artifact := range artifacts {
		if artifact.Role == RoleRequired && artifact.State != StatePresentNonempty {
			return ReadinessNotReady
		}
	}
	return ReadinessReady
}

func reasonCode(state string, sidecar bool, structuredReason string) string {
	switch state {
	case StatePresentNonempty:
		return ""
	case StatePresentEmpty:
		return ReasonArtifactEmpty
	case StateAbsent:
		return ReasonArtifactAbsent
	case StateSymlinkRefused:
		return ReasonArtifactSymlinkRefused
	case StateNotRegular:
		return ReasonArtifactNotRegular
	case StateUnreadable:
		return ReasonArtifactUnreadable
	case StateOversize:
		return ReasonArtifactOversize
	case StateInvalidStructured:
		return structuredReason
	case StateUnstable:
		return ReasonArtifactUnstable
	default:
		panic("intent.reasonCode: artifact state outside the closed enum: " + state)
	}
}

func remediation(spec artifactSpec, path, slug, state string) string {
	if spec.role != RoleRequired || state == StatePresentNonempty {
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
	case StateInvalidStructured:
		// Unreachable: only the optional sidecar can be invalid-structured,
		// and the optional role returned above.
		panic("intent.remediation: invalid-structured is not reachable for a required artifact")
	default:
		panic("intent.remediation: artifact state outside the closed enum: " + state)
	}
}

func sidecarAdvisory(state string) *Advisory {
	var code, message string
	switch state {
	case StatePresentNonempty:
		return nil
	case StateAbsent:
		code, message = AdvisorySidecarAbsent, "artifacts/analysis.json is written by the CLI-driven analyze phase and is not produced by analyze --manual. Its absence is not a defect."
	case StatePresentEmpty:
		code, message = AdvisorySidecarEmpty, "artifacts/analysis.json exists but contains no non-whitespace bytes. This is not a readiness input; the file can be regenerated by re-running the analyze phase or removed."
	case StateInvalidStructured:
		code, message = AdvisorySidecarInvalid, "artifacts/analysis.json exists but is not a JSON object. This is not a readiness input; the file can be regenerated by re-running the analyze phase or removed."
	case StateUnstable:
		code, message = AdvisorySidecarUnstable, "artifacts/analysis.json changed while it was being inspected, so its state was not determined. This is not a readiness input; re-run when no other tpatch process is writing this feature."
	case StateSymlinkRefused:
		code, message = AdvisorySidecarSymlinkRefused, "artifacts/analysis.json is a symbolic link and was not followed or read. This is not a readiness input; replace it with a regular file or remove it."
	case StateNotRegular:
		code, message = AdvisorySidecarNotRegular, "artifacts/analysis.json exists but is not a regular file, so it was not read. This is not a readiness input."
	case StateUnreadable:
		code, message = AdvisorySidecarUnreadable, "artifacts/analysis.json could not be inspected. This is not a readiness input; check the file's permissions or remove it."
	case StateOversize:
		code, message = AdvisorySidecarOversize, "artifacts/analysis.json exceeds the 4 MiB inspection limit and was not read. This is not a readiness input; inspect it manually."
	default:
		panic("intent.sidecarAdvisory: sidecar state outside the closed enum: " + state)
	}
	return &Advisory{Code: code, ArtifactID: "analysis_sidecar", Message: message}
}

func abortMessage(code AbortCode, slug string) string {
	switch code {
	case AbortSlugUnsafe:
		return "the requested feature name is not a canonical tpatch slug. Canonical slugs are lowercase letters, digits and single dashes, 1-60 bytes. Create features with tpatch add, or rename a hand-made feature directory under .tpatch/features/ to a canonical name."
	case AbortUnsupportedPlatform:
		return "this build of tpatch cannot guarantee that artifact inspection stays inside the repository on this platform, so prepare --check refuses to run here. Inspect the files under .tpatch/features/ directly."
	case AbortWorkspaceMissing:
		return "no tpatch workspace was found here or in any parent directory. Run tpatch init in the repository root, or pass --path with the repository directory."
	case AbortRootUnopenable:
		return "the repository root could not be opened for inspection. Check that the directory still exists and is readable, then re-run."
	case AbortFeatureUnsafe:
		return fmt.Sprintf(".tpatch/features/%s could not be inspected safely: a directory on the way to it is a symbolic link, a reparse point, or not a directory. Replace it with a real directory, or inspect the feature by hand.", slug)
	case AbortFeatureNotFound:
		return fmt.Sprintf("no feature directory exists at .tpatch/features/%s. Run tpatch status to list the features in this workspace.", slug)
	case AbortStatusSymlink:
		return fmt.Sprintf(".tpatch/features/%s/status.json is a symbolic link or reparse point and was not followed. Replace it with a regular file, then run tpatch doctor.", slug)
	case AbortStatusNotRegular:
		return fmt.Sprintf(".tpatch/features/%s/status.json is not a regular file and was not read. Replace it with a regular file, then run tpatch doctor.", slug)
	case AbortStatusOversize:
		return fmt.Sprintf(".tpatch/features/%s/status.json exceeds the 1 MiB inspection limit and was not read. Inspect it by hand, then run tpatch doctor.", slug)
	case AbortStatusUnreadable:
		return fmt.Sprintf(".tpatch/features/%s/status.json could not be read and closed cleanly, so the lifecycle state was not determined. Check the file's permissions and the filesystem it lives on, then run tpatch doctor.", slug)
	case AbortStatusUnstable:
		return fmt.Sprintf(".tpatch/features/%s/status.json changed while it was being read, so the lifecycle state could not be determined. Re-run when no other tpatch process is writing this feature.", slug)
	case AbortStatusMalformed:
		return fmt.Sprintf(".tpatch/features/%s/status.json was read but is not a valid tpatch status document. Run tpatch doctor to inspect and repair the workspace metadata.", slug)
	case AbortStatusInvalidState:
		return fmt.Sprintf(".tpatch/features/%s/status.json was read but records a lifecycle state this version of tpatch does not recognise. Upgrade tpatch, or run tpatch doctor to inspect the workspace metadata.", slug)
	default:
		panic("intent.abortMessage: abort code outside the closed catalog: " + string(code))
	}
}

// AbortCode returns the report's closed abort code, or "" when the report
// carries no abort. "" is never a catalog member.
func (r Report) AbortCode() AbortCode {
	if r.Abort == nil {
		return ""
	}
	return r.Abort.Code
}

// Readiness returns the report's closed structural-readiness value.
func (r Report) Readiness() Readiness {
	return r.Overall.StructuralReadiness
}
