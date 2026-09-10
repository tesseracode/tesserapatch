package workflow

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/tesseracode/tesserapatch/internal/safety"
	"github.com/tesseracode/tesserapatch/internal/store"
)

// Only one alias group's image is retained at a time. Operation strings are
// shared; newly read or constructed images are limited before allocation.
const recipePrefixMaxBytes = 8 << 20

var (
	errRecipePrefixLimit      = errors.New("ordered prefix image exceeds 8388608-byte proof limit")
	errRecipePrefixPathSafety = errors.New("ordered prefix path safety")
)

type recipePrefixPreview struct {
	message string
	warning string
	err     error
}

type recipePrefixTarget struct {
	path string
	info os.FileInfo
	err  error
}

type recipePrefixImage struct {
	text      string
	exists    bool
	directory bool
	unknown   error
}

func recipePrefixDrift(slug string, index int, op RecipeOperation, reason string) string {
	return fmt.Sprintf("recipe drift: [%s] op %d %s: exact postimage no-write proof failed: %s; regenerate the recipe against the current tree or reconcile before replay",
		slug, index, op.Path, reason)
}

// Resolve through existing ancestors, without treating a dangling/cyclic link
// as an absent ordinary path. Lexical containment alone cannot prove an alias.
func resolveRecipePrefixTarget(root, path string) recipePrefixTarget {
	target := recipePrefixTarget{path: filepath.Join(root, path)}
	if err := safety.EnsureSafeRepoPath(root, target.path); err != nil {
		target.err = fmt.Errorf("%w: %v", errRecipePrefixPathSafety, err)
		return target
	}
	physicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		target.err = err
		return target
	}
	ancestor := target.path
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(ancestor)
		if err == nil {
			if err := safety.EnsureSafeRepoPath(physicalRoot, resolved); err != nil {
				target.err = fmt.Errorf("%w: %v", errRecipePrefixPathSafety, err)
				return target
			}
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			target.path = resolved
			target.info, err = os.Stat(resolved)
			if err != nil && !os.IsNotExist(err) {
				target.err = err
			}
			return target
		}
		if _, lerr := os.Lstat(ancestor); !os.IsNotExist(lerr) {
			target.err = fmt.Errorf("cannot resolve alias: %w", err)
			return target
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			target.err = err
			return target
		}
		suffix = append(suffix, filepath.Base(ancestor))
		ancestor = parent
	}
}

func sameRecipePrefixTarget(a, b recipePrefixTarget) bool {
	return a.err == nil && b.err == nil &&
		(a.path == b.path || (a.info != nil && b.info != nil && os.SameFile(a.info, b.info)))
}

func readRecipePrefixImage(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, recipePrefixMaxBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > recipePrefixMaxBytes {
		return "", errRecipePrefixLimit
	}
	return string(data), nil
}

// maxBytes == 0 is the unchanged executor's unbounded first-replacement
// semantics. The proof uses the same helper, checking growth before allocation.
func replaceRecipeText(text string, op RecipeOperation, maxBytes int) (string, int, error) {
	at, err := recipeReplacementIndex(text, op)
	if err != nil {
		return "", at, err
	}
	if maxBytes > 0 && (len(op.Replace) > maxBytes || len(text)-len(op.Search) > maxBytes-len(op.Replace)) {
		return "", at, errRecipePrefixLimit
	}
	return strings.Replace(text, op.Search, op.Replace, 1), at, nil
}

func recipeReplacementIndex(text string, op RecipeOperation) (int, error) {
	at := strings.Index(text, op.Search)
	if at < 0 {
		return at, fmt.Errorf("search text not found in %s", op.Path)
	}
	return at, nil
}

func proveRecipePrefix(s *store.Store, recipe ApplyRecipe, candidates map[int]bool, out *PreimagePrecheckResult) {
	last := -1
	for i := range candidates {
		last = max(last, i)
	}
	if last < 0 {
		return
	}
	targets := make([]recipePrefixTarget, last+1)
	for i, op := range recipe.Operations[:last+1] {
		targets[i] = resolveRecipePrefixTarget(s.Root, op.Path)
		if errors.Is(targets[i].err, errRecipePrefixPathSafety) {
			out.Errors = append(out.Errors, fmt.Sprintf("recipe drift: [%s] op %d %s: %v",
				recipe.Feature, i, op.Path, targets[i].err))
		}
	}
	if len(out.Errors) != 0 {
		return
	}
	done := make(map[int]bool)
	failures := make(map[int]string)
	for anchor := 0; anchor <= last; anchor++ {
		if !candidates[anchor] || done[anchor] {
			continue
		}
		target := targets[anchor]
		groupLast, groupSize := anchor, 0
		for i := range targets {
			if sameRecipePrefixTarget(target, targets[i]) {
				if candidates[i] {
					groupLast = i
				}
			}
		}
		for i := 0; i <= groupLast; i++ {
			if sameRecipePrefixTarget(target, targets[i]) {
				groupSize++
			}
		}
		image := recipePrefixImage{unknown: target.err}
		if target.err == nil && target.info != nil {
			image.exists, image.directory = true, target.info.IsDir()
			if target.info.Mode().IsRegular() {
				image.text, image.unknown = readRecipePrefixImage(target.path)
			} else {
				image.unknown = errors.New("target is not a regular file")
			}
		}
		initiallyAbsent := target.info == nil
		topologyUnknown := false
		for i, op := range recipe.Operations[:groupLast+1] {
			same := sameRecipePrefixTarget(target, targets[i])
			// An unresolved prefix alias cannot be assumed unrelated. For
			// absent groups, cross-path structural changes also require a
			// topology proof; they cannot create postimage-only permission.
			knownFailure := errors.Is(targets[i].err, syscall.ENOTDIR)
			if (targets[i].err != nil && !knownFailure) || (initiallyAbsent && !same && !knownFailure &&
				(strings.HasPrefix(target.path, targets[i].path+string(filepath.Separator)) ||
					strings.HasPrefix(targets[i].path, target.path+string(filepath.Separator)))) {
				topologyUnknown = true
			}
			if !same && i != anchor {
				continue
			}
			if topologyUnknown {
				image.text = ""
				image.unknown = errors.New("prefix alias or absent-target topology is unproven")
			}
			if candidates[i] {
				done[i] = true
				if image.unknown == nil && image.exists && !image.directory && image.text == op.Content {
					out.AlreadyPresent[i] = true
					continue
				}
				if !out.WriteAuthorized[i] {
					reason := "preceding operations invalidate the initial exact postimage"
					if image.unknown != nil {
						reason = image.unknown.Error()
					}
					failures[i] = recipePrefixDrift(recipe.Feature, i, op, reason)
				}
			}
			preview, proved := projectRecipePrefixOperation(s, recipe.Feature, op, &image)
			if proved && groupSize > 1 {
				out.prefixPreview[i] = preview
			}
		}
		// image and its intermediate bodies do not escape this alias group.
	}
	for i := 0; i <= last; i++ {
		if msg := failures[i]; msg != "" {
			out.appendDrift(msg, out.superseded, out.superseder)
		}
	}
}

// Project only the operations that can affect this image. Known operation
// errors leave it unchanged, just as ExecuteRecipe continues after an error.
// Missing-target contextual operations use the original dry-run path, retaining
// its created_by warning behavior without emitting a second advisory here.
func projectRecipePrefixOperation(s *store.Store, slug string, op RecipeOperation, image *recipePrefixImage) (recipePrefixPreview, bool) {
	preview := recipePrefixPreview{}
	if image.unknown != nil {
		return preview, false
	}
	switch op.Type {
	case "write-file":
		if image.directory {
			preview.err = fmt.Errorf("target is a directory: %s", op.Path)
			return preview, true
		}
		if len(op.Content) > recipePrefixMaxBytes {
			image.text, image.unknown = "", errRecipePrefixLimit
			return preview, false
		}
		// MkdirAll cannot turn an existing ancestor file into a directory.
		for parent := filepath.Dir(filepath.Join(s.Root, op.Path)); ; parent = filepath.Dir(parent) {
			if info, err := os.Stat(parent); err == nil {
				if !info.IsDir() {
					preview.err = fmt.Errorf("parent is not a directory: %s", op.Path)
					return preview, true
				}
				break
			} else if !os.IsNotExist(err) || filepath.Dir(parent) == parent {
				image.unknown = err
				return preview, false
			}
		}
		preview.message = fmt.Sprintf("[write-file] would write %s (%d bytes)", op.Path, len(op.Content))
		image.text, image.exists = op.Content, true
	case "append-file", "replace-in-file":
		if !image.exists {
			return preview, false
		}
		if err := checkCreatedByGate(s, slug, op, true); err != nil {
			preview.err = err
			return preview, true
		}
		if image.directory {
			preview.err = fmt.Errorf("target is a directory: %s", op.Path)
			return preview, true
		}
		if op.Type == "append-file" {
			if len(op.Content) > recipePrefixMaxBytes-len(image.text) {
				image.text, image.unknown = "", errRecipePrefixLimit
				return preview, false
			}
			preview.message = fmt.Sprintf("[append-file] would append to %s (%d bytes)", op.Path, len(op.Content))
			image.text += op.Content
		} else {
			replaced, at, err := replaceRecipeText(image.text, op, recipePrefixMaxBytes)
			if errors.Is(err, errRecipePrefixLimit) {
				image.text, image.unknown = "", err
				return preview, false
			}
			if err != nil {
				preview.err = err
				return preview, true
			}
			line := strings.Count(image.text[:at], "\n") + 1
			preview.message = fmt.Sprintf("[replace-in-file] would replace in %s (match at line %d)", op.Path, line)
			image.text = replaced
		}
	case "ensure-directory":
		if image.exists && !image.directory {
			preview.err = fmt.Errorf("target is not a directory: %s", op.Path)
			return preview, true
		}
		if image.directory {
			preview.message = fmt.Sprintf("[ensure-directory] %s already exists", op.Path)
		} else {
			preview.message = fmt.Sprintf("[ensure-directory] would create %s", op.Path)
		}
		image.exists, image.directory = true, true
	default:
		preview.err = fmt.Errorf("unknown operation type %q", op.Type)
	}
	return preview, true
}

func recipeNoopStillPresent(root string, op RecipeOperation) (bool, error) {
	target := resolveRecipePrefixTarget(root, op.Path)
	if target.err != nil {
		return false, target.err
	}
	if target.info == nil || !target.info.Mode().IsRegular() {
		return false, errors.New("ordered no-write target is missing or not a regular file")
	}
	text, err := readRecipePrefixImage(target.path)
	return err == nil && text == op.Content, err
}
