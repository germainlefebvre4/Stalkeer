## ADDED Requirements

### Requirement: Radarr Movie struct captures TVDB ID from API
The `radarr.Movie` struct SHALL include a `TvdbID int` field mapped to the `"tvdbId"` JSON key from Radarr's REST API response, so the TVDB ID is available for database matching.

#### Scenario: TVDB ID is parsed from Radarr response
- **WHEN** Radarr's API returns a movie object with `"tvdbId": 12345`
- **THEN** the parsed `radarr.Movie.TvdbID` field SHALL equal `12345`

#### Scenario: Missing tvdbId field defaults to zero
- **WHEN** Radarr's API returns a movie object without a `"tvdbId"` key
- **THEN** `radarr.Movie.TvdbID` SHALL equal `0` and no error SHALL occur

### Requirement: download radarr uses TVDB ID as primary match key
The `download radarr` command SHALL pass `movie.TvdbID` to `MatchMovieByTVDB` so the TVDB-primary key path is used when available, falling through to TMDB-ID and fuzzy title matching only when `TvdbID == 0`.

#### Scenario: Movie matched by TVDB ID when available
- **WHEN** a Radarr movie has `TvdbID = 12345` and a `Movie` record exists in the database with `tvdb_id = 12345`
- **THEN** the command SHALL match that record with confidence 100% via the TVDB-primary path

#### Scenario: Falls through to TMDB ID when TVDB ID is zero
- **WHEN** a Radarr movie has `TvdbID = 0` and a `Movie` record exists with the matching `tmdb_id`
- **THEN** the command SHALL match via the TMDB-ID path

#### Scenario: Falls through to fuzzy matching when both IDs miss
- **WHEN** a Radarr movie has `TvdbID = 0` and no `Movie` record matches its `TMDBID`
- **THEN** the command SHALL attempt fuzzy title+year matching as the final fallback
