# Spec: tvdb-id-backfill

## Purpose

Provide a CLI command that backfills missing TVDB IDs for `Movie` and `TVShow` records by querying the TMDB external IDs API and updating the database accordingly.

## Requirements

### Requirement: Backfill missing TVDB IDs for movies
The system SHALL provide a command that queries all `Movie` records where `tvdb_id IS NULL` and `tmdb_id != 0`, fetches their external IDs from the TMDB API, and updates the database with the returned `tvdb_id`.

#### Scenario: Movie with missing TVDB ID is updated
- **WHEN** a `Movie` record has `tvdb_id = NULL` and a valid `tmdb_id`
- **THEN** the command SHALL call `GetMovieExternalIDs(tmdb_id)` and update `tvdb_id` if the API returns a non-nil TVDB ID

#### Scenario: Movie with no TVDB entry on TMDB is skipped gracefully
- **WHEN** `GetMovieExternalIDs` returns a nil `tvdb_id` (film not referenced in TVDB)
- **THEN** the command SHALL leave the record unchanged and continue without error

#### Scenario: TMDB API error on a record is skipped gracefully
- **WHEN** `GetMovieExternalIDs` returns an error for a specific movie
- **THEN** the command SHALL log a warning, increment an error counter, and continue to the next record

### Requirement: Backfill missing TVDB IDs for TV shows
The system SHALL apply the same backfill logic to all `TVShow` records where `tvdb_id IS NULL` and `tmdb_id != 0`, using `GetTVShowExternalIDs`.

#### Scenario: TVShow with missing TVDB ID is updated
- **WHEN** a `TVShow` record has `tvdb_id = NULL` and a valid `tmdb_id`
- **THEN** the command SHALL call `GetTVShowExternalIDs(tmdb_id)` and update `tvdb_id` if the API returns a non-nil TVDB ID

#### Scenario: Multiple TVShow rows share the same TMDB ID
- **WHEN** several `TVShow` rows reference the same `tmdb_id` (different seasons/episodes)
- **THEN** each row SHALL be updated independently, and the TMDB API SHALL be called only once per unique `tmdb_id` (deduplication)

### Requirement: CLI command `enrich-tvdb`
The system SHALL expose a `stalkeer enrich-tvdb` CLI command with the following flags:
- `--dry-run` : affiche les enregistrements qui seraient mis à jour sans effectuer de modification
- `--limit N` : arrête après avoir traité N enregistrements (0 = pas de limite)
- `--verbose` : affiche le détail de chaque enregistrement traité

#### Scenario: Dry-run reports without modifying
- **WHEN** the command is run with `--dry-run`
- **THEN** no database writes SHALL occur and the output SHALL list movies/shows that would be updated

#### Scenario: Summary is printed on completion
- **WHEN** the command finishes
- **THEN** the output SHALL include counts for: total processed, updated, skipped (no TVDB on TMDB), errors
