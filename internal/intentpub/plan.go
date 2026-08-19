package intentpub

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

type Plan struct {
	slug       string
	mode       Mode
	stageRel   string
	entries    []Entry
	planDigest string
}

func (p Plan) Slug() string {
	return p.slug
}

func (p Plan) Mode() Mode {
	return p.mode
}

func (p Plan) StageRel() string {
	return p.stageRel
}

func (p Plan) Digest() string {
	return p.planDigest
}

func (p Plan) Entries() []Entry {
	return cloneEntries(p.entries)
}

type PlanBuilder struct {
	slug     string
	mode     Mode
	stageRel string
	entries  []Entry
	frozen   bool
}

func NewPlanBuilder(slug string, mode Mode, stageRel string) *PlanBuilder {
	return &PlanBuilder{slug: slug, mode: mode, stageRel: stageRel}
}

func (b *PlanBuilder) Add(entry Entry) error {
	if b == nil || b.frozen {
		return transactionError(CodeInvalidPlan, entry.ArtifactID, "plan-frozen", "the publication plan is already frozen", 5)
	}
	b.entries = append(b.entries, entry)
	return nil
}

func (b *PlanBuilder) Freeze() (Plan, error) {
	if b == nil || b.frozen {
		return Plan{}, transactionError(CodeInvalidPlan, "", "plan-frozen", "the publication plan cannot be frozen again", 5)
	}
	b.frozen = true
	entries := cloneEntries(b.entries)
	if err := validatePlanShape(b.slug, b.mode, b.stageRel, entries); err != nil {
		return Plan{}, err
	}
	digest, err := PlanDigest(entries)
	if err != nil {
		return Plan{}, err
	}
	return Plan{
		slug:       b.slug,
		mode:       b.mode,
		stageRel:   b.stageRel,
		entries:    entries,
		planDigest: digest,
	}, nil
}

func NewPlan(slug string, mode Mode, stageRel string, entries []Entry) (Plan, error) {
	builder := NewPlanBuilder(slug, mode, stageRel)
	for _, entry := range entries {
		if err := builder.Add(entry); err != nil {
			return Plan{}, err
		}
	}
	return builder.Freeze()
}

// canonicalPlanEntries is the version-1 plan-digest encoding. It is one
// compact JSON array of fixed-field structs in publication order. It includes
// every identity, recovery source, and staged reference. It uses no maps.
type canonicalPlanEntry struct {
	ArtifactID      ArtifactID `json:"artifact_id"`
	Rel             string     `json:"rel"`
	Action          Action     `json:"action"`
	Preimage        Identity   `json:"preimage"`
	PreimageBlob    string     `json:"preimage_blob"`
	PreimageBlobRel string     `json:"preimage_blob_rel"`
	PreimageRawRel  string     `json:"preimage_raw_rel"`
	NewImage        Identity   `json:"new_image"`
	StagedRel       string     `json:"staged_rel"`
}

func CanonicalPlanEncoding(entries []Entry) ([]byte, error) {
	if entries == nil {
		return nil, transactionError(CodeInvalidPlan, "", "null-entries", "publication entries cannot be null", 5)
	}
	canonical := make([]canonicalPlanEntry, 0, len(entries))
	for _, entry := range entries {
		canonical = append(canonical, canonicalPlanEntry{
			ArtifactID:      entry.ArtifactID,
			Rel:             entry.Rel,
			Action:          entry.Action,
			Preimage:        entry.Preimage,
			PreimageBlob:    entry.PreimageBlob,
			PreimageBlobRel: entry.PreimageBlobRel,
			PreimageRawRel:  entry.PreimageRawRel,
			NewImage:        entry.NewImage,
			StagedRel:       entry.StagedRel,
		})
	}
	return json.Marshal(canonical)
}

func PlanDigest(entries []Entry) (string, error) {
	encoded, err := CanonicalPlanEncoding(entries)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func canonicalTempSuffix(runNonce string, entry Entry) string {
	sum := sha256.Sum256([]byte("intentpub-canonical-temp-v1\x00" +
		runNonce + "\x00" + string(entry.ArtifactID) + "\x00" + entry.Rel))
	return hex.EncodeToString(sum[:6])
}

func canonicalTempRel(runNonce string, entry Entry) string {
	return path.Join(path.Dir(entry.Rel), "."+path.Base(entry.Rel)+".tmp-"+canonicalTempSuffix(runNonce, entry))
}

func controlTempSuffix(runNonce, purpose string) string {
	sum := sha256.Sum256([]byte("intentpub-control-temp-v1\x00" + runNonce + "\x00" + purpose))
	return hex.EncodeToString(sum[:6])
}

func validatePlanShape(slug string, mode Mode, stageRel string, entries []Entry) error {
	if !validSlug(slug) {
		return transactionError(CodeInvalidPlan, "", "slug", "the feature slug is not a valid path component", 5)
	}
	if mode != ModeGenerate && mode != ModeRegenerate {
		return transactionError(CodeInvalidPlan, "", "mode", "transactional publication supports generate or regenerate mode", 5)
	}
	if !validStageRel(slug, stageRel) {
		return transactionError(CodeInvalidPlan, "", "stage-path", "the staging root is not in the owned transaction lane", 5)
	}
	if len(entries) == 0 {
		return transactionError(CodeInvalidPlan, "", "empty-plan", "the publication plan has no entries", 5)
	}

	seen := make(map[ArtifactID]bool, len(entries))
	lastOrder := -1
	for _, entry := range entries {
		order := artifactIndex(entry.ArtifactID)
		if order < 0 {
			return transactionError(CodeInvalidPlan, entry.ArtifactID, "artifact-id", "the plan contains an unknown artifact", 5)
		}
		if seen[entry.ArtifactID] {
			return transactionError(CodeInvalidPlan, entry.ArtifactID, "duplicate-entry", "the plan contains a duplicate artifact", 5)
		}
		if order <= lastOrder {
			return transactionError(CodeInvalidPlan, entry.ArtifactID, "entry-order", "the publication entries are not in fixed order", 5)
		}
		seen[entry.ArtifactID] = true
		lastOrder = order

		if entry.Rel != canonicalRel(slug, entry.ArtifactID) {
			return transactionError(CodeInvalidPlan, entry.ArtifactID, "canonical-path", "the canonical path does not match the artifact", 5)
		}
		if entry.StagedRel != stageRel+"/"+stagedBase(entry.ArtifactID) {
			return transactionError(CodeInvalidPlan, entry.ArtifactID, "staged-path", "the staged path does not match the frozen artifact target", 5)
		}
		if err := validateEntryIdentity(entry); err != nil {
			return err
		}
		if err := validateRecoverySource(slug, entry); err != nil {
			return err
		}

	}

	if entries[len(entries)-1].ArtifactID != ArtifactStatus || entries[len(entries)-1].Action != ActionReplace {
		return transactionError(CodeInvalidPlan, ArtifactStatus, "status-order", "status must be a replacement and the last publication entry", 5)
	}
	if mode == ModeGenerate {
		if err := validateGenerateSet(entries); err != nil {
			return err
		}
	} else {
		if err := validateRegenerateSet(entries); err != nil {
			return err
		}
	}
	return nil
}

func validateGenerateSet(entries []Entry) error {
	ids := entryIDs(entries)
	valid := equalArtifactIDs(ids, []ArtifactID{ArtifactExploration, ArtifactStatus}) ||
		equalArtifactIDs(ids, []ArtifactID{ArtifactSpec, ArtifactExploration, ArtifactStatus}) ||
		equalArtifactIDs(ids, []ArtifactID{
			ArtifactAnalysis,
			ArtifactSpec,
			ArtifactExploration,
			ArtifactAnalysisSidecar,
			ArtifactStatus,
		})
	if !valid {
		return transactionError(CodeInvalidPlan, "", "generate-set", "generate mode requires one exact dependency-coherent missing suffix plus status", 5)
	}
	for _, entry := range entries[:len(entries)-1] {
		if entry.Action != ActionCreate {
			return transactionError(CodeInvalidPlan, entry.ArtifactID, "generate-action", "generate mode may only create missing intent artifacts", 5)
		}
	}
	return nil
}

func validateRegenerateSet(entries []Entry) error {
	if len(entries) != 5 && len(entries) != 6 {
		return transactionError(CodeInvalidPlan, "", "regenerate-set", "regenerate mode requires the complete intent bundle and status", 5)
	}
	required := []ArtifactID{
		ArtifactAnalysis,
		ArtifactSpec,
		ArtifactExploration,
		ArtifactAnalysisSidecar,
	}
	if !equalArtifactIDs(entryIDs(entries[:4]), required) {
		return transactionError(CodeInvalidPlan, "", "regenerate-set", "regenerate mode requires all four intent artifacts in fixed order", 5)
	}
	replacedIntent := false
	for _, entry := range entries[:4] {
		if entry.Action == ActionReplace {
			replacedIntent = true
		}
	}
	hasIndex := len(entries) == 6
	if hasIndex {
		if entries[4].ArtifactID != ArtifactArchiveIndex || entries[5].ArtifactID != ArtifactStatus {
			return transactionError(CodeInvalidPlan, ArtifactArchiveIndex, "regenerate-order", "the archive index must immediately precede status", 5)
		}
	} else if entries[4].ArtifactID != ArtifactStatus {
		return transactionError(CodeInvalidPlan, ArtifactStatus, "regenerate-order", "status must follow the four intent artifacts", 5)
	}
	if hasIndex != replacedIntent {
		return transactionError(CodeInvalidPlan, ArtifactArchiveIndex, "index-coherence", "the archive index is required exactly when an intent artifact is replaced", 5)
	}
	return nil
}

func entryIDs(entries []Entry) []ArtifactID {
	ids := make([]ArtifactID, len(entries))
	for index, entry := range entries {
		ids[index] = entry.ArtifactID
	}
	return ids
}

func equalArtifactIDs(first, second []ArtifactID) bool {
	if len(first) != len(second) {
		return false
	}
	for index := range first {
		if first[index] != second[index] {
			return false
		}
	}
	return true
}

func validateEntryIdentity(entry Entry) error {
	if entry.Action != ActionCreate && entry.Action != ActionReplace {
		return transactionError(CodeInvalidPlan, entry.ArtifactID, "action", "the entry action is not create or replace", 5)
	}
	if err := validateIdentity(entry.Preimage); err != nil {
		return transactionError(CodeInvalidPlan, entry.ArtifactID, "preimage", err.Error(), 5)
	}
	if err := validateIdentity(entry.NewImage); err != nil {
		return transactionError(CodeInvalidPlan, entry.ArtifactID, "new-image", err.Error(), 5)
	}
	if !entry.NewImage.Exists {
		return transactionError(CodeInvalidPlan, entry.ArtifactID, "new-image", "the intended image must exist", 5)
	}
	switch entry.Action {
	case ActionCreate:
		if entry.Preimage.Exists {
			return transactionError(CodeInvalidPlan, entry.ArtifactID, "create-preimage", "a create entry must expect absence", 5)
		}
		if fs.FileMode(entry.NewImage.Mode).Perm() != 0o644 {
			return transactionError(CodeInvalidPlan, entry.ArtifactID, "create-mode", "new tracked files must use mode 0644", 5)
		}
	case ActionReplace:
		if !entry.Preimage.Exists {
			return transactionError(CodeInvalidPlan, entry.ArtifactID, "replace-preimage", "a replace entry requires an existing preimage", 5)
		}
		if entry.NewImage.Mode != entry.Preimage.Mode {
			return transactionError(CodeInvalidPlan, entry.ArtifactID, "replace-mode", "a replacement must preserve permission bits", 5)
		}
	}
	return nil
}

func validateRecoverySource(slug string, entry Entry) error {
	hasBlob := entry.PreimageBlob != "" || entry.PreimageBlobRel != ""
	hasRaw := entry.PreimageRawRel != ""
	if entry.Action == ActionCreate {
		if hasBlob || hasRaw {
			return transactionError(CodeInvalidPlan, entry.ArtifactID, "create-source", "a create entry cannot carry a preimage source", 5)
		}
		return nil
	}
	switch entry.ArtifactID {
	case ArtifactArchiveIndex:
		if hasBlob || !hasRaw || entry.PreimageRawRel != laneRel(slug)+"/index.preimage.json" {
			return transactionError(CodeInvalidPlan, entry.ArtifactID, "raw-source", "the archive index requires its exact raw preimage reference", 5)
		}
	case ArtifactStatus:
		if hasBlob || !hasRaw || entry.PreimageRawRel != laneRel(slug)+"/status.preimage.json" {
			return transactionError(CodeInvalidPlan, entry.ArtifactID, "raw-source", "status requires its exact raw preimage reference", 5)
		}
	default:
		if hasRaw || !validHash(entry.PreimageBlob) || !validRootRel(entry.PreimageBlobRel) {
			return transactionError(CodeInvalidPlan, entry.ArtifactID, "archive-source", "a replaced artifact requires an archive blob hash and path", 5)
		}
		if entry.PreimageBlob != entry.Preimage.SHA256 {
			return transactionError(CodeInvalidPlan, entry.ArtifactID, "archive-hash", "the archive blob hash must equal the preimage hash", 5)
		}
		archivePrefix := featureRel(slug) + "/artifacts/intent-archive/blobs"
		if path.Dir(entry.PreimageBlobRel) != archivePrefix ||
			path.Base(entry.PreimageBlobRel) != entry.PreimageBlob+".blob" {
			return transactionError(CodeInvalidPlan, entry.ArtifactID, "archive-path", "the archive blob path does not match the preimage hash", 5)
		}
	}
	return nil
}

func validateIdentity(identity Identity) error {
	if !identity.Exists {
		if identity.SHA256 != "" || identity.Size != 0 || identity.Mode != 0 {
			return fmt.Errorf("an absent identity must have zero hash, size, and mode")
		}
		return nil
	}
	if !validHash(identity.SHA256) {
		return fmt.Errorf("a present identity requires a full lowercase SHA-256")
	}
	if identity.Size < 0 || identity.Size > MaxArtifactBytes {
		return fmt.Errorf("identity size is outside the accepted bound")
	}
	mode := fs.FileMode(identity.Mode)
	if identity.Mode == 0 || identity.Mode > 0o777 || uint32(mode.Perm()) != identity.Mode {
		return fmt.Errorf("identity mode is not a permission-bit value")
	}
	return nil
}

func cloneEntries(entries []Entry) []Entry {
	if entries == nil {
		return nil
	}
	out := make([]Entry, len(entries))
	copy(out, entries)
	return out
}

func artifactIndex(id ArtifactID) int {
	for index, candidate := range artifactOrder {
		if candidate == id {
			return index
		}
	}
	return -1
}

func validSlug(slug string) bool {
	if len(slug) == 0 || len(slug) > 60 || !fs.ValidPath(slug) || strings.Contains(slug, "/") {
		return false
	}
	segmentStart := true
	for _, character := range []byte(slug) {
		if character == '-' {
			if segmentStart {
				return false
			}
			segmentStart = true
			continue
		}
		if !((character >= 'a' && character <= 'z') || (character >= '0' && character <= '9')) {
			return false
		}
		segmentStart = false
	}
	if segmentStart {
		return false
	}
	switch strings.ToUpper(slug) {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return false
	default:
		return true
	}
}

func validRootRel(rel string) bool {
	return rel != "" && rel != "." && fs.ValidPath(rel)
}

func contained(parent, child string) bool {
	return validRootRel(parent) && validRootRel(child) &&
		(child == parent || strings.HasPrefix(child, parent+"/"))
}

func featureRel(slug string) string {
	return ".tpatch/features/" + slug
}

func laneRel(slug string) string {
	return ".tpatch/local/intent-prepare/" + slug
}

func canonicalRel(slug string, id ArtifactID) string {
	base := featureRel(slug)
	switch id {
	case ArtifactAnalysis:
		return base + "/analysis.md"
	case ArtifactSpec:
		return base + "/spec.md"
	case ArtifactExploration:
		return base + "/exploration.md"
	case ArtifactAnalysisSidecar:
		return base + "/artifacts/analysis.json"
	case ArtifactArchiveIndex:
		return base + "/artifacts/intent-archive/index.json"
	case ArtifactStatus:
		return base + "/status.json"
	default:
		return ""
	}
}

func CanonicalPath(slug string, id ArtifactID) (string, error) {
	if !validSlug(slug) || artifactIndex(id) < 0 {
		return "", transactionError(CodeInvalidPlan, id, "canonical-path", "the canonical artifact path cannot be derived", 5)
	}
	return canonicalRel(slug, id), nil
}

func ArchiveBlobPath(slug, digest string) (string, error) {
	if !validSlug(slug) || !validHash(digest) {
		return "", transactionError(CodeInvalidPlan, "", "archive-path", "the archive blob path cannot be derived", 5)
	}
	return featureRel(slug) + "/artifacts/intent-archive/blobs/" + digest + ".blob", nil
}

func validStageRel(slug, rel string) bool {
	prefix := laneRel(slug)
	if !contained(prefix, rel) || path.Dir(rel) != prefix {
		return false
	}
	base := path.Base(rel)
	return strings.HasPrefix(base, "stage-") && validHex(base[len("stage-"):], 12)
}

func validHash(value string) bool {
	return validHex(value, 64)
}

func validHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
