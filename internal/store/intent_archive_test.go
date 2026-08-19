package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"testing"
)

type archiveMemoryBlob struct {
	kind    IntentArchiveBlobKind
	data    []byte
	version int
}

type archiveMemoryStorage struct {
	indexExists  bool
	index        []byte
	indexVersion int
	indexHistory [][]byte
	blobs        map[string]archiveMemoryBlob
	calls        []string
	fail         map[string]int
	postError    map[string]error
	hooks        map[string]func(*archiveMemoryStorage)
}

func newArchiveMemoryStorage(t *testing.T, index IntentArchiveIndex) *archiveMemoryStorage {
	t.Helper()
	wire, err := EncodeIntentArchiveIndex(index)
	if err != nil {
		t.Fatalf("EncodeIntentArchiveIndex: %v", err)
	}
	return &archiveMemoryStorage{
		indexExists:  true,
		index:        wire,
		indexVersion: 1,
		blobs:        map[string]archiveMemoryBlob{},
		fail:         map[string]int{},
		postError:    map[string]error{},
		hooks:        map[string]func(*archiveMemoryStorage){},
	}
}

func newEmptyArchiveMemoryStorage() *archiveMemoryStorage {
	return &archiveMemoryStorage{
		blobs:     map[string]archiveMemoryBlob{},
		fail:      map[string]int{},
		postError: map[string]error{},
		hooks:     map[string]func(*archiveMemoryStorage){},
	}
}

func (m *archiveMemoryStorage) CaptureIndex(indexRel string) (IntentArchiveIndexCapture, error) {
	if err := m.call("capture-index"); err != nil {
		return IntentArchiveIndexCapture{}, err
	}
	return IntentArchiveIndexCapture{
		Exists:   m.indexExists,
		Raw:      append([]byte(nil), m.index...),
		Identity: m.indexIdentity(),
	}, nil
}

func (m *archiveMemoryStorage) EnumerateBlobs(blobsRel string) ([]string, error) {
	if err := m.call("enumerate-blobs"); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(m.blobs))
	for rel := range m.blobs {
		if path.Dir(rel) == blobsRel {
			paths = append(paths, rel)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func (m *archiveMemoryStorage) ProbeBlob(blobRel string) (IntentArchiveBlobProbe, error) {
	if err := m.call("probe:" + path.Base(blobRel)); err != nil {
		return IntentArchiveBlobProbe{}, err
	}
	blob, ok := m.blobs[blobRel]
	if !ok {
		return IntentArchiveBlobProbe{
			Kind:     IntentArchiveBlobKindAbsent,
			Identity: IntentArchiveIdentityToken("absent:" + blobRel),
		}, nil
	}
	probe := IntentArchiveBlobProbe{
		Kind:      blob.kind,
		SizeBytes: int64(len(blob.data)),
		Identity:  m.blobIdentity(blobRel, blob),
	}
	if blob.kind == IntentArchiveBlobKindRegular {
		sum := sha256.Sum256(blob.data)
		probe.SHA256 = hex.EncodeToString(sum[:])
	}
	return probe, nil
}

func (m *archiveMemoryStorage) PreflightIndexCAS(indexRel string, expected IntentArchiveIdentityToken) error {
	if err := m.call("preflight-index-cas"); err != nil {
		return err
	}
	if expected != m.indexIdentity() {
		return &IntentArchiveError{Code: IntentArchiveCodePurgeIndexChanged, ExitClass: 3}
	}
	return nil
}

func (m *archiveMemoryStorage) PreflightBlobRemove(blobRel string, expected IntentArchiveIdentityToken) error {
	if err := m.call("preflight-remove:" + path.Base(blobRel)); err != nil {
		return err
	}
	blob, ok := m.blobs[blobRel]
	if !ok || expected != m.blobIdentity(blobRel, blob) {
		return &IntentArchiveError{Code: IntentArchiveCodeBlobCorrupt, ExitClass: 3}
	}
	return nil
}

func (m *archiveMemoryStorage) PublishBlob(blobRel, contentSHA256 string, data []byte) (IntentArchiveMutationResult, error) {
	if err := m.call("publish:" + path.Base(blobRel)); err != nil {
		return IntentArchiveMutationResult{}, err
	}
	if blob, ok := m.blobs[blobRel]; ok {
		if blob.kind != IntentArchiveBlobKindRegular || !bytes.Equal(blob.data, data) {
			return IntentArchiveMutationResult{}, &IntentArchiveError{
				Code:      IntentArchiveCodeBlobCorrupt,
				Hash:      contentSHA256,
				ExitClass: 3,
			}
		}
		return IntentArchiveMutationResult{Reused: true, Phase: IntentArchiveStoragePhaseValidated}, nil
	}
	m.blobs[blobRel] = archiveMemoryBlob{
		kind:    IntentArchiveBlobKindRegular,
		data:    append([]byte(nil), data...),
		version: 1,
	}
	result := IntentArchiveMutationResult{Committed: true, Phase: IntentArchiveStoragePhaseRenamed}
	if err := m.takePostError("publish:" + path.Base(blobRel)); err != nil {
		return result, err
	}
	return result, nil
}

func (m *archiveMemoryStorage) CASIndex(indexRel string, expected IntentArchiveIdentityToken, canonical []byte) (IntentArchiveMutationResult, error) {
	if hook := m.hooks["before-index-cas"]; hook != nil {
		hook(m)
	}
	if err := m.call("cas-index"); err != nil {
		return IntentArchiveMutationResult{}, err
	}
	if expected != m.indexIdentity() {
		return IntentArchiveMutationResult{}, &IntentArchiveError{
			Code:      IntentArchiveCodePurgeIndexChanged,
			ExitClass: 3,
		}
	}
	m.indexExists = true
	m.index = append([]byte(nil), canonical...)
	m.indexVersion++
	m.indexHistory = append(m.indexHistory, append([]byte(nil), canonical...))
	result := IntentArchiveMutationResult{Committed: true, Phase: IntentArchiveStoragePhaseRenamed}
	if hook := m.hooks["after-index-cas"]; hook != nil {
		hook(m)
	}
	return result, nil
}

func (m *archiveMemoryStorage) RemoveBlob(blobRel string, expected IntentArchiveIdentityToken) (IntentArchiveMutationResult, error) {
	if err := m.call("remove:" + path.Base(blobRel)); err != nil {
		return IntentArchiveMutationResult{}, err
	}
	blob, ok := m.blobs[blobRel]
	if !ok || expected != m.blobIdentity(blobRel, blob) {
		return IntentArchiveMutationResult{}, &IntentArchiveError{
			Code:      IntentArchiveCodeBlobCorrupt,
			ExitClass: 3,
		}
	}
	if hook := m.hooks["after-remove-revalidate"]; hook != nil {
		hook(m)
	}
	delete(m.blobs, blobRel)
	if hook := m.hooks["after-remove"]; hook != nil {
		hook(m)
	}
	result := IntentArchiveMutationResult{Committed: true, Phase: IntentArchiveStoragePhaseRemoved}
	if err := m.takePostError("remove:" + path.Base(blobRel)); err != nil {
		return result, err
	}
	return result, nil
}

func (m *archiveMemoryStorage) SyncDirectory(dirRel string) error {
	return m.call("sync:" + path.Base(dirRel))
}

func (m *archiveMemoryStorage) call(name string) error {
	m.calls = append(m.calls, name)
	if hook := m.hooks["before:"+name]; hook != nil {
		hook(m)
	}
	if remaining := m.fail[name]; remaining > 0 {
		m.fail[name] = remaining - 1
		return errors.New("injected archive storage failure")
	}
	return nil
}

func (m *archiveMemoryStorage) takePostError(name string) error {
	err := m.postError[name]
	delete(m.postError, name)
	return err
}

func (m *archiveMemoryStorage) indexIdentity() IntentArchiveIdentityToken {
	if !m.indexExists {
		return IntentArchiveIdentityToken(fmt.Sprintf("index-absent:%d", m.indexVersion))
	}
	sum := sha256.Sum256(m.index)
	return IntentArchiveIdentityToken(fmt.Sprintf("index:%d:%x", m.indexVersion, sum[:]))
}

func (m *archiveMemoryStorage) blobIdentity(rel string, blob archiveMemoryBlob) IntentArchiveIdentityToken {
	sum := sha256.Sum256(blob.data)
	return IntentArchiveIdentityToken(fmt.Sprintf("blob:%s:%s:%d:%x", rel, blob.kind, blob.version, sum[:]))
}

func (m *archiveMemoryStorage) putRegular(feature, hash string, data []byte) string {
	rel, _ := IntentArchiveBlobRel(feature, hash)
	m.blobs[rel] = archiveMemoryBlob{
		kind:    IntentArchiveBlobKindRegular,
		data:    append([]byte(nil), data...),
		version: 1,
	}
	return rel
}

func (m *archiveMemoryStorage) putNonRegular(feature, hash string) string {
	return m.putKind(feature, hash, IntentArchiveBlobKindNonRegular)
}

func (m *archiveMemoryStorage) putKind(feature, hash string, kind IntentArchiveBlobKind) string {
	rel, _ := IntentArchiveBlobRel(feature, hash)
	m.blobs[rel] = archiveMemoryBlob{
		kind:    kind,
		version: 1,
	}
	return rel
}

func (m *archiveMemoryStorage) decodedIndex(t *testing.T, feature string) IntentArchiveIndex {
	t.Helper()
	index, err := DecodeIntentArchiveIndex(m.index, feature)
	if err != nil {
		t.Fatalf("DecodeIntentArchiveIndex: %v", err)
	}
	return index
}

func (m *archiveMemoryStorage) externalSetIndex(t *testing.T, index IntentArchiveIndex) {
	t.Helper()
	wire, err := EncodeIntentArchiveIndex(index)
	if err != nil {
		t.Fatalf("EncodeIntentArchiveIndex: %v", err)
	}
	m.indexExists = true
	m.index = wire
	m.indexVersion++
}

func (m *archiveMemoryStorage) mutationCalls() []string {
	var calls []string
	for _, call := range m.calls {
		if strings.HasPrefix(call, "publish:") ||
			call == "cas-index" ||
			strings.HasPrefix(call, "remove:") ||
			strings.HasPrefix(call, "sync:") {
			calls = append(calls, call)
		}
	}
	return calls
}

func archiveHash(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}

func archiveReplacement(t *testing.T, id IntentArchiveArtifactID, data string, state IntentArchiveWireState) IntentArchiveReplacement {
	t.Helper()
	artifactPath, err := IntentArchiveArtifactPath(id)
	if err != nil {
		t.Fatal(err)
	}
	hash := archiveHash(data)
	replacement := IntentArchiveReplacement{
		ArtifactID:    id,
		Path:          artifactPath,
		ContentSHA256: hash,
		SizeBytes:     int64(len(data)),
	}
	setIntentArchiveReplacementState(&replacement, state)
	return replacement
}

func archiveGeneration(t *testing.T, feature string, replacements ...IntentArchiveReplacement) IntentArchiveGeneration {
	t.Helper()
	sort.SliceStable(replacements, func(i, j int) bool {
		return replacements[i].ArtifactID < replacements[j].ArtifactID
	})
	id, _, err := ComputeIntentArchiveGenerationID(feature, replacements)
	if err != nil {
		t.Fatalf("ComputeIntentArchiveGenerationID: %v", err)
	}
	return IntentArchiveGeneration{
		GenerationID: id,
		Mode:         IntentArchiveModeRegenerate,
		Replaced:     replacements,
	}
}

func archiveIndex(t *testing.T, feature string, generations ...IntentArchiveGeneration) IntentArchiveIndex {
	t.Helper()
	if generations == nil {
		generations = []IntentArchiveGeneration{}
	}
	index := IntentArchiveIndex{
		SchemaVersion: IntentArchiveSchemaVersion,
		Feature:       feature,
		Generations:   generations,
	}
	if err := ValidateIntentArchiveIndex(index, feature); err != nil {
		t.Fatalf("ValidateIntentArchiveIndex: %v", err)
	}
	return index
}

func archiveObservation(feature, hash string, state IntentArchiveBlobState, size int64) IntentArchiveBlobObservation {
	rel, _ := IntentArchiveBlobRel(feature, hash)
	return IntentArchiveBlobObservation{
		Hash:      hash,
		Path:      rel,
		State:     state,
		SizeBytes: size,
	}
}

func assertArchiveCode(t *testing.T, err error, code IntentArchiveErrorCode) *IntentArchiveError {
	t.Helper()
	var typed *IntentArchiveError
	if !errors.As(err, &typed) {
		t.Fatalf("error %v is not *IntentArchiveError", err)
	}
	if typed.Code != code {
		t.Fatalf("code = %q, want %q (error %v)", typed.Code, code, err)
	}
	return typed
}

func callIndex(calls []string, prefix string) int {
	for index, call := range calls {
		if strings.HasPrefix(call, prefix) {
			return index
		}
	}
	return -1
}
