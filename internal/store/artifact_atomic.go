package store

import (
	"fmt"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/safety"
)

// WriteArtifactAtomic preserves WriteArtifact's confinement and patch refusal,
// using the existing single-file replacement machinery. It is not a transaction
// across artifacts.
func (s *Store) WriteArtifactAtomic(slug, name, content string) error {
	target := s.featureArtifactPath(slug, name)
	if err := safety.EnsureSafeRepoPath(s.Root, target); err != nil {
		return fmt.Errorf("unsafe path in WriteArtifactAtomic: %w", err)
	}
	if strings.HasSuffix(name, ".patch") {
		if offending, ok := patchReferencesGitInternal(content); ok {
			return fmt.Errorf("WriteArtifactAtomic refused: %s/%s references repository-internal path %q", slug, name, offending)
		}
	}
	return writeFileAtomic(target, []byte(content), 0o644)
}
