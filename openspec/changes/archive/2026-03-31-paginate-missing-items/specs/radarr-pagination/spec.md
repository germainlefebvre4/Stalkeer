## ADDED Requirements

### Requirement: Paginated movie fetching via wanted/missing
The Radarr client SHALL fetch all missing movies using the `wanted/missing` endpoint with server-side filtering, iterating all pages until `totalRecords` is exhausted.

#### Scenario: Fetch completes in a single page
- **WHEN** totalRecords <= pageSize (1000)
- **THEN** exactly one HTTP request is made and all records are returned

#### Scenario: Fetch spans multiple pages
- **WHEN** totalRecords > pageSize
- **THEN** the client iterates pages (1, 2, ...) until len(fetched) >= totalRecords, returning the full collection

#### Scenario: Empty result set
- **WHEN** totalRecords is 0
- **THEN** the client returns an empty slice without error

#### Scenario: Retry on transient page failure
- **WHEN** a page request fails with a retryable error
- **THEN** only that page is retried according to retry.Config, not the entire collection

### Requirement: Server-side filtering replaces client-side filtering
The Radarr client SHALL use the `wanted/missing` endpoint which returns only monitored movies without files, removing the need for client-side `monitored && !hasFile` filtering.

#### Scenario: Only missing monitored movies returned
- **WHEN** GetMissingMovies is called
- **THEN** only movies that are monitored and missing a file are returned, as filtered by the API

### Requirement: Pagination progress logging
The Radarr client SHALL log pagination progress at INFO level when a logger is configured.

#### Scenario: Logger configured
- **WHEN** Config.Logger is non-nil and multiple pages are fetched
- **THEN** each page fetch emits an INFO log with page number, fetched count, and total count

#### Scenario: No logger configured
- **WHEN** Config.Logger is nil
- **THEN** no logging occurs and the client operates normally without error
