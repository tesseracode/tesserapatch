// Capture orchestration for
// PRD-feature-resource-claims-and-capture-adapters §7.3.
//
// Staging is all-or-nothing and entirely in bounded in-process memory:
// if any targeted resource's staging fails, no batch file is written,
// the ephemeral scratch is removed, the lock is released, and the
// command exits with that refusal's own code. Only after every targeted
// resource has staged successfully does the publication transaction run.

package rescap

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tesseracode/tesserapatch/internal/redact"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// Engine owns one invocation's capture work.
type Engine struct {
	Store    *store.Store
	Slug     string
	RepoRoot string

	// InvocationTimeout, TerminateGrace, ReapDeadline and DrainDeadline
	// override the §6.4 defaults. Tests set them; production leaves
	// them zero.
	InvocationTimeout time.Duration
	TerminateGrace    time.Duration
	ReapDeadline      time.Duration
	DrainDeadline     time.Duration
	OutputCap         int64

	Diagnostics []string
}

// NewEngine builds an engine for a slug.
func NewEngine(s *store.Store, slug string) *Engine {
	return &Engine{Store: s, Slug: slug, RepoRoot: s.Root}
}

// StagedBatch is the fully-computed, not-yet-published candidate.
type StagedBatch struct {
	Batch     store.Batch
	Canonical []byte
}

// Stage runs §7.3 step 1 for every targeted resource and then step 2's
// content-addressed batch_id computation. It writes nothing.
func (e *Engine) Stage(resources []store.Resource, scratch *Scratch) (StagedBatch, error) {
	results := make([]store.BatchResult, 0, len(resources))
	for _, r := range resources {
		res, err := e.stageOne(r, scratch)
		if err != nil {
			return StagedBatch{}, err
		}
		results = append(results, res)
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].ResourceID < results[j].ResourceID })
	batchID, canonical, err := store.ComputeBatchID(e.Slug, results)
	if err != nil {
		return StagedBatch{}, Internal(ReasonAdapterOutputReadFailed, "computing the batch id: %v", err)
	}
	return StagedBatch{
		Batch:     store.Batch{BatchID: batchID, Feature: e.Slug, Results: results},
		Canonical: canonical,
	}, nil
}

// stageOne performs one resource's kind-specific capture work.
func (e *Engine) stageOne(r store.Resource, scratch *Scratch) (store.BatchResult, error) {
	if err := e.scanDeclaration(r); err != nil {
		return store.BatchResult{}, err
	}
	entry := store.BatchResult{
		ResourceID: r.ResourceID,
		Kind:       r.Kind,
		Selector:   r.Selector,
		Adapter:    r.Adapter,
		Capability: r.Capability,
		Args:       r.Args,
	}
	switch r.Kind {
	case store.ResourceKindIgnoredFile:
		if err := e.recheckIgnoredFileGates(r.Selector); err != nil {
			return store.BatchResult{}, err
		}
		result, raw, err := CaptureIgnoredFile(e.RepoRoot, r.Selector)
		if err != nil {
			return store.BatchResult{}, err
		}
		entry.Result = result
		entry.Raw = raw
		return entry, nil
	case store.ResourceKindGitMetadata:
		result, err := CaptureGitMetadata(e.RepoRoot, classifyGitMetadataView(r.Selector, r.Capability), r.Selector)
		if err != nil {
			return store.BatchResult{}, err
		}
		entry.Result = result
		entry.Raw = nil
		return entry, nil
	case store.ResourceKindAdapterSnapshot:
		result, raw, tool, err := e.captureDolt(r, scratch)
		if err != nil {
			return store.BatchResult{}, err
		}
		entry.Result = result
		entry.Raw = raw
		entry.ToolIdentity = tool
		return entry, nil
	default:
		return store.BatchResult{}, Invalid(ReasonInvalidDeclaration, "unknown resource kind %q", r.Kind)
	}
}

// scanDeclaration applies §8.3's unconditional pre-write scan to the
// selector and every args value.
func (e *Engine) scanDeclaration(r store.Resource) error {
	candidates := []string{r.Selector}
	for _, a := range r.Args {
		candidates = append(candidates, a.Value)
	}
	for _, c := range candidates {
		if findings := redact.ScanString(c); len(findings) > 0 {
			return Refuse(ReasonRedactionRefused,
				"a declared value matched forbidden content classes %s", strings.Join(findings, ", "))
		}
	}
	return nil
}

// recheckIgnoredFileGates re-runs §5.1's two gates at every capture,
// not merely at add time.
func (e *Engine) recheckIgnoredFileGates(selector string) error {
	ignored, err := IsIgnored(e.RepoRoot, selector)
	if err != nil {
		return err
	}
	if !ignored {
		return Refuse(ReasonNotIgnored, "%s is not ignored by git", selector)
	}
	tracked, err := IsTracked(e.RepoRoot, selector)
	if err != nil {
		return err
	}
	if tracked {
		return Refuse(ReasonTrackedAndIgnored,
			"%s reports ignored but is also tracked by git", selector)
	}
	return nil
}

// captureDolt runs the whole capture-time Dolt sequence.
func (e *Engine) captureDolt(r store.Resource, scratch *Scratch) (store.CanonNode, *store.RawInfo, *store.ToolIdentity, error) {
	if scratch == nil {
		return store.CanonNull(), nil, nil, Internal(ReasonAdapterCopyFailed,
			"an adapter-snapshot capture requires an ephemeral scratch directory")
	}
	if r.Trust == nil || r.Trust.BinarySHA256 == "" {
		return store.CanonNull(), nil, nil, Refuse(ReasonDoltTrustRequired,
			"%s has no trust.binary_sha256 pin", r.ResourceID)
	}
	dbPath, _ := r.Arg("db_path")
	table, _ := r.Arg("table")
	from, _ := r.Arg("from")
	to, _ := r.Arg("to")

	gatedDB, err := GatePath(e.RepoRoot, dbPath)
	if err != nil {
		return store.CanonNull(), nil, nil, err
	}
	// The held directory descriptor stays open for the entire lifetime
	// of the child process: cmd.Dir receives only the pathname string,
	// but holding the descriptor guarantees the underlying inode cannot
	// be deleted-and-reused while it is live, and gives the post-exit
	// check a stable reference to compare a fresh resolution against.
	defer func() { _ = gatedDB.Close() }()
	if !gatedDB.IsDir {
		return store.CanonNull(), nil, nil, Refuse(ReasonPathMissing, "db_path %s is not a directory", dbPath)
	}

	resolved, err := ResolveExternalExecutable(e.RepoRoot, store.ResourceAdapterDolt,
		Refuse(ReasonAdapterMissing, "no dolt executable was found on PATH"))
	if err != nil {
		return store.CanonNull(), nil, nil, err
	}

	copyFile, err := MakeVerifiedPrivateCopy(resolved, scratch.Root, r.Trust.BinarySHA256)
	if err != nil {
		return store.CanonNull(), nil, nil, err
	}
	defer copyFile.Remove()

	home, err := scratch.EnsureDoltHome()
	if err != nil {
		return store.CanonNull(), nil, nil, err
	}

	// Immediately before cmd.Start(): a fresh pathname resolution
	// compared against the already-held descriptor.
	if err := SamePathIdentity(e.RepoRoot, dbPath, gatedDB.Info); err != nil {
		return store.CanonNull(), nil, nil, err
	}

	sql := BuildDiffSummarySQL(from, to, table)
	argv := DiffSummaryArgv(copyFile.Path, sql)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Path = copyFile.Path
	cmd.Dir = gatedDB.AbsPath
	// A fresh, minimal environment: no inherited credentials, and no
	// PATH at all, since the adapter is invoked by an absolute path.
	cmd.Env = []string{"HOME=" + home, "DOLT_ROOT_PATH=" + home}

	runner := &ProcessRunner{
		Cmd:               cmd,
		InvocationTimeout: e.InvocationTimeout,
		TerminateGrace:    e.TerminateGrace,
		ReapDeadline:      e.ReapDeadline,
		DrainDeadline:     e.DrainDeadline,
		OutputCap:         e.OutputCap,
	}
	outcome, runErr := runner.Run()
	e.Diagnostics = append(e.Diagnostics, outcome.Diagnostics...)
	if runErr != nil {
		if startErr, ok := runErr.(*StartFailureError); ok {
			return store.CanonNull(), nil, nil, Refuse(ReasonAdapterMissing,
				"the private dolt copy could not be started: %v", startErr.Err)
		}
		return store.CanonNull(), nil, nil, runErr
	}

	// After the child process exits: resolve db_path fresh a third
	// time and compare against the same held descriptor. A mismatch is
	// a hard refusal — publishing a result gathered against a db_path
	// that no longer resolves to the validated directory would mean the
	// tracked result describes an unverified database.
	if err := SamePathIdentity(e.RepoRoot, dbPath, gatedDB.Info); err != nil {
		return store.CanonNull(), nil, nil, err
	}

	if outcome.WaitErr != nil {
		e.Diagnostics = append(e.Diagnostics,
			fmt.Sprintf("dolt stderr: %s", strings.TrimSpace(string(outcome.Stderr))))
		return store.CanonNull(), nil, nil, Refuse(ReasonDoltQueryError,
			"the dolt query exited non-zero: %v", outcome.WaitErr)
	}

	// Only stdout is ever handed to the parser; stderr exists solely
	// for redaction scanning and local diagnostics.
	if findings := redact.Scan(outcome.Stdout); len(findings) > 0 {
		return store.CanonNull(), nil, nil, Refuse(ReasonRedactionRefused,
			"dolt stdout matched forbidden content classes %s", strings.Join(findings, ", "))
	}
	if findings := redact.Scan(outcome.Stderr); len(findings) > 0 {
		return store.CanonNull(), nil, nil, Refuse(ReasonRedactionRefused,
			"dolt stderr matched forbidden content classes %s", strings.Join(findings, ", "))
	}
	rows, err := ParseDiffSummaryJSON(outcome.Stdout)
	if err != nil {
		return store.CanonNull(), nil, nil, err
	}
	rawDigest := sha256Hex(outcome.Stdout)
	return DiffSummaryResult(rows),
		&store.RawInfo{Hash: "sha256:" + rawDigest, ByteCount: uint64(len(outcome.Stdout))},
		&store.ToolIdentity{Basename: filepath.Base(resolved), BinarySHA256: r.Trust.BinarySHA256},
		nil
}

// Publish runs §7.3 steps 3-4.
func (e *Engine) Publish(staged StagedBatch) (store.PublishOutcome, error) {
	outcome, err := e.Store.PublishBatch(e.Slug, staged.Batch, staged.Canonical)
	if err != nil {
		if pubErr, ok := err.(*store.PublicationError); ok {
			return outcome, Refuse(pubErr.Reason, "%s", pubErr.Detail)
		}
		return outcome, Internal(ReasonAdapterCopyFailed, "publishing the batch: %v", err)
	}
	return outcome, nil
}

// RemoveScratch cleans up the ephemeral tree and records any failure as
// a local diagnostic.
func (e *Engine) RemoveScratch(scratch *Scratch) {
	e.Diagnostics = append(e.Diagnostics, scratch.Remove()...)
}

// WriteLocalDiagnostics writes a redacted, bounded failure summary into
// the ephemeral tree, which is deleted at the end of the invocation
// regardless of outcome. No tracked failure envelope is ever written.
func (e *Engine) WriteLocalDiagnostics(scratch *Scratch) {
	if scratch == nil || len(e.Diagnostics) == 0 {
		return
	}
	path := filepath.Join(scratch.Root, "diagnostics.txt")
	body := strings.Join(e.Diagnostics, "\n") + "\n"
	if findings := redact.Scan([]byte(body)); len(findings) > 0 {
		body = "diagnostics withheld: they matched forbidden content classes\n"
	}
	_ = os.WriteFile(path, []byte(body), 0o600)
}

// sha256Hex hashes bytes and returns lowercase hex.
func sha256Hex(b []byte) string {
	d := sha256.Sum256(b)
	return hex.EncodeToString(d[:])
}
