// Wire-domain tests for the resource-capture store
// (PRD-feature-resource-claims-and-capture-adapters §12/§13, ADR-033
// D3/D7/D11).
//
// AC coverage claimed here is enumerated in
// internal/rescap/ac_coverage_test.go; every test name below appears
// there against the AC/matrix rows it discharges.

package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newResourceTestStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	s, err := Init(root)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := os.MkdirAll(s.featureDir("model-picker"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(s.featureStatusPath("model-picker"), []byte(`{"slug":"model-picker"}`), 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}
	return s
}

// TestGoldenResourceIDVectors pins all four §13.3 / ADR-033 D3 golden
// vectors, including vector 3's order-independence.
func TestGoldenResourceIDVectors(t *testing.T) {
	cases := []struct {
		name       string
		kind       string
		selector   string
		adapter    string
		capability string
		args       []ResourceArg
		want       string
	}{
		{
			name: "vector1-git-metadata-head",
			kind: ResourceKindGitMetadata, selector: "head",
			want: "res_acc91dc23a8b",
		},
		{
			name: "vector2-dolt-declaration-order",
			kind: ResourceKindAdapterSnapshot, selector: "dolt:diff-summary:users",
			adapter: "dolt", capability: "diff-summary",
			args: []ResourceArg{
				{"contract", "dolt-diff-summary-v1"},
				{"db_path", "data/dolt-db"},
				{"table", "users"},
				{"from", "main"},
				{"to", "HEAD"},
			},
			want: "res_4b62313b6cce",
		},
		{
			name: "vector3-reordered-args-identical-id",
			kind: ResourceKindAdapterSnapshot, selector: "dolt:diff-summary:users",
			adapter: "dolt", capability: "diff-summary",
			args: []ResourceArg{
				{"to", "HEAD"},
				{"db_path", "data/dolt-db"},
				{"table", "users"},
				{"from", "main"},
				{"contract", "dolt-diff-summary-v1"},
			},
			want: "res_4b62313b6cce",
		},
		{
			name: "vector4-ignored-file",
			kind: ResourceKindIgnoredFile, selector: "config/local-secrets.env.template",
			want: "res_79f5ac5dca13",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeResourceID("model-picker", tc.kind, tc.selector, tc.adapter, tc.capability, tc.args)
			if got != tc.want {
				t.Fatalf("resource_id = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestTrustExcludedFromResourceIdentity proves the pin is mutable
// metadata: two declarations differing only in trust share one ID.
func TestTrustExcludedFromResourceIdentity(t *testing.T) {
	args := []ResourceArg{
		{"contract", "dolt-diff-summary-v1"},
		{"db_path", "data/dolt-db"},
		{"from", "main"},
		{"table", "users"},
		{"to", "HEAD"},
	}
	a := Resource{
		ResourceID: "res_4b62313b6cce", Kind: ResourceKindAdapterSnapshot,
		Selector: "dolt:diff-summary:users", Adapter: "dolt", Capability: "diff-summary",
		Args: args, Trust: &ResourceTrust{BinarySHA256: strings.Repeat("a", 64)},
	}
	b := a
	b.Trust = &ResourceTrust{BinarySHA256: strings.Repeat("b", 64)}
	_, idA := a.Identity("model-picker")
	_, idB := b.Identity("model-picker")
	if idA != idB {
		t.Fatalf("trust perturbed identity: %s != %s", idA, idB)
	}
	if idA != "res_4b62313b6cce" {
		t.Fatalf("identity drifted from the golden vector: %s", idA)
	}
}

// TestCanonicalArgsJSONEncoding pins §13.1's encoding rules: sorted
// keys, no whitespace, only \ and " escaped (never HTML escaping), and
// `{}` for the empty set.
func TestCanonicalArgsJSONEncoding(t *testing.T) {
	if got := CanonicalArgsJSON(nil); got != "{}" {
		t.Fatalf("empty args = %q, want {}", got)
	}
	got := CanonicalArgsJSON([]ResourceArg{
		{"z", `a"b`},
		{"a", `c\d`},
		{"m", "<html>&"},
	})
	want := `{"a":"c\\d","m":"<html>&","z":"a\"b"}`
	if got != want {
		t.Fatalf("canonical args = %s, want %s", got, want)
	}
	if strings.Contains(got, `\u003c`) {
		t.Fatal("canonical args must not HTML-escape")
	}
}

// TestControlByteRejection covers §13.1 rule 6's detection primitive.
func TestControlByteRejection(t *testing.T) {
	for _, s := range []string{"a\x00b", "a\x01b", "tab\there", "nl\nhere", "del\x7f"} {
		if !HasControlBytes(s) {
			t.Fatalf("%q should be rejected", s)
		}
	}
	for _, s := range []string{"plain", "with space", "utf8-é", "dolt:diff-summary:users"} {
		if HasControlBytes(s) {
			t.Fatalf("%q should be accepted", s)
		}
	}
}

// goldenBatch builds the exact §12.3 worked example.
func goldenBatch() Batch {
	return Batch{
		BatchID: "rb_507f520c56f892f882bb06f6e8117040f605fcd06f99f3217fad4b95bc4f1021",
		Feature: "model-picker",
		Results: []BatchResult{
			{
				ResourceID: "res_4b62313b6cce", Kind: ResourceKindAdapterSnapshot,
				Selector: "dolt:diff-summary:users", Adapter: "dolt", Capability: "diff-summary",
				Args: []ResourceArg{
					{"contract", "dolt-diff-summary-v1"},
					{"db_path", "data/dolt-db"},
					{"from", "main"},
					{"table", "users"},
					{"to", "HEAD"},
				},
				ToolIdentity: &ToolIdentity{
					Basename:     "dolt",
					BinarySHA256: "3f9c8e1a2d4c5b6a7e8f9d0c1b2a3e4d5c6b7a8f9e0d1c2b3a4e5f607b0f6f7b",
				},
				Result: CanonObject(CanonFieldOf("tables", CanonArray(CanonObject(
					CanonFieldOf("from_table_name", CanonString("users")),
					CanonFieldOf("to_table_name", CanonString("users")),
					CanonFieldOf("diff_type", CanonString("modified")),
					CanonFieldOf("data_change", CanonBool(true)),
					CanonFieldOf("schema_change", CanonBool(false)),
				)))),
				Raw: &RawInfo{
					Hash:      "sha256:9d0c1b2a3e4d5c6b7a8f9e0d1c2b3a4e5f607b0f6f7b3f9c8e1a2d4c5b6a7e8f",
					ByteCount: 187,
				},
			},
			{
				ResourceID: "res_79f5ac5dca13", Kind: ResourceKindIgnoredFile,
				Selector: "config/local-secrets.env.template",
				Args:     []ResourceArg{},
				Result: CanonObject(
					CanonFieldOf("file_kind", CanonString("text")),
					CanonFieldOf("size_bytes", CanonUint(214)),
					CanonFieldOf("hash", CanonString("sha256:7b0f6f7b3f9c8e1a2d4c5b6a7e8f9d0c1b2a3e4d5c6b7a8f9e0d1c2b3a4e5f60")),
				),
				Raw: &RawInfo{
					Hash:      "sha256:7b0f6f7b3f9c8e1a2d4c5b6a7e8f9d0c1b2a3e4d5c6b7a8f9e0d1c2b3a4e5f60",
					ByteCount: 214,
				},
			},
			{
				ResourceID: "res_acc91dc23a8b", Kind: ResourceKindGitMetadata,
				Selector: "head",
				Args:     []ResourceArg{},
				Result: CanonObject(
					CanonFieldOf("symbolic_ref", CanonString("refs/heads/main")),
					CanonFieldOf("oid", CanonString("9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08")),
					CanonFieldOf("detached", CanonBool(false)),
				),
			},
		},
	}
}

// TestGoldenBatchIDVector reproduces §12.3's exact worked batch_id from
// the canonical hash input.
func TestGoldenBatchIDVector(t *testing.T) {
	b := goldenBatch()
	got, canonical, err := ComputeBatchID(b.Feature, b.Results)
	if err != nil {
		t.Fatalf("ComputeBatchID: %v", err)
	}
	if got != b.BatchID {
		t.Fatalf("batch_id = %s, want %s", got, b.BatchID)
	}
	if strings.Contains(string(canonical), "batch_id") {
		t.Fatal("the hash input must never contain batch_id (no self-reference)")
	}
	if strings.Contains(string(canonical), "\n") || strings.Contains(string(canonical), "  ") {
		t.Fatal("the hash input must be compact and unindented")
	}
	// results are sorted by resource_id byte-ascending regardless of
	// input ordering.
	shuffled := []BatchResult{b.Results[2], b.Results[0], b.Results[1]}
	reordered, _, err := ComputeBatchID(b.Feature, shuffled)
	if err != nil {
		t.Fatalf("ComputeBatchID(reordered): %v", err)
	}
	if reordered != b.BatchID {
		t.Fatalf("result ordering perturbed batch_id: %s", reordered)
	}
}

// TestBatchFileWireShape pins the exact file-wire bytes: 2-space
// indent, trailing newline, batch_id present as an ordinary field,
// arrays never null, inapplicable fields present with explicit null.
func TestBatchFileWireShape(t *testing.T) {
	wire, err := BatchFileWireBytes(goldenBatch())
	if err != nil {
		t.Fatalf("BatchFileWireBytes: %v", err)
	}
	text := string(wire)
	if !strings.HasSuffix(text, "}\n") {
		t.Fatal("file wire must end with a trailing newline")
	}
	if !strings.Contains(text, "\n  \"batch_id\": \"rb_") {
		t.Fatal("batch_id must be an ordinary 2-space-indented field")
	}
	if !strings.Contains(text, `"args": []`) {
		t.Fatal("empty args must render as [], never null")
	}
	if !strings.Contains(text, `"tool_identity": null`) {
		t.Fatal("inapplicable tool_identity must be present with an explicit null")
	}
	if !strings.Contains(text, `"raw": null`) {
		t.Fatal("git-metadata raw must be present with an explicit null")
	}
	if strings.Contains(text, "timestamp") || strings.Contains(text, "created_at") {
		t.Fatal("no tracked file may contain a wall-clock timestamp field")
	}
	// The file wire round-trips through the decoder with field order
	// preserved, which is what the §7.3 step 3 comparison depends on.
	var decoded Batch
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	again, err := BatchFileWireBytes(decoded)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	if string(again) != text {
		t.Fatal("file wire does not round-trip byte-identically")
	}
}

// TestCanonNodeRejectsDuplicateKeys proves the order-preserving decoder
// refuses a tracked artifact with two entries for one key.
func TestCanonNodeRejectsDuplicateKeys(t *testing.T) {
	var n CanonNode
	if err := n.UnmarshalJSON([]byte(`{"a":1,"a":2}`)); err == nil {
		t.Fatal("expected a duplicate-key refusal")
	}
}

// TestCanonNodePreservesFieldOrder proves decode order is the file's
// own order, not a sorted or map-iteration order.
func TestCanonNodePreservesFieldOrder(t *testing.T) {
	var n CanonNode
	if err := n.UnmarshalJSON([]byte(`{"z":1,"a":2,"m":3}`)); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var keys []string
	for _, f := range n.Object {
		keys = append(keys, f.Key)
	}
	if strings.Join(keys, ",") != "z,a,m" {
		t.Fatalf("field order = %v, want z,a,m", keys)
	}
}

// TestPublishBatchFirstWriteAndIdempotency covers §7.3 steps 3-4: the
// first write, the byte-identical idempotent re-publish (zero new batch
// bytes), and the pointer's carry-forward of untouched resources.
func TestPublishBatchFirstWriteAndIdempotency(t *testing.T) {
	s := newResourceTestStore(t)
	b := goldenBatch()
	_, canonical, err := ComputeBatchID(b.Feature, b.Results)
	if err != nil {
		t.Fatalf("ComputeBatchID: %v", err)
	}

	outcome, err := s.PublishBatch("model-picker", b, canonical)
	if err != nil {
		t.Fatalf("PublishBatch: %v", err)
	}
	if !outcome.WroteBatch {
		t.Fatal("the first publish must write the batch file")
	}
	pointer, err := s.LoadCurrentPointer("model-picker")
	if err != nil {
		t.Fatalf("LoadCurrentPointer: %v", err)
	}
	if pointer.CurrentBatchID != b.BatchID || len(pointer.Resources) != 3 {
		t.Fatalf("unexpected pointer: %+v", pointer)
	}

	outcome2, err := s.PublishBatch("model-picker", b, canonical)
	if err != nil {
		t.Fatalf("re-PublishBatch: %v", err)
	}
	if outcome2.WroteBatch {
		t.Fatal("an identical re-publish must write zero new batch bytes")
	}

	// A second invocation touching only one resource carries every
	// other resource's entry forward unchanged.
	partial := Batch{Feature: "model-picker", Results: []BatchResult{b.Results[2]}}
	partial.Results[0].Result = CanonObject(
		CanonFieldOf("symbolic_ref", CanonString("refs/heads/other")),
		CanonFieldOf("oid", CanonString("1111111111111111111111111111111111111111")),
		CanonFieldOf("detached", CanonBool(false)),
	)
	id, canon2, err := ComputeBatchID(partial.Feature, partial.Results)
	if err != nil {
		t.Fatalf("ComputeBatchID: %v", err)
	}
	partial.BatchID = id
	if _, err := s.PublishBatch("model-picker", partial, canon2); err != nil {
		t.Fatalf("partial PublishBatch: %v", err)
	}
	pointer, err = s.LoadCurrentPointer("model-picker")
	if err != nil {
		t.Fatalf("LoadCurrentPointer: %v", err)
	}
	if pointer.CurrentBatchID != id {
		t.Fatalf("current_batch_id = %s, want %s", pointer.CurrentBatchID, id)
	}
	carried, _ := pointer.BatchFor("res_4b62313b6cce")
	if carried != b.BatchID {
		t.Fatalf("untouched resource was not carried forward: %s", carried)
	}
	touched, _ := pointer.BatchFor("res_acc91dc23a8b")
	if touched != id {
		t.Fatalf("touched resource points at %s, want %s", touched, id)
	}
}

// TestPublishBatchPresentationDriftIsIdempotent proves a byte-level
// file-wire difference on semantically identical content is treated as
// presentation drift — the immutable file is not rewritten and the
// pointer publish proceeds — rather than as a collision.
func TestPublishBatchPresentationDriftIsIdempotent(t *testing.T) {
	s := newResourceTestStore(t)
	b := goldenBatch()
	_, canonical, _ := ComputeBatchID(b.Feature, b.Results)
	if _, err := s.PublishBatch("model-picker", b, canonical); err != nil {
		t.Fatalf("PublishBatch: %v", err)
	}
	path := s.ResourceBatchPath("model-picker", b.BatchID)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Re-encode with 4-space indentation: different bytes, identical
	// semantic body.
	var decoded Batch
	if err := json.Unmarshal(original, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	drifted, err := json.MarshalIndent(decoded, "", "    ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, append(drifted, '\n'), 0o644); err != nil {
		t.Fatalf("write drifted: %v", err)
	}
	outcome, err := s.PublishBatch("model-picker", b, canonical)
	if err != nil {
		t.Fatalf("drift publish refused: %v", err)
	}
	if !outcome.DriftIgnored {
		t.Fatal("presentation drift should be recognized as such")
	}
	if outcome.WroteBatch {
		t.Fatal("an already-published batch is never rewritten in place")
	}
	after, _ := os.ReadFile(path)
	if string(after) == string(original) {
		t.Fatal("the immutable file must be left exactly as found, drift included")
	}
}

// TestPublishBatchCollisionAndCorruption covers the two fatal branches
// §7.3 step 3 distinguishes.
func TestPublishBatchCollisionAndCorruption(t *testing.T) {
	t.Run("semantic-collision", func(t *testing.T) {
		s := newResourceTestStore(t)
		b := goldenBatch()
		_, canonical, _ := ComputeBatchID(b.Feature, b.Results)
		if _, err := s.PublishBatch("model-picker", b, canonical); err != nil {
			t.Fatalf("PublishBatch: %v", err)
		}
		// A genuinely different staged body claiming the same batch_id.
		colliding := goldenBatch()
		colliding.Results[2].Result = CanonObject(
			CanonFieldOf("symbolic_ref", CanonString("refs/heads/different")),
			CanonFieldOf("oid", CanonString("2222222222222222222222222222222222222222")),
			CanonFieldOf("detached", CanonBool(false)),
		)
		_, otherCanonical, _ := ComputeBatchID(colliding.Feature, colliding.Results)
		_, err := s.PublishBatch("model-picker", colliding, otherCanonical)
		var pubErr *PublicationError
		if err == nil || !asPublicationError(err, &pubErr) || pubErr.Reason != ReasonBatchIDCollision {
			t.Fatalf("want batch-id-collision, got %v", err)
		}
	})
	t.Run("unparseable-file", func(t *testing.T) {
		s := newResourceTestStore(t)
		b := goldenBatch()
		_, canonical, _ := ComputeBatchID(b.Feature, b.Results)
		if err := s.EnsureResourceCaptureTree("model-picker"); err != nil {
			t.Fatalf("EnsureResourceCaptureTree: %v", err)
		}
		path := s.ResourceBatchPath("model-picker", b.BatchID)
		if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		_, err := s.PublishBatch("model-picker", b, canonical)
		var pubErr *PublicationError
		if err == nil || !asPublicationError(err, &pubErr) || pubErr.Reason != ReasonBatchFileCorrupt {
			t.Fatalf("want batch-file-corrupt, got %v", err)
		}
	})
	t.Run("self-inconsistent-batch-id", func(t *testing.T) {
		s := newResourceTestStore(t)
		b := goldenBatch()
		_, canonical, _ := ComputeBatchID(b.Feature, b.Results)
		if err := s.EnsureResourceCaptureTree("model-picker"); err != nil {
			t.Fatalf("EnsureResourceCaptureTree: %v", err)
		}
		wrong := goldenBatch()
		wrong.BatchID = "rb_" + strings.Repeat("0", 64)
		wire, _ := BatchFileWireBytes(wrong)
		if err := os.WriteFile(s.ResourceBatchPath("model-picker", b.BatchID), wire, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		_, err := s.PublishBatch("model-picker", b, canonical)
		var pubErr *PublicationError
		if err == nil || !asPublicationError(err, &pubErr) || pubErr.Reason != ReasonBatchFileCorrupt {
			t.Fatalf("want batch-file-corrupt, got %v", err)
		}
	})
}

func asPublicationError(err error, target **PublicationError) bool {
	p, ok := err.(*PublicationError)
	if ok {
		*target = p
	}
	return ok
}

// TestTrackedBatchMissing covers §4.1: a pointer naming an absent batch
// file is a data-integrity condition with its own name.
func TestTrackedBatchMissing(t *testing.T) {
	s := newResourceTestStore(t)
	if err := s.EnsureResourceCaptureTree("model-picker"); err != nil {
		t.Fatalf("EnsureResourceCaptureTree: %v", err)
	}
	_, err := s.LoadBatch("model-picker", "rb_"+strings.Repeat("f", 64))
	p, ok := err.(*PublicationError)
	if !ok || p.Reason != ReasonTrackedBatchMissing {
		t.Fatalf("want tracked-batch-missing, got %v", err)
	}
}

// TestResourcesManifestCorruptionAndCollision covers §4's split
// taxonomy: one entry inconsistent with itself is
// resources-file-corrupt; two distinct entries sharing an ID is
// resource-id-collision.
func TestResourcesManifestCorruptionAndCollision(t *testing.T) {
	t.Run("self-inconsistent-entry", func(t *testing.T) {
		s := newResourceTestStore(t)
		body := `{"version":1,"feature":"model-picker","resources":[{"resource_id":"res_deadbeef0000","kind":"git-metadata","selector":"head","adapter":"","capability":"","args":[],"trust":null,"added_by_tool_version":"tpatch/test"}]}`
		writeManifest(t, s, body)
		_, err := LoadResources(s, "model-picker")
		m, ok := err.(*ResourceManifestError)
		if !ok || m.Reason != ReasonResourcesFileCorrupt {
			t.Fatalf("want resources-file-corrupt, got %v", err)
		}
	})
	t.Run("two-entries-one-id", func(t *testing.T) {
		// A real SHA-256 collision is not producible for a fixture, so
		// the derivation function itself is substituted. Production
		// code has no test-only branch on it.
		restore := SetResourceIDDeriverForTest(func(feature, kind, selector, adapter, capability string, args []ResourceArg) string {
			return "res_000000000000"
		})
		defer restore()
		resources := []Resource{
			{ResourceID: "res_000000000000", Kind: ResourceKindGitMetadata, Selector: "head", Args: []ResourceArg{}},
			{ResourceID: "res_000000000000", Kind: ResourceKindIgnoredFile, Selector: "other", Args: []ResourceArg{}},
		}
		err := ValidateLoadedResources("model-picker", resources)
		m, ok := err.(*ResourceManifestError)
		if !ok || m.Reason != ReasonResourceIDCollision {
			t.Fatalf("want resource-id-collision, got %v", err)
		}
	})
}

func writeManifest(t *testing.T, s *Store, body string) {
	t.Helper()
	path := s.ResourcesPath("model-picker")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// TestResourcesManifestRoundTrip proves a missing file is an empty
// manifest, entries are sorted by resource_id, and args are sorted by
// key.
func TestResourcesManifestRoundTrip(t *testing.T) {
	s := newResourceTestStore(t)
	m, err := LoadResources(s, "model-picker")
	if err != nil {
		t.Fatalf("LoadResources: %v", err)
	}
	if len(m.Resources) != 0 || m.Version != ResourcesManifestVersion {
		t.Fatalf("missing file should load as an empty manifest, got %+v", m)
	}
	add := func(kind, selector, adapter, capability string, args []ResourceArg) Resource {
		return Resource{
			ResourceID: ComputeResourceID("model-picker", kind, selector, adapter, capability, args),
			Kind:       kind, Selector: selector, Adapter: adapter, Capability: capability, Args: args,
		}
	}
	m.Resources = append(m.Resources,
		add(ResourceKindIgnoredFile, "config/local-secrets.env.template", "", "", nil),
		add(ResourceKindGitMetadata, "head", "", "", nil),
	)
	if err := SaveResources(s, m); err != nil {
		t.Fatalf("SaveResources: %v", err)
	}
	loaded, err := LoadResources(s, "model-picker")
	if err != nil {
		t.Fatalf("LoadResources: %v", err)
	}
	if len(loaded.Resources) != 2 {
		t.Fatalf("want 2 resources, got %d", len(loaded.Resources))
	}
	if loaded.Resources[0].ResourceID > loaded.Resources[1].ResourceID {
		t.Fatal("entries must be sorted by resource_id")
	}
}

// TestFindResourcePrefixResolution covers exact-ID, unambiguous-prefix
// and ambiguous-prefix handling.
func TestFindResourcePrefixResolution(t *testing.T) {
	m := &ResourcesManifest{Resources: []Resource{
		{ResourceID: "res_aaaaaaaaaaaa"},
		{ResourceID: "res_aaaaaaaaaaab"},
		{ResourceID: "res_bbbbbbbbbbbb"},
	}}
	if _, ok, err := FindResource(m, "res_bbbbbbbbbbbb"); !ok || err != nil {
		t.Fatalf("exact match failed: ok=%v err=%v", ok, err)
	}
	if _, ok, err := FindResource(m, "bbbbbbb"); !ok || err != nil {
		t.Fatalf("unambiguous prefix failed: ok=%v err=%v", ok, err)
	}
	if _, _, err := FindResource(m, "aaaaaaa"); err == nil {
		t.Fatal("ambiguous prefix must error")
	}
	if _, ok, err := FindResource(m, "zzzzzzz"); ok || err != nil {
		t.Fatalf("no match should be (false, nil): ok=%v err=%v", ok, err)
	}
}

// TestSetResourceTrustLeavesEverythingElseByteIdentical covers §12.6's
// trust-update wire.
func TestSetResourceTrustLeavesEverythingElseByteIdentical(t *testing.T) {
	s := newResourceTestStore(t)
	args := []ResourceArg{
		{"contract", "dolt-diff-summary-v1"},
		{"db_path", "data/dolt-db"},
		{"from", "main"},
		{"table", "users"},
		{"to", "HEAD"},
	}
	m := ResourcesManifest{Version: ResourcesManifestVersion, Feature: "model-picker", Resources: []Resource{{
		ResourceID: "res_4b62313b6cce", Kind: ResourceKindAdapterSnapshot,
		Selector: "dolt:diff-summary:users", Adapter: "dolt", Capability: "diff-summary",
		Args: args, Trust: &ResourceTrust{BinarySHA256: strings.Repeat("3", 64)},
		AddedByToolVersion: "tpatch/0.13.0",
	}}}
	if err := SaveResources(s, m); err != nil {
		t.Fatalf("SaveResources: %v", err)
	}
	before, _ := os.ReadFile(s.ResourcesPath("model-picker"))
	if !SetResourceTrust(&m, "res_4b62313b6cce", strings.Repeat("6", 64)) {
		t.Fatal("SetResourceTrust reported no match")
	}
	if err := SaveResources(s, m); err != nil {
		t.Fatalf("SaveResources: %v", err)
	}
	after, _ := os.ReadFile(s.ResourcesPath("model-picker"))
	diffLines := 0
	beforeLines := strings.Split(string(before), "\n")
	afterLines := strings.Split(string(after), "\n")
	if len(beforeLines) != len(afterLines) {
		t.Fatal("the trust update must not change the file's line structure")
	}
	for i := range beforeLines {
		if beforeLines[i] != afterLines[i] {
			diffLines++
			if !strings.Contains(afterLines[i], "binary_sha256") {
				t.Fatalf("unexpected line changed: %q", afterLines[i])
			}
		}
	}
	if diffLines != 1 {
		t.Fatalf("exactly one line should change, got %d", diffLines)
	}
	// Neither current.json nor any batch file exists, and none is
	// created by a trust update.
	if _, err := os.Stat(s.ResourceCurrentPath("model-picker")); !os.IsNotExist(err) {
		t.Fatal("trust-dolt must never create current.json")
	}
}

// TestSweepTrackedTempArtifacts covers the tracked half of the
// lock-gated orphan sweep.
func TestSweepTrackedTempArtifacts(t *testing.T) {
	s := newResourceTestStore(t)
	if err := s.EnsureResourceCaptureTree("model-picker"); err != nil {
		t.Fatalf("EnsureResourceCaptureTree: %v", err)
	}
	orphanBatch := filepath.Join(s.ResourceBatchesDir("model-picker"), "rb_abc.tmp-0123456789ab.json")
	if err := os.WriteFile(orphanBatch, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(s.ResourceCurrentTempPath("model-picker"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if diags := s.SweepTrackedTempArtifacts("model-picker"); len(diags) != 0 {
		t.Fatalf("unexpected sweep diagnostics: %v", diags)
	}
	if _, err := os.Stat(orphanBatch); !os.IsNotExist(err) {
		t.Fatal("orphan batch temp survived the sweep")
	}
	if _, err := os.Stat(s.ResourceCurrentTempPath("model-picker")); !os.IsNotExist(err) {
		t.Fatal("orphan pointer temp survived the sweep")
	}
}

// TestTrackedArtifactPermissions covers §7.4: tracked artifacts use
// ordinary repository file permissions.
func TestTrackedArtifactPermissions(t *testing.T) {
	s := newResourceTestStore(t)
	b := goldenBatch()
	_, canonical, _ := ComputeBatchID(b.Feature, b.Results)
	if _, err := s.PublishBatch("model-picker", b, canonical); err != nil {
		t.Fatalf("PublishBatch: %v", err)
	}
	for _, p := range []string{
		s.ResourceBatchPath("model-picker", b.BatchID),
		s.ResourceCurrentPath("model-picker"),
	} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s: %v", p, err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Fatalf("%s has mode %v, want 0644", p, info.Mode().Perm())
		}
	}
}

// TestMkdirAllAndSyncChainIsRetrySafe covers §7.1 step 4's
// unconditional whole-chain retry-fsync: a second call re-fsyncs
// already-existing directories rather than assuming visibility implies
// durability.
func TestMkdirAllAndSyncChainIsRetrySafe(t *testing.T) {
	root := t.TempDir()
	leaf := filepath.Join(root, "a", "b", "c")
	if err := MkdirAllAndSyncChain(leaf, root, 0o700); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := MkdirAllAndSyncChain(leaf, root, 0o700); err != nil {
		t.Fatalf("retry: %v", err)
	}
	chain := DirChain(leaf, root)
	if len(chain) != 4 {
		t.Fatalf("chain = %v, want 4 entries", chain)
	}
	if chain[0] != leaf || chain[len(chain)-1] != root {
		t.Fatalf("chain must run leaf-first to stopAt: %v", chain)
	}
}

// TestNearestExistingAncestor covers §7.1 step 2's statfs target
// selection on a fresh clone where the leaf does not yet exist.
func TestNearestExistingAncestor(t *testing.T) {
	root := t.TempDir()
	got, err := NearestExistingAncestor(filepath.Join(root, "does", "not", "exist"))
	if err != nil {
		t.Fatalf("NearestExistingAncestor: %v", err)
	}
	if got != root {
		t.Fatalf("nearest existing ancestor = %s, want %s", got, root)
	}
}
