package intent

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// WriteJSON writes the deterministic version-one JSON report.
func (r Report) WriteJSON(w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(r)
}

// WriteHuman writes the complete human report.
func (r Report) WriteHuman(w io.Writer) {
	if r.Slug == "" {
		fmt.Fprintln(w, "prepare --check  (slug withheld: not a canonical tpatch slug)")
	} else {
		fmt.Fprintf(w, "prepare --check  %s\n", r.Slug)
	}
	fmt.Fprintf(w, "lifecycle state: %s  (%s)\n\n", r.FeatureState, lifecycleAnnotation(r))

	if r.Abort != nil {
		fmt.Fprintln(w, "no artifacts were inspected")
		fmt.Fprintln(w)
		fmt.Fprintf(w, "abort: %s\n", r.Abort.Code)
		writeWrapped(w, "  ", "  ", r.Abort.Message)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "readiness: indeterminate")
		fmt.Fprintln(w, disclaimer)
		return
	}

	fmt.Fprintln(w, "required")
	for _, artifact := range r.Artifacts {
		if artifact.Role != RoleRequired {
			continue
		}
		writeArtifact(w, artifact)
	}
	fmt.Fprintln(w, "optional")
	for _, artifact := range r.Artifacts {
		if artifact.Role == RoleOptional {
			writeArtifact(w, artifact)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "provenance: unknown (all artifacts)")
	fmt.Fprintln(w)
	if len(r.Advisories) > 0 {
		fmt.Fprintln(w, "advisories")
		for _, advisory := range r.Advisories {
			fmt.Fprintf(w, "  %s\n", advisory.Code)
		}
		fmt.Fprintln(w)
	}
	if r.Readiness() == ReadinessNotReady {
		fmt.Fprintf(w, "readiness: not_ready (%d of 3 required artifacts are present-nonempty)\n", r.Overall.RequiredSatisfied)
	} else {
		fmt.Fprintf(w, "readiness: %s\n", string(r.Readiness()))
	}
	fmt.Fprintln(w, disclaimer)
}

// WriteQuiet writes the one-line non-JSON report form.
func (r Report) WriteQuiet(w io.Writer) {
	if r.Abort != nil {
		if r.Slug == "" {
			fmt.Fprintf(w, "prepare --check — indeterminate (%s)\n", r.Abort.Code)
			return
		}
		fmt.Fprintf(w, "prepare --check %s — indeterminate (%s)\n", r.Slug, r.Abort.Code)
		return
	}
	fmt.Fprintf(w, "prepare --check %s — %s\n", r.Slug, r.Readiness())
}

// ExitMessage is the process-level diagnostic for report-bearing failures.
func (r Report) ExitMessage() string {
	if r.Abort != nil {
		if r.Abort.Code == AbortSlugUnsafe {
			return "prepare --check: indeterminate (slug-unsafe)"
		}
		return fmt.Sprintf("prepare --check %s: indeterminate (%s)", r.Slug, r.Abort.Code)
	}
	switch r.Readiness() {
	case ReadinessNotReady:
		return fmt.Sprintf("prepare --check %s: not_ready (%d of 3 required artifacts are present-nonempty)", r.Slug, r.Overall.RequiredSatisfied)
	case ReadinessIndeterminate:
		return fmt.Sprintf("prepare --check %s: indeterminate (a required artifact changed while it was being inspected; re-run when no other tpatch process is writing this feature)", r.Slug)
	default:
		return ""
	}
}

func writeArtifact(w io.Writer, artifact Artifact) {
	label := artifact.Path
	if index := strings.LastIndex(artifact.Path, "/"); index >= 0 {
		label = artifact.Path[index+1:]
		if strings.HasPrefix(artifact.ID, "analysis_sidecar") {
			label = "artifacts/analysis.json"
		}
	}
	fmt.Fprintf(w, "  %-34s %s\n", label, artifact.State)
	if artifact.Remediation != "" {
		writeWrapped(w, "    → ", "      ", artifact.Remediation)
	}
}

func lifecycleAnnotation(r Report) string {
	if r.Abort == nil {
		if r.FeatureState == FeatureStateUnknown {
			return "this feature directory has no status.json"
		}
		return "echoed from status.json; not evaluated by this check"
	}
	switch r.Abort.Code {
	case AbortSlugUnsafe:
		return "no feature was identified, so status.json was not read"
	case AbortUnsupportedPlatform:
		return "inspection was refused on this platform, so status.json was not read"
	case AbortWorkspaceMissing:
		return "no workspace was found, so status.json was not read"
	case AbortRootUnopenable:
		return "the repository root could not be opened, so status.json was not read"
	case AbortFeatureUnsafe:
		return "the feature directory could not be inspected safely, so status.json was not read"
	case AbortFeatureNotFound:
		return "no feature directory exists, so status.json was not read"
	case AbortStatusSymlink:
		return "status.json is a symbolic link or reparse point and was not followed"
	case AbortStatusNotRegular:
		return "status.json is not a regular file and was not read"
	case AbortStatusOversize:
		return "status.json exceeds the inspection limit and was not read"
	case AbortStatusUnreadable:
		return "status.json could not be read and closed cleanly"
	case AbortStatusUnstable:
		return "status.json changed while it was being read"
	case AbortStatusMalformed:
		return "status.json was read but is not a valid status document"
	case AbortStatusInvalidState:
		return "status.json was read but records a state this tpatch does not recognise"
	default:
		panic("intent.lifecycleAnnotation: abort code outside the closed catalog: " + string(r.Abort.Code))
	}
}

func writeWrapped(w io.Writer, firstPrefix, nextPrefix, message string) {
	const width = 78
	words := strings.Fields(message)
	prefix := firstPrefix
	line := prefix
	for _, word := range words {
		if len(line) > len(prefix) && len(line)+1+len(word) > width {
			fmt.Fprintln(w, line)
			prefix = nextPrefix
			line = prefix + word
			continue
		}
		if len(line) > len(prefix) {
			line += " "
		}
		line += word
	}
	if line != prefix {
		fmt.Fprintln(w, line)
	}
}
