.PHONY: build test fmt install clean lint all

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
