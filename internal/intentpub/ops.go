package intentpub

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
)

type RootFile interface {
	io.Reader
	io.Writer
	Sync() error
	Close() error
	Stat() (fs.FileInfo, error)
	Chmod(fs.FileMode) error
	Readdirnames(int) ([]string, error)
}

type RootOps interface {
	Lstat(string) (fs.FileInfo, error)
	Open(string) (RootFile, error)
	OpenFile(string, int, fs.FileMode) (RootFile, error)
	SameFile(fs.FileInfo, fs.FileInfo) bool
	Mkdir(string, fs.FileMode) error
	Rename(string, string) error
	Remove(string) error
}

type osRootOps struct {
	root *os.Root
}

func NewRootOps(root *os.Root) RootOps {
	return osRootOps{root: root}
}

func (o osRootOps) Lstat(name string) (fs.FileInfo, error) {
	return o.root.Lstat(name)
}

func (o osRootOps) Open(name string) (RootFile, error) {
	return o.root.Open(name)
}

func (o osRootOps) OpenFile(name string, flag int, mode fs.FileMode) (RootFile, error) {
	return o.root.OpenFile(name, flag, mode)
}

func (o osRootOps) SameFile(first, second fs.FileInfo) bool {
	return os.SameFile(first, second)
}

func (o osRootOps) Mkdir(name string, mode fs.FileMode) error {
	return o.root.Mkdir(name, mode)
}

func (o osRootOps) Rename(oldName, newName string) error {
	return o.root.Rename(oldName, newName)
}

func (o osRootOps) Remove(name string) error {
	return o.root.Remove(name)
}

type Options struct {
	RootOpsFactory func(*os.Root) RootOps
	RandomHex12    func() (string, error)
	Hook           func(CrashPoint, *os.Root, *Entry) error
	Scratch        []byte
	ScratchFactory func(int) []byte
}

func (options Options) rootOps(root *os.Root) RootOps {
	if options.RootOpsFactory != nil {
		return options.RootOpsFactory(root)
	}
	return NewRootOps(root)
}

func (options Options) randomHex12() (string, error) {
	if options.RandomHex12 != nil {
		return options.RandomHex12()
	}
	var bytes [6]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func NewRunNonce() (string, error) {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func (options Options) withScratch() (Options, error) {
	if options.Scratch == nil {
		if options.ScratchFactory != nil {
			options.Scratch = options.ScratchFactory(MaxArtifactBytes + 1)
		} else {
			options.Scratch = make([]byte, MaxArtifactBytes+1)
		}
	}
	if len(options.Scratch) != MaxArtifactBytes+1 {
		return Options{}, transactionError(
			CodeInvalidPlan,
			"",
			"scratch",
			fmt.Sprintf("the shared scratch buffer must be exactly %d bytes", MaxArtifactBytes+1),
			5,
		)
	}
	return options, nil
}

func (options Options) capture(ops RootOps, rel string) (Identity, error) {
	captured, err := captureRegular(ops, rel, options.Scratch)
	return captured.Identity, err
}

func (options Options) captureBytes(ops RootOps, rel string) (capturedFile, error) {
	return captureRegular(ops, rel, options.Scratch)
}
