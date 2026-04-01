## ADDED Requirements

### Requirement: Paginated episode fetching
The Sonarr client SHALL fetch all missing episodes across all pages of the `wanted/missing` endpoint, continuing until the number of fetched records equals `totalRecords` returned by the API.

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

### Requirement: Pagination progress logging
The Sonarr client SHALL log pagination progress at INFO level when a logger is configured.

#### Scenario: Logger configured
- **WHEN** Config.Logger is non-nil and multiple pages are fetched
- **THEN** each page fetch emits an INFO log with page number, fetched count, and total count

#### Scenario: No logger configured
- **WHEN** Config.Logger is nil
- **THEN** no logging occurs and the client operates normally without error
