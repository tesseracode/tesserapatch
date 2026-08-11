// Canonical JSON value model for the resource-capture wire domain
// (PRD-feature-resource-claims-and-capture-adapters §12, ADR-033 D11).
//
// Three serializations exist in that design and only two of them are
// implemented here:
//
//   - Canonical `args` JSON (§13.1) — CanonicalArgsJSON below.
//   - Canonical batch JSON (§12 intro) — CanonicalBatchJSON, the hash
//     input for the content-addressed batch_id.
//
// The third, the file wire format, is ordinary encoding/json over
// fixed-field structs. Result bodies are kind-tagged variants with
// different field sets per kind, so they are modelled as an ordered
// CanonNode tree rather than a Go map: a map would make both the file
// wire and the hash input depend on encoding/json's map-key ordering,
// which ADR-033 D11 forbids outright ("No Go map type appears in any
// tracked wire schema").

package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
)

// CanonKind tags a CanonNode's JSON type. The set is closed: only the
// types CanonicalBatchJSON is defined over are representable.
type CanonKind int

// Canonical node kinds.
const (
	CanonKindNull CanonKind = iota
	CanonKindString
	CanonKindBool
	CanonKindUint
	CanonKindArray
	CanonKindObject
)

// CanonField is one key/value pair of a fixed-field canonical object.
// Field order is the declared order, never sorted at encode time —
// callers that need a sorted array (args, files, tables) sort before
// constructing the node.
type CanonField struct {
	Key   string
	Value CanonNode
}

// CanonNode is a JSON value restricted to the types ADR-033 D11's
// canonical encoder supports: strings, booleans, null, non-negative
// integers, arrays, and fixed-field objects.
type CanonNode struct {
	Kind   CanonKind
	Str    string
	Bool   bool
	Uint   uint64
	Array  []CanonNode
	Object []CanonField
}

// CanonNull returns the JSON null node.
func CanonNull() CanonNode { return CanonNode{Kind: CanonKindNull} }

// CanonString returns a JSON string node.
func CanonString(s string) CanonNode { return CanonNode{Kind: CanonKindString, Str: s} }

// CanonBool returns a JSON boolean node.
func CanonBool(b bool) CanonNode { return CanonNode{Kind: CanonKindBool, Bool: b} }

// CanonUint returns a JSON non-negative integer node.
func CanonUint(u uint64) CanonNode { return CanonNode{Kind: CanonKindUint, Uint: u} }

// CanonArray returns a JSON array node. A nil/empty slice still
// encodes as `[]`, never `null` (ADR-033 D11).
func CanonArray(items ...CanonNode) CanonNode {
	if items == nil {
		items = []CanonNode{}
	}
	return CanonNode{Kind: CanonKindArray, Array: items}
}

// CanonArrayOf returns a JSON array node from an existing slice.
func CanonArrayOf(items []CanonNode) CanonNode {
	if items == nil {
		items = []CanonNode{}
	}
	return CanonNode{Kind: CanonKindArray, Array: items}
}

// CanonObject returns a fixed-field JSON object node in declared field
// order.
func CanonObject(fields ...CanonField) CanonNode {
	if fields == nil {
		fields = []CanonField{}
	}
	return CanonNode{Kind: CanonKindObject, Object: fields}
}

// CanonFieldOf is a terse CanonField constructor.
func CanonFieldOf(key string, value CanonNode) CanonField {
	return CanonField{Key: key, Value: value}
}

// IsNull reports whether the node is the JSON null value.
func (n CanonNode) IsNull() bool { return n.Kind == CanonKindNull }

// Field returns the value of a named object field and whether it was
// present. Non-object nodes always report false.
func (n CanonNode) Field(key string) (CanonNode, bool) {
	if n.Kind != CanonKindObject {
		return CanonNode{}, false
	}
	for _, f := range n.Object {
		if f.Key == key {
			return f.Value, true
		}
	}
	return CanonNode{}, false
}

// MarshalJSON renders the node as compact JSON with fields in declared
// order. encoding/json re-indents this output when the enclosing
// struct is marshalled with MarshalIndent, so the file wire format
// stays ordinary indented JSON while field order stays ours.
func (n CanonNode) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	if err := n.encodeStandard(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// encodeStandard writes the node using encoding/json's own string
// escaping (including HTML escaping), which is what the file wire
// format uses.
func (n CanonNode) encodeStandard(buf *bytes.Buffer) error {
	switch n.Kind {
	case CanonKindNull:
		buf.WriteString("null")
	case CanonKindString:
		data, err := json.Marshal(n.Str)
		if err != nil {
			return err
		}
		buf.Write(data)
	case CanonKindBool:
		if n.Bool {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case CanonKindUint:
		buf.WriteString(strconv.FormatUint(n.Uint, 10))
	case CanonKindArray:
		buf.WriteByte('[')
		for i, item := range n.Array {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := item.encodeStandard(buf); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case CanonKindObject:
		buf.WriteByte('{')
		for i, f := range n.Object {
			if i > 0 {
				buf.WriteByte(',')
			}
			key, err := json.Marshal(f.Key)
			if err != nil {
				return err
			}
			buf.Write(key)
			buf.WriteByte(':')
			if err := f.Value.encodeStandard(buf); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		return fmt.Errorf("canonical json: unknown node kind %d", n.Kind)
	}
	return nil
}

// encodeCanonical writes the node using ADR-033 D11's minimal string
// escaping (only \ and ") rather than encoding/json's HTML-escaping
// defaults. This is the batch_id hash input encoding.
func (n CanonNode) encodeCanonical(buf *bytes.Buffer) error {
	switch n.Kind {
	case CanonKindNull:
		buf.WriteString("null")
	case CanonKindString:
		writeCanonicalString(buf, n.Str)
	case CanonKindBool:
		if n.Bool {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case CanonKindUint:
		buf.WriteString(strconv.FormatUint(n.Uint, 10))
	case CanonKindArray:
		buf.WriteByte('[')
		for i, item := range n.Array {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := item.encodeCanonical(buf); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case CanonKindObject:
		buf.WriteByte('{')
		for i, f := range n.Object {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeCanonicalString(buf, f.Key)
			buf.WriteByte(':')
			if err := f.Value.encodeCanonical(buf); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		return fmt.Errorf("canonical json: unknown node kind %d", n.Kind)
	}
	return nil
}

// writeCanonicalString applies §13.1's escaping rule: only a backslash
// and a double quote are escaped. encoding/json.Marshal is deliberately
// not used here because it also HTML-escapes < > &.
func writeCanonicalString(buf *bytes.Buffer, s string) {
	buf.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			buf.WriteString(`\\`)
		case '"':
			buf.WriteString(`\"`)
		default:
			buf.WriteByte(s[i])
		}
	}
	buf.WriteByte('"')
}

// UnmarshalJSON decodes JSON into an order-preserving CanonNode tree.
//
// Order preservation is load-bearing: §7.3 step 3 re-canonicalizes an
// already-published batch file's own body and compares it against a
// freshly-staged candidate's hash input. Decoding through
// map[string]any would lose field order and make that comparison
// depend on Go's map iteration, so the decode walks json.Decoder
// tokens directly.
//
// Duplicate object keys are rejected: a tracked artifact with two
// entries for one key has no single well-defined canonical form.
func (n *CanonNode) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	node, err := decodeCanonValue(dec)
	if err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("canonical json: trailing content after value")
	}
	*n = node
	return nil
}

func decodeCanonValue(dec *json.Decoder) (CanonNode, error) {
	tok, err := dec.Token()
	if err != nil {
		return CanonNode{}, err
	}
	return decodeCanonFromToken(dec, tok)
}

func decodeCanonFromToken(dec *json.Decoder, tok json.Token) (CanonNode, error) {
	switch v := tok.(type) {
	case nil:
		return CanonNull(), nil
	case string:
		return CanonString(v), nil
	case bool:
		return CanonBool(v), nil
	case json.Number:
		u, err := strconv.ParseUint(v.String(), 10, 64)
		if err != nil {
			return CanonNode{}, fmt.Errorf("canonical json: %q is not a non-negative integer", v.String())
		}
		return CanonUint(u), nil
	case json.Delim:
		switch v {
		case '[':
			items := []CanonNode{}
			for dec.More() {
				item, err := decodeCanonValue(dec)
				if err != nil {
					return CanonNode{}, err
				}
				items = append(items, item)
			}
			if _, err := dec.Token(); err != nil {
				return CanonNode{}, err
			}
			return CanonArrayOf(items), nil
		case '{':
			fields := []CanonField{}
			seen := map[string]struct{}{}
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return CanonNode{}, err
				}
				key, ok := keyTok.(string)
				if !ok {
					return CanonNode{}, fmt.Errorf("canonical json: non-string object key")
				}
				if _, dup := seen[key]; dup {
					return CanonNode{}, fmt.Errorf("canonical json: duplicate object key %q", key)
				}
				seen[key] = struct{}{}
				val, err := decodeCanonValue(dec)
				if err != nil {
					return CanonNode{}, err
				}
				fields = append(fields, CanonField{Key: key, Value: val})
			}
			if _, err := dec.Token(); err != nil {
				return CanonNode{}, err
			}
			return CanonObject(fields...), nil
		}
	}
	return CanonNode{}, fmt.Errorf("canonical json: unsupported token %v", tok)
}

// CanonicalArgsJSON encodes a resource's declared args exactly as
// §13.1 specifies: keys sorted byte-ascending, no whitespace, only \
// and " escaped, and `{}` for the empty set.
//
// The input is not required to be pre-sorted; the encoder sorts a copy
// so callers can pass declaration order.
func CanonicalArgsJSON(args []ResourceArg) string {
	sorted := make([]ResourceArg, len(args))
	copy(sorted, args)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })

	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, a := range sorted {
		if i > 0 {
			buf.WriteByte(',')
		}
		writeCanonicalString(&buf, a.Key)
		buf.WriteByte(':')
		writeCanonicalString(&buf, a.Value)
	}
	buf.WriteByte('}')
	return buf.String()
}

// HasControlBytes reports whether s contains a NUL or any other C0
// control byte (0x00-0x1F) or DEL (0x7F). §13.1 rule 6 makes any such
// byte a validation error in every identity-bearing field.
func HasControlBytes(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c <= 0x1F || c == 0x7F {
			return true
		}
	}
	return false
}

// CanonicalBatchJSON returns the exact hash-input bytes for a batch's
// content-addressed batch_id: `{"feature": <feature>, "results":
// <results sorted by resource_id>}` with the batch_id field itself
// deliberately absent (no self-reference).
//
// Results are sorted by resource_id byte-ascending on a copy, so the
// caller's slice ordering never affects the digest.
func CanonicalBatchJSON(feature string, results []BatchResult) ([]byte, error) {
	sorted := make([]BatchResult, len(results))
	copy(sorted, results)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ResourceID < sorted[j].ResourceID })

	items := make([]CanonNode, 0, len(sorted))
	for _, r := range sorted {
		items = append(items, r.canonNode())
	}
	body := CanonObject(
		CanonFieldOf("feature", CanonString(feature)),
		CanonFieldOf("results", CanonArrayOf(items)),
	)
	var buf bytes.Buffer
	if err := body.encodeCanonical(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// canonicalizeDecodedBatchBody re-encodes an already-decoded batch
// file's own {feature, results} body through the identical encoder
// CanonicalBatchJSON uses, dropping batch_id exactly as the hash input
// does. §7.3 step 3 compares this against a freshly-staged candidate's
// hash input to tell presentation drift apart from a real collision.
func canonicalizeDecodedBatchBody(b Batch) ([]byte, error) {
	return CanonicalBatchJSON(b.Feature, b.Results)
}
