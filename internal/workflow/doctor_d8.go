package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strconv"
)

var lineInErrorRe = regexp.MustCompile(`line ([0-9]+)`)

func runDoctorD8(ctx *doctorContext) {
	// Hard invariants are validated before any selected check runs. Keeping D8
	// registered makes `--check D8` a valid, reportable check ID.
}

func lineForJSONError(path string, err error) int {
	if line := lineFromErrorString(err); line > 0 {
		return line
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return 1
	}
	return lineForJSONErrorBytes(data, err)
}

func lineForJSONErrorBytes(data []byte, err error) int {
	if line := lineFromErrorString(err); line > 0 {
		return line
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return bytes.Count(data[:minInt64(int64(len(data)), syntax.Offset)], []byte("\n")) + 1
	}
	return 1
}

func lineFromErrorString(err error) int {
	if err == nil {
		return 0
	}
	m := lineInErrorRe.FindStringSubmatch(err.Error())
	if len(m) != 2 {
		return 0
	}
	line, _ := strconv.Atoi(m[1])
	return line
}

func minInt64(a, b int64) int {
	if a < b {
		return int(a)
	}
	return int(b)
}
