#!/bin/sh
set -eu

root=$(mktemp -d "${TMPDIR:-/tmp}/tpatch-adjacent-args.XXXXXX")
trap 'rm -rf "$root"' EXIT

write_base() {
	cat >"$1/command.go" <<'EOF'
package command

func buildArgs() []string {
	args := []string{
		"--old-a",
		"--old-b",
	}
	return run(args)
}

func run(args []string) []string { return args }
EOF
}

write_master() {
	case "$2" in
	delete-all)
		sed '/"--old-a",/d; /"--old-b",/d' "$1/command.go" >"$1/next.go"
		;;
	delete-first)
		sed '/"--old-a",/d' "$1/command.go" >"$1/next.go"
		;;
	esac
	mv "$1/next.go" "$1/command.go"
}

write_feature() {
	case "$2" in
	after)
		sed '/"--old-b",/a\
		"--feature-x",\
		"--feature-y",' "$1/command.go" >"$1/next.go"
		;;
	before)
		sed '/"--old-a",/i\
		"--feature-x",\
		"--feature-y",' "$1/command.go" >"$1/next.go"
		;;
	between)
		sed '/"--old-a",/a\
		"--feature-x",\
		"--feature-y",' "$1/command.go" >"$1/next.go"
		;;
	append)
		sed '/^	}/a\
	args = append(args, "--feature-x", "--feature-y")' "$1/command.go" >"$1/next.go"
		;;
	esac
	mv "$1/next.go" "$1/command.go"
}

write_resolved() {
	case "$2" in
	delete-all)
		cat >"$1/command.go" <<'EOF'
package command

func buildArgs() []string {
	args := []string{
		"--feature-x",
		"--feature-y",
	}
	return run(args)
}

func run(args []string) []string { return args }
EOF
		;;
	delete-first)
		cat >"$1/command.go" <<'EOF'
package command

func buildArgs() []string {
	args := []string{
		"--feature-x",
		"--feature-y",
		"--old-b",
	}
	return run(args)
}

func run(args []string) []string { return args }
EOF
		;;
	esac
}

run_case() {
	name=$1
	feature_shape=$2
	master_shape=$3
	expected=$4
	repo="$root/$name"

	git init -q -b master "$repo"
	git -C "$repo" config user.name Fixture
	git -C "$repo" config user.email fixture@example.invalid
	write_base "$repo"
	git -C "$repo" add command.go
	git -C "$repo" commit -q -m base
	git -C "$repo" switch -q -c feature
	write_feature "$repo" "$feature_shape"
	git -C "$repo" add command.go
	git -C "$repo" commit -q -m feature
	git -C "$repo" switch -q master
	write_master "$repo" "$master_shape"
	git -C "$repo" add command.go
	git -C "$repo" commit -q -m upstream-delete

	merge="$root/$name-merge"
	rebase="$root/$name-rebase"
	git clone -q "$repo" "$merge"
	git clone -q "$repo" "$rebase"
	git -C "$merge" switch -q feature
	git -C "$rebase" switch -q feature

	set +e
	git -C "$merge" merge --no-commit --no-ff origin/master >/dev/null 2>&1
	merge_rc=$?
	git -C "$rebase" rebase origin/master >/dev/null 2>&1
	rebase_rc=$?
	set -e

	if [ "$merge_rc" -ne "$expected" ] || [ "$rebase_rc" -ne "$expected" ]; then
		printf 'unexpected result for %s: merge=%s rebase=%s expected=%s\n' \
			"$name" "$merge_rc" "$rebase_rc" "$expected" >&2
		exit 1
	fi
	if [ "$expected" -ne 0 ]; then
		write_resolved "$merge" "$master_shape"
		git -C "$merge" add command.go
		test "$(grep -c -- '--feature-x' "$merge/command.go")" -eq 1
		test "$(grep -c -- '--old-a' "$merge/command.go")" -eq 0
		case "$master_shape" in
		delete-all)
			test "$(grep -c -- '--old-b' "$merge/command.go")" -eq 0
			;;
		delete-first)
			test "$(grep -c -- '--old-b' "$merge/command.go")" -eq 1
			;;
		esac
	fi
	printf '%-31s merge=%s rebase=%s\n' "$name" "$merge_rc" "$rebase_rc"
}

run_case adjacent-after-delete-all after delete-all 1
run_case adjacent-before-delete-all before delete-all 1
run_case adjacent-between-delete-first between delete-first 1
run_case separate-append-delete-all append delete-all 0
