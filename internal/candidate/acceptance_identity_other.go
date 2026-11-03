//go:build !linux && !darwin && !windows

package candidate

import (
	"fmt"
	"os"
	"runtime"
)

func readAcceptanceDirectoryIdentity(*os.Root) (AcceptanceDirectoryIdentity, error) {
	return AcceptanceDirectoryIdentity{}, fmt.Errorf("durable acceptance directory identity is unsupported on %s", runtime.GOOS)
}
