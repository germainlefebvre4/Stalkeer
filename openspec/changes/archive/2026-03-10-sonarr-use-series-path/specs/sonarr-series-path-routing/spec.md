## ADDED Requirements

### Requirement: Sonarr series download path routing
When downloading a missing TV show episode, the system SHALL derive the full destination directory from `series.Path` as returned by the Sonarr API, which encodes the root folder chosen in Sonarr's Media Management.

#### Scenario: Series in primary root folder
- **WHEN** `series.Path` is `/downloads/sonarr/Breaking Bad`
- **THEN** the episode is downloaded to `/downloads/sonarr/Breaking Bad/Season 01/Breaking Bad - S01E01`

#### Scenario: Series in secondary root folder
- **WHEN** `series.Path` is `/downloads/sonarr-bis/Malcolm in the Middle`
- **THEN** the episode is downloaded to `/downloads/sonarr-bis/Malcolm in the Middle/Season 01/Malcolm in the Middle - S01E01`

#### Scenario: Empty series path fallback
- **WHEN** `series.Path` is empty
- **THEN** the system SHALL fall back to constructing the path from `cfg.Downloads.TVShowsPath` and `series.Title` (previous behavior)

### Requirement: Radarr movie download path routing
When downloading a missing movie, the system SHALL derive the full destination directory from `movie.Path` as returned by the Radarr API.

#### Scenario: Movie in primary root folder
- **WHEN** `movie.Path` is `/downloads/radarr/The Matrix (1999)`
- **THEN** the movie is downloaded to `/downloads/radarr/The Matrix (1999)/The Matrix (1999)`

#### Scenario: Movie in secondary root folder
- **WHEN** `movie.Path` is `/downloads/radarr-4k/Inception (2010)`
- **THEN** the movie is downloaded to `/downloads/radarr-4k/Inception (2010)/Inception (2010)`

#### Scenario: Empty movie path fallback
- **WHEN** `movie.Path` is empty
- **THEN** the system SHALL fall back to constructing the path from `cfg.Downloads.MoviesPath` and `movie.Title` (previous behavior)
