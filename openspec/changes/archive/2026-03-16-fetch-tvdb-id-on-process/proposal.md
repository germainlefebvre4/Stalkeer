## Why

The `download radarr` command can rarely match movies because the Radarr API response includes a `tvdbId` field that the local `radarr.Movie` struct does not capture. As a result, `MatchMovieByTVDB` is always called with `tvdbID = 0`, the TVDB-primary match path is permanently skipped, and every lookup falls through to TMDB-ID fuzzy matching — which fails whenever TMDB enrichment during `process` did not succeed for that title.

## What Changes

- Add `TvdbID int` (`json:"tvdbId"`) to the `radarr.Movie` struct so the Radarr API value is parsed
- Update `download radarr` to pass `movie.TvdbID` (instead of `0`) to `MatchMovieByTVDB`
- Add a comment clarifying the matching priority: TVDB ID → TMDB ID → fuzzy title/year

## Capabilities

### New Capabilities
*(none)*

### Modified Capabilities
- `radarr-movie-matching`: the Radarr client Movie model now includes `tvdbId`, enabling primary-key TVDB matching in the download command

## Impact

- `internal/external/radarr/radarr.go` — add `TvdbID` field to `Movie` struct
- `cmd/main.go` — update the `download radarr` loop to pass `movie.TvdbID` to `MatchMovieByTVDB`
- No migration, no schema change, no other packages affected
