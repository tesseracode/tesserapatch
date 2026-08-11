// Dolt adapter tests (PRD §6, ADR-033 D5/D6).
//
// The suite never depends on an installed Dolt binary: the executable
// resolver is substituted with a controlled fixture, and every output
// shape is a fixture buffer rather than a live query.

package rescap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/store"
)

func doltArgs(overrides map[string]string) []store.ResourceArg {
	base := map[string]string{
		"contract": "dolt-diff-summary-v1",
		"db_path":  "data/dolt-db",
		"from":     "main",
		"table":    "users",
		"to":       "HEAD",
	}
	for k, v := range overrides {
		if v == "\x00DELETE" {
			delete(base, k)
			continue
		}
		base[k] = v
	}
	out := make([]store.ResourceArg, 0, len(base))
	for _, k := range []string{"contract", "db_path", "from", "table", "to"} {
		if v, ok := base[k]; ok {
			out = append(out, store.ResourceArg{Key: k, Value: v})
		}
	}
	for k, v := range base {
		known := false
		for _, d := range DoltDeclaredKeys {
			if d == k {
				known = true
			}
		}
		if !known {
			out = append(out, store.ResourceArg{Key: k, Value: v})
		}
	}
	return out
}

// TestBuildDiffSummarySQLShape pins the single exact query shape this
// design ever emits, plus the complete argv.
func TestBuildDiffSummarySQLShape(t *testing.T) {
	sql := BuildDiffSummarySQL("main", "HEAD", "users")
	want := "SELECT from_table_name, to_table_name, diff_type, data_change, schema_change " +
		"FROM dolt_diff_summary('main', 'HEAD', 'users') " +
		"ORDER BY from_table_name, to_table_name;"
	if sql != want {
		t.Fatalf("sql = %q\nwant %q", sql, want)
	}
	argv := DiffSummaryArgv("/opt/dolt/bin/dolt", sql)
	if len(argv) != 6 || argv[1] != "sql" || argv[2] != "-r" || argv[3] != "json" || argv[4] != "-q" {
		t.Fatalf("argv = %v", argv)
	}
	for _, forbidden := range []string{"--schema", "--data", "--name-only", ".."} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("sql must never contain %q", forbidden)
		}
	}
}

// TestSingleQuoteEscaping proves the one transform applied to an
// otherwise-valid value is doubling a single quote.
func TestSingleQuoteEscaping(t *testing.T) {
	sql := BuildDiffSummarySQL("bran'ch", "HEAD", "ta'ble")
	if !strings.Contains(sql, "'bran''ch'") || !strings.Contains(sql, "'ta''ble'") {
		t.Fatalf("single quotes not doubled: %s", sql)
	}
}

// TestValidateDoltArgs covers the whole add-time declared-field
// contract: contract first, required keys, unknown/duplicate keys,
// control bytes, backslashes, "..", WORKING/STAGED, and the
// selector/table match.
func TestValidateDoltArgs(t *testing.T) {
	cases := []struct {
		name       string
		args       []store.ResourceArg
		table      string
		wantReason string
		wantCode   int
	}{
		{name: "valid", args: doltArgs(nil), table: "users", wantReason: "", wantCode: 0},
		{name: "missing-contract", args: doltArgs(map[string]string{"contract": "\x00DELETE"}), table: "users", wantReason: ReasonInvalidDeclaration, wantCode: ExitValidation},
		{name: "unsupported-contract", args: doltArgs(map[string]string{"contract": "dolt-diff-summary-v2"}), table: "users", wantReason: ReasonDoltContractUnsupported, wantCode: ExitValidation},
		{name: "missing-db-path", args: doltArgs(map[string]string{"db_path": "\x00DELETE"}), table: "users", wantReason: ReasonInvalidDeclaration, wantCode: ExitValidation},
		{name: "missing-table", args: doltArgs(map[string]string{"table": "\x00DELETE"}), table: "users", wantReason: ReasonInvalidDeclaration, wantCode: ExitValidation},
		{name: "missing-from", args: doltArgs(map[string]string{"from": "\x00DELETE"}), table: "users", wantReason: ReasonInvalidDeclaration, wantCode: ExitValidation},
		{name: "missing-to", args: doltArgs(map[string]string{"to": "\x00DELETE"}), table: "users", wantReason: ReasonInvalidDeclaration, wantCode: ExitValidation},
		{name: "unknown-key", args: append(doltArgs(nil), store.ResourceArg{Key: "extra", Value: "x"}), table: "users", wantReason: ReasonInvalidDeclaration, wantCode: ExitValidation},
		{name: "duplicate-key", args: append(doltArgs(nil), store.ResourceArg{Key: "table", Value: "users"}), table: "users", wantReason: ReasonInvalidDeclaration, wantCode: ExitValidation},
		{name: "control-byte", args: doltArgs(map[string]string{"from": "ma\x01in"}), table: "users", wantReason: ReasonDoltArgumentRefused, wantCode: ExitValidation},
		{name: "nul-byte", args: doltArgs(map[string]string{"table": "us\x00ers"}), table: "us\x00ers", wantReason: ReasonDoltArgumentRefused, wantCode: ExitValidation},
		{name: "backslash", args: doltArgs(map[string]string{"to": `HEAD\x`}), table: "users", wantReason: ReasonDoltArgumentRefused, wantCode: ExitValidation},
		{name: "dot-range", args: doltArgs(map[string]string{"from": "main..HEAD"}), table: "users", wantReason: ReasonDoltArgumentRefused, wantCode: ExitValidation},
		{name: "working-upper", args: doltArgs(map[string]string{"from": "WORKING"}), table: "users", wantReason: ReasonDoltArgumentRefused, wantCode: ExitValidation},
		{name: "working-mixed", args: doltArgs(map[string]string{"from": "Working"}), table: "users", wantReason: ReasonDoltArgumentRefused, wantCode: ExitValidation},
		{name: "staged-lower", args: doltArgs(map[string]string{"to": "staged"}), table: "users", wantReason: ReasonDoltArgumentRefused, wantCode: ExitValidation},
		{name: "selector-table-mismatch", args: doltArgs(nil), table: "orders", wantReason: ReasonInvalidDeclaration, wantCode: ExitValidation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateDoltArgs(tc.args, tc.table)
			if tc.wantReason == "" {
				if err != nil {
					t.Fatalf("unexpected refusal: %v", err)
				}
				return
			}
			r := AsRefusal(err)
			if r == nil {
				t.Fatalf("want %s, got %v", tc.wantReason, err)
			}
			if r.Reason != tc.wantReason {
				t.Fatalf("reason = %s, want %s", r.Reason, tc.wantReason)
			}
			if r.Code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d", r.Code, tc.wantCode)
			}
		})
	}
}

// TestParseDiffSummaryJSON covers §6.3 exhaustively: whitespace
// trimming against the real cited output shapes, the two valid
// top-level shapes, strict five-field checking, and the refusal to
// coerce.
func TestParseDiffSummaryJSON(t *testing.T) {
	row := `{"from_table_name":"users","to_table_name":"users","diff_type":"modified","data_change":true,"schema_change":false}`
	cases := []struct {
		name     string
		raw      string
		wantRows int
		wantErr  bool
	}{
		{name: "zero-rows-bare", raw: `{}`, wantRows: 0, wantErr: false},
		{name: "zero-rows-real-shape", raw: "{}\n\n", wantRows: 0, wantErr: false},
		{name: "nonempty-real-shape", raw: `{"rows":[` + row + "]}\n", wantRows: 1, wantErr: false},
		{name: "nonempty-no-trailing-whitespace", raw: `{"rows":[` + row + `]}`, wantRows: 1, wantErr: false},
		{name: "leading-whitespace", raw: "  \t{\"rows\":[" + row + "]}\n", wantRows: 1, wantErr: false},
		{name: "empty-rows-array", raw: `{"rows":[]}`, wantRows: 0, wantErr: false},
		{name: "schema-key-refused", raw: `{"schema":[],"rows":[]}`, wantRows: 0, wantErr: true},
		{name: "extra-top-level-key", raw: `{"rows":[],"extra":1}`, wantRows: 0, wantErr: true},
		{name: "rows-not-array", raw: `{"rows":{}}`, wantRows: 0, wantErr: true},
		{name: "missing-rows-key", raw: `{"data":[]}`, wantRows: 0, wantErr: true},
		{name: "not-json", raw: `not json`, wantRows: 0, wantErr: true},
		{name: "not-an-object", raw: `[]`, wantRows: 0, wantErr: true},
		{name: "row-missing-field", raw: `{"rows":[{"from_table_name":"u","to_table_name":"u","diff_type":"modified","data_change":true}]}`, wantRows: 0, wantErr: true},
		{name: "row-extra-field", raw: `{"rows":[{"from_table_name":"u","to_table_name":"u","diff_type":"modified","data_change":true,"schema_change":false,"x":1}]}`, wantRows: 0, wantErr: true},
		{name: "row-duplicate-key", raw: `{"rows":[{"from_table_name":"u","from_table_name":"u","to_table_name":"u","diff_type":"m","data_change":true,"schema_change":false}]}`, wantRows: 0, wantErr: true},
		{name: "boolean-as-int", raw: `{"rows":[{"from_table_name":"u","to_table_name":"u","diff_type":"modified","data_change":1,"schema_change":0}]}`, wantRows: 0, wantErr: true},
		{name: "boolean-as-string", raw: `{"rows":[{"from_table_name":"u","to_table_name":"u","diff_type":"modified","data_change":"true","schema_change":"false"}]}`, wantRows: 0, wantErr: true},
		{name: "name-as-number", raw: `{"rows":[{"from_table_name":1,"to_table_name":"u","diff_type":"modified","data_change":true,"schema_change":false}]}`, wantRows: 0, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := ParseDiffSummaryJSON([]byte(tc.raw))
			if tc.wantErr {
				r := AsRefusal(err)
				if r == nil || r.Reason != ReasonDoltJSONParseError || r.Code != ExitRefusal {
					t.Fatalf("want dolt-json-parse-error exit 3, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(rows) != tc.wantRows {
				t.Fatalf("rows = %d, want %d", len(rows), tc.wantRows)
			}
		})
	}
}

// TestRealOutputShapesParseIdentically proves the cited trailing
// whitespace makes no difference to the parsed result.
func TestRealOutputShapesParseIdentically(t *testing.T) {
	row := `{"from_table_name":"users","to_table_name":"users","diff_type":"modified","data_change":true,"schema_change":false}`
	withWS, err := ParseDiffSummaryJSON([]byte(`{"rows":[` + row + "]}\n"))
	if err != nil {
		t.Fatalf("with whitespace: %v", err)
	}
	withoutWS, err := ParseDiffSummaryJSON([]byte(`{"rows":[` + row + `]}`))
	if err != nil {
		t.Fatalf("without whitespace: %v", err)
	}
	if len(withWS) != len(withoutWS) || withWS[0] != withoutWS[0] {
		t.Fatal("trailing whitespace changed the parse result")
	}
}

// TestDiffTypeTrackedVerbatim proves a rename is tracked as one row
// with differing names, and an unknown fifth diff_type is preserved
// verbatim rather than validated against the four-value enum.
func TestDiffTypeTrackedVerbatim(t *testing.T) {
	raw := `{"rows":[` +
		`{"from_table_name":"old","to_table_name":"new","diff_type":"renamed","data_change":false,"schema_change":true},` +
		`{"from_table_name":"","to_table_name":"added_t","diff_type":"added","data_change":true,"schema_change":true},` +
		`{"from_table_name":"gone_t","to_table_name":"","diff_type":"dropped","data_change":true,"schema_change":true},` +
		`{"from_table_name":"f","to_table_name":"f","diff_type":"future-value","data_change":false,"schema_change":false}` +
		`]}`
	rows, err := ParseDiffSummaryJSON([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(rows))
	}
	if rows[0].FromTableName != "old" || rows[0].ToTableName != "new" {
		t.Fatal("a rename must not be collapsed into an added+dropped pair")
	}
	if rows[1].FromTableName != "" || rows[2].ToTableName != "" {
		t.Fatal("added/dropped use the empty string, never null or omitted")
	}
	if rows[3].DiffType != "future-value" {
		t.Fatal("an unknown diff_type must be tracked verbatim")
	}
	result := DiffSummaryResult(rows)
	tables, ok := result.Field("tables")
	if !ok || tables.Kind != store.CanonKindArray || len(tables.Array) != 4 {
		t.Fatal("result must be a tables array of four rows")
	}
}

// TestZeroRowResultShape proves both zero-row causes use the same
// `{"tables": []}` shape, never a special-cased schema.
func TestZeroRowResultShape(t *testing.T) {
	for _, raw := range []string{"{}\n\n", `{"rows":[]}`} {
		rows, err := ParseDiffSummaryJSON([]byte(raw))
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		node := DiffSummaryResult(rows)
		data, err := node.MarshalJSON()
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if string(data) != `{"tables":[]}` {
			t.Fatalf("zero-row result = %s, want {\"tables\":[]}", data)
		}
	}
}

// TestParseDoltSelector covers the selector shape contract.
func TestParseDoltSelector(t *testing.T) {
	capability, table, err := ParseDoltSelector("dolt:diff-summary:users")
	if err != nil || capability != "diff-summary" || table != "users" {
		t.Fatalf("got (%q,%q,%v)", capability, table, err)
	}
	for _, bad := range []string{"dolt:diff-summary", "other:diff-summary:users", "dolt:table-diff:users", "dolt:diff-summary:"} {
		if _, _, err := ParseDoltSelector(bad); err == nil {
			t.Fatalf("%q should be refused", bad)
		}
	}
}

// TestIsValidBinarySHA256 covers trust-dolt's pre-lock validation.
func TestIsValidBinarySHA256(t *testing.T) {
	if !IsValidBinarySHA256(strings.Repeat("a", 64)) {
		t.Fatal("64 lowercase hex must be accepted")
	}
	for _, bad := range []string{"", strings.Repeat("a", 63), strings.Repeat("a", 65), strings.Repeat("A", 64), strings.Repeat("g", 64)} {
		if IsValidBinarySHA256(bad) {
			t.Fatalf("%q must be refused", bad)
		}
	}
}

// writeFixtureExecutable creates a controlled stand-in binary outside
// any repository.
func writeFixtureExecutable(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// TestHashExecutableDescriptorIsCopyFree covers §6.1's add-time TOFU:
// the digest is computed from the opened descriptor and no scratch file
// is created.
func TestHashExecutableDescriptorIsCopyFree(t *testing.T) {
	dir := t.TempDir()
	path := writeFixtureExecutable(t, dir, "dolt", "#!/bin/sh\nexit 0\n")
	digest, err := HashExecutableDescriptor(path)
	if err != nil {
		t.Fatalf("HashExecutableDescriptor: %v", err)
	}
	if !IsValidBinarySHA256(digest) {
		t.Fatalf("digest %q is not 64 lowercase hex", digest)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("the bootstrap created %d extra files; it must create zero", len(entries)-1)
	}
}

// TestMakeVerifiedPrivateCopy covers the capture-time sequence's whole
// mode/verify contract, including the untrusted refusal leaving no
// executable behind.
func TestMakeVerifiedPrivateCopy(t *testing.T) {
	srcDir := t.TempDir()
	scratch := t.TempDir()
	path := writeFixtureExecutable(t, srcDir, "dolt", "#!/bin/sh\nexit 0\n")
	digest, err := HashExecutableDescriptor(path)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	t.Run("verified-copy-is-0500", func(t *testing.T) {
		copyFile, err := MakeVerifiedPrivateCopy(path, scratch, digest)
		if err != nil {
			t.Fatalf("MakeVerifiedPrivateCopy: %v", err)
		}
		defer copyFile.Remove()
		info, err := os.Stat(copyFile.Path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if info.Mode().Perm() != 0o500 {
			t.Fatalf("private copy mode = %v, want 0500", info.Mode().Perm())
		}
		if !strings.HasPrefix(filepath.Base(copyFile.Path), "dolt-copy-") {
			t.Fatalf("unexpected private copy name %s", copyFile.Path)
		}
		if copyFile.Digest != digest {
			t.Fatal("the recorded digest must be the streamed digest")
		}
	})

	t.Run("untrusted-digest-refuses-and-leaves-nothing", func(t *testing.T) {
		fresh := t.TempDir()
		_, err := MakeVerifiedPrivateCopy(path, fresh, strings.Repeat("0", 64))
		r := AsRefusal(err)
		if r == nil || r.Reason != ReasonAdapterBinaryUntrusted || r.Code != ExitRefusal {
			t.Fatalf("want adapter-binary-untrusted exit 3, got %v", err)
		}
		entries, _ := os.ReadDir(fresh)
		if len(entries) != 0 {
			t.Fatalf("a refused copy left %d files behind", len(entries))
		}
	})

	t.Run("missing-pin-refuses-before-opening", func(t *testing.T) {
		fresh := t.TempDir()
		_, err := MakeVerifiedPrivateCopy(path, fresh, "")
		r := AsRefusal(err)
		if r == nil || r.Reason != ReasonDoltTrustRequired || r.Code != ExitRefusal {
			t.Fatalf("want dolt-trust-required exit 3, got %v", err)
		}
	})
}

// TestResolveExternalExecutablePolicy covers §9.2's opposite-direction
// policy: outside the repo is accepted (symlinks followed), inside the
// repo or under .git is refused, and a LookPath failure carries the
// caller's chosen name.
func TestResolveExternalExecutablePolicy(t *testing.T) {
	repoRoot := t.TempDir()
	outside := t.TempDir()

	t.Run("outside-repo-accepted-through-symlink", func(t *testing.T) {
		real := writeFixtureExecutable(t, outside, "dolt-real", "#!/bin/sh\nexit 0\n")
		shim := filepath.Join(outside, "dolt-shim")
		if err := os.Symlink(real, shim); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		restore := SetLookPathForTest(func(string) (string, error) { return shim, nil })
		defer restore()
		got, err := ResolveExternalExecutable(repoRoot, "dolt", Refuse(ReasonAdapterMissing, "missing"))
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if filepath.Base(got) != "dolt-real" {
			t.Fatalf("resolved to %s; the symlink target should be validated", got)
		}
	})

	t.Run("inside-repo-refused", func(t *testing.T) {
		inRepo := writeFixtureExecutable(t, repoRoot, "dolt", "#!/bin/sh\nexit 0\n")
		restore := SetLookPathForTest(func(string) (string, error) { return inRepo, nil })
		defer restore()
		_, err := ResolveExternalExecutable(repoRoot, "dolt", Refuse(ReasonAdapterMissing, "missing"))
		r := AsRefusal(err)
		if r == nil || r.Reason != ReasonAdapterExecutableInRepo {
			t.Fatalf("want adapter-executable-in-repo, got %v", err)
		}
	})

	t.Run("under-git-directory-refused", func(t *testing.T) {
		gitDir := filepath.Join(outside, ".git", "hooks")
		if err := os.MkdirAll(gitDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		inGit := writeFixtureExecutable(t, gitDir, "dolt", "#!/bin/sh\nexit 0\n")
		restore := SetLookPathForTest(func(string) (string, error) { return inGit, nil })
		defer restore()
		_, err := ResolveExternalExecutable(repoRoot, "dolt", Refuse(ReasonAdapterMissing, "missing"))
		r := AsRefusal(err)
		if r == nil || r.Reason != ReasonAdapterExecutableInRepo {
			t.Fatalf("want adapter-executable-in-repo, got %v", err)
		}
	})

	t.Run("lookpath-failure-carries-callers-name", func(t *testing.T) {
		restore := SetLookPathForTest(func(string) (string, error) { return "", os.ErrNotExist })
		defer restore()
		_, addErr := ResolveExternalExecutable(repoRoot, "dolt",
			Invalid(ReasonAdapterMissingAtAdd, "add-time"))
		r := AsRefusal(addErr)
		if r == nil || r.Reason != ReasonAdapterMissingAtAdd || r.Code != ExitValidation {
			t.Fatalf("want adapter-missing-at-add exit 2, got %v", addErr)
		}
		_, capErr := ResolveExternalExecutable(repoRoot, "dolt",
			Refuse(ReasonAdapterMissing, "capture-time"))
		r = AsRefusal(capErr)
		if r == nil || r.Reason != ReasonAdapterMissing || r.Code != ExitRefusal {
			t.Fatalf("want adapter-missing exit 3, got %v", capErr)
		}
	})

	t.Run("non-executable-file-refused", func(t *testing.T) {
		plain := filepath.Join(outside, "not-exec")
		if err := os.WriteFile(plain, []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		restore := SetLookPathForTest(func(string) (string, error) { return plain, nil })
		defer restore()
		_, err := ResolveExternalExecutable(repoRoot, "dolt", Refuse(ReasonAdapterMissing, "missing"))
		if AsRefusal(err) == nil {
			t.Fatalf("want a refusal, got %v", err)
		}
	})
}

// TestNoVersionProbeAnywhere proves `dolt version` is never invoked:
// the string does not appear in any argv this package can construct.
func TestNoVersionProbeAnywhere(t *testing.T) {
	argv := DiffSummaryArgv("/bin/dolt", BuildDiffSummarySQL("main", "HEAD", "users"))
	for _, a := range argv {
		if strings.Contains(a, "version") {
			t.Fatalf("argv must never contain a version probe: %v", argv)
		}
	}
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, f := range sources {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if strings.Contains(string(body), `"version"`) {
			t.Fatalf("%s references a version subcommand", f)
		}
	}
}
