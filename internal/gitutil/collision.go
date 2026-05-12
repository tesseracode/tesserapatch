package gitutil

import (
	"crypto/sha256"
	"encoding/hex"
)

// PatchSignature returns the canonical signature of a patch string used
// by record-time collision detection (PRD-record-collision-detection
// §4): the hex-encoded SHA-256 digest and the byte length.
//
// The signature operates on the patch string exactly as record is about
// to persist it. Callers MUST NOT trim, reorder, or reserialize the
// patch before passing it in — `CapturePatchScoped` and
// `CapturePatchFromCommitsScoped` already return canonical
// newline-terminated strings.
//
// A byte-identical match across two feature directories is a strong
// signal that at least one feature boundary is wrong (see WP-001).
// Defence-in-depth byte comparison after SHA-256 equality is the
// responsibility of the caller; this primitive only computes the
// signature pair.
func PatchSignature(patch string) (sha256Hex string, bytes int) {
	sum := sha256.Sum256([]byte(patch))
	return hex.EncodeToString(sum[:]), len(patch)
}
