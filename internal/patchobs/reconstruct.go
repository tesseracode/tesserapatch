package patchobs

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
)

// ReconstructedSide is independently obtained reference/payload evidence.
// Available is separate from the historical event's observation ceiling.
type ReconstructedSide struct {
	Available     bool
	Present       bool
	Mode          string
	SHA256        string
	Bytes         []byte
	Limitation    string
	PayloadAbsent bool
	DigestProved  bool
	NULPresent    bool
}

type ReconstructedEffect struct {
	Effect    gitutil.PatchEffect
	Pre, Post ReconstructedSide
}

// Reconstruct reads only the named local commit and reconstructs payloads in
// memory. Images share the same per-observation budget as producer capture.
// No worktree, index, object write, or optional cached body supplies evidence.
func Reconstruct(root, commit, patch string) ([]ReconstructedEffect, error) {
	return reconstructWithEnv(root, commit, patch, gitutil.CaptureReadOnlyEnv())
}

// ReconstructOffline retains the same image budget and grammar while isolating
// candidate reads from inherited Git routing, configuration and replacements.
func ReconstructOffline(root, commit, patch string) ([]ReconstructedEffect, error) {
	return reconstructWithEnv(root, commit, patch, gitutil.HardenedNoLazyGitEnv("GIT_OPTIONAL_LOCKS=0"))
}

// CheckCommitPathsOffline proves affected upstream objects exist without
// retaining their bodies or introducing another source-image size limit.
func CheckCommitPathsOffline(root, commit string, paths []string) error {
	_, err := SnapshotCommitPathsOffline(root, commit, paths)
	return err
}

// CommitPath is immutable tree/object metadata, not a live path observation.
type CommitPath struct {
	Path     string
	Present  bool
	Mode     string
	ObjectID string
	Size     int64
}

func SnapshotCommitPathsOffline(root, commit string, paths []string) ([]CommitPath, error) {
	env := gitutil.HardenedNoLazyGitEnv("GIT_OPTIONAL_LOCKS=0")
	if resolved, ok := resolveCommitWithEnv(root, commit+"^{commit}", env); !ok || resolved != commit {
		return nil, fmt.Errorf("named local reference is unavailable or is not a commit")
	}
	tree := readTreeEntriesWithEnv(root, commit, paths, env)
	if !tree.read {
		return nil, fmt.Errorf("cannot read named local reference tree")
	}
	ids := neededBlobIDs(tree)
	sizes := map[string]int64{}
	if len(ids) != 0 {
		raw, err := runGitWithEnv(root, []byte(strings.Join(ids, "\n")+"\n"), env, "cat-file", "--batch-check")
		if err != nil {
			return nil, fmt.Errorf("cannot inspect local tree objects: %w", err)
		}
		lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
		if len(lines) != len(ids) {
			return nil, fmt.Errorf("local object inventory is incomplete")
		}
		for i, line := range lines {
			fields := strings.Fields(line)
			if len(fields) != 3 || fields[0] != ids[i] || (fields[1] != "blob" && fields[1] != "tree") {
				return nil, fmt.Errorf("required upstream object is unavailable")
			}
			size, err := strconv.ParseInt(fields[2], 10, 64)
			if err != nil || size < 0 {
				return nil, fmt.Errorf("invalid local object size")
			}
			sizes[ids[i]] = size
		}
	}
	result := make([]CommitPath, 0, len(paths))
	for _, path := range sortedUnique(paths) {
		entry, found, _ := tree.lookup(path)
		result = append(result, CommitPath{Path: path, Present: found, Mode: entry.mode, ObjectID: entry.objectSHA, Size: sizes[entry.objectSHA]})
	}
	return result, nil
}

var ErrCommitObjectLimit = errors.New("immutable object exceeds read limit")

// ReadCommitObjectOffline reads a captured immutable identity, not a path or ref.
func ReadCommitObjectOffline(root, objectID string, limit int64) ([]byte, error) {
	if !isFullCommitHex(objectID) || limit < 0 {
		return nil, fmt.Errorf("invalid immutable object read")
	}
	blobs := readBlobsWithEnv(root, []string{objectID}, &imageBudget{limit: limit},
		gitutil.HardenedNoLazyGitEnv("GIT_OPTIONAL_LOCKS=0"))
	blob, ok := blobs[objectID]
	if !ok {
		return nil, fmt.Errorf("immutable object unavailable")
	}
	if blob.diagnostic != "" {
		return nil, ErrCommitObjectLimit
	}
	return blob.bytes, nil
}

func reconstructWithEnv(root, commit, patch string, env []string) ([]ReconstructedEffect, error) {
	effects, err := gitutil.NormalizePatchEffects(patch)
	if err != nil {
		return nil, fmt.Errorf("canonical patch is not reconstructable")
	}
	resolved, ok := resolveCommitWithEnv(root, commit+"^{commit}", env)
	if !ok || resolved != commit {
		return nil, fmt.Errorf("named local reference is unavailable or is not a commit")
	}
	paths := make([]string, 0, len(effects))
	for _, effect := range effects {
		path := effect.Path
		if effect.OldPath != "" {
			path = effect.OldPath
		}
		paths = append(paths, path)
	}
	tree := readTreeEntriesWithEnv(root, commit, paths, env)
	if !tree.read {
		return nil, fmt.Errorf("cannot read named local reference tree")
	}
	budget := &imageBudget{limit: imageRetentionLimit}
	blobs := readBlobsWithEnv(root, neededBlobIDs(tree), budget, env)
	result := make([]ReconstructedEffect, 0, len(effects))
	for _, effect := range effects {
		path := effect.Path
		if effect.OldPath != "" {
			path = effect.OldPath
		}
		entry, found, _ := tree.lookup(path)
		side := sideFromTree(tree, blobs, path)
		pre := ReconstructedSide{Available: side.observed, Present: side.present,
			Mode: side.mode, SHA256: side.sha256, Bytes: side.bytes, Limitation: side.diagnostic}
		if found && !side.observed && side.diagnostic == "" {
			return nil, fmt.Errorf("%s: required local preimage object unavailable", path)
		}
		if found && entry.mode != gitutil.ModeRegular && entry.mode != gitutil.ModeExecutable &&
			entry.mode != gitutil.ModeSymlink && entry.mode != gitutil.ModeGitlink {
			return nil, fmt.Errorf("%s: reference entry is not a supported file object", path)
		}
		row := ReconstructedEffect{Effect: effect, Pre: pre}
		preExtant, postExtant := effect.ExtantSides()
		if pre.Available && pre.Present != preExtant {
			return nil, fmt.Errorf("%s: reference presence contradicts patch", path)
		}
		if pre.Available && pre.Present && effect.HeaderOldMode != "" && pre.Mode != effect.HeaderOldMode {
			return nil, fmt.Errorf("%s: reference mode contradicts patch", path)
		}
		if !postExtant {
			row.Post = ReconstructedSide{Available: true}
		}
		if !pre.Available {
			row.Post.Limitation = "preimage unavailable within image retention budget"
			result = append(result, row)
			continue
		}
		mode := effect.HeaderNewMode
		if mode == "" {
			mode = pre.Mode
		}
		if mode == "" && postExtant {
			return nil, fmt.Errorf("%s: postimage mode not established by the patch", effect.Path)
		}
		input := pre.Bytes
		if pre.Mode == gitutil.ModeGitlink {
			input = []byte("Subproject commit " + entry.objectSHA + "\n")
		}
		postLimit := budget.limit - budget.used
		gitlinkPost := postExtant && mode == gitutil.ModeGitlink
		if gitlinkPost {
			// Gitlink text is bounded scratch, not a retained file image.
			postLimit = int64(len("Subproject commit ") + 40 + 1)
		}
		post, available, err := gitutil.ReconstructPatchPostimage(patch, effect, input, postLimit)
		if err != nil {
			if gitlinkPost && errors.Is(err, gitutil.ErrPatchImageBudget) {
				return nil, fmt.Errorf("%s: invalid gitlink payload", effect.Path)
			}
			var proof *gitutil.PatchImageBudgetProof
			if errors.As(err, &proof) {
				if !postExtant && proof.Size != 0 {
					return nil, fmt.Errorf("%s: deletion leaves reference content", effect.Path)
				}
				row.Post = ReconstructedSide{
					Present: postExtant, Mode: mode, SHA256: proof.SHA256,
					DigestProved: true, NULPresent: proof.NULPresent,
					Limitation: "postimage independently verified by streaming; body not retained within image budget",
				}
				result = append(result, row)
				continue
			}
			if errors.Is(err, gitutil.ErrPatchImageBudget) {
				row.Post.Limitation = err.Error()
				result = append(result, row)
				continue
			}
			return nil, fmt.Errorf("%s: %w", effect.Path, err)
		}
		if !available {
			if postExtant {
				row.Post = ReconstructedSide{Limitation: "strict binary representation has no reconstructable postimage payload", PayloadAbsent: true}
			}
		} else if postExtant {
			if gitlinkPost {
				id := strings.TrimSuffix(strings.TrimPrefix(string(post), "Subproject commit "), "\n")
				if !isFullCommitHex(id) || string(post) != "Subproject commit "+id+"\n" {
					return nil, fmt.Errorf("%s: invalid gitlink payload", effect.Path)
				}
				row.Post = ReconstructedSide{Available: true, Present: true, Mode: mode, SHA256: sha256Hex([]byte(id))}
			} else {
				row.Post = ReconstructedSide{Available: true, Present: true, Mode: mode, Bytes: post, SHA256: sha256Hex(post)}
				if len(post) != 0 && (len(pre.Bytes) == 0 || &post[0] != &pre.Bytes[0]) {
					budget.used += int64(len(post))
				}
			}
		}
		result = append(result, row)
	}
	return result, nil
}
