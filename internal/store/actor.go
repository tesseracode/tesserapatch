package store

import (
	"os"
	"os/exec"
	"strings"
)

// ActorUnknown is the terminal fallback in the actor-resolution
// precedence chain (ADR-031 D9, PRD §6 "Actor resolution precedence").
const ActorUnknown = "unknown"

// ActorEnvVar is the environment variable consulted at precedence
// tier 2.
const ActorEnvVar = "TPATCH_ACTOR"

// gitConfigUserEmail is a package-level hook so tests can exercise the
// precedence chain without depending on the ambient git configuration.
var gitConfigUserEmail = func(repoRoot string) string {
	cmd := exec.Command("git", "config", "user.email")
	if repoRoot != "" {
		cmd.Dir = repoRoot
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ResolveActor resolves the `rejected_by` / reopen actor via the fixed
// precedence chain from ADR-031 D9:
//
//  1. the `--actor` flag value, if non-empty;
//  2. else $TPATCH_ACTOR, if set and non-empty;
//  3. else `git config user.email` in the current working directory;
//  4. else the literal string "unknown".
//
// It deliberately does NOT derive from the OS username or hostname
// (privacy), and is unrelated to the commit-trailer attribution
// convention (which attributes commits, not status mutations).
//
// Prefer ResolveActorIn when a target repository root is known — the
// git-config tier should read the repository being operated on, not
// whatever directory the process happens to be in.
func ResolveActor(flagActor string) string {
	return ResolveActorIn(flagActor, "")
}

// ResolveActorIn is ResolveActor with an explicit repository root for
// the `git config user.email` precedence tier.
func ResolveActorIn(flagActor, repoRoot string) string {
	if a := strings.TrimSpace(flagActor); a != "" {
		return a
	}
	if env := strings.TrimSpace(os.Getenv(ActorEnvVar)); env != "" {
		return env
	}
	if email := strings.TrimSpace(gitConfigUserEmail(repoRoot)); email != "" {
		return email
	}
	return ActorUnknown
}
