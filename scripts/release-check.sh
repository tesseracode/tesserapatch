#!/bin/sh
# Release-candidate check (AGENTS.md → Wave-Close Checklist, OV-D18).
#
# Usage: sh scripts/release-check.sh <vX.Y.Z> <gated-commit-sha>
#        make release-check RELEASE=vX.Y.Z GATED=<gated-commit-sha>
#
# Run from the repository top level with the release (tag) commit checked
# out, before `git tag -a`. GATED is the commit the wave-close gate
# validated. The CHANGELOG section and the release-guard pin must already
# be in that gated source; the tag commit may differ from it only in the
# tracking files listed below. Every check reads committed bytes and stops
# at the first failure. Exit 0 = PASS, 1 = FAIL, 2 = usage error.
set -u

tracking_files="docs/handoff/CURRENT.md docs/handoff/HISTORY.md docs/supervisor/LOG.md docs/ROADMAP.md"
pin_file="assets/recipe_authority_parity_test.go"

fail() {
	echo "  FAIL: $*"
	echo "=== Release check: FAIL ==="
	exit 1
}

release=${1-}
gated=${2-}
if [ "$#" -ne 2 ] || [ -z "$release" ] || [ -z "$gated" ]; then
	echo "  FAIL: usage: make release-check RELEASE=vX.Y.Z GATED=<gated-commit-sha>"
	exit 2
fi
if ! cdup=$(git rev-parse --show-cdup 2>/dev/null) || [ -n "$cdup" ]; then
	echo "  FAIL: run from the repository top level"
	exit 2
fi

echo "=== Release check: $release (gated $gated) ==="

echo "[1/9] Release version syntax..."
if ! printf '%s\n' "$release" | grep -Eqx 'v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)'; then
	fail "release \`$release\` is not vMAJOR.MINOR.PATCH"
fi
version=${release#v}
echo "  OK"

echo "[2/9] Tracked working tree and index clean..."
if ! changes=$(git status --porcelain=v1 --untracked-files=no 2>/dev/null); then
	fail "git status failed"
fi
if [ -n "$changes" ]; then
	printf '%s\n' "$changes" | sed 's/^/    /'
	fail "uncommitted tracked changes; the tag must point at committed bytes"
fi
echo "  OK"

echo "[3/9] Gated commit exists and is HEAD or an ancestor of HEAD..."
if ! printf '%s\n' "$gated" | grep -Eqx '[0-9a-f]{7,40}'; then
	fail "GATED \`$gated\` is not a 7-40 character lowercase commit SHA"
fi
if ! gated_full=$(git rev-parse --verify --quiet "$gated^{commit}" 2>/dev/null); then
	fail "GATED \`$gated\` does not name a commit"
fi
if ! head_full=$(git rev-parse --verify --quiet "HEAD^{commit}" 2>/dev/null); then
	fail "HEAD does not name a commit"
fi
if ! git merge-base --is-ancestor "$gated_full" "$head_full" 2>/dev/null; then
	fail "GATED $gated_full is not HEAD or an ancestor of HEAD ($head_full)"
fi
echo "  OK ($gated_full)"

echo "[4/9] HEAD differs from the gated commit only in tracking files..."
# Every changed path is compared with the exact tracking names. Pathspec
# excludes are not used: GIT_LITERAL_PATHSPECS turns them into plain names,
# and an exclude also hides a directory that replaced a tracking file.
# Without --no-renames, a source file renamed onto a tracking name would list
# only the tracking name.
if ! changed=$(git diff --no-renames --name-only "$gated_full" "$head_full" 2>/dev/null); then
	fail "git diff failed"
fi
extra=$(printf '%s\n' "$changed" | while IFS= read -r path; do
	[ -n "$path" ] || continue
	allowed=no
	for f in $tracking_files; do
		if [ "$path" = "$f" ]; then
			allowed=yes
		fi
	done
	if [ "$allowed" = no ]; then
		printf '%s\n' "$path"
	fi
done)
if [ -n "$extra" ]; then
	printf '%s\n' "$extra" | sed 's/^/    /'
	fail "the tag commit changes files outside tracking; move them into gated source and re-gate"
fi
echo "  OK"

echo "[5/9] CHANGELOG.md top release heading is $release..."
if ! changelog=$(git show "$head_full:CHANGELOG.md" 2>/dev/null); then
	fail "CHANGELOG.md is not in HEAD"
fi
heading=$(printf '%s\n' "$changelog" | grep -m1 '^## ' || true)
case "$heading" in
"## $release — "?*) ;;
*) fail "top CHANGELOG heading is \`$heading\`; want \`## $release — <date> — <title>\`" ;;
esac
echo "  OK"

echo "[6/9] Release-guard pin in $pin_file is $version..."
if ! pin_body=$(git show "$head_full:$pin_file" 2>/dev/null); then
	fail "$pin_file is not in HEAD"
fi
pins=$(printf '%s\n' "$pin_body" | grep -c -x -F "const rgaCurrentChangelogRelease = \"$version\"" || true)
if [ "$pins" != 1 ]; then
	fail "want exactly one \`const rgaCurrentChangelogRelease = \"$version\"\` line; found ${pins:-0}"
fi
echo "  OK"

echo "[7/9] CURRENT.md \`**Cluster state**:\` is terminal..."
if ! current=$(git show "$head_full:docs/handoff/CURRENT.md" 2>/dev/null); then
	fail "docs/handoff/CURRENT.md is not in HEAD"
fi
states=$(printf '%s\n' "$current" | grep -c -E '^\*\*Cluster state\*\*:' || true)
if [ "$states" != 1 ]; then
	fail "want exactly one \`**Cluster state**:\` field; found ${states:-0}"
fi
token=$(printf '%s\n' "$current" | grep -m1 -E '^\*\*Cluster state\*\*:' | sed -E 's/^\*\*Cluster state\*\*:[[:space:]]*//;s/[[:space:]]*$//' | tr '[:lower:]' '[:upper:]')
case "$token" in
SHIPPED | APPROVED | ACCEPTED | IDLE) ;;
*) fail "Cluster state \`$token\` is not terminal (SHIPPED | APPROVED | ACCEPTED | IDLE)" ;;
esac
echo "  OK ($token)"

echo "[8/9] docs/supervisor/LOG.md records the gated commit..."
gated_short=$(printf '%s' "$gated_full" | cut -c1-7)
if ! log_body=$(git show "$head_full:docs/supervisor/LOG.md" 2>/dev/null); then
	fail "docs/supervisor/LOG.md is not in HEAD"
fi
if ! printf '%s\n' "$log_body" | grep -q -F "$gated_short"; then
	fail "docs/supervisor/LOG.md never mentions $gated_short; record the gate verdict for the gated commit first"
fi
echo "  OK"

echo "[9/9] Tag $release absent locally and on origin..."
if git rev-parse --verify --quiet "refs/tags/$release" >/dev/null 2>&1; then
	fail "tag $release already exists locally"
fi
GIT_TERMINAL_PROMPT=0 git ls-remote --exit-code --tags origin "refs/tags/$release" >/dev/null 2>&1
remote_status=$?
case "$remote_status" in
2) ;;
0) fail "tag $release already exists on origin" ;;
*) fail "could not query origin for tag $release (git ls-remote exited $remote_status)" ;;
esac
echo "  OK"

echo "=== Release check: PASS ==="
