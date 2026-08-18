package intent

import (
	"io"
	"io/fs"
	"testing"
	"time"
)

// The doubles below drive the §7.1.1 `RootOps`/`FileOps` seam. They are the
// only way the injected populations (row 13 identity, rows 14/15/17/18/19/20,
// 20a/20b/20c, the status ladder's 8/9/12/14/16a) can be reached
// deterministically, and they are deliberately *faithful*:
//
//   - `fakeFile.Read` maintains a real offset, so `io.ReadFull` produces the
//     genuine `io.EOF` / `io.ErrUnexpectedEOF` / `err == nil` taxonomy of
//     AVP-173 and AVP-174 rather than a hand-forced error;
//   - every `Read` records the requested length and the backing array of the
//     slice it was handed, so AVP-116/170/171/197 assert the real buffer;
//   - every `OpenFile` that returns a descriptor is counted against its
//     `Close`, so AVP-205 can prove zero leaks.

type fakeInfo struct {
	name string
	mode fs.FileMode
	size int64
}

func (i fakeInfo) Name() string       { return i.name }
func (i fakeInfo) Size() int64        { return i.size }
func (i fakeInfo) Mode() fs.FileMode  { return i.mode }
func (i fakeInfo) ModTime() time.Time { return time.Time{} }
func (i fakeInfo) IsDir() bool        { return i.mode.IsDir() }
func (i fakeInfo) Sys() any           { return nil }

func regular(name string, data []byte) fakeInfo {
	return fakeInfo{name: name, mode: 0, size: int64(len(data))}
}

func sized(name string, size int64) fakeInfo {
	return fakeInfo{name: name, mode: 0, size: size}
}

func dir(name string) fakeInfo {
	return fakeInfo{name: name, mode: fs.ModeDir}
}

type readRecord struct {
	requested int
	capacity  int
	base      *byte
}

type fakeFile struct {
	name string
	data []byte

	// statInfos[i] is the FileInfo returned by the (i+1)-th Stat call; the
	// last entry repeats. statErrs is indexed identically.
	statInfos []fs.FileInfo
	statErrs  []error
	onStat    map[int]func()

	// readErrOn is the 1-based Read call that fails with readErr.
	readErrOn int
	readErr   error
	onRead    map[int]func()

	closeErr error

	offset    int
	statCalls int
	readCalls int
	closes    int
	reads     []readRecord
}

func (f *fakeFile) at(index int) (fs.FileInfo, error) {
	if index < len(f.statErrs) && f.statErrs[index] != nil {
		return nil, f.statErrs[index]
	}
	if len(f.statInfos) == 0 {
		return nil, fs.ErrInvalid
	}
	if index >= len(f.statInfos) {
		index = len(f.statInfos) - 1
	}
	return f.statInfos[index], nil
}

func (f *fakeFile) Stat() (fs.FileInfo, error) {
	f.statCalls++
	if hook, ok := f.onStat[f.statCalls]; ok {
		hook()
	}
	return f.at(f.statCalls - 1)
}

func (f *fakeFile) Read(p []byte) (int, error) {
	f.readCalls++
	record := readRecord{requested: len(p), capacity: cap(p)}
	if len(p) > 0 {
		record.base = &p[0]
	}
	f.reads = append(f.reads, record)
	if hook, ok := f.onRead[f.readCalls]; ok {
		hook()
	}
	if f.readErr != nil && f.readCalls == f.readErrOn {
		return 0, f.readErr
	}
	if f.offset >= len(f.data) {
		return 0, io.EOF
	}
	n := copy(p, f.data[f.offset:])
	f.offset += n
	return n, nil
}

func (f *fakeFile) Close() error {
	f.closes++
	return f.closeErr
}

func (f *fakeFile) bytesRead() int {
	total := 0
	for _, record := range f.reads {
		total += record.requested
	}
	return total
}

type fakeNode struct {
	info    fs.FileInfo
	lstat   error
	file    *fakeFile
	openErr error
}

type fakeRoot struct {
	nodes map[string]*fakeNode

	lstatNames []string
	opened     []string
	handedOut  []*fakeFile

	// beforeOpen fires between the leaf Lstat and the OpenFile for that
	// name — the §7.1.1 "before" hook.
	beforeOpen map[string]func(*fakeRoot)

	// sameFile is the injectable identity verdict (AVP-206). nil means
	// "identical".
	sameFile func(a, b fs.FileInfo) bool

	sameFileCalls int
}

func newFakeRoot() *fakeRoot {
	return &fakeRoot{
		nodes:      map[string]*fakeNode{},
		beforeOpen: map[string]func(*fakeRoot){},
	}
}

func (r *fakeRoot) Lstat(name string) (fs.FileInfo, error) {
	r.lstatNames = append(r.lstatNames, name)
	node, ok := r.nodes[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	if node.lstat != nil {
		return nil, node.lstat
	}
	return node.info, nil
}

func (r *fakeRoot) OpenFile(name string, flag int, perm fs.FileMode) (FileOps, error) {
	if hook, ok := r.beforeOpen[name]; ok {
		delete(r.beforeOpen, name)
		hook(r)
	}
	r.opened = append(r.opened, name)
	node, ok := r.nodes[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	if node.openErr != nil {
		return nil, node.openErr
	}
	if node.file == nil {
		return nil, fs.ErrInvalid
	}
	r.handedOut = append(r.handedOut, node.file)
	return node.file, nil
}

func (r *fakeRoot) SameFile(a, b fs.FileInfo) bool {
	r.sameFileCalls++
	if r.sameFile != nil {
		return r.sameFile(a, b)
	}
	return true
}

func (r *fakeRoot) opensOf(name string) int {
	count := 0
	for _, opened := range r.opened {
		if opened == name {
			count++
		}
	}
	return count
}

func (r *fakeRoot) lstatsOf(name string) int {
	count := 0
	for _, seen := range r.lstatNames {
		if seen == name {
			count++
		}
	}
	return count
}

func (r *fakeRoot) set(name string, info fs.FileInfo) {
	r.nodes[name] = &fakeNode{info: info}
}

func (r *fakeRoot) setFile(name string, data []byte) *fakeFile {
	info := regular(name, data)
	file := &fakeFile{name: name, data: data, statInfos: []fs.FileInfo{info}}
	r.nodes[name] = &fakeNode{info: info, file: file}
	return file
}

func (r *fakeRoot) remove(name string) {
	delete(r.nodes, name)
}

const (
	testSlug        = "feature"
	testBase        = ".tpatch/features/feature"
	testStatus      = testBase + "/status.json"
	testAnalysis    = testBase + "/analysis.md"
	testSpec        = testBase + "/spec.md"
	testExploration = testBase + "/exploration.md"
	testSidecarDir  = testBase + "/artifacts"
	testSidecar     = testBase + "/artifacts/analysis.json"
)

// fixtureRoot is a fully ready feature: valid status, three non-empty
// required Markdown artifacts and a valid JSON-object sidecar.
func fixtureRoot(t *testing.T) *fakeRoot {
	t.Helper()
	root := newFakeRoot()
	for _, name := range []string{".tpatch", ".tpatch/features", testBase, testSidecarDir} {
		root.set(name, dir(name))
	}
	root.setFile(testStatus, []byte(`{"state":"defined"}`))
	root.setFile(testAnalysis, []byte("analysis"))
	root.setFile(testSpec, []byte("spec"))
	root.setFile(testExploration, []byte("exploration"))
	root.setFile(testSidecar, []byte(`{"summary":"x"}`))
	return root
}

func scratchBuffer() []byte {
	return make([]byte, MaxArtifactBytes+1)
}

func inspectFixture(t *testing.T, root *fakeRoot) Report {
	t.Helper()
	return Inspect(root, testSlug, scratchBuffer())
}

func artifactByID(t *testing.T, report Report, id string) Artifact {
	t.Helper()
	for _, artifact := range report.Artifacts {
		if artifact.ID == id {
			return artifact
		}
	}
	t.Fatalf("report has no artifact %q", id)
	return Artifact{}
}

func advisoryCodes(report Report) []string {
	out := make([]string, 0, len(report.Advisories))
	for _, advisory := range report.Advisories {
		out = append(out, advisory.Code)
	}
	return out
}

func hasAdvisory(report Report, code string) bool {
	for _, advisory := range report.Advisories {
		if advisory.Code == code {
			return true
		}
	}
	return false
}

// assertNoLeaks proves every descriptor handed out was closed exactly once.
func assertNoLeaks(t *testing.T, root *fakeRoot) {
	t.Helper()
	for _, file := range root.handedOut {
		if file.closes != 1 {
			t.Fatalf("descriptor %q closed %d times, want exactly 1", file.name, file.closes)
		}
	}
}
