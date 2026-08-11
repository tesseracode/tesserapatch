// Resource declaration manifest + tracked capture store for
// PRD-feature-resource-claims-and-capture-adapters (ADR-033).
//
// Two tracked artifacts per feature, both under the existing
// per-feature artifacts directory and never inside apply-recipe.json
// or any unapply/lifecycle-state file (ADR-033 D1):
//
//	artifacts/resources.json                        declaration manifest
//	artifacts/resource-captures/batches/<id>.json   immutable content-addressed batches
//	artifacts/resource-captures/current.json        the atomically-rewritten pointer
//
// resources.json is written only by add/remove/clear/trust-dolt; the
// resource-captures tree is written only by capture/record --resources.

package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResourcesManifestVersion is the on-disk schema version of
// resources.json.
const ResourcesManifestVersion = 1

// Closed resource-kind set (ADR-033 D2). There is no plugin mechanism
// and generic-command is deliberately absent.
const (
	ResourceKindIgnoredFile     = "ignored-file"
	ResourceKindGitMetadata     = "git-metadata"
	ResourceKindAdapterSnapshot = "adapter-snapshot"
)

// The single v1 adapter and its single capability.
const (
	ResourceAdapterDolt      = "dolt"
	ResourceCapabilityDiff   = "diff-summary"
	DoltContractDiffSummary1 = "dolt-diff-summary-v1"
)

// Closed git-metadata views (§5.2).
const (
	GitMetadataViewHead       = "head"
	GitMetadataViewRef        = "ref"
	GitMetadataViewIndexEntry = "index-entry"
	GitMetadataViewConfig     = "config"
)

// ResourceIDPrefix and BatchIDPrefix are the reader-facing ID prefixes.
// resource_id truncates to 12 hex characters (a short, reader-facing
// ID whose collisions are always caught at add/load time); batch_id
// carries the full digest because a batch collision would otherwise be
// silently overwrite-prone (§4, §7.3 step 2).
const (
	ResourceIDPrefix    = "res_"
	ResourceIDHexLen    = 12
	BatchIDPrefix       = "rb_"
	ResourceIDPrefixMin = 7
)

// ResourceArg is one declared argument. Args are a sorted array of
// key/value pairs on the wire, never a bare JSON object, so tracked
// output never depends on encoding/json's map-key ordering
// (ADR-033 D11).
type ResourceArg struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ResourceTrust is the mutable trust pin for an adapter-snapshot/dolt
// resource. It is deliberately excluded from resource_id's identity
// hash so a legitimate Dolt upgrade re-pinned via `trust-dolt` keeps
// the resource's ID, current.json entry and batch history intact
// (§4, §6.1).
type ResourceTrust struct {
	BinarySHA256 string `json:"binary_sha256"`
}

// Resource is one declared entry of resources.json.
type Resource struct {
	ResourceID         string         `json:"resource_id"`
	Kind               string         `json:"kind"`
	Selector           string         `json:"selector"`
	Adapter            string         `json:"adapter"`
	Capability         string         `json:"capability"`
	Args               []ResourceArg  `json:"args"`
	Trust              *ResourceTrust `json:"trust"`
	AddedByToolVersion string         `json:"added_by_tool_version"`
}

// ResourcesManifest is the full on-disk shape of resources.json.
type ResourcesManifest struct {
	Version   int        `json:"version"`
	Feature   string     `json:"feature"`
	Resources []Resource `json:"resources"`
}

// Arg returns a declared argument's value and whether it is present.
func (r Resource) Arg(key string) (string, bool) {
	for _, a := range r.Args {
		if a.Key == key {
			return a.Value, true
		}
	}
	return "", false
}

// IsDoltAdapter reports whether this resource is the adapter-snapshot/
// dolt shape `trust-dolt` and the capture-time adapter path require.
func (r Resource) IsDoltAdapter() bool {
	return r.Kind == ResourceKindAdapterSnapshot && r.Adapter == ResourceAdapterDolt
}

// ResourceIdentityPayload builds §13.2's exact hash-input string. The
// trust pin never participates.
func ResourceIdentityPayload(feature, kind, selector, adapter, capability string, args []ResourceArg) string {
	return strings.Join([]string{feature, kind, selector, adapter, capability, CanonicalArgsJSON(args)}, "\x00")
}

// ComputeResourceID derives `res_` + the first 12 lowercase-hex
// characters of SHA-256 over §13.2's payload.
func ComputeResourceID(feature, kind, selector, adapter, capability string, args []ResourceArg) string {
	digest := sha256.Sum256([]byte(ResourceIdentityPayload(feature, kind, selector, adapter, capability, args)))
	return ResourceIDPrefix + hex.EncodeToString(digest[:])[:ResourceIDHexLen]
}

// resourceIDDeriver is the substitutable derivation seam §4 requires
// for exercising the resource-id-collision branch: a real SHA-256
// collision is not producible for a fixture, so tests swap this
// function rather than adding a test-only branch to production code.
var resourceIDDeriver = ComputeResourceID

// SetResourceIDDeriverForTest replaces the resource-ID derivation
// function and returns a restore func. Tests only; the production code
// path has no branch on it.
func SetResourceIDDeriverForTest(fn func(feature, kind, selector, adapter, capability string, args []ResourceArg) string) func() {
	prev := resourceIDDeriver
	resourceIDDeriver = fn
	return func() { resourceIDDeriver = prev }
}

// DeriveResourceID applies the currently-installed derivation
// function.
func DeriveResourceID(feature, kind, selector, adapter, capability string, args []ResourceArg) string {
	return resourceIDDeriver(feature, kind, selector, adapter, capability, args)
}

// Identity returns this resource's freshly-recomputed identity payload
// and ID for the given feature.
func (r Resource) Identity(feature string) (payload string, id string) {
	return ResourceIdentityPayload(feature, r.Kind, r.Selector, r.Adapter, r.Capability, r.Args),
		DeriveResourceID(feature, r.Kind, r.Selector, r.Adapter, r.Capability, r.Args)
}

// ─── manifest paths ──────────────────────────────────────────────────────────

// ResourcesPath returns the absolute path to a feature's
// resources.json.
func (s *Store) ResourcesPath(slug string) string {
	return filepath.Join(s.featureArtifactsDir(slug), "resources.json")
}

// ResourceCapturesDir returns the tracked capture store root.
func (s *Store) ResourceCapturesDir(slug string) string {
	return filepath.Join(s.featureArtifactsDir(slug), "resource-captures")
}

// ResourceBatchesDir returns the immutable batch directory.
func (s *Store) ResourceBatchesDir(slug string) string {
	return filepath.Join(s.ResourceCapturesDir(slug), "batches")
}

// ResourceBatchPath returns the path of one immutable batch file.
func (s *Store) ResourceBatchPath(slug, batchID string) string {
	return filepath.Join(s.ResourceBatchesDir(slug), batchID+".json")
}

// ResourceCurrentPath returns the tracked pointer path.
func (s *Store) ResourceCurrentPath(slug string) string {
	return filepath.Join(s.ResourceCapturesDir(slug), "current.json")
}

// ResourceCurrentTempPath returns the one exact temp name used for the
// pointer write (§7.1's tracked-tree diagram). The per-slug flock
// already serializes pointer writers, so no random suffix is needed.
func (s *Store) ResourceCurrentTempPath(slug string) string {
	return filepath.Join(s.ResourceCapturesDir(slug), ".tmp-current.json")
}

// ─── manifest load/save ──────────────────────────────────────────────────────

// ResourceManifestError is a named, load-time refusal from
// LoadResources. Reason is one of the ADR-033 refusal names
// (`resources-file-corrupt`, `resource-id-collision`).
type ResourceManifestError struct {
	Reason     string
	ResourceID string
	Detail     string
}

// Error satisfies the error interface.
func (e *ResourceManifestError) Error() string {
	if e.ResourceID == "" {
		return fmt.Sprintf("%s: %s", e.Reason, e.Detail)
	}
	return fmt.Sprintf("%s: %s (%s)", e.Reason, e.Detail, e.ResourceID)
}

// Named load-time refusals.
const (
	ReasonResourcesFileCorrupt = "resources-file-corrupt"
	ReasonResourceIDCollision  = "resource-id-collision"
)

// LoadResources reads a feature's declaration manifest. A missing file
// is an empty manifest, not an error (mirroring LoadClaims).
//
// Every loaded entry is re-validated against §4's corruption/collision
// taxonomy before it is returned:
//
//   - an entry whose recorded resource_id does not match the ID freshly
//     recomputed from its own fields is `resources-file-corrupt` — one
//     entry inconsistent with itself, only reachable by hand-editing;
//   - two individually self-consistent entries sharing one recorded
//     resource_id is `resource-id-collision`.
func LoadResources(s *Store, slug string) (ResourcesManifest, error) {
	path := s.ResourcesPath(slug)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ResourcesManifest{
				Version:   ResourcesManifestVersion,
				Feature:   slug,
				Resources: []Resource{},
			}, nil
		}
		return ResourcesManifest{}, err
	}
	var m ResourcesManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return ResourcesManifest{}, &ResourceManifestError{
			Reason: ReasonResourcesFileCorrupt,
			Detail: fmt.Sprintf("%s is not valid JSON: %v", path, err),
		}
	}
	if m.Version != ResourcesManifestVersion {
		return ResourcesManifest{}, &ResourceManifestError{
			Reason: ReasonResourcesFileCorrupt,
			Detail: fmt.Sprintf("%s declares version %d, want %d", path, m.Version, ResourcesManifestVersion),
		}
	}
	if m.Feature != "" && m.Feature != slug {
		return ResourcesManifest{}, &ResourceManifestError{
			Reason: ReasonResourcesFileCorrupt,
			Detail: fmt.Sprintf("%s declares feature %q, want %q", path, m.Feature, slug),
		}
	}
	m.Feature = slug
	if m.Resources == nil {
		m.Resources = []Resource{}
	}
	for i := range m.Resources {
		if m.Resources[i].Args == nil {
			m.Resources[i].Args = []ResourceArg{}
		}
	}
	if err := ValidateLoadedResources(slug, m.Resources); err != nil {
		return ResourcesManifest{}, err
	}
	return m, nil
}

// ValidateLoadedResources applies §4's load-time corruption and
// collision checks to an already-decoded entry list.
func ValidateLoadedResources(feature string, resources []Resource) error {
	byID := map[string]struct{}{}
	for _, r := range resources {
		_, want := r.Identity(feature)
		if r.ResourceID != want {
			return &ResourceManifestError{
				Reason:     ReasonResourcesFileCorrupt,
				ResourceID: r.ResourceID,
				Detail:     fmt.Sprintf("recorded resource_id does not match its own fields (recomputed %s)", want),
			}
		}
		if _, dup := byID[r.ResourceID]; dup {
			return &ResourceManifestError{
				Reason:     ReasonResourceIDCollision,
				ResourceID: r.ResourceID,
				Detail:     "two declarations share one resource_id",
			}
		}
		byID[r.ResourceID] = struct{}{}
	}
	return nil
}

// SaveResources writes the manifest atomically (temp file in the same
// directory, fsync, rename, directory fsync) with entries sorted by
// resource_id so the tracked bytes are deterministic.
func SaveResources(s *Store, m ResourcesManifest) error {
	if m.Version == 0 {
		m.Version = ResourcesManifestVersion
	}
	if m.Resources == nil {
		m.Resources = []Resource{}
	}
	sorted := make([]Resource, len(m.Resources))
	copy(sorted, m.Resources)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ResourceID < sorted[j].ResourceID })
	for i := range sorted {
		if sorted[i].Args == nil {
			sorted[i].Args = []ResourceArg{}
		}
		sort.SliceStable(sorted[i].Args, func(a, b int) bool { return sorted[i].Args[a].Key < sorted[i].Args[b].Key })
	}
	m.Resources = sorted
	if err := os.MkdirAll(filepath.Dir(s.ResourcesPath(m.Feature)), 0o755); err != nil {
		return err
	}
	return writeJSONAtomic(s.ResourcesPath(m.Feature), m)
}

// FindResource resolves an exact resource_id or an unambiguous
// prefix (at least ResourceIDPrefixMin characters after the `res_`
// tag, matching the claims-manifest convention). Ambiguity is an
// error; "no match" is (Resource{}, false, nil).
func FindResource(m *ResourcesManifest, arg string) (Resource, bool, error) {
	if arg == "" {
		return Resource{}, false, nil
	}
	for _, r := range m.Resources {
		if r.ResourceID == arg {
			return r, true, nil
		}
	}
	needle := strings.TrimPrefix(arg, ResourceIDPrefix)
	if len(needle) < ResourceIDPrefixMin {
		return Resource{}, false, nil
	}
	var matches []Resource
	for _, r := range m.Resources {
		if strings.HasPrefix(strings.TrimPrefix(r.ResourceID, ResourceIDPrefix), needle) {
			matches = append(matches, r)
		}
	}
	switch len(matches) {
	case 0:
		return Resource{}, false, nil
	case 1:
		return matches[0], true, nil
	default:
		ids := make([]string, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, m.ResourceID)
		}
		sort.Strings(ids)
		return Resource{}, false, fmt.Errorf("ambiguous resource prefix %q matches %s", arg, strings.Join(ids, ", "))
	}
}

// RemoveResource drops the entry with the given ID in place and
// reports whether anything was removed.
func RemoveResource(m *ResourcesManifest, resourceID string) bool {
	for i, r := range m.Resources {
		if r.ResourceID == resourceID {
			m.Resources = append(m.Resources[:i], m.Resources[i+1:]...)
			return true
		}
	}
	return false
}

// SetResourceTrust rewrites only the target entry's
// trust.binary_sha256 field, leaving every other field of that entry
// and every other entry byte-for-byte unchanged (§12.6).
func SetResourceTrust(m *ResourcesManifest, resourceID, binarySHA256 string) bool {
	for i := range m.Resources {
		if m.Resources[i].ResourceID == resourceID {
			m.Resources[i].Trust = &ResourceTrust{BinarySHA256: binarySHA256}
			return true
		}
	}
	return false
}
