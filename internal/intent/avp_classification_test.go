package intent

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"
)

func runInspect(t *testing.T, mutate func(*fakeRoot)) (Report, *fakeRoot) {
	t.Helper()
	root := fixtureRoot(t)
	if mutate != nil {
		mutate(root)
	}
	report := Inspect(root, testSlug, scratchBuffer())
	assertNoLeaks(t, root)
	return report, root
}

// TestAVPStructuralClassification covers matrix category B — the total
// per-artifact classification ladder (AVP-011…AVP-030).
func TestAVPStructuralClassification(t *testing.T) {
	cases := []struct {
		id         string
		target     string
		mutate     func(*fakeRoot)
		wantState  string
		wantReason string
		// zeroOpens asserts the classification was decided before any open.
		zeroOpens bool
	}{
		{
			id: "AVP-011", target: "analysis",
			mutate:    func(r *fakeRoot) { r.setFile(testAnalysis, []byte("ordinary content\n")) },
			wantState: StatePresentNonempty, wantReason: "",
		},
		{
			id: "AVP-012", target: "spec",
			mutate:    func(r *fakeRoot) { r.setFile(testSpec, nil) },
			wantState: StatePresentEmpty, wantReason: ReasonArtifactEmpty,
		},
		{
			id: "AVP-013", target: "spec",
			mutate:    func(r *fakeRoot) { r.setFile(testSpec, []byte(" \t\n\r\n")) },
			wantState: StatePresentEmpty, wantReason: ReasonArtifactEmpty,
		},
		{
			id: "AVP-014", target: "spec",
			mutate:    func(r *fakeRoot) { r.setFile(testSpec, []byte("x")) },
			wantState: StatePresentNonempty, wantReason: "",
		},
		{
			id: "AVP-015", target: "analysis",
			mutate:    func(r *fakeRoot) { r.remove(testAnalysis) },
			wantState: StateAbsent, wantReason: ReasonArtifactAbsent, zeroOpens: true,
		},
		{
			id: "AVP-016", target: "spec",
			mutate: func(r *fakeRoot) {
				r.set(testSpec, fakeInfo{name: "spec.md", mode: fs.ModeSymlink, size: 12})
			},
			wantState: StateSymlinkRefused, wantReason: ReasonArtifactSymlinkRefused, zeroOpens: true,
		},
		{
			id: "AVP-017", target: "spec",
			mutate: func(r *fakeRoot) {
				r.set(testSpec, fakeInfo{name: "spec.md", mode: fs.ModeSymlink, size: 12})
			},
			wantState: StateSymlinkRefused, wantReason: ReasonArtifactSymlinkRefused, zeroOpens: true,
		},
		{
			id: "AVP-018", target: "spec",
			mutate: func(r *fakeRoot) {
				// A dangling link still stats as a link through Lstat.
				r.set(testSpec, fakeInfo{name: "spec.md", mode: fs.ModeSymlink})
			},
			wantState: StateSymlinkRefused, wantReason: ReasonArtifactSymlinkRefused, zeroOpens: true,
		},
		{
			id: "AVP-019", target: "analysis_sidecar",
			mutate: func(r *fakeRoot) {
				r.set(testSidecarDir, fakeInfo{name: "artifacts", mode: fs.ModeSymlink | fs.ModeDir})
			},
			wantState: StateSymlinkRefused, wantReason: ReasonArtifactSymlinkRefused, zeroOpens: true,
		},
		{
			id: "AVP-020", target: "spec",
			mutate:    func(r *fakeRoot) { r.set(testSpec, dir("spec.md")) },
			wantState: StateNotRegular, wantReason: ReasonArtifactNotRegular, zeroOpens: true,
		},
		{
			id: "AVP-021", target: "exploration",
			mutate: func(r *fakeRoot) {
				r.set(testExploration, fakeInfo{name: "exploration.md", mode: fs.ModeNamedPipe})
			},
			wantState: StateNotRegular, wantReason: ReasonArtifactNotRegular, zeroOpens: true,
		},
		{
			id: "AVP-022", target: "spec",
			mutate: func(r *fakeRoot) {
				r.nodes[testSpec].openErr = fs.ErrPermission
			},
			wantState: StateUnreadable, wantReason: ReasonArtifactUnreadable,
		},
		{
			id: "AVP-023", target: "spec",
			mutate: func(r *fakeRoot) {
				data := make([]byte, MaxArtifactBytes)
				for i := range data {
					data[i] = 'a'
				}
				r.setFile(testSpec, data)
			},
			wantState: StatePresentNonempty, wantReason: "",
		},
		{
			id: "AVP-024", target: "spec",
			mutate:    func(r *fakeRoot) { r.set(testSpec, sized("spec.md", MaxArtifactBytes+1)) },
			wantState: StateOversize, wantReason: ReasonArtifactOversize, zeroOpens: true,
		},
		{
			id: "AVP-025", target: "analysis_sidecar",
			mutate:    func(r *fakeRoot) { r.setFile(testSidecar, []byte(`{"summary":"x"}`)) },
			wantState: StatePresentNonempty, wantReason: "",
		},
		{
			id: "AVP-026", target: "analysis_sidecar",
			mutate:    func(r *fakeRoot) { r.setFile(testSidecar, []byte("{")) },
			wantState: StateInvalidStructured, wantReason: ReasonSidecarNotJSON,
		},
		{
			id: "AVP-027", target: "analysis_sidecar",
			mutate:    func(r *fakeRoot) { r.setFile(testSidecar, []byte("[1,2,3]")) },
			wantState: StateInvalidStructured, wantReason: ReasonSidecarNotJSONObject,
		},
		{
			id: "AVP-028", target: "analysis_sidecar",
			mutate:    func(r *fakeRoot) { r.setFile(testSidecar, []byte(`"a string"`)) },
			wantState: StateInvalidStructured, wantReason: ReasonSidecarNotJSONObject,
		},
		{
			id: "AVP-029", target: "analysis_sidecar",
			mutate:    func(r *fakeRoot) { r.setFile(testSidecar, []byte(`{"unknown_future_field":1}`)) },
			wantState: StatePresentNonempty, wantReason: "",
		},
		{
			id: "AVP-030", target: "analysis_sidecar",
			mutate:    func(r *fakeRoot) { r.setFile(testSidecar, []byte("   \n\t ")) },
			wantState: StatePresentEmpty, wantReason: ReasonArtifactEmpty,
		},
	}
	specNames := map[string]string{
		"analysis":         testAnalysis,
		"spec":             testSpec,
		"exploration":      testExploration,
		"analysis_sidecar": testSidecar,
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			deadline := time.Now().Add(20 * time.Second)
			report, root := runInspect(t, tc.mutate)
			if time.Now().After(deadline) {
				t.Fatalf("%s did not complete under its hard deadline", tc.id)
			}
			if report.Abort != nil {
				t.Fatalf("unexpected abort %q", report.Abort.Code)
			}
			artifact := artifactByID(t, report, tc.target)
			if artifact.State != tc.wantState {
				t.Fatalf("state = %q, want %q", artifact.State, tc.wantState)
			}
			if artifact.ReasonCode != tc.wantReason {
				t.Fatalf("reason_code = %q, want %q", artifact.ReasonCode, tc.wantReason)
			}
			if tc.zeroOpens {
				if got := root.opensOf(specNames[tc.target]); got != 0 {
					t.Fatalf("OpenFile called %d times for %s; the ladder must decide before the open", got, tc.target)
				}
				if tc.wantState == StateOversize {
					if file := root.nodes[specNames[tc.target]]; file != nil && file.file != nil && file.file.bytesRead() != 0 {
						t.Fatal("oversize artifact was read")
					}
				}
			}
		})
	}

	t.Run("AVP-023-read-is-cap-plus-one-bounded", func(t *testing.T) {
		data := make([]byte, MaxArtifactBytes)
		for i := range data {
			data[i] = 'a'
		}
		var file *fakeFile
		report, _ := runInspect(t, func(r *fakeRoot) { file = r.setFile(testSpec, data) })
		if got := artifactByID(t, report, "spec").State; got != StatePresentNonempty {
			t.Fatalf("exactly-cap artifact = %q, want present-nonempty", got)
		}
		if file.reads[0].requested != MaxArtifactBytes+1 {
			t.Fatalf("first read requested %d bytes, want %d", file.reads[0].requested, MaxArtifactBytes+1)
		}
	})
}

// TestAVPSnapshotAndCaptureRaces covers J (AVP-083…AVP-085) and O
// (AVP-107…AVP-115) — the injected capture rows.
func TestAVPSnapshotAndCaptureRaces(t *testing.T) {
	t.Run("AVP-083", func(t *testing.T) {
		// Deleted between the leaf Lstat and the OpenFile: unstable, not absent.
		report, root := runInspect(t, func(r *fakeRoot) {
			r.beforeOpen[testSpec] = func(root *fakeRoot) { root.remove(testSpec) }
		})
		artifact := artifactByID(t, report, "spec")
		if artifact.State != StateUnstable || artifact.ReasonCode != ReasonArtifactUnstable {
			t.Fatalf("spec = (%q, %q), want unstable/artifact-snapshot-unstable", artifact.State, artifact.ReasonCode)
		}
		if root.opensOf(testSpec) != 1 {
			t.Fatalf("expected exactly one open attempt, got %d", root.opensOf(testSpec))
		}
	})

	t.Run("AVP-084", func(t *testing.T) {
		// Row 13: a different inode between Lstat and fstat.
		var file *fakeFile
		report, root := runInspect(t, func(r *fakeRoot) {
			file = r.nodes[testSpec].file
			r.sameFile = func(a, b fs.FileInfo) bool { return a.Name() != testSpec }
		})
		if got := artifactByID(t, report, "spec").State; got != StateUnstable {
			t.Fatalf("spec = %q, want unstable", got)
		}
		if file.bytesRead() != 0 {
			t.Fatal("row 13 must reject before any byte is read")
		}
		if root.sameFileCalls == 0 {
			t.Fatal("identity was not routed through the RootOps.SameFile seam")
		}
	})

	t.Run("AVP-085", func(t *testing.T) {
		// Row 18: truncated to zero after fstat, before the read.
		report, _ := runInspect(t, func(r *fakeRoot) {
			info := regular(testSpec, []byte("0123456789"))
			file := &fakeFile{name: testSpec, data: nil, statInfos: []fs.FileInfo{info}}
			r.nodes[testSpec] = &fakeNode{info: info, file: file}
		})
		artifact := artifactByID(t, report, "spec")
		if artifact.State != StateUnstable {
			t.Fatalf("spec = %q, want unstable (never present-empty)", artifact.State)
		}
	})

	t.Run("AVP-107", func(t *testing.T) {
		// Row 14: the descriptor is a FIFO although Lstat said regular.
		deadline := time.Now().Add(20 * time.Second)
		report, _ := runInspect(t, func(r *fakeRoot) {
			pre := regular(testSpec, []byte("spec"))
			post := fakeInfo{name: "spec.md", mode: fs.ModeNamedPipe}
			r.nodes[testSpec] = &fakeNode{info: pre, file: &fakeFile{
				name: testSpec, data: []byte("spec"), statInfos: []fs.FileInfo{post},
			}}
		})
		if time.Now().After(deadline) {
			t.Fatal("raced-FIFO capture did not return under its hard deadline")
		}
		if got := artifactByID(t, report, "spec").State; got != StateUnstable {
			t.Fatalf("spec = %q, want unstable", got)
		}
	})

	t.Run("AVP-108", func(t *testing.T) {
		// Row 13 again, this time the substituted object is an in-root
		// symlink to a different file: unstable, never symlink-refused.
		var substituted *fakeFile
		report, _ := runInspect(t, func(r *fakeRoot) {
			r.beforeOpen[testSpec] = func(root *fakeRoot) {
				substituted = root.setFile(testSpec, []byte("other file contents"))
				root.nodes[testSpec].info = fakeInfo{name: "decoy", mode: 0, size: 19}
			}
			r.sameFile = func(a, b fs.FileInfo) bool { return a.Name() != "decoy" }
		})
		artifact := artifactByID(t, report, "spec")
		if artifact.State != StateUnstable {
			t.Fatalf("spec = %q, want unstable (not symlink-refused)", artifact.State)
		}
		if substituted != nil && substituted.bytesRead() != 0 {
			t.Fatal("bytes were read from the substituted object")
		}
		if strings.Contains(artifact.Remediation, "decoy") {
			t.Fatal("substituted object was named in output")
		}
	})

	t.Run("AVP-109", func(t *testing.T) {
		// Row 12: injected fstat failure on the descriptor.
		report, _ := runInspect(t, func(r *fakeRoot) {
			info := regular(testSpec, []byte("spec"))
			r.nodes[testSpec] = &fakeNode{info: info, file: &fakeFile{
				name: testSpec, data: []byte("spec"),
				statInfos: []fs.FileInfo{info},
				statErrs:  []error{errors.New("fstat failed")},
			}}
		})
		artifact := artifactByID(t, report, "spec")
		if artifact.State != StateUnreadable || artifact.ReasonCode != ReasonArtifactUnreadable {
			t.Fatalf("spec = (%q, %q), want unreadable", artifact.State, artifact.ReasonCode)
		}
	})

	t.Run("AVP-110", func(t *testing.T) {
		report, _ := runInspect(t, func(r *fakeRoot) {
			pre := regular(testSpec, []byte("spec"))
			post := fakeInfo{name: "spec.md", mode: fs.ModeDevice | fs.ModeCharDevice}
			r.nodes[testSpec] = &fakeNode{info: pre, file: &fakeFile{
				name: testSpec, data: []byte("spec"), statInfos: []fs.FileInfo{post},
			}}
		})
		if got := artifactByID(t, report, "spec").State; got != StateUnstable {
			t.Fatalf("spec = %q, want unstable", got)
		}
	})

	t.Run("AVP-111", func(t *testing.T) {
		report, _ := runInspect(t, func(r *fakeRoot) {
			pre := regular(testSpec, []byte("spec"))
			post := sized("spec.md", 9999)
			r.nodes[testSpec] = &fakeNode{info: pre, file: &fakeFile{
				name: testSpec, data: []byte("spec"), statInfos: []fs.FileInfo{post},
			}}
		})
		if got := artifactByID(t, report, "spec").State; got != StateUnstable {
			t.Fatalf("spec = %q, want unstable", got)
		}
	})

	t.Run("AVP-112", func(t *testing.T) {
		// Row 17: grows past the cap during the read. io.ReadFull returns
		// err == nil with n == Max+1 and the classification is unstable,
		// never oversize.
		grown := make([]byte, MaxArtifactBytes+4096)
		for i := range grown {
			grown[i] = 'g'
		}
		var file *fakeFile
		report, _ := runInspect(t, func(r *fakeRoot) {
			pre := sized(testSpec, 16)
			file = &fakeFile{name: testSpec, data: grown, statInfos: []fs.FileInfo{sized(testSpec, 16)}}
			r.nodes[testSpec] = &fakeNode{info: pre, file: file}
		})
		artifact := artifactByID(t, report, "spec")
		if artifact.State != StateUnstable {
			t.Fatalf("spec = %q, want unstable (never oversize)", artifact.State)
		}
		if total := file.bytesRead(); total > MaxArtifactBytes+1 {
			t.Fatalf("requested %d bytes, ceiling is %d", total, MaxArtifactBytes+1)
		}
		if file.offset != MaxArtifactBytes+1 {
			t.Fatalf("consumed %d bytes, want exactly %d", file.offset, MaxArtifactBytes+1)
		}
	})

	t.Run("AVP-113", func(t *testing.T) {
		// Row 18: grows within the cap during the read.
		report, _ := runInspect(t, func(r *fakeRoot) {
			pre := sized(testSpec, 4)
			file := &fakeFile{name: testSpec, data: []byte("much longer than four"), statInfos: []fs.FileInfo{sized(testSpec, 4)}}
			r.nodes[testSpec] = &fakeNode{info: pre, file: file}
		})
		if got := artifactByID(t, report, "spec").State; got != StateUnstable {
			t.Fatalf("spec = %q, want unstable", got)
		}
	})

	t.Run("AVP-114", func(t *testing.T) {
		// Row 20: the post-read fstat reports a different size.
		report, _ := runInspect(t, func(r *fakeRoot) {
			data := []byte("spec")
			info := regular(testSpec, data)
			file := &fakeFile{
				name: testSpec, data: data,
				statInfos: []fs.FileInfo{info, sized(testSpec, 77)},
			}
			r.nodes[testSpec] = &fakeNode{info: info, file: file}
		})
		if got := artifactByID(t, report, "spec").State; got != StateUnstable {
			t.Fatalf("spec = %q, want unstable", got)
		}
	})

	t.Run("AVP-115", func(t *testing.T) {
		// Row 19: the post-read fstat itself fails.
		report, _ := runInspect(t, func(r *fakeRoot) {
			data := []byte("spec")
			info := regular(testSpec, data)
			file := &fakeFile{
				name: testSpec, data: data,
				statInfos: []fs.FileInfo{info, info},
				statErrs:  []error{nil, errors.New("post-read fstat failed")},
			}
			r.nodes[testSpec] = &fakeNode{info: info, file: file}
		})
		if got := artifactByID(t, report, "spec").State; got != StateUnreadable {
			t.Fatalf("spec = %q, want unreadable", got)
		}
	})

	t.Run("AVP-117", func(t *testing.T) {
		// The eight instability probes applied to the sidecar leave the
		// three Markdown artifacts ready.
		probes := []struct {
			name  string
			apply func(*fakeRoot)
		}{
			{"pre-open-vanish", func(r *fakeRoot) {
				r.beforeOpen[testSidecar] = func(root *fakeRoot) { root.remove(testSidecar) }
			}},
			{"identity", func(r *fakeRoot) {
				r.sameFile = func(a, b fs.FileInfo) bool { return a.Name() != testSidecar }
			}},
			{"descriptor-kind", func(r *fakeRoot) {
				r.nodes[testSidecar].file.statInfos = []fs.FileInfo{fakeInfo{name: testSidecar, mode: fs.ModeNamedPipe}}
			}},
			{"fstat-size", func(r *fakeRoot) {
				r.nodes[testSidecar].file.statInfos = []fs.FileInfo{sized(testSidecar, 4096)}
			}},
			{"grow-past-cap", func(r *fakeRoot) {
				r.nodes[testSidecar].file.data = make([]byte, MaxArtifactBytes+2)
			}},
			{"byte-count", func(r *fakeRoot) {
				r.nodes[testSidecar].file.data = []byte(`{"summary":"xxxxxxxxxxxxxx"}`)
			}},
			{"post-read-size", func(r *fakeRoot) {
				file := r.nodes[testSidecar].file
				file.statInfos = []fs.FileInfo{file.statInfos[0], sized(testSidecar, 99)}
			}},
			{"post-walk", func(r *fakeRoot) {
				file := r.nodes[testSidecar].file
				file.onStat = map[int]func(){2: func() { r.remove(testSidecarDir) }}
			}},
		}
		for _, probe := range probes {
			t.Run(probe.name, func(t *testing.T) {
				report, _ := runInspect(t, probe.apply)
				sidecar := artifactByID(t, report, "analysis_sidecar")
				if sidecar.State != StateUnstable || sidecar.ReasonCode != ReasonArtifactUnstable {
					t.Fatalf("sidecar = (%q, %q), want unstable", sidecar.State, sidecar.ReasonCode)
				}
				if !hasAdvisory(report, AdvisorySidecarUnstable) {
					t.Fatalf("advisories = %v, want analysis-sidecar-unstable", advisoryCodes(report))
				}
				if report.Readiness() != ReadinessReady {
					t.Fatalf("readiness = %q, want ready — the sidecar never gates readiness", report.Readiness())
				}
			})
		}
	})
}

// TestAVPBoundedReads covers W — AVP-170, AVP-171, AVP-173 and AVP-174.
func TestAVPBoundedReads(t *testing.T) {
	t.Run("AVP-171", func(t *testing.T) {
		sizes := map[string][]byte{
			"one-byte":     []byte("x"),
			"cap-minus-1":  make([]byte, MaxArtifactBytes-1),
			"cap":          make([]byte, MaxArtifactBytes),
			"beyond-cap":   make([]byte, MaxArtifactBytes+8),
			"empty":        nil,
			"whitespace":   []byte(" \n\t"),
			"json-payload": []byte(`{"a":1}`),
		}
		for name, data := range sizes {
			t.Run(name, func(t *testing.T) {
				var file *fakeFile
				runInspect(t, func(r *fakeRoot) {
					info := sized(testSpec, int64(min(len(data), MaxArtifactBytes)))
					file = &fakeFile{name: testSpec, data: data, statInfos: []fs.FileInfo{info}}
					r.nodes[testSpec] = &fakeNode{info: info, file: file}
				})
				for i, record := range file.reads {
					if record.requested > MaxArtifactBytes+1 {
						t.Fatalf("read %d requested %d bytes, ceiling is %d", i, record.requested, MaxArtifactBytes+1)
					}
				}
				if file.offset > MaxArtifactBytes+1 {
					t.Fatalf("consumed %d bytes, ceiling is %d", file.offset, MaxArtifactBytes+1)
				}
			})
		}
	})

	t.Run("AVP-173", func(t *testing.T) {
		// The EOF taxonomy, asserted on the classification each arm yields.
		cases := []struct {
			name      string
			data      []byte
			declared  int64
			wantState string
		}{
			{"zero-byte-EOF", nil, 0, StatePresentEmpty},
			{"short-unexpected-EOF", []byte("short"), 5, StatePresentNonempty},
			{"exactly-cap", filledBytes(MaxArtifactBytes), MaxArtifactBytes, StatePresentNonempty},
			{"beyond-cap-nil-error", filledBytes(MaxArtifactBytes + 1), MaxArtifactBytes, StateUnstable},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				var file *fakeFile
				report, _ := runInspect(t, func(r *fakeRoot) {
					info := sized(testSpec, tc.declared)
					file = &fakeFile{name: testSpec, data: tc.data, statInfos: []fs.FileInfo{info}}
					r.nodes[testSpec] = &fakeNode{info: info, file: file}
				})
				if got := artifactByID(t, report, "spec").State; got != tc.wantState {
					t.Fatalf("state = %q, want %q", got, tc.wantState)
				}
				if file.reads[0].requested != MaxArtifactBytes+1 {
					t.Fatalf("first read requested %d, want %d", file.reads[0].requested, MaxArtifactBytes+1)
				}
			})
		}
	})

	t.Run("AVP-174", func(t *testing.T) {
		// The same four assertions for the status capture, against its own
		// distinct cap.
		if MaxStatusBytes >= MaxArtifactBytes {
			t.Fatal("the two caps must be distinct and ordered")
		}
		cases := []struct {
			name      string
			data      []byte
			declared  int64
			wantAbort AbortCode
		}{
			{"zero-byte-EOF", nil, 0, AbortStatusMalformed},
			{"short-unexpected-EOF", []byte(`{"state":"defined"}`), 19, ""},
			{"exactly-cap", statusPaddedTo(MaxStatusBytes), MaxStatusBytes, ""},
			{"beyond-cap-nil-error", statusPaddedTo(MaxStatusBytes + 1), MaxStatusBytes, AbortStatusUnstable},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				var file *fakeFile
				report, _ := runInspect(t, func(r *fakeRoot) {
					info := sized(testStatus, tc.declared)
					file = &fakeFile{name: testStatus, data: tc.data, statInfos: []fs.FileInfo{info}}
					r.nodes[testStatus] = &fakeNode{info: info, file: file}
				})
				if report.AbortCode() != tc.wantAbort {
					t.Fatalf("abort = %q, want %q", report.AbortCode(), tc.wantAbort)
				}
				if file.reads[0].requested != MaxStatusBytes+1 {
					t.Fatalf("status read requested %d, want %d", file.reads[0].requested, MaxStatusBytes+1)
				}
			})
		}
	})
}

// statusPaddedTo builds a valid status document padded with whitespace to
// exactly n bytes.
func statusPaddedTo(n int) []byte {
	document := []byte(`{"state":"defined"}`)
	if n <= len(document) {
		return document
	}
	padded := make([]byte, 0, n)
	padded = append(padded, document[:len(document)-1]...)
	for len(padded) < n-1 {
		padded = append(padded, ' ')
	}
	return append(padded, '}')
}

func filledBytes(n int) []byte {
	data := make([]byte, n)
	for i := range data {
		data[i] = 'a'
	}
	return data
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
