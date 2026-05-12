package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// LoadUpstreamLock reads `.tpatch/upstream.lock` from the store root and
// returns the parsed values. A missing file is reported as `os.ErrNotExist`
// (callers can branch on it via errors.Is). An empty / scaffolded lock
// (the file emitted by `tpatch init`) decodes to a zero-valued
// UpstreamLock with no error — that is the conventional "no lock yet"
// signal across record --auto and reconcile --lock-guard.
func LoadUpstreamLock(s *Store) (UpstreamLock, error) {
	if s == nil {
		return UpstreamLock{}, errors.New("nil store")
	}
	data, err := os.ReadFile(s.upstreamLockPath())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return UpstreamLock{}, err
		}
		return UpstreamLock{}, err
	}
	return ParseUpstreamLock(string(data)), nil
}

// ParseUpstreamLock extracts the four well-known scalar keys from the
// YAML-like content of `.tpatch/upstream.lock`. The lock format is
// intentionally flat: each field is a top-level scalar with optional
// surrounding whitespace and optional surrounding quotes. Lines that
// begin with `#` are treated as comments. Unknown keys are ignored.
// Malformed lines (no colon, or a key with no value) are silently
// skipped — they cannot produce a confidently-typed value.
//
// This matches the zero-dep pattern already used by parseYAMLConfig for
// `.tpatch/config.yaml` and is deliberately shared between the
// record --auto (Wave A1) and reconcile --lock-guard (Wave A2) slices.
func ParseUpstreamLock(content string) UpstreamLock {
	var lock UpstreamLock
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.IndexByte(trimmed, ':')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		val := strings.TrimSpace(trimmed[idx+1:])
		// Strip inline `#` comment after a value.
		if i := strings.Index(val, " #"); i >= 0 {
			val = strings.TrimSpace(val[:i])
		}
		// Unquote — accept either " or '.
		if n := len(val); n >= 2 {
			if (val[0] == '"' && val[n-1] == '"') || (val[0] == '\'' && val[n-1] == '\'') {
				val = val[1 : n-1]
			}
		}
		switch key {
		case "remote":
			lock.Remote = val
		case "branch":
			lock.Branch = val
		case "commit":
			lock.Commit = val
		case "url":
			lock.URL = val
		}
	}
	return lock
}

// UpstreamLockPath returns the absolute path to .tpatch/upstream.lock.
// Exported for callers (workflow/reconcile, record --auto) that need to
// reason about the file's existence without going through LoadUpstreamLock.
func (s *Store) UpstreamLockPath() string {
	return filepath.Join(s.tpatchDir(), "upstream.lock")
}
