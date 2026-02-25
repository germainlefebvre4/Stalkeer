# Capability: M3U Title Normalization

## Purpose

Defines how the processor normalizes raw M3U entry titles before querying external services (TMDB). Normalization includes stripping quality/language suffixes, extracting embedded years from various formats, and ensuring database upserts are atomic to handle duplicate TMDB entities within a single processing run.

## Requirements

### Requirement: Strip quality suffixes from movie titles
Before querying TMDB, the processor SHALL remove quality and language suffixes from M3U titles. The following suffixes SHALL be stripped (case-insensitive, preceded by whitespace): `SD`, `HD`, `FHD`, `UHD`, `4K`, `MULTI`, `VOSTFR`, `VF`.

#### Scenario: Strip trailing SD suffix
- **WHEN** a movie title is `"Wonder Woman SD"`
- **THEN** the title sent to TMDB SHALL be `"Wonder Woman"`

#### Scenario: Strip trailing FHD MULTI suffix
- **WHEN** a movie title is `"Die Hart 2 (2024) FHD MULTI"`
- **THEN** the title sent to TMDB SHALL be `"Die Hart 2"` with year `2024`

#### Scenario: Strip trailing HD suffix with accented characters
- **WHEN** a movie title is `"Jumanji : Bienvenue dans la jungle SD"`
- **THEN** the title sent to TMDB SHALL be `"Jumanji : Bienvenue dans la jungle"`

#### Scenario: Title without suffix is unchanged
- **WHEN** a movie title has no quality suffix (e.g., `"Inception (2010)"`)
- **THEN** the title sent to TMDB SHALL be `"Inception"` with year `2010`

### Requirement: Extract year from "Titre - YYYY" format
The processor SHALL recognize the format `<title> - <YYYY>` (where YYYY is a 4-digit year between 1900 and 2100) and extract the year separately from the title.

#### Scenario: Year extraction from dash format
- **WHEN** a movie title is `"Super Dark Times - 2017"`
- **THEN** the title sent to TMDB SHALL be `"Super Dark Times"` with year `2017`

#### Scenario: Year extraction from dash format with accents
- **WHEN** a movie title is `"Une Couronne pour Noël - 2015"`
- **THEN** the title sent to TMDB SHALL be `"Une Couronne pour Noël"` with year `2015`

#### Scenario: Dash not followed by a valid year is preserved
- **WHEN** a movie title is `"Spider-Man : No Way Home"`
- **THEN** the title sent to TMDB SHALL be `"Spider-Man : No Way Home"` with no year

### Requirement: Atomic movie/TV show upsert
When the processor creates a `Movie` or `TVShow` record, it SHALL use an atomic upsert operation to prevent duplicate key violations when multiple M3U entries map to the same TMDB entity within a single processing run.

#### Scenario: Duplicate TMDB movie in same batch
- **WHEN** two M3U entries (e.g., HD and FHD variants) resolve to the same TMDB movie
- **THEN** only one `Movie` row SHALL be created and both `ProcessedLine` entries SHALL reference it without any database error

#### Scenario: Movie already exists from previous run
- **WHEN** a movie with the same `tmdb_id` and `tmdb_year` already exists in the database
- **THEN** the processor SHALL reuse the existing row without error or duplicate insertion
