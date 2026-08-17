package intent

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"testing"
	"time"
)

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

type fakeFile struct {
	info       fs.FileInfo
	postInfo   fs.FileInfo
	data       []byte
	statErr    error
	readErr    error
	closeErr   error
	statCalls  int
	closeCalls int
}

func (f *fakeFile) Stat() (fs.FileInfo, error) {
	f.statCalls++
	if f.statErr != nil {
		return nil, f.statErr
	}
	if f.statCalls > 1 && f.postInfo != nil {
		return f.postInfo, nil
	}
	return f.info, nil
}

func (f *fakeFile) Read(p []byte) (int, error) {
	if f.readErr != nil {
		return 0, f.readErr
	}
	r := bytes.NewReader(f.data)
	n, err := r.Read(p)
	if n == len(f.data) {
		return n, io.EOF
	}
	return n, err
}

func (f *fakeFile) Close() error {
	f.closeCalls++
	return f.closeErr
}

type fakeNode struct {
	info    fs.FileInfo
	lerr    error
	file    *fakeFile
	openErr error
}

type fakeRoot struct {
	nodes      map[string]*fakeNode
	same       bool
	different  string
	openCount  int
	lstatNames []string
}

func (r *fakeRoot) Lstat(name string) (fs.FileInfo, error) {
	r.lstatNames = append(r.lstatNames, name)
	node, ok := r.nodes[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return node.info, node.lerr
}

func (r *fakeRoot) OpenFile(name string, _ int, _ fs.FileMode) (FileOps, error) {
	r.openCount++
	node, ok := r.nodes[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	if node.openErr != nil {
		return nil, node.openErr
	}
	return node.file, nil
}

func (r *fakeRoot) SameFile(a, _ fs.FileInfo) bool {
	if r.different != "" && a.Name() == r.different {
		return false
	}
	return r.same
}

func regular(name string, data []byte) fakeInfo {
	return fakeInfo{name: name, mode: 0, size: int64(len(data))}
}

func fixtureRoot(t *testing.T) *fakeRoot {
	t.Helper()
	root := &fakeRoot{nodes: make(map[string]*fakeNode), same: true}
	for _, name := range []string{".tpatch", ".tpatch/features", ".tpatch/features/feature", ".tpatch/features/feature/artifacts"} {
		root.nodes[name] = &fakeNode{info: fakeInfo{name: name, mode: fs.ModeDir}}
	}
	for _, entry := range []struct {
		name string
		data []byte
	}{
		{".tpatch/features/feature/status.json", []byte(`{"state":"defined"}`)},
		{".tpatch/features/feature/analysis.md", []byte("analysis")},
		{".tpatch/features/feature/spec.md", []byte("spec")},
		{".tpatch/features/feature/exploration.md", []byte("exploration")},
		{".tpatch/features/feature/artifacts/analysis.json", []byte(`{}`)},
	} {
		info := regular(entry.name, entry.data)
		root.nodes[entry.name] = &fakeNode{info: info, file: &fakeFile{info: info, data: entry.data}}
	}
	return root
}

func TestInspectStructuralStatesAndReadiness(t *testing.T) {
	tests := []struct {
		name          string
		change        func(*fakeRoot)
		wantState     string
		wantReason    string
		wantReadiness string
		wantExitAbort string
	}{
		{
			name:          "regular content",
			change:        func(*fakeRoot) {},
			wantState:     StatePresentNonempty,
			wantReason:    "",
			wantReadiness: readinessReady,
		},
		{
			name: "empty",
			change: func(r *fakeRoot) {
				name := ".tpatch/features/feature/spec.md"
				r.nodes[name].info = regular(name, nil)
				r.nodes[name].file = &fakeFile{info: r.nodes[name].info, data: nil}
			},
			wantState:     StatePresentEmpty,
			wantReason:    "artifact-empty",
			wantReadiness: readinessNotReady,
		},
		{
			name: "absent",
			change: func(r *fakeRoot) {
				delete(r.nodes, ".tpatch/features/feature/spec.md")
			},
			wantState:     StateAbsent,
			wantReason:    "artifact-absent",
			wantReadiness: readinessNotReady,
		},
		{
			name: "symlink",
			change: func(r *fakeRoot) {
				r.nodes[".tpatch/features/feature/spec.md"].info = fakeInfo{name: "spec.md", mode: fs.ModeSymlink}
			},
			wantState:     StateSymlinkRefused,
			wantReason:    "artifact-symlink-refused",
			wantReadiness: readinessNotReady,
		},
		{
			name: "not regular",
			change: func(r *fakeRoot) {
				r.nodes[".tpatch/features/feature/spec.md"].info = fakeInfo{name: "spec.md", mode: fs.ModeDir}
			},
			wantState:     StateNotRegular,
			wantReason:    "artifact-not-regular",
			wantReadiness: readinessNotReady,
		},
		{
			name: "oversize",
			change: func(r *fakeRoot) {
				r.nodes[".tpatch/features/feature/spec.md"].info = fakeInfo{name: "spec.md", size: MaxArtifactBytes + 1}
			},
			wantState:     StateOversize,
			wantReason:    "artifact-oversize",
			wantReadiness: readinessNotReady,
		},
		{
			name: "identity mismatch",
			change: func(r *fakeRoot) {
				r.different = ".tpatch/features/feature/spec.md"
			},
			wantState:     StateUnstable,
			wantReason:    "artifact-snapshot-unstable",
			wantReadiness: readinessIndeterminate,
		},
		{
			name: "close failure",
			change: func(r *fakeRoot) {
				r.nodes[".tpatch/features/feature/spec.md"].file.closeErr = errors.New("close")
			},
			wantState:     StateUnreadable,
			wantReason:    "artifact-unreadable",
			wantReadiness: readinessNotReady,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := fixtureRoot(t)
			test.change(root)
			report := Inspect(root, "feature", make([]byte, MaxArtifactBytes+1))
			if report.AbortCode() != test.wantExitAbort {
				t.Fatalf("abort = %q, want %q", report.AbortCode(), test.wantExitAbort)
			}
			spec := report.Artifacts[1]
			if spec.State != test.wantState || spec.ReasonCode != test.wantReason {
				t.Fatalf("spec = (%s, %s), want (%s, %s)", spec.State, spec.ReasonCode, test.wantState, test.wantReason)
			}
			if report.Readiness() != test.wantReadiness {
				t.Fatalf("readiness = %s, want %s", report.Readiness(), test.wantReadiness)
			}
			for _, node := range root.nodes {
				if node.file != nil && node.file.closeCalls > 1 {
					t.Fatalf("descriptor for %q closed %d times", node.info.Name(), node.file.closeCalls)
				}
			}
		})
	}
}

func TestInspectStatusTotalityAndSidecarJSON(t *testing.T) {
	tests := []struct {
		name      string
		change    func(*fakeRoot)
		wantAbort string
		wantSide  string
	}{
		{
			name: "status absent continues",
			change: func(r *fakeRoot) {
				delete(r.nodes, ".tpatch/features/feature/status.json")
			},
			wantSide: StatePresentNonempty,
		},
		{
			name: "status malformed aborts",
			change: func(r *fakeRoot) {
				name := ".tpatch/features/feature/status.json"
				r.nodes[name].info = regular(name, []byte("{"))
				r.nodes[name].file = &fakeFile{info: r.nodes[name].info, data: []byte("{")}
			},
			wantAbort: abortStatusMalformed,
		},
		{
			name: "status unknown state aborts",
			change: func(r *fakeRoot) {
				name := ".tpatch/features/feature/status.json"
				data := []byte(`{"state":"prepared"}`)
				r.nodes[name].info = regular(name, data)
				r.nodes[name].file = &fakeFile{info: r.nodes[name].info, data: data}
			},
			wantAbort: abortStatusInvalidState,
		},
		{
			name: "status close failure aborts",
			change: func(r *fakeRoot) {
				r.nodes[".tpatch/features/feature/status.json"].file.closeErr = errors.New("close")
			},
			wantAbort: abortStatusUnreadable,
		},
		{
			name: "invalid sidecar JSON",
			change: func(r *fakeRoot) {
				name := ".tpatch/features/feature/artifacts/analysis.json"
				data := []byte("{")
				r.nodes[name].info = regular(name, data)
				r.nodes[name].file = &fakeFile{info: r.nodes[name].info, data: data}
			},
			wantSide: StateInvalidStructured,
		},
		{
			name: "sidecar non object",
			change: func(r *fakeRoot) {
				name := ".tpatch/features/feature/artifacts/analysis.json"
				data := []byte("[]")
				r.nodes[name].info = regular(name, data)
				r.nodes[name].file = &fakeFile{info: r.nodes[name].info, data: data}
			},
			wantSide: StateInvalidStructured,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := fixtureRoot(t)
			test.change(root)
			report := Inspect(root, "feature", make([]byte, MaxArtifactBytes+1))
			if report.AbortCode() != test.wantAbort {
				t.Fatalf("abort = %q, want %q", report.AbortCode(), test.wantAbort)
			}
			if test.wantAbort != "" {
				if len(report.Artifacts) != 0 {
					t.Fatalf("abort emitted %d artifacts", len(report.Artifacts))
				}
				return
			}
			if got := report.Artifacts[3].State; got != test.wantSide {
				t.Fatalf("sidecar state = %q, want %q", got, test.wantSide)
			}
		})
	}
}

func TestCanonicalSlugAndReportPrivacy(t *testing.T) {
	for _, raw := range []string{"ok", "a-1", "a23456789012345678901234567890123456789012345678901234567890"} {
		got, err := CanonicalSlug(raw)
		if err != nil || got != raw {
			t.Fatalf("CanonicalSlug(%q) = (%q, %v)", raw, got, err)
		}
	}
	for _, raw := range []string{"", "Upper", "two--dashes", "-start", "end-", "../../etc", "CON", "a\nb"} {
		if _, err := CanonicalSlug(raw); err == nil {
			t.Fatalf("CanonicalSlug(%q) unexpectedly accepted", raw)
		}
	}
	report := NewAbortReport("", abortSlugUnsafe)
	var output bytes.Buffer
	report.WriteHuman(&output)
	if bytes.Contains(output.Bytes(), []byte("../../etc")) || report.Slug != "" {
		t.Fatalf("unsafe slug leaked: %q", output.String())
	}
}
