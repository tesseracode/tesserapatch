.PHONY: build test fmt install clean lint

BINARY=tpatch
BUILD_DIR=./cmd/tpatch

build:
	go build -o $(BINARY) $(BUILD_DIR)

test:
	go test ./...

fmt:
	gofmt -w .

lint:
	gofmt -l . | tee /dev/stderr | (! read)
	go vet ./...

install:
	go install $(BUILD_DIR)

clean:
	rm -f $(BINARY)

all: fmt lint test build
