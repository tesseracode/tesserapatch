//go:build (linux && !android) || (darwin && !ios)

package intentpub

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestS7APDurableWriterContracts(t *testing.T) {
	t.Run("PIB-455", func(t *testing.T) {
		_, authority := acquireWorkspace(t)
		events := []string{}
		rel := ".tpatch/local/intent-prepare/ap-writer/journal.json"
		result, err := DurableWrite(authority, WriteRequest{
			Rel: rel, Data: []byte("control\n"), Mode: 0o600, Role: WriteRoleControl,
		}, Options{
			RandomHex12: fixedHex("abcabcabcabc"),
			RootOpsFactory: func(root *os.Root) RootOps {
				return &s7APWriterOps{RootOps: NewRootOps(root), events: &events}
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{
			"directory-sync",
			"open-temp:0600:o_excl", "write", "file-sync", "close",
			"rename", "directory-sync",
		}
		if fmt.Sprint(events) != fmt.Sprint(want) {
			t.Fatalf("PIB-455 rooted writer sequence = %v, want %v", events, want)
		}
		if !result.Committed || result.Phase != WritePhaseVerified || result.Identity.Mode != 0o600 ||
			string(rootRead(t, authority, rel)) != "control\n" {
			t.Fatalf("PIB-455 durable result = %+v", result)
		}
	})
}

type s7APWriterOps struct {
	RootOps
	events *[]string
}

func (ops *s7APWriterOps) Open(name string) (RootFile, error) {
	file, err := ops.RootOps.Open(name)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(name, "/ap-writer") {
		return &s7APWriterFile{RootFile: file, events: ops.events, directory: true}, nil
	}
	return file, nil
}

func (ops *s7APWriterOps) OpenFile(name string, flag int, mode fs.FileMode) (RootFile, error) {
	file, err := ops.RootOps.OpenFile(name, flag, mode)
	if err != nil {
		return nil, err
	}
	if strings.Contains(name, ".tmp-") {
		exclusive := flag&os.O_EXCL != 0 && flag&os.O_CREATE != 0
		*ops.events = append(
			*ops.events,
			fmt.Sprintf("open-temp:%04o:o_excl", mode.Perm()),
		)
		if !exclusive {
			*ops.events = append(*ops.events, "missing-exclusive-create")
		}
		return &s7APWriterFile{RootFile: file, events: ops.events}, nil
	}
	return file, nil
}

func (ops *s7APWriterOps) Rename(oldName, newName string) error {
	*ops.events = append(*ops.events, "rename")
	return ops.RootOps.Rename(oldName, newName)
}

type s7APWriterFile struct {
	RootFile
	events    *[]string
	directory bool
}

func (file *s7APWriterFile) Write(data []byte) (int, error) {
	*file.events = append(*file.events, "write")
	return file.RootFile.Write(data)
}

func (file *s7APWriterFile) Sync() error {
	if file.directory {
		*file.events = append(*file.events, "directory-sync")
	} else {
		*file.events = append(*file.events, "file-sync")
	}
	return file.RootFile.Sync()
}

func (file *s7APWriterFile) Close() error {
	if !file.directory {
		*file.events = append(*file.events, "close")
	}
	return file.RootFile.Close()
}
