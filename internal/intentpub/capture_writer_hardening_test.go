package intentpub

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
	"unsafe"
)

func TestCaptureUsesComponentWalkAndRejectsAncestorSubstitution(t *testing.T) {
	t.Run("component-pre-post", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		rel := ".tpatch/features/capture/artifacts/value.json"
		rootWrite(t, authority, rel, []byte(`{"ok":true}`), 0o644)
		counts := make(map[string]int)
		_, err := CaptureIdentity(authority, rel, Options{
			RootOpsFactory: func(root *os.Root) RootOps {
				return &componentCountingOps{RootOps: NewRootOps(root), counts: counts}
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		for _, component := range []string{
			".tpatch",
			".tpatch/features",
			".tpatch/features/capture",
			".tpatch/features/capture/artifacts",
		} {
			if counts[component] < 2 {
				t.Fatalf("%s Lstat count = %d, want pre/post walks", component, counts[component])
			}
		}
	})

	t.Run("stable-symlink", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		rootMkdirAll(t, authority, ".tpatch/features/capture/real", 0o755)
		rootWrite(t, authority, ".tpatch/features/capture/real/value.json", []byte(`{}`), 0o644)
		if err := authority.WithRoot(func(root *os.Root) error {
			return root.Symlink("real", ".tpatch/features/capture/artifacts")
		}); err != nil {
			t.Fatal(err)
		}
		_, err := CaptureIdentity(authority, ".tpatch/features/capture/artifacts/value.json", Options{})
		assertCode(t, err, CodeNonRegular)
	})

	t.Run("raced-symlink", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		rel := ".tpatch/features/capture/artifacts/value.json"
		rootWrite(t, authority, rel, []byte(`{"ok":true}`), 0o644)
		state := &ancestorRaceState{}
		_, err := CaptureIdentity(authority, rel, Options{
			RootOpsFactory: func(root *os.Root) RootOps {
				return &ancestorRaceOps{RootOps: NewRootOps(root), root: root, state: state}
			},
		})
		assertCode(t, err, CodeIdentityUnstable)
		if !state.raced {
			t.Fatal("ancestor race seam did not run")
		}
	})
}

func TestCaptureRejectsEveryNonregularFinalKindBeforeOpen(t *testing.T) {
	for name, mode := range map[string]fs.FileMode{
		"directory": fs.ModeDir,
		"fifo":      fs.ModeNamedPipe,
		"socket":    fs.ModeSocket,
		"device":    fs.ModeDevice,
		"symlink":   fs.ModeSymlink,
	} {
		t.Run(name, func(t *testing.T) {
			_, authority := acquireWorkspace(t)
			rel := ".tpatch/features/kinds/value"
			rootWrite(t, authority, rel, []byte("regular"), 0o644)
			opens := 0
			_, err := CaptureIdentity(authority, rel, Options{
				RootOpsFactory: func(root *os.Root) RootOps {
					return &kindOverrideOps{
						RootOps: NewRootOps(root),
						rel:     rel,
						mode:    mode,
						opens:   &opens,
					}
				},
			})
			assertCode(t, err, CodeNonRegular)
			if opens != 0 {
				t.Fatalf("nonregular %s was opened", name)
			}
		})
	}
}

func TestExecuteUsesOneScratchAndSameDirectoryPublication(t *testing.T) {
	_, authority := acquireWorkspace(t)
	plan := stageCreatePlan(t, authority)
	allocations := 0
	backings := make(map[uintptr]struct{})
	var renames [][2]string
	options := Options{
		RandomHex12: sequenceHex(),
		ScratchFactory: func(size int) []byte {
			allocations++
			return make([]byte, size)
		},
		RootOpsFactory: func(root *os.Root) RootOps {
			return &scratchRenameSpyOps{
				RootOps:  NewRootOps(root),
				backings: backings,
				renames:  &renames,
			}
		},
	}
	result, err := Execute(authority, plan, "0123456789abcdef", nil, options)
	if err != nil {
		t.Fatal(err)
	}
	if allocations != 1 || len(backings) != 1 {
		t.Fatalf("scratch allocations=%d backings=%d, want exactly one", allocations, len(backings))
	}
	if result.Outcome != OutcomePublished {
		t.Fatalf("result = %#v", result)
	}
	canonicalRenames := 0
	for _, pair := range renames {
		if !strings.HasPrefix(pair[1], featureRel(testSlug)+"/") {
			continue
		}
		canonicalRenames++
		if path.Dir(pair[0]) != path.Dir(pair[1]) {
			t.Fatalf("cross-directory canonical rename: %q -> %q", pair[0], pair[1])
		}
		if strings.Contains(pair[0], "/stage-") {
			t.Fatalf("local staged file renamed into canonical tree: %q", pair[0])
		}
	}
	if canonicalRenames != len(plan.Entries()) {
		t.Fatalf("canonical renames = %d, want %d", canonicalRenames, len(plan.Entries()))
	}
	for _, entry := range plan.Entries() {
		assertMode(t, authority, entry.Rel, fs.FileMode(entry.NewImage.Mode))
	}
}

func TestJournalFailureLeavesCanonicalTreeByteIdentical(t *testing.T) {
	workspace, authority := acquireWorkspace(t)
	plan := stageCreatePlan(t, authority)
	before := snapshotPath(t, workspace, featureRel(testSlug))
	state := &journalRenameFailureState{}
	result, err := Execute(authority, plan, "0123456789abcdef", nil, Options{
		RandomHex12: sequenceHex(),
		RootOpsFactory: func(root *os.Root) RootOps {
			return &journalRenameFailureOps{RootOps: NewRootOps(root), state: state}
		},
	})
	assertCode(t, err, CodeRootedWrite)
	if result.ExitClass != 5 || !state.failed {
		t.Fatalf("result=%#v state=%#v", result, state)
	}

	after := snapshotPath(t, workspace, featureRel(testSlug))
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("canonical tree changed before journal durability:\nbefore=%v\nafter=%v", before, after)
	}
}

func TestExecuteRequiresCanonicalParentsWithoutCreatingThem(t *testing.T) {
	_, authority := acquireWorkspace(t)
	result, err := Execute(authority, testCreatePlan(t), "0123456789abcdef", nil, Options{
		RandomHex12: sequenceHex(),
	})
	assertCode(t, err, CodeRootedWrite)
	if result.ExitClass != 5 {
		t.Fatalf("result = %#v", result)
	}
	if rootExists(t, authority, ".tpatch") ||
		rootExists(t, authority, JournalRel(testSlug)) {
		t.Fatal("preflight created canonical or journal directories before journal durability")
	}
}

func TestRootedWriterFaultTableAndCommitState(t *testing.T) {
	tests := []struct {
		name          writerFault
		wantCommitted bool
		wantExit      int
		wantTemp      bool
		wantPhase     WritePhase
	}{
		{faultOpenTemp, false, 5, false, WritePhaseParentReady},
		{faultChmodTemp, false, 5, false, WritePhaseTempOpened},
		{faultWriteTemp, false, 5, false, WritePhaseTempOpened},
		{faultSyncTemp, false, 5, false, WritePhaseTempWritten},
		{faultCloseTemp, false, 5, false, WritePhaseTempSynced},
		{faultCAS, false, 5, false, WritePhaseTempClosed},
		{faultRename, false, 5, false, WritePhaseCASValidated},
		{faultCleanupRemove, false, 6, true, WritePhaseTempOpened},
		{faultCleanupSync, false, 6, false, WritePhaseTempOpened},
		{faultDirectorySync, true, 6, false, WritePhaseRenamed},
		{faultPostCapture, true, 6, false, WritePhaseDirectorySynced},
	}
	for _, test := range tests {
		t.Run(string(test.name), func(t *testing.T) {
			_, authority := acquireWorkspace(t)
			directory := ".tpatch/features/writer-fault"
			rootMkdirAll(t, authority, directory, 0o755)
			target := directory + "/value.json"
			state := &writerFaultState{fault: test.name, target: target, directory: directory}
			result, err := DurableWrite(authority, WriteRequest{
				Rel:           target,
				Data:          []byte("value"),
				Mode:          0o640,
				Expected:      identityPointer(AbsentIdentity()),
				MismatchCode:  CodeEntryAppeared,
				RequireParent: true,
			}, Options{
				RandomHex12: fixedHex("abcdefabcdef"),
				RootOpsFactory: func(root *os.Root) RootOps {
					return &writerFaultOps{RootOps: NewRootOps(root), state: state}
				},
			})
			var typed *Error
			if !errors.As(err, &typed) {
				t.Fatalf("error = %#v", err)
			}
			if result.Committed != test.wantCommitted || typed.Committed != test.wantCommitted ||
				typed.ExitClass != test.wantExit || result.Phase != test.wantPhase ||
				typed.WritePhase != test.wantPhase {
				t.Fatalf("result=%#v error=%#v", result, typed)
			}
			if test.wantCommitted {
				if !rootExists(t, authority, target) {
					t.Fatal("committed writer did not leave the destination")
				}
			} else if rootExists(t, authority, target) {
				t.Fatal("uncommitted writer changed the destination")
			}
			hasTemp := directoryHasTemp(t, authority, directory)
			if hasTemp != test.wantTemp {
				t.Fatalf("temp present=%v, want %v", hasTemp, test.wantTemp)
			}
		})
	}
}

func TestPersistJournalPostRenameFailureRetainsEvidence(t *testing.T) {
	_, authority := acquireWorkspace(t)
	rootMkdirAll(t, authority, laneRel(testSlug), 0o700)
	journal, err := BuildJournal(testCreatePlan(t), "0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	state := &writerFaultState{
		fault:     faultDirectorySync,
		target:    JournalRel(testSlug),
		directory: laneRel(testSlug),
	}
	result, err := PersistJournal(authority, journal, Options{
		RandomHex12: fixedHex("abcdefabcdef"),
		RootOpsFactory: func(root *os.Root) RootOps {
			return &writerFaultOps{RootOps: NewRootOps(root), state: state}
		},
	})
	assertCode(t, err, CodePostPublicationDivergence)
	if !result.Committed || !rootExists(t, authority, JournalRel(testSlug)) {
		t.Fatalf("journal commit state = %#v", result)
	}
	assertMode(t, authority, JournalRel(testSlug), 0o600)
}

type componentCountingOps struct {
	RootOps
	counts map[string]int
}

type kindOverrideOps struct {
	RootOps
	rel   string
	mode  fs.FileMode
	opens *int
}

func (ops *kindOverrideOps) Lstat(name string) (fs.FileInfo, error) {
	info, err := ops.RootOps.Lstat(name)
	if err == nil && name == ops.rel {
		return modeOverrideInfo{FileInfo: info, mode: ops.mode}, nil
	}
	return info, err
}

func (ops *kindOverrideOps) OpenFile(name string, flag int, mode fs.FileMode) (RootFile, error) {
	if name == ops.rel {
		*ops.opens++
	}
	return ops.RootOps.OpenFile(name, flag, mode)
}

type modeOverrideInfo struct {
	fs.FileInfo
	mode fs.FileMode
}

func (info modeOverrideInfo) Mode() fs.FileMode {
	return info.mode
}

func (ops *componentCountingOps) Lstat(name string) (fs.FileInfo, error) {
	ops.counts[name]++
	return ops.RootOps.Lstat(name)
}

type ancestorRaceState struct {
	raced bool
}

type ancestorRaceOps struct {
	RootOps
	root  *os.Root
	state *ancestorRaceState
}

func (ops *ancestorRaceOps) OpenFile(name string, flag int, mode fs.FileMode) (RootFile, error) {
	file, err := ops.RootOps.OpenFile(name, flag, mode)
	if err != nil {
		return nil, err
	}
	return &ancestorRaceFile{RootFile: file, root: ops.root, state: ops.state}, nil
}

type ancestorRaceFile struct {
	RootFile
	root      *os.Root
	state     *ancestorRaceState
	statCalls int
}

func (file *ancestorRaceFile) Stat() (fs.FileInfo, error) {
	info, err := file.RootFile.Stat()
	file.statCalls++
	if err == nil && file.statCalls == 2 && !file.state.raced {
		parent := ".tpatch/features/capture/artifacts"
		if renameErr := file.root.Rename(parent, parent+"-real"); renameErr != nil {
			return nil, renameErr
		}
		if symlinkErr := file.root.Symlink("artifacts-real", parent); symlinkErr != nil {
			return nil, symlinkErr
		}
		file.state.raced = true
	}
	return info, err
}

type scratchRenameSpyOps struct {
	RootOps
	backings map[uintptr]struct{}
	renames  *[][2]string
}

func (ops *scratchRenameSpyOps) OpenFile(name string, flag int, mode fs.FileMode) (RootFile, error) {
	file, err := ops.RootOps.OpenFile(name, flag, mode)
	if err != nil {
		return nil, err
	}
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) == 0 {
		return &scratchSpyFile{RootFile: file, backings: ops.backings}, nil
	}
	return file, nil
}

func (ops *scratchRenameSpyOps) Rename(oldName, newName string) error {
	*ops.renames = append(*ops.renames, [2]string{oldName, newName})
	return ops.RootOps.Rename(oldName, newName)
}

type scratchSpyFile struct {
	RootFile
	backings map[uintptr]struct{}
}

func (file *scratchSpyFile) Read(buffer []byte) (int, error) {
	if len(buffer) > 0 {
		offset := MaxArtifactBytes + 1 - cap(buffer)
		base := uintptr(unsafe.Pointer(&buffer[0])) - uintptr(offset)
		file.backings[base] = struct{}{}
	}
	return file.RootFile.Read(buffer)
}

type journalRenameFailureState struct {
	failed bool
}

type journalRenameFailureOps struct {
	RootOps
	state *journalRenameFailureState
}

func (ops *journalRenameFailureOps) Rename(oldName, newName string) error {
	if newName == JournalRel(testSlug) && !ops.state.failed {
		ops.state.failed = true
		return errors.New("journal rename fault")
	}
	return ops.RootOps.Rename(oldName, newName)
}

type writerFault string

const (
	faultOpenTemp      writerFault = "open-temp"
	faultChmodTemp     writerFault = "chmod-temp"
	faultWriteTemp     writerFault = "write-temp"
	faultSyncTemp      writerFault = "sync-temp"
	faultCloseTemp     writerFault = "close-temp"
	faultCAS           writerFault = "cas"
	faultRename        writerFault = "rename"
	faultCleanupRemove writerFault = "cleanup-remove"
	faultCleanupSync   writerFault = "cleanup-sync"
	faultDirectorySync writerFault = "directory-sync"
	faultPostCapture   writerFault = "post-capture"
)

type writerFaultState struct {
	fault        writerFault
	target       string
	directory    string
	temp         string
	tempClosed   bool
	renamed      bool
	removeCalled bool
	fired        bool
}

type writerFaultOps struct {
	RootOps
	state *writerFaultState
}

func (ops *writerFaultOps) Lstat(name string) (fs.FileInfo, error) {
	if name == ops.state.target {
		if ops.state.fault == faultCAS && ops.state.tempClosed && !ops.state.renamed && !ops.state.fired {
			ops.state.fired = true
			return nil, errors.New("cas capture fault")
		}
		if ops.state.fault == faultPostCapture && ops.state.renamed && !ops.state.fired {
			ops.state.fired = true
			return nil, errors.New("post-capture fault")
		}
	}
	return ops.RootOps.Lstat(name)
}

func (ops *writerFaultOps) Open(name string) (RootFile, error) {
	file, err := ops.RootOps.Open(name)
	if err != nil {
		return nil, err
	}
	if name == ops.state.directory {
		return &writerFaultDir{RootFile: file, state: ops.state}, nil
	}
	return file, nil
}

func (ops *writerFaultOps) OpenFile(name string, flag int, mode fs.FileMode) (RootFile, error) {
	if strings.Contains(name, ".tmp-") {
		ops.state.temp = name
		if ops.state.fault == faultOpenTemp && !ops.state.fired {
			ops.state.fired = true
			return nil, errors.New("open temp fault")
		}
	}
	file, err := ops.RootOps.OpenFile(name, flag, mode)
	if err != nil {
		return nil, err
	}
	if name == ops.state.temp {
		return &writerFaultFile{RootFile: file, state: ops.state}, nil
	}
	return file, nil
}

func (ops *writerFaultOps) Rename(oldName, newName string) error {
	if newName == ops.state.target && ops.state.fault == faultRename && !ops.state.fired {
		ops.state.fired = true
		return errors.New("rename fault")
	}
	err := ops.RootOps.Rename(oldName, newName)
	if err == nil && newName == ops.state.target {
		ops.state.renamed = true
	}
	return err
}

func (ops *writerFaultOps) Remove(name string) error {
	if name == ops.state.temp {
		ops.state.removeCalled = true
		if ops.state.fault == faultCleanupRemove && !ops.state.fired {
			ops.state.fired = true
			return errors.New("cleanup remove fault")
		}
	}
	return ops.RootOps.Remove(name)
}

type writerFaultFile struct {
	RootFile
	state *writerFaultState
}

func (file *writerFaultFile) Chmod(mode fs.FileMode) error {
	if file.state.fault == faultChmodTemp && !file.state.fired {
		file.state.fired = true
		return errors.New("chmod fault")
	}
	return file.RootFile.Chmod(mode)
}

func (file *writerFaultFile) Write(data []byte) (int, error) {
	if (file.state.fault == faultWriteTemp || file.state.fault == faultCleanupRemove ||
		file.state.fault == faultCleanupSync) && !file.state.fired {
		if file.state.fault == faultWriteTemp {
			file.state.fired = true
		}
		return 0, errors.New("write fault")
	}
	return file.RootFile.Write(data)
}

func (file *writerFaultFile) Sync() error {
	if file.state.fault == faultSyncTemp && !file.state.fired {
		file.state.fired = true
		return errors.New("file sync fault")
	}
	return file.RootFile.Sync()
}

func (file *writerFaultFile) Close() error {
	err := file.RootFile.Close()
	file.state.tempClosed = true
	if file.state.fault == faultCloseTemp && !file.state.fired {
		file.state.fired = true
		return errors.New("close fault")
	}
	return err
}

type writerFaultDir struct {
	RootFile
	state *writerFaultState
}

func (file *writerFaultDir) Sync() error {
	if file.state.fault == faultCleanupSync && file.state.removeCalled && !file.state.fired {
		file.state.fired = true
		return errors.New("cleanup directory sync fault")
	}
	if file.state.fault == faultDirectorySync && file.state.renamed && !file.state.fired {
		file.state.fired = true
		return errors.New("post-rename directory sync fault")
	}
	return file.RootFile.Sync()
}

func directoryHasTemp(t *testing.T, authority interface {
	WithRoot(func(*os.Root) error) error
}, directory string) bool {
	t.Helper()
	found := false
	err := authority.WithRoot(func(root *os.Root) error {
		file, err := root.Open(directory)
		if err != nil {
			return err
		}
		names, readErr := file.Readdirnames(-1)
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			return errors.Join(readErr, closeErr)
		}
		for _, name := range names {
			if strings.Contains(name, ".tmp-") {
				found = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}
