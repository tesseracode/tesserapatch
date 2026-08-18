// Package intent classifies the read-only intent bundle for prepare --check.
package intent

import (
	"io/fs"
	"os"
)

const (
	MaxArtifactBytes = 4 << 20
	MaxStatusBytes   = 1 << 20
)

// The array length becomes negative when the status cap is not strictly
// smaller than the shared artifact buffer cap.
var _ [MaxArtifactBytes - MaxStatusBytes - 1]struct{}

const (
	StatePresentNonempty   = "present-nonempty"
	StatePresentEmpty      = "present-empty"
	StateAbsent            = "absent"
	StateSymlinkRefused    = "symlink-refused"
	StateNotRegular        = "not-regular"
	StateUnreadable        = "unreadable"
	StateOversize          = "oversize"
	StateInvalidStructured = "invalid-structured"
	StateUnstable          = "unstable"
)

// ArtifactStates is the closed §7.6 structural enum in declaration order.
func ArtifactStates() []string {
	return []string{
		StatePresentNonempty,
		StatePresentEmpty,
		StateAbsent,
		StateSymlinkRefused,
		StateNotRegular,
		StateUnreadable,
		StateOversize,
		StateInvalidStructured,
		StateUnstable,
	}
}

const (
	FeatureStateRequested         = "requested"
	FeatureStateAnalyzed          = "analyzed"
	FeatureStateDefined           = "defined"
	FeatureStateImplementing      = "implementing"
	FeatureStateApplied           = "applied"
	FeatureStateActive            = "active"
	FeatureStateReconciling       = "reconciling"
	FeatureStateReconcilingShadow = "reconciling-shadow"
	FeatureStateBlocked           = "blocked"
	FeatureStateUpstreamMerged    = "upstream_merged"
	FeatureStateRejected          = "rejected"
	FeatureStateUnapplied         = "unapplied"
)

// FeatureStates is the closed lifecycle echo domain in declaration order.
// It mirrors store.FeatureState without importing internal/store; AVP-165
// asserts the two lists are equal by AST.
func FeatureStates() []string {
	return []string{
		FeatureStateRequested,
		FeatureStateAnalyzed,
		FeatureStateDefined,
		FeatureStateImplementing,
		FeatureStateApplied,
		FeatureStateActive,
		FeatureStateReconciling,
		FeatureStateReconcilingShadow,
		FeatureStateBlocked,
		FeatureStateUpstreamMerged,
		FeatureStateRejected,
		FeatureStateUnapplied,
	}
}

const disclaimer = "Structural presence only. This report does not certify semantic quality."

// RootOps is the complete rooted filesystem surface available to Inspect.
type RootOps interface {
	Lstat(name string) (fs.FileInfo, error)
	OpenFile(name string, flag int, perm fs.FileMode) (FileOps, error)
	SameFile(a, b fs.FileInfo) bool
}

// FileOps is the complete descriptor surface available to Inspect.
type FileOps interface {
	Stat() (fs.FileInfo, error)
	Read(p []byte) (int, error)
	Close() error
}

type osRootOps struct {
	root *os.Root
}

func (o osRootOps) Lstat(name string) (fs.FileInfo, error) {
	return o.root.Lstat(name)
}

func (o osRootOps) OpenFile(name string, flag int, perm fs.FileMode) (FileOps, error) {
	file, err := o.root.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return osFileOps{file: file}, nil
}

func (o osRootOps) SameFile(a, b fs.FileInfo) bool {
	return os.SameFile(a, b)
}

type osFileOps struct {
	file *os.File
}

func (o osFileOps) Stat() (fs.FileInfo, error) { return o.file.Stat() }
func (o osFileOps) Read(p []byte) (int, error) { return o.file.Read(p) }
func (o osFileOps) Close() error               { return o.file.Close() }

// NewRootOps adapts a root that remains owned by the caller.
func NewRootOps(root *os.Root) RootOps {
	return osRootOps{root: root}
}

// RootConfinementSupported reports whether this target has the accepted
// rooted-inspection implementation class.
func RootConfinementSupported() bool {
	return rootConfinementSupported
}

func validFeatureState(state string) bool {
	switch state {
	case FeatureStateRequested, FeatureStateAnalyzed, FeatureStateDefined,
		FeatureStateImplementing, FeatureStateApplied, FeatureStateActive,
		FeatureStateReconciling, FeatureStateReconcilingShadow, FeatureStateBlocked,
		FeatureStateUpstreamMerged, FeatureStateRejected, FeatureStateUnapplied:
		return true
	default:
		return false
	}
}
