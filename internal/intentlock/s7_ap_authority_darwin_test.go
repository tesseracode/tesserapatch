//go:build darwin && !ios

package intentlock

import (
	"errors"
	"os"
	"testing"
)

func TestS7APDarwinFilesystemPredicate(t *testing.T) {
	t.Run("PIB-479", func(t *testing.T) {
		for _, name := range []string{"nfs", "smbfs", "webdav", "macfuse", "osxfuse"} {
			class, denied := classifyDarwinFilesystem(name)
			if class != name || !denied {
				t.Fatalf("Darwin denied class %q = (%q,%t)", name, class, denied)
			}
		}
		for _, name := range []string{"sshfs", "sshfs-extra", "mysshfs", "nfs4", "smbfs-extra"} {
			class, denied := classifyDarwinFilesystem(name)
			if class != name || denied {
				t.Fatalf("Darwin unknown-local class %q = (%q,%t)", name, class, denied)
			}
		}

		root := t.TempDir()
		classified := 0
		classifier := func(file *os.File) (string, bool, error) {
			classified++
			info, err := file.Stat()
			if err != nil || !info.IsDir() {
				return "", false, errors.New("sshfs predicate did not receive held directory")
			}
			class, denied := classifyDarwinFilesystem("sshfs")
			return class, denied, nil
		}
		authority, err := AcquireWithFilesystemClassifier(root, classifier)
		if err != nil || authority == nil || classified != 1 {
			t.Fatalf("sshfs unknown-local acquisition = authority:%v classifies:%d err:%v",
				authority, classified, err)
		}
		contender, contenderErr := AcquireWithFilesystemClassifier(root, classifier)
		if contender != nil {
			_ = contender.Release()
			t.Fatal("sshfs unknown-local acquisition bypassed real flock")
		}
		var typed *Error
		if !errors.As(contenderErr, &typed) || typed.Code != CodeTransactionInProgress {
			t.Fatalf("sshfs real-flock contender = %v", contenderErr)
		}
		if err := authority.Release(); err != nil {
			t.Fatal(err)
		}
	})
}
