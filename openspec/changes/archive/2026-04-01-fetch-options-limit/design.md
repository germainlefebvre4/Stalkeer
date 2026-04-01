## Context

Both `sonarr.Client.GetMissingEpisodes` and `radarr.Client.GetMissingMovies` paginate through all pages of the `wanted/missing` endpoint before returning. The `--limit` flag in `cmd/sonarr.go` and `cmd/radarr.go` truncates the result post-fetch, meaning a `--limit 5` run still issues all HTTP requests. This wastes time during debug/validation runs with large libraries.

## Goals / Non-Goals

**Goals:**
- Add `FetchOptions{Limit int}` struct to `internal/external/sonarr` and `internal/external/radarr` packages
- Enable early exit in the pagination loop once `Limit` is reached
- Update both cmd/ call sites to pass `FetchOptions`; remove post-fetch truncation blocks

**Non-Goals:**
- No `debug_limit` field in `config.yml` — flag-only, no config surface
- No server-side limit parameter in the Sonarr/Radarr API request
- No shared `FetchOptions` type across packages

## Decisions

### FetchOptions struct per package, not shared

Each package defines its own `FetchOptions`. A shared package would be premature abstraction for two structs that carry a single `Limit int` field and may evolve independently.

**Alternative considered**: `internal/fetch/options.go` shared type — rejected, adds import indirection with no current benefit.

### Explicit parameter, not variadic

`GetMissingEpisodes(ctx, FetchOptions{})` over `GetMissingEpisodes(ctx, ...FetchOptions)`. The zero value (`FetchOptions{Limit: 0}`) is the unlimited default, keeping the call site explicit.

**Alternative considered**: variadic `opts ...FetchOptions` — rejected, hides intent and requires range access in implementation.

### Trim on page boundary overshoot

If a page push `len(all)` past the limit, the slice is trimmed to exactly `Limit` before returning. This ensures deterministic output regardless of where the boundary falls within a page.

## Risks / Trade-offs

- **Breaking signature change on both public methods** → Mitigated: only two internal call sites (`cmd/sonarr.go`, `cmd/radarr.go`); no external consumers.
- **Tests require signature update** → All existing tests pass `FetchOptions{}` (unlimited), preserving full existing coverage without behavior change.

## Migration Plan

Both call sites are in `cmd/` (internal). Update signatures and remove post-fetch truncation blocks. No data migration, no deployment steps.
