### Requirement: Configurable request rate
The TMDB client SHALL accept a `RequestsPerSecond float64` configuration value that controls the minimum interval between outbound HTTP requests. When set to `0`, rate limiting SHALL be disabled. The default value SHALL be `4.0` requests per second.

#### Scenario: Rate limiting applied between requests
- **WHEN** `RequestsPerSecond` is set to `4.0` and two requests are made back-to-back
- **THEN** the second request SHALL be delayed until at least 250ms after the first request completed

#### Scenario: Rate limiting disabled
- **WHEN** `RequestsPerSecond` is set to `0`
- **THEN** requests SHALL be made without any artificial delay

#### Scenario: Default rate applied when not configured
- **WHEN** `RequestsPerSecond` is not set in config
- **THEN** the client SHALL default to `4.0` requests per second

### Requirement: In-process response cache
The TMDB client SHALL cache successful API responses in memory for the lifetime of the client instance. Cached responses SHALL be returned without making a new HTTP request or consuming a rate-limit slot.

#### Scenario: Cache hit avoids HTTP call
- **WHEN** an identical request URL is made a second time within the same client lifetime
- **THEN** the cached response SHALL be returned immediately without an HTTP request

#### Scenario: Cache miss proceeds normally
- **WHEN** a request URL has not been seen before
- **THEN** the client SHALL proceed with the normal HTTP request flow

#### Scenario: Only successful responses are cached
- **WHEN** an API request returns an error (network error, non-2xx status, or 429)
- **THEN** the response SHALL NOT be cached

#### Scenario: Cache is scoped to client lifetime
- **WHEN** a new `tmdb.Client` is constructed
- **THEN** the cache SHALL be empty regardless of prior client instances

### Requirement: Retry-After header compliance
When the TMDB API returns a 429 status, the client SHALL read the `Retry-After` response header and sleep for the indicated duration before returning the retryable error to the retry loop.

#### Scenario: Retry-After in seconds format
- **WHEN** a 429 response includes `Retry-After: 30`
- **THEN** the client SHALL sleep for 30 seconds before returning the error

#### Scenario: Retry-After in HTTP-date format
- **WHEN** a 429 response includes `Retry-After: Wed, 21 Oct 2025 07:28:00 GMT`
- **THEN** the client SHALL sleep until that timestamp before returning the error

#### Scenario: Retry-After header absent
- **WHEN** a 429 response does not include a `Retry-After` header
- **THEN** the client SHALL return the retryable error immediately, allowing the retry loop's backoff to apply

#### Scenario: Retry-After with past timestamp
- **WHEN** the parsed `Retry-After` timestamp is in the past
- **THEN** the client SHALL NOT sleep (negative duration is treated as zero)
