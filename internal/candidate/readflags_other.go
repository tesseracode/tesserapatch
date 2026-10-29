//go:build !linux && !darwin

package candidate

func candidateReadOpenFlags() int {
	return 0
}
