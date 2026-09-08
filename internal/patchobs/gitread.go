package patchobs

// Batched Git reads for the immutable observation (GH #15 / ADR-036 D2).
//
// An observation describes every effect of a patch, and each effect has
// two sides. Reading them one subprocess at a time makes the cost of
// observing O(effects) — a 200-file patch would fork 400+ times before a
// producer's first write. Every read here is therefore BATCHED: one
// process answers every path of one reference, and one process answers
// every blob body the whole observation needs.
//
// The process budget per observation is:
//
//	≤1  rev-parse per distinct reference (preimage, postimage)
//	≤1  ls-tree   per distinct commit    (all paths of that commit)
//	 1  cat-file --batch                 (all blob bodies, all commits)
//	≤1  ls-files --stage                 (only when a worktree postimage
//	                                      is a gitlink directory)
//
// None of those counts scales with the number of effects, which is the
// property gitProcessCount() exists to let a test measure rather than
// assume.
//
// Two invariants hold for every process started here:
//
//   - `GIT_NO_LAZY_FETCH=1`. Observing a partial clone must never reach
//     the network: a missing object is recorded as UNOBSERVED, which is
//     the truth, instead of being fetched behind the producer's back.
//   - `--literal-pathspecs`. A repo-relative path containing `*`, `?`,
//     `[` or a leading `:` would otherwise be read as pathspec magic, and
//     the observation would describe a different object than the effect
//     names.

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// gitPathspecBudget bounds how many bytes of pathspec arguments one
// process receives. It exists so a pathological patch cannot exceed the
// platform argument limit; at 64 KiB an ordinary patch — even a very
// large one — is answered by a single process.
const gitPathspecBudget = 64 << 10

var (
	gitProcMu    sync.Mutex
	gitProcCount int
)

// gitProcessCount reports how many Git subprocesses this package has
// started. It is the seam a test uses to prove the read cost is bounded
// per reference rather than per effect.
func gitProcessCount() int {
	gitProcMu.Lock()
	defer gitProcMu.Unlock()
	return gitProcCount
}

func countGitProcess() {
	gitProcMu.Lock()
	gitProcCount++
	gitProcMu.Unlock()
}

// runGit runs one local Git plumbing command and returns its stdout.
// Nothing here writes to the repository and nothing fetches.
func runGit(repoRoot string, stdin []byte, args ...string) ([]byte, error) {
	countGitProcess()
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "GIT_NO_LAZY_FETCH=1")
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	return stdout.Bytes(), err
}

// treeEntryRecord is one path's mode and object id in a tree or index.
type treeEntryRecord struct {
	mode      string
	objectSHA string
}

// treeSnapshot is one batched read of a commit's entries for a path set.
//
// `read` distinguishes "the batch ran and this path is genuinely absent"
// from "the batch itself failed". The first is proven absence; the second
// leaves every side of that reference unobserved, because nothing was
// established about any of them.
type treeSnapshot struct {
	entries map[string]treeEntryRecord
	read    bool
}

func (t treeSnapshot) lookup(path string) (treeEntryRecord, bool, bool) {
	if !t.read {
		return treeEntryRecord{}, false, false
	}
	entry, found := t.entries[path]
	return entry, found, true
}

// emptyTreeSnapshot is the result of a batch nobody needed to run: it was
// read (vacuously) and it holds nothing.
func emptyTreeSnapshot() treeSnapshot {
	return treeSnapshot{entries: map[string]treeEntryRecord{}, read: true}
}

// resolveCommit resolves one ref to a full commit id through a single
// counted process. A ref that could look like an option is refused rather
// than passed to Git.
func resolveCommit(repoRoot, ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(ref, "-") {
		return "", false
	}
	out, err := runGit(repoRoot, nil, "rev-parse", "--verify", ref)
	if err != nil {
		return "", false
	}
	commit := strings.TrimSpace(string(out))
	if !isFullCommitHex(commit) {
		return "", false
	}
	return commit, true
}

// readTreeEntries reads every requested path's tree entry at one commit
// in as few processes as the platform argument limit allows.
func readTreeEntries(repoRoot, commit string, paths []string) treeSnapshot {
	unique := sortedUnique(paths)
	if commit == "" || len(unique) == 0 {
		return emptyTreeSnapshot()
	}
	snapshot := treeSnapshot{entries: make(map[string]treeEntryRecord, len(unique)), read: true}
	for _, chunk := range chunkPathspecs(unique) {
		args := append([]string{"--literal-pathspecs", "ls-tree", "-z", "--full-name", commit, "--"}, chunk...)
		out, err := runGit(repoRoot, nil, args...)
		if err != nil {
			return treeSnapshot{}
		}
		if perr := parseLsTreeZ(out, snapshot.entries); perr != nil {
			return treeSnapshot{}
		}
	}
	return snapshot
}

// readIndexEntries reads the index's own entry for each requested path.
// It answers exactly one question the working tree cannot: a directory in
// the worktree that the index records as a gitlink (mode 160000) carries
// a submodule commit id, and that id is the only truthful identity for
// the side.
func readIndexEntries(repoRoot string, paths []string) treeSnapshot {
	unique := sortedUnique(paths)
	if len(unique) == 0 {
		return emptyTreeSnapshot()
	}
	snapshot := treeSnapshot{entries: make(map[string]treeEntryRecord, len(unique)), read: true}
	for _, chunk := range chunkPathspecs(unique) {
		args := append([]string{"--literal-pathspecs", "ls-files", "-s", "-z", "--"}, chunk...)
		out, err := runGit(repoRoot, nil, args...)
		if err != nil {
			return treeSnapshot{}
		}
		if perr := parseLsFilesStageZ(out, snapshot.entries); perr != nil {
			return treeSnapshot{}
		}
	}
	return snapshot
}

type blobObservation struct {
	bytes      []byte
	diagnostic string
}

// readBlobs reads every requested blob body in ONE process, whichever
// reference asked for it. Object ids are repository-global, so batching
// across references is exact rather than an approximation.
//
// A missing object yields no map entry. An over-budget body yields an
// explicit refusal. Both leave the side unobserved, never truncated.
func readBlobs(repoRoot string, objectIDs []string, budget *imageBudget) map[string]blobObservation {
	unique := sortedUnique(objectIDs)
	out := make(map[string]blobObservation, len(unique))
	if len(unique) == 0 {
		return out
	}
	cmd := exec.Command("git", "cat-file", "--batch")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "GIT_NO_LAZY_FETCH=1")
	cmd.Stdin = strings.NewReader(strings.Join(unique, "\n") + "\n")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return out
	}
	countGitProcess()
	if err := cmd.Start(); err != nil {
		stdout.Close()
		return out
	}
	// Never collect raw batch stdout. The only body arrays allocated by
	// the streaming parser are the ones retained within this budget.
	if err := parseCatFileBatch(stdout, out, budget); err != nil {
		// A malformed stream cannot be resynchronized reliably. Stop the
		// child rather than leaving it blocked on an unread stdout pipe.
		_ = cmd.Process.Kill()
	}
	stdout.Close()
	_ = cmd.Wait()
	return out
}

// parseLsTreeZ decodes `<mode> <type> <object>\t<path>\0` records.
func parseLsTreeZ(raw []byte, into map[string]treeEntryRecord) error {
	for _, record := range splitNUL(raw) {
		meta, path, ok := strings.Cut(record, "\t")
		if !ok {
			return fmt.Errorf("unreadable ls-tree record %q", record)
		}
		fields := strings.Fields(meta)
		if len(fields) < 3 {
			return fmt.Errorf("unreadable ls-tree record %q", record)
		}
		into[path] = treeEntryRecord{mode: fields[0], objectSHA: fields[2]}
	}
	return nil
}

// parseLsFilesStageZ decodes `<mode> <object> <stage>\t<path>\0` records.
// A path with more than one stage is conflicted, so no single entry
// describes it; such a path is dropped and the side stays unobserved.
func parseLsFilesStageZ(raw []byte, into map[string]treeEntryRecord) error {
	conflicted := map[string]bool{}
	for _, record := range splitNUL(raw) {
		meta, path, ok := strings.Cut(record, "\t")
		if !ok {
			return fmt.Errorf("unreadable ls-files record %q", record)
		}
		fields := strings.Fields(meta)
		if len(fields) < 3 {
			return fmt.Errorf("unreadable ls-files record %q", record)
		}
		if fields[2] != "0" {
			conflicted[path] = true
			continue
		}
		into[path] = treeEntryRecord{mode: fields[0], objectSHA: fields[1]}
	}
	for path := range conflicted {
		delete(into, path)
	}
	return nil
}

// parseCatFileBatch decodes the `--batch` stream:
//
//	<oid> <type> <size>\n<body>\n      for an existing object
//	<input> missing\n                  for one that is not there
//
// Only `blob` bodies are recorded. Oversized and non-blob bodies are
// drained with fixed scratch space, allowing later small blobs through.
// The header reader is also bounded, including on malformed input.
func parseCatFileBatch(stream io.Reader, into map[string]blobObservation, budget *imageBudget) error {
	reader := bufio.NewReaderSize(stream, 4096)
	var scratch [32 << 10]byte
	for {
		header, err := reader.ReadSlice('\n')
		if err == io.EOF && len(header) == 0 {
			return nil
		}
		if err != nil {
			return fmt.Errorf("unreadable cat-file header: %w", err)
		}
		fields := strings.Fields(string(header))
		if len(fields) == 2 && fields[1] == "missing" {
			continue
		}
		if len(fields) != 3 {
			return fmt.Errorf("unreadable cat-file header")
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || size < 0 {
			return fmt.Errorf("invalid cat-file body size")
		}
		diagnostic := budget.refusal(size)
		_, duplicate := into[fields[0]]
		retain := fields[1] == "blob" && diagnostic == "" && !duplicate
		var body []byte
		if retain {
			body, err = readImage(reader, size, budget)
		} else {
			if fields[1] == "blob" && !duplicate && diagnostic != "" {
				into[fields[0]] = blobObservation{diagnostic: diagnostic}
			}
			err = discardImage(reader, size, scratch[:])
		}
		if err != nil {
			return fmt.Errorf("unreadable cat-file body: %w", err)
		}
		delimiter, err := reader.ReadByte()
		if err != nil || delimiter != '\n' {
			if retain {
				budget.used -= size
			}
			return fmt.Errorf("missing cat-file body delimiter")
		}
		if retain {
			into[fields[0]] = blobObservation{bytes: body}
		}
	}
}

func discardImage(reader io.Reader, size int64, scratch []byte) error {
	for size > 0 {
		n := int64(len(scratch))
		if size < n {
			n = size
		}
		if _, err := io.ReadFull(reader, scratch[:int(n)]); err != nil {
			return err
		}
		size -= n
	}
	return nil
}

func splitNUL(raw []byte) []string {
	text := strings.TrimRight(string(raw), "\x00")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\x00")
}

// chunkPathspecs splits a path set into argument-limit-safe batches. An
// ordinary observation yields exactly one chunk.
func chunkPathspecs(paths []string) [][]string {
	var chunks [][]string
	var current []string
	used := 0
	for _, p := range paths {
		if len(current) > 0 && used+len(p)+1 > gitPathspecBudget {
			chunks = append(chunks, current)
			current, used = nil, 0
		}
		current = append(current, p)
		used += len(p) + 1
	}
	if len(current) > 0 {
		chunks = append(chunks, current)
	}
	return chunks
}

// sortedObjectIDs is a small helper keeping blob batching deterministic.
func sortedObjectIDs(ids map[string]bool) []string {
	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
