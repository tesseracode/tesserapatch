// End-to-end adapter tests driven by a controlled fake `dolt`
// executable (PRD §6, §7.3, §9.1).
//
// The suite must not depend on an installed Dolt binary, so the
// executable resolver is pointed at a fixture script that records the
// argv, working directory and environment it was invoked with. That is
// enough to verify every claim this design makes about *how* the child
// is invoked, which is the part tpatch owns.

package rescap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tesseracode/tesserapatch/internal/store"
)

// doltFixture is a fake dolt binary plus the file it records its
// invocation into.
type doltFixture struct {
	Path       string
	Digest     string
	ObservedAt string
}

// newDoltFixture writes a shell-script stand-in outside any repository
// and returns its pinned digest.
func newDoltFixture(t *testing.T, stdout, stderr string, exitCode int) doltFixture {
	t.Helper()
	dir := t.TempDir()
	observed := filepath.Join(dir, "observed.txt")
	script := "#!/bin/sh\n" +
		"{\n" +
		"  echo \"cwd=$(pwd)\"\n" +
		"  echo \"argv0=$0\"\n" +
		"  for a in \"$@\"; do echo \"arg=$a\"; done\n" +
		"  env | sed 's/^/env=/' | sort\n" +
		"} > " + observed + "\n" +
		"printf '%s' " + shellQuote(stdout) + "\n" +
		"printf '%s' " + shellQuote(stderr) + " 1>&2\n" +
		"exit " + itoa(exitCode) + "\n"
	path := filepath.Join(dir, "dolt")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	digest, err := HashExecutableDescriptor(path)
	if err != nil {
		t.Fatalf("hash fixture: %v", err)
	}
	return doltFixture{Path: path, Digest: digest, ObservedAt: observed}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// doltEngineFixture wires a repo, a store, a declared Dolt resource and
// an engine with test-speed process bounds.
type doltEngineFixture struct {
	Repo     string
	Store    *store.Store
	Resource store.Resource
	Engine   *Engine
	Scratch  *Scratch
	Dolt     doltFixture
}

func newDoltEngineFixture(t *testing.T, fake doltFixture) *doltEngineFixture {
	t.Helper()
	repo := newGitRepo(t)
	if err := os.MkdirAll(filepath.Join(repo, "data", "dolt-db"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	s, err := store.Init(repo)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	args := []store.ResourceArg{
		{Key: "contract", Value: store.DoltContractDiffSummary1},
		{Key: "db_path", Value: "data/dolt-db"},
		{Key: "from", Value: "main"},
		{Key: "table", Value: "users"},
		{Key: "to", Value: "HEAD"},
	}
	res := store.Resource{
		Kind: store.ResourceKindAdapterSnapshot, Selector: "dolt:diff-summary:users",
		Adapter: "dolt", Capability: "diff-summary", Args: args,
		Trust: &store.ResourceTrust{BinarySHA256: fake.Digest},
	}
	res.ResourceID = store.ComputeResourceID("model-picker", res.Kind, res.Selector, res.Adapter, res.Capability, args)

	scratch, err := EphemeralScratch(repo, "model-picker")
	if err != nil {
		t.Fatalf("EphemeralScratch: %v", err)
	}
	t.Cleanup(func() { scratch.Remove() })

	engine := NewEngine(s, "model-picker")
	engine.InvocationTimeout = 10 * time.Second
	engine.TerminateGrace = 10 * time.Millisecond
	engine.ReapDeadline = 3 * time.Second
	engine.DrainDeadline = time.Second

	return &doltEngineFixture{Repo: repo, Store: s, Resource: res, Engine: engine, Scratch: scratch, Dolt: fake}
}

// TestEngineDoltCaptureEndToEnd covers the invocation contract: the
// exact argv, the private-copy argv[0], cmd.Dir bound to the gated
// db_path, the minimal environment, and the tracked tool_identity
// carrying only a basename plus the pinned digest.
func TestEngineDoltCaptureEndToEnd(t *testing.T) {
	row := `{"from_table_name":"users","to_table_name":"users","diff_type":"modified","data_change":true,"schema_change":false}`
	fake := newDoltFixture(t, `{"rows":[`+row+"]}\n", "", 0)
	f := newDoltEngineFixture(t, fake)
	restore := SetLookPathForTest(func(string) (string, error) { return fake.Path, nil })
	defer restore()

	staged, err := f.Engine.Stage([]store.Resource{f.Resource}, f.Scratch)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if len(staged.Batch.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(staged.Batch.Results))
	}
	entry := staged.Batch.Results[0]

	// tool_identity carries only a basename plus the pinned digest —
	// never an absolute path, and never a freshly-recomputed value.
	if entry.ToolIdentity == nil {
		t.Fatal("adapter-snapshot must populate tool_identity")
	}
	if entry.ToolIdentity.Basename != "dolt" {
		t.Fatalf("basename = %q, want dolt", entry.ToolIdentity.Basename)
	}
	if strings.Contains(entry.ToolIdentity.Basename, "/") {
		t.Fatal("tool_identity must never contain a path")
	}
	if entry.ToolIdentity.BinarySHA256 != fake.Digest {
		t.Fatal("tool_identity must record the pinned digest")
	}
	if entry.Raw == nil || entry.Raw.ByteCount == 0 {
		t.Fatalf("raw = %+v, want a populated hash/byte_count", entry.Raw)
	}
	tables, _ := entry.Result.Field("tables")
	if len(tables.Array) != 1 {
		t.Fatalf("tables = %d rows, want 1", len(tables.Array))
	}

	observed, err := os.ReadFile(fake.ObservedAt)
	if err != nil {
		t.Fatalf("read observations: %v", err)
	}
	text := string(observed)

	// cmd.Dir is the gated db_path.
	wantCwd, err := filepath.EvalSymlinks(filepath.Join(f.Repo, "data", "dolt-db"))
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if !strings.Contains(text, "cwd="+wantCwd) {
		t.Fatalf("cwd not bound to db_path:\n%s", text)
	}

	// argv[0] is the private, hash-verified copy under ephemeral
	// scratch — never the originally-resolved pathname.
	if !strings.Contains(text, "argv0="+f.Scratch.Root+"/dolt-copy-") {
		t.Fatalf("argv[0] is not the private copy:\n%s", text)
	}
	if strings.Contains(text, "argv0="+fake.Path) {
		t.Fatal("the originally-resolved pathname must never be executed")
	}

	// The exact argv.
	for _, want := range []string{"arg=sql", "arg=-r", "arg=json", "arg=-q"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if !strings.Contains(text, "dolt_diff_summary('main', 'HEAD', 'users')") {
		t.Fatalf("the 3-argument query shape is missing:\n%s", text)
	}
	if !strings.Contains(text, "ORDER BY from_table_name, to_table_name;") {
		t.Fatalf("the explicit ORDER BY is missing:\n%s", text)
	}

	// The environment is fresh and minimal: HOME and DOLT_ROOT_PATH
	// both point at the ephemeral scratch HOME, PATH is not set, and
	// nothing is inherited.
	var envLines []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "env=") {
			envLines = append(envLines, strings.TrimPrefix(line, "env="))
		}
	}
	seen := map[string]string{}
	for _, kv := range envLines {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		// /bin/sh itself defines a couple of shell-local variables
		// (PWD, SHLVL, _) that are not inherited from tpatch.
		switch k {
		case "PWD", "SHLVL", "_":
			continue
		}
		seen[k] = v
	}
	if len(seen) != 2 {
		t.Fatalf("environment has %d inherited entries, want exactly HOME and DOLT_ROOT_PATH: %v", len(seen), seen)
	}
	home, ok := seen["HOME"]
	if !ok || !strings.HasPrefix(home, f.Scratch.Root) {
		t.Fatalf("HOME = %q, want an ephemeral scratch directory", home)
	}
	if seen["DOLT_ROOT_PATH"] != home {
		t.Fatalf("DOLT_ROOT_PATH = %q, want it to equal HOME", seen["DOLT_ROOT_PATH"])
	}
	if _, ok := seen["PATH"]; ok {
		t.Fatal("PATH must not be set: the adapter is invoked by absolute path")
	}

	// The private copy is deleted after the child exits.
	entries, err := os.ReadDir(f.Scratch.Root)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "dolt-copy-") {
			t.Fatalf("the private copy %s survived the invocation", e.Name())
		}
	}
}

// TestEngineDoltQueryErrorIsExitThree covers the primary-key-set-change
// class of outcome: a non-zero Dolt exit is dolt-query-error (exit 3)
// and its text stays in local diagnostics, never in a tracked artifact.
func TestEngineDoltQueryErrorIsExitThree(t *testing.T) {
	fake := newDoltFixture(t, "", "error: primary key set changed\n", 1)
	f := newDoltEngineFixture(t, fake)
	restore := SetLookPathForTest(func(string) (string, error) { return fake.Path, nil })
	defer restore()

	_, err := f.Engine.Stage([]store.Resource{f.Resource}, f.Scratch)
	r := AsRefusal(err)
	if r == nil || r.Reason != ReasonDoltQueryError || r.Code != ExitRefusal {
		t.Fatalf("want dolt-query-error exit 3, got %v", err)
	}
	joined := strings.Join(f.Engine.Diagnostics, "|")
	if !strings.Contains(joined, "primary key set changed") {
		t.Fatalf("the Dolt error text should be a local diagnostic: %v", f.Engine.Diagnostics)
	}
	if _, statErr := os.Stat(f.Store.ResourceCurrentPath("model-picker")); !os.IsNotExist(statErr) {
		t.Fatal("a failed query must publish nothing")
	}
}

// TestEngineZeroRowCapture covers the nonexistent-table outcome: zero
// rows, not an error, and the same `{"tables": []}` shape.
func TestEngineZeroRowCapture(t *testing.T) {
	fake := newDoltFixture(t, "{}\n\n", "", 0)
	f := newDoltEngineFixture(t, fake)
	restore := SetLookPathForTest(func(string) (string, error) { return fake.Path, nil })
	defer restore()

	staged, err := f.Engine.Stage([]store.Resource{f.Resource}, f.Scratch)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	data, err := staged.Batch.Results[0].Result.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != `{"tables":[]}` {
		t.Fatalf("result = %s, want {\"tables\":[]}", data)
	}
}

// TestEngineDoltJSONParseErrorIsFatal covers §6.3's fail-loudly rule at
// the engine level.
func TestEngineDoltJSONParseErrorIsFatal(t *testing.T) {
	fake := newDoltFixture(t, `{"schema":[],"rows":[]}`+"\n", "", 0)
	f := newDoltEngineFixture(t, fake)
	restore := SetLookPathForTest(func(string) (string, error) { return fake.Path, nil })
	defer restore()

	_, err := f.Engine.Stage([]store.Resource{f.Resource}, f.Scratch)
	r := AsRefusal(err)
	if r == nil || r.Reason != ReasonDoltJSONParseError || r.Code != ExitRefusal {
		t.Fatalf("want dolt-json-parse-error exit 3, got %v", err)
	}
}

// TestEngineUntrustedBinaryRefusesBeforeStarting covers the trust gate:
// a pin mismatch starts no process at all, which the fixture proves by
// never writing its observation file.
func TestEngineUntrustedBinaryRefusesBeforeStarting(t *testing.T) {
	fake := newDoltFixture(t, `{"rows":[]}`, "", 0)
	f := newDoltEngineFixture(t, fake)
	f.Resource.Trust = &store.ResourceTrust{BinarySHA256: strings.Repeat("0", 64)}
	restore := SetLookPathForTest(func(string) (string, error) { return fake.Path, nil })
	defer restore()

	_, err := f.Engine.Stage([]store.Resource{f.Resource}, f.Scratch)
	r := AsRefusal(err)
	if r == nil || r.Reason != ReasonAdapterBinaryUntrusted || r.Code != ExitRefusal {
		t.Fatalf("want adapter-binary-untrusted exit 3, got %v", err)
	}
	if _, statErr := os.Stat(fake.ObservedAt); !os.IsNotExist(statErr) {
		t.Fatal("no process may be started when the digest does not match the pin")
	}
}

// TestEngineDBPathIdentityChangedAfterExit covers §9.1's post-exit
// third resolution: a db_path swapped during the child's execution
// window is a hard refusal and the output is discarded.
func TestEngineDBPathIdentityChangedAfterExit(t *testing.T) {
	fake := newDoltFixture(t, `{"rows":[]}`+"\n", "", 0)
	f := newDoltEngineFixture(t, fake)
	restore := SetLookPathForTest(func(string) (string, error) { return fake.Path, nil })
	defer restore()

	// Swap the directory for a different inode at the same name while
	// the fixture is "running": the fixture writes its observation file
	// first, so a watcher can act during the window. Simpler and fully
	// deterministic: swap it before the post-exit check by running the
	// stage in two halves is not possible, so the swap is performed by
	// the fixture script itself.
	swapScript := "#!/bin/sh\n" +
		"cd " + shellQuote(filepath.Join(f.Repo, "data")) + " || exit 1\n" +
		"mv dolt-db dolt-db-old && mkdir dolt-db\n" +
		"printf '%s' '{\"rows\":[]}'\n"
	if err := os.WriteFile(fake.Path, []byte(swapScript), 0o755); err != nil {
		t.Fatalf("rewrite fixture: %v", err)
	}
	digest, err := HashExecutableDescriptor(fake.Path)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	f.Resource.Trust = &store.ResourceTrust{BinarySHA256: digest}

	_, err = f.Engine.Stage([]store.Resource{f.Resource}, f.Scratch)
	r := AsRefusal(err)
	if r == nil || r.Reason != ReasonDBPathIdentityChanged || r.Code != ExitRefusal {
		t.Fatalf("want db-path-identity-changed exit 3, got %v", err)
	}
	if _, statErr := os.Stat(f.Store.ResourceCurrentPath("model-picker")); !os.IsNotExist(statErr) {
		t.Fatal("a swapped db_path must publish nothing")
	}
}

// TestEngineDoltStdoutRedactionRefusal covers §8.3: Dolt's captured
// stdout is scanned before it is parsed into a tracked result.
func TestEngineDoltStdoutRedactionRefusal(t *testing.T) {
	row := `{"from_table_name":"users@example.com","to_table_name":"users","diff_type":"modified","data_change":true,"schema_change":false}`
	fake := newDoltFixture(t, `{"rows":[`+row+"]}\n", "", 0)
	f := newDoltEngineFixture(t, fake)
	restore := SetLookPathForTest(func(string) (string, error) { return fake.Path, nil })
	defer restore()

	_, err := f.Engine.Stage([]store.Resource{f.Resource}, f.Scratch)
	r := AsRefusal(err)
	if r == nil || r.Reason != ReasonRedactionRefused || r.Code != ExitRefusal {
		t.Fatalf("want redaction-refused exit 3, got %v", err)
	}
}
