## Why

`GetMissingEpisodes` and `GetMissingMovies` always paginate through all pages before any limit is applied, meaning a `--limit 5` debug run still issues all HTTP requests to Sonarr/Radarr. Introducing a `FetchOptions` struct with a `Limit` field enables early exit in the pagination loop, making debug runs faster and the API surface more explicit.

## What Changes

- Add `FetchOptions` struct (with `Limit int`, 0 = unlimited) to both `internal/external/sonarr` and `internal/external/radarr` packages
- Change `GetMissingEpisodes(ctx)` → `GetMissingEpisodes(ctx, FetchOptions)` **BREAKING**
- Change `GetMissingMovies(ctx)` → `GetMissingMovies(ctx, FetchOptions)` **BREAKING**
- Pagination loop exits early when `opts.Limit > 0 && len(all) >= opts.Limit`
- `cmd/sonarr.go`: pass `sonarr.FetchOptions{Limit: limit}` flag value; remove post-fetch truncation block
- `cmd/radarr.go`: pass `radarr.FetchOptions{Limit: limit}` flag value; remove post-fetch truncation block

## Capabilities

### New Capabilities

_(none)_

### Modified Capabilities

- `sonarr-pagination`: add limit-based early exit requirement to pagination loop
- `radarr-pagination`: add limit-based early exit requirement to pagination loop

## Impact

- `internal/external/sonarr/sonarr.go`: `GetMissingEpisodes` signature change
- `internal/external/radarr/radarr.go`: `GetMissingMovies` signature change
- `cmd/sonarr.go`: call site updated, post-fetch truncation removed
- `cmd/radarr.go`: call site updated, post-fetch truncation removed
- `internal/external/sonarr/sonarr_test.go`: tests updated for new signature
- `internal/external/radarr/radarr_test.go`: tests updated for new signature
