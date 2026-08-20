//go:build !((linux && !android) || (darwin && !ios))

package intentpub

import "io/fs"

type tempCleanupIdentity struct{}

func descriptorTempCleanupSupported() bool {
	return canRemoveAtHeldDirectory()
}

func canRemoveAtHeldDirectory() bool {
	return false
}

func retainAndVerifyTemp(RootFile, RootFile, string) (RootFile, tempCleanupIdentity, error) {
	return nil, tempCleanupIdentity{}, fs.ErrInvalid
}

func verifyTempAtHeldDirectory(RootFile, RootFile, string, tempCleanupIdentity) (tempCleanupIdentity, error) {
	return tempCleanupIdentity{}, fs.ErrInvalid
}

func removeVerifiedTempAtHeldDirectory(RootFile, RootFile, string, tempCleanupIdentity) error {
	return fs.ErrInvalid
}

func verifyTempContentAtHeldDirectory(
	RootFile,
	RootFile,
	string,
	tempCleanupIdentity,
	Identity,
	[]byte,
) error {
	return fs.ErrInvalid
}
