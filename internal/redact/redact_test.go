// Shared-redaction tests (ADR-033 D4 / PRD §8.2).
//
// Two policies share one matcher inventory, and both are pinned here:
// the ten session classes must behave exactly as they did before the
// extraction, and the six closed resource classes must catch every
// shape ADR-033 D4 enumerates.

package redact

import (
	"strings"
	"testing"
)

// TestSessionClassInventoryUnchanged pins the ten session classes and
// their order, which audit finding-code lists depend on.
func TestSessionClassInventoryUnchanged(t *testing.T) {
	want := []string{
		"secret-like-string",
		"absolute-home-path",
		"prompt-text-marker",
		"tool-call-argument",
		"command-output-marker",
		"stack-trace-marker",
		"ide-buffer-marker",
		"clipboard-marker",
		"vector-embedding-payload",
		"source-snippet-marker",
	}
	if len(SessionClasses) != len(want) {
		t.Fatalf("session classes = %d, want %d", len(SessionClasses), len(want))
	}
	for i, code := range want {
		if SessionClasses[i].Code != code {
			t.Fatalf("session class %d = %q, want %q", i, SessionClasses[i].Code, code)
		}
	}
}

// TestSessionMatchersUnchanged spot-checks each session class against a
// representative positive and a representative negative, so the
// extraction cannot have altered behaviour.
func TestSessionMatchersUnchanged(t *testing.T) {
	cases := []struct {
		code     string
		positive string
		negative string
	}{
		{"secret-like-string", "key=sk-" + strings.Repeat("a", 24), "just a normal sentence"},
		{"absolute-home-path", "wrote /Users/someone/file", "wrote ./relative/file"},
		{"prompt-text-marker", "the system prompt was long", "the summary was long"},
		{"tool-call-argument", `{"arguments": {"a":1}}`, `{"result": 1}`},
		{"command-output-marker", "stdout: hello", "output looked fine"},
		{"stack-trace-marker", "Traceback (most recent call last)", "no stack here"},
		{"ide-buffer-marker", "editor buffer contents", "editor was open"},
		{"clipboard-marker", "clipboard contents pasted", "copied by hand"},
		{"vector-embedding-payload", "[" + strings.TrimSuffix(strings.Repeat("0.1, ", 20), ", ") + "]", "[1, 2, 3]"},
		{"source-snippet-marker", "```go\nfunc main() {}\n```", "```\nplain\n```"},
	}
	byCode := map[string]Class{}
	for _, c := range SessionClasses {
		byCode[c.Code] = c
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			cls, ok := byCode[tc.code]
			if !ok {
				t.Fatalf("no session class %q", tc.code)
			}
			if !cls.Match(tc.positive) {
				t.Fatalf("%q should match %s", tc.positive, tc.code)
			}
			if cls.Match(tc.negative) {
				t.Fatalf("%q should not match %s", tc.negative, tc.code)
			}
		})
	}
}

// TestResourceClassInventoryIsClosedAtSix pins ADR-033 D4's closed set.
func TestResourceClassInventoryIsClosedAtSix(t *testing.T) {
	want := []string{
		ClassPrivateKey,
		ClassConnectionURL,
		ClassEmailPII,
		ClassCredentialAssignment,
		ClassBearerOrKeyToken,
		ClassHomeAbsolutePath,
	}
	if len(ResourceClasses) != 6 {
		t.Fatalf("resource classes = %d, want exactly 6", len(ResourceClasses))
	}
	for i, code := range want {
		if ResourceClasses[i].Code != code {
			t.Fatalf("resource class %d = %q, want %q", i, ResourceClasses[i].Code, code)
		}
	}
}

// TestScanCoversEveryResourceClass exercises all six classes.
func TestScanCoversEveryResourceClass(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"pem-rsa", "-----BEGIN RSA PRIVATE KEY-----\nabc\n", ClassPrivateKey},
		{"pem-generic", "-----BEGIN PRIVATE KEY-----\nabc\n", ClassPrivateKey},
		{"openssh", "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n", ClassPrivateKey},
		{"putty", "PuTTY-User-Key-File-2: ssh-rsa\n", ClassPrivateKey},
		{"postgres-url", "postgres://user:pw@host:5432/db", ClassConnectionURL},
		{"mysql-url", "mysql://host/db", ClassConnectionURL},
		{"mongodb-srv", "mongodb+srv://cluster/db", ClassConnectionURL},
		{"generic-userinfo", "https://user:secret@example.com/path", ClassConnectionURL},
		{"email", "contact: someone@example.com", ClassEmailPII},
		{"credential-assignment", `password = "hunter2hunter2hunter2"`, ClassCredentialAssignment},
		{"bearer", "Authorization: Bearer " + strings.Repeat("x", 24), ClassBearerOrKeyToken},
		{"github-token", "ghp_" + strings.Repeat("a", 24), ClassBearerOrKeyToken},
		{"aws-key", "AKIA" + strings.Repeat("A", 16), ClassBearerOrKeyToken},
		{"home-path", "cache at /home/someone/.cache", ClassHomeAbsolutePath},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			codes := Scan([]byte(tc.content))
			found := false
			for _, c := range codes {
				if c == tc.want {
					found = true
				}
			}
			if !found {
				t.Fatalf("Scan(%q) = %v, want it to include %s", tc.content, codes, tc.want)
			}
		})
	}
}

// TestScanIsDeterministicAndDeduplicated proves the finding list is
// sorted and carries no duplicates.
func TestScanIsDeterministicAndDeduplicated(t *testing.T) {
	content := []byte("someone@example.com and other@example.com and postgres://u:p@h/d")
	first := Scan(content)
	second := Scan(content)
	if strings.Join(first, ",") != strings.Join(second, ",") {
		t.Fatalf("Scan is not deterministic: %v vs %v", first, second)
	}
	seen := map[string]int{}
	for _, c := range first {
		seen[c]++
		if seen[c] > 1 {
			t.Fatalf("duplicate finding code %q in %v", c, first)
		}
	}
	for i := 1; i < len(first); i++ {
		if first[i-1] > first[i] {
			t.Fatalf("findings are not sorted: %v", first)
		}
	}
}

// TestScanAcceptsBenignStructuralContent proves the six classes do not
// fire on the structural values resource capture legitimately handles.
func TestScanAcceptsBenignStructuralContent(t *testing.T) {
	for _, benign := range []string{
		"refs/heads/main",
		"9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		"config/local-secrets.env.template",
		"dolt:diff-summary:users",
		"100644",
		"dolt-diff-summary-v1",
		"A=1\nB=2\n",
	} {
		if codes := ScanString(benign); len(codes) != 0 {
			t.Fatalf("Scan(%q) = %v, want no findings", benign, codes)
		}
	}
}

// TestScanTakesBytesNotAPath is a structural guarantee: Scan's only
// input is in-memory content, so "raw bytes are never written to disk
// before scanning" cannot be violated by a later call-site change.
func TestScanTakesBytesNotAPath(t *testing.T) {
	if codes := Scan(nil); codes != nil {
		t.Fatalf("Scan(nil) = %v, want nil", codes)
	}
	if codes := Scan([]byte{}); codes != nil {
		t.Fatalf("Scan(empty) = %v, want nil", codes)
	}
}

// TestMatchAnyPreservesClassOrder proves the shared primitive returns
// codes in the class list's own order, which the session audit trail
// relies on.
func TestMatchAnyPreservesClassOrder(t *testing.T) {
	content := "/Users/someone/file with a system prompt inside"
	codes := MatchAny(SessionClasses, content)
	if len(codes) < 2 {
		t.Fatalf("want at least two findings, got %v", codes)
	}
	if codes[0] != "absolute-home-path" || codes[1] != "prompt-text-marker" {
		t.Fatalf("class order not preserved: %v", codes)
	}
}
