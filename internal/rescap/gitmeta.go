// Logical Git-metadata views for
// PRD-feature-resource-claims-and-capture-adapters §5.2.
//
// Four closed views, each producing a distinct fixed-field result
// variant. No view exposes raw object content: these are allowlisted
// logical facts (which ref HEAD names, one index entry's stage/mode/
// oid, one of exactly four config keys) rather than a general Git-read
// surface.
//
// Every resolved string value passes through the redaction scanner
// before it is written anywhere. In practice none of the four closed
// views can produce a value shaped like any of the six classes, but the
// scan runs unconditionally rather than being skipped "because it's Git
// metadata".

package rescap

import (
	"sort"
	"strconv"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/redact"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// AllowedConfigKeys is the closed four-key set the config view accepts.
// No user.*, no wildcarded core.*/remote.*/branch.*.
var AllowedConfigKeys = []string{
	"core.filemode",
	"core.ignorecase",
	"core.symlinks",
	"extensions.objectformat",
}

// IsAllowedConfigKey reports whether key is one of the four.
func IsAllowedConfigKey(key string) bool {
	for _, k := range AllowedConfigKeys {
		if k == key {
			return true
		}
	}
	return false
}

// GitMetadataViews is the closed view set.
var GitMetadataViews = []string{
	store.GitMetadataViewHead,
	store.GitMetadataViewRef,
	store.GitMetadataViewIndexEntry,
	store.GitMetadataViewConfig,
}

// classifyGitMetadataView infers which view a selector names. `head`
// is the literal view name; every other view is distinguished by the
// declared kind's own selector shape, so the resource records the view
// explicitly in its capability field.
func classifyGitMetadataView(selector, capability string) string {
	if capability != "" {
		return capability
	}
	if selector == store.GitMetadataViewHead {
		return store.GitMetadataViewHead
	}
	return ""
}

// CaptureGitMetadata stages one git-metadata resource.
func CaptureGitMetadata(repoRoot, view, selector string) (store.CanonNode, error) {
	switch view {
	case store.GitMetadataViewHead:
		return captureHead(repoRoot)
	case store.GitMetadataViewRef:
		return captureRef(repoRoot, selector)
	case store.GitMetadataViewIndexEntry:
		return captureIndexEntry(repoRoot, selector)
	case store.GitMetadataViewConfig:
		return captureConfig(repoRoot, selector)
	default:
		return store.CanonNull(), Invalid(ReasonInvalidDeclaration,
			"unknown git-metadata view %q; expected one of %s", view, strings.Join(GitMetadataViews, ", "))
	}
}

// captureHead resolves the symbolic ref HEAD points at (null iff
// detached, never independently) and the current commit OID.
func captureHead(repoRoot string) (store.CanonNode, error) {
	oid, err := RunGit(repoRoot, "rev-parse", "HEAD")
	if err != nil {
		return store.CanonNull(), Refuse(ReasonPathMissing, "resolving HEAD: %v", err)
	}
	oidValue := strings.TrimSpace(oid)
	symRef, symErr := RunGit(repoRoot, "symbolic-ref", "--quiet", "HEAD")
	detached := symErr != nil || strings.TrimSpace(symRef) == ""
	symValue := store.CanonNull()
	if !detached {
		name := strings.TrimSpace(symRef)
		if err := scanGitValue(name); err != nil {
			return store.CanonNull(), err
		}
		symValue = store.CanonString(name)
	}
	if err := scanGitValue(oidValue); err != nil {
		return store.CanonNull(), err
	}
	return store.CanonObject(
		store.CanonFieldOf("symbolic_ref", symValue),
		store.CanonFieldOf("oid", store.CanonString(oidValue)),
		store.CanonFieldOf("detached", store.CanonBool(detached)),
	), nil
}

// captureRef resolves an explicitly selected ref to its full name and
// OID.
func captureRef(repoRoot, selector string) (store.CanonNode, error) {
	full, err := RunGit(repoRoot, "rev-parse", "--symbolic-full-name", selector)
	if err != nil || strings.TrimSpace(full) == "" {
		return store.CanonNull(), Refuse(ReasonPathMissing, "ref %q does not resolve", selector)
	}
	oid, err := RunGit(repoRoot, "rev-parse", selector)
	if err != nil {
		return store.CanonNull(), Refuse(ReasonPathMissing, "ref %q does not resolve to an object: %v", selector, err)
	}
	name := strings.TrimSpace(full)
	oidValue := strings.TrimSpace(oid)
	if err := scanGitValue(name); err != nil {
		return store.CanonNull(), err
	}
	if err := scanGitValue(oidValue); err != nil {
		return store.CanonNull(), err
	}
	return store.CanonObject(
		store.CanonFieldOf("ref", store.CanonString(name)),
		store.CanonFieldOf("oid", store.CanonString(oidValue)),
	), nil
}

// IndexEntry is one parsed `ls-files --stage` row.
type IndexEntry struct {
	Path  string
	Mode  string
	OID   string
	Stage uint64
}

// LookupIndexEntry queries one path's index entry via the literal
// pathspec form. A path with no index entry at all reports ok=false.
func LookupIndexEntry(repoRoot, selector string) (IndexEntry, bool, error) {
	out, err := RunGit(repoRoot, "--literal-pathspecs", "ls-files", "--stage", "--", selector)
	if err != nil {
		return IndexEntry{}, false, Refuse(ReasonGitLsFilesError,
			"git ls-files --stage -- %s failed: %v", selector, err)
	}
	line := strings.TrimSpace(out)
	if line == "" {
		return IndexEntry{}, false, nil
	}
	// Format: <mode> SP <oid> SP <stage> TAB <path>
	tab := strings.Index(line, "\t")
	if tab < 0 {
		return IndexEntry{}, false, Refuse(ReasonGitLsFilesError,
			"unexpected ls-files --stage output for %s", selector)
	}
	fields := strings.Fields(line[:tab])
	if len(fields) != 3 {
		return IndexEntry{}, false, Refuse(ReasonGitLsFilesError,
			"unexpected ls-files --stage output for %s", selector)
	}
	stage, convErr := strconv.ParseUint(fields[2], 10, 64)
	if convErr != nil || stage > 3 {
		return IndexEntry{}, false, Refuse(ReasonGitLsFilesError,
			"unexpected stage %q for %s", fields[2], selector)
	}
	return IndexEntry{
		Path:  strings.TrimSpace(line[tab+1:]),
		Mode:  fields[0],
		OID:   fields[1],
		Stage: stage,
	}, true, nil
}

func captureIndexEntry(repoRoot, selector string) (store.CanonNode, error) {
	entry, ok, err := LookupIndexEntry(repoRoot, selector)
	if err != nil {
		return store.CanonNull(), err
	}
	if !ok {
		return store.CanonNull(), Refuse(ReasonIndexEntryMissing,
			"%s has no index entry", selector)
	}
	for _, v := range []string{entry.Path, entry.Mode, entry.OID} {
		if err := scanGitValue(v); err != nil {
			return store.CanonNull(), err
		}
	}
	return store.CanonObject(
		store.CanonFieldOf("path", store.CanonString(entry.Path)),
		store.CanonFieldOf("mode", store.CanonString(entry.Mode)),
		store.CanonFieldOf("oid", store.CanonString(entry.OID)),
		store.CanonFieldOf("stage", store.CanonUint(entry.Stage)),
	), nil
}

// captureConfig reads one of the four allowlisted keys. Unset is a
// valid, reportable state (JSON null), not an error: all four keys have
// sensible defaults when absent.
func captureConfig(repoRoot, key string) (store.CanonNode, error) {
	if !IsAllowedConfigKey(key) {
		return store.CanonNull(), Invalid(ReasonInvalidDeclaration,
			"config key %q is not one of the four allowed keys: %s", key, strings.Join(AllowedConfigKeys, ", "))
	}
	out, err := RunGit(repoRoot, "config", "--get", key)
	value := store.CanonNull()
	if err == nil {
		text := strings.TrimSpace(out)
		if scanErr := scanGitValue(text); scanErr != nil {
			return store.CanonNull(), scanErr
		}
		value = store.CanonString(text)
	}
	return store.CanonObject(
		store.CanonFieldOf("key", store.CanonString(key)),
		store.CanonFieldOf("value", value),
	), nil
}

// scanGitValue applies the unconditional redaction scan to a resolved
// Git value.
func scanGitValue(v string) error {
	findings := redact.ScanString(v)
	if len(findings) == 0 {
		return nil
	}
	sort.Strings(findings)
	return Refuse(ReasonRedactionRefused,
		"a resolved git-metadata value matched forbidden content classes %s", strings.Join(findings, ", "))
}
