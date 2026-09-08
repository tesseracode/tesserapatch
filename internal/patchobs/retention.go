package patchobs

import (
	"fmt"
	"io"
)

// imageRetentionLimit bounds distinct pre/postimage backing arrays per
// observation, including the worktree and Git read maps. Patch/artifact
// bytes and effect metadata are not image bodies (ADR-038).
const imageRetentionLimit int64 = 32 << 20

type imageBudget struct {
	limit int64
	used  int64
}

func (b *imageBudget) refusal(size int64) string {
	if size < 0 || size > b.limit-b.used {
		return fmt.Sprintf("image retention budget exceeded: body size %d bytes, %d of %d bytes available",
			size, b.limit-b.used, b.limit)
	}
	return ""
}

// readImage allocates only after checking the declared size. There is no
// growable read buffer and failed/short reads do not consume the budget.
func readImage(r io.Reader, size int64, budget *imageBudget) ([]byte, error) {
	if diagnostic := budget.refusal(size); diagnostic != "" {
		return nil, fmt.Errorf("%s", diagnostic)
	}
	body := make([]byte, int(size))
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	budget.used += size
	return body, nil
}

// readWorktreeImage additionally refuses a file that grew after its
// descriptor was statted. A prefix is never an observed whole-file image.
func readWorktreeImage(r io.Reader, size int64, budget *imageBudget) ([]byte, error) {
	body, err := readImage(r, size, budget)
	if err != nil {
		return nil, err
	}
	var probe [1]byte
	n, err := io.ReadFull(r, probe[:])
	if n != 0 || err != io.EOF {
		budget.used -= size
		return nil, fmt.Errorf("image changed size during capture; refusing a partial body")
	}
	return body, nil
}

// cloneObservation isolates a recorder from the producer's mutable Go
// slices. Deduplicated images stay deduplicated in the independent copy,
// so repeated references cannot multiply its retained body budget.
func cloneObservation(obs Observation) Observation {
	type bodyKey struct {
		first *byte
		size  int
	}
	bodies := map[bodyKey][]byte{}
	cloneBytes := func(body []byte) []byte {
		if body == nil {
			return nil
		}
		if len(body) == 0 {
			return []byte{}
		}
		key := bodyKey{first: &body[0], size: len(body)}
		if cloned, ok := bodies[key]; ok {
			return cloned
		}
		cloned := make([]byte, len(body))
		copy(cloned, body)
		bodies[key] = cloned
		return cloned
	}
	cloneStrings := func(values []string) []string {
		if values == nil {
			return nil
		}
		return append(make([]string, 0, len(values)), values...)
	}
	cloneArtifact := func(snap *ArtifactSnapshot) *ArtifactSnapshot {
		if snap == nil {
			return nil
		}
		cloned := *snap
		cloned.Bytes = cloneBytes(snap.Bytes)
		return &cloned
	}
	obs.PatchBytes = cloneBytes(obs.PatchBytes)
	obs.Capture.Pathspecs = cloneStrings(obs.Capture.Pathspecs)
	obs.Capture.ClaimIDs = cloneStrings(obs.Capture.ClaimIDs)
	obs.ParentCreatedPaths = cloneStrings(obs.ParentCreatedPaths)
	obs.Reasons = cloneStrings(obs.Reasons)
	if obs.Effects != nil {
		effects := make([]EffectObservation, len(obs.Effects))
		copy(effects, obs.Effects)
		for i := range effects {
			effects[i].Bytes.Preimage = cloneBytes(effects[i].Bytes.Preimage)
			effects[i].Bytes.Postimage = cloneBytes(effects[i].Bytes.Postimage)
			effects[i].ReasonCodes = cloneStrings(effects[i].ReasonCodes)
			effects[i].Contradictions = cloneStrings(effects[i].Contradictions)
		}
		obs.Effects = effects
	}
	obs.ArtifactBefore = cloneArtifact(obs.ArtifactBefore)
	obs.ArtifactAfter = cloneArtifact(obs.ArtifactAfter)
	return obs
}
