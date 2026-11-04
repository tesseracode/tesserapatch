#!/bin/sh
# Wave-close [2/8] forgotten-`git add` sentinel (GH #9).
#
# Usage: sh scripts/wave-close-untracked-sentinel.sh <source-pathspec>...
#
# Run from the repository top level. Prints the indented [2/8] verdict and
# exits 0 for OK, 3 for WARN (untracked source not covered by
# .wave-close-allowlist) and any other status for FAIL (the sentinel could
# not establish an answer). Git failures fail closed instead of reading as
# an empty, clean result.
#
# Git reports an untracked nested repository as one opaque `dir/` entry and
# never enumerates the files inside it. The sentinel keeps the top-level
# `git ls-files --others` query and supplements it: files inside each
# untracked nested repository are listed with find, pruning every `.git`
# entry, so Git internals are never read and Git never runs inside the
# nested repository. Those paths go into a throwaway index, where the
# parent's exclude rules (including .gitignore files on the path) drop
# ignored paths and Git's own pathspec matcher applies the source pathspecs
# and allowlist patterns; the opaque boundary records are then replaced by
# those files, so nested files are classified exactly as if the nested
# `.git` were absent. The temporary index never touches the real
# index or repository configuration.
set -u

if [ "$#" -eq 0 ]; then
	echo "  FAIL: no source pathspecs supplied"
	exit 2
fi
if ! cdup=$(git rev-parse --show-cdup 2>/dev/null) || [ -n "$cdup" ]; then
	echo "  FAIL: not running at the top level of a Git work tree"
	exit 2
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/wave-close-sentinel.XXXXXX") || {
	echo "  FAIL: could not create a temporary directory"
	exit 2
}
trap 'rm -rf "$work"' EXIT
trap 'exit 2' HUP INT TERM

fail() {
	printf '  FAIL: %s\n' "$1"
	exit 2
}

# The throwaway index must not inherit settings that write beside the real
# index (split index) or consult external watchers.
scratch_git() {
	GIT_INDEX_FILE="$work/index" git -c core.splitIndex=false \
		-c core.fsmonitor=false -c core.untrackedCache=false "$@"
}

allow_patterns=""
if [ -f .wave-close-allowlist ]; then
	allow_patterns=$(sed -e 's/#.*//' -e 's/[[:space:]]*$//' .wave-close-allowlist | sed -e '/^[[:space:]]*$/d') ||
		fail "could not read .wave-close-allowlist"
fi
# Allowlist entries are Git pathspecs: split on whitespace, never shell-glob.
set -f

git ls-files --others --exclude-standard -- "$@" >"$work/top" ||
	fail "git could not enumerate untracked files"
: >"$work/top-allowed"
if [ -n "$allow_patterns" ]; then
	# shellcheck disable=SC2086
	git ls-files --others --exclude-standard -- $allow_patterns >"$work/top-allowed" ||
		fail "git rejected a .wave-close-allowlist pattern"
fi

# Nested-repository supplement. With core.quotePath=false Git quotes only
# names containing control characters, quotes or backslashes; such a
# boundary cannot be inspected safely, so it fails closed.
git -c core.quotePath=false ls-files --others --exclude-standard >"$work/all" ||
	fail "git could not enumerate untracked files"
: >"$work/nested"
: >"$work/nested-allowed"
nested_count=0
newline='
'
tab=$(printf '\t')
empty_blob=$(git hash-object --stdin </dev/null) || fail "git could not hash an empty blob"
: >"$work/found"
while IFS= read -r entry; do
	case "$entry" in
	\"*/\") fail "untracked nested repository $entry has a name the sentinel cannot inspect" ;;
	*/) ;;
	*) continue ;;
	esac
	dir=${entry%/}
	if [ -h "$dir" ] || [ ! -d "$dir" ]; then
		fail "untracked nested repository $entry is not a directory"
	fi
	odd=$(find "./$dir" -name .git -prune -o -name "*$newline*" -print) ||
		fail "could not inspect untracked nested repository $entry"
	if [ -n "$odd" ]; then
		fail "untracked nested repository $entry contains a file name with a newline"
	fi
	find "./$dir" -name .git -prune -o \( -type f -o -type l \) -print >>"$work/found" ||
		fail "could not enumerate untracked nested repository $entry"
	nested_count=$((nested_count + 1))
done <"$work/all"

if [ "$nested_count" -gt 0 ] && [ -s "$work/found" ]; then
	sed -e 's|^\./||' -e "s|^|100644 $empty_blob$tab|" "$work/found" | tr '\n' '\000' >"$work/index-info" ||
		fail "could not stage nested repository paths"
	scratch_git update-index -z --index-info <"$work/index-info" ||
		fail "git rejected a nested repository path"
	scratch_git ls-files -z --cached --ignored --exclude-standard >"$work/ignored" ||
		fail "git could not apply ignore rules to nested repository paths"
	if [ -s "$work/ignored" ]; then
		scratch_git update-index -z --force-remove --stdin <"$work/ignored" ||
			fail "git could not drop ignored nested repository paths"
	fi
	scratch_git ls-files --cached -- "$@" >"$work/nested" ||
		fail "git could not match nested repository paths"
	if [ -n "$allow_patterns" ]; then
		# shellcheck disable=SC2086
		scratch_git ls-files --cached -- $allow_patterns >"$work/nested-allowed" ||
			fail "git rejected a .wave-close-allowlist pattern"
	fi
fi

cp "$work/top" "$work/candidates" || fail "could not prepare candidates"
cp "$work/top-allowed" "$work/allowed" || fail "could not prepare allowlisted paths"
if [ "$nested_count" -gt 0 ]; then
	# Git's opaque `dir/` record (C-quoted as `"dir/"` for some names) is
	# replaced by the files inspected inside it, so allowlisting or ignoring
	# those files behaves as if the nested `.git` were absent. Every boundary
	# was inspected above or the sentinel has already failed.
	grep -v -e '/$' -e '/"$' "$work/top" >"$work/candidates"
	[ "$?" -le 1 ] || fail "could not drop nested repository boundaries"
fi
if [ -s "$work/nested" ]; then
	LC_ALL=C sort -u "$work/candidates" "$work/nested" >"$work/candidates-merged" ||
		fail "could not merge nested repository candidates"
	LC_ALL=C sort -u "$work/allowed" "$work/nested-allowed" >"$work/allowed-merged" ||
		fail "could not merge allowlisted paths"
	mv "$work/candidates-merged" "$work/candidates" || fail "could not merge nested repository candidates"
	mv "$work/allowed-merged" "$work/allowed" || fail "could not merge allowlisted paths"
fi

if [ ! -s "$work/candidates" ]; then
	echo "  OK"
	exit 0
fi
if [ -s "$work/allowed" ]; then
	grep -Fxvf "$work/allowed" "$work/candidates" >"$work/residual"
	[ "$?" -le 1 ] || fail "could not subtract allowlisted paths"
	grep -Fxf "$work/allowed" "$work/candidates" >"$work/allowlisted"
	[ "$?" -le 1 ] || fail "could not count allowlisted paths"
	allowed_count=$(wc -l <"$work/allowlisted" | tr -d ' ')
else
	cp "$work/candidates" "$work/residual" || fail "could not prepare candidates"
	allowed_count=0
fi
if [ ! -s "$work/residual" ]; then
	echo "  OK ($allowed_count entries allowlisted)"
	exit 0
fi
residual_count=$(wc -l <"$work/residual" | tr -d ' ')
echo "  WARN: $residual_count untracked files not in allowlist ($allowed_count allowlisted):"
sed 's/^/    /' "$work/residual"
exit 3
