package workflow

import (
	"fmt"
	"strings"
)

// ExtractJSONObject pulls a single balanced JSON object or array out of a
// possibly-noisy LLM response. It is tolerant of:
//
//   - Markdown code fences (```json ... ``` or bare ``` ... ```).
//   - Leading prose ("Sure, here's the recipe: { ... }").
//   - Trailing prose ("{ ... }\n\nExplanation: ...") — the previous
//     implementation handed the trailing prose to json.Unmarshal, which
//     blew up with "invalid character 'E' after top-level value". That
//     failure combined with MaxRetries=2 caused silent heuristic
//     fallback even for near-perfect responses. This helper scans from
//     the first '{' or '[' forward, tracking string state and balanced
//     braces/brackets, and returns exactly the matched span.
//
// Strings inside the JSON are parsed enough to ignore braces inside
// quoted strings and to skip escape sequences (so `"foo\"}bar"` does
// not fool the brace counter).
//
// If no balanced object/array is found, the function returns an error
// and the original string trimmed of surrounding fences (best-effort)
// so callers can still log something useful.
func ExtractJSONObject(s string) (string, error) {
	trimmed := stripCodeFences(s)
	start := -1
	var open, close byte
	for i := 0; i < len(trimmed); i++ {
		c := trimmed[i]
		if c == '{' {
			start, open, close = i, '{', '}'
			break
		}
		if c == '[' {
			start, open, close = i, '[', ']'
			break
		}
	}
	if start < 0 {
		return strings.TrimSpace(trimmed), fmt.Errorf("no JSON object or array found in response")
	}

	depth := 0
	inString := false
	escape := false
	for i := start; i < len(trimmed); i++ {
		c := trimmed[i]
		if escape {
			escape = false
			continue
		}
		if inString {
			switch c {
			case '\\':
				escape = true
			case '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return trimmed[start : i+1], nil
			}
		}
	}
	return strings.TrimSpace(trimmed[start:]), fmt.Errorf("unbalanced JSON: no matching %q for opening at offset %d", close, start)
}

// stripCodeFences removes surrounding markdown fences ( ```json ... ```
// or ``` ... ``` ). If there is only a leading fence with no closing
// fence, it still strips the leading marker so the forward scan can
// proceed. Leaves leading/trailing prose in place — the brace scanner
// handles that.
func stripCodeFences(s string) string {
	if idx := strings.Index(s, "```json"); idx >= 0 {
		s = s[idx+len("```json"):]
		if end := strings.Index(s, "```"); end >= 0 {
			s = s[:end]
		}
		return s
	}
	if idx := strings.Index(s, "```"); idx >= 0 {
		s = s[idx+3:]
		if end := strings.Index(s, "```"); end >= 0 {
			s = s[:end]
		}
	}
	return s
}
