// Ephemeral scratch lifecycle and the local ignore/untracked gate for
// PRD-feature-resource-claims-and-capture-adapters §7.1 / §10.3.
//
// .tpatch/local/ is the existing gitignored local root. Before this
// invocation's first write anywhere under it — including the .lock file
// itself, before its very first creation for a given slug — every
// mutator runs two separate checks with two separate targets:
//
//   - the ignore half targets the exact per-slug leaf and is
//     existence-independent, because git check-ignore answers "would
//     this path be ignored if it existed" without requiring it to exist;
//   - the untracked half targets the whole .tpatch/local/ subtree,
//     because "is anything tracked under this gitignored root" is a
//     subtree question, and a tracked file anywhere under it is a
//     privacy-boundary violation regardless of which slug's scratch
//     tree it sits under.
//
// statfs, by contrast, is a kernel syscall on an existing inode and
// genuinely cannot run against a not-yet-created leaf, so it alone
// narrows to the nearest existing ancestor.

package rescap

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tesseracode/tesserapatch/internal/store"
	"github.com/tesseracode/tesserapatch/internal/workflow"
)

// LocalScratchPrefix is the gitignored root every scratch tree lives
// under.
const LocalScratchPrefix = ".tpatch/local/"

// ResourceScratchSegment is the per-slug scratch subtree.
const ResourceScratchSegment = "resource-scratch"

// ScratchRoot returns .tpatch/local/resource-scratch/<slug>/.
func ScratchRoot(repoRoot, slug string) string {
	return filepath.Join(repoRoot, ".tpatch", "local", ResourceScratchSegment, slug)
}

// EnsureLocalContract runs §10.3's two-target gate. It must run before
// any scratch content, including the lock file, is created.
func EnsureLocalContract(repoRoot, slug string) error {
	leaf := ScratchRoot(repoRoot, slug)
	if err := workflow.EnsureLocalIgnoreContract(repoRoot, leaf); err != nil {
		return Refuse(ReasonLocalRootNotIgnored,
			"%s is not covered by the .tpatch/local/ ignore contract: %v", leaf, err)
	}
	tracked, err := anythingTrackedUnderCompatibility(repoRoot, LocalScratchPrefix)
	if err != nil {
		return err
	}
	if tracked {
		return Refuse(ReasonLocalPathTracked,
			"one or more files under %s are tracked by git; resource capture refuses to write there", LocalScratchPrefix)
	}
	return nil
}

// Scratch is one invocation's ephemeral scratch directory.
type Scratch struct {
	Root      string
	DoltHome  string
	createdAt bool
}

// EphemeralScratch creates a fresh es_<12 hex>/ directory 0700 under
// the per-slug scratch root and fsyncs the whole chain. It is removed
// (best-effort) as the last step of the invocation on both the success
// and failure paths.
func EphemeralScratch(repoRoot, slug string) (*Scratch, error) {
	suffix, err := store.RandomHex12()
	if err != nil {
		return nil, Internal(ReasonAdapterCopyFailed, "generating a scratch suffix: %v", err)
	}
	root := filepath.Join(ScratchRoot(repoRoot, slug), "es_"+suffix)
	if err := store.MkdirAllAndSyncChain(root, repoRoot, 0o700); err != nil {
		return nil, Internal(ReasonAdapterCopyFailed, "creating %s: %v", root, err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return nil, Internal(ReasonAdapterCopyFailed, "setting scratch permissions: %v", err)
	}
	return &Scratch{Root: root, createdAt: true}, nil
}

// EnsureDoltHome creates the isolated HOME/DOLT_ROOT_PATH directory the
// Dolt child process is given. It exists so Dolt may write its own
// ephemeral config/state somewhere that is not the invoking user's real
// home — never a captured byte, never repo content.
func (s *Scratch) EnsureDoltHome() (string, error) {
	if s == nil {
		return "", Internal(ReasonAdapterCopyFailed, "no scratch directory was created")
	}
	if s.DoltHome != "" {
		return s.DoltHome, nil
	}
	home := filepath.Join(s.Root, "dolt-home")
	if err := os.Mkdir(home, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return "", Internal(ReasonAdapterCopyFailed, "creating the scratch HOME: %v", err)
	}
	s.DoltHome = home
	return home, nil
}

// Remove deletes the whole ephemeral tree, best-effort. A removal
// failure is a local diagnostic, never a hard failure.
func (s *Scratch) Remove() []string {
	if s == nil || !s.createdAt {
		return nil
	}
	if err := os.RemoveAll(s.Root); err != nil {
		return []string{fmt.Sprintf("could not remove ephemeral scratch %s: %v", s.Root, err)}
	}
	return nil
}

// SweepLocalOrphans removes leftover es_*/ directories from a prior
// invocation that crashed mid-flight. It runs only after the current
// invocation has itself acquired the live flock, so it can never race a
// different, concurrently-running mutator's in-flight scratch content.
//
// Only capture/record --resources ever call it: add/remove/clear/
// trust-dolt acquire the lock but never create scratch content, so v1
// has no reason to sweep from them.
func SweepLocalOrphans(repoRoot, slug string, keep string) []string {
	var diags []string
	entries, err := os.ReadDir(ScratchRoot(repoRoot, slug))
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() || len(e.Name()) < 3 || e.Name()[:3] != "es_" {
			continue
		}
		full := filepath.Join(ScratchRoot(repoRoot, slug), e.Name())
		if keep != "" && full == keep {
			continue
		}
		if err := os.RemoveAll(full); err != nil {
			diags = append(diags, fmt.Sprintf("could not remove orphan scratch %s: %v", e.Name(), err))
		}
	}
	return diags
}
