package cli

import "os"

// openDevNull returns an *os.File that LOOKS non-TTY (not a char
// device) suitable for stdin substitution in tests that need a non-TTY
// input stream. Used by the cascade non-TTY refusal test. /dev/null is
// itself a char device on Unix, so we return the read end of an
// os.Pipe() which is a regular pipe (not a char device).
func openDevNull() (*os.File, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	_ = w.Close()
	return r, nil
}
