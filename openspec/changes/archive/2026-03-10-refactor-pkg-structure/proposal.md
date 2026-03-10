## Why

`cmd/main.go` has grown to 1264 lines containing 12 commands, shared helpers, and all flag registration. `internal/` packages have naming issues that shadow stdlib (`errors`, `testing`) and model files whose names don't match their contents, making the codebase harder to navigate and extend.

## What Changes

- Split `cmd/main.go` into one file per command, each self-registering via `init()`
- Extract shared formatting helpers into `cmd/format.go`
- Rename `cmd/dryrun.go` → `cmd/analyze.go` to avoid visual collision with `internal/dryrun` import
- Move flag registration out of `main.go` into each command's own `init()`
- Rename `internal/errors` → `internal/apperrors` (shadows `errors` stdlib)
- Rename `internal/testing` → `internal/testutil` (shadows `testing` stdlib; package was never imported)
- Rename `internal/models/` files to match their actual contents:
  - `playlist_item.go` → `processed_line.go`
  - `processing_log.go` → `media.go` + `download.go` + `log.go`
  - `filter_config.go` → `filter.go` (+ `Movie`/`Channel`/`Uncategorized` consolidated into `media.go`)

## Capabilities

### New Capabilities

- `cmd-entrypoints`: Convention for structuring CLI command files — one command per file, each self-registering with `rootCmd` via its own `init()`, flags declared in the same file

### Modified Capabilities

<!-- No behavioral requirement changes — this is a structural refactor only -->

## Impact

- `cmd/main.go`: shrinks to ~30 lines (main, rootCmd, initConfig, global flag)
- `internal/downloader/downloader.go`, `resume.go`, `state_manager.go`: import path updated (`apperrors`)
- `internal/external/radarr/radarr.go`, `sonarr/sonarr.go`: import path updated (`apperrors`)
- `internal/parser/parser.go`: import path updated (`apperrors`)
- No API changes, no database schema changes, no behavioral changes
