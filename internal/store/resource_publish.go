// Tracked capture publication for
// PRD-feature-resource-claims-and-capture-adapters §7.3 / §12.3-§12.4.
//
// The tracked tree is an unordered, content-addressed *set* of
// immutable batch files plus one atomically-rewritten pointer. Nothing
// here records an ordering, a sequence number, or a timestamp: a batch
// file names exactly one distinct piece of content, and current.json
// is the sole authoritative statement of state.

package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ToolIdentity is the static file-fact identity of an adapter binary.
// The resolved absolute path is never tracked — only the basename and
// the pinned digest (§6.1).
type ToolIdentity struct {
	Basename     string `json:"basename"`
	BinarySHA256 string `json:"binary_sha256"`
}

// RawInfo summarizes the never-persisted raw bytes a capture read.
// It is populated for adapter-snapshot/ignored-file and null for
// git-metadata, where no raw-byte concept applies.
type RawInfo struct {
	Hash      string `json:"hash"`
	ByteCount uint64 `json:"byte_count"`
}

// BatchResult is one resource's staged outcome inside a batch. Field
// order here is the declared order used by both the file wire format
// and CanonicalBatchJSON.
type BatchResult struct {
	ResourceID   string        `json:"resource_id"`
	Kind         string        `json:"kind"`
	Selector     string        `json:"selector"`
	Adapter      string        `json:"adapter"`
	Capability   string        `json:"capability"`
	Args         []ResourceArg `json:"args"`
	ToolIdentity *ToolIdentity `json:"tool_identity"`
	Result       CanonNode     `json:"result"`
	Raw          *RawInfo      `json:"raw"`
}

// canonNode renders the result entry as a fixed-field canonical object
// in declared field order.
func (r BatchResult) canonNode() CanonNode {
	args := make([]ResourceArg, len(r.Args))
	copy(args, r.Args)
	sort.SliceStable(args, func(i, j int) bool { return args[i].Key < args[j].Key })
	argNodes := make([]CanonNode, 0, len(args))
	for _, a := range args {
		argNodes = append(argNodes, CanonObject(
			CanonFieldOf("key", CanonString(a.Key)),
			CanonFieldOf("value", CanonString(a.Value)),
		))
	}
	tool := CanonNull()
	if r.ToolIdentity != nil {
		tool = CanonObject(
			CanonFieldOf("basename", CanonString(r.ToolIdentity.Basename)),
			CanonFieldOf("binary_sha256", CanonString(r.ToolIdentity.BinarySHA256)),
		)
	}
	raw := CanonNull()
	if r.Raw != nil {
		raw = CanonObject(
			CanonFieldOf("hash", CanonString(r.Raw.Hash)),
			CanonFieldOf("byte_count", CanonUint(r.Raw.ByteCount)),
		)
	}
	return CanonObject(
		CanonFieldOf("resource_id", CanonString(r.ResourceID)),
		CanonFieldOf("kind", CanonString(r.Kind)),
		CanonFieldOf("selector", CanonString(r.Selector)),
		CanonFieldOf("adapter", CanonString(r.Adapter)),
		CanonFieldOf("capability", CanonString(r.Capability)),
		CanonFieldOf("args", CanonArrayOf(argNodes)),
		CanonFieldOf("tool_identity", tool),
		CanonFieldOf("result", r.Result),
		CanonFieldOf("raw", raw),
	)
}

// Batch is one immutable batches/<batch_id>.json file.
type Batch struct {
	BatchID string        `json:"batch_id"`
	Feature string        `json:"feature"`
	Results []BatchResult `json:"results"`
}

// CurrentResourceEntry maps one resource to the batch holding its
// current result.
type CurrentResourceEntry struct {
	ResourceID string `json:"resource_id"`
	BatchID    string `json:"batch_id"`
}

// CurrentPointer is the tracked current.json file.
//
// CurrentBatchID is this file's own provenance fact — the batch the
// invocation that most recently rewrote this file staged. It is not a
// claim that the batch is chronologically newer than any other; this
// design tracks no ordering at all (§12.4).
type CurrentPointer struct {
	CurrentBatchID string                 `json:"current_batch_id"`
	Resources      []CurrentResourceEntry `json:"resources"`
}

// BatchFor returns the batch ID currently recorded for a resource.
func (c CurrentPointer) BatchFor(resourceID string) (string, bool) {
	for _, e := range c.Resources {
		if e.ResourceID == resourceID {
			return e.BatchID, true
		}
	}
	return "", false
}

// ComputeBatchID hashes the canonical batch body and returns the full,
// untruncated content-addressed ID (§7.3 step 2).
func ComputeBatchID(feature string, results []BatchResult) (string, []byte, error) {
	canonical, err := CanonicalBatchJSON(feature, results)
	if err != nil {
		return "", nil, err
	}
	digest := sha256.Sum256(canonical)
	return BatchIDPrefix + hex.EncodeToString(digest[:]), canonical, nil
}

// BatchFileWireBytes returns the exact, complete on-disk
// representation of a batch file: ordinary encoding/json over the
// fixed-field struct, 2-space indented, trailing newline. This is what
// §7.3 step 3's idempotency check compares against — never the
// compact hash-input bytes.
func BatchFileWireBytes(b Batch) ([]byte, error) {
	if b.Results == nil {
		b.Results = []BatchResult{}
	}
	for i := range b.Results {
		if b.Results[i].Args == nil {
			b.Results[i].Args = []ResourceArg{}
		}
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// PublicationError is a named, tracked-write refusal.
type PublicationError struct {
	Reason  string
	BatchID string
	Detail  string
}

// Error satisfies the error interface.
func (e *PublicationError) Error() string {
	if e.BatchID == "" {
		return fmt.Sprintf("%s: %s", e.Reason, e.Detail)
	}
	return fmt.Sprintf("%s: %s (%s)", e.Reason, e.Detail, e.BatchID)
}

// Named publication refusals.
const (
	ReasonBatchIDCollision    = "batch-id-collision"
	ReasonBatchFileCorrupt    = "batch-file-corrupt"
	ReasonTrackedBatchMissing = "tracked-batch-missing"
)

// PublishOutcome reports what the publication transaction actually did
// so callers can print an honest summary.
type PublishOutcome struct {
	BatchID      string
	WroteBatch   bool
	DriftIgnored bool
}

// EnsureResourceCaptureTree creates the tracked capture directories and
// fsyncs the whole chain unconditionally, including directories that
// already existed — a retried invocation after a crash cannot assume a
// Stat-visible directory is already crash-durable (§7.1 step 4, §7.3's
// first-publication crash row).
func (s *Store) EnsureResourceCaptureTree(slug string) error {
	return MkdirAllAndSyncChain(s.ResourceBatchesDir(slug), s.Root, 0o755)
}

// PublishBatch runs §7.3 steps 3-4: write the immutable batch (or
// recognize an already-published identical one), then atomically
// rewrite the pointer. It is the single commit point of a capture.
//
// staged maps each targeted resource_id to the batch it should now
// point at; every other previously-tracked resource's entry is carried
// forward unchanged.
func (s *Store) PublishBatch(slug string, batch Batch, canonical []byte) (PublishOutcome, error) {
	outcome := PublishOutcome{BatchID: batch.BatchID}
	if err := s.EnsureResourceCaptureTree(slug); err != nil {
		return outcome, err
	}
	wire, err := BatchFileWireBytes(batch)
	if err != nil {
		return outcome, err
	}
	batchPath := s.ResourceBatchPath(slug, batch.BatchID)
	existing, readErr := os.ReadFile(batchPath)
	switch {
	case readErr == nil && bytes.Equal(existing, wire):
		// Already fully written by a prior (possibly
		// crashed-before-pointer) invocation: idempotent re-publish.
	case readErr == nil:
		drift, err := compareSemanticBody(existing, batch.BatchID, canonical)
		if err != nil {
			return outcome, err
		}
		// Semantic bodies match: presentation-only drift. The
		// immutable file is never rewritten in place.
		outcome.DriftIgnored = drift
	case errors.Is(readErr, os.ErrNotExist):
		if err := writeBatchFile(batchPath, wire); err != nil {
			return outcome, err
		}
		outcome.WroteBatch = true
	default:
		return outcome, readErr
	}

	pointer, err := s.LoadCurrentPointer(slug)
	if err != nil {
		return outcome, err
	}
	next := CurrentPointer{CurrentBatchID: batch.BatchID}
	byID := map[string]string{}
	for _, e := range pointer.Resources {
		byID[e.ResourceID] = e.BatchID
	}
	for _, r := range batch.Results {
		byID[r.ResourceID] = batch.BatchID
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	next.Resources = make([]CurrentResourceEntry, 0, len(ids))
	for _, id := range ids {
		next.Resources = append(next.Resources, CurrentResourceEntry{ResourceID: id, BatchID: byID[id]})
	}
	if err := s.writeCurrentPointer(slug, next); err != nil {
		return outcome, err
	}
	return outcome, nil
}

// compareSemanticBody implements §7.3 step 3's drift-vs-collision
// split: decode the on-disk file, verify its own batch_id field,
// re-canonicalize its body, and compare against this invocation's
// hash-input bytes. Byte inequality alone never means "collision".
func compareSemanticBody(existing []byte, batchID string, canonical []byte) (bool, error) {
	var onDisk Batch
	dec := json.NewDecoder(bytes.NewReader(existing))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&onDisk); err != nil {
		return false, &PublicationError{
			Reason:  ReasonBatchFileCorrupt,
			BatchID: batchID,
			Detail:  fmt.Sprintf("existing batch file does not parse: %v", err),
		}
	}
	if onDisk.BatchID != batchID {
		return false, &PublicationError{
			Reason:  ReasonBatchFileCorrupt,
			BatchID: batchID,
			Detail:  fmt.Sprintf("existing batch file records batch_id %q", onDisk.BatchID),
		}
	}
	body, err := canonicalizeDecodedBatchBody(onDisk)
	if err != nil {
		return false, &PublicationError{
			Reason:  ReasonBatchFileCorrupt,
			BatchID: batchID,
			Detail:  fmt.Sprintf("existing batch file cannot be canonicalized: %v", err),
		}
	}
	if !bytes.Equal(body, canonical) {
		return false, &PublicationError{
			Reason:  ReasonBatchIDCollision,
			BatchID: batchID,
			Detail:  "two distinct staged contents produced the identical batch_id",
		}
	}
	return true, nil
}

// writeBatchFile writes the file-wire bytes through a same-directory
// temp with a per-attempt random suffix, fsyncs it, renames, and
// fsyncs the directory.
func writeBatchFile(batchPath string, wire []byte) error {
	dir := filepath.Dir(batchPath)
	suffix, err := RandomHex12()
	if err != nil {
		return err
	}
	base := filepath.Base(batchPath)
	tmpPath := filepath.Join(dir, base[:len(base)-len(".json")]+".tmp-"+suffix+".json")
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(wire); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, batchPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return SyncDir(dir)
}

// writeCurrentPointer performs the single atomic commit point of the
// whole capture.
func (s *Store) writeCurrentPointer(slug string, pointer CurrentPointer) error {
	if pointer.Resources == nil {
		pointer.Resources = []CurrentResourceEntry{}
	}
	data, err := json.MarshalIndent(pointer, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmpPath := s.ResourceCurrentTempPath(slug)
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, s.ResourceCurrentPath(slug)); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return SyncDir(s.ResourceCapturesDir(slug))
}

// LoadCurrentPointer reads the tracked pointer. A missing file is an
// empty pointer, not an error (no capture has ever run).
func (s *Store) LoadCurrentPointer(slug string) (CurrentPointer, error) {
	data, err := os.ReadFile(s.ResourceCurrentPath(slug))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return CurrentPointer{Resources: []CurrentResourceEntry{}}, nil
		}
		return CurrentPointer{}, err
	}
	var p CurrentPointer
	if err := json.Unmarshal(data, &p); err != nil {
		return CurrentPointer{}, &PublicationError{
			Reason: ReasonBatchFileCorrupt,
			Detail: fmt.Sprintf("current.json does not parse: %v", err),
		}
	}
	if p.Resources == nil {
		p.Resources = []CurrentResourceEntry{}
	}
	return p, nil
}

// LoadBatch reads one immutable batch file. A referenced-but-absent
// file is `tracked-batch-missing` — a data-integrity condition
// distinct from "no capture yet" (§4.1).
func (s *Store) LoadBatch(slug, batchID string) (Batch, error) {
	data, err := os.ReadFile(s.ResourceBatchPath(slug, batchID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Batch{}, &PublicationError{
				Reason:  ReasonTrackedBatchMissing,
				BatchID: batchID,
				Detail:  "current.json references a batch file that is not present",
			}
		}
		return Batch{}, err
	}
	var b Batch
	if err := json.Unmarshal(data, &b); err != nil {
		return Batch{}, &PublicationError{
			Reason:  ReasonBatchFileCorrupt,
			BatchID: batchID,
			Detail:  fmt.Sprintf("batch file does not parse: %v", err),
		}
	}
	return b, nil
}

// SweepTrackedTempArtifacts removes leftover tracked temp files from a
// crashed prior invocation. Removal is best-effort; the returned
// diagnostics list names anything that could not be removed.
func (s *Store) SweepTrackedTempArtifacts(slug string) []string {
	var diags []string
	matches, err := filepath.Glob(filepath.Join(s.ResourceBatchesDir(slug), "*.tmp-*.json"))
	if err == nil {
		for _, m := range matches {
			if rmErr := os.Remove(m); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
				diags = append(diags, fmt.Sprintf("could not remove orphan temp %s: %v", filepath.Base(m), rmErr))
			}
		}
	}
	tmpPointer := s.ResourceCurrentTempPath(slug)
	if rmErr := os.Remove(tmpPointer); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		diags = append(diags, fmt.Sprintf("could not remove orphan temp %s: %v", filepath.Base(tmpPointer), rmErr))
	}
	return diags
}
