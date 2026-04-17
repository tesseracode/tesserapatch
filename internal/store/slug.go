package store

import (
	"regexp"
	"strings"
	"unicode"
)

var nonAlphanumDash = regexp.MustCompile(`[^a-z0-9-]+`)
var multiDash = regexp.MustCompile(`-{2,}`)

const maxSlugLen = 60

// Slugify converts a string into a kebab-case slug suitable for directory names.
func Slugify(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// Lowercase
	s = strings.ToLower(s)

	// Replace non-ASCII with ASCII approximations (basic)
	var b strings.Builder
	for _, r := range s {
		if r > unicode.MaxASCII {
			b.WriteRune('-')
		} else {
			b.WriteRune(r)
		}
	}
	s = b.String()

	// Replace non-alphanumeric with dashes
	s = nonAlphanumDash.ReplaceAllString(s, "-")

	// Collapse multiple dashes
	s = multiDash.ReplaceAllString(s, "-")

	// Trim leading/trailing dashes
	s = strings.Trim(s, "-")

	// Truncate to max length, on a dash boundary if possible
	if len(s) > maxSlugLen {
		s = s[:maxSlugLen]
		if idx := strings.LastIndex(s, "-"); idx > maxSlugLen/2 {
			s = s[:idx]
		}
	}

	return s
}
