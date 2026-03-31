## Why

The Sonarr and Radarr clients fetch missing items with a hard-coded single page (pageSize=1000), silently truncating results beyond that limit. With ~3000 missing episodes or movies, up to 2/3 of items are never seen, causing missed downloads.

## What Changes

- `GetMissingEpisodes` (Sonarr): replace single-page fetch with a pagination loop over `/api/v3/wanted/missing`
- `GetMissingMovies` (Radarr): replace `/api/v3/movie` full-dump + client-side filter with a pagination loop over `/api/v3/wanted/missing`
- Both clients gain an optional `Logger` field; pagination progress is logged at INFO level
- `getEpisodes` and a new `getPagedMovies` private helper return `(records, totalRecords, error)` to drive the loop
- Retry is applied per page, not per full collection

## Capabilities

### New Capabilities

- `sonarr-pagination`: Paginated fetching of all missing episodes from Sonarr `wanted/missing` endpoint
- `radarr-pagination`: Paginated fetching of all missing movies from Radarr `wanted/missing` endpoint

### Modified Capabilities

<!-- No existing spec-level requirements are changing -->

## Impact

- `internal/external/sonarr/sonarr.go`: `Client`, `Config`, `GetMissingEpisodes`, `getEpisodes`
- `internal/external/radarr/radarr.go`: `Client`, `Config`, `GetMissingMovies`, new `getPagedMovies`, existing `getMovies` unchanged
- `cmd/sonarr.go`, `cmd/radarr.go`: pass logger to client config (optional, no breaking change)
- No API surface changes; public method signatures are unchanged
