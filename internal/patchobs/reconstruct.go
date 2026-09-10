package patchobs

import (
	"errors"
	"fmt"
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
	effects, err := gitutil.NormalizePatchEffects(patch)
	if err != nil {
		return nil, fmt.Errorf("canonical patch is not reconstructable")
	}
	resolved, ok := resolveCommit(root, commit+"^{commit}")
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
	tree := readTreeEntries(root, commit, paths)
	if !tree.read {
		return nil, fmt.Errorf("cannot read named local reference tree")
	}
	budget := &imageBudget{limit: imageRetentionLimit}
	blobs := readBlobs(root, neededBlobIDs(tree), budget)
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
		post, available, err := gitutil.ReconstructPatchPostimage(patch, effect, input, budget.limit-budget.used)
		if err != nil {
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
			row.Post = ReconstructedSide{Available: true, Present: true, Mode: mode, Bytes: post, SHA256: sha256Hex(post)}
			if len(post) != 0 && (len(pre.Bytes) == 0 || &post[0] != &pre.Bytes[0]) {
				budget.used += int64(len(post))
			}
			if mode == gitutil.ModeGitlink {
				id := strings.TrimSuffix(strings.TrimPrefix(string(post), "Subproject commit "), "\n")
				if !isFullCommitHex(id) || string(post) != "Subproject commit "+id+"\n" {
					return nil, fmt.Errorf("%s: invalid gitlink payload", effect.Path)
				}
				row.Post.Bytes = nil
				row.Post.SHA256 = sha256Hex([]byte(id))
			}
		}
		result = append(result, row)
	}
	return result, nil
}
