## Context

The Sonarr and Radarr clients currently fetch missing items in a single HTTP request. Sonarr's `wanted/missing` endpoint is page-based and returns a `totalRecords` field alongside a `records` array. With a hard-coded `pageSize=1000` and no loop, any installation with more than 1000 missing items silently receives incomplete data. Radarr uses a different endpoint (`/api/v3/movie`) that returns all movies unconditionally and requires client-side filtering; Radarr also exposes a `wanted/missing` endpoint with the same paged envelope.

## Goals / Non-Goals

**Goals:**
- Fetch all pages of missing episodes from Sonarr until `totalRecords` is exhausted
- Fetch all pages of missing movies from Radarr using `wanted/missing` (API-side filtering)
- Apply retry per page so a transient failure retries only that page, not the full collection
- Log pagination progress at INFO when a logger is configured
- Keep public method signatures unchanged

**Non-Goals:**
- Concurrent page fetching (sequential is sufficient and avoids server load)
- Persistent checkpointing across application restarts
- Changing any other client methods (series details, episode details, etc.)

## Decisions

### D1 — Pagination loop inline in each method (not a shared generic helper)

There are only 2 call sites. A generic helper (`fetchAllPages[T]`) would require passing a page-fetching function as a parameter — this adds indirection for no meaningful reuse gain. Inline loops in `GetMissingEpisodes` and `GetMissingMovies` are easier to read and maintain.

*Alternative considered*: `fetchAllPages[T any](ctx, fetchFn)` generic helper. Rejected: over-abstraction for 2 cases, obscures the retry boundary.

### D2 — Retry per page, not per entire collection

Wrapping the full pagination loop in a single `retry.Do` means a failure on page 5 of 10 would restart from page 1. Retrying per page is strictly better: it retries only the failed request and preserves progress. This matches how the TMDB client handles individual API calls.

### D3 — Radarr switches from `/api/v3/movie` to `/api/v3/wanted/missing`

`/api/v3/movie` returns all movies and requires client-side filtering for `monitored && !hasFile`. The `wanted/missing` endpoint supports the same paged envelope as Sonarr and filters server-side. This is more efficient and provides consistency between both clients.

*Alternative considered*: Keep `/api/v3/movie` for Radarr and only fix Sonarr. Rejected: creates asymmetry and still doesn't paginate Radarr's large catalogs.

### D4 — Logger is optional (nil-safe field on Client)

Pattern already established by `tmdb.Client`. Adding `Logger *logger.Logger` to `Config` and storing it on `Client` follows the same convention. No logger = no log output; callers not passing one are unaffected.

### D5 — Private helpers return `(records []T, totalRecords int, error)`

`getEpisodes` is updated to decode `totalRecords` and return it. A new `getPagedMovies` is added for Radarr's paged endpoint. The existing `getMovies` (used for the general movie list) is unchanged to avoid breaking other potential callers.

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| Radarr `wanted/missing` may not exist on older versions | Document minimum Radarr API v3 requirement (already assumed) |
| Very large collections cause many sequential HTTP requests | With pageSize=1000 and ~3000 items, max 3 requests — acceptable |
| `totalRecords` changes between pages (items added/removed mid-run) | Use `len(all) >= totalRecords` stop condition; a slight over/under-fetch is acceptable and self-corrects on next run |
| Existing tests mock `getEpisodes` with old signature | Tests use `httptest` server — updating response JSON to include `totalRecords` is sufficient |

## Migration Plan

- No database migrations required
- No config changes required
- `cmd/sonarr.go` and `cmd/radarr.go` may optionally pass a logger to the client config; existing calls without a logger continue to work unchanged
- Deploy by rebuilding the binary (`go build -o bin/stalkeer cmd/main.go`)

## Open Questions

None — all decisions resolved during exploration.
