package candidate

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/gitutil"
	"github.com/tesseracode/tesserapatch/internal/patchobs"
)

type validationResult struct {
	effectMap []EffectResolution
	treeHash  string
	diff      string
}

type pathChange struct {
	oldMode    string
	newMode    string
	oldPresent bool
}

func validateCandidate(
	reference, upstream *ReferenceRoot,
	candidateRoot *CandidateRoot,
	patch string,
	effects []gitutil.PatchEffect,
	plan Plan,
) (validationResult, error) {
	effectMap, err := buildEffectResolutionMap(plan)
	if err != nil {
		return validationResult{}, err
	}
	if err := validateCandidateIndexBaseline(candidateRoot); err != nil {
		return validationResult{}, err
	}
	changes, err := changedPaths(candidateRoot)
	if err != nil {
		return validationResult{}, err
	}
	if err := validatePathSet(changes, plan); err != nil {
		return validationResult{}, err
	}
	if err := validateTouchedModes(changes, plan); err != nil {
		return validationResult{}, err
	}
	if err := validateNoConflictMarkers(candidateRoot, plan); err != nil {
		return validationResult{}, err
	}
	if err := validateOperationPayloadPostconditions(upstream, candidateRoot, plan); err != nil {
		return validationResult{}, err
	}
	if err := validateExpectedModeledContent(upstream, candidateRoot, plan); err != nil {
		return validationResult{}, err
	}
	treeBefore, diffBefore, err := snapshotCandidateState(candidateRoot)
	if err != nil {
		return validationResult{}, err
	}
	secondPlan, err := derive(reference, candidateRoot, patch, effects)
	if err != nil {
		return validationResult{}, &refusal{
			code:   "candidate-not-idempotent",
			detail: "second-pass: " + secondPassReason(err),
		}
	}
	if err := validateCandidateIndexBaseline(candidateRoot); err != nil {
		return validationResult{}, err
	}
	treeAfter, diffAfter, err := snapshotCandidateState(candidateRoot)
	if err != nil {
		return validationResult{}, err
	}
	if treeBefore != treeAfter {
		return validationResult{}, &refusal{code: "candidate-not-idempotent", detail: "second-pass changed tree hash"}
	}
	if diffBefore != diffAfter {
		return validationResult{}, &refusal{code: "candidate-not-idempotent", detail: "second-pass changed diff bytes"}
	}
	if err := validateSecondPassPlan(secondPlan, len(effects)); err != nil {
		return validationResult{}, err
	}
	return validationResult{effectMap: effectMap, treeHash: treeAfter, diff: diffAfter}, nil
}

func buildEffectResolutionMap(plan Plan) ([]EffectResolution, error) {
	operationOrdinals := map[int][]int{}
	for i, op := range plan.operations {
		operationOrdinals[op.effectOrdinal] = append(operationOrdinals[op.effectOrdinal], i+1)
	}
	seen := map[int]bool{}
	out := make([]EffectResolution, 0, len(plan.effects))
	for _, effect := range plan.effects {
		if seen[effect.ordinal] {
			return nil, &refusal{code: "effect-unexplained", path: effect.path, detail: "duplicate effect ordinal"}
		}
		seen[effect.ordinal] = true
		switch effect.state {
		case present:
			out = append(out, EffectResolution{
				EffectOrdinal: effect.ordinal,
				Path:          effect.path,
				State:         "absorbed-present",
			})
		case applicable:
			ords := append([]int(nil), operationOrdinals[effect.ordinal]...)
			if len(ords) == 0 {
				return nil, &refusal{code: "effect-unexplained", path: effect.path, detail: "applicable effect has no operations"}
			}
			out = append(out, EffectResolution{
				EffectOrdinal:     effect.ordinal,
				Path:              effect.path,
				State:             "candidate-applicable",
				OperationOrdinals: ords,
			})
		default:
			return nil, &refusal{code: "effect-unexplained", path: effect.path, detail: "effect is neither absorbed-present nor candidate-applicable"}
		}
	}
	return out, nil
}

func validateCandidateIndexBaseline(root *CandidateRoot) error {
	if err := root.ensureAvailable(); err != nil {
		return err
	}
	if err := root.validateReopenedCandidateAuthority(root.commit); err != nil {
		return &refusal{code: "candidate-unmodeled-change", detail: err.Error()}
	}
	_, stderr, err := runGitWithIdentity(root.worktree, []string{"GIT_OPTIONAL_LOCKS=0"},
		"diff", "--cached", "--quiet", "--exit-code", "--no-ext-diff", "--no-textconv", root.commit, "--")
	if err != nil {
		return &refusal{code: "candidate-unmodeled-change", detail: "candidate index differs from bound upstream: " + strings.TrimSpace(stderr)}
	}
	return nil
}

func snapshotCandidateState(root *CandidateRoot) (treeHash, diff string, err error) {
	treeHash, err = root.WorktreeTreeHash()
	if err != nil {
		return "", "", err
	}
	diff, err = gitutil.CapturePatchScopedReadOnly(root.worktree.path, nil)
	if err != nil {
		return "", "", fmt.Errorf("capture candidate diff: %w", err)
	}
	return treeHash, diff, nil
}

func changedPaths(root *CandidateRoot) (map[string]pathChange, error) {
	trackedRaw, stderr, err := runGitWithIdentity(root.worktree, nil, "diff", "--raw", "--no-renames", "-z", "--")
	if err != nil {
		return nil, fmt.Errorf("inspect candidate raw diff: %v: %s", err, strings.TrimSpace(stderr))
	}
	changes := map[string]pathChange{}
	records := bytes.Split(trackedRaw, []byte{0})
	for i := 0; i < len(records); {
		meta := records[i]
		i++
		if len(meta) == 0 {
			continue
		}
		var pathName string
		if parts := bytes.SplitN(meta, []byte{'\t'}, 2); len(parts) == 2 {
			meta, pathName = parts[0], string(parts[1])
		} else {
			if i >= len(records) {
				return nil, fmt.Errorf("parse raw diff record: %q", string(meta))
			}
			pathName = string(records[i])
			i++
		}
		normalized, err := normalizeRelativePath(pathName)
		if err != nil {
			return nil, err
		}
		fields := strings.Fields(string(meta))
		if len(fields) < 5 || !strings.HasPrefix(fields[0], ":") {
			return nil, fmt.Errorf("parse raw diff metadata: %q", string(meta))
		}
		oldMode := strings.TrimPrefix(fields[0], ":")
		newMode := fields[1]
		changes[normalized] = pathChange{oldMode: oldMode, newMode: newMode, oldPresent: oldMode != "000000"}
	}
	untrackedRaw, stderr, err := runGitWithIdentity(root.worktree, nil, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, fmt.Errorf("inspect candidate untracked paths: %v: %s", err, strings.TrimSpace(stderr))
	}
	for _, entry := range bytes.Split(untrackedRaw, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		pathName := string(entry)
		normalized, err := normalizeRelativePath(pathName)
		if err != nil {
			return nil, err
		}
		if _, seen := changes[normalized]; seen {
			continue
		}
		info, err := root.worktree.root.Lstat(normalized)
		if err != nil {
			return nil, err
		}
		changes[normalized] = pathChange{oldMode: "000000", newMode: fileModeToGitMode(info.Mode()), oldPresent: false}
	}
	return changes, nil
}

func validatePathSet(changes map[string]pathChange, plan Plan) error {
	derived := map[string]bool{}
	for _, op := range plan.operations {
		derived[op.path] = true
	}
	for path := range changes {
		if !derived[path] {
			return &refusal{code: "candidate-path-set-mismatch", path: path, detail: "path differs but is not in the derived set"}
		}
	}
	for path := range derived {
		if _, ok := changes[path]; !ok {
			return &refusal{code: "candidate-path-set-mismatch", path: path, detail: "derived path does not differ from upstream"}
		}
	}
	return nil
}

func validateTouchedModes(changes map[string]pathChange, plan Plan) error {
	paths := map[string]bool{}
	for _, op := range plan.operations {
		paths[op.path] = true
	}
	for path := range paths {
		change, ok := changes[path]
		if !ok {
			continue
		}
		if change.newMode != gitutil.ModeRegular || (change.oldPresent && change.oldMode != gitutil.ModeRegular) {
			return &refusal{code: "candidate-mode-drift", path: path}
		}
	}
	return nil
}

func validateNoConflictMarkers(root *CandidateRoot, plan Plan) error {
	paths := touchedPaths(plan)
	for _, path := range paths {
		body, err := root.ReadFile(path)
		if err != nil {
			return &refusal{code: "candidate-mode-drift", path: path, detail: err.Error()}
		}
		for _, line := range bytes.Split(body, []byte{'\n'}) {
			line = bytes.TrimSuffix(line, []byte{'\r'})
			if bytes.HasPrefix(line, []byte("<<<<<<<")) ||
				bytes.HasPrefix(line, []byte("=======")) ||
				bytes.HasPrefix(line, []byte(">>>>>>>")) {
				return &refusal{code: "candidate-conflict-marker", path: path}
			}
		}
	}
	return nil
}

func validateOperationPayloadPostconditions(upstream *ReferenceRoot, root *CandidateRoot, plan Plan) error {
	opsByPath := map[string][]operation{}
	for _, op := range plan.operations {
		opsByPath[op.path] = append(opsByPath[op.path], op)
	}
	paths := make([]string, 0, len(opsByPath))
	for path := range opsByPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		upstreamBody, err := upstream.ReadFile(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				upstreamBody = nil
			} else {
				return &refusal{code: "candidate-postcondition-failed", path: path, detail: fmt.Sprintf("read upstream path: %v", err)}
			}
		}
		got, err := root.ReadFile(path)
		if err != nil {
			return &refusal{code: "candidate-postcondition-failed", path: path, detail: fmt.Sprintf("read candidate path: %v", err)}
		}
		pathOps := opsByPath[path]
		baseLen := len(upstreamBody)
		createSeen := false
		insertions := make([]operation, 0, len(pathOps))
		for _, op := range pathOps {
			switch op.kind {
			case "create-file":
				if createSeen {
					return &refusal{code: "candidate-postcondition-failed", path: path, detail: "multiple create-file operations for one path"}
				}
				createSeen = true
				baseLen = len(op.payload)
				if len(pathOps) == 1 && !bytes.Equal(got, op.payload) {
					return &refusal{code: "candidate-postcondition-failed", path: path, detail: "created payload mismatch"}
				}
			case "insert-by-boundary":
				insertions = append(insertions, op)
			default:
				return &refusal{code: "candidate-operation-kind-unsupported", path: op.path, detail: op.kind}
			}
		}
		for i, op := range insertions {
			if op.byteOffset < 0 || op.byteOffset > baseLen {
				return &refusal{code: "candidate-postcondition-failed", path: op.path, detail: "insertion offset is outside upstream content"}
			}
			shift := 0
			for j := i + 1; j < len(insertions); j++ {
				other := insertions[j]
				if other.byteOffset < op.byteOffset || other.byteOffset == op.byteOffset {
					shift += len(other.payload)
				}
			}
			finalOffset := op.byteOffset + shift
			finalEnd := finalOffset + len(op.payload)
			if finalOffset < 0 || finalEnd > len(got) {
				return &refusal{
					code:   "candidate-postcondition-failed",
					path:   op.path,
					detail: fmt.Sprintf("inserted payload missing at final offset=%d len=%d", finalOffset, len(op.payload)),
				}
			}
			if !bytes.Equal(got[finalOffset:finalEnd], op.payload) {
				return &refusal{
					code:   "candidate-postcondition-failed",
					path:   op.path,
					detail: fmt.Sprintf("inserted payload mismatch at final offset=%d len=%d", finalOffset, len(op.payload)),
				}
			}
		}
	}
	return nil
}

func validateExpectedModeledContent(upstream *ReferenceRoot, root *CandidateRoot, plan Plan) error {
	opsByPath := map[string][]operation{}
	for _, op := range plan.operations {
		opsByPath[op.path] = append(opsByPath[op.path], op)
	}
	paths := make([]string, 0, len(opsByPath))
	for path := range opsByPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		body, err := upstream.ReadFile(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				body = nil
			} else {
				return &refusal{code: "candidate-unmodeled-change", path: path, detail: fmt.Sprintf("read upstream path: %v", err)}
			}
		}
		current := append([]byte(nil), body...)
		for _, op := range opsByPath[path] {
			switch op.kind {
			case "create-file":
				current = append([]byte(nil), op.payload...)
			case "insert-by-boundary":
				if op.byteOffset < 0 || op.byteOffset > len(current) {
					return &refusal{code: "candidate-unmodeled-change", path: op.path, detail: "modeled insertion offset is outside expected content"}
				}
				next := make([]byte, 0, len(current)+len(op.payload))
				next = append(next, current[:op.byteOffset]...)
				next = append(next, op.payload...)
				next = append(next, current[op.byteOffset:]...)
				current = next
			default:
				return &refusal{code: "candidate-operation-kind-unsupported", path: op.path, detail: op.kind}
			}
		}
		got, err := root.ReadFile(path)
		if err != nil {
			return &refusal{code: "candidate-unmodeled-change", path: path, detail: fmt.Sprintf("read candidate path: %v", err)}
		}
		if !bytes.Equal(got, current) {
			return &refusal{code: "candidate-unmodeled-change", path: path}
		}
	}
	return nil
}

func validateSecondPassPlan(plan Plan, effectCount int) error {
	if len(plan.effects) != effectCount {
		return &refusal{
			code:   "candidate-not-idempotent",
			detail: fmt.Sprintf("second-pass effect count=%d want=%d", len(plan.effects), effectCount),
		}
	}
	presentCount, applicableCount, conflictCount := plan.Counts()
	if presentCount == effectCount && applicableCount == 0 && conflictCount == 0 {
		return nil
	}
	detail := fmt.Sprintf("second-pass counts present=%d applicable=%d conflict=%d", presentCount, applicableCount, conflictCount)
	for _, effect := range plan.effects {
		if effect.state == present {
			continue
		}
		reason := effect.reason
		if reason == "" {
			reason = "unspecified"
		}
		detail = fmt.Sprintf("%s; first non-present effect=%d path=%s state=%s reason=%s", detail, effect.ordinal, effect.path, effect.state, reason)
		break
	}
	return &refusal{code: "candidate-not-idempotent", detail: detail}
}

func secondPassReason(err error) string {
	var refusalErr *refusal
	if errors.As(err, &refusalErr) {
		parts := []string{refusalErr.code}
		if refusalErr.path != "" {
			parts = append(parts, "path="+refusalErr.path)
		}
		if refusalErr.detail != "" {
			parts = append(parts, refusalErr.detail)
		}
		return strings.Join(parts, " ")
	}
	return err.Error()
}

func touchedPaths(plan Plan) []string {
	paths := map[string]bool{}
	for _, op := range plan.operations {
		paths[op.path] = true
	}
	out := make([]string, 0, len(paths))
	for path := range paths {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func (r *CandidateRoot) snapshotPaths(paths []string) (treeSnapshot, error) {
	if err := r.ensureAvailable(); err != nil {
		return treeSnapshot{}, err
	}
	entries := map[string]snapshotEntry{}
	for _, path := range paths {
		normalized, err := normalizeRelativePath(path)
		if err != nil {
			return treeSnapshot{}, err
		}
		info, err := r.worktree.root.Lstat(normalized)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				entries[path] = snapshotEntry{}
				continue
			}
			return treeSnapshot{}, err
		}
		entries[path] = snapshotEntry{Present: true, Mode: fileModeToGitMode(info.Mode()), Size: info.Size()}
	}
	if identityErr := r.ensureAvailable(); identityErr != nil {
		return treeSnapshot{}, identityErr
	}
	return treeSnapshot{
		entries: entries,
		reader: func(path string, limit int64) ([]byte, error) {
			entry, known := entries[path]
			if !known || !entry.Present {
				return nil, fmt.Errorf("snapshot path is absent")
			}
			if entry.Size > limit {
				return nil, patchobs.ErrCommitObjectLimit
			}
			body, err := r.ReadFile(path)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil, fmt.Errorf("snapshot path is absent")
				}
				return nil, err
			}
			if int64(len(body)) != entry.Size {
				return nil, fmt.Errorf("snapshot object size differs")
			}
			return body, nil
		},
	}, nil
}

func fileModeToGitMode(mode fs.FileMode) string {
	switch {
	case mode.IsRegular() && mode&0o111 != 0:
		return gitutil.ModeExecutable
	case mode.IsRegular():
		return gitutil.ModeRegular
	case mode&fs.ModeSymlink != 0:
		return gitutil.ModeSymlink
	case mode.IsDir():
		return "040000"
	default:
		return ""
	}
}
