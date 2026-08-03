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
wave-close-check:
	@echo "=== Wave-Close Checklist (mechanical gate) ==="
	@fail=0; \
	echo "[1/6] Working tree clean (no uncommitted changes to tracked files)..."; \
	if ! git diff --quiet HEAD 2>/dev/null; then \
		echo "  FAIL: uncommitted changes to tracked files present"; \
		git status --short | grep -v '^??' || true; \
		fail=1; \
	else \
		echo "  OK"; \
	fi; \
	echo "[2/6] HEAD pushed to origin/main..."; \
	git fetch --quiet origin main 2>/dev/null || true; \
	local_head=$$(git rev-parse HEAD); \
	remote_head=$$(git rev-parse origin/main 2>/dev/null || echo none); \
	if [ "$$local_head" != "$$remote_head" ]; then \
		echo "  FAIL: HEAD ($$local_head) != origin/main ($$remote_head)"; \
		echo "  A wave is not durably closed until pushed. Run: git push origin main"; \
		fail=1; \
	else \
		echo "  OK ($$local_head)"; \
	fi; \
	echo "[3/6] HEAD commit trailer parses (Rule 18)..."; \
	trailer=$$(git log -1 --format='%(trailers:key=Co-authored-by)' | tr -d '\n'); \
	if ! echo "$$trailer" | grep -q "Copilot <223556219+Copilot@users.noreply.github.com>"; then \
		echo "  FAIL: Co-authored-by: Copilot trailer missing or malformed on HEAD"; \
		echo "  Got: $$trailer"; \
		fail=1; \
	else \
		echo "  OK"; \
	fi; \
	echo "[4/6] CURRENT.md Status line not stale..."; \
	status_line=$$(sed -n '/^## Status/,/^## /p' docs/handoff/CURRENT.md | head -20); \
	if echo "$$status_line" | grep -qE 'rev-[0-9]+ dispatched|IN PROGRESS|awaiting review|dispatched \(rev'; then \
		echo "  FAIL: CURRENT.md Status contains mid-cycle marker; flip before closing"; \
		echo "$$status_line" | grep -E 'rev-[0-9]+ dispatched|IN PROGRESS|awaiting review|dispatched \(rev'; \
		fail=1; \
	else \
		echo "  OK"; \
	fi; \
	echo "[5/6] gofmt clean..."; \
	unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "  FAIL: unformatted files:"; \
		echo "$$unformatted"; \
		fail=1; \
	else \
		echo "  OK"; \
	fi; \
	echo "[6/6] go vet + go build clean..."; \
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
		echo "=== Mechanical gate: PASS ==="; \
	else \
		echo "=== Mechanical gate: FAIL ==="; \
		exit 1; \
	fi
