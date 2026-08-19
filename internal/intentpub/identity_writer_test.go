package intentpub

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestIdentityExactnessModesBoundsAndKinds(t *testing.T) {
	_, authority := acquireWorkspace(t)
	rel := ".tpatch/features/identity/status.json"
	first, err := DurableWrite(authority, WriteRequest{Rel: rel, Data: []byte("abcd"), Mode: 0o600}, Options{RandomHex12: sequenceHex()})
	if err != nil {
		t.Fatal(err)
	}
	captured, err := CaptureIdentity(authority, rel, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !captured.Equal(first.Identity) || captured.Size != 4 || captured.Mode != 0o600 {
		t.Fatalf("identity = %#v, want %#v", captured, first.Identity)
	}
	second, err := identityForBytes([]byte("wxyz"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if captured.Equal(second) {
		t.Fatal("same-size/same-mode different bytes compared equal")
	}
	if !AbsentIdentity().Equal(Identity{Exists: false, SHA256: "ignored", Size: 9, Mode: 7}) {
		t.Fatal("two absent identities did not compare equal")
	}

	err = authority.WithRoot(func(root *os.Root) error {
		if err := root.Mkdir(".tpatch/features/identity/a-directory", 0o755); err != nil {
			return err
		}
		return root.Symlink("status.json", ".tpatch/features/identity/a-link")
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, kindRel := range []string{
		".tpatch/features/identity/a-directory",
		".tpatch/features/identity/a-link",
	} {
		_, err := CaptureIdentity(authority, kindRel, Options{})
		assertCode(t, err, CodeNonRegular)
	}

	oversize := make([]byte, MaxArtifactBytes+1)
	rootWrite(t, authority, ".tpatch/features/identity/oversize", oversize, 0o644)
	_, err = CaptureIdentity(authority, ".tpatch/features/identity/oversize", Options{})
	assertCode(t, err, CodeFileOversize)
}

func TestDurableWriterSequenceCASAndFaultCleanup(t *testing.T) {
	t.Run("sequence", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		events := []string{}
		options := Options{
			RandomHex12: fixedHex("abcdefabcdef"),
			RootOpsFactory: func(root *os.Root) RootOps {
				return &writerSpyOps{RootOps: NewRootOps(root), events: &events}
			},
		}
		rel := ".tpatch/local/intent-prepare/writer/control.json"
		if _, err := DurableWrite(authority, WriteRequest{Rel: rel, Data: []byte("body"), Mode: 0o600}, options); err != nil {
			t.Fatal(err)
		}
		assertSubsequence(t, events, []string{"open-temp", "write", "file-sync", "close", "rename", "directory-sync"})
	})

	t.Run("cas", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		rel := ".tpatch/features/writer/status.json"
		rootWrite(t, authority, rel, []byte("editor"), 0o644)
		_, err := DurableWrite(authority, WriteRequest{
			Rel:          rel,
			Data:         []byte("ours"),
			Mode:         0o644,
			Expected:     identityPointer(AbsentIdentity()),
			MismatchCode: CodeEntryAppeared,
			ArtifactID:   ArtifactStatus,
		}, Options{RandomHex12: fixedHex("aaaaaaaaaaaa")})
		assertCode(t, err, CodeEntryAppeared)
		if string(rootRead(t, authority, rel)) != "editor" {
			t.Fatal("writer CAS overwrote editor bytes")
		}
		assertNoTemps(t, authority, pathDir(rel))
	})

	t.Run("sync-fault", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		rel := ".tpatch/local/intent-prepare/writer/fault.json"
		options := Options{
			RandomHex12: fixedHex("bbbbbbbbbbbb"),
			RootOpsFactory: func(root *os.Root) RootOps {
				return &syncFailOps{RootOps: NewRootOps(root)}
			},
		}
		if _, err := DurableWrite(authority, WriteRequest{Rel: rel, Data: []byte("body"), Mode: 0o600}, options); err == nil {
			t.Fatal("injected sync fault was ignored")
		}
		if rootExists(t, authority, rel) {
			t.Fatal("faulting writer published the destination")
		}
		assertNoTemps(t, authority, pathDir(rel))
	})
}

func TestStageJournalAndRawPreimageModes(t *testing.T) {
	_, authority := acquireWorkspace(t)
	stage, err := Stage(authority, testSlug, []StageInput{
		{ArtifactID: ArtifactStatus, Rel: "status.json", Data: []byte("new"), Mode: 0o644},
	}, Options{RandomHex12: sequenceHex()})
	if err != nil {
		t.Fatal(err)
	}
	assertMode(t, authority, stage.StageRel, 0o700)
	assertMode(t, authority, stage.Files[0].Rel, 0o600)
	rawRel, err := WriteRawPreimage(authority, testSlug, ArtifactStatus, []byte("old"), Options{RandomHex12: sequenceHex()})
	if err != nil {
		t.Fatal(err)
	}
	assertMode(t, authority, rawRel, 0o600)

	preimage, err := identityForBytes([]byte("old"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	entry := Entry{
		ArtifactID:     ArtifactStatus,
		Rel:            canonicalRel(testSlug, ArtifactStatus),
		Action:         ActionReplace,
		Preimage:       preimage,
		PreimageRawRel: rawRel,
		NewImage:       stage.Files[0].NewImage,
		StagedRel:      stage.Files[0].Rel,
	}
	exploration, err := identityForBytes([]byte("exploration"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := NewPlan(testSlug, ModeGenerate, stage.StageRel, []Entry{
		{
			ArtifactID: ArtifactExploration,
			Rel:        canonicalRel(testSlug, ArtifactExploration),
			Action:     ActionCreate,
			Preimage:   AbsentIdentity(),
			NewImage:   exploration,
			StagedRel:  stage.StageRel + "/exploration.md",
		},
		entry,
	})
	if err != nil {
		t.Fatal(err)
	}
	journal, err := BuildJournal(plan, "0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PersistJournal(authority, journal, Options{RandomHex12: sequenceHex()}); err != nil {
		t.Fatal(err)
	}
	assertMode(t, authority, JournalRel(testSlug), 0o600)
	if !rootExists(t, authority, rawRel) {
		t.Fatal("journal persistence changed the raw preimage")
	}
}

func TestInvalidJournalRecoveryPerformsNoRootMutation(t *testing.T) {
	_, authority := acquireWorkspace(t)
	rel := JournalRel(testSlug)
	if _, err := DurableWrite(authority, WriteRequest{Rel: rel, Data: []byte(`{"version":1,"unknown":true}`), Mode: 0o600}, Options{RandomHex12: sequenceHex()}); err != nil {
		t.Fatal(err)
	}
	mutations := 0
	options := Options{
		RootOpsFactory: func(root *os.Root) RootOps {
			return &mutationCountingOps{RootOps: NewRootOps(root), count: &mutations}
		},
	}
	_, err := Recover(authority, testSlug, options)
	assertCode(t, err, CodeJournalCorrupt)
	if mutations != 0 {
		t.Fatalf("strict journal bind performed %d root mutations", mutations)
	}
	if !rootExists(t, authority, rel) {
		t.Fatal("invalid journal was removed")
	}
}

func TestCleanupUnarmedLaneIsBounded(t *testing.T) {
	_, authority := acquireWorkspace(t)
	stage, err := Stage(authority, testSlug, []StageInput{
		{ArtifactID: ArtifactStatus, Rel: "status.json", Data: []byte("new"), Mode: 0o644},
	}, Options{RandomHex12: sequenceHex()})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := WriteRawPreimage(authority, testSlug, ArtifactStatus, []byte("old"), Options{RandomHex12: sequenceHex()})
	if err != nil {
		t.Fatal(err)
	}
	abandoned := laneRel(testSlug) + "/abandoned-keep/evidence"
	rootWrite(t, authority, abandoned, []byte("keep"), 0o600)
	ownedTemp := laneRel(testSlug) + "/.journal.json.tmp-abcdefabcdef"
	foreignTemp := laneRel(testSlug) + "/.foreign.json.tmp-abcdefabcdef"
	rootWrite(t, authority, ownedTemp, []byte("owned"), 0o600)
	rootWrite(t, authority, foreignTemp, []byte("keep"), 0o600)
	otherSlug := laneRel("other") + "/stage-0123456789ab/file"
	rootWrite(t, authority, otherSlug, []byte("keep"), 0o600)

	removed, err := CleanupUnarmedLane(authority, testSlug, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 3 || rootExists(t, authority, stage.StageRel) ||
		rootExists(t, authority, raw) || rootExists(t, authority, ownedTemp) {
		t.Fatalf("unexpected cleanup result: %v", removed)
	}
	if string(rootRead(t, authority, abandoned)) != "keep" ||
		string(rootRead(t, authority, foreignTemp)) != "keep" ||
		string(rootRead(t, authority, otherSlug)) != "keep" {
		t.Fatal("bounded cleanup touched unowned evidence")
	}
}

func assertMode(t *testing.T, authority interface {
	WithRoot(func(*os.Root) error) error
}, rel string, want fs.FileMode) {
	t.Helper()
	err := authority.WithRoot(func(root *os.Root) error {
		info, err := root.Lstat(rel)
		if err != nil {
			return err
		}
		if info.Mode().Perm() != want {
			t.Fatalf("%s mode = %o, want %o", rel, info.Mode().Perm(), want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertNoTemps(t *testing.T, authority interface {
	WithRoot(func(*os.Root) error) error
}, directory string) {
	t.Helper()
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
				t.Fatalf("temporary file survived: %s", name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertSubsequence(t *testing.T, events, want []string) {
	t.Helper()
	index := 0
	for _, event := range events {
		if index < len(want) && event == want[index] {
			index++
		}
	}
	if index != len(want) {
		t.Fatalf("events = %v, missing subsequence %v", events, want)
	}
}

type writerSpyOps struct {
	RootOps
	events *[]string
}

func (ops *writerSpyOps) Open(name string) (RootFile, error) {
	file, err := ops.RootOps.Open(name)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(name, "/writer") {
		return &writerSpyFile{RootFile: file, events: ops.events, directory: true}, nil
	}
	return file, nil
}

func (ops *writerSpyOps) OpenFile(name string, flag int, mode fs.FileMode) (RootFile, error) {
	file, err := ops.RootOps.OpenFile(name, flag, mode)
	if err != nil {
		return nil, err
	}
	if strings.Contains(name, ".tmp-") {
		*ops.events = append(*ops.events, "open-temp")
		return &writerSpyFile{RootFile: file, events: ops.events}, nil
	}
	return file, nil
}

func (ops *writerSpyOps) Rename(oldName, newName string) error {
	*ops.events = append(*ops.events, "rename")
	return ops.RootOps.Rename(oldName, newName)
}

type writerSpyFile struct {
	RootFile
	events    *[]string
	directory bool
}

func (file *writerSpyFile) Write(data []byte) (int, error) {
	*file.events = append(*file.events, "write")
	return file.RootFile.Write(data)
}

func (file *writerSpyFile) Sync() error {
	if file.directory {
		*file.events = append(*file.events, "directory-sync")
	} else {
		*file.events = append(*file.events, "file-sync")
	}
	return file.RootFile.Sync()
}

func (file *writerSpyFile) Close() error {
	if !file.directory {
		*file.events = append(*file.events, "close")
	}
	return file.RootFile.Close()
}

type syncFailOps struct {
	RootOps
}

func (ops *syncFailOps) OpenFile(name string, flag int, mode fs.FileMode) (RootFile, error) {
	file, err := ops.RootOps.OpenFile(name, flag, mode)
	if err != nil {
		return nil, err
	}
	if strings.Contains(name, ".tmp-") {
		return &syncFailFile{RootFile: file}, nil
	}
	return file, nil
}

type syncFailFile struct {
	RootFile
}

func (file *syncFailFile) Sync() error {
	return errors.New("injected sync failure")
}

type mutationCountingOps struct {
	RootOps
	count *int
}

func (ops *mutationCountingOps) OpenFile(name string, flag int, mode fs.FileMode) (RootFile, error) {
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0 {
		*ops.count++
	}
	return ops.RootOps.OpenFile(name, flag, mode)
}

func (ops *mutationCountingOps) Mkdir(name string, mode fs.FileMode) error {
	*ops.count++
	return ops.RootOps.Mkdir(name, mode)
}

func (ops *mutationCountingOps) Rename(oldName, newName string) error {
	*ops.count++
	return ops.RootOps.Rename(oldName, newName)
}

func (ops *mutationCountingOps) Remove(name string) error {
	*ops.count++
	return ops.RootOps.Remove(name)
}
