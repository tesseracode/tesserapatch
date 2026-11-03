//go:build !linux && !darwin

package workflow

import "os"

func operationCandidateReadFlags() int { return os.O_RDONLY }
