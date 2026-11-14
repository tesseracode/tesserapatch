#!/bin/sh
# Extract one tagged release section from CHANGELOG on stdin.
set -eu

if [ "$#" -ne 1 ]; then
	echo "usage: release-notes.sh vMAJOR.MINOR.PATCH < CHANGELOG.md" >&2
	exit 2
fi
case "$1" in
	''|*[!0-9A-Za-z.+-]*)
		echo "release-notes: invalid release tag" >&2
		exit 2
		;;
esac
if ! printf '%s\n' "$1" | LC_ALL=C grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z][0-9A-Za-z.+-]*)?$'; then
	echo "release-notes: invalid release tag" >&2
	exit 2
fi

awk -v version="$1" '
	/^## / {
		selected = ($2 == version)
		if (selected) found++
	}
	selected {
		notes = notes $0 "\n"
		if ($0 !~ /^[[:space:]]*$/ && $0 !~ /^#+ /) content++
	}
	END {
		if (found != 1 || !content) {
			print "release-notes: expected one nonempty CHANGELOG section for " version > "/dev/stderr"
			exit 1
		}
		printf "%s", notes
	}'
