# M0 — Bootstrap

**Status**: ✅ Complete

## Tasks

- [x] M0.1 — Initialize Go module (`go mod init`), create `cmd/tpatch/main.go` entry point
- [x] M0.2 — Set up `internal/` package structure: `cli/`, `store/`, `provider/`, `workflow/`, `gitutil/`, `safety/`
- [x] M0.3 — Create basic CLI dispatcher with `--help`, `--version`, `--path` global flag
- [x] M0.4 — Verify `go build ./cmd/tpatch`, `go test ./...`, `gofmt -l .` all pass
- [x] M0.5 — Create `assets/` directory with `embed.go` and placeholder content
- [x] M0.6 — Create `Makefile` with `build`, `test`, `fmt`, `install` targets

## Acceptance Criteria

- `./tpatch --help` prints usage
- `./tpatch --version` prints version
- `go test ./...` passes (even if no real tests yet)
- All source files pass `gofmt`

## Reference

- Port structure from `../gpt/cmd/tpatch/main.go` and `../gpt/internal/cli/app.go`
- Keep the same package boundaries GPT used, extended with `safety/` for path validation
