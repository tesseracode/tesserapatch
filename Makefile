.PHONY: build test fmt install clean lint all wave-close-check

BINARY=bin/tpatch
BUILD_DIR=./cmd/tpatch

# Resolve a version string for ldflags injection. Falls back to "dev"
# when not in a git checkout (e.g. tarball builds, container layers).
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -ldflags "-X github.com/tesseracode/tesserapatch/internal/buildinfo.Version=$(VERSION)"

build:
	@mkdir -p bin
	go build $(LDFLAGS) -o $(BINARY) $(BUILD_DIR)

test:
	go test ./...

fmt:
	gofmt -w .

lint:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "Unformatted files:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	go vet ./...

install:
	go install $(LDFLAGS) $(BUILD_DIR)

clean:
	rm -rf bin/

all: fmt lint test build

# Mechanical Wave-Close Checklist gate. Verifies the subset of the
# checklist (AGENTS.md → Wave-Close Checklist) that can be checked
# programmatically. Human items are printed as a reminder banner.
# Codified 2026-08-02 after Cluster A external challenge #2.
# Rev-1 hardening (2026-08-02): fetch-failure = FAIL, trailer check
# over full WAVE_BASE..HEAD range, Status uses terminal-token
# allowlist, [1/6] flags untracked source-code as WARN. Set
# WAVE_BASE=<ref> to override the default trailer-check range
# (default: origin/main..HEAD excluding HEAD; for wave close on a
# pushed HEAD, we walk the last-shipped-tag..HEAD range instead).
WAVE_BASE ?=
wave-close-check:
	@echo "=== Wave-Close Checklist (mechanical gate) ==="
	@fail=0; warn=0; \
	echo "[1/7] Working tree clean (no uncommitted changes to tracked files)..."; \
	if ! git diff --quiet HEAD 2>/dev/null; then \
		echo "  FAIL: uncommitted changes to tracked files present"; \
		git status --short | grep -v '^??' || true; \
		fail=1; \
	else \
		echo "  OK"; \
	fi; \
	echo "[2/7] Untracked source-code files (forgotten \`git add\` sentinel)..."; \
	untracked_src=$$(git ls-files --others --exclude-standard -- '*.go' 'internal/**' 'cmd/**' 'assets/**' 'docs/adrs/*.md' 'docs/prds/*.md' 'docs/milestones/*.md' 'Makefile' 'go.mod' 'go.sum' 'AGENTS.md' 'SPEC.md' 'CLAUDE.md' 2>/dev/null); \
	if [ -n "$$untracked_src" ]; then \
		echo "  WARN: untracked source or design-doc files (may be forgotten adds):"; \
		echo "$$untracked_src" | sed 's/^/    /'; \
		warn=1; \
	else \
		echo "  OK"; \
	fi; \
	echo "[3/7] HEAD pushed to origin/main..."; \
	if ! git fetch --quiet origin main 2>/dev/null; then \
		echo "  FAIL: could not fetch origin/main; durability cannot be verified offline"; \
		echo "  Rerun with network available before closing the wave."; \
		fail=1; \
	else \
		local_head=$$(git rev-parse HEAD); \
		remote_head=$$(git rev-parse origin/main 2>/dev/null || echo none); \
		if [ "$$local_head" != "$$remote_head" ]; then \
			echo "  FAIL: HEAD ($$local_head) != origin/main ($$remote_head)"; \
			echo "  A wave is not durably closed until pushed. Run: git push origin main"; \
			fail=1; \
		else \
			echo "  OK ($$local_head)"; \
		fi; \
	fi; \
	echo "[4/7] Every wave commit carries the Rule 18 trailer..."; \
	if [ -n "$(WAVE_BASE)" ]; then \
		range="$(WAVE_BASE)..HEAD"; \
	else \
		last_tag=$$(git describe --tags --abbrev=0 2>/dev/null || echo ""); \
		if [ -n "$$last_tag" ]; then \
			range="$$last_tag..HEAD"; \
		else \
			range="HEAD~1..HEAD"; \
		fi; \
	fi; \
	echo "  range: $$range"; \
	if ! commits=$$(git rev-list --no-merges $$range 2>/dev/null); then \
		echo "  FAIL: git rev-list rejected range \`$$range\` (invalid endpoints?)"; \
		echo "  Set WAVE_BASE=<ref> to an explicit boundary."; \
		fail=1; \
	elif [ -z "$$commits" ]; then \
		echo "  FAIL: range \`$$range\` contains zero commits"; \
		echo "  Empty ranges silently false-pass; require WAVE_BASE=<pre-cluster ref>."; \
		fail=1; \
	else \
		bad_trailer=""; \
		for c in $$commits; do \
			t=$$(git log -1 --format='%(trailers:key=Co-authored-by)' $$c | tr -d '\n'); \
			if ! echo "$$t" | grep -q "Copilot <223556219+Copilot@users.noreply.github.com>"; then \
				bad_trailer="$$bad_trailer $$c"; \
			fi; \
		done; \
		if [ -n "$$bad_trailer" ]; then \
			echo "  FAIL: commits missing or malformed Co-authored-by: Copilot trailer:"; \
			for c in $$bad_trailer; do echo "    $$c $$(git log -1 --format='%s' $$c)"; done; \
			fail=1; \
		else \
			commit_count=$$(echo "$$commits" | wc -w | tr -d ' '); \
			echo "  OK ($$commit_count commits)"; \
		fi; \
	fi; \
	echo "[5/7] CURRENT.md \`**Cluster state**:\` canonical field is terminal..."; \
	state_matches=$$(grep -c -E '^\*\*Cluster state\*\*:' docs/handoff/CURRENT.md 2>/dev/null || echo 0); \
	if [ "$$state_matches" -eq 0 ]; then \
		echo "  FAIL: docs/handoff/CURRENT.md missing \`**Cluster state**: <TOKEN>\` field"; \
		echo "  See AGENTS.md \"Cluster State — Canonical Field for Mechanical Gate\"."; \
		fail=1; \
	elif [ "$$state_matches" -gt 1 ]; then \
		echo "  FAIL: docs/handoff/CURRENT.md contains $$state_matches \`**Cluster state**:\` fields; exactly one required"; \
		echo "  Duplicate fields false-pass on the earliest (possibly stale) token — replace, do not append."; \
		grep -n -E '^\*\*Cluster state\*\*:' docs/handoff/CURRENT.md | sed 's/^/    /'; \
		fail=1; \
	else \
		state_line=$$(grep -m1 -E '^\*\*Cluster state\*\*:' docs/handoff/CURRENT.md); \
		state_token=$$(echo "$$state_line" | sed -E 's/^\*\*Cluster state\*\*:[[:space:]]*//;s/[[:space:]]*$$//' | tr '[:lower:]' '[:upper:]'); \
		echo "  found: \`$$state_token\`"; \
		case "$$state_token" in \
			SHIPPED|APPROVED|ACCEPTED|IDLE) \
				echo "  OK (terminal)"; \
				;; \
			"IN PROGRESS"|REV-*" DISPATCHED"|"AWAITING REVIEW"|"NEEDS REVISION"|BLOCKED) \
				echo "  FAIL: mid-cycle or non-closed token; flip to a terminal token before closing"; \
				echo "  Terminal allowlist: SHIPPED | APPROVED | ACCEPTED | IDLE"; \
				fail=1; \
				;; \
			*) \
				echo "  FAIL: token \`$$state_token\` is not on the recognized allowlist or denylist"; \
				echo "  Terminal allowlist: SHIPPED | APPROVED | ACCEPTED | IDLE"; \
				echo "  Mid-cycle denylist: IN PROGRESS | REV-N DISPATCHED | AWAITING REVIEW | NEEDS REVISION | BLOCKED"; \
				fail=1; \
				;; \
		esac; \
	fi; \
	echo "[6/7] gofmt clean..."; \
	unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "  FAIL: unformatted files:"; \
		echo "$$unformatted"; \
		fail=1; \
	else \
		echo "  OK"; \
	fi; \
	echo "[7/7] go vet + go build clean..."; \
	if ! go vet ./... >/dev/null 2>&1; then \
		echo "  FAIL: go vet errors"; \
		go vet ./...; \
		fail=1; \
	elif ! go build ./cmd/tpatch >/dev/null 2>&1; then \
		echo "  FAIL: go build errors"; \
		go build ./cmd/tpatch; \
		fail=1; \
	else \
		echo "  OK"; \
	fi; \
	echo ""; \
	echo "=== Manual items (verify by hand) ==="; \
	echo "  [ ] Supervisor LOG entry prepended for this wave's verdicts"; \
	echo "  [ ] ROADMAP.md status flipped to terminal state with commit range"; \
	echo "  [ ] HISTORY.md archive appended (if final wave of cluster)"; \
	echo "  [ ] Non-invalidation invariants confirmed (Side Research md5, guarded files, Rule 20 repro)"; \
	echo "  [ ] Tag pushed if this ships a release (git push origin vX.Y.Z)"; \
	echo ""; \
	if [ "$$fail" -eq 0 ]; then \
		if [ "$$warn" -eq 0 ]; then \
			echo "=== Mechanical gate: PASS ==="; \
		else \
			echo "=== Mechanical gate: PASS (with warnings) ==="; \
		fi; \
	else \
		echo "=== Mechanical gate: FAIL ==="; \
		exit 1; \
	fi
