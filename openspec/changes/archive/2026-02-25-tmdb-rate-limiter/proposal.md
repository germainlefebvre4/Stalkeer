## Why

When processing large M3U playlists, the TMDB client fires 2–3 API calls per media item (search + details + external IDs) with no throttling. On playlists with thousands of entries this consistently hits TMDB's ~40 req/10s limit, causing cascading 429 errors and wasted retries with dumb exponential backoff. The `Retry-After` header is also ignored, so recovery is slower than it needs to be.

## What Changes

- Add a configurable request rate limiter to the TMDB client (sleep-gap approach, no new dependencies)
- Add an in-memory response cache keyed by request URL, scoped to the client lifetime (one process run), eliminating redundant API calls for the same title/ID
- Parse the `Retry-After` response header on 429 responses (both seconds and HTTP-date formats) and sleep for the indicated duration before retrying
- Add `requests_per_second` field to TMDB config (default: 4.0)

## Capabilities

### New Capabilities

- `tmdb-rate-limiter`: Proactive request throttling and in-process response caching for the TMDB API client

### Modified Capabilities

<!-- No existing spec-level requirements are changing -->

## Impact

- `internal/external/tmdb/tmdb.go` — core changes: new fields, rate limiting logic, cache logic, Retry-After parsing
- `internal/config/config.go` — add `RequestsPerSecond float64` to `TMDBConfig`
- `internal/processor/processor.go` — pass `RequestsPerSecond` when constructing the TMDB client
- `config.yml` / `config.yml.example` — document the new `requests_per_second` field
- No new external dependencies
- No API or database schema changes
