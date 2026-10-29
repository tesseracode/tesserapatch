package gitutil

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var treeHashPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// TreeHash resolves the tree object ID for ref without touching the index or
// working tree, and with lazy network fetches disabled.
func TreeHash(repoRoot, ref string) (string, error) {
	if strings.TrimSpace(ref) == "" {
		return "", fmt.Errorf("tree hash requires a non-empty ref")
	}
	cmd := exec.Command("git", "rev-parse", "--verify", ref+"^{tree}")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), NoLazyFetchEnv, CLocaleEnv)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --verify %s^{tree}: %v: %s", ref, err, strings.TrimSpace(stderr.String()))
	}
	hash := strings.TrimSpace(string(out))
	if !treeHashPattern.MatchString(hash) {
		return "", fmt.Errorf("git returned a non-canonical tree hash %q for %s", hash, ref)
	}
	return hash, nil
}
