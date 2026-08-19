//go:build unix

package intentpub

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
)

func TestExactModesUnderUmask(t *testing.T) {
	if os.Getenv("TPATCH_INTENTPUB_UMASK_HELPER") != "1" {
		command := exec.Command(os.Args[0], "-test.run=^TestExactModesUnderUmask$")
		command.Env = append(os.Environ(), "TPATCH_INTENTPUB_UMASK_HELPER=1")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("umask subprocess failed: %v\n%s", err, output)
		}
		return
	}

	oldMask := syscall.Umask(0o077)
	defer syscall.Umask(oldMask)
	_, authority := acquireWorkspace(t)
	stage, err := Stage(authority, testSlug, []StageInput{{
		ArtifactID: ArtifactSpec,
		Rel:        "spec.md",
		Data:       []byte("spec"),
		Mode:       0o644,
	}}, Options{RandomHex12: sequenceHex()})
	if err != nil {
		t.Fatal(err)
	}
	for rel, mode := range map[string]os.FileMode{
		".tpatch":                      0o755,
		".tpatch/local":                0o700,
		".tpatch/local/intent-prepare": 0o700,
		laneRel(testSlug):              0o700,
		stage.StageRel:                 0o700,
		stage.Files[0].Rel:             0o600,
	} {
		assertMode(t, authority, rel, mode)
	}

	target := ".tpatch/features/modes/value.json"
	if _, err := DurableWrite(authority, WriteRequest{
		Rel:  target,
		Data: []byte("value"),
		Mode: 0o640,
	}, Options{RandomHex12: sequenceHex()}); err != nil {
		t.Fatal(err)
	}
	for rel, mode := range map[string]os.FileMode{
		".tpatch/features":       0o755,
		".tpatch/features/modes": 0o755,
		target:                   0o640,
	} {
		assertMode(t, authority, rel, mode)
	}

	if err := authority.WithRoot(func(root *os.Root) error {
		directory, err := root.Open(".tpatch/features/modes")
		if err != nil {
			return err
		}
		if err := directory.Chmod(0o711); err != nil {
			_ = directory.Close()
			return err
		}
		return directory.Close()
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := DurableWrite(authority, WriteRequest{
		Rel:           ".tpatch/features/modes/second.json",
		Data:          []byte("second"),
		Mode:          0o644,
		RequireParent: true,
	}, Options{RandomHex12: sequenceHex()}); err != nil {
		t.Fatal(err)
	}
	assertMode(t, authority, ".tpatch/features/modes", 0o711)
	assertMode(t, authority, ".tpatch/features/modes/second.json", 0o644)
}
