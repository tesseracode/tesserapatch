//go:build (linux && !android) || (darwin && !ios)

package intentlock

import (
	"errors"
	"os"
	"testing"
)

func TestAcquireWithFilesystemClassifierRunsRealAcquisitionSequence(t *testing.T) {
	sentinel := errors.New("raw classifier failure")
	tests := []struct {
		name      string
		classify  func(*os.File) (string, bool, error)
		wantCode  Code
		wantClass string
	}{
		{
			name: "denied-nfs",
			classify: func(file *os.File) (string, bool, error) {
				return "nfs", true, nil
			},
			wantCode:  CodeLockFilesystemUnsupported,
			wantClass: "nfs",
		},
		{
			name: "raw-classification-error",
			classify: func(file *os.File) (string, bool, error) {
				return "", false, sentinel
			},
			wantCode:  CodeDirectoryFlockUnavailable,
			wantClass: "filesystem-classification",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var events []string
			restoreDefaultAuthorityOps(t, &events)
			var classified *os.File
			authority, err := AcquireWithFilesystemClassifier(
				t.TempDir(),
				func(file *os.File) (string, bool, error) {
					events = append(events, "raw-classifier")
					assertRetainedDirectory(t, file)
					classified = file
					return test.classify(file)
				},
			)
			if authority != nil {
				_ = authority.Release()
				t.Fatal("refused classifier acquired authority")
			}
			assertCode(t, err, test.wantCode)
			var typed *Error
			if !errors.As(err, &typed) || typed.Class != test.wantClass {
				t.Fatalf("classifier error = %#v, want class %q", typed, test.wantClass)
			}
			want := []string{
				"open-root", "open-directory", "identity", "raw-classifier",
				"close-directory", "close-root",
			}
			if !equalStrings(events, want) {
				t.Fatalf("events = %v, want %v", events, want)
			}
			if classified == nil {
				t.Fatal("classifier did not receive the retained directory")
			}
			if _, statErr := classified.Stat(); statErr == nil {
				t.Fatal("retained directory remained open after refusal")
			}
		})
	}
}

func TestAcquireWithFilesystemClassifierAllowsAndRetainsRealAuthority(t *testing.T) {
	var events []string
	restoreDefaultAuthorityOps(t, &events)
	var classified *os.File
	var classifiedIdentity nativeIdentity
	authority, err := AcquireWithFilesystemClassifier(
		t.TempDir(),
		func(file *os.File) (string, bool, error) {
			events = append(events, "raw-classifier")
			assertRetainedDirectory(t, file)
			classified = file
			var identityErr error
			classifiedIdentity, identityErr = identityFromFile(file)
			return "allowed-real-surrogate", false, identityErr
		},
	)
	if err != nil {
		t.Fatalf("AcquireWithFilesystemClassifier: %v", err)
	}
	if authority == nil {
		t.Fatal("allowed classifier returned nil authority")
	}
	if classified != authority.directory || classifiedIdentity != authority.identity {
		_ = authority.Release()
		t.Fatal("classification, identity capture, and retained authority used different directories")
	}
	wantAcquire := []string{
		"open-root", "open-directory", "identity", "raw-classifier", "flock-control",
	}
	if !equalStrings(events, wantAcquire) {
		_ = authority.Release()
		t.Fatalf("acquire events = %v, want %v", events, wantAcquire)
	}
	if err := authority.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	wantRelease := append(wantAcquire, "unlock-control", "close-directory", "close-root")
	if !equalStrings(events, wantRelease) {
		t.Fatalf("full events = %v, want %v", events, wantRelease)
	}
	if _, statErr := classified.Stat(); statErr == nil {
		t.Fatal("retained directory remained open after release")
	}
}

func TestAcquireWithFilesystemClassifierRejectsNilDependency(t *testing.T) {
	var events []string
	restoreDefaultAuthorityOps(t, &events)
	authority, err := AcquireWithFilesystemClassifier(t.TempDir(), nil)
	if authority != nil {
		_ = authority.Release()
		t.Fatal("nil classifier acquired authority")
	}
	assertCode(t, err, CodeDirectoryFlockUnavailable)
	var typed *Error
	if !errors.As(err, &typed) || typed.Class != "invalid-test-dependency" {
		t.Fatalf("nil classifier error = %#v", typed)
	}
	if len(events) != 0 {
		t.Fatalf("nil classifier performed acquisition work: %v", events)
	}
}

func restoreDefaultAuthorityOps(t *testing.T, events *[]string) {
	t.Helper()
	original := defaultAuthorityOps
	t.Cleanup(func() { defaultAuthorityOps = original })
	defaultAuthorityOps = instrumentAuthorityOps(original, events)
}

func assertRetainedDirectory(t *testing.T, file *os.File) {
	t.Helper()
	if file == nil {
		t.Fatal("classifier received nil file")
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatalf("classifier received unusable directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("classifier received %v, want directory", info.Mode())
	}
}
