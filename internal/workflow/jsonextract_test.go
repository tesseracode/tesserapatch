package workflow

import (
	"encoding/json"
	"strings"
	"testing"
)

// Regression tests for bug-extract-json-robustness. The previous
// extractor lopped everything before the first '{' but let trailing
// prose through, so json.Unmarshal kept choking on near-perfect LLM
// responses — and with MaxRetries=2 that meant silent heuristic
// fallback three times in a row.
func TestExtractJSONObject(t *testing.T) {
	tests := []struct {
		name, input, want string
		wantErr           bool
	}{
		{
			name:  "plain object",
			input: `{"a":1}`,
			want:  `{"a":1}`,
		},
		{
			name:  "trailing prose after object",
			input: `{"a":1,"b":"hi"}` + "\n\nExplanation: I chose A because…",
			want:  `{"a":1,"b":"hi"}`,
		},
		{
			name:  "leading prose before object",
			input: "Sure, here's the recipe:\n{\n  \"a\": 1\n}\n",
			want:  "{\n  \"a\": 1\n}",
		},
		{
			name:  "json code fence",
			input: "```json\n{\"a\":1}\n```\n\nDone.",
			want:  "{\"a\":1}",
		},
		{
			name:  "bare code fence",
			input: "```\n{\"a\":1}\n```",
			want:  "{\"a\":1}",
		},
		{
			name:  "braces inside strings do not unbalance",
			input: `{"snippet":"if (x) {return 1;}","n":2}extra`,
			want:  `{"snippet":"if (x) {return 1;}","n":2}`,
		},
		{
			name:  "escaped quote inside string",
			input: `{"msg":"he said \"} hello\"","ok":true} trailing`,
			want:  `{"msg":"he said \"} hello\"","ok":true}`,
		},
		{
			name:  "nested objects",
			input: `{"outer":{"inner":{"x":1}},"done":true}  `,
			want:  `{"outer":{"inner":{"x":1}},"done":true}`,
		},
		{
			name:  "top-level array",
			input: `[1,2,{"x":3}]  tail`,
			want:  `[1,2,{"x":3}]`,
		},
		{
			name:    "no json at all",
			input:   "Sorry, I could not comply.",
			wantErr: true,
		},
		{
			name:    "unbalanced",
			input:   `{"a":1,`,
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractJSONObject(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got none; span=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("\n got:  %q\n want: %q", got, tc.want)
			}
			// Extracted spans must also parse as JSON — the whole point.
			var v any
			if err := json.Unmarshal([]byte(strings.TrimSpace(got)), &v); err != nil {
				t.Fatalf("extracted span is not valid JSON: %v\nspan: %q", err, got)
			}
		})
	}
}

// End-to-end regression through JSONObjectValidator: a response with
// trailing prose (the canonical failure mode) must now validate cleanly
// rather than forcing a retry.
func TestJSONObjectValidatorToleratesTrailingProse(t *testing.T) {
	var target struct {
		Decision string `json:"decision"`
	}
	v := JSONObjectValidator(&target)
	resp := `{"decision":"approve"}

Explanation: looks good to me.`
	if err := v(resp); err != nil {
		t.Fatalf("validator rejected response with trailing prose: %v", err)
	}
	if target.Decision != "approve" {
		t.Fatalf("want decision=approve, got %q", target.Decision)
	}
}
