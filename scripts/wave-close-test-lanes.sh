#!/bin/sh
# Two-lane runner for the wave-close test shards: the default for
# `make wave-close-check` (WAVE_CLOSE_TEST_LANES=2); an empty
# WAVE_CLOSE_TEST_LANES or WAVE_CLOSE_TEST_LANES=0 runs the serial script
# instead, and this runner refuses every value but 2.
#
# scripts/wave-close-test-shards.sh stays the single source of truth: this
# runner copies its environment preamble and its `go test -p=1` lines
# verbatim, so argv, -count=1 and the 40m timeouts cannot drift. It only
# decides which shards share a lane, runs the two lanes concurrently, and
# then runs a short solo tail on its own.
#
# The partition is static so it can be reviewed and guarded:
# - lane B holds every lane shard that peaked at >= 2.5 GiB on the reference
#   host, so two of them never run at once;
# - shard 1 starts lane A. Its store tests create and remove a scratch
#   directory inside the checkout, so lane B starts a shard whose package
#   pattern walks the module only after shard 1 has passed. Lane B begins
#   with ./internal/cli-only shards, so it rarely waits;
# - the solo tail holds the observer shards whose child `go test` used more
#   than half of its fixed wall-clock budget in a serial run. Overlap slows
#   every shard a little, and that was enough to time one of them out. They
#   run one at a time, after both lanes have passed;
# - the lanes are balanced by measured serial time.
# A shard added to or removed from the serial script fails this runner
# closed until the partition is updated.
set -eu

lane_a='1 2 10 18 19 23'
lane_b='15 16 17 20 21 22 24 25 27 31 32 33 34 35 36 14 26 3 4 5 6 7 8 9 11 12 13'
solo='28 29 30'
shards=scripts/wave-close-test-shards.sh

if [ "$#" -ne 1 ] || [ "$1" != 2 ]; then
	echo "usage: sh scripts/wave-close-test-lanes.sh 2" >&2
	exit 2
fi

# The concurrency audit assumed golden recording, live providers and Git
# directory redirection are all off; refuse rather than run unaudited.
for name in TPATCH_RECORD_ROUTING_GOLDENS TPATCH_RECORD_PREPARE_PIB_GOLDENS \
	TPATCH_UPDATE_ROC_S0_GOLDENS TPATCH_LIVE_PROVIDER GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE; do
	if printenv "$name" >/dev/null; then
		echo "wave-close lanes: refusing to run with $name set" >&2
		exit 2
	fi
done

# Cancellation finds escaped descendants through an inherited descriptor;
# refuse rather than run without a way to list who holds it.
if ! command -v lsof >/dev/null 2>&1 && [ ! -d /proc/self/fd ]; then
	echo "wave-close lanes: need lsof or /proc to stop every process on cancellation" >&2
	exit 2
fi

if ! awk -v lanes="$lane_a $lane_b $solo" '
	/^go test -p=1 / { n++ }
	END {
		k = split(lanes, all, " ")
		if (k != n || all[1] != 1) exit 1
		for (i = 1; i <= k; i++) {
			x = all[i]
			if (x !~ /^[0-9]+$/ || x + 0 < 1 || x + 0 > n || seen[x + 0]++) exit 1
		}
	}' "$shards"; then
	echo "wave-close lanes: partition must cover $shards exactly once and start lane A with shard 1" >&2
	exit 2
fi

tmp=$(mktemp -d "${TMPDIR:-/tmp}/wave-close-lanes.XXXXXX")
tmp=$(cd "$tmp" && pwd -P)
# Every lane process opens this marker on descriptor 9, and everything it
# starts inherits it, so cancellation can find a descendant whose parent has
# already exited. Nothing outside this runner can hold it.
marker=$tmp/lanes.marker
: >"$marker"

# descendants prints the pid of every process below this shell.
descendants() {
	ps -A -o pid= -o ppid= | awk -v root="$$" '
		{ parent[$1] = $2 }
		END {
			below[root] = 1
			do {
				grew = 0
				for (pid in parent)
					if (!(pid in below) && (parent[pid] in below)) {
						below[pid] = 1
						grew = 1
					}
			} while (grew)
			for (pid in below)
				if (pid != root) print pid
		}'
}

# marked prints the pid of every process holding the marker.
marked() {
	if command -v lsof >/dev/null 2>&1; then
		lsof -t -- "$marker" 2>/dev/null || true
	else
		find /proc/[0-9]*/fd -lname "$marker" 2>/dev/null | cut -d/ -f3 || true
	fi
}

# members prints the pid of every process this runner started, once.
members() {
	{ descendants; marked; } | awk -v self="$$" '$1 != self && !seen[$1]++'
}

# freeze stops every member not yet in $frozen and adds it there, until no
# new member appears, so no process can start one that escapes the listing.
# The last listing, taken with every member stopped, is kept in $owned.
freeze() {
	while :; do
		owned=$(members | tr '\n' ' ')
		grew=
		for pid in $owned; do
			case " $frozen " in
			*" $pid "*) ;;
			*) if kill -STOP "$pid" 2>/dev/null; then
				frozen="$frozen $pid"
				grew=1
			fi ;;
			esac
		done
		[ -n "$grew" ] || break
	done
}

# alive prints the pids that are still running; a zombie has exited.
alive() {
	for pid in "$@"; do
		case $(ps -o stat= -p "$pid" 2>/dev/null || true) in
		'' | Z*) ;;
		*) echo "$pid" ;;
		esac
	done
}

# verified prints the pids in $frozen that freeze's last listing found. Each
# was stopped before that listing, so unless it is a zombie it cannot have
# exited and had its pid reused since: it is still this runner's.
verified() {
	for pid in $frozen; do
		case " $owned " in
		*" $pid "*) printf '%s ' "$pid" ;;
		esac
	done
}

# release continues every pid in $frozen that is not in $1.
release() {
	for pid in $frozen; do
		case " $1 " in
		*" $pid "*) ;;
		*) kill -CONT "$pid" 2>/dev/null || true ;;
		esac
	done
}

# cleanup stops every process this runner started, including the go test
# processes and test binaries inside each lane. A background lane ignores
# terminal interrupts, so nothing else would stop them. The whole tree is
# frozen, then sent TERM. Whatever is still this runner's and alive 10
# seconds later, such as a process that ignores TERM, is frozen again with
# anything it started and killed. Only live pids verified by a listing taken
# while they were stopped are signalled, never a pid remembered from an
# earlier listing: that process may have exited and its pid been reused.
cleanup() {
	frozen=
	freeze
	if [ -n "$frozen" ]; then
		kill -TERM $(alive $(verified)) 2>/dev/null || true
		release ""
		waited=0
		while [ "$waited" -lt 10 ] && [ -n "$(alive $(members))" ]; do
			sleep 1
			waited=$((waited + 1))
		done
		frozen=
		freeze
		left=$(alive $(verified) | tr '\n' ' ')
		if [ -n "$left" ]; then
			kill -KILL $left 2>/dev/null || true
		fi
		release "$left"
	fi
	wait
	rm -rf "$tmp"
}

# A second signal must not cut cleanup short, so cleanup starts by ignoring
# them. The first signal runs cleanup itself rather than through exit: a
# signal arriving as the EXIT trap starts would otherwise exit again before
# its first command.
on_signal() {
	trap '' INT TERM HUP
	cleanup
	trap - EXIT
	exit "$1"
}
trap 'trap "" INT TERM HUP; cleanup' EXIT
trap 'on_signal 130' INT
trap 'on_signal 143' TERM
trap 'on_signal 129' HUP

# Each shard gets its own script: the serial preamble followed by that
# shard's serial command. The shards whose package pattern walks the module
# are listed in $tmp/walks.
awk -v dir="$tmp" '
	/^go test -p=1 / {
		script = dir "/" ++n ".sh"
		printf "%s%s\n", preamble, $0 >script
		close(script)
		if ($4 ~ /\.\.\.$/) walks = walks " " n
		next
	}
	n == 0 { preamble = preamble $0 "\n" }
	END { print walks " " >(dir "/walks") }' "$shards"
walks=$(cat "$tmp/walks")

# await_shard_1 returns once shard 1 has passed. It fails if shard 1 failed
# or lane A stopped without recording shard 1's status.
await_shard_1() {
	while [ ! -e "$tmp/shard-1.status" ]; do
		if ! kill -0 "$pid_a" 2>/dev/null && [ ! -e "$tmp/shard-1.status" ]; then
			echo "wave-close lanes: lane A stopped before shard 1 finished; lane B stops here"
			return 1
		fi
		sleep 1
	done
	if [ "$(cat "$tmp/shard-1.status")" != 0 ]; then
		echo "wave-close lanes: shard 1 failed; lane B stops before shards that walk the module"
		return 1
	fi
}

# run_lane NAME SHARDS runs the shards in order and stops at the first
# failure. Lane A records shard 1's exit status atomically for lane B.
run_lane() {
	for shard in $2; do
		case "$1$walks" in
		"B"*" $shard "*) await_shard_1 || return 1 ;;
		esac
		status=0
		sh "$tmp/$shard.sh" || status=$?
		if [ "$shard" -eq 1 ]; then
			echo "$status" >"$tmp/shard-1.tmp"
			mv "$tmp/shard-1.tmp" "$tmp/shard-1.status"
		fi
		if [ "$status" -ne 0 ]; then
			return "$status"
		fi
	done
}

run_lane A "$lane_a" >"$tmp/a.out" 2>&1 9<"$marker" &
pid_a=$!
run_lane B "$lane_b" >"$tmp/b.out" 2>&1 9<"$marker" &
pid_b=$!
status_a=0
wait "$pid_a" || status_a=$?
status_b=0
wait "$pid_b" || status_b=$?

# The solo tail runs in the background too: a trapped signal interrupts
# wait at once, but would wait for a foreground shard to finish.
status_s=
if [ "$status_a" -eq 0 ] && [ "$status_b" -eq 0 ]; then
	status_s=0
	run_lane S "$solo" >"$tmp/s.out" 2>&1 9<"$marker" &
	pid_s=$!
	wait "$pid_s" || status_s=$?
fi

# The gate shows only the tail of this output, so a failure goes last.
# When both lanes fail, lane A goes last: lane B may only have stopped at
# shard 1.
report() {
	echo "=== $1 (shards $2): exit $3 ==="
	cat "$4"
}
if [ -z "$status_s" ]; then
	echo "=== solo (shards $solo): not run, a lane failed ==="
fi
if [ "$status_a" -ne 0 ]; then
	report "lane B" "$lane_b" "$status_b" "$tmp/b.out"
	report "lane A" "$lane_a" "$status_a" "$tmp/a.out"
else
	report "lane A" "$lane_a" "$status_a" "$tmp/a.out"
	report "lane B" "$lane_b" "$status_b" "$tmp/b.out"
fi
if [ -n "$status_s" ]; then
	report solo "$solo" "$status_s" "$tmp/s.out"
fi
[ "$status_a" -eq 0 ] && [ "$status_b" -eq 0 ] && [ "$status_s" -eq 0 ]
