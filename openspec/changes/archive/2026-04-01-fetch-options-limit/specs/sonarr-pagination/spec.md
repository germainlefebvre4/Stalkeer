## ADDED Requirements

### Requirement: Fetch limit via FetchOptions
The Sonarr client SHALL accept a `FetchOptions` struct as second parameter to `GetMissingEpisodes`, with a `Limit int` field where 0 means unlimited.

#### Scenario: Zero limit means unlimited
- **WHEN** `FetchOptions.Limit` is 0
- **THEN** all pages are fetched and all records are returned (unchanged behavior)

#### Scenario: Limit stops pagination early
- **WHEN** `FetchOptions.Limit` > 0 and `len(fetched) >= Limit` after a page is appended
- **THEN** pagination stops immediately and the result is trimmed to exactly `Limit` records

#### Scenario: Limit larger than total records
- **WHEN** `FetchOptions.Limit` > `totalRecords`
- **THEN** all pages are fetched and the full set is returned without trimming

## MODIFIED Requirements

### Requirement: Paginated episode fetching
The Sonarr client SHALL fetch missing episodes across pages of the `wanted/missing` endpoint, stopping when the number of fetched records equals `totalRecords` OR reaches `FetchOptions.Limit` (whichever comes first).

#### Scenario: Fetch completes in a single page
- **WHEN** totalRecords <= pageSize (1000)
- **THEN** exactly one HTTP request is made and all records are returned

#### Scenario: Fetch spans multiple pages
- **WHEN** totalRecords > pageSize and FetchOptions.Limit is 0 (or greater than totalRecords)
- **THEN** the client iterates pages (1, 2, ...) until len(fetched) >= totalRecords, returning the full collection

#### Scenario: Empty result set
- **WHEN** totalRecords is 0
- **THEN** the client returns an empty slice without error

#### Scenario: Retry on transient page failure
- **WHEN** a page request fails with a retryable error
- **THEN** only that page is retried according to retry.Config, not the entire collection
