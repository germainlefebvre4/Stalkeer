## Context

The TMDB client (`internal/external/tmdb/tmdb.go`) already has a circuit breaker and exponential-backoff retry, but no proactive rate limiting and no response caching. When processing large M3U playlists sequentially, each media item triggers 2–3 TMDB API calls (search → details → external IDs). TMDB's limit is ~40 requests/10 seconds. Without throttling, the client hammers the API, receives 429 responses, and retries with random backoff — wasting time and quota.

## Goals / Non-Goals

**Goals:**
- Prevent 429s proactively by throttling outbound requests to a configurable rate
- Eliminate redundant API calls within a single processing run via in-memory caching
- Recover correctly from 429s when they do occur, respecting the `Retry-After` header
- Zero new external dependencies

**Non-Goals:**
- Persistent cache (across process runs) — the DB already deduplicates at the media entity level
- Concurrent-safe rate limiting for parallel processing — processing is currently sequential
- Distributed rate limiting — single-process tool

## Decisions

### D1: Sleep-gap rate limiting (not token bucket)

**Decision**: Track `lastRequestAt time.Time` on the client. Before each HTTP call, compute `gap = interval - time.Since(lastRequestAt)` and sleep if `gap > 0`.

**Rationale**: Processing is sequential — there is never a burst of concurrent requests. A token bucket would allow accumulated quota to be consumed instantly when the client was idle (e.g., processing channels between movies), which doesn't help and adds complexity. The sleep-gap approach is 5 lines, no goroutines, no cleanup.

**Alternative considered**: `golang.org/x/time/rate` (token bucket) — rejected because it adds a dependency for marginal benefit in sequential context.

### D2: URL-keyed in-memory cache in `makeRequest`

**Decision**: Add `cache map[string][]byte` to the `Client` struct. The cache key is the full request URL (including params). On cache hit, unmarshal the stored bytes and skip the HTTP call entirely — skipping the rate limiter wait too.

**Rationale**: A single method-level cache means one implementation path, no per-method boilerplate, and the rate limiter skip is automatic (cache check precedes the sleep-gap). Raw-byte storage with re-unmarshal on hit is negligible overhead for batch processing.

**Alternative considered**: Typed caches per public method (6 maps) — rejected for verbosity with no meaningful benefit.

**Cache scope**: The `Client` is constructed fresh per `process` command invocation. The cache lives for exactly one processing run, which is the right scope — no stale data risk.

### D3: `sync.RWMutex` on the cache

**Decision**: Protect the cache with a `sync.RWMutex` even though processing is currently single-threaded.

**Rationale**: The cost is two lines. If concurrent processing is added later, a missing mutex would be a silent data race. Read-heavy access pattern (many lookups, few writes) makes `RWMutex` a natural fit.

### D4: Retry-After — sleep inside the operation, return retryable error

**Decision**: When a 429 is received, parse `Retry-After`, sleep inside the `circuitBrk.Execute` closure, then return the retryable error. The outer `retry.Do` loop handles the attempt count.

**Rationale**: Sleeping inside the operation ensures the minimum required wait is always observed before the next attempt. The retry loop's own backoff adds a small additional delay, which is acceptable (conservative, not harmful).

**Retry-After formats**: Both `seconds` (integer) and `HTTP-date` are supported. `strconv.Atoi` handles seconds; `http.ParseTime` (stdlib) handles dates. No new dependency.

## Risks / Trade-offs

- **Cache memory**: Storing raw JSON bytes per unique URL. A large playlist with many unique titles could accumulate MBs of cached responses. Acceptable for a CLI batch tool — not a long-running service.
- **Sleep-gap conservatism**: The sleep-gap approach doesn't exploit idle time between non-TMDB operations (channels, uncategorized items). Requests come slightly slower than the theoretical maximum. Acceptable — correctness over throughput.
- **Retry-After + retry backoff double-wait**: When a 429 occurs, the total wait is `Retry-After + retry.Do backoff`. This is slightly conservative but safe. The important thing is never retrying _before_ the server says it's ready.

## Migration Plan

1. Add `requests_per_second` to `TMDBConfig` in `config.go` with default `4.0`
2. Wire through `processor.go`
3. Implement rate limiter + cache in `tmdb.go`
4. Add field to `config.yml`

No migration steps required — new fields are optional with defaults. Existing deployments need no config changes.

## Open Questions

None — all decisions made during exploration.
