//go:build !(unix || windows)

package intent

// This value is never used in production on unsupported targets: the CLI
// refuses before it opens a root. It exists so those targets remain buildable.
func openFlags() int {
	return 0
}
