## 1. Radarr Client — Add TvdbID Field

- [x] 1.1 Add `TvdbID int \`json:"tvdbId"\`` to the `Movie` struct in `internal/external/radarr/radarr.go`

## 2. Download Command — Pass TvdbID to Matcher

- [x] 2.1 In `cmd/main.go`, update the `download radarr` loop to pass `movie.TvdbID` instead of `0` as the first argument to `matcher.MatchMovieByTVDB`
- [x] 2.2 Remove or update the comment "Note: Radarr doesn't provide TVDB ID..." to reflect the corrected behavior

## 3. Tests & Verification

- [x] 3.1 Add a unit test in `internal/external/radarr/radarr_test.go` (or equivalent) verifying that a JSON payload with `"tvdbId": 12345` deserialises to `Movie.TvdbID == 12345`, and a payload without `"tvdbId"` gives `TvdbID == 0`
- [x] 3.2 Run `go build ./internal/... ./cmd/...` and confirm no compilation errors
