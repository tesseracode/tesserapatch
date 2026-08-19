//go:build !(unix || windows)

package intentpub

func openFlags() int {
	return 0
}
