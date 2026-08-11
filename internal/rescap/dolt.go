// Dolt adapter for
// PRD-feature-resource-claims-and-capture-adapters §6 / ADR-033 D5.
//
// Dolt is never an authority over tpatch state and is not a build or
// runtime dependency: everything here is reachable only for a resource
// an operator explicitly declared with `--trust-current-dolt`.
//
// The trust model splits identity from trust. A resource's *identity*
// is kind/adapter/db_path/table/from/to/contract — the semantic
// contract being captured — while the pinned binary digest is mutable
// operational metadata in a separate `trust` field, re-pinnable via
// `trust-dolt` without perturbing resource_id, current.json, or any
// batch history.

package rescap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// DoltDeclaredKeys is the exact, complete set of declared args a Dolt
// resource carries. Every key is required; any other key, a missing
// key, or a duplicate --arg is a validation error at add time.
var DoltDeclaredKeys = []string{"contract", "db_path", "from", "table", "to"}

// hex64 matches a 64-lowercase-hex trust pin.
var hex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// IsValidBinarySHA256 reports whether v is exactly 64 lowercase hex
// characters.
func IsValidBinarySHA256(v string) bool { return hex64.MatchString(v) }

// ValidateDoltArgs applies §6.2's declared-field rules. contract is
// checked first, before db_path/table/from/to are even inspected: a
// future contract value would name a different query/schema shape
// entirely, and this design refuses to silently guess at one.
func ValidateDoltArgs(args []store.ResourceArg, selectorTable string) error {
	seen := map[string]string{}
	for _, a := range args {
		if _, dup := seen[a.Key]; dup {
			return Invalid(ReasonInvalidDeclaration, "duplicate --arg key %q", a.Key)
		}
		seen[a.Key] = a.Value
	}
	contract, ok := seen["contract"]
	if !ok {
		return Invalid(ReasonInvalidDeclaration, "missing required --arg contract")
	}
	if contract != store.DoltContractDiffSummary1 {
		return Invalid(ReasonDoltContractUnsupported,
			"contract %q is not supported; v1 accepts only %q", contract, store.DoltContractDiffSummary1)
	}
	for _, key := range DoltDeclaredKeys {
		if _, ok := seen[key]; !ok {
			return Invalid(ReasonInvalidDeclaration, "missing required --arg %s", key)
		}
	}
	for key := range seen {
		if !isDeclaredDoltKey(key) {
			return Invalid(ReasonInvalidDeclaration, "unknown --arg key %q for the dolt adapter", key)
		}
	}
	for _, key := range []string{"from", "to", "table"} {
		if err := validateDoltValue(key, seen[key]); err != nil {
			return err
		}
	}
	if seen["table"] != selectorTable {
		return Invalid(ReasonInvalidDeclaration,
			"selector table %q does not match the declared table %q", selectorTable, seen["table"])
	}
	return nil
}

func isDeclaredDoltKey(key string) bool {
	for _, k := range DoltDeclaredKeys {
		if k == key {
			return true
		}
	}
	return false
}

// validateDoltValue applies §6.2's ordered validation rules. All five
// named cases share the single `dolt-argument-refused` reason so a
// caller does not need to distinguish which specific value shape was
// rejected from the exit code alone.
func validateDoltValue(key, value string) error {
	if value == "" {
		return Invalid(ReasonInvalidDeclaration, "--arg %s must not be empty", key)
	}
	if store.HasControlBytes(value) {
		return Invalid(ReasonDoltArgumentRefused,
			"--arg %s contains a NUL or C0 control byte", key)
	}
	if strings.Contains(value, `\`) {
		// Whether a backslash is itself an escape character inside a
		// Dolt/MySQL string literal depends on the session's sql_mode
		// (NO_BACKSLASH_ESCAPES), which this design neither controls
		// nor verifies. Refusing is simpler and strictly safer.
		return Invalid(ReasonDoltArgumentRefused,
			"--arg %s contains a literal backslash", key)
	}
	if strings.Contains(value, "..") {
		// dolt_diff_summary's own argument-count validation inspects
		// the first argument's literal SQL-expression string for a ".."
		// substring to choose between its dot-range and explicit parse
		// branches, so a legitimate ".." would misroute this design's
		// fixed 3-argument invocation.
		return Invalid(ReasonDoltArgumentRefused,
			"--arg %s contains the two-character substring \"..\"", key)
	}
	if key != "table" {
		upper := strings.ToUpper(value)
		if upper == "WORKING" || upper == "STAGED" {
			// The working tree/staged index, unlike any committed ref,
			// is gated by Dolt's own dolt_ignore table, which would
			// reintroduce a silent-omission path this design closed by
			// making `table` mandatory. v1 is committed-refs-only.
			return Invalid(ReasonDoltArgumentRefused,
				"--arg %s names %s; v1 accepts committed refs only", key, value)
		}
	}
	return nil
}

// escapeSQLLiteral applies the only transform an otherwise-valid value
// receives: doubling a single quote, the one escaping rule that is
// unambiguous under both interpretations of sql_mode.
func escapeSQLLiteral(v string) string { return strings.ReplaceAll(v, "'", "''") }

// BuildDiffSummarySQL renders the single, exact query shape this design
// ever emits. There is no whole-database invocation, no
// --schema/--data/--name-only flag combination, and no dot-range form.
func BuildDiffSummarySQL(from, to, table string) string {
	return fmt.Sprintf(
		"SELECT from_table_name, to_table_name, diff_type, data_change, schema_change "+
			"FROM dolt_diff_summary('%s', '%s', '%s') "+
			"ORDER BY from_table_name, to_table_name;",
		escapeSQLLiteral(from), escapeSQLLiteral(to), escapeSQLLiteral(table))
}

// DiffSummaryArgv renders the complete argv for one invocation.
func DiffSummaryArgv(execPath, sql string) []string {
	return []string{execPath, "sql", "-r", "json", "-q", sql}
}

// ─── trust bootstrap and private-copy execution ──────────────────────────────

// HashExecutableDescriptor implements §6.1's add-time trust bootstrap
// (TOFU): open the resolved path and hash the opened descriptor
// directly. No private copy is ever created, no Dolt process is ever
// started, and no es_<id>/ scratch directory or file is created.
func HashExecutableDescriptor(resolvedPath string) (string, error) {
	f, err := os.Open(resolvedPath)
	if err != nil {
		return "", Internal(ReasonAdapterCopyFailed, "opening %s: %v", resolvedPath, err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", Internal(ReasonAdapterCopyFailed, "hashing %s: %v", resolvedPath, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// PrivateCopy is a hash-verified, owner-only executable copy.
type PrivateCopy struct {
	Path   string
	Digest string
}

// Remove deletes the private copy, best-effort.
func (c *PrivateCopy) Remove() {
	if c == nil || c.Path == "" {
		return
	}
	_ = os.Remove(c.Path)
}

// MakeVerifiedPrivateCopy implements §6.1's capture-time sequence
// steps 2-7:
//
//  2. open the resolved path (this is the descriptor every subsequent
//     step operates on; no step re-resolves the pathname after it),
//  3. preflight the scratch filesystem for a noexec mount,
//  4. stream-copy while hashing in a single io.TeeReader pass, then
//     Sync — so the digest computed is provably the digest of the exact
//     bytes that land in the copy,
//  5. verify the digest against the pin *before* finalizing; a mismatch
//     deletes the copy and starts no process at all,
//  6. harden via Fchmod on the still-open descriptor (never a
//     path-based os.Chmod, which re-resolves the pathname and could
//     race a swap), confirm via f.Stat(), then close,
//  7. optionally re-verify immediately before exec.
//
// No unverified bytes are ever made executable: the file is 0600 — not
// executable at all — for the entire duration of the copy and the digest
// comparison, and only becomes 0500 after the digest already matched.
func MakeVerifiedPrivateCopy(resolvedPath, scratchDir, pinnedDigest string) (*PrivateCopy, error) {
	if pinnedDigest == "" {
		return nil, Refuse(ReasonDoltTrustRequired,
			"this resource has no trust.binary_sha256 pin; run `tpatch feature resource trust-dolt` first")
	}
	source, err := os.Open(resolvedPath)
	if err != nil {
		return nil, Internal(ReasonAdapterCopyFailed, "opening %s: %v", resolvedPath, err)
	}
	defer func() { _ = source.Close() }()

	// Step 3: preflight the scratch filesystem for a noexec mount
	// BEFORE the private copy file is created at all — creating an
	// executable-intent copy on a filesystem the OS already marked
	// non-executable can only fail later, and more confusingly, at
	// cmd.Start().
	if err := scratchExecCheck(scratchDir); err != nil {
		return nil, err
	}

	suffix, err := store.RandomHex12()
	if err != nil {
		return nil, Internal(ReasonAdapterCopyFailed, "generating a scratch suffix: %v", err)
	}
	copyPath := filepath.Join(scratchDir, "dolt-copy-"+suffix)
	rawDst, err := os.OpenFile(copyPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, Internal(ReasonAdapterCopyFailed, "creating the private copy: %v", err)
	}
	// The destination is reached through a narrow indirection so a test
	// can inject the exact host errnos this step must survive — ENOSPC
	// mid-write and EIO on Sync — without weakening the production
	// path, which uses the file itself unchanged.
	dst := wrapPrivateCopyTarget(rawDst)
	hasher := sha256.New()
	if _, err := io.Copy(dst, io.TeeReader(source, hasher)); err != nil {
		_ = dst.Close()
		_ = os.Remove(copyPath)
		return nil, Internal(ReasonAdapterCopyFailed, "copying %s: %v", resolvedPath, err)
	}
	if err := dst.Sync(); err != nil {
		_ = dst.Close()
		_ = os.Remove(copyPath)
		return nil, Internal(ReasonAdapterCopyFailed, "syncing the private copy: %v", err)
	}
	digest := hex.EncodeToString(hasher.Sum(nil))
	if digest != pinnedDigest {
		_ = dst.Close()
		_ = os.Remove(copyPath)
		return nil, Refuse(ReasonAdapterBinaryUntrusted,
			"the resolved binary's digest %s does not match the pinned %s; no invocation was attempted", digest, pinnedDigest)
	}
	if err := dst.Chmod(0o500); err != nil {
		_ = dst.Close()
		_ = os.Remove(copyPath)
		return nil, Internal(ReasonAdapterCopyFailed, "hardening the private copy: %v", err)
	}
	info, err := dst.Stat()
	if err != nil || info.Mode().Perm() != 0o500 {
		_ = dst.Close()
		_ = os.Remove(copyPath)
		return nil, Internal(ReasonAdapterCopyFailed, "the private copy's mode change could not be confirmed")
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(copyPath)
		return nil, Internal(ReasonAdapterCopyFailed, "closing the private copy: %v", err)
	}

	// Step 7: cheap additional closure of the window between the
	// Fchmod and cmd.Start().
	recheck, err := os.Open(copyPath)
	if err != nil {
		_ = os.Remove(copyPath)
		return nil, Internal(ReasonAdapterCopyFailed, "re-opening the private copy: %v", err)
	}
	reHasher := sha256.New()
	if _, err := io.Copy(reHasher, recheck); err != nil {
		_ = recheck.Close()
		_ = os.Remove(copyPath)
		return nil, Internal(ReasonAdapterCopyFailed, "re-hashing the private copy: %v", err)
	}
	_ = recheck.Close()
	if hex.EncodeToString(reHasher.Sum(nil)) != pinnedDigest {
		_ = os.Remove(copyPath)
		return nil, Refuse(ReasonAdapterBinaryUntrusted,
			"the private copy's digest changed before execution; no invocation was attempted")
	}
	return &PrivateCopy{Path: copyPath, Digest: digest}, nil
}

// ─── output parsing ──────────────────────────────────────────────────────────

// DoltDiffRow is one strictly-parsed dolt_diff_summary row.
type DoltDiffRow struct {
	FromTableName string
	ToTableName   string
	DiffType      string
	DataChange    bool
	SchemaChange  bool
}

// requiredDoltFields is the exact five-field shape every row must have.
var requiredDoltFields = []string{"from_table_name", "to_table_name", "diff_type", "data_change", "schema_change"}

// ParseDiffSummaryJSON implements §6.3.
//
// The captured buffer carries trailing whitespace beyond the JSON body
// itself — a real zero-row capture is "{}\n\n" and a real nonempty one
// is "...]}\n" — so leading and trailing ASCII whitespace is trimmed
// before either valid top-level shape is matched. This is grounded in
// the cited real output shape, not a purely defensive guess.
//
// Exactly two top-level shapes are valid. Anything else — a missing
// "rows" key, a "schema" key that does not exist in the real output,
// an extra unknown top-level key, or "rows" present but not an array —
// is a fatal parse error, never a best-effort partial parse. Values are
// never coerced: a 0/1 in a boolean position indicates a real
// parsing/version mismatch that must fail loudly.
func ParseDiffSummaryJSON(raw []byte) ([]DoltDiffRow, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "{}" {
		return []DoltDiffRow{}, nil
	}
	var top store.CanonNode
	if err := json.Unmarshal([]byte(trimmed), &top); err != nil {
		return nil, Refuse(ReasonDoltJSONParseError, "output is not valid JSON: %v", err)
	}
	if top.Kind != store.CanonKindObject {
		return nil, Refuse(ReasonDoltJSONParseError, "output is not a JSON object")
	}
	if len(top.Object) != 1 || top.Object[0].Key != "rows" {
		var keys []string
		for _, f := range top.Object {
			keys = append(keys, f.Key)
		}
		return nil, Refuse(ReasonDoltJSONParseError,
			"expected exactly one top-level key \"rows\", got [%s]", strings.Join(keys, " "))
	}
	rowsNode := top.Object[0].Value
	if rowsNode.Kind != store.CanonKindArray {
		return nil, Refuse(ReasonDoltJSONParseError, "\"rows\" is present but is not a JSON array")
	}
	rows := make([]DoltDiffRow, 0, len(rowsNode.Array))
	for i, item := range rowsNode.Array {
		if item.Kind != store.CanonKindObject {
			return nil, Refuse(ReasonDoltJSONParseError, "row %d is not a JSON object", i)
		}
		if len(item.Object) != len(requiredDoltFields) {
			return nil, Refuse(ReasonDoltJSONParseError,
				"row %d has %d fields, want exactly %d", i, len(item.Object), len(requiredDoltFields))
		}
		var row DoltDiffRow
		for _, name := range requiredDoltFields {
			val, ok := item.Field(name)
			if !ok {
				return nil, Refuse(ReasonDoltJSONParseError, "row %d is missing %q", i, name)
			}
			switch name {
			case "from_table_name", "to_table_name", "diff_type":
				if val.Kind != store.CanonKindString {
					return nil, Refuse(ReasonDoltJSONParseError, "row %d field %q is not a JSON string", i, name)
				}
			default:
				if val.Kind != store.CanonKindBool {
					return nil, Refuse(ReasonDoltJSONParseError,
						"row %d field %q is not a native JSON boolean", i, name)
				}
			}
			switch name {
			case "from_table_name":
				row.FromTableName = val.Str
			case "to_table_name":
				row.ToTableName = val.Str
			case "diff_type":
				row.DiffType = val.Str
			case "data_change":
				row.DataChange = val.Bool
			case "schema_change":
				row.SchemaChange = val.Bool
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// DiffSummaryResult renders parsed rows as the tagged result variant.
// diff_type is tracked verbatim rather than validated against the
// source-confirmed four-value enum, so a future Dolt version adding a
// fifth value does not break capture.
func DiffSummaryResult(rows []DoltDiffRow) store.CanonNode {
	items := make([]store.CanonNode, 0, len(rows))
	for _, r := range rows {
		items = append(items, store.CanonObject(
			store.CanonFieldOf("from_table_name", store.CanonString(r.FromTableName)),
			store.CanonFieldOf("to_table_name", store.CanonString(r.ToTableName)),
			store.CanonFieldOf("diff_type", store.CanonString(r.DiffType)),
			store.CanonFieldOf("data_change", store.CanonBool(r.DataChange)),
			store.CanonFieldOf("schema_change", store.CanonBool(r.SchemaChange)),
		))
	}
	return store.CanonObject(store.CanonFieldOf("tables", store.CanonArrayOf(items)))
}

// ParseDoltSelector splits a `dolt:<capability>:<table>` selector.
func ParseDoltSelector(selector string) (capability, table string, err error) {
	parts := strings.SplitN(selector, ":", 3)
	if len(parts) != 3 || parts[0] != store.ResourceAdapterDolt {
		return "", "", Invalid(ReasonInvalidDeclaration,
			"selector %q must have the shape dolt:<capability>:<table>", selector)
	}
	if parts[1] != store.ResourceCapabilityDiff {
		return "", "", Invalid(ReasonInvalidDeclaration,
			"capability %q is not supported; v1 accepts only %q", parts[1], store.ResourceCapabilityDiff)
	}
	if parts[2] == "" {
		return "", "", Invalid(ReasonInvalidDeclaration, "selector %q names an empty table", selector)
	}
	return parts[1], parts[2], nil
}

// privateCopyTarget is the exact subset of *os.File the private-copy
// sequence uses. Naming it lets a test substitute a wrapper that fails
// at one precise step with one precise errno; production passes the
// *os.File straight through.
type privateCopyTarget interface {
	io.Writer
	Sync() error
	Chmod(mode os.FileMode) error
	Stat() (os.FileInfo, error)
	Close() error
}

// privateCopyTargetWrapper is nil in production. When a test installs
// one, it wraps the real destination file, so the bytes still land on
// disk and the partial-copy cleanup path is genuinely exercised.
var privateCopyTargetWrapper func(privateCopyTarget) privateCopyTarget

func wrapPrivateCopyTarget(f *os.File) privateCopyTarget {
	if w := privateCopyTargetWrapper; w != nil {
		return w(f)
	}
	return f
}

// SetPrivateCopyTargetWrapperForTest installs a wrapper around the
// private copy's destination file and returns a restore func. Tests use
// it to inject exact host errnos (syscall.ENOSPC, syscall.EIO) at the
// write and Sync steps.
func SetPrivateCopyTargetWrapperForTest(fn func(privateCopyTarget) privateCopyTarget) func() {
	prev := privateCopyTargetWrapper
	privateCopyTargetWrapper = fn
	return func() { privateCopyTargetWrapper = prev }
}
