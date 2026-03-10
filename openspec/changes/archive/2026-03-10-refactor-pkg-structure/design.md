## Context

`cmd/main.go` is a 1264-line monolith defining 12 Cobra commands, all flag registrations, and shared formatting helpers. The Cobra framework supports distributing commands across files because all `init()` functions in `package main` run before `main()`. Two files (`cleanup.go`, `resume_downloads.go`) were partially extracted but inconsistently — `cleanup.go` has no `init()` so its flags live in `main.go`, creating implicit coupling.

Additionally, two `internal/` packages shadow stdlib names (`errors`, `testing`), which forces import aliases and creates confusion. Model files in `internal/models/` have misleading names (`filter_config.go` contains `Movie`; `processing_log.go` contains `TVShow`).

## Goals / Non-Goals

**Goals:**
- Each Cobra command in its own file; each file fully self-contained (definition + flags + `rootCmd.AddCommand`)
- `cmd/main.go` reduced to: `rootCmd`, `initConfig()`, global persistent flag, `main()`
- Shared CLI helpers (`formatBytes`, `sanitizeFilename`, `valueOrEmpty`) in `cmd/format.go`
- `internal/errors` renamed to `internal/apperrors`
- `internal/testing` renamed to `internal/testutil`
- `internal/models/` files renamed to match their contents

**Non-Goals:**
- No behavioral changes to any command
- No API or database schema changes
- No changes to `internal/` package logic, only filenames and import paths
- No splitting of `internal/downloader/` or other multi-file packages beyond what's needed

## Decisions

### D1: Self-registration via `init()` (Option B)

Each command file calls `rootCmd.AddCommand(xyzCmd)` in its own `init()`. `main.go` has no `AddCommand` calls.

**Alternatives considered:**
- Centralized registration in `main.go`: rejected — defeats the purpose of splitting, every new command still touches `main.go`

**Rationale:** Go initializes all `init()` functions in a package before `main()`. Self-registration is idiomatic for plugin-style CLI tools and makes each command file completely standalone.

### D2: `dryrun.go` → `analyze.go`

The cobra command `Use: "dryrun"` needs a file. Naming it `dryrun.go` visually conflicts with the `internal/dryrun` import in the same file. Renaming to `analyze.go` is clearer.

**Rationale:** Go doesn't have a naming collision at the compiler level, but it causes confusion for readers. `analyze.go` accurately describes the file's purpose (M3U analysis without DB writes).

### D3: M3U commands grouped in `m3u.go`

`downloadM3UCmd`, `listM3UArchivesCmd`, `cleanupM3UArchivesCmd` share the same domain and both listing/cleanup commands use `formatBytes`. A single `m3u.go` groups them.

**Alternatives considered:**
- Subdirectory `cmd/m3u/`: requires a new package, a public `Register()` func, and wires back to `rootCmd` — overcomplicated for 3 commands.

### D4: `internal/errors` → `internal/apperrors`

Shadowing `errors` stdlib forces every consumer to use an alias or carefully avoid ambiguity.

**Impact:** 5 source files need import path update. Package declaration changes from `package errors` to `package apperrors`.

### D5: `internal/testing` → `internal/testutil`

Package was never imported by any other file (grep confirms zero usages). Rename is zero-risk. `testutil` is the conventional name for test helper packages in Go.

### D6: `internal/models/` file renames — cosmetic only

Files renamed, package name `models` stays unchanged. Import paths unchanged. Pure cosmetic.

Target layout:
```
processed_line.go  ← ProcessedLine, ContentType, ProcessingState
media.go           ← Movie, TVShow, Channel, Uncategorized
download.go        ← DownloadInfo, DownloadStatus
filter.go          ← FilterConfig
log.go             ← ProcessingLog
```

## Risks / Trade-offs

- **[Risk] `init()` execution order** → In a single `package main`, Go guarantees all `init()` functions run before `main()`, but order between files is alphabetical by filename. Cobra commands can be added in any order — no dependency between command registrations exists, so this is safe.
- **[Risk] Missing `AddCommand` call** → If a file's `init()` is forgotten during extraction, the command silently disappears from the CLI. Mitigation: run `stalkeer --help` as a smoke test after each extraction batch.
- **[Risk] `apperrors` rename breaks build** → All 5 import sites must be updated atomically. Mitigation: update all files in one commit; CI will catch any miss.

## Migration Plan

1. Extract shared helpers → `cmd/format.go`
2. Extract commands one by one: server, process, analyze, config, migrate, radarr, sonarr, m3u (3 commands), enrich_tvdb, cleanup (add init), leaving resume_downloads as-is
3. Slim `cmd/main.go` to essentials
4. Rename `internal/testing/` → `internal/testutil/` (update package declaration)
5. Rename `internal/errors/` → `internal/apperrors/` (update package declaration + all 5 import sites)
6. Rename `internal/models/` files
7. Build + run full test suite

Rollback: all changes are in `cmd/` and `internal/` with no external surface changes — revert commit is sufficient.

## Open Questions

None — all decisions are resolved based on codebase exploration.
