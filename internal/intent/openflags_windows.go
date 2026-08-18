//go:build windows

package intent

// openFlags returns the extra open flags for the final leaf on Windows: none.
// Reparse points are refused before the open by the Root.Lstat mode test, so
// no caller-side flag is needed (PRD §7.4.3).
func openFlags() int {
	return 0
}
