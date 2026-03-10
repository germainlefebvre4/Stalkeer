## 1. cmd/ — Extract shared helpers

- [x] 1.1 Create `cmd/format.go` with `formatBytes()`, `sanitizeFilename()`, `valueOrEmpty()`
- [x] 1.2 Remove those three functions from `cmd/main.go`

## 2. cmd/ — Extract commands (one file per command)

- [x] 2.1 Create `cmd/server.go` with `serverCmd`, flags, and `rootCmd.AddCommand(serverCmd)` in `init()`
- [x] 2.2 Create `cmd/process.go` with `processCmd`, flags, and `rootCmd.AddCommand(processCmd)` in `init()`
- [x] 2.3 Create `cmd/analyze.go` with `dryrunCmd`, flags, and `rootCmd.AddCommand(dryrunCmd)` in `init()`
- [x] 2.4 Create `cmd/config.go` with `configCmd`, flags, and `rootCmd.AddCommand(configCmd)` in `init()`
- [x] 2.5 Create `cmd/migrate.go` with `migrateCmd` and `rootCmd.AddCommand(migrateCmd)` in `init()`
- [x] 2.6 Create `cmd/radarr.go` with `radarrCmd`, flags, and `rootCmd.AddCommand(radarrCmd)` in `init()`
- [x] 2.7 Create `cmd/sonarr.go` with `sonarrCmd`, flags, and `rootCmd.AddCommand(sonarrCmd)` in `init()`
- [x] 2.8 Create `cmd/m3u.go` with `downloadM3UCmd`, `listM3UArchivesCmd`, `cleanupM3UArchivesCmd`, all flags, and all three `rootCmd.AddCommand` calls in `init()`
- [x] 2.9 Create `cmd/enrich_tvdb.go` with `enrichTVDBCmd`, flags, and `rootCmd.AddCommand(enrichTVDBCmd)` in `init()`
- [x] 2.10 Add `init()` to `cmd/cleanup.go` with flag declarations and `rootCmd.AddCommand(cleanupCmd)`

## 3. cmd/ — Slim main.go

- [x] 3.1 Remove all command variables from `cmd/main.go` (now in dedicated files)
- [x] 3.2 Remove all flag declarations from `cmd/main.go` `init()` (now in each command file)
- [x] 3.3 Remove all `rootCmd.AddCommand` calls from `cmd/main.go` `init()` (now in each command file)
- [x] 3.4 Remove unused imports from `cmd/main.go`
- [x] 3.5 Verify `cmd/main.go` contains only: `rootCmd`, `configFile` var, `initConfig()`, global persistent flag, `cobra.OnInitialize`, and `main()`

## 4. internal/ — Rename testing package

- [x] 4.1 Rename directory `internal/testing/` → `internal/testutil/`
- [x] 4.2 Update package declaration in `internal/testutil/helpers.go` from `package testing` to `package testutil`
- [x] 4.3 Verify no other files import `internal/testing` (zero usages confirmed)

## 5. internal/ — Rename errors package

- [x] 5.1 Rename directory `internal/errors/` → `internal/apperrors/`
- [x] 5.2 Update package declaration in `internal/apperrors/errors.go` from `package errors` to `package apperrors`
- [x] 5.3 Update import path in `internal/external/radarr/radarr.go`
- [x] 5.4 Update import path in `internal/external/sonarr/sonarr.go`
- [x] 5.5 Update import path in `internal/downloader/downloader.go`
- [x] 5.6 Update import path in `internal/downloader/state_manager.go`
- [x] 5.7 Update import path in `internal/downloader/resume.go`
- [x] 5.8 Update import path in `internal/parser/parser.go`

## 6. internal/models/ — Rename files

- [x] 6.1 Rename `internal/models/playlist_item.go` → `internal/models/processed_line.go`
- [x] 6.2 Rename `internal/models/processing_log.go` → `internal/models/media.go` (contains TVShow) and split out `download.go` (DownloadInfo, DownloadStatus) and `log.go` (ProcessingLog)
- [x] 6.3 Rename `internal/models/filter_config.go` → `internal/models/filter.go` (FilterConfig) and move Movie, Channel, Uncategorized into `media.go`

## 7. Validation

- [x] 7.1 Run `go build ./...` — no compilation errors
- [x] 7.2 Run `go test ./...` — all tests pass
- [x] 7.3 Run `./bin/stalkeer --help` — all commands visible
- [x] 7.4 Verify `cmd/main.go` is fewer than 50 lines
