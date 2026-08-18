package intent

import (
	"errors"
	"io/fs"
	"runtime"
	"strings"
	"testing"
)

// TestAVPAncestorPolicy covers category T's unit rows — the component walk,
// its refusal predicate and the ordering it must obey.
func TestAVPAncestorPolicy(t *testing.T) {
	components := []struct {
		name      string
		path      string
		wantAbort AbortCode
	}{
		{"tpatch", ".tpatch", AbortFeatureUnsafe},
		{"features", ".tpatch/features", AbortFeatureUnsafe},
		{"slug", testBase, AbortFeatureUnsafe},
		{"artifacts", testSidecarDir, ""},
	}

	t.Run("AVP-145", func(t *testing.T) {
		for _, component := range components {
			t.Run(component.name, func(t *testing.T) {
				report, root := runInspect(t, func(r *fakeRoot) {
					r.set(component.path, fakeInfo{name: component.path, mode: fs.ModeSymlink | fs.ModeDir})
				})
				assertComponentRefusal(t, report, root, component.wantAbort)
			})
		}
	})

	t.Run("AVP-146", func(t *testing.T) {
		// The same four fixtures using a reparse-point-shaped FileInfo. The
		// predicate is ModeSymlink|ModeIrregular, so a junction — which has
		// no ModeSymlink bit — must be refused identically.
		for _, component := range components {
			t.Run(component.name, func(t *testing.T) {
				junction := fakeInfo{name: component.path, mode: fs.ModeIrregular}
				if junction.Mode()&fs.ModeSymlink != 0 {
					t.Fatal("the junction fixture must not carry ModeSymlink")
				}
				report, root := runInspect(t, func(r *fakeRoot) { r.set(component.path, junction) })
				assertComponentRefusal(t, report, root, component.wantAbort)
			})
		}
	})

	t.Run("AVP-147", func(t *testing.T) {
		report, root := runInspect(t, func(r *fakeRoot) {
			r.set(testBase, regular(testBase, []byte("not a directory")))
		})
		if report.AbortCode() != AbortFeatureUnsafe {
			t.Fatalf("abort = %q, want %q", report.AbortCode(), AbortFeatureUnsafe)
		}
		for _, name := range []string{testStatus, testAnalysis, testSpec, testExploration, testSidecar} {
			if root.lstatsOf(name) != 0 {
				t.Fatalf("%q was stat'd after the ancestor refusal", name)
			}
		}
	})

	t.Run("AVP-148", func(t *testing.T) {
		// A raced in-root ancestor symlink that resolves the leaf to a
		// different object is caught by the row-13 identity probe, before
		// any byte is read, and the substituted directory is never named.
		var substituted *fakeFile
		report, _ := runInspect(t, func(r *fakeRoot) {
			r.beforeOpen[testSidecar] = func(root *fakeRoot) {
				root.set(testSidecarDir, fakeInfo{name: "decoy-dir", mode: fs.ModeSymlink | fs.ModeDir})
				substituted = root.setFile(testSidecar, []byte(`{"decoy":true}`))
			}
			r.sameFile = func(a, b fs.FileInfo) bool { return a.Name() != testSidecar }
		})
		sidecar := artifactByID(t, report, "analysis_sidecar")
		if sidecar.State != StateUnstable {
			t.Fatalf("sidecar = %q, want unstable", sidecar.State)
		}
		if substituted != nil && substituted.bytesRead() != 0 {
			t.Fatal("bytes were read through the substituted ancestor")
		}
		if reportMentions(report, "decoy-dir") {
			t.Fatal("the substituted directory was named in output")
		}
	})

	t.Run("AVP-149", func(t *testing.T) {
		for _, form := range []string{"relative", "absolute"} {
			t.Run(form, func(t *testing.T) {
				var file *fakeFile
				report, root := runInspect(t, func(r *fakeRoot) {
					file = r.nodes[testSidecar].file
					// Root refuses to resolve an escaping symlink: the open
					// fails and nothing outside the root is ever touched.
					r.nodes[testSidecar].openErr = fs.ErrPermission
				})
				sidecar := artifactByID(t, report, "analysis_sidecar")
				if sidecar.State != StateUnreadable {
					t.Fatalf("sidecar = %q, want unreadable", sidecar.State)
				}
				if file.bytesRead() != 0 {
					t.Fatal("bytes were read from an escaping target")
				}
				for _, name := range append(append([]string{}, root.lstatNames...), root.opened...) {
					if strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
						t.Fatalf("a name outside the root was used: %q", name)
					}
				}
			})
		}
	})

	t.Run("AVP-151", func(t *testing.T) {
		// Raced leaf alias with the SAME identity: the capture proceeds and
		// reports the object it actually read. Nothing claims detection.
		report, _ := runInspect(t, func(r *fakeRoot) {
			// The alias resolves to the same inode, so the object served
			// by the open is byte-identical in size to the one Lstat saw.
			r.beforeOpen[testSpec] = func(root *fakeRoot) {
				root.setFile(testSpec, []byte("alia"))
			}
		})
		spec := artifactByID(t, report, "spec")
		if spec.State != StatePresentNonempty {
			t.Fatalf("spec = %q, want present-nonempty — the alias is not detectable", spec.State)
		}
		for _, claim := range []string{"alias", "refused the link", "detected"} {
			if reportMentions(report, claim) {
				t.Fatalf("output claims %q, which the design cannot deliver", claim)
			}
		}
	})

	t.Run("AVP-144-names-are-fs-ValidPath", func(t *testing.T) {
		roots := []*fakeRoot{}
		_, ready := runInspect(t, nil)
		roots = append(roots, ready)
		_, unsafeDir := runInspect(t, func(r *fakeRoot) {
			r.set(testBase, fakeInfo{name: testBase, mode: fs.ModeSymlink | fs.ModeDir})
		})
		roots = append(roots, unsafeDir)
		_, missing := runInspect(t, func(r *fakeRoot) { r.remove(testBase) })
		roots = append(roots, missing)
		for _, root := range roots {
			for _, name := range append(append([]string{}, root.lstatNames...), root.opened...) {
				if !fs.ValidPath(name) {
					t.Fatalf("%q is not a canonical fs.ValidPath name", name)
				}
				if strings.HasPrefix(name, "/") || strings.Contains(name, `\`) {
					t.Fatalf("%q is not a relative slash-separated name", name)
				}
			}
		}
	})

	t.Run("AVP-182-walk-order", func(t *testing.T) {
		// The component AFTER the offending one is never passed to Lstat.
		for _, tc := range []struct {
			name      string
			offender  string
			following []string
			wantAbort AbortCode
		}{
			{"tpatch", ".tpatch", []string{".tpatch/features", testBase}, AbortFeatureUnsafe},
			{"features", ".tpatch/features", []string{testBase}, AbortFeatureUnsafe},
			{"slug", testBase, nil, AbortFeatureUnsafe},
			{"missing-slug", testBase, nil, AbortFeatureNotFound},
		} {
			t.Run(tc.name, func(t *testing.T) {
				report, root := runInspect(t, func(r *fakeRoot) {
					if tc.wantAbort == AbortFeatureNotFound {
						r.remove(tc.offender)
						return
					}
					r.set(tc.offender, fakeInfo{name: tc.offender, mode: fs.ModeSymlink | fs.ModeDir})
				})
				if report.AbortCode() != tc.wantAbort {
					t.Fatalf("abort = %q, want %q", report.AbortCode(), tc.wantAbort)
				}
				for _, following := range tc.following {
					if root.lstatsOf(following) != 0 {
						t.Fatalf("%q was stat'd after %q was refused", following, tc.offender)
					}
				}
			})
		}
	})
}

func assertComponentRefusal(t *testing.T, report Report, root *fakeRoot, wantAbort AbortCode) {
	t.Helper()
	if wantAbort != "" {
		if report.AbortCode() != wantAbort {
			t.Fatalf("abort = %q, want %q", report.AbortCode(), wantAbort)
		}
		if len(root.opened) != 0 {
			t.Fatalf("OpenFile was called %d times on a refused walk", len(root.opened))
		}
		return
	}
	sidecar := artifactByID(t, report, "analysis_sidecar")
	if sidecar.State != StateSymlinkRefused {
		t.Fatalf("sidecar = %q, want symlink-refused", sidecar.State)
	}
	if root.opensOf(testSidecar) != 0 {
		t.Fatal("a refused sidecar component was opened")
	}
}

func reportMentions(report Report, needle string) bool {
	var builder strings.Builder
	builder.WriteString(report.Slug)
	builder.WriteString(report.FeatureState)
	builder.WriteString(report.Disclaimer)
	for _, artifact := range report.Artifacts {
		builder.WriteString(artifact.Path)
		builder.WriteString(artifact.ReasonCode)
		builder.WriteString(artifact.Remediation)
	}
	for _, advisory := range report.Advisories {
		builder.WriteString(advisory.Code)
		builder.WriteString(advisory.Message)
	}
	if report.Abort != nil {
		builder.WriteString(string(report.Abort.Code))
		builder.WriteString(report.Abort.Message)
	}
	builder.WriteString(report.ExitMessage())
	return strings.Contains(builder.String(), needle)
}

// TestAVPRootedBoundaryHonesty covers Z's unit rows — AVP-190, AVP-195,
// AVP-196, AVP-197, AVP-203, AVP-205 and the behavioral half of AVP-206.
func TestAVPRootedBoundaryHonesty(t *testing.T) {
	t.Run("AVP-190", func(t *testing.T) {
		t.Run("directory-shaped-mount-point", func(t *testing.T) {
			report, root := runInspect(t, func(r *fakeRoot) {
				r.set(testSpec, dir(testSpec))
			})
			if got := artifactByID(t, report, "spec").State; got != StateNotRegular {
				t.Fatalf("spec = %q, want not-regular", got)
			}
			if root.opensOf(testSpec) != 0 {
				t.Fatal("a mount-point-shaped leaf was opened")
			}
		})
		t.Run("proc-shaped-zero-size-stream", func(t *testing.T) {
			var file *fakeFile
			report, _ := runInspect(t, func(r *fakeRoot) {
				info := sized(testSpec, 0)
				file = &fakeFile{
					name: testSpec, data: filledBytes(MaxArtifactBytes + 4096),
					statInfos: []fs.FileInfo{info},
				}
				r.nodes[testSpec] = &fakeNode{info: info, file: file}
			})
			spec := artifactByID(t, report, "spec")
			if spec.State != StateUnstable {
				t.Fatalf("spec = %q, want unstable via row 17", spec.State)
			}
			if total := file.bytesRead(); total > MaxArtifactBytes+1 {
				t.Fatalf("requested %d bytes, ceiling is %d", total, MaxArtifactBytes+1)
			}
			if reportMentions(report, "aaaa") {
				t.Fatal("streamed content reached the report")
			}
		})
		t.Run("character-device-leaf", func(t *testing.T) {
			report, _ := runInspect(t, func(r *fakeRoot) {
				r.set(testSpec, fakeInfo{name: testSpec, mode: fs.ModeDevice | fs.ModeCharDevice})
			})
			if got := artifactByID(t, report, "spec").State; got != StateNotRegular {
				t.Fatalf("spec = %q, want not-regular", got)
			}
		})
		t.Run("socket-leaf", func(t *testing.T) {
			report, _ := runInspect(t, func(r *fakeRoot) {
				r.set(testSpec, fakeInfo{name: testSpec, mode: fs.ModeSocket})
			})
			if got := artifactByID(t, report, "spec").State; got != StateNotRegular {
				t.Fatalf("spec = %q, want not-regular", got)
			}
		})
	})

	t.Run("AVP-195", func(t *testing.T) {
		// The post-capture component walk, driven by the "after" hook on the
		// post-read Stat call. The read succeeded and the leaf identity
		// matched, yet no content state may be reported.
		for _, tc := range []struct {
			name  string
			apply func(*fakeRoot)
		}{
			{"becomes-symlink", func(r *fakeRoot) {
				r.set(testSidecarDir, fakeInfo{name: testSidecarDir, mode: fs.ModeSymlink | fs.ModeDir})
			}},
			{"vanishes", func(r *fakeRoot) { r.remove(testSidecarDir) }},
			{"becomes-regular-file", func(r *fakeRoot) {
				r.set(testSidecarDir, regular(testSidecarDir, []byte("x")))
			}},
			{"becomes-irregular", func(r *fakeRoot) {
				r.set(testSidecarDir, fakeInfo{name: testSidecarDir, mode: fs.ModeIrregular})
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				report, _ := runInspect(t, func(r *fakeRoot) {
					file := r.nodes[testSidecar].file
					file.onStat = map[int]func(){2: func() { tc.apply(r) }}
				})
				sidecar := artifactByID(t, report, "analysis_sidecar")
				if sidecar.State != StateUnstable {
					t.Fatalf("sidecar = %q, want unstable (never a content state)", sidecar.State)
				}
				for _, contentState := range []string{StatePresentEmpty, StatePresentNonempty, StateInvalidStructured, StateAbsent} {
					if sidecar.State == contentState {
						t.Fatalf("captured bytes were classified as %q", contentState)
					}
				}
			})
		}
	})

	t.Run("AVP-196", func(t *testing.T) {
		// The three identity limits, each asserted as a limit.
		t.Run("hard-link-alias", func(t *testing.T) {
			report, root := runInspect(t, func(r *fakeRoot) {
				r.beforeOpen[testSpec] = func(root *fakeRoot) {
					root.setFile(testSpec, []byte("link"))
				}
				r.sameFile = func(a, b fs.FileInfo) bool { return true }
			})
			if got := artifactByID(t, report, "spec").State; got != StatePresentNonempty {
				t.Fatalf("spec = %q, want present-nonempty", got)
			}
			if root.sameFileCalls == 0 {
				t.Fatal("identity was not probed")
			}
			if reportMentions(report, "hard link") || reportMentions(report, "detected") {
				t.Fatal("output claims a hard-link alias was detected")
			}
		})
		t.Run("file-id-reuse", func(t *testing.T) {
			// Two genuinely different objects that report the same identity.
			report, _ := runInspect(t, func(r *fakeRoot) {
				info := regular(testSpec, []byte("recycled"))
				r.nodes[testSpec] = &fakeNode{info: info, file: &fakeFile{
					name: testSpec, data: []byte("recycled"), statInfos: []fs.FileInfo{info},
				}}
				r.sameFile = func(a, b fs.FileInfo) bool { return true }
			})
			if got := artifactByID(t, report, "spec").State; got != StatePresentNonempty {
				t.Fatalf("spec = %q, want present-nonempty — reuse is undetectable", got)
			}
			if reportMentions(report, "inode") {
				t.Fatal("output makes an inode claim")
			}
		})
		t.Run("swap-and-restore", func(t *testing.T) {
			report, _ := runInspect(t, func(r *fakeRoot) {
				r.beforeOpen[testSpec] = func(root *fakeRoot) {
					root.setFile(testSpec, []byte("rest"))
				}
			})
			if got := artifactByID(t, report, "spec").State; got != StatePresentNonempty {
				t.Fatalf("spec = %q, want present-nonempty", got)
			}
			if reportMentions(report, "swap") {
				t.Fatal("output claims a swap was defeated")
			}
		})
	})

	t.Run("AVP-197", func(t *testing.T) {
		root := fixtureRoot(t)
		root.setFile(testAnalysis, nil)
		root.setFile(testSpec, []byte("x"))
		root.setFile(testExploration, filledBytes(MaxArtifactBytes-1))
		root.setFile(testSidecar, append([]byte(`{"a":"`), append(filledBytes(MaxArtifactBytes-10), []byte(`"}`)...)...))

		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		scratch := make([]byte, MaxArtifactBytes+1)
		report := Inspect(root, testSlug, scratch)
		runtime.ReadMemStats(&after)

		if report.Abort != nil {
			t.Fatalf("unexpected abort %q", report.Abort.Code)
		}
		allocated := after.TotalAlloc - before.TotalAlloc
		const buffer = MaxArtifactBytes + 1
		if allocated < buffer {
			t.Fatalf("allocated %d bytes; the one scratch buffer alone is %d", allocated, buffer)
		}
		if allocated >= 2*buffer {
			t.Fatalf("allocated %d bytes, which is room for a second %d-byte data buffer", allocated, buffer)
		}
		if len(scratch) != buffer {
			t.Fatalf("scratch length = %d, want %d", len(scratch), buffer)
		}
		// Every capture — status and artifacts alike — started at index 0
		// of the one caller-owned array and saw its full capacity, so none
		// of them allocated a buffer of its own.
		base := &scratch[0]
		checked := 0
		for _, file := range root.handedOut {
			if len(file.reads) == 0 {
				continue
			}
			first := file.reads[0]
			if first.base != base {
				t.Fatalf("%s read from a different backing array", file.name)
			}
			if first.capacity != buffer {
				t.Fatalf("%s was handed a slice with capacity %d, want the shared %d", file.name, first.capacity, buffer)
			}
			checked++
		}
		if checked != 5 {
			t.Fatalf("%d captures read; want the status capture plus four artifacts", checked)
		}
		if got := root.nodes[testStatus].file.reads[0].requested; got != MaxStatusBytes+1 {
			t.Fatalf("status read requested %d bytes, want the %d sub-slice", got, MaxStatusBytes+1)
		}
	})

	t.Run("AVP-203", func(t *testing.T) {
		t.Run("required-artifact", func(t *testing.T) {
			report, _ := runInspect(t, func(r *fakeRoot) {
				r.nodes[testSpec].file.closeErr = errors.New("close failed")
			})
			spec := artifactByID(t, report, "spec")
			if spec.State != StateUnreadable || spec.ReasonCode != ReasonArtifactUnreadable {
				t.Fatalf("spec = (%q, %q), want unreadable", spec.State, spec.ReasonCode)
			}
			if report.Readiness() != ReadinessNotReady {
				t.Fatalf("readiness = %q, want not_ready (exit 2)", report.Readiness())
			}
			if spec.Remediation == "" {
				t.Fatal("a failing required artifact must carry a remediation")
			}
		})
		t.Run("sidecar", func(t *testing.T) {
			report, _ := runInspect(t, func(r *fakeRoot) {
				r.nodes[testSidecar].file.closeErr = errors.New("close failed")
			})
			sidecar := artifactByID(t, report, "analysis_sidecar")
			if sidecar.State != StateUnreadable {
				t.Fatalf("sidecar = %q, want unreadable", sidecar.State)
			}
			if !hasAdvisory(report, AdvisorySidecarUnreadable) {
				t.Fatalf("advisories = %v, want analysis-sidecar-unreadable", advisoryCodes(report))
			}
			if report.Readiness() != ReadinessReady {
				t.Fatalf("readiness = %q, want ready", report.Readiness())
			}
		})
	})

	t.Run("AVP-205", func(t *testing.T) {
		// Every ladder row that can follow a successful open, plus the abort
		// paths, must balance opens against closes exactly.
		corpus := map[string]func(*fakeRoot){
			"row-12-fstat-failure": func(r *fakeRoot) {
				r.nodes[testSpec].file.statErrs = []error{errors.New("fstat")}
			},
			"row-13-identity": func(r *fakeRoot) {
				r.sameFile = func(a, b fs.FileInfo) bool { return a.Name() != testSpec }
			},
			"row-14-kind": func(r *fakeRoot) {
				r.nodes[testSpec].file.statInfos = []fs.FileInfo{fakeInfo{name: testSpec, mode: fs.ModeNamedPipe}}
			},
			"row-15-size": func(r *fakeRoot) {
				r.nodes[testSpec].file.statInfos = []fs.FileInfo{sized(testSpec, 4096)}
			},
			"row-17-growth": func(r *fakeRoot) {
				r.nodes[testSpec].file.data = filledBytes(MaxArtifactBytes + 2)
			},
			"row-18-byte-count": func(r *fakeRoot) {
				r.nodes[testSpec].file.data = []byte("longer than the declared size")
			},
			"row-19-post-read-fstat": func(r *fakeRoot) {
				file := r.nodes[testSpec].file
				file.statInfos = []fs.FileInfo{file.statInfos[0], file.statInfos[0]}
				file.statErrs = []error{nil, errors.New("post-read fstat")}
			},
			"row-20-post-read-size": func(r *fakeRoot) {
				file := r.nodes[testSpec].file
				file.statInfos = []fs.FileInfo{file.statInfos[0], sized(testSpec, 12345)}
			},
			"row-20a-post-walk": func(r *fakeRoot) {
				r.nodes[testSidecar].file.onStat = map[int]func(){2: func() { r.remove(testSidecarDir) }}
			},
			"row-20b-read-error": func(r *fakeRoot) {
				file := r.nodes[testSpec].file
				file.readErrOn, file.readErr = 1, errors.New("read")
			},
			"row-20c-close-failure": func(r *fakeRoot) {
				r.nodes[testSpec].file.closeErr = errors.New("close")
			},
			"row-21-empty":           func(r *fakeRoot) { r.setFile(testSpec, nil) },
			"row-22-nonempty":        func(r *fakeRoot) { r.setFile(testSpec, []byte("body")) },
			"row-23-sidecar-invalid": func(r *fakeRoot) { r.setFile(testSidecar, []byte("{")) },
			"row-24-sidecar-valid":   func(r *fakeRoot) { r.setFile(testSidecar, []byte(`{"k":1}`)) },
			"status-close-failure": func(r *fakeRoot) {
				r.nodes[testStatus].file.closeErr = errors.New("close")
			},
			"status-unstable":  func(r *fakeRoot) { r.sameFile = func(a, b fs.FileInfo) bool { return a.Name() != testStatus } },
			"status-malformed": func(r *fakeRoot) { r.setFile(testStatus, []byte("nope")) },
			"feature-missing":  func(r *fakeRoot) { r.remove(testBase) },
			"feature-unsafe": func(r *fakeRoot) {
				r.set(testBase, fakeInfo{name: testBase, mode: fs.ModeSymlink | fs.ModeDir})
			},
		}
		for name, apply := range corpus {
			t.Run(name, func(t *testing.T) {
				root := fixtureRoot(t)
				apply(root)
				Inspect(root, testSlug, scratchBuffer())
				if len(root.handedOut) == 0 && len(root.opened) > 0 {
					t.Fatal("an open succeeded but no descriptor was tracked")
				}
				for _, file := range root.handedOut {
					if file.closes != 1 {
						t.Fatalf("descriptor %q closed %d times, want exactly 1", file.name, file.closes)
					}
				}
			})
		}
	})

	t.Run("AVP-206", func(t *testing.T) {
		t.Run("verdict-true-proceeds", func(t *testing.T) {
			var file *fakeFile
			report, root := runInspect(t, func(r *fakeRoot) {
				file = r.nodes[testSpec].file
				r.sameFile = func(a, b fs.FileInfo) bool { return true }
			})
			if got := artifactByID(t, report, "spec").State; got != StatePresentNonempty {
				t.Fatalf("spec = %q, want present-nonempty", got)
			}
			if file.bytesRead() == 0 {
				t.Fatal("an identity-equal capture must proceed to the read")
			}
			if root.sameFileCalls < 5 {
				t.Fatalf("SameFile called %d times, want one per capture", root.sameFileCalls)
			}
		})
		t.Run("verdict-false-artifact", func(t *testing.T) {
			var file *fakeFile
			report, _ := runInspect(t, func(r *fakeRoot) {
				file = r.nodes[testSpec].file
				r.sameFile = func(a, b fs.FileInfo) bool { return a.Name() != testSpec }
			})
			if got := artifactByID(t, report, "spec").State; got != StateUnstable {
				t.Fatalf("spec = %q, want unstable", got)
			}
			if file.bytesRead() != 0 {
				t.Fatal("row 13 read bytes it must not read")
			}
		})
		t.Run("verdict-false-status", func(t *testing.T) {
			report, _ := runInspect(t, func(r *fakeRoot) {
				r.sameFile = func(a, b fs.FileInfo) bool { return a.Name() != testStatus }
			})
			if report.AbortCode() != AbortStatusUnstable {
				t.Fatalf("abort = %q, want %q", report.AbortCode(), AbortStatusUnstable)
			}
		})
	})
}
