// Path and executable safety for
// PRD-feature-resource-claims-and-capture-adapters §9.
//
// Two deliberately opposite policies live here:
//
//   - §9.1 — repo-owned content (ignored-file selectors, every
//     directory descendant, and db_path) must stay *inside* the
//     repository working tree, and a symlink anywhere in the component
//     chain is refused outright. No symlink is ever resolved or
//     inspected for where it points; refusing all of them is strictly
//     more conservative than checking whether a specific one escapes.
//   - §9.2/§6.1 — the adapter executable must live *outside* the
//     repository tree and any .git directory, and symlinks in its
//     resolution chain are followed because an external tool is
//     expected to be installed via a version manager's shim.

package rescap

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/safety"
)

// GatedPath is a validated repo-relative path plus the open descriptor
// whose identity was confirmed.
type GatedPath struct {
	RelPath string
	AbsPath string
	File    *os.File
	Info    os.FileInfo
	PreOpen os.FileInfo
	IsDir   bool
}

// Close releases the held descriptor.
func (g *GatedPath) Close() error {
	if g == nil || g.File == nil {
		return nil
	}
	f := g.File
	g.File = nil
	return f.Close()
}

// LexicalContainment is the coarse pre-filter that runs before any
// Lstat of any component. It reuses the pre-existing, lexical-only
// safety.EnsureSafeRepoPath; its refusal is named `path-outside-repo`
// here so a caller can tell it apart from the symlink-specific names
// the ancestor walk produces once containment has already passed.
func LexicalContainment(repoRoot, relPath string) (string, error) {
	if relPath == "" {
		return "", Refuse(ReasonPathOutsideRepo, "an empty path never resolves inside the repository")
	}
	if filepath.IsAbs(relPath) {
		return "", Refuse(ReasonPathOutsideRepo, "%q is absolute; selectors are repository-relative", relPath)
	}
	abs := filepath.Join(repoRoot, filepath.Clean(relPath))
	if err := safety.EnsureSafeRepoPath(repoRoot, abs); err != nil {
		return "", Refuse(ReasonPathOutsideRepo, "%q resolves outside the repository root", relPath)
	}
	return abs, nil
}

// GatePath runs §9.1's full five-step gate:
//
//  1. Lstat every successive prefix from the repository root down.
//  2. Any symlink component anywhere refuses outright.
//  3. A missing prefix component refuses `path-missing`.
//  4. Open the final path with O_NOFOLLOW; an ELOOP from a symlink
//     that appeared between the walk and the open refuses the same as
//     step 2.
//  5. fstat the *opened descriptor* and compare it, via os.SameFile,
//     against the FileInfo captured for the final component during the
//     walk. This is a real property of the thing that was actually
//     opened, not a second pathname lookup that could itself race a
//     further swap. A pathname re-Lstat still runs afterwards as
//     defence in depth.
func GatePath(repoRoot, relPath string) (*GatedPath, error) {
	abs, err := LexicalContainment(repoRoot, relPath)
	if err != nil {
		return nil, err
	}
	rel, relErr := filepath.Rel(repoRoot, abs)
	if relErr != nil {
		return nil, Refuse(ReasonPathOutsideRepo, "%q cannot be expressed relative to the repository root", relPath)
	}

	var finalInfo os.FileInfo
	cur := repoRoot
	components := strings.Split(filepath.ToSlash(rel), "/")
	rootInfo, err := os.Lstat(repoRoot)
	if err != nil {
		return nil, Refuse(ReasonPathMissing, "repository root %s does not exist", repoRoot)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, Refuse(ReasonSymlinkComponentRefused, "repository root %s is a symlink", repoRoot)
	}
	for _, comp := range components {
		if comp == "" || comp == "." {
			continue
		}
		cur = filepath.Join(cur, comp)
		info, lstatErr := os.Lstat(cur)
		if lstatErr != nil {
			if errors.Is(lstatErr, os.ErrNotExist) {
				return nil, Refuse(ReasonPathMissing, "%s does not exist", cur)
			}
			return nil, Refuse(ReasonPathMissing, "%s could not be inspected: %v", cur, lstatErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, Refuse(ReasonSymlinkComponentRefused,
				"%s is a symlink; resource capture refuses a symlink anywhere in the component chain", cur)
		}
		finalInfo = info
	}
	if finalInfo == nil {
		return nil, Refuse(ReasonPathOutsideRepo, "%q resolves to the repository root itself", relPath)
	}

	f, openErr := openNoFollow(abs)
	if openErr != nil {
		if isSymlinkLoopError(openErr) {
			return nil, Refuse(ReasonSymlinkComponentRefused,
				"%s became a symlink between the component walk and the open", abs)
		}
		if errors.Is(openErr, os.ErrNotExist) {
			return nil, Refuse(ReasonPathMissing, "%s disappeared before it could be opened", abs)
		}
		return nil, Refuse(ReasonPathMissing, "%s could not be opened: %v", abs, openErr)
	}
	postInfo, statErr := f.Stat()
	if statErr != nil {
		_ = f.Close()
		return nil, Refuse(ReasonPathReplacedDuringOpen, "%s could not be fstat'd after opening: %v", abs, statErr)
	}
	if !os.SameFile(finalInfo, postInfo) {
		_ = f.Close()
		return nil, Refuse(ReasonPathReplacedDuringOpen,
			"%s was replaced between the component walk and the open", abs)
	}
	// Defence in depth: a pathname re-Lstat after the descriptor
	// identity check, which remains the primary, load-bearing guarantee.
	if recheck, err := os.Lstat(abs); err != nil || !os.SameFile(finalInfo, recheck) {
		_ = f.Close()
		return nil, Refuse(ReasonPathReplacedDuringOpen,
			"%s no longer resolves to the entry that was validated", abs)
	}

	return &GatedPath{
		RelPath: filepath.ToSlash(rel),
		AbsPath: abs,
		File:    f,
		Info:    postInfo,
		PreOpen: finalInfo,
		IsDir:   postInfo.IsDir(),
	}, nil
}

// SamePathIdentity resolves relPath's component chain from scratch and
// compares the result against an already-held descriptor's identity.
//
// This is deliberately a pathname-vs-descriptor comparison: an fstat on
// a held descriptor always matches itself regardless of what happened
// to the *name*, so comparing a descriptor against itself would be a
// tautology that can never detect a swap. The real question is always
// "does the pathname still resolve to the exact directory we have
// open", which requires a fresh pathname resolution each time (§9.1's
// db_path honesty subsection).
func SamePathIdentity(repoRoot, relPath string, held os.FileInfo) error {
	fresh, err := GatePath(repoRoot, relPath)
	if err != nil {
		return err
	}
	defer func() { _ = fresh.Close() }()
	if !os.SameFile(held, fresh.Info) {
		return Refuse(ReasonDBPathIdentityChanged,
			"%s no longer resolves to the directory this invocation validated", relPath)
	}
	return nil
}

// ResolveExternalExecutable applies §6.1's shared resolution prefix:
// LookPath, EvalSymlinks, regular-file-with-an-executable-bit, and the
// outside-the-repo requirement.
//
// missingReason lets the caller name the LookPath failure differently
// at add time (`adapter-missing-at-add`, exit 2) and capture time
// (`adapter-missing`, exit 3) — the same underlying failure has two
// distinct names because this design allows each named refusal in
// exactly one row.
func ResolveExternalExecutable(repoRoot, name string, missing *Refusal) (string, error) {
	found, err := lookPath(name)
	if err != nil {
		return "", missing
	}
	resolved, err := filepath.EvalSymlinks(found)
	if err != nil {
		return "", missing
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", missing
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", Refuse(ReasonAdapterExecutableInRepo,
			"%s is not a regular file with an executable bit set", resolved)
	}
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", Refuse(ReasonAdapterExecutableInRepo, "repository root %s could not be resolved", repoRoot)
	}
	if realRoot, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = realRoot
	}
	if pathIsInside(resolved, absRoot) {
		return "", Refuse(ReasonAdapterExecutableInRepo,
			"%s resolves inside the repository working tree; a trusted external tool must live outside it", resolved)
	}
	if pathHasGitComponent(resolved) {
		return "", Refuse(ReasonAdapterExecutableInRepo,
			"%s resolves under a .git directory", resolved)
	}
	return resolved, nil
}

// lookPath is a seam so tests can resolve a fixture binary without
// mutating the process PATH.
var lookPath = defaultLookPath

func pathIsInside(child, parent string) bool {
	child = filepath.Clean(child)
	parent = filepath.Clean(parent)
	if child == parent {
		return true
	}
	return strings.HasPrefix(child, parent+string(filepath.Separator))
}

func pathHasGitComponent(p string) bool {
	for _, comp := range strings.Split(filepath.ToSlash(filepath.Clean(p)), "/") {
		if comp == ".git" {
			return true
		}
	}
	return false
}

func defaultLookPath(name string) (string, error) { return exec.LookPath(name) }

// SetLookPathForTest substitutes the executable resolver and returns a
// restore func. Tests use it to point at a controlled fixture so the
// suite never depends on an installed Dolt binary.
func SetLookPathForTest(fn func(name string) (string, error)) func() {
	prev := lookPath
	lookPath = fn
	return func() { lookPath = prev }
}
